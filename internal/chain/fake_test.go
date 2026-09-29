package chain

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFake_Render(t *testing.T) {
	f := NewFake()
	f.SetRender("gno.land/r/foo", "", "# Hello\nThis is a realm.")
	got, err := f.Render(context.Background(), "gno.land/r/foo", "")
	require.NoError(t, err, "Render")
	assert.Equal(t, "# Hello\nThis is a realm.", got)
}

func TestFake_Render_unknownRealm(t *testing.T) {
	f := NewFake()
	_, err := f.Render(context.Background(), "gno.land/r/missing", "")
	require.Error(t, err, "expected error for unknown realm")
}

func TestFake_Render_pathMismatch(t *testing.T) {
	f := NewFake()
	f.SetRender("gno.land/r/foo", "page-a", "body A")
	_, err := f.Render(context.Background(), "gno.land/r/foo", "page-b")
	require.Error(t, err, "expected miss when realm matches but path does not")
}

func TestFake_Eval(t *testing.T) {
	f := NewFake()
	f.SetEval("gno.land/r/x", "Total()", "42")
	got, err := f.Eval(context.Background(), "gno.land/r/x", "Total()")
	require.NoError(t, err, "Eval")
	assert.Equal(t, "42", got)
}

func TestFake_File(t *testing.T) {
	f := NewFake()
	f.SetFile("gno.land/r/x", "x.gno", "package x\n")
	got, err := f.File(context.Background(), "gno.land/r/x", "x.gno")
	require.NoError(t, err, "File")
	assert.Equal(t, "package x\n", got)
}

func TestFake_ListFiles(t *testing.T) {
	f := NewFake()
	f.SetListing("gno.land/r/x", []string{"x.gno", "helper.gno"})
	got, err := f.ListFiles(context.Background(), "gno.land/r/x")
	require.NoError(t, err, "ListFiles")
	require.Len(t, got, 2)
	assert.Equal(t, "x.gno", got[0])
	assert.Equal(t, "helper.gno", got[1])
}

func TestFake_ListFiles_unknownRealm(t *testing.T) {
	f := NewFake()
	_, err := f.ListFiles(context.Background(), "gno.land/r/missing")
	require.Error(t, err, "expected error for unknown realm listing")
}

func TestFake_Doc(t *testing.T) {
	f := NewFake()
	f.SetDoc("gno.land/r/x", "package x // does things\nfunc Foo()")
	got, err := f.Doc(context.Background(), "gno.land/r/x")
	require.NoError(t, err, "Doc")
	assert.Equal(t, "package x // does things\nfunc Foo()", got)
}

// ---- signerStub for write-method tests (fakeSignerStub avoids redeclaring signerStub from types_test.go)

type fakeSignerStub struct{}

func (fakeSignerStub) Address() string               { return "g1stub" }
func (fakeSignerStub) Pubkey() []byte                { return make([]byte, 32) }
func (fakeSignerStub) Sign(_ []byte) ([]byte, error) { return nil, nil }

// ---- Call tests

func TestFake_Call_returnsSeededResult(t *testing.T) {
	f := NewFake()
	f.SetCallAsUser("gno.land/r/x", "Foo", []string{"hi"}, CallResult{TxHash: "0xabc", Result: "ok"})

	got, err := f.CallAsUser(context.Background(), fakeSignerStub{}, "", "gno.land/r/x", "Foo", []string{"hi"}, "", false)
	require.NoError(t, err, "CallAsUser")
	// A broadcast result carries the offered fee (the Fake's configured gas fee).
	want := CallResult{TxHash: "0xabc", Result: "ok", GasFeeUgnot: DefaultGasFeeUgnot}
	assert.Equal(t, want, got)
}

func TestFake_Call_unseededReturnsError(t *testing.T) {
	f := NewFake()
	_, err := f.CallAsUser(context.Background(), fakeSignerStub{}, "", "gno.land/r/x", "Bar", nil, "", false)
	require.Error(t, err, "expected error for unseeded call")
	assert.True(t, strings.Contains(err.Error(), "fake: no call"), "error should mention 'fake: no call', got: %v", err)
}

func TestFake_Call_simulateReturnsSeededWithSimulatedTrue(t *testing.T) {
	f := NewFake()
	f.SetCallAsUser("gno.land/r/x", "Foo", []string{"hi"}, CallResult{TxHash: "0xabc", Result: "ok", Simulated: false})

	got, err := f.CallAsUser(context.Background(), fakeSignerStub{}, "", "gno.land/r/x", "Foo", []string{"hi"}, "", true)
	require.NoError(t, err, "CallAsUser")
	assert.True(t, got.Simulated, "expected Simulated=true when simulate=true")
	assert.Equal(t, "0xabc", got.TxHash)
}

func TestFake_Call_setCallErrorTakesPriority(t *testing.T) {
	f := NewFake()
	f.SetCallAsUser("gno.land/r/x", "Foo", []string{}, CallResult{TxHash: "0xok"})
	f.SetCallAsUserError("gno.land/r/x", "Foo", ErrSimulateUnsupported)

	_, err := f.CallAsUser(context.Background(), fakeSignerStub{}, "", "gno.land/r/x", "Foo", []string{}, "", true)
	require.Error(t, err, "expected error from SetCallAsUserError")
	assert.True(t, errors.Is(err, ErrSimulateUnsupported), "error = %v, want ErrSimulateUnsupported", err)
}

// ---- Run tests

func TestFake_Run_returnsSeededResult(t *testing.T) {
	f := NewFake()
	f.SetRunAsUser("package main\nfunc main() {}", RunResult{TxHash: "0xdef", Output: "hello"})

	got, err := f.RunAsUser(context.Background(), fakeSignerStub{}, "", "package main\nfunc main() {}", false)
	require.NoError(t, err, "RunAsUser")
	// A broadcast result carries the offered fee (the Fake's configured gas fee).
	want := RunResult{TxHash: "0xdef", Output: "hello", GasFeeUgnot: DefaultGasFeeUgnot}
	assert.Equal(t, want, got)
}

func TestFake_Run_unseededReturnsError(t *testing.T) {
	f := NewFake()
	_, err := f.RunAsUser(context.Background(), fakeSignerStub{}, "", "package main\nfunc main() {}", false)
	require.Error(t, err, "expected error for unseeded run")
	assert.True(t, strings.Contains(err.Error(), "fake: no run"), "error should mention 'fake: no run', got: %v", err)
}

func TestFake_Run_setRunErrorTakesPriority(t *testing.T) {
	f := NewFake()
	code := "package main\nfunc main() {}"
	f.SetRunAsUser(code, RunResult{Output: "ok"})
	f.SetRunAsUserError(code, ErrSimulateUnsupported)

	_, err := f.RunAsUser(context.Background(), fakeSignerStub{}, "", code, true)
	require.Error(t, err, "expected error from SetRunAsUserError")
	assert.True(t, errors.Is(err, ErrSimulateUnsupported), "error = %v, want ErrSimulateUnsupported", err)
}

func TestFake_Run_simulateSetSimulatedTrue(t *testing.T) {
	f := NewFake()
	code := "package main\nfunc main() {}"
	f.SetRunAsUser(code, RunResult{Output: "hi", Simulated: false})

	got, err := f.RunAsUser(context.Background(), fakeSignerStub{}, "", code, true)
	require.NoError(t, err, "RunAsUser")
	assert.True(t, got.Simulated, "expected Simulated=true when simulate=true")
}

// ---- QuerySession tests

func TestFake_QuerySession_returnsSeededStatus(t *testing.T) {
	f := NewFake()
	want := SessionStatus{Active: true, AllowPaths: []string{"gno.land/r/x"}}
	f.SetSession("g1master", "g1session", want)

	got, err := f.QuerySession(context.Background(), "g1master", "g1session")
	require.NoError(t, err, "QuerySession")
	assert.True(t, got.Active)
	require.Len(t, got.AllowPaths, 1)
	assert.Equal(t, "gno.land/r/x", got.AllowPaths[0])
}

func TestFake_QuerySession_unknownReturnsInactive(t *testing.T) {
	f := NewFake()

	got, err := f.QuerySession(context.Background(), "g1master", "g1unknown")
	require.NoError(t, err, "QuerySession: unexpected error")
	assert.False(t, got.Active, "expected Active=false for unknown session")
}

// ---- Balance tests

func TestFake_Balance_seededAddress(t *testing.T) {
	f := NewFake()
	f.SetBalance("g1abc", 500)
	got, err := f.Balance(context.Background(), "g1abc")
	require.NoError(t, err, "Balance")
	require.Equal(t, int64(500), got)
}

func TestFake_Balance_unknownAddressIsZero(t *testing.T) {
	f := NewFake()
	got, err := f.Balance(context.Background(), "g1unknown")
	require.NoError(t, err, "Balance")
	require.Equal(t, int64(0), got, "unset must be 0")
}

func TestFake_ListPaths(t *testing.T) {
	f := NewFake()
	f.SetPaths("gno.land/r/demo/", []string{"gno.land/r/demo/foo", "gno.land/r/demo/bar"})

	got, err := f.ListPaths(context.Background(), "gno.land/r/demo/", 0)
	require.NoError(t, err)
	require.Equal(t, []string{"gno.land/r/demo/foo", "gno.land/r/demo/bar"}, got)

	_, err = f.ListPaths(context.Background(), "gno.land/r/unseeded/", 0)
	require.Error(t, err)
}

// ---- VM params and package meta

func TestFake_PackageMeta_unseededIsAbsent(t *testing.T) {
	f := NewFake()
	got, err := f.PackageMeta(context.Background(), "gno.land/r/x/never")
	require.NoError(t, err)
	assert.Equal(t, PackageAbsent, got.Status)
}

func TestFake_PackageMeta_sequenceThenLastSticks(t *testing.T) {
	f := NewFake()
	parked := PackageMeta{Status: PackageInert, Reason: "waiting", Pending: true}
	live := PackageMeta{Status: PackageLive}
	f.SetPackageMetaSequence("gno.land/r/x/y", parked, live)

	var got []string
	for range 3 {
		m, err := f.PackageMeta(context.Background(), "gno.land/r/x/y")
		require.NoError(t, err)
		got = append(got, m.Status)
	}
	assert.Equal(t, []string{PackageInert, PackageLive, PackageLive}, got)
	assert.Equal(t, 3, f.PackageMetaCalls("gno.land/r/x/y"))
}

func TestFake_PackageMeta_failFromACall(t *testing.T) {
	f := NewFake()
	f.SetPackageMetaSequence("gno.land/r/x/y", PackageMeta{Status: PackageInert, Pending: true})
	f.FailPackageMetaFrom("gno.land/r/x/y", 1, errors.New("rpc down"))

	_, err := f.PackageMeta(context.Background(), "gno.land/r/x/y")
	require.NoError(t, err, "call 0 answers")
	_, err = f.PackageMeta(context.Background(), "gno.land/r/x/y")
	require.Error(t, err, "call 1 onward fails")
}

// A plain error seeder fails every call, whatever an earlier
// FailPackageMetaFrom set.
func TestFake_PackageMeta_errAfterFailFromFailsFromTheFirstCall(t *testing.T) {
	f := NewFake()
	f.FailPackageMetaFrom("gno.land/r/x/y", 3, errors.New("later"))
	f.SetPackageMetaErr("gno.land/r/x/y", errors.New("now"))

	_, err := f.PackageMeta(context.Background(), "gno.land/r/x/y")
	require.Error(t, err)
}

func TestFake_PackageMeta_err(t *testing.T) {
	f := NewFake()
	f.SetPackageMetaErr("gno.land/r/x/y", errors.New("rpc down"))
	_, err := f.PackageMeta(context.Background(), "gno.land/r/x/y")
	require.Error(t, err)
}

func TestFake_SubmissionPolicy(t *testing.T) {
	f := NewFake()
	got, err := f.SubmissionPolicy(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got, "unset by default, like a chain predating the param")

	f.SetSubmissionPolicy(SubmissionPolicyInert)
	got, err = f.SubmissionPolicy(context.Background())
	require.NoError(t, err)
	assert.Equal(t, SubmissionPolicyInert, got)

	f.SetSubmissionPolicyErr(errors.New("rpc down"))
	_, err = f.SubmissionPolicy(context.Background())
	require.Error(t, err)
}

func TestFake_RunSubmitters(t *testing.T) {
	f := NewFake()
	got, err := f.RunSubmitters(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)

	f.SetRunSubmitters([]string{"g1allowed"})
	got, err = f.RunSubmitters(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{"g1allowed"}, got)

	f.SetRunSubmittersErr(errors.New("rpc down"))
	_, err = f.RunSubmitters(context.Background())
	require.Error(t, err)
}
