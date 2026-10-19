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

### Sanitizing untrusted text in Render output

The `path` argument and any user-authored state (titles, descriptions, addresses) are attacker-controlled. Escape them before concatenating into markdown, or an attacker injects links, blockquotes, code fences, and other syntax.

The author-facing sanitizer is `gno.land/p/nt/markdown/sanitize/v0` — it wraps the internal `chain/markdown` escaper natives behind a policy layer:

```go
import "gno.land/p/nt/markdown/sanitize/v0"

func Render(path string) string {
    return "# Echo\n\n" + sanitize.InlineText(path)
}
```

`InlineText` is for inline contexts; the package also exposes `Block`, `Blockquote`, `TableCell`, `URL`, `CodeBlock`, and others for different positions. It is **not idempotent** — sanitize once, at the point of output. The `chain/markdown` stdlib escapers are documented as internal building blocks; prefer the `sanitize/v0` helpers. As with any stdlib/community package, verify it's on your target chain (`gno_read gno.land/p/nt/markdown/sanitize/v0`) before relying on it.

The escaping runs in `chain/markdown`, a native the node binary carries, so it follows the chain's release rather than the deployed package's code. From v1.6.0, `InlineText`, `TableCell`, `Block`, `BlockRich`, `Blockquote` and `BlockquoteRich` escape every `<gno-button` wherever it sits (inside a code span the backslash shows); `Block`, `BlockRich` and the blockquote helpers still let a mid-line `<gno-icon />` render. None of them escapes `$` (§ Math).

## Pipeline behavior an author needs to know

1. `vm/qrender` invokes the realm's `Render(path)`. Output is markdown.
2. Gnoweb passes that markdown through goldmark with a fixed extension set (listed below).
3. Extensions transform the markdown into HTML; gnoweb wraps the result and serves.

**Raw HTML is dropped.** By default gnoweb runs goldmark in safe mode (an operator can turn it off with `-html`, gnodev with `-web-html`): a raw HTML block or inline tag that no extension claims (`<script>`, `<iframe>`, `<b>`, `<img onerror=…>`, a malformed `<gno-…>` tag) renders as `<!-- raw HTML omitted -->`. A `javascript:`, `vbscript:`, `file:` or non-image `data:` link destination renders as an empty `href`. What an attacker who controls part of the markdown keeps is markdown itself: links, images, buttons, forms, mentions and headings that pass for the realm's own. That is why user text goes through a sanitizer.

From gnoweb v1.6.0, a client that sends `Accept: text/markdown` gets the raw `Render()` output instead of the HTML page.

## Markdown extension surface

Every extension is **globally enabled** by gnoweb's render configuration; a realm cannot opt in or out per page. Next to the goldmark defaults below, gnoweb adds alerts, `<gno-columns>`, `<gno-frame>`, `<gno-form>`, `<gno-button />`, `<gno-icon />`, math, `<gno-foreign>`, link classification, mentions and an image filter. Frames, buttons, icons and math need gnoweb v1.6.0 or later; an older gnoweb omits the tags as raw HTML and shows `$…$` as plain text.

The monorepo's `r/docs/markdown` realm documents each one with rendered examples: `examples/quarantined/gno.land/r/docs/markdown/markdown.gno` (quarantined, so not in the onyx or mainnet genesis; read it on GitHub). Take syntax from there; the sections below keep what an author gets wrong.

### `ext_alerts` — collapsible callouts

**In**:

```markdown
> [!NOTE]
> This is a note.
```

**Out**: `<details class="gno-alert gno-alert-note" open><summary>…Note…</summary><div><p>This is a note.</p></div></details>`

**Types**: `NOTE`, `TIP`, `CAUTION`, `WARNING`, `SUCCESS`, `INFO`, in any case: `[!note]` matches too.

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

**Gotchas**: each tag sits alone on its line; `<gno-columns-sep>` works with or without `/>`. A misplaced tag renders as an HTML comment, an unclosed grid closes at the end of the page, and a `<gno-columns>` opened inside a column is dropped.

**Status**: STABLE.

### `<gno-frame>`: bordered blocks and cards

**In**:

```markdown
<gno-frame>
## Build on gno.land

Write your first realm. [Get started](/r/docs/home)
</gno-frame>
```

**Gotchas**: each tag sits alone on its line and takes no attributes. A frame opens only at the top level of the page or of a column, never inside a list, a quote, an alert or directly inside another frame; a misplaced tag renders as an HTML comment. A frame alone in a column renders as a card. A frame may hold a whole `<gno-columns>` grid, which must close before the frame does.

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

### `<gno-button />`: call-to-action links

**In**: `<gno-button href="/r/gnoland/blog" label="Read the blog" variant="outline" />`

**Rules**: self-closing and on one line, or gnoweb omits it as raw HTML. `href` and `label` are required; `variant` takes `outline` and the alert kinds, space-separated. The label is plain text. The `href` follows the link rules below (icons, `rel`, transaction links); a `javascript:`, `vbscript:`, `file:` or `data:` one drops the button. Buttons are inline, so they work in paragraphs, lists, alerts, columns, frames and tables (write a `|` in an attribute as `&#124;`), but not inside `<gno-foreign>`.

**Security**: a button looks like gnoweb's own interface. User text that reaches the page unsanitized can plant one, a transaction link included; the `sanitize/v0` helpers escape it.

### `<gno-icon />`: inline icons

**In**: `## <gno-icon name="rocket" /> Launch`, or `<gno-icon name="check-circle" label="Done" />` for an icon that carries meaning.

**Rules**: self-closing, on one line, with a lowercase `name` listed in `gno.land/pkg/gnoweb/markdown/ICONS.md`. An unknown name or a tag without `/>` renders nothing (the page source holds an HTML comment saying why). `label` is what a screen reader announces; leave it out for a decorative icon, and give it to a link or heading that holds only an icon.

### Math

**In**: `$E = mc^2$` inline; `$$` on the lines before and after a display formula, or a code fence with the info string `math`. `\\(…\\)` and `\\[…\\]` work too: two backslashes in the returned markdown, so `"\\\\(x\\\\)"` in an interpreted Gno string.

**Out**: MathML, built on the server. An expression gnoweb cannot convert shows as its TeX source.

**Gotchas**:
- A `$` opens math only before a non-space, and closes it only after a non-space and before a non-digit. `$5 and $10` stays text, but `$GNOT/$ATOM` renders `GNOT/` as math. Write `\$` for a literal dollar.
- The `sanitize/v0` helpers leave `$` alone, so user text spliced into a page can write math. The output stays escaped, size-bounded MathML; after `sanitize.InlineText`, replace `$` with `\$` when literal dollars matter.
- Display math ends at any line that would start another block (`- `, `> `, `#`). Put `{}` before a formula line that starts that way.
- Math does not render inside `<gno-foreign>`.

### `<gno-foreign>`: sandbox for markdown another realm wrote

Wrap markdown you did not author, such as another realm's `Render` output, with `foreign.Foreign(body)` or `foreign.ForeignWithLabel(label, body)` from `gno.land/p/nt/markdown/foreign/v0`. gnoweb renders the body in a separate sub-renderer: it cannot end the block early or reach the page around it, its links are marked as user content, and forms, buttons and math do not render inside it.

### `ext_imgvalidator`: image sources

Two layers decide whether an image shows:

1. **Renderer**: gnoweb drops every `data:` image except `data:image/svg+xml`. Any other source passes through unchanged.
2. **Browser CSP** (`img-src` response header): the browser loads an image only from gnoweb's own origin, a `data:` URI, or a host in `cspImgHost` in `gno.land/cmd/gnoweb/main.go` (GitHub, imgur, IPFS gateways and a few gno project hosts). Read that list in the release the target runs; it changes between releases.

**Authoring guidance**: use an SVG data URI (`data:image/svg+xml;base64,…`) or a listed host.

**Footgun**: `![alt](https://cdn.example/img.png)` renders an `<img>` the browser refuses to load. The realm gets no error.

**Status**: STABLE at code level. The image-proxy proposal (#4079) was closed `not_planned` — operators run `go-camo` themselves if they want proxying.

### `ext_link`: link classification

gnoweb classifies each link and marks it with an icon and a `rel` attribute.

**Behaviors**:
- External (another host, `//host`, and from gnoweb v1.6.0 `https:///host`): external icon and `rel="noopener nofollow ugc"`.
- Another package on the same site (`/r/gnoland/blog`): cross-package icon. A link within the same package gets none.
- A user page (`/u/…`): user icon.
- A transaction link (`/r/x$help&func=Transfer&to=…`): transaction icon, which tells the reader the link prefills a call.
- On a community page (§ What gnoweb shows around a page) every link, internal ones included, also carries `rel="nofollow ugc"`.

**Status**: STABLE.

### `ext_mentions`: auto-link users

**In**: `paid to g1mpkp5lm8lwpm0pym4388836d009zfe4maxlqsq, thanks @alice`

**Out**: both link to `/u/…`.

**Rules**:
- A bech32 `g1…` address links to `/u/{address}`, and an `@name` of 3 or more letters, digits or `_` links to `/u/{name}`.
- A bare email address stays text; `<bob@example.com>` becomes a `mailto:` link.
- From gnoweb v1.6.0, a `/u/` page exists only for an address, a registered name, or a namespace that holds packages, so an `@name` that is none of these links to a 404.

**Status**: STABLE.

### Goldmark defaults also on

PR #4501 turned on:
- **TaskList**: `- [x]`, `- [ ]`.
- **Footnote**: `[^1]` reference syntax.

Worth knowing they work without writing custom syntax.

## What gnoweb shows around a page

From gnoweb v1.6.0, gnoweb sorts every `/r/`, `/p/` and `/u/` page by namespace. A page under the operator's trusted paths (`-trusted-paths`, default list `DefaultTrustedPaths` in `gno.land/pkg/gnoweb/app.go`) is **official**; any other is **community**. No `g1…` address namespace is on the default list, so a realm an agent deploys under its own address is a community page.

On a community page the `gnoweb` binary, by default:

- shows a "Community realm" notice under the header, telling the reader to read the code before interacting or sending coins. It appears on the render, `$source`, `$help` and `$state` views and on the user page; a markdown response carries the header `X-Gnoweb-Realm-Notice: community` instead. The notice says who vouches for the code, not whether the deploy went live: `package_status` answers that.
- titles the page from its path (`<name> · realm by <namespace>`) and gives it a summary and share image built from the path. Nothing the realm renders reaches `<title>`, the description or a link preview.
- marks every link `rel="nofollow ugc"` and keeps the page out of search engines.

On an official page reached with no arguments and no query, a leading `# heading` becomes the `<title>` and the first paragraph of 40 or more characters before the next heading or rule becomes the description. A realm sets no other page metadata: the canonical URL, share image and robots policy are gnoweb's. gnodev leaves the notice off.

## Path-routing conventions

There is **no enforced routing framework**. Each realm parses `path` itself. Three patterns observed across `examples/`:

| Pattern | Used in | Shape |
|---|---|---|
| **No routing** | `r/sys/users`, `r/sys/cla` | Ignores `path`; always renders the same content |
| **`p/nt/mux/v0` router** | `r/gnoland/boards2/v0`, `r/gnoland/coins`, `r/demo/profile`, `r/gnoland/blog` | mux-style segment dispatch |
| **`p/moul/realmpath` parse** | `r/sys/namereg/v0` | single-segment dispatch |

For agents: pick mux for any realm with more than 2 distinct views. Static home + a handful of detail views → static is fine.

Unknown-path handling is **ad-hoc** — no standard 404. Different realms return different shapes: blockquote, custom 404 markdown, empty string, or panic. Pick a convention and stick with it within your realm.

## Audit signals (auditor mode)

When reviewing a realm's `Render()`:

| Pattern | Signal | Action |
|---|---|---|
| `Render()` calling state-mutating methods | RED | Should be read-only — by convention, not by compiler. |
| `<gno-form>` with hidden `<input type="hidden">` | RED | Renderer strips them; design intent is full transparency. Refactor to visible `readonly` inputs. |
| `![alt](https://<host>/…)` with a host outside gnoweb's `img-src` list | YELLOW | The browser blocks it silently. Use an SVG data URI or a listed host. |
| Raw `<script>`, `<style>`, `<iframe>` in output | RED | A default gnoweb drops raw HTML, so it does not run there, but a client that reads the raw markdown (`Accept: text/markdown`, an agent) gets it verbatim. |
| User-submitted strings echoed in markdown without a `sanitize/v0` helper | YELLOW | Markdown injection: users plant links, `<gno-button>` transaction links and `<gno-form>` blocks that pass for the realm's own. |
| Literal `$` in prices or tickers (`$GNOT/$ATOM`), or user text that may hold `$` | YELLOW | gnoweb renders `$…$` as math and the sanitizers leave `$` alone. Write `\$`. |
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

Behavior documented here follows gnoweb's renderer, `gno.land/pkg/gnoweb/markdown/` in the monorepo. `gno_render <pkgPath>` returns the realm's raw markdown, the input to that pipeline, not its HTML; load the page in gnoweb to see what a reader sees.

The exact rendered HTML for every extension is a gnoweb implementation detail (gnoweb is in Beta) — if downstream tooling parses literal markup, pin to a gnoweb commit.
