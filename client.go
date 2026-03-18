package main

import (
	"bytes"
	"encoding/json"
	//"fmt"
	"io"
	"net/http"
)

type Client struct {
	baseURL string
	apiKey  string
}

type ChatRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
}

type ChatResponse struct {
	Choices []Choice `json:"choices"`
}

type Choice struct {
	Message ChatMessage `json:"message"`
}

func (c Client) sendRequest(messages []ChatMessage, model string) (ChatMessage, error) {
	chatRequest := ChatRequest{Model: model, Messages: messages}
	jsonData, err := json.Marshal(chatRequest) // Encode Go struct to JSON bytes
	if err != nil {
		// Handle error
		return ChatMessage{}, err
	}
	// Create a POST request with the JSON data as an io.Reader
	req, err := http.NewRequest(http.MethodPost, c.baseURL, bytes.NewBuffer(jsonData))
	if err != nil {
		// Handle error
		return ChatMessage{}, err
	}

	// Set necessary headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return ChatMessage{}, err
	}
	defer res.Body.Close()

	// Read and process the response body as needed
	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return ChatMessage{}, err
	}

	var response ChatResponse
	json.Unmarshal(resBody, &response)
	return response.Choices[0].Message, nil
}
