# BombVault

**Vos données Unraid, scellées dans un coffre. Déposez une sauvegarde. Déclenchez une restauration.**

![L'aperçu : l'en-tête d'état, ce qui vous attend, la protection de chaque section, les destinations et l'historique.](assets/screenshots/overview/home-dark.png#only-dark)
![L'aperçu : l'en-tête d'état, ce qui vous attend, la protection de chaque section, les destinations et l'historique.](assets/screenshots/overview/home-light.png#only-light)

BombVault est une application web auto-hébergée pour la **sauvegarde et la reprise complète après sinistre** sur Unraid et d'autres hôtes Docker, comme TrueNAS SCALE, ZimaOS ou un simple serveur Linux. Elle sauvegarde les conteneurs Docker, les VMs KVM/libvirt, les dossiers, les jeux de données ZFS et la clé flash d'Unraid. Elle s'exécute comme un unique conteneur Docker multi-arch, vous offre une interface web en thème sombre, clair ou système, et gère tout le cycle de vie : sauvegarder, planifier, vérifier et restaurer.

Les restaurations sont automatiques. Sur Unraid, les conteneurs réapparaissent dans l'onglet Docker exactement comme avant, et les VMs sont redéfinies dans le VM Manager avec leurs disques et leur NVRAM UEFI rattachés. Aucune réinstallation manuelle, aucune reconfiguration, aucun drame. Sur TrueNAS, un conteneur restauré à côté de l'original tourne comme un simple conteneur Docker que l'écran Apps ne liste pas.

Propulsé par [restic](https://restic.net), chaque sauvegarde est donc dédupliquée, incrémentale et chiffrée par défaut.

!!! note "Gardez votre APP_KEY en lieu sûr"
    BombVault dérive le mot de passe du dépôt restic d'un secret de 32 octets nommé `APP_KEY`. Le perdre, avec le kit de récupération, rend les sauvegardes chiffrées irrécupérables. Générez-en un avec `openssl rand -hex 32` et conservez-le en lieu sûr. Voir [Configuration](configuration.md).

## Ce que BombVault protège {#what-bombvault-protects}

Chaque conteneur, VM, ensemble de dossiers, la clé flash, chaque jeu de données ZFS et les réglages de BombVault lui-même sont un **élément** avec sa propre page. Les éléments d'un même type forment une **section**, que vous activez sous **Paramètres, Général**.

| Section | Ce qui est sauvegardé |
|---|---|
| **Conteneurs** | Le répertoire appdata plus la définition du conteneur (image, variables d'environnement, ports, étiquettes, volumes). |
| **VMs** | La ou les images disque de la VM, la définition XML et la NVRAM UEFI, sauvegardées via SSH (sans montage libvirt). |
| **Flash** | Toute la clé USB flash (`/boot`) : système, licence, configuration de la grappe, partages, réseau et configuration des plugins. |
| **Auto-sauvegarde** | Le `/config` de BombVault lui-même : sa base de réglages, les identifiants de ses destinations, la paire de clés SSH libvirt, la clé et les clés d'hôte épinglées des serveurs ZFS et les certificats. |
| **Dossiers** | N'importe quel dossier du serveur comme ensemble de dossiers, chacun avec des motifs d'exclusion facultatifs. |
| **Jeux de données ZFS** | Un jeu de données et tous ceux qu'il contient, lus à partir d'un seul instantané ZFS et stockés comme un dossier. Voir [Jeux de données ZFS](zfs-datasets.md). |

## La restauration est la vedette {#restore-is-the-star}

Après avoir recopié les données depuis l'instantané restic, BombVault rejoue la définition de conteneur enregistrée auprès de l'API Docker, si bien que le conteneur réapparaît dans l'onglet Docker d'Unraid comme s'il avait toujours été là (même image, mêmes réglages, mêmes ports). Les VMs voient leur XML redéfini via SSH et leurs disques et leur NVRAM UEFI rattachés, même après la suppression de la VM.

Une seule fenêtre restaure tous les types. Par défaut, elle choisit **À côté de l'original**, qui ramène la sauvegarde sous forme de copie et laisse l'original tel quel. **Remplacer l'original** sauvegarde d'abord l'état actuel, une restauration peut donc être annulée. Un dossier ou un nouveau jeu de données ZFS sont les autres cibles.

Quand une sauvegarde arrête des conteneurs dépendants, ils reviennent dans le bon ordre : BombVault les redémarre dans l'ordre `depends_on` de leur Compose et attend que chacun se déclare sain avant de démarrer ceux qui en dépendent, si bien que rien ne court devant une base de données ou une passerelle qui n'est pas encore prête. Voir [Restauration](restore.md) et [Commandes et autres conteneurs](backups.md#commands).

## Comment ça marche {#how-it-works}

```
Browser --HTTPS--> BombVault container
                   |- Go binary: JSON API + embedded React UI
                   |- Background worker (per-section scheduler + job executor)
                   |
                   |- /var/run/docker.sock  -> Docker API (container stop/inspect/recreate)
                   |- qemu+ssh://host       -> libvirt / KVM on the HOST over SSH (no mount)
                   |- /mnt/ -> /host/user   -> appdata, VM disks + restic repos (read/write)
                   |- /boot/ -> /host/boot  -> Unraid flash backup (whole USB)
                   |- /config               -> BombVault's own settings + sign-ins (self-backup)
                   '- <repo path>           -> restic repository (local or remote: rclone/s3/rest/sftp)
```

BombVault utilise le socket Docker pour arrêter les conteneurs avant une sauvegarde et les recréer après une restauration. Pour les VMs, il exécute `virsh` sur l'hôte via SSH (`qemu+ssh://`) pour éteindre proprement un domaine ou prendre un instantané à chaud. Il ne monte jamais de chemin libvirt en bind, il ne peut donc pas gêner le VM Manager de l'hôte.

BombVault est la couche d'orchestration et d'interface, pas le moteur de stockage. Tout déplacement réel de données passe par restic.

## Démarrage rapide {#quick-start}

Nouveau ici ? Rendez-vous sur **[Premiers pas](getting-started.md)** pour installer BombVault et lancer votre première sauvegarde, puis configurez **[Hors site et récupération](offsite-recovery.md)**. Les **[exemples pas à pas](guides.md)** parcourent les tâches courantes du début à la fin, la page **[Paramètres](settings.md)** liste chaque tuile, et **[Fonctionnalités](features.md)** est le court catalogue de tout ce que fait BombVault.

Chaque section peut copier vers plusieurs destinations à la fois, un BombVault appairé peut garder vos copies hors site sur un serveur qui refuse les suppressions, et l'**Assistant d'urgence** ramène un serveur perdu à partir de son kit de récupération, via votre groupe ou avec sa clé saisie. Pour reporter votre configuration sur un nouveau serveur, utilisez la carte **Exporter / importer les paramètres**. Voir [Hors site et récupération](offsite-recovery.md) et [Configuration](configuration.md#portable-settings-export-and-import).

L'**[application Android](android.md)** met tous les serveurs de votre groupe sur votre téléphone, avec le journal d'activité de tous sur un seul écran.

La documentation existe en 26 langues et l'interface en 42. Les pages qui n'ont pas encore de traduction s'affichent en anglais.

## Les mots employés ici {#glossary}

Quelques mots ont un sens fixe dans BombVault et dans ces pages.

| Mot | Sens |
|---|---|
| **Élément** | Une chose que BombVault sauvegarde : un conteneur, une VM, un ensemble de dossiers, la clé flash, un jeu de données ZFS avec ceux qu'il contient, ou ses propres réglages. Chacun a sa propre page. |
| **Section** | Tous les éléments d'un même type, comme Conteneurs. Vous activez les sections sous **Paramètres, Général**. |
| **Destination** | Un endroit où vont les sauvegardes : un dossier sur ce serveur, un partage, un serveur à vous ou un service cloud. Voir [Destinations](offsite-recovery.md#destinations). |
| **Local** | La destination sur ce serveur qui contient les dossiers des sections. |
| **Dossier propre** | Le dépôt dans lequel une section écrit d'abord ses sauvegardes. Il est par défaut sur ce serveur et peut se trouver dans une destination. Voir [Le dossier propre d'une section](offsite-recovery.md#own-folder). |
| **Copie** | Une sauvegarde copiée du dossier propre vers une autre destination avec `restic copy`. |
| **Dépôt direct** | Un dépôt à une destination hors site vers lequel les éléments sauvegardent directement, sans copie locale intermédiaire. Voir [Dépôts directs](offsite-recovery.md#direct-repositories). |
| **Identifiant** | La clé ou le mot de passe enregistré pour un service S3 ou un rest-server. L'un d'eux est celui par défaut. Voir [Identifiants](offsite-recovery.md#sign-ins). |
| **Destinations d'un élément** | Les destinations qui reçoivent les sauvegardes d'un élément, réglées sur l'élément ou comme valeur par défaut de sa section. Voir [Destinations par élément](offsite-recovery.md#placement). |
| **Réplique** | Une copie exacte au bloc près d'un élément ZFS sur un second serveur ZFS, faite avec `zfs send`. Elle se tient à côté de la sauvegarde et ne la remplace pas. Voir [Réplique](zfs-datasets.md#replica). |
| **Rôle** | Ce qu'une instance de votre groupe fait pour une autre : Récepteur, Rapatrieur, Serveur ZFS ou Gardien. Voir [Instances et rôles](offsite-recovery.md#roles). |

## Remerciements {#credits}

- **[VolumeVault](https://github.com/Darkdragon14/VolumeVault)** de [@Darkdragon14](https://github.com/Darkdragon14) (Apache-2.0) a donné à BombVault son idée de départ : la sauvegarde en un clic et la réinstallation automatique de conteneurs Docker. BombVault est une implémentation à part, en Go et sur restic, qui prolonge l'idée aux VMs, à la clé flash et plus encore.
- **[restic](https://restic.net/)** est le moteur de sauvegarde rapide, sûr et dédupliquant que BombVault pilote.
- **[rclone](https://rclone.org/)** fournit les moteurs cloud.
- La plupart des pictogrammes des boutons viennent du jeu gratuit Core Solid de **[Streamline](https://streamlinehq.com)** ([CC BY 4.0](https://creativecommons.org/licenses/by/4.0/), [source](https://github.com/webalys-hq/streamline-vectors)). Les autres viennent de Font Awesome Free, Material Design Icons, Simple Icons et Tabler Icons, ou ont été dessinés pour le projet.

## Licence {#license}

Copyright (C) 2026 Junker der Provinz. BombVault est un logiciel libre sous la **GNU Affero General Public License v3.0** ([LICENSE](https://github.com/junkerderprovinz/bombvault/blob/main/LICENSE)). Vous pouvez l'exécuter, l'étudier, le partager et le modifier. Si vous le distribuez, ou exécutez une version modifiée comme service réseau, vous devez publier vos sources sous la même licence et conserver les mentions de copyright et d'attribution existantes.

Le nom et l'identité visuelle ne sont pas sous licence. L'AGPL ne couvre que le code source : « BombVault », son logo et son identité visuelle restent réservés, un fork doit donc utiliser un nom et une identité à lui et ne peut pas se présenter comme BombVault.

## Liens {#links}

- **Code source :** [github.com/junkerderprovinz/bombvault](https://github.com/junkerderprovinz/bombvault)
- **Fil de support Unraid :** [forums.unraid.net](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/)
- **Tickets :** [github.com/junkerderprovinz/bombvault/issues](https://github.com/junkerderprovinz/bombvault/issues)

!!! warning "Contrôle équivalent à root sur l'hôte"
    Via le socket Docker, BombVault peut arrêter, supprimer et recréer des conteneurs et lire et écrire appdata, et pour la sauvegarde de VM il se connecte à l'hôte via SSH pour exécuter `virsh`. Toute personne qui peut atteindre son interface web a en pratique root sur l'hôte. N'exécutez BombVault que sur un réseau de confiance et non exposé, et définissez un mot de passe de connexion sous **Paramètres, Sécurité**. Le kit de récupération, les dépôts de clés, le ZIP flash et l'export d'une sauvegarde flash ou d'une auto-sauvegarde en demandent un. Voir [Configuration](configuration.md#security-model) pour le modèle de sécurité complet.
