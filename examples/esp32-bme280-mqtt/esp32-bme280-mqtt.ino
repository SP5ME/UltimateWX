#include <Adafruit_BME280.h>
#include <ArduinoMqttClient.h>
#include <WiFi.h>
#include <Wire.h>
#include <time.h>

// Fill in the local Wi-Fi and MQTT broker settings before uploading.
const char *WIFI_SSID = "YOUR_WIFI_SSID";
const char *WIFI_PASSWORD = "YOUR_WIFI_PASSWORD";
const char *MQTT_HOST = "192.168.1.10";
const uint16_t MQTT_PORT = 1883;
const char *MQTT_USERNAME = ""; // Optional; use a dedicated broker account.
const char *MQTT_PASSWORD = "";
const char *MQTT_TOPIC = "ultimatewx/roof/weather";
const char *DEVICE_ID = "roof-esp32";
const uint32_t SAMPLE_INTERVAL_MS = 60000;

WiFiClient network;
MqttClient mqtt(network);
Adafruit_BME280 bme;
uint32_t lastSample = 0;
bool hasSample = false;

bool connectWiFi() {
  if (WiFi.status() == WL_CONNECTED) return true;
  WiFi.mode(WIFI_STA);
  WiFi.begin(WIFI_SSID, WIFI_PASSWORD);
  Serial.print("Connecting to Wi-Fi");
  for (uint8_t attempt = 0; attempt < 30 && WiFi.status() != WL_CONNECTED; ++attempt) {
    delay(500);
    Serial.print('.');
  }
  Serial.println();
  return WiFi.status() == WL_CONNECTED;
}

bool connectMqtt() {
  if (mqtt.connected()) return true;
  mqtt.setId(DEVICE_ID);
  mqtt.setKeepAliveInterval(30 * 1000);
  if (MQTT_USERNAME[0] != '\0') mqtt.setUsernamePassword(MQTT_USERNAME, MQTT_PASSWORD);
  Serial.printf("Connecting to MQTT %s:%u...\n", MQTT_HOST, MQTT_PORT);
  return mqtt.connect(MQTT_HOST, MQTT_PORT);
}

void setup() {
  Serial.begin(115200);
  Wire.begin(); // ESP32 DevKit defaults: SDA GPIO 21, SCL GPIO 22.
  if (!bme.begin(0x76) && !bme.begin(0x77)) {
    Serial.println("BME280 not found at 0x76 or 0x77; check wiring/address.");
    while (true) delay(1000);
  }
  if (!connectWiFi()) Serial.println("Wi-Fi unavailable; will retry.");
  configTime(0, 0, "pool.ntp.org", "time.nist.gov");
}

void loop() {
  if (!connectWiFi() || !connectMqtt()) {
    delay(5000);
    return;
  }
  mqtt.poll();
  const uint32_t now = millis();
  if (hasSample && now - lastSample < SAMPLE_INTERVAL_MS) {
    delay(100);
    return;
  }

  struct tm utc;
  char timestamp[25];
  if (!getLocalTime(&utc, 1000)) {
    Serial.println("Waiting for NTP time; sample skipped.");
    delay(5000);
    return;
  }
  lastSample = now;
  hasSample = true;
  strftime(timestamp, sizeof(timestamp), "%Y-%m-%dT%H:%M:%SZ", &utc);
  char payload[256];
  const int length = snprintf(payload, sizeof(payload),
      "{\"device\":\"%s\",\"timestamp\":\"%s\",\"temperature_c\":%.2f,\"humidity_percent\":%.2f,\"pressure_hpa\":%.2f}",
      DEVICE_ID, timestamp, bme.readTemperature(), bme.readHumidity(), bme.readPressure() / 100.0F);
  if (length <= 0 || length >= static_cast<int>(sizeof(payload))) {
    Serial.println("Could not encode sample.");
    return;
  }
  mqtt.beginMessage(MQTT_TOPIC);
  mqtt.write(reinterpret_cast<const uint8_t *>(payload), length);
  if (mqtt.endMessage() == 1) Serial.printf("Published %s\n", payload);
  else Serial.println("MQTT publish failed.");
}
