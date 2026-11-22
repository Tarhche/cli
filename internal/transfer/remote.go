package transfer

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
)

// statScript says what $1 is, on a line of its own: a symlink, followed by
// what it points to, a directory, a file, or missing. It is POSIX sh, but
// for readlink, which every Linux has.
const statScript = `if [ -L "$1" ]; then echo symlink; readlink "$1"; elif [ -d "$1" ]; then echo directory; elif [ -e "$1" ]; then echo file; else echo missing; fi`

// VM is a VM files are copied in and out of by tar and a POSIX shell,
// which runner runs in it. Its paths are taken from its root, as docker
// takes a container's.
func VM(runner Runner) Remote {
	return vm{runner: runner}
}

type vm struct {
	runner Runner
}

// Stat says what path is, as the shell finds it.
func (v vm) Stat(ctx context.Context, path string) (Info, error) {
	var said bytes.Buffer

	if err := v.runner.Run(ctx, []string{"sh", "-c", statScript, "sh", fromRoot(path)}, nil, &said); err != nil {
		return Info{}, err
	}

	kind, rest, _ := strings.Cut(said.String(), "\n")

	switch kind {
	case "directory":
		return Info{Exists: true, IsDir: true}, nil
	case "file":
		return Info{Exists: true}, nil
	case "missing":
		return Info{}, nil
	case "symlink":
		return Info{Exists: true, Link: strings.TrimSuffix(rest, "\n")}, nil
	}

	return Info{}, fmt.Errorf("cannot tell what %s is from %q", path, said.String())
}

// Archive archives path with tar, once it found it is there.
func (v vm) Archive(ctx context.Context, path string, follow bool, w io.Writer) error {
	found, err := v.find(ctx, fromRoot(path), follow)
	if err != nil {
		return err
	}

	dir, base := remotePaths.split(found)
	if base == "/" {
		base = "."
	}

	return v.runner.Run(ctx, []string{"tar", "-c", "-f", "-", "-C", dir, "--", base}, nil, w)
}

// find is path, or what the symlink it is points to when follow says so,
// and a *NotFoundError when it is not there.
func (v vm) find(ctx context.Context, path string, follow bool) (string, error) {
	current := path

	for range maxSymlinks + 1 {
		info, err := v.Stat(ctx, current)

		switch {
		case err != nil:
			return "", err
		case !info.Exists:
			return "", &NotFoundError{Path: current}
		case len(info.Link) == 0 || !follow:
			return current, nil
		}

		current = remotePaths.linkTarget(current, info.Link)
	}

	return "", fmt.Errorf("too many symlinks in %s", path)
}

// Extract unpacks r with tar, as whoever runs it unless owners keeps the
// owners the archive names.
func (v vm) Extract(ctx context.Context, dir string, r io.Reader, owners bool) error {
	command := []string{"tar", "-x"}
	if !owners {
		command = append(command, "-o")
	}

	return v.runner.Run(ctx, append(command, "-f", "-", "-C", fromRoot(dir)), r, io.Discard)
}

// fromRoot is path taken from the root: a path that is not absolute is made
// one, its trailing separator or "." kept.
func fromRoot(path string) string {
	if strings.HasPrefix(path, "/") {
		return path
	}

	return "/" + path
}

// Container is a container files are copied in and out of by docker cp,
// which runner runs in the Docker VM it lives in. id is its Docker id or
// name.
func Container(runner Runner, id string) Remote {
	return container{runner: runner, id: id}
}

type container struct {
	runner Runner
	id     string
}

// errStatted is what an archive read to stat a path reads once the path's
// first entry was read.
var errStatted = errors.New("statted")

// Stat says what path is from the first entry of its archive, which docker
// cp makes of a stopped container as well as of a running one, and of an
// image with no shell. Docker cp failing says it is not there, as far as a
// copy into it goes: docker's own cp takes it so.
func (c container) Stat(ctx context.Context, path string) (Info, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	reader, writer := io.Pipe()
	ran := make(chan error, 1)

	go func() {
		err := c.runner.Run(ctx, []string{"docker", "cp", c.id + ":" + path, Stdio}, nil, writer)
		_ = writer.CloseWithError(err)
		ran <- err
	}()

	header, readErr := tar.NewReader(reader).Next()

	_ = reader.CloseWithError(errStatted)
	cancel()

	runErr := <-ran

	var failed exitCoder

	switch {
	case readErr == nil:
		return infoOf(header), nil
	case errors.As(runErr, &failed):
		return Info{}, nil
	case runErr != nil:
		return Info{}, runErr
	}

	return Info{}, fmt.Errorf("cannot tell what %s is: %w", path, readErr)
}

// infoOf is what the entry of header is.
func infoOf(header *tar.Header) Info {
	switch header.Typeflag {
	case tar.TypeDir:
		return Info{Exists: true, IsDir: true}
	case tar.TypeSymlink:
		return Info{Exists: true, Link: header.Linkname}
	}

	return Info{Exists: true}
}

// Archive archives path with docker cp.
func (c container) Archive(ctx context.Context, path string, follow bool, w io.Writer) error {
	command := []string{"docker", "cp"}
	if follow {
		command = append(command, "--follow-link")
	}

	return c.runner.Run(ctx, append(command, c.id+":"+path, Stdio), nil, w)
}

// Extract unpacks r with docker cp, as the container's root unless owners
// keeps the owners the archive names.
func (c container) Extract(ctx context.Context, dir string, r io.Reader, owners bool) error {
	command := []string{"docker", "cp"}
	if owners {
		command = append(command, "--archive")
	}

	return c.runner.Run(ctx, append(command, Stdio, c.id+":"+dir), r, io.Discard)
}
