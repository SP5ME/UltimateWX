# Arduino + BME280 over USB/serial

This standalone sketch sends one UltimateWX-compatible JSON object per line at 9600 baud, once per minute.

## Wiring

Connect the BME280 to the board's I²C SDA, SCL, 3.3 V and GND pins. Pin numbers depend on the Arduino model. Check that the sensor module's logic voltage is compatible with the board. The sketch defaults to address `0x76`; set `BME280_ADDRESS` to `0x77` if selected by the module's SDO/address jumper.

## Setup

1. Install **Adafruit BME280 Library** and its dependency **Adafruit Unified Sensor** through Arduino IDE's Library Manager.
2. Select your board and upload the sketch.
3. Connect the board by USB to the Linux host running UltimateWX. Find its device path (commonly `/dev/ttyACM0` or `/dev/ttyUSB0`) and grant the service user access to that device.
4. In **Configuration → Sensors**, enter the serial device path and baud rate `9600`, then save.

The sketch omits `timestamp` because it assumes the Arduino has no real-time clock. UltimateWX timestamps each received sample. The `pressure_hpa` value is station pressure; the sketch does not apply an altitude correction.
