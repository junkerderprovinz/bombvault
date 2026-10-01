# API a integrace

BombVault má malé HTTP API pro skripty, dashboardy a domácí automatizaci. Čte totéž, co ukazuje přehled, a umí spustit zálohu. Všechno ostatní, jako obnovy, mazání záloh a nastavení, zůstává ve webovém rozhraní.

## Tokeny {#tokens}

Každý požadavek potřebuje token API, i když není nastavené přihlašovací heslo. Vytvoříš ho v **Nastavení, Integrace, Tokeny API**:

1. Napiš název, který říká, kde se token používá, třeba „Home Assistant“ nebo „Uptime Kuma“.
2. Zapni **Povolit spouštění záloh**, pokud má token spouštět zálohy. Bez toho může jen číst.
3. Klikni na **Vytvořit token**. Token se ukáže jen jednou. BombVault si ponechá jen jeho otisk, takže ho zkopíruj hned.

Token posílej v hlavičce, buď `Authorization: Bearer <token>`, nebo `X-API-Key: <token>`. Token začíná na `bvapi_`. Otevírá jen API: klíč MCP tu nefunguje a token nefunguje pro MCP.

Každý token má dlaždici s názvem, informací, zda smí spouštět zálohy, posledními čtyřmi znaky, časem a místem posledního použití a dnešními voláními. Na dlaždici ho můžeš přejmenovat, změnit, co smí, nahradit ho nebo zneplatnit. **Protokol** ukáže zálohy, které spustil, a jeho poslední volání. Obnova konfigurace BombVault ze zálohy zneplatní všechny tokeny, protože záloha může obsahovat tokeny, které jsi později zneplatnil.

Bez přihlašovacího hesla může token vytvořit každý, kdo otevře webové rozhraní. Když BombVault otevřeš pod jménem, které vypadá veřejně, a heslo není nastavené, z této adresy tokeny vytvořit nejde, stejně jako [klíče MCP](mcp.md#switch-on).

## Koncové body {#endpoints}

| Cesta | Co vrací nebo dělá | Token |
|---|---|---|
| `GET /api/v1/health` | Verze, název instance, zda běží záloha a co tento token smí | čtení |
| `GET /api/v1/status` | Stav ochrany podle oblasti: poslední úspěšná záloha, očekávaný interval, kontroly, příští plánované běhy | čtení |
| `GET /api/v1/activity` | Co právě běží, s fází a procenty | čtení |
| `GET /api/v1/items` | Každá chráněná položka s plánem, tím, co záloha zastaví, a poslední zálohou; `?domain=` pro jednu oblast | čtení |
| `GET /api/v1/runs` | Historie běhů, nejnovější první; filtry `limit`, `domain`, `item`, `status`, `kind`, `since` | čtení |
| `GET /api/v1/anomalies` | Anomálie s přehledem otevřených; filtry `state`, `severity`, `domain`, `limit` | čtení |
| `GET /api/v1/anomalies/{id}` | Jedna anomálie | čtení |
| `GET /api/v1/storage/{domain}` | Vývoj velikosti, týdenní růst a volné místo každého repozitáře oblasti | čtení |
| `POST /api/v1/backups` | Zálohuje jednu položku (`{"domain":"containers","item":"plex"}`) nebo celou oblast (`{"domain":"vms"}`) | spouštění |
| `POST /api/v1/backups/everything` | Spustí Zálohu všeho | spouštění |
| `POST /api/v1/runs/{id}/cancel` | Zruší běžící zálohu, kterou spustil tento token | spouštění |

Oblasti jsou `containers`, `vms`, `files`, `zfs`, `flash` a `config`. Časy jsou v unixových sekundách. Odpovědi jsou stejné jako u [nástrojů MCP](mcp.md#tools) stejného jména, takže obojí zůstává v souladu.

Záloha spuštěná tady je stejná jako ta z webového rozhraní: běžící kontejner se zastaví, dokud jeho záloha neskončí. Požadavek se vrátí hned a `/api/v1/activity` a `/api/v1/runs` ukážou, jak to jde.

## Příklady {#examples}

```sh
# Jak jsou na tom zálohy?
curl -s -H "Authorization: Bearer $BOMBVAULT_TOKEN" https://tower:3443/api/v1/status

# Zálohovat jeden kontejner hned.
curl -s -X POST -H "Authorization: Bearer $BOMBVAULT_TOKEN" \
  -H "Content-Type: application/json" -d '{"domain":"containers","item":"plex"}' \
  https://tower:3443/api/v1/backups
```

S vlastním certifikátem BombVault podepsaným sám sebou přidej `--cacert bombvault-cert.pem` (soubor z tlačítka **Stáhnout certifikát** na kartě MCP), nebo v síti, které věříš, `-k`.

## Chyby a limity {#errors}

Chyba se vrátí jako `{"error": {"code": "...", "message": "..."}}` s odpovídajícím stavem:

| Stav | Kódy | Význam |
|---|---|---|
| 400 | `invalid_argument`, `ambiguous` | Argument chybí nebo je špatně |
| 401 | `no_token`, `invalid_token` | Žádný token, nebo ne aktivní |
| 403 | `not_permitted`, `forbidden_origin` | Token smí jen číst, nebo ten běh nespustil, nebo požadavek přišel ze stránky s jiným původem |
| 404 | `not_found` | Taková položka, běh nebo anomálie neexistuje |
| 409 | `busy`, `domain_off`, `nothing_to_back_up`, `not_running` | Už něco běží, oblast je vypnutá, nebo není co dělat |
| 429 | `throttled`, `rate_limited`, `cooldown`, `retention_guard` | Limit požadavek zdrží; `Retry-After` říká, kdy to zkusit znovu |

Spuštění mají stejné limity jako [spuštění přes MCP](mcp.md#starting-backups): 12 za hodinu na token, 15 minut mezi dvěma spuštěními téže položky, nejvýše 4 spuštění jedné položky za 24 hodin a ochrana uchovávání. Poslední tři počítají spuštění přes MCP, přes API a z Home Assistant dohromady. Token smí poslat 120 požadavků za minutu. Pět neúspěšných pokusů z jedné adresy ji na minutu zablokuje.

## OpenAPI {#openapi}

BombVault poskytuje popis těchto cest na `/api/v1/openapi.json` (OpenAPI 3.1). Nepotřebuje token. Načti ho do Swagger UI, Postmanu nebo generátoru kódu.

## Home Assistant {#home-assistant}

BombVault se může v Home Assistant objevit jako zařízení díky zjišťování MQTT. Home Assistant k tomu potřebuje svou integraci MQTT a broker, třeba doplněk Mosquitto. Žádná vlastní komponenta není potřeba.

1. V BombVault otevři **Nastavení, Integrace, Home Assistant**.
2. Zadej adresu a port brokeru, a pokud je vyžaduje, i uživatelské jméno a heslo. Zapni **Použít TLS**, pokud broker používá TLS, obvykle na portu 8883; jeho certifikát musí platit pro zadanou adresu. Když změníš adresu, port nebo uživatelské jméno, zadej heslo znovu: BombVault uložené heslo jinému brokeru ani uživateli nepředá.
3. Zapni **Připojit k Home Assistant** a klikni na **Uložit**. Karta ukáže, až spojení naběhne.

Zařízení se jmenuje BombVault, případně BombVault s názvem instance v závorce, a má tyto entity:

| Entita | Co ukazuje |
|---|---|
| Status | `ok`, `warning`, `failed` nebo `off`, nejhorší ze zapnutých oblastí |
| Running job | Co právě běží, nebo `idle` |
| Open anomalies | Kolik anomálií je otevřených |
| Next scheduled backup | Kdy začne další plánovaná záloha |
| *Oblast* last backup | Kdy proběhla poslední úspěšná záloha oblasti |
| *Oblast* last result | Jak skončila její poslední záloha |
| *Oblast* repository free space | Volné místo tam, kde leží její hlavní repozitář, pokud ho BombVault umí přečíst |
| Back up *oblast* | Tlačítko, které zálohuje celou oblast |

Názvy entit jsou anglicky, protože Home Assistant je přebírá tak, jak je BombVault posílá. Každá zapnutá oblast dostane vlastní entity a vypnutá o ně přijde. Tlačítka se objeví, jakmile zapneš **Tlačítka spouštějí zálohy**; nová instalace to má vypnuté. Mají stejné limity jako [spuštění přes API](#errors). Navíc BombVault bere pro každou oblast jen jedno stisknutí najednou a nejvýš šest za minutu a stisknutí, které broker uchoval jako retained zprávu, ignoruje. Stisknout je může každý, kdo smí do brokeru publikovat, proto brokeru dej heslo.

BombVault čte svůj stav každých 15 sekund a zveřejní ho, když se něco změnilo, jako JSON pod `<předpona>/<uzel>/state`. Předpona je `bombvault`, dokud ji nezměníš, a uzel je krátký identifikátor, který si BombVault jednou zvolí. Zprávy zjišťování jdou pod výchozí předponu Home Assistant `homeassistant`. Obojí se uchovává (retained). Poslední vůle (last will) označí zařízení jako nedostupné, když se BombVault zastaví bez ohlášení. Vypnutím propojení BombVault zařízení i jeho entity z Home Assistant odstraní.

## Najít BombVault v síti {#mdns}

BombVault ohlašuje své webové rozhraní v místní síti přes mDNS, protokol, na kterém stojí Bonjour a Avahi. Prohlížeč ho pak najde jako `https://bombvault.local:3443`, s `HTTP_ONLY` jako `http://bombvault.local:3000`, a prohlížeče služeb ho vypíšou jako webovou službu s podtypem `_bombvault`. Jeho záznamy TXT nesou verzi a cestu. Přepínač je v **Nastavení, Integrace, Najít v síti** a ve výchozím stavu je zapnutý. Pokud název už používá jiné zařízení, BombVault si vezme `bombvault-2.local` a tak dál a karta ukáže adresu, kterou dostal. Když se BombVault zastaví nebo ohlašování vypneš, dá o tom síti vědět a prohlížeče záznam hned odstraní.

Zda ohlášení doputuje do tvé sítě, záleží na tom, jak je kontejner připojený:

- **bridge**, výchozí volba v šabloně Unraid: ohlášení zůstane v síti Dockeru a v místní síti ho nikdo neuvidí. BombVault otevírej jako dosud přes adresu hostitele.
- **br0** nebo jiná síť macvlan či ipvlan: kontejner má vlastní adresu v místní síti a ohlášení se tam dostane.
- **host**: ohlášení jde ven přes rozhraní hostitele, vedle ohlášení samotného Unraidu. Mosty Dockeru a libvirtu vynechává.

Ohlašují se jen adresy IPv4.
