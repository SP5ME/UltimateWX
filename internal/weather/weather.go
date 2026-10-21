package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Observation struct {
	Parameter       string    `json:"parameter"`
	Value           float64   `json:"value"`
	Unit            string    `json:"unit"`
	ObservedAt      time.Time `json:"observed_at"`
	Source          string    `json:"source"`
	DeviceID        string    `json:"device_id,omitempty"`
	DeviceName      string    `json:"device_name,omitempty"`
	Quality         string    `json:"quality"`
	Station         string    `json:"station,omitempty"`
	PressureKind    string    `json:"pressure_kind,omitempty"`
	SensorAltitudeM *float64  `json:"sensor_altitude_m,omitempty"`
	FrameID         string    `json:"frame_id,omitempty"`
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

const maxOpenMeteoResponseBytes = 4 << 20
const maxForecastTimeOffset = 90 * time.Minute

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
			Time        string   `json:"time"`
			Interval    int      `json:"interval"`
			Temperature *float64 `json:"temperature_2m"`
			Humidity    *float64 `json:"relative_humidity_2m"`
			Feels       *float64 `json:"apparent_temperature"`
			Precip      *float64 `json:"precipitation"`
			Code        *int     `json:"weather_code"`
			Wind        *float64 `json:"wind_speed_10m"`
			WindDir     *float64 `json:"wind_direction_10m"`
			Pressure    *float64 `json:"surface_pressure"`
		} `json:"current"`
		CurrentUnits map[string]string `json:"current_units"`
		Hourly       struct {
			Time        []string   `json:"time"`
			Temperature []*float64 `json:"temperature_2m"`
			Pop         []*float64 `json:"precipitation_probability"`
			Precip      []*float64 `json:"precipitation"`
			Wind        []*float64 `json:"wind_speed_10m"`
			WindDir     []*float64 `json:"wind_direction_10m"`
		} `json:"hourly"`
		HourlyUnits map[string]string `json:"hourly_units"`
	}
	body, e := io.ReadAll(io.LimitReader(resp.Body, maxOpenMeteoResponseBytes+1))
	if e != nil {
		return Snapshot{}, e
	}
	if len(body) > maxOpenMeteoResponseBytes {
		return Snapshot{}, fmt.Errorf("Open-Meteo response exceeds %d-byte limit", maxOpenMeteoResponseBytes)
	}
	if e = json.Unmarshal(body, &raw); e != nil {
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
	s := Snapshot{UpdatedAt: now, Attribution: "Weather data by Open-Meteo (CC BY 4.0)"}
	appendCurrent := func(parameter string, value *float64, measurementUnit string) {
		if value != nil {
			s.Current = append(s.Current, obs(parameter, *value, measurementUnit, at, "open-meteo"))
		}
	}
	appendCurrent("temperature", raw.Current.Temperature, unit("temperature_2m", "°C"))
	appendCurrent("humidity", raw.Current.Humidity, "%")
	appendCurrent("feels_like", raw.Current.Feels, unit("apparent_temperature", "°C"))
	appendCurrent("precipitation", raw.Current.Precip, unit("precipitation", "mm"))
	if raw.Current.Code != nil {
		s.Current = append(s.Current, obs("weather_code", float64(*raw.Current.Code), "WMO code", at, "open-meteo"))
	}
	appendCurrent("wind_speed", raw.Current.Wind, unit("wind_speed_10m", "km/h"))
	appendCurrent("wind_direction", raw.Current.WindDir, "°")
	if raw.Current.Pressure != nil {
		s.Current = append(s.Current, Observation{Parameter: "pressure", Value: *raw.Current.Pressure, Unit: unit("surface_pressure", "hPa"), ObservedAt: at, Source: "open-meteo", Quality: "modelled", PressureKind: "station"})
	}
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
	if idx >= 0 && best <= maxForecastTimeOffset {
		forecastAt, _ := time.ParseInLocation("2006-01-02T15:04", raw.Hourly.Time[idx], time.UTC)
		appendForecast := func(parameter string, values []*float64, unit string) {
			if idx < len(values) && values[idx] != nil {
				s.Forecast = append(s.Forecast, obs(parameter, *values[idx], defaultString(unit), forecastAt.UTC(), "open-meteo"))
			}
		}
		appendForecast("temperature", raw.Hourly.Temperature, raw.HourlyUnits["temperature_2m"])
		appendForecast("precipitation_probability", raw.Hourly.Pop, "%")
		appendForecast("precipitation", raw.Hourly.Precip, raw.HourlyUnits["precipitation"])
		appendForecast("wind_speed", raw.Hourly.Wind, raw.HourlyUnits["wind_speed_10m"])
		appendForecast("wind_direction", raw.Hourly.WindDir, "°")
	}
	return s, nil
}
func obs(p string, v float64, u string, t time.Time, src string) Observation {
	return Observation{Parameter: p, Value: v, Unit: u, ObservedAt: t, Source: src, Quality: "modelled"}
}
func defaultString(s string) string {
	if strings.TrimSpace(s) == "" {
		return "unknown"
	}
	return s
}

func Convert(o Observation, units string) Observation {
	switch {
	case o.Parameter == "pressure" || o.Parameter == "pressure_msl":
		unit := strings.ToLower(strings.TrimSpace(o.Unit))
		hectopascalsPerUnit := 0.0
		switch unit {
		case "hpa", "hectopascal", "hectopascals", "mbar", "mb", "millibar", "millibars":
			hectopascalsPerUnit = 1
		case "pa", "pascal", "pascals":
			hectopascalsPerUnit = 0.01
		case "kpa", "kilopascal", "kilopascals":
			hectopascalsPerUnit = 10
		case "bar":
			hectopascalsPerUnit = 1000
		case "inhg", "in hg":
			hectopascalsPerUnit = 33.8638866667
		case "mmhg", "mm hg":
			hectopascalsPerUnit = 1.33322387415
		}
		if hectopascalsPerUnit > 0 {
			o.Value *= hectopascalsPerUnit
			o.Unit = "hPa"
		}
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
		unit := strings.ToLower(strings.TrimSpace(o.Unit))
		metersPerUnit := 0.0
		switch unit {
		case "m/s", "mps", "ms-1", "m s-1":
			metersPerUnit = 1
		case "km/h", "kmh", "kph":
			metersPerUnit = 1.0 / 3.6
		case "mph", "mi/h":
			metersPerUnit = 0.44704
		case "kt", "kts", "kn", "knot", "knots":
			metersPerUnit = 0.514444
		default:
			return o
		}
		if units == "imperial" {
			o.Value = o.Value * metersPerUnit / 0.44704
			o.Unit = "mph"
		} else if units == "aviation" {
			o.Value = o.Value * metersPerUnit / 0.514444
			o.Unit = "kt"
		} else {
			o.Value = o.Value * metersPerUnit * 3.6
			o.Unit = "km/h"
		}
	case o.Parameter == "wind_direction":
		switch strings.ToLower(strings.TrimSpace(o.Unit)) {
		case "°", "deg", "degree", "degrees":
			o.Unit = "°"
		}
	case o.Parameter == "precipitation":
		unit := strings.ToLower(strings.TrimSpace(o.Unit))
		millimetersPerUnit := 0.0
		switch unit {
		case "mm", "millimeter", "millimeters", "millimetre", "millimetres":
			millimetersPerUnit = 1
		case "in", "inch", "inches":
			millimetersPerUnit = 25.4
		default:
			return o
		}
		if units == "imperial" {
			o.Value = o.Value * millimetersPerUnit / 25.4
			o.Unit = "inch"
		} else {
			o.Value *= millimetersPerUnit
			o.Unit = "mm"
		}
	}
	return o
}

// ValidateSensorObservation checks values accepted from configurable sensor integrations.
func ValidateSensorObservation(o Observation) error {
	if o.ObservedAt.IsZero() || o.ObservedAt.After(time.Now().UTC().Add(5*time.Minute)) {
		return fmt.Errorf("measurement timestamp is missing or more than five minutes in the future")
	}
	if o.SensorAltitudeM != nil && (math.IsNaN(*o.SensorAltitudeM) || math.IsInf(*o.SensorAltitudeM, 0) || *o.SensorAltitudeM < -500 || *o.SensorAltitudeM > 9000) {
		return fmt.Errorf("sensor altitude is outside the supported range -500..9000 m")
	}
	if math.IsNaN(o.Value) || math.IsInf(o.Value, 0) {
		return fmt.Errorf("measurement value is not finite")
	}
	normalized := Convert(o, "metric")
	switch o.Parameter {
	case "temperature", "feels_like":
		if normalized.Unit != "°C" || normalized.Value < -100 || normalized.Value > 70 {
			return fmt.Errorf("temperature is outside the supported range -100..70 °C")
		}
	case "humidity":
		if normalized.Unit != "%" || normalized.Value < 0 || normalized.Value > 100 {
			return fmt.Errorf("humidity is outside the supported range 0..100%%")
		}
	case "pressure":
		if normalized.Unit != "hPa" || normalized.Value < 300 || normalized.Value > 1200 {
			return fmt.Errorf("pressure is outside the supported range 300..1200 hPa")
		}
	case "wind_speed":
		if normalized.Unit != "km/h" || normalized.Value < 0 || normalized.Value > 432 {
			return fmt.Errorf("wind speed is outside the supported range 0..120 m/s")
		}
	case "wind_direction":
		if normalized.Unit != "°" || normalized.Value < 0 || normalized.Value > 360 {
			return fmt.Errorf("wind direction is outside the supported range 0..360 degrees")
		}
	case "precipitation":
		if normalized.Unit != "mm" || normalized.Value < 0 || normalized.Value > 10000 {
			return fmt.Errorf("precipitation is outside the supported range 0..10000 mm")
		}
	default:
		return fmt.Errorf("unsupported sensor parameter %q", o.Parameter)
	}
	return nil
}

func DecodeSerialJSON(line []byte) ([]Observation, error) {
	var v struct {
		Device        string    `json:"device"`
		Name          string    `json:"name"`
		Timestamp     time.Time `json:"timestamp"`
		Altitude      *float64  `json:"sensor_altitude_m"`
		Temperature   *float64  `json:"temperature_c"`
		Humidity      *float64  `json:"humidity_percent"`
		Pressure      *float64  `json:"pressure_hpa"`
		WindSpeed     *float64  `json:"wind_speed_mps"`
		WindDirection *float64  `json:"wind_direction_deg"`
		Precipitation *float64  `json:"precipitation_mm"`
	}
	if e := json.Unmarshal(line, &v); e != nil {
		return nil, e
	}
	v.Device = strings.TrimSpace(v.Device)
	v.Name = strings.TrimSpace(v.Name)
	if v.Device == "" {
		return nil, fmt.Errorf("device field is required")
	}
	if len(v.Device) > 128 || len(v.Name) > 128 {
		return nil, fmt.Errorf("device ID and name must be at most 128 characters")
	}
	if v.Name == "" {
		v.Name = v.Device
	}
	at := v.Timestamp
	if at.IsZero() {
		at = time.Now().UTC()
	}
	if at.After(time.Now().UTC().Add(5 * time.Minute)) {
		return nil, fmt.Errorf("timestamp is more than five minutes in the future")
	}
	for _, measurement := range []struct {
		name  string
		value *float64
		min   float64
		max   float64
	}{
		{"temperature_c", v.Temperature, -100, 70},
		{"humidity_percent", v.Humidity, 0, 100},
		{"pressure_hpa", v.Pressure, 300, 1200},
		{"wind_speed_mps", v.WindSpeed, 0, 120},
		{"wind_direction_deg", v.WindDirection, 0, 360},
		{"precipitation_mm", v.Precipitation, 0, 10000},
		{"sensor_altitude_m", v.Altitude, -500, 9000},
	} {
		if measurement.value == nil {
			continue
		}
		value := *measurement.value
		if math.IsNaN(value) || math.IsInf(value, 0) || value < measurement.min || value > measurement.max {
			return nil, fmt.Errorf("%s is outside the supported range %.0f..%.0f", measurement.name, measurement.min, measurement.max)
		}
	}
	var out []Observation
	add := func(p, u string, x *float64, kind string) {
		if x != nil {
			out = append(out, Observation{Parameter: p, Value: *x, Unit: u, ObservedAt: at, Source: "serial:" + v.Device, DeviceID: v.Device, DeviceName: v.Name, Quality: "measured", PressureKind: kind, SensorAltitudeM: v.Altitude})
		}
	}
	add("temperature", "°C", v.Temperature, "")
	add("humidity", "%", v.Humidity, "")
	add("pressure", "hPa", v.Pressure, "station")
	add("wind_speed", "m/s", v.WindSpeed, "")
	add("wind_direction", "°", v.WindDirection, "")
	add("precipitation", "mm", v.Precipitation, "")
	if len(out) == 0 {
		return nil, fmt.Errorf("sample contains no recognized measurements")
	}
	return out, nil
}
