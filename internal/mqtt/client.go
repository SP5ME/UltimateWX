package mqtt

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Message struct {
	Topic   string
	Payload []byte
}

func Connect(broker, clientID, username, password, topic string) (net.Conn, error) {
	u, e := url.Parse(broker)
	if e != nil {
		return nil, e
	}
	if u.Scheme != "tcp" && u.Scheme != "mqtt" {
		return nil, fmt.Errorf("unsupported MQTT scheme %q", u.Scheme)
	}
	addr := u.Host
	if !strings.Contains(addr, ":") {
		addr = net.JoinHostPort(addr, "1883")
	}
	conn, e := net.DialTimeout("tcp", addr, 8*time.Second)
	if e != nil {
		return nil, e
	}
	flags := byte(0x02)
	if username != "" {
		flags |= 0x80
	}
	if password != "" {
		flags |= 0x40
	}
	var body []byte
	body = appendString(body, "MQTT")
	body = append(body, 4, flags, 0, 60)
	body = appendString(body, clientID)
	if username != "" {
		body = appendString(body, username)
	}
	if password != "" {
		body = appendString(body, password)
	}
	if e = writePacket(conn, 0x10, body); e != nil {
		conn.Close()
		return nil, e
	}
	conn.SetReadDeadline(time.Now().Add(8 * time.Second))
	h, ack, e := readPacket(conn)
	if e != nil {
		conn.Close()
		return nil, e
	}
	if h>>4 != 2 || len(ack) < 2 || ack[1] != 0 {
		conn.Close()
		return nil, fmt.Errorf("MQTT connection rejected (packet 0x%x)", h)
	}
	var sub []byte
	sub = append(sub, 0, 1)
	sub = appendString(sub, topic)
	sub = append(sub, 0)
	if e = writePacket(conn, 0x82, sub); e != nil {
		conn.Close()
		return nil, e
	}
	conn.SetReadDeadline(time.Now().Add(8 * time.Second))
	h, ack, e = readPacket(conn)
	if e != nil || h>>4 != 9 {
		conn.Close()
		if e != nil {
			return nil, e
		}
		return nil, fmt.Errorf("MQTT subscribe failed")
	}
	conn.SetReadDeadline(time.Time{})
	return conn, nil
}

func Read(conn net.Conn) (Message, error) {
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	h, p, e := readPacket(conn)
	if e != nil {
		return Message{}, e
	}
	if h>>4 != 3 || len(p) < 2 {
		return Message{}, nil
	}
	n := int(binary.BigEndian.Uint16(p[:2]))
	if n+2 > len(p) {
		return Message{}, fmt.Errorf("invalid MQTT topic")
	}
	return Message{Topic: string(p[2 : 2+n]), Payload: append([]byte(nil), p[2+n:]...)}, nil
}
func Ping(conn net.Conn) error { return writePacket(conn, 0xC0, nil) }
func writePacket(w io.Writer, header byte, payload []byte) error {
	var fixed []byte
	fixed = append(fixed, header)
	n := len(payload)
	for {
		digit := byte(n % 128)
		n /= 128
		if n > 0 {
			digit |= 0x80
		}
		fixed = append(fixed, digit)
		if n == 0 {
			break
		}
	}
	fixed = append(fixed, payload...)
	_, e := w.Write(fixed)
	return e
}
func readPacket(r io.Reader) (byte, []byte, error) {
	h := []byte{0}
	if _, e := io.ReadFull(r, h); e != nil {
		return 0, nil, e
	}
	multiplier, n := 1, 0
	for i := 0; i < 4; i++ {
		var d [1]byte
		if _, e := io.ReadFull(r, d[:]); e != nil {
			return 0, nil, e
		}
		n += int(d[0]&127) * multiplier
		if d[0]&128 == 0 {
			if n > 1<<20 {
				return 0, nil, fmt.Errorf("MQTT packet too large")
			}
			p := make([]byte, n)
			_, e := io.ReadFull(r, p)
			return h[0], p, e
		}
		multiplier *= 128
	}
	return 0, nil, fmt.Errorf("invalid MQTT remaining length")
}
func appendString(b []byte, s string) []byte {
	if len(s) > 65535 {
		return b
	}
	var n [2]byte
	binary.BigEndian.PutUint16(n[:], uint16(len(s)))
	b = append(b, n[:]...)
	return append(b, []byte(s)...)
}
func BrokerAddress(broker string) (string, error) {
	u, e := url.Parse(broker)
	if e != nil {
		return "", e
	}
	if u.Hostname() == "" {
		return "", fmt.Errorf("MQTT broker host missing")
	}
	port := u.Port()
	if port == "" {
		port = "1883"
	}
	if _, e = strconv.Atoi(port); e != nil {
		return "", e
	}
	return net.JoinHostPort(u.Hostname(), port), nil
}
