package notifier

import (
	"context"
	"log/slog"
)

// LogNotifier writes alerts to structured logs instead of sending them externally.
type LogNotifier struct{}

func NewLogNotifier() *LogNotifier {
	return &LogNotifier{}
}

func (l *LogNotifier) Send(_ context.Context, alert Alert) error {
	slog.Info("ALERT",
		"cluster", alert.Cluster,
		"namespace", alert.Namespace,
		"kind", alert.ResourceKind,
		"name", alert.ResourceName,
		"reason", alert.Reason,
		"count", alert.Count,
		"window", alert.Window,
		"message", alert.Message,
		"diagnosis", alert.Diagnosis,
	)
	return nil
}
