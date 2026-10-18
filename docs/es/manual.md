---
lang: es
title: Manual de usuario de UltimateWX
---

# Manual de usuario de UltimateWX

UltimateWX es un locutor meteorológico local para Linux y estaciones de radioaficionado. Combina pronósticos en línea, tramas meteorológicas APRS cercanas y lecturas de sensores, y genera un anuncio con bloques configurables. La versión 1 no transmite por radio ni controla PTT.

## Instalación

El instalador de versiones admite Raspberry Pi OS, Ubuntu y Alpine Linux en ARMv7, ARM64 y x86-64. Cuando se publique el repositorio, ejecuta el comando del README con permisos de administrador, normalmente mediante `sudo`. El instalador detecta la distribución, la arquitectura y el gestor de servicios, descarga el archivo adecuado, comprueba su suma SHA-256, instala el servicio y activa el inicio automático. El propietario del repositorio del comando debe coincidir con el proyecto publicado en GitHub.

En la red local, abre `http://<ip-del-dispositivo>:8080`. En el primer inicio, complete primero el asistente de configuración. Después puede establecer una contraseña de al menos seis caracteres o continuar sin iniciar sesión. La configuración y el historial se guardan en `/var/lib/ultimatewx`. Incluye este directorio en las copias de seguridad: contiene el hash de la contraseña y credenciales de integraciones.

Para ejecutar la versión de desarrollo, instala Go 1.23 o posterior y ejecuta `go run ./cmd/ultimatewx --listen 127.0.0.1:8080`. Los datos de desarrollo se guardan en `./data`.

## Configuración

En el primer inicio, el asistente de configuración aparece antes de configurar el acceso al panel. Después puede establecer una contraseña o continuar sin iniciar sesión. Sin contraseña, cualquier persona que pueda acceder al panel puede utilizarlo; use este modo solo en una red local de confianza.

La página **Estado** muestra las fuentes configuradas, los dispositivos de audio, la última actualización meteorológica en línea y una vista previa del anuncio. En **Configuración** puedes definir ubicación, coordenadas, altitud, unidades, idioma, radio APRS, interfaces, horario y bloques del anuncio. **Estadísticas** presenta las mediciones, tramas y reproducciones conservadas. El **Monitor APRS** muestra todas las tramas guardadas o solo las WX e indica si cada una puede aportar datos meteorológicos.

Open-Meteo se consulta cada 15 minutos. Su API gratuita está destinada a uso no comercial y sus datos requieren atribución CC BY 4.0. El panel identifica la fuente. Si esas condiciones no se ajustan a tu uso, desactiva Open-Meteo y configura otro proveedor cuando haya un adaptador disponible.

APRS se conecta a un punto TCP KISS externo de Dire Wolf, normalmente `127.0.0.1:8001`. Configura Dire Wolf para recibir el audio de radio y enviar tramas KISS a UltimateWX. El radio WX predeterminado es de 10 km. Las tramas sin posición compatible, con campos meteorológicos inválidos, fuera del radio o recibidas hace más de dos horas siguen visibles en el monitor, pero no se usan como datos meteorológicos.

La entrada y la salida de audio se eligen por separado. La entrada corresponde al receptor configurado en Dire Wolf; `aplay` usa la salida para reproducir el anuncio. Para detectar dispositivos y reproducir audio se necesitan herramientas ALSA (`arecord`, `aplay`) y un comando Piper local. La selección de entrada no configura Dire Wolf automáticamente.

El editor del anuncio guarda bloques ordenados en JSON. Cada bloque puede activarse, asignarse a un tipo y una fuente, y recibir campos o texto. Los horarios predeterminados están desactivados. Un bloque `forecast` utiliza el pronóstico de Open-Meteo para dentro de cuatro horas; `local-estimate` se identifica claramente como estimación y se omite si faltan mediciones recientes de presión.

## Sensores

Todas las fuentes de sensores usan los mismos conceptos: nombre del dispositivo, fecha y hora, parámetro, valor numérico y unidad. MQTT espera un objeto JSON por mensaje, por ejemplo:

```json
{"device":"roof-esp32","timestamp":"2026-10-18T10:00:00Z","temperature_c":12.5,"humidity_percent":72,"pressure_hpa":1008.2}
```

Configura un broker como `tcp://homeassistant.local:1883` y un filtro de tema como `ultimatewx/+/weather`. El firmware de ESP32 puede publicar este objeto por Wi-Fi. Usa una cuenta específica para el broker y no incluyas credenciales en registros públicos.

La integración con Home Assistant usa su API REST y un token de acceso. Configura la URL base y el token. Se consultan periódicamente las entidades de sensor con clases reconocidas: temperatura, humedad, presión, velocidad del viento o precipitación.

Para USB/serie, selecciona un dispositivo Linux como `/dev/ttyUSB0`, configura la velocidad y envía un objeto JSON terminado en nueva línea por cada muestra. Se aceptan los mismos nombres de campo que en el ejemplo MQTT.

Para conectar directamente un BME280 a Raspberry Pi, activa el perfil I²C, indica `/dev/i2c-1` y la dirección 118 (`0x76`) o 119 (`0x77`), y conecta el sensor a los pines I²C con el voltaje lógico correcto. La aplicación guarda la presión de estación y calcula por separado la presión al nivel del mar según la altitud configurada. Comprueba la dirección y el cableado antes de activar el perfil.

## Preguntas frecuentes y solución de problemas

**No se abre el panel.** Comprueba que el servicio funciona y que el puerto TCP 8080 está accesible en la red local. Con systemd: `systemctl status ultimatewx`; con OpenRC: `rc-service ultimatewx status`.

**No llegan datos APRS.** Comprueba el audio del receptor en Dire Wolf, el host y puerto KISS TCP, y que el TNC entregue tramas AX.25 UI. La entrada de audio seleccionada sirve actualmente como referencia para configurar el receptor; UltimateWX consume el flujo KISS resultante.

**No se utiliza una trama cercana.** Revisa su posición, campos meteorológicos, hora de recepción y distancia en el monitor. Aumenta el radio solo si coincide con la cobertura prevista.

**La voz TTS no está disponible.** En la pestaña de voz, elige un idioma y carga el catálogo cuando haya Internet. La aplicación solo muestra licencias reconocidas y verifica los archivos del modelo. Piper debe estar instalado y disponible con el comando configurado; las voces descargadas permanecen en el dispositivo.

**¿Qué significa la estimación sin conexión?** Es un indicador sencillo de tendencia de presión, no un pronóstico meteorológico. Necesita al menos dos mediciones recientes de presión de estación separadas por una hora o más, dentro de las últimas cinco horas. El anuncio la denomina estimación local.

**¿Cómo se actualiza?** Pulsa **Buscar actualizaciones**, revisa la información de la versión y selecciona **Instalar actualización y reiniciar**. La aplicación verifica el paquete con el manifiesto SHA-256. La cuenta del servicio necesita permiso de escritura en la carpeta del programa; el instalador oficial configura esos permisos.

**¿Dónde están las copias de seguridad?** Guarda `/var/lib/ultimatewx`, incluidos `config.json`, el historial JSONL y las voces descargadas. La copia puede contener credenciales.
