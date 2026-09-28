# Lugares de almacenamiento

Un lugar de almacenamiento es un sitio donde BombVault guarda copias de seguridad: una carpeta en este Unraid, un recurso compartido en un NAS, un bucket en un proveedor de nube, un rest-server, una cuenta SFTP o un Nextcloud. Conectas cada lugar una sola vez, en **Ajustes, Almacenamiento**, y sus credenciales, su retención, su protección y el sitio donde se encuentra le pertenecen. Después, los dominios (contenedores, VMs, el flash, la propia configuración de BombVault, los conjuntos de archivos y los conjuntos de datos ZFS) eligen entre los lugares: dónde se almacena cada dominio y adónde se copia. Los conjuntos de datos ZFS tienen su fila en la tarjeta Dominios mientras el dominio ZFS está activado.

## Añadir un lugar {#add-a-place}

**Añadir lugar** abre una ventana con un mosaico por proveedor, en tres grupos: almacenamiento en la nube, servicios autoalojados, y dispositivos NAS y este servidor.

1. Elige un mosaico y rellena su formulario. El botón del ojo muestra un secreto que hayas escrito.
2. **Probar conexión** comprueba el lugar y no crea nada. Para la carpeta de cada dominio indica lo que encontró: vacía o todavía inexistente, con un repositorio de restic ya dentro, o el error que impidió la prueba.
3. Ponle un nombre al lugar; el nombre del proveedor ya viene puesto. Para un dispositivo que gestionas tú, responde a **¿Dónde está el dispositivo?**. Los proveedores de nube están siempre en otro sitio, y una carpeta en este Unraid está siempre aquí.
4. **Añadir** guarda el lugar.

Un lugar nuevo todavía no lo usa ningún dominio. Elígelo en **Almacenado en** o **Copiado a** en la [tarjeta Dominios](#domains), o en la tarjeta de un elemento solo para ese elemento.

## Carpetas {#folders}

Un lugar tiene una carpeta por dominio: `container`, `vms`, `flash`, `config`, `files` y `zfs`, los mismos nombres que usan las ubicaciones de copia predeterminadas. Las carpetas aparecen en los detalles del lugar y se pueden renombrar ahí (consulta [Cambiar una dirección](#addresses)). Un dominio sin carpeta en un lugar no puede elegir ese lugar.

Cuando un dominio usa un lugar en las dos funciones, la segunda función recibe un sufijo y la primera conserva su carpeta. Un lugar que ya recibe las copias de un dominio guarda en `<folder>-direct` los elementos enviados directamente a él; un lugar que ya almacena un dominio recibe sus copias en `<folder>-copies`.

Algunos lugares son en sí un repositorio de restic: una dirección que ya contenía un repositorio cuando se añadió el lugar, un repositorio con nombre de una instalación existente, o un destino de copia en la raíz de un bucket. Un lugar así no tiene carpetas, todos los dominios comparten su único repositorio y no admite una segunda función. Para almacenar más en el mismo proveedor, conecta otro bucket u otra carpeta como un lugar propio.

## Detalles del lugar {#details}

Cada lugar es una fila con su proveedor, para qué se usa y su última prueba o copia. **Probar** comprueba todas las direcciones del lugar, y **Detalles** abre sus ajustes. Cada cambio en los detalles se guarda en el momento en que lo haces.

- **General**: el nombre, el interruptor que activa y desactiva el lugar, la dirección y, para un dispositivo que gestionas tú, **¿Dónde está el dispositivo?** (consulta [Fuera del local](#off-the-premises)).
- **Retención**: conservar últimas, diarias, semanales y mensuales, para cada repositorio del lugar. Un lugar nuevo empieza con las reglas predeterminadas; un lugar con todas las reglas a cero nunca recorta.
- **Protección**: el interruptor **Append-only**. El otro extremo tiene que imponer el append-only; con el interruptor activado, BombVault nunca poda ni borra allí. En un rest-server con append-only activado, **Probar append-only** ejecuta la prueba de manipulación contra cada ruta de dominio, copia activada y repositorio del lugar y muestra una sola respuesta para todo el lugar, *borrado rechazado* o *borrado aceptado* (consulta [Copia externa y recuperación](offsite-recovery.md)). Solo los lugares remotos tienen esta sección, porque nada en esta máquina puede impedir que se borre un repositorio local.
- **Acceso**: las credenciales y, para S3, la clase de almacenamiento. Un lugar que usa las credenciales compartidas recibe un conjunto propio con el primer cambio. Un repositorio directo del lugar que las credenciales nuevas no pueden abrir conserva las antiguas, y la respuesta lo dice. Los lugares de carpeta, SFTP y rclone no tienen esta sección.
- **Límites**: la velocidad de subida y de bajada y el presupuesto de crecimiento.
- **Carpetas**: un interruptor por dominio, con el nombre de su carpeta. Un dominio desactivado aquí no puede elegir el lugar.

Bajar la retención pide confirmación y dice a cuántos elementos afecta; desactivar el append-only pide confirmación y dice cuántos repositorios del lugar lo pierden. Desactivar un lugar desactiva todos sus repositorios; un lugar en el que se almacena un dominio no se puede desactivar.

## La tarjeta Dominios {#domains}

La tarjeta tiene una fila por dominio, con su calendario, dónde se almacena, adónde se copia y sus excepciones.

- **Almacenado en**: mientras la ubicación de copia del dominio no contenga copias de seguridad, el lugar elegido pasa a ser el lugar donde se almacena el dominio, y la ubicación se traslada allí. En cuanto contiene copias de seguridad, la elección para contenedores, VMs y conjuntos de archivos pasa a ser el valor predeterminado de los elementos nuevos, que lo toman en su primera copia de seguridad; los elementos que ya tienen copias de seguridad se quedan donde están, porque BombVault nunca mueve una copia de seguridad. El flash y la propia configuración de BombVault sí se trasladan al lugar nuevo, y las copias de seguridad ya escritas se quedan en el antiguo.
- **Copiado a**: un chip por cada lugar que puede recibir las copias del dominio. Marcar un chip convierte el lugar en destino de copia del dominio; la primera vez, BombVault dice de antemano cuántos elementos e instantáneas y cuántos datos envía la primera ejecución. Desmarcarlo detiene las copias nuevas: las copias que ya están allí se quedan y envejecen según la retención del lugar, y los elementos con elección propia siguen copiando allí. Desmarcar el último chip detiene todas las copias, también a lugares añadidos más tarde, hasta que se vuelva a marcar uno. Un lugar desactivado aparece como un chip atenuado y no se puede elegir.
- **Excepciones**: los elementos con elección propia, en una lista con enlaces a sus tarjetas.
- **Copiar ahora** ejecuta al momento las copias del dominio.

Un dominio en pausa tras una reconstrucción mediante Descubrir muestra la pausa en su fila, con **Confirmar valor predeterminado** (consulta [Ubicación por elemento](offsite-recovery.md#placement)).

## Cambiar una dirección {#addresses}

La carpeta de un dominio se puede cambiar en los detalles del lugar, y también la dirección de un lugar local, por ejemplo después de mover a mano un repositorio a otro disco. BombVault prueba cada dirección a la que afecta el cambio y lo acepta cuando cada dirección nueva está vacía y no había nada almacenado en la antigua, o cuando cada dirección nueva contiene el mismo repositorio de restic que la antigua. Cualquier otro caso se rechaza, con el número de copias de seguridad que siguen en la dirección antigua. Un lugar remoto conserva su dirección; para hacer copias de seguridad en otro sitio, conéctalo como un lugar propio.

BombVault construye la lista de lugares a partir de su propia base de datos y nunca lista un repositorio remoto para rellenarla; la prueba solo se ejecuta cuando cambias algo.

## Quitar un lugar {#remove}

Un lugar solo se puede quitar mientras nada lo usa: ningún dominio se almacena allí, ningún valor predeterminado apunta a él, ningún elemento está almacenado allí y ningún repositorio directo en él contiene elementos. Si no, el rechazo enumera lo que lo retiene. Al quitarlo se van con él sus destinos de copia, y también sus propias credenciales, salvo que las use una fuente de recogida u otro lugar. En el almacenamiento en sí no se borra nada, y la confirmación dice cuántas copias se quedan allí.

## Sin lugar {#without-a-place}

Una dirección que no encaja en la forma de un lugar más una carpeta sigue funcionando y aparece en **Sin lugar**, con su dirección. Entre ellas están las direcciones nativas `b2:`, `gs:` y `swift:`. **Asignar a un lugar** vincula esa fila a un lugar, tras la misma prueba que al [cambiar una dirección](#addresses). Un destino de copia sin lugar también aparece en la fila de su dominio, junto a los chips, y sigue copiando. Una fila remota de esa lista tiene su propio interruptor **Append-only**, y desactivarlo pide confirmación con el número de elementos que tienen copias en esa dirección. Un repositorio directo sigue el interruptor de su destino.

## Fuera del local {#off-the-premises}

**¿Dónde está el dispositivo?** tiene dos respuestas: **Aquí, en casa** y **En otro sitio**. Una copia cuenta como una sede propia, para la línea 3-2-1 de las tarjetas y para las comprobaciones externas del Panel, solo cuando su lugar está en otro sitio. Un segundo disco o un NAS en la misma casa es una segunda copia, no una segunda sede. La respuesta no cambia ninguna copia. Los proveedores de nube están siempre en otro sitio y una carpeta en este Unraid siempre aquí, así que el formulario no lo pregunta para ellos; para cualquier otro lugar, cambia la respuesta en sus detalles. Un lugar en otro sitio lleva la marca **Otro sitio** en su fila.

## Tipos de conexión

### Carpeta en este Unraid o un NAS {#kind-local}

La dirección es una ruta bajo `/mnt`, escrita sin `/mnt`, por ejemplo `user/bombvault`, y la carpeta de cada dominio está debajo: `user/bombvault/container`.

- **Carpeta en este Unraid** elige entre los recursos compartidos, los discos y los pools.
- **Synology**, **QNAP**, **TrueNAS**, **Otro Unraid** y **Otro recurso compartido** eligen en `/mnt/remotes`. Monta primero el recurso compartido en Unraid, por ejemplo con el plugin Unassigned Devices. Host Data tiene que estar montado como Read/Write - Slave, o un recurso compartido que se monte después de que arranque BombVault seguirá invisible hasta un reinicio (consulta [Configuración](configuration.md)).

El selector de carpetas crea una carpeta donde está con **Nueva carpeta**. La prueba comprueba que la carpeta está vacía o no existe y que BombVault puede escribir en ella.

### S3 {#kind-s3}

La dirección es `s3:https://<endpoint>/<bucket>/<path>`, por ejemplo `s3:https://s3.eu-central-003.backblazeb2.com/tower-backups/bombvault`.

- **Backblaze B2** solo necesita el ID de clave y la clave de aplicación. BombVault pregunta a B2 a qué bucket, endpoint S3 y carpeta está limitada la clave, y construye la dirección con ellos. Una clave que puede acceder a todos los buckets ofrece sus buckets para elegir.
- **Amazon S3**, **Cloudflare R2**, **Wasabi**, **Hetzner Object Storage**, **Storj**, **IDrive e2**, **Scaleway**, **OVHcloud**, **DigitalOcean Spaces**, **IONOS**, **Contabo**, **Exoscale** y **Vultr** piden la clave y, cuando el proveedor lo necesita, la región, el ID de cuenta o el endpoint. BombVault rellena el endpoint y lista los buckets cuando la clave puede listarlos; si no, escribe el nombre del bucket.
- **Google Cloud Storage** funciona a través de su interfaz S3 con una clave HMAC, que se crea en los ajustes de Cloud Storage, en Interoperabilidad. Un archivo de cuenta de servicio no sirve aquí.
- **MinIO**, **SeaweedFS**, **Garage**, **Ceph**, **JuiceFS**, **RustFS**, **Versity S3 Gateway** y **Otro servicio S3** piden la dirección del servicio y una clave.

La clase de almacenamiento se define en los detalles del lugar, limitada a niveles que una restauración puede leer sin deshielo.

### rest-server {#kind-rest}

La dirección es `rest:<url>/<user>`, por ejemplo `rest:https://nas.lan:8000/tower`. El formulario pide la dirección del servidor, un usuario y una contraseña. Con `--private-repos`, un usuario solo puede acceder a rutas que empiezan por su propio nombre, así que BombVault pone el usuario delante salvo que escribas otra ruta. Cuando el servidor rechaza una ruta que no es la del propio usuario, el error lo dice.

El formulario de rest-server incluye una receta lista para pegar para un rest-server en modo append-only con un usuario para este BombVault. **Mostrar receta** crea una contraseña, que se muestra una sola vez, y da una línea `docker run`, un archivo compose y una plantilla de Unraid, cada uno con la línea `htpasswd` que hay que poner en el servidor; el usuario y la contraseña pasan directamente al formulario.

**Otro BombVault** muestra, encima de sus propios campos, las ofertas abiertas que otras instancias enviaron por Flota. Aceptar una añade un lugar que guarda solo copias del dominio ofrecido, porque una oferta lleva un usuario para ese único dominio. Aceptarla en la página Flota añade el mismo lugar.

### SFTP {#kind-sftp}

La dirección es `sftp://<user>@<host>:<port>/<path>`, por ejemplo `sftp://bv@backup.lan:22/bombvault`. El formulario pide host, puerto y usuario y muestra la clave pública de BombVault. Añade esa clave al `~/.ssh/authorized_keys` del usuario en el servidor; allí no hay que instalar nada más. BombVault acepta la clave de host del servidor en el primer contacto y la comprueba a partir de entonces.

**Hetzner Storage Box** rellena `<user>.your-storagebox.de` y el puerto 23. Instala la clave en la Storage Box con el comando propio de Hetzner, que pide una vez la contraseña de la Storage Box:

```sh
echo '<public key>' | ssh -p 23 <user>@<user>.your-storagebox.de install-ssh-key
```

### WebDAV: Nextcloud, ownCloud, OpenCloud {#kind-webdav}

El formulario pide la dirección del servidor, el usuario y una contraseña de aplicación. Crea la contraseña de aplicación en los ajustes de seguridad de la cuenta, e introduce el ID de usuario y no una dirección de correo. BombVault construye la ruta WebDAV que usa el producto y pasa la conexión a restic mediante las variables de entorno de rclone, con la contraseña en la forma ofuscada de rclone. La dirección queda como `rclone:bvp<id>:<path>`, donde `bvp<id>` es un remoto que solo existe en ese entorno; no se escribe nada en la configuración de rclone.

### Azure Blob {#kind-azure}

La dirección es `azure:<container>:/<path>`. El formulario pide la cuenta de almacenamiento y su clave de acceso; tras **Probar conexión** lista los contenedores de la cuenta para elegir, o escribes el nombre de un contenedor. BombVault pasa la cuenta y la clave a restic como `AZURE_ACCOUNT_NAME` y `AZURE_ACCOUNT_KEY`.

### rclone {#kind-rclone}

La dirección es `rclone:<remote>:<path>`. El formulario lista los remotos de la configuración de rclone de BombVault para elegir. Para sustituir esa configuración, pega un `rclone.conf` completo en **Configuración de rclone** y haz clic en **Guardar configuración**. Se guarda al momento y sirve a todos los lugares rclone, tanto si la ventana añade uno después como si no.

## Copias entre lugares con credenciales distintas {#different-credentials}

Un dominio almacenado en un lugar remoto es el origen de sus copias. `restic copy` se ejecuta con un solo entorno, y BombVault añade las credenciales del origen a las del destino siempre que los dos no den a la misma variable valores distintos. Un lugar Nextcloud y un lugar B2 usan variables distintas, así que un dominio almacenado en Nextcloud se puede copiar a B2. Dos cuentas S3 o dos usuarios de rest-server necesitarían las mismas variables con valores distintos; restic no puede tomar ambos, y el chip de la tarjeta Dominios dice que las credenciales no encajan.
