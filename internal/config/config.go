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
const DefaultTimezone = "Europe/Warsaw"

type Schedule struct {
	Enabled       bool   `json:"enabled"`
	Time          string `json:"time,omitempty"`
	EveryHour     bool   `json:"every_hour,omitempty"`
	OffsetMinutes int    `json:"offset_minutes,omitempty"`
}
type Block struct {
	ID              string   `json:"id"`
	Enabled         bool     `json:"enabled"`
	Title           string   `json:"title"`
	Kind            string   `json:"kind"`
	Source          string   `json:"source"`
	FallbackSources []string `json:"fallback_sources,omitempty"`
	Units           string   `json:"units,omitempty"`
	Format          string   `json:"format,omitempty"`
	Fields          []string `json:"fields"`
	Text            string   `json:"text,omitempty"`
}
type SensorDeviceConfig struct {
	Name                string   `json:"name,omitempty"`
	AltitudeM           *float64 `json:"altitude_m,omitempty"`
	PressureKind        string   `json:"pressure_kind,omitempty"`
	ExcludedParameters  []string `json:"excluded_parameters,omitempty"`
	PreferredParameters []string `json:"preferred_parameters,omitempty"`
}

type Config struct {
	PasswordHash          string                        `json:"password_hash,omitempty"`
	AuthDisabled          bool                          `json:"auth_disabled,omitempty"`
	Latitude              float64                       `json:"latitude"`
	Longitude             float64                       `json:"longitude"`
	AltitudeM             float64                       `json:"altitude_m"`
	LocationName          string                        `json:"location_name"`
	Timezone              string                        `json:"timezone"`
	Units                 string                        `json:"units"`
	Locale                string                        `json:"locale"`
	StationCallsign       string                        `json:"station_callsign"`
	StationLocator        string                        `json:"station_locator"`
	APRSRadiusKM          float64                       `json:"aprs_radius_km"`
	APRSBlacklist         []string                      `json:"aprs_blacklist"`
	APRSFrameBlacklist    []string                      `json:"aprs_frame_blacklist,omitempty"`
	WarningsEnabled       bool                          `json:"warnings_enabled"`
	WarningTERYT          []string                      `json:"warning_teryt_codes"`
	WarningRepeat1        int                           `json:"warning_repeat_level_1_minutes"`
	WarningRepeat2        int                           `json:"warning_repeat_level_2_minutes"`
	WarningRepeat3        int                           `json:"warning_repeat_level_3_minutes"`
	WeatherSource         string                        `json:"weather_source"`
	WeatherSourceOrder    map[string][]string           `json:"weather_source_order,omitempty"`
	APRSHost              string                        `json:"aprs_host"`
	APRSPort              int                           `json:"aprs_port"`
	AudioInput            string                        `json:"audio_input"`
	AudioOutput           string                        `json:"audio_output"`
	AudioVolume           int                           `json:"audio_volume"`
	MQTTBroker            string                        `json:"mqtt_broker"`
	MQTTTopic             string                        `json:"mqtt_topic"`
	MQTTSensorAltitudeM   *float64                      `json:"mqtt_sensor_altitude_m,omitempty"`
	MQTTUsername          string                        `json:"mqtt_username,omitempty"`
	MQTTPassword          string                        `json:"mqtt_password,omitempty"`
	HAURL                 string                        `json:"ha_url"`
	HAToken               string                        `json:"ha_token,omitempty"`
	SerialPort            string                        `json:"serial_port"`
	SerialBaud            int                           `json:"serial_baud"`
	SerialSensorAltitudeM *float64                      `json:"serial_sensor_altitude_m,omitempty"`
	SensorDevices         map[string]SensorDeviceConfig `json:"sensor_devices,omitempty"`
	BMEEnabled            bool                          `json:"bme_enabled"`
	I2CDevice             string                        `json:"i2c_device"`
	I2CAddress            int                           `json:"i2c_address"`
	TTSBinary             string                        `json:"tts_binary"`
	TTSVoice              string                        `json:"tts_voice"`
	HistoryDays           int                           `json:"history_days"`
	Schedules             []Schedule                    `json:"schedules"`
	Blocks                []Block                       `json:"blocks"`
	UpdatedAt             time.Time                     `json:"updated_at"`
}

func Defaults() Config {
	return Config{Latitude: 0, Longitude: 0, LocationName: "", Timezone: DefaultTimezone, Units: "metric", Locale: "", APRSRadiusKM: 10, WarningsEnabled: true, WarningRepeat1: 60, WarningRepeat2: 30, WarningRepeat3: 15, WeatherSource: "auto", APRSPort: 8001, I2CDevice: "/dev/i2c-1", I2CAddress: 0x76,
		MQTTTopic: "ultimatewx/+/weather", SerialBaud: 9600, HistoryDays: 7, AudioVolume: 100,
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
	tmp, e := os.CreateTemp(filepath.Dir(path), ".config-*.tmp")
	if e != nil {
		return fmt.Errorf("create temporary config: %w", e)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if e = tmp.Chmod(0600); e == nil {
		_, e = tmp.Write(append(b, '\n'))
	}
	if e == nil {
		e = tmp.Sync()
	}
	closeErr := tmp.Close()
	if e != nil {
		return fmt.Errorf("write temporary config: %w", e)
	}
	if closeErr != nil {
		return fmt.Errorf("close temporary config: %w", closeErr)
	}
	if e = replaceConfigFile(tmpPath, path); e != nil {
		return fmt.Errorf("replace config: %w", e)
	}
	return nil
}
func Validate(c Config) error {
	if len(c.Timezone) > 64 || strings.TrimSpace(c.Timezone) == "" {
		return errors.New("station time zone must be a non-empty IANA name of at most 64 characters")
	}
	if _, err := time.LoadLocation(c.Timezone); err != nil {
		return errors.New("station time zone must be a valid IANA name")
	}
	if len(c.AudioInput) > 256 || strings.ContainsAny(c.AudioInput, "\r\n\x00") {
		return errors.New("audio input device must be at most 256 characters and contain no line breaks")
	}
	if len(c.SensorDevices) > 100 {
		return errors.New("at most 100 sensor devices may be configured")
	}
	validDeviceSources := map[string]bool{"mqtt": true, "serial": true, "home-assistant": true, "bme280": true}
	validSensorParameters := map[string]bool{"temperature": true, "humidity": true, "pressure": true, "wind_speed": true, "wind_direction": true, "precipitation": true}
	preferredDeviceBySourceParameter := make(map[string]string)
	for key, device := range c.SensorDevices {
		parts := strings.SplitN(key, ":", 2)
		if len(parts) != 2 || !validDeviceSources[parts[0]] || strings.TrimSpace(parts[1]) == "" || len(key) > 256 || strings.ContainsAny(key, "\r\n\t") {
			return errors.New("sensor device keys must contain a supported source and device ID")
		}
		if len(device.Name) > 128 || strings.ContainsAny(device.Name, "\r\n") {
			return errors.New("sensor device names must be at most 128 characters and contain no line breaks")
		}
		if device.AltitudeM != nil && (*device.AltitudeM < -500 || *device.AltitudeM > 9000) {
			return errors.New("sensor device altitude must be between -500 and 9000 metres")
		}
		if device.PressureKind != "" && device.PressureKind != "station" && device.PressureKind != "sea_level" {
			return errors.New("sensor pressure kind must be station or sea_level")
		}
		seenExcludedParameters := make(map[string]bool, len(device.ExcludedParameters))
		for _, parameter := range device.ExcludedParameters {
			if !validSensorParameters[parameter] || seenExcludedParameters[parameter] {
				return errors.New("sensor device excluded parameters must be unique supported measurements")
			}
			seenExcludedParameters[parameter] = true
		}
		seenPreferredParameters := make(map[string]bool, len(device.PreferredParameters))
		for _, parameter := range device.PreferredParameters {
			if !validSensorParameters[parameter] || seenPreferredParameters[parameter] || seenExcludedParameters[parameter] {
				return errors.New("sensor device preferred parameters must be unique supported measurements that are enabled")
			}
			seenPreferredParameters[parameter] = true
			preferenceKey := parts[0] + "|" + parameter
			if preferredDeviceBySourceParameter[preferenceKey] != "" {
				return errors.New("only one device per source and parameter may be preferred")
			}
			preferredDeviceBySourceParameter[preferenceKey] = key
		}
	}
	for _, altitude := range []*float64{c.MQTTSensorAltitudeM, c.SerialSensorAltitudeM} {
		if altitude != nil && (*altitude < -500 || *altitude > 9000) {
			return errors.New("sensor altitude must be between -500 and 9000 metres")
		}
	}
	validParameters := map[string]bool{"temperature": true, "humidity": true, "pressure": true, "wind_speed": true, "wind_direction": true, "precipitation": true, "feels_like": true}
	validSources := map[string]bool{"open-meteo": true, "aprs": true, "bme280": true, "mqtt": true, "home-assistant": true, "serial": true}
	for parameter, order := range c.WeatherSourceOrder {
		if !validParameters[parameter] || len(order) > len(validSources) {
			return errors.New("weather source order contains an unsupported parameter or too many sources")
		}
		seen := map[string]bool{}
		for _, source := range order {
			if !validSources[source] || seen[source] {
				return errors.New("weather source order contains an unsupported or duplicate source")
			}
			seen[source] = true
		}
	}
	if c.Latitude < -90 || c.Latitude > 90 || c.Longitude < -180 || c.Longitude > 180 {
		return errors.New("latitude/longitude out of range")
	}
	if c.APRSRadiusKM < 0 || c.APRSRadiusKM > 1000 {
		return errors.New("APRS radius must be 0..1000 km")
	}
	if len(c.APRSBlacklist) > 100 {
		return errors.New("at most 100 APRS callsigns may be blacklisted")
	}
	for _, callsign := range c.APRSBlacklist {
		callsign = strings.TrimSpace(callsign)
		if callsign == "" || len(callsign) > 9 || strings.ContainsAny(callsign, ",; \t\r\n") {
			return errors.New("APRS blacklist entries must be callsigns, separated into individual entries")
		}
	}
	if len(c.APRSFrameBlacklist) > 500 {
		return errors.New("at most 500 APRS frames may be blacklisted")
	}
	seenAPRSFrames := make(map[string]bool, len(c.APRSFrameBlacklist))
	for _, frameID := range c.APRSFrameBlacklist {
		if len(frameID) != 64 || strings.Trim(frameID, "0123456789abcdef") != "" || seenAPRSFrames[frameID] {
			return errors.New("APRS frame blacklist entries must be unique lowercase SHA-256 hashes")
		}
		seenAPRSFrames[frameID] = true
	}
	if len(c.WarningTERYT) > 20 {
		return errors.New("at most 20 warning TERYT codes are allowed")
	}
	for _, code := range c.WarningTERYT {
		if len(code) != 4 {
			return errors.New("warning TERYT codes must contain exactly four digits")
		}
		for _, digit := range code {
			if digit < '0' || digit > '9' {
				return errors.New("warning TERYT codes must contain exactly four digits")
			}
		}
	}
	for _, minutes := range []int{c.WarningRepeat1, c.WarningRepeat2, c.WarningRepeat3} {
		if minutes < 5 || minutes > 1440 {
			return errors.New("warning repeat intervals must be between 5 and 1440 minutes")
		}
	}
	if c.WeatherSource != "" && c.WeatherSource != "auto" && c.WeatherSource != "open-meteo" && c.WeatherSource != "aprs" && c.WeatherSource != "bme280" && c.WeatherSource != "mqtt" && c.WeatherSource != "home-assistant" && c.WeatherSource != "serial" {
		return errors.New("unsupported preferred weather source")
	}
	if c.HistoryDays < 1 || c.HistoryDays > 365 {
		return errors.New("history retention must be 1..365 days")
	}
	if c.I2CAddress != 0x76 && c.I2CAddress != 0x77 {
		return errors.New("BME280 I2C address must be 0x76 or 0x77")
	}
	if c.AudioVolume < 0 || c.AudioVolume > 100 {
		return errors.New("announcement audio volume must be 0..100 percent")
	}
	if c.Units != "metric" && c.Units != "imperial" && c.Units != "aviation" {
		return errors.New("units must be metric, imperial or aviation")
	}
	if len(c.Blocks) > 64 {
		return errors.New("too many announcement blocks")
	}
	for _, block := range c.Blocks {
		if block.Units != "" && block.Units != "metric" && block.Units != "imperial" && block.Units != "aviation" {
			return errors.New("announcement block units must be metric, imperial or aviation")
		}
		if block.Format != "" && block.Format != "standard" && block.Format != "compact" {
			return errors.New("announcement block format must be standard or compact")
		}
		if len(block.FallbackSources) > 5 || len(block.FallbackSources) > 0 && (block.Kind != "weather" || block.Source == "" || block.Source == "auto") {
			return errors.New("fallback sources require a weather block with an explicit primary source and allow at most five entries")
		}
		seen := map[string]bool{}
		for _, source := range block.FallbackSources {
			if !validSources[source] || source == "auto" || seen[source] || source == block.Source {
				return errors.New("announcement block has an unsupported, duplicate, or primary fallback source")
			}
			seen[source] = true
		}
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
			// Reuse the keyed HMAC state and U buffer. Recreating both 180,000
			// times made password verification unnecessarily slow on Raspberry Pi.
			m.Reset()
			m.Write(u)
			u = m.Sum(u[:0])
			for j := range t {
				t[j] ^= u[j]
			}
		}
		out = append(out, t...)
	}
	return out[:length]
}
