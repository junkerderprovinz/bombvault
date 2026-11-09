# Notifications

BombVault peut vous prévenir quand une sauvegarde échoue, quand elle réussit, quand l'une est en retard, et une fois par semaine pour faire le point. Tout se règle sous **Paramètres, Notifications** : ce que BombVault envoie en plus des sauvegardes isolées dans la carte **Notifications**, et où vont les messages dans la carte **Canaux**. Chaque canal décide lui-même quand il se manifeste, et un contrôle Healthchecks est aussi un canal.

![Paramètres, Notifications avec le canal Gotify rempli.](assets/screenshots/notifications/gotify-dark.png#only-dark)
![Paramètres, Notifications avec le canal Gotify rempli.](assets/screenshots/notifications/gotify-light.png#only-light)

## Ce qui envoie un message {#what}

Les interrupteurs de la carte **Notifications** valent pour tous les canaux :

- **Résumer les exécutions planifiées** : un message par exécution planifiée, par exemple *42 sur 45 réussies*, au lieu d'un par conteneur ou par VM. Une sauvegarde que vous lancez à la main envoie quand même son propre message. Un invité figé, une rétention échouée ou un budget de croissance dépassé partent tout de suite, même avec les résumés activés.
- **Notifier lors de la mise à jour d'un container** : un message pour chaque conteneur que **Mettre à jour les conteneurs après une sauvegarde réussie** a fait passer à une image plus récente, pour que vous puissiez vérifier qu'il fonctionne encore.
- **Signaler les sauvegardes en retard** : une fois par jour à 09:00, BombVault vérifie si une sauvegarde activée est en retard, c'est-à-dire plus ancienne que le double de son planning, et envoie un message par cas, qui nomme l'élément en retard. La prochaine sauvegarde réussie l'efface.
- **Résumé hebdomadaire** : un récapitulatif par semaine avec le nombre d'exécutions, les nouvelles données sauvegardées, les sections en retard, l'état d'actualité des copies hors site et les principaux échecs. Son plan est un bouton comme tous les autres plans, et **Désactivé** le coupe. Voir [Plannings](schedules.md#plans).

C'est sur chaque canal, sous **Notifier**, que vous choisissez s'il signale aussi les réussites. Les anomalies qui envoient un message se règlent sous **Paramètres, Vérification**, voir [Anomalies](anomalies.md). Une sauvegarde reportée envoie un message quand elle commence à attendre et un quand l'attente se termine, pas un par essai.

## Canaux {#channels}

**Ajouter un canal**, en bas de la carte **Canaux**, propose **Notifications Unraid**, **ntfy**, **Gotify**, **Discord**, **Slack**, **Matrix**, **E-mail (SMTP)**, **Apprise**, **Webhook** et **Healthchecks**. Ajoutez-en autant que vous voulez, y compris deux du même type, par exemple un sujet ntfy pour les échecs et un autre pour tout. Un nouveau canal démarre désactivé, si bien qu'il est enregistré avant que ses champs soient remplis.

Chaque canal a :

- son propre interrupteur, qui le coupe sans perdre ses champs.
- **Notifier** : **Jamais**, **Uniquement en cas d'échec** ou **En cas de succès et d'échec**. Un nouveau canal démarre sur **Uniquement en cas d'échec**. Une sauvegarde réussie qui a laissé quelque chose de côté atteint aussi un canal réservé aux échecs.
- **Nom**, facultatif, qui s'affiche à la place du type, pratique quand vous en avez deux du même type.
- **Envoyer un test**, qui envoie un message de test par ce canal avec les valeurs telles qu'elles sont dans le formulaire, enregistrées ou non. La ligne sous le canal dit si le service l'a pris, et sinon pourquoi.
- **Retirer le canal**, la corbeille à côté.

Chaque canal envoie de son côté avec sa propre limite de temps, si bien qu'un hôte qui ne répond jamais ne retient pas les autres. Quand un service échoue d'une façon qu'un nouvel essai pourrait régler, le canal essaie jusqu'à trois fois. Une alerte de retard, de budget ou d'anomalie qu'aucun canal n'a prise est retentée plus tard, tout comme un résumé hebdomadaire.

| Canal | Champs | Remarques |
|---|---|---|
| **Notifications Unraid** | aucun | Envoie vers le système de notifications propre à Unraid, qui peut transmettre à Pushover, par e-mail ou à Discord. Cela passe par la connexion SSH de **Paramètres, Intégrations, SSH de l'hôte**, cette clé doit donc être autorisée. Sur TrueNAS et d'autres hôtes, il reste muet, et le canal dit pourquoi. |
| **ntfy** | **URL du sujet**, **Jeton d'accès (facultatif)** | L'adresse se termine par votre sujet. Un jeton n'est utile que pour un sujet protégé. |
| **Gotify** | **URL du serveur**, **Jeton de l'application**, **Priorité en cas d'échec**, **Priorité en cas de réussite** | Les priorités vont de 0 à 10 et valent 8 par défaut pour les échecs et 5 pour les réussites. |
| **Discord** | **URL du Webhook** | L'adresse d'un webhook tirée des paramètres du salon Discord. |
| **Slack** | **URL du Webhook** | L'adresse d'un webhook entrant de votre application Slack. |
| **Matrix** | **URL du Homeserver**, **Jeton d'accès**, **ID du salon** | Le compte derrière le jeton doit être dans le salon. |
| **E-mail (SMTP)** | **Hôte SMTP**, **Port**, **Chiffrement**, **Nom d'utilisateur**, **Mot de passe**, **Adresse d'expéditeur**, **Adresse de destinataire** | **Chiffrement** vaut **STARTTLS**, **TLS (implicit)** ou **None**. |
| **Apprise** | **URL de l'API Apprise**, **Tags (optionnel)**, **Adresses Apprise (facultatif)** | Pointez-le vers le point d'accès `/notify/<key>` de votre conteneur Apprise API pour atteindre ses plus de 100 services, et les tags choisissent un sous-ensemble des cibles de cette clé. Ou listez des adresses Apprise, une par ligne, par exemple `tgram://` pour Telegram ou `pover://` pour Pushover, et terminez l'URL de l'API par `/notify`, sans clé. |
| **Webhook** | **URL du Webhook**, **Méthode**, **En-têtes**, **Corps** | Une requête que vous composez vous-même, voir [Webhook](#webhook). |
| **Healthchecks** | **URL de ping** | Voir [Healthchecks](#healthchecks). |

Les secrets comme les jetons, le mot de passe SMTP et les valeurs des en-têtes de webhook sont stockés chiffrés, et le serveur ne les renvoie jamais à la page. Sous un secret enregistré, le champ indique **Enregistré. Laissez le champ vide pour le conserver.** Quand vous changez la destination d'un canal, BombVault n'envoie pas l'ancien secret au nouvel endroit : le champ indique **La nouvelle adresse est enregistrée dès que vous saisissez ceci à nouveau.**, et la nouvelle adresse est enregistrée dès que vous le saisissez.

Les réglages d'une version antérieure à la liste de canaux sont repris au premier démarrage, et rien ne se perd. Chaque canal que vous aviez rempli devient un élément à part avec le choix **Notifier** que vous aviez. Un webhook au format Discord, Slack, Gotify ou ntfy devient un canal de ce type, et un webhook Generic JSON continue d'envoyer `title`, `message` et `ok` par son **Corps**. L'interrupteur Unraid devient un canal Unraid, et l'URL Healthchecks avec ses exceptions un canal Healthchecks.

### Webhook {#webhook}

Un canal **Webhook** envoie une requête HTTP vers n'importe quelle adresse. **Méthode** vaut GET, POST ou PUT. **Ajouter un en-tête** ajoute une ligne sous **En-têtes**, et les valeurs des en-têtes sont conservées comme des secrets, un en-tête `Authorization` y est donc à l'abri. **Corps** est envoyé tel que vous l'écrivez, avec ces variables remplies :

| Variable | Devient |
|---|---|
| `%%title%%` | le titre du message |
| `%%body%%` | le message lui-même |
| `%%ok%%` | `true`, ou `false` en cas d'échec |
| `%%event%%` | ce qui s'est passé, par exemple `container.failed` |
| `%%severity%%` | `info`, `success`, `warning`, `failure` ou `critical` |
| `%%importance%%` | le niveau qu'Unraid lui donne : `normal`, `warning` ou `alert` |
| `%%link%%` | un lien qui accompagne le message, vide s'il n'y en a pas |
| `%%key%%` | la section, par exemple `container` ou `VM` |
| `%%time%%` | l'heure en UTC, par exemple `2026-11-09T03:00:00Z` |
| `%%source%%` | `BombVault` |

Les variables fonctionnent aussi dans l'adresse et dans les valeurs d'en-tête, et chaque valeur est échappée selon sa place : en JSON quand l'en-tête `Content-Type` indique du JSON, et comme paramètre de requête dans l'adresse. Un nouveau webhook démarre avec cet en-tête et envoie `{"title":"%%title%%","message":"%%body%%","ok":%%ok%%}`. Un nom que BombVault ne connaît pas reste tel quel, si bien qu'une faute de frappe se voit dans le message. Laissez **Corps** vide pour n'en envoyer aucun.

## Healthchecks {#healthchecks}

Un service comme [Healthchecks.io](https://healthchecks.io) donne l'alerte quand BombVault cesse de se manifester. Ajoutez un canal **Healthchecks** et collez l'**URL de ping** du contrôle, et BombVault la pingue pendant tout le cycle de chaque sauvegarde : au début, puis en cas de succès ou avec `/fail`. Ce canal n'a pas de choix **Notifier**, car un contrôle a besoin de ses pings de réussite pour rester vert. Une section qui n'a nulle part où sauvegarder compte comme en échec, si bien que son contrôle passe au rouge au lieu de recevoir un succès.

Les sections sur la même URL partagent un seul contrôle. Dès qu'une sauvegarde de l'une a échoué, les sauvegardes réussies des autres pinguent aussi `/fail` et nomment l'élément en échec, si bien que le contrôle reste rouge jusqu'à ce que cet élément ait de nouveau une sauvegarde réussie ou que vous accusiez réception de l'échec sous **À traiter**.

Une section peut pinguer une adresse à elle : **Exception** dans le canal Healthchecks en ajoute une, et la ligne indique alors **Toutes les autres sections envoient un ping à cette URL.** **Envoyer un test** écrit une entrée dans le journal de chaque contrôle, ce qui laisse son état tel quel.

## Exemple : Gotify sur votre téléphone {#example-gotify}

Vous faites tourner Gotify sur `https://gotify.example.com` et vous voulez que les échecs sonnent sur votre téléphone pendant que les réussites arrivent discrètement.

1. Dans Gotify, ouvrez **Apps**, créez une application appelée `BombVault` et copiez son jeton.
2. Dans BombVault, ouvrez **Paramètres, Notifications** et activez **Résumer les exécutions planifiées**, pour qu'une nuit de 45 sauvegardes envoie un message au lieu de 45.
3. Dans **Canaux**, appuyez sur **Ajouter un canal** et choisissez **Gotify**.
4. Saisissez `https://gotify.example.com` comme **URL du serveur**. BombVault ajoute `/message` lui-même.
5. Collez le jeton dans **Jeton de l'application**.
6. Réglez **Notifier** sur **En cas de succès et d'échec**. Laissez **Priorité en cas d'échec** à 8 et **Priorité en cas de réussite** à 5. L'application Android de Gotify traite 8 et plus comme élevé, seuls les échecs font donc du bruit.
7. Activez le canal et appuyez sur **Envoyer un test**. La ligne sous le canal dit si Gotify l'a pris, et un jeton refusé est nommé comme tel.

La prochaine exécution planifiée envoie un résumé à la priorité 5, ou à 8 si quelque chose y a échoué.

## Exemple : ntfy sans compte {#example-ntfy}

ntfy n'a pas besoin de compte pour un sujet public. Choisissez un nom de sujet difficile à deviner, car toute personne qui le connaît peut le lire.

1. Dans **Canaux**, appuyez sur **Ajouter un canal** et choisissez **ntfy**.
2. Saisissez l'adresse du sujet comme **URL du sujet**, par exemple `https://ntfy.sh/bombvault-tower-7f3k`, ou un sujet sur votre propre serveur ntfy. Laissez **Jeton d'accès (facultatif)** vide pour un sujet public.
3. Abonnez-vous au même sujet dans l'application ntfy, activez le canal et appuyez sur **Envoyer un test**.

BombVault envoie le message comme corps avec son titre dans l'en-tête `Title`. Un échec part avec une priorité haute et l'étiquette `warning`, il ressort donc dans l'application, et une VM restée figée part en urgent.

## Quand les messages n'arrivent pas {#troubleshooting}

Voir [Les notifications n'arrivent pas](troubleshooting.md#notifications-do-not-arrive) dans Dépannage.
