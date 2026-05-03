package ui

import (
	"strings"
	"testing"

	awssvc "github.com/S73PZ3R0/aws-renew/internal/aws"
)

func TestHeader(t *testing.T) {
	out := Header("1.5.0")
	if !strings.Contains(out, "1.5.0") {
		t.Error("header missing version")
	}
	if !strings.Contains(out, "AWS_RENEWER") {
		t.Error("header missing product name")
	}
}

func TestEnvPanel(t *testing.T) {
	out := EnvPanel("1.2.3.4", 3)
	if !strings.Contains(out, "1.2.3.4") {
		t.Error("env panel missing IP")
	}
	if !strings.Contains(out, "3") {
		t.Error("env panel missing region count")
	}
}

func TestDiscoveryTree(t *testing.T) {
	instances := map[string][]awssvc.Instance{
		"us-east-1": {
			{InstanceID: "i-abc123", Tags: []awssvc.Tag{{Key: "Name", Value: "web-server"}}},
		},
	}
	out := DiscoveryTree(instances)
	if !strings.Contains(out, "i-abc123") {
		t.Error("tree missing instance ID")
	}
	if !strings.Contains(out, "web-server") {
		t.Error("tree missing instance name")
	}
	if !strings.Contains(out, "us-east-1") {
		t.Error("tree missing region")
	}
}

func TestDiscoveryTreeEmptyRegion(t *testing.T) {
	instances := map[string][]awssvc.Instance{
		"": {{InstanceID: "i-xyz", Tags: []awssvc.Tag{{Key: "Name", Value: "solo"}}}},
	}
	out := DiscoveryTree(instances)
	if !strings.Contains(out, "default") {
		t.Error("empty region should display as 'default'")
	}
}

func TestTaskGroupOrdered(t *testing.T) {
	tasks := map[string]*TaskStatus{
		"i-1": {ID: "i-1", Name: "Alpha", Status: "success", Msg: "Updated 1 rule(s)"},
		"i-2": {ID: "i-2", Name: "Beta", Status: "running", Msg: "Synchronizing..."},
		"i-3": {ID: "i-3", Name: "Gamma", Status: "error", Msg: "timeout"},
	}
	order := []string{"i-1", "i-2", "i-3"}
	out := TaskGroupOrdered(tasks, order, "⠋")

	if !strings.Contains(out, "Alpha") {
		t.Error("missing Alpha")
	}
	if !strings.Contains(out, "Beta") {
		t.Error("missing Beta")
	}
	if !strings.Contains(out, "Gamma") {
		t.Error("missing Gamma")
	}
	if !strings.Contains(out, "⠋") {
		t.Error("spinner frame missing for running task")
	}
}

func TestSummaryPanel(t *testing.T) {
	out := SummaryPanel(Stats{Success: 3, Skipped: 1, Error: 0, Revoked: 2})
	if !strings.Contains(out, "3") {
		t.Error("summary missing success count")
	}
	if !strings.Contains(out, "SESSION_COMPLETE") {
		t.Error("summary missing SESSION_COMPLETE")
	}
}

func TestExecModelInit(t *testing.T) {
	tasks := map[string]*TaskStatus{
		"i-1": {ID: "i-1", Name: "Alpha", Status: "pending", Msg: "Waiting..."},
	}
	m := NewExecModel(tasks, []string{"i-1"})
	if m.Done {
		t.Error("model should not be done on init")
	}
	if m.Init() == nil {
		t.Error("Init should return a tick command")
	}
}

func TestExecModelTaskUpdate(t *testing.T) {
	tasks := map[string]*TaskStatus{
		"i-1": {ID: "i-1", Name: "Alpha", Status: "pending", Msg: "Waiting..."},
	}
	m := NewExecModel(tasks, []string{"i-1"})

	updated, _ := m.Update(TaskUpdateMsg{ID: "i-1", Status: "success", Msg: "Done"})
	result := updated.(ExecModel)

	if result.Tasks["i-1"].Status != "success" {
		t.Errorf("expected success, got %s", result.Tasks["i-1"].Status)
	}
	if result.Tasks["i-1"].Msg != "Done" {
		t.Errorf("expected 'Done', got %s", result.Tasks["i-1"].Msg)
	}
}

func TestExecModelDone(t *testing.T) {
	tasks := map[string]*TaskStatus{}
	m := NewExecModel(tasks, nil)

	updated, _ := m.Update(ExecDoneMsg{})
	result := updated.(ExecModel)
	if !result.Done {
		t.Error("model should be done after ExecDoneMsg")
	}
}
