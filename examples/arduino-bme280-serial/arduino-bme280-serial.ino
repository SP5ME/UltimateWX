#include <Adafruit_BME280.h>
#include <Wire.h>

// Set this to 0x77 if the BME280 SDO/address jumper selects that address.
const uint8_t BME280_ADDRESS = 0x76;
const char *DEVICE_ID = "usb-bme280";
const unsigned long SAMPLE_INTERVAL_MS = 60000;

Adafruit_BME280 bme;
unsigned long lastSample = 0;
bool hasSample = false;

void setup() {
  Serial.begin(9600);
  while (!Serial) { } // Needed on native-USB boards; harmless on classic Uno.
  Wire.begin();
  if (!bme.begin(BME280_ADDRESS)) {
    Serial.println(F("BME280 not found; check wiring and I2C address."));
    while (true) { delay(1000); }
  }
}

void loop() {
  const unsigned long now = millis();
  if (hasSample && now - lastSample < SAMPLE_INTERVAL_MS) {
    delay(100);
    return;
  }
  lastSample = now;
  hasSample = true;

  // No hardware clock is assumed. UltimateWX timestamps this sample on receipt.
  Serial.print(F("{\"device\":\""));
  Serial.print(DEVICE_ID);
  Serial.print(F("\",\"temperature_c\":"));
  Serial.print(bme.readTemperature(), 2);
  Serial.print(F(",\"humidity_percent\":"));
  Serial.print(bme.readHumidity(), 2);
  Serial.print(F(",\"pressure_hpa\":"));
  Serial.print(bme.readPressure() / 100.0F, 2);
  Serial.println(F("}")); // One JSON object per line, as expected by UltimateWX.
}
