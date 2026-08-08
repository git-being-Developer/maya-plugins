package tools

import (
	"context"
	"sync"
)

// DisplayRequest is something a Handler wants shown on the dashboard
// alongside its spoken reply. Deliberately just a title and a markdown
// body — the contract has no opinion on what a tool shows (a list, a
// table, an image, plain text), so it never needs new fields when the
// next tool wants to show something shaped differently than the last
// one. The dashboard has exactly one rendering path for all of it.
type DisplayRequest struct {
	Title string
	Body  string // markdown: headers, bold/italic, bullet lists, images, links
}

type displayContextKey struct{}

// DisplaySink collects what a Handler does with the display during one
// Call — the host application creates one via WithDisplay before
// invoking a Handler, and reads it back via Take once the Handler
// returns.
type DisplaySink struct {
	mu             sync.Mutex
	req            *DisplayRequest
	closeRequested bool
}

// WithDisplay returns ctx augmented with a fresh DisplaySink a Handler
// can push to via PushDisplay or CloseDisplay, plus the sink itself so
// the caller can read back what happened once the Handler returns.
func WithDisplay(ctx context.Context) (context.Context, *DisplaySink) {
	sink := &DisplaySink{}
	return context.WithValue(ctx, displayContextKey{}, sink), sink
}

// PushDisplay shows something on the dashboard alongside the Handler's
// spoken reply. No-op if ctx wasn't set up via WithDisplay (e.g. a
// Handler invoked directly in a test that doesn't care about display).
func PushDisplay(ctx context.Context, req DisplayRequest) {
	if sink, ok := ctx.Value(displayContextKey{}).(*DisplaySink); ok {
		sink.mu.Lock()
		sink.req = &req
		sink.closeRequested = false
		sink.mu.Unlock()
	}
}

// CloseDisplay dismisses whatever's currently shown — the explicit,
// conversation-driven way to close it (see the close_display tool),
// mirroring how end_conversation explicitly closes a session rather than
// relying only on a timeout.
func CloseDisplay(ctx context.Context) {
	if sink, ok := ctx.Value(displayContextKey{}).(*DisplaySink); ok {
		sink.mu.Lock()
		sink.req = nil
		sink.closeRequested = true
		sink.mu.Unlock()
	}
}

// Take returns whatever happened during the call: show reports whether
// req is a DisplayRequest that should now be shown; closeRequested
// reports whether CloseDisplay was called instead. Both false means
// neither happened.
func (s *DisplaySink) Take() (req DisplayRequest, show bool, closeRequested bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.req != nil {
		return *s.req, true, false
	}
	if s.closeRequested {
		return DisplayRequest{}, false, true
	}
	return DisplayRequest{}, false, false
}
