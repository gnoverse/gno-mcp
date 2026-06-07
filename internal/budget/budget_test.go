package budget

import (
	"strings"
	"testing"
)

func TestApplyBudget_Small(t *testing.T) {
	r := Apply("hello", "https://gno.land/r/x", false)
	if r.Full != "hello" || r.Truncated {
		t.Errorf("small content should be returned in full: %+v", r)
	}
}

func TestApplyBudget_Large(t *testing.T) {
	large := string(make([]byte, 5000))
	r := Apply(large, "https://gno.land/r/x", false)
	if !r.Truncated || r.Full != "" {
		t.Errorf("large content should be truncated: %+v", r)
	}
	if r.Summary == "" {
		t.Error("expected summary for large content")
	}
}

func TestApplyBudget_SliceRequested(t *testing.T) {
	large := string(make([]byte, 5000))
	r := Apply(large, "https://gno.land/r/x", true)
	if r.Truncated || r.Full == "" {
		t.Errorf("slice requested should always return full: %+v", r)
	}
}

func TestApplyBudget_NoURL(t *testing.T) {
	big := make([]byte, DefaultBudget+1)
	r := Apply(string(big), "", false)
	if !r.Truncated {
		t.Fatal("expected truncation")
	}
	if !strings.Contains(r.Summary, "preview omitted") || strings.Contains(r.Summary, "view at ") {
		t.Errorf("empty-URL summary must omit the 'view at' clause: %q", r.Summary)
	}
}
