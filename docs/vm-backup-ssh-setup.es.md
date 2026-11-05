# Copias de VMs

BombVault copia VMs KVM/libvirt sin montar ninguna ruta de libvirt. Ejecuta `virsh` en el host por SSH (`qemu+ssh://`), lee y escribe los discos de las VMs a través del montaje Host Data (`/mnt` en el host es `/host/user` en el contenedor) y copia la NVRAM UEFI por SSH. Como no se monta por bind nada que pertenezca a libvirt, BombVault nunca puede estorbar al VM Manager de Unraid.

Esta guía cubre la configuración, los métodos de copia, las VMs con instantáneas de Unraid y TrueNAS SCALE. Las copias de contenedores no necesitan nada de esto.

Las copias de conjuntos de datos ZFS usan el mismo enlace SSH, con la misma clave, host y usuario. En cuanto las copias de VMs conectan, los elementos de conjuntos de datos ZFS también conectan. Consulta [Conjuntos de datos ZFS](zfs-datasets.md).

## Qué configuras {#what-you-configure}

| Ajuste | Dónde | Valor predeterminado | Significado |
|---|---|---|---|
| **SSH del host: Dirección** (`LIBVIRT_HOST`) | variable de la plantilla | `host.docker.internal` | El host Unraid, al que se llega por SSH |
| **SSH del host: Puerto** (`LIBVIRT_SSH_PORT`) | variable de la plantilla | `22` | El puerto SSH de Unraid |
| **SSH del host: Usuario** (`LIBVIRT_SSH_USER`) | variable de la plantilla | `root` | El usuario SSH en el host |
| Clave pública | **Ajustes, Integraciones, SSH del host** | se crea en el primer arranque | Autorízala en el host |

BombVault crea su par de claves SSH en el primer arranque en `/config/ssh/id_ed25519`, guardado en appdata. La clave del host se fija en `/config/ssh/known_hosts` en la primera conexión.

La plantilla rellena **SSH del host: Dirección** con el marcador `192.168.x.x`. BombVault lo trata como no definido e intenta `host.docker.internal`, que funciona en la red `bridge` predeterminada. En una red personalizada, sustitúyelo por la IP de LAN del host (paso 3 B).

## Paso 1: Activa SSH en el host Unraid {#step-1}

1. En Unraid, abre **Settings, Management Access** y pon **Use SSH** en **Yes**.
2. Si cambiaste el puerto SSH, anótalo. **SSH del host: Puerto** tiene que coincidir.

## Paso 2: Autoriza la clave pública de BombVault {#step-2}

El `/root` de Unraid vive en memoria y se borra en cada arranque, así que la clave tiene que estar también en la unidad flash. Ejecuta esto en el terminal de Unraid:

```sh
K=$(docker exec BombVault cat /config/ssh/id_ed25519.pub)
mkdir -p /root/.ssh /boot/config/ssh && chmod 700 /root/.ssh
grep -qxF "$K" /root/.ssh/authorized_keys 2>/dev/null  || echo "$K" >> /root/.ssh/authorized_keys
grep -qxF "$K" /boot/config/ssh/root.pubkeys 2>/dev/null || echo "$K" >> /boot/config/ssh/root.pubkeys
chmod 600 /root/.ssh/authorized_keys
```

Unraid 6.9 y posteriores copian `/boot/config/ssh/root.pubkeys` en el `authorized_keys` de root en cada arranque, así que esto sobrevive a los reinicios. Vale para cualquier inicio de sesión de `root`, sea cual sea el nombre de host o el puerto que use BombVault.

## Paso 3: Red {#step-3}

BombVault tiene que abrir una conexión TCP desde el contenedor al puerto SSH del host. Elige el caso que corresponda a tu configuración.

### A. BombVault en la red bridge predeterminada {#bridge}

- Deja **SSH del host: Dirección** en `host.docker.internal` o en el marcador de la plantilla. La plantilla añade `--add-host=host.docker.internal:host-gateway`, que apunta a la puerta de enlace de docker0, que es el host.
- Pon **SSH del host: Puerto** en tu puerto SSH.

### B. BombVault en una red personalizada {#custom-network}

Un contenedor en `br0.x`, macvlan o ipvlan no puede llegar al host mediante `host.docker.internal`, porque 172.17.0.1 es docker0 y no es accesible desde `br0.x`. En su lugar:

1. En Unraid, pon **Settings, Docker, Host access to custom networks** en **Enabled**.
2. Pon **SSH del host: Dirección** en la IP de LAN del host Unraid, la dirección con la que abres la interfaz web.
3. Si la red del contenedor y el host están en VLAN distintas, permite la ruta en tu router o cortafuegos: desde la VLAN del contenedor a la IP del host en el puerto SSH, por TCP.
4. Si aun así no se puede llegar a la IP de LAN del host, usa la IP shim del host en la subred del contenedor. Búscala en el host con `ip -4 addr show | grep -B2 <container-subnet>`.

### Comprueba que el host responde {#verify-reachability}

Ejecuta esto en el terminal de Unraid, con tu IP y tu puerto:

```sh
docker exec BombVault timeout 6 bash -c 'echo > /dev/tcp/192.168.1.10/22' && echo OPEN || echo UNREACHABLE
```

`OPEN` significa que el camino funciona. `UNREACHABLE` significa que hay que arreglar antes que nada la red del paso 3.

## Paso 4: Define las variables y prueba {#step-4}

1. En Unraid, abre **Docker**, haz clic en BombVault, elige **Edit**, pon **SSH del host: Dirección** (y **SSH del host: Puerto** si no es el 22) y pulsa **Apply**. Si faltan las variables, importa de nuevo la plantilla, porque Unraid conserva la configuración guardada de un contenedor existente.
2. En BombVault, abre **Ajustes, Integraciones, SSH del host** y pulsa **Probar conexión**. Debería ponerse en verde.

La misma llamada desde el terminal lista tus VMs:

```sh
docker exec BombVault virsh -c "qemu+ssh://root@192.168.1.10:22/system?keyfile=/config/ssh/id_ed25519&known_hosts=/config/ssh/known_hosts&known_hosts_verify=auto" list --all
```

## Paso 5: El método por VM {#per-vm-method}

Cada VM tiene un **Método** en la tarjeta **VM** de su pestaña Ajustes (abre la VM en **Copias de seguridad**):

- **Ordenado (apagado)** es el valor predeterminado. Apaga la VM, copia los discos y la vuelve a iniciar, así que la copia siempre es coherente. Espera hasta diez minutos a que el invitado se apague y nunca fuerza el apagado. Un invitado que sigue funcionando pasado ese tiempo se deja en paz, y esa copia falla con el motivo. Si el invitado se apaga dentro de la hora siguiente, BombVault lo vuelve a arrancar. Si ese arranque falla, la ejecución fallida lo indica.
- **Instantánea en vivo** copia una VM en ejecución mediante una instantánea externa y `blockcommit`. Necesita el disco en `/mnt/cache` o `/mnt/diskX`, no en `/mnt/user`. Con el agente invitado de qemu en la VM, la instantánea se hace con el sistema de archivos en reposo, y sin él es coherente ante caídas. En una VM apagada, Instantánea en vivo pasa a Ordenado por sí sola. Cuando no se puede hacer la instantánea, esa copia falla y la VM sigue en marcha, porque una VM que elegiste copiar en vivo nunca se apaga para ello.

## Lo que una copia deja fuera {#left-out}

Una copia puede salir bien y aun así no contener toda la VM. La ejecución lleva entonces una nota en tono de aviso que nombra lo que falta:

- **Discos que BombVault no lee.** Un disco de solo lectura, un `lun` y un disco `network` o `volume` se dejan fuera, y la nota nombra cada uno con su dispositivo de destino y su tipo. La definición de la copia sigue listándolos. Un cdrom y un disquete no se nombran. Un dispositivo de bloques normal que no es un zvol de ZFS hace fallar la copia en su lugar.
- **El estado de un TPM emulado.** No tiene ruta en el XML del dominio. El VM Manager de Unraid configura un TPM así para una VM con la BIOS **OVMF TPM**, así que la nota también aparece en Unraid. Un invitado que guarda un secreto en su TPM, como puede hacer BitLocker, puede pedir su clave de recuperación tras una restauración que pierde ese estado. Ten esa clave a mano.

## Discos finos {#thin-disks}

Una restauración mantiene fino un disco fino, tanto si reemplaza el disco como si escribe uno nuevo: restic escribe las series de ceros como huecos en un archivo nuevo, y BombVault libera los bloques de ceros de un archivo reemplazado con `fallocate --dig-holes` una vez escrito. Mientras restic reemplaza un archivo, ese archivo ocupa su tamaño completo, si el pool tiene sitio. Un sistema de archivos que no permite perforar huecos deja el disco con su tamaño completo, y el registro lo indica. La comprobación de espacio sigue contando un disco nuevo con su tamaño completo.

## Una restauración que falla {#restore-fails}

Una restauración in situ apaga la VM, escribe los discos y reemplaza la definición y la NVRAM solo cuando los discos están escritos. Una restauración que falla deja la VM definida con su NVRAM. Si sus discos no cambiaron y estaba en marcha, vuelve a funcionar. Si pueden haber cambiado, se queda apagada, porque arrancar con un disco a medio escribir escribiría encima, y la ejecución lo indica. Restaurar de nuevo la misma copia termina el trabajo.

Las instantáneas registradas en libvirt, que no es como las crea el VM Manager de Unraid, detienen una restauración in situ antes de tocar la VM, porque libvirt no puede eliminar una VM que las tenga. Bórralas primero.

## Solo bloques modificados {#changed-blocks}

Una VM con discos qcow2 se puede copiar leyendo solo lo que cambió. Activa **Solo bloques modificados** en la misma tarjeta, y BombVault usa la API de copias de libvirt en lugar del método de arriba:

- Inicia un trabajo de copia en modo pull (`virsh backup-begin`) que crea en el mismo instante un punto de control llamado `bombvault-<time>`, y lee los discos por el enlace SSH a través de un socket Unix reenviado. Cuando el agente invitado responde, los sistemas de archivos del invitado se congelan solo mientras arranca el trabajo. Una descongelación que falla se reintenta durante unos quince segundos, y si el invitado sigue sin descongelarse, BombVault envía una notificación.
- La primera ejecución lee todos los bloques asignados. Las siguientes leen solo los bloques que el mapa de bits del punto de control marca como escritos desde la copia anterior.
- Cada disco se guarda como una carpeta de segmentos de tamaño fijo que contienen lo que ve el invitado. Los segmentos sin cambios se toman de la instantánea anterior sin leerse, así que cada instantánea está completa y se restaura por sí sola.
- Una restauración reconstruye el disco a partir de sus segmentos y lo convierte de nuevo a qcow2 con `qemu-img`. El disco tiene el mismo contenido que en el momento de la copia, pero el archivo es un archivo qcow2 nuevo, y una cadena de respaldo vuelve aplanada.
- La comprobación previa a una restauración así lista cada disco que reconstruye y lo cuenta con el tamaño completo del invitado, dos veces para qcow2, ya que la imagen raw y su copia convertida conviven hasta que se sustituye el archivo antiguo. Es lo máximo que puede ocupar un disco, así que una falta de espacio según esta cuenta deja la línea de espacio en gris en lugar de bloquear. La comprobación de restauración tras la primera copia, y **Comprobar restauración**, también aseguran que la instantánea contiene todos los segmentos de todos los discos con el tamaño que deben tener.
- Entre ejecuciones la VM no tiene ningún punto de control registrado en libvirt. BombVault guarda la definición del último punto de control `bombvault-` en `/config/vm-checkpoints` y quita solo el registro de libvirt (`virsh checkpoint-delete --metadata`). Su mapa de bits se queda en el archivo qcow2 y sigue contando escrituras, y la siguiente ejecución vuelve a poner el punto de control (`virsh checkpoint-create --redefine`) antes de empezar. libvirt se niega a anular la definición de una VM que tiene puntos de control, así que esto es lo que permite que **Remove VM** de Unraid y una restauración funcionen como siempre.
- Un trabajo de copia que BombVault dejó en marcha, porque se detuvo a mitad de una copia, se termina cuando BombVault vuelve a arrancar, junto con los puntos de control que esa ejecución dejó registrados.
- Desactivar el interruptor, sacar la VM del calendario o borrar sus copias también termina un trabajo así, y borra además el mapa de bits mientras la VM está en marcha. En una VM apagada, libvirt no puede borrarlo y se queda sin usar en la imagen.
- Los puntos de control de otras herramientas se dejan en paz. Una VM que tiene alguno no se puede restaurar hasta que se borren, y la restauración se detiene antes de tocar la VM.

El interruptor recurre a otra vía por sí solo. Un punto de control que falta o está roto, un disco que cambió de tamaño o una instantánea más reciente que no es de bloques modificados significan que el disco entero se lee una vez más. Una VM apagada, un disco raw o zvol, ningún enlace SSH al host, un libvirt sin la API de copias o muy poco espacio libre en `/config` significan la copia habitual con el método de arriba. La tarjeta dice cuál de estos usó la última ejecución y por qué.

Requisitos: libvirt con la API de copias (probado con libvirt 12.2 y QEMU 10.2 en Unraid 7.3), sshd con `AllowStreamLocalForwarding yes` (el valor predeterminado, también en Unraid, que desactiva el reenvío TCP) y discos qcow2. libvirt rechaza los puntos de control en discos raw, y Unraid crea vdisks raw por defecto. La VM sigue funcionando durante la copia, como con Instantánea en vivo, y los segmentos que esperan a restic ocupan hasta 4 GiB en `/config`.

## VMs con instantáneas de Unraid {#unraid-snapshots}

Una instantánea que toma el VM Manager de Unraid convierte un disco qcow2 en una cadena: la imagen original abajo, y encima un archivo por instantánea, cada uno con solo lo que cambió desde el de debajo. La VM escribe en el archivo más reciente. Un disco con dos instantáneas son tres archivos.

BombVault copia una VM así por completo:

- **Cada archivo de la cadena.** Sigue la cadena de respaldo de cada disco hasta el final y copia todos los archivos de ella, junto con la lista de instantáneas de Unraid (`/etc/libvirt/qemu/snapshotdb/<vm>/snapshots.db`, leída por SSH). Una cadena que no se puede seguir hasta el final, por ejemplo porque falta un archivo intermedio, hace fallar la copia y nombra el archivo.
- **Una copia en vivo conserva las instantáneas.** **Instantánea en vivo** pone una superposición temporal encima mientras lee el disco y la fusiona de nuevo después, solo en la imagen inmediatamente inferior. La cadena es la misma después de la copia que antes. Una VM sin instantáneas se comporta como siempre.
- **Una restauración devuelve la cadena.** Una restauración in situ escribe cada archivo de vuelta donde estaba y devuelve la lista de instantáneas de Unraid, así que el VM Manager vuelve a listar las instantáneas. Una restauración junto al original escribe los archivos en la carpeta de la copia y apunta los discos restaurados a los archivos restaurados, de modo que la copia nunca lee de las imágenes del original. La lista de instantáneas de Unraid solo vuelve con una restauración in situ, porque nombra los archivos donde se copiaron. Sin ella la copia arranca igual, y el VM Manager simplemente no lista instantáneas para ella.
- **Solo bloques modificados** lee lo que ve el invitado, así que su restauración vuelve como un único archivo qcow2 aplanado sin las instantáneas.
- **Una restauración devuelve el disco de la copia que elijas.** Las instantáneas tomadas o fusionadas desde esa copia no cambian eso. Una copia de antes de una instantánea define la VM sobre el archivo que contiene, y una copia de antes de una fusión devuelve su superposición y funciona sobre ella. Los archivos más nuevos se quedan donde están, sin usar y sin cambios. Una copia que contiene una superposición pero no el archivo de debajo se rechaza antes de tocar la VM. Tras una restauración, la definición de la VM no lista ninguna cadena, así que libvirt lee la cadena de los archivos restaurados.

### Ejemplo: una VM con dos instantáneas {#example-unraid-snapshots}

La VM `homeassistant` tiene un disco, `vdisk1.qcow2`, y dos instantáneas que tomaste en el VM Manager antes de dos actualizaciones. Su disco es ahora una cadena de tres archivos. Funciona todo el tiempo, así que usa **Instantánea en vivo**.

1. Abre **Copias de seguridad**, luego `homeassistant` y su pestaña **Ajustes**. En la tarjeta **VM**, comprueba que **Método** es **Instantánea en vivo** y que **Solo bloques modificados** está desactivado.
2. Pulsa **Copiar ahora** en la cabecera de la página.
3. Cuando termina la ejecución, su línea en **Ejecuciones recientes** de la pestaña **Verificación** está en verde. La VM siguió funcionando todo el tiempo, y la copia contiene los tres archivos del disco y la lista de instantáneas de Unraid.
4. En el VM Manager, las dos instantáneas siguen ahí, y el disco sigue leyendo a través de los mismos tres archivos.

Un mes después una actualización sale mal y quieres el estado de ayer junto al roto, para compararlos:

1. Abre **Restauración**, elige **VMs** y haz clic en `homeassistant`.
2. Conserva la copia recomendada de anoche y elige **Junto al original** en **Dónde**.
3. Pulsa **Restaurar**. Los tres archivos acaban en una carpeta con el nombre de la VM más `-restored`, y el disco restaurado apunta a los archivos restaurados. No cambia nada en la VM en ejecución.

Para devolver toda la VM a su estado anterior, elige en su lugar **Reemplazar original** con **Antes, copia del estado actual** activado. BombVault copia el estado roto, apaga la VM, escribe de vuelta los tres archivos y la lista de instantáneas de Unraid, y la vuelve a iniciar.

## TrueNAS SCALE {#truenas-scale}

Los pasos de arriba están escritos para Unraid, pero el mismo enlace `qemu+ssh://` funciona contra cualquier libvirtd al que llegue el contenedor, TrueNAS SCALE incluido, que vuelve a tener VMs sobre libvirt desde la 25.04.2. En TrueNAS la cadena de conexión necesita una sustitución y la clave va a root en los propios ajustes de usuario de TrueNAS. Otras tres cosas cambian lo que contiene una copia: los discos en zvols, el estado del vTPM y los nombres que TrueNAS da a sus VMs.

### 1. El socket de libvirt {#truenas-socket}

El libvirtd de TrueNAS no escucha donde espera la cadena de conexión construida a partir de los campos de **SSH del host**. Define `LIBVIRT_URI` en su lugar, con el parámetro adicional `?socket=`:

```sh
LIBVIRT_URI=qemu+ssh://<user>@<truenas-host>/system?socket=/run/truenas_libvirt/libvirt-sock
```

BombVault usa `LIBVIRT_URI` exactamente como está escrita, y `LIBVIRT_HOST`, `LIBVIRT_SSH_USER` y `LIBVIRT_SSH_PORT` no hacen falta. De cada una de ellas que esté sin definir, BombVault toma el host, el usuario y el puerto de la URI también para sus propios comandos SSH, de modo que la copia de la NVRAM y las copias de conjuntos de datos ZFS llegan a la misma máquina que `virsh`. Una variable que definas tú prevalece siempre, y el registro de arranque nombra el destino que tomó de la URI. En TrueNAS un usuario que no sea root necesita además `zfs allow <user> snapshot,destroy,mount <dataset>` para las copias de conjuntos de datos ZFS, consulta [Conjuntos de datos ZFS](zfs-datasets.md#truenas).

Pon la dirección de LAN de TrueNAS en la URI. Con `LIBVIRT_URI` y `LIBVIRT_HOST` sin definir, BombVault toma como host la puerta de enlace predeterminada de la app, porque `host.docker.internal` no se resuelve en una app de TrueNAS, y el registro de arranque nombra la dirección que eligió. Eso basta para las copias de conjuntos de datos ZFS. Las copias de VMs siguen necesitando la URI por el socket.

La app necesita la capacidad `SETPCAP` para las copias de VMs, junto a `CHOWN`, `DAC_OVERRIDE`, `FOWNER`, `SETGID` y `SETUID`. Sin ella libvirt se detiene con "cannot apply process capabilities". El YAML de [TrueNAS SCALE](getting-started.md#truenas) las define todas, junto con la `SYS_CHROOT` que necesita la instantánea en vez de parada.

En TrueNAS SCALE 25.10.0 la ruta del socket se comprobó en hardware real: el `qemu:///system` de serie falla con `Failed to connect socket to '/var/run/libvirt/libvirt-sock': No such file or directory`, y la misma llamada con `?socket=/run/truenas_libvirt/libvirt-sock` lista los dominios.

### 2. La clave de BombVault en TrueNAS {#truenas-key}

El paso 2 de arriba escribe en la unidad flash de Unraid y aquí no se aplica. En TrueNAS:

1. Activa el servicio SSH en **System, Services**.
2. En **Credentials, Users**, edita **root** y pega la clave pública de BombVault de **Ajustes, Integraciones, SSH del host** como su clave pública SSH. TrueNAS la conserva entre reinicios y actualizaciones.

Las instalaciones nuevas de TrueNAS 24.10 y posteriores no dan contraseña a root, y el inicio de sesión con clave no la necesita. Si prefieres no usar root, autoriza la clave para una cuenta de administrador con sudo y pon su nombre en lugar de `root` en `LIBVIRT_URI`. Las copias de conjuntos de datos ZFS necesitan entonces `zfs allow` como se describió arriba.

Una réplica ZFS a un segundo servidor inicia sesión con una clave propia, que muestra el diálogo de la réplica. La clave SSH del host no autoriza una réplica, y la clave de la réplica no autoriza las copias de VMs.

!!! warning "TrueNAS no ve los cambios hechos con virsh"
    El middleware de TrueNAS, la capa que hay detrás de sus pantallas de VMs, está pensado para ser la única vía por la que se cambian sus VMs. Un comando `virsh` por este enlace SSH, que es lo que ejecutan la copia y la restauración de VMs de BombVault, lo rodea, y TrueNAS podría más tarde reconciliar o sobrescribir un estado que no creó. Esto se deduce del comportamiento documentado de TrueNAS y no se ha visto en un equipo real. Es el mismo tipo de riesgo que cuando el VM Manager de Unraid reconcilia cambios hechos fuera de él, solo que más agudo en TrueNAS.

### 3. Discos en zvols {#truenas-zvols}

Los discos de las VMs de TrueNAS suelen ser zvols, dispositivos de bloques ZFS en `/dev/zvol/<pool>/<dataset>`, no archivos qcow2. restic no puede copiar un dispositivo de bloques por su ruta como copia un archivo, así que BombVault reconoce un disco zvol en el XML del dominio (`<source dev="...">` en lugar de `<source file="...">`) y lo maneja por el mismo enlace SSH. El contenedor de BombVault no necesita herramientas ZFS.

Una copia de un zvol:

1. `zfs snapshot <dataset>@bombvault-vm-<time>` fija un punto en el tiempo, también mientras la VM funciona.
2. `zfs clone -o volmode=dev -o readonly=on` hace que esa instantánea sea legible como dispositivo. `volmode=dev` evita que el host escanee las particiones del invitado.
3. `dd` lee el dispositivo, y la imagen raw entra en restic como un solo archivo, `/vm-disks/<dataset>.img`, con la etiqueta `vm:<name>:zvol:<dev>`. restic lo corta en fragmentos según el contenido, así que los bloques que no cambiaron desde la última copia no se vuelven a guardar.
4. El clon y la instantánea se destruyen después, haya funcionado la copia o no. Si BombVault se detuvo a mitad de una copia, la siguiente copia de ese zvol quita primero la instantánea `bombvault-vm-<time>` y su clon `<dataset>-bombvault-vm-<time>`. Deja en paz cualquier otra instantánea, y conserva aquella de la que se clonó otra cosa. El registro nombra lo que quitó o conservó.

BombVault no usa `zfs send` para esto. Cada registro de un flujo de send lleva el GUID de la instantánea y una suma de comprobación acumulada, así que restic no encontraría nada de la última copia que reutilizar y guardaría el disco entero cada vez. Medido en TrueNAS SCALE 25.10 con un zvol de 512 MiB con 200 MiB de datos: la primera copia añadió 211 MB, una segunda sin cambios 52 KB, y una tras cambiar 1 MiB 3,6 MB. El mismo zvol mediante `zfs send` añadió 214 MB en cada ejecución.

Cada ejecución también escribe una pequeña instantánea con la etiqueta `vm:<name>` con el XML del dominio, una vez que todos los discos están dentro. Una VM cuyos discos son todos zvols no tiene instantánea de archivos, y esta es la que lista la ejecución bajo la VM y la hace restaurable.

Una restauración escribe cada zvol de vuelta en su sitio, igual que un disco de archivo se escribe de vuelta en su archivo, de modo que la VM restaurada arranca con los datos restaurados:

1. Antes de tocar nada, BombVault comprueba que la ejecución contiene una imagen de cada disco zvol. Una ejecución que carece de alguna se rechaza y la VM se deja como está.
2. Un zvol existente recibe primero una instantánea, `<dataset>@bombvault-before-restore-<time>`, con la hora en UTC como todos los nombres de instantánea que hace BombVault, así que puede diferir en horas del reloj local. El registro la nombra, y `zfs rollback` hasta ella deshace la restauración mientras sea la instantánea más reciente del zvol. Cuando existe una más nueva, como la de la siguiente restauración, `zfs rollback -r` lo hace y destruye las instantáneas más nuevas. Se queda hasta que la quites con `zfs destroy` cuando la VM restaurada funcione.
3. Un zvol que ya no existe se crea con el tamaño que tenía cuando se copió, y uno existente de otro tamaño se devuelve a ese tamaño.
4. `dd` escribe la imagen en `/dev/zvol/<dataset>`. Con la compresión activada, el valor predeterminado de TrueNAS, las partes vacías de la imagen vuelven como huecos y no ocupan espacio.

**Una restauración en otra instancia necesita un pool de destino.** Mueve los discos de archivo a una carpeta de destino, pero una carpeta no nombra ningún pool ZFS, así que sin uno BombVault rechaza la restauración antes de empezar. Pasa el pool como parámetro `zvolPool` de `POST /api/foreign/restore`: cada zvol conserva su ruta por debajo del pool (`tank/vms/win10/disk0` en `flashpool` pasa a `flashpool/vms/win10/disk0`), y el XML del dominio apunta a él. La ventana **De una carpeta de copias antigua** de la página Restauración no tiene campo para esto, así que solo se alcanza con una llamada directa a la API. Una restauración en la misma instancia escribe cada zvol de vuelta sobre sí mismo.

**Exportar como archivos deja fuera los zvols.** Una VM cuyos discos son todos zvols se rechaza con ese motivo, así que restáurala desde una copia. Para una VM que además tiene discos de archivo, la exportación contiene los discos de archivo y una nota que nombra los zvols que dejó fuera.

Esto se probó en TrueNAS SCALE 25.10 con ZFS 2.3.4: tres copias de un zvol de prueba, una restauración in situ después de que el zvol se sobrescribiera y creciera, y una restauración sobre un zvol que se había borrado. Cada restauración se releyó con el mismo SHA-256 que el origen, el zvol recuperó su antiguo tamaño y no quedó ninguna instantánea ni clon. Una VM entera cuyo único disco es un zvol se copió en ejecución, con Ordenado y con Instantánea en vivo, y apagada, y una restauración de VM entera (anular la definición, escribir los zvols, definir, arrancar) se ejecutó también allí: el zvol se releyó con la suma de comprobación del origen y la VM arrancó.

Los discos de archivo, como los que usa Unraid, no se ven afectados por nada de esto.

### 4. Estado del vTPM {#truenas-vtpm}

Una VM puede tener un TPM virtual, que necesitan Secure Boot y los invitados con Windows 11. Su estado vive fuera del disco y de la NVRAM, así que una copia de solo esos dos devolvería un invitado con un TPM nuevo y vacío.

- **Un TPM de paso directo**, un chip real pasado al invitado, nombra su ruta en el XML del dominio. BombVault guarda su estado junto a la NVRAM por SSH y lo escribe de vuelta antes de `virsh define`, en la medida de lo posible, igual que la NVRAM: una lectura o escritura fallida se registra y no hace fallar la copia ni la restauración.
- **Un vTPM emulado**, el tipo que TrueNAS configura para invitados con Windows 11 y Secure Boot, no publica ninguna ruta de estado en el XML del dominio, así que su estado no se copia. BombVault no adivina una ruta, y la copia termina con una nota de que el estado del TPM de la VM no está en ella. Ten a mano la clave de recuperación de un invitado así, como su clave de BitLocker, antes de restaurarlo.
- **No puedes conectar tú mismo un TPM a una VM de TrueNAS.** La API de dispositivos de VM de TrueNAS SCALE 25.10 acepta solo `CDROM`, `DISPLAY`, `NIC`, `PCI`, `RAW`, `DISK` y `USB`, y rechaza `TPM` como tipo desconocido.

El manejo del TPM solo está probado con pruebas unitarias y no se ha ejecutado contra un invitado real de TrueNAS con vTPM. Una VM sin TPM se copia y restaura exactamente como antes.

La NVRAM se captura como de costumbre. TrueNAS la guarda por VM en `/var/db/system/vm/nvram/<id>_<name>_VARS.fd`, y el XML del dominio nombra esa ruta.

### 5. Nombres de VMs en TrueNAS {#truenas-names}

En Unraid el nombre libvirt de una VM es el nombre que elegiste. TrueNAS inventa el nombre libvirt, y lo ha cambiado dos veces:

| Versión de TrueNAS | Nombre libvirt | Dónde vive tu nombre |
|---|---|---|
| 25.10 "Goldeye" | `{id}_{name}`, como `1_debian` | en el nombre libvirt, detrás del `{id}_` |
| 26 | el UUID de la VM, como `550e8400-e29b-41d4-a716-446655440000` | solo en el `<title>` del XML del dominio |

En un host que BombVault detecta como TrueNAS, o que `PLATFORM=truenas` nombra, la lista de Copias de seguridad muestra tu nombre: `debian` en lugar de `1_debian` en la 25.10, y el `<title>` en lugar del UUID en la 26, o el UUID cuando no hay título. Esto es solo lo que muestra la lista. Cada llamada a `virsh` y los ajustes por VM siguen usando el nombre libvirt, así que la copia y la restauración actúan siempre sobre la VM correcta. En TrueNAS 26 eso significa que una VM que TrueNAS vuelve a crear con un UUID nuevo aparece como un elemento nuevo sin sus ajustes, aunque su nombre parezca el mismo. En Unraid no cambia nada, tampoco para una VM que llamaste `10_Windows`.

El esquema de la 25.10 se vio en un equipo real con TrueNAS SCALE 25.10. El esquema de TrueNAS 26 procede de la documentación de TrueNAS y solo está probado con pruebas unitarias.

## Solución de problemas {#troubleshooting}

| Síntoma | Causa y solución |
|---|---|
| **Probar conexión** se queda colgado y luego falla | No se puede llegar al host. Revisa el paso 3 (red, VLAN, cortafuegos) y **SSH del host: Dirección** y **Puerto**. |
| `Permission denied (publickey)` aunque la clave está en `authorized_keys` | `StrictModes` de sshd rechaza el archivo de claves porque una carpeta superior tiene el propietario o el modo equivocado. Es habitual cuando `/root/.ssh` es un enlace a la unidad flash FAT (`/boot/config/ssh/...`), que es escribible para todos. El registro del host muestra `Authentication refused: bad ownership or modes for directory ...`. Añade `StrictModes no` a la configuración de sshd del host en la sección global, antes de cualquier bloque `Match`, y ejecuta `/etc/rc.d/rc.sshd restart`. Unraid escribe `/etc/ssh/sshd_config` de nuevo en cada arranque, así que hazlo persistente como se describe más abajo. |
| `Permission denied (publickey)` y la clave realmente falta | Repite el paso 2 y comprueba que SSH está activado. |
| `Host key verification failed` | Ejecuta `docker exec BombVault rm -f /config/ssh/known_hosts` y vuelve a probar. |
| La comprobación del paso 3 dice `UNREACHABLE` | El contenedor no puede llegar al puerto SSH del host. Consulta el paso 3: acceso del host a redes personalizadas, enrutamiento de VLAN, o la IP shim o de LAN. |
| Faltan las variables en **Edit** | Importa de nuevo la plantilla. Un contenedor existente conserva su antigua configuración guardada. |
| Una VM con instantáneas de Unraid falla y nombra un archivo | La cadena de respaldo del disco no se puede seguir hasta el final. Comprueba en el VM Manager que cada instantánea de ese disco sigue ahí y que el archivo nombrado existe bajo `/mnt`. |

Hay más síntomas de VMs en [Solución de problemas](troubleshooting.md#vm-backup-will-not-connect).

## Mantenerlo tras los reinicios {#persistence}

Unraid escribe `/etc/ssh/sshd_config` de nuevo en cada arranque, así que los cambios del lado del host hay que volver a hacerlos al arrancar:

- El **puerto SSH** se guarda en `/boot/config/ident.cfg`, fijado desde la interfaz web de Unraid.
- **`StrictModes no`**, si lo necesitaste arriba, no se conserva. Aplícalo al arrancar desde `/boot/config/go`:

  ```sh
  # /boot/config/go: keep SSH key auth working with flash-based authorized_keys
  ( for i in $(seq 1 30); do [ -f /etc/ssh/sshd_config ] && break; sleep 2; done
    grep -q '^StrictModes no' /etc/ssh/sshd_config || \
      sed -i '0,/^Match /s/^Match /StrictModes no\n&/' /etc/ssh/sshd_config
    # SIGHUP reloads sshd's config in place: it keeps the listener + open sessions
    # and avoids the "killing listener process / Restarting SSH server daemon"
    # warning that `rc.sshd restart` prints at boot. Only reload if sshd is already
    # up; if it starts after this runs, it reads the edited config on its own.
    pidof sshd >/dev/null && killall -HUP sshd ) &
  ```

- La propia clave SSH de BombVault y `~/.ssh/config` viven en `/config` (appdata) y se crean de nuevo en cada arranque, así que sobreviven a los reinicios del contenedor y del host.

## Seguridad {#security}

La clave autorizada concede root en el host, el mismo nivel de confianza que el socket de Docker que BombVault ya usa. Mantén BombVault en una red de confianza. La clave es ed25519, se guarda con el modo `0600` en appdata, y la clave del host está fijada. Cada comando del host se pasa como argumentos separados, nunca a través de un shell remoto, y la conexión SSH usa `BatchMode` y un tiempo límite de conexión.
