# Solución de problemas

Un breve FAQ. Para la tabla completa de solución de problemas del lado del host en las copias de VMs por SSH (permission-denied, verificación de la clave del host, variables de plantilla que faltan y más), consulta la [guía de copias de VMs por SSH](vm-backup-ssh-setup.md).

## Algo no está bien conectado {#something-is-not-wired-up-correctly}

Añade la tarjeta **Integración con el host** al resumen en **Personalizar**. La comprobación de la integración con el host sondea cada montaje y cada CLI (socket de Docker, libvirt, restic, qemu-img, rclone) e informa de lo que falte, y **Comprobar ahora** la ejecuta de nuevo. Empieza por aquí antes de suponer un fallo: un montaje que falta o un host inalcanzable se ve al instante.

## No puedo llegar a la interfaz web {#i-cannot-reach-the-web-ui}

BombVault sirve HTTPS de serie en el puerto `3443` (certificado autofirmado), así que abre `https://<tu-ip-de-unraid>:3443`. Acepta el aviso del certificado autofirmado, o pon BombVault detrás de un proxy inverso con tu propio certificado. Si lo ejecutas con `HTTP_ONLY=true`, sirve HTTP sin cifrar en el puerto `3000` en su lugar (pensado para usarse detrás de un proxy que termine TLS).

## He perdido mi APP_KEY {#i-lost-my-app_key}

`APP_KEY` deriva la contraseña del repositorio restic. Sin ella, sin el kit de recuperación y sin un depósito de claves en tu grupo, las copias cifradas no se pueden recuperar. Por eso el resumen muestra **Guarda tu kit de recuperación** hasta que lo hayas descargado. Consulta [Copias externas y recuperación](offsite-recovery.md#recovery-kit). Genera una clave con `openssl rand -hex 32` y guárdala fuera del servidor antes de confiar en cualquier copia.

## He olvidado la contraseña de acceso {#i-forgot-the-login-password}

Elige **¿Has olvidado la contraseña?** en la página de inicio de sesión. BombVault escribe un código de 12 caracteres en el registro de su contenedor (en Unraid: pestaña Docker, icono de BombVault, **Logs**). Escríbelo con una contraseña nueva en un plazo de 10 minutos. El restablecimiento también desactiva la verificación en dos pasos, elimina todas las claves de acceso, revoca todos los tokens de API, claves de MCP e inicios de sesión OAuth, termina todas las demás sesiones y desactiva el emparejamiento de teléfonos. Vuelve a añadir tus claves de acceso en **Ajustes, Seguridad**, vuelve a activar allí el emparejamiento de teléfonos y crea tokens y claves nuevos donde aún los necesites. Consulta [Modelo de seguridad](configuration.md#login).

## Las copias están detenidas y dicen que la clave ya no se puede abrir {#backups-stopped-and-say-the-key-can-no-longer-be-opened}

Este servidor adoptó la clave de copias de otro servidor en el Asistente de emergencia, y `APP_KEY` se cambió después. La clave adoptada está sellada con la APP_KEY antigua, así que BombVault no puede abrirla y no copia nada en lugar de escribir con la clave equivocada. **Necesita atención** en el resumen lo nombra. Vuelve a poner APP_KEY como estaba, o vuelve a subir el kit de recuperación en la página Restauración, en **Emergencia**.

## Un teléfono acaba en la página de inicio de sesión {#a-phone-lands-on-the-login-page}

Un teléfono emparejado recibe su sesión a través del grupo solo mientras **Los teléfonos pueden emparejarse con las doce palabras del grupo** esté activado en **Ajustes, Seguridad**. Un restablecimiento de contraseña lo desactiva. Una app más antigua que el servidor tampoco recibe sesión hasta que se actualice. Un servidor añadido por su dirección siempre pide la contraseña. Un teléfono quitado de la página Instancias vuelve desactivado cuando su app se abre de nuevo. Activa **Consultar esta instancia** en **Supervisión**, dentro de sus detalles, para dejarlo entrar.

## La copia de VMs no conecta {#vm-backup-will-not-connect}

La copia de VMs habla con libvirt por SSH, nunca por un montaje.

- Confirma que SSH está activado en el host y que la clave pública de BombVault está autorizada en `/root/.ssh/authorized_keys` (Ajustes, Integraciones, SSH del host muestra la clave y un botón **Probar conexión**).
- En una red personalizada `br0.x`, pon `LIBVIRT_HOST` a la IP de LAN de tu Unraid (allí el contenedor no puede llegar al host mediante `host.docker.internal`). Activa **Settings, Docker, Host access to custom networks**.
- Si cambiaste el puerto SSH de Unraid, pon `LIBVIRT_SSH_PORT` a juego.
- En TrueNAS, define `LIBVIRT_URI` con el propio socket de libvirt de TrueNAS, da a la app la capacidad `SETPCAP` y añade la clave a root en **Credentials, Users** (consulta [TrueNAS SCALE](getting-started.md#truenas)).
- El diagnóstico completo paso a paso (prueba de alcance, enrutamiento de VLAN, `Permission denied (publickey)`, `Host key verification failed`) está en la [guía de copias de VMs por SSH](vm-backup-ssh-setup.md).

## No se ejecutó una instantánea en vivo de una VM {#a-live-vm-snapshot-did-not-run}

Las instantáneas en vivo necesitan el agente invitado de qemu instalado en la VM y el disco en `/mnt/cache` (o `/mnt/diskX`), no en `/mnt/user`. En una VM apagada, lo en vivo pasa automáticamente a ordenado. Una copia ordenada apaga la VM, copia los discos y luego la reinicia, así que siempre es coherente. Espera hasta diez minutos a que el invitado se apague y nunca fuerza el apagado, de modo que un invitado Windows que instala actualizaciones no se corta a medias. Un invitado que sigue funcionando pasado ese tiempo se deja en paz, y esa copia falla con el motivo.

## Una copia falló con "repository is already locked" {#a-backup-failed-with-repository-is-already-locked}

Normalmente es un bloqueo huérfano de restic que quedó cuando el contenedor se actualizó o se reinició a mitad de una operación. BombVault detecta un bloqueo demostrablemente huérfano, lo limpia a la fuerza y reintenta una vez, automáticamente. Si persiste, abre la tarjeta **Comprobar las copias** en **Ajustes, Verificación** y elige **Desbloquear** bajo **Más** en la fila de la sección afectada para limpiar a mano un bloqueo caducado. Un problema genuino sigue saliendo a la luz en lugar de ocultarse. Tras un reinicio BombVault espera hasta que un bloqueo así lleve diez minutos sin renovarse. Un restic que sigue en ejecución, por ejemplo en un segundo BombVault sobre el mismo repositorio, renueva su bloqueo cada cinco minutos.

## Mi copia externa no se hizo después de una copia {#my-off-site-copy-did-not-happen-after-a-backup}

La replicación externa es de mejor esfuerzo por diseño, así que un contratiempo externo nunca hace fallar la copia local. Comprueba cuándo copia el destino: su tarjeta **Copiar** dice **Después de cada copia** o nombra un calendario propio, y **Ajustes, Calendario, Copias externas por sección** puede dar a una sección su propio calendario. Usa **Replicar ahora** en la fila de la sección de la tarjeta **Copias externas** de **Ajustes, Destinos** para una ejecución a demanda, y mira la cabecera de estado del resumen, que dice **Copia externa en curso** mientras se ejecuta. Una copia fallida aparece en **Necesita atención**.

## No se confía en el certificado de un rest-server {#a-rest-servers-certificate-is-not-trusted}

restic rechaza un certificado que no conoce, y el error dice dónde ponerlo. Copia el certificado autofirmado del rest-server, o el de la autoridad que lo firmó, como archivo PEM en la carpeta `certs` de la carpeta de datos de BombVault (`/config/certs` en el contenedor, `appdata/bombvault/certs` en Unraid) y vuelve a probar. No hace falta reiniciar.

## Una copia dice que no se copiaron datos fuera de Host Data {#data-outside-host-data}

El contenedor guarda datos en una carpeta que BombVault no puede leer a través de Host Data. La copia guarda la definición y todas las carpetas a las que llega, pone naranja su línea en el registro de actividad y nombra las carpetas que dejó fuera. Hay dos casos habituales: un ixVolume en TrueNAS bajo `/mnt/.ix-apps`, y un volumen con nombre de Docker bajo `/var/lib/docker/volumes`, como el volumen de la base de datos de un stack compose. La tarjeta **Carpetas a copiar** de la pestaña **Ajustes** del contenedor lista cada una de esas carpetas con su ruta dentro del contenedor y su ruta en el host, marcada en rojo.

Un volumen anónimo de Docker no se notifica. Docker crea uno por sí mismo para una carpeta que declara la imagen y que nadie asignó, lo nombra con 64 dígitos hexadecimales y lo guarda bajo `/var/lib/docker/volumes`. Suele contener una caché, así que la copia se queda en verde y la tarjeta solo lo lista, con una nota de que es anónimo.

- **Quieres copiar esos datos.** Asigna la carpeta a una ruta del host bajo Host Data, por ejemplo una carpeta de tu recurso compartido appdata, y la siguiente copia la incluye.
- **No los necesitas.** Añade una de sus dos rutas como línea en la tarjeta **Patrones de exclusión** de la misma pestaña: la ruta dentro del contenedor, como `/cache`, o la ruta del host que nombra el aviso, como `/var/lib/docker/volumes/jellyfin_cache/_data`. A partir de la siguiente copia, el aviso deja fuera esa carpeta, y la vista previa bajo el campo dice por qué. Una línea para una carpeta superior también sirve, un nombre suelto o `/` no.

## Una copia dice Aplazado {#deferred-backups}

La app del contenedor estaba ocupada, así que su copia espera y se vuelve a intentar sola. La línea del registro de actividad dice por qué y cuándo es el siguiente intento. La tarjeta **Cuando la app está ocupada** de la pestaña **Ajustes** del contenedor dice lo mismo y cuándo termina la espera. No hay que hacer nada. Para copiar de inmediato, usa **Copiar ahora**: una copia que inicias tú nunca espera, tampoco por un código de salida 75.

- **Tu propia comprobación.** Haz que el **Comando pre-copia** termine con el código 75 mientras la app esté ocupada, por ejemplo `[ "$(cat /tmp/players)" = 0 ] || exit 75`. Cualquier otro código distinto de 0 hace fallar la copia como antes, así que un script roto sigue apareciendo en **Necesita atención**.
- **Nunca se pregunta a Plex, Jellyfin o Emby.** BombVault pregunta primero al servidor en la propia dirección del contenedor y, cuando no responde, ejecuta curl o wget dentro del contenedor. Una imagen sin ninguno de los dos no puede consultarse por sí misma. El registro lo dice entonces. Con **Copiar cuando el servidor no responde** activado, una copia así se ejecuta igualmente, y con él desactivado espera.
- **Jellyfin o Emby rechaza.** Comprueba la **Clave de API**. Se crea en el panel del servidor, en Claves de API.
- **Plex rechaza.** BombVault lee el token de `Preferences.xml` en la carpeta montada en `/config`. Un servidor en el que nunca se inició sesión no tiene ninguno y solo responde donde permite el acceso sin él.
- **Una película en pausa mantiene la copia esperando toda la noche.** Define **Ignorar sesiones en pausa tras (min)**, por ejemplo en 30.
- **La comprobación de tráfico nunca deja pasar la copia.** Sube **Ocupado a partir de (KiB/s)** por encima de lo que envía el contenedor cuando nadie lo usa. Justo después de arrancar BombVault, la comprobación espera hasta haber vigilado el contenedor durante **Tranquilo durante (min)** minutos.
- **La copia nunca se ejecuta de noche.** Con una franja fijada, los intentos solo ocurren dentro de ella. Una espera que llega a su plazo fuera de la franja termina igualmente en el plazo.

## Una copia grande fue eliminada por quedarse sin memoria {#a-large-backup-was-killed-for-running-out-of-memory}

Define **Límite de memoria por copia (MiB, 0 = ninguno)** en la tarjeta **Rendimiento** de **Ajustes, Sistema**, bastante por debajo de lo que puede usar el contenedor. Cerca del límite restic libera memoria con más frecuencia, lo que cuesta algo de velocidad, de modo que la copia se ralentiza en lugar de ser eliminada.

## Una restauración se abortó antes de empezar {#a-restore-aborted-before-it-started}

Antes de detener o eliminar nada, la restauración ejecuta una comprobación previa de conflictos: verifica que la IP estática del contenedor y los puertos del host publicados estén libres. Si otro contenedor ya tiene alguno, aborta con un mensaje claro y útil en lugar de dejar una restauración a medias. Libera el puerto o la IP en conflicto y vuelve a intentarlo.

Si la red del contenedor no existe, por ejemplo porque se eliminó su app de TrueNAS, la comprobación de la restauración nombra la red y la restauración se niega antes de cambiar nada. Reinstala la app en TrueNAS con el mismo nombre, lo que vuelve a crear la red, y luego restaura sus datos.

## Una exportación sin formato falló en lugar de escribir un archivo {#a-plain-export-failed-instead-of-writing-a-file}

Si **Cifrar exportaciones con age** está activado en **Ajustes, Sistema** pero no hay ningún destinatario válido, una exportación falla con un error claro en lugar de escribir texto sin cifrar. Añade un destinatario válido (una clave pública age o una clave pública SSH), o desactiva el cifrado si quieres que la exportación sea en texto sin cifrar. Consulta [Exportaciones sin formato](backups.md#exports).

## Falló un volcado de base de datos {#a-database-dump-failed}

Un volcado fallido nunca hace fallar la copia que lo rodea. Se registra como una ejecución fallida propia, y el motivo nombra qué hay que arreglar.

- **Acceso rechazado.** El volcado inicia sesión con las propias variables de contraseña del contenedor (`POSTGRES_PASSWORD`, `MARIADB_ROOT_PASSWORD`, `MYSQL_ROOT_PASSWORD` o sus versiones `_FILE`). Compruébalas en el contenedor de la base de datos. Una variable `_FILE` que apunta a un secreto que el propio usuario del contenedor no puede leer falla igual.
- **Faltan privilegios.** Con una contraseña de root aleatoria el volcado solo puede iniciar sesión como el usuario de la app, así que contiene esa base de datos, y MySQL 8.4 y posteriores pueden rechazarlo de plano. Da al contenedor una contraseña de root real, o desactiva el volcado para él.
- **Las tablas del sistema necesitan una actualización.** MariaDB se niega a volcar cuando sus tablas del sistema proceden de una versión anterior (error 1558). Añade la variable `MARIADB_AUTO_UPGRADE=1` y reinicia el contenedor, o ejecuta `mariadb-upgrade` dentro de él una vez.
- **No hay herramienta de volcado.** Una imagen reducida o compilada por ti sin `pg_dump`, `mysqldump` o `mariadb-dump` no se puede volcar. Usa la imagen oficial, o desactiva el volcado.
- **Un límite de tiempo.** Un volcado recibe `DB_DUMP_MAX_HOURS` (6 por defecto), la copia que lo rodea recibe `BACKUP_MAX_HOURS`, y un volcado que deja de progresar se corta tras `BACKUP_STALL_HOURS`. La causa habitual de lo último es un bloqueo que mantiene la aplicación. Sube el límite que saltó, o vuelca mientras la aplicación esté tranquila.
- **El contenedor está en pausa o reiniciándose.** El volcado habla con el servidor en ejecución. Si el contenedor no deja de reiniciarse, su propio registro dice por qué.
- **No se pudo eliminar un volcado dañado.** Un volcado que BombVault no pudo terminar se borra de nuevo. Cuando ese borrado falla, el volcado se queda en la lista marcado como dañado, y puedes borrarlo allí.

## Falló una importación {#an-import-failed}

Una importación detiene el contenedor, aparta su carpeta de datos y deja que la imagen cree una vacía en su lugar. Si falla un paso previo a la importación en sí, la carpeta antigua se devuelve automáticamente. Si falla la importación, el contenedor conserva la carpeta nueva y la antigua se queda a su lado como `<data folder>.bombvault-before-import-<timestamp>`. El mensaje de error de la ejecución nombra la ruta exacta.

Una carpeta de datos que es un conjunto de datos propio, como en TrueNAS, no se puede renombrar. La importación deja entonces la carpeta en su sitio y mueve su contenido a la carpeta conservada a su lado, así que el conjunto de datos de arriba necesita espacio para ello. La importación lo comprueba antes de detener el contenedor.

Para devolverla a mano: detén el contenedor, renombra la carpeta de datos actual para quitarla de en medio, devuelve a la carpeta conservada su nombre original e inicia el contenedor. Para un conjunto de datos, mueve el contenido en lugar de renombrar las carpetas. En Unraid, el gestor de archivos lo hace desde la pestaña Shares.

## Falló la copia de un conjunto de datos ZFS o se saltó uno {#zfs-datasets}

Cada problema lleva un código de motivo entre corchetes, y la página [Conjuntos de datos ZFS](zfs-datasets.md#reason-codes) los lista todos con su solución. Los tres más habituales:

- **`snapshot-loop`**: la instantánea no llegó a BombVault porque Host Data no deja pasar los montajes nuevos. Edita el contenedor, pon el Access Mode de Host Data en Read/Write - Slave y reinicia BombVault.
- **`key-not-loaded`**: un conjunto de datos cifrado cuya clave no está cargada se salta. Carga la clave con `zfs load-key` y monta el conjunto de datos. La siguiente copia lo incluye.
- **`ssh-auth`**: el servidor rechazó la clave de BombVault. **Conexión ZFS con este servidor**, que se abre con **Ver conexión** en la página Copias de seguridad, muestra el comando que la autoriza. Ejecútalo una vez en el servidor.

## Un elemento se queda en "Aprendiendo N/10" {#an-entry-stays-at-learning-n10}

La mayoría de las comprobaciones de anomalías empiezan tras 10 copias correctas de un elemento, y la cuenta vuelve a empezar tras **Marcar como esperada** y tras cambiar la selección del elemento. Un elemento que no está programado no aprende, y un contenedor sin appdata no tiene de qué aprender, algo que dice su insignia. **Progreso del aprendizaje** en la página Anomalías muestra un anillo de diez partes por elemento.

## La retención dejó de borrar las copias antiguas de un elemento {#retention-stopped-deleting-old-backups-of-one-entry}

Lo impide una anomalía crítica abierta: el origen del elemento está casi vacío, se redujo mucho, o una copia guardó de nuevo la mayor parte de sus datos. Abre la anomalía desde la insignia del elemento. Si faltan datos o se cifraron, restaura primero desde la última copia buena enlazada. Luego confirma la anomalía, o márcala como esperada si el cambio fue tuyo, y la siguiente ejecución poda como de costumbre. La vista previa de retención marca un elemento así como conservado. Para un elemento ZFS solo el conjunto de datos nombrado en la anomalía conserva sus copias antiguas, y los demás conjuntos de datos del árbol se podan como de costumbre.

## La poda manual dice que se conservaron algunos elementos {#manual-prune-says-some-entries-were-kept}

La misma causa: la poda deja en paz las copias antiguas de un elemento con una anomalía así y nombra el elemento en su mensaje. Todo lo demás se poda como de costumbre.

## La importación del historial dice que no se pudo leer un repositorio {#history-import-says-a-repository-could-not-be-read}

Tras la actualización BombVault lee una vez de cada repositorio los tamaños de las copias anteriores. Un repositorio al que no pudo llegar en ese momento, como un destino externo caído o un recurso compartido sin montar, se cuenta en la tarjeta **Anomalías** de **Ajustes, Verificación** y se vuelve a intentar una vez al día. Sus elementos aprenden mientras tanto de las copias nuevas.

## El aviso de espacio en disco no coincide con el panel de Unraid {#the-disk-space-warning-does-not-match-the-unraid-dashboard}

En el recurso compartido de usuario de Unraid (`/mnt/user`) el espacio libre es el de todo el array, no el de un disco. Los destinos remotos a los que BombVault llega mediante rclone, como SFTP, la Hetzner Storage Box, SMB, WebDAV o una unidad en la nube, se miden cuando el otro lado informa de su espacio libre, y lo mismo vale para una dirección `sftp:` escrita a mano. Los repositorios S3, B2 y REST no tienen cifra: su destino dice **Tamaño desconocido**, y la tarjeta **Anomalías** los lista como no medidos.

## Un asistente de IA no puede conectar {#an-ai-assistant-cannot-connect}

La [página del servidor MCP](mcp.md#troubleshooting) lista qué significa cada código de estado y cada rechazo del endpoint MCP y qué hacer al respecto.

## El contenedor se reinicia sin parar o parece no estar sano {#the-container-keeps-restarting-or-looks-unhealthy}

BombVault informa de sano o no sano desde su propio `/api/health`. Una herramienta de autorreparación (como Autoheal) puede reiniciarlo automáticamente si el motor llegara a atascarse. Mira el registro del contenedor y la tarjeta **Integración con el host** para encontrar la causa de fondo.

## Un destino se niega a guardarse {#location-refused}

BombVault comprueba un destino antes de guardarlo, y el rechazo dice por qué en una frase. Los habituales:

- **"Esta ubicación está dentro de otro repositorio o destino, o contiene uno."** Un repositorio dentro de un repositorio estropearía a los dos. El mensaje nombra el otro lugar. Elige otra carpeta, o quita antes aquel.
- **Es un repositorio que configuraste.** El destino es una de las filas de **Repositorios**, o está dentro o alrededor de una. Elige otro lugar, o quita antes ese repositorio.
- **El mismo lugar que la carpeta propia de una sección.** Una copia en la carpeta propia de una sección copiaría sus copias sobre sí mismas en cada ejecución, así que se rechaza. Elige otro lugar, o consulta [La carpeta propia de una sección](offsite-recovery.md#own-folder).
- **"That is not an off-site target of this domain"** vino de un enlace a un destino que se había quitado. Una versión actual los limpia al arrancar y en una importación. Si aún lo ves, actualiza BombVault.
- **Una ruta completa del host** como `/mnt/user/backups`. BombVault se ejecuta en un contenedor, así que quiere la ruta por debajo de Host Data, `user/backups`, y el mensaje nombra la que hay que introducir.

## Un destino remoto dice que su repositorio ha desaparecido {#repository-gone}

Un destino que contenía copias y de repente no contiene ningún repositorio no recibe uno nuevo y vacío por sí solo, porque ese ocultaría al verdadero cuando vuelva un disco. Un rest-server cuyo disco no se montó es la causa habitual, así que vuelve a montar el disco y ejecuta de nuevo la copia o la réplica. Solo si vaciaste o borraste el repositorio antiguo a propósito, pulsa **Crear aquí un repositorio nuevo** en la página del destino. Empieza de nuevo para todas las secciones de ese destino a la vez.

## Caducó un inicio de sesión en la nube {#cloud-sign-in-expired}

La prueba o la siguiente copia dice entonces que el inicio de sesión caducó. **Cambiar la conexión** en la página del destino inicia sesión de nuevo.

- **Box, Jottacloud y otros con tokens de refresco de un solo uso:** BombVault conserva el token que rclone renovó, también tras reinicios y en el kit de recuperación. Tras una restauración desde una copia de ajustes más antigua, el token que contiene puede haber caducado, así que inicia sesión una vez más. El repositorio se queda como está.
- **iCloud Drive:** Apple termina el inicio de sesión a los 30 días. Necesita un nuevo token de confianza y cookies de `rclone config`.
- **OneDrive:** un inicio de sesión con varias unidades pregunta cuál usar, así que un destino que se deja sin elegir sigue inutilizable hasta que elijas. Para OneDrive Business y Google Drive consulta la nota sobre datos podados en [Destinos](offsite-recovery.md#destinations).

## "The off-site copy of the last backup is still running" {#copy-still-running}

La copia anterior terminó y su copia externa aún está en camino, lo que bloquea una copia nueva de esa sección. Espera hasta que la cabecera de estado ya no diga **Copia externa en curso**. Los botones que inician una copia dicen lo mismo mientras esperan. La primera copia a un destino nuevo puede tardar horas, consulta el [ejemplo de S3](offsite-recovery.md#example-s3).

## Una réplica ZFS se detiene porque el destino tiene instantáneas más nuevas {#replica-target-newer}

El destino tiene instantáneas más nuevas que la más reciente que comparte con este servidor, por ejemplo porque el conjunto de datos de aquí se revirtió o la copia del destino se modificó. La tarjeta Réplica y el historial de ejecuciones nombran la instantánea en **Revertir el destino a**. Ejecuta `zfs rollback -r` hasta esa instantánea en el destino, para cada conjunto de datos que nombre la ejecución, y pulsa **Replicar ahora**. Consulta [Cuánto tiempo se conservan las instantáneas](zfs-datasets.md#replica-retention).

## Una VM con instantáneas de Unraid falla y nombra un archivo {#vm-chain}

BombVault sigue la cadena de respaldo de cada disco y copia todos los archivos de ella. Una cadena que no se puede seguir hasta el final hace fallar la copia en lugar de dejar fuera parte del disco, y el mensaje nombra el archivo. Comprueba que el archivo existe bajo `/mnt` y que la instantánea sigue listada en el VM Manager. Un archivo fuera de Host Data no se puede leer en absoluto. Consulta [VMs con instantáneas de Unraid](vm-backup-ssh-setup.md#unraid-snapshots).

## Las copias externas están en pausa tras una reconstrucción {#copies-paused}

Tras **Descubrir copias** o el Asistente de emergencia sin ajustes restaurados, las copias externas de cada sección reconstruida esperan, porque BombVault ya no sabe qué elementos dejaste fuera. El resumen lo dice. Pulsa **Confirmar valor predeterminado** en **Destinos por sección**, revisa la vista previa y deja fuera lo que deba quedar fuera. Importar un archivo de ajustes no termina la pausa. Consulta [Después de una reconstrucción](offsite-recovery.md#after-a-rebuild).

## Las notificaciones no llegan {#notifications-do-not-arrive}

Empieza con **Enviar prueba** en **Ajustes, Notificaciones**. La línea bajo la tarjeta dice qué pasó.

- **No se envía nada en absoluto.** Comprueba **Notificar**. Con **Nunca** no sale nada, y con **Solo en caso de fallo** una ejecución buena se queda en silencio. Con **Resumir las ejecuciones programadas** activado, una ejecución programada envía un resumen al final.
- **Una dirección nueva.** Tras cambiar el servidor de Matrix, Gotify o el correo, el campo vuelve a pedir el token o la contraseña, y la dirección nueva se guarda solo cuando lo escribes. BombVault no envía el secreto antiguo a un sitio nuevo.
- **Gotify** dice que el servidor rechazó el token de la aplicación: cópialo de nuevo de la aplicación en Gotify. La **URL del servidor** es la dirección base, sin `/message`.
- **Webhook:** comprueba que el **Formato del Webhook** encaje con el servicio. Una dirección de Discord necesita **Discord**, y ntfy necesita **ntfy**.
- **Las notificaciones de Unraid** funcionan por la conexión SSH del host, así que prueba antes eso en **Ajustes, Integraciones**. En otra plataforma se quedan desactivadas, y la tarjeta lo dice.
- **Nunca llega un correo de notificación.** Comprueba **Cifrado** contra el puerto, normalmente **STARTTLS** en el 587 o **TLS (implicit)** en el 465.

Los pings de Healthchecks no dependen de nada de esto, consulta [Healthchecks](notifications.md#healthchecks).

## La app de Android no puede abrir un servidor {#android-cannot-open}

La pantalla de inicio dice por qué en la tarjeta del servidor:

- **Sin conexión:** no responden ni la dirección del servidor ni el repetidor. Con **Sin repetidor**, la app llega a un servidor solo en su propia dirección.
- **Comprobar certificado:** el servidor muestra otro certificado distinto del que habías aceptado. La app solo lo abre después de que confíes en el nuevo, así que asegúrate de que el cambio es tuyo.
- **Inicio de sesión necesario:** un servidor añadido por su dirección pide la contraseña. Consulta también [Un teléfono acaba en la página de inicio de sesión](#a-phone-lands-on-the-login-page).
- **La clave cambió:** la pantalla de inicio muestra la huella de la clave nueva, y la página solo se abre después de que confíes en ella. Consulta [Fuera de casa](android.md#away).

## Otro usuario no puede leer el repositorio local {#repository-permissions}

El repositorio local sigue siendo legible para otros usuarios, como un usuario de sincronización por SMB. Antes de la 10.4.1, los archivos que escribía una poda, una etiqueta, una recogida, un movimiento de sección, un borrado o una copia de un stack podían quedar ilegibles hasta la noche siguiente. Actualiza BombVault y son legibles al instante. El repositorio de la autocopia sigue siendo legible solo para root, a propósito, porque contiene los ajustes de BombVault.

## ¿Sigues atascado? {#still-stuck}

- Lee las páginas completas de [Configuración](configuration.md) y [Copias externas y recuperación](offsite-recovery.md).
- **Informe de error** en la tarjeta **Ayuda** de **Ajustes, Info** descarga un ZIP para una incidencia de GitHub: la comprobación del host, tu configuración sin ningún secreto, las ejecuciones recientes, lo que está programado a continuación y el registro reciente desde la última vez que arrancó el contenedor. También contiene el último volcado de cada base de datos, los elementos ZFS con sus montajes, las anomalías abiertas y cuántas claves de MCP existen. Las contraseñas, los tokens y la configuración de rclone se eliminan, y el paquete lo dice en su propio manifiesto. Requiere una contraseña de acceso. Para un fallo que reinició el contenedor, mira `docker logs`.
- Pregunta en el [hilo de soporte de Unraid](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/).
- Abre una [incidencia en GitHub](https://github.com/junkerderprovinz/bombvault/issues).
