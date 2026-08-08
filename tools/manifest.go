package tools

import (
	"context"
	"encoding/json"
)

// Capability is one action within a Manifest — the leaf-level thing that
// actually runs. Same shape as Tool, minus EndsSession (doesn't apply to a
// leaf action) and minus a globally-unique Name (scoped within its own
// Manifest, not the whole Registry). Watchdog carries the same meaning as
// Tool.Watchdog — a multi-capability integration can mark just the one
// capability that creates watchdog-monitored state, alongside other
// capabilities that don't.
type Capability struct {
	Name        string
	Description string
	Parameters  json.RawMessage
	Watchdog    bool
	Handler     func(ctx context.Context, arguments string) (string, error)
}

// Manifest groups related capabilities under one integration — e.g. a
// "gmail" manifest might hold summarize_emails, schedule_a_meeting,
// send_email. Manifests aren't registered as individual realtime tools —
// a Router resolves (manifest, capability) from a plain-language request
// and calls the matching Capability's Handler directly. That's what keeps
// a realtime session's own tool list small and fixed-size no matter how
// many manifests or capabilities exist — see Router.
type Manifest struct {
	Name         string
	Description  string
	Capabilities []Capability
}
