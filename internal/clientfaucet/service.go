package clientfaucet

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gnoverse/gno-mcp/internal/chain"
)

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
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return Outcome{}, fmt.Errorf("faucet service error: %s: %s", resp.Status, strings.TrimSpace(string(b)))
	}
	var body struct {
		TxHash string `json:"tx_hash"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return Outcome{}, fmt.Errorf("faucet service: bad response: %w", err)
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
