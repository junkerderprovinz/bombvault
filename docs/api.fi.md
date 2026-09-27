# API ja integraatiot

BombVaultissa on pieni HTTP-API skripteille, kojelaudoille ja kotiautomaatiolle. Se lukee samoja asioita, joita kojelauta näyttää, ja voi käynnistää varmuuskopion. Kaikki muu, kuten palautukset, kopioiden poistaminen ja asetukset, pysyy käyttöliittymässä.

## Tokenit {#tokens}

Jokainen pyyntö tarvitsee API-tokenin, vaikka kirjautumissalasanaa ei olisi asetettu. Luo token kohdassa **Asetukset, Järjestelmä, API-tokenit**:

1. Kirjoita nimi, joka kertoo, missä tokenia käytetään, esimerkiksi "Home Assistant" tai "Uptime Kuma".
2. Kytke **Salli varmuuskopioiden käynnistys** päälle, jos tokenin pitää voida käynnistää kopioita. Muuten se voi vain lukea.
3. Napsauta **Luo token**. Token näytetään kerran. BombVault säilyttää siitä vain sormenjäljen, joten kopioi se nyt.

Lähetä token otsakkeessa, joko `Authorization: Bearer <token>` tai `X-API-Key: <token>`. Token alkaa `bvapi_`. Se avaa vain API:n: MCP-avain ei toimi tässä, eikä token toimi MCP:lle.

Jokaisella tokenilla on ruutu, jossa näkyvät nimi, saako se käynnistää kopioita, neljä viimeistä merkkiä, milloin ja mistä sitä viimeksi käytettiin sekä tämän päivän kutsut. Ruudussa voit nimetä sen uudelleen, muuttaa sen oikeuksia, vaihtaa sen tai mitätöidä sen. **Loki** näyttää sen käynnistämät kopiot ja viimeisimmät kutsut. Kun palautat BombVaultin määritykset varmuuskopiosta, kaikki tokenit mitätöidään, koska kopiossa voi olla tokeneita, jotka mitätöit myöhemmin.

Ilman kirjautumissalasanaa jokainen, joka voi avata käyttöliittymän, voi myös luoda tokenin. Jos avaat BombVaultin julkiselta näyttävällä nimellä eikä salasanaa ole, siitä osoitteesta ei voi luoda tokeneita, sama sääntö kuin [MCP-avaimilla](mcp.md#switch-on).

## Päätepisteet {#endpoints}

| Reitti | Mitä se palauttaa tai tekee | Token |
|---|---|---|
| `GET /api/v1/health` | Versio, instanssin nimi, onko varmuuskopio käynnissä ja mitä tämä token saa tehdä | luku |
| `GET /api/v1/status` | Suojauksen tila alueittain: viimeisin onnistunut kopio, odotettu väli, tarkistukset, seuraavat ajastetut ajot | luku |
| `GET /api/v1/activity` | Mitä on käynnissä juuri nyt, vaiheen ja prosentin kanssa | luku |
| `GET /api/v1/items` | Jokainen suojattu kohde aikatauluineen, mitä kopio pysäyttää ja viimeisin kopio; `?domain=` yhdelle alueelle | luku |
| `GET /api/v1/runs` | Ajohistoria, uusimmat ensin; suodattimet `limit`, `domain`, `item`, `status`, `kind`, `since` | luku |
| `GET /api/v1/anomalies` | Poikkeamat ja yhteenveto avoimista; suodattimet `state`, `severity`, `domain`, `limit` | luku |
| `GET /api/v1/anomalies/{id}` | Yksi poikkeama | luku |
| `GET /api/v1/storage/{domain}` | Koon historia, kasvu viikossa ja vapaa tila alueen jokaisessa repositoryssä | luku |
| `POST /api/v1/backups` | Varmuuskopioi yhden kohteen (`{"domain":"containers","item":"plex"}`) tai koko alueen (`{"domain":"vms"}`) | käynnistys |
| `POST /api/v1/backups/everything` | Ajaa Backup Everythingin | käynnistys |
| `POST /api/v1/runs/{id}/cancel` | Peruu käynnissä olevan kopion, jonka tämä token käynnisti | käynnistys |

Alueet ovat `containers`, `vms`, `files`, `zfs`, `flash` ja `config`. Ajat ovat Unix-sekunteja. Vastaukset ovat samat kuin samannimisillä [MCP-työkaluilla](mcp.md#tools), joten ne pysyvät samoina.

Täällä käynnistetty kopio on sama kopio, jonka käyttöliittymä käynnistää: käynnissä oleva kontti pysäytetään, kunnes sen kopio on valmis. Pyyntö palaa heti, ja `/api/v1/activity` ja `/api/v1/runs` näyttävät, miten se etenee.

## Esimerkit {#examples}

```sh
# Miten varmuuskopiot voivat?
curl -s -H "Authorization: Bearer $BOMBVAULT_TOKEN" https://tower:3443/api/v1/status

# Varmuuskopioi yksi kontti nyt.
curl -s -X POST -H "Authorization: Bearer $BOMBVAULT_TOKEN" \
  -H "Content-Type: application/json" -d '{"domain":"containers","item":"plex"}' \
  https://tower:3443/api/v1/backups
```

BombVaultin omalla itse allekirjoitetulla varmenteella lisää `--cacert bombvault-cert.pem` (tiedosto saadaan MCP-kortin **Lataa varmenne** -painikkeesta) tai luotetussa verkossa `-k`.

## Virheet ja rajat {#errors}

Virhe palaa muodossa `{"error": {"code": "...", "message": "..."}}` vastaavalla tilakoodilla:

| Tila | Koodit | Merkitys |
|---|---|---|
| 400 | `invalid_argument`, `ambiguous` | Argumentti puuttuu tai on väärä |
| 401 | `no_token`, `invalid_token` | Ei tokenia, tai se ei ole aktiivinen |
| 403 | `not_permitted` | Token saa vain lukea, tai se ei käynnistänyt ajoa |
| 404 | `not_found` | Tällaista kohdetta, ajoa tai poikkeamaa ei ole |
| 409 | `busy`, `domain_off`, `nothing_to_back_up`, `not_running` | Jotain muuta on käynnissä, alue on pois päältä tai tehtävää ei ole |
| 429 | `throttled`, `rate_limited`, `cooldown`, `retention_guard` | Raja pidättää pyynnön; `Retry-After` kertoo, milloin voi yrittää uudelleen |

Käynnistyksiin pätevät samat rajat kuin [MCP:n kautta tehtyihin](mcp.md#starting-backups): 12 tunnissa tokenia kohti, 15 minuuttia saman kohteen kahden käynnistyksen välillä, enintään 4 saman kohteen käynnistystä 24 tunnissa sekä säilytyssuoja. Kolme viimeistä laskevat MCP:n ja API:n kautta tehdyt käynnistykset yhteen. Token voi tehdä 120 pyyntöä minuutissa. Viisi epäonnistunutta yritystä samasta osoitteesta lukitsee sen minuutiksi.

## OpenAPI {#openapi}

BombVault tarjoaa näiden reittien kuvauksen osoitteessa `/api/v1/openapi.json` (OpenAPI 3.1). Se ei vaadi tokenia. Lataa se Swagger UI:hin, Postmaniin tai koodigeneraattoriin.
