NAME = lazyworktree

all: build

mkdir:
	mkdir -p bin

build: mkdir
	go build -o bin/$(NAME) ./cmd/$(NAME)

sanity: lint format test

lint:
	golangci-lint run --fix ./...

format:
	gofumpt -w .

test:
	go test ./...

coverage:
	go test ./... -covermode=count -coverprofile=coverage.out
	go tool cover -func=coverage.out -o=coverage.out

release:
	./hack/make-release.sh

.PHONY: all build lint format test coverage sanity mkdir release
