package models

import (
	"bytes"
	"testing"
)

func TestNodeAuth_Bytes(t *testing.T) {
	na := &NodeAuth{
		AuthUsername: "testuser",
		AuthPassword: "testpass",
	}

	bytes := na.Bytes()
	if len(bytes) == 0 {
		t.Errorf("Expected Bytes() to return non-empty data")
	}
}

func TestNodeAuth_String(t *testing.T) {
	na := &NodeAuth{
		AuthUsername: "testuser",
		AuthPassword: "testpass",
	}

	str := na.String()
	if len(str) == 0 {
		t.Errorf("Expected String() to return non-empty string")
	}
}

func TestNodeAuth_WriteTo(t *testing.T) {
	na := &NodeAuth{
		AuthUsername: "testuser",
		AuthPassword: "testpass",
	}

	var buf bytes.Buffer
	n, err := na.WriteTo(&buf)
	if err != nil {
		t.Errorf("Expected WriteTo to succeed, got error: %v", err)
	}

	if n <= 0 {
		t.Errorf("Expected WriteTo to return positive number of bytes written, got %d", n)
	}
}

func TestNodeAuth_ReadFrom(t *testing.T) {
	// First write a NodeAuth
	na := &NodeAuth{
		AuthUsername: "testuser",
		AuthPassword: "testpass",
	}
	var writeBuf bytes.Buffer
	_, _ = na.WriteTo(&writeBuf)

	// Now read it back
	var readNA NodeAuth
	var readBuf bytes.Buffer
	readBuf.Write(writeBuf.Bytes())

	n, err := readNA.ReadFrom(&readBuf)
	if err != nil {
		t.Errorf("Expected ReadFrom to succeed, got error: %v", err)
	}

	if n <= 0 {
		t.Errorf("Expected ReadFrom to return positive number of bytes read, got %d", n)
	}

	if readNA.AuthUsername != na.AuthUsername {
		t.Errorf("Expected ReadFrom to recover AuthUsername, got %s, expected %s", readNA.AuthUsername, na.AuthUsername)
	}

	if readNA.AuthPassword != na.AuthPassword {
		t.Errorf("Expected ReadFrom to recover AuthPassword, got %s, expected %s", readNA.AuthPassword, na.AuthPassword)
	}
}

func TestNodeAuth_ReadFrom_InvalidType(t *testing.T) {
	var readNA NodeAuth
	var buf bytes.Buffer

	// Write an invalid payload type
	buf.WriteByte(StringPayload) // Wrong type
	buf.Write(make([]byte, 4))   // Size

	_, err := readNA.ReadFrom(&buf)
	if err == nil {
		t.Errorf("Expected ReadFrom to return error for invalid payload type, got nil")
	}
}

func TestNodeAuth_ReadFrom_Empty(t *testing.T) {
	var readNA NodeAuth
	var buf bytes.Buffer

	_, err := readNA.ReadFrom(&buf)
	if err == nil {
		t.Errorf("Expected ReadFrom to return error for empty buffer, got nil")
	}
}

func TestNodeAuth_ReadFrom_MaxPayloadSize(t *testing.T) {
	var readNA NodeAuth
	var buf bytes.Buffer

	// Write payload type
	buf.WriteByte(NodeAuthPayload)
	// Write size that exceeds MaxPayloadSize
	size := MaxPayloadSize + 1
	var sizeBytes [4]byte
	sizeBytes[0] = byte(size >> 24)
	sizeBytes[1] = byte(size >> 16)
	sizeBytes[2] = byte(size >> 8)
	sizeBytes[3] = byte(size)
	buf.Write(sizeBytes[:])

	_, err := readNA.ReadFrom(&buf)
	if err == nil {
		t.Errorf("Expected ReadFrom to return error for payload exceeding MaxPayloadSize, got nil")
	}
	if err != ErrMaxPayloadSize {
		t.Errorf("Expected ReadFrom to return ErrMaxPayloadSize, got %v", err)
	}
}

func TestNodeAuth_RoundTrip(t *testing.T) {
	testCases := []struct {
		username string
		password string
	}{
		{"user1", "pass1"},
		{"", ""},
		{"verylongusername123456789", "verylongpassword123456789"},
		{"user@domain.com", "p@ssw0rd!"},
	}

	for _, tc := range testCases {
		na := &NodeAuth{
			AuthUsername: tc.username,
			AuthPassword: tc.password,
		}

		var buf bytes.Buffer
		_, err := na.WriteTo(&buf)
		if err != nil {
			t.Errorf("WriteTo failed for test case %+v: %v", tc, err)
			continue
		}

		var readNA NodeAuth
		var readBuf bytes.Buffer
		readBuf.Write(buf.Bytes())
		_, err = readNA.ReadFrom(&readBuf)
		if err != nil {
			t.Errorf("ReadFrom failed for test case %+v: %v", tc, err)
			continue
		}

		if readNA.AuthUsername != na.AuthUsername {
			t.Errorf("Round trip failed for username: got %q, expected %q", readNA.AuthUsername, na.AuthUsername)
		}
		if readNA.AuthPassword != na.AuthPassword {
			t.Errorf("Round trip failed for password: got %q, expected %q", readNA.AuthPassword, na.AuthPassword)
		}
	}
}

