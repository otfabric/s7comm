package wire

import (
	"testing"

	"github.com/otfabric/go-cotp"
	"github.com/otfabric/go-tpkt"
)

func TestInspectFrameTPKTOnly(t *testing.T) {
	dtBytes, err := EncodeCOTPDT(nil)
	if err != nil {
		t.Fatalf("EncodeCOTPDT: %v", err)
	}
	frame, err := tpkt.Encode(dtBytes)
	if err != nil {
		t.Fatalf("tpkt.Encode: %v", err)
	}
	s, err := InspectFrame(frame)
	if err != nil {
		t.Fatalf("InspectFrame error: %v", err)
	}
	if s.COTPType != byte(cotp.TypeDT) {
		t.Fatalf("unexpected COTP type: 0x%02X", s.COTPType)
	}
	if s.ROSCTR != 0 {
		t.Fatalf("expected no S7 ROSCTR, got 0x%02X", s.ROSCTR)
	}
}

func TestInspectFrameWithS7(t *testing.T) {
	s7 := EncodeS7Header(ROSCTRJob, 1, 1, 0)
	s7 = append(s7, FuncReadVar)
	dtBytes, err := EncodeCOTPDT(s7)
	if err != nil {
		t.Fatalf("EncodeCOTPDT: %v", err)
	}
	frame, err := tpkt.Encode(dtBytes)
	if err != nil {
		t.Fatalf("tpkt.Encode: %v", err)
	}
	s, err := InspectFrame(frame)
	if err != nil {
		t.Fatalf("InspectFrame error: %v", err)
	}
	if s.ROSCTR != byte(ROSCTRJob) {
		t.Fatalf("unexpected ROSCTR: 0x%02X", s.ROSCTR)
	}
	if s.Function != FuncReadVar {
		t.Fatalf("unexpected function: 0x%02X", s.Function)
	}
}

func FuzzInspectFrame(f *testing.F) {
	dtBytes, _ := EncodeCOTPDT(EncodeS7Header(ROSCTRJob, 1, 2, 0))
	frame, _ := tpkt.Encode(dtBytes)
	f.Add(frame)
	f.Fuzz(func(t *testing.T, frame []byte) {
		_, _ = InspectFrame(frame)
	})
}
