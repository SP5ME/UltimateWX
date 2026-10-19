package audio

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Device struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}
type Devices struct {
	Inputs  []Device `json:"inputs"`
	Outputs []Device `json:"outputs"`
}

func List() Devices { return Devices{Inputs: []Device{}, Outputs: list("aplay")} }
func list(command string) []Device {
	out, err := exec.Command(command, "-L").Output()
	if err != nil {
		return nil
	}
	var devices []Device
	seen := map[string]bool{}
	var fallback []Device
	var current *Device
	for _, line := range strings.Split(string(out), "\n") {
		trim := strings.TrimSpace(line)
		if trim == "" {
			continue
		}
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			if current != nil && current.Description == "" {
				current.Description = trim
			}
			continue
		}
		if strings.HasPrefix(trim, "plughw:") {
			if !seen[trim] {
				devices = append(devices, Device{Name: trim})
				current = &devices[len(devices)-1]
				seen[trim] = true
			} else {
				current = nil
			}
			continue
		}
		if trim == "pulse" || trim == "pipewire" || trim == "jack" || trim == "bluealsa" {
			if !seen[trim] {
				devices = append(devices, Device{Name: trim})
				current = &devices[len(devices)-1]
				seen[trim] = true
			} else {
				current = nil
			}
			continue
		}
		// Keep hardware entries as a fallback for ALSA setups without plughw aliases.
		if strings.HasPrefix(trim, "hw:") && !seen[trim] {
			fallback = append(fallback, Device{Name: trim})
			current = &fallback[len(fallback)-1]
			seen[trim] = true
		} else {
			current = nil
		}
	}
	if len(devices) == 0 {
		devices = fallback
	}
	return devices
}
func PlayWAV(path, device string) error {
	args := []string{}
	if device != "" {
		args = append(args, "-D", device)
	}
	args = append(args, path)
	cmd := exec.Command("aplay", args...)
	if b, e := cmd.CombinedOutput(); e != nil {
		return fmt.Errorf("aplay: %w: %s", e, strings.TrimSpace(string(b)))
	}
	return nil
}
func Speak(text, model, engine, device string, volume int) error {
	if strings.TrimSpace(text) == "" {
		return fmt.Errorf("empty announcement")
	}
	if engine == "" {
		engine = "piper"
	}
	if model == "" {
		return fmt.Errorf("select and download a TTS voice first")
	}
	if volume < 0 || volume > 100 {
		return fmt.Errorf("announcement audio volume must be 0..100 percent")
	}
	tmp := filepath.Join(os.TempDir(), fmt.Sprintf("ultimatewx-%d.wav", time.Now().UnixNano()))
	defer os.Remove(tmp)
	piper, err := resolveEngine(engine)
	if err != nil {
		return err
	}
	p := exec.Command(piper, "--model", model, "--output_file", tmp)
	p.Stdin = bytes.NewBufferString(text)
	if b, e := p.CombinedOutput(); e != nil {
		return fmt.Errorf("TTS engine: %w: %s", e, strings.TrimSpace(string(b)))
	}
	if err := setWAVVolume(tmp, volume); err != nil {
		return fmt.Errorf("set announcement volume: %w", err)
	}
	return PlayWAV(tmp, device)
}

func resolveEngine(engine string) (string, error) {
	if engine != "piper" {
		return engine, nil
	}
	if path, err := exec.LookPath(engine); err == nil {
		return path, nil
	}
	for _, path := range []string{
		"/opt/ultimatewx/piper-venv/bin/piper",
	} {
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0111 != 0 {
			return path, nil
		}
	}
	return "", fmt.Errorf("Piper TTS nie jest zainstalowany; uruchom aktualizację aplikacji albo instalator systemowy")
}
