package client

import (
	"testing"
	"time"
)

func TestWithTSAP(t *testing.T) {
	c := New("host", WithTSAP(0x0100, 0x0301))
	// Verify via connect behavior or options; we just ensure it doesn't panic and client is created
	if c == nil {
		t.Fatal("New with WithTSAP returned nil")
	}
}

func TestWithAutoRackSlot(t *testing.T) {
	c := New("host", WithAutoRackSlot(false))
	if c == nil {
		t.Fatal("New with WithAutoRackSlot(false) returned nil")
	}
	c = New("host", WithAutoRackSlot(true))
	if c == nil {
		t.Fatal("New with WithAutoRackSlot(true) returned nil")
	}
}

func TestWithRateLimit(t *testing.T) {
	c := New("host", WithRateLimit(10*time.Millisecond))
	if c == nil {
		t.Fatal("New with WithRateLimit returned nil")
	}
}

func TestWithLogger(t *testing.T) {
	c := New("host", WithLogger(nil))
	if c == nil {
		t.Fatal("New with WithLogger(nil) returned nil")
	}
	c = New("host", WithLogger(&mockLogger{}))
	if c == nil {
		t.Fatal("New with WithLogger(mock) returned nil")
	}
}

func TestWithMaxPDU(t *testing.T) {
	c := New("host", WithMaxPDU(960))
	if c == nil {
		t.Fatal("New with WithMaxPDU returned nil")
	}
}

type mockLogger struct{}

func (m *mockLogger) Debug(msg string, args ...interface{}) {}
func (m *mockLogger) Info(msg string, args ...interface{})  {}
func (m *mockLogger) Error(msg string, args ...interface{}) {}
