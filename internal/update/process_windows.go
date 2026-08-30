//go:build windows

package update

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// defaultProcessAlive reports whether pid currently refers to a live process.
func defaultProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	_ = windows.CloseHandle(handle)
	return true
}

// DetachProcess configures the updater helper to run detached from the
// console of the quitting agent so the swap can proceed after it exits.
func DetachProcess(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x00000008 | 0x00000200} // DETACHED_PROCESS | CREATE_NEW_PROCESS_GROUP
}
