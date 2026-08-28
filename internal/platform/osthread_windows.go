//go:build windows

package platform

import "runtime"

func lockOSThread() { runtime.LockOSThread() }
