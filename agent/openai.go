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

// OpenAIClient works with OpenAI and any OpenAI-compatible API
// (Azure OpenAI, Ollama, vLLM, Together, etc.)
type OpenAIClient struct {
	apiKey     string
	model      string
	tools      []Tool
	httpClient *http.Client
	baseURL    string
}

func NewOpenAIClient(apiKey, model, baseURL string, tools []Tool) *OpenAIClient {
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}
	return &OpenAIClient{
		apiKey:     apiKey,
		model:      model,
		tools:      tools,
		httpClient: &http.Client{Timeout: 120 * time.Second},
		baseURL:    baseURL,
	}
}

func (c *OpenAIClient) Send(ctx context.Context, system string, messages []Message) (Response, error) {
	oaiMsgs := []oaiMessage{{Role: "system", Content: strPtr(system)}}
	for _, m := range messages {
		oaiMsgs = append(oaiMsgs, toOpenAIMessages(m)...)
	}

	var oaiTools []oaiTool
	for _, t := range c.tools {
		oaiTools = append(oaiTools, oaiTool{
			Type: "function",
			Function: oaiFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.InputSchema,
			},
		})
	}

	reqBody := oaiRequest{
		Model:    c.model,
		Messages: oaiMsgs,
		Tools:    oaiTools,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return Response{}, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return Response{}, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

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

	var oaiResp oaiResponse
	if err := json.Unmarshal(respBody, &oaiResp); err != nil {
		return Response{}, fmt.Errorf("unmarshal response: %w", err)
	}

	if len(oaiResp.Choices) == 0 {
		return Response{Done: true}, nil
	}

	return fromOpenAIResponse(oaiResp.Choices[0]), nil
}

// OpenAI API types

type oaiRequest struct {
	Model    string       `json:"model"`
	Messages []oaiMessage `json:"messages"`
	Tools    []oaiTool    `json:"tools,omitempty"`
}

type oaiMessage struct {
	Role       string          `json:"role"`
	Content    *string         `json:"content"`
	ToolCalls  []oaiToolCall   `json:"tool_calls,omitempty"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
}

type oaiTool struct {
	Type     string      `json:"type"`
	Function oaiFunction `json:"function"`
}

type oaiFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type oaiToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type oaiResponse struct {
	Choices []oaiChoice `json:"choices"`
}

type oaiChoice struct {
	Message      oaiMessage `json:"message"`
	FinishReason string     `json:"finish_reason"`
}

func toOpenAIMessages(m Message) []oaiMessage {
	var msgs []oaiMessage

	if m.Role == "assistant" {
		msg := oaiMessage{Role: "assistant"}
		if m.Text != "" || len(m.ToolCalls) == 0 {
			msg.Content = strPtr(m.Text)
		}
		for _, tc := range m.ToolCalls {
			msg.ToolCalls = append(msg.ToolCalls, oaiToolCall{
				ID:   tc.ID,
				Type: "function",
				Function: struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				}{
					Name:      tc.Name,
					Arguments: string(tc.Input),
				},
			})
		}
		msgs = append(msgs, msg)
	} else if len(m.ToolResults) > 0 {
		for _, tr := range m.ToolResults {
			msgs = append(msgs, oaiMessage{
				Role:       "tool",
				Content:    strPtr(tr.Content),
				ToolCallID: tr.ToolCallID,
			})
		}
	} else {
		msgs = append(msgs, oaiMessage{Role: "user", Content: strPtr(m.Text)})
	}

	return msgs
}

func strPtr(s string) *string { return &s }

func fromOpenAIResponse(choice oaiChoice) Response {
	var text string
	if choice.Message.Content != nil {
		text = *choice.Message.Content
	}
	r := Response{
		Text: text,
		Done: choice.FinishReason == "stop",
	}
	for _, tc := range choice.Message.ToolCalls {
		r.ToolCalls = append(r.ToolCalls, ToolCall{
			ID:    tc.ID,
			Name:  tc.Function.Name,
			Input: json.RawMessage(tc.Function.Arguments),
		})
	}
	return r
}
