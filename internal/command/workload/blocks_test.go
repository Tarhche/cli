package workload

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/danceable/console"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/workload/workloadtest"
)

// box2 is a second Docker VM of one's own.
const vmBox2 = "0193a5b2-7c41-7000-8000-000000000003"

var box2 = workloadtest.VM(vmBox2, "box2", true, "running")

var (
	nginxImage = workloadtest.Image("i-nginx", "sha256:1111222233334444aaaa", "nginx:1.27")
	appImage   = workloadtest.Image("i-app", "sha256:5555666677778888bbbb", "ghcr.io/acme/app@sha256:abc").With("status.size", 0)
	backend    = workloadtest.Network("n-backend", "9999888877776666", "backend")
	bridge     = workloadtest.Network("n-bridge", "1234123412341234", "bridge").With("spec.labels", workloadtest.Object{"a": "b"})
	data       = workloadtest.Volume("o-data", "data")
)

func TestListImagesCommand(t *testing.T) {
	t.Parallel()

	header := []string{"REPOSITORY", "TAG", "IMAGE ID", "CREATED", "SIZE", "VM"}

	t.Run("the images of every one of one's docker vms", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, mine+"/vms", http.StatusOK, workloadtest.Page(web, box, box2)).
			On(http.MethodGet, mine+"/vms/"+vmWeb+"/images", http.StatusOK, workloadtest.Items(nginxImage, appImage)).
			On(http.MethodGet, mine+"/vms/"+vmBox2+"/images", http.StatusOK, workloadtest.Items(nginxImage))
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitSuccess, h.Run(newListImagesCommand(h.Cli)))
		assert.Equal(t, table(header,
			[]string{"ghcr.io/acme/app", "<none>", "555566667777", "3 hours ago", "", "web"},
			[]string{"nginx", "1.27", "111122223333", "3 hours ago", "187MiB", "web"},
			[]string{"nginx", "1.27", "111122223333", "3 hours ago", "187MiB", "box2"},
		), h.Out.String())

		_, asked := fake.Sent(http.MethodGet, mine+"/vms/"+vmBox+"/images")
		assert.False(t, asked, "a machine holds no images")
	})

	t.Run("ids alone, untruncated, of anybody's vm", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, everyone+"/vms", http.StatusOK, workloadtest.Page(web)).
			On(http.MethodGet, everyone+"/vms/"+vmWeb+"/images", http.StatusOK, workloadtest.Items(nginxImage, workloadtest.Image("i-new", "", "busybox")))
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitSuccess, h.Run(newListImagesCommand(h.Cli), "-A", "--vm", "web", "-q", "--no-trunc"))
		assert.ElementsMatch(t, []string{"sha256:1111222233334444aaaa", "i-new"}, splitLines(h.Out.String()))
	})

	t.Run("a vm that is not there, or images that cannot be listed, is an error", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, mine+"/vms", http.StatusOK, workloadtest.Page(web)).
			On(http.MethodGet, mine+"/vms/"+vmWeb+"/images", http.StatusInternalServerError, `{"code":"internal"}`)
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitFailure, h.Run(newListImagesCommand(h.Cli), "--vm", "box"))
		assert.Equal(t, console.ExitFailure, h.Run(newListImagesCommand(h.Cli)))
		assert.Equal(t, "Error: no such vm: box\nError: internal\n", h.Err.String())
	})

	t.Run("vms that cannot be listed are an error", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().On(http.MethodGet, mine+"/vms", http.StatusForbidden, `{"code":"forbidden"}`)
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitFailure, h.Run(newListImagesCommand(h.Cli)))
		assert.Equal(t, "Error: forbidden\n", h.Err.String())
	})

	testRequiresArguments(t, newListImagesCommand, `"tarhche ls" accepts no arguments.`, "nginx")
}

func TestPullImageCommand(t *testing.T) {
	t.Parallel()

	t.Run("an image is pulled into one's only docker vm", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, mine+"/vms", http.StatusOK, workloadtest.Page(web, box)).
			On(http.MethodPost, mine+"/vms/"+vmWeb+"/images", http.StatusCreated, workloadtest.Image("i-nginx", "sha256:1", "docker.io/library/nginx:latest"))
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitSuccess, h.Run(newPullImageCommand(h.Cli), "nginx"))
		assert.Equal(t, "docker.io/library/nginx:latest\n", h.Out.String())
		assert.Equal(t, map[string]any{"reference": "nginx"}, sentJSON(t, fake, http.MethodPost, mine+"/vms/"+vmWeb+"/images"))
	})

	t.Run("an image is pulled into the vm --vm names", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, everyone+"/vms/"+vmBox2, http.StatusOK, box2).
			On(http.MethodPost, everyone+"/vms/"+vmBox2+"/images", http.StatusCreated, workloadtest.Object{"kind": "image", "spec": workloadtest.Object{}})
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitSuccess, h.Run(newPullImageCommand(h.Cli), "-A", "--vm", vmBox2, "redis:7"))
		assert.Equal(t, "redis:7\n", h.Out.String())
	})

	t.Run("a docker vm has to be named when there is not just one", func(t *testing.T) {
		t.Parallel()

		for name, vms := range map[string]workloadtest.Object{
			"you have 2 Docker VMs: name the Docker VM with --vm":                                                workloadtest.Page(web, box2),
			`you have no Docker VM: make one with "tarhche vm create --docker", or name the Docker VM with --vm`: workloadtest.Page(box),
		} {
			fake := workloadtest.New().On(http.MethodGet, mine+"/vms", http.StatusOK, vms)
			h := harnessOf(t, fake)

			assert.Equal(t, console.ExitFailure, h.Run(newPullImageCommand(h.Cli), "nginx"), name)
			assert.Equal(t, "Error: "+name+"\n", h.Err.String())
		}

		h := harnessOf(t, workloadtest.New())
		assert.Equal(t, console.ExitFailure, h.Run(newPullImageCommand(h.Cli), "-A", "nginx"))
		assert.Equal(t, "Error: name the Docker VM with --vm: anybody's are not chosen from\n", h.Err.String())
	})

	t.Run("vms that cannot be listed, or a pull refused, is an error", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, mine+"/vms", http.StatusInternalServerError, `{"code":"internal"}`).
			On(http.MethodGet, mine+"/vms/"+vmWeb, http.StatusOK, web).
			On(http.MethodPost, mine+"/vms/"+vmWeb+"/images", http.StatusGatewayTimeout, `{"code":"timeout","message":"pulling took too long"}`)
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitFailure, h.Run(newPullImageCommand(h.Cli), "nginx"))
		assert.Equal(t, console.ExitFailure, h.Run(newPullImageCommand(h.Cli), "--vm", vmWeb, "nginx"))
		assert.Equal(t, "Error: internal\nError: pulling took too long\n", h.Err.String())
	})

	testRequiresArguments(t, newPullImageCommand, `"tarhche pull" requires exactly 1 argument.`)
}

func TestRemoveImagesCommand(t *testing.T) {
	t.Parallel()

	testAction(t, newRemoveImagesCommand, mine+"/vms/"+vmWeb+"/images", workloadtest.Items(nginxImage, appImage), http.MethodDelete,
		func(uuid string) string { return mine + "/vms/" + vmWeb + "/images/" + uuid },
		map[string]string{"nginx:1.27": "i-nginx", "55556666": "i-app"}, "--vm", vmWeb)

	t.Run("an image is looked for across one's docker vms, and removed by force", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, mine+"/vms", http.StatusOK, workloadtest.Page(web, box2)).
			On(http.MethodGet, mine+"/vms/"+vmWeb+"/images", http.StatusOK, workloadtest.Items(nginxImage)).
			On(http.MethodGet, mine+"/vms/"+vmBox2+"/images", http.StatusOK, workloadtest.Items(appImage)).
			On(http.MethodDelete, mine+"/vms/"+vmBox2+"/images/i-app", http.StatusNoContent, nil)
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitSuccess, h.Run(newRemoveImagesCommand(h.Cli), "-f", "ghcr.io/acme/app@sha256:abc"))

		sent, _ := fake.Sent(http.MethodDelete, mine+"/vms/"+vmBox2+"/images/i-app")
		assert.Equal(t, url.Values{"force": {"true"}}, sent.Query)
	})

	t.Run("vms that cannot be listed are an error", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().On(http.MethodGet, mine+"/vms", http.StatusUnauthorized, `{"code":"unauthorized"}`)
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitFailure, h.Run(newRemoveImagesCommand(h.Cli), "nginx"))
		assert.Equal(t, "Error: unauthorized\n", h.Err.String())
	})

	testRequiresArguments(t, newRemoveImagesCommand, `"tarhche rm" requires at least 1 argument.`)
}

func TestInspectImagesCommand(t *testing.T) {
	t.Parallel()

	t.Run("images are written as the api answered them", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, mine+"/vms", http.StatusOK, workloadtest.Page(web)).
			On(http.MethodGet, mine+"/vms/"+vmWeb+"/images", http.StatusOK, workloadtest.Items(nginxImage))
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitFailure, h.Run(newInspectImagesCommand(h.Cli), "-f", "{{.status.size}} {{.spec.reference}}", "nginx:1.27", "redis"))
		assert.Equal(t, "196083712 nginx:1.27\n", h.Out.String())
		assert.Equal(t, "Error: no such image: redis\n", h.Err.String())
	})

	t.Run("images that cannot be listed are an error", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, everyone+"/vms", http.StatusOK, workloadtest.Page(web)).
			On(http.MethodGet, everyone+"/vms/"+vmWeb+"/images", http.StatusForbidden, `{"code":"forbidden"}`)
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitFailure, h.Run(newInspectImagesCommand(h.Cli), "-A", "--vm", "web", "nginx"))
		assert.Equal(t, console.ExitFailure, h.Run(newInspectImagesCommand(h.Cli), "-A", "--vm", "nothing", "nginx"))
		assert.Equal(t, "Error: forbidden\nError: no such vm: nothing\n", h.Err.String())
	})

	testRequiresArguments(t, newInspectImagesCommand, `"tarhche inspect" requires at least 1 argument.`)
}

func TestListNetworksCommand(t *testing.T) {
	t.Parallel()

	t.Run("the networks of one's docker vms, by name", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, mine+"/vms", http.StatusOK, workloadtest.Page(web, box2)).
			On(http.MethodGet, mine+"/vms/"+vmWeb+"/networks", http.StatusOK, workloadtest.Items(bridge, backend)).
			On(http.MethodGet, mine+"/vms/"+vmBox2+"/networks", http.StatusOK, workloadtest.Items(workloadtest.Object{"kind": "network", "metadata": workloadtest.Object{"uuid": "n-new"}, "spec": workloadtest.Object{"name": "backend"}, "status": workloadtest.Object{"state": "pending"}}))
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitSuccess, h.Run(newListNetworksCommand(h.Cli)))
		assert.Equal(t, table([]string{"NETWORK ID", "NAME", "DRIVER", "SCOPE", "VM"},
			[]string{"n-new", "backend", "", "", "box2"},
			[]string{"999988887777", "backend", "bridge", "local", "web"},
			[]string{"123412341234", "bridge", "bridge", "local", "web"},
		), h.Out.String())
	})

	t.Run("ids alone, untruncated, and docker's template fields", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, everyone+"/vms/"+vmWeb, http.StatusOK, web).
			On(http.MethodGet, everyone+"/vms/"+vmWeb+"/networks", http.StatusOK, workloadtest.Items(bridge))
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitSuccess, h.Run(newListNetworksCommand(h.Cli), "-A", "--vm", vmWeb, "-q", "--no-trunc"))
		assert.Equal(t, console.ExitSuccess, h.Run(newListNetworksCommand(h.Cli), "-A", "--vm", vmWeb, "--format", `{{.Label "a"}} {{.Internal}}`))
		assert.Equal(t, "1234123412341234\nb false\n", h.Out.String())
	})

	t.Run("networks that cannot be listed are an error", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, mine+"/vms", http.StatusOK, workloadtest.Page(web)).
			On(http.MethodGet, mine+"/vms/"+vmWeb+"/networks", http.StatusInternalServerError, `{"code":"internal"}`)
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitFailure, h.Run(newListNetworksCommand(h.Cli)))
		assert.Equal(t, console.ExitFailure, h.Run(newListNetworksCommand(h.Cli), "--vm", "nothing"))
		assert.Equal(t, "Error: internal\nError: no such vm: nothing\n", h.Err.String())
	})

	testRequiresArguments(t, newListNetworksCommand, `"tarhche ls" accepts no arguments.`, "x")
}

func TestCreateNetworkCommand(t *testing.T) {
	t.Parallel()

	t.Run("a network is created in one's only docker vm", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, mine+"/vms", http.StatusOK, workloadtest.Page(web)).
			On(http.MethodPost, mine+"/vms/"+vmWeb+"/networks", http.StatusCreated, backend)
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitSuccess, h.Run(newCreateNetworkCommand(h.Cli), "-d", "bridge", "--internal", "--label", "team=web", "--label", "flag", "backend"))
		assert.Equal(t, "9999888877776666\n", h.Out.String())
		assert.Equal(t, map[string]any{"name": "backend", "driver": "bridge", "internal": true, "labels": map[string]any{"team": "web", "flag": ""}}, sentJSON(t, fake, http.MethodPost, mine+"/vms/"+vmWeb+"/networks"))
	})

	t.Run("a network not made yet is said by its uuid", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, everyone+"/vms/"+vmWeb, http.StatusOK, web).
			On(http.MethodPost, everyone+"/vms/"+vmWeb+"/networks", http.StatusCreated, workloadtest.Object{"metadata": workloadtest.Object{"uuid": "n-1"}})
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitSuccess, h.Run(newCreateNetworkCommand(h.Cli), "-A", "--vm", vmWeb, "backend"))
		assert.Equal(t, "n-1\n", h.Out.String())
		assert.Equal(t, map[string]any{"name": "backend"}, sentJSON(t, fake, http.MethodPost, everyone+"/vms/"+vmWeb+"/networks"))
	})

	t.Run("a vm that is not there, or a network refused, is an error", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, mine+"/vms", http.StatusOK, workloadtest.Page(web)).
			On(http.MethodPost, mine+"/vms/"+vmWeb+"/networks", http.StatusBadRequest, workloadtest.Refused(map[string]string{"name": "invalid name"}))
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitFailure, h.Run(newCreateNetworkCommand(h.Cli), "--vm", "nothing", "backend"))
		assert.Equal(t, console.ExitFailure, h.Run(newCreateNetworkCommand(h.Cli), "Bad!"))
		assert.Equal(t, "Error: no such vm: nothing\nError: name: invalid name\n", h.Err.String())
	})

	testRequiresArguments(t, newCreateNetworkCommand, `"tarhche create" requires exactly 1 argument.`)
}

func TestRemoveNetworksCommand(t *testing.T) {
	t.Parallel()

	testAction(t, newRemoveNetworksCommand, mine+"/vms/"+vmWeb+"/networks", workloadtest.Items(backend, bridge), http.MethodDelete,
		func(uuid string) string { return mine + "/vms/" + vmWeb + "/networks/" + uuid },
		map[string]string{"backend": "n-backend", "12341234": "n-bridge"}, "--vm", vmWeb)

	t.Run("what still needs a network is said", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, mine+"/vms", http.StatusOK, workloadtest.Page(web)).
			On(http.MethodGet, mine+"/vms/"+vmWeb+"/networks", http.StatusOK, workloadtest.Items(backend)).
			On(http.MethodDelete, mine+"/vms/"+vmWeb+"/networks/n-backend", http.StatusAccepted, `{"dependents":[{"kind":"stack","uuid":"s1","name":"blog","recreates":true},{"kind":"container","uuid":"c1"}],"recreated":true}`)
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitSuccess, h.Run(newRemoveNetworksCommand(h.Cli), "--cascade", "backend"))
		assert.Equal(t, "backend\n", h.Out.String())
		assert.Equal(t, "Warning: backend is needed by stack blog, container c1, which will make it again: delete them with it with --cascade.\n", h.Err.String())

		sent, _ := fake.Sent(http.MethodDelete, mine+"/vms/"+vmWeb+"/networks/n-backend")
		assert.Equal(t, url.Values{"cascade": {"true"}}, sent.Query)
	})

	t.Run("networks that cannot be listed are an error", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().On(http.MethodGet, mine+"/vms", http.StatusOK, workloadtest.Page())
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitFailure, h.Run(newRemoveNetworksCommand(h.Cli), "--vm", "web", "backend"))
		assert.Equal(t, "Error: no such vm: web\n", h.Err.String())
	})

	testRequiresArguments(t, newRemoveNetworksCommand, `"tarhche rm" requires at least 1 argument.`)
}

func TestInspectNetworksCommand(t *testing.T) {
	t.Parallel()

	t.Run("networks are written as the api answered them", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, mine+"/vms", http.StatusOK, workloadtest.Page(web)).
			On(http.MethodGet, mine+"/vms/"+vmWeb+"/networks", http.StatusOK, workloadtest.Items(backend))
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitSuccess, h.Run(newInspectNetworksCommand(h.Cli), "backend"))

		var written []map[string]any
		require.NoError(t, json.Unmarshal(h.Out.Bytes(), &written))
		assert.Equal(t, "network", written[0]["kind"])
	})

	t.Run("networks that cannot be listed are an error", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, everyone+"/vms", http.StatusOK, workloadtest.Page(web)).
			On(http.MethodGet, everyone+"/vms/"+vmWeb+"/networks", http.StatusForbidden, `{"code":"forbidden"}`)
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitFailure, h.Run(newInspectNetworksCommand(h.Cli), "-A", "backend"))
		assert.Equal(t, console.ExitFailure, h.Run(newInspectNetworksCommand(h.Cli), "-A", "--vm", "box", "backend"))
		assert.Equal(t, "Error: forbidden\nError: no such vm: box\n", h.Err.String())
	})

	testRequiresArguments(t, newInspectNetworksCommand, `"tarhche inspect" requires at least 1 argument.`)
}

func TestConnectNetworkCommand(t *testing.T) {
	t.Parallel()

	t.Run("a container is put on a network, named first as docker's network connect names it", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, mine+"/vms/"+vmWeb, http.StatusOK, web).
			On(http.MethodGet, mine+"/containers", http.StatusOK, workloadtest.Items(nginx)).
			On(http.MethodPost, mine+"/vms/"+vmWeb+"/containers/c-nginx/networks", http.StatusNoContent, nil)
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitSuccess, h.Run(newConnectNetworkCommand(h.Cli), "--vm", vmWeb, "--alias", "www", "backend", "nginx"))
		assert.Equal(t, map[string]any{"network": "backend", "aliases": []any{"www"}}, sentJSON(t, fake, http.MethodPost, mine+"/vms/"+vmWeb+"/containers/c-nginx/networks"))
	})

	testRequiresArguments(t, newConnectNetworkCommand, `"tarhche connect" requires exactly 2 arguments.`)
}

func TestDisconnectNetworkCommand(t *testing.T) {
	t.Parallel()

	t.Run("a container is taken off a network, named first", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, everyone+"/containers", http.StatusOK, workloadtest.Items(nginx)).
			On(http.MethodDelete, everyone+"/vms/"+vmWeb+"/containers/c-nginx/networks/backend", http.StatusBadRequest, workloadtest.Refused(map[string]string{"network": "not on it"}))
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitFailure, h.Run(newDisconnectNetworkCommand(h.Cli), "-A", "backend", "nginx"))
		assert.Equal(t, "Error: network: not on it\n", h.Err.String())
	})

	testRequiresArguments(t, newDisconnectNetworkCommand, `"tarhche disconnect" requires exactly 2 arguments.`, "a")
}

func TestListVolumesCommand(t *testing.T) {
	t.Parallel()

	t.Run("the volumes of one's docker vms, by name", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, mine+"/vms", http.StatusOK, workloadtest.Page(web, box2)).
			On(http.MethodGet, mine+"/vms/"+vmWeb+"/volumes", http.StatusOK, workloadtest.Items(data, workloadtest.Volume("o-cache", "cache").With("status.labels", workloadtest.Object{"x": "y"}))).
			On(http.MethodGet, mine+"/vms/"+vmBox2+"/volumes", http.StatusOK, workloadtest.Items(workloadtest.Object{"metadata": workloadtest.Object{"uuid": "o-new"}, "spec": workloadtest.Object{"name": "data"}, "status": workloadtest.Object{"state": "pending"}}))
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitSuccess, h.Run(newListVolumesCommand(h.Cli)))
		assert.Equal(t, table([]string{"DRIVER", "VOLUME NAME", "VM"},
			[]string{"local", "cache", "web"},
			[]string{"local", "data", "box2"},
			[]string{"local", "data", "web"},
		), h.Out.String())
	})

	t.Run("names alone, of anybody's vm, and docker's template fields", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, everyone+"/vms/"+vmWeb, http.StatusOK, web).
			On(http.MethodGet, everyone+"/vms/"+vmWeb+"/volumes", http.StatusOK, workloadtest.Items(data.With("spec.labels", workloadtest.Object{"x": "y"})))
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitSuccess, h.Run(newListVolumesCommand(h.Cli), "-A", "--vm", vmWeb, "-q"))
		assert.Equal(t, console.ExitSuccess, h.Run(newListVolumesCommand(h.Cli), "-A", "--vm", vmWeb, "--format", `{{.Label "x"}} {{.Mountpoint}}`))
		assert.Equal(t, "data\ny /var/lib/docker/volumes/data/_data\n", h.Out.String())
	})

	t.Run("volumes that cannot be listed are an error", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, mine+"/vms", http.StatusOK, workloadtest.Page(web)).
			On(http.MethodGet, mine+"/vms/"+vmWeb+"/volumes", http.StatusInternalServerError, `{"code":"internal"}`)
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitFailure, h.Run(newListVolumesCommand(h.Cli)))
		assert.Equal(t, console.ExitFailure, h.Run(newListVolumesCommand(h.Cli), "--vm", "box"))
		assert.Equal(t, "Error: internal\nError: no such vm: box\n", h.Err.String())
	})

	testRequiresArguments(t, newListVolumesCommand, `"tarhche ls" accepts no arguments.`, "x")
}

func TestCreateVolumeCommand(t *testing.T) {
	t.Parallel()

	t.Run("a volume is created in one's only docker vm", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, mine+"/vms", http.StatusOK, workloadtest.Page(web)).
			On(http.MethodPost, mine+"/vms/"+vmWeb+"/volumes", http.StatusCreated, data)
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitSuccess, h.Run(newCreateVolumeCommand(h.Cli), "-d", "local", "--label", "k=v", "data"))
		assert.Equal(t, "data\n", h.Out.String())
		assert.Equal(t, map[string]any{"name": "data", "driver": "local", "labels": map[string]any{"k": "v"}}, sentJSON(t, fake, http.MethodPost, mine+"/vms/"+vmWeb+"/volumes"))
	})

	t.Run("a volume given no name is given one", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, everyone+"/vms/"+vmWeb, http.StatusOK, web).
			On(http.MethodPost, everyone+"/vms/"+vmWeb+"/volumes", http.StatusCreated, workloadtest.Object{})
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitSuccess, h.Run(newCreateVolumeCommand(h.Cli), "-A", "--vm", vmWeb))

		name := sentJSON(t, fake, http.MethodPost, everyone+"/vms/"+vmWeb+"/volumes")["name"].(string)
		assert.Regexp(t, `^[a-z2-7]{26}$`, name)
		assert.Equal(t, name+"\n", h.Out.String())
	})

	t.Run("a vm that is not there, or a volume refused, is an error", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, mine+"/vms", http.StatusOK, workloadtest.Page(web)).
			On(http.MethodPost, mine+"/vms/"+vmWeb+"/volumes", http.StatusBadRequest, workloadtest.Refused(map[string]string{"driver": "invalid volume driver"}))
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitFailure, h.Run(newCreateVolumeCommand(h.Cli), "--vm", "box", "data"))
		assert.Equal(t, console.ExitFailure, h.Run(newCreateVolumeCommand(h.Cli), "-d", "nfs", "data"))
		assert.Equal(t, "Error: no such vm: box\nError: driver: invalid volume driver\n", h.Err.String())
	})

	testRequiresArguments(t, newCreateVolumeCommand, `"tarhche create" requires at most 1 argument.`, "a", "b")
}

func TestRemoveVolumesCommand(t *testing.T) {
	t.Parallel()

	testAction(t, newRemoveVolumesCommand, mine+"/vms/"+vmWeb+"/volumes", workloadtest.Items(data), http.MethodDelete,
		func(name string) string { return mine + "/vms/" + vmWeb + "/volumes/" + name },
		map[string]string{"o-data": "data"}, "--vm", vmWeb)

	t.Run("a volume is removed by force with what needs it, saying what still waits on it", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, mine+"/vms", http.StatusOK, workloadtest.Page(web)).
			On(http.MethodGet, mine+"/vms/"+vmWeb+"/volumes", http.StatusOK, workloadtest.Items(data)).
			On(http.MethodDelete, mine+"/vms/"+vmWeb+"/volumes/data", http.StatusAccepted, `{"dependents":[{"kind":"container","uuid":"c1","name":"db","recreates":false}],"recreated":false}`)
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitSuccess, h.Run(newRemoveVolumesCommand(h.Cli), "-f", "--cascade", "data"))
		assert.Equal(t, "Warning: data is needed by container db.\n", h.Err.String())

		sent, _ := fake.Sent(http.MethodDelete, mine+"/vms/"+vmWeb+"/volumes/data")
		assert.Equal(t, url.Values{"force": {"true"}, "cascade": {"true"}}, sent.Query)
	})

	t.Run("volumes that cannot be listed are an error", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().On(http.MethodGet, mine+"/vms", http.StatusOK, workloadtest.Page())
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitFailure, h.Run(newRemoveVolumesCommand(h.Cli), "--vm", "web", "data"))
		assert.Equal(t, "Error: no such vm: web\n", h.Err.String())
	})

	testRequiresArguments(t, newRemoveVolumesCommand, `"tarhche rm" requires at least 1 argument.`)
}

func TestInspectVolumesCommand(t *testing.T) {
	t.Parallel()

	t.Run("volumes are written as the api answered them", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, mine+"/vms", http.StatusOK, workloadtest.Page(web)).
			On(http.MethodGet, mine+"/vms/"+vmWeb+"/volumes", http.StatusOK, workloadtest.Items(data))
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitSuccess, h.Run(newInspectVolumesCommand(h.Cli), "-f", "{{.status.mountpoint}}", "data"))
		assert.Equal(t, "/var/lib/docker/volumes/data/_data\n", h.Out.String())
	})

	t.Run("volumes that cannot be listed are an error", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, everyone+"/vms", http.StatusOK, workloadtest.Page(web)).
			On(http.MethodGet, everyone+"/vms/"+vmWeb+"/volumes", http.StatusForbidden, `{"code":"forbidden"}`)
		h := harnessOf(t, fake)

		assert.Equal(t, console.ExitFailure, h.Run(newInspectVolumesCommand(h.Cli), "-A", "data"))
		assert.Equal(t, console.ExitFailure, h.Run(newInspectVolumesCommand(h.Cli), "-A", "--vm", "box", "data"))
		assert.Equal(t, "Error: forbidden\nError: no such vm: box\n", h.Err.String())
	})

	testRequiresArguments(t, newInspectVolumesCommand, `"tarhche inspect" requires at least 1 argument.`)
}

// splitLines are the lines of s.
func splitLines(s string) []string {
	return strings.Split(strings.TrimSuffix(s, "\n"), "\n")
}
