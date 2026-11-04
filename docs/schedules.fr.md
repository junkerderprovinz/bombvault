# Plannings

**Paramètres, Planning** contient tous les plannings : la sauvegarde complète qui couvre tout, un planning propre à une section, les copies hors site, les éléments qui y participent et dans quel ordre. Un élément peut recevoir un planning à lui sur sa page. Cette page explique comment tout cela s'articule et parcourt une configuration type.

## Des plannings qui se lisent comme des phrases {#plans}

Chaque planning est un bouton qui se lit comme une phrase, par exemple *tous les jours à 03:00* ou *tous les 2 jours à 01:30*. Un clic l'ouvre avec cinq modes :

| Mode | Ce que vous réglez |
|---|---|
| **Désactivé** | Rien ne s'exécute avec ce planning. |
| **Quotidien** | **Heure** |
| **Hebdomadaire** | **Jours** et **Heure** |
| **Tous les N jours** | **Tous les** tant de **jours**, et l'**Heure**. Le décompte part de la dernière exécution planifiée, sauvegarder un élément à la main ne repousse donc pas le planning. |
| **Cron** | Une **Expression** de cinq champs, par exemple `0 */6 * * *` pour toutes les 6 heures. **Exemples** en propose quelques-uns, et la fenêtre montre les prochaines exécutions. |

Toutes les heures sont dans le fuseau horaire du serveur. Sur Unraid, c'est celui du système. Ailleurs, réglez `TZ`, sinon chaque planning s'exécute en UTC, voir [Variables d'environnement](configuration.md#environment-variables).

## Semaine {#week}

La carte **Semaine** dessine chaque exécution prévue du lundi au dimanche, une ligne par jour : sauvegardes, copies hors site et contrôles. Pointez un trait pour lire ce qui démarre là. Là où trois plannings démarrent à moins de deux heures d'écart, la plage est marquée, pour que vous puissiez les espacer.

![Paramètres, Planning : la bande Semaine en haut, puis la sauvegarde complète et les plannings par section.](assets/screenshots/schedule/week-dark.png#only-dark)
![Paramètres, Planning : la bande Semaine en haut, puis la sauvegarde complète et les plannings par section.](assets/screenshots/schedule/week-light.png#only-light)

## Sauvegarde complète {#full-backup}

La **Sauvegarde complète** est le planning par défaut pour tout. **Quand les sauvegardes s'exécutent** la règle : un seul passage qui sauvegarde chaque section à tour de rôle, conteneurs, VMs, clé flash, dossiers, jeux de données ZFS et l'auto-sauvegarde en dernier. La ligne en dessous dit quelles sections elle inclut. **Désactivé** la coupe, et alors seul ce qui a un planning propre s'exécute.

- **Rattraper les sauvegardes manquées après le démarrage** : quand le serveur était éteint à l'heure prévue, BombVault lance cette sauvegarde environ deux minutes après son démarrage.
- **Commandes avant et après** : **Commande avant la sauvegarde complète** s'exécute sur le serveur avec `sh -c` avant le début du passage, par exemple un script qui arrête un service. Si elle échoue, la sauvegarde complète s'exécute quand même. **Commande après la sauvegarde complète** s'exécute une fois que chaque section a été tentée, même si l'une a échoué, ce qui convient à un ping qui confirme que tout le serveur est protégé. Les commandes d'un seul conteneur se trouvent sur sa page.

**Tout sauvegarder maintenant** sur la page Sauvegardes lance le même passage à la main, sur toutes les sections activées.

## Plannings par section {#plans-per-section}

Une section peut quitter la sauvegarde complète et s'exécuter à son heure. Dans **Plannings par section**, **Exception** ajoute une section avec un planning propre, et **Supprimer l'exception** la renvoie. La ligne de la carte dit ce que font les autres, par exemple **Toutes les autres sections s'exécutent dans la sauvegarde complète.**

Une section avec son propre planning n'est pas sauvegardée de nouveau dans la sauvegarde complète, donc rien n'est sauvegardé deux fois. Il en va de même pour un élément isolé qui a son propre planning.

**Copies hors site par section** fonctionne de la même façon pour les copies. Par défaut, chaque section copie vers ses destinations hors site juste après chaque sauvegarde. **Exception** donne à une section un planning de copie à elle, par exemple pour que les VMs ne copient vers Backblaze B2 que le dimanche soir. La carte **Copier** propre à une destination peut aussi régler **Quand copier**, voir [Copies hors site](offsite-recovery.md#off-site-copies).

## Éléments planifiés {#scheduled-entries}

**Éléments planifiés** dit pour chaque section combien d'éléments participent, par exemple **38 sur 41 planifiés**. **Éléments** ouvre la liste avec un interrupteur **Au planning** par élément. Un élément désactivé n'est sauvegardé que quand vous le lancez à la main. Il compte comme non protégé, sauf si vous le marquez **Ne pas sauvegarder**.

## Ordre {#order}

**Ordre** fixe l'ordre dans lequel les exécutions planifiées et la sauvegarde complète sauvegardent les conteneurs et les VMs. Faites glisser un élément à sa place ou utilisez les flèches, et chaque déplacement est enregistré aussitôt. Les éléments que vous laissez sans ordre sont sauvegardés du plus en retard au moins en retard. Une sauvegarde isolée lancée à la main n'est pas concernée.

**Redémarrage après la sauvegarde**, dans la même carte, couvre les conteneurs qu'**Arrêter d'autres conteneurs** a arrêtés pendant une sauvegarde. Ils redémarrent dans l'ordre de leurs dépendances, et avec **Attendre que les dépendances soient saines avant de démarrer le suivant** chacun doit se déclarer sain ou en marche avant que le suivant démarre. **Délai de santé par conteneur (secondes)** plafonne l'attente, 120 par défaut, dans une plage de 5 à 3600.

## Instantané plutôt qu'arrêt {#snapshot-instead-of-stop}

**N'arrêter les conteneurs sur ZFS que le temps d'un instantané** est activé par défaut. Un conteneur dont les données sont sur ZFS n'est arrêté que pendant que BombVault prend un instantané de ces données, en général quelques secondes, et la sauvegarde est ensuite lue depuis l'instantané. Il faut pour cela la connexion SSH vers l'hôte qu'utilisent les jeux de données ZFS. Un conteneur dont une partie des données est ailleurs est arrêté pendant toute la sauvegarde comme avant, et son exécution dit pourquoi, alors qu'un conteneur sans aucune donnée sur ZFS est arrêté sans remarque. Cela fonctionne sur un partage utilisateur Unraid qui se trouve sur un seul pool ZFS. Sur TrueNAS, l'application a aussi besoin de la capacité `SYS_CHROOT`, voir [TrueNAS SCALE](getting-started.md#truenas). Un conteneur peut être réglé autrement sur sa propre page.

La sauvegarde atterrit sous les mêmes chemins et les mêmes tags que celle faite avec le conteneur arrêté, les restaurations et les contrôles traitent donc les deux de la même façon. La commande avant la sauvegarde et le dump de base de données s'exécutent toujours d'abord, les autres conteneurs à arrêter s'arrêtent et reviennent avec lui, et la commande après la sauvegarde s'exécute dès que le conteneur est de nouveau en marche. Une mise à jour après la sauvegarde attend toujours que la sauvegarde soit terminée. Ses exécutions affichent **Instantané** et le nombre de secondes pendant lesquelles le conteneur a été arrêté. Si BombVault s'arrête au milieu d'une sauvegarde de conteneur, il redémarre au prochain démarrage les conteneurs qu'il tenait arrêtés.

## Mises à jour après la sauvegarde {#updates}

- **Mettre à jour les conteneurs après une sauvegarde réussie** ne tire une image plus récente qu'une fois la sauvegarde précédente terminée, si bien qu'un point de restauration frais existe toujours d'abord. C'est désactivé par défaut, et un conteneur peut être réglé autrement sur sa page sous **Mettre à jour après une sauvegarde réussie**.
- **Supprimer l'ancienne image après une mise à jour** supprime l'image que le conteneur a quittée. Une image de base partagée avec d'autres conteneurs n'est jamais supprimée.
- **Actualiser l'état des mises à jour d'Unraid après la mise à jour d'un conteneur** efface la bannière de mise à jour d'Unraid pour ce conteneur.

Un message par conteneur mis à jour se règle sous **Paramètres, Notifications** avec **Notifier lors de la mise à jour d'un container**.

## Repos avant la sauvegarde {#idle}

La carte **Repos avant la sauvegarde** fixe quand une application compte comme au repos, pour un conteneur dont la sauvegarde planifiée l'attend : **CPU sous (% d'un cœur)**, **Trafic sous (Mbit/s)** et **Pendant au moins (min)**. Un serveur multimédia compte comme au repos quand il ne diffuse pas, et pour un conteneur sur le réseau de l'hôte, seul le CPU compte. L'attente elle-même se règle sur chaque conteneur, voir [Quand l'application est occupée](backups.md#app-busy).

## Un planning propre pour un élément {#own-plan}

Chaque élément a **Planning propre** dans la carte **Planning et destination** de son onglet Paramètres. Avec un planning là, l'élément s'exécute à ses propres heures et quitte la sauvegarde complète et le planning de sa section. Sans planning, il les suit.

## Exemple : trois plannings qui vont ensemble {#example}

Un serveur Unraid typique : la plupart des conteneurs peuvent être sauvegardés la nuit, les VMs sont grosses et changent lentement, et Immich reçoit de nouvelles photos toute la journée.

1. **Sauvegarde complète à 03:00.** Ouvrez **Paramètres, Planning**. Dans **Sauvegarde complète**, réglez **Quand les sauvegardes s'exécutent** sur **Quotidien** à `03:00`. Activez **Rattraper les sauvegardes manquées après le démarrage**, pour qu'une nuit serveur éteint soit rattrapée.
2. **Les VMs une fois par semaine.** Dans **Plannings par section**, appuyez sur **Exception**, choisissez **VMs** et réglez son planning sur **Hebdomadaire**, **Jours** dimanche, **Heure** `04:00`. La carte indique maintenant que toutes les autres sections s'exécutent dans la sauvegarde complète.
3. **Les VMs copient aussi hors site le dimanche.** Dans **Copies hors site par section**, appuyez sur **Exception**, choisissez **VMs** et réglez **Hebdomadaire**, dimanche à `06:00`, après que leur sauvegarde a eu le temps de se terminer.
4. **Immich toutes les 6 heures.** Ouvrez **Sauvegardes**, puis `immich`, puis son onglet **Paramètres**. Réglez **Planning propre** sur **Cron** avec `0 */6 * * *`.

Ce qui s'exécute maintenant :

| Quand | Quoi |
|---|---|
| Chaque jour à 03:00 | Les conteneurs sauf `immich`, la clé flash, les dossiers, les jeux de données ZFS et l'auto-sauvegarde, une section après l'autre |
| Dimanche à 04:00 | Les VMs |
| Dimanche à 06:00 | La copie hors site des VMs |
| 00:00, 06:00, 12:00 et 18:00 | `immich` seul |

La carte **Semaine** montre la ligne de 03:00 chaque jour, les traits de 04:00 et de 06:00 le dimanche, et quatre traits par jour pour `immich`. `immich` n'est pas sauvegardé une seconde fois à 03:00, car un élément avec son propre planning quitte la sauvegarde complète. Les copies des autres sections s'exécutent toujours juste après chaque sauvegarde.

Quand vous supprimerez plus tard le planning propre d'`immich`, il rejoindra de nouveau l'exécution de 03:00 avec sa section.
