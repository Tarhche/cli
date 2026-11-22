package workload

import (
	"cmp"
	"context"
	"net/http"
	"strings"
	"time"
)

// ImageManifest is an image in a Docker VM, in its kind's manifest shape:
// one for every reference the image is held under.
type ImageManifest struct {
	Kind     string      `json:"kind"`
	Metadata Metadata    `json:"metadata"`
	Spec     ImageSpec   `json:"spec"`
	Status   ImageStatus `json:"status"`

	kept
}

// UnmarshalJSON reads an image, keeping it as it was answered.
func (m *ImageManifest) UnmarshalJSON(data []byte) error {
	type plain ImageManifest

	k, err := keep(data, (*plain)(m))
	m.kept = k

	return err
}

// ImageSpec is the reference an image was asked for by.
type ImageSpec struct {
	Reference string `json:"reference"`
}

// ImageStatus is what an image is doing: in its kind's words, and as its
// VM's dockerd last said, once it is pulled.
type ImageStatus struct {
	Status
	ImageDocker

	Failure *Failure `json:"failure,omitempty"`
}

// ImageDocker is an image as its VM's dockerd last had it. Its size is in
// bytes.
type ImageDocker struct {
	Reference string    `json:"reference,omitempty"`
	ID        string    `json:"id,omitempty"`
	Tags      []string  `json:"tags,omitempty"`
	Size      int64     `json:"size,omitempty"`
	CreatedAt time.Time `json:"created_at,omitzero"`
	InUse     bool      `json:"in_use,omitempty"`
}

// Reference is the reference the image is held under: as docker has it,
// else as it was asked for.
func (m ImageManifest) Reference() string {
	return cmp.Or(m.Status.Reference, m.Spec.Reference)
}

// References are every reference the image is known by, normalized.
func (m ImageManifest) References() []string {
	var references []string

	for _, reference := range append([]string{m.Spec.Reference, m.Status.Reference}, m.Status.Tags...) {
		if len(reference) > 0 && !strings.HasPrefix(reference, "<none>") {
			references = append(references, NormalizedReference(reference))
		}
	}

	return references
}

// Images are the images the Docker VM of vmUUID holds.
func (c *Client) Images(ctx context.Context, vmUUID string) ([]ImageManifest, error) {
	return items[ImageManifest](ctx, c, c.path("vms", vmUUID, "images"), nil)
}

// PullImage pulls reference into the Docker VM of vmUUID, and is the image
// once it is pulled.
func (c *Client) PullImage(ctx context.Context, vmUUID string, reference string) (ImageManifest, error) {
	var image ImageManifest
	if err := c.patient().post(ctx, c.path("vms", vmUUID, "images"), map[string]string{"reference": reference}, &image); err != nil {
		return ImageManifest{}, err
	}

	return image, nil
}

// DeleteImage removes the image id names, its uuid, its id or one of its
// tags, from the Docker VM of vmUUID: one a container uses only by force.
func (c *Client) DeleteImage(ctx context.Context, vmUUID string, id string, force bool) error {
	return c.act(ctx, http.MethodDelete, c.path("vms", vmUUID, "images", id), flags(map[string]bool{"force": force}), nil)
}

// NormalizedReference is an image reference as docker resolves it: by its
// tag, latest when it names none, or by its digest, with docker's own
// registry and library left out, so that nginx, nginx:latest and
// docker.io/library/nginx:latest are one.
func NormalizedReference(reference string) string {
	name, digest, digested := strings.Cut(strings.TrimSpace(reference), "@")

	tag := ""
	if i := strings.LastIndex(name, ":"); i > strings.LastIndex(name, "/") {
		name, tag = name[:i], name[i+1:]
	}

	for _, registry := range []string{"docker.io/", "index.docker.io/", "registry-1.docker.io/"} {
		if trimmed, found := strings.CutPrefix(name, registry); found {
			name = trimmed

			break
		}
	}

	name = strings.TrimPrefix(name, "library/")

	switch {
	case digested:
		return name + "@" + digest
	case len(tag) == 0:
		return name + ":latest"
	default:
		return name + ":" + tag
	}
}
