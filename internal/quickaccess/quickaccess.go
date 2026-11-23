// Package quickaccess keeps the folders of the sidebar pinned to the Quick access
// of Windows, which the open and save dialogs of other programs show too.
//
// It changes only what it did itself: a folder that was pinned already stays
// pinned when it leaves the sidebar, and a folder the user unpinned in Explorer
// is not pinned again while it stays in the sidebar.
package quickaccess

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
)

// Shell pins and unpins folders.
type Shell interface {
	// Pinned returns the pinned folders.
	Pinned() ([]string, error)
	Pin(path string) error
	Unpin(path string) error
}

// Record is what an earlier sync did.
type Record struct {
	// Handled are the sidebar folders already dealt with: pinned by the sync,
	// or pinned before it.
	Handled []string
	// Pinned are those of Handled that the sync pinned, so it unpins them when
	// they leave the sidebar.
	Pinned []string
}

// Folders returns the places that can be pinned: folders, without drives and
// repeats.
func Folders(places []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, place := range places {
		if place == "" || isDriveRoot(place) {
			continue
		}
		k := key(place)
		if !seen[k] {
			seen[k] = true
			out = append(out, place)
		}
	}
	return out
}

// Sync pins the folders of want that it has not dealt with yet, and unpins the
// folders it pinned that are no longer in want. Pass nil to take back all it
// pinned. A step that fails is tried again on the next sync.
func Sync(shell Shell, record Record, want []string) (Record, error) {
	want = Folders(want)
	wanted := keys(want)
	ours := keys(record.Pinned)
	dealt := keys(record.Handled)

	var now map[string]bool
	if len(want) > 0 || len(record.Pinned) > 0 {
		pinned, err := shell.Pinned()
		if err != nil {
			return record, err
		}
		now = keys(pinned)
	}

	var out Record
	var errs []error
	keep := func(path string, pinnedByUs bool) {
		out.Handled = append(out.Handled, path)
		if pinnedByUs {
			out.Pinned = append(out.Pinned, path)
		}
	}

	// Folders that left the sidebar give back the pins the sync made.
	gone := map[string]bool{}
	for _, path := range append(slices.Clone(record.Handled), record.Pinned...) {
		k := key(path)
		if wanted[k] || gone[k] {
			continue
		}
		gone[k] = true
		if ours[k] && now[k] {
			if err := shell.Unpin(path); err != nil {
				errs = append(errs, err)
				keep(path, true)
			}
		}
	}

	// The sidebar folders, in its order. One dealt with before is left as the
	// user keeps it, pinned or not.
	for _, path := range want {
		k := key(path)
		if dealt[k] || now[k] {
			keep(path, ours[k])
			continue
		}
		if err := shell.Pin(path); err != nil {
			errs = append(errs, err)
			continue
		}
		keep(path, true)
	}
	return out, errors.Join(errs...)
}
func keys(paths []string) map[string]bool {
	set := make(map[string]bool, len(paths))
	for _, path := range paths {
		set[key(path)] = true
	}
	return set
}

func key(path string) string {
	path = filepath.Clean(path)
	if !isDriveRoot(path) {
		path = strings.TrimRight(path, `\`)
	}
	return strings.ToLower(path)
}

// isDriveRoot reports whether path is a drive such as "C:\" or "C:".
func isDriveRoot(path string) bool {
	path = strings.TrimRight(path, `\/`)
	return len(path) == 2 && path[1] == ':'
}
