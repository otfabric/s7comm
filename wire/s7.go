package wire

import (
	"encoding/binary"
	"fmt"
)

// S7 ROSCTR types (message types)
const (
	ROSCTRJob      = 0x01 // Job request
	ROSCTRAck      = 0x02 // Acknowledgement without data
	ROSCTRAckData  = 0x03 // Acknowledgement with data
	ROSCTRUserdata = 0x07 // Userdata (for SZL, etc.)
)

// S7 function codes
const (
	FuncSetupComm     = 0xF0 // Setup communication
	FuncReadVar       = 0x04 // Read variable
	FuncWriteVar      = 0x05 // Write variable
	FuncDownloadStart = 0x1A
	FuncDownload      = 0x1B
	FuncDownloadEnd   = 0x1C
	FuncUploadStart   = 0x1D
	FuncUpload        = 0x1E
	FuncUploadEnd     = 0x1F
	FuncPIService     = 0x28 // Program invocation
	FuncPLCStop       = 0x29
	FuncPLCControl    = 0x28
)

// S7Header represents an S7 protocol header
type S7Header struct {
	ProtocolID   byte   // Always 0x32
	ROSCTR       byte   // Remote Operating Service Control
	RedundancyID uint16 // Usually 0
	PDURef       uint16 // PDU reference
	ParamLength  uint16 // Parameter length
	DataLength   uint16 // Data length
	ErrorClass   byte   // Error class (for Ack)
	ErrorCode    byte   // Error code (for Ack)
}

const S7HeaderSize = 10

// EncodeS7Header creates an S7 protocol header
func EncodeS7Header(rosctr byte, pduRef uint16, paramLen, dataLen int) []byte {
	buf := make([]byte, S7HeaderSize)
	buf[0] = 0x32 // S7 Protocol ID
	buf[1] = rosctr
	binary.BigEndian.PutUint16(buf[2:4], 0)      // Redundancy ID
	binary.BigEndian.PutUint16(buf[4:6], pduRef) // PDU Ref
	binary.BigEndian.PutUint16(buf[6:8], uint16(paramLen))
	binary.BigEndian.PutUint16(buf[8:10], uint16(dataLen))
	return buf
}

// ParseS7Header parses an S7 header from data
func ParseS7Header(data []byte) (*S7Header, []byte, error) {
	if len(data) < S7HeaderSize {
		return nil, nil, ErrShortS7Header
	}
	if data[0] != 0x32 {
		return nil, nil, fmt.Errorf("%w: got 0x%02X", ErrInvalidS7ProtocolID, data[0])
	}

	h := &S7Header{
		ProtocolID:   data[0],
		ROSCTR:       data[1],
		RedundancyID: binary.BigEndian.Uint16(data[2:4]),
		PDURef:       binary.BigEndian.Uint16(data[4:6]),
		ParamLength:  binary.BigEndian.Uint16(data[6:8]),
		DataLength:   binary.BigEndian.Uint16(data[8:10]),
	}

	offset := S7HeaderSize
	if h.ROSCTR == ROSCTRAckData || h.ROSCTR == ROSCTRAck {
		if len(data) < S7HeaderSize+2 {
			return nil, nil, ErrShortS7AckHeader
		}
		h.ErrorClass = data[10]
		h.ErrorCode = data[11]
		offset = 12
	}

	need := int(h.ParamLength) + int(h.DataLength)
	if len(data[offset:]) < need {
		return nil, nil, fmt.Errorf("%w: need %d, got %d", ErrS7PayloadLength, need, len(data[offset:]))
	}

	return h, data[offset:], nil
}
