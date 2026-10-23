package audio

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"math"
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

// TestOutput plays a short tone without depending on the TTS engine or a voice model.
func TestOutput(ctx context.Context, device string, volume int) error {
	if volume < 0 || volume > 100 {
		return fmt.Errorf("audio volume must be 0..100 percent")
	}
	f, err := os.CreateTemp("", "ultimatewx-audio-test-*.wav")
	if err != nil {
		return fmt.Errorf("create audio test WAV: %w", err)
	}
	path := f.Name()
	defer os.Remove(path)

	const sampleRate = 16000
	const duration = 600 * time.Millisecond
	samples := int(float64(sampleRate) * duration.Seconds())
	dataSize := samples * 2
	header := make([]byte, 44)
	copy(header[0:4], "RIFF")
	binary.LittleEndian.PutUint32(header[4:8], uint32(36+dataSize))
	copy(header[8:12], "WAVEfmt ")
	binary.LittleEndian.PutUint32(header[16:20], 16)
	binary.LittleEndian.PutUint16(header[20:22], 1)
	binary.LittleEndian.PutUint16(header[22:24], 1)
	binary.LittleEndian.PutUint32(header[24:28], sampleRate)
	binary.LittleEndian.PutUint32(header[28:32], sampleRate*2)
	binary.LittleEndian.PutUint16(header[32:34], 2)
	binary.LittleEndian.PutUint16(header[34:36], 16)
	copy(header[36:40], "data")
	binary.LittleEndian.PutUint32(header[40:44], uint32(dataSize))
	if _, err := f.Write(header); err != nil {
		f.Close()
		return fmt.Errorf("write audio test WAV header: %w", err)
	}
	for i := 0; i < samples; i++ {
		// Fade the tone in and out to avoid clicks on playback.
		envelope := math.Min(1, math.Min(float64(i)/400, float64(samples-1-i)/400))
		value := int16(math.Sin(2*math.Pi*440*float64(i)/sampleRate) * 6000 * envelope)
		if err := binary.Write(f, binary.LittleEndian, value); err != nil {
			f.Close()
			return fmt.Errorf("write audio test WAV data: %w", err)
		}
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return fmt.Errorf("flush audio test WAV: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close audio test WAV: %w", err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Size() != int64(44+dataSize) {
		if err != nil {
			return fmt.Errorf("verify audio test WAV: %w", err)
		}
		return fmt.Errorf("verify audio test WAV: incomplete file (%d bytes)", info.Size())
	}
	if err := setWAVVolume(path, volume); err != nil {
		return fmt.Errorf("prepare audio test WAV: %w", err)
	}
	return PlayWAV(ctx, path, device)
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
	info, err := os.Stat(tmp)
	if err != nil {
		return fmt.Errorf("TTS engine did not create its WAV output: %w", err)
	}
	if info.Size() < 44 {
		return fmt.Errorf("TTS engine created an incomplete WAV output (%d bytes)", info.Size())
	}
	if err := setWAVVolume(tmp, volume); err != nil {
		return fmt.Errorf("set announcement volume: %w", err)
	}
	return PlayWAV(ctx, tmp, device)
}

func resolveEngine(engine string) (string, error) {
	if engine != "piper" {
		path, err := exec.LookPath(engine)
		if err != nil {
			return "", fmt.Errorf("TTS engine %q was not found: %w", engine, err)
		}
		return path, nil
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
