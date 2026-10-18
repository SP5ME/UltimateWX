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

func List() Devices { return Devices{Inputs: list("arecord"), Outputs: list("aplay")} }
func list(command string) []Device {
	out, err := exec.Command(command, "-L").Output()
	if err != nil {
		return nil
	}
	var devices []Device
	for _, line := range strings.Split(string(out), "\n") {
		trim := strings.TrimSpace(line)
		if trim == "" {
			continue
		}
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			if len(devices) > 0 && devices[len(devices)-1].Description == "" {
				devices[len(devices)-1].Description = trim
			}
			continue
		}
		devices = append(devices, Device{Name: trim})
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
func Speak(text, model, engine, device string) error {
	if strings.TrimSpace(text) == "" {
		return fmt.Errorf("empty announcement")
	}
	if engine == "" {
		engine = "piper"
	}
	if model == "" {
		return fmt.Errorf("select and download a TTS voice first")
	}
	tmp := filepath.Join(os.TempDir(), fmt.Sprintf("ultimatewx-%d.wav", time.Now().UnixNano()))
	defer os.Remove(tmp)
	p := exec.Command(engine, "--model", model, "--output_file", tmp)
	p.Stdin = bytes.NewBufferString(text)
	if b, e := p.CombinedOutput(); e != nil {
		return fmt.Errorf("TTS engine: %w: %s", e, strings.TrimSpace(string(b)))
	}
	return PlayWAV(tmp, device)
}
