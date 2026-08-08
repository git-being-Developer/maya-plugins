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

## `Tool` vs `Manifest`: one action, or several related ones

A realtime voice session sees every registered `Tool` as its own top-level
function definition. That's fine for a handful — but an integration like
Gmail naturally has several actions (summarize emails, send an email,
schedule a meeting), and registering each as its own top-level `Tool`
means the model has to pick the right one out of an ever-growing flat
list as more integrations get added. Past a certain point that hurts
selection accuracy, not just payload size.

If your tool is genuinely **one action** — `weather/weather.go`, one
`Tool` — nothing changes, use `Registry.Register` as above.

If it's **several related actions under one integration**, group them as
a `Manifest` instead:

```go
type Manifest struct {
    Name         string
    Description  string
    Capabilities []Capability // Name, Description, Parameters, Handler — same shape as Tool
}
```

A `Manifest` is never registered directly on a `Registry` — it's handed to
a `Router`, which fronts *every* manifest behind a single dispatcher tool
that the model actually sees. When the model calls the dispatcher with a
plain-language request, the `Router` resolves it in two classification
calls — "which manifest?" then "which capability within it?" — and calls
that `Capability`'s `Handler` directly:

```go
router := tools.NewRouter(classify, gmailManifest, upiManifest, calendarManifest)

registry.Register(tools.Tool{
    Name:        "run_task",
    Description: "Handle anything involving a connected integration — describe the request in plain language.",
    Handler: func(ctx context.Context, arguments string) (string, error) {
        var args struct{ Request string `json:"request"` }
        json.Unmarshal([]byte(arguments), &args)
        return router.Resolve(ctx, args.Request)
    },
})
```

This is what keeps the realtime tool list a small, fixed size no matter
how many manifests or capabilities exist — it grows with *fixed built-ins
+ 1 dispatcher*, never with the number of integrations. `Resolve` returns
a plain "nothing available for that" result (not an error) when nothing
matches confidently at either stage, and short-circuits with zero
classify calls at all when no manifests are registered.

`classify` (type `Classify`) is pluggable — it's just
`func(ctx, request string, options []Option) (string, error)`, one call
per stage. Maya's own default is a single small OpenAI text call
(`internal/realtime/router.go`, not part of this repo), but nothing here
requires OpenAI specifically — a smaller or local model can implement the
same signature.

## Proactive tools: `Watchdog` + `Announcer`

Every `Tool`/`Capability` above is pull-only — the model calls it, it
returns, done. Some things need the other direction: a reminder whose
time has come, a price alert, anything that should reach the user
without them opening a session first. `Tool` and `Capability` both carry
a `Watchdog bool` field for this — internal routing metadata only, never
part of what `Definitions()` shows the model. A `Watchdog: true` tool is
expected to also implement:

```go
type Watchdog interface {
    Check(ctx context.Context) ([]AnnouncementRequest, error)
}
```

`Check` must return **only** items that are already due — not upcoming
ones. `Announcer` is the one shared polling engine every `Watchdog`
registers with, and it's deliberately stateless between polls: a
`Watchdog` is responsible for self-filtering (comparing its own stored
due time against `time.Now()`) and for not reporting the same thing
again once it's been included in a returned batch — e.g. by deleting its
own backing record. Holding a not-yet-due item inside `Announcer` itself
across polls would risk it being reported by the `Watchdog` *and* still
pending in `Announcer` on the same tick, so `Announcer` just doesn't try:

```go
announcer := tools.NewAnnouncer(30*time.Second, remindersTool, priceAlertTool)
go announcer.Run(ctx)
for req := range announcer.Subscribe() {
    // deliver req.Text however the host application reaches the user
}
```

This repo has no host application wired to actually *deliver* an
`AnnouncementRequest` (that's Maya's job — opening a voice session and
having it speak, in Maya's own repo, not here). A `Watchdog`-backed tool
built here should still store its own due-time state the same way as any
other tool state — via `Memory`, e.g. a small JSON blob per entry keyed by
whatever it needs to look itself up later — rather than inventing a
separate storage mechanism just for scheduling.

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
