# BombVault

**Tus datos de Unraid, sellados en una cámara acorazada. Suelta una copia. Detona una restauración.**

BombVault es una aplicación web autoalojada y nativa de Unraid para **copia de seguridad y recuperación completa ante desastres** de tus contenedores Docker y tus VMs KVM/libvirt. Se ejecuta como un único contenedor Docker multiarquitectura, te ofrece una interfaz web con tema oscuro, claro o del sistema, y gestiona todo el ciclo de vida: copiar, programar, verificar y restaurar.

Las restauraciones son automáticas. Los contenedores reaparecen en la pestaña Docker de Unraid exactamente como estaban antes, y las VMs se vuelven a definir en el VM Manager con sus discos y su NVRAM UEFI reconectados. Sin reinstalación manual, sin reconfiguración, sin dramas.

Basado en [restic](https://restic.net), por lo que cada copia está deduplicada, es incremental y siempre está cifrada.

!!! note "Mantén a salvo tu APP_KEY"
    BombVault deriva la contraseña del repositorio restic a partir de un secreto de 32 bytes llamado `APP_KEY`. Si lo pierdes junto con el kit de recuperación, las copias cifradas quedan irrecuperables. Genera uno con `openssl rand -hex 32` y guárdalo en un lugar seguro. Consulta [Configuración](configuration.md).

## Qué protege BombVault

Cada contenedor, cada VM, cada conjunto de carpetas, el flash, cada conjunto de datos ZFS y los propios ajustes de BombVault son un **elemento** con una página propia. Los elementos del mismo tipo forman una **sección**, que se activa en **Ajustes, General**.

| Sección | Qué se guarda |
|---|---|
| **Contenedores** | El directorio appdata más la definición del contenedor (imagen, variables de entorno, puertos, etiquetas, volúmenes). |
| **VMs** | La(s) imagen(es) de disco de la VM, la definición XML y la NVRAM UEFI, copiadas por SSH (sin montaje de libvirt). |
| **Flash** | Todo el USB flash (`/boot`): SO, licencia, configuración del array, recursos compartidos, red y configuración de plugins. |
| **Autocopia** | El propio `/config` de BombVault: su base de datos de ajustes, los accesos de sus destinos y el par de claves SSH de libvirt. |
| **Carpetas** | Cualquier carpeta del servidor como conjunto de carpetas, cada uno con patrones de exclusión opcionales. |
| **Conjuntos de datos ZFS** | Un conjunto de datos con todos los que tiene debajo, leído de una instantánea ZFS y guardado como una carpeta. Consulta [Conjuntos de datos ZFS](zfs-datasets.md). |

## La restauración es la protagonista

Tras copiar los datos de vuelta desde la instantánea de restic, BombVault reproduce la definición del contenedor guardada contra la API de Docker, de modo que el contenedor reaparece en la pestaña Docker de Unraid como si siempre hubiera estado ahí (misma imagen, mismos ajustes, mismas asignaciones de puertos). Las VMs recuperan su XML redefinido por SSH y sus discos y su NVRAM UEFI reconectados, incluso después de que la VM haya sido eliminada.

Una sola ventana restaura todos los tipos. Por defecto propone **Junto al original**, que devuelve la copia como duplicado y deja el original como está. **Reemplazar original** hace antes una copia del estado actual, así que una restauración se puede deshacer. Una carpeta o un conjunto de datos ZFS nuevo son los otros destinos.

Cuando una copia detiene contenedores dependientes, estos vuelven en el orden correcto: BombVault los reinicia en el orden de `depends_on` de su Compose y espera a que cada uno informe de que está saludable antes de iniciar los que dependen de él, de modo que nada se adelanta a una base de datos o una pasarela que aún no está activa. Consulta [Funciones](features.md).

## Cómo funciona

```
Browser --HTTPS--> BombVault container
                   |- Go binary: JSON API + embedded React UI
                   |- Background worker (per-section scheduler + job executor)
                   |
                   |- /var/run/docker.sock  -> Docker API (container stop/inspect/recreate)
                   |- qemu+ssh://host       -> libvirt / KVM on the HOST over SSH (no mount)
                   |- /mnt/ -> /host/user   -> appdata, VM disks + restic repos (read/write)
                   |- /boot/ -> /host/boot  -> Unraid flash backup (whole USB)
                   |- /config               -> BombVault's own settings + credentials (self-backup)
                   '- <repo path>           -> restic repository (local or remote: rclone/s3/rest/sftp)
```

BombVault usa el socket de Docker para detener los contenedores antes de una copia y recrearlos después de una restauración. Para las VMs ejecuta `virsh` en el host por SSH (`qemu+ssh://`) para apagar una VM de forma ordenada o tomar una instantánea en vivo. Nunca monta una ruta de libvirt, así que no puede interferir con el VM Manager del host.

BombVault es la capa de orquestación y de interfaz, no el motor de almacenamiento. Todo el movimiento real de datos pasa por restic.

## Inicio rápido

¿Nuevo por aquí? Ve a **[Primeros pasos](getting-started.md)** para instalar BombVault en Unraid mediante Community Applications y ejecutar tu primera copia. Después explora todas las **[Funciones](features.md)**, ajusta tu **[Configuración](configuration.md)** y prepara **[Copia externa y recuperación](offsite-recovery.md)**.

Cada sección puede copiar a varios destinos a la vez, un BombVault emparejado puede guardar tus copias externas en un servidor que rechaza los borrados, y el **Asistente de emergencia** recupera un servidor perdido a partir de su kit de recuperación, a través de tu grupo o con su clave escrita a mano. Para llevar tu configuración a una máquina nueva, usa la tarjeta **Exportar / importar ajustes**. Consulta [Copia externa y recuperación](offsite-recovery.md) y [Configuración](configuration.md#portable-settings-export-and-import).

La **[App de Android](android.md)** lleva todos los servidores de tu grupo al móvil, con el registro de actividad de todos ellos en una sola pantalla.

## Créditos {#credits}

- **[VolumeVault](https://github.com/Darkdragon14/VolumeVault)** de [@Darkdragon14](https://github.com/Darkdragon14) (Apache-2.0) dio a BombVault su idea de partida: copia de un clic y reinstalación automática de contenedores Docker. BombVault es una implementación independiente, sobre Go y restic, que lleva la idea a las VMs, al flash y a más cosas.
- **[restic](https://restic.net/)** es el motor de copias rápido, seguro y con deduplicación que BombVault controla.
- **[rclone](https://rclone.org/)** proporciona los backends de la nube.
- La mayoría de los iconos de los botones proceden del conjunto gratuito Core Solid de **[Streamline](https://streamlinehq.com)** ([CC BY 4.0](https://creativecommons.org/licenses/by/4.0/), [fuente](https://github.com/webalys-hq/streamline-vectors)). Los demás proceden de Font Awesome Free, Material Design Icons, Simple Icons y Tabler Icons, o se dibujaron para el proyecto.

## Licencia {#license}

Copyright (C) 2026 Junker der Provinz. BombVault es software libre bajo la **GNU Affero General Public License v3.0** ([LICENSE](https://github.com/junkerderprovinz/bombvault/blob/main/LICENSE)). Puedes ejecutarlo, estudiarlo, compartirlo y modificarlo. Si lo distribuyes, o si ejecutas una versión modificada como servicio de red, debes publicar tu código fuente bajo la misma licencia y conservar los avisos de copyright y de atribución existentes.

El nombre y la marca no se licencian. La AGPL cubre solo el código fuente: "BombVault", su logotipo y su imagen de marca quedan reservados, así que un fork tiene que usar un nombre y una imagen de marca propios y no puede presentarse como BombVault.

## Enlaces

- **Código fuente:** [github.com/junkerderprovinz/bombvault](https://github.com/junkerderprovinz/bombvault)
- **Hilo de soporte de Unraid:** [forums.unraid.net](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/)
- **Incidencias:** [github.com/junkerderprovinz/bombvault/issues](https://github.com/junkerderprovinz/bombvault/issues)

!!! warning "Control del host equivalente a root"
    A través del socket de Docker, BombVault puede detener, eliminar y recrear contenedores y leer/escribir appdata, y para la copia de VMs inicia sesión en el host por SSH para ejecutar `virsh`. Cualquiera que pueda alcanzar su interfaz web tiene, en la práctica, acceso root al host. Ejecuta BombVault solo en una red de confianza y no expuesta, y define una contraseña de inicio de sesión en **Ajustes, Seguridad**. El kit de recuperación, los depósitos de claves y el borrado de las copias de un destino la necesitan. Consulta [Configuración](configuration.md#security-model) para conocer el modelo de seguridad completo.
