package credentials

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

// FileName is the file sessions are kept in when there is no keychain,
// beside the configuration.
const FileName = "credentials.json"

// File keeps credentials in a file only its owner may read and write: an
// object of the credentials of every context, by its name.
type File struct {
	path string
	lock sync.Mutex
}

var _ Store = &File{}

// NewFile is a store keeping credentials in FileName in dir.
func NewFile(dir string) *File {
	return &File{path: filepath.Join(dir, FileName)}
}

// Path is where the file is.
func (f *File) Path() string {
	return f.path
}

// Get is the credentials kept for a context, or ErrNotFound.
func (f *File) Get(context string) (Credentials, error) {
	f.lock.Lock()
	defer f.lock.Unlock()

	kept, err := f.read()
	if err != nil {
		return Credentials{}, err
	}

	credentials, found := kept[context]
	if !found {
		return Credentials{}, ErrNotFound
	}

	return credentials, nil
}

// Set keeps credentials for a context, replacing what was kept.
func (f *File) Set(context string, credentials Credentials) error {
	f.lock.Lock()
	defer f.lock.Unlock()

	kept, err := f.read()
	if err != nil {
		return err
	}

	kept[context] = credentials

	return f.write(kept)
}

// Delete forgets the credentials of a context.
func (f *File) Delete(context string) error {
	f.lock.Lock()
	defer f.lock.Unlock()

	kept, err := f.read()
	if err != nil {
		return err
	}

	if _, found := kept[context]; !found {
		return nil
	}

	delete(kept, context)

	return f.write(kept)
}

func (f *File) read() (map[string]Credentials, error) {
	kept := make(map[string]Credentials)

	content, err := os.ReadFile(f.path)
	if errors.Is(err, fs.ErrNotExist) {
		return kept, nil
	} else if err != nil {
		return nil, fmt.Errorf("reading %s: %w", f.path, err)
	}

	if err := json.Unmarshal(content, &kept); err != nil {
		return nil, fmt.Errorf("reading %s: %w", f.path, err)
	}

	if kept == nil {
		kept = make(map[string]Credentials)
	}

	return kept, nil
}

// write replaces the file with kept at once rather than a write at a time,
// readable by its owner alone.
func (f *File) write(kept map[string]Credentials) error {
	dir := filepath.Dir(f.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating %s: %w", dir, err)
	}

	content, err := json.MarshalIndent(kept, "", "  ")
	if err != nil {
		return err
	}

	temporary, err := os.CreateTemp(dir, FileName+".*")
	if err != nil {
		return fmt.Errorf("writing %s: %w", f.path, err)
	}

	defer os.Remove(temporary.Name())

	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()

		return fmt.Errorf("writing %s: %w", f.path, err)
	}

	if _, err := temporary.Write(append(content, '\n')); err != nil {
		_ = temporary.Close()

		return fmt.Errorf("writing %s: %w", f.path, err)
	}

	if err := temporary.Close(); err != nil {
		return fmt.Errorf("writing %s: %w", f.path, err)
	}

	if err := os.Rename(temporary.Name(), f.path); err != nil {
		return fmt.Errorf("writing %s: %w", f.path, err)
	}

	return nil
}
