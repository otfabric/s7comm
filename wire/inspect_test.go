package wire

import "testing"

func TestInspectFrameTPKTOnly(t *testing.T) {
	frame := EncodeTPKT(EncodeCOTPData())
	s, err := InspectFrame(frame)
	if err != nil {
		t.Fatalf("InspectFrame error: %v", err)
	}
	if s.COTPType != COTPTypeDT {
		t.Fatalf("unexpected COTP type: 0x%02X", s.COTPType)
	}
	if s.ROSCTR != 0 {
		t.Fatalf("expected no S7 ROSCTR, got 0x%02X", s.ROSCTR)
	}
}

func TestInspectFrameWithS7(t *testing.T) {
	s7 := EncodeS7Header(ROSCTRJob, 1, 1, 0)
	s7 = append(s7, FuncReadVar)
	frame := EncodeTPKT(append(EncodeCOTPData(), s7...))

	s, err := InspectFrame(frame)
	if err != nil {
		t.Fatalf("InspectFrame error: %v", err)
	}
	if s.ROSCTR != ROSCTRJob {
		t.Fatalf("unexpected ROSCTR: 0x%02X", s.ROSCTR)
	}
	if s.Function != FuncReadVar {
		t.Fatalf("unexpected function: 0x%02X", s.Function)
	}
}
