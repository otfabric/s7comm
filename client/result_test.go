package client

import (
	"errors"
	"testing"
)

func TestReadResult_OK(t *testing.T) {
	if !(&ReadResult{Status: ReadStatusSuccess}).OK() {
		t.Error("OK() should be true for success")
	}
	if (&ReadResult{Status: ReadStatusShortRead}).OK() {
		t.Error("OK() should be false for short-read")
	}
	if (&ReadResult{Status: ReadStatusEmptyRead}).OK() {
		t.Error("OK() should be false for empty-read")
	}
	if (*ReadResult)(nil).OK() {
		t.Error("OK() should be false for nil result")
	}
}

func TestReadResult_Err(t *testing.T) {
	if (*ReadResult)(nil).Err() == nil {
		t.Error("Err() should be non-nil for nil result")
	}
	if (&ReadResult{Status: ReadStatusSuccess}).Err() != nil {
		t.Error("Err() should be nil for success")
	}
	err := (&ReadResult{Status: ReadStatusEmptyRead, Error: "no data"}).Err()
	if err == nil {
		t.Fatal("Err() should be non-nil for empty-read")
	}
	var outErr *ReadOutcomeError
	if !errors.As(err, &outErr) {
		t.Errorf("Err() should wrap *ReadOutcomeError, got %T", err)
	}
	if outErr.Result.Status != ReadStatusEmptyRead {
		t.Errorf("outErr.Result.Status = %q", outErr.Result.Status)
	}
	if err.Error() != "no data" {
		t.Errorf("Err().Error() = %q", err.Error())
	}
}

func TestReadOutcomeError_Error(t *testing.T) {
	e := &ReadOutcomeError{Result: &ReadResult{Status: ReadStatusRejected}, message: "rejected"}
	if e.Error() != "rejected" {
		t.Errorf("Error() = %q", e.Error())
	}
}

func TestReadStatusConstants(t *testing.T) {
	statuses := []ReadStatus{
		ReadStatusSuccess, ReadStatusShortRead, ReadStatusEmptyRead, ReadStatusRejected,
		ReadStatusTimeout, ReadStatusTransportErr, ReadStatusProtocolErr, ReadStatusInconclusive,
	}
	for _, s := range statuses {
		if s == "" {
			t.Error("ReadStatus constant is empty")
		}
	}
}

func TestClassifyReadOutcome(t *testing.T) {
	tests := []struct {
		requested, returned int
		want                ReadStatus
	}{
		{0, 0, ReadStatusSuccess},
		{0, 5, ReadStatusSuccess}, // requested<=0: success regardless of returned
		{10, 10, ReadStatusSuccess},
		{10, 0, ReadStatusEmptyRead},
		{10, 5, ReadStatusShortRead},
		{10, 9, ReadStatusShortRead},
		{1, 1, ReadStatusSuccess},
		{1, 0, ReadStatusEmptyRead},
	}
	for _, tt := range tests {
		got := ClassifyReadOutcome(tt.requested, tt.returned)
		if got != tt.want {
			t.Errorf("ClassifyReadOutcome(%d, %d) = %q, want %q", tt.requested, tt.returned, got, tt.want)
		}
	}
}
