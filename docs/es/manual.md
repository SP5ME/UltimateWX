---
lang: es
title: Manual de usuario de UltimateWX
---

# Manual de usuario de UltimateWX

UltimateWX es un locutor meteorológico local para Linux y estaciones de radioaficionado. Combina pronósticos en línea, tramas meteorológicas APRS cercanas y lecturas de sensores, y genera un anuncio con bloques configurables. La versión 1 no transmite por radio ni controla PTT.

## Instalación

El instalador de versiones admite Raspberry Pi OS, Ubuntu y Alpine Linux en ARMv7, ARM64 y x86-64. Versiones mínimas compatibles: Raspberry Pi OS/Debian 12, Ubuntu 24.04 LTS y Alpine Linux 3.22. Cuando se publique el repositorio, ejecuta el comando del README con permisos de administrador, normalmente mediante `sudo`. El instalador detecta la distribución, la arquitectura y el gestor de servicios, descarga el archivo adecuado, comprueba su suma SHA-256, instala el servicio y activa el inicio automático. En Raspberry Pi OS/Debian y Ubuntu para x86-64/ARM64 instala Piper 1.8.0 en un entorno aislado bajo `/opt/ultimatewx/piper`. En ARMv7 instala una versión experimental de Piper 1.2.0 en el mismo directorio. Alpine no recibe una instalación automática; puedes configurar manualmente un motor compatible con musl en los ajustes avanzados.

En el primer inicio, introduce primero el token de configuración de un solo uso y después establece una contraseña o continúa sin iniciar sesión. A continuación aparecerá el asistente de configuración de la estación. Sin contraseña, cualquier persona que pueda acceder al panel puede utilizarlo; use este modo solo en una red local de confianza.

Para ejecutar la versión de desarrollo, instala Go 1.23 o posterior y ejecuta `go run ./cmd/ultimatewx --listen 127.0.0.1:8080`. Los datos de desarrollo se guardan en `./data`.

## Idioma de la interfaz

El panel incluye inglés, polaco, alemán, francés, español e italiano. Puede añadir otro idioma sin volver a compilar el programa: coloque un archivo JSON en el directorio `locales` dentro del directorio de datos de la aplicación (predeterminado: `/var/lib/ultimatewx/locales`) y vuelva a cargar el panel. Hay un ejemplo y el formato del archivo en [`examples/locales`](../../examples/locales). Las etiquetas sin traducir se muestran en inglés. El archivo solo cambia el idioma de la interfaz; los anuncios de voz siguen usando el idioma de la voz TTS seleccionada.

## Configuración

En el primer inicio, introduce primero el token de configuración de un solo uso y después establece una contraseña o continúa sin iniciar sesión. A continuación aparecerá el asistente de configuración de la estación. Sin contraseña, cualquier persona que pueda acceder al panel puede utilizarlo; use este modo solo en una red local de confianza.

La pestaña **Configuración → Aplicación** permite buscar e instalar actualizaciones, además de exportar e importar copias de la configuración. La copia incluye los ajustes de integración, incluidas las contraseñas MQTT y los tokens de Home Assistant, pero no la contraseña del panel ni el historial de mediciones. Guarde el archivo de copia en un lugar seguro.


La página **Estado** muestra las fuentes configuradas, los dispositivos de audio, la última actualización meteorológica en línea y una vista previa del anuncio. En **Configuración** puedes definir ubicación, coordenadas, altitud, unidades, idioma, radio APRS, interfaces, horario y bloques del anuncio. **Estadísticas** presenta las mediciones, tramas y reproducciones conservadas. El **Monitor APRS** muestra todas las tramas guardadas o solo las WX e indica si cada una puede aportar datos meteorológicos. El intervalo del gráfico se elige por separado; «Historial (días)» controla la conservación de mediciones, tramas APRS y reproducciones. Las tramas APRS también tienen un límite de 10 000 entradas. En los bloques del anuncio con fuente automática, puede definir el orden de prioridad para cada parámetro; este orden determina qué mediciones disponibles se prefieren. El perfil de aviación muestra °C, nudos (kt), hPa y mm; el perfil imperial usa °F, mph, hPa y pulgadas. En los bloques meteorológicos, puede definir hasta cinco fuentes de respaldo ordenadas después de la fuente principal. Cada bloque meteorológico también puede usar unidades globales o propias y un formato hablado estándar o breve. Indica una zona IANA, por ejemplo `Europe/Madrid`; se usa para el horario, la fecha hablada y la validez de los avisos.

Open-Meteo se consulta cada 15 minutos. Su API gratuita está destinada a uso no comercial y sus datos requieren atribución CC BY 4.0. El panel identifica la fuente. Si esas condiciones no se ajustan a tu uso, desactiva Open-Meteo y configura otro proveedor cuando haya un adaptador disponible.

APRS se conecta a un punto TCP KISS externo de Dire Wolf, normalmente `127.0.0.1:8001`. Configura Dire Wolf para recibir el audio de radio y enviar tramas KISS a UltimateWX. El radio WX predeterminado es de 10 km. Las tramas sin posición compatible, con campos meteorológicos inválidos, fuera del radio o recibidas hace más de dos horas siguen visibles en el monitor, pero no se usan como datos meteorológicos. Los paquetes de una estación bloqueada siguen visibles en el monitor. Use el botón de la fila para bloquear o desbloquear su indicativo; el filtro se aplica a las lecturas actuales y futuras de esa estación y no elimina el historial. El analizador lee la dirección y la velocidad del campo posterior al símbolo meteorológico; el campo `s` sigue indicando nieve y se omiten los valores fuera de rango. El monitor también permite bloquear una trama WX concreta. Las tramas coincidentes siguen visibles, pero se omiten sus lecturas; use el mismo botón para desbloquearlas. Las lecturas identificadas como procedentes de estaciones y tramas bloqueadas se excluyen de las lecturas actuales y las estadísticas; los datos guardados no se eliminan.

La pestaña Audio y voz muestra los dispositivos de captura ALSA y prueba la entrada seleccionada capturando una muestra de un segundo. Configure el mismo dispositivo de entrada en Dire Wolf/TNC, que sigue decodificando el audio de radio y reenviando tramas por KISS. Elija la salida del anuncio y ajuste su volumen por software entre el 0 % y el 100 %. La reproducción requiere ALSA (`aplay`) y un comando Piper local.

El editor del anuncio guarda bloques ordenados en JSON. Cada bloque puede activarse, asignarse a un tipo y una fuente, y recibir campos o texto. Los horarios predeterminados están desactivados. Un bloque `forecast` utiliza el pronóstico de Open-Meteo para dentro de cuatro horas; `local-estimate` se identifica claramente como estimación y se omite si faltan mediciones recientes de presión.

## Sensores

Todas las fuentes de sensores usan los mismos conceptos: nombre del dispositivo, fecha y hora, parámetro, valor numérico y unidad. MQTT espera un objeto JSON por mensaje, por ejemplo:

```json
{"device":"roof-esp32","timestamp":"2026-10-18T10:00:00Z","temperature_c":12.5,"humidity_percent":72,"pressure_hpa":1008.2}
```

Configura un broker como `tcp://homeassistant.local:1883` o una conexión TLS como `mqtts://broker.example.org` (puerto predeterminado 8883), y un filtro de tema como `ultimatewx/+/weather`. El firmware de ESP32 puede publicar este objeto por Wi-Fi. Usa una cuenta específica para el broker y no incluyas credenciales en registros públicos.

En **Configuración → Sensores**, puede desmarcar para cada dispositivo detectado las mediciones que proporciona incorrectamente. No se usarán para el tiempo actual ni para la estimación local de presión, pero se conservan en el historial. Marque **Preferir en los anuncios** para elegir primero ese dispositivo para el parámetro cuando tenga una lectura reciente. Siguen aplicándose las prioridades globales de las integraciones y las fuentes de respaldo de los bloques; si falta una lectura reciente, se utiliza la selección habitual.

Las lecturas de sensores MQTT, USB/serie, Home Assistant y BME280 se rechazan si su fecha supera en más de cinco minutos el reloj de la aplicación o si algún valor queda fuera de los rangos admitidos: temperatura de −100 a 70 °C, humedad de 0–100 %, presión de 300–1200 hPa, velocidad del viento de 0–120 m/s, dirección opcional del viento de 0–360° (campo JSON `wind_direction_deg`; requiere un sensor de dirección), precipitación de 0–10 000 mm y altitud del sensor de −500–9000 m.

La integración con Home Assistant usa su API REST y un token de acceso. Configura la URL base y el token. Se consultan periódicamente las entidades de sensor con clases reconocidas: temperatura, humedad, presión, velocidad y dirección del viento (unidad `°`) o precipitación.

Para USB/serie, selecciona un dispositivo Linux como `/dev/ttyUSB0`, configura la velocidad y envía un objeto JSON terminado en nueva línea por cada muestra. **Probar conexión** comprueba el acceso al dispositivo y lee sus ajustes con `stty`; no verifica la recepción de muestras ni su formato JSON. Se aceptan los mismos nombres de campo que en el ejemplo MQTT.

Para conectar directamente un BME280 a Raspberry Pi, activa el perfil I²C, indica `/dev/i2c-1` y la dirección 118 (`0x76`) o 119 (`0x77`), y conecta el sensor a los pines I²C con el voltaje lógico correcto. La aplicación guarda la presión de estación y calcula por separado la presión al nivel del mar según la altitud configurada. Comprueba la dirección y el cableado antes de activar el perfil.

## Preguntas frecuentes y solución de problemas

**No se abre el panel.** Comprueba que el servicio funciona y que el puerto TCP 8080 está accesible en la red local. Con systemd: `systemctl status ultimatewx`; con OpenRC: `rc-service ultimatewx status`.

**No llegan datos APRS.** Comprueba el audio del receptor en Dire Wolf, el host y puerto KISS TCP, y que el TNC entregue tramas AX.25 UI. UltimateWX consume el flujo KISS resultante; la prueba de audio solo captura una muestra breve y no sustituye la decodificación continua de Dire Wolf/TNC.

**No se utiliza una trama cercana.** Revisa su posición, campos meteorológicos, hora de recepción y distancia en el monitor. Aumenta el radio solo si coincide con la cobertura prevista.

**La voz TTS no está disponible.** En la sección Voz y anuncios, elige un idioma, carga el catálogo y descarga una voz. El catálogo requiere Internet; los modelos descargados funcionan localmente. El instalador del sistema coloca Piper en `/opt/ultimatewx/piper` en Debian/Raspberry Pi OS y Ubuntu x86-64/ARM64, e instala una versión experimental en ARMv7. Si falla la instalación o usas Alpine, instala un motor compatible e indica su ruta en los ajustes avanzados de audio plegados.

**¿Qué significa la estimación sin conexión?** Es un indicador sencillo de tendencia de presión, no un pronóstico meteorológico. Necesita al menos dos mediciones recientes de presión de estación separadas por una hora o más, dentro de las últimas cinco horas. El anuncio la denomina estimación local.

**¿Cómo se actualiza?** Pulsa **Buscar actualizaciones**, revisa la información de la versión y selecciona **Instalar actualización y reiniciar**. La aplicación verifica el paquete con el manifiesto SHA-256. La cuenta del servicio necesita permiso de escritura en la carpeta del programa; el instalador oficial configura esos permisos.

**¿Dónde están las copias de seguridad?** Guarda `/var/lib/ultimatewx`, incluidos `config.json`, el historial JSONL y las voces descargadas. La copia puede contener credenciales.
