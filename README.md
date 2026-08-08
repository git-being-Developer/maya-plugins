# maya-plugins

Tool-calling contract and (eventually) contributed tools for
[Maya](https://github.com/git-being-Developer/maya), a personal voice
assistant. Extracted out of Maya's own repo so it can be worked on and
contributed to independently — see [`tools/README.md`](tools/README.md)
for the contract itself, how to give a tool its own isolated/private
memory, and where a new tool's code should live.

## Status

This currently mirrors the `tools/` package inside the main Maya repo —
they're not wired together yet. The plan is a CI step that picks up merged
PRs here and syncs them back into Maya on a release cadence; that's a later
phase, not built yet. For now, treat this as the contract's source of
truth going forward, with Maya's own copy following manually until that
sync exists.

## License

Not chosen yet.
