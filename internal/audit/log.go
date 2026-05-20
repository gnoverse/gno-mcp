// Package audit writes JSON-lines audit records of MCP tool invocations.
package audit

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"
)

// Entry is one audit record. Persisted as JSON lines.
type Entry struct {
	Time           time.Time     `json:"time"`
	Tool           string        `json:"tool"`
	Profile        string        `json:"profile,omitempty"`
	ArgsSummary    string        `json:"args_summary,omitempty"`
	Result         string        `json:"result"` // ok / sim / sim_err / broadcast_err / tool_err
	Duration       time.Duration `json:"duration_ms"`
	SessionAddress string        `json:"session_address,omitempty"` // Milestone B
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
