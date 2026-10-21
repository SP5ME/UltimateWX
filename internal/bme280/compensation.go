package bme280

import (
	"fmt"
	"math"
	"time"
)

func waitForStatus(readStatus func() (byte, error), mask byte, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		status, err := readStatus()
		if err != nil {
			return err
		}
		if status&mask == 0 {
			return nil
		}
		if !time.Now().Before(deadline) {
			return fmt.Errorf("status bit mask 0x%02X remained set for %s", mask, timeout)
		}
		time.Sleep(time.Millisecond)
	}
}

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
