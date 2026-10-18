package config

import (
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
	c.Latitude = 91
	if err := Validate(c); err == nil {
		t.Fatal("expected invalid latitude to be rejected")
	}
}

func TestPasswordHashAndConfigRoundTrip(t *testing.T) {
	if _, err := HashPassword("12345"); err == nil {
		t.Fatal("expected a five-character password to be rejected")
	}
	shortHash, err := HashPassword("123456")
	if err != nil || !VerifyPassword(shortHash, "123456") {
		t.Fatalf("six-character password should be accepted: %v", err)
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
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.PasswordHash != hash || loaded.APRSRadiusKM != 10 || loaded.StationCallsign != c.StationCallsign || loaded.StationLocator != c.StationLocator {
		t.Fatalf("config did not round-trip: %+v", loaded)
	}
}
