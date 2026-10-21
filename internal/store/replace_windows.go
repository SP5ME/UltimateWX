//go:build windows

package store

import "os"

func replaceStoreFile(source, destination string) error {
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	if err := os.WriteFile(destination, data, 0600); err != nil {
		return err
	}
	return os.Remove(source)
}
