# Sauvegarde hors site et récupération

Les sauvegardes locales vous protègent d'un conteneur perdu ou d'une mauvaise mise à jour. La réplication hors site et un kit de récupération testé vous protègent de la perte de toute la machine, d'un rançongiciel ou d'un incendie. Cette page couvre la réplication hors site, l'inviolabilité de cette copie, la preuve que vous pouvez restaurer, et la récupération lorsque BombVault lui-même a disparu.

## Réplication hors site

Conservez la sauvegarde locale rapide et copiez-la vers un ou plusieurs autres lieux. Vous choisissez les lieux vers lesquels un domaine est copié sur la carte **Domaines** sous **Paramètres, Stockage**, un chip par lieu (voir [Lieux de stockage](storage-places.md#domains)). BombVault y copie les nouveaux instantanés avec `restic copy` au mieux, de sorte qu'une copie échouée ne fait jamais échouer la sauvegarde locale. Le lieu où un domaine est stocké n'a pas besoin d'être local ; voir [Un domaine stocké dans un lieu distant](#remote-primary-repositories).

- **Plusieurs lieux de copie par domaine.** Un domaine peut être copié vers plusieurs lieux à la fois, par exemple un rest-server chez un ami et un bucket B2. La rétention, la classe de stockage, l'append-only, les limites et le budget de croissance appartiennent au lieu, de sorte que chaque copie suit les règles du lieu où elle arrive.
- **Planning de copie par domaine** (édité aux côtés de tous les autres plannings dans Paramètres, Plannings) : laissez-le vide pour copier après chaque sauvegarde locale, ou définissez une cadence (par exemple `weekly Sun 03:00`) pour copier moins souvent que vous ne sauvegardez. **Copier maintenant** sur la ligne du domaine le lance à la demande.
- **Rétention par lieu.** Chaque lieu garde ses propres règles, de sorte qu'un lieu hors site peut garder les copies plus longtemps comme archive. Un lieu dont toutes les règles sont à zéro ne rogne jamais.
- **Les limites de bande passante** par lieu plafonnent le débit d'envoi et de téléchargement de restic afin que la copie ne sature pas votre WAN.
- Un **indicateur de réplication** montre quel domaine copie pendant que la copie s'exécute (sur sa page et le tableau de bord). C'est un indicateur d'activité, pas une barre de pourcentage, car `restic copy` n'expose aucune progression lisible par machine.

!!! note "Restaurer depuis n'importe quel endroit"
    Chaque conteneur, VM, jeu de fichiers, le flash et la configuration de l'application listent leurs sauvegardes comme une seule chronologie à travers tous les endroits où se trouve une sauvegarde. Une sauvegarde copiée vers B2 apparaît une fois, marquée avec chaque endroit qui la détient. Une restauration prend le premier endroit qu'elle peut atteindre, en commençant par le dépôt où l'élément est écrit, et vous pouvez choisir un autre endroit par ligne. Les endroits hors site ne sont lus que lorsque vous les ouvrez. Supprimer à un endroit vérifie d'abord les autres et dit si c'était la dernière copie.

## Emplacement par élément {#placement}

Chaque carte de conteneur, VM et jeu de fichiers a une ligne **Emplacement** avec trois segments :

- **Local** écrit l'élément dans le dépôt indiqué sous **Stocké sur** et ne le copie nulle part. Utilisez-le pour des données qui ont déjà une seconde copie, par exemple un partage qui vit sur un NAS.
- **Local + hors site** l'écrit là aussi et le copie vers les cibles cochées sous **Copier vers**, un chip par cible hors site du domaine. Décochez un chip et cette cible ne reçoit plus rien de nouveau de cet élément.
- **Hors site uniquement** écrit l'élément directement dans le lieu indiqué sous **Envoyer vers**, n'importe quel lieu autre que le lieu principal du domaine. Si le domaine est déjà copié vers ce lieu, l'élément reçoit un dépôt direct à côté des copies ; sinon, BombVault y crée un dépôt pour le domaine.

L'emplacement est fixé dès la première sauvegarde de l'élément, parce que BombVault ne déplace jamais les sauvegardes entre dépôts. Les copies peuvent changer à tout moment. Une cible qui ne reçoit plus un élément conserve les copies qu'elle a et les ramène à sa propre rétention à la prochaine exécution hors site du domaine ; **Supprimer dans B2** sur la carte les supprime aussitôt. Quand certaines de ces copies n'existent nulle part ailleurs, la confirmation les liste par date et demande le nom de l'élément. On ne peut rien supprimer des cibles en ajout seul.

Sous la ligne, la carte dit où va l'élément et ce qui s'y trouve réellement : combien de sites le détiennent, quand chaque cible a été vue pour la dernière fois, et si 3-2-1 est respectée. Un site est le serveur avec les données d'origine et chaque lieu situé sur un autre site (voir [Hors des locaux](#off-the-premises-mark)). BombVault vérifie les copies et les sites ; il ne vérifie pas la partie « deux supports » de 3-2-1.

### Valeurs par défaut par domaine

La carte **Domaines** sous Paramètres, Stockage a une ligne par domaine. **Copié vers** s'applique aussitôt à chaque élément sans choix propre, et aux dossiers de projet des piles Compose. Dès qu'un domaine a des sauvegardes, **Stocké dans** s'applique à un nouvel élément à sa première sauvegarde, et le changer ne déplace aucune sauvegarde. Avant d'enregistrer, la ligne nomme chaque lieu qui gagne ou perd des éléments et combien d'instantanés cela représente, et la question porte l'interrupteur **Appliquer aux éléments sans sauvegarde**, qui remet aussi sur la nouvelle valeur par défaut chaque élément qui n'a pas encore de sauvegarde. **Exceptions** liste les éléments qui ont fait leur propre choix.

Cocher un nouveau lieu sous **Copié vers** lui fait recevoir chaque élément qui n'est pas réglé sur Local. La confirmation dit combien d'éléments et, quand c'est connu, combien d'historique cela représente.

### Dépôts directs

Choisir sous Hors site uniquement un lieu vers lequel le domaine est déjà copié demande une confirmation, puis crée un dépôt direct à côté des copies, par exemple `s3:https://s3.eu-central-003.backblazeb2.com/bucket/container-direct`, et y pointe l'élément. Pour une cible de copie sans lieu, ce choix ouvre une boîte de dialogue avec une adresse suggérée et un test de connexion qui ne crée rien, et **Créer et utiliser** crée le dépôt. Un dépôt direct reprend la clé, la classe de stockage, les limites, le réglage append-only et la rétention du lieu, et change avec eux. Quand une nouvelle clé du lieu ne peut pas l'ouvrir, le dépôt direct conserve la clé qu'il a et l'enregistrement le signale. Ses instantanés portent le tag `bv:direct`, et chaque autre passage de rétention les conserve, si bien qu'un dépôt direct qui a perdu son lien avec son lieu ne vieillit jamais selon les règles locales. Une clé B2 limitée à un dossier doit couvrir l'adresse du lieu, pas seulement le dossier du domaine, sinon le dossier voisin reste hors de portée.

### Hors des locaux {#off-the-premises-mark}

Une copie compte comme un site à part entière seulement quand son lieu se trouve sur un autre site. Un lieu cloud compte toujours et un dossier sur cet Unraid jamais ; pour un NAS, un rest-server ou un serveur SFTP, répondez à **Où se trouve l'appareil ?** dans les détails du lieu par **Ici, dans les locaux** ou **Sur un autre site**. La réponse compte seulement pour les sites et le 3-2-1 sur les cartes et le tableau de bord. Elle ne change aucune copie.

### Après une reconstruction

Les choix de copie vivent dans les propres réglages de BombVault. Après une reconstruction via Découvrir sans un `/config` restauré, ils ont disparu, et tout copier renverrait vers B2 les éléments que vous aviez laissés de côté. La réplication hors site de chaque domaine reconstruit se met donc en pause. Le tableau de bord le montre en orange, et la ligne du domaine sur la carte Domaines propose **Confirmer la valeur par défaut** avec un aperçu de ce que copie la prochaine exécution et les noms dans les sauvegardes qui n'ont pas d'entrée, que vous pouvez laisser de côté à cet endroit. Seule la confirmation met fin à la pause ; importer un fichier de réglages ramène les règles et les valeurs par défaut mais n'y met pas fin.

## Un domaine stocké dans un lieu distant {#remote-primary-repositories}

Un domaine n'a pas besoin d'être stocké localement. Tant que son emplacement de sauvegarde ne contient aucune sauvegarde, choisissez un lieu distant sous **Stocké dans** sur la carte Domaines, et le domaine sauvegarde directement vers ce lieu, sans copie locale ni étape de copie. Le dépôt distant est alors la seule copie, sauf si le domaine est aussi copié vers un autre lieu. Chaque lieu distant offre les mêmes garde-fous :

- **Un test de connexion** avant que quoi que ce soit ne soit écrit.
- **Des limites de bande passante** pour la sauvegarde elle-même, les mêmes options `--limit-upload` et `--limit-download` qu'utilise une copie.
- **Une protection append-only**, vérifiée par le même test de sabotage actif. Quand elle est activée, BombVault n'élague jamais le dépôt, car les identifiants de cette machine ne doivent pas pouvoir supprimer l'unique copie de la sauvegarde.
- **Un budget de croissance**, tiré de la même tendance de taille que suit la carte Stockage.

Un domaine stocké dans un lieu distant est la source de ses copies, comme un domaine local ; voir [Copies entre lieux aux identifiants différents](storage-places.md#different-credentials).

!!! note "Les identifiants appartiennent au lieu"
    Un lieu distant garde ses propres identifiants. Un lieu configuré avec les identifiants cloud partagés continue de les utiliser jusqu'à ce que son accès soit modifié dans ses détails.

## Hors site immuable (append-only)

Marquez un dépôt hors site en append-only afin qu'un rançongiciel, ou un hôte compromis, ne puisse ni supprimer ni réécrire vos sauvegardes. Le côté distant (un `restic/rest-server` s'exécutant en mode `--append-only`) l'**impose**. BombVault ne fait que le **vérifier** et n'affiche jamais du vert sur la seule foi d'une déclaration de configuration.

La fenêtre **Ajouter un lieu** contient une recette prête à coller pour un rest-server en mode append-only, avec un utilisateur pour ce BombVault. Sur un lieu rest-server avec **Append-only** activé, **Tester append-only** dans les détails du lieu lance le test de sabotage sur chaque chemin de domaine, copie activée et dépôt du lieu et donne une seule réponse pour le lieu, de sorte que le hors site append-only soit accessible sans édition manuelle des configs.

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

Sur un lieu, **Tester append-only** sonde chaque chemin de domaine, copie activée et dépôt qui s'y trouvent avec leurs propres identifiants et regroupe les verdicts en une seule réponse : un seul dépôt qui accepte une suppression fait passer tout le lieu en *suppressions acceptées*.

## Essais de reprise après sinistre

BombVault offre deux niveaux de preuve que vos sauvegardes sont réellement restaurables, pas seulement présentes.

- **Essais de vérification de restaurabilité (local).** BombVault exécute périodiquement `restic check --read-data-subset` (borné, jamais une restauration complète qui remplirait le disque) et affiche un badge *dernière restaurabilité vérifiée* par domaine. La cadence vit dans Paramètres, Plannings ; le badge dans Paramètres, Intégrité.
- **Essais de reprise après sinistre (hors site).** BombVault restaure une vraie cible depuis le dépôt hors site dans un bac à sable jetable, la vérifie fichier par fichier et octet par octet, puis nettoie. Cela prouve que vous pouvez récupérer depuis le hors site, pas seulement que le dépôt répond. Seuls les lieux situés sur un autre site sont exercés, car une copie dans les mêmes locaux ne prouve rien sur la perte des locaux. Un domaine copié vers plusieurs d'entre eux est exercé sur l'un d'eux à chaque exécution planifiée, à tour de rôle, et le tableau de bord indique le lieu du dernier essai.

Le **tableau de bord de protection contre les rançongiciels** du tableau de bord synthétise cela en une posture verte / orange / rouge par domaine, avec une liste de contrôle horodatée (hors site configuré, append-only vérifié, réplication à jour, essai de restauration réussi, chiffrement activé, stratégie d'élagage définie). Chaque ligne rouge renvoie directement au correctif, et la carte ne passe au vert que sur des faits vérifiés.

## Tableau de bord récepteur (le côté réception)

![Le côté récepteur, surveillé en lecture seule, avec un contrôle d'intégrité exécuté sur cette machine.](assets/screenshots/receiver.png)

*Le côté récepteur, surveillé en lecture seule, avec un contrôle d'intégrité exécuté sur cette machine.*

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

**1. Sur VAULT, mettez en place le serveur append-only.** Dans BombVault sur TOWER, ouvrez *Paramètres → Stockage*, cliquez sur **Ajouter un lieu**, choisissez **rest-server** et cliquez sur **Afficher la recette**. Copiez le bloc **Modèle Unraid**, enregistrez-le sur VAULT sous `/boot/config/plugins/dockerMan/templates-user/my-rest-server.xml`, puis *Docker → Add Container* et choisissez **rest-server** dans la liste des modèles. Avant de le démarrer, écrivez la ligne `htpasswd` affichée dans `/mnt/user/appdata/rest-server/.htpasswd` sur VAULT. Le mot de passe n'est affiché qu'une fois et n'est jamais conservé ; la recette l'a déjà reporté, avec l'utilisateur, dans le formulaire sur TOWER, laissez donc cette fenêtre ouverte. La ligne `htpasswd` porte le même mot de passe, déjà haché en bcrypt pour vous, vous n'avez donc rien à hacher vous-même.

    Laissez `--append-only` dans le champ OPTIONS. Sans lui, VAULT n'est plus qu'un partage ordinaire.

**2. Sur TOWER, ajoutez le lieu.** Saisissez l'adresse de VAULT, `http://VAULT:8000`, à côté de l'utilisateur et du mot de passe que la recette a remplis, puis cliquez sur **Tester la connexion**. BombVault construit l'adresse à partir de ces informations :

    rest:http://VAULT:8000/tower

Le premier segment du chemin est l'utilisateur htpasswd, ici `tower`, et chaque domaine reçoit son dossier en dessous, par exemple `rest:http://VAULT:8000/tower/container`. Répondez à **Où se trouve l'appareil ?** par **Sur un autre site**, cliquez sur **Ajouter**, et cochez le lieu sous **Copié vers** pour les domaines qui doivent y aller.

**3. Sur TOWER, activez Append-only** sous **Protection** dans les détails du lieu, puis cliquez sur **Tester append-only**. Le test sonde chaque chemin de domaine, copie et dépôt du lieu et donne une seule réponse pour le lieu, qui doit être *suppressions refusées*. Ce que signifient les réponses :

| Résultat | Ce qui s'est passé |
| --- | --- |
| **suppressions refusées** | VAULT a refusé la suppression. C'est le seul état qui passe. |
| **suppressions acceptées** | VAULT a accepté une suppression. `--append-only` manque ou a été retiré. |
| un message au lieu d'un résultat | Le test n'a pas pu s'exécuter. En général, l'adresse n'est pas celle qu'utilise restic lui-même, ou les identifiants ont changé. Rien n'est enregistré et aucune alerte n'est déclenchée. |

**4. Sur VAULT, regardez ce qui arrive.** Activez *Paramètres → Récepteur*, ouvrez l'onglet **Récepteur** et enregistrez le dépôt en lecture seule.

!!! warning "L'emplacement est un chemin **à l'intérieur** du conteneur, écrit relativement au montage hôte"
    Saisissez `user/appdata/rest-server/tower/container`, et **non** `/mnt/user/appdata/…`. BombVault s'exécute dans un conteneur où le `/mnt` de l'hôte est monté ailleurs ; un chemin hôte absolu n'y existe pas. Si vous en collez un, BombVault vous indique le chemin relatif à utiliser.

    L'**APP_KEY d'envoi** est la clé de TOWER, pas celle de VAULT. Vous la trouvez sur TOWER sous *Paramètres → Système*.

**5. Rendez-le mutuel, si vous le souhaitez.** Répétez les mêmes cinq étapes dans l'autre sens : un rest-server sur TOWER qui reçoit la copie de VAULT. Chaque machine impose alors l'immuabilité à l'autre, et aucune ne peut supprimer les sauvegardes de l'autre.

## Récupération guidée

Un onglet **Récupération** dédié accompagne une installation neuve ou reconstruite à travers le cas de sinistre, au même endroit :

1. **Vérifie que BombVault peut lire vos sauvegardes** (le piège de la clé de chiffrement en amont).
2. **Restaure les propres réglages de BombVault**, afin que les chemins de sauvegarde, les cibles hors site et les identifiants dont le reste du flux a besoin soient pré-remplis. Il lit la sauvegarde des réglages depuis le lieu que la ligne Auto-sauvegarde indique sous **Stocké dans**, ou depuis la copie de l'Auto-sauvegarde sous **Copié vers**, et affiche ce lieu avec son adresse ; pour lire depuis un autre lieu, modifiez d'abord la ligne Auto-sauvegarde à l'étape 3. La restauration est appliquée via un auto-redémarrage sur le socket Docker, de sorte que la base de réglages active n'est jamais écrasée sous un handle ouvert.
3. **Rattache vos sauvegardes existantes** via les lignes de la carte Domaines : sur la ligne de chaque domaine, choisissez sous **Stocké dans** le lieu où se trouvent ses sauvegardes et sous **Copié vers** les lieux qui contiennent ses copies. Un lieu qu'aucune ligne ne propose encore, comme un partage, un serveur ou un bucket cloud, se connecte avec **Ajouter un lieu**, la même fenêtre que dans Paramètres, Stockage. **Connexion et aperçu** vérifie ensuite que les sauvegardes sont lisibles.
4. **Découvre** les conteneurs, VMs et jeux de fichiers qui y sont stockés.
5. **Les restaure tous** (laissés arrêtés, afin que vous les démarriez délibérément), avec votre kit de récupération à un clic.

!!! note "Les copies hors site attendent après une reconstruction"
    Quand l'étape 4 reconstruit des entrées sans les anciens réglages, la réplication hors site de ces domaines se met en pause jusqu'à ce que l'emplacement par défaut soit confirmé. Voir [Emplacement par élément](#placement).

!!! tip "Migration planifiée versus sinistre"
    La récupération guidée restaure les propres réglages de BombVault depuis une sauvegarde. Pour un déplacement *planifié* vers une nouvelle machine, vous pouvez plutôt emporter votre configuration directement avec la carte **Exporter et importer les réglages** (un fichier JSON portable). Voir [Configuration](configuration.md#portable-settings-export-and-import).

### Restauration depuis un autre dépôt BombVault

Une carte distincte dans l'onglet **Récupération** ouvre le dépôt d'une *autre* instance BombVault (un partage monté sous `/mnt`, ou une URL distante) avec **l'`APP_KEY` de cette instance**, dans une session unique en lecture seule. Parcourez les conteneurs, VMs et jeux de fichiers qui y sont stockés, choisissez un instantané et restaurez-le, et l'objet restauré devient un conteneur, une VM ou un jeu de fichiers local normal. Rien n'est jamais écrit dans l'autre dépôt, et vos propres réglages de sauvegarde restent intacts (la session vit en mémoire et expire d'elle-même). Déplacer un conteneur du serveur A vers le serveur B ne signifie plus repointer vos réglages de dépôt puis les rétablir ensuite. La fédération serveur-à-serveur en direct est explicitement hors du périmètre ; c'est un tirage ponctuel délibéré.

## Kit de récupération de clé de chiffrement

C'est la pièce qui rend la reprise après sinistre possible même lorsqu'il n'y a aucun BombVault en fonctionnement.

Un clic télécharge la **clé maîtresse**, le **mot de passe restic dérivé**, et les **emplacements et commandes exacts du dépôt**, afin que vous puissiez restaurer directement avec le CLI restic sur n'importe quelle machine. Un rappel du tableau de bord vous relance jusqu'à ce que vous l'ayez conservé.

!!! danger "Conservez le kit de récupération hors du serveur"
    Le kit contient le secret qui déchiffre vos sauvegardes. Gardez-le en lieu sûr et à l'écart du serveur (un gestionnaire de mots de passe, une copie imprimée dans un coffre). Si vous perdez à la fois BombVault et `APP_KEY` sans kit de récupération, vos sauvegardes chiffrées ne peuvent pas être récupérées.

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
