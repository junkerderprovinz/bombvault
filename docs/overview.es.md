# Resumen

El resumen es la primera página de la barra lateral y la que conviene mirar cada mañana. Responde a dos preguntas de un vistazo: ¿está todo copiado?, ¿necesita algo tu atención? Empieza con una cabecera de estado y cuatro tarjetas, y hay más tarjetas esperando en **Personalizar**.

![El resumen tras unos días: la cabecera de estado, Necesita atención con un aviso, la protección de cada sección, los destinos y el historial.](assets/screenshots/overview/home-dark.png#only-dark)
![El resumen tras unos días: la cabecera de estado, Necesita atención con un aviso, la protección de cada sección, los destinos y el historial.](assets/screenshots/overview/home-light.png#only-light)

## La cabecera de estado {#status-head}

La cabecera está en lo alto de la página y siempre se queda ahí. Su anillo muestra cuánto está protegido, por ejemplo **41 de 43 protegidos**, y la línea de al lado dice cómo están las cosas:

| La cabecera dice | Qué significa |
|---|---|
| **Todo copiado** | Cada elemento de cada sección activada tiene una copia al día. |
| **Copiado, con avisos** | Todo está copiado, y algunas ejecuciones dejaron un aviso, como archivos que no pudieron leer. |
| **Todavía no todo está protegido** | Algunos elementos no tienen una copia al día, por ejemplo porque esperan su primera. |
| **3 cosas necesitan atención** | Algo falló o espera una decisión. La tarjeta **Necesita atención** lo lista. |
| **Copiando ahora** | Hay una copia en marcha. El anillo se convierte en un anillo de progreso con las estaciones **Detener**, **Leer** e **Iniciar**, y una copia total cuenta sus elementos sobre la marcha, por ejemplo **Copiando ahora: 12 de 43**. **Ver ejecución** abre la ejecución. |
| **Copia externa en curso** | Hay una copia a un destino en marcha. No muestra porcentaje, porque `restic copy` no informa de ninguno. |

Debajo de la línea están cuándo fue la última ejecución y cuándo empieza la siguiente prevista.

## Necesita atención {#needs-you}

Esta tarjeta lista lo que deberías mirar, lo más reciente primero, y dice **Nada que hacer.** cuando la lista está vacía. Cada fila tiene su propia salida:

- **Una copia fallida** nombra el elemento y su motivo. **Ver errores** muestra la salida propia de restic, y **Reintentar** inicia la copia otra vez.
- **Falló la copia externa** nombra la sección y desde cuándo. **Replicar ahora** lo intenta de nuevo.
- **Anomalía en plex** lleva al hallazgo con **Revisar**. Mientras un hallazgo crítico está abierto, la retención conserva las copias antiguas de ese elemento.
- Los elementos **Nuevos** que BombVault encontró y que ninguna copia cubre todavía, como un contenedor que acabas de instalar, ofrecen **Empezar a copiar** o **No copiar**.
- **Una copia que se rindió** porque su app siguió ocupada durante toda la espera, consulta [Cuando la app está ocupada](backups.md#app-busy).
- **Una réplica ZFS fallida**, con palabras sencillas, por ejemplo que el servidor no tiene el comando zfs, con la salida del servidor en **Ver**.
- **Las copias están detenidas: la clave ya no se puede abrir** aparece cuando este servidor adoptó la clave de otro servidor y `APP_KEY` cambió después. Consulta [Solución de problemas](troubleshooting.md#backups-stopped-and-say-the-key-can-no-longer-be-opened).
- **Guarda tu kit de recuperación** se queda hasta que hayas descargado el kit y pulsado **Lo he guardado de forma segura**.

## Estado de protección {#protection-status}

Cada sección activada tiene una fila con cuatro comprobaciones:

| Comprobación | Se pone en verde cuando | Otras palabras que puedes ver |
|---|---|---|
| **Local** | la última copia está al día | todavía sin copia, sin programación, la última ejecución falló, copia atrasada, no todos los elementos están protegidos, copia próxima |
| **Externo** | la última copia fuera de las instalaciones está al día | sin copia externa, copia en curso, copia atrasada, copia en pausa, aún no hay copia, la réplica falló |
| **Protección contra borrado** | una prueba de manipulación demostró que el otro lado rechaza los borrados | ninguna, ninguna y riesgo aceptado, aún sin demostrar, la prueba falló, sin demostrar porque su comprobación está desactivada |
| **Restauración de prueba** | pasó una restauración desde la copia externa | nunca ejecutada, en curso, fallida, atrasada, solo manual, no es posible para esta sección |

Una sección solo se pone en verde con hechos verificados. Una copia en este servidor no cuenta como externa, y un destino que marcaste con **Aceptar sin protección contra borrado** muestra un anillo gris en lugar de un punto ámbar, de modo que el color de la sección sigue a sus otras comprobaciones.

Una sección programada dice **Aún no hay copia de seguridad** y sigue neutral hasta que llega la hora de su primera ejecución. Después dice **Atrasado** y avisa una vez. Una sección sin nada que copiar, porque ninguna programación alcanza a sus elementos, dice **Sin programar** y nunca está atrasada.

Un clic en una sección abre su ventana con cada comprobación detallada, los elementos que cuenta y **Iniciar la restauración de prueba** o **Repetir la restauración de prueba**. También dice **Regla de poda definida** o **Sin regla de poda**, y la regla de conservación de la carpeta propia de una sección cuenta igual que sus reglas externas.

![La ventana de protección de la sección Contenedores con sus cuatro comprobaciones detalladas.](assets/screenshots/overview/protection-window-dark.png#only-dark)
![La ventana de protección de la sección Contenedores con sus cuatro comprobaciones detalladas.](assets/screenshots/overview/protection-window-light.png#only-light)

## Destinos {#storage-locations}

Cada destino aparece aquí como un recipiente con su nivel de llenado, lo rápido que crece, por ejemplo **Crece 4,2 GB/semana**, y cuándo estaría lleno a ese ritmo, por ejemplo **lleno en unas 31 semanas**. S3, B2 y los rest-server no informan de cuánto espacio queda, así que muestran **Espacio libre desconocido** y solo su crecimiento. Un clic lleva a la página del destino. Consulta [Destinos](offsite-recovery.md#destinations).

## Historial {#history}

**Historial** tiene dos pestañas. **Ejecuciones** lista cada ejecución con su resultado, duración y motivo, y un clic la abre. **Registro** muestra cada línea con su hora, tal como la escribe BombVault, y se puede acotar a un día. Una línea se pone naranja cuando una copia terminó pero dejó algo fuera, y la línea nombra lo que dejó fuera.

## Más tarjetas en Personalizar {#customize}

**Personalizar**, en lo alto de la página, te deja organizar las tarjetas. Arrastra una tarjeta por su barra para moverla, cambia su tamaño en el punto de su esquina inferior u ocúltala. Las tarjetas ocultas esperan en **Tarjetas ocultas**, y **Mostrar** devuelve una. Mientras las organizas las tarjetas se balancean un poco, y **Listo** lo termina. La disposición se guarda en este navegador.

Estas tarjetas empiezan ocultas:

- **Anomalías**: los hallazgos abiertos, consulta [Anomalías](anomalies.md).
- **Sin copia de seguridad**: todo lo de este servidor que ninguna copia programada cubre, cada cosa con su motivo, como **nunca añadido**, **no incluido en la programación** o **todos los destinos de su sección están desactivados**. Lee los contenedores del propio Docker, así que también encuentra uno que BombVault nunca ha visto.
- **Calendario**: los últimos 90 días, un cuadrado por día. Un clic en un día muestra sus líneas en el registro de **Historial**.
- **Almacenamiento**: cuánto ocupa cada sección y lo bien que se deduplican sus datos.
- **Resumen**: la instantánea más antigua de todas las secciones, cuándo se ejecuta la próxima copia y cómo fue la última.
- **Estadísticas**: cuántos contenedores y VMs hay, cuántos errores y cuántos calendarios están activos o en pausa.
- **Integración con el host**: lo que BombVault alcanza y necesita en el host, como el socket de Docker, libvirt, restic y rclone. Empieza aquí cuando algo parezca mal conectado.

![El modo Personalizar: las tarjetas se pueden arrastrar, cambiar de tamaño y ocultar, y las ocultas esperan debajo.](assets/screenshots/overview/customize-dark.png#only-dark)
![El modo Personalizar: las tarjetas se pueden arrastrar, cambiar de tamaño y ocultar, y las ocultas esperan debajo.](assets/screenshots/overview/customize-light.png#only-light)

## Cómo cuenta BombVault {#counting}

Cada tarjeta y la cabecera cuentan igual: los elementos de las secciones activadas que siguen instalados y que no marcaste como **No copiar**. Un elemento cuenta como protegido solo mientras un calendario lo alcanza y tiene una copia. Una copia antigua por sí sola no cuenta, así que un contenedor que sacaste del calendario aparece como sin protección hasta que lo devuelvas o lo marques como **No copiar**.

## Primeros pasos {#first-steps}

En una instalación nueva el resumen empieza con la tarjeta **Primeros pasos**: **Elige las secciones**, **Añadir destino** y **Primera copia**. Se queda hasta que termina la primera copia. [Primeros pasos](getting-started.md#first-run) la recorre.
