# Ejemplos prácticos

Cada uno de estos recorre un trabajo real de principio a fin, con nombres y números incluidos. Viven en la página que explica la función, así que los detalles están justo al lado.

## Primeros pasos {#start}

- [Tu primera copia de un contenedor, de principio a fin](getting-started.md#example-first-backup): desde la instalación hasta el kit de recuperación.

## Almacenamiento y copias externas {#storage}

- [Una copia externa en Backblaze B2](offsite-recovery.md#example-s3): un bucket, una clave para ese bucket y la primera copia. Cloudflare R2 funciona igual.
- [Un rest-server que ejecutas tú](offsite-recovery.md#example-rest-server): una copia externa que rechaza los borrados, con la línea `docker run` y la prueba de manipulación.
- [Qué conserva Equilibrado](offsite-recovery.md#example-retention): la retención a lo largo de dos años de copias nocturnas, copia por copia.
- [Los contenedores se mudan a un NAS](offsite-recovery.md#example-own-folder): la carpeta propia de una sección cambia de sitio, y qué pasa con las copias ya hechas.

## Copias y calendarios {#backups}

- [Tres calendarios que encajan](schedules.md#example): una copia total por la noche, las VMs una vez por semana y un calendario propio para un elemento cada 6 horas.
- [Plex espera mientras alguien mira algo](backups.md#example-plex): una copia programada que no interrumpe una película.
- [Una VM con dos instantáneas de Unraid](vm-backup-ssh-setup.md#example-unraid-snapshots): una copia en vivo que conserva las instantáneas, y una restauración junto al original.
- [Una réplica a un segundo equipo Unraid](zfs-datasets.md#example-replica): una copia ZFS que puedes leer en cualquier momento, y tres formas de sacar de ella un archivo.

## Recuperar cosas {#restore}

- [Recuperar un solo archivo](restore.md#example-single-file): un archivo suelto de la copia de anoche de un contenedor.
- [Un archivo de una copia ZFS](zfs-datasets.md#example-zfs-file): lo mismo para un conjunto de datos.
- [Un servidor nuevo a partir del kit de recuperación](offsite-recovery.md#example-emergency): un servidor muerto vuelve en un equipo nuevo, sin su antigua `APP_KEY` en la plantilla.

## Mantenerse informado {#staying-informed}

- [Gotify en tu teléfono](notifications.md#example-gotify): los fallos suenan y los éxitos llegan en silencio.
- [ntfy como webhook](notifications.md#example-ntfy): mensajes sin cuenta.

## Varios servidores {#several-servers}

- [Emparejar dos equipos Unraid](offsite-recovery.md#worked-example-two-unraid-boxes-end-to-end): dos servidores que guardan las copias del otro, y ninguno puede borrar las copias del otro.
- [Una tarjeta y una alerta en Home Assistant](api.md#home-assistant-example): BombVault en un panel, con un mensaje cuando falla una copia.
