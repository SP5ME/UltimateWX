# Odbiór sprzętowy UltimateWX

Ta lista uzupełnia testy CI. Nie zastępuje testów automatycznych ani nie potwierdza działania sprzętu, dopóki nie zostanie wykonana na docelowym urządzeniu.

## Dane z testu

- Data i wersja UltimateWX:
- Model Raspberry Pi i architektura:
- System i wersja:
- Interfejs audio / urządzenie wejściowe:
- Interfejs audio / urządzenie wyjściowe:
- Odbiornik APRS i wersja Dire Wolf/TNC:
- Czujnik BME280 oraz adres I²C:
- Osoba wykonująca test:

Przy każdym punkcie wpisz `zaliczony`, `niezaliczony` albo `nie dotyczy`, a przy wyniku innym niż zaliczony dodaj objaw i log. Nie umieszczaj haseł, tokenów ani konfiguracji zawierającej sekrety w załączanych logach.

Wersję aplikacji odczytaj z karty **Wersja aplikacji** na ekranie Status. Architekturę i system możesz zebrać poleceniami:

```sh
uname -m
cat /etc/os-release
```

Dla systemd dołącz zredagowany status i ostatnie logi usługi:

```sh
sudo systemctl status ultimatewx.service --no-pager
sudo journalctl -u ultimatewx.service --since "30 minutes ago" --no-pager
```

Dla OpenRC sprawdź stan przez `sudo rc-service ultimatewx status`. Logi pobierz z używanego w systemie sysloga (np. `sudo logread`, jeśli jest dostępne). Przed udostępnieniem usuń z logów hasła, tokeny, adresy prywatne i inne dane, których nie chcesz ujawniać.

## Instalacja i uruchomienie

- [ ] Zainstaluj aplikację na czystym, wspieranym Raspberry Pi OS/Debianie zgodnie z instrukcją.
- [ ] Sprawdź, czy usługa uruchamia się po instalacji i po ponownym uruchomieniu urządzenia.
- [ ] Otwórz panel z innego urządzenia w zaufanej sieci lokalnej; sprawdź pierwszą konfigurację, logowanie i zapis ustawień.
- [ ] Zapisz, czy status usługi, źródeł oraz ostatniej aktualizacji odpowiada rzeczywistemu stanowi urządzenia.

## BME280 przez I²C

- [ ] Podłącz zgodny czujnik i sprawdź go przy użyciu przycisku testu połączenia w konfiguracji.
- [ ] Włącz źródło BME280 i sprawdź, czy status pokazuje odbiór pomiarów.
- [ ] Porównaj temperaturę, wilgotność i ciśnienie z odczytem niezależnego narzędzia; zanotuj różnice i konfigurację wysokości/rodzaju ciśnienia.
- [ ] Odłącz czujnik. Panel powinien przestać oznaczać pomiar jako świeży i pokazać błąd źródła; po ponownym podłączeniu odbiór powinien wrócić.

## Wejście i wyjście audio

- [ ] Wybierz wejście APRS RX i wykonaj jednosekundowy test przechwycenia. Potwierdź, że wybrano właściwe urządzenie. Dekodowanie radiowe pozostaje zadaniem Dire Wolf/TNC.
- [ ] Wybierz głos Piper, wyjście audio i poziom głośności; odtwórz próbkę oraz komunikat z panelu.
- [ ] Sprawdź status po odłączeniu wybranego urządzenia i po jego ponownym podłączeniu.

## Odbiór APRS WX

- [ ] Sprawdź test połączenia do skonfigurowanego serwera KISS/Dire Wolf.
- [ ] Odbierz poprawną, aktualną ramkę WX z pozycją w skonfigurowanym promieniu. Powinna być widoczna w monitorze i — jeżeli spełnia pozostałe kryteria — zasilić bieżącą pogodę.
- [ ] Odbierz lub odtwórz w środowisku testowym ramkę błędną, niepogodową, bez pozycji, nieaktualną oraz spoza promienia. Sprawdź powód odrzucenia i upewnij się, że odrzucone pomiary nie zmieniają bieżącej pogody.
- [ ] Dodaj znak stacji do czarnej listy i sprawdź, że jej ramki pozostają widoczne, ale nie zasilają pogody.
- [ ] Zablokuj identyfikator pojedynczej ramki WX w monitorze i sprawdź ten sam warunek dla ramki, bez blokowania całego znaku.

## Harmonogram i ostrzeżenia

- [ ] Ustaw najbliższy harmonogram w strefie czasowej stacji i potwierdź odtworzenie o wskazanej porze.
- [ ] Sprawdź status adapterów ostrzeżeń i lokalne dopasowanie dla skonfigurowanej lokalizacji. Brak aktywnych ostrzeżeń nie jest błędem testu; nie wywołuj prawdziwego alertu w celu testowania.
- [ ] Jeżeli dostawca publikuje w czasie testu ostrzeżenie dla lokalizacji stacji, potwierdź jego obszar, stopień, źródło, ważność i powtarzanie komunikatu. W przeciwnym razie oznacz ten punkt `nie dotyczy` i polegaj na testach adaptera w CI.

## Wydajność starszego Raspberry Pi

Jeśli wydanie zawiera eksperymentalny Piper ARMv7, wykonaj próbne pobranie głosu i odtworzenie komunikatu na deklarowanym urządzeniu docelowym (w szczególności Raspberry Pi 2 B). Zapisz czas generowania i odtwarzania, zużycie pamięci oraz ewentualne błędy. Nie wnioskuj o obsłudze tej platformy wyłącznie z udanej kompilacji.

## Wynik

- Wady blokujące wydanie:
- Wady nieblokujące:
- Wynik końcowy: `zaliczony` / `niezaliczony` / `niekompletny`
- Link lub ścieżka do zredagowanych logów i notatek:
