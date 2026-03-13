package wire

import "testing"

func TestParseStartUploadResponse(t *testing.T) {
	param := []byte{FuncUploadStart, 0, 0, 0, 0, 0, 0, 0, 4, 'A', 'B', 'C', 'D'}
	session, err := ParseStartUploadResponse(param)
	if err != nil {
		t.Fatalf("ParseStartUploadResponse error: %v", err)
	}
	if session != "ABCD" {
		t.Fatalf("unexpected session: %q", session)
	}
}

func TestParseUploadResponse(t *testing.T) {
	param := []byte{FuncUpload, 0}
	data := []byte{RetCodeSuccess, 0x09, 0x00, 0x10, 0xAA, 0xBB}
	chunk, err := ParseUploadResponse(param, data)
	if err != nil {
		t.Fatalf("ParseUploadResponse error: %v", err)
	}
	if !chunk.Done {
		t.Fatal("expected done=true for status 0")
	}
	if len(chunk.Data) != 2 {
		t.Fatalf("unexpected chunk length: %d", len(chunk.Data))
	}
}
