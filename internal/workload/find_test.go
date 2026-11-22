package workload

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tarhche/cli/internal/api"
	"github.com/Tarhche/cli/internal/workload/workloadtest"
)

const (
	uuidOfWeb = "0193a5b2-7c41-7000-8000-000000000001"
	uuidOfBox = "0193a5b2-7c41-7000-8000-000000000002"
)

func vms() workloadtest.Object {
	return workloadtest.Page(
		workloadtest.VM(uuidOfWeb, "web", true, "running"),
		workloadtest.VM(uuidOfBox, "box", false, "stopped"),
		workloadtest.VM("0193a5b2-7c41-7000-8000-000000000003", "twin", false, "running"),
		workloadtest.VM("0193a5b2-7c41-7000-8000-000000000004", "twin", true, "running"),
	)
}

func TestClientFindVM(t *testing.T) {
	t.Parallel()

	t.Run("a uuid is asked for as it is, and nothing is listed", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().On(http.MethodGet, mine+"/vms/"+uuidOfWeb, http.StatusOK, workloadtest.VM(uuidOfWeb, "web", true, "running"))

		vm, err := clientOf(t, fake, Mine).FindVM(context.Background(), uuidOfWeb)

		require.NoError(t, err)
		assert.Equal(t, "web", vm.Metadata.Name)
		assert.Equal(t, []string{"GET " + mine + "/vms/" + uuidOfWeb}, fake.Routes())
	})

	t.Run("a uuid that is no vm's is looked for among their names", func(t *testing.T) {
		t.Parallel()

		other := "0193a5b2-7c41-7000-8000-0000000000ff"
		fake := workloadtest.New().On(http.MethodGet, everyone+"/vms", http.StatusOK, workloadtest.Page(workloadtest.VM(uuidOfWeb, other, true, "running")))

		vm, err := clientOf(t, fake, Everyone).FindVM(context.Background(), other)

		require.NoError(t, err)
		assert.Equal(t, uuidOfWeb, vm.Metadata.UUID)
		assert.Equal(t, []string{"GET " + everyone + "/vms/" + other, "GET " + everyone + "/vms"}, fake.Routes())
	})

	t.Run("a vm is named by its slug before its name", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().On(http.MethodGet, mine+"/vms", http.StatusOK, workloadtest.Page(
			workloadtest.VM(uuidOfWeb, "box-x1y2", true, "running"),
			workloadtest.VM(uuidOfBox, "box", false, "running"),
		))
		c := clientOf(t, fake, Mine)

		vm, err := c.FindVM(context.Background(), "box-x1y2")
		require.NoError(t, err)
		assert.Equal(t, uuidOfBox, vm.Metadata.UUID, "the slug of box, not the name of web")

		vm, err = c.FindVM(context.Background(), "box")
		require.NoError(t, err)
		assert.Equal(t, uuidOfBox, vm.Metadata.UUID)
	})

	t.Run("a name two vms have names neither", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().On(http.MethodGet, mine+"/vms", http.StatusOK, vms())

		_, err := clientOf(t, fake, Mine).FindVM(context.Background(), "twin")

		require.ErrorIs(t, err, ErrAmbiguous)
		assert.EqualError(t, err, `"twin" names more than one vm: twin (0193a5b2-7c41-7000-8000-000000000003), twin (0193a5b2-7c41-7000-8000-000000000004)`)
	})

	t.Run("what names no vm says what was looked for", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().On(http.MethodGet, mine+"/vms", http.StatusOK, vms())
		c := clientOf(t, fake, Mine)

		_, err := c.FindVM(context.Background(), "nothing")
		require.ErrorIs(t, err, ErrNotFound)
		assert.EqualError(t, err, "no such vm: nothing")

		var notFound *NotFoundError
		require.ErrorAs(t, err, &notFound)
		assert.Equal(t, "vm", notFound.Kind)

		_, err = c.FindVM(context.Background(), "")
		assert.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("what the api answers otherwise is the answer", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, mine+"/vms/"+uuidOfWeb, http.StatusForbidden, `{"code":"forbidden"}`).
			On(http.MethodGet, mine+"/vms", http.StatusInternalServerError, `{"code":"internal"}`)
		c := clientOf(t, fake, Mine)

		_, err := c.FindVM(context.Background(), uuidOfWeb)
		assert.True(t, api.IsForbidden(err))

		_, err = c.FindVM(context.Background(), "web")
		assert.EqualError(t, err, "internal")
	})
}

func TestClientFindContainer(t *testing.T) {
	t.Parallel()

	containers := workloadtest.Items(
		workloadtest.Container("c1", "abcdef0000000001", "nginx", "running", uuidOfWeb, "web"),
		workloadtest.Container("c2", "abcdef0000000002", "redis", "running", uuidOfWeb, "web"),
		workloadtest.Container("c3", "123456789abc", "api", "stopped", uuidOfBox, "box"),
		workloadtest.Container("c4", "fedcba987654", "api", "running", uuidOfWeb, "web"),
		workloadtest.Container("c5", "", "pending", "pending", uuidOfWeb, "web").With("status.name", ""),
	)

	cases := map[string]string{
		"c2":               "c2",
		"abcdef0000000001": "c1",
		"redis":            "c2",
		"pending":          "c5",
		"1234":             "c3",
		"fedcba98":         "c4",
	}

	for reference, want := range cases {
		t.Run("finds "+reference, func(t *testing.T) {
			t.Parallel()

			fake := workloadtest.New().On(http.MethodGet, mine+"/containers", http.StatusOK, containers)

			container, err := clientOf(t, fake, Mine).FindContainer(context.Background(), "", reference)

			require.NoError(t, err)
			assert.Equal(t, want, container.Metadata.UUID)
		})
	}

	t.Run("a name in two vms, or a prefix of two ids, names neither", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().On(http.MethodGet, mine+"/containers", http.StatusOK, containers)
		c := clientOf(t, fake, Mine)

		_, err := c.FindContainer(context.Background(), "", "api")
		assert.EqualError(t, err, `"api" names more than one container: api (c3) in vm box (`+uuidOfBox+`), api (c4) in vm web (`+uuidOfWeb+`)`)

		_, err = c.FindContainer(context.Background(), "", "abcdef")
		assert.ErrorIs(t, err, ErrAmbiguous)
	})

	t.Run("a prefix of fewer than four characters names nothing", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().On(http.MethodGet, mine+"/containers", http.StatusOK, containers)

		_, err := clientOf(t, fake, Mine).FindContainer(context.Background(), "", "123")

		assert.EqualError(t, err, "no such container: 123")
	})

	t.Run("a container is looked for in one vm", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().On(http.MethodGet, everyone+"/containers", http.StatusOK, workloadtest.Items())

		_, err := clientOf(t, fake, Everyone).FindContainer(context.Background(), uuidOfWeb, "nginx")

		assert.ErrorIs(t, err, ErrNotFound)

		sent, _ := fake.Sent(http.MethodGet, everyone+"/containers")
		assert.Equal(t, uuidOfWeb, sent.Query.Get("vm"))
	})

	t.Run("what the api answers otherwise is the answer", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().On(http.MethodGet, mine+"/containers", http.StatusServiceUnavailable, `{"code":"unavailable"}`)

		_, err := clientOf(t, fake, Mine).FindContainer(context.Background(), "", "nginx")

		assert.EqualError(t, err, "unavailable")
	})
}

func TestClientFindInVMs(t *testing.T) {
	t.Parallel()

	web := VMManifest{Metadata: Metadata{UUID: uuidOfWeb, Name: "web"}, Docker: true}
	box := VMManifest{Metadata: Metadata{UUID: uuidOfBox, Name: "box"}, Docker: true}

	fake := func() *workloadtest.API {
		return workloadtest.New().
			On(http.MethodGet, mine+"/vms/"+uuidOfWeb+"/images", http.StatusOK, workloadtest.Items(
				workloadtest.Image("i1", "sha256:aaaa1111bbbb2222", "nginx:latest"),
				workloadtest.Image("i2", "sha256:aaaa1111bbbb2222", "nginx:1.27"),
				workloadtest.Image("i3", "sha256:cccc3333dddd4444", "ghcr.io/acme/app:2"),
			)).
			On(http.MethodGet, mine+"/vms/"+uuidOfBox+"/images", http.StatusOK, workloadtest.Items(
				workloadtest.Image("i4", "sha256:eeee5555ffff6666", "nginx:latest"),
			)).
			On(http.MethodGet, mine+"/vms/"+uuidOfWeb+"/networks", http.StatusOK, workloadtest.Items(
				workloadtest.Network("n1", "f00dbabe0001", "backend"),
			)).
			On(http.MethodGet, mine+"/vms/"+uuidOfBox+"/networks", http.StatusNotFound, `{"code":"not_found"}`).
			On(http.MethodGet, mine+"/vms/"+uuidOfWeb+"/volumes", http.StatusOK, workloadtest.Items(
				workloadtest.Volume("o1", "data"),
			)).
			On(http.MethodGet, mine+"/vms/"+uuidOfBox+"/volumes", http.StatusOK, workloadtest.Items(
				workloadtest.Volume("o2", "data"),
			))
	}

	t.Run("an image is named by its uuid, its id, a reference or the beginning of its id", func(t *testing.T) {
		t.Parallel()

		c := clientOf(t, fake(), Mine)

		for reference, want := range map[string]string{
			"i3":                           "i3",
			"sha256:cccc3333dddd4444":      "i3",
			"cccc3333dddd4444":             "i3",
			"ghcr.io/acme/app:2":           "i3",
			"nginx:1.27":                   "i2",
			"docker.io/library/nginx:1.27": "i2",
			"eeee":                         "i4",
			"sha256:eeee5555":              "i4",
		} {
			found, err := c.FindImage(context.Background(), []VMManifest{web, box}, reference)

			require.NoError(t, err, reference)
			assert.Equal(t, want, found.Item.Metadata.UUID, reference)
		}
	})

	t.Run("the references of one image name it once, but one image in two vms is two", func(t *testing.T) {
		t.Parallel()

		c := clientOf(t, fake(), Mine)

		found, err := c.FindImage(context.Background(), []VMManifest{web}, "aaaa1111")
		require.NoError(t, err)
		assert.Equal(t, "i1", found.Item.Metadata.UUID)
		assert.Equal(t, uuidOfWeb, found.VM.Metadata.UUID)

		_, err = c.FindImage(context.Background(), []VMManifest{web, box}, "nginx")
		assert.EqualError(t, err, `"nginx" names more than one image: nginx:latest (aaaa1111bbbb) in vm web (`+uuidOfWeb+`), nginx:latest (eeee5555ffff) in vm box (`+uuidOfBox+`)`)
	})

	t.Run("a network is named by its uuid, id, name or the beginning of its id, and a vm gone holds none", func(t *testing.T) {
		t.Parallel()

		c := clientOf(t, fake(), Mine)

		for _, reference := range []string{"n1", "f00dbabe0001", "backend", "f00d"} {
			found, err := c.FindNetwork(context.Background(), []VMManifest{web, box}, reference)

			require.NoError(t, err, reference)
			assert.Equal(t, "n1", found.Item.Metadata.UUID)
		}

		_, err := c.FindNetwork(context.Background(), []VMManifest{web, box}, "frontend")
		assert.EqualError(t, err, "no such network: frontend")
	})

	t.Run("a volume is named by its uuid or its name", func(t *testing.T) {
		t.Parallel()

		c := clientOf(t, fake(), Mine)

		found, err := c.FindVolume(context.Background(), []VMManifest{web, box}, "o2")
		require.NoError(t, err)
		assert.Equal(t, uuidOfBox, found.VM.Metadata.UUID)

		found, err = c.FindVolume(context.Background(), []VMManifest{box}, "data")
		require.NoError(t, err)
		assert.Equal(t, "o2", found.Item.Metadata.UUID)

		_, err = c.FindVolume(context.Background(), []VMManifest{web, box}, "data")
		assert.ErrorIs(t, err, ErrAmbiguous)
	})

	t.Run("what lives in vms is listed in their order", func(t *testing.T) {
		t.Parallel()

		c := clientOf(t, fake(), Mine)

		images, err := c.ImagesIn(context.Background(), []VMManifest{box, web})
		require.NoError(t, err)

		uuids := make([]string, len(images))
		for i, image := range images {
			uuids[i] = image.Item.Metadata.UUID
		}

		assert.Equal(t, []string{"i4", "i1", "i2", "i3"}, uuids)

		images, err = c.ImagesIn(context.Background(), nil)
		require.NoError(t, err)
		assert.Empty(t, images)
	})

	t.Run("a vm that cannot be listed fails the listing", func(t *testing.T) {
		t.Parallel()

		failing := workloadtest.New().
			On(http.MethodGet, mine+"/vms/"+uuidOfWeb+"/volumes", http.StatusOK, workloadtest.Items()).
			On(http.MethodGet, mine+"/vms/"+uuidOfBox+"/volumes", http.StatusInternalServerError, `{"code":"internal"}`).
			On(http.MethodGet, mine+"/vms/"+uuidOfBox+"/images", http.StatusInternalServerError, `{"code":"internal"}`).
			On(http.MethodGet, mine+"/vms/"+uuidOfBox+"/networks", http.StatusInternalServerError, `{"code":"internal"}`)
		c := clientOf(t, failing, Mine)

		_, err := c.FindVolume(context.Background(), []VMManifest{web, box}, "data")
		assert.EqualError(t, err, "internal")

		_, err = c.FindImage(context.Background(), []VMManifest{box}, "nginx")
		assert.EqualError(t, err, "internal")

		_, err = c.FindNetwork(context.Background(), []VMManifest{box}, "backend")
		assert.EqualError(t, err, "internal")
	})
}

func TestClientFindStackAndSnapshot(t *testing.T) {
	t.Parallel()

	stackUUID := "0193a5b2-7c41-7000-8000-00000000000a"
	snapshotUUID := "0193a5b2-7c41-7000-8000-00000000000b"

	t.Run("a stack is named by its uuid, slug or name", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, mine+"/stacks/"+stackUUID, http.StatusOK, workloadtest.Stack(stackUUID, "blog", uuidOfWeb, "web")).
			On(http.MethodGet, mine+"/stacks", http.StatusOK, workloadtest.Page(workloadtest.Stack(stackUUID, "blog", uuidOfWeb, "web")))
		c := clientOf(t, fake, Mine)

		for _, reference := range []string{stackUUID, "blog-a1b2", "blog"} {
			stack, err := c.FindStack(context.Background(), reference)

			require.NoError(t, err, reference)
			assert.Equal(t, stackUUID, stack.Metadata.UUID)
		}

		_, err := c.FindStack(context.Background(), "shop")
		assert.EqualError(t, err, "no such stack: shop")
	})

	t.Run("a stack's uuid that is not there is looked for by name", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().On(http.MethodGet, mine+"/stacks", http.StatusInternalServerError, `{"code":"internal"}`)

		_, err := clientOf(t, fake, Mine).FindStack(context.Background(), stackUUID)

		assert.EqualError(t, err, "internal")
	})

	t.Run("a snapshot is named by its uuid or its name", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, everyone+"/snapshots/"+snapshotUUID, http.StatusOK, workloadtest.Snapshot(snapshotUUID, "before", uuidOfWeb, "web")).
			On(http.MethodGet, everyone+"/snapshots", http.StatusOK, workloadtest.Page(workloadtest.Snapshot(snapshotUUID, "before", uuidOfWeb, "web")))
		c := clientOf(t, fake, Everyone)

		for _, reference := range []string{snapshotUUID, "before"} {
			snapshot, err := c.FindSnapshot(context.Background(), reference)

			require.NoError(t, err, reference)
			assert.Equal(t, snapshotUUID, snapshot.Metadata.UUID)
		}

		_, err := c.FindSnapshot(context.Background(), "after")
		assert.EqualError(t, err, "no such snapshot: after")
	})

	t.Run("a snapshot that cannot be listed is the answer", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().On(http.MethodGet, mine+"/snapshots", http.StatusInternalServerError, `{"code":"internal"}`)

		_, err := clientOf(t, fake, Mine).FindSnapshot(context.Background(), "before")

		assert.EqualError(t, err, "internal")
	})
}

func TestIsUUID(t *testing.T) {
	t.Parallel()

	assert.True(t, IsUUID("0193a5b2-7c41-7000-8000-000000000001"))
	assert.True(t, IsUUID("0193A5B2-7C41-7000-8000-00000000000F"))

	for _, s := range []string{"", "web", "0193a5b2-7c41-7000-8000-00000000000", "0193a5b2x7c41-7000-8000-000000000001", "0193a5b2-7c41-7000-8000-00000000000g"} {
		assert.False(t, IsUUID(s), s)
	}
}

func TestDescriptions(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "web (v1)", named("web", "v1"))
	assert.Equal(t, "v1", named("", "v1"), "what has no name is told by its id")

	assert.Equal(t, "0123456789ab", shortened("0123456789abcdef", "u1"))
	assert.Equal(t, "abc", shortened("abc", "u1"))
	assert.Equal(t, "u1", shortened("", "u1"), "what docker has no id for yet is told by its uuid")
}

func TestErrors(t *testing.T) {
	t.Parallel()

	notFound := &NotFoundError{Kind: "image", Reference: "nginx"}
	assert.EqualError(t, notFound, "no such image: nginx")
	assert.True(t, errors.Is(notFound, ErrNotFound))
	assert.False(t, errors.Is(notFound, ErrAmbiguous))

	ambiguous := &AmbiguousError{Kind: "vm", Reference: "web", Matches: []string{"a", "b"}}
	assert.EqualError(t, ambiguous, `"web" names more than one vm: a, b`)
	assert.True(t, errors.Is(ambiguous, ErrAmbiguous))
}

func TestResolver(t *testing.T) {
	t.Parallel()

	t.Run("a vm is resolved by its name among one's own", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().On(http.MethodGet, mine+"/vms", http.StatusOK, vms())
		server := clientOf(t, fake, Mine)

		vm, err := NewResolver(server.api).VM(context.Background(), "web")

		require.NoError(t, err)
		assert.Equal(t, VM{UUID: uuidOfWeb, Name: "web", Slug: "web-x1y2", Docker: true, State: "running", Ports: []uint{8080}}, vm)
	})

	t.Run("a vm that is not there is an error", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().On(http.MethodGet, mine+"/vms", http.StatusOK, vms())

		_, err := NewResolver(clientOf(t, fake, Mine).api).VM(context.Background(), "nothing")

		assert.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("a container is resolved with its vm filled in", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().
			On(http.MethodGet, everyone+"/containers", http.StatusOK, workloadtest.Items(workloadtest.Container("c1", "abcdef123456", "nginx", "running", uuidOfWeb, "web"))).
			On(http.MethodGet, everyone+"/vms/"+uuidOfWeb, http.StatusOK, workloadtest.VM(uuidOfWeb, "web", true, "running"))

		container, err := clientOf(t, fake, Everyone).Resolver().Container(context.Background(), "abcdef")

		require.NoError(t, err)
		assert.Equal(t, Container{
			UUID: "c1", DockerID: "abcdef123456", Name: "nginx", State: "running",
			VM: VM{UUID: uuidOfWeb, Name: "web", Slug: "web-x1y2", Docker: true, State: "running", Ports: []uint{8080}},
		}, container)
		assert.Equal(t, []string{"GET " + everyone + "/containers", "GET " + everyone + "/vms/" + uuidOfWeb}, fake.Routes())
	})

	t.Run("a container that is not there, or whose vm is gone, is an error", func(t *testing.T) {
		t.Parallel()

		fake := workloadtest.New().On(http.MethodGet, mine+"/containers", http.StatusOK, workloadtest.Items(workloadtest.Container("c1", "abcdef123456", "nginx", "running", uuidOfWeb, "web")))
		resolver := NewResolver(clientOf(t, fake, Mine).api)

		_, err := resolver.Container(context.Background(), "redis")
		assert.EqualError(t, err, "no such container: redis")

		_, err = resolver.Container(context.Background(), "nginx")
		assert.True(t, api.IsNotFound(err))
		assert.ErrorContains(t, err, "the vm of container nginx")
	})
}

func TestMatch(t *testing.T) {
	t.Parallel()

	web := VMManifest{Metadata: Metadata{UUID: "v1", Name: "web"}}
	box := VMManifest{Metadata: Metadata{UUID: "v2"}}

	t.Run("what a name names twice is told apart by what makes each one", func(t *testing.T) {
		t.Parallel()

		_, err := MatchNetwork([]Placed[NetworkManifest]{
			{VM: web, Item: NetworkManifest{Metadata: Metadata{UUID: "n1"}, Spec: NetworkSpec{Name: "backend"}}},
			{VM: box, Item: NetworkManifest{Metadata: Metadata{UUID: "n2"}, Spec: NetworkSpec{Name: "backend"}}},
		}, "backend")
		assert.EqualError(t, err, `"backend" names more than one network: backend (n1) in vm web (v1), backend (n2) in vm v2`)

		_, err = MatchStack([]StackManifest{{Metadata: Metadata{UUID: "s1", Name: "blog"}}, {Metadata: Metadata{UUID: "s2", Name: "blog"}}}, "blog")
		assert.EqualError(t, err, `"blog" names more than one stack: blog (s1), blog (s2)`)

		_, err = MatchSnapshot([]SnapshotManifest{{Metadata: Metadata{UUID: "p1", Name: "x"}}, {Metadata: Metadata{UUID: "p2", Name: "x"}}}, "x")
		assert.EqualError(t, err, `"x" names more than one snapshot: x (p1), x (p2)`)
	})

	t.Run("images docker has not pulled yet are told apart by their uuids", func(t *testing.T) {
		t.Parallel()

		pending := func(uuid string) Placed[ImageManifest] {
			return Placed[ImageManifest]{VM: web, Item: ImageManifest{Metadata: Metadata{UUID: uuid}, Spec: ImageSpec{Reference: "redis"}}}
		}

		_, err := MatchImage([]Placed[ImageManifest]{pending("i1"), pending("i2")}, "redis:latest")
		assert.EqualError(t, err, `"redis:latest" names more than one image: redis (i1) in vm web (v1), redis (i2) in vm web (v1)`)

		found, err := MatchImage([]Placed[ImageManifest]{pending("i1")}, "redis")
		require.NoError(t, err)
		assert.Equal(t, "i1", found.Item.Metadata.UUID)
	})
}
