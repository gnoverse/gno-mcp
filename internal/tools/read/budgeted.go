package read

import (
	"strings"

	"github.com/gnoverse/gno-mcp/internal/budget"
)

// budgetBody applies the output budget to a resource body. Returns the
// (possibly summarized) body and whether it was truncated. v2 has no
// line/symbol slicing param, so the budget always applies — including to a
// single large source file (the most common context-blowup payload).
func budgetBody(full, gnowebURL string) (string, bool) {
	r := budget.Apply(full, gnowebURL, false)
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
