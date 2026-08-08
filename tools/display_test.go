package tools

import (
	"context"
	"testing"
)

func TestDisplaySinkPushThenTakeReportsShow(t *testing.T) {
	ctx, sink := WithDisplay(context.Background())
	PushDisplay(ctx, DisplayRequest{Title: "Sweet options", Body: "- Gulab Jamun"})

	req, show, closeRequested := sink.Take()
	if !show {
		t.Fatal("expected show=true")
	}
	if closeRequested {
		t.Fatal("expected closeRequested=false")
	}
	if req.Title != "Sweet options" || req.Body != "- Gulab Jamun" {
		t.Fatalf("unexpected req: %#v", req)
	}
}

func TestDisplaySinkCloseThenTakeReportsClose(t *testing.T) {
	ctx, sink := WithDisplay(context.Background())
	CloseDisplay(ctx)

	_, show, closeRequested := sink.Take()
	if show {
		t.Fatal("expected show=false")
	}
	if !closeRequested {
		t.Fatal("expected closeRequested=true")
	}
}

func TestDisplaySinkNeitherCalledReportsNeither(t *testing.T) {
	_, sink := WithDisplay(context.Background())

	_, show, closeRequested := sink.Take()
	if show || closeRequested {
		t.Fatalf("expected neither, got show=%v closeRequested=%v", show, closeRequested)
	}
}

func TestDisplaySinkCloseAfterPushOverridesToClose(t *testing.T) {
	// A Handler that pushes something and then, later in the same call,
	// decides to close instead — the last action wins.
	ctx, sink := WithDisplay(context.Background())
	PushDisplay(ctx, DisplayRequest{Title: "x", Body: "y"})
	CloseDisplay(ctx)

	_, show, closeRequested := sink.Take()
	if show {
		t.Fatal("expected show=false after a later CloseDisplay")
	}
	if !closeRequested {
		t.Fatal("expected closeRequested=true")
	}
}

func TestPushDisplayWithoutWithDisplayIsNoOp(t *testing.T) {
	// A Handler invoked directly (e.g. in another package's test) against
	// a plain context — must not panic, must simply do nothing.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("PushDisplay panicked on a plain context: %v", r)
		}
	}()
	PushDisplay(context.Background(), DisplayRequest{Title: "x"})
	CloseDisplay(context.Background())
}
