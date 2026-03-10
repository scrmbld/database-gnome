// This package contains code for interfacing with our AI apis
package glue

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
)

const LlamaUrl string = "https://127.0.0.1:8080"
const GroqUrl string = "https://api.groq.com/openai"

type Provider interface {
	Request(model string, systemPrompt string, userPrompt string) (CompletionResponse, error)
	FimRequest(model string, beforeContext string, afterContext string) (FimResponse, error)
}

type CompletionResponse struct {
	Created int `json:"created"`
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

type FimResponse struct {
	Content string `json:content`
}

type OAIApiProvider struct {
	apiToken    string
	providerUrl string
	logger      *log.Logger
}

func parseCompletionResponse(resp []byte) (CompletionResponse, error) {
	var result CompletionResponse
	err := json.Unmarshal(resp, &result)
	if err != nil {
		return CompletionResponse{}, err
	}

	return result, nil
}

func parseFimResponse(resp []byte) (FimResponse, error) {
	var result FimResponse
	err := json.Unmarshal(resp, &result)
	if err != nil {
		return FimResponse{}, err
	}

	return result, nil
}

func (g *OAIApiProvider) Request(model string, systemPrompt string, userPrompt string) (CompletionResponse, error) {
	g.logger.Printf("Model: Request: %s", userPrompt)

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
	"temperature": 0.4,
	"max_completion_tokens": 512,
	"top_p": 0.85
	}`, systemPrompt, userPrompt, model))
	bodyReader := bytes.NewReader(jsonBody)
	req, err := http.NewRequest("POST", g.providerUrl+"/v1/chat/completions", bodyReader)
	if err != nil {
		return CompletionResponse{}, err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", g.apiToken))
	req.Header.Add("Content-Type", "application/json")

	resp, err := client.Do(req)

	if err != nil {
		return CompletionResponse{}, err
	}

	respScanner := bufio.NewScanner(resp.Body)
	var respSb strings.Builder
	for respScanner.Scan() {
		buf := respScanner.Bytes()
		respSb.Write(buf)
	}

	g.logger.Printf("Model: %s", respSb.String())

	return parseCompletionResponse([]byte(respSb.String()))
}

func (g *OAIApiProvider) FimRequest(model string, beforeContext string, afterContext string) (FimResponse, error) {
	client := &http.Client{}
	jsonBody := []byte(fmt.Sprintf(`{
	"input_prefix": "%s",
	"input_suffix": "%s",
	"model": "starcoder2",
	"temperature": 0,
	"max_completion_tokens": 16,
	"top_p": 1
	}`, beforeContext, afterContext))

	bodyReader := bytes.NewReader(jsonBody)
	req, err := http.NewRequest("POST", g.providerUrl+"/infill", bodyReader)
	if err != nil {
		return FimResponse{}, err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", g.apiToken))
	req.Header.Add("Content-Type", "application/json")

	resp, err := client.Do(req)

	if err != nil {
		return FimResponse{}, err
	}

	respScanner := bufio.NewScanner(resp.Body)
	var respSb strings.Builder
	for respScanner.Scan() {
		buf := respScanner.Bytes()
		respSb.Write(buf)
	}

	g.logger.Printf("Model: %s", respSb.String())

	return parseFimResponse([]byte(respSb.String()))
}

func NewOAIApiProvider(apiToken string, providerUrl string, logger *log.Logger) OAIApiProvider {
	return OAIApiProvider{
		apiToken:    apiToken,
		providerUrl: providerUrl,
		logger:      logger,
	}
}
