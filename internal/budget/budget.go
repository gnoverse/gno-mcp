package budget

import "fmt"

const DefaultBudget = 4 * 1024 // bytes

type Result struct {
	Summary   string `json:"summary,omitempty"`
	GnowebURL string `json:"gnoweb_url"`
	Truncated bool   `json:"truncated"`
	Size      int    `json:"size"`
	Full      string `json:"full,omitempty"`
}

func Apply(full, gnowebURL string, sliceRequested bool) Result {
	r := Result{GnowebURL: gnowebURL, Size: len(full)}
	if sliceRequested || len(full) <= DefaultBudget {
		r.Full = full
		return r
	}
	if gnowebURL == "" {
		r.Summary = fmt.Sprintf("%d bytes; preview omitted. Request a smaller slice (a specific file or path).", len(full))
	} else {
		r.Summary = fmt.Sprintf("%d bytes; preview omitted. Request a slice (file/path), or view at %s", len(full), gnowebURL)
	}
	r.Truncated = true
	return r
}
