package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Alert struct {
	Cluster      string
	Namespace    string
	ResourceKind string
	ResourceName string
	Reason       string
	Count        int
	Window       time.Duration
	Message      string
	Diagnosis    string
}

type SlackNotifier struct {
	webhookURL string
	httpClient *http.Client
}

func NewSlackNotifier(webhookURL string) *SlackNotifier {
	return &SlackNotifier{
		webhookURL: webhookURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *SlackNotifier) Send(ctx context.Context, alert Alert) error {
	payload := buildSlackPayload(alert)
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal slack payload: %w", err)
	}

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
	return nil
}

func buildSlackPayload(alert Alert) map[string]any {
	blocks := []map[string]any{
		{
			"type": "header",
			"text": map[string]string{
				"type": "plain_text",
				"text": fmt.Sprintf("K8s Alert: %s [%s]", alert.Reason, alert.Cluster),
			},
		},
		{
			"type": "section",
			"fields": []map[string]string{
				{"type": "mrkdwn", "text": fmt.Sprintf("*Cluster:*\n%s", alert.Cluster)},
				{"type": "mrkdwn", "text": fmt.Sprintf("*Namespace:*\n%s", alert.Namespace)},
				{"type": "mrkdwn", "text": fmt.Sprintf("*%s:*\n%s", alert.ResourceKind, alert.ResourceName)},
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
			"type": "header",
			"text": map[string]string{
				"type": "plain_text",
				"text": "AI Diagnosis",
			},
		},
		{
			"type": "section",
			"text": map[string]string{
				"type": "mrkdwn",
				"text": alert.Diagnosis,
			},
		},
	}

	return map[string]any{"blocks": blocks}
}
