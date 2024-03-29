package network

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"
)

const (
	// DefaultTimeout is the default length of time to wait for first byte.
	DefaultTimeout = 30 * time.Second
)

type Mux struct {
	ln   net.Listener
	m    map[byte]*listener
	addr net.Addr

	logger *log.Logger
	wg     sync.WaitGroup
}

func NewMux(ln net.Listener, adv net.Addr) *Mux {
	addr := adv
	if addr == nil {
		addr = ln.Addr()
	}

	return &Mux{
		ln:   ln,
		addr: addr,
		m:    make(map[byte]*listener),
	}
}

// Serve handles connections from ln and multiplexes then across registered listener.
func (mux *Mux) Serve() error {
	for {
		// Wait for the next connection.
		// If it returns a temporary error then simply retry.
		// If it returns any other error then exit immediately.
		conn, err := mux.ln.Accept()
		if err, ok := err.(interface {
			Temporary() bool
		}); ok && err.Temporary() {
			continue
		}
		if err != nil {
			// Wait for all connections to be demuxed
			mux.wg.Wait()
			mux.logger.Println("closing connection due to error", err.Error())

			for _, ln := range mux.m {
				close(ln.c)
			}
			return err
		}

		// Demux in a goroutine to
		mux.wg.Add(1)
		go mux.handleConn(conn)
	}
}

// Layer represents the connection between nodes. It can be both used to
// make connections to other nodes, and receive connections from other
// nodes.
type Layer struct {
	ln   net.Listener
	addr net.Addr

	dialer *Dialer
}

// Dial creates a new network connection.
func (l *Layer) Dial(addr string, timeout time.Duration) (net.Conn, error) {
	return l.dialer.Dial(addr, timeout)
}

// Accept waits for the next connection.
func (l *Layer) Accept() (net.Conn, error) { return l.ln.Accept() }

// Close closes the layer.
func (l *Layer) Close() error { return l.ln.Close() }

// Addr returns the local address for the layer.
func (l *Layer) Addr() net.Addr {
	return l.addr
}

// Listen returns a Layer associated with the given header. Any connection
// accepted by mux is multiplexed based on the initial header byte.
func (mux *Mux) Listen(header byte) *Layer {
	// Ensure two listeners are not created for the same header byte.
	if _, ok := mux.m[header]; ok {
		panic(fmt.Sprintf("listener already registered under header byte: %d", header))
	}

	// Create a new listener and assign it.
	ln := &listener{
		c:      make(chan net.Conn),
		logger: mux.logger,
	}
	mux.m[header] = ln

	layer := &Layer{
		ln:   ln,
		addr: mux.addr,
	}
	layer.dialer = NewDialer(header)

	return layer
}

func (mux *Mux) handleConn(conn net.Conn) {

	defer mux.wg.Done()
	// Set a read deadline so connections with no data don't timeout.
	if err := conn.SetReadDeadline(time.Now().Add(DefaultTimeout)); err != nil {
		log.Println("closing connection due to error", err.Error())
		conn.Close()
		return
	}

	// Read first byte from connection to determine handler.
	var typ [1]byte
	if _, err := io.ReadFull(conn, typ[:]); err != nil {
		log.Println("closing connection due to error", err.Error())
		conn.Close()
		return
	}

	// Reset read deadline and let the listener handle that.
	if err := conn.SetReadDeadline(time.Time{}); err != nil {
		log.Println("closing connection due to error", err.Error())
		conn.Close()
		return
	}

	// Retrieve handler based on first byte.
	handler := mux.m[typ[0]]
	if handler == nil {
		log.Println("closing connection due to error because no handler for ", conn.RemoteAddr().String())
		conn.Close()
		return
	}

	// Send connection to handler.  The handler is responsible for closing the connection.
	handler.c <- conn
}

// listener is a receiver for connections received by Mux.
type listener struct {
	c      chan net.Conn
	logger *log.Logger
}

// Accept waits for and returns the next connection to the listener.
func (ln *listener) Accept() (c net.Conn, err error) {
	conn, ok := <-ln.c
	if !ok {
		return nil, errors.New("network connection closed")
	}
	return conn, nil
}

// Close is a no-op. The mux's listener should be closed instead.
func (ln *listener) Close() error { return nil }

// Addr always returns nil
func (ln *listener) Addr() net.Addr { return nil }
