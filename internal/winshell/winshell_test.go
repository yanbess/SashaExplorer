package winshell

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestTypeNames(t *testing.T) {
	pool := NewPool(1)
	names := pool.TypeNames([]string{".txt", ".txt"})
	if names[".txt"] == "" {
		t.Errorf("no type name for .txt: %v", names)
	}
}

func TestThumbnailAndIcon(t *testing.T) {
	dir := t.TempDir()
	picture := image.NewRGBA(image.Rect(0, 0, 64, 32))
	for x := range 64 {
		for y := range 32 {
			picture.Set(x, y, color.RGBA{R: 200, G: 40, B: 220, A: 255})
		}
	}
	var encoded bytes.Buffer
	png.Encode(&encoded, picture)
	path := filepath.Join(dir, "sample.png")
	if err := os.WriteFile(path, encoded.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	pool := NewPool(2)
	thumb, err := pool.Thumbnail(path, 128)
	if err != nil {
		t.Fatalf("Thumbnail: %v", err)
	}
	decoded, _, err := image.Decode(bytes.NewReader(thumb.Data))
	if err != nil {
		t.Fatalf("decode thumbnail: %v", err)
	}
	if bounds := decoded.Bounds(); bounds.Dx() != 128 && bounds.Dy() != 128 && bounds.Dx() != 64 {
		t.Errorf("unexpected size %v", bounds)
	}
	r, g, b, _ := decoded.At(decoded.Bounds().Dx()/2, decoded.Bounds().Dy()/2).RGBA()
	if r>>8 < 150 || g>>8 > 100 || b>>8 < 170 {
		t.Errorf("colors are swapped: %d %d %d", r>>8, g>>8, b>>8)
	}

	icon, err := pool.Icon(dir, 32)
	if err != nil || icon.ContentType != "image/png" || len(icon.Data) == 0 {
		t.Errorf("folder icon: %v %q", err, icon.ContentType)
	}
	if _, err := pool.Thumbnail(filepath.Join(dir, "missing.png"), 64); err == nil {
		t.Error("missing file gave a thumbnail")
	}
}
