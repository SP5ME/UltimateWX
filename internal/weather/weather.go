package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Observation struct {
	Parameter    string    `json:"parameter"`
	Value        float64   `json:"value"`
	Unit         string    `json:"unit"`
	ObservedAt   time.Time `json:"observed_at"`
	Source       string    `json:"source"`
	Quality      string    `json:"quality"`
	Station      string    `json:"station,omitempty"`
	PressureKind string    `json:"pressure_kind,omitempty"`
}
type Snapshot struct {
	Location    string        `json:"location"`
	UpdatedAt   time.Time     `json:"updated_at"`
	Current     []Observation `json:"current"`
	Forecast    []Observation `json:"forecast"`
	Attribution string        `json:"attribution"`
	Error       string        `json:"error,omitempty"`
}
type Provider interface {
	Fetch(context.Context, float64, float64, string) (Snapshot, error)
}
type OpenMeteo struct {
	Client   *http.Client
	Endpoint string
}

func (p OpenMeteo) Fetch(ctx context.Context, lat, lon float64, units string) (Snapshot, error) {
	q := url.Values{}
	q.Set("latitude", strconv.FormatFloat(lat, 'f', 4, 64))
	q.Set("longitude", strconv.FormatFloat(lon, 'f', 4, 64))
	q.Set("current", "temperature_2m,relative_humidity_2m,apparent_temperature,precipitation,weather_code,wind_speed_10m,wind_direction_10m,surface_pressure")
	q.Set("hourly", "temperature_2m,precipitation_probability,precipitation,wind_speed_10m,wind_direction_10m")
	q.Set("forecast_days", "2")
	q.Set("timezone", "UTC")
	if units == "imperial" {
		q.Set("temperature_unit", "fahrenheit")
		q.Set("wind_speed_unit", "mph")
		q.Set("precipitation_unit", "inch")
	}
	endpoint := p.Endpoint
	if endpoint == "" {
		endpoint = "https://api.open-meteo.com/v1/forecast"
	}
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+q.Encode(), nil)
	if e != nil {
		return Snapshot{}, e
	}
	client := p.Client
	if client == nil {
		client = &http.Client{Timeout: 12 * time.Second}
	}
	resp, e := client.Do(req)
	if e != nil {
		return Snapshot{}, e
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Snapshot{}, fmt.Errorf("Open-Meteo returned %s", resp.Status)
	}
	var raw struct {
		Current struct {
			Time        string  `json:"time"`
			Interval    int     `json:"interval"`
			Temperature float64 `json:"temperature_2m"`
			Humidity    float64 `json:"relative_humidity_2m"`
			Feels       float64 `json:"apparent_temperature"`
			Precip      float64 `json:"precipitation"`
			Code        int     `json:"weather_code"`
			Wind        float64 `json:"wind_speed_10m"`
			WindDir     float64 `json:"wind_direction_10m"`
			Pressure    float64 `json:"surface_pressure"`
		} `json:"current"`
		CurrentUnits map[string]string `json:"current_units"`
		Hourly       struct {
			Time        []string  `json:"time"`
			Temperature []float64 `json:"temperature_2m"`
			Pop         []float64 `json:"precipitation_probability"`
			Precip      []float64 `json:"precipitation"`
			Wind        []float64 `json:"wind_speed_10m"`
			WindDir     []float64 `json:"wind_direction_10m"`
		} `json:"hourly"`
		HourlyUnits map[string]string `json:"hourly_units"`
	}
	if e = json.NewDecoder(resp.Body).Decode(&raw); e != nil {
		return Snapshot{}, e
	}
	now := time.Now().UTC()
	at := now
	if t, err := time.ParseInLocation("2006-01-02T15:04", raw.Current.Time, time.UTC); err == nil {
		at = t.UTC()
	}
	unit := func(k, def string) string {
		if x := raw.CurrentUnits[k]; x != "" {
			return x
		}
		return def
	}
	s := Snapshot{UpdatedAt: now, Attribution: "Weather data by Open-Meteo (CC BY 4.0)", Current: []Observation{
		obs("temperature", raw.Current.Temperature, unit("temperature_2m", "°C"), at, "open-meteo"), obs("humidity", raw.Current.Humidity, "%", at, "open-meteo"), obs("feels_like", raw.Current.Feels, unit("apparent_temperature", "°C"), at, "open-meteo"), obs("precipitation", raw.Current.Precip, unit("precipitation", "mm"), at, "open-meteo"), obs("weather_code", float64(raw.Current.Code), "WMO code", at, "open-meteo"), obs("wind_speed", raw.Current.Wind, unit("wind_speed_10m", "km/h"), at, "open-meteo"), obs("wind_direction", raw.Current.WindDir, "°", at, "open-meteo"), {Parameter: "pressure", Value: raw.Current.Pressure, Unit: unit("surface_pressure", "hPa"), ObservedAt: at, Source: "open-meteo", Quality: "modelled", PressureKind: "station"}}}
	currentTime := at
	target := currentTime.Add(4 * time.Hour)
	idx := -1
	best := time.Duration(1<<63 - 1)
	for i, ts := range raw.Hourly.Time {
		t, err := time.ParseInLocation("2006-01-02T15:04", ts, time.UTC)
		if err != nil {
			continue
		}
		d := t.Sub(target)
		if d < 0 {
			d = -d
		}
		if d < best {
			best = d
			idx = i
		}
	}
	if idx >= 0 && idx < len(raw.Hourly.Temperature) {
		forecastAt, _ := time.ParseInLocation("2006-01-02T15:04", raw.Hourly.Time[idx], time.UTC)
		items := [][3]any{{"temperature", raw.Hourly.Temperature[idx], raw.HourlyUnits["temperature_2m"]}, {"precipitation_probability", valueAt(raw.Hourly.Pop, idx), "%"}, {"precipitation", valueAt(raw.Hourly.Precip, idx), raw.HourlyUnits["precipitation"]}, {"wind_speed", valueAt(raw.Hourly.Wind, idx), raw.HourlyUnits["wind_speed_10m"]}, {"wind_direction", valueAt(raw.Hourly.WindDir, idx), "°"}}
		for _, it := range items {
			if v, ok := it[1].(float64); ok {
				s.Forecast = append(s.Forecast, obs(it[0].(string), v, defaultString(it[2].(string)), forecastAt.UTC(), "open-meteo"))
			}
		}
	}
	return s, nil
}
func obs(p string, v float64, u string, t time.Time, src string) Observation {
	return Observation{Parameter: p, Value: v, Unit: u, ObservedAt: t, Source: src, Quality: "modelled"}
}
func valueAt(a []float64, i int) float64 {
	if i < len(a) {
		return a[i]
	}
	return 0
}
func defaultString(s string) string {
	if strings.TrimSpace(s) == "" {
		return "unknown"
	}
	return s
}

func Convert(o Observation, units string) Observation {
	switch {
	case o.Parameter == "temperature" || o.Parameter == "feels_like":
		if units == "imperial" && strings.Contains(o.Unit, "°C") {
			o.Value = o.Value*9/5 + 32
			o.Unit = "°F"
		}
		if units != "imperial" && strings.Contains(o.Unit, "°F") {
			o.Value = (o.Value - 32) * 5 / 9
			o.Unit = "°C"
		}
	case o.Parameter == "wind_speed":
		if units == "imperial" && !strings.Contains(o.Unit, "mph") {
			o.Value *= 0.621371
			o.Unit = "mph"
		}
		if units != "imperial" && strings.Contains(o.Unit, "mph") {
			o.Value /= 0.621371
			o.Unit = "km/h"
		}
	case o.Parameter == "precipitation":
		if units == "imperial" && !strings.Contains(o.Unit, "inch") {
			o.Value /= 25.4
			o.Unit = "inch"
		}
		if units != "imperial" && strings.Contains(o.Unit, "inch") {
			o.Value *= 25.4
			o.Unit = "mm"
		}
	}
	return o
}

func DecodeSerialJSON(line []byte) ([]Observation, error) {
	var v struct {
		Device        string    `json:"device"`
		Timestamp     time.Time `json:"timestamp"`
		Temperature   *float64  `json:"temperature_c"`
		Humidity      *float64  `json:"humidity_percent"`
		Pressure      *float64  `json:"pressure_hpa"`
		WindSpeed     *float64  `json:"wind_speed_mps"`
		Precipitation *float64  `json:"precipitation_mm"`
	}
	if e := json.Unmarshal(line, &v); e != nil {
		return nil, e
	}
	if v.Device == "" {
		return nil, fmt.Errorf("device field is required")
	}
	at := v.Timestamp
	if at.IsZero() {
		at = time.Now().UTC()
	}
	var out []Observation
	add := func(p, u string, x *float64, kind string) {
		if x != nil {
			out = append(out, Observation{Parameter: p, Value: *x, Unit: u, ObservedAt: at, Source: "serial:" + v.Device, Quality: "measured", PressureKind: kind})
		}
	}
	add("temperature", "°C", v.Temperature, "")
	add("humidity", "%", v.Humidity, "")
	add("pressure", "hPa", v.Pressure, "station")
	add("wind_speed", "m/s", v.WindSpeed, "")
	add("precipitation", "mm", v.Precipitation, "")
	if len(out) == 0 {
		return nil, fmt.Errorf("sample contains no recognized measurements")
	}
	return out, nil
}
