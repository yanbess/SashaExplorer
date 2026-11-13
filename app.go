package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/sys/windows"

	"sashaexplorer/internal/files"
	"sashaexplorer/internal/media"
	"sashaexplorer/internal/protect"
	"sashaexplorer/internal/shellmenu"
	"sashaexplorer/internal/state"
	"sashaexplorer/internal/terminals"
	"sashaexplorer/internal/thumbcache"
	"sashaexplorer/internal/undo"
	"sashaexplorer/internal/update"
	"sashaexplorer/internal/watch"
	"sashaexplorer/internal/winshell"
)

const (
	stateFileName = "sashaexplorer.state"
	// thumbCacheLimit is how much disk the thumbnails may take.
	thumbCacheLimit = 1 << 30
	// folderPictures is how many pictures a folder icon shows.
	folderPictures = 3

	eventDirChanged  = "dir:changed"
	eventOpenPaths   = "open:paths"
	eventSearchFound = "search:found"
	eventSizeFound   = "sizes:found"
	eventUndoChanged = "undo:changed"
)

// App connects the frontend with the file system and the Windows shell.
type App struct {
	ctx      context.Context
	exeDir   string
	state    *state.Store
	pool     *winshell.Pool
	ui       *winshell.UI
	watchers [2]*watch.Watcher
	icons    *imageCache
	// thumbs keeps recent thumbnails in memory, and disk keeps them between
	// runs. disk is nil when its folder cannot be used.
	thumbs   *imageCache
	disk     *thumbcache.Cache
	sizes    *sizeCache
	media    *memo[media.Info]
	pictures *memo[[]files.Picture]
	journal  *undo.Journal

	mu        sync.Mutex
	startArgs []string
	jobs      map[string]*job

	// layout is the size of the window at the start, which a reset goes back to.
	layout windowLayout
	// started is the state read at the start, which places the window.
	started state.State
	// saved is the last state the page saved, and window where the window was
	// when it last closed or the program started. The window goes into each
	// save, because the page does not know it.
	saved    state.State
	hasSaved bool
	window   *state.Window
}

// NewApp creates the application with its data file next to the executable.
func NewApp(args []string) (*App, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("locate executable: %w", err)
	}
	ui, err := winshell.NewUI()
	if err != nil {
		return nil, fmt.Errorf("start shell thread: %w", err)
	}
	dir := filepath.Dir(exe)
	a := &App{
		exeDir:    dir,
		state:     state.New(filepath.Join(dir, stateFileName), protect.DPAPI{Entropy: []byte("SashaExplorer.State.v1")}),
		pool:      winshell.NewPool(4),
		ui:        ui,
		icons:     newImageCache(600),
		thumbs:    newImageCache(400),
		media:     newMemo[media.Info](500),
		pictures:  newMemo[[]files.Picture](5000),
		startArgs: args,
		jobs:      map[string]*job{},
		sizes:     newSizeCache(),
	}
	for i := range a.watchers {
		a.watchers[i] = watch.New(func(path string) {
			a.sizes.invalidate(path)
			a.emit(eventDirChanged, path)
		})
	}
	if cacheDir, err := os.UserCacheDir(); err == nil {
		// The first cache could keep icons that stood in for thumbnails Windows
		// failed to make, so it is dropped for one that keeps thumbnails only.
		os.RemoveAll(filepath.Join(cacheDir, "SashaExplorer", "Thumbnails"))
		disk, err := thumbcache.Open(filepath.Join(cacheDir, "SashaExplorer", "Thumbnails2"),
			protect.DPAPI{Entropy: []byte("SashaExplorer.Thumbnails.v1")}, thumbCacheLimit)
		if err == nil {
			a.disk = disk
			go disk.Prune()
		}
	}
	a.journal = undo.New(func(state undo.State) { a.emit(eventUndoChanged, state) })
	a.started = a.state.Load()
	a.window = a.started.Window
	return a, nil
}

func (a *App) startup(ctx context.Context) {
	a.mu.Lock()
	a.ctx = ctx
	a.mu.Unlock()
	// The window exists but is not shown yet, so it can move without a jump.
	// Wails shows it once the page is loaded, maximized if it was.
	if saved := a.started.Window; saved != nil && a.started.KeepWindowSize {
		if hwnd := winshell.FindMainWindow(); hwnd != 0 {
			placeHidden(hwnd, *saved)
		}
	}
	runtimeTheme(ctx, a.started.Theme)
}

// startMaximized tells whether the window opens maximized.
func (a *App) startMaximized() bool {
	return a.started.KeepMaximized && a.started.Window != nil && a.started.Window.Maximized
}

// beforeClose remembers where the window is. It never stops the closing.
func (a *App) beforeClose(context.Context) bool {
	hwnd := winshell.FindMainWindow()
	if hwnd == 0 {
		return false
	}
	place, ok := readPlacement(hwnd)
	if !ok {
		return false
	}
	a.mu.Lock()
	value := a.saved
	hasSaved := a.hasSaved
	a.window = &place
	a.mu.Unlock()
	if !hasSaved {
		value = a.state.Load()
	}
	value.Window = &place
	a.state.Save(value)
	return false
}

func (a *App) domReady(context.Context) {
	a.ui.SetOwner(winshell.FindMainWindow())
	if exe, err := os.Executable(); err == nil {
		shellmenu.RememberPath(exe)
	}
}

func (a *App) shutdown(context.Context) {
	for _, watcher := range a.watchers {
		watcher.Close()
	}
}

func (a *App) emit(event string, data any) {
	a.mu.Lock()
	ctx := a.ctx
	a.mu.Unlock()
	if ctx != nil {
		runtime.EventsEmit(ctx, event, data)
	}
}

// secondInstance takes the folders of another start of the program, such as
// "SashaExplorer.exe D:\dev", and opens them as tabs here.
func (a *App) secondInstance(data options.SecondInstanceData) {
	paths := resolveArgs(data.Args, data.WorkingDirectory)
	a.mu.Lock()
	ctx := a.ctx
	a.mu.Unlock()
	if ctx == nil {
		return
	}
	runtime.WindowUnminimise(ctx)
	runtime.WindowShow(ctx)
	if len(paths) > 0 {
		runtime.EventsEmit(ctx, eventOpenPaths, paths)
	}
}

func resolveArgs(args []string, workingDir string) []string {
	var paths []string
	for _, arg := range args {
		if arg == "" || strings.HasPrefix(arg, "-") {
			continue
		}
		if !filepath.IsAbs(arg) && workingDir != "" && !(len(arg) >= 2 && arg[1] == ':') {
			arg = filepath.Join(workingDir, arg)
		}
		if path, err := files.Clean(arg); err == nil {
			paths = append(paths, path)
		}
	}
	return paths
}

// Startup is what the window needs when it opens.
type Startup struct {
	State     state.State          `json:"state"`
	Known     map[string]string    `json:"known"`
	Drives    []files.Drive        `json:"drives"`
	Paths     []string             `json:"paths"`
	Terminals []terminals.Terminal `json:"terminals"`
	Version   string               `json:"version"`
	// DefaultTerminal is the one used when none was chosen.
	DefaultTerminal string `json:"defaultTerminal"`
}

// GetStartup returns the saved state, the user folders, the drives, and the
// folders given on the command line.
func (a *App) GetStartup() Startup {
	saved := a.state.Load()
	known := files.KnownFolders()
	if !saved.PlacesReady {
		// The first start shows the desktop. Drives join on their own.
		if desktop := known["desktop"]; desktop != "" {
			saved.Places = []string{desktop}
		}
		saved.PlacesReady = true
	}
	a.mu.Lock()
	args := a.startArgs
	a.startArgs = nil
	a.mu.Unlock()
	found := terminals.Find(a.exeDir)
	return Startup{
		State:           saved,
		Known:           known,
		Drives:          files.Drives(),
		Paths:           resolveArgs(args, ""),
		Terminals:       found,
		Version:         version,
		DefaultTerminal: terminals.Default(found),
	}
}

// SaveState remembers the tabs, places, history, layout, and settings.
func (a *App) SaveState(value state.State) error {
	a.mu.Lock()
	value.Window = a.window
	a.saved = value
	a.hasSaved = true
	a.mu.Unlock()
	return a.state.Save(value)
}

// SetTheme gives the title bar of the window the theme of the page.
func (a *App) SetTheme(theme string) {
	a.mu.Lock()
	ctx := a.ctx
	a.mu.Unlock()
	if ctx != nil {
		runtimeTheme(ctx, theme)
	}
}

func runtimeTheme(ctx context.Context, theme string) {
	switch theme {
	case state.ThemeLight:
		runtime.WindowSetLightTheme(ctx)
	case state.ThemeDark:
		runtime.WindowSetDarkTheme(ctx)
	default:
		runtime.WindowSetSystemDefaultTheme(ctx)
	}
}

// ResetWindow puts the window back to its first size in the middle of the
// screen, and forgets where it was.
func (a *App) ResetWindow() {
	a.mu.Lock()
	ctx := a.ctx
	a.window = nil
	a.mu.Unlock()
	if ctx == nil {
		return
	}
	runtime.WindowUnmaximise(ctx)
	runtime.WindowSetSize(ctx, a.layout.width, a.layout.height)
	runtime.WindowCenter(ctx)
}

// Version is the version of this build, such as "0.2.0", or "dev".
func (a *App) Version() string {
	return version
}

// CheckUpdate asks GitHub for the latest release.
func (a *App) CheckUpdate() (update.Release, error) {
	return update.Latest(context.Background(), version)
}

// OpenURL opens a page of the project in the browser. Only GitHub pages of
// the project are opened.
func (a *App) OpenURL(url string) error {
	if !strings.HasPrefix(url, "https://github.com/"+update.Repository) {
		return errors.New("only pages of the project open from here")
	}
	a.mu.Lock()
	ctx := a.ctx
	a.mu.Unlock()
	if ctx != nil {
		runtime.BrowserOpenURL(ctx, url)
	}
	return nil
}

// Terminals lists the terminals found on the computer.
func (a *App) Terminals() []terminals.Terminal {
	return terminals.Find(a.exeDir)
}

// OpenTerminal opens a terminal in a folder.
func (a *App) OpenTerminal(terminal terminals.Terminal, folder string) error {
	return terminals.Open(terminal, folder, a.exeDir)
}

// PickProgram asks for a program file, for a terminal added by hand. It
// returns an empty string if the user cancels.
func (a *App) PickProgram() (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:   "Choose a terminal program",
		Filters: []runtime.FileFilter{{DisplayName: "Programs (*.exe;*.cmd;*.bat)", Pattern: "*.exe;*.cmd;*.bat"}, {DisplayName: "All files", Pattern: "*.*"}},
	})
}

// Drives lists the drives.
func (a *App) Drives() []files.Drive {
	return files.Drives()
}

// Folder is a listing with the names of its file types.
type Folder struct {
	files.Listing
	Types map[string]string `json:"types"`
}

// ListDir reads a folder.
func (a *App) ListDir(path string) (Folder, error) {
	listing, err := files.List(path)
	if err != nil {
		return Folder{}, err
	}
	seen := map[string]bool{}
	var extensions []string
	for _, entry := range listing.Entries {
		if !entry.Dir && !seen[entry.Ext] {
			seen[entry.Ext] = true
			extensions = append(extensions, entry.Ext)
		}
	}
	return Folder{Listing: listing, Types: a.pool.TypeNames(extensions)}, nil
}

// Location is where the address bar leads.
type Location struct {
	Folder string `json:"folder"`
	// Select is a file in Folder when the address named a file.
	Select string `json:"select"`
}

// Resolve checks an address typed by the user.
func (a *App) Resolve(path string) (Location, error) {
	entry, err := files.Stat(path)
	if err != nil {
		return Location{}, err
	}
	if entry.Dir {
		return Location{Folder: entry.Path}, nil
	}
	return Location{Folder: filepath.Dir(entry.Path), Select: entry.Path}, nil
}

// Stat describes one item.
func (a *App) Stat(path string) (files.Entry, error) {
	return files.Stat(path)
}

// Describe returns details for the preview panel.
func (a *App) Describe(path string) (files.Details, error) {
	return files.Describe(path)
}

// FolderSize adds up a folder for the preview panel. Another call with the same
// job ends the earlier one, and so does CancelFolderSize.
func (a *App) FolderSize(job, path string) (files.Size, error) {
	ctx, done := a.startJob("size:" + job)
	defer done()
	size, err := files.FolderSize(ctx, path)
	if errors.Is(err, context.Canceled) {
		return size, errors.New("cancelled")
	}
	if err == nil {
		a.sizes.put(path, size.Bytes)
	}
	return size, err
}

// CancelFolderSize stops adding up a folder.
func (a *App) CancelFolderSize(job string) {
	a.cancelJob("size:" + job)
}

// ReadText reads the start of a text file.
func (a *App) ReadText(path string) (files.TextPreview, error) {
	return files.ReadText(path)
}

// OpenItem opens a file with its program. A shortcut to a folder is not opened:
// its folder is returned, to be shown here.
func (a *App) OpenItem(path string) (string, error) {
	if strings.EqualFold(filepath.Ext(path), ".lnk") {
		if target, err := a.pool.LinkTarget(path); err == nil && target != "" {
			if info, err := os.Stat(target); err == nil && info.IsDir() {
				return target, nil
			}
		}
	}
	return "", a.ui.Open(path)
}

// OpenWith asks which program should open a file.
func (a *App) OpenWith(path string) error {
	return a.ui.OpenWith(path)
}

// Properties opens the Windows properties of items in one folder.
func (a *App) Properties(paths []string) error {
	return a.ui.ShowProperties(paths)
}

// NativeMenu shows the Windows context menu of items in one folder.
func (a *App) NativeMenu(paths []string, extended bool) error {
	return a.ui.ShowContextMenu(paths, extended)
}

// SetClipboard puts files on the clipboard. cut moves them when pasted.
func (a *App) SetClipboard(paths []string, cut bool) error {
	_, err := a.ui.SetClipboardFiles(paths, cut)
	return err
}

// ClipboardState tells whether there are files to paste, and which were cut, so
// the list can dim them.
type ClipboardState struct {
	HasFiles bool     `json:"hasFiles"`
	Cut      []string `json:"cut"`
}

// GetClipboard reports what can be pasted.
func (a *App) GetClipboard() ClipboardState {
	if !winshell.HasClipboardFiles() {
		return ClipboardState{}
	}
	paths, cut, _, err := a.ui.ClipboardFiles()
	if err != nil {
		return ClipboardState{HasFiles: true}
	}
	state := ClipboardState{HasFiles: len(paths) > 0}
	if cut {
		state.Cut = paths
	}
	return state
}

// Paste copies or moves the files on the clipboard into dest.
func (a *App) Paste(dest string) error {
	paths, cut, sequence, err := a.ui.ClipboardFiles()
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		return errors.New("there are no files to paste")
	}
	mode := "copy"
	if cut {
		mode = "move"
	}
	err = a.transfer(paths, dest, mode)
	if err == nil && cut {
		a.ui.ClearClipboardIf(sequence)
	}
	return err
}

// Transfer copies or moves dropped items into dest. Mode is "copy", "move", or
// "auto", which moves within one drive and copies to another, as Windows does.
func (a *App) Transfer(paths []string, dest, mode string) error {
	if mode == "auto" {
		mode = "copy"
		if len(paths) > 0 && strings.EqualFold(filepath.VolumeName(paths[0]), filepath.VolumeName(dest)) {
			mode = "move"
		}
	}
	return a.transfer(paths, dest, mode)
}

// transfer runs a copy or a move and records what arrived, so it can be undone.
// A cancelled operation may have done part of its work, and that part counts.
func (a *App) transfer(paths []string, dest, mode string) error {
	before := files.Names(dest)
	var err error
	if mode == "move" {
		err = a.ui.Move(paths, dest)
	} else {
		err = a.ui.Copy(paths, dest)
	}
	pairs := files.Arrivals(paths, dest, before, mode == "move")
	a.sizes.invalidate(dest)
	for _, path := range paths {
		a.sizes.invalidate(filepath.Dir(path))
	}
	if len(pairs) > 0 {
		if mode == "move" {
			a.journal.Add(a.moveStep(pairs))
		} else {
			a.journal.Add(a.copyStep(pairs))
		}
	}
	if errors.Is(err, winshell.ErrCancelled) {
		return nil
	}
	return err
}

// moveStep undoes a move by moving each item back under its old name.
func (a *App) moveStep(pairs []files.Pair) undo.Step {
	return undo.Step{
		Label: "move",
		Undo: func() error {
			var result error
			for _, pair := range pairs {
				result = errors.Join(result, a.moveBack(pair.To, pair.From))
			}
			return result
		},
		Redo: func() error {
			var result error
			for _, pair := range pairs {
				result = errors.Join(result, a.moveBack(pair.From, pair.To))
			}
			return result
		},
	}
}

// moveBack moves one item from one path to another. Within a drive that is a
// rename; across drives Windows moves it and shows its progress.
func (a *App) moveBack(from, to string) error {
	if _, err := os.Lstat(from); err != nil {
		return fmt.Errorf("%s is no longer there", from)
	}
	if _, err := os.Lstat(to); err == nil {
		return fmt.Errorf("%s already exists", to)
	}
	defer a.sizes.invalidate(filepath.Dir(from))
	defer a.sizes.invalidate(filepath.Dir(to))
	if strings.EqualFold(filepath.VolumeName(from), filepath.VolumeName(to)) {
		return os.Rename(from, to)
	}
	if err := a.ui.Move([]string{from}, filepath.Dir(to)); err != nil {
		return err
	}
	moved := filepath.Join(filepath.Dir(to), filepath.Base(from))
	if moved != to {
		return os.Rename(moved, to)
	}
	return nil
}

// copyStep undoes a copy by moving the copies to the Recycle Bin, and redoes it
// by bringing them back from there.
func (a *App) copyStep(pairs []files.Pair) undo.Step {
	copies := make([]string, len(pairs))
	for i, pair := range pairs {
		copies[i] = pair.To
	}
	return a.recycleStep("copy", copies, false)
}

// recycleStep is a step whose undo or redo moves items to the Recycle Bin.
// deleted tells whether the step itself deleted them, so undo brings them back.
func (a *App) recycleStep(label string, paths []string, deleted bool) undo.Step {
	since := time.Now().Add(-2 * time.Second)
	remove := func() error {
		since = time.Now().Add(-2 * time.Second)
		err := a.ui.Delete(paths, false)
		for _, path := range paths {
			a.sizes.invalidate(filepath.Dir(path))
		}
		if errors.Is(err, winshell.ErrCancelled) {
			return errors.New("cancelled")
		}
		return err
	}
	restore := func() error {
		_, err := files.Restore(paths, since)
		for _, path := range paths {
			a.sizes.invalidate(filepath.Dir(path))
		}
		return err
	}
	if deleted {
		return undo.Step{Label: label, Undo: restore, Redo: remove}
	}
	return undo.Step{Label: label, Undo: remove, Redo: restore}
}

// Delete moves items to the Recycle Bin, or deletes them for good. Only the
// first can be undone.
func (a *App) Delete(paths []string, permanent bool) error {
	err := a.ui.Delete(paths, permanent)
	for _, path := range paths {
		a.sizes.invalidate(filepath.Dir(path))
	}
	if !permanent {
		var gone []string
		for _, path := range paths {
			if _, statErr := os.Lstat(path); statErr != nil {
				gone = append(gone, path)
			}
		}
		if len(gone) > 0 {
			a.journal.Add(a.recycleStep("delete", gone, true))
		}
	}
	if errors.Is(err, winshell.ErrCancelled) {
		return nil
	}
	return err
}

// CreateFolder makes a folder and returns its path.
func (a *App) CreateFolder(parent, name string) (string, error) {
	path, err := files.CreateFolder(parent, name)
	if err == nil {
		a.sizes.invalidate(parent)
		a.journal.Add(a.recycleStep("new folder", []string{path}, false))
	}
	return path, err
}

// CreateFile makes an empty file and returns its path.
func (a *App) CreateFile(parent, name string) (string, error) {
	path, err := files.CreateFile(parent, name)
	if err == nil {
		a.journal.Add(a.recycleStep("new file", []string{path}, false))
	}
	return path, err
}

// Rename renames an item and returns its new path.
func (a *App) Rename(path, name string) (string, error) {
	renamed, err := files.Rename(path, name)
	if err == nil && renamed != path {
		a.journal.Add(undo.Step{
			Label: "rename",
			Undo: func() error {
				_, err := files.Rename(renamed, filepath.Base(path))
				return err
			},
			Redo: func() error {
				_, err := files.Rename(path, filepath.Base(renamed))
				return err
			},
		})
	}
	return renamed, err
}

// Undo reverts the last file operation and returns what it was.
func (a *App) Undo() (string, error) {
	return a.journal.Undo()
}

// Redo does the last undone file operation again.
func (a *App) Redo() (string, error) {
	return a.journal.Redo()
}

// UndoState tells what undo and redo would do.
func (a *App) UndoState() undo.State {
	return a.journal.State()
}

// Watch reports changes in the folder shown in a pane. Slot is the pane: 0 or 1.
func (a *App) Watch(slot int, path string) {
	if slot >= 0 && slot < len(a.watchers) {
		a.watchers[slot].Watch(path)
	}
}

// SearchDone ends a search.
type SearchDone struct {
	Count     int  `json:"count"`
	Truncated bool `json:"truncated"`
	Cancelled bool `json:"cancelled"`
}

// SearchFound carries results of a search as they come.
type SearchFound struct {
	Job     string        `json:"job"`
	Entries []files.Entry `json:"entries"`
}

// Search looks for names in a folder and its subfolders. Results arrive as
// "search:found" events. Another search with the same job ends this one.
func (a *App) Search(job, root, query string, hidden bool) (SearchDone, error) {
	ctx, done := a.startJob("search:" + job)
	defer done()

	const batchSize = 200
	const batchDelay = 150 * time.Millisecond
	var batch []files.Entry
	last := time.Now()
	count := 0
	flush := func() {
		if len(batch) > 0 {
			a.emit(eventSearchFound, SearchFound{Job: job, Entries: batch})
			batch = nil
		}
		last = time.Now()
	}
	truncated, err := files.Search(ctx, root, query, hidden, func(entry files.Entry) {
		batch = append(batch, entry)
		count++
		if len(batch) >= batchSize || time.Since(last) > batchDelay {
			flush()
		}
	})
	if errors.Is(err, context.Canceled) {
		return SearchDone{Count: count, Cancelled: true}, nil
	}
	flush()
	return SearchDone{Count: count, Truncated: truncated}, err
}

// CancelSearch stops a search.
func (a *App) CancelSearch(job string) {
	a.cancelJob("search:" + job)
}

// SizeFound is the total of one folder.
type SizeFound struct {
	Job   string `json:"job"`
	Path  string `json:"path"`
	Bytes int64  `json:"bytes"`
}

// FolderSizes adds up folders for the list. Each total arrives as a
// "sizes:found" event. Totals are remembered for a while; fresh ignores that.
// Another call with the same job ends this one.
func (a *App) FolderSizes(job string, paths []string, fresh bool) {
	ctx, done := a.startJob("sizes:" + job)
	defer done()

	const workers = 3
	work := make(chan string)
	var wait sync.WaitGroup
	for range workers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for path := range work {
				if bytes, ok := a.sizes.get(path, fresh); ok {
					a.emit(eventSizeFound, SizeFound{Job: job, Path: path, Bytes: bytes})
					continue
				}
				size, err := files.FolderSize(ctx, path)
				if err != nil && ctx.Err() != nil {
					continue
				}
				a.sizes.put(path, size.Bytes)
				a.emit(eventSizeFound, SizeFound{Job: job, Path: path, Bytes: size.Bytes})
			}
		}()
	}
send:
	for _, path := range paths {
		select {
		case work <- path:
		case <-ctx.Done():
			break send
		}
	}
	close(work)
	wait.Wait()
}

// CancelFolderSizes stops adding up folders for a pane.
func (a *App) CancelFolderSizes(job string) {
	a.cancelJob("sizes:" + job)
}

// StartDrag starts a Windows drag of items, so they can go to other programs. It
// returns when the drag ends.
func (a *App) StartDrag(paths []string) (int, error) {
	return a.ui.StartDrag(paths)
}

// ExplorerMenu reports whether Explorer shows "Open in SashaExplorer".
func (a *App) ExplorerMenu() bool {
	exe, err := os.Executable()
	return err == nil && shellmenu.Enabled(exe)
}

// SetExplorerMenu adds or removes "Open in SashaExplorer" in Explorer.
func (a *App) SetExplorerMenu(enabled bool) error {
	if !enabled {
		return shellmenu.Disable()
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return shellmenu.Enable(exe)
}

// FoldersDefault reports whether folders and Win+E open in SashaExplorer.
func (a *App) FoldersDefault() bool {
	exe, err := os.Executable()
	return err == nil && shellmenu.DefaultEnabled(exe)
}

// SetFoldersDefault makes SashaExplorer the program for folders and Win+E, or
// puts back what was there before.
func (a *App) SetFoldersDefault(enabled bool) error {
	if !enabled {
		return shellmenu.DisableDefault()
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return shellmenu.EnableDefault(exe)
}

type job struct {
	cancel context.CancelFunc
}

// startJob registers a cancellable job. A job with the same key ends first.
func (a *App) startJob(key string) (context.Context, func()) {
	ctx, cancel := context.WithCancel(context.Background())
	entry := &job{cancel: cancel}
	a.mu.Lock()
	if previous := a.jobs[key]; previous != nil {
		previous.cancel()
	}
	a.jobs[key] = entry
	a.mu.Unlock()
	return ctx, func() {
		cancel()
		a.mu.Lock()
		if a.jobs[key] == entry {
			delete(a.jobs, key)
		}
		a.mu.Unlock()
	}
}

func (a *App) cancelJob(key string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if entry := a.jobs[key]; entry != nil {
		entry.cancel()
	}
}

// PickFolder asks for a folder. It returns an empty string if the user cancels.
func (a *App) PickFolder() (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: "Choose a folder"})
}

// Modifiers reports the Ctrl and Shift keys, which decide whether files dropped
// from another program are copied or moved.
type Modifiers struct {
	Ctrl  bool `json:"ctrl"`
	Shift bool `json:"shift"`
}

var procGetAsyncKeyState = windows.NewLazySystemDLL("user32.dll").NewProc("GetAsyncKeyState")

// GetModifiers reads the keyboard now.
func (a *App) GetModifiers() Modifiers {
	down := func(key uintptr) bool {
		r, _, _ := procGetAsyncKeyState.Call(key)
		return r&0x8000 != 0
	}
	return Modifiers{Ctrl: down(0x11), Shift: down(0x10)}
}

// MediaInfo describes the streams and tags of a video or a song, or what a
// camera wrote into a picture. ffprobe knows more codecs and containers, so it
// goes first when it is installed; Windows answers otherwise, and always for
// pictures, because their camera details come from Windows.
func (a *App) MediaInfo(path string, picture bool) (media.Info, error) {
	info, err := os.Stat(path)
	if err != nil {
		return media.Info{}, err
	}
	key := fmt.Sprintf("%s|%d|%d|%t", strings.ToLower(path), info.Size(), info.ModTime().UnixNano(), picture)
	if cached, ok := a.media.get(key); ok {
		return cached, nil
	}
	var result media.Info
	if !picture && media.HaveProbe() {
		result, err = media.Probe(context.Background(), path)
	}
	if picture || err != nil || result.Source == "" {
		result, err = a.pool.MediaInfo(path)
	}
	if err == nil {
		a.media.put(key, result)
	}
	return result, err
}

// FolderPictures returns, for each folder, the pictures its icon shows. A folder
// without pictures gets an empty list.
func (a *App) FolderPictures(paths []string) map[string][]files.Picture {
	result := make(map[string][]files.Picture, len(paths))
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil || !info.IsDir() {
			result[path] = nil
			continue
		}
		// A folder's time changes when items come or go, which is what matters.
		key := strings.ToLower(path) + "|" + fmt.Sprint(info.ModTime().UnixNano())
		pictures, ok := a.pictures.get(key)
		if !ok {
			pictures = files.Pictures(path, folderPictures)
			a.pictures.put(key, pictures)
		}
		result[path] = pictures
	}
	return result
}

// RecycleBin tells what the Recycle Bin holds.
func (a *App) RecycleBin() (winshell.RecycleBin, error) {
	return winshell.QueryRecycleBin()
}

// EmptyRecycleBin empties the Recycle Bin. Windows asks first.
func (a *App) EmptyRecycleBin() error {
	err := a.ui.EmptyRecycleBin()
	if errors.Is(err, winshell.ErrCancelled) {
		return nil
	}
	return err
}

// OpenRecycleBin shows the Recycle Bin in Explorer, where items can be restored.
func (a *App) OpenRecycleBin() error {
	return winshell.OpenRecycleBin()
}

// ThumbnailCacheSize reports how much disk the saved thumbnails take.
func (a *App) ThumbnailCacheSize() int64 {
	if a.disk == nil {
		return 0
	}
	return a.disk.Size()
}

// ClearThumbnailCache removes the saved thumbnails.
func (a *App) ClearThumbnailCache() error {
	a.thumbs.clear()
	if a.disk == nil {
		return nil
	}
	return a.disk.Clear()
}

// ListRecycleBin lists the items in the Recycle Bins of all drives, with the
// names of their file types.
func (a *App) ListRecycleBin() Folder {
	var roots []string
	for _, drive := range files.Drives() {
		if drive.Ready && drive.Kind != "network" && drive.Kind != "cd" {
			roots = append(roots, drive.Path)
		}
	}
	entries := files.ListRecycleBin(roots)
	seen := map[string]bool{}
	var extensions []string
	for _, entry := range entries {
		if !entry.Dir && !seen[entry.Ext] {
			seen[entry.Ext] = true
			extensions = append(extensions, entry.Ext)
		}
	}
	if entries == nil {
		entries = []files.Entry{}
	}
	return Folder{Listing: files.Listing{Entries: entries}, Types: a.pool.TypeNames(extensions)}
}

// RestoreFromBin puts items of the Recycle Bin, given by their paths there, back
// where they were. Undo deletes them again.
func (a *App) RestoreFromBin(items []string) error {
	restored, err := files.RestoreRecycled(items)
	for _, path := range restored {
		a.sizes.invalidate(filepath.Dir(path))
	}
	if len(restored) > 0 {
		a.journal.Add(a.recycleStep("restore", restored, false))
	}
	return err
}

// PurgeFromBin deletes items of the Recycle Bin for good.
func (a *App) PurgeFromBin(items []string) error {
	return files.PurgeRecycled(items)
}
