package wire

import (
	"encoding/binary"
)

// S7 Area codes
const (
	AreaSysInfo  = 0x03
	AreaSysFlags = 0x05
	AreaS7200AN  = 0x06
	AreaInputs   = 0x81
	AreaOutputs  = 0x82
	AreaMerkers  = 0x83
	AreaDB       = 0x84
	AreaDI       = 0x85
	AreaLocal    = 0x86
	AreaV        = 0x87
	AreaCounter  = 0x1C
	AreaTimer    = 0x1D
)

// Transport size codes
const (
	TransportSizeBit   = 0x01
	TransportSizeByte  = 0x02
	TransportSizeChar  = 0x03
	TransportSizeWord  = 0x04
	TransportSizeInt   = 0x05
	TransportSizeDWord = 0x06
	TransportSizeDInt  = 0x07
	TransportSizeReal  = 0x08
)

// S7AnyAddress is a specification of an S7 variable address
type S7AnyAddress struct {
	Area     byte
	DBNumber int
	Start    int // Byte offset
	Size     int // Number of bytes
}

// EncodeS7Any encodes an S7Any address specification
func EncodeS7Any(addr S7AnyAddress) []byte {
	buf := make([]byte, 12)
	buf[0] = 0x12 // Var spec
	buf[1] = 0x0A // Length of following
	buf[2] = 0x10 // S7Any
	buf[3] = TransportSizeByte

	binary.BigEndian.PutUint16(buf[4:6], uint16(addr.Size))
	binary.BigEndian.PutUint16(buf[6:8], uint16(addr.DBNumber))
	buf[8] = addr.Area

	// Start address in bits
	startBit := addr.Start * 8
	buf[9] = byte(startBit >> 16)
	buf[10] = byte(startBit >> 8)
	buf[11] = byte(startBit)

	return buf
}

// EncodeReadVarRequest creates a read variable request
func EncodeReadVarRequest(pduRef uint16, addrs []S7AnyAddress) []byte {
	param := make([]byte, 2)
	param[0] = FuncReadVar
	param[1] = byte(len(addrs))

	for _, addr := range addrs {
		param = append(param, EncodeS7Any(addr)...)
	}

	header := EncodeS7Header(ROSCTRJob, pduRef, len(param), 0)
	return append(header, param...)
}

// ReadVarItem represents a single read result
type ReadVarItem struct {
	ReturnCode byte
	Data       []byte
}

// ParseReadVarResponse parses a read variable response
func ParseReadVarResponse(param, data []byte) ([]ReadVarItem, error) {
	if len(param) < 2 {
		return nil, &S7Error{Message: "read response param too short"}
	}
	if param[0] != FuncReadVar {
		return nil, &S7Error{Message: "not a read var response"}
	}

	itemCount := int(param[1])
	items := make([]ReadVarItem, 0, itemCount)
	offset := 0

	for i := 0; i < itemCount && offset < len(data); i++ {
		if offset+4 > len(data) {
			break
		}
		retCode := data[offset]
		transportSize := data[offset+1]
		length := int(binary.BigEndian.Uint16(data[offset+2 : offset+4]))

		// Length is in bits for bit transfers, bytes otherwise
		byteLen := length
		if transportSize == 0x03 || transportSize == 0x04 {
			byteLen = (length + 7) / 8
		}

		item := ReadVarItem{ReturnCode: retCode}
		if retCode == RetCodeSuccess && offset+4+byteLen <= len(data) {
			item.Data = data[offset+4 : offset+4+byteLen]
		}
		items = append(items, item)

		offset += 4 + byteLen
		// Align to even byte boundary
		if byteLen%2 != 0 {
			offset++
		}
	}

	return items, nil
}

// EncodeWriteVarRequest creates a write variable request
func EncodeWriteVarRequest(pduRef uint16, addr S7AnyAddress, value []byte) []byte {
	param := make([]byte, 2)
	param[0] = FuncWriteVar
	param[1] = 1 // Item count
	param = append(param, EncodeS7Any(addr)...)

	// Data
	data := make([]byte, 4)
	data[0] = RetCodeSuccess                                    // Return code (reserved)
	data[1] = 0x04                                              // Transport size (byte/word/dword)
	binary.BigEndian.PutUint16(data[2:4], uint16(len(value)*8)) // Length in bits
	data = append(data, value...)
	// Pad to even length
	if len(data)%2 != 0 {
		data = append(data, 0x00)
	}

	header := EncodeS7Header(ROSCTRJob, pduRef, len(param), len(data))
	result := append(header, param...)
	return append(result, data...)
}

// ParseWriteVarResponse parses a write variable response
func ParseWriteVarResponse(param, data []byte) error {
	if len(param) < 2 {
		return &S7Error{Message: "write response param too short"}
	}
	if param[0] != FuncWriteVar {
		return &S7Error{Message: "not a write var response"}
	}
	if len(data) < 1 {
		return &S7Error{Message: "write response data too short"}
	}
	return ReturnCodeError(data[0])
}
