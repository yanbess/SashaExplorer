package winshell

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"sashaexplorer/internal/files"
)

// needsRecycleBin skips a test on GitHub Actions, whose runners have no Recycle
// Bin that a deleted file can be found in and brought back from.
func needsRecycleBin(t *testing.T) {
	t.Helper()
	if os.Getenv("GITHUB_ACTIONS") == "true" {
		t.Skip("needs the Recycle Bin of a signed-in user")
	}
}

// Deletes a temporary file to the real Recycle Bin and brings it back.
func TestDeleteAndRestore(t *testing.T) {
	needsRecycleBin(t)
	ui, err := sharedUI()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "sasha-restore-test.txt")
	os.WriteFile(path, []byte("keep me"), 0o644)
	since := time.Now().Add(-2 * time.Second)
	if err := ui.Delete([]string{path}, false); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := os.Stat(path); err == nil {
		t.Fatal("still there")
	}
	restored, err := files.Restore([]string{path}, since)
	if err != nil || len(restored) != 1 {
		t.Fatalf("restore: %v %v", restored, err)
	}
	if content, _ := os.ReadFile(path); string(content) != "keep me" {
		t.Errorf("content %q", content)
	}
}

// Reads the real Recycle Bin, which also checks the layout of SHQUERYRBINFO:
// a wrong one fails the call or gives nonsense.
func TestQueryRecycleBin(t *testing.T) {
	bin, err := QueryRecycleBin()
	if err != nil {
		t.Fatal(err)
	}
	if bin.Items < 0 || bin.Bytes < 0 || (bin.Items == 0 && bin.Bytes != 0) {
		t.Errorf("odd answer %+v", bin)
	}
	t.Logf("%+v", bin)
}

// Lists a deleted file in the Recycle Bin, restores it, deletes it again, and
// removes it for good.
func TestRecycleBinView(t *testing.T) {
	needsRecycleBin(t)
	ui, err := sharedUI()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "sasha-bin-view-test.txt")
	os.WriteFile(path, []byte("hello bin"), 0o644)
	find := func() *files.Entry {
		for _, entry := range files.ListRecycleBin([]string{filepath.VolumeName(dir) + `\`}) {
			if strings.EqualFold(entry.Original, path) {
				return &entry
			}
		}
		return nil
	}
	if err := ui.Delete([]string{path}, false); err != nil {
		t.Fatal(err)
	}
	item := find()
	if item == nil {
		t.Fatal("not listed")
	}
	if item.Name != "sasha-bin-view-test.txt" || item.Ext != ".txt" || item.Size != 9 || item.Deleted == 0 {
		t.Errorf("entry %+v", item)
	}
	if restored, err := files.RestoreRecycled([]string{item.Path}); err != nil || len(restored) != 1 {
		t.Fatalf("restore %v %v", restored, err)
	}
	if content, _ := os.ReadFile(path); string(content) != "hello bin" {
		t.Errorf("content %q", content)
	}
	if err := ui.Delete([]string{path}, false); err != nil {
		t.Fatal(err)
	}
	item = find()
	if item == nil {
		t.Fatal("not listed again")
	}
	if err := files.PurgeRecycled([]string{item.Path}); err != nil {
		t.Fatal(err)
	}
	if find() != nil {
		t.Error("still listed after purge")
	}
}

// sharedUI starts one UI thread for all tests: its window class can be
// registered only once in a process.
var sharedUI = sync.OnceValues(NewUI)
