# Dépannage

Une courte FAQ. Pour le tableau complet de dépannage côté hôte de la sauvegarde de VM via SSH (permission refusée, vérification de clé d'hôte, variables de modèle manquantes et plus), voir le [guide de sauvegarde de VM via SSH](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) sur GitHub.

## Quelque chose n'est pas correctement branché

Ajoutez la carte **Intégration hôte** à l'aperçu sous **Personnaliser**. La vérification de l'intégration hôte sonde chaque montage et CLI (socket Docker, libvirt, restic, qemu-img, rclone) et signale toute pièce manquante, et **Vérifier maintenant** la relance. Commencez ici avant de supposer un bug : un montage manquant ou un hôte injoignable apparaît immédiatement.

## Je ne peux pas atteindre l'interface web

BombVault sert du HTTPS par défaut sur le port `3443` (certificat auto-signé), ouvrez donc `https://<votre-ip-unraid>:3443`. Acceptez l'avertissement de certificat auto-signé, ou placez BombVault derrière un reverse proxy avec votre propre certificat. Si vous l'exécutez avec `HTTP_ONLY=true`, il sert du HTTP simple sur le port `3000` à la place (destiné à un usage derrière un proxy terminant le TLS).

## J'ai perdu mon APP_KEY

`APP_KEY` dérive le mot de passe du dépôt restic. Sans lui, sans le kit de récupération et sans copie déposée dans votre groupe, les sauvegardes chiffrées ne peuvent pas être récupérées. C'est pourquoi l'aperçu affiche **Sauvegardez votre kit de récupération** tant que vous ne l'avez pas téléchargé. Voir [Sauvegarde hors site et récupération](offsite-recovery.md#recovery-kit). Générez une clé avec `openssl rand -hex 32` et conservez-la hors du serveur avant de vous fier à une quelconque sauvegarde.

## J'ai oublié le mot de passe de connexion

Choisissez **Mot de passe oublié ?** sur la page de connexion. BombVault écrit un code de 12 caractères dans le journal de son conteneur (sous Unraid : onglet Docker, icône BombVault, **Logs**). Saisissez-le avec un nouveau mot de passe dans les 10 minutes. La réinitialisation désactive aussi l'authentification à deux facteurs, supprime toutes les clés d'accès, révoque tous les jetons d'API, clés MCP et connexions OAuth, ferme toutes les autres sessions et désactive l'appairage des téléphones. Ajoutez à nouveau vos clés d'accès sous **Paramètres, Sécurité**, réactivez-y l'appairage des téléphones, et créez de nouveaux jetons et de nouvelles clés là où vous en avez encore besoin. Voir [Modèle de sécurité](configuration.md#login).

## Les sauvegardes se sont arrêtées et indiquent que la clé ne peut plus être ouverte

Ce serveur a repris la clé de sauvegarde d'un autre serveur dans l'Assistant d'urgence, et `APP_KEY` a été modifiée ensuite. La clé reprise est scellée avec l'ancienne APP_KEY : BombVault ne peut donc pas l'ouvrir et ne sauvegarde rien plutôt que d'écrire avec la mauvaise clé. **À traiter** sur l'aperçu la nomme. Remettez APP_KEY comme elle était, ou téléversez à nouveau le kit de récupération sur la page Restauration sous **Urgence**.

## Un téléphone arrive sur la page de connexion

Un téléphone appairé reçoit sa session via le groupe seulement tant que **Les téléphones peuvent s'appairer avec les douze mots du groupe** est activé sous **Paramètres, Sécurité**. Une réinitialisation du mot de passe le désactive. Une application plus ancienne que le serveur ne reçoit pas non plus de session avant d'être mise à jour. Un serveur ajouté par son adresse demande toujours le mot de passe. Un téléphone supprimé de la page Instances revient désactivé quand son application s'ouvre de nouveau ; activez **Interroger cette instance** sous **Surveillance** dans ses détails pour le laisser entrer.

## La sauvegarde de VM ne se connecte pas

La sauvegarde de VM dialogue avec libvirt via SSH, jamais un montage.

- Confirmez que SSH est activé sur l'hôte et que la clé publique de BombVault est autorisée dans `/root/.ssh/authorized_keys` (Paramètres, Intégrations, SSH de l'hôte affiche la clé et un bouton **Tester la connexion**).
- Sur un réseau `br0.x` personnalisé, réglez `LIBVIRT_HOST` sur l'IP LAN de votre Unraid (le conteneur ne peut pas y atteindre l'hôte via `host.docker.internal`). Activez **Settings, Docker, Host access to custom networks**.
- Si vous avez changé le port SSH d'Unraid, réglez `LIBVIRT_SSH_PORT` en conséquence.
- Sur TrueNAS, définissez `LIBVIRT_URI` avec le socket libvirt propre à TrueNAS, donnez à l'application la capacité `SETPCAP` et ajoutez la clé à root sous **Credentials, Users** (voir [TrueNAS SCALE](getting-started.md#truenas)).
- Le diagnostic complet pas à pas (test d'accessibilité, routage VLAN, `Permission denied (publickey)`, `Host key verification failed`) se trouve dans le [guide de sauvegarde de VM via SSH](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md).

## Un instantané de VM à chaud ne s'est pas exécuté

Les instantanés à chaud nécessitent l'agent invité qemu installé dans la VM et le disque sur `/mnt/cache` (ou `/mnt/diskX`), pas `/mnt/user`. Sur une VM éteinte, le mode à chaud se rabat automatiquement sur le mode propre. Une sauvegarde propre arrête la VM, sauvegarde les disques, puis la redémarre, elle est donc toujours cohérente. Elle attend jusqu'à dix minutes que l'invité s'arrête et ne le force jamais à s'éteindre, pour qu'un invité Windows en train d'installer des mises à jour ne soit pas coupé en plein milieu. Un invité qui tourne encore après ce délai est laissé tranquille, et cette sauvegarde échoue en indiquant la raison.

## Une sauvegarde a échoué avec « repository is already locked »

C'est généralement un verrou restic orphelin laissé lorsque le conteneur a été mis à jour ou redémarré en pleine opération. BombVault détecte un verrou prouvé orphelin, le force à se libérer et réessaie une fois, automatiquement. S'il persiste, ouvrez la carte **Vérifier les sauvegardes** sous **Paramètres, Vérification** et choisissez **Déverrouiller** sous **Plus** dans la ligne de la section concernée pour libérer un verrou bloqué à la main. Un vrai problème remonte tout de même au lieu d'être caché. Après un redémarrage, BombVault attend qu'un tel verrou soit resté dix minutes sans être rafraîchi. Un restic encore en marche, par exemple dans un second BombVault sur le même dépôt, rafraîchit son verrou toutes les cinq minutes.

## Ma copie hors site n'a pas eu lieu après une sauvegarde

La réplication hors site est au mieux par conception, de sorte qu'un accroc hors site ne fait jamais échouer la sauvegarde locale. Vérifiez quand la destination copie : sa carte **Copier** indique **Après chaque sauvegarde** ou nomme un planning propre, et **Paramètres, Planning, Copies hors site par section** peut donner à une section son propre planning. Utilisez **Répliquer maintenant** dans la carte **Copie hors site** de la section sous **Paramètres, Destinations** pour une exécution à la demande, et surveillez l'en-tête d'état de l'aperçu, qui affiche **Copie hors site en cours** pendant l'exécution. Une copie échouée apparaît sous **À traiter**.

## Le certificat d'un rest-server n'est pas reconnu

restic refuse un certificat qu'il ne connaît pas, et le message d'erreur indique où le placer. Copiez le certificat auto-signé du rest-server, ou celui de l'autorité qui l'a signé, sous forme de fichier PEM dans le dossier `certs` du dossier de données de BombVault (`/config/certs` dans le conteneur, `appdata/bombvault/certs` sous Unraid), puis testez de nouveau. Aucun redémarrage n'est nécessaire.

## Une sauvegarde indique que des données hors de Host Data n'ont pas été sauvegardées {#data-outside-host-data}

Le conteneur garde des données dans un dossier que BombVault ne peut pas lire via Host Data. La sauvegarde enregistre la définition et chaque dossier qu'elle atteint, colore sa ligne en orange dans le journal d'activité et nomme les dossiers laissés de côté. Deux cas sont fréquents : un ixVolume sur TrueNAS sous `/mnt/.ix-apps`, et un volume Docker nommé sous `/var/lib/docker/volumes`, comme le volume de base de données d'une stack compose. La carte **Dossiers à sauvegarder** de l'onglet **Paramètres** du conteneur affiche chacun de ces dossiers avec son chemin dans le conteneur et son chemin sur l'hôte, marqué en rouge.

Un volume Docker anonyme n'est pas signalé. Docker le crée de lui-même pour un dossier déclaré par l'image et que personne n'a mappé, lui donne un nom de 64 chiffres hexadécimaux et le range sous `/var/lib/docker/volumes`. Il contient le plus souvent un cache, la sauvegarde reste donc verte et la carte se contente de l'afficher, avec une note indiquant qu'il est anonyme.

- **Vous voulez sauvegarder ces données.** Mappez le dossier sur un chemin de l'hôte sous Host Data, par exemple un dossier de votre partage appdata, et la prochaine sauvegarde l'inclut.
- **Vous n'en avez pas besoin.** Ajoutez l'un de ses deux chemins comme ligne dans la carte **Motifs d'exclusion** du même onglet : le chemin dans le conteneur, comme `/cache`, ou le chemin sur l'hôte que nomme l'avertissement, comme `/var/lib/docker/volumes/jellyfin_cache/_data`. À partir de la prochaine sauvegarde, l'avertissement laisse ce dossier de côté, et l'aperçu sous le champ dit pourquoi. Une ligne pour un dossier au-dessus fonctionne aussi, un nom simple ou `/` non.

## Une sauvegarde indique Reporté {#deferred-backups}

L'application du conteneur était occupée, sa sauvegarde attend donc et sera réessayée d'elle-même. La ligne du journal d'activité dit pourquoi et quand a lieu le prochain essai ; la carte **Quand l'application est occupée** de l'onglet **Paramètres** du conteneur dit la même chose, ainsi que la fin de l'attente. Vous n'avez rien à faire. Pour sauvegarder tout de suite, utilisez **Sauvegarder maintenant** une fois l'application libre.

- **Votre propre vérification.** Faites sortir la **Commande pré-sauvegarde** avec le code 75 tant que l'application est occupée, par exemple `[ "$(cat /tmp/players)" = 0 ] || exit 75`. Tout autre code différent de 0 fait échouer la sauvegarde comme avant, un script cassé apparaît donc toujours sous **À traiter**.
- **Plex, Jellyfin ou Emby n'est jamais interrogé.** BombVault interroge d'abord le serveur à l'adresse du conteneur et, si rien n'y répond, lance curl ou wget dans le conteneur. Une image qui n'a ni l'un ni l'autre ne peut pas être interrogée par elle-même ; le journal le dit alors. Avec **Sauvegarder quand le serveur ne répond pas** activé, une telle sauvegarde a lieu quand même ; désactivé, elle attend.
- **Jellyfin ou Emby refuse.** Vérifiez la **Clé API**. Elle se crée dans le tableau de bord du serveur, sous Clés API.
- **Plex refuse.** BombVault lit le jeton dans `Preferences.xml`, dans le dossier monté sur `/config`. Un serveur qui n'a jamais été connecté à un compte n'en a pas et ne répond que là où il autorise l'accès sans jeton.
- **Un film en pause fait attendre la sauvegarde toute la nuit.** Réglez **Ignorer les sessions en pause après (min)**, par exemple sur 30.
- **La vérification du trafic ne laisse jamais passer la sauvegarde.** Montez **Occupé à partir de (KiB/s)** au-dessus de ce que le conteneur envoie quand personne ne l'utilise. Juste après le démarrage de BombVault, la vérification attend d'avoir observé le conteneur pendant **Calme pendant (min)** minutes.
- **La sauvegarde ne s'exécute jamais la nuit.** Avec une plage définie, les essais n'ont lieu qu'à l'intérieur. Une attente qui atteint son échéance hors de la plage se termine quand même à l'échéance.

## Une grosse sauvegarde a été tuée faute de mémoire

Réglez **Limite de mémoire par sauvegarde (MiB, 0 = aucune)** dans la carte **Performances** sous **Paramètres, Système**, bien en dessous de ce que le conteneur peut utiliser. À l'approche de la limite, restic libère la mémoire plus souvent, ce qui coûte un peu de vitesse : la sauvegarde ralentit au lieu d'être tuée.

## Une restauration s'est interrompue avant de démarrer

Avant que quoi que ce soit ne soit arrêté ou supprimé, la restauration exécute une vérification de conflit avant lancement : elle vérifie que l'IP statique du conteneur et les ports hôtes publiés sont libres. Si un autre conteneur en détient déjà un, elle s'interrompt avec un message clair et actionnable au lieu de laisser une restauration à moitié faite. Libérez le port ou l'IP en conflit, puis réessayez.

Si le réseau du conteneur n'existe pas, par exemple parce que son application TrueNAS a été supprimée, le contrôle avant la restauration nomme le réseau et la restauration s'arrête avant de modifier quoi que ce soit. Réinstallez l'application dans TrueNAS sous le même nom, ce qui recrée le réseau, puis restaurez ses données.

## Un export en clair a échoué au lieu d'écrire un fichier

Si **Chiffrer les exports avec age** est activé sous **Paramètres, Système** mais qu'aucun destinataire valide n'est défini, un export échoue avec une erreur claire au lieu d'écrire du texte en clair. Ajoutez un destinataire valide (une clé publique age ou une clé publique SSH), ou désactivez le chiffrement si vous voulez que l'export soit en clair. Voir [Fonctionnalités](features.md).

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

Un dossier de données qui est lui-même un jeu de données, comme sur TrueNAS, ne peut pas être renommé. L'import laisse alors le dossier en place et déplace son contenu dans le dossier conservé à côté : le jeu de données au-dessus doit donc avoir la place pour le recevoir. L'import le vérifie avant d'arrêter le conteneur.

Pour le remettre à la main : arrêtez le conteneur, renommez le dossier de données actuel pour le dégager, renommez le dossier conservé sous son nom d'origine, puis démarrez le conteneur. Pour un jeu de données, déplacez le contenu au lieu de renommer les dossiers. Sur Unraid, le gestionnaire de fichiers de l'onglet Shares fait cela.

## Une sauvegarde de jeu de données ZFS a échoué ou a ignoré un jeu {#zfs-datasets}

Chaque problème porte un code de raison entre crochets, et la page [Jeux de données ZFS](zfs-datasets.md#reason-codes) les liste tous avec leur solution. Les trois plus fréquents :

- **`snapshot-loop`** : l'instantané n'est pas parvenu à BombVault parce que Host Data ne transmet pas les nouveaux montages. Modifiez le conteneur, réglez l'Access Mode de Host Data sur Read/Write - Slave et redémarrez BombVault.
- **`key-not-loaded`** : un jeu de données chiffré dont la clé n'est pas chargée est ignoré. Chargez la clé avec `zfs load-key` et montez le jeu ; la sauvegarde suivante l'inclut.
- **`ssh-auth`** : le serveur a refusé la clé de BombVault. **Connexion ZFS à ce serveur**, qu'ouvre **Voir la connexion** sur la page Sauvegardes, affiche la commande qui l'autorise ; exécutez-la une fois sur le serveur.

## Un élément reste sur « Apprend N/10 »

La plupart des contrôles d'anomalies commencent après 10 sauvegardes réussies d'un élément, et le compte repart de zéro après **Marquer comme attendue** et après une modification de la sélection de l'élément. Un élément non planifié n'apprend pas, et un conteneur sans appdata n'a rien dont apprendre, ce que son badge indique. **Progression de l'apprentissage** sur la page Anomalies montre un anneau de dix parties par élément.

## La rétention ne supprime plus les anciennes sauvegardes d'un élément

Une anomalie critique ouverte les retient : la source de l'élément est presque vide, a fortement rétréci, ou une sauvegarde a réenregistré la plupart de ses données. Ouvrez l'anomalie depuis le badge de l'élément. Si des données manquent ou ont été chiffrées, restaurez d'abord depuis la dernière bonne sauvegarde indiquée. Accusez ensuite réception de l'anomalie, ou marquez-la comme attendue si le changement vient de vous, et l'exécution suivante nettoie comme d'habitude. L'aperçu de rétention signale un tel élément comme conservé. Pour un élément ZFS, seul le jeu de données nommé dans l'anomalie garde ses anciennes sauvegardes ; les autres jeux de données de l'arborescence sont nettoyés comme d'habitude.

## Le nettoyage manuel indique que certains éléments ont été conservés

Même cause : le nettoyage laisse intactes les anciennes sauvegardes d'un élément ayant une telle anomalie et nomme l'élément dans son message. Tout le reste est nettoyé comme d'habitude.

## L'import de l'historique indique qu'un dépôt n'a pas pu être lu

Après la mise à jour, BombVault lit une fois la taille des sauvegardes antérieures dans chaque dépôt. Un dépôt injoignable à ce moment, par exemple une destination hors site en panne ou un partage non monté, est compté dans la carte **Anomalies** de **Paramètres, Vérification** et réessayé une fois par jour. Ses éléments apprennent entre-temps des nouvelles sauvegardes.

## L'alerte d'espace disque ne correspond pas au tableau de bord Unraid

Sur le partage utilisateur d'Unraid (`/mnt/user`), l'espace libre est celui de toute la grappe, pas d'un seul disque. Les dépôts distants ne sont mesurés que via les remotes rclone qui indiquent leur espace libre ; les dépôts S3, B2, REST et SFTP n'ont pas de valeur et figurent comme non mesurés dans la carte **Anomalies**.

## Un assistant IA n'arrive pas à se connecter

La page [Serveur MCP](mcp.md#troubleshooting) indique ce que signifie chaque code d'état et chaque refus du point de connexion MCP, et ce qu'il faut faire.

## Le conteneur redémarre sans cesse ou semble non sain

BombVault se signale sain/non sain depuis son propre `/api/health`. Un outil d'auto-réparation (comme Autoheal) peut le redémarrer automatiquement si le moteur venait à se coincer. Vérifiez le journal du conteneur et la carte **Intégration hôte** pour la cause sous-jacente.

## Toujours bloqué ?

- Lisez les pages complètes [Configuration](configuration.md) et [Sauvegarde hors site et récupération](offsite-recovery.md).
- Demandez sur le [fil de support Unraid](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/).
- Ouvrez un [ticket GitHub](https://github.com/junkerderprovinz/bombvault/issues).
