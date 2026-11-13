package files

import (
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"
	"unicode/utf16"
)

func TestParseRecycleInfoV2(t *testing.T) {
	path := `C:\Users\me\file.txt`
	units := utf16.Encode([]rune(path + "\x00"))
	content := make([]byte, 28+len(units)*2)
	binary.LittleEndian.PutUint64(content[0:], 2)
	binary.LittleEndian.PutUint64(content[8:], 123)
	binary.LittleEndian.PutUint64(content[16:], 133000000000000000)
	binary.LittleEndian.PutUint32(content[24:], uint32(len(units)))
	for i, u := range units {
		binary.LittleEndian.PutUint16(content[28+2*i:], u)
	}
	got, when, err := parseRecycleInfo(content)
	if err != nil || got != path || when.Year() < 2020 {
		t.Errorf("got %q %v %v", got, when, err)
	}
}

func TestArrivals(t *testing.T) {
	src, dest := t.TempDir(), t.TempDir()
	os.WriteFile(filepath.Join(dest, "a.txt"), nil, 0o644)
	before := Names(dest)
	// a.txt arrives renamed, b.txt as it is, c.txt was skipped.
	os.WriteFile(filepath.Join(dest, "a (2).txt"), nil, 0o644)
	os.WriteFile(filepath.Join(dest, "b.txt"), nil, 0o644)
	sources := []string{filepath.Join(src, "a.txt"), filepath.Join(src, "b.txt"), filepath.Join(src, "c.txt")}
	pairs := Arrivals(sources, dest, before, false)
	if len(pairs) != 2 {
		t.Fatalf("pairs %+v", pairs)
	}
	for _, pair := range pairs {
		if filepath.Base(pair.From) == "a.txt" && filepath.Base(pair.To) != "a (2).txt" {
			t.Errorf("renamed %+v", pair)
		}
	}
}

func TestSearch(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "x", "y"), 0o755)
	os.WriteFile(filepath.Join(root, "x", "y", "Photo.JPG"), nil, 0o644)
	os.WriteFile(filepath.Join(root, "x", "notes.txt"), nil, 0o644)
	var names []string
	_, err := Search(context.Background(), root, "*.jpg", false, func(e Entry) { names = append(names, e.Name) })
	if err != nil || len(names) != 1 || names[0] != "Photo.JPG" {
		t.Errorf("pattern: %v %v", names, err)
	}
	names = nil
	Search(context.Background(), root, "y", false, func(e Entry) { names = append(names, e.Name) })
	if len(names) != 1 || names[0] != "y" {
		t.Errorf("text: %v", names)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Search(ctx, root, "a", false, func(Entry) {}); err == nil {
		t.Error("cancelled search finished")
	}
	_ = time.Second
}

func TestRecycledItemRefusesOtherPaths(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "$Rabc.txt")
	os.WriteFile(path, []byte("x"), 0o644)
	if err := PurgeRecycled([]string{path, `C:\Windows`}); err == nil {
		t.Error("purged outside the Recycle Bin")
	}
	if _, err := os.Stat(path); err != nil {
		t.Error("file outside the Recycle Bin was removed")
	}
	if _, err := RestoreRecycled([]string{path}); err == nil {
		t.Error("restored from outside the Recycle Bin")
	}
}
