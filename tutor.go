package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

type ChatMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

func buildSystemPrompt(config Config, userFacts []string) string {

	var languageInstruction string
	var languageWarning string
	switch config.TutorLanguage {
	case TutorLanguageSource:
		languageInstruction = fmt.Sprintf("Respond in %s (the student's native language).", config.SourceLanguage)
		languageWarning = fmt.Sprintf("IMPORTANT: You MUST respond only in %s. Do NOT use any other language — not even a single word or phrase.", config.SourceLanguage)
	case TutorLanguageTarget:
		languageInstruction = fmt.Sprintf("Respond in %s (the language they are learning).", config.TargetLanguage)
		languageWarning = fmt.Sprintf("IMPORTANT: You MUST respond only in %s. Do NOT use any other language — not even a single word or phrase.", config.TargetLanguage)
	case TutorLanguageMixed:
		languageInstruction = fmt.Sprintf("Respond mainly in %s but naturally sprinkle in simple %s words and phrases to help the student learn.", config.SourceLanguage, config.TargetLanguage)
		languageWarning = fmt.Sprintf("IMPORTANT: Use your judgement on the mix — lean on %s for clarity but introduce %s vocabulary and short phrases naturally as the conversation flows.", config.SourceLanguage, config.TargetLanguage)
	}

	var factsSection string
	if len(userFacts) > 0 {
		factsSection = "What you know about the student:\n"
		for _, f := range userFacts {
			factsSection += "  - " + f + "\n"
		}
	}

	prompt := fmt.Sprintf(`You are a language tutor.
  The student speaks %s and is learning %s.
  %s
  %s
  %s
  Mode, i.e. whether you guide them in lessons, just converse with them, or a mix of conversation that breaks into mini lessons: %s
  Strictness level: %d. When the student makes a mistake, whether to correct it and store it depends on the strictness level:
    Level 1 — only correct and store serious mistakes that impede understanding. Ignore minor errors.
    Level 2 — correct and store most grammatical and vocabulary mistakes, but use judgement on minor ones.
    Level 3 — correct and store every mistake, no matter how small.
  When you do correct a mistake, call store_problem_word with the CORRECT form of the word (not the student's incorrect version) and the sentence they used it in. Then in your reply briefly explain what was wrong, what the correct form is, and why — keep it concise but informative. Never store simple typos (e.g. "biin" instead of "bin") regardless of strictness level.
  When the student asks to be quizzed or you decide it is a good time to quiz them, call the get_due_words tool to retrieve words due for review, then quiz them one word at a time using this exact two-step format:
    Step 1: Ask "What does [term] mean?" and wait for their answer.
    Step 2: Ask them to use the term correctly in a sentence and wait for their answer.
  Only after both steps are complete, call record_quiz_result — pass=true only if they answered both steps correctly. Then move on to the next word.
  IMPORTANT: During a quiz, do NOT call store_problem_word — even if the student makes a mistake. Only call record_quiz_result to record the outcome.
  After every tool call you MUST always follow up with a response to the student — never leave the conversation silent after a tool call.
  When the student mentions something personal about themselves (name, job, hobbies, family, etc.) that is worth remembering, call store_user_fact with a concise summary of the fact.`, config.SourceLanguage,
		config.TargetLanguage, languageInstruction, languageWarning, factsSection, config.Mode, config.Strictness)
	return prompt
}

type Tutor struct {
	config    Config
	client    Client
	messages  []ChatMessage
	tools     []Tool
	db        *sql.DB
	userFacts []string
}

func NewTutor(config Config) (Tutor, error) {
	key := os.Getenv("OPENROUTER_API_KEY")
	if key == "" {
		return Tutor{}, errors.New("OPENROUTER_API_KEY not set")
	}
	client := Client{
		baseURL: "https://openrouter.ai/api/v1/chat/completions",
		apiKey:  key,
	}

	userFacts := loadUserFacts()
	systemPrompt := buildSystemPrompt(config, userFacts)
	messages := []ChatMessage{{Role: "system", Content: systemPrompt}}

	db := openDB()

	return Tutor{config: config, client: client, messages: messages, tools: allTools, db: db, userFacts: userFacts}, nil
}

// rebuildSystemMessage updates the first message (system prompt) with the latest user facts.
func (t *Tutor) rebuildSystemMessage() {
	t.messages[0] = ChatMessage{Role: "system", Content: buildSystemPrompt(t.config, t.userFacts)}
}

func (t *Tutor) callModel(prompt string) (string, error) {
	newMessage := ChatMessage{Role: "user", Content: prompt}
	t.messages = append(t.messages, newMessage)

	// The model can return content and tool calls in the same message — it has already
	// composed a reply but also wants to trigger a side effect (e.g. storing a mistake).
	// We save that content as a fallback in case the follow-up response after tool
	// execution comes back empty.
	var fallbackContent string
	for {
		retries := 0  // we retry if the model returns an empty looking results
		var msg ChatMessage
		var err error
		for {
			msg, err = t.client.sendRequest(t.config.Model, t.messages, t.tools)
			//fmt.Printf("msg: %+v\n", msg)
			if err != nil {
				return "", err
			}
			if len(msg.ToolCalls) == 0 && strings.TrimSpace(msg.Content) == "" {
				retries++
				if retries >= 3 {
					return "", errors.New("model returned empty response")
				}
				continue  // retry without appending bad message to history
			}
			break // good message
		}

		t.messages = append(t.messages, msg)

		if len(msg.ToolCalls) == 0 {
			if msg.Content == "" && fallbackContent != "" {
				return fallbackContent, nil
			}
			return msg.Content, nil
		}

		// update fallback whenever the model includes content alongside tool calls
		if msg.Content != "" {
			fallbackContent = msg.Content
		}

		// execute each tool call and append results
		for _, tc := range msg.ToolCalls {
			result := t.executeTool(tc)
			t.messages = append(t.messages, ChatMessage{
				Role:       "tool",
				Content:    result,
				ToolCallID: tc.ID,
			})
		}
		// loop back and call model again with tool results
	}
}

func (t *Tutor) executeTool(toolCall ToolCall) string {
	switch toolCall.Function.Name {
	case "store_problem_word":
		var args StoreProblemWordArgs
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
			return "error parsing arguments"
		}
		fmt.Println(toolStyle.Render("⟳ Saving problem term: \"" + args.Term + "\""))
		if err := storeTerm(t.db, args.Term, args.ProblemSentence); err != nil {
			return "error storing term"
		}
		return "stored successfully"
	case "get_due_words":
		fmt.Println(toolStyle.Render("⟳ Fetching due words..."))
		terms, err := getDueTerms(t.db)
		if err != nil {
			return "error fetching due terms"
		}
		if len(terms) == 0 {
			return "no terms due for review"
		}
		result, _ := json.Marshal(terms)
		return string(result)
	case "record_quiz_result":
		var args RecordQuizResultArgs
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
			return "error parsing arguments"
		}
		passed := "✗"
		if args.Passed {
			passed = "✓"
		}
		fmt.Println(toolStyle.Render("⟳ Recording quiz result for: \"" + args.Term + "\" " + passed))
		if err := recordResult(t.db, args.Term, args.Passed); err != nil {
			return "error recording result"
		}
		return "recorded successfully"
	case "store_user_fact":
		var args StoreUserFactArgs
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
			return "error parsing arguments"
		}
		fmt.Println(toolStyle.Render("⟳ Remembering: \"" + args.Fact + "\""))
		if err := appendUserFact(args.Fact); err != nil {
			return "error storing fact"
		}
		t.userFacts = append(t.userFacts, args.Fact)
		t.rebuildSystemMessage()
		return "stored successfully"
	default:
		return "unknown tool"
	}
}
