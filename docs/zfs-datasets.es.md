# Conjuntos de datos ZFS

La página **ZFS** hace copias de seguridad de conjuntos de datos ZFS. Un elemento es un conjunto de datos junto con todos los conjuntos de datos que tiene debajo. Para cada copia, BombVault toma una sola instantánea ZFS de todo el árbol, así que cada conjunto de datos que contiene queda capturado en el mismo instante. Después lee los archivos de cada conjunto de datos desde esa instantánea, los guarda con restic igual que guarda una carpeta y elimina la instantánea justo después. Las copias están deduplicadas, puedes explorar cada una de ellas y se pueden restaurar archivos sueltos.

BombVault nunca usa `zfs send` para conjuntos de datos, nunca revierte un conjunto de datos y nunca destruye ninguno.

## Requisitos {#requirements}

- **La conexión SSH con este servidor.** Los conjuntos de datos ZFS usan la misma clave, el mismo host y el mismo usuario que las copias de VM. Si las copias de VM ya funcionan, esto también. Si no, sigue la [guía de copia de VM por SSH](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) en GitHub. Los campos de la plantilla se llaman **Host SSH: Address**, **Host SSH: Port** y **Host SSH: User**.
- **El comando `zfs` en ese host.** Unraid 6.12 o posterior y TrueNAS SCALE lo tienen.
- **Host Data mapeado como `/mnt` con Access Mode Read/Write - Slave.** Es el valor por defecto de la plantilla. La instantánea de un conjunto de datos aparece dentro de su carpeta `.zfs/snapshot` solo después de que BombVault haya arrancado, así que el contenedor tiene que recibir los montajes que el host hace más tarde.
- **Los conjuntos de datos montados bajo `/mnt`.** En Unraid los pools están en `/mnt/<pool>`, así que eso ya se cumple.

Activa el dominio en **Ajustes, General** (Conjuntos de datos ZFS). La página ZFS muestra entonces una tarjeta **Conexión con este servidor**. Prueba la conexión SSH, indica el usuario y el host con los que se conecta y dice qué falta cuando falta algo. La comprobación de integración con el host (`/spike`) muestra el mismo resultado.

## Elementos y conjuntos de datos hijos {#items-and-children}

Abre **Añadir conjuntos de datos** en la página ZFS. La lista viene del servidor. Elige el conjunto de datos que está más arriba de lo que quieres copiar, por ejemplo `cache/appdata`, y el elemento lo cubre a él y a todos los conjuntos de datos que tiene debajo.

- **Los conjuntos de datos hijos nuevos se añaden solos.** Un conjunto de datos creado más tarde bajo el elemento se copia en la siguiente ejecución, que lo señala como nuevo. Su primera copia lo lee entero una vez; después solo se leen los cambios.
- **Puedes dejar fuera hijos sueltos.** Desactiva un hijo en los ajustes del elemento y queda fuera junto con todo lo que tiene debajo. Un hijo excluido que ya no existe en el servidor se marca como tal y se puede quitar de la lista.
- **Los hijos que no se pueden leer se omiten, nunca en silencio.** La ejecución los enumera, el elemento muestra cuántos se omitieron y la tarjeta de cobertura del panel cuenta cada uno como no protegido. La ejecución copia igualmente todo lo demás y no falla por un hijo omitido. Los motivos están en la [tabla de códigos de motivo](#reason-codes): un conjunto de datos sin montar, con `canmount=off`, con un punto de montaje `legacy` o sin él, una clave de cifrado sin cargar, el acceso a instantáneas desactivado o un punto de montaje que BombVault no ve.
- **Un conjunto de datos omitido no arrastra a sus hijos.** Un conjunto de datos con `canmount=off` que solo contiene otros conjuntos de datos se omite (se muestra como "solo estructura") y sus hijos montados se copian. Un conjunto de datos cifrado cuya clave no está cargada se omite junto con los hijos que comparten su clave.
- **Los hijos que son discos de VM o datos del sistema empiezan desactivados** en el diálogo de añadir, con el motivo junto al interruptor. Añadir un pool entero pide una confirmación que enumera lo que contiene.

### Volúmenes {#volumes}

Un volumen (zvol) contiene un disco virtual en lugar de archivos, y la página ZFS nunca lo copia.

- Un volumen que usa una VM se copia con esa VM en la página **VMs**.
- Un volumen que no usa ninguna VM (un extent iSCSI, un disco que desconectaste) **no lo copia BombVault**. El diálogo de añadir y la página ZFS cuentan estos volúmenes y lo dicen. Una versión posterior los copiará.

Los volúmenes dentro del árbol de un elemento se omiten y se nombran en cada ejecución.

### El almacenamiento de Docker {#docker-storage}

Con el controlador de almacenamiento ZFS de Docker, cada capa de imagen es un conjunto de datos con un punto de montaje `legacy`. El diálogo de añadir los agrupa en una línea por padre. Un árbol que contiene más de 20 de estos conjuntos de datos no puede convertirse en elemento: mientras exista una instantánea de él, Docker no puede eliminar capas de imagen. Añade en su lugar los conjuntos de datos que hay debajo, por ejemplo `appdata`.

### Los elementos nunca se solapan {#overlap}

Un conjunto de datos solo puede pertenecer a un elemento. BombVault rechaza un elemento nuevo que esté dentro de uno existente o que contendría uno. Para unir varios elementos hijos en un elemento padre, elimina primero los elementos hijos eligiendo conservar sus copias y luego añade el padre. Cada conjunto de datos conserva su historial con su propio nombre, así que la siguiente copia sigue donde lo dejaron los elementos antiguos y no vuelve a leerlo todo.

## Detener contenedores y ejecutar comandos alrededor de la instantánea {#consistency}

Una instantánea de una base de datos en marcha es como un corte de luz repentino: la base de datos normalmente se recupera, pero tiene que hacerlo. Cada elemento puede hacer dos cosas al respecto, y ambas cubren solo el instante de la instantánea, no toda la copia.

- **Detener estos contenedores para la instantánea.** BombVault detiene los contenedores de la lista, toma la instantánea y los vuelve a arrancar enseguida. Los contenedores de un mismo nivel de dependencias se detienen en paralelo, primero los dependientes, así que toda la ventana suele durar unos segundos; la ejecución muestra cuánto duró. Después la copia lee la instantánea congelada mientras las aplicaciones ya vuelven a funcionar. Solo se detienen los contenedores que estaban en marcha.
- **Un comando antes y después de la instantánea.** Se ejecuta dentro del contenedor que elijas, por ejemplo para volcar una base de datos en el conjunto de datos justo antes de la instantánea, sin detener nada. Si el comando de antes de la instantánea falla, la copia falla y no se toma ninguna instantánea. Un comando de después de la instantánea que falla se muestra en la ejecución pero no hace fallar la copia.

Qué pasa cuando algo sale mal:

- Si un contenedor no se puede detener, BombVault arranca los que ya había detenido y la copia falla indicando el contenedor. Nunca recurre a una instantánea de aplicaciones en marcha.
- La detención espera a que termine una copia de contenedor en curso (hasta 30 minutos en una ejecución manual, hasta el límite de tiempo de la copia en una programada), para que las dos nunca detengan y arranquen el mismo contenedor a la vez.
- Antes de que se detenga el primer contenedor, BombVault anota cuáles detiene. Si BombVault muere dentro de la ventana, vuelve a arrancar esos contenedores la próxima vez que arranca, envía una notificación y el elemento muestra una nota roja por cada contenedor que no pudo arrancar.

Los volcados automáticos de bases de datos (ver [Funciones](features.md)) se ejecutan con la copia propia de un contenedor en la página **Contenedores**, no con un elemento ZFS. Una base de datos cuyo contenedor solo se copia a través de su conjunto de datos no recibe volcado, así que dale un comando aquí.

Un contenedor puede estar en esta lista y en la página **Contenedores** a la vez. Entonces sus datos se guardan dos veces, en dos repositorios, y la **Copia total** lo detiene dos veces. El elemento lo avisa.

## Restaurar {#restore}

Abre **Copias de seguridad** en el elemento, elige la copia y después el conjunto de datos. Por defecto es el conjunto de datos superior del elemento.

- **Restaurar dentro del conjunto de datos.** Los archivos de la copia se escriben en el punto de montaje del conjunto de datos. Los archivos con el mismo nombre se sobrescriben, los demás se quedan. El conjunto de datos nunca se revierte ni se reemplaza. BombVault comprueba que el conjunto de datos está montado, visible y escribible, una vez antes de empezar y otra justo antes de escribir. Donde hay un conjunto de datos hijo montado dentro, no se escribe nada: el hijo conserva sus archivos, su propietario y sus permisos, y se restaura desde su propia copia.
- **Restaurar en una carpeta.** Elige una carpeta bajo `/mnt`. BombVault comprueba que la carpeta está en un pool o recurso compartido montado y que hay espacio libre suficiente. Funciona sin la conexión SSH y con conjuntos de datos que ya no existen.
- **Elegir archivos** (avanzado): escribir de vuelta en el conjunto de datos solo los archivos y carpetas que elijas.
- **Todos los conjuntos de datos de esta copia** (avanzado): cada conjunto de datos del árbol en su propia subcarpeta de la carpeta que elijas. Se nombran los conjuntos de datos que se omitieron en esa copia.
- **Desde otro servidor:** la página **Recuperación** restaura desde el repositorio de otro BombVault, siempre en una carpeta: todos los conjuntos de datos de una copia, cada uno en su propia subcarpeta, o un conjunto de datos del árbol, entero o archivos elegidos.

La lista de contenedores a detener del elemento también se ofrece para una restauración dentro del conjunto de datos. Esos contenedores siguen detenidos durante toda la restauración, y mientras tanto las copias de contenedores esperan.

### La instantánea de seguridad {#safety-snapshot}

Antes de escribir en un conjunto de datos, BombVault toma una instantánea ZFS de ese único conjunto de datos, llamada `bombvault-prerestore-<hora>`. Está activada por defecto; desactivarla requiere una segunda confirmación. Si no se puede tomar la instantánea, no se restaura nada.

BombVault nunca borra por sí solo una instantánea de seguridad. El elemento las enumera con su antigüedad y tamaño, cada una con la acción **Eliminar**, y avisa cuando la más antigua tiene más de 30 días, porque retiene en el pool datos borrados y modificados.

Para volver atrás tras una restauración, copia archivos sueltos desde `.zfs/snapshot/bombvault-prerestore-<hora>` dentro del conjunto de datos. `zfs rollback <dataset>@bombvault-prerestore-<hora>` solo funciona mientras sea la instantánea más reciente de ese conjunto de datos. `zfs rollback -r` borra todas las instantáneas más recientes, incluidas las automáticas.

### Restaurar como conjunto de datos nuevo {#new-dataset}

BombVault no crea conjuntos de datos. Créalo en el servidor con las propiedades que quieras y luego restaura en una carpeta que sea su punto de montaje:

```
zfs create -o compression=lz4 cache/appdata-restored
```

y en BombVault restaura en la carpeta `cache/appdata-restored` bajo `/mnt`.

## Qué contiene la copia {#contents}

En la copia: los archivos y carpetas de cada conjunto de datos copiado, con su propietario, permisos, marcas de tiempo y atributos extendidos tal como los guarda restic.

No está en la copia:

- las propiedades ZFS de los conjuntos de datos (compression, recordsize, quota, mountpoint y las demás);
- el propietario y los permisos de la carpeta superior de cada conjunto de datos (todo lo que hay debajo sí se incluye). Una restauración dentro del conjunto de datos deja la carpeta superior existente como está; una restauración en una carpeta la crea con permisos `0755`;
- las instantáneas ZFS existentes;
- los hijos que se omitieron o se dejaron fuera;
- los volúmenes.

Para restaurar en un pool nuevo, crea primero los conjuntos de datos con las propiedades que quieras. Aún no se ha comprobado si las ACL NFSv4, tal como las usa TrueNAS en conjuntos de datos SMB, vuelven como esperas, así que prueba una restauración con tus propios datos antes de confiar en ellas.

## Conjuntos de datos cifrados {#encryption}

Un conjunto de datos cifrado solo se copia mientras su clave está cargada. Si no, se omite con un aviso; carga la clave con `zfs load-key` y monta el conjunto de datos. BombVault lee los datos descifrados y los guarda en el repositorio de restic, que está cifrado. Si desactivaste el cifrado en BombVault, ese repositorio no lo está.

## Instantáneas sobrantes {#leftover-snapshots}

La instantánea de una copia se llama `<dataset>@bombvault-<14 dígitos>`, por ejemplo `cache/appdata@bombvault-20260924021500` (UTC). BombVault la elimina justo después de la copia. Si eso falla, por ejemplo porque el conjunto de datos está ocupado o BombVault se detuvo, BombVault la elimina:

- antes de la siguiente copia de ese elemento,
- cuando BombVault arranca, para cada elemento, también con el dominio desactivado,
- cuando eliminas el elemento,
- cuando pulsas **Quitar ahora** en el elemento, que muestra cuántas quedan.

Solo se eliminan los nombres que coinciden exactamente con `bombvault-` más 14 dígitos. Las instantáneas de seguridad, tus propias instantáneas y las automáticas nunca se tocan. Para eliminar una a mano:

```
zfs destroy -r cache/appdata@bombvault-20260924021500
```

## Anomalías {#anomalies}

Un hijo que se vació apenas cambia el total de un árbol grande, así que la detección de anomalías vigila cada conjunto de datos de un elemento por separado: su tamaño, su número de archivos, sus datos nuevos y su tiempo de restic tienen cada uno su propio historial. Ese historial pertenece al nombre del conjunto de datos, así que se conserva cuando más tarde otro elemento copia el árbol.

Un conjunto de datos que la ejecución anterior copió y que esta no pudo leer cuenta como vaciado, siempre que la selección del elemento no haya cambiado. Eso cubre una clave sin cargar, un conjunto de datos sin montar y uno que ha desaparecido del árbol. Un hijo que excluyes tú mismo cambia la selección, así que su historial empieza de cero. Mientras un hallazgo sobre datos perdidos está abierto, la retención conserva las copias antiguas de ese único conjunto de datos y poda el resto del árbol como siempre.

En la pestaña **Elementos** de la página **Anomalías**, cada conjunto de datos tiene su propia línea bajo su elemento, y el árbol del elemento en esta página muestra los hallazgos abiertos junto a cada conjunto de datos. El enlace de un hallazgo abre el panel de restauración del elemento en la última copia buena del conjunto de datos. Si una ejecución termina se juzga para el elemento entero, porque una ejecución tiene éxito o falla en conjunto.

Las comprobaciones en sí se describen en [Funciones](features.md). Un asistente conectado a través del [servidor MCP](mcp.md) puede enumerar los puntos de restauración de un elemento ZFS, iniciar su copia y leer los hallazgos, pero reconocer uno se hace en la página **Anomalías**.

## Códigos de motivo {#reason-codes}

La página, el historial de ejecuciones y las notificaciones nombran un problema con uno de estos códigos. La mayoría tienen también la solución al lado en la página.

| Código | Significado | Qué hacer |
|---|---|---|
| `ssh-missing` | La conexión SSH no está configurada en este contenedor. | Configura la conexión SSH como para las copias de VM. |
| `host-placeholder` | Host SSH: Address sigue siendo el valor de ejemplo, y `host.docker.internal` tampoco respondió. | Pon en Host SSH: Address la IP LAN de este servidor. |
| `host-fallback` | Host SSH: Address sigue siendo el valor de ejemplo, y `host.docker.internal` funciona. | Nada, o pon la IP LAN. |
| `ssh-unreachable` | No se llega al servidor por SSH. | Revisa la dirección y el puerto, y que SSH esté activado. |
| `ssh-auth` | El servidor rechazó la clave de BombVault. | Ejecuta una vez en el servidor el comando que muestra la tarjeta de conexión. |
| `zfs-not-found` | El host SSH no tiene el comando `zfs`. | Apunta Host SSH: Address a la máquina dueña de los pools. |
| `zfs-permission` | El usuario SSH no puede ejecutar este comando zfs. | Usa root, o ver [TrueNAS SCALE](#truenas). |
| `uri-mismatch` | `LIBVIRT_URI` indica otro host u otro usuario que los campos SSH. | Haz que coincidan, o vacía los campos SSH para que ambos vengan de la URI. |
| `zfs-error` | zfs informó de otro error. | Los detalles muestran su mensaje. |
| `propagation-missing` | Los montajes nuevos del host no llegan al contenedor. | Pon el Access Mode de Host Data en Read/Write - Slave y reinicia BombVault. |
| `invalid-name` | Un nombre de conjunto de datos que BombVault no acepta. | Renombra el conjunto de datos. |
| `name-too-long` | Un conjunto de datos del árbol es demasiado largo para un nombre de instantánea. | Renómbralo, o añade como elemento un conjunto de datos de debajo. |
| `invalid-exclude` | Un patrón de exclusión o un hijo excluido no encaja con el elemento. | Corrige la entrada que nombra el mensaje. Para dejar fuera un conjunto de datos hijo entero, desactívalo en lugar de escribir un patrón. |
| `not-found` | El conjunto de datos no existe en el servidor. | Quita el elemento o vuelve a crear el conjunto de datos. Sus copias siguen siendo restaurables. |
| `not-filesystem` | Es un volumen, no un sistema de archivos. | Ver [Volúmenes](#volumes). |
| `overlaps-item` | El conjunto de datos se solapa con un elemento existente. | Ver [Los elementos nunca se solapan](#overlap). |
| `docker-storage` | El árbol contiene el almacenamiento de imágenes de Docker. | Ver [El almacenamiento de Docker](#docker-storage). |
| `nothing-readable` | Ahora mismo no se puede leer ningún conjunto de datos del elemento. | Mira los códigos de los conjuntos de datos omitidos. |
| `snapshot-failed` | No se pudo crear la instantánea. | Los detalles muestran el mensaje de zfs. |
| `containers-busy` | Una copia de contenedor seguía en marcha cuando los contenedores tenían que detenerse. | Vuelve a empezar más tarde. Las ejecuciones programadas esperan solas. |
| `consistency-stop-failed` | Un contenedor no se pudo detener, así que no se tomó ninguna instantánea. | Revisa el contenedor o quítalo de la lista. |
| `pre-snapshot-failed` | El comando de antes de la instantánea falló. | Los detalles de la ejecución muestran su salida. |
| `container-unknown` | Un contenedor de la lista no existe. | Quítalo de la lista. |
| `container-is-self` | BombVault no puede detener su propio contenedor. | Quítalo de la lista. |
| `leftover-snapshots` | Siguen en el servidor instantáneas que BombVault no pudo eliminar. | Pulsa **Quitar ahora**, ver [Instantáneas sobrantes](#leftover-snapshots). |
| `zvol` | Un volumen en el árbol, omitido. | Ver [Volúmenes](#volumes). |
| `canmount-off` | Nunca montado (`canmount=off`), omitido. | Si contiene datos, móntalo o mueve los datos a un conjunto de datos hijo. |
| `legacy-mount` | Punto de montaje legacy, omitido. | Dale un punto de montaje bajo `/mnt`. |
| `no-mountpoint` | Sin punto de montaje, omitido. | Dale un punto de montaje bajo `/mnt`. |
| `not-mounted` | No montado en el servidor, omitido. | Móntalo con `zfs mount`, o pon `canmount=on`. |
| `key-not-loaded` | Cifrado y la clave no está cargada, omitido. | `zfs load-key` y luego móntalo. |
| `snapdir-disabled` | El acceso a instantáneas está desactivado, omitido. | `zfs set snapdir=hidden <dataset>`. La carpeta `.zfs` sigue oculta. |
| `not-visible` | BombVault no ve el punto de montaje del conjunto de datos. | Mueve el punto de montaje bajo la ruta de Host Data, o mapéalo en el contenedor en la misma ruta con Read/Write - Slave. |
| `shfs-only` | El conjunto de datos solo es visible a través de `/mnt/user`, que oculta las instantáneas. | Mapea `/mnt`, no `/mnt/user`, como Host Data. |
| `snapshot-not-visible` | La instantánea se creó pero no apareció dentro de BombVault. | Ejecuta **Probar el acceso a instantáneas**; ver más abajo. |
| `snapshot-loop` | La instantánea no llegó a BombVault porque Host Data no deja pasar montajes nuevos. | Pon el Access Mode de Host Data en Read/Write - Slave y reinicia BombVault. |
| `backup-failed` | restic falló para este conjunto de datos. | Los detalles de la ejecución muestran por qué. |
| `not-reached` | La ejecución terminó antes de este conjunto de datos. | Vuelve a ejecutar la copia. |
| `gone` | El conjunto de datos ya no está en el servidor. | Nada. Sus copias siguen siendo restaurables. |
| `read-only-mount` | BombVault solo puede leer el conjunto de datos, así que no puede restaurar dentro de él. | Pon el mapeo en Read/Write - Slave, o restaura en una carpeta. |
| `destination-not-mounted` | La carpeta no está en un pool o recurso compartido montado. | Elige una carpeta en un pool o recurso compartido. |
| `not-enough-space` | No hay espacio libre suficiente en el destino. | Libera espacio o elige otra carpeta. |
| `safety-snapshot-failed` | No se pudo tomar la instantánea de seguridad, así que no se restauró nada. | Los detalles muestran el mensaje de zfs. |
| `safety-name-too-long` | El nombre del conjunto de datos es demasiado largo para una instantánea de seguridad. | Desactiva la instantánea de seguridad, o restaura en una carpeta. |

### Comprobar lo que ve el contenedor {#mountinfo}

**Probar el acceso a instantáneas** en un elemento toma una instantánea real de su árbol, la busca dentro de BombVault para cada conjunto de datos y la vuelve a eliminar. Es la forma más rápida de comprobar todo el camino antes de la primera ejecución programada.

Para mirarlo tú mismo, ejecuta esto en el servidor:

```
docker exec BombVault grep zfs /proc/self/mountinfo
```

Cada línea es un montaje dentro del contenedor. La línea de un conjunto de datos muestra su ruta dentro del contenedor (bajo `/host/user`) y el nombre del conjunto de datos. Un campo `master:N` en esa línea significa que el montaje recibe los montajes que el host hace más tarde, que es lo que necesita el acceso a instantáneas. Si falta, pon el Access Mode de Host Data en Read/Write - Slave y reinicia BombVault.

## TrueNAS SCALE {#truenas}

- Cuando `LIBVIRT_URI` está definida (como para las copias de VM en TrueNAS), BombVault toma de la URI el host, el usuario y el puerto SSH de sus comandos zfs, cada uno que no esté definido por separado. Sin copias de VM, define en su lugar `LIBVIRT_HOST`, `LIBVIRT_SSH_USER` y `LIBVIRT_SSH_PORT`. Añade las variables en **Additional Environment Variables**.
- Un usuario que no sea root necesita permiso sobre el conjunto de datos superior del elemento, que entonces cubre todos los conjuntos de datos de debajo:

  ```
  zfs allow <user> snapshot,destroy,mount <dataset>
  ```

  Una sesión SSH sin root en TrueNAS no tiene `/usr/sbin` en su ruta; BombVault llama entonces a `/usr/sbin/zfs` directamente.
- El **Host Data** de la app tiene que ser una ruta del host por encima de los conjuntos de datos, por ejemplo `/mnt/tank`, no un ixVolume. Con una ruta del host, la app pasa a BombVault los montajes nuevos del host (`rslave`), que es lo que necesita el acceso a instantáneas.
