# Anomalies

Un rançongiciel et une application cassée se ressemblent dans une sauvegarde : d'un coup, la plus grande partie des données est nouvelle, ou elle a disparu. BombVault compare chaque sauvegarde à l'historique propre de l'élément et signale une anomalie quand quelque chose cloche. Tant qu'une anomalie sur des données perdues est ouverte, il conserve aussi les anciennes sauvegardes de l'élément, pour que la dernière bonne ne disparaisse pas avec le temps. L'entrée **Anomalies** de la barre latérale s'affiche tant que la détection est activée.

![La page Anomalies avec une anomalie critique ouverte.](assets/screenshots/anomalies/page-dark.png#only-dark)
![La page Anomalies avec une anomalie critique ouverte.](assets/screenshots/anomalies/page-light.png#only-light)

## La page {#page}

Les tuiles **Critique**, **Avertissements** et **Remarques** choisissent un niveau de gravité, et **Ouverte** et **Fermée** au-dessus de la liste basculent entre ce qui est ouvert et ce qui a été fermé ces 30 derniers jours. Une anomalie critique figure sur la même surface que les autres, et sa gravité se voit sur son badge et sur son bord coloré. Une anomalie propose d'abord sa solution, et ses détails montrent la valeur sous forme de courbe sur la plage habituelle.

Quand rien n'est ouvert, **Progression de l'apprentissage** montre un anneau de dix parts par élément, une part pour chaque sauvegarde dont BombVault a appris. **Surveillance** ouvre la liste des éléments avec leurs réglages propres.

## Ce qui est vérifié {#checks}

Chaque sauvegarde d'un conteneur, d'une VM, d'un ensemble de dossiers, d'un dump de base de données, de la clé USB flash et de la sauvegarde de BombVault lui-même est comparée à l'historique propre de l'élément. Les contrôles portent sur :

- les nouvelles données qu'une exécution a stockées, comparées aux plus grandes quantités habituelles des sauvegardes récentes et au débit habituel par heure,
- une sauvegarde qui a stocké de nouveau la plus grande partie des données, y compris des fichiers renommés puis réécrits,
- la taille de la source et le nombre de fichiers que restic indique pour chaque élément et chaque dump de base de données,
- la durée de sauvegarde mesurée par restic lui-même,
- les séries d'échecs et les échecs intermittents,
- les contrôles de restauration qui ne réussissent plus,
- l'espace libre des dépôts locaux, SFTP et rclone, projeté d'après la croissance du dépôt.

Un élément apprend de ses 10 premières sauvegardes. Une source presque vide, une réécriture de la plus grande partie des données et les échecs sont contrôlés dès le début.

Une anomalie sur la taille ou le nombre de fichiers d'une source est datée de la première sauvegarde où elle est apparue. **Comparer avec la sauvegarde précédente** liste les dossiers où des fichiers ont disparu, sont arrivés ou ont changé, avec une remarque quand presque tout se trouve dans un index de recherche, un cache ou des miniatures que l'application reconstruit toute seule.

Les éléments ZFS sont surveillés jeu de données par jeu de données, voir [Jeux de données ZFS](zfs-datasets.md#anomalies). Un assistant peut lire les anomalies ouvertes via le [serveur MCP](mcp.md#tools).

Les contrôles ont des limites. Les dépôts S3, B2 et REST n'ont pas de chiffre d'espace libre, sur le partage utilisateur d'Unraid l'espace libre est celui de toute la grappe, et les sauvegardes faites avant restic 0.17 n'ont pas d'historique de taille.

## Fermer une anomalie {#closing}

- Les **Avertissements** se ferment tout seuls dès que la cause a disparu.
- Les anomalies **critiques** sur des données perdues et sur un disque qui se remplit restent jusqu'à ce que vous agissiez.
- **Accuser réception** marque une anomalie comme vue et la ferme. Elle n'est plus signalée de nouveau tant que sa cause n'a pas disparu au moins une fois.
- **Marquer comme attendue** dit que le changement venait de vous. Le nouveau niveau devient normal après 10 sauvegardes, mais le contrôle « presque vide » n'est jamais désactivé.
- Une sélection modifiée, par exemple un dossier que vous avez coché ou décoché, repart d'un historique neuf toute seule.

Une notification part une fois par épisode, et les échecs et contrôles de restauration qui notifient déjà ne sont pas signalés deux fois.

## Les anciennes sauvegardes sont conservées entre-temps {#retention-hold}

Tant qu'une source est presque vide, a fortement rétréci ou a vu la plus grande partie de ses données stockées de nouveau, la rétention et l'élagage laissent les anciennes sauvegardes de cet élément tranquilles, et l'anomalie renvoie vers la dernière bonne sauvegarde, que la fenêtre de restauration présélectionne. Avant que vous ne fermiez une telle anomalie, BombVault vous prévient que le prochain élagage supprimera de nouveau les anciennes sauvegardes selon les règles de conservation. Si des données manquent ou ont été chiffrées, restaurez donc d'abord et fermez l'anomalie ensuite.

## Paramètres {#settings}

La carte **Anomalies** sous **Paramètres, Vérification** contient les réglages globaux. Chaque commande s'enregistre dès que vous la modifiez, et les trois sous l'interrupteur sont masquées tant que la détection est désactivée.

| Réglage | Par défaut | Effet |
|---|---|---|
| **Détecter les anomalies** | Activé | Compare chaque sauvegarde à l'historique propre de l'élément. Désactivé, plus rien de nouveau n'est contrôlé et l'entrée **Anomalies** quitte la barre latérale. La carte renvoie toujours vers les anomalies passées. |
| **Sensibilité** | Équilibré | Stricte signale de plus petits changements, Permissive seulement les grands. |
| **Envoyer une notification pour** | Seulement les constats critiques | Le niveau de gravité le plus bas qui envoie un message par les canaux configurés sous [Notifications](notifications.md). Les échecs répétés de sauvegarde et de dump et les contrôles de restauration planifiés échoués envoient déjà leur propre message et ne sont pas envoyés deux fois. |
| **Garder les anciennes sauvegardes quand une source rétrécit fortement ou est réécrite** | Activé | Tant qu'un élément a une anomalie ouverte pour une source presque vide, un fort rétrécissement ou la plus grande partie de ses données stockées de nouveau, la rétention et l'élagage laissent ses anciennes sauvegardes tranquilles. Accusez réception de l'anomalie ou marquez-la comme attendue pour les libérer. |

Chaque élément peut avoir sa propre sensibilité et son propre minimum de notification. Réglez-les sur l'onglet **Vérification** de l'élément, ou sur la page **Anomalies** sous **Surveillance**. Pour un élément ZFS, ils s'appliquent à chaque jeu de données de son arborescence.
