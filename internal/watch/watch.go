// Package watch reports changes in one folder, so the list can update without a
// refresh. It watches only the folder itself, not its subfolders.
package watch

import (
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sys/windows"
)

// Delay groups a burst of changes, such as a copy of many files, into one report.
const Delay = 300 * time.Millisecond

const notifyFilter = windows.FILE_NOTIFY_CHANGE_FILE_NAME |
	windows.FILE_NOTIFY_CHANGE_DIR_NAME |
	windows.FILE_NOTIFY_CHANGE_SIZE |
	windows.FILE_NOTIFY_CHANGE_LAST_WRITE |
	windows.FILE_NOTIFY_CHANGE_ATTRIBUTES

// Watcher watches one folder at a time.
type Watcher struct {
	mu      sync.Mutex
	path    string
	stop    windows.Handle
	changed func(path string)
}

// New creates a watcher that calls changed with the folder path.
func New(changed func(path string)) *Watcher {
	return &Watcher{changed: changed}
}

// Watch switches to path. An empty path stops watching.
func (w *Watcher) Watch(path string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if strings.EqualFold(path, w.path) && w.stop != 0 {
		return
	}
	if w.stop != 0 {
		windows.SetEvent(w.stop)
		w.stop = 0
	}
	w.path = path
	if path == "" {
		return
	}
	stop, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		return
	}
	w.stop = stop
	go w.run(path, stop)
}

// Close stops watching.
func (w *Watcher) Close() {
	w.Watch("")
}

func (w *Watcher) run(path string, stop windows.Handle) {
	// Watch signals stop and never touches it again, so it is closed here, but
	// only after the signal: closing it sooner could let Windows reuse the handle
	// value before Watch signals it.
	defer func() {
		windows.WaitForSingleObject(stop, windows.INFINITE)
		windows.CloseHandle(stop)
	}()
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return
	}
	// Every kind of sharing, so the folder can still be renamed or deleted.
	dir, err := windows.CreateFile(name, windows.FILE_LIST_DIRECTORY,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil,
		windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OVERLAPPED, 0)
	if err != nil {
		return
	}
	defer windows.CloseHandle(dir)

	event, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		return
	}
	defer windows.CloseHandle(event)

	buffer := make([]byte, 64*1024)
	// A file that keeps growing must not hold the report back, so the first
	// change starts the delay and later ones join it.
	var pending atomic.Bool
	for {
		overlapped := windows.Overlapped{HEvent: event}
		windows.ResetEvent(event)
		err := windows.ReadDirectoryChanges(dir, &buffer[0], uint32(len(buffer)), false, notifyFilter, nil, &overlapped, 0)
		if err != nil {
			return
		}
		handles := []windows.Handle{event, stop}
		result, err := windows.WaitForMultipleObjects(handles, false, windows.INFINITE)
		if err != nil || result != windows.WAIT_OBJECT_0 {
			windows.CancelIoEx(dir, &overlapped)
			var done uint32
			windows.GetOverlappedResult(dir, &overlapped, &done, true)
			return
		}
		var done uint32
		if err := windows.GetOverlappedResult(dir, &overlapped, &done, false); err != nil {
			// The folder is gone or no longer readable. One last report lets the
			// list show that.
			w.changed(path)
			return
		}
		if pending.CompareAndSwap(false, true) {
			time.AfterFunc(Delay, func() {
				pending.Store(false)
				w.changed(path)
			})
		}
	}
}
