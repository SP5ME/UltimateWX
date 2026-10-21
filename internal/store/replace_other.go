//go:build !windows

package store

import "os"

func replaceStoreFile(source, destination string) error {
	return os.Rename(source, destination)
}
