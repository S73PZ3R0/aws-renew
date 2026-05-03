package svcmgr

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/S73PZ3R0/aws-renew/internal/config"
	"github.com/S73PZ3R0/aws-renew/internal/daemon"
	"github.com/kardianos/service"
)

const svcName = "aws-renew"

// IsTermux returns true when running inside Termux on Android.
func IsTermux() bool {
	return os.Getenv("TERMUX_VERSION") != ""
}

// ── Termux runit/sv helpers ───────────────────────────────────────────────────

func termuxPrefix() string {
	if p := os.Getenv("PREFIX"); p != "" {
		return p
	}
	return "/data/data/com.termux/files/usr"
}

func termuxSvcDir() string {
	return filepath.Join(termuxPrefix(), "var", "service", svcName)
}

func termuxInstall(_ *config.Config) error {
	svcDir := termuxSvcDir()
	if err := os.MkdirAll(svcDir, 0755); err != nil {
		return fmt.Errorf("creating service dir: %w", err)
	}
	execPath, err := os.Executable()
	if err != nil {
		execPath = svcName
	}
	sh := filepath.Join(termuxPrefix(), "bin", "sh")
	runScript := fmt.Sprintf("#!%s\nexec %s --daemon\n", sh, execPath)
	runPath := filepath.Join(svcDir, "run")
	if err := os.WriteFile(runPath, []byte(runScript), 0755); err != nil {
		return fmt.Errorf("writing run script: %w", err)
	}
	return nil
}

func termuxUninstall(_ *config.Config) error {
	exec.Command("sv", "stop", svcName).Run() //nolint:errcheck
	return os.RemoveAll(termuxSvcDir())
}

func termuxSv(subcmd string) error {
	if _, err := exec.LookPath("sv"); err != nil {
		return fmt.Errorf("sv not found — run: pkg install termux-services")
	}
	out, err := exec.Command("sv", subcmd, svcName).CombinedOutput()
	if err != nil {
		return fmt.Errorf("sv %s: %s", subcmd, strings.TrimSpace(string(out)))
	}
	return nil
}

func termuxStatus() (string, error) {
	if _, err := exec.LookPath("sv"); err != nil {
		return "", fmt.Errorf("sv not found — run: pkg install termux-services")
	}
	out, err := exec.Command("sv", "status", svcName).Output()
	if err != nil {
		return "", fmt.Errorf("sv status: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

type program struct {
	cfg    *config.Config
	cancel context.CancelFunc
	done   chan struct{}
}

func (p *program) Start(s service.Service) error {
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	p.done = make(chan struct{})
	go func() {
		defer close(p.done)
		_ = daemon.New(p.cfg).Run(ctx)
	}()
	return nil
}

func (p *program) Stop(s service.Service) error {
	if p.cancel != nil {
		p.cancel()
	}
	if p.done != nil {
		<-p.done
	}
	return nil
}

func svcConfig() *service.Config {
	return &service.Config{
		Name:        svcName,
		DisplayName: "AWS Access Renewer",
		Description: "Keeps EC2 security group rules in sync with your current public IP.",
	}
}

func newService(cfg *config.Config) (service.Service, error) {
	return service.New(&program{cfg: cfg}, svcConfig())
}

// RunForeground runs the daemon in the foreground (--daemon flag and Termux).
func RunForeground(ctx context.Context, cfg *config.Config) error {
	return daemon.New(cfg).Run(ctx)
}

// Install registers the OS service (systemd / launchd / Windows SCM / Termux sv).
func Install(cfg *config.Config) error {
	if IsTermux() {
		return termuxInstall(cfg)
	}
	svc, err := newService(cfg)
	if err != nil {
		return err
	}
	return svc.Install()
}

// Uninstall removes the OS service registration.
func Uninstall(cfg *config.Config) error {
	if IsTermux() {
		return termuxUninstall(cfg)
	}
	svc, err := newService(cfg)
	if err != nil {
		return err
	}
	return svc.Uninstall()
}

// Start starts the installed OS service.
func Start(cfg *config.Config) error {
	if IsTermux() {
		return termuxSv("start")
	}
	svc, err := newService(cfg)
	if err != nil {
		return err
	}
	return svc.Start()
}

// Stop stops the running OS service.
func Stop(cfg *config.Config) error {
	if IsTermux() {
		return termuxSv("stop")
	}
	svc, err := newService(cfg)
	if err != nil {
		return err
	}
	return svc.Stop()
}

// Status returns a human-readable service status string.
func Status(cfg *config.Config) (string, error) {
	if IsTermux() {
		return termuxStatus()
	}
	svc, err := newService(cfg)
	if err != nil {
		return "", err
	}
	st, err := svc.Status()
	if err != nil {
		return "", err
	}
	switch st {
	case service.StatusRunning:
		return "running", nil
	case service.StatusStopped:
		return "stopped", nil
	default:
		return fmt.Sprintf("unknown (%d)", st), nil
	}
}
