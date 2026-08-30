//go:build unix

package update

import (
	"errors"
	"os/exec"
	"syscall"
)

// defaultProcessAlive reports whether pid currently refers to a live process
// owned by anyone (EPERM still proves existence).
func defaultProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

// DetachProcess configures the updater helper to run in its own session so
// it survives the quitting agent that spawned it.
func DetachProcess(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
