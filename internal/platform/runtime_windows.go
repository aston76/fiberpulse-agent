//go:build windows

package platform

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	gdi32                   = windows.NewLazySystemDLL("gdi32.dll")
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
	procLoadImage           = user32.NewProc("LoadImageW")
	procMessageBox          = user32.NewProc("MessageBoxW")
	procDestroyIcon         = user32.NewProc("DestroyIcon")
	procGetDC               = user32.NewProc("GetDC")
	procReleaseDC           = user32.NewProc("ReleaseDC")
	procDrawIconEx          = user32.NewProc("DrawIconEx")
	procCreateIconIndirect  = user32.NewProc("CreateIconIndirect")
	procShellNotifyIcon     = shell32.NewProc("Shell_NotifyIconW")
	procCreateCompatibleDC  = gdi32.NewProc("CreateCompatibleDC")
	procCreateDIBSection    = gdi32.NewProc("CreateDIBSection")
	procCreateBitmap        = gdi32.NewProc("CreateBitmap")
	procSelectObject        = gdi32.NewProc("SelectObject")
	procDeleteObject        = gdi32.NewProc("DeleteObject")
	procDeleteDC            = gdi32.NewProc("DeleteDC")
)

const (
	shutdownEventName = "Local\\FiberPulse-Shutdown-v1"
	instanceMutexName = "Local\\FiberPulse-Agent-v1"
)

type InstanceLock struct{ handle windows.Handle }

func ShutdownPath(string) string { return "" }

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
	nimModify      = 1
	nimDelete      = 2
	nifMessage     = 1
	nifIcon        = 2
	nifTip         = 4
	mfString       = 0
	mfDisabled     = 0x0002
	mfSeparator    = 0x0800
	tpmRightButton = 2
	cmdOpen        = 1001
	cmdTest        = 1002
	cmdPause       = 1003
	cmdReport      = 1004
	cmdUpdate      = 1005
	cmdInstall     = 1006
	cmdQuit        = 1007
	imageIcon      = 1
	lrLoadFromFile = 0x0010
	mbOK           = 0x00000000
	mbYesNo        = 0x00000004
	mbIconInfo     = 0x00000040
	mbIconError    = 0x00000010
	idYes          = 6
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
type bitmapInfoHeader struct {
	Size                  uint32
	Width, Height         int32
	Planes, BitCount      uint16
	Compression           uint32
	SizeImage             uint32
	XPels, YPels          int32
	ClrUsed, ClrImportant uint32
}
type bitmapInfo struct {
	Header bitmapInfoHeader
	Colors [1]uint32
}
type iconInfo struct {
	Icon               int32
	XHotspot, YHotspot uint32
	Mask, Color        uintptr
}

var trayMu sync.Mutex
var trayCurrent TrayActions
var trayWindow uintptr
var trayReady chan error
var trayNID notifyIconData
var trayIcon uintptr
var trayNotificationIcon uintptr
var trayShowingNotification bool
var trayPollStop chan struct{}

func StartTray(actions TrayActions) (func(), error) {
	trayMu.Lock()
	trayCurrent = actions
	trayReady = make(chan error, 1)
	trayPollStop = make(chan struct{})
	trayMu.Unlock()
	go trayLoop()
	if err := <-trayReady; err != nil {
		return nil, err
	}
	return func() {
		trayMu.Lock()
		if trayPollStop != nil {
			close(trayPollStop)
			trayPollStop = nil
		}
		trayMu.Unlock()
		if trayWindow != 0 {
			procPostMessage.Call(trayWindow, wmClose, 0, 0)
		}
	}, nil
}

func RunTray(actions TrayActions, done <-chan struct{}) error {
	stop, err := StartTray(actions)
	if err != nil {
		return err
	}
	defer stop()
	<-done
	return nil
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
	trayIcon = loadFiberPulseIcon()
	trayNotificationIcon = createBadgedIcon(trayIcon)
	trayNID = notifyIconData{Size: uint32(unsafe.Sizeof(notifyIconData{})), HWnd: hwnd, ID: 1, Flags: nifMessage | nifIcon | nifTip, CallbackMessage: wmUser + 1, Icon: trayIcon}
	copy(trayNID.Tip[:], syscall.StringToUTF16("FiberPulse — measured Internet performance"))
	if r, _, err := procShellNotifyIcon.Call(nimAdd, uintptr(unsafe.Pointer(&trayNID))); r == 0 {
		trayReady <- err
		return
	}
	trayReady <- nil
	go func(window uintptr, stop <-chan struct{}) {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				procPostMessage.Call(window, wmUser+2, 0, 0)
			case <-stop:
				return
			}
		}
	}(hwnd, trayPollStop)
	var m msg
	for {
		r, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessage.Call(uintptr(unsafe.Pointer(&m)))
	}
	procShellNotifyIcon.Call(nimDelete, uintptr(unsafe.Pointer(&trayNID)))
	if trayIcon != 0 {
		procDestroyIcon.Call(trayIcon)
	}
	if trayNotificationIcon != 0 {
		procDestroyIcon.Call(trayNotificationIcon)
	}
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
	case wmUser + 2:
		updateTrayVisual()
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
	state := currentTrayState()
	header := fmt.Sprintf("FiberPulse %s  |  Connection monitor", state.Version)
	headerText, _ := windows.UTF16PtrFromString(header)
	procAppendMenu.Call(menu, mfString|mfDisabled, 0, uintptr(unsafe.Pointer(headerText)))
	procAppendMenu.Call(menu, mfSeparator, 0, 0)
	pauseText := "Pause automatic monitoring"
	if state.Paused {
		pauseText = "Resume automatic monitoring"
	}
	updateText := "Check for updates..."
	if state.UpdateStatus == "available" && state.AvailableVersion != "" {
		updateText = fmt.Sprintf("Update ready: %s -> %s", state.Version, state.AvailableVersion)
	}
	items := []struct {
		id   int
		text string
	}{{cmdOpen, "Open dashboard"}, {cmdTest, "Run a manual connection test"}, {cmdPause, pauseText}, {cmdReport, "History and reports"}, {0, ""}, {cmdUpdate, updateText}}
	if state.UpdateStatus == "available" && state.AvailableVersion != "" {
		items = append(items, struct {
			id   int
			text string
		}{cmdInstall, fmt.Sprintf("Install FiberPulse %s...", state.AvailableVersion)})
	}
	items = append(items, struct {
		id   int
		text string
	}{0, ""}, struct {
		id   int
		text string
	}{cmdQuit, "Quit FiberPulse completely"})
	for _, item := range items {
		if item.id == 0 {
			procAppendMenu.Call(menu, mfSeparator, 0, 0)
			continue
		}
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
		fn = a.CheckUpdate
	case cmdInstall:
		fn = a.InstallUpdate
	case cmdQuit:
		fn = a.Quit
	}
	if fn != nil {
		go fn()
	}
}

func currentTrayState() TrayState {
	trayMu.Lock()
	a := trayCurrent
	trayMu.Unlock()
	if a.State == nil {
		return TrayState{}
	}
	return a.State()
}

func updateTrayVisual() {
	state := currentTrayState()
	text := fmt.Sprintf("FiberPulse %s — measured Internet performance", state.Version)
	hasNotification := state.UpdateStatus == "available" && state.AvailableVersion != ""
	if hasNotification {
		text = fmt.Sprintf("FiberPulse %s — update %s available", state.Version, state.AvailableVersion)
	}
	if hasNotification != trayShowingNotification {
		if hasNotification && trayNotificationIcon != 0 {
			trayNID.Icon = trayNotificationIcon
		} else {
			trayNID.Icon = trayIcon
		}
		trayShowingNotification = hasNotification
	}
	trayNID.Tip = [128]uint16{}
	copy(trayNID.Tip[:], syscall.StringToUTF16(text))
	procShellNotifyIcon.Call(nimModify, uintptr(unsafe.Pointer(&trayNID)))
}

func createBadgedIcon(base uintptr) uintptr {
	if base == 0 {
		return 0
	}
	screenDC, _, _ := procGetDC.Call(0)
	if screenDC == 0 {
		return 0
	}
	defer procReleaseDC.Call(0, screenDC)
	memoryDC, _, _ := procCreateCompatibleDC.Call(screenDC)
	if memoryDC == 0 {
		return 0
	}
	defer procDeleteDC.Call(memoryDC)
	info := bitmapInfo{Header: bitmapInfoHeader{Size: uint32(unsafe.Sizeof(bitmapInfoHeader{})), Width: 32, Height: -32, Planes: 1, BitCount: 32}}
	var pixels uintptr
	color, _, _ := procCreateDIBSection.Call(memoryDC, uintptr(unsafe.Pointer(&info)), 0, uintptr(unsafe.Pointer(&pixels)), 0, 0)
	if color == 0 || pixels == 0 {
		return 0
	}
	defer procDeleteObject.Call(color)
	previous, _, _ := procSelectObject.Call(memoryDC, color)
	procDrawIconEx.Call(memoryDC, 0, 0, base, 32, 32, 0, 0, 3)
	procSelectObject.Call(memoryDC, previous)
	buffer := unsafe.Slice((*uint32)(unsafe.Pointer(pixels)), 32*32)
	for y := 0; y < 12; y++ {
		for x := 20; x < 32; x++ {
			dx, dy := x-26, y-6
			distance := dx*dx + dy*dy
			if distance <= 36 {
				buffer[y*32+x] = 0xffff384d
			} else if distance <= 49 {
				buffer[y*32+x] = 0xffffffff
			}
		}
	}
	mask, _, _ := procCreateBitmap.Call(32, 32, 1, 1, 0)
	if mask == 0 {
		return 0
	}
	defer procDeleteObject.Call(mask)
	details := iconInfo{Icon: 1, Mask: mask, Color: color}
	icon, _, _ := procCreateIconIndirect.Call(uintptr(unsafe.Pointer(&details)))
	return icon
}

func loadFiberPulseIcon() uintptr {
	executable, err := os.Executable()
	if err == nil {
		for _, path := range []string{filepath.Join(filepath.Dir(executable), "FiberPulse.ico"), filepath.Join(filepath.Dir(executable), "Assets", "FiberPulse.ico")} {
			wide, wideErr := windows.UTF16PtrFromString(path)
			if wideErr != nil {
				continue
			}
			icon, _, _ := procLoadImage.Call(0, uintptr(unsafe.Pointer(wide)), imageIcon, 32, 32, lrLoadFromFile)
			if icon != 0 {
				return icon
			}
		}
	}
	icon, _, _ := procLoadIcon.Call(0, 32512)
	return icon
}

func PresentUpdateResult(state TrayState) bool {
	title, _ := windows.UTF16PtrFromString("FiberPulse updates")
	message := "No signed update channel is configured for this build."
	flags := uintptr(mbOK | mbIconInfo)
	if state.UpdateStatus == "available" && state.AvailableVersion != "" {
		message = fmt.Sprintf("A signed FiberPulse update is available.\n\nInstalled version: %s\nAvailable version: %s\n\nDownload, verify and install it now?", state.Version, state.AvailableVersion)
		flags = mbYesNo | mbIconInfo
	} else if state.UpdateStatus == "up_to_date" {
		message = fmt.Sprintf("FiberPulse %s is the newest verified release available for this channel.", state.Version)
	} else if state.UpdateError != "" {
		message = state.UpdateError
		flags = mbOK | mbIconError
	}
	body, _ := windows.UTF16PtrFromString(message)
	result, _, _ := procMessageBox.Call(0, uintptr(unsafe.Pointer(body)), uintptr(unsafe.Pointer(title)), flags)
	return result == idYes
}
