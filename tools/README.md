# tools

The pluggable tool-calling contract Maya's voice sessions run on: a name, a
description, a JSON schema for arguments, and a Go function. It has no
dependency on anything Maya-specific — `Registry`/`Tool` are the whole
package, deliberately kept that way.

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

This package is meant to be extractable to its own repository and published
separately — it's kept out of `internal/` specifically so external code (and
eventually community-contributed tools) can import it. Not done yet:
choosing a license, publishing it, and designing how third-party tools get
loaded into a *running* Maya instance rather than compiled in.

## Where a new tool's code lives

`internal/realtime/tools.go` used to register every built-in tool in one
function — fine for a handful, but every new tool meant editing a block
everyone else's tools also lived in. It's now a thin aggregator that just
calls one `register*Tools` function per group:

- **A small, tightly-coupled tool** (a few lines, no state of its own) —
  its own file next to the others, e.g. `internal/realtime/tools_weather.go`,
  registered with one new line in `tools.go`'s `DefaultTools`. This is what
  `get_time`/`check_notifications` (`tools_system.go`), `get_weather`
  (`tools_weather.go`), and `remember`/`recall`/`forget`
  (`tools_memory.go`) do today.
- **Anything non-trivial** — its own state (via `internal/toolmemory`,
  below), its own tests, more than one handler — gets its own package under
  `internal/tools/<name>/` (e.g. `internal/tools/upi/`) instead of a file
  in `internal/realtime`. It exposes a `Register(registry *tools.Registry, ...)`
  function that `DefaultTools` calls, the same shape as the file-based ones.

Either way, a new tool touches its own file/package plus one new line in
`DefaultTools` — not a shared function that keeps growing. `internal/`
only blocks code *outside this repo* from importing it; a contributor
working in a clone/fork of this repo can add either shape freely.

## Giving a tool its own memory

Maya's shared memory (`internal/memory`) is for facts the model should
always have in context, or can look up by category — name, birthday,
preferences, reminders. It's the wrong place for a tool's own state: a
Splitwise tool's last-used group, or a payments tool's account ID. That
belongs to the tool alone, shouldn't bloat every conversation's context, and
in the account-ID case must never reach the model at all.

`internal/toolmemory` covers that. A tool opens its own store where it's
registered and captures it in the `Handler` closure — the same way
`internal/realtime/tools.go` already captures `weatherClient`:

```go
mem, err := toolmemory.Open("upi") // one encrypted file per tool, data/tools/upi.json
if err != nil {
    log.Fatalf("open upi memory: %v", err)
}

registry.Register(tools.Tool{
    Name: "pay_upi",
    Handler: func(ctx context.Context, arguments string) (string, error) {
        // Never returned to the model — GetPrivate/SetPrivate values only
        // exist inside this Handler.
        upiID, ok := mem.GetPrivate("upi_id")
        if !ok {
            return "No UPI ID is set up yet.", nil
        }

        // Fine to surface — a tool-visible fact the Handler chose to echo
        // back, not something injected into every conversation.
        lastPayee, _ := mem.Get("last_payee")

        // ... build and make the payment using upiID ...

        _ = mem.Set("last_payee", payee)
        return fmt.Sprintf("Paid %s.", payee), nil
    },
})
```

`Set`/`Get` and `SetPrivate`/`GetPrivate` are tier-locked in both
directions: a key written via `SetPrivate` can only be read back via
`GetPrivate` — calling `Get` on it returns not-found, not the value. A
mixup fails safe (nothing) instead of leaking a secret into whatever the
`Handler` is about to hand back to the model. Nothing in `tools.Registry` or
the relay ever calls `GetPrivate` — the owning `Handler` is the only code
that can touch a private value. What that `Handler` then does with it is
still on the tool author, same as in any framework, but Maya's own plumbing
has no path that leaks it.
