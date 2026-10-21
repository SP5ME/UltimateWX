//go:build linux

package bme280

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"syscall"
	"time"
)

const i2cSlave = 0x0703

func Read(device string, address uint16) (temperatureC, humidityPercent, pressureHPa float64, err error) {
	f, e := os.OpenFile(device, os.O_RDWR, 0)
	if e != nil {
		return 0, 0, 0, e
	}
	defer f.Close()
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), i2cSlave, uintptr(address))
	if errno != 0 {
		return 0, 0, 0, errno
	}
	chip, e := readRegs(f, 0xD0, 1)
	if e != nil {
		return 0, 0, 0, e
	}
	if chip[0] != 0x60 {
		return 0, 0, 0, fmt.Errorf("I2C 0x%02X is not a BME280 (chip ID 0x%02X)", address, chip[0])
	}
	if e = waitForStatus(func() (byte, error) {
		status, err := readRegs(f, 0xF3, 1)
		if err != nil {
			return 0, err
		}
		return status[0], nil
	}, 0x01, 100*time.Millisecond); e != nil {
		return 0, 0, 0, fmt.Errorf("wait for BME280 calibration copy: %w", e)
	}
	cal, e := readCalibration(f)
	if e != nil {
		return 0, 0, 0, e
	}
	if cal.p1 == 0 {
		return 0, 0, 0, fmt.Errorf("invalid BME280 calibration")
	}
	if e = writeReg(f, 0xF2, 0x01); e != nil {
		return 0, 0, 0, e
	}
	if e = writeReg(f, 0xF5, 0xA0); e != nil {
		return 0, 0, 0, e
	}
	if e = writeReg(f, 0xF4, 0x25); e != nil {
		return 0, 0, 0, e
	}
	// The configured ×1 oversampling for humidity, pressure and temperature
	// has a 9.3 ms maximum conversion time according to the BME280 datasheet.
	time.Sleep(12 * time.Millisecond)
	if e = waitForStatus(func() (byte, error) {
		status, err := readRegs(f, 0xF3, 1)
		if err != nil {
			return 0, err
		}
		return status[0], nil
	}, 0x09, 100*time.Millisecond); e != nil {
		return 0, 0, 0, fmt.Errorf("wait for BME280 measurement: %w", e)
	}
	raw, e := readRegs(f, 0xF7, 8)
	if e != nil {
		return 0, 0, 0, e
	}
	adcP := int32(raw[0])<<12 | int32(raw[1])<<4 | int32(raw[2])>>4
	adcT := int32(raw[3])<<12 | int32(raw[4])<<4 | int32(raw[5])>>4
	adcH := int32(raw[6])<<8 | int32(raw[7])
	temperatureC, tFine := compensateTemperature(adcT, cal)
	pressureHPa = compensatePressure(adcP, tFine, cal)
	humidityPercent = compensateHumidity(adcH, tFine, cal)
	if math.IsNaN(pressureHPa) || pressureHPa < 300 || pressureHPa > 1200 {
		return 0, 0, 0, fmt.Errorf("BME280 returned implausible pressure")
	}
	return temperatureC, humidityPercent, pressureHPa, nil
}
func readCalibration(f *os.File) (calibration, error) {
	var c calibration
	tp, e := readRegs(f, 0x88, 26)
	if e != nil {
		return c, e
	}
	c.t1 = binary.LittleEndian.Uint16(tp[0:2])
	c.t2 = int16(binary.LittleEndian.Uint16(tp[2:4]))
	c.t3 = int16(binary.LittleEndian.Uint16(tp[4:6]))
	c.p1 = binary.LittleEndian.Uint16(tp[6:8])
	c.p2 = int16(binary.LittleEndian.Uint16(tp[8:10]))
	c.p3 = int16(binary.LittleEndian.Uint16(tp[10:12]))
	c.p4 = int16(binary.LittleEndian.Uint16(tp[12:14]))
	c.p5 = int16(binary.LittleEndian.Uint16(tp[14:16]))
	c.p6 = int16(binary.LittleEndian.Uint16(tp[16:18]))
	c.p7 = int16(binary.LittleEndian.Uint16(tp[18:20]))
	c.p8 = int16(binary.LittleEndian.Uint16(tp[20:22]))
	c.p9 = int16(binary.LittleEndian.Uint16(tp[22:24]))
	c.h1 = tp[25]
	h, e := readRegs(f, 0xE1, 7)
	if e != nil {
		return c, e
	}
	c.h2 = int16(binary.LittleEndian.Uint16(h[0:2]))
	c.h3 = h[2]
	c.h4 = int16(int16(h[3])<<4 | int16(h[4]&0x0F))
	if c.h4&0x800 != 0 {
		c.h4 |= ^0x0FFF
	}
	c.h5 = int16(int16(h[5])<<4 | int16(h[4]>>4))
	if c.h5&0x800 != 0 {
		c.h5 |= ^0x0FFF
	}
	c.h6 = int8(h[6])
	return c, nil
}
func readRegs(f *os.File, register byte, n int) ([]byte, error) {
	if _, e := f.Write([]byte{register}); e != nil {
		return nil, e
	}
	out := make([]byte, n)
	_, e := io.ReadFull(f, out)
	return out, e
}
func writeReg(f *os.File, register, value byte) error {
	_, e := f.Write([]byte{register, value})
	return e
}
