package watcher

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	eventsv1 "k8s.io/api/events/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

type EventHandler func(ctx context.Context, ev *eventsv1.Event)

type EventWatcher struct {
	client    kubernetes.Interface
	namespace string
	filter    *ReasonFilter
	handler   EventHandler
	startTime time.Time
}

func NewEventWatcher(client kubernetes.Interface, namespace string, filter *ReasonFilter, handler EventHandler) *EventWatcher {
	return &EventWatcher{
		client:    client,
		namespace: namespace,
		filter:    filter,
		handler:   handler,
		startTime: time.Now(),
	}
}

func (w *EventWatcher) Run(ctx context.Context) error {
	for {
		if err := w.watch(ctx); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			slog.Error("watch failed, reconnecting", "error", err)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(5 * time.Second):
			}
		}
	}
}

func (w *EventWatcher) watch(ctx context.Context) error {
	watcher, err := w.client.EventsV1().Events(w.namespace).Watch(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("starting watch: %w", err)
	}
	defer watcher.Stop()

	slog.Info("watching events", "namespace", w.namespaceLabel())

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-watcher.ResultChan():
			if !ok {
				return fmt.Errorf("watch channel closed")
			}
			if event.Type != watch.Added && event.Type != watch.Modified {
				continue
			}
			ev, ok := event.Object.(*eventsv1.Event)
			if !ok {
				continue
			}
			slog.Debug("event received",
				"reason", ev.Reason,
				"type", ev.Type,
				"kind", ev.Regarding.Kind,
				"namespace", ev.Regarding.Namespace,
				"name", ev.Regarding.Name,
				"note", ev.Note,
			)
			if w.isStale(ev) {
				slog.Debug("event skipped (stale)", "reason", ev.Reason, "name", ev.Regarding.Name)
				continue
			}
			if !w.filter.Matches(ev) {
				slog.Debug("event skipped (filtered)", "reason", ev.Reason, "type", ev.Type, "name", ev.Regarding.Name)
				continue
			}
			slog.Info("event matched", "reason", ev.Reason, "kind", ev.Regarding.Kind, "namespace", ev.Regarding.Namespace, "name", ev.Regarding.Name)
			w.handler(ctx, ev)
		}
	}
}

func (w *EventWatcher) isStale(ev *eventsv1.Event) bool {
	var eventTime time.Time

	if ev.Series != nil {
		eventTime = ev.Series.LastObservedTime.Time
	} else if !ev.EventTime.IsZero() {
		eventTime = ev.EventTime.Time
	} else {
		eventTime = ev.CreationTimestamp.Time
	}

	return eventTime.Before(w.startTime)
}

func (w *EventWatcher) namespaceLabel() string {
	if w.namespace == "" {
		return "all"
	}
	return w.namespace
}
