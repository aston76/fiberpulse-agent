//go:build darwin

package platform

/*
#cgo LDFLAGS: -framework Cocoa
void FiberPulseTrayRun(void);
void FiberPulseTrayStop(void);
*/
import "C"

import (
	"runtime"
	"sync"
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
		<-done
		C.FiberPulseTrayStop()
	}()
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
		action = actions.Update
	case 6:
		action = actions.Quit
	}
	if action != nil {
		go action()
	}
}
