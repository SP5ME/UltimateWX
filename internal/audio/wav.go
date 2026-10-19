package audio

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"os"
)

// setWAVVolume applies a software output level to uncompressed PCM/IEEE-float WAV data.
func setWAVVolume(path string, percent int) error {
	if percent == 100 {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if len(data) < 12 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return errors.New("Piper output is not a RIFF/WAVE file")
	}
	var format uint16
	var bits uint16
	var hasFormat, hasData bool
	var chunks [][2]int
	for offset := 12; offset+8 <= len(data); {
		chunkSize := uint64(binary.LittleEndian.Uint32(data[offset+4 : offset+8]))
		start := uint64(offset + 8)
		end := start + chunkSize
		if end > uint64(len(data)) {
			return errors.New("invalid WAV chunk length")
		}
		switch string(data[offset : offset+4]) {
		case "fmt ":
			if chunkSize < 16 {
				return errors.New("invalid WAV format chunk")
			}
			format = binary.LittleEndian.Uint16(data[start : start+2])
			bits = binary.LittleEndian.Uint16(data[start+14 : start+16])
			if format == 0xfffe && chunkSize >= 40 {
				format = binary.LittleEndian.Uint16(data[start+24 : start+26])
			}
			hasFormat = true
		case "data":
			hasData = true
			chunks = append(chunks, [2]int{int(start), int(end)})
		}
		next := end + chunkSize%2
		if next > uint64(len(data)) {
			return errors.New("invalid WAV padding")
		}
		offset = int(next)
	}
	if !hasFormat || !hasData {
		return errors.New("WAV file is missing format or audio data")
	}
	for _, chunk := range chunks {
		if err := scaleWAVSamples(data[chunk[0]:chunk[1]], format, bits, percent); err != nil {
			return err
		}
	}
	return os.WriteFile(path, data, 0600)
}

func scaleWAVSamples(data []byte, format, bits uint16, percent int) error {
	gain := float64(percent) / 100
	switch {
	case format == 1 && bits == 8:
		for i, sample := range data {
			value := math.Round((float64(sample)-128)*gain) + 128
			data[i] = byte(math.Max(0, math.Min(255, value)))
		}
	case format == 1 && bits == 16:
		if len(data)%2 != 0 {
			return errors.New("invalid 16-bit PCM data length")
		}
		for i := 0; i < len(data); i += 2 {
			value := int16(binary.LittleEndian.Uint16(data[i : i+2]))
			scaled := math.Round(float64(value) * gain)
			binary.LittleEndian.PutUint16(data[i:i+2], uint16(int16(math.Max(-32768, math.Min(32767, scaled)))))
		}
	case format == 1 && bits == 24:
		if len(data)%3 != 0 {
			return errors.New("invalid 24-bit PCM data length")
		}
		for i := 0; i < len(data); i += 3 {
			value := int32(data[i]) | int32(data[i+1])<<8 | int32(data[i+2])<<16
			if value&0x800000 != 0 {
				value |= ^int32(0xffffff)
			}
			scaled := int64(math.Round(float64(value) * gain))
			if scaled > 0x7fffff {
				scaled = 0x7fffff
			} else if scaled < -0x800000 {
				scaled = -0x800000
			}
			encoded := uint32(int32(scaled))
			data[i], data[i+1], data[i+2] = byte(encoded), byte(encoded>>8), byte(encoded>>16)
		}
	case format == 1 && bits == 32:
		if len(data)%4 != 0 {
			return errors.New("invalid 32-bit PCM data length")
		}
		for i := 0; i < len(data); i += 4 {
			value := int32(binary.LittleEndian.Uint32(data[i : i+4]))
			scaled := math.Round(float64(value) * gain)
			binary.LittleEndian.PutUint32(data[i:i+4], uint32(int32(math.Max(-2147483648, math.Min(2147483647, scaled)))))
		}
	case format == 3 && bits == 32:
		if len(data)%4 != 0 {
			return errors.New("invalid 32-bit float WAV data length")
		}
		for i := 0; i < len(data); i += 4 {
			value := math.Float32frombits(binary.LittleEndian.Uint32(data[i : i+4]))
			scaled := float32(math.Max(-1, math.Min(1, float64(value)*gain)))
			binary.LittleEndian.PutUint32(data[i:i+4], math.Float32bits(scaled))
		}
	default:
		return fmt.Errorf("unsupported WAV encoding %d with %d-bit samples", format, bits)
	}
	return nil
}
