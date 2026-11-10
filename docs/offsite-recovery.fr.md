# Hors site et récupération

!!! note "Les copies hors site attendent après une reconstruction"
    Quand l'assistant d'urgence ou **Découvrir les sauvegardes** reconstruit des éléments sans les anciens réglages, les copies hors site de ces sections attendent que vous adoptiez leurs sauvegardes pour ce serveur. Voir [Après une reconstruction](#after-a-rebuild).

Les sauvegardes locales vous protègent d'un conteneur perdu ou d'une mauvaise mise à jour. Les copies hors site et un kit de récupération testé vous protègent de la perte du serveur entier, d'un rançongiciel ou d'un incendie. Cette page couvre les destinations et les copies hors site, comment rendre une copie à l'épreuve des suppressions, comment prouver que vous pouvez restaurer, l'appairage de plusieurs serveurs, et comment tout récupérer quand BombVault lui-même a disparu.

## Copies hors site {#off-site-copies}

Gardez la sauvegarde locale rapide et ajoutez une ou plusieurs destinations hors des locaux. BombVault y copie les nouveaux instantanés avec `restic copy` au mieux, si bien qu'un accroc hors site ne fait jamais échouer la sauvegarde locale. Dans cette forme, chaque section écrit d'abord dans son propre dossier et le dépôt hors site est une copie. Le dossier propre n'a pas besoin d'être sur ce serveur : voir [Le dossier propre d'une section](#own-folder) pour le garder dans une destination comme S3 ou un rest-server.

- **Plusieurs destinations par section.** Chaque section (conteneurs, VMs, clé flash, auto-sauvegarde, dossiers et jeux de données ZFS) peut copier vers plusieurs destinations hors site à la fois, si bien que vous pouvez garder, par exemple, un rest-server chez un ami et un bucket S3 en parallèle. La carte **Destinations par section** de la page Destinations active ou désactive chaque ligne d'une section vers une destination.
- **Quand les copies s'exécutent.** La carte **Copier** d'une destination règle **Quand copier** : **Après chaque sauvegarde**, ou selon un planning à elle qui rassemble les instantanés et copie à heures fixes. Sous **Paramètres, Planning, Copies hors site par section**, **Exception** donne à une section son propre planning de copie.
- **La carte Copies hors site.** Sous **Paramètres, Destinations**, chaque section qui a une copie hors site a une ligne dans **Copies hors site**, avec **Tester la connexion**, **Répliquer maintenant** pour lancer une copie à la demande, et **Configurer** ou **Modifier**. Le reste des réglages de copie de la section s'ouvre de là dans sa fenêtre.
- **La rétention** se règle par destination sous **Durée de conservation**, si bien que vous pouvez garder les copies hors site plus longtemps comme archive. La copie dans le champ hors site d'une section la suit comme toute autre copie, et une section peut garder ses propres valeurs sous **Règles par section**. Tout à zéro ne taille jamais les instantanés hors site. Voir [Durée de conservation](#keep).
- **Les limites de bande passante** (**Bande passante hors site** sous **Paramètres, Destinations**, ou la carte **Copier** d'une destination) plafonnent le débit d'envoi et de téléchargement de restic pour que les copies ne saturent pas votre WAN. Elles se saisissent en KiB/s, l'unité que prend restic, et chaque champ montre le même débit en MB/s et en Mbit/s pendant que vous tapez : 2048 se lit environ 2,0 MB/s (16,8 Mbit/s).
- **La compression** est **Désactivé**, **Automatique** (le réglage par défaut de restic) ou **Maximum**, réglée sur chaque destination et, pour une section, dans ses règles là-bas. Les sauvegardes, les copies et l'élagage écrivent avec elle, et le kit de récupération la nomme, si bien que restic seul peut continuer à écrire de la même façon.
- **Le streaming d'abord.** Pendant qu'un serveur multimédia comme Plex, Jellyfin ou Emby diffuse, les copies s'envoient avec une limite plus basse et reviennent à la normale quelques minutes après la fin du flux. La carte **Streaming** de la page Destinations a **Ralentir les copies hors site pendant le streaming**, **Serveurs multimédias** (les conteneurs avec Plex, Jellyfin ou Emby dans le nom de l'image tant que vous ne choisissez pas), **Streaming au-delà de (Mbit/s)**, **Limite d'envoi pendant le streaming (KiB/s)** et **Retour à la normale après (min)**. BombVault lit le trafic depuis Docker, un serveur multimédia sur le réseau de l'hôte ne peut donc pas être mesuré. REST, S3, B2, Azure, Google Cloud, Swift et rclone sur HTTP ralentissent en pleine copie. SFTP et les dossiers locaux ou montés prennent la limite plus basse à leur prochaine étape de copie.
- **Stockage froid (S3).** Une destination S3 peut utiliser une **Classe de stockage** moins chère que restic peut encore lire tout de suite, comme Standard-IA, One Zone-IA, Intelligent-Tiering ou Glacier Instant Retrieval. Glacier Flexible Retrieval et Deep Archive sont exclus, car restic ne peut pas restaurer depuis eux. Un remote rclone règle sa classe dans la configuration rclone.
- **Connexions simultanées.** Sur une destination S3, B2, Azure, Google Cloud, Swift, rest-server, SFTP ou rclone, **Connexions simultanées** dans sa carte **Connexion** définit combien de connexions restic ouvre à la fois, de 1 à 32. Laissé vide, restic garde sa valeur par défaut de 5. Davantage de connexions peut accélérer une liaison lointaine ou lente, moins ménage un fournisseur qui limite le débit. Les sauvegardes, les copies, les vérifications, les élagages et les restaurations de la destination l'utilisent tous, et un export des réglages l'emporte avec lui. Un dossier local ou monté n'a pas ce réglage.
- **Budget de croissance.** **Budget de croissance (GB, 0 = désactivé)**, dans la fenêtre de copie d'une section, déclenche une alarme quand le dépôt dépasse cette taille. Il ne supprime jamais rien, il convient donc à une copie en ajout seul que rien n'élague d'ici.
- Pendant qu'une copie tourne, l'en-tête d'état de l'aperçu indique **Copie hors site en cours**. C'est un indicateur d'activité, pas une barre de pourcentage, car `restic copy` n'expose aucune progression lisible par machine.

!!! note "Restaurer depuis n'importe quel endroit"
    Chaque élément liste ses sauvegardes comme une seule chronologie sur tous les endroits où une sauvegarde se trouve, sur son onglet **Sauvegardes**. Une sauvegarde copiée vers B2 apparaît une fois, marquée avec chaque endroit qui la contient. Une restauration prend le premier endroit qu'elle peut atteindre, en commençant par le dépôt dans lequel l'élément écrit, et vous pouvez choisir un autre endroit par instantané. Les destinations hors site ne sont lues que quand vous cliquez sur **Vérifier**, car lister un stockage cloud prend du temps. Supprimer à un endroit vérifie d'abord les autres et dit si c'était la dernière copie. Voir [Restauration](restore.md).

## Destinations {#destinations}

Une destination est un endroit où vont les sauvegardes : un dossier local, un partage, un serveur à vous ou un service cloud. Elle se configure une fois et sert ensuite toutes les sections. Ouvrez **Destinations** depuis sa tuile de paramètres, ou activez-y **Afficher aussi dans la barre latérale** pour l'épingler.

![La page Destinations : la liste des destinations et le flux qui dit quelle section va où.](assets/screenshots/storage/locations-dark.png#only-dark)
![La page Destinations : la liste des destinations et le flux qui dit quelle section va où.](assets/screenshots/storage/locations-light.png#only-light)

**Ajouter une destination** ouvre une fenêtre en quatre étapes :

1. **Fournisseur.** Chaque service est listé avec son logo, avec une recherche en haut à droite de la fenêtre, par groupes : **Local** pour un disque ou un partage attaché à ce serveur, **NAS ou serveur** (rest-server, Hetzner Storage Box, SFTP, SMB, WebDAV et autres), **Service S3** (Backblaze B2, Wasabi, Cloudflare R2, Hetzner Object Storage, Amazon S3 et autres), votre propre serveur S3 (Garage, SeaweedFS, RustFS, Silo, Ceph, JuiceFS, Versity S3 Gateway), **Stockage cloud** (OneDrive, Google Drive, Dropbox, pCloud, Nextcloud et le reste de ce que prend rclone en charge) et **Depuis votre groupe**, les serveurs récepteurs de votre groupe. Chacun dit à quel point il convient aux sauvegardes : les lecteurs cloud ralentissent sous de nombreuses requêtes, si bien que la première sauvegarde et l'élagage y prennent plus de temps. Pour un service qu'aucune tuile ne couvre, la tuile **Rclone** sous **Configuration rclone personnelle** accepte des remotes collés depuis un rclone.conf.
2. **Connexion.** Les champs dépendent du service : une clé d'accès pour S3, un utilisateur et un mot de passe pour WebDAV et SMB, un mot de passe d'application là où la connexion à deux facteurs bloque le mot de passe normal, la clé publique SSH de BombVault pour SFTP et la Storage Box, ou un jeton pour les services qui se connectent par un navigateur. Pour ceux-ci, la fenêtre montre une commande `rclone authorize` à exécuter sur un ordinateur avec un navigateur, et le jeton qu'elle affiche va dans le champ. Là où vous avez déjà des [identifiants enregistrés](#sign-ins) pour le service, **Identifiants enregistrés** les choisit au lieu de **Nouvelles clés**. **Tester la connexion** vérifie la connexion avant que quoi que ce soit soit enregistré. La fenêtre liste alors les dossiers de la destination, avec **Nouveau dossier** pour en créer un et la place libre là où le service l'indique. Un dossier vide est le plus sûr. BombVault y met un dossier par section.
3. **Protection.** La fenêtre dit clairement ce que le service peut faire. Un rest-server en mode ajout seul refuse la suppression, et le test d'altération le vérifie. Un bucket S3 peut garder d'anciennes versions avec le versionnage et le verrouillage d'objets, ce que BombVault ne sait pas encore vérifier. Un lecteur cloud ne peut pas du tout refuser la suppression : quiconque entre dans le serveur entre aussi dans cette copie. N'activez **Protection contre la suppression (ajout seul)** que là où l'autre côté refuse vraiment la suppression. BombVault n'élague alors jamais là-bas. **En cas d'urgence** dit ce que le kit de récupération contient pour cette destination.
4. **Pour quelles sections.** Choisissez les sections qui sauvegardent ici. La première copie emporte tout l'historique. Les éléments qui ont choisi leurs propres destinations ne reçoivent pas la nouvelle d'eux-mêmes, et la fenêtre les nomme.

![Ajouter une destination : la première étape, avec chaque fournisseur sous forme de tuile.](assets/screenshots/storage/add-providers-dark.png#only-dark)
![Ajouter une destination : la première étape, avec chaque fournisseur sous forme de tuile.](assets/screenshots/storage/add-providers-light.png#only-light)

![Ajouter une destination : l'étape de connexion après un test réussi.](assets/screenshots/storage/add-signin-dark.png#only-dark)
![Ajouter une destination : l'étape de connexion après un test réussi.](assets/screenshots/storage/add-signin-light.png#only-light)

Certains services demandent un peu plus à la connexion :

- **SFTP et la Storage Box.** **Ajouter la clé** place la clé publique de BombVault sur le serveur avec le **Mot de passe du compte**, qui sert pour cette seule connexion et n'est gardé nulle part. La commande pour le faire à la main est montrée en dessous. Le test montre la **Clé du serveur** avec son empreinte. BombVault la retient et refuse le serveur s'il en montre plus tard une différente.
- **Les clés S3 pour un seul bucket**, comme un jeton d'objet Cloudflare R2 ou une clé AWS restreinte, peuvent ne pas lister les autres buckets. Saisissez le bucket dans **Bucket**, et le test et la liste des dossiers ouvrent ce bucket.
- **Régions et adresses S3.** La région est lue depuis un point de terminaison qui la nomme (AWS, Backblaze B2, Hetzner, Scaleway, OVH, Wasabi), Garage démarre avec sa propre région par défaut, et IDrive e2 demande le point de terminaison que montre son tableau de bord. Une adresse collée avec le bucket dessus, comme le montre le tableau de bord de R2, remplit le serveur et le bucket. Un bucket créé depuis le navigateur de dossiers va dans la région que nomme le formulaire.
- **Classe de stockage.** Le sélecteur ne propose que les classes que prend le service, et **Plus de paramètres** ne montre que les options qui façonnent un nouveau bucket.
- **Google Drive et OneDrive.** Les données élaguées sont supprimées pour de bon au lieu d'aller à la corbeille, où elles compteraient encore dans le quota. Un lecteur partagé Google va sous **Plus de paramètres**.
- **Les serveurs SMB qui ne listent pas leurs partages** prennent le nom de l'un sous **Partage**, et le test ouvre directement ce partage.
- **FTP** demande son **Chiffrement** : **Aucun**, **TLS explicite (FTPES)** ou **TLS implicite (FTPS, port 990)**.
- **OneDrive et Zoho WorkDrive.** Quand une connexion ouvre plusieurs lecteurs ou espaces de travail, la fenêtre demande dans lequel vont les sauvegardes.
- **Un rest-server avec un certificat autosigné.** Placez le certificat comme fichier PEM dans le dossier `certs` du dossier de données de BombVault (`/config/certs` dans le conteneur, `appdata/bombvault/certs` sur Unraid). Les sauvegardes, les copies et le test de protection contre la suppression lui font confiance sans redémarrage.

Les services S3 passent par le moteur S3 propre à restic, ce qui permet à une classe de stockage et au verrouillage d'objets de s'appliquer. Un rest-server est joint directement par restic. Tous les autres services passent par le rclone que livre BombVault, et sa section de la configuration rclone s'affiche dans la carte **rclone.conf** de la page de la destination, avec les secrets masqués. Un export de réglages porte les destinations, et avec les identifiants inclus, il porte aussi leurs identifiants de connexion.

Enregistrer une destination qui contient déjà des sauvegardes de conteneurs, de VMs ou de dossiers, par exemple une que vous aviez supprimée et que vous rajoutez, l'ajoute à ces sections comme copie non cochée. Rien n'y est copié tant que vous ne la cochez pas, mais **Restaurer** propose tout de suite les anciennes sauvegardes.

Le dossier sur ce serveur s'appelle **Local** partout : sur les pages de stockage, dans le flux, sur un élément et sur Restauration. Avec plusieurs dossiers sur ce serveur, chacun porte son propre nom. Une destination qu'établit le champ hors site d'une section est nommée d'après son endroit, le dernier dossier de son adresse ou, sans lui, son serveur, et reçoit un numéro quand une autre destination porte déjà ce nom. Chaque destination peut être renommée dans le champ **Nom** de sa carte **Connexion**, aussi un dossier de bucket que plusieurs sections partagent et la copie que décrit le champ hors site d'une section, et un nom que vous lui donnez reste. Là où deux destinations de même nom se côtoient, chacune porte son serveur ou son dossier.

### La page d'une destination {#location-page}

Chaque destination a sa propre page. Elle ne montre que les cartes qui s'appliquent à cette destination, dans cet ordre :

![La page d'une destination : Dossiers propres, Connexion et Identifiants.](assets/screenshots/storage/location-page-dark.png#only-dark)
![La page d'une destination : Dossiers propres, Connexion et Identifiants.](assets/screenshots/storage/location-page-light.png#only-light)

- **Dossiers propres** : les sections qui écrivent d'abord leurs sauvegardes ici. Les identifiants, la protection et les limites de cette destination s'appliquent à elles. **Utiliser comme dossier propre** déplace ici le dossier propre d'autres sections, et **Modifier** en éloigne un de nouveau. Voir [Le dossier propre d'une section](#own-folder).
- **Sauvegardé directement ici** : chaque section dont les éléments sauvegardent directement à cet endroit au lieu de leur dossier propre, et combien d'éléments le font. Un tel dépôt direct reprend tous ses réglages de cette destination. Un dépôt vers lequel plus rien ne sauvegarde le dit, et **Supprimer** le retire de la liste. Les sauvegardes à son endroit restent. Voir [Dépôts directs](#direct-repositories).
- **Connexion** : le **Nom**, l'interrupteur **Disponible** (désactivé, BombVault n'écrit plus ici, et les éléments qui sauvegardent ici échouent avec un message clair au lieu d'atterrir ailleurs), le dossier, **Compression**, **Classe de stockage**, **Connexions simultanées** et **Hors des locaux**. Sur la destination **Local**, elle liste le dossier de chaque section sous **Dossiers propres ici**. **Modifier la connexion** teste et enregistre une nouvelle adresse ou de nouveaux identifiants. Quand d'autres endroits se connectent avec les mêmes identifiants enregistrés, la fenêtre le dit, et une modification ici donne à cette destination ses propres identifiants au lieu de réécrire ceux qui sont partagés.
- **Identifiants** : quels identifiants enregistrés elle utilise (**Se connecte avec**), quelles autres destinations les utilisent aussi (**Également utilisés par**), et **Modifier**, qui ouvre la fenêtre des identifiants. Voir [Identifiants](#sign-ins).
- **rclone.conf** : la section de la configuration rclone de cette destination, pour une destination qui passe par rclone.
- **Protection contre la suppression**, avec **Protection contre la suppression (ajout seul)**, **Accepter sans protection contre la suppression**, **Tester la protection contre la suppression maintenant** et **Qui élague**, qui dit si BombVault élague la copie et les dossiers propres ici, selon la carte **Durée de conservation** de cette destination ou les valeurs personnalisées d'une section sous **Règles par section**, ou laisse cela à l'autre côté. Un dossier local ne montre cette carte que s'il a une protection que vous pouvez désactiver. Voir [Protection contre la suppression](#delete-protection-append-only).
- **Copier** : **Quand copier**, et la **Limite d'envoi (KiB/s)** et la **Limite de téléchargement (KiB/s)**. Une destination qui ne contient que des dossiers propres et aucune copie montre les limites dans une carte appelée **Bande passante**.
- **Durée de conservation** : **Court**, **Équilibré**, **Long** ou **Valeurs personnalisées**, avec une chronologie des sauvegardes que la règle garde. Voir [Durée de conservation](#keep).
- **Déjà sous cette destination** : les sections dont l'adresse saisie à la main se trouve dans un dossier de cette destination. En reprendre une laisse le dépôt où il est. Les identifiants, la classe de stockage et la protection contre la suppression viennent alors de la destination. BombVault vérifie d'abord que les identifiants de la destination ouvrent le dépôt, et il refuse de placer un dépôt en ajout seul sous une destination qui n'est pas en ajout seul.
- **Règles par section** : le dépôt propre, la rétention ou la compression d'une section à cette destination. **Modifier les règles** les ouvre, et **Revenir à la destination** y ramène une section. Les identifiants, la bande passante et le moment des copies viennent toujours de la destination.
- Un récipient qui montre à quel point la destination est pleine, avec la marque où elle est pleine et combien de semaines il reste au rythme de croissance des dernières semaines. Une destination qui n'indique pas sa taille dit **Taille inconnue**.

**Déplacer** et **Supprimer la destination** figurent parmi les actions de la page, décrits sous [Déplacer et supprimer](#move-remove).

Une destination distante qui contenait des sauvegardes et ne contient soudain plus de dépôt, comme un rest-server dont le disque ne s'est pas monté, n'en reçoit pas un nouveau et vide d'elle-même. Sa page le dit dès qu'une sauvegarde ou une copie constate que le dépôt a disparu. Remontez le disque. Si vous avez vidé l'ancien dépôt exprès, **Créer un nouveau dépôt ici** repart de zéro, pour toutes les sections de la destination et ses dépôts à la fois. Cette protection vaut pour une destination restée en place. Une destination que vous ajoutez, ou une section que vous cochez sur l'une d'elles, part de zéro : sans dépôt à cette adresse, la première copie en crée un, même là où une destination supprimée auparavant en avait un.

Deux refus nomment ce qu'il faut corriger, dans l'exécution de la copie et sur la page de la destination. **Ce bucket n'existe pas sur le serveur** apparaît dès qu'un serveur S3 le signale, et BombVault n'y crée rien, si bien qu'un nom de bucket mal saisi reste visible. Un serveur qui ne répond pas du tout est un autre cas : restic continue de réessayer pendant environ 15 minutes par appel, au cas où il reviendrait. **Un dépôt restic existe déjà ici, et la clé de cette instance ne l'ouvre pas** signifie que les sauvegardes qui s'y trouvent appartiennent à une autre `APP_KEY`.

Un serveur récepteur que gère une autre instance de votre groupe apparaît parmi les fournisseurs comme **Depuis votre groupe**. Voir [Serveur récepteur](#receiving-server).

BombVault contrôle ses destinations à chaque démarrage. Il corrige ce qui ne demande aucune décision de votre part, comme un lien vers une copie ou un dépôt qui a disparu, une copie sur le dossier propre d'une section, ou un dépôt direct dont la copie a disparu, et écrit une ligne dans le journal pour chaque correction. Ce qui n'a pas de remède sûr est seulement consigné.

![La même page sur un téléphone.](assets/screenshots/storage/location-page-phone-dark.png#only-dark)
![La même page sur un téléphone.](assets/screenshots/storage/location-page-phone-light.png#only-light)

### Durée de conservation {#keep}

La durée de conservation des sauvegardes se règle par destination, dans sa carte **Durée de conservation**. La règle couvre les copies à cette destination et les dossiers propres qui s'y trouvent, la destination **Local** comprise. Une section peut garder ses propres valeurs à une destination sous **Règles par section**, et celles-ci l'emportent sur la carte de la destination pour cette section.

Une règle garde une sauvegarde par jour, semaine, mois et année, aussi loin que vous le réglez, et élimine le reste après chaque sauvegarde ou copie :

| Préréglage | Dernières sauvegardes | Jours | Semaines | Mois | Années |
|---|---|---|---|---|---|
| **Court** | 0 | 7 | 4 | 3 | 0 |
| **Équilibré** | 0 | 7 | 4 | 6 | 2 |
| **Long** | 0 | 14 | 8 | 12 | 3 |

**Valeurs personnalisées** règle vous-même les cinq nombres. Un 0 désactive ce nombre, et une règle dont tous les nombres sont à 0 garde tout. Les points de la carte montrent lesquelles de 40 sauvegardes d'exemple une règle garde, la plus récente tout à droite. L'échantillon est le même pour chaque destination, car l'image est dessinée avant que BombVault ait lu les sauvegardes de l'endroit. Il remonte à partir d'aujourd'hui, donc le nombre de points qui restent change avec la date.

Un dossier sur cet hôte et un dossier distant saisi à la main ont aussi leur propre carte, qui couvre les dossiers propres de ce dossier et aucun autre. Tant que vous ne l'avez pas enregistrée, un tel dossier vieillit selon la règle que BombVault avait déjà pour lui.

**Demander avant de conserver moins**, activé par défaut, demande avant d'enregistrer un changement qui garde moins de sauvegardes, et dit combien des 40 sauvegardes d'exemple la nouvelle règle supprime. Il s'applique à chaque destination. **Revenir à la destination** sous **Règles par section** pose la même question.

Une destination avec protection contre la suppression n'est jamais élaguée depuis ce serveur. Son autre côté garde ou abandonne d'anciennes sauvegardes selon ses propres règles. Tant qu'un élément a une anomalie critique ouverte sur des données perdues, la rétention garde les anciennes sauvegardes de cet élément jusqu'à ce que vous en accusiez réception. Voir [Anomalies](anomalies.md).

Une sauvegarde qui contient moins qu'elle ne le devrait ne fait jamais élaguer celles d'avant. C'est une sauvegarde qui ne lit aucun fichier là où les précédentes en contenaient, à laquelle il manque un dossier sélectionné, ou qui n'a pas pu lire certains fichiers. Son exécution se termine par un avertissement qui nomme ce qui manque et dit que les sauvegardes plus anciennes ont été gardées, vous recevez le même message sur vos canaux de notification, et aucun élagage, ni au dossier propre de l'élément ni à ses copies, ne touche les anciennes sauvegardes de cet élément jusqu'à ce qu'une sauvegarde complète passe. Une première sauvegarde, et un dossier sélectionné qui a toujours été vide, comptent comme complets.

Pour voir ce que la prochaine exécution supprimerait vraiment, ouvrez **Aperçu** sous **Paramètres, Destinations**. Il liste **Ce que la prochaine exécution supprimerait** pour le dossier propre de la section et pour chacune de ses copies, par dépôt et par élément avec les points de restauration nommés. Il ne prend aucun verrou et ne change rien.

![Durée de conservation avec Équilibré choisi : les points montrent lesquelles des 40 sauvegardes d'exemple restent.](assets/screenshots/storage/keep-dark.png#only-dark)
![Durée de conservation avec Équilibré choisi : les points montrent lesquelles des 40 sauvegardes d'exemple restent.](assets/screenshots/storage/keep-light.png#only-light)

#### Exemple : ce que garde Équilibré {#example-retention}

Un conteneur est sauvegardé chaque nuit à 03:00 depuis le 1er octobre 2024, si bien que le mercredi 4 novembre 2026 il a eu 765 sauvegardes. Sa destination est réglée sur **Équilibré** : 7 jours, 4 semaines, 6 mois et 2 ans.

Chaque nombre garde la sauvegarde la plus récente de son jour, sa semaine, son mois ou son année, et une sauvegarde que plusieurs nombres choisissent n'est gardée qu'une fois. Les semaines vont du lundi au dimanche :

| Règle | Sauvegardes qu'elle choisit | Nouvelles qu'elle ajoute |
|---|---|---|
| 7 jours | 4 nov, 3 nov, 2 nov, 1 nov, 31 oct, 30 oct, 29 oct | les 7 |
| 4 semaines | 4 nov, 1 nov, 25 oct, 18 oct | 25 oct, 18 oct |
| 6 mois | 4 nov, 31 oct, 30 sept, 31 août, 31 juil, 30 juin | 30 sept, 31 août, 31 juil, 30 juin |
| 2 ans | 4 nov, 31 déc 2025 | 31 déc 2025 |

Il reste donc 14 sauvegardes sur 765. La plus ancienne date du 31 décembre 2025, vous pouvez donc aujourd'hui remonter un peu plus de dix mois en arrière. Grâce à la déduplication, elles ne prennent guère plus de place qu'une sauvegarde complète plus ce qui a changé entre-temps.

**Années** mérite un second regard. Il compte les années civiles qui contiennent une sauvegarde, en commençant par l'année en cours, et la sauvegarde la plus récente de cette année est celle d'aujourd'hui. La première année n'ajoute donc rien aux autres nombres, et la seconde ajoute la dernière sauvegarde de l'année précédente. C'est pourquoi la portée d'une règle varie au fil de l'année. Les mêmes sauvegardes avec chaque préréglage :

| Préréglage | Sauvegardes gardées aujourd'hui | Sauvegarde la plus ancienne aujourd'hui | Portée au fil de l'année |
|---|---|---|---|
| **Court** | 10 | 30 septembre 2026 | 1 à 2 mois |
| **Équilibré** | 14 | 31 décembre 2025 | environ 4 mois au début du printemps, une année entière le 31 décembre |
| **Long** | 30 | 31 décembre 2024 | 1 an le 1er janvier, 2 ans le 31 décembre |

Passer d'**Équilibré** à **Court** demande d'abord confirmation, à cause de **Demander avant de conserver moins**. La question indique combien des 40 sauvegardes de l'image la nouvelle règle retire. Ce nombre vient de l'échantillon, il change donc avec la date et ne compte pas vos propres sauvegardes. Parmi les sauvegardes de cet exemple, le 31 août, le 31 juillet, le 30 juin et le 31 décembre 2025 partiraient. **Enregistrer quand même** applique la règle, et le prochain élagage les supprime de cette destination pour de bon.

Pour garder les conteneurs plus longtemps que le reste à cette destination, appuyez sur **Modifier les règles** sous **Règles par section**, choisissez **Conteneurs**, et réglez des **Valeurs personnalisées** dans **Règles pour Conteneurs**, par exemple 14 jours, 8 semaines, 12 mois et 2 ans. Les autres sections continuent de suivre la carte de la destination, et **Revenir à la destination** ramène Conteneurs.

### Identifiants {#sign-ins}

Des identifiants sont la clé ou le mot de passe que BombVault utilise pour un service S3 ou un rest-server. Les identifiants forment une seule liste, la carte **Identifiants** sous **Paramètres, Destinations**, et chaque ligne dit quelles destinations les utilisent (**Utilisés par**).

- **Enregistrés quand vous ajoutez une destination.** Ajouter une destination enregistre les clés que vous saisissez comme identifiants. La destination suivante chez le même service peut les choisir sous **Identifiants enregistrés** au lieu de **Nouvelles clés**, et une modification ultérieure s'applique à chaque destination qui les utilise.
- **Le réglage par défaut.** Une étoile marque les identifiants par défaut. Ils servent partout où aucun identifiant n'est choisi, comme une adresse saisie à la main ou une récupération sur une installation neuve. Une source de rapatriement ne les emprunte pas. **Définir par défaut** donne l'étoile à d'autres.
- **Modifier et Tester.** **Modifier**, ici ou dans la carte **Identifiants** d'une destination, ouvre une seule fenêtre pour les identifiants. **Tester** se connecte à chaque destination qui les utilise avec ce que vous avez saisi, avant que quoi que ce soit soit enregistré. **Enregistrer** les stocke.
- **Suppression.** Des identifiants ne peuvent être supprimés que si aucune destination, source de rapatriement ou dépôt reçu ne les utilise et qu'aucune ancienne sauvegarde n'en a besoin, et ceux par défaut ne partent qu'en dernier. D'anciennes sauvegardes en ont besoin quand un dossier propre a quitté un endroit qui se connecte avec eux et les y a laissées. BombVault dit alors de quel endroit il s'agit.
- **Modifier la connexion d'une destination** ne réécrit jamais des identifiants que d'autres endroits partagent. La destination reçoit à la place des identifiants à elle, et la fenêtre dit d'avance que ses identifiants sont partagés. Pour modifier ceux qui sont partagés partout, modifiez-les dans la carte **Identifiants**.

![La carte Identifiants : chaque identifiant enregistré avec ce qui l'utilise, et l'étoile sur celui par défaut.](assets/screenshots/storage/signins-dark.png#only-dark)
![La carte Identifiants : chaque identifiant enregistré avec ce qui l'utilise, et l'étoile sur celui par défaut.](assets/screenshots/storage/signins-light.png#only-light)

Un dossier sur ce serveur n'a besoin d'aucun identifiant. SMB, WebDAV, SFTP, la Storage Box et les lecteurs cloud gardent les leurs dans la configuration rclone, que montre la carte **rclone.conf** de la destination, ils n'apparaissent donc pas dans cette liste.

### SMB, WebDAV et autres remotes rclone {#smb-webdav}

Un partage Windows ou Samba et un serveur WebDAV (Nextcloud, ownCloud, SharePoint ou tout autre) sont des tuiles de fournisseur dans **Ajouter une destination**, sous **NAS ou serveur**. Renseignez l'hôte et le partage (SMB) ou l'adresse et le type de serveur (WebDAV), l'utilisateur et le mot de passe, et BombVault écrit la section rclone pour vous. rclone obscurcit le mot de passe avant de le stocker.

Pour un service qu'aucune tuile ne couvre, la tuile **Rclone** sous **Configuration rclone personnelle** accepte des remotes collés depuis un rclone.conf. Ils sont ajoutés aux remotes que BombVault a déjà et ne les remplacent jamais, et ils sont stockés chiffrés. Pour sauvegarder une section vers l'un d'eux, saisissez `rclone:<remote>:<folder>` comme dépôt hors site de cette section, par exemple `rclone:nas:backups/bombvault`. Le partage ou le bucket est le premier segment du chemin, il ne fait pas partie du nom.

C'est la meilleure voie que de monter le partage sur Unraid : restic déconseille de garder un dépôt sur un partage CIFS monté, et ici rien n'est monté. NFS n'a pas de tuile car ni restic ni rclone n'ont de moteur NFS. Pour NFS, montez l'export sur l'hôte et utilisez-le comme dossier local.

### Déplacer et supprimer {#move-remove}

- **Déplacer** emmène la copie d'une section vers une autre destination, avec tout son historique. Tout reste à l'ancien endroit. BombVault n'y supprime rien. Le déplacement s'exécute en arrière-plan et peut être long : environ 9 minutes pour 100 GB entre disques locaux, des heures sur une liaison lente. L'aperçu l'indique pendant qu'il tourne.
- **Supprimer la destination** propose deux voies. **Supprimer la destination, garder les sauvegardes** cesse d'y écrire et laisse les sauvegardes en place. Vous pouvez les lire plus tard sous **Restauration, Depuis un ancien dossier de sauvegarde**. **Supprimer la destination et effacer les sauvegardes qui s'y trouvent** lit d'abord ce qui y est stocké et le liste : quelles sauvegardes partent, lesquelles ne sont nulle part ailleurs, et quels dépôts restent parce que la protection contre la suppression refuse, que des éléments sauvegardent encore directement vers eux, qu'une autre destination ou section y garde ses copies, ou qu'ils n'ont pas pu être lus. Les éléments qui perdraient leur seule sauvegarde sont listés à part et conservés, sauf si vous activez **Effacer aussi celles-ci**. Vous confirmez en saisissant le nom de la destination, et la suppression s'exécute en arrière-plan.
- Cela fonctionne pour tous les types de destinations : celle que plusieurs sections utilisent, une cible configurée pour une section, un dépôt que vous avez configuré, et le dossier dans lequel une section écrit ses propres sauvegardes. Les éléments qui sauvegardent vers un dépôt supprimé suivent de nouveau leur section, et il en va de même pour une valeur par défaut de section qui pointait là. Le dossier d'une section ne peut pas simplement partir, car chaque section en a besoin d'un. La fenêtre demande le dossier vers lequel les sections se déplacent, et chaque section garde son nom de dossier dessous.
- Supprimer les sauvegardes d'une destination fonctionne aussi sans mot de passe de connexion. Si un mot de passe est défini, cela demande une connexion sur l'instance elle-même.

### Exemple : une copie hors site sur Backblaze B2 {#example-s3}

Les conteneurs et l'auto-sauvegarde doivent aussi vivre chez Backblaze B2, pour qu'un serveur mort ne les emporte pas. Les mêmes étapes conviennent à Wasabi, Hetzner Object Storage et à la plupart des autres services S3.

1. **Dans B2,** créez un bucket privé, par exemple `tower-backups`, dans une région proche de chez vous. Sous **Application Keys**, créez une clé avec accès en lecture et en écriture, limitée à ce bucket, et notez son ID de clé et sa clé d'application. B2 montre la clé d'application une seule fois. Réglez le cycle de vie du bucket pour ne garder que la dernière version, sinon B2 garde les données élaguées comme versions masquées et vous les facture.
2. **Fournisseur.** Dans BombVault, ouvrez **Destinations**, appuyez sur **Ajouter une destination** et choisissez **Backblaze B2** sous **Service S3**.
3. **Connexion.** Saisissez l'ID de clé comme **ID de clé d'accès** et la clé d'application comme **Clé d'accès secrète**. Saisissez le point de terminaison du bucket sous **Point de terminaison**, par exemple `s3.eu-central-003.backblazeb2.com`, et BombVault en lit la région. Saisissez `tower-backups` dans **Bucket**, car cette clé n'ouvre qu'un bucket.
4. **Tester la connexion.** Elle devrait dire **Connexion établie** et lister les dossiers du bucket. Appuyez sur **Nouveau dossier** et nommez-le `bombvault`. BombVault y met un dossier par section, par exemple `bombvault/containers`.
5. **Protection.** La fenêtre explique que B2 peut garder d'anciennes versions avec le verrouillage d'objets et que BombVault ne peut pas le vérifier. Laissez **Protection contre la suppression (ajout seul)** désactivée sauf si le bucket refuse vraiment les suppressions, pour que BombVault puisse élaguer selon les règles de conservation de la destination.
6. **Pour quelles sections.** Cochez **Conteneurs** et **Auto-sauvegarde**. L'auto-sauvegarde porte vos réglages, pour qu'une nouvelle installation puisse les récupérer. La fenêtre dit combien d'éléments et, quand c'est connu, combien d'historique la première copie emporte.
7. Appuyez sur **Enregistrer la destination**.

La première copie s'exécute après la prochaine sauvegarde. Pour la lancer maintenant, ouvrez **Paramètres, Destinations** et appuyez sur **Répliquer maintenant** dans la ligne de la section, dans **Copies hors site**. L'en-tête d'état indique **Copie hors site en cours**. La première copie emporte tout l'historique, donc 200 GB sur une liaison montante de 100 Mbit/s prennent environ quatre heures et demie. Les copies suivantes n'envoient que ce qui est nouveau.

Pour garder votre ligne libre, ouvrez la page de la destination et réglez la **Limite d'envoi (KiB/s)** dans la carte **Copier**, par exemple sur `5120`, ce qui se lit environ 5,0 MB/s (41,9 Mbit/s). Profitez-en pour vérifier **Durée de conservation** sur la même page.

Pour voir que ça a marché, regardez **État de protection** sur l'aperçu, où **Hors site** passe à **copie à jour**, et l'onglet **Sauvegardes** d'un conteneur, où ses instantanés nomment maintenant `tower-backups` comme second endroit. Appuyez ensuite de nouveau sur **Télécharger le kit de récupération** sous **Paramètres, Restauration**, car le kit liste la nouvelle destination et ses identifiants.

**Cloudflare R2 fonctionne de la même façon,** avec un jeton d'objet au lieu d'une clé. Collez l'adresse S3 que montre le tableau de bord de R2, avec le bucket dessus, et BombVault remplit le serveur et le bucket.

### Exemple : un rest-server que vous hébergez vous-même {#example-rest-server}

Un rest-server en mode ajout seul refuse les suppressions, si bien qu'une copie dessus survit même si quelqu'un prend le contrôle de votre serveur. Une deuxième machine, chez un ami ou dans le garage, suffit. Un BombVault appairé peut la configurer pour vous, voir [Serveur récepteur](#receiving-server). Pour la lancer à la main sur n'importe quel hôte Docker :

```sh
docker run -d --name rest-server --restart unless-stopped \
  -p 8000:8000 \
  -v /mnt/user/backups/rest-server:/data \
  -e OPTIONS="--append-only --private-repos" \
  restic/rest-server
docker exec -it rest-server create_user bombvault
```

La dernière ligne demande un mot de passe et écrit l'utilisateur dans le fichier `.htpasswd` du dossier de données.

1. Dans BombVault, appuyez sur **Ajouter une destination** et choisissez **rest-server** sous **NAS ou serveur**.
2. Saisissez l'adresse avec l'utilisateur comme dossier, par exemple `http://192.168.2.20:8000/bombvault`, ainsi que le **Nom d'utilisateur** et le **Mot de passe**. Appuyez sur **Tester la connexion**.
3. Dans **Protection**, activez **Protection contre la suppression (ajout seul)**. BombVault n'élague alors jamais là-bas et laisse cela à l'autre côté.
4. Cochez les sections et appuyez sur **Enregistrer la destination**.
5. Une fois la première copie là, ouvrez la page de la destination et appuyez sur **Tester la protection contre la suppression maintenant**. BombVault essaie de supprimer un objet de test. La carte devrait dire **Dernier : suppression refusée**, et **Protection contre la suppression** sur l'aperçu passe à **vérifiée**. Si elle dit **Dernier : suppression non refusée**, `--append-only` manque.

Comme rien n'élague cette copie d'ici, réglez **Budget de croissance (GB, 0 = désactivé)** dans la fenêtre de la section, ouverte avec **Modifier** dans sa ligne de **Copies hors site**, pour être prévenu avant que le disque se remplisse. Avec un certificat autosigné, voir la remarque sur `certs` plus haut.

## Le dossier propre d'une section {#own-folder}

Chaque section a un dossier propre : le dépôt dans lequel ses sauvegardes sont d'abord écrites. Les copies vers d'autres destinations partent de là. Une nouvelle installation garde les dossiers propres sur ce serveur, sous `/mnt/user/bombvault`, et ensemble ils forment la destination **Local**.

Un dossier propre peut aussi se trouver dans une destination que vous avez déjà, comme un NAS, un rest-server ou un bucket S3. La section sauvegarde alors directement là, sans copie locale intermédiaire, et le dossier propre suit les identifiants, la protection contre la suppression et les limites de bande passante de cette destination. BombVault l'élague après chaque sauvegarde selon la carte **Durée de conservation** de la destination, et les valeurs personnalisées d'une section sous **Règles par section** l'emportent sur la carte. La destination liste la section dans sa carte **Dossiers propres**, et **Destinations par section** montre la destination comme le domicile de la section.

### Déplacer un dossier propre {#move-own-folder}

1. Ouvrez la destination qui doit contenir le dossier propre et choisissez **Utiliser comme dossier propre**, ou **Ajouter des sections** dans sa carte **Dossiers propres**. Pour déplacer plusieurs sections à la fois, choisissez **Déplacer les dossiers propres** à côté de **Dossiers propres ici** dans la carte **Connexion** de la destination Local.
2. Choisissez les sections sous **Sections** et l'endroit sous **Déplacer vers**. **Un autre endroit** accepte un dossier sur cet hôte ou l'adresse d'un dépôt distant saisie à la main, avec des identifiants pris parmi vos identifiants enregistrés.
3. Lisez **Ce qui se passe**. Pour chaque section, cela dit où vont les nouvelles sauvegardes, si une copie déjà présente au nouvel endroit devient le dossier propre, et quelles sauvegardes sont d'abord copiées.
4. **Copier les sauvegardes déjà faites** est activé par défaut. Désactivé, les anciennes sauvegardes restent où elles sont, mais l'élément ne les liste plus. **Depuis un ancien dossier de sauvegarde** sur la page Restauration les lit toujours avec l'adresse du dossier et, pour un dossier distant, ses identifiants de connexion.
5. Choisissez **Basculer**. Quand la carte **Durée de conservation** du nouvel endroit garde moins que la règle actuelle de la section, BombVault demande d'abord **Conserver moins ?** Là, **Conserver les valeurs actuelles** de cette section **dans Règles par section** reprend les valeurs d'aujourd'hui, **Enregistrer quand même** poursuit, et **Retour** laisse tout tel quel. Quand des sauvegardes doivent d'abord être copiées, BombVault les copie puis bascule le dossier propre. Quand elles sont déjà là, il les contrôle avant de basculer.

Après le déplacement, les nouvelles sauvegardes vont au dossier propre déplacé, aussi pour une section dont **Local** était désactivé et qui sauvegardait directement vers une copie. Une copie qui se trouvait déjà au nouvel endroit devient le dossier propre, et la copie vers là s'arrête. Là où cette copie gardait certaines sauvegardes plus longtemps que les règles de conservation de la section, la section garde ce que l'une ou l'autre règle garde, si bien qu'aucune n'est perdue.

Quand le déplacement ne peut pas avoir lieu, la fenêtre dit pourquoi dans un encadré sous **Ce qui se passe**. Quand seule la copie des anciennes sauvegardes gêne, désactiver **Copier les sauvegardes déjà faites** laisse le dossier propre se déplacer sans elles.

Une section a toujours besoin d'un dossier propre. Supprimer la destination qui le contient demande le dossier vers lequel la section se déplace, voir [Déplacer et supprimer](#move-remove). Un dossier propre devant lequel passent les nouvelles sauvegardes, parce que la section sauvegarde directement vers une copie, est marqué **(désactivé)** là où une destination liste qui l'utilise.

![La fenêtre qui déplace un dossier propre, avec Ce qui se passe renseigné pour une section.](assets/screenshots/storage/move-own-folder-dark.png#only-dark)
![La fenêtre qui déplace un dossier propre, avec Ce qui se passe renseigné pour une section.](assets/screenshots/storage/move-own-folder-light.png#only-light)

#### Exemple : les conteneurs déménagent vers un NAS {#example-own-folder}

Le pool de cache se remplit, et vous avez une destination appelée `nas`, un partage SMB sur une seconde machine. La section Conteneurs a 61 sauvegardes dans son dossier propre sur ce serveur et copie vers Backblaze B2.

1. Ouvrez **Destinations**, puis `nas`, et appuyez sur **Utiliser comme dossier propre**.
2. Cochez **Conteneurs** sous **Sections**.
3. Lisez **Ce qui se passe**. Cela dit que les nouvelles sauvegardes de Conteneurs vont au dossier sur `nas`, que les 61 sauvegardes de Local y sont d'abord copiées, que les copies vers Backblaze B2 continuent, et que la protection, les limites de bande passante et l'alarme de croissance viennent de `nas`.
4. Laissez **Copier les sauvegardes déjà faites** activé et appuyez sur **Basculer**.

BombVault copie les sauvegardes en arrière-plan puis bascule le dossier propre. La page de `nas` liste Conteneurs sous **Dossiers propres**, un élément qui suit sa section nomme `nas` comme son domicile, et la prochaine sauvegarde y atterrit, avec la copie vers B2 qui part de `nas`. À partir de maintenant, la carte **Durée de conservation** de `nas` élague le dossier. L'ancien dossier sur ce serveur reste où il est, et BombVault n'y supprime rien.

### Saisir une adresse distante à la main {#remote-primary-repositories}

La carte **Connexion** de la destination Local accepte encore une adresse distante pour le dossier propre d'une section : **Modifier** à côté de la section ouvre une fenêtre avec un interrupteur **Local / Distant**. **Distant** accepte une adresse restic comme `s3:...`, `rest:http://host:8000/repo`, `sftp:user@host:/repo` ou `rclone:remote:bucket/path`, et les **Paramètres de sécurité du dépôt primaire distant** ajoutent un test de connexion, des limites d'envoi et de téléchargement, une protection en ajout seul avec le même test d'altération que celui d'une destination hors site, et une alarme de budget de croissance.

Une telle adresse se connecte avec les identifiants par défaut. Quand elle est exactement le dossier qu'une destination que vous avez garde pour la section, elle suit cette destination d'elle-même, carte de la destination et interrupteur **Disponible** compris, sauf si elle se connecte autrement ou est en ajout seul là où la destination ne l'est pas. Une règle qui élaguait le dossier jusque-là reste la règle propre de la section sous **Règles par section**. Quand elle se trouve dans un autre dossier de cette destination, la page de cette destination liste la section sous **Déjà sous cette destination**, et la reprendre lui donne les identifiants, la classe de stockage et la protection contre la suppression de la destination. Déplacer le dossier propre dans une destination avec **Utiliser comme dossier propre** est la voie la plus simple, car la fenêtre contrôle les sauvegardes déjà faites avant que quoi que ce soit change.

## Destinations par élément {#placement}

La carte **Planning et destination** de l'onglet Paramètres d'un élément a une rangée de boutons : **Local** et un bouton par destination hors site de la section. Les boutons allumés reçoivent les sauvegardes de l'élément.

- Avec **Local** allumé, l'élément est écrit dans le dépôt indiqué sous **Stocké sur** et copié vers chaque autre destination allumée. Éteignez une destination et elle ne reçoit plus rien de nouveau de cet élément. Local seul ne copie nulle part, ce qui convient à des données qui ont déjà une seconde copie, par exemple un partage qui vit sur un NAS.
- Avec **Local** éteint, l'élément est écrit directement dans le dépôt direct de la première destination allumée et copié de là vers les autres allumées. La première fois, une boîte de dialogue crée ce dépôt direct.
- Un bouton reste allumé, car une sauvegarde a besoin d'un endroit où aller. Pour laisser un élément de côté, utilisez **Ne pas sauvegarder**.

BombVault ne déplace jamais de sauvegardes entre dépôts. Un élément qui suit sa section suit aussi quand la valeur par défaut de la section change : sa prochaine sauvegarde atterrit au nouvel endroit, et les sauvegardes qu'il a restent où elles sont. Une destination choisie sur l'élément lui-même reste figée dès sa première sauvegarde. La carte **Dépôt** de l'onglet Paramètres la nomme. Les copies peuvent changer à tout moment. Une destination qui ne reçoit plus un élément garde les copies qu'elle a et les taille selon sa propre rétention à la prochaine exécution hors site de la section. Les destinations en ajout seul ne peuvent pas être supprimées.

L'endroit qu'un élément a quitté garde ses sauvegardes. Sur l'onglet **Sauvegardes** de l'élément et sur **Restauration**, il figure comme **Anciennes sauvegardes uniquement**, une restauration peut encore y lire, et un élagage là-bas laisse ces sauvegardes tranquilles.

**Suivre le réglage par défaut** ramène un élément qui a un choix à lui vers sa section. Cela demande d'abord confirmation, dit où vont alors les nouvelles sauvegardes, et nomme ce que chaque destination cesse de recevoir et ce que la prochaine exécution enverrait.

Sous la rangée, la carte dit où va l'élément et ce qui s'y trouve réellement : combien de sites le contiennent, quand chaque destination a été vue pour la dernière fois, et si le 3-2-1 est respecté. Un site est le serveur avec les données d'origine, chaque destination hors site et chaque dépôt marqué **Hors des locaux**. BombVault contrôle les copies et les sites. Il ne contrôle pas la partie « deux supports » du 3-2-1.

### Valeurs par défaut par section {#placement-defaults}

La carte **Destinations par section** de la page Destinations dessine la valeur par défaut de chaque section comme un flux 3-2-1. Chaque ligne d'une section vers une destination est un interrupteur, **Local** compris, pour chaque section, la clé flash, les jeux de données ZFS et l'auto-sauvegarde aussi. Un clic sur une ligne ou un point l'active ou la désactive aussitôt, et les lignes en pointillés sont désactivées. Seule la désactivation de **Local** d'une section demande d'abord confirmation, dans la même boîte de dialogue que la carte de la valeur par défaut, car la section sauvegarde alors directement vers une copie. **Annuler** laisse tout comme avant. **Graphe / Interrupteurs** montre la même chose sous forme de rangées de boutons. Un interrupteur décide où vont les nouvelles sauvegardes. Les éléments sans choix propre, et les dossiers de projet des stacks Compose, suivent aussitôt, si bien que leur prochaine sauvegarde va aux nouveaux endroits. Les sauvegardes déjà stockées restent où elles sont et peuvent encore y être restaurées. La clé flash, les jeux de données ZFS et l'auto-sauvegarde sauvegardent vers leur dossier local et copient hors site à partir de cette sauvegarde. Leur ligne vers **Local** peut aussi être désactivée. Leurs sauvegardes vont alors directement à leurs destinations hors site, et un élément ZFS avec une réplique et aucune destination n'envoie que sa réplique.

Avec **Local** désactivé et seulement des destinations hors site activées, une section sauvegarde directement vers elles, et BombVault choisit lui-même le dépôt là-bas. Les conteneurs, les VMs et les ensembles de dossiers vont au dépôt direct de la première destination activée, décrit plus bas, et sont copiés de là vers les autres. La clé flash, les jeux de données ZFS et l'auto-sauvegarde écrivent chaque sauvegarde dans chaque destination activée, et chaque destination les garde selon sa propre rétention.

Une section dont toutes les lignes sont désactivées dit **Pas de sauvegarde**. Ses éléments ne reçoivent aucune sauvegarde et comptent comme non protégés.

Les éléments qui ont choisi leurs propres destinations sur leur page les gardent. Le flux dit combien il y en a dans une section, par exemple **2 éléments diffèrent**, et y mène : vers l'onglet Paramètres de l'élément quand il n'y en a qu'un, vers une liste avec l'endroit où chacun sauvegarde quand il y en a plusieurs. **Suivre le réglage par défaut** y ramène un élément.

Une nouvelle destination hors site reçoit chaque élément qui n'est pas réglé sur Local. La fenêtre qui l'ajoute dit combien d'éléments et, quand c'est connu, combien d'historique cela représente.

### Dépôts directs {#direct-repositories}

Un dépôt direct est l'endroit où des éléments sauvegardent directement vers une destination hors site. BombVault le place à côté du dossier de la destination avec `-direct` ajouté au nom, ou, quand la destination est à la racine de son bucket, de son remote ou d'un utilisateur rest-server, dans un dossier à l'intérieur nommé d'après la section, comme `containers-direct`. Dans le flux de la page Destinations, il est créé sans question. Éteindre Local pour un seul élément, de sorte qu'une destination sans dépôt direct devienne son domicile, ouvre une boîte de dialogue avec la destination suggérée, par exemple `s3:https://s3.eu-central-003.backblazeb2.com/bucket/containers-direct`, et un test de connexion qui ne crée rien. **Créer et utiliser** crée le dépôt et y fait pointer l'élément. Un dépôt direct n'a aucun réglage propre. Il suit l'interrupteur, les identifiants, la classe de stockage, les limites, le réglage d'ajout seul et la rétention de sa destination, et change avec eux. La page de la destination le liste sous **Sauvegardé directement ici**, avec chaque section qui sauvegarde directement vers lui et combien d'éléments le font. Un dépôt vers lequel plus rien ne sauvegarde le dit là et peut être supprimé, et les sauvegardes à son endroit restent. Quand une nouvelle clé de la destination ne peut pas l'ouvrir, le dépôt direct garde la clé qu'il a et l'enregistrement le dit. Un élément sur un dépôt direct est copié de là vers les autres destinations allumées, jamais vers la destination à laquelle appartient le dépôt. Ses instantanés portent l'étiquette `bv:direct`, et chaque autre passe de rétention les garde, si bien qu'un dépôt direct qui a perdu son lien avec sa destination ne vieillit jamais selon les règles locales. B2 est joint par son point de terminaison S3, avec l'ID de clé et la clé d'application saisis comme clés S3. Une clé limitée au propre dossier de la destination ne peut pas atteindre le dossier voisin, limitez donc la clé au dossier du dessus.

### Hors des locaux {#off-the-premises}

Une destination ou un dépôt peut être marqué **Hors des locaux**. Les distants démarrent marqués. Désactivez-le pour un rest-server dans le même bâtiment. La marque ne sert qu'à compter les sites et le 3-2-1 sur les cartes. Elle ne change aucune copie.

Une réplique ZFS est une copie hors des locaux, mais pas une sauvegarde avec historique. Elle ne garde que les instantanés que vous réglez, et les anciennes versions et les fichiers isolés viennent toujours des sauvegardes. Elle va d'un serveur ZFS directement à l'autre, via SSH vers un serveur ZFS ou vers le point de réception d'une instance appairée. Le groupe d'appairage ne porte que la demande. Voir [Réplique](zfs-datasets.md#replica).

### Après une reconstruction {#after-a-rebuild}

Les choix de copie vivent dans les propres réglages de BombVault. Après une reconstruction par **Découvrir les sauvegardes** ou l'assistant d'urgence sans réglages restaurés, ils ont disparu, et tout copier enverrait de nouveau vers B2 les éléments que vous aviez laissés de côté. Il en va de même quand une section qui n'a jamais copié hors site depuis cette base de données trouve ses propres sauvegardes sur une destination, ou des sauvegardes plus anciennes dans son dossier propre, par exemple après l'ajout d'une destination par-dessus les sauvegardes d'une installation précédente avec la même `APP_KEY`. Les copies hors site de cette section attendent alors votre décision.

**À traiter** sur l'aperçu et la page de chaque destination vers laquelle la section copie demandent si les sauvegardes sont bien celles de ce serveur. **Les adopter pour ce serveur** confirme le réglage par défaut de la section, et ses copies reprennent dès le passage suivant. **Vérifier d'abord les règles** mène à la ligne de la section dans **Destinations par section**, où vous voyez où va chaque élément et pouvez régler de nouveau une règle perdue sur la page de l'élément. La copie continue d'y attendre jusqu'à ce que vous appuyiez sur **Les adopter pour ce serveur**, qui montre d'abord ce que copiera la prochaine exécution et les noms figurant dans les sauvegardes qui n'ont pas d'élément, que vous pouvez laisser de côté. La sauvegarde la plus récente de chaque élément en attente le signale dans son exécution, avec un lien vers la décision, et une notification part quand les notifications sont activées. Importer un fichier de réglages ramène les règles et les valeurs par défaut mais ne prend pas la décision à votre place.

![À traiter après une reconstruction : les ensembles de dossiers ont trouvé des sauvegardes antérieures à cette base de données, et leurs copies hors site attendent la décision.](assets/screenshots/offsite/copy-decision-dark.png#only-dark)
![À traiter après une reconstruction : les ensembles de dossiers ont trouvé des sauvegardes antérieures à cette base de données, et leurs copies hors site attendent la décision.](assets/screenshots/offsite/copy-decision-light.png#only-light)

## Protection contre la suppression (ajout seul) {#delete-protection-append-only}

Activez la protection contre la suppression pour une destination hors site afin qu'un rançongiciel, ou un hôte compromis, ne puisse ni supprimer ni réécrire vos sauvegardes. L'autre côté (un `restic/rest-server` lancé en mode `--append-only`) l'**impose**. BombVault ne fait que la **vérifier** et n'affiche jamais de vert sur une simple déclaration de configuration. Un dossier sur ce serveur ne peut pas refuser une suppression, car quiconque contrôle BombVault contrôle aussi le dossier, les destinations locales ne proposent donc pas l'interrupteur.

L'interrupteur appartient à la destination : une section qui copie là le reprend de la destination, activez-le donc depuis la page de la destination sous **Destinations**. La **Vérifier la protection contre la suppression** planifiée ne s'exécute que tant qu'une section copie vers une destination dont l'interrupteur est activé. L'autre côté doit être démarré avec `--append-only`. Avec l'image `restic/rest-server`, sur TrueNAS comme sur tout autre hôte Docker, cela va dans la variable `OPTIONS`.

Certains endroits ne peuvent pas du tout refuser une suppression, un lecteur cloud comme OneDrive par exemple. Si vous y gardez des copies exprès, activez **Accepter sans protection contre la suppression** sur la page de la destination. L'aperçu montre alors l'absence de protection contre la suppression d'une section qui ne copie que vers des destinations acceptées comme un risque accepté, un anneau gris au lieu d'un point ambre, et la couleur de la section suit ses autres contrôles. Rien d'autre ne change : les sauvegardes qui y sont peuvent toujours être supprimées, BombVault les élague toujours selon les règles de conservation de la destination, et l'interrupteur disparaît dès que la protection contre la suppression est activée. Une copie qui passe à un autre endroit perd l'acceptation. Activer la protection contre la suppression l'efface aussi.

Le moyen le plus rapide d'obtenir une copie en ajout seul est un BombVault appairé avec un [serveur récepteur](#receiving-server) : il lance le rest-server pour vous, en mode ajout seul.

Dans un [dépôt partagé](#shared-repository), cela vaut pour les membres et pas pour le serveur qui l'héberge. Ce serveur accède directement au dossier et y oublie et élague les sauvegardes.

!!! note "Une suppression réussie sous `/locks/` est normale"
    Ajout seul ne veut pas dire que rien ne peut jamais être retiré. restic doit prendre et relâcher ses propres verrous, `/locks/` reste donc inscriptible et supprimable par conception. Les instantanés et les données qui les sous-tendent, ce que viserait un rançongiciel, ne peuvent pas être supprimés. Si vous sondez vous-même l'autre côté, une suppression qui réussit sous `/locks/` est un comportement correct et non une faille dans la protection.

!!! warning "Les dépôts en ajout seul ne sont jamais élagués depuis ce serveur"
    Une destination en ajout seul n'élague jamais d'anciens instantanés depuis ici. L'élagage est le travail de l'autre côté. Réglez-lui une **alarme de budget de croissance** pour être alerté avant que la taille du dépôt s'emballe.

## Test d'altération {#tamper-test}

BombVault prouve périodiquement la garantie d'ajout seul en tentant réellement une suppression sur le dépôt hors site, visant un objet qui n'existe pas. **Tester la protection contre la suppression maintenant**, sur la page d'une destination, le lance à la demande, et **Vérifier la protection contre la suppression** sous **Paramètres, Vérification, Vérifications automatiques** le lance selon un planning.

Désactiver **Vérifier la protection contre la suppression** ne cache pas une lacune. Une destination avec protection contre la suppression qui n'a jamais été testée se lit alors « non prouvée, son contrôle est désactivé », et une destination sans protection contre la suppression se lit « aucune » dans les deux cas.

Une copie désactivée n'est pas testée, et la destination le dit. Le test de connexion s'exécute quand même dessus, car il ne fait que lire, si bien qu'une copie peut être contrôlée avant d'être activée.

- **Refusée** veut dire protégée.
- **Acceptée** veut dire non protégée.
- Un résultat **non concluant** (serveur injoignable, erreur d'authentification) ne fait jamais basculer le verdict enregistré.

Un vrai basculement de protégé à non protégé déclenche une seule alerte.

## Exercices de reprise {#dr-drills}

BombVault offre deux niveaux de preuve que vos sauvegardes sont réellement restaurables, pas seulement présentes.

- **Exercices de vérification de restauration (local).** BombVault exécute périodiquement `restic check --read-data-subset` (borné, jamais une restauration complète qui remplit le disque). La cadence se règle sous **Paramètres, Vérification, Vérifications automatiques**, et la carte **Vérifier les sauvegardes** de la même tuile montre le résultat.
- **Restauration de test (hors site).** BombVault restaure une vraie cible depuis le dépôt hors site dans un bac à sable isolé, la vérifie fichier par fichier et octet par octet, puis nettoie. Cela prouve que vous pouvez récupérer depuis le hors site, pas seulement que le dépôt répond.

La carte **État de protection** de l'aperçu résume cela par section en quatre contrôles : une sauvegarde à jour sur ce serveur, une copie hors site, la protection contre la suppression là-bas et une restauration de test depuis là. Un clic sur une section montre le détail, et la carte ne passe au vert que sur des faits vérifiés. Une destination que vous avez acceptée sans protection contre la suppression y compte comme un risque accepté, pas comme un avertissement.

## Appairage des instances {#pairing}

Plusieurs BombVault travaillent ensemble comme un groupe : ils se montrent mutuellement leur état de protection, prennent des rôles les uns pour les autres et peuvent garder la clé des autres. Une instance rejoint le groupe avec douze mots.

![La page d'appairage avec ses trois cartes d'étapes.](assets/screenshots/offsite/pairing-dark.png#only-dark)
![La page d'appairage avec ses trois cartes d'étapes.](assets/screenshots/offsite/pairing-light.png#only-light)

Sur la première instance, ouvrez **Paramètres, Appairage** et appuyez sur **Générer une phrase**. Trois cartes d'étapes montrent le déroulement. Douze mots apparaissent dans une fenêtre avec un bouton **Copier**. Sur chaque autre instance, ouvrez le même endroit, appuyez sur **Saisir une phrase** et collez-les ou tapez-les, ou appuyez sur **Coller** dans cette fenêtre. Un mot qui n'est pas dans la liste est nommé avec sa position pendant que vous tapez, et le dernier mot porte une somme de contrôle, si bien qu'un mot mal saisi ou échangé est détecté avant tout appairage. Générez la phrase sur une seule instance : deux instances qui en génèrent chacune une forment deux groupes distincts. Si personne n'apparaît pendant une minute, la page propose deux issues : montrer de nouveau les mots pour les saisir là-bas, ou saisir les mots de l'autre instance et rejoindre son groupe en une seule étape. L'appairage fonctionne sans mot de passe de connexion, mais définissez-en un : sans lui, toute personne qui peut ouvrir l'interface web peut lire les mots et, via le groupe, demander des rôles qui remettent le mot de passe restic des instances qu'il contient. La carte d'appairage le dit tant qu'aucun mot de passe n'est défini. Avec un mot de passe, montrer de nouveau la phrase le demande. **Quitter le groupe** retire une instance. La carte **Instances** de la page d'appairage liste les membres du groupe. L'image de qui fait quoi pour qui se trouve sur la page Instances.

Toute personne qui connaît les mots peut rejoindre le groupe, traitez-les donc comme un mot de passe.

**Comment les membres se joignent.** Chaque instance apprend sa propre adresse sur le réseau par votre navigateur au moment où vous vous connectez, listée sous **Adresses connues** dans la carte **Cette instance** de la page d'appairage avec celles que l'instance a trouvées elle-même. Corrigez-la là si un reverse proxy ou un port inhabituel se trouve devant. Sur le même réseau, les membres annoncent cette adresse par multicast et se parlent directement, et là où le multicast ne peut pas traverser un réseau de conteneur, comme le bridge par défaut de Docker, une instance cherche plutôt les autres dans son propre sous-réseau avec un appel signé auquel seul un membre du groupe peut répondre, si bien que l'appairage se termine quand même en quelques secondes sans relais. Si rien n'apparaît, **Vous ne la trouvez pas ?** accepte une adresse à la main, pour un autre sous-réseau ou un port non standard. Les instances sur des réseaux différents passent par un relais, choisi dans une rangée de quatre dans la carte **Relais** de la même page. Chaque forme vient avec une image :

- **Relais du projet** (le réglage par défaut) : `parleyport.halleluja.design`, le relais qu'utilise aussi KnightLoader. Rien à configurer.
- **Relais personnel** : le conteneur [**ParleyPort**](https://github.com/junkerderprovinz/parleyport) des Unraid Community Apps (aussi sous **Paramètres, Applications**). Saisissez son adresse sur chaque instance qui doit l'utiliser.
- **Instance BombVault** : l'une de vos instances déjà joignable de l'extérieur, avec **Servir de relais** activé. Cette instance répond alors à `/relay/connect` sur sa propre adresse, derrière le reverse proxy et le certificat qu'elle a déjà, et ne laisse entrer que votre groupe. Saisissez son adresse sur chaque instance qui doit l'utiliser.
- **Aucun relais** : les membres se trouvent automatiquement sur le même réseau, et nulle part ailleurs.

**Ce que voit le relais.** Un bouton du même nom à côté des quatre formes le montre pour la forme choisie. Chaque appel entre membres est scellé avec AES-256-GCM sous une clé dérivée des douze mots, et cette clé ne quitte jamais vos instances. Le relais apprend un hachage qui regroupe les connexions, pour quelle instance est un message, sa taille et le moment où il passe. Un appel direct sur le réseau local est scellé de la même façon et signé en plus, si bien que rien ne dépend du certificat autosigné qu'une instance sert. Les appels qui remettent quelque chose (réponses de rôle, mot de passe restic, connexions de récepteurs, dépôts de clés) sont aussi signés avec une clé du membre qui les envoie, épinglée à la première vue. Voir [Modèle de sécurité](configuration.md#security-model).

**Ce qui voyage dans le groupe.** L'état de protection sur la page Instances, une demande de contrôler une section maintenant, les demandes de rôle et leurs réponses, les offres de stockage, et ce dont un récepteur ou un rapatrieur a besoin une fois son rôle autorisé : les emplacements de dépôt de l'autre instance et son mot de passe restic. Les données de sauvegarde, jamais. Elles vont toujours directement vers les moteurs restic. L'APP_KEY non plus : le mot de passe restic ouvre les dépôts de cette instance et rien d'autre, ni ses secrets stockés, ni ses sessions, ni ses codes de récupération.

**Éléments d'avant l'appairage.** Les instances ajoutées avec un jeton de flotte, et les dépôts reçus et sources de rapatriement configurés avec l'APP_KEY de l'autre instance, restent après la mise à jour et sont marqués **Appairer à nouveau**. Ils continuent de fonctionner : à son premier démarrage, BombVault remplace chaque APP_KEY enregistrée par le mot de passe restic qui en est dérivé. Appairez les deux instances, puis modifiez l'élément et choisissez son instance. D'ici là, de tels éléments figurent dans la carte **Sans instance** de la page Instances.

Le seul endroit qui prend encore une APP_KEY à la main est [Depuis un ancien dossier de sauvegarde](#restore-from-another-bombvault-repo), pour le cas où l'autre instance a disparu et ne peut pas répondre dans un groupe.

## Instances et rôles {#roles}

La page **Instances** (une tuile de paramètres qui peut être épinglée à la barre latérale) montre le groupe comme un réseau de qui fait quoi pour qui, une carte pour cette instance et une carte pour chaque autre membre, les téléphones Android et les serveurs ZFS. **Appairage**, en bas à droite, mène à la page d'appairage. Les éléments activés se règlent sous **Paramètres, Instances, Plusieurs serveurs** : **Instances**, **Récepteur** et **Rapatriement**.

![La page Instances : le groupe comme un réseau, et une carte par instance avec ses tuiles de rôles, l'une d'elles Inactive.](assets/screenshots/offsite/instances-dark.png#only-dark)
![La page Instances : le groupe comme un réseau, et une carte par instance avec ses tuiles de rôles, l'une d'elles Inactive.](assets/screenshots/offsite/instances-light.png#only-light)

Chaque carte montre l'état de protection du membre et ses rôles sous forme de tuiles. Un rôle est quelque chose qu'une instance fait pour une autre :

- **Récepteur** garde les copies hors site de ce serveur dans un rest-server en mode ajout seul. Ce serveur envoie après chaque sauvegarde et ne peut rien y supprimer, même si quelqu'un en prend le contrôle.
- **Rapatrieur** rapatrie les sauvegardes de ce serveur selon son propre planning. Ce serveur n'a pas besoin d'être éveillé pour cela. L'instance qui rapatrie doit avoir **Rapatriement** activé avant de pouvoir autoriser la demande.
- **Serveur ZFS** prend les répliques ZFS de ce serveur. Chaque élément ZFS le choisit sous **Réplique** dans ses paramètres.
- **Gardien** garde une copie scellée du kit de récupération de ce serveur. Voir [Gardien](#key-keeper).

Activer un rôle envoie une demande, et rien ne démarre tant que quelqu'un sur l'autre instance n'a pas appuyé sur **Autoriser**. La demande apparaît sur la carte de cette instance sous forme de badge, et ses **Détails** la listent avec **Autoriser** et **Refuser**. Un rôle autorisé peut y être révoqué à tout moment. Une ligne en pointillés dans le réseau est un rôle encore en attente. Une instance sous BombVault 9.x ne peut pas répondre à une demande, si bien qu'un rôle y est actif dès que vous l'activez, et une demande d'une instance 9.x attend **Autoriser** ici.

**Détails** sur une carte ouvre le reste : les dépôts reçus et les sources de rapatriement de ce membre, les offres de stockage, la surveillance, **Supprimer** et **Oublier la clé**.

Sous les cartes, la carte **Dépôt partagé** sur une machine avec un serveur récepteur et la carte **Dépôts partagés** sur un membre concernent le [dépôt partagé](#shared-repository).

Un membre peut offrir son propre stockage hors site à un autre membre via le groupe. L'autre administrateur voit l'offre dans les **Détails** du membre et l'accepte ou la refuse. L'accepter crée des identifiants enregistrés et une destination ordinaire, après les mêmes contrôles que pour une destination que vous saisissez. La destination arrive avec la protection contre la suppression activée, car la recette que montre l'offre est un rest-server en ajout seul, si bien que **Durée de conservation** ne s'y applique pas. Chaque offre a son propre identifiant de connexion et son propre dossier, si bien qu'offrir de nouveau une section, au même membre ou à un autre, laisse tranquilles les copies du premier partenaire : ajoutez la nouvelle ligne à `.htpasswd` et laissez le serveur tourner. Seuls des détails de connexion voyagent ainsi, jamais de données de sauvegarde. **Vérifier maintenant** demande à un membre de vérifier le dépôt d'une section. Rien sur la page ne peut lancer une sauvegarde, une restauration ni supprimer quoi que ce soit sur une autre machine.

Les instances éteintes et les téléphones dont l'application est fermée gardent leur carte comme **Inactive**, en gris, avec le moment où ils ont été vus pour la dernière fois, et apparaissent estompés dans le réseau jusqu'à ce que vous les **Supprimiez**. Juste après le démarrage de ce serveur, une telle carte dit **Vérification** jusqu'à ce que le groupe soit revenu. **Supprimer** fonctionne sur une telle carte quels que soient les éléments activés, et met aussi fin aux demandes ouvertes du membre, à ses emplacements de réception ZFS inactifs et à ses demandes de clés en attente. La carte **Instances** de la page d'appairage liste aussi ces membres comme **Inactive**.

## Récepteur et rapatrieur {#receiver}

Sur la machine qui **reçoit** des copies en ajout seul d'un autre BombVault, les dépôts reçus vous donnent une surveillance indépendante en lecture seule sur le matériel récepteur, si bien qu'une panne silencieuse à l'autre bout ne passe pas inaperçue. Activez **Récepteur** sous **Paramètres, Instances**, puis ouvrez les **Détails** de l'instance émettrice sur la page Instances et ajoutez le dépôt reçu (en lecture seule, ouvert avec le mot de passe restic de l'instance émettrice, qu'il reçoit via le [groupe d'appairage](#pairing)). Vous obtenez :

- **Un inventaire par source**, pour voir exactement quels conteneurs, VMs et ensembles de dossiers sont arrivés. Les sauvegardes que l'instance émettrice a faites avant d'avoir son [propre hôte](#backup-host) comptent dans la même source.
- **Dernière réception** par source, pour savoir à quel point chacune est fraîche.
- **Un `restic check` indépendant** exécuté sur le matériel récepteur, si bien que l'intégrité est vérifiée là où les données se trouvent réellement, pas seulement chez l'émetteur.
- **Un interrupteur d'homme mort :** une alerte quand une source cesse d'envoyer dans une fenêtre que vous réglez. Il regarde une fois par jour, à 09:15. Vous recevez aussi une alerte quand le dépôt ne peut pas être lu du tout, quel que soit le planning des contrôles, et quand rien n'est arrivé une fenêtre entière après que vous l'avez ajouté. Un dépôt que vous ajoutez avant la première copie de l'émetteur est en règle jusque-là.
- **Des alertes d'intégrité :** une alerte quand un contrôle côté récepteur échoue. Un contrôle qui n'a pas pu ouvrir le dépôt ne dit rien de l'intégrité, il ne laisse donc aucun verdict et réessaie le lendemain à la même heure.

Un dépôt sur un autre serveur a besoin des identifiants de connexion de ce serveur : enregistrez-les comme [identifiants](#sign-ins) et choisissez-les sous **Identifiants** dans la fenêtre.

Le récepteur est strictement en lecture seule. Il n'écrit jamais dans le dépôt reçu, il ne peut donc jamais briser la garantie d'ajout seul sur laquelle compte l'émetteur.

Un rapatrieur fonctionne dans l'autre sens : avec **Rapatriement** activé, une instance lit les sauvegardes d'un autre membre selon son propre planning, dans un dépôt à elle pour chaque source, si bien qu'elles ne comptent jamais comme les propres sauvegardes de cette machine. Ses sources figurent aussi dans les détails de ce membre : l'emplacement du dépôt, le type de sauvegarde qu'il contient et la fréquence de rapatriement. Le mot de passe restic passe par le groupe, jamais l'APP_KEY, et l'autre côté n'a rien de plus à configurer et n'a pas besoin d'être en marche. La source n'est jamais que lue, et chaque côté garde ses propres identifiants. Un rapatriement s'exécute au jour et à l'heure que dit son planning, et une nouvelle source attend aussi ce moment. **Rapatrier maintenant** n'attend pas. Un rapatriement manqué parce que cette machine était éteinte s'exécute dès qu'elle est de retour. Un rapatriement qui a échoué réessaie le lendemain à la même heure, aussi avec un planning hebdomadaire, et un rapatriement échoué apparaît dans le journal d'activité et envoie une seule alerte, jusqu'à ce qu'un rapatriement fonctionne de nouveau. Une source `rclone:` est refusée, car rclone l'atteindrait avec les remotes de cette instance.

Une copie rapatriée ne fait que grandir. BombVault y ajoute des données et ne l'élague ni ne la contrôle, si bien que ce que la source a oublié depuis longtemps reste ici jusqu'à ce que vous supprimiez le dossier, et la page Destinations ne la compte pas. Elle se trouve dans `pulled/<source id>` à l'intérieur du dossier propre de la section, et quand ce dossier est déplacé, la copie reste où elle est et les rapatriements suivants continuent là-bas au lieu de tout recopier. Si vous la supprimez, le rapatriement suivant repart de zéro dans le dossier propre qu'a alors la section.

### Serveur récepteur {#receiving-server}

La machine réceptrice peut aussi lancer le rest-server vers lequel les autres copient. Il est configuré la première fois que vous autorisez une demande **Récepteur** qui le réclame : la fenêtre dit que le serveur a besoin d'un serveur récepteur et demande un dossier sur un partage, avec **Nouveau dossier** pour en créer un, et un port (8000 sauf si un autre conteneur l'utilise). BombVault alors :

1. refuse si un conteneur appelé `rest-server` existe déjà ou si un autre conteneur tient le port,
2. tire `restic/rest-server` et le démarre via le socket Docker en mode ajout seul avec des dépôts privés et un fichier de connexion dans le dossier,
3. écrit son modèle Unraid sur la clé flash, pour que le conteneur reste modifiable dans l'onglet Docker, ou propose le modèle en téléchargement quand la clé flash est hors d'atteinte,
4. exécute le test d'altération contre lui et montre s'il refuse les suppressions.

Les instances de votre groupe trouvent alors le serveur parmi les fournisseurs sous **Depuis votre groupe**, nommé d'après la machine réceptrice, et le rôle **Récepteur** le propose comme choix sûr. Chaque instance reçoit un identifiant à elle la première fois qu'elle utilise le serveur, et n'écrit que dans son propre dossier là-bas. Avec **Récepteur** activé sous **Paramètres, Instances**, **Serveur récepteur** sur la carte de cette instance rouvre le serveur. Sa fenêtre liste les identifiants sous **Partenaires avec identifiant**, et **Révoquer l'identifiant** en retire un. Ce que cette instance a déjà copié reste dans le dossier. La configuration crée aussi un **Identifiant hors du groupe**, dont la fenêtre montre le mot de passe une seule fois.

Une instance qui ne joint la machine réceptrice que par le relais ne peut pas utiliser le serveur, car le relais ne porte aucune sauvegarde. Ajoutez d'abord l'adresse de la machine réceptrice sur la page d'appairage. Quand BombVault tourne sur une adresse IP à lui (sur br0, par exemple), renseignez **Adresse pour les partenaires**, car le serveur écoute sur l'adresse de l'hôte.

Une machine réceptrice peut aussi héberger un [dépôt partagé](#shared-repository) pour le groupe.

## Dépôt partagé {#shared-repository}

Plusieurs serveurs d'un groupe peuvent copier ensemble dans un même dépôt restic. Les données qu'ils ont en commun, comme le même ensemble de dossiers ou les mêmes appdata, n'y sont stockées qu'une fois, si bien qu'ensemble ils ont besoin de moins de place qu'avec un dépôt chacun.

Un serveur du groupe héberge le dépôt : c'est l'hébergeur, et le seul qui y oublie les anciennes sauvegardes et qui élague. Les autres sont membres : ils atteignent le dépôt via un second rest-server en mode ajout seul et ne peuvent qu'y ajouter. Chaque serveur écrit sous un [hôte qui lui est propre](#backup-host), si bien que les sauvegardes restent séparées même quand deux serveurs ont un élément du même nom.

Ce qu'il faut :

- un [groupe d'appairage](#pairing),
- **Chiffrer les sauvegardes** activé, sur l'hébergeur et sur chaque membre, car chaque serveur reçoit sa propre clé restic,
- un [serveur récepteur](#receiving-server) sur l'hébergeur, que les membres peuvent joindre. Quand BombVault tourne sur une adresse IP à lui, renseignez d'abord **Adresse pour les partenaires** dans la fenêtre du serveur récepteur.

Un serveur héberge au plus un dépôt partagé.

### Le mettre en place {#shared-set-up}

Une fois le serveur récepteur en marche, la page **Instances** de sa machine a une carte **Dépôt partagé** sous les instances. Appuyez sur **Configurer le dépôt partagé**, choisissez un **Dossier du dépôt partagé** sur un partage avec beaucoup de place, gardez le **Port** à 8001 sauf si un autre conteneur l'utilise, et appuyez sur **Configurer et démarrer**. BombVault alors :

1. refuse si un conteneur appelé `rest-server-shared` existe déjà ou si un autre conteneur tient le port,
2. crée le dépôt dans un dossier `shared` à l'intérieur, avec le mot de passe de ce serveur,
3. démarre `restic/rest-server` sous le nom `rest-server-shared`, en mode ajout seul avec un fichier de connexion dans le dossier,
4. sur Unraid, écrit son modèle sur la clé flash, pour que le conteneur reste modifiable dans l'onglet Docker,
5. exécute le test d'altération contre lui. **Vérifier à nouveau** sur la carte le relance.

La carte liste les **Membres**, chacun avec ses clés et sa dernière sauvegarde, les demandes d'adhésion, et **Retirer** pour chaque membre.

### Rejoindre {#shared-join}

Sur le serveur qui veut rejoindre le dépôt, ouvrez **Ajouter une destination**. Sous **Depuis votre groupe**, l'hébergeur apparaît une seconde fois, comme **Dépôt partagé sur** suivi de son nom. Le choisir envoie une demande à l'hébergeur, où elle figure sur la carte **Dépôt partagé** avec **Autoriser** et **Refuser**.

Après **Autoriser**, l'hébergeur remet via le groupe une clé à usage unique et un identifiant sur son second rest-server. Le membre ajoute avec elle sa propre clé restic, puis l'hébergeur retire la clé à usage unique. Un membre qui n'a pas terminé dans la journée perd de nouveau la clé à usage unique et l'identifiant, et peut redemander. Quand l'hébergeur n'a pas pu joindre le membre avec sa réponse, la carte propose **Le lui redire**.

L'hébergeur refuse un serveur tant que celui-ci n'écrit pas encore sous son propre hôte, ce qui veut dire qu'il tourne sur une version plus ancienne, et quand un autre membre écrit déjà sous le même hôte. Deux serveurs partagent un hôte quand ils partagent une `APP_KEY`.

Une fois le membre admis, une destination appelée **Shared with** suivi du nom de l'hébergeur apparaît sous **Destinations**, avec chaque section désactivée. Activez sur la page de la destination les sections qui doivent y copier. La destination a la protection contre la suppression et pas de règles de conservation à elle, car seul l'hébergeur y oublie des sauvegardes. Seules les sauvegardes sous le propre hôte du membre y vont, si bien que les sauvegardes plus anciennes sous l'hôte `bombvault` restent où elles sont.

L'hébergeur peut aussi y copier ses propres sauvegardes : **Copier aussi les sauvegardes de ce serveur** sur sa carte y ajoute aussi la destination. Pour l'hébergeur, cette copie se trouve sur ses propres disques et rien ne l'empêche d'y supprimer, ce n'est donc pas une copie hors site.

### Qui fait le ménage, et quand {#shared-upkeep}

Seul l'hébergeur oublie et élague. Ses réglages se trouvent dans la partie **Dépôt partagé** de la fenêtre **Serveur récepteur**, qui s'ouvre depuis la propre carte de l'hébergeur sur la page Instances :

- **Fenêtre de ménage** fixe le moment de l'élagage, chaque jour ou chaque semaine. Tant que vous ne la changez pas, elle commence le dimanche à 03:00, et **Durée de la fenêtre en heures** vaut 6.
- **Espace inutilisé qu'un élagage peut laisser, en %** vaut 5 sauf si vous le changez. Une valeur plus haute reconditionne moins, si bien que l'élagage finit plus tôt.
- **Données relues après un élagage, en %** vaut 5 sauf si vous le changez. Après chaque élagage, l'hébergeur vérifie le dépôt et relit cette part des données.
- **Durée de conservation** suit les règles hors site de l'hébergeur pour chaque section, et **Définir une règle propre** donne au dépôt partagé la sienne. Les sauvegardes de chaque serveur vieillissent chacune de leur côté.
- **Faire le ménage maintenant** oublie, élague et vérifie tout de suite, après avoir prévenu les membres.

En dehors de la fenêtre, l'hébergeur oublie une fois par jour, et l'espace revient avec l'élagage suivant. Avant un élagage, il prévient les membres, et ceux-ci ne copient rien dans le dépôt jusqu'à la fermeture de la fenêtre. Un membre qui copie encore a jusqu'à une demi-heure pour finir, sinon l'élagage passe à la fenêtre suivante. Une copie qui tombe dans la fenêtre attend, apparaît comme reportée dans le journal d'activité et sur la carte du membre, et le passage suivant rattrape le retard.

Quand l'hébergeur a un doute, il garde davantage. Un élément qu'un membre retient à cause d'une [anomalie](anomalies.md) garde toutes ses sauvegardes, un membre qui ne peut pas dire ce qu'il retient garde cette fois-là toutes ses sauvegardes, et les sauvegardes sous un hôte qu'aucun serveur du groupe n'utilise ne sont jamais oubliées. La fenêtre montre aussi l'espace : ce que contient le dépôt, ce dont les serveurs auraient besoin séparément, et ce que cela économise. Les chiffres arrivent avec chaque ménage.

### Ce que voient les membres {#shared-members}

Sur un membre, la page Instances a une carte **Dépôts partagés** avec chaque dépôt partagé dans lequel le serveur copie ou qu'il a demandé à rejoindre, et où en est sa demande : **En attente d'un oui**, **Adhésion en cours**, **Membre**, **Refusé**, **Expiré** ou **Plus membre**. Pour un dépôt dont il fait partie, la carte montre **Qui élague**, **Prochain ménage**, **Partagé avec** pour les autres membres, **Dernière copie ici**, et combien d'espace le dépôt occupe et économise.

### Partir {#shared-leave}

**Quitter** sur la carte du membre retire le serveur : l'hébergeur retire sa clé et son identifiant, et sa destination là-bas est désactivée. **Retirer** sur la carte de l'hébergeur retire un membre depuis l'autre côté. Dans les deux cas, les sauvegardes du serveur restent dans le dépôt et vieillissent selon les règles de l'hébergeur. Sur l'hébergeur, elles figurent sous **Serveurs partis**, où **Oublier les sauvegardes** les oublie toutes une fois que vous avez saisi le nom du serveur. La fenêtre de ménage suivante rend l'espace.

### Les sauvegardes des autres serveurs {#shared-restore}

Chaque serveur trouve ses propres sauvegardes du dépôt partagé dans l'onglet **Sauvegardes** de ses éléments, comme toute autre copie. Les sauvegardes des autres membres se trouvent sur la page Restauration sous [Autres serveurs](restore.md#other-servers).

### Après une urgence {#shared-emergency}

Le [kit de récupération](#recovery-kit) a un chapitre **Shared repositories**. Il nomme chaque dépôt partagé avec son hébergeur, les sections qui y copient, les autres membres avec leurs hôtes, et une commande `restic snapshots` avec l'hôte de ce serveur. Le kit de l'hébergeur nomme aussi le port, le dossier et l'identifiant de son rest-server.

Dans l'assistant d'urgence, le dépôt apparaît parmi les endroits comme **Dépôt partagé sur** suivi du nom de l'hébergeur. Une nouvelle installation qui reprend la clé d'un membre écrit sous le même hôte, car l'hôte vient de la clé. Elle retrouve sa destination dans le dépôt partagé et n'y liste et n'y restaure que ses propres sauvegardes.

Une nouvelle installation qui reprend la clé de l'hébergeur héberge le dépôt à sa place. Elle le fait vieillir et y fait le ménage comme avant, et redémarre le rest-server sur le même dossier. Appairez-la de nouveau avec le groupe, configurez son serveur récepteur sous **Instances**, et redéfinissez les règles dans la fenêtre du serveur récepteur, car elles ne sont pas dans le kit. Dès que le groupe joint les membres, le nouvel hébergeur les prévient lui-même. Chaque membre vérifie d'abord : il ajoute au dépôt une clé d'essai que seul un serveur ayant accès au dossier du dépôt peut retirer, et suit le nouvel hébergeur une fois cette clé disparue. Si le nouvel hébergeur répond à une autre adresse, chaque membre y refait cette vérification avec sa propre clé et son propre identifiant, s'assure qu'il retrouve le même dépôt, et ce n'est qu'ensuite qu'il bascule ses destinations vers la nouvelle adresse. D'ici là, il continue de copier vers l'ancienne adresse, et sa carte dit pourquoi. L'hébergeur retire à son ménage suivant les clés restées de ces vérifications.

Si l'ancien hébergeur tourne encore, sa carte dit que l'autre serveur héberge désormais le dépôt. **Reprendre** le refait hébergeur, pour le cas où un kit a été repris par erreur sur un second serveur.

### Limites {#shared-limits}

- **Chaque clé lit tout.** Toutes les clés d'un dépôt restic ouvrent la même clé maîtresse, si bien que chaque membre peut lire les sauvegardes des autres. Les définitions de conteneurs et de VMs restent scellées avec la propre clé de chaque serveur, si bien que les conteneurs et VMs d'un autre serveur ne reviennent que comme fichiers. Un serveur qui a eu accès un jour a pu lire la clé maîtresse, le retirer ne l'écarte donc pas pour de bon. Seul un nouveau dépôt le fait.
- **L'hébergeur peut supprimer.** Aucun membre ne peut y supprimer quoi que ce soit, mais l'hébergeur accède directement au dossier et oublie et élague les sauvegardes de chaque membre. Une erreur ou une prise de contrôle sur l'hébergeur les touche tous.
- **Un dommage touche chaque serveur.** Un pack cassé manque à chaque serveur dont les sauvegardes l'utilisent, un dépôt partagé ne remplace donc pas une seconde copie.
- **Seulement sur un serveur récepteur.** S3, SFTP et les autres endroits ne peuvent pas limiter un membre à l'ajout, BombVault ne propose donc un dépôt partagé que sur un serveur récepteur.
- **Seulement comme cible de copie.** Le dossier propre d'une section ne peut pas se trouver dans un dépôt partagé, car chaque élagage bloquerait les sauvegardes de tous les serveurs.
- **L'historique plus ancien reste en place.** Seules les sauvegardes sous le propre hôte de chaque serveur y entrent, et rien n'est déplacé depuis d'autres dépôts.

## Exemple : appairer deux serveurs Unraid {#worked-example-two-unraid-boxes-end-to-end}

Voici une configuration complète des éléments ci-dessus, car ils sont plus faciles à assembler une fois qu'on les a vus assemblés.

Deux machines : **TOWER** fait tourner les conteneurs et envoie les sauvegardes. **VAULT** les reçoit et impose l'ajout seul. Remplacez par vos propres noms, adresses et chemins de partages.

**1. Appairer les deux machines.** Sur TOWER, ouvrez **Paramètres, Appairage**, appuyez sur **Générer une phrase**, et saisissez les douze mots sur VAULT sous **Saisir une phrase**. Définissez d'abord un mot de passe de connexion sur les deux.

**2. Sur TOWER, demander à VAULT d'être le récepteur.** Sur la page Instances, activez **Récepteur** sur la carte de VAULT, choisissez les sections et **le serveur récepteur de VAULT** comme endroit, et envoyez la demande.

**3. Sur VAULT, l'autoriser.** La page Instances de VAULT montre la demande sur la carte de TOWER. Appuyez sur **Autoriser**. La première fois, VAULT demande un dossier pour son serveur récepteur, par exemple `user/appdata/rest-server`, démarre le rest-server en mode ajout seul et exécute le test d'altération contre lui.

**4. De retour sur TOWER.** À partir de là, TOWER copie vers VAULT après chaque sauvegarde, et la nouvelle destination apparaît sur la page Destinations de TOWER avec sa protection contre la suppression.

Ce que répond le test d'altération :

| Résultat | Ce qui s'est passé |
| --- | --- |
| **Protégé** | VAULT a refusé la suppression. C'est le seul état qui passe. |
| **Non protégé** | VAULT a accepté une suppression. `--append-only` manque ou a été retiré. |
| **Non concluant** | Aucun des deux. En général l'URL n'est pas celle que restic utilise lui-même, ou les identifiants ont changé. Rien n'est enregistré et aucune alerte ne part. |

**5. Surveiller ce qui arrive, si vous le voulez.** Activez **Récepteur** sous **Paramètres, Instances** sur VAULT, ouvrez les **Détails** de TOWER et ajoutez le dépôt reçu : cliquez sur l'un des emplacements que liste la fenêtre. VAULT reçoit le mot de passe restic de TOWER via le groupe, et comme l'emplacement se trouve sur le propre serveur de réception de VAULT, il lit le dossier directement. Personne ne saisit de clé ni d'identifiant de connexion.

**6. Le rendre réciproque.** Répétez les étapes dans l'autre sens : VAULT demande à TOWER d'être son récepteur. Chaque machine impose alors l'ajout seul pour l'autre, et aucune ne peut supprimer les sauvegardes de l'autre.

Un rest-server que vous lancez à la main fonctionne aussi : ajoutez-le avec **Ajouter une destination** comme rest-server, avec l'utilisateur et le mot de passe de son `.htpasswd`, et activez la protection contre la suppression. Gardez `--append-only` dans ses options. Sans lui, l'autre côté redevient un partage ordinaire.

!!! warning "Un chemin local est un chemin dans le conteneur"
    Là où BombVault demande un dossier sur ce serveur, saisissez-le relativement au montage de l'hôte, par exemple `user/appdata/rest-server/bombvault-containers`, pas `/mnt/user/appdata/...`. BombVault tourne dans un conteneur, où le `/mnt` de l'hôte est monté ailleurs. Un chemin d'hôte absolu n'y existe pas. Si vous en collez un, BombVault vous dit le chemin relatif à utiliser à la place.

## Assistant d'urgence {#emergency}

L'**Assistant d'urgence** ramène un serveur qui a disparu ou qui a été reconfiguré. Il démarre sur la page **Restauration** sous **Urgence**, et depuis **Restaurer depuis une sauvegarde** sur la page d'accueil d'une nouvelle installation. Il se déroule en quatre étapes : **Accès**, **Réglages**, **Sélection** et **Restauration**.

1. **Accès.** Montrez à BombVault la clé des anciennes sauvegardes, de l'une de trois façons :
    - **Téléverser le kit de récupération** : le fichier que vous avez téléchargé pendant la configuration. Il contient la clé et chaque destination, et BombVault remplit le reste à partir de lui. Un kit scellé avec age demande l'identité pour laquelle il a été scellé. Elle est utilisée une fois et n'est pas stockée.
    - **Via le groupe** : une instance de votre groupe garde la clé en dépôt. Il vous faut les douze mots du groupe, votre code de déblocage et quelqu'un sur cette instance. Voir [Gardien](#key-keeper).
    - **Saisir la clé** : l'`APP_KEY` de l'ancien serveur, et l'endroit où se trouvent ses sauvegardes.

    BombVault essaie alors la clé sur chaque destination et dit pour chacune si elle est lisible, introuvable, verrouillée par une autre clé, injoignable ou s'il lui manque encore ses identifiants. Rien n'a encore été enregistré, et la clé reste en mémoire pendant un temps limité. **Reprendre la clé** fait écrire ses sauvegardes par ce serveur avec cette clé désormais. **Annuler la reprise** revient à sa propre clé jusqu'à ce que la première sauvegarde ait eu lieu. Un serveur qui a déjà ses propres sauvegardes refuse la reprise et renvoie vers [Depuis un ancien dossier de sauvegarde](#restore-from-another-bombvault-repo). **Enregistrer les identifiants** stocke les identifiants que porte le kit pour les destinations distantes, et demande confirmation avant de remplacer des identifiants que ce serveur a déjà.
2. **Réglages.** **Restaurer les réglages** restaure les propres réglages de BombVault depuis l'auto-sauvegarde la plus récente : destinations, identifiants, plannings et règles. BombVault redémarre pour les appliquer, via le socket Docker, si bien que la base de réglages en service n'est jamais écrasée sous un descripteur ouvert, et l'assistant continue ensuite tout seul. La connexion de ce serveur reste : vous continuez à vous connecter avec le mot de passe que vous utilisez maintenant. Les secrets que l'ancien serveur avait stockés et qui ne peuvent pas être repris sont listés, pour que vous puissiez les saisir de nouveau. La restauration met fin à toutes les sessions et supprime les clés d'accès, configurez-les donc de nouveau ensuite.
3. **Sélection.** BombVault parcourt vos sauvegardes et liste les conteneurs, VMs, ensembles de dossiers et jeux de données ZFS qu'il trouve. L'instantané le plus récent est présélectionné partout. Ce que ce serveur possède encore démarre désactivé, car une restauration le remplacerait. Les ensembles de dossiers ne savent plus d'où ils venaient après une reconstruction, ils vont donc dans un dossier que vous choisissez.
4. **Restauration.** Tout ce qui est sélectionné revient d'un coup, laissé arrêté si vous voulez démarrer vous-même chaque conteneur et chaque VM, ce qui est le plus sûr après une perte totale. Téléchargez ensuite un nouveau kit de récupération : l'ancien appartient à l'installation précédente.

Un dépôt partagé du kit apparaît parmi les endroits comme **Dépôt partagé sur** suivi du nom du serveur qui l'héberge, et l'assistant n'y liste que les propres sauvegardes de ce serveur. Voir [Après une urgence](#shared-emergency).

![L'assistant d'urgence à sa première étape, avec les trois façons d'entrer.](assets/screenshots/offsite/emergency-dark.png#only-dark)
![L'assistant d'urgence à sa première étape, avec les trois façons d'entrer.](assets/screenshots/offsite/emergency-light.png#only-light)

### Exemple : un nouveau serveur à partir du kit de récupération {#example-emergency}

Le serveur TOWER est mort avec ses disques. Il gardait 23 conteneurs et 2 VMs, et les copiait avec son auto-sauvegarde vers Backblaze B2. Vous avez `bombvault-recovery-kit.md` dans votre gestionnaire de mots de passe et une nouvelle machine avec Unraid dessus. Son ancienne `APP_KEY` a disparu, et vous n'en avez pas besoin.

1. **Installez BombVault** sur la nouvelle machine et donnez-lui une nouvelle `APP_KEY`, comme pour toute nouvelle installation. Configurez le lien des VMs sous **Paramètres, Intégrations, SSH de l'hôte** si vous voulez retrouver les VMs, car une restauration de VM en a besoin.
2. **Lancez l'assistant.** Sur **Bienvenue dans BombVault**, choisissez **Restaurer depuis une sauvegarde**. Sur la page Restauration, choisissez **Lancer l'assistant d'urgence** sous **Urgence**.
3. **Accès.** Choisissez **Téléverser le kit de récupération** et déposez le fichier. BombVault essaie sa clé sur chaque destination et dit, par exemple, **Kit de récupération lu. Sa clé ouvre 0 destination sur 2.** Le dossier local dit **introuvable**, puisque le disque est mort, et Backblaze B2 dit **identifiants manquants**.
4. **Enregistrer les identifiants.** Le kit porte les identifiants de chaque destination. Appuyez sur **Enregistrer les identifiants**, et les destinations sont essayées de nouveau, si bien que Backblaze B2 dit maintenant **lisible**. Un kit écrit avant la 10.5 ne contient que les identifiants par défaut. Là où une destination dit encore **identifiants manquants**, saisissez ses identifiants sous **Saisir un identifiant manquant** et appuyez sur **Réessayer les destinations**.
5. **Reprendre la clé.** Ce serveur écrit maintenant ses sauvegardes avec l'ancienne clé, si bien que les nouvelles sauvegardes s'accordent avec les anciennes. Il garde sa propre `APP_KEY` dans le modèle. **Annuler la reprise** fonctionne jusqu'à ce que la première sauvegarde ait eu lieu.
6. **Réglages.** L'étape s'ouvre sur l'endroit où la clé a trouvé l'auto-sauvegarde, ici B2, avec son adresse déjà remplie. Choisissez **Restaurer les réglages**, qui y lit l'auto-sauvegarde la plus récente. BombVault redémarre pour l'appliquer, et l'assistant continue tout seul. Vos destinations, plannings et règles sont de retour, et vous continuez à vous connecter avec le mot de passe de ce serveur. Saisissez de nouveau les secrets que liste l'assistant, comme les réglages de notification.
7. **Sélection.** **Où se trouvent les sauvegardes ?** s'ouvre sur les copies hors site quand il ne reste rien en local. BombVault lit chaque destination, si bien qu'une destination hors service ne cache pas les autres, liste les 23 conteneurs et les 2 VMs et restaure chacun depuis la destination qui contient son instantané le plus récent.
8. **Restauration.** Activez **Laisser arrêté ensuite**, ce qui est le plus sûr après une perte totale, et démarrez. **Ce qui se passe** dit ce qui revient. La ligne de résultat indique, par exemple, **25 restaurés, 0 échoué. Démarrez-les sous Sauvegardes quand il le faut.**
9. **Terminer.** Démarrez d'abord vos bases de données, puis les applications qui les utilisent. Ouvrez **Paramètres, Restauration** et téléchargez un nouveau kit de récupération, car l'ancien appartient à l'installation précédente. Vérifiez **À traiter** sur l'aperçu, et lancez **Tout sauvegarder maintenant** une fois que tout fonctionne.

Si l'auto-sauvegarde manque aussi, l'étape s'ouvre sur **Passer : je n'ai pas d'auto-sauvegarde** et le dit. Les destinations viennent alors du kit seul, vous reconfigurez les plannings et les règles, et les copies hors site attendent que vous adoptiez leurs sauvegardes pour ce serveur, voir [Après une reconstruction](#after-a-rebuild). **Sélection** liste quand même ce que contiennent les copies hors site. Les ensembles de dossiers vont dans un dossier de votre choix. Les conteneurs et les VMs reviennent comme simples fichiers dans un dossier sous **Seuls les fichiers peuvent revenir**, car leurs définitions vivent à côté du dépôt local et dans l'auto-sauvegarde, et une copie hors site ne contient ni l'un ni l'autre.

!!! tip "Migration planifiée ou sinistre"
    L'assistant d'urgence restaure les propres réglages de BombVault depuis une sauvegarde. Pour un déménagement *planifié* vers une nouvelle machine, vous pouvez à la place reporter directement votre configuration avec la carte **Exporter / importer les paramètres** (un fichier JSON portable). Voir [Configuration](configuration.md#portable-settings-export-and-import).

### Depuis un ancien dossier de sauvegarde {#restore-from-another-bombvault-repo}

**Depuis un ancien dossier de sauvegarde**, sous **Autres sources** sur la page Restauration, ouvre le dépôt d'une instance BombVault *différente* (un partage monté sous `/mnt`, ou une adresse distante) avec le kit de récupération de cette instance ou son `APP_KEY` saisie, dans une session unique en lecture seule. Parcourez les conteneurs, VMs, ensembles de dossiers et jeux de données ZFS qui y sont stockés, choisissez un instantané et restaurez-le, et l'objet restauré devient un conteneur, une VM ou un ensemble de dossiers local normal. Rien n'est jamais écrit dans l'autre dépôt, et vos propres réglages de sauvegarde restent intacts (la session vit en mémoire et expire toute seule). Déplacer un conteneur du serveur A vers le serveur B ne veut pas dire repointer vos réglages puis les rétablir ensuite. Cette fenêtre est à usage unique : elle ouvre une session, restaure ce que vous choisissez, et oublie l'autre instance. Si vous voulez plutôt un arrangement durable, où cette machine rapatrie selon un planning les instantanés d'une autre instance dans son propre dépôt, c'est le rôle **Rapatrieur** sur la page Instances.

### Ce que ce serveur a rapatrié {#restore-what-was-fetched}

Chaque source depuis laquelle ce serveur a rapatrié a sa propre ligne **Récupéré depuis** sous **Autres sources** sur la page Restauration. Une copie rapatriée est verrouillée avec la propre clé de ce serveur, pas avec celle de la source, donc rien n'est demandé : la clé de la source ne l'ouvre pas. Les ensembles de dossiers et les jeux de données ZFS vont dans un dossier de votre choix. Les conteneurs et les VMs reviennent comme simples fichiers dans un dossier, car un rapatriement transporte les instantanés et pas les définitions. Pour les recréer, utilisez le kit de récupération et l'auto-sauvegarde de la source dans l'assistant d'urgence.

Un conteneur dont le réseau n'existe pas sur ce serveur, comme un réseau `br0` d'Unraid sur un hôte Docker ordinaire, montre un sélecteur de réseau sous sa ligne. BombVault le crée sur le réseau que vous choisissez, avec ses autres réseaux. L'IP fixe et l'adresse MAC appartenaient à l'ancien réseau et sont abandonnées, c'est donc le nouveau réseau qui les attribue.

## Gardien {#key-keeper}

Un gardien est une instance de votre groupe qui détient une copie scellée du kit de récupération de ce serveur. Il ne peut pas ouvrir la copie lui-même. Si ce serveur est un jour perdu, une nouvelle installation qui rejoint votre groupe demande la copie au gardien et l'ouvre avec votre code de déblocage.

![La fenêtre du code de déblocage : vingt caractères en quatre groupes, montrés une seule fois.](assets/screenshots/offsite/recovery-code-dark.png#only-dark)
![La fenêtre du code de déblocage : vingt caractères en quatre groupes, montrés une seule fois.](assets/screenshots/offsite/recovery-code-light.png#only-light)

1. Sur la page Instances, activez **Gardien** sur la carte de l'instance qui doit le garder, et choisissez **Déposer la copie**. Déposer une copie demande un mot de passe de connexion.
2. Ce serveur montre le **Code de déblocage** une seule fois : vingt lettres et chiffres en quatre groupes de cinq. Notez-le et confirmez avec **J'ai noté le code**. Ce serveur ne stocke pas le code, seulement ce dont il a besoin pour sceller un dépôt pour lui.
3. Le gardien détient maintenant la copie déposée, et ce serveur la renouvelle quand le kit change. **Copie déposée**, sur la carte de cette instance, liste **Copie déposée de ce serveur** avec chaque gardien et son état, et **Copies déposées que ce serveur garde** pour les autres. **Créer un nouveau code** remplace le code. Chaque gardien passe au nouveau quand il reçoit la nouvelle copie.

Ce jour-là, il vous faut trois choses : les douze mots de votre groupe, le code de déblocage, et l'accès à l'interface du gardien, où quelqu'un doit appuyer sur **Autoriser**. Dans l'assistant d'urgence, choisissez **Via le groupe**, choisissez la copie déposée, demandez-la et saisissez le code de déblocage. Le code est utilisé sur le nouveau serveur et n'est pas envoyé au gardien. Sur le gardien, la demande apparaît sous **Copie déposée** avec qui demande, si cette instance a déjà été vue là-bas auparavant et combien de fois elle a demandé. **Autoriser** remet la copie scellée, et autoriser demande un mot de passe de connexion. La nouvelle installation a 30 minutes pour la récupérer après l'autorisation, une demande à laquelle personne ne répond expire au bout d'un jour, et une instance qui demande trop souvent doit attendre une heure.

Sans le code, personne ne peut ouvrir la copie, le gardien compris. Si le code est perdu, créez-en un nouveau tant que ce serveur tourne. Si le serveur a déjà disparu, le kit de récupération et l'APP_KEY saisie fonctionnent toujours. Le gardien peut supprimer une copie déposée à tout moment, et le propriétaire doit alors redemander.

## Kit de récupération {#recovery-kit}

C'est l'élément qui rend la reprise après sinistre possible même quand aucun BombVault ne tourne.

**Télécharger le kit de récupération**, dans la carte **En cas d'urgence** sous **Paramètres, Restauration**, vous donne la **clé**, le **mot de passe restic dérivé**, chaque **destination** avec le dépôt de chaque section, les identifiants dont une nouvelle installation a besoin pour les atteindre, et les **commandes restic exactes**, pour que vous puissiez restaurer directement avec la CLI restic sur n'importe quelle machine. Il demande un mot de passe de connexion. Le kit imprime la clé aussi avec le chiffrement désactivé, car les définitions de conteneurs et de VMs sont toujours scellées avec elle. Après une reprise, il imprime la clé de sauvegarde reprise et l'APP_KEY de ce serveur, dont une nouvelle installation n'a besoin que pour ramener les réglages. Une carte **Sauvegardez votre kit de récupération** sur l'aperçu vous le rappelle jusqu'à ce que vous l'ayez rangé.

!!! danger "Rangez le kit de récupération hors du serveur"
    Le kit contient le secret qui déchiffre vos sauvegardes. Gardez-le en lieu sûr, séparément du serveur (un gestionnaire de mots de passe, une copie imprimée dans un coffre). Si vous perdez à la fois BombVault et `APP_KEY` sans kit de récupération ni copie déposée de la clé, vos sauvegardes chiffrées ne peuvent pas être récupérées.

!!! warning "L'instantané le plus récent n'est pas toujours celui à restaurer"
    Depuis restic 0.17, `restic snapshots` montre la taille de chaque instantané. Après une perte de données, le plus récent peut être celui qui a été vidé, ne restaurez donc pas un instantané bien plus petit que ceux d'avant. Après un rançongiciel, ce peut être celui qui est chiffré, à la taille habituelle. Si BombVault tourne encore, regardez d'abord sa page **Anomalies** : elle nomme la dernière bonne sauvegarde, et la fenêtre de restauration la présélectionne. Une restauration n'a besoin d'aucune donnée d'anomalie de BombVault, et la pause de rétention ne fait que garder plus d'instantanés.

### L'hôte des sauvegardes d'un serveur {#backup-host}

Chaque serveur écrit ses sauvegardes sous son propre hôte restic : `bv-` suivi de douze caractères calculés à partir de la clé des sauvegardes, par exemple `bv-3f9a0c41d2e7`. Les sauvegardes faites avant qu'un serveur ait le sien portent l'hôte `bombvault`. Le kit indique l'hôte de ce serveur, et ses commandes passent les deux avec `--host`, pour que `latest` ne choisisse jamais une sauvegarde qu'un autre serveur a écrite dans le même dépôt. Un serveur qui reprend les sauvegardes d'un autre continue sous l'hôte de ce serveur, car l'hôte vient de la clé. Pour chaque [dépôt partagé](#shared-repository) dans lequel le serveur copie ou qu'il héberge, le chapitre **Shared repositories** du kit nomme les autres serveurs qui s'y trouvent avec leurs hôtes.

### Sceller le kit {#sealing-the-kit}

Si vous avez activé **Chiffrer les exports avec age** sous **Paramètres, Système**, le kit est aussi scellé avec lui et se télécharge sous le nom `bombvault-recovery-kit.md.age`. Il est armé en ASCII plutôt que binaire, c'est donc encore du texte simple : le coller dans un gestionnaire de mots de passe ou l'imprimer fonctionne exactement comme avant, le contenu est illisible sans votre clé.

!!! warning "Ne rangez pas la clé age dans le kit"
    Il vous faut votre clé **privée** age pour ouvrir un kit scellé. Gardez-la quelque part qui ne dépend pas du kit lui-même, sinon vous aurez deux choses à récupérer au lieu d'une. Sceller vaut la peine quand le kit est rangé quelque part que vous ne contrôlez pas entièrement (un gestionnaire de mots de passe partagé, des notes dans le cloud, une impression dans un bureau). Un kit dans votre propre coffre est déjà protégé par le coffre.

    Avec le chiffrement activé et aucun destinataire utilisable configuré, le téléchargement est purement et simplement refusé. BombVault ne se rabat jamais sur la remise de la clé maîtresse en clair.

### Si vous n'avez pas le kit sous la main {#if-you-do-not-have-the-kit-to-hand}

Le mot de passe n'est stocké nulle part, il est **calculé** à partir de la clé des sauvegardes, vous pouvez donc le reproduire vous-même avec seulement la clé et un shell :

```sh
printf 'bombvault:restic-repo'   | openssl dgst -sha256 -mac HMAC -macopt hexkey:$APP_KEY -r   | cut -d' ' -f1
```

C'est un HMAC-SHA256 de la chaîne fixe `bombvault:restic-repo`, avec pour clé les octets bruts de l'`APP_KEY` hexadécimale, affiché en 64 caractères hexadécimaux en minuscules. La même valeur figure dans le kit, listée comme mot de passe restic dérivé. C'est pour le jour où le kit est là où vous n'êtes pas. Sur un serveur qui a repris les sauvegardes d'un autre, utilisez la clé de sauvegarde reprise, qui est l'ancienne APP_KEY de ce serveur.

L'[hôte](#backup-host) vient de la même clé et de la chaîne `bombvault:host` : `bv-` suivi des douze premiers de ces caractères.

```sh
echo "bv-$(printf 'bombvault:host' | openssl dgst -sha256 -mac HMAC -macopt hexkey:$APP_KEY -r | cut -c1-12)"
```

!!! warning "Pour un dépôt reçu, utilisez la clé de l'instance ÉMETTRICE"
    Un dépôt arrivé ici par une copie hors site a été créé par la machine qui l'a envoyé, avec **sa** clé. Dériver à partir de la clé de la machine réceptrice produit un mot de passe que restic rejettera, ce qui ressemble exactement à un dépôt corrompu et n'en est pas un. C'est la raison habituelle pour laquelle `restic check` sur un dépôt reçu demande un mot de passe encore et encore.

Comme les définitions de récupération vivent **dans** chaque dépôt (`<repo>/def`, `<repo>/vm-def`), un dossier de dépôt copié est entièrement autonome, si bien que le kit plus le dépôt est tout ce dont une restauration à partir de zéro a besoin. Une copie hors site et une copie rapatriée contiennent les instantanés sans les définitions : elles rendent des fichiers, et un conteneur ou une VM est recréé à partir de l'auto-sauvegarde.

## Récupérer un dump de base de données {#database-dumps}

Un dump de base de données est un point de restauration à part dans le dépôt des conteneurs, étiqueté `dbdump:<container>`, contenant le fichier unique `/dbdump/<container>.sql`. BombVault les liste, les télécharge et les importe sur l'onglet **Sauvegardes** du conteneur. Voici les mêmes étapes avec restic seul, pour le jour où BombVault n'est pas là.

```sh
restic -r <repo> snapshots --host <host> --host bombvault --tag dbdump:<container>
restic -r <repo> dump --host <host> --host bombvault --tag dbdump:<container> latest /dbdump/<container>.sql > <container>.sql
```

`<host>` est l'[hôte](#backup-host) du serveur indiqué dans le kit de récupération.

Les étiquettes `dbversion:` et `dbname:` de chaque dump disent de quelle version de serveur il vient et quelles bases il contient. Un fichier complet se termine par `-- PostgreSQL database cluster dump complete` ou `-- Dump completed`.

Importez-le dans un conteneur de la même version ou d'une version plus récente (PostgreSQL), ou de la même version majeure (MySQL et MariaDB), démarré une fois avec un dossier de données vide pour qu'il s'initialise. L'hôte n'a besoin d'aucun client de base de données, le conteneur en a un :

```sh
docker exec -i <container> sh -c 'exec psql -X -U "${POSTGRES_USER:-postgres}" -d postgres' < <container>.sql
docker exec -i <container> sh -c 'exec mariadb -uroot -p"$MARIADB_ROOT_PASSWORD"' < <container>.sql
docker exec -i <container> sh -c 'exec mysql -uroot -p"$MYSQL_ROOT_PASSWORD"' < <container>.sql
```

Pour une seule base issue d'un dump complet, MySQL et MariaDB acceptent `--one-database <name>` sur la commande du client. Un dump PostgreSQL a une section par base, chacune commençant par une ligne `\connect <name>` : copiez cette section dans son propre fichier et importez-la avec `-d <name>` après avoir créé la base.

!!! warning "Un dump root contient les utilisateurs du serveur"
    Un dump complet MySQL ou MariaDB pris en root contient la base système `mysql`, si bien que l'importer remplace les comptes du nouveau serveur, y compris le mot de passe de root, par ceux du dump. Sur PostgreSQL, `role ... already exists` pour l'utilisateur que le conteneur a créé est normal et sans danger.
