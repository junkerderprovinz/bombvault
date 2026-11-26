# Application Android

L'application Android met tous les serveurs BombVault de votre groupe sur votre téléphone. Elle s'ouvre sur la liste de vos serveurs, avec en haut le journal d'activité de tous, les mêmes lignes que celles de l'aperçu, et ouvre la vue mobile du serveur que vous touchez. L'application ne sauvegarde rien elle-même.

![L'écran d'accueil de l'application : en haut le journal d'activité de tous les serveurs, puis la liste des serveurs.](assets/screenshots/android/start-dark.png#only-dark)
![L'écran d'accueil de l'application : en haut le journal d'activité de tous les serveurs, puis la liste des serveurs.](assets/screenshots/android/start-light.png#only-light)

## Obtenir l'application {#install}

- **Paramètres, Applications :** la carte de l'application Android propose l'APK de la version que votre serveur exécute, avec un code QR à scanner depuis le téléphone, et renvoie vers la fiche F-Droid.
- **APK :** chaque version publie `bombvault-android.apk` sur sa page de release, et [ce lien](https://github.com/junkerderprovinz/bombvault/releases/latest/download/bombvault-android.apk) télécharge toujours la dernière version. Elle nécessite Android 10 ou ultérieur, et Android demande une fois si l'application avec laquelle vous ouvrez le fichier a le droit d'installer des applications.
- **Google Play :** [BombVault sur Google Play](https://play.google.com/store/apps/details?id=bombvault.halleluja.design). Elle est signée avec la même clé que l'APK, l'une s'installe donc par-dessus l'autre.
- **F-Droid :** [BombVault sur F-Droid](https://f-droid.org/packages/bombvault.halleluja.design/). F-Droid compile lui-même l'application à partir du code source et la signe avec sa propre clé, une nouvelle version y arrive donc quelques jours plus tard. Pour passer de l'application F-Droid à l'APK ou inversement, désinstallez d'abord l'autre.

Appairez l'application avec des serveurs en version 9.7.0 ou ultérieure. Un serveur d'une version plus ancienne peut faire partie du même groupe, mais il affiche le téléphone comme une simple instance, et l'application ne lit l'activité de ce serveur qu'après une connexion.

## Appairage par code QR {#pairing}

1. Sur n'importe quel serveur de votre groupe, ouvrez **Paramètres, Appairage** et choisissez **Afficher la phrase**. Les douze mots apparaissent avec un code QR à côté.
2. Dans l'application, touchez **Scanner un code QR** et pointez le téléphone vers le code. Vous pouvez aussi coller ou saisir les mots.
3. L'application liste les serveurs de ce groupe avant d'enregistrer quoi que ce soit. **Ajouter les N** les ajoute tous.
4. Le téléphone attend ensuite dans la carte **En attente d'approbation** sous **Paramètres, Appairage** sur les serveurs du groupe. Appuyez sur **Approuver** sur l'un d'eux, et l'application affiche les serveurs comme **Connectée**. Les mots seuls ne font entrer aucun nouvel appareil, voir [Appairage des instances](offsite-recovery.md#pairing).

Le téléphone rejoint alors le groupe comme une instance de plus. Il lit ce qui tourne sur chaque serveur via le groupe, sans connexion, directement à la maison et via le relais à l'extérieur. Le fonctionnement du groupe lui-même est décrit sous [Appairage des instances](offsite-recovery.md#pairing).

L'application rejoint le groupe via le relais, même à la maison. Si votre instance est sur **Aucun relais**, choisissez-en un sous **Paramètres, Appairage** avant de scanner le code.

## Hors de chez soi {#away}

L'application essaie d'abord un serveur à sa propre adresse. Quand celle-ci ne répond pas, par exemple sur le réseau mobile, elle ouvre l'interface du serveur via le relais du groupe. Un serveur du groupe dont l'application ne connaît pas encore l'adresse s'ouvre de la même façon. Si ni l'adresse ni le relais ne répondent, l'écran d'accueil le signale sur la carte du serveur.

Via le relais, la page atteint ce qu'un téléphone appairé atteint à la maison, comme décrit plus bas. La connexion avec le mot de passe et l'enregistrement de fichiers ne fonctionnent qu'à l'adresse propre du serveur. Les barres de progression se mettent à jour toutes les deux secondes au lieu de façon continue. Les fichiers de l'interface restent sur le téléphone après la première visite, donc les visites suivantes se chargent plus vite. Un serveur antérieur à 10.0.0 ne s'ouvre qu'à sa propre adresse.

L'application n'accepte une réponse que si le serveur l'a signée avec la clé que l'application a épinglée lors de son premier contact avec ce serveur. Si le serveur répond plus tard avec une autre clé, l'écran d'accueil affiche l'empreinte de la nouvelle clé, et la page ne s'ouvre qu'après que vous l'avez approuvée.

## Changer de serveur {#switch}

Quand un serveur est ouvert, le haut de son aperçu affiche le nom du serveur, précédé d'une flèche de retour. La flèche mène à l'aperçu de l'application, l'écran d'accueil, qui liste tous les serveurs que l'application connaît avec leur état : **Connectée**, **Non connectée**, **Connexion nécessaire** ou **Vérifier le certificat**. Un serveur du groupe qui ne répond pas porte la mention **Inactive**, en gris, avec le moment où le groupe l'a vu pour la dernière fois, et un serveur dont l'application n'a encore rien reçu affiche **Vérification**. À chaque connexion, l'application lit la liste du groupe auprès de l'une de ses instances, si bien qu'une instance qu'elle n'a jamais rencontrée via le relais, par exemple une instance éteinte au moment où le téléphone a rejoint le groupe, figure aussi dans la liste. Touchez-y un autre serveur pour l'ouvrir, directement à la maison et via le relais sinon.

## Connecté d'office sur un téléphone appairé {#sign-in}

Un téléphone appairé à votre groupe ouvre chacun de ses serveurs avec la session déjà ouverte. Avant de charger une page, il demande une session à ce serveur via le groupe, et un serveur n'en accorde qu'à un membre qui est un téléphone.

Un téléphone appairé n'obtient pas tout ce qu'un mot de passe donne. Sa session atteint une liste fixe de ce que montre l'application : l'aperçu, les exécutions et le journal d'activité, les sauvegardes à consulter, à lancer, à vérifier et à restaurer, les anomalies, la recherche de mises à jour et la page Instances à consulter, sans les dépôts reçus ni les sources de rapatriement de ses membres. Sur les pages des sauvegardes et des éléments, il peut aussi ajouter un élément à la planification ou l'en retirer, le refuser, étiqueter une sauvegarde, retirer les instantanés restants d'une arborescence ZFS et chercher des éléments dans le stockage des sauvegardes. L'ajout d'un élément reste réservé à l'interface web, car le nouvel élément se choisit parmi les dossiers du serveur. Les destinations, les serveurs ZFS et les commandes lui parviennent sans leurs adresses, leurs identifiants ni leurs secrets.

Une restauration qui remplace l'original sauvegarde d'abord l'état actuel : un conteneur, une VM, un jeu de dossiers, une stack Compose entière avec son dossier de projet, un plugin de la flash (la flash est sauvegardée) et un dump de base de données importé dans la base en service (le conteneur est sauvegardé). Si cette sauvegarde échoue, rien n'est remplacé. Une restauration dans un dossier, comme un dump de base de données enregistré dans un dossier, va dans un nouveau dossier à l'intérieur du dossier de restauration défini sous **Paramètres, Restauration**, et la fenêtre n'en propose aucun autre.

Sous **Autres sources**, **Depuis un ancien dossier de sauvegarde** ouvre le dépôt d'une autre instance avec sa clé ou son kit de récupération. Un dépôt sur le réseau s'ouvre avec les identifiants que vous saisissez, qui ne servent que pour cette session et ne sont jamais renvoyés. Un dossier sur le serveur ne s'ouvre que depuis le dossier de restauration, et chaque élément revient sous forme de fichiers dans un nouveau dossier à cet endroit : un conteneur ou une VM n'est pas recréé depuis le téléphone. **Importer depuis le plugin Appdata.Backup** et **Tâches de Duplicati ou Duplicacy** demandent une connexion sur le serveur lui-même, car ils lisent des dossiers n'importe où sur le serveur ou créent des destinations et des identifiants.

Une anomalie acquittée ou marquée comme attendue sur le téléphone garde les anciennes sauvegardes qu'elle retient jusqu'à ce que quelqu'un se connecte avec le mot de passe et choisisse **Ne plus conserver les anciennes sauvegardes** parmi les constats fermés. Les clés, le kit de récupération, les copies déposées, les paramètres et la suppression de sauvegardes restent liés à une connexion sur le serveur lui-même, et tout le reste répond qu'il faut cette connexion. Via le relais, la même liste s'applique, aussi sur un serveur sans mot de passe.

Chaque téléphone que le groupe a laissé entrer se connecte tout seul, et aucun réglage ne le commande. Une réinitialisation du mot de passe déconnecte tous les téléphones, puis les téléphones se reconnectent tout seuls. Pour tenir un téléphone à l'écart, supprimez-le sur la page Instances.

## Certificats {#certificates}

Un certificat auto-signé est approuvé une fois, par son empreinte SHA-256. Si le serveur en présente plus tard un autre, l'application vous avertit et ne l'ouvre qu'après que vous avez approuvé le nouveau certificat.

## Le téléphone sur la page Instances {#instances}

Le téléphone obtient sa propre carte sur la page Instances de chaque serveur du groupe, marquée comme application Android et portant le nom que vous avez donné au téléphone. Elle n'a pas de fiche de protection, puisque le téléphone ne sauvegarde rien. **Supprimer** la retire de la page et met fin à sa session sur-le-champ, tout comme la désactivation de **Interroger cette instance** sous **Surveillance** dans ses détails. Le téléphone connaît toujours les mots, il réapparaît donc quand son application s'ouvre, mais désactivé, et il n'obtient aucune session tant que personne ne l'active. Pour un téléphone perdu, supprimez sa carte sur chaque serveur, ou faites **Quitter le groupe** sur chaque instance et appairez-les de nouveau avec de nouveaux mots.

Un téléphone appairé avant la version 10.0.0 commence à signer ses appels lors de sa première visite après la mise à jour, et chaque serveur garde la première clé qu'il reçoit pour ce téléphone. Si quelqu'un d'autre qui connaît les mots du groupe arrive le premier pendant que le téléphone est hors ligne, ce serveur refuse alors le téléphone, et l'application ne peut plus l'ouvrir via le relais. Pour y remédier, connectez-vous à ce serveur dans un navigateur, choisissez **Oublier la clé** sur la carte du téléphone dans la page Instances, puis **Confirmer l'oubli**, et rouvrez l'application. Le serveur prend la clé propre du téléphone à son prochain appel. Si le téléphone est de nouveau refusé, quelqu'un d'autre a les mots : faites **Quitter le groupe** sur chaque instance et appairez-les de nouveau avec de nouveaux mots, comme pour un téléphone perdu.

## Nœuds {#nodes}

Les [nœuds](nodes.md) se gèrent dans l'interface web. La session d'un téléphone appairé ne les atteint pas, l'application n'affiche donc aucune carte de nœud sur la page Instances, aucune carte **Nœuds** dans l'aperçu et aucune section de nœud sous **Sauvegardes**, et **Ajouter un nœud** répond qu'il faut une connexion sur le serveur lui-même. Ouvrez le serveur dans un navigateur et connectez-vous avec le mot de passe pour ajouter un nœud, changer la destination de ses sauvegardes ou restaurer l'un de ses éléments.

## Paramètres {#settings}

La roue dentée à côté du bouton + ouvre les paramètres de l'application :

- la langue, et le nom que le téléphone affiche sur la page Instances (vide, c'est le modèle du téléphone),
- l'apparence, qui suit le premier serveur de la liste jusqu'à ce que vous définissiez la vôtre, et les animations, qui ont leur propre réglage,
- la carte **Version**, qui nomme la boutique d'où viennent les mises à jour (Google Play, F-Droid ou GitHub Releases) et ouvre les notes de cette version sous **Nouveautés**, puis la carte **Composants**,
- un rapport à copier quand vous signalez un problème. Il ne contient ni adresse, ni nom, ni phrase,
- la carte **À propos** avec la politique de confidentialité,
- **Supprimer tous les serveurs**, qui retire tous les serveurs de l'application et quitte le groupe. Chaque serveur que le téléphone joint encore le retire de sa page Instances.

## Téléchargements et envois {#files}

La session d'un téléphone appairé enregistre un seul type de fichier : un dump de base de données se télécharge dans le dossier Téléchargements du téléphone quand l'application ouvre le serveur à sa propre adresse. Pour tout le reste, connectez-vous d'abord avec le mot de passe à l'adresse propre du serveur. Après cette connexion, les exports, les kits de récupération et les ZIP de la flash arrivent dans le dossier Téléchargements du téléphone, comme depuis un navigateur, et un import de paramètres ouvre le sélecteur de fichiers du téléphone. Via le relais, aucun fichier n'est enregistré.

## Sur un écran tactile {#touch}

Rien ne se survole avec un doigt, un contrôle s'assombrit donc tant qu'il est maintenu, et un bouton portant le logo d'une marque s'allume dans la couleur de cette marque jusqu'à ce que le doigt se lève. Un appui long sur un bouton compte comme un toucher lent et n'ouvre aucun menu de lien. Toucher un bouton grisé affiche la raison pour laquelle il est grisé, jusqu'à ce que vous touchiez ailleurs.

## Ou dans un navigateur {#browser}

Chrome et Edge peuvent installer l'interface web de BombVault comme une application dans sa propre fenêtre, sur un téléphone comme sur un ordinateur. Rien n'est mis en cache, une mise à jour apparaît donc immédiatement.

## Confidentialité {#privacy}

L'application n'a ni comptes, ni publicité, ni statistiques d'utilisation, et n'exécute rien en arrière-plan. Sa [politique de confidentialité](https://github.com/junkerderprovinz/bombvault/blob/main/android/PRIVACY.md) indique ce qu'elle stocke et ce qu'elle envoie, et à qui.
