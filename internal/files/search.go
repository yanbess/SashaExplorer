package files

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

// MaxResults limits a search, so a search for "a" on a whole drive stays usable.
const MaxResults = 20000

// Matcher tells whether a name fits a query. A query with * or ? is a pattern,
// such as "*.jpg"; other text matches anywhere in the name. Case does not matter.
func Matcher(query string) func(name string) bool {
	query = strings.ToLower(strings.TrimSpace(query))
	if strings.ContainsAny(query, "*?") {
		return func(name string) bool {
			ok, err := filepath.Match(query, strings.ToLower(name))
			return err == nil && ok
		}
	}
	return func(name string) bool { return strings.Contains(strings.ToLower(name), query) }
}

// Search looks for names in root and its subfolders and passes what it finds to
// found as it goes. Hidden items are left out, with what is inside hidden
// folders, unless hidden is true. It does not follow links, so a junction that
// points back up cannot make it loop. It returns whether it stopped at
// MaxResults.
func Search(ctx context.Context, root, query string, hidden bool, found func(Entry)) (bool, error) {
	root, err := Clean(root)
	if err != nil {
		return false, err
	}
	match := Matcher(query)
	count := 0
	err = filepath.WalkDir(root, func(path string, item fs.DirEntry, err error) error {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if err != nil {
			if item != nil && item.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if path == root {
			return nil
		}
		info, err := item.Info()
		if err != nil {
			return nil
		}
		if !hidden && isHidden(info) {
			if item.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if match(item.Name()) {
			found(entryOf(path, info))
			count++
			if count >= MaxResults {
				return fs.SkipAll
			}
		}
		return nil
	})
	return count >= MaxResults, err
}

func isHidden(info fs.FileInfo) bool {
	data, ok := info.Sys().(*syscall.Win32FileAttributeData)
	return ok && data.FileAttributes&windows.FILE_ATTRIBUTE_HIDDEN != 0
}

// Names returns the names in a folder in lower case, to compare a folder before
// and after an operation.
func Names(dir string) map[string]bool {
	names := map[string]bool{}
	file, err := os.Open(dir)
	if err != nil {
		return names
	}
	defer file.Close()
	list, _ := file.Readdirnames(-1)
	for _, name := range list {
		names[strings.ToLower(name)] = true
	}
	return names
}

// Pair is where an item went.
type Pair struct {
	From string
	To   string
}

// Arrivals works out where items went after they were copied or moved into
// dest, given the names dest had before. Windows may rename an item that meets
// another with its name, such as "photo (2).jpg" or "photo - Copy.jpg", so a new
// name that starts like the item counts too. Items that did not arrive, because
// the user skipped them, are left out. replaced tells whether an item that
// replaced one of the same name counts.
func Arrivals(sources []string, dest string, before map[string]bool, replaced bool) []Pair {
	after := Names(dest)
	fresh := map[string]bool{}
	for name := range after {
		if !before[name] {
			fresh[name] = true
		}
	}
	var pairs []Pair
	var waiting []string
	for _, source := range sources {
		name := strings.ToLower(filepath.Base(source))
		if fresh[name] {
			delete(fresh, name)
			pairs = append(pairs, Pair{source, filepath.Join(dest, filepath.Base(source))})
		} else {
			waiting = append(waiting, source)
		}
	}
	for _, source := range waiting {
		base := filepath.Base(source)
		stem := strings.ToLower(strings.TrimSuffix(base, filepath.Ext(base)))
		matched := ""
		for name := range fresh {
			if strings.HasPrefix(name, stem) {
				matched = name
				break
			}
		}
		if matched != "" {
			delete(fresh, matched)
			pairs = append(pairs, Pair{source, filepath.Join(dest, actualName(dest, matched))})
			continue
		}
		if replaced && after[strings.ToLower(base)] {
			if _, err := os.Lstat(source); err != nil {
				pairs = append(pairs, Pair{source, filepath.Join(dest, base)})
			}
		}
	}
	return pairs
}

// actualName returns the name as it is spelled in the folder.
func actualName(dir, lower string) string {
	file, err := os.Open(dir)
	if err != nil {
		return lower
	}
	defer file.Close()
	list, _ := file.Readdirnames(-1)
	for _, name := range list {
		if strings.ToLower(name) == lower {
			return name
		}
	}
	return lower
}
