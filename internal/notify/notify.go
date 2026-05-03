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
}

type Notifier struct {
	cfg    Config
	client *http.Client
}

func New(cfg Config) *Notifier {
	return &Notifier{cfg: cfg, client: &http.Client{Timeout: 10 * time.Second}}
}

func (n *Notifier) Enabled() bool {
	return n.cfg.WebhookURL != "" || (n.cfg.TelegramToken != "" && n.cfg.TelegramChatID != "")
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

// escMD escapes special characters for Telegram MarkdownV2.
func escMD(s string) string {
	special := `\_*[]()~` + "`" + `>#+-=|{}.!`
	out := make([]byte, 0, len(s))
	for _, c := range []byte(s) {
		for _, sp := range []byte(special) {
			if c == sp {
				out = append(out, '\\')
				break
			}
		}
		out = append(out, c)
	}
	return string(out)
}
