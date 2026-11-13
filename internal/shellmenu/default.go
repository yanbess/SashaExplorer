package shellmenu

import (
	"errors"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// Making SashaExplorer the program for folders changes three things, all in the
// current user's part of the registry, so Windows itself is never touched:
//
//   - The default action of folders and of drives becomes "Open in SashaExplorer".
//     A folder opened from the desktop, or by a program that asks Windows to open
//     it, then opens here. Programs that ask for Explorer by name, and "Show in
//     folder" in many programs, still open Explorer.
//   - Win+E and the File Explorer shortcut start SashaExplorer.
//
// What was there before is kept under appKey and put back when it is turned off.
// Explorer's own "open" action stays, so Explorer still works from its menus.

const (
	appKey      = `Software\SashaExplorer`
	backupKey   = appKey + `\Backup`
	verbName    = "SashaExplorer.open"
	explorerKey = `Software\Classes\CLSID\{52205fd8-5dfb-447d-801a-d0b52f2e83e1}`
	launchKey   = explorerKey + `\shell\opennewwindow\command`
	// noValue stands for a default value that did not exist. The registry takes
	// no NUL in a string, so it is text no real verb would use.
	noValue = "*SashaExplorer: no value*"
)

// The shell keys whose default action changes, with how each passes the path.
var defaultPlaces = []struct {
	shell    string
	argument string
}{
	{`Software\Classes\Directory\shell`, "%1"},
	{`Software\Classes\Drive\shell`, "%1"},
}

// RememberPath records where the program is, so tools such as the VS Code
// extension can find it.
func RememberPath(exe string) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, appKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()
	return key.SetStringValue("Path", exe)
}

func readDefault(path string) string {
	key, err := registry.OpenKey(registry.CURRENT_USER, path, registry.QUERY_VALUE)
	if err != nil {
		return noValue
	}
	defer key.Close()
	value, _, err := key.GetStringValue("")
	if err != nil {
		return noValue
	}
	return value
}

func writeDefault(path, value string) error {
	if value == noValue {
		key, err := registry.OpenKey(registry.CURRENT_USER, path, registry.SET_VALUE)
		if err != nil {
			return nil
		}
		defer key.Close()
		if err := key.DeleteValue(""); err != nil && !errors.Is(err, registry.ErrNotExist) {
			return err
		}
		return nil
	}
	key, _, err := registry.CreateKey(registry.CURRENT_USER, path, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()
	return key.SetStringValue("", value)
}

func backupName(path string) string {
	return strings.ReplaceAll(path, `\`, "|")
}

// EnableDefault makes SashaExplorer the program for folders and Win+E.
func EnableDefault(exe string) error {
	backup, _, err := registry.CreateKey(registry.CURRENT_USER, backupKey, registry.SET_VALUE|registry.QUERY_VALUE)
	if err != nil {
		return err
	}
	defer backup.Close()

	for _, place := range defaultPlaces {
		name := backupName(place.shell)
		// Keep the first backup: enabling twice must not save our own value.
		if _, _, err := backup.GetStringValue(name); errors.Is(err, registry.ErrNotExist) {
			if previous := readDefault(place.shell); previous != verbName {
				if err := backup.SetStringValue(name, previous); err != nil {
					return err
				}
			}
		}
		verb, _, err := registry.CreateKey(registry.CURRENT_USER, place.shell+`\`+verbName, registry.SET_VALUE)
		if err != nil {
			return err
		}
		err = errors.Join(verb.SetStringValue("", label), verb.SetStringValue("Icon", `"`+exe+`",0`))
		verb.Close()
		if err != nil {
			return err
		}
		if err := writeDefault(place.shell+`\`+verbName+`\command`, command(exe, place.argument)); err != nil {
			return err
		}
		if err := writeDefault(place.shell, verbName); err != nil {
			return err
		}
	}

	// Win+E. Only a key this program made is ever removed again.
	if _, _, err := backup.GetIntegerValue("ExplorerKeyCreated"); errors.Is(err, registry.ErrNotExist) {
		created := uint64(1)
		if existing, err := registry.OpenKey(registry.CURRENT_USER, explorerKey, registry.QUERY_VALUE); err == nil {
			existing.Close()
			created = 0
		}
		if err := backup.SetQWordValue("ExplorerKeyCreated", created); err != nil {
			return err
		}
	}
	launch, _, err := registry.CreateKey(registry.CURRENT_USER, launchKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer launch.Close()
	// An empty DelegateExecute makes Windows run the command instead of Explorer.
	return errors.Join(launch.SetStringValue("", `"`+exe+`"`), launch.SetStringValue("DelegateExecute", ""))
}

// DisableDefault puts back what was there before EnableDefault.
func DisableDefault() error {
	var result error
	backup, err := registry.OpenKey(registry.CURRENT_USER, backupKey, registry.QUERY_VALUE)
	hasBackup := err == nil
	for _, place := range defaultPlaces {
		previous := noValue
		if hasBackup {
			if value, _, err := backup.GetStringValue(backupName(place.shell)); err == nil {
				previous = value
			}
		}
		// Another program may have taken over since; then its choice stays.
		if readDefault(place.shell) == verbName {
			result = errors.Join(result, writeDefault(place.shell, previous))
		}
		result = errors.Join(result, deleteKey(place.shell+`\`+verbName+`\command`), deleteKey(place.shell+`\`+verbName))
	}

	created := uint64(0)
	if hasBackup {
		created, _, _ = backup.GetIntegerValue("ExplorerKeyCreated")
		backup.Close()
	}
	if created == 1 {
		for _, path := range []string{launchKey, explorerKey + `\shell\opennewwindow`, explorerKey + `\shell`, explorerKey} {
			result = errors.Join(result, deleteKey(path))
		}
	} else if launch, err := registry.OpenKey(registry.CURRENT_USER, launchKey, registry.SET_VALUE); err == nil {
		launch.DeleteValue("DelegateExecute")
		launch.DeleteValue("")
		launch.Close()
	}
	if result == nil {
		result = deleteKey(backupKey)
	}
	return result
}

// DefaultEnabled reports whether folders open in SashaExplorer at exe.
func DefaultEnabled(exe string) bool {
	for _, place := range defaultPlaces {
		if readDefault(place.shell) != verbName ||
			!strings.EqualFold(readDefault(place.shell+`\`+verbName+`\command`), command(exe, place.argument)) {
			return false
		}
	}
	return true
}

func deleteKey(path string) error {
	err := registry.DeleteKey(registry.CURRENT_USER, path)
	if err != nil && !errors.Is(err, registry.ErrNotExist) {
		return err
	}
	return nil
}
