package main

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestParsePorts(t *testing.T) {
	tests := []struct {
		input       string
		defaultPort int
		want        []int
	}{
		{"", 22, []int{22}},
		{"22", 22, []int{22}},
		{"22,443", 22, []int{22, 443}},
		{"22, 80 , 443", 22, []int{22, 80, 443}},
		{"41337", 22, []int{41337}},
	}
	for _, tc := range tests {
		got := parsePorts(tc.input, tc.defaultPort)
		if len(got) != len(tc.want) {
			t.Errorf("parsePorts(%q) = %v, want %v", tc.input, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("parsePorts(%q)[%d] = %d, want %d", tc.input, i, got[i], tc.want[i])
			}
		}
	}
}

func TestRegionLabel(t *testing.T) {
	if regionLabel("") != "default" {
		t.Error("empty region should return 'default'")
	}
	if regionLabel("us-east-1") != "us-east-1" {
		t.Error("non-empty region should be returned as-is")
	}
}

func TestJsonErr(t *testing.T) {
	out := jsonErr("something failed", "auth_failure")
	var m map[string]string
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("jsonErr produced invalid JSON: %v", err)
	}
	if m["error"] != "something failed" {
		t.Errorf("unexpected error field: %s", m["error"])
	}
	if m["status"] != "auth_failure" {
		t.Errorf("unexpected status field: %s", m["status"])
	}
}

func TestIsAuthErr(t *testing.T) {
	if isAuthErr(nil) {
		t.Error("nil should not be an auth error")
	}

	// These come from the aws package but we test the classifier here
	// via a plain error — isAuthErr should return false for generic errors
	if isAuthErr(fmt.Errorf("some random error")) {
		t.Error("generic error should not be classified as auth error")
	}
}
