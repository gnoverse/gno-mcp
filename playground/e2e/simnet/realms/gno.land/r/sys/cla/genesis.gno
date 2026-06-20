package cla

import "gno.land/p/moul/addrset"

// genesisAdmin is the only caller permitted to flip CLA enforcement through
// SetRequiredHash. It is the simnet's genesis deployer (test1), so the gate is
// activated by a single genesis transaction and cannot be cleared at runtime by
// the agent under test — closing the obvious "just disable the CLA" shortcut.
var genesisAdmin = address("g1jg8mtutu9khhfwc4nxmuhcpftf0pajdhfvsqf5")

// SetRequiredHash activates (newHash != "") or clears (newHash == "") CLA
// enforcement directly. The live network sets the hash through a GovDAO
// ProposeNewCLA proposal (admin.gno upstream); the simnet has no gov/dao realm,
// so it seeds the gate with this sim-only setter, invoked once at genesis by
// the deployer. Changing the hash resets prior signatures, matching upstream
// setRequiredHash semantics.
func SetRequiredHash(cur realm, newHash string) {
	if cur.Previous().Address() != genesisAdmin {
		panic("cla: SetRequiredHash is restricted to the genesis admin")
	}
	requiredHash = newHash
	signatures = addrset.Set{}
}
