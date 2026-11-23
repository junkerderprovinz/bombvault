# Nodos

Un nodo es un BombVault sin interfaz web, por ejemplo en un VPS o en un servidor pequeño en casa de un amigo. Hace copias de los contenedores y las carpetas de su propio servidor, y el BombVault en el que lo añades, al que esta página llama **instancia base**, le dice cuándo hacerlas y adónde van. Lo gestionas desde la interfaz web de la instancia base, junto a tus propios elementos.

![La tarjeta del nodo vps-falkenstein en la página Instancias: Conectada, su versión, sus copias en el servidor receptor, cuándo copia cada sección, y Cambiar destino, Reemplazar y Liberar.](assets/screenshots/nodes/card-dark.png#only-dark)
![La tarjeta del nodo vps-falkenstein en la página Instancias: Conectada, su versión, sus copias en el servidor receptor, cuándo copia cada sección, y Cambiar destino, Reemplazar y Liberar.](assets/screenshots/nodes/card-light.png#only-light)

## Cómo funciona {#how-it-works}

- **La instancia base guarda la clave del nodo.** Cuando añades un nodo, la instancia base crea su `APP_KEY`, guarda una copia sellada y te la entrega una sola vez, en el archivo compose del nodo. Con esa copia, la instancia base llega al nodo y abre sus copias.
- **El nodo no necesita ningún puerto abierto.** La instancia base y el nodo hablan a través del repetidor por un enlace privado propio, aparte de tu grupo de emparejamiento. La instancia base inicia cada llamada y la firma, y el nodo no le pide nada salvo su saludo. El único destino que necesita un puerto abierto es [guardar las copias en el nodo](#keep-on-node).
- **Las copias nunca pasan por el repetidor.** El nodo las envía directamente al sitio que elegiste: el servidor receptor de la instancia base, uno de sus destinos o un rest-server en el propio nodo.
- **El nodo no planifica nada por su cuenta.** No tiene calendarios, no envía notificaciones y no se une a ningún grupo ni a ninguna fortaleza. La instancia base inicia sus copias, vigila cómo van y envía las alertas.
- **Un nodo copia contenedores y conjuntos de carpetas.** Las VMs, la unidad flash, los conjuntos de datos ZFS y la autocopia se quedan en las instalaciones completas.

El enlace usa el repetidor propio de la instancia base cuando su grupo tiene uno, y si no, el repetidor del proyecto, incluso cuando la instancia base se empareja sin repetidor.

## Antes de empezar {#before-you-start}

- **Una contraseña de acceso en la instancia base.** Añadir un nodo, volver a mostrar su archivo compose y abrir sus elementos requieren iniciar sesión con la contraseña, consulta [Inicio de sesión](configuration.md#login).
- **Docker con Compose en el servidor del nodo.**
- **Un sitio para las copias**, uno de los tres de [Adónde van las copias](#placement). El servidor receptor es la opción más segura, porque allí el nodo no puede borrar nada.

## Añadir un nodo {#add}

1. En la instancia base, abre **Instancias** y elige **Añadir nodo** abajo a la derecha.
2. **Nombre.** Escribe cómo aparecerá aquí el nodo, por ejemplo el nombre del servidor, y elige **Siguiente**.

    ![La ventana Añadir nodo en su primer paso, con el nombre vps-helsinki escrito.](assets/screenshots/nodes/add-name-dark.png#only-dark)
    ![La ventana Añadir nodo en su primer paso, con el nombre vps-helsinki escrito.](assets/screenshots/nodes/add-name-light.png#only-light)

3. **Copias.** Elige adónde van las copias y rellena lo que pide ese sitio. **Hace copias cada día a las** es la hora a la que empiezan las dos secciones. La instancia base propone media hora después de sus propias copias de contenedores, para que las dos no tiren de los mismos discos a la vez. Más adelante puedes dar a cada sección su propio plan en la tarjeta. Elige **Siguiente**.

    ![El segundo paso: el servidor receptor elegido, Dirección para nodos rellenada con una dirección https y Hace copias cada día a las 03:00.](assets/screenshots/nodes/add-placement-dark.png#only-dark)
    ![El segundo paso: el servidor receptor elegido, Dirección para nodos rellenada con una dirección https y Hace copias cada día a las 03:00.](assets/screenshots/nodes/add-placement-light.png#only-light)

4. **Inicio.** La ventana muestra el archivo compose del nodo con `MODE`, `APP_KEY`, `NODE_HOME` y `NODE_NAME` ya rellenados. Cópialo y guarda la `APP_KEY` en un lugar seguro. En el servidor del nodo:

    ```sh
    mkdir -p /opt/bombvault-node && cd /opt/bombvault-node
    nano docker-compose.yml   # paste the file from the window
    docker compose up -d
    ```

    Antes de arrancarlo, apunta el volumen Host Data a los datos que quieres copiar y pon `HOST_SOURCE_ROOT` en la misma ruta, como muestra el ejemplo de abajo.

5. La ventana espera al nodo. En cuanto el nodo responde, la instancia base le dice adónde van sus copias y comprueba que llega a ellas, y la ventana dice **vps-helsinki está en línea y recibe órdenes de esta instancia.**

    ![El último paso: el archivo compose y, después, el nodo en línea con sus copias en el servidor receptor.](assets/screenshots/nodes/add-start-dark.png#only-dark)
    ![El último paso: el archivo compose y, después, el nodo en línea con sus copias en el servidor receptor.](assets/screenshots/nodes/add-start-light.png#only-light)

Puedes cerrar la ventana antes de que el nodo responda. Su tarjeta dice entonces **Esperando al nodo**, y la instancia base le dice adónde van sus copias en cuanto aparece.

### Ejemplo: un VPS que copia al servidor receptor {#example-vps}

La instancia base es un equipo Unraid llamado `tower` que ejecuta el [servidor receptor](offsite-recovery.md#receiving-server). Un proxy inverso lo publica como `https://backup.tower.example.org`. El VPS `vps-helsinki` ejecuta Nextcloud y Caddy, con sus datos en `/srv/appdata`.

1. En `tower`: **Instancias**, **Añadir nodo**, nombre `vps-helsinki`, **Siguiente**.
2. Elige **El servidor receptor de esta instancia** y escribe `https://backup.tower.example.org` en **Dirección para nodos**. Los contenedores de la instancia base se copian a las 02:30, así que la ventana propone las 03:00. Déjalo así y elige **Siguiente**.
3. En el VPS, guarda el archivo de la ventana con los volúmenes y `HOST_SOURCE_ROOT` cambiados para `/srv/appdata`:

    ```yaml
    services:
      bombvault-node:
        image: junkerderprovinz/bombvault:latest
        restart: unless-stopped
        hostname: bombvault-node
        container_name: bombvault-node
        volumes:
          - ./config:/config
          - /var/run/docker.sock:/var/run/docker.sock
          - /srv/appdata:/host/user
        environment:
          MODE: node
          APP_KEY: "<filled in by home>"
          NODE_HOME: "<filled in by home>"
          NODE_NAME: "vps-helsinki"
          PLATFORM: generic
          HOST_SOURCE_ROOT: /srv/appdata
    ```

4. `docker compose up -d`. Unos segundos después, la ventana en `tower` dice que el nodo está en línea y **Las copias van a https://backup.tower.example.org.**
5. A las 03:00, la instancia base inicia las copias del nodo. A la mañana siguiente, la tarjeta **Nodos** del resumen muestra la última copia de `vps-helsinki`, y **Copias de seguridad** lista sus contenedores en una sección propia.

Cómo encuentra BombVault los datos de un contenedor en un host Docker normal se describe en [Detección del origen de la copia](configuration.md#backup-source-detection).

## Adónde van las copias {#placement}

Cada nodo tiene un solo destino para sus dos secciones. **Cambiar destino** en su tarjeta lo cambia en cuanto el nodo está en línea. El sitio nuevo recibe repositorios nuevos, y las copias del sitio anterior se quedan donde están.

### El servidor receptor de esta instancia {#to-receiver}

El nodo escribe en el servidor receptor de la instancia base en modo append-only: puede añadir copias, pero no borrar ninguna. Recibe allí un acceso propio con el nombre del nodo y un repositorio por sección en su propia carpeta. La instancia base añade su propia clave a cada repositorio y los poda ella misma, ya que el nodo no puede.

Si la instancia base aún no tiene servidor receptor, el paso lo dice y ofrece **Configurar servidor receptor**. La instancia base no necesita ningún grupo para ello. Elige una carpeta en un recurso compartido y un puerto, pulsa **Configurar y continuar** y luego **Listo** cuando la ventana muestre el servidor, y el paso sigue con él. Los detalles están en [Servidor receptor](offsite-recovery.md#receiving-server).

El nodo llega al servidor en **Dirección para nodos**, que fijas en el segundo paso o en la ventana **Servidor receptor**. Un nodo suele enviar desde internet, así que la dirección tiene que ser una de estas:

- `https://`, por ejemplo detrás de un proxy inverso, Pangolin o un túnel. Si el servidor muestra un certificado propio en lugar de uno de una autoridad pública, pégalo en formato PEM en **Certificado**.
- `http://` con una dirección IP privada, como `10.0.0.5` en un enlace WireGuard o `100.101.102.103` en Tailscale.

Un `http://` simple hacia una dirección pública se rechaza, porque el nodo enviaría sus copias por internet sin cifrar. Cuando cambias la dirección, la instancia base avisa a cada nodo del receptor, y un nodo que no está se entera en cuanto vuelve.

### Un destino {#to-location}

El nodo escribe en uno de los destinos de la instancia base al que puede llegar por sí mismo: un servicio S3 o un servidor REST, por `https://` o en una dirección privada. Sus repositorios están allí en `bombvault-node-<node id>-containers` y `bombvault-node-<node id>-files`.

El nodo inicia sesión con unas [credenciales guardadas](offsite-recovery.md#sign-ins) creadas para él, que tienen que ser distintas de las que usa la instancia base para escribir en ese destino. Si la ventana dice que no hay ninguna, añade unas primero. Con esas credenciales el nodo suele poder borrar lo que hay en el destino, a diferencia del servidor receptor, y la ventana lo avisa.

### Guardarlas en el nodo {#keep-on-node}

El nodo guarda sus copias él mismo, detrás de su propio rest-server en modo append-only, y la instancia base recoge una copia de cada sección en su propio almacenamiento. Esto sirve para un nodo al que la instancia base no puede ofrecer sitio. Es el único destino que necesita un puerto abierto.

![El segundo paso con Guardarlas en el nodo elegido, la dirección https://203.0.113.7:8443 y la nota sobre el puerto abierto.](assets/screenshots/nodes/add-keep-dark.png#only-dark)
![El segundo paso con Guardarlas en el nodo elegido, la dirección https://203.0.113.7:8443 y la nota sobre el puerto abierto.](assets/screenshots/nodes/add-keep-light.png#only-light)

1. En **Dirección del nodo**, escribe dónde llega la instancia base al nodo: su dirección pública y un puerto libre, como `https://203.0.113.7:8443`, sin carpeta detrás. Usa `http://` solo con una dirección privada, como arriba.
2. El nodo arranca el contenedor `bombvault-node-rest-server` en ese puerto, con sus repositorios en la carpeta `bombvault-node` dentro de Host Data. Con `https://` crea un certificado para esa dirección, y la instancia base lo fija.
3. Abre el puerto en el cortafuegos del nodo para que la instancia base pueda llegar a él. Un puerto que ya ocupa otro contenedor en el nodo se rechaza indicando su nombre.

La instancia base recoge una sección justo cuando terminan las copias del nodo de esa sección, y como respaldo cada hora y cuarto, también con **Recogida** desactivada. Lo que la instancia base ha recogido se queda con ella como cualquier [copia recogida](offsite-recovery.md#receiver), incluso cuando el nodo ya no existe. El nodo poda sus propios repositorios cuando la instancia base se lo pide, ya que su rest-server no acepta borrados, y sus conjuntos de carpetas dejan fuera la carpeta `bombvault-node`, para que nunca copie sus propias copias.

### Copias externas con las de la instancia base {#copy-onward}

Activa **Copiar fuera del sitio con las copias de esta instancia** en el paso **Copias** o en **Cambiar destino**, y la instancia base copia los contenedores y los conjuntos de carpetas del nodo a los destinos a los que copian sus propios Contenedores y Carpetas, con el mismo plan. El nodo no necesita para ello una carpeta propia ni un plan de copia propio.

- Las copias están en los mismos repositorios que las copias de la instancia base, bajo el host del nodo. Las reglas de la instancia base en el destino no las tocan, y la instancia base las conserva según la regla del nodo, la misma con la que [poda las copias del nodo](#schedule).
- La instancia base lee los repositorios del nodo con su propia clave y escribe con su propio acceso al destino. El nodo no recibe credenciales nuevas, y el servidor receptor sigue en modo append-only. Para un nodo que guarda sus copias él mismo, la instancia base copia lo que ha recogido.
- Los repositorios del nodo tienen ajustes de chunker propios, así que sus copias comparten pocos datos con las de la instancia base en el destino.
- En un destino append-only no se poda nada, igual que con las copias propias de la instancia base. Un [repositorio compartido](offsite-recovery.md#shared-repository) no recibe copias de un nodo, porque su propietario lo poda según las reglas de los servidores del grupo.
- La tarjeta muestra **Las copias van a** y los destinos. Una copia que falla cuenta como error en la tarjeta y envía una alerta, consulta [Vigilar tus nodos](#watch).
- Desactivado, la instancia base ya no copia nada. Las copias que ya existen se quedan, y las reglas de la instancia base siguen sin tocarlas.

#### Ejemplo: un VPS copiado a B2 con los contenedores de la instancia base {#example-copy-onward}

`tower` conserva 7 copias de cada contenedor en **Cuánto tiempo conservar** y copia sus Contenedores a un bucket de B2 cada noche a las 04:00. El bucket conserva las últimas 14. `vps-helsinki` copia al servidor receptor de `tower`, donde sus copias se podan según las mismas 7. Con el interruptor activado, `tower` copia también los contenedores del VPS al bucket a las 04:00. El bucket guarda entonces 14 copias de cada contenedor de `tower` y 7 de cada contenedor del VPS.

## Cuándo copian los nodos {#schedule}

La instancia base lleva el calendario de cada nodo. La tarjeta muestra un plan por sección, como **Contenedores** *Diario a las 2:00* y **Carpetas** *Semanal (dom) a las 4:00*. Un clic abre el mismo editor que en [Calendarios](schedules.md#plans), y un cambio se guarda al momento.

![El plan de los contenedores de un nodo abierto desde su tarjeta, con Desactivado, Diario, Semanal, Cada N días y Cron.](assets/screenshots/nodes/schedule-dark.png#only-dark)
![El plan de los contenedores de un nodo abierto desde su tarjeta, con Desactivado, Diario, Semanal, Cada N días y Cron.](assets/screenshots/nodes/schedule-light.png#only-light)

- **Un nodo que no estaba** inicia las copias que se perdió en cuanto vuelve. La instancia base mira cada cinco minutos.
- **A un nodo ocupado** se le vuelve a preguntar cinco minutos después.
- **Mientras la instancia base poda** las copias antiguas de un nodo, el nodo no inicia nada hasta que termina. En el servidor receptor y en el propio nodo, esa ventana se abre cada domingo a las 03:00 y dura hasta seis horas. En un destino, sigue el plan de copia externa de la instancia base para ese destino. La instancia base conserva las copias según su propia regla para la sección en **Cuánto tiempo conservar**, o según la regla del destino cuando este tiene una.
- **En la lista de ejecuciones del nodo**, las copias y las restauraciones que inició la instancia base aparecen como iniciadas por ella.

## Vigilar tus nodos {#watch}

La tarjeta **Nodos** del resumen lista cada nodo con su estado, su última copia, sus errores y sus anomalías abiertas. Aparece en cuanto tienes un nodo.

![La tarjeta Nodos del resumen con tres nodos: uno conectado, uno inactivo con dos errores y uno que todavía espera al nodo.](assets/screenshots/nodes/overview-card-dark.png#only-dark)
![La tarjeta Nodos del resumen con tres nodos: uno conectado, uno inactivo con dos errores y uno que todavía espera al nodo.](assets/screenshots/nodes/overview-card-light.png#only-light)

La instancia base envía una alerta por episodio a través de sus [canales de notificación](notifications.md), y la siguiente solo cuando ese episodio ha terminado:

- **Ha fallado una copia**, por ejemplo *BombVault: vps-helsinki: backup failed*, con el elemento y el motivo. Una copia posterior de ese elemento que salga bien termina el episodio.
- **El nodo está fuera de alcance**, por ejemplo *BombVault: vps-helsinki is out of reach*. La instancia base espera lo que marque el interruptor de hombre muerto más corto de los repositorios del nodo, 26 horas si no lo cambias, o 6 horas mientras el nodo todavía no tiene destino.
- **Ha fallado una copia externa**, por ejemplo *BombVault: vps-helsinki: off-site copy failed*, cuando la instancia base no ha podido copiar las copias del nodo a uno de sus destinos. La siguiente copia que sale bien termina el episodio.

La supervisión del receptor de la instancia base también cuida de cada repositorio de un nodo, con un interruptor de hombre muerto y un `restic check` diario a las 04:00, consulta [Receptor y recolector](offsite-recovery.md#receiver).

## Los elementos de un nodo {#entries}

En **Copias de seguridad**, las listas Contenedores y Carpetas tienen una sección para cada nodo debajo de tus propios elementos. El contenedor del propio nodo no aparece. **Copiar ahora** en el menú de un elemento inicia su copia en el nodo, y **Abrir** abre su página.

En la sección de un nodo de la lista Carpetas, **Añadir conjunto de carpetas** añade un conjunto de carpetas en el nodo, y **Editar** en el menú de un conjunto lo cambia. Ambos abren la misma ventana que para tus propios conjuntos. El selector de carpetas de esa ventana recorre las carpetas del nodo, pero no crea carpetas nuevas allí.

![La sección del nodo vps-falkenstein en la lista Contenedores: nextcloud, nextcloud-db, caddy y uptime-kuma.](assets/screenshots/nodes/backups-section-dark.png#only-dark)
![La sección del nodo vps-falkenstein en la lista Contenedores: nextcloud, nextcloud-db, caddy y uptime-kuma.](assets/screenshots/nodes/backups-section-light.png#only-light)

La página de un elemento de un nodo dice **En** y el nombre del nodo, y lista sus copias. **Restaurar** en una de ellas restaura el elemento en el nodo y reemplaza lo que hay ahora. Para ello se detiene el contenedor, igual que en la instancia base.

![La página de nextcloud en vps-falkenstein con sus copias y un botón Restaurar junto a cada una.](assets/screenshots/nodes/entry-dark.png#only-dark)
![La página de nextcloud en vps-falkenstein con sus copias y un botón Restaurar junto a cada una.](assets/screenshots/nodes/entry-light.png#only-light)

A través de la instancia base llegas a una lista fija de rutas propias del nodo: sus contenedores, conjuntos de carpetas, ejecuciones y anomalías, y copiarlos y restaurarlos. Los ajustes, las claves y el almacenamiento del nodo quedan fuera de alcance, y nada en esa lista borra. Consulta [Los nodos en la API](api.md#nodes).

## Reemplazar y liberar {#replace-release}

**Reemplazar** vuelve a mostrar el archivo compose del nodo, tras iniciar sesión con la contraseña. Úsalo para mover el nodo a otro servidor o para reconstruirlo: arranca el archivo allí y apaga el servidor antiguo. La misma `APP_KEY` y el mismo `NODE_HOME` lo convierten de nuevo en el mismo nodo, con sus copias y su historial. La instancia base guarda de cada nodo sus conjuntos de carpetas, sus contenedores con lo necesario para volver a crearlos y sus ejecuciones. Cuando el nodo arranca con una carpeta `config` vacía, la instancia base lo vuelve a acoger, le dice adónde van sus copias y se lo devuelve todo antes de la siguiente copia, así que el nodo puede restaurar un contenedor que ya no existe. En un servidor nuevo, un nodo que guarda sus copias él mismo empieza con repositorios vacíos. Sus copias anteriores siguen en las que recogió la instancia base. Dos servidores con la misma `APP_KEY` cuentan como un solo nodo, así que nunca ejecutes los dos.

**Liberar** deja ir a un nodo. Deja de recibir órdenes de la instancia base y ya no copia nada. Sus copias se quedan donde están, igual que cada copia que recogió la instancia base. Su acceso en el servidor receptor desaparece, y la instancia base deja de vigilar sus repositorios. Un nodo que no está cuando lo liberas conserva su tarjeta como **Liberándose** hasta que se entera. Un nodo liberado no acepta de nuevo ninguna instancia base con la misma `APP_KEY`. Para volver a usar el servidor, añádelo como un nodo nuevo y arráncalo con el archivo compose nuevo y una carpeta `config` vacía.

## Seguridad {#security}

Si alguien toma la instancia base, controla también sus nodos, y un nodo tiene en sus manos el Docker de su servidor, que allí equivale a root. Protege la instancia base con una contraseña y mantenla fuera de internet. Si alguien toma un nodo, no consigue ningún grupo de emparejamiento, ninguna fortaleza, ningún otro nodo ni ningún otro repositorio, y no puede borrar el historial en el servidor receptor. Los detalles están en [Los nodos en el modelo de seguridad](configuration.md#nodes).

El [kit de recuperación](offsite-recovery.md#recovery-kit) tiene un capítulo **Nodes** con la `APP_KEY` de cada nodo, su contraseña de restic y sus repositorios, para que puedas recuperar un nodo o restaurar sus copias solo con restic.
