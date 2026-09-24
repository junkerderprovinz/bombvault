# MCP-palvelin

BombVaultissa on sisäänrakennettu palvelin Model Context Protocolia (MCP) varten. Sen protokollan kautta tekoälyavustajat, kuten Claude Code ja Claude Desktop, käyttävät ulkoisia työkaluja. Palvelimen kautta avustaja voi lukea, miten varmuuskopiosi voivat, ja jos sallit sen, käynnistää varmuuskopion tai perua sellaisen, jonka se itse käynnisti. Palvelin on pois päältä, kunnes luot avaimen: ilman aktiivista avainta päätepiste `/mcp` vastaa kaikkeen `404`.

## Mitä avustaja voi ja ei voi tehdä {#tools}

| Työkalu | Mitä se tekee | Laji |
|---|---|---|
| `get_health` | Versio, instanssin nimi, onko varmuuskopio käynnissä ja mitä tämä avain saa tehdä | luku |
| `get_status` | Suojauksen tila toimialueittain: viimeisin onnistunut varmuuskopio, odotettu väli, tarkistukset ja off-site-valvonta, seuraavat ajastetut ajot | luku |
| `get_coverage` | Mitä BombVault suojaa ja mitä ei, kunkin kohdalla syy | luku |
| `list_items` | Jokainen suojattu kontti, VM ja kansiojoukko, flash-muisti ja sovelluksen asetukset, ajastuksen, varmuuskopion pysäyttämien palveluiden, viimeisimmän varmuuskopion ja sen keston kera; tietokantakonteilla myös viimeisin dumppi; mukana ovat myös ZFS-datasetit viimeisimmän tarkistuksensa tuloksen kera | luku |
| `list_runs` | Ajohistoria uusimmat ensin, suodatettavissa toimialueen, kohteen, tilan, lajin ja ajan mukaan | luku |
| `list_restore_points` | Yhden kohteen palautuspisteet sen ensisijaisesta repositoriosta, ja kontille myös sen tietokantadumpit; ZFS-datasetillä on yksi palautuspiste varmuuskopiota kohden, ja siinä on snapshot jokaisesta sen alla olevasta datasetista | luku |
| `get_activity` | Mikä on käynnissä juuri nyt, vaiheen ja prosentin kera | luku |
| `get_storage_stats` | Toimialueen ensisijaisen repositorion koon historia ja kasvu viikossa | luku |
| `list_anomalies` | Epätavalliset varmuuskopiot, jotka BombVault on huomannut, suodatettavissa tilan, vakavuuden ja toimialueen mukaan, sekä yhteenveto avoimista | luku |
| `get_anomaly` | Yksi näistä havainnoista sekä muistiinpano, joka jätettiin sitä kuitatessa | luku |
| `start_backup` | Varmuuskopioi yhden kohteen heti | käynnistys |
| `start_domain_backup` | Varmuuskopioi toimialueen jokaisen suojatun kohteen | käynnistys |
| `start_backup_everything` | Ajaa Backup Everything -kierroksen | käynnistys |
| `cancel_backup` | Peruu käynnissä olevan varmuuskopion, jonka tämä avain käynnisti | peruutus |

Nämä jäävät verkkokäyttöliittymään: kaikenlaiset palautukset (myös tietokantadumpin lataaminen, tallentaminen tai tuonti), varmuuskopioiden poistaminen, prune, unlock, tarkistukset ja harjoitukset, off-site-replikointi, asetukset, tunnukset ja MCP-avaimet sekä sellaisen varmuuskopion peruminen, jonka ajastus, verkkokäyttöliittymä tai toinen avain käynnisti. Sama koskee poikkeaman kuittaamista tai sen merkitsemistä odotetuksi, mikä tehdään **Poikkeamat**-sivulla. Syy on se, että työkalujen vastauksissa on palvelimesi nimiä ja virheilmoituksia, ja mikä tahansa niistä voi sisältää tekstiä, joka on kirjoitettu ohjaamaan avustajaa. Avustaja, joka lankeaa sellaiseen, voi pahimmillaan käynnistää varmuuskopion alla olevien rajojen sisällä tai perua sellaisen, jonka se itse käynnisti.

Jos kohteen ensisijainen repositorio on muualla (S3, REST, SFTP, rclone), `list_restore_points` ottaa siihen yhteyttä, ja kutsu voi kestää hetken. Off-site-kopioita ei voi listata MCP:n kautta.

## Mitä käynnistetty varmuuskopio tekee {#starting-backups}

Avustajan varmuuskopio on sama varmuuskopio, jonka verkkokäyttöliittymä käynnistää. Käynnissä oleva kontti pysäytetään, kunnes sen varmuuskopio on valmis, yhdessä niiden konttien kanssa, jotka on asetettu pysähtymään sen mukana. VM, jolla on "graceful"-menetelmä, sammutetaan ja käynnistetään uudelleen. ZFS-datasetti pysäyttää sille asetetut kontit siksi aikaa, kun sen snapshot otetaan. Kansiojoukot, flash-muisti ja asetukset jatkavat toimintaansa. Sen jälkeen BombVault soveltaa säilytyskäytäntöä ja voi kopioida off-site-repositorioon. `list_items` kertoo avustajalle, mitä kohde pysäyttää ja kauanko sen viimeisin varmuuskopio kesti, ja työkalujen kuvaukset pyytävät sitä kertomaan sen sinulle ennen kuin se käynnistää mitään.

Koska varmuuskopio pysäyttää asioita ja työntää vanhoja palautuspisteitä ulos, MCP:n kautta tehtyjä käynnistyksiä on rajoitettu:

- 12 käynnistettyä varmuuskopiota tunnissa avainta kohden.
- 15 minuuttia saman kohteen, saman toimialueen tai Backup Everythingin kahden MCP-käynnistyksen välillä.
- Enintään 4 saman kohteen MCP-käynnistystä 24 tunnissa.
- **Säilytyssuoja.** Kun toimialue säilyttää kiinteän määrän palautuspisteitä (vain "säilytä viimeiset N" ilman päivittäistä, viikoittaista tai kuukausittaista sääntöä, paikallisesti tai off-site-kohteessa), jokainen uusi varmuuskopio työntää vanhimman ulos. BombVault hylkää silloin kohteen MCP-käynnistyksen, jos sen uusimmat N-1 onnistunutta varmuuskopiota on kaikki käynnistetty MCP:n kautta. Säilytettävään joukkoon jää siksi aina vähintään yksi palautuspiste, jonka ajastus tai sinä olet tehnyt. Asetuksella "säilytä viimeinen 1" avustaja ei voi varmuuskopioida kohdetta lainkaan. Seuraava ajastettu varmuuskopio tekee taas tilaa.

Toimialueen tai Backup Everythingin käynnistys jättää pois kohteet, jotka jokin raja pidättää, ja nimeää ne vastauksessaan. Mikään näistä ei koske verkkokäyttöliittymää eikä ajastusta. Tuntikiintiö on muistissa, joten BombVaultin uudelleenkäynnistys nollaa sen.

## Ota käyttöön {#switch-on}

1. Avaa **Asetukset, Järjestelmä, MCP-palvelin** ja napsauta **Uusi avain**.
2. Anna avaimelle nimi, joka kertoo, missä sitä käytetään, esimerkiksi "Claude Code läppärillä". Kun jokaisella asiakkaalla on oma avaimensa, voit peruuttaa yhden koskematta muihin.
3. Jätä **Salli varmuuskopioiden käynnistys** päälle, tai kytke se pois avaimelta, jonka pitää vain lukea. Voit muuttaa sitä myöhemmin avaimen rivillä, ja muutos koskee avustajan seuraavaa pyyntöä ilman uutta yhteyttä.
4. Napsauta **Luo avain**. Avain näytetään kerran. BombVault tallentaa siitä vain sormenjäljen eikä voi näyttää sitä uudelleen, joten kopioi se heti tai ota jokin alla olevista katkelmista, joissa on silloin oikea avain.

Ilman kirjautumissalasanaa itse verkkokäyttöliittymä on auki kaikille verkossasi, ja kuka tahansa, joka voi avata sen, voi myös luoda avaimen. Kortti kertoo tämän. Jos avaat BombVaultin julkiselta näyttävällä nimellä (esimerkiksi `bombvault.example.com` käänteisen välityspalvelimen takana) eikä kirjautumissalasanaa ole asetettu, siitä osoitteesta ei voi luoda eikä vaihtaa avaimia, jotta mikään internetin verkkosivu ei voi saada selaintasi luomaan sellaista. Aseta kirjautumissalasana, tai avaa BombVault sen IP-osoitteella tai paikallisella nimellä, kuten `tower` tai `tower.local`.

## Yhdistä asiakasohjelma {#clients}

Kortti näyttää valmiit katkelmat sille osoitteelle, jolla avasit sen: valitse asiakasohjelma ja kopioi katkelma. Loput tästä osiosta kertovat, mitä katkelmat tekevät, ja antavat muodot, joita kortti ei näytä.

### Claude Code {#claude-code}

Aja kortin komento kerran päätteessä. Varmenteella, johon tietokoneesi luottaa, se näyttää tältä:

```bash
claude mcp add --transport http bombvault --scope user https://bombvault.example.com/mcp --header "Authorization: Bearer <your key>"
```

Tarkista yhteys komennolla `/mcp` Claude Coden sisällä. `--scope user` tallentaa avaimen käyttäjäasetuksiisi eikä projektitiedostoon.

Komento sisältää avaimen, ja komentotulkkisi voi tallentaa sen historiaansa. Sen välttääksesi laita projektikansioon `.mcp.json` ja pidä avain ympäristömuuttujassa. Claude Code sijoittaa `${BOMBVAULT_MCP_KEY}`:n paikalle arvon lukiessaan tiedoston:

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

Aseta `BOMBVAULT_MCP_KEY` siellä, missä Claude Code käynnistyy, esimerkiksi komentotulkin profiiliin tekstieditorilla eikä kehotteeseen kirjoittamalla. Älä koskaan commitoi `.mcp.json`-tiedostoa, johon avain on kirjoitettu.

BombVaultin omalla varmenteella (katso [TLS ja varmenteet](#tls)) kortin komento käynnistää sen sijaan `mcp-remote`n ja ohjaa Node.js:n ladattuun varmenteeseen:

```bash
claude mcp add bombvault --scope user -e "NODE_EXTRA_CA_CERTS=<path of the downloaded bombvault-cert.pem>" -e "BOMBVAULT_MCP_KEY=<your key>" -- npx -y mcp-remote https://192.168.1.10:3443/mcp --header 'X-API-Key:${BOMBVAULT_MCP_KEY}'
```

Yksinkertaiset lainausmerkit estävät komentotulkkia laajentamasta muuttujaa; sen tekee `mcp-remote` itse. Sama muoto toimii `.mcp.json`-tiedostossa: käytä alla olevaa Claude Desktopin merkintää ja jätä `BOMBVAULT_MCP_KEY` pois sen `env`-osasta, jolloin avain tulee ympäristöstäsi.

### Claude Desktop {#claude-desktop}

Claude Desktop tavoittaa BombVaultin `mcp-remote`n kautta, joka tarvitsee Node.js:n kyseiselle tietokoneelle. Avaa asetustiedosto Claude Desktopissa kohdasta **Settings, Developer, Edit Config**. Se on Windowsissa polussa `%APPDATA%\Claude\claude_desktop_config.json` ja macOS:ssä polussa `~/Library/Application Support/Claude/claude_desktop_config.json`. Lisää kortin merkintä `"mcpServers"`-kohdan sisään muiden siellä jo olevien palvelinten viereen ja käynnistä Claude Desktop uudelleen:

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

- `NODE_EXTRA_CA_CERTS` on mukana vain BombVaultin omaa varmennetta varten. Jätä se pois, jos tietokoneesi jo luottaa varmenteeseen.
- `--allow-http` lisätään vain tavalliselle `http://`-osoitteelle.
- Otsake kirjoitetaan `X-API-Key:${BOMBVAULT_MCP_KEY}` ilman välilyöntiä kaksoispisteen jälkeen ja avain `env`-osassa. Joissakin järjestelmissä `mcp-remote` katkaisee `--header`-arvon ensimmäisen välilyönnin kohdalta, ja välilyönnin jälkeen kirjoitettu avain katoaisi.

### Omat liittimet Clauden asetuksissa {#custom-connectors}

Liittimiä (connectors), jotka lisäät Clauden omiin asetuksiin (claude.ai:ssa ja Claude Desktopin liitinluettelossa), ei vielä tueta. Niihin otetaan yhteys Anthropicin pilvestä, joten ne tarvitsevat julkisen HTTPS-osoitteen, ja ne kirjautuvat OAuthin kautta. Ne eivät voi lähettää kiinteää avainta, ja BombVault tarjoaa vain kiinteitä avaimia, ei OAuth-kirjautumista. BombVaultin vieminen internetiin niiden vuoksi ei auttaisi. Käytä Claude Codea tai Claude Desktopia `mcp-remote`n kautta kuten yllä.

### Muut asiakasohjelmat {#other-clients}

Mikä tahansa Streamable HTTP:tä puhuva asiakasohjelma käy:

- URL: verkkokäyttöliittymän osoite ja perään `/mcp`, esimerkiksi `https://192.168.1.10:3443/mcp`.
- Avain otsakkeessa `Authorization: Bearer <key>` tai `X-API-Key: <key>`. Jos molemmat lähetetään, niissä on oltava sama avain.
- `POST`, jossa `Content-Type: application/json` ja `Accept: application/json, text/event-stream`.
- Yksi JSON-RPC-viesti pyyntöä kohden; eräpyynnöt (batch) hylätään.
- Protokollaversiot 2026-07-28, 2025-11-25, 2025-06-18 ja 2025-03-26.

## TLS ja varmenteet {#tls}

BombVault tarjoaa HTTPS:n itse myöntämällään varmenteella, ja aluksi se varmenne nimeää vain `localhost`, `127.0.0.1` ja `::1`. Claude Code ja `mcp-remote` hylkäävät sen lähiverkon osoitteessa. Kiertotiet siinä järjestyksessä, joka sopii useimpiin Unraid-asennuksiin:

1. **Lisää osoite MCP-kortissa.** Kun kortti avataan HTTPS:llä osoitteessa, jota varmenne ei nimeä, kortti kertoo sen ja tarjoaa painiketta **Lisää tämä osoite varmenteeseen**. BombVault myöntää silloin varmenteensa uudelleen niin, että osoite on mukana (selaimesi varoittaa vielä kerran, kuten ensimmäisellä kerralla). Napsauta sitten **Lataa varmenne**; katkelmat asettavat `NODE_EXTRA_CA_CERTS`:n ladattuun tiedostoon, joten asiakasohjelma luottaa juuri siihen varmenteeseen.
2. **Käänteinen välityspalvelin luotetulla varmenteella** (Nginx Proxy Manager, SWAG, Caddy, Traefik). Asiakasohjelma näkee silloin välityspalvelimen varmenteen eikä tarvitse muuta, eikä kortti varoita BombVaultin omasta.
3. **Tailscale.** `tailscale serve` kontin edessä tai Unraidin Tailscale-integraatio antaa sinulle `ts.net`-nimen luotetulla varmenteella.
4. **`HTTP_ONLY=true`**, vain TLS:n päättävän välityspalvelimen takana tai verkossa, johon luotat täysin. Se vaihtaa koko verkkokäyttöliittymän tavalliseen HTTP:hen, vaatii muutoksen kontin asetuksiin ja lähettää avaimen salaamattomana.

Älä koskaan aseta `NODE_TLS_REJECT_UNAUTHORIZED=0`. Se kytkee varmennetarkistuksen pois kaikelta, minkä kanssa se Node.js-prosessi keskustelee.

Käänteisen välityspalvelimen on välitettävä otsake `Authorization` (tai `X-API-Key`), mitä välityspalvelimet tekevät, ellei niitä käsketä toisin, eikä se saa puskuroida tai kirjoittaa uudelleen `/mcp`:tä. Location-lohko Nginxille tai Nginx Proxy Managerille, joka tarkistaa myös BombVaultin varmenteen:

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

Välityspalvelimen takana jokaisessa pyynnössä on välityspalvelimen osoite. Viisi väärää avainta yhdeltä väärin määritetyltä asiakkaalta lukitsee silloin kaikki saman välityspalvelimen takana olevat MCP-asiakkaat ulos minuutiksi. Nimeä välityspalvelin muuttujassa `TRUSTED_PROXY` (katso [Määritykset](configuration.md)), niin laskenta tehdään asiakaskohtaisesti.

## Turvallisuusmalli {#security}

- Ilman aktiivista avainta `/mcp` vastaa `404`.
- Mikään osoite ei ole poikkeus. Pyynnöt osoitteesta `localhost`, Unraid-isännältä, käänteiseltä välityspalvelimelta tai `tailscale serve`:ltä tarvitsevat avaimen kuten kaikki muutkin, myös silloin kun verkkokäyttöliittymällä ei ole kirjautumissalasanaa.
- Avaimet tallennetaan vain sormenjälkinä, näytetään kerran, ja ne voi nimetä uudelleen, vaihtaa ja peruuttaa. Enintään 10 aktiivista avainta, kullakin oma kytkin **Saa käynnistää varmuuskopioita**.
- Jokainen luonti, vaihto, oikeuksien muutos ja peruutus lähettää ilmoituksen ilmoituskanaviesi kautta osoitteen kera, josta se tuli, ellei ilmoituksia ole kytketty pois.
- 5 väärää avainta minuutissa osoitetta kohden, sen jälkeen `429`. 120 pyyntöä minuutissa ja 12 käynnistettyä varmuuskopiota tunnissa avainta kohden, lisäksi yllä kuvattu odotusaika ja säilytyssuoja.
- Toisesta originista tulevan selainsivun pyynnöt hylätään.
- Niin kauan kuin kirjautumissalasanaa ei ole asetettu, avaimia ei voi luoda julkiselta näyttävästä isäntänimestä.
- Jokainen avustajan käynnistämä varmuuskopio ja siitä seuraavat prune- ja off-site-ajot merkitään "MCP:n kautta" avaimen nimen kera toimintalokiin, virhepaneeliin ja varmuuskopion ilmoitukseen.
- Jokainen työkalukutsu kirjoitetaan kontin lokiin avaimen tunnisteen ja sen neljän viimeisen merkin kera (ei koskaan nimeä) ja lasketaan `/metrics`-sivulla (`bombvault_mcp_requests_total`, `bombvault_mcp_tool_calls_total`, `bombvault_mcp_active_keys`).
- Asetusten varmuuskopion palauttaminen peruuttaa kaikki avaimet, koska palautettu tietokanta voi sisältää avaimia, jotka peruutit sen tallentamisen jälkeen. Luo uudet avaimet sen jälkeen.
- Avain lakkaa toimimasta, kun `APP_KEY` muuttuu (uudelleenasennus tai palautus toiseen konttiin). Kortti huomaa sen ja merkitsee avaimen, ja **Vaihda avain** antaa sille taas kelvollisen salaisuuden.
- Käsittele avainta kuin salasanaa. Claude Code ja Claude Desktop säilyttävät sen selväkielisenä asetuksissaan. Tietokoneella, johon luotat vähemmän, käytä mieluummin avainta, joka saa vain lukea.

## Mitä koneelta lähtee {#privacy}

Kaikki, mitä avustaja lukee, menee sen takana olevalle tekoälypalvelun tarjoajalle: kohteiden nimet, ajastukset, ajohistoria virheilmoituksineen, palautuspisteiden tunnisteet ja ajat, tietokantamoottorien nimet ja dumppien koot, käynnissä oleva toiminta, tallennustilan luvut, kattavuus ja tila. BombVault poistaa isännän polut, repositorioiden sijainnit, isäntänimet, tunnukset, hook-komennot ja avaimet ennen kuin mitään lähtee.

## Vianmääritys {#troubleshooting}

| Mitä näet | Mitä se tarkoittaa |
|---|---|
| `404` | Ei aktiivista avainta, tai väärä polku kuten `/api/mcp`. Päätepiste on `/mcp`. |
| `401` | Avain puuttuu, on kirjoitettu väärin, peruutettu tai vaihdettu. Ehkä välityspalvelin pudottaa otsakkeen `Authorization` (kokeile `X-API-Key`). Jos kortti merkitsee avaimen enää kelpaamattomaksi, `APP_KEY` on muuttunut: vaihda avain. |
| `403` | Pyyntö tuli selainsivulta, jolla on eri origin. Käytä työpöytä- tai komentoriviasiakasta. |
| `405` GET-pyynnöllä | Normaalia. Päätepiste ottaa vastaan vain `POST`-pyyntöjä. |
| `400` "Accept must contain both 'application/json' and 'text/event-stream'" | Asiakasohjelma on liian vanha Streamable HTTP:lle. Päivitä se. |
| `400` "batch requests are not accepted" | Asiakasohjelma lähettää JSON-RPC-eriä. Lähetä yksi viesti pyyntöä kohden. |
| `429` | Liian monta väärää avainta tästä osoitteesta, tai yli 120 pyyntöä minuutissa yhdellä avaimella. Odota minuutti ja tarkista, onko avustaja jumissa silmukassa. |
| Virheet, joissa lukee "certificate", "self-signed" tai "unable to verify" | Asiakasohjelma ei luota BombVaultin varmenteeseen. Katso [TLS ja varmenteet](#tls). |
| `busy` | Toinen varmuuskopio tai ylläpitotehtävä varaa toimialueen. Yritä uudelleen, kun se on valmis. |
| `cooldown` | Tämä kohde, tämä toimialue tai Backup Everything käynnistettiin MCP:n kautta alle 15 minuuttia sitten. |
| `retention_guard` | Vielä yksi MCP-varmuuskopio jättäisi "säilytä viimeiset N" -ikkunaan vain MCP:n tekemiä palautuspisteitä. Seuraava ajastettu varmuuskopio tekee tilaa, tai käynnistä se verkkokäyttöliittymästä. |
| `rate_limited` | Avain on käyttänyt tämän tunnin 12 käynnistystään. |
| `not_permitted` käynnistyksessä | Avain saa vain lukea. Kytke **Saa käynnistää varmuuskopioita** päälle kortissa; uutta yhteyttä ei tarvita. Peruutuksessa se tarkoittaa, että tämä avain ei käynnistänyt ajoa. |
| `domain_off` | Se varmuuskopiolaji on kytketty pois asetuksista. |
| `not_found` | BombVault ei suojaa sitä kohdetta. Lisää se ensin verkkokäyttöliittymässä; MCP ei koskaan luo asetuksia. |

Älä aseta kontille ympäristömuuttujaa `MCPGODEBUG`. Se muuttaa MCP-kirjaston toimintaa, ja virheellinen arvo pysäyttää BombVaultin käynnistyksessä ennen kuin se kirjoittaa yhtäkään lokiriviä.
