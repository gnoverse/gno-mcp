package mcp

import "encoding/json"

type ErrCode string

const (
	ErrOnboardingRequired     ErrCode = "onboarding_required"
	ErrConfirmationRequired   ErrCode = "confirmation_required"
	ErrMainnetWrite           ErrCode = "mainnet_write_blocked"
	ErrAuthenticationRequired ErrCode = "authentication_required"
	ErrAuthenticationExpired  ErrCode = "authentication_expired"
)

type StructuredError struct {
	Code    ErrCode        `json:"code"`
	Message string         `json:"message"`
	Hint    string         `json:"hint,omitempty"`
	Data    map[string]any `json:"data,omitempty"`
}

func (e *StructuredError) Error() string { b, _ := json.Marshal(e); return string(b) }

func OnboardingRequired(toolName string) *StructuredError {
	return &StructuredError{
		Code:    ErrOnboardingRequired,
		Message: "no key configured; cannot " + toolName,
		Hint:    "invoke the gno-onboarding skill to create a testnet key and request faucet funds",
	}
}
