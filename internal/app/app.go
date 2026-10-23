package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ultimatewx/ultimatewx/internal/aprs"
	"github.com/ultimatewx/ultimatewx/internal/audio"
	"github.com/ultimatewx/ultimatewx/internal/bme280"
	"github.com/ultimatewx/ultimatewx/internal/config"
	"github.com/ultimatewx/ultimatewx/internal/mqtt"
	"github.com/ultimatewx/ultimatewx/internal/store"
	"github.com/ultimatewx/ultimatewx/internal/tts"
	"github.com/ultimatewx/ultimatewx/internal/updater"
	"github.com/ultimatewx/ultimatewx/internal/warnings"
	"github.com/ultimatewx/ultimatewx/internal/weather"
)

type App struct {
	configPath, dataDir string
	cfg                 config.Config
	cfgMu               sync.RWMutex
	history             *store.Store
	speechMu            sync.Mutex
	warningMu           sync.RWMutex
	warningUpdateMu     sync.Mutex
	warningError        string
	warningUpdatedAt    time.Time
	warningProviders    map[string]warningProviderHealth
	warningCounty       string
	warningCountyLat    float64
	warningCountyLon    float64
	warningCountyAt     time.Time
	warningRefresh      chan struct{}
	setupMu             sync.Mutex
	setupToken          string
	bmeStatusMu         sync.RWMutex
	bmeLastError        string
	sourceHealthMu      sync.RWMutex
	sourceHealth        map[string]sourceHealth
	aprsDiagMu          sync.RWMutex
	aprsDiag            aprsReceiveDiagnostics
	voices              *tts.Service
	snapshot            weather.Snapshot
	snapMu              sync.RWMutex
	sessions            map[string]time.Time
	sessionMu           sync.Mutex
	loginAttemptMu      sync.Mutex
	loginAttempts       map[string]loginAttemptState
	updateMu            sync.Mutex
	updateInProgress    bool
	mux                 *http.ServeMux
	restart             bool
}

const (
	loginAttemptWindow = 15 * time.Minute
	loginAttemptLimit  = 10
	loginAttemptCap    = 1024
)

type loginAttemptState struct {
	WindowStart  time.Time
	Attempts     int
	BlockedUntil time.Time
}

type sourceHealth struct {
	LastAttemptAt time.Time `json:"last_attempt_at,omitempty"`
	LastSuccessAt time.Time `json:"last_success_at,omitempty"`
	Error         string    `json:"error,omitempty"`
}

type aprsReceiveDiagnostics struct {
	Connected           bool      `json:"connected"`
	ConnectedAt         time.Time `json:"connected_at,omitempty"`
	LastDataAt          time.Time `json:"last_data_at,omitempty"`
	BytesReceived       uint64    `json:"bytes_received"`
	KISSFrames          uint64    `json:"kiss_frames"`
	AX25Frames          uint64    `json:"ax25_frames"`
	FramesStored        uint64    `json:"frames_stored"`
	LastProcessingError string    `json:"last_processing_error,omitempty"`
}

type warningProviderHealth struct {
	State      string    `json:"state"`
	CheckedAt  time.Time `json:"checked_at,omitempty"`
	AlertCount int       `json:"alert_count,omitempty"`
	Error      string    `json:"error,omitempty"`
}

func New(configPath, dataDir string) (*App, error) {
	c, e := config.Load(configPath)
	if e != nil {
		return nil, e
	}
	s, e := store.Open(dataDir)
	if e != nil {
		return nil, e
	}
	setupToken, e := ensureSetupToken(c, filepath.Join(dataDir, "setup-token"))
	if e != nil {
		return nil, e
	}
	a := &App{configPath: configPath, dataDir: dataDir, cfg: c, history: s, voices: tts.New(filepath.Join(dataDir, "voices")), sessions: map[string]time.Time{}, loginAttempts: map[string]loginAttemptState{}, mux: http.NewServeMux(), warningRefresh: make(chan struct{}, 1), setupToken: setupToken, sourceHealth: map[string]sourceHealth{}, warningProviders: newWarningProviderHealth("waiting", time.Time{})}
	a.routes()
	return a, nil
}

func ensureSetupToken(c config.Config, path string) (string, error) {
	if c.PasswordHash != "" || c.AuthDisabled {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Printf("remove obsolete first-run setup token: %v", err)
		}
		return "", nil
	}
	if data, err := os.ReadFile(path); err == nil {
		token := strings.TrimSpace(string(data))
		if len(token) < 32 {
			return "", errors.New("first-run setup token file is invalid; remove it to generate a new token")
		}
		if err := os.Chmod(path, 0600); err != nil {
			return "", fmt.Errorf("secure first-run setup token: %w", err)
		}
		return token, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("read first-run setup token: %w", err)
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return "", fmt.Errorf("generate first-run setup token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(secret)
	if err := os.WriteFile(path, []byte(token+"\n"), 0600); err != nil {
		return "", fmt.Errorf("write first-run setup token: %w", err)
	}
	if err := os.Chmod(path, 0600); err != nil {
		return "", fmt.Errorf("secure first-run setup token: %w", err)
	}
	log.Printf("First-run setup token is stored in %s", path)
	return token, nil
}

func (a *App) Config() config.Config { a.cfgMu.RLock(); defer a.cfgMu.RUnlock(); return a.cfg }
func (a *App) setConfig(c config.Config) error {
	if strings.TrimSpace(c.Timezone) == "" {
		c.Timezone = config.DefaultTimezone
	}
	if e := config.Validate(c); e != nil {
		return e
	}
	a.cfgMu.Lock()
	previous := a.cfg
	locationChanged := previous.Latitude != c.Latitude || previous.Longitude != c.Longitude
	var previousWarnings []warnings.Alert
	if locationChanged {
		a.warningUpdateMu.Lock()
		previousWarnings = a.history.Warnings()
		if e := a.history.ReplaceWarnings(nil); e != nil {
			a.warningUpdateMu.Unlock()
			a.cfgMu.Unlock()
			return fmt.Errorf("clear warnings for previous station location: %w", e)
		}
	}
	if e := config.Save(a.configPath, c); e != nil {
		var restoreErr error
		if locationChanged {
			restoreErr = a.history.ReplaceWarnings(previousWarnings)
			a.warningUpdateMu.Unlock()
		}
		a.cfgMu.Unlock()
		if locationChanged {
			a.requestWarningRefresh()
		}
		if restoreErr != nil {
			return fmt.Errorf("save configuration: %v; restore previous station warnings: %w", e, restoreErr)
		}
		return e
	}
	a.cfg = c
	if locationChanged {
		a.warningUpdateMu.Unlock()
	}
	a.cfgMu.Unlock()
	if previous.WarningsEnabled != c.WarningsEnabled || previous.Latitude != c.Latitude || previous.Longitude != c.Longitude || previous.Locale != c.Locale || strings.Join(previous.WarningTERYT, ",") != strings.Join(c.WarningTERYT, ",") {
		a.warningMu.Lock()
		if previous.Latitude != c.Latitude || previous.Longitude != c.Longitude || strings.Join(previous.WarningTERYT, ",") != strings.Join(c.WarningTERYT, ",") {
			a.warningCounty = ""
			a.warningCountyAt = time.Time{}
		}
		a.warningUpdatedAt = time.Time{}
		a.warningError = ""
		providerState := "waiting"
		if !c.WarningsEnabled {
			providerState = "disabled"
		}
		a.warningProviders = newWarningProviderHealth(providerState, time.Time{})
		a.warningMu.Unlock()
		a.requestWarningRefresh()
	}
	return nil
}

func (a *App) requestWarningRefresh() {
	select {
	case a.warningRefresh <- struct{}{}:
	default:
	}
}

func (a *App) warningConfigCurrent(c config.Config) bool {
	a.cfgMu.RLock()
	defer a.cfgMu.RUnlock()
	return sameWarningConfig(a.cfg, c)
}

func (a *App) replaceWarningsForConfig(c config.Config, items []warnings.Alert) (bool, error) {
	a.cfgMu.RLock()
	defer a.cfgMu.RUnlock()
	if !sameWarningConfig(a.cfg, c) {
		return false, nil
	}
	a.warningUpdateMu.Lock()
	defer a.warningUpdateMu.Unlock()
	return true, a.history.ReplaceWarnings(items)
}

func (a *App) reconcileWarningsForConfig(c config.Config, items []warnings.Alert) (bool, error) {
	a.cfgMu.RLock()
	defer a.cfgMu.RUnlock()
	if !sameWarningConfig(a.cfg, c) {
		return false, nil
	}
	a.warningUpdateMu.Lock()
	defer a.warningUpdateMu.Unlock()
	return true, a.history.ReconcileWarnings(items)
}

func (a *App) reconcilePolledWarningsForConfig(c config.Config, items []warnings.Alert) (bool, error) {
	a.cfgMu.RLock()
	defer a.cfgMu.RUnlock()
	if !sameWarningConfig(a.cfg, c) {
		return false, nil
	}
	a.warningUpdateMu.Lock()
	defer a.warningUpdateMu.Unlock()
	now := time.Now().UTC()
	for _, item := range a.history.Warnings() {
		if item.Source == warnings.MeteoAlarmSource && item.ValidTo.After(now) {
			items = append(items, item)
		}
	}
	return true, a.history.ReconcileWarnings(items)
}

func (a *App) applyMeteoAlarmResult(c config.Config, result warnings.MeteoAlarmResult) (bool, error) {
	a.cfgMu.RLock()
	defer a.cfgMu.RUnlock()
	if !sameWarningConfig(a.cfg, c) || !c.WarningsEnabled {
		return false, nil
	}
	a.warningUpdateMu.Lock()
	defer a.warningUpdateMu.Unlock()
	remove := make(map[string]bool, len(result.RemoveIDs)+1)
	for _, id := range result.RemoveIDs {
		remove[strings.TrimSpace(id)] = true
	}
	if result.AlertID != "" {
		remove[result.AlertID] = true
	}
	now := time.Now().UTC()
	previous := a.history.Warnings()
	merged := make([]warnings.Alert, 0, len(previous)+1)
	for _, item := range previous {
		if item.Source == warnings.MeteoAlarmSource && remove[item.ID] {
			continue
		}
		if item.ValidTo.After(now) {
			merged = append(merged, item)
		}
	}
	if result.Alert != nil && result.Alert.ValidTo.After(now) {
		merged = append(merged, *result.Alert)
	}
	return true, a.history.ReconcileWarnings(merged)
}

func (a *App) setMeteoAlarmHealth(err error) {
	a.warningMu.Lock()
	defer a.warningMu.Unlock()
	if a.warningProviders == nil {
		a.warningProviders = newWarningProviderHealth("waiting", time.Time{})
	}
	provider := warningProviderHealth{CheckedAt: time.Now().UTC()}
	if err != nil {
		provider.State = "error"
		provider.Error = err.Error()
	} else {
		provider.State = "ok"
		for _, item := range a.history.Warnings() {
			if item.Source == warnings.MeteoAlarmSource && item.ValidTo.After(provider.CheckedAt) {
				provider.AlertCount++
			}
		}
	}
	a.warningProviders[warnings.MeteoAlarmSource] = provider
}

func (a *App) setMeteoAlarmConnected() {
	a.warningMu.Lock()
	defer a.warningMu.Unlock()
	if a.warningProviders == nil {
		a.warningProviders = newWarningProviderHealth("waiting", time.Time{})
	}
	provider := warningProviderHealth{State: "connected", CheckedAt: time.Now().UTC()}
	for _, item := range a.history.Warnings() {
		if item.Source == warnings.MeteoAlarmSource && item.ValidTo.After(provider.CheckedAt) {
			provider.AlertCount++
		}
	}
	a.warningProviders[warnings.MeteoAlarmSource] = provider
}

func (a *App) setWarningHealthForConfig(c config.Config, updated time.Time, message string) {
	a.cfgMu.RLock()
	defer a.cfgMu.RUnlock()
	if sameWarningConfig(a.cfg, c) {
		a.setWarningHealth(updated, message)
	}
}

func (a *App) setWarningProviderHealthForConfig(c config.Config, providers map[string]warningProviderHealth) {
	a.cfgMu.RLock()
	defer a.cfgMu.RUnlock()
	if !sameWarningConfig(a.cfg, c) {
		return
	}
	a.warningMu.Lock()
	if current, ok := a.warningProviders[warnings.MeteoAlarmSource]; ok && current.CheckedAt.After(providers[warnings.MeteoAlarmSource].CheckedAt) {
		providers[warnings.MeteoAlarmSource] = current
	}
	a.warningProviders = cloneWarningProviderHealth(providers)
	a.warningMu.Unlock()
}

func (a *App) warningProviderHealthSnapshot() map[string]warningProviderHealth {
	a.warningMu.RLock()
	defer a.warningMu.RUnlock()
	return cloneWarningProviderHealth(a.warningProviders)
}

func newWarningProviderHealth(state string, checkedAt time.Time) map[string]warningProviderHealth {
	providers := make(map[string]warningProviderHealth, 4)
	for _, name := range []string{"IMGW-PIB", "DWD", "GeoSphere Austria", warnings.MeteoAlarmSource} {
		providers[name] = warningProviderHealth{State: state, CheckedAt: checkedAt}
	}
	return providers
}

func cloneWarningProviderHealth(providers map[string]warningProviderHealth) map[string]warningProviderHealth {
	clone := make(map[string]warningProviderHealth, len(providers))
	for name, status := range providers {
		clone[name] = status
	}
	return clone
}

func sameWarningConfig(current, requested config.Config) bool {
	return current.WarningsEnabled == requested.WarningsEnabled &&
		current.Latitude == requested.Latitude &&
		current.Longitude == requested.Longitude &&
		current.Locale == requested.Locale &&
		strings.Join(current.WarningTERYT, ",") == strings.Join(requested.WarningTERYT, ",")
}

func (a *App) Handler() http.Handler { return a.security(a.mux) }

func (a *App) markSourceAttempt(source string) {
	a.sourceHealthMu.Lock()
	state := a.sourceHealth[source]
	state.LastAttemptAt = time.Now().UTC()
	a.sourceHealth[source] = state
	a.sourceHealthMu.Unlock()
}

func (a *App) markSourceSuccess(source string) {
	a.sourceHealthMu.Lock()
	state := a.sourceHealth[source]
	state.LastSuccessAt = time.Now().UTC()
	state.Error = ""
	a.sourceHealth[source] = state
	a.sourceHealthMu.Unlock()
}

func (a *App) markSourceError(source, message string) {
	a.sourceHealthMu.Lock()
	state := a.sourceHealth[source]
	state.Error = message
	a.sourceHealth[source] = state
	a.sourceHealthMu.Unlock()
}

func (a *App) sourceHealthSnapshot() map[string]sourceHealth {
	a.sourceHealthMu.RLock()
	defer a.sourceHealthMu.RUnlock()
	result := make(map[string]sourceHealth, len(a.sourceHealth))
	for name, state := range a.sourceHealth {
		result[name] = state
	}
	return result
}

func (a *App) aprsDiagnosticsSnapshot() aprsReceiveDiagnostics {
	a.aprsDiagMu.RLock()
	defer a.aprsDiagMu.RUnlock()
	return a.aprsDiag
}

func (a *App) setAPRSTCPConnected(connected bool) {
	a.aprsDiagMu.Lock()
	a.aprsDiag.Connected = connected
	if connected {
		a.aprsDiag.ConnectedAt = time.Now().UTC()
	}
	a.aprsDiagMu.Unlock()
	if connected {
		a.sourceHealthMu.Lock()
		state := a.sourceHealth["aprs"]
		state.Error = ""
		a.sourceHealth["aprs"] = state
		a.sourceHealthMu.Unlock()
	}
}

func (a *App) recordAPRSBytes(count int) {
	if count <= 0 {
		return
	}
	a.aprsDiagMu.Lock()
	a.aprsDiag.BytesReceived += uint64(count)
	a.aprsDiag.LastDataAt = time.Now().UTC()
	a.aprsDiagMu.Unlock()
}

func (a *App) recordAPRSKISSFrame() {
	a.aprsDiagMu.Lock()
	a.aprsDiag.KISSFrames++
	a.aprsDiag.LastProcessingError = ""
	a.aprsDiagMu.Unlock()
}

func (a *App) recordAPRSAx25Frame() {
	a.aprsDiagMu.Lock()
	a.aprsDiag.AX25Frames++
	a.aprsDiagMu.Unlock()
}

func (a *App) recordAPRSStoredFrame() {
	a.aprsDiagMu.Lock()
	a.aprsDiag.FramesStored++
	a.aprsDiagMu.Unlock()
}

func (a *App) recordAPRSProcessingError(err error) {
	a.aprsDiagMu.Lock()
	if err == nil {
		a.aprsDiag.LastProcessingError = ""
	} else {
		a.aprsDiag.LastProcessingError = err.Error()
	}
	a.aprsDiagMu.Unlock()
}

func (a *App) Run(ctx context.Context) {
	go a.weatherLoop(ctx)
	go a.scheduleLoop(ctx)
	go a.serialLoop(ctx)
	go a.aprsLoop(ctx)
	go a.haLoop(ctx)
	go a.mqttLoop(ctx)
	go a.bmeLoop(ctx)
	go a.maintenanceLoop(ctx)
	go a.warningFetchLoop(ctx)
	go a.meteoAlarmLoop(ctx)
	go a.warningSpeakLoop(ctx)
	<-ctx.Done()
}
func (a *App) weatherLoop(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()
	for {
		a.fetchWeather(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
func (a *App) fetchWeather(ctx context.Context) {
	c := a.Config()
	s, e := (weather.OpenMeteo{}).Fetch(ctx, c.Latitude, c.Longitude, c.Units)
	if e != nil {
		a.snapMu.Lock()
		a.snapshot.Error = e.Error()
		a.snapMu.Unlock()
		log.Printf("weather update: %v", e)
		return
	}
	for i := range s.Forecast {
		s.Forecast[i].Parameter = "forecast_" + s.Forecast[i].Parameter
	}
	for _, o := range append(s.Current, s.Forecast...) {
		if e := a.history.AddObservation(o); e != nil {
			log.Printf("save observation: %v", e)
		}
	}
	a.snapMu.Lock()
	a.snapshot = s
	a.snapMu.Unlock()
}

func (a *App) routes() {
	a.mux.HandleFunc("/", a.index)
	a.mux.HandleFunc("/api/auth", a.authState)
	a.mux.HandleFunc("/api/locales", a.locales)
	a.mux.HandleFunc("/api/setup", a.setup)
	a.mux.HandleFunc("/api/login", a.login)
	a.mux.HandleFunc("/api/logout", a.logout)
	a.mux.HandleFunc("/api/security", a.securitySettings)
	a.mux.HandleFunc("/api/status", a.protect(a.status))
	a.mux.HandleFunc("/api/config", a.setupAccess(a.configuration))
	a.mux.HandleFunc("/api/backup", a.protect(a.configurationBackup))
	a.mux.HandleFunc("/api/weather", a.protect(a.weatherStatus))
	a.mux.HandleFunc("/api/warnings", a.protect(a.warningStatus))
	a.mux.HandleFunc("/api/warnings/refresh", a.protect(a.refreshWarnings))
	a.mux.HandleFunc("/api/stats", a.protect(a.stats))
	a.mux.HandleFunc("/api/aprs", a.protect(a.aprsFrames))
	a.mux.HandleFunc("/api/aprs/blacklist", a.protect(a.aprsBlacklist))
	a.mux.HandleFunc("/api/aprs/frame-blacklist", a.protect(a.aprsFrameBlacklist))
	a.mux.HandleFunc("/api/aprs/test", a.setupAccess(a.testAPRSConnection))
	a.mux.HandleFunc("/api/bme/test", a.setupAccess(a.testBMEConnection))
	a.mux.HandleFunc("/api/mqtt/test", a.setupAccess(a.testMQTTConnection))
	a.mux.HandleFunc("/api/home-assistant/test", a.setupAccess(a.testHomeAssistantConnection))
	a.mux.HandleFunc("/api/serial/test", a.setupAccess(a.testSerialConnection))
	a.mux.HandleFunc("/api/audio", a.setupAccess(a.audioDevices))
	a.mux.HandleFunc("/api/audio/test-input", a.setupAccess(a.testAudioInput))
	a.mux.HandleFunc("/api/audio/test-output", a.setupAccess(a.testAudioOutput))
	a.mux.HandleFunc("/api/announcement/preview", a.protect(a.preview))
	a.mux.HandleFunc("/api/announcement/play", a.protect(a.play))
	a.mux.HandleFunc("/api/location/search", a.setupAccess(a.searchLocation))
	a.mux.HandleFunc("/api/update", a.protect(a.update))
	a.mux.HandleFunc("/api/tts/catalog", a.protect(a.ttsCatalog))
	a.mux.HandleFunc("/api/tts/installed", a.protect(a.ttsInstalled))
	a.mux.HandleFunc("/api/tts/download", a.protect(a.ttsDownload))
	a.mux.HandleFunc("/api/tts/select", a.protect(a.ttsSelect))
	a.mux.HandleFunc("/api/tts/delete", a.protect(a.ttsDelete))
	a.mux.HandleFunc("/api/tts/test", a.protect(a.ttsTest))
}
func (a *App) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	io.WriteString(w, indexHTML)
}
func (a *App) security(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' https://tile.openstreetmap.org data:; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline'")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}
func (a *App) protect(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := a.Config()
		if c.AuthDisabled {
			next(w, r)
			return
		}
		if c.PasswordHash == "" {
			writeError(w, 403, "first-run setup required")
			return
		}
		cookie, e := r.Cookie("ultimatewx_session")
		if e != nil || !a.validSession(cookie.Value) {
			writeError(w, 401, "authentication required")
			return
		}
		next(w, r)
	}
}
func (a *App) setupAccess(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a.setupMu.Lock()
		c := a.Config()
		if c.PasswordHash == "" && !c.AuthDisabled {
			validToken := len(a.setupToken) >= 32 && subtle.ConstantTimeCompare([]byte(r.Header.Get("X-UltimateWX-Setup-Token")), []byte(a.setupToken)) == 1
			a.setupMu.Unlock()
			if !validToken {
				writeError(w, http.StatusForbidden, "first-run setup token required")
				return
			}
			next(w, r)
			return
		}
		a.setupMu.Unlock()
		a.protect(next)(w, r)
	}
}
func (a *App) validSession(token string) bool {
	a.sessionMu.Lock()
	defer a.sessionMu.Unlock()
	expiry, ok := a.sessions[token]
	if !ok {
		return false
	}
	if time.Now().After(expiry) {
		delete(a.sessions, token)
		return false
	}
	a.sessions[token] = time.Now().Add(12 * time.Hour)
	return true
}
func (a *App) newSession(w http.ResponseWriter) bool {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		http.Error(w, "session error", 500)
		return false
	}
	token := base64.RawURLEncoding.EncodeToString(b)
	now := time.Now()
	a.sessionMu.Lock()
	for existing, expiry := range a.sessions {
		if !expiry.After(now) {
			delete(a.sessions, existing)
		}
	}
	a.sessions[token] = now.Add(12 * time.Hour)
	a.sessionMu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: "ultimatewx_session", Value: token, Path: "/", HttpOnly: true, Secure: rIsTLS(w), SameSite: http.SameSiteStrictMode, MaxAge: 43200})
	return true
}
func rIsTLS(w http.ResponseWriter) bool { return false }
func (a *App) authState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		method(w)
		return
	}
	c := a.Config()
	writeJSON(w, 200, map[string]any{"setup_required": c.PasswordHash == "" && !c.AuthDisabled, "authentication_disabled": c.AuthDisabled})
}
func (a *App) setup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		method(w)
		return
	}
	a.setupMu.Lock()
	defer a.setupMu.Unlock()
	c := a.Config()
	if c.PasswordHash != "" || c.AuthDisabled {
		writeError(w, 409, "setup is already complete")
		return
	}
	var req struct {
		SetupToken   string   `json:"setup_token"`
		Password     string   `json:"password"`
		DisableAuth  bool     `json:"disable_auth"`
		LocationName string   `json:"location_name"`
		Latitude     *float64 `json:"latitude"`
		Longitude    *float64 `json:"longitude"`
		AltitudeM    *float64 `json:"altitude_m"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if len(a.setupToken) < 32 || subtle.ConstantTimeCompare([]byte(req.SetupToken), []byte(a.setupToken)) != 1 {
		writeError(w, http.StatusForbidden, "invalid first-run setup token")
		return
	}
	if req.DisableAuth {
		c.AuthDisabled = true
		if e := a.setConfig(c); e != nil {
			writeError(w, 400, e.Error())
			return
		}
		a.finishSetupToken()
		writeJSON(w, 200, map[string]any{"ok": true, "authentication_disabled": true})
		return
	}
	hash, e := config.HashPassword(req.Password)
	if e != nil {
		writeError(w, 400, e.Error())
		return
	}
	c.PasswordHash = hash
	c.AuthDisabled = false
	if req.LocationName != "" {
		c.LocationName = req.LocationName
	}
	if req.Latitude != nil {
		c.Latitude = *req.Latitude
	}
	if req.Longitude != nil {
		c.Longitude = *req.Longitude
	}
	if req.AltitudeM != nil {
		c.AltitudeM = *req.AltitudeM
	}
	if e = a.setConfig(c); e != nil {
		writeError(w, 400, e.Error())
		return
	}
	a.finishSetupToken()
	if !a.newSession(w) {
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (a *App) finishSetupToken() {
	a.setupToken = ""
	if err := os.Remove(filepath.Join(a.dataDir, "setup-token")); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf("remove first-run setup token: %v", err)
	}
}

func (a *App) login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		method(w)
		return
	}
	now := time.Now()
	client := loginClientKey(r.RemoteAddr)
	if a.loginLimited(client, now) {
		w.Header().Set("Retry-After", strconv.Itoa(int(loginAttemptWindow/time.Second)))
		writeError(w, http.StatusTooManyRequests, "too many login attempts; try again later")
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	c := a.Config()
	if c.AuthDisabled || !config.VerifyPassword(c.PasswordHash, req.Password) {
		a.recordLoginFailure(client, now)
		writeError(w, 401, "invalid password")
		return
	}
	if !a.newSession(w) {
		return
	}
	a.clearLoginFailures(client)
	writeJSON(w, 200, map[string]any{"ok": true})
}

func loginClientKey(remoteAddr string) string {
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
		return host
	}
	return strings.TrimSpace(remoteAddr)
}

func (a *App) loginLimited(client string, now time.Time) bool {
	a.loginAttemptMu.Lock()
	defer a.loginAttemptMu.Unlock()
	state, exists := a.loginAttempts[client]
	if exists && now.Sub(state.WindowStart) >= loginAttemptWindow {
		delete(a.loginAttempts, client)
		exists = false
	}
	if exists && state.BlockedUntil.After(now) {
		return true
	}
	if !exists && len(a.loginAttempts) >= loginAttemptCap {
		for key, candidate := range a.loginAttempts {
			if now.Sub(candidate.WindowStart) >= loginAttemptWindow {
				delete(a.loginAttempts, key)
			}
		}
		if len(a.loginAttempts) >= loginAttemptCap {
			return true
		}
	}
	return false
}

func (a *App) recordLoginFailure(client string, now time.Time) {
	a.loginAttemptMu.Lock()
	defer a.loginAttemptMu.Unlock()
	if _, exists := a.loginAttempts[client]; !exists && len(a.loginAttempts) >= loginAttemptCap {
		for key, candidate := range a.loginAttempts {
			if now.Sub(candidate.WindowStart) >= loginAttemptWindow {
				delete(a.loginAttempts, key)
			}
		}
		if len(a.loginAttempts) >= loginAttemptCap {
			return
		}
	}
	state := a.loginAttempts[client]
	if state.WindowStart.IsZero() || now.Sub(state.WindowStart) >= loginAttemptWindow {
		state = loginAttemptState{WindowStart: now}
	}
	state.Attempts++
	if state.Attempts >= loginAttemptLimit {
		state.WindowStart = now
		state.BlockedUntil = now.Add(loginAttemptWindow)
	}
	a.loginAttempts[client] = state
}

func (a *App) clearLoginFailures(client string) {
	a.loginAttemptMu.Lock()
	delete(a.loginAttempts, client)
	a.loginAttemptMu.Unlock()
}

func (a *App) beginUpdate() bool {
	a.updateMu.Lock()
	defer a.updateMu.Unlock()
	if a.updateInProgress {
		return false
	}
	a.updateInProgress = true
	return true
}

func (a *App) finishUpdate() {
	a.updateMu.Lock()
	a.updateInProgress = false
	a.updateMu.Unlock()
}

func (a *App) logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		method(w)
		return
	}
	if c, e := r.Cookie("ultimatewx_session"); e == nil {
		a.sessionMu.Lock()
		delete(a.sessions, c.Value)
		a.sessionMu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{Name: "ultimatewx_session", Value: "", Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: -1})
	writeJSON(w, 200, map[string]any{"ok": true})
}
func (a *App) securitySettings(w http.ResponseWriter, r *http.Request) {
	c := a.Config()
	if r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, map[string]any{"authentication_disabled": c.AuthDisabled, "password_configured": c.PasswordHash != ""})
		return
	}
	if r.Method != http.MethodPost {
		method(w)
		return
	}
	if c.PasswordHash == "" && !c.AuthDisabled {
		writeError(w, http.StatusForbidden, "complete the initial setup first")
		return
	}
	if !c.AuthDisabled {
		cookie, err := r.Cookie("ultimatewx_session")
		if err != nil || !a.validSession(cookie.Value) {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}
	}
	var req struct {
		Action          string `json:"action"`
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if c.AuthDisabled {
		if req.Action != "enable" {
			writeError(w, http.StatusBadRequest, "authentication is already disabled")
			return
		}
	} else {
		if !config.VerifyPassword(c.PasswordHash, req.CurrentPassword) {
			writeError(w, http.StatusUnauthorized, "current password is incorrect")
			return
		}
		if req.Action != "change" && req.Action != "disable" {
			writeError(w, http.StatusBadRequest, "unsupported security action")
			return
		}
	}
	switch req.Action {
	case "change", "enable":
		hash, err := config.HashPassword(req.NewPassword)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		c.PasswordHash = hash
		c.AuthDisabled = false
	case "disable":
		c.AuthDisabled = true
	default:
		writeError(w, http.StatusBadRequest, "unsupported security action")
		return
	}
	if err := a.setConfig(c); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	a.sessionMu.Lock()
	a.sessions = make(map[string]time.Time)
	a.sessionMu.Unlock()
	if !c.AuthDisabled {
		if !a.newSession(w) {
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "authentication_disabled": c.AuthDisabled})
}
func (a *App) status(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		method(w)
		return
	}
	c := a.Config()
	a.snapMu.RLock()
	s := a.snapshot
	a.snapMu.RUnlock()
	now := time.Now()
	observations := filterExcludedSensorObservations(a.history.LatestByDeviceExcludingAPRS(c.APRSBlacklist, c.APRSFrameBlacklist), c.SensorDevices)
	bmeLastReading := latestSourceReading(observations, "bme280")
	aprsLastReading := latestSourceReading(observations, "aprs")
	mqttLastReading := latestSourceReading(observations, "mqtt")
	haLastReading := latestSourceReading(observations, "home-assistant")
	serialLastReading := latestSourceReading(observations, "serial")
	bmeReceiving := c.BMEEnabled && isFreshReading(bmeLastReading, now, sourceFreshnessLimit("bme280"))
	a.bmeStatusMu.RLock()
	bmeError := a.bmeLastError
	a.bmeStatusMu.RUnlock()
	a.warningMu.RLock()
	warningError, warningUpdatedAt := a.warningError, a.warningUpdatedAt
	warningProviders := cloneWarningProviderHealth(a.warningProviders)
	a.warningMu.RUnlock()
	openMeteoFresh := !s.UpdatedAt.IsZero() && now.Sub(s.UpdatedAt) >= 0 && now.Sub(s.UpdatedAt) < sourceFreshnessLimit("open-meteo")
	aprsFresh := c.APRSHost != "" && isFreshReading(aprsLastReading, now, sourceFreshnessLimit("aprs"))
	mqttFresh := c.MQTTBroker != "" && isFreshReading(mqttLastReading, now, sourceFreshnessLimit("mqtt"))
	haFresh := c.HAURL != "" && isFreshReading(haLastReading, now, sourceFreshnessLimit("home-assistant"))
	serialFresh := c.SerialPort != "" && isFreshReading(serialLastReading, now, sourceFreshnessLimit("serial"))
	ttsEngineReady, ttsModelReady, audioPlayerReady := audio.Readiness(c.TTSBinary, c.TTSVoice)
	audioDevices := audio.List()
	audioOutputAvailable := strings.TrimSpace(c.AudioOutput) == "" && audioPlayerReady
	if c.AudioOutput != "" {
		for _, device := range audioDevices.Outputs {
			if device.Name == c.AudioOutput {
				audioOutputAvailable = audioPlayerReady
				break
			}
		}
	}
	ttsReady := speechReady(c.TTSVoice != "", ttsEngineReady, ttsModelReady, audioPlayerReady, audioOutputAvailable)
	audioInputAvailable := c.AudioInput == ""
	if c.AudioInput != "" {
		for _, device := range audioDevices.Inputs {
			if device.Name == c.AudioInput {
				audioInputAvailable = true
				break
			}
		}
	}
	configuredSources := map[string]bool{"open_meteo": true, "aprs": c.APRSHost != "", "mqtt": c.MQTTBroker != "", "home_assistant": c.HAURL != "" && c.HAToken != "", "serial": c.SerialPort != "", "bme280": c.BMEEnabled}
	writeJSON(w, 200, map[string]any{"version": updater.Version, "location": c.LocationName, "weather_updated_at": s.UpdatedAt, "weather_error": s.Error, "sources": map[string]any{"open_meteo": !openMeteoFresh, "aprs": !aprsFresh, "mqtt": !mqttFresh, "home_assistant": !haFresh, "serial": !serialFresh, "bme280": !bmeReceiving}, "source_configured": configuredSources, "source_last_reading_at": map[string]time.Time{"open_meteo": s.UpdatedAt, "aprs": aprsLastReading, "mqtt": mqttLastReading, "home_assistant": haLastReading, "serial": serialLastReading, "bme280": bmeLastReading}, "source_health": a.sourceHealthSnapshot(), "aprs_rx": a.aprsDiagnosticsSnapshot(), "bme280": map[string]any{"enabled": c.BMEEnabled, "receiving": bmeReceiving, "last_reading_at": bmeLastReading, "error": bmeError}, "warnings": map[string]any{"enabled": c.WarningsEnabled, "last_updated_at": warningUpdatedAt, "error": warningError, "providers": warningProviders}, "schedule": scheduleStatus(c.Schedules, now.In(stationLocation(c.Timezone))), "tts_configured": c.TTSVoice != "", "tts_ready": ttsReady, "tts_components": map[string]bool{"engine": ttsEngineReady, "model": ttsModelReady, "audio_player": audioPlayerReady, "audio_output": audioOutputAvailable}, "audio_input": map[string]any{"device": c.AudioInput, "configured": c.AudioInput != "", "available": audioInputAvailable}, "audio_output": map[string]any{"device": c.AudioOutput, "available": audioOutputAvailable, "player_available": audioPlayerReady}, "audio": audioDevices, "uptime_seconds": int64(time.Since(startedAt).Seconds())})
}

func speechReady(configured, engine, model, player, output bool) bool {
	return configured && engine && model && player && output
}

func latestSourceReading(observations []weather.Observation, source string) time.Time {
	var latest time.Time
	now := time.Now()
	for _, observation := range observations {
		if (observation.Source == source || strings.HasPrefix(observation.Source, source+":")) && !observation.ObservedAt.After(now) && observation.ObservedAt.After(latest) {
			latest = observation.ObservedAt
		}
	}
	return latest
}

func isFreshReading(at, now time.Time, maxAge time.Duration) bool {
	age := now.Sub(at)
	return !at.IsZero() && age >= 0 && age < maxAge
}

func sourceFreshnessLimit(source string) time.Duration {
	source, _, _ = strings.Cut(strings.ToLower(source), ":")
	switch source {
	case "open-meteo":
		return 30 * time.Minute
	case "bme280":
		return 90 * time.Second
	case "mqtt", "home-assistant":
		return 15 * time.Minute
	case "serial":
		return 5 * time.Minute
	case "aprs":
		return 2 * time.Hour
	default:
		return 2 * time.Hour
	}
}

func observationQualityCounts(observations []weather.Observation) map[string]int {
	counts := map[string]int{"measured": 0, "modelled": 0, "derived": 0, "unknown": 0}
	for _, observation := range observations {
		if _, ok := counts[observation.Quality]; !ok {
			counts["unknown"]++
			continue
		}
		counts[observation.Quality]++
	}
	return counts
}

func observationFreshnessCounts(observations []weather.Observation, now time.Time) map[string]int {
	counts := map[string]int{"fresh": 0, "stale": 0}
	for _, observation := range observations {
		if isFreshReading(observation.ObservedAt, now, sourceFreshnessLimit(observation.Source)) {
			counts["fresh"]++
		} else {
			counts["stale"]++
		}
	}
	return counts
}

func isFreshObservation(observation weather.Observation, now time.Time) bool {
	return isFreshReading(observation.ObservedAt, now, sourceFreshnessLimit(observation.Source))
}

func sourceFreshnessLimitsSeconds() map[string]int64 {
	limits := map[string]int64{}
	for _, source := range []string{"open-meteo", "aprs", "mqtt", "home-assistant", "serial", "bme280"} {
		limits[source] = int64(sourceFreshnessLimit(source) / time.Second)
	}
	return limits
}

func scheduleStatus(schedules []config.Schedule, now time.Time) map[string]any {
	enabled := 0
	var next time.Time
	for _, schedule := range schedules {
		if !schedule.Enabled {
			continue
		}
		var candidate time.Time
		if schedule.EveryHour {
			if schedule.OffsetMinutes < 0 || schedule.OffsetMinutes > 59 {
				continue
			}
			candidate = time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), schedule.OffsetMinutes, 0, 0, now.Location())
			if !candidate.After(now) {
				candidate = candidate.Add(time.Hour)
			}
		} else {
			parsed, err := time.Parse("15:04", schedule.Time)
			if err != nil {
				continue
			}
			candidate = time.Date(now.Year(), now.Month(), now.Day(), parsed.Hour(), parsed.Minute(), 0, 0, now.Location())
			if !candidate.After(now) {
				candidate = candidate.AddDate(0, 0, 1)
			}
		}
		enabled++
		if next.IsZero() || candidate.Before(next) {
			next = candidate
		}
	}
	return map[string]any{"enabled": enabled > 0, "enabled_count": enabled, "next_at": next}
}
func (a *App) configuration(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		c := a.Config()
		c.PasswordHash = ""
		c.MQTTPassword = ""
		c.HAToken = ""
		writeJSON(w, 200, c)
	case http.MethodPut:
		var c config.Config
		if !decodeJSON(w, r, &c) {
			return
		}
		old := a.Config()
		if c.MQTTPassword == "" {
			c.MQTTPassword = old.MQTTPassword
		}
		if c.HAToken == "" {
			c.HAToken = old.HAToken
		}
		c.PasswordHash = old.PasswordHash
		c.AuthDisabled = old.AuthDisabled
		if e := a.setConfig(c); e != nil {
			writeError(w, 400, e.Error())
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true})
	default:
		method(w)
	}
}

func (a *App) configurationBackup(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		c := a.Config()
		c.PasswordHash = ""
		c.AuthDisabled = false
		backup := struct {
			Format     string        `json:"format"`
			Version    int           `json:"version"`
			ExportedAt time.Time     `json:"exported_at"`
			Config     config.Config `json:"config"`
		}{Format: "ultimatewx-config-backup", Version: 1, ExportedAt: time.Now().UTC(), Config: c}
		data, err := json.MarshalIndent(backup, "", "  ")
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not create configuration backup")
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="ultimatewx-config-backup.json"`)
		w.Header().Set("Cache-Control", "no-store")
		w.Write(append(data, '\n'))
	case http.MethodPost:
		var backup struct {
			Format     string        `json:"format"`
			Version    int           `json:"version"`
			ExportedAt time.Time     `json:"exported_at"`
			Config     config.Config `json:"config"`
		}
		if !decodeJSON(w, r, &backup) {
			return
		}
		if backup.Format != "ultimatewx-config-backup" || backup.Version != 1 {
			writeError(w, http.StatusBadRequest, "unsupported configuration backup format")
			return
		}
		current := a.Config()
		backup.Config.PasswordHash = current.PasswordHash
		backup.Config.AuthDisabled = current.AuthDisabled
		if backup.Config.MQTTPassword == "" {
			backup.Config.MQTTPassword = current.MQTTPassword
		}
		if backup.Config.HAToken == "" {
			backup.Config.HAToken = current.HAToken
		}
		if err := a.setConfig(backup.Config); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		method(w)
	}
}

func (a *App) weatherStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		method(w)
		return
	}
	a.snapMu.RLock()
	s := a.snapshot
	a.snapMu.RUnlock()
	c := a.Config()
	days := c.HistoryDays
	latestByDevice := filterExcludedSensorObservations(a.history.LatestByDeviceExcludingAPRS(c.APRSBlacklist, c.APRSFrameBlacklist), c.SensorDevices)
	for i := range latestByDevice {
		if device, ok := c.SensorDevices[sensorDeviceConfigKey(latestByDevice[i].Source, latestByDevice[i].DeviceID)]; ok && strings.TrimSpace(device.Name) != "" {
			latestByDevice[i].DeviceName = strings.TrimSpace(device.Name)
		}
		latestByDevice[i] = weather.Convert(latestByDevice[i], c.Units)
	}
	latest := latestByParameterSource(latestByDevice)
	for i := range latest {
		latest[i] = weather.Convert(latest[i], c.Units)
	}
	history := excludeBlacklistedAPRS(a.history.Observations(days), c.APRSBlacklist, c.APRSFrameBlacklist)
	history = filterExcludedSensorObservations(history, c.SensorDevices)
	writeJSON(w, 200, map[string]any{"snapshot": s, "latest": latest, "latest_by_device": latestByDevice, "local_estimate": estimate(history), "freshness_limits_seconds": sourceFreshnessLimitsSeconds()})
}
func (a *App) warningStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		method(w)
		return
	}
	if !a.Config().WarningsEnabled {
		writeJSON(w, http.StatusOK, map[string]any{"active": []warnings.Alert{}, "upcoming": []warnings.Alert{}, "ended": a.history.EndedWarnings(), "updated_at": time.Time{}, "error": "", "source": "IMGW-PIB, DWD, GeoSphere Austria, MeteoAlarm/WIS2", "providers": a.warningProviderHealthSnapshot()})
		return
	}
	a.warningMu.RLock()
	updatedAt, err := a.warningUpdatedAt, a.warningError
	providers := cloneWarningProviderHealth(a.warningProviders)
	a.warningMu.RUnlock()
	now := time.Now().UTC()
	items := a.history.Warnings()
	active := make([]warnings.Alert, 0, len(items))
	upcoming := make([]warnings.Alert, 0, len(items))
	ended := a.history.EndedWarnings()
	for _, item := range items {
		if !item.ValidTo.After(now) {
			continue
		}
		if item.ValidFrom.After(now) {
			upcoming = append(upcoming, item)
		} else {
			active = append(active, item)
		}
	}
	sort.Slice(ended, func(i, j int) bool { return ended[i].ValidTo.After(ended[j].ValidTo) })
	writeJSON(w, http.StatusOK, map[string]any{"active": active, "upcoming": upcoming, "ended": ended, "updated_at": updatedAt, "error": err, "source": "IMGW-PIB, DWD, GeoSphere Austria, MeteoAlarm/WIS2", "providers": providers})
}
func (a *App) refreshWarnings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		method(w)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	if err := a.fetchWarnings(ctx); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	statusRequest := r.Clone(ctx)
	statusRequest.Method = http.MethodGet
	a.warningStatus(w, statusRequest)
}
func (a *App) stats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		method(w)
		return
	}
	c := a.Config()
	if err := a.history.Prune(c.HistoryDays); err != nil {
		log.Printf("prune history for statistics: %v", err)
	}
	stats := a.history.StatsExcludingAPRS(c.APRSBlacklist, c.APRSFrameBlacklist)
	chartHistoryDays := c.HistoryDays
	if requestedHours, err := strconv.Atoi(r.URL.Query().Get("hours")); err == nil && requestedHours > 0 {
		if requestedHours > 720 {
			requestedHours = 720
		}
		chartHistoryDays = (requestedHours + 23) / 24
	}
	if chartHistoryDays > 30 {
		chartHistoryDays = 30
	}
	observations := excludeBlacklistedAPRS(a.history.Observations(chartHistoryDays), c.APRSBlacklist, c.APRSFrameBlacklist)
	latestReadings := filterExcludedSensorObservations(a.history.LatestByDeviceExcludingAPRS(c.APRSBlacklist, c.APRSFrameBlacklist), c.SensorDevices)
	stats["quality_counts"] = observationQualityCounts(observations)
	stats["freshness_counts"] = observationFreshnessCounts(latestReadings, time.Now())
	for i := range observations {
		observations[i] = weather.Convert(observations[i], c.Units)
	}
	stats["history"] = observations
	writeJSON(w, 200, stats)
}
func (a *App) aprsFrames(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		method(w)
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 500 {
		limit = 100
	}
	wx := r.URL.Query().Get("wx_only") == "true"
	frames := a.history.Frames(limit, wx)
	c := a.Config()
	for i := range frames {
		if refreshed, err := aprs.ParseTNC2(frames[i].Raw, frames[i].ReceivedAt, c.Latitude, c.Longitude, c.APRSRadiusKM); err == nil {
			frames[i] = refreshed
		}
		frames[i] = aprs.ApplyBlacklist(frames[i], c.APRSBlacklist)
		frames[i] = aprs.ApplyFrameBlacklist(frames[i], c.APRSFrameBlacklist)
	}
	writeJSON(w, 200, map[string]any{"frames": frames})
}

func (a *App) aprsFrameBlacklist(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		method(w)
		return
	}
	var request struct {
		FrameID string `json:"frame_id"`
		Blocked bool   `json:"blocked"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	if len(request.FrameID) != 64 || strings.Trim(request.FrameID, "0123456789abcdef") != "" {
		writeError(w, http.StatusBadRequest, "The APRS frame identifier must be a lowercase SHA-256 hash.")
		return
	}
	c := a.Config()
	filtered := make([]string, 0, len(c.APRSFrameBlacklist)+1)
	found := false
	for _, existing := range c.APRSFrameBlacklist {
		if existing == request.FrameID {
			found = true
			if !request.Blocked {
				continue
			}
		}
		filtered = append(filtered, existing)
	}
	if request.Blocked && !found {
		if len(filtered) >= 500 {
			writeError(w, http.StatusBadRequest, "The APRS frame blacklist already contains 500 entries.")
			return
		}
		filtered = append(filtered, request.FrameID)
	}
	c.APRSFrameBlacklist = filtered
	if err := a.setConfig(c); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "frame_id": request.FrameID, "blocked": request.Blocked})
}

func (a *App) aprsBlacklist(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		method(w)
		return
	}
	var request struct {
		Station string `json:"station"`
		Blocked bool   `json:"blocked"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	station := strings.ToUpper(strings.TrimSpace(request.Station))
	if station == "" || len(station) > 9 || strings.ContainsAny(station, ",; \t\r\n") {
		writeError(w, http.StatusBadRequest, "Enter one valid APRS callsign.")
		return
	}
	c := a.Config()
	filtered := make([]string, 0, len(c.APRSBlacklist)+1)
	found := false
	for _, existing := range c.APRSBlacklist {
		existing = strings.ToUpper(strings.TrimSpace(existing))
		if existing == station {
			found = true
			if !request.Blocked {
				continue
			}
		}
		filtered = append(filtered, existing)
	}
	if request.Blocked && !found {
		if len(filtered) >= 100 {
			writeError(w, http.StatusBadRequest, "The APRS blacklist already contains 100 callsigns.")
			return
		}
		filtered = append(filtered, station)
	}
	c.APRSBlacklist = filtered
	if err := a.setConfig(c); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "station": station, "blocked": request.Blocked})
}

func (a *App) testAPRSConnection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		method(w)
		return
	}
	var request struct {
		Host string `json:"host"`
		Port int    `json:"port"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	request.Host = strings.TrimSpace(request.Host)
	if request.Host == "" || request.Port < 1 || request.Port > 65535 {
		writeError(w, http.StatusBadRequest, "Enter a valid KISS host and TCP port (1–65535).")
		return
	}
	address := net.JoinHostPort(request.Host, strconv.Itoa(request.Port))
	ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", address)
	if err != nil {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("TCP connection to %s failed: %v", address, err))
		return
	}
	_ = conn.Close()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "TCP connection established"})
}

func (a *App) testMQTTConnection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		method(w)
		return
	}
	var request struct {
		Broker   string `json:"broker"`
		Topic    string `json:"topic"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	request.Broker = strings.TrimSpace(request.Broker)
	request.Topic = strings.TrimSpace(request.Topic)
	if request.Broker == "" || request.Topic == "" {
		writeError(w, http.StatusBadRequest, "Enter an MQTT broker and topic.")
		return
	}
	if _, err := mqtt.BrokerAddress(request.Broker); err != nil {
		writeError(w, http.StatusBadRequest, "Enter a valid MQTT broker address.")
		return
	}
	if request.Password == "" {
		request.Password = a.Config().MQTTPassword
	}
	clientID := fmt.Sprintf("ultimatewx-test-%d", time.Now().UnixNano())
	conn, err := mqtt.Connect(request.Broker, clientID, request.Username, request.Password, request.Topic)
	if err != nil {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("MQTT connection or subscription failed: %v", err))
		return
	}
	_ = conn.Close()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "MQTT connection and subscription succeeded"})
}

func (a *App) testHomeAssistantConnection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		method(w)
		return
	}
	var request struct {
		URL   string `json:"url"`
		Token string `json:"token"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	request.URL = strings.TrimRight(strings.TrimSpace(request.URL), "/")
	parsed, err := url.ParseRequestURI(request.URL)
	if err != nil || parsed.Hostname() == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		writeError(w, http.StatusBadRequest, "Enter a valid Home Assistant HTTP or HTTPS URL.")
		return
	}
	if request.Token == "" {
		request.Token = a.Config().HAToken
	}
	if request.Token == "" {
		writeError(w, http.StatusBadRequest, "Enter a Home Assistant token.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, request.URL+"/api/", nil)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Enter a valid Home Assistant URL.")
		return
	}
	req.Header.Set("Authorization", "Bearer "+request.Token)
	resp, err := (&http.Client{Timeout: 8 * time.Second}).Do(req)
	if err != nil {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("Home Assistant connection failed: %v", err))
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("Home Assistant returned HTTP %d", resp.StatusCode))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "Home Assistant API connection succeeded"})
}

func (a *App) testBMEConnection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		method(w)
		return
	}
	var request struct {
		Device  string `json:"device"`
		Address int    `json:"address"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	request.Device = strings.TrimSpace(request.Device)
	if request.Device == "" || (request.Address != 0x76 && request.Address != 0x77) {
		writeError(w, http.StatusBadRequest, "Enter an I²C device and address 118 (0x76) or 119 (0x77).")
		return
	}
	temperature, humidity, pressure, err := bme280.Read(request.Device, uint16(request.Address))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]float64{"temperature_c": temperature, "humidity_percent": humidity, "pressure_hpa": pressure})
}

func (a *App) testSerialConnection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		method(w)
		return
	}
	var request struct {
		Port string `json:"port"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	port := strings.TrimSpace(request.Port)
	cleanPort := filepath.Clean(port)
	if port == "" || len(port) > 256 || strings.ContainsAny(port, "\r\n\x00") || !filepath.IsAbs(port) || !strings.HasPrefix(cleanPort, "/dev/") {
		writeError(w, http.StatusBadRequest, "Enter a serial device path under /dev.")
		return
	}
	info, err := os.Stat(cleanPort)
	if err != nil || info.Mode()&os.ModeCharDevice == 0 {
		writeError(w, http.StatusBadGateway, "Serial device is unavailable or is not a character device.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "stty", "-F", cleanPort, "-a").CombinedOutput()
	if err != nil {
		writeError(w, http.StatusBadGateway, "Serial device cannot be queried: "+strings.TrimSpace(string(output)))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "port": cleanPort, "message": "Serial device is accessible; incoming JSON samples are not verified by this check."})
}

func (a *App) audioDevices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		method(w)
		return
	}
	writeJSON(w, 200, audio.List())
}
func (a *App) testAudioInput(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		method(w)
		return
	}
	var request struct {
		Device string `json:"device"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	if len(request.Device) > 256 || strings.ContainsAny(request.Device, "\r\n\x00") {
		writeError(w, http.StatusBadRequest, "Invalid audio input device.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 6*time.Second)
	defer cancel()
	if err := audio.TestInput(ctx, request.Device); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *App) testAudioOutput(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		method(w)
		return
	}
	var request struct {
		Device string `json:"device"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	request.Device = strings.TrimSpace(request.Device)
	if len(request.Device) > 256 || strings.ContainsAny(request.Device, "\r\n\x00") {
		writeError(w, http.StatusBadRequest, "Invalid audio output device.")
		return
	}
	c := a.Config()
	if request.Device != "" {
		available := false
		for _, device := range audio.List().Outputs {
			if device.Name == request.Device {
				available = true
				break
			}
		}
		if !available {
			writeError(w, http.StatusBadRequest, "Selected audio output device is no longer available.")
			return
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	a.speechMu.Lock()
	err := audio.TestOutput(ctx, request.Device, c.AudioVolume)
	a.speechMu.Unlock()
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *App) ttsCatalog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		method(w)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	voices, e := a.voices.Catalog(ctx, r.URL.Query().Get("language"))
	if e != nil {
		writeError(w, 502, e.Error())
		return
	}
	activePath := filepath.Clean(a.Config().TTSVoice)
	for i := range voices {
		voices[i].Installed = a.voices.Installed(voices[i].ID)
		voices[i].Active = filepath.Clean(filepath.Join(a.dataDir, "voices", voices[i].ID+".onnx")) == activePath
	}
	writeJSON(w, 200, map[string]any{"voices": voices, "source": "Piper voice catalog", "notice": "Sprawdź model card i atrybucję; dostępność zależy od warunków licencji modelu."})
}
func (a *App) ttsInstalled(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		method(w)
		return
	}
	voices, err := a.voices.InstalledVoices()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	activePath := filepath.Clean(a.Config().TTSVoice)
	for i := range voices {
		voices[i].Active = filepath.Clean(filepath.Join(a.dataDir, "voices", voices[i].ID+".onnx")) == activePath
	}
	writeJSON(w, http.StatusOK, map[string]any{"voices": voices})
}
func (a *App) ttsSelect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		method(w)
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	voices, err := a.voices.InstalledVoices()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	for _, voice := range voices {
		if voice.ID != req.ID {
			continue
		}
		c := a.Config()
		c.TTSVoice = filepath.Join(a.dataDir, "voices", voice.ID+".onnx")
		c.Locale = voice.Language
		if err := a.setConfig(c); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "path": c.TTSVoice, "language": c.Locale})
		return
	}
	writeError(w, http.StatusNotFound, "voice is not installed")
}
func (a *App) ttsDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		method(w)
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	voices, err := a.voices.InstalledVoices()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var voiceID string
	for _, voice := range voices {
		if voice.ID == req.ID {
			voiceID = voice.ID
			break
		}
	}
	if voiceID == "" {
		writeError(w, http.StatusNotFound, "voice is not installed")
		return
	}

	c := a.Config()
	modelPath := filepath.Join(a.dataDir, "voices", voiceID+".onnx")
	active := c.TTSVoice != "" && filepath.Clean(c.TTSVoice) == filepath.Clean(modelPath)
	if active {
		previous := c
		c.TTSVoice = ""
		c.Locale = ""
		if err := a.setConfig(c); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if err := a.voices.Delete(voiceID); err != nil {
			if restoreErr := a.setConfig(previous); restoreErr != nil {
				writeError(w, http.StatusInternalServerError, fmt.Sprintf("delete voice: %v; restore configuration: %v", err, restoreErr))
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	} else if err := a.voices.Delete(voiceID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "active_cleared": active})
}
func (a *App) ttsDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		method(w)
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 4*time.Minute)
	defer cancel()
	path, e := a.voices.Download(ctx, req.ID)
	if e != nil {
		writeError(w, 400, e.Error())
		return
	}
	c := a.Config()
	c.TTSVoice = path
	if c.TTSBinary == "" {
		c.TTSBinary = "piper"
	}
	if e = a.setConfig(c); e != nil {
		writeError(w, 500, e.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "path": path})
}
func (a *App) ttsTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		method(w)
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 4*time.Minute)
	defer cancel()
	voices, err := a.voices.InstalledVoices()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var voice *tts.Voice
	for i := range voices {
		if voices[i].ID == req.ID {
			voice = &voices[i]
			break
		}
	}
	downloaded := false
	if voice == nil {
		_, downloadErr := a.voices.Download(ctx, req.ID)
		if downloadErr != nil {
			writeError(w, http.StatusNotFound, downloadErr.Error())
			return
		}
		voice = &tts.Voice{ID: req.ID, Language: strings.ToLower(strings.SplitN(req.ID, "_", 2)[0])}
		downloaded = true
	}
	c := a.Config()
	samples := map[string]string{
		"en": "This is a test of the selected weather station voice.",
		"pl": "To jest test wybranego głosu stacji pogodowej.",
		"de": "Dies ist ein Test der ausgewählten Stimme der Wetterstation.",
		"fr": "Ceci est un test de la voix sélectionnée de la station météo.",
		"es": "Esta es una prueba de la voz seleccionada de la estación meteorológica.",
		"it": "Questa è una prova della voce selezionata della stazione meteo.",
	}
	sample := samples[voice.Language]
	if sample == "" {
		sample = samples["en"]
	}
	model := filepath.Join(a.dataDir, "voices", voice.ID+".onnx")
	a.speechMu.Lock()
	err = audio.Speak(ctx, sample, model, c.TTSBinary, c.AudioOutput, c.AudioVolume)
	a.speechMu.Unlock()
	if err != nil {
		if downloaded {
			writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error(), "downloaded": true})
			return
		}
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "downloaded": downloaded})
}
func (a *App) preview(w http.ResponseWriter, r *http.Request) {
	text := a.buildAnnouncement()
	writeJSON(w, 200, map[string]string{"text": text})
}
func (a *App) play(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		method(w)
		return
	}
	text := a.buildAnnouncement()
	c := a.Config()
	a.speechMu.Lock()
	ctx, cancel := context.WithTimeout(r.Context(), 4*time.Minute)
	defer cancel()
	err := audio.Speak(ctx, text, c.TTSVoice, c.TTSBinary, c.AudioOutput, c.AudioVolume)
	a.speechMu.Unlock()
	result := "played"
	if err != nil {
		result = err.Error()
	}
	_ = a.history.AddPlayback(store.Playback{At: time.Now().UTC(), Text: text, Result: result})
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "text": text})
}
func (a *App) searchLocation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		method(w)
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeError(w, 400, "query required")
		return
	}
	if len(q) > 120 {
		writeError(w, http.StatusBadRequest, "query too long")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	endpoint := "https://geocoding-api.open-meteo.com/v1/search?name=" + urlQuery(q) + "&count=8&format=json"
	if language := a.Config().Locale; language != "" {
		endpoint += "&language=" + urlQuery(language)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not build geocoding request")
		return
	}
	resp, err := (&http.Client{Timeout: 8 * time.Second}).Do(req)
	if err != nil {
		writeError(w, http.StatusBadGateway, "location search unavailable")
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		writeError(w, http.StatusBadGateway, "location search unavailable")
		return
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
	if err != nil {
		writeError(w, http.StatusBadGateway, "could not read location search response")
		return
	}
	if len(body) > 1<<20 {
		writeError(w, http.StatusBadGateway, "location search response too large")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}
func (a *App) update(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		writeJSON(w, 200, updater.Check(ctx, "linux", runtimeArch()))
	case http.MethodPost:
		if !a.beginUpdate() {
			writeError(w, http.StatusConflict, "an update is already in progress")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Minute)
		defer cancel()
		if e := updater.Apply(ctx, "linux", runtimeArch()); e != nil {
			a.finishUpdate()
			writeError(w, 400, e.Error())
			return
		}
		restarting := updater.Version != "dev"
		if !restarting {
			a.finishUpdate()
		}
		writeJSON(w, 200, map[string]any{"ok": true, "restarting": restarting})
		if restarting {
			go func() {
				time.Sleep(1500 * time.Millisecond)
				process, err := os.FindProcess(os.Getpid())
				if err == nil {
					err = process.Signal(os.Interrupt)
				}
				if err != nil {
					log.Printf("graceful update restart failed: %v", err)
					os.Exit(1)
				}
			}()
		}
	default:
		method(w)
	}
}

func (a *App) buildAnnouncement() string {
	c := a.Config()
	deviceObservations := filterExcludedSensorObservations(a.history.LatestByDeviceExcludingAPRS(c.APRSBlacklist, c.APRSFrameBlacklist), c.SensorDevices)
	latest := latestByParameterSource(deviceObservations)
	byKey := map[string]weather.Observation{}
	for _, o := range latest {
		byKey[o.Parameter+"|"+o.Source] = o
	}
	var out []string
	for _, b := range c.Blocks {
		if !b.Enabled {
			continue
		}
		switch b.Kind {
		case "text":
			if text := renderStationTemplate(b.Text, c.StationCallsign, c.StationLocator); text != "" {
				out = append(out, text)
			}
		case "datetime":
			out = append(out, time.Now().In(stationLocation(c.Timezone)).Format("2006-01-02 15:04"))
		case "weather", "forecast":
			prefix := ""
			blockUnits := c.Units
			if b.Units != "" {
				blockUnits = b.Units
			}
			if b.Kind == "forecast" {
				prefix = "forecast_"
			}
			for _, field := range b.Fields {
				if b.Kind == "weather" && (b.Source == "" || b.Source == "auto") {
					var o weather.Observation
					var ok bool
					if order := c.WeatherSourceOrder[field]; len(order) > 0 {
						o, ok = selectObservationByPreferredOrder(byKey, deviceObservations, c.SensorDevices, prefix+field, order)
					}
					if !ok && c.WeatherSource != "" && c.WeatherSource != "auto" {
						o, ok = selectObservationWithDevicePreference(byKey, deviceObservations, c.SensorDevices, prefix+field, c.WeatherSource)
					}
					if !ok {
						o, ok = selectObservationWithDevicePreference(byKey, deviceObservations, c.SensorDevices, prefix+field, "auto")
					}
					if ok {
						out = append(out, renderObservation(weather.Convert(o, blockUnits), c.Locale, b.Format))
					}
					continue
				}
				source := b.Source
				if b.Kind == "weather" && (source == "" || source == "auto") && c.WeatherSource != "" {
					source = c.WeatherSource
				}
				o, ok := selectBlockObservationWithDevicePreference(byKey, deviceObservations, c.SensorDevices, prefix+field, source, b.FallbackSources)
				if !ok && b.Kind == "weather" && (b.Source == "" || b.Source == "auto") && source != "" && source != "auto" {
					o, ok = selectObservationWithDevicePreference(byKey, deviceObservations, c.SensorDevices, prefix+field, "auto")
				}
				if ok {
					out = append(out, renderObservation(weather.Convert(o, blockUnits), c.Locale, b.Format))
				}
			}
		case "local-estimate":
			history := excludeBlacklistedAPRS(a.history.Observations(c.HistoryDays), c.APRSBlacklist, c.APRSFrameBlacklist)
			history = filterExcludedSensorObservations(history, c.SensorDevices)
			if estimateText := estimateSpeech(estimate(history), c.Locale); estimateText != "" {
				out = append(out, estimateText)
			}
		}
	}
	return strings.Join(out, " ")
}

func excludeBlacklistedAPRS(observations []weather.Observation, blacklist, frameBlacklist []string) []weather.Observation {
	if len(blacklist) == 0 && len(frameBlacklist) == 0 {
		return observations
	}
	filtered := make([]weather.Observation, 0, len(observations))
	for _, observation := range observations {
		if strings.EqualFold(observation.Source, "aprs") && (isBlacklistedCallsign(observation.Station, blacklist) || isBlacklistedFrameID(observation.FrameID, frameBlacklist)) {
			continue
		}
		filtered = append(filtered, observation)
	}
	return filtered
}

func isBlacklistedCallsign(station string, blacklist []string) bool {
	for _, callsign := range blacklist {
		if station != "" && strings.EqualFold(strings.TrimSpace(station), strings.TrimSpace(callsign)) {
			return true
		}
	}
	return false
}

func isBlacklistedFrameID(frameID string, blacklist []string) bool {
	for _, blockedID := range blacklist {
		if frameID != "" && frameID == blockedID {
			return true
		}
	}
	return false
}

func renderStationTemplate(text, callsign, locator string) string {
	return strings.TrimSpace(strings.NewReplacer("<call>", callsign, "<loc>", locator).Replace(text))
}

func estimateSpeech(result map[string]any, locale string) string {
	available, _ := result["available"].(bool)
	trend, _ := result["trend"].(string)
	if !available {
		return ""
	}
	labels := map[string][6]string{"rosnące": {"rising", "rosnące", "steigend", "en hausse", "en ascenso", "in aumento"}, "spadające": {"falling", "spadające", "fallend", "en baisse", "en descenso", "in diminuzione"}, "stabilne": {"steady", "stabilne", "stabil", "stable", "estable", "stabile"}}
	idx := map[string]int{"en": 0, "pl": 1, "de": 2, "fr": 3, "es": 4, "it": 5}[locale]
	if phrase, ok := labels[trend]; ok {
		trend = phrase[idx]
	}
	return localized(locale, "Local estimate based on sensor pressure trend: ", "Lokalny szacunek na podstawie trendu ciśnienia z czujnika: ", "Lokale Schätzung anhand des Drucktrends: ", "Estimation locale basée sur la tendance de pression du capteur : ", "Estimación local basada en la tendencia de presión del sensor: ", "Stima locale basata sulla tendenza della pressione del sensore: ") + trend + "."
}
func selectObservation(m map[string]weather.Observation, key, source string) (weather.Observation, bool) {
	now := time.Now()
	if source != "" && source != "auto" {
		o, ok := m[key+"|"+source]
		if ok && isFreshObservation(o, now) {
			return o, true
		}
		if strings.Contains(source, ":") {
			return weather.Observation{}, false
		}
		prefix := key + "|" + source + ":"
		var best weather.Observation
		found := false
		for storedKey, candidate := range m {
			if strings.HasPrefix(storedKey, prefix) && isFreshObservation(candidate, now) && (!found || candidate.ObservedAt.After(best.ObservedAt)) {
				best, found = candidate, true
			}
		}
		return best, found
	}
	var best weather.Observation
	ok := false
	for k, o := range m {
		if strings.HasPrefix(k, key+"|") && isFreshObservation(o, now) && (!ok || o.ObservedAt.After(best.ObservedAt)) {
			best, ok = o, true
		}
	}
	return best, ok
}

func selectObservationWithDevicePreference(observations map[string]weather.Observation, deviceObservations []weather.Observation, devices map[string]config.SensorDeviceConfig, key, source string) (weather.Observation, bool) {
	if preferred, ok := selectPreferredSensorObservation(deviceObservations, devices, key, source); ok {
		return preferred, true
	}
	return selectObservation(observations, key, source)
}

func selectObservationByPreferredOrder(observations map[string]weather.Observation, deviceObservations []weather.Observation, devices map[string]config.SensorDeviceConfig, key string, sources []string) (weather.Observation, bool) {
	for _, source := range sources {
		if observation, ok := selectObservationWithDevicePreference(observations, deviceObservations, devices, key, source); ok {
			return observation, true
		}
	}
	return weather.Observation{}, false
}

func selectBlockObservationWithDevicePreference(observations map[string]weather.Observation, deviceObservations []weather.Observation, devices map[string]config.SensorDeviceConfig, key, source string, fallbacks []string) (weather.Observation, bool) {
	if source == "" || source == "auto" {
		return selectObservationWithDevicePreference(observations, deviceObservations, devices, key, "auto")
	}
	for _, candidate := range append([]string{source}, fallbacks...) {
		if observation, ok := selectObservationWithDevicePreference(observations, deviceObservations, devices, key, candidate); ok {
			return observation, true
		}
	}
	return selectObservationWithDevicePreference(observations, deviceObservations, devices, key, "auto")
}

func selectPreferredSensorObservation(observations []weather.Observation, devices map[string]config.SensorDeviceConfig, parameter, source string) (weather.Observation, bool) {
	now := time.Now()
	var best weather.Observation
	found := false
	for _, observation := range observations {
		if observation.Parameter != parameter || !isFreshObservation(observation, now) {
			continue
		}
		if source != "" && source != "auto" {
			kind := strings.SplitN(observation.Source, ":", 2)[0]
			if strings.Contains(source, ":") {
				if observation.Source != source {
					continue
				}
			} else if observation.Source != source && kind != source {
				continue
			}
		}
		device, ok := devices[sensorDeviceConfigKey(observation.Source, observation.DeviceID)]
		if !ok || !containsSensorParameter(device.PreferredParameters, parameter) {
			continue
		}
		if !found || observation.ObservedAt.After(best.ObservedAt) || (observation.ObservedAt.Equal(best.ObservedAt) && observation.Source+"|"+observation.DeviceID < best.Source+"|"+best.DeviceID) {
			best, found = observation, true
		}
	}
	return best, found
}

func renderObservation(o weather.Observation, locale, format string) string {
	labels := map[string][6]string{"temperature": {"temperature", "temperatura", "Temperatur", "température", "temperatura", "temperatura"}, "humidity": {"humidity", "wilgotność", "Luftfeuchtigkeit", "humidité", "humedad", "umidità"}, "pressure": {"pressure", "ciśnienie", "Luftdruck", "pression", "presión", "pressione"}, "wind_speed": {"wind speed", "prędkość wiatru", "Windgeschwindigkeit", "vitesse du vent", "velocidad del viento", "velocità del vento"}, "wind_direction": {"wind direction", "kierunek wiatru", "Windrichtung", "direction du vent", "dirección del viento", "direzione del vento"}, "precipitation": {"precipitation", "opad", "Niederschlag", "précipitations", "precipitación", "precipitazioni"}, "feels_like": {"feels like", "odczuwalna", "gefühlt", "ressenti", "sensación", "percepita"}, "precipitation_probability": {"precipitation probability", "prawdopodobieństwo opadu", "Niederschlagswahrscheinlichkeit", "probabilité de précipitation", "probabilidad de precipitación", "probabilità di precipitazioni"}}
	idx := map[string]int{"en": 0, "pl": 1, "de": 2, "fr": 3, "es": 4, "it": 5}[locale]
	name := o.Parameter
	if v, ok := labels[o.Parameter]; ok {
		name = v[idx]
	}
	if format == "compact" {
		return fmt.Sprintf("%.1f %s", o.Value, o.Unit)
	}
	return fmt.Sprintf("%s %.1f %s", name, o.Value, o.Unit)
}
func localized(locale string, v ...string) string {
	i := map[string]int{"en": 0, "pl": 1, "de": 2, "fr": 3, "es": 4, "it": 5}[locale]
	if i >= len(v) {
		i = 0
	}
	return v[i]
}
func estimate(obs []weather.Observation) map[string]any {
	now := time.Now()
	groups := make(map[string][]weather.Observation)
	for _, o := range obs {
		o = weather.Convert(o, "metric")
		age := now.Sub(o.ObservedAt)
		if o.Parameter == "pressure" && strings.EqualFold(o.Unit, "hPa") && o.PressureKind == "station" && o.Quality == "measured" && age >= 0 && age < 5*time.Hour {
			identity := o.DeviceID
			if identity == "" {
				identity = o.Station
			}
			key := o.Source + "|" + o.Station + "|" + identity
			groups[key] = append(groups[key], o)
		}
	}
	var first, last weather.Observation
	var samples []weather.Observation
	for _, group := range groups {
		sortObs(group)
		if len(group) < 2 || group[len(group)-1].ObservedAt.Sub(group[0].ObservedAt) < time.Hour {
			continue
		}
		if group[len(group)-1].ObservedAt.After(last.ObservedAt) {
			first, last, samples = group[0], group[len(group)-1], group
		}
	}
	if len(samples) == 0 {
		return map[string]any{"available": false, "reason": "Potrzeba co najmniej dwóch świeżych pomiarów tego samego źródła i stacji, oddzielonych o co najmniej godzinę."}
	}
	hours := last.ObservedAt.Sub(first.ObservedAt).Hours()
	trend := (last.Value - first.Value) / hours * 3
	summary := "stabilne"
	if trend > 1 {
		summary = "rosnące"
	} else if trend < -1 {
		summary = "spadające"
	}
	return map[string]any{"available": true, "label": "szacunek lokalny na podstawie trendu ciśnienia", "pressure_change_hpa_3h": trend, "trend": summary, "based_on": len(samples), "source": last.Source, "station": last.Station, "from": first.ObservedAt, "to": last.ObservedAt}
}
func sortObs(o []weather.Observation) {
	sort.SliceStable(o, func(i, j int) bool { return o[i].ObservedAt.Before(o[j].ObservedAt) })
}

func (a *App) warningFetchLoop(ctx context.Context) {
	if err := a.fetchWarnings(ctx); err != nil {
		log.Printf("weather warnings update: %v", err)
	}
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-a.warningRefresh:
		}
		if err := a.fetchWarnings(ctx); err != nil {
			log.Printf("weather warnings update: %v", err)
		}
	}
}

func (a *App) fetchWarnings(ctx context.Context) error {
	c := a.Config()
	if !c.WarningsEnabled {
		current, err := a.replaceWarningsForConfig(c, nil)
		if err != nil {
			return err
		}
		if !current {
			return nil
		}
		a.setWarningProviderHealthForConfig(c, newWarningProviderHealth("disabled", time.Now().UTC()))
		a.setWarningHealthForConfig(c, time.Now().UTC(), "")
		return nil
	}
	requestCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	now := time.Now().UTC()
	previous := a.history.Warnings()
	updatedBySource := make(map[string][]warnings.Alert, 3)
	failedSources := make(map[string]bool, 3)
	var providerErrors []string
	providerHealth := newWarningProviderHealth("not_applicable", now)
	type providerResult struct {
		source     string
		applicable bool
		alerts     []warnings.Alert
		err        error
	}
	results := make(chan providerResult, 3)
	providers := 0

	imgwApplicable := c.Latitude >= 49 && c.Latitude <= 55.1 && c.Longitude >= 14.1 && c.Longitude <= 24.2
	if imgwApplicable {
		providers++
		go func() {
			result := providerResult{source: "IMGW-PIB", applicable: true}
			code, err := a.warningCountyForLocation(requestCtx, c.Latitude, c.Longitude)
			if errors.Is(err, warnings.ErrCountyNotFound) {
				result.applicable = false
				results <- result
				return
			}
			if err != nil {
				result.err = fmt.Errorf("location lookup: %w", err)
				results <- result
				return
			}
			items, err := (warnings.IMGW{}).Fetch(requestCtx)
			if err != nil {
				result.err = err
				results <- result
				return
			}
			for _, item := range items {
				if warnings.MatchesArea(item, []string{code}) && item.ValidTo.After(now) {
					result.alerts = append(result.alerts, item)
				}
			}
			results <- result
		}()
	}

	dwdApplicable := c.Latitude >= 47.2 && c.Latitude <= 55.1 && c.Longitude >= 5.5 && c.Longitude <= 15.1
	if dwdApplicable {
		providers++
		go func() {
			items, err := (warnings.DWD{Locale: c.Locale}).Fetch(requestCtx, c.Latitude, c.Longitude)
			results <- providerResult{source: "DWD", applicable: true, alerts: activeWarnings(items, now), err: err}
		}()
	}
	geoSphereApplicable := c.Latitude >= 46.3 && c.Latitude <= 49.1 && c.Longitude >= 9.4 && c.Longitude <= 17.2
	if geoSphereApplicable {
		providers++
		go func() {
			items, err := (warnings.GeoSphere{Locale: c.Locale}).Fetch(requestCtx, c.Latitude, c.Longitude)
			result := providerResult{source: "GeoSphere Austria", applicable: true, alerts: activeWarnings(items, now), err: err}
			if errors.Is(err, warnings.ErrGeoSphereOutsideCoverage) {
				result.applicable = false
				result.err = nil
			}
			results <- result
		}()
	}
	// No country-specific poller may cover this station; in that case the
	// global MeteoAlarm stream remains the warning source. Otherwise require at
	// least one applicable poller to succeed before replacing stored alerts.
	providerSucceeded := providers == 0
	providerHealth[warnings.MeteoAlarmSource] = a.warningProviderHealthSnapshot()[warnings.MeteoAlarmSource]
	// MeteoAlarm is a continuously subscribed, global provider. Its current
	// state comes from the stream worker; the HTTP refresh must not replace it.
	for i := 0; i < providers; i++ {
		result := <-results
		switch result.source {
		case "IMGW-PIB":
			imgwApplicable = result.applicable
		case "DWD":
			dwdApplicable = result.applicable
		case "GeoSphere Austria":
			geoSphereApplicable = result.applicable
		}
		if result.err != nil {
			failedSources[result.source] = true
			providerErrors = append(providerErrors, result.source+": "+result.err.Error())
			providerHealth[result.source] = warningProviderHealth{State: "error", CheckedAt: now, Error: result.err.Error()}
			continue
		}
		if result.applicable {
			providerSucceeded = true
			providerHealth[result.source] = warningProviderHealth{State: "ok", CheckedAt: now, AlertCount: len(result.alerts)}
		} else {
			providerHealth[result.source] = warningProviderHealth{State: "not_applicable", CheckedAt: now}
		}
		for _, item := range result.alerts {
			updatedBySource[item.Source] = append(updatedBySource[item.Source], item)
		}
	}
	if !a.warningConfigCurrent(c) {
		return nil
	}
	a.setWarningProviderHealthForConfig(c, providerHealth)
	if !imgwApplicable && !dwdApplicable && !geoSphereApplicable {
		// Country-specific adapters may reject a point inside their broad
		// bounding box. In that case, retain the global MeteoAlarm stream and
		// remove any local alerts that no longer apply to this station.
		current, err := a.reconcilePolledWarningsForConfig(c, nil)
		if err != nil {
			a.setWarningHealthForConfig(c, time.Time{}, err.Error())
			return err
		}
		if !current {
			return nil
		}
		a.setWarningHealthForConfig(c, now, "")
		return nil
	}
	if !providerSucceeded {
		message := "all applicable warning providers failed: " + strings.Join(providerErrors, "; ")
		err := errors.New(message)
		a.setWarningHealthForConfig(c, time.Time{}, err.Error())
		return err
	}
	merged := make([]warnings.Alert, 0, len(previous))
	for _, item := range previous {
		if !failedSources[item.Source] {
			continue
		}
		if item.ValidTo.After(now) {
			merged = append(merged, item)
		}
	}
	for _, items := range updatedBySource {
		merged = append(merged, items...)
	}
	current, err := a.reconcilePolledWarningsForConfig(c, merged)
	if err != nil {
		a.setWarningHealthForConfig(c, time.Time{}, err.Error())
		return err
	}
	if !current {
		return nil
	}
	a.setWarningHealthForConfig(c, now, strings.Join(providerErrors, "; "))
	return nil
}

func activeWarnings(items []warnings.Alert, now time.Time) []warnings.Alert {
	active := make([]warnings.Alert, 0, len(items))
	for _, item := range items {
		if item.ValidTo.After(now) {
			active = append(active, item)
		}
	}
	return active
}

func (a *App) meteoAlarmLoop(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}
		if !a.Config().WarningsEnabled {
			a.warningMu.Lock()
			if a.warningProviders == nil {
				a.warningProviders = newWarningProviderHealth("waiting", time.Time{})
			}
			a.warningProviders[warnings.MeteoAlarmSource] = warningProviderHealth{State: "disabled", CheckedAt: time.Now().UTC()}
			a.warningMu.Unlock()
			select {
			case <-ctx.Done():
				return
			case <-time.After(10 * time.Second):
			}
			continue
		}
		watchCtx, cancel := context.WithCancel(ctx)
		monitorDone := make(chan struct{})
		go func() {
			defer close(monitorDone)
			ticker := time.NewTicker(5 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-watchCtx.Done():
					return
				case <-ticker.C:
					if !a.Config().WarningsEnabled {
						cancel()
						return
					}
				}
			}
		}()
		(warnings.MeteoAlarm{}).Watch(watchCtx, func(topic string, payload []byte) error {
			if !warnings.IsMeteoAlarmTopic(topic) {
				return nil
			}
			c := a.Config()
			if !c.WarningsEnabled {
				return nil
			}
			requestCtx, requestCancel := context.WithTimeout(watchCtx, 30*time.Second)
			result, err := (warnings.MeteoAlarm{}).ParseNotification(requestCtx, payload, c.Latitude, c.Longitude, c.Locale)
			requestCancel()
			if err != nil {
				a.setMeteoAlarmHealth(err)
				log.Printf("MeteoAlarm notification: %v", err)
				return nil
			}
			current, err := a.applyMeteoAlarmResult(c, result)
			if err != nil {
				a.setMeteoAlarmHealth(err)
				log.Printf("save MeteoAlarm warning: %v", err)
				return nil
			}
			if current {
				a.setMeteoAlarmHealth(nil)
			}
			return nil
		}, func(err error) {
			if !a.Config().WarningsEnabled {
				a.warningMu.Lock()
				if a.warningProviders == nil {
					a.warningProviders = newWarningProviderHealth("waiting", time.Time{})
				}
				a.warningProviders[warnings.MeteoAlarmSource] = warningProviderHealth{State: "disabled", CheckedAt: time.Now().UTC()}
				a.warningMu.Unlock()
				return
			}
			if err == nil {
				a.setMeteoAlarmConnected()
				return
			}
			a.setMeteoAlarmHealth(err)
		})
		cancel()
		<-monitorDone
	}
}

func (a *App) warningCountyForLocation(ctx context.Context, latitude, longitude float64) (string, error) {
	now := time.Now()
	a.warningMu.RLock()
	if a.warningCounty != "" && a.warningCountyLat == latitude && a.warningCountyLon == longitude && now.Sub(a.warningCountyAt) < 24*time.Hour {
		code := a.warningCounty
		a.warningMu.RUnlock()
		return code, nil
	}
	a.warningMu.RUnlock()

	code, err := warnings.ResolveCountyTERYT(ctx, latitude, longitude)
	if err != nil {
		return "", err
	}
	a.warningMu.Lock()
	a.warningCounty = code
	a.warningCountyLat = latitude
	a.warningCountyLon = longitude
	a.warningCountyAt = now
	a.warningMu.Unlock()
	return code, nil
}

func (a *App) setWarningHealth(updated time.Time, message string) {
	a.warningMu.Lock()
	if !updated.IsZero() {
		a.warningUpdatedAt = updated
	}
	a.warningError = message
	a.warningMu.Unlock()
}

func (a *App) warningSpeakLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		c := a.Config()
		if !c.WarningsEnabled || c.TTSVoice == "" {
			continue
		}
		now := time.Now().UTC()
		for _, item := range a.history.Warnings() {
			if now.Before(item.ValidFrom) || !now.Before(item.ValidTo) {
				continue
			}
			id := warningAnnouncementKey(item)
			signature := warningSignature(item)
			previous, exists := a.history.WarningAnnouncement(id)
			if !exists && id != item.ID {
				previous, exists = a.history.WarningAnnouncement(item.ID)
				if exists {
					if err := a.history.SetWarningAnnouncement(id, previous); err != nil {
						log.Printf("migrate warning announcement state: %v", err)
					}
				}
			}
			interval := warningRepeatInterval(c, item.Severity)
			if !warningAnnouncementDue(now, interval, signature, previous, exists) {
				continue
			}
			text := warningSpeech(item, c.Locale, c.Timezone)
			a.speechMu.Lock()
			speechCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
			err := audio.Speak(speechCtx, text, c.TTSVoice, c.TTSBinary, c.AudioOutput, c.AudioVolume)
			cancel()
			a.speechMu.Unlock()
			result := "played"
			state := previous
			state.LastAttemptSignature = signature
			state.LastAttemptAt = now
			if err != nil {
				result = err.Error()
				log.Printf("scheduled warning audio: %v", err)
			} else {
				state.Signature = signature
				state.At = now
			}
			if err := a.history.SetWarningAnnouncement(id, state); err != nil {
				log.Printf("save warning announcement state: %v", err)
			}
			_ = a.history.AddPlayback(store.Playback{At: now, Text: text, Result: result})
		}
	}
}

func warningRepeatInterval(c config.Config, severity int) time.Duration {
	minutes := c.WarningRepeat1
	if severity == 2 {
		minutes = c.WarningRepeat2
	} else if severity >= 3 {
		minutes = c.WarningRepeat3
	}
	return time.Duration(minutes) * time.Minute
}

func warningAnnouncementDue(now time.Time, interval time.Duration, signature string, previous store.WarningAnnouncement, exists bool) bool {
	if exists && previous.Signature == signature && !previous.At.IsZero() && now.Sub(previous.At) < interval {
		return false
	}
	if exists && previous.LastAttemptSignature == signature && !previous.LastAttemptAt.IsZero() && now.Sub(previous.LastAttemptAt) < 5*time.Minute {
		return false
	}
	return true
}

func warningSignature(item warnings.Alert) string {
	probability := ""
	if item.Probability != nil {
		probability = strconv.Itoa(*item.Probability)
	}
	contentLanguage, _, _ := strings.Cut(strings.ToLower(strings.TrimSpace(item.ContentLanguage)), "-")
	value := fmt.Sprintf("%s|%d|%s|%s|%s|%s|%s|%s|%s|%v", item.Event, item.Severity, item.SeverityLabel, probability, item.ValidFrom.UTC().Format(time.RFC3339), item.ValidTo.UTC().Format(time.RFC3339), contentLanguage, item.Description, item.Area, item.AreaCodes)
	return fmt.Sprintf("%x", sha256.Sum256([]byte(value)))
}

func warningAnnouncementKey(item warnings.Alert) string {
	return strings.ToUpper(strings.TrimSpace(item.Source)) + "|" + item.ID
}

func warningSpeech(item warnings.Alert, locale, timezone string) string {
	locale, _, _ = strings.Cut(strings.ToLower(strings.TrimSpace(locale)), "-")
	if locale == "" {
		locale = "pl"
	}
	level := fmt.Sprintf("%d", item.Severity)
	location := stationLocation(timezone)
	when := item.ValidFrom.In(location).Format("02.01.2006 15:04") + " – " + item.ValidTo.In(location).Format("02.01.2006 15:04")
	provider := "IMGW"
	if item.Source == "DWD" || item.Source == "GeoSphere Austria" {
		provider = item.Source
	} else if item.Source == warnings.MeteoAlarmSource {
		provider = "MeteoAlarm"
	}
	heading := localized(locale,
		provider+" weather warning, level ",
		"Ostrzeżenie "+provider+", stopień ",
		provider+"-Wetterwarnung, Stufe ",
		"Alerte météo "+provider+", niveau ",
		"Aviso meteorológico de "+provider+", nivel ",
		"Avviso meteorologico "+provider+", livello ",
	)
	validity := localized(locale, "Valid from ", "Obowiązuje od ", "Gültig von ", "Valable du ", "Válido desde ", "Valido da ")
	text := heading + level + ". " + item.Event + ". " + validity + when + "."
	if item.Probability != nil {
		probability := localized(locale, " Probability: ", " Prawdopodobieństwo: ", " Wahrscheinlichkeit: ", " Probabilité : ", " Probabilidad: ", " Probabilità: ")
		percent := localized(locale, " percent.", " procent.", " Prozent.", " pour cent.", " por ciento.", " per cento.")
		text += probability + fmt.Sprintf("%d", *item.Probability) + percent
	}
	contentLanguage, _, _ := strings.Cut(strings.ToLower(strings.TrimSpace(item.ContentLanguage)), "-")
	if item.Description != "" && contentLanguage == locale {
		text += " " + item.Description
	}
	return text
}

func stationLocation(name string) *time.Location {
	if strings.TrimSpace(name) == "" {
		name = config.DefaultTimezone
	}
	location, err := time.LoadLocation(name)
	if err != nil {
		return time.UTC
	}
	return location
}

func urlQuery(s string) string { return url.QueryEscape(s) }
func runtimeArch() string      { return runtime.GOARCH }

func (a *App) scheduleLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	done := map[string]bool{}
	checkedDate := ""
	for {
		location := stationLocation(a.Config().Timezone)
		now := time.Now().In(location)
		date := now.Format("2006-01-02")
		if date != checkedDate {
			done = map[string]bool{}
			checkedDate = date
		}
		c := a.Config()
		for _, s := range c.Schedules {
			if !s.Enabled {
				continue
			}
			matches := false
			scheduleKey := s.Time
			if s.EveryHour {
				matches = now.Minute() == s.OffsetMinutes
				scheduleKey = fmt.Sprintf("hourly:%02d", s.OffsetMinutes)
			} else {
				matches = s.Time == now.Format("15:04")
			}
			if !matches {
				continue
			}
			key := now.Format("2006-01-02 15:") + fmt.Sprintf("%02d ", now.Minute()) + scheduleKey
			if done[key] {
				continue
			}
			done[key] = true
			text := a.buildAnnouncement()
			a.speechMu.Lock()
			speechCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
			e := audio.Speak(speechCtx, text, c.TTSVoice, c.TTSBinary, c.AudioOutput, c.AudioVolume)
			cancel()
			a.speechMu.Unlock()
			res := "played"
			if e != nil {
				res = e.Error()
				log.Printf("scheduled audio: %v", e)
			}
			_ = a.history.AddPlayback(store.Playback{At: now.UTC(), Text: text, Result: res})
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
func (a *App) serialLoop(ctx context.Context) {
	for {
		c := a.Config()
		if c.SerialPort != "" {
			a.markSourceAttempt("serial")
			_ = exec.CommandContext(ctx, "stty", "-F", c.SerialPort, strconv.Itoa(c.SerialBaud), "raw", "-echo").Run()
			f, e := os.Open(c.SerialPort)
			if e == nil {
				scanErr := scanSerialLines(ctx, f, func(line []byte) {
					items, er := weather.DecodeSerialJSON(line)
					if er == nil {
						accepted := true
						for _, o := range items {
							if saveErr := a.saveSensorObservation(o); saveErr != nil {
								accepted = false
								log.Printf("serial sample: %v", saveErr)
							}
						}
						if accepted && len(items) > 0 {
							a.markSourceSuccess("serial")
						} else if !accepted {
							a.markSourceError("serial", "Odrzucono pomiar poza obsługiwanym zakresem")
						}
					} else {
						a.markSourceError("serial", "Odrzucono próbkę z nieprawidłowymi danymi")
					}
				})
				if ctx.Err() != nil {
					return
				}
				if scanErr != nil {
					a.markSourceError("serial", "Błąd odczytu portu szeregowego")
				} else {
					a.markSourceError("serial", "Port szeregowy zakończył strumień danych")
				}
			} else {
				if ctx.Err() != nil {
					return
				}
				a.markSourceError("serial", "Nie można otworzyć portu szeregowego")
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(10 * time.Second):
		}
	}
}
func (a *App) aprsLoop(ctx context.Context) {
	for {
		c := a.Config()
		if c.APRSHost != "" {
			addr := net.JoinHostPort(c.APRSHost, strconv.Itoa(c.APRSPort))
			a.markSourceAttempt("aprs")
			conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
			if err != nil {
				a.setAPRSTCPConnected(false)
				a.markSourceError("aprs", "Nie można połączyć się z serwerem KISS")
			} else {
				a.setAPRSTCPConnected(true)
				a.recordAPRSProcessingError(nil)
				var kissDecoder aprs.KISSStreamDecoder
				buf := make([]byte, 1024)
				for {
					_ = conn.SetReadDeadline(time.Now().Add(90 * time.Second))
					n, readErr := conn.Read(buf)
					if n > 0 {
						a.recordAPRSBytes(n)
						for _, packet := range kissDecoder.Feed(buf[:n]) {
							a.recordAPRSKISSFrame()
							line, decodeErr := aprs.AX25ToTNC2(packet)
							if decodeErr != nil {
								a.recordAPRSProcessingError(decodeErr)
								continue
							}
							a.recordAPRSAx25Frame()
							a.recordAPRSProcessingError(nil)
							a.markSourceSuccess("aprs")
							cfg := a.Config()
							parsed, parseErr := aprs.ParseTNC2(line, time.Now(), cfg.Latitude, cfg.Longitude, cfg.APRSRadiusKM)
							if parseErr != nil {
								a.recordAPRSProcessingError(parseErr)
								continue
							}
							parsed = aprs.ApplyBlacklist(parsed, cfg.APRSBlacklist)
							parsed = aprs.ApplyFrameBlacklist(parsed, cfg.APRSFrameBlacklist)
							if storeErr := a.history.AddFrame(parsed); storeErr == nil {
								a.recordAPRSStoredFrame()
								if parsed.Eligible {
									for _, observation := range parsed.Data {
										_ = a.history.AddObservation(observation)
									}
								}
							} else {
								a.recordAPRSProcessingError(storeErr)
							}
						}
					}
					if readErr != nil {
						a.setAPRSTCPConnected(false)
						if ctx.Err() == nil {
							a.markSourceError("aprs", "Połączenie z serwerem KISS zostało przerwane")
						}
						break
					}
				}
				_ = conn.Close()
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
			}
		} else {
			a.setAPRSTCPConnected(false)
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
			}
		}
	}
}
func (a *App) haLoop(ctx context.Context) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for {
		c := a.Config()
		if c.HAURL != "" && c.HAToken != "" {
			a.fetchHA(ctx, c)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
func (a *App) fetchHA(ctx context.Context, c config.Config) {
	a.markSourceAttempt("home_assistant")
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, e := http.NewRequestWithContext(ctx, "GET", strings.TrimRight(c.HAURL, "/")+"/api/states", nil)
	if e != nil {
		a.markSourceError("home_assistant", "Nie można utworzyć żądania do Home Assistant")
		return
	}
	req.Header.Set("Authorization", "Bearer "+c.HAToken)
	resp, e := http.DefaultClient.Do(req)
	if e != nil {
		a.markSourceError("home_assistant", "Nie można połączyć się z Home Assistant")
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		a.markSourceError("home_assistant", fmt.Sprintf("Home Assistant zwrócił HTTP %d", resp.StatusCode))
		return
	}
	var states []struct {
		EntityID    string         `json:"entity_id"`
		State       string         `json:"state"`
		LastUpdated time.Time      `json:"last_updated"`
		LastChanged time.Time      `json:"last_changed"`
		Attributes  map[string]any `json:"attributes"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&states) != nil {
		a.markSourceError("home_assistant", "Nieprawidłowa odpowiedź Home Assistant")
		return
	}
	a.markSourceSuccess("home_assistant")
	latestByDevice := a.history.LatestByDevice(nil)
	latestByEntity := make(map[string]weather.Observation, len(latestByDevice))
	for _, observation := range latestByDevice {
		if strings.HasPrefix(observation.Source, "home-assistant:") {
			latestByEntity[observation.DeviceID+"|"+observation.Parameter] = observation
		}
	}
	for _, s := range states {
		if !strings.HasPrefix(s.EntityID, "sensor.") {
			continue
		}
		v, e := strconv.ParseFloat(s.State, 64)
		if e != nil {
			continue
		}
		unit, _ := s.Attributes["unit_of_measurement"].(string)
		param := haParam(s.Attributes)
		if param == "" {
			continue
		}
		observedAt := s.LastUpdated
		if observedAt.IsZero() {
			observedAt = s.LastChanged
		}
		if observedAt.IsZero() {
			continue
		}
		if observedAt.After(time.Now().UTC()) {
			continue
		}
		if math.IsNaN(v) || math.IsInf(v, 0) {
			continue
		}
		if previous, ok := latestByEntity[s.EntityID+"|"+param]; ok && previous.ObservedAt.Equal(observedAt) && previous.Value == v && previous.Unit == unit {
			metadataUnchanged := true
			if param == "pressure" {
				device := c.SensorDevices[sensorDeviceConfigKey("home-assistant:"+s.EntityID, s.EntityID)]
				expectedAltitude := device.AltitudeM
				if expectedAltitude == nil && device.PressureKind == "station" {
					stationAltitude := c.AltitudeM
					expectedAltitude = &stationAltitude
				}
				altitudeUnchanged := previous.SensorAltitudeM == nil && expectedAltitude == nil ||
					previous.SensorAltitudeM != nil && expectedAltitude != nil && *previous.SensorAltitudeM == *expectedAltitude
				metadataUnchanged = previous.PressureKind == device.PressureKind && altitudeUnchanged
			}
			if metadataUnchanged {
				continue
			}
		}
		deviceName, _ := s.Attributes["friendly_name"].(string)
		deviceName = strings.TrimSpace(deviceName)
		if deviceName == "" {
			deviceName = s.EntityID
		}
		if err := a.saveSensorObservation(weather.Observation{Parameter: param, Value: v, Unit: unit, ObservedAt: observedAt.UTC(), Source: "home-assistant:" + s.EntityID, DeviceID: s.EntityID, DeviceName: deviceName, Quality: "measured"}); err != nil {
			log.Printf("Home Assistant sensor %s: %v", s.EntityID, err)
			a.markSourceError("home_assistant", "Odrzucono odczyt czujnika poza obsługiwanym zakresem")
		}
	}
}
func haParam(a map[string]any) string {
	for _, k := range []string{"device_class", "state_class"} {
		v, _ := a[k].(string)
		switch v {
		case "temperature":
			return "temperature"
		case "humidity":
			return "humidity"
		case "pressure":
			return "pressure"
		case "wind_speed":
			return "wind_speed"
		case "wind_direction":
			return "wind_direction"
		case "precipitation":
			return "precipitation"
		}
	}
	return ""
}
func (a *App) mqttLoop(ctx context.Context) {
	for {
		c := a.Config()
		if c.MQTTBroker != "" {
			a.readMQTT(ctx, c)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}

func (a *App) bmeLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		c := a.Config()
		if c.BMEEnabled {
			temp, humidity, pressure, err := bme280.Read(c.I2CDevice, uint16(c.I2CAddress))
			if err != nil {
				a.bmeStatusMu.Lock()
				a.bmeLastError = err.Error()
				a.bmeStatusMu.Unlock()
				log.Printf("BME280: %v", err)
			} else {
				at := time.Now().UTC()
				deviceID := fmt.Sprintf("bme280:%s:%#x", c.I2CDevice, c.I2CAddress)
				common := weather.Observation{ObservedAt: at, Source: "bme280", DeviceID: deviceID, DeviceName: "BME280", Quality: "measured"}
				var measurementErr error
				temperature := common
				temperature.Parameter, temperature.Value, temperature.Unit = "temperature", temp, "°C"
				measurementErr = a.saveSensorObservation(temperature)
				humidityReading := common
				humidityReading.Parameter, humidityReading.Value, humidityReading.Unit = "humidity", humidity, "%"
				if err := a.saveSensorObservation(humidityReading); err != nil && measurementErr == nil {
					measurementErr = err
				}
				pressureReading := common
				pressureReading.Parameter, pressureReading.Value, pressureReading.Unit, pressureReading.PressureKind = "pressure", pressure, "hPa", "station"
				if err := a.saveSensorObservation(pressureReading); err != nil && measurementErr == nil {
					measurementErr = err
				}
				a.bmeStatusMu.Lock()
				if measurementErr != nil {
					a.bmeLastError = measurementErr.Error()
					log.Printf("BME280 sensor measurement: %v", measurementErr)
				} else {
					a.bmeLastError = ""
				}
				a.bmeStatusMu.Unlock()
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (a *App) maintenanceLoop(ctx context.Context) {
	ticker := time.NewTicker(6 * time.Hour)
	defer ticker.Stop()
	for {
		if err := a.history.Prune(a.Config().HistoryDays); err != nil {
			log.Printf("prune history: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
func (a *App) readMQTT(ctx context.Context, c config.Config) {
	a.markSourceAttempt("mqtt")
	host, _ := os.Hostname()
	conn, e := mqtt.Connect(c.MQTTBroker, "ultimatewx-"+host, c.MQTTUsername, c.MQTTPassword, c.MQTTTopic)
	if e != nil {
		a.markSourceError("mqtt", "Nie można połączyć się z brokerem MQTT")
		log.Printf("MQTT: %v", e)
		select {
		case <-ctx.Done():
		case <-time.After(5 * time.Second):
		}
		return
	}
	defer conn.Close()
	a.markSourceSuccess("mqtt")
	log.Printf("MQTT connected: %s", c.MQTTBroker)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		msg, e := mqtt.Read(conn)
		if e != nil {
			if ne, ok := e.(net.Error); ok && ne.Timeout() {
				_ = mqtt.Ping(conn)
				continue
			}
			log.Printf("MQTT read: %v", e)
			a.markSourceError("mqtt", "Błąd odczytu z brokera MQTT")
			return
		}
		if len(msg.Payload) == 0 {
			continue
		}
		items, e := weather.DecodeSerialJSON(msg.Payload)
		if e != nil {
			log.Printf("MQTT sample %s: %v", msg.Topic, e)
			a.markSourceError("mqtt", "Odrzucono próbkę MQTT z nieprawidłowymi danymi")
			continue
		}
		a.markSourceSuccess("mqtt")
		accepted := true
		for _, o := range items {
			o.Source = "mqtt:" + msg.Topic
			if saveErr := a.saveSensorObservation(o); saveErr != nil {
				accepted = false
				log.Printf("MQTT sensor sample %s: %v", msg.Topic, saveErr)
			}
		}
		if !accepted {
			a.markSourceError("mqtt", "Odrzucono pomiar poza obsługiwanym zakresem")
		}
	}
}

func sensorDeviceConfigKey(source, deviceID string) string {
	if deviceID == "" {
		return ""
	}
	kind := strings.SplitN(source, ":", 2)[0]
	if kind == "bme280" && strings.HasPrefix(deviceID, "bme280:") {
		return deviceID
	}
	return kind + ":" + deviceID
}

func filterExcludedSensorObservations(observations []weather.Observation, devices map[string]config.SensorDeviceConfig) []weather.Observation {
	if len(devices) == 0 || len(observations) == 0 {
		return observations
	}
	filtered := make([]weather.Observation, 0, len(observations))
	for _, observation := range observations {
		device, ok := devices[sensorDeviceConfigKey(observation.Source, observation.DeviceID)]
		if ok && containsSensorParameter(device.ExcludedParameters, observation.Parameter) {
			continue
		}
		filtered = append(filtered, observation)
	}
	return filtered
}

func containsSensorParameter(parameters []string, parameter string) bool {
	for _, candidate := range parameters {
		if candidate == parameter {
			return true
		}
	}
	return false
}

func latestByParameterSource(observations []weather.Observation) []weather.Observation {
	latest := make(map[string]weather.Observation, len(observations))
	for _, observation := range observations {
		key := observation.Parameter + "|" + observation.Source
		if current, ok := latest[key]; !ok || observation.ObservedAt.After(current.ObservedAt) {
			latest[key] = observation
		}
	}
	out := make([]weather.Observation, 0, len(latest))
	for _, observation := range latest {
		out = append(out, observation)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Parameter != out[j].Parameter {
			return out[i].Parameter < out[j].Parameter
		}
		return out[i].Source < out[j].Source
	})
	return out
}

func (a *App) saveSensorObservation(o weather.Observation) error {
	c := a.Config()
	if device, ok := c.SensorDevices[sensorDeviceConfigKey(o.Source, o.DeviceID)]; ok {
		if name := strings.TrimSpace(device.Name); name != "" {
			o.DeviceName = name
		}
		if o.SensorAltitudeM == nil {
			o.SensorAltitudeM = device.AltitudeM
		}
		if o.Parameter == "pressure" && device.PressureKind != "" {
			o.PressureKind = device.PressureKind
		}
	}
	if o.Parameter == "pressure" {
		o = weather.Convert(o, "metric")
	}
	if err := weather.ValidateSensorObservation(o); err != nil {
		return fmt.Errorf("reject %s observation from %s: %w", o.Parameter, o.Source, err)
	}
	if o.SensorAltitudeM == nil {
		switch {
		case strings.HasPrefix(o.Source, "mqtt:"):
			o.SensorAltitudeM = c.MQTTSensorAltitudeM
		case strings.HasPrefix(o.Source, "serial:"):
			o.SensorAltitudeM = c.SerialSensorAltitudeM
		case strings.HasPrefix(o.Source, "home-assistant:") && o.PressureKind == "station":
			altitude := c.AltitudeM
			o.SensorAltitudeM = &altitude
		case o.Source == "bme280":
			altitude := c.AltitudeM
			o.SensorAltitudeM = &altitude
		}
	}
	if err := a.history.AddObservation(o); err != nil {
		return fmt.Errorf("save sensor observation: %w", err)
	}
	if o.Parameter != "pressure" || !strings.EqualFold(o.Unit, "hPa") || o.PressureKind != "station" || o.Quality != "measured" || o.SensorAltitudeM == nil {
		return nil
	}
	altitude := *o.SensorAltitudeM
	if math.IsNaN(altitude) || math.IsInf(altitude, 0) || altitude < -500 || altitude > 9000 || o.Value <= 0 {
		return nil
	}
	corrected := o.Value / math.Pow(1-altitude/44330, 5.255)
	derived := o
	derived.Parameter = "pressure_msl"
	derived.Value = corrected
	derived.Quality = "derived"
	derived.PressureKind = "sea_level"
	derived.SensorAltitudeM = nil
	if err := a.history.AddObservation(derived); err != nil {
		return fmt.Errorf("save derived sensor pressure: %w", err)
	}
	return nil
}

var startedAt = time.Now()

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
func method(w http.ResponseWriter) { writeError(w, http.StatusMethodNotAllowed, "method not allowed") }
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		writeError(w, 400, e.Error())
		return false
	}
	return true
}
