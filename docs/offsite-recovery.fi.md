# Etäsijainti ja palautus

Paikalliset varmuuskopiot suojaavat sinua kadonneelta kontilta tai huonolta päivitykseltä. Etäreplikointi ja testattu palautuspaketti suojaavat sinua koko laatikon menetykseltä, kiristysohjelmalta tai tulipalolta. Tämä sivu käsittelee etäsijaintiin replikoinnin, kyseisen kopion peukalointisuojauksen, palautuskyvyn todistamisen ja toipumisen silloin kun BombVault itse on kadonnut.

## Etäreplikointi

Säilytä nopea paikallinen varmuuskopio ja lisää yksi tai useampi etäreplika. Aseta repo per toimialue **Asetukset, Etä** -välilehdellä. BombVault replikoi uudet tilannevedokset sinne `restic copy` -komennolla parhaan yrityksen periaatteella, joten etäsijainnin nikottelu ei koskaan kaada paikallista varmuuskopiota. Paikallinen repo pysyy ensisijaisena.

- **Useita etäkohteita per toimialue.** Jokainen toimialue (kontit, virtuaalikoneet, flash, config ja tiedostojoukot) voi replikoitua useaan etäkohteeseen kerralla, ei vain yhteen, joten voit pitää esimerkiksi rest-serverin ystävän laatikossa ja S3-ämpärin rinnakkain. Lisää lisäkohteita kohtaan Asetukset, Etä, kukin omalla repositoriollaan, S3-tallennusluokallaan, append-only-lipullaan, säilytyksellään ja kasvubudjetillaan. Olemassa oleva yksittäinen etämääritys siirretään koskemattomana ensimmäiseksi kohteeksi, ja toimialueen jokainen kohde replikoituu kyseisen toimialueen etäaikataulun mukaan.
- **Toimialuekohtainen etäaikataulu** (muokattuna jokaisen muun aikataulun rinnalla kohdassa Asetukset, Aikataulut): jätä se tyhjäksi replikoidaksesi jokaisen paikallisen varmuuskopion jälkeen, tai aseta tahti (esimerkiksi `weekly Sun 03:00`) lähettääksesi etäsijaintiin harvemmin kuin varmuuskopioit paikallisesti. **Replikoi nyt** -painike kattaa pyydettäessä tehtävät ajot.
- **Etäsäilytys** asuu kohdassa Asetukset, Etä, jotta voit säilyttää etäkopioita pidempään arkistona. Jätä käytäntö pelkiksi nolliksi, jotta etätilannevedoksia ei koskaan karsita automaattisesti.
- **Kaistanleveyden rajat** (Asetukset, Etä) rajoittavat resticin lähetys-/latausnopeutta, jotta replikointi ei tuki WAN-yhteyttäsi.
- **Replikointiosoitin** näyttää, mikä toimialue replikoituu sen ollessa käynnissä (sen sivulla ja Kojelaudalla). Se on aktiivisuusosoitin, ei prosenttipalkki, koska `restic copy` ei paljasta koneluettavaa edistymistä.

!!! note "Palautus mistä tahansa paikasta"
    Jokainen kontti, VM, tiedostojoukko, flash ja sovelluksen asetukset listaavat varmuuskopionsa yhtenä aikajanana kaikkien paikkojen yli, joissa varmuuskopio sijaitsee. B2:een kopioitu varmuuskopio näkyy vain kerran, merkittynä jokaisella paikalla joka sen sisältää. Palautus ottaa ensimmäisen paikan, johon se pääsee, aloittaen arkistosta johon kohde on kirjoitettu, ja voit valita toisen paikan riviä kohden. Etäpaikat luetaan vasta, kun avaat ne. Poistaminen yhdessä paikassa tarkistaa ensin muut ja kertoo, oliko se viimeinen kopio.

## Sijoittelu per kohde {#placement}

Jokaisella kontti-, VM- ja tiedostojoukkokortilla on **Sijoittelu**-rivi kolmella segmentillä:

- **Paikallinen** kirjoittaa kohteen arkistoon, joka näkyy kohdassa **Tallennuspaikka**, eikä kopioi sitä minnekään. Käytä tätä datalle, jolla on jo toinen kopio, esimerkiksi jaolle joka asuu NAS-laitteella.
- **Paikallinen + etä** kirjoittaa sen myös sinne ja kopioi sen kohdassa **Kopiointikohteet** rastitettuihin kohteisiin, yksi chip per toimialueen etäkohde. Poista chipin rasti, niin kyseinen kohde ei saa mitään uutta tältä kohteelta.
- **Vain etä** kirjoittaa kohteen suoraan kohdassa **Lähetyskohde** näkyvään paikkaan: suoraan arkistoon etäkohteen vieressä, tai etäarkistoon jonka olet perustanut kohdassa Asetukset, Tallennus, Arkistot.

Sijainti on kiinteä kohteen ensimmäisestä varmuuskopiosta lähtien, koska BombVault ei koskaan siirrä varmuuskopioita arkistojen välillä. Kopiot voivat muuttua milloin tahansa. Kohde, joka ei enää saa uutta kohdetta, säilyttää olemassa olevat kopionsa ja typistää ne omaan säilytykseensä toimialueen seuraavalla etäajolla; **Poista kohteessa B2** kortilla poistaa ne heti. Kun osa noista kopioista ei ole olemassa missään muualla, vahvistus listaa ne päivämäärän mukaan ja pyytää kohteen nimeä. Vain lisäystä sallivista kohteista ei voi poistaa.

Rivin alla kortti kertoo, minne kohde menee ja mitä siellä oikeasti on: kuinka monessa paikassa se on, milloin kukin kohde nähtiin viimeksi, ja täyttyykö 3-2-1. Paikka on alkuperäisen datan sisältävä palvelin, jokainen etäkohde ja jokainen **Rakennuksen ulkopuolella** merkitty arkisto. BombVault tarkistaa kopiot ja paikat; se ei tarkista 3-2-1:n "kaksi mediaa" -osaa.

### Sijoittelun oletukset

Asetukset, Tallennus, **Sijoittelun oletukset** sisältää yhden rivin per toimialue, samoilla kolmella segmentillä. Kopiot pätevät heti jokaiseen kohteeseen, jolla ei ole omaa valintaa, sekä Compose-pinojen projektikansioihin. Sijainti pätee uuteen kohteeseen sen ensimmäisessä varmuuskopiossa; sen muuttaminen ei siirrä yhtään varmuuskopiota. Ennen tallennusta rivi nimeää jokaisen kohteen, joka saa tai menettää kohteita, ja kuinka monta tilannevedosta se tarkoittaa. **Käytä kohteisiin ilman varmuuskopioita** palauttaa oletukseen jokaisen kohteen, jolla ei vielä ole varmuuskopiota.

Uusi etäkohde saa jokaisen kohteen, jota ei ole asetettu Paikalliseksi. Sen lisäävä valintaikkuna kertoo kuinka monta kohdetta on kyseessä ja, jos tiedossa, kuinka paljon historiaa se on, ja tarjoutuu jättämään pois kohteet jotka on jo jätetty pois muista kohteista.

### Suorat arkistot

Kohteen suoran arkiston valitseminen kohdassa Vain etä avaa valintaikkunan, jossa on ehdotettu sijainti kohteen vieressä, esimerkiksi `b2:bucket:containers-direct`, ja yhteystesti joka ei luo mitään. **Luo ja käytä** luo arkiston ja osoittaa kohteen siihen. Suora arkisto ottaa kohteen avaimen, tallennusluokan, rajat, append-only-asetuksen ja säilytyksen, ja muuttuu niiden mukana; Arkistot-kortti näyttää sen vain luku -tilassa. Kun kohteen uusi avain ei avaa sitä, suora arkisto säilyttää avaimen joka sillä on, ja tallennus kertoo sen. Sen tilannevedokset kantavat tunnistetta `bv:direct`, ja jokainen muu säilytysajo säästää ne, joten suora arkisto joka on menettänyt yhteytensä kohteeseensa ei koskaan vanhene paikallisten sääntöjen mukaan.

### Rakennuksen ulkopuolella

Nimetty arkisto voidaan merkitä **Rakennuksen ulkopuolella** Arkistot-kortilla. Etäarkistot alkavat merkittyinä; kytke se pois rest-serveriltä samassa rakennuksessa. Merkintä laskee vain paikat ja 3-2-1:n korteilla. Se ei muuta yhtään kopiota.

### Uudelleenrakennuksen jälkeen

Kopiointivalinnat asuvat BombVaultin omissa asetuksissa. Uudelleenrakennuksen jälkeen Tunnista-toiminnolla ilman palautettua `/config`-kansiota ne ovat poissa, ja kaiken kopiointi lähettäisi B2:een uudelleen kohteet jotka olit jättänyt pois. Siksi jokaisen uudelleenrakennetun toimialueen etäreplikointi keskeytyy. Kojelauta näyttää sen keltaisena, ja Sijoittelun oletukset tarjoaa **Vahvista oletus** -toiminnon, jossa on esikatselu siitä mitä seuraava ajo kopioi ja nimet varmuuskopioissa joilla ei ole merkintää, jotka voit jättää pois siellä. Vain vahvistus lopettaa keskeytyksen; asetustiedoston tuonti tuo takaisin säännöt ja oletukset muttei lopeta sitä.

## Etäsijaintiset ensisijaiset arkistot {#remote-primary-repositories}

Alueen varmuuskopiopolku (Asetukset, Polut ja tallennus) ei rajoitu paikalliseen kansioon: osoita se suoraan restic-etäarkistoon (`s3:...`, `rest:http://isanta:8000/arkisto`, `b2:...`, `sftp:kayttaja@isanta:/arkisto`, `rclone:etä:bucket/polku`), niin BombVault varmuuskopioi suoraan sinne, ilman erillistä paikallista kopiota ja ilman replikointivaihetta. Tämä on aidosti eri muoto kuin yllä kuvattu off-site-replikointi: siellä paikallinen arkisto on ensisijainen ja off-site-arkisto sen paras mahdollinen arkistokopio; täällä etäarkisto **on** ensisijainen ja ainoa kopio, ellet määritä kyseiselle alueelle lisäksi off-site-replikointia (tai toista etäarkistoa).

Kussakin viidestä polkukentästä (Kontit, Virtuaalikoneet, Flash, Kokoonpano, Tiedostot) on aivan vieressä kytkin **Paikallinen / Etä**:

- **Paikallinen** näyttää tutun kansioselaimen.
- **Etä** vaihtaa sen tavalliseen URL-kenttään ja lisää painikkeen, joka avaa saman yhteystestin ja tunnusten valintaikkunan kuin off-site-kohteet käyttävät, mutta tälle ensisijaiselle arkistolle säädettynä. Sieltä saat:
    - **Yhteystestin** todellista polkua vasten, ennen kuin luotat siihen.
    - **Kaistarajat** (lähetys ja lataus), jottei ajastettu varmuuskopiointi etäensisijaiseen arkistoon täytä WAN-yhteyttäsi: samat restic-valitsimet `--limit-upload` ja `--limit-download`, joita off-site-replikointi käyttää, nyt itse varmuuskopiointiin sovellettuina.
    - **Append-only-suojan (muuttumattomuus)**, varmennettuna samalla aktiivisella peukalointitestillä (aito DELETE-koetus vastapuolta vastaan), jonka off-site-kohteet saavat. Päällä ollessaan BombVault kieltäytyy karsimasta arkistoa itse: koska takana ei ole erillistä paikallista kopiota, tämän koneen tunnukset eivät saa kyetä poistamaan varmuuskopion ainoaa kappaletta.
    - **Kasvubudjetin hälytyksen**, joka johdetaan samasta arkiston koon kehityksestä, jota Tallennus-kortti jo seuraa.

Mikään tästä ei ole pakollista: käsin kirjoitettu etäpolku ilman tallennettuja turva-asetuksia varmuuskopioi täsmälleen kuten ennenkin (rajaton kaista, karsittavissa, ei budjettihälytystä). Turvavalintaikkuna on siltä varalta, että haluat samat suojaukset kuin off-site-kopio saa, ilman että sinun tarvitsee luoda erillistä off-site-kohdetta vain sitä varten.

!!! note "Pilvi- ja REST-tunnukset ovat yhteiset"
    Etäensisijainen arkisto tunnistautuu samoilla S3-/REST-tunnuksilla, jotka on määritetty kohdassa Asetukset, Off-site, Pilvitunnukset. Ensisijaisille arkistoille ei ole erillistä tunnusvarastoa.

## Muuttumaton (append-only) etäsijainti

Merkitse etärepo append-only-tilaan, jotta kiristysohjelma, tai vaarantunut isäntä, ei voi poistaa tai uudelleenkirjoittaa varmuuskopioitasi. Vastapuoli (`restic/rest-server`, joka pyörii `--append-only`-tilassa) **valvoo** sitä. BombVault vain aina **todentaa** sen eikä koskaan näytä vihreää pelkän kokoonpanoväitteen perusteella.

**Ohjattu etäsijainnin määritys** -toiminto vie sinut taustajärjestelmän valinnasta (rest-server / rclone / S3) valmiin liitettävän rest-server-käyttöönottokatkelman, yhteystestin, muuttumattomuuskytkimen (joka ajaa peukalointitestin heti) ja säilytysstrategian läpi, joten append-only-etäsijainti on tavoitettavissa ilman määritysten käsin muokkaamista.

!!! note "Onnistunut poisto polussa `/locks/` on odotettua"
    Append-only ei tarkoita, ettei mitään voisi enää poistaa. resticin on otettava ja vapautettava omat lukkonsa, joten `/locks/` pysyy tarkoituksella kirjoitettavana ja poistettavana. Tilannevedoksia ja niiden takana olevaa dataa, eli juuri sitä mihin kiristysohjelma tähtäisi, ei voi poistaa. Jos koettelet vastapuolta itse, onnistunut poisto polussa `/locks/` on oikea toiminta eikä aukko suojauksessa.

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

![Vastaanottava puoli, vain luku -tilassa valvottuna, ja eheystarkistus ajetaan tällä koneella.](assets/screenshots/receiver.png)

*Vastaanottava puoli, vain luku -tilassa valvottuna, ja eheystarkistus ajetaan tällä koneella.*

Kaikki yllä oleva on *lähettävä* puoli. Laatikossa, joka **vastaanottaa** muuttumattomia etäkopioita toisesta BombVaultista, Vastaanottajan kojelauta antaa sinulle riippumattoman, vain luku -tilaisen valvonnan noista repositorioista vastaanottavalla laitteistolla, jotta hiljainen epäonnistuminen vastapäässä ei jää huomaamatta.

Kytke **Vastaanottaja**-kytkin päälle Asetuksissa paljastaaksesi **Vastaanottaja**-välilehden. Se on oletuksena pois päältä; ota se käyttöön vain laatikossa, joka tosiasiassa vastaanottaa muuttumattomia etävarmuuskopioita. Rekisteröi sitten vastaanotettu repositorio (vain luku, avattuna lähettävän instanssin avaimella) saadaksesi:

- **Lähteittäin ryhmitellyn tilannevedosinventaarion**, jotta näet tarkalleen mitkä kontit, virtuaalikoneet ja tiedostojoukot ovat saapuneet.
- **Viimeksi vastaanotettu** per lähde, jotta tiedät kuinka tuore kukin on.
- **Riippumattoman `restic check`** -ajon vastaanottavalla laitteistolla, jotta eheys todennetaan siellä missä data tosiasiassa sijaitsee, ei vain lähettäjällä.
- **Kuolleen miehen kytkimen:** hälytys, kun lähde lakkaa lähettämästä asettamasi ikkunan sisällä.
- **Eheyshälytykset:** hälytys, kun tarkistus vastaanottavalla puolella epäonnistuu.

Vastaanottaja on ehdottoman vain luku -tilainen. Se ei koskaan kirjoita vastaanotettuun repositorioon, joten se ei voi koskaan rikkoa append-only-takuuta, johon lähettäjä nojaa.

## Läpikäyty esimerkki: kaksi Unraid-konetta, päästä päähän

Yllä kuvataan osat. Tässä on yksi kokonainen kokoonpano oikeilla arvoilla, sillä osat on helpompi koota, kun ne on kerran nähnyt koottuina.

Kaksi konetta: **TOWER** ajaa kontit ja lähettää varmuuskopiot, **VAULT** ottaa ne vastaan ja pakottaa muuttumattomuuden. Korvaa omilla nimillä, osoitteilla ja jakopoluilla.

**1. Pystytä append-only-palvelin VAULTiin.** Mene TOWERin BombVaultissa kohtaan *Asetukset → Etäkohde → ohjattu asennus*, valitse **rest-server** ja luo resepti. Kopioi välilehti **Unraid-malli (XML)**, tallenna se VAULTiin nimellä `/boot/config/plugins/dockerMan/templates-user/my-rest-server.xml`, valitse sitten *Docker → Add Container* ja mallilistasta **rest-server**. Kirjoita näytetty `htpasswd`-rivi VAULTissa tiedostoon `/mnt/user/appdata/rest-server/.htpasswd` ennen käynnistystä. Kertakäyttösalasana näytetään kerran eikä sitä tallenneta, joten kopioi se nyt. Se rivi sisältää saman salasanan, jo bcrypt-tiivistettynä: selkoteksti menee TOWERin REST-tunnuksiin, tiivistetty rivi VAULTin `.htpasswd`-tiedostoon. Sinun ei tarvitse tiivistää mitään itse.

    Jätä `--append-only` OPTIONS-kenttään. Se on koko juju: ilman sitä VAULT on taas tavallinen jako.

**2. Osoita etävarasto sinne TOWERissa.** Varaston osoite noudattaa reseptin tulostamaa muotoa:

    rest:http://VAULT:8000/bombvault-containers/containers

Polun ensimmäinen osa on htpasswd-käyttäjä, toinen on varasto. Syötä luotu käyttäjä ja salasana kohteen REST-tunnuksiksi ja aja **yhteystesti**.

**3. Kytke TOWERissa ”Muuttumaton” päälle.** Peukalointitesti ajetaan heti ja sen on sanottava *suojattu*. Mitä vastaukset tarkoittavat:

| Tulos | Mitä tapahtui |
| --- | --- |
| **suojattu** | VAULT kieltäytyi poistosta. Tämä on ainoa hyväksytty tila. |
| **EI suojattu** | VAULT hyväksyi poiston. `--append-only` puuttuu tai se on poistettu. |
| **ei ratkaiseva** | Ei kumpikaan. Yleensä osoite ei ole se, jota restic itse käyttää, tai tunnukset ovat muuttuneet. Mitään ei kirjata eikä hälytystä laukaista. |

**4. Katso VAULTissa, mitä saapuu.** Kytke päälle *Asetukset → Vastaanotin*, avaa **Vastaanotin**-välilehti ja rekisteröi varasto vain luku -tilassa.

!!! warning "Sijainti on polku kontin **sisällä**, kirjoitettuna suhteessa isäntäliitokseen"
    Syötä `user/appdata/rest-server/bombvault-containers/containers`, **ei** `/mnt/user/appdata/…`. BombVault ajetaan kontissa, jossa isännän `/mnt` on liitetty muualle; isännän absoluuttista polkua ei siellä ole. Jos liität sellaisen, BombVault kertoo nyt käytettävän suhteellisen polun.

    **Lähettävä APP_KEY** on TOWERin avain, ei VAULTin. Löydät sen TOWERista kohdasta *Asetukset → Järjestelmä*.

**5. Tee siitä molemminpuolinen, jos haluat.** Toista samat viisi vaihetta toiseen suuntaan: rest-server TOWERissa vastaanottamassa VAULTin kopiota. Silloin kumpikin kone pakottaa muuttumattomuuden toiselle, eikä kumpikaan voi poistaa toisen varmuuskopioita.

## Ohjattu palautus

Erillinen **Palautus**-välilehti opastaa tuoreen tai uudelleenrakennetun asennuksen läpi katastrofitilanteen, yhdessä paikassa:

1. **Palauttaa BombVaultin omat asetukset ensin**, jotta varmuuskopiopolut, etäkohteet ja tunnukset, joita muu kulku tarvitsee, tulevat esitäytettyinä (sovellettuna itsensä uudelleenkäynnistyksellä Docker-soketin yli, joten käynnissä olevaa asetustietokantaa ei koskaan ylikirjoiteta avoimen kahvan alla).
2. **Tarkistaa, että BombVault voi lukea varmuuskopiosi** (salausavaimen kompastuskivi heti alkuun).
3. Antaa sinun **osoittaa olemassa olevaan repoosi** (paikallinen tai etä).
4. **Tunnistaa** siihen tallennetut kontit, virtuaalikoneet ja tiedostojoukot.
5. **Palauttaa ne kaikki** (jätettynä pysäytetyiksi, jotta käynnistät ne harkiten), palautuspakettisi yhden napsautuksen päässä.

!!! note "Etäkopiot odottavat uudelleenrakennuksen jälkeen"
    Kun vaihe 4 rakentaa merkinnät uudelleen ilman vanhoja asetuksia, näiden toimialueiden etäreplikointi keskeytyy, kunnes sijoittelun oletus vahvistetaan. Katso [Sijoittelu per kohde](#placement).

!!! tip "Suunniteltu siirto vastaan katastrofi"
    Ohjattu palautus palauttaa BombVaultin omat asetukset varmuuskopiosta. *Suunniteltua* siirtoa uuteen laatikkoon varten voit sen sijaan kantaa kokoonpanosi mukanasi suoraan **Vie ja tuo asetukset** -kortilla (siirrettävä JSON-tiedosto). Katso [Asetukset](configuration.md#portable-settings-export-and-import).

### Palautus toisesta BombVault-repositoriosta

Erillinen kortti **Palautus**-välilehdellä avaa *toisen* BombVault-instanssin repon (kohtaan `/mnt` liitetty jako tai etä-URL) **kyseisen instanssin `APP_KEY`:llä**, kertaluonteisessa, vain luku -tilaisessa istunnossa. Selaa siihen tallennettuja kontteja, virtuaalikoneita ja tiedostojoukkoja, valitse tilannevedos ja palauta se, ja palautetusta objektista tulee normaali paikallinen kontti, VM tai tiedostojoukko. Toiseen repoon ei koskaan kirjoiteta mitään, ja omat varmuuskopioasetuksesi pysyvät koskemattomina (istunto asuu muistissa ja vanhenee itsestään). Kontin siirtäminen palvelimelta A palvelimelle B ei enää tarkoita repoasetustesi uudelleensuuntaamista ja niiden palauttamista jälkeenpäin. Elävä palvelinten välinen federointi on nimenomaisesti soveltamisalan ulkopuolella; tämä on tarkoituksellinen kertaveto.

## Salausavaimen palautuspaketti

Tämä on se pala, joka tekee katastrofista toipumisen mahdolliseksi silloinkin kun käynnissä olevaa BombVaultia ei ole.

Yksi napsautus lataa **pääavaimen**, **johdetun restic-salasanan** ja **tarkat repon sijainnit ja komennot**, joten voit palauttaa suoraan restic-komentorivillä millä tahansa koneella. Kojelaudan muistutus nalkuttaa, kunnes olet tallentanut sen.

!!! danger "Säilytä palautuspaketti palvelimen ulkopuolella"
    Paketti sisältää salaisuuden, joka purkaa varmuuskopiosi salauksen. Pidä se turvallisessa paikassa erillään palvelimesta (salasananhallinta, tulostettu kopio kassakaapissa). Jos menetät sekä BombVaultin että `APP_KEY`:n ilman palautuspakettia, salattuja varmuuskopioitasi ei voi palauttaa.

### Kun paketti ei ole käsillä

Salasanaa ei ole tallennettu mihinkään, se **lasketaan** `APP_KEY`-avaimesta. Avaimen ja komentotulkin avulla voit siis muodostaa sen itse:

```sh
printf 'bombvault:restic-repo' \
  | openssl dgst -sha256 -mac HMAC -macopt hexkey:$APP_KEY -r \
  | cut -d' ' -f1
```

Kyseessä on HMAC-SHA256 kiinteästä merkkijonosta `bombvault:restic-repo`, avaimena heksadesimaalisen `APP_KEY`-arvon raakatavut, tulostettuna 64 pienenä heksamerkkinä. Sama arvo on paketissa johdettuna restic-salasanana; tämä on sitä päivää varten, jona paketti on jossain muualla kuin sinä.

!!! warning "Vastaanotetussa arkistossa käytä LÄHETTÄVÄN instanssin avainta"
    Arkisto, joka päätyi tänne off-site-replikoinnilla, luotiin sen lähettäneellä koneella **sen omalla** `APP_KEY`-avaimella. Vastaanottavan koneen avaimesta johtaminen tuottaa salasanan, jonka restic hylkää, ja se näyttää täsmälleen rikkinäiseltä arkistolta olematta sitä. Tämä on tavallisin syy siihen, että `restic check` kyselee vastaanotetulla arkistolla salasanaa yhä uudelleen.

Koska palautusmääritykset asuvat kunkin repon **sisällä** (`<repo>/def`, `<repo>/vm-def`), kopioitu repokansio on täysin itsenäinen, joten paketti plus repo on kaikki mitä paljasrautainen palautus tarvitsee.
