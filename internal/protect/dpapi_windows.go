//go:build windows

package protect

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// DPAPI protects data with the Windows account of the current user.
type DPAPI struct {
	// Entropy is extra input that other programs of this user do not know by
	// default. It separates this data from other DPAPI data.
	Entropy []byte
}

func blob(data []byte) *windows.DataBlob {
	if len(data) == 0 {
		return &windows.DataBlob{}
	}
	return &windows.DataBlob{Size: uint32(len(data)), Data: &data[0]}
}

func takeBlob(out *windows.DataBlob) []byte {
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	return append([]byte(nil), unsafe.Slice(out.Data, out.Size)...)
}

// Protect encrypts data.
func (d DPAPI) Protect(data []byte) ([]byte, error) {
	var out windows.DataBlob
	err := windows.CryptProtectData(blob(data), nil, blob(d.Entropy), 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out)
	if err != nil {
		return nil, err
	}
	return takeBlob(&out), nil
}

// Unprotect decrypts data.
func (d DPAPI) Unprotect(data []byte) ([]byte, error) {
	var out windows.DataBlob
	err := windows.CryptUnprotectData(blob(data), nil, blob(d.Entropy), 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out)
	if err != nil {
		return nil, err
	}
	return takeBlob(&out), nil
}
