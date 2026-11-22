// Package upgradetest is a fake of GitHub's releases of tarhche: tests of
// upgrading read releases from it and download their files from it, and
// build what the releases hold with it.
package upgradetest

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"
)

// Repository is the repository the fake publishes releases of.
const Repository = "Tarhche/cli"

// Release is a release the fake has published.
type Release struct {
	// Version is its version: it is tagged v<Version>.
	Version string
	// Name is what it is titled.
	Name string
	// Notes are what it says of itself.
	Notes string
	// PublishedAt is when it was published.
	PublishedAt time.Time
	// Prerelease keeps it from being the latest.
	Prerelease bool
	// Files are the files it holds, by name.
	Files map[string][]byte
}

// Request is a request the fake was sent.
type Request struct {
	Path   string
	Header http.Header
}

// GitHub is the fake: GitHub's API, and the downloads of what its releases
// hold, on one server.
type GitHub struct {
	// URL is where it is.
	URL string

	releases []Release
	lock     sync.Mutex
	requests []Request
}

// New is a fake that has published releases, the newest first: the latest
// is the first that is no prerelease.
func New(t testing.TB, releases ...Release) *GitHub {
	t.Helper()

	github := &GitHub{releases: releases}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /repos/"+Repository+"/releases/latest", github.latest)
	mux.HandleFunc("GET /repos/"+Repository+"/releases/tags/{tag}", github.tagged)
	mux.HandleFunc("GET /"+Repository+"/releases/download/{tag}/{name}", github.download)
	mux.HandleFunc("GET /files/{tag}/{name}", github.file)

	server := httptest.NewServer(github.recording(mux))
	t.Cleanup(server.Close)

	github.URL = server.URL

	return github
}

// Requests are the requests the fake was sent, in the order it was.
func (g *GitHub) Requests() []Request {
	g.lock.Lock()
	defer g.lock.Unlock()

	return slices.Clone(g.requests)
}

// recording keeps every request next is sent.
func (g *GitHub) recording(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.lock.Lock()
		g.requests = append(g.requests, Request{Path: r.URL.Path, Header: r.Header.Clone()})
		g.lock.Unlock()

		next.ServeHTTP(w, r)
	})
}

// latest answers the latest release.
func (g *GitHub) latest(w http.ResponseWriter, _ *http.Request) {
	for _, release := range g.releases {
		if !release.Prerelease {
			g.answer(w, release)

			return
		}
	}

	notFound(w)
}

// tagged answers the release tagged as the path says.
func (g *GitHub) tagged(w http.ResponseWriter, r *http.Request) {
	release, found := g.release(r.PathValue("tag"))
	if !found {
		notFound(w)

		return
	}

	g.answer(w, release)
}

// download sends the client to where a release's file is kept, as GitHub
// does.
func (g *GitHub) download(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/files/"+r.PathValue("tag")+"/"+r.PathValue("name"), http.StatusFound)
}

// file answers a release's file.
func (g *GitHub) file(w http.ResponseWriter, r *http.Request) {
	release, found := g.release(r.PathValue("tag"))

	body, held := release.Files[r.PathValue("name")]
	if !found || !held {
		http.NotFound(w, r)

		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	_, _ = w.Write(body)
}

// release is the release tagged tag.
func (g *GitHub) release(tag string) (Release, bool) {
	for _, release := range g.releases {
		if "v"+release.Version == tag {
			return release, true
		}
	}

	return Release{}, false
}

// answer writes release as GitHub's API answers one.
func (g *GitHub) answer(w http.ResponseWriter, release Release) {
	tag := "v" + release.Version

	assets := make([]map[string]any, 0, len(release.Files))
	for _, name := range slices.Sorted(maps.Keys(release.Files)) {
		assets = append(assets, map[string]any{
			"name":                 name,
			"browser_download_url": g.URL + "/" + Repository + "/releases/download/" + tag + "/" + name,
			"size":                 len(release.Files[name]),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"tag_name":     tag,
		"name":         release.Name,
		"body":         release.Notes,
		"html_url":     g.URL + "/" + Repository + "/releases/tag/" + tag,
		"published_at": release.PublishedAt,
		"prerelease":   release.Prerelease,
		"draft":        false,
		"assets":       assets,
	})
}

// notFound answers as GitHub's API does what is not there.
func notFound(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write([]byte(`{"message":"Not Found","status":"404"}`))
}

// File is what an archive holds at a name.
type File struct {
	// Body is the file's content, or where a link points.
	Body []byte
	// Mode is its type and permissions: 0644 when it says none,
	// fs.ModeDir for a directory and fs.ModeSymlink for a link.
	Mode fs.FileMode
}

// mode is f's mode, 0644 for a file that says none.
func (f File) mode() fs.FileMode {
	if f.Mode.Perm() == 0 {
		return f.Mode | 0o644
	}

	return f.Mode
}

// TarGz is a gzipped tar archive of files, by name.
func TarGz(t testing.TB, files map[string]File) []byte {
	t.Helper()

	var archive bytes.Buffer

	zipped := gzip.NewWriter(&archive)
	writer := tar.NewWriter(zipped)

	for _, name := range slices.Sorted(maps.Keys(files)) {
		file := files[name]

		header := &tar.Header{Name: name, Mode: int64(file.mode().Perm()), Typeflag: tar.TypeReg, Size: int64(len(file.Body))}

		switch {
		case file.Mode.IsDir():
			header.Typeflag, header.Size = tar.TypeDir, 0
		case file.Mode&fs.ModeSymlink != 0:
			header.Typeflag, header.Size, header.Linkname = tar.TypeSymlink, 0, string(file.Body)
		}

		if err := writer.WriteHeader(header); err != nil {
			t.Fatal(err)
		}

		if header.Typeflag == tar.TypeReg {
			if _, err := writer.Write(file.Body); err != nil {
				t.Fatal(err)
			}
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	if err := zipped.Close(); err != nil {
		t.Fatal(err)
	}

	return archive.Bytes()
}

// Zip is a zip archive of files, by name.
func Zip(t testing.TB, files map[string]File) []byte {
	t.Helper()

	var archive bytes.Buffer

	writer := zip.NewWriter(&archive)

	for _, name := range slices.Sorted(maps.Keys(files)) {
		file := files[name]

		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		header.SetMode(file.mode())

		entry, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}

		if !file.Mode.IsDir() {
			if _, err := entry.Write(file.Body); err != nil {
				t.Fatal(err)
			}
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	return archive.Bytes()
}

// Checksummed is files, with the file of their checksums a release of
// version holds beside them.
func Checksummed(version string, files map[string][]byte) map[string][]byte {
	held := maps.Clone(files)
	held[fmt.Sprintf("tarhche_%s_checksums.txt", version)] = Checksums(files)

	return held
}

// Checksums is the file of the SHA-256 checksums of files, by name, as
// sha256sum writes it.
func Checksums(files map[string][]byte) []byte {
	var listing bytes.Buffer

	for _, name := range slices.Sorted(maps.Keys(files)) {
		sum := sha256.Sum256(files[name])
		fmt.Fprintf(&listing, "%s  %s\n", hex.EncodeToString(sum[:]), name)
	}

	return listing.Bytes()
}
