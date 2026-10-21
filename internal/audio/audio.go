package audio

import (
	"bytes"
	"context"
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

func List() Devices { return Devices{Inputs: list("arecord"), Outputs: list("aplay")} }

// Readiness reports whether the configured speech model and required commands
// are present. It does not attempt playback, which may still fail for a device.
func Readiness(engine, model string) (engineReady, modelReady, playerReady bool) {
	if engine == "" {
		engine = "piper"
	}
	_, engineErr := resolveEngine(engine)
	_, modelErr := os.Stat(model)
	_, playerErr := exec.LookPath("aplay")
	return engineErr == nil, modelErr == nil, playerErr == nil
}

func list(command string) []Device {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, command, "-L").Output()
	if err != nil {
		return nil
	}
	return parseDevices(string(out))
}

func parseDevices(output string) []Device {
	var devices []Device
	seen := map[string]bool{}
	var fallback []Device
	currentDevice, currentFallback := -1, -1
	for _, line := range strings.Split(output, "\n") {
		trim := strings.TrimSpace(line)
		if trim == "" {
			continue
		}
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			if currentDevice >= 0 && devices[currentDevice].Description == "" {
				devices[currentDevice].Description = trim
			} else if currentFallback >= 0 && fallback[currentFallback].Description == "" {
				fallback[currentFallback].Description = trim
			}
			continue
		}
		if trim == "default" || strings.HasPrefix(trim, "plughw:") || strings.HasPrefix(trim, "sysdefault:") || strings.HasPrefix(trim, "front:") || strings.HasPrefix(trim, "surround") || strings.HasPrefix(trim, "dsnoop:") {
			if !seen[trim] {
				devices = append(devices, Device{Name: trim})
				currentDevice, currentFallback = len(devices)-1, -1
				seen[trim] = true
			} else {
				currentDevice, currentFallback = -1, -1
			}
			continue
		}
		if trim == "pulse" || trim == "pipewire" || trim == "jack" || trim == "bluealsa" {
			if !seen[trim] {
				devices = append(devices, Device{Name: trim})
				currentDevice, currentFallback = len(devices)-1, -1
				seen[trim] = true
			} else {
				currentDevice, currentFallback = -1, -1
			}
			continue
		}
		// Keep hardware entries as a fallback for ALSA setups without plughw aliases.
		if strings.HasPrefix(trim, "hw:") && !seen[trim] {
			fallback = append(fallback, Device{Name: trim})
			currentDevice, currentFallback = -1, len(fallback)-1
			seen[trim] = true
		} else {
			currentDevice, currentFallback = -1, -1
		}
	}
	if len(devices) == 0 {
		devices = fallback
	}
	return devices
}

// TestInput captures and discards one second of audio from the selected ALSA device.
func TestInput(ctx context.Context, device string) error {
	if strings.TrimSpace(device) == "" {
		device = "default"
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "arecord", "-q", "-D", device, "-t", "raw", "-f", "S16_LE", "-r", "16000", "-c", "1", "-d", "1", "/dev/null")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("arecord: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}
func PlayWAV(ctx context.Context, path, device string) error {
	args := []string{}
	if device != "" {
		args = append(args, "-D", device)
	}
	args = append(args, path)
	cmd := exec.CommandContext(ctx, "aplay", args...)
	if b, e := cmd.CombinedOutput(); e != nil {
		return fmt.Errorf("aplay: %w: %s", e, strings.TrimSpace(string(b)))
	}
	return nil
}
func Speak(ctx context.Context, text, model, engine, device string, volume int) error {
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
	p := exec.CommandContext(ctx, piper, "--model", model, "--output_file", tmp)
	p.Stdin = bytes.NewBufferString(text)
	if b, e := p.CombinedOutput(); e != nil {
		return fmt.Errorf("TTS engine: %w: %s", e, strings.TrimSpace(string(b)))
	}
	if err := setWAVVolume(tmp, volume); err != nil {
		return fmt.Errorf("set announcement volume: %w", err)
	}
	return PlayWAV(ctx, tmp, device)
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
