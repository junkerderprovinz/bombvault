# Copias externas y recuperación

!!! note "Las copias externas esperan tras una reconstrucción"
    Cuando el Asistente de emergencia o **Descubrir copias** reconstruye elementos sin los ajustes antiguos, las copias externas de esas secciones esperan hasta que adoptes sus copias como propias de este servidor. Consulta [Después de una reconstrucción](#after-a-rebuild).

Las copias locales te protegen de un contenedor perdido o de una mala actualización. Las copias externas y un kit de recuperación probado te protegen de la pérdida de todo el equipo, del ransomware o de un incendio. Esta página trata de los destinos y las copias externas, de cómo hacer que una copia no se pueda manipular, de cómo demostrar que puedes restaurar, de emparejar varios servidores y de recuperarlo todo cuando el propio BombVault ya no está.

## Copias externas {#off-site-copies}

Conserva la copia local rápida y añade uno o más destinos fuera de las instalaciones. BombVault copia allí las instantáneas nuevas con `restic copy` en la medida de lo posible, así que un contratiempo externo nunca hace fallar la copia local. En este esquema cada sección escribe primero en su propia carpeta y el repositorio externo es una copia. La carpeta propia no tiene que estar en este servidor: consulta [La carpeta propia de una sección](#own-folder) para mantenerla en un destino como S3 o un rest-server.

- **Varios destinos por sección.** Cada sección (contenedores, VMs, flash, autocopia, carpetas y conjuntos de datos ZFS) puede copiar a varios destinos externos a la vez, de modo que puedas mantener, por ejemplo, un rest-server en el equipo de un amigo y un bucket S3 en paralelo. La tarjeta **Destinos por sección** de la página Destinos activa o desactiva cada línea de una sección a un destino.
- **Cuándo se ejecutan las copias.** La tarjeta **Copiar** de un destino fija **Cuándo copiar**: **Después de cada copia**, o un calendario propio que reúne las instantáneas y copia a horas fijas. En **Ajustes, Calendario, Copias externas por sección**, **Excepción** da a una sección su propio calendario de copia.
- **La tarjeta Copias externas.** En **Ajustes, Destinos** cada sección con una copia externa tiene una fila en **Copias externas**, con **Probar conexión**, **Replicar ahora** para iniciar una copia a demanda, y **Configurar** o **Editar**. El resto de los ajustes de copia de la sección se abre desde ahí en su ventana.
- **La retención** se fija por destino en **Cuánto tiempo conservar**, de modo que puedes conservar las copias externas más tiempo como archivo. La copia del campo externo de una sección la sigue como cualquier otra copia, y una sección puede conservar sus propios valores en **Reglas por sección**. Todo a cero nunca recorta las instantáneas externas. Consulta [Cuánto tiempo conservar](#keep).
- **Los límites de ancho de banda** (**Ancho de banda externo** en **Ajustes, Destinos**, o la tarjeta **Copiar** de un destino) limitan la velocidad de subida y bajada de restic para que las copias no saturen tu WAN. Se introducen en KiB/s, la unidad que acepta restic, y cada campo muestra la misma velocidad en MB/s y Mbit/s mientras escribes: 2048 se lee como unos 2,0 MB/s (16,8 Mbit/s).
- **La compresión** es **Desactivado**, **Automático** (el valor por defecto de restic) o **Máximo**, y se fija en cada destino y, para una sección, en sus reglas allí. Las copias, las réplicas y la poda escriben con ella, y el kit de recuperación la nombra, para que el restic normal pueda seguir escribiendo igual.
- **Primero el streaming.** Mientras un servidor multimedia como Plex, Jellyfin o Emby hace streaming, las copias suben con un límite más bajo y vuelven al normal unos minutos después de que termina el streaming. La tarjeta **Streaming** de la página Destinos tiene **Ralentizar las copias externas durante el streaming**, **Servidores multimedia** (contenedores con Plex, Jellyfin o Emby en el nombre de la imagen hasta que elijas), **Streaming a partir de (Mbit/s)**, **Límite de subida durante el streaming (KiB/s)** y **Volver a lo normal tras (min)**. BombVault lee el tráfico de Docker, así que un servidor multimedia en la red del host no se puede medir. REST, S3, B2, Azure, Google Cloud, Swift y rclone sobre HTTP se ralentizan en mitad de una copia. SFTP y las carpetas locales o montadas toman el límite más bajo en su siguiente paso de copia.
- **Almacenamiento frío (S3).** Un destino S3 puede usar una **Clase de almacenamiento** más barata que restic aún pueda leer de inmediato, como Standard-IA, One Zone-IA, Intelligent-Tiering o Glacier Instant Retrieval. Glacier Flexible Retrieval y Deep Archive se dejan fuera, porque restic no puede restaurar desde ellos. Un remoto de rclone fija su clase en la configuración de rclone.
- **Presupuesto de crecimiento.** **Presupuesto de crecimiento (GB, 0 = desactivado)** en la ventana de copia de una sección da la alarma cuando el repositorio crece por encima de ese tamaño. Nunca borra nada, así que sirve para una copia append-only de la que nada poda desde aquí.
- Mientras se ejecuta una copia, la cabecera de estado del resumen dice **Copia externa en curso**. Es un indicador de actividad, no una barra de porcentaje, porque `restic copy` no expone ningún progreso legible por máquina.

!!! note "Restaurar desde cualquier lugar"
    Cada elemento lista sus copias como una sola línea de tiempo por todos los lugares donde hay una copia, en su pestaña **Copias de seguridad**. Una copia copiada a B2 aparece una vez, marcada con cada lugar que la contiene. Una restauración toma el primer lugar al que puede llegar, empezando por el repositorio en el que se escribe el elemento, y puedes elegir otro lugar por instantánea. Los destinos externos solo se leen cuando haces clic en **Comprobar**, porque listar el almacenamiento en la nube lleva un rato. Borrar en un lugar comprueba antes los demás y dice si era la última copia. Consulta [Restauración](restore.md).

## Destinos {#destinations}

Un destino es un lugar adonde van las copias: una carpeta local, un recurso compartido, un servidor propio o un servicio en la nube. Se configura una vez y luego sirve a todas las secciones. Abre **Destinos** desde su mosaico de ajustes, o activa allí **Mostrar también en la barra lateral** para anclarlo.

![La página Destinos: la lista de destinos y el flujo de qué sección va adónde.](assets/screenshots/storage/locations-dark.png#only-dark)
![La página Destinos: la lista de destinos y el flujo de qué sección va adónde.](assets/screenshots/storage/locations-light.png#only-light)

**Añadir destino** abre una ventana en cuatro pasos:

1. **Proveedor.** Cada servicio aparece con su logotipo, con una búsqueda arriba a la derecha de la ventana, en grupos: **Local** para un disco o recurso compartido conectado a este servidor, **NAS o servidor** (rest-server, Hetzner Storage Box, SFTP, SMB, WebDAV y otros), **Servicio S3** (Backblaze B2, Wasabi, Cloudflare R2, Hetzner Object Storage, Amazon S3 y otros), tu propio servidor S3 (Garage, SeaweedFS, RustFS, Silo, Ceph, JuiceFS, Versity S3 Gateway), **Almacenamiento en la nube** (OneDrive, Google Drive, Dropbox, pCloud, Nextcloud y el resto que admite rclone) y **De tu grupo**, los servidores receptores de tu grupo. Cada uno dice lo bien que sirve para copias: las unidades en la nube se ralentizan con muchas peticiones, así que allí la primera copia y la poda tardan más. Para un servicio que ningún mosaico cubre, el mosaico **Rclone** en **Configuración propia de rclone** acepta remotos pegados de un rclone.conf.
2. **Iniciar sesión.** Los campos dependen del servicio: una clave de acceso para S3, un usuario y contraseña para WebDAV y SMB, una contraseña de aplicación donde el inicio de sesión en dos pasos bloquea la normal, la clave pública SSH de BombVault para SFTP y la Storage Box, o un token para servicios que inician sesión mediante un navegador. Para esos, la ventana muestra un comando `rclone authorize` para ejecutar en un equipo con navegador, y el token que imprime va en el campo. Donde ya tienes unas [credenciales guardadas](#sign-ins) para el servicio, **Credenciales guardadas** las elige en lugar de **Claves nuevas**. **Probar conexión** comprueba las credenciales antes de guardar nada. La ventana lista entonces las carpetas del destino, con **Nueva carpeta** para crear una y el espacio libre donde el servicio lo informa. Una carpeta vacía es lo más seguro. BombVault pone dentro una carpeta por sección.
3. **Protección.** La ventana dice con claridad lo que puede hacer el servicio. Un rest-server en modo append-only rechaza el borrado, y la prueba de manipulación lo comprueba. Un bucket S3 puede conservar versiones antiguas con versionado y bloqueo de objetos, algo que BombVault todavía no puede comprobar. Una unidad en la nube no puede rechazar el borrado en absoluto: quien entra en el servidor entra también en esa copia. Activa **Protección contra borrado (append-only)** solo donde el otro lado rechace de verdad el borrado. BombVault nunca poda allí entonces. **Para una emergencia** dice qué contiene el kit de recuperación para este destino.
4. **Para qué secciones.** Elige las secciones que copian aquí. La primera copia trae consigo todo el historial. Los elementos que eligieron sus propios destinos no reciben el nuevo por sí solos, y la ventana los nombra.

![Añadir destino: el primer paso, con cada proveedor como un mosaico.](assets/screenshots/storage/add-providers-dark.png#only-dark)
![Añadir destino: el primer paso, con cada proveedor como un mosaico.](assets/screenshots/storage/add-providers-light.png#only-light)

![Añadir destino: el paso de inicio de sesión tras una prueba correcta.](assets/screenshots/storage/add-signin-dark.png#only-dark)
![Añadir destino: el paso de inicio de sesión tras una prueba correcta.](assets/screenshots/storage/add-signin-light.png#only-light)

Algunos servicios piden un poco más al iniciar sesión:

- **SFTP y la Storage Box.** **Añadir clave** pone la clave pública de BombVault en el servidor con la **Contraseña de la cuenta**, que se usa para ese único inicio de sesión y no se guarda en ningún sitio. Debajo se muestra el comando para hacerlo a mano. La prueba muestra la **Clave del servidor** con su huella. BombVault la recuerda y rechaza el servidor si más tarde muestra una distinta.
- **Las claves S3 de un solo bucket**, como un token de objeto de Cloudflare R2 o una clave AWS acotada, pueden no listar los demás buckets. Escribe el bucket en **Bucket**, y la prueba y la lista de carpetas abren ese bucket.
- **Regiones y direcciones S3.** La región se lee de un endpoint que la nombra (AWS, Backblaze B2, Hetzner, Scaleway, OVH, Wasabi), Garage empieza con su propia región predeterminada, e IDrive e2 pide el endpoint que muestra su panel. Una dirección pegada con el bucket, como la muestra el panel de R2, rellena el servidor y el bucket. Un bucket creado desde el explorador de carpetas va a la región que nombra el formulario.
- **Clase de almacenamiento.** El selector ofrece solo las clases que acepta el servicio, y **Más ajustes** muestra solo las opciones que dan forma a un bucket nuevo.
- **Google Drive y OneDrive.** Los datos podados se borran definitivamente en lugar de ir a la papelera, donde seguirían contando para la cuota. Una unidad compartida de Google va en **Más ajustes**.
- **Los servidores SMB que no listan sus recursos compartidos** aceptan el nombre de uno en **Recurso compartido**, y la prueba abre ese recurso directamente.
- **FTP** pide su **Cifrado**: **Ninguno**, **TLS explícito (FTPES)** o **TLS implícito (FTPS, puerto 990)**.
- **OneDrive y Zoho WorkDrive.** Cuando un inicio de sesión abre varias unidades o espacios de trabajo, la ventana pregunta a cuál van las copias.
- **Un rest-server con un certificado autofirmado.** Pon el certificado como archivo PEM en la carpeta `certs` de la carpeta de datos de BombVault (`/config/certs` en el contenedor, `appdata/bombvault/certs` en Unraid). Las copias, las réplicas y la prueba de protección contra borrado confían en él sin reiniciar.

Los servicios S3 pasan por el propio backend S3 de restic, que es lo que permite aplicar una clase de almacenamiento y el bloqueo de objetos. A un rest-server llega restic directamente. Cualquier otro servicio pasa por el rclone que incluye BombVault, y su sección de la configuración de rclone se muestra en la tarjeta **rclone.conf** de la página del destino, con los secretos ocultos. Una exportación de ajustes lleva los destinos, y con las credenciales incluidas lleva también sus credenciales.

Guardar un destino que ya contiene copias de contenedores, VMs o carpetas, por ejemplo uno que quitaste y vuelves a añadir, lo añade a esas secciones como una copia sin marcar. No se copia nada allí hasta que lo marques, pero **Restaurar** ofrece enseguida las copias antiguas.

La carpeta de este servidor se llama **Local** en todas partes: en las páginas de almacenamiento, en el flujo, en un elemento y en Restauración. Con varias carpetas en este servidor, cada una va por su propio nombre. Un destino que configura el campo externo de una sección lleva el nombre de su lugar, la última carpeta de su dirección o, sin ella, su servidor, y recibe un número cuando otro destino ya va por ese nombre. Cualquier destino se puede renombrar en el campo **Nombre** de su tarjeta **Conexión**, también una carpeta de bucket que comparten varias secciones y la copia que describe el campo externo de una sección, y un nombre que le des se conserva. Donde dos destinos con el mismo nombre están uno junto a otro, cada uno lleva su servidor o carpeta.

### La página de un destino {#location-page}

Cada destino tiene una página propia. Muestra solo las tarjetas que se aplican a ese destino, en este orden:

![La página de un destino: Carpetas propias, Conexión y Credenciales.](assets/screenshots/storage/location-page-dark.png#only-dark)
![La página de un destino: Carpetas propias, Conexión y Credenciales.](assets/screenshots/storage/location-page-light.png#only-light)

- **Carpetas propias**: las secciones que escriben aquí primero sus copias. Las credenciales, la protección y los límites de este destino se aplican a ellas. **Usar como carpeta propia** mueve aquí la carpeta propia de más secciones, y **Cambiar** mueve una a otro sitio de nuevo. Consulta [La carpeta propia de una sección](#own-folder).
- **Se guarda directamente aquí**: cada sección cuyos elementos copian directamente a este lugar en lugar de a su carpeta propia, y cuántos elementos lo hacen. Un repositorio directo así toma todos sus ajustes de este destino. Uno al que ya no copia nada lo dice, y **Quitar** lo saca de la lista. Las copias de su lugar se quedan. Consulta [Repositorios directos](#direct-repositories).
- **Conexión**: el **Nombre**, el interruptor **Disponible** (desactivado significa que BombVault ya no escribe aquí, y los elementos que copian aquí fallan con un mensaje claro en lugar de acabar en otro sitio), la carpeta, **Compresión**, **Clase de almacenamiento** y **Fuera de las instalaciones**. En el destino **Local** lista la carpeta de cada sección en **Carpetas propias aquí**. **Cambiar conexión** prueba y guarda una dirección o credenciales nuevas. Cuando otros lugares inician sesión con las mismas credenciales guardadas, la ventana lo dice, y un cambio aquí da a este destino unas credenciales propias en lugar de reescribir las compartidas.
- **Credenciales**: qué credenciales guardadas usa (**Inicia sesión con**), qué otros destinos las usan también (**También las usa**), y **Editar**, que abre la ventana de las credenciales. Consulta [Credenciales](#sign-ins).
- **rclone.conf**: la sección de este destino en la configuración de rclone, para un destino que funciona mediante rclone.
- **Protección contra borrado**, con **Protección contra borrado (append-only)**, **Aceptar sin protección contra borrado**, **Probar la protección contra borrado ahora** y **Quién poda**, que dice si BombVault poda aquí la copia y las carpetas propias, según la tarjeta **Cuánto tiempo conservar** de este destino o los valores propios de una sección en **Reglas por sección**, o lo deja al otro lado. Una carpeta local muestra esta tarjeta solo cuando tiene una protección que se puede desactivar. Consulta [Protección contra borrado](#delete-protection-append-only).
- **Copiar**: **Cuándo copiar**, y el **Límite de subida (KiB/s)** y el **Límite de bajada (KiB/s)**. Un destino que contiene solo carpetas propias y ninguna copia muestra los límites como una tarjeta llamada **Ancho de banda**.
- **Cuánto tiempo conservar**: **Corto**, **Equilibrado**, **Largo** o **Valores propios**, con una línea de tiempo de qué copias conserva la regla. Consulta [Cuánto tiempo conservar](#keep).
- **Ya está bajo este destino**: secciones cuya dirección escrita a mano está en una carpeta de este destino. Adoptar una deja el repositorio donde está. Las credenciales, la clase de almacenamiento y la protección contra borrado pasan entonces a venir del destino. BombVault comprueba antes que las credenciales del destino abren el repositorio, y se niega a poner un repositorio append-only bajo un destino que no es append-only.
- **Reglas por sección**: el repositorio, la retención o la compresión propios de una sección en este destino. **Cambiar reglas** las abre, y **Volver al destino** devuelve una sección. Las credenciales, el ancho de banda y cuándo se ejecutan las copias vienen siempre del destino.
- Un recipiente que muestra lo lleno que está el destino, con la marca de dónde está lleno y cuántas semanas quedan al ritmo de crecimiento de las últimas semanas. Un destino que no informa de su tamaño dice **Tamaño desconocido**.

**Mover** y **Eliminar destino** están entre las acciones de la página, descritas en [Mover y eliminar](#move-remove).

Un destino remoto que contenía copias y de repente no contiene ningún repositorio, como un rest-server cuyo disco no se montó, no recibe uno nuevo y vacío por sí solo. Su página lo dice en cuanto una copia o una réplica encuentra que el repositorio ha desaparecido. Vuelve a montar el disco. Si vaciaste el repositorio antiguo a propósito, **Crear aquí un repositorio nuevo** empieza de nuevo, para todas las secciones del destino y sus repositorios a la vez. La protección es para un destino que se quedó en su sitio. Un destino que añades, o una sección que marcas en uno, empieza limpio: sin repositorio en esa dirección, la primera copia crea uno, incluso donde un destino quitado antes tenía uno.

Dos rechazos nombran lo que hay que corregir, en la ejecución de la copia y en la página del destino. **Este bucket no existe en el servidor** aparece en cuanto un servidor S3 lo dice, y BombVault no crea nada allí, así que un nombre de bucket mal escrito sigue a la vista. Un servidor que no responde en absoluto es otro caso: restic sigue reintentando durante unos 15 minutos por llamada, por si vuelve. **Aquí ya hay un repositorio de restic y la clave de esta instancia no lo abre** significa que las copias de allí pertenecen a otra `APP_KEY`.

Un servidor receptor que ejecuta otra instancia de tu grupo aparece entre los proveedores como **De tu grupo**. Consulta [Servidor receptor](#receiving-server).

BombVault comprueba sus destinos en cada arranque. Corrige lo que no necesita ninguna decisión tuya, como un enlace a una copia o repositorio que ya no existe, una copia en la carpeta propia de una sección o un repositorio directo cuya copia ha desaparecido, y escribe una línea en el registro por cada arreglo. Lo que no tiene una solución segura solo se registra.

![La misma página en un teléfono.](assets/screenshots/storage/location-page-phone-dark.png#only-dark)
![La misma página en un teléfono.](assets/screenshots/storage/location-page-phone-light.png#only-light)

### Cuánto tiempo conservar {#keep}

Cuánto tiempo se conservan las copias se fija por destino, en su tarjeta **Cuánto tiempo conservar**. La regla cubre las copias de ese destino y las carpetas propias que están allí, el destino **Local** incluido. Una sección puede conservar sus propios valores en un destino en **Reglas por sección**, y esos prevalecen sobre la tarjeta del destino para esa sección.

Una regla conserva una copia por día, semana, mes y año, tan atrás como fijes, y limpia el resto después de cada copia o réplica:

| Preajuste | Últimas copias | Días | Semanas | Meses | Años |
|---|---|---|---|---|---|
| **Corto** | 0 | 7 | 4 | 3 | 0 |
| **Equilibrado** | 0 | 7 | 4 | 6 | 2 |
| **Largo** | 0 | 14 | 8 | 12 | 3 |

**Valores propios** te deja fijar tú los cinco recuentos. Un 0 desactiva ese recuento, y una regla con todos los recuentos a 0 lo conserva todo. Los puntos de la tarjeta muestran cuáles de 40 copias de muestra conserva una regla, la más reciente en el extremo derecho. La muestra es la misma para todos los destinos, ya que la imagen se dibuja antes de que BombVault haya leído las copias de allí. Cuenta hacia atrás desde hoy, así que el número de puntos que quedan cambia con la fecha.

Una carpeta en este host y una carpeta remota escrita a mano también tienen su propia tarjeta, que cubre las carpetas propias de esa carpeta y ninguna otra. Hasta que la guardes, una carpeta así envejece según la regla que BombVault ya tenía para ella.

**Preguntar antes de conservar menos**, activado por defecto, pregunta antes de guardar un cambio que conserva menos copias, y dice cuántas de las 40 copias de muestra elimina la regla nueva. Se aplica a todos los destinos. **Volver al destino** en **Reglas por sección** hace la misma pregunta.

Un destino con protección contra borrado nunca se poda desde este equipo. Su otro lado conserva o descarta las copias antiguas según sus propias reglas. Mientras un elemento tiene una anomalía crítica abierta por datos perdidos, la retención conserva las copias antiguas de ese elemento hasta que la confirmes. Consulta [Anomalías](anomalies.md).

Una copia que contiene menos de lo que debería nunca hace caducar las anteriores. Es una copia que no lee ningún archivo donde las anteriores tenían alguno, a la que le falta una carpeta seleccionada o que no pudo leer algunos archivos. Su ejecución termina con un aviso que nombra lo que falta y dice que se conservaron las copias más antiguas, recibes el mismo mensaje en tus canales de notificación, y ninguna poda, ni en la carpeta propia del elemento ni en sus copias, toca las copias antiguas de ese elemento hasta que llegue una copia completa. Una primera copia, y una carpeta seleccionada que siempre estuvo vacía, cuentan como completas.

Para ver lo que borraría de verdad la próxima ejecución, abre **Vista previa** en **Ajustes, Destinos**. Lista **Lo que eliminaría la próxima ejecución** para la carpeta propia de la sección y para cada una de sus copias, por repositorio y por elemento con los puntos de restauración nombrados. No toma ningún bloqueo y no cambia nada.

![Cuánto tiempo conservar con Equilibrado elegido: los puntos muestran cuáles de las 40 copias de muestra se quedan.](assets/screenshots/storage/keep-dark.png#only-dark)
![Cuánto tiempo conservar con Equilibrado elegido: los puntos muestran cuáles de las 40 copias de muestra se quedan.](assets/screenshots/storage/keep-light.png#only-light)

#### Ejemplo: qué conserva Equilibrado {#example-retention}

Un contenedor se ha copiado cada noche a las 03:00 desde el 1 de octubre de 2024, así que el miércoles 4 de noviembre de 2026 tiene 765 copias. Su destino está en **Equilibrado**: 7 días, 4 semanas, 6 meses y 2 años.

Cada recuento conserva la copia más reciente de su día, semana, mes o año, y una copia que eligen varios recuentos se conserva una vez. Las semanas van de lunes a domingo:

| Regla | Copias que elige | Nuevas que añade |
|---|---|---|
| 7 días | 4 nov, 3 nov, 2 nov, 1 nov, 31 oct, 30 oct, 29 oct | las 7 |
| 4 semanas | 4 nov, 1 nov, 25 oct, 18 oct | 25 oct, 18 oct |
| 6 meses | 4 nov, 31 oct, 30 sep, 31 ago, 31 jul, 30 jun | 30 sep, 31 ago, 31 jul, 30 jun |
| 2 años | 4 nov, 31 dic 2025 | 31 dic 2025 |

Eso deja 14 copias de 765. La más antigua es del 31 de diciembre de 2025, así que hoy puedes volver atrás un poco más de diez meses. Gracias a la deduplicación ocupan poco más que una copia completa más lo que cambió entre medias.

**Años** merece una segunda mirada. Cuenta los años naturales que tienen una copia, empezando por el actual, y la copia más reciente de este año es la de hoy. Así que el primer año no añade nada a los otros recuentos, y el segundo añade la última copia del año anterior. Por eso el alcance de una regla varía a lo largo del año. Las mismas copias con cada preajuste:

| Preajuste | Copias conservadas hoy | Copia más antigua hoy | Alcance a lo largo del año |
|---|---|---|---|
| **Corto** | 10 | 30 de septiembre de 2026 | 1 a 2 meses |
| **Equilibrado** | 14 | 31 de diciembre de 2025 | unos 4 meses a principios de primavera, un año entero el 31 de diciembre |
| **Largo** | 30 | 31 de diciembre de 2024 | 1 año el 1 de enero, 2 años el 31 de diciembre |

Pasar de **Equilibrado** a **Corto** pregunta antes, por **Preguntar antes de conservar menos**. La pregunta dice cuántas de las 40 copias de la imagen elimina la regla nueva. Ese número sale de la muestra, así que cambia con la fecha y no cuenta tus propias copias. De las copias de este ejemplo, se irían el 31 de agosto, el 31 de julio, el 30 de junio y el 31 de diciembre de 2025. **Guardar de todos modos** aplica la regla, y la siguiente poda las borra de ese destino para siempre.

Para conservar los contenedores más tiempo que el resto en este destino, pulsa **Cambiar reglas** en **Reglas por sección**, elige **Contenedores** y fija **Valores propios** en **Reglas para Contenedores**, por ejemplo 14 días, 8 semanas, 12 meses y 2 años. Las demás secciones siguen la tarjeta del destino, y **Volver al destino** devuelve Contenedores.

### Credenciales {#sign-ins}

Unas credenciales son la clave o contraseña que BombVault usa para un servicio S3 o un rest-server. Las credenciales forman una sola lista, la tarjeta **Credenciales** de **Ajustes, Destinos**, y cada fila dice qué destinos las usan (**Usado por**).

- **Se guardan al añadir un destino.** Añadir destino guarda como credenciales las claves que escribes. El siguiente destino del mismo servicio puede elegirlas en **Credenciales guardadas** en lugar de **Claves nuevas**, y un cambio posterior en ellas se aplica a todos los destinos que las usan.
- **La predeterminada.** Una estrella marca las credenciales predeterminadas. Se usan donde no se haya elegido ninguna, como una dirección escrita a mano o una recuperación en una instalación nueva. Una fuente de recogida no las toma prestadas. **Usar como predeterminadas** da la estrella a otras.
- **Editar y Probar.** **Editar**, aquí o en la tarjeta **Credenciales** de un destino, abre una sola ventana para las credenciales. **Probar** inicia sesión en cada destino que las usa con lo que escribiste, antes de guardar nada. **Guardar** las almacena.
- **Quitar.** Unas credenciales solo se pueden quitar cuando ningún destino, fuente de recogida ni repositorio recibido las usa y ninguna copia antigua las necesita, y las predeterminadas solo se van como las últimas. Las copias antiguas las necesitan cuando una carpeta propia se trasladó desde un lugar que inicia sesión con ellas y las dejó allí. BombVault dice entonces qué lugar es.
- **Cambiar la conexión de un destino** nunca reescribe unas credenciales que comparten otros lugares. El destino recibe en su lugar unas credenciales propias, y la ventana dice de antemano que sus credenciales están compartidas. Para cambiar las compartidas en todas partes, edítalas en la tarjeta **Credenciales**.

![La tarjeta Credenciales: cada credencial guardada con lo que la usa, y la estrella en la predeterminada.](assets/screenshots/storage/signins-dark.png#only-dark)
![La tarjeta Credenciales: cada credencial guardada con lo que la usa, y la estrella en la predeterminada.](assets/screenshots/storage/signins-light.png#only-light)

Una carpeta de este servidor no necesita credenciales. SMB, WebDAV, SFTP, la Storage Box y las unidades en la nube guardan las suyas en la configuración de rclone, que muestra la tarjeta **rclone.conf** del destino, así que no aparecen en esta lista.

### SMB, WebDAV y otros remotos de rclone {#smb-webdav}

Un recurso compartido de Windows o Samba y un servidor WebDAV (Nextcloud, ownCloud, SharePoint u otro cualquiera) son mosaicos de proveedor en **Añadir destino**, en **NAS o servidor**. Rellena el host y el recurso compartido (SMB) o la dirección y el tipo de servidor (WebDAV), el usuario y la contraseña, y BombVault escribe por ti la sección de rclone. rclone ofusca la contraseña antes de guardarla.

Para un servicio que ningún mosaico cubre, el mosaico **Rclone** en **Configuración propia de rclone** acepta remotos pegados de un rclone.conf. Se añaden a los remotos que BombVault ya tiene y nunca los sustituyen, y se guardan cifrados. Para copiar una sección a uno de ellos, introduce `rclone:<remote>:<folder>` como repositorio externo de esa sección, por ejemplo `rclone:nas:backups/bombvault`. El recurso compartido o el bucket es el primer segmento de la ruta, no forma parte del nombre.

Es mejor camino que montar el recurso compartido en Unraid: restic desaconseja mantener un repositorio en un recurso CIFS montado, y aquí no se monta nada. NFS no tiene mosaico porque ni restic ni rclone tienen un backend NFS. Para NFS, monta la exportación en el host y úsala como carpeta local.

### Mover y eliminar {#move-remove}

- **Mover** lleva la copia de una sección a otro destino, con todo su historial. Todo se queda en el lugar antiguo. BombVault no borra nada allí. El movimiento se ejecuta en segundo plano y puede tardar: unos 9 minutos por cada 100 GB entre discos locales, horas por un enlace lento. El resumen lo muestra mientras se ejecuta.
- **Eliminar destino** ofrece dos formas. **Eliminar destino y conservar las copias** deja de escribir allí y deja las copias en su sitio. Puedes leerlas más tarde en **Restauración, De una carpeta de copias antigua**. **Eliminar destino y borrar las copias de seguridad de allí** lee primero lo que hay guardado allí y lo lista: qué copias se van, cuáles de ellas no están en ningún otro sitio y qué repositorios se quedan porque la protección contra borrado lo rechaza, hay elementos que aún copian directamente a ellos, otro destino o sección conserva allí sus copias, o no se pudieron leer. Los elementos que perderían su única copia se listan aparte y se conservan salvo que actives **Borrar también estas**. Confirmas escribiendo el nombre del destino, y el borrado se ejecuta en segundo plano.
- Esto vale para todo tipo de destino: uno que usan varias secciones, un destino configurado para una sección, un repositorio que configuraste y la carpeta en la que una sección escribe sus propias copias. Los elementos que copian a un repositorio eliminado vuelven a seguir a su sección, y también un valor predeterminado de sección que apuntaba allí. La carpeta de una sección no puede simplemente irse, porque toda sección necesita una. La ventana pide la carpeta a la que se mueven las secciones, y cada sección conserva bajo ella su nombre de carpeta.
- Borrar las copias de un destino también funciona sin contraseña de acceso. Si hay una, requiere un inicio de sesión en la propia instancia.

### Ejemplo: una copia externa en Backblaze B2 {#example-s3}

Los contenedores y la autocopia deberían vivir también en Backblaze B2, para que un servidor muerto no se los lleve consigo. Los mismos pasos sirven para Wasabi, Hetzner Object Storage y la mayoría de los demás servicios S3.

1. **En B2,** crea un bucket privado, como `tower-backups`, en una región cercana. En **Application Keys**, crea una clave con acceso de lectura y escritura, limitada a ese bucket, y anota su ID de clave y su clave de aplicación. B2 muestra la clave de aplicación una sola vez. Fija el ciclo de vida del bucket para conservar solo la última versión, o B2 conserva los datos podados como versiones ocultas y te los cobra.
2. **Proveedor.** En BombVault, abre **Destinos**, pulsa **Añadir destino** y elige **Backblaze B2** en **Servicio S3**.
3. **Iniciar sesión.** Introduce el ID de clave como **ID de la clave de acceso** y la clave de aplicación como **Clave de acceso secreta**. Introduce el endpoint del bucket en **Punto de conexión**, como `s3.eu-central-003.backblazeb2.com`, y BombVault lee la región de él. Escribe `tower-backups` en **Bucket**, porque esta clave abre un solo bucket.
4. **Probar conexión.** Debería decir **Conexión establecida** y listar las carpetas del bucket. Pulsa **Nueva carpeta** y llámala `bombvault`. BombVault pone dentro una carpeta por sección, como `bombvault/containers`.
5. **Protección.** La ventana explica que B2 puede conservar versiones antiguas con bloqueo de objetos y que BombVault no puede comprobarlo. Deja **Protección contra borrado (append-only)** desactivada salvo que el bucket rechace de verdad los borrados, para que BombVault pueda podar según las reglas de conservación del destino.
6. **Para qué secciones.** Marca **Contenedores** y **Autocopia**. La autocopia lleva tus ajustes, para que una instalación nueva pueda recuperarlos. La ventana dice cuántos elementos y, donde se sabe, cuánto historial trae la primera copia.
7. Pulsa **Guardar destino**.

La primera copia se ejecuta tras la siguiente copia. Para iniciarla ahora, abre **Ajustes, Destinos** y pulsa **Replicar ahora** en la fila de la sección de **Copias externas**. La cabecera de estado dice **Copia externa en curso**. La primera copia trae todo el historial, así que 200 GB por un enlace de subida de 100 Mbit/s tardan unas cuatro horas y media. Las siguientes copias envían solo lo nuevo.

Para mantener libre tu línea, abre la página del destino y fija el **Límite de subida (KiB/s)** en la tarjeta **Copiar**, por ejemplo en `5120`, que se lee como unos 5,0 MB/s (41,9 Mbit/s). Revisa **Cuánto tiempo conservar** en la misma página ya que estás.

Para ver que funcionó, mira **Estado de protección** en el resumen, donde **Externo** pasa a **copia al día**, y la pestaña **Copias de seguridad** de un contenedor, donde sus instantáneas nombran ahora `tower-backups` como segundo lugar. Luego pulsa de nuevo **Descargar kit de recuperación** en **Ajustes, Restauración**, porque el kit lista el destino nuevo y sus credenciales.

**Cloudflare R2 funciona igual,** con un token de objeto en lugar de una clave. Pega la dirección S3 que muestra el panel de R2, con el bucket, y BombVault rellena el servidor y el bucket.

### Ejemplo: un rest-server que ejecutas tú {#example-rest-server}

Un rest-server en modo append-only rechaza los borrados, así que una copia en él sobrevive aunque alguien se haga con tu servidor. Basta un segundo equipo, en casa de un amigo o en el garaje. Un BombVault emparejado puede configurarlo por ti, consulta [Servidor receptor](#receiving-server). Para ejecutarlo a mano en cualquier host Docker:

```sh
docker run -d --name rest-server --restart unless-stopped \
  -p 8000:8000 \
  -v /mnt/user/backups/rest-server:/data \
  -e OPTIONS="--append-only --private-repos" \
  restic/rest-server
docker exec -it rest-server create_user bombvault
```

La última línea pide una contraseña y escribe el usuario en el archivo `.htpasswd` de la carpeta de datos.

1. En BombVault, pulsa **Añadir destino** y elige **rest-server** en **NAS o servidor**.
2. Introduce la dirección con el usuario como su carpeta, como `http://192.168.2.20:8000/bombvault`, y el **Nombre de usuario** y la **Contraseña**. Pulsa **Probar conexión**.
3. En **Protección**, activa **Protección contra borrado (append-only)**. BombVault nunca poda allí entonces y lo deja al otro lado.
4. Marca las secciones y pulsa **Guardar destino**.
5. Cuando la primera copia esté ahí, abre la página del destino y pulsa **Probar la protección contra borrado ahora**. BombVault intenta borrar un objeto de prueba. La tarjeta debería decir **Último: borrado rechazado**, y **Protección contra borrado** en el resumen pasa a **comprobada**. Si dice **Último: borrado no rechazado**, falta `--append-only`.

Como nada poda esta copia desde aquí, fija **Presupuesto de crecimiento (GB, 0 = desactivado)** en la ventana de la sección, que se abre con **Editar** en su fila de **Copias externas**, para enterarte antes de que se llene el disco. Con un certificado autofirmado, consulta la nota sobre `certs` más arriba.

## La carpeta propia de una sección {#own-folder}

Cada sección tiene una carpeta propia: el repositorio en el que se escriben primero sus copias. Las copias a otros destinos salen de ahí. Una instalación nueva mantiene las carpetas propias en este servidor, bajo `/mnt/user/bombvault`, y juntas forman el destino **Local**.

Una carpeta propia también puede estar en un destino que ya tienes, como un NAS, un rest-server o un bucket S3. La sección copia entonces directamente allí, sin una copia local de por medio, y la carpeta propia sigue las credenciales, la protección contra borrado y los límites de ancho de banda de ese destino. BombVault la poda tras cada copia según la tarjeta **Cuánto tiempo conservar** del destino, y los valores propios de una sección en **Reglas por sección** tienen prioridad sobre la tarjeta. El destino lista la sección en su tarjeta **Carpetas propias**, y **Destinos por sección** muestra el destino como el hogar de la sección.

### Mover una carpeta propia {#move-own-folder}

1. Abre el destino que debe contener la carpeta propia y elige **Usar como carpeta propia**, o **Añadir secciones** en su tarjeta **Carpetas propias**. Para mover varias secciones a la vez, elige **Mover carpetas propias** junto a **Carpetas propias aquí** en la tarjeta **Conexión** del destino Local.
2. Elige las secciones en **Secciones** y el lugar en **Mover a**. **Otro lugar** acepta una carpeta de este host o la dirección de un repositorio remoto escrita a mano, con unas credenciales de tus credenciales guardadas.
3. Lee **Qué pasa**. Para cada sección dice adónde van las copias nuevas, si una copia que ya está en el lugar nuevo pasa a ser la carpeta propia y qué copias se copian primero.
4. **Copiar las copias de seguridad ya hechas** está activado por defecto. Si se desactiva, las copias antiguas se quedan donde están, pero el elemento ya no las lista. **De una carpeta de copias antigua** en la página Restauración sigue leyéndolas con la dirección de la carpeta y, para una carpeta remota, sus datos de acceso.
5. Elige **Cambiar**. Cuando la tarjeta **Cuánto tiempo conservar** del lugar nuevo conserva menos que la regla actual de la sección, BombVault pregunta primero **¿Conservar menos?** Ahí, **Conservar los valores actuales** de esa sección **en Reglas por sección** se lleva los valores de hoy, **Guardar de todos modos** sigue adelante y **Atrás** lo deja todo como estaba. Cuando hay que copiar antes las copias, BombVault las copia y luego cambia la carpeta propia. Cuando ya están allí, las comprueba antes del cambio.

Tras el movimiento, las copias nuevas van a la carpeta propia movida, también para una sección que tenía **Local** desactivado y copiaba directamente a una réplica. Una copia que ya estaba en el lugar nuevo pasa a ser la carpeta propia, y la copia allí termina. Donde esa copia conservaba algunas copias más tiempo que las reglas de conservación de la sección, la sección conserva lo que conserve cualquiera de las dos reglas, así que no se pierde ninguna.

Cuando el movimiento no puede seguir adelante, la ventana dice por qué en un recuadro bajo **Qué pasa**. Cuando solo estorba copiar las copias antiguas, desactivar **Copiar las copias de seguridad ya hechas** permite mover la carpeta propia sin ellas.

Una sección siempre necesita una carpeta propia. Eliminar el destino que la contiene pide la carpeta a la que se mueve la sección, consulta [Mover y eliminar](#move-remove). Una carpeta propia a la que ya no van las copias nuevas, porque la sección copia directamente a una réplica, se marca **(apagado)** donde un destino lista quién la usa.

![La ventana que mueve una carpeta propia, con Qué pasa rellenado para una sección.](assets/screenshots/storage/move-own-folder-dark.png#only-dark)
![La ventana que mueve una carpeta propia, con Qué pasa rellenado para una sección.](assets/screenshots/storage/move-own-folder-light.png#only-light)

#### Ejemplo: Contenedores se mudan a un NAS {#example-own-folder}

El pool de caché se está llenando, y tienes un destino llamado `nas`, un recurso compartido SMB en un segundo equipo. La sección Contenedores tiene 61 copias en su carpeta propia de este servidor y copia a Backblaze B2.

1. Abre **Destinos**, luego `nas`, y pulsa **Usar como carpeta propia**.
2. Marca **Contenedores** en **Secciones**.
3. Lee **Qué pasa**. Dice que las copias nuevas de Contenedores van a la carpeta de `nas`, que las 61 copias de Local se copian allí primero, que las copias siguen yendo a Backblaze B2 y que la protección, los límites de ancho de banda y la alarma de crecimiento vienen de `nas`.
4. Deja activado **Copiar las copias de seguridad ya hechas** y pulsa **Cambiar**.

BombVault copia las copias en segundo plano y luego cambia la carpeta propia. La página de `nas` lista Contenedores en **Carpetas propias**, un elemento que sigue a su sección nombra `nas` como su hogar, y la siguiente copia acaba allí, con la copia a B2 saliendo desde `nas`. A partir de ahora, la tarjeta **Cuánto tiempo conservar** de `nas` poda la carpeta. La carpeta antigua de este servidor se queda donde está, y BombVault no borra nada allí.

### Escribir a mano una dirección remota {#remote-primary-repositories}

La tarjeta **Conexión** del destino Local sigue aceptando una dirección remota para la carpeta propia de una sección: **Cambiar** junto a la sección abre una ventana con un interruptor **Local / Remoto**. **Remoto** acepta una dirección de restic como `s3:...`, `rest:http://host:8000/repo`, `sftp:user@host:/repo` o `rclone:remote:bucket/path`, y **Ajustes de seguridad del repositorio primario remoto** añaden una prueba de conexión, límites de subida y bajada, protección append-only con la misma prueba de manipulación que recibe un destino externo, y una alarma de presupuesto de crecimiento.

Una dirección así inicia sesión con las credenciales predeterminadas. Cuando es exactamente la carpeta que un destino que tienes guarda para la sección, sigue a ese destino por sí sola, incluidos la tarjeta del destino y su interruptor **Disponible**, salvo que inicie sesión de otra forma o sea append-only donde el destino no lo es. Una regla que podaba la carpeta hasta entonces se queda como regla propia de la sección en **Reglas por sección**. Cuando está en otra carpeta de ese destino, la página de ese destino lista la sección en **Ya está bajo este destino**, y adoptarla le da las credenciales, la clase de almacenamiento y la protección contra borrado del destino. Mover la carpeta propia a un destino con **Usar como carpeta propia** es la forma más sencilla, porque la ventana comprueba las copias ya hechas antes de cambiar nada.

## Destino por elemento {#placement}

La tarjeta **Calendario y destino** de la pestaña Ajustes de un elemento tiene una fila de botones: **Local** y un botón por cada destino externo de la sección. Los botones iluminados reciben las copias del elemento.

- Con **Local** iluminado, el elemento se escribe en el repositorio que se muestra en **Almacenado en** y se copia a cada otro destino iluminado. Oscurece un destino y no recibe nada nuevo de este elemento. Local solo no copia a ninguna parte, lo que sirve para datos que ya tienen una segunda copia, por ejemplo un recurso compartido que vive en un NAS.
- Con **Local** oscuro, el elemento se escribe directamente en el repositorio directo del primer destino iluminado y se copia desde allí a los demás iluminados. La primera vez, un diálogo crea ese repositorio directo.
- Un botón se queda iluminado, porque una copia necesita un lugar adonde ir. Para dejar un elemento fuera de las copias, usa **No copiar**.

BombVault nunca mueve copias entre repositorios. Un elemento que sigue a su sección acompaña a la sección cuando cambia su valor predeterminado: su siguiente copia va al lugar nuevo, y las copias que tiene se quedan donde están. Un destino elegido en el propio elemento queda fijo desde su primera copia. La tarjeta **Repositorio** de la pestaña Ajustes lo nombra. Las réplicas pueden cambiar en cualquier momento. Un destino que ya no recibe un elemento conserva las copias que tiene y las recorta según su propia retención en la siguiente ejecución externa de la sección. En los destinos append-only no se puede borrar.

El lugar que un elemento dejó conserva sus copias. En la pestaña **Copias de seguridad** del elemento y en **Restauración** figura como **Solo copias anteriores**, una restauración aún puede leer de él, y una poda allí deja en paz esas copias.

**Seguir el valor por defecto** devuelve a su sección un elemento con una elección propia. Pregunta antes, dice adónde van entonces las copias nuevas y nombra lo que cada destino deja de recibir y lo que subiría la siguiente ejecución.

Bajo la fila la tarjeta dice adónde va el elemento y qué hay realmente allí: cuántos sitios lo contienen, cuándo se vio por última vez cada destino y si se cumple 3-2-1. Un sitio es el servidor con los datos originales, cada destino externo y cada repositorio marcado como **Fuera de las instalaciones**. BombVault comprueba copias y sitios. No comprueba la parte de "dos medios" de 3-2-1.

### Valores predeterminados por sección {#placement-defaults}

La tarjeta **Destinos por sección** de la página Destinos dibuja el valor predeterminado de cada sección como un flujo 3-2-1. Cada línea de una sección a un destino es un interruptor, **Local** incluido, para cada sección, también la unidad flash, los conjuntos de datos ZFS y la autocopia. Un clic en una línea o en un punto lo activa o desactiva al instante, y las líneas discontinuas están desactivadas. Solo desactivar el **Local** de una sección pregunta antes, en el mismo diálogo que la tarjeta predeterminada, porque la sección copia entonces directamente a una réplica. **Cancelar** lo deja todo como estaba. **Gráfico / Interruptores** muestra lo mismo como filas de botones. Un interruptor decide adónde van las copias nuevas. Los elementos sin elección propia, y las carpetas de proyecto de los stacks de Compose, lo siguen al instante, así que su siguiente copia va a los lugares nuevos. Las copias ya guardadas se quedan donde están y aún se pueden restaurar desde allí. El flash, los conjuntos de datos ZFS y la autocopia copian a su carpeta local y copian al exterior desde esa copia. Su línea a **Local** también se puede desactivar. Sus copias van entonces directamente a sus destinos externos, y un elemento ZFS con una réplica y ningún destino solo envía su réplica.

Con **Local** desactivado y solo destinos externos activados, una sección copia directamente a ellos, y BombVault elige allí el repositorio él mismo. Los contenedores, las VMs y los conjuntos de carpetas van al repositorio directo del primer destino activado, descrito más abajo, y se copian desde allí a los demás. El flash, los conjuntos de datos ZFS y la autocopia escriben cada copia en todos los destinos activados, y cada destino las conserva según su propia retención.

Una sección con todas las líneas desactivadas dice **Sin copia de seguridad**. Sus elementos no reciben copias y cuentan como sin protección.

Los elementos que eligieron sus propios destinos en su página los conservan. El flujo dice cuántos hay en una sección, por ejemplo **2 elementos difieren**, y lleva a ellos: a la pestaña Ajustes del elemento cuando hay uno, a una lista con adónde copia cada uno cuando hay varios. **Seguir el valor por defecto** allí devuelve un elemento.

Un destino externo nuevo recibe todos los elementos que no están fijados en Local. La ventana que lo añade dice cuántos elementos y, donde se sabe, cuánto historial es.

### Repositorios directos {#direct-repositories}

Un repositorio directo es donde los elementos copian directamente a un destino externo. BombVault lo pone junto a la carpeta del destino con `-direct` añadido al nombre o, cuando el destino está en la raíz de su bucket, de su remoto o de un usuario de rest-server, en una carpeta dentro de él con el nombre de la sección, como `containers-direct`. En el flujo de la página Destinos se crea sin preguntar. Desactivar Local para un solo elemento, de modo que un destino sin repositorio directo se convierta en su hogar, abre un diálogo con el destino sugerido, por ejemplo `s3:https://s3.eu-central-003.backblazeb2.com/bucket/containers-direct`, y una prueba de conexión que no crea nada. **Crear y usar** crea el repositorio y apunta el elemento a él. Un repositorio directo no tiene ajustes propios. Sigue el interruptor, las credenciales, la clase de almacenamiento, los límites, el ajuste append-only y la retención de su destino, y cambia con ellos. La página del destino lo lista en **Se guarda directamente aquí**, con cada sección que copia directamente a él y cuántos elementos lo hacen. Uno al que ya no copia nada lo dice allí y se puede quitar, y las copias de su lugar se quedan. Cuando una clave nueva del destino no puede abrirlo, el repositorio directo conserva la clave que tiene y el guardado lo dice. Un elemento en un repositorio directo se copia desde allí a los demás destinos iluminados, nunca al destino al que pertenece el repositorio. Sus instantáneas llevan la etiqueta `bv:direct`, y todas las demás pasadas de retención las conservan, de modo que un repositorio directo que perdió su enlace con su destino nunca envejece según las reglas locales. A B2 se llega por su endpoint S3, con el ID de clave y la clave de aplicación introducidos como claves S3. Una clave limitada a la propia carpeta del destino no puede llegar a la carpeta contigua, así que limita la clave a la carpeta de encima.

### Fuera de las instalaciones {#off-the-premises}

Un destino o un repositorio se puede marcar como **Fuera de las instalaciones**. Los remotos empiezan marcados. Desactívalo para un rest-server en el mismo edificio. La marca solo cuenta sitios y 3-2-1 en las tarjetas. No cambia ninguna copia.

Una réplica ZFS es una copia fuera de las instalaciones, pero no una copia de seguridad con historial. Conserva solo las instantáneas que fijes, y las versiones antiguas y los archivos sueltos siguen saliendo de las copias de seguridad. Va de un servidor ZFS directamente al otro, por SSH a un servidor ZFS o al endpoint de recepción de una instancia emparejada. El grupo de emparejamiento solo transporta la solicitud. Consulta [Réplica](zfs-datasets.md#replica).

### Después de una reconstrucción {#after-a-rebuild}

Las elecciones de copia viven en los propios ajustes de BombVault. Tras una reconstrucción mediante **Descubrir copias** o el Asistente de emergencia sin ajustes restaurados, han desaparecido, y copiarlo todo enviaría de nuevo a B2 los elementos que habías dejado fuera. Lo mismo ocurre cuando una sección que nunca copió fuera desde esta base de datos encuentra copias propias en un destino, o copias más antiguas en su carpeta propia, por ejemplo después de añadir un destino sobre las copias de una instalación anterior con la misma `APP_KEY`. Las copias externas de esa sección esperan entonces tu decisión.

**Necesita atención** en el resumen y la página de cada destino al que copia la sección preguntan si las copias son propias de este servidor. **Adoptarlas como propias de este servidor** confirma el valor predeterminado de la sección, y sus copias vuelven a ejecutarse desde la siguiente pasada. **Revisar primero las reglas** lleva a la línea de la sección en **Destinos por sección**, donde ves adónde va cada elemento y puedes volver a fijar una regla perdida en la página del elemento. La copia sigue esperando ahí hasta que pulses **Adoptarlas como propias de este servidor**, que primero muestra lo que copiaría la próxima ejecución y los nombres de las copias que no tienen elemento, que puedes dejar fuera. La copia más reciente de cada elemento en espera lo indica en su ejecución, con un enlace a la decisión, y se envía una notificación cuando las notificaciones están activadas. Importar un archivo de ajustes devuelve reglas y valores predeterminados pero no toma la decisión por ti.

![Necesita atención tras una reconstrucción: los conjuntos de carpetas encontraron copias de antes de esta base de datos, y sus copias externas esperan la decisión.](assets/screenshots/offsite/copy-decision-dark.png#only-dark)
![Necesita atención tras una reconstrucción: los conjuntos de carpetas encontraron copias de antes de esta base de datos, y sus copias externas esperan la decisión.](assets/screenshots/offsite/copy-decision-light.png#only-light)

## Protección contra borrado (append-only) {#delete-protection-append-only}

Activa la protección contra borrado en un destino externo para que el ransomware, o un host comprometido, no pueda borrar ni reescribir tus copias. El otro lado (un `restic/rest-server` ejecutándose en modo `--append-only`) la **aplica**. BombVault solo la **verifica** y nunca muestra verde por una mera afirmación de configuración. Una carpeta de este servidor no puede rechazar el borrado, porque quien controla BombVault controla también la carpeta, así que los destinos locales no ofrecen el interruptor.

El interruptor pertenece al destino: una sección que copia allí lo toma del destino, así que actívalo desde la página del destino en **Destinos**. La **Comprobar la protección contra borrado** programada se ejecuta solo mientras una sección copia a un destino con el interruptor activado. El otro lado tiene que haberse iniciado con `--append-only`. Con la imagen `restic/rest-server`, en TrueNAS como en cualquier otro host Docker, eso va en la variable `OPTIONS`.

Algunos lugares no pueden rechazar un borrado en absoluto, por ejemplo una unidad en la nube como OneDrive. Si conservas copias allí a propósito, activa **Aceptar sin protección contra borrado** en la página del destino. El resumen muestra entonces la protección contra borrado que falta en una sección que copia solo a destinos aceptados como un riesgo aceptado, un anillo gris en lugar de un punto ámbar, y el color de la sección sigue a sus otras comprobaciones. No cambia nada más: las copias de allí aún se pueden borrar, BombVault las sigue podando según las reglas de conservación del destino, y el interruptor desaparece cuando se activa la protección contra borrado. Una copia que se mueve a otro lugar pierde la aceptación. Activar la protección contra borrado también la borra.

La forma más rápida de conseguir una copia append-only es un BombVault emparejado con un [servidor receptor](#receiving-server): ejecuta el rest-server por ti, en modo append-only.

!!! note "Un borrado correcto bajo `/locks/` es lo esperado"
    Append-only no significa que nunca se pueda quitar nada. restic tiene que tomar y liberar sus propios bloqueos, así que `/locks/` sigue siendo escribible y borrable por diseño. Las instantáneas y los datos que hay detrás, que es lo que perseguiría el ransomware, no se pueden quitar. Si pruebas tú mismo el otro lado, un borrado que tiene éxito bajo `/locks/` es el comportamiento correcto y no un agujero en la protección.

!!! warning "Los repositorios append-only nunca se podan desde este equipo"
    Un destino append-only nunca poda instantáneas antiguas desde aquí. Podar es trabajo del otro lado. Fija una **alarma de presupuesto de crecimiento** para que te avisen antes de que el tamaño del repositorio se desboque.

## Prueba de manipulación {#tamper-test}

BombVault demuestra periódicamente la garantía append-only intentando de verdad un borrado contra el repositorio externo, dirigido a un objeto que no existe. **Probar la protección contra borrado ahora** en la página de un destino lo ejecuta a demanda, y **Comprobar la protección contra borrado** en **Ajustes, Verificación, Comprobaciones automáticas** lo ejecuta según un calendario.

Desactivar **Comprobar la protección contra borrado** no oculta una carencia. Un destino con protección contra borrado que nunca se probó figura entonces como "sin demostrar, su comprobación está desactivada", y un destino sin protección contra borrado figura como "ninguna" en ambos casos.

Una copia desactivada no se prueba, y el destino lo dice. La prueba de conexión se ejecuta igualmente sobre ella, porque solo lee, así que se puede comprobar una copia antes de activarla.

- **Rechazado** significa protegido.
- **Aceptado** significa no protegido.
- Un resultado **no concluyente** (servidor inalcanzable, error de autenticación) nunca cambia el veredicto guardado.

Un cambio real de protegido a no protegido dispara una única alerta.

## Simulacros de DR {#dr-drills}

BombVault ofrece dos niveles de prueba de que tus copias son realmente restaurables, no solo que existen.

- **Simulacros de verificación de restauración (local).** BombVault ejecuta periódicamente `restic check --read-data-subset` (acotado, nunca una restauración completa que llene el disco). La cadencia está en **Ajustes, Verificación, Comprobaciones automáticas**, y la tarjeta **Comprobar las copias** del mismo mosaico muestra el resultado.
- **Restauración de prueba (externa).** BombVault restaura un objetivo real del repositorio externo en un entorno aislado, lo verifica archivo por archivo y byte por byte, y luego limpia. Esto demuestra que puedes recuperarte desde lo externo, no solo que el repositorio responde.

La tarjeta **Estado de protección** del resumen lo agrupa por sección en cuatro comprobaciones: una copia al día en este servidor, una copia externa, protección contra borrado allí y una restauración de prueba desde allí. Un clic en una sección muestra los detalles, y la tarjeta solo se pone en verde con hechos verificados. Un destino que aceptaste sin protección contra borrado cuenta allí como un riesgo aceptado, no como una advertencia.

## Emparejar instancias {#pairing}

Varios BombVault trabajan juntos como un grupo: se muestran el estado de protección del otro, asumen roles unos para otros y pueden guardarse la clave unos a otros. Una instancia se une al grupo con doce palabras.

![La página de emparejamiento con sus tres tarjetas de pasos.](assets/screenshots/offsite/pairing-dark.png#only-dark)
![La página de emparejamiento con sus tres tarjetas de pasos.](assets/screenshots/offsite/pairing-light.png#only-light)

En la primera instancia abre **Ajustes, Emparejamiento** y pulsa **Generar frase**. Tres tarjetas de pasos muestran cómo va. Doce palabras aparecen en una ventana con un botón **Copiar**. En cada otra instancia abre el mismo sitio, pulsa **Introducir frase** y pégalas o escríbelas, o pulsa **Pegar** en esa ventana. Una palabra que no está en la lista se nombra con su posición mientras escribes, y la última palabra lleva una suma de comprobación, así que una palabra mal escrita o cambiada se detecta antes de que nada se empareje. Genera la frase en una sola instancia: dos instancias que generan una cada una forman dos grupos distintos. Si nadie aparece en un minuto, la página ofrece dos salidas: mostrar de nuevo las palabras para introducirlas allí, o introducir las palabras de la otra instancia y unirse a su grupo en un solo paso. El emparejamiento funciona sin contraseña de acceso, pero define una: sin ella cualquiera que pueda abrir la interfaz web puede leer las palabras y, a través del grupo, pedir roles que entregan la contraseña de restic de las instancias que hay en él. La tarjeta de emparejamiento lo dice hasta que se define una contraseña. Con una contraseña, volver a mostrar la frase la pide. **Salir del grupo** saca de nuevo a una instancia. La tarjeta **Instancias** de la página de emparejamiento lista los miembros del grupo. La imagen de quién hace qué por quién está en la página Instancias.

Cualquiera que conozca las palabras puede unirse al grupo, así que trátalas como una contraseña.

**Cómo se alcanzan los miembros.** Cada instancia aprende su propia dirección en la red desde tu navegador en el momento en que inicias sesión, listada en **Direcciones conocidas** en la tarjeta **Esta instancia** de la página de emparejamiento junto con las que la instancia encontró por sí misma. Corrígela allí si hay un proxy inverso o un puerto inusual por delante. En la misma red los miembros anuncian esa dirección por multicast y hablan directamente, y donde el multicast no puede cruzar una red de contenedores, como el bridge predeterminado de Docker, una instancia busca en su propia subred a las demás con una llamada firmada que solo puede responder un miembro del grupo, de modo que el emparejamiento aún termina en segundos sin repetidor. Si no aparece nada, **¿No la encuentras?** acepta una dirección a mano, para otra subred o un puerto no estándar. Las instancias en redes distintas pasan por un repetidor, elegido en una fila de cuatro en la tarjeta **Repetidor** de la misma página. Cada forma viene con una imagen:

- **Repetidor del proyecto** (el predeterminado): `parleyport.halleluja.design`, el repetidor que usa también KnightLoader. No hay que configurar nada.
- **Repetidor propio**: el contenedor [**ParleyPort**](https://github.com/junkerderprovinz/parleyport) de las Community Apps de Unraid (también en **Ajustes, Apps**). Introduce su dirección en cada instancia que deba usarlo.
- **Instancia de BombVault**: una de tus instancias que ya es accesible desde fuera, con **Actuar como repetidor** activado. Esa instancia responde entonces en `/relay/connect` en su propia dirección, detrás del proxy inverso y el certificado que ya tiene, y deja entrar solo a tu grupo. Introduce su dirección en cada instancia que deba usarlo.
- **Sin repetidor**: los miembros se encuentran automáticamente en la misma red, y en ningún otro sitio.

**Qué ve el repetidor.** Un botón del mismo nombre junto a las cuatro formas muestra esto para la forma que elegiste. Cada llamada entre miembros se sella con AES-256-GCM bajo una clave derivada de las doce palabras, y esa clave nunca sale de tus instancias. El repetidor conoce un hash que agrupa las conexiones, para qué instancia es un mensaje, lo grande que es y cuándo pasa. Una llamada directa en la red local se sella igual y además se firma, de modo que nada depende del certificado autofirmado que sirve una instancia. Las llamadas que entregan algo (respuestas de roles, una contraseña de restic, inicios de sesión de receptor, depósitos de claves) se firman además con una clave del miembro que las envía, fijada la primera vez que se la ve. Consulta [Modelo de seguridad](configuration.md#security-model).

**Qué viaja por el grupo.** El estado de protección en la página Instancias, una solicitud para comprobar ahora una sección, las solicitudes de roles y sus respuestas, las ofertas de almacenamiento, y lo que necesita un receptor o recolector una vez permitido su rol: las ubicaciones de los repositorios de la otra instancia y su contraseña de restic. Los datos de las copias nunca. Siguen yendo directamente a los backends de restic. Tampoco la APP_KEY: la contraseña de restic abre los repositorios de esa instancia y nada más, ni sus secretos guardados, sesiones ni códigos de recuperación.

**Elementos anteriores al emparejamiento.** Las instancias añadidas con un token de flota, y los repositorios recibidos y las fuentes de recogida configurados con la APP_KEY de la otra instancia, se quedan tras la actualización y se marcan **Emparejar de nuevo**. Siguen funcionando: en su primer arranque BombVault sustituye cada APP_KEY guardada por la contraseña de restic derivada de ella. Empareja las dos instancias, edita luego el elemento y elige su instancia. Hasta entonces esos elementos están en la tarjeta **Sin instancia** de la página Instancias.

El único sitio que aún acepta una APP_KEY a mano es [De una carpeta de copias antigua](#restore-from-another-bombvault-repo), para el caso en que la otra instancia ha desaparecido y no puede responder en un grupo.

## Instancias y roles {#roles}

La página **Instancias** (un mosaico de ajustes que se puede anclar a la barra lateral) muestra el grupo como una red de quién hace qué por quién, una tarjeta para esta instancia y una tarjeta para cada otro miembro, los teléfonos Android y los servidores ZFS. **Emparejamiento**, abajo a la derecha, lleva a la página de emparejamiento. Qué partes están activadas se fija en **Ajustes, Instancias, Varios servidores**: **Instancias**, **Receptor** y **Recogida**.

![La página Instancias: el grupo como una red, y una tarjeta por instancia con sus mosaicos de roles, uno de ellos Inactiva.](assets/screenshots/offsite/instances-dark.png#only-dark)
![La página Instancias: el grupo como una red, y una tarjeta por instancia con sus mosaicos de roles, uno de ellos Inactiva.](assets/screenshots/offsite/instances-light.png#only-light)

Cada tarjeta muestra el estado de protección del miembro y sus roles como mosaicos. Un rol es algo que una instancia hace por otra:

- **Receptor** guarda las copias externas de este servidor en un rest-server en modo append-only. Este servidor envía después de cada copia y no puede borrar nada allí, ni aunque alguien se haga con él.
- **Recolector** recoge las copias de este servidor según su propio calendario. Este servidor no tiene que estar despierto para ello. La instancia que recoge necesita **Recogida** activada antes de poder permitir la solicitud.
- **Servidor ZFS** recibe las réplicas ZFS de este servidor. Cada elemento ZFS lo elige en **Réplica** en sus ajustes.
- **Guardián de claves** guarda una copia sellada del kit de recuperación de este servidor. Consulta [Guardián de claves](#key-keeper).

Activar un rol envía una solicitud, y no empieza nada hasta que alguien de la otra instancia pulse **Permitir**. La solicitud aparece en la tarjeta de esa instancia como una insignia, y sus **Detalles** la listan con **Permitir** y **Rechazar**. Un rol permitido se puede revocar allí en cualquier momento. Una línea discontinua en la red es un rol que aún espera. Una instancia con BombVault 9.x no puede responder a una solicitud, así que un rol está activo allí en cuanto lo activas, y una solicitud de una instancia 9.x espera aquí a **Permitir**.

**Detalles** en una tarjeta abre el resto: los repositorios recibidos y las fuentes de recogida de ese miembro, ofertas de almacenamiento, supervisión, **Eliminar** y **Olvidar la clave**.

Un miembro puede ofrecer su propio almacenamiento externo a otro miembro por el grupo. El otro administrador ve la oferta en los **Detalles** del miembro y la acepta o la rechaza. Aceptar crea unas credenciales guardadas y un destino normal, tras las mismas comprobaciones que un destino que escribes tú. El destino llega con la protección contra borrado activada, porque la receta que muestra la oferta es un rest-server append-only, así que **Cuánto tiempo conservar** no se aplica ahí. Cada oferta tiene sus propios datos de acceso y su propia carpeta, así que ofrecer una sección de nuevo, al mismo miembro o a otro, deja en paz las copias del primer socio: añade la nueva línea a `.htpasswd` y mantén el servidor en marcha. Por este camino viajan solo los datos de conexión, nunca datos de copias. **Comprobar ahora** pide a un miembro que verifique el repositorio de una sección. Nada en la página puede iniciar una copia, una restauración ni borrar nada en otro equipo.

Las instancias apagadas y los teléfonos con la app cerrada conservan su tarjeta como **Inactiva**, en gris, con cuándo se vieron por última vez, y aparecen atenuadas en la red hasta que las **Eliminas**. Justo después de arrancar este servidor, una tarjeta así dice **Comprobando** hasta que el grupo vuelve. **Eliminar** funciona en una tarjeta así tenga las partes que tenga activadas, y además termina las solicitudes abiertas del miembro, sus ranuras de recepción ZFS inactivas y sus solicitudes de clave en espera. La tarjeta **Instancias** de la página de emparejamiento también lista estos miembros como **Inactiva**.

## Receptor y recolector {#receiver}

En el equipo que **recibe** copias append-only de otro BombVault, los repositorios recibidos te dan una supervisión independiente y de solo lectura en el hardware receptor, de modo que un fallo silencioso en el otro extremo no pase desapercibido. Activa **Receptor** en **Ajustes, Instancias**, luego abre los **Detalles** de la instancia emisora en la página Instancias y añade el repositorio recibido (de solo lectura, abierto con la contraseña de restic de la instancia emisora, que recibe por el [grupo de emparejamiento](#pairing)). Obtienes:

- **Un inventario por origen**, para ver exactamente qué contenedores, VMs y conjuntos de carpetas han llegado. Las copias que la instancia emisora hizo antes de tener un [host propio](#backup-host) cuentan para el mismo origen.
- **Última recepción** por origen, para saber lo reciente que es cada uno.
- **Un `restic check` independiente** ejecutado en el hardware receptor, de modo que la integridad se verifica donde están realmente los datos, no solo en el emisor.
- **Un interruptor de hombre muerto:** una alerta cuando un origen deja de enviar dentro de una ventana que fijes. Mira una vez al día, a las 09:15. También recibes una alerta cuando el repositorio no se puede leer en absoluto, diga lo que diga el calendario de comprobaciones, y cuando no ha llegado nada una ventana completa después de añadirlo. Un repositorio que añades antes de la primera copia del emisor está bien hasta entonces.
- **Alertas de integridad:** una alerta cuando falla una comprobación en el lado receptor. Una comprobación que no pudo abrir el repositorio no dice nada sobre la integridad, así que no deja ningún veredicto y lo vuelve a intentar al día siguiente a la misma hora.

Un repositorio en otro servidor necesita los datos de acceso de ese servidor: guárdalos como [credenciales](#sign-ins) y elígelos en **Credenciales** en el diálogo.

El receptor es estrictamente de solo lectura. Nunca escribe en el repositorio recibido, así que nunca puede romper la garantía append-only en la que confía el emisor.

Un recolector funciona al revés: con **Recogida** activada, una instancia lee las copias de otro miembro según su propio calendario, en un repositorio propio para cada origen, de modo que nunca cuentan como copias propias de este equipo. Sus orígenes también están en los detalles de ese miembro: la ubicación del repositorio, qué tipo de copia contiene y cada cuánto recogerla. La contraseña de restic llega por el grupo, nunca la APP_KEY, y el otro lado no tiene nada más que configurar y no tiene que estar en marcha. El origen solo se lee, y cada lado conserva sus propias credenciales. Una recogida se ejecuta el día y a la hora que dice su calendario, y un origen nuevo también espera a ese momento. **Recoger ahora** no espera. Una recogida perdida porque este equipo estaba apagado se ejecuta en cuanto vuelve. Una que falló lo vuelve a intentar al día siguiente a la misma hora, también con un calendario semanal, y una recogida fallida aparece en el registro de actividad y envía una sola alerta hasta que una recogida vuelva a funcionar. Un origen `rclone:` se rechaza, porque rclone llegaría a él con los remotos de esta instancia.

Una copia recogida solo crece. BombVault le añade datos y ni la poda ni la comprueba, así que lo que el origen olvidó hace tiempo se queda aquí hasta que borres la carpeta, y la página Destinos no la cuenta. Está en `pulled/<source id>` dentro de la carpeta propia de la sección, y cuando esa carpeta se mueve, la copia se queda donde está y las recogidas posteriores siguen allí en lugar de copiarlo todo de nuevo. Si la borras, la siguiente recogida empieza de nuevo en la carpeta propia que tenga la sección para entonces.

### Servidor receptor {#receiving-server}

El equipo receptor también puede ejecutar el rest-server al que copian los demás. Se configura la primera vez que permites una solicitud de **Receptor** que lo pide: la ventana dice que el servidor necesita un servidor receptor y pide una carpeta en un recurso compartido, con **Nueva carpeta** para crear una, y un puerto (8000 salvo que lo use otro contenedor). BombVault entonces:

1. se niega si ya existe un contenedor llamado `rest-server` o si otro contenedor tiene el puerto,
2. descarga `restic/rest-server` y lo inicia a través del socket de Docker en modo append-only con repositorios privados y un archivo de acceso en la carpeta,
3. escribe su plantilla de Unraid en la unidad flash, para que el contenedor siga siendo editable en la pestaña Docker, u ofrece la plantilla como descarga cuando la unidad flash está fuera de alcance,
4. ejecuta la prueba de manipulación contra él y muestra si rechaza los borrados.

Las instancias de tu grupo encuentran entonces el servidor entre los proveedores en **De tu grupo**, con el nombre del equipo receptor, y el rol **Receptor** lo ofrece como la opción segura. Cada instancia recibe un acceso propio la primera vez que usa el servidor, y escribe solo en su propia carpeta allí. Con **Receptor** activado en **Ajustes, Instancias**, **Servidor receptor** en la tarjeta de esta instancia vuelve a abrir el servidor. Su ventana lista los accesos en **Socios con acceso**, y **Revocar acceso** quita uno. Lo que esa instancia ya copió se queda en la carpeta. La configuración también crea un **Acceso fuera del grupo**, cuya contraseña la ventana muestra una vez.

Una instancia que llega al equipo receptor solo a través del repetidor no puede usar el servidor, porque el repetidor no transporta copias. Añade antes la dirección del equipo receptor en la página de emparejamiento. Cuando BombVault se ejecuta en una dirección IP propia (en br0, por ejemplo), rellena **Dirección para socios**, porque el servidor escucha en la dirección del host.

## Ejemplo: emparejar dos equipos Unraid {#worked-example-two-unraid-boxes-end-to-end}

Esta es una configuración completa de las piezas de arriba, porque es más fácil montarlas cuando las has visto montadas.

Dos equipos: **TOWER** ejecuta los contenedores y envía las copias. **VAULT** las recibe y aplica append-only. Sustituye por tus propios nombres, direcciones y rutas de recursos compartidos.

**1. Empareja los dos equipos.** En TOWER abre **Ajustes, Emparejamiento**, pulsa **Generar frase** e introduce las doce palabras en VAULT en **Introducir frase**. Antes define una contraseña de acceso en ambos.

**2. En TOWER, pide a VAULT que sea el receptor.** En la página Instancias, activa **Receptor** en la tarjeta de VAULT, elige las secciones y **el servidor receptor de VAULT** como lugar, y envía la solicitud.

**3. En VAULT, permítelo.** La página Instancias de VAULT muestra la solicitud en la tarjeta de TOWER. Pulsa **Permitir**. La primera vez, VAULT pide una carpeta para su servidor receptor, como `user/appdata/rest-server`, inicia el rest-server en modo append-only y ejecuta contra él la prueba de manipulación.

**4. De vuelta en TOWER.** A partir de entonces TOWER copia a VAULT después de cada copia, y el destino nuevo aparece en la página Destinos de TOWER con su protección contra borrado.

Qué responde la prueba de manipulación:

| Resultado | Qué pasó |
| --- | --- |
| **protegido** | VAULT rechazó el borrado. Es el único estado que pasa. |
| **no protegido** | VAULT aceptó un borrado. Falta `--append-only` o se quitó. |
| **no concluyente** | Ninguna de las dos. Normalmente la URL no es la que usa el propio restic, o cambiaron las credenciales. No se registra nada y no salta ninguna alerta. |

**5. Mira lo que llega, si quieres.** Activa **Receptor** en **Ajustes, Instancias** en VAULT, abre los **Detalles** de TOWER y añade el repositorio recibido: haz clic en una de las ubicaciones que lista el diálogo. VAULT recibe la contraseña de restic de TOWER por el grupo, y como la ubicación está en el propio servidor receptor de VAULT, lee la carpeta directamente. Nadie escribe una clave ni unos datos de acceso.

**6. Hazlo mutuo.** Repite los pasos en la otra dirección: VAULT pide a TOWER que sea su receptor. Cada equipo aplica entonces append-only para el otro, y ninguno puede borrar las copias del otro.

Un rest-server que ejecutas a mano también sirve: añádelo con **Añadir destino** como rest-server, con el usuario y la contraseña de su `.htpasswd`, y activa la protección contra borrado. Mantén `--append-only` en sus opciones. Sin eso el otro lado vuelve a ser un recurso compartido normal.

!!! warning "Una ruta local es una ruta dentro del contenedor"
    Donde BombVault pide una carpeta de este servidor, introdúcela relativa al montaje del host, como `user/appdata/rest-server/bombvault-containers`, no `/mnt/user/appdata/...`. BombVault se ejecuta en un contenedor, donde el `/mnt` del host está montado en otro sitio. Una ruta absoluta del host no existe dentro. Si pegas una, BombVault te dice la ruta relativa que debes usar.

## Asistente de emergencia {#emergency}

El **Asistente de emergencia** recupera un servidor que ha desaparecido o se volvió a configurar. Empieza en la página **Restauración**, en **Emergencia**, y desde **Restaurar desde una copia** en la página de bienvenida de una instalación nueva. Funciona en cuatro pasos: **Acceso**, **Ajustes**, **Selección** y **Restauración**.

1. **Acceso.** Muestra a BombVault la clave de las copias antiguas, de una de estas tres formas:
    - **Subir kit de recuperación**: el archivo que descargaste durante la configuración. Contiene la clave y todos los destinos, y BombVault rellena el resto a partir de él. Un kit sellado con age pide la identidad para la que se selló. Se usa una vez y no se guarda.
    - **A través del grupo**: una instancia de tu grupo guarda la clave como depósito. Necesitas las doce palabras del grupo, tu código de recuperación y a alguien en esa instancia. Consulta [Guardián de claves](#key-keeper).
    - **Escribir la clave**: la `APP_KEY` del servidor antiguo, y dónde están sus copias.

    BombVault prueba entonces la clave en cada destino y dice de cada uno si es legible, no se encuentra, está bloqueado con otra clave, no es alcanzable o aún le faltan las credenciales. Todavía no se ha guardado nada, y la clave se queda en memoria durante un tiempo limitado. **Adoptar la clave** hace que este servidor escriba sus copias con esa clave a partir de ahora. **Deshacer la adopción** vuelve a su propia clave hasta que se haya ejecutado la primera copia. Un servidor que ya tiene copias propias rechaza la adopción y remite a [De una carpeta de copias antigua](#restore-from-another-bombvault-repo). **Guardar credenciales** guarda las credenciales que lleva el kit para los destinos remotos, y pregunta antes de sustituir credenciales que este servidor ya tiene.
2. **Ajustes.** **Recuperar ajustes** restaura los ajustes propios de BombVault desde la autocopia más reciente: destinos, credenciales, calendarios y reglas. BombVault se reinicia para aplicarlos, a través del socket de Docker, de modo que la base de datos de ajustes en uso nunca se sobrescribe con un descriptor abierto, y el asistente continúa por sí solo después. El inicio de sesión de este servidor se conserva: sigues iniciando sesión con la contraseña que usas ahora. Los secretos que el servidor antiguo tenía guardados y que no se pueden adoptar se listan, para que puedas introducirlos de nuevo. La restauración termina todas las sesiones y elimina las claves de acceso, así que vuelve a configurarlas después.
3. **Selección.** BombVault examina tus copias y lista los contenedores, VMs, conjuntos de carpetas y conjuntos de datos ZFS que encuentra. La instantánea más reciente viene preseleccionada en todas partes. Lo que este servidor aún tiene empieza desactivado, porque una restauración lo reemplazaría. Los conjuntos de carpetas ya no saben de dónde venían tras una reconstrucción, así que van a una carpeta que eliges.
4. **Restauración.** Todo lo seleccionado vuelve de una vez, dejado detenido si quieres iniciar tú cada contenedor y VM, lo más seguro tras una pérdida total. Después descarga un kit de recuperación nuevo: el antiguo pertenece a la instalación anterior.

![El Asistente de emergencia en su primer paso, con las tres formas de entrar.](assets/screenshots/offsite/emergency-dark.png#only-dark)
![El Asistente de emergencia en su primer paso, con las tres formas de entrar.](assets/screenshots/offsite/emergency-light.png#only-light)

### Ejemplo: un servidor nuevo a partir del kit de recuperación {#example-emergency}

El servidor TOWER murió con sus discos. Tenía 23 contenedores y 2 VMs, y los copiaba, junto con su autocopia, a Backblaze B2. Tienes `bombvault-recovery-kit.md` en tu gestor de contraseñas y un equipo nuevo con Unraid. Su antigua `APP_KEY` se ha perdido, y no la necesitas.

1. **Instala BombVault** en el equipo nuevo y dale una `APP_KEY` nueva, como en cualquier instalación nueva. Configura el enlace de las VMs en **Ajustes, Integraciones, SSH del host** si quieres recuperar las VMs, porque una restauración de VM lo necesita.
2. **Inicia el asistente.** En **Te damos la bienvenida a BombVault**, elige **Restaurar desde una copia**. En la página Restauración, elige **Iniciar el asistente de emergencia** en **Emergencia**.
3. **Acceso.** Elige **Subir kit de recuperación** y suelta el archivo. BombVault prueba su clave en cada destino y dice, por ejemplo, **Kit de recuperación leído. Su clave abre 0 de 2 destinos.** La carpeta local dice **no encontrado**, ya que el disco murió, y Backblaze B2 dice **faltan credenciales**.
4. **Guardar credenciales.** El kit lleva las credenciales de cada destino. Pulsa **Guardar credenciales**, y los destinos se prueban de nuevo, así que Backblaze B2 dice ahora **legible**. Un kit escrito antes de la 10.5 solo contiene las credenciales predeterminadas. Donde un destino siga diciendo **faltan credenciales**, introduce sus credenciales en **Introducir unas credenciales que faltan** y pulsa **Probar los destinos de nuevo**.
5. **Adoptar la clave.** Este servidor escribe ahora sus copias con la clave antigua, de modo que las copias nuevas encajan con las antiguas. Conserva su propia `APP_KEY` en la plantilla. **Deshacer la adopción** funciona hasta que se haya ejecutado la primera copia.
6. **Ajustes.** El paso se abre en el lugar donde la clave encontró la autocopia, aquí B2, con su dirección ya rellenada. Elige **Recuperar ajustes**, que lee de ahí la autocopia más reciente. BombVault se reinicia para aplicarla, y el asistente continúa por sí solo. Tus destinos, calendarios y reglas han vuelto, y sigues iniciando sesión con la contraseña de este servidor. Introduce de nuevo los secretos que lista el asistente, como los ajustes de notificaciones.
7. **Selección.** **¿Dónde están las copias de seguridad?** se abre en las copias externas cuando no queda nada local. BombVault lee todos los destinos, así que uno caído no oculta a los demás, lista los 23 contenedores y las 2 VMs y restaura cada uno desde el destino que tiene su instantánea más reciente.
8. **Restauración.** Activa **Dejar detenido después**, lo más seguro tras una pérdida total, y empieza. **Qué pasa** dice lo que vuelve. La línea de resultado dice, por ejemplo, **25 restaurados, 0 fallidos. Inícialos en Copias de seguridad cuando haga falta.**
9. **Terminar.** Inicia primero tus bases de datos y luego las apps que las usan. Abre **Ajustes, Restauración** y descarga un kit de recuperación nuevo, porque el antiguo pertenece a la instalación anterior. Revisa **Necesita atención** en el resumen, y ejecuta **Copiar todo ahora** cuando todo funcione.

Si también falta la autocopia, el paso se abre en **Omitir: no tengo Autocopia** y lo indica. Los destinos salen entonces solo del kit, vuelves a configurar calendarios y reglas, y las copias externas esperan hasta que adoptes sus copias como propias de este servidor, consulta [Después de una reconstrucción](#after-a-rebuild). **Selección** sigue listando lo que contienen las copias externas. Los conjuntos de carpetas van a una carpeta que elijas. Los contenedores y las VMs vuelven como archivos sueltos en una carpeta en **Solo se pueden recuperar los archivos**, porque sus definiciones viven junto al repositorio local y en la autocopia, y una copia externa no contiene ninguna de las dos cosas.

!!! tip "Migración planificada frente a desastre"
    El Asistente de emergencia restaura los ajustes propios de BombVault desde una copia. Para una mudanza *planificada* a un equipo nuevo, puedes llevarte directamente tu configuración con la tarjeta **Exportar / importar ajustes** (un archivo JSON portátil). Consulta [Configuración](configuration.md#portable-settings-export-and-import).

### De una carpeta de copias antigua {#restore-from-another-bombvault-repo}

**De una carpeta de copias antigua**, en **Otras fuentes** en la página Restauración, abre el repositorio de una instancia de BombVault *distinta* (un recurso compartido montado bajo `/mnt`, o una dirección remota) con el kit de recuperación de esa instancia o su `APP_KEY` escrita, en una sesión única y de solo lectura. Examina los contenedores, VMs, conjuntos de carpetas y conjuntos de datos ZFS que hay guardados allí, elige una instantánea y restáurala, y el objeto restaurado pasa a ser un contenedor, VM o conjunto de carpetas local normal. Nunca se escribe nada en el otro repositorio, y tus propios ajustes de copia quedan intactos (la sesión vive en memoria y caduca sola). Mover un contenedor del servidor A al servidor B no significa reapuntar tus ajustes y revertirlos después. Esta ventana es de un solo uso: abre una sesión, restaura lo que eliges y olvida la otra instancia. Si quieres en cambio un acuerdo permanente, donde este equipo recoge las instantáneas de otra instancia en su propio repositorio según un calendario, ese es el rol de **Recolector** en la página Instancias.

### Lo que este servidor recogió {#restore-what-was-fetched}

Cada origen del que este servidor ha recogido tiene su propia línea **Obtenido de** en **Otras fuentes** en la página Restauración. Una copia recogida está cerrada con la propia clave de este servidor, no con la del origen, así que no se pide nada: la clave del origen no la abre. Los conjuntos de carpetas y los conjuntos de datos ZFS van a una carpeta que elijas. Los contenedores y las VMs vuelven como archivos sueltos en una carpeta, porque una recogida trae las instantáneas y no las definiciones. Para recrearlos, usa el kit de recuperación y la autocopia del origen en el Asistente de emergencia.

Un contenedor cuya red no existe en este servidor, como una red `br0` de Unraid en un host Docker normal, muestra un selector de red bajo su fila. BombVault lo crea en la red que elijas, junto con sus otras redes. La IP fija y la dirección MAC pertenecían a la red antigua y se descartan, de modo que la red nueva las asigna.

## Guardián de claves {#key-keeper}

Un guardián de claves es una instancia de tu grupo que guarda una copia sellada del kit de recuperación de este servidor. No puede abrir la copia por sí mismo. Si este servidor llega a perderse, una instalación nueva que se una a tu grupo pide la copia al guardián y la abre con tu código de recuperación.

![La ventana del código de recuperación: veinte caracteres en cuatro grupos, mostrados una vez.](assets/screenshots/offsite/recovery-code-dark.png#only-dark)
![La ventana del código de recuperación: veinte caracteres en cuatro grupos, mostrados una vez.](assets/screenshots/offsite/recovery-code-light.png#only-light)

1. En la página Instancias, activa **Guardián de claves** en la tarjeta de la instancia que debe guardarla, y elige **Depositar**. Hacer un depósito requiere una contraseña de acceso.
2. Este servidor muestra una vez el **Código de depósito**: veinte letras y dígitos en cuatro grupos de cinco. Anótalo y confirma con **He anotado el código**. Este servidor no guarda el código, solo lo que necesita para sellar un depósito para él.
3. El guardián tiene ahora el depósito, y este servidor lo renueva cuando cambia el kit. **Depósito de claves** en la tarjeta de esta instancia lista el **Depósito de este servidor** con cada guardián y su estado, y **Depósitos que guarda este servidor** para los demás. **Crear un código nuevo** sustituye el código. Cada guardián pasa al nuevo cuando recibe el depósito nuevo.

Ese día necesitas tres cosas: las doce palabras de tu grupo, el código de recuperación y acceso a la interfaz del guardián, donde alguien tiene que pulsar **Permitir**. En el Asistente de emergencia elige **A través del grupo**, elige el depósito, pídelo y escribe el código de recuperación. El código se usa en el servidor nuevo y no se envía al guardián. En el guardián, la solicitud aparece en **Depósito de claves** con quién la pide, si esa instancia se había visto allí antes y cuántas veces lo pidió. **Permitir** entrega la copia sellada, y permitir requiere una contraseña de acceso. La instalación nueva tiene 30 minutos para recogerla tras el Permitir, una solicitud que nadie responde caduca al cabo de un día, y una instancia que pide con demasiada frecuencia tiene que esperar una hora.

Sin el código nadie puede abrir la copia, ni siquiera el guardián. Si se pierde el código, crea uno nuevo mientras este servidor siga funcionando. Si el servidor ya ha desaparecido, el kit de recuperación y la APP_KEY escrita siguen funcionando. El guardián puede borrar un depósito en cualquier momento, y el propietario tiene que pedirlo de nuevo.

## Kit de recuperación {#recovery-kit}

Esta es la pieza que hace posible la recuperación ante desastres incluso cuando no hay ningún BombVault en marcha.

**Descargar kit de recuperación** en la tarjeta **Para emergencias** de **Ajustes, Restauración** te da la **clave**, la **contraseña de restic derivada**, cada **destino** con el repositorio de cada sección, las credenciales que necesita una instalación nueva para llegar a ellos y los **comandos exactos de restic**, para que puedas restaurar directamente con la CLI de restic en cualquier equipo. Requiere una contraseña de acceso. El kit imprime la clave también con el cifrado desactivado, porque las definiciones de contenedores y VMs siempre se sellan con ella. Tras una adopción imprime la clave de las copias adoptada y la APP_KEY de este servidor, que una instalación nueva necesita solo para recuperar los ajustes. Una tarjeta **Guarda tu kit de recuperación** en el resumen te lo recuerda hasta que lo has guardado.

!!! danger "Guarda el kit de recuperación fuera del servidor"
    El kit contiene el secreto que descifra tus copias. Guárdalo en un lugar seguro y separado del servidor (un gestor de contraseñas, una copia impresa en una caja fuerte). Si pierdes a la vez BombVault y `APP_KEY` sin kit de recuperación ni depósito de claves, tus copias cifradas no se pueden recuperar.

!!! warning "La instantánea más reciente no siempre es la que hay que restaurar"
    Desde restic 0.17, `restic snapshots` muestra el tamaño de cada instantánea. Tras una pérdida de datos la instantánea más reciente puede ser la vaciada, así que no restaures una instantánea que sea mucho más pequeña que las anteriores. Tras un ransomware puede ser la cifrada, con el tamaño habitual. Si BombVault aún funciona, mira antes su página **Anomalías**: nombra la última copia buena, y la ventana de restauración la preselecciona. Una restauración no necesita ninguno de los datos de anomalías de BombVault, y la pausa de retención solo conserva más instantáneas.

### El host de las copias de un servidor {#backup-host}

Cada servidor escribe sus copias bajo un host de restic propio: `bv-` y doce caracteres calculados a partir de la clave de las copias, como `bv-3f9a0c41d2e7`. Las copias de antes de que un servidor tuviera uno llevan el host `bombvault`. El kit nombra el host de este servidor, y sus comandos pasan ambos con `--host`, así que `latest` nunca elige una copia que otro servidor escribió en el mismo repositorio. Un servidor que adopta las copias de otro sigue bajo el host de ese servidor, porque el host sale de la clave.

### Sellar el kit {#sealing-the-kit}

Si has activado **Cifrar exportaciones con age** en **Ajustes, Sistema**, el kit también se sella con ello y se descarga como `bombvault-recovery-kit.md.age`. Está en armadura ASCII en lugar de binario, así que sigue siendo texto sin formato: pegarlo en un gestor de contraseñas o imprimirlo funciona exactamente como antes, el contenido es ilegible sin tu clave.

!!! warning "No guardes la clave age dentro del kit"
    Necesitas tu clave **privada** age para abrir un kit sellado. Guárdala en un sitio que no dependa del propio kit, o tendrás dos cosas que recuperar en lugar de una. Sellar merece la pena cuando el kit se guarda en un sitio que no controlas del todo (un gestor de contraseñas compartido, notas en la nube, una copia impresa en una oficina). Un kit en tu propia caja fuerte ya está protegido por la caja fuerte.

    Con el cifrado activado y ningún destinatario utilizable configurado, la descarga se rechaza de plano. BombVault nunca recurre a entregar la clave maestra en claro.

### Si no tienes el kit a mano {#if-you-do-not-have-the-kit-to-hand}

La contraseña no se guarda en ningún sitio, se **calcula** a partir de la clave de las copias, así que puedes reproducirla tú mismo con solo la clave y un shell:

```sh
printf 'bombvault:restic-repo'   | openssl dgst -sha256 -mac HMAC -macopt hexkey:$APP_KEY -r   | cut -d' ' -f1
```

Eso es HMAC-SHA256 sobre la cadena fija `bombvault:restic-repo`, con clave en los bytes en bruto de la `APP_KEY` hexadecimal, impreso como 64 caracteres hexadecimales en minúsculas. El mismo valor está en el kit, listado como la contraseña de restic derivada. Esto es para el día en que el kit esté donde tú no estás. En un servidor que adoptó las copias de otro, usa la clave de las copias adoptada, que es la antigua APP_KEY de ese servidor.

El [host](#backup-host) sale de la misma clave y la cadena `bombvault:host`: `bv-` y los doce primeros de esos caracteres.

```sh
echo "bv-$(printf 'bombvault:host' | openssl dgst -sha256 -mac HMAC -macopt hexkey:$APP_KEY -r | cut -c1-12)"
```

!!! warning "Para un repositorio recibido, usa la clave de la instancia EMISORA"
    Un repositorio que llegó aquí mediante una copia externa lo creó la máquina que lo envió, con **su** clave. Derivar de la clave del equipo receptor produce una contraseña que restic rechazará, lo que se lee exactamente como un repositorio corrupto y no lo es. Es la razón habitual por la que `restic check` en un repositorio recibido pide una y otra vez una contraseña.

Como las definiciones de recuperación viven **dentro** de cada repositorio (`<repo>/def`, `<repo>/vm-def`), una carpeta de repositorio copiada es totalmente autónoma, así que el kit más el repositorio es todo lo que necesita una restauración desde cero. Una copia externa y una copia recogida contienen las instantáneas sin las definiciones: devuelven archivos, y un contenedor o una VM se recrea a partir de la autocopia.

## Recuperar un volcado de base de datos {#database-dumps}

Un volcado de base de datos es un punto de restauración propio en el repositorio de contenedores, con la etiqueta `dbdump:<container>`, que contiene el único archivo `/dbdump/<container>.sql`. BombVault los lista, descarga e importa en la pestaña **Copias de seguridad** del contenedor. A continuación están los mismos pasos solo con restic, para el día en que BombVault no esté.

```sh
restic -r <repo> snapshots --host <host> --host bombvault --tag dbdump:<container>
restic -r <repo> dump --host <host> --host bombvault --tag dbdump:<container> latest /dbdump/<container>.sql > <container>.sql
```

`<host>` es el [host](#backup-host) del servidor que figura en el kit de recuperación.

Las etiquetas `dbversion:` y `dbname:` de cada volcado dicen de qué versión del servidor procede y qué bases de datos contiene. Un archivo completo termina con `-- PostgreSQL database cluster dump complete` o `-- Dump completed`.

Impórtalo en un contenedor de la misma versión o más nueva (PostgreSQL), o de la misma versión mayor (MySQL y MariaDB), iniciado una vez con una carpeta de datos vacía para que se inicialice. El host no necesita ningún cliente de base de datos, el contenedor tiene uno:

```sh
docker exec -i <container> sh -c 'exec psql -X -U "${POSTGRES_USER:-postgres}" -d postgres' < <container>.sql
docker exec -i <container> sh -c 'exec mariadb -uroot -p"$MARIADB_ROOT_PASSWORD"' < <container>.sql
docker exec -i <container> sh -c 'exec mysql -uroot -p"$MYSQL_ROOT_PASSWORD"' < <container>.sql
```

Para una sola base de datos de un volcado completo, MySQL y MariaDB aceptan `--one-database <name>` en el comando del cliente. Un volcado de PostgreSQL tiene una sección por base de datos, cada una empezando con una línea `\connect <name>`: copia esa sección a su propio archivo e impórtala con `-d <name>` después de crear la base de datos.

!!! warning "Un volcado de root lleva los usuarios del servidor"
    Un volcado completo de MySQL o MariaDB hecho como root contiene la base de datos de sistema `mysql`, así que importarlo reemplaza las cuentas del servidor nuevo, incluida la contraseña de root, por las del volcado. En PostgreSQL, `role ... already exists` para el usuario que creó el contenedor es lo esperado y es inofensivo.
