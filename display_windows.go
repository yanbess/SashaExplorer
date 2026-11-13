package main

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows/registry"

	"sashaexplorer/internal/state"
)

const (
	uiScale = 1.25

	// Window sizes at 100% zoom.
	baseWidth     = 1600
	baseHeight    = 800
	baseMinWidth  = 760
	baseMinHeight = 520

	spiGetWorkArea = 0x0030
)

// windowLayout is the zoom and window size in device-independent pixels.
type windowLayout struct {
	zoom      float64
	width     int
	height    int
	minWidth  int
	minHeight int
}

// layoutFor keeps the full zoom when the smallest window fits the work area and
// lowers it on smaller screens, so the window never needs more than the screen.
func layoutFor(areaWidth, areaHeight int) windowLayout {
	zoom := min(uiScale, float64(areaWidth)/baseMinWidth, float64(areaHeight)/baseMinHeight)
	zoom = max(zoom, 1)
	return windowLayout{
		zoom:      zoom,
		width:     min(int(baseWidth*zoom), areaWidth),
		height:    min(int(baseHeight*zoom), areaHeight),
		minWidth:  int(baseMinWidth * zoom),
		minHeight: int(baseMinHeight * zoom),
	}
}

// currentLayout measures the primary display. If that fails, it uses the base
// size at 100% zoom.
func currentLayout() windowLayout {
	width, height, ok := primaryWorkArea()
	if !ok {
		return layoutFor(baseWidth, baseHeight)
	}
	return layoutFor(width, height)
}

// primaryWorkArea returns the primary display's area without the taskbar, in
// device-independent pixels.
func primaryWorkArea() (width, height int, ok bool) {
	user32 := syscall.NewLazyDLL("user32.dll")

	var area struct{ left, top, right, bottom int32 }
	result, _, _ := user32.NewProc("SystemParametersInfoW").Call(
		spiGetWorkArea, 0, uintptr(unsafe.Pointer(&area)), 0,
	)
	if result == 0 {
		return 0, 0, false
	}

	dpi, _, _ := user32.NewProc("GetDpiForSystem").Call()
	if dpi == 0 {
		return 0, 0, false
	}
	return int(area.right-area.left) * 96 / int(dpi), int(area.bottom-area.top) * 96 / int(dpi), true
}

// windowPlacement is WINDOWPLACEMENT.
type windowPlacement struct {
	Length      uint32
	Flags       uint32
	ShowCmd     uint32
	MinPosition [2]int32
	MaxPosition [2]int32
	Normal      struct{ Left, Top, Right, Bottom int32 }
}

const (
	swHide               = 0
	swShowMaximized      = 3
	monitorDefaultToNull = 0
)

var (
	procGetWindowPlacement = syscall.NewLazyDLL("user32.dll").NewProc("GetWindowPlacement")
	procSetWindowPlacement = syscall.NewLazyDLL("user32.dll").NewProc("SetWindowPlacement")
	procMonitorFromRect    = syscall.NewLazyDLL("user32.dll").NewProc("MonitorFromRect")
)

// readPlacement returns where a window is: its rectangle when it is neither
// maximized nor minimized, and whether it is maximized.
func readPlacement(hwnd uintptr) (state.Window, bool) {
	place := windowPlacement{}
	place.Length = uint32(unsafe.Sizeof(place))
	if r, _, _ := procGetWindowPlacement.Call(hwnd, uintptr(unsafe.Pointer(&place))); r == 0 {
		return state.Window{}, false
	}
	return state.Window{
		Left: place.Normal.Left, Top: place.Normal.Top, Right: place.Normal.Right, Bottom: place.Normal.Bottom,
		Maximized: place.ShowCmd == swShowMaximized,
	}, true
}

// placeHidden moves a window that is not shown yet to a remembered rectangle,
// when that rectangle is still on a screen. Showing the window is left to the
// caller.
func placeHidden(hwnd uintptr, saved state.Window) bool {
	rect := struct{ Left, Top, Right, Bottom int32 }{saved.Left, saved.Top, saved.Right, saved.Bottom}
	if monitor, _, _ := procMonitorFromRect.Call(uintptr(unsafe.Pointer(&rect)), monitorDefaultToNull); monitor == 0 {
		return false
	}
	place := windowPlacement{ShowCmd: swHide}
	place.Length = uint32(unsafe.Sizeof(place))
	place.Normal = rect
	r, _, _ := procSetWindowPlacement.Call(hwnd, uintptr(unsafe.Pointer(&place)))
	return r != 0
}

// lightTheme tells whether a theme setting means light colors now: "light", or
// "system" while Windows uses light colors for apps.
func lightTheme(theme string) bool {
	switch theme {
	case state.ThemeLight:
		return true
	case state.ThemeDark:
		return false
	}
	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer key.Close()
	value, _, err := key.GetIntegerValue("AppsUseLightTheme")
	return err == nil && value == 1
}
