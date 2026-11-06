# Ajustes

Los **Ajustes** son catorce mosaicos. Cada mosaico es una página de tarjetas, y cada cambio se escribe en cuanto lo haces, con un destello breve en el control, o una sacudida cuando el servidor lo rechaza. Algunas ventanas conservan un botón **Guardar** porque guardarlas a medias no sería seguro, como unas credenciales o la contraseña de acceso. Las explicaciones están en los globos (i) junto a cada control.

Pulsa `/` o Ctrl+K en cualquier punto de los ajustes para buscar en todos, también por los nombres que tenían las páginas en versiones anteriores. El orden de los mosaicos se puede cambiar en **Aspecto**. Donde una columna de mosaicos junto a la página la apretaría, como en una ventana de unos 1200 píxeles de ancho o en un teléfono, los mosaicos se convierten en una tira sobre la página que se desplaza hacia los lados.

![Ajustes con el mosaico General abierto.](assets/screenshots/getting-started/settings-rail-dark.png#only-dark)
![Ajustes con el mosaico General abierto.](assets/screenshots/getting-started/settings-rail-light.png#only-light)

Esta página lista cada mosaico y sus tarjetas, y enlaza con la página que explica cada una a fondo.

## General {#general}

- **Secciones** activa o desactiva cada sección: **Contenedores**, **VMs**, **Flash**, **Carpetas**, **Conjuntos de datos ZFS** y **Autocopia**. Una sección desactivada no se copia con ningún calendario, tampoco con el calendario propio de un elemento, y sus elementos no cuentan. **Copiar ahora** sigue funcionando. **Flash** solo aparece en Unraid.
- **Contenedores** contiene lo que se aplica a todos los contenedores, como **Volcados automáticos de bases de datos**. Todo lo que afecta a un solo contenedor está en su página. Consulta [Volcados de bases de datos](backups.md#database-dumps).

## Aspecto {#look}

- **Aspecto**: **Idioma** (42 idiomas), **Tema** (**Oscuro**, **Claro** o **Sistema**), **Esquinas**, **Animación** (**Desactivado**, **Sutil**, **Salvaje** o **Desatado**), **Notificaciones discretas**, que oculta los avisos de éxito como las confirmaciones de guardado, y **Orden de las páginas de ajustes**, donde mantienes pulsado un mosaico y lo arrastras a su sitio, o usas Alt y las teclas de flecha.
- **Etiquetas**: cuánto de un control se muestra, por separado para **Botones**, **Barra lateral**, **Pestañas** y **Barra inferior**: **Texto**, **Texto y símbolo**, **Símbolo**, o **Reactivo**, que muestra solo el símbolo y desliza las palabras al apuntar con el puntero. **Barra lateral** afecta solo a la disposición de escritorio y **Barra inferior** solo a la del teléfono.
- **Colores**: un **Color de acento**, o **Modo arcoíris**, donde cada fila de una lista recibe su propio color de una paleta de ocho. El Modo arcoíris añade **Modo disco**, donde los colores se deslizan despacio al siguiente, **Modo reactivo**, donde el color de una fila solo se ve mientras apuntas a ella, se ejecuta o está seleccionada, **Rotación de colores** y la propia **Paleta de colores**.

BombVault guarda el aspecto en el servidor. Un navegador que llega por primera vez lo adopta. Cuando tu sistema pide menos movimiento, eso siempre prevalece sobre **Animación**.

## Calendario {#schedule}

**Semana**, **Copia total** con **Cuándo se ejecutan las copias**, **Recuperar copias perdidas tras el arranque** y **Comandos antes y después**, **Calendarios por sección** con **Copias externas por sección**, **Elementos programados**, **Orden** con **Reinicio tras la copia de seguridad**, **Instantánea en vez de parada**, **Actualizaciones tras la copia** e **Inactividad antes de la copia**. Consulta [Calendarios](schedules.md).

## Destinos {#storage-locations}

- **Mostrar también en la barra lateral** ancla la página Destinos a la barra lateral.
- La lista de tus destinos, con **Añadir destino**. Un clic abre la página de un destino, consulta [La página de un destino](offsite-recovery.md#location-page).
- **Credenciales**: las credenciales guardadas con la estrella de la predeterminada, consulta [Credenciales](offsite-recovery.md#sign-ins).
- **Destinos por sección**: el flujo 3-2-1, donde cada línea es un interruptor, consulta [Valores predeterminados por sección](offsite-recovery.md#placement-defaults).
- **Repositorios**: repositorios hechos para elementos sueltos, consulta [Destino por elemento](offsite-recovery.md#placement).
- **Cuánto tiempo conservar** remite a los destinos, donde se fija la retención, y **Vista previa** muestra **Lo que eliminaría la próxima ejecución**, por repositorio y por elemento, antes de que ocurra. No cambia nada y no bloquea nada, así que responde incluso mientras se ejecuta una copia.
- **Copias externas**: una fila por sección con **Probar conexión**, **Replicar ahora** y **Configurar** o **Editar**, consulta [Copias externas](offsite-recovery.md#off-site-copies).
- **Ancho de banda externo**: **Límite de subida (KiB/s)** y **Límite de bajada (KiB/s)** para cada copia. 0 significa sin límite.

La tarjeta **Streaming**, que baja el límite de subida mientras Plex, Jellyfin o Emby hacen streaming, está en la propia página Destinos.

## Restauración {#restore}

- Un enlace a la página **Restauración**.
- **Carpetas de restauración**: la **Carpeta de restauración predeterminada** para una restauración en una carpeta, con **Subcarpeta por sección** y una **Excepción** por sección. Consulta [Adónde va una restauración en una carpeta](restore.md#folders).
- **Registros de contenedores**: credenciales para registros privados, como una imagen de `ghcr.io` reservada a patrocinadores. Una restauración las usa para descargar una imagen que falta, y una actualización tras la copia para comprobar si hay una más nueva. Las imágenes públicas se descargan sin ellas. **Añadir registro** ofrece registros públicos, autoalojados, los de un proveedor de nube y **Otro registro** para cualquier dirección.
- **Para emergencias**: **Descargar kit de recuperación**, que requiere una contraseña de acceso. Consulta [Kit de recuperación](offsite-recovery.md#recovery-kit).

## Notificaciones {#notifications}

**Notificaciones** con **Notificar**, los resúmenes, **Avisar de copias atrasadas**, **Notificaciones de Unraid** y el **Resumen semanal**, **Canales** con Webhook, Apprise, Matrix, Gotify y Correo (SMTP), y **Healthchecks**. Consulta [Notificaciones](notifications.md).

## Verificación {#verify}

- **Comprobar las copias**: el resultado del último `restic check` por sección, con **Desbloquear** y **Podar** bajo **Más** en cada fila. **Desbloquear** limpia un bloqueo caducado, y **Podar** aplica las reglas de conservación a demanda.
- **Comprobaciones automáticas**: la comprobación de restauración programada con su **Muestra de datos (%)** y sus destinos de prueba, **Prueba de arranque**, que además restaura un contenedor en una copia aislada y lo arranca, el **Simulacro DR externo programado** y **Comprobar la protección contra borrado**, la prueba de manipulación programada. Consulta [Simulacros de DR](offsite-recovery.md#dr-drills) y [Prueba de manipulación](offsite-recovery.md#tamper-test).
- **Anomalías**: la detección, la sensibilidad y qué hallazgos envían un mensaje. Consulta [Anomalías](anomalies.md).

## Seguridad {#security}

- **Contraseña**: la contraseña de acceso, de al menos 12 caracteres. Cambiarla o borrarla pide la actual y, con la verificación en dos pasos activada, el código.
- **Verificación en dos pasos**: un código de una app autenticadora además de la contraseña, con ocho códigos de recuperación que se muestran una vez.
- **Claves de acceso**: inicia sesión con una clave en tu teléfono, portátil o llave de seguridad. Necesitan un nombre de dominio real y un certificado en el que confíe el navegador.
- **App de Android**: **Los teléfonos pueden emparejarse con las doce palabras del grupo**, que permite a un teléfono emparejado iniciar sesión sin la contraseña.
- **Cifrado**: **Cifrar las copias**, activado por defecto.

Varias cosas requieren una contraseña de acceso, como el kit de recuperación y los depósitos de claves. Consulta [Modelo de seguridad](configuration.md#security-model).

## Instancias {#instances}

- **Mostrar también en la barra lateral** ancla la página Instancias a la barra lateral.
- **Varios servidores** activa las partes de un grupo: **Instancias**, la página con una tarjeta por cada miembro, **Receptor** y **Recogida**. Consulta [Instancias y roles](offsite-recovery.md#roles).
- Un enlace a la página Instancias, con las partes que están activadas.

## Emparejamiento {#pairing}

**Emparejamiento** con **Generar frase** o **Introducir frase**, las tarjetas de pasos, el nombre de esta instancia y **Direcciones conocidas**, la tarjeta **Repetidor** con sus cuatro formas y **Qué ve el repetidor**, los miembros del grupo, y **Servidor ZFS sin BombVault** para un destino de réplica por SSH. Consulta [Emparejar instancias](offsite-recovery.md#pairing) y [Enviar a un servidor ZFS](zfs-datasets.md#replica-push).

## Integraciones {#integrations}

- **Servidor MCP**: claves para asistentes de IA, **Elegir un cliente** y el registro de cada clave. Consulta [Servidor MCP](mcp.md).
- **Tokens de API**: tokens con nombre para scripts y paneles. Consulta [Tokens](api.md#tokens).
- **Servicios**: **Home Assistant** por MQTT, **Encontrar en la red** (mDNS como `bombvault.local`) y **Exponer /metrics** para Prometheus, con un **Token de scrape (opcional)**. Consulta [Home Assistant](api.md#home-assistant) y [Encontrar BombVault en la red](api.md#mdns).
- **Widget de panel**: un token de solo lectura y una dirección para cualquier panel que muestre un iframe, como Homepage o Heimdall, con solo el registro de actividad en vivo. Consulta [Widget de panel](api.md#dashboard-widget).
- **SSH del host**: la clave pública de BombVault y **Probar conexión**, para las copias de VMs y de conjuntos de datos ZFS y las notificaciones de Unraid. Consulta la [guía de copias de VMs](vm-backup-ssh-setup.md).

## Apps {#apps}

- **App de Android**: el APK de la versión que ejecuta tu servidor con un código QR, y la ficha de F-Droid. Consulta [App de Android](android.md#install).
- **ParleyPort**: el contenedor repetidor con su plantilla de Unraid y el comando de Docker que lo inicia.
- **BombVault Widget**: un mosaico en el panel de Unraid con el registro de actividad de BombVault y la próxima ejecución. Con una conexión SSH al host la tarjeta instala y quita el plugin por sí misma. Sin ella te da la dirección `.plg` para instalar en **Plugins, Install Plugin**.

## Sistema {#system}

- **Exportar / importar ajustes**: toda tu configuración como un archivo JSON portátil, con o sin las credenciales. La importación muestra primero **Este archivo contiene** y deja fuera los enlaces a destinos que ya no existen. **Cifrar exportaciones con age** sella las exportaciones sin formato, el ZIP del flash y el kit de recuperación para los **Destinatarios age** que indiques. Consulta [Ajustes portátiles](configuration.md#portable-settings-export-and-import).
- **Rendimiento**: **Hilos de CPU (0 = todos)**, **Límite de tamaño de la caché (MB, 0 = ilimitado)** para la caché de restic bajo `/config`, y **Límite de memoria por copia (MiB, 0 = ninguno)**, un límite blando que hace que una copia grande se ralentice en lugar de que la maten por quedarse sin memoria.

## Info {#info}

- **Versión**: BombVault, GlimStone, restic y rclone, con **Buscar actualizaciones** y **Novedades**, que cuenta los cambios que aún no has visto y abre una ventana que los muestra.
- **Ayuda**: **Manual** abre esta documentación, e **Informe de error** descarga un ZIP con los datos sensibles tapados para una incidencia de GitHub. Requiere una contraseña de acceso.
- **Acerca de BombVault**: el proyecto, su licencia y cómo contactar con el autor.
