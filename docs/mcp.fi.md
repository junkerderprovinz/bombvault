# MCP-palvelin

BombVaultissa on sisäänrakennettu palvelin Model Context Protocolille (MCP), jolla tekoälyavustajat kuten Claude Code ja Claude Desktop tavoittavat ulkoisia työkaluja. Sen kautta avustaja voi lukea, miten varmuuskopiosi voivat, ja jos sallit, käynnistää varmuuskopion tai perua itse käynnistämänsä. Palvelin on pois päältä, kunnes luot avaimen tai otat [OAuth-kirjautumisen](#oauth) käyttöön: siihen asti päätepiste `/mcp` vastaa kaikkeen `404`.

## Mitä avustaja voi ja ei voi tehdä {#tools}

| Työkalu | Mitä se tekee | Laji |
|---|---|---|
| `get_health` | Versio, instanssin nimi, onko varmuuskopio käynnissä ja mitä tämä avain saa tehdä | luku |
| `get_status` | Suojauksen tila toimialueittain: viimeisin onnistunut varmuuskopio, odotettu väli, tarkistukset ja off-site-valvonta, seuraavat ajastetut ajot sekä konteille uusin käynnistystesti | luku |
| `get_coverage` | Mitä BombVault suojaa ja mitä ei, kunkin kohdalla syy | luku |
| `list_items` | Jokainen suojattu kontti, VM ja kansiojoukko, flash-muisti ja sovelluksen asetukset, ajastuksen, varmuuskopion pysäyttämien palveluiden, viimeisimmän varmuuskopion ja sen keston kera; tietokantakonteilla myös viimeisin dumppi; mukana ovat myös ZFS-datasetit viimeisimmän tarkistuksensa tuloksen kera; jokaisella kohteella on viimeisin palautustarkistuksensa ja kontilla lisäksi viimeisin käynnistystesti tai syy, miksi sitä ei voi testata | luku |
| `list_runs` | Ajohistoria uusimmat ensin, suodatettavissa toimialueen, kohteen, tilan, lajin ja ajan mukaan | luku |
| `list_restore_points` | Yhden kohteen palautuspisteet sen ensisijaisesta repositoriosta, ja kontille myös sen tietokantadumpit; ZFS-datasetillä on yksi palautuspiste varmuuskopiota kohden, ja siinä on snapshot jokaisesta sen alla olevasta datasetista | luku |
| `get_activity` | Mikä on käynnissä juuri nyt, vaiheen ja prosentin kera | luku |
| `get_storage_stats` | Toimialueen ensisijaisen repositorion koon historia ja kasvu viikossa sekä käytetty, vapaa ja kokonaistila kunkin sen repositorion levyllä tai etäkohteessa | luku |
| `list_anomalies` | Poikkeamat, jotka BombVault on huomannut varmuuskopioissa, suodatettavissa tilan, vakavuuden ja toimialueen mukaan, sekä yhteenveto avoimista | luku |
| `get_anomaly` | Yksi näistä havainnoista sekä muistiinpano, joka jätettiin sitä kuitatessa | luku |
| `start_backup` | Varmuuskopioi yhden kohteen heti | käynnistys |
| `start_domain_backup` | Varmuuskopioi toimialueen jokaisen suojatun kohteen | käynnistys |
| `start_backup_everything` | Ajaa Backup Everything -kierroksen | käynnistys |
| `cancel_backup` | Peruu käynnissä olevan varmuuskopion, jonka tämä avain käynnisti | peruutus |

Nämä jäävät verkkokäyttöliittymään: kaikenlaiset palautukset (myös tietokantadumpin lataaminen, tallentaminen tai tuonti), varmuuskopioiden poistaminen, prune, unlock, tarkistukset ja harjoitukset, off-site-replikointi, asetukset, tunnukset ja MCP-avaimet sekä sellaisen varmuuskopion peruminen, jonka ajastus, verkkokäyttöliittymä tai toinen avain käynnisti. Sama koskee poikkeaman kuittaamista tai sen merkitsemistä odotetuksi, mikä tehdään **Poikkeamat**-sivulla. Syy on se, että työkalujen vastauksissa on palvelimesi nimiä ja virheilmoituksia, ja mikä tahansa niistä voi sisältää tekstiä, joka on kirjoitettu ohjaamaan avustajaa. Avustaja, joka lankeaa sellaiseen, voi pahimmillaan käynnistää varmuuskopion alla olevien rajojen sisällä tai perua sellaisen, jonka se itse käynnisti.

Jos kohteen ensisijainen repositorio on muualla (S3, REST, SFTP, rclone), `list_restore_points` ottaa siihen yhteyttä, ja kutsu voi kestää hetken. Off-site-kopioita ei voi listata MCP:n kautta. Mitä poikkeamien tarkistukset katsovat, kerrotaan sivulla [Ominaisuudet](features.md), ja miten ZFS-kohde pitää yhden tilannevedoksen tietojoukkoa kohden, sivulla [ZFS-tietojoukot](zfs-datasets.md#contents).

## Mitä käynnistetty varmuuskopio tekee {#starting-backups}

Avustajan varmuuskopio on sama varmuuskopio, jonka verkkokäyttöliittymä käynnistää. Käynnissä oleva kontti pysäytetään, kunnes sen varmuuskopio on valmis, yhdessä niiden konttien kanssa, jotka on asetettu pysähtymään sen mukana. VM, jolla on "graceful"-menetelmä, sammutetaan ja käynnistetään uudelleen. ZFS-datasetti pysäyttää sille asetetut kontit siksi aikaa, kun sen snapshot otetaan. Kansiojoukot, flash-muisti ja asetukset jatkavat toimintaansa. Sen jälkeen BombVault soveltaa säilytyskäytäntöä ja voi kopioida off-site-repositorioon. `list_items` kertoo avustajalle, mitä kohde pysäyttää ja kauanko sen viimeisin varmuuskopio kesti, ja työkalujen kuvaukset pyytävät sitä kertomaan sen sinulle ennen kuin se käynnistää mitään.

Koska varmuuskopio pysäyttää asioita ja työntää vanhoja palautuspisteitä ulos, MCP:n kautta tehtyjä käynnistyksiä on rajoitettu:

- 12 käynnistettyä varmuuskopiota tunnissa avainta kohden.
- 15 minuuttia saman kohteen, saman toimialueen tai Backup Everythingin kahden MCP-käynnistyksen välillä.
- Enintään 4 saman kohteen MCP-käynnistystä 24 tunnissa.
- **Säilytyssuoja.** Kun toimialue säilyttää kiinteän määrän palautuspisteitä (vain "säilytä viimeiset N" ilman päivittäistä, viikoittaista tai kuukausittaista sääntöä, paikallisesti tai off-site-kohteessa), jokainen uusi varmuuskopio työntää vanhimman ulos. BombVault hylkää silloin kohteen MCP-käynnistyksen, jos sen uusimmat N-1 onnistunutta varmuuskopiota on kaikki käynnistetty MCP:n kautta. Säilytettävään joukkoon jää siksi aina vähintään yksi palautuspiste, jonka ajastus tai sinä olet tehnyt. Asetuksella "säilytä viimeinen 1" avustaja ei voi varmuuskopioida kohdetta lainkaan. Seuraava ajastettu varmuuskopio tekee taas tilaa.

Toimialueen tai Backup Everythingin käynnistys jättää pois kohteet, jotka jokin raja pidättää, ja nimeää ne vastauksessaan. Mikään näistä ei koske verkkokäyttöliittymää eikä ajastusta. Tuntikiintiö on muistissa, joten BombVaultin uudelleenkäynnistys nollaa sen.

## Ota käyttöön {#switch-on}

1. Avaa **Asetukset, Järjestelmä, MCP-palvelin** ja napsauta asiakasohjelmasi painiketta. Luettelosta puuttuva asiakasohjelma yhdistää kohdan **Muu asiakasohjelma** kautta.
2. Jätä kohtaan **Avain** valinta **Uusi avain** ja ehdotettu nimi, joka on asiakasohjelman nimi, tai kirjoita nimi, joka kertoo, missä avainta käytetään, esimerkiksi ”Claude Code kannettavalla”. Yksi avain asiakasohjelmaa kohden antaa perua yhden koskematta muihin. **Olemassa oleva avain** antaa asiakasohjelmalle aiemmin luomasi avaimen.
3. Kytke **Salli varmuuskopioiden käynnistys** päälle avaimelle, jonka pitää voida käynnistää varmuuskopioita; ilman sitä avain voi vain lukea. Voit muuttaa sitä myöhemmin avaimen ruudussa, ja muutos pätee avustajan seuraavasta pyynnöstä alkaen ilman uutta yhteyttä.
4. Napsauta **Luo avain**. Avain näytetään kerran. BombVault säilyttää siitä vain sormenjäljen eikä voi näyttää sitä uudelleen, joten kopioi se nyt. Jos suljet ikkunan ennen kuin asiakasohjelma on käyttänyt avainta, kortti näyttää sitä edelleen, kunnes vahvistat kopioineesi sen.

Ilman kirjautumissalasanaa itse verkkokäyttöliittymä on auki kaikille verkossasi, ja kuka tahansa, joka voi avata sen, voi myös luoda avaimen. Kortti kertoo tämän. Jos avaat BombVaultin julkiselta näyttävällä nimellä (esimerkiksi `bombvault.example.com` käänteisen välityspalvelimen takana) eikä kirjautumissalasanaa ole asetettu, siitä osoitteesta ei voi luoda eikä vaihtaa avaimia, jotta mikään internetin verkkosivu ei voi saada selaintasi luomaan sellaista. Aseta kirjautumissalasana, tai avaa BombVault sen IP-osoitteella tai paikallisella nimellä, kuten `tower` tai `tower.local`.

## Avaimesi ja niiden loki {#keys}

Jokaisella avaimella on kortilla oma ruutunsa. Siinä näkyy avaimen nimi, saako se käynnistää varmuuskopioita vai vain lukea, avaimen neljä viimeistä merkkiä, milloin se luotiin tai korvattiin viimeksi, milloin asiakas viimeksi käytti sitä ja montako kutsua se on tehnyt tänään. Ruudussa nimeät avaimen uudelleen, muutat sen oikeutta, korvaat sen tai peruutat sen. Peruutettu avain siirtyy peruutettujen avainten luetteloon, josta voit poistaa sen lopullisesti, kun mikään historian ajo ei enää mainitse sitä.

Nimen vieressä ruutu näyttää sen asiakasohjelman merkin, jolle avain luotiin. Kohdan **Muu asiakasohjelma** kautta tai ennen asiakasohjelmien luetteloa luotu avain näyttää sen sijaan avaimen.

Ruudun **Loki** avaa sen, mitä avain on tehnyt. Ensin tulevat sen käynnistämät varmuuskopiot, kukin tilansa kanssa ja linkillä ajoon kojelaudan toimintalokissa. Niiden alla ovat sen kutsut uusin ensin, työkalu ja kutsun lopputulos. Hylkäys kertoo syyn: avain saa vain lukea, säilytyssuoja pidätti varmuuskopion, toinen varmuuskopiointi oli jo käynnissä, kohde varmuuskopioitiin MCP:n kautta muutama minuutti sitten, tai avain lähetti liian monta pyyntöä. Peruutus linkittää ajoon, jota se koski.

BombVault säilyttää kunkin avaimen merkinnät enintään 30 päivää: 500 uusinta onnistunutta käynnistystä ja peruutusta sekä niiden rinnalla 200 uusinta muuta kutsua (luvut, hylkäykset ja virheet), joten avustaja, joka kyselee käynnissä olevaa varmuuskopiointia tai yrittää hylättyä kutsua yhä uudelleen, ei voi työntää sen käynnistystä pois lokista. Jokaisesta kutsusta se tallentaa työkalun, lopputuloksen ja peruutuksen nimeämän ajon. Se ei koskaan tallenna sitä, mitä avustaja lähetti, eikä avainta tai sen sormenjälkeä. Diagnostiikkapaketti vain laskee merkinnät, ja asetusten vienti jättää ne pois.

## Yhdistä asiakasohjelma {#clients}

Jokaisella asiakasohjelmalla on kortilla painike, otsikon **Tällä tietokoneella** tai **Pilvessä** alla. Painike avaa kolmivaiheisen ikkunan: avain; asiakasohjelman määritys osoitteella, jolla avasit kortin, kopiointipainike, määrityksen sijainti ja BombVaultin omaa varmennetta käytettäessä se, mitä asiakasohjelma tarvitsee luottaakseen siihen; sekä asiakasohjelman ensimmäisen kutsun odotus. Ikkuna seuraa avaimen viimeisintä käyttöä ja muuttuu vihreäksi, kun kutsu saapuu.

Ikkuna pitää avaimen poissa kaikilta komentoriveiltä. Kun asiakasohjelma osaa lukea sen ympäristömuuttujasta (`BOMBVAULT_MCP_KEY`), piilotetusta kyselystä tai omasta tiedostostaan, määritys vain mainitsee sen. Kun asiakasohjelmalla ei ole sellaista keinoa, avain on sen määritystiedostossa tai asetuksissa, ja ikkuna kertoo sen. Kun asiakasohjelman dokumentaatio ei kerro, miten se kohtelee tuntematonta varmennetta, ikkuna kirjoittaa vaiheen ohjeeksi siltä varalta, että asiakasohjelma hylkää BombVaultin varmenteen.

| Asiakasohjelma | Käyttöönotto | Mistä avain tulee |
|---|---|---|
| AnythingLLM | määritystiedosto | määritystiedosto |
| Antigravity | määritystiedosto | ympäristömuuttuja |
| Claude Code | komento | avaintiedosto |
| Claude Desktop | määritystiedosto | avaintiedosto |
| Cline | määritystiedosto | määritystiedosto |
| Codex CLI | määritystiedosto | ympäristömuuttuja |
| Continue | määritystiedosto | `~/.continue/.env` |
| Copilot CLI | määritystiedosto | määritystiedosto |
| Cursor | määritystiedosto | ympäristömuuttuja |
| Gemini CLI | määritystiedosto | ympäristömuuttuja |
| GitHub Copilot (VS Code) | määritystiedosto | piilotettu kysely |
| Goose | määritystiedosto | ympäristömuuttuja |
| Jan | lomake sovelluksessa | sovelluksen asetukset |
| JetBrains (AI Assistant, Junie) | määritystiedosto | määritystiedosto |
| Kimi Code | määritystiedosto | määritystiedosto |
| LM Studio | määritystiedosto | määritystiedosto |
| Mistral Vibe | määritystiedosto | ympäristömuuttuja |
| Msty | lomake sovelluksessa | sovelluksen asetukset |
| n8n | lomake sovelluksessa | n8n:n tunnistetiedot |
| Open WebUI | lomake sovelluksessa | sovelluksen asetukset |
| opencode | määritystiedosto | ympäristömuuttuja |
| Perplexity (Mac) | lomake sovelluksessa | avaintiedosto |
| Qwen Code | määritystiedosto | ympäristömuuttuja |
| Roo Code | määritystiedosto | ympäristömuuttuja |
| Visual Studio | määritystiedosto | määritystiedosto |
| Warp | määritystiedosto | määritystiedosto |
| Windsurf | määritystiedosto | ympäristömuuttuja |
| Zed | määritystiedosto | määritystiedosto |
| Grok | lomake, pilvessä | palveluntarjoajan palvelimet |
| Le Chat | lomake, pilvessä | palveluntarjoajan palvelimet |
| ChatGPT | OAuth-kirjautuminen, pilvessä | käyttötunnus, katso [alta](#oauth) |
| Claude (claude.ai) | OAuth-kirjautuminen, pilvessä | käyttötunnus, katso [alta](#oauth) |

Alla olevat osiot selittävät Claude Coden ja Claude Desktopin käyttöönoton tarkemmin ja kertovat, mitä muu asiakasohjelma tarvitsee.

### Claude Code {#claude-code}

Claude Code tavoittaa BombVaultin `mcp-remote`n kautta, joka tarvitsee Node.js:n kyseiselle tietokoneelle. Tallenna ensin avain omaan tekstitiedostoonsa yhdelle riville:

```text
X-API-Key: <your key>
```

Aja sitten kortin komento kerran päätteessä niin, että siihen on täytetty tämän tiedoston polku. Varmenteella, johon tietokoneesi luottaa, se näyttää tältä:

```bash
claude mcp add bombvault --scope user -- npx -y mcp-remote@latest https://bombvault.example.com/mcp --header-file "<path of the file with your key>"
```

BombVaultin omalla varmenteella (katso [TLS ja varmenteet](#tls)) komento ohjaa lisäksi Node.js:n ladattuun varmenteeseen:

```bash
claude mcp add bombvault --scope user -e "NODE_EXTRA_CA_CERTS=<path of the downloaded bombvault-cert.pem>" -- npx -y mcp-remote@latest https://192.168.1.10:3443/mcp --header-file "<path of the file with your key>"
```

Tarkista yhteys komennolla `/mcp` Claude Coden sisällä. `--scope user` tuo BombVaultin käyttöön kaikissa projekteissasi. Claude Code tallentaa vain avaintiedoston polun, joten avain ei näy komennossa eikä komentotulkkisi historiassa, eikä prosessiluettelossa. Pidä tiedosto paikassa, jossa vain sinä voit lukea sen, ja minkään commitoitavan kansion ulkopuolella. `@latest` saa `npx`:n hakemaan ajantasaisen `mcp-remote`n; muuten käytettäisiin vanhempaa, globaalisti asennettua versiota, joka ei tunne valitsinta `--header-file`.

Älä kirjoita viittausta `${BOMBVAULT_MCP_KEY}` Claude Coden `mcp-remote`-argumentteihin. Claude Code täyttää tällaisen viittauksen omasta ympäristöstään ennen kuin se käynnistää `mcp-remote`n, joten avain päätyy sen prosessin komentoriville, josta tietokoneen muut ohjelmat ja käyttäjät voivat lukea sen.

Ilman Node.js:ää ja vain varmenteella, johon tietokoneesi luottaa, Claude Code voi muodostaa yhteyden itse. Laita projektikansioon `.mcp.json`:

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

Aseta `BOMBVAULT_MCP_KEY` siellä, missä Claude Code käynnistyy, esimerkiksi tiedoston `~/.claude/settings.json` kohtaan `"env"` tai komentotulkin profiiliin tekstieditorilla eikä kehotteeseen kirjoittamalla. Tässä viittaus on turvallinen, koska Claude Code ei käynnistä toista prosessia, johon avain päätyisi. BombVaultin oma varmenne ei toimi tällä tavalla: Claude Coden oma yhteys hylkää sen, vaikka `NODE_EXTRA_CA_CERTS` olisi asetettu. Älä koskaan commitoi `.mcp.json`-tiedostoa, johon avain on kirjoitettu.

### Claude Desktop {#claude-desktop}

Claude Desktop tavoittaa BombVaultin `mcp-remote`n kautta, joka tarvitsee Node.js:n kyseiselle tietokoneelle. Tallenna avain ensin omaan tekstitiedostoonsa yhdeksi riviksi, kuten [Claude Coden](#claude-code) kohdalla kuvataan. Avaa asetustiedosto Claude Desktopissa kohdasta **Settings, Developer, Edit Config**. Se on Windowsissa polussa `%APPDATA%\Claude\claude_desktop_config.json` ja macOS:ssä polussa `~/Library/Application Support/Claude/claude_desktop_config.json`. Lisää kortin merkintä `"mcpServers"`-kohdan sisään muiden siellä jo olevien palvelinten viereen ja käynnistä Claude Desktop uudelleen:

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

- `NODE_EXTRA_CA_CERTS` on mukana vain BombVaultin omaa varmennetta varten. Jätä se pois, jos tietokoneesi jo luottaa varmenteeseen.
- `--allow-http` lisätään vain tavalliselle `http://`-osoitteelle.
- Kirjoita Windowsissa polut tavallisilla kauttaviivoilla, esimerkiksi `C:/Users/sam/bombvault-key.txt`, sillä yksittäinen kenoviiva ei ole kelvollista JSONia. Pidä avaintiedoston polku vapaana välilyönneistä: Windowsin Claude Desktop antaa välilyönnin sisältävän polun `npx`:lle kahtena palana.
- Asetuksissa mainitaan vain avaintiedosto, joten avain ei näy niissä eikä prosessiluettelossa. Pidä tiedosto paikassa, jossa vain sinä voit lukea sen.

### Asiakasohjelmat pilvessä {#cloud-clients}

ChatGPT, claude.ai:n Claude, Grok ja Le Chat kutsuvat BombVaultia toimittajansa palvelimilta, joten BombVaultin täytyy näkyä internetiin julkisesti luotetulla varmenteella, esimerkiksi käänteisen välityspalvelimen takana; Le Chat hylkää itse allekirjoitetut. Välityspalvelimen kirjautuminen voi suojata verkkokäyttöliittymää, mutta `/mcp`:n täytyy päästä BombVaultiin ilman sitä: nämä palvelut eivät osaa kirjautua välityspalvelimeen, ja BombVault tarkistaa niiden avaimen tai tunnuksen itse. Grok ja Le Chat lähettävät kiinteän avaimen, ja niiden painikkeet ottavat ne käyttöön kuten muutkin. ChatGPT ja useimmissa organisaatioissa myös claude.ai:n Claude yhdistävät vain OAuth-kirjautumisella, joka kuvataan seuraavaksi.

### OAuth-kirjautuminen {#oauth}

Asiakkaalle, joka ei ota vastaan avainta, BombVault on sen oma OAuth-valtuutuspalvelin. Asiakas rekisteröityy itse, ohjaa sinut BombVaultin sivulle, ja siellä kirjaudut kirjautumissalasanallasi (ja toisella tekijällä, jos olet ottanut sen käyttöön) ja sallit sen. Asiakas saa sitten tunnuksen, joka kelpaa vain tämän BombVaultin MCP-päätepisteeseen, ja uusii sen itse.

1. Aseta kirjautumissalasana kohdassa **Asetukset, Järjestelmä**. Ilman sitä BombVault ei tarjoa kirjautumista lainkaan, koska suostumusta ei olisi keneltä kysyä.
2. Tee BombVault tavoitettavaksi internetistä https-yhteydellä ja varmenteella, johon selaimet luottavat, yleensä käänteisen välityspalvelimen kautta. Asiakas kutsuu polkuja `/mcp`, `/oauth/` ja `/.well-known/` omilta palvelimiltaan, joten välityspalvelimen, jolla on oma kirjautuminen, täytyy päästää nämä kolme polkua läpi BombVaultiin. Suostumussivu osoitteessa `/oauth/authorize` aukeaa omassa selaimessasi ja saa jäädä välityspalvelimen kirjautumisen taakse. Nimeä välityspalvelin myös muuttujassa `TRUSTED_PROXY` (katso [Määritykset](configuration.md)). BombVault rajoittaa asiakkaiden rekisteröintejä osoitekohtaisesti, ja ilman sitä jokainen asiakas näyttää tulevan välityspalvelimelta.
3. Ota MCP-kortilla käyttöön **OAuth-kirjautuminen** ja anna **Julkinen osoite**: https-osoite ilman polkua, esimerkiksi `https://backup.example.com`. Jokainen tunnus on sidottu tähän osoitteeseen, joten muutoksen jälkeen jokaisen asiakkaan on kirjauduttava uudelleen.
4. Napsauta ChatGPT:n tai Clauden painiketta. Ikkuna näyttää **Liittimen URL**:n eli julkisen osoitteen, jonka perässä on `/mcp`, ja kertoo, mihin se kyseisessä asiakkaassa kuuluu. ChatGPT:ssä otat kehittäjätilan käyttöön kohdassa **Asetukset, Sovellukset ja liittimet, Lisäasetukset**, valitset **Luo**, liität liittimen URL:n MCP-palvelimen URL:ksi ja valitset todennukseksi OAuthin. claude.ai:ssa avaat **Asetukset, Liittimet, Lisää mukautettu liitin**, liität liittimen URL:n, jätät OAuth-asiakastunnuksen ja salaisuuden tyhjiksi ja valitset **Yhdistä**.
5. Asiakas avaa suostumussivun. Se näyttää, kuka kysyy, minne vastauksesi vie sinut takaisin, sekä kytkimen **Salli varmuuskopioiden käynnistys**, joka on aluksi pois päältä. Valitse **Salli** tai **Estä**.

Jokainen kirjautunut asiakas saa ruudun avainten viereen, jossa on sen tunnus, sen loki, **Mitätöi** ja **Salli varmuuskopioiden käynnistys**, ja samat rajat kuin avaimella. Mitätöinti tulee voimaan heti. Kun sama asiakas kirjautuu uudelleen, uusi lupa korvaa vanhan, ja lupa, jota kukaan ei ole käyttänyt 30 päivään, vanhenee. Samaan aikaan voi olla kirjautuneena enintään 10 asiakasta 10 avaimen lisäksi.

Suostumussivu hyväksyy pyynnön vain rekisteröidyltä asiakkaalta, joka nimeää täsmälleen yhden rekisteröimistään paluuosoitteista: https, tai loopback-osoite millä tahansa portilla omalla tietokoneellasi olevalle asiakkaalle. Vain valtuutuskoodivirta PKCE:llä (S256) hyväksytään, ja vastauksesi on sidottu istuntoosi, joten mikään muu sivusto ei voi lähettää sitä puolestasi. Käyttötunnukset ovat voimassa tunnin. Päivitystunnus vaihdetaan joka käytöllä, ja jos sellainen ilmestyy sen jälkeen uudelleen, BombVault peruu luvan, koska jollakulla muulla on siitä kopio. Asiakas, joka toistaa viimeisimmän päivityksensä 30 sekunnin sisällä, koska vastaus ei koskaan tavoittanut sitä, saa sen sijaan uudet tunnukset. BombVault ei hae asiakkaiden metatietoja internetistä, joten asiakkaat rekisteröityvät dynaamisella asiakasrekisteröinnillä.

### Muut asiakasohjelmat {#other-clients}

Mikä tahansa Streamable HTTP:tä puhuva asiakasohjelma käy:

- URL: verkkokäyttöliittymän osoite ja perään `/mcp`, esimerkiksi `https://192.168.1.10:3443/mcp`.
- Avain otsakkeessa `Authorization: Bearer <key>` tai `X-API-Key: <key>`. Jos molemmat lähetetään, niissä on oltava sama avain.
- `POST`, jossa `Content-Type: application/json` ja `Accept: application/json, text/event-stream`.
- Yksi JSON-RPC-viesti pyyntöä kohden; eräpyynnöt (batch) hylätään.
- Protokollaversiot 2026-07-28, 2025-11-25, 2025-06-18 ja 2025-03-26.

## TLS ja varmenteet {#tls}

BombVault tarjoaa HTTPS:n itse myöntämällään varmenteella, ja aluksi se varmenne nimeää vain `localhost`, `127.0.0.1` ja `::1`. Claude Code ja `mcp-remote` hylkäävät sen lähiverkon osoitteessa. Kiertotiet siinä järjestyksessä, joka sopii useimpiin Unraid-asennuksiin:

1. **Lisää osoite MCP-kortissa.** Kun kortti avataan HTTPS:llä osoitteessa, jota varmenne ei nimeä, kortti kertoo sen ja tarjoaa painiketta **Lisää tämä osoite varmenteeseen**. BombVault myöntää silloin varmenteensa uudelleen niin, että osoite on mukana (selaimesi varoittaa vielä kerran, kuten ensimmäisellä kerralla). Napsauta sitten **Lataa varmenne**; katkelmat asettavat `NODE_EXTRA_CA_CERTS`:n ladattuun tiedostoon, joten asiakasohjelma luottaa juuri siihen varmenteeseen. Se tarkoittaa myös, että jokainen aiemmin ladatulla tiedostolla määritetty asiakasohjelma lakkaa yhdistämästä heti, kun varmenne myönnetään uudelleen, tällä tietokoneella ja kaikilla muilla, kunnes se saa uuden tiedoston.
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

- Ilman aktiivista avainta ja OAuth-kirjautumisen ollessa pois päältä `/mcp` vastaa `404`.
- OAuth-kirjautumista tarjotaan vain, kun kirjautumissalasana on asetettu. Tunnukset, koodit ja asiakassalaisuudet tallennetaan vain sormenjälkinä, ja tunnus kelpaa vain osoitteeseen, jolle se on myönnetty.
- Asiakas voi rekisteröityä samasta osoitteesta enintään 10 kertaa tunnissa, ja BombVault säilyttää enintään 100 rekisteröityä asiakasta, joilla kukaan ei ole kirjautunut, kutakin vuorokauden. Väärät koodit ja päivitystunnukset lasketaan samaan estoon kuin väärät avaimet.
- Luvat käyttäytyvät kuten avaimet, kun asetusten varmuuskopio palautetaan tai `APP_KEY` vaihtuu: palautuksen jälkeen jokaisen asiakkaan on kirjauduttava uudelleen.
- Mikään osoite ei ole poikkeus. Pyynnöt osoitteesta `localhost`, Unraid-isännältä, käänteiseltä välityspalvelimelta tai `tailscale serve`:ltä tarvitsevat avaimen kuten kaikki muutkin, myös silloin kun verkkokäyttöliittymällä ei ole kirjautumissalasanaa.
- Avaimet tallennetaan vain sormenjälkinä, näytetään kerran, ja ne voi nimetä uudelleen, vaihtaa ja peruuttaa. Enintään 10 aktiivista avainta, kullakin oma kytkin **Salli varmuuskopioiden käynnistys**.
- Jokainen luonti, vaihto, oikeuksien muutos ja peruutus lähettää ilmoituksen ilmoituskanaviesi kautta osoitteen kera, josta se tuli, ellei ilmoituksia ole kytketty pois.
- 5 väärää avainta minuutissa osoitetta kohden, sen jälkeen `429`. 120 pyyntöä minuutissa ja 12 käynnistettyä varmuuskopiota tunnissa avainta kohden, lisäksi yllä kuvattu odotusaika ja säilytyssuoja.
- Toisesta originista tulevan selainsivun pyynnöt hylätään.
- Niin kauan kuin kirjautumissalasanaa ei ole asetettu, avaimia ei voi luoda julkiselta näyttävästä isäntänimestä.
- Jokainen avustajan käynnistämä varmuuskopio ja siitä seuraavat prune- ja off-site-ajot merkitään "MCP:n kautta" avaimen nimen kera toimintalokiin, virhepaneeliin ja varmuuskopion ilmoitukseen.
- Jokainen työkalukutsu kirjoitetaan kontin lokiin avaimen tunnisteen ja sen neljän viimeisen merkin kera (ei koskaan nimeä) ja lasketaan `/metrics`-sivulla (`bombvault_mcp_requests_total`, `bombvault_mcp_tool_calls_total`, `bombvault_mcp_active_keys`).
- Asetusten varmuuskopion palauttaminen peruuttaa kaikki avaimet, koska palautettu tietokanta voi sisältää avaimia, jotka peruutit sen tallentamisen jälkeen. Luo uudet avaimet sen jälkeen.
- Avain lakkaa toimimasta, kun `APP_KEY` muuttuu (uudelleenasennus tai palautus toiseen konttiin). Kortti huomaa sen ja merkitsee avaimen, ja **Vaihda avain** antaa sille taas kelvollisen salaisuuden.
- Kohtele avainta kuin salasanaa. Asiakasohjelma, joka ei osaa lukea avainta ympäristömuuttujasta, kyselystä tai avaintiedostosta, pitää sen selväkielisenä määrityksissään tai asetuksissaan, ja sen ikkuna kertoo sen. Käytä mieluummin vain lukevaa avainta tietokoneella, johon luotat vähemmän.

## Mitä koneelta lähtee {#privacy}

Kaikki, mitä avustaja lukee, menee sen takana olevalle tekoälypalvelun tarjoajalle: kohteiden nimet, ajastukset, ajohistoria virheilmoituksineen, palautuspisteiden tunnisteet ja ajat, tietokantamoottorien nimet ja dumppien koot, käynnissä oleva toiminta, tallennustilan luvut, kattavuus ja tila. BombVault poistaa isännän polut, repositorioiden sijainnit, isäntänimet, tunnukset, hook-komennot ja avaimet ennen kuin mitään lähtee.

## Vianmääritys {#troubleshooting}

| Mitä näet | Mitä se tarkoittaa |
|---|---|
| `404` | Aktiivista avainta ei ole ja OAuth-kirjautuminen on pois päältä, tai polku on väärä, kuten `/api/mcp`. Päätepiste on `/mcp`. |
| `401` | Avain puuttuu, on kirjoitettu väärin, peruutettu tai vaihdettu. Ehkä välityspalvelin pudottaa otsakkeen `Authorization` (kokeile `X-API-Key`). Jos kortti merkitsee avaimen enää kelpaamattomaksi, `APP_KEY` on muuttunut: vaihda avain. |
| `403` | Pyyntö tuli selainsivulta, jolla on eri origin. Käytä työpöytä- tai komentoriviasiakasta. |
| `405` GET-pyynnöllä | Normaalia. Päätepiste ottaa vastaan vain `POST`-pyyntöjä. |
| `400` "Accept must contain both 'application/json' and 'text/event-stream'" | Asiakasohjelma on liian vanha Streamable HTTP:lle. Päivitä se. |
| `400` "batch requests are not accepted" | Asiakasohjelma lähettää JSON-RPC-eriä. Lähetä yksi viesti pyyntöä kohden. |
| `429` | Liian monta väärää avainta tästä osoitteesta, tai yli 120 pyyntöä minuutissa yhdellä avaimella. Odota minuutti ja tarkista, onko avustaja jumissa silmukassa. |
| Virheet, joissa lukee "certificate", "self-signed" tai "unable to verify" | Asiakasohjelma ei luota BombVaultin varmenteeseen. Katso [TLS ja varmenteet](#tls). |
| `busy` | Toinen varmuuskopio tai ylläpitotehtävä varaa toimialueen. Yritä uudelleen, kun se on valmis. |
| `cooldown` | Tämä kohde, tämä toimialue tai Backup Everything käynnistettiin MCP:n kautta alle 15 minuuttia sitten. |
| `retention_guard` | Vielä yksi MCP-varmuuskopio jättäisi "säilytä viimeiset N" -ikkunaan vain MCP:n tekemiä palautuspisteitä, tai kohde on jo saanut 4 varmuuskopiota MCP:n kautta viimeisten 24 tunnin aikana, epäonnistuneet ja perutut mukaan lukien. Ensimmäisessä tapauksessa seuraava ajastettu varmuuskopio tekee tilaa, toisessa kohde vapautuu 24 tuntia vanhimman niistä jälkeen. Verkkokäyttöliittymästä voit käynnistää sen milloin tahansa. |
| `rate_limited` | Avain on käyttänyt tämän tunnin 12 käynnistystään. |
| `not_permitted` käynnistyksessä | Avain saa vain lukea. Kytke **Salli varmuuskopioiden käynnistys** päälle kortissa; uutta yhteyttä ei tarvita. Peruutuksessa se tarkoittaa, että tämä avain ei käynnistänyt ajoa. |
| `domain_off` | Se varmuuskopiolaji on kytketty pois asetuksista. |
| `not_found` | BombVault ei suojaa sitä kohdetta. Lisää se ensin verkkokäyttöliittymässä; MCP ei koskaan luo asetuksia. |
| Asiakas ei löydä valtuutuspalvelinta | OAuth-kirjautuminen on pois päältä, kirjautumissalasanaa ei ole asetettu, tai välityspalvelin ei päästä polkua `/.well-known/` läpi BombVaultiin. |
| Suostumussivu kertoo, ettei paluuosoitetta ole rekisteröity | Asiakas lähetti paluuosoitteen, jota se ei ole rekisteröinyt. Poista liitin asiakkaasta ja lisää se uudelleen. |
| Kirjautunut asiakas saa vastauksen `401` | Sen lupa on peruttu, se on vanhentunut 30 käyttämättömän päivän jälkeen, tai julkinen osoite on muuttunut. Asiakas kirjautuu uudelleen. |

Älä aseta kontille ympäristömuuttujaa `MCPGODEBUG`. Se muuttaa MCP-kirjaston toimintaa, ja virheellinen arvo pysäyttää BombVaultin käynnistyksessä ennen kuin se kirjoittaa yhtäkään lokiriviä.
