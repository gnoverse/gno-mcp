package write

import "fmt"

// signedByLine renders the acting-identity line for a write result.
func signedByLine(identity, signerAddr, master string) string {
	if identity == "session" {
		return fmt.Sprintf("Signed by: session %s on behalf of master %s", signerAddr, master)
	}
	return fmt.Sprintf("Signed by: agent test1 (%s)", signerAddr)
}
