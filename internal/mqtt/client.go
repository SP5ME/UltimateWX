package mqtt

import (
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/url"
	"strconv"
	"time"
)

type Message struct {
	Topic    string
	Payload  []byte
	QoS      byte
	PacketID uint16
}

func Connect(broker, clientID, username, password, topic string) (net.Conn, error) {
	return ConnectWithQoS(broker, clientID, username, password, topic, 0)
}

// ConnectWithQoS subscribes to one MQTT topic at the requested QoS level.
// QoS 0 and 1 are supported by this small MQTT 3.1.1 client.
func ConnectWithQoS(broker, clientID, username, password, topic string, qos byte) (net.Conn, error) {
	if qos > 1 {
		return nil, fmt.Errorf("unsupported MQTT subscription QoS %d", qos)
	}
	u, e := url.Parse(broker)
	if e != nil {
		return nil, e
	}
	if u.Scheme != "tcp" && u.Scheme != "mqtt" && u.Scheme != "mqtts" {
		return nil, fmt.Errorf("unsupported MQTT scheme %q", u.Scheme)
	}
	addr, e := BrokerAddress(broker)
	if e != nil {
		return nil, e
	}
	dialer := &net.Dialer{Timeout: 8 * time.Second}
	var conn net.Conn
	if u.Scheme == "mqtts" {
		conn, e = tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{ServerName: u.Hostname(), MinVersion: tls.VersionTLS12})
	} else {
		conn, e = dialer.Dial("tcp", addr)
	}
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
	sub = append(sub, qos)
	if e = writePacket(conn, 0x82, sub); e != nil {
		conn.Close()
		return nil, e
	}
	conn.SetReadDeadline(time.Now().Add(8 * time.Second))
	h, ack, e = readPacket(conn)
	if e != nil || h>>4 != 9 || len(ack) < 3 || binary.BigEndian.Uint16(ack[:2]) != 1 || ack[2] > 2 {
		conn.Close()
		if e != nil {
			return nil, e
		}
		return nil, fmt.Errorf("MQTT subscribe failed or broker rejected the topic")
	}
	conn.SetReadDeadline(time.Time{})
	return conn, nil
}

func Read(conn net.Conn) (Message, error) {
	message, err := ReadUnacked(conn)
	if err != nil {
		return Message{}, err
	}
	if message.QoS == 1 {
		if err := Acknowledge(conn, message); err != nil {
			return Message{}, err
		}
	}
	return message, nil
}

// ReadUnacked reads a PUBLISH without acknowledging a QoS 1 message. Call
// Acknowledge after the application has accepted the payload.
func ReadUnacked(conn net.Conn) (Message, error) {
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
	offset := 2 + n
	qos := (h >> 1) & 0x03
	if qos == 3 {
		return Message{}, fmt.Errorf("invalid MQTT publish QoS")
	}
	if qos > 0 {
		if offset+2 > len(p) {
			return Message{}, fmt.Errorf("invalid MQTT publish packet identifier")
		}
		packetID := binary.BigEndian.Uint16(p[offset : offset+2])
		offset += 2
		if qos == 2 {
			return Message{}, fmt.Errorf("MQTT QoS 2 publish is not supported")
		}
		return Message{Topic: string(p[2 : 2+n]), Payload: append([]byte(nil), p[offset:]...), QoS: qos, PacketID: packetID}, nil
	}
	return Message{Topic: string(p[2 : 2+n]), Payload: append([]byte(nil), p[offset:]...), QoS: qos}, nil
}

func Acknowledge(conn net.Conn, message Message) error {
	if message.QoS != 1 || message.PacketID == 0 {
		return nil
	}
	var packetID [2]byte
	binary.BigEndian.PutUint16(packetID[:], message.PacketID)
	if err := writePacket(conn, 0x40, packetID[:]); err != nil {
		return fmt.Errorf("acknowledge MQTT publish: %w", err)
	}
	return nil
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
	for len(fixed) > 0 {
		written, err := w.Write(fixed)
		if err != nil {
			return err
		}
		if written == 0 {
			return io.ErrShortWrite
		}
		fixed = fixed[written:]
	}
	return nil
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
	if u.Scheme != "tcp" && u.Scheme != "mqtt" && u.Scheme != "mqtts" {
		return "", fmt.Errorf("unsupported MQTT scheme %q", u.Scheme)
	}
	port := u.Port()
	if port == "" {
		port = "1883"
		if u.Scheme == "mqtts" {
			port = "8883"
		}
	}
	if _, e = strconv.Atoi(port); e != nil {
		return "", e
	}
	return net.JoinHostPort(u.Hostname(), port), nil
}
