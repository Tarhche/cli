package upgrade

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// replaceBinary puts the binary the archive called name, downloaded to
// archive, holds in place of target's, with the mode the replaced one had.
func (u *Upgrader) replaceBinary(version string, target target, name, archive string) (Installation, error) {
	staged, err := u.stageBinary(target, name, archive)
	if err != nil {
		return Installation{}, err
	}

	if err := u.swap(staged, target.path); err != nil {
		_ = u.files.remove(staged)

		return Installation{}, err
	}

	return Installation{Version: version, Path: target.path, Executable: target.path}, nil
}

// stageBinary extracts the binary the archive called name, downloaded to
// archive, holds into a file of its own beside target's, and is that file's
// path.
func (u *Upgrader) stageBinary(target target, name, archive string) (string, error) {
	file, err := u.files.createTemp(filepath.Dir(target.path), stagingPattern)
	if err != nil {
		return "", denied(target.path, err)
	}

	path := file.Name()

	err = extractBinary(archive, name, u.platform.binary(), file, u.limit)
	if err == nil {
		err = file.Chmod(modeOf(target.path))
	}

	if closeErr := file.Close(); err == nil {
		err = closeErr
	}

	if err != nil {
		_ = u.files.remove(path)

		return "", err
	}

	return path, nil
}

// modeOf is the mode of the binary at path, which the one replacing it
// keeps: 0755 when it cannot be read.
func modeOf(path string) fs.FileMode {
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() == 0 {
		return 0o755
	}

	return info.Mode().Perm()
}

// swap puts the binary at staged in place of the one at path. Elsewhere
// it is renamed over it, which the process running the old one outlives;
// Windows will not replace a running binary, but lets it be renamed, so
// there the old one is put aside first, and put back if the new one cannot
// take its place.
func (u *Upgrader) swap(staged, path string) error {
	if u.platform.OS != "windows" {
		return denied(path, u.files.rename(staged, path))
	}

	aside := u.asideOf(path)

	if err := u.files.rename(path, aside); err != nil {
		return denied(path, err)
	}

	if err := u.files.rename(staged, path); err != nil {
		return u.restore(aside, path, denied(path, err))
	}

	_ = u.files.remove(aside)

	return nil
}

// restore puts what was put aside back at path once putting something else
// there failed with err, and is err, with why it could not be put back and
// where it is kept if it could not.
func (u *Upgrader) restore(aside, path string, err error) error {
	if restoreErr := u.files.rename(aside, path); restoreErr != nil {
		return errors.Join(err, fmt.Errorf("putting %s back from %s: %w", path, aside, restoreErr))
	}

	return err
}

// asideOf is where the running Windows binary at path is put aside while
// another takes its place: path.old, once what an earlier upgrade left there
// is gone, else a name of its own, while that is still running.
func (u *Upgrader) asideOf(path string) string {
	aside := path + ".old"

	if err := u.files.remove(aside); err == nil || errors.Is(err, fs.ErrNotExist) {
		return aside
	}

	return fmt.Sprintf("%s.%d.old", path, time.Now().UnixNano())
}

// replaceBundle puts the app bundle the archive called name, downloaded to
// archive, holds in place of target's: extracted beside it, the old one is
// put aside, and put back if the new one cannot take its place.
func (u *Upgrader) replaceBundle(version string, target target, name, archive string) (Installation, error) {
	staging, err := u.files.mkdirTemp(filepath.Dir(target.path), stagingPattern)
	if err != nil {
		return Installation{}, denied(target.path, err)
	}

	aside := filepath.Join(staging, "replaced.app")

	defer func() {
		if _, err := os.Lstat(aside); err != nil {
			_ = u.files.removeAll(staging)
		}
	}()

	if err := unzipBundle(archive, name, staging, u.limit); err != nil {
		return Installation{}, err
	}

	staged := filepath.Join(staging, appBundle)
	executable := filepath.Join("Contents", "MacOS", filepath.Base(target.executable))

	if err := executableIn(staged, executable); err != nil {
		return Installation{}, fmt.Errorf("%w: %s holds no %s", ErrNoBuild, name, filepath.ToSlash(filepath.Join(appBundle, executable)))
	}

	if err := u.files.rename(target.path, aside); err != nil {
		return Installation{}, denied(target.path, err)
	}

	if err := u.files.rename(staged, target.path); err != nil {
		return Installation{}, u.restore(aside, target.path, denied(target.path, err))
	}

	_ = u.files.removeAll(aside)

	return Installation{Version: version, Path: target.path, Executable: filepath.Join(target.path, executable)}, nil
}

// executableIn makes sure the file at executable under bundle is a regular
// file anybody may run.
func executableIn(bundle, executable string) error {
	path := filepath.Join(bundle, executable)

	info, err := os.Lstat(path)
	if err != nil {
		return err
	}

	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s is not a file", path)
	}

	if info.Mode().Perm()&0o111 == 0o111 {
		return nil
	}

	return os.Chmod(path, info.Mode().Perm()|0o755)
}
