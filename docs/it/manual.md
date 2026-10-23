---
lang: it
title: Manuale utente UltimateWX
---

# Manuale utente UltimateWX

UltimateWX è un annunciatore meteo locale per Linux e stazioni radioamatoriali. Combina previsioni online, frame meteorologici APRS nelle vicinanze e letture dei sensori, quindi compone un annuncio con blocchi configurabili. La versione 1 non trasmette via radio e non controlla il PTT.

## Installazione

L’installer delle release supporta Raspberry Pi OS, Ubuntu e Alpine Linux su ARMv7, ARM64 e x86-64. Versioni minime supportate: Raspberry Pi OS/Debian 12, Ubuntu 24.04 LTS e Alpine Linux 3.22. Dopo la pubblicazione del repository, esegui il comando indicato nel README con privilegi amministrativi, normalmente tramite `sudo`. L’installer rileva distribuzione, architettura e sistema init, scarica l’archivio corretto, verifica il checksum SHA-256, installa il servizio e abilita l’avvio automatico. Su Raspberry Pi OS/Debian e Ubuntu per x86-64/ARM64 installa Piper 1.8.0 in un ambiente isolato in `/opt/ultimatewx/piper`. Su ARMv7 installa una versione sperimentale di Piper 1.2.0 nella stessa cartella. Alpine non riceve un’installazione automatica; un motore compatibile con musl può essere configurato manualmente nelle impostazioni avanzate.

Al primo avvio, inserisci prima il token di configurazione monouso, poi imposta una password oppure continua senza accesso. In seguito apparirà la procedura guidata della stazione. Senza password, chiunque possa raggiungere il pannello può usarlo; usa questa modalità solo su una rete locale fidata.

Per eseguire la versione di sviluppo, installa Go 1.23 o successivo e avvia `go run ./cmd/ultimatewx --listen 127.0.0.1:8080`. I dati di sviluppo sono salvati in `./data`.

## Lingua dell’interfaccia

Il pannello include inglese, polacco, tedesco, francese, spagnolo e italiano. Puoi aggiungere un’altra lingua senza ricompilare il programma: inserisci un file JSON nella cartella `locales` dentro la directory dei dati dell’applicazione (predefinita: `/var/lib/ultimatewx/locales`), quindi ricarica il pannello. Un esempio e il formato del file sono disponibili in [`examples/locales`](../../examples/locales). Le etichette non tradotte vengono mostrate in inglese. Il file cambia solo la lingua dell’interfaccia; gli annunci vocali continuano a usare la lingua della voce TTS selezionata.

## Configurazione

Al primo avvio, inserisci prima il token di configurazione monouso, poi imposta una password oppure continua senza accesso. In seguito apparirà la procedura guidata della stazione. Senza password, chiunque possa raggiungere il pannello può usarlo; usa questa modalità solo su una rete locale fidata.

La scheda **Configurazione → Applicazione** permette di cercare e installare aggiornamenti, oltre a esportare e importare un backup della configurazione. Il backup include le impostazioni delle integrazioni, comprese le password MQTT e i token di Home Assistant, ma non la password del pannello né lo storico delle misurazioni. Conserva il file di backup in un luogo sicuro.


La pagina **Stato** mostra le fonti configurate, i dispositivi audio, l’ultimo aggiornamento meteo online e l’anteprima dell’annuncio. In **Configurazione** imposta posizione, coordinate, quota, unità, lingua, raggio APRS, interfacce, orari e blocchi dell’annuncio. **Statistiche** presenta misure, frame e riproduzioni conservati. Il **Monitor APRS** mostra tutti i frame salvati o solo quelli WX e spiega se ciascun frame può fornire dati meteo. L’intervallo del grafico si sceglie separatamente; «Cronologia (giorni)» regola la conservazione di misurazioni, frame APRS e riproduzioni. I frame APRS hanno inoltre un limite di 10.000 voci. Per i blocchi dell’annuncio con fonte automatica, imposta un ordine di priorità per ogni parametro; l’ordine determina quali misurazioni disponibili vengono preferite. Il profilo aeronautico usa °C, nodi (kt), hPa e mm; quello imperiale usa °F, mph, hPa e pollici. Per i blocchi meteo, imposta fino a cinque fonti di riserva in ordine dopo la fonte principale. Ogni blocco meteo può inoltre usare unità globali o proprie e un formato parlato standard o compatto. Indica un fuso IANA, ad esempio `Europe/Rome`; viene usato per il programma, la data parlata e la validità degli avvisi.

Open-Meteo viene interrogato ogni 15 minuti. La sua API gratuita è destinata a uso non commerciale e i dati richiedono attribuzione CC BY 4.0. Il pannello indica la fonte. Se queste condizioni non sono adatte al tuo utilizzo, disattiva Open-Meteo e configura un altro fornitore quando sarà disponibile un adattatore.

APRS si collega a un endpoint TCP KISS esterno di Dire Wolf, normalmente `127.0.0.1:8001`. Configura Dire Wolf per ricevere l’audio radio e inoltrare i frame KISS a UltimateWX. Il raggio WX predefinito è 10 km. I frame senza posizione supportata, con campi meteo non validi, oltre il raggio o ricevuti da più di due ore restano visibili nel monitor ma non vengono usati come dati meteo. I frame di una stazione bloccata restano visibili nel monitor. Usa il pulsante nella riga per bloccare o sbloccare il nominativo; il filtro vale per le letture attuali e future di quella stazione e non cancella lo storico. Il parser legge direzione e velocità dal campo dopo il simbolo meteo; il campo `s` resta dedicato alla neve e i valori fuori intervallo vengono ignorati. Il monitor consente anche di bloccare un singolo frame WX. I frame corrispondenti restano visibili, ma le relative letture vengono escluse; usa lo stesso pulsante per sbloccarli. Le letture identificate come provenienti da stazioni e frame bloccati vengono escluse dalle letture correnti e dalle statistiche; i dati salvati non vengono eliminati.

La scheda Audio e voce elenca i dispositivi di acquisizione ALSA e prova l’ingresso selezionato acquisendo un campione audio di un secondo. Configura lo stesso ingresso in Dire Wolf/TNC, che continua a decodificare l’audio radio e inoltra i frame tramite KISS. Seleziona l’uscita dell’annuncio e regola il volume software da 0% a 100%. Per la riproduzione servono ALSA (`aplay`) e un comando Piper locale.

L’editor dell’annuncio salva blocchi ordinati in JSON. Ogni blocco può essere attivato, assegnato a un tipo e a una fonte e configurato con campi o testo. Gli orari predefiniti sono disattivati. Un blocco `forecast` usa la previsione Open-Meteo per quattro ore dopo; `local-estimate` viene indicato esplicitamente come stima e omesso se mancano misure di pressione recenti.

## Sensori

Tutti i sensori condividono gli stessi concetti: nome del dispositivo, timestamp, parametro, valore numerico e unità. MQTT accetta un oggetto JSON per messaggio, ad esempio:

```json
{"device":"roof-esp32","timestamp":"2026-10-18T10:00:00Z","temperature_c":12.5,"humidity_percent":72,"pressure_hpa":1008.2}
```

Configura un broker come `tcp://homeassistant.local:1883` oppure una connessione TLS come `mqtts://broker.example.org` (porta predefinita 8883), e un filtro topic come `ultimatewx/+/weather`. Il firmware ESP32 può pubblicare questo oggetto tramite Wi-Fi. Usa un account broker dedicato e non inserire credenziali nei log pubblici.

In **Configurazione → Sensori** puoi deselezionare, per ogni dispositivo rilevato, le misure che fornisce in modo errato. Non verranno usate per il meteo attuale né per la stima locale della pressione, ma la cronologia viene mantenuta. Seleziona **Preferisci negli annunci** per scegliere prima quel dispositivo per il parametro quando dispone di una misura recente. Restano attive le priorità globali delle integrazioni e le fonti di riserva dei blocchi; se manca una misura recente, si usa la normale selezione delle fonti.

Le letture dei sensori MQTT, USB/seriale, Home Assistant e BME280 vengono rifiutate se l’orario supera di oltre cinque minuti quello dell’applicazione o se un valore esce dagli intervalli supportati: temperatura da −100 a 70 °C, umidità 0–100 %, pressione 300–1200 hPa, velocità del vento 0–120 m/s, direzione opzionale del vento 0–360° (campo JSON `wind_direction_deg`, richiede un sensore di direzione), precipitazioni 0–10.000 mm e altitudine del sensore −500–9000 m.

L’integrazione Home Assistant usa l’API REST e un token di accesso. Configura l’URL di base e il token. Le entità sensore con classi riconosciute (temperatura, umidità, pressione, velocità e direzione del vento (unità `°`) o precipitazioni) vengono lette periodicamente.

Per USB/seriale, seleziona un dispositivo Linux come `/dev/ttyUSB0`, imposta il baud rate e invia un oggetto JSON seguito da una nuova riga per ogni campione. **Test connessione** verifica l’accesso al dispositivo e legge le impostazioni con `stty`; non verifica la ricezione dei campioni né il loro formato JSON. Sono accettati gli stessi nomi di campo dell’esempio MQTT.

Per collegare direttamente un BME280 a Raspberry Pi, abilita il profilo I²C, imposta `/dev/i2c-1` e l’indirizzo 118 (`0x76`) o 119 (`0x77`), quindi collega il sensore ai pin I²C e alla tensione logica corretta. L’applicazione memorizza la pressione di stazione e calcola separatamente quella ridotta al livello del mare usando la quota configurata. Verifica indirizzo e cablaggio prima di abilitare il profilo.

## FAQ e risoluzione dei problemi

**Il pannello non si apre.** Verifica che il servizio sia attivo e che la porta TCP 8080 sia raggiungibile nella rete locale. Con systemd: `systemctl status ultimatewx`; con OpenRC: `rc-service ultimatewx status`.

**Non arrivano dati APRS.** Controlla l’audio del ricevitore in Dire Wolf, host e porta KISS TCP e che il TNC invii frame AX.25 UI. UltimateWX legge il flusso KISS risultante; il test audio acquisisce solo un breve campione e non sostituisce la decodifica continua di Dire Wolf/TNC.

**Un frame vicino non viene utilizzato.** Controlla posizione, campi meteo, ora di ricezione e distanza nel monitor. Aumenta il raggio solo se corrisponde alla copertura desiderata.

**La voce TTS non è disponibile.** Nella sezione Voce e annunci scegli una lingua, carica il catalogo e scarica una voce. Il catalogo richiede Internet; i modelli scaricati funzionano localmente. L’installer di sistema colloca Piper in `/opt/ultimatewx/piper` su Debian/Raspberry Pi OS e Ubuntu x86-64/ARM64 e installa una versione sperimentale su ARMv7. Se l’installazione non riesce o usi Alpine, installa un motore compatibile e inserisci il percorso nelle impostazioni audio avanzate compresse.

**Che cosa significa la stima offline?** È un semplice indicatore della tendenza della pressione, non una previsione meteorologica. Richiede almeno due misure recenti di pressione di stazione distanti almeno un’ora, negli ultimi cinque ore. L’annuncio la identifica come stima locale.

**Come si aggiorna?** Seleziona **Controlla aggiornamenti**, consulta le note di release e poi **Installa e riavvia**. L’applicazione verifica il pacchetto con il manifesto SHA-256. L’account del servizio deve poter scrivere nella cartella del programma; l’installer ufficiale imposta questi permessi.

**Dove sono i backup?** Salva `/var/lib/ultimatewx`, inclusi `config.json`, la cronologia JSONL e le voci scaricate. Il backup può contenere credenziali.
