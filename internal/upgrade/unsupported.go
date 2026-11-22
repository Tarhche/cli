//go:build nodesktop

package upgrade

// supported is ErrUnsupported: the image's build is upgraded by pulling a
// newer image.
func supported() error {
	return ErrUnsupported
}
