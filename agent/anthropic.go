package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type AnthropicClient struct {
	apiKey     string
	model      string
	tools      []Tool
	httpClient *http.Client
	baseURL    string
}

func NewAnthropicClient(apiKey, model string, tools []Tool) *AnthropicClient {
	return &AnthropicClient{
		apiKey:     apiKey,
		model:      model,
		tools:      tools,
		httpClient: &http.Client{Timeout: 120 * time.Second},
		baseURL:    "https://api.anthropic.com",
	}
}

func (c *AnthropicClient) Send(ctx context.Context, system string, messages []Message) (Response, error) {
	apiMsgs := make([]anthropicMessage, 0, len(messages))
	for _, m := range messages {
		apiMsgs = append(apiMsgs, toAnthropicMessage(m))
	}

	var apiTools []anthropicTool
	for _, t := range c.tools {
		apiTools = append(apiTools, anthropicTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
		})
	}

	reqBody := anthropicRequest{
		Model:     c.model,
		MaxTokens: 4096,
		System:    system,
		Messages:  apiMsgs,
		Tools:     apiTools,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return Response{}, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return Response{}, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Response{}, fmt.Errorf("API call: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return Response{}, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return Response{}, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(respBody))
	}

	var apiResp anthropicResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return Response{}, fmt.Errorf("unmarshal response: %w", err)
	}

	return fromAnthropicResponse(apiResp), nil
}

// Anthropic API types

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system"`
	Messages  []anthropicMessage `json:"messages"`
	Tools     []anthropicTool    `json:"tools,omitempty"`
}

type anthropicMessage struct {
	Role    string                `json:"role"`
	Content []anthropicContent    `json:"content"`
}

type anthropicContent struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   string          `json:"content,omitempty"`
	IsError   bool            `json:"is_error,omitempty"`
}

type anthropicTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

type anthropicResponse struct {
	Content    []anthropicContent `json:"content"`
	StopReason string             `json:"stop_reason"`
}

func toAnthropicMessage(m Message) anthropicMessage {
	var content []anthropicContent

	if m.Text != "" {
		content = append(content, anthropicContent{Type: "text", Text: m.Text})
	}
	for _, tc := range m.ToolCalls {
		content = append(content, anthropicContent{
			Type:  "tool_use",
			ID:    tc.ID,
			Name:  tc.Name,
			Input: tc.Input,
		})
	}
	for _, tr := range m.ToolResults {
		content = append(content, anthropicContent{
			Type:      "tool_result",
			ToolUseID: tr.ToolCallID,
			Content:   tr.Content,
			IsError:   tr.IsError,
		})
	}

	return anthropicMessage{Role: m.Role, Content: content}
}

func fromAnthropicResponse(resp anthropicResponse) Response {
	var r Response
	r.Done = resp.StopReason == "end_turn"

	for _, block := range resp.Content {
		switch block.Type {
		case "text":
			r.Text = block.Text
		case "tool_use":
			r.ToolCalls = append(r.ToolCalls, ToolCall{
				ID:    block.ID,
				Name:  block.Name,
				Input: block.Input,
			})
		}
	}
	return r
}
