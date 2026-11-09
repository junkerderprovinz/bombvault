# Servidor MCP

BombVault tiene un servidor integrado para el Model Context Protocol (MCP), el protocolo que usan asistentes de IA como Claude Code y Claude Desktop para llegar a herramientas externas. A través de él un asistente puede leer cómo van tus copias y, si se lo permites, iniciar una copia o cancelar una que él mismo inició. Está desactivado hasta que creas una clave o activas el [inicio de sesión con OAuth](#oauth): hasta entonces el endpoint `/mcp` responde `404` a todo.

## Qué puede y qué no puede hacer un asistente {#tools}

| Herramienta | Qué hace | Tipo |
|---|---|---|
| `get_health` | Versión, nombre de la instancia, si hay una copia en curso y qué puede hacer esta clave | lectura |
| `get_status` | Estado de protección por sección: última copia correcta, intervalo esperado, comprobaciones de verificación y de copias externas, próximas ejecuciones programadas, copias a la espera de una app inactiva y, para contenedores, la prueba de arranque más reciente | lectura |
| `get_coverage` | Qué protege BombVault y qué no, con el motivo de cada caso | lectura |
| `list_items` | Cada contenedor, VM, conjunto de carpetas, la unidad flash y la configuración de la app que están protegidos, con su calendario, lo que detiene una copia suya, su última copia y cuánto tardó. Los contenedores de bases de datos llevan además su último volcado. También se listan los conjuntos de datos ZFS, con el resultado de su última comprobación. Cada elemento lleva su última comprobación de restauración, y un contenedor su última prueba de arranque o por qué no se puede probar. Un contenedor recreado con otros ajustes desde su última copia lista lo que cambió | lectura |
| `list_runs` | Historial de ejecuciones, las más recientes primero, filtrable por sección, elemento, estado, tipo y tiempo. Una copia lenta que algo frenó lo nombra | lectura |
| `list_restore_points` | Puntos de restauración de un elemento desde su repositorio primario y, para un contenedor, sus volcados de bases de datos. Un conjunto de datos ZFS recibe un punto de restauración por copia, con una instantánea de cada conjunto de datos que tiene debajo | lectura |
| `get_activity` | Lo que se está ejecutando ahora, con fase y porcentaje | lectura |
| `get_storage_stats` | Historial de tamaño del repositorio primario de una sección y su crecimiento por semana, y el espacio usado, libre y total en el disco o remoto de cada uno de sus repositorios | lectura |
| `get_size_breakdown` | Qué carpetas y archivos ocupan el espacio en la copia más reciente de un contenedor, VM o conjunto de carpetas, y cuánto añadió de cada uno la última copia | lectura |
| `search_files` | Busca archivos y carpetas por nombre en las copias de todos los elementos, de una sección o de un elemento (las VM no se buscan) y muestra cada copia que contiene cada resultado | lectura |
| `list_anomalies` | Anomalías que BombVault detectó en las copias, filtrables por estado, gravedad y sección, con un resumen de lo que está abierto | lectura |
| `get_anomaly` | Uno de esos hallazgos, con la nota dejada al confirmarlo | lectura |
| `start_backup` | Copia un elemento ahora | inicio |
| `start_domain_backup` | Copia todos los elementos protegidos de una sección | inicio |
| `start_backup_everything` | Ejecuta la pasada de Copiar todo ahora | inicio |
| `cancel_backup` | Cancela una copia en curso que inició esta clave | cancelación |

Esto se queda en la interfaz web: las restauraciones de cualquier tipo (incluidos descargar, guardar o importar un volcado de base de datos), borrar copias, la poda, el desbloqueo, las comprobaciones y los simulacros, la replicación a copias externas, los ajustes, las credenciales y las claves de MCP, y cancelar una copia que inició el calendario, la interfaz web u otra clave. Lo mismo vale para confirmar una anomalía o marcarla como esperada, algo que se hace en la página **Anomalías**. El motivo es que la salida de las herramientas contiene nombres y mensajes de error de tu servidor, y cualquiera de ellos podría llevar texto escrito para manipular al asistente. Un asistente que cae en ese texto puede, como mucho, iniciar una copia dentro de los límites de abajo, o cancelar una que él mismo inició.

Si el repositorio primario de un elemento es remoto (S3, REST, SFTP, rclone), `list_restore_points` lo contacta, así que la llamada puede tardar. Las copias externas no se pueden listar por MCP. `search_files` solo lee repositorios remotos, copias externas incluidas, cuando el asistente lo pide, porque tarda más y algunos proveedores cobran por los datos que descarga. Una búsqueda puede durar minutos, así que la primera llamada la inicia en segundo plano y una llamada posterior recoge el resultado. Lo que miran las comprobaciones de anomalías se describe en [Anomalías](anomalies.md), y cómo un elemento ZFS guarda una instantánea por conjunto de datos, en [Conjuntos de datos ZFS](zfs-datasets.md#contents).

## Qué hace una copia iniciada {#starting-backups}

La copia de un asistente es la misma copia que inicia la interfaz web. Un contenedor en ejecución se detiene hasta que termina su copia, junto con los contenedores configurados para detenerse con él. Una VM con el método ordenado se apaga y se vuelve a iniciar. Un conjunto de datos ZFS detiene los contenedores configurados para él mientras se toma su instantánea. Los conjuntos de carpetas, la unidad flash y la configuración siguen funcionando. Después BombVault aplica la política de retención y puede copiar al repositorio externo. `list_items` le dice al asistente qué detiene un elemento y cuánto tardó su última copia, y las descripciones de las herramientas le piden que te avise antes de iniciar nada.

Como una copia detiene cosas y va sacando puntos de restauración antiguos, los inicios por MCP están limitados:

- 12 copias iniciadas por hora y clave.
- 15 minutos entre dos inicios por MCP del mismo elemento, sección o Copiar todo ahora.
- Como mucho 4 inicios por MCP del mismo elemento en 24 horas.
- **Protección de retención.** Cuando una sección conserva un número fijo de puntos de restauración (solo "conservar las últimas N", sin regla diaria, semanal ni mensual, en local o en un destino externo), cada copia nueva empuja fuera a la más antigua. BombVault rechaza entonces un inicio por MCP de un elemento cuyas N-1 copias correctas más recientes se iniciaron todas por MCP. Así siempre queda en el conjunto conservado al menos un punto de restauración hecho por el calendario o por ti. Con "conservar la última 1" un asistente no puede copiar ese elemento en absoluto. La siguiente copia programada vuelve a hacer sitio. Una regla anual por sí sola cuenta como "conservar la última 1", ya que conserva un único punto de restauración del año en curso.

El inicio de una sección o de Copiar todo ahora deja fuera los elementos que un límite retiene y los nombra en su respuesta. La interfaz web y el calendario no están limitados por nada de esto. El presupuesto por hora vive en memoria, así que un reinicio de BombVault lo restablece.

Los inicios por la [API](api.md#errors) y desde [Home Assistant](api.md#home-assistant) cuentan para los mismos límites por elemento que los inicios por MCP, y para la protección de retención.

## Actívalo {#switch-on}

1. Abre **Ajustes, Integraciones, Servidor MCP**, haz clic en **Elegir un cliente** y elige el tuyo en la ventana. La búsqueda de arriba a la derecha lo encuentra por nombre. Un cliente que no figura se conecta mediante **Otro cliente**.
2. En **Clave**, deja **Clave nueva** y el nombre que sugiere, que es el del cliente, o escribe uno que diga dónde se usa la clave, por ejemplo "Claude Code en el portátil". Una clave por cliente te permite revocar una sin tocar las demás. **Clave existente** da al cliente una clave que hiciste antes.
3. Activa **Permitir iniciar copias** para una clave que deba poder iniciar copias. Sin eso la clave solo puede leer. Puedes cambiarlo más tarde en los **Detalles** de la clave, y el cambio se aplica a la siguiente petición del asistente sin reconectar.
4. Haz clic en **Crear clave**. La clave se muestra una sola vez. BombVault guarda solo una huella suya y no puede volver a mostrarla, así que cópiala ahora. Si cierras la ventana antes de que el cliente haya usado la clave, la tarjeta sigue mostrándola hasta que confirmes que la has copiado.

Sin contraseña de acceso la propia interfaz web está abierta a todos en tu red, y cualquiera que pueda abrirla también puede crear una clave. La tarjeta lo dice. Si abres BombVault con un nombre de aspecto público (por ejemplo `bombvault.example.com` a través de un proxy inverso) y no hay contraseña de acceso, no se pueden crear ni sustituir claves desde esa dirección, para que ninguna página web de internet pueda hacer que tu navegador cree una. Define una contraseña de acceso, o abre BombVault por su dirección IP o por un nombre local como `tower` o `tower.local`.

## Tus claves y su registro {#keys}

Cada clave tiene su propia fila en **Claves y clientes conectados**. Muestra el nombre de la clave, si puede iniciar copias o solo leer, los últimos cuatro caracteres de la clave, cuándo se creó o sustituyó por última vez y cuándo la usó un cliente por última vez. **Detalles** abre una ventana donde renombras la clave, cambias su permiso, la sustituyes o la revocas, y que muestra cuántas llamadas hizo hoy. Una clave revocada pasa a la lista de claves revocadas, donde puedes borrarla definitivamente cuando ninguna ejecución del historial la nombre.

Junto a su nombre, la fila muestra la marca del cliente para el que se hizo la clave. Una clave hecha mediante **Otro cliente**, o antes de que la tarjeta listara clientes, muestra una llave en su lugar.

El **Registro** en los detalles de una clave muestra lo que hizo esa clave, y una clave revocada conserva un botón **Registro** propio. Primero van las copias que inició, cada una con su estado y un enlace a esa ejecución en el registro de actividad del panel. Debajo están sus llamadas, las más recientes primero, con la herramienta y en qué quedó la llamada. Un rechazo dice por qué: la clave solo puede leer, la protección de retención frenó la copia, ya había otra copia en curso, el mismo elemento, sección o pasada se inició fuera de la interfaz web unos minutos antes, o la clave envió demasiadas peticiones. Una cancelación enlaza con la ejecución a la que se refería.

BombVault conserva las entradas de cada clave hasta 30 días: los 500 inicios y cancelaciones correctos más recientes, y junto a ellos las 200 demás llamadas más recientes (lecturas, rechazos y errores). Un asistente que sigue consultando una copia en curso, o que sigue reintentando una llamada rechazada, no puede sacar del registro el inicio de esa copia. De cada llamada guarda la herramienta, el resultado y la ejecución que nombró una cancelación. Nunca guarda lo que envió el asistente, ni la clave ni su huella. El paquete de diagnóstico solo cuenta las entradas, y una exportación de ajustes las deja fuera.

## Conectar un cliente {#clients}

**Elegir un cliente** abre una ventana con cada cliente como un mosaico, en **En este equipo** o **En la nube**, y una búsqueda arriba a la derecha. Un mosaico abre la configuración del cliente en la misma ventana, en tres pasos: la clave, la configuración para ese cliente, con la dirección en la que abriste la tarjeta, un botón para copiarla, dónde vive la configuración y, con el certificado propio de BombVault, lo que necesita el cliente para confiar en él. Y la espera de la primera llamada del cliente. La ventana vigila el último uso de la clave y se pone en verde cuando entra esa llamada.

La configuración mantiene la clave fuera de cualquier línea de comandos. Cuando el cliente puede leerla de una variable de entorno (`BOMBVAULT_MCP_KEY`), de un aviso enmascarado o de un archivo propio, la configuración solo la nombra. Cuando el cliente no tiene esa vía, la clave está en su archivo de configuración o en sus ajustes, y la configuración lo dice. Cuando la documentación de un cliente no dice cómo trata un certificado que no conoce, la configuración escribe ese paso como lo que hay que hacer si el cliente rechaza el certificado de BombVault.

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
| GitHub Copilot (VS Code) | archivo de configuración | aviso enmascarado |
| Goose | archivo de configuración | variable de entorno |
| Jan | formulario en la app | los ajustes de la app |
| JetBrains (AI Assistant, Junie) | archivo de configuración | el archivo de configuración |
| Kimi Code | archivo de configuración | el archivo de configuración |
| LM Studio | archivo de configuración | el archivo de configuración |
| Mistral Vibe | archivo de configuración | variable de entorno |
| Msty | formulario en la app | los ajustes de la app |
| n8n | formulario en la app | el almacén de credenciales de n8n |
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
| ChatGPT | inicio de sesión con OAuth, en la nube | un token de acceso, mira [más abajo](#oauth) |
| Claude (claude.ai) | inicio de sesión con OAuth, en la nube | un token de acceso, mira [más abajo](#oauth) |

Las secciones de abajo explican con más detalle las configuraciones de Claude Code y Claude Desktop y listan lo que necesita cualquier otro cliente.

### Claude Code {#claude-code}

Claude Code llega a BombVault mediante `mcp-remote`, que necesita Node.js en ese equipo. Primero guarda la clave en un archivo de texto propio, en una sola línea:

```text
X-API-Key: <your key>
```

Luego ejecuta una vez en un terminal el comando de la tarjeta, con la ruta de ese archivo rellenada. Detrás de un certificado en el que confía tu equipo, queda así:

```bash
claude mcp add bombvault --scope user -- npx -y mcp-remote@latest https://bombvault.example.com/mcp --header-file "<path of the file with your key>"
```

Con el certificado propio de BombVault (consulta [TLS y certificados](#tls)) el comando también apunta Node.js al certificado descargado:

```bash
claude mcp add bombvault --scope user -e "NODE_EXTRA_CA_CERTS=<path of the downloaded bombvault-cert.pem>" -- npx -y mcp-remote@latest https://192.168.1.10:3443/mcp --header-file "<path of the file with your key>"
```

Comprueba la conexión con `/mcp` dentro de Claude Code. `--scope user` hace que BombVault esté disponible en todos tus proyectos. Claude Code guarda solo la ruta del archivo de clave, así que la clave no aparece ni en el comando y el historial de tu shell ni en la lista de procesos. Guarda el archivo donde solo tú puedas leerlo y fuera de cualquier carpeta que subas a un repositorio. `@latest` hace que `npx` descargue un `mcp-remote` actual. Se usaría en su lugar uno más antiguo instalado globalmente, y ese no conoce `--header-file`.

No escribas `${BOMBVAULT_MCP_KEY}` en los argumentos de `mcp-remote` para Claude Code. Claude Code rellena esa referencia desde su propio entorno antes de iniciar `mcp-remote`, así que la clave acaba en la línea de comandos de ese proceso, donde otros programas y usuarios del equipo pueden leerla.

Sin Node.js, y solo detrás de un certificado en el que confíe tu equipo, Claude Code puede conectarse por sí solo. Pon un `.mcp.json` en la carpeta del proyecto:

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

Define `BOMBVAULT_MCP_KEY` donde se inicie Claude Code, por ejemplo en `"env"` de `~/.claude/settings.json` o en el perfil de tu shell, editado en un editor de texto en lugar de escrito en el prompt. Aquí la referencia es segura, porque Claude Code no inicia ningún segundo proceso que la lleve. El certificado propio de BombVault no funciona así: la propia conexión de Claude Code lo rechaza incluso con `NODE_EXTRA_CA_CERTS` definido. No subas nunca a un repositorio un `.mcp.json` con la clave escrita dentro.

### Claude Desktop {#claude-desktop}

Claude Desktop llega a BombVault mediante `mcp-remote`, que necesita Node.js en ese equipo. Guarda primero la clave en un archivo de texto propio, en una sola línea, como se describe para [Claude Code](#claude-code). Abre el archivo de configuración con **Settings, Developer, Edit Config** en Claude Desktop. Está en `%APPDATA%\Claude\claude_desktop_config.json` en Windows y en `~/Library/Application Support/Claude/claude_desktop_config.json` en macOS. Añade la entrada de la tarjeta dentro de `"mcpServers"`, junto a los servidores que ya haya, y reinicia Claude Desktop:

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

- `NODE_EXTRA_CA_CERTS` solo está para el certificado propio de BombVault. Quítalo detrás de un certificado en el que tu equipo ya confíe.
- `--allow-http` se añade solo para una dirección `http://` normal.
- En Windows, escribe las rutas con barras normales, por ejemplo `C:/Users/sam/bombvault-key.txt`, porque una sola barra invertida no es JSON válido. Mantén la ruta del archivo de clave sin espacios: Claude Desktop en Windows pasa a `npx` en dos trozos una ruta con espacio.
- La configuración nombra solo el archivo de clave, así que la clave no aparece ni en ella ni en la lista de procesos. Guarda el archivo donde solo tú puedas leerlo.

### Clientes en la nube {#cloud-clients}

ChatGPT, Claude en claude.ai, Grok y Le Chat llaman a BombVault desde los servidores de sus proveedores, así que BombVault tiene que ser accesible desde internet con un certificado de confianza pública, por ejemplo detrás de un proxy inverso. Le Chat rechaza los autofirmados. Un inicio de sesión en el proxy puede proteger la interfaz web, pero `/mcp` tiene que llegar a BombVault sin él: estos servicios no pueden iniciar sesión en un proxy, y BombVault comprueba por sí mismo su clave o token. Grok y Le Chat envían una clave fija, y sus botones los configuran como a los demás. ChatGPT, y Claude en claude.ai en la mayoría de las organizaciones, se conectan solo mediante inicio de sesión con OAuth, que se describe a continuación.

### Inicio de sesión con OAuth {#oauth}

Para un cliente que no puede usar una clave, BombVault es su propio servidor de autorización OAuth. El cliente se registra por sí mismo, te envía a una página de BombVault, y allí inicias sesión con tu contraseña de acceso (y el segundo factor, si lo configuraste) y se lo permites. El cliente recibe entonces un token que sirve solo para el endpoint MCP de este BombVault, y lo renueva por sí solo.

1. Define una contraseña de acceso en **Ajustes, Seguridad**. Sin ella BombVault no ofrece ningún inicio de sesión, porque no habría nadie a quien pedir el consentimiento.
2. Haz que BombVault sea accesible desde internet por https con un certificado en el que confíen los navegadores, normalmente mediante un proxy inverso. El cliente llama a `/mcp`, `/oauth/` y `/.well-known/` desde sus propios servidores, así que un proxy con un inicio de sesión propio tiene que dejar pasar esas tres rutas hasta BombVault. La página de consentimiento en `/oauth/authorize` se abre en tu propio navegador y puede quedarse detrás del inicio de sesión del proxy. Nombra también el proxy en `TRUSTED_PROXY` (consulta [Configuración](configuration.md)). BombVault limita los registros de clientes por dirección, y sin eso cada cliente parece venir del proxy.
3. En la tarjeta de MCP, activa **Inicio de sesión con OAuth** e introduce la **Dirección pública**: la dirección https sin ruta, por ejemplo `https://backup.example.com`. Cada token está ligado a esta dirección, así que tras un cambio cada cliente tiene que volver a iniciar sesión.
4. Elige ChatGPT o Claude en **Elegir un cliente**. La ventana muestra la **URL del conector**, que es la dirección pública más `/mcp`, y dónde va en ese cliente. En ChatGPT, activa el modo Developer en **Settings, Apps & Connectors, Advanced settings**, elige **Create**, pega la URL del conector como URL del servidor MCP y elige OAuth como autenticación. En claude.ai, abre **Settings, Connectors, Add custom connector**, pega la URL del conector, deja vacíos el ID y el secreto del cliente OAuth y elige **Connect**.
5. El cliente abre la página de consentimiento. Muestra quién lo pide, adónde te devuelve tu respuesta y el interruptor **Permitir iniciar copias**, que empieza desactivado. Elige **Permitir** o **Denegar**.

Cada cliente que inició sesión recibe una fila propia en la misma lista, con su marca. Sus **Detalles** contienen su registro, **Revocar** y **Permitir iniciar copias**, y tiene los mismos límites que una clave. Revocarlo surte efecto al instante. Cuando el mismo cliente vuelve a iniciar sesión, su nueva concesión sustituye a la anterior, y una concesión que nadie usó en 30 días caduca. Pueden tener sesión iniciada hasta 10 clientes a la vez, además de las 10 claves.

La página de consentimiento solo acepta una petición de un cliente registrado que nombre exactamente una de sus direcciones de retorno registradas: https, o una dirección de loopback en cualquier puerto para un cliente en tu propio equipo. Solo se acepta el flujo de código de autorización con PKCE (S256), y tu respuesta queda ligada a tu sesión, así que ningún otro sitio web puede enviarla por ti. Los tokens de acceso duran una hora. Un token de refresco se sustituye cada vez que se usa, y uno que reaparece después de eso revoca la concesión, porque otra persona tiene una copia. Un cliente que repite su último refresco en 30 segundos, porque la respuesta nunca le llegó, recibe tokens nuevos. BombVault no descarga documentos de metadatos de clientes desde internet, así que los clientes se registran mediante el registro dinámico de clientes.

### Otros clientes {#other-clients}

Sirve cualquier cliente que hable Streamable HTTP:

- URL: la dirección de la interfaz web más `/mcp`, por ejemplo `https://192.168.1.10:3443/mcp`.
- La clave en `Authorization: Bearer <key>` o en `X-API-Key: <key>`. Si se envían las dos, deben llevar la misma clave.
- `POST` con `Content-Type: application/json` y `Accept: application/json, text/event-stream`.
- Un mensaje JSON-RPC por petición. Los lotes se rechazan.
- Versiones del protocolo 2026-07-28, 2025-11-25, 2025-06-18 y 2025-03-26.

## TLS y certificados {#tls}

BombVault sirve HTTPS con un certificado que hizo él mismo, y al principio ese certificado nombra solo `localhost`, `127.0.0.1` y `::1`. Claude Code y `mcp-remote` lo rechazan en una dirección de la LAN. Las formas de sortearlo, en el orden que mejor encaja con la mayoría de las instalaciones de Unraid:

1. **Añade la dirección en la tarjeta de MCP.** Cuando la tarjeta se abre por HTTPS en una dirección que el certificado no nombra, lo dice y ofrece **Añadir esta dirección al certificado**. BombVault emite de nuevo su certificado con esa dirección añadida (tu navegador avisa una vez más, como la primera vez). Luego haz clic en **Descargar certificado**. Los fragmentos fijan `NODE_EXTRA_CA_CERTS` en el archivo descargado, de modo que el cliente confía exactamente en este certificado. Eso también significa que todo cliente configurado con una descarga anterior deja de conectarse en cuanto se emite de nuevo el certificado, en este equipo y en todos los demás, hasta que reciba el archivo nuevo.
2. **Un proxy inverso con un certificado de confianza** (Nginx Proxy Manager, SWAG, Caddy, Traefik). El cliente ve entonces el certificado del proxy y no necesita nada más, y la tarjeta no avisa sobre el propio de BombVault.
3. **Tailscale.** `tailscale serve` delante del contenedor, o la integración de Tailscale de Unraid, te da un nombre `ts.net` con un certificado de confianza.
4. **`HTTP_ONLY=true`**, solo detrás de un proxy que termine TLS o en una red en la que confíes plenamente. Pasa toda la interfaz web a HTTP sin cifrar, requiere un cambio en los ajustes del contenedor y envía la clave sin cifrar.

No pongas nunca `NODE_TLS_REJECT_UNAUTHORIZED=0`. Desactiva las comprobaciones de certificados para todo con lo que habla ese proceso de Node.js.

Un proxy inverso tiene que dejar pasar la cabecera `Authorization` (o `X-API-Key`), algo que los proxies hacen salvo que se les diga lo contrario, y no debe almacenar en búfer ni reescribir `/mcp`. Un bloque location para Nginx o Nginx Proxy Manager que además comprueba el certificado de BombVault:

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

Detrás de un proxy cada petición lleva la dirección del proxy. Cinco claves erróneas de un cliente mal configurado bloquean entonces durante un minuto a todos los clientes MCP detrás de ese proxy. Nombra el proxy en `TRUSTED_PROXY` (consulta [Configuración](configuration.md)) para contar por cliente.

## Modelo de seguridad {#security}

- Sin una clave activa y con el inicio de sesión con OAuth desactivado, `/mcp` responde `404`.
- El inicio de sesión con OAuth solo se ofrece mientras haya una contraseña de acceso. Los tokens, los códigos y los secretos de cliente se guardan solo como huellas, y un token sirve solo para la dirección para la que se emitió.
- Un cliente puede registrarse como máximo 10 veces por hora desde una dirección, y BombVault conserva como máximo 100 clientes registrados con los que nadie inició sesión, cada uno durante un día. Los códigos y tokens de refresco erróneos cuentan para el mismo bloqueo que las claves erróneas.
- Las concesiones se comportan como las claves cuando se restaura una copia de la configuración o cambia `APP_KEY`: tras una restauración cada cliente tiene que volver a iniciar sesión.
- Ninguna dirección está exenta. Las peticiones desde `localhost`, el host de Unraid, un proxy inverso o `tailscale serve` necesitan una clave como cualquier otra, también cuando la interfaz web no tiene contraseña de acceso.
- Las claves se guardan solo como huellas, se muestran una vez y se pueden renombrar, sustituir y revocar. Hasta 10 claves activas, cada una con su propio interruptor **Permitir iniciar copias**.
- Cada creación, sustitución, cambio de permiso y revocación envía una notificación por tus canales de notificación, con la dirección de origen, salvo que las notificaciones estén desactivadas.
- 5 claves erróneas por minuto y dirección, y luego `429`. 120 peticiones por minuto y 12 copias iniciadas por hora y clave, además del tiempo de espera y la protección de retención de arriba.
- Las peticiones desde una página del navegador de otro origen se rechazan.
- No se pueden crear claves desde un nombre de host de aspecto público mientras no haya contraseña de acceso.
- Cada copia que inicia un asistente, y las podas y copias externas que provoca, se marca como "via MCP" con el nombre de la clave en el registro de actividad, en el panel de errores y en la notificación de la copia.
- Cada llamada a una herramienta se escribe en el registro del contenedor con el id de la clave y sus últimos cuatro caracteres (nunca su nombre) y se cuenta en `/metrics` (`bombvault_mcp_requests_total`, `bombvault_mcp_tool_calls_total`, `bombvault_mcp_active_keys`).
- Restaurar una copia de la configuración revoca todas las claves, porque la base de datos restaurada podría contener claves que revocaste después de guardarla. Crea claves nuevas después.
- Una clave deja de funcionar cuando cambia `APP_KEY` (una reinstalación, o una restauración en otro contenedor). La tarjeta lo detecta y marca la clave, y **Sustituir clave** le da de nuevo un secreto que funciona.
- Trata una clave como una contraseña. Un cliente que no puede leer la clave de una variable de entorno, de un aviso o de un archivo de clave la guarda en texto plano en su configuración o sus ajustes, y su configuración lo dice. Prefiere una clave de solo lectura en un equipo en el que confíes menos.

## Qué sale de tu equipo {#privacy}

Todo lo que lee un asistente va al proveedor de IA que hay detrás: nombres de elementos, calendarios, historial de ejecuciones con mensajes de error, identificadores y horas de puntos de restauración, nombres de motores de bases de datos y tamaños de volcados, actividad actual, cifras de almacenamiento, cobertura y estado. Una búsqueda de archivos también envía los archivos que encuentra: sus nombres, tamaños y rutas dentro de la copia. BombVault quita las rutas del host, las ubicaciones de repositorios, los nombres de host, las credenciales, los comandos de hooks y las claves antes de que salga nada.

## Solución de problemas {#troubleshooting}

| Qué ves | Qué significa |
|---|---|
| `404` | No hay una clave activa y el inicio de sesión con OAuth está desactivado, o una ruta equivocada como `/api/mcp`. El endpoint es `/mcp`. |
| `401` | La clave falta, está mal escrita, revocada o sustituida. Un proxy puede estar descartando la cabecera `Authorization` (prueba `X-API-Key`). Si la tarjeta marca la clave como ya no válida, `APP_KEY` ha cambiado: sustituye la clave. |
| `403` | La petición vino de una página del navegador de otro origen. Usa un cliente de escritorio o de línea de comandos. |
| `405` en GET | Es normal. El endpoint solo acepta `POST`. |
| `400` "Accept must contain both 'application/json' and 'text/event-stream'" | El cliente es demasiado antiguo para Streamable HTTP. Actualízalo. |
| `400` "batch requests are not accepted" | El cliente envía lotes JSON-RPC. Envía un mensaje por petición. |
| `429` | Demasiadas claves erróneas desde esta dirección, o más de 120 peticiones por minuto con una clave. Espera un minuto y comprueba si el asistente está atascado en un bucle. |
| Errores de "certificate", "self-signed" o "unable to verify" | El cliente no confía en el certificado de BombVault. Consulta [TLS y certificados](#tls). |
| `busy` | Otra copia o tarea de mantenimiento retiene esa sección. Vuelve a intentarlo cuando haya terminado. |
| `cooldown` | Este elemento, sección o Copiar todo ahora se inició fuera de la interfaz web hace menos de 15 minutos. |
| `retention_guard` | Una copia más por MCP dejaría solo puntos de restauración hechos por MCP en una ventana de "conservar las últimas N", o el elemento ya recibió 4 copias por MCP en las últimas 24 horas, incluidas las fallidas y las canceladas. En el primer caso la siguiente copia programada hace sitio, en el segundo el elemento vuelve a estar libre 24 horas después de la más antigua de esas copias. En ambos casos puedes iniciarla en la interfaz web. |
| `rate_limited` | La clave ha usado sus 12 inicios de esta hora. |
| `not_permitted` en un inicio | La clave es de solo lectura. Activa **Permitir iniciar copias** en la tarjeta. No hace falta reconectar. En una cancelación significa que la ejecución no la inició esta clave. |
| `domain_off` | Esa sección está desactivada en **Ajustes, General**. |
| `not_found` | BombVault no protege ese elemento. Añádelo antes en la interfaz web. MCP nunca crea configuración. |
| El cliente no encuentra el servidor de autorización | El inicio de sesión con OAuth está desactivado, no hay contraseña de acceso, o el proxy no deja pasar `/.well-known/` hasta BombVault. |
| La página de consentimiento dice que la dirección de retorno no está registrada | El cliente envió una dirección de retorno que no registró. Quita el conector en el cliente y vuelve a añadirlo. |
| Un cliente con sesión iniciada recibe `401` | Su concesión se revocó, caducó tras 30 días sin uso, o cambió la dirección pública. El cliente vuelve a iniciar sesión. |

No definas la variable de entorno `MCPGODEBUG` en el contenedor. Cambia el comportamiento de la biblioteca MCP, y un valor mal formado detiene BombVault al arrancar antes de que escriba una sola línea de registro.
