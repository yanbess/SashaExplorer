package main

import (
	"slices"

	"sashaexplorer/internal/quickaccess"
	"sashaexplorer/internal/state"
	"sashaexplorer/internal/winshell"
)

// quickAccessShell pins through the Windows shell.
type quickAccessShell struct{ ui *winshell.UI }

func (s quickAccessShell) Pinned() ([]string, error) { return s.ui.PinnedFolders() }
func (s quickAccessShell) Pin(path string) error     { return s.ui.PinFolder(path) }
func (s quickAccessShell) Unpin(path string) error   { return s.ui.UnpinFolder(path) }

// QuickAccess reports whether the sidebar folders are pinned to the Quick access
// of Windows, which the open and save dialogs of other programs show.
func (a *App) QuickAccess() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.quick.Enabled
}

// SetQuickAccess pins the sidebar folders to Quick access, or unpins the ones
// it pinned. Folders pinned before stay as they were.
func (a *App) SetQuickAccess(enabled bool) error {
	a.mu.Lock()
	a.quick.Enabled = enabled
	if enabled {
		// Turning it on again pins also the folders unpinned in Explorer since.
		a.quick.Handled = nil
	}
	a.mu.Unlock()
	return a.syncQuickAccess()
}

// requestQuickAccessSync asks the background loop for a sync.
func (a *App) requestQuickAccessSync() {
	select {
	case a.quickWake <- struct{}{}:
	default:
	}
}

func (a *App) quickAccessLoop() {
	for range a.quickWake {
		// A failure here is tried again with the next change of the sidebar,
		// the next start, or the switch in the settings, which shows it.
		a.syncQuickAccess()
	}
}

// syncQuickAccess brings Quick access in line with the sidebar and saves what
// it did.
func (a *App) syncQuickAccess() error {
	a.quickMu.Lock()
	defer a.quickMu.Unlock()

	a.mu.Lock()
	record := quickaccess.Record{Handled: a.quick.Handled, Pinned: a.quick.Pinned}
	var want []string
	if a.quick.Enabled {
		want = slices.Clone(a.quickPlaces)
	}
	a.mu.Unlock()
	if len(want) == 0 && len(record.Handled) == 0 && len(record.Pinned) == 0 {
		a.saveQuickAccess()
		return nil
	}

	record, err := quickaccess.Sync(quickAccessShell{a.ui}, record, want)
	a.mu.Lock()
	a.quick.Handled, a.quick.Pinned = record.Handled, record.Pinned
	a.mu.Unlock()
	a.saveQuickAccess()
	return err
}

// saveQuickAccess writes the setting into the state file next to what the page
// saved last.
func (a *App) saveQuickAccess() {
	a.saveMu.Lock()
	defer a.saveMu.Unlock()
	a.mu.Lock()
	value, hasSaved := a.saved, a.hasSaved
	quick := a.quickAccessCopy()
	window := a.window
	if hasSaved {
		a.saved.QuickAccess = quick
	}
	a.mu.Unlock()
	if !hasSaved {
		value = a.state.Load()
	}
	value.QuickAccess = quick
	value.Window = window
	a.state.Save(value)
}

// quickAccessCopy returns the setting for the state file. a.mu must be held.
func (a *App) quickAccessCopy() *state.QuickAccess {
	return &state.QuickAccess{
		Enabled: a.quick.Enabled,
		Handled: slices.Clone(a.quick.Handled),
		Pinned:  slices.Clone(a.quick.Pinned),
	}
}
