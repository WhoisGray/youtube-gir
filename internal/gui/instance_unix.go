//go:build unix

package gui

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"syscall"
)

// ClaimRunning takes the single-instance lock for dataDir.
// already is true when another process holds it. release drops the lock.
func ClaimRunning(dataDir string) (release func(), already bool, err error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, false, err
	}
	f, err := os.OpenFile(filepath.Join(dataDir, "gui.lock"), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, false, err
	}
	err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
		_ = f.Close()
		return func() {}, true, nil
	}
	if err != nil {
		_ = f.Close()
		return nil, false, err
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
			_ = f.Close()
		})
	}, false, nil
}
