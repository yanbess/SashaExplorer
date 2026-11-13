package winshell

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	procSHDoDragDrop       = shell32.NewProc("SHDoDragDrop")
	procGetAsyncKeyState   = user32.NewProc("GetAsyncKeyState")
	procGetSystemMetrics   = user32.NewProc("GetSystemMetrics")
	procAttachThreadInput  = user32.NewProc("AttachThreadInput")
	procGetCurrentThreadId = kernel.NewProc("GetCurrentThreadId")

	iidDataObject = windows.GUID{Data1: 0x0000010e, Data4: [8]byte{0xC0, 0, 0, 0, 0, 0, 0, 0x46}}
	iidUnknown    = windows.GUID{Data1: 0x00000000, Data4: [8]byte{0xC0, 0, 0, 0, 0, 0, 0, 0x46}}
	iidDropSource = windows.GUID{Data1: 0x00000121, Data4: [8]byte{0xC0, 0, 0, 0, 0, 0, 0, 0x46}}
)

// Drop effects of OLE drag and drop.
const (
	DropEffectNone = 0
	DropEffectCopy = 1
	DropEffectMove = 2
	DropEffectLink = 4
)

const (
	sOK                       = 0
	eNoInterface              = 0x80004002
	dragdropSDrop             = 0x00040100
	dragdropSCancel           = 0x00040101
	dragdropSUseDefaultCursor = 0x00040102

	vkLButton    = 0x01
	vkRButton    = 0x02
	smSwapButton = 23
)

// dropSource is the IDropSource of a drag. The default one of Windows asks the
// key state of the thread that runs the drag, but the button went down in the
// page, which lives in another process, so that thread sees it up and the drag
// ends at once on this window. This one reads the button itself.
type dropSource struct {
	vtbl *[5]uintptr
	refs int32
}

var (
	dropSourceVtbl = [5]uintptr{
		windows.NewCallback(func(this, iid, out uintptr) uintptr {
			id := (*windows.GUID)(unsafe.Pointer(iid))
			if *id == iidUnknown || *id == iidDropSource {
				*(*uintptr)(unsafe.Pointer(out)) = this
				atomic.AddInt32(&(*dropSource)(unsafe.Pointer(this)).refs, 1)
				return sOK
			}
			*(*uintptr)(unsafe.Pointer(out)) = 0
			return eNoInterface
		}),
		windows.NewCallback(func(this uintptr) uintptr {
			return uintptr(atomic.AddInt32(&(*dropSource)(unsafe.Pointer(this)).refs, 1))
		}),
		windows.NewCallback(func(this uintptr) uintptr {
			return uintptr(atomic.AddInt32(&(*dropSource)(unsafe.Pointer(this)).refs, -1))
		}),
		// QueryContinueDrag
		windows.NewCallback(func(this, escape, keys uintptr) uintptr {
			if uint32(escape) != 0 {
				return dragdropSCancel
			}
			if !primaryButtonDown() {
				return dragdropSDrop
			}
			return sOK
		}),
		// GiveFeedback
		windows.NewCallback(func(this, effect uintptr) uintptr {
			return dragdropSUseDefaultCursor
		}),
	}
	// One drag runs at a time, on the UI thread, so one object serves them all.
	theDropSource = dropSource{vtbl: &dropSourceVtbl, refs: 1}
)

// primaryButtonDown reports the physical state of the primary mouse button.
func primaryButtonDown() bool {
	key := uintptr(vkLButton)
	if swapped, _, _ := procGetSystemMetrics.Call(smSwapButton); swapped != 0 {
		key = vkRButton
	}
	state, _, _ := procGetAsyncKeyState.Call(key)
	return state&0x8000 != 0
}

// StartDrag starts a Windows drag of items in one folder, so they can be dropped
// into Explorer, a mail, a chat, or any other program. It returns when the drag
// ends, with the effect the target chose. The mouse button must be down.
//
// The window of this program receives the drag like any other target, so drops
// inside it arrive through the page as dropped files.
//
// The drag runs on the thread of the main window. The mouse went down in the
// page, and only that thread shares its input: a drag on any other thread
// cannot take the mouse and never ends.
func (u *UI) StartDrag(paths []string) (int, error) {
	owner := u.ownerWindow()
	if owner == 0 {
		return 0, errors.New("no main window")
	}
	var (
		effect uint32
		err    error
	)
	ran, runErr := onWindowThread(owner, func() {
		// The page keeps OLE running on this thread; this only makes sure.
		procOleInitialize.Call(0)
		var m *shellMenu
		m, err = openShellObject(owner, paths, &iidDataObject)
		if err != nil {
			return
		}
		defer m.close()
		var hr uintptr
		const effects = DropEffectCopy | DropEffectMove | DropEffectLink
		// Windows draws the picture of a drag itself only once it has the icon,
		// so the first drag would show a blank square. This one sets it first.
		if setDragImage(m.menu, paths[0]) {
			hr, _, _ = procDoDragDrop.Call(uintptr(m.menu), uintptr(unsafe.Pointer(&theDropSource)),
				effects, uintptr(unsafe.Pointer(&effect)))
		} else {
			hr, _, _ = procSHDoDragDrop.Call(owner, uintptr(m.menu), uintptr(unsafe.Pointer(&theDropSource)),
				effects, uintptr(unsafe.Pointer(&effect)))
		}
		// DRAGDROP_S_DROP and DRAGDROP_S_CANCEL are successes.
		err = failed(hr)
	})
	if !ran {
		return 0, runErr
	}
	return int(effect), err
}

const (
	whGetMessage = 3
	pmRemoveFlag = 1
	wmNullMsg    = 0
)

var (
	procSetWindowsHookExW     = user32.NewProc("SetWindowsHookExW")
	procCallNextHookEx        = user32.NewProc("CallNextHookEx")
	procRegisterWindowMessage = user32.NewProc("RegisterWindowMessageW")
)

// windowThread runs calls on the thread of a window: a message posted to the
// window wakes a hook on that thread, which runs the waiting call.
var windowThread struct {
	mu      sync.Mutex // one call at a time
	hooked  map[uintptr]bool
	message uint32
	window  uintptr
	job     atomic.Pointer[func()]
}

var getMessageHook = windows.NewCallback(func(code, wParam, lParam uintptr) uintptr {
	if int32(code) >= 0 && wParam == pmRemoveFlag {
		m := (*struct {
			Hwnd    uintptr
			Message uint32
		})(unsafe.Pointer(lParam))
		if m.Message == windowThread.message && windowThread.message != 0 && m.Hwnd == windowThread.window {
			// The window must not see the message itself.
			m.Message = wmNullMsg
			if job := windowThread.job.Swap(nil); job != nil {
				(*job)()
			}
		}
	}
	r, _, _ := procCallNextHookEx.Call(0, code, wParam, lParam)
	return r
})

// onWindowThread runs f on the thread of hwnd and waits for it. It reports
// false when f could not be sent there.
func onWindowThread(hwnd uintptr, f func()) (bool, error) {
	w := &windowThread
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.message == 0 {
		r, _, err := procRegisterWindowMessage.Call(uintptr(unsafe.Pointer(utf16Ptr("SashaExplorerRunOnWindowThread"))))
		if r == 0 {
			return false, err
		}
		w.message = uint32(r)
		w.hooked = map[uintptr]bool{}
	}
	thread, _, _ := procGetWindowThreadProcess.Call(hwnd, 0)
	if thread == 0 {
		return false, errors.New("the window is gone")
	}
	if !w.hooked[thread] {
		if r, _, err := procSetWindowsHookExW.Call(whGetMessage, getMessageHook, 0, thread); r == 0 {
			return false, err
		}
		w.hooked[thread] = true
	}
	done := make(chan struct{})
	job := func() {
		defer close(done)
		f()
	}
	w.window = hwnd
	w.job.Store(&job)
	if r, _, err := procPostMessageW.Call(hwnd, uintptr(w.message), 0, 0); r == 0 {
		w.job.Store(nil)
		return false, err
	}
	select {
	case <-done:
		return true, nil
	case <-time.After(2 * time.Second):
		// The thread did not pick it up in time; the drag would be too late.
		if w.job.CompareAndSwap(&job, nil) {
			return false, errors.New("the window thread did not answer")
		}
		<-done
		return true, nil
	}
}

var (
	procDoDragDrop = ole32.NewProc("DoDragDrop")

	clsidDragDropHelper = windows.GUID{Data1: 0x4657278A, Data2: 0x411B, Data3: 0x11D2, Data4: [8]byte{0x83, 0x9A, 0x00, 0xC0, 0x4F, 0xD9, 0x18, 0xD0}}
	iidDragSourceHelper = windows.GUID{Data1: 0xDE5BF786, Data2: 0x477A, Data3: 0x11D2, Data4: [8]byte{0x83, 0x9D, 0x00, 0xC0, 0x4F, 0xD9, 0x18, 0xD0}}
	dragImageSize       = 96
	siigbfInCacheOnly   = uintptr(0x10)
	clsctxInprocServer  = uintptr(1)
	colorNone           = uint32(0xFFFFFFFF)
)

// shDragImage is SHDRAGIMAGE.
type shDragImage struct {
	Width, Height    int32
	OffsetX, OffsetY int32
	Bitmap           uintptr
	ColorKey         uint32
}

// setDragImage gives the drag the picture of the first item: its thumbnail
// when Windows has one at hand, or else its icon. It reports whether it did.
func setDragImage(data comObject, path string) bool {
	bitmap := itemBitmap(path, siigbfThumbnailOnly|siigbfInCacheOnly)
	if bitmap == 0 {
		bitmap = itemBitmap(path, siigbfIconOnly)
	}
	if bitmap == 0 {
		return false
	}
	var helper comObject
	hr, _, _ := procCoCreateInstance.Call(uintptr(unsafe.Pointer(&clsidDragDropHelper)), 0, clsctxInprocServer,
		uintptr(unsafe.Pointer(&iidDragSourceHelper)), uintptr(unsafe.Pointer(&helper)))
	if failed(hr) != nil {
		procDeleteObject.Call(bitmap)
		return false
	}
	defer helper.release()
	var info bitmapInfo
	procGetObjectW.Call(bitmap, unsafe.Sizeof(info), uintptr(unsafe.Pointer(&info)))
	image := shDragImage{
		Width: info.Width, Height: info.Height,
		OffsetX: info.Width / 2, OffsetY: info.Height / 2,
		Bitmap: bitmap, ColorKey: colorNone,
	}
	// IDragSourceHelper::InitializeFromBitmap takes the bitmap when it succeeds.
	if failed(helper.call(3, uintptr(unsafe.Pointer(&image)), uintptr(data))) != nil {
		procDeleteObject.Call(bitmap)
		return false
	}
	return true
}

// itemBitmap returns a bitmap of an item from the shell, or 0.
func itemBitmap(path string, flags uintptr) uintptr {
	var factory comObject
	hr, _, _ := procSHCreateItemFromParsingName.Call(
		uintptr(unsafe.Pointer(utf16Ptr(path))), 0,
		uintptr(unsafe.Pointer(&iidShellItemImageFactory)), uintptr(unsafe.Pointer(&factory)),
	)
	if failed(hr) != nil {
		return 0
	}
	defer factory.release()
	sizeArg := uintptr(uint32(dragImageSize)) | uintptr(uint32(dragImageSize))<<32
	var bitmap uintptr
	if failed(factory.call(3, sizeArg, flags, uintptr(unsafe.Pointer(&bitmap)))) != nil {
		return 0
	}
	return bitmap
}
