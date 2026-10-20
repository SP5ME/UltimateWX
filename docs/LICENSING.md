# Licencje i źródła zewnętrzne

## Kod

UltimateWX: GNU GPL, wersja 3 lub późniejsza (`GPL-3.0-or-later`). Backend używa na tym etapie wyłącznie biblioteki standardowej Go.

## Ikony

- Panel WWW osadza lokalnie wybrane ikony Material Design Icons (MDI) w wersji 7.4.47 z pakietu [@mdi/svg](https://github.com/Templarian/MaterialDesign-SVG/tree/v7.4.47). Ikony są objęte Apache License 2.0; jej tekst znajduje się w [`LICENSES/Apache-2.0.txt`](../LICENSES/Apache-2.0.txt).
- Ikony są używane jako dekoracyjne symbole SVG; interfejs nie pobiera ich zewnętrznie przy uruchomieniu. Nie używamy ikon z kategorii Brand / Logos.
- MDI nie jest afiliowane z UltimateWX. Więcej informacji: [Pictogrammers — licencja](https://pictogrammers.com/docs/general/license/).

## Dane i usługi

- Open-Meteo: odpowiedzi pogodowe na wolnym API podlegają CC BY 4.0, wymagają atrybucji; darmowa usługa API jest ograniczona do użytku niekomercyjnego. Sprawdź bieżące [warunki Open-Meteo](https://open-meteo.com/en/terms) przed wdrożeniem.
- IMGW-PIB: ostrzeżenia są pobierane z [publicznego API IMGW](https://danepubliczne.imgw.pl/apiinfo) (`warningsmeteo`). Wymagana atrybucja źródła danych: „Źródłem pochodzenia danych jest Instytut Meteorologii i Gospodarki Wodnej – Państwowy Instytut Badawczy”. Przed wdrożeniem należy sprawdzić aktualne warunki udostępniania danych IMGW.
- PL-WARN/CAWF: UltimateWX nie kopiuje kodu projektu PL-WARN i nie publikuje przez ten kanał. Ewentualna integracja wymaga potwierdzenia pełnej specyfikacji CAWF oraz warunków użycia przed implementacją.
- Home Assistant, MQTT, APRS i czujniki użytkownika są źródłami konfigurowanymi lokalnie.

## TTS

- Piper TTS (`piper-tts`) jest instalowany jako osobny program z oficjalnego PyPI na Raspberry Pi OS/Debian i Ubuntu dla ARM64 oraz x86-64; nie jest łączony z binarium UltimateWX. Pakiet jest licencjonowany na GPL-3.0-or-later, więc może być używany w projekcie UltimateWX na GPL-3.0-or-later. Instalator przypina wersję 1.8.0. Wymagane pakiety Pythona są instalowane w oddzielnym środowisku; licencje głosów nadal trzeba sprawdzić dla każdego modelu osobno.
- Dla Raspberry Pi OS/Debian ARMv7 przygotowano eksperymentalny, osobny pakiet Piper 1.2.0 budowany w GitHub Actions dla `linux/arm/v7`. To starsza linia projektu Piper; jej repozytorium jest zarchiwizowane. Pakiet jest instalowany wyłącznie przez instalator systemowy na ARMv7 i nie jest częścią aktualizacji aplikacji. Każde wydanie z tym binarium zawiera `PIPER-ARMV7-SOURCES.tar.gz` z kodem Piper 1.2.0, Piper-phonemize 1.1.0, przypiętym kodem eSpeak NG, snapshotami źródeł ONNX Runtime 1.14.1 i spdlog 1.11.0 oraz użytymi plikami builda i upstreamowymi informacjami licencyjnymi. eSpeak NG jest na GPL-3.0-or-later; pozostałe komponenty zachowują własne licencje i noty. Sumy binarium, paczki źródeł oraz licencji znajdują się w `PIPER-ARMV7-SHA256SUMS`. Ten wariant jest eksperymentalny i wymaga testu wydajności na Raspberry Pi 2 B.
- Automatyczna instalacja pomija Alpine Linux (musl), bo pakiet ARMv7 jest budowany dla glibc, a oficjalne wydanie PyPI nie publikuje zgodnego koła dla Alpine.
- Katalog głosów pobiera model card, pokazuje tylko rozpoznane licencje z listy dopuszczonej w kodzie i przy instalacji sprawdza sumy kontrolne modelu oraz konfiguracji.
- Licencja i atrybucja muszą być sprawdzone dla każdego głosu osobno. Odrzucone lub niejednoznaczne licencje nie są udostępniane w katalogu aplikacji.

Lista zależności oraz warunków usług musi być aktualizowana przy każdym wydaniu. Zewnętrzna usługa może zmienić warunki lub dostępność; dlatego adaptery danych pozostają wymienne.
