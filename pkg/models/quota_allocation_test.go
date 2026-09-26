package models

import (
	"bytes"
	"testing"
)

func TestQuotaAllocation_Bytes(t *testing.T) {
	qa := &QuotaAllocation{
		AuthUsername: "testuser",
		AuthPassword: "testpass",
		AccountAllocations: map[uint64]uint64{
			1: 100,
			2: 200,
		},
	}

	bytes := qa.Bytes()
	if len(bytes) == 0 {
		t.Errorf("Expected Bytes() to return non-empty data")
	}
}

func TestQuotaAllocation_String(t *testing.T) {
	qa := &QuotaAllocation{
		AccountAllocations: map[uint64]uint64{
			1: 100,
			2: 200,
		},
	}

	str := qa.String()
	if len(str) == 0 {
		t.Errorf("Expected String() to return non-empty string")
	}
}

func TestQuotaAllocation_WriteTo(t *testing.T) {
	qa := &QuotaAllocation{
		AuthUsername: "testuser",
		AuthPassword: "testpass",
		AccountAllocations: map[uint64]uint64{
			1: 100,
			2: 200,
		},
	}

	var buf bytes.Buffer
	n, err := qa.WriteTo(&buf)
	if err != nil {
		t.Errorf("Expected WriteTo to succeed, got error: %v", err)
	}

	if n <= 0 {
		t.Errorf("Expected WriteTo to return positive number of bytes written, got %d", n)
	}
}

func TestQuotaAllocation_ReadFrom(t *testing.T) {
	// First write a QuotaAllocation
	qa := &QuotaAllocation{
		AuthUsername: "testuser",
		AuthPassword: "testpass",
		AccountAllocations: map[uint64]uint64{
			1: 100,
			2: 200,
			3: 300,
		},
	}
	var writeBuf bytes.Buffer
	_, _ = qa.WriteTo(&writeBuf)

	// Now read it back
	var readQA QuotaAllocation
	var readBuf bytes.Buffer
	readBuf.Write(writeBuf.Bytes())

	n, err := readQA.ReadFrom(&readBuf)
	if err != nil {
		t.Errorf("Expected ReadFrom to succeed, got error: %v", err)
	}

	if n <= 0 {
		t.Errorf("Expected ReadFrom to return positive number of bytes read, got %d", n)
	}

	if readQA.AuthUsername != qa.AuthUsername {
		t.Errorf("Expected ReadFrom to recover AuthUsername, got %s, expected %s", readQA.AuthUsername, qa.AuthUsername)
	}

	if readQA.AuthPassword != qa.AuthPassword {
		t.Errorf("Expected ReadFrom to recover AuthPassword, got %s, expected %s", readQA.AuthPassword, qa.AuthPassword)
	}

	if len(readQA.AccountAllocations) != len(qa.AccountAllocations) {
		t.Errorf("Expected ReadFrom to recover same number of allocations, got %d, expected %d", len(readQA.AccountAllocations), len(qa.AccountAllocations))
	}
}

func TestQuotaAllocation_ReadFrom_InvalidType(t *testing.T) {
	var readQA QuotaAllocation
	var buf bytes.Buffer

	// Write an invalid payload type
	buf.WriteByte(StringPayload) // Wrong type
	buf.Write(make([]byte, 4))   // Size

	_, err := readQA.ReadFrom(&buf)
	if err == nil {
		t.Errorf("Expected ReadFrom to return error for invalid payload type, got nil")
	}
}

func TestQuotaAllocation_ReadFrom_Empty(t *testing.T) {
	var readQA QuotaAllocation
	var buf bytes.Buffer

	_, err := readQA.ReadFrom(&buf)
	if err == nil {
		t.Errorf("Expected ReadFrom to return error for empty buffer, got nil")
	}
}

func TestQuotaAllocation_ReadFrom_MaxPayloadSize(t *testing.T) {
	var readQA QuotaAllocation
	var buf bytes.Buffer

	// Write payload type
	buf.WriteByte(QuotaAllocationPayload)
	// Write size that exceeds MaxPayloadSize
	size := MaxPayloadSize + 1
	var sizeBytes [4]byte
	sizeBytes[0] = byte(size >> 24)
	sizeBytes[1] = byte(size >> 16)
	sizeBytes[2] = byte(size >> 8)
	sizeBytes[3] = byte(size)
	buf.Write(sizeBytes[:])

	_, err := readQA.ReadFrom(&buf)
	if err == nil {
		t.Errorf("Expected ReadFrom to return error for payload exceeding MaxPayloadSize, got nil")
	}
	if err != ErrMaxPayloadSize {
		t.Errorf("Expected ReadFrom to return ErrMaxPayloadSize, got %v", err)
	}
}

func TestQuotaAllocation_RoundTrip(t *testing.T) {
	testCases := []struct {
		username            string
		password            string
		accountAllocations  map[uint64]uint64
	}{
		{"user1", "pass1", map[uint64]uint64{1: 100}},
		{"user2", "pass2", map[uint64]uint64{1: 100, 2: 200, 3: 300}},
		{"", "", map[uint64]uint64{}},
	}

	for _, tc := range testCases {
		qa := &QuotaAllocation{
			AuthUsername:       tc.username,
			AuthPassword:       tc.password,
			AccountAllocations: tc.accountAllocations,
		}

		var buf bytes.Buffer
		_, err := qa.WriteTo(&buf)
		if err != nil {
			t.Errorf("WriteTo failed for test case %+v: %v", tc, err)
			continue
		}

		var readQA QuotaAllocation
		var readBuf bytes.Buffer
		readBuf.Write(buf.Bytes())
		_, err = readQA.ReadFrom(&readBuf)
		if err != nil {
			t.Errorf("ReadFrom failed for test case %+v: %v", tc, err)
			continue
		}

		if readQA.AuthUsername != qa.AuthUsername {
			t.Errorf("Round trip failed for username: got %q, expected %q", readQA.AuthUsername, qa.AuthUsername)
		}
		if readQA.AuthPassword != qa.AuthPassword {
			t.Errorf("Round trip failed for password: got %q, expected %q", readQA.AuthPassword, qa.AuthPassword)
		}
		if len(readQA.AccountAllocations) != len(qa.AccountAllocations) {
			t.Errorf("Round trip failed for allocations count: got %d, expected %d", len(readQA.AccountAllocations), len(qa.AccountAllocations))
		}
	}
}

