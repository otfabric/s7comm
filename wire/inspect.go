package wire

import "fmt"

// FrameSummary captures key protocol fields from one S7-over-TPKT frame.
type FrameSummary struct {
	TPKTLength  int
	COTPType    byte
	ROSCTR      byte
	Function    byte
	ParamLength int
	DataLength  int
	ErrorClass  byte
	ErrorCode   byte
}

// InspectFrame decodes a full TPKT frame and extracts high-level protocol metadata.
func InspectFrame(frame []byte) (*FrameSummary, error) {
	tpkt, cotpPayload, err := ParseTPKT(frame)
	if err != nil {
		return nil, fmt.Errorf("parse tpkt: %w", err)
	}

	cotp, s7Payload, err := ParseCOTP(cotpPayload)
	if err != nil {
		return nil, fmt.Errorf("parse cotp: %w", err)
	}

	s := &FrameSummary{
		TPKTLength: int(tpkt.Length),
		COTPType:   cotp.PDUType,
	}

	if len(s7Payload) == 0 || s7Payload[0] != 0x32 {
		return s, nil
	}

	h, rest, err := ParseS7Header(s7Payload)
	if err != nil {
		return nil, fmt.Errorf("parse s7 header: %w", err)
	}

	s.ROSCTR = h.ROSCTR
	s.ParamLength = int(h.ParamLength)
	s.DataLength = int(h.DataLength)
	s.ErrorClass = h.ErrorClass
	s.ErrorCode = h.ErrorCode
	if len(rest) > 0 {
		s.Function = rest[0]
	}

	return s, nil
}
