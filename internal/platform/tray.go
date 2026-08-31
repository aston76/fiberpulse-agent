package platform

type TrayActions struct {
	Open          func()
	Test          func()
	Pause         func()
	Report        func()
	CheckUpdate   func()
	InstallUpdate func()
	State         func() TrayState
	Quit          func()
}

type TrayState struct {
	Version          string
	Paused           bool
	UpdateStatus     string
	AvailableVersion string
	UpdateError      string
}
