package wire

import (
	"encoding/binary"

	"github.com/otfabric/go-cotp"
)

// BuildTSAP creates a TSAP from connection type, rack, and slot (S7 convention).
// Connection type: 1=PG, 2=OP, 3=S7Basic
func BuildTSAP(connType, rack, slot int) uint16 {
	return uint16((connType << 8) | ((rack & 0x1F) << 5) | (slot & 0x1F))
}

// EncodeCOTPCR builds a COTP Connection Request TPDU for the given local/remote TSAPs (go-cotp).
func EncodeCOTPCR(localTSAP, remoteTSAP uint16) ([]byte, error) {
	tpduSize := uint8(0x0A) // 1024 bytes
	cr := &cotp.CR{
		CDT:             0,
		DestinationRef:  0,
		SourceRef:       0,
		ClassOption:     0,
		CallingSelector: binary.BigEndian.AppendUint16(nil, localTSAP),
		CalledSelector:  binary.BigEndian.AppendUint16(nil, remoteTSAP),
		TPDUSize:        &tpduSize,
	}
	return cr.MarshalBinary()
}

// EncodeCOTPDT builds a COTP Data TPDU with EOT and the given S7 payload (go-cotp).
func EncodeCOTPDT(s7Payload []byte) ([]byte, error) {
	dt := &cotp.DT{
		EOT:      true,
		UserData: s7Payload,
	}
	return dt.MarshalBinary()
}
