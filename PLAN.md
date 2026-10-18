# UltimateWX — plan projektu

## Cel

UltimateWX to lekka, konfigurowalna pogodynka krótkofalarska dla małych komputerów z Linuksem, w tym Raspberry Pi. Aplikacja pobiera i łączy dane pogodowe, buduje komunikat z wybieranych sekcji, odtwarza go lokalnie według harmonogramu i udostępnia czytelny panel WWW. Pierwsze wydanie nie steruje nadawaniem ani PTT.

## Założenia platformowe

- Obsługiwane środowiska: Raspberry Pi OS/Raspbian, Ubuntu i Alpine Linux.
- Wspierane architektury Linux: ARMv7, ARM64 i x86-64.
- Usługa ma korzystać z systemu init dostępnego na danej dystrybucji, w tym systemd lub OpenRC.
- Kierunek technologiczny backendu: lekki, modułowy język kompilowany do łatwego wdrożenia na Linuksie (Go jest kandydatem do potwierdzenia na etapie projektu technicznego). Panel WWW powinien być prosty i działać lokalnie.

## Panel i konfiguracja

Panel WWW ma być prosty, prowadzić użytkownika przez konfigurację i rozdzielać funkcje na zakładki:

- **Status** — stan aplikacji, harmonogramu, TTS, urządzeń audio, APRS, źródeł pogody, czujników, MQTT i Home Assistant; ostatnie błędy i czas aktualizacji danych.
- **Konfiguracja** — lokalizacja i wysokość, źródła danych, urządzenia, komunikaty, język, jednostki, harmonogram, retencja danych i ustawienia sieciowe.
- **Statystyki** — historia odczytów pogodowych, jakość/świeżość danych, odebrane ramki i wykonane odtworzenia; zakres i czas przechowywania konfigurowalne.
- **Monitor APRS** — podgląd odebranych ramek, filtr widoku (wszystkie / tylko WX) oraz informacja, czy ramka kwalifikuje się jako źródło pogody.
- **Czujniki i źródła** — lista urządzeń, ich stan, ostatnie dane i możliwość sprawdzenia połączenia.

Wybór urządzeń audio ma być jawny i niezależny dla każdej funkcji: osobne wejście dla APRS RX i osobne wyjście dla komunikatu TTS. Panel ma pokazywać wykryte urządzenia z czytelnymi nazwami, umożliwić próbę/test i zgłosić, gdy skonfigurowane urządzenie zniknie. Dostępne możliwości zależą od systemu audio w danej dystrybucji.

Panel w sieci lokalnej zabezpiecza hasło ustawiane podczas pierwszego uruchomienia. Pierwsza konfiguracja powinna podpowiadać lokalizację i wysokość, lecz pozwalać na ręczną korektę i pracę offline.

## Dane pogodowe i normalizacja

- Źródła pierwszej wersji: Open-Meteo dla prognozy online, APRS WX oraz czujniki lokalne/zdalne.
- Dane z różnych źródeł są normalizowane do wspólnego modelu pojęć meteorologicznych zgodnego z terminologią i jednostkami WMO; użytkownik wybiera parametry, jednostki i sposób prezentacji. Nie oznacza to, że V1 musi eksportować pliki WMO BUFR/GRIB.
- Jednostki i formaty prezentacji są konfigurowalne; konwersje wykonuje aplikacja.
- Każdy parametr może mieć wybrane źródło oraz kolejność źródeł zapasowych. Dane powinny zawierać źródło, czas pomiaru/aktualizacji, jakość i informację o świeżości.
- APRS WX: monitor pokazuje ramki odebrane; do pogody kwalifikują się tylko poprawne, aktualne ramki WX z pozycją, mieszczące się domyślnie w promieniu 10 km. Promień jest konfigurowalny. Ramki poza zasięgiem lub bez pozycji nie zasilają pogody. Nie ma nadawania APRS w pierwszym wydaniu.
- Odbiór APRS przez Dire Wolf lub KISS TNC; konfiguracja portu/urządzenia i osobnego wejścia audio.
- Ciśnienie BME280: przechowywać rozróżnienie między ciśnieniem stacyjnym a zredukowanym do poziomu morza, z uwzględnieniem wysokości i bez ponownej korekty danych już zredukowanych. Wysokość instalacji wpływa na interpretację ciśnienia.
- Gdy Internet jest niedostępny, lokalna prognoza może być jedynie szacunkiem na podstawie historii i lokalnych pomiarów. Komunikat musi wyraźnie oznaczać ją jako lokalny szacunek/prognozę, podawać jej podstawę i pomijać ją, jeśli dane są stare lub niewystarczające.

## Czujniki i integracje

- MQTT do lokalnych czujników i Home Assistant oraz natywna integracja z API Home Assistant.
- USB/serial: wspólny, udokumentowany format jednej linii JSON na próbkę.
- Raspberry Pi: bezpośredni BME280 przez I²C/GPIO jako gotowy profil.
- Materiały dla użytkowników: ogólna instrukcja połączeń i konfiguracji dla wielu płytek, przykładowy firmware ESP32 + BME280 przez Wi-Fi/MQTT oraz Arduino + BME280 przez USB/serial. Opis ma ułatwić uruchomienie bez samodzielnego projektowania protokołu.
- Dla każdego urządzenia możliwość przypisania nazwy, lokalizacji/wysokości (z dziedziczeniem albo korektą), parametrów i źródeł.

## Komunikaty, harmonogram i głos

- Komunikat składa się z uporządkowanych, włączanych/wyłączanych bloczków, np. powitanie, data i godzina, aktualna pogoda, prognoza za 4 godziny.
- W każdym bloku można wybrać podawane parametry, źródło, kolejność/fallback, jednostki i format wypowiedzi.
- Komunikat jest generowany i odtwarzany lokalnie według harmonogramu; bez funkcji nadawania radiowego w V1.
- UI od początku przygotowany na pliki tłumaczeń; języki startowe: en, pl, de, fr, es, it. Brakujące tłumaczenie ma mieć czytelny fallback. Społeczność powinna móc dodawać języki przez pliki.
- Język interfejsu jest niezależny od języka TTS.
- Głosy i silniki TTS pobierane na żądanie podczas konfiguracji, a nie dołączane wszystkie do instalacji. Po pobraniu mają działać lokalnie/offline. Przed publikacją trzeba weryfikować licencję silnika i każdego modelu/głosu osobno.

## Instalacja, aktualizacja i autostart

- Docelowa instalacja jednym poleceniem z oficjalnego repozytorium, np. `curl -fsSL <adres-instalatora> | sh`.
- Instalator wykrywa dystrybucję, architekturę i system init, dobiera zgodny artefakt, sprawdza jego sumę kontrolną, instaluje usługę i włącza autostart. Powinien jasno raportować brak wsparcia lub wymagane zależności.
- Publikowane wydania zawierają artefakty dla wspieranych architektur, sumy kontrolne, instrukcję aktualizacji i odinstalowania oraz wykaz licencji.
- Instalacja i podstawowa konfiguracja powinny działać bez konieczności ręcznego dobierania pakietu przez użytkownika.

## GitHub Actions / CI

- Workflow uruchamiane dla pull requestów, zmian w głównej gałęzi i tagów wydań.
- Macierz kompilacji i testów dla wspieranych architektur (x86-64, ARM64 i ARMv7) oraz sprawdzenie instalacji w środowiskach Ubuntu, Alpine i Raspberry Pi OS w zakresie możliwym w CI.
- GitHub-hosted Ubuntu runner obsługuje natywnie x86-64 i ARM64 oraz kompiluje ARMv7 krzyżowo; Alpine/Raspberry Pi OS należy sprawdzać w odpowiednich obrazach/kontenerach. Rzeczywiste GPIO, wejście/wyjście audio i radio wymagają dodatkowego testu sprzętowego na urządzeniu.
- Przy wydaniu CI buduje pakiety, sumy kontrolne i wykaz zależności/licencji.

## Licencje i publikacja

- Zależności aplikacji, źródła danych, silniki i głosy TTS muszą być bezpłatne i mieć licencje zgodne z publikacją projektu; samo „bezpłatne użycie” nie wystarcza.
- Prowadzić wykaz zależności, źródeł, licencji, wymaganych atrybucji i ograniczeń. Weryfikować warunki API i licencje modeli przed włączeniem adaptera/zasobu do wydania.
- Integracje zewnętrzne powinny być wymienne przez adaptery, żeby można było zastąpić źródło, którego warunki ulegną zmianie.

## Proponowana kolejność prac

1. Potwierdzić architekturę techniczną, granice modułów i minimalne wspierane dystrybucje/architektury.
2. Zbudować szkielet usługi, panelu, konfiguracji, statusu i instalacji developerskiej.
3. Dodać model danych pogodowych, jednostki, Open-Meteo oraz podstawowe bloczki komunikatu i lokalne odtwarzanie TTS.
4. Dodać czujniki: MQTT/HA, USB JSON i Raspberry Pi BME280; przygotować przykłady ESP32/Arduino.
5. Dodać odbiór APRS, monitor ramek i filtry kwalifikacji WX.
6. Dodać harmonogram, statystyki, szacowanie offline z jawnym oznaczeniem niepewności, tłumaczenia i pobieranie głosów.
7. Uzupełnić macierz CI, instalator dla dystrybucji, wydania, dokumentację użytkownika i kontrolę licencji.

## Otwarte decyzje przed implementacją

- Ostateczny wybór języka/backendu i technologii panelu (Go jest kandydatem, nie zatwierdzonym wyborem).
- Konkretne wersje dystrybucji i architektury objęte oficjalnym wsparciem.
- Dokładny system TTS i katalog głosów z licencjami zatwierdzonymi dla każdej pozycji.
- Zakres i metoda lokalnego szacowania prognozy oraz minimalna ilość/świeżość danych.
- Retencja statystyk i konfiguracja kopii zapasowej.
