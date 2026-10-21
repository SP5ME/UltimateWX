package store

import (
	"testing"
	"time"

	"github.com/ultimatewx/ultimatewx/internal/aprs"
	"github.com/ultimatewx/ultimatewx/internal/weather"
)

func TestObservationPersistenceAndRetention(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	_ = s.AddObservation(weather.Observation{Parameter: "temperature", Value: 1, ObservedAt: time.Now().UTC(), Source: "test"})
	_ = s.AddObservation(weather.Observation{Parameter: "temperature", Value: 2, ObservedAt: time.Now().Add(-48 * time.Hour), Source: "test"})
	s.Prune(1)
	if got := s.Observations(1); len(got) != 1 {
		t.Fatalf("expected one fresh observation, got %d", len(got))
	}
	reopened, err := Open(s.dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(reopened.Observations(1)) != 1 {
		t.Fatal("pruned history did not persist")
	}
}

func TestLatestByDeviceKeepsParametersAndDevicesIndependent(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	observations := []weather.Observation{
		{Parameter: "temperature", Value: 19, ObservedAt: now.Add(-2 * time.Minute), Source: "mqtt", DeviceID: "kitchen", Quality: "measured"},
		{Parameter: "humidity", Value: 55, ObservedAt: now.Add(-time.Minute), Source: "mqtt", DeviceID: "kitchen", Quality: "measured"},
		{Parameter: "temperature", Value: 12, ObservedAt: now.Add(-time.Minute), Source: "mqtt", DeviceID: "roof", Quality: "measured"},
		{Parameter: "temperature", Value: 18, ObservedAt: now.Add(-time.Minute), Source: "mqtt", DeviceID: "kitchen", Quality: "measured"},
	}
	for _, observation := range observations {
		if err := s.AddObservation(observation); err != nil {
			t.Fatal(err)
		}
	}

	latest := s.LatestByDevice(nil)
	got := make(map[string]float64, len(latest))
	for _, observation := range latest {
		got[observation.DeviceID+"|"+observation.Parameter] = observation.Value
	}
	want := map[string]float64{
		"kitchen|temperature": 18,
		"kitchen|humidity":    55,
		"roof|temperature":    12,
	}
	if len(got) != len(want) {
		t.Fatalf("latest reading count = %d, want %d (%#v)", len(got), len(want), got)
	}
	for key, value := range want {
		if got[key] != value {
			t.Errorf("latest reading %q = %v, want %v", key, got[key], value)
		}
	}
}

func TestAPRSBlacklistsExcludeMeasurementsWithoutDeletingFrames(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	frames := []aprs.Frame{
		{Raw: "station-blocked", Station: "WX1ABC", ReceivedAt: now, IsWX: true, Eligible: true, FrameID: "station-frame"},
		{Raw: "frame-blocked", Station: "WX2ABC", ReceivedAt: now, IsWX: true, Eligible: true, FrameID: "hash-blocked"},
		{Raw: "allowed", Station: "WX3ABC", ReceivedAt: now, IsWX: true, Eligible: true, FrameID: "allowed-frame"},
	}
	for _, frame := range frames {
		if err := s.AddFrame(frame); err != nil {
			t.Fatal(err)
		}
		if err := s.AddObservation(weather.Observation{Parameter: "temperature", Value: 20, ObservedAt: now, Source: "aprs", Station: frame.Station, FrameID: frame.FrameID, Quality: "measured"}); err != nil {
			t.Fatal(err)
		}
	}

	stationBlacklist := []string{"wx1abc"}
	frameBlacklist := []string{"hash-blocked"}
	latest := s.LatestByDeviceExcludingAPRS(stationBlacklist, frameBlacklist)
	if len(latest) != 1 || latest[0].Station != "WX3ABC" {
		t.Fatalf("blacklisted APRS observations remained in current readings: %+v", latest)
	}
	stats := s.StatsExcludingAPRS(stationBlacklist, frameBlacklist)
	if stats["observations"] != 1 || stats["aprs_frames"] != 3 || stats["aprs_wx_frames"] != 3 || stats["aprs_eligible_frames"] != 1 {
		t.Fatalf("unexpected blacklist statistics: %+v", stats)
	}
	if got := s.Frames(10, true); len(got) != 3 {
		t.Fatalf("blacklisted frames should remain visible in the monitor, got %d", len(got))
	}
}
