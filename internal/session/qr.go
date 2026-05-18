package session

import (
	"bytes"
	"fmt"

	"github.com/mdp/qrterminal/v3"
)

// FundURL is the deeplink format the user opens in their wallet to fund the
// session address. v0.2 demo uses a documented placeholder; a real wallet
// scheme lands once gnoland.wallet supports authorisation deeplinks.
//
// The shape — scheme://send?to=…&amount=…&memo=… — is structurally close to
// the EIP-681 / Solana Pay style and lets us produce stable, copy-pasteable
// links while the spec is informal.
const fundScheme = "gnoland"

// FundURL builds the canonical fund link for an address.
func FundURL(addr string, ugnot int64, memo string) string {
	return fmt.Sprintf("%s://send?to=%s&amount=%dugnot&memo=%s", fundScheme, addr, ugnot, memo)
}

// QRASCII renders a compact terminal-safe QR for the link. Output is one
// string with embedded newlines so it can be returned in a structured-error
// data payload and printed by any MCP client that surfaces text content.
func QRASCII(link string) string {
	var buf bytes.Buffer
	qrterminal.GenerateWithConfig(link, qrterminal.Config{
		Level:     qrterminal.L,
		Writer:    &buf,
		BlackChar: qrterminal.BLACK,
		WhiteChar: qrterminal.WHITE,
		QuietZone: 1,
	})
	return buf.String()
}

// AuthPayload is the structured-error.data shape that write tools return
// when a session needs funding. Clients (and the gno-session-auth skill)
// rely on this shape to present the link to the user.
type AuthPayload struct {
	State         State  `json:"state"`
	Network       string `json:"network"`
	Address       string `json:"session_address"`
	Threshold     int64  `json:"threshold_ugnot"`
	CurrentBal    int64  `json:"current_balance_ugnot"`
	FundURL       string `json:"fund_url"`
	QRASCII       string `json:"qr_ascii,omitempty"`
	WebFundURL    string `json:"web_fund_url"`
	HumanGuidance string `json:"human_guidance"`
}

// BuildAuthPayload returns the data block embedded in authentication_required
// errors. memo is the audit tag we'll later use to scope what the session
// was authorized for.
func BuildAuthPayload(s Snapshot, memo string) AuthPayload {
	link := FundURL(s.Address, s.Threshold, memo)
	return AuthPayload{
		State:      s.State,
		Network:    s.Network,
		Address:    s.Address,
		Threshold:  s.Threshold,
		CurrentBal: s.Balance,
		FundURL:    link,
		QRASCII:    QRASCII(link),
		// Web fallback: gnoweb's send page (or whatever wallet ships). Hard-coded
		// for v0.2 demo; in v0.3 we read the discovered network's wallet URL.
		WebFundURL: fmt.Sprintf("https://%s/r/sys/wallet?send_to=%s&amount=%dugnot&memo=%s",
			s.Network, s.Address, s.Threshold, memo),
		HumanGuidance: fmt.Sprintf(
			"Send at least %d ugnot to %s to authorize this MCP session. Open the link in your gno wallet, or scan the QR. Once funded the session works automatically.",
			s.Threshold, s.Address),
	}
}
