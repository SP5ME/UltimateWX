# Changelog

W tym pliku zapisujemy istotne zmiany dla użytkowników. Format opiera się na [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), a wersje na [Semantic Versioning](https://semver.org/).

## [Unreleased]

## [0.0.6-prealpha.15] - 2026-10-21

### Fixed
- Workflow budowania Piper ponawia pobranie ONNX Runtime po przejściowych błędach sieciowych.
- Smoke-test Alpine przygotowuje katalog `/etc/init.d`, który w pełnej instalacji dostarcza OpenRC.
- Instalator na Alpine jawnie tworzy grupę systemową `ultimatewx`, wymaganą do ustawienia właściciela plików aplikacji.
- Archiwum zakończonych ostrzeżeń ukrywa wpisy starsze niż 30 dni także wtedy, gdy źródła ostrzeżeń są niedostępne.

### Improved
- Etykiety konfiguracji są powiązane z polami formularza, a harmonogram ma semantyczny nagłówek sekcji.
- Statystyki pobierają tylko historię potrzebną do wybranego zakresu wykresu, ignorują spóźnione odpowiedzi po zmianie zakresu i ograniczają gęste serie do 1040 reprezentatywnych punktów z zachowaniem skrajnych wartości.

### Added
- Karta Status pokazuje wersję aplikacji, ułatwiając diagnozowanie i przypisanie wyników testów do wydania.
- Statystyki pokazują liczbę historycznych pomiarów według jakości oraz aktualne i nieaktualne odczyty według progów świeżości źródeł.
- Odczyt USB/serial zamyka port po anulowaniu kontekstu, dzięki czemu wyłączenie usługi nie pozostawia zablokowanego skanera; nieoczekiwany koniec strumienia jest widoczny jako błąd źródła.
- Konfiguracja czujnika USB/serial może sprawdzić dostępność portu przez `stty` bez odczytywania i konsumowania próbki.
- Aktualizator odrzuca równoległe instalacje tego samego wydania; odpowiedź API wskazuje, czy usługa sama uruchomi ponownie aplikację.
- Mapa lokalizacji zachowuje współrzędne `(0, 0)` dla nazwanej lokalizacji zamiast zastępować je domyślnym punktem Warszawy.
- Karta Status rozróżnia źródła pogody nieskonfigurowane od niedostępnych oraz pokazuje TTS jako gotowe dopiero wtedy, gdy silnik, model i wyjście audio są dostępne.
- Komunikaty instalatora Alpine precyzyjniej wyjaśniają, że automatyczna instalacja Piper jest pomijana w środowisku musl.
- Uzupełniono brakujące tłumaczenia ostrzeżeń i statusu w niemieckim, francuskim, hiszpańskim oraz włoskim. CI sprawdza teraz pokrycie wszystkich statycznych etykiet panelu w sześciu językach.
- Monitor APRS tłumaczy powody kwalifikacji i blokady ramek we wszystkich sześciu językach, również przy filtrze odległości.
- Parser APRS rozpoznaje ujemne temperatury w standardowym zapisie WX, np. `t-01`, zamiast pomijać taką ramkę jako niepogodową.
- APRS WX wykorzystuje znacznik czasu raportu UTC (`z`/`h`) do określenia wieku pomiarów; stare i błędnie oznaczone raporty pozostają widoczne w monitorze, ale nie zasilają bieżącej pogody. Poprawny lokalny znacznik APRS `/` zachowuje czas odbioru, bo odbiornik nie zna strefy czasowej nadawcy.
- Model ostrzeżeń rozróżnia nieznane prawdopodobieństwo od poprawnej wartości 0%; panel i komunikat głosowy zachowują tę różnicę.
- Prognoza za cztery godziny jest pomijana, gdy dostępny znacznik czasu jest odległy o więcej niż 90 minut od celu.
- Adapter Open-Meteo pomija brakujące wartości `null` osobno dla każdego parametru i zachowuje pozostałe dostępne pomiary godzinowe.
- Adapter Open-Meteo odrzuca odpowiedzi większe niż 4 MiB, zgodnie z limitami pozostałych źródeł zewnętrznych.
- Zakładki konfiguracji obsługują czytniki ekranu i przełączanie klawiszami strzałek; główna nawigacja oznacza aktualny widok.
- Odpowiedzi endpointów API mają `Cache-Control: no-store`, aby przeglądarka ani pośrednie cache nie przechowywały danych panelu.
- Wielokrajowy adapter MeteoAlarm/WIS2 odbiera ostrzeżenia z publicznych brokerów WMO; sprawdza warunki ponownego użycia, pobiera geometrię GeoJSON i CAP, a objęcie stacji potwierdza poligonem CAP. Alerty bez poligonu są pomijane.
- Ochrona linków MeteoAlarm odrzuca dodatkowe nieroutowalne zakresy IPv4 i IPv6, w tym adresy współdzielone, testowe, dokumentacyjne i translacyjne NAT64.
- Odświeżanie ostrzeżeń zachowuje aktualne alerty i zgłasza błąd, gdy zawiodą wszystkie źródła właściwe dla lokalizacji; brak lokalnych adapterów nie blokuje globalnego strumienia MeteoAlarm.
- Panel konfiguracji pozwala przetestować wybrane wyjście audio krótką próbką głosową, również przed zapisaniem konfiguracji. Próbka używa języka skonfigurowanego głosu niezależnie od języka interfejsu. Naprawiono też podwójne zwolnienie blokady po teście głosu Piper.
- Strefa czasowa stacji jest konfigurowana osobno od języka; harmonogram, data komunikatu i wypowiadane czasy ostrzeżeń używają wskazanej strefy IANA.
- Daty i godziny w panelu, w tym ostrzeżenia, historia pomiarów i etykiety wykresu, są prezentowane w strefie czasowej stacji.
- Odczyt BME280 używa pojedynczego wymuszonego pomiaru i czeka na gotowość danych oraz zakończenie kopiowania kalibracji, zamiast oznaczać poprzedni pomiar jako świeży.
- Baza stref czasowych IANA jest dołączana do aplikacji, aby harmonogram i czasy ostrzeżeń działały również na minimalnych instalacjach Linuksa bez systemowego pakietu `tzdata`.
- Budowa pakietu Piper ARMv7 korzysta teraz z przypiętego commitu eSpeak NG zamiast pobierać zmienną gałąź `master`.
- Nieudany zapis konfiguracji po zmianie lokalizacji przywraca poprzedni zestaw ostrzeżeń stacji.
- Etykieta priorytetu kierunku wiatru jest dostępna we wszystkich sześciu językach panelu.
- Wykaz licencji opisuje licencję danych mapowych OpenStreetMap i ograniczenia publicznego serwera kafelków.
- Na Linuksie zapis konfiguracji używa pliku tymczasowego i atomowej podmiany, aby przerwanie zapisu nie obcięło poprzedniej konfiguracji. Windows zachowuje ścieżkę zapisu zgodną z lokalnymi testami deweloperskimi.
- Przykłady BME280 wysyłają pierwszy pomiar od razu po uruchomieniu; Arduino czeka krótko między iteracjami zamiast zajętej pętli.
- Wyszukiwanie lokalizacji przyjmuje tylko zapytania GET, ogranicza długość wyszukiwanego tekstu i odrzuca błędne lub nadmiernie duże odpowiedzi geokodera.
- Adapter GeoSphere normalizuje tag języka (np. `pl-PL`) przed tłumaczeniem rodzaju i stopnia ostrzeżenia.
- Adaptery ostrzeżeń przyjmują prawdopodobieństwo zagrożenia tylko w zakresie 0–100; wartości błędne są pomijane.
- Panel odróżnia połączenie z brokerem WIS2 od pomyślnego przetworzenia powiadomienia; status nie sugeruje już odbioru danych tylko na podstawie otwartego połączenia.
- Przed pierwszą konfiguracją panelu wymagany jest jednorazowy token z pliku `setup-token`; chroni on również endpointy testowe i zapisy ustawień przed dostępem z innego hosta w sieci.
- Endpointy Status, pogody, statystyk, ramek APRS i listy audio odpowiadają wyłącznie na GET; POST nie uruchamia już operacji tylko do odczytu ani przycinania historii.
- Instalator rozpoznaje uruchomiony systemd albo dostępne narzędzia OpenRC; sama obecność `systemctl` nie wybiera już niedziałającego menedżera usług.
- Ponowne uruchomienie instalatora restartuje działającą usługę systemd/OpenRC po podmianie plików; przy pierwszej instalacji uruchamia nową usługę.
- Logowanie panelu ogranicza nieudane próby do 10 na adres klienta w oknie 15 minut, z limitem pamięci stanu prób.
- Panel pobiera progi świeżości z API pogody, więc oznaczenia pomiarów używają tych samych limitów co backend.
- Adaptery ostrzeżeń odrzucają odpowiedzi przekraczające limit rozmiaru zamiast parsować po cichu obcięty dokument.
- Wybór bieżących pomiarów stosuje te same limity świeżości co status źródeł, więc przeterminowany czujnik ustępuje świeżemu źródłu zapasowemu.
- Bieżące odczyty pokazują nazwę czujnika, jakość i świeżość pomiaru; stare odczyty są wizualnie oznaczone, a etykiety przetłumaczono na sześć języków.
- Ekstrakcja aktualizacji czyta archiwum bezpośrednio z pamięci bajtowej, unikając dodatkowej kopii całego pakietu.
- Aktualizator odrzuca wydania równe lub starsze od bieżącego oraz manifesty SHA-256 z brakującą, błędną lub powieloną sumą pakietu.
- Po udanej aktualizacji proces inicjuje kontrolowane zamknięcie, aby serwer HTTP i pozostałe goroutines zakończyły pracę przed restartem usługi.
- Udane odświeżenie źródła ostrzeżeń przenosi alerty, które zniknęły z bieżącego zestawu (np. po anulowaniu), do archiwum; alerty źródeł z błędem pozostają zachowane jako aktywne.
- Przycinanie historii APRS i odtworzeń odrzuca również wpisy z przyszłymi znacznikami czasu, zgodnie z filtrowaniem pomiarów pogody.
- Przed zapisaniem bieżących ostrzeżeń aplikacja usuwa duplikaty o tym samym źródle, ID i okresie; przy kilku wersjach zachowuje tę z najnowszym czasem wydania.
- Stan ogłoszeń i archiwum ostrzeżeń uwzględnia teraz źródło obok identyfikatora, aby alerty różnych dostawców o takim samym ID nie współdzieliły historii; zapisany stan jest migrowany ze starszego formatu przy pierwszym odczycie.
- Panel WWW ma szerszy układ na dużych ekranach, czytelniejsze odstępy, przewijane zakładki na telefonach, lepiej dopasowane karty pogody oraz oznaczenie stopnia ostrzeżenia.
- Dla każdego czujnika można wskazać preferowane urządzenie osobno dla każdego parametru. Preferencja działa w komunikatach pogodowych w obrębie globalnych priorytetów źródeł, a przy braku świeżego pomiaru wybór przechodzi do zwykłych źródeł zapasowych.
- Instrukcje w sześciu językach opisują wybór preferowanego czujnika oraz zachowanie historii i globalnych priorytetów źródeł.
- Ustawienia per urządzenie pozwalają wyłączyć błędne parametry; takie próbki nie zasilają bieżącej pogody ani lokalnego szacunku ciśnienia, a historia pozostaje zachowana.
- Instrukcje użytkownika w sześciu językach opisują filtrowanie parametrów czujników per urządzenie i wyjaśniają, że kolejność źródeł pozostaje globalna dla parametru.
- Archiwum dokumentacji wydania zawiera teraz przykłady firmware i dodatkowego locale, do których odsyłają instrukcje.
- Instrukcje dodawania własnego języka interfejsu są teraz dostępne we wszystkich sześciu wersjach manuala i rozróżniają język panelu od języka głosu TTS.
- Normalizacja kierunku wiatru przyjmuje równoważne jednostki `deg`, `degree` i `degrees` oraz zapisuje je jako `°`.
- Integracja Home Assistant rozpoznaje również pomiary kierunku wiatru z klasą `wind_direction` i jednostką `°`.
- Status i karta ostrzeżeń pokazują osobny stan IMGW-PIB, DWD i GeoSphere Austria: zakres lokalizacji, czas sprawdzenia, liczbę alertów lub błąd źródła.
- Zmiana konfiguracji ostrzeżeń resetuje statusy dostawców, aby panel nie przypisywał wyników poprzedniej lokalizacji do nowej.
- IMGW, DWD i GeoSphere Austria są odpytane równolegle, dzięki czemu wolniejszy dostawca nie zużywa czasu pozostałym źródłom ostrzeżeń.
- Klient MQTT obsługuje szyfrowane połączenia TLS przez adres `mqtts://` z domyślnym portem 8883.
- Karta Status pokazuje czasy ostatnich odczytów skonfigurowanych źródeł, harmonogram komunikatów i monitoring ostrzeżeń, w tym termin następnego odtworzenia oraz błąd pobierania.
- Monitor APRS pozwala blokować pojedyncze ramki WX po skrócie SHA-256; pozostają widoczne, ale ich rozpoznane pomiary są pomijane w bieżącej pogodzie i statystykach.
- Kierunek wiatru z APRS i Open-Meteo jest widoczny w bieżących odczytach, dostępny w bloczkach komunikatu i ma własne priorytety źródeł.
- APRS WX dekoduje kierunek i prędkość wiatru z pola pozycyjnego; pole `s` nie jest już błędnie używane jako prędkość, a pomiary spoza zakresu są pomijane.
- Monitor APRS pozwala jednym przyciskiem zablokować lub odblokować znak stacji; zablokowane ramki pozostają widoczne, a ich rozpoznane pomiary są pomijane w bieżącej pogodzie i statystykach.
- Zmiana współrzędnych stacji czyści aktywne ostrzeżenia poprzedniej lokalizacji przed zapisaniem nowego punktu.
- Ustalono minimalne wersje systemów i dodano do instalatora kontrolę wersji: Debian/Raspberry Pi OS 12, Ubuntu 24.04 LTS i Alpine 3.22. CI sprawdza też Debian 13 i Alpine 3.24.
- Adapter DWD wybiera jeden blok językowy CAP zgodny z językiem wybranego głosu TTS, w tym polskim, francuskim i hiszpańskim.
- Adapter DWD stosuje referencje CAP `Update` i `Cancel`, aby starsza wersja ostrzeżenia nie pozostała aktywna po aktualizacji lub anulowaniu.
- Stałe części głosowych ostrzeżeń (nagłówek, okres obowiązywania i prawdopodobieństwo) korzystają z języka wybranego głosu TTS.
- Długie opisy ostrzeżeń są odczytywane tylko wtedy, gdy dostawca dostarczył je w języku zgodnym z wybranym głosem; pełny tekst pozostaje w panelu.
- Etykiety rodzajów ostrzeżeń GeoSphere Austria są teraz wyświetlane w wybranym języku zamiast mieszać tłumaczenie z angielskim dopiskiem.
- Etykiety dostępności wyboru języka, sekcji konfiguracji i przycisków mapy zmieniają się razem z językiem interfejsu.
- Doprecyzowano w wykazie licencji podstawę ponownego użycia geolokalizowanych ostrzeżeń DWD i atrybucji.
- Poprawiono polski tekst karty bezpieczeństwa widoczny przed załadowaniem tłumaczeń panelu.
- Stopka panelu wymienia GeoSphere Austria i licencję CC BY 4.0 obok pozostałych źródeł ostrzeżeń.
- Interfejs pokazuje wyraźny fokus klawiatury i ogranicza animacje zgodnie z ustawieniem systemowym ograniczającym ruch.
- Dla każdego bloczka pogodowego można ustawić jednostki (globalne, metryczne, imperialne lub lotnicze) oraz standardowy albo zwięzły format wypowiedzi.
- Edytor bloków pogodowych obsługuje teraz do pięciu uporządkowanych źródeł zapasowych po źródle głównym.
- Dodano przeliczanie prędkości wiatru na jednostki profilu lotniczego (węzły); tryby metryczny i imperialny zachowują odpowiednio km/h i mph.
- Konfiguracja pozwala ustawić kolejność do sześciu źródeł zapasowych osobno dla każdego parametru pogody.
- Panel ostrzeżeń udostępnia zwijane archiwum zakończonych alertów z tego samego zakresu retencji co API (30 dni, maksymalnie 500 wpisów).
- Nieudane odtworzenie ostrzeżenia zapisuje sygnaturę próby i jest ponawiane po 5 minutach, zamiast co 30 sekund; udane komunikaty zachowują interwał zależny od stopnia.
- `Historia (dni)` obejmuje teraz retencję pomiarów pogodowych, ramek APRS i odtworzeń; ramki są przycinane również do limitu 10 000 wpisów, a przycięcie plików jest zapisywane na dysku.
- Konfiguracja audio wykrywa wejścia ALSA, zapisuje przypisane wejście APRS RX, pozwala sprawdzić przechwycenie próbki i zgłasza brak wybranego urządzenia; dekodowanie i przekazywanie ramek KISS pozostaje po stronie Dire Wolf/TNC.
- Wydania GitHub zawierają archiwum README, dokumentacji użytkownika w sześciu językach, wykazu licencji i historii zmian; paczka jest objęta `SHA256SUMS`.
- Instalator odrzuca teraz manifest sum, w którym brakuje wpisu lub wpis dla pobranego artefaktu jest powielony; dotyczy to także opcjonalnego Piper ARMv7.
- Instalator zapewnia narzędzia ALSA `aplay` i `arecord` na Alpine, Debianie, Raspberry Pi OS i Ubuntu; w razie błędu wyświetla informację, że funkcje audio mogą być niedostępne.
- Instalator sprawdza obecność `sed`, którego używa do odczytu tagu najnowszego wydania z GitHub API.
- Status źródeł pokazuje teraz świeżość na podstawie odebranych pomiarów oraz czas ostatniego odczytu, a nie tylko obecność ustawień połączenia.
- Panel statusu pokazuje ostatni błąd i czas próby dla MQTT, Home Assistant oraz USB/serial; komunikaty nie ujawniają adresów ani danych uwierzytelniających.
- Monitorowanie APRS/KISS zapisuje ostatnią próbę połączenia, przerwanie połączenia i czas ostatnio poprawnie zdekodowanej ramki.
- Strona Status pokazuje lokalny szacunek trendu ciśnienia z opisem ograniczeń, źródłem, liczbą próbek i okresem pomiarów; przy niewystarczających danych wyjaśnia, dlaczego szacunek jest niedostępny.
- Automatyczne dopasowanie powiatu IMGW do lokalizacji stacji w Polsce przez ULDK GUGiK; ręczne kody TERYT pozostają opcjonalnym nadpisaniem.
- Archiwum zakończonych ostrzeżeń IMGW z retencją 30 dni i limitem 500 wpisów.
- Status harmonogramu komunikatów z liczbą aktywnych pozycji i terminem następnego odtworzenia.
- Dodano priorytety źródeł bieżącej pogody osobno dla parametrów; panel pokazuje też harmonogram i archiwum zakończonych ostrzeżeń.
- Dodano domyślną wysokość czujników MQTT i USB/serial w panelu, z możliwością nadpisania jej przez `sensor_altitude_m` w próbce.
- Dodano ustawienia per urządzenie: własną nazwę i wysokość dla czujników MQTT, USB/serial, Home Assistant oraz BME280; wysokość próbki ma pierwszeństwo, a następnie działa dziedziczenie konfiguracji źródła lub stacji.
- Dla urządzeń raportujących ciśnienie można jawnie określić ciśnienie stacyjne lub zredukowane do poziomu morza; tryb automatyczny nie zgaduje rodzaju ciśnienia Home Assistant.
- Dodano przykłady firmware ESP32 + BME280/MQTT oraz Arduino + BME280/USB-serial z instrukcjami połączeń i konfiguracji.
- Czujniki MQTT i USB/serial mogą przekazywać wysokość `sensor_altitude_m`; wysokość domyślną można też ustawić osobno dla obu integracji w panelu. Aplikacja zachowuje ciśnienie stacyjne i wylicza ciśnienie zredukowane do poziomu morza.
- Status TTS rozróżnia obecność silnika Piper, modelu głosu i odtwarzacza audio; lokalny szacunek trendu ciśnienia pokazuje się tylko na podstawie świeżych pomiarów tego samego źródła i stacji. Status pokazuje też, czy skonfigurowane wyjście audio nadal jest dostępne.
- Status źródeł rozróżnia integracje nieustawione od skonfigurowanych, które nie dostarczają świeżych pomiarów.
- Dodano niemieckie ostrzeżenia DWD z oficjalnego kanału CAP; punkt stacji jest sprawdzany względem geometrii alertu, a wpisy bez geometrii lub czasu obowiązywania są pomijane.
- Dodano austriackie ostrzeżenia GeoSphere Austria dobierane automatycznie do gminy stacji oraz atrybucję CC BY 4.0.
- Dodano skrypt odinstalowujący obsługujący systemd i OpenRC; dane aplikacji są zachowywane domyślnie i usuwane tylko z opcją `--purge-data`.

### Improved
- Formularz konfiguracji wykorzystuje szerokość panelu, dzięki czemu zakładki i pola nie zostawiają dużej pustej kolumny na szerokich ekranach.

### Fixed
- Lokalny adapter ostrzeżeń, który potwierdzi brak zasięgu dla punktu stacji, nie usuwa już aktywnych alertów z globalnego strumienia MeteoAlarm; wygasają tylko alerty lokalne, które przestały dotyczyć stacji.
- Magazyn ostrzeżeń korzysta z windowsowego zapisu zgodnego z lokalnym środowiskiem developerskim, w którym system odmawia podmiany pliku przez `os.Rename`.
- Zapis konfiguracji w Windows usuwa plik tymczasowy po udanej podmianie.
- Wyszukiwanie lokalizacji zachowuje także poprawną wysokość `0` m lub wartość ujemną; podpowiedź nie jest już pomijana przez sprawdzenie prawdziwości liczby.
- Zmiana języka opisu ostrzeżenia aktualizuje jego podpis ogłoszenia, więc wersja z nowym językiem może zostać odczytana. Regionalne tagi tego samego języka (np. `pl` i `pl-PL`) nie wywołują zbędnego ponownego alarmu.
- Odczyty MQTT, USB/serial, Home Assistant i BME280 są sprawdzane pod kątem czasu z przyszłości oraz zakresów temperatury, wilgotności, ciśnienia, wiatru, opadu i wysokości czujnika.
- Format pomiarów MQTT i USB/serial przyjmuje opcjonalny kierunek wiatru `wind_direction_deg`, dostępny potem w odczytach, priorytetach źródeł i bloczkach komunikatu.
- Odrzucone próbki z Home Assistant, MQTT i USB/serial są widoczne jako błąd źródła; odrzucone pomiary BME280 pojawiają się w stanie czujnika.
- Przerwanie instalatora sygnałem HUP, INT lub TERM kończy jego działanie po sprzątnięciu katalogu tymczasowego.
- Home Assistant zachowuje czas aktualizacji encji zamiast czasu odpytywania API; niezmienione i niepoprawne odczyty nie są ponownie zapisywane.
- Ciśnienie z Home Assistant i innych źródeł obsługujących różne jednostki jest normalizowane do hPa przed korektą wysokości, wykresem i lokalnym szacunkiem trendu.
- Bieżące odczyty i tabela urządzeń stosują wybrany profil jednostek, tak jak wykres i komunikat.
- Awaria pobierania lub dekodowania dokumentu CAP DWD nie jest traktowana jako pusta lista ostrzeżeń; poprzednie alerty pozostają dostępne do czasu poprawnego odczytu źródła.
- Zmiana stacji do lokalizacji bez obsługiwanego dostawcy ostrzeżeń usuwa aktywne alerty poprzedniej lokalizacji; awaria sieci nadal zachowuje ostatnie poprawne ostrzeżenia.
- Parser KISS odrzuca nieznane i niekompletne sekwencje escape zamiast przekazywać uszkodzoną ramkę do dekodera AX.25.
- Procesy Piper i `aplay` są anulowane po zamknięciu żądania lub przekroczeniu limitu czasu, aby zawieszony silnik nie blokował komunikatów i zamykania usługi bez końca.
- Polecenia `arecord -L` i `aplay -L` używane do wykrywania urządzeń audio są przerywane po 5 sekundach.
- Log aktualizacji ostrzeżeń nie przypisuje już błędów dostawców DWD i GeoSphere Austria do IMGW.
- Przy tworzeniu sesji panelu usuwane są z pamięci sesje, których 12-godzinny czas ważności już upłynął.
- Endpointy logowania i konfiguracji kończą odpowiedź błędem, gdy nie uda się bezpiecznie utworzyć tokenu sesji.
- Przyszłe pomiary nie maskują już poprzedniego poprawnego odczytu tego samego źródła ani nie trafiają do wykresu historii i retencji.
- Monitor APRS ponownie ocenia zapisane ramki po zmianie lokalizacji, promienia lub czarnej listy, więc widok nie zachowuje nieaktualnej flagi blokady.
- Konwersja prędkości wiatru uwzględnia jednostkę źródłową m/s, km/h, mph lub węzły, dzięki czemu wartości m/s nie są błędnie przeliczane jak km/h.
- Konwersja opadu rozpoznaje jawnie milimetry oraz cale, w tym zapis `in`, i pozostawia nieznane jednostki bez zmian.
- Bieżące dane pogodowe odrzucają teraz pomiary z przyszłym znacznikiem czasu zamiast uznawać je za świeże.

## [0.0.6-prealpha.11] - 2026-10-20

### Added
- Dodano panel ostrzeżeń IMGW dla skonfigurowanych powiatów TERYT oraz lokalne komunikaty głosowe z powtarzaniem zależnym od stopnia zagrożenia.
- Dodano obsługę źródła i wymaganą atrybucję danych IMGW-PIB w dokumentacji.
- Dodano konfigurowalną czarną listę znaków APRS WX. Zablokowane ramki pozostają widoczne w monitorze, ale nie zasilają bieżących odczytów ani lokalnego szacunku.

## [0.0.6-prealpha.7] - 2026-10-19

### Added
- W wydaniu dodano eksperymentalny Piper ARMv7 wraz z paczką źródeł zależności, informacjami licencyjnymi i sumami kontrolnymi.
- Build Piper-phonemize przypina źródło eSpeak NG do konkretnego commita zamiast pobierać ruchomą gałąź.

### Fixed
- Stan deduplikacji komunikatów głosowych ostrzeżeń usuwa teraz wpisy starsze niż 30 dni, zamiast rosnąć bez ograniczeń.
- Uzupełniono reguły `.dockerignore`, aby źródła eSpeak i lokalnie zbudowane archiwum fonemizacji były dostępne w kontekstach Docker Buildx.
- Build Piper ARMv7 korzysta z przypiętego snapshotu Debiana, aby uniknąć niespójności indeksów pakietów i błędów 404 z mirrorów.
- Build Piper nie wymaga już dodatkowego testowego modelu głosu; gotowe binarium jest weryfikowane poleceniem `--help`.
- Test uruchomieniowy Piper ustawia ścieżkę do bibliotek dołączanych do pakietu ARMv7.

## [0.0.6-prealpha.1] - 2026-10-19

### Fixed
- Naprawiono porównywanie numerów wydań przedpremierowych, aby aktualizacja do kolejnego numeru, np. `.2` po `.1`, była wykrywana poprawnie.

## [0.0.5-prealpha.2] - 2026-10-19

### Added
- Dodano wybór zainstalowanego głosu oraz możliwość jego usunięcia z panelu. Usunięcie aktywnego głosu czyści również jego wybór i język komunikatu.

## [0.0.5-prealpha.1] - 2026-10-19

### Added
- Instalator automatycznie instaluje Piper TTS na Raspberry Pi OS/Debian i Ubuntu dla x86-64 oraz ARM64; na Alpine i ARMv7 wyświetla informację o braku zgodnego gotowego pakietu.
- Dodano w Konfiguracji zakładkę Aplikacja z aktualizatorem oraz importem i eksportem kopii konfiguracji.

## [0.0.4] - 2026-10-19

### Fixed
- Poprawiono sprawdzanie aktualizacji: aplikacja porównuje wersje semantycznie i wybiera najnowsze wydanie zamiast pierwszej pozycji zwracanej przez API GitHub.

## [0.0.3-prealpha.10] - 2026-10-19

### Added
- Dodano regulację programowej głośności komunikatu w zakresie 0–100%.

### Changed
- Usunięto nieużywany wybór wejścia audio: APRS odbierane jest z zewnętrznego TNC przez TCP KISS.
- Status urządzeń audio pokazuje wyjścia używane do odtwarzania komunikatów.
- Zoptymalizowano odczyt katalogu głosów, aby pobieranie z sieci nie blokowało innych operacji usługi TTS.
- Ograniczono pamięć harmonogramu do wpisów z bieżącego dnia.

### Fixed
- Zapis konfiguracji nie jest blokowany przez natywną walidację pól w ukrytych kartach; po zapisie ustawienia BME280 są weryfikowane, a status odświeżany.
- Niekompletne odpowiedzi prognozy nie tworzą już sztucznych odczytów równych zero.
- Ramki APRS WX bez pozycji są widoczne w monitorze WX, ale nie zasilają pogody.

## [0.0.3-prealpha.9] - 2026-10-19

### Added
- Dodano szeroki pasek bieżących odczytów na stronie Status z nazwą źródła i czasem pomiaru.
- Dodano natychmiastowy test odczytu BME280 z wynikami temperatury, wilgotności i ciśnienia.

### Changed
- Przeniesiono stan BME280 do listy usług na stronie Status.
- Przeniesiono potwierdzenie zapisu konfiguracji nad przycisk zapisu, aby było widoczne bez przewijania dalej.

## [0.0.3-prealpha.8] - 2026-10-19

### Added
- Dodano wybór preferowanego źródła bieżącej pogody: Open-Meteo, APRS WX/TNC, BME280, MQTT, Home Assistant lub USB/serial. Przy braku świeżego odczytu aplikacja korzysta z najnowszej dostępnej wartości; prognoza pozostaje z Open-Meteo.
- Dodano wykresy historii temperatury, wilgotności i ciśnienia z wyborem zakresu 24 godzin, 7 dni lub 30 dni.
- Dodano status odbioru BME280 z czasem ostatniego odczytu, błędem czujnika oraz automatycznym odświeżaniem.
- Dodano czytelne potwierdzenie zapisywania, zapisania i błędu zapisu konfiguracji.
- Rozszerzono polską instrukcję o pinout BME280, przykładowy wynik `i2cdetect` oraz wyjaśnienie adresów szesnastkowych i dziesiętnych.
- Zastąpiono surowy JSON historii odtworzeń komunikatów czytelnymi wpisami.

### Changed
- Uporządkowano statystyki i komunikaty, aby wyświetlały dane jako czytelne elementy interfejsu.

## [0.0.3-prealpha.7] - 2026-10-18

### Added
- Dodano zakładkę bezpieczeństwa z możliwością zmiany hasła oraz włączania i wyłączania logowania. Zmiana wymaga weryfikacji obecnego hasła, unieważnia dotychczasowe sesje i odnawia sesję bieżącą.

### Changed
- Ustabilizowano szerokość przycisku wylogowania i selektora języka dla dłuższych tłumaczeń oraz poprawiono układ etykiet i historii w statystykach.

## [0.0.3-prealpha.6] - 2026-10-18

### Fixed
- Poprawiono odczyt licencji z kart głosów Piper zapisanych jako listy Markdown, dzięki czemu zgodne głosy pojawiają się w katalogu.

### Added
- Dodano filtrowanie głosów według rodzaju, gdy katalog jednoznacznie podaje tę informację, oraz przycisk testu audio. Test pobiera głos w razie potrzeby bez ustawiania go jako aktywnego.
- Dodano uporządkowane sekcje konfiguracji czujników dla MQTT, Home Assistant, USB/serial i BME280 GPIO/I²C.

### Changed
- Przeniesiono katalog głosów do konfiguracji audio, a podgląd i odtwarzanie komunikatu do konfiguracji komunikatu i harmonogramu. Uproszczono główne menu aplikacji.

## [0.0.3-prealpha.5] - 2026-10-18

### Fixed
- Naprawiono pobieranie katalogu głosów Piper: katalogi językowe są teraz pobierane z obsługą paginacji API Hugging Face zamiast żądania odrzuconego przez limit serwera.

### Added
- Dodano wyszukiwarkę głosów oraz osobną listę zainstalowanych głosów z oznaczeniem głosu aktualnie używanego.

## [0.0.3-prealpha.4] - 2026-10-18

### Added
- Dodano lokalnie osadzone ikony Material Design Icons do nawigacji i podstawowych działań panelu; panel nie wymaga zewnętrznego CDN.

### Changed
- Wybór lokalizacji odbywa się w osobnym, większym oknie mapy z przesuwaniem, przybliżaniem kółkiem myszy i gestem szczypania oraz zatwierdzaniem punktu.
- Przyspieszono weryfikację hasła PBKDF2 na słabszych urządzeniach, zachowując liczbę iteracji i format zapisanych haseł.

## [0.0.3-prealpha.3] - 2026-10-18

### Fixed
- Przywrócono ładowanie kart statusu po zalogowaniu; błąd podglądu komunikatu nie blokuje wyświetlania pozostałych informacji.
- Przywrócono nawigację między głównymi sekcjami panelu i przełączanie zakładek konfiguracji.
- Przełączanie głównych sekcji nie nadpisuje niezapisanych zmian w konfiguracji; wybór języka odświeża również dynamiczne dane.
- Pierwsze uruchomienie ponownie pyta, czy użytkownik chce skorzystać z konfiguratora.
- Dodano widoczny stan oczekiwania podczas weryfikacji hasła.

## [0.0.3-prealpha.2] - 2026-10-18

### Fixed
- Dodano natychmiastową informację podczas logowania i zapisywania trybu bez logowania, aby kliknięcie nie wyglądało na zignorowane.
- Po instalacji aktualizacji panel czeka na ponowne uruchomienie usługi i automatycznie przeładowuje interfejs; po przekroczeniu limitu czasu udostępnia ręczne odświeżenie.

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

- Formularz ostrzeżeń nie wymaga już ręcznych kodów TERYT; dopasowanie IMGW opiera się na współrzędnych stacji. Starsze pliki konfiguracji nadal zachowują to pole, ale nie jest ono używane.
