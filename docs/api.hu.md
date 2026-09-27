# API és integrációk

A BombVault kis HTTP API-t kínál szkriptekhez, irányítópultokhoz és otthonautomatizáláshoz. Ugyanazt olvassa, amit az irányítópult mutat, és el tud indítani egy mentést. Minden más, például a visszaállítás, a mentések törlése és a beállítások, a webes felületen marad.

## Tokenek {#tokens}

Minden kéréshez API-token kell, akkor is, ha nincs belépési jelszó. A **Beállítások, Rendszer, API-tokenek** alatt hozhatsz létre egyet:

1. Adj meg egy nevet, amely elárulja, hol használod a tokent, például „Home Assistant” vagy „Uptime Kuma”.
2. Kapcsold be a **Mentések indításának engedélyezése** kapcsolót, ha a tokennek mentést kell indítania. Nélküle csak olvasni tud.
3. Kattints a **Token létrehozása** gombra. A token egyszer jelenik meg. A BombVault csak ujjlenyomatot őriz belőle, ezért most másold ki.

A tokent fejlécben küldd, `Authorization: Bearer <token>` vagy `X-API-Key: <token>` formában. A token `bvapi_` előtaggal kezdődik. Csak az API-t nyitja meg: MCP-kulcs itt nem működik, és token sem működik az MCP-hez.

Minden tokennek van egy csempéje a nevével, azzal, hogy indíthat-e mentést, az utolsó négy karakterével, a legutóbbi használat idejével és helyével, valamint a mai hívásaival. A csempén átnevezheted, módosíthatod a jogait, lecserélheted vagy visszavonhatod. A **Napló** mutatja az általa indított mentéseket és a legutóbbi hívásait. Ha a BombVault beállításait mentésből állítod vissza, minden token visszavonódik, mert a mentés olyan tokeneket is tartalmazhat, amelyeket azóta visszavontál.

Belépési jelszó nélkül bárki, aki meg tudja nyitni a webes felületet, tokent is létrehozhat. Ha a BombVaultot nyilvánosnak tűnő néven nyitod meg, és nincs jelszó, arról a címről nem lehet tokent létrehozni, ugyanúgy, mint az [MCP-kulcsoknál](mcp.md#switch-on).

## Végpontok {#endpoints}

| Útvonal | Mit ad vissza vagy tesz | Token |
|---|---|---|
| `GET /api/v1/health` | Verzió, példánynév, fut-e mentés, és mit tehet ez a token | olvasás |
| `GET /api/v1/status` | Védettség területenként: utolsó sikeres mentés, várt időköz, ellenőrzések, következő ütemezett futások | olvasás |
| `GET /api/v1/activity` | Mi fut éppen, fázissal és százalékkal | olvasás |
| `GET /api/v1/items` | Minden védett elem ütemezéssel, azzal, mit állít le egy mentés, és az utolsó mentéssel; `?domain=` egy területhez | olvasás |
| `GET /api/v1/runs` | Futási előzmények, a legújabb elöl; szűrők `limit`, `domain`, `item`, `status`, `kind`, `since` | olvasás |
| `GET /api/v1/anomalies` | Anomáliák a nyitottak összesítésével; szűrők `state`, `severity`, `domain`, `limit` | olvasás |
| `GET /api/v1/anomalies/{id}` | Egy anomália | olvasás |
| `GET /api/v1/storage/{domain}` | Méretelőzmények, heti növekedés és a terület minden tárolójának szabad helye | olvasás |
| `POST /api/v1/backups` | Ment egy elemet (`{"domain":"containers","item":"plex"}`) vagy egy egész területet (`{"domain":"vms"}`) | indítás |
| `POST /api/v1/backups/everything` | Elindítja a Backup Everything menetet | indítás |
| `POST /api/v1/runs/{id}/cancel` | Megszakít egy futó mentést, amelyet ez a token indított | indítás |

A területek: `containers`, `vms`, `files`, `zfs`, `flash` és `config`. Az idők Unix-másodpercek. A válaszok ugyanazok, mint az azonos nevű [MCP-eszközöké](mcp.md#tools), így a kettő együtt marad.

Az itt indított mentés ugyanaz, mint amit a webes felület indít: a futó konténer leáll, amíg a mentése be nem fejeződik. A kérés azonnal visszatér, a `/api/v1/activity` és a `/api/v1/runs` mutatja, hogyan halad.

## Példák {#examples}

```sh
# Hogy állnak a mentések?
curl -s -H "Authorization: Bearer $BOMBVAULT_TOKEN" https://tower:3443/api/v1/status

# Egy konténer mentése most.
curl -s -X POST -H "Authorization: Bearer $BOMBVAULT_TOKEN" \
  -H "Content-Type: application/json" -d '{"domain":"containers","item":"plex"}' \
  https://tower:3443/api/v1/backups
```

A BombVault saját, önaláírt tanúsítványával add hozzá a `--cacert bombvault-cert.pem` kapcsolót (a fájlt az MCP-kártya **Tanúsítvány letöltése** gombja adja), vagy megbízható hálózaton a `-k` kapcsolót.

## Hibák és korlátok {#errors}

A hiba `{"error": {"code": "...", "message": "..."}}` formában tér vissza, a megfelelő állapotkóddal:

| Állapot | Kódok | Jelentés |
|---|---|---|
| 400 | `invalid_argument`, `ambiguous` | Egy argumentum hiányzik vagy hibás |
| 401 | `no_token`, `invalid_token` | Nincs token, vagy nem aktív |
| 403 | `not_permitted` | A token csak olvashat, vagy nem ő indította a futást |
| 404 | `not_found` | Nincs ilyen elem, futás vagy anomália |
| 409 | `busy`, `domain_off`, `nothing_to_back_up`, `not_running` | Már fut valami, a terület ki van kapcsolva, vagy nincs teendő |
| 429 | `throttled`, `rate_limited`, `cooldown`, `retention_guard` | Egy korlát visszatartja a kérést; a `Retry-After` megmondja, mikor próbálkozhatsz újra |

Az indításokra ugyanazok a korlátok vonatkoznak, mint az [MCP-n keresztüliekre](mcp.md#starting-backups): tokenenként óránként 12, ugyanannak az elemnek két indítása között 15 perc, egy elemnek 24 óra alatt legfeljebb 4 indítás, valamint a megőrzési védelem. Az utolsó három együtt számolja az MCP-n és az API-n keresztüli indításokat. Egy token percenként 120 kérést küldhet. Egy címről öt sikertelen próbálkozás után a cím egy percre letiltódik.

## OpenAPI {#openapi}

A BombVault ezeknek az útvonalaknak a leírását a `/api/v1/openapi.json` címen szolgálja ki (OpenAPI 3.1). Ehhez nem kell token. Töltsd be a Swagger UI-ba, a Postmanbe vagy egy kódgenerátorba.
