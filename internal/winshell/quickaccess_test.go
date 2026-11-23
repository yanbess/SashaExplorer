package winshell

import (
	"os"
	"slices"
	"testing"
)

// TestPinFolder pins a new folder to Quick access and unpins it. It changes the
// Quick access of the user for a moment, so it runs only when asked:
// SASHA_QUICKACCESS_TEST=1 go test ./internal/winshell -run TestPinFolder
func TestPinFolder(t *testing.T) {
	if os.Getenv("SASHA_QUICKACCESS_TEST") != "1" {
		t.Skip("set SASHA_QUICKACCESS_TEST=1 to change Quick access")
	}
	ui, err := NewUI()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	isPinned := func() bool {
		paths, err := ui.PinnedFolders()
		if err != nil {
			t.Fatal(err)
		}
		return slices.ContainsFunc(paths, func(p string) bool { return samePath(p, dir) })
	}
	if isPinned() {
		t.Fatal("a new folder is pinned already")
	}
	if err := ui.PinFolder(dir); err != nil {
		t.Fatal(err)
	}
	if !isPinned() {
		t.Fatal("not pinned")
	}
	if err := ui.UnpinFolder(dir); err != nil {
		t.Fatal(err)
	}
	if isPinned() {
		t.Fatal("still pinned")
	}
	if err := ui.UnpinFolder(dir); err != nil {
		t.Fatalf("unpin of a folder not pinned: %v", err)
	}
}
