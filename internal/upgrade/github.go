package upgrade

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/mod/semver"
)

// apiVersion is the version of GitHub's REST API releases are read in.
const apiVersion = "2022-11-28"

// maxRelease is the most of a release GitHub's answer may be: its notes and
// the list of its files, which are far less.
const maxRelease = 8 << 20

// githubRelease is a release as GitHub's API answers it.
type githubRelease struct {
	TagName     string        `json:"tag_name"`
	Name        string        `json:"name"`
	Body        string        `json:"body"`
	HTMLURL     string        `json:"html_url"`
	PublishedAt time.Time     `json:"published_at"`
	Assets      []githubAsset `json:"assets"`
}

// githubAsset is a file of a release as GitHub's API answers it.
type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// release is r as this package says a release.
func (r githubRelease) release() Release {
	assets := make([]Asset, 0, len(r.Assets))
	for _, asset := range r.Assets {
		assets = append(assets, Asset{Name: asset.Name, URL: asset.BrowserDownloadURL, Size: asset.Size})
	}

	return Release{
		Version:     strings.TrimPrefix(r.TagName, "v"),
		Tag:         r.TagName,
		Name:        r.Name,
		Notes:       r.Body,
		URL:         r.HTMLURL,
		PublishedAt: r.PublishedAt,
		Assets:      assets,
	}
}

// read is the release GitHub's API answers at endpoint, under the
// repository's: missing when it answers that there is none.
func (u *Upgrader) read(ctx context.Context, endpoint string, missing error) (Release, error) {
	address := strings.TrimRight(u.api, "/") + "/repos/" + u.repository + "/" + endpoint

	header := http.Header{}
	header.Set("Accept", "application/vnd.github+json")
	header.Set("X-GitHub-Api-Version", apiVersion)

	response, err := u.get(ctx, address, header)
	if err != nil {
		return Release{}, fmt.Errorf("asking GitHub for the releases of tarhche: %w", err)
	}

	defer func() { _ = response.Body.Close() }()

	if response.StatusCode == http.StatusNotFound {
		return Release{}, missing
	}

	if limit, limited := rateLimitOf(response, time.Now()); limited {
		return Release{}, limit
	}

	if response.StatusCode != http.StatusOK {
		return Release{}, fmt.Errorf("asking GitHub for the releases of tarhche: it answered %s", response.Status)
	}

	var answered githubRelease
	if err := json.NewDecoder(io.LimitReader(response.Body, maxRelease)).Decode(&answered); err != nil {
		return Release{}, fmt.Errorf("reading what GitHub answered of the releases of tarhche: %w", err)
	}

	return answered.release(), nil
}

// get asks for address with header, naming the client as its user agent.
func (u *Upgrader) get(ctx context.Context, address string, header http.Header) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}

	maps.Copy(request.Header, header)

	if len(u.userAgent) > 0 {
		request.Header.Set("User-Agent", u.userAgent)
	}

	return u.client.Do(request)
}

// rateLimit is GitHub refusing to answer before its rate limit resets.
type rateLimit struct {
	resets time.Time
}

func (e rateLimit) Error() string {
	if e.resets.IsZero() {
		return "GitHub's rate limit was reached: try again later"
	}

	return "GitHub's rate limit was reached: try again after " + e.resets.Local().Format("15:04")
}

// rateLimitOf is the rate limit response says was reached at now, if it
// says one was: a 429, or a 403 that says no request is left, or that one
// may be made again after a while.
func rateLimitOf(response *http.Response, now time.Time) (rateLimit, bool) {
	retryAfter := response.Header.Get("Retry-After")
	exhausted := response.Header.Get("X-RateLimit-Remaining") == "0" || len(retryAfter) > 0

	if response.StatusCode != http.StatusTooManyRequests && (response.StatusCode != http.StatusForbidden || !exhausted) {
		return rateLimit{}, false
	}

	if seconds, err := strconv.ParseInt(retryAfter, 10, 64); err == nil {
		return rateLimit{resets: now.Add(time.Duration(seconds) * time.Second)}, true
	}

	if seconds, err := strconv.ParseInt(response.Header.Get("X-RateLimit-Reset"), 10, 64); err == nil {
		return rateLimit{resets: time.Unix(seconds, 0)}, true
	}

	return rateLimit{}, true
}

// tagOf is the tag version was released from: v0.2.0 for 0.2.0, v0.2.0 or
// 0.2.
func tagOf(version string) (string, error) {
	tag := semver.Canonical(canonical(strings.TrimSpace(version)))
	if len(tag) == 0 {
		return "", fmt.Errorf("%q is no version of tarhche: a version is like 0.2.0", version)
	}

	return tag, nil
}

// newer reports whether version is newer than current, which every version
// is when current is no release's: a build of one's own, dev.
func newer(version, current string) bool {
	return semver.IsValid(canonical(version)) && semver.Compare(canonical(version), canonical(current)) > 0
}

// canonical is version as semver compares it: with its v.
func canonical(version string) string {
	return "v" + strings.TrimPrefix(version, "v")
}
