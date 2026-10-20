# Changelog

W tym pliku zapisujemy istotne zmiany dla użytkowników. Format opiera się na [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), a wersje na [Semantic Versioning](https://semver.org/).

## [Unreleased]

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
