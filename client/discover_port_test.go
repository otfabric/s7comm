package client

import (
	"context"
	"net"
	"strconv"
	"testing"
)

func TestIsPortOpen(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen error: %v", err)
	}
	defer func() { _ = ln.Close() }()

	port := ln.Addr().(*net.TCPAddr).Port
	if !isPortOpen(context.Background(), "127.0.0.1", port) {
		t.Fatalf("expected port %s to be open", strconv.Itoa(port))
	}
}
