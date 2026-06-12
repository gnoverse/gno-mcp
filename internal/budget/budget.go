package budget

import (
	"fmt"

	"github.com/gnoverse/gno-mcp/internal/untrusted"
)

// The two budget tiers. DefaultBudget caps broad sweeps (outlines, whole
// packages, every text tool); ExplicitBudget caps explicit, informed requests
// (a named file with full=true, a symbols fetch) — a higher ceiling sized for
// real single-file reads, not a bypass.
const (
	DefaultBudget  = 4 * 1024  // bytes
	ExplicitBudget = 64 * 1024 // bytes
)

type Result struct {
	Summary   string `json:"summary,omitempty"`
	GnowebURL string `json:"gnoweb_url"`
	Truncated bool   `json:"truncated"`
	Size      int    `json:"size"`
	Full      string `json:"full,omitempty"`
}

func Apply(full, gnowebURL string, limit int) Result {
	r := Result{GnowebURL: gnowebURL, Size: len(full)}
	if len(full) <= limit {
		r.Full = full
		return r
	}
	if gnowebURL == "" {
		r.Summary = fmt.Sprintf("%d bytes; preview omitted. Request a smaller slice (a specific file, symbol, or path).", len(full))
	} else {
		r.Summary = fmt.Sprintf("%d bytes; preview omitted. Request a slice (file/symbol/path), or view at %s", len(full), gnowebURL)
	}
	r.Truncated = true
	return r
}

// Wrapped applies the DefaultBudget (broad-sweep tier) and wraps surviving
// content in an untrusted-content envelope. See WrappedAt.
func Wrapped(full, gnowebURL, kind, source string) (text string, truncated bool) {
	return WrappedAt(full, gnowebURL, kind, source, DefaultBudget)
}

// WrappedAt applies the given budget limit to chain-derived content and wraps
// the surviving content in an untrusted-content envelope tagged with kind/source.
// When the content is over budget it is dropped for gnomcp's own truncation
// summary (framing, not chain bytes), returned unwrapped; truncated reports
// which case. Explicit, path-targeted tools (e.g. gno_render) pass
// ExplicitBudget — the same ceiling as a named full-file read.
func WrappedAt(full, gnowebURL, kind, source string, limit int) (text string, truncated bool) {
	r := Apply(full, gnowebURL, limit)
	if r.Truncated {
		return r.Summary, true
	}
	return untrusted.Wrap(r.Full, kind, source), false
}
