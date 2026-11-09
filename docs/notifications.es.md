# Notificaciones

BombVault puede avisarte cuando una copia falla, cuando tiene éxito, cuando una va con retraso y, una vez a la semana, cómo han ido las cosas. Todo se configura en **Ajustes, Notificaciones**: lo que BombVault envía aparte de las copias sueltas en la tarjeta **Notificaciones**, y adónde van los mensajes en la tarjeta **Canales**. Cada canal decide por sí mismo cuándo avisa, y una comprobación de Healthchecks también es un canal.

![Ajustes, Notificaciones con el canal de Gotify rellenado.](assets/screenshots/notifications/gotify-dark.png#only-dark)
![Ajustes, Notificaciones con el canal de Gotify rellenado.](assets/screenshots/notifications/gotify-light.png#only-light)

## Qué envía un mensaje {#what}

Los interruptores de la tarjeta **Notificaciones** valen para todos los canales:

- **Resumir las ejecuciones programadas**: un mensaje por ejecución programada, como *42 de 45 correctas*, en lugar de uno por contenedor o VM. Una copia que inicias a mano sigue enviando su propio mensaje. Un invitado congelado, una retención fallida o un presupuesto de crecimiento superado salen de inmediato incluso con los resúmenes activados.
- **Notificar al actualizar un contenedor**: un mensaje por cada contenedor que **Actualizar los contenedores tras una copia de seguridad correcta** pasó a una imagen más nueva, para que puedas comprobar que sigue funcionando.
- **Avisar de copias atrasadas**: una vez al día a las 09:00 BombVault comprueba si una copia activada va con retraso, es decir, si es más antigua que el doble de su calendario, y envía un mensaje por caso, con el nombre del elemento que se quedó atrás. La siguiente copia correcta lo borra.
- **Resumen semanal**: un resumen a la semana con el número de ejecuciones, los datos de copia nuevos, qué secciones van atrasadas, si las copias externas están al día y los principales fallos. Su calendario es un botón como cualquier otro calendario, y **Desactivado** lo apaga. Consulta [Calendarios](schedules.md#plans).

Si un canal también avisa de los éxitos se fija en el propio canal, en **Notificar**. Qué anomalías envían un mensaje se fija en **Ajustes, Verificación**, consulta [Anomalías](anomalies.md). Una copia aplazada envía un mensaje cuando empieza a esperar y otro cuando termina la espera, no uno por intento.

## Canales {#channels}

**Añadir canal**, al pie de la tarjeta **Canales**, ofrece **Notificaciones de Unraid**, **ntfy**, **Gotify**, **Discord**, **Slack**, **Matrix**, **Correo (SMTP)**, **Apprise**, **Webhook** y **Healthchecks**. Añade tantos como quieras, también dos del mismo tipo, como un tema de ntfy para los fallos y otro para todo. Un canal nuevo empieza desactivado, así que se guarda antes de que rellenes sus campos.

Cada canal tiene:

- su propio interruptor, que lo apaga sin perder sus campos.
- **Notificar**: **Nunca**, **Solo en caso de fallo** o **En éxito y en fallo**. Un canal nuevo empieza en **Solo en caso de fallo**. Una copia que salió bien pero dejó algo fuera también llega a un canal de solo fallos.
- **Nombre**, que es opcional y aparece en lugar del tipo, práctico cuando tienes dos del mismo tipo.
- **Enviar prueba**, que envía un mensaje de prueba por este canal con los valores tal como están en el formulario, guardados o no. La línea bajo el canal dice si el servicio lo aceptó, y si no, por qué.
- **Quitar canal**, la papelera de al lado.

Cada canal envía por su cuenta con su propio límite de tiempo, así que un host que nunca responde no frena a los demás. Cuando un servicio falla de una forma que otro intento podría arreglar, el canal lo intenta hasta tres veces. Una alerta de retraso, de presupuesto o de anomalía que ningún canal aceptó se vuelve a intentar más tarde, y lo mismo pasa con un resumen semanal.

| Canal | Campos | Notas |
|---|---|---|
| **Notificaciones de Unraid** | ninguno | Envía al propio sistema de notificaciones de Unraid, que puede reenviar a Pushover, correo o Discord. Funciona por la conexión SSH de **Ajustes, Integraciones, SSH del host**, así que esa clave tiene que estar autorizada. En TrueNAS y otros hosts se queda en silencio, y el canal dice por qué. |
| **ntfy** | **URL del tema**, **Token de acceso (opcional)** | La dirección termina en tu tema. Solo hace falta un token para un tema protegido. |
| **Gotify** | **URL del servidor**, **Token de la aplicación**, **Prioridad si falla**, **Prioridad si sale bien** | Las prioridades van de 0 a 10 y empiezan en 8 para los fallos y en 5 para los éxitos. |
| **Discord** | **URL del Webhook** | La dirección de un webhook de los ajustes del canal de Discord. |
| **Slack** | **URL del Webhook** | La dirección de un webhook entrante de tu app de Slack. |
| **Matrix** | **URL del Homeserver**, **Token de acceso**, **ID de la sala** | La cuenta detrás del token tiene que estar en la sala. |
| **Correo (SMTP)** | **Host SMTP**, **Puerto**, **Cifrado**, **Usuario**, **Contraseña**, **Dirección de origen**, **Dirección de destino** | **Cifrado** es **STARTTLS**, **TLS (implicit)** o **None**. |
| **Apprise** | **URL de la API de Apprise**, **Etiquetas (opcional)**, **Direcciones de Apprise (opcional)** | Apúntalo al endpoint `/notify/<key>` de tu contenedor Apprise API para llegar a sus más de 100 servicios, y las etiquetas eligen un subconjunto de los destinos de esa clave. O escribe direcciones de Apprise, una por línea, como `tgram://` para Telegram o `pover://` para Pushover, y termina la URL de la API en `/notify` sin clave. |
| **Webhook** | **URL del Webhook**, **Método**, **Cabeceras**, **Cuerpo** | Una petición que montas tú mismo, consulta [Webhook](#webhook). |
| **Healthchecks** | **URL de ping** | Consulta [Healthchecks](#healthchecks). |

Los secretos como los tokens, la contraseña SMTP y los valores de las cabeceras del webhook se guardan cifrados, y el servidor nunca los devuelve a la página. Bajo un secreto guardado el campo dice **Guardado. Deja el campo vacío para conservarlo.** Cuando cambias adónde envía un canal, BombVault no envía el secreto antiguo al sitio nuevo: el campo dice **La nueva dirección se guarda en cuanto vuelvas a introducir esto.**, y la dirección nueva se guarda cuando lo escribes.

Los ajustes de una versión anterior a la lista de canales se trasladan en el primer arranque, y no se pierde nada. Cada canal que tenías rellenado pasa a ser un elemento propio con la opción de **Notificar** que tenías. Un webhook con el formato Discord, Slack, Gotify o ntfy pasa a ser un canal de ese tipo, y un webhook Generic JSON sigue enviando `title`, `message` y `ok` a través de su **Cuerpo**. El interruptor de Unraid pasa a ser un canal de Unraid, y la URL de Healthchecks con sus excepciones, un canal de Healthchecks.

### Webhook {#webhook}

Un canal **Webhook** envía una petición HTTP a cualquier dirección. **Método** es GET, POST o PUT. **Añadir cabecera** añade una fila en **Cabeceras**, y los valores de las cabeceras se guardan como secretos, así que una cabecera `Authorization` está segura ahí. **Cuerpo** se envía tal como lo escribes, con estos marcadores rellenados:

| Marcador | Se convierte en |
|---|---|
| `%%title%%` | el título del mensaje |
| `%%body%%` | el propio mensaje |
| `%%ok%%` | `true`, o `false` si es un fallo |
| `%%event%%` | lo que pasó, como `container.failed` |
| `%%severity%%` | `info`, `success`, `warning`, `failure` o `critical` |
| `%%importance%%` | el nivel que Unraid usa para ello: `normal`, `warning` o `alert` |
| `%%link%%` | un enlace que va con el mensaje, vacío si no hay ninguno |
| `%%key%%` | la sección, como `container` o `VM` |
| `%%time%%` | la hora en UTC, como `2026-11-09T03:00:00Z` |
| `%%source%%` | `BombVault` |

Los marcadores también funcionan en la dirección y en los valores de las cabeceras, y cada valor se escapa según dónde está: como JSON cuando la cabecera `Content-Type` dice JSON, y como valor de consulta en la dirección. Un webhook nuevo empieza con esa cabecera y envía `{"title":"%%title%%","message":"%%body%%","ok":%%ok%%}`. Un nombre que BombVault no conoce se queda tal cual, así que una errata se ve en el mensaje. Deja **Cuerpo** vacío para no enviar ninguno.

## Healthchecks {#healthchecks}

Un servicio como [Healthchecks.io](https://healthchecks.io) da la alarma cuando BombVault deja de fichar. Añade un canal **Healthchecks** y pega la **URL de ping** de la comprobación, y BombVault le hace ping durante todo el ciclo de vida de cada copia: al empezar, y luego al terminar bien o con `/fail`. Este canal no tiene la opción **Notificar**, porque una comprobación necesita sus pings de éxito para seguir en verde. Una sección sin ningún sitio al que copiar cuenta como fallida, así que su comprobación se pone en rojo en lugar de recibir un éxito.

Las secciones con la misma URL comparten una comprobación. En cuanto falla una copia de una de ellas, las copias correctas de las demás también hacen ping a `/fail` y nombran el elemento que falló, así que la comprobación sigue en rojo hasta que ese elemento vuelva a tener una copia correcta o confirmes el fallo en **Necesita atención**.

Una sección puede hacer ping a una dirección propia: **Excepción** en el canal de Healthchecks añade una, y la línea dice entonces **Todas las demás secciones hacen ping a esta URL.** **Enviar prueba** escribe una entrada en el registro de cada comprobación, lo que deja su estado como está.

## Ejemplo: Gotify en tu teléfono {#example-gotify}

Tienes Gotify en `https://gotify.example.com` y quieres que los fallos suenen en tu teléfono mientras los éxitos llegan en silencio.

1. En Gotify, abre **Apps**, crea una aplicación llamada `BombVault` y copia su token.
2. En BombVault, abre **Ajustes, Notificaciones** y activa **Resumir las ejecuciones programadas**, para que una noche de 45 copias envíe un mensaje en lugar de 45.
3. En **Canales**, pulsa **Añadir canal** y elige **Gotify**.
4. Introduce `https://gotify.example.com` como **URL del servidor**. BombVault añade `/message` por sí mismo.
5. Pega el token en **Token de la aplicación**.
6. Pon **Notificar** en **En éxito y en fallo**. Deja **Prioridad si falla** en 8 y **Prioridad si sale bien** en 5. La app de Android de Gotify trata el 8 y superiores como altos, así que solo los fallos hacen ruido.
7. Activa el canal y pulsa **Enviar prueba**. La línea bajo el canal dice si Gotify lo aceptó, y un token rechazado se nombra como tal.

La siguiente ejecución programada envía un resumen con prioridad 5, o con 8 si algo falló.

## Ejemplo: ntfy sin cuenta {#example-ntfy}

ntfy no necesita cuenta para un tema público. Elige un nombre de tema difícil de adivinar, porque cualquiera que lo conozca puede leerlo.

1. En **Canales**, pulsa **Añadir canal** y elige **ntfy**.
2. Introduce la dirección del tema como **URL del tema**, como `https://ntfy.sh/bombvault-tower-7f3k`, o un tema de tu propio servidor ntfy. Deja **Token de acceso (opcional)** vacío para un tema público.
3. Suscríbete al mismo tema en la app de ntfy, activa el canal y pulsa **Enviar prueba**.

BombVault envía el mensaje como cuerpo con su título en la cabecera `Title`. Un fallo sale con prioridad alta y la etiqueta `warning`, así que destaca en la app, y una VM que se quedó congelada sale como urgente.

## Cuando los mensajes no llegan {#troubleshooting}

Consulta [Las notificaciones no llegan](troubleshooting.md#notifications-do-not-arrive) en Solución de problemas.
