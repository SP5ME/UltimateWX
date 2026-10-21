# ESP32 + BME280 over MQTT

This standalone Arduino sketch publishes one UltimateWX-compatible JSON sample every 60 seconds.

## Wiring

For a typical ESP32 DevKit, power the BME280 from **3.3 V**, connect GND to GND, SDA to GPIO 21 and SCL to GPIO 22. Board pin assignments may differ. The sketch tries I²C addresses `0x76` and `0x77`; use the module's address jumper/SDO setting if needed. Do not power a 3.3 V ESP32 GPIO from a 5 V sensor signal.

## Setup

1. Install Arduino ESP32 board support and the libraries **Adafruit BME280 Library**, **Adafruit Unified Sensor**, and **ArduinoMqttClient** through Arduino IDE's Library Manager.
2. Edit Wi-Fi credentials, MQTT broker address and (if configured) the dedicated broker username/password in the sketch. Keep credentials private.
3. Select the matching ESP32 board and upload. Open Serial Monitor at `115200` for connection and publish messages.
4. In UltimateWX, configure the same broker (for example `tcp://192.168.1.10:1883`) and MQTT topic filter `ultimatewx/+/weather`.

The broker connection in this example is plain MQTT on the local network. Restrict broker access to the LAN and use an account limited to the required publish topic. Do not expose port 1883 to the Internet.

The pressure value is **station pressure** in hPa, as required by the UltimateWX input format. The ESP32 does not apply an altitude correction. UltimateWX currently derives sea-level pressure automatically for its direct Raspberry Pi BME280 profile; MQTT pressure stays station pressure.
