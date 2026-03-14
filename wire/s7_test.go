package wire

import (
	"encoding/binary"
	"testing"
)

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

func TestParseS7HeaderTooShort(t *testing.T) {
	_, _, err := ParseS7Header([]byte{0x32, 0x01, 0, 0})
	if err == nil {
		t.Fatal("expected too short error")
	}
}

func TestParseS7HeaderAck(t *testing.T) {
	// ROSCTRAck with error class/code
	h := make([]byte, 14)
	h[0] = 0x32
	h[1] = ROSCTRAck
	binary.BigEndian.PutUint16(h[6:8], 0)  // param len
	binary.BigEndian.PutUint16(h[8:10], 0) // data len
	h[10] = 0x00
	h[11] = 0x00
	parsed, _, err := ParseS7Header(h)
	if err != nil {
		t.Fatalf("ParseS7Header ack: %v", err)
	}
	if parsed.ROSCTR != ROSCTRAck || parsed.ErrorClass != 0 || parsed.ErrorCode != 0 {
		t.Fatalf("unexpected parsed ack: %+v", parsed)
	}
}

func TestParseS7HeaderAckTooShort(t *testing.T) {
	h := make([]byte, 11)
	h[0] = 0x32
	h[1] = ROSCTRAck
	_, _, err := ParseS7Header(h)
	if err == nil {
		t.Fatal("expected short ack header error")
	}
}

func TestParseS7HeaderPayloadLengthError(t *testing.T) {
	h := EncodeS7Header(ROSCTRJob, 0, 20, 10) // claims 30 bytes payload
	h = append(h, make([]byte, 5)...)         // but only 5 bytes
	_, _, err := ParseS7Header(h)
	if err == nil {
		t.Fatal("expected payload length error")
	}
}
