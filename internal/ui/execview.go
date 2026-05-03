package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type TaskUpdateMsg struct {
	ID     string
	Status string
	Msg    string
}

type ExecDoneMsg struct{}

type tickMsg time.Time

type ExecModel struct {
	Tasks map[string]*TaskStatus
	Order []string
	Done  bool
	frame int
}

func NewExecModel(tasks map[string]*TaskStatus, order []string) ExecModel {
	return ExecModel{Tasks: tasks, Order: order}
}

func (m ExecModel) Init() tea.Cmd {
	return tick()
}

func tick() tea.Cmd {
	return tea.Tick(80*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m ExecModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tickMsg:
		m.frame++
		if m.Done {
			return m, tea.Quit
		}
		return m, tick()
	case TaskUpdateMsg:
		if t, ok := m.Tasks[msg.ID]; ok {
			t.Status = msg.Status
			t.Msg = msg.Msg
		}
		return m, nil
	case ExecDoneMsg:
		m.Done = true
		return m, nil
	}
	return m, nil
}

func (m ExecModel) View() string {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	frame := frames[m.frame%len(frames)]
	return TaskGroupOrdered(m.Tasks, m.Order, frame)
}
