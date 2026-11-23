package state

import (
	"fmt"
	"testing"
)

func TestNormalizeMovesOldTabsIntoAPane(t *testing.T) {
	got := Normalize(State{
		Tabs: []Tab{
			{Path: `C:\a`},
			{Path: ""},
			{Path: `C:\b`, Pinned: true, View: "odd", SortKey: "odd"},
		},
		ActiveIndex: 0,
		Places:      []string{`C:\`, `c:\`, " "},
		Recent:      []string{`D:\x`, `d:\X`},
	})
	if len(got.Panes) != 1 || len(got.Tabs) != 0 {
		t.Fatalf("panes %+v tabs %+v", got.Panes, got.Tabs)
	}
	pane := got.Panes[0]
	if len(pane.Tabs) != 3 || pane.Tabs[0].Path != `C:\b` || !pane.Tabs[0].Pinned {
		t.Fatalf("tabs %+v", pane.Tabs)
	}
	// Unknown old values are dropped; folders keep their views now.
	if pane.Tabs[0].View != "" || pane.Tabs[0].SortKey != "" {
		t.Errorf("old values %+v", pane.Tabs[0])
	}
	if pane.Tabs[pane.ActiveIndex].Path != `C:\a` {
		t.Errorf("active %d", pane.ActiveIndex)
	}
	if len(got.Places) != 1 || len(got.Recent) != 1 {
		t.Errorf("places %v recent %v", got.Places, got.Recent)
	}
	if got.SidebarWidth == 0 || got.PreviewWidth == 0 || got.TileSize == 0 || got.SplitRatio != 0.5 {
		t.Errorf("sizes %+v", got)
	}
}

func TestNormalizeLimitsPanes(t *testing.T) {
	tab := Tab{Path: `C:\`, View: ViewIcons}
	got := Normalize(State{
		Panes:       []Pane{{Tabs: []Tab{tab}}, {}, {Tabs: []Tab{tab}}, {Tabs: []Tab{tab}}},
		FocusedPane: 5,
		SplitRatio:  0.99,
	})
	if len(got.Panes) != 2 || got.FocusedPane != 0 || got.SplitRatio != 0.5 {
		t.Errorf("got %+v", got)
	}
	if got.Panes[0].Tabs[0].View != ViewIcons {
		t.Errorf("icons view lost")
	}
}

func TestNormalizeFolderViews(t *testing.T) {
	views := map[string]FolderView{
		`c:\photos`: {View: ViewGallery, SortKey: "created", SortDesc: true, Used: 5},
		`c:\odd`:    {View: "odd", SortKey: "odd"},
	}
	got := Normalize(State{FolderViews: views}).FolderViews
	if got[`c:\photos`].View != ViewGallery || got[`c:\photos`].SortKey != "created" || !got[`c:\photos`].SortDesc {
		t.Errorf("photos %+v", got[`c:\photos`])
	}
	if got[`c:\odd`].View != ViewDetails || got[`c:\odd`].SortKey != "name" {
		t.Errorf("odd %+v", got[`c:\odd`])
	}
}

func TestNormalizeKeepsNewestFolderViews(t *testing.T) {
	views := map[string]FolderView{}
	for i := range MaxFolderViews + 10 {
		views[string(rune('a'+i%26))+fmt.Sprint(i)] = FolderView{View: ViewIcons, Used: int64(i)}
	}
	got := Normalize(State{FolderViews: views}).FolderViews
	if len(got) != MaxFolderViews {
		t.Fatalf("kept %d", len(got))
	}
	for _, view := range got {
		if view.Used < 10 {
			t.Fatalf("kept an old one: %+v", view)
		}
	}
}

func TestNormalizePutsOldTabsInTheGeneralGroup(t *testing.T) {
	got := Normalize(State{
		Panes:       []Pane{{Tabs: []Tab{{Path: `C:\a`}}}, {Tabs: []Tab{{Path: `D:\b`}}}},
		FocusedPane: 1,
		SplitRatio:  0.3,
		ActiveGroup: "missing",
	})
	if len(got.Groups) != 1 || got.Groups[0].ID != GeneralGroup || got.ActiveGroup != GeneralGroup {
		t.Fatalf("groups %+v active %q", got.Groups, got.ActiveGroup)
	}
	general := got.Groups[0]
	if len(general.Panes) != 2 || general.FocusedPane != 1 || general.SplitRatio != 0.3 {
		t.Errorf("general %+v", general)
	}
}

func TestNormalizeKeepsTheOrderAndDropsRepeats(t *testing.T) {
	got := Normalize(State{
		Groups: []Group{
			{ID: "work", Name: "  Work  ", Panes: []Pane{{Tabs: []Tab{{Path: `C:\w`}}}}},
			{ID: "work", Name: "Again"},
			{ID: "", Name: "No ID"},
			{ID: GeneralGroup, Name: "Mine"},
			{ID: "empty", Name: ""},
		},
		ActiveGroup: "work",
	})
	if len(got.Groups) != 3 {
		t.Fatalf("groups %+v", got.Groups)
	}
	if got.Groups[0].Name != "Work" || len(got.Groups[0].Panes) != 1 {
		t.Errorf("first %+v", got.Groups[0])
	}
	if got.Groups[1].ID != GeneralGroup || got.Groups[1].Name != "Mine" || got.Groups[2].Name != "Group" {
		t.Errorf("rest %+v", got.Groups[1:])
	}
	if got.ActiveGroup != "work" {
		t.Errorf("active %q", got.ActiveGroup)
	}
}

func TestNormalizeSidebarOrder(t *testing.T) {
	cases := []struct {
		in   []string
		want string
	}{
		{nil, "groups places recent"},
		{[]string{"recent", "odd", "places", "recent"}, "groups recent places"},
		{[]string{"recent", "groups"}, "recent places groups"},
	}
	for _, c := range cases {
		if got := fmt.Sprint(normalizeSidebarOrder(c.in)); got != "["+c.want+"]" {
			t.Errorf("%v: got %s, want [%s]", c.in, got, c.want)
		}
	}
}

func TestNormalizeSectionHeights(t *testing.T) {
	got := normalizeSectionHeights(nil, 219)
	if len(got) != 1 || got["recent"] != 219 {
		t.Errorf("old file: %v", got)
	}
	got = normalizeSectionHeights(map[string]int{"places": 300, "odd": 100, "groups": 5}, 219)
	if len(got) != 1 || got["places"] != 300 {
		t.Errorf("new file: %v", got)
	}
}
