package main

import (
	"errors"
	"fmt"
	"os"
)

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
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
  Strictness (1-3), i.e. how often you will correct their written mistakes: %d`, config.SourceLanguage,
		config.TargetLanguage, languageInstruction, config.Mode, config.Strictness)
	return prompt
}

type Tutor struct {
	config   Config
	client   Client
	messages []ChatMessage
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
	return Tutor{config: config, client: client, messages: messages}, nil
}

func (t *Tutor) callModel(prompt string) (string, error) {
	newMessage := ChatMessage{Role: "user", Content: prompt}
	t.messages = append(t.messages, newMessage)

	msg, err := t.client.sendRequest(t.messages, t.config.Model)
	if err != nil {
		return "", err
	}

	t.messages = append(t.messages, msg)
	return msg.Content, nil

}
