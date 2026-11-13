package shellmenu

import (
	"testing"

	"golang.org/x/sys/windows/registry"
)

// snapshot reads every value this package may change.
func snapshot() map[string]string {
	values := map[string]string{}
	for _, place := range defaultPlaces {
		values[place.shell] = readDefault(place.shell)
		values[place.shell+"/verb"] = readDefault(place.shell + `\` + verbName + `\command`)
	}
	values["launch"] = readDefault(launchKey)
	if key, err := registry.OpenKey(registry.CURRENT_USER, explorerKey, registry.QUERY_VALUE); err == nil {
		key.Close()
		values["explorer"] = "exists"
	}
	if key, err := registry.OpenKey(registry.CURRENT_USER, backupKey, registry.QUERY_VALUE); err == nil {
		key.Close()
		values["backup"] = "exists"
	}
	return values
}

// Changes the real registry of the current user for a moment and checks that
// turning it off leaves everything exactly as it was.
func TestEnableDisableRestoresTheRegistry(t *testing.T) {
	exe := `C:\nowhere\SashaExplorer.exe`
	if DefaultEnabled(exe) {
		t.Skip("already enabled for the test path")
	}
	before := snapshot()
	if before["backup"] != "" {
		t.Skip("SashaExplorer is the folder program on this machine; not touching it")
	}
	if err := EnableDefault(exe); err != nil {
		DisableDefault()
		t.Fatalf("enable: %v", err)
	}
	if !DefaultEnabled(exe) {
		t.Error("not enabled")
	}
	if got := readDefault(launchKey); got != `"`+exe+`"` {
		t.Errorf("Win+E command %q", got)
	}
	// A second enable must not save our own values as the old ones.
	if err := EnableDefault(exe); err != nil {
		t.Errorf("enable again: %v", err)
	}
	if err := DisableDefault(); err != nil {
		t.Fatalf("disable: %v", err)
	}
	after := snapshot()
	for key, value := range before {
		if after[key] != value {
			t.Errorf("%s: %q, was %q", key, after[key], value)
		}
	}
	for key, value := range after {
		if before[key] != value {
			t.Errorf("%s: left %q", key, value)
		}
	}
}
