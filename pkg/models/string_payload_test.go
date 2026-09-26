package models

import (
	"bytes"
	"testing"
)

func TestString_Bytes(t *testing.T) {
	s := String("test string")
	bytes := s.Bytes()

	expected := []byte("test string")
	if string(bytes) != string(expected) {
		t.Errorf("Expected Bytes() to return %s, got %s", string(expected), string(bytes))
	}
}

func TestString_String(t *testing.T) {
	s := String("test string")
	str := s.String()

	if str != "test string" {
		t.Errorf("Expected String() to return 'test string', got %s", str)
	}
}

func TestString_WriteTo(t *testing.T) {
	s := String("test string")
	var buf bytes.Buffer

	n, err := s.WriteTo(&buf)
	if err != nil {
		t.Errorf("Expected WriteTo to succeed, got error: %v", err)
	}

	if n <= 0 {
		t.Errorf("Expected WriteTo to return positive number of bytes written, got %d", n)
	}

	// Verify the written data can be read back
	var readString String
	readN, readErr := readString.ReadFrom(&buf)
	if readErr != nil {
		t.Errorf("Expected ReadFrom to succeed after WriteTo, got error: %v", readErr)
	}

	if readN != n {
		t.Errorf("Expected ReadFrom to read same number of bytes as WriteTo wrote, wrote %d, read %d", n, readN)
	}

	if readString != s {
		t.Errorf("Expected ReadFrom to recover original string, got %s, expected %s", readString, s)
	}
}

func TestString_ReadFrom(t *testing.T) {
	// First write a string
	s := String("test string")
	var writeBuf bytes.Buffer
	_, _ = s.WriteTo(&writeBuf)

	// Now read it back
	var readString String
	var readBuf bytes.Buffer
	readBuf.Write(writeBuf.Bytes())

	n, err := readString.ReadFrom(&readBuf)
	if err != nil {
		t.Errorf("Expected ReadFrom to succeed, got error: %v", err)
	}

	if n <= 0 {
		t.Errorf("Expected ReadFrom to return positive number of bytes read, got %d", n)
	}

	if readString != s {
		t.Errorf("Expected ReadFrom to recover original string, got %s, expected %s", readString, s)
	}
}

func TestString_ReadFrom_InvalidType(t *testing.T) {
	var readString String
	var buf bytes.Buffer

	// Write an invalid payload type
	buf.WriteByte(NodeAuthPayload) // Wrong type
	buf.Write(make([]byte, 4))     // Size

	_, err := readString.ReadFrom(&buf)
	if err == nil {
		t.Errorf("Expected ReadFrom to return error for invalid payload type, got nil")
	}
}

func TestString_ReadFrom_Empty(t *testing.T) {
	var readString String
	var buf bytes.Buffer

	_, err := readString.ReadFrom(&buf)
	if err == nil {
		t.Errorf("Expected ReadFrom to return error for empty buffer, got nil")
	}
}

func TestString_RoundTrip(t *testing.T) {
	testCases := []string{
		"",
		"a",
		"test",
		"hello world",
		"special chars: !@#$%^&*()",
		"unicode: 你好世界",
		"very long string: " + string(make([]byte, 1000)),
	}

	for _, tc := range testCases {
		s := String(tc)
		var buf bytes.Buffer

		// Write
		_, err := s.WriteTo(&buf)
		if err != nil {
			t.Errorf("WriteTo failed for test case %q: %v", tc, err)
			continue
		}

		// Read
		var readString String
		_, err = readString.ReadFrom(&buf)
		if err != nil {
			t.Errorf("ReadFrom failed for test case %q: %v", tc, err)
			continue
		}

		if readString != s {
			t.Errorf("Round trip failed for test case %q: got %q, expected %q", tc, readString, s)
		}
	}
}

