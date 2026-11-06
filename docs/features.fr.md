# Fonctionnalités

Un court catalogue de ce que fait BombVault. Chaque ligne renvoie vers la page qui l'explique.

## Ce qui est sauvegardé {#backup-scope}

- **Conteneurs** : le dossier appdata et la définition du conteneur, avec un choix de dossiers par conteneur. [Sauvegardes et éléments](backups.md#what-is-saved)
- **VMs** : images disque, XML et NVRAM UEFI via SSH, avec un arrêt propre, un instantané à chaud ou les seuls blocs modifiés. Les VMs avec des instantanés Unraid et des disques sur des zvols sont entièrement prises en charge. [Sauvegardes de VM](vm-backup-ssh-setup.md)
- **Flash** : toute la clé USB flash d'Unraid, avec un export zip planifié. [Sauvegardes et éléments](backups.md#settings-tab)
- **Auto-sauvegarde** : les réglages et les identifiants de BombVault lui-même. [Assistant d'urgence](offsite-recovery.md#emergency)
- **Dossiers** : n'importe quel dossier du serveur comme ensemble de dossiers. [Ajouter un dossier](backups.md#add)
- **Jeux de données ZFS** : une arborescence de jeux de données lue à partir d'un seul instantané, avec restauration possible de fichiers isolés et une réplique facultative sur un second serveur ZFS. [Jeux de données ZFS](zfs-datasets.md)
- **Sauvegardes SQL de la base** : PostgreSQL, MySQL et MariaDB, exportés depuis le serveur en marche dans un point de restauration à part. [Dumps de base de données](backups.md#database-dumps)

## Sauvegardes et pages d'éléments {#backups}

- **Une seule liste** avec des tuiles, **Filtres**, **Trier** et **Rechercher**, et les quatorze dernières exécutions de chaque ligne sous forme de bande. [La liste](backups.md#list)
- **Sélection comme dans un gestionnaire de fichiers**, et sauvegarde ou mise en pause d'une sélection sur le serveur. [Choisir plusieurs éléments](backups.md#select)
- **Une page par élément** avec **Sauvegardes**, **Paramètres** et **Vérification**. [La page d'un élément](backups.md#entry-page)
- **Étiquettes et comparaison** pour les instantanés, et **Exporter** en tar, en zip ou en dépôt restic. [Onglet Sauvegardes](backups.md#backups-tab)
- **Motifs d'exclusion** avec aperçu en direct, et exclusions recommandées pour les applications connues. [Motifs d'exclusion](backups.md#excludes)
- **Commandes avant et après**, et **Arrêter d'autres conteneurs** avec un redémarrage conditionné à la santé, dans l'ordre des dépendances. [Commandes et autres conteneurs](backups.md#commands)
- **Quand l'application est occupée** : une sauvegarde planifiée attend Plex, Jellyfin, Emby, votre propre contrôle ou un trafic calme, et réessaie toute seule. [Quand l'application est occupée](backups.md#app-busy)
- **Instantané plutôt qu'arrêt** : un conteneur sur ZFS n'est arrêté que quelques secondes. [Plannings](schedules.md#snapshot-instead-of-stop)
- **Exports simples**, éventuellement scellés avec age. [Exports simples](backups.md#exports)

## Restauration {#restore}

- **Une page et une fenêtre** pour tous les types, avec la sauvegarde recommandée présélectionnée. [Restauration](restore.md)
- **À côté de l'original** par défaut, et **Remplacer l'original** sauvegarde d'abord l'état actuel. [La fenêtre de restauration](restore.md#window)
- **Fichiers isolés** vers un dossier ou à leur place d'origine. [Récupérer un seul fichier](restore.md#example-single-file)
- **Plusieurs éléments à la fois**, chacun depuis sa sauvegarde recommandée ou depuis un même instant. [Restaurer plusieurs éléments](restore.md#several)
- **Stacks Compose entières**, démarrées dans l'ordre des dépendances. [Restaurer une stack](restore.md#stacks)
- **Les conteneurs et les VMs sont recréés** sur l'image avec laquelle ils tournaient, avec GPU, limites et liens. [Ce qui revient](restore.md#kinds)
- **Contrôles avant la restauration**, et une liste de ce qu'elle modifie. [La fenêtre de restauration](restore.md#window)
- **La clé flash en ZIP**, et un plugin à la fois de retour dans la clé flash en service. [Ce qui revient](restore.md#kinds)
- **ZFS** dans le jeu de données avec un instantané de sécurité, dans un dossier ou dans un nouveau jeu de données avec ses propriétés. [Jeux de données ZFS](zfs-datasets.md#restore)
- **Assistant d'urgence** pour un serveur perdu, avec le kit de récupération, via le groupe ou avec la clé saisie. [Assistant d'urgence](offsite-recovery.md#emergency)
- **Depuis un ancien dossier de sauvegarde**, et **Importer depuis le plugin Appdata.Backup**. [Autres sources](restore.md#other-sources)

## Stockage et planification {#storage-scheduling}

- **Destinations** de tous types : dossiers locaux, SMB, WebDAV, SFTP, rest-server, S3, lecteurs cloud et tout remote rclone. [Destinations](offsite-recovery.md#destinations)
- **Identifiants** : une seule liste avec un identifiant par défaut. [Identifiants](offsite-recovery.md#sign-ins)
- **Dossiers propres** sur ce serveur ou dans une destination. [Le dossier propre d'une section](offsite-recovery.md#own-folder)
- **Le 3-2-1 sous forme de flux** où chaque ligne est un interrupteur, et **destinations par élément**. [Destinations par élément](offsite-recovery.md#placement)
- **Copies hors site** vers plusieurs destinations par section, avec limites de bande passante, compression, classes de stockage froid et un ralentissement pendant que Plex diffuse. [Copies hors site](offsite-recovery.md#off-site-copies)
- **Dépôts directs**, listés sous **Sauvegardé directement ici**. [Dépôts directs](offsite-recovery.md#direct-repositories)
- **Rétention sous forme de chronologie** : **Court**, **Équilibré**, **Long** ou **Valeurs personnalisées** par destination, avec des règles par section. [Durée de conservation](offsite-recovery.md#keep)
- **Déplacer l'historique d'une section**, et **Supprimer la destination** avec ou sans ses sauvegardes. [Déplacer et supprimer](offsite-recovery.md#move-remove)
- **Plannings qui se lisent comme des phrases**, la bande **Semaine**, une sauvegarde complète, des plans par section et un planning propre par élément. [Plannings](schedules.md)
- **Rattrapage des sauvegardes manquées**, un ordre manuel, et des mises à jour après une sauvegarde réussie. [Plannings](schedules.md)

## Vue d'ensemble, vérification et surveillance {#insight-verification-monitoring}

- **L'aperçu** : en-tête d'état, **À traiter**, **État de protection**, prévision de stockage, historique et cartes facultatives. [Aperçu](overview.md)
- **Détection d'anomalies** par élément, qui conserve aussi les anciennes sauvegardes quand une source s'effondre. [Anomalies](anomalies.md)
- **Contrôle de restauration** après la première sauvegarde, et **Test de démarrage** dans une copie isolée. [Onglet Vérification](backups.md#verify-tab)
- **Contrôles planifiés et restaurations d'essai** depuis la copie hors site. [Exercices de reprise](offsite-recovery.md#dr-drills)
- **Taille par dossier**, et pourquoi une sauvegarde a été lente. [Ce qu'une exécution vous dit](backups.md#run-notes)
- **Notifications** : webhook, Gotify, Matrix, e-mail, Apprise, Unraid et Healthchecks, avec un résumé hebdomadaire et un rapport de retard. [Notifications](notifications.md)
- **Prometheus**, un appareil Home Assistant, une API HTTP et mDNS. [API et intégrations](api.md)
- **Rapport de bug** : un ZIP expurgé pour un ticket GitHub. [Dépannage](troubleshooting.md#still-stuck)

## Protection contre les rançongiciels {#ransomware-protection}

- **Protection contre la suppression (ajout seul)**, imposée par l'autre côté et prouvée par un test d'altération. [Protection contre la suppression](offsite-recovery.md#delete-protection-append-only)
- **Accepter sans protection contre la suppression** pour un endroit qui ne peut pas refuser une suppression. [Protection contre la suppression](offsite-recovery.md#delete-protection-append-only)
- **Supprimer des sauvegardes pour de bon demande votre propre connexion** dès qu'un mot de passe de connexion est défini, et un téléphone appairé ne le peut pas. [Modèle de sécurité](configuration.md#security-model)
- **Kit de récupération**, éventuellement scellé avec age, et un **gardien** dans votre groupe. [Kit de récupération](offsite-recovery.md#recovery-kit), [Gardien](offsite-recovery.md#key-keeper)

## Plusieurs serveurs {#several-servers}

- **Appairage par douze mots**, chiffré de bout en bout, directement ou via un relais. [Appairage des instances](offsite-recovery.md#pairing)
- **La page Instances** avec le groupe sous forme de réseau et les rôles **Récepteur**, **Rapatrieur**, **Serveur ZFS** et **Gardien**. [Instances et rôles](offsite-recovery.md#roles)
- **Un serveur récepteur** configuré pour vous, en mode ajout seul. [Serveur récepteur](offsite-recovery.md#receiving-server)

## Assistants IA (MCP) {#mcp}

Un serveur MCP intégré permet à un assistant de lire l'état des sauvegardes, la couverture, l'historique des exécutions et les points de restauration, et, avec une clé qui l'autorise, de lancer une sauvegarde. Les restaurations, les suppressions, l'élagage et les réglages restent dans l'interface web. [Serveur MCP](mcp.md)

## Applications et compagnons {#apps}

- **Application Android** : tous les serveurs de votre groupe sur votre téléphone. [Application Android](android.md)
- **BombVault Widget** : une tuile sur le tableau de bord d'Unraid. [Applications](settings.md#apps)
- **Journal d'activité intégrable** pour des tableaux de bord comme Homepage ou Heimdall. [Widget de tableau de bord](api.md#dashboard-widget)

## Divers {#other}

- **42 langues**, trois thèmes, et libellés, couleurs, coins et animation à votre goût. [Apparence](settings.md#look)
- **Des réglages qui s'enregistrent tout seuls**, avec une recherche sur les quatorze tuiles. [Paramètres](settings.md)
- **Nouveautés** après une mise à jour, avec des points verts sur ce qui a changé. [Info](settings.md#info)
- **HTTPS prêt à l'emploi**, et un contrôle de santé pour les outils d'auto-réparation. [Configuration](configuration.md#network)
