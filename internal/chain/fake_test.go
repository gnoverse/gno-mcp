package chain

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestFake_Render(t *testing.T) {
	f := NewFake()
	f.SetRender("gno.land/r/foo", "", "# Hello\nThis is a realm.")
	got, err := f.Render(context.Background(), "gno.land/r/foo", "")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if got != "# Hello\nThis is a realm." {
		t.Errorf("Render = %q", got)
	}
}

func TestFake_Render_unknownRealm(t *testing.T) {
	f := NewFake()
	_, err := f.Render(context.Background(), "gno.land/r/missing", "")
	if err == nil {
		t.Fatal("expected error for unknown realm")
	}
}

func TestFake_Render_pathMismatch(t *testing.T) {
	f := NewFake()
	f.SetRender("gno.land/r/foo", "page-a", "body A")
	_, err := f.Render(context.Background(), "gno.land/r/foo", "page-b")
	if err == nil {
		t.Fatal("expected miss when realm matches but path does not")
	}
}

func TestFake_Eval(t *testing.T) {
	f := NewFake()
	f.SetEval("gno.land/r/x", "Total()", "42")
	got, err := f.Eval(context.Background(), "gno.land/r/x", "Total()")
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	if got != "42" {
		t.Errorf("Eval = %q", got)
	}
}

func TestFake_File(t *testing.T) {
	f := NewFake()
	f.SetFile("gno.land/r/x", "x.gno", "package x\n")
	got, err := f.File(context.Background(), "gno.land/r/x", "x.gno")
	if err != nil {
		t.Fatalf("File: %v", err)
	}
	if got != "package x\n" {
		t.Errorf("File = %q", got)
	}
}

func TestFake_ListFiles(t *testing.T) {
	f := NewFake()
	f.SetListing("gno.land/r/x", []string{"x.gno", "helper.gno"})
	got, err := f.ListFiles(context.Background(), "gno.land/r/x")
	if err != nil {
		t.Fatalf("ListFiles: %v", err)
	}
	if len(got) != 2 || got[0] != "x.gno" || got[1] != "helper.gno" {
		t.Errorf("ListFiles = %v", got)
	}
}

func TestFake_ListFiles_unknownRealm(t *testing.T) {
	f := NewFake()
	_, err := f.ListFiles(context.Background(), "gno.land/r/missing")
	if err == nil {
		t.Fatal("expected error for unknown realm listing")
	}
}

func TestFake_Doc(t *testing.T) {
	f := NewFake()
	f.SetDoc("gno.land/r/x", "package x // does things\nfunc Foo()")
	got, err := f.Doc(context.Background(), "gno.land/r/x")
	if err != nil {
		t.Fatalf("Doc: %v", err)
	}
	if got != "package x // does things\nfunc Foo()" {
		t.Errorf("Doc = %q", got)
	}
}

// ---- signerStub for write-method tests (fakeSignerStub avoids redeclaring signerStub from types_test.go)

type fakeSignerStub struct{}

func (fakeSignerStub) Address() string               { return "g1stub" }
func (fakeSignerStub) Sign(_ []byte) ([]byte, error) { return nil, nil }

// ---- Call tests

func TestFake_Call_returnsSeededResult(t *testing.T) {
	f := NewFake()
	want := CallResult{TxHash: "0xabc", Result: "ok"}
	f.SetCall("gno.land/r/x", "Foo", []string{"hi"}, want)

	got, err := f.Call(context.Background(), fakeSignerStub{}, "gno.land/r/x", "Foo", []string{"hi"}, false)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if got != want {
		t.Errorf("Call = %+v, want %+v", got, want)
	}
}

func TestFake_Call_unseededReturnsError(t *testing.T) {
	f := NewFake()
	_, err := f.Call(context.Background(), fakeSignerStub{}, "gno.land/r/x", "Bar", nil, false)
	if err == nil {
		t.Fatal("expected error for unseeded call")
	}
	if !strings.Contains(err.Error(), "fake: no call") {
		t.Errorf("error should mention 'fake: no call', got: %v", err)
	}
}

func TestFake_Call_simulateReturnsSeededWithSimulatedTrue(t *testing.T) {
	f := NewFake()
	f.SetCall("gno.land/r/x", "Foo", []string{"hi"}, CallResult{TxHash: "0xabc", Result: "ok", Simulated: false})

	got, err := f.Call(context.Background(), fakeSignerStub{}, "gno.land/r/x", "Foo", []string{"hi"}, true)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if !got.Simulated {
		t.Error("expected Simulated=true when simulate=true")
	}
	if got.TxHash != "0xabc" {
		t.Errorf("TxHash = %q, want 0xabc", got.TxHash)
	}
}

func TestFake_Call_setCallErrorTakesPriority(t *testing.T) {
	f := NewFake()
	f.SetCall("gno.land/r/x", "Foo", []string{}, CallResult{TxHash: "0xok"})
	f.SetCallError("gno.land/r/x", "Foo", ErrSimulateUnsupported)

	_, err := f.Call(context.Background(), fakeSignerStub{}, "gno.land/r/x", "Foo", []string{}, true)
	if err == nil {
		t.Fatal("expected error from SetCallError")
	}
	if !errors.Is(err, ErrSimulateUnsupported) {
		t.Errorf("error = %v, want ErrSimulateUnsupported", err)
	}
}

// ---- Run tests

func TestFake_Run_returnsSeededResult(t *testing.T) {
	f := NewFake()
	want := RunResult{TxHash: "0xdef", Output: "hello"}
	f.SetRun("package main\nfunc main() {}", want)

	got, err := f.Run(context.Background(), fakeSignerStub{}, "package main\nfunc main() {}", false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got != want {
		t.Errorf("Run = %+v, want %+v", got, want)
	}
}

func TestFake_Run_unseededReturnsError(t *testing.T) {
	f := NewFake()
	_, err := f.Run(context.Background(), fakeSignerStub{}, "package main\nfunc main() {}", false)
	if err == nil {
		t.Fatal("expected error for unseeded run")
	}
	if !strings.Contains(err.Error(), "fake: no run") {
		t.Errorf("error should mention 'fake: no run', got: %v", err)
	}
}

func TestFake_Run_simulateSetSimulatedTrue(t *testing.T) {
	f := NewFake()
	code := "package main\nfunc main() {}"
	f.SetRun(code, RunResult{Output: "hi", Simulated: false})

	got, err := f.Run(context.Background(), fakeSignerStub{}, code, true)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !got.Simulated {
		t.Error("expected Simulated=true when simulate=true")
	}
}

// ---- QuerySession tests

func TestFake_QuerySession_returnsSeededStatus(t *testing.T) {
	f := NewFake()
	want := SessionStatus{Active: true, AllowPaths: []string{"gno.land/r/x"}}
	f.SetSession("gpub1abc", want)

	got, err := f.QuerySession(context.Background(), "gpub1abc")
	if err != nil {
		t.Fatalf("QuerySession: %v", err)
	}
	if !got.Active || len(got.AllowPaths) != 1 || got.AllowPaths[0] != "gno.land/r/x" {
		t.Errorf("QuerySession = %+v, want %+v", got, want)
	}
}

func TestFake_QuerySession_unknownReturnsInactive(t *testing.T) {
	f := NewFake()

	got, err := f.QuerySession(context.Background(), "gpub1unknown")
	if err != nil {
		t.Fatalf("QuerySession: unexpected error %v", err)
	}
	if got.Active {
		t.Error("expected Active=false for unknown pubkey")
	}
}
