package upgrade

import (
	"archive/zip"
	"bytes"
	"crypto/rand"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/upgrade/upgradetest"
)

// downloaded is archive, written where an upgrade downloads it to.
func downloaded(t *testing.T, archive []byte) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "archive")
	require.NoError(t, os.WriteFile(path, archive, 0o600))

	return path
}

func TestExtractBinary(t *testing.T) {
	t.Parallel()

	t.Run("writes the binary at a gzipped tar's root", func(t *testing.T) {
		t.Parallel()

		archive := downloaded(t, upgradetest.TarGz(t, map[string]upgradetest.File{
			"README.md": {Body: []byte("# tarhche")},
			"./tarhche": {Body: []byte("binary"), Mode: 0o755},
		}))

		var binary bytes.Buffer

		require.NoError(t, extractBinary(archive, "tarhche_0.2.0_linux_amd64.tar.gz", "tarhche", &binary, maxArchive))
		assert.Equal(t, "binary", binary.String())
	})

	t.Run("writes the binary at a zip's root", func(t *testing.T) {
		t.Parallel()

		archive := downloaded(t, upgradetest.Zip(t, map[string]upgradetest.File{
			"LICENSE":     {Body: []byte("license")},
			"tarhche.exe": {Body: []byte("binary"), Mode: 0o755},
		}))

		var binary bytes.Buffer

		require.NoError(t, extractBinary(archive, "tarhche_0.2.0_windows_amd64.zip", "tarhche.exe", &binary, maxArchive))
		assert.Equal(t, "binary", binary.String())
	})

	t.Run("takes no link and no directory for the binary", func(t *testing.T) {
		t.Parallel()

		tarred := downloaded(t, upgradetest.TarGz(t, map[string]upgradetest.File{"tarhche": {Body: []byte("/bin/sh"), Mode: fs.ModeSymlink | 0o777}}))
		zipped := downloaded(t, upgradetest.Zip(t, map[string]upgradetest.File{"tarhche.exe/": {Mode: fs.ModeDir | 0o755}}))

		err := extractBinary(tarred, "tarhche_0.2.0_linux_amd64.tar.gz", "tarhche", &bytes.Buffer{}, maxArchive)
		require.ErrorIs(t, err, ErrNoBuild)
		assert.EqualError(t, err, "the release holds no build of tarhche for this platform: tarhche_0.2.0_linux_amd64.tar.gz holds no tarhche")

		err = extractBinary(zipped, "tarhche_0.2.0_windows_amd64.zip", "tarhche.exe", &bytes.Buffer{}, maxArchive)
		assert.ErrorIs(t, err, ErrNoBuild)
	})

	t.Run("refuses a binary larger than a build may be", func(t *testing.T) {
		t.Parallel()

		archive := downloaded(t, upgradetest.Zip(t, map[string]upgradetest.File{"tarhche.exe": {Body: make([]byte, 100), Mode: 0o755}}))

		err := extractBinary(archive, "tarhche_0.2.0_windows_amd64.zip", "tarhche.exe", &bytes.Buffer{}, 99)

		assert.EqualError(t, err, "extracting tarhche.exe from tarhche_0.2.0_windows_amd64.zip: it is larger than a build of tarhche may be")
	})

	t.Run("fails for what is no gzipped tar", func(t *testing.T) {
		t.Parallel()

		err := extractBinary(downloaded(t, []byte("this is no gzipped tar archive")), "tarhche_0.2.0_linux_amd64.tar.gz", "tarhche", &bytes.Buffer{}, maxArchive)

		assert.ErrorContains(t, err, "extracting tarhche from tarhche_0.2.0_linux_amd64.tar.gz: gzip")
	})

	t.Run("fails for a gzipped tar that is cut short", func(t *testing.T) {
		t.Parallel()

		archive := upgradetest.TarGz(t, map[string]upgradetest.File{"README.md": {Body: bytes.Repeat([]byte("readme "), 1000)}})

		err := extractBinary(downloaded(t, archive[:len(archive)/2]), "tarhche_0.2.0_linux_amd64.tar.gz", "tarhche", &bytes.Buffer{}, maxArchive)

		assert.ErrorContains(t, err, "extracting tarhche from tarhche_0.2.0_linux_amd64.tar.gz")
	})

	t.Run("fails for a binary that is cut short", func(t *testing.T) {
		t.Parallel()

		archive := upgradetest.TarGz(t, map[string]upgradetest.File{"tarhche": {Body: random(t, 64<<10), Mode: 0o755}})

		err := extractBinary(downloaded(t, archive[:len(archive)/2]), "tarhche_0.2.0_linux_amd64.tar.gz", "tarhche", &bytes.Buffer{}, maxArchive)

		assert.ErrorContains(t, err, "extracting tarhche from tarhche_0.2.0_linux_amd64.tar.gz: unexpected EOF")
	})

	t.Run("fails for a binary compressed in a way zip does not know", func(t *testing.T) {
		t.Parallel()

		err := extractBinary(downloaded(t, rawZip(t, "tarhche.exe", 99, []byte("binary"))), "tarhche_0.2.0_windows_amd64.zip", "tarhche.exe", &bytes.Buffer{}, maxArchive)

		assert.ErrorIs(t, err, zip.ErrAlgorithm)
	})

	t.Run("fails for what is no zip", func(t *testing.T) {
		t.Parallel()

		err := extractBinary(downloaded(t, []byte("not zip")), "tarhche_0.2.0_windows_amd64.zip", "tarhche.exe", &bytes.Buffer{}, maxArchive)

		assert.ErrorContains(t, err, "extracting tarhche.exe from tarhche_0.2.0_windows_amd64.zip: zip")
	})

	t.Run("fails for an archive that is not there", func(t *testing.T) {
		t.Parallel()

		err := extractBinary(filepath.Join(t.TempDir(), "missing"), "tarhche_0.2.0_linux_amd64.tar.gz", "tarhche", &bytes.Buffer{}, maxArchive)

		assert.ErrorIs(t, err, fs.ErrNotExist)
	})
}

func TestUnzipBundle(t *testing.T) {
	t.Parallel()

	const name = "tarhche_0.2.0_darwin_app.zip"

	unzip := func(t *testing.T, files map[string]upgradetest.File, limit int64) (string, error) {
		t.Helper()

		into := t.TempDir()

		return into, unzipBundle(downloaded(t, upgradetest.Zip(t, files)), name, into, limit)
	}

	t.Run("extracts the bundle and nothing beside it", func(t *testing.T) {
		t.Parallel()

		into, err := unzip(t, app("binary"), maxArchive)
		require.NoError(t, err)

		entries, err := os.ReadDir(into)
		require.NoError(t, err)
		require.Len(t, entries, 1)
		assert.Equal(t, appBundle, entries[0].Name())

		assert.Equal(t, "binary", read(t, filepath.Join(into, appBundle, "Contents", "MacOS", "tarhche")))

		info, err := os.Stat(filepath.Join(into, appBundle, "Contents", "MacOS", "tarhche"))
		require.NoError(t, err)
		assert.Equal(t, fs.FileMode(0o755), info.Mode().Perm())
	})

	t.Run("lets a link point below where it is, through other links", func(t *testing.T) {
		t.Parallel()

		into, err := unzip(t, map[string]upgradetest.File{
			"Tarhche.app/Contents/Frameworks/Kit.framework/Versions/A/Kit":   {Body: []byte("kit")},
			"Tarhche.app/Contents/Frameworks/Kit.framework/Versions/Current": {Body: []byte("A"), Mode: fs.ModeSymlink | 0o777},
			"Tarhche.app/Contents/Frameworks/Kit.framework/Kit":              {Body: []byte("Versions/Current/Kit"), Mode: fs.ModeSymlink | 0o777},
		}, maxArchive)
		require.NoError(t, err)

		assert.Equal(t, "kit", read(t, filepath.Join(into, appBundle, "Contents", "Frameworks", "Kit.framework", "Kit")))
	})

	escapes := map[string]map[string]upgradetest.File{
		"Tarhche.app/../evil would land out of where the bundle is extracted": {"Tarhche.app/../evil": {Body: []byte("evil")}},
		"../evil would land out of where the bundle is extracted":             {"../evil": {Body: []byte("evil")}},
		"/Tarhche.app/evil would land out of where the bundle is extracted":   {"/Tarhche.app/evil": {Body: []byte("evil")}},
		`Tarhche.app\..\evil would land out of where the bundle is extracted`: {`Tarhche.app\..\evil`: {Body: []byte("evil")}},
		"Tarhche.app/Contents/up links to what may be out of the bundle":      {"Tarhche.app/Contents/up": {Body: []byte("../.."), Mode: fs.ModeSymlink | 0o777}},
		"Tarhche.app/Contents/etc links to what may be out of the bundle":     {"Tarhche.app/Contents/etc": {Body: []byte("/etc"), Mode: fs.ModeSymlink | 0o777}},
		"Tarhche.app/Contents/nothing links to what may be out of the bundle": {"Tarhche.app/Contents/nothing": {Mode: fs.ModeSymlink | 0o777}},
		"Tarhche.app/Contents/long links to what may be out of the bundle":    {"Tarhche.app/Contents/long": {Body: bytes.Repeat([]byte("a/"), maxLink), Mode: fs.ModeSymlink | 0o777}},
		"Tarhche.app/Contents/Resources/x would be written through a link":    {"Tarhche.app/Contents/Resources": {Body: []byte("MacOS"), Mode: fs.ModeSymlink | 0o777}, "Tarhche.app/Contents/Resources/x": {Body: []byte("x")}},
		"Tarhche.app/Contents/pipe is neither a file, a directory nor a link": {"Tarhche.app/Contents/pipe": {Mode: fs.ModeNamedPipe | 0o644}},
		"Tarhche.app/Contents/Info.plist is a file and a directory":           {"Tarhche.app/Contents/Info.plist": {Body: []byte("one")}, "Tarhche.app/Contents/Info.plist/": {Mode: fs.ModeDir | 0o755}},
	}

	for message, files := range escapes {
		t.Run("refuses an archive where "+message, func(t *testing.T) {
			t.Parallel()

			into, err := unzip(t, files, maxArchive)

			require.Error(t, err)
			assert.Regexp(t, "^extracting Tarhche.app from "+name+": ", err.Error())
			assert.NoFileExists(t, filepath.Join(filepath.Dir(into), "evil"))
		})
	}

	t.Run("fails for an entry compressed in a way zip does not know", func(t *testing.T) {
		t.Parallel()

		for _, mode := range []fs.FileMode{0o644, fs.ModeSymlink | 0o777} {
			archive := rawZipOf(t, "Tarhche.app/Contents/Info.plist", 99, []byte("plist"), mode)

			err := unzipBundle(downloaded(t, archive), name, t.TempDir(), maxArchive)

			assert.ErrorIs(t, err, zip.ErrAlgorithm)
		}
	})

	t.Run("fails for an entry whose content is corrupt", func(t *testing.T) {
		t.Parallel()

		for _, mode := range []fs.FileMode{0o644, fs.ModeSymlink | 0o777} {
			archive := rawZipOf(t, "Tarhche.app/Contents/Info.plist", zip.Deflate, []byte("this is no deflated content"), mode)

			err := unzipBundle(downloaded(t, archive), name, t.TempDir(), maxArchive)

			assert.ErrorContains(t, err, "extracting Tarhche.app from "+name)
		}
	})

	t.Run("fails for what would be written under a file", func(t *testing.T) {
		t.Parallel()

		for _, under := range []upgradetest.File{{Body: []byte("x")}, {Body: []byte("x"), Mode: fs.ModeSymlink | 0o777}} {
			_, err := unzip(t, map[string]upgradetest.File{
				"Tarhche.app/Contents/Info.plist":   {Body: []byte("plist")},
				"Tarhche.app/Contents/Info.plist/x": under,
			}, maxArchive)

			assert.ErrorContains(t, err, "not a directory")
		}
	})

	t.Run("fails for what would be written under a file below a directory", func(t *testing.T) {
		t.Parallel()

		_, err := unzip(t, map[string]upgradetest.File{
			"Tarhche.app/Contents/Info.plist":     {Body: []byte("plist")},
			"Tarhche.app/Contents/Info.plist/x/y": {Body: []byte("y")},
		}, maxArchive)

		assert.ErrorContains(t, err, "not a directory")
	})

	t.Run("refuses a file written twice", func(t *testing.T) {
		t.Parallel()

		var archive bytes.Buffer

		writer := zip.NewWriter(&archive)
		for _, body := range []string{"one", "two"} {
			entry, err := writer.Create("Tarhche.app/Contents/Info.plist")
			require.NoError(t, err)

			_, err = entry.Write([]byte(body))
			require.NoError(t, err)
		}

		require.NoError(t, writer.Close())

		err := unzipBundle(downloaded(t, archive.Bytes()), name, t.TempDir(), maxArchive)

		assert.ErrorIs(t, err, fs.ErrExist)
	})

	t.Run("refuses more than a build may be", func(t *testing.T) {
		t.Parallel()

		_, err := unzip(t, map[string]upgradetest.File{
			"Tarhche.app/Contents/MacOS/tarhche":        {Body: make([]byte, 60), Mode: 0o755},
			"Tarhche.app/Contents/Resources/icons.icns": {Body: make([]byte, 60)},
		}, 100)

		assert.EqualError(t, err, "extracting Tarhche.app from "+name+": it is larger than a build of tarhche may be")
	})

	t.Run("refuses more entries than a bundle may hold", func(t *testing.T) {
		t.Parallel()

		files := map[string]upgradetest.File{}
		for i := range maxEntries + 1 {
			files[fmt.Sprintf("Tarhche.app/Contents/Resources/%d", i)] = upgradetest.File{}
		}

		_, err := unzip(t, files, maxArchive)

		assert.EqualError(t, err, fmt.Sprintf("extracting Tarhche.app from %s: it holds more than %d entries", name, maxEntries))
	})

	t.Run("says when it holds no bundle", func(t *testing.T) {
		t.Parallel()

		_, err := unzip(t, map[string]upgradetest.File{"tarhche": {Body: []byte("binary")}}, maxArchive)

		require.ErrorIs(t, err, ErrNoBuild)
		assert.EqualError(t, err, "the release holds no build of tarhche for this platform: "+name+" holds no Tarhche.app")
	})

	t.Run("fails for what is no zip", func(t *testing.T) {
		t.Parallel()

		err := unzipBundle(downloaded(t, []byte("not zip")), name, t.TempDir(), maxArchive)

		assert.ErrorContains(t, err, "extracting Tarhche.app from "+name+": zip")
	})

	t.Run("fails for an archive that is not there", func(t *testing.T) {
		t.Parallel()

		err := unzipBundle(filepath.Join(t.TempDir(), "missing"), name, t.TempDir(), maxArchive)

		assert.ErrorIs(t, err, fs.ErrNotExist)
	})
}

func TestZipReader(t *testing.T) {
	t.Parallel()

	t.Run("fails for a file that cannot be read", func(t *testing.T) {
		t.Parallel()

		file, err := os.Open(downloaded(t, upgradetest.Zip(t, app("binary"))))
		require.NoError(t, err)
		require.NoError(t, file.Close())

		_, err = zipReader(file)

		assert.ErrorIs(t, err, os.ErrClosed)
	})
}

// random is n bytes nothing compresses.
func random(t *testing.T, n int) []byte {
	t.Helper()

	body := make([]byte, n)
	_, err := rand.Read(body)
	require.NoError(t, err)

	return body
}

// rawZip is a zip archive holding a file called name, whose content is
// body, as compressed by method, and nothing else.
func rawZip(t *testing.T, name string, method uint16, body []byte) []byte {
	t.Helper()

	return rawZipOf(t, name, method, body, 0o644)
}

// rawZipOf is a zip archive holding what is called name with mode, whose
// content is body, as compressed by method, and nothing else.
func rawZipOf(t *testing.T, name string, method uint16, body []byte, mode fs.FileMode) []byte {
	t.Helper()

	var archive bytes.Buffer

	writer := zip.NewWriter(&archive)

	header := &zip.FileHeader{Name: name, Method: method, CompressedSize64: uint64(len(body)), UncompressedSize64: uint64(len(body))}
	header.SetMode(mode)

	entry, err := writer.CreateRaw(header)
	require.NoError(t, err)

	_, err = entry.Write(body)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	return archive.Bytes()
}

func TestDownward(t *testing.T) {
	t.Parallel()

	tests := map[string]bool{
		"A":                    true,
		"Versions/Current/Kit": true,
		".":                    true,
		"":                     false,
		"..":                   false,
		"../Resources":         false,
		"A/../../x":            false,
		"/etc":                 false,
		`..\x`:                 false,
	}

	for target, isDownward := range tests {
		t.Run(target, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, isDownward, downward(target))
		})
	}
}
