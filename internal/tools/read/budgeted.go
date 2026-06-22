package read

import (
	"github.com/gnoverse/gno-mcp/internal/budget"
	"github.com/gnoverse/gno-mcp/internal/profiles"
)

// budgetBody applies the output budget to a resource body at the given tier
// (budget.DefaultBudget for whole-package raw, budget.ExplicitBudget for the
// bounded/explicit modes: outline, symbols, full=true on a named file).
// Returns the (possibly summarized) body and whether it was truncated.
//
// Unlike the OutputText tools, gno_read does NOT wrap the body in an
// <untrusted_content> envelope: the body is delivered as an MCP
// EmbeddedResource, whose trust posture is the untrusted-content marker, and
// wrapping would corrupt the txtar archive (the closing tag would merge into
// the last file). This relies on the client honoring the resource boundary; a
// client that flattens resources into the prompt as plain text would see this
// content unwrapped. (The outline path additionally neutralizes embedded
// envelope tags — it is server-rendered, so fidelity is not a contract there.)
func budgetBody(full, gnowebURL string, limit int) (string, bool) {
	r := budget.Apply(full, gnowebURL, limit)
	if r.Truncated {
		return r.Summary, true
	}
	return r.Full, false
}

// gnowebURLFor returns the gnoweb URL for realm under profile p, with an
// optional sub-path (e.g. a render path) appended when non-empty. Returns ""
// when the profile has no usable gnoweb host. The realm→gnoweb route mapping
// and the configured-vs-derived host choice live in Profile.RealmViewURL.
func gnowebURLFor(p profiles.Profile, realm, path string) string {
	u := p.RealmViewURL(realm)
	if u == "" || path == "" {
		return u
	}
	return u + "/" + path
}
