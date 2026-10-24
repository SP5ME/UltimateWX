# UltimateWX

Lekka, otwartoźródłowa pogodynka krótkofalarska dla Linuksa. Łączy prognozę online, odbiór APRS WX i lokalne czujniki, a następnie składa konfigurowalny komunikat głosowy i odtwarza go lokalnie według harmonogramu. UltimateWX nie nadaje przez radio i nie steruje PTT.

## Status projektu

Projekt jest we wczesnym etapie implementacji. Backend i panel zawierają działające podstawowe przepływy, ale interfejs i adaptery sprzętowe wymagają testów na rzeczywistym Raspberry Pi, radiu i czujnikach. Głosy TTS są wybierane i pobierane z poziomu aplikacji; dostępne pozycje zależą od katalogu i licencji poszczególnych modeli.

## Uruchomienie developerskie

Wymagany Go 1.23 lub nowszy.

```sh
go run ./cmd/ultimatewx --listen 127.0.0.1:8080
```

Otwórz `http://127.0.0.1:8080` i zaloguj się danymi `admin` / `wx`. W konfiguracji, w sekcji Bezpieczeństwo, możesz później zmienić hasło albo wyłączyć logowanie. Token pierwszego uruchomienia nie jest wymagany. Potem możesz przejść przez konfigurator stacji. Dane developerskie trafiają do katalogu `./data`.

## Instalacja wydania

Instalator najnowszego wydania można uruchomić jednym poleceniem:

```sh
curl -fsSL https://raw.githubusercontent.com/SP5ME/UltimateWX/main/scripts/install.sh | sudo sh
```

Minimalne wspierane wersje to Raspberry Pi OS/Debian 12, Ubuntu 24.04 LTS i Alpine Linux 3.22. Instalator obsługuje te systemy na ARMv7, ARM64 oraz x86-64, korzystając z systemd lub OpenRC. Instrukcja zawiera też skrypt odinstalowujący, który domyślnie zachowuje dane. Piper TTS instaluje się w stałym katalogu `/opt/ultimatewx/piper` na wszystkich obsługiwanych systemach i architekturach. Szczegóły: [dokumentacja instalacji](docs/pl/manual.md#instalacja).

## Dokumentacja

- [Polski](docs/pl/manual.md)
- [English](docs/en/manual.md)
- [Deutsch](docs/de/manual.md)
- [Français](docs/fr/manual.md)
- [Español](docs/es/manual.md)
- [Italiano](docs/it/manual.md)

Markdown jest źródłem dokumentacji. Pliki PDF są budowane na wydaniach po zatwierdzeniu korekty tłumaczeń. [Historia zmian](CHANGELOG.md).

## Licencja i źródła danych

Kod UltimateWX jest publikowany na licencji GPL-3.0-or-later. Dane pogodowe Open-Meteo wymagają atrybucji i podlegają ograniczeniom bezpłatnego API. Ostrzeżenia są pobierane z IMGW-PIB w Polsce, DWD w Niemczech, GeoSphere Austria w Austrii oraz wielokrajowego strumienia MeteoAlarm/WIS2; panel i komunikat głosowy podają źródło danych. Warunki oraz wymagane atrybucje różnią się między dostawcami i są opisane w [wykazie licencji i źródeł](docs/LICENSING.md). Głosy TTS mają osobne licencje i są pokazywane w menedżerze dopiero po rozpoznaniu dozwolonej licencji.
