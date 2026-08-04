# BombVault

**Vos données Unraid, scellées dans un coffre. Déposez une sauvegarde. Déclenchez une restauration.**

BombVault est une application web auto-hébergée, native Unraid, pour la **sauvegarde et la reprise complète après sinistre** de vos conteneurs Docker et de vos VMs KVM/libvirt. Elle s'exécute comme un unique conteneur Docker multi-arch, vous offre une interface web sombre et moderne, et gère tout le cycle de vie : sauvegarder, planifier, vérifier et restaurer.

Les restaurations sont automatiques. Les conteneurs réapparaissent dans l'onglet Docker d'Unraid exactement comme avant, et les VMs sont redéfinies dans le VM Manager avec leurs disques et leur NVRAM UEFI rattachés. Aucune réinstallation manuelle, aucune reconfiguration, aucun drame.

Propulsé par [restic](https://restic.net), chaque sauvegarde est donc dédupliquée, incrémentale et toujours chiffrée.

!!! note "Gardez votre APP_KEY en lieu sûr"
    BombVault dérive le mot de passe du dépôt restic d'un secret de 32 octets nommé `APP_KEY`. Le perdre rend les sauvegardes chiffrées irrécupérables. Générez-en un avec `openssl rand -hex 32` et conservez-le en lieu sûr. Voir [Configuration](configuration.md).

## Ce que protège BombVault

| Domaine | Ce qui est enregistré |
|---|---|
| **Conteneurs Docker** | Le répertoire appdata plus la définition du conteneur (image, variables d'environnement, ports, labels, volumes). |
| **VMs KVM / libvirt** | La ou les images disque de la VM, la définition XML et la NVRAM UEFI, sauvegardées via SSH (sans montage libvirt). |
| **Flash Unraid** | Toute la clé USB flash (`/boot`) : OS, licence, config de la matrice, partages, réseau et config des plugins. |
| **Configuration de l'application** | Le propre `/config` de BombVault : sa base de réglages, ses identifiants hors site et la paire de clés SSH libvirt. |
| **Fichiers et dossiers** | Des **jeux de fichiers** nommés, n'importe quel dossier du serveur, chacun avec des motifs d'exclusion optionnels par jeu. |

## La restauration est la vedette

Après avoir recopié les données depuis l'instantané restic, BombVault rejoue la définition de conteneur enregistrée contre l'API Docker, de sorte que le conteneur réapparaît dans l'onglet Docker d'Unraid comme s'il avait toujours été là (même image, mêmes réglages, mêmes mappages de ports). Les VMs voient leur XML redéfini via SSH et leurs disques et leur NVRAM UEFI rattachés, même après la suppression de la VM.

Lorsqu'une sauvegarde arrête des conteneurs dépendants, ils reviennent dans le bon ordre : BombVault les redémarre selon leur ordre `depends_on` de Compose et attend que chacun se signale sain avant de démarrer ceux qui en dépendent, afin que rien ne prenne les devants sur une base de données ou une passerelle qui n'est pas encore active. Voir [Fonctionnalités](features.md).

## Comment ça marche

```
Browser --HTTPS--> BombVault container
                   |- Go binary: JSON API + embedded React UI
                   |- Background worker (per-domain scheduler + job executor)
                   |
                   |- /var/run/docker.sock  -> Docker API (container stop/inspect/recreate)
                   |- qemu+ssh://host       -> libvirt / KVM on the HOST over SSH (no mount)
                   |- /mnt/ -> /host/user   -> appdata, VM disks + restic repos (read/write)
                   |- /boot/ -> /host/boot  -> Unraid flash backup (whole USB)
                   |- /config               -> BombVault's own settings + credentials (self-backup)
                   '- <repo path>           -> restic repository (local or remote: rclone/s3/rest/sftp)
```

BombVault est la couche d'orchestration et d'interface, pas le moteur de stockage. Tout le mouvement réel des données passe par restic.

## Démarrage rapide

Nouveau ici ? Rendez-vous sur **[Prise en main](getting-started.md)** pour installer BombVault sur Unraid via Community Applications et lancer votre première sauvegarde. Explorez ensuite l'ensemble des **[Fonctionnalités](features.md)**, ajustez votre **[Configuration](configuration.md)**, et mettez en place la **[Sauvegarde hors site et récupération](offsite-recovery.md)**.

Le hors site peut se répartir sur plusieurs cibles par domaine à la fois, un **tableau de bord récepteur** en lecture seule surveille ces copies sur la machine qui les reçoit, et vous pouvez emporter toute votre configuration vers une nouvelle machine avec la carte **Exporter et importer les réglages**. Voir [Sauvegarde hors site et récupération](offsite-recovery.md) et [Configuration](configuration.md#portable-settings-export-and-import).

## Liens

- **Code source :** [github.com/junkerderprovinz/bombvault](https://github.com/junkerderprovinz/bombvault)
- **Fil de support Unraid :** [forums.unraid.net](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/)
- **Tickets :** [github.com/junkerderprovinz/bombvault/issues](https://github.com/junkerderprovinz/bombvault/issues)

!!! warning "Contrôle de l'hôte équivalent à root"
    Via le socket Docker, BombVault peut arrêter, supprimer et recréer des conteneurs et lire/écrire dans appdata, et pour la sauvegarde de VM il se connecte à l'hôte via SSH pour exécuter `virsh`. Quiconque peut atteindre son interface web dispose de fait de root sur l'hôte. N'exécutez BombVault que sur un réseau de confiance et non exposé, et activez la protection par mot de passe optionnelle (Paramètres, Sécurité) dès que des sauvegardes hors site ou immuables sont utilisées. Voir [Configuration](configuration.md) pour le modèle de sécurité complet.
