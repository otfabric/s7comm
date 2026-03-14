package wire

import (
	"encoding/hex"
	"os"
	"strings"
	"testing"

	"github.com/otfabric/go-cotp"
	"github.com/otfabric/go-tpkt"
)

func TestFixtureTPKTDTFrame(t *testing.T) {
	raw := loadHexFixture(t, "../testdata/frames/tpkt_dt.hex")
	f, err := tpkt.Parse(raw)
	if err != nil {
		t.Fatalf("tpkt.Parse error: %v", err)
	}
	if f.Len() != len(raw) {
		t.Fatalf("unexpected tpkt length: %d", f.Len())
	}
	dec, err := cotp.Decode(f.Payload)
	if err != nil {
		t.Fatalf("cotp.Decode error: %v", err)
	}
	if dec.Type != cotp.TypeDT {
		t.Fatalf("expected DT pdu, got %s", dec.Type)
	}
	if dec.DT == nil {
		t.Fatal("expected DT non-nil")
	}
}

func TestFixtureCOTPCCFrame(t *testing.T) {
	raw := loadHexFixture(t, "../testdata/frames/cotp_cc.hex")
	dec, err := cotp.Decode(raw)
	if err != nil {
		t.Fatalf("cotp.Decode error: %v", err)
	}
	if dec.Type != cotp.TypeCC {
		t.Fatalf("expected CC pdu, got %s", dec.Type)
	}
	if dec.CC == nil {
		t.Fatal("expected CC non-nil")
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
