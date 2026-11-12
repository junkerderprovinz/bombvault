# API és integrációk

A BombVault kis HTTP API-t kínál szkriptekhez, irányítópultokhoz és otthonautomatizáláshoz. Ugyanazt olvassa, amit az áttekintés mutat, és el tud indítani egy mentést. Minden más, például a visszaállítás, a mentések törlése és a beállítások, a webes felületen marad.

## Tokenek {#tokens}

Minden kéréshez API-token kell, akkor is, ha nincs belépési jelszó. A **Beállítások, Integrációk, API-tokenek** alatt hozhatsz létre egyet:

1. Adj meg egy nevet, amely elárulja, hol használod a tokent, például „Home Assistant” vagy „Uptime Kuma”.
2. Kapcsold be a **Biztonsági mentések indításának engedélyezése** kapcsolót, ha a tokennek mentést kell indítania. Nélküle csak olvasni tud.
3. Kattints a **Token létrehozása** gombra. A token egyszer jelenik meg. A BombVault csak ujjlenyomatot őriz belőle, ezért most másold ki.

A tokent fejlécben küldd, `Authorization: Bearer <token>` vagy `X-API-Key: <token>` formában. A token `bvapi_` előtaggal kezdődik. Csak az API-t nyitja meg: MCP-kulcs itt nem működik, és token sem működik az MCP-hez.

Minden tokennek van egy csempéje a nevével, azzal, hogy indíthat-e mentést, az utolsó négy karakterével, a legutóbbi használat idejével és helyével, valamint a mai hívásaival. A csempén átnevezheted, módosíthatod a jogait, lecserélheted vagy visszavonhatod. A **Napló** mutatja az általa indított mentéseket és a legutóbbi hívásait. Ha a BombVault beállításait mentésből állítod vissza, minden token visszavonódik, mert a mentés olyan tokeneket is tartalmazhat, amelyeket azóta visszavontál.

Belépési jelszó nélkül bárki, aki meg tudja nyitni a webes felületet, tokent is létrehozhat. Ha a BombVaultot nyilvánosnak tűnő néven nyitod meg, és nincs jelszó, arról a címről nem lehet tokent létrehozni, ugyanúgy, mint az [MCP-kulcsoknál](mcp.md#switch-on).

## Végpontok {#endpoints}

| Útvonal | Mit ad vissza vagy tesz | Token |
|---|---|---|
| `GET /api/v1/health` | Verzió, példánynév, fut-e mentés, és mit tehet ez a token | olvasás |
| `GET /api/v1/status` | Védettség szakaszonként: utolsó sikeres mentés, várt időköz, ellenőrzések, következő ütemezett futások | olvasás |
| `GET /api/v1/activity` | Mi fut éppen, fázissal és százalékkal | olvasás |
| `GET /api/v1/items` | Minden védett elem ütemezéssel, azzal, mit állít le egy mentés, és az utolsó mentéssel. `?domain=` egy szakaszhoz | olvasás |
| `GET /api/v1/runs` | Futási előzmények, a legújabb elöl. Szűrők: `limit`, `domain`, `item`, `status`, `kind`, `since` | olvasás |
| `GET /api/v1/anomalies` | Anomáliák a nyitottak összesítésével. Szűrők: `state`, `severity`, `domain`, `limit` | olvasás |
| `GET /api/v1/anomalies/{id}` | Egy anomália | olvasás |
| `GET /api/v1/storage/{domain}` | Méretelőzmények, heti növekedés és a szakasz minden tárolójának szabad helye | olvasás |
| `POST /api/v1/backups` | Ment egy elemet (`{"domain":"containers","item":"plex"}`) vagy egy egész szakaszt (`{"domain":"vms"}`) | indítás |
| `POST /api/v1/backups/everything` | Elindítja a Teljes mentés menetet | indítás |
| `POST /api/v1/runs/{id}/cancel` | Megszakít egy futó mentést, amelyet ez a token indított | indítás |

A szakaszok: `containers`, `vms`, `files`, `zfs`, `flash` és `config`. Az idők Unix-másodpercek. A válaszok ugyanazok, mint az azonos nevű [MCP-eszközöké](mcp.md#tools), így a kettő együtt marad.

Az itt indított mentés ugyanaz, mint amit a webes felület indít: a futó konténer leáll, amíg a mentése be nem fejeződik. A kérés azonnal visszatér, a `/api/v1/activity` és a `/api/v1/runs` mutatja, hogyan halad.

## Példák {#examples}

```sh
# How are the backups doing?
curl -s -H "Authorization: Bearer $BOMBVAULT_TOKEN" https://tower:3443/api/v1/status

# Back up one container now.
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
| 403 | `not_permitted`, `forbidden_origin` | A token csak olvashat, vagy nem ő indította a futást, vagy a kérés más eredetű oldalról jött |
| 404 | `not_found` | Nincs ilyen elem, futás vagy anomália |
| 409 | `busy`, `domain_off`, `nothing_to_back_up`, `not_running` | Már fut valami, a szakasz ki van kapcsolva, vagy nincs teendő |
| 429 | `throttled`, `rate_limited`, `cooldown`, `retention_guard` | Egy korlát visszatartja a kérést. A `Retry-After` megmondja, mikor próbálkozhatsz újra |

Az indításokra ugyanazok a korlátok vonatkoznak, mint az [MCP-n keresztüliekre](mcp.md#starting-backups): tokenenként óránként 12, ugyanannak az elemnek két indítása között 15 perc, egy elemnek 24 óra alatt legfeljebb 4 indítás, valamint a megőrzési védelem. Az utolsó három együtt számolja az MCP-n és az API-n keresztüli, valamint a Home Assistantből érkező indításokat. Egy token percenként 120 kérést küldhet. Egy címről öt sikertelen próbálkozás után a cím egy percre letiltódik.

## OpenAPI {#openapi}

A BombVault ezeknek az útvonalaknak a leírását a `/api/v1/openapi.json` címen szolgálja ki (OpenAPI 3.1). Ehhez nem kell token. Töltsd be a Swagger UI-ba, a Postmanbe vagy egy kódgenerátorba.

## Home Assistant {#home-assistant}

A BombVault eszközként jelenhet meg a Home Assistantben, MQTT-felderítéssel. Ehhez a Home Assistantnek kell a saját MQTT-integrációja és egy bróker, például a Mosquitto bővítmény. Saját komponensre nincs szükség.

1. A BombVaultban nyisd meg a **Beállítások, Integrációk** részt, és kapcsold be a **Home Assistant** elemet a **Szolgáltatások** kártyán. A bróker mezői alatta jelennek meg.
2. Add meg a bróker címét és portját, és ha kéri, a felhasználónevet és a jelszót. Kapcsold be a **TLS használata** kapcsolót, ha a bróker TLS-t használ, általában a 8883-as porton. A tanúsítványának érvényesnek kell lennie a megadott címre. Ha módosítod a címet, a portot vagy a felhasználónevet, add meg újra a jelszót: a BombVault a tároltat nem adja tovább másik brókernek vagy felhasználónak.
3. Kattints a **Mentés** gombra. A sor mutatja, amikor a kapcsolat felépült.

Az eszköz neve BombVault, vagy BombVault a példány nevével zárójelben, és ezek az entitásai vannak:

| Entitás | Mit mutat |
|---|---|
| Status | `ok`, `warning`, `overdue`, `failed` vagy `off`, a bekapcsolt szakaszok közül a legrosszabb. Az első mentésére váró szakasz csak akkor számít, ha az a mentés már esedékes volt, és a státuszt csak egy friss mentésű szakasz teszi `ok` értékűvé. Az `overdue` az áttekintéshez hasonlóan azt jelenti, hogy egy szakasz utolsó mentése két időköznél régebbi |
| Running job | Mi fut éppen, vagy `idle` |
| Open anomalies | Hány anomália nyitott |
| Next scheduled backup | Mikor indul a következő ütemezett mentés |
| *Szakasz* last backup | Mikor futott a szakasz utolsó sikeres mentése |
| *Szakasz* last result | Hogyan végződött az utolsó mentése |
| *Szakasz* repository free space | A szabad hely ott, ahol a fő tárolója van, ha a BombVault ki tudja olvasni |
| Back up *szakasz* | Egy gomb, amely az egész szakaszt menti |

Minden bekapcsolt szakasz saját entitásokat kap, a kikapcsolt elveszíti őket. A gombok akkor jelennek meg, ha bekapcsolod az **A gombok biztonsági mentést indítanak** kapcsolót. Új telepítésnél ez ki van kapcsolva. A gombokra ugyanazok a korlátok vonatkoznak, mint az [API-n keresztüli indításokra](#errors). Ezen felül a BombVault szakaszonként egyszerre egy nyomást fogad, percenként legfeljebb hatot, és figyelmen kívül hagyja azt a nyomást, amelyet a bróker retained üzenetként megőrzött. Bárki megnyomhatja őket, aki közzétehet a brókeren, ezért adj jelszót a brókernek.

A BombVault 15 másodpercenként beolvassa az állapotát, és közzéteszi, ha valami változott, JSON-ként a `<előtag>/<csomópont>/state` témában. Az előtag `bombvault`, amíg meg nem változtatod, a csomópont pedig egy rövid azonosító, amelyet a BombVault egyszer választ. A felderítési üzenetek a Home Assistant alapértelmezett `homeassistant` előtagjára mennek. Mindkettő megőrzött (retained). Egy utolsó üzenet (last will) elérhetetlennek jelöli az eszközt, ha a BombVault szó nélkül leáll. Ha kikapcsolod a kapcsolatot, a BombVault eltávolítja az eszközt és az entitásait a Home Assistantből.

### Példa: egy kártya és egy riasztás {#home-assistant-example}

A Home Assistant a nevekből képezi az entitásazonosítókat, így a BombVault eszköz azonosítói például `sensor.bombvault_status` alakúak. Ha a példány neve zárójelben szerepel, az azonosítók azt is tartalmazzák, ezért előbb nézd meg őket a **Developer Tools, States** alatt. Egy kártya az irányítópultra:

```yaml
type: entities
title: Backups
entities:
  - sensor.bombvault_status
  - sensor.bombvault_running_job
  - sensor.bombvault_open_anomalies
  - sensor.bombvault_next_scheduled_backup
```

Egy automatizálás, amely szól a telefonodnak, ha az állapot `failed` vagy `overdue` lesz:

```yaml
alias: BombVault needs you
trigger:
  - platform: state
    entity_id: sensor.bombvault_status
    to:
      - failed
      - overdue
action:
  - service: notify.mobile_app_my_phone
    data:
      message: "BombVault status: {{ trigger.to_state.state }}. Open the overview."
```

## Prometheus {#metrics}

A **Beállítások, Integrációk** alatti **Szolgáltatások** kártyán a **/metrics közzététele** a mentések állapotát, méreteit és időbélyegeit szolgálja ki a Prometheusnak, a Grafanának vagy az Uptime Kumának, a webes felület portjának `/metrics` címén. Alapból ki van kapcsolva. A **Lekérdezési token (opcionális)** azt éri el, hogy a lekérdező bearer tokent küldjön. A címkék nem tartalmaznak titkokat és útvonalakat.

## Irányítópult-widget {#dashboard-widget}

A **Beállítások, Integrációk** alatti **Irányítópult-widget** kártya csak olvasható tokent hoz létre, és ad egy címet minden olyan irányítópulthoz, amely iframe-et jelenít meg, például a Homepage, az Organizr vagy a Heimdall. A cím egy kis oldalt nyit meg a `/widget` útvonalon, csak az élő tevékenységnaplóval. A token ezt a naplót engedi, mást nem, a **Letiltás** pedig azonnal visszavonja. A beágyazott oldal csak angolul érhető el.

## A BombVault megtalálása a hálózaton {#mdns}

A BombVault mDNS-sel bejelenti a webes felületét a helyi hálózaton, azzal a protokollal, amelyre a Bonjour és az Avahi épül. Egy böngésző így `https://bombvault.local:3443` címen éri el, `HTTP_ONLY` esetén `http://bombvault.local:3000` címen, a szolgáltatásböngészők pedig webszolgáltatásként listázzák `_bombvault` altípussal. A TXT-rekordjai a verziót és az útvonalat tartalmazzák. A kapcsoló a **Megtalálás a hálózaton** a **Beállítások, Integrációk** alatti **Szolgáltatások** kártyán, és alapból be van kapcsolva. Ha egy másik eszköz már használja a nevet, a BombVault a `bombvault-2.local` nevet veszi fel és így tovább, a kártya pedig mutatja a kapott címet. Amikor a BombVault leáll, vagy kikapcsolod a bejelentést, szól a hálózatnak, és a böngészők azonnal eltávolítják a bejegyzést.

Hogy a bejelentés eléri-e a hálózatodat, attól függ, hogyan csatlakozik a konténer:

- **bridge**, az Unraid-sablon alapértéke: a bejelentés a Docker hálózatán belül marad, a helyi hálózaton senki sem látja. Nyisd meg a BombVaultot a gazdagép címén, mint eddig.
- **br0** vagy más macvlan- vagy ipvlan-hálózat: a konténernek saját címe van a helyi hálózaton, és a bejelentés eléri azt.
- **host**: a bejelentés a gazdagép csatolóin megy ki, az Unraid saját bejelentése mellett. A Docker és a libvirt hídjai kimaradnak.

Csak IPv4-címek kerülnek bejelentésre.
