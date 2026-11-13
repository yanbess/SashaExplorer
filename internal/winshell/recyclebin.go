package winshell

import (
	"errors"
	"os/exec"
	"syscall"
	"unsafe"
)

var (
	procSHQueryRecycleBinW = shell32.NewProc("SHQueryRecycleBinW")
	procSHEmptyRecycleBinW = shell32.NewProc("SHEmptyRecycleBinW")
)

// shQueryRBInfo is SHQUERYRBINFO. Go aligns the int64 fields to 8 bytes, as the
// 64-bit headers do.
type shQueryRBInfo struct {
	Size  uint32
	Bytes int64
	Items int64
}

// RecycleBin tells what the Recycle Bins of all drives hold.
type RecycleBin struct {
	Items int64 `json:"items"`
	Bytes int64 `json:"bytes"`
}

// QueryRecycleBin adds up the Recycle Bins of all drives.
func QueryRecycleBin() (RecycleBin, error) {
	info := shQueryRBInfo{Size: uint32(unsafe.Sizeof(shQueryRBInfo{}))}
	hr, _, _ := procSHQueryRecycleBinW.Call(0, uintptr(unsafe.Pointer(&info)))
	if err := failed(hr); err != nil {
		return RecycleBin{}, err
	}
	return RecycleBin{Items: info.Items, Bytes: info.Bytes}, nil
}

// EmptyRecycleBin empties the Recycle Bins of all drives. Windows asks first and
// shows its progress.
func (u *UI) EmptyRecycleBin() error {
	owner := u.ownerWindow()
	var err error
	onOwnThread(func() {
		hr, _, _ := procSHEmptyRecycleBinW.Call(owner, 0, 0)
		err = failed(hr)
	})
	var errno syscall.Errno
	if errors.As(err, &errno) && uint32(errno) == 0x800704C7 { // ERROR_CANCELLED
		return ErrCancelled
	}
	return err
}

// OpenRecycleBin shows the Recycle Bin in Explorer, which can restore items.
func OpenRecycleBin() error {
	return exec.Command("explorer.exe", "shell:RecycleBinFolder").Start()
}
