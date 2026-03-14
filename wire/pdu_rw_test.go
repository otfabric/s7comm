package wire

import "testing"

func TestEncodeReadVarRequest(t *testing.T) {
	addrs := []S7AnyAddress{{Area: AreaDB, DBNumber: 1, Start: 0, Size: 4}}
	msg := EncodeReadVarRequest(1, addrs)
	if len(msg) < 14 {
		t.Fatalf("unexpected message length: %d", len(msg))
	}
	if msg[10] != FuncReadVar {
		t.Fatalf("expected read var function at parameter start")
	}
}

func TestParseReadVarResponse(t *testing.T) {
	param := []byte{FuncReadVar, 0x01}
	data := []byte{RetCodeSuccess, 0x04, 0x00, 0x10, 0x12, 0x34}
	items, err := ParseReadVarResponse(param, data)
	if err != nil {
		t.Fatalf("ParseReadVarResponse error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
}

func TestParseWriteVarResponse(t *testing.T) {
	if err := ParseWriteVarResponse([]byte{FuncWriteVar, 1}, []byte{RetCodeSuccess}); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if err := ParseWriteVarResponse([]byte{FuncWriteVar}, []byte{RetCodeSuccess}); err == nil {
		t.Fatal("expected error for param too short")
	}
	if err := ParseWriteVarResponse([]byte{FuncReadVar, 1}, []byte{RetCodeSuccess}); err == nil {
		t.Fatal("expected error for wrong function")
	}
	if err := ParseWriteVarResponse([]byte{FuncWriteVar, 1}, nil); err == nil {
		t.Fatal("expected error for data too short")
	}
	if err := ParseWriteVarResponse([]byte{FuncWriteVar, 1}, []byte{RetCodeAccessFault}); err == nil {
		t.Fatal("expected error for non-success return code")
	}
}

func TestEncodeWriteVarRequest(t *testing.T) {
	addr := S7AnyAddress{Area: AreaDB, DBNumber: 1, Start: 0, Size: 4}
	value := []byte{0x01, 0x02, 0x03, 0x04}
	msg := EncodeWriteVarRequest(1, addr, value)
	if len(msg) < 14 {
		t.Fatalf("unexpected message length: %d", len(msg))
	}
	if msg[10] != FuncWriteVar {
		t.Fatalf("expected write var function at parameter start")
	}
	// Odd-length value gets padded
	msg2 := EncodeWriteVarRequest(1, addr, []byte{0x01, 0x02, 0x03})
	if len(msg2)%2 != 0 {
		t.Fatalf("expected even length for padded message, got %d", len(msg2))
	}
}
