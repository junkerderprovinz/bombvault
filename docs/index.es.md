# BombVault

**Tus datos de Unraid, sellados en una cámara acorazada. Suelta una copia. Detona una restauración.**

BombVault es una aplicación web autoalojada y nativa de Unraid para **copia de seguridad y recuperación completa ante desastres** de tus contenedores Docker y tus VMs KVM/libvirt. Se ejecuta como un único contenedor Docker multiarquitectura, te ofrece una interfaz web moderna que se adapta al tema claro/oscuro de tu sistema, y gestiona todo el ciclo de vida: copiar, programar, verificar y restaurar.

Las restauraciones son automáticas. Los contenedores reaparecen en la pestaña Docker de Unraid exactamente como estaban antes, y las VMs se vuelven a definir en el VM Manager con sus discos y su NVRAM UEFI reconectados. Sin reinstalación manual, sin reconfiguración, sin dramas.

Basado en [restic](https://restic.net), por lo que cada copia está deduplicada, es incremental y siempre está cifrada.

!!! note "Mantén a salvo tu APP_KEY"
    BombVault deriva la contraseña del repositorio restic a partir de un secreto de 32 bytes llamado `APP_KEY`. Si lo pierdes, las copias cifradas quedan irrecuperables. Genera uno con `openssl rand -hex 32` y guárdalo en un lugar seguro. Consulta [Configuración](configuration.md).

## Qué protege BombVault

| Dominio | Qué se guarda |
|---|---|
| **Contenedores Docker** | El directorio appdata más la definición del contenedor (imagen, variables de entorno, puertos, etiquetas, volúmenes). |
| **VMs KVM / libvirt** | La(s) imagen(es) de disco de la VM, la definición XML y la NVRAM UEFI, copiadas por SSH (sin montaje de libvirt). |
| **Flash de Unraid** | Todo el USB flash (`/boot`): SO, licencia, configuración del array, recursos compartidos, red y configuración de plugins. |
| **Configuración de la app** | El propio `/config` de BombVault: su base de datos de ajustes, las credenciales externas y el par de claves SSH de libvirt. |
| **Archivos y carpetas** | **Conjuntos de archivos** con nombre, cualquier carpeta del servidor, cada uno con patrones de exclusión opcionales por conjunto. |

## La restauración es la protagonista

Tras copiar los datos de vuelta desde la instantánea de restic, BombVault reproduce la definición del contenedor guardada contra la API de Docker, de modo que el contenedor reaparece en la pestaña Docker de Unraid como si siempre hubiera estado ahí (misma imagen, mismos ajustes, mismas asignaciones de puertos). Las VMs recuperan su XML redefinido por SSH y sus discos y su NVRAM UEFI reconectados, incluso después de que la VM haya sido eliminada.

Cuando una copia detiene contenedores dependientes, estos vuelven en el orden correcto: BombVault los reinicia en el orden de `depends_on` de su Compose y espera a que cada uno informe de que está saludable antes de iniciar los que dependen de él, de modo que nada se adelanta a una base de datos o una pasarela que aún no está activa. Consulta [Funciones](features.md).

## Cómo funciona

```
Browser --HTTPS--> BombVault container
                   |- Go binary: JSON API + embedded React UI
                   |- Background worker (per-domain scheduler + job executor)
                   |
                   |- /var/run/docker.sock  -> Docker API (container stop/inspect/recreate)
                   |- qemu+ssh://host       -> libvirt / KVM on the HOST over SSH (no mount)
                   |- /mnt/ -> /host/user   -> appdata, VM disks + restic repos (read/write)
                   |- /boot/ -> /host/boot  -> Unraid flash backup (whole USB)
                   |- /config               -> BombVault's own settings + credentials (self-backup)
                   '- <repo path>           -> restic repository (local or remote: rclone/s3/rest/sftp)
```

BombVault es la capa de orquestación y de interfaz, no el motor de almacenamiento. Todo el movimiento real de datos pasa por restic.

## Inicio rápido

¿Nuevo por aquí? Ve a **[Primeros pasos](getting-started.md)** para instalar BombVault en Unraid mediante Community Applications y ejecutar tu primera copia. Después explora todas las **[Funciones](features.md)**, ajusta tu **[Configuración](configuration.md)** y prepara **[Copia externa y recuperación](offsite-recovery.md)**.

La copia externa puede repartirse a varios destinos por dominio a la vez, un **panel receptor** de solo lectura monitoriza esas copias en la máquina que las recibe, y puedes llevar toda tu configuración a una máquina nueva con la tarjeta **Exportar e importar ajustes**. Consulta [Copia externa y recuperación](offsite-recovery.md) y [Configuración](configuration.md#portable-settings-export-and-import).

## Enlaces

- **Código fuente:** [github.com/junkerderprovinz/bombvault](https://github.com/junkerderprovinz/bombvault)
- **Hilo de soporte de Unraid:** [forums.unraid.net](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/)
- **Incidencias:** [github.com/junkerderprovinz/bombvault/issues](https://github.com/junkerderprovinz/bombvault/issues)

!!! warning "Control del host equivalente a root"
    A través del socket de Docker, BombVault puede detener, eliminar y recrear contenedores y leer/escribir appdata, y para la copia de VMs inicia sesión en el host por SSH para ejecutar `virsh`. Cualquiera que pueda alcanzar su interfaz web tiene, en la práctica, acceso root al host. Ejecuta BombVault solo en una red de confianza y no expuesta, y activa la protección opcional por contraseña (Ajustes, Seguridad) en cuanto uses copias externas o inmutables. Consulta [Configuración](configuration.md) para conocer el modelo de seguridad completo.
