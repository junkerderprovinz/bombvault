# Sauvegardes de VM

BombVault sauvegarde les VMs KVM/libvirt sans monter aucun chemin libvirt. Il exécute `virsh` sur l'hôte via SSH (`qemu+ssh://`), lit et écrit les disques des VMs par le montage Host Data (`/mnt` sur l'hôte est `/host/user` dans le conteneur) et copie la NVRAM UEFI via SSH. Comme rien de ce qui appartient à libvirt n'est monté en bind, BombVault ne peut jamais gêner le VM Manager d'Unraid.

Ce guide couvre la configuration, les méthodes de sauvegarde, les VMs avec des instantanés Unraid et TrueNAS SCALE. Les sauvegardes de conteneurs n'ont besoin de rien de tout cela.

Les sauvegardes de jeux de données ZFS utilisent le même lien SSH, avec la même clé, le même hôte et le même utilisateur. Dès que les sauvegardes de VM se connectent, les éléments de jeux de données ZFS se connectent aussi. Voir [Jeux de données ZFS](zfs-datasets.md).

## Ce que vous configurez {#what-you-configure}

| Réglage | Où | Par défaut | Signification |
|---|---|---|---|
| **Host SSH: Address** (`LIBVIRT_HOST`) | variable du modèle | `host.docker.internal` | L'hôte Unraid, joint via SSH |
| **Host SSH: Port** (`LIBVIRT_SSH_PORT`) | variable du modèle | `22` | Le port SSH d'Unraid |
| **Host SSH: User** (`LIBVIRT_SSH_USER`) | variable du modèle | `root` | L'utilisateur SSH sur l'hôte |
| Clé publique | **Paramètres, Intégrations, SSH de l'hôte** | créée au premier démarrage | À autoriser sur l'hôte |

BombVault crée sa paire de clés SSH au premier démarrage dans `/config/ssh/id_ed25519`, conservée dans appdata. La clé de l'hôte est épinglée dans `/config/ssh/known_hosts` à la première connexion.

Le modèle remplit **Host SSH: Address** avec l'espace réservé `192.168.x.x`. BombVault le traite comme non défini et essaie `host.docker.internal`, qui fonctionne sur le réseau `bridge` par défaut. Sur un réseau personnalisé, remplacez-le par l'IP de l'hôte sur le réseau local (étape 3 B).

## Étape 1 : activer SSH sur l'hôte Unraid {#step-1}

1. Dans Unraid, ouvrez **Settings, Management Access** et réglez **Use SSH** sur **Yes**.
2. Si vous avez changé le port SSH, notez-le. **Host SSH: Port** doit correspondre.

## Étape 2 : autoriser la clé publique de BombVault {#step-2}

Le `/root` d'Unraid vit en mémoire et est effacé à chaque démarrage, la clé doit donc aussi se trouver sur la clé flash. Exécutez ceci dans le terminal Unraid :

```sh
K=$(docker exec BombVault cat /config/ssh/id_ed25519.pub)
mkdir -p /root/.ssh /boot/config/ssh && chmod 700 /root/.ssh
grep -qxF "$K" /root/.ssh/authorized_keys 2>/dev/null  || echo "$K" >> /root/.ssh/authorized_keys
grep -qxF "$K" /boot/config/ssh/root.pubkeys 2>/dev/null || echo "$K" >> /boot/config/ssh/root.pubkeys
chmod 600 /root/.ssh/authorized_keys
```

Unraid 6.9 et plus récent copie `/boot/config/ssh/root.pubkeys` dans l'`authorized_keys` de root à chaque démarrage, ce qui survit donc aux redémarrages. Cela vaut pour toute connexion `root`, quels que soient le nom d'hôte ou le port que BombVault utilise.

## Étape 3 : réseau {#step-3}

BombVault doit ouvrir une connexion TCP du conteneur vers le port SSH de l'hôte. Choisissez le cas qui correspond à votre installation.

### A. BombVault sur le réseau bridge par défaut {#bridge}

- Laissez **Host SSH: Address** sur `host.docker.internal` ou sur l'espace réservé du modèle. Le modèle ajoute `--add-host=host.docker.internal:host-gateway`, qui pointe vers la passerelle docker0, c'est-à-dire l'hôte.
- Réglez **Host SSH: Port** sur votre port SSH.

### B. BombVault sur un réseau personnalisé {#custom-network}

Un conteneur sur `br0.x`, macvlan ou ipvlan ne peut pas joindre l'hôte par `host.docker.internal`, car 172.17.0.1 est docker0 et n'est pas joignable depuis `br0.x`. À la place :

1. Dans Unraid, réglez **Settings, Docker, Host access to custom networks** sur **Enabled**.
2. Réglez **Host SSH: Address** sur l'IP de l'hôte Unraid sur le réseau local, l'adresse avec laquelle vous ouvrez l'interface web.
3. Si le réseau du conteneur et l'hôte sont sur des VLAN différents, autorisez la route sur votre routeur ou votre pare-feu : du VLAN du conteneur vers l'IP de l'hôte sur le port SSH, en TCP.
4. Si l'IP de l'hôte sur le réseau local reste injoignable, utilisez l'IP shim de l'hôte sur le sous-réseau du conteneur. Trouvez-la sur l'hôte avec `ip -4 addr show | grep -B2 <container-subnet>`.

### Vérifier que l'hôte répond {#verify-reachability}

Exécutez ceci dans le terminal Unraid, avec votre IP et votre port :

```sh
docker exec BombVault timeout 6 bash -c 'echo > /dev/tcp/192.168.1.10/22' && echo OPEN || echo UNREACHABLE
```

`OPEN` veut dire que le chemin fonctionne. `UNREACHABLE` veut dire que le réseau de l'étape 3 doit être corrigé avant tout le reste.

## Étape 4 : régler les variables et tester {#step-4}

1. Dans Unraid, ouvrez **Docker**, cliquez sur BombVault, choisissez **Edit**, réglez **Host SSH: Address** (et **Host SSH: Port** s'il n'est pas 22) et appuyez sur **Apply**. Si les variables manquent, importez de nouveau le modèle, car Unraid garde la configuration enregistrée d'un conteneur existant.
2. Dans BombVault, ouvrez **Paramètres, Intégrations, SSH de l'hôte** et appuyez sur **Tester la connexion**. Il devrait passer au vert.

Le même appel depuis le terminal liste vos VMs :

```sh
docker exec BombVault virsh -c "qemu+ssh://root@192.168.1.10:22/system?keyfile=/config/ssh/id_ed25519&known_hosts=/config/ssh/known_hosts&known_hosts_verify=auto" list --all
```

## Étape 5 : la méthode par VM {#per-vm-method}

Chaque VM a une **Méthode** dans la carte **VM** de son onglet Paramètres (ouvrez la VM sous **Sauvegardes**) :

- **Propre (arrêt)** est le réglage par défaut. Elle arrête la VM, sauvegarde les disques et la redémarre, la sauvegarde est donc toujours cohérente. Elle attend jusqu'à dix minutes que l'invité s'arrête et ne force jamais l'extinction. Un invité qui tourne encore après cela est laissé tranquille, et cette sauvegarde échoue avec la raison. Si l'invité s'éteint dans l'heure qui suit, BombVault le redémarre. Si ce démarrage échoue, l'exécution en échec le signale.
- **Instantané à chaud** sauvegarde une VM en marche via un instantané externe et `blockcommit`. Il faut le disque sur `/mnt/cache` ou `/mnt/diskX`, pas sur `/mnt/user`. Avec l'agent invité qemu dans la VM, l'instantané est figé, et sans lui, il est cohérent en cas de plantage. Sur une VM éteinte, le mode à chaud retombe tout seul sur le mode propre. Quand l'instantané ne peut pas être fait, cette sauvegarde échoue et la VM continue de tourner, car une VM que vous avez choisi de sauvegarder à chaud n'est jamais arrêtée pour cela.

## Ce qu'une sauvegarde laisse de côté {#left-out}

Une sauvegarde peut réussir sans contenir toute la VM. L'exécution porte alors une note sur un ton d'avertissement qui nomme ce qui manque :

- **Les disques que BombVault ne lit pas.** Un disque en lecture seule, un `lun` et un disque `network` ou `volume` sont laissés de côté, et la note nomme chacun avec son périphérique cible et son type. La définition dans la sauvegarde les liste toujours. Un cdrom et une disquette ne sont pas nommés. Un périphérique bloc ordinaire qui n'est pas un zvol ZFS fait au contraire échouer la sauvegarde.
- **L'état d'un TPM émulé.** Il n'a pas de chemin dans le XML du domaine. Le VM Manager d'Unraid configure un tel TPM pour une VM avec le BIOS **OVMF TPM**, la note apparaît donc aussi sur Unraid. Un invité qui garde un secret dans son TPM, comme peut le faire BitLocker, peut demander sa clé de récupération après une restauration qui perd cet état. Gardez donc cette clé sous la main.

## Disques à allocation fine {#thin-disks}

Une restauration conserve l'allocation fine d'un disque, qu'elle remplace le disque ou en écrive un nouveau : restic écrit les suites de zéros comme des trous dans un nouveau fichier, et BombVault libère les blocs de zéros d'un fichier remplacé avec `fallocate --dig-holes` une fois qu'il est écrit. Pendant que restic remplace un fichier, ce fichier occupe sa taille complète, si le pool a la place. Un système de fichiers qui ne sait pas percer de trous garde le disque à sa taille complète, et le journal le signale. Le contrôle de place compte toujours un nouveau disque à sa taille complète.

## Une restauration qui échoue {#restore-fails}

Une restauration sur place éteint la VM, écrit les disques, et ne remplace la définition et la NVRAM qu'une fois les disques écrits. Une restauration qui échoue laisse la VM définie avec sa NVRAM. Si ses disques n'ont pas changé et qu'elle tournait, elle tourne de nouveau. S'ils ont pu changer, elle reste éteinte, car démarrer sur un disque à moitié écrit écrirait par-dessus, et l'exécution le signale. Restaurer de nouveau la même sauvegarde termine le travail.

Des instantanés enregistrés dans libvirt, ce qui n'est pas la façon dont le VM Manager d'Unraid les crée, arrêtent une restauration sur place avant que la VM soit touchée, car libvirt ne peut pas supprimer une VM qui en a. Supprimez-les d'abord.

## Blocs modifiés uniquement {#changed-blocks}

Une VM avec des disques qcow2 peut être sauvegardée en ne lisant que ce qui a changé. Activez **Blocs modifiés uniquement** dans la même carte, et BombVault utilise l'API de sauvegarde de libvirt au lieu de la méthode ci-dessus :

- Il démarre une tâche de sauvegarde en mode pull (`virsh backup-begin`) qui crée au même instant un point de contrôle nommé `bombvault-<time>`, et lit les disques via le lien SSH par un socket Unix redirigé. Quand l'agent invité répond, les systèmes de fichiers de l'invité ne sont gelés que pendant le démarrage de la tâche. Un dégel qui échoue est retenté pendant environ quinze secondes, et si l'invité ne dégèle toujours pas, BombVault envoie une notification.
- La première exécution lit tous les blocs alloués. Les suivantes ne lisent que les blocs que le bitmap du point de contrôle marque comme écrits depuis la sauvegarde précédente.
- Chaque disque est stocké comme un dossier de segments de taille fixe qui contiennent ce que voit l'invité. Les segments inchangés sont repris de l'instantané précédent sans être lus, si bien que chaque instantané est complet et se restaure seul.
- Une restauration reconstruit le disque à partir de ses segments et le convertit de nouveau en qcow2 avec `qemu-img`. Le disque a le même contenu qu'au moment de la sauvegarde, mais le fichier est un nouveau fichier qcow2, et une chaîne de fichiers de base revient aplatie.
- Le contrôle avant une telle restauration liste chaque disque qu'elle reconstruit et le compte à la pleine taille de l'invité, deux fois pour qcow2, puisque l'image brute et sa copie convertie coexistent jusqu'au remplacement de l'ancien fichier. C'est le maximum qu'un disque peut prendre, si bien qu'un manque selon ce calcul laisse la ligne d'espace en gris au lieu de bloquer. Le contrôle de restauration après la première sauvegarde, et **Vérifier la restauration**, s'assurent aussi que l'instantané contient chaque segment de chaque disque à la taille voulue.
- Entre deux exécutions, la VM n'a aucun point de contrôle enregistré dans libvirt. BombVault garde la définition du point de contrôle `bombvault-` le plus récent sous `/config/vm-checkpoints` et ne retire que l'enregistrement qu'en fait libvirt (`virsh checkpoint-delete --metadata`). Son bitmap reste dans le fichier qcow2 et continue de compter les écritures, et l'exécution suivante remet le point de contrôle en place (`virsh checkpoint-create --redefine`) avant de démarrer. libvirt refuse de supprimer la définition d'une VM qui a des points de contrôle, c'est donc ce qui permet à **Remove VM** d'Unraid et à une restauration de fonctionner comme d'habitude.
- Une tâche de sauvegarde que BombVault a laissée en marche, parce qu'il s'est arrêté au milieu d'une sauvegarde, est terminée au redémarrage de BombVault, avec les points de contrôle que cette exécution a laissés enregistrés.
- Désactiver l'interrupteur, sortir la VM du planning ou supprimer ses sauvegardes termine aussi une telle tâche, et supprime en plus le bitmap tant que la VM tourne. Sur une VM éteinte, libvirt ne peut pas le supprimer, et il reste inutilisé dans l'image.
- Les points de contrôle d'autres outils sont laissés tranquilles. Une VM qui en a ne peut pas être restaurée tant qu'ils ne sont pas supprimés, et la restauration s'arrête avant de toucher à la VM.

L'interrupteur se replie tout seul. Un point de contrôle manquant ou cassé, un disque dont la taille a changé, ou un instantané le plus récent qui n'est pas un instantané de blocs modifiés veut dire que tout le disque est relu une fois. Une VM éteinte, un disque brut ou zvol, pas de lien SSH vers l'hôte, un libvirt sans l'API de sauvegarde, ou trop peu de place libre sous `/config` veut dire la sauvegarde habituelle avec la méthode ci-dessus. La carte dit lequel de ces cas la dernière exécution a utilisé et pourquoi.

Prérequis : libvirt avec l'API de sauvegarde (testé avec libvirt 12.2 et QEMU 10.2 sur Unraid 7.3), sshd avec `AllowStreamLocalForwarding yes` (le réglage par défaut, aussi sur Unraid, qui désactive la redirection TCP), et des disques qcow2. libvirt refuse les points de contrôle sur les disques bruts, et Unraid crée des vdisks bruts par défaut. La VM reste en marche pendant la sauvegarde, comme avec le mode à chaud, et les segments qui attendent restic prennent jusqu'à 4 GiB sous `/config`.

## VMs avec des instantanés Unraid {#unraid-snapshots}

Un instantané que prend le VM Manager d'Unraid transforme un disque qcow2 en chaîne : l'image d'origine en bas, et par-dessus un fichier par instantané, chacun ne contenant que ce qui a changé depuis celui du dessous. La VM écrit dans le fichier le plus récent. Un disque avec deux instantanés fait trois fichiers.

BombVault sauvegarde une telle VM en entier :

- **Chaque fichier de la chaîne.** Il suit la chaîne de fichiers de base de chaque disque jusqu'au bout et sauvegarde chaque fichier qui en fait partie, avec la liste des instantanés d'Unraid (`/etc/libvirt/qemu/snapshotdb/<vm>/snapshots.db`, lue via SSH). Une chaîne qu'on ne peut pas suivre jusqu'au bout, par exemple parce qu'un fichier du milieu a disparu, fait échouer la sauvegarde et nomme le fichier.
- **Une sauvegarde à chaud garde les instantanés.** **Instantané à chaud** pose une surcouche temporaire par-dessus pendant qu'il lit le disque et la fusionne ensuite, seulement dans l'image juste en dessous. La chaîne est la même après la sauvegarde qu'avant. Une VM sans instantanés se comporte comme toujours.
- **Une restauration ramène la chaîne.** Une restauration sur place réécrit chaque fichier là où il était et remet la liste des instantanés d'Unraid, si bien que le VM Manager liste de nouveau les instantanés. Une restauration à côté de l'original écrit les fichiers dans le dossier de la copie et fait pointer les disques restaurés vers les fichiers restaurés, si bien que la copie ne lit jamais les images de l'original. La liste des instantanés d'Unraid ne revient qu'avec une restauration sur place, car elle nomme les fichiers là où ils ont été sauvegardés. Sans elle, la copie démarre pareil, et le VM Manager ne liste simplement aucun instantané pour elle.
- **Blocs modifiés uniquement** lit ce que voit l'invité, sa restauration revient donc comme un seul fichier qcow2 aplati, sans les instantanés.
- **Une restauration ramène le disque de la sauvegarde que vous choisissez.** Les instantanés pris ou fusionnés depuis cette sauvegarde n'y changent rien. Une sauvegarde d'avant un instantané définit la VM sur le fichier qu'elle contient, et une sauvegarde d'avant une fusion ramène sa surcouche et tourne dessus. Les fichiers plus récents restent où ils sont, inutilisés et inchangés. Une sauvegarde qui contient une surcouche mais pas le fichier en dessous est refusée avant que la VM soit touchée. Après une restauration, la définition de la VM ne liste aucune chaîne, si bien que libvirt lit la chaîne à partir des fichiers restaurés.

### Exemple : une VM avec deux instantanés {#example-unraid-snapshots}

La VM `homeassistant` a un disque, `vdisk1.qcow2`, et deux instantanés que vous avez pris dans le VM Manager avant deux mises à jour. Son disque est maintenant une chaîne de trois fichiers. Elle tourne en permanence, elle utilise donc **Instantané à chaud**.

1. Ouvrez **Sauvegardes**, puis `homeassistant`, puis son onglet **Paramètres**. Dans la carte **VM**, vérifiez que **Méthode** est **Instantané à chaud** et que **Blocs modifiés uniquement** est désactivé.
2. Appuyez sur **Sauvegarder maintenant** dans l'en-tête de la page.
3. Quand l'exécution est terminée, sa ligne dans **Exécutions récentes** sur l'onglet **Vérification** est verte. La VM est restée en marche tout du long, et la sauvegarde contient les trois fichiers du disque et la liste des instantanés d'Unraid.
4. Dans le VM Manager, les deux instantanés sont toujours là, et le disque lit toujours à travers les trois mêmes fichiers.

Un mois plus tard, une mise à jour tourne mal et vous voulez retrouver l'état d'hier à côté de l'état cassé, pour comparer :

1. Ouvrez **Restauration**, choisissez **VMs** et cliquez sur `homeassistant`.
2. Gardez la sauvegarde recommandée de la nuit dernière et choisissez **À côté de l'original** sous **Où**.
3. Appuyez sur **Restaurer**. Les trois fichiers atterrissent dans un dossier nommé d'après la VM avec `-restored`, et le disque restauré pointe vers les fichiers restaurés. Rien ne change sur la VM en marche.

Pour faire revenir toute la VM en arrière à la place, choisissez **Remplacer l'original** avec **Sauvegarder d'abord l'état actuel** activé. BombVault sauvegarde l'état cassé, arrête la VM, réécrit les trois fichiers et la liste des instantanés d'Unraid, et la redémarre.

## TrueNAS SCALE {#truenas-scale}

Les étapes ci-dessus sont écrites pour Unraid, mais le même lien `qemu+ssh://` fonctionne contre n'importe quel libvirtd que le conteneur peut joindre, TrueNAS SCALE compris, qui a de nouveau des VMs sur libvirt depuis la 25.04.2. Sur TrueNAS, la chaîne de connexion demande un remplacement et la clé va sur root dans les propres réglages utilisateur de TrueNAS. Trois autres choses changent ce qu'une sauvegarde contient : les disques sur zvols, l'état du vTPM et les noms que TrueNAS donne à ses VMs.

### 1. Le socket libvirt {#truenas-socket}

Le libvirtd de TrueNAS n'écoute pas là où l'attend la chaîne de connexion construite à partir des champs **SSH de l'hôte**. Réglez plutôt `LIBVIRT_URI`, avec le paramètre `?socket=` en plus :

```sh
LIBVIRT_URI=qemu+ssh://<user>@<truenas-host>/system?socket=/run/truenas_libvirt/libvirt-sock
```

BombVault utilise `LIBVIRT_URI` exactement comme écrit, et `LIBVIRT_HOST`, `LIBVIRT_SSH_USER` et `LIBVIRT_SSH_PORT` ne sont pas nécessaires. Pour chacune qui n'est pas définie, BombVault prend l'hôte, l'utilisateur et le port de l'URI aussi pour ses propres commandes SSH, si bien que la copie de la NVRAM et les sauvegardes de jeux de données ZFS atteignent la même machine que `virsh`. Une variable que vous définissez vous-même l'emporte toujours, et le journal de démarrage nomme la cible prise dans l'URI. Sur TrueNAS, un utilisateur autre que root a aussi besoin de `zfs allow <user> snapshot,destroy,mount <dataset>` pour les sauvegardes de jeux de données ZFS, voir [Jeux de données ZFS](zfs-datasets.md#truenas).

Mettez l'adresse de TrueNAS sur le réseau local dans l'URI. Sans `LIBVIRT_URI` ni `LIBVIRT_HOST`, BombVault prend la passerelle par défaut de l'application comme hôte, car `host.docker.internal` ne se résout pas dans une application TrueNAS, et le journal de démarrage nomme l'adresse retenue. Cela suffit pour les sauvegardes de jeux de données ZFS. Les sauvegardes de VM ont toujours besoin de l'URI pour le socket.

L'application a besoin de la capacité `SETPCAP` pour les sauvegardes de VM, à côté de `CHOWN`, `DAC_OVERRIDE`, `FOWNER`, `SETGID` et `SETUID`. Sans elle, libvirt s'arrête avec « cannot apply process capabilities ». Le YAML sous [TrueNAS SCALE](getting-started.md#truenas) les règle toutes, avec le `SYS_CHROOT` dont a besoin l'instantané plutôt que l'arrêt.

Sur TrueNAS SCALE 25.10.0, le chemin du socket a été vérifié sur du vrai matériel : le `qemu:///system` standard échoue avec `Failed to connect socket to '/var/run/libvirt/libvirt-sock': No such file or directory`, et le même appel avec `?socket=/run/truenas_libvirt/libvirt-sock` liste les domaines.

### 2. La clé de BombVault sur TrueNAS {#truenas-key}

L'étape 2 ci-dessus écrit sur la clé flash d'Unraid et ne s'applique pas ici. Sur TrueNAS :

1. Activez le service SSH sous **System, Services**.
2. Sous **Credentials, Users**, modifiez **root** et collez la clé publique de BombVault de **Paramètres, Intégrations, SSH de l'hôte** comme clé publique SSH. TrueNAS la conserve après les redémarrages et les mises à jour.

Les installations neuves de TrueNAS 24.10 et plus récent ne donnent aucun mot de passe à root, et la connexion par clé n'en demande pas. Si vous préférez ne pas utiliser root, autorisez la clé pour un compte administrateur avec sudo et mettez son nom à la place de `root` dans `LIBVIRT_URI`. Les sauvegardes de jeux de données ZFS demandent alors `zfs allow` comme décrit plus haut.

Une réplique ZFS vers un second serveur se connecte avec une clé à elle, que montre la boîte de dialogue de la réplique. La clé SSH de l'hôte n'autorise pas une réplique, et la clé de la réplique n'autorise pas les sauvegardes de VM.

!!! warning "TrueNAS ne voit pas les changements faits avec virsh"
    L'intergiciel de TrueNAS, la couche derrière ses écrans de VM, est censé être le seul moyen de modifier ses VMs. Une commande `virsh` via ce lien SSH, ce qu'exécutent la sauvegarde et la restauration de VM de BombVault, le contourne, et TrueNAS peut plus tard réconcilier ou écraser un état qu'il n'a pas créé. Cela vient du comportement documenté de TrueNAS et n'a pas été observé sur une vraie machine. C'est le même genre de risque que le VM Manager d'Unraid qui réconcilie des changements faits en dehors de lui, en plus aigu sur TrueNAS.

### 3. Disques sur zvols {#truenas-zvols}

Les disques de VM TrueNAS sont souvent des zvols, des périphériques bloc ZFS à `/dev/zvol/<pool>/<dataset>`, pas des fichiers qcow2. restic ne peut pas sauvegarder un périphérique bloc par son chemin comme il sauvegarde un fichier, BombVault reconnaît donc un disque zvol dans le XML du domaine (`<source dev="...">` au lieu de `<source file="...">`) et le traite via le même lien SSH. Le conteneur BombVault n'a besoin d'aucun outil ZFS.

Une sauvegarde de zvol :

1. `zfs snapshot <dataset>@bombvault-vm-<time>` fixe un instant, aussi pendant que la VM tourne.
2. `zfs clone -o volmode=dev -o readonly=on` rend cet instantané lisible comme périphérique. `volmode=dev` évite que l'hôte analyse les partitions de l'invité.
3. `dd` lit le périphérique, et l'image brute entre dans restic comme un seul fichier, `/vm-disks/<dataset>.img`, avec l'étiquette `vm:<name>:zvol:<dev>`. restic la découpe en morceaux d'après le contenu, si bien que les blocs qui n'ont pas changé depuis la dernière sauvegarde ne sont pas stockés de nouveau.
4. Le clone et l'instantané sont détruits ensuite, que la sauvegarde ait fonctionné ou non. Si BombVault s'est arrêté au milieu d'une sauvegarde, la sauvegarde suivante de ce zvol supprime d'abord l'instantané `bombvault-vm-<time>` et son clone `<dataset>-bombvault-vm-<time>`. Elle laisse tous les autres instantanés tranquilles, et garde celui dont autre chose a été cloné. Le journal nomme ce qu'elle a supprimé ou gardé.

BombVault n'utilise pas `zfs send` pour cela. Chaque enregistrement d'un flux d'envoi porte le GUID de l'instantané et une somme de contrôle courante, restic ne trouverait donc rien de la dernière sauvegarde à réutiliser et stockerait tout le disque de nouveau à chaque fois. Mesuré sur TrueNAS SCALE 25.10 avec un zvol de 512 MiB contenant 200 MiB de données : la première sauvegarde a ajouté 211 MB, une deuxième sans rien de changé 52 KB, et une après 1 MiB de changement 3,6 MB. Le même zvol via `zfs send` a ajouté 214 MB à chaque exécution.

Chaque exécution écrit aussi un petit instantané étiqueté `vm:<name>` avec le XML du domaine, une fois tous les disques entrés. Une VM dont tous les disques sont des zvols n'a aucun instantané de fichier, et c'est celui-ci qui liste l'exécution sous la VM et la rend restaurable.

Une restauration réécrit chaque zvol sur place, comme un disque fichier est réécrit dans son fichier, si bien que la VM restaurée démarre sur les données restaurées :

1. Avant que quoi que ce soit soit touché, BombVault vérifie que l'exécution contient une image de chaque disque zvol. Une exécution qui en manque une est refusée et la VM est laissée telle quelle.
2. Un zvol existant reçoit d'abord un instantané, `<dataset>@bombvault-before-restore-<time>`, avec l'heure en UTC comme chaque nom d'instantané que fait BombVault, si bien qu'elle peut avoir des heures d'écart avec l'horloge locale. Le journal le nomme, et `zfs rollback` vers lui annule la restauration tant qu'il est l'instantané le plus récent du zvol. Dès qu'un plus récent existe, comme celui de la restauration suivante, `zfs rollback -r` le fait et détruit les instantanés plus récents. Il reste jusqu'à ce que vous le supprimiez avec `zfs destroy` une fois que la VM restaurée fonctionne.
3. Un zvol qui n'existe plus est créé à la taille qu'il avait lors de la sauvegarde, et un zvol existant d'une autre taille est ramené à cette taille.
4. `dd` écrit l'image sur `/dev/zvol/<dataset>`. Avec la compression activée, la valeur par défaut de TrueNAS, les parties vides de l'image reviennent comme des trous et ne prennent pas de place.

**Une restauration vers une autre instance demande un pool de destination.** Elle déplace les disques fichiers vers un dossier de destination, mais un dossier ne nomme aucun pool ZFS, si bien que sans pool BombVault refuse la restauration avant qu'elle démarre. Passez le pool comme paramètre `zvolPool` de `POST /api/foreign/restore` : chaque zvol garde son chemin sous le pool (`tank/vms/win10/disk0` vers `flashpool` devient `flashpool/vms/win10/disk0`), et le XML du domaine pointe vers lui. La fenêtre **Depuis un ancien dossier de sauvegarde** de la page Restauration n'a pas de champ pour cela, il n'est donc accessible que par un appel direct à l'API. Une restauration sur la même instance réécrit chaque zvol sur lui-même.

**L'export en fichiers laisse les zvols de côté.** Une VM dont tous les disques sont des zvols est refusée avec cette raison, restaurez-la donc depuis une sauvegarde. Pour une VM qui a aussi des disques fichiers, l'export contient les disques fichiers et une remarque qui nomme les zvols laissés de côté.

Cela a été essayé sur TrueNAS SCALE 25.10 avec ZFS 2.3.4 : trois sauvegardes d'un zvol de test, une restauration sur place après que le zvol a été écrasé et agrandi, et une restauration sur un zvol qui avait été supprimé. Chaque restauration relue avait le même SHA-256 que la source, le zvol avait de nouveau son ancienne taille, et aucun instantané ni clone n'a été laissé. Une VM entière dont le seul disque est un zvol a été sauvegardée en marche, avec le mode propre et avec le mode à chaud, et éteinte, et une restauration de VM entière (suppression de la définition, écriture des zvols, définition, démarrage) s'est aussi déroulée là : le zvol relu avait la somme de contrôle de la source et la VM a démarré.

Les disques fichiers, tels que les utilise Unraid, ne sont pas concernés par tout cela.

### 4. État du vTPM {#truenas-vtpm}

Une VM peut avoir un TPM virtuel, dont ont besoin le Secure Boot et les invités Windows 11. Son état vit en dehors du disque et de la NVRAM, si bien qu'une sauvegarde de ces deux seuls éléments ramènerait un invité avec un nouveau TPM vide.

- **Un TPM en passthrough**, une vraie puce passée à l'invité, nomme son chemin dans le XML du domaine. BombVault enregistre son état à côté de la NVRAM via SSH et le réécrit avant `virsh define`, au mieux comme la NVRAM : une lecture ou une écriture qui échoue est consignée dans le journal et ne fait pas échouer la sauvegarde ou la restauration.
- **Un vTPM émulé**, celui que TrueNAS configure pour les invités Windows 11 et Secure Boot, ne publie aucun chemin d'état dans le XML du domaine, son état n'est donc pas sauvegardé. BombVault ne devine pas de chemin, et la sauvegarde se termine avec une note indiquant que l'état du TPM de la VM n'y figure pas. Gardez sous la main la clé de récupération d'un tel invité, comme sa clé BitLocker, avant de le restaurer.
- **Vous ne pouvez pas attacher vous-même un TPM à une VM TrueNAS.** L'API de périphériques de VM de TrueNAS SCALE 25.10 n'accepte que `CDROM`, `DISPLAY`, `NIC`, `PCI`, `RAW`, `DISK` et `USB`, et refuse `TPM` comme type inconnu.

La gestion du TPM n'est testée qu'en tests unitaires et n'a pas tourné sur un vrai invité TrueNAS avec un vTPM. Une VM sans TPM se sauvegarde et se restaure exactement comme avant.

La NVRAM est capturée comme d'habitude. TrueNAS la garde par VM à `/var/db/system/vm/nvram/<id>_<name>_VARS.fd`, et le XML du domaine nomme ce chemin.

### 5. Noms de VM sur TrueNAS {#truenas-names}

Sur Unraid, le nom libvirt d'une VM est le nom que vous avez choisi. TrueNAS fabrique le nom libvirt, et a changé deux fois la façon de faire :

| Version de TrueNAS | Nom libvirt | Où se trouve votre nom |
|---|---|---|
| 25.10 « Goldeye » | `{id}_{name}`, comme `1_debian` | dans le nom libvirt, derrière le `{id}_` |
| 26 | l'UUID de la VM, comme `550e8400-e29b-41d4-a716-446655440000` | seulement dans le `<title>` du XML du domaine |

Sur un hôte que BombVault détecte comme TrueNAS, ou que `PLATFORM=truenas` désigne, la liste Sauvegardes montre votre nom : `debian` au lieu de `1_debian` sur la 25.10, et le `<title>` au lieu de l'UUID sur la 26, ou l'UUID quand il n'y a pas de titre. Ce n'est que ce qu'affiche la liste. Chaque appel `virsh` et les réglages par VM utilisent toujours le nom libvirt, si bien que la sauvegarde et la restauration agissent toujours sur la bonne VM. Sur TrueNAS 26, cela veut dire qu'une VM que TrueNAS recrée avec un nouvel UUID apparaît comme un nouvel élément sans ses réglages, même si son nom a l'air identique. Sur Unraid, rien ne change, y compris pour une VM que vous avez nommée `10_Windows`.

Le schéma de la 25.10 a été vu sur une vraie machine TrueNAS SCALE 25.10. Le schéma de TrueNAS 26 vient de la documentation de TrueNAS et n'est testé qu'en tests unitaires.

## Dépannage {#troubleshooting}

| Symptôme | Cause et remède |
|---|---|
| **Tester la connexion** se bloque, puis échoue | L'hôte est injoignable. Vérifiez l'étape 3 (réseau, VLAN, pare-feu) ainsi que **Host SSH: Address** et **Port**. |
| `Permission denied (publickey)` alors que la clé est dans `authorized_keys` | `StrictModes` de sshd rejette le fichier de clés parce qu'un dossier parent a le mauvais propriétaire ou le mauvais mode. C'est fréquent quand `/root/.ssh` est un lien vers la clé flash FAT (`/boot/config/ssh/...`), inscriptible par tout le monde. Le journal de l'hôte montre `Authentication refused: bad ownership or modes for directory ...`. Ajoutez `StrictModes no` à la configuration sshd de l'hôte dans la section globale, avant tout bloc `Match`, puis lancez `/etc/rc.d/rc.sshd restart`. Unraid réécrit `/etc/ssh/sshd_config` à chaque démarrage, rendez-le donc persistant comme décrit plus bas. |
| `Permission denied (publickey)` et la clé manque vraiment | Refaites l'étape 2, et vérifiez que SSH est activé. |
| `Host key verification failed` | Exécutez `docker exec BombVault rm -f /config/ssh/known_hosts` et testez de nouveau. |
| Le contrôle de l'étape 3 dit `UNREACHABLE` | Le conteneur ne peut pas joindre le port SSH de l'hôte. Voir l'étape 3 : accès de l'hôte aux réseaux personnalisés, routage VLAN, ou IP shim ou IP du réseau local. |
| Les variables manquent sous **Edit** | Importez de nouveau le modèle. Un conteneur existant garde son ancienne configuration enregistrée. |
| Une VM avec des instantanés Unraid échoue et nomme un fichier | La chaîne de fichiers de base du disque ne peut pas être suivie jusqu'au bout. Vérifiez dans le VM Manager que chaque instantané de ce disque est toujours là, et que le fichier nommé existe sous `/mnt`. |

D'autres symptômes de VM figurent dans [Dépannage](troubleshooting.md#vm-backup-will-not-connect).

## Conserver la configuration après les redémarrages {#persistence}

Unraid réécrit `/etc/ssh/sshd_config` à chaque démarrage, les changements côté hôte doivent donc être refaits au démarrage :

- Le **port SSH** est conservé dans `/boot/config/ident.cfg`, réglé via l'interface web d'Unraid.
- **`StrictModes no`**, si vous en avez eu besoin plus haut, n'est pas conservé. Appliquez-le au démarrage depuis `/boot/config/go` :

  ```sh
  # /boot/config/go: keep SSH key auth working with flash-based authorized_keys
  ( for i in $(seq 1 30); do [ -f /etc/ssh/sshd_config ] && break; sleep 2; done
    grep -q '^StrictModes no' /etc/ssh/sshd_config || \
      sed -i '0,/^Match /s/^Match /StrictModes no\n&/' /etc/ssh/sshd_config
    # SIGHUP reloads sshd's config in place: it keeps the listener + open sessions
    # and avoids the "killing listener process / Restarting SSH server daemon"
    # warning that `rc.sshd restart` prints at boot. Only reload if sshd is already
    # up; if it starts after this runs, it reads the edited config on its own.
    pidof sshd >/dev/null && killall -HUP sshd ) &
  ```

- La propre clé SSH de BombVault et son `~/.ssh/config` vivent dans `/config` (appdata) et sont recréés à chaque démarrage, ils survivent donc aux redémarrages du conteneur et de l'hôte.

## Sécurité {#security}

La clé autorisée donne root sur l'hôte, le même niveau de confiance que le socket Docker que BombVault utilise déjà. Gardez BombVault sur un réseau de confiance. La clé est de type ed25519, stockée avec le mode `0600` dans appdata, et la clé de l'hôte est épinglée. Chaque commande sur l'hôte est passée comme arguments séparés, jamais par un shell distant, et la connexion SSH utilise `BatchMode` et un délai de connexion.
