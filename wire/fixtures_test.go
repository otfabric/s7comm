package wire

import (
	"encoding/hex"
	"os"
	"strings"
	"testing"
)

func TestFixtureTPKTDTFrame(t *testing.T) {
	raw := loadHexFixture(t, "../testdata/frames/tpkt_dt.hex")
	tpkt, payload, err := ParseTPKT(raw)
	if err != nil {
		t.Fatalf("ParseTPKT error: %v", err)
	}
	if tpkt.Length != uint16(len(raw)) {
		t.Fatalf("unexpected tpkt length: %d", tpkt.Length)
	}
	c, rest, err := ParseCOTP(payload)
	if err != nil {
		t.Fatalf("ParseCOTP error: %v", err)
	}
	if c.PDUType != COTPTypeDT {
		t.Fatalf("expected DT pdu, got 0x%02X", c.PDUType)
	}
	if len(rest) != 0 {
		t.Fatalf("expected no rest, got %d", len(rest))
	}
}

func TestFixtureCOTPCCFrame(t *testing.T) {
	raw := loadHexFixture(t, "../testdata/frames/cotp_cc.hex")
	c, rest, err := ParseCOTP(raw)
	if err != nil {
		t.Fatalf("ParseCOTP error: %v", err)
	}
	if c.PDUType != COTPTypeCC {
		t.Fatalf("expected CC pdu, got 0x%02X", c.PDUType)
	}
	if len(rest) != 0 {
		t.Fatalf("expected no rest, got %d", len(rest))
	}
}

func loadHexFixture(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}
	s := strings.TrimSpace(string(b))
	raw, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("decode fixture %s: %v", path, err)
	}
	return raw
}
