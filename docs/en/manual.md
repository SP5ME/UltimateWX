---
lang: en
title: UltimateWX user manual
---

# UltimateWX user manual

UltimateWX is a local weather announcer for Linux and amateur-radio stations. It combines online forecasts, nearby APRS weather frames and sensor readings, then builds a spoken report from configurable blocks. Version 1 does not transmit over radio and does not control PTT.

## Installation

The release installer supports Raspberry Pi OS, Ubuntu and Alpine Linux on ARMv7, ARM64 and x86-64. Minimum supported versions are Raspberry Pi OS/Debian 12, Ubuntu 24.04 LTS and Alpine Linux 3.22. After the repository is published, run the command shown on the project README as root (normally with `sudo`). The installer checks the distribution, CPU architecture and service manager, downloads the matching archive, verifies its SHA-256 checksum, installs the service and enables autostart. On Raspberry Pi OS/Debian and Ubuntu with x86-64 or ARM64, it also installs Piper TTS in an isolated Python environment. Piper is optional on Alpine and ARMv7 because the official package does not publish a compatible prebuilt wheel for those platforms. The repository owner in the command must match the published GitHub repository.

Open `http://<device-ip>:8080` on the local network. On first launch, read the one-time token with `sudo cat /var/lib/ultimatewx/setup-token` and enter it in the panel before choosing a password (at least six characters) or continuing without login. The station setup wizard appears after access is configured. The service stores configuration and history under `/var/lib/ultimatewx`. Keep this directory in backups; it contains the password hash and integration credentials.

### Uninstalling

Download `uninstall.sh` from the release assets or `scripts/uninstall.sh` from the repository, then run `sudo sh uninstall.sh`. It stops and removes the systemd or OpenRC service, application files and Piper files installed with UltimateWX. By default, it keeps `/var/lib/ultimatewx` and the service account so configuration, history and voices remain available. To remove that data permanently as well, explicitly run `sudo sh uninstall.sh --purge-data`; this option deletes the data directory.

For a development run, install Go 1.23 or newer and run `go run ./cmd/ultimatewx --listen 127.0.0.1:8080`. Development data is stored in `./data`.

## Interface language

The panel includes English, Polish, German, French, Spanish and Italian. Add another language without rebuilding the program by placing a JSON file in the `locales` directory inside the application data directory (default: `/var/lib/ultimatewx/locales`), then reload the panel. See [`examples/locales`](../../examples/locales) for a sample and the file format. Missing translations fall back to English. This changes interface text only; spoken announcements still use the language of the selected TTS voice.

## Configuration

On first launch, the station setup wizard appears before panel access is configured. After setup, you can set a password or continue without login. Without a password, anyone who can reach the panel can use it; use this mode only on a trusted local network.

The **Configuration → Application** tab contains update checking and installation, plus configuration backup export and import. The backup includes integration settings, including MQTT passwords and Home Assistant tokens, but excludes the panel password and measurement history. Store the backup file securely.


The **Status** page reports configured sources, the number of audio outputs and whether the selected output is still available, the last online-weather update and the announcement preview. **Configuration** is divided into these tabs: Weather station, Station location, Weather sources, Sensors, Audio and voice, and Announcement and schedule. On Weather station, set the callsign and Maidenhead locator. Use the second tab, Station location, to search for a place, set altitude and select a point on the map. Click the map or drag it until the station point is under the crosshair; coordinates are shown in the fields and saved with the shared form. The map uses OpenStreetMap tiles and requires an internet connection. In a text announcement block, use `<call>` for the callsign and `<loc>` for the locator, for example `Weather station <call> located in <loc>.` Add fixed times or an hourly schedule, such as five minutes after every full hour, and enable the entries when the announcement should play. Switching tabs preserves edits; use the shared save button to store the form. Use **Statistics** for retained observations, received frames and playbacks. The **APRS monitor** shows all stored frames or only WX frames and explains whether each WX frame is eligible for weather data. The chart range is selected separately; History (days) controls retention for readings, APRS frames and playbacks. APRS frames also have a 10,000-entry cap. For announcement blocks using automatic sources, set a priority order for each parameter; the order determines which available readings are preferred. The aviation profile uses °C, knots (kt), hPa and mm; the imperial profile uses °F, mph, hPa and inches. For weather blocks, set up to five fallback sources in order after the primary source. Each weather block can also use global or block-specific units and standard or compact speech. Set Station time zone to an IANA name such as `Europe/London`; it controls the schedule, spoken date and warning validity times.

Open-Meteo is queried every 15 minutes. Its free API is for non-commercial use and its data requires CC BY 4.0 attribution; the panel identifies the attribution. If those terms do not fit your use, disable that source and configure another provider when an adapter is available.

### IMGW, DWD and GeoSphere Austria warnings

The Warnings page selects warning data using the saved station coordinates. In Poland, it retrieves active and upcoming IMGW warnings for the county resolved automatically from station coordinates through GUGiK ULDK. In Germany, it uses the Deutscher Wetterdienst (DWD) CAP feed and displays alerts only when their geometry contains the station point. CAP `Update` and `Cancel` messages replace the referenced earlier versions so a cancelled warning does not remain active. In Austria, it queries the official GeoSphere Austria API for the municipality containing the saved station coordinates. DWD alert text follows the configured TTS voice language, with English as a fallback; the panel language remains a separate setting. Internet access is required, and German alerts without geometry or a valid period are skipped. For other countries, the application uses the public MeteoAlarm/WIS2 stream and confirms station coverage against the CAP area polygon; alerts without a polygon are skipped. The stream does not replay notifications missed during an outage; saved alerts remain until they expire. Data refreshes every 10 minutes. Ended warnings are available in a collapsible 30-day archive, capped at 500 entries. Spoken alerts repeat at separate intervals for levels 1, 2 and 3 (60, 30 and 15 minutes by default). A changed description, level or validity period can trigger a new announcement. A TTS voice is required. After a playback failure, the next attempt is delayed by at least five minutes. Descriptions are spoken only when their language matches the TTS voice; the panel still shows the full provider text. Warning cards show each provider and its attribution. UltimateWX does not transmit warnings over radio or APRS-IS.

In **Configuration → Weather sources**, choose a global current-weather source or set an ordered source list separately for temperature, humidity, pressure, wind speed, wind direction, precipitation and feels-like temperature. Use the priority selectors in order from 1 onward; blank positions are skipped. The first source with a fresh reading is selected. If none in the list are available, the global source and then the newest available reading are used. Leave all priorities blank to use only the global setting. Forecast data continues to come from Open-Meteo.

APRS connects to an external Dire Wolf/KISS TCP endpoint, normally `127.0.0.1:8001`. Configure Dire Wolf to use the radio receiver and forward KISS frames to UltimateWX. The default weather-source radius is 10 km. Frames without a supported position, with invalid weather fields, outside the radius or received more than two hours ago remain visible in the monitor but are excluded from weather observations. Frames from a blocked station remain visible in the monitor. Use the button in a frame row to block or unblock its callsign; the filter applies to current and future readings from that station, and does not delete history. The parser reads direction and speed from the field following the weather-station symbol; the `s` field remains snowfall, and out-of-range values are ignored. The monitor also lets you block one WX frame. Matching frames remain visible but their readings are excluded; use the same button to unblock them. Identified readings from blocked stations and frames are excluded from current readings and statistics; stored data is not deleted.

The Audio and voice tab lists ALSA capture devices and can test the selected input by capturing a one-second sample. Configure the same input device in Dire Wolf/TNC, which decodes radio audio and forwards frames through KISS. Select the announcement output and set its software volume from 0% to 100%. Linux ALSA playback (`aplay`) and a working local Piper command are needed.

The report editor stores ordered blocks as JSON. Each block can be enabled, assigned a type and source, and given fields or text. The schedule editor supports fixed times and hourly repetition with an offset from 0 to 59 minutes after each full hour. Enable or disable each entry individually. Default times are disabled. A `forecast` block reads the four-hour Open-Meteo forecast; a `local-estimate` block is explicitly labelled as an estimate and is omitted without fresh pressure measurements.

## Sensors

All sensor paths use the same concepts: a device name, timestamp, parameter, numeric value and unit. MQTT payloads use one JSON object per message, for example:

```json
{"device":"roof-esp32","name":"Roof sensor","timestamp":"2026-10-18T10:00:00Z","sensor_altitude_m":180,"temperature_c":12.5,"humidity_percent":72,"pressure_hpa":1008.2}
```

The required `device` field is the identifier; optional `name` is a readable label shown on the Status page. If omitted, the identifier is used. Latest samples sharing an MQTT topic are grouped by device identifier.

Configure a broker such as `tcp://homeassistant.local:1883` or a TLS connection such as `mqtts://broker.example.org` (default port 8883), and a topic filter such as `ultimatewx/+/weather`. The connection check confirms that the broker accepts the connection and topic subscription. ESP32 firmware can publish this object over Wi-Fi. Use a dedicated broker account and do not expose credentials in public logs.

In **Configuration → Sensors**, each detected device has measurement checkboxes. Uncheck parameters it reports incorrectly; those readings will not feed current weather or the local pressure estimate, while saved history is kept. Mark **Prefer for announcements** to select that device first for a parameter when a fresh reading is available. Global integration priorities and block fallbacks still apply; if the preferred reading is missing or stale, normal source selection continues.

Sensor readings from MQTT, USB/serial, Home Assistant and BME280 are rejected when their timestamp is more than five minutes ahead of the application clock or a value is outside the supported range: temperature −100 to 70 °C, humidity 0–100%, pressure 300–1200 hPa, wind speed 0–120 m/s, optional wind direction 0–360° (JSON field `wind_direction_deg`; requires a direction sensor), precipitation 0–10,000 mm and sensor altitude −500–9,000 m.

In **Configuration → Sensors**, set a default altitude for MQTT or USB/serial measurements, then assign a separate name and altitude to each detected device. A blank per-device altitude inherits the source default; a sample `sensor_altitude_m` value takes precedence over both. For pressure sensors, choose whether the device reports station pressure or sea-level pressure. Automatic mode keeps the source format and does not guess for Home Assistant. BME280 inherits the station elevation by default.

Home Assistant integration uses its REST API token. Configure the base URL and a long-lived access token. The connection check validates authorized API access; sensor entities are still polled periodically. Recognized device classes are temperature, humidity, pressure, wind speed, wind direction (unit `°`) and precipitation. Supported pressure units are converted to hPa before display and any altitude correction. Sample time comes from the Home Assistant entity update timestamp, so an unchanged value is not treated as a new reading.

For USB/serial devices, select a Linux device such as `/dev/ttyUSB0`, choose the baud rate and send one JSON object followed by a newline for each sample. **Test connection** checks device access and reads its settings with `stty`; it does not verify incoming samples or their JSON format. The same field names as the MQTT example are accepted. `sensor_altitude_m` is optional; when supplied, UltimateWX stores station pressure and also derives sea-level pressure. Use the altitude of the sensor itself when it differs from the station elevation. Ready-to-upload sketches are in [`examples/arduino-bme280-serial`](../../examples/arduino-bme280-serial) and [`examples/esp32-bme280-mqtt`](../../examples/esp32-bme280-mqtt); their guides cover wiring, dependencies and UltimateWX configuration.

For a Raspberry Pi BME280, enable the direct I²C profile, set `/dev/i2c-1` and address 118 (`0x76`) or 119 (`0x77`), and connect the sensor using the board's I²C pins and correct logic voltage. The application records station pressure and separately derives sea-level pressure from the configured elevation. Confirm the physical sensor address and wiring before enabling the profile.

## FAQ and troubleshooting

**The panel does not open.** Check that the service is running and that TCP port 8080 is reachable on the local network. Use `systemctl status ultimatewx` on systemd or `rc-service ultimatewx status` on OpenRC.

**There is no APRS data.** Verify Dire Wolf's receiver audio, KISS TCP host and port, and that the TNC is delivering AX.25 UI frames. UltimateWX consumes the resulting KISS stream; the audio input test captures only a short sample and does not replace continuous decoding by Dire Wolf/TNC.

**A nearby frame is not used.** Check its position, weather fields, reception time and distance in the APRS monitor. Increase the configured radius only if that matches the station's intended coverage.

**The TTS voice is unavailable.** No language or voice is selected by default. In Voice and announcement, choose a language, load its catalogue and download a voice; the downloaded voice sets the announcement language. Internet access is required for the catalogue. The application shows only recognized licences and verifies model files before installation. The Linux installer installs Piper TTS automatically on supported Debian/Raspberry Pi OS and Ubuntu systems. On Alpine and ARMv7, install Piper separately and set its executable path in the Piper command field. Downloaded voices remain local for offline use.

**What does the offline estimate mean?** It is a simple pressure-trend indicator, not a meteorological forecast. It needs at least two recent measured station-pressure samples from the same source and station, at least one hour apart within the last five hours. When available, it appears on the Status page and can be included in an announcement as a local estimate.

**How do I update?** Use **Check for updates**, review the release information, then select **Install update and restart**. The application verifies the package against the release SHA-256 manifest. The service account must own the installed binary directory, as set by the official installer.

**Where are backups?** Back up `/var/lib/ultimatewx`, including `config.json`, the JSONL history and downloaded voices. Protect the backup because it may contain integration secrets.

\n
