package profiles

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// DiscoverLocal probes a local-type profile's RPC /status endpoint.
// Returns ok=true iff the endpoint is reachable AND its reported network/chain-id matches the profile's chain-id.
// Non-local profiles always return ok=false without probing.
func DiscoverLocal(ctx context.Context, p Profile, timeout time.Duration) (bool, error) {
	if p.ChainType != ChainTypeLocal {
		return false, nil
	}
	url := p.RPCURL + "/status"
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false, fmt.Errorf("build status request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, nil // unreachable is not a hard error; profile just unavailable
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return false, nil
	}
	var body struct {
		Result struct {
			NodeInfo struct {
				Network string `json:"network"`
			} `json:"node_info"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return false, fmt.Errorf("decode status: %w", err)
	}
	return body.Result.NodeInfo.Network == p.ChainID, nil
}
