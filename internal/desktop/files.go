package desktop

import (
	"context"
	"path"

	"github.com/Tarhche/cli/internal/command"
)

// CopyRequest is something to copy between the host and a workload: a file,
// or a directory and what is in it.
type CopyRequest struct {
	Target Target `json:"target"`

	// Path is where it is in the workload, or is to go.
	Path string `json:"path"`

	// LocalPath is where it is on the host, or is to go.
	LocalPath string `json:"local_path"`
}

// Copier copies between the host and workloads, as docker cp does, through
// the workload ingress of the context cli talks to.
type Copier interface {
	// CopyIn copies what is at the request's local path into the workload,
	// at its path.
	CopyIn(ctx context.Context, cli *command.Cli, request CopyRequest) error

	// CopyOut copies what is at the request's path in the workload to its
	// local path.
	CopyOut(ctx context.Context, cli *command.Cli, request CopyRequest) error
}

// Chooser asks the user for a path on the host, in the operating system's
// own dialogs.
type Chooser interface {
	// Open asks for a file or a directory to read; empty is none chosen.
	Open(title string) (string, error)

	// Save asks where to write what is called filename; empty is none
	// chosen.
	Save(title, filename string) (string, error)
}

// Files is how the frontend copies between the host and workloads, asking
// where on the host in the operating system's own dialogs.
type Files struct {
	workspace *Workspace
	copier    Copier
	chooser   Chooser
}

// NewFiles is the file service of workspace, copying through copier and
// asking for paths on the host through chooser.
func NewFiles(workspace *Workspace, copier Copier, chooser Chooser) *Files {
	return &Files{workspace: workspace, copier: copier, chooser: chooser}
}

// CopyIn asks for a file or a directory on the host and copies it into
// target at remotePath. It answers what it copied, or nothing when nothing
// was chosen.
func (f *Files) CopyIn(ctx context.Context, target Target, remotePath string) (string, error) {
	local, err := f.chooser.Open("Copy into " + remotePath)
	if err != nil || len(local) == 0 {
		return "", err
	}

	if err := f.copier.CopyIn(ctx, f.workspace.Cli(), CopyRequest{Target: target, Path: remotePath, LocalPath: local}); err != nil {
		return "", err
	}

	return local, nil
}

// CopyOut asks where on the host to copy what is at remotePath in target,
// and copies it there. It answers where it copied it, or nothing when
// nowhere was chosen.
func (f *Files) CopyOut(ctx context.Context, target Target, remotePath string) (string, error) {
	local, err := f.chooser.Save("Copy "+remotePath+" to", path.Base(remotePath))
	if err != nil || len(local) == 0 {
		return "", err
	}

	if err := f.copier.CopyOut(ctx, f.workspace.Cli(), CopyRequest{Target: target, Path: remotePath, LocalPath: local}); err != nil {
		return "", err
	}

	return local, nil
}
