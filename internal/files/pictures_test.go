package files

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPictures(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"b.JPG", "a.png", "clip.mp4", "notes.txt", ".hidden.jpg", "c.webp", "d.gif"} {
		os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644)
	}
	os.Mkdir(filepath.Join(dir, "sub.jpg"), 0o755)
	got := Pictures(dir, 3)
	var names []string
	for _, picture := range got {
		names = append(names, filepath.Base(picture.Path))
	}
	if len(names) != 3 || names[0] != "a.png" || names[1] != "b.JPG" || names[2] != "c.webp" {
		t.Errorf("got %v", names)
	}
	// Videos fill in when there are too few pictures.
	only := t.TempDir()
	os.WriteFile(filepath.Join(only, "one.jpg"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(only, "clip.mp4"), []byte("x"), 0o644)
	if got := Pictures(only, 3); len(got) != 2 || filepath.Base(got[1].Path) != "clip.mp4" {
		t.Errorf("videos %v", got)
	}
}
