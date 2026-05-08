package utils

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
)

// UpgradeToWebSocket performs the WebSocket handshake and returns the hijacked connection.
func UpgradeToWebSocket(w http.ResponseWriter, r *http.Request) (net.Conn, error) {
	if r.Header.Get("Upgrade") != "websocket" {
		http.Error(w, "not a websocket upgrade", http.StatusBadRequest)
		return nil, fmt.Errorf("not a websocket upgrade")
	}

	const magic = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
	key := r.Header.Get("Sec-WebSocket-Key")
	sum := sha1.Sum([]byte(key + magic))
	acceptStr := base64.StdEncoding.EncodeToString(sum[:])

	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "hijacking not supported", http.StatusInternalServerError)
		return nil, fmt.Errorf("webserver doesn't support hijacking")
	}
	conn, bufrw, err := hj.Hijack()
	if err != nil {
		return nil, err
	}

	bufrw.WriteString("HTTP/1.1 101 Switching Protocols\r\n")
	bufrw.WriteString("Upgrade: websocket\r\n")
	bufrw.WriteString("Connection: Upgrade\r\n")
	bufrw.WriteString("Sec-WebSocket-Accept: " + acceptStr + "\r\n\r\n")
	bufrw.Flush()

	return conn, nil
}

// WriteWSFrame sends data to the browser as a binary WebSocket frame.
// Server-to-client frames are never masked. Supports payloads of any size.
func WriteWSFrame(conn net.Conn, data []byte) error {
	n := len(data)

	// Byte 0: FIN=1, opcode=2 (binary)
	var header [10]byte
	header[0] = 0x82
	headerLen := 2

	switch {
	case n <= 125:
		header[1] = byte(n)
	case n <= 65535:
		header[1] = 126
		binary.BigEndian.PutUint16(header[2:], uint16(n))
		headerLen = 4
	default:
		header[1] = 127
		binary.BigEndian.PutUint64(header[2:], uint64(n))
		headerLen = 10
	}

	if _, err := conn.Write(header[:headerLen]); err != nil {
		return err
	}
	_, err := conn.Write(data)
	return err
}

// ReadWSFrame reads and unmasks a single WebSocket frame from the browser.
// Supports all three WebSocket payload length encodings.
func ReadWSFrame(conn net.Conn) ([]byte, error) {
	// Read first two header bytes.
	var hdr [2]byte
	if _, err := io.ReadFull(conn, hdr[:]); err != nil {
		return nil, err
	}

	// opcode := hdr[0] & 0x0F  // not inspected; treat all as data
	masked := (hdr[1] & 0x80) != 0
	rawLen := hdr[1] & 0x7F

	var payloadLen int64
	switch rawLen {
	case 126:
		var ext [2]byte
		if _, err := io.ReadFull(conn, ext[:]); err != nil {
			return nil, err
		}
		payloadLen = int64(binary.BigEndian.Uint16(ext[:]))
	case 127:
		var ext [8]byte
		if _, err := io.ReadFull(conn, ext[:]); err != nil {
			return nil, err
		}
		payloadLen = int64(binary.BigEndian.Uint64(ext[:]))
	default:
		payloadLen = int64(rawLen)
	}

	// Read optional masking key (always present for browser→server).
	var mask [4]byte
	if masked {
		if _, err := io.ReadFull(conn, mask[:]); err != nil {
			return nil, err
		}
	}

	payload := make([]byte, payloadLen)
	if _, err := io.ReadFull(conn, payload); err != nil {
		return nil, err
	}

	if masked {
		for i := range payload {
			payload[i] ^= mask[i%4]
		}
	}

	return payload, nil
}

// WSWriter wraps a raw net.Conn to send data as WebSocket binary frames.
type WSWriter struct {
	Conn net.Conn
}

func (w WSWriter) Write(p []byte) (int, error) {
	if err := WriteWSFrame(w.Conn, p); err != nil {
		return 0, err
	}
	return len(p), nil
}
