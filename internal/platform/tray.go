package platform

type TrayActions struct {
	Open   func()
	Test   func()
	Pause  func()
	Report func()
	Update func()
	Quit   func()
}
