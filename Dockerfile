# syntax=docker/dockerfile:1

# Every image is named in its FROM line, never through an ARG: Dependabot
# reads FROM lines alone. CI reads the linter's and GoReleaser's versions
# from here too, so that make lint and make snapshot run what CI runs.

FROM golang:1.27-trixie AS golang

FROM golangci/golangci-lint:v2.15 AS golangci-lint

FROM goreleaser/goreleaser:v2.20.3 AS goreleaser

# develop is the toolbox every make target runs in: Go, Node, wails3, the
# linter and GoReleaser, so that nothing but Docker has to be installed on
# the host. It is Debian rather than Alpine because the race detector needs
# glibc. The checkout mounted on /app is owned by whoever owns it on the
# host, which git refuses to read as root unless it is told it is safe.
FROM node:26.13-trixie AS develop

COPY --from=golang /usr/local/go /usr/local/go
COPY --from=golangci-lint /usr/bin/golangci-lint /usr/local/bin/golangci-lint
COPY --from=goreleaser /usr/bin/goreleaser /usr/local/bin/goreleaser

ENV GOPATH=/go \
    PATH=/go/bin:/usr/local/go/bin:${PATH} \
    GOTOOLCHAIN=local

ARG WAILS_VERSION=v3.1.0
RUN CGO_ENABLED=0 go install -tags purego github.com/wailsapp/wails/v3/cmd/wails3@${WAILS_VERSION} \
    && rm -rf /root/.cache/go-build /go/pkg/mod \
    && git config --system --add safe.directory /app

WORKDIR /app

# build cross-compiles the CLI alone on the machine building the image: the
# image has no display to open the desktop app on, and a binary that never
# loads a web view (nodesktop) is static, which scratch needs. A release
# binary loads one at run time (Wails' purego backend), and so needs the C
# library of a desktop.
FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS build

ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT
ARG VERSION=dev

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN GOARM="${TARGETVARIANT#v}" CGO_ENABLED=0 GOOS="${TARGETOS}" GOARCH="${TARGETARCH}" \
    go build -trimpath -tags nodesktop \
        -ldflags "-s -w -X github.com/Tarhche/cli/internal/version.Version=${VERSION}" \
        -o /out/tarhche ./cmd/tarhche

# rootfs is what the image holds beside the binary: the certificates the API
# is verified with, and a home for the configuration and the sessions.
FROM --platform=$BUILDPLATFORM alpine:3.24 AS rootfs

RUN mkdir -p /home/tarhche/.tarhche

# runtime is the image without its binary, which production builds from the
# source and release copies from what GoReleaser built.
FROM scratch AS runtime

COPY --from=rootfs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=rootfs --chown=65532:65532 /home/tarhche /home/tarhche

ENV HOME=/home/tarhche

USER 65532:65532

WORKDIR /home/tarhche

ENTRYPOINT ["/usr/local/bin/tarhche"]

# release is the image a release publishes (.goreleaser.yaml): GoReleaser
# puts the binary it built for each platform in the build context, under the
# platform's name (linux/arm/v7/tarhche).
FROM runtime AS release

ARG TARGETPLATFORM

COPY ${TARGETPLATFORM}/tarhche /usr/local/bin/tarhche

# production, the last stage and so what a plain docker build builds, is the
# same image with the binary built from the source.
FROM runtime AS production

COPY --from=build /out/tarhche /usr/local/bin/tarhche
