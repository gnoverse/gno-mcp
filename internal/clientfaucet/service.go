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
		return Outcome{}, fmt.Errorf("faucet busy (rate-limited) — retry later")
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

func (s *ServiceFaucet) Funded(ctx context.Context, address string) (bool, error) {
	bal, err := s.chain.Balance(ctx, address)
	if err != nil {
		return false, err
	}
	return bal > 0, nil
}
