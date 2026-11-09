# Dépannage

Une courte FAQ. Pour le tableau complet de dépannage côté hôte de la sauvegarde de VM via SSH (permission refusée, vérification de la clé d'hôte, variables de modèle manquantes et plus), voir le [guide de la sauvegarde de VM via SSH](vm-backup-ssh-setup.md).

## Quelque chose n'est pas raccordé correctement {#something-is-not-wired-up-correctly}

Ajoutez la carte **Intégration hôte** à l'aperçu sous **Personnaliser**. Le contrôle de l'intégration à l'hôte sonde chaque montage et chaque CLI (socket Docker, libvirt, restic, qemu-img, rclone) et signale les éléments manquants, et **Vérifier maintenant** le relance. Commencez ici avant de supposer un bug : un montage manquant ou un hôte injoignable se voit tout de suite.

## Je n'arrive pas à joindre l'interface web {#i-cannot-reach-the-web-ui}

BombVault sert du HTTPS par défaut sur le port `3443` (certificat autosigné), ouvrez donc `https://<your-unraid-ip>:3443`. Acceptez l'avertissement du certificat autosigné, ou placez BombVault derrière un reverse proxy avec votre propre certificat. Si vous l'exécutez avec `HTTP_ONLY=true`, il sert du HTTP simple sur le port `3000` à la place (prévu pour un usage derrière un proxy qui termine le TLS).

## J'ai perdu mon APP_KEY {#i-lost-my-app_key}

`APP_KEY` sert à dériver le mot de passe du dépôt restic. Sans elle, sans le kit de récupération et sans dépôt de clé dans votre groupe, les sauvegardes chiffrées ne peuvent pas être récupérées. C'est pourquoi l'aperçu affiche **Sauvegardez votre kit de récupération** tant que vous ne l'avez pas téléchargé. Voir [Hors site et récupération](offsite-recovery.md#recovery-kit). Générez une clé avec `openssl rand -hex 32` et conservez-la hors du serveur avant de compter sur une sauvegarde.

## J'ai oublié le mot de passe de connexion {#i-forgot-the-login-password}

Choisissez **Mot de passe oublié ?** sur la page de connexion. BombVault écrit un code de 12 caractères dans le journal de son conteneur (sur Unraid : onglet Docker, icône BombVault, **Journaux**). Saisissez-le avec un nouveau mot de passe dans les 10 minutes. La réinitialisation désactive aussi l'authentification à deux facteurs, supprime toutes les clés d'accès, révoque tous les jetons d'API, toutes les clés MCP et toutes les connexions OAuth, met fin à toutes les autres sessions et désactive l'appairage des téléphones. Ajoutez de nouveau vos clés d'accès sous **Paramètres, Sécurité**, réactivez-y l'appairage des téléphones, et créez de nouveaux jetons et de nouvelles clés là où vous en avez encore besoin. Voir [Modèle de sécurité](configuration.md#login).

## Les sauvegardes se sont arrêtées et disent que la clé ne peut plus être ouverte {#backups-stopped-and-say-the-key-can-no-longer-be-opened}

Ce serveur a repris la clé de sauvegarde d'un autre serveur dans l'assistant d'urgence, et `APP_KEY` a été changée ensuite. La clé reprise est scellée avec l'ancienne APP_KEY, BombVault ne peut donc pas l'ouvrir et ne sauvegarde rien plutôt que d'écrire avec la mauvaise clé. **À traiter** sur l'aperçu le nomme. Remettez APP_KEY comme elle était, ou téléversez de nouveau le kit de récupération sur la page Restauration sous **Urgence**.

## Un téléphone arrive sur la page de connexion {#a-phone-lands-on-the-login-page}

Un téléphone appairé n'obtient sa session via le groupe que tant que **Les téléphones peuvent s'appairer avec les douze mots du groupe** est activé sous **Paramètres, Sécurité**. Une réinitialisation du mot de passe le désactive. Une application plus ancienne que le serveur n'obtient pas non plus de session avant d'être mise à jour. Un serveur ajouté par son adresse demande toujours le mot de passe. Un téléphone retiré de la page Instances revient désactivé quand son application s'ouvre de nouveau. Activez **Interroger cette instance** sous **Surveillance** dans ses détails pour le laisser entrer.

## La sauvegarde de VM ne se connecte pas {#vm-backup-will-not-connect}

La sauvegarde de VM parle à libvirt via SSH, jamais par un montage.

- Vérifiez que SSH est activé sur l'hôte et que la clé publique de BombVault est autorisée dans `/root/.ssh/authorized_keys` (Paramètres, Intégrations, SSH de l'hôte montre la clé et un bouton **Tester la connexion**).
- Sur un réseau personnalisé `br0.x`, réglez `LIBVIRT_HOST` sur l'IP de votre Unraid sur le réseau local (le conteneur ne peut pas joindre l'hôte via `host.docker.internal` là). Activez **Settings, Docker, Host access to custom networks**.
- Si vous avez changé le port SSH d'Unraid, réglez `LIBVIRT_SSH_PORT` en conséquence.
- Sur TrueNAS, réglez `LIBVIRT_URI` avec le propre socket libvirt de TrueNAS, donnez à l'application la capacité `SETPCAP` et ajoutez la clé à root sous **Credentials, Users** (voir [TrueNAS SCALE](getting-started.md#truenas)).
- Le diagnostic complet pas à pas (test de joignabilité, routage VLAN, `Permission denied (publickey)`, `Host key verification failed`) se trouve dans le [guide de la sauvegarde de VM via SSH](vm-backup-ssh-setup.md).

## Un instantané de VM à chaud n'a pas eu lieu {#a-live-vm-snapshot-did-not-run}

Les instantanés à chaud demandent le disque sur `/mnt/cache` (ou `/mnt/diskX`), pas sur `/mnt/user`. Avec l'agent invité qemu dans la VM, l'instantané est figé, et sans lui, il est cohérent en cas de plantage. Sur une VM éteinte, le mode à chaud retombe automatiquement sur le mode propre. Une sauvegarde propre arrête la VM, sauvegarde les disques, puis la redémarre, elle est donc toujours cohérente. Elle attend jusqu'à dix minutes que l'invité s'arrête et ne force jamais l'extinction, si bien qu'un invité Windows qui installe des mises à jour n'est pas coupé en plein milieu. Un invité qui tourne encore après cela est laissé tranquille, et cette sauvegarde échoue avec la raison.

## Une sauvegarde a échoué avec « repository is already locked » {#a-backup-failed-with-repository-is-already-locked}

C'est en général un verrou restic orphelin laissé quand le conteneur a été mis à jour ou redémarré au milieu d'une opération. BombVault détecte un verrou manifestement orphelin, le lève et réessaie une fois, automatiquement. Après un redémarrage, il attend qu'un tel verrou soit resté dix minutes sans être rafraîchi. Un restic qui tourne encore rafraîchit son verrou toutes les cinq minutes, et BombVault ne retire jamais un verrou qui est encore rafraîchi, car il peut appartenir à un second BombVault qui écrit dans le même dépôt. Un verrou resté 30 minutes sans être rafraîchi est retiré, quel que soit celui qui l'a posé. Si l'erreur persiste, ouvrez la carte **Vérifier les sauvegardes** sous **Paramètres, Vérification** et choisissez **Déverrouiller** sous **Plus** dans la ligne de la section concernée pour lever un verrou périmé à la main. **Déverrouiller** laisse lui aussi en place un verrou encore rafraîchi et vous indique quand il l'a été pour la dernière fois. Un vrai problème refait surface au lieu d'être caché.

## Ma copie hors site n'a pas eu lieu après une sauvegarde {#my-off-site-copy-did-not-happen-after-a-backup}

La réplication hors site est faite au mieux par conception, si bien qu'un accroc hors site ne fait jamais échouer la sauvegarde locale. Vérifiez quand la destination copie : sa carte **Copier** indique **Après chaque sauvegarde** ou nomme un planning à elle, et **Paramètres, Planning, Copies hors site par section** peut donner à une section son propre planning. Utilisez **Répliquer maintenant** dans la ligne de la section, dans la carte **Copies hors site** sous **Paramètres, Destinations**, pour une exécution à la demande, et surveillez l'en-tête d'état de l'aperçu, qui indique **Copie hors site en cours** pendant qu'elle tourne. Une copie échouée apparaît sous **À traiter**.

## Le certificat d'un rest-server n'est pas approuvé {#a-rest-servers-certificate-is-not-trusted}

restic refuse un certificat qu'il ne connaît pas, et l'erreur dit où le placer. Copiez le certificat autosigné du rest-server, ou celui de l'autorité qui l'a signé, comme fichier PEM dans le dossier `certs` du dossier de données de BombVault (`/config/certs` dans le conteneur, `appdata/bombvault/certs` sur Unraid) et testez de nouveau. Aucun redémarrage n'est nécessaire.

## Une sauvegarde dit que des données en dehors de Host Data n'ont pas été sauvegardées {#data-outside-host-data}

Le conteneur garde des données dans un dossier que BombVault ne peut pas lire via Host Data. La sauvegarde enregistre la définition et chaque dossier qu'elle peut atteindre, passe sa ligne du journal d'activité en orange et nomme les dossiers laissés de côté. Deux cas sont fréquents : un ixVolume sur TrueNAS sous `/mnt/.ix-apps`, et un volume Docker nommé sous `/var/lib/docker/volumes`, comme le volume de base de données d'une stack compose. La carte **Dossiers à sauvegarder** de l'onglet **Paramètres** du conteneur liste chacun de ces dossiers avec son chemin dans le conteneur et son chemin sur l'hôte, marqués en rouge.

Un volume Docker anonyme n'est pas signalé. Docker en crée un tout seul pour un dossier que l'image déclare et que personne n'a associé, le nomme avec 64 chiffres hexadécimaux et le garde sous `/var/lib/docker/volumes`. Il contient en général un cache, la sauvegarde reste donc verte et la carte ne fait que le lister, avec une remarque disant qu'il est anonyme.

- **Vous voulez que ces données soient sauvegardées.** Associez le dossier à un chemin de l'hôte sous Host Data, par exemple un dossier de votre partage appdata, et la prochaine sauvegarde l'inclut.
- **Vous n'en avez pas besoin.** Ajoutez l'un de ses deux chemins comme ligne dans la carte **Motifs d'exclusion** du même onglet : le chemin dans le conteneur, comme `/cache`, ou le chemin sur l'hôte que nomme l'avertissement, comme `/var/lib/docker/volumes/jellyfin_cache/_data`. À partir de la prochaine sauvegarde, l'avertissement laisse ce dossier de côté, et l'aperçu sous le champ dit pourquoi. Une ligne pour un dossier au-dessus marche aussi, un simple nom ou `/` non.

## Une sauvegarde indique Reporté {#deferred-backups}

L'application du conteneur était occupée, si bien que sa sauvegarde attend et est retentée toute seule. La ligne du journal d'activité dit pourquoi et quand a lieu le prochain essai. La carte **Quand l'application est occupée** de l'onglet **Paramètres** du conteneur dit la même chose, ainsi que la fin de l'attente. Il n'y a rien à faire. Pour sauvegarder tout de suite, utilisez **Sauvegarder maintenant** : une sauvegarde que vous lancez vous-même n'attend jamais, pas même pour un code de sortie 75.

- **Votre propre contrôle.** Faites terminer la **Commande pré-sauvegarde** avec le code 75 tant que l'application est occupée, par exemple `[ "$(cat /tmp/players)" = 0 ] || exit 75`. Tout autre code différent de 0 fait échouer la sauvegarde comme avant, si bien qu'un script cassé apparaît toujours sous **À traiter**.
- **Plex, Jellyfin ou Emby n'est jamais interrogé.** BombVault interroge d'abord le serveur à l'adresse propre du conteneur et, quand elle ne répond pas, exécute curl ou wget dans le conteneur. Une image qui n'a ni l'un ni l'autre ne peut pas être interrogée par elle-même. Le journal le dit alors. Avec **Sauvegarder quand le serveur ne répond pas** activé, une telle sauvegarde s'exécute quand même, désactivé elle attend.
- **Jellyfin ou Emby refuse.** Vérifiez la **Clé API**. Elle se crée dans le tableau de bord du serveur sous API keys.
- **Plex refuse.** BombVault lit le jeton dans `Preferences.xml`, dans le dossier monté sur `/config`. Un serveur auquel on ne s'est jamais connecté n'en a pas et ne répond que là où il autorise l'accès sans jeton.
- **Un film en pause garde la sauvegarde en attente toute la nuit.** Réglez **Ignorer les sessions en pause après (min)**, par exemple sur 30.
- **Le contrôle du trafic ne laisse jamais passer la sauvegarde.** Augmentez **Occupé à partir de (KiB/s)** au-dessus de ce que le conteneur envoie quand personne ne s'en sert. Juste après le démarrage de BombVault, le contrôle attend d'avoir observé le conteneur pendant **Calme pendant (min)** minutes.
- **La sauvegarde ne s'exécute jamais la nuit.** Avec une plage horaire réglée, les essais n'ont lieu qu'à l'intérieur. Une attente qui atteint son échéance hors de la plage se termine quand même à l'échéance.

## Une grosse sauvegarde a été tuée faute de mémoire {#a-large-backup-was-killed-for-running-out-of-memory}

Réglez **Limite de mémoire par sauvegarde (MiB, 0 = aucune)** dans la carte **Performances** sous **Paramètres, Système**, bien en dessous de ce que le conteneur peut utiliser. Près de la limite, restic libère la mémoire plus souvent, ce qui coûte un peu de vitesse, la sauvegarde ralentit donc au lieu d'être tuée.

## Une restauration s'est interrompue avant de démarrer {#a-restore-aborted-before-it-started}

Avant que quoi que ce soit soit arrêté ou supprimé, la restauration lance un contrôle préalable de conflits : elle vérifie que l'IP statique du conteneur et ses ports d'hôte publiés sont libres. Si un autre conteneur en tient déjà un, elle s'interrompt avec un message clair et exploitable au lieu de laisser une restauration à moitié faite. Libérez le port ou l'IP en conflit, puis réessayez.

Si le réseau du conteneur n'existe pas, par exemple parce que son application TrueNAS a été supprimée, le contrôle de restauration nomme le réseau et la restauration refuse avant de rien changer. Réinstallez l'application dans TrueNAS sous le même nom, ce qui recrée le réseau, puis restaurez ses données.

## Un export simple a échoué au lieu d'écrire un fichier {#a-plain-export-failed-instead-of-writing-a-file}

Si **Chiffrer les exports avec age** est activé sous **Paramètres, Système** mais qu'aucun destinataire valide n'est défini, un export échoue avec une erreur claire au lieu d'écrire du texte en clair. Ajoutez un destinataire valide (une clé publique age ou une clé publique SSH), ou désactivez le chiffrement si vous voulez que l'export soit en clair. Voir [Exports simples](backups.md#exports).

## Un dump de base de données a échoué {#a-database-dump-failed}

Un dump échoué ne fait jamais échouer la sauvegarde qui l'entoure. Il est enregistré comme sa propre exécution échouée, et la raison nomme ce qu'il faut corriger.

- **Connexion refusée.** Le dump se connecte avec les variables de mot de passe du conteneur lui-même (`POSTGRES_PASSWORD`, `MARIADB_ROOT_PASSWORD`, `MYSQL_ROOT_PASSWORD` ou leurs versions `_FILE`). Vérifiez-les sur le conteneur de la base. Une variable `_FILE` qui pointe vers un secret que l'utilisateur du conteneur ne peut pas lire échoue de la même façon.
- **Privilèges manquants.** Avec un mot de passe root aléatoire, le dump ne peut se connecter que comme utilisateur de l'application, il ne contient donc que cette base, et MySQL 8.4 et plus récent peut le refuser net. Donnez au conteneur un vrai mot de passe root, ou désactivez le dump pour lui.
- **Les tables système demandent une mise à niveau.** MariaDB refuse de dumper quand ses tables système viennent d'une version plus ancienne (erreur 1558). Ajoutez la variable `MARIADB_AUTO_UPGRADE=1` et redémarrez le conteneur, ou lancez `mariadb-upgrade` dedans une fois.
- **Pas d'outil de dump.** Une image allégée ou construite à la main sans `pg_dump`, `mysqldump` ou `mariadb-dump` ne peut pas être exportée. Utilisez l'image officielle, ou désactivez le dump.
- **Une limite de temps.** Un dump reçoit `DB_DUMP_MAX_HOURS` (6 par défaut), la sauvegarde qui l'entoure reçoit `BACKUP_MAX_HOURS`, et un dump qui cesse de progresser est coupé après `BACKUP_STALL_HOURS`. La cause habituelle de ce dernier cas est un verrou que tient l'application. Augmentez la limite qui s'est déclenchée, ou exportez quand l'application est calme.
- **Le conteneur est en pause ou redémarre.** Le dump parle au serveur en marche. Si le conteneur redémarre sans cesse, son propre journal dit pourquoi.
- **Un dump endommagé n'a pas pu être supprimé.** Un dump que BombVault n'a pas pu terminer est supprimé de nouveau. Quand cette suppression échoue, le dump reste dans la liste marqué comme endommagé, et vous pouvez l'y supprimer.

## Un import a échoué {#an-import-failed}

Un import arrête le conteneur, met son dossier de données de côté et laisse l'image en créer un vide à la place. Si une étape avant l'import lui-même échoue, l'ancien dossier est remis en place automatiquement. Si l'import échoue, le conteneur garde le dossier neuf et l'ancien reste à côté sous le nom `<data folder>.bombvault-before-import-<timestamp>`. Le message d'erreur de l'exécution nomme le chemin exact.

Un dossier de données qui est lui-même un dataset, comme sur TrueNAS, ne peut pas être renommé. L'import laisse alors le dossier en place et déplace son contenu dans le dossier conservé à côté, le dataset du dessus a donc besoin de place pour lui. L'import le vérifie avant d'arrêter le conteneur.

Pour le remettre à la main : arrêtez le conteneur, renommez le dossier de données actuel pour le mettre de côté, redonnez au dossier conservé son nom d'origine et démarrez le conteneur. Pour un dataset, déplacez le contenu au lieu de renommer les dossiers. Sur Unraid, le gestionnaire de fichiers le fait depuis l'onglet Shares.

## Une sauvegarde de jeu de données ZFS a échoué ou a ignoré un jeu de données {#zfs-datasets}

Chaque problème porte un code de raison entre crochets, et la page [Jeux de données ZFS](zfs-datasets.md#reason-codes) les liste tous avec le remède. Les trois plus fréquents :

- **`snapshot-loop`** : l'instantané n'est pas parvenu à BombVault parce que Host Data ne transmet pas les nouveaux montages. Modifiez le conteneur, réglez le mode d'accès de Host Data sur Read/Write - Slave et redémarrez BombVault.
- **`key-not-loaded`** : un jeu de données chiffré dont la clé n'est pas chargée est ignoré. Chargez la clé avec `zfs load-key` et montez le jeu de données. La prochaine sauvegarde l'inclut.
- **`ssh-auth`** : le serveur a refusé la clé de BombVault. **Connexion ZFS à ce serveur**, ouverte avec **Voir la connexion** sur la page Sauvegardes, montre la commande qui l'autorise. Exécutez-la une fois sur le serveur.

## Un élément reste à « Apprentissage N/10 » {#an-entry-stays-at-learning-n10}

La plupart des contrôles d'anomalies démarrent après 10 sauvegardes réussies d'un élément, et le décompte repart après **Marquer comme attendue** et après un changement de la sélection de l'élément. Un élément qui n'est pas planifié n'apprend pas, et un conteneur sans appdata n'a rien dont apprendre, ce que dit son badge. **Progression de l'apprentissage** sur la page Anomalies montre un anneau de dix parts par élément.

## La rétention a cessé de supprimer les anciennes sauvegardes d'un élément {#retention-stopped-deleting-old-backups-of-one-entry}

Une anomalie critique ouverte les retient : la source de l'élément est presque vide, a fortement rétréci, ou une sauvegarde a stocké de nouveau la plus grande partie de ses données. Ouvrez l'anomalie depuis le badge de l'élément. Si des données manquent ou ont été chiffrées, restaurez d'abord depuis la dernière bonne sauvegarde indiquée par le lien. Accusez ensuite réception de l'anomalie, ou marquez-la comme attendue si le changement venait de vous, et la prochaine exécution élague comme d'habitude. L'aperçu de la rétention marque un tel élément comme conservé. Pour un élément ZFS, seul le jeu de données nommé dans l'anomalie garde ses anciennes sauvegardes, et les autres jeux de données de l'arborescence sont élagués comme d'habitude.

## L'élagage manuel dit que certains éléments ont été conservés {#manual-prune-says-some-entries-were-kept}

La même cause : l'élagage laisse tranquilles les anciennes sauvegardes d'un élément qui a une telle anomalie et nomme l'élément dans son message. Tout le reste est élagué comme d'habitude.

## Une sauvegarde échoue avec « on no mounted pool or share » {#no-mounted-pool}

Sur Unraid, `/mnt` réside en mémoire, si bien qu'un dossier qui s'y trouve sans pool monté dessus disparaît au redémarrage suivant et reste masqué une fois le pool revenu. BombVault refuse d'y écrire une sauvegarde ou un ZIP flash, même quand un dépôt d'une écriture précédente se trouve déjà dans ce dossier. Montez le pool et relancez la sauvegarde.

## L'import de l'historique dit qu'un dépôt n'a pas pu être lu {#history-import-says-a-repository-could-not-be-read}

Après la mise à niveau, BombVault lit une fois les tailles des sauvegardes antérieures dans chaque dépôt. Un dépôt qu'il n'a pas pu joindre à ce moment-là, comme une cible hors site hors service ou un partage qui n'était pas monté, est compté dans la carte **Anomalies** sous **Paramètres, Vérification** et retenté une fois par jour. Ses éléments apprennent entre-temps des nouvelles sauvegardes.

## L'avertissement d'espace disque ne correspond pas au tableau de bord Unraid {#the-disk-space-warning-does-not-match-the-unraid-dashboard}

Sur le partage utilisateur Unraid (`/mnt/user`), l'espace libre est celui de toute la grappe, pas d'un seul disque. Les destinations distantes que BombVault atteint via rclone, comme SFTP, la Hetzner Storage Box, SMB, WebDAV ou un lecteur cloud, sont mesurées quand l'autre côté indique son espace libre, et il en va de même pour une adresse `sftp:` saisie à la main. Les dépôts S3, B2 et REST n'ont pas de chiffre : leur destination indique **Taille inconnue**, et la carte **Anomalies** les liste comme non mesurés.

## Un assistant IA n'arrive pas à se connecter {#an-ai-assistant-cannot-connect}

La [page du serveur MCP](mcp.md#troubleshooting) liste ce que signifient chaque code d'état et chaque refus du point d'accès MCP et que faire.

## Le conteneur redémarre sans cesse ou semble en mauvaise santé {#the-container-keeps-restarting-or-looks-unhealthy}

BombVault signale sain ou en mauvaise santé d'après son propre `/api/health`. Un outil d'auto-réparation (comme Autoheal) peut le redémarrer automatiquement si le moteur se bloque un jour. Consultez le journal du conteneur et la carte **Intégration hôte** pour trouver la cause.

## Une destination refuse de s'enregistrer {#location-refused}

BombVault contrôle une destination avant de l'enregistrer, et le refus dit pourquoi en une phrase. Les cas habituels :

- **« This location lies inside another repository or target, or contains one. »** Un dépôt dans un dépôt casserait les deux. Le message nomme l'autre endroit. Choisissez un autre dossier, ou supprimez d'abord celui-là.
- **C'est un dépôt que vous avez créé.** La destination est l'une des lignes sous **Dépôts**, ou se trouve dedans ou autour. Choisissez un autre endroit, ou supprimez d'abord ce dépôt.
- **Le même endroit que le dossier propre d'une section.** Une copie vers le dossier propre d'une section copierait ses sauvegardes sur elles-mêmes à chaque exécution, elle est donc refusée. Choisissez un autre endroit, ou voir [Le dossier propre d'une section](offsite-recovery.md#own-folder).
- **« That is not an off-site target of this domain »** venait d'un lien vers une destination supprimée. Une version actuelle les efface au démarrage et dans un import. Si vous le voyez encore, mettez BombVault à jour.
- **Un chemin d'hôte complet** comme `/mnt/user/backups`. BombVault tourne dans un conteneur, il veut donc le chemin sous Host Data, `user/backups`, et le message nomme celui à saisir.

## Une destination distante dit que son dépôt a disparu {#repository-gone}

Une destination qui contenait des sauvegardes et ne contient soudain plus de dépôt n'en reçoit pas un nouveau et vide toute seule, car celui-là cacherait le vrai quand un disque reviendra. Un rest-server dont le disque ne s'est pas monté en est la cause habituelle, remontez donc le disque et relancez la sauvegarde ou la copie. Ce n'est que si vous avez vidé ou supprimé exprès l'ancien dépôt qu'il faut appuyer sur **Créer un nouveau dépôt ici** sur la page de la destination. Cela repart de zéro pour toutes les sections de cette destination à la fois.

## Une connexion cloud a expiré {#cloud-sign-in-expired}

Le test ou la copie suivante dit alors que la connexion a expiré. **Modifier la connexion** sur la page de la destination la refait.

- **Box, Jottacloud et d'autres avec des jetons de rafraîchissement à usage unique :** BombVault garde le jeton que rclone a rafraîchi, aussi après des redémarrages et dans le kit de récupération. Après une restauration depuis une ancienne sauvegarde des réglages, le jeton qu'elle contient peut avoir expiré, reconnectez-vous donc une fois. Le dépôt reste tel quel.
- **iCloud Drive :** Apple met fin à la connexion après 30 jours. Il faut un nouveau jeton de confiance et de nouveaux cookies depuis `rclone config`.
- **OneDrive :** une connexion avec plusieurs lecteurs demande lequel utiliser, si bien qu'une destination laissée sans choix reste inutilisable tant que vous n'en choisissez pas. Pour OneDrive Business et Google Drive, voir la remarque sur les données élaguées sous [Destinations](offsite-recovery.md#destinations).

## « La copie hors site de la dernière sauvegarde tourne encore » {#copy-still-running}

La sauvegarde précédente s'est terminée, et sa copie est encore en route, ce qui bloque une nouvelle sauvegarde de cette section. Attendez que l'en-tête d'état n'indique plus **Copie hors site en cours**. Les boutons qui lancent une sauvegarde disent la même chose pendant qu'ils attendent. La première copie vers une nouvelle destination peut prendre des heures, voir l'[exemple S3](offsite-recovery.md#example-s3).

## Une réplique ZFS s'arrête parce que la cible contient des instantanés plus récents {#replica-target-newer}

La cible contient des instantanés plus récents que le plus récent qu'elle partage avec ce serveur, par exemple parce que le jeu de données ici a été ramené en arrière ou que la copie sur la cible a été modifiée. La carte Réplique et l'historique des exécutions nomment l'instantané sous **Ramener la cible à**. Exécutez `zfs rollback -r` vers cet instantané sur la cible, pour chaque jeu de données que nomme l'exécution, et appuyez sur **Répliquer maintenant**. Voir [Combien de temps les instantanés restent](zfs-datasets.md#replica-retention).

## Une VM avec des instantanés Unraid échoue et nomme un fichier {#vm-chain}

BombVault suit la chaîne de fichiers de base de chaque disque et sauvegarde chaque fichier qui en fait partie. Une chaîne qu'on ne peut pas suivre jusqu'au bout fait échouer la sauvegarde plutôt que de laisser de côté une partie du disque, et le message nomme le fichier. Vérifiez que le fichier existe sous `/mnt` et que l'instantané est toujours listé dans le VM Manager. Un fichier en dehors de Host Data ne peut pas du tout être lu. Voir [VMs avec des instantanés Unraid](vm-backup-ssh-setup.md#unraid-snapshots).

## Les copies hors site attendent votre décision après une reconstruction {#copies-paused}

Après **Découvrir les sauvegardes** ou l'assistant d'urgence sans réglages restaurés, ou quand une section trouve ses propres sauvegardes datant d'avant cette base de données, les copies hors site de cette section attendent, car BombVault ne sait plus quels éléments vous avez laissés de côté. **À traiter** et la page de la destination posent la question. Appuyez sur **Les adopter pour ce serveur** pour continuer, ou sur **Vérifier d'abord les règles** pour regarder les règles de la section avant de le faire. Importer un fichier de réglages ne prend pas la décision à votre place. Voir [Après une reconstruction](offsite-recovery.md#after-a-rebuild).

## Les notifications n'arrivent pas {#notifications-do-not-arrive}

Commencez par **Envoyer un test** sous **Paramètres, Notifications**. La ligne sous la carte dit ce qui s'est passé.

- **Rien n'est envoyé du tout.** Vérifiez **Notifier**. Avec **Jamais**, aucun message ne part, seulement les pings d'une URL Healthchecks, et avec **Uniquement en cas d'échec**, une bonne exécution reste silencieuse. Avec **Résumer les exécutions planifiées** activé, une exécution planifiée envoie un résumé à sa fin.
- **Une nouvelle adresse.** Après un changement du serveur de Matrix, de Gotify ou du courriel, le champ redemande le jeton ou le mot de passe, et la nouvelle adresse n'est enregistrée qu'une fois que vous l'avez saisi. BombVault n'envoie pas l'ancien secret à un nouvel endroit.
- **Gotify** dit que le serveur a refusé le jeton de l'application : copiez-le de nouveau depuis l'application dans Gotify. L'**URL du serveur** est l'adresse de base, sans `/message`.
- **Webhook :** vérifiez que **Format du Webhook** correspond au service. Une adresse Discord demande **Discord**, et ntfy demande **ntfy**.
- Les **Notifications Unraid** passent par la connexion SSH de l'hôte, testez-la donc d'abord sous **Paramètres, Intégrations**. Sur une autre plateforme, elles restent désactivées, et la carte le dit.
- **Un courriel de notification n'arrive jamais.** Vérifiez **Chiffrement** par rapport au port, en général **STARTTLS** sur 587 ou **TLS (implicit)** sur 465.

Les pings Healthchecks ne dépendent d'aucun de ces points, voir [Healthchecks](notifications.md#healthchecks).

## L'application Android n'arrive pas à ouvrir un serveur {#android-cannot-open}

L'écran d'accueil dit pourquoi sur la carte du serveur :

- **Non connecté :** ni l'adresse du serveur ni le relais ne répondent. Avec **Aucun relais**, l'application n'atteint un serveur qu'à son adresse propre.
- **Vérifier le certificat :** le serveur présente un autre certificat que celui que vous aviez approuvé. L'application ne l'ouvre qu'après que vous avez approuvé le nouveau, assurez-vous donc que le changement vient de vous.
- **Connexion nécessaire :** un serveur ajouté par son adresse demande le mot de passe. Voir aussi [Un téléphone arrive sur la page de connexion](#a-phone-lands-on-the-login-page).
- **La clé a changé :** l'écran d'accueil montre l'empreinte de la nouvelle clé, et la page ne s'ouvre qu'après que vous l'avez approuvée. Voir [Hors de chez soi](android.md#away).

## Un autre utilisateur ne peut pas lire le dépôt local {#repository-permissions}

Le dépôt local reste lisible pour d'autres utilisateurs, comme un utilisateur de synchronisation via SMB. Avant la 10.4.1, des fichiers qu'écrivaient un élagage, une étiquette, un rapatriement, un déplacement de section, une suppression ou une sauvegarde de stack pouvaient rester illisibles jusqu'à la nuit suivante. Mettez BombVault à jour, et ils sont lisibles tout de suite. Le dépôt de l'auto-sauvegarde reste lisible par root seulement, exprès, car il contient les réglages de BombVault.

## Toujours bloqué ? {#still-stuck}

- Lisez les pages complètes [Configuration](configuration.md) et [Hors site et récupération](offsite-recovery.md).
- **Rapport de bug**, dans la carte **Aide** sous **Paramètres, Info**, télécharge un ZIP pour un ticket GitHub : le contrôle de l'hôte, votre configuration sans aucun secret, les dernières exécutions, ce qui est planifié ensuite et le journal récent depuis le dernier démarrage du conteneur. Il contient aussi le dernier dump de chaque base de données, les éléments ZFS avec leurs montages, les anomalies ouvertes et le nombre de clés MCP existantes. Les mots de passe, les jetons et la configuration rclone sont retirés, et le paquet le dit dans son propre manifeste. Il demande un mot de passe de connexion. Pour un plantage qui a fait redémarrer le conteneur, regardez `docker logs`.
- Demandez sur le [fil de support Unraid](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/).
- Ouvrez un [ticket GitHub](https://github.com/junkerderprovinz/bombvault/issues).
