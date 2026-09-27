# Servidor MCP

BombVault incluye un servidor para el Model Context Protocol (MCP), el protocolo con el que asistentes de IA como Claude Code y Claude Desktop llegan a herramientas externas. A través de él, un asistente puede leer cómo van tus copias y, si lo permites, iniciar una copia o cancelar una que haya iniciado él. Está apagado hasta que creas una clave o activas el [inicio de sesión con OAuth](#oauth): hasta entonces el endpoint `/mcp` responde `404` a todo.

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
| `get_storage_stats` | Historial de tamaño del repositorio principal de un dominio y su crecimiento semanal, más el espacio usado, libre y total en el disco o remoto de cada uno de sus repositorios | lectura |
| `list_anomalies` | Anomalías que BombVault ha detectado en las copias, filtrables por estado, gravedad y dominio, con un resumen de lo que está abierto | lectura |
| `get_anomaly` | Uno de esos hallazgos, con la nota que se dejó al reconocerlo | lectura |
| `start_backup` | Hace ahora la copia de un elemento | inicio |
| `start_domain_backup` | Hace la copia de cada elemento protegido de un dominio | inicio |
| `start_backup_everything` | Lanza la pasada de Backup Everything | inicio |
| `cancel_backup` | Cancela una copia en curso que inició esta clave | cancelación |

Esto se queda en la interfaz web: las restauraciones de cualquier tipo (también descargar, guardar o importar un volcado de base de datos), borrar copias, prune, unlock, las comprobaciones y los simulacros, la replicación externa, los ajustes, las credenciales y las claves MCP, y cancelar una copia que haya iniciado la programación, la interfaz web u otra clave. Lo mismo vale para reconocer una anomalía o marcarla como esperada, que se hace en la página **Anomalías**. El motivo: las respuestas de las herramientas contienen nombres y mensajes de error de tu servidor, y cualquiera de ellos podría llevar un texto escrito para manipular al asistente. Un asistente que caiga en ese texto puede, como mucho, iniciar una copia dentro de los límites de abajo o cancelar una que haya iniciado él mismo.

Si el repositorio principal de un elemento es remoto (S3, REST, SFTP, rclone), `list_restore_points` lo consulta y la llamada puede tardar un rato. Las copias externas no se pueden listar por MCP. Lo que miran las comprobaciones de anomalías se explica en [Funciones](features.md), y cómo un elemento ZFS guarda una instantánea por conjunto de datos, en [Conjuntos de datos ZFS](zfs-datasets.md#contents).

## Qué hace una copia iniciada {#starting-backups}

La copia de un asistente es la misma que inicia la interfaz web. Un contenedor en marcha se detiene hasta que termina su copia, junto con los contenedores configurados para detenerse con él. Una VM con el método "graceful" se apaga y se vuelve a arrancar. Un dataset ZFS detiene los contenedores configurados para él mientras se toma su snapshot. Los conjuntos de carpetas, la memoria flash y la configuración siguen funcionando. Después, BombVault aplica la política de retención y puede copiar al repositorio externo. `list_items` le dice al asistente qué detiene un elemento y cuánto duró su última copia, y las descripciones de las herramientas le piden que te lo diga antes de iniciar nada.

Como una copia detiene servicios y saca puntos de restauración antiguos, los inicios por MCP están limitados:

- 12 copias iniciadas por hora y clave.
- 15 minutos entre dos inicios MCP del mismo elemento, del mismo dominio o de Backup Everything.
- Como mucho 4 inicios MCP del mismo elemento en 24 horas.
- **Protección de retención.** Cuando un dominio conserva un número fijo de puntos de restauración (solo "conservar los últimos N", sin regla diaria, semanal ni mensual, en local o en un destino externo), cada copia nueva saca la más antigua. BombVault rechaza entonces un inicio MCP de un elemento cuyas N-1 copias correctas más recientes se iniciaron todas por MCP. Así siempre queda en el conjunto conservado al menos un punto de restauración que creó la programación o tú. Con "conservar el último" (N = 1), un asistente no puede hacer copia de ese elemento en absoluto. La próxima copia programada vuelve a hacer sitio.

Un inicio de dominio o de Backup Everything deja fuera los elementos que retiene algún límite y los nombra en su respuesta. Ni la interfaz web ni la programación se ven afectadas por nada de esto. El cupo por hora vive en memoria, así que un reinicio de BombVault lo pone a cero.

Los inicios por la [API](api.md#errors) cuentan para los mismos límites por elemento que los inicios por MCP, y para la protección de retención.

## Activarlo {#switch-on}

1. Abre **Ajustes, Sistema, Servidor MCP** y pulsa el botón de tu cliente. Un cliente que no está en la lista se conecta mediante **Otro cliente**.
2. En **Clave**, deja **Clave nueva** y el nombre que propone, el del cliente, o escribe uno que diga dónde se usa la clave, por ejemplo «Claude Code en el portátil». Una clave por cliente te permite revocar una sin tocar las demás. **Clave existente** da al cliente una clave que creaste antes.
3. Activa **Permitir iniciar copias** para una clave que deba poder iniciar copias; sin eso solo puede leer. Puedes cambiarlo después en el recuadro de la clave, y el cambio vale desde la siguiente petición del asistente, sin reconectar.
4. Pulsa **Crear clave**. La clave se muestra una vez. BombVault solo guarda una huella de ella y no puede volver a mostrarla, así que cópiala ahora. Si cierras el diálogo antes de que el cliente haya usado la clave, la tarjeta la sigue mostrando hasta que confirmes que la has copiado.

Sin contraseña de inicio de sesión, la propia interfaz web está abierta a toda tu red, y quien pueda abrirla también puede crear una clave. La tarjeta lo avisa. Si abres BombVault con un nombre que parece público (por ejemplo `bombvault.example.com` detrás de un proxy inverso) y no hay contraseña de inicio de sesión, desde esa dirección no se pueden crear ni sustituir claves, para que ninguna página web de Internet pueda hacer que tu navegador cree una. Pon una contraseña de inicio de sesión, o abre BombVault por su dirección IP o por un nombre local como `tower` o `tower.local`.

## Tus claves y su registro {#keys}

Cada clave tiene su propia tarjeta. Muestra el nombre de la clave, si puede iniciar copias o solo leer, los cuatro últimos caracteres de la clave, cuándo se creó o se reemplazó por última vez, cuándo la usó un cliente por última vez y cuántas llamadas hizo hoy. En la tarjeta cambias el nombre de la clave, cambias su permiso, la reemplazas o la revocas. Una clave revocada pasa a la lista de claves revocadas, donde puedes borrarla para siempre cuando ninguna ejecución del historial la nombre.

Junto a su nombre, el recuadro muestra el logotipo del cliente para el que se creó la clave. Una clave creada mediante **Otro cliente**, o antes de que la tarjeta listara clientes, muestra una llave en su lugar.

**Registro** en una tarjeta abre lo que hizo esa clave. Primero van las copias que inició, cada una con su estado y un enlace a esa ejecución en el registro de actividad del panel. Debajo están sus llamadas, las más recientes primero, con la herramienta y en qué quedó la llamada. Un rechazo dice por qué: la clave solo puede leer, la protección de retención frenó la copia, ya había otra copia en curso, el elemento se copió por MCP hace unos minutos o la clave envió demasiadas solicitudes. Una cancelación enlaza la ejecución a la que se refería.

BombVault guarda las entradas de cada clave durante 30 días como máximo: los 500 inicios y cancelaciones correctos más recientes y, junto a ellos, las 200 llamadas restantes más recientes (lecturas, rechazos y errores). Así un asistente que consulta una y otra vez una copia en curso, o que repite una llamada rechazada, no puede sacar su inicio del registro. De cada llamada guarda la herramienta, el resultado y la ejecución que nombró una cancelación. Nunca guarda lo que envió el asistente, ni la clave ni su huella. El paquete de diagnóstico solo cuenta las entradas, y una exportación de ajustes las deja fuera.

## Conectar un cliente {#clients}

Cada cliente tiene un botón en la tarjeta, en **En este equipo** o en **En la nube**. El botón abre un diálogo en tres pasos: la clave; la configuración para ese cliente, con la dirección con la que abriste la tarjeta, un botón para copiarla, dónde está la configuración y, con el certificado propio de BombVault, lo que el cliente necesita para confiar en él; y la espera de la primera llamada del cliente. El diálogo vigila el último uso de la clave y se pone verde cuando llega esa llamada.

El diálogo mantiene la clave fuera de cualquier línea de comandos. Si el cliente puede leerla de una variable de entorno (`BOMBVAULT_MCP_KEY`), de una petición oculta o de un archivo propio, la configuración solo la nombra. Si el cliente no puede, la clave queda en su archivo de configuración o en sus ajustes, y el diálogo lo dice. Si la documentación de un cliente no dice cómo trata un certificado que no conoce, el diálogo escribe ese paso como lo que hay que hacer si el cliente rechaza el certificado de BombVault.

| Cliente | Configuración | De dónde sale la clave |
|---|---|---|
| AnythingLLM | archivo de configuración | el archivo de configuración |
| Antigravity | archivo de configuración | variable de entorno |
| Claude Code | comando | archivo de clave |
| Claude Desktop | archivo de configuración | archivo de clave |
| Cline | archivo de configuración | el archivo de configuración |
| Codex CLI | archivo de configuración | variable de entorno |
| Continue | archivo de configuración | `~/.continue/.env` |
| Copilot CLI | archivo de configuración | el archivo de configuración |
| Cursor | archivo de configuración | variable de entorno |
| Gemini CLI | archivo de configuración | variable de entorno |
| GitHub Copilot (VS Code) | archivo de configuración | petición oculta |
| Goose | archivo de configuración | variable de entorno |
| Jan | formulario en la app | los ajustes de la app |
| JetBrains (AI Assistant, Junie) | archivo de configuración | el archivo de configuración |
| Kimi Code | archivo de configuración | el archivo de configuración |
| LM Studio | archivo de configuración | el archivo de configuración |
| Mistral Vibe | archivo de configuración | variable de entorno |
| Msty | formulario en la app | los ajustes de la app |
| n8n | formulario en la app | las credenciales de n8n |
| Open WebUI | formulario en la app | los ajustes de la app |
| opencode | archivo de configuración | variable de entorno |
| Perplexity (Mac) | formulario en la app | archivo de clave |
| Qwen Code | archivo de configuración | variable de entorno |
| Roo Code | archivo de configuración | variable de entorno |
| Visual Studio | archivo de configuración | el archivo de configuración |
| Warp | archivo de configuración | el archivo de configuración |
| Windsurf | archivo de configuración | variable de entorno |
| Zed | archivo de configuración | el archivo de configuración |
| Grok | formulario, en la nube | los servidores del proveedor |
| Le Chat | formulario, en la nube | los servidores del proveedor |
| ChatGPT | inicio de sesión con OAuth, en la nube | un token de acceso, ver [abajo](#oauth) |
| Claude (claude.ai) | inicio de sesión con OAuth, en la nube | un token de acceso, ver [abajo](#oauth) |

Las secciones siguientes explican con más detalle la configuración de Claude Code y Claude Desktop y enumeran lo que necesita cualquier otro cliente.

### Claude Code {#claude-code}

Claude Code llega a BombVault a través de `mcp-remote`, que necesita Node.js en ese equipo. Primero guarda la clave en un archivo de texto aparte, en una sola línea:

```text
X-API-Key: <your key>
```

Después ejecuta una vez en un terminal el comando de la tarjeta, con la ruta de ese archivo. Detrás de un certificado en el que confía tu equipo, se ve así:

```bash
claude mcp add bombvault --scope user -- npx -y mcp-remote@latest https://bombvault.example.com/mcp --header-file "<path of the file with your key>"
```

Con el certificado propio de BombVault (ver [TLS y certificados](#tls)), el comando además indica a Node.js el certificado descargado:

```bash
claude mcp add bombvault --scope user -e "NODE_EXTRA_CA_CERTS=<path of the downloaded bombvault-cert.pem>" -- npx -y mcp-remote@latest https://192.168.1.10:3443/mcp --header-file "<path of the file with your key>"
```

Comprueba la conexión con `/mcp` dentro de Claude Code. `--scope user` hace que BombVault esté disponible en todos tus proyectos. Claude Code solo guarda la ruta del archivo de la clave, así que la clave no aparece ni en el comando y el historial de tu shell, ni en la lista de procesos. Guarda el archivo donde solo tú puedas leerlo y fuera de cualquier carpeta de la que hagas commit. `@latest` hace que `npx` descargue un `mcp-remote` actual; si no, se usaría uno más antiguo instalado globalmente, que no conoce `--header-file`.

No escribas `${BOMBVAULT_MCP_KEY}` en los argumentos de `mcp-remote` para Claude Code. Claude Code sustituye esa referencia con su propio entorno antes de iniciar `mcp-remote`, así que la clave acaba en la línea de comandos de ese proceso, donde otros programas y usuarios del equipo pueden leerla.

Sin Node.js, y solo detrás de un certificado en el que confía tu equipo, Claude Code puede conectarse por sí mismo. Pon un `.mcp.json` en la carpeta del proyecto:

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

Define `BOMBVAULT_MCP_KEY` donde arranca Claude Code, por ejemplo en `"env"` dentro de `~/.claude/settings.json` o en el perfil de tu shell, editándolo con un editor de texto en lugar de escribirlo en la línea de comandos. Aquí la referencia es segura, porque Claude Code no inicia ningún segundo proceso que la lleve. Con el certificado propio de BombVault esto no funciona: la conexión que abre el propio Claude Code lo rechaza aunque `NODE_EXTRA_CA_CERTS` esté definido. Nunca hagas commit de un `.mcp.json` con la clave escrita dentro.

### Claude Desktop {#claude-desktop}

Claude Desktop llega a BombVault a través de `mcp-remote`, que necesita Node.js en ese equipo. Guarda primero la clave en un archivo de texto propio, en una sola línea, como se describe para [Claude Code](#claude-code). Abre el archivo de configuración en Claude Desktop desde **Settings, Developer, Edit Config**. Está en `%APPDATA%\Claude\claude_desktop_config.json` en Windows y en `~/Library/Application Support/Claude/claude_desktop_config.json` en macOS. Añade la entrada de la tarjeta dentro de `"mcpServers"`, junto a los servidores que ya haya, y reinicia Claude Desktop:

```json
{
  "mcpServers": {
    "bombvault": {
      "command": "npx",
      "args": ["-y", "mcp-remote@latest", "https://192.168.1.10:3443/mcp", "--header-file", "<path of the file with your key>"],
      "env": {
        "NODE_EXTRA_CA_CERTS": "<path of the downloaded bombvault-cert.pem>"
      }
    }
  }
}
```

- `NODE_EXTRA_CA_CERTS` solo está para el certificado propio de BombVault. Detrás de un certificado en el que tu equipo ya confía, quítalo.
- `--allow-http` solo se añade para una dirección `http://` sin cifrar.
- En Windows, escribe las rutas con barras normales, por ejemplo `C:/Users/sam/bombvault-key.txt`, porque una barra invertida sola no es JSON válido. Deja la ruta del archivo de la clave sin espacios: Claude Desktop en Windows pasa a `npx` una ruta con un espacio en dos trozos.
- La configuración solo nombra el archivo de la clave, así que la clave no aparece ni en ella ni en la lista de procesos. Guarda el archivo donde solo tú puedas leerlo.

### Clientes en la nube {#cloud-clients}

ChatGPT, Claude en claude.ai, Grok y Le Chat llaman a BombVault desde los servidores de su proveedor, así que BombVault tiene que ser accesible desde Internet con un certificado de confianza pública, por ejemplo detrás de un proxy inverso; Le Chat rechaza los autofirmados. Un inicio de sesión en el proxy puede proteger la interfaz web, pero `/mcp` tiene que llegar a BombVault sin él: estos servicios no pueden iniciar sesión en un proxy, y BombVault comprueba su clave o su token por sí mismo. Grok y Le Chat envían una clave fija, y sus botones los configuran como a los demás. ChatGPT, y Claude en claude.ai en la mayoría de las organizaciones, solo se conectan mediante un inicio de sesión con OAuth, descrito a continuación.

### Inicio de sesión con OAuth {#oauth}

Para un cliente que no acepta una clave, BombVault es su propio servidor de autorización OAuth. El cliente se registra solo, te envía a una página de BombVault, y allí inicias sesión con tu contraseña de acceso (y el segundo factor, si lo configuraste) y le das permiso. El cliente recibe entonces un token que solo sirve para el endpoint MCP de este BombVault, y lo renueva por su cuenta.

1. Define una contraseña de acceso en **Ajustes, Sistema**. Sin ella BombVault no ofrece ningún inicio de sesión, porque no habría nadie a quien pedir el consentimiento.
2. Haz que BombVault sea accesible desde Internet por https con un certificado en el que confíen los navegadores, normalmente a través de un proxy inverso. El cliente llama a `/mcp`, `/oauth/` y `/.well-known/` desde sus propios servidores, así que un proxy con inicio de sesión propio tiene que dejar pasar esas tres rutas hasta BombVault. La página de consentimiento en `/oauth/authorize` se abre en tu propio navegador y puede quedarse detrás del inicio de sesión del proxy. Indica también el proxy en `TRUSTED_PROXY` (ver [Configuración](configuration.md)). BombVault limita los registros de clientes por dirección y, sin eso, todos los clientes parecen venir del proxy.
3. En la tarjeta MCP, activa **Inicio de sesión con OAuth** e introduce la **Dirección pública**: la dirección https sin ruta, por ejemplo `https://backup.example.com`. Cada token está ligado a esta dirección, así que tras un cambio cada cliente tiene que volver a iniciar sesión.
4. Pulsa el botón de ChatGPT o de Claude. El diálogo muestra la **URL del conector**, es decir, la dirección pública seguida de `/mcp`, y dónde va en ese cliente. En ChatGPT, activa el modo desarrollador en **Configuración, Aplicaciones y conectores, Configuración avanzada**, elige **Crear**, pega la URL del conector como URL del servidor MCP y elige OAuth como autenticación. En claude.ai, abre **Configuración, Conectores, Añadir conector personalizado**, pega la URL del conector, deja vacíos el ID de cliente y el secreto de OAuth y elige **Conectar**.
5. El cliente abre la página de consentimiento. Muestra quién lo pide, a dónde te devuelve tu respuesta y el interruptor **Permitir iniciar copias**, que empieza desactivado. Elige **Permitir** o **Denegar**.

Cada cliente con sesión iniciada tiene una tarjeta junto a las claves, con su logotipo, su registro, **Revocar** y **Permitir iniciar copias**, y los mismos límites que una clave. Revocar surte efecto al instante. Cuando el mismo cliente vuelve a iniciar sesión, su nuevo permiso sustituye al anterior, y un permiso que nadie usó durante 30 días caduca. Puede haber hasta 10 clientes con sesión iniciada a la vez, además de las 10 claves.

La página de consentimiento solo acepta una solicitud de un cliente registrado que indique exactamente una de sus direcciones de vuelta registradas: https, o una dirección de loopback en cualquier puerto para un cliente en tu propio ordenador. Solo se acepta el flujo de código de autorización con PKCE (S256), y tu respuesta está ligada a tu sesión, así que ninguna otra web puede enviarla por ti. Los tokens de acceso duran una hora. Un token de actualización se sustituye cada vez que se usa, y si vuelve a aparecer después, BombVault revoca el permiso, porque otra persona tiene una copia. Un cliente que repite su última actualización en menos de 30 segundos, porque la respuesta nunca le llegó, recibe en cambio tokens nuevos. BombVault no descarga metadatos de clientes de Internet, así que los clientes se registran mediante el registro dinámico de clientes.

### Otros clientes {#other-clients}

Sirve cualquier cliente que hable Streamable HTTP:

- URL: la dirección de la interfaz web más `/mcp`, por ejemplo `https://192.168.1.10:3443/mcp`.
- La clave en `Authorization: Bearer <key>` o en `X-API-Key: <key>`. Si se envían las dos, deben llevar la misma clave.
- `POST` con `Content-Type: application/json` y `Accept: application/json, text/event-stream`.
- Un mensaje JSON-RPC por petición; los lotes (batches) se rechazan.
- Versiones del protocolo 2026-07-28, 2025-11-25, 2025-06-18 y 2025-03-26.

## TLS y certificados {#tls}

BombVault sirve HTTPS con un certificado emitido por él mismo, y al principio ese certificado solo nombra `localhost`, `127.0.0.1` y `::1`. Claude Code y `mcp-remote` lo rechazan en una dirección de la red local. Las salidas, en el orden que conviene a la mayoría de las instalaciones de Unraid:

1. **Añadir la dirección en la tarjeta MCP.** Si abres la tarjeta por HTTPS en una dirección que el certificado no nombra, lo dice y ofrece **Añadir esta dirección al certificado**. BombVault vuelve a emitir su certificado con esa dirección (tu navegador avisa una vez más, como la primera vez). Luego haz clic en **Descargar certificado**; los fragmentos ponen `NODE_EXTRA_CA_CERTS` en el archivo descargado, de modo que el cliente confía justo en ese certificado. Eso también significa que cualquier cliente configurado con un archivo descargado antes deja de conectarse en cuanto el certificado se vuelve a emitir, en este ordenador y en todos los demás, hasta que reciba el archivo nuevo.
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

- Sin una clave activa y con el inicio de sesión con OAuth desactivado, `/mcp` responde `404`.
- El inicio de sesión con OAuth solo se ofrece mientras haya una contraseña de acceso. Los tokens, los códigos y los secretos de cliente solo se guardan como huella, y un token solo sirve para la dirección para la que se emitió.
- Un cliente puede registrarse como mucho 10 veces por hora desde una misma dirección, y BombVault guarda como mucho 100 clientes registrados con los que nadie inició sesión, cada uno durante un día. Los códigos y tokens de actualización erróneos cuentan para el mismo bloqueo que las claves erróneas.
- Los permisos se comportan como las claves al restaurar una copia de la configuración o al cambiar `APP_KEY`: tras una restauración, cada cliente tiene que volver a iniciar sesión.
- Ninguna dirección está exenta. Las peticiones desde `localhost`, desde el host Unraid, desde un proxy inverso o desde `tailscale serve` necesitan una clave como cualquier otra, también cuando la interfaz web no tiene contraseña de inicio de sesión.
- Las claves solo se guardan como huella, se muestran una vez y se pueden renombrar, sustituir y revocar. Hasta 10 claves activas, cada una con su propio interruptor **Permitir iniciar copias**.
- Cada creación, sustitución, cambio de permiso y revocación envía una notificación por tus canales de notificación, con la dirección de la que vino, salvo que las notificaciones estén desactivadas.
- 5 claves erróneas por minuto y dirección, después `429`. 120 peticiones por minuto y 12 copias iniciadas por hora y clave, además de la espera y la protección de retención de arriba.
- Se rechazan las peticiones de una página de navegador de otro origen.
- Mientras no haya contraseña de inicio de sesión, no se pueden crear claves desde un nombre de host que parezca público.
- Cada copia que inicia un asistente, y las ejecuciones de prune y de copia externa que provoca, quedan marcadas "vía MCP" con el nombre de la clave en el registro de actividad, en el panel de errores y en la notificación de la copia.
- Cada llamada a una herramienta se escribe en el registro del contenedor con el id de la clave y sus últimos cuatro caracteres (nunca su nombre) y se cuenta en `/metrics` (`bombvault_mcp_requests_total`, `bombvault_mcp_tool_calls_total`, `bombvault_mcp_active_keys`).
- Restaurar una copia de la configuración revoca todas las claves, porque la base de datos restaurada puede contener claves que revocaste después de guardarla. Crea claves nuevas después.
- Una clave deja de funcionar cuando cambia `APP_KEY` (una reinstalación, o una restauración en otro contenedor). La tarjeta lo detecta y marca la clave, y **Sustituir clave** le da de nuevo un secreto válido.
- Trata una clave como una contraseña. Un cliente que no puede leer la clave de una variable de entorno, de una petición ni de un archivo de clave la guarda en texto plano en su configuración o sus ajustes, y su diálogo lo dice. En un equipo del que te fías menos, mejor una clave de solo lectura.

## Qué sale de la máquina {#privacy}

Todo lo que lee un asistente va al proveedor de IA que tiene detrás: nombres de elementos, programaciones, historial de ejecuciones con mensajes de error, ids y horas de los puntos de restauración, nombres de los motores de bases de datos y tamaños de los volcados, actividad en curso, cifras de almacenamiento, cobertura y estado. BombVault quita las rutas del host, las ubicaciones de los repositorios, los nombres de host, las credenciales, los comandos de hook y las claves antes de que salga nada.

## Solución de problemas {#troubleshooting}

| Lo que ves | Lo que significa |
|---|---|
| `404` | No hay ninguna clave activa y el inicio de sesión con OAuth está desactivado, o la ruta es incorrecta, como `/api/mcp`. El endpoint es `/mcp`. |
| `401` | La clave falta, está mal escrita, revocada o sustituida. Puede que un proxy descarte la cabecera `Authorization` (prueba con `X-API-Key`). Si la tarjeta marca la clave como ya no válida, ha cambiado `APP_KEY`: sustituye la clave. |
| `403` | La petición vino de una página de navegador de otro origen. Usa un cliente de escritorio o de línea de comandos. |
| `405` con GET | Normal. El punto de conexión solo acepta `POST`. |
| `400` "Accept must contain both 'application/json' and 'text/event-stream'" | El cliente es demasiado antiguo para Streamable HTTP. Actualízalo. |
| `400` "batch requests are not accepted" | El cliente envía lotes JSON-RPC. Envía un mensaje por petición. |
| `429` | Demasiadas claves erróneas desde esta dirección, o más de 120 peticiones por minuto con una clave. Espera un minuto y comprueba si el asistente está atascado en un bucle. |
| Errores con "certificate", "self-signed" o "unable to verify" | El cliente no confía en el certificado de BombVault. Ver [TLS y certificados](#tls). |
| `busy` | Otra copia o una tarea de mantenimiento ocupa ese dominio. Vuelve a intentarlo cuando termine. |
| `cooldown` | Este elemento, este dominio o Backup Everything se inició por MCP hace menos de 15 minutos. |
| `retention_guard` | Una copia MCP más dejaría solo puntos de restauración de MCP en una ventana de "conservar los últimos N", o el elemento ya recibió 4 copias por MCP en las últimas 24 horas, contando las fallidas y las canceladas. En el primer caso la próxima copia programada hace sitio; en el segundo el elemento vuelve a estar libre 24 horas después de la más antigua de esas copias. En ambos casos puedes iniciarla desde la interfaz web. |
| `rate_limited` | La clave ha gastado sus 12 inicios de esta hora. |
| `not_permitted` al iniciar | La clave es de solo lectura. Activa **Permitir iniciar copias** en la tarjeta; no hace falta reconectar. Al cancelar significa que esta clave no inició la ejecución. |
| `domain_off` | Ese tipo de copia está desactivado en los ajustes. |
| `not_found` | BombVault no protege ese elemento. Añádelo primero en la interfaz web; MCP nunca crea configuración. |
| El cliente no encuentra el servidor de autorización | El inicio de sesión con OAuth está desactivado, no hay contraseña de acceso, o el proxy no deja pasar `/.well-known/` hasta BombVault. |
| La página de consentimiento dice que la dirección de vuelta no está registrada | El cliente envió una dirección de vuelta que no registró. Quita el conector en el cliente y vuelve a añadirlo. |
| Un cliente con sesión iniciada recibe `401` | Su permiso se revocó, caducó tras 30 días sin uso, o la dirección pública cambió. El cliente vuelve a iniciar sesión. |

No pongas la variable de entorno `MCPGODEBUG` en el contenedor. Cambia el comportamiento de la biblioteca MCP, y un valor mal formado detiene BombVault al arrancar antes de que escriba una sola línea de registro.
