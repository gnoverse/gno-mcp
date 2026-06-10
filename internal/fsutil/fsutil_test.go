package fsutil

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteFileAtomic_createsAndReplaces(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "f.bin")

	require.NoError(t, WriteFileAtomic(path, []byte("first"), 0o600))
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "first", string(got))

	require.NoError(t, WriteFileAtomic(path, []byte("second"), 0o600), "atomic write should replace")
	got, err = os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "second", string(got))
}

func TestWriteFileAtomic_permsAndParentDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission check not applicable on Windows")
	}
	path := filepath.Join(t.TempDir(), "nested", "f.bin")
	require.NoError(t, WriteFileAtomic(path, []byte("x"), 0o600))

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestWriteFileExclusive_failsWhenExists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f.bin")
	require.NoError(t, WriteFileExclusive(path, []byte("first"), 0o600))

	err := WriteFileExclusive(path, []byte("second"), 0o600)
	require.Error(t, err, "exclusive write over an existing file must fail")
	require.ErrorIs(t, err, os.ErrExist)

	// The original content must be untouched.
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "first", string(got), "existing file must not be overwritten")
}
