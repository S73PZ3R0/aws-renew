package svcmgr

import (
	"context"
	"fmt"
	"os"

	"github.com/S73PZ3R0/aws-renew/internal/config"
	"github.com/S73PZ3R0/aws-renew/internal/daemon"
	"github.com/kardianos/service"
)

const svcName = "aws-renew"

// IsTermux returns true when running inside Termux on Android.
func IsTermux() bool {
	return os.Getenv("TERMUX_VERSION") != ""
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

// Install registers the OS service (systemd / launchd / Windows SCM).
func Install(cfg *config.Config) error {
	if IsTermux() {
		return fmt.Errorf("service install is not available on Termux — use 'aws-renew --daemon' in a background session instead")
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
		return fmt.Errorf("service uninstall is not available on Termux")
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
		return fmt.Errorf("use 'aws-renew --daemon' on Termux")
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
		return fmt.Errorf("use Ctrl+C to stop the foreground daemon on Termux")
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
		return "Termux — managed via foreground process (aws-renew --daemon)", nil
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
