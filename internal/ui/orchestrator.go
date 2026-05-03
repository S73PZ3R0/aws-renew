package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	awssvc "github.com/S73PZ3R0/aws-renew/internal/aws"
)

type TaskStatus struct {
	ID     string
	Name   string
	Status string // pending, running, success, skipped, error
	Msg    string
}

type Stats struct {
	Success int `json:"success"`
	Skipped int `json:"skipped"`
	Error   int `json:"error"`
	Revoked int `json:"revoked"`
}

func Header(version string) string {
	title := StyleInfo.Bold(true).Render(fmt.Sprintf(" ⚡ SYSTEM.AWS_RENEWER v%s ⚡ ", version))
	return PanelStyle.Render(title) + "\n"
}

func EnvPanel(ip string, regionCount int) string {
	ipPanel := PanelStyle.Render(StyleDim.Render("IP_ADDR: ") + StyleSuccess.Render(ip))
	regPanel := PanelStyle.Render(StyleDim.Render("REGIONS: ") + StyleRegion.Render(fmt.Sprintf("%d", regionCount)))
	return lipgloss.JoinHorizontal(lipgloss.Top, ipPanel, "  ", regPanel) + "\n"
}

func DiscoveryTree(instancesByRegion map[string][]awssvc.Instance) string {
	var sb strings.Builder
	sb.WriteString(StyleBold.Render(StyleInfo.Render(" DATABASE_RESOURCES")) + "\n")
	for region, insts := range instancesByRegion {
		r := region
		if r == "" {
			r = "default"
		}
		sb.WriteString(fmt.Sprintf("  %s\n", StyleRegion.Render("» "+r)))
		for _, inst := range insts {
			sb.WriteString(fmt.Sprintf("    %s %s %s\n",
				StyleID.Render(inst.InstanceID),
				StyleDim.Render("|"),
				StyleWhite.Render(inst.Name()),
			))
		}
	}
	return PanelStyle.Render(sb.String()) + "\n"
}

// TaskGroupOrdered renders the process monitor panel in a stable display order.
func TaskGroupOrdered(tasks map[string]*TaskStatus, order []string, spinnerFrame string) string {
	var lines []string
	for _, id := range order {
		t, ok := tasks[id]
		if !ok {
			continue
		}
		icon, style := "»", StyleInfo
		switch t.Status {
		case "success":
			icon, style = "✔", StyleSuccess
		case "error":
			icon, style = "✘", StyleDanger
		case "skipped":
			icon, style = "•", StyleWarning
		case "running":
			icon, style = spinnerFrame, StyleInfo
		}
		line := style.Render(fmt.Sprintf(" %s ", icon)) +
			StyleWhite.Render(fmt.Sprintf("%-18s", t.Name)) + " " +
			StyleDim.Render(fmt.Sprintf("[%s]", t.ID)) + " " +
			style.Render("» "+t.Msg)
		lines = append(lines, line)
	}
	body := strings.Join(lines, "\n")
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorCyan).
		Padding(0, 1).
		Render(StyleBold.Render(StyleInfo.Render(" PROCESS_MONITOR "))+"\n"+body) + "\n"
}

func SummaryPanel(s Stats) string {
	row := StyleSuccess.Render(" SUCCESS: ") + StyleWhite.Render(fmt.Sprintf("%d", s.Success)) + "  " +
		StyleInfo.Render(" REVOKED: ") + StyleWhite.Render(fmt.Sprintf("%d", s.Revoked)) + "  " +
		StyleWarning.Render(" SKIPPED: ") + StyleWhite.Render(fmt.Sprintf("%d", s.Skipped)) + "  " +
		StyleDanger.Render(" ERRORS:  ") + StyleWhite.Render(fmt.Sprintf("%d", s.Error))
	return "\n" + StyleDim.Render(strings.Repeat("─", 50)) + "\n" +
		PanelStyle.Render(StyleBold.Render(StyleInfo.Render(" EXEC_REPORT "))+"\n"+row) + "\n" +
		StyleBold.Render(StyleSuccess.Render("\n » SESSION_COMPLETE \n"))
}

func WarningLine(msg string) string {
	return StyleWarning.Render("  " + msg) + "\n"
}

func InfoLine(msg string) string {
	return StyleInfo.Render("  " + msg) + "\n"
}

func SuccessLine(msg string) string {
	return StyleSuccess.Render("✔ " + msg) + "\n"
}

func DangerLine(msg string) string {
	return StyleDanger.Render("✘ " + msg) + "\n"
}
