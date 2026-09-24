# Dépannage

Une courte FAQ. Pour le tableau complet de dépannage côté hôte de la sauvegarde de VM via SSH (permission refusée, vérification de clé d'hôte, variables de modèle manquantes et plus), voir le [guide de sauvegarde de VM via SSH](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) sur GitHub.

## Quelque chose n'est pas correctement branché

Ouvrez `/spike` dans l'interface web. La vérification de l'intégration hôte sonde chaque montage et CLI (socket Docker, libvirt, restic, qemu-img, rclone) et signale toute pièce manquante. Commencez ici avant de supposer un bug : un montage manquant ou un hôte injoignable apparaît immédiatement.

## Je ne peux pas atteindre l'interface web

BombVault sert du HTTPS par défaut sur le port `3443` (certificat auto-signé), ouvrez donc `https://<votre-ip-unraid>:3443`. Acceptez l'avertissement de certificat auto-signé, ou placez BombVault derrière un reverse proxy avec votre propre certificat. Si vous l'exécutez avec `HTTP_ONLY=true`, il sert du HTTP simple sur le port `3000` à la place (destiné à un usage derrière un proxy terminant le TLS).

## J'ai perdu mon APP_KEY

`APP_KEY` dérive le mot de passe du dépôt restic. Sans lui (et sans le kit de récupération de clé de chiffrement), les sauvegardes chiffrées ne peuvent pas être récupérées. C'est pourquoi le tableau de bord vous relance pour télécharger le kit de récupération. Voir [Sauvegarde hors site et récupération](offsite-recovery.md). Générez une clé avec `openssl rand -hex 32` et conservez-la hors du serveur avant de vous fier à une quelconque sauvegarde.

## La sauvegarde de VM ne se connecte pas

La sauvegarde de VM dialogue avec libvirt via SSH, jamais un montage.

- Confirmez que SSH est activé sur l'hôte et que la clé publique de BombVault est autorisée dans `/root/.ssh/authorized_keys` (Paramètres, Système, Sauvegarde de VM via SSH affiche la clé et un bouton **Tester la connexion**).
- Sur un réseau `br0.x` personnalisé, réglez `LIBVIRT_HOST` sur l'IP LAN de votre Unraid (le conteneur ne peut pas y atteindre l'hôte via `host.docker.internal`). Activez **Paramètres, Docker, Accès de l'hôte aux réseaux personnalisés**.
- Si vous avez changé le port SSH d'Unraid, réglez `LIBVIRT_SSH_PORT` en conséquence.
- Le diagnostic complet pas à pas (test d'accessibilité, routage VLAN, `Permission denied (publickey)`, `Host key verification failed`) se trouve dans le [guide de sauvegarde de VM via SSH](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md).

## Un instantané de VM à chaud ne s'est pas exécuté

Les instantanés à chaud nécessitent l'agent invité qemu installé dans la VM et le disque sur `/mnt/cache` (ou `/mnt/diskX`), pas `/mnt/user`. Sur une VM éteinte, le mode à chaud se rabat automatiquement sur le mode propre. Une sauvegarde propre arrête la VM, sauvegarde les disques, puis la redémarre, elle est donc toujours cohérente.

## Une sauvegarde a échoué avec « repository is already locked »

C'est généralement un verrou restic orphelin laissé lorsque le conteneur a été mis à jour ou redémarré en pleine opération. BombVault détecte un verrou prouvé orphelin, le force à se libérer et réessaie une fois, automatiquement. S'il persiste, utilisez **Paramètres, Intégrité et maintenance, Déverrouiller** pour le domaine concerné afin de libérer un verrou bloqué à la main. Un vrai problème remonte tout de même au lieu d'être caché.

## Ma copie hors site n'a pas eu lieu après une sauvegarde

La réplication hors site est au mieux par conception, de sorte qu'un accroc hors site ne fait jamais échouer la sauvegarde locale. Vérifiez le planning hors site de ce domaine (Paramètres, Plannings) : un planning vide réplique après chaque sauvegarde locale, tandis qu'une cadence expédie moins souvent. Utilisez **Répliquer maintenant** dans l'onglet Hors site pour une exécution à la demande, et surveillez l'indicateur de réplication sur le tableau de bord.

## Une restauration s'est interrompue avant de démarrer

Avant que quoi que ce soit ne soit arrêté ou supprimé, la restauration exécute une vérification de conflit avant lancement : elle vérifie que l'IP statique du conteneur et les ports hôtes publiés sont libres. Si un autre conteneur en détient déjà un, elle s'interrompt avec un message clair et actionnable au lieu de laisser une restauration à moitié faite. Libérez le port ou l'IP en conflit, puis réessayez.

## Un export en clair a échoué au lieu d'écrire un fichier

Si le chiffrement age est activé (Paramètres) mais qu'aucun destinataire valide n'est défini, un export échoue avec une erreur claire au lieu d'écrire du texte en clair. Ajoutez un destinataire valide (une clé publique age ou une clé publique SSH), ou désactivez le chiffrement si vous voulez que l'export soit en clair. Voir [Fonctionnalités](features.md).

## Un dump de base de données a échoué

Un dump en échec ne fait jamais échouer la sauvegarde qui l'entoure ; il est enregistré comme une exécution en échec à lui seul, et la raison indique quoi corriger.

- **Connexion refusée.** Le dump se connecte avec les variables de mot de passe du conteneur lui-même (`POSTGRES_PASSWORD`, `MARIADB_ROOT_PASSWORD`, `MYSQL_ROOT_PASSWORD` ou leurs versions `_FILE`). Vérifiez-les sur le conteneur de base de données. Une variable `_FILE` pointant vers un secret que l'utilisateur du conteneur ne peut pas lire échoue de la même façon.
- **Privilèges manquants.** Avec un mot de passe root aléatoire, le dump ne peut se connecter que comme utilisateur applicatif, il ne contient donc que cette seule base, et MySQL 8.4 et plus récent peut le refuser purement et simplement. Donnez au conteneur un vrai mot de passe root, ou désactivez son dump.
- **Les tables système doivent être mises à niveau.** MariaDB refuse de se laisser vidanger quand ses tables système viennent d'une version plus ancienne (erreur 1558). Ajoutez la variable `MARIADB_AUTO_UPGRADE=1` et redémarrez le conteneur, ou lancez `mariadb-upgrade` une fois à l'intérieur.
- **Aucun outil de dump.** Une image allégée ou construite maison sans `pg_dump`, `mysqldump` ni `mariadb-dump` ne peut pas être vidangée. Utilisez l'image officielle, ou désactivez le dump.
- **Une limite de temps.** Un dump dispose de `DB_DUMP_MAX_HOURS` (6 par défaut), la sauvegarde autour de lui de `BACKUP_MAX_HOURS`, et un dump qui cesse d'avancer est coupé après `BACKUP_STALL_HOURS`. Ce dernier cas vient le plus souvent d'un verrou tenu par l'application. Relevez la limite qui s'est déclenchée, ou faites le dump pendant que l'application est calme.
- **Le conteneur est en pause ou redémarre.** Le dump parle au serveur en marche. Si le conteneur redémarre en boucle, son propre journal dit pourquoi.
- **Un dump endommagé n'a pas pu être supprimé.** Un dump que BombVault n'a pas pu terminer est supprimé. Quand cette suppression échoue, le dump reste dans la liste marqué comme endommagé et vous pouvez l'y supprimer.

## Un import a échoué

Un import arrête le conteneur, met son dossier de données de côté et laisse l'image en créer un vide à sa place. Si une étape avant l'import lui-même échoue, l'ancien dossier est remis en place tout seul. Si c'est l'import qui échoue, le conteneur garde le dossier neuf et l'ancien reste à côté sous le nom `<dossier de données>.bombvault-before-import-<horodatage>` ; le message d'erreur de l'exécution donne le chemin exact.

Pour le remettre à la main : arrêtez le conteneur, renommez le dossier de données actuel pour le dégager, renommez le dossier conservé sous son nom d'origine, puis démarrez le conteneur. Sur Unraid, le gestionnaire de fichiers de l'onglet Shares fait cela.

## Une sauvegarde de jeu de données ZFS a échoué ou a ignoré un jeu {#zfs-datasets}

Chaque problème porte un code de raison entre crochets, et la page [Jeux de données ZFS](zfs-datasets.md#reason-codes) les liste tous avec leur solution. Les trois plus fréquents :

- **`snapshot-loop`** : l'instantané n'est pas parvenu à BombVault parce que Host Data ne transmet pas les nouveaux montages. Modifiez le conteneur, réglez l'Access Mode de Host Data sur Read/Write - Slave et redémarrez BombVault.
- **`key-not-loaded`** : un jeu de données chiffré dont la clé n'est pas chargée est ignoré. Chargez la clé avec `zfs load-key` et montez le jeu ; la sauvegarde suivante l'inclut.
- **`ssh-auth`** : le serveur a refusé la clé de BombVault. La carte de connexion de la page ZFS affiche la commande qui l'autorise ; exécutez-la une fois sur le serveur.

## Un élément reste sur « Apprend N/10 »

La plupart des contrôles d'anomalies commencent après 10 sauvegardes réussies d'un élément, et le compte repart de zéro après **Marquer comme attendue** et après une modification de la sélection de l'élément. Un élément non planifié n'apprend pas, et un conteneur sans appdata n'a rien dont apprendre, ce que son badge indique.

## La rétention ne supprime plus les anciennes sauvegardes d'un élément

Une anomalie critique ouverte les retient : la source de l'élément est presque vide, a fortement rétréci, ou une sauvegarde a réenregistré la plupart de ses données. Ouvrez l'anomalie depuis le badge de l'élément. Si des données manquent ou ont été chiffrées, restaurez d'abord depuis la dernière bonne sauvegarde indiquée. Accusez ensuite réception de l'anomalie, ou marquez-la comme attendue si le changement vient de vous, et l'exécution suivante nettoie comme d'habitude. L'aperçu de rétention signale un tel élément comme conservé. Pour un élément ZFS, seul le jeu de données nommé dans l'anomalie garde ses anciennes sauvegardes ; les autres jeux de données de l'arborescence sont nettoyés comme d'habitude.

## Le nettoyage manuel indique que certains éléments ont été conservés

Même cause : le nettoyage laisse intactes les anciennes sauvegardes d'un élément ayant une telle anomalie et nomme l'élément dans son message. Tout le reste est nettoyé comme d'habitude.

## L'import de l'historique indique qu'un dépôt n'a pas pu être lu

Après la mise à jour, BombVault lit une fois la taille des sauvegardes antérieures dans chaque dépôt. Un dépôt injoignable à ce moment, par exemple une cible hors site en panne ou un partage non monté, est compté dans la carte **Anomalies** de **Paramètres, Intégrité** et réessayé une fois par jour. Ses éléments apprennent entre-temps des nouvelles sauvegardes.

## L'alerte d'espace disque ne correspond pas au tableau de bord Unraid

Sur le partage utilisateur d'Unraid (`/mnt/user`), l'espace libre est celui de toute la grappe, pas d'un seul disque. Les dépôts distants ne sont mesurés que via les remotes rclone qui indiquent leur espace libre ; les dépôts S3, B2, REST et SFTP n'ont pas de valeur et figurent comme non mesurés dans la carte **Anomalies**.

## Un assistant IA n'arrive pas à se connecter

La page [Serveur MCP](mcp.md#troubleshooting) indique ce que signifie chaque code d'état et chaque refus du point de connexion MCP, et ce qu'il faut faire.

## Le conteneur redémarre sans cesse ou semble non sain

BombVault se signale sain/non sain depuis son propre `/api/health`. Un outil d'auto-réparation (comme Autoheal) peut le redémarrer automatiquement si le moteur venait à se coincer. Vérifiez le journal du conteneur et le rapport `/spike` pour la cause sous-jacente.

## Toujours bloqué ?

- Lisez les pages complètes [Configuration](configuration.md) et [Sauvegarde hors site et récupération](offsite-recovery.md).
- Demandez sur le [fil de support Unraid](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/).
- Ouvrez un [ticket GitHub](https://github.com/junkerderprovinz/bombvault/issues).
