# API e integraciones

BombVault tiene una pequeña API HTTP para scripts, paneles y domótica. Lee lo mismo que muestra el panel y puede iniciar una copia. Todo lo demás, como las restauraciones, el borrado de copias y los ajustes, se queda en la interfaz web.

## Tokens {#tokens}

Cada petición necesita un token de API, aunque no haya contraseña de acceso. Créalo en **Ajustes, Sistema, Tokens de API**:

1. Escribe un nombre que diga dónde se usa el token, por ejemplo «Home Assistant» o «Uptime Kuma».
2. Activa **Permitir iniciar copias** si el token debe poder iniciar copias. Sin eso solo puede leer.
3. Haz clic en **Crear token**. El token se muestra una sola vez. BombVault solo guarda una huella, así que cópialo ahora.

Envía el token en una cabecera, `Authorization: Bearer <token>` o `X-API-Key: <token>`. Un token empieza por `bvapi_`. Solo abre la API: una clave MCP no sirve aquí y un token no sirve para MCP.

Cada token tiene una ficha con su nombre, si puede iniciar copias, sus cuatro últimos caracteres, cuándo y desde dónde se usó por última vez y sus llamadas de hoy. En la ficha puedes renombrarlo, cambiar lo que puede hacer, sustituirlo o revocarlo. **Registro** muestra las copias que inició y sus últimas llamadas. Restaurar la configuración de BombVault desde una copia revoca todos los tokens, porque la copia puede contener tokens que revocaste después.

Sin contraseña de acceso, cualquiera que pueda abrir la interfaz web también puede crear un token. Si abres BombVault con un nombre que parece público y no hay contraseña, no se pueden crear tokens desde esa dirección, igual que con las [claves MCP](mcp.md#switch-on).

## Rutas {#endpoints}

| Ruta | Qué devuelve o hace | Token |
|---|---|---|
| `GET /api/v1/health` | Versión, nombre de la instancia, si hay una copia en marcha y qué puede hacer este token | lectura |
| `GET /api/v1/status` | Estado de protección por dominio: última copia correcta, intervalo esperado, comprobaciones, próximas ejecuciones | lectura |
| `GET /api/v1/activity` | Lo que está en marcha ahora, con fase y porcentaje | lectura |
| `GET /api/v1/items` | Cada elemento protegido con su programación, lo que detiene una copia y su última copia; `?domain=` para un dominio | lectura |
| `GET /api/v1/runs` | Historial de ejecuciones, las más recientes primero; filtros `limit`, `domain`, `item`, `status`, `kind`, `since` | lectura |
| `GET /api/v1/anomalies` | Anomalías con un resumen de lo abierto; filtros `state`, `severity`, `domain`, `limit` | lectura |
| `GET /api/v1/anomalies/{id}` | Una anomalía | lectura |
| `GET /api/v1/storage/{domain}` | Historial de tamaño, crecimiento semanal y espacio libre de cada repositorio de un dominio | lectura |
| `POST /api/v1/backups` | Copia un elemento (`{"domain":"containers","item":"plex"}`) o un dominio entero (`{"domain":"vms"}`) | inicio |
| `POST /api/v1/backups/everything` | Ejecuta Backup Everything | inicio |
| `POST /api/v1/runs/{id}/cancel` | Cancela una copia en marcha que inició este token | inicio |

Los dominios son `containers`, `vms`, `files`, `zfs`, `flash` y `config`. Las horas son segundos Unix. Las respuestas son las de las [herramientas MCP](mcp.md#tools) del mismo nombre, así ambas van a la par.

Una copia iniciada aquí es la misma que inicia la interfaz web: un contenedor en marcha se detiene hasta que termina su copia. La petición vuelve enseguida, y `/api/v1/activity` y `/api/v1/runs` muestran cómo va.

## Ejemplos {#examples}

```sh
# ¿Cómo van las copias?
curl -s -H "Authorization: Bearer $BOMBVAULT_TOKEN" https://tower:3443/api/v1/status

# Copiar un contenedor ahora.
curl -s -X POST -H "Authorization: Bearer $BOMBVAULT_TOKEN" \
  -H "Content-Type: application/json" -d '{"domain":"containers","item":"plex"}' \
  https://tower:3443/api/v1/backups
```

Con el certificado autofirmado de BombVault, añade `--cacert bombvault-cert.pem` (el archivo que da **Descargar certificado** en la tarjeta MCP) o `-k` en una red de confianza.

## Errores y límites {#errors}

Un error vuelve como `{"error": {"code": "...", "message": "..."}}` con el estado correspondiente:

| Estado | Códigos | Significado |
|---|---|---|
| 400 | `invalid_argument`, `ambiguous` | Falta un argumento o es incorrecto |
| 401 | `no_token`, `invalid_token` | Sin token, o no es uno activo |
| 403 | `not_permitted` | El token solo puede leer, o no inició esa ejecución |
| 404 | `not_found` | No existe ese elemento, ejecución o anomalía |
| 409 | `busy`, `domain_off`, `nothing_to_back_up`, `not_running` | Ya hay algo en marcha, el dominio está desactivado o no hay nada que hacer |
| 429 | `throttled`, `rate_limited`, `cooldown`, `retention_guard` | Un límite retiene la petición; `Retry-After` indica cuándo reintentar |

Los inicios siguen los mismos límites que los [inicios por MCP](mcp.md#starting-backups): 12 por hora y token, 15 minutos entre dos inicios del mismo elemento, como mucho 4 inicios de un elemento en 24 horas y la protección de retención. Los tres últimos cuentan juntos los inicios por MCP, por la API y desde Home Assistant. Un token puede hacer 120 peticiones por minuto. Cinco intentos fallidos desde una dirección la bloquean durante un minuto.

## OpenAPI {#openapi}

BombVault sirve una descripción de estas rutas en `/api/v1/openapi.json` (OpenAPI 3.1). No necesita token. Cárgala en Swagger UI, Postman o un generador de código.

## Home Assistant {#home-assistant}

BombVault puede aparecer en Home Assistant como un dispositivo, mediante el descubrimiento MQTT. Home Assistant necesita su integración MQTT y un broker, por ejemplo el complemento Mosquitto. No hace falta ningún componente propio.

1. En BombVault, abre **Ajustes, Sistema, Home Assistant**.
2. Escribe la dirección y el puerto del broker, y el usuario y la contraseña si los pide. Activa **Usar TLS** si el broker usa TLS, normalmente en el puerto 8883; su certificado tiene que ser válido para la dirección que escribiste.
3. Activa **Conectar con Home Assistant** y haz clic en **Guardar**. La tarjeta muestra cuándo está establecida la conexión.

El dispositivo se llama BombVault, o BombVault con el nombre de la instancia entre paréntesis, y tiene estas entidades:

| Entidad | Qué muestra |
|---|---|
| Status | `ok`, `warning`, `failed` u `off`, el peor de los dominios activados |
| Running job | Lo que está en marcha ahora, o `idle` |
| Open anomalies | Cuántas anomalías están abiertas |
| Next scheduled backup | Cuándo empieza la próxima copia programada |
| *Dominio* last backup | Cuándo fue la última copia correcta del dominio |
| *Dominio* last result | Cómo terminó su última copia |
| *Dominio* repository free space | El espacio libre donde está su repositorio principal, si BombVault puede leerlo |
| Back up *dominio* | Un botón que copia el dominio entero |

Los nombres de las entidades están en inglés, porque Home Assistant los toma tal como los envía BombVault. Cada dominio activado tiene sus propias entidades, y uno que desactives las pierde. Los botones siguen los mismos límites que los [inicios por la API](#errors). Cualquiera que pueda publicar en el broker puede pulsarlos, así que ponle una contraseña al broker o desactiva **Los botones inician copias**.

BombVault lee su estado cada 15 segundos y lo publica cuando algo cambia, como JSON en `<prefijo>/<nodo>/state`. El prefijo es `bombvault` mientras no lo cambies, y el nodo es un identificador corto que BombVault elige una vez. Los mensajes de descubrimiento van al prefijo predeterminado de Home Assistant, `homeassistant`. Ambos se conservan (retained). Un último mensaje (last will) marca el dispositivo como no disponible si BombVault se detiene sin avisar. Desactivar el enlace quita el dispositivo y sus entidades de Home Assistant.
