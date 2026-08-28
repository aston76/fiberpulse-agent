//go:build windows

package platform

import (
	"errors"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32                = windows.NewLazySystemDLL("kernel32.dll")
	user32                  = windows.NewLazySystemDLL("user32.dll")
	shell32                 = windows.NewLazySystemDLL("shell32.dll")
	procCreateMutex         = kernel32.NewProc("CreateMutexW")
	procCreateEvent         = kernel32.NewProc("CreateEventW")
	procOpenEvent           = kernel32.NewProc("OpenEventW")
	procSetEvent            = kernel32.NewProc("SetEvent")
	procWaitForSingleObject = kernel32.NewProc("WaitForSingleObject")
	procShellExecute        = shell32.NewProc("ShellExecuteW")
	procRegisterClass       = user32.NewProc("RegisterClassExW")
	procCreateWindow        = user32.NewProc("CreateWindowExW")
	procDefWindowProc       = user32.NewProc("DefWindowProcW")
	procGetMessage          = user32.NewProc("GetMessageW")
	procTranslateMessage    = user32.NewProc("TranslateMessage")
	procDispatchMessage     = user32.NewProc("DispatchMessageW")
	procPostQuitMessage     = user32.NewProc("PostQuitMessage")
	procPostMessage         = user32.NewProc("PostMessageW")
	procDestroyWindow       = user32.NewProc("DestroyWindow")
	procCreatePopupMenu     = user32.NewProc("CreatePopupMenu")
	procAppendMenu          = user32.NewProc("AppendMenuW")
	procTrackPopupMenu      = user32.NewProc("TrackPopupMenu")
	procGetCursorPos        = user32.NewProc("GetCursorPos")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	procLoadIcon            = user32.NewProc("LoadIconW")
	procShellNotifyIcon     = shell32.NewProc("Shell_NotifyIconW")
)

const (
	shutdownEventName = "Local\\FiberPulse-Shutdown-v1"
	instanceMutexName = "Local\\FiberPulse-Agent-v1"
)

type InstanceLock struct{ handle windows.Handle }

func AcquireSingleInstance(_ string) (*InstanceLock, error) {
	name, _ := windows.UTF16PtrFromString(instanceMutexName)
	h, _, err := procCreateMutex.Call(0, 0, uintptr(unsafe.Pointer(name)))
	if h == 0 {
		return nil, err
	}
	if windows.GetLastError() == windows.ERROR_ALREADY_EXISTS {
		windows.CloseHandle(windows.Handle(h))
		return nil, errors.New("FiberPulse is already running")
	}
	return &InstanceLock{windows.Handle(h)}, nil
}
func (l *InstanceLock) Close() error {
	if l == nil || l.handle == 0 {
		return nil
	}
	return windows.CloseHandle(l.handle)
}
func OpenURL(raw string) error {
	verb, _ := windows.UTF16PtrFromString("open")
	target, _ := windows.UTF16PtrFromString(raw)
	r, _, err := procShellExecute.Call(0, uintptr(unsafe.Pointer(verb)), uintptr(unsafe.Pointer(target)), 0, 0, 1)
	if r <= 32 {
		return err
	}
	return nil
}

const (
	wmUser         = 0x0400
	wmCommand      = 0x0111
	wmDestroy      = 0x0002
	wmClose        = 0x0010
	wmLButtonUp    = 0x0202
	wmRButtonUp    = 0x0205
	nimAdd         = 0
	nimDelete      = 2
	nifMessage     = 1
	nifIcon        = 2
	nifTip         = 4
	mfString       = 0
	tpmRightButton = 2
	cmdOpen        = 1001
	cmdTest        = 1002
	cmdPause       = 1003
	cmdReport      = 1004
	cmdUpdate      = 1005
	cmdQuit        = 1006
)

type point struct{ X, Y int32 }
type msg struct {
	Hwnd           uintptr
	Message        uint32
	WParam, LParam uintptr
	Time           uint32
	Pt             point
	Private        uint32
}
type wndClassEx struct {
	Size                               uint32
	Style                              uint32
	WndProc                            uintptr
	ClsExtra, WndExtra                 int32
	Instance, Icon, Cursor, Background uintptr
	MenuName, ClassName                *uint16
	IconSmall                          uintptr
}
type notifyIconData struct {
	Size                       uint32
	HWnd                       uintptr
	ID, Flags, CallbackMessage uint32
	Icon                       uintptr
	Tip                        [128]uint16
	State, StateMask           uint32
	Info                       [256]uint16
	TimeoutOrVersion           uint32
	InfoTitle                  [64]uint16
	InfoFlags                  uint32
	GUID                       [16]byte
	BalloonIcon                uintptr
}

var trayMu sync.Mutex
var trayCurrent TrayActions
var trayWindow uintptr
var trayReady chan error

func StartTray(actions TrayActions) (func(), error) {
	trayMu.Lock()
	trayCurrent = actions
	trayReady = make(chan error, 1)
	trayMu.Unlock()
	go trayLoop()
	if err := <-trayReady; err != nil {
		return nil, err
	}
	return func() {
		if trayWindow != 0 {
			procPostMessage.Call(trayWindow, wmClose, 0, 0)
		}
	}, nil
}
func trayLoop() {
	lockOSThread()
	className, _ := windows.UTF16PtrFromString("FiberPulseTrayWindow")
	instance, _, _ := kernel32.NewProc("GetModuleHandleW").Call(0)
	wc := wndClassEx{Size: uint32(unsafe.Sizeof(wndClassEx{})), WndProc: syscall.NewCallback(windowProc), Instance: instance, ClassName: className}
	if r, _, err := procRegisterClass.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
		trayReady <- err
		return
	}
	hwnd, _, err := procCreateWindow.Call(0, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(className)), 0, 0, 0, 0, 0, 0, 0, instance, 0)
	if hwnd == 0 {
		trayReady <- err
		return
	}
	trayWindow = hwnd
	icon, _, _ := procLoadIcon.Call(0, 32512)
	nid := notifyIconData{Size: uint32(unsafe.Sizeof(notifyIconData{})), HWnd: hwnd, ID: 1, Flags: nifMessage | nifIcon | nifTip, CallbackMessage: wmUser + 1, Icon: icon}
	copy(nid.Tip[:], syscall.StringToUTF16("FiberPulse — measured Internet performance"))
	if r, _, err := procShellNotifyIcon.Call(nimAdd, uintptr(unsafe.Pointer(&nid))); r == 0 {
		trayReady <- err
		return
	}
	trayReady <- nil
	var m msg
	for {
		r, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessage.Call(uintptr(unsafe.Pointer(&m)))
	}
	procShellNotifyIcon.Call(nimDelete, uintptr(unsafe.Pointer(&nid)))
	trayWindow = 0
}
func windowProc(hwnd uintptr, message uint32, wparam, lparam uintptr) uintptr {
	switch message {
	case wmClose:
		procDestroyWindow.Call(hwnd)
		return 0
	case wmCommand:
		invokeAction(int(wparam & 0xffff))
		return 0
	case wmUser + 1:
		if uint32(lparam) == wmLButtonUp {
			invokeAction(cmdOpen)
		} else if uint32(lparam) == wmRButtonUp {
			showMenu(hwnd)
		}
		return 0
	case wmDestroy:
		procPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := procDefWindowProc.Call(hwnd, uintptr(message), wparam, lparam)
	return r
}

func ShutdownRequests(_ string) (<-chan struct{}, func(), error) {
	name, err := windows.UTF16PtrFromString(shutdownEventName)
	if err != nil {
		return nil, nil, err
	}
	h, _, callErr := procCreateEvent.Call(0, 0, 0, uintptr(unsafe.Pointer(name)))
	if h == 0 {
		return nil, nil, callErr
	}
	requests := make(chan struct{}, 1)
	stop := make(chan struct{})
	go func() {
		for {
			r, _, _ := procWaitForSingleObject.Call(h, 500)
			if r == windows.WAIT_OBJECT_0 {
				select {
				case requests <- struct{}{}:
				default:
				}
			}
			select {
			case <-stop:
				return
			default:
			}
		}
	}()
	var once sync.Once
	cleanup := func() {
		once.Do(func() {
			close(stop)
			windows.CloseHandle(windows.Handle(h))
		})
	}
	return requests, cleanup, nil
}

func RequestShutdown(_ string) error {
	name, err := windows.UTF16PtrFromString(shutdownEventName)
	if err != nil {
		return err
	}
	const eventModifyState = 0x0002
	h, _, callErr := procOpenEvent.Call(eventModifyState, 0, uintptr(unsafe.Pointer(name)))
	if h == 0 {
		if callErr == windows.ERROR_FILE_NOT_FOUND {
			return nil
		}
		return callErr
	}
	defer windows.CloseHandle(windows.Handle(h))
	r, _, setErr := procSetEvent.Call(h)
	if r == 0 {
		return setErr
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		mutexName, _ := windows.UTF16PtrFromString(instanceMutexName)
		mutex, _, _ := procCreateMutex.Call(0, 0, uintptr(unsafe.Pointer(mutexName)))
		alreadyRunning := windows.GetLastError() == windows.ERROR_ALREADY_EXISTS
		if mutex != 0 {
			windows.CloseHandle(windows.Handle(mutex))
		}
		if !alreadyRunning {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return errors.New("running FiberPulse did not stop within 10 seconds")
}
func showMenu(hwnd uintptr) {
	menu, _, _ := procCreatePopupMenu.Call()
	items := []struct {
		id   int
		text string
	}{{cmdOpen, "Open FiberPulse"}, {cmdTest, "Run manual test"}, {cmdPause, "Pause / resume"}, {cmdReport, "Open reports"}, {cmdUpdate, "Check for update"}, {cmdQuit, "Quit completely"}}
	for _, item := range items {
		text, _ := windows.UTF16PtrFromString(item.text)
		procAppendMenu.Call(menu, mfString, uintptr(item.id), uintptr(unsafe.Pointer(text)))
	}
	var p point
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&p)))
	procSetForegroundWindow.Call(hwnd)
	procTrackPopupMenu.Call(menu, tpmRightButton, uintptr(p.X), uintptr(p.Y), 0, hwnd, 0)
}
func invokeAction(id int) {
	trayMu.Lock()
	a := trayCurrent
	trayMu.Unlock()
	var fn func()
	switch id {
	case cmdOpen:
		fn = a.Open
	case cmdTest:
		fn = a.Test
	case cmdPause:
		fn = a.Pause
	case cmdReport:
		fn = a.Report
	case cmdUpdate:
		fn = a.Update
	case cmdQuit:
		fn = a.Quit
	}
	if fn != nil {
		go fn()
	}
}
