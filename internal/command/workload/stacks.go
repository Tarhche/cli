package workload

import (
	"time"

	"github.com/Tarhche/cli/internal/formatter"
	"github.com/Tarhche/cli/internal/workload"
)

// stackTable lists stacks, as docker's stack ls does, with what the
// platform knows of each.
const stackTable = `{{.UUID}}\t{{.Name}}\t{{.Services}}\t{{.VM}}\t{{.State}}\t{{.RunningFor}}`

// stackContainersTable lists the containers of a stack, as docker's stack
// ps does its tasks.
const stackContainersTable = `{{.ID}}\t{{.Names}}\t{{.Service}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}`

// stackHeaders name the columns that are not named after their field.
var stackHeaders = map[string]string{"RunningFor": "CREATED"}

// stackRow is a stack as a listing shows one.
type stackRow struct {
	UUID       string
	Name       string
	Slug       string
	Services   int
	State      string
	CreatedAt  time.Time
	RunningFor string
	VM         string
	VMUUID     string
	Owner      string
}

func stackRowOf(s workload.StackManifest) stackRow {
	return stackRow{
		UUID:       s.Metadata.UUID,
		Name:       s.Metadata.Name,
		Slug:       s.Metadata.Slug,
		Services:   len(s.Status.Services),
		State:      stateOf(s.Status.Status, s.Status.Failure),
		CreatedAt:  s.Metadata.CreatedAt,
		RunningFor: formatter.Ago(s.Metadata.CreatedAt),
		VM:         orDefault(s.VMName, s.Spec.VM.UUID),
		VMUUID:     s.Spec.VM.UUID,
		Owner:      ownerOf(s.Owner, s.Metadata.OwnerUUID),
	}
}

// stateOf is what something is doing, in its kind's words, and why it
// failed when it did.
func stateOf(status workload.Status, failure *workload.Failure) string {
	if status.State != stateFailed {
		return status.State
	}

	if failure != nil && len(failure.Message) > 0 {
		return status.State + ": " + failure.Message
	}

	if len(status.Reason) > 0 {
		return status.State + ": " + status.Reason
	}

	return status.State
}

// withOwner is a table with whose each item is at its end, as anybody's
// listings show it.
func withOwner(table string, allUsers bool) string {
	if allUsers {
		return table + `\t{{.Owner}}`
	}

	return table
}
