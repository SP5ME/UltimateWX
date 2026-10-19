---
lang: it
title: Manuale utente UltimateWX
---

# Manuale utente UltimateWX

UltimateWX è un annunciatore meteo locale per Linux e stazioni radioamatoriali. Combina previsioni online, frame meteorologici APRS nelle vicinanze e letture dei sensori, quindi compone un annuncio con blocchi configurabili. La versione 1 non trasmette via radio e non controlla il PTT.

## Installazione

L’installer delle release supporta Raspberry Pi OS, Ubuntu e Alpine Linux su ARMv7, ARM64 e x86-64. Dopo la pubblicazione del repository, esegui il comando indicato nel README con privilegi amministrativi, normalmente tramite `sudo`. L’installer rileva distribuzione, architettura e sistema init, scarica l’archivio corretto, verifica il checksum SHA-256, installa il servizio e abilita l’avvio automatico. Su Raspberry Pi OS/Debian e Ubuntu per x86-64 o ARM64 installa anche Piper TTS in un ambiente Python separato. Piper resta facoltativo su Alpine e ARMv7 perché il pacchetto ufficiale non pubblica un wheel precompilato compatibile con queste piattaforme.

Apri nella rete locale `http://<ip-dispositivo>:8080`. Al primo avvio, completa prima la procedura guidata. Poi puoi impostare una password di almeno sei caratteri oppure continuare senza accesso. Configurazione e cronologia sono conservate in `/var/lib/ultimatewx`. Includi questa cartella nei backup: contiene l’hash della password e le credenziali delle integrazioni.

Per eseguire la versione di sviluppo, installa Go 1.23 o successivo e avvia `go run ./cmd/ultimatewx --listen 127.0.0.1:8080`. I dati di sviluppo sono salvati in `./data`.

## Configurazione

Al primo avvio, la procedura guidata appare prima di configurare l’accesso al pannello. Dopo la configurazione puoi impostare una password oppure continuare senza accesso. Senza password, chiunque possa raggiungere il pannello può usarlo; usa questa modalità solo su una rete locale fidata.

La scheda **Configurazione → Applicazione** permette di cercare e installare aggiornamenti, oltre a esportare e importare un backup della configurazione. Il backup include le impostazioni delle integrazioni, comprese le password MQTT e i token di Home Assistant, ma non la password del pannello né lo storico delle misurazioni. Conserva il file di backup in un luogo sicuro.


La pagina **Stato** mostra le fonti configurate, i dispositivi audio, l’ultimo aggiornamento meteo online e l’anteprima dell’annuncio. In **Configurazione** imposta posizione, coordinate, quota, unità, lingua, raggio APRS, interfacce, orari e blocchi dell’annuncio. **Statistiche** presenta misure, frame e riproduzioni conservati. Il **Monitor APRS** mostra tutti i frame salvati o solo quelli WX e spiega se ciascun frame può fornire dati meteo.

Open-Meteo viene interrogato ogni 15 minuti. La sua API gratuita è destinata a uso non commerciale e i dati richiedono attribuzione CC BY 4.0. Il pannello indica la fonte. Se queste condizioni non sono adatte al tuo utilizzo, disattiva Open-Meteo e configura un altro fornitore quando sarà disponibile un adattatore.

APRS si collega a un endpoint TCP KISS esterno di Dire Wolf, normalmente `127.0.0.1:8001`. Configura Dire Wolf per ricevere l’audio radio e inoltrare i frame KISS a UltimateWX. Il raggio WX predefinito è 10 km. I frame senza posizione supportata, con campi meteo non validi, oltre il raggio o ricevuti da più di due ore restano visibili nel monitor ma non vengono usati come dati meteo.

UltimateWX riceve i frame APRS da un TNC esterno tramite TCP KISS, quindi qui non serve un ingresso audio. Seleziona l’uscita dell’annuncio e regola il volume software da 0% a 100%. Per la riproduzione servono ALSA (`aplay`) e un comando Piper locale.

L’editor dell’annuncio salva blocchi ordinati in JSON. Ogni blocco può essere attivato, assegnato a un tipo e a una fonte e configurato con campi o testo. Gli orari predefiniti sono disattivati. Un blocco `forecast` usa la previsione Open-Meteo per quattro ore dopo; `local-estimate` viene indicato esplicitamente come stima e omesso se mancano misure di pressione recenti.

## Sensori

Tutti i sensori condividono gli stessi concetti: nome del dispositivo, timestamp, parametro, valore numerico e unità. MQTT accetta un oggetto JSON per messaggio, ad esempio:

```json
{"device":"roof-esp32","timestamp":"2026-10-18T10:00:00Z","temperature_c":12.5,"humidity_percent":72,"pressure_hpa":1008.2}
```

Configura un broker come `tcp://homeassistant.local:1883` e un filtro topic come `ultimatewx/+/weather`. Il firmware ESP32 può pubblicare questo oggetto tramite Wi-Fi. Usa un account broker dedicato e non inserire credenziali nei log pubblici.

L’integrazione Home Assistant usa l’API REST e un token di accesso. Configura l’URL di base e il token. Le entità sensore con classi riconosciute (temperatura, umidità, pressione, velocità del vento o precipitazioni) vengono lette periodicamente.

Per USB/seriale, seleziona un dispositivo Linux come `/dev/ttyUSB0`, imposta il baud rate e invia un oggetto JSON seguito da una nuova riga per ogni campione. Sono accettati gli stessi nomi di campo dell’esempio MQTT.

Per collegare direttamente un BME280 a Raspberry Pi, abilita il profilo I²C, imposta `/dev/i2c-1` e l’indirizzo 118 (`0x76`) o 119 (`0x77`), quindi collega il sensore ai pin I²C e alla tensione logica corretta. L’applicazione memorizza la pressione di stazione e calcola separatamente quella ridotta al livello del mare usando la quota configurata. Verifica indirizzo e cablaggio prima di abilitare il profilo.

## FAQ e risoluzione dei problemi

**Il pannello non si apre.** Verifica che il servizio sia attivo e che la porta TCP 8080 sia raggiungibile nella rete locale. Con systemd: `systemctl status ultimatewx`; con OpenRC: `rc-service ultimatewx status`.

**Non arrivano dati APRS.** Controlla l’audio del ricevitore in Dire Wolf, host e porta KISS TCP e che il TNC invii frame AX.25 UI. UltimateWX legge il flusso KISS risultante e non acquisisce direttamente l’audio del ricevitore.

**Un frame vicino non viene utilizzato.** Controlla posizione, campi meteo, ora di ricezione e distanza nel monitor. Aumenta il raggio solo se corrisponde alla copertura desiderata.

**La voce TTS non è disponibile.** Nella scheda voce scegli una lingua e carica il catalogo quando Internet è disponibile. L’applicazione mostra solo licenze riconosciute e verifica i file del modello. L’installer Linux installa automaticamente Piper TTS sui sistemi Debian/Raspberry Pi OS e Ubuntu supportati. Su Alpine e ARMv7, installa Piper separatamente e inserisci il percorso dell’eseguibile nel campo Comando Piper. Le voci scaricate restano locali.

**Che cosa significa la stima offline?** È un semplice indicatore della tendenza della pressione, non una previsione meteorologica. Richiede almeno due misure recenti di pressione di stazione distanti almeno un’ora, negli ultimi cinque ore. L’annuncio la identifica come stima locale.

**Come si aggiorna?** Seleziona **Controlla aggiornamenti**, consulta le note di release e poi **Installa e riavvia**. L’applicazione verifica il pacchetto con il manifesto SHA-256. L’account del servizio deve poter scrivere nella cartella del programma; l’installer ufficiale imposta questi permessi.

**Dove sono i backup?** Salva `/var/lib/ultimatewx`, inclusi `config.json`, la cronologia JSONL e le voci scaricate. Il backup può contenere credenziali.
