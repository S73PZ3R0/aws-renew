# Patch Notes

## [1.5.0] - 2026-05-03

### Added
- **Dynamic Versioning**: Implemented build-time version injection and runtime discovery, eliminating hardcoded version strings in the codebase.
- **Security Verification**: Integrated a robust integrity layer using GPG-signed checksums. The tool now supports automatic retrieval of missing `CHECKSUM.asc` files from GitHub.
- **Verified Updates**: The `--update` command performs comprehensive cryptographic validation (GPG + Hashes) in a secure sandbox prior to application.
- **Multi-Platform Support**: Optimized builds for **Linux (AMD64/ARM64)**, **Windows (32/64-bit)**, and **macOS**. Full native compatibility with **Termux on Android**.
- **Modern TUI**: A high-fidelity interactive "Orchestrator" interface powered by Bubble Tea.

### Changed
- **Performance Optimization**: Transitioned to a high-performance Go-based architecture utilizing the official AWS SDK v2.
- **Deployment Strategy**: Now distributed as a single static binary with support for direct `go install`.
