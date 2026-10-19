---
lang: fr
title: Guide utilisateur UltimateWX
---

# Guide utilisateur UltimateWX

UltimateWX est un annonceur météo local pour Linux et les stations radioamateur. Il combine les prévisions en ligne, les trames météo APRS proches et les mesures de capteurs, puis compose une annonce à partir de blocs configurables. La version 1 n’émet pas par radio et ne commande pas le PTT.

## Installation

L’installateur de version prend en charge Raspberry Pi OS, Ubuntu et Alpine Linux en ARMv7, ARM64 et x86-64. Après la publication du dépôt, lancez la commande indiquée dans le README avec les droits administrateur, généralement via `sudo`. L’installateur détecte la distribution, l’architecture et le système de services, télécharge l’archive adaptée, vérifie sa somme SHA-256, installe le service et active le démarrage automatique. Le propriétaire du dépôt dans la commande doit correspondre au projet GitHub publié.

Depuis le réseau local, ouvrez `http://<adresse-appareil>:8080`. Au premier démarrage, suivez d’abord l’assistant de configuration. Vous pourrez ensuite définir un mot de passe d’au moins six caractères ou continuer sans connexion. La configuration et l’historique sont stockés dans `/var/lib/ultimatewx`. Sauvegardez ce répertoire : il contient le hash du mot de passe et les identifiants des intégrations.

Pour un lancement de développement, installez Go 1.23 ou une version ultérieure et exécutez `go run ./cmd/ultimatewx --listen 127.0.0.1:8080`. Les données de développement sont placées dans `./data`.

## Configuration

Au premier démarrage, l’assistant de configuration s’affiche avant le choix du mode d’accès au panneau. Ensuite, vous pouvez définir un mot de passe ou continuer sans connexion. Sans mot de passe, toute personne pouvant accéder au panneau peut l’utiliser ; réservez ce mode à un réseau local de confiance.

La page **État** affiche les sources configurées, les périphériques audio, la dernière mise à jour météo en ligne et l’aperçu du message. Dans **Configuration**, réglez l’emplacement, les coordonnées, l’altitude, les unités, la langue, le rayon APRS, les interfaces, les horaires et les blocs d’annonce. **Statistiques** présente les mesures, trames et lectures conservées. Le **Moniteur APRS** affiche toutes les trames enregistrées ou seulement les trames WX, avec la raison de leur admissibilité.

Open-Meteo est interrogé toutes les 15 minutes. Son API gratuite est réservée à un usage non commercial et les données nécessitent une attribution CC BY 4.0. Le panneau indique la source. Si ces conditions ne conviennent pas à votre usage, désactivez Open-Meteo et configurez un autre fournisseur lorsqu’un adaptateur sera disponible.

APRS se connecte à un point TCP KISS externe de Dire Wolf, généralement `127.0.0.1:8001`. Configurez Dire Wolf pour recevoir l’audio radio et transmettre les trames KISS à UltimateWX. Le rayon WX par défaut est de 10 km. Les trames sans position reconnue, contenant des champs météo invalides, hors rayon ou reçues depuis plus de deux heures restent visibles dans le moniteur, mais ne sont pas utilisées comme données météo.

UltimateWX reçoit les trames APRS d’un TNC externe via TCP KISS ; aucune entrée audio n’est donc nécessaire ici. Choisissez la sortie de l’annonce et réglez son volume logiciel de 0 % à 100 %. La lecture nécessite ALSA (`aplay`) et une commande Piper locale.

L’éditeur d’annonce enregistre les blocs ordonnés en JSON. Chaque bloc peut être activé, associé à un type et à une source, puis recevoir des champs ou du texte. Les horaires par défaut sont désactivés. Un bloc `forecast` utilise la prévision Open-Meteo à quatre heures ; `local-estimate` est clairement présenté comme une estimation et ignoré sans mesures de pression récentes.

## Capteurs

Tous les capteurs utilisent les mêmes notions : nom du périphérique, horodatage, paramètre, valeur numérique et unité. MQTT attend un objet JSON par message, par exemple :

```json
{"device":"roof-esp32","timestamp":"2026-10-18T10:00:00Z","temperature_c":12.5,"humidity_percent":72,"pressure_hpa":1008.2}
```

Configurez un broker tel que `tcp://homeassistant.local:1883` et un filtre de sujet comme `ultimatewx/+/weather`. Un firmware ESP32 peut publier cet objet en Wi-Fi. Utilisez un compte dédié au broker et ne placez pas les identifiants dans des journaux publics.

L’intégration Home Assistant utilise l’API REST et un jeton d’accès. Renseignez l’URL de base et le jeton. Les entités de capteur dotées de classes reconnues (température, humidité, pression, vitesse du vent ou précipitations) sont lues périodiquement.

Pour USB/série, sélectionnez un périphérique Linux comme `/dev/ttyUSB0`, choisissez le débit et envoyez un objet JSON terminé par une nouvelle ligne pour chaque mesure. Les noms de champs sont ceux de l’exemple MQTT.

Pour un BME280 directement connecté au Raspberry Pi, activez le profil I²C, indiquez `/dev/i2c-1` et l’adresse 118 (`0x76`) ou 119 (`0x77`), puis reliez le capteur aux broches I²C et à la tension logique appropriée. L’application conserve la pression de station et calcule séparément la pression ramenée au niveau de la mer selon l’altitude configurée. Vérifiez l’adresse et le câblage avant d’activer le profil.

## FAQ et dépannage

**Le panneau ne s’ouvre pas.** Vérifiez le service et l’accès au port TCP 8080 sur le réseau local. Avec systemd : `systemctl status ultimatewx`; avec OpenRC : `rc-service ultimatewx status`.

**Aucune donnée APRS n’arrive.** Vérifiez l’audio du récepteur dans Dire Wolf, l’hôte et le port KISS TCP, ainsi que les trames AX.25 UI du TNC. UltimateWX consomme le flux KISS obtenu et ne capture pas directement l’audio du récepteur.

**Une trame proche n’est pas utilisée.** Vérifiez sa position, ses champs météo, l’heure de réception et sa distance dans le moniteur. N’augmentez le rayon que si cela correspond à la couverture voulue.

**La voix TTS est indisponible.** Dans l’onglet voix, choisissez une langue et chargez le catalogue lorsque l’accès Internet est disponible. L’application ne présente que les licences reconnues et vérifie les fichiers du modèle. Piper doit être installé et disponible sous la commande configurée ; les voix téléchargées restent locales.

**Que signifie l’estimation hors ligne ?** C’est un indicateur simple de tendance de pression, pas une prévision météorologique. Il nécessite au moins deux mesures récentes de pression de station espacées d’une heure minimum, dans les cinq dernières heures. L’annonce la nomme comme estimation locale.

**Comment mettre à jour ?** Choisissez **Rechercher des mises à jour**, consultez les informations de version, puis **Installer et redémarrer**. L’application vérifie le paquet avec le manifeste SHA-256. Le compte de service doit pouvoir écrire dans le dossier du programme ; l’installateur officiel configure ces droits.

**Où se trouvent les sauvegardes ?** Sauvegardez `/var/lib/ultimatewx`, notamment `config.json`, l’historique JSONL et les voix téléchargées. Une sauvegarde peut contenir des identifiants secrets.
