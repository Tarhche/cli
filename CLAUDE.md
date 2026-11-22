# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

Nothing but Docker is installed on the host: every tool (Go, Node, wails3, golangci-lint) runs in the `develop` container, through make.

```sh
make image          # builds the development image (docker compose build develop)
make sh             # a shell in it
make test           # go test -race -cover -tags purego, every package but node_modules' (same as CI)
make cover          # coverage.out and the coverage of every function
make lint           # golangci-lint
make generate       # regenerates frontend/bindings from the desktop services (wails3 generate bindings)
make frontend       # npm ci && npm run build in frontend/, into frontend/dist
make frontend-test  # the frontend's lint, type check and tests
make build          # bin/tarhche for this machine, built in the container (CGO_ENABLED=0)
make build-all      # every released platform into dist/
make desktop        # make build, then ./bin/tarhche --desktop on the host
make docker         # the production image

docker compose run --rm develop go test -tags purego ./internal/api -run TestClientDo -v   # a single test
```

Go 1.27. **Every build is `CGO_ENABLED=0 -tags purego`**: the desktop app is Wails v3.1 with its purego backends, which load GTK4/WebKitGTK 6.0 (Linux) and AppKit/WebKit (macOS) at run time; Windows is pure Go. So one static binary per platform is cross-compiled from anywhere, and the CLI runs on a server with no GTK at all: only `--desktop` needs the libraries. Without the tag on Linux, Wails tries cgo and pkg-config.

## What it is

`tarhche` is the Tarhche CLI. Run as it is, it is a command line that works as docker's does; run with `--desktop` it opens a desktop app over the same contexts and sessions. It manages everything the web dashboard manages (content, users, roles, workloads), forwards a workload's ports to the host, and copies files in and out of workloads.

- `cmd/tarhche` — `main`: `--version` and `--desktop` are acted on before the console runs; everything else is the console's.
- `internal/command` — what every command is built on: `Cli` (streams, `Globals`, `Config()`, `Context()`, `Auth()`, `Tokens()`, `Client(ctx)`), `Exit(err)` (how errors are reported and which status a command exits with), `UsageError`/`StatusError`, the argument checks, `Alias`. `commandtest` runs a command against an API a test serves.
- `internal/command/commands` — the console: global flags and every group. `system` (login, logout, version, info, context), `content` (a group per content resource, built from `internal/resource`), `workload` (vm, container, image, network, volume, stack, snapshot and docker's shortcuts: ps, run, rm, logs, ...), `remote` (attach, exec, cp, port-forward; registered by `workload` in the container and vm groups and at the top level).
- `internal/api` — the API client: signs requests with a `TokenSource`, renews a refused token once, `*api.Error` for what is not a success (`code`/`message` failures, `errors` validation by field), `List`/`All` for the `{items, pagination}` listings.
- `internal/auth` — signing in (browser: OAuth 2.1 authorization code + PKCE, answered on 127.0.0.1; or a password) and keeping a session fresh; `internal/credentials` keeps sessions (keychain, else a 0600 file); `internal/config` keeps contexts (`~/.tarhche/config.json`).
- `internal/resource` — every dashboard resource described once (routes, key, default table); the CLI's generic commands and the desktop app's generic pages are both built from it.
- `internal/workload` — which VM or container a reference means; `internal/stream` the websocket streams of `docs/streams.md`.
- `internal/formatter` — docker's `--format`: a table by default, `json`, or a Go template, with docker's template functions and `ago`, `bytes`, `default`.
- `internal/desktop` — the Wails app: services bound to the frontend; `frontend/` — React + TypeScript + Vite + Mantine, built into `frontend/dist` and embedded by `frontend/embed.go`.

## The API it talks to

A context names an estate: its API (default `https://backend.tarhche.com`) and its workload ingress (default `https://workload.tarhche.com`), which streams are opened on (`docs/streams.md`). Listings answer `{"items": [...], "pagination": {"total_pages", "current_page"}}` and take `?page=`. A refusal is `400 {"errors": {"field": "message"}}`; a workload failure is `{"code", "message"}`. Workload resources are answered in their kind's manifest shape, `{kind, metadata, spec, status}`. One's own things are under `/api/dashboard/my/...`, anybody's (what an administrator manages) under `/api/dashboard/...`.

## Conventions

- **A command is a type of its own, in a file of its own named after what it does** (`list.go`: `type listCommand struct`, `newListCommand(cli)`), implementing `console.Command`. `Configure` keeps the flag set and defines the flags with `console.Var`; `Run` is `return c.cli.Exit(c.run(ctx))`, and `run` checks its arguments first (`command.ExactArgs(c.flags, 1)`). Flags come before arguments: the console stops at the first argument. Output goes to `cli.Out` through `internal/formatter`, errors are returned, never printed by the command, and nothing is logged.
- Docker's names and flags wherever docker has the command: `ls` (with `ps` and `list` registered as `Alias`es where docker has them), `rm`, `inspect`, `-q/--quiet`, `-f/--force`, `--format`, `-a/--all`. One's own things are listed by default; `--all-users` (`-A`) lists anybody's, through the administrator's routes.
- Tests: one `Test<Unit>` per unit, `t.Parallel()` first, its cases as `t.Run` subtests, also `t.Parallel()`; testify's `assert`/`require`; an API is an `httptest` server (`commandtest.New`). Test doubles live in a `mock` package beside what they stand in for. Every package's logic is covered.
- Doc comments on every exported (and most unexported) type and function; no inline comments narrating what a block does: rename instead.
- The desktop frontend uses `@mantine/core` components directly, no wrappers of its own for what Mantine ships; one layout adapts to every width.
