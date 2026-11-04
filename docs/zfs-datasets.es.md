# Conjuntos de datos ZFS

Los conjuntos de datos ZFS son elementos de la lista de **Copias de seguridad**, como los contenedores y las VMs. Un elemento es un conjunto de datos junto con todos los que tiene debajo. En cada copia BombVault toma una instantánea ZFS de todo el árbol, de modo que todos sus conjuntos de datos se capturan en el mismo instante. Después lee los archivos de cada conjunto de datos desde esa instantánea y los guarda con restic, igual que guarda una carpeta, y elimina la instantánea enseguida. Las copias están deduplicadas, puedes explorar cada una de ellas y se pueden restaurar archivos sueltos.

La copia nunca envía un flujo a restic y nunca revierte un conjunto de datos. BombVault destruye únicamente las instantáneas que creó él mismo. La [réplica](#replica) opcional es el único sitio que usa `zfs send`: copia los conjuntos de datos a un segundo servidor ZFS y no toca la copia de seguridad.

## Requisitos {#requirements}

- **El enlace SSH a este servidor.** Los conjuntos de datos ZFS usan la misma clave, host y usuario que las copias de VMs. Si las copias de VMs ya funcionan, esto también. Si no, sigue la [guía de copias de VMs por SSH](vm-backup-ssh-setup.md). Los campos de la plantilla se llaman **SSH del host: Dirección**, **SSH del host: Puerto** y **SSH del host: Usuario**.
- **El comando `zfs` en ese host.** Unraid 6.12 y posteriores y TrueNAS SCALE lo tienen.
- **Host Data asignado como `/mnt` con Access Mode Read/Write - Slave.** Es el valor predeterminado de la plantilla. La instantánea de un conjunto de datos aparece dentro de la carpeta `.zfs/snapshot` del conjunto de datos solo después de que BombVault haya arrancado, así que el contenedor tiene que recibir los montajes que el host hace más tarde.
- **Los conjuntos de datos montados bajo `/mnt`.** En Unraid los pools viven en `/mnt/<pool>`, así que ya es así.

Activa la sección en **Ajustes, General** (Conjuntos de datos ZFS). **Conexión ZFS con este servidor**, bajo **Más** en la página Copias de seguridad, abre entonces **Conexión con este servidor**. Prueba el enlace SSH, nombra el usuario y el host a los que se conecta y dice qué falta cuando algo falta. Cuando el enlace está caído, la lista de Copias de seguridad lo dice con **Ver conexión**, y la tarjeta **Integración con el host** del resumen muestra el mismo resultado.

## Elementos y conjuntos de datos hijos {#items-and-children}

Elige **Añadir** en la página Copias de seguridad y luego **Un conjunto de datos ZFS**. La lista viene del servidor. Elige el conjunto de datos que está en lo más alto de lo que quieres copiar, por ejemplo `cache/appdata`, y el elemento lo cubre a él y a todos los que tiene debajo.

- **Los conjuntos de datos hijos nuevos se unen por sí solos.** Un conjunto de datos creado más tarde bajo el elemento se copia con la siguiente ejecución, y esa ejecución lo nombra como nuevo. Su primera copia lo lee entero una vez. Después solo se leen los cambios.
- **Puedes dejar fuera hijos concretos.** Desactiva un hijo en la pestaña Ajustes del elemento y queda fuera junto con todo lo que tiene debajo. Un hijo dejado fuera que ya no existe en el servidor se marca como tal y se puede quitar de la lista.
- **Los hijos que no se pueden leer se saltan, nunca en silencio.** La ejecución los lista, el elemento muestra cuántos se saltaron, y la tarjeta **Sin copia de seguridad** del resumen cuenta cada uno como sin protección. La ejecución sigue copiando todo lo demás y no falla por un hijo saltado. Los motivos están en la [tabla de códigos de motivo](#reason-codes): un conjunto de datos que no está montado, que tiene `canmount=off`, un punto de montaje `legacy` o ninguno, una clave de cifrado que no está cargada, el acceso a instantáneas desactivado, o un punto de montaje que BombVault no puede ver.
- **Un conjunto de datos saltado no se lleva consigo a sus hijos.** Un conjunto de datos `canmount=off` que solo contiene otros conjuntos de datos se salta (se muestra como "structure only") y sus hijos montados se copian. Un conjunto de datos cifrado cuya clave no está cargada se salta junto con los hijos que comparten su clave.
- **Los hijos que son discos de VM o datos del sistema empiezan desactivados** en el diálogo de añadir, con el motivo junto al interruptor. Añadir un pool entero pide una confirmación que lista lo que contiene.

### Volúmenes {#volumes}

Un volumen (zvol) contiene un disco virtual en lugar de archivos, y un elemento ZFS nunca copia uno.

- Un volumen que usa una VM se copia con esa VM.
- Un volumen que ninguna VM usa (una extensión iSCSI, un disco que desconectaste) **no lo copia BombVault**. La ventana de añadir y la página Copias de seguridad cuentan estos volúmenes y lo dicen. Una versión futura los copiará.

Los volúmenes dentro del árbol de un elemento se saltan y se nombran en cada ejecución.

### El almacenamiento de Docker {#docker-storage}

Con el controlador de almacenamiento ZFS de Docker, cada capa de imagen es un conjunto de datos con un punto de montaje `legacy`. El diálogo de añadir los pliega en una línea por padre. Un árbol que contiene más de 20 conjuntos de datos así no puede convertirse en un elemento: mientras existe una instantánea suya, Docker no puede eliminar capas de imagen. Añade en su lugar los conjuntos de datos que tiene debajo, por ejemplo `appdata`.

### Los elementos nunca se solapan {#overlap}

Un conjunto de datos solo puede pertenecer a un elemento. BombVault rechaza un elemento nuevo que esté dentro de uno existente, o que contendría uno. Para fusionar varios elementos hijos en un elemento padre, elimina antes los elementos hijos y elige conservar sus copias, y luego añade el padre. Cada conjunto de datos conserva su historial bajo su propio nombre, así que la siguiente copia continúa donde lo dejaron los elementos antiguos y no lo lee todo de nuevo.

## Detener contenedores y comandos alrededor de la instantánea {#consistency}

Una instantánea de una base de datos en ejecución es como un corte de luz repentino: la base de datos normalmente se recuperará, pero tiene que hacerlo. Cada elemento puede hacer dos cosas al respecto, y ambas cubren solo el instante de la instantánea, no toda la copia.

- **Detener estos contenedores para la instantánea.** BombVault detiene los contenedores indicados, toma la instantánea y los vuelve a iniciar enseguida. Los contenedores de un mismo nivel de dependencias se detienen en paralelo, los dependientes primero, así que toda la ventana suele ser de unos segundos. La ejecución muestra cuánto duró. La copia lee entonces la instantánea congelada mientras las apps ya están funcionando de nuevo. Solo se detienen los contenedores que estaban en ejecución.
- **Un comando antes y después de la instantánea.** Se ejecuta dentro de un contenedor que eliges, por ejemplo para volcar una base de datos en el conjunto de datos justo antes de la instantánea, sin detener nada. Si el comando anterior a la instantánea falla, la copia falla y no se toma ninguna instantánea. Un comando posterior a la instantánea que falla se muestra en la ejecución pero no hace fallar la copia.

Qué pasa cuando algo sale mal:

- Si un contenedor no se puede detener, BombVault inicia los que ya había detenido y la copia falla, nombrando el contenedor. Nunca recurre a una instantánea con las apps en ejecución.
- La parada espera a que termine la copia de un contenedor en curso (hasta 30 minutos para una ejecución manual, hasta el límite de tiempo de la copia para una programada), de modo que las dos nunca detienen e inician el mismo contenedor a la vez.
- Antes de que se detenga el primer contenedor, BombVault anota cuáles detiene. Si BombVault es eliminado dentro de la ventana, vuelve a iniciar esos contenedores la próxima vez que arranca, envía una notificación, y el elemento muestra una nota roja por cada contenedor que no pudo iniciar.

Los [volcados automáticos de bases de datos](backups.md#database-dumps) se ejecutan con la copia propia de un contenedor, no con un elemento ZFS. Una base de datos cuyo contenedor se copia solo a través de su conjunto de datos no recibe volcado, así que dale un comando aquí.

Un contenedor puede estar en esta lista y copiarse a la vez como contenedor. Sus datos se guardan entonces dos veces, en dos repositorios, y la copia total lo detiene dos veces. El elemento lo dice.

## Restaurar {#restore}

Abre el elemento y elige **Restaurar**, o elígelo en la página **Restauración**. La ventana de restauración pide la copia y luego el **Conjunto de datos**. Por defecto es el conjunto de datos superior del elemento. En **Qué** restauras todo o solo los archivos y carpetas que elijas. **Dónde** ofrece:

- **Restaurar dentro del conjunto de datos.** Los archivos de la copia se escriben en el punto de montaje del conjunto de datos. Los archivos con el mismo nombre se sobrescriben, los demás se quedan. El conjunto de datos nunca se revierte ni se reemplaza. BombVault comprueba que el conjunto de datos está montado, visible y escribible, una vez antes de empezar y otra justo antes de escribir. Donde hay un conjunto de datos hijo montado dentro, no se escribe nada: el hijo conserva sus archivos, propietario y permisos, y se restaura desde su propia copia.
- **En una carpeta.** Elige una carpeta bajo `/mnt`. BombVault comprueba que la carpeta está en un pool o recurso compartido montado y que hay espacio libre suficiente. Esto funciona sin el enlace SSH y para conjuntos de datos que ya no existen.
- **En un conjunto de datos nuevo.** Pon nombre a un conjunto de datos que aún no existe. La ventana sugiere `<dataset>-restored`. BombVault lo crea con las propiedades ZFS guardadas en la copia y restaura en él, consulta [Restaurar como conjunto de datos nuevo](#new-dataset).

Elegir **Todos los conjuntos de datos de esta copia** como conjunto de datos escribe cada conjunto de datos del árbol en su propia subcarpeta de la carpeta que elijas. Los conjuntos de datos que se saltaron en esa copia se nombran. Para restaurar desde el repositorio de otro BombVault, usa **De una carpeta de copias antigua** en la página Restauración. Siempre restaura en una carpeta: todos los conjuntos de datos de una copia, cada uno en su propia subcarpeta, o un conjunto de datos del árbol, entero o con archivos elegidos.

La lista de contenedores a detener del elemento también se ofrece para una restauración dentro del conjunto de datos. Esos contenedores se quedan detenidos durante toda la restauración, y las copias de contenedores esperan mientras tanto.

### La instantánea de seguridad {#safety-snapshot}

Antes de escribir en un conjunto de datos, BombVault toma una instantánea ZFS de ese conjunto de datos, llamada `bombvault-prerestore-<time>`. **Guardar antes una instantánea ZFS del estado actual** está activado por defecto. Desactivarlo requiere una segunda confirmación. Si la instantánea no se puede tomar, no se restaura nada.

BombVault nunca borra por sí mismo una instantánea de seguridad. El elemento las lista con su antigüedad y tamaño, cada una con una acción **Eliminar**, y avisa cuando la más antigua tiene más de 30 días, porque conserva en el pool datos borrados y modificados.

Para volver atrás tras una restauración, copia archivos sueltos de `.zfs/snapshot/bombvault-prerestore-<time>` dentro del conjunto de datos. `zfs rollback <dataset>@bombvault-prerestore-<time>` solo funciona mientras sea la instantánea más reciente de ese conjunto de datos. `zfs rollback -r` borra todas las instantáneas más nuevas, las automáticas incluidas.

### Restaurar como un conjunto de datos nuevo {#new-dataset}

BombVault guarda con cada copia las propiedades ZFS definidas localmente de cada conjunto de datos: compresión, tamaño de registro, cuota, reserva, atime, xattr, acltype, sensibilidad a mayúsculas y tus propias propiedades de usuario. Los valores heredados y los de solo lectura se dejan fuera, porque vuelven por sí solos. Las copias anteriores a que BombVault las guardara no tienen ninguna.

- **En un conjunto de datos nuevo** ejecuta `zfs create` con todas las propiedades guardadas. La sensibilidad a mayúsculas, la normalización y utf8only solo se pueden fijar así. Las cuotas y reservas se fijan después de los archivos, para que no puedan rechazarlos. El punto de montaje se deja fuera, para que la copia no choque con el original, y también `canmount`, `readonly` y los ajustes de cifrado, para que la restauración pueda escribir. Un conjunto de datos nuevo bajo uno cifrado adopta su cifrado. El conjunto de datos que está por encima del nuevo tiene que existir. Si algo falla después de crear el conjunto de datos nuevo, se queda en el servidor, porque BombVault nunca destruye un conjunto de datos.
- **Restaurar dentro del conjunto de datos** lista las propiedades guardadas junto a la restauración. **Aplicar también estas propiedades** fija las que un conjunto de datos existente aún puede aceptar, antes de escribir ningún archivo. Las cuotas y reservas se fijan después de los archivos, para que no puedan rechazarlos. Sin ese interruptor el conjunto de datos conserva sus ajustes.

## Qué hay en la copia {#contents}

En la copia: los archivos y carpetas de cada conjunto de datos copiado, con su propietario, modos, marcas de tiempo y atributos extendidos tal como los guarda restic, y las propiedades ZFS definidas localmente de cada conjunto de datos.

No en la copia:

- el propietario y el modo de la propia carpeta superior de cada conjunto de datos (todo lo que hay debajo sí está incluido). Una restauración dentro del conjunto de datos deja la carpeta superior existente como está, una restauración en una carpeta la crea con el modo `0755`,
- las instantáneas ZFS existentes,
- los hijos que se saltaron o se dejaron fuera,
- los volúmenes.

Para restaurar en un pool nuevo, crea el pool y restaura cada conjunto de datos en un conjunto de datos nuevo. Aún no se ha verificado si las ACL NFSv4, como las que usa TrueNAS en los conjuntos de datos SMB, vuelven como esperas, así que comprueba una restauración con tus propios datos antes de confiar en ellas.

## Conjuntos de datos cifrados {#encryption}

Un conjunto de datos cifrado solo se copia mientras su clave está cargada. Si no, se salta con un aviso. Carga la clave con `zfs load-key` y monta el conjunto de datos. BombVault lee los datos descifrados y los guarda en el repositorio de restic, que está cifrado. Si desactivaste el cifrado en BombVault, ese repositorio no lo está.

## Réplica {#replica}

Una réplica es una copia de los conjuntos de datos de un elemento en un segundo servidor ZFS. BombVault la mantiene al día con `zfs send` y `zfs receive`. La primera ejecución lo envía todo, y después solo viajan los bloques modificados. En el otro servidor puedes montar la copia enseguida.

Una réplica nunca sustituye a la copia de seguridad. Las versiones anteriores, los archivos sueltos y la comprobación siguen saliendo de las copias, y la réplica conserva solo tantas instantáneas como fijes. Una réplica al día cuenta como una copia fuera de las instalaciones, pero no como una copia de seguridad.

La réplica no espera a la copia. **Después de cada copia** significa después de la ejecución de copia del elemento, haya funcionado, fallado o no tenido nada que hacer. Cuando la sección ZFS no tiene ningún destino activado y el elemento no tiene un repositorio propio, una ejecución de copia del elemento solo envía la réplica, y la tarjeta Réplica lo dice en el globo junto a **Cuándo**. La lista de Copias de seguridad, la cabecera del elemento y el historial de ejecuciones muestran la réplica: **Replicando** mientras se ejecuta, y **Replicado** con la hora cuando termina.

**Solo réplica, sin copia** en la tarjeta Réplica hace que toda la sección ZFS envíe réplicas y no haga copias. La carpeta propia y los destinos externos conservan sus interruptores, así que al desactivarlo se vuelve a copiar en ellos, y la tarjeta de cada elemento ZFS muestra el interruptor de la sección tal como está. Un elemento así espera su primera réplica en lugar de una copia, **Copiar ahora** se ofrece como **Replicar ahora**, y los contenedores del elemento se detienen y sus comandos se ejecutan alrededor de la instantánea como en una copia. Sus réplicas cuentan para un calendario cada pocos días, la alerta de retraso y la cobertura.

Actívala en la tarjeta **Réplica** de la pestaña Ajustes del elemento. La tarjeta lista cada conjunto de datos y volumen con su estado, y **Replicar ahora** inicia una ejecución. Una ejecución de réplica tiene su propio bloqueo, así que una primera transferencia larga nunca frena las copias.

| Campo | Qué fija |
|---|---|
| **Replica en** | El destino: un servidor ZFS (marcado **por SSH**) o una **instancia emparejada**. **Añadir servidor ZFS** configura uno nuevo. |
| **Cuándo** | **Después de cada copia**, que significa después de la ejecución de copia del elemento, o **Plan propio**. |
| **Conservar en el destino** | Cuántas instantáneas se quedan en el servidor ZFS: **Corto**, **Equilibrado**, **Largo** o **Valores propios**. Una instancia emparejada lo decide ella misma. |
| **Conservar en este servidor** | Cuántas de las instantáneas de réplica más recientes se quedan también en este servidor. 0 o 1 conserva solo la última. |
| **Solo réplica, sin copia** | Toda la sección ZFS envía réplicas y no hace copias. |

![La tarjeta Réplica de un elemento ZFS.](assets/screenshots/zfs/replica-dark.png#only-dark)
![La tarjeta Réplica de un elemento ZFS.](assets/screenshots/zfs/replica-light.png#only-light)

**Replicar todo ahora**, junto a las acciones de copia cuando el mosaico ZFS está elegido en la página Copias de seguridad, envía la réplica de cada elemento ZFS, una tras otra. El servidor ejecuta el lote, así que sigue cuando abandonas la página, y un elemento que no puede replicar se nombra y se salta.

### Enviar a un servidor ZFS {#replica-push}

Cualquier máquina con ZFS y SSH puede recibir, por ejemplo un segundo Unraid o un TrueNAS. BombVault no tiene que estar ejecutándose allí.

1. En **Ajustes, Emparejamiento**, elige **Conectar por SSH** en **Servidor ZFS sin BombVault**, o **Añadir servidor ZFS** en la tarjeta **Réplica** del elemento.
2. Introduce dirección, usuario y puerto. El diálogo muestra la clave pública de BombVault. Añádela al `~/.ssh/authorized_keys` del usuario en el servidor. En Unraid está en **Users, root, SSH keys**. Según **Display Settings**, Users está en el menú de la cabecera o bajo Settings. En TrueNAS, edita el usuario en **Credentials, Users**. Esta es la clave de la réplica. Es distinta de la clave del enlace SSH de las VMs y del host, así que añádela aunque aquella ya esté autorizada.
3. Prueba la conexión. El diálogo lista entonces los pools del servidor. Elige uno y fija la raíz, que por defecto es `<pool>/bombvault-replica`.
4. Elige el servidor nuevo en la tarjeta **Réplica** del elemento. El servidor tiene entonces una tarjeta propia en la página Instancias, y **Abrir** lleva a su página.

Con root no hace falta nada más. Un usuario propio necesita estos permisos en el pool del destino, que el diálogo también muestra:

```
zfs allow <user> receive,create,mount,rollback,destroy,userprop <pool>
```

En el origen el mismo tipo de usuario necesita estos en el conjunto de datos superior del elemento:

```
zfs allow <user> send,snapshot,hold,release,bookmark,destroy <dataset>
```

En esta dirección el BombVault que contiene el elemento también tiene una clave que puede escribir en el servidor.

BombVault fija la clave del host que muestra el servidor en la primera conexión y rechaza una distinta más tarde. Si el servidor se reinstaló, pulsa **Olvidar clave de host** en la página del servidor, y la siguiente conexión fija la clave nueva.

Cuando el host de este servidor puede llegar él mismo al servidor ZFS, canaliza `zfs send` directamente a su propio ssh hacia el servidor, con la clave de la réplica y la clave del host fijada, de modo que el flujo no pasa por BombVault. Si no, el flujo pasa por BombVault.

### Enviar a una instancia emparejada {#replica-receive}

Un BombVault emparejado puede recibir él mismo la réplica. Nadie obtiene acceso SSH al otro host, y ninguna clave va a un archivo `authorized_keys`.

1. En la tarjeta **Réplica** del elemento, elige la instancia emparejada como destino, o activa el rol **Servidor ZFS** en la tarjeta de esa instancia de la página Instancias y elige allí los elementos ZFS. La tarjeta muestra **Esperando aprobación** hasta que responde.
2. En la instancia receptora, abre **Instancias**. La solicitud aparece en la tarjeta de la instancia emisora como una insignia, y sus **Detalles** la listan. Elige el pool y la raíz en ese servidor y cuántas instantáneas se quedan, y luego pulsa **Permitir** o **Rechazar**.
3. Tras **Permitir** el origen envía según su propio calendario, como con cualquier otro destino.

La instancia receptora acepta solo lo que cubre la aprobación: los conjuntos de datos del elemento, en su propia raíz. Ejecuta ella misma `zfs receive`, y el origen no tiene ninguna forma de borrar ni revertir nada allí. Así la copia sobrevive a un origen que alguien haya tomado. La instancia receptora conserva su propia retención. El origen solo sugiere una regla cuando la pide.

**Revocar acceso** en la instancia receptora termina la aprobación en cualquier momento y se lo dice al origen, que muestra entonces que la aprobación se retiró y se detiene. Lo que la instancia receptora ya contiene se queda allí. Una solicitud rechazada sigue rechazada. Más conjuntos de datos, o una solicitud tras una revocación, vuelven a preguntar.

La instancia receptora tiene que responder por HTTPS. El origen no envía nada a una dirección http normal, ya que el token y los datos cruzarían la red sin cifrar. Cada transferencia además deja libre una décima parte del pool receptor y se detiene antes de comérsela, de modo que un origen no pueda llenar el pool del servidor receptor.

### Dónde acaban los datos {#replica-target}

Cada conjunto de datos acaba en `<root>/<server>/<pool>/<path>`. La carpeta del servidor es el nombre de la instancia de origen, fijado en la primera transferencia, así que dos servidores con el mismo nombre de pool nunca se estorban. Por ejemplo, `cache/appdata` de un servidor llamado `tower` acaba en `backup/bombvault-replica/tower/cache/appdata`.

Tras configurar de nuevo BombVault, el destino rechaza la carpeta que llenó este BombVault antes. La tarjeta Réplica ofrece entonces **Tomar la carpeta**, y la siguiente réplica continúa desde las copias que contiene. Tómala solo cuando ningún otro BombVault siga replicando en esa carpeta.

La copia del destino es de solo lectura. Por defecto no está montada, así que nunca cubre nada en ese servidor. Un servidor ZFS conectado por SSH tiene en su página un interruptor **Montar solo lectura en el servidor**: activado, BombVault monta allí las copias en solo lectura después de cada ejecución, para que puedas leer y copiar sus archivos en el destino. Las copias cifradas siguen bloqueadas hasta que se carga su clave, y desactivarlo no desmonta nada. Una instancia emparejada que recibe la réplica la mantiene sin montar. Las propiedades ZFS viajan con ella, salvo el punto de montaje y `sharenfs` y `sharesmb`. Las reservas `reservation` y `refreservation` también se quedan atrás, de modo que la copia solo ocupa el espacio que necesitan sus datos.

### Qué se incluye {#replica-contents}

Entra todo lo que copia el elemento, y también los volúmenes que tiene debajo, que la copia salta. Un conjunto de datos hijo que desactivaste en el elemento se queda fuera. Todos los conjuntos de datos de una ejecución proceden de una instantánea, como en la copia.

### Cuánto tiempo se conservan las instantáneas {#replica-retention}

En el destino, una réplica nueva conserva 7 instantáneas diarias y 3 semanales. Elige en su lugar **Corto**, **Equilibrado** o **Largo**, o fija **Valores propios**. Solo se quitan allí las instantáneas llamadas `bombvault-replica-<14 digits>`, y nunca la más reciente que comparten ambos lados.

En el origen, BombVault conserva la última instantánea de réplica, más un marcador por cada estado que ha enviado. Los marcadores no cuestan espacio. La siguiente transferencia parte de ellos.

Para conservar más también en el origen, fija **Conservar en este servidor** en la tarjeta Réplica, por ejemplo en 7. Esa cantidad de las instantáneas de réplica más recientes se queda entonces en este servidor tras cada réplica, junto a lo que conserva el destino. 0 o 1 conserva solo la última. La última lleva una retención, para que la base de la siguiente transferencia no se destruya por accidente. Las más antiguas no la tienen. Cada instantánea retiene los bloques que cambiaron después de ella, así que cada una que conserves ocupa espacio en el pool. Desactivar la réplica o pasarla a otro destino, o eliminar el elemento, también las quita de este servidor.

Los archivos de una instantánea conservada se pueden leer o copiar enseguida, desde la carpeta `.zfs/snapshot` dentro del conjunto de datos o desde un clon, sin tocar el conjunto de datos ni la réplica:

```
ls /mnt/cache/appdata/.zfs/snapshot/bombvault-replica-20261006014100
zfs clone cache/appdata@bombvault-replica-20261006014100 cache/appdata-20261006
```

Un clon no ocupa espacio hasta que cambia algo en él. Destrúyelo con `zfs destroy cache/appdata-20261006` cuando termines, ya que una instantánea que tiene un clon no se puede quitar.

Volver a la última instantánea de réplica solo necesita `zfs rollback cache/appdata@<latest>` aquí, y nada en el destino. Revertir todo el conjunto de datos a una más antigua destruye todas las instantáneas posteriores, en este servidor y en el destino. La última está retenida, así que levanta primero su retención y luego revierte aquí el conjunto de datos:

```
zfs release -r bombvault-replica cache/appdata@bombvault-replica-20261008014200
zfs rollback -r cache/appdata@bombvault-replica-20261006014100
```

La copia del destino también tiene que volver atrás, pero no siempre a la misma instantánea, ya que sus propias reglas de conservación pueden haberla descartado. Ejecuta la réplica una vez. Se detiene para ese conjunto de datos porque el destino tiene instantáneas más nuevas, y la tarjeta Réplica y el historial de ejecuciones nombran la instantánea a la que revertir el destino: la más reciente que ambos lados aún comparten. Revierte la copia hasta ella en el servidor ZFS, o en el host de la instancia emparejada que recibe la réplica, por ejemplo:

```
zfs rollback -r backup/bombvault-replica/tower/cache/appdata@bombvault-replica-20261005014000
```

`zfs rollback` actúa sobre un conjunto de datos, así que repite las reversiones para cada conjunto de datos hijo que también quieras recuperar. Después, **Replicar ahora** continúa desde la instantánea compartida.

### Conjuntos de datos cifrados en una réplica {#replica-encryption}

Un conjunto de datos cifrado se envía en bruto. Sigue cifrado en el destino, y el destino nunca ve la clave. Guarda bien la clave: la necesitas para abrir la copia tras una restauración, y una réplica sin ella es ilegible.

### Usar la réplica {#replica-use}

Abre la pestaña **Copias de seguridad** del elemento y haz clic en la fila **Réplica ZFS** entre los lugares que contienen sus instantáneas. La hoja lista las instantáneas del destino y muestra los comandos con tus nombres reales.

Para mirar un estado antiguo, clona una instantánea en el destino. Un clon no ocupa espacio hasta que cambia algo, y la réplica queda intacta:

```
zfs clone backup/bombvault-replica/tower/cache/appdata@bombvault-replica-20261006014100 backup/bombvault-replica/clone-appdata
```

Si el origen falla, convierte la copia en un conjunto de datos normal y escribible en el destino:

```
zfs inherit -r readonly backup/bombvault-replica/tower/cache/appdata && zfs inherit -r canmount backup/bombvault-replica/tower/cache/appdata && zfs mount -a
```

BombVault deja de replicar a ese conjunto de datos a partir de entonces, hasta que inicies una primera ejecución nueva.

Para devolver un estado al origen, pulsa **Recuperar como conjunto de datos nuevo** en la hoja. BombVault envía la instantánea a un conjunto de datos nuevo junto al original, llamado `<dataset>-bombvault-restore-` más una marca de tiempo. Nunca escribe sobre el original.

### Ejemplo: una réplica a un segundo equipo Unraid {#example-replica}

El servidor `tower` copia el conjunto de datos `cache/appdata` como un elemento ZFS. Un segundo equipo Unraid en `192.168.1.20`, sin BombVault, tiene un pool llamado `backup` y debe guardar una copia que puedas leer en cualquier momento.

1. Abre **Copias de seguridad**, luego `cache/appdata` y su pestaña **Ajustes**. En la tarjeta **Réplica**, elige **Añadir servidor ZFS** en **Replica en**.
2. Introduce `192.168.1.20`, el usuario `root` y el puerto `22`. El diálogo muestra la clave de la réplica. En el segundo equipo, pégala en **Users, root, SSH keys** y pulsa **Apply**. Esta clave es distinta de la clave SSH del host, así que añádela aunque aquella ya esté ahí.
3. Prueba la conexión. Elige el pool `backup` y conserva la raíz que sugiere, `backup/bombvault-replica`.
4. De vuelta en la tarjeta, el servidor nuevo está elegido en **Replica en**. Deja **Cuándo** en **Después de cada copia**, deja **Conservar en el destino** en sus 7 instantáneas diarias y 3 semanales, y pon **Conservar en este servidor** en `7`.
5. Pulsa **Replicar ahora**. La primera ejecución lo envía todo, así que 300 GB por una línea de 1 Gbit/s tardan al menos 40 minutos. La tarjeta y la lista de Copias de seguridad dicen **Replicando**, y luego **Replicado** con la hora. Las siguientes ejecuciones envían solo los bloques que cambiaron.

Los conjuntos de datos están ahora en `backup/bombvault-replica/tower/cache/appdata` en el segundo equipo, en solo lectura. En el resumen, **Externo** de la sección ZFS cuenta la réplica como una copia fuera de las instalaciones.

Para sacar un archivo del martes pasado tienes tres formas:

- **En este servidor**, porque **Conservar en este servidor** guarda siete instantáneas:

    ```
    cp /mnt/cache/appdata/.zfs/snapshot/bombvault-replica-20261103014100/plex/Preferences.xml /mnt/user/restore/
    ```

- **En el segundo equipo**, con un clon de una instantánea en el destino. La pestaña **Copias de seguridad** del elemento muestra los comandos con tus nombres reales cuando haces clic en la fila **Réplica ZFS**. Quita el clon con `zfs destroy` cuando termines.
- **Con un montaje**, activando **Montar solo lectura en el servidor** en la página del servidor ZFS. Tras la siguiente ejecución las copias están montadas y en solo lectura en el segundo equipo.

Las versiones más antiguas de lo que conserva la réplica siguen saliendo de las copias de seguridad, como muestra el siguiente ejemplo.

### Ejemplo: un archivo de una copia ZFS {#example-zfs-file}

1. Abre **Restauración**, elige el mosaico **ZFS** y haz clic en `cache/appdata`.
2. Elige la copia y, en **Conjunto de datos**, el conjunto de datos que contiene el archivo.
3. En **Qué**, elige **Archivos individuales** y marca el archivo.
4. En **Dónde**, elige **En una carpeta** y escoge una carpeta bajo `/mnt`, como `user/restore`. Esto no necesita enlace SSH y también funciona para un conjunto de datos que ya no existe.
5. Pulsa **Restaurar**. El archivo acaba directamente en esa carpeta, sin la ruta de encima.

## Instantáneas sobrantes {#leftover-snapshots}

La instantánea de una copia se llama `<dataset>@bombvault-<14 digits>`, por ejemplo `cache/appdata@bombvault-20260924021500` (UTC). BombVault la elimina justo después de la copia. Si eso falla, por ejemplo porque el conjunto de datos está ocupado o BombVault se detuvo, BombVault la elimina:

- antes de la siguiente copia de ese elemento,
- cuando BombVault arranca, para cada elemento, también con la sección desactivada,
- cuando eliminas el elemento,
- cuando pulsas **Quitar ahora** en el elemento, que muestra cuántas quedan.

Solo se eliminan los nombres que coinciden exactamente con `bombvault-` más 14 dígitos. Las instantáneas de seguridad, tus propias instantáneas y las instantáneas automáticas nunca se tocan. Para quitar una a mano:

```
zfs destroy -r cache/appdata@bombvault-20260924021500
```

La instantánea de una réplica se llama `<dataset>@bombvault-replica-<14 digits>` y no es una sobrante. Se queda en el origen hasta que una ejecución de réplica más nueva la sustituye, o mientras **Conservar en este servidor** la conserve, y en el destino mientras la retención la conserve. El limpiador nunca la toca, porque solo coincide con `bombvault-` seguido de exactamente 14 dígitos.

## Anomalías {#anomalies}

Un hijo que se vació apenas cambia el total de un árbol grande, así que la detección de anomalías vigila cada conjunto de datos de un elemento por separado: su tamaño, número de archivos, datos nuevos y tiempo de restic tienen cada uno su propio historial. Ese historial pertenece al nombre del conjunto de datos, así que se mantiene cuando el árbol se copia más tarde con otro elemento.

Un conjunto de datos que la ejecución anterior copió y esta no pudo leer cuenta como vaciado, mientras la selección del elemento no haya cambiado. Eso cubre una clave que no está cargada, un conjunto de datos que no está montado y uno que ha desaparecido del árbol. Un hijo que excluyes tú mismo cambia la selección, así que su historial empieza de nuevo. Mientras un hallazgo sobre datos perdidos está abierto, la retención conserva las copias antiguas de ese conjunto de datos y poda el resto del árbol como de costumbre.

Cada conjunto de datos tiene una línea propia en la supervisión del elemento, que se abre con **Supervisión** en la página **Anomalías** y está en la pestaña **Verificación** del elemento. Los hallazgos abiertos aparecen junto a cada conjunto de datos. El enlace de un hallazgo abre la ventana de restauración en la última copia buena del conjunto de datos. Que una ejecución termine bien se juzga para todo el elemento, porque una ejecución sale bien o falla en conjunto.

Las comprobaciones en sí se describen en [Anomalías](anomalies.md). Un asistente conectado mediante el [servidor MCP](mcp.md) puede listar los puntos de restauración de un elemento ZFS, iniciar su copia y leer los hallazgos, pero confirmar uno se hace en la página **Anomalías**.

## Códigos de motivo {#reason-codes}

La página del elemento, el historial de ejecuciones y las notificaciones nombran un problema con uno de estos códigos. La mayoría tienen también la solución al lado en la página.

| Código | Significado | Qué hacer |
|---|---|---|
| `ssh-missing` | La conexión SSH no está configurada en este contenedor. | Configura el enlace SSH como para las copias de VMs. |
| `host-placeholder` | SSH del host: Dirección sigue siendo el marcador, y `host.docker.internal` tampoco respondió. | Pon SSH del host: Dirección en la IP de LAN de este servidor. |
| `host-fallback` | SSH del host: Dirección sigue siendo el marcador, y `host.docker.internal` funciona. | Nada, o pon la IP de LAN. |
| `ssh-unreachable` | No se puede llegar al servidor por SSH. | Comprueba la dirección y el puerto, y que SSH esté activado. |
| `ssh-auth` | El servidor rechazó la clave de BombVault. | Ejecuta una vez en el servidor el comando que se muestra en **Conexión con este servidor**. |
| `zfs-not-found` | El host SSH no tiene el comando `zfs`. | Apunta SSH del host: Dirección a la máquina que posee los pools. |
| `zfs-permission` | El usuario SSH no puede ejecutar este comando zfs. | Usa root, o consulta [TrueNAS SCALE](#truenas). |
| `uri-mismatch` | `LIBVIRT_URI` nombra un host o usuario distinto de los campos SSH. | Haz que coincidan, o borra los campos SSH para que ambos salgan de la URI. |
| `zfs-error` | zfs informó de otro error. | Los detalles muestran su mensaje. |
| `propagation-missing` | Los montajes nuevos del host no llegan al contenedor. | Pon el Access Mode de Host Data en Read/Write - Slave y reinicia BombVault. |
| `invalid-name` | Un nombre de conjunto de datos que BombVault no acepta. | Renombra el conjunto de datos. |
| `name-too-long` | Un conjunto de datos del árbol es demasiado largo para el nombre de una instantánea. | Renómbralo, o añade como elemento un conjunto de datos que esté debajo. |
| `invalid-exclude` | Un patrón de exclusión o un hijo dejado fuera no encaja con el elemento. | Arregla el elemento que nombra el mensaje. Para dejar fuera un conjunto de datos hijo entero, desactívalo en lugar de escribir un patrón. |
| `not-found` | El conjunto de datos no existe en el servidor. | Quita el elemento, o vuelve a crear el conjunto de datos. Sus copias siguen siendo restaurables. |
| `not-filesystem` | Es un volumen, no un sistema de archivos. | Consulta [Volúmenes](#volumes). |
| `overlaps-item` | El conjunto de datos se solapa con un elemento existente. | Consulta [Los elementos nunca se solapan](#overlap). |
| `docker-storage` | El árbol contiene el almacenamiento de imágenes de Docker. | Consulta [El almacenamiento de Docker](#docker-storage). |
| `nothing-readable` | Ahora mismo no se puede leer ningún conjunto de datos del elemento. | Mira los códigos de los conjuntos de datos saltados. |
| `snapshot-failed` | No se pudo crear la instantánea. | Los detalles muestran el mensaje de zfs. |
| `containers-busy` | Una copia de contenedor seguía en marcha cuando había que detener los contenedores. | Vuelve a empezar más tarde. Las ejecuciones programadas esperan por sí solas. |
| `consistency-stop-failed` | Un contenedor no se pudo detener, así que no se tomó ninguna instantánea. | Revisa el contenedor, o quítalo de la lista. |
| `pre-snapshot-failed` | Falló el comando anterior a la instantánea. | Los detalles de la ejecución muestran su salida. |
| `container-unknown` | Un contenedor de la lista no existe. | Quítalo de la lista. |
| `container-is-self` | BombVault no puede detener su propio contenedor. | Quítalo de la lista. |
| `leftover-snapshots` | Quedan en el servidor instantáneas que BombVault no pudo eliminar. | Pulsa **Quitar ahora**, consulta [Instantáneas sobrantes](#leftover-snapshots). |
| `zvol` | Un volumen en el árbol, saltado. | Consulta [Volúmenes](#volumes). |
| `canmount-off` | Nunca montado (`canmount=off`), saltado. | Si contiene datos, móntalo o mueve los datos a un conjunto de datos hijo. |
| `legacy-mount` | Punto de montaje legacy, saltado. | Dale un punto de montaje bajo `/mnt`. |
| `no-mountpoint` | Sin punto de montaje, saltado. | Dale un punto de montaje bajo `/mnt`. |
| `not-mounted` | No montado en el servidor, saltado. | Móntalo con `zfs mount`, o pon `canmount=on`. |
| `key-not-loaded` | Cifrado y la clave no está cargada, saltado. | `zfs load-key`, y luego móntalo. |
| `snapdir-disabled` | El acceso a instantáneas está desactivado, saltado. | `zfs set snapdir=hidden <dataset>`. La carpeta `.zfs` sigue oculta. |
| `not-visible` | BombVault no puede ver el punto de montaje del conjunto de datos. | Mueve el punto de montaje bajo la ruta de Host Data, o asígnalo al contenedor en la misma ruta con Read/Write - Slave. |
| `shfs-only` | El conjunto de datos solo es visible a través de `/mnt/user`, que oculta las instantáneas. | Asigna `/mnt`, no `/mnt/user`, como Host Data. |
| `snapshot-not-visible` | La instantánea se creó pero no apareció dentro de BombVault. | Ejecuta **Probar el acceso a instantáneas**. Mira más abajo. |
| `snapshot-loop` | La instantánea no llegó a BombVault porque Host Data no deja pasar los montajes nuevos. | Pon el Access Mode de Host Data en Read/Write - Slave y reinicia BombVault. |
| `backup-failed` | restic falló para este conjunto de datos. | Los detalles de la ejecución muestran por qué. |
| `not-reached` | La ejecución terminó antes de este conjunto de datos. | Vuelve a ejecutar la copia. |
| `gone` | El conjunto de datos ya no está en el servidor. | Nada. Sus copias siguen siendo restaurables. |
| `read-only-mount` | BombVault solo puede leer el conjunto de datos, así que no puede restaurar en él. | Pon la asignación en Read/Write - Slave, o restaura en una carpeta. |
| `destination-not-mounted` | La carpeta no está en un pool o recurso compartido montado. | Elige una carpeta en un pool o recurso compartido. |
| `not-enough-space` | No hay espacio libre suficiente en el destino. | Libera espacio o elige otra carpeta. |
| `safety-snapshot-failed` | No se pudo tomar la instantánea de seguridad, así que no se restauró nada. | Los detalles muestran el mensaje de zfs. |
| `safety-name-too-long` | El nombre del conjunto de datos es demasiado largo para una instantánea de seguridad. | Desactiva la instantánea de seguridad, o restaura en una carpeta. |
| `dataset-exists` | Ya existe un conjunto de datos con este nombre. | Elige un nombre nuevo, o restaura dentro del propio conjunto de datos. |
| `create-failed` | No se pudo crear el conjunto de datos nuevo. | Los detalles muestran el mensaje de zfs. Comprueba que el conjunto de datos que está por encima existe. |
| `new-dataset-not-visible` | El conjunto de datos nuevo se creó, pero BombVault no puede verlo, así que no se restauró nada. | El conjunto de datos se queda en el servidor. Móntalo bajo la ruta de Host Data y restaura en él. |
| `set-properties-failed` | No se pudieron fijar las propiedades guardadas, así que no se restauró nada. | Los detalles muestran el mensaje de zfs. |
| `set-limits-failed` | Los archivos se restauraron, pero no se pudo fijar la cuota o reserva guardada. | Los detalles muestran el mensaje de zfs. Fija tú la cuota o la reserva con `zfs set`. |

### Comprobar lo que ve el contenedor {#mountinfo}

**Probar el acceso a instantáneas** en la pestaña Ajustes del elemento toma una instantánea real de su árbol, la busca dentro de BombVault para cada conjunto de datos y la vuelve a eliminar. Es la forma más rápida de demostrar todo el camino antes de la primera ejecución programada.

Para mirar tú mismo, ejecuta esto en el servidor:

```
docker exec BombVault grep zfs /proc/self/mountinfo
```

Cada línea es un montaje dentro del contenedor. La línea de un conjunto de datos muestra su ruta dentro del contenedor (bajo `/host/user`) y el nombre del conjunto de datos. Un campo `master:N` en esa línea significa que el montaje recibe los montajes que el host hace más tarde, que es lo que necesita el acceso a instantáneas. Cuando falta, pon el Access Mode de Host Data en Read/Write - Slave y reinicia BombVault.

## TrueNAS SCALE {#truenas}

- Cuando `LIBVIRT_URI` está definida (como para las copias de VMs en TrueNAS), BombVault toma el host SSH, el usuario y el puerto para sus comandos zfs de la URI, cada uno de los que no esté definido por sí solo. Sin copias de VMs no hay que definir nada: `host.docker.internal` no se resuelve en una app de TrueNAS, así que BombVault llega al host por la puerta de enlace predeterminada de la app, como root en el puerto 22. Define `LIBVIRT_HOST`, `LIBVIRT_SSH_USER` y `LIBVIRT_SSH_PORT` cuando quieras otra dirección o usuario, en el `environment` de la app.
- Añade la clave SSH del host de **Ajustes, Integraciones, SSH del host** a root en **Credentials, Users**, y activa el servicio SSH en **System, Services**. Una [réplica](#replica-push) a este u otro TrueNAS necesita además la clave de la réplica, que es distinta.
- Un usuario que no sea root necesita permiso en el conjunto de datos superior del elemento, que entonces cubre todos los que tiene debajo:

  ```
  zfs allow <user> snapshot,destroy,mount <dataset>
  ```

  Restaurar en un conjunto de datos nuevo necesita además `create` en el conjunto de datos que está por encima, y fijar propiedades guardadas necesita permiso para esas propiedades.

  Una sesión SSH que no sea de root en TrueNAS no tiene `/usr/sbin` en su ruta. BombVault llama entonces directamente a `/usr/sbin/zfs`.
- El **Host Data** de la app debe ser una ruta del host por encima de los conjuntos de datos, por ejemplo `/mnt/tank`, no un ixVolume. Con una ruta del host la app pasa a BombVault los montajes nuevos del host (`rslave`), que necesita el acceso a instantáneas. Consulta [TrueNAS SCALE](getting-started.md#truenas) para la configuración completa de la app.
