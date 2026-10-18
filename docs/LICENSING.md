# Licencje i źródła zewnętrzne

## Kod

UltimateWX: GNU GPL, wersja 3 lub późniejsza (`GPL-3.0-or-later`). Backend używa na tym etapie wyłącznie biblioteki standardowej Go.

## Dane i usługi

- Open-Meteo: odpowiedzi pogodowe na wolnym API podlegają CC BY 4.0, wymagają atrybucji; darmowa usługa API jest ograniczona do użytku niekomercyjnego. Sprawdź bieżące [warunki Open-Meteo](https://open-meteo.com/en/terms) przed wdrożeniem.
- Home Assistant, MQTT, APRS i czujniki użytkownika są źródłami konfigurowanymi lokalnie.

## TTS

- Silnik Piper jest uruchamiany jako osobny program i nie jest dołączany do binariów UltimateWX.
- Katalog głosów pobiera model card, pokazuje tylko rozpoznane licencje z listy dopuszczonej w kodzie i przy instalacji sprawdza sumy kontrolne modelu oraz konfiguracji.
- Licencja i atrybucja muszą być sprawdzone dla każdego głosu osobno. Odrzucone lub niejednoznaczne licencje nie są udostępniane w katalogu aplikacji.

Lista zależności oraz warunków usług musi być aktualizowana przy każdym wydaniu. Zewnętrzna usługa może zmienić warunki lub dostępność; dlatego adaptery danych pozostają wymienne.
