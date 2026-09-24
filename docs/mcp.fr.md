# Serveur MCP

BombVault intègre un serveur pour le Model Context Protocol (MCP), le protocole par lequel des assistants IA comme Claude Code et Claude Desktop accèdent à des outils externes. Un assistant peut ainsi lire l'état de vos sauvegardes et, si vous l'autorisez, lancer une sauvegarde ou annuler une sauvegarde qu'il a lancée lui-même. Le serveur reste éteint tant que vous ne créez pas de clé : sans clé active, le point de connexion `/mcp` répond `404` à tout.

## Ce qu'un assistant peut faire et ne peut pas faire {#tools}

| Outil | Ce qu'il fait | Type |
|---|---|---|
| `get_health` | Version, nom de l'instance, si une sauvegarde est en cours et ce que cette clé a le droit de faire | lecture |
| `get_status` | État de protection par domaine : dernière sauvegarde réussie, intervalle attendu, vérifications et contrôles hors site, prochaines exécutions planifiées | lecture |
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

Si le dépôt principal d'un élément est distant (S3, REST, SFTP, rclone), `list_restore_points` le contacte, et l'appel peut prendre un moment. Les copies hors site ne peuvent pas être listées par MCP.

## Ce que fait une sauvegarde lancée {#starting-backups}

La sauvegarde d'un assistant est la même que celle que lance l'interface web. Un conteneur en marche est arrêté jusqu'à la fin de sa sauvegarde, avec les conteneurs configurés pour s'arrêter en même temps. Une VM avec la méthode « graceful » est éteinte puis redémarrée. Un dataset ZFS arrête les conteneurs configurés pour lui pendant la prise de son snapshot. Les ensembles de dossiers, la clé flash et la configuration continuent de tourner. Ensuite, BombVault applique la politique de rétention et peut copier vers le dépôt hors site. `list_items` indique à l'assistant ce qu'un élément arrête et combien de temps a duré sa dernière sauvegarde, et les descriptions des outils lui demandent de vous le dire avant de lancer quoi que ce soit.

Comme une sauvegarde arrête des services et fait sortir d'anciens points de restauration, les lancements par MCP sont limités :

- 12 sauvegardes lancées par heure et par clé.
- 15 minutes entre deux lancements MCP du même élément, du même domaine ou de Backup Everything.
- Au plus 4 lancements MCP du même élément sur 24 heures.
- **Garde de rétention.** Quand un domaine conserve un nombre fixe de points de restauration (uniquement « garder les N derniers », sans règle quotidienne, hebdomadaire ou mensuelle, en local ou sur une destination hors site), chaque nouvelle sauvegarde fait sortir la plus ancienne. BombVault refuse alors un lancement MCP d'un élément dont les N-1 dernières sauvegardes réussies ont toutes été lancées par MCP. Au moins un point de restauration créé par la planification ou par vous reste donc toujours dans l'ensemble conservé. Avec « garder le dernier » (N = 1), un assistant ne peut pas du tout sauvegarder cet élément. La prochaine sauvegarde planifiée refait de la place.

Un lancement de domaine ou de Backup Everything laisse de côté les éléments retenus par une limite et les nomme dans sa réponse. L'interface web et la planification ne sont concernées par aucune de ces limites. Le quota horaire est gardé en mémoire, un redémarrage de BombVault le remet donc à zéro.

## Activer le serveur {#switch-on}

1. Ouvrez **Paramètres, Système, Serveur MCP** et cliquez sur **Nouvelle clé**.
2. Donnez à la clé un nom qui dit où elle sert, par exemple « Claude Code sur le portable ». Avec une clé par client, vous pouvez en révoquer une sans toucher aux autres.
3. Laissez **Autoriser le lancement de sauvegardes** activé, ou désactivez-le pour une clé qui doit seulement lire. Vous pouvez le changer plus tard sur la ligne de la clé, et le changement s'applique dès la requête suivante de l'assistant, sans reconnexion.
4. Cliquez sur **Créer la clé**. La clé s'affiche une seule fois. BombVault n'en garde qu'une empreinte et ne peut plus l'afficher, alors copiez-la tout de suite ou prenez l'un des extraits en dessous, qui contiennent alors la vraie clé.

Sans mot de passe de connexion, l'interface web elle-même est ouverte à tout votre réseau, et quiconque peut l'ouvrir peut aussi créer une clé. La carte le signale. Si vous ouvrez BombVault sous un nom qui a l'air public (par exemple `bombvault.example.com` derrière un reverse proxy) et qu'aucun mot de passe de connexion n'est défini, aucune clé ne peut être créée ni remplacée depuis cette adresse, pour qu'aucune page web sur Internet ne puisse amener votre navigateur à en créer une. Définissez un mot de passe de connexion, ou ouvrez BombVault par son adresse IP ou par un nom local comme `tower` ou `tower.local`.

## Connecter un client {#clients}

La carte affiche des extraits prêts à l'emploi pour l'adresse à laquelle vous l'avez ouverte : choisissez votre client et copiez l'extrait. La suite explique ce que font les extraits et donne les formes que la carte ne montre pas.

### Claude Code {#claude-code}

Exécutez une fois dans un terminal la commande de la carte. Avec un certificat auquel votre ordinateur fait confiance, elle ressemble à ceci :

```bash
claude mcp add --transport http bombvault --scope user https://bombvault.example.com/mcp --header "Authorization: Bearer <your key>"
```

Vérifiez la connexion avec `/mcp` dans Claude Code. `--scope user` range la clé dans votre configuration utilisateur plutôt que dans un fichier de projet.

La commande contient la clé, et votre shell la garde peut-être dans son historique. Pour l'éviter, placez un fichier `.mcp.json` dans le dossier du projet et gardez la clé dans une variable d'environnement. Claude Code remplace `${BOMBVAULT_MCP_KEY}` en lisant le fichier :

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

Définissez `BOMBVAULT_MCP_KEY` là où Claude Code démarre, par exemple dans le profil de votre shell, modifié dans un éditeur de texte plutôt que tapé à l'invite. Ne committez jamais un `.mcp.json` dans lequel la clé est écrite en clair.

Avec le certificat propre à BombVault (voir [TLS et certificats](#tls)), la commande de la carte lance `mcp-remote` à la place et indique à Node.js le certificat téléchargé :

```bash
claude mcp add bombvault --scope user -e "NODE_EXTRA_CA_CERTS=<path of the downloaded bombvault-cert.pem>" -e "BOMBVAULT_MCP_KEY=<your key>" -- npx -y mcp-remote https://192.168.1.10:3443/mcp --header 'X-API-Key:${BOMBVAULT_MCP_KEY}'
```

Les guillemets simples empêchent votre shell de développer la variable ; `mcp-remote` s'en charge lui-même. La même forme marche dans `.mcp.json` : reprenez l'entrée de Claude Desktop ci-dessous et retirez `BOMBVAULT_MCP_KEY` de son `env`, la clé vient alors de votre environnement.

### Claude Desktop {#claude-desktop}

Claude Desktop atteint BombVault par `mcp-remote`, qui a besoin de Node.js sur l'ordinateur. Ouvrez le fichier de configuration dans Claude Desktop par **Settings, Developer, Edit Config**. Il se trouve dans `%APPDATA%\Claude\claude_desktop_config.json` sous Windows et dans `~/Library/Application Support/Claude/claude_desktop_config.json` sous macOS. Ajoutez l'entrée de la carte dans `"mcpServers"`, à côté des serveurs déjà présents, puis redémarrez Claude Desktop :

```json
{
  "mcpServers": {
    "bombvault": {
      "command": "npx",
      "args": ["-y", "mcp-remote", "https://192.168.1.10:3443/mcp", "--header", "X-API-Key:${BOMBVAULT_MCP_KEY}"],
      "env": {
        "BOMBVAULT_MCP_KEY": "<your key>",
        "NODE_EXTRA_CA_CERTS": "<path of the downloaded bombvault-cert.pem>"
      }
    }
  }
}
```

- `NODE_EXTRA_CA_CERTS` n'est là que pour le certificat propre à BombVault. Derrière un certificat auquel votre ordinateur fait déjà confiance, retirez-le.
- `--allow-http` n'est ajouté que pour une adresse `http://` simple.
- L'en-tête s'écrit `X-API-Key:${BOMBVAULT_MCP_KEY}`, sans espace après les deux-points et avec la clé dans `env`. Sur certains systèmes, `mcp-remote` coupe une valeur `--header` au premier espace, et une clé écrite après un espace serait perdue.

### Connecteurs personnalisés dans les réglages de Claude {#custom-connectors}

Les connecteurs ajoutés dans les réglages de Claude lui-même (sur claude.ai et dans la liste des connecteurs de Claude Desktop) ne sont pas encore pris en charge. Ces connecteurs sont appelés depuis le cloud d'Anthropic : il leur faut donc une adresse HTTPS publique, et ils se connectent par OAuth. Ils ne peuvent pas envoyer une clé fixe, et BombVault ne propose que des clés fixes, sans connexion OAuth. Mettre BombVault sur Internet pour eux n'y changerait rien. Utilisez Claude Code, ou Claude Desktop par `mcp-remote` comme ci-dessus.

### Autres clients {#other-clients}

Tout client qui parle Streamable HTTP convient :

- URL : l'adresse de l'interface web suivie de `/mcp`, par exemple `https://192.168.1.10:3443/mcp`.
- La clé dans `Authorization: Bearer <key>` ou dans `X-API-Key: <key>`. Si les deux sont envoyés, ils doivent porter la même clé.
- `POST` avec `Content-Type: application/json` et `Accept: application/json, text/event-stream`.
- Un seul message JSON-RPC par requête ; les lots (batches) sont refusés.
- Versions du protocole 2026-07-28, 2025-11-25, 2025-06-18 et 2025-03-26.

## TLS et certificats {#tls}

BombVault sert le HTTPS avec un certificat qu'il a émis lui-même, et au départ ce certificat ne nomme que `localhost`, `127.0.0.1` et `::1`. Claude Code et `mcp-remote` le refusent sur une adresse du réseau local. Les solutions, dans l'ordre qui convient à la plupart des installations Unraid :

1. **Ajouter l'adresse dans la carte MCP.** Si la carte est ouverte en HTTPS à une adresse que le certificat ne nomme pas, elle le dit et propose **Ajouter cette adresse au certificat**. BombVault émet alors de nouveau son certificat avec cette adresse (votre navigateur avertit une fois de plus, comme la première fois). Cliquez ensuite sur **Télécharger le certificat** ; les extraits règlent `NODE_EXTRA_CA_CERTS` sur le fichier téléchargé, si bien que le client fait confiance exactement à ce certificat.
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

- Sans clé active, `/mcp` répond `404`.
- Aucune adresse n'est exemptée. Les requêtes venant de `localhost`, de l'hôte Unraid, d'un reverse proxy ou de `tailscale serve` ont besoin d'une clé comme toutes les autres, même quand l'interface web n'a pas de mot de passe de connexion.
- Les clés ne sont stockées que sous forme d'empreinte, affichées une seule fois, et peuvent être renommées, remplacées et révoquées. Jusqu'à 10 clés actives, chacune avec son propre interrupteur **Peut lancer des sauvegardes**.
- Chaque création, remplacement, changement de droits et révocation envoie une notification par vos canaux de notification, avec l'adresse d'où elle vient, sauf si les notifications sont désactivées.
- 5 mauvaises clés par minute et par adresse, puis `429`. 120 requêtes par minute et 12 sauvegardes lancées par heure et par clé, plus le délai d'attente et la garde de rétention décrits plus haut.
- Les requêtes d'une page de navigateur d'une autre origine sont refusées.
- Tant qu'aucun mot de passe de connexion n'est défini, aucune clé ne peut être créée depuis un nom d'hôte qui a l'air public.
- Chaque sauvegarde lancée par un assistant, ainsi que les exécutions de prune et de copie hors site qui en découlent, est marquée « via MCP » avec le nom de la clé, dans le journal d'activité, dans le panneau d'erreur et dans la notification de sauvegarde.
- Chaque appel d'outil est écrit dans le journal du conteneur avec l'identifiant de la clé et ses quatre derniers caractères (jamais son nom) et compté dans `/metrics` (`bombvault_mcp_requests_total`, `bombvault_mcp_tool_calls_total`, `bombvault_mcp_active_keys`).
- Restaurer une sauvegarde de la configuration révoque toutes les clés, parce que la base restaurée peut contenir des clés que vous avez révoquées après son enregistrement. Créez-en de nouvelles ensuite.
- Une clé cesse de fonctionner quand `APP_KEY` change (une réinstallation, ou une restauration dans un autre conteneur). La carte le détecte et marque la clé, et **Remplacer la clé** lui redonne un secret valide.
- Traitez une clé comme un mot de passe. Claude Code et Claude Desktop la conservent en clair dans leur configuration. Sur un ordinateur auquel vous faites moins confiance, préférez une clé en lecture seule.

## Ce qui sort de la machine {#privacy}

Tout ce qu'un assistant lit part chez le fournisseur d'IA qui se trouve derrière lui : noms des éléments, planifications, historique des exécutions avec les messages d'erreur, identifiants et dates des points de restauration, noms des moteurs de base de données et tailles des dumps, activité en cours, chiffres de stockage, couverture et état. BombVault retire les chemins de l'hôte, les emplacements des dépôts, les noms d'hôte, les identifiants, les commandes de hook et les clés avant que quoi que ce soit ne sorte.

## Dépannage {#troubleshooting}

| Ce que vous voyez | Ce que cela signifie |
|---|---|
| `404` | Pas de clé active, ou un mauvais chemin comme `/api/mcp`. Le point de connexion est `/mcp`. |
| `401` | La clé manque, est mal saisie, révoquée ou remplacée. Un proxy supprime peut-être l'en-tête `Authorization` (essayez `X-API-Key`). Si la carte marque la clé comme n'étant plus valide, `APP_KEY` a changé : remplacez la clé. |
| `403` | La requête vient d'une page de navigateur d'une autre origine. Utilisez un client de bureau ou en ligne de commande. |
| `405` sur GET | Normal. Le point de connexion n'accepte que `POST`. |
| `400` "Accept must contain both 'application/json' and 'text/event-stream'" | Le client est trop ancien pour Streamable HTTP. Mettez-le à jour. |
| `400` "batch requests are not accepted" | Le client envoie des lots JSON-RPC. Envoyez un message par requête. |
| `429` | Trop de mauvaises clés depuis cette adresse, ou plus de 120 requêtes par minute avec une même clé. Attendez une minute et vérifiez que l'assistant ne tourne pas en boucle. |
| Erreurs contenant "certificate", "self-signed" ou "unable to verify" | Le client ne fait pas confiance au certificat de BombVault. Voir [TLS et certificats](#tls). |
| `busy` | Une autre sauvegarde ou une tâche de maintenance occupe ce domaine. Réessayez quand elle est terminée. |
| `cooldown` | Cet élément, ce domaine ou Backup Everything a été lancé par MCP il y a moins de 15 minutes. |
| `retention_guard` | Une sauvegarde MCP de plus ne laisserait que des points de restauration venant de MCP dans une fenêtre « garder les N derniers ». La prochaine sauvegarde planifiée refait de la place, ou lancez-la depuis l'interface web. |
| `rate_limited` | La clé a épuisé ses 12 lancements pour cette heure. |
| `not_permitted` sur un lancement | La clé est en lecture seule. Activez **Peut lancer des sauvegardes** dans la carte ; aucune reconnexion n'est nécessaire. Sur une annulation, cela signifie que l'exécution n'a pas été lancée par cette clé. |
| `domain_off` | Ce type de sauvegarde est désactivé dans les paramètres. |
| `not_found` | BombVault ne protège pas cet élément. Ajoutez-le d'abord dans l'interface web ; MCP ne crée jamais de configuration. |

Ne définissez pas la variable d'environnement `MCPGODEBUG` sur le conteneur. Elle modifie le comportement de la bibliothèque MCP, et une valeur mal formée arrête BombVault au démarrage avant même qu'il n'écrive une seule ligne de journal.
