# Primeros pasos

Esta página te lleva desde un equipo Unraid recién instalado hasta tu primera copia de seguridad.

## Requisitos {#requirements}

| Requisito | Notas |
|---|---|
| **Unraid 6.12+** | Las versiones anteriores no están probadas. Unraid es el objetivo principal, pero BombVault también funciona en un host Docker normal y en TrueNAS Scale (consulta [Host Docker genérico](#generic-docker-host) y [TrueNAS SCALE](#truenas)). |
| **Ubicación del repositorio restic** | Una ruta local (recomendado: tu array o la caché), SMB, NFS o cualquier backend de rclone. |
| **Socket de Docker** | La plantilla lo monta automáticamente (`/var/run/docker.sock`). |
| **Flash de Unraid** (`/boot`) | La plantilla lo monta entero automáticamente (`/boot` en `/host/boot`). Hace posible la copia del flash y permite que un contenedor restaurado reaparezca como una app de Unraid normal y editable. |
| **VMs KVM** (opcional) | La copia de VMs habla con libvirt por SSH, sin montaje de libvirt. Configúralo en **Ajustes, Integraciones, SSH del host** (consulta [Configuración](configuration.md)). |
| **Conjuntos de datos ZFS** (opcional) | El mismo enlace SSH que las copias de VMs, `zfs` en el host, y Host Data asignado como `/mnt` con modo de acceso Read/Write - Slave, el valor predeterminado de la plantilla. Consulta [Conjuntos de datos ZFS](zfs-datasets.md). |
| **App de Android** (opcional) | Android 10 o posterior, emparejada con servidores de la versión 9.7.0 o posterior. Consulta [App de Android](android.md). |

## Instalar en Unraid {#install-on-unraid}

El camino más fácil es **Community Applications**.

1. Abre la pestaña **Apps** de Unraid.
2. Busca **BombVault**.
3. Haz clic en **Install**, define las variables obligatorias (abajo) y aplica.

!!! tip "Instalación manual de la plantilla"
    Si prefieres añadir la plantilla a mano:

    1. Ve a **Docker, Add Container, Template repositories** y añade:
       ```
       https://github.com/junkerderprovinz/unraid-apps
       ```
    2. Busca **BombVault** en Templates.
    3. Define las variables obligatorias y haz clic en **Apply**.

## Host Docker genérico {#generic-docker-host}

¿No usas Unraid? BombVault también se ejecuta como un contenedor normal en cualquier host Docker. TrueNAS SCALE tiene sus propios pasos en [TrueNAS SCALE](#truenas).

1. Toma del repositorio el archivo listo para editar [`deploy/docker-compose.generic.yml`](https://github.com/junkerderprovinz/bombvault/blob/main/deploy/docker-compose.generic.yml).
2. Define `APP_KEY` (mira más abajo) y apunta el volumen Host Data a la raíz real de tus datos. Los comentarios del archivo explican ambas cosas.
3. `docker compose up -d`, y luego abre `https://<host-ip>:3443/`.

Qué es distinto respecto a Unraid:

- **No hay sección Flash.** No hay USB de arranque que capturar ni restaurar, así que la sección Flash no tiene nada que hacer aquí. En cambio, cuando Host Data llega al `/etc` del host, **Añadir** en la página Copias de seguridad ofrece **Configuración del sistema** como sugerencia de carpeta: un conjunto de carpetas `/etc` de partida que revisas y editas antes de guardar.
- **No hay notificaciones nativas de Unraid.** Los propios canales de notificación de BombVault dentro de la app (webhook, alertas de fallos de copias externas, etc.) funcionan con normalidad. Solo se omite el envío específico de Unraid a su sistema nativo de notificaciones, porque no hay ningún sistema así al que enviar.
- **La copia de VMs es opcional y necesita un host libvirtd aparte accesible por SSH.** Consulta el bloque comentado del archivo compose. Un host Docker genérico no trae gestor de VMs integrado.
- **No hay mosaico de panel.** TollRoll es un plugin de Unraid, así que también se omite ese paso.
- **Encontrar los datos de un contenedor.** Sin la convención `appdata` de Unraid, la carpeta de datos de un contenedor se encuentra a partir de los segmentos de `DATA_ROOT_SEGMENTS`, los volúmenes con nombre de Docker, el directorio de trabajo de un proyecto Compose y la etiqueta `bombvault.data` (consulta [Detección del origen de la copia](configuration.md#backup-source-detection)). Los volúmenes con nombre y el preajuste de `/etc` solo llegan a rutas dentro del montaje de Host Data, así que apunta Host Data a un ancestro común que cubra también la raíz de datos de Docker.
- **`PLATFORM`.** Ponla en `generic` o `truenas`. Si se deja sin definir, BombVault detecta Unraid por su propio marcador en el montaje del flash y trata todo lo demás como genérico, y los pasos exclusivos de Unraid se omiten en lugar de intentarse y fallar.

### TrueNAS SCALE {#truenas}

BombVault todavía no está en el catálogo de apps de TrueNAS, así que instálalo como app personalizada: **Apps, Discover Apps**, y luego **Install via YAML** en el menú junto a **Custom App**. El YAML de abajo define lo mismo que la entrada del catálogo en la carpeta `truenas-apps` del repositorio. Sustituye `tank` por tu pool y rellena `APP_KEY` como describe la siguiente sección.

```yaml
services:
  bombvault:
    image: junkerderprovinz/bombvault:latest
    hostname: bombvault
    restart: unless-stopped
    cap_drop: [ALL]
    cap_add: [CHOWN, DAC_OVERRIDE, FOWNER, SETGID, SETPCAP, SETUID, SYS_CHROOT]
    environment:
      APP_KEY: ""
      PLATFORM: truenas
      HOST_SOURCE_ROOT: /mnt/tank
      HOST_MOUNT_ROOT: /mnt/tank
      # For VM backups:
      # LIBVIRT_URI: qemu+ssh://root@<truenas-ip>/system?socket=/run/truenas_libvirt/libvirt-sock
    volumes:
      - /mnt/tank/apps/bombvault:/config
      - /var/run/docker.sock:/var/run/docker.sock
      - type: bind
        source: /mnt/tank
        target: /mnt/tank
        bind:
          propagation: rslave
    ports:
      - "30528:3443"
```

Después abre `https://<truenas-ip>:30528/`. Lo que figura en [Qué es distinto respecto a Unraid](#generic-docker-host) se aplica también aquí, y en TrueNAS:

- **Capacidades.** BombVault necesita las siete de arriba. Las copias de VMs fallan sin `SETPCAP`, y sin `SYS_CHROOT` cada contenedor se detiene durante toda su copia en lugar de leerse de una instantánea.
- **Host Data** es una ruta del host en un conjunto de datos, aquí `/mnt/tank`, montada en la misma ruta dentro de la app. Por eso `HOST_SOURCE_ROOT` y `HOST_MOUNT_ROOT` la nombran las dos. `rslave` pasa los montajes que el host hace más tarde, que las copias de conjuntos de datos ZFS necesitan.
- **Los datos en un ixVolume no se copian.** Una app cuyo almacenamiento es un ixVolume guarda sus datos bajo `/mnt/.ix-apps`, fuera de Host Data. BombVault guarda solo la definición de la app, y la ejecución nombra las carpetas que dejó fuera. Para cada app cuyos datos quieras copiar, elige almacenamiento **Host Path** en un conjunto de datos bajo Host Data.
- **Las copias de VMs y de conjuntos de datos ZFS** llegan a TrueNAS por SSH como root. Activa el servicio SSH en **System, Services**, y añade la clave pública de **Ajustes, Integraciones, SSH del host** al usuario root en **Credentials, Users**. Las copias de VMs también necesitan `LIBVIRT_URI` como arriba, porque el libvirtd de TrueNAS escucha en un socket propio. Sin `LIBVIRT_URI` ni `LIBVIRT_HOST` definidos, BombVault usa como host la puerta de enlace predeterminada de la app, porque `host.docker.internal` no se resuelve en una app de TrueNAS. La [guía de copias de VMs](vm-backup-ssh-setup.md#truenas-scale) cubre los detalles, incluidas las VMs en zvols.
- **Una réplica ZFS usa otra clave.** Una réplica en un segundo servidor ZFS inicia sesión con la clave que muestra el diálogo de la réplica, no con la clave SSH del host. Añade esa en el servidor receptor.
- **Una app borrada vuelve en dos pasos.** Reinstálala primero en TrueNAS con el mismo nombre, lo que vuelve a crear su red, y después restaura sus datos en BombVault. Hasta entonces la comprobación de restauración informa de la red que falta, y la restauración se niega antes de cambiar nada.
- **Junto al original** crea un contenedor Docker normal. Funciona, pero la pantalla Apps de TrueNAS no lo lista.
- **Los volcados de bases de datos** funcionan en montajes de conjuntos de datos. Una importación en una carpeta de datos que es un conjunto de datos propio mueve el contenido antiguo a una carpeta a su lado, así que el conjunto de datos de arriba necesita espacio para ello.

### ZimaOS, CasaOS y otros hosts {#other-hosts}

- **ZimaOS y CasaOS:** añade la [tienda de la comunidad](https://github.com/junkerderprovinz/zimaos-apps#install) e instala BombVault desde ella. Allí se ejecuta como `PLATFORM=generic` con Host Data en `/DATA`. ZimaOS guarda los datos de las apps en `/DATA/AppData/<app>`, que BombVault encuentra por sí solo, porque `DATA_ROOT_SEGMENTS` coincide con `appdata` en cualquier combinación de mayúsculas.
- **MOS:** instala BombVault desde el [MOS Hub](https://mos-official.net/apps.html?search=BombVault). Allí se ejecuta como `PLATFORM=generic` con Host Data en `/mnt/cache`.
- **Synology:** en Container Manager, crea un proyecto a partir del archivo compose (DSM 7.2 o posterior).
- **Proxmox:** ejecútalo en una VM o en un contenedor LXC que tenga Docker.

Sin comentarios, el archivo compose genérico es corto. Sustituye `./data` por la carpeta del host que contiene los datos de tus contenedores, pon `HOST_SOURCE_ROOT` en esa misma ruta y rellena `APP_KEY`:

```yaml
services:
  bombvault:
    image: junkerderprovinz/bombvault:latest
    restart: unless-stopped
    hostname: bombvault
    container_name: bombvault
    volumes:
      - ./config:/config
      - /var/run/docker.sock:/var/run/docker.sock
      - ./data:/host/user
    environment:
      APP_KEY: ""
      PLATFORM: generic
      HOST_SOURCE_ROOT: /host/user
    ports:
      - "3443:3443"
```

Para copiar también los volúmenes con nombre de Docker, monta una carpeta que contenga tanto tus datos como la raíz de datos de Docker (`docker info -f '{{.DockerRootDir}}'`, normalmente `/var/lib/docker`), como hace la plantilla de Unraid al montar todo `/mnt`.

## El único ajuste obligatorio {#the-one-required-setting}

La única variable que debes definir es `APP_KEY`, un secreto hexadecimal de 32 bytes (64 caracteres hex) del que se deriva la contraseña del repositorio restic.

Genera uno en cualquier equipo:

```bash
openssl rand -hex 32
```

Pega el resultado en el campo `APP_KEY` de la plantilla (Unraid), o en la variable de entorno `APP_KEY` de `docker-compose.yml` (host Docker genérico).

!!! danger "No pierdas tu APP_KEY"
    Perder `APP_KEY` deja tus copias cifradas irrecuperables. Guárdala en un lugar seguro y separado del servidor. Cuando BombVault esté en marcha, define una contraseña de acceso y descarga el **kit de recuperación** en **Ajustes, Restauración** (consulta [Copias externas y recuperación](offsite-recovery.md#recovery-kit)). Contiene la clave, todos los destinos y los comandos de restic.

La plantilla también monta por ti el socket de Docker, el flash (`/boot`) y la raíz de **Host Data** (`/mnt`). Los *orígenes* y los *destinos* de las copias viven ambos bajo Host Data. Para la referencia completa de variables y la configuración de copias externas, consulta [Configuración](configuration.md).

## Primera ejecución {#first-run}

![La página de bienvenida de una instalación nueva, con lo que BombVault encontró en el servidor.](assets/screenshots/getting-started/welcome-dark.png#only-dark)
![La página de bienvenida de una instalación nueva, con lo que BombVault encontró en el servidor.](assets/screenshots/getting-started/welcome-light.png#only-light)

1. Abre la interfaz web en `https://<tu-ip-de-unraid>:3443` (certificado autofirmado de serie). Una instalación nueva se abre en **Te damos la bienvenida a BombVault**, que lista lo que BombVault encontró en el servidor.
2. Elige **Vamos allá**. El resumen muestra entonces la tarjeta **Primeros pasos** con tres pasos:
    1. **Elige las secciones**: activa lo que copia BombVault (Contenedores, VMs, Flash, Carpetas, Conjuntos de datos ZFS, Autocopia). **Elegir** abre la lista de elementos de una sección, donde **Pausar programación** deja uno fuera. Las secciones que actives aquí se ejecutan cada noche, algo que cambias en **Ajustes, Calendario**.
    2. **Añadir destino**: un segundo lugar para las copias. Cada sección ya tiene una carpeta en este servidor, `/mnt/user/bombvault/{container,vms,flash,config,files,zfs}`, creada en la primera copia. Lo más seguro es un segundo destino fuera de las instalaciones, como una Storage Box o Backblaze B2.
    3. **Primera copia**: después se ejecuta sola cada noche. La tarjeta se queda hasta que termina la primera copia.
3. Abre **Copias de seguridad** para ver todos los elementos en una sola lista. **Copiar todo ahora** inicia una ejecución sobre todas las secciones activadas, y **Copiar ahora** en la página de un elemento copia solo ese.

![La tarjeta Primeros pasos con sus tres pasos, el segundo abierto.](assets/screenshots/getting-started/first-steps-dark.png#only-dark)
![La tarjeta Primeros pasos con sus tres pasos, el segundo abierto.](assets/screenshots/getting-started/first-steps-light.png#only-light)

¿Ya usabas BombVault? **Restaurar desde una copia** en la página de bienvenida abre la página Restauración, donde el **Asistente de emergencia** recupera los ajustes, contenedores, VMs y carpetas de un servidor perdido. Consulta [Copias externas y recuperación](offsite-recovery.md#emergency).

!!! tip "Opcional: elegir un orden de copia"
    Si algunos contenedores deben copiarse siempre antes que otros (por ejemplo una base de datos antes de la app que la usa), abre la tarjeta **Orden** en **Ajustes, Calendario** y arrástralos a la secuencia que quieras. Las ejecuciones programadas y la copia total la siguen entonces. Lo que dejes sin ordenar se copia empezando por lo más atrasado.

## Ejemplo: tu primera copia de un contenedor, de principio a fin {#example-first-backup}

Esto recorre un equipo Unraid recién instalado con unos cuantos contenedores, entre ellos `plex`, hasta que la primera copia está hecha y sabes dónde encontrarla. Lleva unos diez minutos más lo que tarde la primera copia.

1. **Instala.** En la pestaña **Apps** de Unraid, busca **BombVault** y pulsa **Install**.
2. **Pon la clave.** En un ordenador, ejecuta `openssl rand -hex 32` y pega los 64 caracteres en el campo `APP_KEY` de la plantilla. Guarda ya una copia en tu gestor de contraseñas, antes que nada. Pulsa **Apply**.
3. **Ábrelo.** Ve a `https://<tu-ip-de-unraid>:3443` y acepta el certificado autofirmado. **Te damos la bienvenida a BombVault** lista lo que encontró, por ejemplo 12 contenedores, 2 VMs y la unidad flash.
4. **Vamos allá.** El resumen se abre con la tarjeta **Primeros pasos**.
5. **Elige las secciones.** **Contenedores** está activada desde el principio. Activa lo demás que quieras, como **Flash**. **Elegir** abre la lista de contenedores que encontró, donde **Pausar programación** deja uno fuera, por ejemplo un contenedor de prueba que no te importa. Las secciones activadas se ejecutan cada noche en la copia total.
6. **Añadir destino.** Cada sección ya tiene su propia carpeta en este servidor, bajo `/mnt/user/bombvault`, así que la primera copia tiene adónde ir. Este paso añade un segundo lugar, y el más seguro está fuera de las instalaciones. Puedes hacerlo ahora con el [ejemplo de S3](offsite-recovery.md#example-s3), o volver a ello cuando termine la primera copia. La tarjeta se queda hasta entonces.
7. **Primera copia.** Abre **Copias de seguridad** y pulsa **Copiar todo ahora**, o espera a la noche. El anillo de la cabecera de estado se convierte en un anillo de progreso y cuenta los elementos, por ejemplo **Copiando ahora: 3 de 12**. Cada contenedor se detiene mientras se leen sus datos y se vuelve a iniciar justo después. Un contenedor cuyos datos están en ZFS está parado solo los pocos segundos de una instantánea. La primera copia lo lee todo, así que es la que más tarda. Las siguientes guardan solo lo que cambió.
8. **Compruébalo.** Cuando el anillo dice **Todo copiado**, abre **Historial** en el resumen: cada contenedor tiene una línea verde en **Ejecuciones**. Abre **Copias de seguridad**, haz clic en `plex` y mira su pestaña **Copias de seguridad**: la primera instantánea está ahí, con **Local** como el lugar que la contiene. Unos minutos después su pestaña **Verificación** muestra la comprobación de restauración, que restauró una muestra de la copia y la volvió a leer.
9. **Protege la clave.** Abre **Ajustes, Seguridad** y define una contraseña de acceso. Luego abre **Ajustes, Restauración** y pulsa **Descargar kit de recuperación** en **Para emergencias**. Guarda el archivo fuera del servidor, junto a la copia de `APP_KEY` del paso 2. El recordatorio **Guarda tu kit de recuperación** del resumen desaparece cuando pulsas **Lo he guardado de forma segura**.

A partir de ahora los contenedores se copian cada noche. Buenos siguientes pasos, en este orden: un [destino externo](offsite-recovery.md#example-s3), para que un servidor muerto no se lleve las copias consigo, luego las [notificaciones](notifications.md), para que un fallo te llegue, y después un vistazo a los [Calendarios](schedules.md) si las 03:00 no te vienen bien.

![La cabecera de estado mientras se ejecuta la primera copia, con el anillo de progreso y sus estaciones.](assets/screenshots/getting-started/status-ring-dark.png#only-dark)
![La cabecera de estado mientras se ejecuta la primera copia, con el anillo de progreso y sus estaciones.](assets/screenshots/getting-started/status-ring-light.png#only-light)

!!! note "Comprobación de la integración con el host"
    La tarjeta **Integración con el host** del resumen sondea cada montaje y cada CLI (socket de Docker, libvirt, restic, qemu-img, rclone) e informa de lo que falte, para que puedas confirmar que el contenedor está bien conectado antes de depender de él. Espera en **Personalizar** hasta que la añadas.

## Orientarse {#navigation}

![Los ajustes como mosaicos, con las tarjetas del mosaico elegido a su lado. Cada cambio se escribe en cuanto lo haces.](assets/screenshots/getting-started/settings-rail-dark.png#only-dark)
![Los ajustes como mosaicos, con las tarjetas del mosaico elegido a su lado. Cada cambio se escribe en cuanto lo haces.](assets/screenshots/getting-started/settings-rail-light.png#only-light)

La barra lateral tiene cinco entradas: **Resumen**, **Anomalías** (mientras la detección de anomalías está activada), **Copias de seguridad**, **Restauración** y, debajo de **Cerrar sesión**, **Ajustes**. La acción principal de una página flota abajo a la derecha, y **Más** contiene las poco frecuentes. En un teléfono la barra contiene Resumen, Copias de seguridad, Restauración y Ajustes, y **Más** tiene el resto.

Las páginas y los ajustes usan todo el ancho de la ventana. Una ventana guarda sus botones en el pie, con el que avanza a la derecha. En un teléfono, un pie con más de tres botones conserva la salida y la acción principal y pone los demás en **Más**.

![En un teléfono, la barra inferior con Más abierto.](assets/screenshots/getting-started/phone-more-dark.png#only-dark)
![En un teléfono, la barra inferior con Más abierto.](assets/screenshots/getting-started/phone-more-light.png#only-light)

- **Resumen** muestra si todo está copiado y qué necesita atención. Consulta [Resumen](overview.md).
- **Copias de seguridad** lista como elementos cada contenedor, VM, conjunto de carpetas, la unidad flash, cada conjunto de datos ZFS y la autocopia. Los mosaicos sobre la lista eligen un tipo, y **Filtros**, **Ordenar** y **Buscar** la acotan. Cada elemento tiene una página propia con tres pestañas: **Copias de seguridad**, **Ajustes** y **Verificación**. Un botón de retroceso a la izquierda del icono del elemento lleva a la lista. Consulta [Copias de seguridad y elementos](backups.md).
- **Restauración** es una sola página para todo: elige una sección, busca un elemento y escoge una copia. El Asistente de emergencia y las otras fuentes también empiezan aquí. Consulta [Restauración](restore.md).
- **Ajustes** son catorce mosaicos: General, Aspecto, Calendario, Destinos, Restauración, Notificaciones, Verificación, Seguridad, Instancias, Emparejamiento, Integraciones, Apps, Sistema e Info. Consulta [Ajustes](settings.md). Pulsa `/` o Ctrl+K para buscar en todos, también por los nombres que tenían las páginas en versiones anteriores. Arrastra los mosaicos a tu propio orden en **Aspecto**. Donde una columna de mosaicos junto a la página la apretaría, como en una ventana de unos 1200 píxeles de ancho, los mosaicos se convierten en una tira sobre la página que se desplaza hacia los lados. Un teléfono muestra la misma tira.
- **Destinos** e **Instancias** son mosaicos de ajustes con páginas propias. **Mostrar también en la barra lateral** en su mosaico los ancla a la barra lateral.

Nada tiene un botón Guardar y nada se despliega: las cosas poco habituales tienen una tarjeta propia o una ventana, y las explicaciones están en los globos (i). Tras una actualización se abre sola una ventana con una tarjeta por cada versión desde tu última versión, y unos puntos verdes marcan lo que cambió. Un clic oculta un punto. **Novedades** en la tarjeta Versión de **Ajustes, Info** cuenta lo que aún no has visto y vuelve a abrir esa ventana, con las notas de cada versión y sus cambios con **Ir allí**.

## Compilar desde el código fuente {#build-from-source}

BombVault es un único binario estático de Go que sirve una API JSON y una interfaz React incrustada. Compila primero la interfaz y luego ejecuta el binario:

```bash
npm --prefix web ci
npm --prefix web run build     # writes web/dist, which the binary embeds
export APP_KEY=$(openssl rand -hex 32)
go test ./...                  # unit and integration tests, with a real restic round trip
golangci-lint run ./...
go run ./cmd/bombvault         # serves https://localhost:3443 with a self-signed certificate
```

La compilación de la interfaz también hace falta para `go run`. El repositorio solo rastrea un marcador vacío bajo `web/dist`, así que sin `npm --prefix web run build` el binario no incrusta nada y responde `500 SPA index not found`, algo esperado. Docker, libvirt y Unraid no se pueden probar en CI, así que comprueba los montajes, restic y el enlace SSH de las VMs en un host real con la tarjeta **Integración con el host** del resumen (o `GET /api/spike`) antes de abrir una pull request.

## Siguientes pasos {#next-steps}

- Añade uno o más destinos fuera de las instalaciones (cada sección puede copiar a varios a la vez) y guarda tu kit de recuperación, como se describe en **[Copias externas y recuperación](offsite-recovery.md)**. El [ejemplo de S3](offsite-recovery.md#example-s3) recorre uno.
- Asegúrate de que un fallo te llegue con las **[Notificaciones](notifications.md)**.
- Revisa cuándo se ejecuta cada cosa en **[Calendarios](schedules.md)**.
- Lleva a tu teléfono todos los servidores de tu grupo con la **[app de Android](android.md)**.
- Los **[ejemplos prácticos](guides.md)** recorren los trabajos habituales, y **[Funciones](features.md)** lista todo lo que hace BombVault.
- ¿Clonas una configuración o te mudas a un equipo nuevo? Lleva toda tu configuración con la tarjeta **Exportar / importar ajustes**. Consulta [Configuración](configuration.md#portable-settings-export-and-import).
- ¿Te has atascado? Consulta **[Solución de problemas](troubleshooting.md)**.
