// Package winshell calls the Windows shell: thumbnails and icons, type names,
// shortcuts, the native context menu, the file clipboard, and file operations with
// the progress and conflict dialogs of Windows.
//
// Shell objects are COM objects, so every call runs on a thread that has COM
// initialized. Go moves goroutines between threads, so each such goroutine locks
// itself to its thread first.
package winshell

import (
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	shell32 = windows.NewLazySystemDLL("shell32.dll")
	user32  = windows.NewLazySystemDLL("user32.dll")
	gdi32   = windows.NewLazySystemDLL("gdi32.dll")
	ole32   = windows.NewLazySystemDLL("ole32.dll")
	kernel  = windows.NewLazySystemDLL("kernel32.dll")

	procSHCreateItemFromParsingName = shell32.NewProc("SHCreateItemFromParsingName")
	procSHGetFileInfoW              = shell32.NewProc("SHGetFileInfoW")
	procSHParseDisplayName          = shell32.NewProc("SHParseDisplayName")
	procSHBindToParent              = shell32.NewProc("SHBindToParent")
	procShellExecuteExW             = shell32.NewProc("ShellExecuteExW")
	procSHFileOperationW            = shell32.NewProc("SHFileOperationW")
	procDragQueryFileW              = shell32.NewProc("DragQueryFileW")
	procILFindLastID                = shell32.NewProc("ILFindLastID")

	procCoCreateInstance = ole32.NewProc("CoCreateInstance")

	procGetObjectW   = gdi32.NewProc("GetObjectW")
	procGetDIBits    = gdi32.NewProc("GetDIBits")
	procDeleteObject = gdi32.NewProc("DeleteObject")

	procGetDC     = user32.NewProc("GetDC")
	procReleaseDC = user32.NewProc("ReleaseDC")

	procGlobalAlloc  = kernel.NewProc("GlobalAlloc")
	procGlobalLock   = kernel.NewProc("GlobalLock")
	procGlobalUnlock = kernel.NewProc("GlobalUnlock")
	procGlobalFree   = kernel.NewProc("GlobalFree")
)

// Interface IDs and class IDs used here.
var (
	iidShellItemImageFactory = windows.GUID{Data1: 0xbcc18b79, Data2: 0xba16, Data3: 0x442f, Data4: [8]byte{0x80, 0xc4, 0x8a, 0x59, 0xc3, 0x0c, 0x46, 0x3b}}
	iidShellFolder           = windows.GUID{Data1: 0x000214E6, Data4: [8]byte{0xC0, 0, 0, 0, 0, 0, 0, 0x46}}
	iidContextMenu           = windows.GUID{Data1: 0x000214E4, Data4: [8]byte{0xC0, 0, 0, 0, 0, 0, 0, 0x46}}
	iidContextMenu2          = windows.GUID{Data1: 0x000214F4, Data4: [8]byte{0xC0, 0, 0, 0, 0, 0, 0, 0x46}}
	iidContextMenu3          = windows.GUID{Data1: 0xBCFCE0A0, Data2: 0xEC17, Data3: 0x11D0, Data4: [8]byte{0x8D, 0x10, 0x00, 0xA0, 0xC9, 0x0F, 0x27, 0x19}}
	iidShellLinkW            = windows.GUID{Data1: 0x000214F9, Data4: [8]byte{0xC0, 0, 0, 0, 0, 0, 0, 0x46}}
	iidPersistFile           = windows.GUID{Data1: 0x0000010b, Data4: [8]byte{0xC0, 0, 0, 0, 0, 0, 0, 0x46}}
	clsidShellLink           = windows.GUID{Data1: 0x00021401, Data4: [8]byte{0xC0, 0, 0, 0, 0, 0, 0, 0x46}}
)

// comObject is a pointer to a COM interface: its first field points to the
// method table.
type comObject uintptr

func (o comObject) method(index int) uintptr {
	table := *(*uintptr)(unsafe.Pointer(o))
	return *(*uintptr)(unsafe.Pointer(table + uintptr(index)*unsafe.Sizeof(uintptr(0))))
}

// call invokes a method by its index in the method table, with the object first.
// Pointers converted to uintptr in its arguments are moved to the heap, so a
// growing stack cannot move what they point to before the call.
//
//go:uintptrescapes
func (o comObject) call(index int, args ...uintptr) uintptr {
	all := append([]uintptr{uintptr(o)}, args...)
	result, _, _ := syscall.SyscallN(o.method(index), all...)
	return result
}

func (o comObject) queryInterface(iid *windows.GUID) (comObject, bool) {
	var out comObject
	hr := o.call(0, uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(&out)))
	return out, failed(hr) == nil && out != 0
}

func (o comObject) release() {
	if o != 0 {
		o.call(2)
	}
}

// failed converts an HRESULT into an error, or nil when it means success.
func failed(hr uintptr) error {
	if int32(hr) < 0 {
		return syscall.Errno(hr)
	}
	return nil
}

// Pool runs calls on threads that have COM initialized as a single-threaded
// apartment, which is what shell extensions expect.
type Pool struct {
	jobs chan func()
}

// NewPool starts n threads.
func NewPool(n int) *Pool {
	p := &Pool{jobs: make(chan func())}
	for range n {
		go p.worker()
	}
	return p
}

func (p *Pool) worker() {
	runtime.LockOSThread()
	// Shell extensions run in this thread, so it stays locked even if one
	// changes the thread state: a locked goroutine that ends takes its thread
	// down with it.
	_ = windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED|windows.COINIT_DISABLE_OLE1DDE)
	for job := range p.jobs {
		job()
	}
}

// Do runs f on a pool thread and waits for it.
func (p *Pool) Do(f func()) {
	done := make(chan struct{})
	p.jobs <- func() {
		defer close(done)
		f()
	}
	<-done
}

// onOwnThread runs f on a new thread with COM initialized and waits for it. It is
// for long calls, such as a copy that shows its own progress window.
func onOwnThread(f func()) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		if err := windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED|windows.COINIT_DISABLE_OLE1DDE); err == nil {
			defer windows.CoUninitialize()
		}
		f()
	}()
	<-done
}

func utf16Ptr(s string) *uint16 {
	p, err := windows.UTF16PtrFromString(s)
	if err != nil {
		// A string with a NUL cannot be a path. An empty one fails safely later.
		p, _ = windows.UTF16PtrFromString("")
	}
	return p
}

// doubleNullList encodes paths the way SHFileOperation and CF_HDROP want them:
// each one ends with a NUL, and one more NUL ends the list.
func doubleNullList(paths []string) []uint16 {
	var list []uint16
	for _, path := range paths {
		encoded, err := windows.UTF16FromString(path)
		if err != nil {
			continue
		}
		list = append(list, encoded...)
	}
	return append(list, 0)
}
