package chain

import (
	"context"
	"strings"
	"testing"

	"github.com/gnolang/gno/gno.land/pkg/gnoland"
	"github.com/gnolang/gno/gno.land/pkg/sdk/vm"
	"github.com/gnolang/gno/tm2/pkg/amino"
	"github.com/gnolang/gno/tm2/pkg/crypto"
	"github.com/gnolang/gno/tm2/pkg/std"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- Session spend pre-flight (mirrors the chain ante's Phase 2a)

func preflightSession(limitUgnot, usedUgnot, reset, period int64) *gnoland.GnoSessionAccount {
	ugnotCoins := func(n int64) std.Coins {
		if n == 0 {
			return nil
		}
		return std.Coins{std.Coin{Denom: "ugnot", Amount: n}}
	}
	return &gnoland.GnoSessionAccount{
		BaseSessionAccount: std.BaseSessionAccount{
			SpendLimit:  ugnotCoins(limitUgnot),
			SpendUsed:   ugnotCoins(usedUgnot),
			SpendReset:  reset,
			SpendPeriod: period,
		},
	}
}

func preflightTx(master crypto.Address, feeUgnot, sendUgnot int64) *std.Tx {
	tx := &std.Tx{}
	if feeUgnot > 0 {
		tx.Fee.GasFee = std.Coin{Denom: "ugnot", Amount: feeUgnot}
	}
	var send std.Coins
	if sendUgnot > 0 {
		send = std.Coins{std.Coin{Denom: "ugnot", Amount: sendUgnot}}
	}
	tx.Msgs = []std.Msg{vm.MsgCall{Caller: master, Send: send, PkgPath: "gno.land/r/test/x", Func: "Do"}}
	return tx
}

func TestCheckSessionSpendForTx_feeAboveRemainingErrors(t *testing.T) {
	// The test13 shape: 1000000ugnot limit, 4000000ugnot offered fee. The
	// chain would reject with a bare "session not allowed error"; the
	// pre-flight must fail first with the numbers and a recovery hint.
	master := crypto.Address{1}
	err := checkSessionSpendForTx(preflightSession(1_000_000, 0, 0, 0), preflightTx(master, 4_000_000, 0), master, 1000)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "4000000ugnot")
	assert.Contains(t, err.Error(), "1000000ugnot")
	assert.Contains(t, err.Error(), "gno_session_propose", "error should point at the recovery path")
}

func TestCheckSessionSpendForTx_feeWithinRemainingOK(t *testing.T) {
	master := crypto.Address{1}
	err := checkSessionSpendForTx(preflightSession(1_000_000, 0, 0, 0), preflightTx(master, 400_000, 0), master, 1000)
	require.NoError(t, err)
}

func TestCheckSessionSpendForTx_feePlusSendCounted(t *testing.T) {
	// fee 400000 + send 700000 = 1100000 > 1000000 limit: the msg's declared
	// outflow counts, exactly as the ante's Phase 2a totals it.
	master := crypto.Address{1}
	err := checkSessionSpendForTx(preflightSession(1_000_000, 0, 0, 0), preflightTx(master, 400_000, 700_000), master, 1000)
	require.Error(t, err)
}

func TestCheckSessionSpendForTx_spendUsedCounted(t *testing.T) {
	master := crypto.Address{1}
	err := checkSessionSpendForTx(preflightSession(1_000_000, 900_000, 0, 0), preflightTx(master, 200_000, 0), master, 1000)
	require.Error(t, err)
}

func TestCheckSessionSpendForTx_elapsedPeriodResetsUsed(t *testing.T) {
	// SpendPeriod elapsed: used resets, the write fits again — the pre-flight
	// must not reject a session the chain would accept.
	master := crypto.Address{1}
	acc := preflightSession(1_000_000, 900_000, 1000, 3600)
	err := checkSessionSpendForTx(acc, preflightTx(master, 200_000, 0), master, 1000+7200)
	require.NoError(t, err)
}

func TestCheckSessionSpendForTx_zeroOutflowOK(t *testing.T) {
	master := crypto.Address{1}
	err := checkSessionSpendForTx(preflightSession(1_000_000, 0, 0, 0), preflightTx(master, 0, 0), master, 1000)
	require.NoError(t, err)
}

// ---- Real.CallAsUser tests

// TestReal_Call_nilSignerSimulate ensures simulate=true still needs a signer.
func TestReal_Call_nilSignerSimulate(t *testing.T) {
	r, err := NewReal("https://rpc.test13.testnets.gno.land:443", "test-13")
	require.NoError(t, err, "NewReal")
	_, err = r.CallAsUser(context.Background(), nil, "g1master", "gno.land/r/x", "Foo", nil, "", true)
	require.Error(t, err, "expected error for nil signer (even with simulate=true)")
	assert.True(t, strings.Contains(err.Error(), "signer"), "error should mention signer, got: %v", err)
}

// TestReal_Call_nilSignerBroadcast ensures non-simulate broadcasts require a signer.
func TestReal_Call_nilSignerBroadcast(t *testing.T) {
	r, err := NewReal("https://rpc.test13.testnets.gno.land:443", "test-13")
	require.NoError(t, err, "NewReal")
	_, err = r.CallAsUser(context.Background(), nil, "g1master", "gno.land/r/x", "Foo", nil, "", false)
	require.Error(t, err, "expected error for nil signer in broadcast mode")
	assert.True(t, strings.Contains(err.Error(), "signer"), "error should mention signer, got: %v", err)
}

// TestReal_Call_emptyMaster ensures master is required.
func TestReal_Call_emptyMaster(t *testing.T) {
	r, err := NewReal("https://rpc.test13.testnets.gno.land:443", "test-13")
	require.NoError(t, err, "NewReal")
	stub := &minimalSigner{addr: "g1notreal"}
	_, err = r.CallAsUser(context.Background(), stub, "", "gno.land/r/x", "Foo", nil, "", false)
	require.Error(t, err, "expected error for empty master")
	assert.True(t, strings.Contains(err.Error(), "master"), "error should mention master, got: %v", err)
}

// minimalSigner is a chain.Signer stub.
type minimalSigner struct{ addr string }

func (m *minimalSigner) Address() string               { return m.addr }
func (m *minimalSigner) Pubkey() []byte                { return make([]byte, 32) }
func (m *minimalSigner) Sign(_ []byte) ([]byte, error) { return nil, nil }

func TestNewReal_validRPCURL(t *testing.T) {
	r, err := NewReal("https://rpc.test5.gno.land:443", "test5")
	require.NoError(t, err, "NewReal")
	require.NotNil(t, r)
}

func TestNewReal_emptyRPCURL(t *testing.T) {
	_, err := NewReal("", "test5")
	require.Error(t, err, "expected error for empty rpc-url")
}

func TestNewReal_emptyChainID(t *testing.T) {
	_, err := NewReal("https://rpc.test5.gno.land:443", "")
	require.Error(t, err, "expected error for empty chain-id")
}

// ---- Real.RunAsUser tests

func TestReal_Run_simulateRequiresSigner(t *testing.T) {
	r, err := NewReal("https://rpc.test13.testnets.gno.land:443", "test-13")
	require.NoError(t, err, "NewReal")
	_, err = r.RunAsUser(context.Background(), nil, "g1master", "package main\nfunc main() {}", true)
	require.Error(t, err, "expected error for nil signer (even with simulate=true)")
	assert.True(t, strings.Contains(err.Error(), "signer"), "error should mention signer, got: %v", err)
}

func TestReal_Run_broadcastRequiresSigner(t *testing.T) {
	r, err := NewReal("https://rpc.test13.testnets.gno.land:443", "test-13")
	require.NoError(t, err, "NewReal")
	_, err = r.RunAsUser(context.Background(), nil, "g1master", "package main\nfunc main() {}", false)
	require.Error(t, err, "expected error for nil signer in broadcast mode")
	assert.True(t, strings.Contains(err.Error(), "signer"), "error should mention signer, got: %v", err)
}

func TestReal_Run_emptyMaster(t *testing.T) {
	r, err := NewReal("https://rpc.test13.testnets.gno.land:443", "test-13")
	require.NoError(t, err, "NewReal")
	stub := &minimalSigner{addr: "g1notreal"}
	_, err = r.RunAsUser(context.Background(), stub, "", "package main\nfunc main() {}", false)
	require.Error(t, err, "expected error for empty master")
	assert.True(t, strings.Contains(err.Error(), "master"), "error should mention master, got: %v", err)
}

func TestReal_File_rejectsEmptyFile(t *testing.T) {
	r, err := NewReal("https://rpc.test5.gno.land:443", "test5")
	require.NoError(t, err, "NewReal")
	_, err = r.File(context.Background(), "gno.land/r/foo", "")
	require.Error(t, err, "expected error for empty file name (use ListFiles instead)")
	assert.True(t, strings.Contains(err.Error(), "ListFiles"), "error should steer caller to ListFiles, got %q", err)
}

// ---- Real.QuerySession tests

// TestReal_QuerySession_emptyArgsReturnsUnsupported: empty master or session
// addr means the caller cannot identify the session record on chain. The
// Manager treats this as Unsupported (keep local state) rather than wiping.
func TestReal_QuerySession_emptyArgsReturnsUnsupported(t *testing.T) {
	r, err := NewReal("https://rpc.test13.testnets.gno.land:443", "test-13")
	require.NoError(t, err, "NewReal")
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
			require.Error(t, err, "expected ErrSessionQueryUnsupported")
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
		AllowPaths: []string{"vm/exec:gno.land/r/test/blog", "vm/exec:gno.land/r/test/forum", "vm/run"},
	}

	data, err := amino.MarshalJSONIndent(original, "", "  ")
	require.NoError(t, err, "amino.MarshalJSONIndent")

	got, err := decodeSessionAccount(data)
	require.NoError(t, err, "decodeSessionAccount:\npayload:\n%s", data)

	assert.Equal(t, uint64(42), got.AccountNumber)
	assert.Equal(t, uint64(7), got.Sequence)
	assert.Equal(t, int64(1735689600), got.ExpiresAt)
	assert.Equal(t, master, got.MasterAddress)
	assert.True(t, got.SpendLimit.IsEqual(original.SpendLimit), "SpendLimit = %s, want %s", got.SpendLimit, original.SpendLimit)
	assert.True(t, got.SpendUsed.IsEqual(original.SpendUsed), "SpendUsed = %s, want %s", got.SpendUsed, original.SpendUsed)
	assert.Equal(t, int64(3600), got.SpendPeriod)
	require.Len(t, got.AllowPaths, 3)
	assert.Equal(t, "vm/exec:gno.land/r/test/blog", got.AllowPaths[0])
	assert.Equal(t, "vm/exec:gno.land/r/test/forum", got.AllowPaths[1])
	assert.Equal(t, "vm/run", got.AllowPaths[2])
}

// TestSplitAllowPaths verifies the chain → internal translation:
// "vm/exec:<realm>" → realmPaths; "vm/run" → allowRun=true; other tokens dropped.
func TestSplitAllowPaths(t *testing.T) {
	cases := []struct {
		name      string
		in        []string
		wantPaths []string
		wantAllow bool
	}{
		{
			name:      "vm/exec entries only",
			in:        []string{"vm/exec:gno.land/r/foo", "vm/exec:gno.land/r/bar"},
			wantPaths: []string{"gno.land/r/foo", "gno.land/r/bar"},
			wantAllow: false,
		},
		{
			name:      "vm/run only",
			in:        []string{"vm/run"},
			wantPaths: nil,
			wantAllow: true,
		},
		{
			name:      "mixed vm/exec and vm/run",
			in:        []string{"vm/exec:gno.land/r/foo", "vm/run"},
			wantPaths: []string{"gno.land/r/foo"},
			wantAllow: true,
		},
		{
			name:      "unknown token dropped",
			in:        []string{"vm/exec:gno.land/r/foo", "bank/send"},
			wantPaths: []string{"gno.land/r/foo"},
			wantAllow: false,
		},
		{
			name:      "empty input",
			in:        nil,
			wantPaths: nil,
			wantAllow: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			paths, allowRun := splitAllowPaths(tc.in)
			assert.Equal(t, tc.wantAllow, allowRun)
			require.Len(t, paths, len(tc.wantPaths))
			for i, p := range paths {
				assert.Equal(t, tc.wantPaths[i], p, "paths[%d]", i)
			}
		})
	}
}
