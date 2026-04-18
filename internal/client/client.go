package client

import "context"

type NetworkInfo struct {
	Chain  string `json:"chain"`
	Domain string `json:"domain"`
	RPC    string `json:"rpc"`
	Height int64  `json:"height"`
}

type AddressInfo struct {
	Address  string      `json:"address"`
	Balance  string      `json:"balance"`
	Sequence int64       `json:"sequence"`
	Account  int64       `json:"account_number"`
	Txs      []TxSummary `json:"recent_txs,omitempty"`
}

type TxSummary struct {
	Hash   string `json:"hash"`
	Height int64  `json:"height"`
	Result string `json:"result"`
}

type RealmInspection struct {
	Path      string    `json:"path"`
	Files     []string  `json:"files"`
	Functions []FuncSig `json:"functions"`
	GnowebURL string    `json:"gnoweb_url"`
}

type FuncSig struct {
	Name   string   `json:"name"`
	Public bool     `json:"public"`
	Params []string `json:"params"`
	Return []string `json:"return"`
	Doc    string   `json:"doc,omitempty"`
}

type CallRequest struct {
	Network string
	Signer  string
	Path    string
	Func    string
	Args    []string
	Send    string
}

type CallResult struct {
	Simulated     bool   `json:"simulated"`
	GasEstimate   int64  `json:"gas_estimate"`
	EstimatedCost string `json:"estimated_cost"`
	TxHash        string `json:"tx_hash,omitempty"`
	Height        int64  `json:"height,omitempty"`
}

type GnopieClient interface {
	NetworkInfo(ctx context.Context, domain string) (*NetworkInfo, error)
	AddressInfo(ctx context.Context, network, addr string) (*AddressInfo, error)
	Render(ctx context.Context, network, path string) (string, error)
	Eval(ctx context.Context, network, expr string) (string, error)
	Read(ctx context.Context, network, path, symbol, file string, lineStart, lineEnd int) (string, error)
	Inspect(ctx context.Context, network, path string) (*RealmInspection, error)
	Keygen(ctx context.Context, name string) (addr, pubkey string, err error)
	FaucetRequest(ctx context.Context, network, addr string) error
	CallSimulate(ctx context.Context, req CallRequest) (*CallResult, error)
	CallBroadcast(ctx context.Context, req CallRequest) (*CallResult, error)
}
