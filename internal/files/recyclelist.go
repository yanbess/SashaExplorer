package files

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// The Recycle Bin view lists the items of every drive's Recycle Bin. Each item
// is its $R file or folder, named and dated as it was before deletion.

// ListRecycleBin lists the items in the Recycle Bins of the drives given as
// roots, such as "C:\". Drives without a Recycle Bin are skipped.
func ListRecycleBin(roots []string) []Entry {
	sid, err := userSID()
	if err != nil {
		return nil
	}
	var entries []Entry
	for _, root := range roots {
		bin := filepath.Join(filepath.VolumeName(root)+`\`, "$Recycle.Bin", sid)
		for _, item := range listRecycled(bin) {
			info, err := os.Lstat(item.item)
			if err != nil {
				// A record without its item is left over; Windows ignores it too.
				continue
			}
			entry := entryOf(item.item, info)
			entry.Name = filepath.Base(item.original)
			entry.Original = item.original
			entry.Deleted = item.deleted.UnixMilli()
			entry.Size = item.size
			entry.Ext = ""
			if !entry.Dir {
				entry.Ext = strings.ToLower(filepath.Ext(entry.Name))
			}
			entries = append(entries, entry)
		}
	}
	return entries
}

// recycledItem checks that path is an item in a Recycle Bin of this user, and
// returns the path of its record.
func recycledItem(path string) (string, error) {
	bin, err := recycleBinOf(path)
	if err != nil {
		return "", err
	}
	base := filepath.Base(path)
	if !strings.EqualFold(filepath.Dir(filepath.Clean(path)), bin) || !strings.HasPrefix(base, "$R") || len(base) < 3 {
		return "", fmt.Errorf("%s is not in the Recycle Bin", path)
	}
	return filepath.Join(bin, "$I"+base[2:]), nil
}

// RestoreRecycled moves items of the Recycle Bin, given by their $R paths, back
// to where they were deleted from. It returns their restored paths, and an error
// for the first that could not go back, such as one whose place is taken.
func RestoreRecycled(items []string) ([]string, error) {
	var restored []string
	var result error
	for _, item := range items {
		record, err := recycledItem(item)
		if err != nil {
			result = cmpErr(result, err)
			continue
		}
		content, err := os.ReadFile(record)
		if err != nil {
			result = cmpErr(result, describe(err, record))
			continue
		}
		original, _, err := parseRecycleInfo(content)
		if err != nil {
			result = cmpErr(result, err)
			continue
		}
		if _, err := os.Lstat(original); err == nil {
			result = cmpErr(result, fmt.Errorf("%s already exists", original))
			continue
		}
		if err := os.MkdirAll(filepath.Dir(original), 0o755); err != nil {
			result = cmpErr(result, describe(err, original))
			continue
		}
		if err := os.Rename(item, original); err != nil {
			result = cmpErr(result, describe(err, original))
			continue
		}
		os.Remove(record)
		restored = append(restored, original)
	}
	return restored, result
}

// PurgeRecycled deletes items of the Recycle Bin for good.
func PurgeRecycled(items []string) error {
	var result error
	for _, item := range items {
		record, err := recycledItem(item)
		if err != nil {
			result = errors.Join(result, err)
			continue
		}
		if err := os.RemoveAll(item); err != nil {
			result = errors.Join(result, describe(err, item))
			continue
		}
		os.Remove(record)
	}
	return result
}

// recycledSize reads the size a $I record keeps, which for a folder is the size
// of all it held.
func recycledSize(content []byte) int64 {
	if len(content) < 16 {
		return 0
	}
	return int64(binary.LittleEndian.Uint64(content[8:16]))
}
