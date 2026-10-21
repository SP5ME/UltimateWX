package warnings

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestResolveCountyTERYTBuildsWGS84QueryAndParsesCounty(t *testing.T) {
	client := &http.Client{Transport: responseRoundTripper(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.Header.Get("Accept") != "text/plain" {
			t.Errorf("Accept = %q, want text/plain", r.Header.Get("Accept"))
		}
		query := r.URL.Query()
		if query.Get("request") != "GetCountyByXY" || query.Get("result") != "teryt" || query.Get("xy") != "21.012300,52.229700,4326" {
			t.Errorf("unexpected ULDK query: %s", r.URL.RawQuery)
		}
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Header: make(http.Header), Body: io.NopCloser(strings.NewReader("0\n1465|Powiat Warszawski\n")), Request: r}, nil
	})}

	code, err := resolveCountyTERYT(context.Background(), client, "https://uldk.test/", 52.2297, 21.0123)
	if err != nil {
		t.Fatalf("resolveCountyTERYT() error = %v", err)
	}
	if code != "1465" {
		t.Fatalf("resolveCountyTERYT() = %q, want 1465", code)
	}
}

func TestResolveCountyTERYTRejectsInvalidResponses(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
	}{
		{name: "not found", status: http.StatusOK, body: "1\n"},
		{name: "short code", status: http.StatusOK, body: "0\n146|County\n"},
		{name: "non numeric code", status: http.StatusOK, body: "0\n14A5|County\n"},
		{name: "server error", status: http.StatusServiceUnavailable, body: "unavailable"},
		{name: "oversized response", status: http.StatusOK, body: fmt.Sprintf("0\n1465|%s", strings.Repeat("x", 64<<10))},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := &http.Client{Transport: responseRoundTripper(func(r *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: test.status, Status: fmt.Sprintf("%d %s", test.status, http.StatusText(test.status)), Header: make(http.Header), Body: io.NopCloser(strings.NewReader(test.body)), Request: r}, nil
			})}
			code, err := resolveCountyTERYT(context.Background(), client, "https://uldk.test/", 52, 21)
			if err == nil {
				t.Fatalf("resolveCountyTERYT() = %q, want an error", code)
			}
			if test.name == "not found" && !strings.Contains(err.Error(), ErrCountyNotFound.Error()) {
				t.Fatalf("not-found error = %v, want ErrCountyNotFound", err)
			}
		})
	}
}
