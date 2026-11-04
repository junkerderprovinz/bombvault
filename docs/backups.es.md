# Copias de seguridad y elementos

Todo lo que BombVault copia está en una sola lista en **Copias de seguridad**: contenedores, VMs, conjuntos de carpetas, la unidad flash, conjuntos de datos ZFS y la autocopia. Cada uno de ellos es un **elemento** con su propia página. Esta página recorre la lista, la página de un elemento con sus tres pestañas y los ajustes que puede tener un elemento.

![La lista de Copias de seguridad con el mosaico Contenedores elegido: cada fila muestra sus últimas ejecuciones como una tira y un único estado.](assets/screenshots/backups/list-dark.png#only-dark)
![La lista de Copias de seguridad con el mosaico Contenedores elegido: cada fila muestra sus últimas ejecuciones como una tira y un único estado.](assets/screenshots/backups/list-light.png#only-light)

## Qué guarda cada sección {#what-is-saved}

| Sección | Qué se guarda |
|---|---|
| **Contenedores** | La carpeta appdata más la definición del contenedor: imagen, variables, puertos, etiquetas y volúmenes. La tarjeta **Carpetas a copiar** de la pestaña **Ajustes** del contenedor marca exactamente qué carpetas cubre la copia, con un recuento en vivo de las rutas y una lista de lo que dejaste fuera. Los datos que un contenedor guarda fuera de los datos del host, por ejemplo en un ixVolume de TrueNAS, no se copian, y la ejecución nombra las carpetas que dejó fuera. |
| **VMs** | Las imágenes de disco, la definición XML y la NVRAM UEFI, con un apagado ordenado o una instantánea en vivo, por SSH. Consulta [Copias de VMs](vm-backup-ssh-setup.md). |
| **Flash** | Solo Unraid. Todo el USB flash (`/boot`): sistema operativo, licencia, configuración del array, recursos compartidos, red y configuración de plugins. |
| **Autocopia** | El propio `/config` de BombVault: su base de datos de ajustes, las credenciales de los destinos y el par de claves SSH de libvirt, con una instantánea hecha con SQLite `VACUUM INTO` para que la base de datos nunca se capture a mitad de una escritura. |
| **Carpetas** | Cualquier carpeta del servidor como conjunto de carpetas, por ejemplo un recurso compartido, tus documentos o una biblioteca de fotos, con patrones de exclusión opcionales. Los conjuntos de carpetas tienen calendarios, retención, copias externas, comprobaciones y simulacros de restauración como todo lo demás. |
| **Conjuntos de datos ZFS** | Un conjunto de datos y todos los que tiene debajo, leídos de una sola instantánea ZFS para que todos procedan del mismo instante, y guardados con restic como una carpeta. Un elemento ZFS también puede mantener una réplica exacta a nivel de bloque en un segundo servidor ZFS. Consulta [Conjuntos de datos ZFS](zfs-datasets.md). |

Toda copia pasa por restic, así que es incremental y deduplicada, y ni siquiera los discos de VM grandes hinchan el repositorio. BombVault nunca copia, y por tanto nunca detiene, su propio contenedor. Una ejecución se puede detener sin riesgo, porque restic escribe su instantánea al final, así que una ejecución abortada deja datos sin referencia y ninguna instantánea. Un bloqueo que deja un reinicio a mitad de una ejecución se limpia y la ejecución se intenta una vez más, y solo cuando el bloqueo lleva diez minutos sin renovarse.

## La lista {#list}

Los mosaicos sobre la lista eligen un tipo, como **Contenedores** o **ZFS**, y cada mosaico cuenta sus elementos. Un mosaico selecciona, no oculta el resto de la página.

Cada fila muestra las últimas catorce ejecuciones del elemento como pequeños cuadrados y un único estado, como **Protegido**, **Sin protección**, **Aplazado** o **Replicando**. Todo lo demás está en el globo de información junto a su nombre, y mientras el puntero descansa sobre la fila ese texto aparece en la propia fila. Una marca junto al nombre de un contenedor indica que se recreó con otra imagen, otros puertos, variables o volúmenes desde su última copia. Desaparece con la siguiente copia.

Tres botones sobre la lista la acotan:

- **Filtros**: por **Calendario** (**Todos**, **Programado**, **En pausa**), por **Copia** (**Con copia**, **Sin copia**) y, para **Contenedores y VMs**, **Instalados** o **No instalado**.
- **Ordenar**: por nombre, estado, IP, última copia, tamaño o calendario. Todos los campos salvo IP ordenan en ambos sentidos, como **Tamaño, primero las más grandes** y **Tamaño, primero las más pequeñas**.
- **Buscar**: por nombre, imagen o IP.

Los elementos que ya no están instalados pero aún tienen copias aparecen aparte bajo **No instalado (solo copias)**. Restáuralos o borra sus copias para liberar el espacio.

La acción principal de la página flota abajo a la derecha: **Copiar todo ahora** inicia una ejecución sobre todas las secciones activadas. **Más** contiene el resto:

- **Seleccionar** inicia una selección, mira más abajo.
- **Descubrir copias** lee lo que guardan los destinos y reconstruye elementos a partir de ello, por ejemplo tras una instalación nueva. Las copias externas de las secciones reconstruidas se pausan hasta que confirmes su valor predeterminado, consulta [Después de una reconstrucción](offsite-recovery.md#after-a-rebuild).
- **Conexión ZFS con este servidor** abre la prueba SSH de la sección ZFS, consulta [Conjuntos de datos ZFS](zfs-datasets.md#requirements).
- **Replicar todo ahora**, con el mosaico ZFS elegido, envía la réplica de cada elemento ZFS.

![La misma lista en un teléfono.](assets/screenshots/backups/list-phone-dark.png#only-dark)
![La misma lista en un teléfono.](assets/screenshots/backups/list-phone-light.png#only-light)

## Elegir varios elementos {#select}

Elige filas como eliges archivos en un gestor de archivos: Ctrl+clic, Mayús+clic, Ctrl+A o una pulsación larga en un teléfono. Una fila elegida muestra una marca en lugar de su icono, y una barra sobre la lista las cuenta y ofrece **Copiar selección**, **Pausar programación** y **Reanudar programación**, **Seleccionar todo** y **Quitar selección**. Cada fila también tiene un menú de clic derecho con sus propias acciones.

Una selección se copia en el servidor, un elemento detrás de otro, así que sigue adelante aunque cierres la pestaña. Para restaurar varios elementos a la vez, usa la [página Restauración](restore.md#several).

![Dos contenedores elegidos, con la barra sobre la lista.](assets/screenshots/backups/bulk-dark.png#only-dark)
![Dos contenedores elegidos, con la barra sobre la lista.](assets/screenshots/backups/bulk-light.png#only-light)

## Añadir una carpeta o un conjunto de datos ZFS {#add}

Los contenedores, las VMs y la unidad flash aparecen solos cuando su sección está activada en **Ajustes, General**. Las carpetas y los conjuntos de datos ZFS los eliges tú, porque BombVault no puede adivinar cuáles quieres. **Añadir** abre **¿Qué más debe copiar BombVault?**:

- **Una carpeta** lista los recursos compartidos que BombVault encontró y acepta cualquier otra carpeta bajo los datos del host. Ponle un nombre al conjunto de carpetas, como `Documents`. El nombre queda fijo tras la primera copia, porque las instantáneas se guardan bajo él.
- **Un conjunto de datos ZFS** lista los conjuntos de datos de este servidor. Elige el que está en lo más alto de lo que quieres copiar, consulta [Elementos y conjuntos de datos hijos](zfs-datasets.md#items-and-children).
- **Configuración del sistema** aparece en un host sin Unraid cuyos datos del host llegan a `/etc`. Es un conjunto de carpetas `/etc` de partida que revisas antes de guardar.

## Dejar fuera un elemento {#dont-back-up}

Un elemento que dejas fuera a propósito, como un contenedor de prueba, se puede marcar como **No copiar**. Entonces deja de contar como sin protección, y **Dejar de excluir** lo devuelve. **Pausar programación** es la forma más suave: el elemento sale del calendario y de la copia total, lleva una insignia gris **Programación en pausa**, y **Copiar ahora** sigue funcionando. Activa el mismo interruptor que **En el calendario** en la pestaña Ajustes del elemento.

## La página de un elemento {#entry-page}

Un clic en una fila abre la página del elemento. La cabecera nombra el elemento y su estado en una línea, como **Última copia de seguridad hace 3 horas** o **Aplazada, próximo intento a las 03:15**, y contiene sus acciones: **Copiar ahora**, **Restaurar**, y bajo **Más** las poco frecuentes, como **Exportar la copia más reciente**, **Exportar contenedor** o **Exportar VM**, **Vincular con un elemento anterior**, **Eliminar todas las copias** y **Eliminar elemento**. Un botón de retroceso a la izquierda del icono lleva a la lista. Mientras se ejecuta una copia, **Cancelar copia** está junto a su progreso, y una ejecución cancelada se registra como cancelada, no como fallida.

La página tiene tres pestañas: **Copias de seguridad**, **Ajustes** y **Verificación**.

### Pestaña Copias de seguridad {#backups-tab}

Esta pestaña lista las instantáneas del elemento, las más recientes primero, con una barra por día. Una copia que está en varios sitios aparece una vez, marcada con cada destino que la contiene. Un clic en una instantánea la restaura, y un clic en un lugar dentro de una instantánea la restaura exactamente desde ahí.

- **Destinos** nombra cada lugar con copias de este elemento y cuántas. Los lugares externos solo se leen cuando pulsas **Comprobar**, porque listar el almacenamiento en la nube lleva un rato. Un lugar al que el elemento ya no copia aparece como **Solo copias anteriores** y todavía se puede restaurar desde él.
- Cada instantánea tiene un menú: **Añadir etiqueta** o **Cambiar etiqueta**, **Comparar con otra copia**, que lista los archivos que llegaron, se fueron o cambiaron, **Exportar** como tar, zip o un repositorio restic propio, y borrar en un solo lugar.
- Los **Volcados de la base de datos** de un contenedor están en una lista aparte, con descarga, guardado en una carpeta e importación.

Cuántas instantáneas se conservan lo fija el destino, consulta [Cuánto tiempo conservar](offsite-recovery.md#keep).

![La pestaña Copias de seguridad de un contenedor: una barra por día y los destinos que contienen cada instantánea.](assets/screenshots/backups/entry-backups-dark.png#only-dark)
![La pestaña Copias de seguridad de un contenedor: una barra por día y los destinos que contienen cada instantánea.](assets/screenshots/backups/entry-backups-light.png#only-light)

### Pestaña Ajustes {#settings-tab}

Cada elemento empieza con **Calendario y destino**:

- **En el calendario** mete o saca el elemento de las ejecuciones programadas.
- **Calendario propio** da a este elemento un calendario propio, consulta [Un calendario propio para un elemento](schedules.md#own-plan). Sin él, se ejecuta según el calendario de su sección o en la copia total.
- Una fila de botones, **Local** y uno por cada destino externo, indica adónde van las copias. **Sigue el valor predeterminado** significa que el elemento va adonde va su sección, y un clic en él abre **Destinos por sección**. Consulta [Destino por elemento](offsite-recovery.md#placement).
- **Repositorio** nombra dónde se escriben las copias (**Almacenado en**). Ese lugar queda fijo desde la primera copia, las réplicas pueden cambiar en cualquier momento.

Las tarjetas de abajo dependen del tipo de elemento:

| Tipo | Tarjetas |
|---|---|
| Contenedor | **Carpetas a copiar** (qué carpetas asignadas se copian, con **Omitir carpetas de caché (CACHEDIR.TAG)**), **Patrones de exclusión**, **Detener otros contenedores**, **Comandos antes y después**, **Base de datos**, **Cuando la app está ocupada**, **Actualizar tras una copia de seguridad correcta**, **Instantánea en vez de parada**, **Nombres anteriores** |
| VM | **VM** con **Método** (**Ordenado (apagado)** o **Instantánea en vivo**) y **Solo bloques modificados**, consulta la [guía de copias de VMs](vm-backup-ssh-setup.md#per-vm-method) |
| Conjunto de carpetas | **Carpeta** y **Patrones de exclusión** |
| Unidad flash | **Exportación de Flash a zip**, que escribe cada copia del flash como un `.zip` normal en una carpeta que eliges |
| Conjunto de datos ZFS | los conjuntos de datos hijos, los contenedores que detener y los comandos alrededor de la instantánea, **Réplica** y **Probar el acceso a instantáneas**, consulta [Conjuntos de datos ZFS](zfs-datasets.md) |

Los ajustes se guardan solos conforme los cambias.

![La pestaña Ajustes de un contenedor, con Calendario propio en la primera tarjeta.](assets/screenshots/backups/entry-settings-dark.png#only-dark)
![La pestaña Ajustes de un contenedor, con Calendario propio en la primera tarjeta.](assets/screenshots/backups/entry-settings-light.png#only-light)

### Pestaña Verificación {#verify-tab}

- **Comprobaciones** muestra la comprobación de restauración que sigue a la primera copia. **Comprobar restauración** la ejecuta de nuevo sobre la copia más reciente: BombVault restaura una muestra de hasta 100 archivos y 256 MiB en una carpeta temporal y hace que restic relea cada archivo contra sus hashes.
- **Prueba de arranque**, en un contenedor, restaura la copia más reciente en una copia aislada y la arranca: un nombre que empieza por `bombvault-test-`, una red interna de Docker propia sin puertos publicados y sin camino a la LAN, 1 CPU y 2 GiB de memoria. Pasa cuando la copia informa de que está sana, responde en su primer puerto expuesto o, si no hace ninguna de las dos cosas, sigue en ejecución. El original nunca se toca, y la copia, su red y sus datos se eliminan después, también tras un reinicio a mitad de una prueba. Los contenedores en la red del host, los privilegiados, los que tienen dispositivos y los que necesitan otro contenedor se muestran como no comprobables. **Prueba de arranque** en **Ajustes, Verificación, Comprobaciones automáticas** prueba un contenedor por ejecución, primero el que se probó hace más tiempo.
- **Anomalías** fija la sensibilidad y el nivel de notificación propios de este elemento y muestra sus hallazgos abiertos.
- **Tamaño por carpeta** muestra qué ocupa el espacio en la copia más reciente y cuánto aportó la última copia, un nivel cada vez.
- **Ejecuciones recientes** lista las últimas ejecuciones con su inicio, fin y duración. Un clic muestra los detalles.

![La pestaña Verificación de un contenedor con Tamaño por carpeta abierto.](assets/screenshots/backups/entry-verify-dark.png#only-dark)
![La pestaña Verificación de un contenedor con Tamaño por carpeta abierto.](assets/screenshots/backups/entry-verify-light.png#only-light)

## Patrones de exclusión {#excludes}

La tarjeta **Patrones de exclusión** acepta un patrón por línea. Escribe las rutas como las ves dentro del contenedor, como `/config/Library/Application Support/Plex Media Server/Cache`, o un nombre suelto como `.git`, que coincide a cualquier profundidad. Una vista previa bajo el campo muestra a qué se resuelve cada línea y avisa cuando una línea no coincide con nada o excluiría todo. Una línea con `/` o `*`, o una que nombre una carpeta copiada entera, se ignora, así que una exclusión nunca puede vaciar una copia. Para dejar fuera una carpeta entera, desmárcala mejor en **Carpetas a copiar**.

Para imágenes conocidas como Plex, Jellyfin, Sonarr, Radarr, Immich y Nextcloud, la tarjeta ofrece las carpetas que la app vuelve a llenar por sí sola, como cachés, registros y miniaturas. No se excluye nada hasta que pulsas **Excluir selección**.

La misma tarjeta lleva avisos que un análisis de carpetas no puede ver. Immich, por ejemplo, guarda sus álbumes, caras y fechas en una base de datos PostgreSQL en un contenedor aparte, así que una copia de archivos solo del contenedor de Immich restaura las fotos sin nada de eso, y la restauración parece haber funcionado. Copia también el contenedor de la base de datos.

## Volcados de bases de datos {#database-dumps}

Los contenedores PostgreSQL, MySQL y MariaDB que BombVault reconoce se vuelcan antes de cada copia, desde el servidor en ejecución, a un punto de restauración propio junto a los archivos. Eso cubre las imágenes oficiales, PostGIS, TimescaleDB, pgvector, pgautoupgrade, las imágenes de base de datos de Immich, las imágenes MariaDB de linuxserver, yobasystems y jc21 y mysql-server de Oracle. Un contenedor que solo se parece a una base de datos recibe la misma opción, desactivada hasta que la elijas.

El volcado fluye directo al repositorio y nunca se escribe en un disco. Las credenciales salen de las variables del propio contenedor, incluidos los secretos `*_FILE`, y nunca lo abandonan. La tarjeta **Base de datos** de la pestaña Ajustes del contenedor indica si la carpeta de datos se guarda con el contenedor detenido, se copia en ejecución o no se guarda, y activa o desactiva el volcado para este contenedor. La etiqueta `bombvault.dbdump=false` hace lo mismo y prevalece sobre la tarjeta. **Volcados automáticos de bases de datos** en **Ajustes, General** lo activa o desactiva para todos.

Un volcado fallido no hace fallar la copia. Aparece como una ejecución fallida propia con el motivo y envía una notificación. Los volcados nunca se cargan de vuelta por sí solos: descarga uno, sin comprimir o comprimido, guárdalo en una carpeta o impórtalo en una base de datos recién iniciada con un clic en la pestaña **Copias de seguridad**. Para recuperar un volcado sin BombVault, consulta [Recuperar un volcado de base de datos](offsite-recovery.md#database-dumps). Cuando uno falla, consulta [Falló un volcado de base de datos](troubleshooting.md#a-database-dump-failed).

## Comandos y otros contenedores {#commands}

- **Comandos antes y después** se ejecutan dentro del contenedor con `sh -c`, por ejemplo para volcar una caché al disco. Un comando fallido antes de la copia la aborta. Un comando antes de la copia que termina con el código 75 aplaza una copia programada en su lugar, mira más abajo.
- **Detener otros contenedores** nombra contenedores que detener mientras se copia este, como su base de datos. Después vuelven a arrancar en su orden `depends_on` de Compose, y de forma predeterminada cada uno tiene que informar de que está sano antes de que arranque el siguiente. La espera y su tiempo límite están en **Reinicio tras la copia de seguridad** en la tarjeta **Orden** de **Ajustes, Calendario**.

## Cuando la app está ocupada {#app-busy}

Una copia detiene su contenedor, así que una programada puede esperar mientras alguien usa la app y volver a intentarlo por sí sola. La tarjeta **Cuando la app está ocupada** de la pestaña Ajustes de un contenedor lo configura:

- **Comprobación de actividad** pregunta a Plex, Jellyfin o Emby antes de una copia programada si alguien está viendo algo. A Plex se le pregunta con el token de su propio `Preferences.xml`, así que no hay nada que introducir. Jellyfin y Emby aceptan una **Clave de API**, creada en el panel del servidor en Claves de API.
- **Ignorar sesiones en pausa tras (min)** deja que una película en pausa durante la noche deje de contar.
- **Copiar cuando el servidor no responde** decide qué pasa cuando no se puede preguntar al servidor: activado, la copia se ejecuta y el registro lo dice. Desactivado, espera como si alguien estuviera viendo algo.
- **Esperar mientras el contenedor envía datos** es para apps sin esa comprobación, como un servidor de juegos o un descargador. La copia espera mientras el contenedor envió más de **Ocupado a partir de (KiB/s)** en cualquier lectura dentro de los últimos **Tranquilo durante (min)** minutos.
- **Reintentar cada (min)**, **Intentar solo dentro de una franja horaria** con **Intentar entre**, y **Esperar como máximo (horas)** fijan cómo van los intentos.
- **Al final de la espera** o bien se rinde, lo que muestra la copia en **Necesita atención** con **Reintentar**, o bien **Copiar de todos modos**.

Una copia en espera se registra como **Aplazado**, en ámbar, y el resto de la ejecución sigue adelante. El resumen, la lista y el elemento lo muestran con el siguiente intento, y la alerta de retraso y el estado de protección lo esperan hasta su plazo. Recibes una notificación cuando se aplaza y otra cuando termina, no una por intento. Un intento espera mientras se ejecuta otra copia, y una espera continúa tras un reinicio. Una copia que inicias tú siempre se ejecuta de inmediato, ya sea con **Copiar ahora**, **Reintentar**, una selección, la API o MCP, y una que sale adelante termina una espera pendiente.

La pestaña Ajustes del contenedor también tiene una forma más antigua y sencilla: **Esperar a que la app esté inactiva** con **Esperar como máximo (horas)**. La copia empieza en cuanto la app está inactiva, algo que define la tarjeta **Inactividad antes de la copia** de **Ajustes, Calendario**. Los miembros de un stack de Compose que toquen en la misma ejecución esperan juntos, y la copia en espera no retiene ningún bloqueo, así que los demás contenedores siguen adelante.

![La tarjeta Cuando la app está ocupada de un contenedor Plex con la comprobación de actividad configurada.](assets/screenshots/backups/app-busy-dark.png#only-dark)
![La tarjeta Cuando la app está ocupada de un contenedor Plex con la comprobación de actividad configurada.](assets/screenshots/backups/app-busy-light.png#only-light)

### Ejemplo: Plex espera mientras alguien mira algo {#example-plex}

La copia total se ejecuta cada noche a las 03:00. Nadie debería ver cómo se corta una película porque Plex se detuvo para su copia, pero la copia debe hacerse igualmente antes de la mañana.

1. Abre **Copias de seguridad**, luego el elemento `plex` y su pestaña **Ajustes**.
2. En **Cuando la app está ocupada**, pon **Comprobación de actividad** en Plex. No hay nada que escribir.
3. Pon **Ignorar sesiones en pausa tras (min)** en `30`, para que una película en pausa a medianoche deje de contar a las doce y media.
4. Deja **Copiar cuando el servidor no responde** activado.
5. Pon **Reintentar cada (min)** en `15`.
6. Activa **Intentar solo dentro de una franja horaria** y pon **Intentar entre** en `22:00` y `06:00`.
7. Pon **Esperar como máximo (horas)** en `3` y **Al final de la espera** en **Copiar de todos modos**.

Qué pasa una noche en que alguien sigue mirando algo a las 03:00: BombVault pregunta a Plex, encuentra una sesión que está reproduciendo y registra la copia como **Aplazado** con el siguiente intento a las 03:15. Los demás contenedores de la ejecución se copian como de costumbre. Cuando la película termina a las 04:10, el intento de las 04:15 encuentra Plex inactivo y lo copia, y la espera termina con una notificación. Si la película siguiera en marcha a las 06:00, la espera llegaría a su final y Plex se copiaría de todos modos, así que sus copias nunca se retrasan más de tres horas.

Para tu propia comprobación, haz que el **Comando pre-copia** termine con el código 75 mientras la app esté ocupada, por ejemplo `[ "$(cat /tmp/players)" = 0 ] || exit 75`. Encontrarás más síntomas y soluciones en [Solución de problemas](troubleshooting.md#deferred-backups).

## Exportaciones sin formato {#exports}

restic sigue siendo el motor, pero una exportación te da una copia que no necesita ni BombVault ni restic.

- **Exportar contenedor** y **Exportar VM** bajo **Más** en la página de un elemento escriben una copia tal como está ahora, junto al repositorio: las carpetas a copiar del contenedor más su plantilla de Unraid, o las imágenes de disco de la VM más su definición, restaurable con `virsh define`.
- **Exportar** en una instantánea escribe una copia guardada en la carpeta de exportación del servidor, como tar, zip o un repositorio restic propio, y puede tomar una instantánea o todas.

Las exportaciones son archivos normales por defecto. **Cifrar exportaciones con age** en **Ajustes, Sistema** las sella, el ZIP del flash incluido, para los destinatarios que indiques, cada uno una clave pública age o una clave pública SSH. Las descifras más tarde fuera del servidor con la clave privada correspondiente. Con el cifrado activado y ningún destinatario válido, una exportación falla con un error claro en lugar de escribir nunca archivos sin cifrar.

## Qué te cuenta una ejecución {#run-notes}

- **Tiempos.** Cada ejecución muestra inicio, fin y duración, y cada elemento tiene sus propias **Ejecuciones recientes**.
- **Por qué una copia fue lenta.** Mientras se ejecuta una copia, BombVault vigila lo ocupados que están la CPU, los discos y la red. Cuando una copia tarda mucho más de lo habitual y algo estaba claramente al límite, la ejecución lo dice, por ejemplo "El disco de destino disk1 estaba ocupado al 98 %". Si no, no dice nada.
- **Progreso de comprobaciones y podas.** El registro de actividad y la tarjeta **Comprobar las copias** muestran hasta dónde ha contado restic, como *12 de 47 paquetes*, con el tiempo que queda en ese paso cuando hay datos suficientes para estimarlo.
- **Archivos que no pudo leer.** Una copia que terminó pero dejó algo fuera pone su línea en naranja y nombra lo que quedó fuera.
