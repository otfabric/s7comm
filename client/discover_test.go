package client

import "testing"

func TestExpandCIDR(t *testing.T) {
	ips, err := expandCIDR("192.168.1.0/30")
	if err != nil {
		t.Fatalf("expandCIDR error: %v", err)
	}
	if len(ips) != 2 {
		t.Fatalf("expected 2 host addresses, got %d", len(ips))
	}
}
