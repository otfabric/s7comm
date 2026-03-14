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

func TestExpandCIDR31(t *testing.T) {
	ips, err := expandCIDR("192.168.1.0/31")
	if err != nil {
		t.Fatalf("expandCIDR: %v", err)
	}
	if len(ips) != 2 {
		t.Fatalf("expected 2 host addresses for /31, got %d", len(ips))
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

func TestDiscoverOptions(t *testing.T) {
	// Ensure option constructors don't panic and Discover accepts them
	opts := []DiscoverOption{
		WithDiscoverTimeout(1000),
		WithDiscoverParallel(5),
		WithDiscoverRackSlotRange(0, 2, 0, 3),
		WithDiscoverRateLimit(50),
	}
	ctx := context.Background()
	// Use /32 to minimize work; may still try to connect
	results, err := Discover(ctx, "127.0.0.1/32", opts...)
	if err != nil {
		t.Fatalf("Discover with options: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result for /32, got %d", len(results))
	}
}
