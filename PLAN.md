# UltimateWX — plan projektu

## Cel

UltimateWX to lekka, konfigurowalna pogodynka krótkofalarska dla małych komputerów z Linuksem, w tym Raspberry Pi. Aplikacja pobiera i łączy dane pogodowe, buduje komunikat z wybieranych sekcji, odtwarza go lokalnie według harmonogramu i udostępnia czytelny panel WWW. Pierwsze wydanie nie steruje nadawaniem ani PTT.

## Założenia platformowe

- Obsługiwane środowiska: Raspberry Pi OS/Raspbian, Ubuntu i Alpine Linux.
- Minimalne wspierane wersje instalatora: Raspberry Pi OS/Debian 12, Ubuntu 24.04 LTS i Alpine Linux 3.22; CI sprawdza także Debian 13 i Alpine 3.24.
- Wspierane architektury Linux: ARMv7, ARM64 i x86-64.
- Usługa ma korzystać z systemu init dostępnego na danej dystrybucji, w tym systemd lub OpenRC.
- Backend wybrany i zaimplementowany w Go. Panel HTML/CSS/JavaScript jest osadzony w binarium Go i udostępniany lokalnie przez ten sam proces.

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
- Ostrzeżenia pogodowe: pobieranie aktywnych ostrzeżeń IMGW lub ich dystrybucja przez APRS-IS PL-WARN/CAWF. Ostrzeżenie musi zawierać źródło, rodzaj zagrożenia, stopień, obszar oraz czas obowiązywania; aplikacja uwzględnia je tylko wtedy, gdy dotyczą skonfigurowanej lokalizacji stacji.
- Aplikacja automatycznie odpytuje adaptery, których zasięg obejmuje zapisane współrzędne stacji; interfejs pokazuje ich status, ale nie wymaga wyboru kraju ani dostawcy. Dopasowanie obszaru wykonuje adapter na podstawie oficjalnych danych dostawcy. Przed wdrożeniem kolejnego adaptera trzeba potwierdzić stabilność jego API, dokładność dopasowania lokalizacji i warunki ponownego użycia danych.
- Wdrożono wielokrajowy strumień [MeteoAlarm/WIS2](https://api.meteoalarm.org/mqtt) przez publiczny broker [WMO](https://community.wmo.int/site/knowledge-hub/programmes-and-initiatives/wmo-information-system-wis/wis-20-global-services). Wymaga jawnych praw i atrybucji z powiadomienia, pobiera GeoJSON i CAP wyłącznie przez HTTPS, sprawdza adresy sieciowe, a alert dopasowuje do stacji wyłącznie wtedy, gdy zawiera ją poligon obszaru CAP. Geometria GeoJSON służy tylko do wczesnego odrzucenia alertu; alerty bez poligonu CAP są pomijane. Ostrzeżenia z inną licencją CAP są pomijane. MQTT jest strumieniem bieżącym, bez odtwarzania wiadomości pominiętych podczas przestoju; poprzednie aktywne alerty pozostają zapisane do wygaśnięcia. Broker i temat są zdefiniowane jako domyślne, a lista brokerów zapasowych może być zmieniana w adapterze.
- Ostrzeżenia są osobnym typem danych od pomiarów i prognozy. Panel pokazuje ostrzeżenia aktywne, nadchodzące i zakończone wraz z obszarem, stopniem, źródłem i okresem obowiązywania.
- Dane z różnych źródeł są normalizowane do wspólnego modelu pojęć meteorologicznych zgodnego z terminologią i jednostkami WMO; użytkownik wybiera parametry, jednostki i sposób prezentacji. Nie oznacza to, że V1 musi eksportować pliki WMO BUFR/GRIB.
- Jednostki i formaty prezentacji są konfigurowalne; konwersje wykonuje aplikacja.
- Każdy parametr, w tym kierunek wiatru, może mieć wybrane źródło oraz kolejność źródeł zapasowych. Dane powinny zawierać źródło, czas pomiaru/aktualizacji, jakość i informację o świeżości.
- APRS WX: monitor pokazuje ramki odebrane; do pogody kwalifikują się tylko poprawne, aktualne ramki WX z pozycją, mieszczące się domyślnie w promieniu 10 km. Promień jest konfigurowalny. Ramki poza zasięgiem lub bez pozycji nie zasilają pogody. Nie ma nadawania APRS w pierwszym wydaniu.
  Parser WX sprawdza zakresy temperatury, wilgotności, ciśnienia i wiatru oraz dekoduje kierunek i prędkość z pola pozycji; pole śniegu `s` nie jest traktowane jako prędkość wiatru.
- Czarna lista APRS jest dostępna bezpośrednio w monitorze na dwóch poziomach: znak nadawcy blokuje całą stację, a identyfikator SHA-256 blokuje pojedynczą treść ramki WX. Ramki pozostają widoczne; pomiary objęte blokadą nie są używane w bieżącej pogodzie.
- Odbiór APRS przez Dire Wolf lub KISS TNC; konfiguracja portu/urządzenia i osobnego wejścia audio.
- Ciśnienie BME280: przechowywać rozróżnienie między ciśnieniem stacyjnym a zredukowanym do poziomu morza, z uwzględnieniem wysokości i bez ponownej korekty danych już zredukowanych. Wysokość instalacji wpływa na interpretację ciśnienia.
- Gdy Internet jest niedostępny, lokalna prognoza może być jedynie szacunkiem na podstawie historii i lokalnych pomiarów. Komunikat musi wyraźnie oznaczać ją jako lokalny szacunek/prognozę, podawać jej podstawę i pomijać ją, jeśli dane są stare lub niewystarczające.

## Czujniki i integracje

- MQTT do lokalnych czujników i Home Assistant oraz natywna integracja z API Home Assistant.
- USB/serial: wspólny, udokumentowany format jednej linii JSON na próbkę.
- Raspberry Pi: bezpośredni BME280 przez I²C/GPIO jako gotowy profil.
- Materiały dla użytkowników: ogólna instrukcja połączeń i konfiguracji dla wielu płytek, przykładowy firmware ESP32 + BME280 przez Wi-Fi/MQTT oraz Arduino + BME280 przez USB/serial. Opis ma ułatwić uruchomienie bez samodzielnego projektowania protokołu.
- Dla każdego urządzenia można przypisać nazwę i wysokość (z dziedziczeniem albo korektą), wybrać używane parametry oraz oznaczyć preferowane urządzenie osobno dla każdego parametru komunikatu. Źródło integracji jest przypisywane automatycznie; globalne priorytety źródeł i fallbacki nadal rozstrzygają kolejność między integracjami.

## Komunikaty, harmonogram i głos

- Komunikat składa się z uporządkowanych, włączanych/wyłączanych bloczków, np. powitanie, data i godzina, aktualna pogoda, prognoza za 4 godziny.
- Aktywne ostrzeżenia mogą być włączone do komunikatu głosowego i powtarzane częściej niż zwykły komunikat pogodowy. Częstotliwość powtórzeń konfiguruje się osobno i może zależeć od stopnia zagrożenia. Nowe ostrzeżenie lub zmiana jego stopnia może uruchomić dodatkowy komunikat; zakończone i niezmienione ostrzeżenia nie powinny być ogłaszane bez końca.
- Aplikacja rozpoznaje aktualizacje i duplikaty ostrzeżeń po ich identyfikatorze oraz okresie obowiązywania, aby zmiana komunikatu mogła zostać ogłoszona, a ponowne odebranie tej samej wersji nie tworzyło kolejnego alarmu.
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

## Decyzje techniczne i pozostałe punkty przed wydaniem

### Rozstrzygnięte w implementacji

- Backend to Go, a panel jest osadzony w binarium; nie wymaga osobnego serwera frontendowego ani kroku budowania zasobów WWW.
- Silnikiem TTS jest Piper. Katalog głosów jest pobierany dynamicznie, a aplikacja weryfikuje rozpoznaną licencję, sumy pobranych plików i pokazuje odnośnik do karty modelu. Każdy głos nadal wymaga sprawdzenia warunków jego własnej karty przed publikacją/rekomendacją.
- Lokalny szacunek nie jest prognozą meteorologiczną. Używa wyłącznie zmierzonych ciśnień stacyjnych z tego samego źródła i identyfikatora urządzenia/stacji; wymaga co najmniej dwóch próbek oddzielonych o godzinę, każda młodsza niż 5 godzin. Wynik opisuje zmianę ciśnienia przeliczoną na 3 godziny i jest pomijany przy niewystarczających danych.
- Retencja historii jest konfigurowalna od 1 do 365 dni (domyślnie 7); monitor APRS przechowuje najwyżej 10 000 ramek. Archiwum zakończonych ostrzeżeń przechowuje do 500 wpisów przez 30 dni.
- Kopia konfiguracji ma format JSON w wersji 1. Nie zawiera hasła panelu ani historii; zawiera konfigurację integracji, w tym sekrety MQTT i Home Assistant. Import zachowuje aktualne dane logowania panelu i nie nadpisuje tych sekretów pustymi polami.

### Do rozstrzygnięcia lub potwierdzenia przed wydaniem

- Minimalne wersje są określone i sprawdzane przez instalator: Raspberry Pi OS/Debian 12, Ubuntu 24.04 LTS i Alpine 3.22. CI uruchamia smoke testy na Ubuntu 24.04, Alpine 3.22 i 3.24 oraz Debian 12 i 13. Testy kontenerowe nie zastępują instalacji na fizycznym Raspberry Pi OS.
- Testy rzeczywistego GPIO/I²C, wejścia i wyjścia audio oraz odbioru radiowego wymagają Raspberry Pi i podłączonego sprzętu. Nie są potwierdzone przez kompilację ani kontenery CI; przebieg i wynik należy zapisać według checklisty [odbioru sprzętowego](docs/HARDWARE-VALIDATION.md).
- Wydanie PDF-ów wymaga jawnego zatwierdzenia ludzkiego tłumaczeń. Skrypt wydania pomija PDF-y, gdy tłumaczenia nie mają takiego zatwierdzenia.
