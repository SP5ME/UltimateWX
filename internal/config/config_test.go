package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultsAndValidation(t *testing.T) {
	c := Defaults()
	if c.APRSRadiusKM != 10 || c.Units != "metric" || len(c.Blocks) < 3 {
		t.Fatalf("unexpected defaults: %+v", c)
	}
	if err := Validate(c); err != nil {
		t.Fatal(err)
	}
	if c.Timezone != "Europe/Warsaw" {
		t.Fatalf("default station time zone = %q", c.Timezone)
	}
	c.Timezone = "Mars/Olympus"
	if err := Validate(c); err == nil {
		t.Fatal("expected invalid station time zone to be rejected")
	}
	c = Defaults()
	c.Latitude = 91
	if err := Validate(c); err == nil {
		t.Fatal("expected invalid latitude to be rejected")
	}
}

func TestPasswordHashAndConfigRoundTrip(t *testing.T) {
	if _, err := HashPassword(""); err == nil {
		t.Fatal("expected an empty password to be rejected")
	}
	shortHash, err := HashPassword("wx")
	if err != nil || !VerifyPassword(shortHash, "wx") {
		t.Fatalf("short default password should be accepted: %v", err)
	}
	hash, err := HashPassword("long-test-password")
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword(hash, "long-test-password") || VerifyPassword(hash, "wrong-password") {
		t.Fatal("password verification failed")
	}
	c := Defaults()
	c.PasswordHash = hash
	c.StationCallsign = "SP1ABC"
	c.StationLocator = "JO91XX"
	path := filepath.Join(t.TempDir(), "config.json")
	if err = Save(path, c); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("configuration temporary file was not removed: %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.PasswordHash != hash || loaded.APRSRadiusKM != 10 || loaded.StationCallsign != c.StationCallsign || loaded.StationLocator != c.StationLocator {
		t.Fatalf("config did not round-trip: %+v", loaded)
	}
}

func TestLoadLegacyConfigAddsDefaultStationTimezone(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.json")
	if err := os.WriteFile(path, []byte(`{"units":"metric","history_days":7}`), 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Timezone != DefaultTimezone {
		t.Fatalf("legacy configuration time zone = %q, want %q", loaded.Timezone, DefaultTimezone)
	}
}

func TestWeatherSourceOrderValidation(t *testing.T) {
	tests := []struct {
		name  string
		order map[string][]string
		valid bool
	}{
		{
			name:  "all supported sources including wind direction",
			order: map[string][]string{"wind_direction": {"aprs", "mqtt", "home-assistant", "serial", "bme280", "open-meteo"}},
			valid: true,
		},
		{name: "unsupported parameter", order: map[string][]string{"dew_point": {"open-meteo"}}},
		{name: "unsupported source", order: map[string][]string{"temperature": {"unknown"}}},
		{name: "duplicate source", order: map[string][]string{"temperature": {"mqtt", "mqtt"}}},
		{name: "more sources than integrations", order: map[string][]string{"temperature": {"open-meteo", "aprs", "bme280", "mqtt", "home-assistant", "serial", "open-meteo"}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := Defaults()
			c.WeatherSourceOrder = test.order
			err := Validate(c)
			if test.valid && err != nil {
				t.Fatalf("valid source order rejected: %v", err)
			}
			if !test.valid && err == nil {
				t.Fatal("invalid source order accepted")
			}
		})
	}
}

func TestWarningDisabledProvidersValidation(t *testing.T) {
	c := Defaults()
	c.WarningDisabledProviders = []string{"imgw", "meteoalarm"}
	if err := Validate(c); err != nil {
		t.Fatalf("supported disabled warning providers rejected: %v", err)
	}
	c.WarningDisabledProviders = []string{"unknown"}
	if err := Validate(c); err == nil {
		t.Fatal("unsupported warning provider accepted")
	}
	c.WarningDisabledProviders = []string{"dwd", "dwd"}
	if err := Validate(c); err == nil {
		t.Fatal("duplicate disabled warning provider accepted")
	}
}

func TestLegacyConfigEnablesAllWarningProvidersByDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.json")
	if err := os.WriteFile(path, []byte(`{"warnings_enabled":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.WarningDisabledProviders) != 0 {
		t.Fatalf("legacy config unexpectedly disables warning providers: %v", c.WarningDisabledProviders)
	}
}

func TestSensorDevicePreferenceValidation(t *testing.T) {
	tests := []struct {
		name    string
		devices map[string]SensorDeviceConfig
		valid   bool
	}{
		{
			name: "one preferred device per source and parameter",
			devices: map[string]SensorDeviceConfig{
				"mqtt:roof":     {PreferredParameters: []string{"temperature"}},
				"serial:indoor": {PreferredParameters: []string{"humidity"}},
			},
			valid: true,
		},
		{
			name: "conflicting preferred devices",
			devices: map[string]SensorDeviceConfig{
				"mqtt:roof":   {PreferredParameters: []string{"temperature"}},
				"mqtt:indoor": {PreferredParameters: []string{"temperature"}},
			},
		},
		{
			name: "preferred parameter is excluded",
			devices: map[string]SensorDeviceConfig{
				"mqtt:roof": {ExcludedParameters: []string{"temperature"}, PreferredParameters: []string{"temperature"}},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := Defaults()
			c.SensorDevices = test.devices
			err := Validate(c)
			if test.valid && err != nil {
				t.Fatalf("valid device preferences rejected: %v", err)
			}
			if !test.valid && err == nil {
				t.Fatal("invalid device preferences accepted")
			}
		})
	}
}
