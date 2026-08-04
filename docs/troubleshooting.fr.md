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

## Le conteneur redémarre sans cesse ou semble non sain

BombVault se signale sain/non sain depuis son propre `/api/health`. Un outil d'auto-réparation (comme Autoheal) peut le redémarrer automatiquement si le moteur venait à se coincer. Vérifiez le journal du conteneur et le rapport `/spike` pour la cause sous-jacente.

## Toujours bloqué ?

- Lisez les pages complètes [Configuration](configuration.md) et [Sauvegarde hors site et récupération](offsite-recovery.md).
- Demandez sur le [fil de support Unraid](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/).
- Ouvrez un [ticket GitHub](https://github.com/junkerderprovinz/bombvault/issues).
