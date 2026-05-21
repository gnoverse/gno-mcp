package chain

import "testing"

func TestSignerInterfaceCompile(t *testing.T) {
	var _ Signer = (*signerStub)(nil)
}

type signerStub struct{}

func (signerStub) Address() string               { return "g1stub" }
func (signerStub) Sign(_ []byte) ([]byte, error) { return nil, nil }
