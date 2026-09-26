package models

import (
	"bytes"
	"testing"
)

func TestLocalQuotaResponse_Bytes(t *testing.T) {
	lqr := &LocalQuotaResponse{
		AccountAllocations: map[uint64]uint64{
			1: 100,
			2: 200,
		},
	}

	bytes := lqr.Bytes()
	if len(bytes) == 0 {
		t.Errorf("Expected Bytes() to return non-empty data")
	}
}

func TestLocalQuotaResponse_String(t *testing.T) {
	lqr := &LocalQuotaResponse{
		AccountAllocations: map[uint64]uint64{
			1: 100,
			2: 200,
		},
	}

	str := lqr.String()
	if len(str) == 0 {
		t.Errorf("Expected String() to return non-empty string")
	}
}

func TestLocalQuotaResponse_WriteTo(t *testing.T) {
	lqr := &LocalQuotaResponse{
		AccountAllocations: map[uint64]uint64{
			1: 100,
			2: 200,
		},
	}

	var buf bytes.Buffer
	n, err := lqr.WriteTo(&buf)
	if err != nil {
		t.Errorf("Expected WriteTo to succeed, got error: %v", err)
	}

	if n <= 0 {
		t.Errorf("Expected WriteTo to return positive number of bytes written, got %d", n)
	}
}

func TestLocalQuotaResponse_ReadFrom(t *testing.T) {
	// First write a LocalQuotaResponse
	lqr := &LocalQuotaResponse{
		AccountAllocations: map[uint64]uint64{
			1: 100,
			2: 200,
			3: 300,
		},
	}
	var writeBuf bytes.Buffer
	_, _ = lqr.WriteTo(&writeBuf)

	// Now read it back
	var readLQR LocalQuotaResponse
	var readBuf bytes.Buffer
	readBuf.Write(writeBuf.Bytes())

	n, err := readLQR.ReadFrom(&readBuf)
	if err != nil {
		t.Errorf("Expected ReadFrom to succeed, got error: %v", err)
	}

	if n <= 0 {
		t.Errorf("Expected ReadFrom to return positive number of bytes read, got %d", n)
	}

	if len(readLQR.AccountAllocations) != len(lqr.AccountAllocations) {
		t.Errorf("Expected ReadFrom to recover same number of allocations, got %d, expected %d", len(readLQR.AccountAllocations), len(lqr.AccountAllocations))
	}

	for k, v := range lqr.AccountAllocations {
		if readLQR.AccountAllocations[k] != v {
			t.Errorf("Expected ReadFrom to recover allocation for account %d, got %d, expected %d", k, readLQR.AccountAllocations[k], v)
		}
	}
}

func TestLocalQuotaResponse_ReadFrom_InvalidType(t *testing.T) {
	var readLQR LocalQuotaResponse
	var buf bytes.Buffer

	// Write an invalid payload type
	buf.WriteByte(StringPayload) // Wrong type
	buf.Write(make([]byte, 4))   // Size

	_, err := readLQR.ReadFrom(&buf)
	if err == nil {
		t.Errorf("Expected ReadFrom to return error for invalid payload type, got nil")
	}
}

func TestLocalQuotaResponse_ReadFrom_Empty(t *testing.T) {
	var readLQR LocalQuotaResponse
	var buf bytes.Buffer

	_, err := readLQR.ReadFrom(&buf)
	if err == nil {
		t.Errorf("Expected ReadFrom to return error for empty buffer, got nil")
	}
}

func TestLocalQuotaResponse_RoundTrip(t *testing.T) {
	testCases := []map[uint64]uint64{
		{},
		{1: 100},
		{1: 100, 2: 200, 3: 300},
		{100: 1000, 200: 2000, 300: 3000},
	}

	for _, tc := range testCases {
		lqr := &LocalQuotaResponse{
			AccountAllocations: tc,
		}

		var buf bytes.Buffer
		_, err := lqr.WriteTo(&buf)
		if err != nil {
			t.Errorf("WriteTo failed for test case %+v: %v", tc, err)
			continue
		}

		var readLQR LocalQuotaResponse
		var readBuf bytes.Buffer
		readBuf.Write(buf.Bytes())
		_, err = readLQR.ReadFrom(&readBuf)
		if err != nil {
			t.Errorf("ReadFrom failed for test case %+v: %v", tc, err)
			continue
		}

		if len(readLQR.AccountAllocations) != len(lqr.AccountAllocations) {
			t.Errorf("Round trip failed: got %d allocations, expected %d", len(readLQR.AccountAllocations), len(lqr.AccountAllocations))
		}

		for k, v := range lqr.AccountAllocations {
			if readLQR.AccountAllocations[k] != v {
				t.Errorf("Round trip failed for account %d: got %d, expected %d", k, readLQR.AccountAllocations[k], v)
			}
		}
	}
}
