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

Zakładka **Status** pokazuje skonfigurowane źródła, urządzenia audio, czas ostatniej aktualizacji pogody online i podgląd komunikatu. **Konfiguracja** jest podzielona na zakładki: Stacja pogodowa, Źródła pogody, Czujniki, Audio i głos oraz Komunikat i harmonogram. W zakładce Stacja pogodowa ustaw znak i lokator Maidenhead, wyszukaj miejscowość lub wskaż punkt na mapie. Kliknij mapę albo przeciągnij ją tak, by punkt stacji znalazł się pod celownikiem; współrzędne są widoczne w polach i zapisują się ze wspólnym formularzem. Mapa używa kafelków OpenStreetMap i wymaga internetu. W tekstowym bloku komunikatu użyj `<call>` dla znaku i `<loc>` dla lokatora, np. `Tu stacja pogodowa <call> znajdująca się w <loc>.` W harmonogramie dodaj stałe godziny albo cykliczne odtwarzanie co godzinę, np. 5 minut po każdej pełnej godzinie. Zaznacz pozycje, które mają być aktywne. Przełączanie zakładek zachowuje wpisane zmiany; użyj wspólnego przycisku zapisu, aby zapisać formularz. **Statystyki** pokazują zachowane pomiary, ramki i odtworzenia. **Monitor APRS** wyświetla wszystkie zapisane ramki albo tylko WX oraz wyjaśnia, czy ramka WX może zasilać pogodę.

Open-Meteo jest odpytywane co 15 minut. Darmowe API jest przeznaczone do użytku niekomercyjnego, a dane wymagają atrybucji CC BY 4.0; panel pokazuje informację o źródle. Jeśli te warunki nie pasują do sposobu użycia, wyłącz źródło i skonfiguruj innego dostawcę, gdy będzie dostępny odpowiedni adapter.

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

Aby podłączyć BME280 bezpośrednio do Raspberry Pi, włącz profil I²C, ustaw `/dev/i2c-1` i adres 118 (`0x76`) albo 119 (`0x77`), a czujnik podłącz do pinów I²C płytki z właściwym napięciem logicznym. Aplikacja zapisuje ciśnienie stacyjne i osobno wylicza ciśnienie zredukowane do poziomu morza na podstawie ustawionej wysokości. Przed włączeniem sprawdź adres czujnika i przewody.

## FAQ i diagnostyka

**Panel się nie otwiera.** Sprawdź działanie usługi i dostępność portu TCP 8080 w sieci lokalnej. Dla systemd użyj `systemctl status ultimatewx`, a dla OpenRC `rc-service ultimatewx status`.

**Brak danych APRS.** Sprawdź audio odbiornika w Dire Wolf, host i port KISS TCP oraz to, czy TNC przekazuje ramki AX.25 UI. Wybrane wejście audio jest obecnie wskazówką dla konfiguracji odbiornika; UltimateWX odbiera wynikowy strumień KISS.

**Ramka z bliska nie jest używana.** W monitorze sprawdź pozycję, pola pogody, czas odebrania i odległość. Zwiększ promień tylko wtedy, gdy odpowiada to planowanemu zasięgowi stacji.

**Głos TTS jest niedostępny.** Żaden język ani głos nie jest wybrany domyślnie. W zakładce Głos i komunikat wybierz język, wczytaj katalog i pobierz głos; aplikacja ustawi język komunikatu zgodnie z pobranym głosem. Katalog wymaga Internetu. Aplikacja pokazuje tylko rozpoznane licencje i weryfikuje pliki modelu. Sam Piper musi być zainstalowany i dostępny jako skonfigurowane polecenie; pobrany głos działa lokalnie.

**Co oznacza szacunek offline?** To prosty wskaźnik trendu ciśnienia, a nie prognoza meteorologiczna. Potrzebuje co najmniej dwóch świeżych pomiarów ciśnienia stacyjnego w odstępie minimum godziny, z ostatnich pięciu godzin. Komunikat nazywa wynik lokalnym szacunkiem.

**Jak zaktualizować aplikację?** Wybierz **Sprawdź aktualizacje**, zapoznaj się z informacją o wydaniu, a następnie wybierz **Zainstaluj aktualizację i uruchom ponownie**. Aplikacja sprawdza pakiet względem manifestu SHA-256. Konto usługi musi mieć prawo zapisu do katalogu programu — oficjalny instalator ustawia takie uprawnienia.

**Gdzie jest kopia zapasowa?** Zapisz cały `/var/lib/ultimatewx`, w tym `config.json`, historię JSONL i pobrane głosy. Kopia może zawierać dane dostępowe.

\n