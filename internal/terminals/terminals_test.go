package terminals

import (
	"strings"
	"testing"
)

func TestFindHasCommandPrompt(t *testing.T) {
	found := Find(t.TempDir())
	ids := map[string]bool{}
	for _, terminal := range found {
		if ids[terminal.ID] {
			t.Fatalf("%s is listed twice", terminal.ID)
		}
		ids[terminal.ID] = true
		if terminal.Path == "" || terminal.Name == "" || terminal.Custom {
			t.Fatalf("incomplete terminal %+v", terminal)
		}
	}
	for _, id := range []string{CommandPrompt, PowerShell} {
		if !ids[id] {
			t.Fatalf("%s was not found", id)
		}
	}
	if Default(found) != PowerShell {
		t.Fatalf("without LucyShell the default is %q", Default(found))
	}
}

func TestExpandArgs(t *testing.T) {
	got := expandArgs(`-d {folder} --title x`, `C:\My Files`)
	if got != `-d "C:\My Files" --title x` {
		t.Fatalf("got %s", got)
	}
	if got := expandArgs(`{folder}`, `C:\dev`); got != `C:\dev` {
		t.Fatalf("got %s", got)
	}
}

func TestEscapeTrailing(t *testing.T) {
	if got := escapeTrailing(`C:\`); strings.HasSuffix(got, `\"`) {
		t.Fatalf("a root ends in an escaped quote: %s", got)
	}
}

func TestOpenRefusesUnknown(t *testing.T) {
	if err := Open(Terminal{ID: "nothing"}, t.TempDir(), t.TempDir()); err != ErrNotFound {
		t.Fatalf("got %v", err)
	}
	if err := Open(Terminal{ID: PowerShell}, `Z:\no\such\folder`, t.TempDir()); err == nil {
		t.Fatal("a missing folder opened")
	}
}
