package updater

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var Repository = ""
var Version = "dev"

const maxReleaseBytes = 100 << 20

type Release struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	Body        string    `json:"body"`
	HTMLURL     string    `json:"html_url"`
	PublishedAt time.Time `json:"published_at"`
	Assets      []Asset   `json:"assets"`
}
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}
type Status struct {
	Available      bool   `json:"available"`
	CurrentVersion string `json:"current_version"`
	LatestVersion  string `json:"latest_version,omitempty"`
	Notes          string `json:"notes,omitempty"`
	URL            string `json:"url,omitempty"`
	Error          string `json:"error,omitempty"`
}

func Check(ctx context.Context, goos, arch string) Status {
	out := Status{CurrentVersion: Version}
	if Repository == "" {
		out.Error = "Aktualizacje wymagają wydania z ustawionym repozytorium projektu."
		return out
	}
	r, e := latestRelease(ctx)
	if e != nil {
		out.Error = e.Error()
		return out
	}
	out.LatestVersion = r.TagName
	out.Notes = r.Body
	out.URL = r.HTMLURL
	out.Available = normalize(r.TagName) != normalize(Version)
	asset := archiveName(goos, arch)
	if !hasAsset(r, asset) || !hasAsset(r, "SHA256SUMS") {
		out.Available = false
		out.Error = "Wydanie nie zawiera pakietu dla tej platformy albo pliku SHA256SUMS."
	}
	return out
}
func normalize(v string) string { return strings.TrimPrefix(strings.TrimSpace(v), "v") }
func latestRelease(ctx context.Context) (Release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/repos/"+Repository+"/releases?per_page=1", nil)
	if err != nil {
		return Release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "UltimateWX")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Release{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Release{}, fmt.Errorf("GitHub API: %s", resp.Status)
	}
	var releases []Release
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&releases); err != nil {
		return Release{}, err
	}
	if len(releases) == 0 {
		return Release{}, errors.New("brak opublikowanych wydań")
	}
	return releases[0], nil
}
func archiveName(goos, arch string) string {
	if goos == "linux" && arch == "arm" {
		arch = "armv7"
	}
	return fmt.Sprintf("ultimatewx_%s_%s.tar.gz", goos, arch)
}
func hasAsset(r Release, name string) bool {
	for _, a := range r.Assets {
		if a.Name == name {
			return true
		}
	}
	return false
}
func assetURL(r Release, name string) string {
	for _, a := range r.Assets {
		if a.Name == name {
			return a.BrowserDownloadURL
		}
	}
	return ""
}
func Apply(ctx context.Context, goos, arch string) error {
	if Repository == "" {
		return errors.New("repozytorium aktualizacji nie jest skonfigurowane")
	}
	r, e := latestRelease(ctx)
	if e != nil {
		return e
	}
	name := archiveName(goos, arch)
	archiveURL := assetURL(r, name)
	sumURL := assetURL(r, "SHA256SUMS")
	if archiveURL == "" || sumURL == "" {
		return errors.New("brak pakietu lub sumy kontrolnej dla platformy")
	}
	get := func(url string, limit int64) ([]byte, error) {
		req, e := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if e != nil {
			return nil, e
		}
		req.Header.Set("User-Agent", "UltimateWX")
		resp, e := http.DefaultClient.Do(req)
		if e != nil {
			return nil, e
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("pobieranie %s: %s", url, resp.Status)
		}
		return io.ReadAll(io.LimitReader(resp.Body, limit+1))
	}
	sumBytes, e := get(sumURL, 1<<20)
	if e != nil {
		return e
	}
	expected := ""
	for _, line := range strings.Split(string(sumBytes), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == name {
			expected = strings.ToLower(fields[0])
			break
		}
	}
	if len(expected) != 64 {
		return errors.New("brak poprawnej sumy SHA-256 w wydaniu")
	}
	archive, e := get(archiveURL, maxReleaseBytes)
	if e != nil {
		return e
	}
	if len(archive) > maxReleaseBytes {
		return errors.New("pakiet przekracza limit rozmiaru")
	}
	actual := sha256.Sum256(archive)
	if hex.EncodeToString(actual[:]) != expected {
		return errors.New("niezgodna suma kontrolna pakietu")
	}
	current, e := os.Executable()
	if e != nil {
		return e
	}
	bin, e := extractBinary(archive)
	if e != nil {
		return e
	}
	dir := filepath.Dir(current)
	tmp := filepath.Join(dir, ".ultimatewx-update")
	if e = os.WriteFile(tmp, bin, 0755); e != nil {
		return fmt.Errorf("zapis aktualizacji (sprawdź uprawnienia): %w", e)
	}
	if e = os.Chmod(tmp, 0755); e != nil {
		os.Remove(tmp)
		return e
	}
	if e = os.Rename(tmp, current); e != nil {
		os.Remove(tmp)
		return fmt.Errorf("podmiana aplikacji: %w", e)
	}
	return nil
}
func extractBinary(data []byte) ([]byte, error) {
	gz, e := gzip.NewReader(strings.NewReader(string(data)))
	if e != nil {
		return nil, e
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		h, e := tr.Next()
		if e == io.EOF {
			break
		}
		if e != nil {
			return nil, e
		}
		name := filepath.Base(filepath.Clean(h.Name))
		if name != "ultimatewx" || h.Typeflag != tar.TypeReg {
			continue
		}
		if h.Size <= 0 || h.Size > 50<<20 {
			return nil, errors.New("niepoprawny rozmiar pliku wykonywalnego")
		}
		return io.ReadAll(io.LimitReader(tr, h.Size))
	}
	return nil, errors.New("pakiet nie zawiera pliku ultimatewx")
}
