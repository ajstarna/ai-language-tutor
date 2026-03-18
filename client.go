package main

import (
	"bytes"
	"encoding/json"
	"fmt"
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

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func (c Client) send(chatRequest ChatRequest) (string, error) {
	jsonData, err := json.Marshal(chatRequest) // Encode Go struct to JSON bytes
	if err != nil {
		// Handle error
		return "", err
	}
	// Create a POST request with the JSON data as an io.Reader
	req, err := http.NewRequest(http.MethodPost, c.baseURL, bytes.NewBuffer(jsonData))
	if err != nil {
		// Handle error
		return "", err
	}

	// Set necessary headers
	req.Header.Set("Content-Type", "application/json")     // Inform the server the body is JSON
	req.Header.Set("Authorization", "Bearer your_api_key") // Add an authorization token


	httpClient := &http.DefaultClient{}
	res, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	// Read and process the response body as needed
	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	fmt.Printf("client: status code: %d\n", res.StatusCode)
	fmt.Printf("client: response body: %s\n", resBody)

	return resBody, nil
}
