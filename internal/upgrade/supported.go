//go:build !nodesktop

package upgrade

// supported is nil: a release's build upgrades itself.
func supported() error {
	return nil
}
