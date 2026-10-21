package warnings

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"
)

const DefaultIMGWEndpoint = "https://danepubliczne.imgw.pl/api/data/warningsmeteo"

// Alert is a normalized IMGW meteorological warning. AreaCodes are TERYT
// county identifiers supplied by IMGW.
type Alert struct {
	ID              string    `json:"id"`
	Event           string    `json:"event"`
	ContentLanguage string    `json:"content_language,omitempty"`
	Severity        int       `json:"severity"`
	SeverityLabel   string    `json:"severity_label,omitempty"`
	Probability     *int      `json:"probability,omitempty"`
	ValidFrom       time.Time `json:"valid_from"`
	ValidTo         time.Time `json:"valid_to"`
	IssuedAt        time.Time `json:"issued_at"`
	Description     string    `json:"description"`
	Comment         string    `json:"comment,omitempty"`
	Office          string    `json:"office,omitempty"`
	Area            string    `json:"area,omitempty"`
	Attribution     string    `json:"attribution,omitempty"`
	AreaCodes       []string  `json:"area_codes"`
	Source          string    `json:"source"`
	FetchedAt       time.Time `json:"fetched_at"`
}

type IMGW struct {
	Client   *http.Client
	Endpoint string
}

func (p IMGW) Fetch(ctx context.Context) ([]Alert, error) {
	endpoint := p.Endpoint
	if endpoint == "" {
		endpoint = DefaultIMGWEndpoint
	}
	client := p.Client
	if client == nil {
		client = &http.Client{Timeout: 12 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "UltimateWX/0.1 (IMGW warning client)")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("IMGW returned %s", resp.Status)
	}
	var raw []struct {
		ID          string   `json:"id"`
		Event       string   `json:"nazwa_zdarzenia"`
		Severity    string   `json:"stopien"`
		Probability string   `json:"prawdopodobienstwo"`
		ValidTo     string   `json:"obowiazuje_do"`
		ValidFrom   string   `json:"obowiazuje_od"`
		IssuedAt    string   `json:"opublikowano"`
		Description string   `json:"tresc"`
		Comment     string   `json:"komentarz"`
		Office      string   `json:"biuro"`
		AreaCodes   []string `json:"teryt"`
	}
	body, err := readLimitedBody(resp.Body, 4<<20)
	if err != nil {
		return nil, fmt.Errorf("read IMGW warnings: %w", err)
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode IMGW warnings: %w", err)
	}
	location, err := time.LoadLocation("Europe/Warsaw")
	if err != nil {
		return nil, fmt.Errorf("load Europe/Warsaw time zone: %w", err)
	}
	now := time.Now().UTC()
	alerts := make([]Alert, 0, len(raw))
	for _, item := range raw {
		severity, err := strconv.Atoi(strings.TrimSpace(item.Severity))
		if err != nil || severity < 1 || severity > 3 || strings.TrimSpace(item.ID) == "" {
			continue
		}
		from, err := parseLocalTime(item.ValidFrom, location)
		if err != nil {
			continue
		}
		to, err := parseLocalTime(item.ValidTo, location)
		if err != nil || !to.After(from) {
			continue
		}
		issued, _ := parseLocalTime(item.IssuedAt, location)
		probabilityValue, hasProbability := parseProbability(item.Probability)
		var probability *int
		if hasProbability {
			probability = &probabilityValue
		}
		codes := make([]string, 0, len(item.AreaCodes))
		for _, code := range item.AreaCodes {
			code = strings.TrimSpace(code)
			if code != "" {
				codes = append(codes, code)
			}
		}
		alerts = append(alerts, Alert{
			ID: strings.TrimSpace(item.ID), Event: strings.TrimSpace(item.Event), Severity: severity,
			ContentLanguage: "pl",
			Probability:     probability, ValidFrom: from.UTC(), ValidTo: to.UTC(), IssuedAt: issued.UTC(),
			Description: strings.TrimSpace(item.Description), Comment: strings.TrimSpace(item.Comment),
			Office: strings.TrimSpace(item.Office), AreaCodes: codes, Source: "IMGW-PIB", FetchedAt: now,
		})
	}
	return alerts, nil
}

func parseLocalTime(value string, location *time.Location) (time.Time, error) {
	return time.ParseInLocation("2006-01-02 15:04:05", strings.TrimSpace(value), location)
}

func MatchesArea(alert Alert, configured []string) bool {
	if len(configured) == 0 {
		return false
	}
	areas := make(map[string]struct{}, len(configured))
	for _, code := range configured {
		if code = strings.TrimSpace(code); code != "" {
			areas[code] = struct{}{}
		}
	}
	for _, code := range alert.AreaCodes {
		if _, ok := areas[strings.TrimSpace(code)]; ok {
			return true
		}
	}
	return false
}
