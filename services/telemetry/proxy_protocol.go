package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"time"
)

var proxyV2Signature = []byte("\r\n\r\n\x00\r\nQUIT\n")

// proxyV2Listener trusts PROXY headers only because the underlying collector
// listener is loopback-only and systemd denies every non-loopback socket.
type proxyV2Listener struct {
	net.Listener
}

func (l proxyV2Listener) Accept() (net.Conn, error) {
	for {
		conn, err := l.Listener.Accept()
		if err != nil {
			return nil, err
		}
		wrapped, err := readProxyV2(conn)
		if err != nil {
			_ = conn.Close()
			continue
		}
		return wrapped, nil
	}
}

type proxyV2Conn struct {
	net.Conn
	remote net.Addr
}

func (c *proxyV2Conn) RemoteAddr() net.Addr { return c.remote }

func readProxyV2(conn net.Conn) (net.Conn, error) {
	if err := conn.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		return nil, fmt.Errorf("set proxy header deadline: %w", err)
	}
	var header [16]byte
	if _, err := io.ReadFull(conn, header[:]); err != nil {
		return nil, fmt.Errorf("read proxy header: %w", err)
	}
	if !bytes.Equal(header[:12], proxyV2Signature) || header[12] != 0x21 {
		return nil, errors.New("invalid PROXY protocol v2 header")
	}
	length := int(binary.BigEndian.Uint16(header[14:16]))
	if length < 0 || length > 216 {
		return nil, errors.New("invalid PROXY protocol v2 address length")
	}
	address := make([]byte, length)
	if _, err := io.ReadFull(conn, address); err != nil {
		return nil, fmt.Errorf("read proxy address: %w", err)
	}
	var remote *net.TCPAddr
	switch header[13] {
	case 0x11:
		if len(address) < 12 {
			return nil, errors.New("short PROXY protocol IPv4 address")
		}
		remote = &net.TCPAddr{IP: net.IP(address[:4]), Port: int(binary.BigEndian.Uint16(address[8:10]))}
	case 0x21:
		if len(address) < 36 {
			return nil, errors.New("short PROXY protocol IPv6 address")
		}
		remote = &net.TCPAddr{IP: net.IP(address[:16]), Port: int(binary.BigEndian.Uint16(address[32:34]))}
	default:
		return nil, errors.New("unsupported PROXY protocol address family")
	}
	if err := conn.SetReadDeadline(time.Time{}); err != nil {
		return nil, fmt.Errorf("clear proxy header deadline: %w", err)
	}
	return &proxyV2Conn{Conn: conn, remote: remote}, nil
}
