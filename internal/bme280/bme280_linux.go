//go:build linux

package bme280

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"syscall"
)

const i2cSlave = 0x0703

type calibration struct {
	t1                             uint16
	t2, t3                         int16
	p1                             uint16
	p2, p3, p4, p5, p6, p7, p8, p9 int16
	h1                             uint8
	h2                             int16
	h3                             uint8
	h4, h5                         int16
	h6                             int8
}

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
	if e = writeReg(f, 0xF4, 0x27); e != nil {
		return 0, 0, 0, e
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
func compensateTemperature(adc int32, c calibration) (float64, int64) {
	v1 := ((int64(adc>>3) - int64(c.t1)*2) * int64(c.t2)) >> 11
	d := int64(adc>>4) - int64(c.t1)
	v2 := (((d * d) >> 12) * int64(c.t3)) >> 14
	tFine := v1 + v2
	t := float64((tFine*5+128)>>8) / 100
	return t, tFine
}
func compensatePressure(adc int32, tFine int64, c calibration) float64 {
	v1 := tFine - 128000
	v2 := v1 * v1 * int64(c.p6)
	v2 += ((v1 * int64(c.p5)) << 17)
	v2 += (int64(c.p4) << 35)
	v1 = ((v1 * v1 * int64(c.p3)) >> 8) + ((v1 * int64(c.p2)) << 12)
	v1 = ((((int64(1) << 47) + v1) * int64(c.p1)) >> 33)
	if v1 == 0 {
		return math.NaN()
	}
	p := int64(1048576 - adc)
	p = (((p << 31) - v2) * 3125) / v1
	v1 = (int64(c.p9) * (p >> 13) * (p >> 13)) >> 25
	v2 = (int64(c.p8) * p) >> 19
	p = ((p + v1 + v2) >> 8) + (int64(c.p7) << 4)
	return float64(p) / 256 / 100
}
func compensateHumidity(adc int32, tFine int64, c calibration) float64 {
	v := float64(tFine) - 76800
	v = (float64(adc) - (float64(c.h4)*64 + float64(c.h5)/16384*v)) * (float64(c.h2) / 65536 * (1 + float64(c.h6)/67108864*v*(1+float64(c.h3)/67108864*v)))
	v *= 1 - float64(c.h1)*v/524288
	if v < 0 {
		v = 0
	}
	if v > 100 {
		v = 100
	}
	return v
}
