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

type TelegramNotifier struct {
	botToken   string
	chatID     string
	mention    string
	baseURL    string
	httpClient *http.Client
}

func NewTelegramNotifier(botToken, chatID, mention string) *TelegramNotifier {
	return &TelegramNotifier{
		botToken:   botToken,
		chatID:     chatID,
		mention:    mention,
		baseURL:    "https://api.telegram.org",
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (t *TelegramNotifier) Send(ctx context.Context, alert Alert) error {
	text := t.buildMessage(alert)

	payload := map[string]string{
		"chat_id":    t.chatID,
		"text":       text,
		"parse_mode": "MarkdownV2",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal telegram payload: %w", err)
	}

	url := fmt.Sprintf("%s/bot%s/sendMessage", t.baseURL, t.botToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send telegram message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram returned status %d", resp.StatusCode)
	}

	slog.Info("telegram alert sent",
		"cluster", alert.Cluster,
		"namespace", alert.Namespace,
		"kind", alert.ResourceKind,
		"name", alert.ResourceName,
		"reason", alert.Reason,
	)
	return nil
}

func (t *TelegramNotifier) buildMessage(alert Alert) string {
	e := telegramEscape
	var b strings.Builder

	if t.mention != "" {
		fmt.Fprintf(&b, "%s ", e(t.mention))
	}
	fmt.Fprintf(&b, "*K8s Alert: %s*\n\n", e(alert.Reason))
	fmt.Fprintf(&b, "*Cluster:* %s\n", e(alert.Cluster))
	fmt.Fprintf(&b, "*Namespace:* %s\n", e(alert.Namespace))
	fmt.Fprintf(&b, "*%s:* %s\n", e(alert.ResourceKind), e(alert.ResourceName))
	fmt.Fprintf(&b, "*Events:* %s in %s\n\n", e(fmt.Sprintf("%d", alert.Count)), e(alert.Window.String()))
	fmt.Fprintf(&b, "```\n%s\n```\n\n", alert.Message)
	fmt.Fprintf(&b, "*AI Diagnosis*\n\n%s", e(stripCodeFences(alert.Diagnosis)))

	return b.String()
}

// telegramEscape escapes special characters for Telegram MarkdownV2.
func telegramEscape(s string) string {
	for _, c := range []string{"_", "*", "[", "]", "(", ")", "~", "`", ">", "#", "+", "-", "=", "|", "{", "}", ".", "!"} {
		s = strings.ReplaceAll(s, c, "\\"+c)
	}
	return s
}
