# Etäsijainti ja palautus

Paikalliset varmuuskopiot suojaavat sinua kadonneelta kontilta tai huonolta päivitykseltä. Etäreplikointi ja testattu palautuspaketti suojaavat sinua koko laatikon menetykseltä, kiristysohjelmalta tai tulipalolta. Tämä sivu käsittelee etäsijaintiin replikoinnin, kyseisen kopion peukalointisuojauksen, palautuskyvyn todistamisen ja toipumisen silloin kun BombVault itse on kadonnut.

## Etäreplikointi

Säilytä nopea paikallinen varmuuskopio ja lisää yksi tai useampi etäreplika. Aseta repo per toimialue **Asetukset, Etä** -välilehdellä. BombVault replikoi uudet tilannevedokset sinne `restic copy` -komennolla parhaan yrityksen periaatteella, joten etäsijainnin nikottelu ei koskaan kaada paikallista varmuuskopiota. Paikallinen repo pysyy ensisijaisena.

- **Useita etäkohteita per toimialue.** Jokainen toimialue (kontit, virtuaalikoneet, flash, config ja tiedostojoukot) voi replikoitua useaan etäkohteeseen kerralla, ei vain yhteen, joten voit pitää esimerkiksi rest-serverin ystävän laatikossa ja S3-ämpärin rinnakkain. Lisää lisäkohteita kohtaan Asetukset, Etä, kukin omalla repositoriollaan, S3-tallennusluokallaan, append-only-lipullaan, säilytyksellään ja kasvubudjetillaan. Olemassa oleva yksittäinen etämääritys siirretään koskemattomana ensimmäiseksi kohteeksi, ja toimialueen jokainen kohde replikoituu kyseisen toimialueen etäaikataulun mukaan.
- **Toimialuekohtainen etäaikataulu** (muokattuna jokaisen muun aikataulun rinnalla kohdassa Asetukset, Aikataulut): jätä se tyhjäksi replikoidaksesi jokaisen paikallisen varmuuskopion jälkeen, tai aseta tahti (esimerkiksi `weekly Sun 03:00`) lähettääksesi etäsijaintiin harvemmin kuin varmuuskopioit paikallisesti. **Replikoi nyt** -painike kattaa pyydettäessä tehtävät ajot.
- **Etäsäilytys** asuu kohdassa Asetukset, Etä, jotta voit säilyttää etäkopioita pidempään arkistona. Jätä käytäntö pelkiksi nolliksi, jotta etätilannevedoksia ei koskaan karsita automaattisesti.
- **Kaistanleveyden rajat** (Asetukset, Etä) rajoittavat resticin lähetys-/latausnopeutta, jotta replikointi ei tuki WAN-yhteyttäsi.
- **Replikointiosoitin** näyttää, mikä toimialue replikoituu sen ollessa käynnissä (sen sivulla ja Kojelaudalla). Se on aktiivisuusosoitin, ei prosenttipalkki, koska `restic copy` ei paljasta koneluettavaa edistymistä.

!!! note "Palautus suoraan etäsijainnista"
    Jokaisessa varmuuskopioselaimessa on **Paikallinen / Etä** -kytkin, joten jos paikallinen repo katoaa tai vioittuu, voit listata ja palauttaa suoraan etäreplikasta. Poisto on lähdekohtainen: varmuuskopion poistaminen vaikuttaa vain katselemaasi kopioon.

## Muuttumaton (append-only) etäsijainti

Merkitse etärepo append-only-tilaan, jotta kiristysohjelma, tai vaarantunut isäntä, ei voi poistaa tai uudelleenkirjoittaa varmuuskopioitasi. Vastapuoli (`restic/rest-server`, joka pyörii `--append-only`-tilassa) **valvoo** sitä. BombVault vain aina **todentaa** sen eikä koskaan näytä vihreää pelkän kokoonpanoväitteen perusteella.

**Ohjattu etäsijainnin määritys** -toiminto vie sinut taustajärjestelmän valinnasta (rest-server / rclone / S3) valmiin liitettävän rest-server-käyttöönottokatkelman, yhteystestin, muuttumattomuuskytkimen (joka ajaa peukalointitestin heti) ja säilytysstrategian läpi, joten append-only-etäsijainti on tavoitettavissa ilman määritysten käsin muokkaamista.

!!! warning "Muuttumattomia repoja ei koskaan karsita tästä laatikosta"
    Muuttumaton etäsijainti ei tarkoituksella koskaan karsi vanhoja tilannevedoksia. Aseta sille **kasvubudjettihälytys**, jotta saat hälytyksen ennen kuin repon koko karkaa käsistä.

## Peukalointitesti

BombVault todistaa ajoittain append-only-takuun tosiasiallisesti yrittämällä poistoa etärepoa vasten, kohdistettuna olemattomaan objektiin:

- **Torjuttu** tarkoittaa suojattua.
- **Hyväksytty** tarkoittaa ei-suojattua.
- **Epäselvä** tulos (palvelin tavoittamattomissa, todennusvirhe) ei koskaan käännä tallennettua tuomiota.

Todellinen suojatusta suojaamattomaksi -kääntyminen laukaisee yhden hälytyksen.

## DR-harjoitukset

BombVault tarjoaa kaksi tasoa todisteita siitä, että varmuuskopiosi ovat tosiasiassa palautuskelpoisia, eivät vain olemassa.

- **Palautuksen tarkistusharjoitukset (paikallinen).** BombVault ajaa ajoittain `restic check --read-data-subset` (rajattu, ei koskaan levyn täyttävää täyspalautusta) ja näyttää *viimeksi todennettu palautuskelpoiseksi* -merkin per toimialue. Tahti asuu kohdassa Asetukset, Aikataulut; merkki kohdassa Asetukset, Eheys.
- **DR-harjoitukset (etä).** BombVault palauttaa oikean kohteen etärepositoriosta kertakäyttöiseen hiekkalaatikkoon, tarkistaa sen tiedosto tiedostolta ja tavu tavulta, ja siivoaa sitten. Tämä todistaa, että voit toipua etäsijainnista, ei vain että repo vastaa.

**Kiristysohjelmasuojan tuloskortti** Kojelaudalla kokoaa tämän vihreä / keltainen / punainen -asennoksi per toimialue, iällä leimatun tarkistuslistan kera (etäsijainti määritetty, append-only todennettu, replikointi ajan tasalla, palautusharjoitus läpäisty, salaus päällä, karsintastrategia asetettu). Jokainen punainen rivi linkittää syvälle korjaukseen, ja kortti muuttuu vihreäksi vain todennettujen tosiasioiden perusteella.

## Vastaanottajan kojelauta (vastaanottava puoli)

Kaikki yllä oleva on *lähettävä* puoli. Laatikossa, joka **vastaanottaa** muuttumattomia etäkopioita toisesta BombVaultista, Vastaanottajan kojelauta antaa sinulle riippumattoman, vain luku -tilaisen valvonnan noista repositorioista vastaanottavalla laitteistolla, jotta hiljainen epäonnistuminen vastapäässä ei jää huomaamatta.

Kytke **Vastaanottaja**-kytkin päälle Asetuksissa paljastaaksesi **Vastaanottaja**-välilehden. Se on oletuksena pois päältä; ota se käyttöön vain laatikossa, joka tosiasiassa vastaanottaa muuttumattomia etävarmuuskopioita. Rekisteröi sitten vastaanotettu repositorio (vain luku, avattuna lähettävän instanssin avaimella) saadaksesi:

- **Lähteittäin ryhmitellyn tilannevedosinventaarion**, jotta näet tarkalleen mitkä kontit, virtuaalikoneet ja tiedostojoukot ovat saapuneet.
- **Viimeksi vastaanotettu** per lähde, jotta tiedät kuinka tuore kukin on.
- **Riippumattoman `restic check`** -ajon vastaanottavalla laitteistolla, jotta eheys todennetaan siellä missä data tosiasiassa sijaitsee, ei vain lähettäjällä.
- **Kuolleen miehen kytkimen:** hälytys, kun lähde lakkaa lähettämästä asettamasi ikkunan sisällä.
- **Eheyshälytykset:** hälytys, kun tarkistus vastaanottavalla puolella epäonnistuu.

Vastaanottaja on ehdottoman vain luku -tilainen. Se ei koskaan kirjoita vastaanotettuun repositorioon, joten se ei voi koskaan rikkoa append-only-takuuta, johon lähettäjä nojaa.

## Ohjattu palautus

Erillinen **Palautus**-välilehti opastaa tuoreen tai uudelleenrakennetun asennuksen läpi katastrofitilanteen, yhdessä paikassa:

1. **Palauttaa BombVaultin omat asetukset ensin**, jotta varmuuskopiopolut, etäkohteet ja tunnukset, joita muu kulku tarvitsee, tulevat esitäytettyinä (sovellettuna itsensä uudelleenkäynnistyksellä Docker-soketin yli, joten käynnissä olevaa asetustietokantaa ei koskaan ylikirjoiteta avoimen kahvan alla).
2. **Tarkistaa, että BombVault voi lukea varmuuskopiosi** (salausavaimen kompastuskivi heti alkuun).
3. Antaa sinun **osoittaa olemassa olevaan repoosi** (paikallinen tai etä).
4. **Tunnistaa** siihen tallennetut kontit, virtuaalikoneet ja tiedostojoukot.
5. **Palauttaa ne kaikki** (jätettynä pysäytetyiksi, jotta käynnistät ne harkiten), palautuspakettisi yhden napsautuksen päässä.

!!! tip "Suunniteltu siirto vastaan katastrofi"
    Ohjattu palautus palauttaa BombVaultin omat asetukset varmuuskopiosta. *Suunniteltua* siirtoa uuteen laatikkoon varten voit sen sijaan kantaa kokoonpanosi mukanasi suoraan **Vie ja tuo asetukset** -kortilla (siirrettävä JSON-tiedosto). Katso [Asetukset](configuration.md#portable-settings-export-and-import).

### Palautus toisesta BombVault-repositoriosta

Erillinen kortti **Palautus**-välilehdellä avaa *toisen* BombVault-instanssin repon (kohtaan `/mnt` liitetty jako tai etä-URL) **kyseisen instanssin `APP_KEY`:llä**, kertaluonteisessa, vain luku -tilaisessa istunnossa. Selaa siihen tallennettuja kontteja, virtuaalikoneita ja tiedostojoukkoja, valitse tilannevedos ja palauta se, ja palautetusta objektista tulee normaali paikallinen kontti, VM tai tiedostojoukko. Toiseen repoon ei koskaan kirjoiteta mitään, ja omat varmuuskopioasetuksesi pysyvät koskemattomina (istunto asuu muistissa ja vanhenee itsestään). Kontin siirtäminen palvelimelta A palvelimelle B ei enää tarkoita repoasetustesi uudelleensuuntaamista ja niiden palauttamista jälkeenpäin. Elävä palvelinten välinen federointi on nimenomaisesti soveltamisalan ulkopuolella; tämä on tarkoituksellinen kertaveto.

## Salausavaimen palautuspaketti

Tämä on se pala, joka tekee katastrofista toipumisen mahdolliseksi silloinkin kun käynnissä olevaa BombVaultia ei ole.

Yksi napsautus lataa **pääavaimen**, **johdetun restic-salasanan** ja **tarkat repon sijainnit ja komennot**, joten voit palauttaa suoraan restic-komentorivillä millä tahansa koneella. Kojelaudan muistutus nalkuttaa, kunnes olet tallentanut sen.

!!! danger "Säilytä palautuspaketti palvelimen ulkopuolella"
    Paketti sisältää salaisuuden, joka purkaa varmuuskopiosi salauksen. Pidä se turvallisessa paikassa erillään palvelimesta (salasananhallinta, tulostettu kopio kassakaapissa). Jos menetät sekä BombVaultin että `APP_KEY`:n ilman palautuspakettia, salattuja varmuuskopioitasi ei voi palauttaa.

Koska palautusmääritykset asuvat kunkin repon **sisällä** (`<repo>/def`, `<repo>/vm-def`), kopioitu repokansio on täysin itsenäinen, joten paketti plus repo on kaikki mitä paljasrautainen palautus tarvitsee.
