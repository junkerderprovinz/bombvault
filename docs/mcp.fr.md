# Serveur MCP

BombVault intègre un serveur pour le Model Context Protocol (MCP), le protocole par lequel les assistants IA comme Claude Code et Claude Desktop accèdent à des outils externes. Par ce biais, un assistant peut lire l'état de tes sauvegardes et, si tu le permets, lancer une sauvegarde ou annuler une sauvegarde qu'il a lancée. Le serveur reste éteint tant que tu n'as pas créé de clé ni activé la [connexion par OAuth](#oauth) : jusque-là, le point de terminaison `/mcp` répond `404` à tout.

## Ce qu'un assistant peut faire et ne peut pas faire {#tools}

| Outil | Ce qu'il fait | Type |
|---|---|---|
| `get_health` | Version, nom de l'instance, si une sauvegarde est en cours et ce que cette clé a le droit de faire | lecture |
| `get_status` | État de protection par domaine : dernière sauvegarde réussie, intervalle attendu, vérifications et contrôles hors site, prochaines exécutions planifiées, sauvegardes qui attendent une application au repos | lecture |
| `get_coverage` | Ce que BombVault protège et ce qu'il ne protège pas, avec la raison pour chaque élément | lecture |
| `list_items` | Chaque conteneur, VM, ensemble de dossiers protégé, la clé USB flash et la configuration de l'application, avec sa planification, ce qu'une sauvegarde arrête, sa dernière sauvegarde et sa durée ; les conteneurs de base de données indiquent aussi leur dernier dump ; les datasets ZFS y figurent aussi, avec le résultat de leur dernière vérification | lecture |
| `list_runs` | Historique des exécutions, les plus récentes d'abord, filtrable par domaine, élément, statut, type et date | lecture |
| `list_restore_points` | Points de restauration d'un élément dans son dépôt principal, et pour un conteneur ses dumps de base de données ; un dataset ZFS a un point de restauration par sauvegarde, avec un snapshot de chaque dataset en dessous | lecture |
| `get_activity` | Ce qui tourne en ce moment, avec la phase et le pourcentage | lecture |
| `get_storage_stats` | Historique de taille du dépôt principal d'un domaine et sa croissance par semaine | lecture |
| `list_anomalies` | Les anomalies repérées par BombVault dans les sauvegardes, filtrables par état, gravité et domaine, avec un résumé de ce qui est ouvert | lecture |
| `get_anomaly` | Une de ces anomalies, avec la note laissée lors de sa prise en compte | lecture |
| `start_backup` | Sauvegarde un élément tout de suite | lancement |
| `start_domain_backup` | Sauvegarde chaque élément protégé d'un domaine | lancement |
| `start_backup_everything` | Lance la passe Backup Everything | lancement |
| `cancel_backup` | Annule une sauvegarde en cours que cette clé a lancée | annulation |

Tout ceci reste dans l'interface web : les restaurations de toute sorte (y compris télécharger, enregistrer ou importer un dump de base de données), la suppression de sauvegardes, prune, unlock, les vérifications et les exercices, la réplication hors site, les paramètres, les identifiants et les clés MCP, ainsi que l'annulation d'une sauvegarde lancée par la planification, par l'interface web ou par une autre clé. Il en va de même pour prendre acte d'une anomalie ou la marquer comme attendue, ce qui se fait sur la page **Anomalies**. La raison : les réponses des outils contiennent des noms et des messages d'erreur venant de votre serveur, et n'importe lequel d'entre eux peut contenir un texte écrit pour manipuler l'assistant. Un assistant qui s'y laisse prendre peut au pire lancer une sauvegarde dans les limites ci-dessous, ou annuler une sauvegarde qu'il a lancée lui-même.

Si le dépôt principal d'un élément est distant (S3, REST, SFTP, rclone), `list_restore_points` le contacte, et l'appel peut prendre un moment. Les copies hors site ne peuvent pas être listées par MCP. Ce que regardent les contrôles d'anomalies est décrit dans [Fonctionnalités](features.md), et la façon dont un élément ZFS garde un instantané par jeu de données, dans [Jeux de données ZFS](zfs-datasets.md#contents).

## Ce que fait une sauvegarde lancée {#starting-backups}

La sauvegarde d'un assistant est la même que celle que lance l'interface web. Un conteneur en marche est arrêté jusqu'à la fin de sa sauvegarde, avec les conteneurs configurés pour s'arrêter en même temps. Une VM avec la méthode « graceful » est éteinte puis redémarrée. Un dataset ZFS arrête les conteneurs configurés pour lui pendant la prise de son snapshot. Les ensembles de dossiers, la clé flash et la configuration continuent de tourner. Ensuite, BombVault applique la politique de rétention et peut copier vers le dépôt hors site. `list_items` indique à l'assistant ce qu'un élément arrête et combien de temps a duré sa dernière sauvegarde, et les descriptions des outils lui demandent de vous le dire avant de lancer quoi que ce soit.

Comme une sauvegarde arrête des services et fait sortir d'anciens points de restauration, les lancements par MCP sont limités :

- 12 sauvegardes lancées par heure et par clé.
- 15 minutes entre deux lancements MCP du même élément, du même domaine ou de Backup Everything.
- Au plus 4 lancements MCP du même élément sur 24 heures.
- **Garde de rétention.** Quand un domaine conserve un nombre fixe de points de restauration (uniquement « garder les N derniers », sans règle quotidienne, hebdomadaire ou mensuelle, en local ou sur une destination hors site), chaque nouvelle sauvegarde fait sortir la plus ancienne. BombVault refuse alors un lancement MCP d'un élément dont les N-1 dernières sauvegardes réussies ont toutes été lancées par MCP. Au moins un point de restauration créé par la planification ou par vous reste donc toujours dans l'ensemble conservé. Avec « garder le dernier » (N = 1), un assistant ne peut pas du tout sauvegarder cet élément. La prochaine sauvegarde planifiée refait de la place.

Un lancement de domaine ou de Backup Everything laisse de côté les éléments retenus par une limite et les nomme dans sa réponse. L'interface web et la planification ne sont concernées par aucune de ces limites. Le quota horaire est gardé en mémoire, un redémarrage de BombVault le remet donc à zéro.

## Activer le serveur {#switch-on}

1. Ouvre **Paramètres, Système, Serveur MCP** et clique sur le bouton de ton client. Un client qui n'est pas dans la liste se connecte via **Autre client**.
2. Sous **Clé**, garde **Nouvelle clé** et le nom proposé, celui du client, ou saisis un nom qui dit où la clé sert, par exemple « Claude Code sur le portable ». Une clé par client permet d'en révoquer une sans toucher aux autres. **Clé existante** donne au client une clé que tu as créée avant.
3. Active **Autoriser le lancement de sauvegardes** pour une clé qui doit pouvoir lancer des sauvegardes ; sans cela, elle peut seulement lire. Tu peux le changer plus tard sur la tuile de la clé, et le changement vaut dès la requête suivante de l'assistant, sans reconnexion.
4. Clique sur **Créer la clé**. La clé s'affiche une seule fois. BombVault n'en garde qu'une empreinte et ne peut pas la remontrer, alors copie-la maintenant. Si tu fermes la boîte de dialogue avant que le client ait utilisé la clé, la carte continue de l'afficher jusqu'à ce que tu confirmes l'avoir copiée.

Sans mot de passe de connexion, l'interface web elle-même est ouverte à tout votre réseau, et quiconque peut l'ouvrir peut aussi créer une clé. La carte le signale. Si vous ouvrez BombVault sous un nom qui a l'air public (par exemple `bombvault.example.com` derrière un reverse proxy) et qu'aucun mot de passe de connexion n'est défini, aucune clé ne peut être créée ni remplacée depuis cette adresse, pour qu'aucune page web sur Internet ne puisse amener votre navigateur à en créer une. Définissez un mot de passe de connexion, ou ouvrez BombVault par son adresse IP ou par un nom local comme `tower` ou `tower.local`.

## Vos clés et leur journal {#keys}

Chaque clé a sa propre tuile sur la carte. Elle affiche le nom de la clé, si elle peut lancer des sauvegardes ou seulement lire, les quatre derniers caractères de la clé, quand elle a été créée ou remplacée pour la dernière fois, quand un client l'a utilisée pour la dernière fois et combien d'appels elle a faits aujourd'hui. Sur la tuile, vous renommez la clé, changez son autorisation, la remplacez ou la révoquez. Une clé révoquée passe dans la liste des clés révoquées, où vous pouvez la supprimer définitivement dès qu'aucune exécution de l'historique ne la nomme.

À côté de son nom, la tuile montre le logo du client pour lequel la clé a été créée. Une clé créée via **Autre client**, ou avant que la carte liste les clients, montre une clé à la place.

**Journal** sur une tuile ouvre ce que cette clé a fait. D'abord les sauvegardes qu'elle a lancées, chacune avec son état et un lien vers cette exécution dans le journal d'activité du tableau de bord. En dessous, ses appels, les plus récents d'abord, avec l'outil et ce qu'est devenu l'appel. Un refus dit pourquoi : la clé peut seulement lire, la protection de rétention a retenu la sauvegarde, une autre sauvegarde était déjà en cours, l'élément a été sauvegardé via MCP il y a quelques minutes, ou la clé a envoyé trop de requêtes. Une annulation renvoie vers l'exécution concernée.

BombVault garde les entrées de chaque clé pendant 30 jours au plus : les 500 lancements et annulations réussis les plus récents et, à côté, les 200 autres appels les plus récents (lectures, refus et erreurs). Un assistant qui interroge sans cesse une sauvegarde en cours, ou qui répète un appel refusé, ne peut donc pas faire sortir son lancement du journal. Pour chaque appel, il enregistre l'outil, le résultat et l'exécution nommée par une annulation. Il n'enregistre jamais ce que l'assistant a envoyé, ni la clé ou son empreinte. Le paquet de diagnostic ne fait que compter les entrées, et un export des réglages les laisse de côté.

## Connecter un client {#clients}

Chaque client a un bouton sur la carte, sous **Sur cet ordinateur** ou **Dans le cloud**. Le bouton ouvre une boîte de dialogue en trois étapes : la clé ; la configuration pour ce client, avec l'adresse à laquelle tu as ouvert la carte, un bouton pour la copier, l'endroit où se trouve la configuration et, avec le certificat propre de BombVault, ce dont le client a besoin pour lui faire confiance ; puis l'attente du premier appel du client. La boîte de dialogue surveille la dernière utilisation de la clé et passe au vert quand cet appel arrive.

La boîte de dialogue garde la clé hors de toute ligne de commande. Quand le client sait la lire dans une variable d'environnement (`BOMBVAULT_MCP_KEY`), une invite masquée ou un fichier à lui, la configuration ne fait que la nommer. Quand le client ne le sait pas, la clé se trouve dans son fichier de configuration ou ses réglages, et la boîte de dialogue le dit. Quand la documentation d'un client ne dit pas comment il traite un certificat inconnu, la boîte de dialogue écrit cette étape comme ce qu'il faut faire si le client refuse le certificat de BombVault.

| Client | Configuration | D'où vient la clé |
|---|---|---|
| AnythingLLM | fichier de configuration | le fichier de configuration |
| Antigravity | fichier de configuration | variable d'environnement |
| Claude Code | commande | fichier de clé |
| Claude Desktop | fichier de configuration | fichier de clé |
| Cline | fichier de configuration | le fichier de configuration |
| Codex CLI | fichier de configuration | variable d'environnement |
| Continue | fichier de configuration | `~/.continue/.env` |
| Copilot CLI | fichier de configuration | le fichier de configuration |
| Cursor | fichier de configuration | variable d'environnement |
| Gemini CLI | fichier de configuration | variable d'environnement |
| GitHub Copilot (VS Code) | fichier de configuration | invite masquée |
| Goose | fichier de configuration | variable d'environnement |
| Jan | formulaire dans l'app | les réglages de l'app |
| JetBrains (AI Assistant, Junie) | fichier de configuration | le fichier de configuration |
| Kimi Code | fichier de configuration | le fichier de configuration |
| LM Studio | fichier de configuration | le fichier de configuration |
| Mistral Vibe | fichier de configuration | variable d'environnement |
| Msty | formulaire dans l'app | les réglages de l'app |
| n8n | formulaire dans l'app | les identifiants de n8n |
| Open WebUI | formulaire dans l'app | les réglages de l'app |
| opencode | fichier de configuration | variable d'environnement |
| Perplexity (Mac) | formulaire dans l'app | fichier de clé |
| Qwen Code | fichier de configuration | variable d'environnement |
| Roo Code | fichier de configuration | variable d'environnement |
| Visual Studio | fichier de configuration | le fichier de configuration |
| Warp | fichier de configuration | le fichier de configuration |
| Windsurf | fichier de configuration | variable d'environnement |
| Zed | fichier de configuration | le fichier de configuration |
| Grok | formulaire, dans le cloud | les serveurs de l'éditeur |
| Le Chat | formulaire, dans le cloud | les serveurs de l'éditeur |
| ChatGPT | connexion par OAuth, dans le cloud | un jeton d'accès, voir [plus bas](#oauth) |
| Claude (claude.ai) | connexion par OAuth, dans le cloud | un jeton d'accès, voir [plus bas](#oauth) |

Les sections ci-dessous expliquent plus en détail la configuration de Claude Code et de Claude Desktop et listent ce dont tout autre client a besoin.

### Claude Code {#claude-code}

Claude Code atteint BombVault par `mcp-remote`, qui a besoin de Node.js sur l'ordinateur. Enregistrez d'abord la clé dans un fichier texte à part, sur une seule ligne :

```text
X-API-Key: <your key>
```

Exécutez ensuite une fois dans un terminal la commande de la carte, avec le chemin de ce fichier. Derrière un certificat auquel votre ordinateur fait confiance, elle ressemble à ceci :

```bash
claude mcp add bombvault --scope user -- npx -y mcp-remote@latest https://bombvault.example.com/mcp --header-file "<path of the file with your key>"
```

Avec le certificat propre à BombVault (voir [TLS et certificats](#tls)), la commande indique en plus à Node.js le certificat téléchargé :

```bash
claude mcp add bombvault --scope user -e "NODE_EXTRA_CA_CERTS=<path of the downloaded bombvault-cert.pem>" -- npx -y mcp-remote@latest https://192.168.1.10:3443/mcp --header-file "<path of the file with your key>"
```

Vérifiez la connexion avec `/mcp` dans Claude Code. `--scope user` rend BombVault disponible dans tous vos projets. Claude Code ne garde que le chemin du fichier de clé, si bien que la clé n'apparaît ni dans la commande et l'historique de votre shell, ni dans la liste des processus. Rangez le fichier à un endroit que vous seul pouvez lire, et en dehors de tout dossier que vous committez. `@latest` fait télécharger par `npx` un `mcp-remote` récent ; sans cela, une version plus ancienne installée globalement serait utilisée, et elle ne connaît pas `--header-file`.

N'écrivez pas `${BOMBVAULT_MCP_KEY}` dans les arguments de `mcp-remote` pour Claude Code. Claude Code remplace une telle référence à partir de son propre environnement avant de lancer `mcp-remote` : la clé se retrouve alors sur la ligne de commande de ce processus, où d'autres programmes et utilisateurs de l'ordinateur peuvent la lire.

Sans Node.js, et uniquement derrière un certificat auquel votre ordinateur fait confiance, Claude Code peut se connecter tout seul. Placez un fichier `.mcp.json` dans le dossier du projet :

```json
{
  "mcpServers": {
    "bombvault": {
      "type": "http",
      "url": "https://bombvault.example.com/mcp",
      "headers": {
        "Authorization": "Bearer ${BOMBVAULT_MCP_KEY}"
      }
    }
  }
}
```

Définissez `BOMBVAULT_MCP_KEY` là où Claude Code démarre, par exemple sous `"env"` dans `~/.claude/settings.json` ou dans le profil de votre shell, modifié dans un éditeur de texte plutôt que tapé à l'invite. Ici, la référence est sans risque, car Claude Code ne lance aucun second processus qui la porterait. Le certificat propre à BombVault ne fonctionne pas de cette façon : la connexion que Claude Code établit lui-même le refuse, même avec `NODE_EXTRA_CA_CERTS` défini. Ne committez jamais un `.mcp.json` dans lequel la clé est écrite en clair.

### Claude Desktop {#claude-desktop}

Claude Desktop atteint BombVault par `mcp-remote`, qui a besoin de Node.js sur l'ordinateur. Enregistrez d'abord la clé dans un fichier texte à part, sur une seule ligne, comme décrit pour [Claude Code](#claude-code). Ouvrez le fichier de configuration dans Claude Desktop par **Settings, Developer, Edit Config**. Il se trouve dans `%APPDATA%\Claude\claude_desktop_config.json` sous Windows et dans `~/Library/Application Support/Claude/claude_desktop_config.json` sous macOS. Ajoutez l'entrée de la carte dans `"mcpServers"`, à côté des serveurs déjà présents, puis redémarrez Claude Desktop :

```json
{
  "mcpServers": {
    "bombvault": {
      "command": "npx",
      "args": ["-y", "mcp-remote@latest", "https://192.168.1.10:3443/mcp", "--header-file", "<path of the file with your key>"],
      "env": {
        "NODE_EXTRA_CA_CERTS": "<path of the downloaded bombvault-cert.pem>"
      }
    }
  }
}
```

- `NODE_EXTRA_CA_CERTS` n'est là que pour le certificat propre à BombVault. Derrière un certificat auquel votre ordinateur fait déjà confiance, retirez-le.
- `--allow-http` n'est ajouté que pour une adresse `http://` simple.
- Sous Windows, écrivez les chemins avec des barres obliques normales, par exemple `C:/Users/sam/bombvault-key.txt`, car une barre oblique inverse seule n'est pas du JSON valide. Gardez le chemin du fichier de clé sans espaces : sous Windows, Claude Desktop passe à `npx` un chemin contenant un espace en deux morceaux.
- La configuration ne nomme que le fichier de clé, si bien que la clé n'apparaît ni dans celle-ci ni dans la liste des processus. Rangez le fichier à un endroit que vous seul pouvez lire.

### Clients dans le cloud {#cloud-clients}

ChatGPT, Claude sur claude.ai, Grok et Le Chat appellent BombVault depuis les serveurs de leur éditeur. BombVault doit donc être joignable depuis Internet avec un certificat reconnu publiquement, par exemple derrière un proxy inverse ; Le Chat refuse les certificats auto-signés. Une connexion sur le proxy peut protéger l'interface web, mais `/mcp` doit passer jusqu'à BombVault sans elle : ces services ne savent pas se connecter à un proxy, et BombVault vérifie lui-même leur clé ou leur jeton. Grok et Le Chat envoient une clé fixe, et leurs boutons les configurent comme les autres. ChatGPT, et Claude sur claude.ai dans la plupart des organisations, se connectent uniquement par une connexion OAuth, décrite ci-dessous.

### Connexion par OAuth {#oauth}

Pour un client qui n'accepte pas de clé, BombVault est son propre serveur d'autorisation OAuth. Le client s'enregistre lui-même, t'envoie sur une page de BombVault, et là tu te connectes avec ton mot de passe de connexion (et le second facteur, si tu en as configuré un) et tu l'autorises. Le client reçoit alors un jeton qui ne vaut que pour le point de terminaison MCP de ce BombVault, et le renouvelle tout seul.

1. Définis un mot de passe de connexion sous **Paramètres, Système**. Sans lui, BombVault ne propose aucune connexion, car personne ne pourrait donner son accord.
2. Rends BombVault joignable depuis Internet en https avec un certificat auquel les navigateurs font confiance, en général via un proxy inverse. Le client appelle `/mcp`, `/oauth/` et `/.well-known/` depuis ses propres serveurs, donc un proxy avec sa propre connexion doit laisser passer ces trois chemins jusqu'à BombVault. La page de consentement sous `/oauth/authorize` s'ouvre dans ton propre navigateur et peut rester derrière la connexion du proxy. Indique aussi le proxy dans `TRUSTED_PROXY` (voir [Configuration](configuration.md)). BombVault limite les enregistrements de clients par adresse, et sans cela chaque client semble venir du proxy.
3. Sur la carte MCP, active **Connexion par OAuth** et saisis l'**Adresse publique** : l'adresse https sans chemin, par exemple `https://backup.example.com`. Chaque jeton est lié à cette adresse, donc après un changement chaque client doit se reconnecter.
4. Clique sur le bouton de ChatGPT ou de Claude. La boîte de dialogue affiche l'**URL du connecteur**, c'est-à-dire l'adresse publique suivie de `/mcp`, et où la saisir dans ce client. Dans ChatGPT, active le mode développeur sous **Paramètres, Applications et connecteurs, Paramètres avancés**, choisis **Créer**, colle l'URL du connecteur comme URL du serveur MCP et choisis OAuth comme authentification. Sur claude.ai, ouvre **Paramètres, Connecteurs, Ajouter un connecteur personnalisé**, colle l'URL du connecteur, laisse vides l'ID client et le secret OAuth, puis choisis **Connecter**.
5. Le client ouvre la page de consentement. Elle montre qui demande, où ta réponse te renvoie, et l'interrupteur **Autoriser le lancement de sauvegardes**, désactivé au départ. Choisis **Autoriser** ou **Refuser**.

Chaque client connecté reçoit une tuile à côté des clés, avec son logo, son journal, **Révoquer** et **Autoriser le lancement de sauvegardes**, et les mêmes limites qu'une clé. La révocation agit immédiatement. Quand le même client se reconnecte, sa nouvelle autorisation remplace l'ancienne, et une autorisation que personne n'a utilisée pendant 30 jours expire. Jusqu'à 10 clients peuvent être connectés en même temps, en plus des 10 clés.

La page de consentement n'accepte une demande que d'un client enregistré qui indique exactement l'une de ses adresses de retour enregistrées : https, ou une adresse de bouclage sur n'importe quel port pour un client sur ton propre ordinateur. Seul le flux par code d'autorisation avec PKCE (S256) est accepté, et ta réponse est liée à ta session, donc aucun autre site ne peut l'envoyer à ta place. Les jetons d'accès valent une heure. Un jeton d'actualisation est remplacé à chaque utilisation, et s'il réapparaît ensuite, BombVault révoque l'autorisation, car quelqu'un d'autre en détient une copie. Un client qui répète sa dernière actualisation dans les 30 secondes, parce que la réponse ne lui est jamais parvenue, reçoit à la place de nouveaux jetons. BombVault ne télécharge pas de métadonnées de client depuis Internet, les clients s'enregistrent donc par l'enregistrement dynamique de client.

### Autres clients {#other-clients}

Tout client qui parle Streamable HTTP convient :

- URL : l'adresse de l'interface web suivie de `/mcp`, par exemple `https://192.168.1.10:3443/mcp`.
- La clé dans `Authorization: Bearer <key>` ou dans `X-API-Key: <key>`. Si les deux sont envoyés, ils doivent porter la même clé.
- `POST` avec `Content-Type: application/json` et `Accept: application/json, text/event-stream`.
- Un seul message JSON-RPC par requête ; les lots (batches) sont refusés.
- Versions du protocole 2026-07-28, 2025-11-25, 2025-06-18 et 2025-03-26.

## TLS et certificats {#tls}

BombVault sert le HTTPS avec un certificat qu'il a émis lui-même, et au départ ce certificat ne nomme que `localhost`, `127.0.0.1` et `::1`. Claude Code et `mcp-remote` le refusent sur une adresse du réseau local. Les solutions, dans l'ordre qui convient à la plupart des installations Unraid :

1. **Ajouter l'adresse dans la carte MCP.** Si la carte est ouverte en HTTPS à une adresse que le certificat ne nomme pas, elle le dit et propose **Ajouter cette adresse au certificat**. BombVault émet alors de nouveau son certificat avec cette adresse (votre navigateur avertit une fois de plus, comme la première fois). Cliquez ensuite sur **Télécharger le certificat** ; les extraits règlent `NODE_EXTRA_CA_CERTS` sur le fichier téléchargé, si bien que le client fait confiance exactement à ce certificat. Cela veut aussi dire que tout client configuré avec un fichier téléchargé plus tôt cesse de se connecter dès que le certificat est émis de nouveau, sur cet ordinateur comme sur tous les autres, jusqu'à ce qu'il reçoive le nouveau fichier.
2. **Un reverse proxy avec un certificat reconnu** (Nginx Proxy Manager, SWAG, Caddy, Traefik). Le client voit alors le certificat du proxy et n'a besoin de rien d'autre, et la carte n'avertit pas au sujet de celui de BombVault.
3. **Tailscale.** `tailscale serve` devant le conteneur, ou l'intégration Tailscale d'Unraid, vous donne un nom `ts.net` avec un certificat reconnu.
4. **`HTTP_ONLY=true`**, uniquement derrière un proxy qui termine le TLS ou sur un réseau auquel vous faites entièrement confiance. Il fait passer toute l'interface web en HTTP simple, demande une modification des paramètres du conteneur et envoie la clé sans chiffrement.

Ne définissez jamais `NODE_TLS_REJECT_UNAUTHORIZED=0`. Cela désactive la vérification des certificats pour tout ce à quoi ce processus Node.js parle.

Un reverse proxy doit transmettre l'en-tête `Authorization` (ou `X-API-Key`), ce que font les proxys sauf consigne contraire, et ne doit ni mettre `/mcp` en tampon ni le réécrire. Un bloc location pour Nginx ou Nginx Proxy Manager qui vérifie aussi le certificat de BombVault :

```nginx
location /mcp {
    proxy_pass https://192.168.1.10:3443;
    proxy_ssl_verify on;
    proxy_ssl_trusted_certificate /data/bombvault-cert.pem;
    proxy_ssl_name localhost;
    proxy_http_version 1.1;
    proxy_buffering off;
    proxy_set_header Host $host;
}
```

Derrière un proxy, chaque requête porte l'adresse du proxy. Cinq mauvaises clés venant d'un seul client mal configuré bloquent alors pendant une minute tous les clients MCP derrière ce proxy. Indiquez le proxy dans `TRUSTED_PROXY` (voir [Configuration](configuration.md)) pour compter par client.

## Modèle de sécurité {#security}

- Sans clé active et avec la connexion par OAuth désactivée, `/mcp` répond `404`.
- La connexion par OAuth n'est proposée que tant qu'un mot de passe de connexion est défini. Les jetons, les codes et les secrets de client ne sont stockés que sous forme d'empreinte, et un jeton ne vaut que pour l'adresse pour laquelle il a été émis.
- Un client peut s'enregistrer au plus 10 fois par heure depuis une même adresse, et BombVault garde au plus 100 clients enregistrés avec lesquels personne ne s'est connecté, chacun pendant un jour. Les codes et jetons d'actualisation erronés comptent dans le même blocage que les clés erronées.
- Les autorisations se comportent comme les clés lors de la restauration d'une sauvegarde de configuration ou d'un changement de `APP_KEY` : après une restauration, chaque client doit se reconnecter.
- Aucune adresse n'est exemptée. Les requêtes venant de `localhost`, de l'hôte Unraid, d'un reverse proxy ou de `tailscale serve` ont besoin d'une clé comme toutes les autres, même quand l'interface web n'a pas de mot de passe de connexion.
- Les clés ne sont stockées que sous forme d'empreinte, affichées une seule fois, et peuvent être renommées, remplacées et révoquées. Jusqu'à 10 clés actives, chacune avec son propre interrupteur **Autoriser le lancement de sauvegardes**.
- Chaque création, remplacement, changement de droits et révocation envoie une notification par vos canaux de notification, avec l'adresse d'où elle vient, sauf si les notifications sont désactivées.
- 5 mauvaises clés par minute et par adresse, puis `429`. 120 requêtes par minute et 12 sauvegardes lancées par heure et par clé, plus le délai d'attente et la garde de rétention décrits plus haut.
- Les requêtes d'une page de navigateur d'une autre origine sont refusées.
- Tant qu'aucun mot de passe de connexion n'est défini, aucune clé ne peut être créée depuis un nom d'hôte qui a l'air public.
- Chaque sauvegarde lancée par un assistant, ainsi que les exécutions de prune et de copie hors site qui en découlent, est marquée « via MCP » avec le nom de la clé, dans le journal d'activité, dans le panneau d'erreur et dans la notification de sauvegarde.
- Chaque appel d'outil est écrit dans le journal du conteneur avec l'identifiant de la clé et ses quatre derniers caractères (jamais son nom) et compté dans `/metrics` (`bombvault_mcp_requests_total`, `bombvault_mcp_tool_calls_total`, `bombvault_mcp_active_keys`).
- Restaurer une sauvegarde de la configuration révoque toutes les clés, parce que la base restaurée peut contenir des clés que vous avez révoquées après son enregistrement. Créez-en de nouvelles ensuite.
- Une clé cesse de fonctionner quand `APP_KEY` change (une réinstallation, ou une restauration dans un autre conteneur). La carte le détecte et marque la clé, et **Remplacer la clé** lui redonne un secret valide.
- Traite une clé comme un mot de passe. Un client qui ne sait lire la clé ni dans une variable d'environnement, ni par une invite, ni dans un fichier de clé la garde en clair dans sa configuration ou ses réglages, et sa boîte de dialogue le dit. Préfère une clé en lecture seule sur un ordinateur auquel tu fais moins confiance.

## Ce qui sort de la machine {#privacy}

Tout ce qu'un assistant lit part chez le fournisseur d'IA qui se trouve derrière lui : noms des éléments, planifications, historique des exécutions avec les messages d'erreur, identifiants et dates des points de restauration, noms des moteurs de base de données et tailles des dumps, activité en cours, chiffres de stockage, couverture et état. BombVault retire les chemins de l'hôte, les emplacements des dépôts, les noms d'hôte, les identifiants, les commandes de hook et les clés avant que quoi que ce soit ne sorte.

## Dépannage {#troubleshooting}

| Ce que vous voyez | Ce que cela signifie |
|---|---|
| `404` | Aucune clé active et la connexion par OAuth est désactivée, ou un mauvais chemin comme `/api/mcp`. Le point de terminaison est `/mcp`. |
| `401` | La clé manque, est mal saisie, révoquée ou remplacée. Un proxy supprime peut-être l'en-tête `Authorization` (essayez `X-API-Key`). Si la carte marque la clé comme n'étant plus valide, `APP_KEY` a changé : remplacez la clé. |
| `403` | La requête vient d'une page de navigateur d'une autre origine. Utilisez un client de bureau ou en ligne de commande. |
| `405` sur GET | Normal. Le point de connexion n'accepte que `POST`. |
| `400` "Accept must contain both 'application/json' and 'text/event-stream'" | Le client est trop ancien pour Streamable HTTP. Mettez-le à jour. |
| `400` "batch requests are not accepted" | Le client envoie des lots JSON-RPC. Envoyez un message par requête. |
| `429` | Trop de mauvaises clés depuis cette adresse, ou plus de 120 requêtes par minute avec une même clé. Attendez une minute et vérifiez que l'assistant ne tourne pas en boucle. |
| Erreurs contenant "certificate", "self-signed" ou "unable to verify" | Le client ne fait pas confiance au certificat de BombVault. Voir [TLS et certificats](#tls). |
| `busy` | Une autre sauvegarde ou une tâche de maintenance occupe ce domaine. Réessayez quand elle est terminée. |
| `cooldown` | Cet élément, ce domaine ou Backup Everything a été lancé par MCP il y a moins de 15 minutes. |
| `retention_guard` | Une sauvegarde MCP de plus ne laisserait que des points de restauration venant de MCP dans une fenêtre « garder les N derniers », ou l'élément a déjà reçu 4 sauvegardes par MCP au cours des dernières 24 heures, échecs et annulations compris. Dans le premier cas, la prochaine sauvegarde planifiée refait de la place ; dans le second, l'élément redevient disponible 24 heures après la plus ancienne de ces sauvegardes. Dans les deux cas, vous pouvez la lancer depuis l'interface web. |
| `rate_limited` | La clé a épuisé ses 12 lancements pour cette heure. |
| `not_permitted` sur un lancement | La clé est en lecture seule. Activez **Autoriser le lancement de sauvegardes** dans la carte ; aucune reconnexion n'est nécessaire. Sur une annulation, cela signifie que l'exécution n'a pas été lancée par cette clé. |
| `domain_off` | Ce type de sauvegarde est désactivé dans les paramètres. |
| `not_found` | BombVault ne protège pas cet élément. Ajoutez-le d'abord dans l'interface web ; MCP ne crée jamais de configuration. |
| Le client ne trouve pas le serveur d'autorisation | La connexion par OAuth est désactivée, aucun mot de passe de connexion n'est défini, ou le proxy ne laisse pas passer `/.well-known/` jusqu'à BombVault. |
| La page de consentement signale une adresse de retour non enregistrée | Le client a envoyé une adresse de retour qu'il n'a pas enregistrée. Supprime le connecteur dans le client et ajoute-le à nouveau. |
| Un client connecté reçoit `401` | Son autorisation a été révoquée, a expiré après 30 jours sans utilisation, ou l'adresse publique a changé. Le client se reconnecte. |

Ne définissez pas la variable d'environnement `MCPGODEBUG` sur le conteneur. Elle modifie le comportement de la bibliothèque MCP, et une valeur mal formée arrête BombVault au démarrage avant même qu'il n'écrive une seule ligne de journal.
