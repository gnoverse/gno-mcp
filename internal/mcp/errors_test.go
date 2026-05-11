package mcp

import (
	"strings"
	"testing"
)

func TestOnboardingRequired(t *testing.T) {
	err := OnboardingRequired("gno_call")
	if err.Code != ErrOnboardingRequired {
		t.Errorf("wrong code: %s", err.Code)
	}
	s := err.Error()
	if !strings.Contains(s, "onboarding_required") {
		t.Errorf("Error() missing code: %s", s)
	}
}
