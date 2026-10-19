# Licencje i źródła zewnętrzne

## Kod

UltimateWX: GNU GPL, wersja 3 lub późniejsza (`GPL-3.0-or-later`). Backend używa na tym etapie wyłącznie biblioteki standardowej Go.

## Ikony

- Panel WWW osadza lokalnie wybrane ikony Material Design Icons (MDI) w wersji 7.4.47 z pakietu [@mdi/svg](https://github.com/Templarian/MaterialDesign-SVG/tree/v7.4.47). Ikony są objęte Apache License 2.0; jej tekst znajduje się w [`LICENSES/Apache-2.0.txt`](../LICENSES/Apache-2.0.txt).
- Ikony są używane jako dekoracyjne symbole SVG; interfejs nie pobiera ich zewnętrznie przy uruchomieniu. Nie używamy ikon z kategorii Brand / Logos.
- MDI nie jest afiliowane z UltimateWX. Więcej informacji: [Pictogrammers — licencja](https://pictogrammers.com/docs/general/license/).

## Dane i usługi

- Open-Meteo: odpowiedzi pogodowe na wolnym API podlegają CC BY 4.0, wymagają atrybucji; darmowa usługa API jest ograniczona do użytku niekomercyjnego. Sprawdź bieżące [warunki Open-Meteo](https://open-meteo.com/en/terms) przed wdrożeniem.
- Home Assistant, MQTT, APRS i czujniki użytkownika są źródłami konfigurowanymi lokalnie.

## TTS

- Piper TTS (`piper-tts`) jest instalowany jako osobny program z oficjalnego PyPI na Raspberry Pi OS/Debian i Ubuntu dla ARM64 oraz x86-64; nie jest łączony z binarium UltimateWX. Pakiet jest licencjonowany na GPL-3.0-or-later, więc może być używany w projekcie UltimateWX na GPL-3.0-or-later. Instalator przypina wersję 1.8.0. Wymagane pakiety Pythona są instalowane w oddzielnym środowisku; licencje głosów nadal trzeba sprawdzić dla każdego modelu osobno.
- Automatyczna instalacja pomija Alpine Linux (musl) i ARMv7, ponieważ oficjalne wydanie PyPI nie publikuje dla nich zgodnych kół binarnych (wheels). W tych systemach Piper pozostaje opcjonalny i wymaga osobnej instalacji.
- Katalog głosów pobiera model card, pokazuje tylko rozpoznane licencje z listy dopuszczonej w kodzie i przy instalacji sprawdza sumy kontrolne modelu oraz konfiguracji.
- Licencja i atrybucja muszą być sprawdzone dla każdego głosu osobno. Odrzucone lub niejednoznaczne licencje nie są udostępniane w katalogu aplikacji.

Lista zależności oraz warunków usług musi być aktualizowana przy każdym wydaniu. Zewnętrzna usługa może zmienić warunki lub dostępność; dlatego adaptery danych pozostają wymienne.
