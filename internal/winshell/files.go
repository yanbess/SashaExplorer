package winshell

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// SHFileOperation functions and flags.
const (
	foMove   = 1
	foCopy   = 2
	foDelete = 3

	fofRenameOnCollision = 0x0008
	fofNoConfirmation    = 0x0010
	fofAllowUndo         = 0x0040
	fofNoConfirmMkdir    = 0x0200
	fofWantNukeWarning   = 0x4000
)

type fileOpStruct struct {
	Hwnd          uintptr
	Func          uint32
	From          *uint16
	To            *uint16
	Flags         uint16
	Aborted       int32
	NameMappings  uintptr
	ProgressTitle *uint16
}

// ErrCancelled means the user stopped the operation in a Windows dialog.
var ErrCancelled = errors.New("cancelled")

// Copy copies paths into the folder dest. Copies into the same folder get names
// such as "name - Copy", as in Windows.
func (u *UI) Copy(paths []string, dest string) error {
	flags := uint16(fofAllowUndo | fofNoConfirmMkdir)
	if allIn(paths, dest) {
		flags |= fofRenameOnCollision
	}
	return u.fileOperation(foCopy, paths, dest, flags)
}

// Move moves paths into the folder dest. Items already there stay.
func (u *UI) Move(paths []string, dest string) error {
	var moving []string
	for _, path := range paths {
		if !sameFolder(filepath.Dir(path), dest) {
			moving = append(moving, path)
		}
	}
	if len(moving) == 0 {
		return nil
	}
	return u.fileOperation(foMove, moving, dest, fofAllowUndo|fofNoConfirmMkdir)
}

// Delete moves paths to the Recycle Bin, or deletes them for good. Windows still
// warns when an item is too large for the Recycle Bin.
func (u *UI) Delete(paths []string, permanent bool) error {
	flags := uint16(fofNoConfirmation)
	if !permanent {
		flags |= fofAllowUndo | fofWantNukeWarning
	}
	return u.fileOperation(foDelete, paths, "", flags)
}

// fileOperation runs on a thread of its own, because it can take minutes while
// Windows shows its progress and conflict dialogs.
func (u *UI) fileOperation(function uint32, paths []string, dest string, flags uint16) error {
	if len(paths) == 0 {
		return nil
	}
	from := doubleNullList(paths)
	op := fileOpStruct{Hwnd: u.ownerWindow(), Func: function, From: &from[0], Flags: flags}
	var to []uint16
	if dest != "" {
		to = doubleNullList([]string{dest})
		op.To = &to[0]
	}
	var result uintptr
	onOwnThread(func() {
		result, _, _ = procSHFileOperationW.Call(uintptr(unsafe.Pointer(&op)))
	})
	if op.Aborted != 0 {
		return ErrCancelled
	}
	if result != 0 {
		return fmt.Errorf("windows could not finish the operation (code 0x%X)", result)
	}
	return nil
}

func allIn(paths []string, folder string) bool {
	for _, path := range paths {
		if !sameFolder(filepath.Dir(path), folder) {
			return false
		}
	}
	return true
}

func sameFolder(a, b string) bool {
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

// Clipboard formats.
const (
	cfHDrop       = 15
	gmemMoveable  = 0x0002
	gmemZeroInit  = 0x0040
	dropEffectCpy = 1
	dropEffectMov = 2
)

var (
	procOpenClipboard              = user32.NewProc("OpenClipboard")
	procCloseClipboard             = user32.NewProc("CloseClipboard")
	procEmptyClipboard             = user32.NewProc("EmptyClipboard")
	procSetClipboardData           = user32.NewProc("SetClipboardData")
	procGetClipboardData           = user32.NewProc("GetClipboardData")
	procIsClipboardFormatAvailable = user32.NewProc("IsClipboardFormatAvailable")
	procRegisterClipboardFormatW   = user32.NewProc("RegisterClipboardFormatW")
	procGetClipboardSequenceNumber = user32.NewProc("GetClipboardSequenceNumber")

	preferredDropEffect, _, _ = procRegisterClipboardFormatW.Call(uintptr(unsafe.Pointer(utf16Ptr("Preferred DropEffect"))))
)

// SetClipboardFiles puts files on the Windows clipboard the way Explorer does,
// so they can be pasted there too. cut marks them to be moved.
func (u *UI) SetClipboardFiles(paths []string, cut bool) (sequence uint32, err error) {
	u.Do(func() {
		if r, _, callErr := procOpenClipboard.Call(u.hwnd); r == 0 {
			err = fmt.Errorf("open clipboard: %w", callErr)
			return
		}
		defer procCloseClipboard.Call()
		procEmptyClipboard.Call()

		// DROPFILES: offset of the list, a point, two flags. The list is UTF-16.
		list := doubleNullList(paths)
		const headerSize = 20
		drop, lockErr := globalBytes(headerSize + len(list)*2)
		if lockErr != nil {
			err = lockErr
			return
		}
		buffer := lockGlobal(drop, headerSize+len(list)*2)
		*(*uint32)(unsafe.Pointer(&buffer[0])) = headerSize
		*(*uint32)(unsafe.Pointer(&buffer[16])) = 1 // Wide characters.
		copy(buffer[headerSize:], unsafe.Slice((*byte)(unsafe.Pointer(&list[0])), len(list)*2))
		procGlobalUnlock.Call(drop)
		if r, _, callErr := procSetClipboardData.Call(cfHDrop, drop); r == 0 {
			procGlobalFree.Call(drop)
			err = fmt.Errorf("set clipboard: %w", callErr)
			return
		}

		effect, effectErr := globalBytes(4)
		if effectErr == nil {
			value := uint32(dropEffectCpy)
			if cut {
				value = dropEffectMov
			}
			buffer := lockGlobal(effect, 4)
			*(*uint32)(unsafe.Pointer(&buffer[0])) = value
			procGlobalUnlock.Call(effect)
			if r, _, _ := procSetClipboardData.Call(preferredDropEffect, effect); r == 0 {
				procGlobalFree.Call(effect)
			}
		}
	})
	r, _, _ := procGetClipboardSequenceNumber.Call()
	return uint32(r), err
}

// ClipboardFiles returns the files on the clipboard and whether they were cut.
func (u *UI) ClipboardFiles() (paths []string, cut bool, sequence uint32, err error) {
	u.Do(func() {
		if r, _, callErr := procOpenClipboard.Call(u.hwnd); r == 0 {
			err = fmt.Errorf("open clipboard: %w", callErr)
			return
		}
		defer procCloseClipboard.Call()

		drop, _, _ := procGetClipboardData.Call(cfHDrop)
		if drop == 0 {
			return
		}
		count, _, _ := procDragQueryFileW.Call(drop, 0xFFFFFFFF, 0, 0)
		for i := range count {
			length, _, _ := procDragQueryFileW.Call(drop, i, 0, 0)
			buffer := make([]uint16, length+1)
			procDragQueryFileW.Call(drop, i, uintptr(unsafe.Pointer(&buffer[0])), length+1)
			paths = append(paths, windows.UTF16ToString(buffer))
		}

		if effect, _, _ := procGetClipboardData.Call(preferredDropEffect); effect != 0 {
			if pointer, _, _ := procGlobalLock.Call(effect); pointer != 0 {
				value := *(*uint32)(unsafe.Pointer(pointer))
				cut = value&dropEffectMov != 0 && value&dropEffectCpy == 0
				procGlobalUnlock.Call(effect)
			}
		}
	})
	r, _, _ := procGetClipboardSequenceNumber.Call()
	return paths, cut, uint32(r), err
}

// ClearClipboardIf empties the clipboard if it has not changed since sequence.
// Files that were cut and then moved are no longer there to paste.
func (u *UI) ClearClipboardIf(sequence uint32) {
	u.Do(func() {
		if r, _, _ := procGetClipboardSequenceNumber.Call(); uint32(r) != sequence {
			return
		}
		if r, _, _ := procOpenClipboard.Call(u.hwnd); r == 0 {
			return
		}
		procEmptyClipboard.Call()
		procCloseClipboard.Call()
	})
}

// HasClipboardFiles reports whether there are files to paste.
func HasClipboardFiles() bool {
	r, _, _ := procIsClipboardFormatAvailable.Call(cfHDrop)
	return r != 0
}

func globalBytes(size int) (uintptr, error) {
	handle, _, err := procGlobalAlloc.Call(gmemMoveable|gmemZeroInit, uintptr(size))
	if handle == 0 {
		return 0, fmt.Errorf("allocate clipboard memory: %w", err)
	}
	return handle, nil
}

func lockGlobal(handle uintptr, size int) []byte {
	pointer, _, _ := procGlobalLock.Call(handle)
	return unsafe.Slice((*byte)(unsafe.Pointer(pointer)), size)
}
