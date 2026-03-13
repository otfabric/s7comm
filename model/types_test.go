package model

import "testing"

func TestNumericDecoders(t *testing.T) {
	if DecodeWord([]byte{0x12, 0x34}) != 0x1234 {
		t.Fatal("DecodeWord mismatch")
	}
	if DecodeDWord([]byte{0x01, 0x02, 0x03, 0x04}) != 0x01020304 {
		t.Fatal("DecodeDWord mismatch")
	}
}

func TestStringEncodingRoundTrip(t *testing.T) {
	enc := EncodeString("HELLO", 12)
	got := DecodeString(enc)
	if got != "HELLO" {
		t.Fatalf("round trip mismatch: %q", got)
	}
}

func TestDecodeBool(t *testing.T) {
	if !DecodeBool([]byte{0x04}, 2) {
		t.Fatal("expected bit 2 set")
	}
}
