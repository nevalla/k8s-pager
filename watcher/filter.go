package watcher

import (
	"strings"

	eventsv1 "k8s.io/api/events/v1"
)

// ReasonFilter determines whether a Kubernetes event matches the configured reasons.
type ReasonFilter struct {
	reasons map[string]struct{}
}

func NewReasonFilter(reasons []string) *ReasonFilter {
	m := make(map[string]struct{}, len(reasons))
	for _, r := range reasons {
		m[r] = struct{}{}
	}
	return &ReasonFilter{reasons: m}
}

func (f *ReasonFilter) Matches(ev *eventsv1.Event) bool {
	_, ok := f.reasons[ev.Reason]
	if !ok {
		return false
	}
	// For Unhealthy events, ensure it's a probe failure we care about
	if ev.Reason == "Unhealthy" {
		note := strings.ToLower(ev.Note)
		return strings.Contains(note, "liveness") ||
			strings.Contains(note, "readiness") ||
			strings.Contains(note, "startup")
	}
	return true
}
