package store

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ultimatewx/ultimatewx/internal/aprs"
	"github.com/ultimatewx/ultimatewx/internal/warnings"
	"github.com/ultimatewx/ultimatewx/internal/weather"
)

type Playback struct {
	At     time.Time `json:"at"`
	Text   string    `json:"text"`
	Result string    `json:"result"`
}
type WarningAnnouncement struct {
	Signature            string    `json:"signature"`
	At                   time.Time `json:"at"`
	LastAttemptSignature string    `json:"last_attempt_signature,omitempty"`
	LastAttemptAt        time.Time `json:"last_attempt_at,omitempty"`
}
type Store struct {
	dir                  string
	mu                   sync.RWMutex
	observations         []weather.Observation
	frames               []aprs.Frame
	framesNeedRewrite    bool
	playbacks            []Playback
	warnings             []warnings.Alert
	endedWarnings        []warnings.Alert
	warningAnnouncements map[string]WarningAnnouncement
}

func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	s := &Store{dir: dir, warningAnnouncements: map[string]WarningAnnouncement{}}
	s.frames = load[aprs.Frame](filepath.Join(dir, "aprs.jsonl"))
	if len(s.frames) > 10000 {
		s.frames = append([]aprs.Frame(nil), s.frames[len(s.frames)-10000:]...)
		if err := rewrite(filepath.Join(dir, "aprs.jsonl"), s.frames); err != nil {
			return nil, err
		}
	}
	s.observations = load[weather.Observation](filepath.Join(dir, "observations.jsonl"))
	s.backfillAPRSFrameIDs()
	s.playbacks = load[Playback](filepath.Join(dir, "playbacks.jsonl"))
	s.warnings = load[warnings.Alert](filepath.Join(dir, "warnings.jsonl"))
	s.endedWarnings = load[warnings.Alert](filepath.Join(dir, "warnings_ended.jsonl"))
	if b, err := os.ReadFile(filepath.Join(dir, "warning_announcements.json")); err == nil {
		_ = json.Unmarshal(b, &s.warningAnnouncements)
	}
	return s, nil
}

func (s *Store) backfillAPRSFrameIDs() {
	frameIDs := make(map[string]string, len(s.frames))
	for _, frame := range s.frames {
		if !frame.IsWX || frame.Station == "" || frame.ReceivedAt.IsZero() {
			continue
		}
		frameID := frame.FrameID
		if frameID == "" {
			frameID = aprs.FrameFingerprint(frame.Raw)
		}
		key := strings.ToUpper(frame.Station) + "|" + frame.ReceivedAt.UTC().Format(time.RFC3339Nano)
		if existing, ok := frameIDs[key]; ok && existing != frameID {
			frameIDs[key] = ""
		} else {
			frameIDs[key] = frameID
		}
	}
	for i := range s.observations {
		observation := &s.observations[i]
		if observation.FrameID != "" || !strings.EqualFold(observation.Source, "aprs") || observation.Station == "" {
			continue
		}
		key := strings.ToUpper(observation.Station) + "|" + observation.ObservedAt.UTC().Format(time.RFC3339Nano)
		observation.FrameID = frameIDs[key]
	}
}

func load[T any](path string) []T {
	f, e := os.Open(path)
	if e != nil {
		return nil
	}
	defer f.Close()
	var out []T
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 4096), 1<<20)
	for sc.Scan() {
		var x T
		if json.Unmarshal(sc.Bytes(), &x) == nil {
			out = append(out, x)
		}
	}
	return out
}
func appendJSON(path string, v any) error {
	b, e := json.Marshal(v)
	if e != nil {
		return e
	}
	f, e := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	defer f.Close()
	_, e = f.Write(append(b, '\n'))
	return e
}
func (s *Store) AddObservation(o weather.Observation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := appendJSON(filepath.Join(s.dir, "observations.jsonl"), o); e != nil {
		return e
	}
	s.observations = append(s.observations, o)
	return nil
}
func (s *Store) AddFrame(f aprs.Frame) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := appendJSON(filepath.Join(s.dir, "aprs.jsonl"), f); e != nil {
		return e
	}
	s.frames = append(s.frames, f)
	if len(s.frames) > 10000 {
		s.frames = append([]aprs.Frame(nil), s.frames[len(s.frames)-10000:]...)
		s.framesNeedRewrite = true
	}
	return nil
}
func (s *Store) AddPlayback(p Playback) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := appendJSON(filepath.Join(s.dir, "playbacks.jsonl"), p); e != nil {
		return e
	}
	s.playbacks = append(s.playbacks, p)
	return nil
}

func deduplicateWarnings(items []warnings.Alert) []warnings.Alert {
	unique := make([]warnings.Alert, 0, len(items))
	indexes := make(map[string]int, len(items))
	for _, item := range items {
		key := warningKey(item)
		index, exists := indexes[key]
		if !exists {
			indexes[key] = len(unique)
			unique = append(unique, item)
			continue
		}
		current := unique[index]
		itemVersion, currentVersion := item.IssuedAt, current.IssuedAt
		if itemVersion.IsZero() {
			itemVersion = item.FetchedAt
		}
		if currentVersion.IsZero() {
			currentVersion = current.FetchedAt
		}
		if itemVersion.After(currentVersion) {
			unique[index] = item
		}
	}
	return unique
}

func (s *Store) ReplaceWarnings(items []warnings.Alert) error {
	return s.replaceWarnings(items, false)
}

// ReconcileWarnings persists a successful provider snapshot and archives
// warnings that disappeared from that snapshot, including cancellations.
func (s *Store) ReconcileWarnings(items []warnings.Alert) error {
	return s.replaceWarnings(items, true)
}

func (s *Store) replaceWarnings(items []warnings.Alert, archiveRemoved bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	items = deduplicateWarnings(items)
	now := time.Now().UTC()
	cutoff := now.Add(-30 * 24 * time.Hour)
	active := make(map[string]bool, len(items))
	for _, item := range items {
		active[warningKey(item)] = true
	}
	ended := make([]warnings.Alert, 0, len(s.endedWarnings)+len(s.warnings))
	seen := make(map[string]bool, len(ended))
	for _, item := range s.endedWarnings {
		key := warningKey(item)
		if item.ValidTo.After(cutoff) && !active[key] && !seen[key] {
			ended = append(ended, item)
			seen[key] = true
		}
	}
	for _, item := range s.warnings {
		key := warningKey(item)
		shouldArchive := !item.ValidTo.After(now) || archiveRemoved
		if shouldArchive && item.ValidTo.After(cutoff) && !active[key] && !seen[key] {
			ended = append(ended, item)
			seen[key] = true
		}
	}
	if len(ended) > 500 {
		ended = ended[len(ended)-500:]
	}
	if err := rewrite(filepath.Join(s.dir, "warnings_ended.jsonl"), ended); err != nil {
		return err
	}
	if err := rewrite(filepath.Join(s.dir, "warnings.jsonl"), items); err != nil {
		return err
	}
	s.warnings = append([]warnings.Alert(nil), items...)
	s.endedWarnings = ended
	return nil
}
func (s *Store) Warnings() []warnings.Alert {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]warnings.Alert(nil), s.warnings...)
}
func (s *Store) EndedWarnings() []warnings.Alert {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cutoff := time.Now().UTC().Add(-30 * 24 * time.Hour)
	ended := make([]warnings.Alert, 0, len(s.endedWarnings))
	for _, item := range s.endedWarnings {
		if item.ValidTo.After(cutoff) {
			ended = append(ended, item)
		}
	}
	return ended
}

func warningKey(item warnings.Alert) string {
	return strings.ToUpper(strings.TrimSpace(item.Source)) + "|" + item.ID + "|" + item.ValidFrom.UTC().Format(time.RFC3339) + "|" + item.ValidTo.UTC().Format(time.RFC3339)
}
func (s *Store) WarningAnnouncement(id string) (WarningAnnouncement, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.warningAnnouncements[id]
	return item, ok
}
func (s *Store) SetWarningAnnouncement(id string, item WarningAnnouncement) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	updated := make(map[string]WarningAnnouncement, len(s.warningAnnouncements)+1)
	activityAt := item.LastAttemptAt
	if activityAt.IsZero() {
		activityAt = item.At
	}
	cutoff := activityAt.Add(-30 * 24 * time.Hour)
	for key, value := range s.warningAnnouncements {
		lastActivity := value.LastAttemptAt
		if lastActivity.IsZero() {
			lastActivity = value.At
		}
		if !lastActivity.Before(cutoff) {
			updated[key] = value
		}
	}
	updated[id] = item
	data, err := json.MarshalIndent(updated, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.dir, 0700); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(s.dir, "warning_announcements.json.tmp"), append(data, '\n'), 0600); err != nil {
		return err
	}
	if err := os.Rename(filepath.Join(s.dir, "warning_announcements.json.tmp"), filepath.Join(s.dir, "warning_announcements.json")); err != nil {
		return err
	}
	s.warningAnnouncements = updated
	return nil
}
func (s *Store) Observations(days int) []weather.Observation {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := time.Now()
	cutoff := now.Add(-time.Duration(days) * 24 * time.Hour)
	out := make([]weather.Observation, 0)
	for _, o := range s.observations {
		if o.ObservedAt.After(cutoff) && !o.ObservedAt.After(now) {
			out = append(out, o)
		}
	}
	return out
}
func (s *Store) Latest() []weather.Observation {
	return s.LatestExcludingStations(nil)
}
func (s *Store) LatestExcludingStations(blacklist []string) []weather.Observation {
	return s.LatestExcludingAPRS(blacklist, nil)
}

func (s *Store) LatestExcludingAPRS(blacklist, frameBlacklist []string) []weather.Observation {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := time.Now()
	last := map[string]weather.Observation{}
	for _, o := range s.observations {
		if o.ObservedAt.After(now) {
			continue
		}
		if strings.EqualFold(o.Source, "aprs") && (blacklistedStation(o.Station, blacklist) || blacklistedFrame(o.FrameID, frameBlacklist)) {
			continue
		}
		k := o.Parameter + "|" + o.Source
		if old, ok := last[k]; !ok || o.ObservedAt.After(old.ObservedAt) {
			last[k] = o
		}
	}
	out := make([]weather.Observation, 0, len(last))
	for _, o := range last {
		out = append(out, o)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Parameter < out[j].Parameter })
	return out
}

// LatestByDevice returns the newest sample per parameter, source and device.
// It keeps sensors sharing one MQTT topic or integration visible independently.
func (s *Store) LatestByDevice(blacklist []string) []weather.Observation {
	return s.LatestByDeviceExcludingAPRS(blacklist, nil)
}

func (s *Store) LatestByDeviceExcludingAPRS(blacklist, frameBlacklist []string) []weather.Observation {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := time.Now()
	type key struct {
		parameter string
		source    string
		identity  string
	}
	last := map[key]weather.Observation{}
	for _, observation := range s.observations {
		if observation.ObservedAt.After(now) {
			continue
		}
		if strings.EqualFold(observation.Source, "aprs") && (blacklistedStation(observation.Station, blacklist) || blacklistedFrame(observation.FrameID, frameBlacklist)) {
			continue
		}
		identity := observation.DeviceID
		if identity == "" {
			identity = observation.Station
		}
		group := key{parameter: observation.Parameter, source: observation.Source, identity: identity}
		if previous, ok := last[group]; !ok || observation.ObservedAt.After(previous.ObservedAt) {
			last[group] = observation
		}
	}
	out := make([]weather.Observation, 0, len(last))
	for _, observation := range last {
		out = append(out, observation)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Parameter != out[j].Parameter {
			return out[i].Parameter < out[j].Parameter
		}
		if out[i].Source != out[j].Source {
			return out[i].Source < out[j].Source
		}
		if out[i].DeviceID != out[j].DeviceID {
			return out[i].DeviceID < out[j].DeviceID
		}
		return out[i].Station < out[j].Station
	})
	return out
}

func blacklistedFrame(frameID string, blacklist []string) bool {
	for _, blockedID := range blacklist {
		if frameID != "" && frameID == blockedID {
			return true
		}
	}
	return false
}

func blacklistedStation(station string, blacklist []string) bool {
	station = strings.TrimSpace(station)
	for _, callsign := range blacklist {
		if station != "" && strings.EqualFold(station, strings.TrimSpace(callsign)) {
			return true
		}
	}
	return false
}
func (s *Store) Frames(limit int, wxOnly bool) []aprs.Frame {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]aprs.Frame, 0)
	for i := len(s.frames) - 1; i >= 0 && len(out) < limit; i-- {
		if !wxOnly || s.frames[i].IsWX {
			out = append(out, s.frames[i])
		}
	}
	return out
}
func (s *Store) Stats() map[string]any {
	return s.StatsExcludingAPRS(nil, nil)
}

func (s *Store) StatsExcludingAPRS(stationBlacklist, frameBlacklist []string) map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	observations := 0
	for _, observation := range s.observations {
		if strings.EqualFold(observation.Source, "aprs") && (blacklistedStation(observation.Station, stationBlacklist) || blacklistedFrame(observation.FrameID, frameBlacklist)) {
			continue
		}
		observations++
	}
	eligible := 0
	wx := 0
	for _, f := range s.frames {
		if f.IsWX {
			wx++
		}
		frameID := f.FrameID
		if frameID == "" {
			frameID = aprs.FrameFingerprint(f.Raw)
		}
		if f.Eligible && !blacklistedStation(f.Station, stationBlacklist) && !blacklistedFrame(frameID, frameBlacklist) {
			eligible++
		}
	}
	return map[string]any{"observations": observations, "aprs_frames": len(s.frames), "aprs_wx_frames": wx, "aprs_eligible_frames": eligible, "playbacks": len(s.playbacks), "recent_playbacks": s.playbacks[max(0, len(s.playbacks)-20):]}
}
func (s *Store) Prune(days int) error {
	if days < 1 || days > 365 {
		return fmt.Errorf("history retention must be 1..365 days")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-time.Duration(days) * 24 * time.Hour)
	observations := make([]weather.Observation, 0, len(s.observations))
	for _, o := range s.observations {
		if o.ObservedAt.After(cutoff) && !o.ObservedAt.After(now) {
			observations = append(observations, o)
		}
	}
	if len(observations) != len(s.observations) {
		if err := rewrite(filepath.Join(s.dir, "observations.jsonl"), observations); err != nil {
			return err
		}
	}
	s.observations = observations

	frames := make([]aprs.Frame, 0, min(len(s.frames), 10000))
	for _, frame := range s.frames {
		if frame.ReceivedAt.After(cutoff) && !frame.ReceivedAt.After(now) {
			frames = append(frames, frame)
		}
	}
	if len(frames) > 10000 {
		frames = frames[len(frames)-10000:]
	}
	if len(frames) != len(s.frames) || s.framesNeedRewrite {
		if err := rewrite(filepath.Join(s.dir, "aprs.jsonl"), frames); err != nil {
			return err
		}
		s.framesNeedRewrite = false
	}
	s.frames = frames

	playbacks := make([]Playback, 0, len(s.playbacks))
	for _, playback := range s.playbacks {
		if playback.At.After(cutoff) && !playback.At.After(now) {
			playbacks = append(playbacks, playback)
		}
	}
	if len(playbacks) != len(s.playbacks) {
		if err := rewrite(filepath.Join(s.dir, "playbacks.jsonl"), playbacks); err != nil {
			return err
		}
	}
	s.playbacks = playbacks
	return nil
}
func rewrite[T any](path string, items []T) error {
	tmp := path + ".tmp"
	f, e := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	for _, v := range items {
		b, er := json.Marshal(v)
		if er != nil {
			f.Close()
			return er
		}
		if _, er = f.Write(append(b, '\n')); er != nil {
			f.Close()
			return er
		}
	}
	if e = f.Close(); e != nil {
		return e
	}
	return replaceStoreFile(tmp, path)
}
