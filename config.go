package main

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Kubeconfig string
	Namespace  string // empty = all namespaces

	EventReasons []string
	WindowSize   time.Duration
	Threshold    int
	Cooldown     time.Duration

	ClusterName     string
	SlackWebhookURL string
	SlackMention    string

	TelegramBotToken string
	TelegramChatID   string
	TelegramMention  string

	// LLM provider: "anthropic", "openai"
	LLMProvider string
	LLMAPIKey   string
	LLMModel    string
	LLMBaseURL  string // for OpenAI-compatible endpoints (Azure, Ollama, etc.)
	LogLevel    slog.Level
}

func LoadConfig() (Config, error) {
	c := Config{
		Kubeconfig:   os.Getenv("KUBECONFIG"),
		Namespace:    os.Getenv("WATCH_NAMESPACE"),
		EventReasons: []string{
			// Pod events
			"Failed", "BackOff", "OOMKilled", "OOMKilling", "Unhealthy", "FailedScheduling",
			// Node/system events
			"EvictionThresholdMet", "SystemOOM",
			// Volume/storage events
			"FailedAttachVolume", "FailedMount",
			// Pod lifecycle events
			"FailedCreate", "DeadlineExceeded", "BackoffLimitExceeded",
			// Flux helm-controller
			"InstallFailed", "UpgradeFailed", "RollbackFailed", "TestFailed",
			// Flux kustomize-controller
			"BuildFailed", "HealthCheckFailed", "ReconciliationFailed", "ArtifactFailed", "PruneFailed",
			// Flux source-controller
			"GitOperationFailed", "AuthenticationFailed", "IndexationFailed",
		},
		WindowSize:   10 * time.Minute,
		Threshold:    5,
		Cooldown:     1 * time.Hour,
		LLMProvider:  "anthropic",
		LLMModel:     "claude-haiku-4-5-20251001",
		LogLevel:     slog.LevelInfo,
	}

	if v := os.Getenv("EVENT_REASONS"); v != "" {
		c.EventReasons = strings.Split(v, ",")
	}
	if v := os.Getenv("WINDOW_SIZE"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return c, fmt.Errorf("invalid WINDOW_SIZE: %w", err)
		}
		c.WindowSize = d
	}
	if v := os.Getenv("THRESHOLD"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return c, fmt.Errorf("invalid THRESHOLD: %w", err)
		}
		c.Threshold = n
	}
	if v := os.Getenv("COOLDOWN"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return c, fmt.Errorf("invalid COOLDOWN: %w", err)
		}
		c.Cooldown = d
	}

	c.ClusterName = os.Getenv("CLUSTER_NAME")

	c.SlackWebhookURL = os.Getenv("SLACK_WEBHOOK_URL")
	c.SlackMention = os.Getenv("SLACK_MENTION")

	c.TelegramBotToken = os.Getenv("TELEGRAM_BOT_TOKEN")
	c.TelegramChatID = os.Getenv("TELEGRAM_CHAT_ID")
	c.TelegramMention = os.Getenv("TELEGRAM_MENTION")

	if v := os.Getenv("LLM_PROVIDER"); v != "" {
		c.LLMProvider = v
	}
	if v := os.Getenv("LLM_MODEL"); v != "" {
		c.LLMModel = v
	}
	c.LLMBaseURL = os.Getenv("LLM_BASE_URL")

	if v := os.Getenv("LOG_LEVEL"); v != "" {
		switch strings.ToLower(v) {
		case "debug":
			c.LogLevel = slog.LevelDebug
		case "info":
			c.LogLevel = slog.LevelInfo
		case "warn":
			c.LogLevel = slog.LevelWarn
		case "error":
			c.LogLevel = slog.LevelError
		default:
			return c, fmt.Errorf("invalid LOG_LEVEL: %s (use debug, info, warn, error)", v)
		}
	}

	c.LLMAPIKey = os.Getenv("LLM_API_KEY")
	if c.LLMAPIKey == "" {
		return c, fmt.Errorf("LLM_API_KEY is required")
	}

	return c, nil
}
