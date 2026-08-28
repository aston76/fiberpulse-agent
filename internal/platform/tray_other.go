//go:build !windows && !darwin

package platform

func StartTray(TrayActions) (func(), error) { return func() {}, nil }

func RunTray(TrayActions, <-chan struct{}) error { return nil }
