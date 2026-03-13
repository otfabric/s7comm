package wire

import "testing"

func TestEncodeParseTPKT(t *testing.T) {
	payload := []byte{0x11, 0x22, 0x33}
	frame := EncodeTPKT(payload)

	tpkt, parsed, err := ParseTPKT(frame)
	if err != nil {
		t.Fatalf("ParseTPKT error: %v", err)
	}
	if tpkt.Version != TPKTVersion {
		t.Fatalf("unexpected version: %d", tpkt.Version)
	}
	if len(parsed) != len(payload) {
		t.Fatalf("payload length mismatch: %d != %d", len(parsed), len(payload))
	}
}

func TestParseTPKTRejectsShortData(t *testing.T) {
	_, _, err := ParseTPKT([]byte{0x03, 0x00, 0x00})
	if err == nil {
		t.Fatal("expected error for short data")
	}
}
