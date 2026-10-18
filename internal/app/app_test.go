package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/ultimatewx/ultimatewx/internal/config"
)

func TestFirstRunSetupProtectsAPIAndDoesNotExposePasswordHash(t *testing.T) {
	dir := t.TempDir()
	a, err := New(filepath.Join(dir, "config.json"), filepath.Join(dir, "data"))
	if err != nil {
		t.Fatal(err)
	}
	handler := a.Handler()
	unauth := httptest.NewRecorder()
	handler.ServeHTTP(unauth, httptest.NewRequest(http.MethodGet, "/api/status", nil))
	if unauth.Code != http.StatusForbidden {
		t.Fatalf("status before setup: %d", unauth.Code)
	}
	body := bytes.NewBufferString(`{"password":"ab1234"}`)
	setup := httptest.NewRecorder()
	handler.ServeHTTP(setup, httptest.NewRequest(http.MethodPost, "/api/setup", body))
	if setup.Code != 200 {
		t.Fatalf("setup: %d %s", setup.Code, setup.Body.String())
	}
	cookie := setup.Result().Cookies()[0]
	configResponse := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	req.AddCookie(cookie)
	handler.ServeHTTP(configResponse, req)
	if configResponse.Code != 200 {
		t.Fatalf("config: %d", configResponse.Code)
	}
	var result map[string]any
	if err = json.NewDecoder(configResponse.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if _, ok := result["password_hash"]; ok {
		t.Fatal("password hash leaked through config API")
	}
}

func TestLoginRejectsWrongPassword(t *testing.T) {
	dir := t.TempDir()
	a, err := New(filepath.Join(dir, "config.json"), filepath.Join(dir, "data"))
	if err != nil {
		t.Fatal(err)
	}
	c := a.Config()
	c.PasswordHash, _ = config.HashPassword("long-test-password")
	if err = a.setConfig(c); err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	a.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewBufferString(`{"password":"not-the-password"}`)))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestLoginAcceptsSixCharacterPassword(t *testing.T) {
	dir := t.TempDir()
	a, err := New(filepath.Join(dir, "config.json"), filepath.Join(dir, "data"))
	if err != nil {
		t.Fatal(err)
	}
	c := a.Config()
	c.PasswordHash, err = config.HashPassword("ab1234")
	if err != nil {
		t.Fatal(err)
	}
	if err = a.setConfig(c); err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	a.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewBufferString(`{"password":"ab1234"}`)))
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestRenderStationTemplate(t *testing.T) {
	got := renderStationTemplate("Tu stacja pogodowa <call> znajdująca się w <loc>.", "SP1ABC", "JO91XX")
	want := "Tu stacja pogodowa SP1ABC znajdująca się w JO91XX."
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if got := renderStationTemplate("WX <call> <loc>", "", ""); got != "WX" {
		t.Fatalf("empty station variables should be omitted, got %q", got)
	}
}
