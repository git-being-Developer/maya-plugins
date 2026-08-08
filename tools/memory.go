package tools

// Memory is a tool's own isolated, tier-locked key-value store — not
// Maya's shared/general memory (name, birthday, preferences, and the rest
// of what a conversation should always have in context), which stays
// internal to Maya and deliberately isn't part of this contract. This is
// only for a tool's own state: a Splitwise tool's last-used group, a
// payment tool's account id.
//
// Entries are tagged at write time and locked to that tier on read: a key
// written via SetPrivate can only be read back via GetPrivate, and
// Set/Get only ever see the tool-visible tier. A tier mixup returns
// not-found, not the value — so a mistake fails safe (nothing) instead of
// leaking a private value into whatever a Handler is about to return to
// the model. Nothing in Registry calls GetPrivate; only a tool's own
// Handler code ever sees a private value.
//
// A Handler that needs memory should take one as a constructor parameter
// rather than reaching for a package-level store — that's what makes it
// testable without Maya's own (encrypted, disk-backed) implementation:
// tests can pass an in-memory fake instead. See the memorytest package.
type Memory interface {
	// Set/Get: tool-visible. A Handler may read these back and choose to
	// include them in its reply to the model.
	Set(key, value string) error
	Get(key string) (string, bool)

	// SetPrivate/GetPrivate: never meant to reach the model. Only the
	// owning tool's own Handler code should call GetPrivate.
	SetPrivate(key, value string) error
	GetPrivate(key string) (string, bool)

	Delete(key string) error
	List() map[string]Entry
}

// Entry is one stored value plus which tier it belongs to.
type Entry struct {
	Value   string `json:"value"`
	Private bool   `json:"private"`
}
