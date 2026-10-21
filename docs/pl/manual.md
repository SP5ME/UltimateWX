---
lang: pl
title: Instrukcja użytkownika UltimateWX
---

# Instrukcja użytkownika UltimateWX

UltimateWX to lokalna pogodynka dla Linuksa i stacji krótkofalarskiej. Łączy prognozę z Internetu, pobliskie ramki pogodowe APRS i pomiary czujników, a następnie składa komunikat z konfigurowalnych bloczków. Wersja 1 nie nadaje przez radio i nie steruje PTT.

## Instalacja

Instalator wydania obsługuje Raspberry Pi OS, Ubuntu i Alpine Linux na ARMv7, ARM64 oraz x86-64. Minimalna wersja to Raspberry Pi OS/Debian 12, Ubuntu 24.04 LTS lub Alpine Linux 3.22. Po opublikowaniu repozytorium uruchom polecenie z README z uprawnieniami administratora (zwykle przez `sudo`). Instalator sprawdza dystrybucję, architekturę procesora i system usług, pobiera właściwe archiwum, weryfikuje sumę SHA-256, instaluje usługę i włącza autostart. Na Raspberry Pi OS/Debian i Ubuntu z x86-64 lub ARM64 instaluje też Piper TTS w osobnym środowisku Pythona. Na Alpine i ARMv7 Piper pozostaje opcjonalny, ponieważ oficjalny pakiet nie udostępnia dla nich zgodnego, gotowego pliku.

Otwórz w sieci lokalnej adres `http://<adres-urzadzenia>:8080`. Przy pierwszym uruchomieniu odczytaj jednorazowy token poleceniem `sudo cat /var/lib/ultimatewx/setup-token` i wpisz go w panelu, zanim ustawisz hasło (co najmniej 6 znaków) albo wybierzesz pracę bez logowania. Konfigurator stacji pojawi się po zabezpieczeniu dostępu. Konfiguracja i historia są przechowywane w `/var/lib/ultimatewx`. Uwzględnij ten katalog w kopii zapasowej — zawiera hash hasła i dane dostępowe do integracji.

### Odinstalowanie

Pobierz `uninstall.sh` z plików wydania albo `scripts/uninstall.sh` z repozytorium, a następnie uruchom `sudo sh uninstall.sh`. Skrypt zatrzymuje i usuwa usługę systemd albo OpenRC oraz pliki programu i Piper zainstalowane razem z UltimateWX. Domyślnie zachowuje `/var/lib/ultimatewx` i konto usługi, aby można było zachować konfigurację, historię i głosy. Aby trwale usunąć również te dane, uruchom świadomie `sudo sh uninstall.sh --purge-data`; ta opcja bezpowrotnie usuwa katalog danych.

Do pracy developerskiej zainstaluj Go 1.23 lub nowsze i uruchom `go run ./cmd/ultimatewx --listen 127.0.0.1:8080`. Dane developerskie trafiają do `./data`.

## Język interfejsu

Panel zawiera języki angielski, polski, niemiecki, francuski, hiszpański i włoski. Dodatkowy język można dodać bez przebudowy programu: umieść plik JSON w katalogu `locales` w katalogu danych aplikacji (domyślnie `/var/lib/ultimatewx/locales`) i odśwież panel. Przykładowy plik oraz format znajdują się w [`examples/locales`](../../examples/locales). Nieprzetłumaczone etykiety korzystają z angielskich tekstów. Plik zmienia tylko język interfejsu; komunikaty głosowe nadal używają języka wybranego głosu TTS.

## Konfiguracja

Przy pierwszym uruchomieniu konfigurator stacji pojawia się przed ustawieniem dostępu do panelu. Po konfiguracji możesz ustawić hasło albo wybrać pracę bez logowania. Tryb bez hasła pozwala każdej osobie mającej dostęp do panelu korzystać z niego; używaj go tylko w zaufanej sieci lokalnej.

Zakładka **Konfiguracja → Aplikacja** zawiera sprawdzanie i instalowanie aktualizacji oraz eksport i import kopii konfiguracji. Kopia obejmuje ustawienia integracji wraz z hasłami MQTT i tokenami Home Assistant, ale nie zawiera hasła panelu ani historii pomiarów. Przechowuj plik kopii w bezpiecznym miejscu.


Zakładka **Status** pokazuje skonfigurowane źródła, liczbę wyjść audio i stan wybranego wyjścia, czas ostatniej aktualizacji pogody online oraz podgląd komunikatu. **Konfiguracja** jest podzielona na zakładki: Stacja pogodowa, Lokalizacja stacji, Źródła pogody, Czujniki, Audio i głos oraz Komunikat i harmonogram. W zakładce Stacja pogodowa ustaw znak i lokator Maidenhead. Druga zakładka, Lokalizacja stacji, służy do wyszukiwania miejscowości, ustawienia wysokości i wskazania punktu na mapie. Kliknij mapę albo przeciągnij ją tak, by punkt stacji znalazł się pod celownikiem; współrzędne są widoczne w polach i zapisują się ze wspólnym formularzem. Mapa używa kafelków OpenStreetMap i wymaga internetu. W tekstowym bloku komunikatu użyj `<call>` dla znaku i `<loc>` dla lokatora, np. `Tu stacja pogodowa <call> znajdująca się w <loc>.` W harmonogramie dodaj stałe godziny albo cykliczne odtwarzanie co godzinę, np. 5 minut po każdej pełnej godzinie. Zaznacz pozycje, które mają być aktywne. Przełączanie zakładek zachowuje wpisane zmiany; użyj wspólnego przycisku zapisu, aby zapisać formularz. **Statystyki** pokazują zachowane pomiary, ramki i odtworzenia. **Monitor APRS** wyświetla wszystkie zapisane ramki albo tylko WX oraz wyjaśnia, czy ramka WX może zasilać pogodę. Zakres w Statystykach można wybrać osobno; ustawienie Historia (dni) określa retencję pomiarów, ramek APRS i odtworzeń. Ramki są dodatkowo ograniczone do 10 000 najnowszych wpisów. Dla bloczków komunikatu ze źródłem automatycznym ustaw kolejność źródeł osobno dla każdego parametru; kolejność określa, które dostępne pomiary mają pierwszeństwo. Profil lotniczy używa °C, węzłów (kt), hPa i mm; profil imperialny używa °F, mph, hPa i cali. Dla bloczków pogodowych możesz ustawić do pięciu źródeł zapasowych w kolejności po źródle głównym. Każdy bloczek pogodowy może też używać jednostek globalnych lub własnych oraz pełnego albo zwięzłego formatu wypowiedzi. Ustaw strefę czasową stacji jako nazwę IANA, np. `Europe/Warsaw`; jest używana w harmonogramie, dacie komunikatu i wypowiadanym czasie ważności ostrzeżeń.

Open-Meteo jest odpytywane co 15 minut. Darmowe API jest przeznaczone do użytku niekomercyjnego, a dane wymagają atrybucji CC BY 4.0; panel pokazuje informację o źródle. W **Konfiguracja → Źródła pogody** wybierz preferowane źródło bieżących pomiarów: automatycznie, Open-Meteo, pobliskie ramki APRS WX odbierane przez TNC, BME280, MQTT, Home Assistant albo USB/serial. Preferencja dotyczy bloczków komunikatu ustawionych na źródło automatyczne; gdy brakuje świeżego pomiaru wybranego parametru, używany jest najnowszy dostępny odczyt. Prognoza pozostaje z Open-Meteo. W edytorze bloczka możesz wskazać źródło osobno.

W tym samym panelu ustawisz kolejność źródeł osobno dla temperatury, wilgotności, ciśnienia, prędkości i kierunku wiatru, opadu i temperatury odczuwalnej. Użyj selektorów priorytetu w kolejności od 1; puste pozycje są pomijane. Aplikacja wybierze pierwsze źródło ze świeżym pomiarem. Jeśli żadne źródło z listy nie ma pomiaru, użyje źródła globalnego, a potem najnowszego odczytu. Pozostaw wszystkie priorytety puste, aby korzystać tylko z ustawienia globalnego.

W tym samym panelu można wpisać znaki stacji APRS WX do czarnej listy, oddzielone przecinkami. Dopasowanie obejmuje pełny znak stacji wraz z SSID i nie rozróżnia wielkości liter. Zablokowane ramki nadal są dostępne w monitorze z oznaczeniem blokady, ale ich nowe i zapisane pomiary są pomijane przy wyznaczaniu bieżących danych i lokalnego szacunku. Historia pomiarów nie jest usuwana.

### Ostrzeżenia IMGW, DWD i GeoSphere Austria

Zakładka **Ostrzeżenia** automatycznie dopasowuje źródło do zapisanych współrzędnych. W Polsce pobiera aktywne i nadchodzące ostrzeżenia IMGW dla powiatu wyznaczonego automatycznie na podstawie współrzędnych stacji przez ULDK GUGiK. W Niemczech korzysta z kanału CAP Deutscher Wetterdienst (DWD) i pokazuje tylko alerty, których geometria obejmuje punkt stacji. Komunikaty CAP typu `Update` i `Cancel` zastępują wskazane poprzednie wersje, aby anulowane ostrzeżenie nie pozostało aktywne. W Austrii pyta oficjalne API GeoSphere Austria o ostrzeżenia dla gminy zawierającej zapisane współrzędne stacji. Treść DWD jest wybierana według języka skonfigurowanego głosu TTS (z angielskim fallbackiem); język panelu pozostaje osobnym ustawieniem. Do dopasowania potrzebne jest połączenie z Internetem, a niemieckie ostrzeżenia bez geometrii lub poprawnego czasu obowiązywania są pomijane. Dla pozostałych państw aplikacja korzysta z publicznego strumienia MeteoAlarm/WIS2 i potwierdza objęcie stacji poligonem z obszaru CAP; alerty bez poligonu są pomijane. Strumień nie odtwarza powiadomień pominiętych podczas przerwy w połączeniu; zapisane alerty pozostają w aplikacji do wygaśnięcia. Aplikacja odświeża dane co 10 minut. Archiwum zakończonych ostrzeżeń jest dostępne w zwijanej sekcji przez 30 dni (maksymalnie 500 wpisów). Głosowe powtarzanie ostrzeżeń ma osobny interwał dla stopnia 1, 2 i 3 (domyślnie 60, 30 i 15 minut); zmiana treści, stopnia albo okresu pozwala ogłosić nową wersję. Wymagany jest głos TTS. Po błędzie odtworzenia ponowna próba nastąpi najwcześniej po 5 minutach. Opisy ostrzeżeń są odczytywane głosem tylko wtedy, gdy ich język jest zgodny z językiem TTS; w panelu nadal widać pełną treść dostawcy. Karty ostrzeżeń pokazują dostawcę i jego atrybucję. Aplikacja nie nadaje ostrzeżeń przez radio ani APRS-IS.

APRS łączy się z zewnętrznym punktem KISS TCP Dire Wolf, zwykle `127.0.0.1:8001`. Skonfiguruj Dire Wolf, aby odbierał dźwięk z radia i przekazywał ramki KISS do UltimateWX. Domyślny promień kwalifikowania źródła WX to 10 km. Ramki bez obsługiwanej pozycji, z błędnymi polami pogody, poza promieniem lub odebrane ponad dwie godziny temu pozostają w monitorze, ale nie zasilają odczytów pogodowych. Ramki od zablokowanej stacji pozostają widoczne w monitorze. Znak można dodać do czarnej listy lub z niej usunąć przyciskiem w wierszu ramki; filtr obejmuje bieżące i przyszłe pomiary tej stacji, a historia nie jest kasowana. Parser odczytuje kierunek i prędkość z pola po symbolu stacji pogodowej; symbol `s` pozostaje śniegiem, a wartości spoza zakresu są pomijane. W monitorze można też zablokować pojedynczą ramkę WX. Identyczne ramki pozostają widoczne, lecz ich pomiary są pomijane; blokadę można cofnąć tym samym przyciskiem. Rozpoznane pomiary z zablokowanych stacji i ramek są pomijane w bieżących odczytach oraz statystykach; zapisane dane nie są kasowane.

W zakładce Audio i głos aplikacja wykrywa wejścia ALSA i może sprawdzić wybrane, przechwytując jednosekundową próbkę audio. Ustaw to samo urządzenie wejściowe w Dire Wolf/TNC, który dekoduje sygnał radiowy i przekazuje ramki przez KISS. Wybierz też wyjście komunikatu i ustaw jego programową głośność od 0% do 100%. Do odtwarzania wymagane są ALSA (`aplay`) i lokalne polecenie Piper.

Edytor komunikatu zapisuje uporządkowane bloczki w JSON. Każdy można włączyć lub wyłączyć, określić jego typ i źródło, pola albo tekst. W edytorze harmonogramu można dodać stałe godziny albo cykl co godzinę z przesunięciem od 0 do 59 minut po pełnej godzinie. Każdą pozycję można osobno włączyć lub wyłączyć. Domyślne pory są wyłączone. Blok `forecast` korzysta z prognozy Open-Meteo za cztery godziny; `local-estimate` jest jawnie oznaczony jako szacunek i pomijany bez świeżych pomiarów ciśnienia.

## Czujniki

Wszystkie źródła czujnikowe używają wspólnych pojęć: nazwy urządzenia, czasu pomiaru, parametru, wartości liczbowej i jednostki. MQTT przyjmuje jeden obiekt JSON na wiadomość, na przykład:

```json
{"device":"roof-esp32","name":"Czujnik dachowy","timestamp":"2026-10-18T10:00:00Z","sensor_altitude_m":180,"temperature_c":12.5,"humidity_percent":72,"pressure_hpa":1008.2}
```

Pole `device` jest identyfikatorem wymaganym, a opcjonalne `name` jest czytelną nazwą pokazywaną w statusie. Bez `name` użyty zostanie identyfikator. Ostatnie próbki z jednego tematu MQTT są rozdzielane według identyfikatora urządzenia.

Skonfiguruj broker, np. `tcp://homeassistant.local:1883`, albo szyfrowane połączenie TLS `mqtts://broker.example.org` (domyślny port 8883), i filtr tematu, np. `ultimatewx/+/weather`. Przycisk testu sprawdza zestawienie połączenia oraz przyjęcie subskrypcji przez broker. Firmware ESP32 może publikować taki obiekt przez Wi-Fi. Używaj osobnego konta brokera i nie umieszczaj haseł w publicznych logach.

W **Konfiguracja → Czujniki** przy każdym wykrytym urządzeniu możesz odznaczyć parametry, które podaje błędnie. Takie odczyty nie zasilają bieżącej pogody ani lokalnego szacunku ciśnienia, ale pozostają w historii. Zaznacz **Preferuj w komunikacie**, aby dla danego parametru najpierw wybierać to urządzenie, gdy ma świeży odczyt. Globalne priorytety integracji i źródła zapasowe bloczków nadal obowiązują; przy braku świeżego odczytu działa zwykły wybór źródła.

Odczyty czujników z MQTT, USB/serial, Home Assistant i BME280 są odrzucane, jeśli znacznik czasu wyprzedza zegar aplikacji o ponad 5 minut albo wartości przekraczają obsługiwane zakresy: temperatura od −100 do 70 °C, wilgotność 0–100%, ciśnienie 300–1200 hPa, wiatr 0–120 m/s, opcjonalny kierunek wiatru 0–360° (pole JSON `wind_direction_deg`, wymaga czujnika kierunku), opad 0–10 000 mm i wysokość czujnika −500–9000 m.

W **Konfiguracja → Czujniki** możesz ustawić domyślną wysokość dla wszystkich pomiarów odbieranych z MQTT albo USB/serial, a po pierwszym odczycie przypisać osobną nazwę i wysokość każdemu wykrytemu urządzeniu. Pusta wysokość urządzenia dziedziczy ustawienie globalne, a `sensor_altitude_m` z próbki ma pierwszeństwo przed obiema konfiguracjami. Przy pomiarach ciśnienia możesz określić, czy urządzenie podaje ciśnienie stacyjne, czy zredukowane do poziomu morza; automatycznie aplikacja zachowuje rodzaj wynikający z formatu źródła i nie zgaduje go dla Home Assistant. BME280 domyślnie dziedziczy wysokość stacji.

Integracja Home Assistant korzysta z REST API i tokenu dostępu. Ustaw adres bazowy oraz token. Przycisk testu sprawdza autoryzowany dostęp do API; odczyty encji są pobierane okresowo. Rozpoznawane są klasy: temperatura, wilgotność, ciśnienie, prędkość i kierunek wiatru (jednostka `°`) oraz opad. Obsługiwane jednostki ciśnienia są przeliczane do hPa przed wyświetleniem i ewentualną korektą wysokości. Czas próbki pochodzi ze znacznika aktualizacji encji Home Assistant, więc niezmieniona wartość nie jest uznawana za nowy pomiar.

Dla USB/serial wybierz urządzenie, np. `/dev/ttyUSB0`, ustaw prędkość transmisji i wysyłaj jeden obiekt JSON zakończony znakiem nowej linii dla każdej próbki. Przycisk **Test połączenia** sprawdza dostęp urządzenia i odczyt jego ustawień przez `stty`; nie potwierdza odbioru ani poprawności próbek JSON. Obowiązują nazwy pól z przykładu MQTT. Pole `sensor_altitude_m` jest opcjonalne; gdy je podasz, UltimateWX zapisze ciśnienie stacyjne i dodatkowo wyliczy ciśnienie zredukowane do poziomu morza. Wysokość dotyczy konkretnego czujnika, a nie samej anteny/stacji, jeśli są na różnych wysokościach. Gotowe szkice znajdziesz w [`examples/arduino-bme280-serial`](../../examples/arduino-bme280-serial) oraz [`examples/esp32-bme280-mqtt`](../../examples/esp32-bme280-mqtt); ich instrukcje opisują połączenia, zależności i konfigurację UltimateWX.

### BME280 bezpośrednio do Raspberry Pi (I²C)

Przy wyłączonym zasilaniu połącz moduł z 40-pinowym złączem Raspberry Pi:

| Pin na module BME280 | Pin fizyczny Raspberry Pi | Sygnał |
| --- | ---: | --- |
| `VCC` lub `VIN` | 1 (3V3) | Zasilanie 3,3 V |
| `GND` | 6 (GND) | Masa |
| `SDA` lub `SDI` | 3 (GPIO2 / SDA1) | Dane I²C |
| `SCL` lub `SCK` | 5 (GPIO3 / SCL1) | Zegar I²C |

Użyj 3,3 V i nie podłączaj linii GPIO do 5 V. Jeśli moduł ma tylko cztery wyprowadzenia (`VCC`, `GND`, `SDA`, `SCL`), powyższe połączenia wystarczą. Przy module z dodatkowymi pinami ustaw `CS`/`CSB` na 3,3 V, aby wybrać I²C. Pin `SDO` wybiera adres: połączenie z `GND` oznacza `0x76`, a z 3,3 V — `0x77`. W niektórych modułach adres jest już ustawiony zworką lub ścieżką; sprawdź opis swojej płytki. W konfiguracji UltimateWX wpisz odpowiednio `118` dla `0x76` lub `119` dla `0x77`.

W Raspberry Pi OS włącz I²C poleceniem `sudo raspi-config`, wybierając **Interface Options → I2C → Enable**, i uruchom ponownie komputer. Jeśli nie masz narzędzia do sprawdzenia magistrali, zainstaluj `i2c-tools` (`sudo apt install i2c-tools`), a potem uruchom `sudo i2cdetect -y 1`. W tabeli powinien pojawić się adres `76` albo `77`. Przykładowy wynik dla czujnika pod adresem `0x76`:

```text
pi@raspberrypi:~ $ sudo i2cdetect -y 1
     0  1  2  3  4  5  6  7  8  9  a  b  c  d  e  f
00:                         -- -- -- -- -- -- -- --
10: -- -- -- -- -- -- -- -- -- -- -- -- -- -- -- --
20: -- -- -- -- -- -- -- -- -- -- -- -- -- -- -- --
30: -- -- -- -- -- -- -- -- -- -- -- -- -- -- -- --
40: -- -- -- -- -- -- -- -- -- -- -- -- -- -- -- --
50: -- -- -- -- -- -- -- -- -- -- -- -- -- -- -- --
60: -- -- -- -- -- -- -- -- -- -- -- -- -- -- -- --
70: -- -- -- -- -- -- 76 --
pi@raspberrypi:~ $
```

`i2cdetect` wyświetla adresy szesnastkowo — dlatego w tabeli widzisz `76`, bez prefiksu `0x`. Prefiks `0x` oznacza zapis szesnastkowy. UltimateWX w polu adresu przyjmuje tę samą wartość jako liczbę dziesiętną: `0x76` to `7 × 16 + 6 = 118`, a `0x77` to `7 × 16 + 7 = 119`. Nie musisz samodzielnie ustalać adresu: odczytaj `76` lub `77` z wyniku `i2cdetect` i wpisz odpowiadającą mu liczbę dziesiętną.

To, czy moduł odpowiada pod `0x76`, czy `0x77`, zależy od pinu `SDO` albo zworki adresowej na płytce: `SDO` połączone z masą wybiera `0x76`, a z 3,3 V — `0x77`. W zakładce **Konfiguracja → Czujniki** włącz BME280, pozostaw urządzenie `/dev/i2c-1`, ustaw wyliczoną wartość adresu i zapisz konfigurację.

BME280 mierzy ciśnienie stacyjne. UltimateWX osobno wylicza ciśnienie zredukowane do poziomu morza na podstawie wysokości stacji. Schemat pinów Raspberry Pi znajdziesz w [oficjalnej dokumentacji GPIO](https://www.raspberrypi.com/documentation/computers/raspberry-pi.html), a wymagania trybu I²C i adresów opisuje [karta katalogowa Bosch BME280](https://www.bosch-sensortec.com/media/boschsensortec/downloads/datasheets/bst-bme280-ds002.pdf).

## FAQ i diagnostyka

**Panel się nie otwiera.** Sprawdź działanie usługi i dostępność portu TCP 8080 w sieci lokalnej. Dla systemd użyj `systemctl status ultimatewx`, a dla OpenRC `rc-service ultimatewx status`.

**Brak danych APRS.** Sprawdź audio odbiornika w Dire Wolf, host i port KISS TCP oraz to, czy TNC przekazuje ramki AX.25 UI. UltimateWX odbiera wynikowy strumień KISS; test wejścia audio przechwytuje tylko próbkę i nie zastępuje ciągłego dekodowania przez Dire Wolf/TNC.

**Ramka z bliska nie jest używana.** W monitorze sprawdź pozycję, pola pogody, czas odebrania i odległość. Zwiększ promień tylko wtedy, gdy odpowiada to planowanemu zasięgowi stacji.

**Głos TTS jest niedostępny.** Żaden język ani głos nie jest wybrany domyślnie. W zakładce Głos i komunikat wybierz język, wczytaj katalog i pobierz głos; aplikacja ustawi język komunikatu zgodnie z pobranym głosem. Katalog wymaga Internetu. Aplikacja pokazuje tylko rozpoznane licencje i weryfikuje pliki modelu. Instalator Linuksa automatycznie instaluje Piper TTS na obsługiwanych systemach Debian/Raspberry Pi OS i Ubuntu. Na Alpine i ARMv7 Piper trzeba zainstalować osobno i wskazać jego ścieżkę w polu Polecenie Piper. Pobrany głos działa lokalnie.

**Co oznacza szacunek offline?** To prosty wskaźnik trendu ciśnienia, a nie prognoza meteorologiczna. Potrzebuje co najmniej dwóch świeżych pomiarów ciśnienia stacyjnego z tego samego źródła i stacji, w odstępie minimum godziny i z ostatnich pięciu godzin. Gdy warunki są spełnione, wynik pojawia się na stronie Status i może być dodany do komunikatu jako lokalny szacunek.

**Jak zaktualizować aplikację?** Wybierz **Sprawdź aktualizacje**, zapoznaj się z informacją o wydaniu, a następnie wybierz **Zainstaluj aktualizację i uruchom ponownie**. Aplikacja sprawdza pakiet względem manifestu SHA-256. Konto usługi musi mieć prawo zapisu do katalogu programu — oficjalny instalator ustawia takie uprawnienia.

**Gdzie jest kopia zapasowa?** Zapisz cały `/var/lib/ultimatewx`, w tym `config.json`, historię JSONL i pobrane głosy. Kopia może zawierać dane dostępowe.

\n
