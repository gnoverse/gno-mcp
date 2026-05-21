package chain

import (
	"context"
	"testing"
)

func TestSignerInterfaceCompile(t *testing.T) {
	var _ Signer = (*signerStub)(nil)
}

type signerStub struct{}

func (signerStub) Address() string               { return "g1stub" }
func (signerStub) Sign(_ []byte) ([]byte, error) { return nil, nil }

func TestCallResultZeroValue(t *testing.T) {
	var r CallResult
	if r.TxHash != "" || r.Simulated {
		t.Fatal("zero value unexpectedly non-zero")
	}
}

func TestRunResultZeroValue(t *testing.T) {
	var r RunResult
	if r.TxHash != "" {
		t.Fatal("zero value unexpectedly non-zero")
	}
}

func TestSessionStatusZeroValue(t *testing.T) {
	var s SessionStatus
	if s.Active {
		t.Fatal("zero value unexpectedly active")
	}
}

var _ = context.Background
