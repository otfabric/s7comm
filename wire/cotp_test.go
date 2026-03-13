package wire

import "testing"

func TestEncodeParseCOTPCR(t *testing.T) {
	pdu := EncodeCOTPCR(0x0100, 0x0301)
	c, _, err := ParseCOTP(pdu)
	if err != nil {
		t.Fatalf("ParseCOTP error: %v", err)
	}
	if c.PDUType != COTPTypeCR {
		t.Fatalf("unexpected pdu type: 0x%02X", c.PDUType)
	}
}

func TestBuildTSAP(t *testing.T) {
	ts := BuildTSAP(3, 0, 1)
	if ts != 0x0301 {
		t.Fatalf("unexpected tsap: 0x%04X", ts)
	}
}
