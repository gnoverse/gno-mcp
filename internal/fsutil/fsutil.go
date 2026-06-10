// Package fsutil provides crash-safe atomic file writes for sensitive on-disk
// state (keys, sessions, config). A write either lands in full or not at all,
// so a crash or full disk never leaves a partial file behind.
package fsutil

import (
	"fmt"
	"os"
	"path/filepath"
)

// WriteFileAtomic writes data to path atomically, replacing any existing file.
// It writes a temp file in the same directory, fsyncs it, and renames it over
// path. The parent directory is created with 0700 if absent.
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	tmp, err := writeTemp(path, data, perm)
	if err != nil {
		return err
	}
	defer os.Remove(tmp) // no-op once the rename succeeds
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("fsutil: rename onto %s: %w", path, err)
	}
	return syncDir(filepath.Dir(path))
}

// WriteFileExclusive writes data to path atomically but fails with os.ErrExist
// if path already exists. Like WriteFileAtomic it never leaves a partial file;
// it hard-links the fully written temp file into place, so the create-or-fail
// decision is atomic against concurrent writers. Callers can match the
// already-exists case with errors.Is(err, os.ErrExist).
func WriteFileExclusive(path string, data []byte, perm os.FileMode) error {
	tmp, err := writeTemp(path, data, perm)
	if err != nil {
		return err
	}
	defer os.Remove(tmp)
	if err := os.Link(tmp, path); err != nil {
		return fmt.Errorf("fsutil: link onto %s: %w", path, err)
	}
	return syncDir(filepath.Dir(path))
}

// writeTemp creates a uniquely named temp file in path's directory, writes data
// with the given permissions, fsyncs, and returns the temp file name. The temp
// file is removed on any failure so nothing is left behind.
func writeTemp(path string, data []byte, perm os.FileMode) (string, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("fsutil: mkdir %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return "", fmt.Errorf("fsutil: create temp in %s: %w", dir, err)
	}
	name := tmp.Name()
	cleanup := func(wrap string, err error) (string, error) {
		tmp.Close()
		os.Remove(name)
		return "", fmt.Errorf("fsutil: %s: %w", wrap, err)
	}
	if err := tmp.Chmod(perm); err != nil {
		return cleanup("chmod temp", err)
	}
	if _, err := tmp.Write(data); err != nil {
		return cleanup("write temp", err)
	}
	if err := tmp.Sync(); err != nil {
		return cleanup("sync temp", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return "", fmt.Errorf("fsutil: close temp: %w", err)
	}
	return name, nil
}

// syncDir fsyncs a directory so that a rename or link into it survives a crash.
// Opening a directory for sync is not supported on every platform (e.g.
// Windows); there the open fails and the call is a no-op, since the file
// contents are already durable.
func syncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return nil
	}
	defer d.Close()
	if err := d.Sync(); err != nil {
		return fmt.Errorf("fsutil: sync dir %s: %w", dir, err)
	}
	return nil
}
