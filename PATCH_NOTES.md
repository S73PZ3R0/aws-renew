# Patch Notes

## [1.6.3] - 2026-05-03

**Author:** S73PZ3R0 (YGNight)

### Added
- **Shell Autocompletions**: Added full support for Bash and Zsh completions. Setup instructions included in `README.md`.

### Fixed
- **Integrity Verification**: Resolved an `INTEGRITY_ALERT` in `--update` by correctly handling missing source files in binary releases.

## [1.6.2] - 2026-05-03

**Author:** S73PZ3R0 (YGNight)

### Fixed
- **Integrity Verification**: (Hotfix) Addressed false-positive tampering alerts during updates.

## [1.6.1] - 2026-05-03

**Author:** S73PZ3R0 (YGNight)

### Added
- **Interactive Configuration**: New `configure` command for seamless setup.
- **Discord & Slack Support**: Expanded notification suite.
- **Notification Persistence**: CLI flags now persist to config automatically.
- **Root Installation**: Simplified Go installation.
