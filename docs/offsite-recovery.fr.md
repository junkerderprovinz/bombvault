# Sauvegarde hors site et récupération

!!! note "Les copies hors site attendent après une reconstruction"
    Quand l'étape 4 reconstruit des entrées sans les anciens réglages, la réplication hors site de ces domaines se met en pause jusqu'à ce que l'emplacement par défaut soit confirmé. Voir [Emplacement par élément](#placement).

Les sauvegardes locales vous protègent d'un conteneur perdu ou d'une mauvaise mise à jour. La réplication hors site et un kit de récupération testé vous protègent de la perte de toute la machine, d'un rançongiciel ou d'un incendie. Cette page couvre la réplication hors site, l'inviolabilité de cette copie, la preuve que vous pouvez restaurer, et la récupération lorsque BombVault lui-même a disparu.

## Réplication hors site

Conservez la sauvegarde locale rapide et ajoutez un ou plusieurs réplicas hors site. Définissez un dépôt par domaine dans la page **Paramètres, Hors site**. BombVault y réplique les nouveaux instantanés avec `restic copy` au mieux, de sorte qu'un accroc hors site ne fait jamais échouer la sauvegarde locale. Sous cette forme, le dépôt local reste primaire et le dépôt hors site en est un réplica, mais le dépôt primaire d'un domaine n'a pas du tout besoin d'être local ; voir [Dépôts primaires distants](#remote-primary-repositories) ci-dessous pour sauvegarder directement vers S3, rest-server, etc. au lieu d'y répliquer.

- **Plusieurs cibles hors site par domaine.** Chaque domaine (conteneurs, VMs, flash, config, jeux de fichiers et jeux de données ZFS) peut répliquer vers plusieurs destinations hors site à la fois, pas seulement une, de sorte que vous pouvez garder, par exemple, un rest-server sur la machine d'un ami et un bucket S3 en parallèle. Ajoutez des cibles supplémentaires dans Paramètres, Hors site, chacune avec son propre dépôt, sa classe de stockage S3, son indicateur append-only, sa rétention et son budget de croissance. Une configuration hors site unique existante est reprise intacte comme première cible, et chaque cible d'un domaine réplique selon le planning hors site de ce domaine.
- **Planning hors site par domaine** (édité aux côtés de tous les autres plannings dans Paramètres, Plannings) : laissez-le vide pour répliquer après chaque sauvegarde locale, ou définissez une cadence (par exemple `weekly Sun 03:00`) pour expédier hors site moins souvent que vous ne sauvegardez localement. Un bouton **Répliquer maintenant** couvre les exécutions à la demande.
- **La rétention hors site** vit dans Paramètres, Rétention afin que vous puissiez garder les copies hors site plus longtemps comme archive. Laissez la politique entièrement à zéro pour ne jamais rogner automatiquement les instantanés hors site.
- **Les limites de bande passante** (Paramètres, Hors site) plafonnent le débit d'envoi/de téléchargement de restic afin que la réplication ne sature pas votre WAN.
- Un **indicateur de réplication** montre quel domaine réplique pendant qu'elle s'exécute (sur sa page et le tableau de bord). C'est un indicateur d'activité, pas une barre de pourcentage, car `restic copy` n'expose aucune progression lisible par machine.

!!! note "Restaurer depuis n'importe quel endroit"
    Chaque conteneur, VM, jeu de fichiers, le flash et la configuration de l'application listent leurs sauvegardes comme une seule chronologie à travers tous les endroits où se trouve une sauvegarde. Une sauvegarde copiée vers B2 apparaît une fois, marquée avec chaque endroit qui la détient. Une restauration prend le premier endroit qu'elle peut atteindre, en commençant par le dépôt où l'élément est écrit, et vous pouvez choisir un autre endroit par ligne. Les endroits hors site ne sont lus que lorsque vous les ouvrez. Supprimer à un endroit vérifie d'abord les autres et dit si c'était la dernière copie.

## Destinations {#destinations}

Paramètres, Hors site commence par **Destinations** : les endroits où vont les copies hors site, configurés une seule fois pour tous les domaines. Une destination apparaît ensuite comme un bouton dans la ligne **Emplacement** de chaque domaine et de chaque élément. La première fois qu'elle est activée pour un domaine, BombVault crée le dépôt de ce domaine dans un dossier placé dessous, par exemple `rclone:onedrive:BombVault/containers`.

**Ajouter une destination** ouvre un assistant en cinq étapes :

1. **Où envoyer les sauvegardes ?** Chaque service est listé avec son logo, en quatre groupes : les services de stockage avec des buckets S3 (Backblaze B2, Wasabi, Cloudflare R2, Hetzner Object Storage, Amazon S3 et d'autres), votre propre serveur S3 (Garage, SeaweedFS, RustFS, Ceph, JuiceFS, Versity S3 Gateway), votre propre serveur et vos partages (rest-server, Hetzner Storage Box, SFTP, SMB, WebDAV, un chemin monté) et le stockage cloud (OneDrive, Google Drive, Dropbox, pCloud, Nextcloud et tout ce que rclone prend en charge). Chacun indique dans quelle mesure il convient aux sauvegardes : les disques cloud ralentissent sous un grand nombre de requêtes, la première sauvegarde et l'élagage y prennent donc plus de temps.
2. **Se connecter.** Les champs dépendent du service : une clé d'accès pour S3, un utilisateur et un mot de passe pour WebDAV et SMB, un mot de passe d'application quand l'authentification à deux facteurs bloque le mot de passe normal, la clé SSH publique de BombVault pour SFTP et la Storage Box, ou un jeton pour les services qui se connectent par le navigateur. Pour ces derniers, l'assistant affiche une commande `rclone authorize` à lancer sur un ordinateur doté d'un navigateur ; le jeton qu'elle affiche se colle dans le champ. **Tester la connexion** vérifie l'identification avant que quoi que ce soit soit enregistré.
3. **Choisir un dossier.** L'assistant liste les dossiers de la destination, avec **Nouveau dossier** pour en créer un et l'espace libre quand le service l'indique. Un dossier vide est le plus sûr.
4. **Protection contre la suppression.** L'assistant dit clairement ce que le service peut faire. Un rest-server en mode append-only refuse la suppression, et le test de sabotage le vérifie. Un bucket S3 peut conserver d'anciennes versions grâce au versionnage et au verrouillage d'objet, ce que BombVault ne sait pas encore vérifier. Un disque cloud ne peut pas du tout refuser la suppression : qui accède au serveur accède aussi à cette copie. N'activez **Immuable (append-only)** que là où l'autre bout refuse réellement la suppression ; BombVault n'y élague alors jamais.
5. **En cas d'urgence.** Le kit de récupération liste chaque destination avec le dépôt de chaque domaine placé dessous. L'identification revient avec la sauvegarde des réglages de BombVault ; sur une installation neuve sans elle, configurez de nouveau la destination au même endroit.

Les services S3 passent par le backend S3 propre à restic, ce qui permet d'appliquer une classe de stockage et le verrouillage d'objet. Tous les autres services passent par le rclone fourni avec BombVault, et leur remote apparaît ensuite dans la config rclone sous Paramètres, Accès cloud. Un export des réglages contient les destinations ; avec les identifiants inclus, il contient aussi leur identification.

Un serveur récepteur qu'une autre instance de votre groupe exécute apparaît dans l'assistant sous **Depuis votre groupe** ; voir [Serveur récepteur](#receiving-server).

La cible d'un domaine créée à partir d'une destination reprend le nom, l'emplacement, les identifiants, la classe de stockage et le réglage Immuable de la destination. Sa rétention, sa compression et son budget de croissance restent propres à chaque domaine, et son emplacement ne peut pas changer, parce que le dépôt du domaine s'y trouve. **Ajouter une cible pour ce domaine uniquement**, sous chaque domaine, accepte toujours une URL de dépôt saisie à la main.

## Emplacement par élément {#placement}

Chaque carte de conteneur, VM et jeu de fichiers a une ligne **Emplacement** de boutons : **Local** et un bouton par cible hors site du domaine, suivis des destinations sous lesquelles le domaine n'a pas encore de cible. Les boutons allumés reçoivent les sauvegardes de l'élément.

- Avec **Local** allumé, l'élément est écrit dans le dépôt indiqué sous **Stocké sur** et copié vers toutes les autres cibles allumées. Éteignez une cible et elle ne reçoit plus rien de nouveau de cet élément. Local seul ne copie nulle part, ce qui convient à des données qui ont déjà une seconde copie, par exemple un partage qui vit sur un NAS.
- Avec **Local** éteint, l'élément est écrit directement dans le dépôt direct de la première cible allumée, puis copié de là vers les autres cibles allumées. La première fois, une boîte de dialogue crée ce dépôt direct.
- Un bouton de destination crée la cible du domaine sous la destination et l'allume pour cet élément seulement. Tous les autres éléments commencent sans copie à cet endroit.
- Un bouton reste allumé, car une sauvegarde a besoin d'un endroit où aller. Pour laisser quelque chose hors des sauvegardes, excluez-le.

L'emplacement est fixé dès la première sauvegarde de l'élément, parce que BombVault ne déplace jamais les sauvegardes entre dépôts. Les copies peuvent changer à tout moment. Une cible qui ne reçoit plus un élément conserve les copies qu'elle a et les ramène à sa propre rétention à la prochaine exécution hors site du domaine ; **Supprimer dans B2** sur la carte les supprime aussitôt. Quand certaines de ces copies n'existent nulle part ailleurs, la confirmation les liste par date et demande le nom de l'élément. On ne peut rien supprimer des cibles en ajout seul.

Sous la ligne, la carte dit où va l'élément et ce qui s'y trouve réellement : combien de sites le détiennent, quand chaque cible a été vue pour la dernière fois, et si 3-2-1 est respectée. Un site est le serveur avec les données d'origine, chaque cible hors site et chaque dépôt marqué **Hors des locaux**. BombVault vérifie les copies et les sites ; il ne vérifie pas la partie « deux supports » de 3-2-1.

### Emplacements par défaut

Paramètres, Stockage, **Emplacements par défaut** a une ligne par domaine avec les mêmes boutons. Les copies s'appliquent aussitôt à chaque élément sans choix propre, et aux dossiers de projet des piles Compose. L'emplacement s'applique à un nouvel élément à sa première sauvegarde ; le changer ne déplace aucune sauvegarde. Avant d'enregistrer, la ligne nomme chaque cible qui gagne ou perd des éléments et combien d'instantanés cela représente. **Appliquer aux éléments sans sauvegarde** remet au défaut chaque élément qui n'a pas encore de sauvegarde.

Une nouvelle cible hors site reçoit chaque élément qui n'est pas réglé sur Local. La boîte de dialogue qui l'ajoute dit combien d'éléments et, quand c'est connu, combien d'historique cela représente, et propose de laisser de côté les éléments déjà exclus des autres cibles.

### Dépôts directs

Éteindre Local pour un élément, de sorte qu'une cible sans dépôt direct devient son foyer, ouvre une boîte de dialogue avec un emplacement suggéré à côté de la cible, par exemple `s3:https://s3.eu-central-003.backblazeb2.com/bucket/containers-direct`, et un test de connexion qui ne crée rien. **Créer et utiliser** crée le dépôt et y pointe l'élément. Un dépôt direct reprend la clé, la classe de stockage, les limites, le réglage append-only et la rétention de la cible, et change avec eux ; la carte Dépôts l'affiche en lecture seule. Quand une nouvelle clé de la cible ne peut pas l'ouvrir, le dépôt direct conserve la clé qu'il a et l'enregistrement le signale. Un élément placé sur un dépôt direct est copié de là vers les autres cibles allumées, jamais vers la cible à laquelle appartient le dépôt. Ses instantanés portent le tag `bv:direct`, et chaque autre passage de rétention les conserve, si bien qu'un dépôt direct qui a perdu son lien avec sa cible ne vieillit jamais selon les règles locales. B2 se joint via son point de terminaison S3, avec l'ID de clé et la clé d'application saisis comme identifiants S3 ; une clé limitée au dossier propre de la cible ne peut pas atteindre le dossier voisin, limitez donc plutôt la clé au dossier au-dessus de la cible.

### Hors des locaux

Un dépôt nommé peut être marqué **Hors des locaux** sur la carte Dépôts. Les dépôts distants démarrent marqués ; désactivez-le pour un rest-server dans le même bâtiment. La marque compte seulement pour les sites et le 3-2-1 sur les cartes. Elle ne change aucune copie.

### Après une reconstruction

Les choix de copie vivent dans les propres réglages de BombVault. Après une reconstruction via Découvrir les sauvegardes sans un `/config` restauré, ils ont disparu, et tout copier renverrait vers B2 les éléments que vous aviez laissés de côté. La réplication hors site de chaque domaine reconstruit se met donc en pause. Le tableau de bord le montre en orange, et Emplacements par défaut propose **Confirmer la valeur par défaut** avec un aperçu de ce que copie la prochaine exécution et les noms dans les sauvegardes qui n'ont pas d'entrée, que vous pouvez laisser de côté à cet endroit. Seule la confirmation met fin à la pause ; importer un fichier de réglages ramène les règles et les valeurs par défaut mais n'y met pas fin.

## Dépôts primaires distants {#remote-primary-repositories}

Le chemin de sauvegarde d'un domaine (Paramètres, Stockage) ne se limite pas à un dossier local : pointez-le directement vers un dépôt restic distant (`s3:...`, `rest:http://hôte:8000/depot`, `sftp:utilisateur@hôte:/depot`, `rclone:remote:bucket/chemin`) et BombVault y sauvegarde directement, sans copie locale séparée ni étape de réplication. C'est une forme vraiment différente de la réplication hors site vue plus haut : là, le dépôt local est primaire et le dépôt hors site en est une archive au mieux ; ici, le dépôt distant **est** le primaire, et c'est la seule copie tant que vous n'ajoutez pas aussi une réplication hors site (ou un second dépôt distant) pour ce domaine.

Chacun des six champs de chemin (Conteneurs, VMs, Flash, Auto-sauvegarde, Dossiers, Jeux de données ZFS) porte juste à côté un commutateur **Local / Distant** :

- **Local** affiche l'explorateur de dossiers habituel.
- **Distant** le remplace par un simple champ d'URL, plus un bouton qui ouvre la même boîte de dialogue de test de connexion et d'identifiants que les destinations hors site, mais configurée pour ce dépôt primaire. Vous y trouvez :
    - **Un test de connexion** contre le chemin réel, avant de vous y fier.
    - **Des limites de bande passante** (envoi et réception) pour qu'une sauvegarde planifiée vers un primaire distant ne sature pas votre liaison WAN : les mêmes options restic `--limit-upload` et `--limit-download` qu'utilise la réplication hors site, appliquées cette fois à la sauvegarde elle-même.
    - **Une protection append-only (immuabilité)**, vérifiée par le même test d'altération actif (une véritable sonde DELETE contre l'autre bout) dont bénéficient les destinations hors site. Activée, elle interdit à BombVault d'élaguer le dépôt lui-même : comme aucune copie locale séparée ne se trouve derrière, les identifiants de cette machine ne doivent pas pouvoir supprimer l'unique copie de la sauvegarde.
    - **Une alerte de budget de croissance**, tirée de la même tendance de taille du dépôt que suit déjà la carte Stockage.

Rien de tout cela n'est obligatoire : un chemin distant saisi à la main, sans réglages de sécurité enregistrés, sauvegarde exactement comme avant (bande passante illimitée, élagage possible, pas d'alerte de budget). La boîte de dialogue de sécurité est là pour le jour où vous voulez les mêmes protections qu'une copie hors site, sans devoir créer une destination hors site rien que pour cela.

!!! note "Les identifiants cloud et REST sont partagés"
    Un dépôt primaire distant s'authentifie avec les mêmes identifiants S3/REST configurés sous Paramètres, Accès cloud, Identifiants cloud partagés. Il n'existe pas de magasin d'identifiants distinct pour les dépôts primaires.

### SMB et WebDAV sans montage sur l'hôte {#smb-webdav}

Paramètres, Accès cloud, rclone propose un formulaire pour un partage Windows ou Samba et pour un serveur WebDAV (Nextcloud, ownCloud, SharePoint ou tout autre). Renseignez un nom court, l'hôte et le partage (SMB) ou l'URL et le type de serveur (WebDAV), l'utilisateur et le mot de passe, et BombVault écrit la section rclone pour vous. rclone masque lui-même le mot de passe avant qu'il soit enregistré ; ajouter une destination avec un nom qui existe déjà remplace cette section au lieu d'en ajouter une seconde.

Le formulaire répond avec l'emplacement final, par exemple `rclone:nas:backups`. Mettez-le dans un Chemin de sauvegarde ou une destination hors site et ajoutez un sous-dossier si vous le souhaitez (`rclone:nas:backups/bombvault`). Le partage est le premier segment du chemin, il ne fait pas partie du nom.

C'est une meilleure voie que de monter le partage sur Unraid : restic déconseille de garder un dépôt sur un partage CIFS monté, et ici rien n'est monté. NFS ne figure pas dans le formulaire, car ni restic ni rclone n'a de backend NFS ; pour NFS, montez l'export sur l'hôte et pointez-y un Chemin de sauvegarde.

## Hors site immuable (append-only)

Marquez un dépôt hors site en append-only afin qu'un rançongiciel, ou un hôte compromis, ne puisse ni supprimer ni réécrire vos sauvegardes. Le côté distant (un `restic/rest-server` s'exécutant en mode `--append-only`) l'**impose**. BombVault ne fait que le **vérifier** et n'affiche jamais du vert sur la seule foi d'une déclaration de configuration.

L'assistant de **configuration hors site guidée** vous accompagne du choix du backend (rest-server / rclone / S3) jusqu'à un extrait de déploiement rest-server prêt à coller, un test de connexion, la bascule d'immuabilité (qui lance le test de sabotage immédiatement) et une stratégie de rétention, de sorte que le hors site append-only soit accessible sans édition manuelle des configs.

!!! note "Une suppression réussie sous `/locks/` est attendue"
    Append-only ne signifie pas que plus rien ne peut être supprimé. restic doit poser et relâcher ses propres verrous, `/locks/` reste donc volontairement accessible en écriture et en suppression. Les snapshots et les données derrière eux, précisément ce que viserait un rançongiciel, ne peuvent pas être supprimés. Si vous sondez vous-même le serveur distant, une suppression qui réussit sous `/locks/` est le comportement correct et non une faille.

!!! warning "Les dépôts immuables ne sont jamais élagués depuis cette machine"
    Un hors site immuable n'élague délibérément jamais les anciens instantanés. Définissez une **alarme de budget de croissance** pour lui afin d'être alerté avant que la taille du dépôt ne s'emballe.

## Test de sabotage

BombVault prouve périodiquement la garantie append-only en tentant réellement une suppression contre le dépôt hors site, visant un objet inexistant :

- **Refusé** signifie protégé.
- **Accepté** signifie non protégé.
- Un résultat **non concluant** (serveur injoignable, erreur d'authentification) ne renverse jamais le verdict stocké.

Un vrai basculement de protégé à non protégé déclenche une alerte unique.

## Essais de reprise après sinistre

BombVault offre deux niveaux de preuve que vos sauvegardes sont réellement restaurables, pas seulement présentes.

- **Essais de vérification de restaurabilité (local).** BombVault exécute périodiquement `restic check --read-data-subset` (borné, jamais une restauration complète qui remplirait le disque) et affiche un badge *Restaurabilité vérifiée* par domaine. La cadence vit dans Paramètres, Plannings ; le badge dans Paramètres, Intégrité.
- **Essais de reprise après sinistre (hors site).** BombVault restaure une vraie cible depuis le dépôt hors site dans un bac à sable jetable, la vérifie fichier par fichier et octet par octet, puis nettoie. Cela prouve que vous pouvez récupérer depuis le hors site, pas seulement que le dépôt répond.

Le **tableau de bord de protection contre les rançongiciels** du tableau de bord synthétise cela en une posture verte / orange / rouge par domaine, avec une liste de contrôle horodatée (hors site configuré, append-only vérifié, réplication à jour, essai de restauration réussi, chiffrement activé, stratégie d'élagage définie). Chaque ligne rouge renvoie directement au correctif, et la carte ne passe au vert que sur des faits vérifiés.

## Appairage des instances {#pairing}

Les Récepteurs, les sources de Rapatriement, la page Instances et le Mesh hors site parlent tous à un autre BombVault. Ils le font en tant que membres d'un même groupe d'appairage, et une instance rejoint le groupe avec douze mots.

Sur la première instance, ouvrez **Paramètres → Appairage** et cliquez sur **Générer une phrase** dans les cartes d'appairage. Douze mots apparaissent dans une fenêtre avec un bouton **Copier**. Sur chaque autre instance, ouvrez le même endroit, cliquez sur **Saisir une phrase** et collez-les ou tapez-les, ou cliquez sur **Coller** dans cette fenêtre. Un mot absent de la liste est signalé avec sa position dès la saisie, et le dernier mot porte une somme de contrôle, si bien qu'un mot mal saisi ou interverti est détecté avant qu'un appairage n'ait lieu. Générez la phrase sur une seule instance : deux instances qui créent chacune une phrase forment deux groupes distincts. Si personne ne se présente au bout d'une minute, l'onglet propose deux façons de s'en sortir : réafficher les mots pour les saisir là-bas, ou saisir les mots de l'autre instance et rejoindre son groupe en une seule étape. L'appairage fonctionne sans mot de passe de connexion, mais définissez-en un : sans lui, quiconque peut ouvrir cette interface web peut lire les mots et obtenir, via le groupe, le mot de passe restic de chaque instance qui s'y trouve. La carte d'appairage le signale tant qu'aucun mot de passe n'est défini. Avec un mot de passe, réafficher la phrase le demande. **Quitter le groupe** fait ressortir une instance.

Quiconque connaît les mots peut rejoindre le groupe, traitez-les donc comme un mot de passe.

**Comment les membres se joignent.** Chaque instance apprend sa propre adresse sur le réseau depuis votre navigateur dès que vous vous connectez, affichée dans la carte du relais comme **Cette instance sur votre réseau** ; corrigez-la là si un reverse proxy ou un port inhabituel se trouve devant. Sur le même réseau, les membres annoncent cette adresse par multicast et se parlent directement, et là où le multicast ne peut pas traverser un réseau de conteneurs, comme le réseau bridge par défaut de Docker, une instance recherche plutôt les autres dans son propre sous-réseau avec un appel signé auquel seul un membre du groupe peut répondre, si bien que l'appairage se termine quand même en quelques secondes sans relais. Si rien n'apparaît, **Vous ne la trouvez pas ?**, sous la carte d'appairage, accepte une adresse saisie à la main, pour un autre sous-réseau ou un port non standard. Les instances sur des réseaux différents passent par un relais, choisi sur le même onglet :

- **Relais du projet** (par défaut) : `parleyport.halleluja.design`, le relais qu'utilise aussi KnightLoader. Rien à configurer.
- **Relais personnel** : le conteneur [**ParleyPort**](https://github.com/junkerderprovinz/parleyport) des Unraid Community Apps, ou l'une de vos instances déjà joignable depuis l'extérieur avec **Servir de relais** activé. Cette instance répond alors sur `/relay/connect` à sa propre adresse, derrière le reverse proxy et le certificat qu'elle possède déjà, et n'y laisse entrer que votre groupe. Saisissez l'adresse du relais sur chaque instance qui doit l'utiliser.
- **Aucun relais** : les membres se trouvent automatiquement uniquement sur le même réseau, et nulle part ailleurs.

**Ce que voit le relais.** Chaque appel entre membres est scellé avec AES-256-GCM sous une clé dérivée des douze mots, et cette clé ne quitte jamais vos instances. Le relais apprend un hash qui regroupe les connexions, pour quelle instance un message est destiné, sa taille et son horodatage. Un appel direct sur le réseau local est scellé de la même façon et signé également, si bien que rien ne dépend du certificat auto-signé que sert une instance.

**Ce qui transite par le groupe.** Les fiches de protection de la page Instances, une demande de vérification immédiate d'un domaine, les offres de stockage hors site du Mesh, et ce dont un Récepteur ou une source de Rapatriement a besoin : les emplacements de dépôt de l'autre instance et son mot de passe restic. Les données de sauvegarde, elles, n'y passent jamais : elles vont toujours directement aux backends restic. L'`APP_KEY` non plus : le mot de passe restic ouvre les dépôts de cette instance et rien d'autre, ni ses secrets stockés, ni ses sessions, ni ses codes de récupération.

**Entrées antérieures à l'appairage.** Les instances ajoutées avec un jeton de flotte, ainsi que les Récepteurs et sources de Rapatriement configurés avec l'`APP_KEY` de l'autre instance, restent après la mise à jour et sont marqués **Appairer à nouveau**. Les Récepteurs et sources de Rapatriement continuent de fonctionner : à son premier démarrage, BombVault remplace chaque `APP_KEY` stocké par le mot de passe restic qui en est dérivé. Appairez les deux instances, puis modifiez l'entrée et choisissez son instance. Une telle instance reprend son ancienne carte dès qu'une instance du même nom apparaît dans le groupe.

Le seul endroit qui prend encore un `APP_KEY` à la main est [Restauration depuis un autre dépôt BombVault](#restore-from-another-bombvault-repo), pour le cas où l'autre instance a disparu et ne peut pas répondre dans un groupe.

## Tableau de bord récepteur (le côté réception)

![Le côté récepteur, surveillé en lecture seule, avec un contrôle d'intégrité exécuté sur cette machine.](assets/screenshots/receiver.png)

*Le côté récepteur, surveillé en lecture seule, avec un contrôle d'intégrité exécuté sur cette machine.*

Tout ce qui précède est le côté *émetteur*. Sur la machine qui **reçoit** des copies hors site immuables d'un autre BombVault, le tableau de bord récepteur vous donne une surveillance indépendante et en lecture seule de ces dépôts sur le matériel de réception, afin qu'une défaillance silencieuse à l'autre bout ne passe pas inaperçue.

Activez la bascule **Récepteur** dans les Paramètres pour révéler un onglet **Récepteur**. Il est désactivé par défaut ; ne l'activez que sur une machine qui reçoit réellement des sauvegardes hors site immuables. Enregistrez ensuite un dépôt reçu (en lecture seule, ouvert avec le mot de passe restic de l'instance émettrice, qu'il obtient via le [groupe d'appairage](#pairing)) pour obtenir :

- **Un inventaire d'instantanés groupé par source**, afin que vous puissiez voir exactement quels conteneurs, VMs et jeux de fichiers sont arrivés.
- **La dernière réception** par source, afin que vous sachiez à quel point chacune est fraîche.
- **Un `restic check` indépendant** exécuté sur le matériel de réception, afin que l'intégrité soit vérifiée là où les données se trouvent réellement, pas seulement sur l'émetteur.
- **Un dispositif d'homme mort :** une alerte lorsqu'une source cesse d'émettre dans une fenêtre que vous définissez.
- **Des alertes d'intégrité :** une alerte lorsqu'une vérification côté réception échoue.

Le récepteur est strictement en lecture seule. Il n'écrit jamais dans le dépôt reçu, il ne peut donc jamais briser la garantie append-only sur laquelle l'émetteur compte.

### Serveur récepteur {#receiving-server}

La machine qui reçoit peut aussi exécuter le rest-server vers lequel les autres copient. **Configurer le serveur récepteur**, en haut de l'onglet **Récepteur**, demande un dossier sur un partage, avec **Nouveau dossier** pour en créer un, et un port (8000, sauf si un autre conteneur l'utilise). BombVault :

1. refuse de continuer si un conteneur nommé `rest-server` existe déjà ou si un autre conteneur occupe le port ;
2. télécharge `restic/rest-server` et le démarre via le socket Docker en mode append-only, avec des dépôts privés et un fichier d'identifiants dans le dossier ;
3. écrit son modèle Unraid sur la clé flash, afin que le conteneur reste modifiable dans l'onglet Docker, ou propose le modèle en téléchargement quand la clé flash est hors d'atteinte ;
4. lance le test de sabotage contre lui et indique s'il refuse les suppressions.

Les instances de votre groupe trouvent ensuite le serveur dans l'assistant de destination sous **Depuis votre groupe**, nommé d'après la machine de réception. Chaque instance reçoit son propre identifiant la première fois qu'elle choisit le serveur, et n'y écrit que dans son propre dossier. La carte liste ces identifiants, et **Révoquer l'identifiant** en retire un ; ce que cette instance a déjà copié reste dans le dossier. La configuration crée aussi un identifiant pour quelqu'un hors du groupe, dont la carte montre le mot de passe une seule fois.

Une instance qui n'atteint la machine de réception que par le relais ne peut pas utiliser le serveur, car le relais ne transporte aucune sauvegarde. Ajoutez d'abord l'adresse de la machine de réception sous **Paramètres → Appairage**. Quand BombVault tourne sur une adresse IP à lui (sur br0, par exemple), renseignez **Adresse pour les partenaires**, car le serveur écoute sur l'adresse de l'hôte.

## Exemple complet : deux machines Unraid, de bout en bout

Ce qui précède décrit les pièces. Voici une installation complète avec de vraies valeurs, parce que les pièces s'assemblent plus facilement quand on les a vues assemblées une fois.

Deux machines : **TOWER** fait tourner les conteneurs et envoie les sauvegardes, **VAULT** les reçoit et impose l'immuabilité. Remplacez par vos propres noms, adresses et chemins de partage.

**1. Sur VAULT, mettez en place le serveur append-only.** Dans BombVault sur TOWER, allez dans *Paramètres → Hors site → Configurer*, choisissez **rest-server** et générez la recette. Copiez l'onglet **Modèle Unraid (XML)**, enregistrez-le sur VAULT sous `/boot/config/plugins/dockerMan/templates-user/my-rest-server.xml`, puis *Docker → Add Container* et choisissez **rest-server** dans la liste des modèles. Avant de le démarrer, écrivez la ligne `htpasswd` affichée dans `/mnt/user/appdata/rest-server/.htpasswd` sur VAULT. Le mot de passe à usage unique n'est affiché qu'une fois et n'est jamais conservé : copiez-le maintenant. Cette ligne porte le même mot de passe, déjà haché en bcrypt pour vous : le texte en clair va dans les identifiants REST sur TOWER, la ligne hachée dans le `.htpasswd` sur VAULT. Vous n'avez rien à hacher vous-même.

    Laissez `--append-only` dans le champ OPTIONS. C'est tout l'intérêt : sans lui, VAULT redevient un partage ordinaire.

**2. Sur TOWER, pointez le dépôt hors site dessus.** L'URL du dépôt suit le modèle imprimé par la recette :

    rest:http://VAULT:8000/bombvault-containers/containers

Le premier segment du chemin est l'utilisateur htpasswd, le second le dépôt. Saisissez l'utilisateur et le mot de passe générés comme identifiants REST de la destination, puis lancez le **test de connexion**.

**3. Sur TOWER, activez « Immuable ».** Le test d'altération s'exécute immédiatement et doit indiquer *protégé*. Ce que signifient les réponses :

| Résultat | Ce qui s'est passé |
| --- | --- |
| **protégé** | VAULT a refusé la suppression. C'est le seul état qui passe. |
| **NON protégé** | VAULT a accepté une suppression. `--append-only` manque ou a été retiré. |
| **non concluant** | Ni l'un ni l'autre. En général, l'URL n'est pas celle qu'utilise restic lui-même, ou les identifiants ont changé. Rien n'est enregistré et aucune alerte n'est déclenchée. |

**4. Sur VAULT, regardez ce qui arrive.** Appairez les deux machines ([Appairage des instances](#pairing)), activez *Paramètres → Général → Récepteur*, ouvrez l'onglet **Récepteur** et enregistrez le dépôt en lecture seule avec TOWER comme instance émettrice.

!!! warning "L'emplacement est un chemin **à l'intérieur** du conteneur, écrit relativement au montage hôte"
    Saisissez `user/appdata/rest-server/bombvault-containers/containers`, et **non** `/mnt/user/appdata/…`. BombVault s'exécute dans un conteneur où le `/mnt` de l'hôte est monté ailleurs ; un chemin hôte absolu n'y existe pas. Si vous en collez un, BombVault vous indique désormais le chemin relatif à utiliser.

    VAULT reçoit le mot de passe restic de TOWER via le groupe au moment où vous enregistrez ; personne n'a de clé à saisir.

**5. Rendez-le mutuel, si vous le souhaitez.** Répétez les mêmes cinq étapes dans l'autre sens : un rest-server sur TOWER qui reçoit la copie de VAULT. Chaque machine impose alors l'immuabilité à l'autre, et aucune ne peut supprimer les sauvegardes de l'autre.

## Récupération guidée

Un onglet **Récupération** dédié accompagne une installation neuve ou reconstruite à travers le cas de sinistre, au même endroit :

1. **Restaure d'abord les propres réglages de BombVault**, afin que les chemins de sauvegarde, les cibles hors site et les identifiants dont le reste du flux a besoin soient pré-remplis (appliqué via un auto-redémarrage sur le socket Docker, de sorte que la base de réglages active n'est jamais écrasée sous un handle ouvert).
2. **Vérifie que BombVault peut lire vos sauvegardes** (le piège de la clé de chiffrement en amont).
3. Vous laisse **pointer vers votre dépôt existant** (local ou hors site).
4. **Découvre** les conteneurs, VMs, jeux de fichiers et jeux de données ZFS qui y sont stockés.
5. **Restaure les conteneurs et les VMs en une fois** (laissés arrêtés, afin que vous les démarriez délibérément) et liste les jeux de fichiers et les éléments ZFS à restaurer un par un ; les éléments ZFS reviennent désactivés. Votre kit de récupération est à un clic.

!!! tip "Migration planifiée versus sinistre"
    La récupération guidée restaure les propres réglages de BombVault depuis une sauvegarde. Pour un déplacement *planifié* vers une nouvelle machine, vous pouvez plutôt emporter votre configuration directement avec la carte **Exporter / importer les paramètres** (un fichier JSON portable). Voir [Configuration](configuration.md#portable-settings-export-and-import).

### Restauration depuis un autre dépôt BombVault {#restore-from-another-bombvault-repo}

Une carte distincte dans l'onglet **Récupération** ouvre le dépôt d'une *autre* instance BombVault (un partage monté sous `/mnt`, ou une URL distante) avec **l'`APP_KEY` de cette instance**, dans une session unique en lecture seule. Parcourez les conteneurs, VMs et jeux de fichiers qui y sont stockés, choisissez un instantané et restaurez-le, et l'objet restauré devient un conteneur, une VM ou un jeu de fichiers local normal. Rien n'est jamais écrit dans l'autre dépôt, et vos propres réglages de sauvegarde restent intacts (la session vit en mémoire et expire d'elle-même). Déplacer un conteneur du serveur A vers le serveur B ne signifie plus repointer vos réglages de dépôt puis les rétablir ensuite. Cette carte ne sert qu'une fois : elle ouvre une session, restaure ce que vous choisissez et oublie l'autre instance. Si vous voulez plutôt un arrangement permanent, où cette machine récupère selon un planning les instantanés d'une autre instance dans son propre dépôt, c'est l'onglet **Rapatriement** de la page **Instances**.

## Kit de récupération de clé de chiffrement

C'est la pièce qui rend la reprise après sinistre possible même lorsqu'il n'y a aucun BombVault en fonctionnement.

Un clic télécharge la **clé maîtresse**, le **mot de passe restic dérivé**, et les **emplacements et commandes exacts du dépôt**, afin que vous puissiez restaurer directement avec le CLI restic sur n'importe quelle machine. Un rappel du tableau de bord vous relance jusqu'à ce que vous l'ayez conservé.

!!! danger "Conservez le kit de récupération hors du serveur"
    Le kit contient le secret qui déchiffre vos sauvegardes. Gardez-le en lieu sûr et à l'écart du serveur (un gestionnaire de mots de passe, une copie imprimée dans un coffre). Si vous perdez à la fois BombVault et `APP_KEY` sans kit de récupération, vos sauvegardes chiffrées ne peuvent pas être récupérées.

!!! warning "Le snapshot le plus récent n'est pas toujours celui à restaurer"
    Depuis restic 0.17, `restic snapshots` affiche la taille de chaque snapshot. Après une perte de données, le snapshot le plus récent peut être celui qui a été vidé : ne restaurez donc pas un snapshot beaucoup plus petit que les précédents. Après un rançongiciel, ce peut être le snapshot chiffré, de taille habituelle. Si BombVault tourne encore, consultez d'abord sa page **Anomalies** : elle indique la dernière bonne sauvegarde. Une restauration n'a besoin d'aucune donnée d'anomalie de BombVault, et la pause de rétention ne fait jamais que garder plus de snapshots.

### Sceller le kit

Si vous avez activé le chiffrement age pour les exports en clair (Paramètres), le kit est scellé lui aussi et se télécharge sous `bombvault-recovery-kit.md.age`. Il est en armure ASCII plutôt que binaire, c'est donc toujours du texte : le coller dans un gestionnaire de mots de passe ou l'imprimer fonctionne exactement comme avant, simplement son contenu est illisible sans votre clé.

!!! warning "Ne rangez pas la clé age dans le kit"
    Il vous faut votre clé age **privée** pour ouvrir un kit scellé. Gardez-la à un endroit qui ne dépend pas du kit lui-même, sinon vous aurez deux choses à récupérer au lieu d'une. Sceller le kit vaut la peine quand il est stocké à un endroit que vous ne contrôlez pas entièrement (un gestionnaire de mots de passe partagé, des notes dans le cloud, une impression au bureau) ; un kit dans votre propre coffre est déjà protégé par le coffre.

    Avec le chiffrement activé et aucun destinataire utilisable configuré, le téléchargement est refusé d'emblée. BombVault ne se rabat jamais sur la remise de la clé maîtresse en clair.

### Si le kit n'est pas sous la main

Le mot de passe n'est stocké nulle part, il est **calculé** à partir de l'`APP_KEY`. Avec la clé et un shell, vous pouvez donc le reproduire vous-même :

```sh
printf 'bombvault:restic-repo' \
  | openssl dgst -sha256 -mac HMAC -macopt hexkey:$APP_KEY -r \
  | cut -d' ' -f1
```

C'est un HMAC-SHA256 sur la chaîne fixe `bombvault:restic-repo`, avec pour clé les octets bruts de l'`APP_KEY` hexadécimale, affiché en 64 caractères hexadécimaux minuscules. La même valeur figure dans le kit sous le mot de passe restic dérivé ; ceci sert pour le jour où le kit est ailleurs que vous.

!!! warning "Pour un dépôt reçu, utilisez la clé de l'instance ÉMETTRICE"
    Un dépôt arrivé ici par réplication hors site a été créé par la machine qui l'a envoyé, avec **son** `APP_KEY`. Dériver depuis la clé de la machine réceptrice donne un mot de passe que restic refuse, ce qui ressemble exactement à un dépôt corrompu sans en être un. C'est la raison habituelle pour laquelle `restic check` sur un dépôt reçu redemande le mot de passe encore et encore.

Parce que les définitions de récupération vivent **à l'intérieur** de chaque dépôt (`<repo>/def`, `<repo>/vm-def`), un dossier de dépôt copié est entièrement autonome, de sorte que le kit plus le dépôt sont tout ce dont une restauration sur machine nue a besoin.

## Récupérer un dump de base de données {#database-dumps}

Un dump de base de données est un point de restauration à part entière dans le dépôt des conteneurs, portant l'étiquette `dbdump:<container>` et contenant le seul fichier `/dbdump/<container>.sql`. BombVault les liste, les télécharge et les importe sous **Sauvegardes** ; voici les mêmes étapes avec restic seul, pour le jour où BombVault n'est pas là.

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
