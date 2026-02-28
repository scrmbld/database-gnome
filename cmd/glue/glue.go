// This package contains code for interfacing with our AI apis
package glue

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
)

type ModelResponse struct {
	Created int `json:"created"`
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func parseResponse(resp []byte) (ModelResponse, error) {
	var result ModelResponse
	err := json.Unmarshal(resp, &result)
	if err != nil {
		return ModelResponse{}, err
	}

	return result, nil
}

func Request(logger *log.Logger, model string, systemPrompt string, userPrompt string) (ModelResponse, error) {
	logger.Printf("Model: Request: %s", userPrompt)

	groqToken := os.Getenv("GROQ_API_KEY")

	client := &http.Client{}
	jsonBody := []byte(fmt.Sprintf(`{"messages": [
		{
			"role": "system",
			"content": "%s"
		},
		{
			"role": "user",
			"content": "%s"
		}
	],
	"model": "%s",
	"temperature": 0,
	"max_completion_tokens": 512,
	"reasoning_effort": "low",
	"top_p": 1
	}`, systemPrompt, userPrompt, model))
	bodyReader := bytes.NewReader(jsonBody)
	req, err := http.NewRequest("POST", "https://api.groq.com/openai/v1/chat/completions", bodyReader)
	if err != nil {
		return ModelResponse{}, err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", groqToken))
	req.Header.Add("Content-Type", "application/json")

	resp, err := client.Do(req)

	if err != nil {
		return ModelResponse{}, err
	}

	respScanner := bufio.NewScanner(resp.Body)
	var respSb strings.Builder
	for respScanner.Scan() {
		buf := respScanner.Bytes()
		respSb.Write(buf)
	}

	logger.Printf("Model: %s", respSb.String())

	return parseResponse([]byte(respSb.String()))
}
