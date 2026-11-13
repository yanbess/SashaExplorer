package update

import "testing"

func TestNewer(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"0.2.0", "0.1.0", true},
		{"v0.10.0", "0.9.9", true},
		{"1.0.0", "1.0.0", false},
		{"0.1.0", "0.2.0", false},
		{"1.2.3", "dev", false},
		{"dev", "1.2.3", false},
		{"1.2.4-beta.1", "1.2.3", true},
		{"1.2", "1.1.0", false},
	}
	for _, c := range cases {
		if got := Newer(c.a, c.b); got != c.want {
			t.Errorf("Newer(%q, %q) = %v", c.a, c.b, got)
		}
	}
}
