# Application Android

L'application Android met tous les serveurs BombVault de votre groupe sur votre téléphone. Elle s'ouvre sur la liste de vos serveurs, avec en haut le journal d'activité de tous, les mêmes lignes que celles de l'aperçu, et ouvre la vue mobile du serveur que vous touchez. L'application ne sauvegarde rien elle-même.

## Obtenir l'application {#install}

- **Paramètres, Applications :** la carte de l'application Android propose l'APK de la version que votre serveur exécute, avec un code QR à scanner depuis le téléphone.
- **APK :** chaque version publie `bombvault-android.apk` sur sa page de release, et [ce lien](https://github.com/junkerderprovinz/bombvault/releases/latest/download/bombvault-android.apk) télécharge toujours la dernière version. Elle nécessite Android 10 ou ultérieur, et Android demande une fois si l'application avec laquelle vous ouvrez le fichier a le droit d'installer des applications.
- **Google Play :** l'application est en test fermé jusqu'à ce qu'elle puisse être publiée. Google Play ne publie l'application d'un nouveau compte développeur qu'après qu'au moins 12 testeurs l'ont gardée installée pendant 14 jours. Pour aider, rejoignez le [groupe de testeurs](https://groups.google.com/g/arrowloop-testers), ouvrez la [page de test](https://play.google.com/apps/testing/bombvault.halleluja.design), touchez **Devenir testeur** et installez BombVault depuis Google Play.
- **F-Droid :** la fiche suivra.

Appairez l'application avec des serveurs en version 9.7.0 ou ultérieure. Un serveur d'une version plus ancienne peut faire partie du même groupe, mais il affiche le téléphone comme une simple instance, et l'application ne lit l'activité de ce serveur qu'après une connexion.

## Appairage par code QR {#pairing}

1. Sur n'importe quel serveur de votre groupe, ouvrez **Paramètres, Appairage** et choisissez **Afficher la phrase**. Les douze mots apparaissent avec un code QR à côté.
2. Dans l'application, touchez **Scanner un code QR** et pointez le téléphone vers le code. Vous pouvez aussi coller ou saisir les mots.
3. L'application liste les serveurs de ce groupe avant d'enregistrer quoi que ce soit. **Ajouter les N** les ajoute tous.

Le téléphone rejoint alors le groupe comme une instance de plus. Il lit ce qui tourne sur chaque serveur via le groupe, sans connexion, directement à la maison et via le relais à l'extérieur. Le fonctionnement du groupe lui-même est décrit sous [Appairage des instances](offsite-recovery.md#pairing).

L'application rejoint le groupe via le relais, même à la maison. Si votre instance est sur **Aucun relais**, choisissez-en un sous **Paramètres, Appairage** avant de scanner le code.

## Hors de chez soi {#away}

L'application essaie d'abord un serveur à sa propre adresse. Quand celle-ci ne répond pas, par exemple sur le réseau mobile, elle ouvre l'interface du serveur via le relais du groupe. Un serveur du groupe dont l'application ne connaît pas encore l'adresse s'ouvre de la même façon. Si ni l'adresse ni le relais ne répondent, l'écran d'accueil le signale sur la carte du serveur.

Via le relais, la page atteint ce qu'un téléphone appairé atteint à la maison, comme décrit plus bas. La connexion avec le mot de passe et l'enregistrement de fichiers ne fonctionnent qu'à l'adresse propre du serveur. Les barres de progression se mettent à jour toutes les deux secondes au lieu de façon continue. Les fichiers de l'interface restent sur le téléphone après la première visite, donc les visites suivantes se chargent plus vite. Un serveur antérieur à 10.0.0 ne s'ouvre qu'à sa propre adresse.

L'application n'accepte une réponse que si le serveur l'a signée avec la clé que l'application a épinglée lors de son premier contact avec ce serveur. Si le serveur répond plus tard avec une autre clé, l'écran d'accueil affiche l'empreinte de la nouvelle clé, et la page ne s'ouvre qu'après que vous l'avez approuvée.

## Changer de serveur {#switch}

Quand un serveur est ouvert, le haut de son aperçu affiche le nom du serveur. Dès que l'application connaît d'autres serveurs, une flèche se trouve à côté du nom, et un toucher liste tous les serveurs que l'application connaît, celui qui est ouvert marqué **Cette instance** et chacun des autres avec son état : **Connectée**, **Non connectée**, **Connexion nécessaire** ou **Vérifier le certificat**. Touchez-en un autre pour l'ouvrir, directement à la maison et via le relais sinon.

## Connecté d'office sur un téléphone appairé {#sign-in}

Un téléphone appairé à votre groupe ouvre chacun de ses serveurs avec la session déjà ouverte. Avant de charger une page, il demande une session à ce serveur via le groupe, et un serveur n'en accorde qu'à un membre qui est un téléphone. Un serveur ajouté par son adresse demande le mot de passe, comme dans un navigateur.

Un téléphone appairé n'obtient pas tout ce qu'un mot de passe donne. Sa session atteint une liste fixe de ce que montre l'application : l'aperçu, les exécutions et le journal d'activité, les sauvegardes à consulter, à lancer et à restaurer, les anomalies à lire et la page Instances à consulter. Les destinations et les commandes lui parviennent sans leurs adresses ni leurs secrets. Une restauration qui remplace l'original sauvegarde d'abord l'état actuel. Les clés, le kit de récupération, les copies déposées, les paramètres et la suppression de sauvegardes restent liés à une connexion sur le serveur lui-même, et tout le reste répond qu'il faut cette connexion. Via le relais, la même liste s'applique, aussi sur un serveur sans mot de passe.

Le réglage correspondant est **Les téléphones peuvent s'appairer avec les douze mots du groupe**, sur la carte **App Android** sous **Paramètres, Sécurité**. Une réinitialisation du mot de passe le désactive et déconnecte tous les téléphones, jusqu'à ce que vous le réactiviez là.

## Serveurs hors d'un groupe {#other-servers}

- **Ajouter un serveur** prend l'adresse avec laquelle vous ouvrez BombVault dans un navigateur, par exemple `192.168.1.10:3443`. Sans `http://` ou `https://` devant, l'application utilise https.
- Les serveurs qui s'annoncent sur le réseau local apparaissent sous **Sur ce réseau** et s'ouvrent d'un toucher. Ils le font tant que **Trouver sur le réseau** est activé dans la carte **Services** sous **Paramètres, Intégrations**. Un serveur sur un autre réseau ou derrière un VPN n'y apparaît pas.
- Un certificat auto-signé est approuvé une fois, par son empreinte SHA-256. Si le serveur en présente plus tard un autre, l'application vous avertit et ne l'ouvre qu'après que vous avez approuvé le nouveau certificat.

## Le téléphone sur la page Instances {#instances}

Le téléphone obtient sa propre carte sur la page Instances de chaque serveur du groupe, marquée comme application Android et portant le nom que vous avez donné au téléphone. Elle n'a pas de fiche de protection, puisque le téléphone ne sauvegarde rien, et **Supprimer** la retire de la page. Un téléphone qui connaît les mots peut s'appairer de nouveau à tout moment. Pour un téléphone perdu, désactivez donc **Les téléphones peuvent s'appairer avec les douze mots du groupe** sous **Paramètres, Sécurité**, ou faites **Quitter le groupe** sur chaque instance et appairez-les de nouveau avec de nouveaux mots.

Un téléphone appairé avant la version 10.0.0 commence à signer ses appels lors de sa première visite après la mise à jour, et chaque serveur garde la première clé qu'il reçoit pour ce téléphone. Si quelqu'un d'autre qui connaît les mots du groupe arrive le premier pendant que le téléphone est hors ligne, ce serveur refuse alors le téléphone, et l'application ne peut plus l'ouvrir via le relais. Pour y remédier, connectez-vous à ce serveur dans un navigateur, choisissez **Oublier la clé** sur la carte du téléphone dans la page Instances, puis **Confirmer l'oubli**, et rouvrez l'application. Le serveur prend la clé propre du téléphone à son prochain appel. Si le téléphone est de nouveau refusé, quelqu'un d'autre a les mots : faites **Quitter le groupe** sur chaque instance et appairez-les de nouveau avec de nouveaux mots, comme pour un téléphone perdu.

## Paramètres {#settings}

La roue dentée à côté du bouton + ouvre les paramètres de l'application :

- la langue, et le nom que le téléphone affiche sur la page Instances (vide, c'est le modèle du téléphone),
- l'apparence, qui suit le premier serveur de la liste jusqu'à ce que vous définissiez la vôtre, et les animations, qui ont leur propre réglage,
- un rapport à copier quand vous signalez un problème ; il ne contient ni adresse, ni nom, ni phrase,
- la carte **À propos** avec la politique de confidentialité,
- **Supprimer tous les serveurs**, qui retire tous les serveurs de l'application et quitte le groupe. Rien ne change sur les serveurs eux-mêmes.

## Téléchargements et envois {#files}

Les exports, les kits de récupération, les ZIP de la flash et les dumps de base de données arrivent dans le dossier Téléchargements du téléphone, comme depuis un navigateur. Un import de paramètres ouvre le sélecteur de fichiers du téléphone. Via le relais, aucun fichier n'est enregistré ; ouvrez pour cela le serveur à sa propre adresse.

## Sur un écran tactile {#touch}

Rien ne se survole avec un doigt : un contrôle s'assombrit donc tant qu'il est maintenu, et un bouton portant le logo d'une marque s'allume dans la couleur de cette marque jusqu'à ce que le doigt se lève. Un appui long sur un bouton compte comme un toucher lent et n'ouvre aucun menu de lien.

## Ou dans un navigateur {#browser}

Chrome et Edge peuvent installer l'interface web de BombVault comme une application dans sa propre fenêtre, sur un téléphone comme sur un ordinateur. Rien n'est mis en cache, une mise à jour apparaît donc immédiatement.

## Confidentialité {#privacy}

L'application n'a ni comptes, ni publicité, ni statistiques d'utilisation, et n'exécute rien en arrière-plan. Sa [politique de confidentialité](https://github.com/junkerderprovinz/bombvault/blob/main/android/PRIVACY.md) indique ce qu'elle stocke et ce qu'elle envoie, et à qui.
