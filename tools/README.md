# tools

The pluggable tool-calling contract [Maya](https://github.com/git-being-Developer/maya)'s
voice sessions run tool calls through: a name, a description, a JSON schema
for arguments, and a Go function. It has no dependency on anything
Maya-specific — `Registry`/`Tool` are the whole package, deliberately kept
that way.

```go
registry := tools.NewRegistry()
registry.Register(tools.Tool{
    Name:        "get_time",
    Description: "Get the current local day, date, and time.",
    Handler: func(ctx context.Context, arguments string) (string, error) {
        return time.Now().Format(time.RFC1123), nil
    },
})
```

`Registry.Definitions()` shapes the registered tools for an OpenAI Realtime
`session.update` event; `Registry.Call` dispatches a function-call event to
the matching handler by name.

Not done yet: choosing a license, and designing how a tool built here gets
loaded into a *running* Maya instance rather than compiled in.

## Where a new tool's code lives

Not inside this `tools/` folder — that's the contract itself (`Tool`,
`Registry`, `Memory`, `Entry`), kept small and stable on purpose. A new tool
gets its own **top-level folder** in this repo, e.g. `weather/`, `upi/`,
named after the tool:

- **A small, self-contained tool** — one file, e.g. `weather/weather.go`,
  exposing a `New(...) tools.Tool` (or `Register(*tools.Registry, ...)`)
  function.
- **Anything non-trivial** — its own state, its own tests, more than one
  handler — is still just its own folder, but likely more than one file
  inside it (e.g. `upi/upi.go`, `upi/upi_test.go`).

Either way it imports this package (`github.com/git-being-Developer/maya-plugins/tools`)
for `Tool`/`Registry`/`Memory`, and nothing else in this repo needs to
change to add it.

**Getting it into Maya itself is a separate, not-yet-automated step.**
Nothing in Maya currently depends on this repo — the plan is a CI job that
picks up merged PRs here and syncs them into Maya on a release cadence, but
that's a later phase. Until it exists, using a tool built here means Maya
adding this repo as a `go.mod` dependency and importing it explicitly.

## Giving a tool its own memory

Maya's shared/general memory (name, birthday, preferences, reminders —
what a conversation should always have in context or can look up by
category) stays internal to Maya and deliberately isn't exposed here. This
package only exposes **tool-scoped** memory, via the `Memory` interface:

```go
type Memory interface {
    Set(key, value string) error
    Get(key string) (string, bool)
    SetPrivate(key, value string) error
    GetPrivate(key string) (string, bool)
    Delete(key string) error
    List() map[string]Entry
}
```

`Set`/`Get` and `SetPrivate`/`GetPrivate` are tier-locked in both
directions: a key written via `SetPrivate` can only be read back via
`GetPrivate` — calling `Get` on it returns not-found, not the value. A
mixup fails safe (nothing) instead of leaking a secret into whatever a
`Handler` is about to hand back to the model. Nothing in `Registry` ever
calls `GetPrivate` — the owning tool's own `Handler` is the only code that
can touch a private value. What that `Handler` then does with it is still
on the tool author, same as in any framework, but nothing in this contract
leaks it.

This repo has no concrete, production `Memory` implementation — Maya's own
is encrypted and disk-backed, and lives inside Maya itself, not here. A
tool should take a `Memory` as a **constructor parameter** rather than
reaching for a package-level store, so it works with whatever
implementation is handed to it and — critically — so it's actually
testable in this repo, without Maya:

```go
// upi/upi.go
package upi

import (
    "context"
    "fmt"

    "github.com/git-being-Developer/maya-plugins/tools"
)

func New(mem tools.Memory) tools.Tool {
    return tools.Tool{
        Name:        "pay_upi",
        Description: "Pay a saved contact via UPI.",
        Handler: func(ctx context.Context, arguments string) (string, error) {
            // Never returned to the model — GetPrivate values only exist
            // inside this Handler.
            upiID, ok := mem.GetPrivate("upi_id")
            if !ok {
                return "No UPI ID is set up yet.", nil
            }

            // ... parse arguments, make the payment using upiID ...
            payee := "Landlord" // from arguments, in a real tool

            // Fine to surface — a tool-visible fact the Handler chose to
            // echo back, not something injected into every conversation.
            _ = mem.Set("last_payee", payee)
            return fmt.Sprintf("Paid %s.", payee), nil
        },
    }
}
```

```go
// upi/upi_test.go
package upi

import (
    "context"
    "testing"

    "github.com/git-being-Developer/maya-plugins/tools/memorytest"
)

func TestPayUPI(t *testing.T) {
    mem := memorytest.New()
    mem.SetPrivate("upi_id", "choco@upi")

    tool := New(mem)
    result, err := tool.Handler(context.Background(), `{}`)
    if err != nil {
        t.Fatalf("Handler: %v", err)
    }
    if result != "Paid Landlord." {
        t.Fatalf("got %q", result)
    }
    if got, _ := mem.Get("last_payee"); got != "Landlord" {
        t.Fatalf("last_payee = %q, want \"Landlord\"", got)
    }
}
```

`memorytest.New()` is an in-memory, non-persistent `Memory` — same
tier-locking behavior as any real implementation, nothing written to disk.
It's what makes the test above possible without Maya's own store at all:
write the tool once against the `Memory` interface, test it here with
`memorytest`, and Maya wires in its real encrypted store when the tool
actually gets loaded into a running instance.
