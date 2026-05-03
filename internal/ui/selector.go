package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	awssvc "github.com/S73PZ3R0/aws-renew/internal/aws"
)

// ── shared helpers ────────────────────────────────────────────────────────────

func fuzzyMatch(haystack, needle string) bool {
	if needle == "" {
		return true
	}
	h, n := strings.ToLower(haystack), strings.ToLower(needle)
	hi := 0
	for _, c := range n {
		found := false
		for hi < len(h) {
			if rune(h[hi]) == c {
				hi++
				found = true
				break
			}
			hi++
		}
		if !found {
			return false
		}
	}
	return true
}

func newSearchInput(placeholder string) textinput.Model {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.PromptStyle = StyleDim
	ti.TextStyle = StyleWhite
	ti.Prompt = "  🔍 "
	ti.Focus()
	return ti
}

// ── Instance selector ─────────────────────────────────────────────────────────

type instanceItem struct {
	instance awssvc.Instance
	selected bool
}

type InstanceSelectorModel struct {
	items       []instanceItem
	filtered    []int // indices into items that pass the current filter
	cursor      int   // position within filtered
	done        bool
	aborted     bool
	search      textinput.Model
	searchFocus bool
}

func NewInstanceSelector(instances []awssvc.Instance) InstanceSelectorModel {
	items := make([]instanceItem, len(instances))
	for i, inst := range instances {
		items[i] = instanceItem{instance: inst, selected: true}
	}
	m := InstanceSelectorModel{items: items, search: newSearchInput("filter by name, ID, SG…")}
	m.refilter()
	return m
}

func (m *InstanceSelectorModel) refilter() {
	q := m.search.Value()
	m.filtered = m.filtered[:0]
	for i, item := range m.items {
		inst := item.instance
		if fuzzyMatch(inst.Name(), q) || fuzzyMatch(inst.InstanceID, q) || matchSGs(inst, q) {
			m.filtered = append(m.filtered, i)
		}
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
}

func matchSGs(inst awssvc.Instance, q string) bool {
	for _, sg := range inst.SecurityGroups {
		if fuzzyMatch(sg.GroupID, q) || fuzzyMatch(sg.GroupName, q) {
			return true
		}
	}
	return false
}

func (m InstanceSelectorModel) Init() tea.Cmd { return textinput.Blink }

func (m InstanceSelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.aborted = true
			return m, tea.Quit
		case "enter":
			m.done = true
			return m, tea.Quit
		case " ":
			if len(m.filtered) > 0 {
				idx := m.filtered[m.cursor]
				m.items[idx].selected = !m.items[idx].selected
			}
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
			}
		case "ctrl+a":
			for i := range m.items {
				m.items[i].selected = true
			}
		case "ctrl+d":
			for i := range m.items {
				m.items[i].selected = false
			}
		default:
			var cmd tea.Cmd
			m.search, cmd = m.search.Update(msg)
			m.refilter()
			return m, cmd
		}
	}
	return m, nil
}

func (m InstanceSelectorModel) View() string {
	var sb strings.Builder
	sb.WriteString(StyleBold.Render(StyleInfo.Render(" RESOURCE_SELECTION")) + "\n")
	sb.WriteString(m.search.View() + "\n\n")

	if len(m.filtered) == 0 {
		sb.WriteString(StyleDim.Render("  no matches") + "\n")
	}
	for fi, idx := range m.filtered {
		item := m.items[idx]
		check := StyleInfo.Render("●")
		if !item.selected {
			check = StyleDim.Render("○")
		}
		sgs := make([]string, 0, len(item.instance.SecurityGroups))
		for _, sg := range item.instance.SecurityGroups {
			name := sg.GroupName
			if name == "" {
				name = sg.GroupID
			}
			sgs = append(sgs, name)
		}
		sgStr := StyleDim.Render("[" + strings.Join(sgs, ", ") + "]")
		label := StyleID.Render(item.instance.InstanceID) + " " +
			StyleWhite.Render(item.instance.Name()) + " " + sgStr

		if fi == m.cursor {
			sb.WriteString(lipgloss.NewStyle().Background(lipgloss.Color("#1A1A2E")).
				Render(StyleInfo.Render("» ")+label) + "\n")
		} else {
			sb.WriteString(fmt.Sprintf("  %s  %s\n", check, label))
		}
	}
	sb.WriteString("\n" + StyleDim.Render(" ↑↓:Navigate  SPACE:Toggle  CTRL+A:All  CTRL+D:None  ENTER:Confirm  ESC:Abort"))
	return PanelStyle.Render(sb.String())
}

func (m InstanceSelectorModel) Selected() []awssvc.Instance {
	var out []awssvc.Instance
	for _, item := range m.items {
		if item.selected {
			out = append(out, item.instance)
		}
	}
	return out
}

func RunInstanceSelector(instances []awssvc.Instance) ([]awssvc.Instance, bool, error) {
	m := NewInstanceSelector(instances)
	p := tea.NewProgram(m)
	final, err := p.Run()
	if err != nil {
		return nil, false, err
	}
	result := final.(InstanceSelectorModel)
	if result.aborted {
		return nil, true, nil
	}
	return result.Selected(), false, nil
}

// ── Port / rule selector ──────────────────────────────────────────────────────

type ruleItem struct {
	port        int
	sgID        string
	sgName      string
	description string
	selected    bool
}

type RuleSelectorModel struct {
	items    []ruleItem
	filtered []int
	cursor   int
	done     bool
	aborted  bool
	search   textinput.Model
}

func NewRuleSelector(rules []awssvc.SecurityGroupRule) RuleSelectorModel {
	seen := map[string]bool{}
	var items []ruleItem
	for _, r := range rules {
		key := fmt.Sprintf("%s:%d", r.GroupID, r.FromPort)
		if seen[key] || r.IsEgress || r.IpProtocol != "tcp" {
			continue
		}
		seen[key] = true
		name := r.GroupName
		if name == "" {
			name = r.GroupID
		}
		items = append(items, ruleItem{
			port: r.FromPort, sgID: r.GroupID, sgName: name,
			description: r.Description, selected: true,
		})
	}
	m := RuleSelectorModel{items: items, search: newSearchInput("filter by port, SG name, description…")}
	m.refilter()
	return m
}

func (m *RuleSelectorModel) refilter() {
	q := m.search.Value()
	m.filtered = m.filtered[:0]
	for i, item := range m.items {
		portStr := fmt.Sprintf("%d", item.port)
		if fuzzyMatch(portStr, q) || fuzzyMatch(item.sgName, q) ||
			fuzzyMatch(item.sgID, q) || fuzzyMatch(item.description, q) {
			m.filtered = append(m.filtered, i)
		}
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
}

func (m RuleSelectorModel) SelectedPorts() []int {
	seen := map[int]bool{}
	var ports []int
	for _, item := range m.items {
		if item.selected && !seen[item.port] {
			seen[item.port] = true
			ports = append(ports, item.port)
		}
	}
	return ports
}

func (m RuleSelectorModel) Init() tea.Cmd { return textinput.Blink }

func (m RuleSelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.aborted = true
			return m, tea.Quit
		case "enter":
			m.done = true
			return m, tea.Quit
		case " ":
			if len(m.filtered) > 0 {
				idx := m.filtered[m.cursor]
				m.items[idx].selected = !m.items[idx].selected
			}
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
			}
		case "ctrl+a":
			for i := range m.items {
				m.items[i].selected = true
			}
		case "ctrl+d":
			for i := range m.items {
				m.items[i].selected = false
			}
		default:
			var cmd tea.Cmd
			m.search, cmd = m.search.Update(msg)
			m.refilter()
			return m, cmd
		}
	}
	return m, nil
}

func (m RuleSelectorModel) View() string {
	var sb strings.Builder
	sb.WriteString(StyleBold.Render(StyleInfo.Render(" RULE_SELECTION")) + "\n")
	sb.WriteString(m.search.View() + "\n\n")

	if len(m.filtered) == 0 {
		sb.WriteString(StyleDim.Render("  no matches") + "\n")
	}
	for fi, idx := range m.filtered {
		item := m.items[idx]
		check := StyleInfo.Render("●")
		if !item.selected {
			check = StyleDim.Render("○")
		}
		desc := ""
		if item.description != "" {
			desc = StyleDim.Render(" — " + item.description)
		}
		label := StyleWhite.Render(fmt.Sprintf("PORT %-6d", item.port)) +
			StyleRegion.Render(item.sgName) + desc

		if fi == m.cursor {
			sb.WriteString(lipgloss.NewStyle().Background(lipgloss.Color("#1A1A2E")).
				Render(StyleInfo.Render("» ")+label) + "\n")
		} else {
			sb.WriteString(fmt.Sprintf("  %s  %s\n", check, label))
		}
	}
	sb.WriteString("\n" + StyleDim.Render(" ↑↓:Navigate  SPACE:Toggle  CTRL+A:All  CTRL+D:None  ENTER:Confirm  ESC:Abort"))
	return PanelStyle.Render(sb.String())
}

func RunRuleSelector(rules []awssvc.SecurityGroupRule) ([]int, bool, error) {
	m := NewRuleSelector(rules)
	if len(m.items) == 0 {
		return []int{22}, false, nil
	}
	p := tea.NewProgram(m)
	final, err := p.Run()
	if err != nil {
		return nil, false, err
	}
	result := final.(RuleSelectorModel)
	if result.aborted {
		return nil, true, nil
	}
	return result.SelectedPorts(), false, nil
}

// ── Port selector (simple, for batch override) ────────────────────────────────

type PortSelectorModel struct {
	items   []portItem
	cursor  int
	done    bool
	aborted bool
}

type portItem struct {
	port     int
	selected bool
}

func NewPortSelector(ports []int) PortSelectorModel {
	items := make([]portItem, len(ports))
	for i, p := range ports {
		items[i] = portItem{port: p, selected: true}
	}
	return PortSelectorModel{items: items}
}

func (m PortSelectorModel) Init() tea.Cmd { return nil }

func (m PortSelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.aborted = true
			return m, tea.Quit
		case "enter":
			m.done = true
			return m, tea.Quit
		case " ":
			m.items[m.cursor].selected = !m.items[m.cursor].selected
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		}
	}
	return m, nil
}

func (m PortSelectorModel) View() string {
	var sb strings.Builder
	sb.WriteString(StyleBold.Render(StyleInfo.Render(" PORT_SELECTION")) + "\n\n")
	for i, item := range m.items {
		check := StyleInfo.Render("●")
		if !item.selected {
			check = StyleDim.Render("○")
		}
		label := StyleWhite.Render(fmt.Sprintf("PORT %d", item.port))
		if i == m.cursor {
			sb.WriteString(lipgloss.NewStyle().Background(lipgloss.Color("#1A1A2E")).
				Render(StyleInfo.Render("» ")+label) + "\n")
		} else {
			sb.WriteString(fmt.Sprintf("  %s  %s\n", check, label))
		}
	}
	sb.WriteString("\n" + StyleDim.Render(" SPACE:Toggle | ENTER:Confirm | ESC:Abort"))
	return PanelStyle.Render(sb.String())
}

func (m PortSelectorModel) Selected() []int {
	var out []int
	for _, item := range m.items {
		if item.selected {
			out = append(out, item.port)
		}
	}
	return out
}

func RunPortSelector(ports []int) ([]int, bool, error) {
	m := NewPortSelector(ports)
	p := tea.NewProgram(m)
	final, err := p.Run()
	if err != nil {
		return nil, false, err
	}
	result := final.(PortSelectorModel)
	if result.aborted {
		return nil, true, nil
	}
	return result.Selected(), false, nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
