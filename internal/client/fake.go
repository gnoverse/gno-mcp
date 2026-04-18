package client

import (
	"context"
	"fmt"
	"sync"
)

type Fake struct {
	mu        sync.Mutex
	Network   NetworkInfo
	Addresses map[string]*AddressInfo
	Realms    map[string]*RealmInspection
	Renders   map[string]string
	Sources   map[string]string
	Keys      map[string]string
	Calls     []CallRequest
}

func NewFake() *Fake {
	return &Fake{
		Network:   NetworkInfo{Chain: "test", Domain: "gno.land", RPC: "http://fake:26657", Height: 42},
		Addresses: map[string]*AddressInfo{},
		Realms:    map[string]*RealmInspection{},
		Renders:   map[string]string{},
		Sources:   map[string]string{},
		Keys:      map[string]string{},
	}
}

func (f *Fake) NetworkInfo(_ context.Context, domain string) (*NetworkInfo, error) {
	cp := f.Network
	if domain != "" {
		cp.Domain = domain
	}
	return &cp, nil
}

func (f *Fake) AddressInfo(_ context.Context, _, addr string) (*AddressInfo, error) {
	if a, ok := f.Addresses[addr]; ok {
		return a, nil
	}
	return &AddressInfo{Address: addr, Balance: "0ugnot"}, nil
}

func (f *Fake) Render(_ context.Context, _, path string) (string, error) {
	if r, ok := f.Renders[path]; ok {
		return r, nil
	}
	return "", fmt.Errorf("no render for %s", path)
}

func (f *Fake) Eval(_ context.Context, _, expr string) (string, error) {
	return "(\"fake result\" string)", nil
}

func (f *Fake) Read(_ context.Context, _, path, symbol, file string, _, _ int) (string, error) {
	key := path + "|" + symbol + "|" + file
	if src, ok := f.Sources[key]; ok {
		return src, nil
	}
	return "// fake source for " + key, nil
}

func (f *Fake) Inspect(_ context.Context, _, path string) (*RealmInspection, error) {
	if r, ok := f.Realms[path]; ok {
		return r, nil
	}
	return nil, fmt.Errorf("no inspection for %s", path)
}

func (f *Fake) Keygen(_ context.Context, name string) (string, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	addr := "g1fake" + name
	f.Keys[name] = addr
	return addr, "pub" + name, nil
}

func (f *Fake) FaucetRequest(_ context.Context, _, addr string) error {
	f.Addresses[addr] = &AddressInfo{Address: addr, Balance: "1000000000ugnot"}
	return nil
}

func (f *Fake) CallSimulate(_ context.Context, req CallRequest) (*CallResult, error) {
	return &CallResult{Simulated: true, GasEstimate: 120000, EstimatedCost: "0.000120 ugnot"}, nil
}

func (f *Fake) CallBroadcast(_ context.Context, req CallRequest) (*CallResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls = append(f.Calls, req)
	return &CallResult{Simulated: false, GasEstimate: 120000, EstimatedCost: "0.000120 ugnot", TxHash: "FAKEHASH", Height: 100}, nil
}
