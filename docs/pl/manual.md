---
lang: pl
title: Instrukcja użytkownika UltimateWX
---

# Instrukcja użytkownika UltimateWX

UltimateWX to lokalna pogodynka dla Linuksa i stacji krótkofalarskiej. Łączy prognozę z Internetu, pobliskie ramki pogodowe APRS i pomiary czujników, a następnie składa komunikat z konfigurowalnych bloczków. Wersja 1 nie nadaje przez radio i nie steruje PTT.

## Instalacja

Instalator wydania obsługuje Raspberry Pi OS, Ubuntu i Alpine Linux na ARMv7, ARM64 oraz x86-64. Po opublikowaniu repozytorium uruchom polecenie z README z uprawnieniami administratora (zwykle przez `sudo`). Instalator sprawdza dystrybucję, architekturę procesora i system usług, pobiera właściwe archiwum, weryfikuje sumę SHA-256, instaluje usługę i włącza autostart. Właściciel repozytorium w poleceniu musi odpowiadać repozytorium na GitHubie.

Otwórz w sieci lokalnej adres `http://<adres-urzadzenia>:8080`. Przy pierwszym uruchomieniu przejdź konfigurator stacji, a potem wybierz hasło (co najmniej 6 znaków) albo pracę bez logowania. Konfiguracja i historia są przechowywane w `/var/lib/ultimatewx`. Uwzględnij ten katalog w kopii zapasowej — zawiera hash hasła i dane dostępowe do integracji.

Do pracy developerskiej zainstaluj Go 1.23 lub nowsze i uruchom `go run ./cmd/ultimatewx --listen 127.0.0.1:8080`. Dane developerskie trafiają do `./data`.

## Konfiguracja

Przy pierwszym uruchomieniu konfigurator stacji pojawia się przed ustawieniem dostępu do panelu. Po konfiguracji możesz ustawić hasło albo wybrać pracę bez logowania. Tryb bez hasła pozwala każdej osobie mającej dostęp do panelu korzystać z niego; używaj go tylko w zaufanej sieci lokalnej.

Zakładka **Status** pokazuje skonfigurowane źródła, urządzenia audio, czas ostatniej aktualizacji pogody online i podgląd komunikatu. **Konfiguracja** jest podzielona na zakładki: Stacja pogodowa, Lokalizacja stacji, Źródła pogody, Czujniki, Audio i głos oraz Komunikat i harmonogram. W zakładce Stacja pogodowa ustaw znak i lokator Maidenhead. Druga zakładka, Lokalizacja stacji, służy do wyszukiwania miejscowości, ustawienia wysokości i wskazania punktu na mapie. Kliknij mapę albo przeciągnij ją tak, by punkt stacji znalazł się pod celownikiem; współrzędne są widoczne w polach i zapisują się ze wspólnym formularzem. Mapa używa kafelków OpenStreetMap i wymaga internetu. W tekstowym bloku komunikatu użyj `<call>` dla znaku i `<loc>` dla lokatora, np. `Tu stacja pogodowa <call> znajdująca się w <loc>.` W harmonogramie dodaj stałe godziny albo cykliczne odtwarzanie co godzinę, np. 5 minut po każdej pełnej godzinie. Zaznacz pozycje, które mają być aktywne. Przełączanie zakładek zachowuje wpisane zmiany; użyj wspólnego przycisku zapisu, aby zapisać formularz. **Statystyki** pokazują zachowane pomiary, ramki i odtworzenia. **Monitor APRS** wyświetla wszystkie zapisane ramki albo tylko WX oraz wyjaśnia, czy ramka WX może zasilać pogodę.

Open-Meteo jest odpytywane co 15 minut. Darmowe API jest przeznaczone do użytku niekomercyjnego, a dane wymagają atrybucji CC BY 4.0; panel pokazuje informację o źródle. W **Konfiguracja → Źródła pogody** wybierz preferowane źródło bieżących pomiarów: automatycznie, Open-Meteo, pobliskie ramki APRS WX odbierane przez TNC, BME280, MQTT, Home Assistant albo USB/serial. Preferencja dotyczy bloczków komunikatu ustawionych na źródło automatyczne; gdy brakuje świeżego pomiaru wybranego parametru, używany jest najnowszy dostępny odczyt. Prognoza pozostaje z Open-Meteo. W edytorze bloczka możesz wskazać źródło osobno.

APRS łączy się z zewnętrznym punktem KISS TCP Dire Wolf, zwykle `127.0.0.1:8001`. Skonfiguruj Dire Wolf, aby odbierał dźwięk z radia i przekazywał ramki KISS do UltimateWX. Domyślny promień kwalifikowania źródła WX to 10 km. Ramki bez obsługiwanej pozycji, z błędnymi polami pogody, poza promieniem lub odebrane ponad dwie godziny temu pozostają w monitorze, ale nie zasilają odczytów pogodowych.

Wejście i wyjście audio wybiera się osobno. Wejście dotyczy toru odbiorczego skonfigurowanego w Dire Wolf, a wyjście służy do odtwarzania komunikatu przez `aplay`. Do wykrywania urządzeń i odtwarzania potrzebne są narzędzia ALSA (`arecord`, `aplay`) oraz lokalne polecenie Piper. Wybór wejścia w panelu nie konfiguruje automatycznie Dire Wolf.

Edytor komunikatu zapisuje uporządkowane bloczki w JSON. Każdy można włączyć lub wyłączyć, określić jego typ i źródło, pola albo tekst. W edytorze harmonogramu można dodać stałe godziny albo cykl co godzinę z przesunięciem od 0 do 59 minut po pełnej godzinie. Każdą pozycję można osobno włączyć lub wyłączyć. Domyślne pory są wyłączone. Blok `forecast` korzysta z prognozy Open-Meteo za cztery godziny; `local-estimate` jest jawnie oznaczony jako szacunek i pomijany bez świeżych pomiarów ciśnienia.

## Czujniki

Wszystkie źródła czujnikowe używają wspólnych pojęć: nazwy urządzenia, czasu pomiaru, parametru, wartości liczbowej i jednostki. MQTT przyjmuje jeden obiekt JSON na wiadomość, na przykład:

```json
{"device":"roof-esp32","timestamp":"2026-10-18T10:00:00Z","temperature_c":12.5,"humidity_percent":72,"pressure_hpa":1008.2}
```

Skonfiguruj broker, np. `tcp://homeassistant.local:1883`, i filtr tematu, np. `ultimatewx/+/weather`. Firmware ESP32 może publikować taki obiekt przez Wi-Fi. Używaj osobnego konta brokera i nie umieszczaj haseł w publicznych logach.

Integracja Home Assistant korzysta z REST API i tokenu dostępu. Ustaw adres bazowy oraz token. Odczytywane są okresowo encje czujników o rozpoznanych klasach: temperatura, wilgotność, ciśnienie, prędkość wiatru lub opad.

Dla USB/serial wybierz urządzenie, np. `/dev/ttyUSB0`, ustaw prędkość transmisji i wysyłaj jeden obiekt JSON zakończony znakiem nowej linii dla każdej próbki. Obowiązują nazwy pól z przykładu MQTT.

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

**Brak danych APRS.** Sprawdź audio odbiornika w Dire Wolf, host i port KISS TCP oraz to, czy TNC przekazuje ramki AX.25 UI. Wybrane wejście audio jest obecnie wskazówką dla konfiguracji odbiornika; UltimateWX odbiera wynikowy strumień KISS.

**Ramka z bliska nie jest używana.** W monitorze sprawdź pozycję, pola pogody, czas odebrania i odległość. Zwiększ promień tylko wtedy, gdy odpowiada to planowanemu zasięgowi stacji.

**Głos TTS jest niedostępny.** Żaden język ani głos nie jest wybrany domyślnie. W zakładce Głos i komunikat wybierz język, wczytaj katalog i pobierz głos; aplikacja ustawi język komunikatu zgodnie z pobranym głosem. Katalog wymaga Internetu. Aplikacja pokazuje tylko rozpoznane licencje i weryfikuje pliki modelu. Sam Piper musi być zainstalowany i dostępny jako skonfigurowane polecenie; pobrany głos działa lokalnie.

**Co oznacza szacunek offline?** To prosty wskaźnik trendu ciśnienia, a nie prognoza meteorologiczna. Potrzebuje co najmniej dwóch świeżych pomiarów ciśnienia stacyjnego w odstępie minimum godziny, z ostatnich pięciu godzin. Komunikat nazywa wynik lokalnym szacunkiem.

**Jak zaktualizować aplikację?** Wybierz **Sprawdź aktualizacje**, zapoznaj się z informacją o wydaniu, a następnie wybierz **Zainstaluj aktualizację i uruchom ponownie**. Aplikacja sprawdza pakiet względem manifestu SHA-256. Konto usługi musi mieć prawo zapisu do katalogu programu — oficjalny instalator ustawia takie uprawnienia.

**Gdzie jest kopia zapasowa?** Zapisz cały `/var/lib/ultimatewx`, w tym `config.json`, historię JSONL i pobrane głosy. Kopia może zawierać dane dostępowe.

\n