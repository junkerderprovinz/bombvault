# Premiers pas

Cette page vous mène d'un serveur Unraid tout neuf à votre première sauvegarde.

## Prérequis {#requirements}

| Prérequis | Remarques |
|---|---|
| **Unraid 6.12+** | Les versions plus anciennes ne sont pas testées. Unraid est la cible principale, mais BombVault tourne aussi sur un hôte Docker ordinaire et sur TrueNAS Scale (voir [Hôte Docker générique](#generic-docker-host) et [TrueNAS SCALE](#truenas)). |
| **Emplacement du dépôt restic** | Un chemin local (recommandé : votre grappe ou votre cache), SMB, NFS ou n'importe quel moteur rclone. |
| **Socket Docker** | Monté automatiquement par le modèle (`/var/run/docker.sock`). |
| **Flash Unraid** (`/boot`) | Monté en entier automatiquement par le modèle (`/boot` vers `/host/boot`). Il permet la sauvegarde de la clé flash et fait qu'un conteneur restauré réapparaît comme une application Unraid normale et modifiable. |
| **VMs KVM** (facultatif) | La sauvegarde de VM parle à libvirt via SSH, sans montage libvirt. Configurez-la sous **Paramètres, Intégrations, SSH de l'hôte** (voir [Configuration](configuration.md)). |
| **Jeux de données ZFS** (facultatif) | Le même lien SSH que les sauvegardes de VM, `zfs` sur l'hôte, et Host Data associé à `/mnt` avec le mode d'accès Read/Write - Slave, la valeur par défaut du modèle. Voir [Jeux de données ZFS](zfs-datasets.md). |
| **Application Android** (facultatif) | Android 10 ou ultérieur, appairée avec des serveurs en version 9.7.0 ou ultérieure. Voir [Application Android](android.md). |

## Installer sur Unraid {#install-on-unraid}

Le chemin le plus simple passe par **Community Applications**.

1. Ouvrez l'onglet **Apps** d'Unraid.
2. Cherchez **BombVault**.
3. Cliquez sur **Install**, réglez les variables obligatoires (voir plus bas) et appliquez.

!!! tip "Installation manuelle du modèle"
    Si vous préférez ajouter le modèle à la main :

    1. Allez dans **Docker, Add Container, Template repositories** et ajoutez :
       ```
       https://github.com/junkerderprovinz/unraid-apps
       ```
    2. Cherchez **BombVault** dans les modèles.
    3. Réglez les variables obligatoires et cliquez sur **Apply**.

## Hôte Docker générique {#generic-docker-host}

Pas sur Unraid ? BombVault tourne aussi comme simple conteneur sur n'importe quel hôte Docker. TrueNAS SCALE a ses propres étapes sous [TrueNAS SCALE](#truenas).

1. Récupérez dans le dépôt le fichier [`deploy/docker-compose.generic.yml`](https://github.com/junkerderprovinz/bombvault/blob/main/deploy/docker-compose.generic.yml), prêt à être modifié.
2. Réglez `APP_KEY` (voir plus bas) et faites pointer le volume Host Data vers la vraie racine de vos données. Les commentaires du fichier expliquent les deux.
3. `docker compose up -d`, puis ouvrez `https://<host-ip>:3443/`.

Ce qui diffère d'Unraid :

- **Pas de section Flash.** Il n'y a pas de clé USB de démarrage à capturer ou à restaurer, la section Flash n'a donc rien à faire ici. À la place, quand Host Data atteint le `/etc` de l'hôte, **Ajouter** sur la page Sauvegardes propose **Configuration du système** comme suggestion de dossier : un ensemble de dossiers `/etc` de départ que vous relisez et modifiez avant d'enregistrer.
- **Pas de notifications natives d'Unraid.** Les canaux de notification propres à BombVault (webhook, alertes d'échec hors site, etc.) fonctionnent normalement. Seul l'envoi propre à Unraid vers son système de notifications natif est ignoré, puisqu'il n'y a pas de tel système où envoyer.
- **La sauvegarde de VM est facultative et demande un hôte libvirtd distinct, joignable via SSH.** Voir le bloc commenté dans le fichier compose. Un hôte Docker générique n'a pas de gestionnaire de VM intégré.
- **Pas de tuile de tableau de bord.** TollRoll est un plugin Unraid, cette étape est donc ignorée aussi.
- **Trouver les données d'un conteneur.** Sans la convention `appdata` d'Unraid, le dossier de données d'un conteneur se trouve d'après les segments de `DATA_ROOT_SEGMENTS`, les volumes nommés Docker, le répertoire de travail d'un projet Compose et l'étiquette `bombvault.data` (voir [Détection des sources de sauvegarde](configuration.md#backup-source-detection)). Les volumes nommés et le préréglage `/etc` n'atteignent que des chemins à l'intérieur du montage Host Data, faites donc pointer Host Data vers un ancêtre commun qui couvre aussi la racine de données de Docker.
- **`PLATFORM`.** Réglez-le sur `generic` ou `truenas`. Non défini, BombVault détecte Unraid par son propre marqueur sur le montage flash et traite tout le reste comme générique, et les étapes propres à Unraid sont ignorées au lieu d'être tentées et d'échouer.

### TrueNAS SCALE {#truenas}

BombVault n'est pas encore dans le catalogue d'applications de TrueNAS, installez-le donc comme application personnalisée : **Apps, Discover Apps**, puis **Install via YAML** dans le menu à côté de **Custom App**. Le YAML ci-dessous règle les mêmes choses que l'entrée de catalogue du dossier `truenas-apps` du dépôt. Remplacez `tank` par votre pool et renseignez `APP_KEY` comme le décrit la section suivante.

```yaml
services:
  bombvault:
    image: junkerderprovinz/bombvault:latest
    hostname: bombvault
    restart: unless-stopped
    cap_drop: [ALL]
    cap_add: [CHOWN, DAC_OVERRIDE, FOWNER, SETGID, SETPCAP, SETUID, SYS_CHROOT]
    environment:
      APP_KEY: ""
      PLATFORM: truenas
      HOST_SOURCE_ROOT: /mnt/tank
      HOST_MOUNT_ROOT: /mnt/tank
      # For VM backups:
      # LIBVIRT_URI: qemu+ssh://root@<truenas-ip>/system?socket=/run/truenas_libvirt/libvirt-sock
    volumes:
      - /mnt/tank/apps/bombvault:/config
      - /var/run/docker.sock:/var/run/docker.sock
      - type: bind
        source: /mnt/tank
        target: /mnt/tank
        bind:
          propagation: rslave
    ports:
      - "30528:3443"
```

Ouvrez ensuite `https://<truenas-ip>:30528/`. Ce qui figure sous [Ce qui diffère d'Unraid](#generic-docker-host) vaut ici aussi, et sur TrueNAS :

- **Capacités.** BombVault a besoin des sept ci-dessus. Les sauvegardes de VM échouent sans `SETPCAP`, et sans `SYS_CHROOT` chaque conteneur est arrêté pendant toute sa sauvegarde au lieu d'être lu depuis un instantané.
- **Host Data** est un chemin d'hôte sur un dataset, ici `/mnt/tank`, monté au même chemin dans l'application. C'est pourquoi `HOST_SOURCE_ROOT` et `HOST_MOUNT_ROOT` le nomment tous deux. `rslave` transmet les montages que l'hôte fait plus tard, dont les sauvegardes de jeux de données ZFS ont besoin.
- **Les données dans un ixVolume ne sont pas sauvegardées.** Une application dont le stockage est un ixVolume garde ses données sous `/mnt/.ix-apps`, en dehors de Host Data. BombVault ne sauvegarde que la définition de l'application, et l'exécution nomme les dossiers qu'elle a laissés de côté. Pour chaque application dont vous voulez sauvegarder les données, choisissez le stockage **Host Path** sur un dataset sous Host Data.
- **Les sauvegardes de VM et de jeux de données ZFS** joignent TrueNAS via SSH en root. Activez le service SSH sous **System, Services**, et ajoutez la clé publique de **Paramètres, Intégrations, SSH de l'hôte** à l'utilisateur root sous **Credentials, Users**. Les sauvegardes de VM demandent aussi `LIBVIRT_URI` comme ci-dessus, car le libvirtd de TrueNAS écoute sur un socket à lui. Sans `LIBVIRT_URI` ni `LIBVIRT_HOST`, BombVault utilise la passerelle par défaut de l'application comme hôte, puisque `host.docker.internal` ne se résout pas dans une application TrueNAS. Le [guide des sauvegardes de VM](vm-backup-ssh-setup.md#truenas-scale) donne les détails, VMs sur zvols comprises.
- **Une réplique ZFS utilise une autre clé.** Une réplique sur un second serveur ZFS se connecte avec la clé que montre la boîte de dialogue de la réplique, pas avec la clé SSH de l'hôte. Ajoutez celle-là sur le serveur récepteur.
- **Une application supprimée revient en deux temps.** Réinstallez-la d'abord dans TrueNAS sous le même nom, ce qui recrée son réseau, puis restaurez ses données dans BombVault. Jusque-là, le contrôle de restauration signale le réseau manquant, et la restauration refuse avant de rien changer.
- **À côté de l'original** crée un simple conteneur Docker. Il fonctionne, mais l'écran Apps de TrueNAS ne le liste pas.
- **Les dumps de base de données** fonctionnent sur les montages de datasets. Un import dans un dossier de données qui est lui-même un dataset déplace l'ancien contenu dans un dossier à côté, le dataset du dessus a donc besoin de place pour lui.

### ZimaOS, CasaOS et autres hôtes {#other-hosts}

- **ZimaOS et CasaOS :** ajoutez le [store communautaire](https://github.com/junkerderprovinz/zimaos-apps#install) et installez-y BombVault. Il y tourne avec `PLATFORM=generic` et Host Data sur `/DATA`. ZimaOS garde les données des applications dans `/DATA/AppData/<app>`, que BombVault trouve tout seul, puisque `DATA_ROOT_SEGMENTS` reconnaît `appdata` quelle que soit la casse.
- **MOS :** installez BombVault depuis le [MOS Hub](https://mos-official.net/apps.html?search=BombVault). Il y tourne avec `PLATFORM=generic` et Host Data sur `/mnt/cache`.
- **Synology :** dans Container Manager, créez un projet à partir du fichier compose (DSM 7.2 ou plus récent).
- **Proxmox :** lancez-le dans une VM ou un conteneur LXC qui a Docker.

Sans les commentaires, le fichier compose générique est court. Remplacez `./data` par le dossier de l'hôte qui contient les données de vos conteneurs, réglez `HOST_SOURCE_ROOT` sur ce même chemin et renseignez `APP_KEY` :

```yaml
services:
  bombvault:
    image: junkerderprovinz/bombvault:latest
    restart: unless-stopped
    hostname: bombvault
    container_name: bombvault
    volumes:
      - ./config:/config
      - /var/run/docker.sock:/var/run/docker.sock
      - ./data:/host/user
    environment:
      APP_KEY: ""
      PLATFORM: generic
      HOST_SOURCE_ROOT: /host/user
    ports:
      - "3443:3443"
```

Pour sauvegarder aussi les volumes nommés Docker, montez un dossier qui contient à la fois vos données et la racine de données de Docker (`docker info -f '{{.DockerRootDir}}'`, en général `/var/lib/docker`), comme le modèle Unraid monte tout `/mnt`.

## Le seul réglage obligatoire {#the-one-required-setting}

La seule variable à régler obligatoirement est `APP_KEY`, un secret hexadécimal de 32 octets (64 caractères hexadécimaux) qui sert à dériver le mot de passe du dépôt restic.

Générez-en un sur n'importe quelle machine :

```bash
openssl rand -hex 32
```

Collez le résultat dans le champ `APP_KEY` du modèle (Unraid) ou dans la variable d'environnement `APP_KEY` de `docker-compose.yml` (hôte Docker générique).

!!! danger "Ne perdez pas votre APP_KEY"
    Perdre `APP_KEY` rend vos sauvegardes chiffrées irrécupérables. Conservez-la en lieu sûr, séparément du serveur. Une fois BombVault en marche, définissez un mot de passe de connexion et téléchargez le **kit de récupération** sous **Paramètres, Restauration** (voir [Hors site et récupération](offsite-recovery.md#recovery-kit)). Il contient la clé, toutes les destinations et les commandes restic.

Le modèle monte aussi pour vous le socket Docker, la flash (`/boot`) et la racine **Host Data** (`/mnt`). Les *sources* et les *destinations* de sauvegarde vivent toutes deux sous Host Data. Pour la référence complète des variables et la configuration hors site, voir [Configuration](configuration.md).

## Première utilisation {#first-run}

![La page d'accueil d'une nouvelle installation, avec ce que BombVault a trouvé sur le serveur.](assets/screenshots/getting-started/welcome-dark.png#only-dark)
![La page d'accueil d'une nouvelle installation, avec ce que BombVault a trouvé sur le serveur.](assets/screenshots/getting-started/welcome-light.png#only-light)

1. Ouvrez l'interface web à l'adresse `https://<your-unraid-ip>:3443` (certificat autosigné par défaut). Une nouvelle installation s'ouvre sur **Bienvenue dans BombVault**, qui liste ce que BombVault a trouvé sur le serveur.
2. Choisissez **C'est parti**. L'aperçu affiche alors la carte **Premiers pas** avec trois étapes :
    1. **Choisir les sections** : activez ce que BombVault sauvegarde (Conteneurs, VMs, Flash, Dossiers, Jeux de données ZFS, Auto-sauvegarde). **Choisir** ouvre la liste des éléments d'une section, où **Suspendre le planning** en laisse un de côté. Les sections que vous activez ici s'exécutent chaque nuit, ce que vous changez sous **Paramètres, Planning**.
    2. **Ajouter une destination** : un second endroit pour les sauvegardes. Chaque section a déjà un dossier sur ce serveur, `/mnt/user/bombvault/{container,vms,flash,config,files,zfs}`, créé à la première sauvegarde. Le plus sûr est une seconde destination hors des locaux, comme une Storage Box ou Backblaze B2.
    3. **Première sauvegarde** : ensuite elle s'exécute toute seule chaque nuit. La carte reste jusqu'à la fin de la première sauvegarde.
3. Ouvrez **Sauvegardes** pour voir tous les éléments dans une seule liste. **Tout sauvegarder maintenant** lance une exécution sur toutes les sections activées, et **Sauvegarder maintenant** sur la page d'un élément ne sauvegarde que celui-là.

![La carte Premiers pas avec ses trois étapes, la deuxième ouverte.](assets/screenshots/getting-started/first-steps-dark.png#only-dark)
![La carte Premiers pas avec ses trois étapes, la deuxième ouverte.](assets/screenshots/getting-started/first-steps-light.png#only-light)

Vous avez déjà utilisé BombVault ? **Restaurer depuis une sauvegarde** sur la page d'accueil ouvre la page Restauration, où l'**Assistant d'urgence** ramène les réglages, les conteneurs, les VMs et les dossiers d'un serveur perdu. Voir [Hors site et récupération](offsite-recovery.md#emergency).

!!! tip "Facultatif : choisir un ordre de sauvegarde"
    Si certains conteneurs doivent toujours être sauvegardés avant d'autres (par exemple une base de données avant l'application qui l'utilise), ouvrez la carte **Ordre** sous **Paramètres, Planning** et faites-les glisser dans la séquence voulue. Les exécutions planifiées et la sauvegarde complète la suivent alors. Tout ce que vous laissez sans ordre est sauvegardé du plus en retard au moins en retard.

## Exemple : votre première sauvegarde de conteneur, de bout en bout {#example-first-backup}

Cet exemple suit un serveur Unraid tout neuf avec quelques conteneurs, dont `plex`, jusqu'à ce que la première sauvegarde soit faite et que vous sachiez où la trouver. Comptez environ dix minutes, plus le temps de la première sauvegarde.

1. **Installer.** Dans l'onglet **Apps** d'Unraid, cherchez **BombVault** et appuyez sur **Install**.
2. **Régler la clé.** Sur un ordinateur, lancez `openssl rand -hex 32` et collez les 64 caractères dans le champ `APP_KEY` du modèle. Gardez-en tout de suite une copie dans votre gestionnaire de mots de passe, avant toute autre chose. Appuyez sur **Apply**.
3. **L'ouvrir.** Allez sur `https://<your-unraid-ip>:3443` et acceptez le certificat autosigné. **Bienvenue dans BombVault** liste ce qu'il a trouvé, par exemple 12 conteneurs, 2 VMs et la clé flash.
4. **C'est parti.** L'aperçu s'ouvre avec la carte **Premiers pas**.
5. **Choisir les sections.** **Conteneurs** est activé dès le départ. Activez ce que vous voulez en plus, par exemple **Flash**. **Choisir** ouvre la liste des conteneurs trouvés, où **Suspendre le planning** en laisse un de côté, par exemple un conteneur de test qui ne vous importe pas. Les sections activées s'exécutent chaque nuit dans la sauvegarde complète.
6. **Ajouter une destination.** Chaque section a déjà son propre dossier sur ce serveur, sous `/mnt/user/bombvault`, la première sauvegarde a donc où aller. Cette étape ajoute un second endroit, et le plus sûr est hors des locaux. Vous pouvez le faire maintenant avec l'[exemple S3](offsite-recovery.md#example-s3), ou y revenir une fois la première sauvegarde faite. La carte reste jusque-là.
7. **Première sauvegarde.** Ouvrez **Sauvegardes** et appuyez sur **Tout sauvegarder maintenant**, ou attendez la nuit. L'anneau de l'en-tête d'état devient un anneau de progression et compte les éléments, par exemple **Sauvegarde en cours : 3 sur 12**. Chaque conteneur est arrêté pendant la lecture de ses données et redémarré juste après. Un conteneur dont les données sont sur ZFS n'est arrêté que pendant les quelques secondes d'un instantané. La première sauvegarde lit tout, c'est donc la plus longue. Les suivantes ne stockent que ce qui a changé.
8. **La vérifier.** Quand l'anneau indique **Tout est sauvegardé**, ouvrez **Historique** sur l'aperçu : chaque conteneur a une ligne verte dans **Exécutions**. Ouvrez **Sauvegardes**, cliquez sur `plex` et regardez son onglet **Sauvegardes** : le premier instantané y est, avec **Local** comme endroit qui le contient. Quelques minutes plus tard, son onglet **Vérification** montre le contrôle de restauration, qui a restauré un échantillon de la sauvegarde et l'a relu.
9. **Protéger la clé.** Ouvrez **Paramètres, Sécurité** et définissez un mot de passe de connexion. Ouvrez ensuite **Paramètres, Restauration** et appuyez sur **Télécharger le kit de récupération** dans **En cas d'urgence**. Conservez le fichier hors du serveur, à côté de la copie d'`APP_KEY` de l'étape 2. Le rappel **Sauvegardez votre kit de récupération** sur l'aperçu disparaît quand vous appuyez sur **Je l'ai conservé en lieu sûr**.

Dès maintenant, les conteneurs sont sauvegardés chaque nuit. De bonnes prochaines étapes, dans cet ordre : une [destination hors site](offsite-recovery.md#example-s3), pour qu'un serveur mort n'emporte pas les sauvegardes avec lui, puis les [notifications](notifications.md), pour qu'un échec vous parvienne, puis un coup d'œil aux [Plannings](schedules.md) si 03:00 ne vous convient pas.

![L'en-tête d'état pendant la première sauvegarde, avec l'anneau de progression et ses étapes.](assets/screenshots/getting-started/status-ring-dark.png#only-dark)
![L'en-tête d'état pendant la première sauvegarde, avec l'anneau de progression et ses étapes.](assets/screenshots/getting-started/status-ring-light.png#only-light)

!!! note "Contrôle de l'intégration à l'hôte"
    La carte **Intégration hôte** de l'aperçu sonde chaque montage et chaque CLI (socket Docker, libvirt, restic, qemu-img, rclone) et signale les éléments manquants, pour que vous puissiez confirmer que le conteneur est bien raccordé avant de compter dessus. Elle attend sous **Personnaliser** jusqu'à ce que vous l'ajoutiez.

## S'y retrouver {#navigation}

![Les paramètres sous forme de tuiles, avec les cartes de la tuile choisie à côté. Chaque modification est enregistrée au moment où vous la faites.](assets/screenshots/getting-started/settings-rail-dark.png#only-dark)
![Les paramètres sous forme de tuiles, avec les cartes de la tuile choisie à côté. Chaque modification est enregistrée au moment où vous la faites.](assets/screenshots/getting-started/settings-rail-light.png#only-light)

La barre latérale a cinq entrées : **Aperçu**, **Anomalies** (tant que la détection d'anomalies est activée), **Sauvegardes**, **Restauration** et, sous **Se déconnecter**, **Paramètres**. L'action principale d'une page flotte en bas à droite, et **Plus** contient les plus rares. Sur un téléphone, la barre contient Aperçu, Sauvegardes, Restauration et Paramètres, et **Plus** a le reste.

Les pages et les paramètres utilisent toute la largeur de la fenêtre. Une fenêtre garde ses boutons dans son pied, avec celui qui avance à droite. Sur un téléphone, un pied de plus de trois boutons garde la sortie et l'action principale et range les autres sous **Plus**.

![Sur un téléphone, la barre du bas avec Plus ouvert.](assets/screenshots/getting-started/phone-more-dark.png#only-dark)
![Sur un téléphone, la barre du bas avec Plus ouvert.](assets/screenshots/getting-started/phone-more-light.png#only-light)

- **Aperçu** montre si tout est sauvegardé et ce qui vous attend. Voir [Aperçu](overview.md).
- **Sauvegardes** liste comme éléments chaque conteneur, VM, ensemble de dossiers, la clé flash, chaque jeu de données ZFS et l'auto-sauvegarde. Des tuiles au-dessus de la liste choisissent un type, et **Filtres**, **Trier** et **Rechercher** l'affinent. Chaque élément a une page à lui avec trois onglets : **Sauvegardes**, **Paramètres** et **Vérification**. Un bouton retour à gauche de l'icône de l'élément ramène à la liste. Voir [Sauvegardes et éléments](backups.md).
- **Restauration** est une seule page pour tout : choisissez une section, cherchez un élément et choisissez une sauvegarde. L'assistant d'urgence et les autres sources démarrent aussi ici. Voir [Restauration](restore.md).
- **Paramètres** compte quatorze tuiles : Général, Apparence, Planning, Destinations, Restauration, Notifications, Vérification, Sécurité, Instances, Appairage, Intégrations, Applications, Système et Info. Voir [Paramètres](settings.md). Appuyez sur `/` ou Ctrl+K pour les parcourir toutes, aussi par les noms que des pages portaient dans des versions antérieures. Faites glisser les tuiles dans votre propre ordre sous **Apparence**. Quand une colonne de tuiles à côté de la page l'écraserait, comme dans une fenêtre d'environ 1200 pixels de large, les tuiles deviennent une bande au-dessus de la page qui défile sur les côtés. Un téléphone montre la même bande.
- **Destinations** et **Instances** sont des tuiles de paramètres avec leurs propres pages. **Afficher aussi dans la barre latérale** sur leur tuile les épingle à la barre latérale.

Rien n'a de bouton Enregistrer et rien ne se déplie : les choses rares ont une carte à elles ou une fenêtre, et les explications se trouvent dans les bulles (i). Après une mise à jour, une fenêtre s'ouvre toute seule avec une carte pour chaque version depuis votre dernière, et des points verts marquent ce qui a changé. Un clic masque un point. **Nouveau** dans la carte Version sous **Paramètres, Info** compte ce que vous n'avez pas encore vu et rouvre cette fenêtre, avec les notes de chaque version et ses changements avec **Y aller**.

## Compiler depuis les sources {#build-from-source}

BombVault est un unique binaire Go statique qui sert une API JSON et une interface React intégrée. Compilez d'abord l'interface, puis lancez le binaire :

```bash
npm --prefix web ci
npm --prefix web run build     # writes web/dist, which the binary embeds
export APP_KEY=$(openssl rand -hex 32)
go test ./...                  # unit and integration tests, with a real restic round trip
golangci-lint run ./...
go run ./cmd/bombvault         # serves https://localhost:3443 with a self-signed certificate
```

La compilation de l'interface est nécessaire aussi pour `go run`. Le dépôt ne suit qu'un marqueur vide sous `web/dist`, donc sans `npm --prefix web run build` le binaire n'intègre rien et répond `500 SPA index not found`, ce qui est normal. Docker, libvirt et Unraid ne peuvent pas être testés en CI, vérifiez donc les montages, restic et le lien SSH des VMs sur un vrai hôte avec la carte **Intégration hôte** de l'aperçu (ou `GET /api/spike`) avant d'ouvrir une pull request.

## Prochaines étapes {#next-steps}

- Ajoutez une ou plusieurs destinations hors des locaux (chaque section peut copier vers plusieurs à la fois) et sauvegardez votre kit de récupération, comme décrit sous **[Hors site et récupération](offsite-recovery.md)**. L'[exemple S3](offsite-recovery.md#example-s3) en parcourt une.
- Assurez-vous qu'un échec vous parvienne avec les **[Notifications](notifications.md)**.
- Vérifiez quand les choses s'exécutent sous **[Plannings](schedules.md)**.
- Mettez tous les serveurs de votre groupe sur votre téléphone avec l'**[application Android](android.md)**.
- Les **[exemples pas à pas](guides.md)** parcourent les tâches courantes, et **[Fonctionnalités](features.md)** liste tout ce que fait BombVault.
- Vous clonez une installation ou vous passez à un nouveau serveur ? Reportez toute votre configuration avec la carte **Exporter / importer les paramètres**. Voir [Configuration](configuration.md#portable-settings-export-and-import).
- Un souci ? Voir **[Dépannage](troubleshooting.md)**.
