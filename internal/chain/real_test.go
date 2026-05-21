package chain

import (
	"context"
	"strings"
	"testing"
)

// ---- Real.Call tests

// TestReal_Call_nilSignerSimulate ensures simulate=true still needs a signer.
func TestReal_Call_nilSignerSimulate(t *testing.T) {
	r, err := NewReal("https://rpc.test11.testnets.gno.land:443", "test11")
	if err != nil {
		t.Fatalf("NewReal: %v", err)
	}
	_, err = r.Call(context.Background(), nil, "gno.land/r/x", "Foo", nil, true)
	if err == nil {
		t.Fatal("expected error for nil signer (even with simulate=true)")
	}
	if !strings.Contains(err.Error(), "signer") {
		t.Errorf("error should mention signer, got: %v", err)
	}
}

// TestReal_Call_nilSignerBroadcast ensures non-simulate broadcasts require a signer.
func TestReal_Call_nilSignerBroadcast(t *testing.T) {
	r, err := NewReal("https://rpc.test11.testnets.gno.land:443", "test11")
	if err != nil {
		t.Fatalf("NewReal: %v", err)
	}
	_, err = r.Call(context.Background(), nil, "gno.land/r/x", "Foo", nil, false)
	if err == nil {
		t.Fatal("expected error for nil signer in broadcast mode")
	}
	if !strings.Contains(err.Error(), "signer") {
		t.Errorf("error should mention signer, got: %v", err)
	}
}

// TestReal_Call_signerMustProvideGnoclientSigner: a chain.Signer that does NOT
// implement gnoclientSignerProvider yields an explanatory error.
func TestReal_Call_signerMustProvideGnoclientSigner(t *testing.T) {
	r, err := NewReal("https://rpc.test11.testnets.gno.land:443", "test11")
	if err != nil {
		t.Fatalf("NewReal: %v", err)
	}
	stub := &minimalSigner{addr: "g1notreal"}
	_, err = r.Call(context.Background(), stub, "gno.land/r/x", "Foo", nil, false)
	if err == nil {
		t.Fatal("expected error for signer missing gnoclient.Signer provider")
	}
	if !strings.Contains(err.Error(), "gnoclient.Signer") && !strings.Contains(err.Error(), "session keypair") {
		t.Errorf("error should mention gnoclient.Signer requirement, got: %v", err)
	}
}

// minimalSigner is a chain.Signer stub that does NOT implement gnoclientSignerProvider.
type minimalSigner struct{ addr string }

func (m *minimalSigner) Address() string               { return m.addr }
func (m *minimalSigner) Sign(_ []byte) ([]byte, error) { return nil, nil }

func TestNewReal_validRPCURL(t *testing.T) {
	r, err := NewReal("https://rpc.test5.gno.land:443", "test5")
	if err != nil {
		t.Fatalf("NewReal: %v", err)
	}
	if r == nil {
		t.Fatal("NewReal returned nil")
	}
}

func TestNewReal_emptyRPCURL(t *testing.T) {
	_, err := NewReal("", "test5")
	if err == nil {
		t.Fatal("expected error for empty rpc-url")
	}
}

// ---- Real.Run tests

func TestReal_Run_simulateRequiresSigner(t *testing.T) {
	r, err := NewReal("https://rpc.test11.testnets.gno.land:443", "test11")
	if err != nil {
		t.Fatalf("NewReal: %v", err)
	}
	_, err = r.Run(context.Background(), nil, "package main\nfunc main() {}", true)
	if err == nil {
		t.Fatal("expected error for nil signer (even with simulate=true)")
	}
	if !strings.Contains(err.Error(), "signer") {
		t.Errorf("error should mention signer, got: %v", err)
	}
}

func TestReal_Run_broadcastRequiresSigner(t *testing.T) {
	r, err := NewReal("https://rpc.test11.testnets.gno.land:443", "test11")
	if err != nil {
		t.Fatalf("NewReal: %v", err)
	}
	_, err = r.Run(context.Background(), nil, "package main\nfunc main() {}", false)
	if err == nil {
		t.Fatal("expected error for nil signer in broadcast mode")
	}
	if !strings.Contains(err.Error(), "signer") {
		t.Errorf("error should mention signer, got: %v", err)
	}
}

func TestReal_Run_signerMustProvideGnoclientSigner(t *testing.T) {
	r, err := NewReal("https://rpc.test11.testnets.gno.land:443", "test11")
	if err != nil {
		t.Fatalf("NewReal: %v", err)
	}
	stub := &minimalSigner{addr: "g1notreal"}
	_, err = r.Run(context.Background(), stub, "package main\nfunc main() {}", false)
	if err == nil {
		t.Fatal("expected error for signer missing gnoclient.Signer provider")
	}
	if !strings.Contains(err.Error(), "gnoclient.Signer") && !strings.Contains(err.Error(), "session keypair") {
		t.Errorf("error should mention gnoclient.Signer requirement, got: %v", err)
	}
}

func TestReal_File_rejectsEmptyFile(t *testing.T) {
	r, err := NewReal("https://rpc.test5.gno.land:443", "test5")
	if err != nil {
		t.Fatalf("NewReal: %v", err)
	}
	_, err = r.File(context.Background(), "gno.land/r/foo", "")
	if err == nil {
		t.Fatal("expected error for empty file name (use ListFiles instead)")
	}
	if !strings.Contains(err.Error(), "ListFiles") {
		t.Errorf("error should steer caller to ListFiles, got %q", err)
	}
}
