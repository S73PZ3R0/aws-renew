package aws

import "testing"

func TestInstanceName(t *testing.T) {
	tests := []struct {
		name     string
		instance Instance
		want     string
	}{
		{
			name: "has name tag",
			instance: Instance{Tags: []Tag{
				{Key: "Env", Value: "prod"},
				{Key: "Name", Value: "my-server"},
			}},
			want: "my-server",
		},
		{
			name:     "no tags",
			instance: Instance{},
			want:     "N/A",
		},
		{
			name: "tags without name",
			instance: Instance{Tags: []Tag{
				{Key: "Env", Value: "prod"},
			}},
			want: "N/A",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.instance.Name(); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestAWSAuthError(t *testing.T) {
	err := &AWSAuthError{"auth failed"}
	if err.Error() != "auth failed" {
		t.Errorf("unexpected error message: %s", err.Error())
	}
	// Ensure it satisfies the error interface
	var _ error = err
}

func TestAWSConfigError(t *testing.T) {
	err := &AWSConfigError{"no region"}
	if err.Error() != "no region" {
		t.Errorf("unexpected error message: %s", err.Error())
	}
	var _ error = err
}

func TestErrorTypeDistinction(t *testing.T) {
	var authErr error = &AWSAuthError{"x"}
	var confErr error = &AWSConfigError{"x"}

	if _, ok := authErr.(*AWSAuthError); !ok {
		t.Error("AWSAuthError type assertion failed")
	}
	if _, ok := confErr.(*AWSConfigError); !ok {
		t.Error("AWSConfigError type assertion failed")
	}
	if _, ok := authErr.(*AWSConfigError); ok {
		t.Error("AWSAuthError should not match AWSConfigError")
	}
}
