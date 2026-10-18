package tts

import (
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

const repository = "rhasspy/piper-voices"
const revision = "v1.0.0"
const maxModelSize = 250 << 20

type Voice struct {
	ID           string `json:"id"`
	Language     string `json:"language"`
	Name         string `json:"name"`
	Quality      string `json:"quality"`
	License      string `json:"license"`
	Attribution  string `json:"attribution"`
	ModelCardURL string `json:"model_card_url"`
	Installed    bool   `json:"installed"`
	modelPath    string
	configPath   string
	cardPath     string
	modelHash    string
	configHash   string
	configSHA1   bool
}
type File struct {
	Path string `json:"path"`
	Type string `json:"type"`
	OID  string `json:"oid"`
	Size int64  `json:"size"`
	LFS  struct {
		OID  string `json:"oid"`
		Size int64  `json:"size"`
	} `json:"lfs"`
}
type Service struct {
	root     string
	mu       sync.Mutex
	cachedAt map[string]time.Time
	catalog  map[string][]Voice
	client   *http.Client
}

func New(root string) *Service {
	return &Service{root: root, cachedAt: map[string]time.Time{}, catalog: map[string][]Voice{}, client: &http.Client{Timeout: 25 * time.Second}}
}
func (s *Service) Catalog(ctx context.Context, language string) ([]Voice, error) {
	language = strings.ToLower(strings.TrimSpace(language))
	allowed := map[string]bool{"en": true, "pl": true, "de": true, "fr": true, "es": true, "it": true}
	if !allowed[language] {
		return nil, fmt.Errorf("unsupported language")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if time.Since(s.cachedAt[language]) > time.Hour || len(s.catalog[language]) == 0 {
		catalog, e := s.fetchCatalog(ctx, language)
		if e != nil {
			return nil, e
		}
		s.catalog[language] = catalog
		s.cachedAt[language] = time.Now()
	}
	out := make([]Voice, 0)
	for _, v := range s.catalog[language] {
		out = append(out, v)
	}
	return out, nil
}
func (s *Service) fetchCatalog(ctx context.Context, language string) ([]Voice, error) {
	endpoint := "https://huggingface.co/api/models/" + repository + "/tree/" + revision + "?recursive=true&expand=true&limit=10000"
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if e != nil {
		return nil, e
	}
	req.Header.Set("User-Agent", "UltimateWX/0.1 (open-source amateur radio weather)")
	resp, e := s.client.Do(req)
	if e != nil {
		return nil, e
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("voice catalog returned %s", resp.Status)
	}
	var files []File
	if e = json.NewDecoder(io.LimitReader(resp.Body, 20<<20)).Decode(&files); e != nil {
		return nil, e
	}
	byPath := map[string]File{}
	for _, f := range files {
		byPath[f.Path] = f
	}
	var out []Voice
	cardCache := map[string]struct{ license, attribution string }{}
	for _, f := range files {
		if !strings.HasSuffix(f.Path, ".onnx") || f.Type != "file" || f.Size <= 0 || f.Size > maxModelSize {
			continue
		}
		dir := filepath.ToSlash(filepath.Dir(f.Path))
		jsonPath := f.Path + ".json"
		cardPath := dir + "/MODEL_CARD"
		cf, cok := byPath[jsonPath]
		card, cark := byPath[cardPath]
		if !cok || !cark || f.LFS.OID == "" {
			continue
		}
		modelHash := strings.TrimPrefix(f.LFS.OID, "sha256:")
		if len(modelHash) != 64 {
			continue
		}
		configHash := cf.OID
		isSHA1 := len(configHash) == 40
		if !isSHA1 && len(strings.TrimPrefix(cf.LFS.OID, "sha256:")) == 64 {
			configHash = strings.TrimPrefix(cf.LFS.OID, "sha256:")
		} else if isSHA1 {
		} else {
			continue
		}
		base := strings.TrimSuffix(filepath.Base(f.Path), ".onnx")
		parts := strings.Split(base, "-")
		if len(parts) < 3 {
			continue
		}
		lang := strings.ToLower(strings.Split(parts[0], "_")[0])
		if lang != language {
			continue
		}
		licenseInfo, exists := cardCache[cardPath]
		if !exists {
			cardText, cardErr := s.getText(ctx, resolve(card.Path))
			if cardErr != nil {
				continue
			}
			license, attribution := licenseFor(cardText)
			licenseInfo = struct{ license, attribution string }{license, attribution}
			cardCache[cardPath] = licenseInfo
		}
		if licenseInfo.license == "" {
			continue
		}
		voice := Voice{ID: base, Language: lang, Name: strings.Join(parts[1:len(parts)-1], "-"), Quality: parts[len(parts)-1], License: licenseInfo.license, Attribution: licenseInfo.attribution, ModelCardURL: "https://huggingface.co/" + repository + "/blob/" + revision + "/" + card.Path, modelPath: f.Path, configPath: jsonPath, cardPath: cardPath, modelHash: modelHash, configHash: configHash, configSHA1: isSHA1}
		out = append(out, voice)
	}
	return out, nil
}

var licenseRE = regexp.MustCompile(`(?im)^\s*license\s*:\s*(CC[- ]BY[- ]4\.0|CC[- ]BY[- ]SA[- ]4\.0|CC0[- ]1\.0|Apache[- ]2\.0|MIT|GPL[- ]3\.0(?:-or-later)?)\s*$`)

func licenseFor(card string) (string, string) {
	match := licenseRE.FindStringSubmatch(card)
	if len(match) < 2 {
		return "", ""
	}
	norm := strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(match[1], " ", "-"), "--", "-"))
	switch norm {
	case "CC-BY-4.0":
		return "CC-BY-4.0", "Attribution required; see model card"
	case "CC-BY-SA-4.0":
		return "CC-BY-SA-4.0", "Attribution and share-alike required; see model card"
	case "CC0-1.0":
		return "CC0-1.0", "Public-domain dedication; see model card"
	case "APACHE-2.0":
		return "Apache-2.0", "See model card"
	case "MIT":
		return "MIT", "See model card"
	case "GPL-3.0", "GPL-3.0-OR-LATER":
		return norm, "See model card"
	}
	return "", ""
}
func (s *Service) getText(ctx context.Context, u string) (string, error) {
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if e != nil {
		return "", e
	}
	req.Header.Set("User-Agent", "UltimateWX")
	resp, e := s.client.Do(req)
	if e != nil {
		return "", e
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("model card: %s", resp.Status)
	}
	b, e := io.ReadAll(io.LimitReader(resp.Body, 256<<10))
	return string(b), e
}
func resolve(path string) string {
	return "https://huggingface.co/" + repository + "/resolve/" + revision + "/" + strings.ReplaceAll(path, " ", "%20")
}
func (s *Service) Download(ctx context.Context, id string) (string, error) {
	var chosen *Voice
	s.mu.Lock()
	for _, voices := range s.catalog {
		for i := range voices {
			if voices[i].ID == id {
				v := voices[i]
				chosen = &v
				break
			}
		}
		if chosen != nil {
			break
		}
	}
	s.mu.Unlock()
	if chosen == nil {
		return "", fmt.Errorf("voice is not in the verified catalog")
	}
	v := *chosen
	if e := os.MkdirAll(s.root, 0700); e != nil {
		return "", e
	}
	modelPath := filepath.Join(s.root, v.ID+".onnx")
	configPath := modelPath + ".json"
	if _, e := os.Stat(modelPath); e == nil {
		if _, e = os.Stat(configPath); e == nil {
			return modelPath, nil
		}
	}
	model, e := s.downloadFile(ctx, v.modelPath, maxModelSize)
	if e != nil {
		return "", e
	}
	if digest := sha256.Sum256(model); hex.EncodeToString(digest[:]) != v.modelHash {
		return "", fmt.Errorf("model SHA-256 mismatch")
	}
	cfg, e := s.downloadFile(ctx, v.configPath, 1<<20)
	if e != nil {
		return "", e
	}
	if v.configSHA1 {
		h := sha1.Sum(append(append([]byte(fmt.Sprintf("blob %d\x00", len(cfg))), cfg...), []byte{}...))
		if hex.EncodeToString(h[:]) != v.configHash {
			return "", fmt.Errorf("model config Git SHA-1 mismatch")
		}
	} else {
		h := sha256.Sum256(cfg)
		if hex.EncodeToString(h[:]) != v.configHash {
			return "", fmt.Errorf("model config SHA-256 mismatch")
		}
	}
	card, e := s.downloadFile(ctx, v.cardPath, 256<<10)
	if e != nil {
		return "", e
	}
	license, _ := licenseFor(string(card))
	if license != v.License {
		return "", fmt.Errorf("voice license changed since catalog check")
	}
	if e = atomicFile(modelPath, model, 0600); e != nil {
		return "", e
	}
	if e = atomicFile(configPath, cfg, 0600); e != nil {
		os.Remove(modelPath)
		return "", e
	}
	if e = atomicFile(filepath.Join(s.root, v.ID+".MODEL_CARD"), card, 0600); e != nil {
		return "", e
	}
	metadata, _ := json.MarshalIndent(map[string]any{"id": v.ID, "language": v.Language, "license": v.License, "attribution": v.Attribution, "model_card_url": v.ModelCardURL}, "", "  ")
	_ = atomicFile(filepath.Join(s.root, v.ID+".license.json"), metadata, 0600)
	return modelPath, nil
}
func (s *Service) downloadFile(ctx context.Context, path string, max int64) ([]byte, error) {
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, resolve(path), nil)
	if e != nil {
		return nil, e
	}
	req.Header.Set("User-Agent", "UltimateWX")
	resp, e := s.client.Do(req)
	if e != nil {
		return nil, e
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("download voice file: %s", resp.Status)
	}
	b, e := io.ReadAll(io.LimitReader(resp.Body, max+1))
	if e != nil {
		return nil, e
	}
	if int64(len(b)) > max {
		return nil, fmt.Errorf("voice file exceeds size limit")
	}
	return b, nil
}
func atomicFile(path string, data []byte, mode os.FileMode) error {
	tmp := path + ".tmp"
	if e := os.WriteFile(tmp, data, mode); e != nil {
		return e
	}
	if e := os.Rename(tmp, path); e != nil {
		os.Remove(tmp)
		return e
	}
	return nil
}
func (s *Service) Installed(id string) bool {
	_, e := os.Stat(filepath.Join(s.root, id+".onnx"))
	return e == nil
}
func EncodeLangURL(s string) string { return url.QueryEscape(s) }
