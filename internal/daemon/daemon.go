package daemon

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	awssvc "github.com/S73PZ3R0/aws-renew/internal/aws"
	"github.com/S73PZ3R0/aws-renew/internal/config"
	"github.com/S73PZ3R0/aws-renew/internal/network"
	"github.com/S73PZ3R0/aws-renew/internal/notify"
	"github.com/S73PZ3R0/aws-renew/internal/updater"
)

type Daemon struct {
	cfg    *config.Config
	logger *log.Logger
}

func New(cfg *config.Config) *Daemon {
	return &Daemon{cfg: cfg, logger: newLogger(cfg.LogFile)}
}

func newLogger(path string) *log.Logger {
	var w io.Writer = os.Stdout
	if path != "" {
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err == nil {
			w = io.MultiWriter(os.Stdout, f)
		}
	}
	return log.New(w, "[aws-renew] ", log.LstdFlags)
}

func (d *Daemon) Run(ctx context.Context) error {
	interval, err := time.ParseDuration(d.cfg.PollInterval)
	if err != nil || interval <= 0 {
		interval = 5 * time.Minute
	}

	notifier := notify.New(notify.Config{
		WebhookURL:     d.cfg.Notify.WebhookURL,
		TelegramToken:  d.cfg.Notify.Telegram.BotToken,
		TelegramChatID: d.cfg.Notify.Telegram.ChatID,
	})

	var lastIP string
	d.logger.Println("daemon started, poll interval:", interval)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Run immediately on start, then on every tick.
	d.poll(ctx, notifier, &lastIP)

	for {
		select {
		case <-ctx.Done():
			d.logger.Println("daemon stopping")
			return ctx.Err()
		case <-ticker.C:
			d.poll(ctx, notifier, &lastIP)
		}
	}
}

func (d *Daemon) poll(ctx context.Context, notifier *notify.Notifier, lastIP *string) {
	ip, err := network.FetchPublicIP()
	if err != nil {
		d.logger.Printf("ERROR fetching public IP: %v", err)
		return
	}
	if ip == *lastIP {
		d.logger.Printf("IP unchanged (%s), skipping update", ip)
		return
	}
	d.logger.Printf("IP change detected: %q → %q", *lastIP, ip)
	oldIP := *lastIP
	*lastIP = ip

	regions := d.resolveRegions(ctx)
	ports := d.parsePorts()

	for _, region := range regions {
		client, err := awssvc.NewClient(ctx, d.cfg.Profile, region)
		if err != nil {
			d.logger.Printf("ERROR creating client [%s]: %v", regionLabel(region), err)
			continue
		}
		ec2svc := &awssvc.EC2Service{Client: client}
		instances, err := ec2svc.ListInstances(ctx)
		if err != nil {
			d.logger.Printf("ERROR listing instances [%s]: %v", regionLabel(region), err)
			continue
		}
		filtered := filterInstances(instances, d.cfg.Instances)
		for _, inst := range filtered {
			d.updateInstance(ctx, client, inst, ports, ip, oldIP, notifier)
		}
	}
}

func (d *Daemon) updateInstance(ctx context.Context, client *awssvc.Client, inst awssvc.Instance, ports []int, newIP, oldIP string, notifier *notify.Notifier) {
	u, err := updater.New(inst, ports, []string{newIP}, client, false, d.cfg.Cleanup, d.cfg.RuleDescription)
	if err != nil {
		d.logger.Printf("ERROR building updater [%s]: %v", inst.InstanceID, err)
		return
	}
	sgSvc := &awssvc.SecurityGroupService{Client: client}
	sgIDs := make([]string, 0, len(inst.SecurityGroups))
	for _, sg := range inst.SecurityGroups {
		sgIDs = append(sgIDs, sg.GroupID)
	}
	rules, err := sgSvc.ListRulesForGroups(ctx, sgIDs)
	if err != nil {
		d.logger.Printf("ERROR listing rules [%s]: %v", inst.InstanceID, err)
		return
	}
	res, err := u.Update(ctx, rules)
	if err != nil {
		d.logger.Printf("ERROR updating [%s]: %v", inst.InstanceID, err)
		return
	}
	d.logger.Printf("OK [%s] %s: updated=%d skipped=%d revoked=%d",
		inst.InstanceID, inst.Name(), res.Updated, res.Skipped, res.Revoked)

	if notifier.Enabled() && (res.Updated > 0 || res.Revoked > 0) {
		if err := notifier.Send(notify.Event{
			Instance: inst.InstanceID,
			Name:     inst.Name(),
			Region:   inst.Region,
			OldIP:    oldIP,
			NewIP:    newIP,
			Updated:  res.Updated,
			Revoked:  res.Revoked,
			Skipped:  res.Skipped,
		}); err != nil {
			d.logger.Printf("WARN notification failed: %v", err)
		}
	}
}

func (d *Daemon) resolveRegions(ctx context.Context) []string {
	if d.cfg.Regions == "" {
		return []string{""}
	}
	if strings.ToLower(d.cfg.Regions) == "all" {
		client, err := awssvc.NewClient(ctx, d.cfg.Profile, "")
		if err != nil {
			d.logger.Printf("ERROR creating client for region enumeration: %v", err)
			return []string{""}
		}
		svc := &awssvc.EC2Service{Client: client}
		regions, err := svc.ListRegions(ctx)
		if err != nil {
			d.logger.Printf("ERROR listing regions: %v", err)
			return []string{""}
		}
		return regions
	}
	var out []string
	for _, r := range strings.Split(d.cfg.Regions, ",") {
		r = strings.TrimSpace(r)
		if r != "" {
			out = append(out, r)
		}
	}
	return out
}

func (d *Daemon) parsePorts() []int {
	if d.cfg.SSHPort == "" {
		return []int{22}
	}
	var ports []int
	for _, p := range strings.Split(d.cfg.SSHPort, ",") {
		var n int
		if _, err := fmt.Sscanf(strings.TrimSpace(p), "%d", &n); err == nil {
			ports = append(ports, n)
		}
	}
	if len(ports) == 0 {
		return []int{22}
	}
	return ports
}

func filterInstances(all []awssvc.Instance, want []string) []awssvc.Instance {
	if len(want) == 0 {
		return all
	}
	set := make(map[string]bool, len(want))
	for _, w := range want {
		set[w] = true
	}
	var out []awssvc.Instance
	for _, inst := range all {
		if set[inst.InstanceID] || set[inst.Name()] {
			out = append(out, inst)
		}
	}
	return out
}

func regionLabel(r string) string {
	if r == "" {
		return "default"
	}
	return r
}
