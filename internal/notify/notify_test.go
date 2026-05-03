package notify

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

// testTransport captures HTTP requests without making real network calls.
type testTransport struct {
	status int // 0 → 200
	reqs   []*http.Request
	bodies [][]byte
}

func (t *testTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	b, _ := io.ReadAll(req.Body)
	clone := req.Clone(req.Context())
	t.reqs = append(t.reqs, clone)
	t.bodies = append(t.bodies, b)
	status := t.status
	if status == 0 {
		status = http.StatusOK
	}
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader("")),
		Header:     make(http.Header),
	}, nil
}

func newTestNotifier(cfg Config, status int) (*Notifier, *testTransport) {
	tr := &testTransport{status: status}
	n := New(cfg)
	n.client = &http.Client{Transport: tr}
	return n, tr
}

func baseEvent() Event {
	return Event{
		Instance: "i-1234567890abcdef0",
		Name:     "prod-server",
		Region:   "us-east-1",
		NewIP:    "1.2.3.4/32",
		Updated:  2,
		Revoked:  1,
		Skipped:  0,
	}
}

// ── Enabled ───────────────────────────────────────────────────────────────────

func TestEnabled(t *testing.T) {
	cases := []struct {
		name string
		cfg  Config
		want bool
	}{
		{"empty", Config{}, false},
		{"webhook only", Config{WebhookURL: "http://x"}, true},
		{"discord only", Config{DiscordURL: "http://x"}, true},
		{"slack only", Config{SlackURL: "http://x"}, true},
		{"telegram token only", Config{TelegramToken: "tok"}, false},
		{"telegram both", Config{TelegramToken: "tok", TelegramChatID: "123"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := New(tc.cfg).Enabled(); got != tc.want {
				t.Errorf("Enabled() = %v, want %v", got, tc.want)
			}
		})
	}
}

// ── Webhook ───────────────────────────────────────────────────────────────────

func TestWebhookPayload(t *testing.T) {
	ev := baseEvent()
	n, tr := newTestNotifier(Config{WebhookURL: "http://test/hook"}, 0)
	if err := n.Send(ev); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if len(tr.bodies) != 1 {
		t.Fatalf("expected 1 request, got %d", len(tr.bodies))
	}
	var got Event
	if err := json.Unmarshal(tr.bodies[0], &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Instance != ev.Instance {
		t.Errorf("Instance: got %q, want %q", got.Instance, ev.Instance)
	}
	if got.NewIP != ev.NewIP {
		t.Errorf("NewIP: got %q, want %q", got.NewIP, ev.NewIP)
	}
	if got.Updated != ev.Updated || got.Revoked != ev.Revoked {
		t.Errorf("counts: got Updated=%d Revoked=%d, want %d/%d", got.Updated, got.Revoked, ev.Updated, ev.Revoked)
	}
}

func TestWebhookPortsAndSGsInPayload(t *testing.T) {
	ev := baseEvent()
	ev.Ports = []int{22, 443}
	ev.SecurityGroups = []string{"sg-prod (sg-abc123)"}
	n, tr := newTestNotifier(Config{WebhookURL: "http://test/hook"}, 0)
	n.Send(ev)
	var got Event
	json.Unmarshal(tr.bodies[0], &got)
	if len(got.Ports) != 2 || got.Ports[0] != 22 {
		t.Errorf("Ports: got %v", got.Ports)
	}
	if len(got.SecurityGroups) != 1 || got.SecurityGroups[0] != "sg-prod (sg-abc123)" {
		t.Errorf("SecurityGroups: got %v", got.SecurityGroups)
	}
}

func TestWebhookHTTPError(t *testing.T) {
	n, _ := newTestNotifier(Config{WebhookURL: "http://test/hook"}, http.StatusInternalServerError)
	if err := n.Send(baseEvent()); err == nil {
		t.Error("expected error for HTTP 500, got nil")
	}
}

// ── Discord ───────────────────────────────────────────────────────────────────

func TestDiscordPayload(t *testing.T) {
	ev := baseEvent()
	ev.Ports = []int{22, 443}
	ev.SecurityGroups = []string{"sg-web (sg-abc123)"}
	n, tr := newTestNotifier(Config{DiscordURL: "http://test/discord"}, 0)
	if err := n.Send(ev); err != nil {
		t.Fatalf("Send: %v", err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(tr.bodies[0], &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	embeds := payload["embeds"].([]interface{})
	embed := embeds[0].(map[string]interface{})
	if embed["title"] != "🔐 AWS Access Renewer" {
		t.Errorf("unexpected title: %v", embed["title"])
	}
	fields := embed["fields"].([]interface{})
	fm := make(map[string]string)
	for _, f := range fields {
		m := f.(map[string]interface{})
		fm[m["name"].(string)] = m["value"].(string)
	}
	if !strings.Contains(fm["Ports"], "22") || !strings.Contains(fm["Ports"], "443") {
		t.Errorf("Ports field: %q", fm["Ports"])
	}
	if !strings.Contains(fm["Security Groups"], "sg-web") {
		t.Errorf("Security Groups field: %q", fm["Security Groups"])
	}
	if !strings.Contains(fm["Result"], "Updated") {
		t.Errorf("Result field: %q", fm["Result"])
	}
}

func TestDiscordOmitsPortsAndSGsWhenEmpty(t *testing.T) {
	n, tr := newTestNotifier(Config{DiscordURL: "http://test/discord"}, 0)
	n.Send(baseEvent())
	var payload map[string]interface{}
	json.Unmarshal(tr.bodies[0], &payload)
	embeds := payload["embeds"].([]interface{})
	fields := embeds[0].(map[string]interface{})["fields"].([]interface{})
	for _, f := range fields {
		name := f.(map[string]interface{})["name"].(string)
		if name == "Ports" || name == "Security Groups" {
			t.Errorf("unexpected field %q when event has no ports/SGs", name)
		}
	}
}

func TestDiscordOldIPTransition(t *testing.T) {
	ev := baseEvent()
	ev.OldIP = "9.9.9.9/32"
	n, tr := newTestNotifier(Config{DiscordURL: "http://test/discord"}, 0)
	n.Send(ev)
	var payload map[string]interface{}
	json.Unmarshal(tr.bodies[0], &payload)
	embeds := payload["embeds"].([]interface{})
	fields := embeds[0].(map[string]interface{})["fields"].([]interface{})
	fm := make(map[string]string)
	for _, f := range fields {
		m := f.(map[string]interface{})
		fm[m["name"].(string)] = m["value"].(string)
	}
	if !strings.Contains(fm["IP Address"], "9.9.9.9") || !strings.Contains(fm["IP Address"], "1.2.3.4") {
		t.Errorf("IP Address field missing transition: %q", fm["IP Address"])
	}
}

// ── Slack ─────────────────────────────────────────────────────────────────────

func TestSlackPayload(t *testing.T) {
	ev := baseEvent()
	ev.Ports = []int{22}
	ev.SecurityGroups = []string{"sg-prod (sg-xyz)"}
	n, tr := newTestNotifier(Config{SlackURL: "http://test/slack"}, 0)
	if err := n.Send(ev); err != nil {
		t.Fatalf("Send: %v", err)
	}
	var payload map[string]interface{}
	json.Unmarshal(tr.bodies[0], &payload)
	text := payload["text"].(string)
	for _, want := range []string{"prod-server", "us-east-1", "1.2.3.4", "22", "sg-prod"} {
		if !strings.Contains(text, want) {
			t.Errorf("Slack text missing %q: %s", want, text)
		}
	}
}

func TestSlackOldIPTransition(t *testing.T) {
	ev := baseEvent()
	ev.OldIP = "9.9.9.9/32"
	n, tr := newTestNotifier(Config{SlackURL: "http://test/slack"}, 0)
	n.Send(ev)
	var payload map[string]interface{}
	json.Unmarshal(tr.bodies[0], &payload)
	text := payload["text"].(string)
	if !strings.Contains(text, "9.9.9.9/32") || !strings.Contains(text, "1.2.3.4/32") {
		t.Errorf("old→new IP not in Slack text: %q", text)
	}
}

func TestSlackOmitsPortsAndSGsWhenEmpty(t *testing.T) {
	n, tr := newTestNotifier(Config{SlackURL: "http://test/slack"}, 0)
	n.Send(baseEvent())
	var payload map[string]interface{}
	json.Unmarshal(tr.bodies[0], &payload)
	text := payload["text"].(string)
	if strings.Contains(text, "Ports:") || strings.Contains(text, "Security Groups:") {
		t.Errorf("Slack text should not contain Ports/Security Groups when empty: %q", text)
	}
}

// ── Telegram ──────────────────────────────────────────────────────────────────

func withTelegramBase(t *testing.T, base string) func() {
	t.Helper()
	orig := telegramAPIBase
	telegramAPIBase = base
	return func() { telegramAPIBase = orig }
}

func TestTelegramPayload(t *testing.T) {
	defer withTelegramBase(t, "http://test")()
	ev := baseEvent()
	ev.Ports = []int{22}
	ev.SecurityGroups = []string{"sg-abc"}
	n, tr := newTestNotifier(Config{TelegramToken: "mytoken", TelegramChatID: "999"}, 0)
	if err := n.Send(ev); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !strings.Contains(tr.reqs[0].URL.Path, "mytoken") {
		t.Errorf("URL missing token: %v", tr.reqs[0].URL)
	}
	var payload map[string]interface{}
	json.Unmarshal(tr.bodies[0], &payload)
	if payload["chat_id"] != "999" {
		t.Errorf("chat_id: got %v, want 999", payload["chat_id"])
	}
	if payload["parse_mode"] != "MarkdownV2" {
		t.Errorf("parse_mode: got %v", payload["parse_mode"])
	}
	text := payload["text"].(string)
	if !strings.Contains(text, "prod\\-server") && !strings.Contains(text, "prod-server") {
		t.Errorf("text missing instance name: %q", text)
	}
	if !strings.Contains(text, "22") {
		t.Errorf("text missing port 22: %q", text)
	}
	if !strings.Contains(text, "sg") {
		t.Errorf("text missing security group: %q", text)
	}
}

func TestTelegramHTTPError(t *testing.T) {
	defer withTelegramBase(t, "http://test")()
	n, _ := newTestNotifier(Config{TelegramToken: "tok", TelegramChatID: "1"}, http.StatusBadRequest)
	if err := n.Send(baseEvent()); err == nil {
		t.Error("expected error for HTTP 400, got nil")
	}
}

func TestTelegramOmitsPortsAndSGsWhenEmpty(t *testing.T) {
	defer withTelegramBase(t, "http://test")()
	n, tr := newTestNotifier(Config{TelegramToken: "tok", TelegramChatID: "1"}, 0)
	n.Send(baseEvent())
	var payload map[string]interface{}
	json.Unmarshal(tr.bodies[0], &payload)
	text := payload["text"].(string)
	if strings.Contains(text, "Ports") || strings.Contains(text, "Groups") {
		t.Errorf("Telegram text should not contain Ports/Groups when empty: %q", text)
	}
}

// ── Multi-channel ─────────────────────────────────────────────────────────────

func TestSendAllChannels(t *testing.T) {
	defer withTelegramBase(t, "http://test")()
	n, tr := newTestNotifier(Config{
		WebhookURL:     "http://test/hook",
		DiscordURL:     "http://test/discord",
		SlackURL:       "http://test/slack",
		TelegramToken:  "tok",
		TelegramChatID: "1",
	}, 0)
	if err := n.Send(baseEvent()); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if len(tr.reqs) != 4 {
		t.Errorf("expected 4 requests, got %d", len(tr.reqs))
	}
}

func TestSendCollectsAllErrors(t *testing.T) {
	defer withTelegramBase(t, "http://test")()
	n, _ := newTestNotifier(Config{
		WebhookURL: "http://test/hook",
		DiscordURL: "http://test/discord",
	}, http.StatusInternalServerError)
	err := n.Send(baseEvent())
	if err == nil {
		t.Fatal("expected combined error, got nil")
	}
	if !strings.Contains(err.Error(), "webhook") || !strings.Contains(err.Error(), "discord") {
		t.Errorf("error should mention both channels: %v", err)
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func TestFormatPorts(t *testing.T) {
	if got := formatPorts([]int{22}); got != "22" {
		t.Errorf("got %q, want %q", got, "22")
	}
	if got := formatPorts([]int{22, 443, 8080}); got != "22, 443, 8080" {
		t.Errorf("got %q, want %q", got, "22, 443, 8080")
	}
}

func TestEscMD(t *testing.T) {
	cases := []struct{ in, wantContains string }{
		{"us-east-1", `us\-east\-1`},
		{"hello.world", `hello\.world`},
		{"snake_case", `snake\_case`},
		{"plain", "plain"},
	}
	for _, tc := range cases {
		got := escMD(tc.in)
		if !strings.Contains(got, tc.wantContains) {
			t.Errorf("escMD(%q) = %q, want it to contain %q", tc.in, got, tc.wantContains)
		}
	}
}
