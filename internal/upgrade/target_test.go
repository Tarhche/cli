package upgrade

import (
	"path/filepath"
	"runtime/debug"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBundleOf(t *testing.T) {
	t.Parallel()

	tests := []struct {
		executable string
		bundle     string
	}{
		{executable: "/Applications/Tarhche.app/Contents/MacOS/tarhche", bundle: "/Applications/Tarhche.app"},
		{executable: "/Users/sarah/Apps/Tarhche 2.app/Contents/MacOS/tarhche", bundle: "/Users/sarah/Apps/Tarhche 2.app"},
		{executable: "/usr/local/bin/tarhche"},
		{executable: "/Applications/Tarhche.app/Contents/Resources/tarhche"},
		{executable: "/Applications/Tarhche.app/MacOS/tarhche"},
		{executable: "/Applications/Tarhche/Contents/MacOS/tarhche"},
	}

	for _, test := range tests {
		t.Run(test.executable, func(t *testing.T) {
			t.Parallel()

			bundle, isBundled := bundleOf(filepath.FromSlash(test.executable))

			assert.Equal(t, len(test.bundle) > 0, isBundled)
			assert.Equal(t, filepath.FromSlash(test.bundle), bundle)
		})
	}
}

func TestArchiveName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		platform Platform
		name     string
	}{
		{platform: Platform{OS: "darwin", Arch: "amd64"}, name: "tarhche_0.2.0_darwin_amd64.tar.gz"},
		{platform: Platform{OS: "darwin", Arch: "arm64"}, name: "tarhche_0.2.0_darwin_arm64.tar.gz"},
		{platform: Platform{OS: "linux", Arch: "386"}, name: "tarhche_0.2.0_linux_386.tar.gz"},
		{platform: Platform{OS: "linux", Arch: "amd64"}, name: "tarhche_0.2.0_linux_amd64.tar.gz"},
		{platform: Platform{OS: "linux", Arch: "arm", Arm: "7"}, name: "tarhche_0.2.0_linux_armv7.tar.gz"},
		{platform: Platform{OS: "linux", Arch: "arm64"}, name: "tarhche_0.2.0_linux_arm64.tar.gz"},
		{platform: Platform{OS: "windows", Arch: "386"}, name: "tarhche_0.2.0_windows_386.zip"},
		{platform: Platform{OS: "windows", Arch: "amd64"}, name: "tarhche_0.2.0_windows_amd64.zip"},
		{platform: Platform{OS: "windows", Arch: "arm64"}, name: "tarhche_0.2.0_windows_arm64.zip"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.name, archiveName("0.2.0", test.platform))
		})
	}

	t.Run("names the app bundle's archive and the checksums", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "tarhche_0.2.0_darwin_app.zip", appArchiveName("0.2.0"))
		assert.Equal(t, "tarhche_0.2.0_checksums.txt", checksumsName("0.2.0"))
	})
}

func TestPlatformOf(t *testing.T) {
	t.Parallel()

	settings := func(values ...string) *debug.BuildInfo {
		info := &debug.BuildInfo{}
		for i := 0; i < len(values); i += 2 {
			info.Settings = append(info.Settings, debug.BuildSetting{Key: values[i], Value: values[i+1]})
		}

		return info
	}

	tests := []struct {
		name     string
		goarch   string
		info     *debug.BuildInfo
		platform Platform
	}{
		{name: "takes no GOARM beside arm", goarch: "amd64", info: settings("GOARM", "7"), platform: Platform{OS: "linux", Arch: "amd64"}},
		{name: "reads the GOARM arm was built for", goarch: "arm", info: settings("GOOS", "linux", "GOARM", "6"), platform: Platform{OS: "linux", Arch: "arm", Arm: "6"}},
		{name: "leaves out how arm does floating point", goarch: "arm", info: settings("GOARM", "7,softfloat"), platform: Platform{OS: "linux", Arch: "arm", Arm: "7"}},
		{name: "takes 7 when the build says none", goarch: "arm", info: settings("GOOS", "linux"), platform: Platform{OS: "linux", Arch: "arm", Arm: "7"}},
		{name: "takes 7 when there is no build to read", goarch: "arm", platform: Platform{OS: "linux", Arch: "arm", Arm: "7"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.platform, platformOf("linux", test.goarch, test.info))
		})
	}
}

func TestPlatform(t *testing.T) {
	t.Parallel()

	t.Run("is said as Go says it, arm with its version", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "linux/amd64", Platform{OS: "linux", Arch: "amd64"}.name())
		assert.Equal(t, "linux/armv7", Platform{OS: "linux", Arch: "arm", Arm: "7"}.name())
		assert.Equal(t, "linux/arm", Platform{OS: "linux", Arch: "arm"}.name())
	})

	t.Run("names its binary", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "tarhche.exe", Platform{OS: "windows", Arch: "amd64"}.binary())
		assert.Equal(t, "tarhche", Platform{OS: "darwin", Arch: "arm64"}.binary())
	})
}
