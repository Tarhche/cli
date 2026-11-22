package access

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/stream"
	"github.com/Tarhche/cli/internal/workload"
)

func TestStreamPermission(t *testing.T) {
	t.Parallel()

	cases := []struct {
		action   string
		mine     string
		everyone string
	}{
		{action: stream.ActionAttach, mine: "self.workload.vms.attach", everyone: "workload.vms.attach"},
		{action: stream.ActionExec, mine: "self.workload.vms.exec", everyone: "workload.vms.exec"},
		{action: stream.ActionForward, mine: "self.workload.vms.forward", everyone: "workload.vms.forward"},
		{action: "logs"},
	}

	for _, c := range cases {
		t.Run("the "+c.action+" action, on one's own vm and on anybody's", func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, c.mine, StreamPermission(c.action, workload.Mine))
			assert.Equal(t, c.everyone, StreamPermission(c.action, workload.Everyone))
		})
	}

	t.Run("a token naming the permission an action is named by allows it", func(t *testing.T) {
		t.Parallel()

		for action := range streamPermissions {
			for _, audience := range []workload.Audience{workload.Mine, workload.Everyone} {
				assert.True(t, GrantedBy(tokenAllowing(StreamPermission(action, audience))).AllowsStream(action, audience), action)
			}
		}
	})
}

func TestStreamExplainer(t *testing.T) {
	t.Parallel()

	refusalOf := func(kind, action string) *stream.Error {
		return &stream.Error{Stream: kind + "/0190/" + action, Kind: kind, Action: action, StatusCode: http.StatusForbidden}
	}

	t.Run("a vm's stream refused is the permission its action needs on one's own vms, denied", func(t *testing.T) {
		t.Parallel()

		refused := refusalOf(stream.KindVMs, stream.ActionExec)

		err := NewStreamExplainer(workload.Mine).Explain(context.Background(), refused)

		var denied *DeniedError
		require.ErrorAs(t, err, &denied)
		assert.Equal(t, &DeniedError{Permission: "self.workload.vms.exec", Refused: refused}, denied)
		assert.EqualError(t, err, `permission denied: this needs the "self.workload.vms.exec" permission, which none of your roles gives you`)
		assert.True(t, stream.IsForbidden(err), "a denial is still the ingress's refusal")
	})

	t.Run("on anybody's, the anybody's permission", func(t *testing.T) {
		t.Parallel()

		err := NewStreamExplainer(workload.Everyone).Explain(context.Background(), refusalOf(stream.KindVMs, stream.ActionForward))

		assert.EqualError(t, err, `permission denied: this needs the "workload.vms.forward" permission, which none of your roles gives you`)
	})

	t.Run("a stream of no vm's, or an action no permission is known for, is denied as the ingress said", func(t *testing.T) {
		t.Parallel()

		for _, refused := range []*stream.Error{refusalOf(stream.KindTasks, stream.ActionAttach), refusalOf(stream.KindVMs, "logs")} {
			err := NewStreamExplainer(workload.Mine).Explain(context.Background(), refused)

			var denied *DeniedError
			require.ErrorAs(t, err, &denied, refused.Stream)
			assert.Empty(t, denied.Permission, refused.Stream)
			assert.EqualError(t, err, "permission denied: your roles do not allow this", refused.Stream)
		}
	})
}
