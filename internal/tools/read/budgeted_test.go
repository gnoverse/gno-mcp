package read

import (
	"strings"
	"testing"
)

func TestBudgetResourceBody_TruncatesLarge(t *testing.T) {
	big := strings.Repeat("x", 5000) // > DefaultBudget (4096)
	body, truncated := budgetBody(big, "https://test11.testnets.gno.land/r/foo")
	if !truncated {
		t.Fatal("expected truncation for >4KB body")
	}
	if body == big {
		t.Error("body should have been replaced by a summary")
	}
}

func TestBudgetResourceBody_KeepsSmall(t *testing.T) {
	body, truncated := budgetBody("small", "https://x")
	if truncated || body != "small" {
		t.Errorf("small body should pass through unchanged: %q %v", body, truncated)
	}
}

func TestGnowebURLFor(t *testing.T) {
	cases := []struct{ rpc, realm, path, want string }{
		{"https://rpc.test11.testnets.gno.land:443", "gno.land/r/gnoland/home", "", "https://test11.testnets.gno.land/r/gnoland/home"},
		{"https://rpc.test11.testnets.gno.land:443", "gno.land/r/demo/boards", "post/1", "https://test11.testnets.gno.land/r/demo/boards/post/1"},
		{"http://127.0.0.1:26657", "gno.land/r/x", "", ""}, // local → not derivable
	}
	for _, tc := range cases {
		if got := gnowebURLFor(tc.rpc, tc.realm, tc.path); got != tc.want {
			t.Errorf("gnowebURLFor(%q,%q,%q) = %q, want %q", tc.rpc, tc.realm, tc.path, got, tc.want)
		}
	}
}
