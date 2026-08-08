// Package tools is the pluggable tool-calling contract Maya's voice sessions
// run on: a name, a description, a JSON schema for arguments, and a Go
// function. It has no dependency on anything Maya-specific — anyone building
// a Realtime-API-backed assistant can use this exact shape to let a model
// invoke server-side actions mid-conversation.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// Tool is a server-side action the model can invoke mid-conversation.
// EndsSession marks a tool (like an end-the-call action) whose call should
// close the voice session once its resulting reply finishes playing.
//
// Watchdog marks that calling this tool creates state a Watchdog
// implementation elsewhere periodically checks for something to
// proactively announce (a reminder's due time, say) — internal routing
// metadata only, never part of what Definitions() shows the model. A
// Watchdog: true tool is expected to have a corresponding Watchdog
// registered with an Announcer; Go can't enforce that pairing at compile
// time, so it's a documented convention, not a guarantee.
type Tool struct {
	Name        string
	Description string
	Parameters  json.RawMessage
	EndsSession bool
	Watchdog    bool
	Handler     func(ctx context.Context, arguments string) (string, error)
}

// Registry dispatches function calls to registered handlers by name.
type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

func (r *Registry) Register(tool Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[tool.Name] = tool
}

func (r *Registry) Call(ctx context.Context, name, arguments string) (string, error) {
	r.mu.RLock()
	tool, ok := r.tools[name]
	r.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("unknown tool %q", name)
	}
	return tool.Handler(ctx, arguments)
}

// EndsSession reports whether calling the named tool should close the
// voice session after its reply finishes. False for unknown tool names.
func (r *Registry) EndsSession(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.tools[name].EndsSession
}

// Definitions returns the tool list shaped for the Realtime session.update event.
func (r *Registry) Definitions() []map[string]any {
	r.mu.RLock()
	defer r.mu.RUnlock()

	defs := make([]map[string]any, 0, len(r.tools))
	for _, tool := range r.tools {
		var params any = map[string]any{"type": "object", "properties": map[string]any{}}
		if len(tool.Parameters) > 0 {
			params = tool.Parameters
		}
		defs = append(defs, map[string]any{
			"type":        "function",
			"name":        tool.Name,
			"description": tool.Description,
			"parameters":  params,
		})
	}
	return defs
}
