# Copia externa y recuperación

Las copias locales te protegen de un contenedor perdido o de una mala actualización. La replicación externa y un kit de recuperación probado te protegen de la pérdida de toda la máquina, del ransomware o de un incendio. Esta página cubre la replicación externa, cómo hacer esa copia a prueba de manipulaciones, cómo demostrar que puedes restaurar y cómo recuperarte cuando el propio BombVault ha desaparecido.

## Replicación externa

Conserva la copia local rápida y cópiala a uno o varios lugares más. Los lugares a los que se copia un dominio se eligen en la tarjeta **Dominios** de **Ajustes, Almacenamiento**, un chip por lugar (consulta [Lugares de almacenamiento](storage-places.md#domains)). BombVault copia ahí las nuevas instantáneas con `restic copy` en modo de mejor esfuerzo, de modo que una copia fallida nunca hace fallar la copia local. El lugar donde se almacena un dominio no tiene por qué ser local; consulta [Un dominio almacenado en un lugar remoto](#remote-primary-repositories).

- **Varios lugares de copia por dominio.** Un dominio puede copiarse a varios lugares a la vez, por ejemplo a un rest-server en casa de un amigo y a un bucket de B2. La retención, la clase de almacenamiento, el append-only, los límites y el presupuesto de crecimiento pertenecen al lugar, así que cada copia sigue las reglas del lugar al que llega.
- **Calendario de copia por dominio** (editado junto a todos los demás calendarios en Ajustes, Calendarios): déjalo en blanco para copiar tras cada copia local, o establece una cadencia (por ejemplo `weekly Sun 03:00`) para copiar con menos frecuencia de la que haces copias de seguridad. **Copiar ahora** en la fila del dominio lo ejecuta bajo demanda.
- **Retención por lugar.** Cada lugar tiene sus propias reglas, así que un lugar externo puede conservar las copias más tiempo como archivo. Un lugar con todas las reglas a cero nunca recorta.
- Los **límites de ancho de banda** por lugar limitan la velocidad de subida y de bajada de restic para que la copia no sature tu WAN.
- Un **indicador de replicación** muestra qué dominio se está copiando mientras se ejecuta (en su página y en el Panel). Es un indicador activo, no una barra de porcentaje, porque `restic copy` no expone ningún progreso legible por máquina.

!!! note "Restaurar desde cualquier lugar"
    Cada contenedor, VM, conjunto de archivos, el flash y la configuración de la app listan sus copias de seguridad como una única línea de tiempo a través de todos los lugares donde reside una copia. Una copia enviada a B2 aparece una sola vez, marcada con cada lugar que la tiene. Una restauración toma el primer lugar al que puede llegar, empezando por el repositorio en el que se escribe el elemento, y puedes elegir otro lugar por fila. Los lugares externos solo se leen cuando los abres. Borrar en un lugar comprueba primero los demás y dice si era la última copia.

## Ubicación por elemento {#placement}

Cada tarjeta de contenedor, VM y conjunto de archivos tiene una fila de **Ubicación** con tres segmentos:

- **Local** escribe el elemento en el repositorio que se muestra bajo **Almacenado en** y no lo copia a ningún sitio. Úsalo para datos que ya tienen una segunda copia, por ejemplo un recurso compartido que vive en un NAS.
- **Local + externo** lo escribe también ahí y lo copia a los destinos marcados bajo **Copiar a**, un chip por cada destino externo del dominio. Desmarca un chip y ese destino no recibe nada nuevo de este elemento.
- **Solo externo** escribe el elemento directamente en el lugar bajo **Enviar a**, cualquier lugar distinto de aquel donde se almacena el dominio. Si el dominio ya se copia a ese lugar, el elemento recibe un repositorio directo junto a las copias; si no, BombVault crea allí un repositorio para el dominio.

La ubicación queda fija desde la primera copia de seguridad del elemento, porque BombVault nunca mueve copias entre repositorios. Las copias pueden cambiar en cualquier momento. Un destino que deja de recibir un elemento conserva las copias que tiene y las recorta a su propia retención en la siguiente ejecución externa del dominio; **Borrar en B2** en la tarjeta las elimina de inmediato. Cuando algunas de esas copias no existen en ningún otro lugar, la confirmación las enumera por fecha y pide el nombre del elemento. De los destinos de solo añadir no se puede borrar.

Bajo la fila, la tarjeta dice adónde va el elemento y qué hay realmente ahí: cuántas sedes lo tienen, cuándo se vio cada destino por última vez, y si se cumple 3-2-1. Una sede es el servidor con los datos originales y cada lugar que está en otro sitio (consulta [Fuera del local](#off-the-premises-mark)). BombVault comprueba copias y sedes; no comprueba la parte de "dos soportes" de 3-2-1.

### Valores predeterminados por dominio

La tarjeta **Dominios** de Ajustes, Almacenamiento tiene una fila por dominio. **Copiado a** se aplica de inmediato a cada elemento sin elección propia, y a las carpetas de proyecto de las pilas de Compose. En cuanto un dominio tiene copias de seguridad, **Almacenado en** se aplica a un elemento nuevo en su primera copia de seguridad, y cambiarlo no mueve ninguna copia de seguridad. Antes de guardar, la fila nombra cada lugar que gana o pierde elementos y cuántas instantáneas supone eso, y la pregunta incluye el interruptor **Aplicar a elementos sin copias de seguridad**, que además pone en el nuevo valor predeterminado a todo elemento que aún no tiene copia de seguridad. **Excepciones** enumera los elementos con elección propia.

Marcar un lugar nuevo en **Copiado a** hace que reciba todo elemento que no esté en Local. La confirmación dice cuántos elementos son y, cuando se sabe, cuánto historial supone eso.

### Repositorios directos

Elegir en Solo externo un lugar al que el dominio ya se copia pide confirmación una vez y luego crea un repositorio directo junto a las copias, por ejemplo `s3:https://s3.eu-central-003.backblazeb2.com/bucket/container-direct`, y apunta el elemento a él. Para un destino de copia sin lugar, la elección abre un diálogo con una dirección sugerida y una prueba de conexión que no crea nada, y **Crear y usar** crea el repositorio. Un repositorio directo toma la clave, la clase de almacenamiento, los límites, la configuración append-only y la retención del lugar, y cambia con ellos. Cuando una clave nueva del lugar no puede abrirlo, el repositorio directo conserva la clave que tiene y el guardado lo dice. Sus instantáneas llevan la etiqueta `bv:direct`, y las demás pasadas de retención las conservan, así que un repositorio directo que perdió su enlace con su lugar nunca envejece por las reglas locales. Una clave de B2 limitada a una carpeta tiene que cubrir la dirección del lugar, no solo la carpeta del dominio, o la carpeta de al lado queda fuera de su alcance.

### Fuera del local {#off-the-premises-mark}

Una copia cuenta como una sede propia solo cuando su lugar está en otro sitio. Un lugar en la nube siempre cuenta y una carpeta en este Unraid nunca; para un NAS, un rest-server o un servidor SFTP, responde a **¿Dónde está el dispositivo?** en los detalles del lugar con **Aquí, en casa** o **En otro sitio**. La respuesta solo cuenta para las sedes y el 3-2-1 en las tarjetas y en el Panel. No cambia ninguna copia.

### Tras una reconstrucción

Las elecciones de copia viven en los propios ajustes de BombVault. Tras una reconstrucción mediante Descubrir sin un `/config` restaurado, desaparecen, y copiar todo enviaría de nuevo a B2 los elementos que habías dejado fuera. Por eso la replicación externa de cada dominio reconstruido se pausa. El Panel lo muestra en ámbar, y la fila del dominio en la tarjeta Dominios ofrece **Confirmar valor predeterminado** con una vista previa de lo que copia la siguiente ejecución y los nombres en las copias de seguridad que no tienen entrada, que puedes dejar fuera ahí. Solo la confirmación termina la pausa; importar un archivo de ajustes trae de vuelta reglas y valores predeterminados pero no la termina.

## Un dominio almacenado en un lugar remoto {#remote-primary-repositories}

Un dominio no tiene por qué almacenarse en local. Mientras su ubicación de copia no contenga copias de seguridad, elige un lugar remoto en **Almacenado en** en la tarjeta Dominios y el dominio hará sus copias de seguridad directamente allí, sin copia local y sin paso de copia. El repositorio remoto es entonces la única copia, salvo que el dominio se copie además a otro lugar. Todo lugar remoto trae las mismas salvaguardas:

- **Una prueba de conexión** antes de que se escriba nada.
- **Límites de ancho de banda** para la propia copia de seguridad, los mismos parámetros `--limit-upload` y `--limit-download` que usa una copia.
- **Protección append-only**, verificada con la misma prueba activa de manipulación. Con ella activada, BombVault nunca poda el repositorio, porque las credenciales de esta máquina no deben poder borrar la única copia de la copia de seguridad.
- **Un presupuesto de crecimiento**, tomado de la misma tendencia de tamaño que sigue la tarjeta Almacenamiento.

Un dominio almacenado en un lugar remoto es el origen de sus copias, igual que uno local; consulta [Copias entre lugares con credenciales distintas](storage-places.md#different-credentials).

!!! note "Las credenciales pertenecen al lugar"
    Un lugar remoto guarda sus propias credenciales. Un lugar configurado con las credenciales de nube compartidas las sigue usando hasta que se cambie su acceso en sus detalles.

### SMB y WebDAV sin montaje en el host {#smb-webdav}

El formulario rclone de la ventana **Añadir lugar** tiene un formulario para un recurso compartido de Windows o Samba y para un servidor WebDAV (Nextcloud, ownCloud, SharePoint o cualquier otro). Rellena un nombre corto, el host y el recurso compartido (SMB) o la URL y el tipo de servidor (WebDAV), el usuario y la contraseña, y BombVault escribe la sección de rclone por ti. rclone ofusca la contraseña por sí mismo antes de guardarla; añadir un destino con un nombre que ya existe sustituye esa sección en lugar de añadir una segunda.

El remoto nuevo aparece luego en la lista de remotos del formulario, donde lo eliges para el lugar. El recurso compartido es el primer segmento de la ruta, no forma parte del nombre.

Es mejor camino que montar el recurso compartido en Unraid: restic desaconseja guardar un repositorio en un recurso CIFS montado, y aquí no se monta nada. NFS no está en el formulario porque ni restic ni rclone tienen un backend NFS; para NFS, monta el export en el host y añádelo como lugar con **Otro recurso compartido**.

## Externo inmutable (append-only)

Marca un repo externo como append-only para que el ransomware, o un host comprometido, no puedan eliminar ni reescribir tus copias. El otro extremo (un `restic/rest-server` ejecutándose en modo `--append-only`) lo **impone**. BombVault solo lo **verifica** y nunca muestra verde basándose únicamente en una afirmación de configuración.

La ventana **Añadir lugar** incluye una receta lista para pegar para un rest-server en modo append-only, con un usuario para este BombVault. En un lugar rest-server con **Append-only** activado, **Probar append-only** en los detalles del lugar ejecuta la prueba de manipulación contra cada ruta de dominio, copia activada y repositorio del lugar y da una sola respuesta para el lugar, de modo que el externo append-only es alcanzable sin editar configuraciones a mano.

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

En un lugar, **Probar append-only** sondea cada ruta de dominio, copia activada y repositorio que hay allí con sus propias credenciales y reúne los veredictos en una sola respuesta: basta un repositorio que acepte un borrado para que todo el lugar quede en *borrado aceptado*.

## Ensayos de DR

BombVault ofrece dos niveles de prueba de que tus copias son realmente restaurables, no solo de que están presentes.

- **Ensayos de verificación de restauración (local).** BombVault ejecuta periódicamente `restic check --read-data-subset` (acotado, nunca una restauración completa que llene el disco) y muestra una insignia de *restaurable verificado por última vez* por dominio. La cadencia vive en Ajustes, Calendarios; la insignia en Ajustes, Integridad.
- **Ensayos de DR (externo).** BombVault restaura un objetivo real desde el repo externo en un entorno de pruebas desechable, lo verifica archivo por archivo y byte por byte, y luego limpia. Esto demuestra que puedes recuperarte desde el externo, no solo que el repo responde. Solo se ensayan los lugares en otro sitio, porque una copia en la misma casa no demuestra nada ante la pérdida de la casa. Un dominio copiado a varios de ellos se ensaya contra uno en cada ejecución programada, por turnos, y el Panel indica el lugar del último ensayo.

El **cuadro de mando de protección contra ransomware** en el Panel lo resume en una postura verde / ámbar / rojo por dominio, con una lista de comprobación con marca de antigüedad (externo configurado, append-only verificado, replicación al día, ensayo de restauración superado, cifrado activado, estrategia de poda definida). Cada fila roja enlaza directamente con la solución, y la tarjeta solo se pone verde con hechos verificados.

## Panel receptor (el lado receptor)

![El lado receptor, vigilado en solo lectura, con una comprobación de integridad hecha en esta máquina.](assets/screenshots/receiver.png)

*El lado receptor, vigilado en solo lectura, con una comprobación de integridad hecha en esta máquina.*

Todo lo anterior es el lado *emisor*. En la máquina que **recibe** copias externas inmutables de otro BombVault, el panel receptor te ofrece monitorización independiente y de solo lectura de esos repositorios en el hardware receptor, de modo que un fallo silencioso en el otro extremo no pase desapercibido.

Activa el conmutador **Receptor** en Ajustes para revelar una pestaña **Receptor**. Está desactivado por defecto; actívalo solo en una máquina que realmente reciba copias externas inmutables. Después registra un repositorio recibido (de solo lectura, abierto con la clave de la instancia emisora) para obtener:

- **Un inventario de instantáneas agrupado por fuente**, para que puedas ver exactamente qué contenedores, VMs y conjuntos de archivos han llegado.
- **Última recepción** por fuente, para que sepas cómo de fresca es cada una.
- **Un `restic check` independiente** ejecutado en el hardware receptor, de modo que la integridad se verifica donde los datos realmente residen, no solo en el emisor.
- **Un interruptor de hombre muerto:** una alerta cuando una fuente deja de enviar dentro de una ventana que tú defines.
- **Alertas de integridad:** una alerta cuando una comprobación del lado receptor falla.

El Receptor es estrictamente de solo lectura. Nunca escribe en el repositorio recibido, de modo que nunca puede romper la garantía append-only en la que confía el emisor.

## Ejemplo completo: dos equipos Unraid, de principio a fin

Lo anterior describe las piezas. Esto es una instalación completa con valores reales, porque las piezas se montan mejor cuando uno las ha visto montadas una vez.

Dos equipos: **TOWER** ejecuta los contenedores y envía las copias, **VAULT** las recibe e impone la inmutabilidad. Sustituye por tus propios nombres, direcciones y rutas de recurso compartido.

**1. En VAULT, levanta el servidor append-only.** En BombVault en TOWER abre *Ajustes → Almacenamiento*, haz clic en **Añadir lugar**, elige **rest-server** y haz clic en **Mostrar receta**. Copia el bloque **Plantilla de Unraid**, guárdalo en VAULT como `/boot/config/plugins/dockerMan/templates-user/my-rest-server.xml`, luego *Docker → Add Container* y elige **rest-server** en la lista de plantillas. Antes de arrancarlo, escribe la línea `htpasswd` mostrada en `/mnt/user/appdata/rest-server/.htpasswd` en VAULT. La contraseña se muestra una vez y nunca se guarda; la receta ya la ha puesto, junto con el usuario, en el formulario de TOWER, así que deja esa ventana abierta. La línea `htpasswd` lleva la misma contraseña, ya cifrada con bcrypt para ti, así que no tienes que cifrar nada tú.

    Deja `--append-only` en el campo OPTIONS. Sin él, VAULT vuelve a ser un recurso compartido corriente.

**2. En TOWER, añade el lugar.** Introduce la dirección de VAULT, `http://VAULT:8000`, junto al usuario y la contraseña que rellenó la receta, y haz clic en **Probar conexión**. BombVault construye la dirección con ellos:

    rest:http://VAULT:8000/tower

El primer segmento de la ruta es el usuario htpasswd, aquí `tower`, y cada dominio recibe su carpeta debajo, por ejemplo `rest:http://VAULT:8000/tower/container`. Responde a **¿Dónde está el dispositivo?** con **En otro sitio**, haz clic en **Añadir** y marca el lugar en **Copiado a** para los dominios que deban ir allí.

**3. En TOWER, activa Append-only** en **Protección**, en los detalles del lugar, y haz clic en **Probar append-only**. La prueba sondea cada ruta de dominio, copia y repositorio del lugar y da una sola respuesta para el lugar, que debe ser *borrado rechazado*. Qué significan las respuestas:

| Resultado | Qué ocurrió |
| --- | --- |
| **borrado rechazado** | VAULT rechazó el borrado. Es el único estado que aprueba. |
| **borrado aceptado** | VAULT aceptó un borrado. Falta `--append-only` o se ha quitado. |
| un mensaje en lugar de un resultado | La prueba no pudo ejecutarse. Normalmente la dirección no es la que usa restic, o las credenciales han cambiado. No se registra nada ni se dispara ninguna alerta. |

**4. En VAULT, observa lo que llega.** Activa *Ajustes → Receptor*, abre la pestaña **Receptor** y registra el repositorio en solo lectura.

!!! warning "La ubicación es una ruta **dentro** del contenedor, escrita relativa al montaje del host"
    Introduce `user/appdata/rest-server/tower/container`, **no** `/mnt/user/appdata/...`. BombVault se ejecuta en un contenedor donde el `/mnt` del host está montado en otro sitio; una ruta absoluta del host no existe ahí. Si pegas una, BombVault te indica la ruta relativa que debes usar.

    La **APP_KEY emisora** es la clave de TOWER, no la de VAULT. La encuentras en TOWER en *Ajustes → Sistema*.

**5. Hazlo mutuo, si quieres.** Repite los mismos cinco pasos en sentido contrario: un rest-server en TOWER que reciba la copia de VAULT. Entonces cada equipo impone la inmutabilidad al otro, y ninguno puede borrar las copias del otro.

## Recuperación guiada

Una pestaña **Recuperación** dedicada guía una instalación nueva o reconstruida a través del caso de desastre, en un solo lugar:

1. **Comprueba que BombVault puede leer tus copias** (la trampa de la clave de cifrado por adelantado).
2. **Restaura los propios ajustes de BombVault**, de modo que las rutas de copia, los destinos externos y las credenciales que necesita el resto del flujo vengan rellenados de antemano. Lee la copia de configuración del lugar que la fila Autocopia indica en **Almacenado en**, o de la copia de la Autocopia en **Copiado a**, y muestra ese lugar con su dirección; para leer de otro lugar, cambia primero la fila Autocopia en el paso 3. La restauración se aplica mediante un autoreinicio a través del socket de Docker, de modo que la base de datos de ajustes en ejecución nunca se sobrescribe bajo un descriptor abierto.
3. **Adjunta tus copias existentes** mediante las filas de la tarjeta Dominios: en la fila de cada dominio, elige en **Almacenado en** el lugar donde están sus copias de seguridad y en **Copiado a** los lugares que guardan sus copias. Un lugar que ninguna fila ofrece todavía, como un recurso compartido, un servidor o un bucket en la nube, se conecta con **Añadir lugar**, la misma ventana que en Ajustes, Almacenamiento. Después, **Conectar y previsualizar** comprueba que las copias se pueden leer.
4. **Descubre** los contenedores, VMs, conjuntos de archivos y conjuntos de datos ZFS almacenados en él.
5. **Restaura los contenedores y las VMs de una vez** (dejados detenidos, para que los inicies deliberadamente) y lista los conjuntos de archivos y los elementos ZFS para restaurarlos uno a uno; los elementos ZFS vuelven desactivados. Tu kit de recuperación está a un clic de distancia.

!!! note "Las copias externas esperan tras una reconstrucción"
    Cuando el paso 4 reconstruye entradas sin los ajustes antiguos, la replicación externa de esos dominios se pausa hasta que se confirme el valor predeterminado de ubicación. Consulta [Ubicación por elemento](#placement).

!!! tip "Migración planificada frente a desastre"
    La recuperación guiada restaura los propios ajustes de BombVault desde una copia. Para un traslado *planificado* a una máquina nueva, puedes en su lugar llevar tu configuración directamente con la tarjeta **Exportar e importar ajustes** (un archivo JSON portátil). Consulta [Configuración](configuration.md#portable-settings-export-and-import).

### Restaurar desde otro repo de BombVault

Una tarjeta aparte en la pestaña **Recuperación** abre el repo de una instancia *distinta* de BombVault (un recurso compartido montado bajo `/mnt`, o una URL remota) con **la `APP_KEY` de esa instancia**, en una sesión única y de solo lectura. Explora los contenedores, VMs y conjuntos de archivos almacenados ahí, elige una instantánea y restáurala, y el objeto restaurado se convierte en un contenedor, VM o conjunto de archivos local normal. Nunca se escribe nada en el otro repo, y tus propios ajustes de copia quedan intactos (la sesión vive en memoria y expira por sí sola). Mover un contenedor del servidor A al servidor B ya no significa reapuntar los ajustes de tu repo y revertirlos después. La federación en vivo servidor a servidor queda explícitamente fuera de alcance; esto es una extracción única y deliberada.

## Kit de recuperación de la clave de cifrado

Esta es la pieza que hace posible la recuperación ante desastres incluso cuando no hay ningún BombVault en ejecución.

Un clic descarga la **clave maestra**, la **contraseña restic derivada** y las **ubicaciones y comandos exactos del repo**, para que puedas restaurar directamente con la CLI de restic en cualquier máquina. Un recordatorio del Panel insiste hasta que lo hayas guardado.

!!! danger "Guarda el kit de recuperación fuera del servidor"
    El kit contiene el secreto que descifra tus copias. Guárdalo en un lugar seguro y separado del servidor (un gestor de contraseñas, una copia impresa en una caja fuerte). Si pierdes tanto BombVault como `APP_KEY` sin kit de recuperación, tus copias cifradas no se pueden recuperar.

!!! warning "La instantánea más reciente no siempre es la que hay que restaurar"
    Desde restic 0.17, `restic snapshots` muestra el tamaño de cada instantánea. Tras una pérdida de datos, la instantánea más reciente puede ser la vaciada, así que no restaures una instantánea mucho más pequeña que las anteriores. Tras un ransomware puede ser la cifrada, con el tamaño habitual. Si BombVault sigue funcionando, mira antes su página **Anomalías**: indica la última copia buena. Una restauración no necesita ningún dato de anomalías de BombVault, y la pausa de retención solo conserva más instantáneas, nunca menos.

### Sellar el kit

Si has activado el cifrado age para las exportaciones sencillas (Ajustes), el kit también se sella con él y se descarga como `bombvault-recovery-kit.md.age`. Está en ASCII armor en lugar de binario, así que sigue siendo texto plano: pegarlo en un gestor de contraseñas o imprimirlo funciona exactamente igual que antes, solo que el contenido es ilegible sin tu clave.

!!! warning "No guardes la clave age dentro del kit"
    Para abrir un kit sellado necesitas tu clave age **privada**. Guárdala en un lugar que no dependa del propio kit, o tendrás dos cosas que recuperar en vez de una. Sellar merece la pena cuando el kit se guarda en un sitio que no controlas del todo (un gestor de contraseñas compartido, notas en la nube, una copia impresa en una oficina); a un kit en tu propia caja fuerte ya lo protege la caja fuerte.

    Con el cifrado activado y sin ningún destinatario válido configurado, la descarga se rechaza directamente. BombVault nunca recurre a entregar la clave maestra en claro.

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
