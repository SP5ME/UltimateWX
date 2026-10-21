package warnings

import (
	"fmt"
	"io"
	"strconv"
	"strings"
)

func readLimitedBody(body io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("response exceeds %d-byte limit", limit)
	}
	return data, nil
}

func parseProbability(value string) (int, bool) {
	probability, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || probability < 0 || probability > 100 {
		return 0, false
	}
	return probability, true
}
