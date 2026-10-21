package warnings

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const DefaultULDKEndpoint = "https://uldk.gugik.gov.pl/"

var ErrCountyNotFound = fmt.Errorf("county not found")

// ResolveCountyTERYT finds the Polish county containing a WGS84 station point.
func ResolveCountyTERYT(ctx context.Context, latitude, longitude float64) (string, error) {
	client := &http.Client{Timeout: 6 * time.Second}
	return resolveCountyTERYT(ctx, client, DefaultULDKEndpoint, latitude, longitude)
}

func resolveCountyTERYT(ctx context.Context, client *http.Client, endpoint string, latitude, longitude float64) (string, error) {
	query := url.Values{}
	query.Set("request", "GetCountyByXY")
	query.Set("xy", fmt.Sprintf("%.6f,%.6f,4326", longitude, latitude))
	query.Set("result", "teryt")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+query.Encode(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "text/plain")
	req.Header.Set("User-Agent", "UltimateWX/0.1 (station county lookup)")
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("GUGiK ULDK request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GUGiK ULDK returned %s", resp.Status)
	}
	body, err := readLimitedBody(resp.Body, 64<<10)
	if err != nil {
		return "", fmt.Errorf("read GUGiK ULDK response: %w", err)
	}
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	if len(lines) < 2 || strings.TrimSpace(lines[0]) != "0" {
		return "", fmt.Errorf("GUGiK ULDK did not find a county for the station coordinates: %w", ErrCountyNotFound)
	}
	code := strings.TrimSpace(strings.SplitN(strings.TrimSpace(lines[1]), "|", 2)[0])
	if len(code) != 4 {
		return "", fmt.Errorf("GUGiK ULDK returned an invalid county TERYT code")
	}
	for _, digit := range code {
		if digit < '0' || digit > '9' {
			return "", fmt.Errorf("GUGiK ULDK returned an invalid county TERYT code")
		}
	}
	return code, nil
}
