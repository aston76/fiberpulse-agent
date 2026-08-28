//go:build darwin

package platform

import (
	"errors"
	"sync"
	"time"

	"github.com/getlantern/systray"
)

func StartTray(actions TrayActions) (func(), error) {
	ready := make(chan struct{})
	exited := make(chan struct{})
	go func() {
		systray.Run(func() {
			systray.SetTitle("FP")
			systray.SetTooltip("FiberPulse — measured Internet performance")
			open := systray.AddMenuItem("Open FiberPulse", "Open the local dashboard")
			test := systray.AddMenuItem("Run manual test", "Run a consented NDT7 test")
			pause := systray.AddMenuItem("Pause / resume", "Pause or resume automatic tests")
			report := systray.AddMenuItem("Open reports", "Open local reporting")
			update := systray.AddMenuItem("Check for update", "Check the signed update channel")
			systray.AddSeparator()
			quit := systray.AddMenuItem("Quit completely", "Stop FiberPulse and all local listeners")
			close(ready)
			go routeMenu(open.ClickedCh, actions.Open)
			go routeMenu(test.ClickedCh, actions.Test)
			go routeMenu(pause.ClickedCh, actions.Pause)
			go routeMenu(report.ClickedCh, actions.Report)
			go routeMenu(update.ClickedCh, actions.Update)
			go routeMenu(quit.ClickedCh, actions.Quit)
		}, func() { close(exited) })
	}()
	select {
	case <-ready:
	case <-time.After(5 * time.Second):
		return nil, errors.New("macOS menu bar did not initialize")
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			systray.Quit()
			select {
			case <-exited:
			case <-time.After(2 * time.Second):
			}
		})
	}, nil
}

func routeMenu(clicks <-chan struct{}, action func()) {
	for range clicks {
		if action != nil {
			go action()
		}
	}
}
