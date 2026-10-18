//go:build !linux

package bme280

import "fmt"

func Read(device string, address uint16) (temperatureC, humidityPercent, pressureHPa float64, err error) {
	return 0, 0, 0, fmt.Errorf("direct BME280 I2C access is available only on Linux")
}
