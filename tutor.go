package main

import (
	"context"
	"errors"
	"os"
	"github.com/anthropics/anthropic-sdk-go"
)

type Tutor struct {
	config Config
	client *anthropic.Client
}

func NewTutor(config Config) (Tutor, error) {
	key := os.Getenv("ANTHROPIC_API_KEY")
	if key == "" {
		return Tutor{}, errors.New("ANTHROPIC_API_KEY not set")
	}
	client := anthropic.NewClient() // defaults to os.LookupEnv("ANTHROPIC_API_KEY")
	return Tutor{config: config, client: &client}, nil
}

func (t Tutor) callModel(prompt string) string {
	message, err := t.client.Messages.New(context.TODO(), anthropic.MessageNewParams{
		MaxTokens: 1024,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
		Model: anthropic.ModelClaudeHaiku4_5,
	})
	if err != nil {
		panic(err.Error())
	}
	//fmt.Printf("%+v\n", message.Content)
	return message.Content[0].AsText().Text
}
