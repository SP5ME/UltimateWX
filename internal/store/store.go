package store

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/ultimatewx/ultimatewx/internal/aprs"
	"github.com/ultimatewx/ultimatewx/internal/weather"
)

type Playback struct {
	At     time.Time `json:"at"`
	Text   string    `json:"text"`
	Result string    `json:"result"`
}
type Store struct {
	dir          string
	mu           sync.RWMutex
	observations []weather.Observation
	frames       []aprs.Frame
	playbacks    []Playback
}

func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	s := &Store{dir: dir}
	s.observations = load[weather.Observation](filepath.Join(dir, "observations.jsonl"))
	s.frames = load[aprs.Frame](filepath.Join(dir, "aprs.jsonl"))
	s.playbacks = load[Playback](filepath.Join(dir, "playbacks.jsonl"))
	return s, nil
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
func (s *Store) Observations(days int) []weather.Observation {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cutoff := time.Now().Add(-time.Duration(days) * 24 * time.Hour)
	out := make([]weather.Observation, 0)
	for _, o := range s.observations {
		if o.ObservedAt.After(cutoff) {
			out = append(out, o)
		}
	}
	return out
}
func (s *Store) Latest() []weather.Observation {
	s.mu.RLock()
	defer s.mu.RUnlock()
	last := map[string]weather.Observation{}
	for _, o := range s.observations {
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
	s.mu.RLock()
	defer s.mu.RUnlock()
	eligible := 0
	wx := 0
	for _, f := range s.frames {
		if f.IsWX {
			wx++
		}
		if f.Eligible {
			eligible++
		}
	}
	return map[string]any{"observations": len(s.observations), "aprs_frames": len(s.frames), "aprs_wx_frames": wx, "aprs_eligible_frames": eligible, "playbacks": len(s.playbacks), "recent_playbacks": s.playbacks[max(0, len(s.playbacks)-20):]}
}
func (s *Store) Prune(days int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cutoff := time.Now().Add(-time.Duration(days) * 24 * time.Hour)
	keep := s.observations[:0]
	for _, o := range s.observations {
		if o.ObservedAt.After(cutoff) {
			keep = append(keep, o)
		}
	}
	s.observations = keep
	_ = rewrite(filepath.Join(s.dir, "observations.jsonl"), keep)
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
	return os.Rename(tmp, path)
}
