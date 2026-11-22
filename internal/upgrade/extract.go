package upgrade

import (
	"archive/tar"
	"archive/zip"
	"cmp"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

// maxEntries is the most entries an app bundle's archive may hold.
const maxEntries = 10000

// maxLink is the longest the target of a link in an app bundle may be.
const maxLink = 4096

// extractBinary writes the file called binary at the root of the archive
// called name, downloaded to archive, to w: a zip when name says it is one,
// a gzipped tar otherwise. More than limit is refused.
func extractBinary(archive, name, binary string, w io.Writer, limit int64) error {
	file, err := os.Open(archive)
	if err != nil {
		return err
	}

	defer func() { _ = file.Close() }()

	if strings.HasSuffix(name, ".zip") {
		err = unzipBinary(file, binary, w, limit)
	} else {
		err = untarBinary(file, binary, w, limit)
	}

	if errors.Is(err, errNotHeld) {
		return fmt.Errorf("%w: %s holds no %s", ErrNoBuild, name, binary)
	}

	if err != nil {
		return fmt.Errorf("extracting %s from %s: %w", binary, name, err)
	}

	return nil
}

// errNotHeld is an archive that does not hold what was looked for in it.
var errNotHeld = errors.New("not in the archive")

// untarBinary writes the regular file called binary at the root of the
// gzipped tar archive to w.
func untarBinary(archive io.Reader, binary string, w io.Writer, limit int64) error {
	unzipped, err := gzip.NewReader(archive)
	if err != nil {
		return err
	}

	defer func() { _ = unzipped.Close() }()

	reader := tar.NewReader(unzipped)

	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return errNotHeld
		}

		if err != nil {
			return err
		}

		if path.Clean(header.Name) == binary && header.Typeflag == tar.TypeReg {
			return copyAtMost(w, reader, limit)
		}
	}
}

// unzipBinary writes the regular file called binary at the root of the zip
// archive to w.
func unzipBinary(archive *os.File, binary string, w io.Writer, limit int64) error {
	reader, err := zipReader(archive)
	if err != nil {
		return err
	}

	for _, file := range reader.File {
		if path.Clean(file.Name) != binary || !file.Mode().IsRegular() {
			continue
		}

		content, err := file.Open()
		if err != nil {
			return err
		}

		defer func() { _ = content.Close() }()

		return copyAtMost(w, content, limit)
	}

	return errNotHeld
}

// zipReader reads archive as a zip.
func zipReader(archive *os.File) (*zip.Reader, error) {
	info, err := archive.Stat()
	if err != nil {
		return nil, err
	}

	return zip.NewReader(archive, info.Size())
}

// copyAtMost copies r to w, refusing more than limit bytes.
func copyAtMost(w io.Writer, r io.Reader, limit int64) error {
	written, err := io.Copy(w, io.LimitReader(r, limit+1))
	if err != nil {
		return err
	}

	if written > limit {
		return errors.New("it is larger than a build of tarhche may be")
	}

	return nil
}

// unzipBundle extracts what the zip archive called name, downloaded to
// archive, holds of the app bundle into the directory into, and refuses
// what would land outside it: more than limit bytes or maxEntries entries
// in all, absolute paths, paths through .. or through a link, and links
// that point anywhere but below where they are.
func unzipBundle(archive, name, into string, limit int64) error {
	file, err := os.Open(archive)
	if err != nil {
		return err
	}

	defer func() { _ = file.Close() }()

	reader, err := zipReader(file)
	if err != nil {
		return fmt.Errorf("extracting %s from %s: %w", appBundle, name, err)
	}

	if len(reader.File) > maxEntries {
		return fmt.Errorf("extracting %s from %s: it holds more than %d entries", appBundle, name, maxEntries)
	}

	extraction := &bundleExtraction{into: into, budget: limit}

	for _, entry := range reader.File {
		if err := extraction.extract(entry); err != nil {
			return fmt.Errorf("extracting %s from %s: %w", appBundle, name, err)
		}
	}

	if !extraction.found {
		return fmt.Errorf("%w: %s holds no %s", ErrNoBuild, name, appBundle)
	}

	return nil
}

// bundleExtraction is an app bundle being extracted into a directory.
type bundleExtraction struct {
	into   string
	budget int64
	found  bool
}

// extract writes entry, when it is in the bundle, under the directory the
// bundle is extracted into.
func (e *bundleExtraction) extract(entry *zip.File) error {
	name, err := bundleEntry(entry.Name)
	if err != nil || len(name) == 0 {
		return err
	}

	if linked, err := e.throughLink(name); err != nil || linked {
		return cmp.Or(err, fmt.Errorf("%s would be written through a link", entry.Name))
	}

	e.found = true
	destination := filepath.Join(e.into, filepath.FromSlash(name))
	mode := entry.Mode()

	switch {
	case mode.IsDir():
		return os.MkdirAll(destination, 0o755)
	case mode&fs.ModeSymlink != 0:
		return e.link(entry, destination)
	case mode.IsRegular():
		return e.write(entry, destination, mode.Perm())
	default:
		return fmt.Errorf("%s is neither a file, a directory nor a link", entry.Name)
	}
}

// write writes the regular file entry to destination with perm (0644 when
// the archive says none), out of what is left of the budget.
func (e *bundleExtraction) write(entry *zip.File, destination string, perm fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}

	if perm == 0 {
		perm = 0o644
	}

	content, err := entry.Open()
	if err != nil {
		return err
	}

	defer func() { _ = content.Close() }()

	file, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}

	written, err := io.Copy(file, io.LimitReader(content, e.budget+1))
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}

	if err != nil {
		return err
	}

	e.budget -= written
	if e.budget < 0 {
		return errors.New("it is larger than a build of tarhche may be")
	}

	return nil
}

// throughLink reports whether a directory the entry called name is in is a
// link: nothing is extracted through one.
func (e *bundleExtraction) throughLink(name string) (bool, error) {
	directory := e.into

	for part := range strings.SplitSeq(path.Dir(name), "/") {
		if part == "." {
			continue
		}

		directory = filepath.Join(directory, part)

		info, err := os.Lstat(directory)
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}

		if err != nil {
			return false, err
		}

		if info.Mode()&fs.ModeSymlink != 0 {
			return true, nil
		}
	}

	return false, nil
}

// link makes destination the link entry is, refusing one that points
// anywhere but below where it is: as nothing is extracted through a link,
// what such links point at, however many of them it goes through, is in the
// bundle.
func (e *bundleExtraction) link(entry *zip.File, destination string) error {
	content, err := entry.Open()
	if err != nil {
		return err
	}

	defer func() { _ = content.Close() }()

	target, err := io.ReadAll(io.LimitReader(content, maxLink+1))
	if err != nil {
		return err
	}

	if len(target) > maxLink || !downward(string(target)) {
		return fmt.Errorf("%s links to what may be out of the bundle", entry.Name)
	}

	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}

	return os.Symlink(filepath.FromSlash(string(target)), destination)
}

// bundleEntry is name, an entry of an app bundle's archive, cleaned: empty
// for what is not in the bundle, an error for what would land outside the
// directory it is extracted into.
func bundleEntry(name string) (string, error) {
	if escapes(name) {
		return "", fmt.Errorf("%s would land out of where the bundle is extracted", name)
	}

	cleaned := path.Clean(name)
	if cleaned != appBundle && !strings.HasPrefix(cleaned, appBundle+"/") {
		return "", nil
	}

	return cleaned, nil
}

// downward reports whether target, where a link points, is below the
// directory the link is in.
func downward(target string) bool {
	return len(target) > 0 && !escapes(target)
}

// escapes reports whether the entry called name could land out of where an
// archive is extracted: an absolute path, a Windows one, or one through ..
func escapes(name string) bool {
	return path.IsAbs(name) || strings.Contains(name, `\`) || slices.Contains(strings.Split(name, "/"), "..")
}
