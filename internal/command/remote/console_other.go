//go:build !windows

package remote

// virtualTerminal is nothing to do where every terminal understands a
// remote terminal's escape sequences.
func virtualTerminal(any) func() error {
	return func() error { return nil }
}
