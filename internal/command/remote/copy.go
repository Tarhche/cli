package remote

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Tarhche/cli/internal/formatter"
	"github.com/Tarhche/cli/internal/transfer"
)

// copyFlagUsages are the usages of cp's flags, docker's.
const (
	archiveUsage    = "Archive mode (copy all uid/gid information)"
	followLinkUsage = "Always follow symbol link in SRC_PATH"
	quietUsage      = "Suppress progress output during copy. Progress output is automatically suppressed if no terminal is attached"
)

// copyPlan is a copy that was asked for: which way, the workload's
// reference and path, and the path here.
type copyPlan struct {
	download   bool
	reference  string
	remotePath string
	localPath  string
}

// planCopy reads cp's arguments, one naming a path in a workload of kind as
// REFERENCE:PATH and the other a path here.
func planCopy(src, dst, kind string) (copyPlan, error) {
	switch {
	case len(src) == 0:
		return copyPlan{}, errors.New("source can not be empty")
	case len(dst) == 0:
		return copyPlan{}, errors.New("destination can not be empty")
	}

	srcReference, srcPath := splitCopyArg(src)
	dstReference, dstPath := splitCopyArg(dst)

	var plan copyPlan

	switch {
	case len(srcReference) > 0 && len(dstReference) > 0:
		return copyPlan{}, fmt.Errorf("copying between %ss is not supported", kind)
	case len(srcReference) > 0:
		plan = copyPlan{download: true, reference: srcReference, remotePath: srcPath, localPath: dstPath}
	case len(dstReference) > 0:
		plan = copyPlan{reference: dstReference, remotePath: dstPath, localPath: srcPath}
	default:
		return copyPlan{}, fmt.Errorf("must specify at least one %s source", kind)
	}

	if len(plan.remotePath) == 0 {
		return copyPlan{}, fmt.Errorf("the path in %s:%s can not be empty", plan.reference, plan.remotePath)
	}

	return plan, nil
}

// splitCopyArg splits a copy's argument into a workload's reference and the
// path in it, as docker splits CONTAINER:PATH: an absolute path here, and
// one led by ".", is a path here.
func splitCopyArg(arg string) (string, string) {
	if filepath.IsAbs(arg) {
		return "", arg
	}

	reference, path, found := strings.Cut(arg, ":")
	if !found || strings.HasPrefix(reference, ".") {
		return "", arg
	}

	return reference, path
}

// copy carries out plan through remote, the workload being a kind, and says
// how much it copied on a terminal unless quiet says not to.
func (r *remote) copy(ctx context.Context, plan copyPlan, remote transfer.Remote, options transfer.Options, quiet bool, kind string) error {
	var (
		size int64
		err  error
		to   string
	)

	if plan.download {
		size, err = transfer.Download(ctx, remote, plan.remotePath, plan.localPath, r.cli.Out, options)
		to = plan.localPath
	} else {
		size, err = transfer.Upload(ctx, remote, plan.localPath, plan.remotePath, r.cli.In, options)
		to = plan.reference + ":" + plan.remotePath
	}

	if notFound, isNotFound := errors.AsType[*transfer.NotFoundError](err); isNotFound {
		return fmt.Errorf("could not find the file %s in %s %s", notFound.Path, kind, plan.reference)
	}

	if err != nil {
		return err
	}

	if !quiet && to != transfer.Stdio && r.terminal.IsTerminal(r.cli.Err) {
		_, _ = fmt.Fprintf(r.cli.Err, "Successfully copied %s to %s\n", formatter.Bytes(size), to)
	}

	return nil
}
