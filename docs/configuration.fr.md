# Configuration

Cette page couvre les variables d'environnement du conteneur, les montages fournis par le modèle, la sauvegarde de VM via SSH et la configuration hors site. Les **chemins de dépôt** de sauvegarde se configurent dans l'application (Paramètres, Chemins de sauvegarde), pas via des variables d'environnement.

## Variables d'environnement

| Variable | Requise | Description |
|---|---|---|
| `APP_KEY` | **Oui** | Secret hexadécimal de 32 octets (64 caractères hexa) utilisé pour dériver le mot de passe du dépôt restic. Générez avec `openssl rand -hex 32`. Gardez-le en lieu sûr : le perdre rend les sauvegardes chiffrées irrécupérables. |
| `LIBVIRT_HOST` | Pour les VMs | Hôte Unraid atteint via SSH pour la sauvegarde de VM (par défaut `host.docker.internal` ; le modèle pré-remplit un placeholder d'IP LAN). Utilisez l'IP LAN de votre Unraid, requis sur un réseau `br0.x` personnalisé. |
| `LIBVIRT_SSH_PORT` | Non | Port SSH de l'hôte pour la sauvegarde de VM (par défaut `22`). |
| `LIBVIRT_SSH_USER` | Non | Utilisateur SSH sur l'hôte pour la sauvegarde de VM (par défaut `root`). |
| `LIBVIRT_URI` | Non | URI de connexion libvirt complète, utilisée **telle quelle** au lieu d'en construire une à partir des trois variables `LIBVIRT_*` ci-dessus (qui sont alors ignorées pour la chaîne de connexion). Non définie par défaut. Nécessaire sur TrueNAS Scale, dont le libvirtd écoute sur un socket non standard que le format construit automatiquement ne peut pas exprimer : `qemu+ssh://<user>@<truenas-host>/system?socket=/run/truenas_libvirt/libvirt-sock`. Voir la section TrueNAS Scale de [docs/vm-backup-ssh-setup.md](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md). |
| `PORT` | Non | Port HTTP (par défaut `3000` ; utilisé uniquement avec `HTTP_ONLY=true`). |
| `HTTPS_PORT` | Non | Port HTTPS (par défaut `3443` ; le modèle le publie en 1:1, de sorte que l'interface web répond sur `https://<ip>:3443`). |
| `HTTP_ONLY` | Non | Mettez `true` pour désactiver l'écouteur HTTPS auto-signé et ne servir que du HTTP simple (pour un usage derrière un reverse proxy terminant le TLS). |
| `HOST_SOURCE_ROOT` | Non | Le chemin hôte monté en tant que **Host Data** (par défaut `/mnt`). BombVault traduit les sources de bind-mount rapportées par Docker en chemins sous ce montage. À changer uniquement si vous avez monté une racine hôte différente. |
| `DATA_ROOT_SEGMENTS` | Non | Noms de segments de chemin séparés par des virgules qui marquent une source de bind-mount comme donnée de sauvegarde (par défaut `appdata`, conforme à la convention `/mnt/user/appdata/<container>` d'Unraid). Le bind-mount d'un conteneur est automatiquement sélectionné pour la sauvegarde dès que N'IMPORTE LEQUEL des segments listés apparaît comme un segment de chemin complet de sa source hôte : par exemple, `DATA_ROOT_SEGMENTS=appdata,config` récupère aussi un bind `.../config`. Voir [Détection des sources de sauvegarde](#backup-source-detection) pour les autres méthodes, toujours actives, par lesquelles le dossier de données d'un conteneur est trouvé. |
| `PLATFORM` | Non | Force la plateforme sur laquelle BombVault se considère comme s'exécutant, au lieu de la détecter automatiquement : `unraid`, `generic` ou `truenas` (non définie par défaut : détecte automatiquement Unraid en sondant son marqueur `dockerMan` sous le montage flash, sinon `generic` ; une valeur non reconnue retombe elle aussi sur `generic`, journalisé). Définissez-la explicitement sur un hôte Docker générique ou sur TrueNAS Scale plutôt que de vous fier à la sonde automatique propre à Unraid : c'est ce que fait le fichier compose générique. Modifie la convention de repli appdata, les valeurs par défaut de destination de restauration entre instances, et si les étapes de notification/plugin compagnon propres à Unraid sont tentées ou non (voir `internal/platform`). |
| `BOMBVAULT_SELF_CONTAINER` | Non | Le nom du conteneur BombVault lui-même, afin qu'il ne se sauvegarde jamais (et donc ne s'arrête jamais) lui-même (par défaut `BombVault` ; détecté automatiquement via le nom d'hôte en réseau bridge). |
| `BACKUP_MAX_HOURS` | Non | Nombre maximal d'heures d'horloge qu'une exécution de sauvegarde unique peut détenir le verrou de son domaine avant d'être forcée à s'annuler (une protection pour qu'une exécution coincée ne puisse pas bloquer le domaine à jamais). Vide (la valeur par défaut) utilise `48`. Augmentez-le pour de très grandes ou lentes sauvegardes cloud (une exécution annulée au plafond échoue avec `context deadline exceeded`). Mettez `0` pour désactiver complètement le plafond. |
| `TZ` | Non | Fuseau horaire pour le planificateur (par exemple `Europe/Berlin`). **Si elle n'est pas définie, toutes les planifications s'exécutent en UTC** : une planification à 02:30 démarre alors à 02:30 UTC et non à l'heure locale. Sur Unraid, vous ne le définissez jamais vous-même : le système transmet son propre fuseau horaire à chaque conteneur. |

## Montages

Montez le socket Docker, la flash (`/boot`) et la racine **Host Data** (`/mnt`) comme indiqué dans le modèle CA. Les *sources* et les *destinations* de sauvegarde vivent toutes deux sous Host Data, et elle est montée en **rslave** afin qu'un partage distant qui se monte après le démarrage du conteneur (par exemple sous `/mnt/remotes`) devienne visible sans redémarrage.

Les chemins de dépôt de sauvegarde ont pour valeur par défaut `/mnt/user/bombvault/{container,vms,flash,config,files}`, créés à la première sauvegarde. Changez l'emplacement à tout moment dans **Paramètres, Chemins de sauvegarde**.

!!! note "Vérification de l'intégration hôte"
    Ouvrez `/spike` dans l'interface web après le démarrage du conteneur. Il sonde chaque montage et CLI (socket Docker, libvirt, restic, qemu-img, rclone) et signale toute pièce manquante.

## Modèle de sécurité

!!! warning "Contrôle de l'hôte équivalent à root"
    Via le socket Docker, BombVault peut arrêter, supprimer et recréer des conteneurs et lire/écrire dans appdata, et pour la sauvegarde de VM il se connecte à l'hôte via SSH (`qemu+ssh://`, root par défaut) pour exécuter `virsh`. Quiconque peut atteindre son interface web dispose de fait de root sur l'hôte.

- **Protection par mot de passe optionnelle** (Paramètres, Sécurité) : définissez un mot de passe pour exiger une connexion, effacez-le pour désactiver. Désactivée par défaut pour un usage sur LAN de confiance. Les sessions sont signées (HMAC dérivé de `APP_KEY`) et changer le mot de passe les invalide ; les connexions sont limitées en débit.
- Parce que la protection est optionnelle, lorsqu'elle n'est pas définie, toute l'interface et l'API (y compris la configuration hors site, les routes de test de sabotage et le kit de récupération) sont accessibles à quiconque peut atteindre le port. Activez la protection dès que des sauvegardes hors site, immuables ou du chiffrement sont utilisés.
- N'exécutez BombVault que sur un réseau de confiance et non exposé. Pour un accès distant, placez-le derrière un reverse proxy qui ajoute authentification et TLS. Les réponses portent des en-têtes de sécurité de base (CSP, `nosniff`, `X-Frame-Options`, `Referrer-Policy`).
- Avec `HTTP_ONLY=true`, le cookie de session perd son indicateur `Secure` (il le doit, pour fonctionner sur du HTTP simple), n'activez donc le mot de passe derrière un proxy terminant le TLS que si la confidentialité importe.
- La connexion SSH de sauvegarde de VM fait confiance à la clé d'hôte à la première connexion (TOFU) et l'épingle ensuite. Vérifiez la clé de l'hôte hors bande si votre chemin conteneur-vers-hôte n'est pas de confiance.
- Les sauvegardes sont chiffrées par restic lorsque le chiffrement est activé (Paramètres ; activé par défaut), avec la clé dérivée de `APP_KEY`.

## Sauvegarde de VM via SSH

BombVault sauvegarde les VMs KVM/libvirt **sans monter aucun chemin libvirt**. Il exécute `virsh` sur l'hôte via SSH (`qemu+ssh://`), de sorte qu'il ne peut jamais affecter le VM Manager de votre hôte.

Configuration rapide :

1. **Paramètres, Système, Sauvegarde de VM via SSH :** copiez la clé publique affichée.
2. Ajoutez-la à l'`/root/.ssh/authorized_keys` d'Unraid (également persistée sur la flash afin qu'elle survive aux redémarrages).
3. Cliquez sur **Tester la connexion**.

Le modèle ajoute `--add-host=host.docker.internal:host-gateway` afin que le conteneur puisse atteindre l'hôte. Définissez `LIBVIRT_HOST` sur l'IP LAN de votre Unraid si ce nom ne se résout pas (par exemple lorsque le conteneur s'exécute sur un réseau `br0.x` personnalisé). Si vous avez changé le port SSH d'Unraid, réglez `LIBVIRT_SSH_PORT` en conséquence. Les **instantanés à chaud** nécessitent en plus l'agent invité qemu dans la VM et le disque sur `/mnt/cache` (pas `/mnt/user`).

!!! important "Guide complet de configuration et de réseau des VMs"
    Le guide complet pas à pas (activation SSH, autorisation persistante de la clé, routage réseau personnalisé et VLAN, méthode par VM et dépannage côté hôte) se trouve sur [docs/vm-backup-ssh-setup.md](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) sur GitHub.

## Configuration hors site

Configurez un réplica hors site dans l'onglet **Paramètres, Hors site**. Voir [Sauvegarde hors site et récupération](offsite-recovery.md) pour le flux de travail complet (immuable/append-only, test de sabotage et essais de reprise après sinistre). En bref :

- **Backends :** SMB/CIFS et NFS (montez le partage et pointez-y un Chemin de sauvegarde), backends restic natifs sans rclone (`s3:...`, `rest:http://host:8000/repo`, `b2:...`, `sftp:user@host:/repo`), ou n'importe quel remote rclone (`rclone:<remote>:<bucket>/path`).
- **Les identifiants cloud** sont stockés chiffrés sous Paramètres, Hors site, Identifiants cloud.
- **Les cibles SSH ne nécessitent rien d'installé côté distant.** `sftp:` requiert seulement un serveur SSH. Ajoutez la clé publique de **Paramètres, Système, Sauvegarde de VM via SSH** (aussi disponible à `/config/ssh/id_ed25519.pub`) à l'`~/.ssh/authorized_keys` de l'utilisateur cible.
- **Copie hors site :** BombVault réplique les nouveaux instantanés avec `restic copy` au mieux. Le dépôt local reste principal. Chaque domaine a son propre planning hors site, plus un bouton **Répliquer maintenant**.
- **Plusieurs cibles hors site par domaine :** chaque domaine peut répliquer vers plusieurs destinations hors site à la fois. Ajoutez des cibles supplémentaires dans Paramètres, Hors site, chacune avec son propre dépôt, sa classe de stockage S3, son indicateur append-only, sa rétention et son budget de croissance ; elles répliquent toutes selon le planning hors site de ce domaine. Une configuration hors site unique existante est reprise comme première cible.
- **Rétention par source :** la politique locale vit dans Paramètres, Chemins et stockage ; la politique hors site dans Paramètres, Hors site (laissez-la entièrement à zéro pour ne jamais rogner automatiquement les instantanés hors site).
- **Limites de bande passante :** plafonnez le débit d'envoi/de téléchargement de restic sous Paramètres, Hors site.
- **Classe de stockage froid et archivage (S3) :** pour un dépôt hors site S3 natif, choisissez un niveau lisible à la restauration (Standard, Standard-IA, One Zone-IA, Intelligent-Tiering, Glacier Instant Retrieval). Les remotes rclone définissent leur classe dans la config rclone.

## Réglages portables (exporter et importer) {#portable-settings-export-and-import}

La carte **Exporter et importer les réglages** sur la page Paramètres écrit toute votre configuration BombVault (réglages de domaine, cibles hors site, plannings, rétention, notifications) dans un fichier JSON portable que vous pouvez importer sur une autre instance, de sorte que migrer vers une nouvelle machine ou cloner une configuration ne signifie pas tout ressaisir à la main. L'import affiche un aperçu et demande confirmation, et ne touche jamais à vos données ou votre historique de sauvegarde.

!!! warning "L'export peut contenir des identifiants"
    Vous choisissez d'inclure ou non les identifiants hors site et de notification dans le fichier. Avec les identifiants inclus, l'export est aussi sensible que votre kit de récupération, conservez-le donc en lieu sûr. Sans eux, le fichier ne contient que des réglages non secrets.
