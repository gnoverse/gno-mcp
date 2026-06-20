package chain

import (
	"context"
	"fmt"
	"strings"

	gnoclient "github.com/gnolang/gno/gno.land/pkg/gnoclient"
	"github.com/gnolang/gno/tm2/pkg/std"
)

// Fake is an in-memory Client implementation for use in unit tests.
// Not safe for concurrent use; tests should hold a Fake on a single goroutine.
type Fake struct {
	renders       map[string]string   // key: realm+"|"+path
	evals         map[string]string   // key: realm+"|"+expr
	files         map[string]string   // key: realm+"|"+file
	listings      map[string][]string // key: realm
	paths         map[string][]string // key: qpaths target (prefix or @namespace)
	docs          map[string]string   // key: realm
	calls         map[string]CallResult
	callErrors    map[string]error
	runs          map[string]RunResult
	runErrors     map[string]error
	sessions      map[string]SessionStatus // key: master+"|"+sessionAddr
	sessionErrors map[string]error         // key: master+"|"+sessionAddr; checked before sessions
	balances      map[string]int64         // key: bech32 addr; absent = 0 (never-funded)
	accounts      map[string]AccountInfo   // key: bech32 addr; absent = Exists=false
	accountErrors map[string]error         // key: bech32 addr; checked before accounts
	status        NodeStatus
	statusSet     bool
	statusErr     error // checked before status
	// agent-identity (standard tx, no session) maps
	agentCalls      map[string]CallResult
	agentRuns       map[string]RunResult
	addPkgs         map[string]AddPackageResult
	addPkgErrs      map[string]error          // key: deployPath; when set, AddPackage returns it (sim + broadcast)
	addPkgBcasts    map[string]int            // key: deployPath; count of broadcast (simulate=false) calls
	lastAddPkgFiles map[string][]*std.MemFile // key: deployPath; set on every AddPackage call
	lastSend        string                    // send arg of the most recent Call/CallAsUser
	lastAsUserMstr  string                    // master arg of the most recent CallAsUser/RunAsUser
	bankSends       []SendRecord              // every bank/MsgSend via Send, in order
	sendErr         error                     // when set, Send returns it
}

// SendRecord captures one bank/MsgSend made through the Fake (test introspection).
type SendRecord struct {
	To     string
	Amount int64
}

func NewFake() *Fake {
	return &Fake{
		renders:         map[string]string{},
		evals:           map[string]string{},
		files:           map[string]string{},
		listings:        map[string][]string{},
		paths:           map[string][]string{},
		docs:            map[string]string{},
		calls:           map[string]CallResult{},
		callErrors:      map[string]error{},
		runs:            map[string]RunResult{},
		runErrors:       map[string]error{},
		sessions:        map[string]SessionStatus{},
		sessionErrors:   map[string]error{},
		balances:        map[string]int64{},
		accounts:        map[string]AccountInfo{},
		accountErrors:   map[string]error{},
		agentCalls:      map[string]CallResult{},
		agentRuns:       map[string]RunResult{},
		addPkgs:         map[string]AddPackageResult{},
		addPkgErrs:      map[string]error{},
		addPkgBcasts:    map[string]int{},
		lastAddPkgFiles: map[string][]*std.MemFile{},
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

func (f *Fake) SetPaths(target string, paths []string) { f.paths[target] = paths }

func (f *Fake) ListPaths(_ context.Context, target string, _ int) ([]string, error) {
	v, ok := f.paths[target]
	if !ok {
		return nil, fmt.Errorf("fake: no paths for target=%q", target)
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

// CallAsUser records the master (so tool tests can assert it is sourced from the
// session, not the profile); the result map is keyed by (realm, fn, args) only.
func (f *Fake) CallAsUser(_ context.Context, _ Signer, master, realm, fn string, args []string, send string, simulate bool) (CallResult, error) {
	f.lastSend = send
	f.lastAsUserMstr = master
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

// RunAsUser records the master; the result map is keyed by code only.
func (f *Fake) RunAsUser(_ context.Context, _ Signer, master, code string, simulate bool) (RunResult, error) {
	f.lastAsUserMstr = master
	if err, ok := f.runErrors[code]; ok {
		return RunResult{}, err
	}
	r, ok := f.runs[code]
	if !ok {
		return RunResult{}, fmt.Errorf("fake: no run for code (%d chars)", len(code))
	}
	if simulate {
		r.Simulated = true
	}
	return r, nil
}

// QuerySession returns the seeded SessionStatus for (master, sessionAddr), or a
// seeded error (see SetSessionError) to exercise the query-failure path. If no
// seed is set, returns the zero value (Active=false) without error — matching
// chain semantics where an unknown session is "not found".
func (f *Fake) QuerySession(_ context.Context, master, sessionAddr string) (SessionStatus, error) {
	key := sessionKey(master, sessionAddr)
	if err := f.sessionErrors[key]; err != nil {
		return SessionStatus{}, err
	}
	return f.sessions[key], nil
}

// SetSessionError seeds an error returned by QuerySession for (master,
// sessionAddr). Use chain.ErrSessionQueryUnsupported (or an error wrapping it)
// to exercise the "preserve local state on a flake" path.
func (f *Fake) SetSessionError(master, sessionAddr string, err error) {
	f.sessionErrors[sessionKey(master, sessionAddr)] = err
}

func (f *Fake) SetCallAsUser(realm, fn string, args []string, result CallResult) {
	f.calls[callKey(realm, fn, args)] = result
}

func (f *Fake) SetCallAsUserError(realm, fn string, err error) {
	f.callErrors[callKey(realm, fn, nil)] = err
}

func (f *Fake) SetRunAsUser(code string, result RunResult) {
	f.runs[code] = result
}

// SetRunAsUserError seeds an error returned by RunAsUser for the given code string.
// Checked before the runs map. Use with ErrSimulateUnsupported to exercise
// the simulate_unsupported error path in tool tests.
func (f *Fake) SetRunAsUserError(code string, err error) {
	f.runErrors[code] = err
}

// SetSession seeds the SessionStatus returned by QuerySession for the given
// (master, sessionAddr) pair.
func (f *Fake) SetSession(master, sessionAddr string, status SessionStatus) {
	f.sessions[sessionKey(master, sessionAddr)] = status
}

// Call returns the seeded result for (realm, fn, args), ignoring signer.
func (f *Fake) Call(_ context.Context, _ gnoclient.Signer, realm, fn string, args []string, send string, simulate bool) (CallResult, error) {
	f.lastSend = send
	r, ok := f.agentCalls[callKey(realm, fn, args)]
	if !ok {
		return CallResult{}, fmt.Errorf("fake: no call for realm=%q fn=%q args=%v", realm, fn, args)
	}
	if simulate {
		r.Simulated = true
	}
	return r, nil
}

// Run returns the seeded result for the given code, ignoring signer.
func (f *Fake) Run(_ context.Context, _ gnoclient.Signer, code string, simulate bool) (RunResult, error) {
	r, ok := f.agentRuns[code]
	if !ok {
		return RunResult{}, fmt.Errorf("fake: no run for code (%d chars)", len(code))
	}
	if simulate {
		r.Simulated = true
	}
	return r, nil
}

// AddPackage returns the seeded result for deployPath, ignoring signer.
// Records files so addpkg tests can assert the file list.
func (f *Fake) AddPackage(_ context.Context, _ gnoclient.Signer, deployPath string, files []*std.MemFile, simulate bool) (AddPackageResult, error) {
	f.lastAddPkgFiles[deployPath] = files
	if !simulate {
		f.addPkgBcasts[deployPath]++
	}
	if err, ok := f.addPkgErrs[deployPath]; ok {
		return AddPackageResult{}, err
	}
	r, ok := f.addPkgs[deployPath]
	if !ok {
		return AddPackageResult{}, fmt.Errorf("fake: no addpackage for deployPath=%q", deployPath)
	}
	if simulate {
		r.Simulated = true
	}
	return r, nil
}

// SetAddPackageError makes AddPackage(deployPath, …) return err for both the
// validation (simulate) and broadcast calls — used to drive the
// validate-before-broadcast path in handler tests.
func (f *Fake) SetAddPackageError(deployPath string, err error) {
	f.addPkgErrs[deployPath] = err
}

// AddPackageBroadcasts reports how many times AddPackage was called with
// simulate=false for deployPath — 0 proves a deploy was never broadcast.
func (f *Fake) AddPackageBroadcasts(deployPath string) int {
	return f.addPkgBcasts[deployPath]
}

func (f *Fake) SetCall(realm, fn string, args []string, result CallResult) {
	f.agentCalls[callKey(realm, fn, args)] = result
}

// LastSend returns the send arg of the most recent Call/CallAsUser, for
// asserting that the tool plumbed attached coins through to the chain.
func (f *Fake) LastSend() string { return f.lastSend }

// LastAsUserMaster returns the master arg of the most recent CallAsUser/RunAsUser,
// for asserting it is sourced from the session record, not the profile.
func (f *Fake) LastAsUserMaster() string { return f.lastAsUserMstr }

func (f *Fake) SetRun(code string, result RunResult) {
	f.agentRuns[code] = result
}

func (f *Fake) SetAddPackage(deployPath string, result AddPackageResult) {
	f.addPkgs[deployPath] = result
}

// LastAddPackageFiles returns the files recorded by the most recent AddPackage
// call for deployPath (test introspection).
func (f *Fake) LastAddPackageFiles(deployPath string) []*std.MemFile {
	return f.lastAddPkgFiles[deployPath]
}

// Send records a bank/MsgSend and moves the seeded balances (debit from, credit
// to) so a follow-up Balance reflects the transfer. Returns the configured
// SetSendError, if any.
func (f *Fake) Send(_ context.Context, signer gnoclient.Signer, toAddr string, amountUgnot int64) (SendResult, error) {
	if f.sendErr != nil {
		return SendResult{}, f.sendErr
	}
	if amountUgnot <= 0 { // mirror Real.Send so the fake enforces the same contract
		return SendResult{}, fmt.Errorf("send: amount must be positive, got %d", amountUgnot)
	}
	f.bankSends = append(f.bankSends, SendRecord{To: toAddr, Amount: amountUgnot})
	if signer != nil {
		if info, err := signer.Info(); err == nil {
			f.balances[info.GetAddress().String()] -= amountUgnot
		}
	}
	f.balances[toAddr] += amountUgnot
	return SendResult{TxHash: "0xsend", Height: 1, GasUsed: 1}, nil
}

// SetSendError makes Send return err for all subsequent calls until reset (it is
// sticky, not one-shot).
func (f *Fake) SetSendError(err error) { f.sendErr = err }

// BankSends returns every bank/MsgSend recorded by Send, in order.
func (f *Fake) BankSends() []SendRecord { return f.bankSends }

// SetBalance seeds the ugnot balance for addr (used in tests to simulate a funded account).
func (f *Fake) SetBalance(addr string, ugnot int64) { f.balances[addr] = ugnot }

// Balance returns the seeded balance for addr; absent entries return 0 (never-funded).
func (f *Fake) Balance(_ context.Context, addr string) (int64, error) {
	return f.balances[addr], nil
}

// SetAccount seeds the AccountInfo returned by Account for addr.
func (f *Fake) SetAccount(addr string, info AccountInfo) { f.accounts[addr] = info }

// SetAccountError seeds an error returned by Account for addr (RPC-failure path).
func (f *Fake) SetAccountError(addr string, err error) { f.accountErrors[addr] = err }

// Account returns the seeded info for addr. An unseeded address returns the
// zero value (Exists=false) without error — matching chain semantics where an
// unknown address is "no record", not a failure.
func (f *Fake) Account(_ context.Context, addr string) (AccountInfo, error) {
	if err := f.accountErrors[addr]; err != nil {
		return AccountInfo{}, err
	}
	return f.accounts[addr], nil
}

// SetStatus seeds the NodeStatus returned by Status.
func (f *Fake) SetStatus(st NodeStatus) { f.status, f.statusSet = st, true }

// SetStatusError seeds an error returned by Status (RPC-failure path).
func (f *Fake) SetStatusError(err error) { f.statusErr = err }

// Status returns the seeded NodeStatus. Unlike Account, there is no valid
// "no status" answer from a live node, so an unseeded Fake errors.
func (f *Fake) Status(_ context.Context) (NodeStatus, error) {
	if f.statusErr != nil {
		return NodeStatus{}, f.statusErr
	}
	if !f.statusSet {
		return NodeStatus{}, fmt.Errorf("fake: no status seeded")
	}
	return f.status, nil
}

func callKey(realm, fn string, args []string) string {
	if args == nil {
		args = []string{}
	}
	return realm + "|" + fn + "|" + strings.Join(args, ",")
}

func sessionKey(master, sessionAddr string) string {
	return master + "|" + sessionAddr
}

// Assert Fake satisfies the interface at compile time.
var _ Client = (*Fake)(nil)
