# BombVault

**Vos données Unraid, scellées dans un coffre. Déposez une sauvegarde. Déclenchez une restauration.**

BombVault est une application web auto-hébergée, native Unraid, pour la **sauvegarde et la reprise complète après sinistre** de vos conteneurs Docker et de vos VMs KVM/libvirt. Elle s'exécute comme un unique conteneur Docker multi-arch, vous offre une interface web en thème sombre, clair ou système, et gère tout le cycle de vie : sauvegarder, planifier, vérifier et restaurer.

Les restaurations sont automatiques. Les conteneurs réapparaissent dans l'onglet Docker d'Unraid exactement comme avant, et les VMs sont redéfinies dans le VM Manager avec leurs disques et leur NVRAM UEFI rattachés. Aucune réinstallation manuelle, aucune reconfiguration, aucun drame.

Propulsé par [restic](https://restic.net), chaque sauvegarde est donc dédupliquée, incrémentale et toujours chiffrée.

!!! note "Gardez votre APP_KEY en lieu sûr"
    BombVault dérive le mot de passe du dépôt restic d'un secret de 32 octets nommé `APP_KEY`. Le perdre, avec le kit de récupération, rend les sauvegardes chiffrées irrécupérables. Générez-en un avec `openssl rand -hex 32` et conservez-le en lieu sûr. Voir [Configuration](configuration.md).

## Ce que protège BombVault

Chaque conteneur, chaque VM, chaque jeu de dossiers, la clé flash, chaque jeu de données ZFS et les réglages de BombVault eux-mêmes forment un **élément** doté de sa propre page. Les éléments d'un même type forment une **section**, que vous activez sous **Paramètres, Général**.

| Section | Ce qui est enregistré |
|---|---|
| **Conteneurs** | Le répertoire appdata plus la définition du conteneur (image, variables d'environnement, ports, labels, volumes). |
| **VMs** | La ou les images disque de la VM, la définition XML et la NVRAM UEFI, sauvegardées via SSH (sans montage libvirt). |
| **Flash** | Toute la clé USB flash (`/boot`) : OS, licence, config de la matrice, partages, réseau et config des plugins. |
| **Auto-sauvegarde** | Le propre `/config` de BombVault : sa base de réglages, les identifiants de ses destinations et la paire de clés SSH libvirt. |
| **Dossiers** | N'importe quel dossier du serveur comme jeu de dossiers, chacun avec des motifs d'exclusion optionnels. |
| **Jeux de données ZFS** | Un jeu de données avec tous ceux qui se trouvent en dessous, lu depuis un instantané ZFS et stocké comme un dossier. Voir [Jeux de données ZFS](zfs-datasets.md). |

## La restauration est la vedette

Après avoir recopié les données depuis l'instantané restic, BombVault rejoue la définition de conteneur enregistrée contre l'API Docker, de sorte que le conteneur réapparaît dans l'onglet Docker d'Unraid comme s'il avait toujours été là (même image, mêmes réglages, mêmes mappages de ports). Les VMs voient leur XML redéfini via SSH et leurs disques et leur NVRAM UEFI rattachés, même après la suppression de la VM.

Une seule fenêtre restaure tous les types. Par défaut, elle choisit **À côté de l'original**, qui ramène la sauvegarde sous forme de copie et laisse l'original tel quel. **Remplacer l'original** sauvegarde d'abord l'état actuel, de sorte qu'une restauration peut être annulée. Un dossier ou un nouveau jeu de données ZFS sont les autres cibles.

Lorsqu'une sauvegarde arrête des conteneurs dépendants, ils reviennent dans le bon ordre : BombVault les redémarre selon leur ordre `depends_on` de Compose et attend que chacun se signale sain avant de démarrer ceux qui en dépendent, afin que rien ne prenne les devants sur une base de données ou une passerelle qui n'est pas encore active. Voir [Fonctionnalités](features.md).

## Comment ça marche

```
Browser --HTTPS--> BombVault container
                   |- Go binary: JSON API + embedded React UI
                   |- Background worker (per-section scheduler + job executor)
                   |
                   |- /var/run/docker.sock  -> Docker API (container stop/inspect/recreate)
                   |- qemu+ssh://host       -> libvirt / KVM on the HOST over SSH (no mount)
                   |- /mnt/ -> /host/user   -> appdata, VM disks + restic repos (read/write)
                   |- /boot/ -> /host/boot  -> Unraid flash backup (whole USB)
                   |- /config               -> BombVault's own settings + credentials (self-backup)
                   '- <repo path>           -> restic repository (local or remote: rclone/s3/rest/sftp)
```

BombVault utilise le socket Docker pour arrêter les conteneurs avant une sauvegarde et les recréer après une restauration. Pour les VMs, il exécute `virsh` sur l'hôte via SSH (`qemu+ssh://`) afin d'arrêter proprement une VM ou d'en prendre un instantané à chaud. Il ne monte jamais de chemin libvirt, il ne peut donc pas gêner le VM Manager de l'hôte.

BombVault est la couche d'orchestration et d'interface, pas le moteur de stockage. Tout le mouvement réel des données passe par restic.

## Démarrage rapide

Nouveau ici ? Rendez-vous sur **[Prise en main](getting-started.md)** pour installer BombVault sur Unraid via Community Applications et lancer votre première sauvegarde. Explorez ensuite l'ensemble des **[Fonctionnalités](features.md)**, ajustez votre **[Configuration](configuration.md)**, et mettez en place la **[Sauvegarde hors site et récupération](offsite-recovery.md)**.

Chaque section peut copier vers plusieurs destinations à la fois, un BombVault appairé peut garder vos copies hors site sur un serveur qui refuse les suppressions, et l'**Assistant d'urgence** remet en route un serveur perdu à partir de son kit de récupération, via votre groupe ou avec sa clé saisie. Pour emporter votre configuration vers une nouvelle machine, utilisez la carte **Exporter / importer les paramètres**. Voir [Sauvegarde hors site et récupération](offsite-recovery.md) et [Configuration](configuration.md#portable-settings-export-and-import).

L'**[application Android](android.md)** met tous les serveurs de votre groupe sur votre téléphone, avec le journal d'activité de chacun d'eux sur un seul écran.

## Remerciements {#credits}

- **[VolumeVault](https://github.com/Darkdragon14/VolumeVault)** de [@Darkdragon14](https://github.com/Darkdragon14) (Apache-2.0) a donné à BombVault son idée de départ : la sauvegarde en un clic et la réinstallation automatique des conteneurs Docker. BombVault en est une implémentation distincte, en Go et avec restic, qui étend l'idée aux VMs, à la flash et au-delà.
- **[restic](https://restic.net/)** est le moteur de sauvegarde rapide, sûr et dédupliquant que pilote BombVault.
- **[rclone](https://rclone.org/)** fournit les backends cloud.
- La plupart des pictogrammes des boutons proviennent du jeu gratuit Core Solid de **[Streamline](https://streamlinehq.com)** ([CC BY 4.0](https://creativecommons.org/licenses/by/4.0/), [source](https://github.com/webalys-hq/streamline-vectors)). Les autres proviennent de Font Awesome Free, Material Design Icons, Simple Icons et Tabler Icons, ou ont été dessinés pour le projet.

## Licence {#license}

Copyright (C) 2026 Junker der Provinz. BombVault est un logiciel libre sous **GNU Affero General Public License v3.0** ([LICENSE](https://github.com/junkerderprovinz/bombvault/blob/main/LICENSE)). Vous pouvez l'exécuter, l'étudier, le partager et le modifier. Si vous le distribuez, ou si vous exécutez une version modifiée en tant que service réseau, vous devez publier votre code source sous la même licence et conserver les mentions de copyright et d'attribution existantes.

Le nom et l'identité visuelle ne sont pas couverts par la licence. L'AGPL ne couvre que le code source : « BombVault », son logo et son identité visuelle restent réservés, si bien qu'un fork doit utiliser un nom et une identité visuelle qui lui sont propres et n'a pas le droit de se présenter comme BombVault.

## Liens

- **Code source :** [github.com/junkerderprovinz/bombvault](https://github.com/junkerderprovinz/bombvault)
- **Fil de support Unraid :** [forums.unraid.net](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/)
- **Tickets :** [github.com/junkerderprovinz/bombvault/issues](https://github.com/junkerderprovinz/bombvault/issues)

!!! warning "Contrôle de l'hôte équivalent à root"
    Via le socket Docker, BombVault peut arrêter, supprimer et recréer des conteneurs et lire/écrire dans appdata, et pour la sauvegarde de VM il se connecte à l'hôte via SSH pour exécuter `virsh`. Quiconque peut atteindre son interface web dispose de fait de root sur l'hôte. N'exécutez BombVault que sur un réseau de confiance et non exposé, et définissez un mot de passe de connexion sous **Paramètres, Sécurité**. Le kit de récupération, les copies déposées et la suppression des sauvegardes d'une destination en ont besoin. Voir [Configuration](configuration.md#security-model) pour le modèle de sécurité complet.
