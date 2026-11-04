# Restauration

Chaque restauration commence sur la page **Restauration** : choisissez une section, cherchez ce que vous voulez récupérer et choisissez une sauvegarde. Une seule fenêtre restaure tous les types d'éléments et pose ses questions par étapes. Par défaut, rien ne change sur l'original, car la sauvegarde revient à côté de lui.

![La page Restauration avec les sections sous forme de tuiles, la recherche et la carte Stacks.](assets/screenshots/restore/page-dark.png#only-dark)
![La page Restauration avec les sections sous forme de tuiles, la recherche et la carte Stacks.](assets/screenshots/restore/page-light.png#only-light)

## La page Restauration {#page}

- **Sections** liste les sections sous forme de tuiles, chacune avec le nombre d'éléments qui ont des instantanés.
- **Que voulez-vous restaurer ?** cherche parmi les éléments de la section choisie. Chaque ligne montre son instantané le plus récent, et **Ouvrir l'élément** mène à la page de l'élément.
- **Stacks** liste les projets Compose avec au moins deux conteneurs sauvegardés, comme `immich` avec sa base de données et Redis. La recherche trouve une stack par son nom ou par n'importe lequel de ses conteneurs.
- **En cours** montre une restauration pendant qu'elle tourne, même après la fermeture de sa fenêtre.
- **Urgence** lance l'assistant d'urgence pour un serveur qui a disparu ou qui a été reconfiguré, voir [Assistant d'urgence](offsite-recovery.md#emergency).
- **Autres sources** contient **Depuis un ancien dossier de sauvegarde**, pour les sauvegardes d'un autre BombVault, et **Importer depuis le plugin Appdata.Backup**.

![La page Restauration sur un téléphone.](assets/screenshots/restore/page-phone-dark.png#only-dark)
![La page Restauration sur un téléphone.](assets/screenshots/restore/page-phone-light.png#only-light)

## La fenêtre de restauration {#window}

Un clic sur un élément ouvre **Restauration** avec son nom. Elle pose ses questions par étapes, et chaque étape peut être modifiée avant le départ :

1. **Sauvegarde** : l'instantané recommandé est présélectionné. C'est le plus récent, ou, après une anomalie ouverte, le dernier avant elle. Les plus anciens figurent dans la liste, et **Comparer** montre ce qui diffère entre deux d'entre eux.
2. **Source** : d'où viennent les fichiers, quand un instantané existe à plusieurs endroits. **Local** est le plus rapide.
3. **Quoi** : **Tous**, ou **Fichiers individuels**, où vous cochez les fichiers et dossiers voulus.
4. **Où** : la cible. Les choix dépendent du type d'élément :

    | Cible | Ce qu'elle fait |
    |---|---|
    | **À côté de l'original** | Le réglage par défaut. Un conteneur revient comme un second conteneur nommé `<name>-restored` avec ses propres ports, les disques et la NVRAM d'une VM atterrissent dans un dossier nommé `<folder>-restored`, et un ensemble de dossiers revient sous `<path>-restored`. Rien ne change sur l'original. |
    | **Remplacer l'original** | Écrase ce qui s'y trouve maintenant. Avec **Sauvegarder d'abord l'état actuel**, activé par défaut, BombVault sauvegarde d'abord l'état d'aujourd'hui pour que vous puissiez revenir en arrière. Si cette sauvegarde échoue, la restauration ne démarre pas. |
    | **Dans un dossier** | Seulement les fichiers, pour les parcourir. |
    | **Télécharger en ZIP** | La clé flash, pour une nouvelle clé. BombVault n'écrit jamais dans la clé flash en service. |
    | **Dans un nouveau jeu de données** et **Restaurer dans le jeu de données** | Jeux de données ZFS, voir [Jeux de données ZFS](zfs-datasets.md#restore). |

5. **Ce qui se passe** dit en mots ce que fera la restauration, et une scène montre la cible choisie.

Avant que **Restaurer** se déverrouille, la fenêtre vérifie que le dépôt répond, que la clé l'ouvre, que l'instantané est là et que la cible a la place voulue. Tant qu'un contrôle échoue, le bouton reste verrouillé, et le (i) qu'il contient dit lequel.

La fenêtre montre aussi ce que la restauration change par rapport à ce qui est là maintenant : fichiers nouveaux, remplacés et inchangés, avec la liste à la demande, et les fichiers de la cible qui ne sont pas dans la sauvegarde et restent où ils sont. restic établit cela à blanc d'après la taille et la date de modification, sans lire les fichiers, et une très grande arborescence s'arrête après 30 secondes et le dit. Pour les conteneurs et les VMs, elle compare les réglages que la restauration recrée à ceux en service : image et tag, ports, noms de variables et volumes, ou mémoire, vCPUs, disques et réseau.

Certaines choses peuvent arrêter une restauration avant qu'elle change quoi que ce soit. L'IP statique et les ports publiés d'un conteneur doivent être libres, et son réseau doit exister. Une restauration sur place nomme tous les autres conteneurs dont un montage bind atteint un dossier dans lequel elle écrit, par exemple « ce chemin est aussi utilisé par nextcloud-db », mais ce n'est qu'un avertissement.

Quelques options apparaissent là où elles s'appliquent : **Laisser arrêté ensuite**, pour examiner un conteneur ou une VM avant qu'il ne tourne de nouveau, et **Restaurer sans GPU ni runtime** sur un hôte qui n'a pas le GPU ou le runtime avec lequel la sauvegarde a été faite.

La restauration s'exécute en arrière-plan. Vous pouvez fermer la fenêtre, et la progression s'affiche aussi sous **En cours**. Une longue restauration affiche un pourcentage et peut être annulée, et une restauration annulée est enregistrée comme annulée, pas comme échouée.

![La fenêtre de restauration à l'étape Quoi, avec des fichiers isolés cochés.](assets/screenshots/restore/window-what-dark.png#only-dark)
![La fenêtre de restauration à l'étape Quoi, avec des fichiers isolés cochés.](assets/screenshots/restore/window-what-light.png#only-light)

### Où va une restauration dans un dossier {#folders}

**Dossiers de restauration** sous **Paramètres, Restauration** règle le **Dossier de restauration par défaut**, avec un **Sous-dossier par section** si vous le voulez, et une **Exception** pour une section. Les fichiers choisis atterrissent directement dans le dossier cible : un fichier revient seul, un dossier revient avec son contenu, et des choix dans des dossiers différents ne gardent que leur chemin sous le dossier qu'ils partagent. BombVault refuse une cible qui n'est pas sur un stockage monté, car sur Unraid un tel dossier vit en mémoire et disparaît au redémarrage, et il ne restaure jamais dans son propre dossier de données.

## Exemple : récupérer un seul fichier {#example-single-file}

Quelqu'un a modifié le `config.php` de Nextcloud cette nuit et Nextcloud ne démarre plus. La sauvegarde nocturne de 03:00 contient encore le bon fichier. Vous voulez ce fichier, pas tout le conteneur.

1. Ouvrez **Restauration**, choisissez la tuile **Conteneurs** et tapez `nextcloud` dans **Que voulez-vous restaurer ?**.
2. Cliquez sur `nextcloud`. La fenêtre s'ouvre avec la sauvegarde de 03:00 présélectionnée comme recommandée.
3. Laissez **Source** sur **Local**.
4. Sous **Quoi**, choisissez **Fichiers individuels**. Ouvrez les dossiers jusqu'à `config` et cochez `config.php`.
5. Sous **Où**, choisissez **Dans un dossier** et choisissez un dossier, par exemple `user/restore`, soit `/mnt/user/restore` sur l'hôte.
6. Lisez **Ce qui se passe** : le fichier atterrit dans le dossier cible, et rien ne change sur le conteneur. Appuyez sur **Restaurer**.

La fenêtre se termine par **Terminé : les fichiers sont dans /mnt/user/restore**. Vous trouvez le fichier sous `/mnt/user/restore/config.php`, sans les dossiers au-dessus. Comparez-le à celui qui est cassé, recopiez-le dans le dossier appdata et redémarrez Nextcloud.

Pour remettre le fichier directement à sa place, choisissez **Remplacer l'original** à l'étape 5. Gardez **Sauvegarder d'abord l'état actuel** activé, pour que BombVault sauvegarde l'état cassé avant d'écrire, tapez `nextcloud` pour confirmer et appuyez sur **Restaurer**. Nextcloud est arrêté pendant que le fichier choisi est réécrit à sa place, et seuls les fichiers que vous avez cochés sont écrits.

![La fenêtre de restauration à l'étape Ce qui se passe, avec Remplacer l'original choisi.](assets/screenshots/restore/window-replace-dark.png#only-dark)
![La fenêtre de restauration à l'étape Ce qui se passe, avec Remplacer l'original choisi.](assets/screenshots/restore/window-replace-light.png#only-light)

## Ce qui revient, selon le type {#kinds}

Les **conteneurs** sont recréés à partir de leur définition enregistrée via l'API Docker, avec leur entrypoint, leur nom d'hôte, leurs réglages d'arrêt, leurs volumes et montages, leurs limites de ressources, leur pilote de journaux, leurs réglages DNS et leurs liens hérités. Un conteneur revient sur l'image avec laquelle il tournait au moment de la sauvegarde, pas sur une plus récente tirée plus tard, et une image manquante est d'abord tirée, avec l'identifiant de **Registres de containers** sous **Paramètres, Restauration**. Son GPU ou son runtime (`--gpus`, `--runtime=nvidia`) revient aussi. Sur un hôte sans ce pilote ou ce runtime, la restauration le dit et propose **Restaurer sans GPU ni runtime**. Un lien vers un conteneur qui manque, ou qui est arrêté pendant que le conteneur restauré démarre, est omis, et l'historique des exécutions le dit. Une restauration qui échoue après la suppression du conteneur le remet en place, arrêté.

Les **VMs** voient leur XML défini de nouveau via SSH et leurs disques et leur NVRAM UEFI rattachés, même après la suppression de la VM. Voir [Sauvegardes de VM](vm-backup-ssh-setup.md) pour les VMs avec des instantanés Unraid et des disques sur des zvols.

Les **ensembles de dossiers** reviennent avec leurs propriétaires, leurs modes et leurs horodatages tels que restic les stocke.

La **clé flash** ne touche jamais le `/boot` en service. La restauration est un téléchargement appelé `flash-<id>.zip`, prêt à être déposé dans l'Unraid USB Creator. La fenêtre liste aussi les plugins de chaque sauvegarde de la clé flash avec leur version et leur taille, et peut en remettre un seul dans la clé flash en service : son fichier `.plg`, son dossier sous `config/plugins` et les fichiers de paquet que la sauvegarde contient. Unraid l'installe au prochain démarrage, ou tout de suite sous **Plugins, Install Plugin**.

Les **jeux de données ZFS** ont leurs propres cibles et gardent leurs propriétés ZFS, voir [Restaurer](zfs-datasets.md#restore).

L'**auto-sauvegarde** ne revient que par l'assistant d'urgence, qui redémarre BombVault, si bien que la base de données en service n'est jamais écrasée sous un descripteur ouvert.

Les **dumps de base de données** ne sont jamais rechargés tout seuls. Une ligne sous **Sauvegardes SQL de la base** peut être téléchargée, enregistrée dans un dossier ou importée dans une base fraîchement démarrée. Voir [Dumps de base de données](backups.md#database-dumps).

## Restaurer plusieurs éléments à la fois {#several}

Sur la page Restauration, choisissez des éléments comme vous choisissez des lignes dans la liste Sauvegardes : Ctrl+clic, Maj+clic, Ctrl+A, un appui long ou **Sélectionner**. **Restaurer** ouvre alors une fenêtre pour tous.

- **Quelle sauvegarde** : **Recommandée** prend pour chaque élément la sauvegarde que sa propre fenêtre recommanderait. **Un même instant pour tous** prend pour chaque élément sa sauvegarde la plus récente à un **Instant** que vous choisissez ou avant, par exemple la veille au soir d'un incident.
- **Où** : **À côté de l'original** ou **Remplacer l'original**, avec **Sauvegarder d'abord l'état actuel**.

Chaque élément est contrôlé comme pour une restauration isolée, puis ils s'exécutent l'un après l'autre. Chaque ligne montre comment la sienne s'est passée : **En attente**, **En cours**, **Restauré**, **Échec** ou **Laissé de côté**. **Arrêter après celui-ci** laisse la restauration en cours se terminer et n'en démarre plus. La clé flash arrive en ZIP à télécharger, et les jeux de données ZFS et l'auto-sauvegarde sont restaurés un par un.

![Restaurer plusieurs éléments : une ligne par élément avec sa progression.](assets/screenshots/restore/several-dark.png#only-dark)
![Restaurer plusieurs éléments : une ligne par élément avec sa progression.](assets/screenshots/restore/several-light.png#only-light)

## Restaurer une stack {#stacks}

Un clic sur une stack dans la carte **Stacks** ouvre **Restaurer la stack**. Elle restaure chaque membre depuis sa dernière sauvegarde en même temps, et avec **Démarrer dans l'ordre des dépendances après la restauration** elle les démarre dans leur ordre `depends_on` : bases de données et caches d'abord, puis l'application. Quand le dossier de projet d'un membre manque à l'endroit que vous avez choisi, la fenêtre propose l'état depuis Local à la place.

## Autres sources {#other-sources}

- **Depuis un ancien dossier de sauvegarde** ouvre le dépôt d'un autre BombVault qui ne tourne plus, sur un disque ou dans une destination hors site, avec son kit de récupération ou son `APP_KEY` saisie. La session est en lecture seule et ne vit qu'en mémoire. Voir [Depuis un ancien dossier de sauvegarde](offsite-recovery.md#restore-from-another-bombvault-repo).
- **Importer depuis le plugin Appdata.Backup** lit le dossier de sauvegarde du plugin. Chaque archive de conteneur devient un point de restauration de son conteneur, daté du moment où le plugin l'a faite. Les archives déjà importées sont ignorées, et les archives elles-mêmes ne sont que lues. Le conteneur a d'abord besoin d'une sauvegarde dans BombVault, pour que la restauration ait sa définition. La rétention laisse les points de restauration importés tranquilles, supprimez-en donc un vous-même quand vous n'en avez plus besoin.

## Après une restauration {#after}

Une copie restaurée à côté de l'original est listée sous **Sauvegardes** comme exclue, elle ne compte donc pas comme une tâche ouverte. Ajoutez-la au planning si vous voulez la garder. Un conteneur ou une VM qui tournait au moment de la sauvegarde revient en marche, un qui était arrêté reste arrêté. Sur Unraid, un conteneur restauré réapparaît dans l'onglet Docker comme une application normale et modifiable, et une VM dans le VM Manager avec ses disques et sa NVRAM rattachés.
