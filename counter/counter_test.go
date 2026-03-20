package counter

import (
	"testing"
	"time"
)

func TestSlidingWindow_ThresholdReached(t *testing.T) {
	w := NewSlidingWindow(10*time.Minute, 3)
	now := time.Now()
	key := Key("ns/Pod/my-pod/BackOff")

	if w.Record(key, now) {
		t.Fatal("should not trigger on first event")
	}
	if w.Record(key, now.Add(1*time.Minute)) {
		t.Fatal("should not trigger on second event")
	}
	if !w.Record(key, now.Add(2*time.Minute)) {
		t.Fatal("should trigger on third event (threshold=3)")
	}
}

func TestSlidingWindow_EventsOutsideWindow(t *testing.T) {
	w := NewSlidingWindow(5*time.Minute, 3)
	now := time.Now()
	key := Key("ns/Pod/my-pod/BackOff")

	w.Record(key, now)
	w.Record(key, now.Add(1*time.Minute))
	// Third event is outside the window relative to the first
	if w.Record(key, now.Add(10*time.Minute)) {
		t.Fatal("should not trigger when old events fall outside window")
	}
}

func TestSlidingWindow_DifferentKeys(t *testing.T) {
	w := NewSlidingWindow(10*time.Minute, 2)
	now := time.Now()

	w.Record(Key("a"), now)
	w.Record(Key("b"), now)

	if w.Record(Key("a"), now.Add(1*time.Minute)) != true {
		t.Fatal("key 'a' should reach threshold independently")
	}
	if w.Record(Key("b"), now.Add(1*time.Minute)) != true {
		t.Fatal("key 'b' should reach threshold independently")
	}
}

func TestSlidingWindow_Prune(t *testing.T) {
	w := NewSlidingWindow(1*time.Millisecond, 2)
	now := time.Now()
	key := Key("ns/Pod/my-pod/BackOff")

	w.Record(key, now)
	time.Sleep(5 * time.Millisecond)
	w.Prune()

	// After prune, old entries should be gone
	if w.Record(key, time.Now()) {
		t.Fatal("should not trigger after prune removed old entries")
	}
}

func TestDeduplicator_ShouldNotify(t *testing.T) {
	d := NewDeduplicator(100 * time.Millisecond)
	key := Key("ns/Pod/my-pod/BackOff")

	if !d.ShouldNotify(key) {
		t.Fatal("first notification should be allowed")
	}
	if d.ShouldNotify(key) {
		t.Fatal("second notification within cooldown should be suppressed")
	}

	time.Sleep(150 * time.Millisecond)

	if !d.ShouldNotify(key) {
		t.Fatal("notification after cooldown should be allowed")
	}
}

func TestDeduplicator_DifferentKeys(t *testing.T) {
	d := NewDeduplicator(1 * time.Hour)

	if !d.ShouldNotify(Key("a")) {
		t.Fatal("key 'a' first notification should be allowed")
	}
	if !d.ShouldNotify(Key("b")) {
		t.Fatal("key 'b' should be independent from key 'a'")
	}
}

func TestDeduplicator_Prune(t *testing.T) {
	d := NewDeduplicator(1 * time.Millisecond)
	key := Key("ns/Pod/my-pod/BackOff")

	d.ShouldNotify(key)
	time.Sleep(5 * time.Millisecond)
	d.Prune()

	if !d.ShouldNotify(key) {
		t.Fatal("should allow notification after prune removed expired entry")
	}
}
