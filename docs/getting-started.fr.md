# Prise en main

Cette page vous accompagne depuis une machine Unraid vierge jusqu'à votre première sauvegarde.

## Prérequis

| Prérequis | Notes |
|---|---|
| **Unraid 6.12+** | Les versions antérieures ne sont pas testées. Unraid est la cible principale, mais BombVault fonctionne aussi sur un simple hôte Docker et sur TrueNAS Scale (voir [Hôte Docker générique](#generic-docker-host) et [TrueNAS SCALE](#truenas)). |
| **Emplacement du dépôt restic** | Un chemin local (recommandé : votre matrice ou votre cache), SMB, NFS, ou n'importe quel backend rclone. |
| **Socket Docker** | Monté automatiquement par le modèle (`/var/run/docker.sock`). |
| **Flash Unraid** (`/boot`) | Montée entièrement par le modèle automatiquement (`/boot` vers `/host/boot`). Alimente la sauvegarde flash et permet à un conteneur restauré de réapparaître comme une application Unraid normale et modifiable. |
| **VMs KVM** (optionnel) | La sauvegarde de VM dialogue avec libvirt via SSH, sans montage libvirt. Configurez-la sous **Paramètres, Intégrations, SSH de l'hôte** (voir [Configuration](configuration.md)). |
| **Jeux de données ZFS** (optionnel) | La même liaison SSH que pour les sauvegardes de VM, `zfs` sur l'hôte, et Host Data mappé sur `/mnt` avec le mode d'accès Read/Write - Slave, la valeur par défaut du modèle. Voir [Jeux de données ZFS](zfs-datasets.md). |
| **Application Android** (facultative) | Android 10 ou ultérieur, appairée avec des serveurs en version 9.7.0 ou ultérieure. Voir [Application Android](android.md). |

## Installer sur Unraid

Le chemin le plus simple est **Community Applications**.

1. Ouvrez l'onglet **Apps** dans Unraid.
2. Cherchez **BombVault**.
3. Cliquez sur **Install**, définissez les variables requises (ci-dessous) et appliquez.

!!! tip "Installation manuelle du modèle"
    Si vous préférez ajouter le modèle à la main :

    1. Allez dans **Docker, Add Container, Template repositories** et ajoutez :
       ```
       https://github.com/junkerderprovinz/unraid-apps
       ```
    2. Cherchez **BombVault** dans Templates.
    3. Définissez les variables requises et cliquez sur **Apply**.

## Hôte Docker générique {#generic-docker-host}

Pas sur Unraid ? BombVault fonctionne aussi comme simple conteneur sur n'importe quel hôte Docker. TrueNAS SCALE a ses propres étapes sous [TrueNAS SCALE](#truenas).

1. Récupérez le fichier [`deploy/docker-compose.generic.yml`](https://github.com/junkerderprovinz/bombvault/blob/main/deploy/docker-compose.generic.yml), prêt à éditer, depuis le dépôt.
2. Définissez `APP_KEY` (voir plus bas) et pointez le volume Host Data vers votre véritable racine de données. Les commentaires du fichier détaillent les deux.
3. `docker compose up -d`, puis ouvrez `https://<ip-hôte>:3443/`.

Ce qui change par rapport à Unraid :

- **Pas de section Flash.** Il n'y a pas de clé de démarrage à capturer ou à restaurer, la section Flash n'a donc rien à faire ici. À la place, quand Host Data atteint le `/etc` de l'hôte, **Ajouter** sur la page Sauvegardes propose **Configuration du système** comme suggestion de dossiers : un jeu de dossiers `/etc` de départ, que vous relisez et modifiez avant d'enregistrer.
- **Pas de notifications natives Unraid.** Les canaux de notification propres à BombVault (webhook, alertes d'échec hors site, etc.) fonctionnent normalement ; seule la remontée spécifique au système de notification d'Unraid est omise, puisqu'un tel système n'existe pas ici.
- **La sauvegarde de VM est optionnelle et exige un hôte libvirtd distinct joignable en SSH.** Voyez le bloc commenté du fichier compose. Un hôte Docker générique n'embarque aucun gestionnaire de VM.
- **Pas de widget de tableau de bord.** Le BombVault Widget est un plugin Unraid, cette étape est donc omise elle aussi.
- **Trouver les données d'un conteneur.** Sans la convention `appdata` d'Unraid, le dossier de données d'un conteneur est trouvé à partir des segments de `DATA_ROOT_SEGMENTS`, des volumes nommés Docker, du répertoire de travail d'un projet Compose et du label `bombvault.data` (voir [Détection des sources de sauvegarde](configuration.md#backup-source-detection)). Les volumes nommés et le préréglage `/etc` n'atteignent que des chemins situés dans le montage Host Data : faites donc pointer Host Data vers un dossier parent commun qui couvre aussi la racine de données de Docker.
- **`PLATFORM`.** Définissez-la sur `generic` ou `truenas`. Si elle n'est pas définie, BombVault reconnaît Unraid à son propre marqueur sur le montage de la flash et traite tout le reste comme générique, et les étapes propres à Unraid sont omises au lieu d'être tentées puis d'échouer.

### TrueNAS SCALE {#truenas}

BombVault ne figure pas encore dans le catalogue d'applications de TrueNAS : installez-le donc comme application personnalisée via **Apps, Discover Apps**, puis **Install via YAML** dans le menu à côté de **Custom App**. Le YAML ci-dessous règle les mêmes choses que l'entrée de catalogue du dossier `truenas-apps` du dépôt. Remplacez `tank` par votre pool et renseignez `APP_KEY` comme l'explique la section suivante.

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
      # Pour les sauvegardes de VM :
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

Ouvrez ensuite `https://<truenas-ip>:30528/`. Ce qui figure sous [Ce qui change par rapport à Unraid](#generic-docker-host) vaut aussi ici, et sur TrueNAS :

- **Capacités.** BombVault a besoin des sept capacités ci-dessus. Sans `SETPCAP`, les sauvegardes de VM échouent, et sans `SYS_CHROOT`, chaque conteneur est arrêté pendant toute sa sauvegarde au lieu d'être lu depuis un instantané.
- **Host Data** est un chemin hôte sur un jeu de données, ici `/mnt/tank`, monté au même chemin dans l'application. C'est pourquoi `HOST_SOURCE_ROOT` et `HOST_MOUNT_ROOT` le nomment tous les deux. `rslave` transmet les montages que l'hôte fait plus tard, ce dont les sauvegardes de jeux de données ZFS ont besoin.
- **Les données d'un ixVolume ne sont pas sauvegardées.** Une application dont le stockage est un ixVolume garde ses données sous `/mnt/.ix-apps`, en dehors de Host Data. BombVault n'enregistre que la définition de l'application, et l'exécution nomme les dossiers qu'elle a laissés de côté. Pour chaque application dont vous voulez sauvegarder les données, choisissez un stockage **Host Path** sur un jeu de données situé sous Host Data.
- **Les sauvegardes de VM et de jeux de données ZFS** atteignent TrueNAS en SSH en tant que root. Activez le service SSH sous **System, Services** et ajoutez la clé publique de **Paramètres, Intégrations, SSH de l'hôte** à l'utilisateur root sous **Credentials, Users**. Les sauvegardes de VM ont aussi besoin de `LIBVIRT_URI` comme ci-dessus, car le libvirtd de TrueNAS écoute sur un socket qui lui est propre. Si ni `LIBVIRT_URI` ni `LIBVIRT_HOST` n'est défini, BombVault prend la passerelle par défaut de l'application comme hôte, puisque `host.docker.internal` ne se résout pas dans une application TrueNAS. Le [guide de sauvegarde de VM](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md#truenas-scale) donne les détails, VMs sur zvols comprises.
- **Un réplica ZFS utilise une autre clé.** Un réplica sur un second serveur ZFS se connecte avec la clé qu'affiche la boîte de dialogue du réplica, et non avec la clé SSH de l'hôte. Ajoutez celle-là sur le serveur qui reçoit.
- **Une application supprimée revient en deux étapes.** Réinstallez-la d'abord dans TrueNAS sous le même nom, ce qui recrée son réseau, puis restaurez ses données dans BombVault. D'ici là, le contrôle avant la restauration signale le réseau manquant, et la restauration s'arrête avant de modifier quoi que ce soit.
- **Une restauration À côté de l'original** crée un simple conteneur Docker. Il tourne, mais l'écran Apps de TrueNAS ne le liste pas.
- **Les dumps de bases de données** fonctionnent sur des montages de jeux de données. Un import dans un dossier de données qui est lui-même un jeu de données déplace l'ancien contenu dans un dossier voisin : le jeu de données au-dessus doit donc avoir la place pour le recevoir.

## L'unique réglage requis

La seule variable que vous devez définir est `APP_KEY`, un secret hexadécimal de 32 octets (64 caractères hexadécimaux) utilisé pour dériver le mot de passe du dépôt restic.

Générez-en un sur n'importe quelle machine :

```bash
openssl rand -hex 32
```

Collez le résultat dans le champ `APP_KEY` du modèle (Unraid), ou dans la variable d'environnement `APP_KEY` de `docker-compose.yml` (hôte Docker générique).

!!! danger "Ne perdez pas votre APP_KEY"
    Perdre `APP_KEY` rend vos sauvegardes chiffrées irrécupérables. Conservez-le en lieu sûr et à l'écart du serveur. Une fois BombVault en fonctionnement, définissez un mot de passe de connexion et téléchargez le **kit de récupération** sous **Paramètres, Restauration** (voir [Sauvegarde hors site et récupération](offsite-recovery.md#recovery-kit)). Il contient la clé, toutes les destinations et les commandes restic.

Le modèle monte aussi pour vous le socket Docker, la flash (`/boot`) et la racine **Host Data** (`/mnt`). Les *sources* et les *destinations* de sauvegarde vivent toutes deux sous Host Data. Pour la référence complète des variables et la configuration hors site, voir [Configuration](configuration.md).

## Première exécution

![L'aperçu après une première sauvegarde : ce qui vous attend, la protection de chaque section, les destinations et l'historique.](assets/screenshots/dashboard.png)

*L'aperçu après une première sauvegarde : ce qui vous attend, la protection de chaque section, les destinations et l'historique.*

1. Ouvrez l'interface web à `https://<votre-ip-unraid>:3443` (certificat auto-signé par défaut). Une nouvelle installation s'ouvre sur **Bienvenue dans BombVault**, qui énumère ce que BombVault a trouvé sur le serveur.
2. Choisissez **C'est parti**. L'aperçu affiche alors la carte **Premiers pas** avec trois étapes :
    1. **Choisir les sections** : activez ce que BombVault sauvegarde (Conteneurs, VMs, Flash, Dossiers, Jeux de données ZFS, Auto-sauvegarde). **Choisir** ouvre la liste des éléments d'une section, où **Suspendre le planning** en laisse un de côté. Les sections activées ici s'exécutent chaque nuit, ce que vous modifiez sous **Paramètres, Planning**.
    2. **Ajouter une destination** : où vont les sauvegardes. Un dossier local suffit pour commencer. Le plus sûr est une seconde destination hors des locaux, comme une Storage Box ou Backblaze B2. Les dépôts locaux ont pour valeur par défaut `/mnt/user/bombvault/{container,vms,flash,config,files,zfs}` et sont créés à la première sauvegarde.
    3. **Première sauvegarde** : ensuite, elle s'exécute toute seule chaque nuit.
3. Ouvrez **Sauvegardes** pour voir tous les éléments dans une seule liste. **Tout sauvegarder maintenant** lance une exécution sur toutes les sections activées, et **Sauvegarder maintenant** sur la page d'un élément ne sauvegarde que celui-ci.

Vous avez déjà utilisé BombVault ? **Restaurer depuis une sauvegarde** sur la page de bienvenue ouvre la page Restauration, où l'**Assistant d'urgence** ramène les réglages, les conteneurs, les VMs et les dossiers d'un serveur perdu. Voir [Sauvegarde hors site et récupération](offsite-recovery.md#emergency).

!!! tip "Optionnel : choisir un ordre de sauvegarde"
    Si certains conteneurs doivent toujours être sauvegardés avant d'autres (par exemple une base de données avant l'application qui l'utilise), ouvrez la carte **Ordre** sous **Paramètres, Planning** et faites-les glisser dans la séquence voulue. Les exécutions planifiées et la sauvegarde complète la suivent alors ; tout ce que vous laissez sans ordre est sauvegardé du plus en retard au moins en retard.

!!! note "Vérification d'intégration hôte"
    La carte **Intégration hôte** de l'aperçu sonde chaque montage et CLI (socket Docker, libvirt, restic, qemu-img, rclone) et signale toute pièce manquante, afin que vous puissiez confirmer que le conteneur est correctement branché avant de vous y fier. Elle attend sous **Personnaliser** que vous l'ajoutiez.

## S'y retrouver {#navigation}

![Les paramètres en tuiles, avec les cartes de la tuile choisie à côté. Chaque changement est écrit au moment où vous le faites.](assets/screenshots/settings.png)

*Les paramètres en tuiles, avec les cartes de la tuile choisie à côté. Chaque changement est écrit au moment où vous le faites.*

La barre latérale compte cinq entrées : **Aperçu**, **Anomalies** (tant que la détection d'anomalies est activée), **Sauvegardes**, **Restauration** et, sous **Se déconnecter**, **Paramètres**. L'action principale d'une page flotte en bas à droite, et **Plus** contient les rares. Sur un téléphone, la barre contient Aperçu, Sauvegardes, Restauration et Paramètres, et **Plus** a le reste.

Les pages et les paramètres occupent toute la largeur de la fenêtre. Une fenêtre garde ses boutons dans son pied, celui qui fait avancer à droite. Sur un téléphone, un pied de plus de trois boutons garde la sortie et l'action principale et met les autres sous **Plus**.

- **Sauvegardes** liste comme éléments chaque conteneur, chaque VM, chaque jeu de dossiers, la clé flash, chaque jeu de données ZFS et l'auto-sauvegarde. Des tuiles au-dessus de la liste choisissent un type, et **Filtres**, **Trier** et **Rechercher** l'affinent. Chaque élément a une page à lui avec trois onglets : **Sauvegardes**, **Paramètres** et **Vérification**. Un bouton de retour à gauche de l'icône de l'élément mène à la liste.
- **Restauration** est une seule page pour tout : choisissez une section, cherchez un élément et choisissez une sauvegarde. L'Assistant d'urgence et les autres sources commencent aussi ici.
- **Paramètres** sont quatorze tuiles : Général, Apparence, Planning, Destinations, Restauration, Notifications, Vérification, Sécurité, Instances, Appairage, Intégrations, Applications, Système et Info. Appuyez sur `/` ou Ctrl+K pour les chercher toutes, y compris par les noms que les pages portaient dans les versions précédentes. Faites glisser les tuiles dans l'ordre de votre choix sous **Apparence**. Là où une colonne de tuiles à côté de la page la comprimerait, comme dans une fenêtre d'environ 1200 pixels de large, les tuiles deviennent une bande au-dessus de la page qui défile latéralement. Un téléphone montre la même bande.
- **Destinations** et **Instances** sont des tuiles de paramètres avec des pages à elles. **Afficher aussi dans la barre latérale** sur leur tuile les épingle à la barre latérale.

Rien n'a de bouton Enregistrer et rien ne se déplie : les choses rares ont une carte à elles ou une fenêtre, et les explications se trouvent dans les bulles (i). Après une mise à jour, les notes de version s'ouvrent d'elles-mêmes, et des points verts marquent ce qui a changé. Un clic masque un point. **Nouveautés** dans la carte Version sous **Paramètres, Info** compte ce que vous n'avez pas encore vu et ouvre les notes de version et la liste de chaque changement avec **Y aller**, sous forme de deux onglets d'une même fenêtre.

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

La compilation de l'interface est nécessaire aussi pour `go run`. Le dépôt ne suit qu'un marqueur vide sous `web/dist` : sans `npm --prefix web run build`, le binaire n'intègre rien et répond `500 SPA index not found`, ce qui est normal. Docker, libvirt et Unraid ne peuvent pas être testés en CI : vérifiez donc les montages, restic et la liaison SSH des VM sur un vrai hôte avec la carte **Intégration hôte** de l'aperçu (ou `GET /api/spike`) avant d'ouvrir une pull request.

## Étapes suivantes

- Parcourez l'ensemble des **[Fonctionnalités](features.md)**.
- Mettez tous les serveurs de votre groupe sur votre téléphone avec l'**[application Android](android.md)**.
- Ajoutez une ou plusieurs destinations hors des locaux (chaque section peut copier vers plusieurs à la fois) et sauvegardez votre kit de récupération, comme décrit sous **[Sauvegarde hors site et récupération](offsite-recovery.md)**.
- Vous clonez une configuration ou migrez vers une nouvelle machine ? Emportez toute votre configuration avec la carte **Exporter / importer les paramètres**. Voir [Configuration](configuration.md#portable-settings-export-and-import).
- Un souci ? Voir **[Dépannage](troubleshooting.md)**.
