# Tarhche CLI

`tarhche` manages a [Tarhche](https://tarhche.com) estate from a terminal, the way `docker` manages containers. It manages everything the web dashboard does: content (articles, comments, users, roles and the rest) and workloads (VMs, containers, images, networks, volumes, stacks and snapshots). It also opens a terminal in a workload, runs commands there, copies files in and out, and forwards a workload's ports to your machine. Run with `--desktop`, or opened as an app is (from Finder or the Dock, Explorer, or a desktop entry), it is a desktop app that uses the same contexts and sessions.

It is one binary per platform. On macOS and Windows it needs nothing beside it. On Linux it needs the C library every mainstream distribution has (glibc), which the desktop app loads its web view through; on a musl distribution such as Alpine, use the [Docker image](#docker), whose binary is the CLI alone and static. The desktop app needs a web view (see [Linux](#linux-the-desktop-apps-libraries)).

## Install

### Release binaries

Every [release](https://github.com/Tarhche/cli/releases) has an archive for each platform, plus a checksums file:

| OS | Architectures | Archive |
| --- | --- | --- |
| macOS | `amd64` (Intel), `arm64` (Apple silicon), `universal` (both) | `tarhche_<version>_darwin_<arch>.tar.gz` |
| Linux | `386`, `amd64`, `armv7`, `arm64` | `tarhche_<version>_linux_<arch>.tar.gz` |
| Windows | `386`, `amd64`, `arm64` | `tarhche_<version>_windows_<arch>.zip` |
| macOS, as an app | `universal` | `tarhche_<version>_darwin_app.zip` (`Tarhche.app`) |

A Linux archive also holds the desktop app's entry and icon (`tarhche.desktop`, `tarhche.png`).

On Linux or macOS:

```sh
version=1.2.3 os=linux arch=amd64   # the release you want, without its v
base=https://github.com/Tarhche/cli/releases/download/v$version

curl -fsSLO "$base/tarhche_${version}_${os}_${arch}.tar.gz"
curl -fsSLO "$base/tarhche_${version}_checksums.txt"
sha256sum --check --ignore-missing "tarhche_${version}_checksums.txt"   # macOS: shasum -a 256 --check --ignore-missing ...

tar -xzf "tarhche_${version}_${os}_${arch}.tar.gz" tarhche
sudo install tarhche /usr/local/bin/
tarhche --version
```

On Windows, extract `tarhche.exe` from the zip into a directory on your `PATH`.

### The desktop app

The binary is the desktop app too. Started with no command and no terminal of its own, as an app is started, it opens the desktop app instead of printing its help; from a terminal it is the CLI, and `--desktop` opens the app from anywhere.

- **macOS:** unzip `tarhche_<version>_darwin_app.zip` and move `Tarhche.app` to `/Applications`. Finder, the Dock, Launchpad and Spotlight open it as the desktop app. Its binary is the CLI as well, so link it instead of installing a second one:

  ```sh
  sudo ln -sf /Applications/Tarhche.app/Contents/MacOS/tarhche /usr/local/bin/tarhche
  ```

- **Windows:** double-click `tarhche.exe`, or a shortcut to it, to open the desktop app. In a terminal it is the CLI. It is a console program, so a console window may flash for a moment as the app starts.
- **Linux:** with the binary on your `PATH`, install the desktop entry and its icon from the archive. Your desktop then lists Tarhche among its applications:

  ```sh
  install -Dm644 tarhche.desktop ~/.local/share/applications/tarhche.desktop
  install -Dm644 tarhche.png ~/.local/share/icons/hicolor/512x512/apps/tarhche.png
  ```

  Opened from a file manager without a terminal, the binary opens the desktop app as well. Where there is no display, it stays the CLI.

### Docker

```sh
docker run --rm -it ghcr.io/tarhche/cli --help
```

The image holds the CLI alone (no desktop app), a static binary on an empty base. It is published for `linux/amd64`, `linux/arm64`, `linux/386` and `linux/arm/v7`. Its tags are `latest` (the latest release), each release's version (`1.2.3`), and `main` (the latest commit on main). See [In Docker](#in-docker) below for keeping a session.

### With Go

```sh
CGO_ENABLED=0 go install -tags purego github.com/Tarhche/cli/cmd/tarhche@latest
```

This needs Go 1.27 or newer. The `purego` tag lets the desktop app be built without cgo; `-tags nodesktop` instead builds the CLI alone, a static binary that runs without any C library. A build like this reports its version as `dev`, and it has no desktop interface: the interface is built from `frontend/` separately, and only release builds embed it. Use a release binary for `--desktop`.

### macOS

The binaries and the app are not notarized. Anything downloaded with a browser is quarantined, and Gatekeeper refuses to run it. Remove the quarantine once:

```sh
xattr -d com.apple.quarantine /usr/local/bin/tarhche
xattr -dr com.apple.quarantine /Applications/Tarhche.app
```

Files downloaded with `curl`, as shown above, are not quarantined.

### Linux: the desktop app's libraries

The CLI needs no libraries. Only `--desktop` does: it loads GTK 4 and WebKitGTK 6.0 at run time.

| Distribution | Install |
| --- | --- |
| Debian, Ubuntu | `sudo apt install libgtk-4-1 libwebkitgtk-6.0-4` |
| Fedora | `sudo dnf install gtk4 webkitgtk6.0` |
| Arch | `sudo pacman -S gtk4 webkitgtk-6.0` |

On macOS the desktop app uses the system's WebKit. On Windows it uses WebView2, which ships with Windows 10 and 11.

## Upgrading

`tarhche upgrade` looks for a newer release, tells what it says, and replaces the running binary with it once you agree:

```sh
tarhche upgrade            # the latest release, asking first
tarhche upgrade --check    # only say whether a newer release is out, and what it says
tarhche upgrade --yes      # upgrade without asking, as a script would
tarhche upgrade 1.2.3      # the release of 1.2.3 instead: a downgrade, or a reinstall of the version running
```

It downloads the archive of the release for your platform from GitHub, checks it against the release's checksums file, and replaces the binary only once all of that succeeded. A binary run from the macOS app bundle (`Tarhche.app/Contents/MacOS/tarhche`) has the whole bundle replaced. On Windows the running `tarhche.exe` is renamed to `tarhche.exe.old` first, since Windows does not let a running program be replaced; the next upgrade removes it.

A binary only root may replace, such as one installed into `/usr/local/bin` with `sudo install`, is upgraded with `sudo tarhche upgrade`; on Windows, a binary under `Program Files` is upgraded from a terminal run as administrator. The Docker image cannot upgrade itself: pull a newer image instead (`docker pull ghcr.io/tarhche/cli`).

## Quick start

Sign in to the estate of the current context:

```sh
tarhche login                                            # in a browser
tarhche login -u you@example.com --password-stdin < pw   # or with a password
tarhche info                                             # who you are, and where
```

A context names an estate: its API and its workload ingress. The `default` context is Tarhche itself (`https://backend.tarhche.com`). Each context keeps its own session.

```sh
tarhche context create --api https://api.example.com --ingress https://workload.example.com staging
tarhche context use staging
tarhche context ls
tarhche --context default article ls     # one command against another context
```

Workloads have docker's commands and flags wherever docker has the command:

```sh
tarhche vm ls
tarhche ps                               # your containers, as docker ps
tarhche run [OPTIONS] IMAGE [COMMAND]    # a container in one of your Docker VMs
tarhche logs -f CONTAINER
tarhche exec -it CONTAINER sh            # a command, with a terminal
tarhche cp ./site CONTAINER:/srv         # files in, and out the other way round
tarhche port-forward CONTAINER 8080:80   # localhost:8080 is the container's port 80
```

A VM has the same, in its group: a terminal of its own, commands, files and ports:

```sh
tarhche vm create --docker --name builds
tarhche vm attach builds                 # its shell
tarhche vm exec builds uname -a
tarhche vm cp builds:/var/log/syslog .
tarhche vm port-forward builds 5432      # a port it publishes, or one inside a Docker VM
```

A port a VM publishes is forwarded through the ingress as it is; one it does not, in a Docker VM, is relayed from inside it, which reaches a container's ports too.

Content has a group of commands per resource:

```sh
tarhche article ls                       # your own articles
tarhche article ls -A                    # everybody's, if you may see them
tarhche article inspect CORRELATION_UUID/en   # an article is one of its languages
tarhche user ls
tarhche role create -f role.json
```

Listings are tables by default. As with docker, `--format` takes `json` (one item per line), a Go template, or `table` followed by a template for a table with headers. `-q` prints only the IDs:

```sh
tarhche article ls --format json
tarhche vm ls -q
```

Every command has `--help`. Global flags go before the command's name: `--context`/`-c`, `--host`/`-H` (another API), `--ingress`, `--language` (`en` or `fa`), `--debug`/`-D`, and `--config` (default `~/.tarhche`). Each one can also be set with an environment variable (`TARHCHE_CONTEXT`, `TARHCHE_HOST`, ...).

Open the desktop app, or open it as any other app (see [The desktop app](#the-desktop-app)):

```sh
tarhche --desktop
```

It signs in the same way (it shows its own sign-in when the current context has no session), and has a page for everything above: containers, VMs and the rest with their logs, terminals, files and forwarded ports, and every content resource the dashboard manages.

It upgrades itself: **Check for Updates…** (in the tray's menu, the app's menu on macOS, the account menu, which also shows the version running, and on the sign-in screen) looks for a newer release on GitHub, shows what it says of itself, and downloads and installs it in place of the binary, or of the app bundle on macOS, when you ask; then it restarts into it. Where only an administrator may write (`/usr/local/bin`, say), it says so: upgrade from a terminal with `sudo tarhche upgrade` instead.

Sessions are kept in the operating system's keychain, or in a file only you may read (`~/.tarhche/credentials.json`) where there is no keychain. An access token is renewed before it runs out; when a session is gone, a command run in a terminal signs in again in the browser by itself.

### In Docker

A container has no keychain, so the configuration and the session are kept in a file under `/home/tarhche/.tarhche`. Mount a volume there so they outlive the container. A browser cannot reach a sign-in that runs inside a container, so sign in with a password from stdin:

```sh
docker volume create tarhche
printf '%s\n' "$TARHCHE_PASSWORD" | docker run --rm -i -v tarhche:/home/tarhche/.tarhche \
  ghcr.io/tarhche/cli login -u you@example.com --password-stdin
docker run --rm -v tarhche:/home/tarhche/.tarhche ghcr.io/tarhche/cli article ls
```

## Development

Nothing but Docker is needed. Every tool (Go, Node, wails3, golangci-lint, GoReleaser) runs in the `develop` container, through make:

| Command | What it does |
| --- | --- |
| `make image` | builds the development image |
| `make sh` | a shell in it |
| `make test` | the Go tests, with the race detector, as CI runs them |
| `make cover` | `coverage.out`, and the coverage of every function |
| `make lint` | golangci-lint, as `.golangci.yaml` configures it |
| `make generate` | the frontend's bindings to the desktop services |
| `make frontend` | the desktop UI, built into `frontend/dist` |
| `make frontend-test` | the UI's lint, type check and tests |
| `make build` | `bin/tarhche` for this machine |
| `make build-all` | every released platform, into `dist/` |
| `make desktop` | `make build`, then `./bin/tarhche --desktop` on this machine |
| `make app` | `bin/Tarhche.app` (and its zip) for a Mac of this architecture: `open bin/Tarhche.app` |
| `make icons` | every icon the app ships, rendered from the Tarhche logo (`build/icon.svg`) |
| `make docker` | the production image |
| `make release-check` | checks `.goreleaser.yaml` |
| `make snapshot` | everything a release publishes except the images, into `dist/`, with nothing published |

CI (`.github/workflows/ci.yaml`) runs on every pull request and every push to main. It runs the linters, the tests (coverage in the job summary), the frontend's checks, a build of every released platform, the image for every platform, and a GoReleaser snapshot.

To release, push a tag:

```sh
git tag v1.2.3 && git push origin v1.2.3
```

`.github/workflows/release.yaml` then builds the binaries, archives and checksums, writes the release notes from the commits, publishes the GitHub release, and pushes `ghcr.io/tarhche/cli:1.2.3` and `:latest`. A tag with a prerelease suffix (`v1.3.0-rc.1`) is published as a prerelease and is not tagged `latest`.

[CLAUDE.md](CLAUDE.md) describes the code's layout and conventions. [docs/streams.md](docs/streams.md) describes the websocket protocol that terminals, commands and forwarded ports use.
