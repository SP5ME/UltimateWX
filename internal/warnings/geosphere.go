package warnings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const DefaultGeoSphereEndpoint = "https://warnungen.zamg.at/wsapp/api/getWarningsForCoords"

var ErrGeoSphereOutsideCoverage = errors.New("station is outside GeoSphere Austria warning coverage")

// GeoSphere fetches official GeoSphere Austria warnings for the municipality
// containing the configured station coordinates.
type GeoSphere struct {
	Client   *http.Client
	Endpoint string
	Locale   string
}

type geoSphereResponse struct {
	Type       string `json:"type"`
	Message    string `json:"msg"`
	Properties struct {
		Location struct {
			Properties struct {
				Name string `json:"name"`
			} `json:"properties"`
		} `json:"location"`
		Warnings []struct {
			Properties struct {
				WarningID json.RawMessage `json:"warnid"`
				ChangeID  json.RawMessage `json:"chgid"`
				Revision  json.RawMessage `json:"verlaufid"`
				Kind      int             `json:"warntypid"`
				Level     int             `json:"warnstufeid"`
				Text      string          `json:"text"`
				Impacts   string          `json:"auswirkungen"`
				Advice    string          `json:"empfehlungen"`
				Created   string          `json:"create"`
				Raw       struct {
					Start json.RawMessage `json:"start"`
					End   json.RawMessage `json:"end"`
				} `json:"rawinfo"`
			} `json:"properties"`
		} `json:"warnings"`
	} `json:"properties"`
}

func (p GeoSphere) Fetch(ctx context.Context, latitude, longitude float64) ([]Alert, error) {
	endpoint := p.Endpoint
	if endpoint == "" {
		endpoint = DefaultGeoSphereEndpoint
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme != "https" || parsed.Host != "warnungen.zamg.at" {
		return nil, fmt.Errorf("invalid GeoSphere warning endpoint")
	}
	locale, _, _ := strings.Cut(strings.ToLower(strings.TrimSpace(p.Locale)), "-")
	language := "en"
	if locale == "de" {
		language = "de"
	}
	query := parsed.Query()
	query.Set("lat", strconv.FormatFloat(latitude, 'f', 6, 64))
	query.Set("lon", strconv.FormatFloat(longitude, 'f', 6, 64))
	query.Set("lang", language)
	parsed.RawQuery = query.Encode()
	client := p.Client
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/geo+json, application/json")
	req.Header.Set("User-Agent", "UltimateWX/0.1 (GeoSphere warning client)")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GeoSphere warning request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GeoSphere returned %s", resp.Status)
	}
	var doc geoSphereResponse
	body, err := readLimitedBody(resp.Body, 4<<20)
	if err != nil {
		return nil, fmt.Errorf("read GeoSphere warnings: %w", err)
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("decode GeoSphere warnings: %w", err)
	}
	if strings.EqualFold(doc.Type, "Error") {
		message := strings.TrimSpace(doc.Message)
		if strings.Contains(strings.ToLower(message), "could not find municipal") {
			return nil, fmt.Errorf("%w: %s", ErrGeoSphereOutsideCoverage, message)
		}
		return nil, fmt.Errorf("GeoSphere: %s", message)
	}
	municipality := strings.TrimSpace(doc.Properties.Location.Properties.Name)
	now := time.Now().UTC()
	alerts := make([]Alert, 0, len(doc.Properties.Warnings))
	for _, row := range doc.Properties.Warnings {
		w := row.Properties
		if w.Level < 1 || w.Level > 3 {
			continue
		}
		start, startOK := geoSphereEpoch(w.Raw.Start)
		end, endOK := geoSphereEpoch(w.Raw.End)
		if !startOK || !endOK || !end.After(start) {
			continue
		}
		warningID := rawJSONText(w.WarningID)
		if warningID == "" {
			continue
		}
		id := strings.Join([]string{warningID, rawJSONText(w.ChangeID), rawJSONText(w.Revision), start.UTC().Format(time.RFC3339)}, ":")
		event := geoSphereEvent(w.Kind, locale)
		label := geoSphereSeverityLabel(w.Level, locale)
		description := joinNonEmpty("\n", w.Text, w.Impacts, w.Advice)
		issued, _ := time.Parse(time.RFC3339Nano, strings.TrimSpace(w.Created))
		alerts = append(alerts, Alert{ID: "geosphere:" + id, Event: event, ContentLanguage: language, Severity: w.Level, SeverityLabel: label, ValidFrom: start.UTC(), ValidTo: end.UTC(), IssuedAt: issued.UTC(), Description: description, Area: municipality, Attribution: "© GeoSphere Austria; CC BY 4.0", Source: "GeoSphere Austria", FetchedAt: now})
	}
	return alerts, nil
}

func geoSphereEpoch(value json.RawMessage) (time.Time, bool) {
	text := strings.TrimSpace(string(value))
	if text == "" || text == "null" {
		return time.Time{}, false
	}
	if text[0] == '"' {
		var decoded string
		if json.Unmarshal(value, &decoded) != nil {
			return time.Time{}, false
		}
		text = strings.TrimSpace(decoded)
	}
	seconds, err := strconv.ParseInt(text, 10, 64)
	if err != nil || seconds <= 0 {
		return time.Time{}, false
	}
	return time.Unix(seconds, 0).UTC(), true
}

func rawJSONText(value json.RawMessage) string {
	text := strings.TrimSpace(string(value))
	if len(text) > 0 && text[0] == '"' {
		var decoded string
		if json.Unmarshal(value, &decoded) == nil {
			return strings.TrimSpace(decoded)
		}
	}
	return text
}

func geoSphereEvent(kind int, locale string) string {
	names := map[int][6]string{
		1: {"Wind", "silny wiatr", "Wind", "vent", "viento", "vento"},
		2: {"Rain", "intensywny deszcz", "Regen", "pluie", "lluvia", "pioggia"},
		3: {"Snow", "opady śniegu", "Schnee", "neige", "nieve", "neve"},
		4: {"Black ice", "gołoledź", "Glatteis", "verglas", "hielo", "ghiaccio"},
		5: {"Thunderstorm", "burza", "Gewitter", "orage", "tormenta", "temporale"},
		6: {"Heat", "upał", "Hitze", "chaleur", "calor", "caldo"},
		7: {"Cold", "mróz", "Kälte", "froid", "frío", "freddo"},
	}
	name, ok := names[kind]
	if !ok {
		unknown := map[string]string{"pl": "zjawisko pogodowe", "de": "Wettergefahr", "fr": "phénomène météo", "es": "fenómeno meteorológico", "it": "fenomeno meteorologico"}
		if value := unknown[locale]; value != "" {
			return value
		}
		return "Weather"
	}
	index := map[string]int{"en": 0, "pl": 1, "de": 2, "fr": 3, "es": 4, "it": 5}[locale]
	return name[index]
}

func geoSphereSeverityLabel(level int, locale string) string {
	labels := map[int][6]string{1: {"Yellow", "żółty", "Gelb", "jaune", "amarillo", "giallo"}, 2: {"Orange", "pomarańczowy", "Orange", "orange", "naranja", "arancione"}, 3: {"Red", "czerwony", "Rot", "rouge", "rojo", "rosso"}}
	values, ok := labels[level]
	if !ok {
		return ""
	}
	index := map[string]int{"en": 0, "pl": 1, "de": 2, "fr": 3, "es": 4, "it": 5}[locale]
	return values[index]
}

func joinNonEmpty(separator string, values ...string) string {
	clean := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			clean = append(clean, value)
		}
	}
	return strings.Join(clean, separator)
}
