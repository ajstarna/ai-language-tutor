package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"fmt"
	"net/http"
)

type Client struct {
	baseURL string
	apiKey  string
}

type ChatRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
	Tools    []Tool        `json:"tools,omitempty"`
}

type ChatResponse struct {
	Choices []Choice `json:"choices"`
}

type Choice struct {
	Message ChatMessage `json:"message"`
}

func (c Client) sendRequest(model string, messages []ChatMessage, tools []Tool) (ChatMessage, error) {
	chatRequest := ChatRequest{Model: model, Messages: messages, Tools: tools}
	jsonData, err := json.Marshal(chatRequest) // Encode Go struct to JSON bytes

	//fmt.Printf("request body: %s\n", jsonData)

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

	fmt.Println(string(resBody))

	var response ChatResponse

	if err := json.Unmarshal(resBody, &response); err != nil {
		return ChatMessage{}, err
	}

	if len(response.Choices) == 0 {
		return ChatMessage{}, errors.New("empty response from API")
	}

	return response.Choices[0].Message, nil
}
