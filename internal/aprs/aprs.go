package aprs

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ultimatewx/ultimatewx/internal/weather"
)

type Frame struct {
	Raw        string                `json:"raw"`
	Station    string                `json:"station"`
	ReceivedAt time.Time             `json:"received_at"`
	IsWX       bool                  `json:"is_wx"`
	Eligible   bool                  `json:"eligible"`
	Reason     string                `json:"reason"`
	Latitude   float64               `json:"latitude,omitempty"`
	Longitude  float64               `json:"longitude,omitempty"`
	Data       []weather.Observation `json:"data,omitempty"`
}

var positionRE = regexp.MustCompile(`([0-9]{4}\.[0-9]{2})([NS])[/\\]([0-9]{5}\.[0-9]{2})([EW])`)
var weatherFieldRE = regexp.MustCompile(`(?:t-?[0-9]{3}|h(?:[0-9]{2}|//)|b[0-9]{5}|[rgpP][0-9]{3}|[sg][0-9]{3})`)

func ParseTNC2(line string, received time.Time, centerLat, centerLon, radiusKM float64) (Frame, error) {
	line = strings.TrimSpace(line)
	sep := strings.IndexByte(line, ':')
	if sep <= 0 {
		return Frame{}, fmt.Errorf("invalid TNC2 frame")
	}
	header, info := line[:sep], line[sep+1:]
	gt := strings.IndexByte(header, '>')
	if gt <= 0 {
		return Frame{}, fmt.Errorf("missing source callsign")
	}
	station := strings.TrimSpace(header[:gt])
	f := Frame{Raw: line, Station: station, ReceivedAt: received, IsWX: hasWX(info)}
	if !f.IsWX {
		f.Reason = "not a weather frame"
		return f, nil
	}
	position := positionRE.FindStringSubmatch(info)
	if len(position) == 0 {
		f.Reason = "weather frame has no supported position"
		return f, nil
	}
	lat, e1 := coordinate(position[1], position[2], 2)
	lon, e2 := coordinate(position[3], position[4], 3)
	if e1 != nil || e2 != nil {
		f.Reason = "invalid position"
		return f, nil
	}
	f.Latitude, f.Longitude = lat, lon
	if time.Since(received) > 2*time.Hour || received.After(time.Now().Add(5*time.Minute)) {
		f.Reason = "stale/future frame"
		return f, nil
	}
	distance := DistanceKM(centerLat, centerLon, lat, lon)
	if distance > radiusKM {
		f.Reason = fmt.Sprintf("outside %.1f km radius (%.1f km)", radiusKM, distance)
		return f, nil
	}
	f.Data = parseWX(info, station, received)
	if len(f.Data) == 0 {
		f.Reason = "weather frame has no recognized weather fields"
		return f, nil
	}
	f.Eligible = true
	f.Reason = "eligible nearby WX station"
	return f, nil
}
func hasWX(s string) bool {
	return weatherFieldRE.MatchString(s)
}
func coordinate(s, hemisphere string, degreeDigits int) (float64, error) {
	n, e := strconv.ParseFloat(s, 64)
	if e != nil {
		return 0, e
	}
	deg := math.Floor(n / 100)
	minutes := n - deg*100
	if minutes >= 60 {
		return 0, fmt.Errorf("minutes out of range")
	}
	v := deg + minutes/60
	if hemisphere == "S" || hemisphere == "W" {
		v = -v
	}
	if (degreeDigits == 2 && math.Abs(v) > 90) || (degreeDigits == 3 && math.Abs(v) > 180) {
		return 0, fmt.Errorf("coordinate out of range")
	}
	return v, nil
}
func parseWX(s, station string, at time.Time) []weather.Observation {
	var out []weather.Observation
	fields := []struct {
		key             string
		length          int
		parameter, unit string
		scale           float64
	}{{"t", 3, "temperature", "°F", 1}, {"h", 2, "humidity", "%", 1}, {"b", 5, "pressure", "hPa", 0.1}, {"s", 3, "wind_speed", "mph", 1}, {"g", 3, "wind_gust", "mph", 1}, {"r", 3, "rain_1h", "mm", 0.254}, {"p", 3, "rain_24h", "mm", 0.254}, {"P", 3, "rain_midnight", "mm", 0.254}}
	for i := 0; i < len(s); i++ {
		for _, field := range fields {
			if len(field.key) == 0 || s[i] != field.key[0] || i+1+field.length > len(s) {
				continue
			}
			raw := s[i+1 : i+1+field.length]
			if field.key == "h" && raw == "//" {
				continue
			}
			v, e := strconv.ParseFloat(raw, 64)
			if e != nil {
				continue
			}
			if field.key == "h" && v == 0 {
				v = 100
			}
			v *= field.scale
			unit := field.unit
			if field.parameter == "pressure" {
				unit = "hPa"
			}
			if field.parameter == "temperature" {
				unit = "°F"
			}
			if field.parameter == "humidity" {
				unit = "%"
			}
			if strings.HasPrefix(field.parameter, "rain") {
				unit = "mm"
			}
			out = append(out, weather.Observation{Parameter: field.parameter, Value: v, Unit: unit, ObservedAt: at, Source: "aprs", Station: station, Quality: "measured", PressureKind: map[bool]string{true: "station"}[field.parameter == "pressure"]})
			i += field.length
			break
		}
	}
	return out
}
func DistanceKM(lat1, lon1, lat2, lon2 float64) float64 {
	const r = 6371
	dLat := (lat2 - lat1) * math.Pi / 180
	dLon := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * r * math.Asin(math.Sqrt(a))
}

// DecodeKISS converts one complete KISS frame to the TNC payload (AX.25).
func DecodeKISS(frame []byte) ([]byte, bool) {
	if len(frame) < 2 || frame[0] != 0xC0 || frame[len(frame)-1] != 0xC0 {
		return nil, false
	}
	body := make([]byte, 0, len(frame)-2)
	escaped := false
	for _, b := range frame[1 : len(frame)-1] {
		if escaped {
			if b == 0xDC {
				b = 0xC0
			} else if b == 0xDD {
				b = 0xDB
			}
			escaped = false
		} else if b == 0xDB {
			escaped = true
			continue
		}
		body = append(body, b)
	}
	if len(body) < 2 || body[0]&0x0F != 0 {
		return nil, false
	}
	return body[1:], true
}
func AX25ToTNC2(packet []byte) (string, error) {
	if len(packet) < 16 {
		return "", fmt.Errorf("short AX.25 frame")
	}
	var calls []string
	offset := 0
	last := false
	for !last {
		if offset+7 > len(packet) || len(calls) > 10 {
			return "", fmt.Errorf("invalid AX.25 address list")
		}
		a := packet[offset : offset+7]
		call := strings.TrimSpace(string([]byte{a[0] >> 1, a[1] >> 1, a[2] >> 1, a[3] >> 1, a[4] >> 1, a[5] >> 1}))
		ssid := int((a[6] >> 1) & 0x0F)
		if ssid > 0 {
			call += fmt.Sprintf("-%d", ssid)
		}
		calls = append(calls, call)
		last = a[6]&1 == 1
		offset += 7
	}
	if offset+2 > len(packet) {
		return "", fmt.Errorf("missing AX.25 control/PID")
	}
	if packet[offset] != 0x03 || packet[offset+1] != 0xF0 {
		return "", fmt.Errorf("unsupported AX.25 frame")
	}
	offset += 2
	if len(calls) < 2 {
		return "", fmt.Errorf("missing AX.25 source/destination")
	}
	header := calls[1] + ">" + calls[0]
	if len(calls) > 2 {
		header += "," + strings.Join(calls[2:], ",")
	}
	return header + ":" + string(packet[offset:]), nil
}
