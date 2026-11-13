// Package thumbcache keeps thumbnails on disk, so a folder seen before shows its
// pictures at once, even after the program restarts.
//
// A thumbnail shows what a picture holds, so each file is encrypted with AES-GCM.
// The key is stored next to them, protected for the current Windows user, so
// other accounts on the computer cannot read the cache.
package thumbcache

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"sashaexplorer/internal/protect"
)

const (
	keyFile = "key"
	// A file read again is marked as used at most this often, to spare the disk.
	touchEvery = time.Hour
	// The cache is trimmed after this many new files.
	pruneEvery = 500
)

// Cache is a folder of encrypted thumbnails.
type Cache struct {
	dir   string
	limit int64
	aead  cipher.AEAD
	puts  atomic.Int64
	prune sync.Mutex
}

// Open uses the folder dir, which is made if needed, and keeps it under limit
// bytes.
func Open(dir string, protector protect.Protector, limit int64) (*Cache, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	key, err := loadKey(filepath.Join(dir, keyFile), protector)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Cache{dir: dir, limit: limit, aead: aead}, nil
}

// loadKey reads the key, or makes a new one when there is none or it belongs to
// another user. Thumbnails sealed with an old key then simply fail to open.
func loadKey(path string, protector protect.Protector) ([]byte, error) {
	if key, err := protect.LoadFile(path, protector); err == nil && len(key) == 32 {
		return key, nil
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	if err := protect.SaveFile(path, protector, key); err != nil {
		return nil, err
	}
	return key, nil
}

func (c *Cache) file(key string) string {
	sum := sha256.Sum256([]byte(key))
	name := hex.EncodeToString(sum[:16])
	return filepath.Join(c.dir, name[:2], name[2:])
}

// Get returns a thumbnail and its media type.
func (c *Cache) Get(key string) ([]byte, string, bool) {
	path := c.file(key)
	sealed, err := os.ReadFile(path)
	if err != nil || len(sealed) < c.aead.NonceSize() {
		return nil, "", false
	}
	nonce, body := sealed[:c.aead.NonceSize()], sealed[c.aead.NonceSize():]
	plain, err := c.aead.Open(nil, nonce, body, []byte(key))
	if err != nil || len(plain) < 1 {
		os.Remove(path)
		return nil, "", false
	}
	if info, err := os.Stat(path); err == nil && time.Since(info.ModTime()) > touchEvery {
		now := time.Now()
		os.Chtimes(path, now, now)
	}
	return plain[1:], typeOf(plain[0]), true
}

// Put stores a thumbnail.
func (c *Cache) Put(key string, data []byte, contentType string) error {
	mark, ok := markOf(contentType)
	if !ok {
		return errors.New("thumbcache: unknown media type")
	}
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return err
	}
	plain := append([]byte{mark}, data...)
	sealed := c.aead.Seal(nonce, nonce, plain, []byte(key))

	path := c.file(key)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "tmp-*")
	if err != nil {
		return err
	}
	_, writeErr := tmp.Write(sealed)
	closeErr := tmp.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	if c.puts.Add(1)%pruneEvery == 0 {
		go c.Prune()
	}
	return nil
}

func markOf(contentType string) (byte, bool) {
	switch contentType {
	case "image/jpeg":
		return 'j', true
	case "image/png":
		return 'p', true
	}
	return 0, false
}

func typeOf(mark byte) string {
	if mark == 'j' {
		return "image/jpeg"
	}
	return "image/png"
}

type cached struct {
	path string
	size int64
	used time.Time
}

// Prune removes the least recently used thumbnails while the cache is over its
// limit, down to three quarters of it.
func (c *Cache) Prune() {
	if !c.prune.TryLock() {
		return
	}
	defer c.prune.Unlock()
	var all []cached
	var total int64
	filepath.WalkDir(c.dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || filepath.Dir(path) == c.dir {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		all = append(all, cached{path, info.Size(), info.ModTime()})
		total += info.Size()
		return nil
	})
	if total <= c.limit {
		return
	}
	sort.Slice(all, func(i, j int) bool { return all[i].used.Before(all[j].used) })
	target := c.limit * 3 / 4
	for _, item := range all {
		if total <= target {
			break
		}
		if os.Remove(item.path) == nil {
			total -= item.size
		}
	}
}

// Size adds up the files of the cache.
func (c *Cache) Size() int64 {
	var total int64
	filepath.WalkDir(c.dir, func(path string, entry fs.DirEntry, err error) error {
		if err == nil && !entry.IsDir() && filepath.Dir(path) != c.dir {
			if info, err := entry.Info(); err == nil {
				total += info.Size()
			}
		}
		return nil
	})
	return total
}

// Clear removes every thumbnail and keeps the key.
func (c *Cache) Clear() error {
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return err
	}
	var result error
	for _, entry := range entries {
		if entry.IsDir() {
			result = errors.Join(result, os.RemoveAll(filepath.Join(c.dir, entry.Name())))
		}
	}
	return result
}
