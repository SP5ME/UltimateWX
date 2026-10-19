package aprs

import (
	"testing"
	"time"
)

func TestParseNearbyWeatherFrame(t *testing.T) {
	now := time.Now().UTC()
	f, err := ParseTNC2("WX1ABC>APRS,TCPIP*:!5230.00N/02100.00E_090/005g010t020h50b10132", now, 52.5, 21, 10)
	if err != nil {
		t.Fatal(err)
	}
	if !f.IsWX || !f.Eligible || f.Station != "WX1ABC" {
		t.Fatalf("unexpected frame qualification: %+v", f)
	}
	if len(f.Data) < 4 {
		t.Fatalf("weather fields not parsed: %+v", f.Data)
	}
	var pressure float64
	for _, o := range f.Data {
		if o.Parameter == "pressure" {
			pressure = o.Value
		}
	}
	if pressure != 1013.2 {
		t.Fatalf("pressure was not parsed in tenths of hPa: %v", pressure)
	}
}

func TestWeatherFrameOutsideRadiusRemainsVisibleButIneligible(t *testing.T) {
	f, err := ParseTNC2("WX1ABC>APRS:!5330.00N/02100.00E_t020", time.Now().UTC(), 52.5, 21, 10)
	if err != nil {
		t.Fatal(err)
	}
	if !f.IsWX || f.Eligible {
		t.Fatalf("expected WX frame to be filtered by radius: %+v", f)
	}
}

func TestWeatherFrameWithoutPositionRemainsVisibleButIneligible(t *testing.T) {
	f, err := ParseTNC2("WX1ABC>APRS: t020h50b10132", time.Now().UTC(), 52.5, 21, 10)
	if err != nil {
		t.Fatal(err)
	}
	if !f.IsWX || f.Eligible || f.Reason != "weather frame has no supported position" {
		t.Fatalf("expected a positionless weather frame to remain visible but ineligible: %+v", f)
	}
}

func TestNonWeatherFrameAndKISS(t *testing.T) {
	f, err := ParseTNC2("N0CALL>APRS:hello", time.Now().UTC(), 52, 21, 10)
	if err != nil {
		t.Fatal(err)
	}
	if f.IsWX {
		t.Fatal("ordinary APRS message identified as weather")
	}
	got, ok := DecodeKISS([]byte{0xC0, 0x00, 0x01, 0x02, 0xC0})
	if !ok || len(got) != 2 || got[0] != 1 {
		t.Fatalf("KISS decode: %v %v", got, ok)
	}
}
