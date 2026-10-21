package bme280

import (
	"errors"
	"testing"
	"time"
)

func TestWaitForStatusClearsRequestedBit(t *testing.T) {
	values := []byte{0x09, 0x08, 0x00}
	calls := 0
	err := waitForStatus(func() (byte, error) {
		value := values[calls]
		calls++
		return value, nil
	}, 0x09, time.Second)
	if err != nil {
		t.Fatalf("waitForStatus returned error: %v", err)
	}
	if calls != 3 {
		t.Fatalf("status read %d times, want 3", calls)
	}
}

func TestWaitForStatusTimesOut(t *testing.T) {
	err := waitForStatus(func() (byte, error) { return 0x08, nil }, 0x08, 5*time.Millisecond)
	if err == nil {
		t.Fatal("waitForStatus did not time out while the requested bit stayed set")
	}
}

func TestWaitForStatusPropagatesReadError(t *testing.T) {
	want := errors.New("I2C read failed")
	err := waitForStatus(func() (byte, error) { return 0, want }, 0x08, time.Second)
	if !errors.Is(err, want) {
		t.Fatalf("waitForStatus error = %v, want %v", err, want)
	}
}
