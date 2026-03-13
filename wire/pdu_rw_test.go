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
}
