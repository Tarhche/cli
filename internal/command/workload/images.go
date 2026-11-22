package workload

import (
	"cmp"
	"slices"
	"strings"
	"time"

	"github.com/Tarhche/cli/internal/formatter"
	"github.com/Tarhche/cli/internal/workload"
)

// imageTable is docker's images, and the Docker VM each image is in.
const imageTable = `{{.Repository}}\t{{.Tag}}\t{{.ID}}\t{{.CreatedSince}}\t{{.Size}}\t{{.VM}}`

// imageHeaders name the columns docker names otherwise.
var imageHeaders = map[string]string{"ID": "IMAGE ID", "CreatedSince": "CREATED"}

// noneTag is what docker shows for a tag or a repository there is none of.
const noneTag = "<none>"

// imageRow is an image as docker's images lists one: once for every
// reference it is held under.
type imageRow struct {
	Repository   string
	Tag          string
	Digest       string
	ID           string
	UUID         string
	CreatedAt    time.Time
	CreatedSince string
	Size         string
	InUse        bool
	VM           string
	VMUUID       string
}

func imageRowOf(placed workload.Placed[workload.ImageManifest], untruncated bool) imageRow {
	image := placed.Item
	repository, tag, digest := splitReference(image.Reference())

	created := image.Status.CreatedAt
	if created.IsZero() {
		created = image.Metadata.CreatedAt
	}

	size := ""
	if image.Status.Size > 0 {
		size = formatter.Bytes(image.Status.Size)
	}

	return imageRow{
		Repository:   repository,
		Tag:          tag,
		Digest:       digest,
		ID:           imageIDOf(image, untruncated),
		UUID:         image.Metadata.UUID,
		CreatedAt:    created,
		CreatedSince: formatter.Ago(created),
		Size:         size,
		InUse:        image.Status.InUse,
		VM:           orDefault(placed.VM.Metadata.Name, placed.VM.Metadata.UUID),
		VMUUID:       placed.VM.Metadata.UUID,
	}
}

// imageRows are images as docker's images lists them, the newest first.
func imageRows(images []workload.Placed[workload.ImageManifest], untruncated bool) []imageRow {
	rows := make([]imageRow, len(images))
	for i, image := range images {
		rows[i] = imageRowOf(image, untruncated)
	}

	slices.SortStableFunc(rows, func(a, b imageRow) int {
		return cmp.Or(b.CreatedAt.Compare(a.CreatedAt), strings.Compare(a.Repository+":"+a.Tag, b.Repository+":"+b.Tag))
	})

	return rows
}

// imageIDOf is an image's id as docker shows it, without its algorithm, or
// its uuid until it is pulled.
func imageIDOf(image workload.ImageManifest, untruncated bool) string {
	id := strings.TrimPrefix(image.Status.ID, "sha256:")
	if len(id) == 0 {
		return image.Metadata.UUID
	}

	if untruncated {
		return image.Status.ID
	}

	return truncated(id, false)
}

// splitReference is the repository, the tag and the digest of an image's
// reference, as docker's images shows them.
func splitReference(reference string) (string, string, string) {
	if len(reference) == 0 {
		return noneTag, noneTag, ""
	}

	normalized := workload.NormalizedReference(reference)

	if name, digest, digested := strings.Cut(normalized, "@"); digested {
		return name, noneTag, digest
	}

	i := strings.LastIndex(normalized, ":")

	return normalized[:i], normalized[i+1:], ""
}
