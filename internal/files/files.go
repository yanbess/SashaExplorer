// Package files reads folders and files for the file list and the preview panel.
package files

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unicode/utf16"
	"unicode/utf8"

	"golang.org/x/sys/windows"
)

// Entry is an item of a folder.
type Entry struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Dir      bool   `json:"dir"`
	Size     int64  `json:"size"`
	Modified int64  `json:"modified"`
	Created  int64  `json:"created"`
	Ext      string `json:"ext"`
	Hidden   bool   `json:"hidden"`
	System   bool   `json:"system"`
	Link     bool   `json:"link"`
	// Original and Deleted are set for items of the Recycle Bin: where the item
	// was, and when it was deleted, in Unix milliseconds.
	Original string `json:"original,omitempty"`
	Deleted  int64  `json:"deleted,omitempty"`
}

// Listing is the content of a folder.
type Listing struct {
	Path    string  `json:"path"`
	Entries []Entry `json:"entries"`
	// Unreadable counts items that could not be read, such as locked system files.
	Unreadable int `json:"unreadable"`
}

// Clean makes a path absolute and keeps the backslash of a drive root, which
// "C:" without it would lose: that means the current folder of drive C.
func Clean(path string) (string, error) {
	path = strings.TrimSpace(path)
	path = strings.Trim(path, `"`)
	if path == "" {
		return "", errors.New("the path is empty")
	}
	if expanded, err := expandEnv(path); err == nil {
		path = expanded
	}
	path = strings.ReplaceAll(path, "/", `\`)
	if len(path) == 2 && path[1] == ':' {
		path += `\`
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return abs, nil
}

func expandEnv(path string) (string, error) {
	if !strings.Contains(path, "%") {
		return path, nil
	}
	source, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return "", err
	}
	buffer := make([]uint16, 1024)
	for {
		n, err := windows.ExpandEnvironmentStrings(source, &buffer[0], uint32(len(buffer)))
		if err != nil {
			return "", err
		}
		if int(n) <= len(buffer) {
			return windows.UTF16ToString(buffer[:n]), nil
		}
		buffer = make([]uint16, n)
	}
}

// List reads a folder.
func List(path string) (Listing, error) {
	path, err := Clean(path)
	if err != nil {
		return Listing{}, err
	}
	items, err := os.ReadDir(path)
	if err != nil && len(items) == 0 {
		return Listing{}, describe(err, path)
	}
	listing := Listing{Path: path, Entries: make([]Entry, 0, len(items))}
	for _, item := range items {
		info, err := item.Info()
		if err != nil {
			listing.Unreadable++
			continue
		}
		listing.Entries = append(listing.Entries, entryOf(filepath.Join(path, item.Name()), info))
	}
	return listing, nil
}

// Stat describes one item.
func Stat(path string) (Entry, error) {
	path, err := Clean(path)
	if err != nil {
		return Entry{}, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return Entry{}, describe(err, path)
	}
	return entryOf(path, info), nil
}

func entryOf(path string, info fs.FileInfo) Entry {
	entry := Entry{
		Name:     info.Name(),
		Path:     path,
		Dir:      info.IsDir(),
		Size:     info.Size(),
		Modified: info.ModTime().UnixMilli(),
		Link:     info.Mode()&(fs.ModeSymlink|fs.ModeIrregular) != 0,
	}
	if data, ok := info.Sys().(*syscall.Win32FileAttributeData); ok {
		attributes := data.FileAttributes
		entry.Dir = attributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0
		entry.Hidden = attributes&windows.FILE_ATTRIBUTE_HIDDEN != 0
		entry.System = attributes&windows.FILE_ATTRIBUTE_SYSTEM != 0
		entry.Created = time.Unix(0, data.CreationTime.Nanoseconds()).UnixMilli()
	}
	if entry.Dir {
		entry.Size = 0
	} else {
		entry.Ext = strings.ToLower(filepath.Ext(entry.Name))
	}
	return entry
}

// describe turns an error into a message without the Go prefixes.
func describe(err error, path string) error {
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("%s was not found", path)
	case errors.Is(err, fs.ErrPermission):
		return fmt.Errorf("access to %s is denied", path)
	}
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		return fmt.Errorf("%s: %w", path, pathErr.Err)
	}
	return err
}

// Details adds what the list does not need to an entry.
type Details struct {
	Entry
	Accessed int64 `json:"accessed"`
	ReadOnly bool  `json:"readOnly"`
	Archive  bool  `json:"archive"`
	// Items counts the direct content of a folder, or -1 if it cannot be read.
	Items int `json:"items"`
}

// Describe returns details of one item.
func Describe(path string) (Details, error) {
	entry, err := Stat(path)
	if err != nil {
		return Details{}, err
	}
	details := Details{Entry: entry, Items: -1}
	if info, err := os.Lstat(entry.Path); err == nil {
		if data, ok := info.Sys().(*syscall.Win32FileAttributeData); ok {
			details.Accessed = time.Unix(0, data.LastAccessTime.Nanoseconds()).UnixMilli()
			details.ReadOnly = data.FileAttributes&windows.FILE_ATTRIBUTE_READONLY != 0
			details.Archive = data.FileAttributes&windows.FILE_ATTRIBUTE_ARCHIVE != 0
		}
	}
	if entry.Dir {
		if dir, err := os.Open(entry.Path); err == nil {
			names, _ := dir.Readdirnames(-1)
			dir.Close()
			details.Items = len(names)
		}
	}
	return details, nil
}

// Size is the total of a folder.
type Size struct {
	Bytes      int64 `json:"bytes"`
	Files      int   `json:"files"`
	Folders    int   `json:"folders"`
	Unreadable int   `json:"unreadable"`
}

// FolderSize adds up a folder. It does not follow links, so a junction that
// points to a parent cannot make it loop.
func FolderSize(ctx context.Context, path string) (Size, error) {
	path, err := Clean(path)
	if err != nil {
		return Size{}, err
	}
	var size Size
	err = filepath.WalkDir(path, func(current string, item fs.DirEntry, err error) error {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if err != nil {
			size.Unreadable++
			if item != nil && item.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if current == path {
			return nil
		}
		if item.IsDir() {
			size.Folders++
			return nil
		}
		info, err := item.Info()
		if err != nil {
			size.Unreadable++
			return nil
		}
		if info.Mode().IsRegular() {
			size.Files++
			size.Bytes += info.Size()
		}
		return nil
	})
	return size, err
}

// MaxTextPreview limits how much of a file the text preview reads.
const MaxTextPreview = 64 * 1024

// TextPreview is the start of a text file.
type TextPreview struct {
	Text      string `json:"text"`
	Truncated bool   `json:"truncated"`
	Binary    bool   `json:"binary"`
}

// ReadText reads the start of a file for the preview. A file that does not look
// like text gives Binary.
func ReadText(path string) (TextPreview, error) {
	file, err := os.Open(path)
	if err != nil {
		return TextPreview{}, describe(err, path)
	}
	defer file.Close()
	buffer := make([]byte, MaxTextPreview+1)
	n, err := io.ReadFull(file, buffer)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return TextPreview{}, describe(err, path)
	}
	truncated := n > MaxTextPreview
	content := buffer[:min(n, MaxTextPreview)]
	text, ok := decodeText(content)
	if !ok {
		return TextPreview{Binary: true}, nil
	}
	return TextPreview{Text: text, Truncated: truncated}, nil
}

func decodeText(content []byte) (string, bool) {
	switch {
	case len(content) >= 2 && content[0] == 0xFF && content[1] == 0xFE:
		return decodeUTF16(content[2:], false), true
	case len(content) >= 2 && content[0] == 0xFE && content[1] == 0xFF:
		return decodeUTF16(content[2:], true), true
	case len(content) >= 3 && content[0] == 0xEF && content[1] == 0xBB && content[2] == 0xBF:
		content = content[3:]
	}
	control := 0
	for _, b := range content {
		if b == 0 {
			return "", false
		}
		if b < 0x20 && b != '\n' && b != '\r' && b != '\t' && b != '\f' && b != 0x1b {
			control++
		}
	}
	if len(content) > 0 && control*20 > len(content) {
		return "", false
	}
	// The end may cut a character in half.
	for i := 0; i < 3 && len(content) > 0 && !utf8.Valid(content); i++ {
		content = content[:len(content)-1]
	}
	return strings.ToValidUTF8(string(content), "�"), true
}

func decodeUTF16(content []byte, bigEndian bool) string {
	units := make([]uint16, len(content)/2)
	for i := range units {
		if bigEndian {
			units[i] = uint16(content[2*i])<<8 | uint16(content[2*i+1])
		} else {
			units[i] = uint16(content[2*i+1])<<8 | uint16(content[2*i])
		}
	}
	return string(utf16.Decode(units))
}

// invalidName lists characters Windows does not allow in names.
const invalidName = `<>:"/\|?*`

// CheckName rejects names Windows cannot use.
func CheckName(name string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("enter a name")
	}
	if name == "." || name == ".." {
		return errors.New("this name is reserved")
	}
	if strings.ContainsAny(name, invalidName) {
		return fmt.Errorf("a name cannot contain any of %s", invalidName)
	}
	for _, r := range name {
		if r < 32 {
			return errors.New("a name cannot contain control characters")
		}
	}
	if strings.HasSuffix(name, " ") || strings.HasSuffix(name, ".") {
		return errors.New("a name cannot end with a space or a dot")
	}
	base := strings.ToUpper(strings.TrimSpace(strings.SplitN(name, ".", 2)[0]))
	switch base {
	case "CON", "PRN", "AUX", "NUL",
		"COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
		"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9":
		return fmt.Errorf("%s is reserved by Windows", base)
	}
	return nil
}

// CreateFolder makes a folder in parent.
func CreateFolder(parent, name string) (string, error) {
	if err := CheckName(name); err != nil {
		return "", err
	}
	path := filepath.Join(parent, name)
	if err := os.Mkdir(path, 0o755); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return "", fmt.Errorf("%s already exists", name)
		}
		return "", describe(err, path)
	}
	return path, nil
}

// CreateFile makes an empty file in parent. It never replaces a file.
func CreateFile(parent, name string) (string, error) {
	if err := CheckName(name); err != nil {
		return "", err
	}
	path := filepath.Join(parent, name)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		if errors.Is(err, fs.ErrExist) {
			return "", fmt.Errorf("%s already exists", name)
		}
		return "", describe(err, path)
	}
	return path, file.Close()
}

// Rename gives an item a new name in its folder. Changing only the case of
// letters works too.
func Rename(path, name string) (string, error) {
	if err := CheckName(name); err != nil {
		return "", err
	}
	target := filepath.Join(filepath.Dir(path), name)
	if target == path {
		return path, nil
	}
	if !strings.EqualFold(target, path) {
		if _, err := os.Lstat(target); err == nil {
			return "", fmt.Errorf("%s already exists", name)
		}
	}
	if err := os.Rename(path, target); err != nil {
		return "", describe(err, path)
	}
	return target, nil
}
