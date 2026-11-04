# Anomalías

El ransomware y una app rota se parecen en una copia de seguridad: de repente la mayoría de los datos son nuevos, o falta la mayoría. BombVault compara cada copia con el historial propio de ese elemento y genera un hallazgo cuando algo parece raro. Mientras un hallazgo sobre datos perdidos está abierto, también conserva las copias antiguas del elemento, de modo que la última buena no pueda caducar. La entrada **Anomalías** de la barra lateral aparece mientras la detección está activada.

![La página Anomalías con un hallazgo crítico abierto.](assets/screenshots/anomalies/page-dark.png#only-dark)
![La página Anomalías con un hallazgo crítico abierto.](assets/screenshots/anomalies/page-light.png#only-light)

## La página {#page}

Los mosaicos **Crítica**, **Advertencias** y **Avisos** eligen una gravedad, y **Abierta** y **Cerrada** sobre la lista alternan entre lo que está abierto y lo que se cerró en los últimos 30 días. Un hallazgo crítico está en la misma superficie que los demás, y su gravedad se ve en su insignia y en su borde de color. Un hallazgo ofrece primero su solución, y sus detalles muestran el valor como una curva sobre el rango habitual.

Cuando no hay nada abierto, **Progreso del aprendizaje** muestra un anillo de diez partes por elemento, una parte por cada copia de la que BombVault ha aprendido. **Supervisión** abre la lista de elementos con sus propios ajustes.

## Qué comprueba {#checks}

Cada copia de un contenedor, una VM, un conjunto de carpetas, un volcado de base de datos, la unidad flash y la autocopia se compara con el historial propio del elemento. Las comprobaciones miran:

- los datos nuevos que guardó una ejecución, frente a las mayores cantidades habituales de las copias recientes y frente al ritmo habitual por hora,
- una copia que volvió a guardar la mayor parte de los datos, lo que incluye archivos que se renombraron y se reescribieron,
- el tamaño del origen y el número de archivos que restic informa para cada elemento y cada volcado de base de datos,
- el tiempo de copia propio de restic,
- rachas de fallos y fallos intermitentes,
- comprobaciones de restauración que dejaron de pasar,
- el espacio libre de los repositorios locales, SFTP y rclone, proyectado a partir del crecimiento del repositorio.

Un elemento aprende de sus primeras 10 copias. Un origen casi vacío, una reescritura de la mayor parte de los datos y los fallos se comprueban desde el principio.

Un hallazgo sobre el tamaño o el número de archivos de un origen se fecha en la primera copia en la que apareció. **Comparar con la copia anterior** lista las carpetas donde desaparecieron, entraron o cambiaron archivos, con una nota cuando casi todo está en un índice de búsqueda, una caché o miniaturas que la app reconstruye por sí sola.

Los elementos ZFS se vigilan conjunto de datos por conjunto de datos, consulta [Conjuntos de datos ZFS](zfs-datasets.md#anomalies). Un asistente puede leer los hallazgos abiertos a través del [servidor MCP](mcp.md#tools).

Las comprobaciones tienen límites. Los repositorios S3, B2 y REST no tienen cifra de espacio libre, en el recurso compartido de usuario de Unraid el espacio libre es el de todo el array, y las copias hechas antes de restic 0.17 no llevan historial de tamaño.

## Cerrar un hallazgo {#closing}

- Las **Advertencias** se cierran solas cuando la causa desaparece.
- Los hallazgos **Críticos** sobre datos perdidos y un disco que se llena se quedan hasta que actúes.
- **Confirmar** marca un hallazgo como visto y lo cierra. No se vuelve a generar hasta que su causa haya desaparecido una vez.
- **Marcar como esperada** dice que el cambio fue tuyo. El nuevo nivel pasa a ser normal tras 10 copias, pero la comprobación de casi vacío nunca se desactiva.
- Una selección que cambia, como una carpeta que marcaste o desmarcaste, reinicia por sí sola el historial del elemento.

Se envía una notificación una vez por episodio, y los fallos y las comprobaciones de restauración que ya notifican no se comunican dos veces.

## Mientras tanto se conservan las copias antiguas {#retention-hold}

Mientras un origen está casi vacío, se ha reducido mucho o ha vuelto a guardar la mayor parte de sus datos, la retención y la poda dejan en paz las copias antiguas de ese elemento, y el hallazgo enlaza con la última copia buena, que la ventana de restauración preselecciona. Antes de que cierres un hallazgo así, BombVault avisa de que la próxima limpieza volverá a borrar copias antiguas según las reglas de conservación. Así que si faltan datos o se cifraron, restaura primero y cierra el hallazgo después.

## Ajustes {#settings}

La tarjeta **Anomalías** de **Ajustes, Verificación** contiene los ajustes globales. Cada control se guarda en cuanto lo cambias, y los tres de debajo del interruptor se ocultan mientras la detección está desactivada.

| Ajuste | Valor predeterminado | Qué hace |
|---|---|---|
| **Detectar anomalías** | Activado | Compara cada copia con el historial propio del elemento. Si se desactiva, no se comprueba nada nuevo y la entrada **Anomalías** sale de la barra lateral. La tarjeta sigue enlazando con los hallazgos anteriores. |
| **Sensibilidad** | Equilibrada | Estricta informa de cambios más pequeños, Permisiva solo de los grandes. |
| **Enviar una notificación para** | Solo hallazgos críticos | La gravedad más baja que envía un mensaje por los canales configurados en [Notificaciones](notifications.md). Los fallos repetidos de copias y volcados y las comprobaciones de restauración programadas fallidas ya envían su propio mensaje y no se envían dos veces. |
| **Conservar las copias antiguas cuando un origen se reduce mucho o se reescribe** | Activado | Mientras un elemento tiene un hallazgo abierto por un origen casi vacío, una reducción brusca o la mayor parte de sus datos guardados otra vez, la retención y la poda dejan en paz las copias antiguas de ese elemento. Confirma el hallazgo o márcalo como esperado para dejar que se vayan. |

Cada elemento puede usar su propia sensibilidad y su propio mínimo de notificación. Configúralos en la pestaña **Verificación** del elemento, o en la página **Anomalías** en **Supervisión**. Para un elemento ZFS se aplican a todos los conjuntos de datos de su árbol.
