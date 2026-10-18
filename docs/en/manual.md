---
lang: en
title: UltimateWX user manual
---

# UltimateWX user manual

UltimateWX is a local weather announcer for Linux and amateur-radio stations. It combines online forecasts, nearby APRS weather frames and sensor readings, then builds a spoken report from configurable blocks. Version 1 does not transmit over radio and does not control PTT.

## Installation

The release installer supports Raspberry Pi OS, Ubuntu and Alpine Linux on ARMv7, ARM64 and x86-64. After the repository is published, run the command shown on the project README as root (normally with `sudo`). The installer checks the distribution, CPU architecture and service manager, downloads the matching archive, verifies its SHA-256 checksum, installs the service and enables autostart. The repository owner in the command must match the published GitHub repository.

Open `http://<device-ip>:8080` on the local network. On first launch, complete the station setup wizard, then choose a password (at least six characters) or continue without login. The service stores configuration and history under `/var/lib/ultimatewx`. Keep this directory in backups; it contains the password hash and integration credentials.

For a development run, install Go 1.23 or newer and run `go run ./cmd/ultimatewx --listen 127.0.0.1:8080`. Development data is stored in `./data`.

## Configuration

On first launch, the station setup wizard appears before panel access is configured. After setup, you can set a password or continue without login. Without a password, anyone who can reach the panel can use it; use this mode only on a trusted local network.

The **Status** page reports the configured sources, audio devices, last online-weather update and announcement preview. **Configuration** is divided into these tabs: Weather station, Station location, Weather sources, Sensors, Audio and voice, and Announcement and schedule. On Weather station, set the callsign and Maidenhead locator. Use the second tab, Station location, to search for a place, set altitude and select a point on the map. Click the map or drag it until the station point is under the crosshair; coordinates are shown in the fields and saved with the shared form. The map uses OpenStreetMap tiles and requires an internet connection. In a text announcement block, use `<call>` for the callsign and `<loc>` for the locator, for example `Weather station <call> located in <loc>.` Add fixed times or an hourly schedule, such as five minutes after every full hour, and enable the entries when the announcement should play. Switching tabs preserves edits; use the shared save button to store the form. Use **Statistics** for retained observations, received frames and playbacks. The **APRS monitor** shows all stored frames or only WX frames and explains whether each WX frame is eligible for weather data.

Open-Meteo is queried every 15 minutes. Its free API is for non-commercial use and its data requires CC BY 4.0 attribution; the panel identifies the attribution. If those terms do not fit your use, disable that source and configure another provider when an adapter is available.

APRS connects to an external Dire Wolf/KISS TCP endpoint, normally `127.0.0.1:8001`. Configure Dire Wolf to use the radio receiver and forward KISS frames to UltimateWX. The default weather-source radius is 10 km. Frames without a supported position, with invalid weather fields, outside the radius or received more than two hours ago remain visible in the monitor but are excluded from weather observations.

Audio input and output are selected separately. The input is the receiver-side audio device used by your Dire Wolf setup; the output is used by `aplay` for the spoken report. Linux ALSA tools (`arecord`, `aplay`) and a working local Piper command are needed for device enumeration and playback. The input choice does not configure Dire Wolf automatically.

The report editor stores ordered blocks as JSON. Each block can be enabled, assigned a type and source, and given fields or text. The schedule editor supports fixed times and hourly repetition with an offset from 0 to 59 minutes after each full hour. Enable or disable each entry individually. Default times are disabled. A `forecast` block reads the four-hour Open-Meteo forecast; a `local-estimate` block is explicitly labelled as an estimate and is omitted without fresh pressure measurements.

## Sensors

All sensor paths use the same concepts: a device name, timestamp, parameter, numeric value and unit. MQTT payloads use one JSON object per message, for example:

```json
{"device":"roof-esp32","timestamp":"2026-10-18T10:00:00Z","temperature_c":12.5,"humidity_percent":72,"pressure_hpa":1008.2}
```

Configure a broker such as `tcp://homeassistant.local:1883` and a topic filter such as `ultimatewx/+/weather`. ESP32 firmware can publish this object over Wi-Fi. Use a dedicated broker account and do not expose credentials in public logs.

Home Assistant integration uses its REST API token. Configure the base URL and a long-lived access token. Sensor entities with recognized device classes (temperature, humidity, pressure, wind speed or precipitation) are read periodically.

For USB/serial devices, select a Linux device such as `/dev/ttyUSB0`, choose the baud rate and send one JSON object followed by a newline for each sample. The same field names as the MQTT example are accepted.

For a Raspberry Pi BME280, enable the direct I²C profile, set `/dev/i2c-1` and address 118 (`0x76`) or 119 (`0x77`), and connect the sensor using the board's I²C pins and correct logic voltage. The application records station pressure and separately derives sea-level pressure from the configured elevation. Confirm the physical sensor address and wiring before enabling the profile.

## FAQ and troubleshooting

**The panel does not open.** Check that the service is running and that TCP port 8080 is reachable on the local network. Use `systemctl status ultimatewx` on systemd or `rc-service ultimatewx status` on OpenRC.

**There is no APRS data.** Verify Dire Wolf's receiver audio, KISS TCP host and port, and that the TNC is delivering AX.25 UI frames. The selected audio input is currently guidance for the receiver configuration; UltimateWX consumes the resulting KISS stream.

**A nearby frame is not used.** Check its position, weather fields, reception time and distance in the APRS monitor. Increase the configured radius only if that matches the station's intended coverage.

**The TTS voice is unavailable.** No language or voice is selected by default. In Voice and announcement, choose a language, load its catalogue and download a voice; the downloaded voice sets the announcement language. Internet access is required for the catalogue. The application shows only recognized licences and verifies model files before installation. Piper itself must be installed and available as the configured command; downloaded voices remain local for offline use.

**What does the offline estimate mean?** It is a simple pressure-trend indicator, not a meteorological forecast. It needs at least two recent measured station-pressure samples at least one hour apart within the last five hours. The announcement names it as a local estimate.

**How do I update?** Use **Check for updates**, review the release information, then select **Install update and restart**. The application verifies the package against the release SHA-256 manifest. The service account must own the installed binary directory, as set by the official installer.

**Where are backups?** Back up `/var/lib/ultimatewx`, including `config.json`, the JSONL history and downloaded voices. Protect the backup because it may contain integration secrets.

\n