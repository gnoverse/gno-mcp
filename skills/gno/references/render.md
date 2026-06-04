# Render() and gnoweb markdown surface

> **Category: rendering / on-chain content.** Update when gnoweb's markdown extension surface or render pipeline changes in master.

## Purpose

Realm `Render(path string) string` output is the dual-audience surface of Gno: humans read it via gnoweb, agents read it via the same `vm/qrender` query. This reference teaches both how to author Render() output that renders cleanly and how to evaluate someone else's Render() for quality and safety.

## Core contract

```go
// Realm-defined. Optional but conventional.
func Render(path string) string
```

- `path` is the URL suffix after the realm pkgPath (`""` for the home page, `"post/42"` for a sub-route). Gnoweb strips leading/trailing slashes before calling.
- Returns markdown, processed by goldmark + gnoweb's extension set on every page load.
- **Not a crossing function** — no `cur realm` parameter. Read-only by convention.
- **Trust posture**: realm-authored content is attacker-controlled from the consumer's perspective. Treat it as untrusted input in both human and agent renderers.

## Pipeline behavior an author needs to know

1. `vm/qrender` invokes the realm's `Render(path)`. Output is markdown.
2. Gnoweb passes that markdown through goldmark with a fixed extension set (six extensions documented below).
3. Extensions transform the markdown into HTML; gnoweb wraps the result and serves.

**No HTML sanitization layer.** Realm-output raw HTML blocks pass through goldmark's HTML-block handler. Text nodes get escaped; raw `<script>` would render verbatim if present — realm code must not output one. Extension attribute escaping is per-extension (forms escape attributes; links copy URLs as-is — realm responsible for safe URLs).

## Markdown extension surface

All six extensions are **globally enabled** by gnoweb's render configuration. Realms cannot opt in or out per page. The image validator is the only configurable knob, set at gnoweb startup.

### `ext_alerts` — collapsible callouts

**In**:

```markdown
> [!NOTE]
> This is a note.
```

**Out**: `<details class="gno-alert gno-alert-note" open><summary>…Note…</summary><div><p>This is a note.</p></div></details>`

**Types**: `NOTE`, `TIP`, `CAUTION`, `WARNING`, `SUCCESS`, `INFO`. Case-sensitive — `[!note]` does NOT match.

**Gotchas**: alert content parses recursively (markdown inside works). Title optional (`> [!NOTE] Custom Title`). Blockquote `>` prefix required on every line.

**Status**: STABLE.

### `ext_columns` — multi-column layouts

**In** (uses HTML-like tags, not pure markdown):

```markdown
<gno-columns>
## Title 1
content 1
<gno-columns-sep />
## Title 2
content 2
</gno-columns>
```

**Out**: `<div class="gno-columns"><div class="gno-column"><h2>Title 1</h2><p>content 1</p></div><div class="gno-column">…</div></div>`

**Gotchas**: `<gno-columns-sep />` must self-close. Malformed/unclosed tags cause parse errors. Nesting `<gno-columns>` blocks is rejected.

**Status**: STABLE.

### `ext_forms` — Markdown → tx submission

The only Markdown → transaction primitive. `<gno-form exec="FuncName">` becomes a wallet-mediated `MsgCall` submission.

**In**:

```markdown
<gno-form exec="CreateThread">
<gno-input name="title" placeholder="Title" required="true" />
<gno-textarea name="body" rows="5" />
</gno-form>
```

**Out**: `<form class="gno-form" method="post" action="/r/{realm}" …>` with inputs and a submit button that the wallet intercepts.

**Inputs**: `<gno-input>` (types: text, email, password, number, tel, radio, checkbox), `<gno-textarea>` (rows clamped 2–10), `<gno-select>`. Attributes include `name` (required), `type`, `placeholder`, `value`, `checked`, `readonly`, `required`, `description`.

**Hard rules** (don't fight these — they're design intent, not bugs):

- **No hidden form fields.** PR #4858 explicitly rejected hidden inputs on transparency grounds: *"users should always see exactly what data will be submitted to the blockchain"* (gfanton). Don't try to slip context via hidden inputs — the renderer strips them and the design is deliberate.
- **Attributes are HTML-escaped.** `name="test<script>"` becomes `name="test&lt;script&gt;"`. Realm-authored attribute values are escaped at render time.
- **Form action is fixed.** `action="/r/{realm}"` is hardcoded; you cannot post a `gno-form` to a different realm.
- **The submit button is disabled by design.** The wallet intercepts; clicking the visible button without a wallet does nothing.

**Gotchas**:
- Invalid input types render as comment errors and revert to text.
- `placeholder` doubles as label text; no placeholder → empty label.
- Realm must implement an exported function matching `exec="FuncName"` to handle the call.
- CSP `form-action 'self'` is set; cross-site form post is blocked (PR #5046).

**Status**: STABLE (author surface). The `<gno-form exec>` surface and its input tags are registered unconditionally alongside the other stable extensions; the PR train that introduced it (#4858 → #4978 → #4974 → #5002 → #5046) landed by early 2026. Caveat: gnoweb as a whole is in Beta, and the exact rendered HTML (`<form class="gno-form" ...>`) is an implementation detail — if downstream tooling parses the literal markup, pin to a gnoweb commit.

### `ext_imgvalidator` — image URL filtering (two-layered)

Two distinct mechanisms work together:

1. **Renderer level**: gnoweb strips `https://` from `<img src>` at render time. The `<img>` tag remains with `src=""` and alt text preserved.
2. **Browser CSP** (HTTP response header): `img-src` allowlist — only specific hosts pass the browser-side check. Allowed hosts (per PR #4058): `gnolang.github.io`, `assets.gnoteam.com`, `sa.gno.services`, imgur, GitHub Pages, IPFS gateways (Cloudflare, ipfs.io).
3. **Data URI filter**: for `data:` URIs, only `data:image/svg+xml;…` is accepted. Other `data:` mime-types stripped.

**Bottom-line authoring guidance**: use SVG data URIs (`data:image/svg+xml;base64,…`) for inline images. `http://` URLs render the tag but are browser-mixed-content-unsafe. `https://` URLs silently fail (renderer strips). Other image hosts will be CSP-blocked by the browser.

**Footgun**: `![alt](https://cdn.example/img.png)` produces `<img src="" alt="alt">` — broken image, no error signal to the realm. This is undocumented to realm authors.

**Status**: STABLE at code level. The image-proxy proposal (#4079) was closed `not_planned` — operators run `go-camo` themselves if they want proxying.

### `ext_link` — link classification

External links get a security marker; internal links don't.

**In**: `[example](https://example.com)`

**Out**: `<a href="https://example.com" rel="noopener nofollow ugc">example <span class="link-external">…icon…</span></a>`

**Behaviors**:
- External (`http://`, `https://`): security icon + `rel="noopener nofollow ugc"`.
- Internal / relative (`/r/docs`, `:board/123`): no icon, normal anchor.
- Gno transaction URLs (`https://host/r/path:func&arg=val`): treated as external; icon appended.

**Status**: STABLE.

### `ext_mentions` — auto-link addresses and emails

**In**: `paid to g1mpkp5lm8lwpm0pym4388836d009zfe4maxlqsq`

**Out**: bech32 address auto-linked to `/u/{address}`.

**Rules**:
- Gno addresses (`g1` prefix, 40-char bech32) → `/u/{addr}` user profile link.
- Email addresses → `mailto:` link.
- Word-boundary aware — addresses embedded inside other text are skipped.

**Gotcha**: `@username` is NOT auto-linked. Only bech32 addresses. For username display, use explicit markdown `[@alice](/r/sys/users:alice)`.

**Status**: STABLE.

### Goldmark defaults also on

PR #4501 turned on:
- **TaskList**: `- [x]`, `- [ ]`.
- **Footnote**: `[^1]` reference syntax.

Worth knowing they work without writing custom syntax.

## Path-routing conventions

There is **no enforced routing framework**. Each realm parses `path` itself. Three patterns observed across `examples/`:

| Pattern | Used in | Shape |
|---|---|---|
| **No routing** | `r/gnoland/home`, `r/sys/users`, `r/sys/cla` | Ignores `path`; always renders the same content |
| **`p/nt/mux/v0` router** | `r/gnoland/boards2/v1`, `r/gnoland/coins`, `r/demo/profile`, `r/gnoland/blog` | mux-style segment dispatch |
| **`p/moul/realmpath` parse** | `r/sys/namereg/v1` | single-segment dispatch |

For agents: pick mux for any realm with more than 2 distinct views. Static home + a handful of detail views → static is fine.

Unknown-path handling is **ad-hoc** — no standard 404. Different realms return different shapes: blockquote, custom 404 markdown, empty string, or panic. Pick a convention and stick with it within your realm.

## Audit signals (auditor mode)

When reviewing a realm's `Render()`:

| Pattern | Signal | Action |
|---|---|---|
| `Render()` calling state-mutating methods | RED | Should be read-only — by convention, not by compiler. |
| `<gno-form>` with hidden `<input type="hidden">` | RED | Renderer strips them; design intent is full transparency. Refactor to visible `readonly` inputs. |
| `![alt](https://…)` | YELLOW | Will silently break (renderer strips). Use SVG data URI or self-hosted whitelisted host. |
| Raw `<script>`, `<style>`, `<iframe>` in output | RED | Goldmark renders raw HTML blocks verbatim; gnoweb has no sanitizer. |
| User-submitted strings echoed in markdown without escaping | YELLOW | XSS surface; realm must escape before emitting. |
| Markdown that *needs* an extension to be disabled | RED | Extensions are global — can't be turned off. Rewrite to avoid the conflicting syntax. |
| Render() length unbounded by `path` | YELLOW | Long output is gas-cheap but bandwidth-real. Consider pagination. |
| `<gno-form exec>` whose target function isn't exported | RED | Submission will fail at chain side; user UX degraded. |
| `<gno-form>` posting to a different realm | RED | Action is hardcoded to `/r/{realm}`; can't be overridden. |

## Cross-references

- `interrealm.md` — `Render()` is not a crossing function; no `cur realm`
- `security.md` — XSS / untrusted-content posture for general (non-Render) state echoed in `Render()`
- `patterns.md` — Render() conventions (routing, pagination, error pages)
- `stdlib.md` — `vm/qrender` query surface (chain side)

## Source

Behavior documented here is empirically observed via the per-extension example tables. For unfamiliar edge cases, the gnomcp design exposes `gno_render <pkgPath>` which executes the same pipeline a real gnoweb load uses; query that for exact rendered output rather than guessing.

The exact rendered HTML for every extension is a gnoweb implementation detail (gnoweb is in Beta) — if downstream tooling parses literal markup rather than re-rendering via `gno_render`, pin to a gnoweb commit.
