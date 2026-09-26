package network

import (
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"os"
	"time"
)

// NewDialer returns an initialized Dialer
func NewDialer(header byte, tlsConfig *tls.Config) *Dialer {
	return &Dialer{
		header:    header,
		tlsConfig: tlsConfig,
		logger:    defaultLogger,
	}
}

// Dialer supports dialing a cluster service.
type Dialer struct {
	header    byte
	tlsConfig *tls.Config
	logger    *log.Logger
}

var defaultLogger = log.New(os.Stderr, "[dialer] ", log.LstdFlags)

// Dial dials the cluster service at the given addr and returns a connection.
func (d *Dialer) Dial(addr string, timeout time.Duration) (conn net.Conn, retErr error) {
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	dialer := &net.Dialer{Timeout: timeout}
	protocol := "tcp"
	if d.tlsConfig != nil {
		protocol = "tls"
	}
	d.logger.Printf("dialing %s address %s with timeout %v (header: %d)", protocol, addr, timeout, d.header)

	if d.tlsConfig == nil {
		conn, retErr = dialer.Dial("tcp", addr)
	} else {
		conn, retErr = tls.DialWithDialer(dialer, "tcp", addr, d.tlsConfig)
	}
	if retErr != nil {
		// Provide more detailed error information for network issues
		if netErr, ok := retErr.(net.Error); ok {
			if netErr.Timeout() {
				d.logger.Printf("failed to establish connection to %s: timeout error: %v", addr, retErr)
			} else {
				d.logger.Printf("failed to establish connection to %s: network error: %v", addr, retErr)
			}
		} else if opErr, ok := retErr.(*net.OpError); ok {
			d.logger.Printf("failed to establish connection to %s: operation error: %v (op: %s, net: %s, err: %v)",
				addr, retErr, opErr.Op, opErr.Net, opErr.Err)
		} else {
			d.logger.Printf("failed to establish connection to %s: %v", addr, retErr)
		}
		return nil, retErr
	}

	defer func() {
		if retErr != nil && conn != nil {
			d.logger.Printf("closing connection to %s due to error: %v", addr, retErr)
			conn.Close()
		}
	}()

	// Write a marker byte to indicate message type.
	// Use a reasonable default timeout if the provided timeout is 0 or too small
	writeDeadline := timeout
	if writeDeadline == 0 || writeDeadline < 5*time.Second {
		writeDeadline = 5 * time.Second
		d.logger.Printf("using default write deadline %v for %s (provided timeout was %v)", writeDeadline, addr, timeout)
	}

	if err := conn.SetWriteDeadline(time.Now().Add(writeDeadline)); err != nil {
		retErr = fmt.Errorf("failed to set WriteDeadline for header: %s", err.Error())
		d.logger.Printf("error setting write deadline for %s: %v", addr, retErr)
		return nil, retErr
	}
	if _, err := conn.Write([]byte{d.header}); err != nil {
		retErr = err
		d.logger.Printf("error writing header byte to %s: %v", addr, retErr)
		return nil, retErr
	}

	// Clear the write deadline so raft can manage its own deadlines for subsequent operations
	if err := conn.SetWriteDeadline(time.Time{}); err != nil {
		retErr = fmt.Errorf("failed to clear WriteDeadline after header: %s", err.Error())
		d.logger.Printf("error clearing write deadline for %s: %v", addr, retErr)
		return nil, retErr
	}

	d.logger.Printf("successfully established connection to %s", addr)
	return conn, nil
}
