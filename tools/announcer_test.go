package tools

import (
	"context"
	"sync"
	"testing"
	"time"
)

// fakeWatchdog returns whatever's queued via push, once per Check call —
// mirrors the real contract (Check only returns items ready to announce
// right now; a fake driving that decision itself, not the Announcer).
type fakeWatchdog struct {
	mu      sync.Mutex
	queued  []AnnouncementRequest
	checked int
}

func (w *fakeWatchdog) push(req AnnouncementRequest) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.queued = append(w.queued, req)
}

func (w *fakeWatchdog) Check(_ context.Context) ([]AnnouncementRequest, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.checked++
	out := w.queued
	w.queued = nil
	return out, nil
}

func TestAnnouncerDeliversDueRequest(t *testing.T) {
	wd := &fakeWatchdog{}
	wd.push(AnnouncementRequest{AnnounceAt: time.Now(), Text: "pay the electricity bill"})

	announcer := NewAnnouncer(10 * time.Millisecond, wd)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go announcer.Run(ctx)

	select {
	case req := <-announcer.Subscribe():
		if req.Text != "pay the electricity bill" {
			t.Fatalf("got %q", req.Text)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for announcement")
	}
}

func TestAnnouncerFansInMultipleWatchdogs(t *testing.T) {
	wd1 := &fakeWatchdog{}
	wd2 := &fakeWatchdog{}
	wd1.push(AnnouncementRequest{AnnounceAt: time.Now(), Text: "from watchdog one"})
	wd2.push(AnnouncementRequest{AnnounceAt: time.Now(), Text: "from watchdog two"})

	announcer := NewAnnouncer(10*time.Millisecond, wd1, wd2)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go announcer.Run(ctx)

	seen := map[string]bool{}
	deadline := time.After(2 * time.Second)
	for len(seen) < 2 {
		select {
		case req := <-announcer.Subscribe():
			seen[req.Text] = true
		case <-deadline:
			t.Fatalf("timed out, only saw %v", seen)
		}
	}
	if !seen["from watchdog one"] || !seen["from watchdog two"] {
		t.Fatalf("expected both watchdogs' announcements, got %v", seen)
	}
}

func TestAnnouncerDoesNotDeliverWithNothingQueued(t *testing.T) {
	wd := &fakeWatchdog{}
	announcer := NewAnnouncer(10*time.Millisecond, wd)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go announcer.Run(ctx)

	select {
	case req := <-announcer.Subscribe():
		t.Fatalf("expected no announcement, got %v", req)
	case <-time.After(200 * time.Millisecond):
		// expected: nothing delivered
	}

	wd.mu.Lock()
	checked := wd.checked
	wd.mu.Unlock()
	if checked == 0 {
		t.Fatal("expected Check to have been called at least once")
	}
}

func TestAnnouncerStopsOnContextCancel(t *testing.T) {
	wd := &fakeWatchdog{}
	announcer := NewAnnouncer(10*time.Millisecond, wd)
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		announcer.Run(ctx)
		close(done)
	}()

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after context cancellation")
	}
}
