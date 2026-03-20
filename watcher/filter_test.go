package watcher

import (
	"testing"

	eventsv1 "k8s.io/api/events/v1"
)

func TestReasonFilter_Matches(t *testing.T) {
	f := NewReasonFilter([]string{"BackOff", "Failed", "Unhealthy"})

	tests := []struct {
		name   string
		event  eventsv1.Event
		expect bool
	}{
		{
			name:   "exact match",
			event:  eventsv1.Event{Reason: "BackOff"},
			expect: true,
		},
		{
			name:   "no match",
			event:  eventsv1.Event{Reason: "Scheduled"},
			expect: false,
		},
		{
			name:   "unhealthy with liveness probe",
			event:  eventsv1.Event{Reason: "Unhealthy", Note: "Liveness probe failed: connection refused"},
			expect: true,
		},
		{
			name:   "unhealthy with readiness probe",
			event:  eventsv1.Event{Reason: "Unhealthy", Note: "Readiness probe failed: timeout"},
			expect: true,
		},
		{
			name:   "unhealthy with startup probe",
			event:  eventsv1.Event{Reason: "Unhealthy", Note: "Startup probe failed: exit code 1"},
			expect: true,
		},
		{
			name:   "unhealthy without probe info",
			event:  eventsv1.Event{Reason: "Unhealthy", Note: "some other message"},
			expect: false,
		},
		{
			name:   "empty reason",
			event:  eventsv1.Event{Reason: ""},
			expect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := f.Matches(&tt.event); got != tt.expect {
				t.Errorf("Matches() = %v, want %v", got, tt.expect)
			}
		})
	}
}

func TestReasonFilter_EmptyReasons(t *testing.T) {
	f := NewReasonFilter([]string{})
	ev := &eventsv1.Event{Reason: "BackOff"}

	if f.Matches(ev) {
		t.Fatal("empty filter should not match anything")
	}
}
