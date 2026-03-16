package agent

import (
	"context"
	"encoding/json"
)

// ToolCall represents a tool invocation requested by the model.
type ToolCall struct {
	ID    string
	Name  string
	Input json.RawMessage
}

// Response represents a model response, which may contain text, tool calls, or both.
type Response struct {
	Text      string
	ToolCalls []ToolCall
	Done      bool // true if the model is finished (no more tool calls)
}

// ToolResult is the outcome of executing a tool call.
type ToolResult struct {
	ToolCallID string
	Content    string
	IsError    bool
}

// Tool describes a tool the model can use.
type Tool struct {
	Name        string
	Description string
	InputSchema map[string]any
}

// LLMClient abstracts the LLM provider for multi-turn tool-use conversations.
type LLMClient interface {
	// Send sends messages to the model and returns a response.
	// The implementation handles provider-specific message formatting.
	Send(ctx context.Context, system string, messages []Message) (Response, error)
}

// Message represents a conversation message in a provider-agnostic format.
type Message struct {
	Role        string       // "user" or "assistant"
	Text        string       // text content (for user/assistant messages)
	ToolCalls   []ToolCall   // tool calls (for assistant messages)
	ToolResults []ToolResult // tool results (for user messages)
}
