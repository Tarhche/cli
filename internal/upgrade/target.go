package upgrade

import (
	"fmt"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
)

// defaultArm is the GOARM a build for arm is released for when its build
// does not say.
const defaultArm = "7"

// appBundle is what the macOS app bundle a release holds is called.
const appBundle = "Tarhche.app"

// target is what installing a release replaces.
type target struct {
	// path is the binary, or the app bundle it is run from.
	path string
	// executable is the binary, symlinks resolved.
	executable string
	// bundle reports whether path is an app bundle.
	bundle bool
}

// target is what installing a release replaces: the running binary, or the
// macOS app bundle it is run from.
func (u *Upgrader) target() (target, error) {
	if err := supported(); err != nil {
		return target{}, err
	}

	executable := u.executable
	if len(executable) == 0 {
		running, err := u.files.running()
		if err != nil {
			return target{}, fmt.Errorf("finding the running binary: %w", err)
		}

		executable = running
	}

	resolved, err := filepath.EvalSymlinks(executable)
	if err != nil {
		return target{}, fmt.Errorf("finding the running binary: %w", err)
	}

	resolved, err = filepath.Abs(resolved)
	if err != nil {
		return target{}, fmt.Errorf("finding the running binary: %w", err)
	}

	if bundle, isBundled := bundleOf(resolved); isBundled && u.platform.OS == "darwin" {
		return target{path: bundle, executable: resolved, bundle: true}, nil
	}

	return target{path: resolved, executable: resolved}, nil
}

// bundleOf is the macOS app bundle executable is the binary of, when it is
// one's: <dir>/<Name>.app/Contents/MacOS/<file>.
func bundleOf(executable string) (string, bool) {
	macOS := filepath.Dir(executable)
	contents := filepath.Dir(macOS)
	bundle := filepath.Dir(contents)

	if filepath.Base(macOS) != "MacOS" || filepath.Base(contents) != "Contents" || !strings.HasSuffix(filepath.Base(bundle), ".app") {
		return "", false
	}

	return bundle, true
}

// archiveFor is what the archive of version that holds what replaces target
// is called: the app bundle's, or this platform's.
func (u *Upgrader) archiveFor(version string, target target) string {
	if target.bundle {
		return appArchiveName(version)
	}

	return archiveName(version, u.platform)
}

// archiveName is what the archive a release of version holds of platform's
// build is called, as .goreleaser.yaml names it.
func archiveName(version string, platform Platform) string {
	extension := ".tar.gz"
	if platform.OS == "windows" {
		extension = ".zip"
	}

	return fmt.Sprintf("tarhche_%s_%s_%s%s", version, platform.OS, platform.arch(), extension)
}

// appArchiveName is what the archive of the macOS app bundle a release of
// version holds is called.
func appArchiveName(version string) string {
	return fmt.Sprintf("tarhche_%s_darwin_app.zip", version)
}

// checksumsName is what the file of the checksums of a release of version's
// files is called.
func checksumsName(version string) string {
	return fmt.Sprintf("tarhche_%s_checksums.txt", version)
}

// runningPlatform is the platform the running binary was built for.
func runningPlatform() Platform {
	info, _ := debug.ReadBuildInfo()

	return platformOf(runtime.GOOS, runtime.GOARCH, info)
}

// platformOf is goos and goarch, and, for arm, the GOARM info says the build
// was made for (defaultArm when it says none).
func platformOf(goos, goarch string, info *debug.BuildInfo) Platform {
	platform := Platform{OS: goos, Arch: goarch}
	if goarch != "arm" {
		return platform
	}

	platform.Arm = defaultArm

	if info == nil {
		return platform
	}

	for _, setting := range info.Settings {
		if setting.Key == "GOARM" && len(setting.Value) > 0 {
			platform.Arm, _, _ = strings.Cut(setting.Value, ",")
		}
	}

	return platform
}

// arch is p's architecture as an archive is named after it: armv7 for arm.
func (p Platform) arch() string {
	if p.Arch == "arm" && len(p.Arm) > 0 {
		return p.Arch + "v" + p.Arm
	}

	return p.Arch
}

// name is p as it is said: linux/amd64, linux/armv7.
func (p Platform) name() string {
	return p.OS + "/" + p.arch()
}

// binary is what tarhche's binary is called in p's archives.
func (p Platform) binary() string {
	if p.OS == "windows" {
		return "tarhche.exe"
	}

	return "tarhche"
}
