package app

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
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
	"github.com/ultimatewx/ultimatewx/internal/weather"
)

type App struct {
	configPath, dataDir string
	cfg                 config.Config
	cfgMu               sync.RWMutex
	history             *store.Store
	voices              *tts.Service
	snapshot            weather.Snapshot
	snapMu              sync.RWMutex
	sessions            map[string]time.Time
	sessionMu           sync.Mutex
	mux                 *http.ServeMux
	restart             bool
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
	a := &App{configPath: configPath, dataDir: dataDir, cfg: c, history: s, voices: tts.New(filepath.Join(dataDir, "voices")), sessions: map[string]time.Time{}, mux: http.NewServeMux()}
	a.routes()
	return a, nil
}
func (a *App) Config() config.Config { a.cfgMu.RLock(); defer a.cfgMu.RUnlock(); return a.cfg }
func (a *App) setConfig(c config.Config) error {
	if e := config.Validate(c); e != nil {
		return e
	}
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	if e := config.Save(a.configPath, c); e != nil {
		return e
	}
	a.cfg = c
	return nil
}
func (a *App) Handler() http.Handler { return a.security(a.mux) }
func (a *App) Run(ctx context.Context) {
	go a.weatherLoop(ctx)
	go a.scheduleLoop(ctx)
	go a.serialLoop(ctx)
	go a.aprsLoop(ctx)
	go a.haLoop(ctx)
	go a.mqttLoop(ctx)
	go a.bmeLoop(ctx)
	go a.maintenanceLoop(ctx)
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
	a.mux.HandleFunc("/api/setup", a.setup)
	a.mux.HandleFunc("/api/login", a.login)
	a.mux.HandleFunc("/api/logout", a.logout)
	a.mux.HandleFunc("/api/status", a.protect(a.status))
	a.mux.HandleFunc("/api/config", a.setupAccess(a.configuration))
	a.mux.HandleFunc("/api/weather", a.protect(a.weatherStatus))
	a.mux.HandleFunc("/api/stats", a.protect(a.stats))
	a.mux.HandleFunc("/api/aprs", a.protect(a.aprsFrames))
	a.mux.HandleFunc("/api/aprs/test", a.setupAccess(a.testAPRSConnection))
	a.mux.HandleFunc("/api/audio", a.setupAccess(a.audioDevices))
	a.mux.HandleFunc("/api/announcement/preview", a.protect(a.preview))
	a.mux.HandleFunc("/api/announcement/play", a.protect(a.play))
	a.mux.HandleFunc("/api/location/search", a.setupAccess(a.searchLocation))
	a.mux.HandleFunc("/api/update", a.protect(a.update))
	a.mux.HandleFunc("/api/tts/catalog", a.protect(a.ttsCatalog))
	a.mux.HandleFunc("/api/tts/installed", a.protect(a.ttsInstalled))
	a.mux.HandleFunc("/api/tts/download", a.protect(a.ttsDownload))
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
		c := a.Config()
		if c.PasswordHash == "" && !c.AuthDisabled {
			next(w, r)
			return
		}
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
func (a *App) newSession(w http.ResponseWriter) {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		http.Error(w, "session error", 500)
		return
	}
	token := base64.RawURLEncoding.EncodeToString(b)
	a.sessionMu.Lock()
	a.sessions[token] = time.Now().Add(12 * time.Hour)
	a.sessionMu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: "ultimatewx_session", Value: token, Path: "/", HttpOnly: true, Secure: rIsTLS(w), SameSite: http.SameSiteStrictMode, MaxAge: 43200})
}
func rIsTLS(w http.ResponseWriter) bool { return false }
func (a *App) authState(w http.ResponseWriter, r *http.Request) {
	c := a.Config()
	writeJSON(w, 200, map[string]any{"setup_required": c.PasswordHash == "" && !c.AuthDisabled, "authentication_disabled": c.AuthDisabled})
}
func (a *App) setup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		method(w)
		return
	}
	c := a.Config()
	if c.PasswordHash != "" || c.AuthDisabled {
		writeError(w, 409, "setup is already complete")
		return
	}
	var req struct {
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
	if req.DisableAuth {
		c.AuthDisabled = true
		if e := a.setConfig(c); e != nil {
			writeError(w, 400, e.Error())
			return
		}
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
	a.newSession(w)
	writeJSON(w, 200, map[string]any{"ok": true})
}
func (a *App) login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		method(w)
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if a.Config().AuthDisabled || !config.VerifyPassword(a.Config().PasswordHash, req.Password) {
		writeError(w, 401, "invalid password")
		return
	}
	a.newSession(w)
	writeJSON(w, 200, map[string]any{"ok": true})
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
func (a *App) status(w http.ResponseWriter, r *http.Request) {
	c := a.Config()
	a.snapMu.RLock()
	s := a.snapshot
	a.snapMu.RUnlock()
	writeJSON(w, 200, map[string]any{"version": updater.Version, "location": c.LocationName, "weather_updated_at": s.UpdatedAt, "weather_error": s.Error, "sources": map[string]any{"open_meteo": s.UpdatedAt.IsZero() || time.Since(s.UpdatedAt) > 30*time.Minute, "aprs": c.APRSHost == "", "mqtt": c.MQTTBroker == "", "home_assistant": c.HAURL == "", "serial": c.SerialPort == "", "bme280": !c.BMEEnabled}, "tts_configured": c.TTSVoice != "", "audio": audio.List(), "uptime_seconds": int64(time.Since(startedAt).Seconds())})
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
func (a *App) weatherStatus(w http.ResponseWriter, r *http.Request) {
	a.snapMu.RLock()
	s := a.snapshot
	a.snapMu.RUnlock()
	days := a.Config().HistoryDays
	writeJSON(w, 200, map[string]any{"snapshot": s, "latest": a.history.Latest(), "local_estimate": estimate(a.history.Observations(days))})
}
func (a *App) stats(w http.ResponseWriter, r *http.Request) {
	a.history.Prune(a.Config().HistoryDays)
	writeJSON(w, 200, a.history.Stats())
}
func (a *App) aprsFrames(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 500 {
		limit = 100
	}
	wx := r.URL.Query().Get("wx_only") == "true"
	writeJSON(w, 200, map[string]any{"frames": a.history.Frames(limit, wx)})
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
func (a *App) audioDevices(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, audio.List()) }
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
	err := audio.Speak(text, c.TTSVoice, c.TTSBinary, c.AudioOutput)
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
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeError(w, 400, "query required")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	endpoint := "https://geocoding-api.open-meteo.com/v1/search?name=" + urlQuery(q) + "&count=8&format=json"
	if language := a.Config().Locale; language != "" {
		endpoint += "&language=" + urlQuery(language)
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	resp, e := http.DefaultClient.Do(req)
	if e != nil {
		writeError(w, 502, e.Error())
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", "application/json")
	io.Copy(w, io.LimitReader(resp.Body, 1<<20))
}
func (a *App) update(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		writeJSON(w, 200, updater.Check(ctx, "linux", runtimeArch()))
	case http.MethodPost:
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Minute)
		defer cancel()
		if e := updater.Apply(ctx, "linux", runtimeArch()); e != nil {
			writeError(w, 400, e.Error())
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true, "restarting": true})
		if updater.Version != "dev" {
			go func() { time.Sleep(1500 * time.Millisecond); os.Exit(0) }()
		}
	default:
		method(w)
	}
}

func (a *App) buildAnnouncement() string {
	c := a.Config()
	latest := a.history.Latest()
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
			out = append(out, time.Now().Format("2006-01-02 15:04"))
		case "weather", "forecast":
			prefix := ""
			if b.Kind == "forecast" {
				prefix = "forecast_"
			}
			for _, field := range b.Fields {
				o, ok := selectObservation(byKey, prefix+field, b.Source)
				if ok {
					out = append(out, renderObservation(weather.Convert(o, c.Units), c.Locale))
				}
			}
		case "local-estimate":
			if estimateText := estimateSpeech(estimate(a.history.Observations(c.HistoryDays)), c.Locale); estimateText != "" {
				out = append(out, estimateText)
			}
		}
	}
	return strings.Join(out, " ")
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
	if source != "" && source != "auto" {
		o, ok := m[key+"|"+source]
		if ok && time.Since(o.ObservedAt) < 2*time.Hour {
			return o, true
		}
		return weather.Observation{}, false
	}
	var best weather.Observation
	ok := false
	for k, o := range m {
		if strings.HasPrefix(k, key+"|") && time.Since(o.ObservedAt) < 2*time.Hour && (!ok || o.ObservedAt.After(best.ObservedAt)) {
			best, ok = o, true
		}
	}
	return best, ok
}
func renderObservation(o weather.Observation, locale string) string {
	labels := map[string][6]string{"temperature": {"temperature", "temperatura", "Temperatur", "température", "temperatura", "temperatura"}, "humidity": {"humidity", "wilgotność", "Luftfeuchtigkeit", "humidité", "humedad", "umidità"}, "pressure": {"pressure", "ciśnienie", "Luftdruck", "pression", "presión", "pressione"}, "wind_speed": {"wind speed", "prędkość wiatru", "Windgeschwindigkeit", "vitesse du vent", "velocidad del viento", "velocità del vento"}, "precipitation": {"precipitation", "opad", "Niederschlag", "précipitations", "precipitación", "precipitazioni"}, "feels_like": {"feels like", "odczuwalna", "gefühlt", "ressenti", "sensación", "percepita"}, "precipitation_probability": {"precipitation probability", "prawdopodobieństwo opadu", "Niederschlagswahrscheinlichkeit", "probabilité de précipitation", "probabilidad de precipitación", "probabilità di precipitazioni"}}
	idx := map[string]int{"en": 0, "pl": 1, "de": 2, "fr": 3, "es": 4, "it": 5}[locale]
	name := o.Parameter
	if v, ok := labels[o.Parameter]; ok {
		name = v[idx]
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
	var samples []weather.Observation
	for _, o := range obs {
		if o.Parameter == "pressure" && o.PressureKind == "station" && o.Quality == "measured" && time.Since(o.ObservedAt) < 5*time.Hour {
			samples = append(samples, o)
		}
	}
	if len(samples) < 2 {
		return map[string]any{"available": false, "reason": "Potrzeba co najmniej dwóch świeżych pomiarów ciśnienia z odstępem czasu."}
	}
	sortObs(samples)
	first, last := samples[0], samples[len(samples)-1]
	hours := last.ObservedAt.Sub(first.ObservedAt).Hours()
	if hours < 1 {
		return map[string]any{"available": false, "reason": "Pomiary są zbyt blisko siebie."}
	}
	trend := (last.Value - first.Value) / hours * 3
	summary := "stabilne"
	if trend > 1 {
		summary = "rosnące"
	} else if trend < -1 {
		summary = "spadające"
	}
	return map[string]any{"available": true, "label": "szacunek lokalny na podstawie trendu ciśnienia", "pressure_change_hpa_3h": trend, "trend": summary, "based_on": len(samples), "from": first.ObservedAt, "to": last.ObservedAt}
}
func sortObs(o []weather.Observation) {
	for i := 1; i < len(o); i++ {
		for j := i; j > 0 && o[j].ObservedAt.Before(o[j-1].ObservedAt); j-- {
			o[j], o[j-1] = o[j-1], o[j]
		}
	}
}
func urlQuery(s string) string { return url.QueryEscape(s) }
func runtimeArch() string      { return runtime.GOARCH }

func (a *App) scheduleLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	done := map[string]bool{}
	for {
		now := time.Now()
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
			e := audio.Speak(text, c.TTSVoice, c.TTSBinary, c.AudioOutput)
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
			_ = exec.CommandContext(ctx, "stty", "-F", c.SerialPort, strconv.Itoa(c.SerialBaud), "raw", "-echo").Run()
			f, e := os.Open(c.SerialPort)
			if e == nil {
				sc := bufio.NewScanner(f)
				sc.Buffer(make([]byte, 4096), 1<<20)
				for sc.Scan() {
					items, er := weather.DecodeSerialJSON(sc.Bytes())
					if er == nil {
						for _, o := range items {
							_ = a.history.AddObservation(o)
						}
					}
					select {
					case <-ctx.Done():
						f.Close()
						return
					default:
					}
				}
				f.Close()
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
			conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
			if err == nil {
				var frame []byte
				buf := make([]byte, 1024)
				for {
					_ = conn.SetReadDeadline(time.Now().Add(90 * time.Second))
					n, readErr := conn.Read(buf)
					if readErr != nil {
						break
					}
					for _, b := range buf[:n] {
						if b == 0xC0 {
							if len(frame) > 0 {
								if packet, ok := aprs.DecodeKISS(frame); ok {
									if line, decodeErr := aprs.AX25ToTNC2(packet); decodeErr == nil {
										cfg := a.Config()
										f, parseErr := aprs.ParseTNC2(line, time.Now(), cfg.Latitude, cfg.Longitude, cfg.APRSRadiusKM)
										if parseErr == nil {
											_ = a.history.AddFrame(f)
											if f.Eligible {
												for _, o := range f.Data {
													_ = a.history.AddObservation(o)
												}
											}
										}
									}
								}
							}
							frame = []byte{0xC0}
						} else if len(frame) < 4096 {
							frame = append(frame, b)
						}
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
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, e := http.NewRequestWithContext(ctx, "GET", strings.TrimRight(c.HAURL, "/")+"/api/states", nil)
	if e != nil {
		return
	}
	req.Header.Set("Authorization", "Bearer "+c.HAToken)
	resp, e := http.DefaultClient.Do(req)
	if e != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return
	}
	var states []struct {
		EntityID   string         `json:"entity_id"`
		State      string         `json:"state"`
		Attributes map[string]any `json:"attributes"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&states) != nil {
		return
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
		_ = a.history.AddObservation(weather.Observation{Parameter: param, Value: v, Unit: unit, ObservedAt: time.Now().UTC(), Source: "home-assistant:" + s.EntityID, Quality: "measured"})
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
				log.Printf("BME280: %v", err)
			} else {
				at := time.Now().UTC()
				_ = a.history.AddObservation(weather.Observation{Parameter: "temperature", Value: temp, Unit: "°C", ObservedAt: at, Source: "bme280", Quality: "measured"})
				_ = a.history.AddObservation(weather.Observation{Parameter: "humidity", Value: humidity, Unit: "%", ObservedAt: at, Source: "bme280", Quality: "measured"})
				_ = a.history.AddObservation(weather.Observation{Parameter: "pressure", Value: pressure, Unit: "hPa", ObservedAt: at, Source: "bme280", Quality: "measured", PressureKind: "station"})
				if c.AltitudeM > -500 && c.AltitudeM < 9000 {
					corrected := pressure / math.Pow(1-c.AltitudeM/44330, 5.255)
					_ = a.history.AddObservation(weather.Observation{Parameter: "pressure_msl", Value: corrected, Unit: "hPa", ObservedAt: at, Source: "bme280", Quality: "derived", PressureKind: "sea_level"})
				}
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
		a.history.Prune(a.Config().HistoryDays)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
func (a *App) readMQTT(ctx context.Context, c config.Config) {
	host, _ := os.Hostname()
	conn, e := mqtt.Connect(c.MQTTBroker, "ultimatewx-"+host, c.MQTTUsername, c.MQTTPassword, c.MQTTTopic)
	if e != nil {
		log.Printf("MQTT: %v", e)
		select {
		case <-ctx.Done():
		case <-time.After(5 * time.Second):
		}
		return
	}
	defer conn.Close()
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
			return
		}
		if len(msg.Payload) == 0 {
			continue
		}
		items, e := weather.DecodeSerialJSON(msg.Payload)
		if e != nil {
			log.Printf("MQTT sample %s: %v", msg.Topic, e)
			continue
		}
		for _, o := range items {
			o.Source = "mqtt:" + msg.Topic
			if er := a.history.AddObservation(o); er != nil {
				log.Printf("save MQTT observation: %v", er)
			}
		}
	}
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
