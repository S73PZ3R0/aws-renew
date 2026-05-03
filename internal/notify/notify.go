package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Event struct {
	Instance string `json:"instance"`
	Name     string `json:"name"`
	Region   string `json:"region"`
	OldIP    string `json:"old_ip,omitempty"`
	NewIP    string `json:"new_ip"`
	Updated  int    `json:"updated"`
	Revoked  int    `json:"revoked"`
	Skipped  int    `json:"skipped"`
}

type Config struct {
	WebhookURL     string
	TelegramToken  string
	TelegramChatID string
	DiscordURL     string
	SlackURL       string
}

type Notifier struct {
	cfg    Config
	client *http.Client
}

func New(cfg Config) *Notifier {
	return &Notifier{cfg: cfg, client: &http.Client{Timeout: 10 * time.Second}}
}

func (n *Notifier) Enabled() bool {
	return n.cfg.WebhookURL != "" ||
		(n.cfg.TelegramToken != "" && n.cfg.TelegramChatID != "") ||
		n.cfg.DiscordURL != "" ||
		n.cfg.SlackURL != ""
}

func (n *Notifier) Send(ev Event) error {
	var errs []string
	if n.cfg.WebhookURL != "" {
		if err := n.webhook(ev); err != nil {
			errs = append(errs, fmt.Sprintf("webhook: %v", err))
		}
	}
	if n.cfg.TelegramToken != "" && n.cfg.TelegramChatID != "" {
		if err := n.telegram(ev); err != nil {
			errs = append(errs, fmt.Sprintf("telegram: %v", err))
		}
	}
	if n.cfg.DiscordURL != "" {
		if err := n.discord(ev); err != nil {
			errs = append(errs, fmt.Sprintf("discord: %v", err))
		}
	}
	if n.cfg.SlackURL != "" {
		if err := n.slack(ev); err != nil {
			errs = append(errs, fmt.Sprintf("slack: %v", err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("notification errors: %v", errs)
	}
	return nil
}

func (n *Notifier) webhook(ev Event) error {
	body, _ := json.Marshal(ev)
	resp, err := n.client.Post(n.cfg.WebhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}

func (n *Notifier) discord(ev Event) error {
	ipInfo := ev.NewIP
	if ev.OldIP != "" {
		ipInfo = fmt.Sprintf("%s → %s", ev.OldIP, ev.NewIP)
	}
	payload := map[string]interface{}{
		"content": "",
		"embeds": []map[string]interface{}{
			{
				"title": "🔐 AWS Access Renewer",
				"color": 5814783,
				"fields": []map[string]interface{}{
					{"name": "Instance", "value": fmt.Sprintf("`%s` (%s)", ev.Name, ev.Instance), "inline": false},
					{"name": "Region", "value": fmt.Sprintf("`%s`", ev.Region), "inline": true},
					{"name": "IP Address", "value": fmt.Sprintf("`%s`", ipInfo), "inline": true},
					{"name": "Result", "value": fmt.Sprintf("✅ %d Updated | 🗑 %d Revoked | ⏭ %d Skipped", ev.Updated, ev.Revoked, ev.Skipped), "inline": false},
				},
				"footer": map[string]string{"text": "aws-renew security event"},
			},
		},
	}
	body, _ := json.Marshal(payload)
	resp, err := n.client.Post(n.cfg.DiscordURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (n *Notifier) slack(ev Event) error {
	ipInfo := ev.NewIP
	if ev.OldIP != "" {
		ipInfo = fmt.Sprintf("%s → %s", ev.OldIP, ev.NewIP)
	}
	text := fmt.Sprintf(
		"*🔐 AWS Access Renewer*\n"+
			"• *Instance:* `%s` (%s)\n"+
			"• *Region:* `%s`\n"+
			"• *IP:* `%s`\n"+
			"• *Result:* %d Updated, %d Revoked, %d Skipped",
		ev.Name, ev.Instance, ev.Region, ipInfo,
		ev.Updated, ev.Revoked, ev.Skipped,
	)
	payload := map[string]interface{}{"text": text}
	body, _ := json.Marshal(payload)
	resp, err := n.client.Post(n.cfg.SlackURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (n *Notifier) telegram(ev Event) error {
	ipInfo := ev.NewIP
	if ev.OldIP != "" {
		ipInfo = fmt.Sprintf("%s → %s", ev.OldIP, ev.NewIP)
	}
	text := fmt.Sprintf(
		"🔐 *AWS Access Renewer*\n\n"+
			"📦 Instance: `%s` \\(%s\\)\n"+
			"🌍 Region: `%s`\n"+
			"🌐 IP: `%s`\n\n"+
			"✅ Updated: %d  🗑 Revoked: %d  ⏭ Skipped: %d",
		escMD(ev.Name), escMD(ev.Instance), escMD(ev.Region), escMD(ipInfo),
		ev.Updated, ev.Revoked, ev.Skipped,
	)
	payload := map[string]interface{}{
		"chat_id":    n.cfg.TelegramChatID,
		"text":       text,
		"parse_mode": "MarkdownV2",
	}
	body, _ := json.Marshal(payload)
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", n.cfg.TelegramToken)
	resp, err := n.client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}

func escMD(s string) string {
	special := `\_*[]()~` + "`" + `>#+-=|{}.!`
	out := make([]byte, 0, len(s))
	for _, c := range []byte(s) {
		matched := false
		for _, sp := range []byte(special) {
			if c == sp {
				out = append(out, '\\')
				out = append(out, c)
				matched = true
				break
			}
		}
		if !matched {
			out = append(out, c)
		}
	}
	return string(out)
}
