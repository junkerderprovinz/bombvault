# Paramètres

Les **Paramètres** sont quatorze tuiles. Chaque tuile est une page de cartes, et chaque modification est enregistrée au moment où vous la faites, avec un bref éclair sur la commande, ou une secousse quand le serveur la refuse. Quelques fenêtres gardent un bouton **Enregistrer** parce qu'enregistrer à moitié serait risqué, comme un identifiant ou le mot de passe de connexion. Les explications se trouvent dans les bulles (i) à côté de chaque commande.

Appuyez sur `/` ou Ctrl+K n'importe où dans les paramètres pour tous les parcourir, aussi par les noms que des pages portaient dans des versions antérieures. L'ordre des tuiles se change sous **Apparence**. Quand une colonne de tuiles à côté de la page l'écraserait, comme dans une fenêtre d'environ 1200 pixels de large ou sur un téléphone, les tuiles deviennent une bande au-dessus de la page qui défile sur les côtés.

![Les paramètres avec la tuile Général ouverte.](assets/screenshots/getting-started/settings-rail-dark.png#only-dark)
![Les paramètres avec la tuile Général ouverte.](assets/screenshots/getting-started/settings-rail-light.png#only-light)

Cette page liste chaque tuile et ses cartes, et renvoie vers la page qui explique chacune en détail.

## Général {#general}

- **Sections** active ou désactive chaque section : **Conteneurs**, **VMs**, **Flash**, **Dossiers**, **Jeux de données ZFS** et **Auto-sauvegarde**. Une section désactivée n'est sauvegardée par aucun planning, propre planning d'un élément compris, et ses éléments ne comptent pas. **Sauvegarder maintenant** fonctionne toujours. **Flash** n'apparaît que sur Unraid.
- **Conteneurs** contient ce qui s'applique à tous les conteneurs, comme **Exports SQL automatiques**. Tout ce qui concerne un seul conteneur se trouve sur sa page. Voir [Dumps de base de données](backups.md#database-dumps).

## Apparence {#look}

- **Apparence** : **Langue** (42 langues), **Thème** (**Sombre**, **Clair** ou **Système**) avec **OLED** en dessous, qui donne un vrai noir aux écrans OLED et AMOLED tant que le thème sombre s'affiche, **Angles**, **Animation** (**Désactivé**, **Léger**, **Sauvage** ou **Déchaîné**), **Notifications discrètes**, qui masque les avis de réussite comme les confirmations d'enregistrement, et **Ordre des pages de paramètres**, où vous maintenez une tuile et la faites glisser à sa place, ou utilisez Alt et les touches fléchées.
- **Libellés** : la part d'une commande qui est affichée, séparément pour **Boutons**, **Barre latérale**, **Onglets** et **Barre inférieure** : **Texte**, **Texte et symbole**, **Symbole**, ou **Réactif**, qui n'affiche que le symbole et fait glisser les mots quand on le pointe. **Barre latérale** n'agit que sur la disposition de bureau et **Barre inférieure** que sur celle du téléphone.
- **Couleurs** : une **Couleur d'accentuation**, ou le **Mode arc-en-ciel**, où chaque ligne d'une liste reçoit sa propre couleur parmi une palette de huit. Le Mode arc-en-ciel ajoute le **Mode disco**, où les couleurs glissent lentement vers la suivante, le **Mode réactif**, où la couleur d'une ligne ne s'affiche que quand on la pointe, qu'elle tourne ou qu'elle est sélectionnée, la **Rotation des couleurs** et la **Palette de couleurs** elle-même. La baguette au début des couleurs d'accentuation, **Couleur du système**, reprend la couleur propre de votre appareil : Material You dans l'application Android à partir d'Android 12, et la couleur du système dans Firefox et Safari. Elle n'apparaît que là où l'appareil a une telle couleur.

BombVault garde l'apparence sur le serveur. Un navigateur qui vient pour la première fois la reprend. **OLED** et **Couleur du système** font exception : chaque appareil les garde pour lui, car elles dépendent de son écran. Quand votre système demande moins de mouvement, cela l'emporte toujours sur **Animation**.

## Planning {#schedule}

**Semaine**, **Sauvegarde complète** avec **Quand les sauvegardes s'exécutent**, **Rattraper les sauvegardes manquées après le démarrage** et **Commandes avant et après**, **Plannings par section** avec **Copies hors site par section**, **Éléments planifiés**, **Ordre** avec **Redémarrage après la sauvegarde**, **Instantané plutôt qu'arrêt**, **Mises à jour après la sauvegarde** et **Repos avant la sauvegarde**. Voir [Plannings](schedules.md).

## Destinations {#storage-locations}

- **Afficher aussi dans la barre latérale** épingle la page Destinations à la barre latérale.
- La liste de vos destinations, avec **Ajouter une destination**. Un clic ouvre la page d'une destination, voir [La page d'une destination](offsite-recovery.md#location-page).
- **Identifiants** : les identifiants enregistrés avec l'étoile du réglage par défaut, voir [Identifiants](offsite-recovery.md#sign-ins).
- **Destinations par section** : le flux 3-2-1, où chaque ligne est un interrupteur, voir [Valeurs par défaut par section](offsite-recovery.md#placement-defaults).
- **Dépôts** : les dépôts créés pour des éléments isolés, voir [Destinations par élément](offsite-recovery.md#placement).
- **Durée de conservation** renvoie aux destinations, où la rétention se règle, et **Aperçu** montre **Ce que la prochaine exécution supprimerait**, par dépôt et par élément, avant que cela arrive. Cela ne change rien et ne verrouille rien, il répond donc même pendant qu'une sauvegarde tourne.
- **Copies hors site** : une ligne par section avec **Tester la connexion**, **Répliquer maintenant** et **Configurer** ou **Modifier**, voir [Copies hors site](offsite-recovery.md#off-site-copies).
- **Bande passante hors site** : **Limite d'envoi (KiB/s)** et **Limite de téléchargement (KiB/s)** pour chaque copie. 0 veut dire illimité.

La carte **Streaming**, qui abaisse la limite d'envoi pendant que Plex, Jellyfin ou Emby diffusent, se trouve sur la page Destinations elle-même.

## Restauration {#restore}

- Un lien vers la page **Restauration**.
- **Dossiers de restauration** : le **Dossier de restauration par défaut** pour une restauration dans un dossier, avec **Sous-dossier par section** et une **Exception** par section. Voir [Où va une restauration dans un dossier](restore.md#folders).
- **Registres de containers** : identifiants pour les registres privés, comme une image `ghcr.io` réservée aux sponsors. Une restauration s'en sert pour tirer une image manquante, et une mise à jour après la sauvegarde pour chercher une version plus récente. Les images publiques sont tirées sans identifiants. **Ajouter un registre** propose des registres publics, des registres auto-hébergés, ceux d'un fournisseur cloud et **Autre registre** pour n'importe quelle adresse.
- **En cas d'urgence** : **Télécharger le kit de récupération**, qui demande un mot de passe de connexion. Voir [Kit de récupération](offsite-recovery.md#recovery-kit).

## Notifications {#notifications}

**Notifications** avec **Notifier**, les résumés, **Signaler les sauvegardes en retard**, **Notifications Unraid** et le **Résumé hebdomadaire**, **Canaux** avec Webhook, Apprise, Matrix, Gotify et E-mail (SMTP), et **Healthchecks**. Voir [Notifications](notifications.md).

## Vérification {#verify}

- **Vérifier les sauvegardes** : le résultat du dernier `restic check` par section, avec **Déverrouiller** et **Élaguer** sous **Plus** dans chaque ligne. **Déverrouiller** lève un verrou périmé, et **Élaguer** applique les règles de conservation à la demande.
- **Vérifications automatiques** : le contrôle de restauration planifié avec son **Échantillon de données (%)** et ses cibles de test, **Test de démarrage**, qui restaure aussi un conteneur dans une copie isolée et le démarre, l'**Exercice DR hors site planifié** et **Vérifier la protection contre la suppression**, le test d'altération planifié. Voir [Exercices de reprise](offsite-recovery.md#dr-drills) et [Test d'altération](offsite-recovery.md#tamper-test).
- **Anomalies** : la détection, la sensibilité et les anomalies qui envoient un message. Voir [Anomalies](anomalies.md).

## Sécurité {#security}

- **Mot de passe** : le mot de passe de connexion, d'au moins 12 caractères. Le changer ou l'effacer demande l'actuel et, avec l'authentification à deux facteurs, le code.
- **Authentification à deux facteurs** : un code d'une application d'authentification en plus du mot de passe, avec huit codes de récupération montrés une seule fois.
- **Clés d'accès** : connectez-vous avec une clé sur votre téléphone, votre ordinateur portable ou une clé de sécurité. Elles demandent un vrai nom de domaine et un certificat que le navigateur approuve.
- **Application Android** : **Les téléphones peuvent s'appairer avec les douze mots du groupe**, ce qui permet à un téléphone appairé de se connecter sans le mot de passe.
- **Chiffrement** : **Chiffrer les sauvegardes**, activé par défaut.

Plusieurs choses demandent un mot de passe de connexion, comme le kit de récupération et les dépôts de clés. Voir [Modèle de sécurité](configuration.md#security-model).

## Instances {#instances}

- **Afficher aussi dans la barre latérale** épingle la page Instances à la barre latérale.
- **Plusieurs serveurs** active les éléments d'un groupe : **Instances**, la page avec une carte pour chaque membre, **Récepteur** et **Rapatriement**. Voir [Instances et rôles](offsite-recovery.md#roles).
- Un lien vers la page Instances, avec les éléments activés.

## Appairage {#pairing}

**Appairage** avec **Générer une phrase** ou **Saisir une phrase**, les cartes d'étapes, le nom de cette instance et les **Adresses connues**, la carte **Relais** avec ses quatre formes et **Ce que voit le relais**, les membres du groupe, et **Serveur ZFS sans BombVault** pour une cible de réplique via SSH. Voir [Appairage des instances](offsite-recovery.md#pairing) et [Envoyer vers un serveur ZFS](zfs-datasets.md#replica-push).

## Intégrations {#integrations}

- **Serveur MCP** : des clés pour les assistants IA, **Choisir un client** et le journal de chaque clé. Voir [Serveur MCP](mcp.md).
- **Jetons d'API** : des jetons nommés pour les scripts et les tableaux de bord. Voir [Jetons](api.md#tokens).
- **Services** : **Home Assistant** via MQTT, **Trouver sur le réseau** (mDNS sous le nom `bombvault.local`) et **Exposer /metrics** pour Prometheus, avec un **Jeton de scrape (facultatif)**. Voir [Home Assistant](api.md#home-assistant) et [Trouver BombVault sur le réseau](api.md#mdns).
- **Widget de tableau de bord** : un jeton en lecture seule et une adresse pour tout tableau de bord qui affiche une iframe, comme Homepage ou Heimdall, avec seulement le journal d'activité en direct. Voir [Widget de tableau de bord](api.md#dashboard-widget).
- **SSH de l'hôte** : la clé publique de BombVault et **Tester la connexion**, pour les sauvegardes de VM et de jeux de données ZFS et les notifications Unraid. Voir le [guide des sauvegardes de VM](vm-backup-ssh-setup.md).

## Applications {#apps}

- **Application Android** : l'APK de la version que votre serveur exécute avec un code QR, et la fiche F-Droid. Voir [Application Android](android.md#install).
- **ParleyPort** : le conteneur de relais avec son modèle Unraid et la commande Docker qui le démarre.
- **BombVault Widget** : une tuile sur le tableau de bord d'Unraid avec le journal d'activité de BombVault et la prochaine exécution. Avec une connexion SSH vers l'hôte, la carte installe et supprime le plugin elle-même. Sans elle, elle vous donne l'adresse du `.plg` à installer sous **Plugins, Install Plugin**.

## Système {#system}

- **Exporter / importer les paramètres** : toute votre configuration comme fichier JSON portable, avec ou sans les identifiants. L'import montre d'abord **Ce fichier contient** et omet les liens vers des destinations qui n'existent plus. **Chiffrer les exports avec age** scelle les exports simples, le ZIP de la clé flash et le kit de récupération pour les **Destinataires age** que vous listez. Voir [Réglages portables](configuration.md#portable-settings-export-and-import).
- **Performances** : **Threads CPU (0 = tous)**, **Limite de taille du cache (Mo, 0 = illimité)** pour le cache de restic sous `/config`, et **Limite de mémoire par sauvegarde (MiB, 0 = aucune)**, une limite souple qui fait ralentir une grosse sauvegarde au lieu de la faire tuer faute de mémoire.

## Info {#info}

- **Version** : BombVault, GlimStone, restic et rclone, avec **Rechercher des mises à jour** et **Nouveau**, qui compte les changements que vous n'avez pas encore vus et ouvre une fenêtre qui les liste.
- **Aide** : **Manuel** ouvre cette documentation, et **Rapport de bug** télécharge un ZIP expurgé pour un ticket GitHub. Il demande un mot de passe de connexion.
- **À propos de BombVault** : le projet, sa licence et comment joindre l'auteur.
