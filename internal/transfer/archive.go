package transfer

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// pack writes a tar archive of the local path to w, its root named root,
// and its entries owned by root (uid and gid 0) unless owners keeps their
// own. A symlink is archived as it is; sockets are left out.
func pack(w io.Writer, local, root string, owners bool) error {
	archive := tar.NewWriter(w)
	top := filepath.Clean(local)

	err := filepath.WalkDir(top, func(p string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relative := strings.TrimLeft(strings.TrimPrefix(p, top), string(filepath.Separator))

		return packEntry(archive, p, entryName(root, filepath.ToSlash(relative)), owners)
	})
	if err != nil {
		return err
	}

	return archive.Close()
}

// entryName is what the entry at relative under an archive's root is
// called.
func entryName(root, relative string) string {
	if len(relative) == 0 {
		return root
	}

	return root + "/" + relative
}

// packEntry writes the entry for the local p, called name, and its content.
func packEntry(archive *tar.Writer, p, name string, owners bool) error {
	info, err := os.Lstat(p)
	if err != nil {
		return err
	}

	if info.Mode()&fs.ModeSocket != 0 {
		return nil
	}

	var link string

	if info.Mode()&fs.ModeSymlink != 0 {
		if link, err = os.Readlink(p); err != nil {
			return err
		}
	}

	header, err := tar.FileInfoHeader(info, link)
	if err != nil {
		return err
	}

	header.Name = name
	if info.IsDir() {
		header.Name += "/"
	}

	if !owners {
		header.Uid, header.Gid, header.Uname, header.Gname = 0, 0, "", ""
	}

	if err := archive.WriteHeader(header); err != nil {
		return err
	}

	if !info.Mode().IsRegular() {
		return nil
	}

	file, err := os.Open(p)
	if err != nil {
		return err
	}

	_, copyErr := io.Copy(archive, file)

	return errors.Join(copyErr, file.Close())
}

// unpack reads a tar archive of what was asked for as name, and unpacks it
// on this machine where a copy to dst lands.
func unpack(r io.Reader, name string, dst destination) error {
	archive := tar.NewReader(r)

	first, err := archive.Next()
	if errors.Is(err, io.EOF) {
		return errors.New("the archive is empty")
	}

	if err != nil {
		return fmt.Errorf("reading the archive: %w", err)
	}

	from := archiveRoot(first.Name)
	isDir := first.Typeflag == tar.TypeDir || cleanName(first.Name) != from

	placement, err := place(source{isDir: isDir, name: name}, dst, localPaths)
	if err != nil {
		return err
	}

	return extract(archive, first, placement.dir, from, placement.root)
}

// archiveRoot is the root of an archive whose first entry is called name:
// "." when its entries are named after a directory's contents, else the
// first element of name.
func archiveRoot(name string) string {
	trimmed := strings.TrimLeft(name, "/")
	if trimmed == "." || strings.HasPrefix(trimmed, "./") {
		return "."
	}

	root, _, _ := strings.Cut(cleanName(name), "/")

	return root
}

// cleanName is an entry's name, cleaned and taken as relative, as tar takes
// it.
func cleanName(name string) string {
	trimmed := strings.TrimLeft(name, "/")
	if len(trimmed) == 0 {
		return "."
	}

	return path.Clean(trimmed)
}

// extraction unpacks an archive into a directory, and nowhere else: no
// entry is written outside it, or through a symlink.
type extraction struct {
	root string
	from string
	to   string

	// real are the directories under root known to be directories of
	// their own, by the names they were reached by.
	real map[string]bool

	// made are the directories extraction made, which are given their
	// modes and times once all of their contents are in them.
	made []madeDir
}

type madeDir struct {
	path    string
	mode    fs.FileMode
	modTime time.Time
}

// extract unpacks first and what follows it in archive into dir, renaming
// the archive's root from to to: "." into dir as they are.
func extract(archive *tar.Reader, first *tar.Header, dir, from, to string) error {
	root, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return err
	}

	x := &extraction{root: root, from: from, to: to, real: map[string]bool{}}

	header := first

	for {
		if err := x.entry(archive, header); err != nil {
			return err
		}

		next, err := archive.Next()
		if errors.Is(err, io.EOF) {
			return x.finish()
		}

		if err != nil {
			return fmt.Errorf("reading the archive: %w", err)
		}

		header = next
	}
}

// rename is where the entry named raw lands under the root, slash
// separated, and an error for one that would land outside it.
func (x *extraction) rename(raw string) (string, error) {
	name := cleanName(raw)
	if !filepath.IsLocal(filepath.FromSlash(name)) {
		return "", fmt.Errorf("refusing %q: it is outside the archive", raw)
	}

	var relative string

	switch {
	case x.from == ".":
		relative = name
	case name == x.from:
		relative = "."
	case strings.HasPrefix(name, x.from+"/"):
		relative = name[len(x.from)+1:]
	default:
		return "", fmt.Errorf("refusing %q: it is not in %q, which is what was copied", raw, x.from)
	}

	switch {
	case x.to == ".":
		return relative, nil
	case relative == ".":
		return x.to, nil
	}

	return x.to + "/" + relative, nil
}

// entry unpacks one entry.
func (x *extraction) entry(archive *tar.Reader, header *tar.Header) error {
	switch header.Typeflag {
	case tar.TypeDir, tar.TypeReg, tar.TypeSymlink, tar.TypeLink:
	default:
		return nil
	}

	name, err := x.rename(header.Name)
	if err != nil {
		return err
	}

	if name == "." {
		if header.Typeflag != tar.TypeDir {
			return fmt.Errorf("refusing %q: it would replace the directory it is copied into", header.Name)
		}

		return nil
	}

	if err := x.prepare(name); err != nil {
		return err
	}

	target := filepath.Join(x.root, filepath.FromSlash(name))

	if header.Typeflag == tar.TypeDir {
		return x.directory(target, name, header)
	}

	if err := x.clear(target, header.Name); err != nil {
		return err
	}

	switch header.Typeflag {
	case tar.TypeSymlink:
		return x.symlink(target, name, header)
	case tar.TypeLink:
		return x.link(target, header)
	}

	return x.file(target, archive, header)
}

// prepare makes sure the directories name is in are directories of their
// own under the root, never reached through a symlink, making those that
// are not there.
func (x *extraction) prepare(name string) error {
	parent := path.Dir(name)
	if parent == "." || x.real[parent] {
		return nil
	}

	if err := x.prepare(parent); err != nil {
		return err
	}

	target := filepath.Join(x.root, filepath.FromSlash(parent))
	info, err := os.Lstat(target)

	switch {
	case errors.Is(err, fs.ErrNotExist):
		err = os.Mkdir(target, 0o755)
	case err != nil:
	case info.Mode()&fs.ModeSymlink != 0:
		err = fmt.Errorf("refusing to write %s through the symlink %s", name, parent)
	case !info.IsDir():
		err = fmt.Errorf("cannot write %s: %w: %s", name, ErrNotDirectory, parent)
	}

	if err != nil {
		return err
	}

	x.real[parent] = true

	return nil
}

// directory makes the directory target, or leaves it as it is when it is
// there already.
func (x *extraction) directory(target, name string, header *tar.Header) error {
	info, err := os.Lstat(target)

	switch {
	case err == nil && !info.IsDir():
		return fmt.Errorf("cannot overwrite %s, which is not a directory, with a directory", name)
	case errors.Is(err, fs.ErrNotExist):
		err = os.Mkdir(target, 0o700)
		x.made = append(x.made, madeDir{path: target, mode: header.FileInfo().Mode().Perm(), modTime: header.ModTime})
	}

	if err != nil {
		return err
	}

	x.real[name] = true

	return nil
}

// clear makes room for an entry that is not a directory at target: what
// is there is replaced, but for a directory. What cannot be looked at is
// left to making the entry, which says why it cannot be made.
func (x *extraction) clear(target, raw string) error {
	info, err := os.Lstat(target)

	switch {
	case err != nil:
		return nil
	case info.IsDir():
		return fmt.Errorf("cannot overwrite the directory %s with %q, which is not one", target, raw)
	}

	return os.Remove(target)
}

// symlink makes the symlink target, refusing one that points outside the
// root by a relative path.
func (x *extraction) symlink(target, name string, header *tar.Header) error {
	if !path.IsAbs(header.Linkname) {
		pointed := path.Join(path.Dir(name), header.Linkname)
		if !filepath.IsLocal(filepath.FromSlash(pointed)) {
			return fmt.Errorf("refusing %q: it points outside the directory it is copied into, at %q", header.Name, header.Linkname)
		}
	}

	return os.Symlink(header.Linkname, target)
}

// link makes target a hard link to an entry unpacked before it.
func (x *extraction) link(target string, header *tar.Header) error {
	name, err := x.rename(header.Linkname)
	if err != nil {
		return err
	}

	if err := x.prepare(name); err != nil {
		return err
	}

	existing := filepath.Join(x.root, filepath.FromSlash(name))

	info, err := os.Lstat(existing)
	if err != nil {
		return err
	}

	if !info.Mode().IsRegular() {
		return fmt.Errorf("refusing %q: it links to %q, which is not a file", header.Name, header.Linkname)
	}

	return os.Link(existing, target)
}

// file writes the file target, as its entry has it.
func (x *extraction) file(target string, content io.Reader, header *tar.Header) error {
	file, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}

	_, copyErr := io.Copy(file, content)

	return errors.Join(copyErr, file.Close(), os.Chmod(target, header.FileInfo().Mode().Perm()), os.Chtimes(target, header.ModTime, header.ModTime))
}

// finish gives the directories it made their modes and times, the deepest
// first, so that a directory's time is not changed by what is made in it.
func (x *extraction) finish() error {
	var errs []error

	for i := len(x.made) - 1; i >= 0; i-- {
		made := x.made[i]
		errs = append(errs, os.Chmod(made.path, made.mode), os.Chtimes(made.path, made.modTime, made.modTime))
	}

	return errors.Join(errs...)
}
