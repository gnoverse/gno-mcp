// Package txsign signs gno transactions over the legacy signature payload, the
// one every writable chain verifies: gno v1.5.0 chains accept it alongside the
// payload v1.5.0 clients produce by default, and older chains accept nothing
// else. It imports only the gno toolchain, so internal/keystore and the
// standalone faucet share it.
//
// ceiling: legacy payload while a writable chain predates gno v1.5.0; sign with
// gnoclient.SignerFromBip39 once none does.
package txsign

import (
	"fmt"

	"github.com/gnolang/gno/gno.land/pkg/gnoclient"
	"github.com/gnolang/gno/tm2/pkg/crypto/keys"
	"github.com/gnolang/gno/tm2/pkg/std"
)

// SignerFromBip39 is gnoclient.SignerFromBip39 (account 0, index 0, no
// passphrase) with Sign producing a legacy-payload signature.
func SignerFromBip39(mnemonic, chainID string) (gnoclient.Signer, error) {
	const name, password = "default", ""
	kb := keys.NewInMemory()
	if _, err := kb.CreateAccount(name, mnemonic, "", password, 0, 0); err != nil {
		return nil, err
	}
	return legacySigner{gnoclient.SignerFromKeybase{
		Keybase:  kb,
		Account:  name,
		Password: password,
		ChainID:  chainID,
	}}, nil
}

type legacySigner struct {
	gnoclient.SignerFromKeybase
}

// Sign mirrors gnoclient.SignerFromKeybase.Sign with the legacy payload.
func (s legacySigner) Sign(cfg gnoclient.SignCfg) (*std.Tx, error) {
	tx := cfg.UnsignedTX
	signers := tx.GetSigners()
	if tx.Signatures == nil {
		tx.Signatures = make([]std.Signature, len(signers))
	}
	if err := tx.ValidateBasic(); err != nil {
		return nil, err
	}

	signbz, err := tx.GetSignBytesLegacy(s.ChainID, cfg.AccountNumber, cfg.SequenceNumber)
	if err != nil {
		return nil, fmt.Errorf("unable to get tx signature payload, %w", err)
	}
	sig, pub, err := s.Keybase.Sign(s.Account, s.Password, signbz)
	if err != nil {
		return nil, err
	}

	addr := pub.Address()
	found := false
	for i := range tx.Signatures {
		if signers[i] == addr {
			found = true
			tx.Signatures[i] = std.Signature{PubKey: pub, Signature: sig}
		}
	}
	if !found {
		return nil, fmt.Errorf("address %v (%s) not in signer set", addr, s.Account)
	}
	return &tx, nil
}
