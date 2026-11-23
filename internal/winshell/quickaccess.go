package winshell

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	ole "github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

// quickAccessFolder is the Quick access of Explorer, called Home on Windows 11.
// Its left pane is also in the open and save dialogs of other programs.
const quickAccessFolder = "shell:::{679f85cb-0220-4080-b29b-5540cc05aab6}"

// PinnedFolders returns the folders pinned to Quick access, without the ones
// that are there only because they were used often.
func (u *UI) PinnedFolders() ([]string, error) {
	var paths []string
	var err error
	u.Do(func() { paths, err = pinnedFolders() })
	return paths, err
}

// PinFolder pins a folder to Quick access, as "Pin to Quick access" does.
func (u *UI) PinFolder(path string) error {
	var err error
	u.Do(func() {
		err = withShell(func(shell *ole.IDispatch) error {
			item, err := folderItem(shell, path)
			if err != nil {
				return err
			}
			defer item.Release()
			_, err = oleutil.CallMethod(item, "InvokeVerb", "pintohome")
			return err
		})
		if err == nil {
			err = checkPinned(path, true)
		}
	})
	return err
}

// UnpinFolder takes a folder out of Quick access. A folder that is not pinned is
// left alone.
func (u *UI) UnpinFolder(path string) error {
	var err error
	u.Do(func() {
		found := false
		err = eachQuickAccessItem(func(item *ole.IDispatch, itemPath string, pinned bool) error {
			if !pinned || !samePath(itemPath, path) {
				return nil
			}
			found = true
			_, err := oleutil.CallMethod(item, "InvokeVerb", "unpinfromhome")
			return err
		})
		if err == nil && found {
			err = checkPinned(path, false)
		}
	})
	return err
}

func pinnedFolders() ([]string, error) {
	var paths []string
	err := eachQuickAccessItem(func(_ *ole.IDispatch, path string, pinned bool) error {
		if pinned {
			paths = append(paths, path)
		}
		return nil
	})
	return paths, err
}

// checkPinned reports an error when the folder is not pinned, or still pinned.
// InvokeVerb does not say whether the command worked.
func checkPinned(path string, want bool) error {
	paths, err := pinnedFolders()
	if err != nil {
		return err
	}
	for _, pinned := range paths {
		if samePath(pinned, path) {
			if want {
				return nil
			}
			return fmt.Errorf("Windows did not unpin %s from Quick access", path)
		}
	}
	if want {
		return fmt.Errorf("Windows did not pin %s to Quick access", path)
	}
	return nil
}

// withShell runs f with the Shell.Application object. It runs on the UI thread,
// where COM is ready.
func withShell(f func(shell *ole.IDispatch) error) error {
	unknown, err := oleutil.CreateObject("Shell.Application")
	if err != nil {
		return err
	}
	defer unknown.Release()
	shell, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return err
	}
	defer shell.Release()
	return f(shell)
}

// folderItem returns the shell item of a folder.
func folderItem(shell *ole.IDispatch, path string) (*ole.IDispatch, error) {
	result, err := oleutil.CallMethod(shell, "NameSpace", path)
	if err != nil {
		return nil, err
	}
	folder := result.ToIDispatch()
	if folder == nil {
		return nil, fmt.Errorf("%s is not a folder that can be opened", path)
	}
	defer folder.Release()
	self, err := oleutil.GetProperty(folder, "Self")
	if err != nil {
		return nil, err
	}
	item := self.ToIDispatch()
	if item == nil {
		return nil, fmt.Errorf("%s is not a folder that can be opened", path)
	}
	return item, nil
}

// eachQuickAccessItem calls f for every item of Quick access with its path and
// whether it is pinned.
func eachQuickAccessItem(f func(item *ole.IDispatch, path string, pinned bool) error) error {
	return withShell(func(shell *ole.IDispatch) error {
		result, err := oleutil.CallMethod(shell, "NameSpace", quickAccessFolder)
		if err != nil {
			return err
		}
		folder := result.ToIDispatch()
		if folder == nil {
			return errors.New("Quick access is not available")
		}
		defer folder.Release()
		itemsResult, err := oleutil.CallMethod(folder, "Items")
		if err != nil {
			return err
		}
		items := itemsResult.ToIDispatch()
		if items == nil {
			return errors.New("Quick access is not available")
		}
		defer items.Release()
		// FolderItems has no _NewEnum by name, so the items go by index.
		countResult, err := oleutil.GetProperty(items, "Count")
		if err != nil {
			return err
		}
		count, _ := countResult.Value().(int32)
		for i := int32(0); i < count; i++ {
			if err := quickAccessItem(items, i, f); err != nil {
				return err
			}
		}
		return nil
	})
}

func quickAccessItem(items *ole.IDispatch, index int32, f func(item *ole.IDispatch, path string, pinned bool) error) error {
	result, err := oleutil.CallMethod(items, "Item", index)
	if err != nil {
		return nil
	}
	item := result.ToIDispatch()
	if item == nil {
		return nil
	}
	defer item.Release()
	pathResult, err := oleutil.GetProperty(item, "Path")
	if err != nil {
		return nil
	}
	path := pathResult.ToString()
	pathResult.Clear()
	pinned := false
	if value, err := oleutil.CallMethod(item, "ExtendedProperty", "System.Home.IsPinned"); err == nil {
		pinned, _ = value.Value().(bool)
		value.Clear()
	}
	return f(item, path, pinned)
}

func samePath(a, b string) bool {
	clean := func(p string) string {
		p = filepath.Clean(p)
		if len(p) > 3 {
			p = strings.TrimRight(p, `\`)
		}
		return p
	}
	return strings.EqualFold(clean(a), clean(b))
}
