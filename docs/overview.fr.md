# Aperçu

L'aperçu est la première page de la barre latérale et celle à regarder chaque matin. Il répond d'un coup d'œil à deux questions : tout est-il sauvegardé, et quelque chose vous attend-il ? Il commence par un en-tête d'état et quatre cartes, et d'autres cartes attendent sous **Personnaliser**.

![L'aperçu après quelques jours : l'en-tête d'état, À traiter avec un avertissement, la protection de chaque section, les destinations et l'historique.](assets/screenshots/overview/home-dark.png#only-dark)
![L'aperçu après quelques jours : l'en-tête d'état, À traiter avec un avertissement, la protection de chaque section, les destinations et l'historique.](assets/screenshots/overview/home-light.png#only-light)

## L'en-tête d'état {#status-head}

L'en-tête est tout en haut de la page et y reste toujours. Son anneau montre la part protégée, par exemple **41 sur 43 protégés**, et la ligne à côté dit où l'on en est :

| L'en-tête dit | Ce que cela signifie |
|---|---|
| **Tout est sauvegardé** | Chaque élément de chaque section activée a une sauvegarde à jour. |
| **Sauvegardé, avec remarques** | Tout est sauvegardé, et certaines exécutions ont laissé une remarque, par exemple des fichiers qu'elles n'ont pas pu lire. |
| **Tout n'est pas encore protégé** | Certains éléments n'ont pas de sauvegarde à jour, par exemple parce qu'ils attendent leur première. |
| **3 points à traiter** | Quelque chose a échoué ou attend une décision. La carte **À traiter** le liste. |
| **Sauvegarde en cours** | Une sauvegarde tourne. L'anneau devient un anneau de progression avec les étapes **Arrêt**, **Lecture** et **Démarrage**, et une sauvegarde complète compte ses éléments au fur et à mesure, par exemple **Sauvegarde en cours : 12 sur 43**. **Voir l'exécution** ouvre l'exécution. |
| **Copie hors site en cours** | Une copie vers une destination tourne. Elle n'affiche aucun pourcentage, car `restic copy` n'en indique pas. |

En dessous figurent le moment de la dernière exécution et le démarrage de la prochaine prévue.

## À traiter {#needs-you}

Cette carte liste ce que vous devriez regarder, les plus récents d'abord, et indique **Rien à faire.** quand la liste est vide. Chaque ligne a sa propre sortie :

- **Une sauvegarde échouée** nomme l'élément et sa raison. **Voir les erreurs** montre la sortie propre de restic, et **Réessayer** relance la sauvegarde.
- **Échec de la copie hors site** nomme la section et depuis quand. **Répliquer maintenant** réessaie.
- **Anomalie sur plex** mène à l'anomalie avec **Vérifier**. Tant qu'une anomalie critique est ouverte, la rétention conserve les anciennes sauvegardes de cet élément.
- Les **nouveaux** éléments que BombVault a trouvés et que rien ne sauvegarde encore, comme un conteneur que vous venez d'installer, proposent **Commencer à sauvegarder** ou **Ne pas sauvegarder**.
- **Une sauvegarde qui a renoncé** parce que son application est restée occupée pendant toute l'attente, voir [Quand l'application est occupée](backups.md#app-busy).
- **Une réplique ZFS échouée**, en mots simples, par exemple que le serveur n'a pas de commande zfs, avec la sortie du serveur sous **Afficher**.
- **Les sauvegardes sont arrêtées : la clé ne peut plus être ouverte** apparaît quand ce serveur a repris la clé d'un autre serveur et qu'`APP_KEY` a changé ensuite. Voir [Dépannage](troubleshooting.md#backups-stopped-and-say-the-key-can-no-longer-be-opened).
- **Sauvegardez votre kit de récupération** reste tant que vous n'avez pas téléchargé le kit et appuyé sur **Je l'ai conservé en lieu sûr**.

## État de protection {#protection-status}

Chaque section activée a une ligne avec quatre contrôles :

| Contrôle | Il passe au vert quand | Autres formulations possibles |
|---|---|---|
| **Local** | la dernière sauvegarde est à jour | pas encore de sauvegarde, pas de planning, dernière exécution échouée, sauvegarde en retard, tous les éléments ne sont pas protégés, sauvegarde bientôt due |
| **Hors site** | la dernière copie hors des locaux est à jour | pas de copie hors site, copie en cours, copie en retard, attend votre décision, pas encore de copie, échec de la réplique |
| **Protection contre la suppression** | un test d'altération a prouvé que l'autre côté refuse les suppressions | aucune, aucune et risque accepté, pas encore prouvée, échec du test, non prouvée car son contrôle est désactivé |
| **Restauration de test** | une restauration depuis la copie hors site a réussi | jamais lancée, en cours, échouée, en retard, manuelle uniquement, impossible pour cette section |

Une section n'est verte que sur des faits vérifiés. Une copie sur ce serveur ne compte pas comme hors site, et une destination que vous avez marquée **Accepter sans protection contre la suppression** montre un anneau gris au lieu d'un point ambre, si bien que la couleur de la section suit ses autres contrôles.

Une section planifiée affiche **Pas encore de sauvegarde** et reste neutre jusqu'à ce que sa première exécution soit due, à compter du dernier réglage de son planning. Ensuite, elle indique **En retard** et alerte une fois. **En retard** se fonde sur l'élément qui a pris le plus de retard sur son propre planning, si bien qu'un voisin qui tourne ne cache pas un élément qui s'est arrêté. Un élément dont le conteneur a disparu ou qu'aucun planning n'atteint ne compte pas. Un élément avec son propre planning hebdomadaire n'est pas tenu au planning quotidien de la section. Une section qui n'a rien à sauvegarder, parce qu'aucun planning n'atteint ses éléments, affiche **Non planifié** et n'est jamais en retard.

Un clic sur une section ouvre sa fenêtre avec chaque contrôle détaillé, les éléments qu'elle compte et **Lancer la restauration de test** ou **Relancer la restauration de test**. Elle indique aussi **Règle d'élagage définie** ou **Aucune règle d'élagage**, et la règle de conservation du dossier propre d'une section compte autant que ses règles hors site.

![La fenêtre de protection de la section Conteneurs avec ses quatre contrôles détaillés.](assets/screenshots/overview/protection-window-dark.png#only-dark)
![La fenêtre de protection de la section Conteneurs avec ses quatre contrôles détaillés.](assets/screenshots/overview/protection-window-light.png#only-light)

## Destinations {#storage-locations}

Chaque destination figure ici comme un récipient avec son niveau de remplissage, sa vitesse de croissance, par exemple **Croissance de 4,2 GB/semaine**, et le moment où il serait plein à ce rythme, par exemple **plein dans environ 31 semaines**. S3, B2 et les rest-servers n'indiquent pas la place qui reste, ils affichent donc **Espace libre inconnu** et seulement leur croissance. Un clic mène à la page de la destination. Voir [Destinations](offsite-recovery.md#destinations).

## Historique {#history}

**Historique** a deux onglets. **Exécutions** liste chaque exécution avec son résultat, sa durée et sa raison, et un clic l'ouvre. **Journal** montre chaque ligne avec son heure, telle que BombVault l'écrit, et peut être restreint à un jour. Une ligne passe à l'orange quand une sauvegarde est passée mais a laissé quelque chose de côté, et la ligne nomme ce qu'elle a laissé de côté.

## Autres cartes sous Personnaliser {#customize}

**Personnaliser**, en haut de la page, permet d'organiser les cartes. Faites glisser une carte par sa barre pour la déplacer, redimensionnez-la au point de son coin inférieur, ou masquez-la. Les cartes masquées attendent sous **Cartes masquées**, et **Afficher** en ramène une. Pendant que vous les organisez, les cartes oscillent un peu, et **Terminé** y met fin. La disposition est enregistrée dans ce navigateur.

Ces cartes sont masquées au départ :

- **Anomalies** : les anomalies ouvertes, voir [Anomalies](anomalies.md).
- **Non sauvegardé** : tout ce qui se trouve sur ce serveur et qu'aucune sauvegarde planifiée ne couvre, chacun avec sa raison, par exemple **jamais ajouté**, **non inclus dans le planning** ou **toutes les destinations de sa section sont désactivées**. Elle lit les conteneurs directement dans Docker, elle trouve donc aussi un conteneur que BombVault n'a jamais vu.
- **Calendrier** : les 90 derniers jours, un carré par jour. Un clic sur un jour montre ses lignes dans le journal sous **Historique**.
- **Stockage** : la place que prend chaque section et la qualité de la déduplication de ses données.
- **Aperçu** : l'instantané le plus ancien de toutes les sections, le moment de la prochaine sauvegarde et le déroulement de la dernière.
- **Statistiques** : combien de conteneurs et de VMs, combien d'erreurs, et combien de plans sont actifs ou en pause.
- **Intégration hôte** : ce que BombVault atteint et dont il a besoin sur l'hôte, comme le socket Docker, libvirt, restic et rclone. Commencez ici quand quelque chose semble mal raccordé.

![Le mode Personnaliser : les cartes peuvent être déplacées, redimensionnées et masquées, et les cartes masquées attendent en dessous.](assets/screenshots/overview/customize-dark.png#only-dark)
![Le mode Personnaliser : les cartes peuvent être déplacées, redimensionnées et masquées, et les cartes masquées attendent en dessous.](assets/screenshots/overview/customize-light.png#only-light)

## Comment BombVault compte {#counting}

Chaque carte et l'en-tête comptent de la même façon : les éléments des sections activées qui sont encore installés et que vous n'avez pas marqués **Ne pas sauvegarder**. Un élément ne compte comme protégé que tant qu'un planning l'atteint et qu'il a une sauvegarde. Une ancienne sauvegarde seule ne compte pas, si bien qu'un conteneur que vous avez sorti du planning apparaît comme non protégé jusqu'à ce que vous le remettiez ou le marquiez **Ne pas sauvegarder**.

## Premiers pas {#first-steps}

Sur une nouvelle installation, l'aperçu commence par la carte **Premiers pas** : **Choisir les sections**, **Ajouter une destination** et **Première sauvegarde**. Elle reste jusqu'à la fin de la première sauvegarde. [Premiers pas](getting-started.md#first-run) la parcourt.
