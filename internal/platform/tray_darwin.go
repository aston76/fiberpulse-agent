//go:build darwin

package platform

/*
#cgo LDFLAGS: -framework Cocoa
#include <stdlib.h>
void FiberPulseTrayRun(void);
void FiberPulseTrayStop(void);
void FiberPulseTraySetState(const char *version, int paused, const char *status, const char *available, const char *error);
int FiberPulseTrayPresentUpdate(const char *version, const char *status, const char *available, const char *error);
*/
import "C"

import (
	"runtime"
	"sync"
	"time"
	"unsafe"
)

var darwinTrayMu sync.RWMutex
var darwinTrayActions TrayActions

func init() {
	runtime.LockOSThread()
}

func RunTray(actions TrayActions, done <-chan struct{}) error {
	darwinTrayMu.Lock()
	darwinTrayActions = actions
	darwinTrayMu.Unlock()
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				C.FiberPulseTrayStop()
				return
			case <-ticker.C:
				refreshDarwinTrayState()
			}
		}
	}()
	refreshDarwinTrayState()
	C.FiberPulseTrayRun()
	return nil
}

//export fiberPulseTrayAction
func fiberPulseTrayAction(identifier C.int) {
	darwinTrayMu.RLock()
	actions := darwinTrayActions
	darwinTrayMu.RUnlock()
	var action func()
	switch int(identifier) {
	case 1:
		action = actions.Open
	case 2:
		action = actions.Test
	case 3:
		action = actions.Pause
	case 4:
		action = actions.Report
	case 5:
		action = actions.CheckUpdate
	case 6:
		action = actions.InstallUpdate
	case 7:
		action = actions.Quit
	}
	if action != nil {
		go action()
	}
}

func darwinState() TrayState {
	darwinTrayMu.RLock()
	actions := darwinTrayActions
	darwinTrayMu.RUnlock()
	if actions.State == nil {
		return TrayState{}
	}
	return actions.State()
}

func refreshDarwinTrayState() {
	state := darwinState()
	version := C.CString(state.Version)
	status := C.CString(state.UpdateStatus)
	available := C.CString(state.AvailableVersion)
	errorText := C.CString(state.UpdateError)
	defer C.free(unsafe.Pointer(version))
	defer C.free(unsafe.Pointer(status))
	defer C.free(unsafe.Pointer(available))
	defer C.free(unsafe.Pointer(errorText))
	paused := 0
	if state.Paused {
		paused = 1
	}
	C.FiberPulseTraySetState(version, C.int(paused), status, available, errorText)
}

func PresentUpdateResult(state TrayState) bool {
	version := C.CString(state.Version)
	status := C.CString(state.UpdateStatus)
	available := C.CString(state.AvailableVersion)
	errorText := C.CString(state.UpdateError)
	defer C.free(unsafe.Pointer(version))
	defer C.free(unsafe.Pointer(status))
	defer C.free(unsafe.Pointer(available))
	defer C.free(unsafe.Pointer(errorText))
	return C.FiberPulseTrayPresentUpdate(version, status, available, errorText) == 1
}
