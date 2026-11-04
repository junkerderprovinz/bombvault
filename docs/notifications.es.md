# Notificaciones

BombVault puede avisarte cuando una copia falla, cuando tiene éxito, cuando una va con retraso y, una vez a la semana, cómo han ido las cosas. Todo se configura en **Ajustes, Notificaciones**: qué dispara un mensaje en la tarjeta **Notificaciones**, adónde van los mensajes en la tarjeta **Canales**, y los pings de control en la tarjeta **Healthchecks**.

![Ajustes, Notificaciones con el canal de Gotify rellenado.](assets/screenshots/notifications/gotify-dark.png#only-dark)
![Ajustes, Notificaciones con el canal de Gotify rellenado.](assets/screenshots/notifications/gotify-light.png#only-light)

## Qué envía un mensaje {#what}

La tarjeta **Notificaciones** fija las reglas de todos los canales a la vez:

- **Notificar**: **Nunca**, **Solo en caso de fallo** o **En éxito y en fallo**, para todos los canales de abajo.
- **Resumir las ejecuciones programadas**: un mensaje por ejecución programada, como *42 de 45 correctas*, en lugar de uno por contenedor o VM. Una copia que inicias a mano sigue enviando su propio mensaje. Un invitado congelado, una retención fallida o un presupuesto de crecimiento superado salen de inmediato incluso con los resúmenes activados.
- **Notificar al actualizar un contenedor**: un mensaje por cada contenedor que **Actualizar los contenedores tras una copia de seguridad correcta** pasó a una imagen más nueva, para que puedas comprobar que sigue funcionando.
- **Avisar de copias atrasadas**: una vez al día a las 09:00 BombVault comprueba si una copia activada va con retraso, es decir, si es más antigua que el doble de su calendario, y envía un mensaje por caso. La siguiente copia correcta lo borra.
- **Notificaciones de Unraid**: envía al propio sistema de notificaciones de Unraid, que puede reenviar a Pushover, correo o Discord. Funciona por la conexión SSH de **Ajustes, Integraciones, SSH del host**, así que esa clave tiene que estar autorizada. En TrueNAS y otros hosts se queda desactivado.
- **Resumen semanal**: un resumen a la semana con el número de ejecuciones, los datos de copia nuevos, si las copias externas están al día y los principales fallos. Su calendario es un botón como cualquier otro calendario, y **Desactivado** lo apaga. Consulta [Calendarios](schedules.md#plans).

Qué anomalías envían un mensaje se fija en **Ajustes, Verificación**, consulta [Anomalías](anomalies.md). Una copia aplazada envía un mensaje cuando empieza a esperar y otro cuando termina la espera, no uno por intento.

**Enviar prueba**, al pie de la tarjeta, envía un mensaje de prueba por cada canal activado, con los valores tal como están en el formulario.

## Canales {#channels}

Cada canal tiene su propio interruptor en la tarjeta **Canales**, y sus campos aparecen cuando está activado. Cada canal envía por su cuenta con su propio límite de tiempo, así que un host que nunca responde no frena a los demás, y un mensaje que ningún canal aceptó se vuelve a intentar.

| Canal | Campos | Notas |
|---|---|---|
| **Webhook** | **URL del Webhook**, **Formato del Webhook** | Formatos: **Generic JSON**, **Discord**, **Slack**, **Gotify** y **ntfy**. Generic JSON envía `title`, `message` y `ok`. |
| **Apprise** | **URL de la API de Apprise**, **Etiquetas (opcional)** | Apúntalo al endpoint `/notify/<key>` de tu contenedor Apprise API para llegar a sus más de 100 servicios. Las etiquetas eligen un subconjunto de los destinos de esa clave. |
| **Matrix** | **URL del Homeserver**, **Token de acceso**, **ID de la sala** | El token se queda en el servidor y no se vuelve a mostrar. |
| **Gotify** | **URL del servidor**, **Token de la aplicación**, **Prioridad si falla**, **Prioridad si sale bien** | Las prioridades van de 0 a 10 y empiezan en 8 para los fallos y en 5 para los éxitos. |
| **Correo (SMTP)** | **Host SMTP**, **Puerto**, **Cifrado**, **Usuario**, **Contraseña**, **Dirección de origen**, **Dirección de destino** | **Cifrado** es **STARTTLS**, **TLS (implicit)** o **None**. |

Los secretos como el token de Matrix, el token de la aplicación de Gotify y la contraseña SMTP se guardan cifrados. Bajo un secreto guardado el campo dice **Guardado. Deja el campo vacío para conservarlo.** Cuando cambias la dirección del servidor, BombVault no envía el secreto antiguo al sitio nuevo: el campo lo vuelve a pedir, y la dirección nueva se guarda cuando lo escribes.

## Healthchecks {#healthchecks}

Un servicio como [Healthchecks.io](https://healthchecks.io) da la alarma cuando BombVault deja de fichar. Pega la **URL de ping** de la comprobación, y BombVault le hace ping durante todo el ciclo de vida de cada copia: al empezar, y luego al terminar bien o con `/fail`. Lo hace siempre que haya una URL definida, diga lo que diga **Notificar**, así que la comprobación se mantiene en verde al tener éxito incluso con mensajes solo de fallos.

Una sección puede hacer ping a una dirección propia: **Excepción** en la tarjeta añade una, y la línea dice **Todas las demás secciones hacen ping a esta URL.**

## Ejemplo: Gotify en tu teléfono {#example-gotify}

Tienes Gotify en `https://gotify.example.com` y quieres que los fallos suenen en tu teléfono mientras los éxitos llegan en silencio.

1. En Gotify, abre **Apps**, crea una aplicación llamada `BombVault` y copia su token.
2. En BombVault, abre **Ajustes, Notificaciones**. En **Notificaciones**, pon **Notificar** en **En éxito y en fallo** y activa **Resumir las ejecuciones programadas**, para que una noche de 45 copias envíe un mensaje en lugar de 45.
3. En **Canales**, activa **Gotify**.
4. Introduce `https://gotify.example.com` como **URL del servidor**. BombVault añade `/message` por sí mismo.
5. Pega el token en **Token de la aplicación**.
6. Deja **Prioridad si falla** en 8 y **Prioridad si sale bien** en 5. La app de Android de Gotify trata el 8 y superiores como altos, así que solo los fallos hacen ruido.
7. Pulsa **Enviar prueba**. La línea bajo la tarjeta dice si Gotify lo aceptó, y un token rechazado se nombra como tal.

La siguiente ejecución programada envía un resumen con prioridad 5, o con 8 si algo falló.

## Ejemplo: ntfy como webhook {#example-ntfy}

ntfy no necesita cuenta para un tema público. Elige un nombre de tema difícil de adivinar, porque cualquiera que lo conozca puede leerlo.

1. En **Canales**, activa **Webhook**.
2. Introduce la dirección del tema como **URL del Webhook**, como `https://ntfy.sh/bombvault-tower-7f3k`, o la dirección de tu propio servidor ntfy.
3. Pon **Formato del Webhook** en **ntfy**.
4. Suscríbete al mismo tema en la app de ntfy y pulsa **Enviar prueba**.

BombVault envía el mensaje como cuerpo con su título en `X-Title`. Un fallo sale con prioridad alta y la etiqueta `warning`, así que destaca en la app.

## Cuando los mensajes no llegan {#troubleshooting}

Consulta [Las notificaciones no llegan](troubleshooting.md#notifications-do-not-arrive) en Solución de problemas.
