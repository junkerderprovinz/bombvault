# Copia externa y recuperación

!!! note "Las copias externas esperan tras una reconstrucción"
    Cuando el paso 4 reconstruye entradas sin los ajustes antiguos, la replicación externa de esos dominios se pausa hasta que se confirme el valor predeterminado de ubicación. Consulta [Ubicación por elemento](#placement).

Las copias locales te protegen de un contenedor perdido o de una mala actualización. La replicación externa y un kit de recuperación probado te protegen de la pérdida de toda la máquina, del ransomware o de un incendio. Esta página cubre la replicación externa, cómo hacer esa copia a prueba de manipulaciones, cómo demostrar que puedes restaurar y cómo recuperarte cuando el propio BombVault ha desaparecido.

## Replicación externa

Conserva la copia local rápida y añade una o varias réplicas externas. Define un repo por dominio en la página **Ajustes, Externo**. BombVault replica ahí las nuevas instantáneas con `restic copy` en modo de mejor esfuerzo, de modo que un contratiempo externo nunca hace fallar la copia local. En esta forma el repo local sigue siendo el primario y el repo externo es una réplica, pero el repo primario de un dominio no tiene por qué ser local; consulta [Repositorios primarios remotos](#remote-primary-repositories) más abajo para copiar directamente a S3, rest-server, etc. en lugar de replicar hacia allí.

- **Varios destinos externos por dominio.** Cada dominio (contenedores, VMs, flash, config, conjuntos de archivos y conjuntos de datos ZFS) puede replicarse a varios destinos externos a la vez, no solo a uno, de modo que puedes mantener, por ejemplo, un rest-server en la máquina de un amigo y un bucket S3 en paralelo. Añade destinos adicionales en Ajustes, Externo, cada uno con su propio repositorio, clase de almacenamiento S3, marca append-only, retención y presupuesto de crecimiento. Una configuración externa única existente se traslada intacta como el primer destino, y cada destino de un dominio se replica según el calendario externo de ese dominio.
- **Calendario externo por dominio** (editado junto a todos los demás calendarios en Ajustes, Programaciones): déjalo en blanco para replicar tras cada copia local, o establece una cadencia (por ejemplo `weekly Sun 03:00`) para enviar fuera del sitio con menos frecuencia de la que copias localmente. Un botón **Replicar ahora** cubre las ejecuciones bajo demanda.
- La **retención externa** vive en Ajustes, Retención para que puedas conservar las copias externas más tiempo como archivo. Deja la política toda a cero para no recortar nunca automáticamente las instantáneas externas.
- Los **límites de ancho de banda** (Ajustes, Externo) limitan la velocidad de subida/bajada de restic para que la replicación no sature tu WAN.
- Un **indicador de replicación** muestra qué dominio se está replicando mientras se ejecuta (en su página y en el Panel). Es un indicador activo, no una barra de porcentaje, porque `restic copy` no expone ningún progreso legible por máquina.

!!! note "Restaurar desde cualquier lugar"
    Cada contenedor, VM, conjunto de archivos, el flash y la configuración de la app listan sus copias de seguridad como una única línea de tiempo a través de todos los lugares donde reside una copia. Una copia enviada a B2 aparece una sola vez, marcada con cada lugar que la tiene. Una restauración toma el primer lugar al que puede llegar, empezando por el repositorio en el que se escribe el elemento, y puedes elegir otro lugar por fila. Los lugares externos solo se leen cuando los abres. Borrar en un lugar comprueba primero los demás y dice si era la última copia.

## Ubicación por elemento {#placement}

Cada tarjeta de contenedor, VM y conjunto de archivos tiene una fila de **Ubicación** con tres segmentos:

- **Local** escribe el elemento en el repositorio que se muestra bajo **Almacenado en** y no lo copia a ningún sitio. Úsalo para datos que ya tienen una segunda copia, por ejemplo un recurso compartido que vive en un NAS.
- **Local + externo** lo escribe también ahí y lo copia a los destinos marcados bajo **Copiar a**, un chip por cada destino externo del dominio. Desmarca un chip y ese destino no recibe nada nuevo de este elemento.
- **Solo externo** escribe el elemento directamente en el lugar bajo **Enviar a**: un repositorio directo junto a un destino externo, o un repositorio remoto que configuraste en Ajustes, Almacenamiento, Repositorios.

La ubicación queda fija desde la primera copia de seguridad del elemento, porque BombVault nunca mueve copias entre repositorios. Las copias pueden cambiar en cualquier momento. Un destino que deja de recibir un elemento conserva las copias que tiene y las recorta a su propia retención en la siguiente ejecución externa del dominio; **Borrar en B2** en la tarjeta las elimina de inmediato. Cuando algunas de esas copias no existen en ningún otro lugar, la confirmación las enumera por fecha y pide el nombre del elemento. De los destinos de solo añadir no se puede borrar.

Bajo la fila, la tarjeta dice adónde va el elemento y qué hay realmente ahí: cuántas sedes lo tienen, cuándo se vio cada destino por última vez, y si se cumple 3-2-1. Una sede es el servidor con los datos originales, cada destino externo y cada repositorio marcado **Fuera del local**. BombVault comprueba copias y sedes; no comprueba la parte de "dos soportes" de 3-2-1.

### Valores predeterminados de ubicación

Ajustes, Almacenamiento, **Valores predeterminados de ubicación** tiene una fila por dominio con los mismos tres segmentos. Las copias se aplican de inmediato a cada elemento sin elección propia, y a las carpetas de proyecto de las pilas de Compose. La ubicación se aplica a un elemento nuevo en su primera copia de seguridad; cambiarla no mueve ninguna copia. Antes de guardar, la fila nombra cada destino que gana o pierde elementos y cuántas instantáneas supone eso. **Aplicar a elementos sin copias de seguridad** devuelve al valor predeterminado a todo elemento que aún no tiene copia de seguridad.

Un destino externo nuevo recibe todo elemento que no esté en Local. El diálogo que lo añade dice cuántos elementos son y, cuando se sabe, cuánto historial supone eso, y ofrece dejar fuera los elementos ya excluidos de otros destinos.

### Repositorios directos

Elegir el repositorio directo de un destino bajo Solo externo abre un diálogo con una ubicación sugerida junto al destino, por ejemplo `s3:https://s3.eu-central-003.backblazeb2.com/bucket/containers-direct`, y una prueba de conexión que no crea nada. **Crear y usar** crea el repositorio y apunta el elemento a él. Un repositorio directo toma la clave, la clase de almacenamiento, los límites, la configuración de solo añadir y la retención del destino, y cambia con ellos; la tarjeta Repositorios lo muestra de solo lectura. Cuando una clave nueva del destino no puede abrirlo, el repositorio directo conserva la clave que tiene y el guardado lo dice. Sus instantáneas llevan la etiqueta `bv:direct`, y las demás pasadas de retención las conservan, así que un repositorio directo que perdió su enlace con su destino nunca envejece por las reglas locales. Se accede a B2 a través de su punto de conexión S3, introduciendo el ID de clave y la clave de aplicación como credenciales de S3; una clave limitada a la carpeta propia del destino no puede alcanzar la carpeta contigua a ella, así que limita la clave a la carpeta superior al destino en su lugar.

### Fuera del local

Un repositorio con nombre puede marcarse como **Fuera del local** en la tarjeta Repositorios. Los repositorios remotos empiezan marcados; desactívalo para un rest-server en el mismo edificio. La marca solo cuenta para las sedes y el 3-2-1 en las tarjetas. No cambia ninguna copia.

### Tras una reconstrucción

Las elecciones de copia viven en los propios ajustes de BombVault. Tras una reconstrucción mediante Descubrir copias sin un `/config` restaurado, desaparecen, y copiar todo enviaría de nuevo a B2 los elementos que habías dejado fuera. Por eso la replicación externa de cada dominio reconstruido se pausa. El Panel lo muestra en ámbar, y Valores predeterminados de ubicación ofrece **Confirmar valor predeterminado** con una vista previa de lo que copia la siguiente ejecución y los nombres en las copias de seguridad que no tienen entrada, que puedes dejar fuera ahí. Solo la confirmación termina la pausa; importar un archivo de ajustes trae de vuelta reglas y valores predeterminados pero no la termina.

## Repositorios primarios remotos {#remote-primary-repositories}

La ruta de copia de un dominio (Ajustes, Almacenamiento) no se limita a una carpeta local: apúntala directamente a un remoto de restic (`s3:...`, `rest:http://host:8000/repo`, `sftp:usuario@host:/repo`, `rclone:remoto:bucket/ruta`) y BombVault copia allí directamente, sin copia local aparte y sin paso de replicación. Es una forma realmente distinta de la replicación fuera de sede de más arriba: allí el repositorio local es el primario y el de fuera de sede es un archivo suyo en la medida de lo posible; aquí el repositorio remoto **es** el primario, y es la única copia mientras no configures además una replicación fuera de sede (o un segundo remoto) para ese dominio.

Cada uno de los seis campos de ruta (Contenedores, VMs, Flash, Autocopia, Carpetas, Conjuntos de datos ZFS) lleva justo al lado un conmutador **Local / Remoto**:

- **Local** muestra el explorador de carpetas de siempre.
- **Remoto** lo cambia por un campo de URL sencillo, más un botón que abre el mismo diálogo de prueba de conexión y credenciales que usan los destinos fuera de sede, configurado para este primario. Desde ahí obtienes:
    - **Una prueba de conexión** contra la ruta real, antes de confiar en ella.
    - **Límites de ancho de banda** (subida y bajada) para que una copia programada hacia un primario remoto no sature tu enlace WAN: los mismos parámetros de restic `--limit-upload` y `--limit-download` que usa la replicación fuera de sede, aplicados a la propia copia.
    - **Protección append-only (inmutabilidad)**, verificada con la misma prueba activa de manipulación (una sonda DELETE real contra el otro extremo) que reciben los destinos fuera de sede. Con ella activada, BombVault se niega a podar el repositorio: como detrás no hay copia local aparte, las credenciales de esta máquina no deben poder borrar la única copia de la copia de seguridad.
    - **Una alarma de presupuesto de crecimiento**, tomada de la misma tendencia de tamaño del repositorio que la tarjeta Almacenamiento ya sigue.

Nada de esto es obligatorio: una ruta remota escrita a mano y sin ajustes de seguridad guardados copia exactamente como siempre (ancho de banda ilimitado, podable, sin alarma de presupuesto). El diálogo de seguridad está ahí para cuando quieras las mismas protecciones que recibe una copia fuera de sede, sin tener que crear un destino fuera de sede solo para eso.

!!! note "Las credenciales de nube y REST se comparten"
    Un primario remoto se autentica con las mismas credenciales S3/REST configuradas en Ajustes, Acceso a la nube, Credenciales de nube compartidas. No hay un almacén de credenciales aparte para los repositorios primarios.

### SMB y WebDAV sin montaje en el host {#smb-webdav}

Ajustes, Acceso a la nube, rclone tiene un formulario para un recurso compartido de Windows o Samba y para un servidor WebDAV (Nextcloud, ownCloud, SharePoint o cualquier otro). Rellena un nombre corto, el host y el recurso compartido (SMB) o la URL y el tipo de servidor (WebDAV), el usuario y la contraseña, y BombVault escribe la sección de rclone por ti. rclone ofusca la contraseña por su cuenta antes de guardarla; añadir un destino con un nombre que ya existe sustituye esa sección en lugar de añadir una segunda.

El formulario responde con la ubicación final, por ejemplo `rclone:nas:backups`. Ponla en una Ruta de copia o en un destino externo y añade una subcarpeta si quieres (`rclone:nas:backups/bombvault`). El recurso compartido es el primer segmento de la ruta, no parte del nombre.

Esta es mejor vía que montar el recurso compartido en Unraid: restic desaconseja tener un repositorio en un recurso CIFS montado, y aquí no se monta nada. NFS no está en el formulario porque ni restic ni rclone tienen un backend NFS; para NFS, monta la exportación en el host y apunta una Ruta de copia a ella.

## Externo inmutable (append-only)

Marca un repo externo como append-only para que el ransomware, o un host comprometido, no puedan eliminar ni reescribir tus copias. El otro extremo (un `restic/rest-server` ejecutándose en modo `--append-only`) lo **impone**. BombVault solo lo **verifica** y nunca muestra verde basándose únicamente en una afirmación de configuración.

El asistente de **configuración externa guiada** te lleva desde la elección del backend (rest-server / rclone / S3), pasando por un fragmento de despliegue de rest-server listo para pegar, una prueba de conexión, el conmutador inmutable (que ejecuta la prueba de manipulación de inmediato) y una estrategia de retención, de modo que el externo append-only es alcanzable sin editar configuraciones a mano.

!!! note "Un borrado correcto en `/locks/` es lo esperado"
    Append-only no significa que ya no se pueda borrar nada. restic tiene que tomar y liberar sus propios bloqueos, así que `/locks/` sigue siendo escribible y borrable a propósito. Las instantáneas y los datos que hay detrás, que es justo lo que buscaría un ransomware, no se pueden eliminar. Si compruebas tú mismo el lado remoto, un borrado que funcione bajo `/locks/` es el comportamiento correcto y no un agujero.

!!! warning "Los repos inmutables nunca se podan desde esta máquina"
    Un externo inmutable nunca poda deliberadamente las instantáneas antiguas. Establece para él una **alarma de presupuesto de crecimiento** para que recibas un aviso antes de que el tamaño del repo se descontrole.

## Prueba de manipulación

BombVault demuestra periódicamente la garantía append-only intentando realmente un borrado contra el repo externo, dirigido a un objeto inexistente:

- **Rechazado** significa protegido.
- **Aceptado** significa no protegido.
- Un resultado **no concluyente** (servidor inalcanzable, error de autenticación) nunca cambia el veredicto almacenado.

Un cambio real de protegido a no protegido dispara una única alerta.

## Ensayos de DR

BombVault ofrece dos niveles de prueba de que tus copias son realmente restaurables, no solo de que están presentes.

- **Ensayos de verificación de restauración (local).** BombVault ejecuta periódicamente `restic check --read-data-subset` (acotado, nunca una restauración completa que llene el disco) y muestra una insignia de *Restaurabilidad verificada* por dominio. La cadencia vive en Ajustes, Programaciones; la insignia en Ajustes, Integridad.
- **Ensayos de DR (externo).** BombVault restaura un objetivo real desde el repo externo en un entorno de pruebas desechable, lo verifica archivo por archivo y byte por byte, y luego limpia. Esto demuestra que puedes recuperarte desde el externo, no solo que el repo responde.

El **cuadro de mando de protección contra ransomware** en el Panel lo resume en una postura verde / ámbar / rojo por dominio, con una lista de comprobación con marca de antigüedad (externo configurado, append-only verificado, replicación al día, ensayo de restauración superado, cifrado activado, estrategia de poda definida). Cada fila roja enlaza directamente con la solución, y la tarjeta solo se pone verde con hechos verificados.

## Emparejamiento de instancias {#pairing}

Los receptores, las fuentes de recogida, la página de Instancias y Mesh externo hablan todos con otro BombVault. Lo hacen como miembros de un grupo de emparejamiento, y una instancia se une al grupo con doce palabras.

En la primera instancia, abre **Ajustes → Emparejamiento** y pulsa **Generar frase** en las tarjetas de emparejamiento. Aparecen doce palabras en una ventana con un botón **Copiar**. En cada una de las demás instancias, abre el mismo lugar, pulsa **Introducir frase** y pégalas o escríbelas, o pulsa **Pegar** en esa ventana. Una palabra que no está en la lista se indica con su posición mientras escribes, y la última palabra lleva una suma de comprobación, así que una palabra mal escrita o intercambiada se detecta antes de que se complete el emparejamiento. Genera la frase en una sola instancia: dos instancias que crean cada una una frase forman dos grupos separados. Si nadie aparece en un minuto, la pestaña ofrece dos salidas: volver a mostrar las palabras para introducirlas allí, o introducir las palabras de la otra instancia y unirte a su grupo en un solo paso. Emparejar funciona sin contraseña de acceso, pero configura una: sin ella, cualquiera que pueda abrir esta interfaz web puede leer las palabras y conseguir, a través del grupo, la contraseña de restic de cada instancia que hay en él. La tarjeta de emparejamiento lo indica hasta que se configura una contraseña. Con una contraseña, volver a mostrar la frase la pide. **Salir del grupo** saca de nuevo a una instancia.

Cualquiera que conozca las palabras puede unirse al grupo, así que trátalas como una contraseña.

**Cómo se encuentran los miembros entre sí.** Cada instancia aprende su propia dirección en la red desde tu navegador en el momento en que inicias sesión, mostrada en la tarjeta del relay como **Esta instancia en tu red**; corrígela ahí si un proxy inverso o un puerto inusual se interpone. En la misma red los miembros anuncian esa dirección por multicast y hablan directamente, y donde el multicast no puede cruzar una red de contenedores, como la red bridge por defecto de Docker, una instancia busca en su propia subred a las demás con una llamada firmada que solo un miembro del grupo puede responder, así que el emparejamiento sigue completándose en segundos sin relay. Si no aparece nada, **¿No la encuentras?**, debajo de la tarjeta de emparejamiento, acepta una dirección a mano, para otra subred o un puerto no estándar. Las instancias en redes distintas pasan por un relay, elegido en la misma pestaña:

- **Relay del proyecto** (el predeterminado): `relay.halleluja.design`, el mismo relay que usa también KnightLoader. Nada que configurar.
- **Relay propio**: el contenedor **BombVault Relay** de las Unraid Community Apps, o una de tus instancias que ya sea accesible desde fuera con **Actuar como relay** activado. Esa instancia responde entonces en `/relay/connect` en su propia dirección, detrás del proxy inverso y el certificado que ya tiene, y deja entrar solo a tu grupo. Introduce la dirección del relay en cada instancia que deba usarlo.
- **Sin relay**: los miembros se encuentran automáticamente solo en la misma red, y en ningún otro sitio.

**Lo que ve el relay.** Cada llamada entre miembros va sellada con AES-256-GCM bajo una clave derivada de las doce palabras, y esa clave nunca sale de tus instancias. El relay conoce un hash que agrupa las conexiones, para qué instancia es un mensaje, qué tamaño tiene y cuándo pasa. Una llamada directa en la red local va sellada del mismo modo y además firmada, así que nada depende del certificado autofirmado que ofrece una instancia.

**Lo que viaja por el grupo.** Los cuadros de mando de la página de Instancias, una petición de comprobar un dominio ahora, las ofertas Mesh externo, y lo que necesita un receptor o una fuente de recogida: las ubicaciones del repositorio de la otra instancia y su contraseña restic. Los datos de copia nunca lo hacen; siguen yendo directos a los backends de restic. Tampoco la APP_KEY: la contraseña restic abre los repositorios de esa instancia y nada más, ni sus secretos guardados, ni sesiones, ni códigos de recuperación.

**Entradas de antes del emparejamiento.** Las instancias añadidas con un token de fleet, y los receptores y fuentes de recogida configurados con la APP_KEY de la otra instancia, se mantienen tras la actualización y quedan marcados como **Emparejar de nuevo**. Los receptores y fuentes de recogida siguen funcionando: en su primer arranque, BombVault sustituye cada APP_KEY guardada por la contraseña restic derivada de ella. Empareja ambas instancias, luego edita la entrada y elige su instancia. Una instancia así retoma su tarjeta anterior en cuanto aparece en el grupo una instancia con el mismo nombre.

El único lugar que todavía acepta una APP_KEY a mano es [Restaurar desde otro repo de BombVault](#restore-from-another-bombvault-repo), para el caso en que la otra instancia haya desaparecido y no pueda responder en un grupo.

## Panel receptor (el lado receptor)

![El lado receptor, vigilado en solo lectura, con una comprobación de integridad hecha en esta máquina.](assets/screenshots/receiver.png)

*El lado receptor, vigilado en solo lectura, con una comprobación de integridad hecha en esta máquina.*

Todo lo anterior es el lado *emisor*. En la máquina que **recibe** copias externas inmutables de otro BombVault, el panel receptor te ofrece monitorización independiente y de solo lectura de esos repositorios en el hardware receptor, de modo que un fallo silencioso en el otro extremo no pase desapercibido.

Activa el conmutador **Receptor** en Ajustes para revelar una pestaña **Receptor**. Está desactivado por defecto; actívalo solo en una máquina que realmente reciba copias externas inmutables. Después registra un repositorio recibido (de solo lectura, abierto con la contraseña restic de la instancia emisora, que llega a través del [grupo de emparejamiento](#pairing)) para obtener:

- **Un inventario de instantáneas agrupado por fuente**, para que puedas ver exactamente qué contenedores, VMs y conjuntos de archivos han llegado.
- **Última recepción** por fuente, para que sepas cómo de fresca es cada una.
- **Un `restic check` independiente** ejecutado en el hardware receptor, de modo que la integridad se verifica donde los datos realmente residen, no solo en el emisor.
- **Un interruptor de hombre muerto:** una alerta cuando una fuente deja de enviar dentro de una ventana que tú defines.
- **Alertas de integridad:** una alerta cuando una comprobación del lado receptor falla.

El Receptor es estrictamente de solo lectura. Nunca escribe en el repositorio recibido, de modo que nunca puede romper la garantía append-only en la que confía el emisor.

## Ejemplo completo: dos equipos Unraid, de principio a fin

Lo anterior describe las piezas. Esto es una instalación completa con valores reales, porque las piezas se montan mejor cuando uno las ha visto montadas una vez.

Dos equipos: **TOWER** ejecuta los contenedores y envía las copias, **VAULT** las recibe e impone la inmutabilidad. Sustituye por tus propios nombres, direcciones y rutas de recurso compartido.

**1. En VAULT, levanta el servidor append-only.** En BombVault en TOWER ve a *Ajustes → Externo → Configurar*, elige **rest-server** y genera la receta. Copia la pestaña **Plantilla de Unraid (XML)**, guárdala en VAULT como `/boot/config/plugins/dockerMan/templates-user/my-rest-server.xml`, luego *Docker → Add Container* y elige **rest-server** en la lista de plantillas. Antes de arrancarlo, escribe la línea `htpasswd` mostrada en `/mnt/user/appdata/rest-server/.htpasswd` en VAULT. La contraseña de un solo uso se muestra una vez y nunca se guarda: cópiala ahora. Esa línea lleva la misma contraseña, ya cifrada con bcrypt para ti: el texto en claro va en las credenciales REST de TOWER, la línea cifrada en el `.htpasswd` de VAULT. No tienes que cifrar nada tú.

    Deja `--append-only` en el campo OPTIONS. Es el sentido de todo esto: sin él, VAULT vuelve a ser un recurso compartido normal.

**2. En TOWER, apunta el repositorio externo hacia él.** La URL del repositorio sigue el patrón que imprime la receta:

    rest:http://VAULT:8000/bombvault-containers/containers

El primer segmento de la ruta es el usuario htpasswd, el segundo el repositorio. Introduce el usuario y la contraseña generados como credenciales REST del destino y ejecuta la **prueba de conexión**.

**3. En TOWER, activa «Inmutable».** La prueba de manipulación se ejecuta de inmediato y debe decir *protegido*. Qué significan las respuestas:

| Resultado | Qué ocurrió |
| --- | --- |
| **protegido** | VAULT rechazó el borrado. Es el único estado que aprueba. |
| **NO protegido** | VAULT aceptó un borrado. Falta `--append-only` o se ha quitado. |
| **no concluyente** | Ninguna de las dos. Normalmente la URL no es la que usa restic, o las credenciales han cambiado. No se registra nada ni se dispara ninguna alerta. |

**4. En VAULT, observa lo que llega.** Empareja los dos equipos ([Emparejamiento de instancias](#pairing)), activa *Ajustes → General → Receptor*, abre la pestaña **Receptor** y registra el repositorio en solo lectura con TOWER como instancia emisora.

!!! warning "La ubicación es una ruta **dentro** del contenedor, escrita relativa al montaje del host"
    Introduce `user/appdata/rest-server/bombvault-containers/containers`, **no** `/mnt/user/appdata/…`. BombVault se ejecuta en un contenedor donde el `/mnt` del host está montado en otro sitio; una ruta absoluta del host no existe ahí. Si pegas una, BombVault ahora te indica la ruta relativa que debes usar.

    VAULT recibe la contraseña restic de TOWER a través del grupo al guardar; nadie escribe una clave.

**5. Hazlo mutuo, si quieres.** Repite los mismos cinco pasos en sentido contrario: un rest-server en TOWER que reciba la copia de VAULT. Entonces cada equipo impone la inmutabilidad al otro, y ninguno puede borrar las copias del otro.

## Recuperación guiada

Una pestaña **Recuperación** dedicada guía una instalación nueva o reconstruida a través del caso de desastre, en un solo lugar:

1. **Restaura primero los propios ajustes de BombVault**, de modo que las rutas de copia, los destinos externos y las credenciales que necesita el resto del flujo vengan rellenados de antemano (aplicado mediante un autoreinicio a través del socket de Docker, de modo que la base de datos de ajustes en ejecución nunca se sobrescribe bajo un descriptor abierto).
2. **Comprueba que BombVault puede leer tus copias** (la trampa de la clave de cifrado por adelantado).
3. Te permite **apuntar a tu repo existente** (local o externo).
4. **Descubre** los contenedores, VMs, conjuntos de archivos y conjuntos de datos ZFS almacenados en él.
5. **Restaura los contenedores y las VMs de una vez** (dejados detenidos, para que los inicies deliberadamente) y lista los conjuntos de archivos y los elementos ZFS para restaurarlos uno a uno; los elementos ZFS vuelven desactivados. Tu kit de recuperación está a un clic de distancia.

!!! tip "Migración planificada frente a desastre"
    La recuperación guiada restaura los propios ajustes de BombVault desde una copia. Para un traslado *planificado* a una máquina nueva, puedes en su lugar llevar tu configuración directamente con la tarjeta **Exportar / importar ajustes** (un archivo JSON portátil). Consulta [Configuración](configuration.md#portable-settings-export-and-import).

### Restaurar desde otro repo de BombVault {#restore-from-another-bombvault-repo}

Una tarjeta aparte en la pestaña **Recuperación** abre el repo de una instancia *distinta* de BombVault (un recurso compartido montado bajo `/mnt`, o una URL remota) con **la `APP_KEY` de esa instancia**, en una sesión única y de solo lectura. Explora los contenedores, VMs y conjuntos de archivos almacenados ahí, elige una instantánea y restáurala, y el objeto restaurado se convierte en un contenedor, VM o conjunto de archivos local normal. Nunca se escribe nada en el otro repo, y tus propios ajustes de copia quedan intactos (la sesión vive en memoria y expira por sí sola). Mover un contenedor del servidor A al servidor B ya no significa reapuntar los ajustes de tu repo y revertirlos después. Esta tarjeta es de un solo uso: abre una sesión, restaura lo que eliges y se olvida de la otra instancia. Si en cambio quieres un arreglo permanente, en el que esta máquina trae según una programación las instantáneas de otra instancia a su propio repositorio, eso es la pestaña **Recogida** de la página **Instancias**.

## Kit de recuperación de la clave de cifrado

Esta es la pieza que hace posible la recuperación ante desastres incluso cuando no hay ningún BombVault en ejecución.

Un clic descarga la **clave maestra**, la **contraseña restic derivada** y las **ubicaciones y comandos exactos del repo**, para que puedas restaurar directamente con la CLI de restic en cualquier máquina. Un recordatorio del Panel insiste hasta que lo hayas guardado.

!!! danger "Guarda el kit de recuperación fuera del servidor"
    El kit contiene el secreto que descifra tus copias. Guárdalo en un lugar seguro y separado del servidor (un gestor de contraseñas, una copia impresa en una caja fuerte). Si pierdes tanto BombVault como `APP_KEY` sin kit de recuperación, tus copias cifradas no se pueden recuperar.

!!! warning "La instantánea más reciente no siempre es la que hay que restaurar"
    Desde restic 0.17, `restic snapshots` muestra el tamaño de cada instantánea. Tras una pérdida de datos, la instantánea más reciente puede ser la vaciada, así que no restaures una instantánea mucho más pequeña que las anteriores. Tras un ransomware puede ser la cifrada, con el tamaño habitual. Si BombVault sigue funcionando, mira antes su página **Anomalías**: indica la última copia buena. Una restauración no necesita ningún dato de anomalías de BombVault, y la pausa de retención solo conserva más instantáneas, nunca menos.

### Sellar el kit

Si has activado el cifrado age para las exportaciones sencillas (Ajustes), el kit también se sella con él y se descarga como `bombvault-recovery-kit.md.age`. Está en ASCII armor y no en binario, así que sigue siendo texto: pegarlo en un gestor de contraseñas o imprimirlo funciona exactamente igual que antes, solo que el contenido es ilegible sin tu clave.

!!! warning "No guardes la clave age dentro del kit"
    Necesitas tu clave age **privada** para abrir un kit sellado. Guárdala en un sitio que no dependa del propio kit, o tendrás dos cosas que recuperar en lugar de una. Sellar compensa cuando el kit se guarda en un sitio que no controlas del todo (un gestor de contraseñas compartido, notas en la nube, una copia impresa en una oficina); un kit en tu propia caja fuerte ya está protegido por la caja fuerte.

    Con el cifrado activado y sin ningún destinatario utilizable configurado, la descarga se rechaza sin más. BombVault nunca recurre a entregar la clave maestra en claro.

### Si no tienes el kit a mano

La contraseña no se guarda en ningún sitio, se **calcula** a partir de la `APP_KEY`. Con la clave y una shell puedes reproducirla tú mismo:

```sh
printf 'bombvault:restic-repo' \
  | openssl dgst -sha256 -mac HMAC -macopt hexkey:$APP_KEY -r \
  | cut -d' ' -f1
```

Es un HMAC-SHA256 sobre la cadena fija `bombvault:restic-repo`, con los bytes crudos de la `APP_KEY` hexadecimal como clave, impreso como 64 caracteres hexadecimales en minúscula. El mismo valor está en el kit, como contraseña restic derivada; esto es para el día en que el kit esté en otro sitio que tú.

!!! warning "Para un repositorio recibido, usa la clave de la instancia EMISORA"
    Un repositorio que llegó aquí por replicación fuera de sede lo creó la máquina que lo envió, con **su** `APP_KEY`. Derivar desde la clave de la máquina receptora produce una contraseña que restic rechaza, lo que se lee exactamente como un repositorio corrupto sin serlo. Esa es la razón habitual de que `restic check` sobre un repositorio recibido pida la contraseña una y otra vez.

Como las definiciones de recuperación viven **dentro** de cada repo (`<repo>/def`, `<repo>/vm-def`), una carpeta de repo copiada es totalmente autocontenida, de modo que el kit más el repo es todo lo que una restauración desde cero necesita.

## Recuperar un volcado de base de datos {#database-dumps}

Un volcado de base de datos es un punto de restauración propio en el repositorio de contenedores, con la etiqueta `dbdump:<container>` y un único archivo, `/dbdump/<container>.sql`. BombVault los lista, los descarga y los importa en **Copias**; abajo están los mismos pasos solo con restic, para el día en que BombVault no esté.

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
