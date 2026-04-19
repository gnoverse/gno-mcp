package mcp

type SecurityBlock struct {
	Network              string `json:"network"`
	Signer               string `json:"signer,omitempty"`
	Address              string `json:"address,omitempty"`
	SessionScope         any    `json:"session_scope"`
	SimulatedGas         int64  `json:"simulated_gas,omitempty"`
	EstimatedCost        string `json:"estimated_cost,omitempty"`
	ConfirmationRequired bool   `json:"confirmation_required"`
}
