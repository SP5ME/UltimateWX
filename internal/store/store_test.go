package store

import (
	"testing"
	"time"

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
