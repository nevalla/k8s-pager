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

type OpenClawNotifier struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

func NewOpenClawNotifier(baseURL, token string) *OpenClawNotifier {
	return &OpenClawNotifier{
		baseURL:    strings.TrimRight(baseURL, "/"),
		token:      token,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (o *OpenClawNotifier) Send(ctx context.Context, alert Alert) error {
	payload := map[string]string{
		"message":  buildOpenClawMessage(alert),
		"name":     "k8s-pager",
		"wakeMode": "now",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal openclaw payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL+"/hooks/agent", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-openclaw-token", o.token)

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send openclaw webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("openclaw returned status %d", resp.StatusCode)
	}

	slog.Info("openclaw alert sent",
		"cluster", alert.Cluster,
		"namespace", alert.Namespace,
		"kind", alert.ResourceKind,
		"name", alert.ResourceName,
		"reason", alert.Reason,
	)
	return nil
}

func buildOpenClawMessage(alert Alert) string {
	var b strings.Builder

	fmt.Fprintf(&b, "K8s Alert: %s\n\n", alert.Reason)
	fmt.Fprintf(&b, "Cluster: %s\n", alert.Cluster)
	fmt.Fprintf(&b, "Namespace: %s\n", alert.Namespace)
	fmt.Fprintf(&b, "%s: %s\n", alert.ResourceKind, alert.ResourceName)
	fmt.Fprintf(&b, "Events: %d in %s\n\n", alert.Count, alert.Window)
	fmt.Fprintf(&b, "%s\n\n", alert.Message)
	fmt.Fprintf(&b, "Diagnosis:\n%s", stripCodeFences(alert.Diagnosis))

	return b.String()
}
