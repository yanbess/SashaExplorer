package files

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf16"

	"golang.org/x/sys/windows"
)

// The Recycle Bin keeps each deleted item as two files in
// X:\$Recycle.Bin\<user SID>: $Rxxxxxx.ext is the item itself, and $Ixxxxxx.ext
// tells where it came from and when it was deleted. Restoring moves the item back
// and removes its record, which is what Explorer does too.

// recycled is one item in the Recycle Bin.
type recycled struct {
	original string
	deleted  time.Time
	size     int64
	info     string // The $I file.
	item     string // The $R file or folder.
}

func userSID() (string, error) {
	token := windows.GetCurrentProcessToken()
	user, err := token.GetTokenUser()
	if err != nil {
		return "", err
	}
	return user.User.Sid.String(), nil
}

// parseRecycleInfo reads a $I file: a version, the size, the deletion time, and
// the original path.
func parseRecycleInfo(content []byte) (string, time.Time, error) {
	if len(content) < 24 {
		return "", time.Time{}, errors.New("record is too short")
	}
	version := binary.LittleEndian.Uint64(content[0:8])
	filetime := windows.Filetime{
		LowDateTime:  binary.LittleEndian.Uint32(content[16:20]),
		HighDateTime: binary.LittleEndian.Uint32(content[20:24]),
	}
	deleted := time.Unix(0, filetime.Nanoseconds())
	var raw []byte
	switch version {
	case 1:
		raw = content[24:]
	case 2:
		if len(content) < 28 {
			return "", time.Time{}, errors.New("record is too short")
		}
		count := int(binary.LittleEndian.Uint32(content[24:28]))
		raw = content[28:]
		if count*2 < len(raw) {
			raw = raw[:count*2]
		}
	default:
		return "", time.Time{}, fmt.Errorf("unknown record version %d", version)
	}
	units := make([]uint16, len(raw)/2)
	for i := range units {
		units[i] = binary.LittleEndian.Uint16(raw[2*i:])
	}
	for i, unit := range units {
		if unit == 0 {
			units = units[:i]
			break
		}
	}
	return string(utf16.Decode(units)), deleted, nil
}

func recycleBinOf(path string) (string, error) {
	sid, err := userSID()
	if err != nil {
		return "", err
	}
	volume := filepath.VolumeName(path)
	if volume == "" {
		return "", errors.New("the item has no drive")
	}
	return filepath.Join(volume+`\`, "$Recycle.Bin", sid), nil
}

func listRecycled(bin string) []recycled {
	names, err := os.ReadDir(bin)
	if err != nil {
		return nil
	}
	var items []recycled
	for _, name := range names {
		if !strings.HasPrefix(name.Name(), "$I") {
			continue
		}
		info := filepath.Join(bin, name.Name())
		content, err := os.ReadFile(info)
		if err != nil {
			continue
		}
		original, deleted, err := parseRecycleInfo(content)
		if err != nil {
			continue
		}
		items = append(items, recycled{
			original: original,
			deleted:  deleted,
			size:     recycledSize(content),
			info:     info,
			item:     filepath.Join(bin, "$R"+name.Name()[2:]),
		})
	}
	return items
}

// Restore brings items back from the Recycle Bin to where they were. For each
// path it takes the newest item deleted after since. It returns the paths it
// restored and an error for the first one it could not.
func Restore(paths []string, since time.Time) ([]string, error) {
	bins := map[string][]recycled{}
	var restored []string
	var firstErr error
	for _, path := range paths {
		bin, err := recycleBinOf(path)
		if err != nil {
			firstErr = cmpErr(firstErr, err)
			continue
		}
		if _, ok := bins[bin]; !ok {
			bins[bin] = listRecycled(bin)
		}
		best := -1
		for i, item := range bins[bin] {
			if strings.EqualFold(item.original, path) && !item.deleted.Before(since) &&
				(best < 0 || item.deleted.After(bins[bin][best].deleted)) {
				best = i
			}
		}
		if best < 0 {
			firstErr = cmpErr(firstErr, fmt.Errorf("%s is no longer in the Recycle Bin", filepath.Base(path)))
			continue
		}
		item := bins[bin][best]
		if _, err := os.Lstat(path); err == nil {
			firstErr = cmpErr(firstErr, fmt.Errorf("%s already exists", path))
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			firstErr = cmpErr(firstErr, describe(err, path))
			continue
		}
		if err := os.Rename(item.item, path); err != nil {
			firstErr = cmpErr(firstErr, describe(err, path))
			continue
		}
		os.Remove(item.info)
		bins[bin] = append(bins[bin][:best], bins[bin][best+1:]...)
		restored = append(restored, path)
	}
	return restored, firstErr
}

func cmpErr(first, next error) error {
	if first != nil {
		return first
	}
	return next
}
