package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewOpenAIClient_Defaults(t *testing.T) {
	client := NewOpenAIClient(OpenAIConfig{APIKey: "sk-test"})

	if client.apiKey != "sk-test" {
		t.Errorf("expected apiKey 'sk-test', got %q", client.apiKey)
	}
	if client.baseURL != "https://api.openai.com" {
		t.Errorf("expected baseURL 'https://api.openai.com', got %q", client.baseURL)
	}
	if client.model != "gpt-4o-mini" {
		t.Errorf("expected model 'gpt-4o-mini', got %q", client.model)
	}
}

func TestNewOpenAIClient_DeepSeekBaseURL(t *testing.T) {
	// DeepSeek with /v1 suffix
	client := NewOpenAIClient(OpenAIConfig{
		APIKey:  "sk-deepseek",
		BaseURL: "https://api.deepseek.com/v1",
		Model:   "deepseek-chat",
	})

	if client.baseURL != "https://api.deepseek.com" {
		t.Errorf("expected baseURL without /v1, got %q", client.baseURL)
	}
}

func TestNewOpenAIClient_TrailingSlash(t *testing.T) {
	client := NewOpenAIClient(OpenAIConfig{
		APIKey:  "sk-test",
		BaseURL: "https://api.deepseek.com/",
	})

	if client.baseURL != "https://api.deepseek.com" {
		t.Errorf("expected baseURL without trailing slash, got %q", client.baseURL)
	}
}

func TestNewOpenAIClient_CustomBaseURL(t *testing.T) {
	client := NewOpenAIClient(OpenAIConfig{
		APIKey:  "sk-test",
		BaseURL: "https://my-proxy.example.com",
		Model:   "custom-model",
	})

	if client.baseURL != "https://my-proxy.example.com" {
		t.Errorf("baseURL mismatch: got %q", client.baseURL)
	}
	if client.model != "custom-model" {
		t.Errorf("model mismatch: got %q", client.model)
	}
}

func TestChat_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request path
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer sk-test" {
			t.Errorf("unexpected auth header: %s", r.Header.Get("Authorization"))
		}

		resp := chatResponse{
			Choices: []struct {
				Message message `json:"message"`
			}{
				{Message: message{Role: "assistant", Content: "Hello! How can I help?"}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewOpenAIClient(OpenAIConfig{
		APIKey:  "sk-test",
		BaseURL: server.URL,
		Model:   "test-model",
	})

	messages := []Message{
		{Role: "user", Content: "Hi"},
	}
	response, err := client.Chat(context.Background(), "You are helpful", messages)
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}
	if response != "Hello! How can I help?" {
		t.Errorf("unexpected response: %q", response)
	}
}

func TestChat_SystemPromptPrepended(t *testing.T) {
	var capturedMessages []message
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req chatRequest
		json.NewDecoder(r.Body).Decode(&req)
		capturedMessages = req.Messages

		resp := chatResponse{
			Choices: []struct {
				Message message `json:"message"`
			}{
				{Message: message{Role: "assistant", Content: "ok"}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewOpenAIClient(OpenAIConfig{
		APIKey:  "sk-test",
		BaseURL: server.URL,
	})

	_, _ = client.Chat(context.Background(), "System prompt here", []Message{
		{Role: "user", Content: "User message"},
	})

	if len(capturedMessages) != 2 {
		t.Fatalf("expected 2 messages (system + user), got %d", len(capturedMessages))
	}
	if capturedMessages[0].Role != "system" {
		t.Errorf("expected first message role 'system', got %q", capturedMessages[0].Role)
	}
	if capturedMessages[0].Content != "System prompt here" {
		t.Errorf("expected system prompt, got %q", capturedMessages[0].Content)
	}
	if capturedMessages[1].Role != "user" {
		t.Errorf("expected second message role 'user', got %q", capturedMessages[1].Role)
	}
}

func TestChat_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":{"message":"Invalid API key","type":"auth_error"}}`))
	}))
	defer server.Close()

	client := NewOpenAIClient(OpenAIConfig{
		APIKey:  "bad-key",
		BaseURL: server.URL,
	})

	_, err := client.Chat(context.Background(), "sys", []Message{{Role: "user", Content: "hi"}})
	if err == nil {
		t.Error("expected error for 401 response")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("expected 401 in error, got: %v", err)
	}
}

func TestChat_EmptyChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := chatResponse{Choices: nil}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewOpenAIClient(OpenAIConfig{
		APIKey:  "sk-test",
		BaseURL: server.URL,
	})

	_, err := client.Chat(context.Background(), "sys", []Message{{Role: "user", Content: "hi"}})
	if err == nil {
		t.Error("expected error for empty choices")
	}
}

func TestChat_ResponseErrorField(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := chatResponse{
			Error: &struct {
				Message string `json:"message"`
				Type    string `json:"type"`
			}{Message: "rate limited", Type: "rate_limit"},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewOpenAIClient(OpenAIConfig{
		APIKey:  "sk-test",
		BaseURL: server.URL,
	})

	_, err := client.Chat(context.Background(), "sys", []Message{{Role: "user", Content: "hi"}})
	if err == nil {
		t.Error("expected error for API error response")
	}
}

func TestSummarize(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := chatResponse{
			Choices: []struct {
				Message message `json:"message"`
			}{
				{Message: message{Role: "assistant", Content: "- Added auth\n- Fixed bugs"}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewOpenAIClient(OpenAIConfig{
		APIKey:  "sk-test",
		BaseURL: server.URL,
	})

	summary, err := client.Summarize(context.Background(), "Add auth", "Conversation here...")
	if err != nil {
		t.Fatalf("Summarize failed: %v", err)
	}
	if summary == "" {
		t.Error("expected non-empty summary")
	}
}

func TestChat_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer server.Close()

	client := NewOpenAIClient(OpenAIConfig{
		APIKey:  "sk-test",
		BaseURL: server.URL,
	})

	_, err := client.Chat(context.Background(), "sys", []Message{{Role: "user", Content: "hi"}})
	if err == nil {
		t.Error("expected error for invalid JSON response")
	}
}
