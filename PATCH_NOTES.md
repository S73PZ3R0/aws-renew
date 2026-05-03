# Patch Notes

## [1.6.8] - 2026-05-04

**Author:** S73PZ3R0 (YGNight)

### Fixed
- **`--update` Integrity Check**: Removed erroneous `stripTopDir` from archive extraction — GoReleaser archives have no top-level wrapper directory, so stripping the first path component was incorrectly rewriting `internal/pkg/file.go` → `pkg/file.go`, causing all internal files to fail hash verification.
- **Checksum Scope**: `make checksum` now hashes only the files included in the release archive. Previously it used `find .` which included development-only files absent from the archive, causing spurious `INTEGRITY_ALERT` failures on `--update`.

---

## [1.6.7] - 2026-05-04

**Author:** S73PZ3R0 (YGNight)

### Added
- **Termux Service Management**: `aws-renew service install/start/stop/status` now works on Termux via `sv` (runit). Requires `pkg install termux-services`. Creates a proper run script at `$PREFIX/var/service/aws-renew/run`.

---

## [1.6.6] - 2026-05-04

**Author:** S73PZ3R0 (YGNight)

### Fixed
- **Secure Update**: `CHECKSUM.asc` is now GPG-clearsigned at build time, fixing the `--update` command which was failing with `SECURITY_ALERT: Update binary is UNSIGNED`.
- **Termux Update**: `--update` now works on Termux. Removed the `android` build target (Termux is a Linux userspace — `linux/arm64` is correct). Old `GOOS=android` binaries fall back to the `linux/arm64` asset and apply the update via `os.Args[0]` when `os.Executable()` is unavailable.

---

## [1.6.5] - 2026-05-03

**Author:** S73PZ3R0 (YGNight)

### Added
- **Richer Notifications**: All notification channels (Telegram, Discord, Slack, Webhook) now include the managed ports and security group names/IDs in alert messages.

### Fixed
- **Duplicate Notifications**: Removed a redundant post-run notification block that was sending a second alert for Webhook and Telegram channels after the per-instance notifications had already fired.
- **Test Coverage**: Added full test suite for the `notify` package covering all channels, port/SG fields, HTTP error handling, and multi-channel delivery.
- **Update Integrity**: Fixed archive extraction to preserve directory structure so `--verify` and `--update` correctly validate all source file hashes rather than silently skipping nested files.

---

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
