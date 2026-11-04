# API e integraciones

BombVault tiene una pequeña API HTTP para scripts, paneles y domótica. Lee lo mismo que muestra el resumen y puede iniciar una copia. Todo lo demás, como las restauraciones, el borrado de copias y los ajustes, se queda en la interfaz web.

## Tokens {#tokens}

Cada petición necesita un token de API, incluso cuando no hay contraseña de acceso. Crea uno en **Ajustes, Integraciones, Tokens de API**:

1. Escribe un nombre que diga dónde se usa el token, por ejemplo "Home Assistant" o "Uptime Kuma".
2. Activa **Permitir iniciar copias** si el token debe poder iniciar copias. Sin eso el token solo puede leer.
3. Haz clic en **Crear token**. El token se muestra una sola vez. BombVault guarda únicamente una huella suya, así que cópialo ahora.

Envía el token en una cabecera, bien `Authorization: Bearer <token>` o bien `X-API-Key: <token>`. Un token empieza por `bvapi_`. Solo abre la API: una clave de MCP no sirve aquí, y un token no sirve para MCP.

Cada token tiene un mosaico con su nombre, si puede iniciar copias, los últimos cuatro caracteres, cuándo se usó por última vez y desde dónde, y sus llamadas de hoy. En el mosaico puedes renombrarlo, cambiar lo que puede hacer, sustituirlo o revocarlo. **Registro** muestra las copias que inició y sus llamadas recientes. Restaurar la configuración de BombVault desde una copia revoca todos los tokens, porque la copia puede contener tokens que revocaste más tarde.

Sin contraseña de acceso, cualquiera que pueda abrir la interfaz web también puede crear un token. Si abres BombVault con un nombre de aspecto público y no hay contraseña, no se pueden crear tokens desde esa dirección, la misma regla que para las [claves de MCP](mcp.md#switch-on).

## Endpoints {#endpoints}

| Ruta | Qué devuelve o hace | Token |
|---|---|---|
| `GET /api/v1/health` | Versión, nombre de la instancia, si hay una copia en curso y qué puede hacer este token | lectura |
| `GET /api/v1/status` | Estado de protección por sección: última copia correcta, intervalo esperado, comprobaciones, próximas ejecuciones programadas | lectura |
| `GET /api/v1/activity` | Lo que se está ejecutando ahora, con fase y porcentaje | lectura |
| `GET /api/v1/items` | Cada elemento protegido con su calendario, lo que detiene una copia y su última copia. `?domain=` para una sección | lectura |
| `GET /api/v1/runs` | Historial de ejecuciones, las más recientes primero. Filtros `limit`, `domain`, `item`, `status`, `kind`, `since` | lectura |
| `GET /api/v1/anomalies` | Anomalías con un resumen de lo que está abierto. Filtros `state`, `severity`, `domain`, `limit` | lectura |
| `GET /api/v1/anomalies/{id}` | Una anomalía | lectura |
| `GET /api/v1/storage/{domain}` | Historial de tamaño, crecimiento por semana y espacio libre de cada repositorio de una sección | lectura |
| `POST /api/v1/backups` | Copia un elemento (`{"domain":"containers","item":"plex"}`) o una sección entera (`{"domain":"vms"}`) | inicio |
| `POST /api/v1/backups/everything` | Ejecuta la pasada de Copiar todo ahora | inicio |
| `POST /api/v1/runs/{id}/cancel` | Cancela una copia en curso que inició este token | inicio |

Las secciones son `containers`, `vms`, `files`, `zfs`, `flash` y `config`. Las horas son segundos unix. Las respuestas son las mismas que las de las [herramientas MCP](mcp.md#tools) con los mismos nombres, así que ambas van a la par.

Una copia iniciada es la misma copia que inicia la interfaz web: un contenedor en ejecución se detiene hasta que termina su copia. La petición vuelve enseguida, y `/api/v1/activity` y `/api/v1/runs` muestran cómo va.

## Ejemplos {#examples}

```sh
# How are the backups doing?
curl -s -H "Authorization: Bearer $BOMBVAULT_TOKEN" https://tower:3443/api/v1/status

# Back up one container now.
curl -s -X POST -H "Authorization: Bearer $BOMBVAULT_TOKEN" \
  -H "Content-Type: application/json" -d '{"domain":"containers","item":"plex"}' \
  https://tower:3443/api/v1/backups
```

Con el certificado autofirmado de BombVault, añade `--cacert bombvault-cert.pem` (el archivo que te da **Descargar certificado** en la tarjeta de MCP) o `-k` en una red de confianza.

## Errores y límites {#errors}

Un error vuelve como `{"error": {"code": "...", "message": "..."}}` con un estado acorde:

| Estado | Códigos | Significado |
|---|---|---|
| 400 | `invalid_argument`, `ambiguous` | Falta un argumento o es incorrecto |
| 401 | `no_token`, `invalid_token` | No hay token, o no es uno activo |
| 403 | `not_permitted`, `forbidden_origin` | El token solo puede leer, la ejecución no la inició él, o la petición vino de una página de otro origen |
| 404 | `not_found` | No existe ese elemento, ejecución o anomalía |
| 409 | `busy`, `domain_off`, `nothing_to_back_up`, `not_running` | Hay otra cosa en marcha, la sección está desactivada, o no hay nada que hacer |
| 429 | `throttled`, `rate_limited`, `cooldown`, `retention_guard` | Un límite retiene la petición. `Retry-After` dice cuándo volver a intentarlo |

Los inicios siguen los mismos límites que los [inicios por MCP](mcp.md#starting-backups): 12 por hora y token, 15 minutos entre dos inicios del mismo elemento, como mucho 4 inicios de un elemento en 24 horas y la protección de retención. Los tres últimos cuentan juntos los inicios por MCP, por la API y por Home Assistant. Un token puede hacer 120 peticiones por minuto. Cinco intentos fallidos desde una dirección la bloquean durante un minuto.

## OpenAPI {#openapi}

BombVault ofrece una descripción de estas rutas en `/api/v1/openapi.json` (OpenAPI 3.1). No necesita token. Cárgala en Swagger UI, Postman o un generador de código.

## Home Assistant {#home-assistant}

BombVault puede aparecer en Home Assistant como dispositivo, mediante el descubrimiento MQTT. Home Assistant necesita su integración MQTT y un broker, por ejemplo el complemento Mosquitto. No interviene ningún componente personalizado.

1. En BombVault, abre **Ajustes, Integraciones** y activa **Home Assistant** en la tarjeta **Servicios**. Debajo aparecen los campos del broker.
2. Introduce la dirección y el puerto del broker, y su nombre de usuario y contraseña si los pide. Activa **Usar TLS** si el broker acepta TLS, normalmente en el puerto 8883. Su certificado tiene que ser válido para la dirección que introdujiste. Si cambias la dirección, el puerto o el nombre de usuario, vuelve a introducir la contraseña: BombVault no pasa la guardada a otro broker o usuario.
3. Haz clic en **Guardar**. La fila muestra cuándo está activa la conexión.

El dispositivo se llama BombVault, o BombVault con el nombre de la instancia entre paréntesis, y tiene estas entidades:

| Entidad | Qué muestra |
|---|---|
| Status | `ok`, `warning`, `failed` u `off`, la peor de las secciones activadas. Una sección que espera su primera copia solo cuenta cuando esa copia ya tocaba, y solo una sección al día da el estado `ok` |
| Running job | Lo que se ejecuta ahora, o `idle` |
| Open anomalies | Cuántas anomalías están abiertas |
| Next scheduled backup | Cuándo empieza la próxima copia programada |
| *Domain* last backup | Cuándo se ejecutó la última copia correcta de la sección |
| *Domain* last result | Cómo terminó su última copia |
| *Domain* repository free space | Espacio libre donde está su repositorio primario, si BombVault puede leerlo |
| Back up *domain* | Un botón que copia la sección entera |

Cada sección activada tiene sus propias entidades, y una sección que desactives las pierde. Los botones aparecen cuando activas **Los botones inician copias**. Una instalación nueva lo tiene desactivado. Siguen los mismos límites que los [inicios por la API](#errors). Además, BombVault atiende una pulsación por sección a la vez y no más de seis por minuto, e ignora una pulsación que el broker guardó como mensaje retenido. Cualquiera que pueda publicar en el broker puede pulsarlos, así que ponle una contraseña al broker.

BombVault lee su estado cada 15 segundos y lo publica cuando algo ha cambiado, como JSON bajo `<prefix>/<node>/state`. El prefijo es `bombvault` salvo que lo cambies, y el nodo es un identificador corto que BombVault elige una vez. Los mensajes de descubrimiento van al prefijo predeterminado de Home Assistant, `homeassistant`. Ambos son retenidos. Un último deseo marca el dispositivo como no disponible si BombVault se detiene sin avisar. Desactivar el enlace elimina el dispositivo y sus entidades de Home Assistant.

### Ejemplo: una tarjeta y una alerta {#home-assistant-example}

Home Assistant forma los identificadores de entidad a partir de los nombres, así que el dispositivo BombVault recibe identificadores como `sensor.bombvault_status`. Con un nombre de instancia entre paréntesis los identificadores también lo llevan, así que compruébalos antes en **Developer Tools, States**. Una tarjeta para el panel:

```yaml
type: entities
title: Backups
entities:
  - sensor.bombvault_status
  - sensor.bombvault_running_job
  - sensor.bombvault_open_anomalies
  - sensor.bombvault_next_scheduled_backup
```

Una automatización que avisa a tu teléfono cuando el estado pasa a `failed`:

```yaml
alias: BombVault backup failed
trigger:
  - platform: state
    entity_id: sensor.bombvault_status
    to: failed
action:
  - service: notify.mobile_app_my_phone
    data:
      message: A BombVault backup failed. Open Needs you on the overview.
```

## Prometheus {#metrics}

**Exponer /metrics** en la tarjeta **Servicios** de **Ajustes, Integraciones** sirve el estado de las copias, los tamaños y las marcas de tiempo para Prometheus, Grafana o Uptime Kuma, en `/metrics` en el puerto de la interfaz web. Está desactivado de forma predeterminada. **Token de scrape (opcional)** hace que el recolector envíe un token bearer. Las etiquetas no llevan secretos ni rutas.

## Widget de panel {#dashboard-widget}

La tarjeta **Widget de panel** de **Ajustes, Integraciones** genera un token de solo lectura y te da una dirección para cualquier panel que muestre un iframe, como Homepage, Organizr o Heimdall. La dirección abre una página pequeña en `/widget` con solo el registro de actividad en vivo. El token da acceso a ese registro y a nada más, y **Desactivar** lo revoca al instante. La página incrustada solo está en inglés.

## Encontrar BombVault en la red {#mdns}

BombVault anuncia su interfaz web en la red local por mDNS, el protocolo que hay detrás de Bonjour y Avahi. Entonces un navegador llega a ella como `https://bombvault.local:3443`, o `http://bombvault.local:3000` con `HTTP_ONLY`, y los exploradores de servicios la listan como un servicio web con el subtipo `_bombvault`. Sus registros TXT llevan la versión y la ruta. El interruptor es **Encontrar en la red** en la tarjeta **Servicios** de **Ajustes, Integraciones** y viene activado de forma predeterminada. Si otro dispositivo ya usa el nombre, BombVault toma `bombvault-2.local` y así sucesivamente, y la tarjeta muestra la dirección que obtuvo. Cuando BombVault se detiene o desactivas el anuncio, avisa a la red, así que los navegadores quitan la entrada al instante.

Que el anuncio llegue a tu red depende de cómo esté conectado el contenedor:

- **bridge**, el valor predeterminado en la plantilla de Unraid: el anuncio se queda dentro de la red de Docker y nadie en la LAN lo ve. Abre BombVault por la dirección del host como antes.
- **br0** u otra red macvlan o ipvlan: el contenedor tiene su propia dirección en la LAN, y el anuncio llega a ella.
- **host**: el anuncio sale por las interfaces del host, junto al propio de Unraid. Se salta los puentes de Docker y de libvirt.

Solo se anuncian direcciones IPv4.
