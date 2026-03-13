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

func TestDecodeBoolNegativeBit(t *testing.T) {
	if DecodeBool([]byte{0xFF}, -1) {
		t.Fatal("expected false for negative bit index")
	}
}

func TestEncodeStringBounds(t *testing.T) {
	small := EncodeString("A", 1)
	if len(small) != 2 {
		t.Fatalf("expected minimum encoded length 2, got %d", len(small))
	}
	if small[0] != 0 || small[1] != 0 {
		t.Fatalf("expected empty metadata for too-small max len, got [%d %d]", small[0], small[1])
	}

	large := EncodeString("HELLO", 300)
	if len(large) != 256 {
		t.Fatalf("expected clamped encoded length 256, got %d", len(large))
	}
	if large[0] != 254 {
		t.Fatalf("expected max string length marker 254, got %d", large[0])
	}
}

func TestEncodeStringTruncatesValue(t *testing.T) {
	encoded := EncodeString("ABCDEFGHIJ", 8)
	if encoded[0] != 6 {
		t.Fatalf("expected max payload marker 6, got %d", encoded[0])
	}
	if encoded[1] != 6 {
		t.Fatalf("expected actual payload length 6, got %d", encoded[1])
	}
	if got := DecodeString(encoded); got != "ABCDEF" {
		t.Fatalf("expected truncated payload ABCDEF, got %q", got)
	}
}
