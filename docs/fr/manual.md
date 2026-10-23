---
lang: fr
title: Guide utilisateur UltimateWX
---

# Guide utilisateur UltimateWX

UltimateWX est un annonceur météo local pour Linux et les stations radioamateur. Il combine les prévisions en ligne, les trames météo APRS proches et les mesures de capteurs, puis compose une annonce à partir de blocs configurables. La version 1 n’émet pas par radio et ne commande pas le PTT.

## Installation

L’installateur de version prend en charge Raspberry Pi OS, Ubuntu et Alpine Linux en ARMv7, ARM64 et x86-64. Versions minimales prises en charge : Raspberry Pi OS/Debian 12, Ubuntu 24.04 LTS et Alpine Linux 3.22. Après la publication du dépôt, lancez la commande indiquée dans le README avec les droits administrateur, généralement via `sudo`. L’installateur détecte la distribution, l’architecture et le système de services, télécharge l’archive adaptée, vérifie sa somme SHA-256, installe le service et active le démarrage automatique. Sur Raspberry Pi OS/Debian et Ubuntu en x86-64/ARM64, il installe Piper 1.8.0 dans un environnement isolé sous `/opt/ultimatewx/piper`. Sur ARMv7, il installe une version expérimentale Piper 1.2.0 dans le même dossier. Alpine ne reçoit pas d’installation automatique ; un moteur compatible avec musl peut être configuré manuellement dans les paramètres avancés.

Au premier démarrage, saisissez d’abord le jeton de configuration à usage unique, puis définissez un mot de passe ou continuez sans connexion. L’assistant de configuration de la station apparaît ensuite. Sans mot de passe, toute personne pouvant accéder au panneau peut l’utiliser ; réservez ce mode à un réseau local de confiance.

Pour un lancement de développement, installez Go 1.23 ou une version ultérieure et exécutez `go run ./cmd/ultimatewx --listen 127.0.0.1:8080`. Les données de développement sont placées dans `./data`.

## Langue de l’interface

Le panneau comprend l’anglais, le polonais, l’allemand, le français, l’espagnol et l’italien. Vous pouvez ajouter une autre langue sans reconstruire le programme : placez un fichier JSON dans le dossier `locales` du répertoire de données de l’application (par défaut : `/var/lib/ultimatewx/locales`), puis rechargez le panneau. Un exemple et le format du fichier sont disponibles dans [`examples/locales`](../../examples/locales). Les libellés non traduits restent affichés en anglais. Ce fichier ne modifie que la langue de l’interface ; les annonces vocales conservent la langue de la voix TTS sélectionnée.

## Configuration

Au premier démarrage, saisissez d’abord le jeton de configuration à usage unique, puis définissez un mot de passe ou continuez sans connexion. L’assistant de configuration de la station apparaît ensuite. Sans mot de passe, toute personne pouvant accéder au panneau peut l’utiliser ; réservez ce mode à un réseau local de confiance.

L’onglet **Configuration → Application** permet de rechercher et d’installer les mises à jour, ainsi que d’exporter et d’importer une sauvegarde de la configuration. Celle-ci contient les paramètres d’intégration, y compris les mots de passe MQTT et les jetons Home Assistant, mais pas le mot de passe du panneau ni l’historique des mesures. Conservez le fichier de sauvegarde en lieu sûr.


La page **État** affiche les sources configurées, les périphériques audio, la dernière mise à jour météo en ligne et l’aperçu du message. Dans **Configuration**, réglez l’emplacement, les coordonnées, l’altitude, les unités, la langue, le rayon APRS, les interfaces, les horaires et les blocs d’annonce. **Statistiques** présente les mesures, trames et lectures conservées. Le **Moniteur APRS** affiche toutes les trames enregistrées ou seulement les trames WX, avec la raison de leur admissibilité. La période du graphique se choisit séparément ; « Historique (jours) » règle la conservation des mesures, trames APRS et lectures. Les trames APRS sont aussi limitées à 10 000 entrées. Pour les blocs d’annonce en source automatique, définissez un ordre de priorité par paramètre ; cet ordre détermine les mesures disponibles à privilégier. Le profil aviation affiche °C, nœuds (kt), hPa et mm ; le profil impérial utilise °F, mph, hPa et pouces. Pour les blocs météo, définissez jusqu’à cinq sources de secours dans l’ordre après la source principale. Chaque bloc météo peut aussi utiliser les unités globales ou ses propres unités, avec un format vocal standard ou concis. Indiquez un fuseau IANA, par exemple `Europe/Paris` ; il sert au programme, à la date prononcée et à la validité des alertes.

Open-Meteo est interrogé toutes les 15 minutes. Son API gratuite est réservée à un usage non commercial et les données nécessitent une attribution CC BY 4.0. Le panneau indique la source. Si ces conditions ne conviennent pas à votre usage, désactivez Open-Meteo et configurez un autre fournisseur lorsqu’un adaptateur sera disponible.

APRS se connecte à un point TCP KISS externe de Dire Wolf, généralement `127.0.0.1:8001`. Configurez Dire Wolf pour recevoir l’audio radio et transmettre les trames KISS à UltimateWX. Le rayon WX par défaut est de 10 km. Les trames sans position reconnue, contenant des champs météo invalides, hors rayon ou reçues depuis plus de deux heures restent visibles dans le moniteur, mais ne sont pas utilisées comme données météo. Les trames d’une station bloquée restent visibles dans le moniteur. Le bouton de la ligne permet de bloquer ou débloquer son indicatif ; le filtre s’applique aux mesures actuelles et futures de cette station sans supprimer l’historique. L’analyseur lit la direction et la vitesse dans le champ suivant le symbole météo ; le champ `s` reste la neige, et les valeurs hors plage sont ignorées. Le moniteur permet aussi de bloquer une trame WX précise. Les trames correspondantes restent visibles, mais leurs mesures sont ignorées ; le même bouton permet de les débloquer. Les mesures identifiées comme provenant de stations ou de trames bloquées sont exclues des relevés actuels et des statistiques ; les données enregistrées ne sont pas supprimées.

L’onglet Audio et voix répertorie les périphériques de capture ALSA et teste l’entrée choisie en capturant un échantillon d’une seconde. Configurez le même périphérique d’entrée dans Dire Wolf/TNC, qui décode toujours l’audio radio et transmet les trames via KISS. Choisissez la sortie de l’annonce et réglez son volume logiciel de 0 % à 100 %. La lecture nécessite ALSA (`aplay`) et une commande Piper locale.

L’éditeur d’annonce enregistre les blocs ordonnés en JSON. Chaque bloc peut être activé, associé à un type et à une source, puis recevoir des champs ou du texte. Les horaires par défaut sont désactivés. Un bloc `forecast` utilise la prévision Open-Meteo à quatre heures ; `local-estimate` est clairement présenté comme une estimation et ignoré sans mesures de pression récentes.

## Capteurs

Tous les capteurs utilisent les mêmes notions : nom du périphérique, horodatage, paramètre, valeur numérique et unité. MQTT attend un objet JSON par message, par exemple :

```json
{"device":"roof-esp32","timestamp":"2026-10-18T10:00:00Z","temperature_c":12.5,"humidity_percent":72,"pressure_hpa":1008.2}
```

Configurez un broker tel que `tcp://homeassistant.local:1883` ou une connexion TLS telle que `mqtts://broker.example.org` (port par défaut 8883), ainsi qu’un filtre de sujet comme `ultimatewx/+/weather`. Un firmware ESP32 peut publier cet objet en Wi-Fi. Utilisez un compte dédié au broker et ne placez pas les identifiants dans des journaux publics.

Dans **Configuration → Capteurs**, vous pouvez décocher, pour chaque appareil détecté, les mesures qu’il fournit incorrectement. Elles ne sont utilisées ni pour la météo actuelle ni pour l’estimation locale de pression, mais l’historique est conservé. Cochez **À privilégier dans les annonces** pour choisir cet appareil en premier pour le paramètre lorsqu’il fournit une mesure récente. Les priorités globales des intégrations et les sources de secours des blocs restent actives ; sans mesure récente, la sélection habituelle des sources s’applique.

Les mesures des capteurs MQTT, USB/série, Home Assistant et BME280 sont rejetées si leur horodatage dépasse l’horloge de l’application de plus de cinq minutes ou si une valeur sort des plages prises en charge : température −100 à 70 °C, humidité 0–100 %, pression 300–1200 hPa, vitesse du vent 0–120 m/s, direction du vent facultative de 0–360° (champ JSON `wind_direction_deg`, nécessite un capteur de direction), précipitations 0–10 000 mm et altitude du capteur −500–9000 m.

L’intégration Home Assistant utilise l’API REST et un jeton d’accès. Renseignez l’URL de base et le jeton. Les entités de capteur dotées de classes reconnues (température, humidité, pression, vitesse et direction du vent (unité `°`) ou précipitations) sont lues périodiquement.

Pour USB/série, sélectionnez un périphérique Linux comme `/dev/ttyUSB0`, choisissez le débit et envoyez un objet JSON terminé par une nouvelle ligne pour chaque mesure. **Tester la connexion** vérifie l’accès au périphérique et lit ses paramètres avec `stty` ; cela ne valide ni la réception des mesures ni leur format JSON. Les noms de champs sont ceux de l’exemple MQTT.

Pour un BME280 directement connecté au Raspberry Pi, activez le profil I²C, indiquez `/dev/i2c-1` et l’adresse 118 (`0x76`) ou 119 (`0x77`), puis reliez le capteur aux broches I²C et à la tension logique appropriée. L’application conserve la pression de station et calcule séparément la pression ramenée au niveau de la mer selon l’altitude configurée. Vérifiez l’adresse et le câblage avant d’activer le profil.

## FAQ et dépannage

**Le panneau ne s’ouvre pas.** Vérifiez le service et l’accès au port TCP 8080 sur le réseau local. Avec systemd : `systemctl status ultimatewx`; avec OpenRC : `rc-service ultimatewx status`.

**Aucune donnée APRS n’arrive.** Vérifiez l’audio du récepteur dans Dire Wolf, l’hôte et le port KISS TCP, ainsi que les trames AX.25 UI du TNC. UltimateWX consomme le flux KISS obtenu ; le test audio ne capture qu’un court échantillon et ne remplace pas le décodage continu par Dire Wolf/TNC.

**Une trame proche n’est pas utilisée.** Vérifiez sa position, ses champs météo, l’heure de réception et sa distance dans le moniteur. N’augmentez le rayon que si cela correspond à la couverture voulue.

**La voix TTS est indisponible.** Dans l’onglet voix et annonces, choisissez une langue, chargez le catalogue puis téléchargez une voix. Le catalogue nécessite Internet ; les modèles téléchargés fonctionnent localement. L’installateur système place Piper sous `/opt/ultimatewx/piper` sur Debian/Raspberry Pi OS et Ubuntu x86-64/ARM64, et installe une version expérimentale sur ARMv7. Si l’installation échoue ou si vous utilisez Alpine, installez un moteur compatible et indiquez son chemin dans les paramètres audio avancés repliés.

**Que signifie l’estimation hors ligne ?** C’est un indicateur simple de tendance de pression, pas une prévision météorologique. Il nécessite au moins deux mesures récentes de pression de station espacées d’une heure minimum, dans les cinq dernières heures. L’annonce la nomme comme estimation locale.

**Comment mettre à jour ?** Choisissez **Rechercher des mises à jour**, consultez les informations de version, puis **Installer et redémarrer**. L’application vérifie le paquet avec le manifeste SHA-256. Le compte de service doit pouvoir écrire dans le dossier du programme ; l’installateur officiel configure ces droits.

**Où se trouvent les sauvegardes ?** Sauvegardez `/var/lib/ultimatewx`, notamment `config.json`, l’historique JSONL et les voix téléchargées. Une sauvegarde peut contenir des identifiants secrets.
