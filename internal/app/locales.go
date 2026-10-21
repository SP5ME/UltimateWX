package app

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const maxLocaleFileSize = 1 << 20

var localeCodePattern = regexp.MustCompile(`^[a-z]{2,3}(?:-[A-Za-z0-9]{2,8})*$`)

type localeFile struct {
	Label        string            `json:"label"`
	Translations map[string]string `json:"translations"`
}

type availableLocale struct {
	Code  string `json:"code"`
	Label string `json:"label"`
}

func (a *App) locales(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		method(w)
		return
	}
	code := r.URL.Query().Get("code")
	if code != "" {
		locale, err := a.readLocale(code)
		if err != nil {
			writeError(w, http.StatusNotFound, "Locale file not found or invalid.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"code": code, "label": locale.Label, "translations": locale.Translations})
		return
	}

	directory := filepath.Join(a.dataDir, "locales")
	directoryInfo, err := os.Lstat(directory)
	if errors.Is(err, os.ErrNotExist) {
		writeJSON(w, http.StatusOK, map[string]any{"locales": []availableLocale{}})
		return
	}
	if err != nil || !directoryInfo.IsDir() {
		writeError(w, http.StatusInternalServerError, "Could not read locale directory.")
		return
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not read locale directory.")
		return
	}
	locales := make([]availableLocale, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		code := strings.TrimSuffix(entry.Name(), ".json")
		if !localeCodePattern.MatchString(code) {
			continue
		}
		locale, err := a.readLocale(code)
		if err != nil {
			continue
		}
		locales = append(locales, availableLocale{Code: code, Label: locale.Label})
		if len(locales) == 50 {
			break
		}
	}
	sort.Slice(locales, func(i, j int) bool { return locales[i].Code < locales[j].Code })
	writeJSON(w, http.StatusOK, map[string]any{"locales": locales})
}

func (a *App) readLocale(code string) (localeFile, error) {
	if !localeCodePattern.MatchString(code) {
		return localeFile{}, errors.New("invalid locale code")
	}
	path := filepath.Join(a.dataDir, "locales", code+".json")
	info, err := os.Lstat(path)
	if err != nil {
		return localeFile{}, err
	}
	if !info.Mode().IsRegular() || info.Size() > maxLocaleFileSize {
		return localeFile{}, errors.New("locale file is not a regular file or exceeds the size limit")
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return localeFile{}, err
	}
	var locale localeFile
	if err := json.Unmarshal(contents, &locale); err != nil {
		return localeFile{}, err
	}
	locale.Label = strings.TrimSpace(locale.Label)
	if locale.Label == "" || len(locale.Label) > 64 || len(locale.Translations) == 0 {
		return localeFile{}, errors.New("locale file needs a label and translations")
	}
	for key, value := range locale.Translations {
		if strings.TrimSpace(key) == "" || len(key) > 128 || len(value) > 4096 {
			return localeFile{}, errors.New("locale key or translation exceeds its limit")
		}
	}
	return locale, nil
}
