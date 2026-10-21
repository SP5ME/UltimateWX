package warnings

import (
	"context"
	"encoding/xml"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultDWDAtomEndpoint  = "https://www.dwd.de/DWD/warnungen/cap-feed/de/atom.xml"
	dwdEnglishAtomEndpoint  = "https://www.dwd.de/DWD/warnungen/cap-feed/en/atom.xml"
	dwdMultilingualEndpoint = "https://www.dwd.de/DWD/warnungen/cap-feed/MUL/atom-event-MUL.xml"
)

// DWD reads the official municipality-level CAP status feed. Alerts without
// usable polygons are ignored so that a station is never matched by a broad
// bounding box or an unrelated administrative code.
type DWD struct {
	Client   *http.Client
	Endpoint string
	Locale   string
}

type atomFeed struct {
	Entries []struct {
		Links []struct {
			Rel  string `xml:"rel,attr"`
			Type string `xml:"type,attr"`
			Href string `xml:"href,attr"`
		} `xml:"link"`
		Content struct {
			Type string `xml:"type,attr"`
			Src  string `xml:"src,attr"`
			Body []byte `xml:",innerxml"`
		} `xml:"content"`
	} `xml:"entry"`
}

type capAlert struct {
	Identifier string    `xml:"identifier"`
	Sender     string    `xml:"sender"`
	Sent       string    `xml:"sent"`
	Status     string    `xml:"status"`
	MsgType    string    `xml:"msgType"`
	References []string  `xml:"references"`
	Info       []capInfo `xml:"info"`
}

type capInfo struct {
	Language    string    `xml:"language"`
	Event       string    `xml:"event"`
	Severity    string    `xml:"severity"`
	Effective   string    `xml:"effective"`
	Onset       string    `xml:"onset"`
	Expires     string    `xml:"expires"`
	Headline    string    `xml:"headline"`
	Description string    `xml:"description"`
	Instruction string    `xml:"instruction"`
	Area        []capArea `xml:"area"`
	Parameters  []struct {
		Name  string `xml:"valueName"`
		Value string `xml:"value"`
	} `xml:"parameter"`
}

type capArea struct {
	Description string   `xml:"areaDesc"`
	Polygons    []string `xml:"polygon"`
	Geocodes    []struct {
		Name  string `xml:"valueName"`
		Value string `xml:"value"`
	} `xml:"geocode"`
}

func (p DWD) Fetch(ctx context.Context, latitude, longitude float64) ([]Alert, error) {
	endpoint := p.Endpoint
	if endpoint == "" {
		endpoint = dwdFeedEndpoint(dwdLanguage(p.Locale))
	}
	client := p.Client
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second}
	}
	feed, err := fetchXML(ctx, client, endpoint, "application/atom+xml, application/xml")
	if err != nil {
		return nil, fmt.Errorf("DWD CAP feed: %w", err)
	}
	var doc atomFeed
	if err := xml.Unmarshal(feed, &doc); err != nil {
		return nil, fmt.Errorf("decode DWD Atom feed: %w", err)
	}
	now := time.Now().UTC()
	alerts := make([]Alert, 0, len(doc.Entries))
	var documentErrors []string
	capAlerts := make([]capAlert, 0, len(doc.Entries))
	for _, entry := range doc.Entries {
		capXML := entry.Content.Body
		var documentErr error
		if len(bytesTrimSpace(capXML)) == 0 && strings.TrimSpace(entry.Content.Src) != "" {
			capXML, documentErr = fetchDWDLink(ctx, client, endpoint, entry.Content.Src)
		}
		if len(bytesTrimSpace(capXML)) == 0 {
			for _, link := range entry.Links {
				if strings.Contains(strings.ToLower(link.Type), "xml") && (link.Rel == "enclosure" || link.Rel == "alternate") {
					var linkErr error
					capXML, linkErr = fetchDWDLink(ctx, client, endpoint, link.Href)
					if linkErr == nil {
						documentErr = nil
						break
					}
					documentErr = linkErr
				}
			}
		}
		if len(capXML) == 0 {
			if documentErr != nil {
				documentErrors = append(documentErrors, documentErr.Error())
			} else {
				documentErrors = append(documentErrors, "Atom entry has no CAP document")
			}
			continue
		}
		var cap capAlert
		if err := xml.Unmarshal(capXML, &cap); err != nil {
			documentErrors = append(documentErrors, err.Error())
			continue
		}
		capAlerts = append(capAlerts, cap)
	}
	if len(documentErrors) > 0 {
		return nil, fmt.Errorf("read DWD CAP alert documents: %s", strings.Join(documentErrors, "; "))
	}
	referenced := make(map[string]bool)
	for _, cap := range capAlerts {
		if cap.Status != "Actual" || (cap.MsgType != "Update" && cap.MsgType != "Cancel") {
			continue
		}
		for _, value := range cap.References {
			for _, reference := range strings.Fields(value) {
				parts := strings.SplitN(reference, ",", 3)
				if len(parts) == 3 && strings.TrimSpace(parts[0]) != "" && strings.TrimSpace(parts[1]) != "" && strings.TrimSpace(parts[2]) != "" {
					referenced[capReferenceKey(parts[0], parts[1])] = true
				}
			}
		}
	}
	for _, cap := range capAlerts {
		if cap.Status != "Actual" || cap.MsgType == "Cancel" || strings.TrimSpace(cap.Identifier) == "" || referenced[capReferenceKey(cap.Sender, cap.Identifier)] {
			continue
		}
		sent, _ := time.Parse(time.RFC3339, strings.TrimSpace(cap.Sent))
		for _, info := range selectCAPInfo(cap.Info, dwdLanguage(p.Locale)) {
			severity := dwdSeverity(info.Severity)
			validFrom := parseCAPTime(info.Onset)
			if validFrom.IsZero() {
				validFrom = parseCAPTime(info.Effective)
			}
			if validFrom.IsZero() {
				validFrom = sent
			}
			validTo := parseCAPTime(info.Expires)
			if severity == 0 || validFrom.IsZero() || validTo.IsZero() || !validTo.After(validFrom) {
				continue
			}
			var areaNames []string
			matches := false
			for _, area := range info.Area {
				var include, exclude bool
				for _, polygon := range area.Polygons {
					if polygonContains(polygon, latitude, longitude) {
						include = true
					}
				}
				for _, code := range area.Geocodes {
					if strings.EqualFold(strings.TrimSpace(code.Name), "EXCLUDE_POLYGON") && polygonContains(code.Value, latitude, longitude) {
						exclude = true
					}
				}
				if include && !exclude {
					matches = true
					if name := strings.TrimSpace(area.Description); name != "" {
						areaNames = append(areaNames, name)
					}
				}
			}
			if !matches {
				continue
			}
			var probability *int
			attribution := "© Deutscher Wetterdienst"
			for _, parameter := range info.Parameters {
				if strings.EqualFold(strings.TrimSpace(parameter.Name), "PROBABILITY") {
					if value, ok := parseProbability(parameter.Value); ok {
						probability = &value
					}
				}
				if strings.EqualFold(strings.TrimSpace(parameter.Name), "LICENSE") && strings.TrimSpace(parameter.Value) != "" {
					attribution += "; Geodata license: " + strings.TrimSpace(parameter.Value)
				}
			}
			description := joinNonEmpty("\n", info.Headline, info.Description, info.Instruction)
			id := strings.TrimSpace(cap.Identifier) + "|" + strings.TrimSpace(info.Event) + "|" + validFrom.UTC().Format(time.RFC3339)
			alerts = append(alerts, Alert{ID: id, Event: strings.TrimSpace(info.Event), ContentLanguage: dwdLanguage(info.Language), Severity: severity, SeverityLabel: strings.TrimSpace(info.Severity), Probability: probability, ValidFrom: validFrom.UTC(), ValidTo: validTo.UTC(), IssuedAt: sent.UTC(), Description: description, Area: strings.Join(uniqueStrings(areaNames), ", "), Attribution: attribution, Source: "DWD", FetchedAt: now})
		}
	}
	return alerts, nil
}

func capReferenceKey(sender, identifier string) string {
	return strings.TrimSpace(sender) + "\x00" + strings.TrimSpace(identifier)
}

func dwdLanguage(locale string) string {
	language, _, _ := strings.Cut(strings.ToLower(strings.TrimSpace(locale)), "-")
	switch language {
	case "de", "en", "fr", "es", "it", "pl":
		return language
	default:
		return "en"
	}
}

func dwdFeedEndpoint(language string) string {
	switch language {
	case "de":
		return DefaultDWDAtomEndpoint
	case "fr", "es", "pl":
		return dwdMultilingualEndpoint
	default:
		return dwdEnglishAtomEndpoint
	}
}

func selectCAPInfo(infos []capInfo, language string) []capInfo {
	if len(infos) < 2 {
		return infos
	}
	for _, info := range infos {
		infoLanguage, _, _ := strings.Cut(strings.ToLower(strings.TrimSpace(info.Language)), "-")
		if infoLanguage == language {
			return []capInfo{info}
		}
	}
	for _, info := range infos {
		infoLanguage, _, _ := strings.Cut(strings.ToLower(strings.TrimSpace(info.Language)), "-")
		if infoLanguage == "en" {
			return []capInfo{info}
		}
	}
	return infos[:1]
}

func fetchXML(ctx context.Context, client *http.Client, endpoint, accept string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", accept)
	req.Header.Set("User-Agent", "UltimateWX/0.1 (DWD CAP warning client)")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("returned %s", resp.Status)
	}
	body, err := readLimitedBody(resp.Body, 16<<20)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func fetchDWDLink(ctx context.Context, client *http.Client, feedURL, href string) ([]byte, error) {
	base, err := url.Parse(feedURL)
	if err != nil {
		return nil, err
	}
	ref, err := url.Parse(strings.TrimSpace(href))
	if err != nil {
		return nil, err
	}
	ref = base.ResolveReference(ref)
	if ref.Scheme != "https" || (ref.Hostname() != "www.dwd.de" && ref.Hostname() != "opendata.dwd.de") {
		return nil, fmt.Errorf("refused non-DWD CAP link")
	}
	return fetchXML(ctx, client, ref.String(), "application/xml, application/cap+xml")
}

func dwdSeverity(value string) int {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "minor":
		return 1
	case "moderate":
		return 2
	case "severe", "extreme":
		return 3
	default:
		return 0
	}
}

func parseCAPTime(value string) time.Time {
	t, _ := time.Parse(time.RFC3339, strings.TrimSpace(value))
	return t
}

func polygonContains(value string, latitude, longitude float64) bool {
	var points [][2]float64
	for _, pair := range strings.Fields(strings.TrimSpace(value)) {
		parts := strings.Split(pair, ",")
		if len(parts) != 2 {
			return false
		}
		lat, e1 := strconv.ParseFloat(parts[0], 64)
		lon, e2 := strconv.ParseFloat(parts[1], 64)
		if e1 != nil || e2 != nil || math.IsNaN(lat) || math.IsNaN(lon) || math.IsInf(lat, 0) || math.IsInf(lon, 0) || lat < -90 || lat > 90 || lon < -180 || lon > 180 {
			return false
		}
		points = append(points, [2]float64{lat, lon})
	}
	if len(points) < 4 || points[0] != points[len(points)-1] {
		return false
	}
	inside := false
	for i, j := 0, len(points)-1; i < len(points); j, i = i, i+1 {
		pi, pj := points[i], points[j]
		if pointOnSegment(latitude, longitude, pi, pj) {
			return true
		}
		if (pi[0] > latitude) != (pj[0] > latitude) && longitude < (pj[1]-pi[1])*(latitude-pi[0])/(pj[0]-pi[0])+pi[1] {
			inside = !inside
		}
	}
	return inside
}

func pointOnSegment(latitude, longitude float64, a, b [2]float64) bool {
	cross := (longitude-a[1])*(b[0]-a[0]) - (latitude-a[0])*(b[1]-a[1])
	if math.Abs(cross) > 1e-9 {
		return false
	}
	return latitude >= math.Min(a[0], b[0])-1e-9 && latitude <= math.Max(a[0], b[0])+1e-9 && longitude >= math.Min(a[1], b[1])-1e-9 && longitude <= math.Max(a[1], b[1])+1e-9
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" && !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	return out
}

func bytesTrimSpace(value []byte) []byte { return []byte(strings.TrimSpace(string(value))) }
