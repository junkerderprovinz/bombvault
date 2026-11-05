# BombVault

**Tus datos de Unraid, sellados en una cámara acorazada. Suelta una copia. Detona una restauración.**

![El resumen: la cabecera de estado, lo que necesita atención, la protección de cada sección, los destinos y el historial.](assets/screenshots/overview/home-dark.png#only-dark)
![El resumen: la cabecera de estado, lo que necesita atención, la protección de cada sección, los destinos y el historial.](assets/screenshots/overview/home-light.png#only-light)

BombVault es una aplicación web autoalojada para **copia de seguridad y recuperación completa ante desastres** en Unraid y otros hosts Docker, como TrueNAS SCALE, ZimaOS o un Linux normal. Copia contenedores Docker, VMs KVM/libvirt, carpetas, conjuntos de datos ZFS y la unidad flash de Unraid. Se ejecuta como un único contenedor Docker multiarquitectura, te ofrece una interfaz web con tema oscuro, claro o del sistema, y gestiona todo el ciclo de vida: copiar, programar, verificar y restaurar.

Las restauraciones son automáticas. En Unraid, los contenedores reaparecen en la pestaña Docker exactamente como estaban antes, y las VMs se vuelven a definir en el VM Manager con sus discos y su NVRAM UEFI reconectados. Sin reinstalación manual, sin reconfiguración, sin dramas. En TrueNAS, un contenedor restaurado junto al original se ejecuta como un contenedor Docker normal que la pantalla Apps no lista.

Basado en [restic](https://restic.net), por lo que cada copia está deduplicada, es incremental y siempre está cifrada por defecto.

!!! note "Mantén a salvo tu APP_KEY"
    BombVault deriva la contraseña del repositorio restic a partir de un secreto de 32 bytes llamado `APP_KEY`. Si lo pierdes junto con el kit de recuperación, las copias cifradas quedan irrecuperables. Genera uno con `openssl rand -hex 32` y guárdalo en un lugar seguro. Consulta [Configuración](configuration.md).

## Qué protege BombVault {#what-bombvault-protects}

Cada contenedor, cada VM, cada conjunto de carpetas, la unidad flash, cada conjunto de datos ZFS y los propios ajustes de BombVault son un **elemento** con una página propia. Los elementos del mismo tipo forman una **sección**, que se activa en **Ajustes, General**.

| Sección | Qué se guarda |
|---|---|
| **Contenedores** | El directorio appdata más la definición del contenedor (imagen, variables de entorno, puertos, etiquetas, volúmenes). |
| **VMs** | La(s) imagen(es) de disco de la VM, la definición XML y la NVRAM UEFI, copiadas por SSH (sin montaje de libvirt). |
| **Flash** | Todo el USB flash (`/boot`): sistema operativo, licencia, configuración del array, recursos compartidos, red y configuración de plugins. |
| **Autocopia** | El propio `/config` de BombVault: su base de datos de ajustes, las credenciales de sus destinos, el par de claves SSH de libvirt, la clave y las claves de host fijadas de los servidores ZFS y los certificados. |
| **Carpetas** | Cualquier carpeta del servidor como conjunto de carpetas, cada uno con patrones de exclusión opcionales. |
| **Conjuntos de datos ZFS** | Un conjunto de datos y todos los que tiene debajo, leídos de una instantánea ZFS y guardados como una carpeta. Consulta [Conjuntos de datos ZFS](zfs-datasets.md). |

## La restauración es la protagonista {#restore-is-the-star}

Tras copiar los datos de vuelta desde la instantánea de restic, BombVault reproduce la definición del contenedor guardada contra la API de Docker, de modo que el contenedor reaparece en la pestaña Docker de Unraid como si siempre hubiera estado ahí (misma imagen, mismos ajustes, mismas asignaciones de puertos). A las VMs se les vuelve a definir el XML por SSH y se les reconectan los discos y la NVRAM UEFI, incluso después de que la VM haya sido eliminada.

Una sola ventana restaura cada tipo. Por defecto elige **Junto al original**, que devuelve la copia como una copia nueva y deja el original como está. **Reemplazar original** copia antes el estado actual, así que una restauración se puede deshacer. Una carpeta o un conjunto de datos ZFS nuevo son los otros destinos.

Cuando una copia detiene contenedores dependientes, estos vuelven en el orden correcto: BombVault los reinicia en su orden `depends_on` de Compose y espera a que cada uno informe de que está sano antes de iniciar los que dependen de él, de modo que nada se adelanta a una base de datos o a una pasarela que aún no está lista. Consulta [Restauración](restore.md) y [Comandos y otros contenedores](backups.md#commands).

## Cómo funciona {#how-it-works}

```
Browser --HTTPS--> BombVault container
                   |- Go binary: JSON API + embedded React UI
                   |- Background worker (per-section scheduler + job executor)
                   |
                   |- /var/run/docker.sock  -> Docker API (container stop/inspect/recreate)
                   |- qemu+ssh://host       -> libvirt / KVM on the HOST over SSH (no mount)
                   |- /mnt/ -> /host/user   -> appdata, VM disks + restic repos (read/write)
                   |- /boot/ -> /host/boot  -> Unraid flash backup (whole USB)
                   |- /config               -> BombVault's own settings + sign-ins (self-backup)
                   '- <repo path>           -> restic repository (local or remote: rclone/s3/rest/sftp)
```

BombVault usa el socket de Docker para detener contenedores antes de una copia y para recrearlos tras una restauración. Para las VMs ejecuta `virsh` en el host por SSH (`qemu+ssh://`) para apagar un dominio de forma ordenada o tomar una instantánea en vivo. Nunca monta por bind una ruta de libvirt, así que no puede estorbar al VM Manager del host.

BombVault es la capa de orquestación y de interfaz, no el motor de almacenamiento. Todo el movimiento real de datos pasa por restic.

## Inicio rápido {#quick-start}

¿Eres nuevo por aquí? Ve a **[Primeros pasos](getting-started.md)** para instalar BombVault y hacer tu primera copia, y después configura **[Copias externas y recuperación](offsite-recovery.md)**. Los **[ejemplos prácticos](guides.md)** recorren de principio a fin los trabajos habituales, la página de **[Ajustes](settings.md)** lista cada mosaico, y **[Funciones](features.md)** es el catálogo breve de todo lo que hace BombVault.

Cada sección puede copiar a varios destinos a la vez, un BombVault emparejado puede guardar tus copias externas en un servidor que rechaza los borrados, y el **Asistente de emergencia** recupera un servidor perdido a partir de su kit de recuperación, a través de tu grupo o con su clave escrita. Para llevar tu configuración a un equipo nuevo, usa la tarjeta **Exportar / importar ajustes**. Consulta [Copias externas y recuperación](offsite-recovery.md) y [Configuración](configuration.md#portable-settings-export-and-import).

La **[app de Android](android.md)** lleva a tu teléfono todos los servidores de tu grupo, con el registro de actividad de todos ellos en una sola pantalla.

La documentación está en 26 idiomas y la interfaz en 42. Las páginas que aún no tienen traducción se muestran en inglés.

## Palabras que usamos aquí {#glossary}

Algunas palabras tienen un significado fijo en BombVault y en estas páginas.

| Palabra | Significado |
|---|---|
| **Elemento** | Una cosa que BombVault copia: un contenedor, una VM, un conjunto de carpetas, la unidad flash, un conjunto de datos ZFS con los que tiene debajo, o sus propios ajustes. Cada uno tiene su propia página. |
| **Sección** | Todos los elementos de un mismo tipo, como Contenedores. Las secciones se activan en **Ajustes, General**. |
| **Destino** | Un lugar adonde van las copias: una carpeta de este servidor, un recurso compartido, un servidor propio o un servicio en la nube. Consulta [Destinos](offsite-recovery.md#destinations). |
| **Local** | El destino de este servidor que contiene las carpetas de las secciones. |
| **Carpeta propia** | El repositorio en el que una sección escribe primero sus copias. Por defecto está en este servidor y puede estar en un destino. Consulta [La carpeta propia de una sección](offsite-recovery.md#own-folder). |
| **Copia externa** | Una copia de seguridad copiada de la carpeta propia a otro destino con `restic copy`. |
| **Repositorio directo** | Un repositorio en un lugar externo al que los elementos copian directamente, sin una copia local de por medio. Consulta [Repositorios directos](offsite-recovery.md#direct-repositories). |
| **Credenciales** | La clave o contraseña guardada para un servicio S3 o un rest-server. Una de ellas es la predeterminada. Consulta [Credenciales](offsite-recovery.md#sign-ins). |
| **Destino por elemento** | Qué destinos reciben las copias de un elemento, fijado en el elemento o como valor predeterminado de su sección. Consulta [Destino por elemento](offsite-recovery.md#placement). |
| **Réplica** | Una copia exacta a nivel de bloque de un elemento ZFS en un segundo servidor ZFS, hecha con `zfs send`. Está junto a la copia de seguridad y no la sustituye. Consulta [Réplica](zfs-datasets.md#replica). |
| **Rol** | Algo que una instancia de tu grupo hace por otra: Receptor, Recolector, Servidor ZFS o Guardián de claves. Consulta [Instancias y roles](offsite-recovery.md#roles). |

## Créditos {#credits}

- **[VolumeVault](https://github.com/Darkdragon14/VolumeVault)** de [@Darkdragon14](https://github.com/Darkdragon14) (Apache-2.0) dio a BombVault su idea de partida: copia con un clic y reinstalación automática de contenedores Docker. BombVault es una implementación aparte en Go y restic que lleva la idea a las VMs, al flash y más.
- **[restic](https://restic.net/)** es el motor de copias, rápido, seguro y con deduplicación, que maneja BombVault.
- **[rclone](https://rclone.org/)** aporta los backends en la nube.
- La mayoría de los glifos de los botones proceden del conjunto gratuito Core Solid de **[Streamline](https://streamlinehq.com)** ([CC BY 4.0](https://creativecommons.org/licenses/by/4.0/), [fuente](https://github.com/webalys-hq/streamline-vectors)). Los demás proceden de Font Awesome Free, Material Design Icons, Simple Icons y Tabler Icons, o se dibujaron para el proyecto.

## Licencia {#license}

Copyright (C) 2026 Junker der Provinz. BombVault es software libre bajo la **GNU Affero General Public License v3.0** ([LICENSE](https://github.com/junkerderprovinz/bombvault/blob/main/LICENSE)). Puedes ejecutarlo, estudiarlo, compartirlo y cambiarlo. Si lo distribuyes, o ejecutas una versión modificada como servicio de red, debes publicar tu código fuente bajo la misma licencia y conservar los avisos de copyright y atribución existentes.

El nombre y la marca no tienen licencia. La AGPL cubre solo el código fuente: "BombVault", su logotipo y su marca siguen reservados, así que una bifurcación tiene que usar un nombre y una marca propios y no puede presentarse como BombVault.

## Enlaces {#links}

- **Código fuente:** [github.com/junkerderprovinz/bombvault](https://github.com/junkerderprovinz/bombvault)
- **Hilo de soporte de Unraid:** [forums.unraid.net](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/)
- **Incidencias:** [github.com/junkerderprovinz/bombvault/issues](https://github.com/junkerderprovinz/bombvault/issues)

!!! warning "Control equivalente a root sobre el host"
    A través del socket de Docker, BombVault puede detener, eliminar y recrear contenedores y leer y escribir appdata, y para la copia de VMs inicia sesión en el host por SSH para ejecutar `virsh`. Cualquiera que pueda llegar a su interfaz web tiene en la práctica root en el host. Ejecuta BombVault solo en una red de confianza y no expuesta, y define una contraseña de acceso en **Ajustes, Seguridad**. El kit de recuperación, los depósitos de claves, el ZIP del flash, exportar una copia del flash o una autocopia y el borrado de las copias de un destino la necesitan. Consulta [Configuración](configuration.md#security-model) para el modelo de seguridad completo.
