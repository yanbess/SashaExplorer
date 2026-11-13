// Package shellmenu adds "Open in SashaExplorer" to the context menu of folders
// and drives in Windows Explorer. It writes only to the current user's part of the
// registry, so it needs no administrator rights. On Windows 11 the item is under
// "Show more options".
package shellmenu

import (
	"errors"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const (
	keyName = "SashaExplorer"
	label   = "Open in SashaExplorer"
)

// The menus it goes into, and how each passes the folder.
var places = []struct {
	key      string
	argument string
}{
	{`Software\Classes\Directory\shell\` + keyName, "%1"},
	{`Software\Classes\Directory\Background\shell\` + keyName, "%V"},
	{`Software\Classes\Drive\shell\` + keyName, "%1"},
}

func command(exe, argument string) string {
	return `"` + exe + `" "` + argument + `"`
}

// Enable adds the item, pointing at exe.
func Enable(exe string) error {
	for _, place := range places {
		key, _, err := registry.CreateKey(registry.CURRENT_USER, place.key, registry.SET_VALUE)
		if err != nil {
			return err
		}
		err = errors.Join(key.SetStringValue("", label), key.SetStringValue("Icon", `"`+exe+`",0`))
		key.Close()
		if err != nil {
			return err
		}
		commandKey, _, err := registry.CreateKey(registry.CURRENT_USER, place.key+`\command`, registry.SET_VALUE)
		if err != nil {
			return err
		}
		err = commandKey.SetStringValue("", command(exe, place.argument))
		commandKey.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// Disable removes the item.
func Disable() error {
	var result error
	for _, place := range places {
		for _, name := range []string{place.key + `\command`, place.key} {
			if err := registry.DeleteKey(registry.CURRENT_USER, name); err != nil && !errors.Is(err, registry.ErrNotExist) {
				result = errors.Join(result, err)
			}
		}
	}
	return result
}

// Enabled reports whether the item is there and points at exe. An item that
// points at another copy of the program counts as missing, so turning it on
// again fixes it.
func Enabled(exe string) bool {
	for _, place := range places {
		key, err := registry.OpenKey(registry.CURRENT_USER, place.key+`\command`, registry.QUERY_VALUE)
		if err != nil {
			return false
		}
		value, _, err := key.GetStringValue("")
		key.Close()
		if err != nil || !strings.EqualFold(value, command(exe, place.argument)) {
			return false
		}
	}
	return true
}
