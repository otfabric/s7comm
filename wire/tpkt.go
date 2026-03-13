// Package wire implements S7 protocol encoding/decoding.
package wire

import (
	"encoding/binary"
	"fmt"
)

// TPKT header constants
const (
	TPKTVersion    = 3
	TPKTHeaderSize = 4
)

// TPKT represents a TPKT header (RFC 1006)
type TPKT struct {
	Version  byte
	Reserved byte
	Length   uint16
}

// EncodeTPKT wraps payload in a TPKT header
func EncodeTPKT(payload []byte) []byte {
	length := TPKTHeaderSize + len(payload)
	buf := make([]byte, length)
	buf[0] = TPKTVersion
	buf[1] = 0
	binary.BigEndian.PutUint16(buf[2:4], uint16(length))
	copy(buf[4:], payload)
	return buf
}

// ParseTPKT extracts TPKT header and payload from a frame
func ParseTPKT(data []byte) (*TPKT, []byte, error) {
	if len(data) < TPKTHeaderSize {
		return nil, nil, ErrShortTPKTHeader
	}
	if data[0] != TPKTVersion {
		return nil, nil, fmt.Errorf("%w: got %d", ErrInvalidTPKTVersion, data[0])
	}
	length := binary.BigEndian.Uint16(data[2:4])
	if int(length) > len(data) {
		return nil, nil, fmt.Errorf("%w: declared %d, frame %d", ErrTPKTLengthExceeds, length, len(data))
	}
	tpkt := &TPKT{
		Version:  data[0],
		Reserved: data[1],
		Length:   length,
	}
	return tpkt, data[TPKTHeaderSize:length], nil
}
