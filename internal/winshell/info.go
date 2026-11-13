package winshell

import (
	"errors"
	"strings"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	shgfiTypeName          = 0x400
	shgfiUseFileAttributes = 0x10
	fileAttributeNormal    = 0x80
)

type shFileInfo struct {
	Icon        uintptr
	IconIndex   int32
	Attributes  uint32
	DisplayName [windows.MAX_PATH]uint16
	TypeName    [80]uint16
}

var typeNames sync.Map

// TypeNames returns the names Windows gives to file types, such as "Text
// Document", for extensions with their dot. They come from the registry and are
// remembered.
func (p *Pool) TypeNames(extensions []string) map[string]string {
	names := make(map[string]string, len(extensions))
	var missing []string
	for _, ext := range extensions {
		key := strings.ToLower(ext)
		if name, ok := typeNames.Load(key); ok {
			names[ext] = name.(string)
		} else {
			missing = append(missing, ext)
		}
	}
	if len(missing) == 0 {
		return names
	}
	p.Do(func() {
		for _, ext := range missing {
			var info shFileInfo
			r, _, _ := procSHGetFileInfoW.Call(
				uintptr(unsafe.Pointer(utf16Ptr("file"+ext))), fileAttributeNormal,
				uintptr(unsafe.Pointer(&info)), unsafe.Sizeof(info),
				shgfiTypeName|shgfiUseFileAttributes,
			)
			name := ""
			if r != 0 {
				name = windows.UTF16ToString(info.TypeName[:])
			}
			typeNames.Store(strings.ToLower(ext), name)
			names[ext] = name
		}
	})
	return names
}

// LinkTarget returns the path a shortcut (.lnk) points to, or "" for a shortcut
// to something that is not a file, such as a control panel item.
func (p *Pool) LinkTarget(path string) (string, error) {
	var (
		target string
		err    error
	)
	p.Do(func() { target, err = linkTarget(path) })
	return target, err
}

func linkTarget(path string) (string, error) {
	var link comObject
	hr, _, _ := procCoCreateInstance.Call(
		uintptr(unsafe.Pointer(&clsidShellLink)), 0, 1, // CLSCTX_INPROC_SERVER
		uintptr(unsafe.Pointer(&iidShellLinkW)), uintptr(unsafe.Pointer(&link)),
	)
	if err := failed(hr); err != nil {
		return "", err
	}
	defer link.release()

	file, ok := link.queryInterface(&iidPersistFile)
	if !ok {
		return "", errors.New("shortcut cannot be read")
	}
	defer file.release()
	// IPersistFile::Load with STGM_READ.
	if err := failed(file.call(5, uintptr(unsafe.Pointer(utf16Ptr(path))), 0)); err != nil {
		return "", err
	}

	var buffer [windows.MAX_PATH * 4]uint16
	// IShellLinkW::GetPath. It returns S_FALSE without a path.
	if err := failed(link.call(3, uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)), 0, 0)); err != nil {
		return "", err
	}
	return windows.UTF16ToString(buffer[:]), nil
}
