COMPOSE ?= docker compose
RUN ?= $(COMPOSE) run --rm develop

TAGS := purego
BUILD_TAGS := $(TAGS),production
VERSION ?= $(or $(shell git describe --tags --always --dirty 2>/dev/null | sed 's/^v//'),dev)
LDFLAGS := -s -w -X github.com/Tarhche/cli/internal/version.Version=$(VERSION)

HOST_OS := $(shell uname -s | tr '[:upper:]' '[:lower:]')
HOST_ARCH := $(shell uname -m | sed -e 's/x86_64/amd64/' -e 's/aarch64/arm64/')

PLATFORMS := darwin/amd64 darwin/arm64 linux/386 linux/amd64 linux/arm linux/arm64 windows/386 windows/amd64 windows/arm64

.PHONY: image sh deps test cover lint generate icons frontend frontend-test build build-all app desktop docker release-check snapshot clean

image: ## builds the development image every target runs in
	$(COMPOSE) build develop

sh: ## opens a shell in the development container
	$(RUN) bash

deps: ## downloads the Go modules and installs the frontend's packages
	$(RUN) sh -c 'go mod download && npm --prefix frontend ci'

test: ## runs the Go tests, with the race detector, as CI does
	$(RUN) sh -c 'go test -race -cover -tags $(TAGS) ./... && go test -race -tags nodesktop ./cmd/tarhche ./internal/upgrade'

cover: ## writes coverage.out and prints the coverage of every function
	$(RUN) sh -c 'go test -tags $(TAGS) -coverprofile=coverage.out ./... && go tool cover -func=coverage.out'

lint: ## lints the Go code, as .golangci.yaml says
	$(RUN) golangci-lint run ./...

generate: ## regenerates the frontend's bindings to the desktop services
	$(RUN) sh -c 'CGO_ENABLED=0 wails3 generate bindings -f "-tags=$(TAGS)" -ts -d frontend/bindings -clean ./internal/desktop/...'

icons: ## renders every icon the app ships from build/icon.svg, and the Windows resources
	$(RUN) ./scripts/icons.sh

frontend: ## builds the desktop UI into frontend/dist
	$(RUN) sh -c 'cd frontend && npm ci && npm run build'

frontend-test: ## lints, type checks and tests the desktop UI
	$(RUN) sh -c 'cd frontend && npm ci && npm run lint && npm run typecheck && npm test'

build: frontend ## builds bin/tarhche for this machine
	$(RUN) env CGO_ENABLED=0 GOOS=$(HOST_OS) GOARCH=$(HOST_ARCH) go build -trimpath -tags $(BUILD_TAGS) -ldflags "$(LDFLAGS)" -o bin/tarhche ./cmd/tarhche

build-all: frontend ## builds every released platform into dist/
	$(RUN) sh -c 'for platform in $(PLATFORMS); do \
		os=$${platform%/*}; arch=$${platform#*/}; ext=; [ $$os = windows ] && ext=.exe; \
		echo "building $$os/$$arch"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch GOARM=7 go build -trimpath -tags $(BUILD_TAGS) -ldflags "$(LDFLAGS)" -o dist/tarhche_$${os}_$${arch}$$ext ./cmd/tarhche || exit 1; \
	done'

app: frontend ## builds bin/Tarhche.app, the desktop app for a Mac of this architecture, and its zip
	$(RUN) sh -c 'CGO_ENABLED=0 GOOS=darwin GOARCH=$(HOST_ARCH) go build -trimpath -tags $(BUILD_TAGS) -ldflags "$(LDFLAGS)" -o bin/darwin/tarhche ./cmd/tarhche \
		&& ./scripts/app.sh bin/darwin/tarhche $(VERSION) bin'

desktop: build ## builds bin/tarhche and opens the desktop UI on this machine
	./bin/tarhche --desktop

docker: ## builds the production image
	docker build --target production --build-arg VERSION=$(VERSION) -t tarhche/cli:$(VERSION) .

release-check: ## checks the release configuration, .goreleaser.yaml
	$(RUN) goreleaser check

snapshot: ## builds what a release publishes into dist/, but for its images, and publishes nothing
	$(RUN) goreleaser release --snapshot --clean --skip=docker

clean: ## removes what the builds wrote
	rm -rf bin dist coverage.out
