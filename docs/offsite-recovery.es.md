# Copia externa y recuperación

Las copias locales te protegen de un contenedor perdido o de una mala actualización. La replicación externa y un kit de recuperación probado te protegen de la pérdida de toda la máquina, del ransomware o de un incendio. Esta página cubre la replicación externa, cómo hacer esa copia a prueba de manipulaciones, cómo demostrar que puedes restaurar y cómo recuperarte cuando el propio BombVault ha desaparecido.

## Replicación externa

Conserva la copia local rápida y añade una o varias réplicas externas. Define un repo por dominio en la pestaña **Ajustes, Externo**. BombVault replica ahí las nuevas instantáneas con `restic copy` en modo de mejor esfuerzo, de modo que un contratiempo externo nunca hace fallar la copia local. El repo local sigue siendo el principal.

- **Varios destinos externos por dominio.** Cada dominio (contenedores, VMs, flash, config y conjuntos de archivos) puede replicarse a varios destinos externos a la vez, no solo a uno, de modo que puedes mantener, por ejemplo, un rest-server en la máquina de un amigo y un bucket S3 en paralelo. Añade destinos adicionales en Ajustes, Externo, cada uno con su propio repositorio, clase de almacenamiento S3, marca append-only, retención y presupuesto de crecimiento. Una configuración externa única existente se traslada intacta como el primer destino, y cada destino de un dominio se replica según el calendario externo de ese dominio.
- **Calendario externo por dominio** (editado junto a todos los demás calendarios en Ajustes, Calendarios): déjalo en blanco para replicar tras cada copia local, o establece una cadencia (por ejemplo `weekly Sun 03:00`) para enviar fuera del sitio con menos frecuencia de la que copias localmente. Un botón **Replicar ahora** cubre las ejecuciones bajo demanda.
- La **retención externa** vive en Ajustes, Externo para que puedas conservar las copias externas más tiempo como archivo. Deja la política toda a cero para no recortar nunca automáticamente las instantáneas externas.
- Los **límites de ancho de banda** (Ajustes, Externo) limitan la velocidad de subida/bajada de restic para que la replicación no sature tu WAN.
- Un **indicador de replicación** muestra qué dominio se está replicando mientras se ejecuta (en su página y en el Panel). Es un indicador activo, no una barra de porcentaje, porque `restic copy` no expone ningún progreso legible por máquina.

!!! note "Restaurar directamente desde externo"
    Cada navegador de copias tiene un conmutador **Local / Externo**, de modo que si un repo local se pierde o se corrompe puedes listar y restaurar directamente desde la réplica externa. Eliminar es por fuente: quitar una copia solo afecta a la copia que estás viendo.

## Repositorios primarios remotos {#remote-primary-repositories}

La ruta de copia de un dominio (Ajustes, Rutas y almacenamiento) no se limita a una carpeta local: apúntala directamente a un remoto de restic (`s3:...`, `rest:http://host:8000/repo`, `b2:...`, `sftp:usuario@host:/repo`, `rclone:remoto:bucket/ruta`) y BombVault copia allí directamente, sin copia local aparte y sin paso de replicación. Es una forma realmente distinta de la replicación fuera de sede de más arriba: allí el repositorio local es el primario y el de fuera de sede es un archivo suyo en la medida de lo posible; aquí el repositorio remoto **es** el primario, y es la única copia mientras no configures además una replicación fuera de sede (o un segundo remoto) para ese dominio.

Cada uno de los cinco campos de ruta (Contenedores, Máquinas virtuales, Flash, Configuración, Ficheros) lleva justo al lado un conmutador **Local / Remoto**:

- **Local** muestra el explorador de carpetas de siempre.
- **Remoto** lo cambia por un campo de URL sencillo, más un botón que abre el mismo diálogo de prueba de conexión y credenciales que usan los destinos fuera de sede, configurado para este primario. Desde ahí obtienes:
    - **Una prueba de conexión** contra la ruta real, antes de confiar en ella.
    - **Límites de ancho de banda** (subida y bajada) para que una copia programada hacia un primario remoto no sature tu enlace WAN: los mismos parámetros de restic `--limit-upload` y `--limit-download` que usa la replicación fuera de sede, aplicados a la propia copia.
    - **Protección append-only (inmutabilidad)**, verificada con la misma prueba activa de manipulación (una sonda DELETE real contra el otro extremo) que reciben los destinos fuera de sede. Con ella activada, BombVault se niega a podar el repositorio: como detrás no hay copia local aparte, las credenciales de esta máquina no deben poder borrar la única copia de la copia de seguridad.
    - **Una alarma de presupuesto de crecimiento**, tomada de la misma tendencia de tamaño del repositorio que la tarjeta Almacenamiento ya sigue.

Nada de esto es obligatorio: una ruta remota escrita a mano y sin ajustes de seguridad guardados copia exactamente como siempre (ancho de banda ilimitado, podable, sin alarma de presupuesto). El diálogo de seguridad está ahí para cuando quieras las mismas protecciones que recibe una copia fuera de sede, sin tener que crear un destino fuera de sede solo para eso.

!!! note "Las credenciales de nube y REST se comparten"
    Un primario remoto se autentica con las mismas credenciales S3/REST configuradas en Ajustes, Fuera de sede, Credenciales de nube. No hay un almacén de credenciales aparte para los repositorios primarios.

## Externo inmutable (append-only)

Marca un repo externo como append-only para que el ransomware, o un host comprometido, no puedan eliminar ni reescribir tus copias. El otro extremo (un `restic/rest-server` ejecutándose en modo `--append-only`) lo **impone**. BombVault solo lo **verifica** y nunca muestra verde basándose únicamente en una afirmación de configuración.

El asistente de **configuración externa guiada** te lleva desde la elección del backend (rest-server / rclone / S3), pasando por un fragmento de despliegue de rest-server listo para pegar, una prueba de conexión, el conmutador inmutable (que ejecuta la prueba de manipulación de inmediato) y una estrategia de retención, de modo que el externo append-only es alcanzable sin editar configuraciones a mano.

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

- **Ensayos de verificación de restauración (local).** BombVault ejecuta periódicamente `restic check --read-data-subset` (acotado, nunca una restauración completa que llene el disco) y muestra una insignia de *restaurable verificado por última vez* por dominio. La cadencia vive en Ajustes, Calendarios; la insignia en Ajustes, Integridad.
- **Ensayos de DR (externo).** BombVault restaura un objetivo real desde el repo externo en un entorno de pruebas desechable, lo verifica archivo por archivo y byte por byte, y luego limpia. Esto demuestra que puedes recuperarte desde el externo, no solo que el repo responde.

El **cuadro de mando de protección contra ransomware** en el Panel lo resume en una postura verde / ámbar / rojo por dominio, con una lista de comprobación con marca de antigüedad (externo configurado, append-only verificado, replicación al día, ensayo de restauración superado, cifrado activado, estrategia de poda definida). Cada fila roja enlaza directamente con la solución, y la tarjeta solo se pone verde con hechos verificados.

## Panel receptor (el lado receptor)

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

**1. En VAULT, levanta el servidor append-only.** En BombVault en TOWER ve a *Ajustes → Externo → configuración guiada*, elige **rest-server** y genera la receta. Copia la pestaña **Plantilla de Unraid (XML)**, guárdala en VAULT como `/boot/config/plugins/dockerMan/templates-user/my-rest-server.xml`, luego *Docker → Add Container* y elige **rest-server** en la lista de plantillas. Antes de arrancarlo, escribe la línea `htpasswd` mostrada en `/mnt/user/appdata/rest-server/.htpasswd` en VAULT. La contraseña de un solo uso se muestra una vez y nunca se guarda: cópiala ahora.

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

**4. En VAULT, observa lo que llega.** Activa *Ajustes → Receptor*, abre la pestaña **Receptor** y registra el repositorio en solo lectura.

!!! warning "La ubicación es una ruta **dentro** del contenedor, escrita relativa al montaje del host"
    Introduce `user/appdata/rest-server/bombvault-containers/containers`, **no** `/mnt/user/appdata/…`. BombVault se ejecuta en un contenedor donde el `/mnt` del host está montado en otro sitio; una ruta absoluta del host no existe ahí. Si pegas una, BombVault ahora te indica la ruta relativa que debes usar.

    La **APP_KEY emisora** es la clave de TOWER, no la de VAULT. La encuentras en TOWER en *Ajustes → Sistema*.

**5. Hazlo mutuo, si quieres.** Repite los mismos cinco pasos en sentido contrario: un rest-server en TOWER que reciba la copia de VAULT. Entonces cada equipo impone la inmutabilidad al otro, y ninguno puede borrar las copias del otro.

## Recuperación guiada

Una pestaña **Recuperación** dedicada guía una instalación nueva o reconstruida a través del caso de desastre, en un solo lugar:

1. **Restaura primero los propios ajustes de BombVault**, de modo que las rutas de copia, los destinos externos y las credenciales que necesita el resto del flujo vengan rellenados de antemano (aplicado mediante un autoreinicio a través del socket de Docker, de modo que la base de datos de ajustes en ejecución nunca se sobrescribe bajo un descriptor abierto).
2. **Comprueba que BombVault puede leer tus copias** (la trampa de la clave de cifrado por adelantado).
3. Te permite **apuntar a tu repo existente** (local o externo).
4. **Descubre** los contenedores, VMs y conjuntos de archivos almacenados en él.
5. **Los restaura todos** (dejados detenidos, para que los inicies deliberadamente), con tu kit de recuperación a un clic de distancia.

!!! tip "Migración planificada frente a desastre"
    La recuperación guiada restaura los propios ajustes de BombVault desde una copia. Para un traslado *planificado* a una máquina nueva, puedes en su lugar llevar tu configuración directamente con la tarjeta **Exportar e importar ajustes** (un archivo JSON portátil). Consulta [Configuración](configuration.md#portable-settings-export-and-import).

### Restaurar desde otro repo de BombVault

Una tarjeta aparte en la pestaña **Recuperación** abre el repo de una instancia *distinta* de BombVault (un recurso compartido montado bajo `/mnt`, o una URL remota) con **la `APP_KEY` de esa instancia**, en una sesión única y de solo lectura. Explora los contenedores, VMs y conjuntos de archivos almacenados ahí, elige una instantánea y restáurala, y el objeto restaurado se convierte en un contenedor, VM o conjunto de archivos local normal. Nunca se escribe nada en el otro repo, y tus propios ajustes de copia quedan intactos (la sesión vive en memoria y expira por sí sola). Mover un contenedor del servidor A al servidor B ya no significa reapuntar los ajustes de tu repo y revertirlos después. La federación en vivo servidor a servidor queda explícitamente fuera de alcance; esto es una extracción única y deliberada.

## Kit de recuperación de la clave de cifrado

Esta es la pieza que hace posible la recuperación ante desastres incluso cuando no hay ningún BombVault en ejecución.

Un clic descarga la **clave maestra**, la **contraseña restic derivada** y las **ubicaciones y comandos exactos del repo**, para que puedas restaurar directamente con la CLI de restic en cualquier máquina. Un recordatorio del Panel insiste hasta que lo hayas guardado.

!!! danger "Guarda el kit de recuperación fuera del servidor"
    El kit contiene el secreto que descifra tus copias. Guárdalo en un lugar seguro y separado del servidor (un gestor de contraseñas, una copia impresa en una caja fuerte). Si pierdes tanto BombVault como `APP_KEY` sin kit de recuperación, tus copias cifradas no se pueden recuperar.

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
