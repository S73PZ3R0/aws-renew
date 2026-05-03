package network

import "testing"

func TestNormalizeIPv4(t *testing.T) {
	got, err := NormalizeIP("1.2.3.4")
	if err != nil || got != "1.2.3.4/32" {
		t.Errorf("got %q, err %v", got, err)
	}
	got, err = NormalizeIP("1.2.3.4/24")
	if err != nil || got != "1.2.3.4/24" {
		t.Errorf("got %q, err %v", got, err)
	}
}

func TestNormalizeIPv6(t *testing.T) {
	got, err := NormalizeIP("2001:db8::1")
	if err != nil || got != "2001:db8::1/128" {
		t.Errorf("got %q, err %v", got, err)
	}
	got, err = NormalizeIP("2001:db8::/32")
	if err != nil || got != "2001:db8::/32" {
		t.Errorf("got %q, err %v", got, err)
	}
}

func TestNormalizeIPInvalid(t *testing.T) {
	_, err := NormalizeIP("not-an-ip")
	if err == nil {
		t.Error("expected error for invalid IP")
	}
}
