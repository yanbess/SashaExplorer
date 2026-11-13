package winshell

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// A picture gets its thumbnail; a file without one gets its icon, marked as a
// fallback so it is not kept.
func TestThumbnailFallback(t *testing.T) {
	dir := t.TempDir()
	picture := filepath.Join(dir, "wide.png")
	pixels := image.NewRGBA(image.Rect(0, 0, 400, 200))
	for x := range 400 {
		for y := range 200 {
			pixels.Set(x, y, color.RGBA{uint8(x), uint8(y), 128, 255})
		}
	}
	file, _ := os.Create(picture)
	png.Encode(file, pixels)
	file.Close()
	text := filepath.Join(dir, "notes.txt")
	os.WriteFile(text, []byte("hello"), 0o644)

	pool := NewPool(1)
	thumb, err := pool.Thumbnail(picture, 256)
	if err != nil || thumb.Fallback {
		t.Errorf("picture: fallback=%v err=%v", thumb.Fallback, err)
	}
	icon, err := pool.Thumbnail(text, 256)
	if err != nil || !icon.Fallback || len(icon.Data) == 0 {
		t.Errorf("text: fallback=%v err=%v", icon.Fallback, err)
	}
}
