# Patch Notes

## [1.6.0] - 2026-05-03

**Author:** S73PZ3R0 (YGNight)

### Added
- **Local Install Automation**: Introduced `make install` to streamline local deployment to `$GOPATH/bin`.
- **Author Attribution**: Updated project metadata with official author credentials.
- **Dynamic Versioning**: Full implementation of build-time version injection and runtime discovery.
- **Security Verification**: Enhanced integrity layer with GPG-signed checksums and automatic recovery.
- **Verified Updates**: The `--update` command now performs cryptographic validation in a secure sandbox.
- **Platform Support**: Native builds for **Linux (AMD64/ARM64)**, **Windows (32/64-bit)**, and **macOS**. Full **Termux** compatibility.

### Changed
- **Architecture**: Completed high-performance Go-based implementation using AWS SDK v2.
- **TUI**: Advanced interactive interface powered by Bubble Tea.
