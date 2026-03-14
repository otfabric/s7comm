package wire

import (
	"testing"

	"github.com/otfabric/go-cotp"
)

func TestEncodeParseCOTPCR(t *testing.T) {
	pdu, err := EncodeCOTPCR(0x0100, 0x0301)
	if err != nil {
		t.Fatalf("EncodeCOTPCR: %v", err)
	}
	dec, err := cotp.Decode(pdu)
	if err != nil {
		t.Fatalf("cotp.Decode: %v", err)
	}
	if dec.Type != cotp.TypeCR {
		t.Fatalf("unexpected pdu type: %s", dec.Type)
	}
	if dec.CR == nil {
		t.Fatal("expected CR non-nil")
	}
}

func TestBuildTSAP(t *testing.T) {
	ts := BuildTSAP(3, 0, 1)
	if ts != 0x0301 {
		t.Fatalf("unexpected tsap: 0x%04X", ts)
	}
}
