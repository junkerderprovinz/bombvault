# Restauración

Toda restauración empieza en la página **Restauración**: elige una sección, busca lo que quieres recuperar y escoge una copia. Una sola ventana restaura cada tipo de elemento y pregunta por pasos. Por defecto no cambia nada en el original, porque la copia vuelve junto a él.

![La página Restauración con las secciones como mosaicos, la búsqueda y la tarjeta Stacks.](assets/screenshots/restore/page-dark.png#only-dark)
![La página Restauración con las secciones como mosaicos, la búsqueda y la tarjeta Stacks.](assets/screenshots/restore/page-light.png#only-light)

## La página Restauración {#page}

- **Secciones** lista las secciones como mosaicos, cada una con cuántos elementos tienen instantáneas.
- **¿Qué quieres recuperar?** busca entre los elementos de la sección elegida. Cada fila muestra su instantánea más reciente, y **Abrir elemento** lleva a la página del elemento.
- **Stacks** lista los proyectos de Compose con al menos dos contenedores copiados, como `immich` con su base de datos y Redis. La búsqueda encuentra un stack por su nombre o por cualquiera de sus contenedores.
- **En curso** muestra una restauración mientras se ejecuta, también después de cerrar su ventana.
- **Emergencia** inicia el Asistente de emergencia para un servidor que ha desaparecido o se volvió a configurar, consulta [Asistente de emergencia](offsite-recovery.md#emergency).
- **Otras fuentes** contiene **De una carpeta de copias antigua**, para las copias de otro BombVault, e **Importar desde el plugin Appdata.Backup**.

![La página Restauración en un teléfono.](assets/screenshots/restore/page-phone-dark.png#only-dark)
![La página Restauración en un teléfono.](assets/screenshots/restore/page-phone-light.png#only-light)

## La ventana de restauración {#window}

Un clic en un elemento abre **Restaurar** con su nombre. Pregunta por pasos, y cada paso se puede cambiar antes de empezar:

1. **Copia**: la instantánea recomendada viene preseleccionada. Es la más reciente o, tras una anomalía abierta, la última anterior a ella. La copia hecha antes de una restauración anterior guarda el estado que esa restauración reemplazó, así que se descarta salvo que no haya otra. Las más antiguas están en la lista, y **Comparar** muestra qué difiere entre dos de ellas.
2. **Origen**: de dónde salen los archivos, cuando una instantánea está en varios sitios. **Local** es lo más rápido.
3. **Qué**: **Todo**, o **Archivos individuales**, donde marcas los archivos y carpetas que quieres.
4. **Dónde**: el destino de la restauración. Las opciones dependen del tipo de elemento:

    | Destino | Qué hace |
    |---|---|
    | **Junto al original** | El valor predeterminado. Un contenedor vuelve como un segundo contenedor llamado `<name>-restored` con puertos propios, los discos y la NVRAM de una VM acaban en una carpeta llamada `<folder>-restored`, y un conjunto de carpetas vuelve como `<path>-restored`. No cambia nada en el original. |
    | **Reemplazar original** | Sobrescribe lo que hay ahora. Con **Antes, copia del estado actual**, que está activado por defecto, BombVault copia primero el estado de hoy para que puedas volver atrás. Si esa copia falla, la restauración no empieza. Esto también funciona con un contenedor o una VM que ya no está en el servidor: las carpetas que la restauración sobrescribe se copian por separado. |
    | **En una carpeta** | Solo los archivos, para examinarlos. |
    | **Descargar como ZIP** | La unidad flash, para un lápiz nuevo. BombVault nunca escribe en la unidad flash en funcionamiento. |
    | **En un conjunto de datos nuevo** y **Restaurar dentro del conjunto de datos** | Conjuntos de datos ZFS, consulta [Conjuntos de datos ZFS](zfs-datasets.md#restore). |

5. **Qué pasa** dice con palabras lo que hará la restauración, y una escena muestra el destino que elegiste.

Antes de que **Restaurar** se desbloquee, la ventana comprueba que el repositorio responde, que la clave lo abre, que la instantánea está ahí y que el destino tiene espacio. Mientras una comprobación falla el botón sigue bloqueado, y la (i) que lleva dice cuál.

La ventana también muestra qué cambia la restauración respecto a lo que hay ahora: archivos nuevos, reemplazados y sin cambios, con la lista si la pides, y los archivos del destino que no están en la copia y se quedan donde están. restic lo calcula como una simulación a partir del tamaño y de la hora de modificación, sin leer los archivos, y un árbol muy grande se detiene a los 30 segundos y lo dice. Para contenedores y VMs compara los ajustes que recrea la restauración con los que están en ejecución: imagen y etiqueta, puertos, nombres de variables y volúmenes, o memoria, vCPUs, discos y red.

Algunas cosas pueden detener una restauración antes de que cambie nada. La IP estática de un contenedor y sus puertos publicados tienen que estar libres, y su red tiene que existir. En Unraid, toda restauración in situ rechaza una carpeta que no esté en un pool o recurso compartido montado, porque los archivos acabarían en la memoria y desaparecerían tras un reinicio. Una restauración in situ nombra cada otro contenedor cuyo bind mount llega a una carpeta en la que escribe, por ejemplo "this path is also used by nextcloud-db", pero eso solo avisa.

Algunas opciones aparecen donde se aplican: **Dejar detenido después**, para mirar un contenedor o una VM antes de que vuelva a ejecutarse, y **Restaurar sin GPU ni entorno de ejecución** en un host que carece de la GPU o del entorno de ejecución con que se hizo la copia.

La restauración se ejecuta en segundo plano. Puedes cerrar la ventana, y el progreso también se muestra en **En curso**. Una restauración larga muestra un porcentaje y se puede cancelar, y una restauración cancelada se registra como cancelada, no como fallida. Una restauración cancelada mientras el contenedor se detiene, o una que no puede eliminar el contenedor, todavía no ha restaurado nada, así que el contenedor vuelve a arrancar. Un contenedor que toma sus volúmenes de otro con `--volumes-from` los conserva, y la restauración se niega a empezar mientras ese otro contenedor no exista.

![La ventana de restauración en el paso Qué, con archivos sueltos marcados.](assets/screenshots/restore/window-what-dark.png#only-dark)
![La ventana de restauración en el paso Qué, con archivos sueltos marcados.](assets/screenshots/restore/window-what-light.png#only-light)

### Adónde va una restauración en una carpeta {#folders}

**Carpetas de restauración** en **Ajustes, Restauración** fija la **Carpeta de restauración predeterminada**, con una **Subcarpeta por sección** si quieres, y una **Excepción** para una sección. Los archivos elegidos acaban directamente en la carpeta de destino: un archivo vuelve como ese archivo, una carpeta como la carpeta con su contenido, y las selecciones de carpetas distintas conservan solo su ruta por debajo de la carpeta que comparten. BombVault rechaza un destino que no está en almacenamiento montado, porque en Unraid una carpeta así vive en memoria y desaparece tras un reinicio, y nunca restaura en su propia carpeta de datos.

## Ejemplo: recuperar un solo archivo {#example-single-file}

Alguien editó anoche el `config.php` de Nextcloud y Nextcloud ya no arranca. La copia nocturna de las 03:00 aún tiene el archivo bueno. Quieres ese archivo, no todo el contenedor.

1. Abre **Restauración**, elige el mosaico **Contenedores** y escribe `nextcloud` en **¿Qué quieres recuperar?**.
2. Haz clic en `nextcloud`. La ventana se abre con la copia de las 03:00 preseleccionada como la recomendada.
3. Deja **Origen** en **Local**.
4. En **Qué**, elige **Archivos individuales**. Abre las carpetas hasta `config` y marca `config.php`.
5. En **Dónde**, elige **En una carpeta** y escoge una carpeta, por ejemplo `user/restore`, que es `/mnt/user/restore` en el host.
6. Lee **Qué pasa**: el archivo acaba en la carpeta de destino y no cambia nada en el contenedor. Pulsa **Restaurar**.

La ventana termina con **Listo: los archivos están en /mnt/user/restore**. Encuentras el archivo como `/mnt/user/restore/config.php`, sin las carpetas de encima. Compáralo con el roto, cópialo de vuelta en la carpeta appdata y reinicia Nextcloud.

Para devolver el archivo directamente a su sitio, elige **Reemplazar original** en el paso 5. Deja activado **Antes, copia del estado actual**, para que BombVault copie el estado roto antes de escribir, escribe `nextcloud` para confirmar y pulsa **Restaurar**. Nextcloud se detiene mientras el archivo elegido se escribe de vuelta en su sitio y vuelve a arrancar después, y solo se escriben los archivos que marcaste. Una restauración que falla a medias deja Nextcloud detenido, porque el archivo que lee puede estar a medio escribir, y la ejecución lo indica.

![La ventana de restauración en Qué pasa, con Reemplazar original elegido.](assets/screenshots/restore/window-replace-dark.png#only-dark)
![La ventana de restauración en Qué pasa, con Reemplazar original elegido.](assets/screenshots/restore/window-replace-light.png#only-light)

## Qué se recupera, según el tipo {#kinds}

Los **contenedores** se recrean a partir de su definición guardada mediante la API de Docker, con su entrypoint, nombre de host, ajustes de parada, volúmenes y montajes, límites de recursos, controlador de registros, ajustes de DNS y enlaces heredados. Un contenedor vuelve con la imagen con que se ejecutaba cuando se hizo la copia, no con una más nueva descargada después, y una imagen que falta se descarga primero, con las credenciales de **Registros de contenedores** en **Ajustes, Restauración**. Su GPU o entorno de ejecución (`--gpus`, `--runtime=nvidia`) también vuelve. En un host sin ese controlador o entorno de ejecución la restauración lo dice y ofrece **Restaurar sin GPU ni entorno de ejecución**. Un enlace a un contenedor que falta, o que está detenido mientras arranca el restaurado, se deja fuera, y el historial de ejecuciones lo dice. Una restauración que falla después de eliminar el contenedor lo devuelve, detenido.

Una copia puede contener carpetas que el contenedor ya no tiene seleccionadas, por ejemplo una más antigua hecha antes de que desapareciera una carpeta. Una carpeta que ya no existe o que está vacía vuelve. Una que ahora contiene archivos se queda como está, porque puede que la desmarcaras a propósito, y la comprobación previa a la restauración y el historial de ejecuciones la nombran. Recupérala con **Archivos individuales** o **En una carpeta**.

A las **VMs** se les vuelve a definir el XML por SSH y se les reconectan los discos y la NVRAM UEFI, incluso después de eliminar la VM. Consulta [Copias de VMs](vm-backup-ssh-setup.md) para las VMs con instantáneas de Unraid y discos en zvols.

Los **conjuntos de carpetas** vuelven con su propietario, modos y marcas de tiempo tal como los guarda restic. Donde el destino solo rechaza el propietario, como hace `/mnt/user`, la ejecución termina con una nota que lo indica. BombVault compara entonces los archivos con la copia, y un archivo que no llegó hace fallar la restauración.

La **unidad flash** nunca toca el `/boot` real. La restauración es una descarga llamada `flash-<id>.zip`, lista para soltar en el USB Creator de Unraid. El ZIP contiene `APP_KEY`, así que descargarlo requiere una contraseña de acceso e iniciar sesión en la instancia. La ventana también lista los plugins de cada copia del flash con su versión y tamaño, y puede devolver uno solo al flash en funcionamiento: su archivo `.plg`, su carpeta bajo `config/plugins` y los archivos de paquete que contiene la copia. Unraid lo instala en el siguiente arranque, o enseguida en **Plugins, Install Plugin**.

Los **conjuntos de datos ZFS** tienen destinos propios y conservan sus propiedades ZFS, consulta [Restaurar](zfs-datasets.md#restore).

La **autocopia** vuelve solo mediante el Asistente de emergencia, que reinicia BombVault, de modo que la base de datos en uso nunca se sobrescribe con un descriptor abierto.

Los **volcados de bases de datos** nunca se cargan de vuelta por sí solos. Una fila de **Volcados de la base de datos** se puede descargar, guardar en una carpeta o importar en una base de datos recién iniciada. Consulta [Volcados de bases de datos](backups.md#database-dumps).

## Restaurar varios elementos a la vez {#several}

En la página Restauración, elige elementos como eliges filas en la lista de Copias de seguridad: Ctrl+clic, Mayús+clic, Ctrl+A, una pulsación larga o **Seleccionar**. **Restaurar** abre entonces una sola ventana para todos.

- **Qué copia**: **Recomendada** toma para cada elemento la copia que recomendaría su propia ventana. **Un momento para todos** toma para cada elemento su copia más reciente de un **Momento** que elijas o anterior, por ejemplo la tarde anterior a que algo saliera mal.
- **Dónde**: **Junto al original** o **Reemplazar original**, con **Antes, copia del estado actual**.

Cada elemento se comprueba como una restauración individual, y luego se ejecutan uno tras otro. Cada fila muestra cómo fue la suya: **En espera**, **En ejecución**, **Restaurado**, **Error** o **Dejado fuera**. **Parar después de este** deja terminar la restauración en curso y no inicia más. La unidad flash llega como un ZIP para descargar, y los conjuntos de datos ZFS y la autocopia se restauran de uno en uno.

![Restaurar varios elementos: una fila por elemento con su progreso.](assets/screenshots/restore/several-dark.png#only-dark)
![Restaurar varios elementos: una fila por elemento con su progreso.](assets/screenshots/restore/several-light.png#only-light)

## Restaurar un stack {#stacks}

Un clic en un stack de la tarjeta **Stacks** abre **Restaurar stack**. Restaura todos los miembros a la vez desde su última copia, y con **Iniciar en orden de dependencias tras la restauración** los inicia en su orden `depends_on`: primero las bases de datos y cachés, luego la app. Cuando falta la carpeta del proyecto de un miembro en el lugar que elegiste, la ventana ofrece en su lugar el estado de Local.

BombVault comprueba primero los miembros y la carpeta del proyecto, y una restauración que no podría salir bien no cambia nada. Si no, detiene los miembros en marcha, las apps antes que sus bases de datos, luego escribe la carpeta del proyecto y después restaura los miembros. Si la carpeta del proyecto no vuelve, los miembros se restauran igualmente pero se quedan detenidos, porque la carpeta puede estar a medio escribir y es donde suelen estar los bind mounts de un miembro. La ejecución de cada miembro lo indica.

Una base de datos que solo tiene volcados de base de datos y ninguna copia de archivos, y cuyo contenedor y volumen con nombre ya no existen, vuelve sin sus datos. La comprobación lo avisa antes de la restauración. Se queda detenida, igual que los contenedores que dependen de ella, para que ninguna app ejecute su configuración inicial sobre una base de datos vacía. Arráncala, importa un volcado y arranca los demás. Consulta [Volcados de bases de datos](backups.md#database-dumps).

## Otras fuentes {#other-sources}

- **De una carpeta de copias antigua** abre el repositorio de otro BombVault que ya no funciona, en un disco o en un destino externo, con su kit de recuperación o su `APP_KEY` escrita. La sesión es de solo lectura y vive solo en memoria. Consulta [De una carpeta de copias antigua](offsite-recovery.md#restore-from-another-bombvault-repo).
- **Importar desde el plugin Appdata.Backup** lee la carpeta de copias del plugin. Cada archivo de contenedor se convierte en un punto de restauración de su contenedor, con la fecha en que lo hizo el plugin. Los archivos importados antes se omiten, y los archivos en sí solo se leen. El contenedor necesita antes una copia en BombVault, para que la restauración tenga su definición. La retención deja en paz los puntos de restauración importados, así que borra uno tú cuando ya no lo necesites.

## Después de una restauración {#after}

Una copia restaurada junto al original aparece en **Copias de seguridad** como excluida, para que no cuente como tarea abierta. Añádela al calendario si quieres conservarla. Un contenedor o VM que estaba en ejecución cuando se copió vuelve en ejecución, y uno que estaba detenido sigue detenido. En Unraid un contenedor restaurado reaparece en la pestaña Docker como una app normal y editable, y una VM en el VM Manager con sus discos y su NVRAM reconectados.
