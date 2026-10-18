package weather

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestDecodeSerialJSON(t *testing.T) {
	items, err := DecodeSerialJSON([]byte(`{"device":"esp32-roof","timestamp":"2026-10-18T10:00:00Z","temperature_c":12.5,"humidity_percent":72,"pressure_hpa":1008.2}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 || items[0].Source != "serial:esp32-roof" || items[2].PressureKind != "station" {
		t.Fatalf("unexpected observations: %+v", items)
	}
	if _, err = DecodeSerialJSON([]byte(`{"temperature_c":10}`)); err == nil {
		t.Fatal("missing device should fail")
	}
}

func TestConvertTemperatureWindAndRain(t *testing.T) {
	items := []Observation{{Parameter: "temperature", Value: 0, Unit: "°C"}, {Parameter: "wind_speed", Value: 10, Unit: "km/h"}, {Parameter: "precipitation", Value: 25.4, Unit: "mm"}}
	got := Convert(items[0], "imperial")
	if got.Value != 32 || got.Unit != "°F" {
		t.Fatalf("temperature conversion: %+v", got)
	}
	got = Convert(items[1], "imperial")
	if got.Value < 6.21 || got.Value > 6.22 || got.Unit != "mph" {
		t.Fatalf("wind conversion: %+v", got)
	}
	got = Convert(items[2], "imperial")
	if got.Value != 1 || got.Unit != "inch" {
		t.Fatalf("rain conversion: %+v", got)
	}
}

func TestOpenMeteoProviderMapsCurrentAndFourHourForecast(t *testing.T) {
	payload := `{"current":{"time":"2026-10-18T10:00","temperature_2m":11,"relative_humidity_2m":60,"apparent_temperature":10,"precipitation":0,"weather_code":2,"wind_speed_10m":5,"wind_direction_10m":90,"surface_pressure":1005},"current_units":{"temperature_2m":"°C","wind_speed_10m":"km/h","surface_pressure":"hPa"},"hourly":{"time":["2026-10-18T10:00","2026-10-18T14:00","2026-10-18T15:00"],"temperature_2m":[11,13,14],"precipitation_probability":[0,20,40],"precipitation":[0,0.1,0.2],"wind_speed_10m":[5,6,7],"wind_direction_10m":[90,100,110]},"hourly_units":{"temperature_2m":"°C","wind_speed_10m":"km/h"}}`
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Query().Get("timezone") != "UTC" {
			t.Errorf("timezone not normalized to UTC")
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(payload)), Request: r}, nil
	})}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	got, err := (OpenMeteo{Client: client, Endpoint: "https://weather.invalid/forecast"}).Fetch(ctx, 52, 21, "metric")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Current) != 8 || len(got.Forecast) != 5 {
		t.Fatalf("unexpected snapshot sizes current=%d forecast=%d", len(got.Current), len(got.Forecast))
	}
	if got.Forecast[0].Value != 13 {
		t.Fatalf("forecast not chosen near +4h: %+v", got.Forecast[0])
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
