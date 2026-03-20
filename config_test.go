package main

import (
	"log/slog"
	"testing"
)

func setEnv(t *testing.T, key, value string) {
	t.Helper()
	t.Setenv(key, value)
}

func TestLoadConfig_Defaults(t *testing.T) {
	// Clear all env vars that LoadConfig reads
	for _, key := range []string{
		"KUBECONFIG", "WATCH_NAMESPACE", "EVENT_REASONS", "WINDOW_SIZE",
		"THRESHOLD", "COOLDOWN", "CLUSTER_NAME", "SLACK_WEBHOOK_URL",
		"SLACK_MENTION", "TELEGRAM_BOT_TOKEN", "TELEGRAM_CHAT_ID",
		"TELEGRAM_MENTION", "LLM_PROVIDER",
		"LLM_MODEL", "LLM_BASE_URL", "LOG_LEVEL",
	} {
		t.Setenv(key, "")
	}
	setEnv(t, "LLM_API_KEY", "test-key")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}

	if cfg.Threshold != 5 {
		t.Errorf("Threshold = %d, want 5", cfg.Threshold)
	}
	if cfg.LLMProvider != "anthropic" {
		t.Errorf("LLMProvider = %q, want anthropic", cfg.LLMProvider)
	}
	if cfg.LogLevel != slog.LevelInfo {
		t.Errorf("LogLevel = %v, want Info", cfg.LogLevel)
	}
	if len(cfg.EventReasons) == 0 {
		t.Error("expected default event reasons")
	}
}

func TestLoadConfig_RequiresLLMAPIKey(t *testing.T) {
	t.Setenv("LLM_API_KEY", "")
	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected error when LLM_API_KEY is empty")
	}
}

func TestLoadConfig_InvalidThreshold(t *testing.T) {
	t.Setenv("LLM_API_KEY", "test-key")
	t.Setenv("THRESHOLD", "notanumber")

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected error for invalid THRESHOLD")
	}
}

func TestLoadConfig_InvalidWindowSize(t *testing.T) {
	t.Setenv("LLM_API_KEY", "test-key")
	t.Setenv("WINDOW_SIZE", "invalid")

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected error for invalid WINDOW_SIZE")
	}
}

func TestLoadConfig_InvalidLogLevel(t *testing.T) {
	t.Setenv("LLM_API_KEY", "test-key")
	t.Setenv("LOG_LEVEL", "verbose")

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected error for invalid LOG_LEVEL")
	}
}

func TestLoadConfig_LogLevels(t *testing.T) {
	tests := []struct {
		level  string
		expect slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"error", slog.LevelError},
		{"DEBUG", slog.LevelDebug},
	}
	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			t.Setenv("LLM_API_KEY", "test-key")
			t.Setenv("LOG_LEVEL", tt.level)
			// Clear other vars that might interfere
			for _, key := range []string{"THRESHOLD", "WINDOW_SIZE", "COOLDOWN", "EVENT_REASONS"} {
				t.Setenv(key, "")
			}

			cfg, err := LoadConfig()
			if err != nil {
				t.Fatalf("LoadConfig() error: %v", err)
			}
			if cfg.LogLevel != tt.expect {
				t.Errorf("LogLevel = %v, want %v", cfg.LogLevel, tt.expect)
			}
		})
	}
}

func TestLoadConfig_CustomEventReasons(t *testing.T) {
	t.Setenv("LLM_API_KEY", "test-key")
	t.Setenv("EVENT_REASONS", "BackOff,Failed")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}
	if len(cfg.EventReasons) != 2 {
		t.Fatalf("expected 2 event reasons, got %d", len(cfg.EventReasons))
	}
	if cfg.EventReasons[0] != "BackOff" || cfg.EventReasons[1] != "Failed" {
		t.Errorf("unexpected reasons: %v", cfg.EventReasons)
	}
}

func TestLoadConfig_NotifierConfig(t *testing.T) {
	t.Setenv("LLM_API_KEY", "test-key")
	t.Setenv("SLACK_WEBHOOK_URL", "https://hooks.slack.com/test")
	t.Setenv("SLACK_MENTION", "U12345")
	t.Setenv("TELEGRAM_BOT_TOKEN", "bot-token")
	t.Setenv("TELEGRAM_CHAT_ID", "chat-id")
	t.Setenv("TELEGRAM_MENTION", "@mybot")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}

	if cfg.SlackWebhookURL != "https://hooks.slack.com/test" {
		t.Error("SlackWebhookURL not set")
	}
	if cfg.SlackMention != "U12345" {
		t.Error("SlackMention not set")
	}
	if cfg.TelegramBotToken != "bot-token" {
		t.Error("TelegramBotToken not set")
	}
	if cfg.TelegramMention != "@mybot" {
		t.Error("TelegramMention not set")
	}
}
