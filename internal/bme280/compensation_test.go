package bme280

import (
	"math"
	"testing"
)

func TestCompensateBoschTemperatureAndPressureExample(t *testing.T) {
	cal := calibration{
		t1: 27504, t2: 26435, t3: -1000,
		p1: 36477, p2: -10685, p3: 3024, p4: 2855, p5: 140, p6: -7, p7: 15500, p8: -14600, p9: 6000,
	}
	temperature, tFine := compensateTemperature(519888, cal)
	if math.Abs(temperature-25.08) > 0.01 {
		t.Fatalf("compensated temperature = %.4f °C, want 25.08 °C", temperature)
	}
	pressure := compensatePressure(415148, tFine, cal)
	if math.Abs(pressure-1006.5325) > 0.02 {
		t.Fatalf("compensated pressure = %.4f hPa, want about 1006.53 hPa", pressure)
	}
}

func TestCompensatePressureRejectsZeroCalibrationDivisor(t *testing.T) {
	if pressure := compensatePressure(415148, 128422, calibration{}); !math.IsNaN(pressure) {
		t.Fatalf("pressure with zero calibration divisor = %v, want NaN", pressure)
	}
}

func TestCompensateHumidityClampsOutput(t *testing.T) {
	cal := calibration{h1: 75, h2: 362, h3: 0, h4: 334, h5: 50, h6: 30}
	for _, adc := range []int32{-1000, 0, 32768, 65535, 100000} {
		humidity := compensateHumidity(adc, 128422, cal)
		if math.IsNaN(humidity) || humidity < 0 || humidity > 100 {
			t.Errorf("compensateHumidity(%d) = %v, want a value in 0..100%%", adc, humidity)
		}
	}
}
