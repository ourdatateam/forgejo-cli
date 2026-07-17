.PHONY: build test docs docs-check fmt vet release clean

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS  = -s -w -X github.com/ourdatateam/forgejo-cli/internal/cmd.Version=$(VERSION)

# Release matrix: darwin (Intel + Apple Silicon) and linux (amd64 + arm64).
PLATFORMS = darwin/amd64 darwin/arm64 linux/amd64 linux/arm64

build:
	go build -ldflags '$(LDFLAGS)' -o bin/forgejo ./cmd/forgejo

test:
	go test ./...

docs:
	go run ./tools/gendocs

docs-check:
	@tmp=$$(mktemp -d); \
	trap 'rm -rf "$$tmp"' EXIT; \
	go run ./tools/gendocs -out "$$tmp"; \
	diff -ru --exclude=recipes.md "$$tmp" skills/forgejo-cli/references

fmt:
	gofmt -w ./cmd ./internal ./tools

vet:
	go vet ./...

# Cross-compiled, static release binaries + tarballs + checksums in dist/.
# Pure-Go dependency tree, so CGO_ENABLED=0 works from any build host.
release: clean
	@set -e; \
	for p in $(PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; \
		out="dist/forgejo_$(VERSION)_$${os}_$${arch}"; \
		echo "building $$out"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch \
			go build -trimpath -ldflags '$(LDFLAGS)' -o "$$out/forgejo" ./cmd/forgejo; \
		tar -C "$$out" -czf "$$out.tar.gz" forgejo; \
		rm -r "$$out"; \
	done; \
	cd dist && sha256sum *.tar.gz > SHA256SUMS; \
	ls -l

clean:
	rm -rf bin dist
