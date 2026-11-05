# Jeux de données ZFS

Les jeux de données ZFS sont des éléments de la liste **Sauvegardes**, comme les conteneurs et les VMs. Un élément est un jeu de données avec tous ceux qu'il contient. Pour chaque sauvegarde, BombVault prend un instantané ZFS de toute l'arborescence, si bien que chaque jeu de données est capturé au même instant. Il lit ensuite les fichiers de chaque jeu de données depuis cet instantané et les stocke avec restic, comme il stocke un dossier, puis supprime l'instantané aussitôt. Les sauvegardes sont dédupliquées, vous pouvez parcourir chacune d'elles, et des fichiers isolés peuvent être restaurés.

La sauvegarde n'envoie jamais de flux dans restic et ne ramène jamais un jeu de données en arrière. BombVault ne détruit que les instantanés qu'il a créés lui-même. La [réplique](#replica) facultative est le seul endroit qui utilise `zfs send` : elle copie les jeux de données vers un second serveur ZFS et ne touche pas à la sauvegarde.

## Prérequis {#requirements}

- **Le lien SSH vers ce serveur.** Les jeux de données ZFS utilisent la même clé, le même hôte et le même utilisateur que les sauvegardes de VM. Si les sauvegardes de VM fonctionnent déjà, celles-ci aussi. Sinon, suivez le [guide de la sauvegarde de VM via SSH](vm-backup-ssh-setup.md). Les champs du modèle s'appellent **Host SSH: Address**, **Host SSH: Port** et **Host SSH: User**.
- **La commande `zfs` sur cet hôte.** Unraid 6.12 et plus récent et TrueNAS SCALE l'ont.
- **Host Data associé à `/mnt` avec le mode d'accès Read/Write - Slave.** C'est la valeur par défaut du modèle. L'instantané d'un jeu de données n'apparaît dans le dossier `.zfs/snapshot` du jeu de données qu'après le démarrage de BombVault, le conteneur doit donc recevoir les montages que l'hôte fait plus tard.
- **Les jeux de données montés sous `/mnt`.** Sur Unraid, les pools vivent sous `/mnt/<pool>`, c'est donc déjà le cas.

Activez la section sous **Paramètres, Général** (Jeux de données ZFS). **Connexion ZFS à ce serveur**, sous **Plus** sur la page Sauvegardes, ouvre alors **Connexion à ce serveur**. Elle teste le lien SSH, nomme l'utilisateur et l'hôte auxquels elle se connecte, et dit ce qui manque quand quelque chose manque. Quand le lien est coupé, la liste Sauvegardes le dit avec **Voir la connexion**, et la carte **Intégration hôte** de l'aperçu montre le même résultat.

## Éléments et jeux de données enfants {#items-and-children}

Choisissez **Ajouter** sur la page Sauvegardes, puis **Un jeu de données ZFS**. La liste vient du serveur. Choisissez le jeu de données tout en haut de ce que vous voulez sauvegarder, par exemple `cache/appdata`, et l'élément le couvre avec tous ceux qu'il contient.

- **Les nouveaux jeux de données enfants se joignent tout seuls.** Un jeu de données créé plus tard sous l'élément est sauvegardé à l'exécution suivante, et cette exécution le nomme comme nouveau. Sa première sauvegarde le lit en entier une fois. Ensuite, seuls les changements sont lus.
- **Vous pouvez laisser de côté des enfants isolés.** Désactivez un enfant sur l'onglet Paramètres de l'élément et il est laissé de côté avec tout ce qu'il contient. Un enfant laissé de côté qui n'existe plus sur le serveur est marqué comme tel et peut être retiré de la liste.
- **Les enfants illisibles sont ignorés, jamais en silence.** L'exécution les liste, l'élément montre combien ont été ignorés, et la carte **Non sauvegardé** de l'aperçu compte chacun comme non protégé. L'exécution sauvegarde quand même tout le reste et n'échoue pas à cause d'un enfant ignoré. Les raisons figurent dans le [tableau des codes de raison](#reason-codes) : un jeu de données non monté, avec `canmount=off`, un point de montage `legacy` ou absent, une clé de chiffrement non chargée, l'accès aux instantanés désactivé, ou un point de montage que BombVault ne voit pas.
- **Un jeu de données ignoré n'emporte pas ses enfants.** Un jeu de données `canmount=off` qui ne fait que contenir d'autres jeux de données est ignoré (affiché comme « structure only ») et ses enfants montés sont sauvegardés. Un jeu de données chiffré dont la clé n'est pas chargée est ignoré avec les enfants qui partagent sa clé.
- **Les enfants qui sont des disques de VM ou des données système démarrent désactivés** dans la boîte de dialogue d'ajout, avec la raison à côté de l'interrupteur. Ajouter un pool entier demande une confirmation qui liste ce qu'il contient.

### Volumes {#volumes}

Un volume (zvol) contient un disque virtuel au lieu de fichiers, et un élément ZFS n'en sauvegarde jamais.

- Un volume qu'utilise une VM est sauvegardé avec cette VM.
- Un volume qu'aucune VM n'utilise (une extension iSCSI, un disque que vous avez détaché) n'est **pas sauvegardé par BombVault**. La fenêtre d'ajout et la page Sauvegardes comptent ces volumes et le disent. Une version ultérieure les sauvegardera.

Les volumes dans l'arborescence d'un élément sont ignorés et nommés à chaque exécution.

### Le stockage de Docker {#docker-storage}

Avec le pilote de stockage ZFS de Docker, chaque couche d'image est un jeu de données avec un point de montage `legacy`. La boîte de dialogue d'ajout les regroupe en une ligne par parent. Une arborescence qui contient plus de 20 de ces jeux de données ne peut pas devenir un élément : tant qu'un instantané existe, Docker ne peut pas supprimer de couches d'image. Ajoutez plutôt les jeux de données en dessous, par exemple `appdata`.

### Les éléments ne se chevauchent jamais {#overlap}

Un jeu de données ne peut appartenir qu'à un seul élément. BombVault refuse un nouvel élément qui se trouve dans un élément existant, ou qui en contiendrait un. Pour fusionner plusieurs éléments enfants en un élément parent, supprimez d'abord les éléments enfants en choisissant de garder leurs sauvegardes, puis ajoutez le parent. Chaque jeu de données garde son historique sous son propre nom, si bien que la sauvegarde suivante reprend là où les anciens éléments s'étaient arrêtés et ne relit pas tout.

## Arrêter des conteneurs et exécuter des commandes autour de l'instantané {#consistency}

Un instantané d'une base de données en marche ressemble à une coupure de courant soudaine : la base se rétablit en général, mais il faut qu'elle le fasse. Chaque élément peut faire deux choses à ce sujet, et toutes deux ne couvrent que l'instant de l'instantané, pas toute la sauvegarde.

- **Arrêter ces conteneurs pour l'instantané.** BombVault arrête les conteneurs listés, prend l'instantané et les redémarre aussitôt. Les conteneurs d'un même niveau de dépendances s'arrêtent en parallèle, les dépendants d'abord, si bien que la fenêtre entière dure en général quelques secondes. L'exécution montre sa durée. La sauvegarde lit ensuite l'instantané figé alors que les applications ont déjà redémarré. Seuls les conteneurs qui tournaient sont arrêtés.
- **Une commande avant et après l'instantané.** Elle s'exécute dans un conteneur de votre choix, par exemple pour exporter une base de données dans le jeu de données juste avant l'instantané, sans rien arrêter. Si la commande avant l'instantané échoue, la sauvegarde échoue et aucun instantané n'est pris. Une commande qui échoue après l'instantané est montrée sur l'exécution mais ne fait pas échouer la sauvegarde.

Ce qui se passe quand quelque chose tourne mal :

- Si un conteneur ne peut pas être arrêté, BombVault redémarre ceux qu'il a déjà arrêtés et la sauvegarde échoue en nommant le conteneur. Il ne se rabat jamais sur un instantané d'applications en marche.
- L'arrêt attend la fin d'une sauvegarde de conteneur en cours (jusqu'à 30 minutes pour une exécution manuelle, jusqu'à la limite de temps de sauvegarde pour une exécution planifiée), si bien que les deux n'arrêtent et ne démarrent jamais le même conteneur en même temps.
- Avant l'arrêt du premier conteneur, BombVault note lesquels il arrête. Si BombVault est tué dans la fenêtre, il redémarre ces conteneurs au prochain démarrage, envoie une notification, et l'élément affiche une note rouge pour tout conteneur qu'il n'a pas pu démarrer.

Les [dumps de base de données](backups.md#database-dumps) automatiques s'exécutent avec la sauvegarde propre d'un conteneur, pas avec un élément ZFS. Une base de données dont le conteneur n'est sauvegardé que par son jeu de données n'a pas de dump, donnez-lui donc une commande ici.

Un conteneur peut figurer sur cette liste et être sauvegardé comme conteneur en même temps. Ses données sont alors stockées deux fois, dans deux dépôts, et la sauvegarde complète l'arrête deux fois. L'élément le dit.

## Restaurer {#restore}

Ouvrez l'élément et choisissez **Restaurer**, ou choisissez-le sur la page **Restauration**. La fenêtre de restauration demande la sauvegarde, puis le **Jeu de données**. Par défaut, c'est le jeu de données du haut de l'élément. Sous **Quoi**, vous restaurez tout ou seulement les fichiers et dossiers que vous choisissez. **Où** propose :

- **Restaurer dans le jeu de données.** Les fichiers de la sauvegarde sont écrits dans le point de montage du jeu de données. Les fichiers de même nom sont écrasés, les autres restent. Le jeu de données n'est jamais ramené en arrière ni remplacé. BombVault vérifie que le jeu de données est monté, visible et inscriptible, une fois avant de démarrer et encore juste avant d'écrire. Là où un jeu de données enfant est monté à l'intérieur, rien n'est écrit : l'enfant garde ses fichiers, son propriétaire et ses permissions, et se restaure depuis sa propre sauvegarde.
- **Dans un dossier.** Choisissez un dossier sous `/mnt`. BombVault vérifie que le dossier se trouve sur un pool ou un partage monté et qu'il y a assez de place libre. Cela fonctionne sans le lien SSH et pour des jeux de données qui n'existent plus.
- **Dans un nouveau jeu de données.** Nommez un jeu de données qui n'existe pas encore. La fenêtre suggère `<dataset>-restored`. BombVault le crée avec les propriétés ZFS stockées dans la sauvegarde et restaure dedans, voir [Restaurer comme nouveau jeu de données](#new-dataset).

Choisir **Tous les jeux de données de cette sauvegarde** comme jeu de données écrit chaque jeu de données de l'arborescence dans son propre sous-dossier du dossier que vous choisissez. Les jeux de données ignorés dans cette sauvegarde sont nommés. Pour restaurer depuis le dépôt d'un autre BombVault, utilisez **Depuis un ancien dossier de sauvegarde** sur la page Restauration. Elle restaure toujours dans un dossier : tous les jeux de données d'une sauvegarde, chacun dans son propre sous-dossier, ou un jeu de données de l'arborescence, en entier ou des fichiers choisis.

La liste des conteneurs à arrêter de l'élément est aussi proposée pour une restauration dans le jeu de données. Ces conteneurs restent arrêtés pendant toute la restauration, et les sauvegardes de conteneurs attendent entre-temps.

### L'instantané de sécurité {#safety-snapshot}

Avant d'écrire dans un jeu de données, BombVault prend un instantané ZFS de ce seul jeu de données, nommé `bombvault-prerestore-<time>`. **Garder d'abord un instantané ZFS de l'état actuel** est activé par défaut. Le désactiver demande une seconde confirmation. Si l'instantané ne peut pas être pris, rien n'est restauré.

BombVault ne supprime jamais un instantané de sécurité de lui-même. L'élément les liste avec leur âge et leur taille, chacun avec une action **Supprimer**, et prévient quand le plus ancien a plus de 30 jours, car il garde des données supprimées et modifiées sur le pool.

Pour revenir en arrière après une restauration, copiez des fichiers isolés depuis `.zfs/snapshot/bombvault-prerestore-<time>` dans le jeu de données. `zfs rollback <dataset>@bombvault-prerestore-<time>` ne fonctionne que tant qu'il est l'instantané le plus récent de ce jeu de données. `zfs rollback -r` supprime tous les instantanés plus récents, automatiques compris.

### Restaurer comme nouveau jeu de données {#new-dataset}

BombVault stocke avec chaque sauvegarde les propriétés ZFS définies localement pour chaque jeu de données : compression, taille d'enregistrement, quota, réservation, atime, xattr, acltype, sensibilité à la casse et vos propres propriétés utilisateur. Les valeurs héritées et en lecture seule sont omises, car elles reviennent d'elles-mêmes. Les sauvegardes d'avant que BombVault les stocke n'en ont pas.

- **Dans un nouveau jeu de données** exécute `zfs create` avec chaque propriété stockée. La sensibilité à la casse, la normalisation et utf8only ne peuvent se régler que de cette façon. Les quotas et les réservations sont réglés après les fichiers, ils ne peuvent donc pas les refuser. Le point de montage est omis, pour que la copie n'entre pas en collision avec l'original, ainsi que `canmount`, `readonly` et les réglages de chiffrement, pour que la restauration puisse écrire. Un nouveau jeu de données sous un jeu de données chiffré reprend son chiffrement. Le jeu de données au-dessus du nouveau doit exister. Si quelque chose échoue après la création du nouveau jeu de données, il reste sur le serveur, car BombVault ne détruit jamais un jeu de données.
- **Restaurer dans le jeu de données** liste les propriétés stockées à côté de la restauration. **Appliquer aussi ces propriétés** règle celles qu'un jeu de données existant peut encore accepter, avant l'écriture de tout fichier. Les quotas et les réservations sont réglés après les fichiers, ils ne peuvent donc pas les refuser. Sans cet interrupteur, le jeu de données garde ses réglages.

## Ce que contient la sauvegarde {#contents}

Dans la sauvegarde : les fichiers et dossiers de chaque jeu de données sauvegardé, avec leurs propriétaires, modes, horodatages et attributs étendus tels que restic les stocke, et les propriétés ZFS définies localement pour chaque jeu de données.

Pas dans la sauvegarde :

- le propriétaire et le mode du dossier racine de chaque jeu de données lui-même (tout ce qu'il contient est inclus). Une restauration dans le jeu de données laisse le dossier racine existant tel quel, une restauration vers un dossier le crée avec le mode `0755`,
- les instantanés ZFS existants,
- les enfants qui ont été ignorés ou laissés de côté,
- les volumes.

Pour restaurer sur un nouveau pool, créez le pool et restaurez chaque jeu de données dans un nouveau jeu de données. Savoir si les ACL NFSv4, telles que les utilise TrueNAS sur les jeux de données SMB, reviennent comme vous l'attendez n'a pas encore été vérifié, contrôlez donc une restauration sur vos propres données avant de compter dessus.

## Jeux de données chiffrés {#encryption}

Un jeu de données chiffré n'est sauvegardé que tant que sa clé est chargée. Sinon, il est ignoré avec un avertissement. Chargez la clé avec `zfs load-key` et montez le jeu de données. BombVault lit les données déchiffrées et les stocke dans le dépôt de restic, qui est chiffré. Si vous avez désactivé le chiffrement dans BombVault, ce dépôt ne l'est pas.

## Réplique {#replica}

Une réplique est une copie des jeux de données d'un élément sur un second serveur ZFS. BombVault la tient à jour avec `zfs send` et `zfs receive`. La première exécution envoie tout, ensuite seuls les blocs modifiés voyagent. Sur l'autre serveur, vous pouvez monter la copie tout de suite.

Une réplique ne remplace jamais la sauvegarde. Les versions plus anciennes, les fichiers isolés et le contrôle viennent toujours des sauvegardes, et la réplique ne garde que le nombre d'instantanés que vous réglez. Une réplique à jour compte comme une copie hors des locaux, mais pas comme une sauvegarde.

La réplique n'attend pas la sauvegarde. **Après chaque sauvegarde** veut dire après l'exécution de sauvegarde de l'élément, que cette exécution ait réussi, échoué ou n'ait rien trouvé à faire. Quand la section ZFS n'a aucune destination activée et que l'élément n'a pas de dépôt à lui, une exécution de sauvegarde de l'élément n'envoie que la réplique, et la carte Réplique le dit dans la bulle à côté de **Quand**. La liste Sauvegardes, l'en-tête de l'élément et l'historique des exécutions montrent la réplique : **Réplication en cours** pendant qu'elle tourne, et **Répliqué** avec l'heure une fois faite.

**Réplique seule, sans sauvegarde**, sur la carte Réplique, fait envoyer des répliques à toute la section ZFS et ne fait prendre aucune sauvegarde. Le dossier propre et les destinations hors site gardent leurs interrupteurs, le désactiver sauvegarde donc de nouveau vers eux, et la carte de chaque élément ZFS montre l'interrupteur de la section tel qu'il est. Un tel élément attend sa première réplique au lieu d'une sauvegarde, **Sauvegarder maintenant** est proposé comme **Répliquer maintenant**, et les conteneurs de l'élément s'arrêtent et ses commandes s'exécutent autour de l'instantané comme pour une sauvegarde. Ses répliques comptent pour un planning tous les quelques jours, l'alerte de retard et la couverture.

Activez-la dans la carte **Réplique** de l'onglet Paramètres de l'élément. La carte liste chaque jeu de données et chaque volume avec son état, et **Répliquer maintenant** démarre une exécution. Une exécution de réplique a son propre verrou, si bien qu'un long premier transfert ne retient jamais les sauvegardes.

| Champ | Ce qu'il règle |
|---|---|
| **Réplique vers** | La cible : un serveur ZFS (marqué **via SSH**) ou une **instance appairée**. **Ajouter un serveur ZFS** en configure un nouveau. |
| **Quand** | **Après chaque sauvegarde**, c'est-à-dire après l'exécution de sauvegarde de l'élément, ou **Planning propre**. |
| **Conserver sur la cible** | Combien d'instantanés restent sur le serveur ZFS : **Court**, **Équilibré**, **Long** ou **Valeurs personnalisées**. Une instance appairée en décide elle-même. |
| **Conserver sur ce serveur** | Combien des instantanés de réplique les plus récents restent aussi sur ce serveur. 0 ou 1 ne garde que le dernier. |
| **Réplique seule, sans sauvegarde** | Toute la section ZFS envoie des répliques et ne prend aucune sauvegarde. |

![La carte Réplique d'un élément ZFS.](assets/screenshots/zfs/replica-dark.png#only-dark)
![La carte Réplique d'un élément ZFS.](assets/screenshots/zfs/replica-light.png#only-light)

**Tout répliquer maintenant**, à côté des actions de sauvegarde quand la tuile ZFS est choisie sur la page Sauvegardes, envoie la réplique de chaque élément ZFS, l'un après l'autre. Le serveur exécute le lot, il continue donc quand vous quittez la page, et un élément qui ne peut pas répliquer est nommé et ignoré.

### Envoyer vers un serveur ZFS {#replica-push}

Toute machine avec ZFS et SSH peut recevoir, par exemple un second Unraid ou un TrueNAS. BombVault n'a pas besoin d'y tourner.

1. Sous **Paramètres, Appairage**, choisissez **Connecter via SSH** dans **Serveur ZFS sans BombVault**, ou **Ajouter un serveur ZFS** dans la carte **Réplique** de l'élément.
2. Saisissez l'adresse, l'utilisateur et le port. La boîte de dialogue montre la clé publique de BombVault. Ajoutez-la au `~/.ssh/authorized_keys` de l'utilisateur sur le serveur. Sur Unraid, c'est sous **Users, root, SSH keys**. Selon **Display Settings**, Users se trouve dans le menu d'en-tête ou sous Settings. Sur TrueNAS, modifiez l'utilisateur sous **Credentials, Users**. C'est la clé de la réplique. Elle diffère de la clé du lien SSH des VMs et de l'hôte, ajoutez-la donc même si celle-là est déjà autorisée.
3. Testez la connexion. La boîte de dialogue liste alors les pools du serveur. Choisissez-en un et réglez la racine, qui vaut par défaut `<pool>/bombvault-replica`.
4. Choisissez le nouveau serveur dans la carte **Réplique** de l'élément. Le serveur a alors une carte à lui sur la page Instances, et **Ouvrir** mène à sa page.

Avec root, rien d'autre n'est nécessaire. Un utilisateur à part a besoin de ces permissions sur le pool de la cible, que la boîte de dialogue montre aussi :

```
zfs allow <user> receive,create,mount,rollback,destroy,userprop,readonly,canmount <pool>
```

Sur la source, le même genre d'utilisateur a besoin de celles-ci sur le jeu de données du haut de l'élément :

```
zfs allow <user> send,snapshot,hold,release,bookmark,destroy,mount <dataset>
```

Dans ce sens, le BombVault qui détient l'élément détient aussi une clé qui peut écrire sur le serveur.

BombVault épingle la clé d'hôte que le serveur montre à la première connexion et en refuse une différente plus tard. Si le serveur a été réinstallé, appuyez sur **Oublier la clé d'hôte** sur la page du serveur, et la connexion suivante épingle la nouvelle clé.

Quand l'hôte de ce serveur peut joindre lui-même le serveur ZFS, il envoie `zfs send` directement dans son propre ssh vers le serveur, avec la clé de la réplique et la clé d'hôte épinglée, si bien que le flux ne passe pas par BombVault. Sinon, le flux passe par BombVault.

### Envoyer vers une instance appairée {#replica-receive}

Un BombVault appairé peut recevoir lui-même la réplique. Personne n'obtient d'accès SSH à l'autre hôte, et aucune clé n'entre dans un fichier `authorized_keys`.

1. Dans la carte **Réplique** de l'élément, choisissez l'instance appairée comme cible, ou activez le rôle **Serveur ZFS** sur la carte de cette instance dans la page Instances et choisissez-y les éléments ZFS. La carte affiche **En attente d'approbation** jusqu'à ce qu'elle réponde.
2. Sur l'instance réceptrice, ouvrez **Instances**. La demande apparaît sur la carte de l'instance émettrice sous forme de badge, et ses **Détails** la listent. Choisissez le pool et la racine sur ce serveur et le nombre d'instantanés qui restent, puis appuyez sur **Autoriser** ou **Refuser**.
3. Après **Autoriser**, la source envoie selon son propre planning, comme pour toute autre cible.

L'instance réceptrice n'accepte que ce que couvre l'approbation : les jeux de données de l'élément, dans sa propre racine. Elle exécute elle-même `zfs receive`, et la source n'a aucun moyen de supprimer ou de ramener quoi que ce soit là-bas en arrière. La copie survit donc à une source dont quelqu'un a pris le contrôle. L'instance réceptrice garde sa propre rétention. La source ne fait que suggérer une règle quand elle demande. Une telle source peut tout de même envoyer de nouveaux instantanés, et avec le temps ils font sortir les plus anciens selon la règle, comme tous les autres. Un nom d'instantané daté en avance sur l'horloge de l'instance réceptrice ne compte pas pour la règle tant que cette heure n'est pas arrivée.

**Révoquer l'accès** sur l'instance réceptrice met fin à l'approbation à tout moment et le dit à la source, qui montre alors que l'approbation a été retirée et s'arrête. Ce que l'instance réceptrice détient déjà y reste. Une demande refusée reste refusée. D'autres jeux de données, ou une demande après une révocation, redemandent.

L'instance réceptrice doit répondre en HTTPS. La source n'envoie rien à une adresse http simple, puisque le jeton et les données traverseraient le réseau non chiffrés. Chaque transfert laisse aussi un dixième du pool récepteur libre et s'arrête avant d'y entamer, si bien qu'une source ne peut pas remplir le pool du serveur récepteur.

### Où atterrissent les données {#replica-target}

Chaque jeu de données atterrit à `<root>/<server>/<pool>/<path>`. Le dossier du serveur est le nom de l'instance source, figé au premier transfert, si bien que deux serveurs qui ont le même nom de pool ne se gênent jamais. Par exemple, `cache/appdata` d'un serveur appelé `tower` se retrouve à `backup/bombvault-replica/tower/cache/appdata`.

Après une nouvelle installation de BombVault, la cible refuse le dossier que ce BombVault avait rempli avant. La carte Réplique propose alors **Reprendre le dossier**, et la réplique suivante continue à partir des copies qu'il contient. Ne le reprenez que si aucun autre BombVault ne réplique encore dans ce dossier.

La copie sur la cible est en lecture seule. Par défaut, elle n'est pas montée, si bien qu'elle ne recouvre jamais rien sur ce serveur. Un serveur ZFS connecté via SSH a sur sa page un interrupteur **Monter en lecture seule sur le serveur** : activé, BombVault monte les copies là-bas en lecture seule après chaque exécution, pour que vous puissiez lire et copier leurs fichiers sur la cible. Les copies chiffrées restent verrouillées jusqu'au chargement de leur clé, et le désactiver ne démonte rien. Une instance appairée qui reçoit la réplique la garde non montée. Les propriétés ZFS voyagent avec, sauf le point de montage et `sharenfs` et `sharesmb`. Les réservations `reservation` et `refreservation` restent aussi derrière, si bien que la copie ne prend que la place dont ses données ont besoin.

### Ce qui entre {#replica-contents}

Tout ce que l'élément sauvegarde entre, et les volumes en dessous aussi, que la sauvegarde ignore. Un jeu de données enfant que vous avez désactivé dans l'élément reste dehors. Tous les jeux de données d'une exécution viennent d'un seul instantané, comme dans la sauvegarde.

Une copie qu'un autre BombVault réplique vers ce serveur n'est pas transmise plus loin, car cela réduirait l'historique dont sa source a encore besoin. Quand l'arborescence d'un élément en contient une, l'exécution échoue pour ce jeu de données et dit pourquoi, jusqu'à ce que vous le désactiviez dans l'élément. Un élément qui est lui-même une telle copie ne peut pas répliquer du tout. Supprimer un élément ou désactiver sa réplique laisse ces copies et leurs instantanés tranquilles. Une fois que vous reprenez une copie, comme décrit sous [Utiliser la réplique](#replica-use), elle compte comme un jeu de données normal.

### Combien de temps les instantanés restent {#replica-retention}

Sur la cible, une nouvelle réplique garde 7 instantanés quotidiens et 3 hebdomadaires. Choisissez plutôt **Court**, **Équilibré** ou **Long**, ou réglez des **Valeurs personnalisées**. Des valeurs personnalisées toutes à zéro gardent chaque instantané, et la carte le dit. Seuls les instantanés nommés `bombvault-replica-<14 digits>` y sont jamais supprimés, et jamais le plus récent que les deux côtés partagent.

Sur la source, BombVault garde le dernier instantané de réplique, plus un bookmark pour chaque état qu'il a envoyé. Les bookmarks ne coûtent pas de place. Le transfert suivant part d'eux.

Pour en garder plus aussi sur la source, réglez **Conserver sur ce serveur** sur la carte Réplique, par exemple sur 7. Autant des instantanés de réplique les plus récents restent alors sur ce serveur après chaque réplique, à côté de ce que garde la cible. 0 ou 1 ne garde que le dernier. Le dernier porte un hold, pour que la base du transfert suivant ne soit pas détruite par accident. Les plus anciens n'en ont pas. Chaque instantané retient les blocs qui ont changé après lui, si bien que chacun que vous gardez prend de la place sur le pool. Désactiver la réplique ou la passer à une autre cible, ou supprimer l'élément, les supprime aussi de ce serveur. Les instantanés d'un autre élément imbriqué en dessous restent. Si l'hôte ne peut pas être joint pendant que vous supprimez l'élément, les instantanés restent aussi et la page le dit. Supprimez-les à la main : `zfs release -r bombvault-replica <dataset>@<snapshot>`, puis `zfs destroy`.

Les fichiers d'un instantané conservé peuvent être lus ou copiés tout de suite, depuis le dossier `.zfs/snapshot` du jeu de données ou depuis un clone, sans toucher au jeu de données ni à la réplique :

```
ls /mnt/cache/appdata/.zfs/snapshot/bombvault-replica-20261006014100
zfs clone cache/appdata@bombvault-replica-20261006014100 cache/appdata-20261006
```

Un clone ne prend aucune place tant que rien n'y change. Détruisez-le avec `zfs destroy cache/appdata-20261006` quand vous avez fini, car un instantané qui a un clone ne peut pas être supprimé.

Revenir au dernier instantané de réplique n'exige que `zfs rollback cache/appdata@<latest>` ici, et rien sur la cible. Ramener tout le jeu de données à un plus ancien détruit chaque instantané qui le suit, sur ce serveur et sur la cible. Le dernier est retenu, levez donc d'abord son hold, puis ramenez le jeu de données en arrière ici :

```
zfs release -r bombvault-replica cache/appdata@bombvault-replica-20261008014200
zfs rollback -r cache/appdata@bombvault-replica-20261006014100
```

La copie sur la cible doit revenir en arrière aussi, mais pas toujours vers le même instantané, car ses propres règles de conservation peuvent avoir supprimé celui-là. Lancez la réplique une fois. Elle s'arrête pour ce jeu de données parce que la cible contient des instantanés plus récents, et la carte Réplique et l'historique des exécutions nomment l'instantané auquel ramener la cible : le plus récent que les deux côtés partagent encore. Ramenez la copie à celui-ci sur le serveur ZFS, ou sur l'hôte de l'instance appairée qui reçoit la réplique, par exemple :

```
zfs rollback -r backup/bombvault-replica/tower/cache/appdata@bombvault-replica-20261005014000
```

`zfs rollback` agit sur un seul jeu de données, répétez donc les retours en arrière pour chaque jeu de données enfant que vous voulez aussi récupérer. Ensuite, **Répliquer maintenant** continue à partir de l'instantané partagé.

### Jeux de données chiffrés dans une réplique {#replica-encryption}

Un jeu de données chiffré est envoyé brut. Il reste chiffré sur la cible, et la cible ne voit jamais la clé. Gardez la clé en lieu sûr : vous en avez besoin pour ouvrir la copie après une restauration, et une réplique sans elle est illisible.

### Utiliser la réplique {#replica-use}

Ouvrez l'onglet **Sauvegardes** de l'élément et cliquez sur la ligne **Réplique ZFS** parmi les endroits qui contiennent ses instantanés. La feuille liste les instantanés de la cible et montre les commandes avec vos vrais noms.

Pour regarder un ancien état, clonez un instantané sur la cible. Un clone ne prend aucune place tant que rien ne change, et la réplique reste intacte :

```
zfs clone backup/bombvault-replica/tower/cache/appdata@bombvault-replica-20261006014100 backup/bombvault-replica/clone-appdata
```

Si la source tombe en panne, transformez la copie en jeu de données normal et inscriptible sur la cible :

```
zfs inherit -r readonly backup/bombvault-replica/tower/cache/appdata && zfs list -rH -o name -t filesystem backup/bombvault-replica/tower/cache/appdata | xargs zfs set canmount=on && zfs mount -a
```

BombVault cesse ensuite de répliquer vers ce jeu de données et ses enfants. L'exécution suivante échoue avec une raison, et ce que vous avez écrit reste. Il en va de même pour tout ce qui a été écrit d'autre dans la copie depuis son instantané le plus récent : BombVault ne ramène jamais une copie en arrière. Pour y répliquer de nouveau, déplacez ce dont vous avez besoin hors de la copie et détruisez-la sur la cible. L'exécution suivante renvoie alors tout depuis le début.

Pour récupérer un état vers la source, appuyez sur **Restaurer comme nouveau jeu de données** dans la feuille. BombVault envoie l'instantané dans un nouveau jeu de données à côté de l'original, nommé `<dataset>-bombvault-restore-` plus un horodatage. Il n'écrit jamais par-dessus l'original. Un jeu de données enfant perdu sur la source revient aussi, tant que la copie contient encore cet instantané. Un enfant qui ne le contient pas est laissé de côté, et l'exécution le dit.

### Exemple : une réplique vers un second serveur Unraid {#example-replica}

Le serveur `tower` sauvegarde le jeu de données `cache/appdata` comme élément ZFS. Un second serveur Unraid à `192.168.1.20`, sans BombVault, a un pool appelé `backup` et doit contenir une copie que vous pouvez lire à tout moment.

1. Ouvrez **Sauvegardes**, puis `cache/appdata`, puis son onglet **Paramètres**. Dans la carte **Réplique**, choisissez **Ajouter un serveur ZFS** sous **Réplique vers**.
2. Saisissez `192.168.1.20`, l'utilisateur `root` et le port `22`. La boîte de dialogue montre la clé de la réplique. Sur le second serveur, collez-la sous **Users, root, SSH keys** et appuyez sur **Apply**. Cette clé diffère de la clé SSH de l'hôte, ajoutez-la donc même si celle-là est déjà là.
3. Testez la connexion. Choisissez le pool `backup` et gardez la racine qu'il suggère, `backup/bombvault-replica`.
4. De retour sur la carte, le nouveau serveur est choisi sous **Réplique vers**. Laissez **Quand** sur **Après chaque sauvegarde**, laissez **Conserver sur la cible** à ses 7 instantanés quotidiens et 3 hebdomadaires, et réglez **Conserver sur ce serveur** sur `7`.
5. Appuyez sur **Répliquer maintenant**. La première exécution envoie tout, donc 300 GB sur une ligne de 1 Gbit/s prennent au moins 40 minutes. La carte et la liste Sauvegardes indiquent **Réplication en cours**, puis **Répliqué** avec l'heure. Les exécutions suivantes n'envoient que les blocs modifiés.

Les jeux de données se trouvent maintenant à `backup/bombvault-replica/tower/cache/appdata` sur le second serveur, en lecture seule. Sur l'aperçu, **Hors site** de la section ZFS compte la réplique comme une copie hors des locaux.

Pour récupérer un fichier de mardi dernier, vous avez trois voies :

- **Sur ce serveur**, car **Conserver sur ce serveur** garde sept instantanés :

    ```
    cp /mnt/cache/appdata/.zfs/snapshot/bombvault-replica-20261103014100/plex/Preferences.xml /mnt/user/restore/
    ```

- **Sur le second serveur**, avec un clone d'un instantané sur la cible. L'onglet **Sauvegardes** de l'élément montre les commandes avec vos vrais noms quand vous cliquez sur la ligne **Réplique ZFS**. Supprimez le clone avec `zfs destroy` quand vous avez fini.
- **Avec un montage**, en activant **Monter en lecture seule sur le serveur** sur la page du serveur ZFS. Après l'exécution suivante, les copies sont montées en lecture seule sur le second serveur.

Les versions plus anciennes que ce que garde la réplique viennent toujours des sauvegardes, comme le montre l'exemple suivant.

### Exemple : un fichier d'une sauvegarde ZFS {#example-zfs-file}

1. Ouvrez **Restauration**, choisissez la tuile **ZFS** et cliquez sur `cache/appdata`.
2. Choisissez la sauvegarde, et sous **Jeu de données** le jeu de données qui contient le fichier.
3. Sous **Quoi**, choisissez **Fichiers individuels** et cochez le fichier.
4. Sous **Où**, choisissez **Dans un dossier** et choisissez un dossier sous `/mnt`, comme `user/restore`. Cela ne demande aucun lien SSH et fonctionne aussi pour un jeu de données qui n'existe plus.
5. Appuyez sur **Restaurer**. Le fichier atterrit directement dans ce dossier, sans le chemin au-dessus.

## Instantanés restants {#leftover-snapshots}

L'instantané d'une sauvegarde est nommé `<dataset>@bombvault-<14 digits>`, par exemple `cache/appdata@bombvault-20260924021500` (UTC). BombVault le supprime juste après la sauvegarde. Si cela échoue, par exemple parce que le jeu de données est occupé ou que BombVault a été arrêté, BombVault le supprime :

- avant la sauvegarde suivante de cet élément,
- au démarrage de BombVault, pour chaque élément, même avec la section désactivée,
- quand vous supprimez l'élément,
- quand vous appuyez sur **Retirer maintenant** sur l'élément, qui montre combien il en reste.

Seuls les noms qui correspondent exactement à `bombvault-` plus 14 chiffres sont supprimés. Les instantanés de sécurité, vos propres instantanés et les instantanés automatiques ne sont jamais touchés. Pour en supprimer un à la main :

```
zfs destroy -r cache/appdata@bombvault-20260924021500
```

L'instantané d'une réplique est nommé `<dataset>@bombvault-replica-<14 digits>` et n'est pas un reste. Il reste sur la source jusqu'à ce qu'une exécution de réplique plus récente le remplace, ou aussi longtemps que **Conserver sur ce serveur** le garde, et sur la cible aussi longtemps que la rétention le garde. Le nettoyeur ne le touche jamais, car il ne retient que `bombvault-` suivi d'exactement 14 chiffres.

## Anomalies {#anomalies}

Un enfant vidé change à peine le total d'une grande arborescence, la détection d'anomalies surveille donc chaque jeu de données d'un élément séparément : sa taille, son nombre de fichiers, ses nouvelles données et sa durée restic ont chacun leur propre historique. Cet historique appartient au nom du jeu de données, il reste donc quand l'arborescence est sauvegardée plus tard par un autre élément.

Un jeu de données que l'exécution précédente a sauvegardé et que celle-ci n'a pas pu lire compte comme vidé, tant que la sélection de l'élément n'a pas changé. Cela couvre une clé non chargée, un jeu de données non monté et un jeu de données qui a disparu de l'arborescence. Un enfant que vous excluez vous-même change la sélection, son historique repart donc de zéro. Tant qu'une anomalie sur des données perdues est ouverte, la rétention garde les anciennes sauvegardes de ce seul jeu de données et élague le reste de l'arborescence comme d'habitude.

Chaque jeu de données a une ligne à lui dans la surveillance de l'élément, qui s'ouvre avec **Surveillance** sur la page **Anomalies** et se trouve sur l'onglet **Vérification** de l'élément. Les anomalies ouvertes s'affichent à côté de chaque jeu de données. Le lien d'une anomalie ouvre la fenêtre de restauration sur la dernière bonne sauvegarde du jeu de données. Le fait qu'une exécution se termine est jugé pour tout l'élément, car une exécution réussit ou échoue en bloc.

Les contrôles eux-mêmes sont décrits sous [Anomalies](anomalies.md). Un assistant connecté via le [serveur MCP](mcp.md) peut lister les points de restauration d'un élément ZFS, lancer sa sauvegarde et lire les anomalies, mais en accuser réception se fait sur la page **Anomalies**.

## Codes de raison {#reason-codes}

La page de l'élément, l'historique des exécutions et les notifications nomment un problème avec l'un de ces codes. La plupart ont aussi le remède à côté sur la page.

| Code | Signification | Que faire |
|---|---|---|
| `ssh-missing` | La connexion SSH n'est pas configurée dans ce conteneur. | Configurez le lien SSH comme pour les sauvegardes de VM. |
| `host-placeholder` | Host SSH: Address est encore l'espace réservé, et `host.docker.internal` n'a pas répondu non plus. | Réglez Host SSH: Address sur l'IP de ce serveur sur le réseau local. |
| `host-fallback` | Host SSH: Address est encore l'espace réservé, et `host.docker.internal` fonctionne. | Rien, ou réglez l'IP du réseau local. |
| `ssh-unreachable` | Le serveur est injoignable via SSH. | Vérifiez l'adresse et le port, et que SSH est activé. |
| `ssh-auth` | Le serveur a refusé la clé de BombVault. | Exécutez une fois sur le serveur la commande montrée sous **Connexion à ce serveur**. |
| `zfs-not-found` | L'hôte SSH n'a pas de commande `zfs`. | Faites pointer Host SSH: Address vers la machine qui possède les pools. |
| `zfs-permission` | L'utilisateur SSH n'a pas le droit d'exécuter cette commande zfs. | Utilisez root, ou voir [TrueNAS SCALE](#truenas). |
| `uri-mismatch` | `LIBVIRT_URI` nomme un autre hôte ou utilisateur que les champs SSH. | Faites-les concorder, ou videz les champs SSH pour que les deux viennent de l'URI. |
| `zfs-error` | zfs a signalé une autre erreur. | Les détails montrent son message. |
| `propagation-missing` | Les nouveaux montages de l'hôte n'atteignent pas le conteneur. | Réglez le mode d'accès de Host Data sur Read/Write - Slave et redémarrez BombVault. |
| `invalid-name` | Un nom de jeu de données que BombVault n'accepte pas. | Renommez le jeu de données. |
| `name-too-long` | Un jeu de données de l'arborescence est trop long pour un nom d'instantané. | Renommez-le, ou ajoutez comme élément un jeu de données en dessous. |
| `invalid-exclude` | Un motif d'exclusion ou un enfant laissé de côté ne convient pas à l'élément. | Corrigez l'élément que nomme le message. Pour laisser de côté tout un jeu de données enfant, désactivez-le au lieu d'écrire un motif. |
| `not-found` | Le jeu de données n'existe pas sur le serveur. | Supprimez l'élément, ou recréez le jeu de données. Ses sauvegardes restent restaurables. |
| `not-filesystem` | C'est un volume, pas un système de fichiers. | Voir [Volumes](#volumes). |
| `overlaps-item` | Le jeu de données chevauche un élément existant. | Voir [Les éléments ne se chevauchent jamais](#overlap). |
| `docker-storage` | L'arborescence contient le stockage d'images de Docker. | Voir [Le stockage de Docker](#docker-storage). |
| `nothing-readable` | Aucun jeu de données de l'élément ne peut être lu en ce moment. | Regardez les codes des jeux de données ignorés. |
| `snapshot-failed` | L'instantané n'a pas pu être créé. | Les détails montrent le message de zfs. |
| `containers-busy` | Une sauvegarde de conteneur tournait encore quand les conteneurs devaient s'arrêter. | Recommencez plus tard. Les exécutions planifiées attendent toutes seules. |
| `consistency-stop-failed` | Un conteneur n'a pas pu être arrêté, donc aucun instantané n'a été pris. | Vérifiez le conteneur, ou retirez-le de la liste. |
| `pre-snapshot-failed` | La commande avant l'instantané a échoué. | Les détails de l'exécution montrent sa sortie. |
| `container-unknown` | Un conteneur listé n'existe pas. | Retirez-le de la liste. |
| `container-is-self` | BombVault ne peut pas arrêter son propre conteneur. | Retirez-le de la liste. |
| `leftover-snapshots` | Des instantanés que BombVault n'a pas pu supprimer sont encore sur le serveur. | Appuyez sur **Retirer maintenant**, voir [Instantanés restants](#leftover-snapshots). |
| `zvol` | Un volume dans l'arborescence, ignoré. | Voir [Volumes](#volumes). |
| `canmount-off` | Jamais monté (`canmount=off`), ignoré. | S'il contient des données, montez-le ou déplacez les données dans un jeu de données enfant. |
| `legacy-mount` | Point de montage legacy, ignoré. | Donnez-lui un point de montage sous `/mnt`. |
| `no-mountpoint` | Pas de point de montage, ignoré. | Donnez-lui un point de montage sous `/mnt`. |
| `not-mounted` | Non monté sur le serveur, ignoré. | Faites-lui un `zfs mount`, ou réglez `canmount=on`. |
| `key-not-loaded` | Chiffré et la clé n'est pas chargée, ignoré. | `zfs load-key`, puis montez-le. |
| `snapdir-disabled` | L'accès aux instantanés est désactivé, ignoré. | `zfs set snapdir=hidden <dataset>`. Le dossier `.zfs` reste masqué. |
| `not-visible` | BombVault ne voit pas le point de montage du jeu de données. | Déplacez le point de montage sous le chemin Host Data, ou associez-le au conteneur au même chemin avec Read/Write - Slave. |
| `shfs-only` | Le jeu de données n'est visible que via `/mnt/user`, qui masque les instantanés. | Associez `/mnt`, pas `/mnt/user`, comme Host Data. |
| `snapshot-not-visible` | L'instantané a été créé mais n'est pas apparu dans BombVault. | Lancez **Tester l'accès aux instantanés**. Voir plus bas. |
| `snapshot-loop` | L'instantané n'est pas parvenu à BombVault parce que Host Data ne transmet pas les nouveaux montages. | Réglez le mode d'accès de Host Data sur Read/Write - Slave et redémarrez BombVault. |
| `backup-failed` | restic a échoué pour ce jeu de données. | Les détails de l'exécution disent pourquoi. |
| `not-reached` | L'exécution s'est terminée avant ce jeu de données. | Relancez la sauvegarde. |
| `gone` | Le jeu de données n'est plus sur le serveur. | Rien. Ses sauvegardes restent restaurables. |
| `read-only-mount` | BombVault ne peut que lire le jeu de données, il ne peut donc pas restaurer dedans. | Réglez l'association sur Read/Write - Slave, ou restaurez dans un dossier. |
| `destination-not-mounted` | Le dossier n'est pas sur un pool ou un partage monté. | Choisissez un dossier sur un pool ou un partage. |
| `not-enough-space` | Pas assez de place libre à la destination. | Libérez de la place ou choisissez un autre dossier. |
| `safety-snapshot-failed` | L'instantané de sécurité n'a pas pu être pris, donc rien n'a été restauré. | Les détails montrent le message de zfs. |
| `safety-name-too-long` | Le nom du jeu de données est trop long pour un instantané de sécurité. | Désactivez l'instantané de sécurité, ou restaurez dans un dossier. |
| `dataset-exists` | Un jeu de données de ce nom existe déjà. | Choisissez un nouveau nom, ou restaurez dans le jeu de données lui-même. |
| `create-failed` | Le nouveau jeu de données n'a pas pu être créé. | Les détails montrent le message de zfs. Vérifiez que le jeu de données au-dessus existe. |
| `new-dataset-not-visible` | Le nouveau jeu de données a été créé, mais BombVault ne le voit pas, donc rien n'a été restauré. | Le jeu de données reste sur le serveur. Montez-le sous le chemin Host Data et restaurez dedans. |
| `set-properties-failed` | Les propriétés stockées n'ont pas pu être réglées, donc rien n'a été restauré. | Les détails montrent le message de zfs. |
| `set-limits-failed` | Les fichiers ont été restaurés, mais le quota ou la réservation stocké n'a pas pu être réglé. | Les détails montrent le message de zfs. Réglez vous-même le quota ou la réservation avec `zfs set`. |

### Vérifier ce que voit le conteneur {#mountinfo}

**Tester l'accès aux instantanés**, sur l'onglet Paramètres de l'élément, prend un vrai instantané de son arborescence, le cherche dans BombVault pour chaque jeu de données et le supprime de nouveau. C'est le moyen le plus rapide de prouver tout le chemin avant la première exécution planifiée.

Pour regarder vous-même, exécutez ceci sur le serveur :

```
docker exec BombVault grep zfs /proc/self/mountinfo
```

Chaque ligne est un montage dans le conteneur. La ligne d'un jeu de données montre son chemin dans le conteneur (sous `/host/user`) et le nom du jeu de données. Un champ `master:N` sur cette ligne veut dire que le montage reçoit les montages que l'hôte fait plus tard, ce dont l'accès aux instantanés a besoin. S'il manque, réglez le mode d'accès de Host Data sur Read/Write - Slave et redémarrez BombVault.

## TrueNAS SCALE {#truenas}

- Quand `LIBVIRT_URI` est définie (comme pour les sauvegardes de VM sur TrueNAS), BombVault prend l'hôte, l'utilisateur et le port SSH de ses commandes zfs dans l'URI, chacun de ceux qui ne sont pas définis séparément. Sans sauvegardes de VM, rien n'est à régler : `host.docker.internal` ne se résout pas dans une application TrueNAS, BombVault joint donc l'hôte par la passerelle par défaut de l'application, en root sur le port 22. Réglez `LIBVIRT_HOST`, `LIBVIRT_SSH_USER` et `LIBVIRT_SSH_PORT` quand vous voulez une autre adresse ou un autre utilisateur, dans l'`environment` de l'application.
- Ajoutez la clé SSH de l'hôte de **Paramètres, Intégrations, SSH de l'hôte** à root sous **Credentials, Users**, et activez le service SSH sous **System, Services**. Une [réplique](#replica-push) vers ce TrueNAS ou un autre demande aussi la clé de la réplique, qui est une autre.
- Un utilisateur autre que root a besoin d'une permission sur le jeu de données du haut de l'élément, qui couvre alors chaque jeu de données en dessous :

  ```
  zfs allow <user> snapshot,destroy,mount <dataset>
  ```

  Restaurer dans un nouveau jeu de données demande aussi `create` sur le jeu de données au-dessus, et régler les propriétés stockées demande la permission pour ces propriétés.

  Une session SSH non root sur TrueNAS n'a pas `/usr/sbin` dans son chemin. BombVault appelle alors directement `/usr/sbin/zfs`.
- Le **Host Data** de l'application doit être un chemin de l'hôte au-dessus des jeux de données, par exemple `/mnt/tank`, pas un ixVolume. Avec un chemin de l'hôte, l'application transmet à BombVault les nouveaux montages de l'hôte (`rslave`), ce dont l'accès aux instantanés a besoin. Voir [TrueNAS SCALE](getting-started.md#truenas) pour toute la configuration de l'application.
