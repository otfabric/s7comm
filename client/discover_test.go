package client

import (
	"context"
	"testing"
)

func TestExpandCIDR(t *testing.T) {
	ips, err := expandCIDR("192.168.1.0/30")
	if err != nil {
		t.Fatalf("expandCIDR error: %v", err)
	}
	if len(ips) != 2 {
		t.Fatalf("expected 2 host addresses, got %d", len(ips))
	}
}

func TestExpandCIDR32(t *testing.T) {
	ips, err := expandCIDR("127.0.0.1/32")
	if err != nil {
		t.Fatalf("expandCIDR error: %v", err)
	}
	if len(ips) != 1 || ips[0] != "127.0.0.1" {
		t.Fatalf("expected single host 127.0.0.1, got %v", ips)
	}
}

func TestExpandCIDRInvalid(t *testing.T) {
	if _, err := expandCIDR("bad-cidr"); err == nil {
		t.Fatal("expected invalid CIDR error")
	}
}

func TestDiscoverCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := Discover(ctx, "192.168.1.0/30")
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
}
