package config

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

const passwordIterations = 180000

type Schedule struct {
	Enabled       bool   `json:"enabled"`
	Time          string `json:"time,omitempty"`
	EveryHour     bool   `json:"every_hour,omitempty"`
	OffsetMinutes int    `json:"offset_minutes,omitempty"`
}
type Block struct {
	ID      string   `json:"id"`
	Enabled bool     `json:"enabled"`
	Title   string   `json:"title"`
	Kind    string   `json:"kind"`
	Source  string   `json:"source"`
	Fields  []string `json:"fields"`
	Text    string   `json:"text,omitempty"`
}
type Config struct {
	PasswordHash    string     `json:"password_hash,omitempty"`
	AuthDisabled    bool       `json:"auth_disabled,omitempty"`
	Latitude        float64    `json:"latitude"`
	Longitude       float64    `json:"longitude"`
	AltitudeM       float64    `json:"altitude_m"`
	LocationName    string     `json:"location_name"`
	Units           string     `json:"units"`
	Locale          string     `json:"locale"`
	StationCallsign string     `json:"station_callsign"`
	StationLocator  string     `json:"station_locator"`
	APRSRadiusKM    float64    `json:"aprs_radius_km"`
	APRSHost        string     `json:"aprs_host"`
	APRSPort        int        `json:"aprs_port"`
	AudioInput      string     `json:"audio_input"`
	AudioOutput     string     `json:"audio_output"`
	MQTTBroker      string     `json:"mqtt_broker"`
	MQTTTopic       string     `json:"mqtt_topic"`
	MQTTUsername    string     `json:"mqtt_username,omitempty"`
	MQTTPassword    string     `json:"mqtt_password,omitempty"`
	HAURL           string     `json:"ha_url"`
	HAToken         string     `json:"ha_token,omitempty"`
	SerialPort      string     `json:"serial_port"`
	SerialBaud      int        `json:"serial_baud"`
	BMEEnabled      bool       `json:"bme_enabled"`
	I2CDevice       string     `json:"i2c_device"`
	I2CAddress      int        `json:"i2c_address"`
	TTSBinary       string     `json:"tts_binary"`
	TTSVoice        string     `json:"tts_voice"`
	HistoryDays     int        `json:"history_days"`
	Schedules       []Schedule `json:"schedules"`
	Blocks          []Block    `json:"blocks"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

func Defaults() Config {
	return Config{Latitude: 0, Longitude: 0, LocationName: "", Units: "metric", Locale: "", APRSRadiusKM: 10, APRSPort: 8001, I2CDevice: "/dev/i2c-1", I2CAddress: 0x76,
		MQTTTopic: "ultimatewx/+/weather", SerialBaud: 9600, HistoryDays: 7,
		Schedules: []Schedule{{Time: "07:00"}, {Time: "12:00"}, {Time: "18:00"}},
		Blocks: []Block{{ID: "greeting", Enabled: true, Title: "Powitanie", Kind: "text", Text: "Komunikat pogodowy UltimateWX."},
			{ID: "datetime", Enabled: true, Title: "Data i godzina", Kind: "datetime", Source: "system"},
			{ID: "current", Enabled: true, Title: "Aktualna pogoda", Kind: "weather", Source: "auto", Fields: []string{"temperature", "humidity", "pressure", "wind_speed", "precipitation"}},
			{ID: "forecast4h", Enabled: true, Title: "Prognoza za 4 godziny", Kind: "forecast", Source: "open-meteo", Fields: []string{"temperature", "precipitation_probability", "wind_speed"}}}}
}
func Load(path string) (Config, error) {
	c := Defaults()
	b, e := os.ReadFile(path)
	if errors.Is(e, os.ErrNotExist) {
		return c, nil
	}
	if e != nil {
		return c, e
	}
	if e = json.Unmarshal(b, &c); e != nil {
		return c, fmt.Errorf("read config: %w", e)
	}
	return c, nil
}
func Save(path string, c Config) error {
	if e := Validate(c); e != nil {
		return e
	}
	c.UpdatedAt = time.Now().UTC()
	b, e := json.MarshalIndent(c, "", "  ")
	if e != nil {
		return e
	}
	if e = os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return e
	}
	return os.WriteFile(path, append(b, '\n'), 0600)
}
func Validate(c Config) error {
	if c.Latitude < -90 || c.Latitude > 90 || c.Longitude < -180 || c.Longitude > 180 {
		return errors.New("latitude/longitude out of range")
	}
	if c.APRSRadiusKM < 0 || c.APRSRadiusKM > 1000 {
		return errors.New("APRS radius must be 0..1000 km")
	}
	if c.HistoryDays < 1 || c.HistoryDays > 365 {
		return errors.New("history retention must be 1..365 days")
	}
	if c.I2CAddress != 0x76 && c.I2CAddress != 0x77 {
		return errors.New("BME280 I2C address must be 0x76 or 0x77")
	}
	if c.Units != "metric" && c.Units != "imperial" && c.Units != "aviation" {
		return errors.New("units must be metric, imperial or aviation")
	}
	if len(c.Blocks) > 64 {
		return errors.New("too many announcement blocks")
	}
	for _, s := range c.Schedules {
		if s.EveryHour {
			if s.OffsetMinutes < 0 || s.OffsetMinutes > 59 {
				return errors.New("hourly schedule offset must be 0..59 minutes")
			}
			continue
		}
		if s.Time != "" {
			if _, e := time.Parse("15:04", s.Time); e != nil {
				return fmt.Errorf("invalid schedule time %q", s.Time)
			}
		}
	}
	return nil
}
func HashPassword(p string) (string, error) {
	if utf8.RuneCountInString(p) < 6 {
		return "", errors.New("hasło musi mieć co najmniej 6 znaków")
	}
	salt := make([]byte, 16)
	if _, e := rand.Read(salt); e != nil {
		return "", e
	}
	k := pbkdf2([]byte(p), salt, passwordIterations, 32)
	return fmt.Sprintf("pbkdf2-sha256$%d$%s$%s", passwordIterations, base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(k)), nil
}
func VerifyPassword(encoded, p string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2-sha256" {
		return false
	}
	var n int
	if _, e := fmt.Sscanf(parts[1], "%d", &n); e != nil || n < 10000 || n > 1000000 {
		return false
	}
	s, e1 := base64.RawStdEncoding.DecodeString(parts[2])
	want, e2 := base64.RawStdEncoding.DecodeString(parts[3])
	if e1 != nil || e2 != nil {
		return false
	}
	return hmac.Equal(pbkdf2([]byte(p), s, n, len(want)), want)
}
func pbkdf2(password, salt []byte, iterations, length int) []byte {
	var out []byte
	for block := uint32(1); len(out) < length; block++ {
		m := hmac.New(sha256.New, password)
		m.Write(salt)
		m.Write([]byte{byte(block >> 24), byte(block >> 16), byte(block >> 8), byte(block)})
		u := m.Sum(nil)
		t := append([]byte(nil), u...)
		for i := 1; i < iterations; i++ {
			m = hmac.New(sha256.New, password)
			m.Write(u)
			u = m.Sum(nil)
			for j := range t {
				t[j] ^= u[j]
			}
		}
		out = append(out, t...)
	}
	return out[:length]
}
