# Notifications

BombVault peut vous prévenir quand une sauvegarde échoue, quand elle réussit, quand l'une est en retard, et une fois par semaine pour faire le point. Tout se règle sous **Paramètres, Notifications** : ce qui déclenche un message dans la carte **Notifications**, où vont les messages dans la carte **Canaux**, et les pings de contrôle dans la carte **Healthchecks**.

![Paramètres, Notifications avec le canal Gotify rempli.](assets/screenshots/notifications/gotify-dark.png#only-dark)
![Paramètres, Notifications avec le canal Gotify rempli.](assets/screenshots/notifications/gotify-light.png#only-light)

## Ce qui envoie un message {#what}

La carte **Notifications** fixe les règles pour tous les canaux à la fois :

- **Notifier** : **Jamais**, **Uniquement en cas d'échec** ou **En cas de succès et d'échec**, pour tous les canaux ci-dessous. Une nouvelle installation démarre sur **Uniquement en cas d'échec**, si bien qu'un canal que vous remplissez vous prévient quand une sauvegarde échoue.
- **Résumer les exécutions planifiées** : un message par exécution planifiée, par exemple *42 sur 45 réussies*, au lieu d'un par conteneur ou par VM. Une sauvegarde que vous lancez à la main envoie quand même son propre message. Un invité figé, une rétention échouée ou un budget de croissance dépassé partent tout de suite, même avec les résumés activés.
- **Notifier lors de la mise à jour d'un container** : un message pour chaque conteneur que **Mettre à jour les conteneurs après une sauvegarde réussie** a fait passer à une image plus récente, pour que vous puissiez vérifier qu'il fonctionne encore.
- **Signaler les sauvegardes en retard** : une fois par jour à 09:00, BombVault vérifie si une sauvegarde activée est en retard, c'est-à-dire plus ancienne que le double de son planning, et envoie un message par cas, qui nomme l'élément en retard. La prochaine sauvegarde réussie l'efface.
- **Notifications Unraid** : envoie vers le système de notifications propre à Unraid, qui peut transmettre à Pushover, par e-mail ou à Discord. Cela passe par la connexion SSH de **Paramètres, Intégrations, SSH de l'hôte**, cette clé doit donc être autorisée. Sur TrueNAS et d'autres hôtes, c'est désactivé.
- **Résumé hebdomadaire** : un récapitulatif par semaine avec le nombre d'exécutions, les nouvelles données sauvegardées, les sections en retard, l'état d'actualité des copies hors site et les principaux échecs. Son plan est un bouton comme tous les autres plans, et **Désactivé** le coupe. Voir [Plannings](schedules.md#plans).

Les anomalies qui envoient un message se règlent sous **Paramètres, Vérification**, voir [Anomalies](anomalies.md). Une sauvegarde reportée envoie un message quand elle commence à attendre et un quand l'attente se termine, pas un par essai.

**Envoyer un test**, en bas de la carte, envoie un message de test par chaque canal activé, avec les valeurs telles qu'elles sont dans le formulaire, et nomme chaque canal qui a échoué. Un contrôle Healthchecks reçoit une entrée dans son journal, ce qui laisse son état tel quel.

## Canaux {#channels}

Chaque canal a son propre interrupteur dans la carte **Canaux**, et ses champs apparaissent dès qu'il est activé. Chaque canal envoie de son côté avec sa propre limite de temps, si bien qu'un hôte qui ne répond jamais ne retient pas les autres. Une alerte de retard, de budget ou d'anomalie qu'aucun canal n'a prise est retentée, tout comme un résumé hebdomadaire. Le message d'une sauvegarde isolée ou d'un récapitulatif n'est envoyé qu'une fois.

| Canal | Champs | Remarques |
|---|---|---|
| **Webhook** | **URL du Webhook**, **Format du Webhook** | Formats : **Generic JSON**, **Discord**, **Slack**, **Gotify** et **ntfy**. Generic JSON envoie `title`, `message` et `ok`. |
| **Apprise** | **URL de l'API Apprise**, **Tags (optionnel)** | Pointez-le vers le point d'accès `/notify/<key>` de votre conteneur Apprise API pour atteindre ses plus de 100 services. Les tags choisissent un sous-ensemble des cibles de cette clé. |
| **Matrix** | **URL du Homeserver**, **Jeton d'accès**, **ID du salon** | Le jeton reste sur le serveur et n'est plus jamais affiché. |
| **Gotify** | **URL du serveur**, **Jeton de l'application**, **Priorité en cas d'échec**, **Priorité en cas de réussite** | Les priorités vont de 0 à 10 et valent 8 par défaut pour les échecs et 5 pour les réussites. |
| **E-mail (SMTP)** | **Hôte SMTP**, **Port**, **Chiffrement**, **Nom d'utilisateur**, **Mot de passe**, **Adresse d'expéditeur**, **Adresse de destinataire** | **Chiffrement** vaut **STARTTLS**, **TLS (implicit)** ou **Aucun**. |

Les secrets comme le jeton Matrix, le jeton d'application Gotify et le mot de passe SMTP sont stockés chiffrés. Sous un secret enregistré, le champ indique **Enregistré. Laissez le champ vide pour le conserver.** Quand vous changez l'adresse du serveur, BombVault n'envoie pas l'ancien secret au nouvel endroit : le champ le redemande, et la nouvelle adresse est enregistrée dès que vous le saisissez.

## Healthchecks {#healthchecks}

Un service comme [Healthchecks.io](https://healthchecks.io) donne l'alerte quand BombVault cesse de se manifester. Collez l'**URL de ping** du contrôle, et BombVault la pingue pendant tout le cycle de chaque sauvegarde : au début, puis en cas de succès ou avec `/fail`. Il le fait dès qu'une URL est définie, quel que soit le réglage **Notifier**, si bien que le contrôle reste vert en cas de succès même avec des messages uniquement en cas d'échec. Une section qui n'a nulle part où sauvegarder compte comme en échec, si bien que son contrôle passe au rouge au lieu de recevoir un succès.

Les sections sur la même URL partagent un seul contrôle. Dès qu'une sauvegarde de l'une a échoué, les sauvegardes réussies des autres pinguent aussi `/fail` et nomment l'élément en échec, si bien que le contrôle reste rouge jusqu'à ce que cet élément ait de nouveau une sauvegarde réussie ou que vous accusiez réception de l'échec sous **À traiter**.

Une section peut pinguer une adresse à elle : **Exception** dans la carte en ajoute une, et la ligne indique **Toutes les autres sections envoient un ping à cette URL.**

## Exemple : Gotify sur votre téléphone {#example-gotify}

Vous faites tourner Gotify sur `https://gotify.example.com` et vous voulez que les échecs sonnent sur votre téléphone pendant que les réussites arrivent discrètement.

1. Dans Gotify, ouvrez **Apps**, créez une application appelée `BombVault` et copiez son jeton.
2. Dans BombVault, ouvrez **Paramètres, Notifications**. Dans **Notifications**, réglez **Notifier** sur **En cas de succès et d'échec** et activez **Résumer les exécutions planifiées**, pour qu'une nuit de 45 sauvegardes envoie un message au lieu de 45.
3. Dans **Canaux**, activez **Gotify**.
4. Saisissez `https://gotify.example.com` comme **URL du serveur**. BombVault ajoute `/message` lui-même.
5. Collez le jeton dans **Jeton de l'application**.
6. Laissez **Priorité en cas d'échec** à 8 et **Priorité en cas de réussite** à 5. L'application Android de Gotify traite 8 et plus comme élevé, seuls les échecs font donc du bruit.
7. Appuyez sur **Envoyer un test**. La ligne sous la carte dit si Gotify l'a pris, et un jeton refusé est nommé comme tel.

La prochaine exécution planifiée envoie un résumé à la priorité 5, ou à 8 si quelque chose y a échoué.

## Exemple : ntfy comme webhook {#example-ntfy}

ntfy n'a pas besoin de compte pour un sujet public. Choisissez un nom de sujet difficile à deviner, car toute personne qui le connaît peut le lire.

1. Dans **Canaux**, activez **Webhook**.
2. Saisissez l'adresse du sujet comme **URL du Webhook**, par exemple `https://ntfy.sh/bombvault-tower-7f3k`, ou l'adresse de votre propre serveur ntfy.
3. Réglez **Format du Webhook** sur **ntfy**.
4. Abonnez-vous au même sujet dans l'application ntfy et appuyez sur **Envoyer un test**.

BombVault envoie le message comme corps avec son titre dans `X-Title`. Un échec part avec une priorité haute et l'étiquette `warning`, il ressort donc dans l'application.

## Quand les messages n'arrivent pas {#troubleshooting}

Voir [Les notifications n'arrivent pas](troubleshooting.md#notifications-do-not-arrive) dans Dépannage.
