// Package llm provides LLM client interfaces and OpenAI implementation.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OpenAIClient implements Client using the OpenAI Chat Completions API.
type OpenAIClient struct {
	apiKey  string
	baseURL string
	model   string
	client  *http.Client
}

// OpenAIConfig holds configuration for the OpenAI client.
type OpenAIConfig struct {
	APIKey  string
	BaseURL string // defaults to https://api.openai.com
	Model   string // defaults to gpt-4o-mini
	Timeout time.Duration
}

// NewOpenAIClient creates a new OpenAI API client.
// BaseURL should be the API root (e.g. "https://api.deepseek.com" or "https://api.openai.com").
// Trailing "/v1" is automatically stripped to avoid double-prefixing.
func NewOpenAIClient(cfg OpenAIConfig) *OpenAIClient {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.openai.com"
	}
	if cfg.Model == "" {
		cfg.Model = "gpt-4o-mini"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	// Strip trailing /v1 to avoid double /v1/v1 in URL construction
	baseURL := strings.TrimSuffix(cfg.BaseURL, "/v1")
	baseURL = strings.TrimSuffix(baseURL, "/")
	return &OpenAIClient{
		apiKey:  cfg.APIKey,
		baseURL: baseURL,
		model:   cfg.Model,
		client:  &http.Client{Timeout: cfg.Timeout},
	}
}

type chatRequest struct {
	Model    string    `json:"model"`
	Messages []message `json:"messages"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message message `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

// Chat sends messages and returns the assistant's response.
func (c *OpenAIClient) Chat(ctx context.Context, systemPrompt string, messages []Message) (string, error) {
	msgs := make([]message, 0, len(messages)+1)
	msgs = append(msgs, message{Role: "system", Content: systemPrompt})
	for _, m := range messages {
		msgs = append(msgs, message{Role: m.Role, Content: m.Content})
	}
	return c.doChat(ctx, msgs)
}

// Summarize generates a concise summary of the given content.
func (c *OpenAIClient) Summarize(ctx context.Context, objective, conversation string) (string, error) {
	systemPrompt := `You are a technical summarizer. Given an objective and conversation history
of coding agents that worked on it, produce a concise summary of:
1. What was accomplished
2. Key technical decisions made
3. Files changed or created
4. Any unresolved issues or follow-up items

Keep the summary concise and actionable. Use bullet points.`

	userMsg := fmt.Sprintf("Objective: %s\n\nConversation History:\n%s", objective, conversation)
	msgs := []message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userMsg},
	}
	return c.doChat(ctx, msgs)
}

func (c *OpenAIClient) doChat(ctx context.Context, msgs []message) (string, error) {
	body := chatRequest{
		Model:    c.model,
		Messages: msgs,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	url := c.baseURL + "/v1/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result chatResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if result.Error != nil {
		return "", fmt.Errorf("API error: %s (%s)", result.Error.Message, result.Error.Type)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	return result.Choices[0].Message.Content, nil
}
