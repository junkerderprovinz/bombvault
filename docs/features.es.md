# Funciones

Un breve catálogo de lo que hace BombVault. Cada línea enlaza con la página que lo explica.

## Alcance de las copias {#backup-scope}

- **Contenedores**: la carpeta appdata y la definición del contenedor, con una elección de carpetas por contenedor. [Copias de seguridad y elementos](backups.md#what-is-saved)
- **VMs**: imágenes de disco, XML y NVRAM UEFI por SSH, con un apagado ordenado, una instantánea en vivo o solo los bloques modificados. Las VMs con instantáneas de Unraid y discos en zvols quedan cubiertas por completo. [Copias de VMs](vm-backup-ssh-setup.md)
- **Flash**: todo el USB flash de Unraid, con una exportación zip programada. [Copias de seguridad y elementos](backups.md#settings-tab)
- **Autocopia**: los ajustes y las credenciales propios de BombVault. [Asistente de emergencia](offsite-recovery.md#emergency)
- **Carpetas**: cualquier carpeta del servidor como conjunto de carpetas. [Añadir una carpeta](backups.md#add)
- **Conjuntos de datos ZFS**: un árbol de conjuntos de datos leído de una instantánea, con restauración de archivos sueltos y una réplica opcional en un segundo servidor ZFS. [Conjuntos de datos ZFS](zfs-datasets.md)
- **Volcados de bases de datos**: PostgreSQL, MySQL y MariaDB, volcados desde el servidor en ejecución a un punto de restauración propio. [Volcados de bases de datos](backups.md#database-dumps)

## Copias de seguridad y páginas de elementos {#backups}

- **Una sola lista** con mosaicos, **Filtros**, **Ordenar** y **Buscar**, y las últimas catorce ejecuciones de cada fila como una tira. [La lista](backups.md#list)
- **Selecciona como en un gestor de archivos**, y copia o pausa una selección en el servidor. [Elegir varios elementos](backups.md#select)
- **Una página por elemento** con **Copias de seguridad**, **Ajustes** y **Verificación**. [La página de un elemento](backups.md#entry-page)
- **Etiquetas y comparación** de instantáneas, y **Exportar** como tar, zip o un repositorio restic. [Pestaña Copias de seguridad](backups.md#backups-tab)
- **Patrones de exclusión** con vista previa en vivo, y exclusiones recomendadas para apps conocidas. [Patrones de exclusión](backups.md#excludes)
- **Comandos antes y después**, y **Detener otros contenedores** con un reinicio condicionado a la salud y en orden de dependencias. [Comandos y otros contenedores](backups.md#commands)
- **Cuando la app está ocupada**: una copia programada espera a Plex, Jellyfin, Emby, a tu propia comprobación o a que el tráfico se calme, y vuelve a intentarlo por sí sola. [Cuando la app está ocupada](backups.md#app-busy)
- **Instantánea en vez de parada**: un contenedor en ZFS está parado solo unos segundos. [Calendarios](schedules.md#snapshot-instead-of-stop)
- **Exportaciones sin formato**, opcionalmente selladas con age. [Exportaciones sin formato](backups.md#exports)

## Restauración {#restore}

- **Una página y una ventana** para cada tipo, con la copia recomendada preseleccionada. [Restauración](restore.md)
- **Junto al original** por defecto, y **Reemplazar original** copia antes el estado actual. [La ventana de restauración](restore.md#window)
- **Archivos sueltos** a una carpeta o de vuelta a su sitio. [Recuperar un solo archivo](restore.md#example-single-file)
- **Varios elementos a la vez**, cada uno desde su copia recomendada o desde un mismo momento. [Restaurar varios elementos](restore.md#several)
- **Stacks de Compose enteros**, iniciados en orden de dependencias. [Restaurar un stack](restore.md#stacks)
- **Los contenedores y las VMs se recrean** sobre la imagen con la que se ejecutaban, con GPU, límites y enlaces. [Qué se recupera](restore.md#kinds)
- **Comprobaciones antes de la restauración**, y una lista de lo que cambia. [La ventana de restauración](restore.md#window)
- **El flash como ZIP**, y un plugin cada vez de vuelta al flash en funcionamiento. [Qué se recupera](restore.md#kinds)
- **ZFS** dentro del conjunto de datos con una instantánea de seguridad, en una carpeta o en un conjunto de datos nuevo con sus propiedades. [Conjuntos de datos ZFS](zfs-datasets.md#restore)
- **Asistente de emergencia** para un servidor perdido, con el kit de recuperación, a través del grupo o con la clave escrita. [Asistente de emergencia](offsite-recovery.md#emergency)
- **Desde una carpeta de copias antigua**, **Importar desde el plugin Appdata.Backup** y **Tareas de Duplicati o Duplicacy**, que convierte sus tareas de copia en conjuntos de carpetas. [Otras fuentes](restore.md#other-sources)

## Almacenamiento y calendarios {#storage-scheduling}

- **Destinos** de todo tipo: carpetas locales, SMB, WebDAV, SFTP, rest-server, S3, unidades en la nube y cualquier remoto de rclone. [Destinos](offsite-recovery.md#destinations)
- **Credenciales**: una sola lista con una predeterminada. [Credenciales](offsite-recovery.md#sign-ins)
- **Carpetas propias** en este servidor o en un destino. [La carpeta propia de una sección](offsite-recovery.md#own-folder)
- **3-2-1 como un flujo** donde cada línea es un interruptor, y **destino por elemento**. [Destino por elemento](offsite-recovery.md#placement)
- **Copias externas** a varios destinos por sección, con límites de ancho de banda, compresión, clases de almacenamiento frío y una ralentización mientras Plex hace streaming. [Copias externas](offsite-recovery.md#off-site-copies)
- **Repositorios directos**, listados en **Se guarda directamente aquí**. [Repositorios directos](offsite-recovery.md#direct-repositories)
- **Retención como línea de tiempo**: **Corto**, **Equilibrado**, **Largo** o **Valores propios** por destino, con reglas por sección. [Cuánto tiempo conservar](offsite-recovery.md#keep)
- **Mover el historial de una sección**, y **Eliminar destino** con o sin sus copias. [Mover y eliminar](offsite-recovery.md#move-remove)
- **Calendarios que se leen como frases**, la banda **Semana**, una copia total, calendarios por sección y un calendario propio por elemento. [Calendarios](schedules.md)
- **Recuperar copias perdidas**, un orden manual y actualizaciones tras una copia correcta. [Calendarios](schedules.md)

## Visión, verificación y supervisión {#insight-verification-monitoring}

- **El resumen**: cabecera de estado, **Necesita atención**, **Estado de protección**, previsión de almacenamiento, historial y tarjetas opcionales. [Resumen](overview.md)
- **Detección de anomalías** por elemento, que además conserva las copias antiguas cuando un origen se desploma. [Anomalías](anomalies.md)
- **Comprobación de restauración** tras la primera copia, y **Prueba de arranque** en una copia aislada. [Pestaña Verificación](backups.md#verify-tab)
- **Comprobaciones programadas y restauraciones de prueba** desde la copia externa. [Simulacros de DR](offsite-recovery.md#dr-drills)
- **Tamaño por carpeta**, y por qué una copia fue lenta. [Qué te cuenta una ejecución](backups.md#run-notes)
- **Notificaciones**: una lista de canales para Unraid, ntfy, Gotify, Discord, Slack, Matrix, correo, Apprise, cualquier webhook y Healthchecks, cada uno con su propia regla, con un resumen semanal y un informe de retrasos. [Notificaciones](notifications.md)
- **Prometheus**, un dispositivo de Home Assistant, una API HTTP y mDNS. [API e integraciones](api.md)
- **Informe de error**: un ZIP con los datos sensibles tapados para una incidencia de GitHub. [Solución de problemas](troubleshooting.md#still-stuck)

## Protección contra ransomware {#ransomware-protection}

- **Protección contra borrado (append-only)**, aplicada por el otro lado y demostrada con una prueba de manipulación. [Protección contra borrado](offsite-recovery.md#delete-protection-append-only)
- **Aceptar sin protección contra borrado** para un lugar que no puede rechazar un borrado. [Protección contra borrado](offsite-recovery.md#delete-protection-append-only)
- **Borrar copias para siempre requiere que inicies sesión tú mismo** cuando hay una contraseña de acceso, y un teléfono emparejado no puede hacerlo. [Modelo de seguridad](configuration.md#security-model)
- **Kit de recuperación**, opcionalmente sellado con age, y un **guardián de claves** en tu grupo. [Kit de recuperación](offsite-recovery.md#recovery-kit), [Guardián de claves](offsite-recovery.md#key-keeper)

## Varios servidores {#several-servers}

- **Emparejamiento con doce palabras**, cifrado de extremo a extremo, directamente o a través de un repetidor. [Emparejar instancias](offsite-recovery.md#pairing)
- **La página Instancias** con el grupo como una red y los roles **Receptor**, **Recolector**, **Servidor ZFS** y **Guardián de claves**. [Instancias y roles](offsite-recovery.md#roles)
- **Un servidor receptor** configurado por ti, en modo append-only. [Servidor receptor](offsite-recovery.md#receiving-server)
- **Un repositorio compartido** en el que copian varios servidores, para que lo que tienen en común se guarde una sola vez. Solo poda el servidor que lo aloja. [Repositorio compartido](offsite-recovery.md#shared-repository)

## Asistentes de IA (MCP) {#mcp}

Un servidor MCP integrado permite a un asistente leer el estado de las copias, la cobertura, el historial de ejecuciones y los puntos de restauración, y con una clave que lo permita iniciar una copia. Las restauraciones, los borrados, la poda y los ajustes se quedan en la interfaz web. [Servidor MCP](mcp.md)

## Apps y acompañantes {#apps}

- **App de Android**: todos los servidores de tu grupo en tu teléfono. [App de Android](android.md)
- **La fortaleza**: BombVault se une a las demás apps de la familia. ShipLog y TowerTorch piden una copia de seguridad antes de cambiar un contenedor, FireSquire comprueba cuándo se hizo la última copia del flash, y las apps que saben hacer su propia copia se la entregan a BombVault. [La fortaleza](offsite-recovery.md#fortress)
- **TollRoll**: un solo mosaico en el panel de Unraid para BombVault y las demás apps de tu fortaleza. [Apps](settings.md#apps)
- **Registro de actividad incrustable** para paneles como Homepage o Heimdall. [Widget de panel](api.md#dashboard-widget)

## Otros {#other}

- **42 idiomas**, tres temas, y etiquetas, colores, esquinas y animación a tu gusto. [Aspecto](settings.md#look)
- **Ajustes que se guardan solos**, con una búsqueda en los catorce mosaicos. [Ajustes](settings.md)
- **Novedades** tras una actualización, con puntos verdes en lo que cambió. [Info](settings.md#info)
- **HTTPS de serie**, y una comprobación de salud para herramientas de autorreparación. [Configuración](configuration.md#network)
