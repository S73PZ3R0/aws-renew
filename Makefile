.PHONY: build test checksum verify clean help

BINARY_NAME=aws-renew

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

build-local:
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
	@echo "Generating checksums..."
	@find . -maxdepth 4 -not -path '*/.*' -not -path './dist/*' -not -path './build/*' -not -name "$(BINARY_NAME)" -not -name "CHECKSUM.asc" -type f -exec sha256sum {} + | sed 's| \./| |' | sort -k 2 > CHECKSUM.asc
	@echo "Checksums updated in CHECKSUM.asc"

verify: build
	./$(BINARY_NAME) --verify

release:
	@echo "Creating production release with GoReleaser..."
	@goreleaser release --clean --parallelism 1

clean:
	rm -f $(BINARY_NAME)
	rm -rf dist/
	rm -rf build/
