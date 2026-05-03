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
	@find . -maxdepth 4 -not -path '*/.*' -not -path './dist/*' -not -path './build/*' -not -name "$(BINARY_NAME)" -not -name "CHECKSUM.asc" -type f -exec sha256sum {} + | sed 's| \./| |' | sort -k 2 | gpg --batch --yes --local-user "S73PZ3R0" --output CHECKSUM.asc --clearsign -
	@echo "Checksums signed in CHECKSUM.asc"

verify: build
	./$(BINARY_NAME) --verify

release:
	@echo "Creating production release with GoReleaser..."
	@goreleaser release --clean --parallelism 1

clean:
	rm -f $(BINARY_NAME)
	rm -rf dist/
	rm -rf build/
