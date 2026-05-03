# Patch Notes

## [1.6.4] - 2026-05-03

**Author:** S73PZ3R0 (YGNight)

### Added
- **Termux (Android) Support**: Automated updates now correctly identify and download ARM64 binaries on Termux.
- **Compatibility Layer**: Included source metadata in release archives to allow legacy v1.6.1/v1.6.2 binaries to successfully verify and upgrade to this version.

### Fixed
- **Integrity Logic**: (Finalized) Improved the verifier to gracefully handle binary-only releases without triggering false `INTEGRITY_ALERT` warnings.

## [1.6.3] - 2026-05-03

**Author:** S73PZ3R0 (YGNight)

### Added
- **Shell Autocompletions**: Support for Bash and Zsh completions.
