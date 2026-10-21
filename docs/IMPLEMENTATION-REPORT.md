# Sprawozdanie z wprowadzonych zmian w UltimateWX

**Data:** 2026-10-21
**Zakres:** zmiany przygotowane do pre-release `v0.0.6-prealpha.14`; publikacja i wynik CI są w toku.

## Podsumowanie

Rozbudowano UltimateWX jako lokalną pogodynkę krótkofalarską: aplikacja łączy prognozę Open-Meteo, pomiary z czujników i APRS WX, obsługuje ostrzeżenia z kilku krajów, składa komunikaty głosowe według harmonogramu i udostępnia konfigurację przez panel WWW. Dodano zabezpieczenia filtracji danych, śledzenie świeżości, obsługę różnych platform, dokumentację oraz kontrolę CI. Nie kopiowano kodu PL-WARN ani innych omawianych projektów; warunki źródeł i licencji zostały opisane w `docs/LICENSING.md`.

## Funkcje i poprawki

### Pogoda i czujniki

- Rozwinięto adapter Open-Meteo: brakujące wartości są obsługiwane osobno dla każdego parametru, odpowiedzi mają limit rozmiaru, a odległe dane godzinowe nie są przedstawiane jako prognoza za cztery godziny.
- Dodano normalizację jednostek, jakość i świeżość odczytów oraz wybór źródła i urządzenia preferowanego dla parametrów komunikatu.
- Dodano lokalny szacunek trendu ciśnienia. Jest on jawnie oznaczony jako szacunek i wymaga dostatecznie świeżych pomiarów ciśnienia stacyjnego z tego samego urządzenia.
- Rozwinięto MQTT, w tym `mqtts://`, Home Assistant i USB/serial z jednowierszowymi próbkami JSON. Odczyt serial zamyka port po anulowaniu pracy.
- Uporządkowano obsługę BME280 przez I²C, kompensację pomiarów, rozróżnienie ciśnienia stacyjnego i zredukowanego do poziomu morza oraz raportowanie błędów.
- Dodano przykłady firmware ESP32 + BME280 + MQTT oraz Arduino + BME280 + USB/serial.

### APRS WX

- Rozwinięto parser ramek WX: zakresy wartości, ujemne temperatury, czas raportu, pozycja oraz kierunek/prędkość wiatru. Odrzucone ramki pozostają widoczne w monitorze i nie zasilają pogody.
- Dodano filtr promienia od lokalizacji stacji oraz czarną listę znaków stacji i osobną blokadę pojedynczej ramki po identyfikatorze SHA-256.
- Monitor pokazuje kwalifikację i powód odrzucenia; opisy są dostępne w sześciu językach.

### Ostrzeżenia

- Połączono źródła IMGW-PIB, DWD, GeoSphere Austria oraz wielokrajowy strumień MeteoAlarm/WIS2. Adaptery dopasowują alerty do skonfigurowanej lokalizacji, a status panelu pokazuje ich stan i błędy.
- Dla MeteoAlarm/WIS2 sprawdzane są warunki ponownego użycia, HTTPS, adresy sieciowe i poligon CAP; alert bez wymaganego obszaru CAP jest pomijany.
- Ostrzeżenia rozróżniają aktywne, nadchodzące i zakończone. Zmiany i duplikaty są obsługiwane po identyfikatorze, źródle i okresie ważności.
- Komunikaty głosowe można powtarzać w różnych odstępach zależnie od stopnia zagrożenia. Aktualizacja lub zmiana stopnia może wywołać nowy komunikat; powtórne odebranie tej samej wersji nie tworzy alarmu.
- Archiwum zakończonych alertów jest ograniczone do ostatnich 30 dni również podczas długiej niedostępności źródła.

### Panel i komunikaty

- Rozbudowano panel o status usług i urządzeń, aktualne odczyty, lokalny szacunek, statystyki jakości/świeżości, historię wykresów, ostrzeżenia i monitor APRS.
- Konfigurator pierwszego uruchomienia prowadzi przez ustawienia stacji, lokalizacji, źródeł, czujników, audio oraz komunikatu. Wspiera ręczne ustawienie lokalizacji, mapę i zapis bez obowiązkowego wyszukiwania online.
- Wykresy pobierają historię dopasowaną do wybranego zakresu, ograniczają gęste serie do 1040 reprezentatywnych punktów i ignorują spóźnione odpowiedzi po zmianie zakresu.
- Etykiety pól są powiązane z kontrolkami, harmonogram ma semantyczny nagłówek, a obsługa zakładek uwzględnia klawiaturę i czytniki ekranu.
- Interfejs obsługuje sześć języków (pl, en, de, fr, es, it) i zewnętrzne pliki locale. Dodano testy pokrycia statycznych kluczy tłumaczeń.
- Status pokazuje wersję aplikacji, ułatwiając przypisanie zgłoszeń i testów do wydania.

### Bezpieczeństwo i utrzymanie

- Pierwsza konfiguracja wymaga jednorazowego tokenu z katalogu danych; dodano ochronę endpointów konfiguracji i logowania z ograniczeniem prób.
- Odpowiedzi API są oznaczone `Cache-Control: no-store`; endpointy tylko do odczytu nie wykonują operacji po POST.
- Kopia konfiguracji zachowuje hasło panelu i nie nadpisuje sekretów integracji pustymi polami podczas importu.
- Zabezpieczono aktualizację przed równoległymi instalacjami, starymi wydaniami i nieprawidłowymi sumami; po aktualizacji usługa jest kontrolowanie restartowana.
- Instalator rozpoznaje systemd/OpenRC, sprawdza minimalne wersje systemu i architekturę, weryfikuje SHA-256 i restartuje działającą usługę po aktualizacji. Dodano skrypt odinstalowania z jawną opcją usunięcia danych.
- Dodano zgodne z platformą zapisy konfiguracji i historii, w tym wariant dla lokalnego środowiska Windows.

### Dokumentacja, licencje i CI

- Zaktualizowano plan, README, changelog i instrukcje użytkownika w sześciu językach.
- Rozszerzono wykaz źródeł i licencji, w tym dane pogodowe, mapę OpenStreetMap, adaptery ostrzeżeń, MDI oraz przykłady firmware.
- CI sprawdza kompilacje amd64/arm64/armv7, testy Go, analizę statyczną, składnię JavaScript i skryptów, dokumentację, smoke testy instalatora oraz obrazy Debian/Alpine.
- Workflow wydania przygotowuje pakiety, sumy kontrolne, dokumentację i licencje. PDF-y są dołączane tylko po ludzkim zatwierdzeniu tłumaczeń.
- Przygotowano checklistę odbioru sprzętowego w `docs/HARDWARE-VALIDATION.md`.

## Weryfikacja wykonana lokalnie

- `python scripts/check-ui-locales.py` — zaliczone: 345 statycznych kluczy, sześć języków.
- `python scripts/check-docs.py` — zaliczone: kompletność struktury dokumentacji.
- `node --check` osadzonego JavaScript — zaliczone.
- `git diff --check` — zaliczone.
- Kompilacje Go dla Linux amd64, arm64 i armv7 — zaliczone. Środowisko zgłaszało brak dostępu do zapisu w cache modułów, ale kompilacja kończyła się poprawnie.
- Repozytorium zawiera dodatkowe testy regresji m.in. dla APRS, ostrzeżeń, pogody, konfiguracji, magazynu danych, BME280, audio i endpointów aplikacji. Nie uruchamiano pełnego zestawu testów po ostatnich zmianach.

## Pozostałe kroki

1. Wykonać odbiór na fizycznym Raspberry Pi według `docs/HARDWARE-VALIDATION.md`, w tym BME280/I²C, wejście i wyjście audio, Dire Wolf/KISS, harmonogram oraz zachowanie źródeł. Użytkownik potwierdził, że może wykonać te testy; ich wynik nie został jeszcze przekazany.
2. Tłumaczenia en, pl, de, fr, es i it mają w `docs/translation-status.json` stan `needs-review`. Użytkownik wskazał, że ludzka korekta nie jest możliwa na tym etapie. Zgodnie z workflow można przygotować pre-release bez PDF-ów; pełne wydanie PDF wymaga późniejszego przeglądu.
3. Poczekać na pełne testy CI uruchamiane dla taga pre-release i sprawdzić wynik budowania pakietów.

## Najważniejsze pliki

- Panel i endpointy: `internal/app/index.html`, `internal/app/app.go`
- Pogoda i APRS: `internal/weather/`, `internal/aprs/`
- Ostrzeżenia: `internal/warnings/`
- Czujniki, konfiguracja i historia: `internal/bme280/`, `internal/mqtt/`, `internal/config/`, `internal/store/`
- Instalacja i CI: `scripts/install.sh`, `scripts/uninstall.sh`, `.github/workflows/ci.yml`
- Dokumentacja: `PLAN.md`, `CHANGELOG.md`, `docs/LICENSING.md`, `docs/HARDWARE-VALIDATION.md`
