package tools

import (
	"context"
	"testing"
)

func TestRegistryCallDispatchesToHandler(t *testing.T) {
	registry := NewRegistry()
	registry.Register(Tool{
		Name:        "echo",
		Description: "echoes arguments",
		Handler: func(_ context.Context, arguments string) (string, error) {
			return "got:" + arguments, nil
		},
	})

	result, err := registry.Call(context.Background(), "echo", `{"a":1}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != `got:{"a":1}` {
		t.Fatalf("unexpected result: %q", result)
	}
}

func TestRegistryCallUnknownToolErrors(t *testing.T) {
	registry := NewRegistry()
	if _, err := registry.Call(context.Background(), "missing", ""); err == nil {
		t.Fatal("expected error for unknown tool")
	}
}

func TestDefinitionsIncludesRegisteredTools(t *testing.T) {
	registry := NewRegistry()
	registry.Register(Tool{Name: "get_time", Description: "current time"})

	defs := registry.Definitions()
	if len(defs) != 1 {
		t.Fatalf("expected 1 definition, got %d", len(defs))
	}
	if defs[0]["name"] != "get_time" {
		t.Fatalf("unexpected definition: %#v", defs[0])
	}
	if defs[0]["type"] != "function" {
		t.Fatalf("unexpected type: %#v", defs[0])
	}
}

func TestEndsSession(t *testing.T) {
	registry := NewRegistry()
	registry.Register(Tool{Name: "get_time", Description: "current time"})
	registry.Register(Tool{Name: "end_conversation", Description: "stop", EndsSession: true})

	if registry.EndsSession("get_time") {
		t.Fatal("get_time should not end the session")
	}
	if !registry.EndsSession("end_conversation") {
		t.Fatal("end_conversation should end the session")
	}
	if registry.EndsSession("unknown_tool") {
		t.Fatal("unknown tool should not end the session")
	}
}
