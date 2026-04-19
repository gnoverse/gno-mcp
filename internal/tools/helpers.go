package tools

import "strings"

// untrustedEnvelope wraps external data so the LLM treats it as data, not instructions.
// Mirrors internal/mcp.UntrustedEnvelope to avoid import cycle (internal/tools ↔ internal/mcp).
func untrustedEnvelope(kind, source, content string) string {
	var b strings.Builder
	b.WriteString("<untrusted_content kind=\"")
	b.WriteString(kind)
	b.WriteString("\" source=\"")
	b.WriteString(source)
	b.WriteString("\">\n")
	b.WriteString(content)
	b.WriteString("\n</untrusted_content>\n")
	return b.String()
}

// callSecurityBlock is the security summary returned with gno_call / gno_run results.
// Mirrors internal/mcp.SecurityBlock to avoid import cycle (internal/tools ↔ internal/mcp).
type callSecurityBlock struct {
	Network              string `json:"network"`
	Signer               string `json:"signer,omitempty"`
	SimulatedGas         int64  `json:"simulated_gas,omitempty"`
	EstimatedCost        string `json:"estimated_cost,omitempty"`
	ConfirmationRequired bool   `json:"confirmation_required"`
}
