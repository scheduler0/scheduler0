package models

import (
	"bytes"
	"testing"
)

func TestFetchRemoteData_Bytes(t *testing.T) {
	frd := &FetchRemoteData{
		Phase:        "test-phase",
		AuthUsername: "testuser",
		AuthPassword: "testpass",
		RequestId:    "req-123",
	}

	bytes := frd.Bytes()
	if len(bytes) == 0 {
		t.Errorf("Expected Bytes() to return non-empty data")
	}
}

func TestFetchRemoteData_String(t *testing.T) {
	frd := &FetchRemoteData{
		Phase:        "test-phase",
		AuthUsername: "testuser",
		AuthPassword: "testpass",
		RequestId:    "req-123",
	}

	str := frd.String()
	if len(str) == 0 {
		t.Errorf("Expected String() to return non-empty string")
	}
}

func TestFetchRemoteData_WriteTo(t *testing.T) {
	frd := &FetchRemoteData{
		Phase:        "test-phase",
		AuthUsername: "testuser",
		AuthPassword: "testpass",
		RequestId:    "req-123",
	}

	var buf bytes.Buffer
	n, err := frd.WriteTo(&buf)
	if err != nil {
		t.Errorf("Expected WriteTo to succeed, got error: %v", err)
	}

	if n <= 0 {
		t.Errorf("Expected WriteTo to return positive number of bytes written, got %d", n)
	}
}

func TestFetchRemoteData_ReadFrom(t *testing.T) {
	// First write a FetchRemoteData
	frd := &FetchRemoteData{
		Phase:        "test-phase",
		AuthUsername: "testuser",
		AuthPassword: "testpass",
		RequestId:    "req-123",
	}
	var writeBuf bytes.Buffer
	_, _ = frd.WriteTo(&writeBuf)

	// Now read it back
	var readFRD FetchRemoteData
	var readBuf bytes.Buffer
	readBuf.Write(writeBuf.Bytes())

	n, err := readFRD.ReadFrom(&readBuf)
	if err != nil {
		t.Errorf("Expected ReadFrom to succeed, got error: %v", err)
	}

	if n <= 0 {
		t.Errorf("Expected ReadFrom to return positive number of bytes read, got %d", n)
	}

	if readFRD.Phase != frd.Phase {
		t.Errorf("Expected ReadFrom to recover Phase, got %s, expected %s", readFRD.Phase, frd.Phase)
	}

	if readFRD.AuthUsername != frd.AuthUsername {
		t.Errorf("Expected ReadFrom to recover AuthUsername, got %s, expected %s", readFRD.AuthUsername, frd.AuthUsername)
	}

	if readFRD.AuthPassword != frd.AuthPassword {
		t.Errorf("Expected ReadFrom to recover AuthPassword, got %s, expected %s", readFRD.AuthPassword, frd.AuthPassword)
	}

	if readFRD.RequestId != frd.RequestId {
		t.Errorf("Expected ReadFrom to recover RequestId, got %s, expected %s", readFRD.RequestId, frd.RequestId)
	}
}

func TestFetchRemoteData_ReadFrom_InvalidType(t *testing.T) {
	var readFRD FetchRemoteData
	var buf bytes.Buffer

	// Write an invalid payload type
	buf.WriteByte(StringPayload) // Wrong type
	buf.Write(make([]byte, 4))     // Size

	_, err := readFRD.ReadFrom(&buf)
	if err == nil {
		t.Errorf("Expected ReadFrom to return error for invalid payload type, got nil")
	}
}

func TestFetchRemoteData_ReadFrom_Empty(t *testing.T) {
	var readFRD FetchRemoteData
	var buf bytes.Buffer

	_, err := readFRD.ReadFrom(&buf)
	if err == nil {
		t.Errorf("Expected ReadFrom to return error for empty buffer, got nil")
	}
}

func TestFetchRemoteData_ReadFrom_MaxPayloadSize(t *testing.T) {
	var readFRD FetchRemoteData
	var buf bytes.Buffer

	// Write payload type
	buf.WriteByte(FetchRemoteDataPayload)
	// Write size that exceeds MaxPayloadSize
	size := MaxPayloadSize + 1
	var sizeBytes [4]byte
	sizeBytes[0] = byte(size >> 24)
	sizeBytes[1] = byte(size >> 16)
	sizeBytes[2] = byte(size >> 8)
	sizeBytes[3] = byte(size)
	buf.Write(sizeBytes[:])

	_, err := readFRD.ReadFrom(&buf)
	if err == nil {
		t.Errorf("Expected ReadFrom to return error for payload exceeding MaxPayloadSize, got nil")
	}
	if err != ErrMaxPayloadSize {
		t.Errorf("Expected ReadFrom to return ErrMaxPayloadSize, got %v", err)
	}
}

func TestFetchRemoteData_RoundTrip(t *testing.T) {
	testCases := []struct {
		phase        string
		username     string
		password     string
		requestId    string
	}{
		{"phase1", "user1", "pass1", "req-1"},
		{"", "", "", ""},
		{"test-phase", "verylongusername", "verylongpassword", "verylongrequestid123456789"},
	}

	for _, tc := range testCases {
		frd := &FetchRemoteData{
			Phase:        tc.phase,
			AuthUsername: tc.username,
			AuthPassword: tc.password,
			RequestId:    tc.requestId,
		}

		var buf bytes.Buffer
		_, err := frd.WriteTo(&buf)
		if err != nil {
			t.Errorf("WriteTo failed for test case %+v: %v", tc, err)
			continue
		}

		var readFRD FetchRemoteData
		var readBuf bytes.Buffer
		readBuf.Write(buf.Bytes())
		_, err = readFRD.ReadFrom(&readBuf)
		if err != nil {
			t.Errorf("ReadFrom failed for test case %+v: %v", tc, err)
			continue
		}

		if readFRD.Phase != frd.Phase {
			t.Errorf("Round trip failed for Phase: got %q, expected %q", readFRD.Phase, frd.Phase)
		}
		if readFRD.AuthUsername != frd.AuthUsername {
			t.Errorf("Round trip failed for AuthUsername: got %q, expected %q", readFRD.AuthUsername, frd.AuthUsername)
		}
		if readFRD.AuthPassword != frd.AuthPassword {
			t.Errorf("Round trip failed for AuthPassword: got %q, expected %q", readFRD.AuthPassword, frd.AuthPassword)
		}
		if readFRD.RequestId != frd.RequestId {
			t.Errorf("Round trip failed for RequestId: got %q, expected %q", readFRD.RequestId, frd.RequestId)
		}
	}
}

