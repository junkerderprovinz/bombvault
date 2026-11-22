# Nœuds

Un nœud est un BombVault sans interface web, par exemple sur un VPS ou sur un petit serveur chez un ami. Il sauvegarde les conteneurs et les dossiers de son propre serveur, et le BombVault sur lequel vous l'ajoutez, appelé **instance d'attache** sur cette page, lui dit quand sauvegarder et où envoyer les sauvegardes. Vous le gérez depuis l'interface web de l'instance d'attache, à côté de vos propres éléments.

![La carte du nœud vps-falkenstein sur la page Instances : Connectée, sa version, ses sauvegardes sur le serveur récepteur, quand chaque section sauvegarde, et Changer la destination, Remplacer et Libérer.](assets/screenshots/nodes/card-dark.png#only-dark)
![La carte du nœud vps-falkenstein sur la page Instances : Connectée, sa version, ses sauvegardes sur le serveur récepteur, quand chaque section sauvegarde, et Changer la destination, Remplacer et Libérer.](assets/screenshots/nodes/card-light.png#only-light)

## Comment ça marche {#how-it-works}

- **L'instance d'attache détient la clé du nœud.** Quand vous ajoutez un nœud, l'instance d'attache crée son `APP_KEY`, en garde une copie scellée et vous la remet une seule fois, dans le fichier compose du nœud. Avec cette copie, l'instance d'attache joint le nœud et ouvre ses sauvegardes.
- **Le nœud n'a besoin d'aucun port ouvert.** L'instance d'attache et le nœud se parlent par le relais, sur un lien privé qui leur est propre, à l'écart de votre groupe d'appairage. L'instance d'attache lance chaque appel et le signe, et le nœud ne lui envoie rien d'autre que son bonjour. La seule destination qui demande un port ouvert est [garder les sauvegardes sur le nœud](#keep-on-node).
- **Les sauvegardes ne passent jamais par le relais.** Le nœud les envoie directement à l'endroit que vous avez choisi : le serveur récepteur de l'instance d'attache, l'une de ses destinations ou un rest-server sur le nœud lui-même.
- **Le nœud ne planifie rien lui-même.** Il n'a pas de planning, n'envoie pas de notifications et ne rejoint aucun groupe ni aucune forteresse. L'instance d'attache lance ses sauvegardes, suit leur déroulement et envoie les alertes.
- **Un nœud sauvegarde des conteneurs et des ensembles de dossiers.** Les VMs, la clé flash, les jeux de données ZFS et l'auto-sauvegarde restent réservés aux installations complètes.

Le lien passe par le relais propre de l'instance d'attache quand son groupe en a un, et sinon par le relais du projet, même quand l'instance d'attache s'appaire sans relais.

## Avant de commencer {#before-you-start}

- **Un mot de passe de connexion sur l'instance d'attache.** Ajouter un nœud, afficher de nouveau son fichier compose et ouvrir ses éléments demandent une connexion avec le mot de passe, voir [Connexion](configuration.md#login).
- **Docker avec Compose sur le serveur du nœud.**
- **Un endroit pour les sauvegardes**, l'un des trois décrits sous [Où vont les sauvegardes](#placement). Le serveur récepteur est le choix le plus sûr, car le nœud ne peut rien y supprimer.

## Ajouter un nœud {#add}

1. Sur l'instance d'attache, ouvrez **Instances** et choisissez **Ajouter un nœud** en bas à droite.
2. **Nom.** Saisissez le nom sous lequel le nœud apparaît ici, par exemple celui du serveur, et choisissez **Suivant**.

    ![La fenêtre Ajouter un nœud à sa première étape, avec le nom vps-helsinki saisi.](assets/screenshots/nodes/add-name-dark.png#only-dark)
    ![La fenêtre Ajouter un nœud à sa première étape, avec le nom vps-helsinki saisi.](assets/screenshots/nodes/add-name-light.png#only-light)

3. **Sauvegardes.** Choisissez où vont les sauvegardes et remplissez ce que cet endroit demande. **Sauvegarde chaque jour à** est l'heure à laquelle les deux sections démarrent. L'instance d'attache propose une demi-heure après ses propres sauvegardes de conteneurs, pour que les deux ne sollicitent pas les mêmes disques en même temps. Vous pourrez plus tard donner à chaque section son propre planning sur la carte. Choisissez **Suivant**.

    ![La deuxième étape : le serveur récepteur choisi, Adresse pour les nœuds remplie avec une adresse https, et Sauvegarde chaque jour à 03:00.](assets/screenshots/nodes/add-placement-dark.png#only-dark)
    ![La deuxième étape : le serveur récepteur choisi, Adresse pour les nœuds remplie avec une adresse https, et Sauvegarde chaque jour à 03:00.](assets/screenshots/nodes/add-placement-light.png#only-light)

4. **Démarrage.** La fenêtre montre le fichier compose du nœud avec `MODE`, `APP_KEY`, `NODE_HOME` et `NODE_NAME` déjà remplis. Copiez-le et rangez l'`APP_KEY` en lieu sûr. Sur le serveur du nœud :

    ```sh
    mkdir -p /opt/bombvault-node && cd /opt/bombvault-node
    nano docker-compose.yml   # paste the file from the window
    docker compose up -d
    ```

    Avant de le démarrer, faites pointer le volume Host Data vers les données à sauvegarder et réglez `HOST_SOURCE_ROOT` sur le même chemin, comme le montre l'exemple ci-dessous.

5. La fenêtre attend le nœud. Dès que le nœud répond, l'instance d'attache lui dit où vont ses sauvegardes et vérifie qu'il les atteint, puis la fenêtre affiche **vps-helsinki est en ligne et reçoit ses ordres de cette instance.**

    ![La dernière étape : le fichier compose, puis le nœud en ligne et ses sauvegardes placées sur le serveur récepteur.](assets/screenshots/nodes/add-start-dark.png#only-dark)
    ![La dernière étape : le fichier compose, puis le nœud en ligne et ses sauvegardes placées sur le serveur récepteur.](assets/screenshots/nodes/add-start-light.png#only-light)

Vous pouvez fermer la fenêtre avant que le nœud réponde. Sa carte affiche alors **En attente du nœud**, et l'instance d'attache lui indique où vont ses sauvegardes dès qu'il se présente.

### Exemple : un VPS qui sauvegarde vers le serveur récepteur {#example-vps}

L'instance d'attache est un serveur Unraid appelé `tower` qui fait tourner le [serveur récepteur](offsite-recovery.md#receiving-server). Un reverse proxy le publie sous `https://backup.tower.example.org`. Le VPS `vps-helsinki` fait tourner Nextcloud et Caddy, avec leurs données sous `/srv/appdata`.

1. Sur `tower` : **Instances**, **Ajouter un nœud**, nom `vps-helsinki`, **Suivant**.
2. Choisissez **Le serveur récepteur de cette instance** et saisissez `https://backup.tower.example.org` sous **Adresse pour les nœuds**. Les conteneurs de l'instance d'attache se sauvegardent à 02:30, la fenêtre propose donc 03:00. Gardez cette heure et choisissez **Suivant**.
3. Sur le VPS, enregistrez le fichier de la fenêtre, avec les volumes et `HOST_SOURCE_ROOT` adaptés à `/srv/appdata` :

    ```yaml
    services:
      bombvault-node:
        image: junkerderprovinz/bombvault:latest
        restart: unless-stopped
        hostname: bombvault-node
        container_name: bombvault-node
        volumes:
          - ./config:/config
          - /var/run/docker.sock:/var/run/docker.sock
          - /srv/appdata:/host/user
        environment:
          MODE: node
          APP_KEY: "<filled in by home>"
          NODE_HOME: "<filled in by home>"
          NODE_NAME: "vps-helsinki"
          PLATFORM: generic
          HOST_SOURCE_ROOT: /srv/appdata
    ```

4. `docker compose up -d`. Quelques secondes plus tard, la fenêtre sur `tower` indique que le nœud est en ligne et **Les sauvegardes vont vers https://backup.tower.example.org.**
5. À 03:00, l'instance d'attache lance les sauvegardes du nœud. Le lendemain matin, la carte **Nœuds** de l'aperçu montre la dernière sauvegarde de `vps-helsinki`, et **Sauvegardes** liste ses conteneurs dans une section à part.

La façon dont BombVault trouve les données d'un conteneur sur un simple hôte Docker est décrite sous [Détection des sources de sauvegarde](configuration.md#backup-source-detection).

## Où vont les sauvegardes {#placement}

Chaque nœud a une seule destination pour ses deux sections. **Changer la destination** sur sa carte la modifie une fois le nœud en ligne. Le nouvel endroit reçoit de nouveaux dépôts, et les sauvegardes de l'ancien endroit restent où elles sont.

### Le serveur récepteur de cette instance {#to-receiver}

Le nœud écrit sur le serveur récepteur de l'instance d'attache en mode ajout seul : il peut ajouter des sauvegardes mais pas en supprimer. Il y reçoit un identifiant à lui, au nom du nœud, et un dépôt par section dans son propre dossier. L'instance d'attache ajoute sa propre clé à chaque dépôt et les élague elle-même, puisque le nœud ne le peut pas.

Si l'instance d'attache n'a pas encore de serveur récepteur, l'étape le dit et propose **Configurer le serveur récepteur**. L'instance d'attache n'a besoin d'aucun groupe pour cela. Choisissez un dossier sur un partage et un port, puis **Configurer et continuer**, et **Terminé** dès que la fenêtre montre le serveur : l'étape continue alors avec lui. Les détails sont sous [Serveur récepteur](offsite-recovery.md#receiving-server).

Le nœud joint le serveur à l'**Adresse pour les nœuds**, que vous réglez à la deuxième étape ou dans la fenêtre **Serveur récepteur**. Un nœud envoie le plus souvent depuis Internet, l'adresse doit donc être l'une de celles-ci :

- `https://`, par exemple derrière un reverse proxy, Pangolin ou un tunnel. Si le serveur présente son propre certificat au lieu d'un certificat d'une autorité publique, collez-le au format PEM sous **Certificat**.
- `http://` avec une adresse IP privée, comme `10.0.0.5` sur un lien WireGuard ou `100.101.102.103` sur Tailscale.

Un simple `http://` vers une adresse publique est refusé, car le nœud enverrait ses sauvegardes en clair à travers Internet. Quand vous changez l'adresse, l'instance d'attache prévient chaque nœud qui utilise le récepteur, et un nœud absent l'apprend dès son retour.

### Une destination {#to-location}

Le nœud écrit sur une destination de l'instance d'attache qu'il peut joindre lui-même : un service S3 ou un serveur REST, en `https://` ou à une adresse privée. Ses dépôts s'y trouvent dans `bombvault-node-<node id>-containers` et `bombvault-node-<node id>-files`.

Le nœud se connecte avec des [identifiants enregistrés](offsite-recovery.md#sign-ins) créés pour lui, qui doivent être différents de ceux avec lesquels l'instance d'attache écrit sur la destination. Ajoutez-en d'abord si la fenêtre indique qu'il n'y en a pas. Avec ces identifiants, le nœud peut en général supprimer ce qui se trouve sur la destination, contrairement au serveur récepteur, et la fenêtre le signale.

### Les garder sur le nœud {#keep-on-node}

Le nœud garde ses sauvegardes lui-même, derrière son propre rest-server en mode ajout seul, et l'instance d'attache rapatrie une copie de chaque section dans son propre stockage. Cela convient à un nœud auquel l'instance d'attache ne peut offrir aucune place. C'est la seule destination qui demande un port ouvert.

![La deuxième étape avec Les garder sur le nœud choisi, l'adresse https://203.0.113.7:8443 et la remarque sur le port ouvert.](assets/screenshots/nodes/add-keep-dark.png#only-dark)
![La deuxième étape avec Les garder sur le nœud choisi, l'adresse https://203.0.113.7:8443 et la remarque sur le port ouvert.](assets/screenshots/nodes/add-keep-light.png#only-light)

1. Sous **Adresse du nœud**, saisissez l'adresse à laquelle l'instance d'attache joint le nœud : son adresse publique et un port libre, comme `https://203.0.113.7:8443`, sans dossier derrière. N'utilisez `http://` qu'avec une adresse privée, comme ci-dessus.
2. Le nœud démarre le conteneur `bombvault-node-rest-server` sur ce port, avec ses dépôts dans le dossier `bombvault-node` sous Host Data. En `https://`, il crée un certificat pour cette adresse, et l'instance d'attache l'épingle.
3. Ouvrez le port dans le pare-feu du nœud, pour que l'instance d'attache puisse le joindre. Un port déjà pris par un autre conteneur sur le nœud est refusé, avec le nom de ce conteneur.

L'instance d'attache rapatrie une section juste après la fin des sauvegardes du nœud pour cette section, et en secours toutes les heures et quart, même avec **Rapatriement** désactivé. Ce que l'instance d'attache a rapatrié reste chez elle comme toute [copie rapatriée](offsite-recovery.md#receiver), même après le départ du nœud. Le nœud élague ses propres dépôts quand l'instance d'attache le lui demande, puisque son rest-server n'accepte aucune suppression, et ses ensembles de dossiers laissent de côté le dossier `bombvault-node`, pour qu'il ne sauvegarde jamais ses propres sauvegardes.

## Quand les nœuds sauvegardent {#schedule}

L'instance d'attache tient le planning de chaque nœud. La carte montre un planning par section, comme **Conteneurs** *Tous les jours à 2:00* et **Dossiers** *Hebdomadaire (dim.) à 4:00*. Un clic ouvre le même éditeur que sous [Plannings](schedules.md#plans), et une modification est enregistrée aussitôt.

![Le planning des conteneurs d'un nœud ouvert depuis sa carte, avec Désactivé, Quotidien, Hebdomadaire, Tous les N jours et Cron.](assets/screenshots/nodes/schedule-dark.png#only-dark)
![Le planning des conteneurs d'un nœud ouvert depuis sa carte, avec Désactivé, Quotidien, Hebdomadaire, Tous les N jours et Cron.](assets/screenshots/nodes/schedule-light.png#only-light)

- **Un nœud qui était absent** lance les sauvegardes manquées dès son retour. L'instance d'attache vérifie toutes les cinq minutes.
- **Un nœud occupé** est sollicité de nouveau cinq minutes plus tard.
- **Pendant que l'instance d'attache élague** les anciennes sauvegardes d'un nœud, le nœud ne lance rien avant qu'elle ait fini. Sur le serveur récepteur et sur le nœud lui-même, cette fenêtre s'ouvre chaque dimanche à 03:00 et dure jusqu'à six heures. Sur une destination, elle suit le planning de copie hors site de l'instance d'attache pour cette destination. L'instance d'attache conserve les sauvegardes selon sa propre règle pour la section sous **Durée de conservation**, ou selon la règle de la destination quand elle en a une.
- **Dans la liste des exécutions du nœud**, les sauvegardes et les restaurations lancées par l'instance d'attache apparaissent comme lancées par elle.

## Surveiller vos nœuds {#watch}

La carte **Nœuds** de l'aperçu liste chaque nœud avec son état, sa dernière sauvegarde, ses erreurs et ses anomalies ouvertes. Elle apparaît dès que vous avez un nœud.

![La carte Nœuds de l'aperçu avec trois nœuds : un connecté, un inactif avec deux erreurs, et un encore en attente du nœud.](assets/screenshots/nodes/overview-card-dark.png#only-dark)
![La carte Nœuds de l'aperçu avec trois nœuds : un connecté, un inactif avec deux erreurs, et un encore en attente du nœud.](assets/screenshots/nodes/overview-card-light.png#only-light)

L'instance d'attache envoie une alerte par épisode via ses [canaux de notification](notifications.md), et la suivante seulement une fois cet épisode terminé :

- **Une sauvegarde a échoué**, par exemple *BombVault: vps-helsinki: backup failed*, avec l'élément et la raison. Une sauvegarde ultérieure de cet élément qui réussit met fin à l'épisode.
- **Le nœud est injoignable**, par exemple *BombVault: vps-helsinki is out of reach*. L'instance d'attache attend le délai de l'interrupteur d'homme mort le plus court parmi les dépôts du nœud, 26 heures sauf si vous le changez, ou 6 heures tant que le nœud n'a pas encore de destination.

La surveillance du récepteur de l'instance d'attache s'occupe aussi de chaque dépôt d'un nœud, avec un interrupteur d'homme mort et un `restic check` quotidien à 04:00, voir [Récepteur et rapatrieur](offsite-recovery.md#receiver).

## Les éléments d'un nœud {#entries}

Sous **Sauvegardes**, les listes Conteneurs et Dossiers ont une section pour chaque nœud, sous vos propres éléments. Le conteneur du nœud lui-même n'y figure pas. **Sauvegarder maintenant** dans le menu d'un élément lance sa sauvegarde sur le nœud, et **Ouvrir** ouvre sa page.

Dans la section d'un nœud de la liste Dossiers, **Ajouter un jeu de dossiers** ajoute un jeu de dossiers sur le nœud, et **Modifier** dans le menu d'un jeu le change. Les deux ouvrent la même fenêtre que pour vos propres jeux. Le sélecteur de dossiers de cette fenêtre parcourt les dossiers du nœud, mais n'y crée pas de nouveaux dossiers.

![La section du nœud vps-falkenstein dans la liste Conteneurs : nextcloud, nextcloud-db, caddy et uptime-kuma.](assets/screenshots/nodes/backups-section-dark.png#only-dark)
![La section du nœud vps-falkenstein dans la liste Conteneurs : nextcloud, nextcloud-db, caddy et uptime-kuma.](assets/screenshots/nodes/backups-section-light.png#only-light)

La page d'un élément d'un nœud affiche **Sur** suivi du nom du nœud et liste ses sauvegardes. **Restaurer** sur l'une d'elles restaure l'élément sur le nœud et remplace ce qui s'y trouve. Un conteneur est arrêté pour cela, comme sur l'instance d'attache.

![La page de nextcloud sur vps-falkenstein avec ses sauvegardes et un bouton Restaurer à côté de chacune.](assets/screenshots/nodes/entry-dark.png#only-dark)
![La page de nextcloud sur vps-falkenstein avec ses sauvegardes et un bouton Restaurer à côté de chacune.](assets/screenshots/nodes/entry-light.png#only-light)

Par l'instance d'attache, vous atteignez une liste fixe de routes du nœud : ses conteneurs, ses ensembles de dossiers, ses exécutions et ses anomalies, ainsi que leur sauvegarde et leur restauration. Les réglages, les clés et le stockage du nœud restent hors de portée, et rien sur cette liste ne supprime quoi que ce soit. Voir [Les nœuds dans l'API](api.md#nodes).

## Remplacer et libérer {#replace-release}

**Remplacer** affiche de nouveau le fichier compose du nœud, après une connexion avec le mot de passe. Servez-vous-en pour déplacer le nœud sur un autre serveur ou pour le reconstruire : démarrez le fichier là-bas et éteignez l'ancien serveur. Le même `APP_KEY` et le même `NODE_HOME` en refont le même nœud, avec ses sauvegardes et son historique. L'instance d'attache conserve de chaque nœud ses ensembles de dossiers, ses conteneurs avec ce qu'il faut pour les recréer et ses exécutions. Quand le nœud démarre avec un dossier `config` vide, l'instance d'attache le reprend, lui indique où vont ses sauvegardes et lui rend tout cela avant la sauvegarde suivante, si bien que le nœud peut restaurer un conteneur qui n'existe plus. Sur un nouveau serveur, un nœud qui garde ses sauvegardes lui-même démarre avec des dépôts vides. Ses sauvegardes antérieures restent dans les copies que l'instance d'attache a rapatriées. Deux serveurs avec le même `APP_KEY` comptent comme un seul nœud, ne les faites donc jamais tourner tous les deux.

**Libérer** laisse partir un nœud. Il cesse de recevoir des ordres de l'instance d'attache et ne sauvegarde plus rien. Ses sauvegardes restent où elles sont, tout comme chaque copie que l'instance d'attache a rapatriée. Son identifiant sur le serveur récepteur disparaît, et l'instance d'attache cesse de surveiller ses dépôts. Un nœud absent au moment où vous le libérez garde sa carte avec **Libération en cours** jusqu'à ce qu'il l'apprenne. Un nœud libéré n'accepte plus d'instance d'attache avec le même `APP_KEY`. Pour réutiliser le serveur, ajoutez-le comme nouveau nœud et démarrez-le avec le nouveau fichier compose et un dossier `config` vide.

## Sécurité {#security}

Si l'instance d'attache est prise, l'attaquant contrôle aussi ses nœuds, et un nœud tient Docker sur son serveur, ce qui revient à root sur ce serveur. Protégez l'instance d'attache par un mot de passe et gardez-la à l'écart d'Internet. Si un nœud est pris, l'attaquant n'obtient ni groupe d'appairage, ni forteresse, ni autre nœud, ni autre dépôt, et ne peut pas effacer l'historique sur le serveur récepteur. Les détails se trouvent sous [Les nœuds dans le modèle de sécurité](configuration.md#nodes).

Le [kit de récupération](offsite-recovery.md#recovery-kit) contient un chapitre **Nodes** avec l'`APP_KEY` de chaque nœud, son mot de passe restic et ses dépôts, pour que vous puissiez faire revenir un nœud ou restaurer ses sauvegardes avec restic seul.
