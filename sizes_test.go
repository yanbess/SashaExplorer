package main

import "testing"

func TestSizeCacheInvalidatesParents(t *testing.T) {
	c := newSizeCache()
	c.put(`C:\a`, 1)
	c.put(`C:\a\b`, 2)
	c.put(`C:\a\bc`, 3)
	c.put(`C:\z`, 4)
	c.invalidate(`C:\A\b\deep`)
	for path, want := range map[string]bool{`C:\a`: false, `C:\a\b`: false, `C:\a\bc`: true, `C:\z`: true} {
		if _, ok := c.get(path, false); ok != want {
			t.Errorf("%s cached=%v", path, ok)
		}
	}
	if _, ok := c.get(`C:\z`, true); ok {
		t.Error("fresh used the cache")
	}
}
