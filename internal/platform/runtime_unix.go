//go:build !windows

package platform

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"golang.org/x/sys/unix"
)

func ShutdownPath(dataDir string) string {
	digest := sha256.Sum256([]byte(dataDir))
	return filepath.Join(os.TempDir(), "fiberpulse-"+hex.EncodeToString(digest[:8])+".sock")
}

type InstanceLock struct{ file *os.File }

func AcquireSingleInstance(path string) (*InstanceLock, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		f.Close()
		return nil, errors.New("FiberPulse is already running")
	}
	return &InstanceLock{file: f}, nil
}

func (l *InstanceLock) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	_ = unix.Flock(int(l.file.Fd()), unix.LOCK_UN)
	return l.file.Close()
}

func OpenURL(raw string) error {
	if runtime.GOOS == "darwin" {
		return exec.Command("open", raw).Start()
	}
	return exec.Command("xdg-open", raw).Start()
}

func ShutdownRequests(socketPath string) (<-chan struct{}, func(), error) {
	if socketPath == "" {
		return nil, nil, errors.New("shutdown socket path is required")
	}
	if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
		return nil, nil, err
	}
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, nil, err
	}
	_ = os.Chmod(socketPath, 0o600)
	requests := make(chan struct{}, 1)
	go func() {
		for {
			connection, err := listener.Accept()
			if err != nil {
				return
			}
			_ = connection.Close()
			select {
			case requests <- struct{}{}:
			default:
			}
		}
	}()
	var once sync.Once
	return requests, func() {
		once.Do(func() {
			_ = listener.Close()
			_ = os.Remove(socketPath)
		})
	}, nil
}

func RequestShutdown(socketPath string) error {
	connection, err := net.DialTimeout("unix", socketPath, time.Second)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	_ = connection.Close()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		_, err := os.Stat(socketPath)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		time.Sleep(100 * time.Millisecond)
	}
	return errors.New("running FiberPulse did not stop within 10 seconds")
}
