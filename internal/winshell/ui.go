package winshell

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	procOleInitialize          = ole32.NewProc("OleInitialize")
	procRegisterClassExW       = user32.NewProc("RegisterClassExW")
	procCreateWindowExW        = user32.NewProc("CreateWindowExW")
	procDefWindowProcW         = user32.NewProc("DefWindowProcW")
	procMsgWaitForMultipleObj  = user32.NewProc("MsgWaitForMultipleObjects")
	procPeekMessageW           = user32.NewProc("PeekMessageW")
	procTranslateMessage       = user32.NewProc("TranslateMessage")
	procDispatchMessageW       = user32.NewProc("DispatchMessageW")
	procCreatePopupMenu        = user32.NewProc("CreatePopupMenu")
	procDestroyMenu            = user32.NewProc("DestroyMenu")
	procTrackPopupMenuEx       = user32.NewProc("TrackPopupMenuEx")
	procGetCursorPos           = user32.NewProc("GetCursorPos")
	procSetForegroundWindow    = user32.NewProc("SetForegroundWindow")
	procPostMessageW           = user32.NewProc("PostMessageW")
	procEnumWindows            = user32.NewProc("EnumWindows")
	procGetWindowThreadProcess = user32.NewProc("GetWindowThreadProcessId")
	procGetClassNameW          = user32.NewProc("GetClassNameW")
)

// Window messages that a shell context menu needs to draw its submenus, such as
// "Send to" and "Open with".
const (
	wmNull          = 0x0000
	wmDrawItem      = 0x002B
	wmMeasureItem   = 0x002C
	wmInitMenuPopup = 0x0117
	wmMenuChar      = 0x0120
	wmMenuSelect    = 0x011F
)

const (
	tpmReturnCmd   = 0x0100
	tpmRightButton = 0x0002

	cmfNormal        = 0x0000
	cmfExplore       = 0x0004
	cmfExtendedVerbs = 0x0100

	cmicMaskUnicode   = 0x00004000
	cmicMaskPtInvoke  = 0x20000000
	cmicMaskNoAsync   = 0x00000100
	cmicMaskFlagNoUI  = 0x00000400
	firstMenuCommand  = 1
	lastMenuCommand   = 0x7FFF
	swShowNormal      = 1
	seeMaskInvokeList = 0x0000000C
	seeMaskNoAsync    = 0x00000100
	seeMaskFlagNoUI   = 0x00000400
)

// UI is a thread with a hidden window. It shows the native context menu and runs
// calls that need a window or a message loop: the clipboard and shell commands.
type UI struct {
	jobs  chan func()
	wake  windows.Handle
	hwnd  uintptr
	owner uintptr
	mu    sync.Mutex
}

// The menu being shown. Only the UI thread touches it.
var menuHandlers struct {
	menu2 comObject
	menu3 comObject
}

// NewUI starts the thread.
func NewUI() (*UI, error) {
	wake, err := windows.CreateEvent(nil, 0, 0, nil)
	if err != nil {
		return nil, err
	}
	u := &UI{jobs: make(chan func(), 16), wake: wake}
	ready := make(chan error, 1)
	go u.loop(ready)
	if err := <-ready; err != nil {
		return nil, err
	}
	return u, nil
}

// SetOwner sets the main window, which owns the dialogs that commands open.
func (u *UI) SetOwner(hwnd uintptr) {
	u.mu.Lock()
	u.owner = hwnd
	u.mu.Unlock()
}

// ownerWindow returns the main window. The page may be ready before the window
// is shown, so a window not found then is looked for again.
func (u *UI) ownerWindow() uintptr {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.owner == 0 {
		u.owner = FindMainWindow()
	}
	return u.owner
}

func (u *UI) loop(ready chan<- error) {
	runtime.LockOSThread()
	procOleInitialize.Call(0)

	className := utf16Ptr("SashaExplorerShellHost")
	instance, _, _ := kernel.NewProc("GetModuleHandleW").Call(0)
	type wndClassEx struct {
		Size       uint32
		Style      uint32
		WndProc    uintptr
		ClsExtra   int32
		WndExtra   int32
		Instance   uintptr
		Icon       uintptr
		Cursor     uintptr
		Background uintptr
		MenuName   *uint16
		ClassName  *uint16
		IconSm     uintptr
	}
	class := wndClassEx{
		WndProc:   windows.NewCallback(windowProc),
		Instance:  instance,
		ClassName: className,
	}
	class.Size = uint32(unsafe.Sizeof(class))
	if r, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&class))); r == 0 {
		ready <- err
		return
	}
	const wsPopup = 0x80000000
	const wsExToolWindow = 0x80
	hwnd, _, err := procCreateWindowExW.Call(
		wsExToolWindow, uintptr(unsafe.Pointer(className)), 0, wsPopup,
		0, 0, 0, 0, 0, 0, instance, 0,
	)
	if hwnd == 0 {
		ready <- err
		return
	}
	u.hwnd = hwnd
	ready <- nil

	type msg struct {
		Hwnd    uintptr
		Message uint32
		WParam  uintptr
		LParam  uintptr
		Time    uint32
		Pt      [2]int32
		Private uint32
	}
	const qsAllInput = 0x04FF
	const pmRemove = 1
	for {
		r, _, _ := procMsgWaitForMultipleObj.Call(1, uintptr(unsafe.Pointer(&u.wake)), 0, windows.INFINITE, qsAllInput)
		if r == windows.WAIT_OBJECT_0 {
		drain:
			for {
				select {
				case job := <-u.jobs:
					job()
				default:
					break drain
				}
			}
		}
		var m msg
		for {
			if r, _, _ := procPeekMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0, pmRemove); r == 0 {
				break
			}
			procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
			procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
		}
	}
}

func windowProc(hwnd, message, wParam, lParam uintptr) uintptr {
	switch message {
	case wmInitMenuPopup, wmDrawItem, wmMeasureItem, wmMenuChar, wmMenuSelect:
		if menuHandlers.menu3 != 0 {
			var result uintptr
			if failed(menuHandlers.menu3.call(7, message, wParam, lParam, uintptr(unsafe.Pointer(&result)))) == nil {
				return result
			}
		} else if menuHandlers.menu2 != 0 {
			if failed(menuHandlers.menu2.call(6, message, wParam, lParam)) == nil {
				return 0
			}
		}
	}
	r, _, _ := procDefWindowProcW.Call(hwnd, message, wParam, lParam)
	return r
}

// Do runs f on the UI thread and waits for it.
func (u *UI) Do(f func()) {
	done := make(chan struct{})
	u.jobs <- func() {
		defer close(done)
		f()
	}
	windows.SetEvent(u.wake)
	<-done
}

// shellMenu is a shell object of items in one folder: their context menu, or
// their data for a drag.
type shellMenu struct {
	menu  comObject
	pidls []uintptr
}

func (m *shellMenu) close() {
	m.menu.release()
	for _, pidl := range m.pidls {
		windows.CoTaskMemFree(unsafe.Pointer(pidl))
	}
}

// openShellMenu asks the shell for the context menu of paths, which must share
// a folder.
func openShellMenu(owner uintptr, paths []string) (*shellMenu, error) {
	return openShellObject(owner, paths, &iidContextMenu)
}

// openShellObject asks the shell for an object of items in one folder, such as
// their context menu or their data for a drag.
func openShellObject(owner uintptr, paths []string, iid *windows.GUID) (*shellMenu, error) {
	if len(paths) == 0 {
		return nil, errors.New("nothing is selected")
	}
	m := &shellMenu{}
	children := make([]uintptr, 0, len(paths))
	for _, path := range paths {
		var pidl uintptr
		hr, _, _ := procSHParseDisplayName.Call(uintptr(unsafe.Pointer(utf16Ptr(path))), 0, uintptr(unsafe.Pointer(&pidl)), 0, 0)
		if err := failed(hr); err != nil {
			m.close()
			return nil, err
		}
		m.pidls = append(m.pidls, pidl)
		child, _, _ := procILFindLastID.Call(pidl)
		children = append(children, child)
	}

	var folder comObject
	hr, _, _ := procSHBindToParent.Call(m.pidls[0], uintptr(unsafe.Pointer(&iidShellFolder)), uintptr(unsafe.Pointer(&folder)), 0)
	if err := failed(hr); err != nil {
		m.close()
		return nil, err
	}
	defer folder.release()

	// IShellFolder::GetUIObjectOf
	hr = folder.call(10, owner, uintptr(len(children)), uintptr(unsafe.Pointer(&children[0])),
		uintptr(unsafe.Pointer(iid)), 0, uintptr(unsafe.Pointer(&m.menu)))
	if err := failed(hr); err != nil {
		m.close()
		return nil, err
	}
	return m, nil
}

type invokeInfo struct {
	Size        uint32
	Mask        uint32
	Hwnd        uintptr
	Verb        uintptr
	Parameters  uintptr
	Directory   uintptr
	Show        int32
	HotKey      uint32
	Icon        uintptr
	Title       uintptr
	VerbW       uintptr
	ParametersW uintptr
	DirectoryW  uintptr
	TitleW      uintptr
	Point       [2]int32
}

// ShowContextMenu shows the native context menu of paths at the mouse pointer
// and runs the chosen command. extended adds the commands Windows shows with
// Shift held.
func (u *UI) ShowContextMenu(paths []string, extended bool) error {
	var err error
	u.Do(func() { err = u.showContextMenu(paths, extended) })
	return err
}

func (u *UI) showContextMenu(paths []string, extended bool) error {
	owner := u.ownerWindow()
	m, err := openShellMenu(u.hwnd, paths)
	if err != nil {
		return err
	}
	defer m.close()

	menu, _, _ := procCreatePopupMenu.Call()
	if menu == 0 {
		return errors.New("create menu")
	}
	defer procDestroyMenu.Call(menu)

	flags := uintptr(cmfNormal | cmfExplore)
	if extended {
		flags |= cmfExtendedVerbs
	}
	// IContextMenu::QueryContextMenu
	if err := failed(m.menu.call(3, menu, 0, firstMenuCommand, lastMenuCommand, flags)); err != nil {
		return err
	}

	menu2, _ := m.menu.queryInterface(&iidContextMenu2)
	menu3, _ := m.menu.queryInterface(&iidContextMenu3)
	menuHandlers.menu2, menuHandlers.menu3 = menu2, menu3
	defer func() {
		menuHandlers.menu2, menuHandlers.menu3 = 0, 0
		menu2.release()
		menu3.release()
	}()

	var point [2]int32
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&point)))
	// The menu closes on a click elsewhere only when its window is in front.
	procSetForegroundWindow.Call(u.hwnd)
	command, _, _ := procTrackPopupMenuEx.Call(menu, tpmReturnCmd|tpmRightButton,
		uintptr(point[0]), uintptr(point[1]), u.hwnd, 0)
	procPostMessageW.Call(u.hwnd, wmNull, 0, 0)
	if owner != 0 {
		procSetForegroundWindow.Call(owner)
	}
	if command == 0 {
		return nil
	}

	info := invokeInfo{
		Mask:       cmicMaskUnicode | cmicMaskPtInvoke,
		Hwnd:       owner,
		Verb:       command - firstMenuCommand,
		VerbW:      command - firstMenuCommand,
		Show:       swShowNormal,
		DirectoryW: uintptr(unsafe.Pointer(utf16Ptr(filepath.Dir(paths[0])))),
		Point:      point,
	}
	info.Size = uint32(unsafe.Sizeof(info))
	return failed(m.menu.call(4, uintptr(unsafe.Pointer(&info))))
}

// ShowProperties opens the Windows properties dialog of paths, which must share a
// folder.
func (u *UI) ShowProperties(paths []string) error {
	var err error
	u.Do(func() {
		var m *shellMenu
		m, err = openShellMenu(u.hwnd, paths)
		if err != nil {
			return
		}
		defer m.close()
		verb := []byte("properties\x00")
		verbW := utf16Ptr("properties")
		info := invokeInfo{
			Mask:  cmicMaskUnicode,
			Hwnd:  u.ownerWindow(),
			Verb:  uintptr(unsafe.Pointer(&verb[0])),
			VerbW: uintptr(unsafe.Pointer(verbW)),
			Show:  swShowNormal,
		}
		info.Size = uint32(unsafe.Sizeof(info))
		err = failed(m.menu.call(4, uintptr(unsafe.Pointer(&info))))
		runtime.KeepAlive(verb)
	})
	return err
}

type shellExecuteInfo struct {
	Size       uint32
	Mask       uint32
	Hwnd       uintptr
	Verb       *uint16
	File       *uint16
	Parameters *uint16
	Directory  *uint16
	Show       int32
	InstApp    uintptr
	IDList     uintptr
	Class      *uint16
	KeyClass   uintptr
	HotKey     uint32
	Icon       uintptr
	Process    uintptr
}

// Open opens a file with its program, as a double click in Windows does.
func (u *UI) Open(path string) error {
	return u.execute("", path)
}

// OpenWith asks which program should open a file.
func (u *UI) OpenWith(path string) error {
	return u.execute("openas", path)
}

func (u *UI) execute(verb, path string) error {
	var err error
	u.Do(func() {
		info := shellExecuteInfo{
			Mask:      seeMaskInvokeList | seeMaskNoAsync,
			Hwnd:      u.ownerWindow(),
			File:      utf16Ptr(path),
			Directory: utf16Ptr(filepath.Dir(path)),
			Show:      swShowNormal,
		}
		if verb != "" {
			info.Verb = utf16Ptr(verb)
		}
		info.Size = uint32(unsafe.Sizeof(info))
		if r, _, callErr := procShellExecuteExW.Call(uintptr(unsafe.Pointer(&info))); r == 0 {
			if !errors.Is(callErr, windows.ERROR_CANCELLED) {
				err = callErr
			}
		}
	})
	return err
}

var findWindow struct {
	mu    sync.Mutex
	pid   uint32
	found uintptr
}

var enumCallback = windows.NewCallback(func(hwnd, _ uintptr) uintptr {
	var owner uint32
	procGetWindowThreadProcess.Call(hwnd, uintptr(unsafe.Pointer(&owner)))
	if owner != findWindow.pid {
		return 1
	}
	var name [64]uint16
	procGetClassNameW.Call(hwnd, uintptr(unsafe.Pointer(&name[0])), uintptr(len(name)))
	if windows.UTF16ToString(name[:]) == "wailsWindow" {
		findWindow.found = hwnd
		return 0
	}
	return 1
})

// FindMainWindow returns the Wails window of this process, or 0. It may be
// hidden: Wails shows it only once the page is loaded.
func FindMainWindow() uintptr {
	findWindow.mu.Lock()
	defer findWindow.mu.Unlock()
	findWindow.pid = uint32(os.Getpid())
	findWindow.found = 0
	procEnumWindows.Call(enumCallback, 0)
	return findWindow.found
}
