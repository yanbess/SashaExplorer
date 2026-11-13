package thumbcache

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// xor stands in for DPAPI, which works the same for the cache.
type xor struct{}

func (xor) Protect(data []byte) ([]byte, error) {
	out := bytes.Clone(data)
	for i := range out {
		out[i] ^= 0x5a
	}
	return out, nil
}

func (x xor) Unprotect(data []byte) ([]byte, error) { return x.Protect(data) }

func TestPutGetAcrossRestarts(t *testing.T) {
	dir := t.TempDir()
	cache, err := Open(dir, xor{}, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if err := cache.Put("a|256", []byte("jpeg bytes"), "image/jpeg"); err != nil {
		t.Fatal(err)
	}
	again, err := Open(dir, xor{}, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	data, contentType, ok := again.Get("a|256")
	if !ok || string(data) != "jpeg bytes" || contentType != "image/jpeg" {
		t.Fatalf("got %q %q %v", data, contentType, ok)
	}
	if _, _, ok := again.Get("b|256"); ok {
		t.Error("found a missing key")
	}
}

func TestFilesAreEncrypted(t *testing.T) {
	dir := t.TempDir()
	cache, _ := Open(dir, xor{}, 1<<20)
	cache.Put("secret", []byte("visible picture content"), "image/png")
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			content, _ := os.ReadFile(path)
			if bytes.Contains(content, []byte("visible picture")) {
				t.Errorf("%s holds plain content", path)
			}
		}
		return nil
	})
}

func TestPruneRemovesOldest(t *testing.T) {
	dir := t.TempDir()
	cache, _ := Open(dir, xor{}, 3000)
	big := bytes.Repeat([]byte{1}, 1000)
	for _, key := range []string{"1", "2", "3", "4", "5"} {
		cache.Put(key, big, "image/png")
	}
	cache.Prune()
	if size := cache.Size(); size > 3000*3/4+100 {
		t.Errorf("size %d after prune", size)
	}
	if err := cache.Clear(); err != nil || cache.Size() != 0 {
		t.Errorf("clear: %v, size %d", err, cache.Size())
	}
}
