package access

import (
	"context"

	"github.com/Tarhche/cli/internal/stream"
	"github.com/Tarhche/cli/internal/workload"
)

// streamPermissions are the permissions the streams of anybody's VMs are
// opened under, by action; one's own are opened under the self permission
// of each as well.
var streamPermissions = map[string]string{
	stream.ActionAttach:  "workload.vms.attach",
	stream.ActionExec:    "workload.vms.exec",
	stream.ActionForward: "workload.vms.forward",
}

// StreamPermission is the permission the stream action of a VM of audience
// is named by: workload.vms.<action> on anybody's, and its self permission,
// self.workload.vms.<action>, on one's own, where the anybody's one opens it
// too. A container's streams are its Docker VM's: docker exec, docker
// attach, docker cp and a relay to one of its ports are all that VM's exec,
// as a VM's files and a relay to a port it does not publish are; a port it
// publishes is its forward. An action of no VM's is "".
func StreamPermission(action string, audience workload.Audience) string {
	permission, found := streamPermissions[action]
	if !found || audience == workload.Everyone {
		return permission
	}

	return "self." + permission
}

// AllowsStream reports whether the roles allow the stream action of a VM of
// audience, as the node holding the VM decides it from the session's token
// alone: on anybody's, the anybody's permission; on one's own, its self
// permission or the anybody's one. An action of no VM's, always.
func (g Granted) AllowsStream(action string, audience workload.Audience) bool {
	anybodys := StreamPermission(action, workload.Everyone)
	if len(anybodys) == 0 || g.Has(anybodys) {
		return true
	}

	return audience != workload.Everyone && g.Has(StreamPermission(action, audience))
}

// StreamExplainer says why the ingress refused to open a stream for the
// roles of whoever asked (403): on a VM, the permission its action needs on
// the VMs of its audience, which none of the session's roles gives. The node
// decides from what the session's token says alone, so that is all there is
// to say.
type StreamExplainer struct {
	audience workload.Audience
}

var _ stream.Explainer = StreamExplainer{}

// NewStreamExplainer explains the refusals of streams opened on the VMs of
// audience: one's own, or anybody's.
func NewStreamExplainer(audience workload.Audience) StreamExplainer {
	return StreamExplainer{audience: audience}
}

// Explain is the stream refused, denied: naming the permission it needs when
// it is a VM's.
func (e StreamExplainer) Explain(_ context.Context, refused *stream.Error) error {
	denied := &DeniedError{Refused: refused}

	if refused.Kind == stream.KindVMs {
		denied.Permission = StreamPermission(refused.Action, e.audience)
	}

	return denied
}
