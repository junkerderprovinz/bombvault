# Sauvegarde hors site et récupération

Les sauvegardes locales vous protègent d'un conteneur perdu ou d'une mauvaise mise à jour. La réplication hors site et un kit de récupération testé vous protègent de la perte de toute la machine, d'un rançongiciel ou d'un incendie. Cette page couvre la réplication hors site, l'inviolabilité de cette copie, la preuve que vous pouvez restaurer, et la récupération lorsque BombVault lui-même a disparu.

## Réplication hors site

Conservez la sauvegarde locale rapide et ajoutez un ou plusieurs réplicas hors site. Définissez un dépôt par domaine dans l'onglet **Paramètres, Hors site**. BombVault y réplique les nouveaux instantanés avec `restic copy` au mieux, de sorte qu'un accroc hors site ne fait jamais échouer la sauvegarde locale. Le dépôt local reste principal.

- **Plusieurs cibles hors site par domaine.** Chaque domaine (conteneurs, VMs, flash, config et jeux de fichiers) peut répliquer vers plusieurs destinations hors site à la fois, pas seulement une, de sorte que vous pouvez garder, par exemple, un rest-server sur la machine d'un ami et un bucket S3 en parallèle. Ajoutez des cibles supplémentaires dans Paramètres, Hors site, chacune avec son propre dépôt, sa classe de stockage S3, son indicateur append-only, sa rétention et son budget de croissance. Une configuration hors site unique existante est reprise intacte comme première cible, et chaque cible d'un domaine réplique selon le planning hors site de ce domaine.
- **Planning hors site par domaine** (édité aux côtés de tous les autres plannings dans Paramètres, Plannings) : laissez-le vide pour répliquer après chaque sauvegarde locale, ou définissez une cadence (par exemple `weekly Sun 03:00`) pour expédier hors site moins souvent que vous ne sauvegardez localement. Un bouton **Répliquer maintenant** couvre les exécutions à la demande.
- **La rétention hors site** vit dans Paramètres, Hors site afin que vous puissiez garder les copies hors site plus longtemps comme archive. Laissez la politique entièrement à zéro pour ne jamais rogner automatiquement les instantanés hors site.
- **Les limites de bande passante** (Paramètres, Hors site) plafonnent le débit d'envoi/de téléchargement de restic afin que la réplication ne sature pas votre WAN.
- Un **indicateur de réplication** montre quel domaine réplique pendant qu'elle s'exécute (sur sa page et le tableau de bord). C'est un indicateur d'activité, pas une barre de pourcentage, car `restic copy` n'expose aucune progression lisible par machine.

!!! note "Restaurer directement depuis le hors site"
    Chaque navigateur de sauvegardes dispose d'un commutateur **Local / Hors site**, de sorte que si un dépôt local est perdu ou corrompu, vous pouvez lister et restaurer directement depuis le réplica hors site. La suppression est par source : retirer une sauvegarde n'affecte que la copie que vous consultez.

## Dépôts primaires distants {#remote-primary-repositories}

Le chemin de sauvegarde d'un domaine (Paramètres, Chemins et stockage) ne se limite pas à un dossier local : pointez-le directement vers un dépôt restic distant (`s3:...`, `rest:http://hôte:8000/depot`, `b2:...`, `sftp:utilisateur@hôte:/depot`, `rclone:remote:bucket/chemin`) et BombVault y sauvegarde directement, sans copie locale séparée ni étape de réplication. C'est une forme vraiment différente de la réplication hors site vue plus haut : là, le dépôt local est primaire et le dépôt hors site en est une archive au mieux ; ici, le dépôt distant **est** le primaire, et c'est la seule copie tant que vous n'ajoutez pas aussi une réplication hors site (ou un second dépôt distant) pour ce domaine.

Chacun des cinq champs de chemin (Conteneurs, VM, Flash, Configuration, Fichiers) porte juste à côté un commutateur **Local / Distant** :

- **Local** affiche l'explorateur de dossiers habituel.
- **Distant** le remplace par un simple champ d'URL, plus un bouton qui ouvre la même boîte de dialogue de test de connexion et d'identifiants que les destinations hors site, mais configurée pour ce dépôt primaire. Vous y trouvez :
    - **Un test de connexion** contre le chemin réel, avant de vous y fier.
    - **Des limites de bande passante** (envoi et réception) pour qu'une sauvegarde planifiée vers un primaire distant ne sature pas votre liaison WAN : les mêmes options restic `--limit-upload` et `--limit-download` qu'utilise la réplication hors site, appliquées cette fois à la sauvegarde elle-même.
    - **Une protection append-only (immuabilité)**, vérifiée par le même test d'altération actif (une véritable sonde DELETE contre l'autre bout) dont bénéficient les destinations hors site. Activée, elle interdit à BombVault d'élaguer le dépôt lui-même : comme aucune copie locale séparée ne se trouve derrière, les identifiants de cette machine ne doivent pas pouvoir supprimer l'unique copie de la sauvegarde.
    - **Une alerte de budget de croissance**, tirée de la même tendance de taille du dépôt que suit déjà la carte Stockage.

Rien de tout cela n'est obligatoire : un chemin distant saisi à la main, sans réglages de sécurité enregistrés, sauvegarde exactement comme avant (bande passante illimitée, élagage possible, pas d'alerte de budget). La boîte de dialogue de sécurité est là pour le jour où vous voulez les mêmes protections qu'une copie hors site, sans devoir créer une destination hors site rien que pour cela.

!!! note "Les identifiants cloud et REST sont partagés"
    Un dépôt primaire distant s'authentifie avec les mêmes identifiants S3/REST configurés sous Paramètres, Hors site, Identifiants cloud. Il n'existe pas de magasin d'identifiants distinct pour les dépôts primaires.

## Hors site immuable (append-only)

Marquez un dépôt hors site en append-only afin qu'un rançongiciel, ou un hôte compromis, ne puisse ni supprimer ni réécrire vos sauvegardes. Le côté distant (un `restic/rest-server` s'exécutant en mode `--append-only`) l'**impose**. BombVault ne fait que le **vérifier** et n'affiche jamais du vert sur la seule foi d'une déclaration de configuration.

L'assistant de **configuration hors site guidée** vous accompagne du choix du backend (rest-server / rclone / S3) jusqu'à un extrait de déploiement rest-server prêt à coller, un test de connexion, la bascule d'immuabilité (qui lance le test de sabotage immédiatement) et une stratégie de rétention, de sorte que le hors site append-only soit accessible sans édition manuelle des configs.

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

- **Essais de vérification de restaurabilité (local).** BombVault exécute périodiquement `restic check --read-data-subset` (borné, jamais une restauration complète qui remplirait le disque) et affiche un badge *dernière restaurabilité vérifiée* par domaine. La cadence vit dans Paramètres, Plannings ; le badge dans Paramètres, Intégrité.
- **Essais de reprise après sinistre (hors site).** BombVault restaure une vraie cible depuis le dépôt hors site dans un bac à sable jetable, la vérifie fichier par fichier et octet par octet, puis nettoie. Cela prouve que vous pouvez récupérer depuis le hors site, pas seulement que le dépôt répond.

Le **tableau de bord de protection contre les rançongiciels** du tableau de bord synthétise cela en une posture verte / orange / rouge par domaine, avec une liste de contrôle horodatée (hors site configuré, append-only vérifié, réplication à jour, essai de restauration réussi, chiffrement activé, stratégie d'élagage définie). Chaque ligne rouge renvoie directement au correctif, et la carte ne passe au vert que sur des faits vérifiés.

## Tableau de bord récepteur (le côté réception)

Tout ce qui précède est le côté *émetteur*. Sur la machine qui **reçoit** des copies hors site immuables d'un autre BombVault, le tableau de bord récepteur vous donne une surveillance indépendante et en lecture seule de ces dépôts sur le matériel de réception, afin qu'une défaillance silencieuse à l'autre bout ne passe pas inaperçue.

Activez la bascule **Récepteur** dans les Paramètres pour révéler un onglet **Récepteur**. Il est désactivé par défaut ; ne l'activez que sur une machine qui reçoit réellement des sauvegardes hors site immuables. Enregistrez ensuite un dépôt reçu (en lecture seule, ouvert avec la clé de l'instance émettrice) pour obtenir :

- **Un inventaire d'instantanés groupé par source**, afin que vous puissiez voir exactement quels conteneurs, VMs et jeux de fichiers sont arrivés.
- **La dernière réception** par source, afin que vous sachiez à quel point chacune est fraîche.
- **Un `restic check` indépendant** exécuté sur le matériel de réception, afin que l'intégrité soit vérifiée là où les données se trouvent réellement, pas seulement sur l'émetteur.
- **Un dispositif d'homme mort :** une alerte lorsqu'une source cesse d'émettre dans une fenêtre que vous définissez.
- **Des alertes d'intégrité :** une alerte lorsqu'une vérification côté réception échoue.

Le récepteur est strictement en lecture seule. Il n'écrit jamais dans le dépôt reçu, il ne peut donc jamais briser la garantie append-only sur laquelle l'émetteur compte.

## Exemple complet : deux machines Unraid, de bout en bout

Ce qui précède décrit les pièces. Voici une installation complète avec de vraies valeurs, parce que les pièces s'assemblent plus facilement quand on les a vues assemblées une fois.

Deux machines : **TOWER** fait tourner les conteneurs et envoie les sauvegardes, **VAULT** les reçoit et impose l'immuabilité. Remplacez par vos propres noms, adresses et chemins de partage.

**1. Sur VAULT, mettez en place le serveur append-only.** Dans BombVault sur TOWER, allez dans *Paramètres → Hors site → configuration guidée*, choisissez **rest-server** et générez la recette. Copiez l'onglet **Modèle Unraid (XML)**, enregistrez-le sur VAULT sous `/boot/config/plugins/dockerMan/templates-user/my-rest-server.xml`, puis *Docker → Add Container* et choisissez **rest-server** dans la liste des modèles. Avant de le démarrer, écrivez la ligne `htpasswd` affichée dans `/mnt/user/appdata/rest-server/.htpasswd` sur VAULT. Le mot de passe à usage unique n'est affiché qu'une fois et n'est jamais conservé : copiez-le maintenant.

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

**4. Sur VAULT, regardez ce qui arrive.** Activez *Paramètres → Récepteur*, ouvrez l'onglet **Récepteur** et enregistrez le dépôt en lecture seule.

!!! warning "L'emplacement est un chemin **à l'intérieur** du conteneur, écrit relativement au montage hôte"
    Saisissez `user/appdata/rest-server/bombvault-containers/containers`, et **non** `/mnt/user/appdata/…`. BombVault s'exécute dans un conteneur où le `/mnt` de l'hôte est monté ailleurs ; un chemin hôte absolu n'y existe pas. Si vous en collez un, BombVault vous indique désormais le chemin relatif à utiliser.

    L'**APP_KEY d'envoi** est la clé de TOWER, pas celle de VAULT. Vous la trouvez sur TOWER sous *Paramètres → Système*.

**5. Rendez-le mutuel, si vous le souhaitez.** Répétez les mêmes cinq étapes dans l'autre sens : un rest-server sur TOWER qui reçoit la copie de VAULT. Chaque machine impose alors l'immuabilité à l'autre, et aucune ne peut supprimer les sauvegardes de l'autre.

## Récupération guidée

Un onglet **Récupération** dédié accompagne une installation neuve ou reconstruite à travers le cas de sinistre, au même endroit :

1. **Restaure d'abord les propres réglages de BombVault**, afin que les chemins de sauvegarde, les cibles hors site et les identifiants dont le reste du flux a besoin soient pré-remplis (appliqué via un auto-redémarrage sur le socket Docker, de sorte que la base de réglages active n'est jamais écrasée sous un handle ouvert).
2. **Vérifie que BombVault peut lire vos sauvegardes** (le piège de la clé de chiffrement en amont).
3. Vous laisse **pointer vers votre dépôt existant** (local ou hors site).
4. **Découvre** les conteneurs, VMs et jeux de fichiers qui y sont stockés.
5. **Les restaure tous** (laissés arrêtés, afin que vous les démarriez délibérément), avec votre kit de récupération à un clic.

!!! tip "Migration planifiée versus sinistre"
    La récupération guidée restaure les propres réglages de BombVault depuis une sauvegarde. Pour un déplacement *planifié* vers une nouvelle machine, vous pouvez plutôt emporter votre configuration directement avec la carte **Exporter et importer les réglages** (un fichier JSON portable). Voir [Configuration](configuration.md#portable-settings-export-and-import).

### Restauration depuis un autre dépôt BombVault

Une carte distincte dans l'onglet **Récupération** ouvre le dépôt d'une *autre* instance BombVault (un partage monté sous `/mnt`, ou une URL distante) avec **l'`APP_KEY` de cette instance**, dans une session unique en lecture seule. Parcourez les conteneurs, VMs et jeux de fichiers qui y sont stockés, choisissez un instantané et restaurez-le, et l'objet restauré devient un conteneur, une VM ou un jeu de fichiers local normal. Rien n'est jamais écrit dans l'autre dépôt, et vos propres réglages de sauvegarde restent intacts (la session vit en mémoire et expire d'elle-même). Déplacer un conteneur du serveur A vers le serveur B ne signifie plus repointer vos réglages de dépôt puis les rétablir ensuite. La fédération serveur-à-serveur en direct est explicitement hors du périmètre ; c'est un tirage ponctuel délibéré.

## Kit de récupération de clé de chiffrement

C'est la pièce qui rend la reprise après sinistre possible même lorsqu'il n'y a aucun BombVault en fonctionnement.

Un clic télécharge la **clé maîtresse**, le **mot de passe restic dérivé**, et les **emplacements et commandes exacts du dépôt**, afin que vous puissiez restaurer directement avec le CLI restic sur n'importe quelle machine. Un rappel du tableau de bord vous relance jusqu'à ce que vous l'ayez conservé.

!!! danger "Conservez le kit de récupération hors du serveur"
    Le kit contient le secret qui déchiffre vos sauvegardes. Gardez-le en lieu sûr et à l'écart du serveur (un gestionnaire de mots de passe, une copie imprimée dans un coffre). Si vous perdez à la fois BombVault et `APP_KEY` sans kit de récupération, vos sauvegardes chiffrées ne peuvent pas être récupérées.

Parce que les définitions de récupération vivent **à l'intérieur** de chaque dépôt (`<repo>/def`, `<repo>/vm-def`), un dossier de dépôt copié est entièrement autonome, de sorte que le kit plus le dépôt sont tout ce dont une restauration sur machine nue a besoin.
