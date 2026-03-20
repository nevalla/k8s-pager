package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	eventsv1 "k8s.io/api/events/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/nevalla/k8s-pager/agent"
	"github.com/nevalla/k8s-pager/counter"
	"github.com/nevalla/k8s-pager/kube"
	"github.com/nevalla/k8s-pager/notifier"
	"github.com/nevalla/k8s-pager/watcher"
)

func main() {
	cfg, err := LoadConfig()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: cfg.LogLevel})))

	kubeClient, dynClient, err := buildKubeClients(cfg)
	if err != nil {
		slog.Error("failed to create kube client", "error", err)
		os.Exit(1)
	}

	if cfg.ClusterName == "" {
		cfg.ClusterName = clusterNameFromKubeconfig(cfg.Kubeconfig)
	}

	filter := watcher.NewReasonFilter(cfg.EventReasons)
	window := counter.NewSlidingWindow(cfg.WindowSize, cfg.Threshold)
	dedup := counter.NewDeduplicator(cfg.Cooldown)
	executor := kube.NewExecutor(kubeClient, dynClient)
	tools := agent.DefaultTools()
	llmClient, err := buildLLMClient(cfg, tools)
	if err != nil {
		slog.Error("failed to create LLM client", "error", err)
		os.Exit(1)
	}
	troubleshooter := agent.NewTroubleshootingAgent(llmClient, executor)

	var notifiers []notifier.Notifier
	if cfg.SlackWebhookURL != "" {
		notifiers = append(notifiers, notifier.NewSlackNotifier(cfg.SlackWebhookURL, cfg.SlackMention))
	}
	if cfg.TelegramBotToken != "" {
		notifiers = append(notifiers, notifier.NewTelegramNotifier(cfg.TelegramBotToken, cfg.TelegramChatID, cfg.TelegramMention))
	}

	var notify notifier.Notifier
	switch len(notifiers) {
	case 0:
		slog.Info("no notifiers configured, using log notifier")
		notify = notifier.NewLogNotifier()
	case 1:
		notify = notifiers[0]
	default:
		notify = notifier.NewMultiNotifier(notifiers...)
	}

	handler := func(ctx context.Context, ev *eventsv1.Event) {
		key := counter.Key(fmt.Sprintf("%s/%s/%s/%s", ev.Regarding.Namespace, ev.Regarding.Kind, ev.Regarding.Name, ev.Reason))

		eventTime := ev.CreationTimestamp.Time
		if !ev.EventTime.IsZero() {
			eventTime = ev.EventTime.Time
		}

		if !window.Record(key, eventTime) {
			return
		}
		if !dedup.ShouldNotify(key) {
			return
		}

		alert := notifier.Alert{
			Cluster:      cfg.ClusterName,
			Namespace:    ev.Regarding.Namespace,
			ResourceKind: ev.Regarding.Kind,
			ResourceName: ev.Regarding.Name,
			Reason:       ev.Reason,
			Count:        cfg.Threshold,
			Window:       cfg.WindowSize,
			Message:      ev.Note,
		}

		slog.Info("threshold reached, starting diagnosis",
			"namespace", alert.Namespace,
			"kind", alert.ResourceKind,
			"name", alert.ResourceName,
			"reason", alert.Reason,
		)

		// Run diagnosis then send single alert with results
		go func() {
			diagCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()

			diagnosis, err := troubleshooter.Diagnose(diagCtx, agent.DiagnosisRequest{
				Cluster:      cfg.ClusterName,
				Namespace:    ev.Regarding.Namespace,
				ResourceKind: ev.Regarding.Kind,
				ResourceName: ev.Regarding.Name,
				Reason:       ev.Reason,
				EventNote:    ev.Note,
			})
			if err != nil {
				slog.Error("diagnosis failed", "error", err)
				diagnosis = fmt.Sprintf("Diagnosis failed: %v", err)
			}

			alert.Diagnosis = diagnosis
			if err := notify.Send(diagCtx, alert); err != nil {
				slog.Error("failed to send alert", "error", err)
			}
		}()
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	// Periodic pruning of stale counter/dedup entries
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				window.Prune()
				dedup.Prune()
			}
		}
	}()

	w := watcher.NewEventWatcher(kubeClient, cfg.Namespace, filter, handler)
	slog.Info("starting k8s-pager",
		"cluster", cfg.ClusterName,
		"namespace", cfg.Namespace,
		"threshold", cfg.Threshold,
		"window", cfg.WindowSize,
		"cooldown", cfg.Cooldown,
		"reasons", cfg.EventReasons,
	)

	if err := w.Run(ctx); err != nil && ctx.Err() == nil {
		slog.Error("watcher failed", "error", err)
		os.Exit(1)
	}

	slog.Info("shutting down")
}

func buildKubeClients(cfg Config) (kubernetes.Interface, dynamic.Interface, error) {
	var restCfg *rest.Config
	var err error

	if cfg.Kubeconfig != "" {
		restCfg, err = clientcmd.BuildConfigFromFlags("", cfg.Kubeconfig)
	} else {
		restCfg, err = rest.InClusterConfig()
	}
	if err != nil {
		return nil, nil, fmt.Errorf("build kube config: %w", err)
	}

	client, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return nil, nil, fmt.Errorf("create kube client: %w", err)
	}

	dynClient, err := dynamic.NewForConfig(restCfg)
	if err != nil {
		return nil, nil, fmt.Errorf("create dynamic client: %w", err)
	}

	return client, dynClient, nil
}

func clusterNameFromKubeconfig(kubeconfig string) string {
	rules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig}
	config, err := rules.Load()
	if err != nil || config.CurrentContext == "" {
		return "unknown"
	}
	return config.CurrentContext
}

func buildLLMClient(cfg Config, tools []agent.Tool) (agent.LLMClient, error) {
	switch cfg.LLMProvider {
	case "anthropic":
		return agent.NewAnthropicClient(cfg.LLMAPIKey, cfg.LLMModel, tools), nil
	case "openai":
		return agent.NewOpenAIClient(cfg.LLMAPIKey, cfg.LLMModel, cfg.LLMBaseURL, tools), nil
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s (use 'anthropic' or 'openai')", cfg.LLMProvider)
	}
}
