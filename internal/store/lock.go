package store

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func (s *Store) lockPath(id string) string {
	return filepath.Join(s.instanceDir(id), "state.lock")
}

// tryLock attempts to create the lock file exclusively. It returns a release
// func on success, or an error if the lock is already held.
func (s *Store) tryLock(id string) (func(), error) {
	path := s.lockPath(id)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("instance %q is locked", id)
	}
	_ = f.Close()
	return func() { _ = os.Remove(path) }, nil
}

// Lock blocks (up to ~2s) until the instance lock can be acquired.
func (s *Store) Lock(id string) (func(), error) {
	deadline := time.Now().Add(2 * time.Second)
	for {
		release, err := s.tryLock(id)
		if err == nil {
			return release, nil
		}
		if time.Now().After(deadline) {
			return nil, err
		}
		time.Sleep(20 * time.Millisecond)
	}
}
