package wire

import (
	"encoding/binary"
	"fmt"

	"github.com/otfabric/go-cotp"
)

// EncodeRackSlotTSAP returns the low byte of a classic S7 TSAP for rack/slot.
// Protocol: bits 0..4 = slot, bits 5..7 = rack. Rack must be 0..7, slot 0..31.
// This is the single canonical TSAP rack/slot encoder; do not duplicate bit-packing elsewhere.
func EncodeRackSlotTSAP(rack, slot byte) byte {
	return ((rack & 0x07) << 5) | (slot & 0x1F)
}

// ValidateRackSlot returns an error if rack or slot are outside classic S7 range (rack 0..7, slot 0..31).
func ValidateRackSlot(rack, slot int) error {
	if rack < 0 || rack > 7 {
		return fmt.Errorf("rack must be 0..7, got %d", rack)
	}
	if slot < 0 || slot > 31 {
		return fmt.Errorf("slot must be 0..31, got %d", slot)
	}
	return nil
}

// BuildTSAP creates a TSAP from connection type, rack, and slot (S7 convention).
// Connection type: 1=PG, 2=OP, 3=S7Basic. Rack must be 0..7, slot 0..31; returns error if out of range.
func BuildTSAP(connType, rack, slot int) (uint16, error) {
	if err := ValidateRackSlot(rack, slot); err != nil {
		return 0, err
	}
	low := EncodeRackSlotTSAP(byte(rack), byte(slot))
	return uint16(connType)<<8 | uint16(low), nil
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
