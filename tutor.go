package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

type ChatMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

func buildSystemPrompt(config Config) string {

	var languageInstruction string
	switch config.TutorLanguage {
	case TutorLanguageSource:
		languageInstruction = fmt.Sprintf("Respond in %s (the student's native language).", config.SourceLanguage)
	case TutorLanguageTarget:
		languageInstruction = fmt.Sprintf("Respond in %s (the language they are learning).", config.TargetLanguage)
	case TutorLanguageMixed:
		languageInstruction = fmt.Sprintf("Mix %s and %s in your responses.", config.TargetLanguage, config.SourceLanguage)
	}

	prompt := fmt.Sprintf(`You are a language tutor.
  The student speaks %s and is learning %s.
  %s
  Mode, i.e. whether you guide them in lessons, just converse with them, or a mix of conversation that breaks into mini lessons: %s
  Strictness (1-3), i.e. how often you will correct their written mistakes: %d

  When the student makes a grammar or vocabulary mistake with a specific word, you MUST call the store_problem_word tool with the CORRECT form of the word (not the student's incorrect version) and the sentence they used it in. Then correct them naturally in your reply.
  When the student asks to be quizzed or you decide it is a good time to quiz them, call the get_due_words tool to retrieve words due for review, then quiz them one at a time.
  After each quiz question is answered, call record_quiz_result with whether they passed or failed.`, config.SourceLanguage,
		config.TargetLanguage, languageInstruction, config.Mode, config.Strictness)
	return prompt
}

type Tutor struct {
	config   Config
	client   Client
	messages []ChatMessage
	tools    []Tool
	db   *sql.DB
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

	systemPrompt := buildSystemPrompt(config)
	fmt.Println(systemPrompt)
	messages := []ChatMessage{{Role: "system", Content: systemPrompt}}

	db := openDB()

	return Tutor{config: config, client: client, messages: messages, tools: allTools, db: db}, nil
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
		msg, err := t.client.sendRequest(t.config.Model, t.messages, t.tools)
		fmt.Printf("msg: %+v\n", msg)
		if err != nil {
			return "", err
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
	fmt.Printf("Inside executeTool: %+v\n", toolCall)
	switch toolCall.Function.Name {
	case "store_problem_word":
		var args StoreProblemWordArgs
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
			return "error parsing arguments"
		}
		if err := storeTerm(t.db, args.Term, args.ProblemSentence); err != nil {
			return "error storing term"
		}
		return "stored successfully"
	case "get_due_words":
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
		if err := recordResult(t.db, args.Term, args.Passed); err != nil {
			return "error recording result"
		}
		return "recorded successfully"
	default:
		return "unknown tool"
	}
}
