// Package objectstore provides a local-filesystem implementation of the
// types.ObjectStore interface for storing and retrieving binary objects.
package objectstore

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"abot/pkg/types"
)

// LocalStore implements types.ObjectStore using the local filesystem.
type LocalStore struct {
	root string
}

// NewLocalStore creates a LocalStore rooted at the given directory.
func NewLocalStore(root string) *LocalStore {
	return &LocalStore{root: root}
}

// sanitize validates that path stays within the store root.
// Rejects path traversal (../), absolute paths, and symlink escapes.
func (s *LocalStore) sanitize(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("objectstore: empty path")
	}
	if filepath.IsAbs(path) {
		return "", fmt.Errorf("objectstore: absolute path rejected: %s", path)
	}

	full := filepath.Join(s.root, filepath.Clean(path))
	absRoot, err := filepath.Abs(s.root)
	if err != nil {
		return "", fmt.Errorf("objectstore: resolve root: %w", err)
	}
	absFull, err := filepath.Abs(full)
	if err != nil {
		return "", fmt.Errorf("objectstore: resolve path: %w", err)
	}

	// Primary check: cleaned path must be under root.
	if !strings.HasPrefix(absFull, absRoot+string(filepath.Separator)) && absFull != absRoot {
		return "", fmt.Errorf("objectstore: path escapes root: %s", path)
	}

	// Secondary check: resolve symlinks on the path or its nearest existing ancestor.
	checkPath := full
	for checkPath != absRoot {
		real, err := filepath.EvalSymlinks(checkPath)
		if err != nil {
			// Path doesn't exist yet — try parent.
			checkPath = filepath.Dir(checkPath)
			continue
		}
		realRoot, _ := filepath.EvalSymlinks(s.root)
		if realRoot == "" {
			realRoot = absRoot
		}
		if !strings.HasPrefix(real, realRoot+string(filepath.Separator)) && real != realRoot {
			return "", fmt.Errorf("objectstore: symlink escapes root: %s", path)
		}
		break
	}

	return full, nil
}

func (s *LocalStore) Put(_ context.Context, path string, data io.Reader) error {
	full, err := s.sanitize(path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	f, err := os.Create(full)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, data)
	return err
}

func (s *LocalStore) Get(_ context.Context, path string) (io.ReadCloser, error) {
	full, err := s.sanitize(path)
	if err != nil {
		return nil, err
	}
	return os.Open(full)
}

func (s *LocalStore) Delete(_ context.Context, path string) error {
	full, err := s.sanitize(path)
	if err != nil {
		return err
	}
	return os.Remove(full)
}

func (s *LocalStore) Exists(_ context.Context, path string) (bool, error) {
	full, err := s.sanitize(path)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(full)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

var _ types.ObjectStore = (*LocalStore)(nil)
