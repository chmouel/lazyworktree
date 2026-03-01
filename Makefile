NAME = lazyworktree
MKDOCS = NO_MKDOCS_2_WARNING=1 uvx --with 'mkdocs<2' --with mkdocs-material --with pymdown-extensions --with mkdocs-glightbox mkdocs

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

docs-build:
	$(MKDOCS) build --strict

docs-serve:
	$(MKDOCS) serve  -a 0.0.0.0:7827

release:
	./hack/make-release.sh

optimize:
	for i in .github/screenshots/*.png;do pngquant --ext .new.png --skip-if-larger --quality 75 -f $$i;t=$${i/.png/.new.png};[[ -e $$t ]] && mv -vf $$t $$i || true;done
.PHONY: all build lint format test coverage sanity mkdir release docs-build docs-serve
