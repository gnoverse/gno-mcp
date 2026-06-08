package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/gnoverse/gno-mcp/internal/secret"
)

// SessionMeta is the on-disk + in-memory representation of a session.
type SessionMeta struct {
	Version        int      `json:"version"`
	SessionAddress string   `json:"session_address"`
	SessionPubkey  string   `json:"session_pubkey"`
	MasterAddress  string   `json:"master_address"` // bech32 master address the session was issued under; mirrored from Profile.MasterAddress at session creation
	Privkey        []byte   `json:"privkey"`
	Encrypted      bool     `json:"encrypted"`
	AllowPaths     []string `json:"allow_paths"`
	AllowRun       bool     `json:"allow_run"`
	SpendLimit     string   `json:"spend_limit"`
	SpendRemaining string   `json:"spend_remaining"`
	ExpiresAt      int64    `json:"expires_at"`
	CreatedAt      int64    `json:"created_at"`
	State          string   `json:"state"`
}

// Store persists SessionMeta values to the filesystem using a
// directory-per-profile, file-per-session layout with mode 0600.
type Store struct {
	rootDir    string
	passphrase string
}

// NewStore returns a Store rooted at rootDir. When passphrase is non-empty,
// the Privkey field is encrypted at write time and decrypted at read time.
func NewStore(rootDir, passphrase string) *Store {
	return &Store{rootDir: rootDir, passphrase: passphrase}
}

func (s *Store) profileDir(profile string) string {
	return filepath.Join(s.rootDir, profile)
}

func (s *Store) keyPath(profile, sessionAddr string) string {
	return filepath.Join(s.profileDir(profile), sessionAddr+".key")
}

// Write serialises meta to disk. Creates the profile dir if absent.
// When passphrase is set, encrypts Privkey before writing and sets Encrypted=true.
func (s *Store) Write(profile string, meta *SessionMeta) error {
	dir := s.profileDir(profile)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("session/store: mkdir %s: %w", dir, err)
	}

	m := *meta
	if s.passphrase != "" {
		encrypted, err := secret.Encrypt(m.Privkey, s.passphrase)
		if err != nil {
			return fmt.Errorf("session/store: encrypt privkey: %w", err)
		}
		m.Privkey = encrypted
		m.Encrypted = true
	} else {
		m.Encrypted = false
	}

	data, err := json.Marshal(&m)
	if err != nil {
		return fmt.Errorf("session/store: marshal meta: %w", err)
	}

	path := s.keyPath(profile, meta.SessionAddress)
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("session/store: write %s: %w", path, err)
	}
	return nil
}

// Read loads and decrypts the session at sessionAddr for the given profile.
func (s *Store) Read(profile, sessionAddr string) (*SessionMeta, error) {
	path := s.keyPath(profile, sessionAddr)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("session/store: read %s: %w", path, err)
	}
	return s.decode(data, path)
}

// Delete removes the session file. Missing file is not an error.
func (s *Store) Delete(profile, sessionAddr string) error {
	path := s.keyPath(profile, sessionAddr)
	err := os.Remove(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("session/store: delete %s: %w", path, err)
	}
	return nil
}

// List returns all valid sessions for the given profile. Files that fail
// to decode are logged to stderr and skipped.
func (s *Store) List(profile string) ([]*SessionMeta, error) {
	dir := s.profileDir(profile)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("session/store: read dir %s: %w", dir, err)
	}

	var metas []*SessionMeta
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".key") {
			continue
		}
		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			log.Printf("session/store: read %s: skipped: %v", path, err)
			continue
		}
		m, err := s.decode(data, path)
		if err != nil {
			log.Printf("session/store: decode %s: skipped: %v", path, err)
			continue
		}
		metas = append(metas, m)
	}
	return metas, nil
}

// listProfiles returns the names of all subdirectories in rootDir (each
// corresponds to a profile). Returns nil, nil when rootDir does not exist yet.
func (s *Store) listProfiles() ([]string, error) {
	entries, err := os.ReadDir(s.rootDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("list profiles: %w", err)
	}
	var ps []string
	for _, e := range entries {
		if e.IsDir() {
			ps = append(ps, e.Name())
		}
	}
	return ps, nil
}

func (s *Store) decode(data []byte, path string) (*SessionMeta, error) {
	var m SessionMeta
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("unmarshal %s: %w", path, err)
	}
	if m.Encrypted {
		plain, err := secret.Decrypt(m.Privkey, s.passphrase)
		if err != nil {
			return nil, fmt.Errorf("could not decrypt session file at %s: %w", path, err)
		}
		m.Privkey = plain
		m.Encrypted = false
	}
	return &m, nil
}
