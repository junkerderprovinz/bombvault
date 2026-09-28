# Lieux de stockage

Un lieu de stockage est un endroit où BombVault conserve des sauvegardes : un dossier sur cet Unraid, un partage sur un NAS, un bucket chez un fournisseur cloud, un rest-server, un compte SFTP ou un Nextcloud. Vous connectez chaque lieu une seule fois, dans **Paramètres, Stockage**, et ses identifiants, sa rétention, sa protection et sa localisation lui appartiennent. Les domaines (conteneurs, VMs, la flash, la propre configuration de BombVault, les jeux de fichiers et les jeux de données ZFS) choisissent ensuite parmi les lieux : où chaque domaine est stocké et vers où il est copié. Les jeux de données ZFS ont leur ligne sur la carte Domaines tant que le domaine ZFS est activé.

## Ajouter un lieu {#add-a-place}

**Ajouter un lieu** ouvre une fenêtre avec une tuile par fournisseur, en trois groupes : stockage cloud, services auto-hébergés, et NAS et ce serveur.

1. Choisissez une tuile et remplissez son formulaire. Le bouton en forme d'œil affiche un secret que vous avez saisi.
2. **Tester la connexion** vérifie le lieu et ne crée rien. Pour le dossier de chaque domaine, il indique ce qu'il a trouvé : vide ou pas encore présent, contenant déjà un dépôt restic, ou l'erreur qui l'a arrêté.
3. Donnez un nom au lieu ; le nom du fournisseur est prérempli. Pour un appareil que vous gérez vous-même, répondez à **Où se trouve l'appareil ?**. Les fournisseurs cloud sont toujours sur un autre site, et un dossier sur cet Unraid est toujours ici.
4. **Ajouter** enregistre le lieu.

Un nouveau lieu n'est encore utilisé par aucun domaine. Choisissez-le sous **Stocké dans** ou **Copié vers** sur la [carte Domaines](#domains), ou sur la carte d'un élément pour cet élément seul.

## Dossiers {#folders}

Un lieu conserve un dossier par domaine : `container`, `vms`, `flash`, `config`, `files` et `zfs`, les noms qu'utilisent les emplacements de sauvegarde par défaut. Les dossiers sont listés dans les détails du lieu et peuvent y être renommés (voir [Changer une adresse](#addresses)). Un domaine sans dossier dans un lieu ne peut pas choisir ce lieu.

Quand un domaine utilise un lieu dans les deux rôles, le second rôle reçoit un suffixe et le premier garde son dossier. Un lieu qui reçoit déjà les copies d'un domaine stocke les éléments envoyés directement vers lui dans `<folder>-direct` ; un lieu qui stocke déjà un domaine reçoit ses copies dans `<folder>-copies`.

Certains lieux sont eux-mêmes un dépôt restic : une adresse qui contenait déjà un dépôt quand le lieu a été ajouté, un dépôt nommé issu d'une configuration existante, ou une cible de copie à la racine d'un bucket. Un tel lieu n'a pas de dossiers, tous les domaines partagent son unique dépôt, et il n'accepte pas de second rôle. Pour stocker davantage chez le même fournisseur, connectez un autre bucket ou un autre dossier comme lieu à part entière.

## Détails d'un lieu {#details}

Chaque lieu est une ligne avec son fournisseur, ce à quoi il sert et son dernier test ou sa dernière copie. **Tester** vérifie chaque adresse du lieu, et **Détails** ouvre ses réglages. Chaque modification dans les détails est enregistrée dès que vous la faites.

- **Général** : le nom, l'interrupteur qui active et désactive le lieu, l'adresse et, pour un appareil que vous gérez vous-même, **Où se trouve l'appareil ?** (voir [Hors des locaux](#off-the-premises)).
- **Rétention** : conserver les derniers, quotidiens, hebdomadaires et mensuels, pour chaque dépôt du lieu. Un nouveau lieu démarre avec les règles par défaut ; un lieu dont toutes les règles sont à zéro ne rogne jamais.
- **Protection** : l'interrupteur **Append-only**. C'est l'autre bout qui doit imposer l'append-only ; interrupteur activé, BombVault n'élague et ne supprime jamais rien là-bas. Sur un rest-server avec append-only activé, **Tester append-only** lance le test de sabotage sur chaque chemin de domaine, copie activée et dépôt du lieu et affiche une seule réponse pour tout le lieu, *suppressions refusées* ou *suppressions acceptées* (voir [Sauvegarde hors site et récupération](offsite-recovery.md)). Seuls les lieux distants ont cette section, car rien sur cette machine ne peut empêcher la suppression d'un dépôt local.
- **Accès** : les identifiants et, pour S3, la classe de stockage. Un lieu qui utilise les identifiants partagés reçoit son propre ensemble à la première modification. Un dépôt direct du lieu que les nouveaux identifiants ne peuvent pas ouvrir garde les anciens, et la réponse le signale. Les lieux de type dossier, SFTP et rclone n'ont pas cette section.
- **Limites** : le débit d'envoi et de téléchargement et le budget de croissance.
- **Dossiers** : un interrupteur par domaine, avec le nom de son dossier. Un domaine désactivé ici ne peut pas choisir le lieu.

Baisser la rétention demande d'abord confirmation et indique combien d'éléments sont concernés ; désactiver l'append-only demande d'abord confirmation et indique combien de dépôts du lieu le perdent. Désactiver un lieu désactive chaque dépôt qui s'y trouve ; un lieu dans lequel un domaine est stocké ne peut pas être désactivé.

## La carte Domaines {#domains}

La carte a une ligne par domaine, avec son planning, où il est stocké, vers où il est copié et ses exceptions.

- **Stocké dans** : tant que l'emplacement de sauvegarde du domaine ne contient aucune sauvegarde, le lieu choisi devient le lieu principal du domaine et l'emplacement y est déplacé. Dès qu'il contient des sauvegardes, le choix pour les conteneurs, les VMs et les jeux de fichiers devient la valeur par défaut des nouveaux éléments, qui l'adoptent à leur première sauvegarde ; les éléments qui ont déjà des sauvegardes restent où ils sont, car BombVault ne déplace jamais une sauvegarde. Pour la flash et la propre configuration de BombVault, le lieu principal change, et les sauvegardes déjà écrites restent à l'ancien lieu.
- **Copié vers** : un chip par lieu qui peut recevoir les copies du domaine. Cocher un chip fait du lieu une cible de copie du domaine ; la première fois, BombVault indique à l'avance combien d'éléments et d'instantanés et quelle quantité de données la première exécution envoie. Décocher arrête les nouvelles copies : les copies déjà présentes restent et vieillissent selon la rétention du lieu, et les éléments qui ont fait leur propre choix continuent d'y copier. Décocher le dernier chip arrête toutes les copies, y compris vers les lieux ajoutés plus tard, jusqu'à ce qu'un chip soit coché de nouveau. Un lieu désactivé apparaît comme un chip grisé et ne peut pas être choisi.
- **Exceptions** : les éléments qui ont fait leur propre choix, sous forme de liste avec des liens vers leurs cartes.
- **Copier maintenant** lance aussitôt les copies du domaine.

Un domaine mis en pause après une reconstruction via Découvrir affiche la pause sur sa ligne, avec **Confirmer la valeur par défaut** (voir [Emplacement par élément](offsite-recovery.md#placement)).

## Changer une adresse {#addresses}

Le dossier d'un domaine peut être changé dans les détails du lieu, tout comme l'adresse d'un lieu local, par exemple après qu'un dépôt a été déplacé à la main vers un autre disque. BombVault teste chaque adresse concernée par le changement et l'accepte quand chaque nouvelle adresse est vide et que rien n'était stocké à l'ancienne, ou quand chaque nouvelle adresse contient le même dépôt restic que l'ancienne. Tout le reste est refusé, avec le nombre de sauvegardes encore présentes à l'ancienne adresse. Un lieu distant garde son adresse ; pour sauvegarder ailleurs, connectez cet autre endroit comme lieu à part entière.

BombVault construit la liste des lieux à partir de sa propre base de données et ne liste jamais un dépôt distant pour la remplir ; le test ne s'exécute que lorsque vous changez quelque chose.

## Retirer un lieu {#remove}

Un lieu ne peut être retiré que tant que rien ne l'utilise : aucun domaine n'y est stocké, aucune valeur par défaut n'y renvoie, aucun élément n'y est stocké, et aucun dépôt direct qui s'y trouve ne contient d'éléments. Sinon, le refus liste ce qui le retient. Le retirer emporte ses cibles de copie, ainsi que ses propres identifiants sauf si une source de rapatriement ou un autre lieu les utilise. Rien n'est supprimé dans le stockage lui-même, et la confirmation indique combien de copies y restent.

## Sans lieu {#without-a-place}

Une adresse qui ne correspond pas à la forme d'un lieu plus un dossier continue de fonctionner et apparaît sous **Sans lieu**, avec son adresse. Les adresses natives `b2:`, `gs:` et `swift:` en font partie. **Attribuer à un lieu** rattache une telle ligne à un lieu, après le même test que pour [changer une adresse](#addresses). Une cible de copie sans lieu est aussi nommée sur la ligne de son domaine, à côté des chips, et continue de copier. Une ligne distante y a son propre interrupteur **Append-only**, et le désactiver demande d'abord confirmation avec le nombre d'éléments qui gardent des sauvegardes à cette adresse. Un dépôt direct suit l'interrupteur de sa cible.

## Hors des locaux {#off-the-premises}

**Où se trouve l'appareil ?** a deux réponses : **Ici, dans les locaux** et **Sur un autre site**. Une copie compte comme un site à part entière, pour la ligne 3-2-1 des cartes et pour les vérifications hors site du tableau de bord, seulement quand son lieu se trouve sur un autre site. Un second disque ou un NAS dans les mêmes locaux est une seconde copie, pas un second site. La réponse ne change aucune copie. Les fournisseurs cloud sont toujours sur un autre site et un dossier sur cet Unraid toujours ici, le formulaire ne pose donc pas la question pour eux ; pour tout autre lieu, changez la réponse dans ses détails. Un lieu situé sur un autre site porte la marque **Autre site** sur sa ligne.

## Types de connexion

### Dossier sur cet Unraid ou un NAS {#kind-local}

L'adresse est un chemin sous `/mnt`, écrit sans `/mnt`, par exemple `user/bombvault`, et le dossier de chaque domaine se trouve en dessous : `user/bombvault/container`.

- **Dossier sur cet Unraid** choisit parmi les partages, les disques et les pools.
- **Synology**, **QNAP**, **TrueNAS**, **Autre Unraid** et **Autre partage** choisissent dans `/mnt/remotes`. Montez d'abord le partage sur Unraid, par exemple avec le plugin Unassigned Devices. Host Data doit être monté en Read/Write - Slave, sinon un partage qui se monte après le démarrage de BombVault reste invisible jusqu'à un redémarrage (voir [Configuration](configuration.md)).

Le sélecteur de dossiers crée un dossier là où il se trouve avec **Nouveau dossier**. Le test vérifie que le dossier est vide ou absent et que BombVault peut y écrire.

### S3 {#kind-s3}

L'adresse est `s3:https://<endpoint>/<bucket>/<path>`, par exemple `s3:https://s3.eu-central-003.backblazeb2.com/tower-backups/bombvault`.

- **Backblaze B2** n'a besoin que de l'ID de clé et de la clé d'application. BombVault demande à B2 à quel bucket, quel point de terminaison S3 et quel dossier la clé est limitée, et construit l'adresse à partir de ces informations. Une clé qui peut atteindre tous les buckets propose ses buckets au choix.
- **Amazon S3**, **Cloudflare R2**, **Wasabi**, **Hetzner Object Storage**, **Storj**, **IDrive e2**, **Scaleway**, **OVHcloud**, **DigitalOcean Spaces**, **IONOS**, **Contabo**, **Exoscale** et **Vultr** demandent la clé et, quand le fournisseur en a besoin, la région, l'ID de compte ou le point de terminaison. BombVault remplit le point de terminaison et liste les buckets quand la clé a le droit de les lister ; sinon, saisissez le nom du bucket.
- **Google Cloud Storage** passe par son interface S3 avec une clé HMAC, créée dans les paramètres de Cloud Storage sous Interopérabilité. Un fichier de compte de service ne fonctionne pas ici.
- **MinIO**, **SeaweedFS**, **Garage**, **Ceph**, **JuiceFS**, **RustFS**, **Versity S3 Gateway** et **Autre service S3** demandent l'adresse du service et une clé.

La classe de stockage se règle dans les détails du lieu, limitée aux niveaux qu'une restauration peut lire sans dégel.

### rest-server {#kind-rest}

L'adresse est `rest:<url>/<user>`, par exemple `rest:https://nas.lan:8000/tower`. Le formulaire demande l'adresse du serveur, un utilisateur et un mot de passe. Avec `--private-repos`, un utilisateur ne peut atteindre que les chemins qui commencent par son propre nom, donc BombVault place l'utilisateur en tête sauf si vous saisissez un autre chemin. Quand le serveur refuse un chemin en dehors de celui de l'utilisateur, l'erreur le dit.

Le formulaire rest-server contient une recette prête à coller pour un rest-server en mode append-only avec un utilisateur pour ce BombVault. **Afficher la recette** crée un mot de passe, affiché une seule fois, et fournit une ligne `docker run`, un fichier compose et un modèle Unraid, chacun avec la ligne `htpasswd` à placer sur le serveur ; l'utilisateur et le mot de passe vont directement dans le formulaire.

**Autre BombVault** liste, au-dessus de ses propres champs, les offres ouvertes que d'autres instances ont envoyées via Flotte. En accepter une ajoute un lieu qui ne conserve que les copies du domaine proposé, car une offre porte un utilisateur pour ce seul domaine. Accepter depuis la page Flotte ajoute le même lieu.

### SFTP {#kind-sftp}

L'adresse est `sftp://<user>@<host>:<port>/<path>`, par exemple `sftp://bv@backup.lan:22/bombvault`. Le formulaire demande l'hôte, le port et l'utilisateur et affiche la clé publique de BombVault. Ajoutez cette clé au `~/.ssh/authorized_keys` de l'utilisateur sur le serveur ; rien d'autre n'a besoin d'y être installé. BombVault accepte la clé d'hôte du serveur au premier contact et la vérifie ensuite.

**Hetzner Storage Box** remplit `<user>.your-storagebox.de` et le port 23. Installez la clé sur la box avec la commande de Hetzner, qui demande une fois le mot de passe de la box :

```sh
echo '<public key>' | ssh -p 23 <user>@<user>.your-storagebox.de install-ssh-key
```

### WebDAV : Nextcloud, ownCloud, OpenCloud {#kind-webdav}

Le formulaire demande l'adresse du serveur, l'utilisateur et un mot de passe d'application. Créez le mot de passe d'application dans les paramètres de sécurité du compte, et saisissez l'identifiant de l'utilisateur plutôt qu'une adresse e-mail. BombVault construit le chemin WebDAV qu'utilise le produit et transmet la connexion à restic par les variables d'environnement de rclone, avec le mot de passe sous la forme masquée de rclone. L'adresse se lit `rclone:bvp<id>:<path>`, où `bvp<id>` est un remote qui n'existe que dans cet environnement ; rien n'est écrit dans la configuration rclone.

### Azure Blob {#kind-azure}

L'adresse est `azure:<container>:/<path>`. Le formulaire demande le compte de stockage et sa clé d'accès ; après **Tester la connexion**, il liste les conteneurs du compte au choix, ou vous saisissez le nom d'un conteneur. BombVault transmet le compte et la clé à restic dans `AZURE_ACCOUNT_NAME` et `AZURE_ACCOUNT_KEY`.

### rclone {#kind-rclone}

L'adresse est `rclone:<remote>:<path>`. Le formulaire propose au choix les remotes de la configuration rclone de BombVault. Pour remplacer cette configuration, collez un `rclone.conf` complet sous **Configuration rclone** et cliquez sur **Enregistrer la configuration**. Elle est enregistrée aussitôt et sert à chaque lieu rclone, que la fenêtre ajoute ensuite un lieu ou non.

## Copies entre lieux aux identifiants différents {#different-credentials}

Un domaine stocké dans un lieu distant est la source de ses copies. `restic copy` s'exécute avec un seul environnement, et BombVault ajoute les identifiants de la source à ceux de la cible quand les deux ne donnent pas des valeurs différentes à la même variable. Un lieu Nextcloud et un lieu B2 utilisent des variables différentes, donc un domaine stocké dans Nextcloud peut être copié vers B2. Deux comptes S3 ou deux utilisateurs rest-server auraient besoin des mêmes variables avec des valeurs différentes ; restic ne peut pas prendre les deux, et le chip sur la carte Domaines indique que les identifiants ne sont pas compatibles.
