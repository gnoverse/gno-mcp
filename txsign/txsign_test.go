package txsign

import (
	"testing"

	"github.com/gnolang/gno/gno.land/pkg/gnoclient"
	"github.com/gnolang/gno/tm2/pkg/crypto"
	"github.com/gnolang/gno/tm2/pkg/sdk/bank"
	"github.com/gnolang/gno/tm2/pkg/std"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The well-known gno test1 mnemonic.
const testMnemonic = "source bonus chronic canvas draft south burst lottery vacant surface solve popular case indicate oppose farm nothing bullet exhibit title speed wink action roast"

func sendTx(from crypto.Address) std.Tx {
	return std.Tx{
		Msgs: []std.Msg{bank.MsgSend{
			FromAddress: from,
			ToAddress:   crypto.AddressFromPreimage([]byte("recipient")),
			Amount:      std.NewCoins(std.NewCoin("ugnot", 1)),
		}},
		Fee: std.NewFee(100_000, std.NewCoin("ugnot", 1_000)),
	}
}

// Chains predating gno v1.5.0 verify only the legacy payload; v1.5.0 chains
// verify either, so the legacy one is the payload every writable chain takes.
func TestSignerFromBip39_signsLegacyPayload(t *testing.T) {
	signer, err := SignerFromBip39(testMnemonic, "test-chain")
	require.NoError(t, err)
	info, err := signer.Info()
	require.NoError(t, err)
	tx := sendTx(info.GetAddress())

	signed, err := signer.Sign(gnoclient.SignCfg{UnsignedTX: tx, AccountNumber: 3, SequenceNumber: 4})
	require.NoError(t, err)
	require.Len(t, signed.Signatures, 1)
	sig := signed.Signatures[0]

	legacy, err := tx.GetSignBytesLegacy("test-chain", 3, 4)
	require.NoError(t, err)
	assert.True(t, sig.PubKey.VerifyBytes(legacy, sig.Signature), "must verify over the legacy payload")
	current, err := tx.GetSignBytes("test-chain", 3, 4)
	require.NoError(t, err)
	assert.False(t, sig.PubKey.VerifyBytes(current, sig.Signature), "signed over the v1.5.0 payload, which older chains reject")
}

func TestSign_refusesATxItIsNotASignerOf(t *testing.T) {
	signer, err := SignerFromBip39(testMnemonic, "test-chain")
	require.NoError(t, err)

	_, err = signer.Sign(gnoclient.SignCfg{UnsignedTX: sendTx(crypto.AddressFromPreimage([]byte("someone else")))})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not in signer set")
}
