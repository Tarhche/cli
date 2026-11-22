# syntax=docker/dockerfile:1

ARG GO_VERSION=1.27
ARG NODE_VERSION=26.13
ARG GOLANGCI_LINT_VERSION=v2.15

FROM golang:${GO_VERSION}-trixie AS golang

FROM golangci/golangci-lint:${GOLANGCI_LINT_VERSION} AS golangci-lint

# develop is the toolbox every make target runs in: Go, Node, wails3 and the
# linter, so that nothing but Docker has to be installed on the host. It is
# Debian rather than Alpine because the race detector needs glibc.
FROM node:${NODE_VERSION}-trixie AS develop

COPY --from=golang /usr/local/go /usr/local/go
COPY --from=golangci-lint /usr/bin/golangci-lint /usr/local/bin/golangci-lint

ENV GOPATH=/go \
    PATH=/go/bin:/usr/local/go/bin:${PATH} \
    GOTOOLCHAIN=local

ARG WAILS_VERSION=v3.1.0
RUN CGO_ENABLED=0 go install -tags purego github.com/wailsapp/wails/v3/cmd/wails3@${WAILS_VERSION} \
    && rm -rf /root/.cache/go-build /go/pkg/mod

WORKDIR /app

FROM --platform=$BUILDPLATFORM node:${NODE_VERSION}-alpine AS frontend

WORKDIR /app/frontend

COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci

COPY frontend/ ./
RUN npm run build

# build cross-compiles on the machine building the image: the desktop UI is
# loaded at run time (Wails' purego backend), so no C toolchain is involved
# for any target.
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine AS build

ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT
ARG VERSION=dev

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=frontend /app/frontend/dist ./frontend/dist

RUN GOARM="${TARGETVARIANT#v}" CGO_ENABLED=0 GOOS="${TARGETOS}" GOARCH="${TARGETARCH}" \
    go build -trimpath -tags purego \
        -ldflags "-s -w -X github.com/Tarhche/cli/internal/version.Version=${VERSION}" \
        -o /out/tarhche ./cmd/tarhche \
    && mkdir -p /out/home/.tarhche

FROM scratch AS production

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build --chown=65532:65532 /out/home /home/tarhche
COPY --from=build /out/tarhche /usr/local/bin/tarhche

ENV HOME=/home/tarhche

USER 65532:65532

WORKDIR /home/tarhche

ENTRYPOINT ["/usr/local/bin/tarhche"]
