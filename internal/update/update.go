// Package update asks GitHub for the latest release of the program. It only
// reads: downloading and installing stay with the user.
package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Repository is where releases are published.
const Repository = "yanbess/SashaExplorer"

// ReleasesURL is the page that lists every release.
const ReleasesURL = "https://github.com/" + Repository + "/releases"

const latestURL = "https://api.github.com/repos/" + Repository + "/releases/latest"

// Release is the latest published release.
type Release struct {
	// Version is the tag without its "v", such as "0.2.0".
	Version string `json:"version"`
	Name    string `json:"name"`
	// URL is the page of the release.
	URL string `json:"url"`
	// Notes are what changed, in Markdown.
	Notes     string `json:"notes"`
	Published string `json:"published"`
	// Newer is set when it is newer than the running version.
	Newer bool `json:"newer"`
}

// ErrNoRelease means nothing has been published yet.
var ErrNoRelease = errors.New("no release has been published yet")

// Latest asks GitHub for the latest release and compares it with current.
func Latest(ctx context.Context, current string) (Release, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, latestURL, nil)
	if err != nil {
		return Release{}, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "SashaExplorer/"+current)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return Release{}, fmt.Errorf("GitHub could not be reached: %w", err)
	}
	defer response.Body.Close()
	switch {
	case response.StatusCode == http.StatusNotFound:
		return Release{}, ErrNoRelease
	case response.StatusCode != http.StatusOK:
		return Release{}, fmt.Errorf("GitHub answered %s", response.Status)
	}
	var body struct {
		TagName     string `json:"tag_name"`
		Name        string `json:"name"`
		HTMLURL     string `json:"html_url"`
		Body        string `json:"body"`
		PublishedAt string `json:"published_at"`
		Draft       bool   `json:"draft"`
		Prerelease  bool   `json:"prerelease"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&body); err != nil {
		return Release{}, fmt.Errorf("read the answer of GitHub: %w", err)
	}
	if body.Draft || body.Prerelease || body.TagName == "" {
		return Release{}, ErrNoRelease
	}
	release := Release{
		Version:   strings.TrimPrefix(body.TagName, "v"),
		Name:      body.Name,
		URL:       body.HTMLURL,
		Notes:     body.Body,
		Published: body.PublishedAt,
	}
	// Only a page of this repository is opened from here.
	if !strings.HasPrefix(release.URL, ReleasesURL+"/") {
		release.URL = ReleasesURL
	}
	release.Newer = Newer(release.Version, current)
	return release, nil
}

// Newer reports whether version a is newer than b. Both are like "1.2.3",
// with or without a "v". A version that cannot be read, such as "dev", is
// never newer and nothing is newer than it.
func Newer(a, b string) bool {
	x, okA := parse(a)
	y, okB := parse(b)
	if !okA || !okB {
		return false
	}
	for i := range x {
		if x[i] != y[i] {
			return x[i] > y[i]
		}
	}
	return false
}

func parse(version string) ([3]int, bool) {
	var out [3]int
	version = strings.TrimPrefix(strings.TrimSpace(version), "v")
	// A pre-release or build suffix does not count here.
	if i := strings.IndexAny(version, "-+"); i >= 0 {
		version = version[:i]
	}
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return out, false
	}
	for i, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return out, false
		}
		out[i] = n
	}
	return out, true
}
