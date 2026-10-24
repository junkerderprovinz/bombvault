# Resolución de problemas

Un breve FAQ. Para la tabla completa de resolución de problemas del lado del host de la copia de VM por SSH (permiso denegado, verificación de clave de host, variables de plantilla faltantes y más), consulta la [guía de copia de VM por SSH](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) en GitHub.

## Algo no está bien conectado

Añade la tarjeta **Integración con el host** al resumen en **Personalizar**. La comprobación de integración con el host sondea cada montaje y CLI (socket de Docker, libvirt, restic, qemu-img, rclone) e informa de cualquier pieza que falte, y **Comprobar ahora** la ejecuta de nuevo. Empieza aquí antes de asumir que es un error: un montaje faltante o un host inalcanzable aparece de inmediato.

## No puedo alcanzar la interfaz web

BombVault sirve HTTPS de fábrica en el puerto `3443` (certificado autofirmado), así que abre `https://<your-unraid-ip>:3443`. Acepta el aviso de certificado autofirmado, o pon BombVault detrás de un proxy inverso con tu propio certificado. Si lo ejecutas con `HTTP_ONLY=true`, sirve HTTP en texto plano en el puerto `3000` en su lugar (pensado para uso detrás de un proxy que termina TLS).

## Perdí mi APP_KEY

`APP_KEY` deriva la contraseña del repositorio restic. Sin ella, sin el kit de recuperación y sin un depósito de claves en tu grupo, las copias cifradas no se pueden recuperar. Por eso el resumen muestra **Guarda tu kit de recuperación** hasta que lo descargas. Consulta [Copia externa y recuperación](offsite-recovery.md#recovery-kit). Genera una clave con `openssl rand -hex 32` y guárdala fuera del servidor antes de confiar en ninguna copia.

## Olvidé la contraseña de inicio de sesión

Elige **¿Has olvidado la contraseña?** en la página de inicio de sesión. BombVault escribe un código de 12 caracteres en el registro de su contenedor (en Unraid: pestaña Docker, icono de BombVault, **Logs**). Escríbelo con una contraseña nueva en 10 minutos. El restablecimiento también desactiva la verificación en dos pasos, elimina todas las claves de acceso, revoca todos los tokens de API, claves MCP e inicios de sesión OAuth, cierra todas las demás sesiones y desactiva el emparejamiento de móviles. Vuelve a añadir tus claves de acceso en **Ajustes, Seguridad**, activa de nuevo ahí el emparejamiento de móviles y crea tokens y claves nuevos donde aún los necesites. Consulta [Modelo de seguridad](configuration.md#login).

## Las copias se detuvieron y dicen que la clave ya no se puede abrir

Este servidor adoptó la clave de copias de otro servidor en el Asistente de emergencia, y después se cambió `APP_KEY`. La clave adoptada está sellada con la antigua APP_KEY, así que BombVault no puede abrirla y no copia nada en lugar de escribir con la clave equivocada. **Necesita atención** en el resumen la nombra. Vuelve a poner APP_KEY como estaba, o sube de nuevo el kit de recuperación en la página Restauración, en **Emergencia**.

## Un móvil llega a la página de inicio de sesión

Un móvil emparejado recibe su sesión a través del grupo solo mientras **Los teléfonos pueden emparejarse con las doce palabras del grupo** esté activado en **Ajustes, Seguridad**. Un restablecimiento de la contraseña lo desactiva. Una app más antigua que el servidor tampoco recibe sesión hasta que se actualiza. Un servidor añadido por su dirección siempre pide la contraseña. Un móvil eliminado de la página de Instancias regresa desactivado cuando su app vuelve a abrirse; activa **Consultar esta instancia** en **Supervisión** dentro de sus detalles para dejarlo entrar.

## La copia de VM no conecta

La copia de VM habla con libvirt por SSH, nunca con un montaje.

- Confirma que SSH está habilitado en el host y que la clave pública de BombVault está autorizada en `/root/.ssh/authorized_keys` (Ajustes, Integraciones, SSH del host muestra la clave y un botón **Probar conexión**).
- En una red `br0.x` personalizada, establece `LIBVIRT_HOST` a la IP LAN de tu Unraid (el contenedor no puede alcanzar el host mediante `host.docker.internal` ahí). Habilita **Settings, Docker, Host access to custom networks**.
- Si cambiaste el puerto SSH de Unraid, establece `LIBVIRT_SSH_PORT` para que coincida.
- En TrueNAS, define `LIBVIRT_URI` con el socket libvirt propio de TrueNAS, da a la aplicación la capacidad `SETPCAP` y añade la clave a root en **Credentials, Users** (consulta [TrueNAS SCALE](getting-started.md#truenas)).
- El diagnóstico paso a paso completo (prueba de accesibilidad, enrutamiento VLAN, `Permission denied (publickey)`, `Host key verification failed`) está en la [guía de copia de VM por SSH](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md).

## Una instantánea de VM en vivo no se ejecutó

Las instantáneas en vivo necesitan el agente invitado de qemu instalado en la VM y el disco en `/mnt/cache` (o `/mnt/diskX`), no en `/mnt/user`. En una VM apagada, la modalidad en vivo recurre automáticamente a la ordenada. Una copia ordenada apaga la VM, copia los discos y luego la reinicia, de modo que siempre es consistente. Espera hasta diez minutos a que el invitado se apague y nunca lo fuerza a apagarse, para que un invitado Windows que está instalando actualizaciones no se corte a medias. Un invitado que sigue en marcha después de ese tiempo se deja tranquilo, y esa copia falla con el motivo.

## Una copia falló con "repository is already locked"

Suele tratarse de un bloqueo de restic huérfano dejado atrás cuando el contenedor se actualizó o reinició a mitad de operación. BombVault detecta un bloqueo huérfano de forma demostrable, lo fuerza a limpiar y lo reintenta una vez, automáticamente. Si persiste, abre la tarjeta **Comprobar las copias** en **Ajustes, Verificación** y elige **Desbloquear** bajo **Más** en la fila de la sección afectada para limpiar un bloqueo obsoleto a mano. Un problema genuino sigue saliendo a la superficie en lugar de quedar oculto. Tras un reinicio, BombVault espera a que ese bloqueo lleve diez minutos sin renovarse. Un restic que sigue en marcha, por ejemplo en un segundo BombVault sobre el mismo repositorio, renueva su bloqueo cada cinco minutos.

## Mi copia externa no ocurrió tras una copia

La replicación externa es de mejor esfuerzo por diseño, de modo que un contratiempo externo nunca hace fallar la copia local. Comprueba cuándo copia el destino: su tarjeta **Copiar** dice **Después de cada copia** o nombra un plan propio, y **Ajustes, Calendario, Copias externas por sección** puede dar a una sección su propio plan. Usa **Replicar ahora** en la tarjeta **Copia externa** de la sección, en **Ajustes, Destinos**, para una ejecución bajo demanda, y observa la cabecera de estado del resumen, que dice **Copia externa en curso** mientras dura. Una copia fallida aparece en **Necesita atención**.

## No se confía en el certificado de un rest-server

restic rechaza un certificado que no conoce, y el error dice dónde ponerlo. Copia el certificado autofirmado del rest-server, o el de la autoridad que lo firmó, como archivo PEM en la carpeta `certs` de la carpeta de datos de BombVault (`/config/certs` en el contenedor, `appdata/bombvault/certs` en Unraid) y vuelve a probar. No hace falta reiniciar.

## Una copia dice que los datos fuera de Host Data no se copiaron {#data-outside-host-data}

El contenedor guarda datos en una carpeta que BombVault no puede leer a través de Host Data. La copia guarda la definición y cada carpeta a la que llega, pinta de naranja su línea en el registro de actividad y nombra las carpetas que dejó fuera. Hay dos casos habituales: un ixVolume en TrueNAS bajo `/mnt/.ix-apps` y un volumen anónimo de Docker bajo `/var/lib/docker/volumes`, que Docker crea por su cuenta para una carpeta que la imagen declara y que nadie ha mapeado, como los datos de un agente de monitorización o la caché de un servidor multimedia.

- **Quieres copiar esos datos.** Mapea la carpeta a una ruta del host bajo Host Data, por ejemplo una carpeta de tu recurso appdata, y la próxima copia la incluye.
- **No los necesitas.** Añade la ruta de la carpeta dentro del contenedor, como `/var/lib/beszel-agent` o `/cache`, como una línea en la tarjeta **Patrones de exclusión** de la pestaña **Ajustes** del contenedor. A partir de la próxima copia el aviso deja fuera esa carpeta, y la vista previa bajo el campo dice por qué. Una línea para una carpeta superior también sirve, un nombre simple no.

## Una copia grande se terminó por quedarse sin memoria

Ajusta **Límite de memoria por copia (MiB, 0 = ninguno)** en la tarjeta **Rendimiento** de **Ajustes, Sistema**, bastante por debajo de lo que puede usar el contenedor. Cerca del límite restic libera memoria más a menudo, lo que cuesta algo de velocidad, así que la copia va más lenta en lugar de terminarse.

## Una restauración se abortó antes de empezar

Antes de detener o eliminar nada, la restauración ejecuta una comprobación de conflictos previa al arranque: verifica que la IP estática del contenedor y los puertos publicados del host estén libres. Si otro contenedor ya ocupa uno, aborta con un mensaje claro y accionable en lugar de dejar una restauración a medias. Libera el puerto o la IP en conflicto y vuelve a intentarlo.

Si la red del contenedor no existe, por ejemplo porque se borró su aplicación de TrueNAS, la comprobación antes de restaurar nombra la red y la restauración se detiene antes de cambiar nada. Reinstala la aplicación en TrueNAS con el mismo nombre, lo que vuelve a crear la red, y luego restaura sus datos.

## Una exportación sencilla falló en lugar de escribir un archivo

Si **Cifrar exportaciones con age** está activado en **Ajustes, Sistema** pero no hay ningún destinatario válido definido, una exportación falla con un error claro en lugar de escribir texto plano. Añade un destinatario válido (una clave pública age o una clave pública SSH), o desactiva el cifrado si quieres que la exportación sea en texto plano. Consulta [Funciones](features.md).

## Un volcado de base de datos falló

Un volcado fallido nunca hace fallar la copia que lo rodea; queda registrado como una ejecución fallida propia, y el motivo dice qué arreglar.

- **Acceso rechazado.** El volcado entra con las variables de contraseña del propio contenedor (`POSTGRES_PASSWORD`, `MARIADB_ROOT_PASSWORD`, `MYSQL_ROOT_PASSWORD` o sus versiones `_FILE`). Revísalas en el contenedor de la base de datos. Una variable `_FILE` que apunta a un secreto que el usuario del contenedor no puede leer falla igual.
- **Faltan privilegios.** Con una contraseña de root aleatoria, el volcado solo puede entrar como usuario de la aplicación, así que contiene esa única base, y MySQL 8.4 y posteriores pueden rechazarlo del todo. Dale al contenedor una contraseña de root de verdad, o apágale el volcado.
- **Las tablas del sistema necesitan una actualización.** MariaDB se niega a volcarse cuando sus tablas del sistema vienen de una versión anterior (error 1558). Añade la variable `MARIADB_AUTO_UPGRADE=1` y reinicia el contenedor, o ejecuta `mariadb-upgrade` dentro una vez.
- **Sin herramienta de volcado.** Una imagen ligera o hecha a mano sin `pg_dump`, `mysqldump` ni `mariadb-dump` no se puede volcar. Usa la imagen oficial, o apaga el volcado.
- **Un límite de tiempo.** Un volcado tiene `DB_DUMP_MAX_HOURS` (6 por defecto), la copia que lo rodea tiene `BACKUP_MAX_HOURS`, y un volcado que deja de avanzar se corta tras `BACKUP_STALL_HOURS`. Lo último suele deberse a un bloqueo que mantiene la aplicación. Sube el límite que saltó, o vuelca mientras la aplicación está tranquila.
- **El contenedor está pausado o reiniciándose.** El volcado habla con el servidor en marcha. Si el contenedor se reinicia una y otra vez, su propio registro dice por qué.
- **Un volcado dañado no se pudo eliminar.** Un volcado que BombVault no pudo terminar se borra. Cuando ese borrado falla, el volcado se queda en la lista marcado como dañado y puedes eliminarlo ahí.

## Una importación falló

Una importación para el contenedor, aparta su carpeta de datos y deja que la imagen cree una vacía en su lugar. Si falla un paso previo a la importación en sí, la carpeta antigua se devuelve sola. Si falla la importación, el contenedor se queda con la carpeta nueva y la antigua permanece al lado como `<carpeta de datos>.bombvault-before-import-<marca de tiempo>`; el mensaje de error de la ejecución nombra la ruta exacta.

Una carpeta de datos que es un conjunto de datos propio, como en TrueNAS, no se puede renombrar. Entonces la importación deja la carpeta en su sitio y mueve su contenido a la carpeta guardada junto a ella, así que el conjunto de datos de arriba necesita espacio para él. La importación lo comprueba antes de parar el contenedor.

Para devolverla a mano: para el contenedor, renombra la carpeta de datos actual para quitarla de en medio, renombra la carpeta guardada a su nombre original y arranca el contenedor. Con un conjunto de datos, mueve el contenido en lugar de renombrar las carpetas. En Unraid, el gestor de archivos de la pestaña Shares hace esto.

## Una copia de un conjunto de datos ZFS falló u omitió un conjunto {#zfs-datasets}

Cada problema lleva un código de motivo entre corchetes, y la página [Conjuntos de datos ZFS](zfs-datasets.md#reason-codes) los enumera todos con su solución. Los tres más habituales:

- **`snapshot-loop`**: la instantánea no llegó a BombVault porque Host Data no transmite los montajes nuevos. Edita el contenedor, pon el Access Mode de Host Data en Read/Write - Slave y reinicia BombVault.
- **`key-not-loaded`**: un conjunto cifrado cuya clave no está cargada se omite. Carga la clave con `zfs load-key` y monta el conjunto; la siguiente copia lo incluye.
- **`ssh-auth`**: el servidor rechazó la clave de BombVault. **Conexión ZFS con este servidor**, que se abre con **Ver conexión** en la página Copias de seguridad, muestra el comando que la autoriza; ejecútalo una vez en el servidor.

## Un elemento se queda en "Aprendiendo N/10"

La mayoría de las comprobaciones de anomalías empiezan tras 10 copias correctas de un elemento, y la cuenta vuelve a empezar tras **Marcar como esperada** y tras cambiar la selección del elemento. Un elemento sin programación no aprende, y un contenedor sin appdata no tiene de qué aprender, como indica su insignia. **Progreso del aprendizaje** en la página Anomalías muestra un anillo de diez partes por elemento.

## La retención dejó de borrar las copias antiguas de un elemento

Una anomalía crítica abierta las retiene: la fuente del elemento está casi vacía, encogió mucho, o una copia volvió a guardar la mayoría de sus datos. Abre la anomalía desde la insignia del elemento. Si faltan datos o se cifraron, restaura primero desde la última copia buena enlazada. Después confirma la anomalía, o márcala como esperada si el cambio fue tuyo, y la siguiente ejecución limpia como siempre. La vista previa de retención marca ese elemento como conservado. En un elemento ZFS solo el conjunto de datos que nombra la anomalía conserva sus copias antiguas; los demás conjuntos del árbol se limpian como siempre.

## La limpieza manual dice que se conservaron algunos elementos

La misma causa: la limpieza no toca las copias antiguas de un elemento con una anomalía así y lo nombra en su mensaje. Todo lo demás se limpia como siempre.

## La importación del historial dice que no se pudo leer un repositorio

Tras la actualización, BombVault lee una vez el tamaño de las copias anteriores de cada repositorio. Un repositorio que no estaba accesible en ese momento, como un destino externo caído o un recurso compartido sin montar, aparece en la tarjeta **Anomalías** de **Ajustes, Verificación** y se reintenta una vez al día. Mientras tanto, sus elementos aprenden de las copias nuevas.

## El aviso de espacio en disco no coincide con el panel de Unraid

En el recurso compartido de usuario de Unraid (`/mnt/user`) el espacio libre es el de todo el array, no el de un disco. Los repositorios remotos solo se miden mediante remotos de rclone que informan de su espacio libre; los repositorios S3, B2, REST y SFTP no tienen dato y aparecen como no medidos en la tarjeta **Anomalías**.

## Un asistente de IA no consigue conectarse

La página [Servidor MCP](mcp.md#troubleshooting) explica qué significa cada código de estado y cada rechazo del punto de conexión MCP, y qué hacer en cada caso.

## El contenedor se reinicia constantemente o parece no saludable

BombVault informa de saludable/no saludable desde su propio `/api/health`. Una herramienta de autorreparación (como Autoheal) puede reiniciarlo automáticamente si el motor se atasca alguna vez. Comprueba el registro del contenedor y la tarjeta **Integración con el host** para conocer la causa subyacente.

## ¿Sigues atascado?

- Lee las páginas completas de [Configuración](configuration.md) y [Copia externa y recuperación](offsite-recovery.md).
- Pregunta en el [hilo de soporte de Unraid](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/).
- Abre una [incidencia en GitHub](https://github.com/junkerderprovinz/bombvault/issues).
