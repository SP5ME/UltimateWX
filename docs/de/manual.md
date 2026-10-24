---
lang: de
title: UltimateWX Benutzerhandbuch
---

# UltimateWX Benutzerhandbuch

UltimateWX ist ein lokaler Wetteransager für Linux und Amateurfunkstationen. Die Anwendung verbindet Online-Vorhersagen, nahe APRS-Wetterpakete und lokale Sensormessungen. Daraus erstellt sie eine Ansage aus frei anordenbaren Bausteinen. Version 1 sendet nicht über Funk und steuert kein PTT.

## Installation

Der Release-Installer unterstützt Raspberry Pi OS, Ubuntu und Alpine Linux auf ARMv7, ARM64 und x86-64. Mindestversionen: Raspberry Pi OS/Debian 12, Ubuntu 24.04 LTS und Alpine Linux 3.22. Nach Veröffentlichung des Repositorys führen Sie den Befehl aus der README mit Administratorrechten aus, üblicherweise mit `sudo`. Der Installer erkennt Distribution, Architektur und Init-System, lädt das passende Archiv herunter, prüft die SHA-256-Prüfsumme, installiert den Dienst und aktiviert den Autostart. Auf Raspberry Pi OS/Debian und Ubuntu für x86-64/ARM64 installiert er Piper 1.8.0 in einer isolierten Umgebung unter `/opt/ultimatewx/piper`. Auf ARMv7 installiert er eine experimentelle Piper-Version 1.2.0 im selben Verzeichnis. Alpine erhält keine automatische Installation; eine musl-kompatible Engine kann manuell in den erweiterten Einstellungen konfiguriert werden.

Melden Sie sich beim ersten Start mit `admin` / `wx` an. Unter Konfiguration > Sicherheit können Sie das Passwort später ändern oder die Anmeldung deaktivieren. Ein Token für den ersten Start ist nicht erforderlich. Mit „Angemeldet bleiben“ bleibt die Sitzung bis zu 30 Tage bestehen. Der Stationsassistent erscheint nach der Anmeldung. Die Daten liegen in `/var/lib/ultimatewx`; sichern Sie dieses Verzeichnis, da es Passwort-Hash und Integrationsgeheimnisse enthält.

Für die Entwicklung installieren Sie Go 1.23 oder neuer und starten `go run ./cmd/ultimatewx --listen 127.0.0.1:8080`. Entwicklungsdaten liegen in `./data`.

## Sprache der Benutzeroberfläche

Das Bedienfeld enthält Englisch, Polnisch, Deutsch, Französisch, Spanisch und Italienisch. Eine weitere Sprache können Sie ohne Neubau hinzufügen: Legen Sie eine JSON-Datei im Ordner `locales` des Anwendungsdatenverzeichnisses ab (Standard: `/var/lib/ultimatewx/locales`) und laden Sie das Bedienfeld neu. Ein Beispiel und das Dateiformat finden Sie unter [`examples/locales`](../../examples/locales). Nicht übersetzte Beschriftungen werden auf Englisch angezeigt. Die Datei ändert nur die Sprache der Benutzeroberfläche; Sprachansagen verwenden weiterhin die Sprache der ausgewählten TTS-Stimme.

## Konfiguration

Melden Sie sich beim ersten Start mit `admin` / `wx` an. Unter **Konfiguration → Sicherheit** können Sie das Passwort später ändern oder die Anmeldung deaktivieren. Ohne Anmeldung kann jeder mit Zugriff auf das Panel es verwenden; nutzen Sie diesen Modus nur in einem vertrauenswürdigen lokalen Netzwerk.

Die Registerkarte **Konfiguration → Anwendung** bietet die Suche und Installation von Updates sowie den Export und Import einer Konfigurationssicherung. Die Sicherung enthält Integrationseinstellungen einschließlich MQTT-Passwörtern und Home-Assistant-Tokens, jedoch weder das Panel-Passwort noch den Messverlauf. Bewahren Sie die Sicherungsdatei sicher auf.


Die Seite **Status** zeigt konfigurierte Quellen, Audiogeräte, den Zeitpunkt der letzten Online-Wetteraktualisierung und eine Ansagevorschau. Unter **Konfiguration** stellen Sie Standort, Koordinaten, Höhe, Einheiten, Sprache, APRS-Radius, Schnittstellen, Zeitplan und Ansageblöcke ein. **Statistiken** zeigen gespeicherte Messungen, Frames und Wiedergaben. Der **APRS-Monitor** zeigt alle gespeicherten Frames oder nur WX-Frames und erklärt, ob ein WX-Frame als Wetterquelle gilt. Der Diagrammzeitraum wird separat gewählt; „Verlauf (Tage)“ steuert die Aufbewahrung von Messungen, APRS-Frames und Wiedergaben. Für APRS-Frames gilt zusätzlich eine Grenze von 10.000 Einträgen. Für automatisch eingestellte Ansageblöcke können Sie die Quellenpriorität je Messwert festlegen; die Reihenfolge bestimmt, welche verfügbaren Messungen bevorzugt werden. Im Luftfahrtmodus werden Temperatur in °C, Wind in Knoten (kt), Luftdruck in hPa und Niederschlag in mm angezeigt; das imperiale Profil verwendet °F, mph und Zoll. Für Wetterblöcke können Sie neben der Hauptquelle bis zu fünf Ersatzquellen in Reihenfolge festlegen. Für jeden Wetterblock lassen sich außerdem globale oder eigene Einheiten und ein Standard- oder kompaktes Sprachformat einstellen. Geben Sie als Zeitzone der Station einen IANA-Namen wie `Europe/Berlin` an; er gilt für Zeitplan, Ansagedatum und Gültigkeitszeiten der Warnungen.

Open-Meteo wird alle 15 Minuten abgefragt. Die kostenlose API ist für nicht kommerzielle Nutzung vorgesehen; die Daten erfordern eine Attribution nach CC BY 4.0. Das Panel weist die Quelle aus. Wenn diese Bedingungen nicht zu Ihrem Einsatz passen, deaktivieren Sie Open-Meteo und konfigurieren Sie einen anderen Anbieter, sobald ein passender Adapter verfügbar ist.

APRS verbindet sich mit einem externen Dire-Wolf/KISS-TCP-Endpunkt, normalerweise `127.0.0.1:8001`. Konfigurieren Sie Dire Wolf für den Radioempfang und die Weitergabe von KISS-Frames an UltimateWX. Der Standardradius für WX-Quellen beträgt 10 km. Frames ohne unterstützte Position, mit ungültigen Wetterfeldern, außerhalb des Radius oder älter als zwei Stunden bleiben im Monitor sichtbar, werden aber nicht als Wetterdaten verwendet. Frames einer gesperrten Station bleiben im Monitor sichtbar. Mit der Schaltfläche in einer Frame-Zeile können Sie das Rufzeichen sperren oder freigeben; der Filter gilt für aktuelle und künftige Messwerte dieser Station und löscht den Verlauf nicht. Der Parser liest Richtung und Geschwindigkeit aus dem Feld nach dem Wettersymbol; das Feld `s` bleibt Schneefall, und Werte außerhalb des Bereichs werden ignoriert. Im Monitor können Sie auch einen einzelnen WX-Frame sperren. Übereinstimmende Frames bleiben sichtbar, ihre Messwerte werden jedoch ignoriert; mit derselben Schaltfläche können Sie die Sperre aufheben. Erkannte Messwerte gesperrter Stationen und Frames werden aus aktuellen Messwerten und Statistiken ausgeblendet; gespeicherte Daten werden nicht gelöscht.

Wählen Sie unter Audio und Stimme den Ansageausgang. **1-kHz-Test starten** gibt einen kontinuierlichen Ton mit festem, mäßigem Pegel wieder, unabhängig von Ansagelautstärke und Piper. **Test stoppen** beendet die Wiedergabe. Der aktive Status bestätigt, dass ALSA gestartet wurde; prüfen Sie, ob der Ton tatsächlich aus dem ausgewählten Lautsprecher zu hören ist. Stellen Sie die Softwarelautstärke der Ansagen zwischen 0 % und 100 % ein. Der Eingangstest nimmt eine Sekunde Audio auf; Dire Wolf/TNC dekodiert weiterhin das Funksignal und leitet Frames über KISS weiter. Der Ausgabetest benötigt ALSA (`aplay`); Sprachansagen benötigen zusätzlich einen lokalen Piper-Befehl.

Der Ansageeditor speichert geordnete Bausteine als JSON. Jeder Baustein lässt sich aktivieren, einem Typ und einer Quelle zuordnen und mit Feldern oder Text versehen. Die Standardzeiten sind zunächst deaktiviert. Ein `forecast`-Baustein verwendet die Open-Meteo-Vorhersage für vier Stunden später; `local-estimate` wird ausdrücklich als Schätzung bezeichnet und ohne aktuelle Druckmessungen ausgelassen.

## Sensoren

Alle Sensorpfade verwenden dieselben Grunddaten: Gerätename, Messzeitpunkt, Parameter, Zahlenwert und Einheit. MQTT erwartet ein JSON-Objekt pro Nachricht, zum Beispiel:

```json
{"device":"roof-esp32","timestamp":"2026-10-18T10:00:00Z","temperature_c":12.5,"humidity_percent":72,"pressure_hpa":1008.2}
```

Konfigurieren Sie einen Broker wie `tcp://homeassistant.local:1883` oder eine TLS-Verbindung wie `mqtts://broker.example.org` (Standardport 8883) und einen Topic-Filter wie `ultimatewx/+/weather`. ESP32-Firmware kann dieses Objekt über WLAN veröffentlichen. Verwenden Sie ein eigenes Broker-Konto und schreiben Sie Zugangsdaten nicht in öffentliche Protokolle.

Unter **Konfiguration → Sensoren** können Sie bei jedem erkannten Gerät Messwerte abwählen, die es fehlerhaft liefert. Diese Messwerte fließen weder in das aktuelle Wetter noch in die lokale Druckschätzung ein; die Historie bleibt erhalten. Aktivieren Sie **In Ansagen bevorzugen**, damit dieses Gerät für den Messwert bei einem aktuellen Messwert zuerst gewählt wird. Globale Integrationsprioritäten und Ersatzquellen der Blöcke gelten weiterhin; fehlt ein aktueller Messwert, wird die normale Quellenauswahl verwendet.

Messwerte von MQTT, USB/seriell, Home Assistant und BME280 werden abgelehnt, wenn ihr Zeitstempel mehr als fünf Minuten in der Zukunft liegt oder ein Wert außerhalb des unterstützten Bereichs liegt: Temperatur −100 bis 70 °C, Feuchtigkeit 0–100 %, Druck 300–1200 hPa, Windgeschwindigkeit 0–120 m/s, optionale Windrichtung 0–360° (JSON-Feld `wind_direction_deg`; benötigt einen Windrichtungssensor), Niederschlag 0–10.000 mm und Sensorhöhe −500–9000 m.

Die Home-Assistant-Integration verwendet die REST-API und ein Zugriffstoken. Tragen Sie Basis-URL und Token ein. Sensorentitäten mit erkannten Geräteklassen für Temperatur, Feuchte, Druck, Windgeschwindigkeit, Windrichtung (Einheit `°`) oder Niederschlag werden regelmäßig gelesen. Unterstützte Druckeinheiten werden vor der Anzeige und einer möglichen Höhenkorrektur in hPa umgerechnet. Der Messzeitpunkt stammt aus dem Aktualisierungszeitstempel der Home-Assistant-Entität; ein unveränderter Wert gilt daher nicht als neue Messung.

Für USB/seriell wählen Sie ein Linux-Gerät wie `/dev/ttyUSB0`, legen die Baudrate fest und senden pro Messung ein JSON-Objekt mit abschließendem Zeilenumbruch. **Verbindung testen** prüft den Gerätezugriff und liest die Einstellungen mit `stty`; eingehende Messungen und deren JSON-Format werden dabei nicht geprüft. Es gelten dieselben Feldnamen wie im MQTT-Beispiel.

Für einen BME280 am Raspberry Pi aktivieren Sie das direkte I²C-Profil, tragen `/dev/i2c-1` und Adresse 118 (`0x76`) oder 119 (`0x77`) ein und verbinden den Sensor mit den I²C-Pins und der passenden Logikspannung. Die Anwendung speichert Stationsdruck und leitet den auf Meereshöhe reduzierten Druck separat aus der konfigurierten Höhe ab. Prüfen Sie Adresse und Verdrahtung, bevor Sie das Profil aktivieren.

## FAQ und Fehlerbehebung

**Das Panel öffnet sich nicht.** Prüfen Sie den Dienst und TCP-Port 8080 im lokalen Netzwerk. Bei systemd: `systemctl status ultimatewx`; bei OpenRC: `rc-service ultimatewx status`.

**Es kommen keine APRS-Daten an.** Prüfen Sie den Empfänger-Audiopfad in Dire Wolf, KISS-TCP-Host und Port sowie AX.25-UI-Frames des TNC. UltimateWX liest den resultierenden KISS-Datenstrom; der Audiotest nimmt nur eine kurze Probe auf und ersetzt nicht die kontinuierliche Dekodierung durch Dire Wolf/TNC.

**Ein naher Frame wird nicht verwendet.** Prüfen Sie Position, Wetterfelder, Empfangszeit und Entfernung im Monitor. Erhöhen Sie den Radius nur, wenn er zur beabsichtigten Stationsabdeckung passt.

**Die TTS-Stimme ist nicht verfügbar.** Wählen Sie im Bereich Stimme und Ansage eine Sprache, laden Sie den Katalog und anschließend eine Stimme herunter. Der Katalog benötigt Internet; heruntergeladene Modelle funktionieren lokal. Der Systeminstaller installiert Piper unter `/opt/ultimatewx/piper` auf Debian/Raspberry Pi OS und Ubuntu x86-64/ARM64 sowie eine experimentelle Version auf ARMv7. Falls die Installation fehlschlägt oder Alpine verwendet wird, installieren Sie eine kompatible Engine und tragen Sie den Programmpfad in den eingeklappten erweiterten Audioeinstellungen ein.

**Was bedeutet die Offline-Schätzung?** Es ist ein einfacher Drucktrend-Indikator, keine meteorologische Vorhersage. Er benötigt mindestens zwei aktuelle Stationsdruckmessungen im Abstand von mindestens einer Stunde aus den letzten fünf Stunden. Die Ansage kennzeichnet das Ergebnis als lokale Schätzung.

**Wie aktualisiere ich?** Wählen Sie **Nach Updates suchen**, prüfen Sie die Release-Informationen und dann **Update installieren und neu starten**. Die Anwendung prüft das Paket anhand des SHA-256-Manifests. Das Dienstkonto benötigt Schreibrechte im Installationsverzeichnis; der offizielle Installer richtet diese ein.

**Wo liegen Sicherungen?** Sichern Sie `/var/lib/ultimatewx` einschließlich `config.json`, JSONL-Verlauf und heruntergeladenen Stimmen. Die Sicherung kann Zugangsdaten enthalten.
