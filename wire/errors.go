package wire

import (
	"errors"
	"fmt"
)

var (
	ErrShortTPKTHeader     = errors.New("data too short for TPKT header")
	ErrInvalidTPKTVersion  = errors.New("invalid TPKT version")
	ErrTPKTLengthExceeds   = errors.New("TPKT length exceeds data")
	ErrShortCOTPHeader     = errors.New("data too short for COTP header")
	ErrCOTPLengthExceeds   = errors.New("COTP length exceeds data")
	ErrShortCOTPCRCC       = errors.New("COTP CR/CC too short")
	ErrShortS7Header       = errors.New("data too short for S7 header")
	ErrInvalidS7ProtocolID = errors.New("invalid S7 protocol ID")
	ErrShortS7AckHeader    = errors.New("data too short for S7 ack header")
	ErrS7PayloadLength     = errors.New("S7 payload shorter than parameter/data lengths")
)

// S7 Error classes
const (
	ErrClassNoError  = 0x00
	ErrClassApp      = 0x81
	ErrClassObject   = 0x82
	ErrClassResource = 0x83
	ErrClassService  = 0x84
	ErrClassSupplies = 0x85
	ErrClassAccess   = 0x87
)

// Return codes for Read/Write operations
const (
	RetCodeSuccess       = 0xFF // Success
	RetCodeHWFault       = 0x01 // Hardware fault
	RetCodeAccessFault   = 0x03 // Illegal object access
	RetCodeAddressFault  = 0x05 // Invalid address
	RetCodeDataTypeFault = 0x06 // Data type not supported
	RetCodeDataSizeFault = 0x07 // Data size mismatch
	RetCodeBusy          = 0x09 // Object is busy
	RetCodeNotAvailable  = 0x0A // Object not available
)

// S7Error represents an S7 protocol error
type S7Error struct {
	Class   byte
	Code    byte
	Message string
}

func (e *S7Error) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("S7 error: class=0x%02X, code=0x%02X", e.Class, e.Code)
}

// NewS7Error creates a new S7 error with a descriptive message
func NewS7Error(class, code byte) *S7Error {
	msg := ""
	switch class {
	case ErrClassAccess:
		switch code {
		case 0x00:
			msg = "no access rights"
		case 0x01:
			msg = "invalid address"
		case 0x04:
			msg = "invalid data type"
		default:
			msg = "access error"
		}
	case ErrClassObject:
		msg = "object does not exist"
	case ErrClassResource:
		msg = "resource busy"
	}
	return &S7Error{Class: class, Code: code, Message: msg}
}

// ReturnCodeError returns an error for a return code
func ReturnCodeError(code byte) error {
	switch code {
	case RetCodeSuccess:
		return nil
	case RetCodeHWFault:
		return &S7Error{Code: code, Message: "hardware fault"}
	case RetCodeAccessFault:
		return &S7Error{Code: code, Message: "access denied"}
	case RetCodeAddressFault:
		return &S7Error{Code: code, Message: "invalid address"}
	case RetCodeDataTypeFault:
		return &S7Error{Code: code, Message: "data type not supported"}
	case RetCodeDataSizeFault:
		return &S7Error{Code: code, Message: "data size mismatch"}
	case RetCodeBusy:
		return &S7Error{Code: code, Message: "object busy"}
	case RetCodeNotAvailable:
		return &S7Error{Code: code, Message: "object not available"}
	default:
		return &S7Error{Code: code, Message: fmt.Sprintf("return code 0x%02X", code)}
	}
}
