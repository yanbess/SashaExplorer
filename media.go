package main

import (
	"container/list"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	"sashaexplorer/internal/winshell"
)

// The page loads pictures and media from these paths. They are served by this
// program inside the window and are not reachable from outside it.
const (
	routeThumb = "/_sasha/thumb"
	routeIcon  = "/_sasha/icon"
	routeFile  = "/_sasha/file"
)

// mediaMiddleware serves thumbnails, icons, and files for the preview. It runs
// before the page assets, so it also works with the development server.
func (a *App) mediaMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case routeThumb:
			a.serveImage(w, r, false)
		case routeIcon:
			a.serveImage(w, r, true)
		case routeFile:
			a.serveFile(w, r)
		default:
			next.ServeHTTP(w, r)
		}
	})
}

func (a *App) serveImage(w http.ResponseWriter, r *http.Request, icon bool) {
	query := r.URL.Query()
	path := query.Get("p")
	size, _ := strconv.Atoi(query.Get("s"))
	if path == "" || size <= 0 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	// An icon that is the same for a whole file type comes with a key, such as
	// ".txt", and is made once.
	key := ""
	if icon && query.Get("k") != "" {
		key = query.Get("k") + "|" + strconv.Itoa(size)
		if cached, ok := a.icons.get(key); ok {
			writeImage(w, cached)
			return
		}
	}
	// Other pictures belong to one file. They are kept by the file's size and
	// time, so a changed file gets a new one.
	stored := ""
	if key == "" {
		if info, err := os.Stat(path); err == nil {
			stored = fmt.Sprintf("%s|%d|%d|%d|%t", strings.ToLower(path), size, info.Size(), info.ModTime().UnixNano(), icon)
			if cached, ok := a.thumbs.get(stored); ok {
				writeImage(w, cached)
				return
			}
			if a.disk != nil {
				if data, contentType, ok := a.disk.Get(stored); ok {
					image := winshell.Image{Data: data, ContentType: contentType}
					a.thumbs.put(stored, image)
					writeImage(w, image)
					return
				}
			}
		}
	}
	var (
		image winshell.Image
		err   error
	)
	if icon {
		image, err = a.pool.Icon(path, size)
	} else {
		image, err = a.pool.Thumbnail(path, size)
	}
	if err != nil {
		http.Error(w, "no image", http.StatusNotFound)
		return
	}
	switch {
	case key != "":
		a.icons.put(key, image)
	case stored != "" && !image.Fallback:
		a.thumbs.put(stored, image)
		if a.disk != nil {
			a.disk.Put(stored, image.Data, image.ContentType)
		}
	}
	writeImage(w, image)
}

func writeImage(w http.ResponseWriter, image winshell.Image) {
	w.Header().Set("Content-Type", image.ContentType)
	if image.Fallback {
		// An icon in place of a thumbnail: the next look asks Windows again.
		w.Header().Set("Cache-Control", "no-store")
	} else {
		// The address has the file time in it, so a changed file gets a new address.
		w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
	}
	w.Write(image.Data)
}

// serveFile sends a file for the preview, with ranges so a video can seek. The
// sandbox keeps a file that the window would show as a page, such as SVG or HTML,
// from running scripts.
func (a *App) serveFile(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("p")
	if path == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	file, err := os.Open(path)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || info.IsDir() {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	header := w.Header()
	header.Set("Content-Security-Policy", "sandbox; default-src 'none'; img-src 'self' data:; style-src 'unsafe-inline'")
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("Cache-Control", "no-cache")
	if contentType := mediaType(path); contentType != "" {
		header.Set("Content-Type", contentType)
	}
	http.ServeContent(w, r, info.Name(), info.ModTime(), file)
}

// mediaType names the types the preview shows. The registry can map an extension
// to something odd, so the common ones are fixed here.
func mediaType(path string) string {
	dot := strings.LastIndex(path, ".")
	if dot < 0 {
		return "application/octet-stream"
	}
	switch strings.ToLower(path[dot:]) {
	case ".jpg", ".jpeg", ".jfif":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".bmp":
		return "image/bmp"
	case ".ico":
		return "image/x-icon"
	case ".svg":
		return "image/svg+xml"
	case ".avif":
		return "image/avif"
	case ".mp4", ".m4v":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	case ".mov":
		return "video/quicktime"
	case ".mkv":
		return "video/x-matroska"
	case ".ogv":
		return "video/ogg"
	case ".mp3":
		return "audio/mpeg"
	case ".m4a", ".aac":
		return "audio/mp4"
	case ".wav":
		return "audio/wav"
	case ".ogg", ".oga", ".opus":
		return "audio/ogg"
	case ".flac":
		return "audio/flac"
	case ".pdf":
		return "application/pdf"
	}
	return "application/octet-stream"
}

// imageCache keeps the most recently used icons.
type imageCache struct {
	mu    sync.Mutex
	limit int
	order *list.List
	items map[string]*list.Element
}

type cacheItem struct {
	key   string
	image winshell.Image
}

func newImageCache(limit int) *imageCache {
	return &imageCache{limit: limit, order: list.New(), items: map[string]*list.Element{}}
}

func (c *imageCache) get(key string) (winshell.Image, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	element, ok := c.items[key]
	if !ok {
		return winshell.Image{}, false
	}
	c.order.MoveToFront(element)
	return element.Value.(cacheItem).image, true
}

func (c *imageCache) put(key string, image winshell.Image) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if element, ok := c.items[key]; ok {
		element.Value = cacheItem{key, image}
		c.order.MoveToFront(element)
		return
	}
	c.items[key] = c.order.PushFront(cacheItem{key, image})
	for c.order.Len() > c.limit {
		oldest := c.order.Back()
		c.order.Remove(oldest)
		delete(c.items, oldest.Value.(cacheItem).key)
	}
}

func (c *imageCache) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.order.Init()
	c.items = map[string]*list.Element{}
}

// memo remembers a limited number of results. When it is full it starts over,
// which is simple and good enough for what it holds.
type memo[T any] struct {
	mu    sync.Mutex
	limit int
	items map[string]T
}

func newMemo[T any](limit int) *memo[T] {
	return &memo[T]{limit: limit, items: map[string]T{}}
}

func (m *memo[T]) get(key string) (T, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	value, ok := m.items[key]
	return value, ok
}

func (m *memo[T]) put(key string, value T) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.items) >= m.limit {
		m.items = map[string]T{}
	}
	m.items[key] = value
}
