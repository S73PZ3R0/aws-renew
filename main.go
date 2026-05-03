package main

//go:generate make checksum

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
	"strconv"
	"strings"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	awssvc "github.com/S73PZ3R0/aws-renew/internal/aws"
	"github.com/S73PZ3R0/aws-renew/internal/config"
	"github.com/S73PZ3R0/aws-renew/internal/network"
	"github.com/S73PZ3R0/aws-renew/internal/notify"
	"github.com/S73PZ3R0/aws-renew/internal/selfupdate"
	"github.com/S73PZ3R0/aws-renew/internal/svcmgr"
	"github.com/S73PZ3R0/aws-renew/internal/ui"
	"github.com/S73PZ3R0/aws-renew/internal/updater"
	"github.com/S73PZ3R0/aws-renew/internal/verifier"
	"github.com/spf13/cobra"
)

var version = "1.6.5"

func getVersion() string {
	if version != "devel" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		if info.Main.Version != "" && info.Main.Version != "(devel)" {
			return info.Main.Version
		}
	}
	return version
}

var (
	flagConfigPath      string
	flagInstanceID      string
	flagInstanceName    string
	flagSSHPort         string
	flagSourceIP        string
	flagRegions         string
	flagProfile         string
	flagDryRun          bool
	flagCleanup         bool
	flagBatch           bool
	flagVerify          bool
	flagUpdate          bool
	flagDaemon          bool
	flagRuleDescription string
	flagTelegramToken   string
	flagTelegramChatID  string
	flagDiscordWebhook  string
	flagSlackWebhook    string
)

func loadCfg() *config.Config {
	var cfg *config.Config
	var err error
	if flagConfigPath != "" {
		cfg, err = config.LoadFrom(flagConfigPath)
	} else {
		cfg, err = config.Load()
	}
	if err != nil {
		cfg = config.Default()
	}
	// CLI flags override config file values
	if flagProfile != "" {
		cfg.Profile = flagProfile
	}
	if flagRegions != "" {
		cfg.Regions = flagRegions
	}
	if flagSSHPort != "" {
		cfg.SSHPort = flagSSHPort
	}
	if flagRuleDescription != "" {
		cfg.RuleDescription = flagRuleDescription
	}
	if flagDryRun {
		cfg.DryRun = true
	}
	if flagCleanup {
		cfg.Cleanup = true
	}
	return cfg
}

func parsePorts(s string, def int) []int {
	if s == "" {
		return []int{def}
	}
	var ports []int
	for _, p := range strings.Split(s, ",") {
		if n, err := strconv.Atoi(strings.TrimSpace(p)); err == nil {
			ports = append(ports, n)
		}
	}
	if len(ports) == 0 {
		return []int{def}
	}
	return ports
}

func run(cmd *cobra.Command, args []string) error {
	cfg := loadCfg()
	ctx := context.Background()
	currentVer := getVersion()

	// Persistence for Notification CLI flags
	updatedConfig := false
	if flagTelegramToken != "" {
		cfg.Notify.Telegram.BotToken = flagTelegramToken
		updatedConfig = true
	}
	if flagTelegramChatID != "" {
		cfg.Notify.Telegram.ChatID = flagTelegramChatID
		updatedConfig = true
	}
	if flagDiscordWebhook != "" {
		cfg.Notify.DiscordURL = flagDiscordWebhook
		updatedConfig = true
	}
	if flagSlackWebhook != "" {
		cfg.Notify.SlackURL = flagSlackWebhook
		updatedConfig = true
	}

	if updatedConfig {
		if err := config.Save(cfg); err != nil {
			fmt.Print(ui.WarningLine(fmt.Sprintf("Failed to save config: %v", err)))
		} else {
			fmt.Print(ui.SuccessLine("Notification configuration persisted to config.yaml"))
		}
	}

	if flagUpdate {
		fmt.Print(ui.InfoLine("Checking for updates..."))
		rel, err := selfupdate.LatestRelease()
		if err != nil {
			return fmt.Errorf("update check failed: %w", err)
		}
		if rel.TagName == "v"+currentVer {
			fmt.Print(ui.SuccessLine(fmt.Sprintf("Already up-to-date (%s)", currentVer)))
			return nil
		}
		fmt.Print(ui.InfoLine(fmt.Sprintf("New version found: %s (current: %s)", rel.TagName, currentVer)))
		assetURL, err := rel.AssetURL()
		if err != nil {
			return fmt.Errorf("no matching binary for your platform: %w", err)
		}
		fmt.Print(ui.InfoLine("Downloading and applying update..."))
		if err := selfupdate.Apply(assetURL); err != nil {
			return fmt.Errorf("update failed: %w", err)
		}
		fmt.Print(ui.SuccessLine(fmt.Sprintf("Updated to %s — please restart.", rel.TagName)))
		return nil
	}

	if flagVerify {
		fmt.Print(ui.InfoLine("STARTING_INTEGRITY_VERIFICATION..."))

		// Check if CHECKSUM.asc exists, if not, try to download it
		if _, err := os.Stat("CHECKSUM.asc"); os.IsNotExist(err) {
			fmt.Print(ui.InfoLine("CHECKSUM.asc missing, attempting to download..."))
			if err := selfupdate.DownloadChecksum(currentVer, "CHECKSUM.asc"); err != nil {
				fmt.Print(ui.WarningLine(fmt.Sprintf("Could not download CHECKSUM.asc: %v", err)))
			} else {
				fmt.Print(ui.SuccessLine("CHECKSUM.asc downloaded successfully"))
			}
		}

		sigOk, sigErr := verifier.VerifySignature()
		
		if sigErr != nil {
			fmt.Print(ui.WarningLine(fmt.Sprintf("SIGNATURE_WARNING: %v", sigErr)))
		} else if !sigOk {
			fmt.Print(ui.WarningLine("NO_VALID_GPG_SIGNATURE_FOUND (Integrity not cryptographically guaranteed)"))
		} else {
			fmt.Print(ui.SuccessLine("GPG_SIGNATURE_VALID (Trusted Signer: S73PZ3R0)"))
		}

		matches, mismatches, err := verifier.VerifyFiles()
		if err != nil {
			return fmt.Errorf("HASH_VERIFICATION_FAILED: %w", err)
		}
		
		if len(mismatches) == 0 {
			fmt.Print(ui.SuccessLine(fmt.Sprintf("ALL_FILES_VERIFIED (%d files match checksums)", len(matches))))
			if !sigOk {
				fmt.Println(ui.StyleDim.Render("  Note: Files are intact but the checksum file itself is unsigned."))
			}
		} else {
			fmt.Print(ui.DangerLine("INTEGRITY_FAILURE: TAMPERED_FILES_DETECTED"))
			for _, m := range mismatches {
				fmt.Println("  " + ui.StyleDim.Render("-") + " " + m)
			}
			os.Exit(1)
		}
		return nil
	}

	if flagDaemon {
		dctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		fmt.Print(ui.InfoLine("Daemon mode started (Ctrl+C to stop)"))
		return svcmgr.RunForeground(dctx, cfg)
	}

	if !flagBatch {
		fmt.Print(ui.Header(version))
		if ok, _ := verifier.VerifySignature(); !ok {
			fmt.Print(ui.WarningLine("WARNING: GPG signature missing or untrusted. Integrity not guaranteed."))
		}
	}

	for {
		ok, err := runOrchestrator(ctx, cfg)
		if err != nil {
			if flagBatch {
				fmt.Println(jsonErr(err.Error(), "internal_error"))
			}
			return err
		}
		if ok {
			break
		}
		creds, aborted := promptCredentials()
		if aborted {
			fmt.Print(ui.WarningLine("ABORTED: NO_CREDENTIALS_PROVIDED"))
			break
		}
		os.Setenv("AWS_ACCESS_KEY_ID", creds["aws_access_key_id"])
		os.Setenv("AWS_SECRET_ACCESS_KEY", creds["aws_secret_access_key"])
		os.Setenv("AWS_DEFAULT_REGION", creds["region"])
		fmt.Print(ui.SuccessLine("CREDENTIALS_APPLIED"))
		fmt.Print(ui.StyleDim.Render("Retrying operation...\n\n"))
	}
	return nil
}

func runOrchestrator(ctx context.Context, cfg *config.Config) (bool, error) {
	// 1. Resolve source IPs.
	var srcIPs []string
	if flagSourceIP != "" {
		for _, ip := range strings.Split(flagSourceIP, ",") {
			srcIPs = append(srcIPs, strings.TrimSpace(ip))
		}
	} else {
		ip, err := network.FetchPublicIP()
		if err != nil {
			return false, err
		}
		srcIPs = []string{ip}
	}
	displayIP := srcIPs[0]
	if len(srcIPs) > 1 {
		displayIP = fmt.Sprintf("%d IPs", len(srcIPs))
	}

	// 2. Resolve regions.
	regions := []string{""}
	if cfg.Regions != "" {
		if strings.ToLower(cfg.Regions) == "all" {
			client, err := awssvc.NewClient(ctx, cfg.Profile, "")
			if err != nil {
				if isAuthErr(err) {
					return false, nil
				}
				return false, err
			}
			var lerr error
			regions, lerr = (&awssvc.EC2Service{Client: client}).ListRegions(ctx)
			if lerr != nil {
				if isAuthErr(lerr) {
					return false, nil
				}
				return false, lerr
			}
		} else {
			regions = nil
			for _, r := range strings.Split(cfg.Regions, ",") {
				regions = append(regions, strings.TrimSpace(r))
			}
		}
	}

	if !flagBatch {
		fmt.Print(ui.EnvPanel(displayIP, len(regions)))
	}

	// 3. Scan instances.
	instancesByRegion := map[string][]awssvc.Instance{}
	var allInstances []awssvc.Instance
	// Keep one client per region for later use.
	clientByRegion := map[string]*awssvc.Client{}

	for _, region := range regions {
		client, err := awssvc.NewClient(ctx, cfg.Profile, region)
		if err != nil {
			if isAuthErr(err) {
				return false, nil
			}
			if !flagBatch {
				fmt.Print(ui.DangerLine(fmt.Sprintf("CLIENT_ERROR [%s]: %v", regionLabel(region), err)))
			}
			continue
		}
		clientByRegion[region] = client
		insts, err := (&awssvc.EC2Service{Client: client}).ListInstances(ctx)
		if err != nil {
			if isAuthErr(err) {
				return false, nil
			}
			if !flagBatch {
				fmt.Print(ui.DangerLine(fmt.Sprintf("REGION_ERROR [%s]: %v", regionLabel(region), err)))
			}
			continue
		}
		for _, inst := range insts {
			if flagInstanceID != "" && inst.InstanceID != flagInstanceID {
				continue
			}
			if flagInstanceName != "" && inst.Name() != flagInstanceName {
				continue
			}
			if len(cfg.Instances) > 0 && !inInstanceList(inst, cfg.Instances) {
				continue
			}
			instancesByRegion[region] = append(instancesByRegion[region], inst)
			allInstances = append(allInstances, inst)
		}
	}

	if len(allInstances) == 0 {
		if flagBatch {
			fmt.Println(jsonErr("no_targets_found", "failed"))
		} else {
			fmt.Print(ui.WarningLine("NO_TARGETS_FOUND"))
		}
		return true, nil
	}

	if !flagBatch {
		fmt.Print(ui.DiscoveryTree(instancesByRegion))
	}

	// 4. Select instances and ports.
	var selected []awssvc.Instance
	var ports []int

	if flagBatch {
		selected = allInstances
		ports = parsePorts(cfg.SSHPort, 22)
	} else {
		var aborted bool
		var err error
		selected, aborted, err = ui.RunInstanceSelector(allInstances)
		if err != nil {
			return false, err
		}
		if aborted || len(selected) == 0 {
			fmt.Print(ui.WarningLine("ABORTED: NO_RESOURCES_SELECTED"))
			return true, nil
		}

		if cfg.SSHPort != "" {
			ports = parsePorts(cfg.SSHPort, 22)
		} else {
			// Collect rules from selected instances for the fuzzy rule selector.
			var allRules []awssvc.SecurityGroupRule
			for _, inst := range selected {
				client, ok := clientByRegion[inst.Region]
				if !ok {
					continue
				}
				sgIDs := make([]string, 0, len(inst.SecurityGroups))
				for _, sg := range inst.SecurityGroups {
					sgIDs = append(sgIDs, sg.GroupID)
				}
				rules, err := (&awssvc.SecurityGroupService{Client: client}).ListRulesForGroups(ctx, sgIDs)
				if err == nil {
					allRules = append(allRules, rules...)
				}
			}

			if len(allRules) == 0 {
				ports, aborted, err = ui.RunPortSelector([]int{22})
			} else {
				ports, aborted, err = ui.RunRuleSelector(allRules)
			}
			if err != nil {
				return false, err
			}
			if aborted || len(ports) == 0 {
				fmt.Print(ui.WarningLine("ABORTED: NO_PORTS_SELECTED"))
				return true, nil
			}
		}
	}

	// 5. Execute updates.
	type batchResult struct {
		InstanceID     string   `json:"instance_id"`
		Name           string   `json:"name"`
		Region         string   `json:"region"`
		Status         string   `json:"status"`
		Details        string   `json:"details"`
		SecurityGroups []string `json:"security_groups,omitempty"`
		updated        int
		revoked        int
	}

	tasks := map[string]*ui.TaskStatus{}
	order := make([]string, 0, len(selected))
	for _, inst := range selected {
		tasks[inst.InstanceID] = &ui.TaskStatus{
			ID: inst.InstanceID, Name: inst.Name(), Status: "pending", Msg: "Waiting...",
		}
		order = append(order, inst.InstanceID)
	}

	resultsCh := make(chan batchResult, len(selected))

	execFn := func(prog *tea.Program) {
		for _, inst := range selected {
			inst := inst
			tid := inst.InstanceID
			sgLabels := make([]string, 0, len(inst.SecurityGroups))
			for _, sg := range inst.SecurityGroups {
				label := sg.GroupID
				if sg.GroupName != "" {
					label = sg.GroupName + " (" + sg.GroupID + ")"
				}
				sgLabels = append(sgLabels, label)
			}
			res := batchResult{
				InstanceID:     inst.InstanceID,
				Name:           inst.Name(),
				Region:         regionLabel(inst.Region),
				SecurityGroups: sgLabels,
			}

			if prog != nil {
				prog.Send(ui.TaskUpdateMsg{ID: tid, Status: "running", Msg: "Synchronizing..."})
			}

			client, ok := clientByRegion[inst.Region]
			if !ok {
				// Client may not exist if region scan failed; try creating it.
				var err error
				client, err = awssvc.NewClient(ctx, cfg.Profile, inst.Region)
				if err != nil {
					tasks[tid].Status = "error"
					tasks[tid].Msg = err.Error()
					res.Status, res.Details = "error", err.Error()
					if prog != nil {
						prog.Send(ui.TaskUpdateMsg{ID: tid, Status: "error", Msg: err.Error()})
					}
					resultsCh <- res
					continue
				}
			}

			u, err := updater.New(inst, ports, srcIPs, client, cfg.DryRun, cfg.Cleanup, cfg.RuleDescription)
			if err != nil {
				tasks[tid].Status = "error"
				tasks[tid].Msg = err.Error()
				res.Status, res.Details = "error", err.Error()
			} else {
				sgIDs := make([]string, 0, len(inst.SecurityGroups))
				for _, sg := range inst.SecurityGroups {
					sgIDs = append(sgIDs, sg.GroupID)
				}
				rules, err := (&awssvc.SecurityGroupService{Client: client}).ListRulesForGroups(ctx, sgIDs)
				if err != nil {
					tasks[tid].Status = "error"
					tasks[tid].Msg = err.Error()
					res.Status, res.Details = "error", err.Error()
				} else {
					result, err := u.Update(ctx, rules)
					if err != nil {
						tasks[tid].Status = "error"
						tasks[tid].Msg = err.Error()
						res.Status, res.Details = "error", err.Error()
					} else {
						res.updated = result.Updated
						res.revoked = result.Revoked
						var parts []string
						if result.Updated > 0 {
							parts = append(parts, fmt.Sprintf("Updated %d", result.Updated))
						}
						if result.Revoked > 0 {
							parts = append(parts, fmt.Sprintf("Revoked %d", result.Revoked))
						}
						if len(parts) > 0 {
							tasks[tid].Status = "success"
							tasks[tid].Msg = strings.Join(parts, " & ") + " rule(s)"
						} else {
							tasks[tid].Status = "skipped"
							tasks[tid].Msg = "Already up-to-date"
						}
						res.Status = tasks[tid].Status
						res.Details = tasks[tid].Msg

						// Notification logic
						n := notify.New(notify.Config{
							WebhookURL:     cfg.Notify.WebhookURL,
							TelegramToken:  cfg.Notify.Telegram.BotToken,
							TelegramChatID: cfg.Notify.Telegram.ChatID,
							DiscordURL:     cfg.Notify.DiscordURL,
							SlackURL:       cfg.Notify.SlackURL,
						})
						if n.Enabled() && (result.Updated > 0 || result.Revoked > 0) {
							_ = n.Send(notify.Event{
								Instance:       tid,
								Name:           inst.Name(),
								Region:         res.Region,
								NewIP:          displayIP,
								Ports:          ports,
								SecurityGroups: res.SecurityGroups,
								Updated:        result.Updated,
								Revoked:        result.Revoked,
								Skipped:        result.Skipped,
							})
						}
					}
				}
			}

			if prog != nil {
				prog.Send(ui.TaskUpdateMsg{ID: tid, Status: tasks[tid].Status, Msg: tasks[tid].Msg})
			}
			resultsCh <- res
		}
		if prog != nil {
			prog.Send(ui.ExecDoneMsg{})
		}
	}

	if !flagBatch {
		fmt.Print(ui.StyleBold.Render(ui.StyleInfo.Render("  EXECUTION_START")) + "\n")
		prog := tea.NewProgram(ui.NewExecModel(tasks, order))
		go execFn(prog)
		if _, err := prog.Run(); err != nil {
			return false, err
		}
	} else {
		execFn(nil)
	}

	close(resultsCh)

	stats := ui.Stats{}
	var batchResults []batchResult
	for r := range resultsCh {
		batchResults = append(batchResults, r)
		switch r.Status {
		case "success":
			stats.Success++
			stats.Revoked += r.revoked
		case "skipped":
			stats.Skipped++
		case "error":
			stats.Error++
		}
	}

	if !flagBatch {
		fmt.Print(ui.SummaryPanel(stats))
	} else {
		out := map[string]interface{}{
			"source_ips": srcIPs,
			"summary":    stats,
			"results":    batchResults,
		}
		b, _ := json.MarshalIndent(out, "", "  ")
		fmt.Println(string(b))
	}
	return true, nil
}

func inInstanceList(inst awssvc.Instance, want []string) bool {
	for _, w := range want {
		if inst.InstanceID == w || inst.Name() == w {
			return true
		}
	}
	return false
}

func promptCredentials() (map[string]string, bool) {
	fmt.Print(ui.DangerLine("AUTHENTICATION_REQUIRED"))
	fmt.Println(ui.StyleDim.Render(" Your AWS credentials are missing or invalid.") + "\n")

	creds := map[string]string{}

	// Use ui styling for prompts
	fmt.Print(ui.StyleBold.Render("  AWS Access Key ID: "))
	var keyID string
	fmt.Scanln(&keyID)
	if keyID == "" {
		return nil, true
	}
	creds["aws_access_key_id"] = keyID

	// Masked input for Secret Key
	fmt.Print(ui.StyleBold.Render("  AWS Secret Access Key: "))
	secret, err := ui.ReadPassword()
	if err != nil || secret == "" {
		return nil, true
	}
	fmt.Println() // Newline after masked input
	creds["aws_secret_access_key"] = secret

	creds["region"] = "us-east-1"
	fmt.Print(ui.StyleBold.Render("  Default Region [us-east-1]: "))
	var region string
	fmt.Scanln(&region)
	if region != "" {
		creds["region"] = region
	}
	fmt.Println()
	return creds, false
}

func isAuthErr(err error) bool {
	if err == nil {
		return false
	}
	_, isAuth := err.(*awssvc.AWSAuthError)
	_, isConf := err.(*awssvc.AWSConfigError)
	return isAuth || isConf
}

func regionLabel(r string) string {
	if r == "" {
		return "default"
	}
	return r
}

func jsonErr(msg, status string) string {
	b, _ := json.Marshal(map[string]string{"error": msg, "status": status})
	return string(b)
}

func configureCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "configure",
		Short: "Interactively configure the application",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := loadCfg()
			fmt.Println(ui.StyleBold.Render("\n 🛠  AWS-RENEW CONFIGURATION \n"))

			fmt.Print("AWS Profile [", cfg.Profile, "]: ")
			var profile string
			fmt.Scanln(&profile)
			if profile != "" {
				cfg.Profile = profile
			}

			fmt.Print("AWS Regions (comma-separated or 'all') [", cfg.Regions, "]: ")
			var regions string
			fmt.Scanln(&regions)
			if regions != "" {
				cfg.Regions = regions
			}

			fmt.Print("SSH Ports (comma-separated) [", cfg.SSHPort, "]: ")
			var ports string
			fmt.Scanln(&ports)
			if ports != "" {
				cfg.SSHPort = ports
			}

			fmt.Println(ui.StyleBold.Render("\n 🔔 NOTIFICATIONS"))

			fmt.Print("Telegram Bot Token: ")
			var tToken string
			fmt.Scanln(&tToken)
			if tToken != "" {
				cfg.Notify.Telegram.BotToken = tToken
			}

			fmt.Print("Telegram Chat ID: ")
			var tCID string
			fmt.Scanln(&tCID)
			if tCID != "" {
				cfg.Notify.Telegram.ChatID = tCID
			}

			fmt.Print("Discord Webhook URL: ")
			var dURL string
			fmt.Scanln(&dURL)
			if dURL != "" {
				cfg.Notify.DiscordURL = dURL
			}

			fmt.Print("Slack Webhook URL: ")
			var sURL string
			fmt.Scanln(&sURL)
			if sURL != "" {
				cfg.Notify.SlackURL = sURL
			}

			if err := config.Save(cfg); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}

			fmt.Println(ui.SuccessLine("\nConfiguration saved successfully!"))
			return nil
		},
	}
}

func main() {
	rootCmd := &cobra.Command{
		Use:     "aws-renew",
		Short:   "Keep EC2 security group rules in sync with your current IP",
		Version: getVersion(),
		RunE:    run,
	}

	f := rootCmd.Flags()
	f.StringVar(&flagConfigPath, "config", "", fmt.Sprintf("Config file (default: %s)", config.Path()))
	f.StringVarP(&flagInstanceID, "instance-id", "i", "", "Filter by EC2 instance ID")
	f.StringVarP(&flagInstanceName, "instance-name", "n", "", "Filter by EC2 Name tag")
	f.StringVarP(&flagSSHPort, "ssh-port", "p", "", "Ports to manage, comma-separated (default: 22)")
	f.StringVar(&flagSourceIP, "source-ip", "", "Override public IP detection (comma-separated)")
	f.StringVar(&flagRegions, "regions", "", "Comma-separated regions or 'all'")
	f.StringVar(&flagProfile, "profile", "", "AWS profile name")
	f.BoolVarP(&flagDryRun, "dry-run", "d", false, "Preview changes without applying")
	f.BoolVar(&flagCleanup, "cleanup", false, "Remove excess stale rules")
	f.BoolVarP(&flagBatch, "batch", "b", false, "Non-interactive mode with JSON output")
	f.BoolVar(&flagVerify, "verify", false, "Verify code integrity against CHECKSUM.asc")
	f.BoolVar(&flagUpdate, "update", false, "Update to the latest GitHub release")
	f.BoolVar(&flagDaemon, "daemon", false, "Run in watch/daemon mode (foreground)")
	f.StringVar(&flagRuleDescription, "rule-description", "", "Description tag for managed rules")
	f.StringVar(&flagTelegramToken, "telegram-btoken", "", "Set Telegram Bot Token and save to config")
	f.StringVar(&flagTelegramChatID, "telegram-cid", "", "Set Telegram Chat ID and save to config")
	f.StringVar(&flagDiscordWebhook, "discord-webhook", "", "Set Discord Webhook URL and save to config")
	f.StringVar(&flagSlackWebhook, "slack-webhook", "", "Set Slack Webhook URL and save to config")

	rootCmd.AddCommand(configureCmd())

	serviceCmd := &cobra.Command{
		Use:   "service",
		Short: "Manage the aws-renew OS service",
	}
	serviceCmd.AddCommand(
		&cobra.Command{
			Use:   "install",
			Short: "Install aws-renew as a system service",
			RunE: func(cmd *cobra.Command, args []string) error {
				if err := svcmgr.Install(loadCfg()); err != nil {
					return err
				}
				fmt.Print(ui.SuccessLine("Service installed successfully."))
				return nil
			},
		},
		&cobra.Command{
			Use:   "uninstall",
			Short: "Uninstall the system service",
			RunE: func(cmd *cobra.Command, args []string) error {
				if err := svcmgr.Uninstall(loadCfg()); err != nil {
					return err
				}
				fmt.Print(ui.SuccessLine("Service uninstalled."))
				return nil
			},
		},
		&cobra.Command{
			Use:   "start",
			Short: "Start the system service",
			RunE: func(cmd *cobra.Command, args []string) error {
				if err := svcmgr.Start(loadCfg()); err != nil {
					return err
				}
				fmt.Print(ui.SuccessLine("Service started."))
				return nil
			},
		},
		&cobra.Command{
			Use:   "stop",
			Short: "Stop the system service",
			RunE: func(cmd *cobra.Command, args []string) error {
				if err := svcmgr.Stop(loadCfg()); err != nil {
					return err
				}
				fmt.Print(ui.SuccessLine("Service stopped."))
				return nil
			},
		},
		&cobra.Command{
			Use:   "status",
			Short: "Show service status",
			RunE: func(cmd *cobra.Command, args []string) error {
				st, err := svcmgr.Status(loadCfg())
				if err != nil {
					return err
				}
				fmt.Print(ui.InfoLine("Service status: " + st))
				return nil
			},
		},
	)
	rootCmd.AddCommand(serviceCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
