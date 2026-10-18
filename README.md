# UltimateWX

Lekka, otwartoźródłowa pogodynka krótkofalarska dla Linuksa. Łączy prognozę online, odbiór APRS WX i lokalne czujniki, a następnie składa konfigurowalny komunikat głosowy i odtwarza go lokalnie według harmonogramu. UltimateWX nie nadaje przez radio i nie steruje PTT.

## Status projektu

Projekt jest we wczesnym etapie implementacji. Backend i panel zawierają działające podstawowe przepływy, ale interfejs i adaptery sprzętowe wymagają testów na rzeczywistym Raspberry Pi, radiu i czujnikach. Głosy TTS są wybierane i pobierane z poziomu aplikacji; dostępne pozycje zależą od katalogu i licencji poszczególnych modeli.

## Uruchomienie developerskie

Wymagany Go 1.23 lub nowszy.

```sh
go run ./cmd/ultimatewx --listen 127.0.0.1:8080
```

Otwórz `http://127.0.0.1:8080`. Przy pierwszym uruchomieniu przejdź konfigurator stacji, a następnie ustaw hasło panelu albo wybierz pracę bez logowania. Dane developerskie trafiają do katalogu `./data`.

## Instalacja wydania

Instalator najnowszego wydania można uruchomić jednym poleceniem:

```sh
curl -fsSL https://raw.githubusercontent.com/SP5ME/UltimateWX/main/scripts/install.sh | sudo sh
```

Instalator obsługuje Raspberry Pi OS, Ubuntu i Alpine na ARM64 oraz x86-64, korzystając z systemd lub OpenRC. Szczegóły i aktualizacja z panelu: [dokumentacja instalacji](docs/pl/manual.md#instalacja).

## Dokumentacja

- [Polski](docs/pl/manual.md)
- [English](docs/en/manual.md)
- [Deutsch](docs/de/manual.md)
- [Français](docs/fr/manual.md)
- [Español](docs/es/manual.md)
- [Italiano](docs/it/manual.md)

Markdown jest źródłem dokumentacji. Pliki PDF są budowane na wydaniach po zatwierdzeniu korekty tłumaczeń. [Historia zmian](CHANGELOG.md).

## Licencja i źródła danych

Kod UltimateWX jest publikowany na licencji GPL-3.0-or-later. Darmowe API Open-Meteo jest przeznaczone do użytku niekomercyjnego i wymaga atrybucji; przed wdrożeniem w innym modelu użycia należy skonfigurować dostawcę z odpowiednimi warunkami. Głosy TTS mają osobne licencje i są pokazywane w menedżerze dopiero po rozpoznaniu dozwolonej licencji. Zobacz [wykaz licencji i źródeł](docs/LICENSING.md).
