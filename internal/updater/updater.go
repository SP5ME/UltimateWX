package updater

import (
	"archive/tar"
	"bytes"
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
	"strconv"
	"strings"
	"time"
)

var Repository = ""
var Version = "dev"

const maxReleaseBytes = 100 << 20

type Release struct {
	TagName     string    `json:"tag_name"`
	Draft       bool      `json:"draft"`
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
	if comparison, ok := compareVersions(r.TagName, Version); ok {
		out.Available = comparison > 0
	} else {
		out.Available = normalize(r.TagName) != normalize(Version)
	}
	asset := archiveName(goos, arch)
	if !hasAsset(r, asset) || !hasAsset(r, "SHA256SUMS") {
		out.Available = false
		out.Error = "Wydanie nie zawiera pakietu dla tej platformy albo pliku SHA256SUMS."
	}
	return out
}
func normalize(v string) string { return strings.TrimPrefix(strings.TrimSpace(v), "v") }
func latestRelease(ctx context.Context) (Release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/repos/"+Repository+"/releases?per_page=100", nil)
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
	release, ok := latestSemverRelease(releases)
	if !ok {
		return Release{}, errors.New("brak opublikowanych wydań")
	}
	return release, nil
}

func latestSemverRelease(releases []Release) (Release, bool) {
	var latest Release
	found := false
	for _, release := range releases {
		if release.Draft {
			continue
		}
		if _, _, _, _, ok := parseVersion(release.TagName); !ok {
			continue
		}
		if !found {
			latest, found = release, true
			continue
		}
		if comparison, ok := compareVersions(release.TagName, latest.TagName); ok && comparison > 0 {
			latest = release
		}
	}
	return latest, found
}

func compareVersions(a, b string) (int, bool) {
	aMajor, aMinor, aPatch, aPre, aOK := parseVersion(a)
	bMajor, bMinor, bPatch, bPre, bOK := parseVersion(b)
	if !aOK || !bOK {
		return 0, false
	}
	for _, pair := range [][2]uint64{{aMajor, bMajor}, {aMinor, bMinor}, {aPatch, bPatch}} {
		if pair[0] < pair[1] {
			return -1, true
		}
		if pair[0] > pair[1] {
			return 1, true
		}
	}
	if len(aPre) == 0 && len(bPre) > 0 {
		return 1, true
	}
	if len(aPre) > 0 && len(bPre) == 0 {
		return -1, true
	}
	for i := 0; i < len(aPre) && i < len(bPre); i++ {
		aNumeric, bNumeric := numericIdentifier(aPre[i]), numericIdentifier(bPre[i])
		switch {
		case aNumeric && bNumeric:
			if len(aPre[i]) < len(bPre[i]) {
				return -1, true
			}
			if len(aPre[i]) > len(bPre[i]) {
				return 1, true
			}
			if aPre[i] < bPre[i] {
				return -1, true
			}
			if aPre[i] > bPre[i] {
				return 1, true
			}
		case aNumeric && !bNumeric:
			return -1, true
		case !aNumeric && bNumeric:
			return 1, true
		default:
			if aPre[i] < bPre[i] {
				return -1, true
			}
			if aPre[i] > bPre[i] {
				return 1, true
			}
		}
	}
	if len(aPre) < len(bPre) {
		return -1, true
	}
	if len(aPre) > len(bPre) {
		return 1, true
	}
	return 0, true
}

func parseVersion(version string) (uint64, uint64, uint64, []string, bool) {
	version = normalize(version)
	if version == "" || strings.Count(version, "+") > 1 {
		return 0, 0, 0, nil, false
	}
	version, _, _ = strings.Cut(version, "+")
	core, preText, hasPre := strings.Cut(version, "-")
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return 0, 0, 0, nil, false
	}
	var numbers [3]uint64
	for i, part := range parts {
		if part == "" || (len(part) > 1 && part[0] == '0') {
			return 0, 0, 0, nil, false
		}
		for _, ch := range part {
			if ch < '0' || ch > '9' {
				return 0, 0, 0, nil, false
			}
		}
		number, err := strconv.ParseUint(part, 10, 64)
		if err != nil {
			return 0, 0, 0, nil, false
		}
		numbers[i] = number
	}
	var pre []string
	if hasPre {
		pre = strings.Split(preText, ".")
		for _, identifier := range pre {
			if identifier == "" {
				return 0, 0, 0, nil, false
			}
			numeric := true
			for _, ch := range identifier {
				if !((ch >= '0' && ch <= '9') || (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || ch == '-') {
					return 0, 0, 0, nil, false
				}
				if ch < '0' || ch > '9' {
					numeric = false
				}
			}
			if numeric && len(identifier) > 1 && identifier[0] == '0' {
				return 0, 0, 0, nil, false
			}
		}
	}
	return numbers[0], numbers[1], numbers[2], pre, true
}

func numericIdentifier(identifier string) bool {
	for _, ch := range identifier {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return identifier != ""
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
	if comparison, ok := compareVersions(r.TagName, Version); ok && comparison <= 0 {
		return errors.New("brak nowszego wydania do zainstalowania")
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
	matches := 0
	for _, line := range strings.Split(string(sumBytes), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == name {
			matches++
			expected = strings.ToLower(fields[0])
		}
	}
	if matches != 1 || len(expected) != sha256.Size*2 {
		return errors.New("manifest SHA-256 musi zawierać dokładnie jeden poprawny wpis dla pakietu")
	}
	if _, err := hex.DecodeString(expected); err != nil {
		return errors.New("manifest zawiera niepoprawną sumę SHA-256 pakietu")
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
	gz, e := gzip.NewReader(bytes.NewReader(data))
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
		binary, err := io.ReadAll(io.LimitReader(tr, h.Size))
		if err != nil {
			return nil, err
		}
		if int64(len(binary)) != h.Size {
			return nil, errors.New("pakiet zawiera niekompletny plik wykonywalny")
		}
		return binary, nil
	}
	return nil, errors.New("pakiet nie zawiera pliku ultimatewx")
}
