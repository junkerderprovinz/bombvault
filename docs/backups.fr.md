# Sauvegardes et éléments

Tout ce que BombVault sauvegarde figure dans une seule liste sous **Sauvegardes** : conteneurs, VMs, ensembles de dossiers, clé USB flash, jeux de données ZFS et auto-sauvegarde. Chacun est un **élément** avec sa propre page. Cette page parcourt la liste, la page d'un élément et ses trois onglets, ainsi que les réglages qu'un élément peut avoir.

![La liste Sauvegardes avec la tuile Conteneurs choisie : chaque ligne montre ses dernières exécutions sous forme de bande et un seul état.](assets/screenshots/backups/list-dark.png#only-dark)
![La liste Sauvegardes avec la tuile Conteneurs choisie : chaque ligne montre ses dernières exécutions sous forme de bande et un seul état.](assets/screenshots/backups/list-light.png#only-light)

## Ce que chaque section sauvegarde {#what-is-saved}

| Section | Ce qui est sauvegardé |
|---|---|
| **Conteneurs** | Le dossier appdata plus la définition du conteneur : image, variables, ports, étiquettes et volumes. La carte **Dossiers à sauvegarder** de l'onglet **Paramètres** du conteneur coche exactement quels dossiers la sauvegarde couvre, avec un décompte en direct des chemins et la liste de ce que vous avez laissé de côté. Les données qu'un conteneur garde en dehors de Host Data, par exemple dans un ixVolume TrueNAS ou un projet Compose sous `/opt`, ne sont pas sauvegardées, et l'exécution nomme les dossiers qu'elle a laissés de côté. L'exécution nomme aussi un dossier de Host Data qui n'existe pas en ce moment. Les chemins système comme `/etc/localtime` ou le socket Docker ne comptent pas comme des données. |
| **VMs** | Les images disque, la définition XML et la NVRAM UEFI, avec un arrêt propre ou un instantané à chaud, via SSH. Voir [Sauvegardes de VM](vm-backup-ssh-setup.md). |
| **Flash** | Unraid seulement. Toute la clé USB flash (`/boot`) : système, licence, configuration de la grappe, partages, réseau et configuration des plugins. |
| **Auto-sauvegarde** | Le `/config` de BombVault lui-même : sa base de réglages, les identifiants des destinations, la paire de clés SSH libvirt, la clé et les clés d'hôte épinglées des serveurs ZFS et les certificats, capturés avec `VACUUM INTO` de SQLite pour que la base ne soit jamais saisie au milieu d'une écriture. |
| **Dossiers** | N'importe quel dossier du serveur comme ensemble de dossiers, par exemple un partage, vos documents ou une photothèque, avec des motifs d'exclusion facultatifs. Les ensembles de dossiers ont des plannings, une rétention, des copies hors site, des vérifications et des exercices de restauration comme tout le reste. |
| **Jeux de données ZFS** | Un jeu de données et tous ceux qu'il contient, lus à partir d'un seul instantané ZFS pour que tous viennent du même instant, et stockés avec restic comme un dossier. Un élément ZFS peut aussi garder une réplique exacte au bloc près sur un second serveur ZFS. Voir [Jeux de données ZFS](zfs-datasets.md). |

Chaque sauvegarde passe par restic, elle est donc incrémentielle et dédupliquée, et même les gros disques de VM ne font pas gonfler le dépôt. BombVault ne sauvegarde jamais son propre conteneur, et ne l'arrête donc jamais. Une exécution peut être arrêtée sans risque, car restic écrit son instantané en dernier : une exécution interrompue laisse des données non référencées et aucun instantané. Un verrou laissé par un redémarrage au milieu d'une exécution est levé et l'exécution retentée une fois, mais seulement après que le verrou est resté dix minutes sans être rafraîchi.

## La liste {#list}

Les tuiles au-dessus de la liste choisissent un type, comme **Conteneurs** ou **ZFS**, et chaque tuile compte ses éléments. Une tuile sélectionne, elle ne masque pas le reste de la page.

Chaque ligne montre les quatorze dernières exécutions de l'élément sous forme de petits carrés et un seul état, comme **Protégé**, **Non protégé**, **Reporté** ou **Réplication en cours**. Tout le reste figure dans la bulle d'info à côté de son nom, et tant que le pointeur reste sur la ligne, ce texte s'affiche dans la ligne elle-même. Une marque à côté du nom d'un conteneur indique qu'il a été recréé avec une autre image, d'autres ports, variables ou volumes depuis sa dernière sauvegarde. Elle disparaît à la sauvegarde suivante.

Trois boutons au-dessus de la liste permettent de l'affiner :

- **Filtres** : par **Planning** (**Tous**, **Planifié**, **En pause**), par **Sauvegarde** (**Sauvegardé**, **Jamais sauvegardé**) et, pour **Conteneurs et VMs**, **Installés** ou **Non installé**.
- **Trier** : par nom, état, IP, dernière sauvegarde, taille ou planning. Chaque champ sauf IP se trie dans les deux sens, comme **Taille, la plus grande d'abord** et **Taille, la plus petite d'abord**.
- **Rechercher** : par nom, image ou IP.

Les éléments qui ne sont plus installés mais ont encore des sauvegardes sont à part, sous **Non installé (sauvegardes uniquement)**. Restaurez-les, ou supprimez leurs sauvegardes pour libérer la place.

L'action principale de la page flotte en bas à droite : **Tout sauvegarder maintenant** lance une exécution sur toutes les sections activées. **Plus** contient le reste :

- **Sélectionner** démarre une sélection, voir plus bas.
- **Découvrir les sauvegardes** lit ce que contiennent les destinations et reconstruit des éléments à partir de là, par exemple après une installation neuve. Un conteneur sans données propres n'a pas d'instantané, et il revient à partir de la définition recopiée à côté du dépôt. Les copies hors site des sections reconstruites attendent alors que vous adoptiez leurs sauvegardes pour ce serveur, voir [Après une reconstruction](offsite-recovery.md#after-a-rebuild).
- **Connexion ZFS à ce serveur** ouvre le test SSH de la section ZFS, voir [Jeux de données ZFS](zfs-datasets.md#requirements).
- **Tout répliquer maintenant**, avec la tuile ZFS choisie, envoie la réplique de chaque élément ZFS.

![La même liste sur un téléphone.](assets/screenshots/backups/list-phone-dark.png#only-dark)
![La même liste sur un téléphone.](assets/screenshots/backups/list-phone-light.png#only-light)

## Choisir plusieurs éléments {#select}

Choisissez des lignes comme vous choisissez des fichiers dans un gestionnaire de fichiers : Ctrl+clic, Maj+clic, Ctrl+A, ou un appui long sur un téléphone. Une ligne choisie affiche une coche à la place de son icône, et une barre au-dessus de la liste les compte et propose **Sauvegarder la sélection**, **Suspendre le planning** et **Reprendre le planning**, **Tout sélectionner** et **Effacer la sélection**. Chaque ligne a aussi un menu contextuel avec ses propres actions.

Une sélection est sauvegardée sur le serveur, un élément après l'autre, elle continue donc quand vous fermez l'onglet. Pour restaurer plusieurs éléments ensemble, utilisez la [page Restauration](restore.md#several).

![Deux conteneurs choisis, avec la barre au-dessus de la liste.](assets/screenshots/backups/bulk-dark.png#only-dark)
![Deux conteneurs choisis, avec la barre au-dessus de la liste.](assets/screenshots/backups/bulk-light.png#only-light)

## Ajouter un dossier ou un jeu de données ZFS {#add}

Les conteneurs, les VMs et la clé flash apparaissent d'eux-mêmes dès que leur section est activée sous **Paramètres, Général**. Les dossiers et les jeux de données ZFS, vous les choisissez vous-même, car BombVault ne peut pas deviner lesquels vous voulez. **Ajouter** ouvre **Que doit sauvegarder BombVault en plus ?** :

- **Un dossier** liste les partages que BombVault a trouvés et accepte n'importe quel autre dossier sous Host Data. Donnez un nom à l'ensemble de dossiers, par exemple `Documents`. Le nom est figé après la première sauvegarde, car les instantanés sont stockés sous ce nom.
- **Un jeu de données ZFS** liste les jeux de données de ce serveur. Choisissez celui qui est tout en haut de ce que vous voulez sauvegarder, voir [Éléments et jeux de données enfants](zfs-datasets.md#items-and-children).
- **Configuration du système** apparaît sur un hôte sans Unraid dont Host Data atteint `/etc`. C'est un ensemble de dossiers `/etc` de départ que vous relisez avant d'enregistrer.

## Laisser un élément de côté {#dont-back-up}

Un élément que vous laissez de côté exprès, comme un conteneur de test, peut être marqué **Ne pas sauvegarder**. Il cesse alors de compter comme non protégé, et **Ne plus exclure** le remet en jeu. **Suspendre le planning** est la manière plus douce : l'élément quitte le planning et la sauvegarde complète, porte un badge gris **Planning suspendu**, et **Sauvegarder maintenant** fonctionne toujours. Cela règle le même interrupteur que **Au planning** sur l'onglet Paramètres de l'élément.

## La page d'un élément {#entry-page}

Un clic sur une ligne ouvre la page de l'élément. L'en-tête nomme l'élément et son état en une ligne, comme **Dernière sauvegarde il y a 3 heures** ou **Reportée, prochain essai à 03:15**, et contient ses actions : **Sauvegarder maintenant**, **Restaurer**, et sous **Plus** les plus rares, comme **Exporter la dernière sauvegarde**, **Exporter le conteneur** ou **Exporter la VM**, **Lier à un ancien élément**, **Supprimer toutes les sauvegardes** et **Supprimer l'élément**. Un bouton retour à gauche de l'icône ramène à la liste. Pendant qu'une sauvegarde tourne, **Annuler la sauvegarde** se tient à côté de sa progression, et une exécution annulée est enregistrée comme annulée, pas comme échouée.

La page a trois onglets : **Sauvegardes**, **Paramètres** et **Vérification**.

### Onglet Sauvegardes {#backups-tab}

Cet onglet liste les instantanés de l'élément, les plus récents d'abord, avec une barre par jour. Une sauvegarde qui existe à plusieurs endroits n'apparaît qu'une fois, marquée avec chaque destination qui la contient. Un clic sur un instantané le restaure, et un clic sur un endroit à l'intérieur d'un instantané le restaure exactement à partir de là.

- **Destinations** nomme chaque endroit qui contient des sauvegardes de cet élément et leur nombre. Les endroits hors site ne sont lus que quand vous appuyez sur **Vérifier**, car lister un stockage dans le cloud prend du temps. Un endroit où l'élément ne sauvegarde plus figure sous **Anciennes sauvegardes uniquement** et peut encore servir à restaurer.
- Chaque instantané a un menu : **Ajouter une étiquette** ou **Changer le tag**, **Comparer avec une autre sauvegarde**, qui liste les fichiers arrivés, partis ou modifiés, **Exporter** en tar, en zip ou en dépôt restic à part, et la suppression à un seul endroit.
- Les **Sauvegardes SQL de la base** d'un conteneur figurent dans une liste à part, avec téléchargement, enregistrement dans un dossier et import.

Le nombre d'instantanés conservés dépend de la destination, voir [Durée de conservation](offsite-recovery.md#keep).

![L'onglet Sauvegardes d'un conteneur : une barre par jour, et les destinations qui contiennent chaque instantané.](assets/screenshots/backups/entry-backups-dark.png#only-dark)
![L'onglet Sauvegardes d'un conteneur : une barre par jour, et les destinations qui contiennent chaque instantané.](assets/screenshots/backups/entry-backups-light.png#only-light)

### Onglet Paramètres {#settings-tab}

Chaque élément commence par **Planning et destination** :

- **Au planning** fait entrer l'élément dans les exécutions planifiées ou l'en fait sortir.
- **Planning propre** donne à cet élément un plan à lui, voir [Un plan propre pour un élément](schedules.md#own-plan). Sans plan propre, il suit le plan de sa section ou la sauvegarde complète.
- Une rangée de boutons, **Local** et un par destination hors site, dit où vont les sauvegardes. **Suit la valeur par défaut** signifie que l'élément va là où va sa section, et un clic dessus ouvre **Destinations par section**. Voir [Destinations par élément](offsite-recovery.md#placement).
- **Dépôt** nomme l'endroit où les sauvegardes sont écrites (**Stocké sur**). Cet endroit est fixé dès la première sauvegarde, les copies peuvent changer à tout moment.

Les cartes en dessous dépendent du type d'élément :

| Type | Cartes |
|---|---|
| Conteneur | **Dossiers à sauvegarder** (quels dossiers montés sont sauvegardés, avec **Ignorer les dossiers de cache (CACHEDIR.TAG)**), **Motifs d'exclusion**, **Arrêter d'autres conteneurs**, **Commandes avant et après**, **Base de données**, **Quand l'application est occupée**, **Mettre à jour après une sauvegarde réussie**, **Instantané plutôt qu'arrêt**, **Anciens noms** |
| VM | **VM** avec **Méthode** (**Propre (arrêt)** ou **Instantané à chaud**) et **Blocs modifiés uniquement**, voir le [guide des sauvegardes de VM](vm-backup-ssh-setup.md#per-vm-method) |
| Ensemble de dossiers | **Dossier** et **Motifs d'exclusion** |
| Clé flash | **Export Flash en zip**, qui écrit chaque sauvegarde de la clé flash en `.zip` simple dans un dossier de votre choix, en une seule étape pour qu'un outil de synchronisation ne voie jamais un fichier à moitié écrit. Une sauvegarde flash qui contient moins qu'elle ne le devrait laisse le dernier ZIP en place, et l'exécution de l'export le signale. |
| Jeu de données ZFS | les jeux de données enfants, les conteneurs à arrêter et les commandes autour de l'instantané, **Réplique** et **Tester l'accès aux instantanés**, voir [Jeux de données ZFS](zfs-datasets.md) |

Les réglages s'enregistrent d'eux-mêmes quand vous les modifiez.

![L'onglet Paramètres d'un conteneur, avec Planning propre dans la première carte.](assets/screenshots/backups/entry-settings-dark.png#only-dark)
![L'onglet Paramètres d'un conteneur, avec Planning propre dans la première carte.](assets/screenshots/backups/entry-settings-light.png#only-light)

### Onglet Vérification {#verify-tab}

- **Vérifications** montre le contrôle de restauration qui suit la première sauvegarde. **Vérifier la restauration** le relance sur la dernière sauvegarde : BombVault restaure un échantillon d'au plus 100 fichiers et 256 MiB dans un dossier temporaire et fait relire chaque fichier par restic d'après ses empreintes.
- **Test de démarrage**, sur un conteneur, restaure la dernière sauvegarde dans une copie isolée et la démarre : un nom qui commence par `bombvault-test-`, un réseau Docker interne à part, sans port publié et sans accès au réseau local, 1 CPU et 2 GiB de mémoire. Le test réussit quand la copie se déclare saine, répond sur son premier port exposé ou, sans l'un ni l'autre, reste en marche. L'original n'est jamais touché, et la copie, son réseau et ses données sont supprimés ensuite, y compris après un redémarrage au milieu d'un test. Les conteneurs sur le réseau de l'hôte, les conteneurs privilégiés, ceux qui ont des périphériques et ceux qui ont besoin d'un autre conteneur sont indiqués comme non testables. **Test de démarrage** sous **Paramètres, Vérification, Vérifications automatiques** teste un conteneur par exécution, celui testé le plus anciennement d'abord.
- **Anomalies** règle la sensibilité et le niveau de notification propres à cet élément et montre ses anomalies ouvertes.
- **Taille par dossier** montre ce qui occupe la place dans la dernière sauvegarde et la part que la dernière sauvegarde a apportée, un niveau à la fois.
- **Exécutions récentes** liste les dernières exécutions avec leur début, leur fin et leur durée. Un clic en montre le détail.

![L'onglet Vérification d'un conteneur avec Taille par dossier ouvert.](assets/screenshots/backups/entry-verify-dark.png#only-dark)
![L'onglet Vérification d'un conteneur avec Taille par dossier ouvert.](assets/screenshots/backups/entry-verify-light.png#only-light)

## Motifs d'exclusion {#excludes}

La carte **Motifs d'exclusion** prend un motif par ligne. Saisissez les chemins tels que vous les voyez dans le conteneur, comme `/config/Library/Application Support/Plex Media Server/Cache`, ou un simple nom comme `.git`, qui correspond à n'importe quelle profondeur. Sur un ensemble de dossiers, saisissez un chemin tel que l'hôte le connaît, comme `/mnt/user/data/cache`. Un aperçu sous le champ montre ce que devient chaque ligne et prévient quand une ligne ne correspond à rien ou exclurait tout. Une ligne `/` ou `*`, ou une ligne qui nomme un dossier sauvegardé en entier, est ignorée, si bien qu'une exclusion ne peut jamais vider une sauvegarde. Pour laisser de côté un dossier entier, décochez-le plutôt sous **Dossiers à sauvegarder**.

Pour les images bien connues comme Plex, Jellyfin, Sonarr, Radarr, Immich et Nextcloud, la carte propose les dossiers que l'application remplit de nouveau toute seule, comme les caches, les journaux et les miniatures. Rien n'est exclu tant que vous n'appuyez pas sur **Exclure la sélection**.

La même carte porte des avertissements qu'une analyse de dossiers ne peut pas voir. Immich, par exemple, garde ses albums, ses visages et ses dates dans une base PostgreSQL dans un conteneur séparé : une sauvegarde de fichiers du seul conteneur Immich restaure donc les images sans rien de tout cela, et la restauration a l'air d'avoir réussi. Sauvegardez aussi le conteneur de la base.

## Sauvegardes SQL de la base {#database-dumps}

Les conteneurs PostgreSQL, MySQL et MariaDB que BombVault reconnaît sont exportés avant chaque sauvegarde, depuis le serveur en marche, dans un point de restauration à part à côté des fichiers. Cela couvre les images officielles, PostGIS, TimescaleDB, pgvector, pgautoupgrade, les images de base de données d'Immich, les images MariaDB de linuxserver, yobasystems et jc21 et le mysql-server d'Oracle. Un conteneur qui ressemble seulement à une base de données reçoit la même option, désactivée jusqu'à ce que vous la choisissiez.

Le dump va directement dans le dépôt et n'est jamais écrit sur un disque. Les identifiants viennent des variables du conteneur lui-même, y compris les secrets `*_FILE`, et ne le quittent jamais. La carte **Base de données** de l'onglet Paramètres du conteneur dit si le dossier de données est sauvegardé à l'arrêt, copié en marche ou non sauvegardé, et active ou désactive le dump pour ce conteneur. L'étiquette `bombvault.dbdump=false` fait la même chose et l'emporte sur la carte. **Exports SQL automatiques** sous **Paramètres, Général** l'active ou le désactive pour tous.

Un dump qui échoue ne fait pas échouer la sauvegarde. Il apparaît comme sa propre exécution échouée avec la raison et envoie une notification. Les dumps ne sont jamais rechargés tout seuls : téléchargez-en un, brut ou compressé, enregistrez-le dans un dossier, ou importez-le dans une base fraîchement démarrée en un clic sur l'onglet **Sauvegardes**. Pour récupérer un dump sans BombVault, voir [Récupérer un dump de base de données](offsite-recovery.md#database-dumps). Quand l'un d'eux échoue, voir [Un dump de base de données a échoué](troubleshooting.md#a-database-dump-failed).

## Commandes et autres conteneurs {#commands}

- **Commandes avant et après** s'exécutent dans le conteneur avec `sh -c`, par exemple pour vider un cache sur le disque. Une commande qui échoue avant la sauvegarde l'interrompt. Une commande avant la sauvegarde qui se termine avec le code 75 reporte plutôt une sauvegarde planifiée, voir plus bas.
- **Arrêter d'autres conteneurs** nomme les conteneurs à arrêter pendant que celui-ci est sauvegardé, comme sa base de données. Ensuite ils redémarrent dans l'ordre `depends_on` de leur Compose, et par défaut chacun doit se déclarer sain avant que le suivant démarre. L'attente et son délai se règlent sous **Redémarrage après la sauvegarde** dans la carte **Ordre** sous **Paramètres, Planning**.
- Un conteneur qui partage l'espace de noms réseau, PID ou IPC d'un conteneur qui s'arrête, via `--net=container:` ou `network_mode: service:` de Compose, s'arrête et redémarre avec lui même s'il n'est pas dans la liste, et il démarre après le conteneur qu'il rejoint. Un conteneur redémarré reçoit un nouveau réseau, et un conteneur resté en marche sur l'ancien réseau n'aurait plus que son interface loopback. La liste elle-même reste telle que vous l'avez réglée. Si un tel conteneur n'a pas pu être arrêté, l'exécution indique qu'il n'a pas de réseau tant qu'il n'est pas redémarré. Il en va de même pour le conteneur de BombVault lui-même, qui ne s'arrête jamais de lui-même et ne peut pas être mis dans la liste.
- Un conteneur qui ne repart pas ensuite est nommé dans l'exécution et dans une notification, même quand la sauvegarde elle-même a échoué.

## Quand l'application est occupée {#app-busy}

Une sauvegarde arrête son conteneur, une sauvegarde planifiée peut donc attendre pendant que quelqu'un utilise l'application et réessayer toute seule. La carte **Quand l'application est occupée** sur l'onglet Paramètres d'un conteneur la configure :

- **Vérification d'activité** demande à Plex, Jellyfin ou Emby, avant une sauvegarde planifiée, si quelqu'un regarde. Plex est interrogé avec le jeton de son propre `Preferences.xml`, il n'y a donc rien à saisir. Jellyfin et Emby demandent une **Clé API**, créée dans le tableau de bord du serveur sous API keys.
- **Ignorer les sessions en pause après (min)** fait qu'un film mis en pause pour la nuit cesse de compter.
- **Sauvegarder quand le serveur ne répond pas** décide de ce qui se passe quand on ne peut pas interroger le serveur : activé, la sauvegarde s'exécute et le journal le dit. Désactivé, elle attend comme si quelqu'un regardait.
- **Attendre tant que le conteneur envoie des données** sert aux applications sans un tel contrôle, comme un serveur de jeu ou un outil de téléchargement. La sauvegarde attend tant que le conteneur a envoyé plus que **Occupé à partir de (KiB/s)** à une mesure quelconque au cours des **Calme pendant (min)** dernières minutes.
- **Réessayer toutes les (min)**, **Essayer seulement dans une plage horaire** avec **Essayer entre**, et **Attendre au plus (heures)** règlent le déroulement des essais.
- **À la fin de l'attente** soit abandonne, ce qui montre la sauvegarde sous **À traiter** avec **Réessayer**, soit **Sauvegarder quand même**.

Une sauvegarde en attente est enregistrée comme **Reporté**, en ambre, et le reste de l'exécution continue. L'aperçu, la liste et l'élément la montrent avec le prochain essai, et l'alerte de retard et l'état de protection l'attendent jusqu'à son échéance. Vous recevez une notification quand elle est reportée et une quand elle se termine, pas une par essai. Un essai attend pendant qu'une autre sauvegarde tourne, et une attente continue après un redémarrage. Une sauvegarde que vous lancez vous-même s'exécute toujours tout de suite, que ce soit avec **Sauvegarder maintenant**, **Réessayer**, une sélection, l'API ou MCP, et celle qui passe met fin à une attente en cours.

L'onglet Paramètres du conteneur a aussi une manière plus ancienne et plus simple : **Attendre que l'application soit au repos** avec **Attendre au plus (heures)**. La sauvegarde démarre dès que l'application est au repos, ce que définit la carte **Repos avant la sauvegarde** sous **Paramètres, Planning**. Les membres d'une stack Compose attendus dans la même exécution attendent ensemble, et la sauvegarde en attente ne tient aucun verrou, si bien que les autres conteneurs avancent.

![La carte Quand l'application est occupée d'un conteneur Plex avec la vérification d'activité configurée.](assets/screenshots/backups/app-busy-dark.png#only-dark)
![La carte Quand l'application est occupée d'un conteneur Plex avec la vérification d'activité configurée.](assets/screenshots/backups/app-busy-light.png#only-light)

### Exemple : Plex attend tant que quelqu'un regarde {#example-plex}

La sauvegarde complète s'exécute chaque nuit à 03:00. Personne ne devrait voir un film s'arrêter parce que Plex s'est éteint pour sa sauvegarde, mais la sauvegarde doit quand même avoir lieu avant le matin.

1. Ouvrez **Sauvegardes**, puis l'élément `plex`, puis son onglet **Paramètres**.
2. Dans **Quand l'application est occupée**, réglez **Vérification d'activité** sur Plex. Il n'y a rien à saisir.
3. Réglez **Ignorer les sessions en pause après (min)** sur `30`, pour qu'un film mis en pause à minuit cesse de compter à minuit et demi.
4. Laissez **Sauvegarder quand le serveur ne répond pas** activé.
5. Réglez **Réessayer toutes les (min)** sur `15`.
6. Activez **Essayer seulement dans une plage horaire** et réglez **Essayer entre** sur `22:00` et `06:00`.
7. Réglez **Attendre au plus (heures)** sur `3` et **À la fin de l'attente** sur **Sauvegarder quand même**.

Voici ce qui se passe une nuit où quelqu'un regarde encore à 03:00 : BombVault interroge Plex, trouve une session en lecture et enregistre la sauvegarde comme **Reporté** avec le prochain essai à 03:15. Les autres conteneurs de l'exécution sont sauvegardés comme d'habitude. Quand le film se termine à 04:10, l'essai de 04:15 trouve Plex au repos et le sauvegarde, et l'attente se termine avec une notification. Si le film tournait encore à 06:00, l'attente arriverait à son terme et Plex serait sauvegardé quand même, ses sauvegardes ne prennent donc jamais plus de trois heures de retard.

Pour votre propre contrôle, faites terminer la **Commande pré-sauvegarde** avec le code 75 tant que l'application est occupée, par exemple `[ "$(cat /tmp/players)" = 0 ] || exit 75`. D'autres symptômes et remèdes figurent dans [Dépannage](troubleshooting.md#deferred-backups).

## Exports simples {#exports}

restic reste le moteur, mais un export vous donne une copie qui n'a besoin ni de BombVault ni de restic.

- **Exporter le conteneur** et **Exporter la VM** sous **Plus** sur la page d'un élément écrivent une copie telle qu'elle est maintenant, à côté du dépôt : les dossiers sauvegardés du conteneur plus son modèle Unraid, ou les images disque de la VM plus sa définition, restaurable avec `virsh define`.
- **Exporter** sur un instantané écrit une sauvegarde stockée dans le dossier d'export du serveur, en tar, en zip ou en dépôt restic à part, et peut prendre un instantané ou tous. Une sauvegarde flash et une auto-sauvegarde contiennent des clés, donc en exporter une demande un mot de passe de connexion et une connexion sur l'instance.

Les exports sont des fichiers simples par défaut. **Chiffrer les exports avec age** sous **Paramètres, Système** les scelle, le zip de la clé flash compris, pour les destinataires que vous listez, chacun une clé publique age ou une clé publique SSH. Vous les déchiffrez plus tard hors du serveur avec la clé privée correspondante. Avec le chiffrement activé et aucun destinataire valide, un export échoue avec une erreur claire au lieu d'écrire jamais des fichiers simples.

## Ce qu'une exécution vous dit {#run-notes}

- **Durée.** Chaque exécution indique début, fin et durée, et chaque élément a ses propres **Exécutions récentes**.
- **Pourquoi une sauvegarde a été lente.** Pendant qu'une sauvegarde tourne, BombVault surveille l'occupation du CPU, des disques et du réseau. Quand une sauvegarde prend beaucoup plus de temps que d'habitude et qu'une seule chose était nettement à sa limite, l'exécution le dit, par exemple « Le disque cible disk1 était occupé à 98 % ». Sinon, elle ne dit rien.
- **Progression des vérifications et des élagages.** Le journal d'activité et la carte **Vérifier les sauvegardes** montrent jusqu'où restic a compté, par exemple *12 paquets sur 47*, avec le temps restant pour cette étape dès qu'il y a assez de données pour l'estimer.
- **Fichiers illisibles.** Une sauvegarde qui est passée mais a laissé quelque chose de côté passe sa ligne en orange et nomme ce qui a été laissé de côté.
