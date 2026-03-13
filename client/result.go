package client

import "errors"

// ReadStatus classifies the outcome of a read request.
type ReadStatus string

const (
	ReadStatusSuccess      ReadStatus = "success"    // returned length == requested
	ReadStatusShortRead    ReadStatus = "short-read" // 0 < returned < requested
	ReadStatusEmptyRead    ReadStatus = "empty-read" // requested > 0, returned == 0
	ReadStatusRejected     ReadStatus = "rejected"   // target/S7 explicitly rejected
	ReadStatusTimeout      ReadStatus = "timeout"    // transport/context timeout
	ReadStatusTransportErr ReadStatus = "transport-error"
	ReadStatusProtocolErr  ReadStatus = "protocol-error"
	ReadStatusInconclusive ReadStatus = "inconclusive" // retries gave mixed results
)

// ReadResult holds the full outcome of a read (single logical read, possibly chunked).
type ReadResult struct {
	Status          ReadStatus
	RequestedLength int
	ReturnedLength  int
	Data            []byte
	Warnings        []string
	Error           string

	// Protocol detail (per-item return code when available)
	ItemStatus string
	ReturnCode byte
}

// OK returns true if Status == ReadStatusSuccess.
func (r *ReadResult) OK() bool {
	return r != nil && r.Status == ReadStatusSuccess
}

// Err returns a non-nil error when Status is not success, for use in "if err := result.Err(); err != nil".
func (r *ReadResult) Err() error {
	if r == nil {
		return errors.New("read result is nil")
	}
	if r.Status == ReadStatusSuccess {
		return nil
	}
	msg := string(r.Status)
	if r.Error != "" {
		msg = r.Error
	}
	return &ReadOutcomeError{Result: r, message: msg}
}

// ReadOutcomeError wraps a failed read result for use as error.
type ReadOutcomeError struct {
	Result  *ReadResult
	message string
}

func (e *ReadOutcomeError) Error() string {
	return e.message
}
