package upgrade

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChecksumsOf(t *testing.T) {
	t.Parallel()

	t.Run("reads what sha256sum writes", func(t *testing.T) {
		t.Parallel()

		listing := "ABCDEF  tarhche_0.2.0_linux_amd64.tar.gz\n" +
			"012345 *tarhche_0.2.0_windows_amd64.zip\r\n" +
			"\n" +
			"not a line of checksums\n" +
			"6789ab  tarhche_0.2.0_darwin_app.zip"

		assert.Equal(t, map[string]string{
			"tarhche_0.2.0_linux_amd64.tar.gz": "abcdef",
			"tarhche_0.2.0_windows_amd64.zip":  "012345",
			"tarhche_0.2.0_darwin_app.zip":     "6789ab",
		}, checksumsOf(listing))
	})

	t.Run("lists nothing of nothing", func(t *testing.T) {
		t.Parallel()

		assert.Empty(t, checksumsOf(""))
	})
}
