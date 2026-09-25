# Jeux de données ZFS

La page **ZFS** sauvegarde des jeux de données ZFS. Un élément est un jeu de données avec tous les jeux de données situés en dessous. Pour chaque sauvegarde, BombVault prend un seul instantané ZFS de toute l'arborescence, si bien que chaque jeu de données qu'elle contient est capturé au même instant. Il lit ensuite les fichiers de chaque jeu de données depuis cet instantané, les stocke avec restic de la même façon qu'un dossier, puis supprime l'instantané aussitôt. Les sauvegardes sont dédupliquées, chacune peut être parcourue et des fichiers isolés peuvent être restaurés.

BombVault n'utilise jamais `zfs send` pour les jeux de données, ne fait jamais revenir un jeu de données en arrière et n'en détruit jamais un.

## Prérequis {#requirements}

- **La liaison SSH vers ce serveur.** Les jeux de données ZFS utilisent la même clé, le même hôte et le même utilisateur que les sauvegardes de VM. Si les sauvegardes de VM fonctionnent déjà, ceci fonctionne aussi. Sinon, suivez le [guide de la sauvegarde de VM via SSH](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) sur GitHub. Les champs du modèle s'appellent **Host SSH: Address**, **Host SSH: Port** et **Host SSH: User**.
- **La commande `zfs` sur cet hôte.** Unraid 6.12 et plus récent ainsi que TrueNAS SCALE l'ont.
- **Host Data mappé sur `/mnt` avec l'Access Mode Read/Write - Slave.** C'est la valeur par défaut du modèle. L'instantané d'un jeu de données n'apparaît dans le dossier `.zfs/snapshot` du jeu de données qu'après le démarrage de BombVault, le conteneur doit donc recevoir les montages que l'hôte fait plus tard.
- **Les jeux de données montés sous `/mnt`.** Sous Unraid, les pools se trouvent dans `/mnt/<pool>`, c'est donc déjà le cas.

Activez le domaine sous **Paramètres, Général** (Jeux de données ZFS). La page ZFS affiche alors une carte **Connexion à ce serveur**. Elle teste la liaison SSH, indique l'utilisateur et l'hôte auxquels elle se connecte et dit ce qui manque quand il manque quelque chose. La vérification de l'intégration à l'hôte (`/spike`) affiche le même résultat.

## Éléments et jeux de données enfants {#items-and-children}

Ouvrez **Ajouter des jeux de données** sur la page ZFS. La liste vient du serveur. Choisissez le jeu de données situé tout en haut de ce que vous voulez sauvegarder, par exemple `cache/appdata`, et l'élément le couvre ainsi que chaque jeu de données en dessous.

- **Les nouveaux jeux de données enfants s'ajoutent d'eux-mêmes.** Un jeu de données créé plus tard sous l'élément est sauvegardé lors de l'exécution suivante, qui le signale comme nouveau. Sa première sauvegarde le lit entièrement une fois ; ensuite, seules les modifications sont lues.
- **Vous pouvez exclure des enfants isolés.** Désactivez un enfant dans les réglages de l'élément et il est exclu avec tout ce qui se trouve en dessous. Un enfant exclu qui n'existe plus sur le serveur est signalé comme tel et peut être retiré de la liste.
- **Les enfants illisibles sont ignorés, jamais en silence.** L'exécution les liste, l'élément indique combien ont été ignorés et la carte de couverture du tableau de bord compte chacun comme non protégé. L'exécution sauvegarde tout de même le reste et n'échoue pas à cause d'un enfant ignoré. Les raisons figurent dans le [tableau des codes de raison](#reason-codes) : un jeu de données non monté, avec `canmount=off`, un point de montage `legacy` ou absent, une clé de chiffrement non chargée, l'accès aux instantanés désactivé ou un point de montage que BombVault ne voit pas.
- **Un jeu de données ignoré n'entraîne pas ses enfants avec lui.** Un jeu de données en `canmount=off` qui ne contient que d'autres jeux de données est ignoré (affiché comme « structure seulement ») et ses enfants montés sont sauvegardés. Un jeu de données chiffré dont la clé n'est pas chargée est ignoré avec les enfants qui partagent sa clé.
- **Les enfants qui sont des disques de VM ou des données système démarrent désactivés** dans la boîte de dialogue d'ajout, avec la raison à côté de l'interrupteur. Ajouter un pool entier demande une confirmation qui liste ce qu'il contient.

### Volumes {#volumes}

Un volume (zvol) contient un disque virtuel au lieu de fichiers, et la page ZFS n'en sauvegarde jamais.

- Un volume utilisé par une VM est sauvegardé avec cette VM sur la page **VMs**.
- Un volume qu'aucune VM n'utilise (un extent iSCSI, un disque détaché) **n'est pas sauvegardé par BombVault**. La boîte de dialogue d'ajout et la page ZFS comptent ces volumes et le disent. Une version ultérieure les sauvegardera.

Les volumes présents dans l'arborescence d'un élément sont ignorés et nommés à chaque exécution.

### Le stockage de Docker {#docker-storage}

Avec le pilote de stockage ZFS de Docker, chaque couche d'image est un jeu de données avec un point de montage `legacy`. La boîte de dialogue d'ajout les regroupe en une ligne par parent. Une arborescence qui contient plus de 20 de ces jeux de données ne peut pas devenir un élément : tant qu'un instantané en existe, Docker ne peut pas supprimer de couches d'image. Ajoutez plutôt les jeux de données situés en dessous, par exemple `appdata`.

### Les éléments ne se chevauchent jamais {#overlap}

Un jeu de données ne peut appartenir qu'à un seul élément. BombVault refuse un nouvel élément situé à l'intérieur d'un élément existant ou qui en contiendrait un. Pour fusionner plusieurs éléments enfants en un élément parent, supprimez d'abord les éléments enfants en choisissant de garder leurs sauvegardes, puis ajoutez le parent. Chaque jeu de données garde son historique sous son propre nom, si bien que la sauvegarde suivante reprend là où les anciens éléments s'étaient arrêtés et ne relit pas tout.

## Arrêter des conteneurs et lancer des commandes autour de l'instantané {#consistency}

Un instantané d'une base de données en cours d'exécution ressemble à une coupure de courant soudaine : la base de données s'en remet généralement, mais elle doit le faire. Chaque élément peut faire deux choses à ce sujet, et les deux ne couvrent que l'instant de l'instantané, pas toute la sauvegarde.

- **Arrêter ces conteneurs pour l'instantané.** BombVault arrête les conteneurs listés, prend l'instantané et les redémarre aussitôt. Les conteneurs d'un même niveau de dépendances s'arrêtent en parallèle, les dépendants d'abord, si bien que toute la fenêtre dure généralement quelques secondes ; l'exécution indique combien de temps. La sauvegarde lit ensuite l'instantané figé pendant que les applications tournent déjà de nouveau. Seuls les conteneurs qui tournaient sont arrêtés.
- **Une commande avant et après l'instantané.** Elle s'exécute dans un conteneur de votre choix, par exemple pour exporter une base de données dans le jeu de données juste avant l'instantané, sans rien arrêter. Si la commande d'avant l'instantané échoue, la sauvegarde échoue et aucun instantané n'est pris. Une commande d'après l'instantané qui échoue est signalée sur l'exécution mais ne fait pas échouer la sauvegarde.

Ce qui se passe quand quelque chose tourne mal :

- Si un conteneur ne peut pas être arrêté, BombVault redémarre ceux qu'il a déjà arrêtés et la sauvegarde échoue en nommant le conteneur. Il ne se rabat jamais sur un instantané d'applications en cours d'exécution.
- L'arrêt attend la fin d'une sauvegarde de conteneur en cours (jusqu'à 30 minutes pour une exécution manuelle, jusqu'à la durée limite de sauvegarde pour une exécution planifiée), afin que les deux n'arrêtent et ne démarrent jamais le même conteneur en même temps.
- Avant que le premier conteneur s'arrête, BombVault note lesquels il arrête. Si BombVault est tué pendant la fenêtre, il redémarre ces conteneurs à son prochain démarrage, envoie une notification, et l'élément affiche une note rouge pour chaque conteneur qu'il n'a pas pu démarrer.

Les exports automatiques de bases de données (voir [Fonctionnalités](features.md)) s'exécutent avec la sauvegarde propre d'un conteneur sur la page **Conteneurs**, pas avec un élément ZFS. Une base de données dont le conteneur n'est sauvegardé qu'à travers son jeu de données n'obtient aucun export, donnez-lui donc une commande ici.

Un conteneur peut figurer à la fois dans cette liste et sur la page **Conteneurs**. Ses données sont alors stockées deux fois, dans deux dépôts, et la **Sauvegarde complète** l'arrête deux fois. L'élément le signale.

## Restaurer {#restore}

Ouvrez **Sauvegardes** sur l'élément, choisissez la sauvegarde, puis le jeu de données. Par défaut, c'est le jeu de données supérieur de l'élément.

- **Restaurer dans le jeu de données.** Les fichiers de la sauvegarde sont écrits dans le point de montage du jeu de données. Les fichiers portant le même nom sont écrasés, les autres restent. Le jeu de données n'est jamais ramené en arrière ni remplacé. BombVault vérifie que le jeu de données est monté, visible et accessible en écriture, une fois avant de commencer et de nouveau juste avant d'écrire. Là où un jeu de données enfant est monté à l'intérieur, rien n'est écrit : l'enfant garde ses fichiers, son propriétaire et ses permissions, et est restauré depuis sa propre sauvegarde.
- **Restaurer dans un dossier.** Choisissez un dossier sous `/mnt`. BombVault vérifie que le dossier se trouve sur un pool ou un partage monté et qu'il y a assez d'espace libre. Cela fonctionne sans la liaison SSH et pour des jeux de données qui n'existent plus.
- **Choisir des fichiers** (avancé) : n'écrire dans le jeu de données que les fichiers et dossiers que vous choisissez.
- **Tous les jeux de données de cette sauvegarde** (avancé) : chaque jeu de données de l'arborescence dans son propre sous-dossier du dossier choisi. Les jeux de données ignorés dans cette sauvegarde sont nommés.
- **Depuis un autre serveur :** la page **Récupération** restaure depuis le dépôt d'un autre BombVault, toujours dans un dossier : tous les jeux de données d'une sauvegarde, chacun dans son propre sous-dossier, ou un jeu de données de l'arborescence, en entier ou des fichiers choisis.

La liste des conteneurs à arrêter de l'élément est aussi proposée pour une restauration dans le jeu de données. Ces conteneurs restent arrêtés pendant toute la restauration, et les sauvegardes de conteneurs attendent entre-temps.

### L'instantané de sécurité {#safety-snapshot}

Avant d'écrire dans un jeu de données, BombVault prend un instantané ZFS de ce seul jeu de données, nommé `bombvault-prerestore-<heure>`. Il est activé par défaut ; le désactiver demande une deuxième confirmation. Si l'instantané ne peut pas être pris, rien n'est restauré.

BombVault ne supprime jamais de lui-même un instantané de sécurité. L'élément les liste avec leur âge et leur taille, chacun avec une action **Supprimer**, et avertit quand le plus ancien a plus de 30 jours, car il garde sur le pool des données supprimées et modifiées.

Pour revenir en arrière après une restauration, copiez des fichiers isolés depuis `.zfs/snapshot/bombvault-prerestore-<heure>` dans le jeu de données. `zfs rollback <dataset>@bombvault-prerestore-<heure>` ne fonctionne que tant que c'est l'instantané le plus récent de ce jeu de données. `zfs rollback -r` supprime tous les instantanés plus récents, y compris les automatiques.

### Restaurer comme nouveau jeu de données {#new-dataset}

BombVault ne crée pas de jeux de données. Créez-le sur le serveur avec les propriétés voulues, puis restaurez dans un dossier qui est son point de montage :

```
zfs create -o compression=lz4 cache/appdata-restored
```

puis, dans BombVault, restaurez dans le dossier `cache/appdata-restored` sous `/mnt`.

## Ce que contient la sauvegarde {#contents}

Dans la sauvegarde : les fichiers et dossiers de chaque jeu de données sauvegardé, avec leur propriétaire, leurs permissions, leurs horodatages et leurs attributs étendus, tels que restic les stocke.

Pas dans la sauvegarde :

- les propriétés ZFS des jeux de données (compression, recordsize, quota, mountpoint et les autres) ;
- le propriétaire et les permissions du dossier supérieur de chaque jeu de données lui-même (tout ce qui est en dessous est inclus). Une restauration dans le jeu de données laisse le dossier supérieur existant tel quel, une restauration dans un dossier le crée avec les permissions `0755` ;
- les instantanés ZFS existants ;
- les enfants ignorés ou exclus ;
- les volumes.

Pour restaurer sur un nouveau pool, créez d'abord les jeux de données avec les propriétés voulues. Il n'a pas encore été vérifié si les ACL NFSv4, telles que TrueNAS les utilise sur les jeux de données SMB, reviennent comme vous l'attendez ; testez donc une restauration sur vos propres données avant de compter dessus.

## Jeux de données chiffrés {#encryption}

Un jeu de données chiffré n'est sauvegardé que tant que sa clé est chargée. Sinon, il est ignoré avec un avertissement ; chargez la clé avec `zfs load-key` et montez le jeu de données. BombVault lit les données déchiffrées et les stocke dans le dépôt de restic, qui est chiffré. Si vous avez désactivé le chiffrement dans BombVault, ce dépôt ne l'est pas.

## Instantanés restants {#leftover-snapshots}

L'instantané d'une sauvegarde s'appelle `<dataset>@bombvault-<14 chiffres>`, par exemple `cache/appdata@bombvault-20260924021500` (UTC). BombVault le supprime juste après la sauvegarde. Si cela échoue, par exemple parce que le jeu de données est occupé ou que BombVault a été arrêté, BombVault le supprime :

- avant la sauvegarde suivante de cet élément,
- au démarrage de BombVault, pour chaque élément, même avec le domaine désactivé,
- quand vous supprimez l'élément,
- quand vous appuyez sur **Retirer maintenant** sur l'élément, qui indique combien il en reste.

Seuls les noms qui correspondent exactement à `bombvault-` suivi de 14 chiffres sont supprimés. Les instantanés de sécurité, vos propres instantanés et les instantanés automatiques ne sont jamais touchés. Pour en supprimer un à la main :

```
zfs destroy -r cache/appdata@bombvault-20260924021500
```

## Anomalies {#anomalies}

Un enfant qui a été vidé change à peine le total d'une grande arborescence, c'est pourquoi la détection d'anomalies surveille chaque jeu de données d'un élément séparément : sa taille, son nombre de fichiers, ses nouvelles données et sa durée restic ont chacun leur propre historique. Cet historique appartient au nom du jeu de données, il reste donc quand l'arborescence est ensuite sauvegardée par un autre élément.

Un jeu de données que l'exécution précédente a sauvegardé et que celle-ci n'a pas pu lire compte comme vidé, tant que la sélection de l'élément n'a pas changé. Cela couvre une clé non chargée, un jeu de données non monté et un jeu de données qui a disparu de l'arborescence. Un enfant que vous excluez vous-même change la sélection, son historique repart donc de zéro. Tant qu'un constat de perte de données est ouvert, la rétention garde les anciennes sauvegardes de ce seul jeu de données et élague le reste de l'arborescence comme d'habitude.

Dans l'onglet **Éléments** de la page **Anomalies**, chaque jeu de données a sa propre ligne sous son élément, et l'arborescence de l'élément sur cette page affiche les constats ouverts à côté de chaque jeu de données. Le lien d'un constat ouvre le panneau de restauration de l'élément sur la dernière bonne sauvegarde du jeu de données. La question de savoir si une exécution se termine est jugée pour l'élément entier, car une exécution réussit ou échoue d'un bloc.

Les vérifications elles-mêmes sont décrites sous [Fonctionnalités](features.md). Un assistant connecté par le [serveur MCP](mcp.md) peut lister les points de restauration d'un élément ZFS, lancer sa sauvegarde et lire les constats, mais un constat se confirme sur la page **Anomalies**.

## Codes de raison {#reason-codes}

La page, l'historique des exécutions et les notifications nomment un problème avec l'un de ces codes. La plupart ont aussi la solution à côté sur la page.

| Code | Signification | Que faire |
|---|---|---|
| `ssh-missing` | La connexion SSH n'est pas configurée dans ce conteneur. | Configurez la liaison SSH comme pour les sauvegardes de VM. |
| `host-placeholder` | Host SSH: Address est toujours la valeur d'exemple, et `host.docker.internal` n'a pas répondu non plus. | Réglez Host SSH: Address sur l'IP LAN de ce serveur. |
| `host-fallback` | Host SSH: Address est toujours la valeur d'exemple, et `host.docker.internal` fonctionne. | Rien, ou réglez l'IP LAN. |
| `ssh-unreachable` | Le serveur est injoignable en SSH. | Vérifiez l'adresse et le port, et que SSH est activé. |
| `ssh-auth` | Le serveur a refusé la clé de BombVault. | Exécutez une fois sur le serveur la commande affichée sur la carte de connexion. |
| `zfs-not-found` | L'hôte SSH n'a pas de commande `zfs`. | Faites pointer Host SSH: Address vers la machine qui possède les pools. |
| `zfs-permission` | L'utilisateur SSH n'a pas le droit d'exécuter cette commande zfs. | Utilisez root, ou voir [TrueNAS SCALE](#truenas). |
| `uri-mismatch` | `LIBVIRT_URI` désigne un autre hôte ou un autre utilisateur que les champs SSH. | Mettez-les d'accord, ou videz les champs SSH pour que les deux viennent de l'URI. |
| `zfs-error` | zfs a signalé une autre erreur. | Les détails affichent son message. |
| `propagation-missing` | Les nouveaux montages sur l'hôte n'atteignent pas le conteneur. | Réglez l'Access Mode de Host Data sur Read/Write - Slave et redémarrez BombVault. |
| `invalid-name` | Un nom de jeu de données que BombVault n'accepte pas. | Renommez le jeu de données. |
| `name-too-long` | Un jeu de données de l'arborescence est trop long pour un nom d'instantané. | Renommez-le, ou ajoutez comme élément un jeu de données situé en dessous. |
| `invalid-exclude` | Un motif d'exclusion ou un enfant exclu ne correspond pas à l'élément. | Corrigez l'entrée que le message nomme. Pour exclure un jeu de données enfant entier, désactivez-le au lieu d'écrire un motif. |
| `not-found` | Le jeu de données n'existe pas sur le serveur. | Retirez l'élément ou recréez le jeu de données. Ses sauvegardes restent restaurables. |
| `not-filesystem` | C'est un volume, pas un système de fichiers. | Voir [Volumes](#volumes). |
| `overlaps-item` | Le jeu de données chevauche un élément existant. | Voir [Les éléments ne se chevauchent jamais](#overlap). |
| `docker-storage` | L'arborescence contient le stockage d'images de Docker. | Voir [Le stockage de Docker](#docker-storage). |
| `nothing-readable` | Aucun jeu de données de l'élément n'est lisible pour le moment. | Regardez les codes des jeux de données ignorés. |
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

### Vérifier ce que voit le conteneur {#mountinfo}

**Tester l'accès aux instantanés** sur un élément prend un vrai instantané de son arborescence, le cherche dans BombVault pour chaque jeu de données, puis le supprime. C'est le moyen le plus rapide de valider tout le chemin avant la première exécution planifiée.

Pour regarder vous-même, lancez ceci sur le serveur :

```
docker exec BombVault grep zfs /proc/self/mountinfo
```

Chaque ligne est un montage dans le conteneur. La ligne d'un jeu de données montre son chemin dans le conteneur (sous `/host/user`) et le nom du jeu de données. Un champ `master:N` sur cette ligne signifie que le montage reçoit les montages que l'hôte fait plus tard, ce dont l'accès aux instantanés a besoin. S'il manque, réglez l'Access Mode de Host Data sur Read/Write - Slave et redémarrez BombVault.

## TrueNAS SCALE {#truenas}

- Quand `LIBVIRT_URI` est défini (comme pour les sauvegardes de VM sous TrueNAS), BombVault prend dans l'URI l'hôte, l'utilisateur et le port SSH de ses commandes zfs, chacun de ceux qui ne sont pas définis séparément. Sans sauvegardes de VM, définissez plutôt `LIBVIRT_HOST`, `LIBVIRT_SSH_USER` et `LIBVIRT_SSH_PORT`. Ajoutez les variables sous **Additional Environment Variables**.
- Un utilisateur autre que root a besoin de permissions sur le jeu de données supérieur de l'élément, qui couvrent alors chaque jeu de données en dessous :

  ```
  zfs allow <user> snapshot,destroy,mount <dataset>
  ```

  Une session SSH non root sous TrueNAS n'a pas `/usr/sbin` dans son chemin ; BombVault appelle alors `/usr/sbin/zfs` directement.
- Le **Host Data** de l'application doit être un chemin d'hôte situé au-dessus des jeux de données, par exemple `/mnt/tank`, et non un ixVolume. Avec un chemin d'hôte, l'application transmet à BombVault les nouveaux montages de l'hôte (`rslave`), ce dont l'accès aux instantanés a besoin.
