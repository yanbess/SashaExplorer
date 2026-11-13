package files

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckName(t *testing.T) {
	for _, bad := range []string{"", " ", "a/b", "a:b", "con", "Con.txt", "name.", "name ", "..", "a\x01"} {
		if CheckName(bad) == nil {
			t.Errorf("%q accepted", bad)
		}
	}
	for _, good := range []string{"a", "конспект.txt", ".gitignore", "console.log"} {
		if err := CheckName(good); err != nil {
			t.Errorf("%q: %v", good, err)
		}
	}
}

func TestClean(t *testing.T) {
	if got, _ := Clean("C:"); got != `C:\` {
		t.Errorf("drive: %q", got)
	}
	if got, _ := Clean(`"C:/Windows"`); got != `C:\Windows` {
		t.Errorf("quoted: %q", got)
	}
	got, _ := Clean(`%WINDIR%`)
	if !strings.EqualFold(got, os.Getenv("WINDIR")) {
		t.Errorf("env: %q", got)
	}
}

func TestListCreateRename(t *testing.T) {
	dir := t.TempDir()
	if _, err := CreateFolder(dir, "Sub"); err != nil {
		t.Fatal(err)
	}
	file, err := CreateFile(dir, "note.TXT")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateFile(dir, "note.TXT"); err == nil {
		t.Error("existing file replaced")
	}
	os.WriteFile(file, []byte("hello"), 0o644)

	listing, err := List(dir)
	if err != nil || len(listing.Entries) != 2 {
		t.Fatalf("list: %v %+v", err, listing)
	}
	for _, entry := range listing.Entries {
		if entry.Name == "Sub" && !entry.Dir {
			t.Error("folder not marked")
		}
		if entry.Name == "note.TXT" && (entry.Size != 5 || entry.Ext != ".txt" || entry.Created == 0) {
			t.Errorf("file entry %+v", entry)
		}
	}

	renamed, err := Rename(file, "Note.txt")
	if err != nil || filepath.Base(renamed) != "Note.txt" {
		t.Fatalf("case rename: %v %q", err, renamed)
	}
	if _, err := Rename(renamed, "Sub"); err == nil {
		t.Error("rename over a folder accepted")
	}

	size, err := FolderSize(context.Background(), dir)
	if err != nil || size.Files != 1 || size.Folders != 1 || size.Bytes != 5 {
		t.Errorf("size %+v %v", size, err)
	}
	if _, err := List(filepath.Join(dir, "missing")); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("missing folder: %v", err)
	}
}

func TestDecodeText(t *testing.T) {
	if text, ok := decodeText([]byte("привет\nworld")); !ok || text != "привет\nworld" {
		t.Errorf("utf8: %q %v", text, ok)
	}
	if _, ok := decodeText([]byte{'M', 'Z', 0, 1, 2}); ok {
		t.Error("binary accepted")
	}
	if text, ok := decodeText([]byte{0xFF, 0xFE, 'h', 0, 'i', 0}); !ok || text != "hi" {
		t.Errorf("utf16: %q", text)
	}
	cut := []byte("ab\xd0")
	if text, ok := decodeText(cut); !ok || text != "ab" {
		t.Errorf("cut rune: %q", text)
	}
}

func TestDrivesAndKnownFolders(t *testing.T) {
	drives := Drives()
	if len(drives) == 0 {
		t.Fatal("no drives")
	}
	for _, drive := range drives {
		if drive.Path == `C:\` && (drive.FileSystem == "" || drive.Total == 0 || len(drive.Serial) != 9) {
			t.Errorf("system drive %+v", drive)
		}
	}
	if KnownFolders()["desktop"] == "" {
		t.Error("no desktop")
	}
}
