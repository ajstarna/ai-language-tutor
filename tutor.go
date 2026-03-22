package main

import (
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"text/template"
)

//go:embed prompts/*.tmpl
var promptFiles embed.FS

type ChatMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type promptData struct {
	SourceLanguage      string
	TargetLanguage      string
	LanguageInstruction string
	LanguageWarning     string
	Mode                Mode
	Strictness          int
	UserFacts           []string
}

func buildSystemPrompt(config Config, userFacts []string) string {
	var languageInstruction, languageWarning string
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

	data := promptData{
		SourceLanguage:      config.SourceLanguage,
		TargetLanguage:      config.TargetLanguage,
		LanguageInstruction: languageInstruction,
		LanguageWarning:     languageWarning,
		Mode:                config.Mode,
		Strictness:          config.Strictness,
		UserFacts:           userFacts,
	}

	files := []string{
		"prompts/base.tmpl",
		"prompts/corrections.tmpl",
		"prompts/quiz.tmpl",
		"prompts/tools.tmpl",
	}

	var buf strings.Builder
	for _, f := range files {
		tmpl := template.Must(template.ParseFS(promptFiles, f))
		if err := tmpl.Execute(&buf, data); err != nil {
			log.Fatalf("failed to render prompt template %s: %v", f, err)
		}
		buf.WriteString("\n")
	}
	return buf.String()
}

type Tutor struct {
	config    Config
	client    Client
	messages  []ChatMessage
	tools     []Tool
	db        *sql.DB
	userFacts []string
	inQuiz    bool
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
			activeTools := t.tools
		if t.inQuiz {
			activeTools = quizTools
		}
		msg, err = t.client.sendRequest(t.config.Model, t.messages, activeTools)
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
			t.inQuiz = false
			return "no terms due for review"
		}
		t.inQuiz = true
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
