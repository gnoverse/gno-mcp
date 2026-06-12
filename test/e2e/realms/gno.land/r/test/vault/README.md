# r/test/vault

Per-user note store used by the gnomcp e2e harness.

Exports:
- `Set(user address, note string)` — stores a note for a user.
- `Clear(user address)` — removes a user's note (owner-gated).
- `Get(user address) string` — returns the stored note, or "".
- `Render(path string) string` — markdown count of stored notes.
