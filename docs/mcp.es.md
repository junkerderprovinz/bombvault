# Servidor MCP

BombVault incluye un servidor para el Model Context Protocol (MCP), el protocolo con el que asistentes de IA como Claude Code y Claude Desktop acceden a herramientas externas. A través de él, un asistente puede consultar cómo van tus copias y, si lo permites, iniciar una copia o cancelar una que haya iniciado él mismo. Está apagado hasta que creas una clave: sin una clave activa, el punto de conexión `/mcp` responde `404` a todo.

## Qué puede hacer un asistente y qué no {#tools}

| Herramienta | Qué hace | Tipo |
|---|---|---|
| `get_health` | Versión, nombre de la instancia, si hay una copia en curso y qué puede hacer esta clave | lectura |
| `get_status` | Estado de protección por dominio: última copia correcta, intervalo esperado, verificaciones y comprobaciones externas, próximas ejecuciones programadas | lectura |
| `get_coverage` | Qué protege BombVault y qué no, con el motivo de cada caso | lectura |
| `list_items` | Cada contenedor, VM y conjunto de carpetas protegido, la memoria flash y la configuración de la app, con su programación, lo que detiene una copia, su última copia y cuánto duró; los contenedores de bases de datos indican también su último volcado; los datasets ZFS también aparecen, con el resultado de su última comprobación | lectura |
| `list_runs` | Historial de ejecuciones, primero las más recientes, filtrable por dominio, elemento, estado, tipo y fecha | lectura |
| `list_restore_points` | Puntos de restauración de un elemento en su repositorio principal y, para un contenedor, sus volcados de base de datos; un dataset ZFS tiene un punto de restauración por copia, con un snapshot de cada dataset que cuelga de él | lectura |
| `get_activity` | Lo que se está ejecutando ahora, con fase y porcentaje | lectura |
| `get_storage_stats` | Historial de tamaño del repositorio principal de un dominio y su crecimiento semanal | lectura |
| `list_anomalies` | Copias inusuales que BombVault ha detectado, filtrables por estado, gravedad y dominio, con un resumen de lo que está abierto | lectura |
| `get_anomaly` | Uno de esos hallazgos, con la nota que se dejó al reconocerlo | lectura |
| `start_backup` | Hace ahora la copia de un elemento | inicio |
| `start_domain_backup` | Hace la copia de cada elemento protegido de un dominio | inicio |
| `start_backup_everything` | Lanza la pasada de Backup Everything | inicio |
| `cancel_backup` | Cancela una copia en curso que inició esta clave | cancelación |

Esto se queda en la interfaz web: las restauraciones de cualquier tipo (también descargar, guardar o importar un volcado de base de datos), borrar copias, prune, unlock, las comprobaciones y los simulacros, la replicación externa, los ajustes, las credenciales y las claves MCP, y cancelar una copia que haya iniciado la programación, la interfaz web u otra clave. Lo mismo vale para reconocer una anomalía o marcarla como esperada, que se hace en la página **Anomalías**. El motivo: las respuestas de las herramientas contienen nombres y mensajes de error de tu servidor, y cualquiera de ellos podría llevar un texto escrito para manipular al asistente. Un asistente que caiga en ese texto puede, como mucho, iniciar una copia dentro de los límites de abajo o cancelar una que haya iniciado él mismo.

Si el repositorio principal de un elemento es remoto (S3, REST, SFTP, rclone), `list_restore_points` lo consulta y la llamada puede tardar un rato. Las copias externas no se pueden listar por MCP.

## Qué hace una copia iniciada {#starting-backups}

La copia de un asistente es la misma que inicia la interfaz web. Un contenedor en marcha se detiene hasta que termina su copia, junto con los contenedores configurados para detenerse con él. Una VM con el método "graceful" se apaga y se vuelve a arrancar. Un dataset ZFS detiene los contenedores configurados para él mientras se toma su snapshot. Los conjuntos de carpetas, la memoria flash y la configuración siguen funcionando. Después, BombVault aplica la política de retención y puede copiar al repositorio externo. `list_items` le dice al asistente qué detiene un elemento y cuánto duró su última copia, y las descripciones de las herramientas le piden que te lo diga antes de iniciar nada.

Como una copia detiene servicios y saca puntos de restauración antiguos, los inicios por MCP están limitados:

- 12 copias iniciadas por hora y clave.
- 15 minutos entre dos inicios MCP del mismo elemento, del mismo dominio o de Backup Everything.
- Como mucho 4 inicios MCP del mismo elemento en 24 horas.
- **Protección de retención.** Cuando un dominio conserva un número fijo de puntos de restauración (solo "conservar los últimos N", sin regla diaria, semanal ni mensual, en local o en un destino externo), cada copia nueva saca la más antigua. BombVault rechaza entonces un inicio MCP de un elemento cuyas N-1 copias correctas más recientes se iniciaron todas por MCP. Así siempre queda en el conjunto conservado al menos un punto de restauración que creó la programación o tú. Con "conservar el último" (N = 1), un asistente no puede hacer copia de ese elemento en absoluto. La próxima copia programada vuelve a hacer sitio.

Un inicio de dominio o de Backup Everything deja fuera los elementos que retiene algún límite y los nombra en su respuesta. Ni la interfaz web ni la programación se ven afectadas por nada de esto. El cupo por hora vive en memoria, así que un reinicio de BombVault lo pone a cero.

## Activarlo {#switch-on}

1. Abre **Ajustes, Sistema, Servidor MCP** y haz clic en **Clave nueva**.
2. Dale a la clave un nombre que diga dónde se usa, por ejemplo "Claude Code en el portátil". Con una clave por cliente puedes revocar una sin tocar las demás.
3. Deja **Permitir iniciar copias** activado, o desactívalo para una clave que solo deba leer. Puedes cambiarlo más tarde en la fila de la clave, y el cambio vale desde la siguiente petición del asistente, sin reconectar.
4. Haz clic en **Crear clave**. La clave se muestra una sola vez. BombVault solo guarda una huella de ella y no puede volver a mostrarla, así que cópiala ahora o toma uno de los fragmentos de debajo, que entonces llevan la clave real.

Sin contraseña de inicio de sesión, la propia interfaz web está abierta a toda tu red, y quien pueda abrirla también puede crear una clave. La tarjeta lo avisa. Si abres BombVault con un nombre que parece público (por ejemplo `bombvault.example.com` detrás de un proxy inverso) y no hay contraseña de inicio de sesión, desde esa dirección no se pueden crear ni sustituir claves, para que ninguna página web de Internet pueda hacer que tu navegador cree una. Pon una contraseña de inicio de sesión, o abre BombVault por su dirección IP o por un nombre local como `tower` o `tower.local`.

## Conectar un cliente {#clients}

La tarjeta muestra fragmentos listos para la dirección con la que la abriste: elige tu cliente y copia el fragmento. Lo que sigue explica qué hacen los fragmentos y da las formas que la tarjeta no muestra.

### Claude Code {#claude-code}

Ejecuta una vez en un terminal el comando de la tarjeta. Con un certificado en el que confía tu equipo, se ve así:

```bash
claude mcp add --transport http bombvault --scope user https://bombvault.example.com/mcp --header "Authorization: Bearer <your key>"
```

Comprueba la conexión con `/mcp` dentro de Claude Code. `--scope user` guarda la clave en tu configuración de usuario y no en un archivo de proyecto.

El comando contiene la clave, y tu shell puede guardarlo en su historial. Para evitarlo, pon un `.mcp.json` en la carpeta del proyecto y guarda la clave en una variable de entorno. Claude Code sustituye `${BOMBVAULT_MCP_KEY}` al leer el archivo:

```json
{
  "mcpServers": {
    "bombvault": {
      "type": "http",
      "url": "https://bombvault.example.com/mcp",
      "headers": {
        "Authorization": "Bearer ${BOMBVAULT_MCP_KEY}"
      }
    }
  }
}
```

Define `BOMBVAULT_MCP_KEY` donde arranca Claude Code, por ejemplo en el perfil de tu shell, editándolo con un editor de texto en lugar de escribirlo en la línea de comandos. Nunca hagas commit de un `.mcp.json` con la clave escrita dentro.

Con el certificado propio de BombVault (ver [TLS y certificados](#tls)), el comando de la tarjeta ejecuta en su lugar `mcp-remote` e indica a Node.js el certificado descargado:

```bash
claude mcp add bombvault --scope user -e "NODE_EXTRA_CA_CERTS=<path of the downloaded bombvault-cert.pem>" -e "BOMBVAULT_MCP_KEY=<your key>" -- npx -y mcp-remote https://192.168.1.10:3443/mcp --header 'X-API-Key:${BOMBVAULT_MCP_KEY}'
```

Las comillas simples impiden que tu shell expanda la variable; de eso se encarga `mcp-remote`. La misma forma sirve en `.mcp.json`: usa la entrada de Claude Desktop de abajo y quita `BOMBVAULT_MCP_KEY` de su `env`, así la clave sale de tu entorno.

### Claude Desktop {#claude-desktop}

Claude Desktop llega a BombVault a través de `mcp-remote`, que necesita Node.js en ese equipo. Abre el archivo de configuración en Claude Desktop desde **Settings, Developer, Edit Config**. Está en `%APPDATA%\Claude\claude_desktop_config.json` en Windows y en `~/Library/Application Support/Claude/claude_desktop_config.json` en macOS. Añade la entrada de la tarjeta dentro de `"mcpServers"`, junto a los servidores que ya haya, y reinicia Claude Desktop:

```json
{
  "mcpServers": {
    "bombvault": {
      "command": "npx",
      "args": ["-y", "mcp-remote", "https://192.168.1.10:3443/mcp", "--header", "X-API-Key:${BOMBVAULT_MCP_KEY}"],
      "env": {
        "BOMBVAULT_MCP_KEY": "<your key>",
        "NODE_EXTRA_CA_CERTS": "<path of the downloaded bombvault-cert.pem>"
      }
    }
  }
}
```

- `NODE_EXTRA_CA_CERTS` solo está para el certificado propio de BombVault. Detrás de un certificado en el que tu equipo ya confía, quítalo.
- `--allow-http` solo se añade para una dirección `http://` sin cifrar.
- La cabecera se escribe `X-API-Key:${BOMBVAULT_MCP_KEY}`, sin espacio después de los dos puntos y con la clave en `env`. En algunos sistemas `mcp-remote` corta un valor de `--header` en el primer espacio, y una clave escrita después de un espacio se perdería.

### Conectores personalizados en los ajustes de Claude {#custom-connectors}

Los conectores que se añaden en los propios ajustes de Claude (en claude.ai y en la lista de conectores de Claude Desktop) todavía no están soportados. A esos conectores se llega desde la nube de Anthropic, así que necesitan una dirección HTTPS pública, e inician sesión mediante OAuth. No pueden enviar una clave fija, y BombVault solo ofrece claves fijas, sin inicio de sesión OAuth. Poner BombVault en Internet para ellos no serviría de nada. Usa Claude Code, o Claude Desktop con `mcp-remote` como arriba.

### Otros clientes {#other-clients}

Sirve cualquier cliente que hable Streamable HTTP:

- URL: la dirección de la interfaz web más `/mcp`, por ejemplo `https://192.168.1.10:3443/mcp`.
- La clave en `Authorization: Bearer <key>` o en `X-API-Key: <key>`. Si se envían las dos, deben llevar la misma clave.
- `POST` con `Content-Type: application/json` y `Accept: application/json, text/event-stream`.
- Un mensaje JSON-RPC por petición; los lotes (batches) se rechazan.
- Versiones del protocolo 2026-07-28, 2025-11-25, 2025-06-18 y 2025-03-26.

## TLS y certificados {#tls}

BombVault sirve HTTPS con un certificado emitido por él mismo, y al principio ese certificado solo nombra `localhost`, `127.0.0.1` y `::1`. Claude Code y `mcp-remote` lo rechazan en una dirección de la red local. Las salidas, en el orden que conviene a la mayoría de las instalaciones de Unraid:

1. **Añadir la dirección en la tarjeta MCP.** Si abres la tarjeta por HTTPS en una dirección que el certificado no nombra, lo dice y ofrece **Añadir esta dirección al certificado**. BombVault vuelve a emitir su certificado con esa dirección (tu navegador avisa una vez más, como la primera vez). Luego haz clic en **Descargar certificado**; los fragmentos ponen `NODE_EXTRA_CA_CERTS` en el archivo descargado, de modo que el cliente confía justo en ese certificado.
2. **Un proxy inverso con un certificado de confianza** (Nginx Proxy Manager, SWAG, Caddy, Traefik). El cliente ve entonces el certificado del proxy y no necesita nada más, y la tarjeta no avisa del de BombVault.
3. **Tailscale.** `tailscale serve` delante del contenedor, o la integración de Tailscale en Unraid, te da un nombre `ts.net` con un certificado de confianza.
4. **`HTTP_ONLY=true`**, solo detrás de un proxy que termine TLS o en una red en la que confíes del todo. Pasa toda la interfaz web a HTTP sin cifrar, requiere un cambio en los ajustes del contenedor y envía la clave sin cifrar.

Nunca pongas `NODE_TLS_REJECT_UNAUTHORIZED=0`. Desactiva la comprobación de certificados para todo lo que hable ese proceso de Node.js.

Un proxy inverso tiene que dejar pasar la cabecera `Authorization` (o `X-API-Key`), cosa que los proxys hacen salvo que se les diga lo contrario, y no debe almacenar en búfer ni reescribir `/mcp`. Un bloque location para Nginx o Nginx Proxy Manager que además comprueba el certificado de BombVault:

```nginx
location /mcp {
    proxy_pass https://192.168.1.10:3443;
    proxy_ssl_verify on;
    proxy_ssl_trusted_certificate /data/bombvault-cert.pem;
    proxy_ssl_name localhost;
    proxy_http_version 1.1;
    proxy_buffering off;
    proxy_set_header Host $host;
}
```

Detrás de un proxy, cada petición lleva la dirección del proxy. Cinco claves erróneas de un solo cliente mal configurado bloquean entonces durante un minuto a todos los clientes MCP detrás de ese proxy. Indica el proxy en `TRUSTED_PROXY` (ver [Configuración](configuration.md)) para contar por cliente.

## Modelo de seguridad {#security}

- Sin una clave activa, `/mcp` responde `404`.
- Ninguna dirección está exenta. Las peticiones desde `localhost`, desde el host Unraid, desde un proxy inverso o desde `tailscale serve` necesitan una clave como cualquier otra, también cuando la interfaz web no tiene contraseña de inicio de sesión.
- Las claves solo se guardan como huella, se muestran una vez y se pueden renombrar, sustituir y revocar. Hasta 10 claves activas, cada una con su propio interruptor **Puede iniciar copias**.
- Cada creación, sustitución, cambio de permiso y revocación envía una notificación por tus canales de notificación, con la dirección de la que vino, salvo que las notificaciones estén desactivadas.
- 5 claves erróneas por minuto y dirección, después `429`. 120 peticiones por minuto y 12 copias iniciadas por hora y clave, además de la espera y la protección de retención de arriba.
- Se rechazan las peticiones de una página de navegador de otro origen.
- Mientras no haya contraseña de inicio de sesión, no se pueden crear claves desde un nombre de host que parezca público.
- Cada copia que inicia un asistente, y las ejecuciones de prune y de copia externa que provoca, quedan marcadas "vía MCP" con el nombre de la clave en el registro de actividad, en el panel de errores y en la notificación de la copia.
- Cada llamada a una herramienta se escribe en el registro del contenedor con el id de la clave y sus últimos cuatro caracteres (nunca su nombre) y se cuenta en `/metrics` (`bombvault_mcp_requests_total`, `bombvault_mcp_tool_calls_total`, `bombvault_mcp_active_keys`).
- Restaurar una copia de la configuración revoca todas las claves, porque la base de datos restaurada puede contener claves que revocaste después de guardarla. Crea claves nuevas después.
- Una clave deja de funcionar cuando cambia `APP_KEY` (una reinstalación, o una restauración en otro contenedor). La tarjeta lo detecta y marca la clave, y **Sustituir clave** le da de nuevo un secreto válido.
- Trata una clave como una contraseña. Claude Code y Claude Desktop la guardan en texto plano en su configuración. En un equipo en el que confíes menos, mejor una clave de solo lectura.

## Qué sale de la máquina {#privacy}

Todo lo que lee un asistente va al proveedor de IA que tiene detrás: nombres de elementos, programaciones, historial de ejecuciones con mensajes de error, ids y horas de los puntos de restauración, nombres de los motores de bases de datos y tamaños de los volcados, actividad en curso, cifras de almacenamiento, cobertura y estado. BombVault quita las rutas del host, las ubicaciones de los repositorios, los nombres de host, las credenciales, los comandos de hook y las claves antes de que salga nada.

## Solución de problemas {#troubleshooting}

| Lo que ves | Lo que significa |
|---|---|
| `404` | No hay clave activa, o la ruta es incorrecta, como `/api/mcp`. El punto de conexión es `/mcp`. |
| `401` | La clave falta, está mal escrita, revocada o sustituida. Puede que un proxy descarte la cabecera `Authorization` (prueba con `X-API-Key`). Si la tarjeta marca la clave como ya no válida, ha cambiado `APP_KEY`: sustituye la clave. |
| `403` | La petición vino de una página de navegador de otro origen. Usa un cliente de escritorio o de línea de comandos. |
| `405` con GET | Normal. El punto de conexión solo acepta `POST`. |
| `400` "Accept must contain both 'application/json' and 'text/event-stream'" | El cliente es demasiado antiguo para Streamable HTTP. Actualízalo. |
| `400` "batch requests are not accepted" | El cliente envía lotes JSON-RPC. Envía un mensaje por petición. |
| `429` | Demasiadas claves erróneas desde esta dirección, o más de 120 peticiones por minuto con una clave. Espera un minuto y comprueba si el asistente está atascado en un bucle. |
| Errores con "certificate", "self-signed" o "unable to verify" | El cliente no confía en el certificado de BombVault. Ver [TLS y certificados](#tls). |
| `busy` | Otra copia o una tarea de mantenimiento ocupa ese dominio. Vuelve a intentarlo cuando termine. |
| `cooldown` | Este elemento, este dominio o Backup Everything se inició por MCP hace menos de 15 minutos. |
| `retention_guard` | Una copia MCP más dejaría solo puntos de restauración de MCP en una ventana de "conservar los últimos N". La próxima copia programada hace sitio, o iníciala desde la interfaz web. |
| `rate_limited` | La clave ha gastado sus 12 inicios de esta hora. |
| `not_permitted` al iniciar | La clave es de solo lectura. Activa **Puede iniciar copias** en la tarjeta; no hace falta reconectar. Al cancelar significa que esta clave no inició la ejecución. |
| `domain_off` | Ese tipo de copia está desactivado en los ajustes. |
| `not_found` | BombVault no protege ese elemento. Añádelo primero en la interfaz web; MCP nunca crea configuración. |

No pongas la variable de entorno `MCPGODEBUG` en el contenedor. Cambia el comportamiento de la biblioteca MCP, y un valor mal formado detiene BombVault al arrancar antes de que escriba una sola línea de registro.
