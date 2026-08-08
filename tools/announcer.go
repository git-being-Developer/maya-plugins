package tools

import (
	"context"
	"time"
)

// AnnouncementRequest is what a Watchdog reports to the shared Announcer
// — a fixed, minimal shape so every watchdog-backed tool speaks the same
// language to the engine, regardless of what it's actually watching.
type AnnouncementRequest struct {
	// AnnounceAt is when this became due — time.Now() for a tool that
	// fires immediately on some condition, or the original due time for
	// something scheduled. Informational; Check is contractually only
	// supposed to return items that are already ready to announce (see
	// Watchdog), so Announcer trusts this rather than re-checking it.
	AnnounceAt time.Time
	// Text is what Maya should say.
	Text string
}

// Watchdog is implemented by anything that needs to proactively reach the
// user, rather than only responding when called — a reminder whose time
// has come, a price alert, anything Maya should bring up unprompted.
// Check runs periodically, driven by the shared Announcer (not by the
// tool itself), and must return only items that are ready to announce
// right now — not upcoming ones, since Announcer has no way to hold a
// future item without risking it being reported (and so delivered)
// twice across polls. A Watchdog is responsible for not reporting the
// same thing again on a later Check — e.g. by deleting its own backing
// record once it's included in a returned batch.
type Watchdog interface {
	Check(ctx context.Context) ([]AnnouncementRequest, error)
}

// Announcer is the single shared engine every Watchdog registers with —
// one poll loop, not one goroutine per tool. Stateless between polls by
// design (see Watchdog) — delivery precision is bounded by interval, an
// accepted trade for a personal reminder system, not a general-purpose
// scheduler.
type Announcer struct {
	interval  time.Duration
	watchdogs []Watchdog
	out       chan AnnouncementRequest
}

// NewAnnouncer builds an Announcer that polls every watchdog on interval
// once Run is called.
func NewAnnouncer(interval time.Duration, watchdogs ...Watchdog) *Announcer {
	return &Announcer{
		interval:  interval,
		watchdogs: watchdogs,
		out:       make(chan AnnouncementRequest),
	}
}

// Run polls every registered Watchdog on the configured interval until
// ctx is done, forwarding everything each Check call returns onto the
// channel Subscribe returns. Blocks — call in its own goroutine.
func (a *Announcer) Run(ctx context.Context) {
	ticker := time.NewTicker(a.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.tick(ctx)
		}
	}
}

func (a *Announcer) tick(ctx context.Context) {
	for _, wd := range a.watchdogs {
		requests, err := wd.Check(ctx)
		if err != nil {
			continue
		}
		for _, req := range requests {
			select {
			case a.out <- req:
			case <-ctx.Done():
				return
			}
		}
	}
}

// Subscribe returns the channel Run delivers AnnouncementRequests on.
func (a *Announcer) Subscribe() <-chan AnnouncementRequest {
	return a.out
}
