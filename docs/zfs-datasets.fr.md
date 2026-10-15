# Jeux de données ZFS

Les jeux de données ZFS sont des éléments de la liste **Sauvegardes**, comme les conteneurs et les VMs. Un élément est un jeu de données avec tous ceux qui se trouvent en dessous. Pour chaque sauvegarde, BombVault prend un instantané ZFS de tout l'arbre, de sorte que chaque jeu de données est capturé au même instant. Il lit ensuite les fichiers de chaque jeu de données depuis cet instantané et les stocke avec restic, de la même façon qu'un dossier, puis supprime l'instantané aussitôt. Les sauvegardes sont dédupliquées, vous pouvez parcourir chacune d'elles, et des fichiers isolés peuvent être restaurés.

La sauvegarde n'envoie jamais de flux dans restic et ne fait jamais revenir un jeu de données en arrière. BombVault ne détruit que les instantanés qu'il a créés lui-même. Le [réplica](#replica) facultatif est le seul endroit qui utilise `zfs send` : il copie les jeux de données vers un second serveur ZFS et ne touche pas à la sauvegarde.

## Prérequis {#requirements}

- **La liaison SSH vers ce serveur.** Les jeux de données ZFS utilisent la même clé, le même hôte et le même utilisateur que les sauvegardes de VM. Si les sauvegardes de VM fonctionnent déjà, ceci fonctionne aussi. Sinon, suivez le [guide de la sauvegarde de VM via SSH](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) sur GitHub. Les champs du modèle s'appellent **Host SSH: Address**, **Host SSH: Port** et **Host SSH: User**.
- **La commande `zfs` sur cet hôte.** Unraid 6.12 et plus récent ainsi que TrueNAS SCALE l'ont.
- **Host Data mappé sur `/mnt` avec l'Access Mode Read/Write - Slave.** C'est la valeur par défaut du modèle. L'instantané d'un jeu de données n'apparaît dans le dossier `.zfs/snapshot` du jeu de données qu'après le démarrage de BombVault, le conteneur doit donc recevoir les montages que l'hôte fait plus tard.
- **Les jeux de données montés sous `/mnt`.** Sous Unraid, les pools se trouvent dans `/mnt/<pool>`, c'est donc déjà le cas.

Activez la section sous **Paramètres, Général** (Jeux de données ZFS). **Connexion ZFS à ce serveur**, sous **Plus** sur la page Sauvegardes, s'ouvre alors. Elle teste la liaison SSH, nomme l'utilisateur et l'hôte auxquels elle se connecte, et dit ce qui manque quand quelque chose manque. Quand la liaison est coupée, la liste Sauvegardes le dit avec **Voir la connexion**, et la carte **Intégration hôte** de l'aperçu montre le même résultat.

## Éléments et jeux enfants {#items-and-children}

Choisissez **Ajouter** sur la page Sauvegardes, puis **Un jeu de données ZFS**. La liste vient du serveur. Choisissez le jeu de données au sommet de ce que vous voulez sauvegarder, par exemple `cache/appdata`, et l'élément le couvre ainsi que tous les jeux de données en dessous.

- **Les nouveaux jeux enfants rejoignent d'eux-mêmes.** Un jeu de données créé plus tard sous l'élément est sauvegardé à l'exécution suivante, et cette exécution le nomme comme nouveau. Sa première sauvegarde le lit en entier une fois ; ensuite, seuls les changements sont lus.
- **Vous pouvez écarter des enfants isolés.** Désactivez un enfant dans l'onglet Paramètres de l'élément et il est écarté avec tout ce qui se trouve en dessous. Un enfant écarté qui n'existe plus sur le serveur est marqué comme tel et peut être retiré de la liste.
- **Les enfants illisibles sont ignorés, jamais en silence.** L'exécution les liste, l'élément montre combien ont été ignorés, et la carte **Non sauvegardé** de l'aperçu compte chacun comme non protégé. L'exécution sauvegarde quand même tout le reste et n'échoue pas à cause d'un enfant ignoré. Les raisons figurent dans le [tableau des codes de motif](#reason-codes) : un jeu de données qui n'est pas monté, a `canmount=off`, un point de montage `legacy` ou aucun, une clé de chiffrement non chargée, l'accès aux instantanés désactivé ou un point de montage que BombVault ne voit pas.
- **Un jeu de données ignoré n'entraîne pas ses enfants avec lui.** Un jeu de données en `canmount=off` qui ne contient que d'autres jeux de données est ignoré (affiché comme « structure seulement ») et ses enfants montés sont sauvegardés. Un jeu de données chiffré dont la clé n'est pas chargée est ignoré avec les enfants qui partagent sa clé.
- **Les enfants qui sont des disques de VM ou des données système démarrent désactivés** dans la boîte de dialogue d'ajout, avec la raison à côté de l'interrupteur. Ajouter un pool entier demande une confirmation qui liste ce qu'il contient.

### Volumes {#volumes}

Un volume (zvol) contient un disque virtuel au lieu de fichiers, et un élément ZFS n'en sauvegarde jamais.

- Un volume qu'utilise une VM est sauvegardé avec cette VM.
- Un volume qu'aucune VM n'utilise (une extension iSCSI, un disque que vous avez détaché) **n'est pas sauvegardé par BombVault**. La fenêtre d'ajout et la page Sauvegardes comptent ces volumes et le disent. Une version ultérieure les sauvegardera.

Les volumes dans l'arbre d'un élément sont ignorés et nommés à chaque exécution.

### Le stockage de Docker {#docker-storage}

Avec le pilote de stockage ZFS de Docker, chaque couche d'image est un jeu de données avec un point de montage `legacy`. La fenêtre d'ajout les replie en une ligne par parent. Un arbre qui contient plus de 20 de ces jeux de données ne peut pas devenir un élément tant qu'un instantané de lui existe : Docker ne peut pas supprimer de couches d'image. Ajoutez à la place les jeux de données en dessous, par exemple `appdata`.

### Les éléments ne se chevauchent jamais {#overlap}

Un jeu de données ne peut appartenir qu'à un seul élément. BombVault refuse un nouvel élément qui se trouve à l'intérieur d'un élément existant ou qui en contiendrait un. Pour fusionner plusieurs éléments enfants en un élément parent, supprimez d'abord les éléments enfants en choisissant de garder leurs sauvegardes, puis ajoutez le parent. Chaque jeu de données garde son historique sous son propre nom, si bien que la sauvegarde suivante reprend là où les anciens éléments se sont arrêtés et ne relit pas tout.

## Arrêter des conteneurs et lancer des commandes autour de l'instantané {#consistency}

Un instantané d'une base de données en marche ressemble à une coupure de courant soudaine : la base de données récupère en général, mais elle doit le faire. Chaque élément peut faire deux choses à ce sujet, et les deux ne couvrent que l'instant de l'instantané, pas toute la sauvegarde.

- **Arrêter ces conteneurs pour l'instantané.** BombVault arrête les conteneurs listés, prend l'instantané et les redémarre aussitôt. Les conteneurs d'un même niveau de dépendances s'arrêtent en parallèle, les dépendants d'abord, si bien que toute la fenêtre dure généralement quelques secondes ; l'exécution indique combien de temps. La sauvegarde lit ensuite l'instantané figé pendant que les applications tournent déjà de nouveau. Seuls les conteneurs qui tournaient sont arrêtés.
- **Une commande avant et après l'instantané.** Elle s'exécute dans un conteneur de votre choix, par exemple pour exporter une base de données dans le jeu de données juste avant l'instantané, sans rien arrêter. Si la commande d'avant l'instantané échoue, la sauvegarde échoue et aucun instantané n'est pris. Une commande d'après l'instantané qui échoue est signalée sur l'exécution mais ne fait pas échouer la sauvegarde.

Ce qui se passe quand quelque chose tourne mal :

- Si un conteneur ne peut pas être arrêté, BombVault redémarre ceux qu'il a déjà arrêtés et la sauvegarde échoue en nommant le conteneur. Il ne se rabat jamais sur un instantané d'applications en cours d'exécution.
- L'arrêt attend la fin d'une sauvegarde de conteneur en cours (jusqu'à 30 minutes pour une exécution manuelle, jusqu'à la durée limite de sauvegarde pour une exécution planifiée), afin que les deux n'arrêtent et ne démarrent jamais le même conteneur en même temps.
- Avant l'arrêt du premier conteneur, BombVault note lesquels il arrête. Si BombVault est tué dans la fenêtre, il redémarre ces conteneurs au prochain démarrage, envoie une notification, et l'élément montre une remarque rouge pour chaque conteneur qu'il n'a pas pu démarrer.

Les dumps automatiques de bases de données (voir [Fonctionnalités](features.md)) s'exécutent avec la sauvegarde propre d'un conteneur, pas avec un élément ZFS. Une base de données dont le conteneur n'est sauvegardé que via son jeu de données n'a pas de dump, donnez-lui donc une commande ici.

Un conteneur peut figurer dans cette liste et être sauvegardé comme conteneur en même temps. Ses données sont alors stockées deux fois, dans deux dépôts, et la sauvegarde complète l'arrête deux fois. L'élément le dit.

## Restaurer {#restore}

Ouvrez l'élément et choisissez **Restauration**, ou choisissez-le sur la page **Restauration**. La fenêtre de restauration demande la sauvegarde, puis le **Jeu de données**. Par défaut, c'est le jeu de données supérieur de l'élément. Sous **Quoi** vous restaurez tout ou seulement les fichiers et dossiers choisis. **Où** propose :

- **Restaurer dans le jeu de données.** Les fichiers de la sauvegarde sont écrits dans le point de montage du jeu de données. Les fichiers portant le même nom sont écrasés, les autres restent. Le jeu de données n'est jamais ramené en arrière ni remplacé. BombVault vérifie que le jeu de données est monté, visible et accessible en écriture, une fois avant de commencer et de nouveau juste avant d'écrire. Là où un jeu de données enfant est monté à l'intérieur, rien n'est écrit : l'enfant garde ses fichiers, son propriétaire et ses permissions, et est restauré depuis sa propre sauvegarde.
- **Dans un dossier.** Choisissez un dossier sous `/mnt`. BombVault vérifie que le dossier est sur un pool ou un partage monté et qu'il y a assez d'espace libre. Cela fonctionne sans la liaison SSH et pour des jeux de données qui n'existent plus.
- **Dans un nouveau jeu de données.** Nommez un jeu de données qui n'existe pas encore ; la fenêtre suggère `<jeu de données>-restored`. BombVault le crée avec les propriétés ZFS stockées dans la sauvegarde et restaure dedans, voir [Restaurer comme nouveau jeu de données](#new-dataset).

Choisir **Tous les jeux de données de cette sauvegarde** comme jeu de données écrit chaque jeu de données de l'arbre dans son propre sous-dossier du dossier choisi. Les jeux de données ignorés dans cette sauvegarde sont nommés. Pour restaurer depuis le dépôt d'un autre BombVault, utilisez **Depuis un ancien dossier de sauvegarde** sur la page Restauration ; il restaure toujours dans un dossier : tous les jeux de données d'une sauvegarde, chacun dans son sous-dossier, ou un jeu de données de l'arbre, entier ou avec des fichiers choisis.

La liste des conteneurs à arrêter de l'élément est aussi proposée pour une restauration dans le jeu de données. Ces conteneurs restent arrêtés pendant toute la restauration, et les sauvegardes de conteneurs attendent entre-temps.

### L'instantané de sécurité {#safety-snapshot}

Avant d'écrire dans un jeu de données, BombVault prend un instantané ZFS de ce seul jeu de données, nommé `bombvault-prerestore-<heure>`. **Garder d'abord un instantané ZFS de l'état actuel** est activé par défaut ; le désactiver demande une seconde confirmation. Si l'instantané ne peut pas être pris, rien n'est restauré.

BombVault ne supprime jamais un instantané de sécurité de lui-même. L'élément les liste avec leur âge et leur taille, chacun avec une action **Supprimer**, et avertit quand le plus ancien a plus de 30 jours, car il garde sur le pool des données supprimées et modifiées.

Pour revenir en arrière après une restauration, copiez des fichiers isolés depuis `.zfs/snapshot/bombvault-prerestore-<heure>` dans le jeu de données. `zfs rollback <dataset>@bombvault-prerestore-<heure>` ne fonctionne que tant que c'est l'instantané le plus récent de ce jeu de données. `zfs rollback -r` supprime tous les instantanés plus récents, y compris les automatiques.

### Restaurer comme nouveau jeu de données {#new-dataset}

BombVault enregistre à chaque sauvegarde les propriétés ZFS définies localement de chaque jeu de données : compression, recordsize, quota, reservation, atime, xattr, acltype, casesensitivity et vos propres propriétés utilisateur. Les valeurs héritées et en lecture seule sont laissées de côté, car elles reviennent d'elles-mêmes. Les sauvegardes antérieures à cet enregistrement n'en ont pas.

- **Dans un nouveau jeu de données** exécute `zfs create` avec chaque propriété enregistrée. casesensitivity, normalization et utf8only ne peuvent être définies que de cette façon. Les quotas et réservations sont appliqués après les fichiers, pour qu'ils ne puissent pas les refuser. Le point de montage est laissé de côté pour que la copie n'entre pas en conflit avec l'original, tout comme `canmount`, `readonly` et le chiffrement, pour que la restauration puisse écrire. Un nouveau jeu de données placé sous un jeu chiffré reprend son chiffrement. Le jeu de données parent doit exister. Si quelque chose échoue après sa création, le nouveau jeu de données reste sur le serveur, car BombVault ne détruit jamais un jeu de données.
- **Restaurer dans le jeu de données** affiche les propriétés enregistrées à côté de la restauration. **Appliquer aussi ces propriétés** applique celles qu'un jeu de données existant accepte encore, avant l'écriture du moindre fichier. Les quotas et réservations sont appliqués après les fichiers, pour qu'ils ne puissent pas les refuser. Sans ce bouton, le jeu de données garde ses réglages.

## Ce que contient la sauvegarde {#contents}

Dans la sauvegarde : les fichiers et dossiers de chaque jeu de données sauvegardé, avec leur propriétaire, leurs permissions, leurs horodatages et leurs attributs étendus, tels que restic les stocke, ainsi que les propriétés ZFS définies localement de chaque jeu de données.

Pas dans la sauvegarde :

- le propriétaire et les permissions du dossier supérieur de chaque jeu de données lui-même (tout ce qui est en dessous est inclus). Une restauration dans le jeu de données laisse le dossier supérieur existant tel quel, une restauration dans un dossier le crée avec les permissions `0755` ;
- les instantanés ZFS existants ;
- les enfants ignorés ou exclus ;
- les volumes.

Pour restaurer sur un nouveau pool, créez le pool et restaurez chaque jeu de données dans un nouveau jeu de données. Il n'a pas encore été vérifié si les ACL NFSv4, telles que TrueNAS les utilise sur les jeux de données SMB, reviennent comme vous l'attendez ; testez donc une restauration sur vos propres données avant de compter dessus.

## Jeux de données chiffrés {#encryption}

Un jeu de données chiffré n'est sauvegardé que tant que sa clé est chargée. Sinon, il est ignoré avec un avertissement ; chargez la clé avec `zfs load-key` et montez le jeu de données. BombVault lit les données déchiffrées et les stocke dans le dépôt de restic, qui est chiffré. Si vous avez désactivé le chiffrement dans BombVault, ce dépôt ne l'est pas.

## Réplica {#replica}

Une réplique est une copie des jeux de données d'un élément sur un second serveur ZFS. BombVault la tient à jour avec `zfs send` et `zfs receive`. La première exécution envoie tout, ensuite seuls les blocs modifiés circulent. Sur l'autre serveur, vous pouvez monter la copie tout de suite.

Une réplique ne remplace jamais la sauvegarde. Les anciennes versions, les fichiers isolés et le contrôle viennent toujours des sauvegardes, et la réplique ne garde que le nombre d'instantanés que vous fixez. Une réplique à jour compte comme une copie hors des locaux, mais un élément avec une réplique et sans sauvegarde reste orange.

Activez-la dans la carte **Réplique** de l'onglet Paramètres de l'élément. Vous y choisissez où va la réplique, quand elle s'exécute (**Après chaque sauvegarde** ou **Planning propre**) et combien d'instantanés restent sur la cible. La carte liste chaque jeu de données et volume avec son état, et **Répliquer maintenant** lance une exécution. Une exécution de réplique a son propre verrou, si bien qu'un long premier transfert ne retient jamais les sauvegardes.

### Envoi vers un serveur ZFS {#replica-push}

N'importe quelle machine avec ZFS et SSH peut recevoir, par exemple un second Unraid ou un TrueNAS. BombVault n'a pas besoin d'y tourner.

1. Sous **Paramètres, Appairage**, choisissez **Connecter via SSH** dans **Serveur ZFS sans BombVault**, ou **Ajouter un serveur ZFS** dans la carte **Réplique** de l'élément.
2. Saisissez l'adresse, l'utilisateur et le port. La boîte de dialogue affiche la clé publique de BombVault. Ajoutez-la au fichier `~/.ssh/authorized_keys` de l'utilisateur sur le serveur. Sur Unraid, cela se trouve sous **Users, root, SSH keys**. Selon **Display Settings**, Users figure dans le menu d'en-tête ou sous Settings. Sur TrueNAS, modifiez l'utilisateur sous **Credentials, Users**. Il s'agit de la clé du réplica. Elle diffère de la clé de la liaison SSH pour les VM et l'hôte, ajoutez-la donc même si celle-là est déjà autorisée.
3. Testez la connexion. La boîte de dialogue liste ensuite les pools du serveur. Choisissez-en un et réglez la racine, qui vaut par défaut `<pool>/bombvault-replica`.
4. Choisissez le nouveau serveur dans la carte **Réplique** de l'élément. Le serveur a alors sa propre carte sur la page Instances, et **Ouvrir** mène à sa page.

Avec root, rien d'autre n'est nécessaire. Un utilisateur dédié a besoin de ces permissions sur le pool de la cible, que la boîte de dialogue affiche aussi :

```
zfs allow <user> receive,create,mount,rollback,destroy,userprop <pool>
```

Sur la source, le même genre d'utilisateur a besoin de ceci sur le jeu de données supérieur de l'élément :

```
zfs allow <user> send,snapshot,hold,release,bookmark,destroy <dataset>
```

Dans ce sens, le BombVault qui détient l'élément détient aussi une clé qui peut écrire sur le serveur.

BombVault enregistre la clé d'hôte que le serveur présente à la première connexion et en refuse ensuite toute autre. Si le serveur a été réinstallé, cliquez sur **Forget host key** sur la page du serveur, et la connexion suivante enregistrera la nouvelle clé.

### Envoi vers une instance appairée {#replica-receive}

Un BombVault appairé peut recevoir lui-même le réplica. Personne n'obtient d'accès SSH à l'autre hôte, et aucune clé n'entre dans un fichier `authorized_keys`.

1. Dans la carte **Réplique** de l'élément, choisissez l'instance appairée comme cible, ou activez le rôle **Serveur ZFS** sur la carte de cette instance sur la page Instances et choisissez-y les éléments ZFS. La carte affiche **En attente d'approbation** jusqu'à ce qu'elle réponde.
2. Sur l'instance réceptrice, ouvrez **Instances**. La demande apparaît sous forme de badge sur la carte de l'instance émettrice, et ses **Détails** la listent. Choisissez le pool et la racine sur ce serveur et le nombre d'instantanés à garder, puis appuyez sur **Autoriser** ou **Refuser**.
3. Après **Allow**, la source envoie selon son propre plan, comme pour toute autre cible.

L'instance réceptrice n'accepte que ce que couvre l'approbation : les jeux de données de l'élément, dans sa propre racine. Elle exécute elle-même `zfs receive`, et la source n'a aucun moyen de supprimer ou de revenir en arrière sur quoi que ce soit là-bas. La copie survit donc à une source dont quelqu'un a pris le contrôle. L'instance réceptrice garde sa propre rétention. La source ne fait que suggérer une règle quand elle demande.

**Revoke access** sur l'instance qui reçoit met fin à l'approbation à tout moment et en informe la source, qui indique alors que l'approbation a été retirée et s'arrête. Ce que l'instance qui reçoit détient déjà reste chez elle. Une demande refusée reste refusée. Des jeux de données supplémentaires, ou une demande après une révocation, redemandent une approbation.

L'instance qui reçoit doit répondre en HTTPS. La source n'envoie rien à une adresse http simple, car le jeton et les données traverseraient le réseau sans chiffrement. Chaque transfert laisse aussi libre un dixième du pool qui reçoit et s'arrête avant d'y toucher, si bien qu'une seule source ne peut pas remplir le pool du serveur qui reçoit.

### Où atterrissent les données {#replica-target}

Chaque jeu de données atterrit dans `<root>/<server>/<pool>/<path>`. Le dossier du serveur est le nom de l'instance source, fixé au premier transfert, si bien que deux serveurs avec le même nom de pool ne se gênent jamais. Par exemple, `cache/appdata` d'un serveur nommé `tower` aboutit dans `backup/bombvault-replica/tower/cache/appdata`.

La copie sur la cible est en lecture seule et non montée, donc elle ne recouvre jamais rien sur ce serveur. Les propriétés ZFS suivent, sauf le point de montage, `sharenfs` et `sharesmb`. Les réservations `reservation` et `refreservation` restent aussi en place, la copie n'occupe donc que l'espace dont ses données ont besoin.

### Ce qui est inclus {#replica-contents}

Tout ce que l'élément sauvegarde y entre, et les volumes en dessous aussi, que la sauvegarde ignore. Un jeu enfant que vous avez désactivé dans l'élément reste dehors. Tous les jeux de données d'une exécution proviennent d'un seul instantané, comme dans la sauvegarde.

### Combien de temps les instantanés restent {#replica-retention}

Sur la cible, un nouveau réplica garde 7 instantanés quotidiens et 3 hebdomadaires. Choisissez plutôt **Short**, **Balanced** ou **Long**, ou réglez **Custom values**. Seuls les instantanés nommés `bombvault-replica-<14 chiffres>` y sont supprimés, et jamais le plus récent que les deux côtés partagent.

Sur la source, BombVault ne garde que le dernier instantané de réplica, plus un signet pour chaque état envoyé. Les signets ne coûtent pas d'espace. Le transfert suivant part de là.

### Jeux de données chiffrés dans un réplica {#replica-encryption}

Un jeu de données chiffré est envoyé brut. Il reste chiffré sur la cible, et la cible ne voit jamais la clé. Gardez bien la clé : vous en avez besoin pour ouvrir la copie après une restauration, et un réplica sans elle est illisible.

### Utiliser le réplica {#replica-use}

Ouvrez l'onglet **Sauvegardes** de l'élément et cliquez sur la ligne **Réplique ZFS** parmi les endroits qui détiennent ses instantanés. La feuille liste les instantanés sur la cible et montre les commandes avec vos vrais noms.

Pour regarder un ancien état, clonez un instantané sur la cible. Un clone ne prend aucune place tant que rien ne change, et le réplica reste intact :

```
zfs clone backup/bombvault-replica/tower/cache/appdata@bombvault-replica-20261006014100 backup/bombvault-replica/clone-appdata
```

Si la source tombe en panne, transformez la copie en jeu de données normal, accessible en écriture, sur la cible :

```
zfs inherit -r readonly backup/bombvault-replica/tower/cache/appdata && zfs inherit -r canmount backup/bombvault-replica/tower/cache/appdata && zfs mount -a
```

Ensuite, BombVault cesse de répliquer vers ce jeu de données, jusqu'à ce que vous lanciez une nouvelle première exécution.

Pour ramener un état sur la source, appuyez sur **Bring back as a new dataset** dans la feuille. BombVault envoie l'instantané dans un nouveau jeu de données à côté de l'original, nommé `<dataset>-bombvault-restore-` suivi d'un horodatage. Il n'écrit jamais par-dessus l'original.

## Instantanés restants {#leftover-snapshots}

L'instantané d'une sauvegarde s'appelle `<dataset>@bombvault-<14 chiffres>`, par exemple `cache/appdata@bombvault-20260924021500` (UTC). BombVault le supprime juste après la sauvegarde. Si cela échoue, par exemple parce que le jeu de données est occupé ou que BombVault a été arrêté, BombVault le supprime :

- avant la prochaine sauvegarde de cet élément,
- au démarrage de BombVault, pour chaque élément, y compris avec la section désactivée,
- quand vous supprimez l'élément,
- quand vous appuyez sur **Retirer maintenant** sur l'élément, qui montre combien il en reste.

Seuls les noms qui correspondent exactement à `bombvault-` suivi de 14 chiffres sont supprimés. Les instantanés de sécurité, vos propres instantanés et les instantanés automatiques ne sont jamais touchés. Pour en supprimer un à la main :

```
zfs destroy -r cache/appdata@bombvault-20260924021500
```

L'instantané d'un réplica s'appelle `<dataset>@bombvault-replica-<14 chiffres>` et n'est pas un reste. Il reste sur la source jusqu'à ce que l'exécution de réplica suivante le remplace, et sur la cible tant que la rétention le garde. Le nettoyeur n'y touche jamais, car il ne correspond qu'à `bombvault-` suivi d'exactement 14 chiffres.

## Anomalies {#anomalies}

Un enfant vidé change à peine le total d'un grand arbre, donc la détection d'anomalies surveille chaque jeu de données d'un élément séparément : sa taille, son nombre de fichiers, ses nouvelles données et son temps restic ont chacun leur propre historique. Cet historique appartient au nom du jeu de données, il subsiste donc quand l'arbre est plus tard sauvegardé par un autre élément.

Un jeu de données que l'exécution précédente a sauvegardé et que celle-ci n'a pas pu lire compte comme vidé, tant que la sélection de l'élément n'a pas changé. Cela couvre une clé non chargée, un jeu de données non monté et un jeu qui a disparu de l'arbre. Un enfant que vous excluez vous-même change la sélection, donc son historique repart à zéro. Tant qu'un constat de données perdues est ouvert, la rétention garde les anciennes sauvegardes de ce seul jeu de données et élague le reste de l'arbre comme d'habitude.

Chaque jeu de données a une ligne à lui dans la surveillance de l'élément, qui s'ouvre avec **Surveillance** sur la page **Anomalies** et se trouve dans l'onglet **Vérification** de l'élément. Les constats ouverts s'affichent à côté de chaque jeu de données. Le lien d'un constat ouvre la fenêtre de restauration de l'élément sur la dernière bonne sauvegarde du jeu de données. Le fait qu'une exécution se termine est jugé pour tout l'élément, car une exécution réussit ou échoue en bloc.

Les contrôles eux-mêmes sont décrits sous [Fonctionnalités](features.md). Un assistant connecté via le [serveur MCP](mcp.md) peut lister les points de restauration d'un élément ZFS, lancer sa sauvegarde et lire les constats, mais en acquitter un se fait sur la page **Anomalies**.

## Codes de raison {#reason-codes}

La page de l'élément, l'historique des exécutions et les notifications nomment un problème avec l'un de ces codes. La plupart ont aussi la solution à côté sur la page.

| Code | Signification | Que faire |
|---|---|---|
| `ssh-missing` | La connexion SSH n'est pas configurée dans ce conteneur. | Configurez la liaison SSH comme pour les sauvegardes de VM. |
| `host-placeholder` | Host SSH: Address est toujours la valeur d'exemple, et `host.docker.internal` n'a pas répondu non plus. | Réglez Host SSH: Address sur l'IP LAN de ce serveur. |
| `host-fallback` | Host SSH: Address est toujours la valeur d'exemple, et `host.docker.internal` fonctionne. | Rien, ou réglez l'IP LAN. |
| `ssh-unreachable` | Le serveur est injoignable en SSH. | Vérifiez l'adresse et le port, et que SSH est activé. |
| `ssh-auth` | Le serveur a refusé la clé de BombVault. | Exécutez une fois sur le serveur la commande affichée sous **Connexion à ce serveur**. |
| `zfs-not-found` | L'hôte SSH n'a pas de commande `zfs`. | Faites pointer Host SSH: Address vers la machine qui possède les pools. |
| `zfs-permission` | L'utilisateur SSH n'a pas le droit d'exécuter cette commande zfs. | Utilisez root, ou voir [TrueNAS SCALE](#truenas). |
| `uri-mismatch` | `LIBVIRT_URI` désigne un autre hôte ou un autre utilisateur que les champs SSH. | Mettez-les d'accord, ou videz les champs SSH pour que les deux viennent de l'URI. |
| `zfs-error` | zfs a signalé une autre erreur. | Les détails affichent son message. |
| `propagation-missing` | Les nouveaux montages sur l'hôte n'atteignent pas le conteneur. | Réglez l'Access Mode de Host Data sur Read/Write - Slave et redémarrez BombVault. |
| `invalid-name` | Un nom de jeu de données que BombVault n'accepte pas. | Renommez le jeu de données. |
| `name-too-long` | Un jeu de données de l'arbre est trop long pour un nom d'instantané. | Renommez-le, ou ajoutez comme élément un jeu de données en dessous. |
| `invalid-exclude` | Un motif d'exclusion ou un enfant écarté ne convient pas à l'élément. | Corrigez l'élément que nomme le message. Pour écarter tout un jeu enfant, désactivez-le au lieu d'écrire un motif. |
| `not-found` | Le jeu de données n'existe pas sur le serveur. | Supprimez l'élément ou recréez le jeu de données. Ses sauvegardes restent restaurables. |
| `not-filesystem` | C'est un volume, pas un système de fichiers. | Voir [Volumes](#volumes). |
| `overlaps-item` | Le jeu de données chevauche un élément existant. | Voir [Les éléments ne se chevauchent jamais](#overlap). |
| `docker-storage` | L'arborescence contient le stockage d'images de Docker. | Voir [Le stockage de Docker](#docker-storage). |
| `nothing-readable` | Aucun jeu de données de l'élément ne peut être lu pour le moment. | Regardez les codes des jeux de données ignorés. |
| `snapshot-failed` | L'instantané n'a pas pu être créé. | Les détails affichent le message de zfs. |
| `containers-busy` | Une sauvegarde de conteneur tournait encore quand les conteneurs devaient s'arrêter. | Relancez plus tard. Les exécutions planifiées attendent d'elles-mêmes. |
| `consistency-stop-failed` | Un conteneur n'a pas pu être arrêté, donc aucun instantané n'a été pris. | Vérifiez le conteneur, ou retirez-le de la liste. |
| `pre-snapshot-failed` | La commande d'avant l'instantané a échoué. | Les détails de l'exécution affichent sa sortie. |
| `container-unknown` | Un conteneur listé n'existe pas. | Retirez-le de la liste. |
| `container-is-self` | BombVault ne peut pas arrêter son propre conteneur. | Retirez-le de la liste. |
| `leftover-snapshots` | Des instantanés que BombVault n'a pas pu supprimer sont encore sur le serveur. | Appuyez sur **Retirer maintenant**, voir [Instantanés restants](#leftover-snapshots). |
| `zvol` | Un volume dans l'arborescence, ignoré. | Voir [Volumes](#volumes). |
| `canmount-off` | Jamais monté (`canmount=off`), ignoré. | S'il contient des données, montez-le ou déplacez les données dans un jeu de données enfant. |
| `legacy-mount` | Point de montage legacy, ignoré. | Donnez-lui un point de montage sous `/mnt`. |
| `no-mountpoint` | Pas de point de montage, ignoré. | Donnez-lui un point de montage sous `/mnt`. |
| `not-mounted` | Non monté sur le serveur, ignoré. | Montez-le avec `zfs mount`, ou réglez `canmount=on`. |
| `key-not-loaded` | Chiffré et la clé n'est pas chargée, ignoré. | `zfs load-key`, puis montez-le. |
| `snapdir-disabled` | L'accès aux instantanés est désactivé, ignoré. | `zfs set snapdir=hidden <dataset>`. Le dossier `.zfs` reste caché. |
| `not-visible` | BombVault ne voit pas le point de montage du jeu de données. | Déplacez le point de montage sous le chemin Host Data, ou mappez-le dans le conteneur au même chemin avec Read/Write - Slave. |
| `shfs-only` | Le jeu de données n'est visible qu'à travers `/mnt/user`, qui cache les instantanés. | Mappez `/mnt`, et non `/mnt/user`, comme Host Data. |
| `snapshot-not-visible` | L'instantané a été créé mais n'est pas apparu dans BombVault. | Lancez **Tester l'accès aux instantanés** ; voir ci-dessous. |
| `snapshot-loop` | L'instantané n'a pas atteint BombVault parce que Host Data ne transmet pas les nouveaux montages. | Réglez l'Access Mode de Host Data sur Read/Write - Slave et redémarrez BombVault. |
| `backup-failed` | restic a échoué pour ce jeu de données. | Les détails de l'exécution disent pourquoi. |
| `not-reached` | L'exécution s'est terminée avant ce jeu de données. | Relancez la sauvegarde. |
| `gone` | Le jeu de données n'est plus sur le serveur. | Rien. Ses sauvegardes restent restaurables. |
| `read-only-mount` | BombVault ne peut que lire le jeu de données, il ne peut donc pas restaurer dedans. | Réglez le mappage sur Read/Write - Slave, ou restaurez dans un dossier. |
| `destination-not-mounted` | Le dossier n'est pas sur un pool ou un partage monté. | Choisissez un dossier sur un pool ou un partage. |
| `not-enough-space` | Pas assez d'espace libre à la destination. | Libérez de l'espace ou choisissez un autre dossier. |
| `safety-snapshot-failed` | L'instantané de sécurité n'a pas pu être pris, donc rien n'a été restauré. | Les détails affichent le message de zfs. |
| `safety-name-too-long` | Le nom du jeu de données est trop long pour un instantané de sécurité. | Désactivez l'instantané de sécurité, ou restaurez dans un dossier. |
| `dataset-exists` | Un jeu de données porte déjà ce nom. | Choisissez un autre nom, ou restaurez dans le jeu de données lui-même. |
| `create-failed` | Le nouveau jeu de données n'a pas pu être créé. | Les détails montrent le message de zfs. Vérifiez que le jeu de données parent existe. |
| `new-dataset-not-visible` | Le nouveau jeu de données a été créé, mais BombVault ne le voit pas, donc rien n'a été restauré. | Le jeu de données reste sur le serveur. Montez-le sous le chemin Host Data et restaurez dedans. |
| `set-properties-failed` | Les propriétés enregistrées n'ont pas pu être appliquées, donc rien n'a été restauré. | Les détails affichent le message de zfs. |
| `set-limits-failed` | Les fichiers ont été restaurés, mais le quota ou la réservation enregistré n'a pas pu être appliqué. | Les détails montrent le message de zfs. Appliquez vous-même le quota ou la réservation avec `zfs set`. |

### Vérifier ce que voit le conteneur {#mountinfo}

**Tester l'accès aux instantanés** dans l'onglet Paramètres de l'élément prend un vrai instantané de son arbre, le cherche à l'intérieur de BombVault pour chaque jeu de données, puis le supprime. C'est le moyen le plus rapide de prouver tout le chemin avant la première exécution planifiée.

Pour regarder vous-même, lancez ceci sur le serveur :

```
docker exec BombVault grep zfs /proc/self/mountinfo
```

Chaque ligne est un montage dans le conteneur. La ligne d'un jeu de données montre son chemin dans le conteneur (sous `/host/user`) et le nom du jeu de données. Un champ `master:N` sur cette ligne signifie que le montage reçoit les montages que l'hôte fait plus tard, ce dont l'accès aux instantanés a besoin. S'il manque, réglez l'Access Mode de Host Data sur Read/Write - Slave et redémarrez BombVault.

## TrueNAS SCALE {#truenas}

- Quand `LIBVIRT_URI` est défini (comme pour les sauvegardes de VM sous TrueNAS), BombVault prend dans l'URI l'hôte, l'utilisateur et le port SSH de ses commandes zfs, chacun de ceux qui ne sont pas définis séparément. Sans sauvegardes de VM, rien n'est à définir : `host.docker.internal` ne se résout pas dans une application TrueNAS, BombVault atteint donc l'hôte via la passerelle par défaut de l'application, en tant que root sur le port 22. Définissez `LIBVIRT_HOST`, `LIBVIRT_SSH_USER` et `LIBVIRT_SSH_PORT` si vous voulez une autre adresse ou un autre utilisateur, dans l'`environment` de l'application.
- Ajoutez la clé SSH de l'hôte de **Paramètres, Intégrations, SSH de l'hôte** à root sous **Credentials, Users**, et activez le service SSH sous **System, Services**. Un [réplica](#replica-push) vers ce TrueNAS ou un autre a aussi besoin de la clé du réplica, qui est différente.
- Un utilisateur autre que root a besoin d'une permission sur le jeu de données supérieur de l'élément, qui couvre alors tous les jeux de données en dessous :

  ```
  zfs allow <user> snapshot,destroy,mount <dataset>
  ```

  Une restauration dans un nouveau jeu de données demande en plus `create` sur le jeu de données parent, et appliquer des propriétés enregistrées demande les droits sur ces propriétés.

  Une session SSH non root sous TrueNAS n'a pas `/usr/sbin` dans son chemin ; BombVault appelle alors `/usr/sbin/zfs` directement.
- Le **Host Data** de l'application doit être un chemin d'hôte situé au-dessus des jeux de données, par exemple `/mnt/tank`, et non un ixVolume. Avec un chemin d'hôte, l'application transmet à BombVault les nouveaux montages de l'hôte (`rslave`), ce dont l'accès aux instantanés a besoin. Voir [TrueNAS SCALE](getting-started.md#truenas) pour toute la configuration de l'application.
