package watch

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWatchReportsChangesAndSwitches(t *testing.T) {
	first, second := t.TempDir(), t.TempDir()
	changed := make(chan string, 10)
	w := New(func(path string) { changed <- path })
	defer w.Close()

	w.Watch(first)
	time.Sleep(100 * time.Millisecond)
	os.WriteFile(filepath.Join(first, "a.txt"), []byte("a"), 0o644)
	select {
	case got := <-changed:
		if got != first {
			t.Errorf("got %q", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no report")
	}

	w.Watch(second)
	time.Sleep(100 * time.Millisecond)
	os.WriteFile(filepath.Join(first, "b.txt"), []byte("b"), 0o644)
	os.WriteFile(filepath.Join(second, "c.txt"), []byte("c"), 0o644)
	deadline := time.After(3 * time.Second)
	for {
		select {
		case got := <-changed:
			if got == second {
				return
			}
		case <-deadline:
			t.Fatal("no report from the second folder")
		}
	}
}
