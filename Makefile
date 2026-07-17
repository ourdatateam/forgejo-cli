.PHONY: build test docs docs-check fmt vet

GOCACHE ?= /tmp/forgejo-cli-gocache
export GOCACHE

build:
	go build -o bin/forgejo ./cmd/forgejo

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
	gofmt -w tools/gendocs/main.go

vet:
	go vet ./...
