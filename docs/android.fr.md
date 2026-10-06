# Application Android

L'application Android met tous les serveurs BombVault de votre groupe sur votre téléphone. Elle s'ouvre sur la liste de vos serveurs, avec en haut le journal d'activité de tous, les mêmes lignes que celles du tableau de bord, et ouvre la vue mobile du serveur que vous touchez. L'application ne sauvegarde rien elle-même.

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

!!! note "L'interface a toujours besoin d'un chemin vers le serveur"
    La liste des serveurs et le journal d'activité passent par le groupe. L'interface d'un serveur s'ouvre directement : le téléphone doit donc pouvoir joindre l'adresse du serveur, à la maison ou via un VPN.

## Connecté d'office sur un téléphone appairé {#sign-in}

Un téléphone appairé à votre groupe ouvre chacun de ses serveurs avec la session déjà ouverte. Avant de charger une page, il demande une session à ce serveur via le groupe, et un serveur n'en accorde qu'à un membre qui est un téléphone. Un serveur ajouté par son adresse demande le mot de passe, comme dans un navigateur. Quiconque possède les douze mots peut déjà ouvrir toutes les sauvegardes du groupe : l'appairage n'accorde donc rien de nouveau.

## Serveurs hors d'un groupe {#other-servers}

- **Ajouter un serveur** prend l'adresse avec laquelle vous ouvrez BombVault dans un navigateur, par exemple `192.168.1.10:3443`. Sans `http://` ou `https://` devant, l'application utilise https.
- Les serveurs qui s'annoncent sur le réseau local apparaissent sous **Sur ce réseau** et s'ouvrent d'un toucher. Ils le font tant que **Trouver sur le réseau** est activé sous Paramètres, Intégrations. Un serveur sur un autre réseau ou derrière un VPN n'y apparaît pas.
- Un certificat auto-signé est approuvé une fois, par son empreinte SHA-256. Si le serveur en présente plus tard un autre, l'application vous avertit et ne l'ouvre qu'après que vous avez approuvé le nouveau certificat.

## Le téléphone sur la page Instances {#instances}

Le téléphone obtient sa propre carte sur la page Instances de chaque serveur du groupe, marquée comme application Android et portant le nom que vous avez donné au téléphone. Elle n'a pas de fiche de protection, puisque le téléphone ne sauvegarde rien, et **Supprimer** la retire de la page.

## Paramètres {#settings}

La roue dentée à côté du bouton + ouvre les paramètres de l'application :

- la langue, et le nom que le téléphone affiche sur la page Instances (vide, c'est le modèle du téléphone),
- l'apparence, qui suit le premier serveur de la liste jusqu'à ce que vous définissiez la vôtre, et les animations, qui ont leur propre réglage,
- un rapport à copier quand vous signalez un problème ; il ne contient ni adresse, ni nom, ni phrase,
- la carte **À propos** avec la politique de confidentialité,
- **Supprimer tous les serveurs**, qui retire tous les serveurs de l'application et quitte le groupe. Rien ne change sur les serveurs eux-mêmes.

## Téléchargements et envois {#files}

Les exports, les kits de récupération, les ZIP de la flash et les dumps de base de données arrivent dans le dossier Téléchargements du téléphone, comme depuis un navigateur. Un import de paramètres ouvre le sélecteur de fichiers du téléphone.

## Sur un écran tactile {#touch}

Rien ne se survole avec un doigt : un contrôle s'assombrit donc tant qu'il est maintenu, et un bouton portant le logo d'une marque s'allume dans la couleur de cette marque jusqu'à ce que le doigt se lève. Un appui long sur un bouton compte comme un toucher lent et n'ouvre aucun menu de lien.

## Ou dans un navigateur {#browser}

Chrome et Edge peuvent installer l'interface web de BombVault comme une application dans sa propre fenêtre, sur un téléphone comme sur un ordinateur. Rien n'est mis en cache, une mise à jour apparaît donc immédiatement.

## Confidentialité {#privacy}

L'application n'a ni comptes, ni publicité, ni statistiques d'utilisation, et n'exécute rien en arrière-plan. Sa [politique de confidentialité](https://github.com/junkerderprovinz/bombvault/blob/main/android/PRIVACY.md) indique ce qu'elle stocke et ce qu'elle envoie, et à qui.
