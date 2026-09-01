# Primeros pasos

Esta página te lleva desde una máquina Unraid recién instalada hasta tu primera copia de seguridad.

## Requisitos

| Requisito | Notas |
|---|---|
| **Unraid 6.12+** | Las versiones anteriores no están probadas. |
| **Ubicación del repo restic** | Una ruta local (recomendado: tu array o caché), SMB, NFS o cualquier backend de rclone. |
| **Socket de Docker** | Lo monta la plantilla automáticamente (`/var/run/docker.sock`). |
| **Flash de Unraid** (`/boot`) | La plantilla lo monta entero automáticamente (`/boot` en `/host/boot`). Habilita la copia del flash y permite que un contenedor restaurado reaparezca como una app de Unraid normal y editable. |
| **VMs KVM** (opcional) | La copia de VMs habla con libvirt por SSH, sin montaje de libvirt. Configúralo en Ajustes (consulta [Configuración](configuration.md)). |

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

## Host Docker genérico

¿No usas Unraid? BombVault también funciona como contenedor sencillo en cualquier host Docker (es además lo que sostiene el soporte de contenedores en TrueNAS Scale, antes de tener su propia entrada en el catálogo de aplicaciones).

1. Coge el fichero [`deploy/docker-compose.generic.yml`](https://github.com/junkerderprovinz/bombvault/blob/main/deploy/docker-compose.generic.yml), listo para editar, del repositorio.
2. Define `APP_KEY` (ver más abajo) y apunta el volumen Host Data a tu raíz de datos real: los comentarios del fichero explican ambas cosas.
3. `docker compose up -d` y luego abre `https://<ip-del-host>:3443/`.

Qué cambia respecto a Unraid:

- **No hay dominio flash/USB.** No existe un USB de arranque que capturar o restaurar, así que el dominio Flash de los ajustes no tiene nada que hacer aquí. En su lugar, el dominio Ficheros ofrece la sugerencia de un clic **Añadir preajuste: configuración del sistema anfitrión** (un conjunto inicial de ficheros de `/etc` que revisas y editas antes de guardar), como equivalente genérico práctico.
- **No hay notificaciones nativas de Unraid.** Los canales de notificación propios de BombVault (webhook, avisos de fallo fuera de sede, etc.) funcionan con normalidad; solo se omite el envío específico al sistema de notificaciones de Unraid, porque aquí no existe tal sistema.
- **La copia de máquinas virtuales es opcional y necesita un host libvirtd aparte, accesible por SSH.** Mira el bloque comentado del fichero compose. Un host Docker genérico no trae ningún gestor de máquinas virtuales.

## El único ajuste obligatorio

La única variable que debes establecer es `APP_KEY`, un secreto hexadecimal de 32 bytes (64 caracteres hexadecimales) usado para derivar la contraseña del repositorio restic.

Genera uno en cualquier máquina:

```bash
openssl rand -hex 32
```

Pega el resultado en el campo `APP_KEY` de la plantilla.

!!! danger "No pierdas tu APP_KEY"
    Perder `APP_KEY` hace que tus copias cifradas queden irrecuperables. Guárdalo en un lugar seguro y separado del servidor. Una vez que BombVault esté en marcha, usa su **kit de recuperación de la clave de cifrado** de un clic (consulta [Copia externa y recuperación](offsite-recovery.md)) para guardar el paquete de recuperación completo.

La plantilla también monta por ti el socket de Docker, el flash (`/boot`) y la raíz de **Host Data** (`/mnt`). Tanto los *orígenes* como los *destinos* de las copias viven bajo Host Data. Para la referencia completa de variables y la configuración externa, consulta [Configuración](configuration.md).

## Primera ejecución

1. Abre la interfaz web en `https://<your-unraid-ip>:3443` (certificado autofirmado de fábrica).
2. En **Ajustes**, habilita los dominios de copia que quieras (Contenedores, VMs, Flash, Config, Archivos) y elige un color de acento.
3. En la pestaña **Contenedores**, elige un contenedor y haz clic en **Copiar** para crear tu primer punto de restauración. Las rutas de repositorio predeterminadas son `/mnt/user/bombvault/{container,vms,flash,config,files}` y se crean en la primera copia.
4. Configura la programación desde **Ajustes, Calendarios**. Hay un *incluir todo en el calendario* de un clic para contenedores y VMs.

!!! tip "Opcional: elige un orden de copia"
    Si algunos contenedores deben copiarse siempre antes que otros (por ejemplo, una base de datos antes que la app que la usa), abre el panel de **orden de copia** en la página de Contenedores y arrástralos a la secuencia que quieras. Las ejecuciones programadas y de selección múltiple la seguirán; todo lo que dejes sin ordenar se copia empezando por lo más atrasado, como antes.

!!! note "Comprobación de integración con el host"
    Abre `/spike` en la interfaz web después de que arranque el contenedor. Sondea cada montaje y CLI (socket de Docker, libvirt, restic, qemu-img, rclone) e informa de cualquier pieza que falte, para que puedas confirmar que el contenedor está bien conectado antes de confiar en él.

## Simple frente a Avanzado

Por defecto, la interfaz muestra solo lo esencial (copiar, restaurar, programar). Usa el conmutador **Simple / Avanzado** de la barra lateral para revelar los controles de experto: retención, copia externa, hooks pre/post, restauración a nivel de archivo, notificaciones, métricas de Prometheus y las herramientas de integridad/mantenimiento. Es una preferencia por navegador y está desactivada por defecto, de modo que los recién llegados obtienen una interfaz limpia y los usuarios avanzados lo tienen todo.

## Siguientes pasos

- Explora todas las **[Funciones](features.md)**.
- Añade una o varias réplicas de **[Copia externa y recuperación](offsite-recovery.md)** (cada dominio puede enviar a varios destinos a la vez) y guarda tu kit de recuperación.
- ¿Clonando una instalación o cambiando de máquina? Lleva toda tu configuración con la tarjeta **Exportar e importar ajustes**. Consulta [Configuración](configuration.md#portable-settings-export-and-import).
- ¿Un problema? Consulta **[Resolución de problemas](troubleshooting.md)**.
