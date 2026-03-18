package main

import (
	"context"
	"errors"
	"os"
)

type Tutor struct {
	config Config
	client Client
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
	return Tutor{config: config, client: &client}, nil
}

func (t Tutor) callModel(prompt string) string {
	return "todo"
	/*
	if err != nil {
		panic(err.Error())
	}
	//fmt.Printf("%+v\n", message.Content)
	return message.Content[0].AsText().Text
	*/
}
