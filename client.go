package main

type ChatRequest struct {
	Model string `json:"model"`
	Messages []ChatMessage `json:"messages"`
}

type ChatMessage struct {
	Role string `json:"role"`
	Content string `json:"content"`
}
