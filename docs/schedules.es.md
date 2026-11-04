# Calendarios

**Ajustes, Calendario** contiene todos los calendarios: la copia total que lo cubre todo, un calendario propio para una sección, las copias externas, qué elementos participan y en qué orden. Un solo elemento puede tener un calendario propio en su página. Esta página explica cómo encajan y recorre una configuración típica.

## Los calendarios se leen como frases {#plans}

Cada calendario es un botón que se lee como una frase, como *a diario a las 03:00* o *cada 2 días a las 01:30*. Un clic lo abre con cinco modos:

| Modo | Qué fijas |
|---|---|
| **Desactivado** | No se ejecuta nada con este calendario. |
| **Diario** | **Hora** |
| **Semanal** | **Días** y **Hora** |
| **Cada N días** | **Cada** tantos **días**, y la **Hora**. Cuenta desde la última ejecución programada, así que copiar un elemento a mano no atrasa el calendario. |
| **Cron** | Una **Expresión** de cinco campos, como `0 */6 * * *` para cada 6 horas. **Ejemplos** ofrece unos cuantos, y la ventana muestra las próximas ejecuciones. |

Todas las horas están en la zona horaria del servidor. En Unraid es la del propio sistema. En otros sitios, define `TZ`, o todos los calendarios se ejecutan en UTC, consulta [Variables de entorno](configuration.md#environment-variables).

## Semana {#week}

La tarjeta **Semana** dibuja cada ejecución prevista de lunes a domingo, una línea por día: copias, copias externas y comprobaciones. Apunta a un trazo para leer qué empieza ahí. Donde tres calendarios empiezan en un margen de dos horas, el tramo se marca, para que puedas repartirlos.

![Ajustes, Calendario: la banda Semana arriba, luego la copia total y los calendarios por sección.](assets/screenshots/schedule/week-dark.png#only-dark)
![Ajustes, Calendario: la banda Semana arriba, luego la copia total y los calendarios por sección.](assets/screenshots/schedule/week-light.png#only-light)

## Copia total {#full-backup}

La **Copia total** es el calendario predeterminado para todo. Lo fija **Cuándo se ejecutan las copias**: una pasada que copia cada sección por turno, contenedores, VMs, flash, carpetas, conjuntos de datos ZFS y la autocopia al final. La línea de debajo dice qué secciones incluye. **Desactivado** la apaga, y entonces solo se ejecuta lo que tiene un calendario propio.

- **Recuperar copias perdidas tras el arranque**: cuando el servidor estaba apagado a la hora prevista, BombVault ejecuta esa copia unos dos minutos después de arrancar.
- **Comandos antes y después**: **Comando antes de la copia total** se ejecuta en el servidor con `sh -c` antes de que empiece la pasada, por ejemplo un script que detiene un servicio. Si falla, la copia total se ejecuta igualmente. **Comando después de la copia total** se ejecuta una vez tras intentar todas las secciones, aunque una de ellas haya fallado, lo que sirve para un ping que confirme que todo el servidor está protegido. Los comandos para un solo contenedor están en su página.

**Copiar todo ahora** en la página Copias de seguridad inicia a mano la misma pasada, sobre todas las secciones activadas.

## Calendarios por sección {#plans-per-section}

Una sección puede salir de la copia total y ejecutarse a su hora. En **Calendarios por sección**, **Excepción** añade una sección con un calendario propio, y **Eliminar excepción** la devuelve. La línea de la tarjeta dice qué hacen las demás, como **Todas las demás secciones se ejecutan en la copia total.**

Una sección con calendario propio no se vuelve a copiar en la copia total, así que nada se copia dos veces. Lo mismo vale para un solo elemento con calendario propio.

**Copias externas por sección** funciona igual para las copias. Por defecto cada sección copia a sus destinos externos justo después de cada copia. **Excepción** da a una sección un calendario de copia propio, por ejemplo para que las VMs copien a Backblaze B2 solo los domingos por la noche. La tarjeta **Copiar** propia de un destino también puede fijar **Cuándo copiar**, consulta [Copias externas](offsite-recovery.md#off-site-copies).

## Elementos programados {#scheduled-entries}

**Elementos programados** dice por sección cuántos elementos participan, como **38 de 41 programados**. **Elementos** abre la lista con un interruptor **En el calendario** por elemento. Un elemento desactivado solo se copia cuando lo inicias a mano. Cuenta como sin protección salvo que lo marques como **No copiar**.

## Orden {#order}

**Orden** fija el orden en que las ejecuciones programadas y la copia total copian contenedores y VMs. Arrastra un elemento a su sitio o usa las flechas, y cada movimiento se guarda al instante. Los elementos que dejes sin ordenar se copian empezando por los más atrasados. Una copia suelta a mano no se ve afectada.

**Reinicio tras la copia de seguridad** en la misma tarjeta cubre los contenedores que **Detener otros contenedores** paró durante una copia. Vuelven a arrancar en su orden de dependencias, y con **Esperar a que las dependencias estén sanas antes de iniciar el siguiente** cada uno tiene que informar de que está sano o en ejecución antes de que arranque el siguiente. **Tiempo de espera de salud por contenedor (segundos)** limita la espera, 120 por defecto, en un rango de 5 a 3600.

## Instantánea en vez de parada {#snapshot-instead-of-stop}

**Detener los contenedores en ZFS solo para una instantánea** está activado por defecto. Un contenedor cuyos datos están en ZFS se detiene solo mientras BombVault toma una instantánea de esos datos, normalmente unos segundos, y la copia se lee entonces de la instantánea. Necesita la conexión SSH al host que usan los conjuntos de datos ZFS. Un contenedor con parte de sus datos en otro sitio se detiene durante toda la copia como antes, y su ejecución dice por qué, mientras que un contenedor sin ningún dato en ZFS se detiene sin nota. Funciona en un recurso compartido de usuario de Unraid que esté en un único pool ZFS. En TrueNAS la app necesita además la capacidad `SYS_CHROOT`, consulta [TrueNAS SCALE](getting-started.md#truenas). Un solo contenedor puede configurarse de otra manera en su propia página.

La copia queda bajo las mismas rutas y etiquetas que una hecha con el contenedor detenido, así que las restauraciones y comprobaciones tratan ambas por igual. El comando antes de la copia y el volcado de la base de datos se ejecutan igualmente primero, los demás contenedores que hay que detener se paran y vuelven con él, y el comando después de la copia se ejecuta en cuanto el contenedor vuelve a estar en marcha. Una actualización tras la copia sigue esperando a que termine la copia. Sus ejecuciones muestran **Instantánea** y cuántos segundos estuvo parado el contenedor. Si BombVault se detiene a mitad de la copia de cualquier contenedor, vuelve a iniciar los contenedores que mantenía parados la próxima vez que arranca.

## Actualizaciones tras la copia {#updates}

- **Actualizar los contenedores tras una copia de seguridad correcta** descarga una imagen más nueva solo después de que haya terminado la copia anterior, de modo que siempre existe antes un punto de restauración reciente. Está desactivado por defecto, y un solo contenedor puede configurarse de otra manera en su página, en **Actualizar tras una copia de seguridad correcta**.
- **Eliminar la imagen antigua tras una actualización** borra la imagen que dejó el contenedor. Una imagen base que comparten otros contenedores nunca se borra.
- **Actualizar el estado de actualización de Unraid tras actualizar un contenedor** borra el aviso de actualización de Unraid para ese contenedor.

Un mensaje por contenedor actualizado se fija en **Ajustes, Notificaciones** con **Notificar al actualizar un contenedor**.

## Inactividad antes de la copia {#idle}

La tarjeta **Inactividad antes de la copia** fija cuándo cuenta una app como inactiva para un contenedor cuya copia programada espera a que lo esté: **CPU por debajo de (% de un núcleo)**, **Tráfico por debajo de (Mbit/s)** y **Durante al menos (min)**. Un servidor multimedia cuenta como inactivo cuando no hace streaming, y para un contenedor en la red del host solo cuenta la CPU. La espera en sí se fija en cada contenedor, consulta [Cuando la app está ocupada](backups.md#app-busy).

## Un calendario propio para un elemento {#own-plan}

Cada elemento tiene **Calendario propio** en la tarjeta **Calendario y destino** de su pestaña Ajustes. Con un calendario ahí, el elemento se ejecuta a sus propias horas y sale de la copia total y del calendario de su sección. Sin él, los sigue.

## Ejemplo: tres calendarios que encajan {#example}

Un equipo Unraid típico: la mayoría de los contenedores se pueden copiar de noche, las VMs son grandes y cambian despacio, e Immich recibe fotos nuevas todo el día.

1. **Copia total a las 03:00.** Abre **Ajustes, Calendario**. En **Copia total**, pon **Cuándo se ejecutan las copias** en **Diario** a las `03:00`. Activa **Recuperar copias perdidas tras el arranque**, para que se compense una noche con el servidor apagado.
2. **VMs una vez por semana.** En **Calendarios por sección**, pulsa **Excepción**, elige **VMs** y pon su calendario en **Semanal**, **Días** domingo, **Hora** `04:00`. La tarjeta dice ahora que todas las demás secciones se ejecutan en la copia total.
3. **Las VMs también copian fuera los domingos.** En **Copias externas por sección**, pulsa **Excepción**, elige **VMs** y pon **Semanal**, domingo a las `06:00`, cuando su copia ya haya tenido tiempo de terminar.
4. **Immich cada 6 horas.** Abre **Copias de seguridad**, luego `immich` y su pestaña **Ajustes**. Pon **Calendario propio** en **Cron** con `0 */6 * * *`.

Qué se ejecuta ahora:

| Cuándo | Qué |
|---|---|
| Cada día a las 03:00 | Los contenedores salvo `immich`, el flash, las carpetas, los conjuntos de datos ZFS y la autocopia, una sección tras otra |
| Domingo a las 04:00 | VMs |
| Domingo a las 06:00 | La copia externa de las VMs |
| 00:00, 06:00, 12:00 y 18:00 | `immich` por su cuenta |

La tarjeta **Semana** muestra la línea de las 03:00 todos los días, los trazos de las 04:00 y las 06:00 el domingo, y cuatro trazos al día para `immich`. `immich` no se copia una segunda vez a las 03:00, porque un elemento con calendario propio sale de la copia total. Las copias de las demás secciones se siguen ejecutando justo después de cada copia.

Cuando más adelante quites el calendario propio de `immich`, volverá a unirse a la ejecución de las 03:00 con su sección.
