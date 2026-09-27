# API et intégrations

BombVault propose une petite API HTTP pour les scripts, les tableaux de bord et la domotique. Elle lit ce que montre le tableau de bord et peut lancer une sauvegarde. Tout le reste, comme les restaurations, la suppression de sauvegardes et les réglages, reste dans l'interface web.

## Jetons {#tokens}

Chaque requête demande un jeton d'API, même sans mot de passe de connexion. Crée-le sous **Paramètres, Système, Jetons d'API** :

1. Saisis un nom qui dit où le jeton sert, par exemple « Home Assistant » ou « Uptime Kuma ».
2. Active **Autoriser le lancement de sauvegardes** si le jeton doit pouvoir lancer des sauvegardes. Sans cela, il peut seulement lire.
3. Clique sur **Créer le jeton**. Le jeton s'affiche une seule fois. BombVault n'en garde qu'une empreinte, alors copie-le maintenant.

Envoie le jeton dans un en-tête, soit `Authorization: Bearer <token>`, soit `X-API-Key: <token>`. Un jeton commence par `bvapi_`. Il n'ouvre que l'API : une clé MCP ne fonctionne pas ici, et un jeton ne fonctionne pas pour MCP.

Chaque jeton a une tuile avec son nom, s'il peut lancer des sauvegardes, ses quatre derniers caractères, quand et d'où il a servi la dernière fois, et ses appels du jour. Sur la tuile, tu peux le renommer, changer ce qu'il peut faire, le remplacer ou le révoquer. **Journal** montre les sauvegardes qu'il a lancées et ses derniers appels. Restaurer la configuration de BombVault depuis une sauvegarde révoque tous les jetons, car la sauvegarde peut contenir des jetons révoqués depuis.

Sans mot de passe de connexion, toute personne qui peut ouvrir l'interface web peut aussi créer un jeton. Si tu ouvres BombVault sous un nom d'apparence publique sans mot de passe, aucun jeton ne peut être créé depuis cette adresse, comme pour les [clés MCP](mcp.md#switch-on).

## Points d'accès {#endpoints}

| Route | Ce qu'elle renvoie ou fait | Jeton |
|---|---|---|
| `GET /api/v1/health` | Version, nom de l'instance, si une sauvegarde tourne et ce que ce jeton peut faire | lecture |
| `GET /api/v1/status` | État de protection par domaine : dernière sauvegarde réussie, intervalle attendu, vérifications, prochains lancements prévus | lecture |
| `GET /api/v1/activity` | Ce qui tourne en ce moment, avec phase et pourcentage | lecture |
| `GET /api/v1/items` | Chaque élément protégé avec sa planification, ce qu'une sauvegarde arrête et sa dernière sauvegarde ; `?domain=` pour un domaine | lecture |
| `GET /api/v1/runs` | Historique des exécutions, les plus récentes d'abord ; filtres `limit`, `domain`, `item`, `status`, `kind`, `since` | lecture |
| `GET /api/v1/anomalies` | Anomalies avec un résumé de ce qui est ouvert ; filtres `state`, `severity`, `domain`, `limit` | lecture |
| `GET /api/v1/anomalies/{id}` | Une anomalie | lecture |
| `GET /api/v1/storage/{domain}` | Historique de taille, croissance par semaine et espace libre de chaque dépôt d'un domaine | lecture |
| `POST /api/v1/backups` | Sauvegarde un élément (`{"domain":"containers","item":"plex"}`) ou un domaine entier (`{"domain":"vms"}`) | lancement |
| `POST /api/v1/backups/everything` | Lance Backup Everything | lancement |
| `POST /api/v1/runs/{id}/cancel` | Annule une sauvegarde en cours lancée par ce jeton | lancement |

Les domaines sont `containers`, `vms`, `files`, `zfs`, `flash` et `config`. Les heures sont des secondes Unix. Les réponses sont celles des [outils MCP](mcp.md#tools) du même nom, les deux restent donc alignés.

Une sauvegarde lancée ici est la même que celle de l'interface web : un conteneur en marche est arrêté jusqu'à la fin de sa sauvegarde. La requête revient tout de suite, et `/api/v1/activity` et `/api/v1/runs` montrent la suite.

## Exemples {#examples}

```sh
# Où en sont les sauvegardes ?
curl -s -H "Authorization: Bearer $BOMBVAULT_TOKEN" https://tower:3443/api/v1/status

# Sauvegarder un conteneur maintenant.
curl -s -X POST -H "Authorization: Bearer $BOMBVAULT_TOKEN" \
  -H "Content-Type: application/json" -d '{"domain":"containers","item":"plex"}' \
  https://tower:3443/api/v1/backups
```

Avec le certificat autosigné de BombVault, ajoute `--cacert bombvault-cert.pem` (le fichier que donne **Télécharger le certificat** sur la carte MCP) ou `-k` sur un réseau de confiance.

## Erreurs et limites {#errors}

Une erreur revient sous la forme `{"error": {"code": "...", "message": "..."}}` avec le statut correspondant :

| Statut | Codes | Sens |
|---|---|---|
| 400 | `invalid_argument`, `ambiguous` | Un argument manque ou est faux |
| 401 | `no_token`, `invalid_token` | Pas de jeton, ou pas un jeton actif |
| 403 | `not_permitted` | Le jeton peut seulement lire, ou il n'a pas lancé cette exécution |
| 404 | `not_found` | Élément, exécution ou anomalie introuvable |
| 409 | `busy`, `domain_off`, `nothing_to_back_up`, `not_running` | Autre chose tourne, le domaine est désactivé, ou il n'y a rien à faire |
| 429 | `throttled`, `rate_limited`, `cooldown`, `retention_guard` | Une limite retient la requête ; `Retry-After` dit quand réessayer |

Les lancements suivent les mêmes limites que [ceux via MCP](mcp.md#starting-backups) : 12 par heure et par jeton, 15 minutes entre deux lancements du même élément, au plus 4 lancements d'un élément en 24 heures, et la protection de rétention. Les trois dernières comptent ensemble les lancements via MCP, via l'API et depuis Home Assistant. Un jeton peut faire 120 requêtes par minute. Après cinq échecs depuis une adresse, elle est bloquée une minute.

## OpenAPI {#openapi}

BombVault sert une description de ces routes sous `/api/v1/openapi.json` (OpenAPI 3.1). Aucun jeton n'est nécessaire. Charge-la dans Swagger UI, Postman ou un générateur de code.

## Home Assistant {#home-assistant}

BombVault peut apparaître dans Home Assistant comme un appareil, grâce à la découverte MQTT. Home Assistant a besoin de son intégration MQTT et d'un broker, par exemple le module Mosquitto. Aucun composant personnalisé n'est nécessaire.

1. Dans BombVault, ouvre **Paramètres, Système, Home Assistant**.
2. Saisis l'adresse et le port du broker, ainsi que le nom d'utilisateur et le mot de passe s'il en demande. Active **Utiliser TLS** si le broker parle TLS, en général sur le port 8883 ; son certificat doit être valide pour l'adresse saisie.
3. Active **Se connecter à Home Assistant** et clique sur **Enregistrer**. La carte indique quand la connexion est établie.

L'appareil s'appelle BombVault, ou BombVault avec le nom de l'instance entre parenthèses, et possède ces entités :

| Entité | Ce qu'elle affiche |
|---|---|
| Status | `ok`, `warning`, `failed` ou `off`, le pire des domaines activés |
| Running job | Ce qui tourne maintenant, ou `idle` |
| Open anomalies | Le nombre d'anomalies ouvertes |
| Next scheduled backup | Quand démarre la prochaine sauvegarde planifiée |
| *Domaine* last backup | Quand a eu lieu la dernière sauvegarde réussie du domaine |
| *Domaine* last result | Comment s'est terminée sa dernière sauvegarde |
| *Domaine* repository free space | L'espace libre là où se trouve son dépôt principal, si BombVault peut le lire |
| Back up *domaine* | Un bouton qui sauvegarde tout le domaine |

Les noms des entités sont en anglais, car Home Assistant les reprend tels que BombVault les envoie. Chaque domaine activé a ses propres entités, et un domaine désactivé les perd. Les boutons suivent les mêmes limites que les [lancements via l'API](#errors). Toute personne qui peut publier sur le broker peut appuyer dessus : protège donc le broker par un mot de passe, ou désactive **Les boutons lancent des sauvegardes**.

BombVault lit son état toutes les 15 secondes et le publie quand quelque chose a changé, en JSON sous `<préfixe>/<nœud>/state`. Le préfixe est `bombvault` tant que tu ne le changes pas, et le nœud est un court identifiant que BombVault choisit une fois. Les messages de découverte vont au préfixe par défaut de Home Assistant, `homeassistant`. Les deux sont conservés (retained). Un dernier message (last will) marque l'appareil comme indisponible si BombVault s'arrête sans prévenir. Désactiver la liaison retire l'appareil et ses entités de Home Assistant.
