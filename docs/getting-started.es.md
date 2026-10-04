# Primeros pasos

Esta página te lleva desde una máquina Unraid recién instalada hasta tu primera copia de seguridad.

## Requisitos

| Requisito | Notas |
|---|---|
| **Unraid 6.12+** | Las versiones anteriores no están probadas. Unraid es el objetivo principal, pero BombVault también funciona en un host Docker cualquiera y en TrueNAS Scale (consulta [Host Docker genérico](#generic-docker-host)). |
| **Ubicación del repo restic** | Una ruta local (recomendado: tu array o caché), SMB, NFS o cualquier backend de rclone. |
| **Socket de Docker** | Lo monta la plantilla automáticamente (`/var/run/docker.sock`). |
| **Flash de Unraid** (`/boot`) | La plantilla lo monta entero automáticamente (`/boot` en `/host/boot`). Habilita la copia del flash y permite que un contenedor restaurado reaparezca como una app de Unraid normal y editable. |
| **VMs KVM** (opcional) | La copia de VMs habla con libvirt por SSH, sin montaje de libvirt. Configúralo en Ajustes (consulta [Configuración](configuration.md)). |
| **Conjuntos de datos ZFS** (opcional) | El mismo enlace SSH que la copia de VMs, `zfs` en el host y Host Data mapeado como `/mnt` con el modo de acceso Read/Write - Slave, el valor por defecto de la plantilla. Consulta [Conjuntos de datos ZFS](zfs-datasets.md). |
| **App de Android** (opcional) | Android 10 o posterior, emparejada con servidores en la versión 9.7.0 o posterior. Consulta [App de Android](android.md). |

## Instalación en Unraid

La vía más sencilla es **Community Applications**.

1. Abre la pestaña **Apps** en Unraid.
2. Busca **BombVault**.
3. Haz clic en **Install**, establece las variables requeridas (más abajo) y aplica.

!!! tip "Instalación manual de la plantilla"
    Si prefieres añadir la plantilla a mano:

    1. Ve a **Docker, Add Container, Template repositories** y añade:
       ```
       https://github.com/junkerderprovinz/unraid-apps
       ```
    2. Busca **BombVault** en Templates.
    3. Establece las variables requeridas y haz clic en **Apply**.

## Host Docker genérico {#generic-docker-host}

¿No usas Unraid? BombVault también funciona como contenedor sencillo en cualquier host Docker (es además lo que sostiene el soporte de contenedores en TrueNAS Scale, antes de tener su propia entrada en el catálogo de aplicaciones).

1. Coge el fichero [`deploy/docker-compose.generic.yml`](https://github.com/junkerderprovinz/bombvault/blob/main/deploy/docker-compose.generic.yml), listo para editar, del repositorio.
2. Define `APP_KEY` (ver más abajo) y apunta el volumen Host Data a tu raíz de datos real: los comentarios del fichero explican ambas cosas.
3. `docker compose up -d` y luego abre `https://<ip-del-host>:3443/`.

Qué cambia respecto a Unraid:

- **No hay dominio flash/USB.** No existe un USB de arranque que capturar o restaurar, así que el dominio Flash de los ajustes no tiene nada que hacer aquí. En su lugar, el dominio Carpetas ofrece la sugerencia de un clic **Añadir preajuste: configuración del sistema anfitrión** (un conjunto inicial de ficheros de `/etc` que revisas y editas antes de guardar), como equivalente genérico práctico.
- **No hay notificaciones nativas de Unraid.** Los canales de notificación propios de BombVault (webhook, avisos de fallo fuera de sede, etc.) funcionan con normalidad; solo se omite el envío específico al sistema de notificaciones de Unraid, porque aquí no existe tal sistema.
- **La copia de máquinas virtuales es opcional y necesita un host libvirtd aparte, accesible por SSH.** Mira el bloque comentado del fichero compose. Un host Docker genérico no trae ningún gestor de máquinas virtuales.
- **No hay widget en el panel.** BombVault Widget es un plugin de Unraid, así que ese paso también se omite.
- **Encontrar los datos de un contenedor.** Sin la convención `appdata` de Unraid, la carpeta de datos de un contenedor se encuentra a partir de los segmentos de `DATA_ROOT_SEGMENTS`, los volúmenes con nombre de Docker, el directorio de trabajo de un proyecto Compose y la etiqueta `bombvault.data` (consulta [Detección de las fuentes de copia](configuration.md#backup-source-detection)). Los volúmenes con nombre y el preajuste `/etc` solo alcanzan rutas dentro del montaje Host Data, así que apunta Host Data a un directorio antecesor común que cubra también la raíz de datos de Docker.
- **`PLATFORM`.** Ponla en `generic` o `truenas`. Si no se define, BombVault detecta Unraid por su propia marca en el montaje del flash y trata todo lo demás como genérico, y los pasos exclusivos de Unraid se omiten en lugar de intentarse y fallar.

**TrueNAS Scale** sigue el mismo camino con compose; hay una entrada de catálogo preparada en el repositorio, pero aún no se ha enviado. Allí la copia de VMs necesita `LIBVIRT_URI`, porque el libvirtd de TrueNAS escucha en un socket propio (`/run/truenas_libvirt/libvirt-sock`) que las tres variables `LIBVIRT_*` no pueden expresar (consulta [Configuración](configuration.md)). Hasta dónde está probado: la copia de zvol se ejecutó contra una máquina TrueNAS Scale real, sobre un zvol conectado a una VM en marcha, y `zfs snapshot`, `zfs send`, restic y `zfs receive` lo llevaron y lo devolvieron idéntico byte a byte. Una restauración completa dirigida por el propio BombVault aún no se ha ejecutado en hardware TrueNAS, y ese zvol era disperso, así que el rendimiento con muchos gigabytes no está probado. Prueba allí una restauración antes de confiar en ello.

## El único ajuste obligatorio

La única variable que debes establecer es `APP_KEY`, un secreto hexadecimal de 32 bytes (64 caracteres hexadecimales) usado para derivar la contraseña del repositorio restic.

Genera uno en cualquier máquina:

```bash
openssl rand -hex 32
```

Pega el resultado en el campo `APP_KEY` de la plantilla (Unraid), o en la variable de entorno `APP_KEY` de `docker-compose.yml` (host Docker genérico).

!!! danger "No pierdas tu APP_KEY"
    Perder `APP_KEY` hace que tus copias cifradas queden irrecuperables. Guárdalo en un lugar seguro y separado del servidor. Una vez que BombVault esté en marcha, usa su **kit de recuperación de la clave de cifrado** de un clic (consulta [Copia externa y recuperación](offsite-recovery.md)) para guardar el paquete de recuperación completo.

La plantilla también monta por ti el socket de Docker, el flash (`/boot`) y la raíz de **Host Data** (`/mnt`). Tanto los *orígenes* como los *destinos* de las copias viven bajo Host Data. Para la referencia completa de variables y la configuración externa, consulta [Configuración](configuration.md).

## Primera ejecución

![El panel tras una primera copia: qué está protegido, qué toca a continuación y un registro en vivo.](assets/screenshots/dashboard.png)

*El panel tras una primera copia: qué está protegido, qué toca a continuación y un registro en vivo.*

1. Abre la interfaz web en `https://<your-unraid-ip>:3443` (certificado autofirmado de fábrica).
2. En **Ajustes**, habilita los dominios de copia que quieras (Contenedores, VMs, Flash, Autocopia, Carpetas, Conjuntos de datos ZFS) y elige un color de acento.
3. En la pestaña **Contenedores**, elige un contenedor y haz clic en **Copiar ahora** para crear tu primer punto de restauración. Las rutas de repositorio predeterminadas son `/mnt/user/bombvault/{container,vms,flash,config,files,zfs}` y se crean en la primera copia.
4. Configura la programación desde **Ajustes, Programaciones**. Hay un *Incluir todo en el calendario* de un clic para contenedores y VMs.

!!! tip "Opcional: elige un orden de copia"
    Si algunos contenedores deben copiarse siempre antes que otros (por ejemplo, una base de datos antes que la app que la usa), abre el panel de **Orden de las copias de seguridad** en la página de Contenedores y arrástralos a la secuencia que quieras. Las ejecuciones programadas y de selección múltiple la seguirán; todo lo que dejes sin ordenar se copia empezando por lo más atrasado, como antes.

!!! note "Comprobación de integración con el host"
    Abre `/spike` en la interfaz web después de que arranque el contenedor. Sondea cada montaje y CLI (socket de Docker, libvirt, restic, qemu-img, rclone) e informa de cualquier pieza que falte, para que puedas confirmar que el contenedor está bien conectado antes de confiar en él.

## Simple frente a Avanzado

![Los ajustes no tienen botón Guardar: cada cambio se escribe en el momento.](assets/screenshots/settings.png)

*Los ajustes no tienen botón Guardar: cada cambio se escribe en el momento.*

Por defecto, la interfaz muestra solo lo esencial (copiar, restaurar, programar). Usa el conmutador **Vista simple / Vista avanzada** de la barra lateral para revelar los controles de experto: retención, copia externa, hooks pre/post, restauración a nivel de archivo, notificaciones, métricas de Prometheus y las herramientas de integridad/mantenimiento. Es una preferencia por navegador y está desactivada por defecto, de modo que los recién llegados obtienen una interfaz limpia y los usuarios avanzados lo tienen todo.

## Compilar desde el código fuente {#build-from-source}

BombVault es un único binario estático de Go que sirve una API JSON y una interfaz React embebida. Compila primero la interfaz y luego ejecuta el binario:

```bash
npm --prefix web ci
npm --prefix web run build     # writes web/dist, which the binary embeds
export APP_KEY=$(openssl rand -hex 32)
go test ./...                  # unit and integration tests, with a real restic round trip
golangci-lint run ./...
go run ./cmd/bombvault         # serves https://localhost:3443 with a self-signed certificate
```

La compilación de la interfaz también hace falta para `go run`. El repositorio solo incluye un marcador vacío en `web/dist`, así que sin `npm --prefix web run build` el binario no embebe nada y responde `500 SPA index not found`, lo cual es lo esperado. Docker, libvirt y Unraid no se pueden probar en CI, así que comprueba los montajes, restic y el enlace SSH de las VMs en un host real con la comprobación de integración con el host (`/spike`) antes de abrir una pull request.

## Siguientes pasos

- Explora todas las **[Funciones](features.md)**.
- Lleva todos los servidores de tu grupo al móvil con la **[App de Android](android.md)**.
- Añade una o varias réplicas de **[Copia externa y recuperación](offsite-recovery.md)** (cada dominio puede enviar a varios destinos a la vez) y guarda tu kit de recuperación.
- ¿Clonando una instalación o cambiando de máquina? Lleva toda tu configuración con la tarjeta **Exportar / importar ajustes**. Consulta [Configuración](configuration.md#portable-settings-export-and-import).
- ¿Un problema? Consulta **[Resolución de problemas](troubleshooting.md)**.
