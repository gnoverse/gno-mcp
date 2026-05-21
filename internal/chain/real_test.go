package chain

import (
	"context"
	"strings"
	"testing"

	"github.com/gnolang/gno/gno.land/pkg/gnoland"
	"github.com/gnolang/gno/tm2/pkg/amino"
	"github.com/gnolang/gno/tm2/pkg/crypto"
	"github.com/gnolang/gno/tm2/pkg/std"
)

// ---- Real.Call tests

// TestReal_Call_nilSignerSimulate ensures simulate=true still needs a signer.
func TestReal_Call_nilSignerSimulate(t *testing.T) {
	r, err := NewReal("https://rpc.test11.testnets.gno.land:443", "test11")
	if err != nil {
		t.Fatalf("NewReal: %v", err)
	}
	_, err = r.Call(context.Background(), nil, "g1master", "gno.land/r/x", "Foo", nil, true)
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
	_, err = r.Call(context.Background(), nil, "g1master", "gno.land/r/x", "Foo", nil, false)
	if err == nil {
		t.Fatal("expected error for nil signer in broadcast mode")
	}
	if !strings.Contains(err.Error(), "signer") {
		t.Errorf("error should mention signer, got: %v", err)
	}
}

// TestReal_Call_emptyMaster ensures master is required.
func TestReal_Call_emptyMaster(t *testing.T) {
	r, err := NewReal("https://rpc.test11.testnets.gno.land:443", "test11")
	if err != nil {
		t.Fatalf("NewReal: %v", err)
	}
	stub := &minimalSigner{addr: "g1notreal"}
	_, err = r.Call(context.Background(), stub, "", "gno.land/r/x", "Foo", nil, false)
	if err == nil {
		t.Fatal("expected error for empty master")
	}
	if !strings.Contains(err.Error(), "master") {
		t.Errorf("error should mention master, got: %v", err)
	}
}

// minimalSigner is a chain.Signer stub.
type minimalSigner struct{ addr string }

func (m *minimalSigner) Address() string               { return m.addr }
func (m *minimalSigner) Pubkey() []byte                { return make([]byte, 32) }
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

func TestNewReal_emptyChainID(t *testing.T) {
	_, err := NewReal("https://rpc.test5.gno.land:443", "")
	if err == nil {
		t.Fatal("expected error for empty chain-id")
	}
}

// ---- Real.Run tests

func TestReal_Run_simulateRequiresSigner(t *testing.T) {
	r, err := NewReal("https://rpc.test11.testnets.gno.land:443", "test11")
	if err != nil {
		t.Fatalf("NewReal: %v", err)
	}
	_, err = r.Run(context.Background(), nil, "g1master", "package main\nfunc main() {}", true)
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
	_, err = r.Run(context.Background(), nil, "g1master", "package main\nfunc main() {}", false)
	if err == nil {
		t.Fatal("expected error for nil signer in broadcast mode")
	}
	if !strings.Contains(err.Error(), "signer") {
		t.Errorf("error should mention signer, got: %v", err)
	}
}

func TestReal_Run_emptyMaster(t *testing.T) {
	r, err := NewReal("https://rpc.test11.testnets.gno.land:443", "test11")
	if err != nil {
		t.Fatalf("NewReal: %v", err)
	}
	stub := &minimalSigner{addr: "g1notreal"}
	_, err = r.Run(context.Background(), stub, "", "package main\nfunc main() {}", false)
	if err == nil {
		t.Fatal("expected error for empty master")
	}
	if !strings.Contains(err.Error(), "master") {
		t.Errorf("error should mention master, got: %v", err)
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

// ---- Real.QuerySession tests

// TestReal_QuerySession_emptyArgsReturnsUnsupported: empty master or session
// addr means the caller cannot identify the session record on chain. The
// Manager treats this as Unsupported (keep local state) rather than wiping.
func TestReal_QuerySession_emptyArgsReturnsUnsupported(t *testing.T) {
	r, err := NewReal("https://rpc.test11.testnets.gno.land:443", "test11")
	if err != nil {
		t.Fatalf("NewReal: %v", err)
	}
	for _, tc := range []struct {
		name             string
		master, sessAddr string
	}{
		{"empty master", "", "g1sess"},
		{"empty session", "g1master", ""},
		{"both empty", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := r.QuerySession(context.Background(), tc.master, tc.sessAddr)
			if err == nil {
				t.Fatalf("expected ErrSessionQueryUnsupported")
			}
		})
	}
}

// TestDecodeSessionAccount_aminoJSON_roundTrip pins the contract that
// auth/accounts/<master>/session/<addr> is amino-JSON encoded (NOT std JSON):
// embedded structs are not flattened, integers are string-encoded, and
// std.Coins is marshaled as a string like "1000ugnot". encoding/json silently
// drops the embedded BaseSessionAccount subtree, which would zero out
// AccountNumber/Sequence/ExpiresAt — corrupting tx signing.
//
// This test marshals a populated GnoSessionAccount via amino.MarshalJSONIndent
// (the exact path tm2/pkg/sdk/auth/handler.go uses) and asserts every
// load-bearing field round-trips through decodeSessionAccount.
func TestDecodeSessionAccount_aminoJSON_roundTrip(t *testing.T) {
	master := crypto.AddressFromPreimage([]byte("master-test-preimage"))
	sessAddr := crypto.AddressFromPreimage([]byte("session-test-preimage"))

	original := &gnoland.GnoSessionAccount{
		BaseSessionAccount: std.BaseSessionAccount{
			BaseAccount: std.BaseAccount{
				Address:       sessAddr,
				AccountNumber: 42,
				Sequence:      7,
			},
			MasterAddress: master,
			ExpiresAt:     1735689600,
			SpendLimit:    std.NewCoins(std.NewCoin("ugnot", 1_000_000)),
			SpendPeriod:   3600,
			SpendUsed:     std.NewCoins(std.NewCoin("ugnot", 250_000)),
			SpendReset:    1735600000,
		},
		AllowPaths: []string{"gno.land/r/test/blog", "gno.land/r/test/forum"},
	}

	data, err := amino.MarshalJSONIndent(original, "", "  ")
	if err != nil {
		t.Fatalf("amino.MarshalJSONIndent: %v", err)
	}

	got, err := decodeSessionAccount(data)
	if err != nil {
		t.Fatalf("decodeSessionAccount: %v\npayload:\n%s", err, data)
	}

	if got.AccountNumber != 42 {
		t.Errorf("AccountNumber = %d, want 42", got.AccountNumber)
	}
	if got.Sequence != 7 {
		t.Errorf("Sequence = %d, want 7", got.Sequence)
	}
	if got.ExpiresAt != 1735689600 {
		t.Errorf("ExpiresAt = %d, want 1735689600", got.ExpiresAt)
	}
	if got.MasterAddress != master {
		t.Errorf("MasterAddress = %s, want %s", got.MasterAddress, master)
	}
	if !got.SpendLimit.IsEqual(original.SpendLimit) {
		t.Errorf("SpendLimit = %s, want %s", got.SpendLimit, original.SpendLimit)
	}
	if !got.SpendUsed.IsEqual(original.SpendUsed) {
		t.Errorf("SpendUsed = %s, want %s", got.SpendUsed, original.SpendUsed)
	}
	if got.SpendPeriod != 3600 {
		t.Errorf("SpendPeriod = %d, want 3600", got.SpendPeriod)
	}
	if len(got.AllowPaths) != 2 || got.AllowPaths[0] != "gno.land/r/test/blog" || got.AllowPaths[1] != "gno.land/r/test/forum" {
		t.Errorf("AllowPaths = %v, want [gno.land/r/test/blog gno.land/r/test/forum]", got.AllowPaths)
	}
}
