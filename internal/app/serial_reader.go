package app

import (
	"bufio"
	"context"
	"io"
)

func scanSerialLines(ctx context.Context, reader io.ReadCloser, consume func([]byte)) error {
	watcherDone := make(chan struct{})
	stopWatcher := make(chan struct{})
	go func() {
		defer close(watcherDone)
		select {
		case <-ctx.Done():
			_ = reader.Close()
		case <-stopWatcher:
		}
	}()

	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 4096), 1<<20)
	for scanner.Scan() {
		consume(scanner.Bytes())
	}
	scanErr := scanner.Err()
	close(stopWatcher)
	<-watcherDone
	_ = reader.Close()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return scanErr
}
