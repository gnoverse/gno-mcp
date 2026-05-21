package session

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func makeTestMeta(addr string) *SessionMeta {
	return &SessionMeta{
		Version:        1,
		SessionAddress: addr,
		SessionPubkey:  "gpub1test" + addr,
		Privkey:        []byte("fake-priv-key-32-bytes-padded!!!"),
		AllowPaths:     []string{"gno.land/r/test/blog"},
		SpendLimit:     "1000000ugnot",
		SpendRemaining: "1000000ugnot",
		ExpiresAt:      9999999999,
		CreatedAt:      1747833600,
	}
}

func TestStore_writeReadCycle(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir, "")

	m1 := makeTestMeta("g1aaaa")
	m2 := makeTestMeta("g1bbbb")

	if err := s.Write("p", m1); err != nil {
		t.Fatalf("Write m1: %v", err)
	}
	if err := s.Write("p", m2); err != nil {
		t.Fatalf("Write m2: %v", err)
	}

	got1, err := s.Read("p", "g1aaaa")
	if err != nil {
		t.Fatalf("Read g1aaaa: %v", err)
	}
	if got1.SessionAddress != m1.SessionAddress {
		t.Errorf("SessionAddress = %q, want %q", got1.SessionAddress, m1.SessionAddress)
	}
	if !bytes.Equal(got1.Privkey, m1.Privkey) {
		t.Errorf("Privkey mismatch after read")
	}

	got2, err := s.Read("p", "g1bbbb")
	if err != nil {
		t.Fatalf("Read g1bbbb: %v", err)
	}
	if got2.SessionAddress != m2.SessionAddress {
		t.Errorf("SessionAddress = %q, want %q", got2.SessionAddress, m2.SessionAddress)
	}
}

func TestStore_writeCreatesProfileDir(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir, "")

	if err := s.Write("myprofile", makeTestMeta("g1zzzz")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	profileDir := filepath.Join(dir, "myprofile")
	if _, err := os.Stat(profileDir); err != nil {
		t.Fatalf("expected profile dir %s to exist: %v", profileDir, err)
	}
}

func TestStore_listEmpty(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir, "")

	metas, err := s.List("no-such-profile")
	if err != nil {
		t.Fatalf("List on missing profile: %v", err)
	}
	if len(metas) != 0 {
		t.Fatalf("List on missing profile returned %d items, want 0", len(metas))
	}
}

func TestStore_listMultiple(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir, "")

	for _, addr := range []string{"g1aaa", "g1bbb", "g1ccc"} {
		if err := s.Write("prof", makeTestMeta(addr)); err != nil {
			t.Fatalf("Write %s: %v", addr, err)
		}
	}

	metas, err := s.List("prof")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(metas) != 3 {
		t.Fatalf("List returned %d items, want 3", len(metas))
	}
}

func TestStore_deleteRemovesFile(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir, "")

	m := makeTestMeta("g1del")
	if err := s.Write("p", m); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := s.Delete("p", "g1del"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err := s.Read("p", "g1del")
	if err == nil {
		t.Fatal("Read after Delete returned nil error, want error")
	}
}

func TestStore_deleteMissingIsNotError(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir, "")
	if err := s.Delete("p", "g1notexist"); err != nil {
		t.Fatalf("Delete missing file: %v", err)
	}
}

func TestStore_filePermsAre0600(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file permission check not applicable on Windows")
	}
	dir := t.TempDir()
	s := NewStore(dir, "")
	m := makeTestMeta("g1perms")
	if err := s.Write("p", m); err != nil {
		t.Fatalf("Write: %v", err)
	}
	path := filepath.Join(dir, "p", "g1perms.key")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("file mode = %04o, want 0600", got)
	}
}

func TestStore_masterAddressRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir, "")

	m := makeTestMeta("g1master")
	m.MasterAddress = "g17ernafy6ctpcz6uepfsq2js8x2vz0wladh5yc3"
	if err := s.Write("p", m); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, err := s.Read("p", "g1master")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got.MasterAddress != m.MasterAddress {
		t.Errorf("MasterAddress = %q, want %q", got.MasterAddress, m.MasterAddress)
	}
}

func TestStore_passphraseRoundTrip(t *testing.T) {
	dir := t.TempDir()
	passphrase := "correct-horse-battery-staple"
	s := NewStore(dir, passphrase)

	m := makeTestMeta("g1enc")
	if err := s.Write("p", m); err != nil {
		t.Fatalf("Write: %v", err)
	}

	path := filepath.Join(dir, "p", "g1enc.key")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if bytes.Contains(raw, m.Privkey) {
		t.Fatal("on-disk file contains plaintext privkey bytes; expected encrypted")
	}

	got, err := s.Read("p", "g1enc")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !bytes.Equal(got.Privkey, m.Privkey) {
		t.Fatalf("decrypted privkey mismatch:\n got  %x\n want %x", got.Privkey, m.Privkey)
	}
	if got.Encrypted {
		t.Fatal("Read returned meta with Encrypted=true; should be cleared after decryption")
	}
}

func TestStore_listSkipsCorruptFile(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir, "")

	if err := s.Write("p", makeTestMeta("g1valid")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	garbage := filepath.Join(dir, "p", "badname.key")
	if err := os.WriteFile(garbage, []byte("not json at all!!!"), 0600); err != nil {
		t.Fatalf("WriteFile garbage: %v", err)
	}

	metas, err := s.List("p")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(metas) != 1 {
		t.Fatalf("List returned %d items, want 1 (corrupt file should be skipped)", len(metas))
	}
	if metas[0].SessionAddress != "g1valid" {
		t.Fatalf("unexpected session addr %q", metas[0].SessionAddress)
	}
}
