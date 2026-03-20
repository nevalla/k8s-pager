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

type WhatsAppNotifier struct {
	apiURL     string
	apiToken   string
	recipient  string
	httpClient *http.Client
}

func NewWhatsAppNotifier(apiURL, apiToken, recipient string) *WhatsAppNotifier {
	return &WhatsAppNotifier{
		apiURL:     strings.TrimRight(apiURL, "/"),
		apiToken:   apiToken,
		recipient:  recipient,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (w *WhatsAppNotifier) Send(ctx context.Context, alert Alert) error {
	text := buildWhatsAppMessage(alert)

	payload := map[string]any{
		"messaging_product": "whatsapp",
		"to":                w.recipient,
		"type":              "text",
		"text": map[string]string{
			"body": text,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal whatsapp payload: %w", err)
	}

	url := w.apiURL + "/messages"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+w.apiToken)

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send whatsapp message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("whatsapp returned status %d", resp.StatusCode)
	}

	slog.Info("whatsapp alert sent",
		"cluster", alert.Cluster,
		"namespace", alert.Namespace,
		"kind", alert.ResourceKind,
		"name", alert.ResourceName,
		"reason", alert.Reason,
	)
	return nil
}

func buildWhatsAppMessage(alert Alert) string {
	var b strings.Builder

	fmt.Fprintf(&b, "⚠️ K8s Alert: %s\n\n", alert.Reason)
	fmt.Fprintf(&b, "Cluster: %s\n", alert.Cluster)
	fmt.Fprintf(&b, "Namespace: %s\n", alert.Namespace)
	fmt.Fprintf(&b, "%s: %s\n", alert.ResourceKind, alert.ResourceName)
	fmt.Fprintf(&b, "Events: %d in %s\n\n", alert.Count, alert.Window)
	fmt.Fprintf(&b, "%s\n\n", alert.Message)
	fmt.Fprintf(&b, "AI Diagnosis:\n%s", stripCodeFences(alert.Diagnosis))

	return b.String()
}
