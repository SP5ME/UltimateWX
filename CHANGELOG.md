# Changelog

W tym pliku zapisujemy istotne zmiany dla użytkowników. Format opiera się na [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), a wersje na [Semantic Versioning](https://semver.org/).

## [Unreleased]

## [0.0.3-prealpha.1] - 2026-10-18

### Added
- Dodano test połączenia TCP z hostem i portem Dire Wolf/KISS w konfiguracji źródeł pogody; panel wyświetla stan testu, a komunikat wyjaśnia, że odbiór ramek należy sprawdzić w Monitorze APRS.

### Changed
- Ograniczono listę urządzeń audio ALSA do bezpośrednich urządzeń kart (`plughw`) i dostępnych serwerów audio, aby nie wyświetlać wielu aliasów tej samej karty. Istniejący wybór pozostaje dostępny po aktualizacji.
- Przeniesiono mapę, wyszukiwanie lokalizacji, współrzędne i wysokość do drugiej zakładki konfiguracji „Lokalizacja stacji”; pierwszy krok konfiguratora pokazuje teraz dane stacji, a drugi jej położenie.
- Usprawniono mapę: przesuwanie na bieżąco aktualizuje współrzędne pod celownikiem, widok dopasowuje się po otwarciu zakładki, a błędy pobierania kafelków są widoczne.

## [0.0.2] - 2026-10-18

### Added
- Dodano pakiet Linux ARMv7 dla 32-bitowych systemów Raspberry Pi OS i innych zgodnych dystrybucji.

### Fixed
- Ustawiono właściwy adres repozytorium instalatora i umożliwiono pobieranie najnowszego prerelease, gdy nie ma jeszcze stabilnego wydania.
- Zmieniono aktualizator panelu, aby wykrywał najnowsze opublikowane prerelease.

## [0.0.1] - 2026-10-18

### Changed
- Obniżono minimalną długość hasła pierwszego uruchomienia do 6 znaków.
- Podzielono konfigurację w panelu na zakładki tematyczne.
- Dodano zakładkę Stacja i zmienne `<call>` oraz `<loc>` w tekstowych blokach komunikatu.
- Przeniesiono lokalizację, współrzędne, jednostki i język komunikatu do zakładki Stacja pogodowa; nowe konfiguracje nie mają domyślnej lokalizacji, języka ani głosu.
- Konfigurator pierwszego uruchomienia działa przed wyborem dostępu do panelu; można ustawić hasło albo świadomie wybrać tryb bez logowania. Nie wybiera się domyślnie języka ani głosu komunikatu; pobranie głosu ustawia język komunikatu. Dodano wybór lokalizacji na mapie OpenStreetMap (kliknięcie lub przesunięcie pod celownik), przybliżanie oraz atrybucję mapy.
- Dodano graficzny edytor harmonogramu.
- Dodano cykliczny harmonogram odtwarzania co godzinę z konfigurowalną liczbą minut po pełnej godzinie.

### Added
- Początkowy szkielet aplikacji Go, lokalny panel WWW i konfiguracja hasła.
- Podstawowe adaptery Open-Meteo, MQTT, Home Assistant, USB/serial JSON i odbiór APRS KISS.
- Lokalny odczyt BME280 przez I²C w Linuksie.
- Konfigurowalne bloki komunikatu, harmonogram i lokalne odtwarzanie przez Piper/aplay.
- Zarządzanie głosami w panelu, aktualizacja z interfejsu, instalator Linux i workflow CI.
- Dokumentacja Markdown w sześciu językach.
