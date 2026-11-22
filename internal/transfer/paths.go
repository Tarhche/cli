package transfer

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// flavor is how paths are spelled on one side of a copy: this machine's
// way, or a workload's, which is POSIX.
type flavor struct {
	separator byte
	clean     func(string) string
	dir       func(string) string
	base      func(string) string
	join      func(...string) string
	isAbs     func(string) bool
}

var (
	localPaths  = flavor{separator: filepath.Separator, clean: filepath.Clean, dir: filepath.Dir, base: filepath.Base, join: filepath.Join, isAbs: filepath.IsAbs}
	remotePaths = flavor{separator: '/', clean: path.Clean, dir: path.Dir, base: path.Base, join: path.Join, isAbs: path.IsAbs}
)

// split splits p into the directory it is in and its last element, as
// docker does: cleaned, but a trailing "." kept, which names a directory's
// contents rather than the directory.
func (f flavor) split(p string) (dir, base string) {
	cleaned := f.clean(p)
	if f.specifiesCurrentDir(p) {
		cleaned += string(f.separator) + "."
	}

	return f.dir(cleaned), f.base(cleaned)
}

// specifiesCurrentDir reports whether p's last element is ".".
func (f flavor) specifiesCurrentDir(p string) bool {
	return f.base(p) == "."
}

// hasTrailingSeparator reports whether p ends with a separator.
func (f flavor) hasTrailingSeparator(p string) bool {
	return len(p) > 0 && p[len(p)-1] == f.separator
}

// assertsDirectory reports whether p says it is a directory, ending with a
// separator or ".".
func (f flavor) assertsDirectory(p string) bool {
	return f.hasTrailingSeparator(p) || f.specifiesCurrentDir(p)
}

// linkTarget is where the symlink at p, pointing to link, points.
func (f flavor) linkTarget(p, link string) string {
	if f.isAbs(link) {
		return link
	}

	dir, _ := f.split(p)

	return f.join(dir, link)
}

// source is what is copied: whether it is a directory, and what the root
// of its archive is called.
type source struct {
	isDir bool
	name  string
}

// destination is where a copy lands, as it was found: its path, symlinks
// followed, whether it is there and whether it is a directory.
type destination struct {
	path   string
	exists bool
	isDir  bool
}

// placement is where an archive is unpacked, and what its root is called
// there.
type placement struct {
	dir  string
	root string
}

// place decides, as docker does, where an archive of src is unpacked for
// the copy to land at dst: inside dst when it is a directory that is there,
// else in the directory dst is in, its root named as dst is.
func place(src source, dst destination, paths flavor) (placement, error) {
	dir, base := paths.split(dst.path)

	switch {
	case dst.exists && dst.isDir:
		return placement{dir: dst.path, root: src.name}, nil
	case dst.exists && src.isDir:
		return placement{}, fmt.Errorf("%w: %s", ErrCannotCopyDir, dst.path)
	case !dst.exists && !src.isDir && paths.assertsDirectory(dst.path):
		return placement{}, fmt.Errorf("%w: %s", ErrNoSuchDirectory, dst.path)
	}

	return placement{dir: dir, root: base}, nil
}

// localDestination finds where a copy to this machine lands: dst, or what
// the symlink it is points to, followed as docker follows it. What is not
// there has to be in a directory that is.
func localDestination(dst string) (destination, error) {
	current := dst

	for range maxSymlinks + 1 {
		link, err := os.Readlink(current)
		if err != nil {
			return foundDestination(current)
		}

		current = localPaths.linkTarget(current, link)
	}

	return destination{}, fmt.Errorf("too many symlinks in %s", dst)
}

// foundDestination is dst, which is not a symlink, as it was found.
func foundDestination(dst string) (destination, error) {
	info, err := os.Lstat(dst)

	switch {
	case errors.Is(err, fs.ErrNotExist):
		return missingDestination(dst)
	case err != nil:
		return destination{}, err
	}

	return destination{path: dst, exists: true, isDir: info.IsDir()}, nil
}

// missingDestination is a destination that is not there, in the directory
// it is to be made in.
func missingDestination(dst string) (destination, error) {
	parent, _ := localPaths.split(dst)

	if info, err := os.Stat(parent); err != nil || !info.IsDir() {
		return destination{}, fmt.Errorf("the directory %s does not exist", parent)
	}

	return destination{path: dst}, nil
}

// localSource finds what is copied from this machine, as docker does: the
// symlinks of the directory src is in followed, and src itself too when
// follow says so, in which case the archive's root keeps src's name. It is
// the path that is archived, and what it is.
func localSource(src string, follow bool) (string, source, error) {
	var resolved, rebased string

	if follow {
		target, err := filepath.EvalSymlinks(src)
		if err != nil {
			return "", source{}, err
		}

		resolved = target

		if localPaths.specifiesCurrentDir(src) && !localPaths.specifiesCurrentDir(resolved) {
			resolved += string(filepath.Separator) + "."
		}
	} else {
		dir, base := filepath.Split(src)
		if len(dir) == 0 {
			dir = "."
		}

		resolvedDir, err := filepath.EvalSymlinks(dir)
		if err != nil {
			return "", source{}, err
		}

		resolved = strings.TrimRight(resolvedDir, string(filepath.Separator)) + string(filepath.Separator) + base
	}

	if base := localPaths.base(src); base != localPaths.base(resolved) {
		rebased = base
	}

	info, err := os.Lstat(resolved)
	if err != nil {
		return "", source{}, err
	}

	_, name := localPaths.split(resolved)
	if len(rebased) > 0 {
		name = rebased
	}

	return resolved, source{isDir: info.IsDir(), name: name}, nil
}
