package app

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"
)

func TestScanSerialLinesStopsWhenContextIsCancelled(t *testing.T) {
	reader, writer := io.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	finished := make(chan error, 1)
	go func() {
		finished <- scanSerialLines(ctx, reader, func([]byte) {})
	}()

	cancel()
	select {
	case err := <-finished:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("scanSerialLines error = %v, want context cancellation", err)
		}
	case <-time.After(time.Second):
		t.Fatal("serial scan did not stop after cancellation")
	}
	_ = writer.Close()
}

func TestScanSerialLinesConsumesNewlineDelimitedSamples(t *testing.T) {
	reader, writer := io.Pipe()
	ctx := context.Background()
	got := make(chan string, 1)
	finished := make(chan error, 1)
	go func() {
		finished <- scanSerialLines(ctx, reader, func(line []byte) { got <- string(line) })
	}()
	if _, err := io.WriteString(writer, `{"temperature_c":12.5}`+"\n"); err != nil {
		t.Fatal(err)
	}
	_ = writer.Close()
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
	if line := <-got; line != `{"temperature_c":12.5}` {
		t.Fatalf("sample = %q", line)
	}
}
