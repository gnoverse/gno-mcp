package audit

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Entry struct {
	Time    time.Time      `json:"time"`
	Tool    string         `json:"tool"`
	Network string         `json:"network,omitempty"`
	Signer  string         `json:"signer,omitempty"`
	TxHash  string         `json:"tx_hash,omitempty"`
	Result  string         `json:"result"`
	Args    map[string]any `json:"args,omitempty"`
}

type Log struct {
	mu   sync.Mutex
	path string
}

func Default() (*Log, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return Open(filepath.Join(home, ".gno-mcp", "audit.jsonl"))
}

func Open(path string) (*Log, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	return &Log{path: path}, nil
}

func (l *Log) Append(e Entry) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(e)
}

func (l *Log) Tail(n int) ([]Entry, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	f, err := os.Open(l.path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var all []Entry
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var e Entry
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			continue
		}
		all = append(all, e)
	}
	if n > 0 && len(all) > n {
		all = all[len(all)-n:]
	}
	return all, sc.Err()
}

// RedactArgs removes sensitive fields before logging.
func RedactArgs(in map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range in {
		switch k {
		case "password", "mnemonic", "private_key":
			continue
		}
		out[k] = v
	}
	return out
}
