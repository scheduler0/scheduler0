package models

import (
	"bytes"
	"testing"
)

func TestLocalQuotaRequest_Bytes(t *testing.T) {
	lqr := &LocalQuotaRequest{
		AuthUsername: "testuser",
		AuthPassword: "testpass",
	}

	bytes := lqr.Bytes()
	if len(bytes) == 0 {
		t.Errorf("Expected Bytes() to return non-empty data")
	}
}

func TestLocalQuotaRequest_String(t *testing.T) {
	lqr := &LocalQuotaRequest{
		AuthUsername: "testuser",
		AuthPassword: "testpass",
	}

	str := lqr.String()
	if str != "LocalQuotaRequest" {
		t.Errorf("Expected String() to return 'LocalQuotaRequest', got %s", str)
	}
}

func TestLocalQuotaRequest_WriteTo(t *testing.T) {
	lqr := &LocalQuotaRequest{
		AuthUsername: "testuser",
		AuthPassword: "testpass",
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

func TestLocalQuotaRequest_ReadFrom(t *testing.T) {
	// First write a LocalQuotaRequest
	lqr := &LocalQuotaRequest{
		AuthUsername: "testuser",
		AuthPassword: "testpass",
	}
	var writeBuf bytes.Buffer
	_, _ = lqr.WriteTo(&writeBuf)

	// Now read it back
	var readLQR LocalQuotaRequest
	var readBuf bytes.Buffer
	readBuf.Write(writeBuf.Bytes())

	n, err := readLQR.ReadFrom(&readBuf)
	if err != nil {
		t.Errorf("Expected ReadFrom to succeed, got error: %v", err)
	}

	if n <= 0 {
		t.Errorf("Expected ReadFrom to return positive number of bytes read, got %d", n)
	}

	if readLQR.AuthUsername != lqr.AuthUsername {
		t.Errorf("Expected ReadFrom to recover AuthUsername, got %s, expected %s", readLQR.AuthUsername, lqr.AuthUsername)
	}

	if readLQR.AuthPassword != lqr.AuthPassword {
		t.Errorf("Expected ReadFrom to recover AuthPassword, got %s, expected %s", readLQR.AuthPassword, lqr.AuthPassword)
	}
}

func TestLocalQuotaRequest_ReadFrom_InvalidType(t *testing.T) {
	var readLQR LocalQuotaRequest
	var buf bytes.Buffer

	// Write an invalid payload type
	buf.WriteByte(StringPayload) // Wrong type
	buf.Write(make([]byte, 4))  // Size

	_, err := readLQR.ReadFrom(&buf)
	if err == nil {
		t.Errorf("Expected ReadFrom to return error for invalid payload type, got nil")
	}
}

func TestLocalQuotaRequest_ReadFrom_Empty(t *testing.T) {
	var readLQR LocalQuotaRequest
	var buf bytes.Buffer

	_, err := readLQR.ReadFrom(&buf)
	if err == nil {
		t.Errorf("Expected ReadFrom to return error for empty buffer, got nil")
	}
}

func TestLocalQuotaRequest_RoundTrip(t *testing.T) {
	testCases := []struct {
		username string
		password string
	}{
		{"user1", "pass1"},
		{"", ""},
		{"verylongusername123456789", "verylongpassword123456789"},
	}

	for _, tc := range testCases {
		lqr := &LocalQuotaRequest{
			AuthUsername: tc.username,
			AuthPassword: tc.password,
		}

		var buf bytes.Buffer
		_, err := lqr.WriteTo(&buf)
		if err != nil {
			t.Errorf("WriteTo failed for test case %+v: %v", tc, err)
			continue
		}

		var readLQR LocalQuotaRequest
		var readBuf bytes.Buffer
		readBuf.Write(buf.Bytes())
		_, err = readLQR.ReadFrom(&readBuf)
		if err != nil {
			t.Errorf("ReadFrom failed for test case %+v: %v", tc, err)
			continue
		}

		if readLQR.AuthUsername != lqr.AuthUsername {
			t.Errorf("Round trip failed for username: got %q, expected %q", readLQR.AuthUsername, lqr.AuthUsername)
		}
		if readLQR.AuthPassword != lqr.AuthPassword {
			t.Errorf("Round trip failed for password: got %q, expected %q", readLQR.AuthPassword, lqr.AuthPassword)
		}
	}
}

