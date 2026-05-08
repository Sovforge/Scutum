package tests

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"scutum/cmd/internal/utils"
)

func TestWSWriterSmallPayload(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	w := utils.WSWriter{Conn: server}
	msg := []byte("hello")

	expectedLen := 2 + len(msg)
	done := make(chan []byte, 1)
	go func() {
		buf := make([]byte, expectedLen)
		io.ReadFull(client, buf)
		done <- buf
	}()

	if _, err := w.Write(msg); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	frame := <-done
	if len(frame) < 2 {
		t.Fatal("frame too short")
	}
	if frame[0] != 0x82 {
		t.Errorf("frame[0] = %x, want 0x82", frame[0])
	}
	if frame[1] != byte(len(msg)) {
		t.Errorf("frame[1] = %d, want %d", frame[1], len(msg))
	}
	if !bytes.Equal(frame[2:], msg) {
		t.Errorf("payload = %q, want %q", frame[2:], msg)
	}
}

func TestWSWriterMediumPayload(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	w := utils.WSWriter{Conn: server}
	msg := make([]byte, 126)

	expectedLen := 4 + len(msg) // 2 bytes + 2 bytes for 16-bit len
	done := make(chan []byte, 1)
	go func() {
		buf := make([]byte, expectedLen)
		io.ReadFull(client, buf)
		done <- buf
	}()

	if _, err := w.Write(msg); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	frame := <-done
	if len(frame) != expectedLen {
		t.Fatalf("expected %d bytes, got %d", expectedLen, len(frame))
	}
	if frame[1] != 126 {
		t.Errorf("frame[1] = %d, want 126", frame[1])
	}
}

func TestReadWSFrame(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	payload := []byte("hello")
	mask := []byte{1, 2, 3, 4}
	masked := make([]byte, len(payload))
	for i, b := range payload {
		masked[i] = b ^ mask[i%4]
	}

	frame := []byte{0x81, byte(0x80 | len(payload))}
	frame = append(frame, mask...)
	frame = append(frame, masked...)

	go func() {
		client.Write(frame)
	}()

	got, err := utils.ReadWSFrame(server)
	if err != nil {
		t.Fatalf("ReadWSFrame() error = %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("ReadWSFrame() = %q, want %q", got, payload)
	}
}

func TestReadWSFrameRoundTrip(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	original := []byte("round trip")
	mask := []byte{0xAB, 0xCD, 0xEF, 0x01}
	masked := make([]byte, len(original))
	for i, b := range original {
		masked[i] = b ^ mask[i%4]
	}

	frame := []byte{0x81, byte(0x80 | len(original))}
	frame = append(frame, mask...)
	frame = append(frame, masked...)

	go func() { client.Write(frame) }()

	got, err := utils.ReadWSFrame(server)
	if err != nil {
		t.Fatalf("ReadWSFrame() error = %v", err)
	}
	if !bytes.Equal(got, original) {
		t.Errorf("got %q, want %q", got, original)
	}
}
func TestUpgradeToWebSocketSuccess(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	out := make(chan []byte, 1)
	go func() {
		data, _ := io.ReadAll(clientConn)
		out <- data
	}()

	rw := newHijackResponseWriter(serverConn)
	req := httptest.NewRequest(http.MethodGet, "http://example.com/ws", nil)
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")

	conn, err := utils.UpgradeToWebSocket(rw, req)
	if err != nil {
		t.Fatalf("UpgradeToWebSocket() error = %v", err)
	}
	conn.Close()

	buf := <-out
	if !strings.Contains(string(buf), "101 Switching Protocols") {
		t.Fatalf("unexpected handshake: %q", string(buf))
	}
}
