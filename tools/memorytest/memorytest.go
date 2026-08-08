// Package memorytest is an in-memory tools.Memory for tests — nothing
// persists to disk and nothing is encrypted, so it's for exercising a
// tool's Handler logic in a `go test`, not for production use. Maya's own
// production implementation is a separate, encrypted, disk-backed store
// internal to Maya; this package has no relationship to it beyond
// implementing the same tools.Memory interface, so a tool written against
// tools.Memory can be tested here and later wired to the real thing with
// no code changes.
package memorytest

import (
	"sync"

	"github.com/git-being-Developer/maya-plugins/tools"
)

// Memory is a tools.Memory backed by a plain map, safe for concurrent use.
// Tier-locking semantics match the production store: a key written via
// SetPrivate can only be read back via GetPrivate, and vice versa.
type Memory struct {
	mu      sync.Mutex
	entries map[string]tools.Entry
}

var _ tools.Memory = (*Memory)(nil)

// New returns an empty Memory.
func New() *Memory {
	return &Memory{entries: make(map[string]tools.Entry)}
}

func (m *Memory) Set(key, value string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries[key] = tools.Entry{Value: value, Private: false}
	return nil
}

func (m *Memory) Get(key string) (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	entry, ok := m.entries[key]
	if !ok || entry.Private {
		return "", false
	}
	return entry.Value, true
}

func (m *Memory) SetPrivate(key, value string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries[key] = tools.Entry{Value: value, Private: true}
	return nil
}

func (m *Memory) GetPrivate(key string) (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	entry, ok := m.entries[key]
	if !ok || !entry.Private {
		return "", false
	}
	return entry.Value, true
}

func (m *Memory) Delete(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.entries, key)
	return nil
}

func (m *Memory) List() map[string]tools.Entry {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make(map[string]tools.Entry, len(m.entries))
	for k, v := range m.entries {
		out[k] = v
	}
	return out
}
