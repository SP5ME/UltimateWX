package warnings

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/ultimatewx/ultimatewx/internal/mqtt"
)

const (
	MeteoAlarmSource        = "MeteoAlarm"
	MeteoAlarmTopic         = "origin/a/wis2/eu-eumetnet-warnings/data/core/weather/advisories-warnings"
	MeteoAlarmDefaultBroker = "mqtts://wis2broker.globaldata.nws.noaa.gov:8883"
)

// MeteoAlarm watches the public WIS2 warning stream. The handler is called
// serially for each GeoJSON notification; callers should keep it bounded.
type MeteoAlarm struct {
	Brokers []string
	Topic   string
	Client  *http.Client
}

func (p MeteoAlarm) Watch(ctx context.Context, handler func(topic string, payload []byte) error, state func(error)) {
	brokers := p.Brokers
	if len(brokers) == 0 {
		brokers = []string{
			MeteoAlarmDefaultBroker,
			"mqtts://globalbroker.meteo.fr:8883",
			"mqtts://globalbroker.inmet.gov.br:8883",
		}
	}
	topic := p.Topic
	if topic == "" {
		topic = MeteoAlarmTopic
	}
	var idBytes [12]byte
	if _, err := rand.Read(idBytes[:]); err != nil {
		idBytes = [12]byte{}
	}
	clientID := "ultimatewx-meteoalarm-" + hex.EncodeToString(idBytes[:])
	backoff := time.Second
	brokerIndex := 0
	for ctx.Err() == nil {
		broker := brokers[brokerIndex%len(brokers)]
		brokerIndex++
		conn, err := mqtt.ConnectWithQoS(broker, clientID, "everyone", "everyone", topic, 1)
		if err == nil {
			if state != nil {
				state(nil)
			}
			backoff = time.Second
			connectionDone := make(chan struct{})
			go func(connection net.Conn) {
				select {
				case <-ctx.Done():
					_ = connection.Close()
				case <-connectionDone:
				}
			}(conn)
			err = readMeteoAlarmStream(ctx, conn, topic, handler)
			close(connectionDone)
			_ = conn.Close()
		}
		if ctx.Err() != nil {
			return
		}
		if state != nil {
			state(err)
		}
		log.Printf("MeteoAlarm WIS2 stream: %v", err)
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return
		case <-timer.C:
		}
		if backoff < time.Minute {
			backoff *= 2
			if backoff > time.Minute {
				backoff = time.Minute
			}
		}
	}
}

func readMeteoAlarmStream(ctx context.Context, conn net.Conn, topic string, handler func(string, []byte) error) error {
	for ctx.Err() == nil {
		message, err := mqtt.ReadUnacked(conn)
		if err != nil {
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				if pingErr := mqtt.Ping(conn); pingErr != nil {
					return fmt.Errorf("ping WIS2 broker: %w", pingErr)
				}
				continue
			}
			return fmt.Errorf("read WIS2 notification: %w", err)
		}
		if message.Topic == "" {
			continue
		}
		if message.Topic != topic {
			if err := mqtt.Acknowledge(conn, message); err != nil {
				return err
			}
			continue
		}
		if len(message.Payload) > 0 && handler != nil {
			if err := handler(message.Topic, message.Payload); err != nil {
				return fmt.Errorf("process WIS2 notification: %w", err)
			}
		}
		if err := mqtt.Acknowledge(conn, message); err != nil {
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return errors.New("WIS2 stream stopped")
}

func IsMeteoAlarmTopic(topic string) bool {
	return strings.TrimSpace(topic) == MeteoAlarmTopic
}

type MeteoAlarmResult struct {
	Alert     *Alert
	AlertID   string
	RemoveIDs []string
}

type meteoFeature struct {
	Type     string `json:"type"`
	ID       string `json:"id"`
	Geometry struct {
		Type        string          `json:"type"`
		Coordinates json.RawMessage `json:"coordinates"`
	} `json:"geometry"`
	Links []struct {
		Href string `json:"href"`
		Rel  string `json:"rel"`
	} `json:"links"`
	Properties struct {
		AlertID            string   `json:"alertId"`
		CountryCode        string   `json:"countryCode"`
		Rights             string   `json:"rights"`
		IndexArea          int      `json:"indexArea"`
		IndexInfo          int      `json:"indexInfo"`
		ReferencedAlertIDs []string `json:"referencedAlertIds"`
	} `json:"properties"`
}

type meteoGeometry struct {
	Type        string          `json:"type"`
	Geometry    *meteoGeometry  `json:"geometry,omitempty"`
	Coordinates json.RawMessage `json:"coordinates"`
}

var (
	meteoHTTPOnce sync.Once
	meteoHTTP     *http.Client
)

// ParseNotification resolves the notification geometry and CAP links. The
// GeoJSON shape may reject an alert early, but only a CAP area polygon can
// confirm that the station is covered.
func (p MeteoAlarm) ParseNotification(ctx context.Context, payload []byte, latitude, longitude float64, locale string) (MeteoAlarmResult, error) {
	var feature meteoFeature
	if len(payload) == 0 || len(payload) > 8192 {
		return MeteoAlarmResult{}, errors.New("invalid MeteoAlarm notification size")
	}
	if err := json.Unmarshal(payload, &feature); err != nil {
		return MeteoAlarmResult{}, fmt.Errorf("decode MeteoAlarm notification: %w", err)
	}
	if feature.Type != "Feature" || strings.TrimSpace(feature.Properties.AlertID) == "" {
		return MeteoAlarmResult{}, errors.New("MeteoAlarm notification is missing its alert identity")
	}
	result := MeteoAlarmResult{AlertID: strings.TrimSpace(feature.Properties.AlertID)}
	for _, id := range feature.Properties.ReferencedAlertIDs {
		if id = strings.TrimSpace(id); id != "" {
			result.RemoveIDs = append(result.RemoveIDs, id)
		}
	}
	license, geometryURL, capURL := "", "", ""
	deleted := false
	for _, link := range feature.Links {
		switch strings.ToLower(strings.TrimSpace(link.Rel)) {
		case "license":
			license = strings.TrimSpace(link.Href)
		case "geometry":
			geometryURL = strings.TrimSpace(link.Href)
		case "deletion":
			deleted = true
		case "xml", "canonical", "update":
			if capURL == "" || strings.EqualFold(strings.TrimSpace(link.Rel), "xml") {
				capURL = strings.TrimSpace(link.Href)
			}
		}
	}
	if deleted {
		return result, nil
	}
	rights := strings.ToLower(feature.Properties.Rights)
	if !strings.Contains(strings.ToLower(license), "creativecommons.org/licenses/by/4.0") ||
		!strings.Contains(rights, "unrestricted") || !strings.Contains(rights, "attribut") {
		return result, errors.New("MeteoAlarm notification has no verified reuse and attribution terms")
	}
	if geometryURL == "" || capURL == "" {
		return result, errors.New("MeteoAlarm notification is missing exact geometry or CAP links")
	}
	client := p.Client
	if client == nil {
		client = meteoAlarmHTTPClient()
	}
	geometryBody, err := fetchMeteoAlarmURL(ctx, client, geometryURL, "application/geo+json, application/json", 2<<20)
	if err != nil {
		return result, fmt.Errorf("fetch MeteoAlarm geometry: %w", err)
	}
	var geometry meteoGeometry
	if err := json.Unmarshal(geometryBody, &geometry); err != nil {
		return result, fmt.Errorf("decode MeteoAlarm geometry: %w", err)
	}
	inside, err := meteoGeometryContains(geometry, latitude, longitude)
	if err != nil {
		return result, fmt.Errorf("validate MeteoAlarm geometry: %w", err)
	}
	capBody, err := fetchMeteoAlarmURL(ctx, client, capURL, "application/cap+xml, application/xml", 4<<20)
	if err != nil {
		return result, fmt.Errorf("fetch MeteoAlarm CAP document: %w", err)
	}
	var cap capAlert
	if err := xml.Unmarshal(capBody, &cap); err != nil {
		return result, fmt.Errorf("decode MeteoAlarm CAP document: %w", err)
	}
	if strings.TrimSpace(cap.Identifier) == "" || strings.TrimSpace(cap.Sender) == "" {
		return result, errors.New("MeteoAlarm CAP document is missing sender or identifier")
	}
	if cap.Status != "Actual" {
		return MeteoAlarmResult{}, nil
	}
	if cap.MsgType != "Alert" && cap.MsgType != "Update" && cap.MsgType != "Cancel" {
		return MeteoAlarmResult{}, nil
	}
	for _, value := range cap.References {
		for _, reference := range strings.Fields(value) {
			parts := strings.SplitN(reference, ",", 3)
			if len(parts) == 3 && strings.TrimSpace(parts[0]) != "" && strings.TrimSpace(parts[1]) != "" {
				result.RemoveIDs = append(result.RemoveIDs, strings.TrimSpace(parts[1]), strings.TrimSpace(parts[0])+"|"+strings.TrimSpace(parts[1]))
			}
		}
	}
	if cap.MsgType == "Cancel" {
		return result, nil
	}
	if !inside {
		return result, nil
	}
	info := selectCAPInfo(cap.Info, dwdLanguage(locale))
	if feature.Properties.IndexInfo >= 0 && feature.Properties.IndexInfo < len(cap.Info) {
		candidate := cap.Info[feature.Properties.IndexInfo]
		if strings.EqualFold(strings.TrimSpace(candidate.Language), dwdLanguage(locale)) {
			info = []capInfo{candidate}
		}
	}
	if len(info) == 0 {
		return result, errors.New("MeteoAlarm CAP document has no warning information")
	}
	selected := info[0]
	validFrom := parseCAPTime(selected.Onset)
	if validFrom.IsZero() {
		validFrom = parseCAPTime(selected.Effective)
	}
	if validFrom.IsZero() {
		validFrom = parseCAPTime(cap.Sent)
	}
	validTo := parseCAPTime(selected.Expires)
	if validFrom.IsZero() || validTo.IsZero() || !validTo.After(validFrom) {
		return result, errors.New("MeteoAlarm CAP warning has invalid validity times")
	}
	severity := dwdSeverity(selected.Severity)
	if severity == 0 {
		return result, nil
	}
	// The GeoJSON geometry can be a bounding box. Use it only as an early
	// rejection filter; station inclusion must be confirmed by the CAP polygon.
	if feature.Properties.IndexArea < 0 || feature.Properties.IndexArea >= len(selected.Area) {
		return result, nil
	}
	if !capAreaContains(selected.Area[feature.Properties.IndexArea], latitude, longitude) {
		return result, nil
	}
	var probability *int
	for _, parameter := range selected.Parameters {
		if strings.EqualFold(strings.TrimSpace(parameter.Name), "PROBABILITY") {
			if value, ok := parseProbability(parameter.Value); ok {
				probability = &value
			}
		}
		if strings.EqualFold(strings.TrimSpace(parameter.Name), "LICENSE") && parameter.Value != "" && !compatibleMeteoAlarmLicense(parameter.Value) {
			return result, errors.New("MeteoAlarm CAP document specifies incompatible reuse terms")
		}
	}
	area := ""
	if feature.Properties.IndexArea >= 0 && feature.Properties.IndexArea < len(selected.Area) {
		area = strings.TrimSpace(selected.Area[feature.Properties.IndexArea].Description)
	}
	issued, _ := time.Parse(time.RFC3339, strings.TrimSpace(cap.Sent))
	result.Alert = &Alert{ID: result.AlertID, Event: strings.TrimSpace(selected.Event), ContentLanguage: capLanguage(selected.Language), Severity: severity, SeverityLabel: strings.TrimSpace(selected.Severity), Probability: probability, ValidFrom: validFrom.UTC(), ValidTo: validTo.UTC(), IssuedAt: issued.UTC(), Description: joinNonEmpty("\n", selected.Headline, selected.Description, selected.Instruction), Area: area, Attribution: "MeteoAlarm / EUMETNET; CC BY 4.0", AreaCodes: []string{strings.ToUpper(strings.TrimSpace(feature.Properties.CountryCode))}, Source: MeteoAlarmSource, FetchedAt: time.Now().UTC()}
	return result, nil
}

func compatibleMeteoAlarmLicense(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return strings.Contains(value, "cc-by-4.0") || strings.Contains(value, "cc by 4.0") || strings.Contains(value, "creativecommons.org/licenses/by/4.0")
}

func capLanguage(value string) string {
	language, _, _ := strings.Cut(strings.ToLower(strings.TrimSpace(value)), "-")
	if language == "" {
		return "en"
	}
	return language
}

func meteoAlarmHTTPClient() *http.Client {
	meteoHTTPOnce.Do(func() {
		dialer := &net.Dialer{Timeout: 8 * time.Second, KeepAlive: 30 * time.Second}
		transport := &http.Transport{Proxy: nil, DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, err
			}
			for _, candidate := range ips {
				if !publicIP(candidate.IP) {
					continue
				}
				return dialer.DialContext(ctx, network, net.JoinHostPort(candidate.IP.String(), port))
			}
			return nil, errors.New("MeteoAlarm link resolved to no public IP address")
		}}
		meteoHTTP = &http.Client{Transport: transport, Timeout: 12 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return errors.New("too many MeteoAlarm redirects")
			}
			return validateMeteoAlarmURL(req.URL)
		}}
	})
	return meteoHTTP
}

var nonPublicIPRanges = func() []*net.IPNet {
	var ranges []*net.IPNet
	for _, cidr := range []string{
		"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24",
		"192.88.99.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24",
		"224.0.0.0/4", "240.0.0.0/4",
		"64:ff9b::/96", "64:ff9b:1::/48", "100::/64",
		"100:0:0:1::/64", "2001::/23", "2001:db8::/32", "3fff::/20", "5f00::/16",
	} {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			panic("invalid non-public IP range " + cidr)
		}
		ranges = append(ranges, network)
	}
	return ranges
}()

func publicIP(ip net.IP) bool {
	if ip == nil || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		return false
	}
	// IsGlobalUnicast includes several ranges that are not globally reachable,
	// such as shared carrier space and documentation/test prefixes. Do not let
	// warning links use those ranges to reach internal or special-purpose hosts.
	ip4 := ip.To4()
	for _, network := range nonPublicIPRanges {
		network4 := network.IP.To4()
		if (ip4 != nil && network4 != nil && network.Contains(ip4)) || (ip4 == nil && network4 == nil && network.Contains(ip)) {
			return false
		}
	}
	return true
}

func validateMeteoAlarmURL(value *url.URL) error {
	if value == nil || value.Scheme != "https" || value.Hostname() == "" || value.User != nil {
		return errors.New("refused non-HTTPS MeteoAlarm link")
	}
	if value.Port() != "" && value.Port() != "443" {
		return errors.New("refused non-standard MeteoAlarm HTTPS port")
	}
	if ip := net.ParseIP(value.Hostname()); ip != nil {
		if !publicIP(ip) {
			return errors.New("refused non-public MeteoAlarm link")
		}
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, value.Hostname())
	if err != nil {
		return fmt.Errorf("resolve MeteoAlarm link host: %w", err)
	}
	for _, candidate := range ips {
		if !publicIP(candidate.IP) {
			return errors.New("refused MeteoAlarm link with a non-public address")
		}
	}
	if len(ips) == 0 {
		return errors.New("MeteoAlarm link host has no addresses")
	}
	return nil
}

func fetchMeteoAlarmURL(ctx context.Context, client *http.Client, rawURL, accept string, limit int64) ([]byte, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return nil, err
	}
	if err := validateMeteoAlarmURL(parsed); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", accept)
	req.Header.Set("User-Agent", "UltimateWX/0.1 (MeteoAlarm warning client)")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MeteoAlarm link returned %s", resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("MeteoAlarm document exceeds %d-byte limit", limit)
	}
	return data, nil
}

func meteoGeometryContains(geometry meteoGeometry, latitude, longitude float64) (bool, error) {
	if geometry.Type == "Feature" && geometry.Geometry != nil {
		geometry = *geometry.Geometry
	}
	if geometry.Type == "Polygon" {
		if len(geometry.Coordinates) == 0 {
			return false, errors.New("polygon has no coordinates")
		}
		var rings [][][2]float64
		if err := json.Unmarshal(geometry.Coordinates, &rings); err != nil {
			return false, err
		}
		if len(rings) == 0 || len(rings[0]) < 4 {
			return false, errors.New("polygon has no closed exterior ring")
		}
		for _, ring := range rings {
			if err := validateMeteoRing(ring); err != nil {
				return false, err
			}
		}
		return pointInMeteoPolygon(rings, latitude, longitude), nil
	}
	if geometry.Type == "MultiPolygon" {
		if len(geometry.Coordinates) == 0 {
			return false, errors.New("multipolygon has no coordinates")
		}
		var polygons [][][][2]float64
		if err := json.Unmarshal(geometry.Coordinates, &polygons); err != nil {
			return false, err
		}
		if len(polygons) == 0 {
			return false, errors.New("multipolygon has no polygon rings")
		}
		for _, polygon := range polygons {
			if len(polygon) == 0 || len(polygon[0]) < 4 {
				return false, errors.New("multipolygon has no closed exterior ring")
			}
			for _, ring := range polygon {
				if err := validateMeteoRing(ring); err != nil {
					return false, err
				}
			}
			if pointInMeteoPolygon(polygon, latitude, longitude) {
				return true, nil
			}
		}
		return false, nil
	}
	return false, fmt.Errorf("unsupported exact geometry type %q", geometry.Type)
}

func validateMeteoRing(points [][2]float64) error {
	if len(points) < 4 || points[0] != points[len(points)-1] {
		return errors.New("polygon ring is not closed")
	}
	for _, point := range points {
		lon, lat := point[0], point[1]
		if math.IsNaN(lon) || math.IsNaN(lat) || math.IsInf(lon, 0) || math.IsInf(lat, 0) || lon < -180 || lon > 180 || lat < -90 || lat > 90 {
			return errors.New("polygon ring contains an invalid coordinate")
		}
	}
	return nil
}

func pointInMeteoPolygon(rings [][][2]float64, latitude, longitude float64) bool {
	if len(rings) == 0 || !pointInMeteoRing(rings[0], latitude, longitude) {
		return false
	}
	for _, hole := range rings[1:] {
		if pointInMeteoRing(hole, latitude, longitude) {
			return false
		}
	}
	return true
}

func pointInMeteoRing(points [][2]float64, latitude, longitude float64) bool {
	if len(points) < 4 || points[0] != points[len(points)-1] {
		return false
	}
	inside := false
	for i, j := 0, len(points)-1; i < len(points); j, i = i, i+1 {
		lonI, latI := points[i][0], points[i][1]
		lonJ, latJ := points[j][0], points[j][1]
		if math.IsNaN(lonI) || math.IsNaN(latI) || math.IsInf(lonI, 0) || math.IsInf(latI, 0) || lonI < -180 || lonI > 180 || latI < -90 || latI > 90 {
			return false
		}
		cross := (longitude-lonI)*(latJ-latI) - (latitude-latI)*(lonJ-lonI)
		if math.Abs(cross) < 1e-9 && latitude >= math.Min(latI, latJ)-1e-9 && latitude <= math.Max(latI, latJ)+1e-9 && longitude >= math.Min(lonI, lonJ)-1e-9 && longitude <= math.Max(lonI, lonJ)+1e-9 {
			return true
		}
		if (latI > latitude) != (latJ > latitude) && longitude < (lonJ-lonI)*(latitude-latI)/(latJ-latI)+lonI {
			inside = !inside
		}
	}
	return inside
}

func capAreaContains(area capArea, latitude, longitude float64) bool {
	if len(area.Polygons) == 0 {
		return false
	}
	for _, polygon := range area.Polygons {
		if polygonContains(polygon, latitude, longitude) {
			return true
		}
	}
	return false
}
