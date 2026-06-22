package clientfaucet

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/gnoverse/gno-mcp/internal/profiles"
)

// maxFaucetRespBytes bounds the response we read from the faucet service: the
// body is a tiny JSON object, and the service is untrusted (profiles permit
// plain http, so a MITM too).
const maxFaucetRespBytes = 4 << 10

// txHashRE is the shape a tx hash must match before it is embedded into
// LLM-visible output. It blocks injection text and over-budget blobs.
var txHashRE = regexp.MustCompile(`^(0x)?[0-9a-fA-F]{1,128}$`)

// ServiceFaucet talks to an automatic agent-faucet service (POST /fund).
type ServiceFaucet struct {
	url   string
	http  *http.Client
	chain chain.Client
}

func (s *ServiceFaucet) Fund(ctx context.Context, address, chainID string) (Outcome, error) {
	reqBody, _ := json.Marshal(map[string]string{"address": address, "chain_id": chainID})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.url+"/fund", bytes.NewReader(reqBody))
	if err != nil {
		return Outcome{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.http.Do(req)
	if err != nil {
		return Outcome{}, fmt.Errorf("faucet service unreachable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		return Outcome{}, rateLimitMessage(address, resp.Body)
	}
	if resp.StatusCode != http.StatusOK {
		// The body is attacker-influenceable prose headed for LLM-visible error
		// text; the label marks it (envelope forgery is neutralized at the SDK
		// error boundary).
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return Outcome{}, fmt.Errorf("faucet service error: %s: [untrusted faucet response] %s", resp.Status, strings.TrimSpace(string(b)))
	}
	var body struct {
		TxHash string `json:"tx_hash"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxFaucetRespBytes)).Decode(&body); err != nil {
		return Outcome{}, fmt.Errorf("faucet service: bad response: %w", err)
	}
	if !txHashRE.MatchString(body.TxHash) {
		return Outcome{}, fmt.Errorf("faucet service: response tx hash is not a valid hash")
	}
	return Outcome{
		Backend:      "service",
		Address:      address,
		TxHash:       body.TxHash,
		Instructions: fmt.Sprintf("Requested an automatic grant for %s (tx %s).", address, body.TxHash),
	}, nil
}

// rateLimitMessage turns a 429 body into agent-facing guidance. A per_address
// limit gets recovery-shaped, informative (not imperative — the address may
// already hold funds) text naming the fresh-key escape hatch; every other limit
// stays generic so the anti-abuse internals are not disclosed. The body is
// untrusted: it is read bounded, and an unparseable body falls back to generic.
func rateLimitMessage(address string, body io.Reader) error {
	var rl struct {
		Limit             string `json:"limit"`
		RetryAfterSeconds int    `json:"retry_after_seconds"`
	}
	_ = json.NewDecoder(io.LimitReader(body, maxFaucetRespBytes)).Decode(&rl)
	hours := rl.RetryAfterSeconds / 3600
	if hours < 1 {
		hours = 24 // body missing/garbage -> coarse default
	}
	if rl.Limit == "per_address" {
		return fmt.Errorf(
			"address %s has already drawn its per-address faucet grant for this window — "+
				"it may already hold funds (check the balance). A different key is not subject "+
				"to this address's cooldown (gno_key_generate) if you need a separate funded "+
				"identity; otherwise retry this address after ~%dh", address, hours)
	}
	return fmt.Errorf("faucet rate-limited; retry after ~%dh", hours)
}

// FaucetLimits mirrors the faucet's GET /limits JSON. Defined here, not imported
// from the faucet package, because gnomcp treats the faucet as an untrusted
// external HTTP service rather than a linked dependency.
type FaucetLimits struct {
	GrantUgnot int64 `json:"grant_ugnot"`
	PerAddress struct {
		Max           int `json:"max"`
		WindowSeconds int `json:"window_seconds"`
	} `json:"per_address"`
}

// FetchServiceLimits returns the per-address policy published by p's automatic
// faucet service, or (nil, nil) when p configures no such service. The body is
// untrusted (plain http permitted): the read is bounded and the result is the
// caller's to treat as best-effort.
func FetchServiceLimits(ctx context.Context, p profiles.Profile, httpClient *http.Client) (*FaucetLimits, error) {
	if p.FaucetServiceURL == "" {
		return nil, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.FaucetServiceURL+"/limits", nil)
	if err != nil {
		return nil, err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("faucet limits unreachable: %w", err)
	}
	defer resp.Body.Close()
	// A faucet that doesn't serve /limits (an older deploy, or a third-party
	// faucet) answers 404/405. That is "no policy advertised", not a failure:
	// return no limits so the caller omits the faucet block rather than reporting
	// an error. This keeps the client deploy-order-independent from the faucet.
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusMethodNotAllowed {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("faucet limits: unexpected status %s", resp.Status)
	}
	var lim FaucetLimits
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxFaucetRespBytes)).Decode(&lim); err != nil {
		return nil, fmt.Errorf("faucet limits: bad response: %w", err)
	}
	return &lim, nil
}

func (s *ServiceFaucet) Funded(ctx context.Context, address string) (bool, error) {
	bal, err := s.chain.Balance(ctx, address)
	if err != nil {
		return false, err
	}
	return bal > 0, nil
}
