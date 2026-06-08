package audit

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLog_appendWritesJSONLine(t *testing.T) {
	var buf bytes.Buffer
	l := NewLog(&buf)
	err := l.Append(Entry{
		Time:     time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC),
		Tool:     "gno_call",
		Profile:  "testnet5",
		Result:   "ok",
		Duration: 150,
	})
	require.NoError(t, err, "Append")
	line := strings.TrimSpace(buf.String())
	var got Entry
	require.NoError(t, json.Unmarshal([]byte(line), &got), "unmarshal")
	assert.Equal(t, "gno_call", got.Tool, "Entry mis-decoded: %+v", got)
	assert.Equal(t, "testnet5", got.Profile, "Entry mis-decoded: %+v", got)
}

func TestLog_appendMultipleEntries(t *testing.T) {
	var buf bytes.Buffer
	l := NewLog(&buf)
	for i := 0; i < 3; i++ {
		require.NoError(t, l.Append(Entry{Tool: "x", Result: "ok"}))
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	assert.Len(t, lines, 3)
}

func TestLog_appendDefaultsTimeToNow(t *testing.T) {
	var buf bytes.Buffer
	l := NewLog(&buf)
	before := time.Now().UTC()
	require.NoError(t, l.Append(Entry{Tool: "x", Result: "ok"}))
	after := time.Now().UTC()
	var got Entry
	require.NoError(t, json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &got))
	assert.False(t, got.Time.Before(before) || got.Time.After(after),
		"Time not auto-defaulted to now: got %v, want in [%v, %v]", got.Time, before, after)
}

func TestLog_durationMarshalsAsMilliseconds(t *testing.T) {
	var buf bytes.Buffer
	l := NewLog(&buf)
	require.NoError(t, l.Append(Entry{Tool: "x", Result: "ok", Duration: 150}))
	assert.True(t, bytes.Contains(buf.Bytes(), []byte(`"duration_ms":150`)),
		"expected duration_ms=150 on wire, got: %s", buf.String())
	// Catch the regression: nanoseconds would surface as a 9-digit number.
	assert.False(t, bytes.Contains(buf.Bytes(), []byte(`"duration_ms":150000000`)),
		"duration_ms is being written in nanoseconds, not milliseconds")
}
