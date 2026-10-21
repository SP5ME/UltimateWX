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

func TestNegativeTemperatureReportIsRecognizedAsWeather(t *testing.T) {
	f, err := ParseTNC2("WX1ABC>APRS:!5230.00N/02100.00E_t-01", time.Now().UTC(), 52.5, 21, 10)
	if err != nil {
		t.Fatal(err)
	}
	if !f.IsWX || !f.Eligible || len(f.Data) != 1 || f.Data[0].Parameter != "temperature" || f.Data[0].Value != -1 {
		t.Fatalf("negative APRS temperature was not recognized: %+v", f)
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

func TestTimestampedWeatherUsesReportTimeAndRejectsStaleReports(t *testing.T) {
	received := time.Now().UTC().Truncate(time.Minute)
	position := "5230.00N/02100.00E_t020"
	frame, err := ParseTNC2("WX1ABC>APRS:@"+received.Format("020304")+"z"+position, received, 52.5, 21, 10)
	if err != nil {
		t.Fatal(err)
	}
	if !frame.Eligible || len(frame.Data) == 0 || !frame.Data[0].ObservedAt.Equal(received) {
		t.Fatalf("report timestamp was not applied: %+v", frame)
	}
	old := received.Add(-3 * time.Hour)
	frame, err = ParseTNC2("WX1ABC>APRS:@"+old.Format("020304")+"z"+position, received, 52.5, 21, 10)
	if err != nil {
		t.Fatal(err)
	}
	if frame.Eligible || frame.Reason != "stale/future frame" || len(frame.Data) != 0 {
		t.Fatalf("stale timestamped report should be rejected: %+v", frame)
	}
}

func TestReportTimestampHandlesMonthAndDayRollover(t *testing.T) {
	received := time.Date(2026, time.January, 1, 0, 5, 0, 0, time.UTC)
	got, valid := reportTimestamp("@312359z", received)
	want := time.Date(2025, time.December, 31, 23, 59, 0, 0, time.UTC)
	if !valid || !got.Equal(want) {
		t.Fatalf("month rollover: got %s, want %s", got, want)
	}
	got, valid = reportTimestamp("@235950h", received)
	want = time.Date(2025, time.December, 31, 23, 59, 50, 0, time.UTC)
	if !valid || !got.Equal(want) {
		t.Fatalf("day rollover: got %s, want %s", got, want)
	}
}

func TestReportTimestampUsesArrivalForLocalTimestampAndRejectsFutureReport(t *testing.T) {
	received := time.Now().UTC().Truncate(time.Minute)
	if got, valid := reportTimestamp("@312359/", received); !valid || !got.Equal(received) {
		t.Fatalf("local timestamp should use arrival time without a known station timezone: got %s, want %s", got, received)
	}
	future := received.Add(10 * time.Minute)
	position := "5230.00N/02100.00E_t020"
	frame, err := ParseTNC2("WX1ABC>APRS:@"+future.Format("020304")+"z"+position, received, 52.5, 21, 10)
	if err != nil {
		t.Fatal(err)
	}
	if frame.Eligible || frame.Reason != "stale/future frame" {
		t.Fatalf("future timestamped report should be rejected: %+v", frame)
	}
}

func TestMalformedWeatherReportTimestampIsIneligible(t *testing.T) {
	received := time.Now().UTC()
	frame, err := ParseTNC2("WX1ABC>APRS:@322460z5230.00N/02100.00E_t020", received, 52.5, 21, 10)
	if err != nil {
		t.Fatal(err)
	}
	if frame.Eligible || frame.Reason != "invalid report timestamp" || len(frame.Data) != 0 {
		t.Fatalf("malformed timestamped WX frame should not supply measurements: %+v", frame)
	}
}

func TestWeatherFrameWithInvalidPositionRemainsVisibleButIneligible(t *testing.T) {
	f, err := ParseTNC2("WX1ABC>APRS:!9160.00N/02100.00E_t020", time.Now().UTC(), 52.5, 21, 10)
	if err != nil {
		t.Fatal(err)
	}
	if !f.IsWX || f.Eligible || f.Reason != "invalid position" || len(f.Data) != 0 {
		t.Fatalf("invalid-position WX frame should remain visible without supplying measurements: %+v", f)
	}
}

func TestWeatherFrameDropsInvalidPressureAndWindButKeepsValidFields(t *testing.T) {
	f, err := ParseTNC2("WX1ABC>APRS:!5230.00N/02100.00E_361/251t020h50b00000", time.Now().UTC(), 52.5, 21, 10)
	if err != nil {
		t.Fatal(err)
	}
	if !f.IsWX || !f.Eligible {
		t.Fatalf("valid weather fields should keep the frame eligible: %+v", f)
	}
	seen := map[string]bool{}
	for _, item := range f.Data {
		seen[item.Parameter] = true
	}
	if !seen["temperature"] || !seen["humidity"] {
		t.Fatalf("valid fields were lost: %+v", f.Data)
	}
	for _, parameter := range []string{"pressure", "wind_direction", "wind_speed"} {
		if seen[parameter] {
			t.Errorf("invalid %s was accepted: %+v", parameter, f.Data)
		}
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

func TestStationBlacklistUsesExactCallsignAndKeepsFrameData(t *testing.T) {
	frame := Frame{Raw: "WX1ABC>APRS:!5230.00N/02100.00E_t020", Station: "WX1ABC", IsWX: true, Eligible: true}
	blocked := ApplyBlacklist(frame, []string{" wx1abc "})
	if !blocked.Blacklisted || !blocked.StationBlacklisted || blocked.Eligible || blocked.Reason != "station is on the APRS blacklist" {
		t.Fatalf("unexpected station-blacklist result: %+v", blocked)
	}
	partial := ApplyBlacklist(frame, []string{"WX1AB"})
	if partial.Blacklisted || !partial.Eligible {
		t.Fatalf("blacklist matched a partial callsign: %+v", partial)
	}
}

func TestFrameBlacklistUsesFingerprintAndKeepsFrameVisible(t *testing.T) {
	raw := "WX1ABC>APRS:!5230.00N/02100.00E_t020"
	frame := Frame{Raw: raw, Station: "WX1ABC", IsWX: true, Eligible: true}
	blocked := ApplyFrameBlacklist(frame, []string{FrameFingerprint(raw)})
	if !blocked.Blacklisted || !blocked.FrameBlacklisted || blocked.FrameID != FrameFingerprint(raw) || blocked.Eligible {
		t.Fatalf("unexpected frame-blacklist result: %+v", blocked)
	}
	if frame.Blacklisted || !frame.Eligible {
		t.Fatalf("applying a blacklist mutated the stored frame: %+v", frame)
	}
}
