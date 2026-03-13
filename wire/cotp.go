package wire

import (
	"encoding/binary"
	"fmt"
)

// COTP PDU types
const (
	COTPTypeCR = 0xE0 // Connection Request
	COTPTypeCC = 0xD0 // Connection Confirm
	COTPTypeDT = 0xF0 // Data Transfer
	COTPTypeDR = 0x80 // Disconnect Request
)

// COTP parameter codes
const (
	COTPParamTSAPCalling = 0xC1
	COTPParamTSAPCalled  = 0xC2
	COTPParamTPDUSize    = 0xC0
)

// COTP represents a COTP header
type COTP struct {
	Length   byte
	PDUType  byte
	DstRef   uint16
	SrcRef   uint16
	ClassOpt byte
	TPDU     byte // TPDU number and EOT for DT
}

// EncodeCOTPCR creates a COTP Connection Request PDU
func EncodeCOTPCR(localTSAP, remoteTSAP uint16) []byte {
	buf := make([]byte, 18)
	buf[0] = 17                                  // Length (excluding length byte)
	buf[1] = COTPTypeCR                          // CR PDU
	binary.BigEndian.PutUint16(buf[2:4], 0x0000) // Dst Ref
	binary.BigEndian.PutUint16(buf[4:6], 0x0001) // Src Ref
	buf[6] = 0x00                                // Class 0

	// TSAP calling
	buf[7] = COTPParamTSAPCalling
	buf[8] = 2
	binary.BigEndian.PutUint16(buf[9:11], localTSAP)

	// TSAP called
	buf[11] = COTPParamTSAPCalled
	buf[12] = 2
	binary.BigEndian.PutUint16(buf[13:15], remoteTSAP)

	// TPDU size
	buf[15] = COTPParamTPDUSize
	buf[16] = 1
	buf[17] = 0x0A // 1024 bytes

	return buf
}

// EncodeCOTPData creates a COTP Data Transfer PDU header
func EncodeCOTPData() []byte {
	return []byte{
		0x02,       // Length
		COTPTypeDT, // DT PDU
		0x80,       // EOT (last data unit)
	}
}

// ParseCOTP parses a COTP PDU from data
func ParseCOTP(data []byte) (*COTP, []byte, error) {
	if len(data) < 2 {
		return nil, nil, ErrShortCOTPHeader
	}
	length := int(data[0])
	if length+1 > len(data) {
		return nil, nil, fmt.Errorf("%w: declared %d, frame %d", ErrCOTPLengthExceeds, length+1, len(data))
	}

	cotp := &COTP{
		Length:  data[0],
		PDUType: data[1] & 0xF0,
	}

	switch cotp.PDUType {
	case COTPTypeCR, COTPTypeCC:
		if length < 6 {
			return nil, nil, ErrShortCOTPCRCC
		}
		cotp.DstRef = binary.BigEndian.Uint16(data[2:4])
		cotp.SrcRef = binary.BigEndian.Uint16(data[4:6])
		cotp.ClassOpt = data[6]
	case COTPTypeDT:
		if length >= 2 {
			cotp.TPDU = data[2]
		}
	}

	return cotp, data[length+1:], nil
}

// BuildTSAP creates a TSAP from connection type, rack, and slot
// Connection type: 1=PG, 2=OP, 3=S7Basic
func BuildTSAP(connType, rack, slot int) uint16 {
	return uint16((connType << 8) | ((rack & 0x1F) << 5) | (slot & 0x1F))
}
