// Package transfer copies files and directories in and out of a workload as
// docker cp copies them in and out of a container: a tar archive of what is
// copied, rebased so that it lands where it is asked to, made on one side
// and unpacked on the other. A workload is reached through a Runner of
// commands in it, which is all the CLI and the desktop app have to give.
package transfer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sync/atomic"
)

// Stdio is the path that means a tar archive on the standard input or
// output instead of a file.
const Stdio = "-"

// maxSymlinks bounds how many symlinks are followed to find a destination.
const maxSymlinks = 10

var (
	// ErrCannotCopyDir is a directory copied onto a file.
	ErrCannotCopyDir = errors.New("cannot copy a directory onto a file")

	// ErrNoSuchDirectory is a file copied into a directory that is not
	// there.
	ErrNoSuchDirectory = errors.New("no such directory")

	// ErrNotDirectory is an archive unpacked into what is not a directory.
	ErrNotDirectory = errors.New("not a directory")

	// errStopped is what an archive still being written reads once its
	// reader stopped.
	errStopped = errors.New("the copy stopped")
)

// Runner runs a command in a workload to its end: stdin is its standard
// input, which nil leaves empty, and its standard output is written to
// stdout. A command that ran and exited with a status other than 0 is an
// error with an ExitCode() int method, saying what it said on its standard
// error.
type Runner interface {
	Run(ctx context.Context, command []string, stdin io.Reader, stdout io.Writer) error
}

// exitCoder is an error of a command that ran, and failed.
type exitCoder interface {
	ExitCode() int
}

// Remote is a workload files are copied in and out of.
type Remote interface {
	// Stat says what path is in the workload.
	Stat(ctx context.Context, path string) (Info, error)

	// Archive writes a tar archive of path to w, its root named after
	// path's last element: "." for path/., a directory's contents. A
	// symlink is archived as it is, or what it points to when follow says
	// so, named after that.
	Archive(ctx context.Context, path string, follow bool, w io.Writer) error

	// Extract unpacks the tar archive r into the directory dir, keeping
	// the owners its entries name when owners says so.
	Extract(ctx context.Context, dir string, r io.Reader, owners bool) error
}

// Info is what a path is in a workload.
type Info struct {
	Exists bool
	IsDir  bool

	// Link is what it points to, when it is a symlink.
	Link string
}

// Options are how a copy is made.
type Options struct {
	// FollowLink copies what a symlink copied points to rather than the
	// symlink, as docker cp's --follow-link does.
	FollowLink bool

	// CopyUIDGID keeps the owners of what is copied into a workload, as
	// docker cp's --archive does; otherwise it is owned by whoever unpacks
	// it there.
	CopyUIDGID bool
}

// NotFoundError is a path that is not in a workload.
type NotFoundError struct {
	Path string
}

// Error says what could not be found.
func (e *NotFoundError) Error() string {
	return "could not find the file " + e.Path
}

// Download copies src in remote to dst on this machine, as docker cp copies
// out of a container, and is the size of the archive it was carried in.
// When dst is Stdio the archive is written to stdout as it is.
func Download(ctx context.Context, remote Remote, src, dst string, stdout io.Writer, options Options) (int64, error) {
	if dst == Stdio {
		counted := &countingWriter{w: stdout}
		err := remote.Archive(ctx, src, options.FollowLink, counted)

		return counted.n.Load(), err
	}

	destination, err := localDestination(filepath.FromSlash(dst))
	if err != nil {
		return 0, err
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	reader, writer := io.Pipe()
	archived := make(chan error, 1)

	go func() {
		err := remote.Archive(ctx, src, options.FollowLink, writer)
		archived <- err
		_ = writer.CloseWithError(err)
	}()

	counted := &countingReader{r: reader}

	err = unpack(counted, archiveName(src), destination)
	if err == nil {
		_, err = io.Copy(io.Discard, counted)
	}

	if err == nil {
		return counted.n.Load(), <-archived
	}

	select {
	case remoteErr := <-archived:
		return counted.n.Load(), cause(err, remoteErr)
	default:
	}

	_ = reader.CloseWithError(err)
	cancel()
	<-archived

	return counted.n.Load(), err
}

// Upload copies src on this machine to dst in remote, as docker cp copies
// into a container, and is the size of the archive it was carried in. When
// src is Stdio the archive is read from stdin, and unpacked into dst, which
// has to be a directory.
func Upload(ctx context.Context, remote Remote, src, dst string, stdin io.Reader, options Options) (int64, error) {
	destination, err := remoteDestination(ctx, remote, dst)
	if err != nil {
		return 0, err
	}

	if src == Stdio {
		if !destination.isDir {
			return 0, fmt.Errorf("%w: %s, which an archive is unpacked into", ErrNotDirectory, dst)
		}

		counted := &countingReader{r: stdin}
		err := remote.Extract(ctx, destination.path, counted, options.CopyUIDGID)

		return counted.n.Load(), err
	}

	local, source, err := localSource(filepath.FromSlash(src), options.FollowLink)
	if err != nil {
		return 0, err
	}

	placement, err := place(source, destination, remotePaths)
	if err != nil {
		return 0, err
	}

	reader, writer := io.Pipe()
	packed := make(chan error, 1)

	go func() {
		err := pack(writer, local, placement.root, options.CopyUIDGID)
		_ = writer.CloseWithError(err)
		packed <- err
	}()

	counted := &countingReader{r: reader}
	err = remote.Extract(ctx, placement.dir, counted, options.CopyUIDGID)

	_ = reader.CloseWithError(errStopped)

	if packErr := <-packed; packErr != nil && !errors.Is(packErr, errStopped) {
		return counted.n.Load(), packErr
	}

	return counted.n.Load(), err
}

// cause is the error a copy failed with once the remote side ended before
// the local one failed: the remote side's when it failed on its own, else
// the local side's. A local side that fails first is the cause on its own:
// the remote side then fails only of the stream it wrote to being closed.
func cause(local, remote error) error {
	switch {
	case remote == nil:
		return local
	case local == nil:
		return remote
	case errors.Is(remote, local), errors.Is(remote, context.Canceled):
		return local
	}

	return remote
}

// remoteDestination finds where a copy into remote lands: dst, or what the
// symlink it is points to, followed as docker follows it.
func remoteDestination(ctx context.Context, remote Remote, dst string) (destination, error) {
	current := dst

	for range maxSymlinks + 1 {
		info, err := remote.Stat(ctx, current)
		if err != nil {
			return destination{}, err
		}

		if len(info.Link) == 0 {
			return destination{path: current, exists: info.Exists, isDir: info.IsDir}, nil
		}

		current = remotePaths.linkTarget(current, info.Link)
	}

	return destination{}, fmt.Errorf("too many symlinks in %s", dst)
}

// archiveName is what the root of an archive of path is called: its last
// element, "." for the contents of a directory.
func archiveName(path string) string {
	_, base := remotePaths.split(path)
	if base == "/" {
		return "."
	}

	return base
}

// countingReader counts what is read through it.
type countingReader struct {
	r io.Reader
	n atomic.Int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n.Add(int64(n))

	return n, err
}

// countingWriter counts what is written through it.
type countingWriter struct {
	w io.Writer
	n atomic.Int64
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n.Add(int64(n))

	return n, err
}
