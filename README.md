# ⚡ AWS-RENEW ⚡

[![Go Version](https://img.shields.io/badge/go-1.24%2B-blue.svg)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![Version](https://img.shields.io/badge/version-1.6.8-gold.svg)](PATCH_NOTES.md)
[![Go Tests](https://github.com/S73PZ3R0/aws-renew/actions/workflows/test.yml/badge.svg)](https://github.com/S73PZ3R0/aws-renew/actions/workflows/test.yml)


**A high-fidelity, high-performance DevOps utility written in Go to automatically synchronize AWS EC2 security group rules with your current public IP address.**

**Author:** S73PZ3R0 (YGNight)

> [!IMPORTANT]
> **Python version is now DEPRECATED.** Development has shifted entirely to Go for Version 1.6 to provide a single-binary, cross-platform experience with improved performance, native TUI capabilities, and broader platform support.

Built for professional terminal environments, featuring a modern "Orchestrator" TUI (powered by Bubble Tea), automated authentication fallback, and headless JSON output for CI/CD automation.

---

## 🚀 Installation (v1.6.8 Go)

### Source Installation
```bash
# Clone the repository
git clone https://github.com/S73PZ3R0/aws-renew.git
cd aws-renew

# Install to your GOPATH/bin
make install
```

### Direct Go Installation
```bash
go install github.com/S73PZ3R0/aws-renew@latest
```

### Initial Configuration
Setup your AWS credentials and notification channels using the interactive wizard:
```bash
aws-renew configure
```

### Shell Autocompletions
To enable autocompletions for your shell:

#### **Bash**
```bash
# Current session
source <(aws-renew completion bash)
# Permanent
aws-renew completion bash | sudo tee /etc/bash_completion.d/aws-renew > /dev/null
```

#### **Zsh**
```bash
# Current session
source <(aws-renew completion zsh)
# Permanent
aws-renew completion zsh > "${fpath[1]}/_aws-renew"
```

---

## 🛡️ Security & Integrity

`aws-renew` features a professional-grade security layer:
- **GPG Verification**: All official releases are signed by the author (`S73PZ3R0`).
  - **Fingerprint**: `4FD1 7220 3AEA D79E A076 668D 1F03 F4A2 861F D560`
- **Binary Integrity**: Run `./aws-renew --verify` at any time to check the binary against its GPG-signed checksum.
- **Automatic Recovery**: If `CHECKSUM.asc` is missing, the tool will automatically download the signed version from GitHub to perform verification.
- **Secure Updates**: The `--update` command extracts updates to a sandbox, verifies the GPG signature and file hashes, and only applies the update if it is **authentic and untampered**.


---

## 🛠 Usage

### 1. Interactive Orchestrator (Default)
Launch the full TUI to discover resources, select targets via keyboard, and monitor real-time synchronization.
```bash
./aws-renew
```

### 2. Automation (Headless Batch Mode)
Suppresses all UI elements and returns a structured **JSON** response. Ideal for cron jobs and pipelines.
```bash
./aws-renew --batch --cleanup
```

### 3. Daemon Mode (Background Monitoring)
Watch for IP changes and update rules automatically in the background.
```bash
./aws-renew --daemon
```

### 4. Notifications
Configure real-time alerts for security group updates:
```bash
# Telegram
aws-renew --telegram-btoken "TOKEN" --telegram-cid "ID"
# Discord
aws-renew --discord-webhook "URL"
# Slack
aws-renew --slack-webhook "URL"
```

### 5. **Integrity Verification**
Verify the integrity of the binary against the included GPG-signed `CHECKSUM.asc`.
```bash
./aws-renew --verify
```

---

## 📱 Termux Support
`aws-renew` is fully compatible with **Termux on Android**.
- Install via `go install github.com/S73PZ3R0/aws-renew@latest`, or download `aws-renew_linux_arm64.tar.gz` from the releases page.
- `--update` works normally on `linux/arm64` builds.

### Background service (runit/sv)
Requires the `termux-services` package (`pkg install termux-services`):
```bash
aws-renew service install   # creates $PREFIX/var/service/aws-renew/run
aws-renew service start
aws-renew service stop
aws-renew service status
```
Use `aws-renew --daemon` for a foreground session instead.

---

## ⚙️ Configuration
The tool automatically looks for a configuration file at `~/.config/aws-renew/config.yaml`.
See [config.yaml.example](config.yaml.example) for a full list of settings, including **Slack**, **Discord**, and **Telegram** notifications.

---

## 🧪 Development

```bash
# Generate checksums
make checksum

# Run tests
make test

# Full clean build
make clean build
```

## 📜 License
Licensed under the MIT License. See [LICENSE](LICENSE) for details.
