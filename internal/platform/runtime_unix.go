//go:build !windows

package platform

import (
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
	lockPath := filepath.Join(filepath.Dir(socketPath), "agent.lock")
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
		if err != nil {
			return err
		}
		err = unix.Flock(int(lock.Fd()), unix.LOCK_EX|unix.LOCK_NB)
		if err == nil {
			_ = unix.Flock(int(lock.Fd()), unix.LOCK_UN)
			_ = lock.Close()
			return nil
		}
		_ = lock.Close()
		time.Sleep(100 * time.Millisecond)
	}
	return errors.New("running FiberPulse did not stop within 10 seconds")
}
