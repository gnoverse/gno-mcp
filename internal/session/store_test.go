package session

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	require.NoError(t, s.Write("p", m1), "Write m1")
	require.NoError(t, s.Write("p", m2), "Write m2")

	got1, err := s.Read("p", "g1aaaa")
	require.NoError(t, err, "Read g1aaaa")
	assert.Equal(t, m1.SessionAddress, got1.SessionAddress)
	assert.True(t, bytes.Equal(got1.Privkey, m1.Privkey), "Privkey mismatch after read")

	got2, err := s.Read("p", "g1bbbb")
	require.NoError(t, err, "Read g1bbbb")
	assert.Equal(t, m2.SessionAddress, got2.SessionAddress)
}

func TestStore_writeCreatesProfileDir(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir, "")

	require.NoError(t, s.Write("myprofile", makeTestMeta("g1zzzz")))

	profileDir := filepath.Join(dir, "myprofile")
	_, err := os.Stat(profileDir)
	require.NoError(t, err, "expected profile dir %s to exist", profileDir)
}

func TestStore_listEmpty(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir, "")

	metas, err := s.List("no-such-profile")
	require.NoError(t, err)
	assert.Empty(t, metas)
}

func TestStore_listMultiple(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir, "")

	for _, addr := range []string{"g1aaa", "g1bbb", "g1ccc"} {
		require.NoError(t, s.Write("prof", makeTestMeta(addr)), "Write %s", addr)
	}

	metas, err := s.List("prof")
	require.NoError(t, err)
	assert.Len(t, metas, 3)
}

func TestStore_deleteRemovesFile(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir, "")

	m := makeTestMeta("g1del")
	require.NoError(t, s.Write("p", m))
	require.NoError(t, s.Delete("p", "g1del"))

	_, err := s.Read("p", "g1del")
	require.Error(t, err, "Read after Delete returned nil error, want error")
}

func TestStore_deleteMissingIsNotError(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir, "")
	require.NoError(t, s.Delete("p", "g1notexist"))
}

func TestStore_filePermsAre0600(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file permission check not applicable on Windows")
	}
	dir := t.TempDir()
	s := NewStore(dir, "")
	m := makeTestMeta("g1perms")
	require.NoError(t, s.Write("p", m))

	path := filepath.Join(dir, "p", "g1perms.key")
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm(), "file mode should be 0600")
}

func TestStore_allowRunRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir, "")

	m := makeTestMeta("g1runallowed")
	m.AllowRun = true
	require.NoError(t, s.Write("p", m))

	got, err := s.Read("p", "g1runallowed")
	require.NoError(t, err)
	assert.True(t, got.AllowRun, "AllowRun should be true after round-trip")
}

func TestStore_masterAddressRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir, "")

	m := makeTestMeta("g1master")
	m.MasterAddress = "g17ernafy6ctpcz6uepfsq2js8x2vz0wladh5yc3"
	require.NoError(t, s.Write("p", m))

	got, err := s.Read("p", "g1master")
	require.NoError(t, err)
	assert.Equal(t, m.MasterAddress, got.MasterAddress)
}

func TestStore_passphraseRoundTrip(t *testing.T) {
	dir := t.TempDir()
	passphrase := "correct-horse-battery-staple"
	s := NewStore(dir, passphrase)

	m := makeTestMeta("g1enc")
	require.NoError(t, s.Write("p", m))

	path := filepath.Join(dir, "p", "g1enc.key")
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.False(t, bytes.Contains(raw, m.Privkey), "on-disk file contains plaintext privkey bytes; expected encrypted")

	got, err := s.Read("p", "g1enc")
	require.NoError(t, err)
	assert.True(t, bytes.Equal(got.Privkey, m.Privkey), "decrypted privkey mismatch")
	assert.False(t, got.Encrypted, "Read returned meta with Encrypted=true; should be cleared after decryption")
}

func TestStore_listSkipsCorruptFile(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir, "")

	require.NoError(t, s.Write("p", makeTestMeta("g1valid")))

	garbage := filepath.Join(dir, "p", "badname.key")
	require.NoError(t, os.WriteFile(garbage, []byte("not json at all!!!"), 0600))

	metas, err := s.List("p")
	require.NoError(t, err)
	require.Len(t, metas, 1, "corrupt file should be skipped")
	assert.Equal(t, "g1valid", metas[0].SessionAddress)
}
