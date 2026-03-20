package notifier

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func sampleAlert() Alert {
	return Alert{
		Cluster:      "test-cluster",
		Namespace:    "default",
		ResourceKind: "Pod",
		ResourceName: "my-app-abc123",
		Reason:       "BackOff",
		Count:        5,
		Window:       10 * time.Minute,
		Message:      "Back-off restarting failed container",
		Diagnosis:    "Container exits with code 1 due to missing DATABASE_URL",
	}
}

func TestSlackEscape(t *testing.T) {
	tests := []struct {
		input, expect string
	}{
		{"hello", "hello"},
		{"a & b", "a &amp; b"},
		{"<script>", "&lt;script&gt;"},
		{":emoji:", "\u200B:\u200Bemoji\u200B:\u200B"},
		{"arn:aws:eks", "arn\u200B:\u200Baws\u200B:\u200Beks"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := slackEscape(tt.input)
			if got != tt.expect {
				t.Errorf("slackEscape(%q) = %q, want %q", tt.input, got, tt.expect)
			}
		})
	}
}

func TestStripCodeFences(t *testing.T) {
	tests := []struct {
		name, input, expect string
	}{
		{"no fences", "hello world", "hello world"},
		{"with fences", "```\nhello\nworld\n```", "hello\nworld"},
		{"with language", "```markdown\nhello\nworld\n```", "hello\nworld"},
		{"single line", "hello", "hello"},
		{"empty fences", "```\n```", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripCodeFences(tt.input)
			if got != tt.expect {
				t.Errorf("stripCodeFences() = %q, want %q", got, tt.expect)
			}
		})
	}
}

func TestTelegramEscape(t *testing.T) {
	input := "hello_world *bold* [link](url)"
	got := telegramEscape(input)
	if !strings.Contains(got, "\\_") {
		t.Error("should escape underscores")
	}
	if !strings.Contains(got, "\\*") {
		t.Error("should escape asterisks")
	}
	if !strings.Contains(got, "\\[") {
		t.Error("should escape brackets")
	}
}

func TestSlackNotifier_Send(t *testing.T) {
	var receivedBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	s := NewSlackNotifier(server.URL, "U12345")
	err := s.Send(context.Background(), sampleAlert())
	if err != nil {
		t.Fatalf("Send() error: %v", err)
	}

	blocks, ok := receivedBody["blocks"].([]any)
	if !ok || len(blocks) == 0 {
		t.Fatal("expected blocks in payload")
	}

	// Check that mention block is present
	lastBlock := blocks[len(blocks)-1].(map[string]any)
	if lastBlock["type"] != "context" {
		t.Error("expected last block to be context (mention)")
	}
}

func TestSlackNotifier_NoMention(t *testing.T) {
	var receivedBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	s := NewSlackNotifier(server.URL, "")
	err := s.Send(context.Background(), sampleAlert())
	if err != nil {
		t.Fatalf("Send() error: %v", err)
	}

	blocks := receivedBody["blocks"].([]any)
	lastBlock := blocks[len(blocks)-1].(map[string]any)
	if lastBlock["type"] == "context" {
		t.Error("should not have mention block when mention is empty")
	}
}

func TestSlackNotifier_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	s := NewSlackNotifier(server.URL, "")
	err := s.Send(context.Background(), sampleAlert())
	if err == nil {
		t.Fatal("expected error on 500 response")
	}
}

func TestTelegramNotifier_Send(t *testing.T) {
	var receivedBody map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tn := &TelegramNotifier{
		botToken:   "fake-token",
		chatID:     "12345",
		mention:    "@openclaw",
		baseURL:    server.URL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
	err := tn.Send(context.Background(), sampleAlert())
	if err != nil {
		t.Fatalf("Send() error: %v", err)
	}

	if receivedBody["chat_id"] != "12345" {
		t.Errorf("expected chat_id=12345, got %s", receivedBody["chat_id"])
	}
	if receivedBody["parse_mode"] != "MarkdownV2" {
		t.Error("expected parse_mode=MarkdownV2")
	}
	if !strings.Contains(receivedBody["text"], "BackOff") {
		t.Error("expected text to contain reason")
	}
}

func TestTelegramNotifier_NoMention(t *testing.T) {
	tn := &TelegramNotifier{chatID: "12345"}
	msg := tn.buildMessage(sampleAlert())
	if strings.HasPrefix(msg, "@") || strings.HasPrefix(msg, "\\@") {
		t.Error("should not have mention prefix when mention is empty")
	}
}

func TestMultiNotifier_FansOut(t *testing.T) {
	var calls int
	n1 := &mockNotifier{fn: func() error { calls++; return nil }}
	n2 := &mockNotifier{fn: func() error { calls++; return nil }}

	m := NewMultiNotifier(n1, n2)
	err := m.Send(context.Background(), sampleAlert())
	if err != nil {
		t.Fatalf("Send() error: %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected 2 calls, got %d", calls)
	}
}

func TestMultiNotifier_CollectsErrors(t *testing.T) {
	n1 := &mockNotifier{fn: func() error { return fmt.Errorf("fail1") }}
	n2 := &mockNotifier{fn: func() error { return fmt.Errorf("fail2") }}

	m := NewMultiNotifier(n1, n2)
	err := m.Send(context.Background(), sampleAlert())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "fail1") || !strings.Contains(err.Error(), "fail2") {
		t.Errorf("expected both errors, got: %v", err)
	}
}

func TestMultiNotifier_PartialFailure(t *testing.T) {
	var calls int
	n1 := &mockNotifier{fn: func() error { calls++; return fmt.Errorf("fail") }}
	n2 := &mockNotifier{fn: func() error { calls++; return nil }}

	m := NewMultiNotifier(n1, n2)
	err := m.Send(context.Background(), sampleAlert())
	if err == nil {
		t.Fatal("expected error from first notifier")
	}
	if calls != 2 {
		t.Fatal("should still call second notifier even if first fails")
	}
}

type mockNotifier struct {
	fn func() error
}

func (m *mockNotifier) Send(_ context.Context, _ Alert) error {
	return m.fn()
}
