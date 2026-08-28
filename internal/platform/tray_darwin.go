//go:build darwin

package platform

import (
	"github.com/getlantern/systray"
)

func RunTray(actions TrayActions, done <-chan struct{}) error {
	go func() {
		<-done
		systray.Quit()
	}()
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
		go routeMenu(open.ClickedCh, actions.Open)
		go routeMenu(test.ClickedCh, actions.Test)
		go routeMenu(pause.ClickedCh, actions.Pause)
		go routeMenu(report.ClickedCh, actions.Report)
		go routeMenu(update.ClickedCh, actions.Update)
		go routeMenu(quit.ClickedCh, actions.Quit)
	}, func() {})
	return nil
}

func routeMenu(clicks <-chan struct{}, action func()) {
	for range clicks {
		if action != nil {
			go action()
		}
	}
}
