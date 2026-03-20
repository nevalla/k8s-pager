package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type SlackNotifier struct {
	webhookURL string
	mention    string
	httpClient *http.Client
}

func NewSlackNotifier(webhookURL, mention string) *SlackNotifier {
	return &SlackNotifier{
		webhookURL: webhookURL,
		mention:    mention,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *SlackNotifier) Send(ctx context.Context, alert Alert) error {
	payload := s.buildPayload(alert)
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal slack payload: %w", err)
	}

	slog.Debug("sending slack alert", "payload", string(body))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send slack message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack returned status %d", resp.StatusCode)
	}

	slog.Info("slack alert sent",
		"cluster", alert.Cluster,
		"namespace", alert.Namespace,
		"kind", alert.ResourceKind,
		"name", alert.ResourceName,
		"reason", alert.Reason,
	)
	return nil
}

func (s *SlackNotifier) buildPayload(alert Alert) map[string]any {
	blocks := []map[string]any{
		{
			"type": "header",
			"text": map[string]string{
				"type": "plain_text",
				"text": fmt.Sprintf("K8s Alert: %s", alert.Reason),
			},
		},
		{
			"type": "section",
			"fields": []map[string]string{
				{"type": "mrkdwn", "text": fmt.Sprintf("*Cluster:*\n%s", slackEscape(alert.Cluster))},
				{"type": "mrkdwn", "text": fmt.Sprintf("*Namespace:*\n%s", slackEscape(alert.Namespace))},
				{"type": "mrkdwn", "text": fmt.Sprintf("*%s:*\n%s", alert.ResourceKind, slackEscape(alert.ResourceName))},
				{"type": "mrkdwn", "text": fmt.Sprintf("*Events:*\n%d in %s", alert.Count, alert.Window)},
			},
		},
		{
			"type": "section",
			"text": map[string]string{
				"type": "mrkdwn",
				"text": fmt.Sprintf("```%s```", alert.Message),
			},
		},
		{
			"type": "divider",
		},
		{
			"type": "section",
			"text": map[string]string{
				"type": "mrkdwn",
				"text": fmt.Sprintf("*AI Diagnosis*\n\n%s", stripCodeFences(alert.Diagnosis)),
			},
		},
	}

	if s.mention != "" {
		blocks = append(blocks, map[string]any{
			"type": "context",
			"elements": []map[string]string{
				{"type": "mrkdwn", "text": fmt.Sprintf("<@%s>", s.mention)},
			},
		})
	}

	return map[string]any{"blocks": blocks}
}

// slackEscape prevents Slack from interpreting special sequences like :emoji: in user-provided text.
func slackEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, ":", "\u200B:\u200B")
	return s
}

// stripCodeFences removes markdown code fences that LLMs sometimes wrap responses in.
func stripCodeFences(s string) string {
	lines := strings.Split(s, "\n")
	if len(lines) < 2 {
		return s
	}
	first := strings.TrimSpace(lines[0])
	if strings.HasPrefix(first, "```") {
		lines = lines[1:]
	}
	if len(lines) > 0 {
		last := strings.TrimSpace(lines[len(lines)-1])
		if last == "```" {
			lines = lines[:len(lines)-1]
		}
	}
	return strings.Join(lines, "\n")
}
