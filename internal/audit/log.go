// Package audit writes JSON-lines audit records of MCP tool invocations.
package audit

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"sync"
	"time"
)

// Entry is one audit record. Persisted as JSON lines.
type Entry struct {
	Time           time.Time `json:"time"`
	Tool           string    `json:"tool"`
	Profile        string    `json:"profile,omitempty"`
	ArgsSummary    string    `json:"args_summary,omitempty"`
	Result         string    `json:"result"`                    // ok / sim / sim_err / broadcast_err / tool_err
	Duration       int64     `json:"duration_ms"`               // milliseconds since the call started, populated via time.Duration.Milliseconds()
	SessionAddress string    `json:"session_address,omitempty"` // non-empty only for session-signed (act-as-user) writes
}

// Log writes audit entries as JSON lines to the underlying writer.
// Append is goroutine-safe.
type Log struct {
	w   io.Writer
	mu  sync.Mutex
	enc *json.Encoder
}

func NewLog(w io.Writer) *Log {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return &Log{w: w, enc: enc}
}

func (l *Log) Append(e Entry) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if e.Time.IsZero() {
		e.Time = time.Now().UTC()
	}
	if err := l.enc.Encode(e); err != nil {
		return fmt.Errorf("audit append: %w", err)
	}
	return nil
}

// Record appends e and logs a warning (to stderr via the standard logger) if the
// write fails. It is the best-effort variant for callers that must not abort the
// tool call on an audit failure but also must not let one pass silently — a
// missing audit record for a write is a security-relevant event.
func (l *Log) Record(e Entry) {
	if err := l.Append(e); err != nil {
		log.Printf("%v", err)
	}
}
