// Package llm provides LLM client interfaces and OpenAI implementation.
package llm

import "context"

// Message represents a chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Client is the interface for LLM interactions.
type Client interface {
	// Chat sends messages and returns the assistant's response.
	Chat(ctx context.Context, systemPrompt string, messages []Message) (string, error)

	// Summarize generates a concise summary of the given content.
	Summarize(ctx context.Context, objective, conversation string) (string, error)
}
