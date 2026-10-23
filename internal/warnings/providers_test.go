package warnings

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

type responseRoundTripper func(*http.Request) (*http.Response, error)

func (f responseRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func staticHTTPClient(body string) *http.Client {
	return &http.Client{Transport: responseRoundTripper(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})}
}

func TestDWDFetchMatchesStationAgainstCAPPolygon(t *testing.T) {
	const feed = `<feed xmlns="http://www.w3.org/2005/Atom"><entry><content type="application/xml"><alert xmlns="urn:oasis:names:tc:emergency:cap:1.2"><identifier>dwd-1</identifier><sender>dwd@example.test</sender><sent>2026-10-21T10:00:00Z</sent><status>Actual</status><msgType>Alert</msgType><info><language>de-DE</language><event>Wind</event><severity>Severe</severity><onset>2026-10-21T11:00:00Z</onset><expires>2026-10-21T15:00:00Z</expires><headline>Starker Wind</headline><area><areaDesc>Testgebiet</areaDesc><polygon>50,19 50,21 52,21 52,19 50,19</polygon></area></info></alert></content></entry></feed>`
	provider := DWD{Client: staticHTTPClient(feed), Endpoint: "https://example.test/feed", Locale: "de"}
	alerts, err := provider.Fetch(context.Background(), 51, 20)
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if len(alerts) != 1 {
		t.Fatalf("Fetch() returned %d alerts, want 1", len(alerts))
	}
	alert := alerts[0]
	if alert.Source != "DWD" || alert.Event != "Wind" || alert.Severity != 3 || alert.Area != "Testgebiet" {
		t.Errorf("unexpected alert fields: %+v", alert)
	}
	if alert.ValidFrom.UTC().Format(time.RFC3339) != "2026-10-21T11:00:00Z" || alert.ValidTo.UTC().Format(time.RFC3339) != "2026-10-21T15:00:00Z" {
		t.Errorf("unexpected alert validity: %s - %s", alert.ValidFrom, alert.ValidTo)
	}
}

func TestDWDFetchIgnoresStationOutsideCAPPolygon(t *testing.T) {
	const feed = `<feed xmlns="http://www.w3.org/2005/Atom"><entry><content type="application/xml"><alert xmlns="urn:oasis:names:tc:emergency:cap:1.2"><identifier>dwd-1</identifier><sender>dwd@example.test</sender><sent>2026-10-21T10:00:00Z</sent><status>Actual</status><msgType>Alert</msgType><info><language>de-DE</language><event>Wind</event><severity>Severe</severity><onset>2026-10-21T11:00:00Z</onset><expires>2026-10-21T15:00:00Z</expires><area><areaDesc>Testgebiet</areaDesc><polygon>50,19 50,21 52,21 52,19 50,19</polygon></area></info></alert></content></entry></feed>`
	alerts, err := (DWD{Client: staticHTTPClient(feed), Endpoint: "https://example.test/feed"}).Fetch(context.Background(), 48, 16)
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if len(alerts) != 0 {
		t.Fatalf("Fetch() returned %d alerts outside the CAP polygon, want 0", len(alerts))
	}
}

func TestMeteoAlarmParseNotificationProducesLicensedCAPAlert(t *testing.T) {
	for _, test := range []struct {
		name            string
		probability     string
		wantProbability *int
	}{
		{name: "zero is a reported value", probability: "0", wantProbability: intPointer(0)},
		{name: "invalid is unavailable", probability: "101"},
	} {
		t.Run(test.name, func(t *testing.T) {
			capDocument := fmt.Sprintf(`<alert><identifier>cap-1</identifier><sender>wmo@example.test</sender><sent>2030-01-10T10:00:00Z</sent><status>Actual</status><msgType>Alert</msgType><info><language>en-US</language><event>Wind</event><severity>Severe</severity><onset>2030-01-10T11:00:00Z</onset><expires>2030-01-10T15:00:00Z</expires><headline>Strong wind</headline><area><areaDesc>Test area</areaDesc><polygon>50,19 50,21 52,21 52,19 50,19</polygon></area><parameter><valueName>PROBABILITY</valueName><value>%s</value></parameter></info></alert>`, test.probability)
			client := &http.Client{Transport: responseRoundTripper(func(r *http.Request) (*http.Response, error) {
				var body string
				switch r.URL.Path {
				case "/geometry":
					body = `{"type":"Polygon","coordinates":[[[19,50],[21,50],[21,52],[19,52],[19,50]]]}`
				case "/cap":
					body = capDocument
				default:
					return nil, fmt.Errorf("unexpected MeteoAlarm request path %q", r.URL.Path)
				}
				return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
			})}
			payload := []byte(`{"type":"Feature","properties":{"alertId":"meteo-1","countryCode":"PL","rights":"Unrestricted with attribution","indexArea":0,"indexInfo":0},"links":[{"rel":"license","href":"https://creativecommons.org/licenses/by/4.0/"},{"rel":"geometry","href":"https://8.8.8.8/geometry"},{"rel":"xml","href":"https://8.8.8.8/cap"}]}`)
			result, err := (MeteoAlarm{Client: client}).ParseNotification(context.Background(), payload, 51, 20, "en")
			if err != nil {
				t.Fatalf("ParseNotification() error = %v", err)
			}
			if result.Alert == nil || result.Alert.ID != "meteo-1" || result.Alert.Event != "Wind" || result.Alert.Severity != 3 || result.Alert.Area != "Test area" {
				t.Fatalf("unexpected normalized alert: %+v", result.Alert)
			}
			if (result.Alert.Probability == nil) != (test.wantProbability == nil) {
				t.Fatalf("probability = %v, want %v", result.Alert.Probability, test.wantProbability)
			}
			if test.wantProbability != nil && *result.Alert.Probability != *test.wantProbability {
				t.Fatalf("probability = %d, want %d", *result.Alert.Probability, *test.wantProbability)
			}
		})
	}
}

func TestMeteoAlarmRejectsUnverifiedRightsBeforeFetchingLinks(t *testing.T) {
	requested := false
	client := &http.Client{Transport: responseRoundTripper(func(r *http.Request) (*http.Response, error) {
		requested = true
		return nil, fmt.Errorf("unexpected request for %s", r.URL)
	})}
	payload := []byte(`{"type":"Feature","properties":{"alertId":"meteo-1","rights":"restricted"},"links":[{"rel":"license","href":"https://creativecommons.org/licenses/by/4.0/"},{"rel":"geometry","href":"https://8.8.8.8/geometry"},{"rel":"xml","href":"https://8.8.8.8/cap"}]}`)
	if _, err := (MeteoAlarm{Client: client}).ParseNotification(context.Background(), payload, 51, 20, "en"); err == nil {
		t.Fatal("expected unverified rights to be rejected")
	}
	if requested {
		t.Fatal("MeteoAlarm links were fetched before reuse rights were verified")
	}
}

func TestValidateMeteoAlarmURLRejectsUnsafeTargets(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{name: "plain HTTP", url: "http://8.8.8.8/alert"},
		{name: "loopback IPv4", url: "https://127.0.0.1/alert"},
		{name: "private IPv4", url: "https://192.168.1.10/alert"},
		{name: "shared carrier IPv4", url: "https://100.64.0.1/alert"},
		{name: "documentation IPv4", url: "https://198.51.100.10/alert"},
		{name: "benchmark IPv4", url: "https://198.18.0.1/alert"},
		{name: "multicast IPv4", url: "https://224.0.0.1/alert"},
		{name: "link-local IPv4", url: "https://169.254.1.1/alert"},
		{name: "loopback IPv6", url: "https://[::1]/alert"},
		{name: "mapped private IPv4", url: "https://[::ffff:192.168.1.10]/alert"},
		{name: "IPv6 documentation", url: "https://[2001:db8::1]/alert"},
		{name: "IPv6 NAT64 local translation", url: "https://[64:ff9b:1::1]/alert"},
		{name: "nonstandard port", url: "https://8.8.8.8:8443/alert"},
		{name: "embedded credentials", url: "https://user:pass@8.8.8.8/alert"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parsed, err := url.Parse(test.url)
			if err != nil {
				t.Fatal(err)
			}
			if err := validateMeteoAlarmURL(parsed); err == nil {
				t.Fatalf("validateMeteoAlarmURL(%q) unexpectedly succeeded", test.url)
			}
		})
	}
}

func TestValidateMeteoAlarmURLAcceptsPublicHTTPSIP(t *testing.T) {
	for _, rawURL := range []string{"https://8.8.8.8/alert", "https://[2001:4860:4860::8888]/alert"} {
		t.Run(rawURL, func(t *testing.T) {
			parsed, err := url.Parse(rawURL)
			if err != nil {
				t.Fatal(err)
			}
			if err := validateMeteoAlarmURL(parsed); err != nil {
				t.Fatalf("validateMeteoAlarmURL() error = %v", err)
			}
		})
	}
}

func intPointer(value int) *int { return &value }

func TestMeteoGeometryContainsHonorsPolygonHoles(t *testing.T) {
	geometry := meteoGeometry{
		Type:        "Polygon",
		Coordinates: []byte(`[[[19,50],[21,50],[21,52],[19,52],[19,50]],[[19.5,50.5],[20.5,50.5],[20.5,51.5],[19.5,51.5],[19.5,50.5]]]`),
	}

	inside, err := meteoGeometryContains(geometry, 50.25, 20)
	if err != nil || !inside {
		t.Fatalf("outer polygon point: inside=%v error=%v, want true", inside, err)
	}
	inside, err = meteoGeometryContains(geometry, 51, 20)
	if err != nil || inside {
		t.Fatalf("hole point: inside=%v error=%v, want false", inside, err)
	}
}

func TestGeoSphereEpochAcceptsNumericAndQuotedSeconds(t *testing.T) {
	for _, input := range []string{`1792580400`, `"1792580400"`} {
		t.Run(strings.Trim(input, `"`), func(t *testing.T) {
			got, ok := geoSphereEpoch([]byte(input))
			if !ok || got.Unix() != 1792580400 || got.Location() != time.UTC {
				t.Fatalf("geoSphereEpoch(%s) = %s, %v", input, got, ok)
			}
		})
	}
	for _, input := range []string{"null", `"not-a-timestamp"`, `-1`} {
		if _, ok := geoSphereEpoch([]byte(input)); ok {
			t.Errorf("geoSphereEpoch(%s) accepted invalid value", input)
		}
	}
}

func TestGeoSphereFetchParsesWarningAndUsesConfiguredCoordinates(t *testing.T) {
	const response = `{"type":"FeatureCollection","properties":{"location":{"properties":{"name":"Wien"}},"warnings":[{"properties":{"warnid":123,"chgid":4,"verlaufid":2,"warntypid":1,"warnstufeid":2,"text":"Starker Wind","auswirkungen":"Äste können brechen","empfehlungen":"Vorsicht","create":"2026-10-21T08:00:00Z","rawinfo":{"start":1792576800,"end":"1792591200"}}}]}}`
	var requestURL *url.URL
	client := &http.Client{Transport: responseRoundTripper(func(r *http.Request) (*http.Response, error) {
		requestURL = r.URL
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Header: make(http.Header), Body: io.NopCloser(strings.NewReader(response)), Request: r}, nil
	})}

	alerts, err := (GeoSphere{Client: client, Locale: "pl-PL"}).Fetch(context.Background(), 48.2082, 16.3738)
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if requestURL == nil {
		t.Fatal("Fetch() did not make a request")
	}
	query := requestURL.Query()
	if query.Get("lat") != "48.208200" || query.Get("lon") != "16.373800" || query.Get("lang") != "en" {
		t.Errorf("unexpected GeoSphere request query: %v", query)
	}
	if len(alerts) != 1 {
		t.Fatalf("Fetch() returned %d alerts, want 1", len(alerts))
	}
	alert := alerts[0]
	if alert.Source != "GeoSphere Austria" || alert.Area != "Wien" || alert.Severity != 2 || alert.SeverityLabel != "pomarańczowy" || alert.Event != "silny wiatr" {
		t.Errorf("unexpected alert fields: %+v", alert)
	}
	if alert.ID != "geosphere:123:4:2:2026-10-21T10:00:00Z" {
		t.Errorf("unexpected alert ID %q", alert.ID)
	}
}

func TestGeoSphereFetchReportsOutsideCoverage(t *testing.T) {
	client := staticHTTPClient(`{"type":"Error","msg":"Could not find municipal area for coordinates"}`)
	_, err := (GeoSphere{Client: client}).Fetch(context.Background(), 0, 0)
	if !errors.Is(err, ErrGeoSphereOutsideCoverage) {
		t.Fatalf("Fetch() error = %v, want ErrGeoSphereOutsideCoverage", err)
	}
}

func TestDWDUpdateAndCancelSuppressReferencedAlert(t *testing.T) {
	for _, messageType := range []string{"Update", "Cancel"} {
		t.Run(messageType, func(t *testing.T) {
			feed := fmt.Sprintf(`<feed xmlns="http://www.w3.org/2005/Atom"><entry><content type="application/xml"><alert xmlns="urn:oasis:names:tc:emergency:cap:1.2"><identifier>new</identifier><sender>dwd@example.test</sender><sent>2026-10-21T10:00:00Z</sent><status>Actual</status><msgType>%s</msgType><references>dwd@example.test,old,2026-10-21T09:00:00Z</references><info><language>de-DE</language><event>Wind</event><severity>Severe</severity><onset>2026-10-21T11:00:00Z</onset><expires>2026-10-21T15:00:00Z</expires><area><areaDesc>Testgebiet</areaDesc><polygon>50,19 50,21 52,21 52,19 50,19</polygon></area></info></alert></content></entry><entry><content type="application/xml"><alert xmlns="urn:oasis:names:tc:emergency:cap:1.2"><identifier>old</identifier><sender>dwd@example.test</sender><sent>2026-10-21T09:00:00Z</sent><status>Actual</status><msgType>Alert</msgType><info><language>de-DE</language><event>Wind</event><severity>Severe</severity><onset>2026-10-21T10:00:00Z</onset><expires>2026-10-21T15:00:00Z</expires><area><areaDesc>Testgebiet</areaDesc><polygon>50,19 50,21 52,21 52,19 50,19</polygon></area></info></alert></content></entry></feed>`, messageType)
			alerts, err := (DWD{Client: staticHTTPClient(feed), Endpoint: "https://example.test/feed"}).Fetch(context.Background(), 51, 20)
			if err != nil {
				t.Fatalf("Fetch() error = %v", err)
			}
			want := 0
			if messageType == "Update" {
				want = 1
			}
			if len(alerts) != want {
				t.Fatalf("Fetch() returned %d alerts, want %d for %s", len(alerts), want, messageType)
			}
			if messageType == "Update" && !strings.HasPrefix(alerts[0].ID, "new|") {
				t.Errorf("Update returned superseded alert %q", alerts[0].ID)
			}
		})
	}
}

func TestIMGWFetchNormalizesPolishAlertAndSkipsInvalidRecords(t *testing.T) {
	const response = `[
		{"id":" 42 ","nazwa_zdarzenia":" Upał ","stopien":"2","prawdopodobienstwo":"75","obowiazuje_od":"2026-01-10 12:00:00","obowiazuje_do":"2026-01-10 18:00:00","opublikowano":"2026-01-10 10:00:00","tresc":"  Ostrzeżenie  ","komentarz":"  Zachowaj ostrożność ","biuro":" IMGW ","teryt":[" 1465 ",""]},
		{"id":"bad-severity","stopien":"4","obowiazuje_od":"2026-01-10 12:00:00","obowiazuje_do":"2026-01-10 18:00:00"},
		{"id":"bad-time","stopien":"1","obowiazuje_od":"not-a-date","obowiazuje_do":"2026-01-10 18:00:00"},
		{"id":"bad-period","stopien":"1","obowiazuje_od":"2026-01-10 18:00:00","obowiazuje_do":"2026-01-10 12:00:00"}
	]`
	alerts, err := (IMGW{Client: staticHTTPClient(response), Endpoint: "https://example.test/warnings"}).Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if len(alerts) != 1 {
		t.Fatalf("Fetch() returned %d alerts, want 1", len(alerts))
	}
	alert := alerts[0]
	if alert.ID != "42" || alert.Event != "Upał" || alert.Description != "Ostrzeżenie" || alert.Comment != "Zachowaj ostrożność" || alert.Office != "IMGW" || alert.Probability == nil || *alert.Probability != 75 {
		t.Errorf("unexpected normalized alert: %+v", alert)
	}
	if alert.ValidFrom.Format(time.RFC3339) != "2026-01-10T11:00:00Z" || alert.ValidTo.Format(time.RFC3339) != "2026-01-10T17:00:00Z" {
		t.Errorf("IMGW local time not converted to UTC: %s - %s", alert.ValidFrom, alert.ValidTo)
	}
	if !MatchesArea(alert, []string{" 1465 "}) || MatchesArea(alert, []string{"1466"}) || MatchesArea(alert, nil) {
		t.Error("MatchesArea() did not require the exact configured TERYT county")
	}
}

func TestIMGWFetchAcceptsNoWarningsMessage(t *testing.T) {
	alerts, err := (IMGW{Client: staticHTTPClient(`{"message":"Brak ostrzeżeń meteorologicznych"}`), Endpoint: "https://example.test/warnings"}).Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if len(alerts) != 0 {
		t.Fatalf("Fetch() returned %d alerts, want none", len(alerts))
	}
}

func TestParseProbabilityAcceptsOnlyPercentRange(t *testing.T) {
	for _, test := range []struct {
		input string
		want  int
		valid bool
	}{
		{input: "0", want: 0, valid: true},
		{input: " 100 ", want: 100, valid: true},
		{input: "55", want: 55, valid: true},
		{input: "-1", want: 0},
		{input: "101", want: 0},
		{input: "unknown", want: 0},
	} {
		got, valid := parseProbability(test.input)
		if got != test.want || valid != test.valid {
			t.Errorf("parseProbability(%q) = (%d, %t), want (%d, %t)", test.input, got, valid, test.want, test.valid)
		}
	}
}

func TestIMGWLocalTimeUsesCETAndCEST(t *testing.T) {
	location, err := time.LoadLocation("Europe/Warsaw")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		local string
		utc   string
	}{
		{local: "2026-01-10 12:00:00", utc: "2026-01-10T11:00:00Z"},
		{local: "2026-07-10 12:00:00", utc: "2026-07-10T10:00:00Z"},
	} {
		got, err := parseLocalTime(test.local, location)
		if err != nil {
			t.Errorf("parseLocalTime(%q): %v", test.local, err)
			continue
		}
		if formatted := got.UTC().Format(time.RFC3339); formatted != test.utc {
			t.Errorf("parseLocalTime(%q) = %s, want %s", test.local, formatted, test.utc)
		}
	}
}
