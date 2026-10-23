# Copia externa y recuperación

!!! note "Las copias externas esperan tras una reconstrucción"
    Cuando el Asistente de emergencia o **Descubrir copias** reconstruye elementos sin los ajustes antiguos, las copias externas de esas secciones se pausan hasta que se confirma su valor predeterminado. Consulta [Tras una reconstrucción](#after-a-rebuild).

Las copias locales te protegen de un contenedor perdido o de una mala actualización. Las copias externas y un kit de recuperación probado te protegen de la máquina entera, del ransomware o de un incendio. Esta página cubre los destinos y las copias externas, cómo hacer que una copia sea a prueba de manipulación, cómo demostrar que puedes restaurar, el emparejamiento de varios servidores y cómo recuperarlo todo cuando BombVault mismo ya no está.

## Copias externas {#off-site-copies}

Conserva la copia local rápida y añade uno o varios destinos fuera del local. BombVault copia allí las nuevas instantáneas con `restic copy` en modo de mejor esfuerzo, así que un contratiempo externo nunca hace fallar la copia local. En esta forma el repo local sigue siendo el primario y el repo externo es una copia, pero el repo primario de una sección no tiene por qué ser local; consulta más abajo [Repositorios primarios remotos](#remote-primary-repositories) para copiar directamente a S3, un rest-server y similares en lugar de copiar a ellos.

  - **Varios destinos por sección.** Cada sección (contenedores, VMs, flash, Autocopia, carpetas y conjuntos de datos ZFS) puede copiar a varios destinos externos a la vez, de modo que puedes mantener, por ejemplo, un rest-server en casa de un amigo y un bucket S3 en paralelo. La tarjeta **Destinos por sección** de la página Destinos activa o desactiva cada línea de una sección a un destino.
  - **Cuándo se copia.** La tarjeta **Copiar** de un destino fija **Cuándo copiar**: **Después de cada copia**, o según un plan propio que reúne las instantáneas y copia a horas fijas. En **Ajustes, Calendario, Copias externas por sección**, **Excepción** da a una sección su propio plan de copia. **Replicar ahora** en la tarjeta **Copia externa** de una sección, en **Ajustes, Destinos**, inicia una copia a demanda.
  - **La retención** se fija por destino en **Cuánto tiempo conservar**, así que puedes conservar las copias externas más tiempo como archivo. La copia en el campo externo de una sección sigue la tarjeta **Retención externa** de **Ajustes, Destinos**; déjala toda a cero para no recortar nunca automáticamente las instantáneas externas.
  - **Los límites de ancho de banda** (**Ancho de banda externo** en **Ajustes, Destinos**, o la tarjeta **Copiar** de un destino) limitan la velocidad de subida/bajada de restic para que las copias no saturen tu WAN.
  - Mientras se ejecuta una copia, la cabecera de estado del resumen dice **Copia externa en curso**. Es un indicador activo, no una barra de porcentaje, porque `restic copy` no expone un progreso legible por máquina.

!!! note "Restaurar desde cualquier lugar"
    Cada elemento lista sus copias como una sola línea de tiempo por todos los lugares donde hay una copia, en su pestaña **Copias de seguridad**. Una copia replicada en B2 aparece una vez, marcada con cada lugar que la guarda. Una restauración toma el primer lugar al que puede llegar, empezando por el repositorio donde se escribe el elemento, y puedes elegir otro lugar por copia. Los destinos externos se leen solo cuando pulsas **Comprobar**, porque listar el almacenamiento en la nube tarda. Borrar en un lugar comprueba antes los demás y dice si era la última copia.

## Destinos {#destinations}

Un destino es un lugar al que van las copias: una carpeta local, un recurso compartido, un servidor propio o un servicio en la nube. Se configura una vez y sirve a todas las secciones. Abre **Destinos** desde su mosaico de ajustes, o activa allí **Mostrar también en la barra lateral** para fijarlo.

**Añadir destino** abre una ventana en cuatro pasos:

1. **Proveedor.** Cada servicio aparece con su logotipo, con un buscador arriba a la derecha de la ventana, en grupos: servicios de almacenamiento con buckets S3 (Backblaze B2, Wasabi, Cloudflare R2, Hetzner Object Storage, Amazon S3 y otros), tu propio servidor S3 (Garage, SeaweedFS, RustFS, Silo, Ceph, JuiceFS, Versity S3 Gateway), tu propio servidor y recursos compartidos (rest-server, Hetzner Storage Box, SFTP, SMB, WebDAV, una ruta montada), almacenamiento en la nube (OneDrive, Google Drive, Dropbox, pCloud, Nextcloud y el resto que admite rclone) y los servidores receptores de tu grupo. Cada uno dice lo bien que sirve para copias: las unidades en la nube se ralentizan con muchas peticiones, así que la primera copia y la poda tardan más allí.
2. **Acceso.** Los campos dependen del servicio: una clave de acceso para S3, un usuario y una contraseña para WebDAV y SMB, una contraseña de aplicación donde la verificación en dos pasos bloquea la normal, la clave pública SSH de BombVault para SFTP y la Storage Box, o un token para servicios que inician sesión mediante un navegador. Para estos, la ventana muestra un comando `rclone authorize` que se ejecuta en un ordenador con navegador; el token que imprime va en el campo. **Probar** comprueba el acceso antes de guardar nada. Después la ventana lista las carpetas del destino, con **Nueva carpeta** para crear una y el espacio libre cuando el servicio lo informa. Una carpeta vacía es lo más seguro; BombVault pone en ella una carpeta por sección.
3. **Protección.** La ventana dice sin rodeos lo que puede hacer el servicio. Un rest-server en modo append-only rechaza el borrado, y la prueba de manipulación lo comprueba. Un bucket S3 puede conservar versiones antiguas con el versionado y el bloqueo de objetos, algo que BombVault todavía no puede comprobar. Una unidad en la nube no puede rechazar el borrado en absoluto: quien entra en el servidor entra también en esa copia. Activa **Protección contra borrado (append-only)** solo donde el otro extremo rechaza de verdad el borrado; BombVault no poda entonces nunca allí. **Para una emergencia** dice lo que guarda el kit de recuperación para este destino.
4. **Para qué secciones.** Elige las secciones que copian aquí. La primera copia trae todo el historial. Los elementos que eligieron sus propios destinos no reciben el nuevo por sí solos, y la ventana los nombra.

Algunos servicios piden un poco más al iniciar sesión:

- **SFTP y la Storage Box.** **Añadir clave** pone la clave pública de BombVault en el servidor con la **Contraseña de la cuenta**, que se usa solo para ese inicio de sesión y no se guarda en ningún sitio. Debajo aparece el comando para hacerlo a mano. La prueba muestra la **Clave del servidor** con su huella; BombVault la recuerda y rechaza el servidor si más adelante muestra otra.
- **Las claves S3 para un solo bucket**, como un token de objetos de Cloudflare R2 o una clave de AWS restringida, pueden no tener permiso para listar los demás buckets. Escribe el bucket en **Bucket**, y la prueba y la lista de carpetas abren ese bucket.
- **Los servidores SMB que no listan sus recursos compartidos** reciben el nombre de uno en **Recurso compartido**, y la prueba abre ese recurso directamente.
- **FTP** pide su **Cifrado**: **Ninguno**, **TLS explícito (FTPES)** o **TLS implícito (FTPS, puerto 990)**.
- **OneDrive y Zoho WorkDrive.** Cuando un inicio de sesión abre varias unidades o espacios de trabajo, la ventana pregunta a cuál van las copias.
- **Un rest-server con un certificado autofirmado.** Pon el certificado como archivo PEM en la carpeta `certs` de la carpeta de datos de BombVault (`/config/certs` en el contenedor, `appdata/bombvault/certs` en Unraid). Las copias de seguridad, las copias externas y la prueba de la protección contra borrado confían en él sin reiniciar.

Los servicios S3 pasan por el propio backend S3 de restic, que es lo que permite aplicar una clase de almacenamiento y el bloqueo de objetos. Todos los demás servicios pasan por el rclone que incluye BombVault, y su remoto aparece entonces en la tarjeta **Externo (rclone)** de **Ajustes, Destinos**. Una exportación de ajustes lleva los destinos; con las credenciales incluidas lleva también su acceso.

La carpeta de este servidor se llama **Local** en todas partes: en las páginas de destinos, en el flujo, en un elemento y en Restauración. Si este servidor tiene varias carpetas, cada una lleva su propio nombre. Un destino que crea el campo externo de una sección recibe el nombre de su lugar, la última carpeta de su dirección o, si no tiene ninguna, su servidor, y lleva un número cuando otro destino ya se llama así. Un nombre que le des en su página se conserva. Cuando dos destinos con el mismo nombre están uno junto al otro, cada uno lleva su servidor o carpeta.

Cada destino tiene una página propia:

- **Conexión** con su nombre, el interruptor **Disponible** (desactivado significa que BombVault ya no escribe aquí, y los elementos que copian aquí fallan con un mensaje claro en lugar de acabar en otro sitio), la carpeta, la compresión, la clase de almacenamiento y **Fuera del local**. **Cambiar conexión** prueba y guarda una dirección o un acceso nuevos.
- **Credenciales**: el conjunto compartido o uno propio.
- **Protección contra borrado**, con **Probar la protección contra borrado ahora** y quién poda esta copia.
- **Copiar**: cuándo copiar, y los límites de subida y bajada.
- **Cuánto tiempo conservar**: **Corto**, **Equilibrado**, **Largo** o **Valores propios**, con una línea de tiempo de las copias que conserva la regla. **Preguntar antes de conservar menos** pregunta antes de que una regla más corta borre copias cuya única copia está aquí.
- **Reglas por sección**: el repositorio, la retención o la compresión propios de una sección en este destino. **Cambiar reglas** las abre, y **Volver al destino** devuelve una sección a lo del destino.
- **Ya está bajo este destino**: secciones cuya dirección escrita a mano está en una carpeta de este destino. Adoptar una deja el repositorio donde está; el acceso, la clase de almacenamiento y la protección contra borrado pasan entonces a ser los del destino. BombVault comprueba antes que el acceso del destino abre el repositorio, y se niega a poner un repositorio append-only bajo un destino que no lo es.
- Un recipiente que muestra cuánto se ha llenado el destino, con la marca de cuando está lleno y cuántas semanas quedan al ritmo de crecimiento de las últimas semanas. Un destino que no informa de su tamaño dice **Tamaño desconocido**.

Un destino remoto que tenía copias y de repente no tiene ningún repositorio, como un rest-server cuyo disco no se montó, no recibe por sí solo uno nuevo y vacío. Su página lo indica en cuanto una copia de seguridad o una copia externa nota que falta el repositorio. Vuelve a montar el disco. Si vaciaste el repositorio antiguo a propósito, **Crear aquí un repositorio nuevo** empieza de cero, para todas las secciones del destino y sus repositorios con nombre a la vez.

Un servidor receptor que ejecuta otra instancia de tu grupo aparece entre los proveedores como **De tu grupo**; consulta [Servidor receptor](#receiving-server).

### Mover y eliminar {#move-remove}

- **Mover** lleva la copia de una sección a otro destino, con todo su historial. Todo se queda en el sitio antiguo; BombVault no borra nada allí. El traslado se ejecuta en segundo plano y puede tardar: unos 9 minutos por cada 100 GB entre discos locales, horas por un enlace lento. El resumen lo muestra mientras dura.
- **Eliminar destino** ofrece dos vías. **Eliminar destino y conservar las copias** deja de escribir allí y deja las copias en su sitio; puedes leerlas después en **Restauración, De una carpeta de copias antigua**. **Eliminar destino y borrar las copias de seguridad de allí** primero lee lo que hay guardado y lo lista: qué copias se van, cuáles de ellas no están en ningún otro sitio y qué repositorios se quedan porque la protección contra borrado lo rechaza, hay elementos que aún copian directamente a ellos, otro destino o sección guarda allí sus copias, o no se pudieron leer. Los elementos que perderían su única copia se listan aparte y se conservan salvo que actives **Borrar también estas**. Confirmas escribiendo el nombre del destino, y el borrado se ejecuta en segundo plano.
- Esto vale para cualquier tipo de destino: uno que usan varias secciones, un destino configurado solo para una sección, un repositorio que has creado tú y la carpeta en la que una sección escribe sus propias copias. Los elementos que copian a un repositorio eliminado vuelven a seguir a su sección, y lo mismo ocurre con un valor predeterminado de la sección que apuntaba allí. La carpeta de una sección no puede desaparecer sin más, porque toda sección necesita una. Por eso la ventana pregunta por la carpeta a la que se trasladan las secciones, y cada sección conserva debajo el nombre de su carpeta.
- Borrar las copias de un destino requiere una contraseña de inicio de sesión y haber iniciado sesión en la instancia. Sin contraseña el destino aún se puede eliminar, solo que sus copias no se pueden borrar desde aquí.

## Ubicación por elemento {#placement}

La tarjeta **Calendario y destino** de la pestaña Ajustes de un elemento tiene una fila de botones: **Local** y un botón por cada destino externo de la sección. Los botones encendidos reciben las copias del elemento.

- Con **Local** encendido, el elemento se escribe en el repositorio indicado en **Almacenado en** y se copia a cada otro destino encendido. Apaga un destino y no recibe nada nuevo de este elemento. Local solo no copia a ningún sitio, lo que conviene a datos que ya tienen una segunda copia, por ejemplo un recurso compartido que vive en un NAS.
- Con **Local** apagado, el elemento se escribe directamente en el repositorio directo del primer destino encendido y se copia desde allí a los demás encendidos. La primera vez, un diálogo crea ese repositorio directo.
- Un botón siempre queda encendido, porque una copia necesita un sitio al que ir. Para dejar un elemento fuera de las copias, usa **No copiar**.

BombVault nunca mueve copias entre repositorios. Un elemento que sigue a su sección se va con ella cuando cambia el valor predeterminado de la sección: su siguiente copia cae en el lugar nuevo, y las copias que ya tiene se quedan donde están. Una ubicación elegida en el propio elemento queda fija desde su primera copia; la tarjeta **Repositorio** de la pestaña Ajustes la nombra. Las copias pueden cambiar en cualquier momento. Un destino que deja de recibir un elemento conserva las copias que tiene y las recorta según su propia retención en la siguiente ejecución externa de la sección. En los destinos append-only no se puede borrar.

El lugar que un elemento dejó conserva sus copias. En la pestaña **Copias de seguridad** del elemento y en Restauración figura como **Solo copias anteriores**, una restauración aún puede leer de él, y una poda allí deja esas copias en paz.

**Seguir el valor por defecto** devuelve a su sección un elemento con elección propia. Pregunta antes, dice adónde van entonces las copias nuevas y nombra lo que cada destino deja de recibir y lo que subiría la siguiente ejecución.

Bajo la fila, la tarjeta dice adónde va el elemento y qué hay realmente: cuántas sedes lo guardan, cuándo se vio por última vez cada destino y si se cumple 3-2-1. Una sede es el servidor con los datos originales, cada destino externo y cada repositorio marcado **Fuera del local**. BombVault comprueba copias y sedes; no comprueba la parte de los "dos soportes" del 3-2-1.

### Valores predeterminados por sección {#placement-defaults}

La tarjeta **Destinos por sección** de la página Destinos dibuja el valor predeterminado de cada sección como un flujo 3-2-1. Cada línea de una sección a un destino es un interruptor, **Local** incluido, para cada sección, también para la unidad flash, los conjuntos de datos ZFS y la autocopia. Un clic en una línea o en un punto la cambia al instante, sin preguntar; las líneas discontinuas están desactivadas. **Gráfico / Interruptores** muestra lo mismo como filas de botones. Un interruptor decide adónde van las copias nuevas. Los elementos sin elección propia y las carpetas de proyecto de los stacks Compose lo siguen al instante, así que su siguiente copia va a los lugares nuevos. Las copias ya guardadas se quedan donde están y se pueden seguir restaurando desde allí. Flash, los conjuntos de datos ZFS y la Autocopia copian en su carpeta local y desde esa copia hacen la copia externa. Su línea hacia **Local** también se puede desactivar; sus copias van entonces directamente a sus destinos externos, y un elemento ZFS con réplica y sin ningún destino solo envía su réplica.

Con **Local** desactivado y solo destinos externos activados, una sección copia directamente a ellos, y BombVault elige allí el repositorio por sí mismo. Los contenedores, las VMs y los conjuntos de carpetas van al repositorio directo del primer destino activado, descrito más abajo, y se copian desde allí a los demás. La unidad flash, los conjuntos de datos ZFS y la autocopia escriben cada copia en todos los destinos activados, y cada destino las conserva según su propia retención.

Una sección con todas las líneas desactivadas dice **No se respalda**. Sus elementos no reciben copias y cuentan como no protegidos.

Los elementos que eligieron sus propios destinos en su página los conservan. El flujo dice cuántos hay en una sección, por ejemplo **2 elementos difieren**, y lleva hasta ellos: a la pestaña Ajustes del elemento cuando hay uno, a una lista con el lugar de copia de cada uno cuando hay varios. Allí, **Seguir el valor por defecto** devuelve un elemento a su sección.

Un destino externo nuevo recibe cada elemento que no está fijado en Local. La ventana que lo añade dice cuántos elementos y, si se sabe, cuánto historial son.

### Repositorios directos

Un repositorio directo es donde los elementos copian directamente a un destino externo. BombVault lo coloca junto a la carpeta del destino, con `-direct` añadido al nombre, o, cuando el destino está en la raíz de su bucket, de su remoto o de un usuario de rest-server, en una carpeta dentro de él con el nombre de la sección, como `containers-direct`. En el flujo de la página Destinos se crea sin preguntar. Apagar Local en un solo elemento, de modo que un destino sin repositorio directo pase a ser su hogar, abre un diálogo con la ubicación sugerida, por ejemplo `s3:https://s3.eu-central-003.backblazeb2.com/bucket/containers-direct`, y una prueba de conexión que no crea nada. **Crear y usar** crea el repositorio y apunta el elemento a él. Un repositorio directo toma la clave, la clase de almacenamiento, los límites, el ajuste append-only y la retención del destino, y cambia con ellos; la tarjeta Repositorios lo muestra en solo lectura. Cuando una clave nueva del destino no puede abrirlo, el repositorio directo conserva la clave que tiene y el guardado lo dice. Un elemento en un repositorio directo se copia desde allí a los demás destinos encendidos, nunca al destino al que pertenece el repositorio. Sus instantáneas llevan la etiqueta `bv:direct`, y cualquier otra pasada de retención las conserva, de modo que un repositorio directo que perdió su vínculo con su destino nunca envejece por las reglas locales. B2 se alcanza a través de su punto de conexión S3, con el ID de clave y la clave de aplicación como credenciales S3; una clave limitada a la propia carpeta del destino no puede llegar a la carpeta vecina, así que limita la clave a la carpeta superior.

### Fuera del local

Un destino o un repositorio con nombre puede marcarse como **Fuera del local**. Los remotos empiezan marcados; desactívalo para un rest-server en el mismo edificio. La marca solo cuenta sedes y 3-2-1 en las tarjetas. No cambia ninguna copia.

Una réplica ZFS es una copia fuera del local, pero no una copia con historial. Solo conserva las instantáneas que fijas, y las versiones antiguas y los archivos sueltos siguen saliendo de las copias de seguridad. Va de un servidor ZFS directamente al otro, por SSH a un servidor ZFS o al punto de recepción de una instancia emparejada. El grupo de emparejamiento solo transporta la solicitud. Consulta [Réplica](zfs-datasets.md#replica).

### Tras una reconstrucción {#after-a-rebuild}

Las elecciones de copia viven en los ajustes propios de BombVault. Tras una reconstrucción con **Descubrir copias** o con el Asistente de emergencia sin los ajustes restaurados, han desaparecido, y copiarlo todo enviaría de nuevo a B2 los elementos que habías dejado fuera. Por eso las copias externas de cada sección reconstruida se pausan. El resumen lo indica, y la tarjeta **Destinos por sección** ofrece **Confirmar valor predeterminado** con una vista previa de lo que copiará la siguiente ejecución y de los nombres en las copias que no tienen elemento, que puedes dejar fuera allí. Solo la confirmación termina la pausa; importar un archivo de ajustes recupera reglas y valores predeterminados pero no la termina.

## Repositorios primarios remotos {#remote-primary-repositories}

La propia ruta de una sección no se limita a una carpeta local: apúntala directamente a un remoto de restic (`s3:...`, `rest:http://host:8000/repo`, `sftp:user@host:/repo`, `rclone:remote:bucket/path`) y BombVault copia directamente allí, sin copia local aparte ni paso de copia. Es una forma diferente de las copias externas anteriores: allí el repo local es el primario y el repo externo es un archivo de mejor esfuerzo; aquí el repo remoto **es** el primario, y es la única copia salvo que además copies esa sección a otro destino.

La ruta de cada sección (Contenedores, VMs, Flash, Autocopia, Carpetas, Conjuntos de datos ZFS) se fija en la página del destino **Local**: **Cambiar** junto a la sección en su tarjeta **Conexión** abre una ventana con un conmutador **Local / Remoto**:

- **Local** muestra el explorador de carpetas de siempre.
- **Remoto** lo sustituye por un campo de URL sencillo y los ajustes de conexión que usa un destino externo, configurados para este primario. Desde ahí obtienes:
    - **Una prueba de conexión** contra la ruta real, antes de confiar en ella.
    - **Límites de ancho de banda** (subida/bajada) para que una copia programada a un primario remoto no sature tu WAN: los mismos indicadores `--limit-upload`/`--limit-download` de restic que usan las copias externas, aplicados a la propia copia.
    - **Protección append-only**, verificada con la misma prueba de manipulación activa (un sondeo DELETE real contra el otro extremo) que reciben los destinos externos. Con ella activada, BombVault se niega a podar el propio repo, ya que no hay una copia local aparte detrás y las credenciales de esta máquina no deben poder borrar la única copia de la copia de seguridad.
    - **Una alarma de presupuesto de crecimiento**, tomada de la misma tendencia de tamaño del repositorio que la tarjeta Almacenamiento ya sigue.

Nada de esto es obligatorio: una ruta remota escrita a mano sin ajustes de seguridad guardados copia sin límites, se puede podar y no tiene alarma de presupuesto. Los ajustes de seguridad están para cuando quieres las mismas protecciones que recibe una copia externa, sin necesitar un destino aparte solo para obtenerlas.

!!! note "Las credenciales de nube y REST se comparten"
    Un primario remoto se autentica con las credenciales de nube compartidas que se configuran en **Ajustes, Destinos**; no hay un almacén de credenciales aparte para los repos primarios.

### SMB y WebDAV sin montaje en el host {#smb-webdav}

La tarjeta **Externo (rclone)** de **Ajustes, Destinos** tiene un formulario para un recurso compartido de Windows o Samba y para un servidor WebDAV (Nextcloud, ownCloud, SharePoint u otro). Rellena un nombre corto, el host y el recurso compartido (SMB) o la URL y el tipo de servidor (WebDAV), el usuario y la contraseña, y BombVault escribe la sección de rclone por ti. rclone ofusca la contraseña antes de guardarla; añadir uno con un nombre que ya existe reemplaza esa sección en lugar de añadir una segunda. **Añadir destino** ofrece SMB y WebDAV como proveedores con los mismos campos.

El formulario responde con la ubicación terminada, por ejemplo `rclone:nas:backups`. Úsala como ruta de una sección o como destino y añade una subcarpeta si quieres una (`rclone:nas:backups/bombvault`). El recurso compartido es el primer segmento de la ruta, no parte del nombre.

Es mejor vía que montar el recurso compartido en Unraid: restic desaconseja guardar un repositorio en un recurso CIFS montado, y aquí no se monta nada. NFS no está en el formulario porque ni restic ni rclone tienen un backend NFS; para NFS, monta la exportación en el host y úsala como carpeta local.

## Protección contra borrado (append-only)

Activa la protección contra borrado en un destino externo para que el ransomware, o un host comprometido, no pueda borrar ni reescribir tus copias. El otro extremo (un `restic/rest-server` en modo `--append-only`) la **hace cumplir**. BombVault solo la **verifica** y nunca muestra verde por una mera declaración de configuración. Una carpeta de este servidor no puede negarse a borrar, porque quien controla BombVault controla también la carpeta, así que los destinos locales no ofrecen el interruptor.

El interruptor es del destino: una sección que copia allí lo toma del destino, así que actívalo en la página del destino, en **Destinos**. La comprobación programada **Comprobar la protección contra borrado** solo se ejecuta mientras una sección copia a un destino con el interruptor activado. El otro lado tiene que arrancar con `--append-only`; con la imagen `restic/rest-server`, en TrueNAS como en cualquier otro host Docker, eso va en la variable `OPTIONS`.

La vía más rápida a una copia append-only es un BombVault emparejado con un [servidor receptor](#receiving-server): ejecuta el rest-server por ti, en modo append-only.

!!! note "Un borrado correcto en `/locks/` es lo esperado"
    Append-only no significa que ya no se pueda borrar nada. restic tiene que tomar y liberar sus propios bloqueos, así que `/locks/` sigue siendo escribible y borrable a propósito. Las instantáneas y los datos que hay detrás, que es justo lo que buscaría un ransomware, no se pueden eliminar. Si compruebas tú mismo el lado remoto, un borrado que funcione bajo `/locks/` es el comportamiento correcto y no un agujero.

!!! warning "Los repos append-only nunca se podan desde esta máquina"
    Un destino append-only nunca poda instantáneas antiguas desde aquí; podar es cosa del otro extremo. Define una **alarma de presupuesto de crecimiento** para que te avisen antes de que el tamaño del repo se desboque.

## Prueba de manipulación

BombVault demuestra periódicamente la garantía append-only intentando realmente un borrado contra el repo externo, dirigido a un objeto inexistente. **Probar la protección contra borrado ahora** en la página de un destino la ejecuta a demanda, y **Comprobar la protección contra borrado** en **Ajustes, Verificación, Comprobaciones automáticas** la ejecuta según un calendario.

Una copia desactivada no se prueba, y el destino lo indica. La prueba de conexión sigue funcionando con ella, porque solo lee, así que una copia se puede comprobar antes de activarla.

- **Rechazado** significa protegido.
- **Aceptado** significa no protegido.
- Un resultado **no concluyente** (servidor inalcanzable, error de autenticación) nunca cambia el veredicto almacenado.

Un cambio real de protegido a no protegido dispara una única alerta.

## Ensayos de DR

BombVault ofrece dos niveles de prueba de que tus copias son realmente restaurables, no solo de que están presentes.

- **Ensayos de verificación de restauración (locales).** BombVault ejecuta periódicamente `restic check --read-data-subset` (acotado, nunca una restauración completa que llene el disco). La cadencia está en **Ajustes, Verificación, Comprobaciones automáticas**, y la tarjeta **Comprobar las copias** del mismo mosaico muestra el resultado.
- **Restauración de prueba (externa).** BombVault restaura un objetivo real desde el repo externo en un entorno de pruebas aislado, lo verifica archivo por archivo y byte por byte, y luego limpia. Esto demuestra que puedes recuperarte desde el externo, no solo que el repo responde.

La tarjeta **Estado de protección** del resumen lo resume por sección en cuatro comprobaciones: una copia actual en este servidor, una copia externa, la protección contra borrado allí y una restauración de prueba desde allí. Un clic en una sección muestra los detalles, y la tarjeta solo se pone en verde con hechos verificados.

## Emparejamiento de instancias {#pairing}

Varios BombVault trabajan juntos como un grupo: se muestran el estado de protección unos a otros, asumen roles unos para otros y pueden guardarse la clave unos a otros. Una instancia se une al grupo con doce palabras.

En la primera instancia abre **Ajustes, Emparejamiento** y pulsa **Generar frase**. Tres tarjetas de pasos muestran cómo va. Doce palabras aparecen en una ventana con un botón **Copiar**. En cada una de las demás instancias abre el mismo sitio, pulsa **Introducir frase** y pégalas o escríbelas, o pulsa **Pegar** en esa ventana. Una palabra que no está en la lista se nombra con su posición mientras escribes, y la última palabra lleva una suma de comprobación, así que una palabra mal escrita o cambiada de sitio se detecta antes de emparejar nada. Genera la frase en una sola instancia: dos instancias que generen una forman dos grupos separados. Si nadie aparece en un minuto, la página ofrece dos salidas: mostrar de nuevo las palabras para introducirlas allí, o introducir las palabras de la otra instancia y unirse a su grupo en un solo paso. El emparejamiento funciona sin contraseña de inicio de sesión, pero define una: sin ella cualquiera que pueda abrir la interfaz web puede leer las palabras y, a través del grupo, pedir roles que entregan la contraseña restic de las instancias que hay en él. La tarjeta de emparejamiento lo dice hasta que se define una contraseña. Con contraseña, mostrar de nuevo la frase la pide. **Salir del grupo** saca de nuevo a una instancia. La tarjeta **Instancias** de la página de emparejamiento lista los miembros del grupo; la imagen de quién hace qué para quién está en la página Instancias.

Cualquiera que conozca las palabras puede unirse al grupo, así que trátalas como una contraseña.

**Cómo se alcanzan los miembros.** Cada instancia aprende su propia dirección en la red desde tu navegador en cuanto inicias sesión, listada en **Direcciones conocidas**, en la tarjeta **Esta instancia** de la página de emparejamiento, junto con las que la instancia encontró por sí misma; corrígela allí si un proxy inverso o un puerto poco habitual está delante. En la misma red los miembros anuncian esa dirección por multidifusión y hablan directamente, y donde la multidifusión no puede cruzar una red de contenedores, como el bridge por defecto de Docker, una instancia busca en cambio en su propia subred a las demás con una llamada firmada que solo un miembro del grupo puede responder, de modo que el emparejamiento termina en segundos sin relay. Si no aparece nada, **¿No la encuentras?** admite una dirección a mano, para otra subred o un puerto no estándar. Las instancias en redes distintas pasan por un relay, elegido en una fila de cuatro en la tarjeta **Repetidor** de la misma página; cada forma trae una imagen:

- **Repetidor del proyecto** (el predeterminado): `parleyport.halleluja.design`, el mismo repetidor que usa también KnightLoader. Nada que configurar.
- **Repetidor propio**: el contenedor [**ParleyPort**](https://github.com/junkerderprovinz/parleyport) de las Community Apps de Unraid (también en **Ajustes, Apps**). Introduce su dirección en cada instancia que deba usarlo.
- **Instancia de BombVault**: una de tus instancias que ya sea accesible desde fuera, con **Actuar como repetidor** activado. Esa instancia responde entonces en `/relay/connect` en su propia dirección, tras el proxy inverso y el certificado que ya tiene, y deja entrar solo a tu grupo. Introduce su dirección en cada instancia que deba usarla.
- **Sin repetidor**: los miembros se encuentran automáticamente solo en la misma red, y en ningún otro sitio.

**Qué ve el repetidor.** Un botón del mismo nombre junto a las cuatro formas lo muestra para la que hayas elegido. Cada llamada entre miembros va sellada con AES-256-GCM bajo una clave derivada de las doce palabras, y esa clave nunca sale de tus instancias. El relay conoce un hash que agrupa las conexiones, para qué instancia es un mensaje, su tamaño y cuándo pasa. Una llamada directa en la red local se sella igual y además se firma, de modo que nada depende del certificado autofirmado que sirve una instancia. Las llamadas que entregan algo (respuestas de rol, una contraseña restic, accesos de receptor, depósitos de claves) también se firman con una clave del miembro que las envía, fijada al verla por primera vez; consulta [Modelo de seguridad](configuration.md#security-model).

**Qué viaja por el grupo.** El estado de protección de la página Instancias, una petición de comprobar una sección ahora, las solicitudes de rol y sus respuestas, las ofertas de destino y lo que necesita un receptor o un recolector una vez permitido su rol: las ubicaciones de repositorio de la otra instancia y su contraseña restic. Los datos de copia nunca; siguen yendo directos a los backends de restic. La APP_KEY tampoco: la contraseña restic abre los repositorios de esa instancia y nada más, ni sus secretos guardados, sesiones o códigos de recuperación.

**Elementos anteriores al emparejamiento.** Las instancias añadidas con un token de flota, y los repositorios recibidos y las fuentes de recogida configurados con la APP_KEY de la otra instancia, se quedan tras la actualización y se marcan **Emparejar de nuevo**. Siguen funcionando: en su primer arranque BombVault sustituye cada APP_KEY guardada por la contraseña restic derivada de ella. Empareja las dos instancias, edita luego el elemento y elige su instancia. Hasta entonces esos elementos están en la tarjeta **Sin instancia** de la página Instancias.

El único sitio que aún acepta una APP_KEY a mano es [De una carpeta de copias antigua](#restore-from-another-bombvault-repo), para el caso en que la otra instancia ya no existe y no puede responder en un grupo.

## Instancias y roles {#roles}

La página **Instancias** (un mosaico de ajustes que se puede fijar en la barra lateral) muestra el grupo como una red de quién hace qué para quién, con una tarjeta para esta instancia y una para cada otro miembro, los móviles Android y los servidores ZFS. **Emparejamiento** abajo a la derecha lleva a la página de emparejamiento. Qué partes están activas se fija en **Ajustes, Instancias, Varios servidores**: **Instancias**, **Receptor** y **Recogida**.

Cada tarjeta muestra el estado de protección del miembro y sus roles como mosaicos. Un rol es algo que una instancia hace para otra:

- **Receptor** guarda las copias externas de este servidor en un rest-server en modo append-only. Este servidor envía tras cada copia y no puede borrar nada allí, ni aunque alguien lo tome.
- **Recolector** recoge las copias de este servidor según su propio calendario. Este servidor no tiene que estar despierto para ello.
- **Servidor ZFS** acepta las réplicas ZFS de este servidor. Cada elemento ZFS lo elige en **Réplica** en sus ajustes.
- **Guardián de claves** guarda una copia sellada del kit de recuperación de este servidor. Consulta [Guardián de claves](#key-keeper).

Activar un rol envía una solicitud, y no empieza nada hasta que alguien en la otra instancia pulsa **Permitir**. La solicitud aparece en la tarjeta de esa instancia como una insignia, y sus **Detalles** la listan con **Permitir** y **Rechazar**; un rol permitido se puede revocar allí en cualquier momento. Una línea discontinua en la red es un rol que aún espera. Una instancia con BombVault 9.x no puede responder a una solicitud, así que un rol está activo allí en cuanto lo activas, y una solicitud de una instancia 9.x espera **Permitir** aquí.

**Detalles** en una tarjeta abre el resto: los repositorios recibidos y las fuentes de recogida de ese miembro, las ofertas de destino, la supervisión, **Eliminar** y **Olvidar la clave**.

Las instancias apagadas y los móviles con la app cerrada conservan su tarjeta como **Inactiva**, en gris, con la última vez que se vieron, y aparecen atenuados en la red hasta que los quites con **Eliminar**. Justo después de arrancar este servidor, una tarjeta así muestra **Comprobando** hasta que el grupo vuelve. **Eliminar** funciona en una tarjeta así sean cuales sean las partes activadas, y además termina las solicitudes abiertas del miembro, sus plazas de recepción ZFS sin usar y sus solicitudes de clave pendientes. La tarjeta **Instancias** de la página de emparejamiento también muestra estos miembros como **Inactiva**.

## Receptor y recolector {#receiver}

![Una copia externa recibida en los detalles de una instancia, vigilada en solo lectura, con una comprobación de integridad ejecutada en este hardware.](assets/screenshots/receiver.png)

*Una copia externa recibida en los detalles de una instancia, vigilada en solo lectura, con una comprobación de integridad ejecutada en este hardware.*

En la máquina que **recibe** copias append-only de otro BombVault, los repositorios recibidos te dan supervisión independiente y de solo lectura en el hardware receptor, de modo que un fallo silencioso en el otro extremo no pase inadvertido. Activa **Receptor** en **Ajustes, Instancias**, abre luego los **Detalles** de la instancia emisora en la página Instancias y añade el repositorio recibido (solo lectura, abierto con la contraseña restic de la instancia emisora, que recibe a través del [grupo de emparejamiento](#pairing)). Obtienes:

- **Un inventario agrupado por fuente**, para ver exactamente qué contenedores, VMs y conjuntos de carpetas han llegado.
- **Última recepción** por fuente, para saber lo fresca que está cada una.
- **Un `restic check` independiente** ejecutado en el hardware receptor, de modo que la integridad se verifica donde los datos realmente residen, no solo en el emisor.
- **Un interruptor de hombre muerto:** una alerta cuando una fuente deja de enviar dentro de una ventana que tú defines.
- **Alertas de integridad:** una alerta cuando una comprobación del lado receptor falla.

El receptor es estrictamente de solo lectura. Nunca escribe en el repositorio recibido, así que nunca puede romper la garantía append-only en la que confía el emisor.

Un recolector funciona al revés: con **Recogida** activada, una instancia lee las copias de otro miembro según su propio calendario, en un repositorio propio para cada fuente, de modo que nunca cuentan como copias de esta máquina. Sus fuentes también están en los detalles de ese miembro.

### Servidor receptor {#receiving-server}

La máquina receptora también puede ejecutar el rest-server al que copian las demás. Se configura la primera vez que permites una solicitud de **Receptor** que lo pide: la ventana dice que el servidor necesita un servidor receptor y pide una carpeta en un recurso compartido, con **Nueva carpeta** para crear una, y un puerto (8000 salvo que otro contenedor lo use). BombVault entonces:

1. se niega a seguir si ya existe un contenedor llamado `rest-server` o si otro contenedor ocupa el puerto;
2. descarga `restic/rest-server` y lo inicia a través del socket de Docker en modo append-only, con repositorios privados y un archivo de inicios de sesión en la carpeta;
3. escribe su plantilla de Unraid en la unidad flash, de modo que el contenedor siga siendo editable en la pestaña Docker, u ofrece la plantilla como descarga cuando la unidad flash no está al alcance;
4. ejecuta la prueba de manipulación contra él y muestra si rechaza los borrados.

Las instancias de tu grupo encuentran entonces el servidor entre los proveedores en **De tu grupo**, con el nombre de la máquina receptora, y el rol **Receptor** lo ofrece como la opción segura. Cada instancia recibe un acceso propio la primera vez que usa el servidor, y escribe solo en su propia carpeta allí. Con **Receptor** activado en **Ajustes, Instancias**, la tarjeta **Servidor receptor** de esta instancia vuelve a abrir el servidor. Su ventana lista los accesos en **Socios con acceso**, y **Revocar acceso** quita uno; lo que esa instancia ya copió se queda en la carpeta. La configuración también crea un **Acceso fuera del grupo**, cuya contraseña la ventana muestra una sola vez.

Una instancia que llega a la máquina receptora solo a través del relay no puede usar el servidor, porque el relay no transporta copias. Añade antes la dirección de la máquina receptora en la página de emparejamiento. Cuando BombVault se ejecuta en una IP propia (en br0, por ejemplo), rellena **Dirección para socios**, porque el servidor escucha en la dirección del host.

## Ejemplo completo: dos equipos Unraid, de principio a fin

Todo lo anterior describe las piezas. Esta es una configuración completa, porque las piezas son más fáciles de montar cuando las has visto montadas una vez.

Dos máquinas: **TOWER** ejecuta los contenedores y envía las copias; **VAULT** las recibe y hace cumplir append-only. Sustituye por tus propios nombres, direcciones y rutas de recursos compartidos.

**1. Empareja las dos máquinas.** En TOWER abre **Ajustes, Emparejamiento**, pulsa **Generar frase** e introduce las doce palabras en VAULT con **Introducir frase**. Define antes una contraseña de inicio de sesión en ambas.

**2. En TOWER, pide a VAULT que sea el receptor.** En la página Instancias, activa **Receptor** en la tarjeta de VAULT, elige las secciones y **el servidor receptor de VAULT** como lugar, y envía la solicitud.

**3. En VAULT, permítelo.** La página Instancias de VAULT muestra la solicitud en la tarjeta de TOWER. Pulsa **Permitir**. La primera vez, VAULT pide una carpeta para su servidor receptor, como `user/appdata/rest-server`, inicia el rest-server en modo append-only y ejecuta contra él la prueba de manipulación.

**4. De vuelta en TOWER.** A partir de entonces TOWER copia a VAULT tras cada copia, y el nuevo destino aparece en la página Destinos de TOWER con su protección contra borrado.

Lo que responde la prueba de manipulación:

| Resultado | Qué ocurrió |
| --- | --- |
| **protegido** | VAULT rechazó el borrado. Es el único estado que aprueba. |
| **no protegido** | VAULT aceptó un borrado. Falta `--append-only` o se quitó. |
| **no concluyente** | Ninguna de las dos. Normalmente la URL no es la que usa restic, o las credenciales han cambiado. No se registra nada ni se dispara ninguna alerta. |

**5. Observa lo que llega, si quieres.** Activa **Receptor** en **Ajustes, Instancias** en VAULT, abre los **Detalles** de TOWER y añade el repositorio recibido. VAULT recibe la contraseña restic de TOWER a través del grupo; nadie escribe una clave.

**6. Hazlo mutuo.** Repite los pasos en el otro sentido: VAULT pide a TOWER que sea su receptor. Cada máquina hace entonces cumplir append-only para la otra, y ninguna puede borrar las copias de la otra.

También sirve un rest-server que ejecutes a mano: añádelo con **Añadir destino** como rest-server, con el usuario y la contraseña de su `.htpasswd`, y activa la protección contra borrado. Mantén `--append-only` en sus opciones; sin él el otro extremo vuelve a ser un recurso compartido corriente.

!!! warning "Una ruta local es una ruta dentro del contenedor"
    Donde BombVault pide una carpeta de este servidor, introdúcela relativa al montaje del host, como `user/appdata/rest-server/bombvault-containers`, no `/mnt/user/appdata/...`. BombVault se ejecuta en un contenedor, donde el `/mnt` del host está montado en otro sitio; una ruta absoluta del host no existe dentro de él. Si pegas una, BombVault te dice la ruta relativa que debes usar en su lugar.

## Asistente de emergencia {#emergency}

El **Asistente de emergencia** recupera un servidor que ya no existe o que se ha vuelto a instalar. Empieza en la página **Restauración**, en **Emergencia**, y desde **Restaurar desde una copia** en la página de bienvenida de una instalación nueva. Se ejecuta en cuatro pasos: **Acceso**, **Ajustes**, **Selección** y **Restauración**.

1. **Acceso.** Muestra a BombVault la clave de las copias antiguas, de una de tres maneras:
    - **Subir kit de recuperación**: el archivo que descargaste durante la configuración. Contiene la clave y todos los destinos, y BombVault rellena el resto a partir de él. Un kit sellado con age pide la identidad para la que se selló; se usa una vez y no se guarda.
    - **A través del grupo**: una instancia de tu grupo guarda la clave como depósito. Necesitas las doce palabras del grupo, tu código de depósito y a alguien en esa instancia. Consulta [Guardián de claves](#key-keeper).
    - **Escribir la clave**: la `APP_KEY` del servidor antiguo, y dónde están sus copias.

    BombVault prueba entonces la clave en cada destino y dice de cada uno si es legible, no se encuentra, está bloqueado con otra clave, no es alcanzable o aún le falta el acceso. Todavía no se ha guardado nada, y la clave permanece en memoria durante un tiempo limitado. **Adoptar la clave** hace que este servidor escriba sus copias con esa clave a partir de ahora; **Deshacer la adopción** vuelve a su propia clave hasta que se ha ejecutado la primera copia. Un servidor que ya tiene copias propias rechaza la adopción y remite a [De una carpeta de copias antigua](#restore-from-another-bombvault-repo). **Guardar credenciales** guarda los accesos que lleva el kit para destinos remotos, y pregunta antes de reemplazar accesos que este servidor ya tiene.
2. **Ajustes.** **Recuperar ajustes** restaura los ajustes propios de BombVault desde la Autocopia más reciente: destinos, credenciales, calendarios y reglas. BombVault se reinicia para aplicarlos, a través del socket de Docker, de modo que la base de datos de ajustes en uso nunca se sobrescribe con un manejador abierto, y el asistente continúa solo después. El inicio de sesión de este servidor se conserva: sigues entrando con la contraseña que usas ahora. Los secretos que tenía guardados el servidor antiguo y que no se pueden adoptar se listan, para que los introduzcas de nuevo. La restauración cierra todas las sesiones y elimina las claves de acceso, así que vuelve a configurarlas después.
3. **Selección.** BombVault revisa tus copias y lista los contenedores, VMs, conjuntos de carpetas y conjuntos de datos ZFS que encuentra. La copia más reciente viene preseleccionada en todas partes. Lo que este servidor aún tiene empieza desactivado, porque una restauración lo reemplazaría. Los conjuntos de carpetas ya no saben de dónde vinieron tras una reconstrucción, así que van a una carpeta que eliges.
4. **Restauración.** Todo lo seleccionado vuelve de una vez, dejado detenido si quieres iniciar tú mismo cada contenedor y VM, que es lo más seguro tras una pérdida total. Después descarga un kit de recuperación nuevo: el antiguo pertenece a la instalación anterior.

!!! tip "Migración planificada frente a desastre"
    El Asistente de emergencia restaura los ajustes propios de BombVault desde una copia. Para un traslado *planificado* a una máquina nueva, puedes llevar tu configuración directamente con la tarjeta **Exportar / importar ajustes** (un archivo JSON portátil). Consulta [Configuración](configuration.md#portable-settings-export-and-import).

### De una carpeta de copias antigua {#restore-from-another-bombvault-repo}

**De una carpeta de copias antigua** en **Otras fuentes** de la página Restauración abre el repositorio de una instancia de BombVault *distinta* (un recurso compartido montado bajo `/mnt`, o una dirección remota) con el kit de recuperación de esa instancia o con su `APP_KEY` escrita, en una sesión única de solo lectura. Explora los contenedores, VMs, conjuntos de carpetas y conjuntos de datos ZFS guardados allí, elige una instantánea y restáurala, y el objeto restaurado pasa a ser un contenedor, una VM o un conjunto de carpetas locales normales. Nunca se escribe nada en el otro repositorio, y tus propios ajustes de copia quedan intactos (la sesión vive en memoria y caduca sola). Mover un contenedor del servidor A al servidor B no significa reapuntar tus ajustes de repo y revertirlos después. Esta ventana es de un solo uso: abre una sesión, restaura lo que eliges y olvida la otra instancia. Si quieres en cambio un arreglo permanente, en el que esta máquina trae las instantáneas de otra instancia a su propio repositorio según un calendario, eso es el rol **Recolector** en la página Instancias.

Un contenedor cuya red no existe en este servidor, como una red `br0` de Unraid en un host Docker normal, muestra un selector de red bajo su fila. BombVault lo crea en la red que elijas, junto con sus otras redes. La IP fija y la dirección MAC pertenecían a la red antigua y se descartan, así que las asigna la nueva red.

## Guardián de claves {#key-keeper}

Un guardián de claves es una instancia de tu grupo que guarda una copia sellada del kit de recuperación de este servidor. No puede abrir la copia por sí misma. Si este servidor se pierde algún día, una instalación nueva que se una a tu grupo pide la copia al guardián y la abre con tu código de depósito.

1. En la página Instancias, activa **Guardián de claves** en la tarjeta de la instancia que debe guardarla, y elige **Depositar**. Hacer un depósito requiere una contraseña de inicio de sesión.
2. Este servidor muestra el **Código de depósito** una sola vez: veinte letras y dígitos en cuatro grupos de cinco. Anótalo y confirma con **He anotado el código**. Este servidor no guarda el código, solo lo que necesita para sellar un depósito para él.
3. El guardián ya tiene el depósito, y este servidor lo renueva cuando cambia el kit. **Depósito de claves** en la tarjeta de esta instancia lista el **Depósito de este servidor** con cada guardián y su estado, y **Depósitos que guarda este servidor** para los de otros. **Crear un código nuevo** sustituye el código; cada guardián pasa al nuevo cuando recibe el depósito nuevo.

Ese día necesitas tres cosas: las doce palabras de tu grupo, el código de depósito y acceso a la interfaz del guardián, donde alguien tiene que pulsar **Permitir**. En el Asistente de emergencia elige **A través del grupo**, escoge el depósito, pídelo y escribe el código de depósito. El código se usa en el servidor nuevo y no se envía al guardián. En el guardián, la solicitud aparece en **Depósito de claves** con quién la pide, si esa instancia se vio alguna vez allí y cuántas veces ha pedido; **Permitir** entrega la copia sellada, y permitir requiere una contraseña de inicio de sesión. La instalación nueva tiene 30 minutos para recogerla tras el Permitir, una solicitud que nadie responde caduca al cabo de un día, y una instancia que pide demasiado a menudo tiene que esperar una hora.

Sin el código nadie puede abrir la copia, el guardián incluido. Si pierdes el código, crea uno nuevo mientras este servidor siga funcionando; si el servidor ya ha desaparecido, el kit de recuperación y la APP_KEY escrita siguen sirviendo. El guardián puede borrar un depósito en cualquier momento, y el propietario tiene entonces que pedirlo de nuevo.

## Kit de recuperación {#recovery-kit}

Esta es la pieza que hace posible la recuperación ante desastres incluso cuando no hay ningún BombVault en ejecución.

**Descargar kit de recuperación** en la tarjeta **Para emergencias** de **Ajustes, Restauración** te da la **clave**, la **contraseña de restic derivada**, cada **destino** con el repositorio de cada sección, los accesos que una instalación nueva necesita para llegar a ellos y los **comandos exactos de restic**, para que puedas restaurar directamente con la CLI de restic en cualquier máquina. Requiere una contraseña de inicio de sesión. El kit imprime la clave también con el cifrado desactivado, porque las definiciones de contenedores y VMs siempre se sellan con ella. Tras una adopción imprime la clave de copias adoptada y la APP_KEY de este servidor, que una instalación nueva necesita solo para recuperar los ajustes. Una tarjeta **Guarda tu kit de recuperación** en el resumen te lo recuerda hasta que lo has guardado.

!!! danger "Guarda el kit de recuperación fuera del servidor"
    El kit contiene el secreto que descifra tus copias. Guárdalo en un lugar seguro y separado del servidor (un gestor de contraseñas, una copia impresa en una caja fuerte). Si pierdes BombVault y `APP_KEY` sin kit de recuperación y sin depósito de claves, tus copias cifradas no se pueden recuperar.

!!! warning "La instantánea más reciente no siempre es la que hay que restaurar"
    Desde restic 0.17, `restic snapshots` muestra el tamaño de cada instantánea. Tras una pérdida de datos la instantánea más reciente puede ser la vaciada, así que no restaures una instantánea mucho más pequeña que las anteriores. Tras un ransomware puede ser la cifrada, con el tamaño habitual. Si BombVault sigue funcionando, mira primero su página **Anomalías**: nombra la última copia buena, y la ventana de restauración la preselecciona. Una restauración no necesita ningún dato de anomalías de BombVault, y la pausa de retención solo conserva más instantáneas.

### Sellar el kit

Si has activado **Cifrar exportaciones con age** en **Ajustes, Sistema**, el kit también se sella con ella y se descarga como `bombvault-recovery-kit.md.age`. Está en ASCII blindado en lugar de binario, así que sigue siendo texto: pegarlo en un gestor de contraseñas o imprimirlo funciona exactamente como antes, solo que el contenido es ilegible sin tu clave.

!!! warning "No guardes la clave age dentro del kit"
    Necesitas tu clave age **privada** para abrir un kit sellado. Guárdala en un sitio que no dependa del propio kit, o tendrás dos cosas que recuperar en lugar de una. Sellar compensa cuando el kit se guarda en un sitio que no controlas del todo (un gestor de contraseñas compartido, notas en la nube, una copia impresa en una oficina); un kit en tu propia caja fuerte ya está protegido por la caja fuerte.

    Con el cifrado activado y sin ningún destinatario utilizable configurado, la descarga se rechaza sin más. BombVault nunca recurre a entregar la clave maestra en claro.

### Si no tienes el kit a mano

La contraseña no se guarda en ningún sitio, se **calcula** a partir de la clave de las copias, así que puedes reproducirla tú mismo solo con la clave y un shell:

```sh
printf 'bombvault:restic-repo' \
  | openssl dgst -sha256 -mac HMAC -macopt hexkey:$APP_KEY -r \
  | cut -d' ' -f1
```

Es HMAC-SHA256 sobre la cadena fija `bombvault:restic-repo`, con clave en los bytes en bruto de la `APP_KEY` hexadecimal, impreso como 64 caracteres hexadecimales en minúscula. El mismo valor está en el kit, listado como la contraseña de restic derivada; esto es para el día en que el kit esté en un sitio donde tú no estás. En un servidor que adoptó las copias de otro, usa la clave de copias adoptada, que es la antigua APP_KEY de ese servidor.

!!! warning "Para un repositorio recibido, usa la clave de la instancia EMISORA"
    Un repositorio que llegó aquí mediante una copia externa lo creó la máquina que lo envió, con **su** clave. Derivar de la clave de la máquina receptora produce una contraseña que restic rechazará, lo que se parece exactamente a un repositorio corrupto y no lo es. Esta es la razón habitual por la que `restic check` en un repo recibido pide la contraseña una y otra vez.

Como las definiciones de recuperación viven **dentro** de cada repo (`<repo>/def`, `<repo>/vm-def`), una carpeta de repo copiada es totalmente autocontenida, de modo que el kit más el repo es todo lo que una restauración desde cero necesita.

## Recuperar un volcado de base de datos {#database-dumps}

Un volcado de base de datos es un punto de restauración propio en el repositorio de contenedores, etiquetado `dbdump:<contenedor>`, que contiene el único archivo `/dbdump/<contenedor>.sql`. BombVault los lista, descarga e importa en la pestaña **Copias de seguridad** del contenedor; a continuación están los mismos pasos solo con restic, para el día en que BombVault no esté.

```sh
restic -r <repo> snapshots --tag dbdump:<container>
restic -r <repo> dump --tag dbdump:<container> latest /dbdump/<container>.sql > <container>.sql
```

Las etiquetas `dbversion:` y `dbname:` de cada volcado dicen de qué versión de servidor viene y qué bases contiene. Un archivo completo termina con `-- PostgreSQL database cluster dump complete` o `-- Dump completed`.

Impórtalo en un contenedor de la misma versión o una más reciente (PostgreSQL), o de la misma versión mayor (MySQL y MariaDB), arrancado una vez con la carpeta de datos vacía para que se inicialice. El anfitrión no necesita cliente de base de datos, el contenedor ya tiene uno:

```sh
docker exec -i <container> sh -c 'exec psql -X -U "${POSTGRES_USER:-postgres}" -d postgres' < <container>.sql
docker exec -i <container> sh -c 'exec mariadb -uroot -p"$MARIADB_ROOT_PASSWORD"' < <container>.sql
docker exec -i <container> sh -c 'exec mysql -uroot -p"$MYSQL_ROOT_PASSWORD"' < <container>.sql
```

Para una sola base dentro de un volcado completo, MySQL y MariaDB aceptan `--one-database <name>` en el comando del cliente. Un volcado de PostgreSQL tiene una sección por base, cada una empezando por una línea `\connect <name>`: copia esa sección a un archivo aparte e impórtalo con `-d <name>` después de crear la base.

!!! warning "Un volcado hecho como root lleva las cuentas del servidor"
    Un volcado completo de MySQL o MariaDB hecho como root contiene la base de sistema `mysql`, así que importarlo reemplaza las cuentas del servidor nuevo, incluida la contraseña de root, por las del volcado. En PostgreSQL, `role ... already exists` para el usuario que creó el contenedor es esperable e inofensivo.
