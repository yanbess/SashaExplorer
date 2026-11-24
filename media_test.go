package main

import (
	"image"
	"image/color"
	"image/png"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"sashaexplorer/internal/winshell"
)

// A picture Windows cannot make a thumbnail of yet, as while it is still being
// written, gets its icon first, and its thumbnail once it can be made.
func TestThumbnailComesLater(t *testing.T) {
	saved := thumbRetryDelays
	thumbRetryDelays = []time.Duration{200 * time.Millisecond, 500 * time.Millisecond, time.Second, 2 * time.Second}
	defer func() { thumbRetryDelays = saved }()

	path := filepath.Join(t.TempDir(), "photo.png")
	if err := os.WriteFile(path, []byte("not a picture yet"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := &App{pool: winshell.NewPool(1), icons: newImageCache(10), thumbs: newImageCache(10)}
	request := httptest.NewRequest("GET", routeThumb+"?s=64&p="+url.QueryEscape(path), nil)
	response := httptest.NewRecorder()
	a.serveImage(response, request, false)
	if response.Header().Get("Cache-Control") != "no-store" && response.Code == 200 {
		t.Fatalf("a broken picture should not get a kept thumbnail")
	}

	// The file is done, and the retry in the background finds its thumbnail.
	pixels := image.NewRGBA(image.Rect(0, 0, 80, 80))
	for x := range 80 {
		for y := range 80 {
			pixels.Set(x, y, color.RGBA{200, uint8(x * 3), uint8(y * 3), 255})
		}
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	png.Encode(file, pixels)
	file.Close()

	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		info, _ := os.Stat(path)
		if image, ok := a.thumbs.get(thumbKey(path, 64, info, false)); ok {
			if image.Fallback || len(image.Data) == 0 {
				t.Fatalf("kept a fallback")
			}
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("the thumbnail did not come")
}
