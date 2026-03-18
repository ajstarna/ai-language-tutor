package main

import (
	"errors"
	"os"
)

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
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
	return Tutor{config: config, client: client}, nil
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
