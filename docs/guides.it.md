# Esempi pratici

Ognuno di questi passa in rassegna un compito reale dall'inizio alla fine, con nomi e numeri compilati. Poggiano sulla pagina che spiega la funzione, così i dettagli sono lì accanto.

## Primi passi {#start}

- [Il tuo primo backup di un container, dall'inizio alla fine](getting-started.md#example-first-backup): dall'installazione al kit di ripristino.

## Archiviazione e off-site {#storage}

- [Una copia off-site su Backblaze B2](offsite-recovery.md#example-s3): un bucket, una chiave per quel bucket e la prima copia. Cloudflare R2 funziona allo stesso modo.
- [Un rest-server gestito da te](offsite-recovery.md#example-rest-server): una copia off-site che rifiuta le cancellazioni, con la riga `docker run` e la prova di manomissione.
- [Cosa conserva Equilibrato](offsite-recovery.md#example-retention): la conservazione su 400 giorni di backup notturni, backup per backup.
- [I container passano a un NAS](offsite-recovery.md#example-own-folder): la cartella propria di una sezione cambia posto, e cosa succede ai backup già fatti.

## Backup e pianificazioni {#backups}

- [Tre pianificazioni che si completano](schedules.md#example): un backup completo di notte, le VM una volta a settimana e una pianificazione propria per un elemento ogni 6 ore.
- [Plex aspetta mentre qualcuno guarda](backups.md#example-plex): un backup pianificato che non interrompe un film.
- [Una VM con due snapshot Unraid](vm-backup-ssh-setup.md#example-unraid-snapshots): un backup a caldo che conserva gli snapshot e un ripristino accanto all'originale.
- [Una replica su un secondo server Unraid](zfs-datasets.md#example-replica): una copia ZFS che puoi leggere in qualsiasi momento e tre modi per ricavarne un file.

## Riavere indietro i dati {#restore}

- [Riavere un solo file](restore.md#example-single-file): un singolo file dal backup di un container della scorsa notte.
- [Un file da un backup ZFS](zfs-datasets.md#example-zfs-file): lo stesso per un dataset.
- [Un nuovo server dal kit di ripristino](offsite-recovery.md#example-emergency): un server morto torna su una macchina nuova, senza la vecchia `APP_KEY` nel template.

## Restare informati {#staying-informed}

- [Gotify sul telefono](notifications.md#example-gotify): gli errori suonano e i successi arrivano in silenzio.
- [ntfy come webhook](notifications.md#example-ntfy): messaggi senza un account.

## Più server {#several-servers}

- [Associare due server Unraid](offsite-recovery.md#worked-example-two-unraid-boxes-end-to-end): due server che si tengono a vicenda le copie, e nessuno dei due può eliminare i backup dell'altro.
- [Una scheda e un avviso in Home Assistant](api.md#home-assistant-example): BombVault su una dashboard, con un messaggio quando un backup fallisce.
