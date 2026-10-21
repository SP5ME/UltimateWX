//go:build windows

package config

import "os"

func replaceConfigFile(source, destination string) error {
	// Windows is a development/test platform only; its sandbox filesystem can
	// deny rename-based replacement, so keep local runs functional there.
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	if err := os.WriteFile(destination, data, 0600); err != nil {
		return err
	}
	return os.Remove(source)
}
