# Configuración

Esta página cubre las variables de entorno del contenedor, los montajes que provee la plantilla, la copia de VMs por SSH y la configuración externa. Las **rutas de repositorio** de copia se configuran dentro de la app (Ajustes, Rutas de copia), no mediante variables de entorno.

## Variables de entorno

| Variable | Requerida | Descripción |
|---|---|---|
| `APP_KEY` | **Sí** | Secreto hexadecimal de 32 bytes (64 caracteres hex) usado para derivar la contraseña del repo restic. Genera con `openssl rand -hex 32`. Mantenlo a salvo: perderlo hace que las copias cifradas queden irrecuperables. |
| `LIBVIRT_HOST` | Para VMs | Host de Unraid alcanzado por SSH para la copia de VMs (por defecto `host.docker.internal`; la plantilla rellena de antemano un marcador de IP LAN). Usa la IP LAN de tu Unraid, requerida en una red `br0.x` personalizada. |
| `LIBVIRT_SSH_PORT` | No | Puerto SSH del host para la copia de VMs (por defecto `22`). |
| `LIBVIRT_SSH_USER` | No | Usuario SSH en el host para la copia de VMs (por defecto `root`). |
| `LIBVIRT_URI` | No | URI de conexión libvirt completa, usada **literalmente** en lugar de construirla a partir de las tres variables `LIBVIRT_*` anteriores (que en ese caso se ignoran para la cadena de conexión). Por defecto, sin definir. Necesaria en TrueNAS Scale, cuyo libvirtd escucha en un socket no estándar que la forma construida no puede expresar: `qemu+ssh://<user>@<truenas-host>/system?socket=/run/truenas_libvirt/libvirt-sock`. Consulta la sección de TrueNAS Scale en [docs/vm-backup-ssh-setup.md](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md). |
| `PORT` | No | Puerto HTTP (por defecto `3000`; solo se usa con `HTTP_ONLY=true`). |
| `HTTPS_PORT` | No | Puerto HTTPS (por defecto `3443`; la plantilla lo publica 1:1, de modo que la WebUI responde en `https://<ip>:3443`). |
| `HTTP_ONLY` | No | Establece `true` para deshabilitar el listener HTTPS autofirmado y servir solo HTTP en texto plano (para uso detrás de un proxy inverso que termina TLS). |
| `HOST_SOURCE_ROOT` | No | La ruta del host montada como **Host Data** (por defecto `/mnt`). BombVault traduce los orígenes de bind-mount que reporta Docker a rutas bajo este montaje. Cámbialo solo si montaste una raíz de host distinta. |
| `DATA_ROOT_SEGMENTS` | No | Nombres de segmentos de ruta, separados por comas, que marcan un origen de bind-mount como datos de copia (por defecto `appdata`, siguiendo la convención de Unraid `/mnt/user/appdata/<container>`). El bind mount de un contenedor se selecciona automáticamente para la copia cuando CUALQUIER segmento indicado aparece como un segmento de ruta completo de su origen en el host; por ejemplo, `DATA_ROOT_SEGMENTS=appdata,config` también recoge un bind `.../config`. Consulta [Detección de orígenes de copia](#backup-source-detection) para conocer las otras formas, siempre activas, en que se encuentra la carpeta de datos de un contenedor. |
| `PLATFORM` | No | Fuerza la plataforma en la que BombVault considera que se está ejecutando, en lugar de detectarla automáticamente: `unraid`, `generic` o `truenas` (sin definir por defecto: detecta Unraid automáticamente sondeando su marcador `dockerMan` bajo el montaje de flash, o usa `generic` en caso contrario; un valor no reconocido también recae en `generic`, y queda registrado). Establécela explícitamente en un host Docker genérico o en TrueNAS Scale, en lugar de depender del autosondeo exclusivo de Unraid: el archivo compose genérico ya lo hace así. Cambia la convención de resguardo de appdata, los destinos de restauración predeterminados entre instancias, y si se intentan o no los pasos de notificación/plugin complementario exclusivos de Unraid (consulta `internal/platform`). |
| `BOMBVAULT_SELF_CONTAINER` | No | El nombre del propio contenedor de BombVault, para que nunca se copie (y por tanto se detenga) a sí mismo (por defecto `BombVault`; autodetectado a través del hostname en redes bridge). |
| `BACKUP_MAX_HOURS` | No | Máximas horas de reloj que una única ejecución de copia puede retener el bloqueo de su dominio antes de que se fuerce su cancelación (una salvaguarda para que una ejecución atascada no pueda bloquear el dominio para siempre). Vacío (el valor por defecto) usa `48`. Súbelo para copias en la nube muy grandes o lentas (una ejecución cancelada en el límite falla con `context deadline exceeded`). Establece `0` para desactivar el límite por completo. |
| `TZ` | No | Zona horaria para el programador (por ejemplo `Europe/Berlin`). **Si no se define, todas las programaciones se ejecutan en UTC**: una programada a las 02:30 se inicia entonces a las 02:30 UTC y no en la hora local. |

## Montajes

Monta el socket de Docker, el flash (`/boot`) y la raíz de **Host Data** (`/mnt`) como se muestra en la plantilla de CA. Tanto los *orígenes* como los *destinos* de las copias viven bajo Host Data, y se monta como **rslave** para que un recurso compartido remoto que se monte después de que arranque el contenedor (por ejemplo bajo `/mnt/remotes`) se vuelva visible sin reiniciar.

Las rutas de repositorio de copia son por defecto `/mnt/user/bombvault/{container,vms,flash,config,files}`, creadas en la primera copia. Cambia la ubicación en cualquier momento en **Ajustes, Rutas de copia**.

!!! note "Comprobación de integración con el host"
    Abre `/spike` en la interfaz web después de que arranque el contenedor. Sondea cada montaje y CLI (socket de Docker, libvirt, restic, qemu-img, rclone) e informa de cualquier pieza que falte.

## Modelo de seguridad

!!! warning "Control del host equivalente a root"
    A través del socket de Docker, BombVault puede detener, eliminar y recrear contenedores y leer/escribir appdata, y para la copia de VMs inicia sesión en el host por SSH (`qemu+ssh://`, root por defecto) para ejecutar `virsh`. Cualquiera que pueda alcanzar su interfaz web tiene, en la práctica, acceso root al host.

- **Protección opcional por contraseña** (Ajustes, Seguridad): establece una contraseña para exigir inicio de sesión, bórrala para deshabilitarla. Desactivada por defecto para uso en LAN de confianza. Las sesiones están firmadas (HMAC derivado de `APP_KEY`) y cambiar la contraseña las invalida; los inicios de sesión tienen límite de frecuencia.
- Como la protección es opcional, cuando no se define, toda la interfaz y la API (incluidas la configuración externa, las rutas de prueba de manipulación y el kit de recuperación) están al alcance de cualquiera que pueda llegar al puerto. Activa la protección en cuanto uses copias externas, inmutables o cifrado.
- Ejecuta BombVault solo en una red de confianza y no expuesta. Para acceso remoto, ponlo detrás de un proxy inverso que añada autenticación y TLS. Las respuestas llevan cabeceras de seguridad básicas (CSP, `nosniff`, `X-Frame-Options`, `Referrer-Policy`).
- Con `HTTP_ONLY=true`, la cookie de sesión pierde su marca `Secure` (tiene que hacerlo para funcionar por HTTP en texto plano), así que activa la contraseña detrás de un proxy que termina TLS solo si la confidencialidad importa.
- La conexión SSH de la copia de VMs confía en la clave del host en la primera conexión (TOFU) y la fija a partir de entonces. Verifica la clave del host fuera de banda si tu ruta contenedor-a-host no es de confianza.
- Las copias están cifradas por restic cuando el cifrado está habilitado (Ajustes; activado por defecto), con la clave derivada de `APP_KEY`.

## Copia de VMs por SSH

BombVault copia las VMs KVM/libvirt **sin montar ninguna ruta de libvirt**. Ejecuta `virsh` en el host por SSH (`qemu+ssh://`), de modo que nunca puede afectar al VM Manager de tu host.

Configuración rápida:

1. **Ajustes, Sistema, Copia de VM por SSH:** copia la clave pública mostrada.
2. Añádela al `/root/.ssh/authorized_keys` de Unraid (también persistido al flash para que sobreviva a los reinicios).
3. Haz clic en **Probar conexión**.

La plantilla añade `--add-host=host.docker.internal:host-gateway` para que el contenedor pueda alcanzar el host. Establece `LIBVIRT_HOST` a la IP LAN de tu Unraid si ese nombre no resuelve (por ejemplo cuando el contenedor se ejecuta en una red `br0.x` personalizada). Si cambiaste el puerto SSH de Unraid, establece `LIBVIRT_SSH_PORT` para que coincida. Las **instantáneas en vivo** necesitan además el agente invitado de qemu en la VM y el disco en `/mnt/cache` (no en `/mnt/user`).

!!! important "Guía completa de configuración de VMs y red"
    La guía paso a paso completa (habilitación de SSH, autorización persistente de claves, enrutamiento en redes personalizadas y VLAN, método por VM y resolución de problemas del lado del host) está en [docs/vm-backup-ssh-setup.md](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) en GitHub.

## Configuración externa

Configura una réplica externa en la pestaña **Ajustes, Externo**. Consulta [Copia externa y recuperación](offsite-recovery.md) para el flujo completo (inmutable/append-only, prueba de manipulación y ensayos de DR). En resumen:

- **Backends:** SMB/CIFS y NFS (monta el recurso compartido y apunta una Ruta de copia a él), backends nativos de restic sin rclone (`s3:...`, `rest:http://host:8000/repo`, `b2:...`, `sftp:user@host:/repo`), o cualquier remoto de rclone (`rclone:<remote>:<bucket>/path`).
- Las **credenciales de la nube** se almacenan cifradas en Ajustes, Externo, Credenciales de la nube.
- **Los destinos SSH no requieren nada instalado en el otro extremo.** `sftp:` solo necesita un servidor SSH. Añade la clave pública de **Ajustes, Sistema, Copia de VM por SSH** (también en `/config/ssh/id_ed25519.pub`) al `~/.ssh/authorized_keys` del usuario de destino.
- **Copia externa:** BombVault replica las nuevas instantáneas con `restic copy` en modo de mejor esfuerzo. El repo local sigue siendo el principal. Cada dominio tiene su propio calendario externo, más un botón **Replicar ahora**.
- **Varios destinos externos por dominio:** cada dominio puede replicarse a varios destinos externos a la vez. Añade destinos adicionales en Ajustes, Externo, cada uno con su propio repositorio, clase de almacenamiento S3, marca append-only, retención y presupuesto de crecimiento; todos se replican según el calendario externo de ese dominio. Una configuración externa única existente se traslada como el primer destino.
- **Retención por fuente:** la política local vive en Ajustes, Rutas y Almacenamiento; la política externa en Ajustes, Externo (déjala toda a cero para no recortar nunca automáticamente las instantáneas externas).
- **Límites de ancho de banda:** limita la velocidad de subida/bajada de restic en Ajustes, Externo.
- **Clase de almacenamiento en frío y de archivo (S3):** para un repo externo S3 nativo, elige un nivel legible para restauración (Standard, Standard-IA, One Zone-IA, Intelligent-Tiering, Glacier Instant Retrieval). Los remotos de rclone establecen su clase en la configuración de rclone.

## Ajustes portátiles (exportar e importar) {#portable-settings-export-and-import}

La tarjeta **Exportar e importar ajustes** en la página de Ajustes escribe toda tu configuración de BombVault (ajustes de dominio, destinos externos, calendarios, retención, notificaciones) en un archivo JSON portátil que puedes importar en otra instancia, para que cambiar de máquina o clonar una instalación no signifique volver a introducirlo todo a mano. La importación muestra una vista previa y pide confirmación, y nunca toca tus datos de copia ni tu historial.

!!! warning "La exportación puede contener credenciales"
    Tú eliges si incluir las credenciales externas y de notificación en el archivo. Con las credenciales incluidas, la exportación es tan sensible como tu kit de recuperación, así que guárdala en un lugar seguro. Sin ellas, el archivo contiene solo ajustes no secretos.
