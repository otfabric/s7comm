package wire

import (
	"testing"
)

func TestS7Error_Error(t *testing.T) {
	e := &S7Error{Class: 0x81, Code: 0x01}
	if e.Error() != "S7 error: class=0x81, code=0x01" {
		t.Fatalf("Error() without message: got %q", e.Error())
	}
	e.Message = "custom"
	if e.Error() != "custom" {
		t.Fatalf("Error() with message: got %q", e.Error())
	}
}

func TestNewS7Error(t *testing.T) {
	e := NewS7Error(ErrClassAccess, 0x00)
	if e == nil || e.Message != "no access rights" {
		t.Fatalf("NewS7Error(access, 0x00): got %v", e)
	}
	e = NewS7Error(ErrClassAccess, 0x01)
	if e.Message != "invalid address" {
		t.Fatalf("NewS7Error(access, 0x01): got %q", e.Message)
	}
	e = NewS7Error(ErrClassAccess, 0x04)
	if e.Message != "invalid data type" {
		t.Fatalf("NewS7Error(access, 0x04): got %q", e.Message)
	}
	e = NewS7Error(ErrClassAccess, 0xFF)
	if e.Message != "access error" {
		t.Fatalf("NewS7Error(access, 0xFF): got %q", e.Message)
	}
	e = NewS7Error(ErrClassObject, 0)
	if e.Message != "object does not exist" {
		t.Fatalf("NewS7Error(object): got %q", e.Message)
	}
	e = NewS7Error(ErrClassResource, 0)
	if e.Message != "resource busy" {
		t.Fatalf("NewS7Error(resource): got %q", e.Message)
	}
	e = NewS7Error(0x00, 0)
	if e.Message != "" {
		t.Fatalf("NewS7Error(no error class): expected empty message, got %q", e.Message)
	}
}

func TestReturnCodeError(t *testing.T) {
	if err := ReturnCodeError(RetCodeSuccess); err != nil {
		t.Fatalf("RetCodeSuccess should return nil: %v", err)
	}
	codes := []struct {
		code byte
		msg  string
	}{
		{RetCodeHWFault, "hardware fault"},
		{RetCodeAccessFault, "access denied"},
		{RetCodeAddressFault, "invalid address"},
		{RetCodeDataTypeFault, "data type not supported"},
		{RetCodeDataSizeFault, "data size mismatch"},
		{RetCodeBusy, "object busy"},
		{RetCodeNotAvailable, "object not available"},
	}
	for _, c := range codes {
		err := ReturnCodeError(c.code)
		if err == nil {
			t.Fatalf("ReturnCodeError(0x%02X) expected error", c.code)
		}
		if err.Error() != c.msg {
			t.Fatalf("ReturnCodeError(0x%02X): got %q, want %q", c.code, err.Error(), c.msg)
		}
	}
	err := ReturnCodeError(0x99)
	if err == nil || err.Error() != "return code 0x99" {
		t.Fatalf("ReturnCodeError(unknown): got %v", err)
	}
}
