package wire

import (
	"encoding/binary"
)

// SZL ID constants
const (
	SZLModuleID       = 0x0011 // Module identification
	SZLComponentID    = 0x001C // Component identification
	SZLCPUState       = 0x0424 // CPU state
	SZLProtectionInfo = 0x0232 // Protection level
	SZLDiagBuffer     = 0x00A0 // Diagnostic buffer
	SZLBlockList      = 0x0111 // Block list
	SZLBlockInfo      = 0x0113 // Block info
)

// Userdata function codes
const (
	UserdataReq      = 0x11
	UserdataRes      = 0x12
	UserdataSubParam = 0x44 // With parameters
	UserdataSZL      = 0x01 // SZL read
)

// EncodeSZLRequest creates a SZL read request
func EncodeSZLRequest(pduRef, szlID, szlIndex uint16) []byte {
	// Parameter block
	param := []byte{
		0x00, 0x01, 0x12, // Parameter header
		0x04, // Parameter length
		UserdataSubParam,
		UserdataSZL,
		0x00, // Sequence number
		0x00, // Data unit reference number
	}

	// Data block
	data := make([]byte, 8)
	data[0] = RetCodeSuccess                 // Return code
	data[1] = 0x09                           // Transport size: OCT
	binary.BigEndian.PutUint16(data[2:4], 4) // Length
	binary.BigEndian.PutUint16(data[4:6], szlID)
	binary.BigEndian.PutUint16(data[6:8], szlIndex)

	header := EncodeS7Header(ROSCTRUserdata, pduRef, len(param), len(data))
	result := append(header, param...)
	return append(result, data...)
}

// SZLResponse holds a parsed SZL response
type SZLResponse struct {
	SZLID      uint16
	SZLIndex   uint16
	DataLength uint16
	Data       []byte
}

// ParseSZLResponse parses a SZL read response
func ParseSZLResponse(paramData, data []byte) (*SZLResponse, error) {
	if len(data) < 8 {
		return nil, &S7Error{Message: "SZL response too short"}
	}

	retCode := data[0]
	if retCode != RetCodeSuccess {
		return nil, ReturnCodeError(retCode)
	}

	// Skip to SZL header
	dataLen := binary.BigEndian.Uint16(data[2:4])
	if int(dataLen)+4 > len(data) {
		return nil, &S7Error{Message: "SZL data length mismatch"}
	}

	resp := &SZLResponse{
		SZLID:      binary.BigEndian.Uint16(data[4:6]),
		SZLIndex:   binary.BigEndian.Uint16(data[6:8]),
		DataLength: dataLen,
	}

	if len(data) > 8 {
		resp.Data = data[8:]
	}

	return resp, nil
}
