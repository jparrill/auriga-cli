BINARY    := auriga
MODULE    := github.com/jparrill/auriga-cli
VERSION   := $(shell git describe --tags --always 2>/dev/null || echo "dev")
COMMIT    := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE      := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS   := -ldflags "-s -w \
	-X $(MODULE)/internal/cli.Version=$(VERSION) \
	-X $(MODULE)/internal/cli.Commit=$(COMMIT)"

.DEFAULT_GOAL := build

.PHONY: build
build:
	CGO_ENABLED=0 go build $(LDFLAGS) -o bin/$(BINARY) ./cmd/auriga/

.PHONY: install
install: build
	@mkdir -p $(HOME)/bin $(HOME)/.config/auriga
	cp bin/$(BINARY) $(HOME)/bin/$(BINARY)
	@if [ ! -f $(HOME)/.config/auriga/config.yaml ]; then \
		cp config.yaml.example $(HOME)/.config/auriga/config.yaml; \
		echo "Created $(HOME)/.config/auriga/config.yaml"; \
	else \
		echo "Config already exists, skipping"; \
	fi
	@mkdir -p $(HOME)/.config/auriga/prompts
	@cp -n internal/benchmark/prompts/*.md $(HOME)/.config/auriga/prompts/ 2>/dev/null || true
	@echo "Prompts synced to $(HOME)/.config/auriga/prompts/"

.PHONY: cross-linux
cross-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY)-linux-amd64 ./cmd/auriga/

.PHONY: deploy
deploy: cross-linux
	@ssh auriga "mkdir -p ~/bin ~/.config/auriga"
	rsync -avz bin/$(BINARY)-linux-amd64 auriga:~/bin/$(BINARY)
	@ssh auriga "cp ~/bin/$(BINARY) ~/infra/bin/$(BINARY) 2>/dev/null || true"
	@ssh auriga "test -f ~/.config/auriga/config.yaml && echo 'Config already exists, skipping' || true"
	@ssh auriga "test -f ~/.config/auriga/config.yaml" || rsync -avz config.yaml.example auriga:~/.config/auriga/config.yaml

.PHONY: test
test:
	go test ./...

.PHONY: lint
lint:
	golangci-lint run ./...

.PHONY: fmt
fmt:
	gofmt -w .
	goimports -w .

.PHONY: vet
vet:
	go vet ./...

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: clean
clean:
	rm -rf bin/

.PHONY: all
all: fmt vet lint test build

.PHONY: release
release: clean all cross-linux deploy

.PHONY: help
help:
	@echo "Targets:"
	@echo "  build        Build for current platform"
	@echo "  install      Build + copy to ~/bin/"
	@echo "  cross-linux  Cross-compile for Linux amd64"
	@echo "  deploy       Cross-compile + rsync to auriga"
	@echo "  test         Run tests"
	@echo "  lint         Run golangci-lint"
	@echo "  fmt          Format code (gofmt + goimports)"
	@echo "  vet          Run go vet"
	@echo "  tidy         Run go mod tidy"
	@echo "  clean        Remove bin/"
	@echo "  all          fmt + vet + lint + test + build"
	@echo "  release      clean + all + cross-linux + deploy"
	@echo "  help         Show this help"
