package read

import (
	"strings"

	"github.com/gnoverse/gno-mcp/internal/budget"
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

// gnowebURLFor derives a best-effort gnoweb URL from an RPC URL by dropping the
// "rpc." host prefix and the :443 port, then mapping the realm to its gnoweb
// route. Returns "" when not derivable (e.g. a local node). path is appended
// when non-empty.
func gnowebURLFor(rpcURL, realm, path string) string {
	base := rpcURL
	base = strings.Replace(base, "://rpc.", "://", 1)
	base = strings.TrimSuffix(base, ":443")
	if !strings.HasPrefix(base, "http") || strings.Contains(base, "127.0.0.1") || strings.Contains(base, "localhost") {
		return ""
	}
	// gnoweb serves realm "gno.land/r/x" at path "/r/x" (verified: test11 returns
	// 200 for /r/gnoland/home, 404 for /gno.land/r/gnoland/home).
	realmPath := strings.TrimPrefix(realm, "gno.land/")
	u := strings.TrimSuffix(base, "/") + "/" + realmPath
	if path != "" {
		u += "/" + path
	}
	return u
}
