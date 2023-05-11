package network

import (
	"errors"
	"io"
	"math"
	"net"
	"strings"
	"testing"
	"time"
)

func TestNewDialer(t *testing.T) {
	tests := []struct {
		name   string
		header byte
	}{
		{"Test case 1", 0x01},
		{"Test case 2", 0x02},
		{"Test case 3", 0xFF},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dialer := NewDialer(tt.header)
			if dialer == nil {
				t.Fatal("NewDialer returned nil")
			}
			if dialer.header != tt.header {
				t.Errorf("NewDialer: expected header %x, got %x", tt.header, dialer.header)
			}
		})
	}
}

func TestDial(t *testing.T) {
	// Mock server to listen for incoming connections.
	mockServer := func(header byte) net.Listener {
		ln, _ := net.Listen("tcp", "127.0.0.1:0")
		go func() {
			conn, _ := ln.Accept()
			defer conn.Close()

			buf := make([]byte, 1)
			_, _ = io.ReadFull(conn, buf)

			if buf[0] != header {
				t.Errorf("Header byte mismatch: expected %x, got %x", header, buf[0])
			}
		}()
		return ln
	}

	tests := []struct {
		name     string
		addr     string
		timeout  time.Duration
		header   byte
		wantErr  bool
		wantConn bool
	}{
		{"Valid address", "", time.Second, 0x01, false, true},
		{"Invalid address", "invalid:address", time.Second, 0x01, true, false},
		{"Short timeout", "", time.Millisecond, 0x01, true, false},
		{"Reasonable timeout", "", time.Second, 0x01, false, true},
		{"Different header bytes", "", time.Second, 0x02, false, true},

		// New test cases for edge cases and input validation
		{"Empty address", "", time.Second, 0x01, true, false},
		{"Malformed address", "127.0.0.1:notaport", time.Second, 0x01, true, false},
		{"Negative timeout", "", -1 * time.Second, 0x01, true, false},
		{"Extremely large timeout", "", time.Duration(math.MaxInt64), 0x01, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantConn {
				ln := mockServer(tt.header)
				defer ln.Close()
				tt.addr = ln.Addr().String()
			}

			dialer := NewDialer(tt.header)
			conn, err := dialer.Dial(tt.addr, tt.timeout)

			if tt.wantErr && err == nil {
				t.Fatal("Expected an error, but got none")
			}

			if !tt.wantErr && err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if tt.wantConn && conn == nil {
				t.Fatal("Expected a connection, but got nil")
			}

			if conn != nil {
				conn.Close()
			}
		})
	}
}

type errorConn struct {
	net.Conn
	setWriteDeadlineError bool
	writeError            bool
}

func (ec *errorConn) SetWriteDeadline(t time.Time) error {
	if ec.setWriteDeadlineError {
		return errors.New("SetWriteDeadline failed")
	}
	return ec.Conn.SetWriteDeadline(t)
}

func (ec *errorConn) Write(b []byte) (int, error) {
	if ec.writeError {
		return 0, errors.New("Write failed")
	}
	return ec.Conn.Write(b)
}

func TestDialErrorHandling(t *testing.T) {
	// Mock server to accept incoming connections and return custom errorConn.
	mockServer := func() net.Listener {
		ln, _ := net.Listen("tcp", "127.0.0.1:0")
		go func() {
			conn, _ := ln.Accept()
			errorConn := &errorConn{Conn: conn, setWriteDeadlineError: true}
			defer errorConn.Close()
		}()
		return ln
	}

	tests := []struct {
		name                  string
		setWriteDeadlineError bool
		writeError            bool
	}{
		{"SetWriteDeadline error", true, false},
		{"Write error", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ln := mockServer()
			defer ln.Close()

			addr := ln.Addr().String()
			dialer := NewDialer(0x01)
			_, err := dialer.Dial(addr, time.Second)

			if err == nil {
				t.Fatal("Expected an error, but got none")
			}

			if tt.setWriteDeadlineError && !strings.Contains(err.Error(), "failed to set WriteDeadline") {
				t.Fatalf("Expected SetWriteDeadline error, got: %v", err)
			}

			if tt.writeError && !strings.Contains(err.Error(), "Write failed") {
				t.Fatalf("Expected Write error, got: %v", err)
			}
		})
	}
}
