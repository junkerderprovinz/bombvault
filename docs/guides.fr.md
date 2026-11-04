# Exemples pas à pas

Chacun de ces exemples suit une vraie tâche du début à la fin, avec des noms et des chiffres renseignés. Ils se trouvent sur la page qui explique la fonction, les détails sont donc juste à côté.

## Premiers pas {#start}

- [Votre première sauvegarde de conteneur, de bout en bout](getting-started.md#example-first-backup) : de l'installation au kit de récupération.

## Stockage et hors site {#storage}

- [Une copie hors site sur Backblaze B2](offsite-recovery.md#example-s3) : un bucket, une clé pour ce bucket et la première copie. Cloudflare R2 fonctionne de la même façon.
- [Un rest-server que vous hébergez vous-même](offsite-recovery.md#example-rest-server) : une copie hors site qui refuse les suppressions, avec la ligne `docker run` et le test d'altération.
- [Ce que garde Équilibré](offsite-recovery.md#example-retention) : la rétention sur deux ans de sauvegardes nocturnes, sauvegarde par sauvegarde.
- [Les conteneurs déménagent vers un NAS](offsite-recovery.md#example-own-folder) : le dossier propre d'une section change d'endroit, et ce qu'il advient des sauvegardes déjà faites.

## Sauvegardes et plannings {#backups}

- [Trois plans qui vont ensemble](schedules.md#example) : une sauvegarde complète la nuit, les VMs une fois par semaine, et un planning propre pour un élément toutes les 6 heures.
- [Plex attend tant que quelqu'un regarde](backups.md#example-plex) : une sauvegarde planifiée qui n'interrompt pas un film.
- [Une VM avec deux instantanés Unraid](vm-backup-ssh-setup.md#example-unraid-snapshots) : une sauvegarde à chaud qui garde les instantanés, et une restauration à côté de l'original.
- [Une réplique vers un second serveur Unraid](zfs-datasets.md#example-replica) : une copie ZFS que vous pouvez lire à tout moment, et trois façons d'en tirer un fichier.

## Récupérer ses données {#restore}

- [Récupérer un seul fichier](restore.md#example-single-file) : un fichier de la sauvegarde de conteneur de la nuit dernière.
- [Un fichier d'une sauvegarde ZFS](zfs-datasets.md#example-zfs-file) : la même chose pour un jeu de données.
- [Un nouveau serveur à partir du kit de récupération](offsite-recovery.md#example-emergency) : un serveur mort revient sur une nouvelle machine, sans son ancienne `APP_KEY` dans le modèle.

## Rester informé {#staying-informed}

- [Gotify sur votre téléphone](notifications.md#example-gotify) : les échecs sonnent et les réussites arrivent discrètement.
- [ntfy comme webhook](notifications.md#example-ntfy) : des messages sans compte.

## Plusieurs serveurs {#several-servers}

- [Appairer deux serveurs Unraid](offsite-recovery.md#worked-example-two-unraid-boxes-end-to-end) : deux serveurs qui gardent les copies l'un de l'autre, sans que ni l'un ni l'autre puisse supprimer les sauvegardes de l'autre.
- [Une carte et une alerte dans Home Assistant](api.md#home-assistant-example) : BombVault sur un tableau de bord, avec un message quand une sauvegarde échoue.
