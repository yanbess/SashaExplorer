package files

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Pictures that Windows makes thumbnails of, and videos to use when a folder
// has too few pictures.
var (
	pictureTypes = setOf(".jpg", ".jpeg", ".jfif", ".png", ".gif", ".webp", ".bmp", ".heic", ".heif", ".avif", ".tif", ".tiff", ".jxl", ".dng", ".cr2", ".cr3", ".nef", ".arw")
	videoTypes   = setOf(".mp4", ".m4v", ".mov", ".mkv", ".webm", ".avi", ".wmv")
)

func setOf(items ...string) map[string]bool {
	set := make(map[string]bool, len(items))
	for _, item := range items {
		set[item] = true
	}
	return set
}

// Picture is a file whose thumbnail stands for its folder.
type Picture struct {
	Path     string `json:"path"`
	Modified int64  `json:"modified"`
}

// Pictures returns up to limit pictures of a folder, by name, for the folder's
// icon. Videos fill in when there are fewer pictures. Hidden files are left out.
func Pictures(dir string, limit int) []Picture {
	items, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var pictures, videos []os.DirEntry
	for _, item := range items {
		if item.IsDir() || strings.HasPrefix(item.Name(), ".") {
			continue
		}
		ext := strings.ToLower(filepath.Ext(item.Name()))
		if pictureTypes[ext] {
			pictures = append(pictures, item)
		} else if videoTypes[ext] {
			videos = append(videos, item)
		}
	}
	byName := func(list []os.DirEntry) {
		sort.SliceStable(list, func(i, j int) bool {
			return strings.ToLower(list[i].Name()) < strings.ToLower(list[j].Name())
		})
	}
	byName(pictures)
	byName(videos)
	var result []Picture
	for _, item := range append(pictures, videos...) {
		if len(result) >= limit {
			break
		}
		info, err := item.Info()
		if err != nil {
			continue
		}
		if entry := entryOf(filepath.Join(dir, item.Name()), info); !entry.Hidden {
			result = append(result, Picture{Path: entry.Path, Modified: entry.Modified})
		}
	}
	return result
}
