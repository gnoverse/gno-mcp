package audit

import (
	"path/filepath"
	"testing"
)

func TestAppendAndTail(t *testing.T) {
	dir := t.TempDir()
	l, err := Open(filepath.Join(dir, "audit.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	for i, tool := range []string{"gno_get", "gno_call", "gno_get"} {
		if err := l.Append(Entry{Tool: tool, Result: "ok", Args: map[string]any{"i": i}}); err != nil {
			t.Fatal(err)
		}
	}
	got, err := l.Tail(2)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[1].Tool != "gno_get" {
		t.Errorf("bad tail: %+v", got)
	}
}

func TestRedactArgs(t *testing.T) {
	in := map[string]any{"path": "gno.land/r/x", "password": "secret"}
	out := RedactArgs(in)
	if _, ok := out["password"]; ok {
		t.Error("password not redacted")
	}
	if out["path"] != "gno.land/r/x" {
		t.Error("path lost")
	}
}
