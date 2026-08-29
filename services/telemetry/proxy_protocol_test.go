package main

import (
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"
)

func TestReadProxyV2PreservesSourceAndHTTPBytes(t *testing.T) {
	server, client := net.Pipe()
	done := make(chan error, 1)
	go func() {
		defer client.Close()
		header := make([]byte, 16+12)
		copy(header, proxyV2Signature)
		header[12] = 0x21
		header[13] = 0x11
		binary.BigEndian.PutUint16(header[14:16], 12)
		copy(header[16:20], net.ParseIP("203.0.113.9").To4())
		copy(header[20:24], net.ParseIP("127.0.0.1").To4())
		binary.BigEndian.PutUint16(header[24:26], 54321)
		binary.BigEndian.PutUint16(header[26:28], 8443)
		if _, err := client.Write(append(header, []byte("GET /healthz HTTP/1.1\r\n")...)); err != nil {
			done <- err
			return
		}
		done <- nil
	}()

	wrapped, err := readProxyV2(server)
	if err != nil {
		t.Fatalf("readProxyV2: %v", err)
	}
	defer wrapped.Close()
	if got, want := wrapped.RemoteAddr().String(), "203.0.113.9:54321"; got != want {
		t.Fatalf("RemoteAddr = %q, want %q", got, want)
	}
	body := make([]byte, len("GET /healthz HTTP/1.1\r\n"))
	if _, err := io.ReadFull(wrapped, body); err != nil {
		t.Fatalf("read HTTP bytes: %v", err)
	}
	if string(body) != "GET /healthz HTTP/1.1\r\n" {
		t.Fatalf("HTTP bytes = %q", body)
	}
	if err := <-done; err != nil {
		t.Fatalf("write proxy fixture: %v", err)
	}
}

func TestProxyV2ListenerSurvivesMalformedConnection(t *testing.T) {
	raw, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer raw.Close()
	listener := proxyV2Listener{Listener: raw}
	accepted := make(chan net.Conn, 1)
	acceptErrors := make(chan error, 1)
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			acceptErrors <- acceptErr
			return
		}
		accepted <- conn
	}()

	malformed, err := net.Dial("tcp", raw.Addr().String())
	if err != nil {
		t.Fatalf("dial malformed: %v", err)
	}
	_, _ = malformed.Write([]byte("GET / HTTP/1.1\r\n\r\n"))
	_ = malformed.Close()

	valid, err := net.Dial("tcp", raw.Addr().String())
	if err != nil {
		t.Fatalf("dial valid: %v", err)
	}
	defer valid.Close()
	header := make([]byte, 16+12)
	copy(header, proxyV2Signature)
	header[12] = 0x21
	header[13] = 0x11
	binary.BigEndian.PutUint16(header[14:16], 12)
	copy(header[16:20], net.ParseIP("198.51.100.7").To4())
	copy(header[20:24], net.ParseIP("127.0.0.1").To4())
	binary.BigEndian.PutUint16(header[24:26], 40000)
	binary.BigEndian.PutUint16(header[26:28], 8443)
	if _, err := valid.Write(header); err != nil {
		t.Fatalf("write valid header: %v", err)
	}
	select {
	case conn := <-accepted:
		defer conn.Close()
		if got := conn.RemoteAddr().String(); got != "198.51.100.7:40000" {
			t.Fatalf("RemoteAddr = %q", got)
		}
	case err := <-acceptErrors:
		t.Fatalf("Accept stopped after malformed connection: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("Accept did not recover after malformed connection")
	}
}

func TestReadProxyV2RejectsPlainHTTP(t *testing.T) {
	server, client := net.Pipe()
	go func() {
		defer client.Close()
		_, _ = client.Write([]byte("GET / HTTP/1.1\r\n\r\n"))
	}()
	if _, err := readProxyV2(server); err == nil {
		t.Fatal("readProxyV2 accepted a connection without a trusted proxy header")
	}
	_ = server.Close()
}
