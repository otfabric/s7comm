package wire

import "testing"

func TestEncodeParseS7Header(t *testing.T) {
	h := EncodeS7Header(ROSCTRJob, 7, 12, 4)
	h = append(h, make([]byte, 16)...)
	parsed, rest, err := ParseS7Header(h)
	if err != nil {
		t.Fatalf("ParseS7Header error: %v", err)
	}
	if parsed.PDURef != 7 {
		t.Fatalf("unexpected pdu ref: %d", parsed.PDURef)
	}
	if parsed.ParamLength != 12 || parsed.DataLength != 4 {
		t.Fatalf("unexpected lengths: %d/%d", parsed.ParamLength, parsed.DataLength)
	}
	if len(rest) != 16 {
		t.Fatalf("expected rest length 16, got %d", len(rest))
	}
}

func TestParseS7HeaderRejectsInvalidProtocol(t *testing.T) {
	_, _, err := ParseS7Header([]byte{0x31, 0x01, 0, 0, 0, 0, 0, 0, 0, 0})
	if err == nil {
		t.Fatal("expected invalid protocol error")
	}
}
