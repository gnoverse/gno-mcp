package chain

import (
	"context"
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
