// Package state remembers the window content between runs: tabs, the places in
// the sidebar, the folder history, and the layout. The file is protected for the
// current Windows user, because a list of folders tells a lot about someone.
package state

import (
	"encoding/json"
	"sort"
	"strings"
	"sync"

	"sashaexplorer/internal/protect"
)

// Limits for values read from the file.
const (
	MaxTabs    = 100
	MaxPlaces  = 200
	MaxRecent  = 30
	MaxColumns = 10
	// MaxFolderViews is how many folders keep their own view and sort order.
	MaxFolderViews = 3000
)

// Views of the file list.
const (
	ViewDetails = "details"
	ViewIcons   = "icons"
	ViewGallery = "gallery"
)

// MaxPanes is how many panes the window splits into.
const MaxPanes = 2

// Sort keys.
var sortKeys = map[string]bool{"name": true, "modified": true, "created": true, "deleted": true, "type": true, "size": true}

// Tab is a remembered tab. An empty Path is the list of drives.
type Tab struct {
	Path   string `json:"path"`
	Pinned bool   `json:"pinned"`
	// View and the sort order are those of earlier versions, where each tab had
	// its own. Folders keep them now, in FolderViews.
	View     string `json:"view,omitempty"`
	SortKey  string `json:"sortKey,omitempty"`
	SortDesc bool   `json:"sortDesc,omitempty"`
}

// FolderView is how one folder is shown.
type FolderView struct {
	View     string `json:"view"`
	SortKey  string `json:"sortKey"`
	SortDesc bool   `json:"sortDesc"`
	// Used is when it was last changed, in Unix milliseconds. The oldest go
	// first when there are too many.
	Used int64 `json:"used"`
}

// Pane is one side of the window with its own tabs.
type Pane struct {
	Tabs        []Tab `json:"tabs"`
	ActiveIndex int   `json:"activeIndex"`
}

// State is the remembered content of the window.
type State struct {
	Panes       []Pane  `json:"panes"`
	FocusedPane int     `json:"focusedPane"`
	SplitRatio  float64 `json:"splitRatio"`
	// Tabs and ActiveIndex are the single pane of earlier versions. Normalize
	// moves them into Panes.
	Tabs        []Tab `json:"tabs,omitempty"`
	ActiveIndex int   `json:"activeIndex,omitempty"`
	// WindowsIcons shows the Windows icons instead of the ones of the app.
	WindowsIcons bool `json:"windowsIcons"`
	// Places is the sidebar in its order. A drive is its root, such as "C:\".
	Places []string `json:"places"`
	// RemovedDrives are drives taken out of the sidebar, so they do not come back
	// on their own like a new drive does.
	RemovedDrives []string       `json:"removedDrives"`
	PlacesReady   bool           `json:"placesReady"`
	Recent        []string       `json:"recent"`
	ShowHidden    bool           `json:"showHidden"`
	SidebarWidth  int            `json:"sidebarWidth"`
	PreviewWidth  int            `json:"previewWidth"`
	PreviewOpen   bool           `json:"previewOpen"`
	TileSize      int            `json:"tileSize"`
	Columns       map[string]int `json:"columns"`
	// FolderViews are by folder: its path in lower case without a trailing
	// backslash, or a name in brackets for views that are not folders, such as
	// "<this pc>".
	FolderViews map[string]FolderView `json:"folderViews"`
	// RecentOpen shows the list of recent folders; it starts folded.
	RecentOpen bool `json:"recentOpen"`
	// RecentHeight is the height of that list when it is shown.
	RecentHeight int `json:"recentHeight"`

	// Theme is "system", "light", or "dark".
	Theme string `json:"theme"`
	// KeepWindowSize brings the window back where it was and as large.
	KeepWindowSize bool `json:"keepWindowSize"`
	// KeepMaximized brings the window back maximized when it was closed so.
	KeepMaximized bool `json:"keepMaximized"`
	// Window is where the window was when it closed. The program writes it, not
	// the page.
	Window *Window `json:"window,omitempty"`

	// Terminals are those added by hand, next to the ones found on the computer.
	Terminals []Terminal `json:"terminals"`
	// DefaultTerminal is the ID of the terminal the button opens.
	DefaultTerminal string `json:"defaultTerminal"`
	// HiddenTerminals are IDs of terminals found on the computer that the user
	// took out of the list.
	HiddenTerminals []string `json:"hiddenTerminals"`

	// CheckUpdates asks GitHub once a day for a newer release.
	CheckUpdates bool `json:"checkUpdates"`
	// LastUpdateCheck is when that was last done, in Unix milliseconds.
	LastUpdateCheck int64 `json:"lastUpdateCheck"`
	// SkippedVersion is a release the user chose not to hear about again.
	SkippedVersion string `json:"skippedVersion"`
}

// Themes of the window.
const (
	ThemeSystem = "system"
	ThemeLight  = "light"
	ThemeDark   = "dark"
)

// Window is the place of the window, in physical pixels of the screen, as
// GetWindowPlacement gives it. The rectangle is that of the window when it is
// neither maximized nor minimized.
type Window struct {
	Left      int32 `json:"left"`
	Top       int32 `json:"top"`
	Right     int32 `json:"right"`
	Bottom    int32 `json:"bottom"`
	Maximized bool  `json:"maximized"`
}

// Terminal is a terminal added by hand. Args may hold {folder}, which becomes
// the folder to open, quoted.
type Terminal struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
	Args string `json:"args"`
}

// MaxTerminals limits the terminals added by hand.
const MaxTerminals = 30

// Store is the state file.
type Store struct {
	mu        sync.Mutex
	path      string
	protector protect.Protector
}

// New uses the file at path.
func New(path string, protector protect.Protector) *Store {
	return &Store{path: path, protector: protector}
}

// Load returns the saved state. A file that is missing, damaged, or protected for
// another user gives the defaults.
func (s *Store) Load() State {
	s.mu.Lock()
	defer s.mu.Unlock()

	loaded := Defaults()
	if plain, err := protect.LoadFile(s.path, s.protector); err == nil {
		if err := json.Unmarshal(plain, &loaded); err != nil {
			loaded = Defaults()
		}
	}
	return Normalize(loaded)
}

// Defaults is the state of a first start. Settings missing from an older file
// keep these values.
func Defaults() State {
	return State{
		PreviewOpen:    true,
		Theme:          ThemeSystem,
		KeepWindowSize: true,
		KeepMaximized:  true,
		CheckUpdates:   true,
	}
}

// Save replaces the saved state.
func (s *Store) Save(value State) error {
	plain, err := json.Marshal(Normalize(value))
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return protect.SaveFile(s.path, s.protector, plain)
}

// Normalize keeps values in range and drops what cannot be used.
func Normalize(value State) State {
	if len(value.Panes) == 0 && len(value.Tabs) > 0 {
		value.Panes = []Pane{{Tabs: value.Tabs, ActiveIndex: value.ActiveIndex}}
	}
	value.Tabs, value.ActiveIndex = nil, 0
	panes := make([]Pane, 0, MaxPanes)
	for _, pane := range value.Panes {
		pane = normalizePane(pane)
		if len(pane.Tabs) > 0 && len(panes) < MaxPanes {
			panes = append(panes, pane)
		}
	}
	value.Panes = panes
	if value.FocusedPane < 0 || value.FocusedPane >= len(panes) {
		value.FocusedPane = 0
	}
	if value.SplitRatio < 0.15 || value.SplitRatio > 0.85 {
		value.SplitRatio = 0.5
	}

	value.Places = uniquePaths(value.Places, MaxPlaces)
	value.RemovedDrives = uniquePaths(value.RemovedDrives, 26)
	value.Recent = uniquePaths(value.Recent, MaxRecent)

	value.SidebarWidth = clampOrDefault(value.SidebarWidth, 160, 480, 230)
	value.PreviewWidth = clampOrDefault(value.PreviewWidth, 200, 900, 320)
	value.TileSize = clampOrDefault(value.TileSize, 96, 320, 160)
	value.RecentHeight = clampOrDefault(value.RecentHeight, 60, 900, 220)

	columns := make(map[string]int, len(value.Columns))
	for name, width := range value.Columns {
		if len(columns) < MaxColumns && len(name) < 32 && width >= 40 && width <= 2000 {
			columns[name] = width
		}
	}
	value.Columns = columns
	value.FolderViews = normalizeFolderViews(value.FolderViews)

	if value.Theme != ThemeLight && value.Theme != ThemeDark {
		value.Theme = ThemeSystem
	}
	if w := value.Window; w != nil && (w.Right-w.Left < 100 || w.Bottom-w.Top < 100) {
		value.Window = nil
	}
	value.Terminals = normalizeTerminals(value.Terminals)
	value.HiddenTerminals = uniquePaths(value.HiddenTerminals, MaxTerminals)
	if len(value.DefaultTerminal) > 200 {
		value.DefaultTerminal = ""
	}
	if len(value.SkippedVersion) > 64 {
		value.SkippedVersion = ""
	}
	return value
}

func normalizeTerminals(list []Terminal) []Terminal {
	out := make([]Terminal, 0, len(list))
	seen := map[string]bool{}
	for _, terminal := range list {
		terminal.Name = strings.TrimSpace(terminal.Name)
		terminal.Path = strings.TrimSpace(terminal.Path)
		if terminal.ID == "" || seen[terminal.ID] || terminal.Path == "" || len(out) >= MaxTerminals ||
			len(terminal.ID) > 100 || len(terminal.Name) > 200 || len(terminal.Path) > 1024 || len(terminal.Args) > 2048 {
			continue
		}
		if terminal.Name == "" {
			terminal.Name = terminal.Path
		}
		seen[terminal.ID] = true
		out = append(out, terminal)
	}
	return out
}

func normalizeFolderViews(views map[string]FolderView) map[string]FolderView {
	type named struct {
		key  string
		view FolderView
	}
	list := make([]named, 0, len(views))
	for key, view := range views {
		if len(key) > 1024 {
			continue
		}
		if !validView(view.View) {
			view.View = ViewDetails
		}
		if !sortKeys[view.SortKey] {
			view.SortKey = "name"
		}
		list = append(list, named{key, view})
	}
	if len(list) > MaxFolderViews {
		sort.Slice(list, func(i, j int) bool { return list[i].view.Used > list[j].view.Used })
		list = list[:MaxFolderViews]
	}
	out := make(map[string]FolderView, len(list))
	for _, item := range list {
		out[item.key] = item.view
	}
	return out
}

func validView(view string) bool {
	return view == ViewDetails || view == ViewIcons || view == ViewGallery
}

func normalizePane(pane Pane) Pane {
	tabs := make([]Tab, 0, len(pane.Tabs))
	for _, tab := range pane.Tabs {
		// An empty path is the "This PC" view.
		tab.Path = strings.TrimSpace(tab.Path)
		if len(tabs) >= MaxTabs {
			continue
		}
		if !validView(tab.View) {
			tab.View = ""
		}
		if !sortKeys[tab.SortKey] {
			tab.SortKey = ""
		}
		tabs = append(tabs, tab)
	}
	// Pinned tabs stay in front, and the active tab stays active.
	var active *Tab
	if pane.ActiveIndex >= 0 && pane.ActiveIndex < len(tabs) {
		active = &tabs[pane.ActiveIndex]
	}
	activeTab := Tab{}
	if active != nil {
		activeTab = *active
	}
	ordered := make([]Tab, 0, len(tabs))
	for _, pinned := range []bool{true, false} {
		for _, tab := range tabs {
			if tab.Pinned == pinned {
				ordered = append(ordered, tab)
			}
		}
	}
	pane.Tabs = ordered
	pane.ActiveIndex = 0
	if active != nil {
		for i, tab := range ordered {
			if tab == activeTab {
				pane.ActiveIndex = i
				break
			}
		}
	}
	return pane
}

func clampOrDefault(value, low, high, fallback int) int {
	if value == 0 {
		return fallback
	}
	return min(max(value, low), high)
}

func uniquePaths(paths []string, limit int) []string {
	unique := make([]string, 0, len(paths))
	seen := map[string]bool{}
	for _, path := range paths {
		path = strings.TrimSpace(path)
		key := strings.ToLower(path)
		if path != "" && !seen[key] && len(unique) < limit {
			seen[key] = true
			unique = append(unique, path)
		}
	}
	return unique
}
