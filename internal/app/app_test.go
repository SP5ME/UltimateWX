package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ultimatewx/ultimatewx/internal/aprs"
	"github.com/ultimatewx/ultimatewx/internal/config"
	"github.com/ultimatewx/ultimatewx/internal/store"
	"github.com/ultimatewx/ultimatewx/internal/warnings"
	"github.com/ultimatewx/ultimatewx/internal/weather"
)

type appRoundTripper func(*http.Request) (*http.Response, error)

func (roundTripper appRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return roundTripper(request)
}

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
	body := bytes.NewBufferString(`{"password":"ab1234","setup_token":"` + a.setupToken + `"}`)
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

func TestAPIResponsesAreNotCached(t *testing.T) {
	dir := t.TempDir()
	a, err := New(filepath.Join(dir, "config.json"), filepath.Join(dir, "data"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/api/auth", "/api/status"} {
		rr := httptest.NewRecorder()
		a.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, path, nil))
		if got := rr.Header().Get("Cache-Control"); got != "no-store" {
			t.Errorf("%s Cache-Control = %q, want no-store", path, got)
		}
	}
}

func TestConfigurationBackupPreservesPanelCredentialsAndIntegrationSecrets(t *testing.T) {
	dir := t.TempDir()
	a, err := New(filepath.Join(dir, "config.json"), filepath.Join(dir, "data"))
	if err != nil {
		t.Fatal(err)
	}
	handler := a.Handler()
	setup := httptest.NewRecorder()
	handler.ServeHTTP(setup, httptest.NewRequest(http.MethodPost, "/api/setup", strings.NewReader(`{"password":"ab1234","setup_token":"`+a.setupToken+`"}`)))
	if setup.Code != http.StatusOK {
		t.Fatalf("setup failed: %d %s", setup.Code, setup.Body.String())
	}
	cookie := setup.Result().Cookies()[0]
	current := a.Config()
	current.MQTTBroker = "tcp://localhost:1883"
	current.MQTTUsername = "sensor-user"
	current.MQTTPassword = "mqtt-secret"
	current.HAURL = "http://homeassistant.local:8123"
	current.HAToken = "ha-secret"
	if err := a.setConfig(current); err != nil {
		t.Fatal(err)
	}

	getBackup := httptest.NewRecorder()
	getRequest := httptest.NewRequest(http.MethodGet, "/api/backup", nil)
	getRequest.AddCookie(cookie)
	handler.ServeHTTP(getBackup, getRequest)
	if getBackup.Code != http.StatusOK {
		t.Fatalf("backup export failed: %d %s", getBackup.Code, getBackup.Body.String())
	}
	var exported struct {
		Format  string        `json:"format"`
		Version int           `json:"version"`
		Config  config.Config `json:"config"`
	}
	if err := json.NewDecoder(getBackup.Body).Decode(&exported); err != nil {
		t.Fatal(err)
	}
	if exported.Format != "ultimatewx-config-backup" || exported.Version != 1 {
		t.Fatalf("unexpected backup format/version: %q/%d", exported.Format, exported.Version)
	}
	if exported.Config.PasswordHash != "" || exported.Config.AuthDisabled {
		t.Fatal("backup must not include panel credentials")
	}
	if exported.Config.MQTTPassword != "mqtt-secret" || exported.Config.HAToken != "ha-secret" {
		t.Fatal("backup did not include integration credentials")
	}

	exported.Config.LocationName = "Restored location"
	exported.Config.MQTTPassword = ""
	exported.Config.HAToken = ""
	backupBody, err := json.Marshal(exported)
	if err != nil {
		t.Fatal(err)
	}
	postBackup := httptest.NewRecorder()
	postRequest := httptest.NewRequest(http.MethodPost, "/api/backup", bytes.NewReader(backupBody))
	postRequest.AddCookie(cookie)
	handler.ServeHTTP(postBackup, postRequest)
	if postBackup.Code != http.StatusOK {
		t.Fatalf("backup import failed: %d %s", postBackup.Code, postBackup.Body.String())
	}

	restored := a.Config()
	if restored.LocationName != "Restored location" {
		t.Errorf("restored location = %q, want imported location", restored.LocationName)
	}
	if restored.PasswordHash != current.PasswordHash || restored.AuthDisabled != current.AuthDisabled {
		t.Error("import changed the current panel credentials")
	}
	if restored.MQTTPassword != "mqtt-secret" || restored.HAToken != "ha-secret" {
		t.Error("import with empty secret fields erased existing integration credentials")
	}
}

func TestAudioOutputTestRequiresInstalledVoice(t *testing.T) {
	dir := t.TempDir()
	a, err := New(filepath.Join(dir, "config.json"), filepath.Join(dir, "data"))
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/audio/test-output", strings.NewReader(`{"device":""}`))
	response := httptest.NewRecorder()
	a.testAudioOutput(response, request)
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "download a TTS voice") {
		t.Fatalf("test output without a voice: status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestAudioOutputTestSampleUsesVoiceLanguage(t *testing.T) {
	tests := []struct {
		voice string
		want  string
	}{
		{voice: `C:\voices\pl_PL-darkman-medium.onnx`, want: "To jest test wyjścia audio stacji pogodowej."},
		{voice: "/voices/de_DE-thorsten-medium.onnx", want: "Dies ist ein Test des Audioausgangs der Wetterstation."},
		{voice: "/voices/en_US-lessac-medium.onnx", want: "This is a test of the selected weather station audio output."},
		{voice: "/voices/unknown.onnx", want: "This is a test of the selected weather station audio output."},
	}
	for _, test := range tests {
		if got := audioOutputTestSample(test.voice); got != test.want {
			t.Errorf("audioOutputTestSample(%q) = %q, want %q", test.voice, got, test.want)
		}
	}
}

func TestSpeechReadyRequiresUsableVoiceAndOutput(t *testing.T) {
	tests := []struct {
		name   string
		values [5]bool
		want   bool
	}{
		{name: "ready", values: [5]bool{true, true, true, true, true}, want: true},
		{name: "no voice", values: [5]bool{false, true, true, true, true}},
		{name: "missing engine", values: [5]bool{true, false, true, true, true}},
		{name: "missing model", values: [5]bool{true, true, false, true, true}},
		{name: "missing player", values: [5]bool{true, true, true, false, true}},
		{name: "missing configured output", values: [5]bool{true, true, true, true, false}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := speechReady(test.values[0], test.values[1], test.values[2], test.values[3], test.values[4]); got != test.want {
				t.Fatalf("speechReady() = %t, want %t", got, test.want)
			}
		})
	}
}

func TestObservationQualityCounts(t *testing.T) {
	counts := observationQualityCounts([]weather.Observation{
		{Quality: "measured"},
		{Quality: "measured"},
		{Quality: "modelled"},
		{Quality: "derived"},
		{Quality: ""},
		{Quality: "unexpected"},
	})
	want := map[string]int{"measured": 2, "modelled": 1, "derived": 1, "unknown": 2}
	for quality, count := range want {
		if counts[quality] != count {
			t.Errorf("quality count %q = %d, want %d", quality, counts[quality], count)
		}
	}
}

func TestObservationFreshnessCountsUsePerSourceLimits(t *testing.T) {
	now := time.Date(2026, time.October, 21, 12, 0, 0, 0, time.UTC)
	counts := observationFreshnessCounts([]weather.Observation{
		{Source: "serial:roof", ObservedAt: now.Add(-2 * time.Minute)},
		{Source: "serial:roof", ObservedAt: now.Add(-6 * time.Minute)},
		{Source: "bme280", ObservedAt: now.Add(-2 * time.Minute)},
		{Source: "mqtt:indoors", ObservedAt: now.Add(time.Minute)},
		{Source: "open-meteo", ObservedAt: time.Time{}},
	}, now)
	if counts["fresh"] != 1 || counts["stale"] != 4 {
		t.Fatalf("freshness counts = %#v, want 1 fresh and 4 stale", counts)
	}
}

func TestAnnouncementSourceSelectionUsesFreshFallbacksBeforeAutomaticSource(t *testing.T) {
	now := time.Now().UTC()
	observations := map[string]weather.Observation{
		"temperature|mqtt:roof": {
			Parameter: "temperature", Value: 20, ObservedAt: now.Add(-20 * time.Minute), Source: "mqtt:roof", DeviceID: "roof", Quality: "measured",
		},
		"temperature|aprs": {
			Parameter: "temperature", Value: 12, ObservedAt: now.Add(-3 * time.Minute), Source: "aprs", Quality: "measured",
		},
		"temperature|home-assistant:living-room": {
			Parameter: "temperature", Value: 21, ObservedAt: now.Add(-time.Minute), Source: "home-assistant:living-room", DeviceID: "living-room", Quality: "measured",
		},
	}

	got, ok := selectBlockObservationWithDevicePreference(observations, nil, nil, "temperature", "mqtt", []string{"aprs"})
	if !ok || got.Source != "aprs" {
		t.Fatalf("explicit fallback selection = (%q, %t), want fresh APRS fallback", got.Source, ok)
	}
	got, ok = selectBlockObservationWithDevicePreference(observations, nil, nil, "temperature", "mqtt", nil)
	if !ok || got.Source != "home-assistant:living-room" {
		t.Fatalf("automatic fallback selection = (%q, %t), want newest fresh source", got.Source, ok)
	}
}

func TestStatsEndpointReturnsFilteredQualityAndFreshnessCounts(t *testing.T) {
	dir := t.TempDir()
	a, err := New(filepath.Join(dir, "config.json"), filepath.Join(dir, "data"))
	if err != nil {
		t.Fatal(err)
	}
	c := a.Config()
	c.AuthDisabled = true
	c.APRSBlacklist = []string{"BAD1"}
	if err := a.setConfig(c); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	for _, observation := range []weather.Observation{
		{Parameter: "temperature", Value: 18, Unit: "°C", ObservedAt: now.Add(-time.Minute), Source: "mqtt:roof", DeviceID: "roof", Quality: "measured"},
		{Parameter: "humidity", Value: 70, Unit: "%", ObservedAt: now.Add(-6 * time.Minute), Source: "serial:indoor", DeviceID: "indoor", Quality: "derived"},
		{Parameter: "temperature", Value: 24, Unit: "°C", ObservedAt: now.Add(-time.Minute), Source: "aprs", Station: "BAD1", Quality: "measured"},
	} {
		if err := a.history.AddObservation(observation); err != nil {
			t.Fatal(err)
		}
	}

	response := httptest.NewRecorder()
	a.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/stats", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("stats status = %d: %s", response.Code, response.Body.String())
	}
	var result struct {
		QualityCounts   map[string]int        `json:"quality_counts"`
		FreshnessCounts map[string]int        `json:"freshness_counts"`
		History         []weather.Observation `json:"history"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.QualityCounts["measured"] != 1 || result.QualityCounts["derived"] != 1 || result.QualityCounts["modelled"] != 0 {
		t.Errorf("quality counts = %#v, want measured=1, derived=1, modelled=0", result.QualityCounts)
	}
	if result.FreshnessCounts["fresh"] != 1 || result.FreshnessCounts["stale"] != 1 {
		t.Errorf("freshness counts = %#v, want fresh=1, stale=1", result.FreshnessCounts)
	}
	for _, observation := range result.History {
		if strings.EqualFold(observation.Source, "aprs") && observation.Station == "BAD1" {
			t.Error("blacklisted APRS observation leaked into stats history")
		}
	}
}

func TestUpdateCannotRunConcurrently(t *testing.T) {
	a := &App{}
	start := make(chan struct{})
	results := make(chan bool, 32)
	var workers sync.WaitGroup
	for range 32 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			results <- a.beginUpdate()
		}()
	}
	close(start)
	workers.Wait()
	close(results)
	acquired := 0
	for result := range results {
		if result {
			acquired++
		}
	}
	if acquired != 1 {
		t.Fatalf("concurrent updates acquired the slot %d times, want once", acquired)
	}
	a.finishUpdate()
	if !a.beginUpdate() {
		t.Fatal("update slot should be available after a failed update is released")
	}
	a.finishUpdate()
}

func TestSerialConnectionCheckRequiresADeviceUnderDev(t *testing.T) {
	dir := t.TempDir()
	a, err := New(filepath.Join(dir, "config.json"), filepath.Join(dir, "data"))
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/serial/test", strings.NewReader(`{"port":"relative-device"}`))
	response := httptest.NewRecorder()
	a.testSerialConnection(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid serial path status = %d, want 400: %s", response.Code, response.Body.String())
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

func TestWarningAnnouncementDueRespectsRepeatAndRetryIntervals(t *testing.T) {
	now := time.Date(2026, time.October, 21, 12, 0, 0, 0, time.UTC)
	interval := time.Hour
	const signature = "current-warning-version"
	tests := []struct {
		name     string
		previous store.WarningAnnouncement
		exists   bool
		want     bool
	}{
		{name: "new warning is announced immediately", want: true},
		{name: "same warning waits for repeat interval", exists: true, previous: store.WarningAnnouncement{Signature: signature, At: now.Add(-30 * time.Minute)}, want: false},
		{name: "same warning repeats after interval", exists: true, previous: store.WarningAnnouncement{Signature: signature, At: now.Add(-time.Hour)}, want: true},
		{name: "updated warning is announced immediately", exists: true, previous: store.WarningAnnouncement{Signature: "previous-version", At: now.Add(-time.Minute)}, want: true},
		{name: "failed attempt waits five minutes", exists: true, previous: store.WarningAnnouncement{LastAttemptSignature: signature, LastAttemptAt: now.Add(-2 * time.Minute)}, want: false},
		{name: "failed attempt retries after five minutes", exists: true, previous: store.WarningAnnouncement{LastAttemptSignature: signature, LastAttemptAt: now.Add(-6 * time.Minute)}, want: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := warningAnnouncementDue(now, interval, signature, test.previous, test.exists); got != test.want {
				t.Errorf("warningAnnouncementDue() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestWarningRepeatIntervalUsesConfiguredSeverity(t *testing.T) {
	c := config.Config{WarningRepeat1: 60, WarningRepeat2: 30, WarningRepeat3: 15}
	for _, test := range []struct {
		severity int
		want     time.Duration
	}{{0, 60 * time.Minute}, {1, 60 * time.Minute}, {2, 30 * time.Minute}, {3, 15 * time.Minute}, {4, 15 * time.Minute}} {
		if got := warningRepeatInterval(c, test.severity); got != test.want {
			t.Errorf("severity %d interval = %s, want %s", test.severity, got, test.want)
		}
	}
}

func TestWarningRefreshKeepsMeteoAlarmWhenLocalAdapterDoesNotCoverStation(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	c := config.Defaults()
	c.Latitude = 48.0
	c.Longitude = 16.0
	if err := config.Save(configPath, c); err != nil {
		t.Fatal(err)
	}
	a, err := New(configPath, filepath.Join(dir, "data"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	globalAlert := warnings.Alert{
		ID:        "meteoalarm:active",
		Event:     "Heavy rain",
		Severity:  2,
		ValidFrom: now.Add(-time.Minute),
		ValidTo:   now.Add(time.Hour),
		Source:    warnings.MeteoAlarmSource,
	}
	if err := a.history.ReplaceWarnings([]warnings.Alert{globalAlert}); err != nil {
		t.Fatal(err)
	}

	previousTransport := http.DefaultTransport
	http.DefaultTransport = appRoundTripper(func(request *http.Request) (*http.Response, error) {
		body := io.NopCloser(strings.NewReader(`{"type":"Error","msg":"Could not find municipal"}`))
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: body, Request: request}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = previousTransport })

	if err := a.fetchWarnings(context.Background()); err != nil {
		t.Fatalf("warning refresh outside local adapter coverage: %v", err)
	}
	stored := a.history.Warnings()
	if len(stored) != 1 || stored[0].ID != globalAlert.ID {
		t.Fatalf("global MeteoAlarm alert was not preserved: %+v", stored)
	}
}

func TestWarningSpeechAndSignaturePreserveExplicitZeroProbability(t *testing.T) {
	zero := 0
	withProbability := warnings.Alert{ID: "alert-1", Source: "IMGW-PIB", Event: "Test", Severity: 1, Probability: &zero}
	withoutProbability := withProbability
	withoutProbability.Probability = nil

	speech := warningSpeech(withProbability, "pl", "Europe/Warsaw")
	if !strings.Contains(speech, "Prawdopodobieństwo: 0 procent.") {
		t.Fatalf("speech does not include explicit zero probability: %q", speech)
	}
	if strings.Contains(warningSpeech(withoutProbability, "pl", "Europe/Warsaw"), "Prawdopodobieństwo") {
		t.Fatal("speech should omit unavailable probability")
	}
	if warningSignature(withProbability) == warningSignature(withoutProbability) {
		t.Fatal("warning signature should distinguish a reported zero from unavailable probability")
	}
}

func TestWarningSignatureTracksSpokenDescriptionLanguage(t *testing.T) {
	alert := warnings.Alert{ID: "alert-1", Source: "IMGW-PIB", Event: "Wind", Description: "Silny wiatr", ContentLanguage: "en"}
	english := warningSignature(alert)
	alert.ContentLanguage = "pl-PL"
	polish := warningSignature(alert)
	if english == polish {
		t.Fatal("warning signature should change when the description language changes")
	}
	if !strings.Contains(warningSpeech(alert, "pl", "Europe/Warsaw"), "Silny wiatr") {
		t.Fatal("description in the configured voice language should be spoken")
	}
	alert.ContentLanguage = "pl"
	if warningSignature(alert) != polish {
		t.Fatal("regional language tags should not trigger a duplicate warning announcement")
	}
}

func TestWarningSpeechUsesConfiguredStationTimezone(t *testing.T) {
	alert := warnings.Alert{
		Event: "Wind", Severity: 2,
		ValidFrom: time.Date(2026, time.July, 1, 12, 0, 0, 0, time.UTC),
		ValidTo:   time.Date(2026, time.July, 1, 13, 0, 0, 0, time.UTC),
	}
	london := warningSpeech(alert, "en", "Europe/London")
	warsaw := warningSpeech(alert, "en", "Europe/Warsaw")
	if !strings.Contains(london, "01.07.2026 13:00 – 01.07.2026 14:00") {
		t.Fatalf("London warning times were not localized: %q", london)
	}
	if !strings.Contains(warsaw, "01.07.2026 14:00 – 01.07.2026 15:00") {
		t.Fatalf("Warsaw warning times were not localized: %q", warsaw)
	}
}

func TestEstimateUsesOnlyFreshMatchingMeasuredStationPressure(t *testing.T) {
	now := time.Now().UTC()
	observation := func(source, device string, age time.Duration, pressure float64, quality, kind string) weather.Observation {
		return weather.Observation{
			Parameter: "pressure", Value: pressure, Unit: "hPa", ObservedAt: now.Add(-age),
			Source: source, Station: "SP1ABC", DeviceID: device, Quality: quality, PressureKind: kind,
		}
	}
	valid := []weather.Observation{
		observation("mqtt", "sensor-1", 2*time.Hour, 1002, "measured", "station"),
		observation("mqtt", "sensor-1", 0, 1004, "measured", "station"),
	}
	result := estimate(valid)
	if result["available"] != true {
		t.Fatalf("estimate should be available for matching fresh samples: %#v", result)
	}
	if got := result["based_on"]; got != 2 {
		t.Fatalf("based_on = %#v, want 2", got)
	}
	if got := result["pressure_change_hpa_3h"]; got != 3.0 {
		t.Fatalf("three-hour pressure change = %#v, want 3", got)
	}

	tests := []struct {
		name string
		obs  []weather.Observation
	}{
		{
			name: "mixing devices is insufficient",
			obs: []weather.Observation{
				observation("mqtt", "sensor-1", 2*time.Hour, 1002, "measured", "station"),
				observation("mqtt", "sensor-2", 0, 1004, "measured", "station"),
			},
		},
		{
			name: "samples less than an hour apart are insufficient",
			obs: []weather.Observation{
				observation("mqtt", "sensor-1", 30*time.Minute, 1002, "measured", "station"),
				observation("mqtt", "sensor-1", 0, 1004, "measured", "station"),
			},
		},
		{
			name: "stale sample is ignored",
			obs: []weather.Observation{
				observation("mqtt", "sensor-1", 6*time.Hour, 1002, "measured", "station"),
				observation("mqtt", "sensor-1", 0, 1004, "measured", "station"),
			},
		},
		{
			name: "modelled and sea-level pressure are ignored",
			obs: []weather.Observation{
				observation("mqtt", "sensor-1", 2*time.Hour, 1002, "modelled", "station"),
				observation("mqtt", "sensor-1", time.Hour, 1003, "measured", "sea_level"),
				observation("mqtt", "sensor-1", 0, 1004, "measured", "station"),
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := estimate(test.obs)["available"]; got != false {
				t.Fatalf("estimate availability = %#v, want false", got)
			}
		})
	}
}

func TestSaveSensorObservationDerivesSeaLevelPressureOnlyFromStationPressure(t *testing.T) {
	dir := t.TempDir()
	a, err := New(filepath.Join(dir, "config.json"), filepath.Join(dir, "data"))
	if err != nil {
		t.Fatal(err)
	}
	altitude := 1000.0
	c := a.Config()
	c.MQTTSensorAltitudeM = &altitude
	if err := a.setConfig(c); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := a.saveSensorObservation(weather.Observation{
		Parameter: "pressure", Value: 900, Unit: "hPa", ObservedAt: now,
		Source: "mqtt:roof", DeviceID: "roof", Quality: "measured", PressureKind: "station",
	}); err != nil {
		t.Fatal(err)
	}
	if err := a.saveSensorObservation(weather.Observation{
		Parameter: "pressure", Value: 1013, Unit: "hPa", ObservedAt: now,
		Source: "mqtt:sea-level", DeviceID: "sea-level", Quality: "measured", PressureKind: "sea_level",
	}); err != nil {
		t.Fatal(err)
	}

	observations := a.history.Observations(1)
	pressures := make(map[string]weather.Observation, len(observations))
	for _, observation := range observations {
		pressures[observation.Source+"|"+observation.Parameter] = observation
	}
	station := pressures["mqtt:roof|pressure"]
	derived, ok := pressures["mqtt:roof|pressure_msl"]
	if station.Value != 900 || station.PressureKind != "station" || !ok {
		t.Fatalf("station pressure and derived sea-level pressure were not both stored: %#v", pressures)
	}
	wantMSL := 900 / math.Pow(1-altitude/44330, 5.255)
	if math.Abs(derived.Value-wantMSL) > 0.01 || derived.PressureKind != "sea_level" || derived.Quality != "derived" || derived.SensorAltitudeM != nil {
		t.Errorf("derived sea-level reading = %+v, want %.2f hPa, derived, without sensor altitude", derived, wantMSL)
	}
	seaLevel := pressures["mqtt:sea-level|pressure"]
	if seaLevel.Value != 1013 || seaLevel.PressureKind != "sea_level" {
		t.Errorf("already reduced sea-level pressure was changed: %+v", seaLevel)
	}
	if _, exists := pressures["mqtt:sea-level|pressure_msl"]; exists {
		t.Error("already reduced sea-level pressure was corrected a second time")
	}
}

func TestAPRSFrameBlacklistEndpointPersistsAndRemovesFingerprint(t *testing.T) {
	dir := t.TempDir()
	a, err := New(filepath.Join(dir, "config.json"), filepath.Join(dir, "data"))
	if err != nil {
		t.Fatal(err)
	}
	handler := a.Handler()
	setup := httptest.NewRecorder()
	handler.ServeHTTP(setup, httptest.NewRequest(http.MethodPost, "/api/setup", bytes.NewBufferString(`{"password":"ab1234","setup_token":"`+a.setupToken+`"}`)))
	if setup.Code != http.StatusOK {
		t.Fatalf("setup failed: %d %s", setup.Code, setup.Body.String())
	}
	cookie := setup.Result().Cookies()[0]
	fingerprint := aprs.FrameFingerprint("WX1ABC>APRS:!5230.00N/02100.00E_t020")
	postBlacklist := func(frameID string, blocked bool) *httptest.ResponseRecorder {
		payload := `{"frame_id":"` + frameID + `","blocked":` + map[bool]string{true: "true", false: "false"}[blocked] + `}`
		req := httptest.NewRequest(http.MethodPost, "/api/aprs/frame-blacklist", strings.NewReader(payload))
		req.AddCookie(cookie)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)
		return recorder
	}
	if response := postBlacklist(fingerprint, true); response.Code != http.StatusOK {
		t.Fatalf("blocking frame failed: %d %s", response.Code, response.Body.String())
	}
	readConfig := func() config.Config {
		req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
		req.AddCookie(cookie)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)
		if recorder.Code != http.StatusOK {
			t.Fatalf("read config failed: %d %s", recorder.Code, recorder.Body.String())
		}
		var current config.Config
		if err := json.NewDecoder(recorder.Body).Decode(&current); err != nil {
			t.Fatal(err)
		}
		return current
	}
	if got := readConfig().APRSFrameBlacklist; len(got) != 1 || got[0] != fingerprint {
		t.Fatalf("frame blacklist was not persisted: %v", got)
	}
	if response := postBlacklist(fingerprint, false); response.Code != http.StatusOK {
		t.Fatalf("unblocking frame failed: %d %s", response.Code, response.Body.String())
	}
	if got := readConfig().APRSFrameBlacklist; len(got) != 0 {
		t.Fatalf("frame blacklist was not cleared: %v", got)
	}
	if response := postBlacklist("A"+strings.Repeat("0", 63), true); response.Code != http.StatusBadRequest {
		t.Fatalf("uppercase fingerprint status = %d, want 400", response.Code)
	}
}

func TestExternalLocalesListsAndServesOnlyValidLocaleFiles(t *testing.T) {
	dir := t.TempDir()
	dataDir := filepath.Join(dir, "data")
	localeDir := filepath.Join(dataDir, "locales")
	if err := os.MkdirAll(localeDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(localeDir, "nl.json"), []byte(`{"label":"Nederlands","translations":{"status":"Status"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(localeDir, "broken.json"), []byte(`{"label":"Broken","translations":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	a, err := New(filepath.Join(dir, "config.json"), dataDir)
	if err != nil {
		t.Fatal(err)
	}
	handler := a.Handler()
	list := httptest.NewRecorder()
	handler.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/api/locales", nil))
	if list.Code != http.StatusOK {
		t.Fatalf("locale list status = %d: %s", list.Code, list.Body.String())
	}
	var listed struct {
		Locales []availableLocale `json:"locales"`
	}
	if err := json.NewDecoder(list.Body).Decode(&listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Locales) != 1 || listed.Locales[0].Code != "nl" || listed.Locales[0].Label != "Nederlands" {
		t.Fatalf("invalid locale was listed or valid locale missing: %+v", listed.Locales)
	}
	read := httptest.NewRecorder()
	handler.ServeHTTP(read, httptest.NewRequest(http.MethodGet, "/api/locales?code=nl", nil))
	if read.Code != http.StatusOK {
		t.Fatalf("locale read status = %d: %s", read.Code, read.Body.String())
	}
	var result struct {
		Code         string            `json:"code"`
		Translations map[string]string `json:"translations"`
	}
	if err := json.NewDecoder(read.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.Code != "nl" || result.Translations["status"] != "Status" {
		t.Fatalf("unexpected locale response: %+v", result)
	}
	invalid := httptest.NewRecorder()
	handler.ServeHTTP(invalid, httptest.NewRequest(http.MethodGet, "/api/locales?code=../config", nil))
	if invalid.Code != http.StatusNotFound {
		t.Fatalf("invalid locale code status = %d, want 404", invalid.Code)
	}
}
