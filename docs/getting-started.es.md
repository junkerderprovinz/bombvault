# Primeros pasos

Esta página te lleva desde una máquina Unraid recién instalada hasta tu primera copia de seguridad.

## Requisitos

| Requisito | Notas |
|---|---|
| **Unraid 6.12+** | Las versiones anteriores no están probadas. Unraid es el objetivo principal, pero BombVault también funciona en un host Docker cualquiera y en TrueNAS Scale (consulta [Host Docker genérico](#generic-docker-host)). |
| **Ubicación del repo restic** | Una ruta local (recomendado: tu array o caché), SMB, NFS o cualquier backend de rclone. |
| **Socket de Docker** | Lo monta la plantilla automáticamente (`/var/run/docker.sock`). |
| **Flash de Unraid** (`/boot`) | La plantilla lo monta entero automáticamente (`/boot` en `/host/boot`). Habilita la copia del flash y permite que un contenedor restaurado reaparezca como una app de Unraid normal y editable. |
| **VMs KVM** (opcional) | La copia de VMs habla con libvirt por SSH, sin montaje de libvirt. Configúralo en **Ajustes, Integraciones, SSH del host** (consulta [Configuración](configuration.md)). |
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
2. Define `APP_KEY` (ver más abajo) y apunta el volumen Host Data a tu raíz de datos real. Los comentarios del fichero explican ambas cosas.
3. `docker compose up -d` y luego abre `https://<ip-del-host>:3443/`.

Qué cambia respecto a Unraid:

- **No hay sección Flash.** No existe un USB de arranque que capturar o restaurar, así que la sección Flash no tiene nada que hacer aquí. En su lugar, **Añadir** en la página Copias de seguridad ofrece **Configuración del sistema** como sugerencia de carpetas: un conjunto inicial de carpetas de `/etc` que revisas y editas antes de guardar.
- **No hay notificaciones nativas de Unraid.** Los canales de notificación propios de BombVault (webhook, avisos de fallo fuera de sede, etc.) funcionan con normalidad; solo se omite el envío específico al sistema de notificaciones de Unraid, porque aquí no existe tal sistema.
- **La copia de máquinas virtuales es opcional y necesita un host libvirtd aparte, accesible por SSH.** Consulta el bloque comentado del fichero compose. Un host Docker genérico no trae ningún gestor de máquinas virtuales.
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
    Perder `APP_KEY` hace que tus copias cifradas queden irrecuperables. Guárdalo en un lugar seguro y separado del servidor. Una vez que BombVault esté en marcha, define una contraseña de inicio de sesión y descarga el **kit de recuperación** en **Ajustes, Restauración** (consulta [Copia externa y recuperación](offsite-recovery.md#recovery-kit)). Contiene la clave, todos los destinos y los comandos de restic.

La plantilla también monta por ti el socket de Docker, el flash (`/boot`) y la raíz de **Host Data** (`/mnt`). Tanto los *orígenes* como los *destinos* de las copias viven bajo Host Data. Para la referencia completa de variables y la configuración externa, consulta [Configuración](configuration.md).

## Primera ejecución

![El resumen tras una primera copia: lo que te necesita, la protección de cada sección, los destinos y el historial.](assets/screenshots/dashboard.png)

*El resumen tras una primera copia: lo que te necesita, la protección de cada sección, los destinos y el historial.*

1. Abre la interfaz web en `https://<your-unraid-ip>:3443` (certificado autofirmado de fábrica). Una instalación nueva se abre en **Te damos la bienvenida a BombVault**, que enumera lo que BombVault ha encontrado en el servidor.
2. Elige **Vamos allá**. El resumen muestra entonces la tarjeta **Primeros pasos** con tres pasos:
    1. **Elige las secciones**: activa lo que BombVault copia (Contenedores, VMs, Flash, Carpetas, Conjuntos de datos ZFS, Autocopia). **Elegir** abre la lista de elementos de una sección, donde **Pausar programación** deja uno fuera. Las secciones que actives aquí se ejecutan cada noche, algo que cambias en **Ajustes, Calendario**.
    2. **Añadir destino**: adónde van las copias. Para empezar basta una carpeta local. Lo más seguro es un segundo destino fuera del local, como una Storage Box o Backblaze B2. Los repositorios locales usan por defecto `/mnt/user/bombvault/{container,vms,flash,config,files,zfs}` y se crean en la primera copia.
    3. **Primera copia**: después se ejecuta sola cada noche.
3. Abre **Copias de seguridad** para ver todos los elementos en una sola lista. **Copiar todo ahora** inicia una ejecución sobre todas las secciones activadas, y **Copiar ahora** en la página de un elemento copia solo ese.

¿Ya usaste BombVault antes? **Restaurar desde una copia** en la página de bienvenida abre la página Restauración, donde el **Asistente de emergencia** recupera los ajustes, los contenedores, las VMs y las carpetas de un servidor perdido. Consulta [Copia externa y recuperación](offsite-recovery.md#emergency).

!!! tip "Opcional: elige un orden de copia"
    Si algunos contenedores deben copiarse siempre antes que otros (por ejemplo, una base de datos antes que la app que la usa), abre la tarjeta **Orden** en **Ajustes, Calendario** y arrástralos a la secuencia que quieras. Las ejecuciones programadas y la copia total la seguirán; todo lo que dejes sin ordenar se copia empezando por lo más atrasado.

!!! note "Comprobación de integración con el host"
    La tarjeta **Integración con el host** del resumen sondea cada montaje y CLI (socket de Docker, libvirt, restic, qemu-img, rclone) e informa de cualquier pieza que falte, para que puedas confirmar que el contenedor está bien conectado antes de confiar en él. Espera en **Personalizar** hasta que la añadas.

## Orientarse {#navigation}

![Los ajustes como mosaicos, con las tarjetas del mosaico elegido a su lado. Cada cambio se escribe en el momento.](assets/screenshots/settings.png)

*Los ajustes como mosaicos, con las tarjetas del mosaico elegido a su lado. Cada cambio se escribe en el momento.*

La barra lateral tiene cinco entradas: **Resumen**, **Anomalías** (mientras la detección de anomalías esté activada), **Copias de seguridad**, **Restauración** y, bajo **Cerrar sesión**, **Ajustes**. La acción principal de una página flota abajo a la derecha, y **Más** reúne las poco habituales. En el móvil la barra contiene Resumen, Copias de seguridad, Restauración y Ajustes, y **Más** tiene el resto.

- **Copias de seguridad** lista como elementos todos los contenedores, VMs, conjuntos de carpetas, la unidad flash, todos los conjuntos de datos ZFS y la autocopia. Los mosaicos sobre la lista eligen un tipo, y **Filtros**, **Ordenar** y **Buscar** la acotan. Cada elemento tiene una página propia con tres pestañas: **Copias de seguridad**, **Ajustes** e **Integridad**.
- **Restauración** es una sola página para todo: elige una sección, busca un elemento y escoge una copia. El Asistente de emergencia y las demás fuentes también empiezan aquí.
- **Ajustes** son catorce mosaicos: General, Aspecto, Calendario, Destinos, Restauración, Notificaciones, Verificación, Seguridad, Instancias, Emparejamiento, Integraciones, Apps, Sistema e Info. Pulsa `/` o Ctrl+K para buscar en todos, también por los nombres que tenían las páginas en versiones anteriores. Arrastra los mosaicos a tu propio orden en **Aspecto**.
- **Destinos** e **Instancias** son mosaicos de ajustes con páginas propias. **Mostrar también en la barra lateral** en su mosaico los fija en la barra lateral.

Nada tiene botón Guardar y nada se despliega: lo poco habitual tiene una tarjeta propia o una ventana, y las explicaciones están en las burbujas (i). Tras una actualización, unos puntos verdes marcan lo que ha cambiado. Un clic oculta un punto, y **Ajustes, Info, Novedades** lista todos los cambios con **Ir allí**.

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

La compilación de la interfaz también hace falta para `go run`. El repositorio solo incluye un marcador vacío en `web/dist`, así que sin `npm --prefix web run build` el binario no embebe nada y responde `500 SPA index not found`, lo cual es lo esperado. Docker, libvirt y Unraid no se pueden probar en CI, así que comprueba los montajes, restic y el enlace SSH de las VMs en un host real con la tarjeta **Integración con el host** del resumen (o `GET /api/spike`) antes de abrir una pull request.

## Siguientes pasos

- Explora todas las **[Funciones](features.md)**.
- Lleva todos los servidores de tu grupo al móvil con la **[App de Android](android.md)**.
- Añade uno o varios destinos fuera del local (cada sección puede copiar a varios a la vez) y guarda tu kit de recuperación, como se describe en **[Copia externa y recuperación](offsite-recovery.md)**.
- ¿Clonando una instalación o cambiando de máquina? Lleva toda tu configuración con la tarjeta **Exportar / importar ajustes**. Consulta [Configuración](configuration.md#portable-settings-export-and-import).
- ¿Un problema? Consulta **[Resolución de problemas](troubleshooting.md)**.
