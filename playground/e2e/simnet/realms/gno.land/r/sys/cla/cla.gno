// Package cla mirrors gno.land/r/sys/cla as deployed on the live gno.land
// networks (test13 et al.), so the simnet exercises the same keeper-facing
// CLA deploy gate (Sign / HasValidSignature) the keeper enforces in
// gno.land/pkg/sdk/vm/keeper.go:checkCLASignature.
//
// Sign and HasValidSignature are copied verbatim from the upstream realm at the
// gno commit pinned in this repo's go.mod. The upstream admin.gno (which sets
// requiredHash through a GovDAO ProposeNewCLA proposal) is intentionally NOT
// vendored: it pulls in the whole r/gov/dao dependency closure the simnet has
// no other use for. The simnet sets requiredHash through the sim-only
// SetRequiredHash in genesis.gno instead — see that file. The gate behaviour
// and storage economics the agent flow hits are identical either way.
package cla

import (
	"chain"

	"gno.land/p/moul/addrset"
)

const SignedEvent = "CLASigned"

var (
	requiredHash string // SHA256 hash of the CLA document; empty = enforcement disabled
	claURL       string // URL where the CLA document can be found
	signatures   addrset.Set
)

// Sign records a CLA signature for the caller.
// The hash must match the current required hash.
func Sign(cur realm, hash string) {
	if hash != requiredHash {
		panic("hash does not match required CLA hash")
	}

	caller := cur.Previous().Address()
	signatures.Add(caller)

	chain.Emit(
		SignedEvent,
		"signer", caller.String(),
		"hash", hash,
	)
}

// HasValidSignature checks if an address has signed the current required CLA.
// Returns true if CLA enforcement is disabled (requiredHash == ""),
// or if the address has signed.
func HasValidSignature(addr address) bool {
	if requiredHash == "" {
		return true
	}
	return signatures.Has(addr)
}
