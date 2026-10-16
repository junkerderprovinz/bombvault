# Sauvegarde hors site et récupération

!!! note "Les copies hors site attendent après une reconstruction"
    Quand l'Assistant d'urgence ou **Découvrir les sauvegardes** reconstruit des éléments sans les anciens réglages, les copies hors site de ces sections se mettent en pause jusqu'à ce que leur valeur par défaut soit confirmée. Voir [Après une reconstruction](#after-a-rebuild).

Les sauvegardes locales vous protègent d'un conteneur perdu ou d'une mauvaise mise à jour. Les copies hors site et un kit de récupération testé vous protègent de la perte de toute la machine, d'un ransomware ou d'un incendie. Cette page couvre les destinations et les copies hors site, le fait de rendre une copie à l'épreuve des manipulations, la preuve que vous pouvez restaurer, l'appairage de plusieurs serveurs et la récupération de tout quand BombVault lui-même a disparu.

## Copies hors site {#off-site-copies}

Gardez la sauvegarde locale rapide et ajoutez une ou plusieurs destinations hors des locaux. BombVault y copie les nouvelles sauvegardes avec `restic copy` au mieux, de sorte qu'un accroc hors site ne fait jamais échouer la sauvegarde locale. Dans cette forme, le dépôt local reste primaire et le dépôt hors site est une copie, mais le dépôt primaire d'une section n'a pas à être local du tout ; voir plus bas [Dépôts primaires distants](#remote-primary-repositories) pour sauvegarder directement vers S3, un rest-server et autres au lieu d'y copier.

  - **Plusieurs destinations par section.** Chaque section (conteneurs, VMs, flash, auto-sauvegarde, dossiers et jeux de données ZFS) peut copier vers plusieurs destinations hors site à la fois, de sorte que vous pouvez garder, par exemple, un rest-server chez un ami et un bucket S3 en parallèle. La carte **Destinations par section** de la page Destinations active ou désactive chaque ligne d'une section vers une destination.
  - **Quand les copies s'exécutent.** La carte **Copier** d'une destination règle **Quand copier** : **Après chaque sauvegarde**, ou selon un planning propre qui rassemble les sauvegardes et copie à heures fixes. Sous **Paramètres, Planning, Copies hors site par section**, **Exception** donne à une section son propre planning de copie. **Répliquer maintenant** dans la carte **Copie hors site** d'une section, sous **Paramètres, Destinations**, lance une copie à la demande.
  - **La rétention** se règle par destination sous **Durée de conservation**, de sorte que vous pouvez garder les copies hors site plus longtemps comme archive. La copie dans le champ hors site d'une section suit la carte **Rétention hors site** sous **Paramètres, Destinations** ; laissez-la entièrement à zéro pour ne jamais rogner automatiquement les sauvegardes hors site.
  - **Les limites de bande passante** (**Bande passante hors site** sous **Paramètres, Destinations**, ou la carte **Copier** d'une destination) plafonnent le débit d'envoi/de téléchargement de restic pour que les copies ne saturent pas votre WAN.
  - Pendant qu'une copie s'exécute, l'en-tête d'état de l'aperçu indique **Copie hors site en cours**. C'est un indicateur d'activité, pas une barre de pourcentage, car `restic copy` n'expose aucune progression lisible par machine.

!!! note "Restaurer depuis n'importe quel endroit"
    Chaque élément liste ses sauvegardes comme une seule chronologie sur tous les endroits où une sauvegarde se trouve, dans son onglet **Sauvegardes**. Une sauvegarde copiée vers B2 apparaît une fois, marquée de chaque endroit qui la détient. Une restauration prend le premier endroit qu'elle peut atteindre, en commençant par le dépôt où l'élément est écrit, et vous pouvez choisir un autre endroit par sauvegarde. Les destinations hors site ne sont lues que lorsque vous cliquez sur **Vérifier**, car lister un stockage cloud prend du temps. Supprimer à un endroit contrôle d'abord les autres et dit si c'était la dernière copie.

## Destinations {#destinations}

Une destination est un endroit où vont les sauvegardes : un dossier local, un partage, un serveur à vous ou un service cloud. Elle se configure une fois et sert ensuite chaque section. Ouvrez **Destinations** depuis sa tuile de paramètres, ou activez-y **Afficher aussi dans la barre latérale** pour l'épingler.

**Ajouter une destination** ouvre une fenêtre en quatre étapes :

1. **Fournisseur.** Chaque service est listé avec son logo, avec une recherche en haut à droite de la fenêtre, en groupes : services de stockage avec des buckets S3 (Backblaze B2, Wasabi, Cloudflare R2, Hetzner Object Storage, Amazon S3 et d'autres), votre propre serveur S3 (Garage, SeaweedFS, RustFS, Silo, Ceph, JuiceFS, Versity S3 Gateway), votre propre serveur et partages (rest-server, Hetzner Storage Box, SFTP, SMB, WebDAV, un chemin monté), stockage cloud (OneDrive, Google Drive, Dropbox, pCloud, Nextcloud et le reste que rclone prend en charge) et les serveurs récepteurs de votre groupe. Chacun dit à quel point il convient aux sauvegardes : les disques cloud ralentissent sous de nombreuses requêtes, donc la première sauvegarde et l'élagage y sont plus longs.
2. **Connexion.** Les champs dépendent du service : une clé d'accès pour S3, un utilisateur et un mot de passe pour WebDAV et SMB, un mot de passe d'application là où la connexion à deux facteurs bloque le mot de passe normal, la clé SSH publique de BombVault pour SFTP et la Storage Box, ou un jeton pour les services qui se connectent via un navigateur. Pour ceux-ci, la fenêtre affiche une commande `rclone authorize` à exécuter sur un ordinateur avec navigateur ; le jeton qu'elle imprime va dans le champ. **Tester** vérifie la connexion avant que rien ne soit enregistré. La fenêtre liste ensuite les dossiers de la destination, avec **Nouveau dossier** pour en créer un et l'espace libre quand le service l'indique. Un dossier vide est le plus sûr ; BombVault y place un dossier par section.
3. **Protection.** La fenêtre dit clairement ce que le service peut faire. Un rest-server en mode append-only refuse la suppression, et le test de sabotage le vérifie. Un bucket S3 peut garder d'anciennes versions avec le versionnage et le verrouillage d'objets, ce que BombVault ne peut pas encore vérifier. Un disque cloud ne peut pas du tout refuser la suppression : quiconque entre sur le serveur entre aussi dans cette copie. N'activez **Protection contre la suppression (append-only)** que là où l'autre côté refuse vraiment la suppression ; BombVault n'y élague alors jamais. **En cas d'urgence** dit ce que le kit de récupération contient pour cette destination.
4. **Pour quelles sections.** Choisissez les sections qui sauvegardent ici. La première copie apporte tout l'historique. Les éléments qui ont choisi leurs propres destinations ne reçoivent pas la nouvelle d'eux-mêmes, et la fenêtre les nomme.

Les services S3 passent par le backend S3 propre de restic, ce qui permet d'appliquer une classe de stockage et le verrouillage d'objets. Tous les autres services passent par le rclone livré avec BombVault, et son remote apparaît alors dans la carte **Hors site (rclone)** sous **Paramètres, Destinations**. Un export des réglages contient les destinations ; avec les identifiants inclus, il contient aussi leur connexion.

Le dossier de ce serveur s'appelle **Local** partout : sur les pages des destinations, dans le flux, sur un élément et dans Restauration. Si ce serveur a plusieurs dossiers, chacun porte son propre nom. Une destination créée par le champ hors site d'une section porte le nom de son lieu, le dernier dossier de son adresse ou, à défaut, son serveur, et reçoit un numéro quand une autre destination porte déjà ce nom. Un nom que vous lui donnez sur sa page est conservé. Quand deux destinations de même nom se côtoient, chacune porte son serveur ou son dossier.

Chaque destination a sa propre page :

- **Connexion** avec son nom, l'interrupteur **Disponible** (désactivé signifie que BombVault n'écrit plus ici, et les éléments qui sauvegardent ici échouent avec un message clair au lieu d'atterrir ailleurs), le dossier, la compression, la classe de stockage et **Hors des locaux**. **Modifier la connexion** teste et enregistre une nouvelle adresse ou une nouvelle connexion.
- **Identifiants** : le jeu partagé ou un jeu à elle.
- **Protection contre la suppression**, avec **Tester la protection contre la suppression maintenant** et qui élague cette copie.
- **Copier** : quand copier, et les limites d'envoi et de téléchargement.
- **Durée de conservation** : **Court**, **Équilibré**, **Long** ou **Valeurs personnalisées**, avec une frise des sauvegardes que la règle conserve. **Demander avant de conserver moins** demande avant qu'une règle plus courte supprime des sauvegardes dont l'unique copie est ici.
- **Règles par section** : le dépôt, la rétention ou la compression propres à une section sur cette destination. **Modifier les règles** les ouvre, et **Revenir à la destination** ramène une section.
- **Déjà sous cette destination** : sections dont l'adresse saisie à la main se trouve dans un dossier de cette destination. En reprendre une laisse le dépôt où il est ; la connexion, la classe de stockage et la protection contre la suppression viennent alors de la destination. BombVault vérifie d'abord que la connexion de la destination ouvre le dépôt, et refuse de placer un dépôt append-only sous une destination qui ne l'est pas.
- Une cuve qui montre le remplissage de la destination, avec le repère de son plein et le nombre de semaines restantes au rythme de croissance des dernières semaines. Une destination qui ne signale pas sa taille indique **Taille inconnue**.

Un serveur récepteur qu'une autre instance de votre groupe exécute apparaît parmi les fournisseurs sous **Depuis votre groupe** ; voir [Serveur récepteur](#receiving-server).

### Déplacer et supprimer {#move-remove}

- **Déplacer** emmène la copie d'une section vers une autre destination, avec tout son historique. Tout reste à l'ancien endroit ; BombVault n'y supprime rien. Le déplacement s'exécute en arrière-plan et peut être long : environ 9 minutes par 100 Go entre disques locaux, des heures sur une liaison lente. L'aperçu le montre pendant qu'il s'exécute.
- **Supprimer la destination** offre deux voies. **Supprimer la destination, garder les sauvegardes** cesse d'y écrire et laisse les sauvegardes en place ; vous pourrez les lire plus tard sous **Restauration, Depuis un ancien dossier de sauvegarde**. **Supprimer la destination et effacer les sauvegardes qui s'y trouvent** lit d'abord ce qui y est stocké et le liste : quelles sauvegardes partent, lesquelles ne sont nulle part ailleurs, et quels dépôts restent parce que la protection contre la suppression refuse, que des éléments sauvegardent encore directement vers eux, qu'une autre destination ou section y garde ses copies, ou qu'ils n'ont pas pu être lus. Les éléments qui perdraient leur unique sauvegarde sont listés à part et conservés sauf si vous activez **Effacer aussi celles-ci**. Vous confirmez en saisissant le nom de la destination, et la suppression s'exécute en arrière-plan.
- Cela vaut pour tous les types de destination : une destination que plusieurs sections utilisent, une cible configurée pour une seule section, un dépôt que vous avez créé et le dossier dans lequel une section écrit ses propres sauvegardes. Les éléments qui sauvegardent vers un dépôt supprimé suivent de nouveau leur section, tout comme une valeur par défaut de la section qui pointait vers lui. Le dossier d'une section ne peut pas simplement disparaître, car chaque section en a besoin d'un. La fenêtre demande donc le dossier vers lequel les sections déménagent, et chaque section garde en dessous le nom de son dossier.
- Supprimer les sauvegardes d'une destination exige un mot de passe de connexion et une connexion sur l'instance. Sans mot de passe, la destination peut encore être supprimée, seules ses sauvegardes ne peuvent pas être effacées d'ici.

## Emplacement par élément {#placement}

La carte **Planning et destination** de l'onglet Paramètres d'un élément a une rangée de boutons : **Local** et un bouton par destination hors site de la section. Les boutons allumés reçoivent les sauvegardes de l'élément.

- Avec **Local** allumé, l'élément est écrit dans le dépôt indiqué sous **Stocké sur** et copié vers chaque autre destination allumée. Éteignez une destination et elle ne reçoit plus rien de neuf de cet élément. Local seul ne copie nulle part, ce qui convient aux données qui ont déjà une seconde copie, par exemple un partage qui vit sur un NAS.
- Avec **Local** éteint, l'élément est écrit directement dans le dépôt direct de la première destination allumée et copié de là vers les autres allumées. La première fois, une fenêtre crée ce dépôt direct.
- Un bouton reste allumé, car une sauvegarde a besoin d'un endroit où aller. Pour laisser un élément hors des sauvegardes, utilisez **Ne pas sauvegarder**.

BombVault ne déplace jamais de sauvegardes entre dépôts. Un élément qui suit sa section suit aussi ses changements : quand la valeur par défaut de la section change, sa prochaine sauvegarde arrive au nouvel endroit, et les sauvegardes qu'il a déjà restent où elles sont. Un emplacement choisi sur l'élément lui-même est fixé dès sa première sauvegarde ; la carte **Dépôt** de l'onglet Paramètres le nomme. Les copies peuvent changer à tout moment. Une destination qui ne reçoit plus un élément garde les copies qu'elle a et les rogne selon sa propre rétention à la prochaine exécution hors site de la section. On ne peut pas supprimer dans les destinations append-only.

L'endroit qu'un élément a quitté garde ses sauvegardes. Dans l'onglet **Sauvegardes** de l'élément et dans Restauration, il apparaît comme **Anciennes sauvegardes uniquement**, une restauration peut toujours y lire, et un prune y laisse ces sauvegardes tranquilles.

**Suivre le réglage par défaut** ramène à sa section un élément qui a un choix propre. Il demande d'abord, dit où iront alors les nouvelles sauvegardes et nomme ce que chaque destination cesse de recevoir et ce que la prochaine exécution enverrait.

Sous la rangée, la carte dit où va l'élément et ce qui s'y trouve réellement : combien de sites le détiennent, quand chaque destination a été vue pour la dernière fois, et si le 3-2-1 est respecté. Un site est le serveur avec les données d'origine, chaque destination hors site et chaque dépôt marqué **Hors des locaux**. BombVault contrôle les copies et les sites ; il ne contrôle pas la partie « deux supports » du 3-2-1.

### Valeurs par défaut par section {#placement-defaults}

La carte **Destinations par section** de la page Destinations dessine la valeur par défaut de chaque section sous forme de flux 3-2-1. Chaque ligne d'une section vers une destination est un interrupteur, **Local** compris, pour chaque section, y compris la clé flash, les jeux de données ZFS et l'auto-sauvegarde. Un clic sur une ligne ou un point la bascule aussitôt, sans question ; les lignes en pointillés sont désactivées. **Graphique / Interrupteurs** montre la même chose sous forme de rangées de boutons. Un interrupteur décide où vont les nouvelles sauvegardes. Les éléments sans choix propre et les dossiers de projet des stacks Compose suivent aussitôt, si bien que leur prochaine sauvegarde va aux nouveaux endroits. Les sauvegardes déjà stockées restent où elles sont et peuvent toujours y être restaurées. La clé flash, les jeux de données ZFS et l'auto-sauvegarde sauvegardent dans leur dossier local et copient hors site à partir de cette sauvegarde. Leur trait vers **Local** peut aussi être désactivé ; leurs sauvegardes vont alors directement vers leurs destinations hors site, et un élément ZFS avec une réplique et sans aucune destination n'envoie que sa réplique.

Avec **Local** désactivé et seulement des destinations hors site activées, une section sauvegarde directement vers elles, et BombVault choisit lui-même le dépôt sur place. Les conteneurs, les VMs et les jeux de dossiers vont dans le dépôt direct de la première destination activée, décrit plus bas, et sont copiés de là vers les autres. La clé flash, les jeux de données ZFS et l'auto-sauvegarde écrivent chaque sauvegarde vers chaque destination activée, et chaque destination les garde selon sa propre rétention.

Une section dont toutes les lignes sont désactivées indique **Pas de sauvegarde**. Ses éléments ne reçoivent aucune sauvegarde et comptent comme non protégés.

Les éléments qui ont choisi leurs propres destinations sur leur page les gardent. Le flux dit combien il y en a dans une section, par exemple **2 éléments diffèrent**, et y mène : vers l'onglet Paramètres de l'élément quand il n'y en a qu'un, vers une liste indiquant où chacun sauvegarde quand il y en a plusieurs. **Suivre le réglage par défaut** y ramène un élément.

Une nouvelle destination hors site reçoit chaque élément qui n'est pas réglé sur Local. La fenêtre qui l'ajoute dit combien d'éléments et, quand on le sait, quel historique cela représente.

### Dépôts directs

Un dépôt direct est l'endroit où les éléments sauvegardent directement vers une destination hors site. BombVault le place à côté du dossier de la destination, avec `-direct` ajouté au nom, ou, quand la destination se trouve à la racine de son bucket, de son remote ou d'un utilisateur rest-server, dans un dossier à l'intérieur qui porte le nom de la section, par exemple `containers-direct`. Dans le flux de la page Destinations, il est créé sans question. Éteindre Local pour un seul élément, de sorte qu'une destination sans dépôt direct devienne son foyer, ouvre une fenêtre avec l'emplacement suggéré, par exemple `s3:https://s3.eu-central-003.backblazeb2.com/bucket/containers-direct`, et un test de connexion qui ne crée rien. **Créer et utiliser** crée le dépôt et y fait pointer l'élément. Un dépôt direct prend la clé, la classe de stockage, les limites, le réglage append-only et la rétention de la destination, et change avec eux ; la carte Dépôts l'affiche en lecture seule. Quand une nouvelle clé de la destination ne peut pas l'ouvrir, le dépôt direct garde la clé qu'il a et l'enregistrement le dit. Un élément sur un dépôt direct est copié de là vers les autres destinations allumées, jamais vers la destination à laquelle appartient le dépôt. Ses sauvegardes portent l'étiquette `bv:direct`, et toute autre passe de rétention les conserve, de sorte qu'un dépôt direct qui a perdu son lien avec sa destination ne vieillit jamais selon les règles locales. B2 s'atteint par son point de terminaison S3, avec l'ID de clé et la clé d'application comme identifiants S3 ; une clé limitée au dossier propre de la destination ne peut pas atteindre le dossier voisin, donc limitez la clé au dossier au-dessus.

### Hors des locaux

Une destination ou un dépôt nommé peut être marqué **Hors des locaux**. Les remotes commencent marqués ; désactivez-le pour un rest-server dans le même bâtiment. Le repère ne fait que compter les sites et le 3-2-1 sur les cartes. Il ne change aucune copie.

Une réplique ZFS est une copie hors des locaux, mais pas une sauvegarde avec historique. Elle ne garde que les instantanés que vous fixez, et les anciennes versions et fichiers isolés viennent toujours des sauvegardes. Elle va d'un serveur ZFS directement à l'autre, via SSH vers un serveur ZFS ou vers le point de réception d'une instance appairée. Le groupe d'appairage ne transporte que la demande. Voir [Réplique](zfs-datasets.md#replica).

### Après une reconstruction {#after-a-rebuild}

Les choix de copie vivent dans les réglages propres de BombVault. Après une reconstruction par **Découvrir les sauvegardes** ou par l'Assistant d'urgence sans réglages restaurés, ils ont disparu, et tout copier renverrait vers B2 les éléments que vous aviez écartés. Les copies hors site de chaque section reconstruite se mettent donc en pause. L'aperçu l'indique, et la carte **Destinations par section** propose **Confirmer la valeur par défaut** avec un aperçu de ce que copiera la prochaine exécution et des noms dans les sauvegardes qui n'ont pas d'élément, que vous pouvez écarter là. Seule la confirmation met fin à la pause ; importer un fichier de réglages ramène règles et valeurs par défaut mais n'y met pas fin.

## Dépôts primaires distants {#remote-primary-repositories}

Le chemin propre d'une section ne se limite pas à un dossier local : pointez-le directement vers un remote restic (`s3:...`, `rest:http://host:8000/repo`, `sftp:user@host:/repo`, `rclone:remote:bucket/path`) et BombVault sauvegarde directement vers lui, sans copie locale séparée ni étape de copie. C'est une forme vraiment différente des copies hors site ci-dessus : là, le dépôt local est primaire et le dépôt hors site en est une archive au mieux ; ici, le dépôt distant **est** le primaire, et c'est la seule copie sauf si vous copiez aussi cette section vers une autre destination.

Le chemin de chaque section (Conteneurs, VMs, Flash, Auto-sauvegarde, Dossiers, Jeux de données ZFS) se règle sur la page de la destination **Local** : **Modifier** à côté de la section dans sa carte **Connexion** ouvre une fenêtre avec un commutateur **Local / Distant** :

- **Local** affiche l'explorateur de dossiers habituel.
- **Distant** le remplace par un simple champ d'URL et les réglages de connexion qu'utilise une destination hors site, configurés pour ce primaire. Vous obtenez alors :
    - **Un test de connexion** contre le chemin réel, avant de vous y fier.
    - **Limites de bande passante** (envoi/téléchargement) pour qu'une sauvegarde planifiée vers un primaire distant ne sature pas votre WAN : les mêmes indicateurs restic `--limit-upload`/`--limit-download` que les copies hors site, appliqués à la sauvegarde elle-même.
    - **Protection append-only**, vérifiée par le même test de sabotage actif (une vraie sonde DELETE contre l'autre côté) que reçoivent les destinations hors site. Activée, BombVault refuse d'élaguer le dépôt lui-même, car il n'y a pas de copie locale séparée derrière et les identifiants de cette machine ne doivent pas pouvoir supprimer l'unique copie de la sauvegarde.
    - **Une alerte de budget de croissance**, tirée de la même tendance de taille du dépôt que suit déjà la carte Stockage.

Rien de tout cela n'est obligatoire : un chemin distant saisi à la main sans réglages de sécurité enregistrés sauvegarde sans limite, peut être élagué et n'a pas d'alarme de budget. Les réglages de sécurité sont là pour quand vous voulez les mêmes protections qu'une copie hors site, sans avoir besoin d'une destination séparée rien que pour les obtenir.

!!! note "Les identifiants cloud et REST sont partagés"
    Un primaire distant s'authentifie avec les identifiants cloud partagés configurés sous **Paramètres, Destinations** ; il n'y a pas de magasin d'identifiants séparé pour les dépôts primaires.

### SMB et WebDAV sans montage sur l'hôte {#smb-webdav}

La carte **Hors site (rclone)** sous **Paramètres, Destinations** a un formulaire pour un partage Windows ou Samba et pour un serveur WebDAV (Nextcloud, ownCloud, SharePoint ou tout autre). Renseignez un nom court, l'hôte et le partage (SMB) ou l'URL et le type de serveur (WebDAV), l'utilisateur et le mot de passe, et BombVault écrit la section rclone pour vous. rclone obscurcit lui-même le mot de passe avant de le stocker ; en ajouter un avec un nom déjà existant remplace cette section au lieu d'en ajouter une seconde. **Ajouter une destination** propose SMB et WebDAV comme fournisseurs avec les mêmes champs.

Le formulaire répond avec l'emplacement terminé, par exemple `rclone:nas:backups`. Utilisez-le comme chemin d'une section ou comme destination et ajoutez un sous-dossier si vous le souhaitez (`rclone:nas:backups/bombvault`). Le partage est le premier segment du chemin, pas une partie du nom.

C'est une meilleure voie que de monter le partage sur Unraid : restic déconseille de garder un dépôt sur un partage CIFS monté, et ici rien n'est monté. NFS n'est pas dans le formulaire parce que ni restic ni rclone n'ont de backend NFS ; pour NFS, montez l'export sur l'hôte et utilisez-le comme dossier local.

## Protection contre la suppression (append-only)

Activez la protection contre la suppression pour une destination hors site afin qu'un ransomware, ou un hôte compromis, ne puisse ni supprimer ni réécrire vos sauvegardes. L'autre côté (un `restic/rest-server` en mode `--append-only`) **l'impose**. BombVault ne fait que la **vérifier** et n'affiche jamais de vert sur une simple déclaration de configuration. Un dossier de ce serveur ne peut pas refuser la suppression, car quiconque contrôle BombVault contrôle aussi le dossier, donc les destinations locales ne proposent pas l'interrupteur.

L'interrupteur appartient à la destination : une section qui y copie le prend de la destination, il s'active donc sur la page de la destination, sous **Destinations**. La vérification planifiée **Vérifier la protection contre la suppression** ne tourne que tant qu'une section copie vers une destination dont l'interrupteur est activé. L'autre côté doit être lancé avec `--append-only` ; avec l'image `restic/rest-server`, sur TrueNAS comme sur tout autre hôte Docker, cela va dans la variable `OPTIONS`.

La voie la plus rapide vers une copie append-only est un BombVault appairé avec un [serveur récepteur](#receiving-server) : il exécute le rest-server pour vous, en mode append-only.

!!! note "Une suppression réussie sous `/locks/` est attendue"
    Append-only ne signifie pas que plus rien ne peut être supprimé. restic doit poser et relâcher ses propres verrous, `/locks/` reste donc volontairement accessible en écriture et en suppression. Les snapshots et les données derrière eux, précisément ce que viserait un rançongiciel, ne peuvent pas être supprimés. Si vous sondez vous-même le serveur distant, une suppression qui réussit sous `/locks/` est le comportement correct et non une faille.

!!! warning "Les dépôts append-only ne sont jamais élagués depuis cette machine"
    Une destination append-only n'élague jamais d'anciennes sauvegardes d'ici ; l'élagage est l'affaire de l'autre côté. Réglez-lui une **alarme de budget de croissance** pour être alerté avant que la taille du dépôt ne s'emballe.

## Test de sabotage

BombVault prouve périodiquement la garantie append-only en tentant réellement une suppression contre le dépôt hors site, visant un objet inexistant. **Tester la protection contre la suppression maintenant** sur la page d'une destination la lance à la demande, et **Vérifier la protection contre la suppression** sous **Paramètres, Vérification, Contrôles automatiques** la lance selon un planning.

Une copie désactivée n'est pas testée, et la destination l'indique. Le test de connexion fonctionne toujours, car il ne fait que lire : une copie peut donc être vérifiée avant d'être activée.

- **Refusé** signifie protégé.
- **Accepté** signifie non protégé.
- Un résultat **non concluant** (serveur injoignable, erreur d'authentification) ne renverse jamais le verdict stocké.

Un vrai basculement de protégé à non protégé déclenche une alerte unique.

## Essais de reprise après sinistre

BombVault offre deux niveaux de preuve que vos sauvegardes sont réellement restaurables, pas seulement présentes.

- **Essais de vérification de restauration (locaux).** BombVault exécute périodiquement `restic check --read-data-subset` (borné, jamais une restauration complète qui remplit le disque). La cadence se règle sous **Paramètres, Vérification, Contrôles automatiques**, et la carte **Vérifier les sauvegardes** de la même tuile montre le résultat.
- **Restauration d'essai (hors site).** BombVault restaure une vraie cible du dépôt hors site dans un bac à sable isolé, la vérifie fichier par fichier et octet par octet, puis nettoie. Cela prouve que vous pouvez vous rétablir depuis le hors site, pas seulement que le dépôt répond.

La carte **État de protection** de l'aperçu résume cela par section en quatre contrôles : une sauvegarde à jour sur ce serveur, une copie hors site, la protection contre la suppression là-bas et une restauration d'essai depuis là-bas. Un clic sur une section montre les détails, et la carte ne passe au vert que sur des faits vérifiés.

## Appairage des instances {#pairing}

Plusieurs BombVault travaillent ensemble comme un groupe : ils se montrent mutuellement leur état de protection, prennent des rôles les uns pour les autres et peuvent garder la clé les uns des autres. Une instance rejoint le groupe avec douze mots.

Sur la première instance, ouvrez **Paramètres, Appairage** et appuyez sur **Générer une phrase**. Trois cartes d'étapes montrent le déroulement. Douze mots apparaissent dans une fenêtre avec un bouton **Copier**. Sur chaque autre instance, ouvrez le même endroit, appuyez sur **Saisir une phrase** et collez-les ou saisissez-les, ou appuyez sur **Coller** dans cette fenêtre. Un mot qui n'est pas dans la liste est nommé avec sa position pendant la saisie, et le dernier mot porte une somme de contrôle, de sorte qu'un mot mal saisi ou échangé est repéré avant que quoi que ce soit s'appaire. Générez la phrase sur une seule instance : deux instances qui en génèrent chacune une forment deux groupes distincts. Si personne n'apparaît en une minute, la page propose deux issues : afficher de nouveau les mots pour les saisir là-bas, ou saisir les mots de l'autre instance et rejoindre son groupe en une seule étape. L'appairage fonctionne sans mot de passe de connexion, mais définissez-en un : sans lui, quiconque peut ouvrir l'interface web peut lire les mots et, via le groupe, demander des rôles qui remettent le mot de passe restic des instances qui s'y trouvent. La carte d'appairage le dit tant qu'aucun mot de passe n'est défini. Avec un mot de passe, afficher de nouveau la phrase le demande. **Quitter le groupe** retire une instance. La carte **Instances** de la page d'appairage liste les membres du groupe ; l'image de qui fait quoi pour qui se trouve sur la page Instances.

Quiconque connaît les mots peut rejoindre le groupe, traitez-les donc comme un mot de passe.

**Comment les membres se joignent.** Chaque instance apprend sa propre adresse sur le réseau depuis votre navigateur dès que vous vous connectez, listée sous **Adresses connues** dans la carte **Cette instance** de la page d'appairage avec celles que l'instance a trouvées elle-même ; corrigez-la là si un reverse proxy ou un port inhabituel se trouve devant. Sur le même réseau, les membres annoncent cette adresse par multicast et se parlent directement, et là où le multicast ne peut pas traverser un réseau de conteneurs, comme le bridge par défaut de Docker, une instance cherche à la place les autres dans son propre sous-réseau avec un appel signé auquel seul un membre du groupe peut répondre, si bien que l'appairage se termine en quelques secondes sans relais. Si rien n'apparaît, **Vous ne la trouvez pas ?** accepte une adresse à la main, pour un autre sous-réseau ou un port non standard. Les instances sur des réseaux différents passent par un relais, choisi sur une rangée de quatre dans la carte **Relais** de la même page ; chaque forme vient avec une image :

- **Relais du projet** (par défaut) : `parleyport.halleluja.design`, le relais qu'utilise aussi KnightLoader. Rien à configurer.
- **Relais personnel** : le conteneur [**ParleyPort**](https://github.com/junkerderprovinz/parleyport) des Community Apps d'Unraid (aussi sous **Paramètres, Applications**). Saisissez son adresse sur chaque instance qui doit l'utiliser.
- **Instance BombVault** : l'une de vos instances déjà joignable de l'extérieur, avec **Servir de relais** activé. Cette instance répond alors sur `/relay/connect` à sa propre adresse, derrière le reverse proxy et le certificat qu'elle a déjà, et ne laisse entrer que votre groupe. Saisissez son adresse sur chaque instance qui doit l'utiliser.
- **Aucun relais** : les membres se trouvent automatiquement uniquement sur le même réseau, et nulle part ailleurs.

**Ce que voit le relais.** Un bouton du même nom à côté des quatre formes le montre pour la forme choisie. Chaque appel entre membres est scellé en AES-256-GCM sous une clé dérivée des douze mots, et cette clé ne quitte jamais vos instances. Le relais apprend un hachage qui regroupe les connexions, à quelle instance un message est destiné, sa taille et le moment où il passe. Un appel direct sur le réseau local est scellé de la même façon et en plus signé, de sorte que rien ne dépend du certificat auto-signé qu'une instance sert. Les appels qui remettent quelque chose (réponses de rôle, mot de passe restic, identifiants de récepteur, copies déposées) sont aussi signés avec une clé du membre qui les envoie, épinglée à la première vue ; voir [Modèle de sécurité](configuration.md#security-model).

**Ce qui circule dans le groupe.** L'état de protection de la page Instances, une demande de vérifier une section maintenant, les demandes de rôle et leurs réponses, les offres de destination et ce dont un récepteur ou un rapatrieur a besoin une fois son rôle autorisé : les emplacements des dépôts de l'autre instance et son mot de passe restic. Les données de sauvegarde jamais ; elles vont toujours directement aux backends restic. L'APP_KEY non plus : le mot de passe restic ouvre les dépôts de cette instance et rien d'autre, ni ses secrets stockés, ni ses sessions, ni ses codes de déblocage.

**Éléments d'avant l'appairage.** Les instances ajoutées avec un jeton de flotte, ainsi que les dépôts reçus et les sources de rapatriement configurés avec l'APP_KEY de l'autre instance, restent après la mise à jour et sont marqués **Appairer à nouveau**. Ils continuent de fonctionner : au premier démarrage, BombVault remplace chaque APP_KEY stockée par le mot de passe restic qui en est dérivé. Appairez les deux instances, puis modifiez l'élément et choisissez son instance. D'ici là, ces éléments figurent dans la carte **Sans instance** de la page Instances.

Le seul endroit qui accepte encore une APP_KEY à la main est [Depuis un ancien dossier de sauvegarde](#restore-from-another-bombvault-repo), pour le cas où l'autre instance a disparu et ne peut pas répondre dans un groupe.

## Instances et rôles {#roles}

La page **Instances** (une tuile de paramètres qu'on peut épingler à la barre latérale) montre le groupe comme un réseau de qui fait quoi pour qui, avec une carte pour cette instance et une carte pour chaque autre membre, les téléphones Android et les serveurs ZFS. **Appairage** en bas à droite mène à la page d'appairage. Les parties activées se règlent sous **Paramètres, Instances, Plusieurs serveurs** : **Instances**, **Récepteur** et **Rapatriement**.

Chaque carte montre l'état de protection du membre et ses rôles sous forme de tuiles. Un rôle est quelque chose qu'une instance fait pour une autre :

- **Récepteur** garde les copies hors site de ce serveur dans un rest-server en mode append-only. Ce serveur envoie après chaque sauvegarde et ne peut rien y supprimer, même si quelqu'un le prend en main.
- **Rapatrieur** rapatrie les sauvegardes de ce serveur selon son propre planning. Ce serveur n'a pas besoin d'être éveillé pour cela.
- **Serveur ZFS** accepte les répliques ZFS de ce serveur. Chaque élément ZFS le choisit sous **Réplique** dans ses paramètres.
- **Gardien** garde une copie scellée du kit de récupération de ce serveur. Voir [Gardien](#key-keeper).

Activer un rôle envoie une demande, et rien ne démarre tant que quelqu'un sur l'autre instance n'a pas appuyé sur **Autoriser**. La demande apparaît sur la carte de cette instance sous forme de badge, et ses **Détails** la listent avec **Autoriser** et **Refuser** ; un rôle autorisé peut y être révoqué à tout moment. Une ligne en pointillés dans le réseau est un rôle qui attend encore. Une instance sous BombVault 9.x ne peut pas répondre à une demande, donc un rôle y est actif dès que vous l'activez, et une demande d'une instance 9.x attend **Autoriser** ici.

**Détails** sur une carte ouvre le reste : les dépôts reçus et les sources de rapatriement de ce membre, les offres de destination, la surveillance, **Supprimer** et **Oublier la clé**.

## Récepteur et rapatrieur {#receiver}

![Une copie hors site reçue dans les détails d'une instance, surveillée en lecture seule, avec un contrôle d'intégrité exécuté sur ce matériel.](assets/screenshots/receiver.png)

*Une copie hors site reçue dans les détails d'une instance, surveillée en lecture seule, avec un contrôle d'intégrité exécuté sur ce matériel.*

Sur la machine qui **reçoit** des copies append-only d'un autre BombVault, les dépôts reçus vous donnent une surveillance indépendante, en lecture seule, sur le matériel récepteur, de sorte qu'une défaillance silencieuse à l'autre bout ne passe pas inaperçue. Activez **Récepteur** sous **Paramètres, Instances**, puis ouvrez les **Détails** de l'instance émettrice sur la page Instances et ajoutez le dépôt reçu (en lecture seule, ouvert avec le mot de passe restic de l'instance émettrice, qui lui parvient par le [groupe d'appairage](#pairing)). Vous obtenez :

- **Un inventaire par source**, pour voir exactement quels conteneurs, VMs et jeux de dossiers sont arrivés.
- **Dernière réception** par source, pour savoir à quel point chacune est fraîche.
- **Un `restic check` indépendant** exécuté sur le matériel de réception, afin que l'intégrité soit vérifiée là où les données se trouvent réellement, pas seulement sur l'émetteur.
- **Un dispositif d'homme mort :** une alerte lorsqu'une source cesse d'émettre dans une fenêtre que vous définissez.
- **Des alertes d'intégrité :** une alerte lorsqu'une vérification côté réception échoue.

Le récepteur est strictement en lecture seule. Il n'écrit jamais dans le dépôt reçu, donc il ne peut jamais rompre la garantie append-only sur laquelle compte l'émetteur.

Un rapatrieur fonctionne dans l'autre sens : avec **Rapatriement** activé, une instance lit les sauvegardes d'un autre membre dans son propre dépôt selon son propre planning. Ses sources figurent aussi dans les détails de ce membre.

### Serveur récepteur {#receiving-server}

La machine réceptrice peut aussi exécuter le rest-server vers lequel les autres copient. Il se met en place la première fois que vous autorisez une demande de **Récepteur** qui le demande : la fenêtre dit que le serveur a besoin d'un serveur récepteur et demande un dossier sur un partage, avec **Nouveau dossier** pour en créer un, et un port (8000 sauf si un autre conteneur l'utilise). BombVault alors :

1. refuse de continuer si un conteneur nommé `rest-server` existe déjà ou si un autre conteneur occupe le port ;
2. télécharge `restic/rest-server` et le démarre via le socket Docker en mode append-only, avec des dépôts privés et un fichier d'identifiants dans le dossier ;
3. écrit son modèle Unraid sur la clé flash, afin que le conteneur reste modifiable dans l'onglet Docker, ou propose le modèle en téléchargement quand la clé flash est hors d'atteinte ;
4. lance le test de sabotage contre lui et indique s'il refuse les suppressions.

Les instances de votre groupe trouvent ensuite le serveur parmi les fournisseurs sous **Depuis votre groupe**, nommé d'après la machine réceptrice, et le rôle **Récepteur** le propose comme choix sûr. Chaque instance reçoit un identifiant à elle la première fois qu'elle utilise le serveur, et n'y écrit que dans son propre dossier. Avec **Récepteur** activé sous **Paramètres, Instances**, la carte **Serveur récepteur** de cette instance rouvre le serveur. Sa fenêtre liste les identifiants sous **Partenaires avec identifiant**, et **Révoquer l'identifiant** en retire un ; ce que cette instance a déjà copié reste dans le dossier. La mise en place crée aussi un **Identifiant hors du groupe**, dont la fenêtre montre le mot de passe une seule fois.

Une instance qui n'atteint la machine réceptrice que par le relais ne peut pas utiliser le serveur, car le relais ne transporte pas de sauvegardes. Ajoutez d'abord l'adresse de la machine réceptrice sur la page d'appairage. Quand BombVault tourne sur une adresse IP à lui (sur br0, par exemple), renseignez **Adresse pour les partenaires**, car le serveur écoute sur l'adresse de l'hôte.

## Exemple complet : deux machines Unraid, de bout en bout

Tout ce qui précède décrit les pièces. Voici une mise en place complète, car les pièces sont plus faciles à assembler quand on les a vues assemblées une fois.

Deux machines : **TOWER** exécute les conteneurs et pousse les sauvegardes ; **VAULT** les reçoit et impose l'append-only. Remplacez par vos propres noms, adresses et chemins de partage.

**1. Appairez les deux machines.** Sur TOWER, ouvrez **Paramètres, Appairage**, appuyez sur **Générer une phrase** et saisissez les douze mots sur VAULT avec **Saisir une phrase**. Définissez d'abord un mot de passe de connexion sur les deux.

**2. Sur TOWER, demandez à VAULT d'être le récepteur.** Sur la page Instances, activez **Récepteur** sur la carte de VAULT, choisissez les sections et **le serveur récepteur de VAULT** comme endroit, et envoyez la demande.

**3. Sur VAULT, autorisez-la.** La page Instances de VAULT montre la demande sur la carte de TOWER. Appuyez sur **Autoriser**. La première fois, VAULT demande un dossier pour son serveur récepteur, tel que `user/appdata/rest-server`, démarre le rest-server en mode append-only et exécute le test de sabotage contre lui.

**4. De retour sur TOWER.** À partir de là, TOWER copie vers VAULT après chaque sauvegarde, et la nouvelle destination apparaît sur la page Destinations de TOWER avec sa protection contre la suppression.

Ce que répond le test de sabotage :

| Résultat | Ce qui s'est passé |
| --- | --- |
| **protégé** | VAULT a refusé la suppression. C'est le seul état qui passe. |
| **non protégé** | VAULT a accepté une suppression. `--append-only` est absent ou a été retiré. |
| **non concluant** | Ni l'un ni l'autre. En général, l'URL n'est pas celle qu'utilise restic lui-même, ou les identifiants ont changé. Rien n'est enregistré et aucune alerte n'est déclenchée. |

**5. Surveillez ce qui arrive, si vous le souhaitez.** Activez **Récepteur** sous **Paramètres, Instances** sur VAULT, ouvrez les **Détails** de TOWER et ajoutez le dépôt reçu. VAULT reçoit le mot de passe restic de TOWER par le groupe ; personne ne saisit de clé.

**6. Rendez-le mutuel.** Répétez les étapes dans l'autre sens : VAULT demande à TOWER d'être son récepteur. Chaque machine impose alors l'append-only pour l'autre, et aucune ne peut supprimer les sauvegardes de l'autre.

Un rest-server que vous lancez à la main fonctionne aussi : ajoutez-le avec **Ajouter une destination** comme rest-server, avec l'utilisateur et le mot de passe de son `.htpasswd`, et activez la protection contre la suppression. Gardez `--append-only` dans ses options ; sans lui, l'autre côté redevient un partage ordinaire.

!!! warning "Un chemin local est un chemin à l'intérieur du conteneur"
    Là où BombVault demande un dossier de ce serveur, saisissez-le relativement au montage de l'hôte, tel que `user/appdata/rest-server/bombvault-containers`, pas `/mnt/user/appdata/...`. BombVault tourne dans un conteneur, où le `/mnt` de l'hôte est monté ailleurs ; un chemin absolu de l'hôte n'existe pas à l'intérieur. Si vous en collez un, BombVault vous dit le chemin relatif à utiliser à la place.

## Assistant d'urgence {#emergency}

L'**Assistant d'urgence** ramène un serveur disparu ou réinstallé. Il démarre sur la page **Restauration** sous **Urgence**, et depuis **Restaurer depuis une sauvegarde** sur la page de bienvenue d'une nouvelle installation. Il se déroule en quatre étapes : **Accès**, **Paramètres**, **Sélection** et **Restauration**.

1. **Accès.** Montrez à BombVault la clé des anciennes sauvegardes, de l'une de trois façons :
    - **Téléverser le kit de récupération** : le fichier que vous avez téléchargé lors de la mise en place. Il contient la clé et chaque destination, et BombVault remplit le reste à partir de lui. Un kit scellé avec age demande l'identité pour laquelle il a été scellé ; elle est utilisée une fois et n'est pas stockée.
    - **Via le groupe** : une instance de votre groupe garde la clé en copie déposée. Il vous faut les douze mots du groupe, votre code de déblocage et quelqu'un sur cette instance. Voir [Gardien](#key-keeper).
    - **Saisir la clé** : l'`APP_KEY` de l'ancien serveur, et l'endroit où se trouvent ses sauvegardes.

    BombVault essaie ensuite la clé sur chaque destination et dit pour chacune si elle est lisible, introuvable, verrouillée par une autre clé, injoignable ou s'il lui manque encore la connexion. Rien n'a encore été enregistré, et la clé reste en mémoire un temps limité. **Reprendre la clé** fait écrire à ce serveur ses sauvegardes avec cette clé désormais ; **Annuler la reprise** revient à sa propre clé jusqu'à ce que la première sauvegarde ait tourné. Un serveur qui a déjà ses propres sauvegardes refuse la reprise et renvoie vers [Depuis un ancien dossier de sauvegarde](#restore-from-another-bombvault-repo). **Enregistrer les identifiants** stocke les identifiants que porte le kit pour les destinations distantes, et demande avant de remplacer des identifiants que ce serveur a déjà.
2. **Paramètres.** **Restaurer les réglages** restaure les réglages propres de BombVault depuis l'auto-sauvegarde la plus récente : destinations, identifiants, plannings et règles. BombVault redémarre pour les appliquer, via le socket Docker, de sorte que la base de réglages en service n'est jamais écrasée sous un descripteur ouvert, et l'assistant continue de lui-même ensuite. La connexion de ce serveur reste : vous continuez à vous connecter avec le mot de passe que vous utilisez maintenant. Les secrets que l'ancien serveur avait stockés et qui ne peuvent pas être repris sont listés, pour que vous puissiez les saisir de nouveau.
3. **Sélection.** BombVault parcourt vos sauvegardes et liste les conteneurs, VMs, jeux de dossiers et jeux de données ZFS qu'il trouve. La sauvegarde la plus récente est présélectionnée partout. Ce que ce serveur a encore commence désactivé, car une restauration le remplacerait. Les jeux de dossiers ne savent plus d'où ils venaient après une reconstruction, donc ils vont dans un dossier que vous choisissez.
4. **Restauration.** Tout ce qui est sélectionné revient d'un coup, laissé arrêté si vous voulez démarrer vous-même chaque conteneur et VM, ce qui est le plus sûr après une perte totale. Téléchargez ensuite un nouveau kit de récupération : l'ancien appartient à l'installation précédente.

!!! tip "Migration planifiée versus sinistre"
    L'Assistant d'urgence restaure les réglages propres de BombVault depuis une sauvegarde. Pour un déménagement *planifié* vers une nouvelle machine, vous pouvez à la place emporter directement votre configuration avec la carte **Exporter / importer les paramètres** (un fichier JSON portable). Voir [Configuration](configuration.md#portable-settings-export-and-import).

### Depuis un ancien dossier de sauvegarde {#restore-from-another-bombvault-repo}

**Depuis un ancien dossier de sauvegarde** sous **Autres sources** sur la page Restauration ouvre le dépôt d'une *autre* instance BombVault (un partage monté sous `/mnt`, ou une adresse distante) avec le kit de récupération de cette instance ou son `APP_KEY` saisie, dans une session unique en lecture seule. Parcourez les conteneurs, VMs, jeux de dossiers et jeux de données ZFS qui y sont stockés, choisissez une sauvegarde et restaurez-la, et l'objet restauré devient un conteneur, une VM ou un jeu de dossiers local ordinaire. Rien n'est jamais écrit dans l'autre dépôt, et vos propres réglages de sauvegarde restent intacts (la session vit en mémoire et expire d'elle-même). Déplacer un conteneur du serveur A au serveur B ne signifie pas repointer vos réglages de dépôt puis les rétablir ensuite. Cette fenêtre est à usage unique : elle ouvre une session, restaure ce que vous choisissez et oublie l'autre instance. Si vous voulez plutôt un arrangement permanent, où cette machine rapatrie selon un planning les sauvegardes d'une autre instance dans son propre dépôt, c'est le rôle **Rapatrieur** sur la page Instances.

Un conteneur dont le réseau n'existe pas sur ce serveur, par exemple un réseau `br0` d'Unraid sur un hôte Docker ordinaire, affiche un choix de réseau sous sa ligne. BombVault le crée sur le réseau que vous choisissez, avec ses autres réseaux. L'adresse IP fixe et l'adresse MAC appartenaient à l'ancien réseau et disparaissent, c'est donc le nouveau réseau qui les attribue.

## Gardien {#key-keeper}

Un gardien est une instance de votre groupe qui détient une copie scellée du kit de récupération de ce serveur. Il ne peut pas ouvrir la copie lui-même. Si ce serveur est un jour perdu, une nouvelle installation qui rejoint votre groupe demande la copie au gardien et l'ouvre avec votre code de déblocage.

1. Sur la page Instances, activez **Gardien** sur la carte de l'instance qui doit la garder, et choisissez **Déposer la copie**. Faire un dépôt exige un mot de passe de connexion.
2. Ce serveur affiche le **Code de déblocage** une seule fois : vingt lettres et chiffres en quatre groupes de cinq. Notez-le et confirmez avec **J'ai noté le code**. Ce serveur ne stocke pas le code, seulement ce dont il a besoin pour sceller un dépôt pour lui.
3. Le gardien détient maintenant la copie, et ce serveur la renouvelle quand le kit change. **Copie déposée** sur la carte de cette instance liste **Copie déposée de ce serveur** avec chaque gardien et son état, et **Copies déposées que ce serveur garde** pour celles des autres. **Créer un nouveau code** remplace le code ; chaque gardien passe au nouveau quand il reçoit la nouvelle copie.

Ce jour-là, vous avez besoin de trois choses : les douze mots de votre groupe, le code de déblocage, et l'accès à l'interface du gardien, où quelqu'un doit appuyer sur **Autoriser**. Dans l'Assistant d'urgence, choisissez **Via le groupe**, choisissez la copie déposée, demandez-la et saisissez le code de déblocage. Le code est utilisé sur le nouveau serveur et n'est pas envoyé au gardien. Sur le gardien, la demande apparaît sous **Copie déposée** avec qui la demande, si cette instance a déjà été vue là-bas et combien de fois elle a demandé ; **Autoriser** remet la copie scellée, et autoriser exige un mot de passe de connexion. La nouvelle installation a 30 minutes pour la récupérer après l'autorisation, une demande à laquelle personne ne répond expire au bout d'un jour, et une instance qui demande trop souvent doit attendre une heure.

Sans le code, personne ne peut ouvrir la copie, le gardien compris. Si le code est perdu, créez-en un nouveau tant que ce serveur fonctionne encore ; si le serveur a déjà disparu, le kit de récupération et l'APP_KEY saisie fonctionnent toujours. Le gardien peut supprimer une copie déposée à tout moment, et le propriétaire doit alors la redemander.

## Kit de récupération {#recovery-kit}

C'est la pièce qui rend la reprise après sinistre possible même lorsqu'il n'y a aucun BombVault en fonctionnement.

**Télécharger le kit de récupération** dans la carte **En cas d'urgence** sous **Paramètres, Restauration** vous donne la **clé**, le **mot de passe restic dérivé**, chaque **destination** avec le dépôt de chaque section, les identifiants dont une nouvelle installation a besoin pour les atteindre et les **commandes restic exactes**, pour que vous puissiez restaurer directement avec la CLI restic sur n'importe quelle machine. Il exige un mot de passe de connexion. Le kit imprime la clé aussi quand le chiffrement est désactivé, car les définitions de conteneurs et de VMs sont toujours scellées avec elle. Après une reprise, il imprime la clé de sauvegarde reprise et l'APP_KEY de ce serveur, dont une nouvelle installation n'a besoin que pour ramener les réglages. Une carte **Sauvegardez votre kit de récupération** sur l'aperçu vous le rappelle tant que vous ne l'avez pas stocké.

!!! danger "Conservez le kit de récupération hors du serveur"
    Le kit contient le secret qui déchiffre vos sauvegardes. Gardez-le en lieu sûr et à l'écart du serveur (un gestionnaire de mots de passe, une copie imprimée dans un coffre). Si vous perdez à la fois BombVault et `APP_KEY` sans kit de récupération et sans copie déposée, vos sauvegardes chiffrées ne peuvent pas être récupérées.

!!! warning "Le snapshot le plus récent n'est pas toujours celui à restaurer"
    Depuis restic 0.17, `restic snapshots` montre la taille de chaque instantané. Après une perte de données, l'instantané le plus récent peut être celui qui a été vidé, donc ne restaurez pas un instantané bien plus petit que les précédents. Après un ransomware, ce peut être celui qui est chiffré, à la taille habituelle. Si BombVault fonctionne encore, regardez d'abord sa page **Anomalies** : elle nomme la dernière bonne sauvegarde, et la fenêtre de restauration la présélectionne. Une restauration n'a besoin d'aucune donnée d'anomalie de BombVault, et la pause de rétention ne fait que conserver plus d'instantanés.

### Sceller le kit

Si vous avez activé **Chiffrer les exports avec age** sous **Paramètres, Système**, le kit est scellé avec elle aussi et se télécharge sous `bombvault-recovery-kit.md.age`. Il est en ASCII blindé plutôt qu'en binaire, donc c'est encore du texte : le coller dans un gestionnaire de mots de passe ou l'imprimer fonctionne exactement comme avant, le contenu est simplement illisible sans votre clé.

!!! warning "Ne rangez pas la clé age dans le kit"
    Il vous faut votre clé age **privée** pour ouvrir un kit scellé. Gardez-la à un endroit qui ne dépend pas du kit lui-même, sinon vous aurez deux choses à récupérer au lieu d'une. Sceller le kit vaut la peine quand il est stocké à un endroit que vous ne contrôlez pas entièrement (un gestionnaire de mots de passe partagé, des notes dans le cloud, une impression au bureau) ; un kit dans votre propre coffre est déjà protégé par le coffre.

    Avec le chiffrement activé et aucun destinataire utilisable configuré, le téléchargement est refusé d'emblée. BombVault ne se rabat jamais sur la remise de la clé maîtresse en clair.

### Si le kit n'est pas sous la main

Le mot de passe n'est stocké nulle part, il est **calculé** à partir de la clé des sauvegardes, de sorte que vous pouvez le reproduire vous-même avec rien d'autre que la clé et un shell :

```sh
printf 'bombvault:restic-repo' \
  | openssl dgst -sha256 -mac HMAC -macopt hexkey:$APP_KEY -r \
  | cut -d' ' -f1
```

C'est un HMAC-SHA256 sur la chaîne fixe `bombvault:restic-repo`, avec pour clé les octets bruts de l'`APP_KEY` hexadécimale, imprimé sous forme de 64 caractères hexadécimaux en minuscules. La même valeur figure dans le kit, listée comme mot de passe restic dérivé ; ceci est pour le jour où le kit se trouve là où vous n'êtes pas. Sur un serveur qui a repris les sauvegardes d'un autre, utilisez la clé de sauvegarde reprise, qui est l'ancienne APP_KEY de ce serveur.

!!! warning "Pour un dépôt reçu, utilisez la clé de l'instance ÉMETTRICE"
    Un dépôt arrivé ici par une copie hors site a été créé par la machine qui l'a envoyé, avec **sa** clé. Dériver de la clé de la machine réceptrice produit un mot de passe que restic rejettera, ce qui ressemble exactement à un dépôt corrompu et ne l'est pas. C'est la raison habituelle pour laquelle `restic check` sur un dépôt reçu redemande le mot de passe encore et encore.

Parce que les définitions de récupération vivent **à l'intérieur** de chaque dépôt (`<repo>/def`, `<repo>/vm-def`), un dossier de dépôt copié est entièrement autonome, de sorte que le kit plus le dépôt sont tout ce dont une restauration sur machine nue a besoin.

## Récupérer un dump de base de données {#database-dumps}

Un dump de base de données est un point de restauration à part dans le dépôt des conteneurs, étiqueté `dbdump:<conteneur>`, contenant l'unique fichier `/dbdump/<conteneur>.sql`. BombVault les liste, les télécharge et les importe dans l'onglet **Sauvegardes** du conteneur ; voici les mêmes étapes avec restic seul, pour le jour où BombVault n'est plus là.

```sh
restic -r <repo> snapshots --tag dbdump:<container>
restic -r <repo> dump --tag dbdump:<container> latest /dbdump/<container>.sql > <container>.sql
```

Les étiquettes `dbversion:` et `dbname:` de chaque dump indiquent la version du serveur d'où il vient et les bases qu'il contient. Un fichier complet se termine par `-- PostgreSQL database cluster dump complete` ou `-- Dump completed`.

Importez-le dans un conteneur de la même version ou d'une plus récente (PostgreSQL), ou de la même version majeure (MySQL et MariaDB), démarré une fois avec un dossier de données vide pour qu'il s'initialise. L'hôte n'a besoin d'aucun client de base de données, le conteneur en a un :

```sh
docker exec -i <container> sh -c 'exec psql -X -U "${POSTGRES_USER:-postgres}" -d postgres' < <container>.sql
docker exec -i <container> sh -c 'exec mariadb -uroot -p"$MARIADB_ROOT_PASSWORD"' < <container>.sql
docker exec -i <container> sh -c 'exec mysql -uroot -p"$MYSQL_ROOT_PASSWORD"' < <container>.sql
```

Pour une seule base issue d'un dump complet, MySQL et MariaDB acceptent `--one-database <name>` sur la commande du client. Un dump PostgreSQL comporte une section par base, chacune commençant par une ligne `\connect <name>` : copiez cette section dans un fichier à part et importez-le avec `-d <name>` après avoir créé la base.

!!! warning "Un dump pris en root emporte les comptes du serveur"
    Un dump complet MySQL ou MariaDB pris en root contient la base système `mysql` : l'importer remplace donc les comptes du nouveau serveur, mot de passe root compris, par ceux du dump. Sur PostgreSQL, `role ... already exists` pour l'utilisateur créé par le conteneur est attendu et sans conséquence.
