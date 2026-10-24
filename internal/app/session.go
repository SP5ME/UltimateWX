package app

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

func sessionKey(token string) string {
	digest := sha256.Sum256([]byte(token))
	return hex.EncodeToString(digest[:])
}

func loadSessions(path string) (map[string]sessionState, error) {
	sessions := make(map[string]sessionState)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return sessions, nil
	}
	if err != nil {
		return sessions, err
	}
	if err := json.Unmarshal(data, &sessions); err != nil {
		return make(map[string]sessionState), err
	}
	now := time.Now()
	for tokenHash, state := range sessions {
		if !state.ExpiresAt.After(now) || state.Duration <= 0 {
			delete(sessions, tokenHash)
		}
	}
	return sessions, nil
}

func (a *App) persistSessionsLocked() error {
	data, err := json.Marshal(a.sessions)
	if err != nil {
		return err
	}
	path := filepath.Join(a.dataDir, "sessions.json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		return err
	}
	return os.Chmod(path, 0600)
}
