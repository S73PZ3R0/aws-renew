.PHONY: build test checksum verify clean help update-readme-version

BINARY_NAME=aws-renew
VERSION=$(shell grep 'var version' main.go | sed 's/.*"\(.*\)".*/\1/')

help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  build     Build the binary"
	@echo "  test      Run tests"
	@echo "  checksum  Generate SHA256 hashes for all source files"
	@echo "  verify    Run integrity verification check"
	@echo "  clean     Remove build artifacts"

build: build-local

update-readme-version:
	@sed -i "s|version-[0-9]*\.[0-9]*\.[0-9]*-gold|version-$(VERSION)-gold|" README.md
	@sed -i "s|Installation (v[0-9]*\.[0-9]*\.[0-9]* Go)|Installation (v$(VERSION) Go)|" README.md
	@echo "README version updated to $(VERSION)"

build-local: update-readme-version
	@echo "Building for current platform..."
	@go build -o $(BINARY_NAME) .
	@$(MAKE) checksum

install:
	@echo "Installing to $(GOPATH)/bin..."
	@go install .

snapshot:
	@echo "Building multi-platform snapshots sequentially (limited resources)..."
	@goreleaser build --snapshot --clean --parallelism 1

test:
	@go test -v ./...

checksum:
	@echo "Generating and signing checksums..."
	@{ find internal -type f; echo "LICENSE"; echo "README.md"; echo "go.mod"; echo "go.sum"; echo "main.go"; echo "main_test.go"; echo "Makefile"; echo "PATCH_NOTES.md"; echo "config.yaml.example"; } | sort | xargs sha256sum | sort -k 2 | gpg --batch --yes --local-user "S73PZ3R0" --output CHECKSUM.asc --clearsign -
	@echo "Checksums signed in CHECKSUM.asc"

verify: build
	./$(BINARY_NAME) --verify

release: update-readme-version checksum
	@if ! git diff --quiet README.md CHECKSUM.asc; then \
		git add README.md CHECKSUM.asc && git commit -m "v$(VERSION): pre-release sync" && git push && git tag -f v$(VERSION) && git push origin v$(VERSION) --force; \
	fi
	@echo "Creating production release with GoReleaser..."
	@goreleaser release --clean --parallelism 1

clean:
	rm -f $(BINARY_NAME)
	rm -rf dist/
	rm -rf build/
