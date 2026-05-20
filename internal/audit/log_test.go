package audit

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestLog_appendWritesJSONLine(t *testing.T) {
	var buf bytes.Buffer
	l := NewLog(&buf)
	if err := l.Append(Entry{
		Time:     time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC),
		Tool:     "gno_call",
		Profile:  "testnet5",
		Result:   "ok",
		Duration: 150 * time.Millisecond,
	}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	line := strings.TrimSpace(buf.String())
	var got Entry
	if err := json.Unmarshal([]byte(line), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Tool != "gno_call" || got.Profile != "testnet5" {
		t.Errorf("Entry mis-decoded: %+v", got)
	}
}

func TestLog_appendMultipleEntries(t *testing.T) {
	var buf bytes.Buffer
	l := NewLog(&buf)
	for i := 0; i < 3; i++ {
		if err := l.Append(Entry{Tool: "x", Result: "ok"}); err != nil {
			t.Fatal(err)
		}
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 {
		t.Errorf("expected 3 lines, got %d", len(lines))
	}
}

func TestLog_appendDefaultsTimeToNow(t *testing.T) {
	var buf bytes.Buffer
	l := NewLog(&buf)
	before := time.Now().UTC()
	if err := l.Append(Entry{Tool: "x", Result: "ok"}); err != nil {
		t.Fatal(err)
	}
	after := time.Now().UTC()
	var got Entry
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &got); err != nil {
		t.Fatal(err)
	}
	if got.Time.Before(before) || got.Time.After(after) {
		t.Errorf("Time not auto-defaulted to now: got %v, want in [%v, %v]", got.Time, before, after)
	}
}
