---
lang: de
title: UltimateWX Benutzerhandbuch
---

# UltimateWX Benutzerhandbuch

UltimateWX ist ein lokaler Wetteransager für Linux und Amateurfunkstationen. Die Anwendung verbindet Online-Vorhersagen, nahe APRS-Wetterpakete und lokale Sensormessungen. Daraus erstellt sie eine Ansage aus frei anordenbaren Bausteinen. Version 1 sendet nicht über Funk und steuert kein PTT.

## Installation

Der Release-Installer unterstützt Raspberry Pi OS, Ubuntu und Alpine Linux auf ARM64 und x86-64. Nach Veröffentlichung des Repositorys führen Sie den Befehl aus der README mit Administratorrechten aus, üblicherweise mit `sudo`. Der Installer erkennt Distribution, Architektur und Init-System, lädt das passende Archiv herunter, prüft die SHA-256-Prüfsumme, installiert den Dienst und aktiviert den Autostart. Der Repository-Eigentümer im Befehl muss mit dem GitHub-Projekt übereinstimmen.

Öffnen Sie im lokalen Netzwerk `http://<geraete-ip>:8080`. Führen Sie beim ersten Start zuerst den Einrichtungsassistenten aus. Danach können Sie ein Passwort mit mindestens sechs Zeichen festlegen oder ohne Anmeldung fortfahren. Konfiguration und Verlauf liegen in `/var/lib/ultimatewx`. Sichern Sie dieses Verzeichnis; es enthält den Passwort-Hash und Zugangsdaten für Integrationen.

Für die Entwicklung installieren Sie Go 1.23 oder neuer und starten `go run ./cmd/ultimatewx --listen 127.0.0.1:8080`. Entwicklungsdaten liegen in `./data`.

## Konfiguration

Beim ersten Start erscheint der Einrichtungsassistent vor der Festlegung des Panelzugriffs. Danach können Sie ein Passwort festlegen oder ohne Anmeldung fortfahren. Ohne Passwort kann jeder mit Zugriff auf das Panel es verwenden; nutzen Sie diesen Modus nur in einem vertrauenswürdigen lokalen Netzwerk.

Die Seite **Status** zeigt konfigurierte Quellen, Audiogeräte, den Zeitpunkt der letzten Online-Wetteraktualisierung und eine Ansagevorschau. Unter **Konfiguration** stellen Sie Standort, Koordinaten, Höhe, Einheiten, Sprache, APRS-Radius, Schnittstellen, Zeitplan und Ansageblöcke ein. **Statistiken** zeigen gespeicherte Messungen, Frames und Wiedergaben. Der **APRS-Monitor** zeigt alle gespeicherten Frames oder nur WX-Frames und erklärt, ob ein WX-Frame als Wetterquelle gilt.

Open-Meteo wird alle 15 Minuten abgefragt. Die kostenlose API ist für nicht kommerzielle Nutzung vorgesehen; die Daten erfordern eine Attribution nach CC BY 4.0. Das Panel weist die Quelle aus. Wenn diese Bedingungen nicht zu Ihrem Einsatz passen, deaktivieren Sie Open-Meteo und konfigurieren Sie einen anderen Anbieter, sobald ein passender Adapter verfügbar ist.

APRS verbindet sich mit einem externen Dire-Wolf/KISS-TCP-Endpunkt, normalerweise `127.0.0.1:8001`. Konfigurieren Sie Dire Wolf für den Radioempfang und die Weitergabe von KISS-Frames an UltimateWX. Der Standardradius für WX-Quellen beträgt 10 km. Frames ohne unterstützte Position, mit ungültigen Wetterfeldern, außerhalb des Radius oder älter als zwei Stunden bleiben im Monitor sichtbar, werden aber nicht als Wetterdaten verwendet.

Audioeingang und -ausgang werden getrennt gewählt. Der Eingang gehört zum Empfängerpfad in Dire Wolf; der Ausgang wird von `aplay` für die Ansage verwendet. Zur Geräteerkennung und Wiedergabe werden ALSA-Werkzeuge (`arecord`, `aplay`) und ein lokaler Piper-Befehl benötigt. Die Auswahl des Eingangs konfiguriert Dire Wolf nicht automatisch.

Der Ansageeditor speichert geordnete Bausteine als JSON. Jeder Baustein lässt sich aktivieren, einem Typ und einer Quelle zuordnen und mit Feldern oder Text versehen. Die Standardzeiten sind zunächst deaktiviert. Ein `forecast`-Baustein verwendet die Open-Meteo-Vorhersage für vier Stunden später; `local-estimate` wird ausdrücklich als Schätzung bezeichnet und ohne aktuelle Druckmessungen ausgelassen.

## Sensoren

Alle Sensorpfade verwenden dieselben Grunddaten: Gerätename, Messzeitpunkt, Parameter, Zahlenwert und Einheit. MQTT erwartet ein JSON-Objekt pro Nachricht, zum Beispiel:

```json
{"device":"roof-esp32","timestamp":"2026-10-18T10:00:00Z","temperature_c":12.5,"humidity_percent":72,"pressure_hpa":1008.2}
```

Konfigurieren Sie einen Broker wie `tcp://homeassistant.local:1883` und einen Topic-Filter wie `ultimatewx/+/weather`. ESP32-Firmware kann dieses Objekt über WLAN veröffentlichen. Verwenden Sie ein eigenes Broker-Konto und schreiben Sie Zugangsdaten nicht in öffentliche Protokolle.

Die Home-Assistant-Integration verwendet die REST-API und ein Zugriffstoken. Tragen Sie Basis-URL und Token ein. Sensorentitäten mit erkannten Geräteklassen für Temperatur, Feuchte, Druck, Windgeschwindigkeit oder Niederschlag werden regelmäßig gelesen.

Für USB/seriell wählen Sie ein Linux-Gerät wie `/dev/ttyUSB0`, legen die Baudrate fest und senden pro Messung ein JSON-Objekt mit abschließendem Zeilenumbruch. Es gelten dieselben Feldnamen wie im MQTT-Beispiel.

Für einen BME280 am Raspberry Pi aktivieren Sie das direkte I²C-Profil, tragen `/dev/i2c-1` und Adresse 118 (`0x76`) oder 119 (`0x77`) ein und verbinden den Sensor mit den I²C-Pins und der passenden Logikspannung. Die Anwendung speichert Stationsdruck und leitet den auf Meereshöhe reduzierten Druck separat aus der konfigurierten Höhe ab. Prüfen Sie Adresse und Verdrahtung, bevor Sie das Profil aktivieren.

## FAQ und Fehlerbehebung

**Das Panel öffnet sich nicht.** Prüfen Sie den Dienst und TCP-Port 8080 im lokalen Netzwerk. Bei systemd: `systemctl status ultimatewx`; bei OpenRC: `rc-service ultimatewx status`.

**Es kommen keine APRS-Daten an.** Prüfen Sie den Empfänger-Audiopfad in Dire Wolf, KISS-TCP-Host und Port sowie AX.25-UI-Frames des TNC. Die ausgewählte Audioeingabe dient derzeit als Hinweis für die Empfängerkonfiguration; UltimateWX liest den resultierenden KISS-Datenstrom.

**Ein naher Frame wird nicht verwendet.** Prüfen Sie Position, Wetterfelder, Empfangszeit und Entfernung im Monitor. Erhöhen Sie den Radius nur, wenn er zur beabsichtigten Stationsabdeckung passt.

**Die TTS-Stimme ist nicht verfügbar.** Wählen Sie im Stimmenbereich eine Sprache und laden Sie bei Internetzugang den Katalog. Die Anwendung zeigt nur erkannte Lizenzen an und prüft die Modelldateien. Piper selbst muss installiert und als konfigurierter Befehl verfügbar sein; heruntergeladene Stimmen bleiben lokal.

**Was bedeutet die Offline-Schätzung?** Es ist ein einfacher Drucktrend-Indikator, keine meteorologische Vorhersage. Er benötigt mindestens zwei aktuelle Stationsdruckmessungen im Abstand von mindestens einer Stunde aus den letzten fünf Stunden. Die Ansage kennzeichnet das Ergebnis als lokale Schätzung.

**Wie aktualisiere ich?** Wählen Sie **Nach Updates suchen**, prüfen Sie die Release-Informationen und dann **Update installieren und neu starten**. Die Anwendung prüft das Paket anhand des SHA-256-Manifests. Das Dienstkonto benötigt Schreibrechte im Installationsverzeichnis; der offizielle Installer richtet diese ein.

**Wo liegen Sicherungen?** Sichern Sie `/var/lib/ultimatewx` einschließlich `config.json`, JSONL-Verlauf und heruntergeladenen Stimmen. Die Sicherung kann Zugangsdaten enthalten.
