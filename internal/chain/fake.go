package chain

import (
	"context"
	"fmt"
	"strings"
)

// Fake is an in-memory Client implementation for use in unit tests.
// Not safe for concurrent use; tests should hold a Fake on a single goroutine.
type Fake struct {
	renders    map[string]string   // key: realm+"|"+path
	evals      map[string]string   // key: realm+"|"+expr
	files      map[string]string   // key: realm+"|"+file
	listings   map[string][]string // key: realm
	docs       map[string]string   // key: realm
	calls      map[string]CallResult
	callErrors map[string]error
	runs       map[string]RunResult
	sessions   map[string]SessionStatus
}

func NewFake() *Fake {
	return &Fake{
		renders:    map[string]string{},
		evals:      map[string]string{},
		files:      map[string]string{},
		listings:   map[string][]string{},
		docs:       map[string]string{},
		calls:      map[string]CallResult{},
		callErrors: map[string]error{},
		runs:       map[string]RunResult{},
		sessions:   map[string]SessionStatus{},
	}
}

func (f *Fake) SetRender(realm, path, body string)      { f.renders[realm+"|"+path] = body }
func (f *Fake) SetEval(realm, expr, result string)      { f.evals[realm+"|"+expr] = result }
func (f *Fake) SetFile(realm, file, body string)        { f.files[realm+"|"+file] = body }
func (f *Fake) SetListing(realm string, files []string) { f.listings[realm] = files }
func (f *Fake) SetDoc(realm, doc string)                { f.docs[realm] = doc }

func (f *Fake) Render(_ context.Context, realm, path string) (string, error) {
	v, ok := f.renders[realm+"|"+path]
	if !ok {
		return "", fmt.Errorf("fake: no render for realm=%q path=%q", realm, path)
	}
	return v, nil
}

func (f *Fake) Eval(_ context.Context, realm, expr string) (string, error) {
	v, ok := f.evals[realm+"|"+expr]
	if !ok {
		return "", fmt.Errorf("fake: no eval for realm=%q expr=%q", realm, expr)
	}
	return v, nil
}

func (f *Fake) File(_ context.Context, realm, file string) (string, error) {
	v, ok := f.files[realm+"|"+file]
	if !ok {
		return "", fmt.Errorf("fake: no file for realm=%q file=%q", realm, file)
	}
	return v, nil
}

func (f *Fake) ListFiles(_ context.Context, realm string) ([]string, error) {
	v, ok := f.listings[realm]
	if !ok {
		return nil, fmt.Errorf("fake: no listing for realm=%q", realm)
	}
	return v, nil
}

func (f *Fake) Doc(_ context.Context, realm string) (string, error) {
	v, ok := f.docs[realm]
	if !ok {
		return "", fmt.Errorf("fake: no doc for realm=%q", realm)
	}
	return v, nil
}

func (f *Fake) Call(_ context.Context, _ Signer, realm, fn string, args []string, simulate bool) (CallResult, error) {
	if err, ok := f.callErrors[callKey(realm, fn, nil)]; ok {
		return CallResult{}, err
	}
	r, ok := f.calls[callKey(realm, fn, args)]
	if !ok {
		return CallResult{}, fmt.Errorf("fake: no call for realm=%q fn=%q args=%v", realm, fn, args)
	}
	if simulate {
		r.Simulated = true
	}
	return r, nil
}

func (f *Fake) Run(_ context.Context, _ Signer, code string, simulate bool) (RunResult, error) {
	r, ok := f.runs[code]
	if !ok {
		return RunResult{}, fmt.Errorf("fake: no run for code (%d chars)", len(code))
	}
	if simulate {
		r.Simulated = true
	}
	return r, nil
}

// QuerySession returns the seeded SessionStatus for pubkey. If no seed is set,
// it returns the zero value (Active=false) without error — matching chain semantics
// where an unknown pubkey returns "not found" rather than an error.
func (f *Fake) QuerySession(_ context.Context, pubkey string) (SessionStatus, error) {
	return f.sessions[pubkey], nil
}

func (f *Fake) SetCall(realm, fn string, args []string, result CallResult) {
	f.calls[callKey(realm, fn, args)] = result
}

func (f *Fake) SetCallError(realm, fn string, err error) {
	f.callErrors[callKey(realm, fn, nil)] = err
}

func (f *Fake) SetRun(code string, result RunResult) {
	f.runs[code] = result
}

func (f *Fake) SetSession(pubkey string, status SessionStatus) {
	f.sessions[pubkey] = status
}

func callKey(realm, fn string, args []string) string {
	return realm + "|" + fn + "|" + strings.Join(args, ",")
}

// Assert Fake satisfies the interface at compile time.
var _ Client = (*Fake)(nil)
