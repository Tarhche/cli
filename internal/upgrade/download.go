package upgrade

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"
)

// maxChecksums is the most a release's file of checksums may be: a line for
// each of its files.
const maxChecksums = 1 << 20

// stagingPattern is what the files and directories an upgrade stages what it
// installs in are called, beside what they replace.
const stagingPattern = ".tarhche-upgrade-*"

// assetNamed is the asset of assets called name.
func assetNamed(assets []Asset, name string) (Asset, bool) {
	for _, asset := range assets {
		if asset.Name == name {
			return asset, true
		}
	}

	return Asset{}, false
}

// checksum is the SHA-256 the checksums of release say its file called name
// has.
func (u *Upgrader) checksum(ctx context.Context, release Release, name string) (string, error) {
	listing, found := assetNamed(release.Assets, checksumsName(release.Version))
	if !found {
		return "", fmt.Errorf("%w: the release lists no checksums", ErrChecksum)
	}

	var checksums strings.Builder
	if err := u.fetch(ctx, listing, &checksums, maxChecksums, nil); err != nil {
		return "", err
	}

	sum, found := checksumsOf(checksums.String())[name]
	if !found {
		return "", fmt.Errorf("%w: its checksums list no %s", ErrChecksum, name)
	}

	return sum, nil
}

// checksumsOf are the SHA-256 sums a file of checksums lists, as sha256sum
// writes them, by the name of the file each is of.
func checksumsOf(listing string) map[string]string {
	sums := map[string]string{}

	for line := range strings.Lines(listing) {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}

		sums[strings.TrimPrefix(fields[1], "*")] = strings.ToLower(fields[0])
	}

	return sums
}

// download writes asset to a file of its own beside what target replaces,
// telling progress how much of it arrived, and is that file's path once
// what arrived is what sum says it is.
func (u *Upgrader) download(ctx context.Context, asset Asset, sum string, target target, progress Progress) (string, error) {
	file, err := u.files.createTemp(filepath.Dir(target.path), stagingPattern)
	if err != nil {
		return "", denied(target.path, err)
	}

	path := file.Name()
	digest := sha256.New()

	err = u.fetch(ctx, asset, io.MultiWriter(file, digest), u.limit, progress)
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}

	if err == nil {
		err = matches(asset.Name, digest, sum)
	}

	if err != nil {
		_ = u.files.remove(path)

		return "", err
	}

	return path, nil
}

// matches is ErrChecksum unless digest, of what was downloaded of name, is
// sum.
func matches(name string, digest hash.Hash, sum string) error {
	got := hex.EncodeToString(digest.Sum(nil))
	if !strings.EqualFold(got, sum) {
		return fmt.Errorf("%w: %s's SHA-256 is %s, its checksums say %s", ErrChecksum, name, got, sum)
	}

	return nil
}

// fetch writes asset to w, telling progress (when there is one) how much of
// it arrived; an asset larger than limit is refused.
func (u *Upgrader) fetch(ctx context.Context, asset Asset, w io.Writer, limit int64, progress Progress) error {
	header := http.Header{}
	header.Set("Accept", "application/octet-stream")

	response, err := u.get(ctx, asset.URL, header)
	if err != nil {
		return fmt.Errorf("downloading %s: %w", asset.Name, err)
	}

	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("downloading %s: GitHub answered %s", asset.Name, response.Status)
	}

	total := response.ContentLength
	if total <= 0 {
		total = max(asset.Size, 0)
	}

	if total > limit {
		return tooLarge(asset.Name)
	}

	if progress != nil {
		w = io.MultiWriter(w, &counter{total: total, progress: progress})
	}

	written, err := io.Copy(w, io.LimitReader(response.Body, limit+1))
	if err != nil {
		return fmt.Errorf("downloading %s: %w", asset.Name, err)
	}

	if written > limit {
		return tooLarge(asset.Name)
	}

	return nil
}

// tooLarge is what what is called name, larger than an archive of tarhche
// may be, is refused with.
func tooLarge(name string) error {
	return fmt.Errorf("%s is larger than a build of tarhche may be", name)
}

// counter tells progress how much was written through it, of total.
type counter struct {
	done     int64
	total    int64
	progress Progress
}

func (c *counter) Write(p []byte) (int, error) {
	c.done += int64(len(p))
	c.progress(c.done, c.total)

	return len(p), nil
}

// denied is err, of replacing path: ErrPermission when the running user may
// not write where path is, and nil for none.
func denied(path string, err error) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, fs.ErrPermission) {
		return fmt.Errorf("cannot replace %s: %w", path, ErrPermission)
	}

	return fmt.Errorf("replacing %s: %w", path, err)
}
