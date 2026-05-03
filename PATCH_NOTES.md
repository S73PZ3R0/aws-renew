# Patch Notes

## [1.6.2] - 2026-05-03

**Author:** S73PZ3R0 (YGNight)

### Fixed
- **Integrity Verification**: Resolved a bug where `--update` would fail with an `INTEGRITY_ALERT` due to missing source files in binary releases. The verifier now correctly skips non-existent files while strictly validating all present artifacts.

## [1.6.1] - 2026-05-03

**Author:** S73PZ3R0 (YGNight)

### Added
- **Interactive Configuration**: New `configure` command for seamless setup.
- **Discord & Slack Support**: Expanded notification suite.
- **Notification Persistence**: CLI flags now persist to config automatically.
- **Root Installation**: Simplified Go installation.
