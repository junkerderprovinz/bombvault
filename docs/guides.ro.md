# Exemple lucrate

Fiecare dintre ele parcurge o sarcină reală de la început până la sfârșit, cu nume și cifre completate. Se sprijină pe pagina care explică funcția, așa că detaliile sunt chiar alături.

## Primii pași {#start}

- [Primul tău backup de container, de la un capăt la altul](getting-started.md#example-first-backup): de la instalare până la kitul de recuperare.

## Stocare și off-site {#storage}

- [O copie off-site pe Backblaze B2](offsite-recovery.md#example-s3): un bucket, o cheie pentru acel bucket și prima copie. Cloudflare R2 funcționează la fel.
- [Un rest-server pe care îl rulezi tu](offsite-recovery.md#example-rest-server): o copie off-site care refuză ștergerile, cu linia `docker run` și testul de manipulare.
- [Ce păstrează Echilibrat](offsite-recovery.md#example-retention): păstrarea pe doi ani de backupuri nocturne, backup cu backup.
- [Containerele se mută pe un NAS](offsite-recovery.md#example-own-folder): folderul propriu al unei secțiuni își schimbă locul și ce se întâmplă cu backupurile deja făcute.

## Backupuri și programări {#backups}

- [Trei planuri care se potrivesc](schedules.md#example): un backup complet noaptea, VM-urile o dată pe săptămână și un plan propriu pentru un element la fiecare 6 ore.
- [Plex așteaptă cât timp se uită cineva](backups.md#example-plex): un backup programat care nu întrerupe un film.
- [Un VM cu două instantanee Unraid](vm-backup-ssh-setup.md#example-unraid-snapshots): un backup live care păstrează instantaneele și o restaurare lângă original.
- [O replică pe un al doilea server Unraid](zfs-datasets.md#example-replica): o copie ZFS pe care o poți citi oricând și trei moduri de a scoate din ea un singur fișier.

## Recuperarea datelor {#restore}

- [Recuperează un singur fișier](restore.md#example-single-file): un singur fișier din backupul de container de azi-noapte.
- [Un fișier dintr-un backup ZFS](zfs-datasets.md#example-zfs-file): același lucru pentru un set de date.
- [Un server nou din kitul de recuperare](offsite-recovery.md#example-emergency): un server mort revine pe o stație nouă, fără vechiul `APP_KEY` în șablon.

## Rămâi informat {#staying-informed}

- [Gotify pe telefon](notifications.md#example-gotify): eșecurile sună, iar succesele sosesc în liniște.
- [ntfy ca webhook](notifications.md#example-ntfy): mesaje fără cont.

## Mai multe servere {#several-servers}

- [Împerecherea a două stații Unraid](offsite-recovery.md#worked-example-two-unraid-boxes-end-to-end): două servere care își păstrează reciproc copiile, fără ca vreunul să poată șterge backupurile celuilalt.
- [Un card și o alertă în Home Assistant](api.md#home-assistant-example): BombVault pe un dashboard, cu un mesaj când un backup eșuează.
