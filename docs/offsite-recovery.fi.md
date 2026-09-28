# Etäsijainti ja palautus

Paikalliset varmuuskopiot suojaavat sinua kadonneelta kontilta tai huonolta päivitykseltä. Etäreplikointi ja testattu palautuspaketti suojaavat sinua koko laatikon menetykseltä, kiristysohjelmalta tai tulipalolta. Tämä sivu käsittelee etäsijaintiin replikoinnin, kyseisen kopion peukalointisuojauksen, palautuskyvyn todistamisen ja toipumisen silloin kun BombVault itse on kadonnut.

## Etäreplikointi

Säilytä nopea paikallinen varmuuskopio ja kopioi se yhteen tai useampaan muuhun paikkaan. Paikat, joihin toimialue kopioidaan, valitset **Toimialueet**-kortilla kohdassa **Asetukset, Tallennustila**, yksi chip kutakin paikkaa kohden (katso [Tallennuspaikat](storage-places.md#domains)). BombVault kopioi uudet tilannevedokset sinne `restic copy` -komennolla parhaan yrityksen periaatteella, joten epäonnistunut kopiointi ei koskaan kaada paikallista varmuuskopiota. Paikan, johon toimialue tallennetaan, ei tarvitse olla paikallinen; katso [Etäpaikkaan tallennettu toimialue](#remote-primary-repositories).

- **Useita kopiointipaikkoja per toimialue.** Toimialueen voi kopioida useaan paikkaan kerralla, esimerkiksi ystävän luona olevaan rest-serveriin ja B2-bucketiin. Säilytys, tallennusluokka, append-only, rajoitukset ja kasvubudjetti kuuluvat paikalle, joten jokainen kopio noudattaa sen paikan sääntöjä, johon se päätyy.
- **Toimialuekohtainen kopiointiaikataulu** (muokattuna jokaisen muun aikataulun rinnalla kohdassa Asetukset, Aikataulut): jätä se tyhjäksi kopioidaksesi jokaisen paikallisen varmuuskopion jälkeen, tai aseta tahti (esimerkiksi `weekly Sun 03:00`) kopioidaksesi harvemmin kuin varmuuskopioit. Toimialueen rivin **Kopioi nyt** ajaa kopioinnin pyydettäessä.
- **Säilytys paikkakohtaisesti.** Jokaisella paikalla on omat sääntönsä, joten etäpaikka voi säilyttää kopioita pidempään arkistona. Paikka, jonka jokainen sääntö on nolla, ei koskaan karsi mitään.
- **Kaistanleveyden rajat** paikkakohtaisesti rajoittavat resticin lähetys- ja latausnopeutta, jotta kopiointi ei tuki WAN-yhteyttäsi.
- **Replikointiosoitin** näyttää, mikä toimialue kopioituu sen ollessa käynnissä (sen sivulla ja Kojelaudalla). Se on aktiivisuusosoitin, ei prosenttipalkki, koska `restic copy` ei paljasta koneluettavaa edistymistä.

!!! note "Palautus mistä tahansa paikasta"
    Jokainen kontti, VM, tiedostojoukko, flash ja sovelluksen asetukset listaavat varmuuskopionsa yhtenä aikajanana kaikkien paikkojen yli, joissa varmuuskopio sijaitsee. B2:een kopioitu varmuuskopio näkyy vain kerran, merkittynä jokaisella paikalla joka sen sisältää. Palautus ottaa ensimmäisen paikan, johon se pääsee, aloittaen arkistosta johon kohde on kirjoitettu, ja voit valita toisen paikan riviä kohden. Etäpaikat luetaan vasta, kun avaat ne. Poistaminen yhdessä paikassa tarkistaa ensin muut ja kertoo, oliko se viimeinen kopio.

## Sijoittelu per kohde {#placement}

Jokaisella kontti-, VM- ja tiedostojoukkokortilla on **Sijoittelu**-rivi kolmella segmentillä:

- **Paikallinen** kirjoittaa kohteen arkistoon, joka näkyy kohdassa **Tallennuspaikka**, eikä kopioi sitä minnekään. Käytä tätä datalle, jolla on jo toinen kopio, esimerkiksi jaolle joka asuu NAS-laitteella.
- **Paikallinen + etä** kirjoittaa sen myös sinne ja kopioi sen kohdassa **Kopiointikohteet** rastitettuihin kohteisiin, yksi chip per toimialueen etäkohde. Poista chipin rasti, niin kyseinen kohde ei saa mitään uutta tältä kohteelta.
- **Vain etä** kirjoittaa kohteen suoraan kohdassa **Lähetyskohde** valittuun paikkaan, joka voi olla mikä tahansa muu paikka kuin toimialueen kotipaikka. Jos toimialue kopioidaan jo siihen paikkaan, kohde saa suoran arkiston kopioiden viereen; muuten BombVault luo sinne arkiston toimialueelle.

Sijainti on kiinteä kohteen ensimmäisestä varmuuskopiosta lähtien, koska BombVault ei koskaan siirrä varmuuskopioita arkistojen välillä. Kopiot voivat muuttua milloin tahansa. Kohde, joka ei enää saa uutta kohdetta, säilyttää olemassa olevat kopionsa ja typistää ne omaan säilytykseensä toimialueen seuraavalla etäajolla; **Poista kohteessa B2** kortilla poistaa ne heti. Kun osa noista kopioista ei ole olemassa missään muualla, vahvistus listaa ne päivämäärän mukaan ja pyytää kohteen nimeä. Vain lisäystä sallivista kohteista ei voi poistaa.

Rivin alla kortti kertoo, minne kohde menee ja mitä siellä oikeasti on: kuinka monessa paikassa se on, milloin kukin kohde nähtiin viimeksi, ja täyttyykö 3-2-1. Erillisiksi paikoiksi lasketaan alkuperäisen datan sisältävä palvelin ja jokainen tallennuspaikka, joka on toisessa rakennuksessa (katso [Rakennuksen ulkopuolella](#off-the-premises-mark)). BombVault tarkistaa kopiot ja paikat; se ei tarkista 3-2-1:n "kaksi mediaa" -osaa.

### Oletukset toimialueittain

Kohdan Asetukset, Tallennustila **Toimialueet**-kortilla on yksi rivi kutakin toimialuetta kohden. **Kopiointikohteet** pätee heti jokaiseen kohteeseen, jolla ei ole omaa valintaa, sekä Compose-pinojen projektikansioihin. Kun toimialueella on varmuuskopioita, **Tallennuspaikka** pätee uuteen kohteeseen sen ensimmäisessä varmuuskopiossa, eikä sen muuttaminen siirrä yhtään varmuuskopiota. Ennen tallennusta rivi nimeää jokaisen paikan, joka saa tai menettää kohteita, ja kuinka monta tilannevedosta se tarkoittaa, ja kysymyksessä on kytkin **Käytä kohteisiin ilman varmuuskopioita**, joka siirtää uuteen oletukseen myös jokaisen kohteen, jolla ei vielä ole varmuuskopiota. **Poikkeukset** luettelee kohteet, joilla on oma valinta.

Kun rastitat uuden paikan kohdassa **Kopiointikohteet**, se saa jokaisen kohteen, jota ei ole asetettu Paikalliseksi. Vahvistus kertoo, kuinka monta kohdetta on kyseessä ja, jos tiedossa, kuinka paljon historiaa se on.

### Suorat arkistot

Kun valitset kohdassa Vain etä paikan, johon toimialue jo kopioidaan, BombVault kysyy kerran ja luo sitten suoran arkiston kopioiden viereen, esimerkiksi `s3:https://s3.eu-central-003.backblazeb2.com/bucket/container-direct`, ja osoittaa kohteen siihen. Jos kopiointikohteella ei ole paikkaa, valinta avaa valintaikkunan, jossa on ehdotettu osoite ja yhteystesti, joka ei luo mitään, ja **Luo ja käytä** luo arkiston. Suora arkisto ottaa paikan avaimen, tallennusluokan, rajat, append-only-asetuksen ja säilytyksen, ja muuttuu niiden mukana. Kun paikan uusi avain ei avaa sitä, suora arkisto säilyttää avaimen, joka sillä on, ja tallennus kertoo sen. Sen tilannevedokset kantavat tunnistetta `bv:direct`, ja jokainen muu säilytysajo säästää ne, joten suora arkisto, joka on menettänyt yhteytensä paikkaansa, ei koskaan vanhene paikallisten sääntöjen mukaan. Yhteen kansioon rajatun B2-avaimen on katettava paikan osoite eikä vain toimialueen kansio, muuten viereinen kansio jää ulottumattomiin.

### Rakennuksen ulkopuolella {#off-the-premises-mark}

Kopio lasketaan rakennuksen ulkopuoliseksi vain, kun sen paikka on toisessa rakennuksessa. Pilvipaikka lasketaan aina ja kansio tässä Unraidissa ei koskaan; NAS-laitteelle, rest-serverille tai SFTP-palvelimelle vastaa paikan tiedoissa kysymykseen **Missä laite on?** joko **Tässä rakennuksessa** tai **Toisessa rakennuksessa**. Vastaus vaikuttaa vain siihen, miten kortit ja Kojelauta laskevat paikat ja 3-2-1:n. Se ei muuta yhtään kopiota.

### Uudelleenrakennuksen jälkeen

Kopiointivalinnat asuvat BombVaultin omissa asetuksissa. Uudelleenrakennuksen jälkeen Tunnista-toiminnolla ilman palautettua `/config`-kansiota ne ovat poissa, ja kaiken kopiointi lähettäisi B2:een uudelleen kohteet jotka olit jättänyt pois. Siksi jokaisen uudelleenrakennetun toimialueen etäreplikointi keskeytyy. Kojelauta näyttää sen keltaisena, ja toimialueen rivi Toimialueet-kortilla tarjoaa **Vahvista oletus** -toiminnon, jossa on esikatselu siitä mitä seuraava ajo kopioi ja nimet varmuuskopioissa joilla ei ole merkintää, jotka voit jättää pois siellä. Vain vahvistus lopettaa keskeytyksen; asetustiedoston tuonti tuo takaisin säännöt ja oletukset muttei lopeta sitä.

## Etäpaikkaan tallennettu toimialue {#remote-primary-repositories}

Toimialuetta ei tarvitse tallentaa paikallisesti. Niin kauan kuin sen varmuuskopiosijainnissa ei ole varmuuskopioita, valitse Toimialueet-kortilla kohdassa **Tallennuspaikka** etäpaikka, niin toimialue varmuuskopioituu suoraan sinne ilman paikallista kopiota ja ilman kopiointivaihetta. Etäarkisto on silloin ainoa kopio, ellei toimialuetta kopioida myös toiseen paikkaan. Jokaisella etäpaikalla on samat suojaukset:

- **Yhteystesti** ennen kuin mitään kirjoitetaan.
- **Kaistarajat** itse varmuuskopioinnille, samat valitsimet `--limit-upload` ja `--limit-download`, joita kopiointi käyttää.
- **Append-only-suoja**, varmennettuna samalla aktiivisella peukalointitestillä. Kun se on päällä, BombVault ei koskaan karsi arkistoa, koska tämän koneen tunnukset eivät saa kyetä poistamaan varmuuskopion ainoaa kappaletta.
- **Kasvubudjetti**, joka johdetaan samasta koon kehityksestä, jota Tallennustila-kortti seuraa.

Etäpaikkaan tallennettu toimialue on kopioidensa lähde samoin kuin paikallinen; katso [Kopiot paikkojen välillä eri tunnuksilla](storage-places.md#different-credentials).

!!! note "Tunnukset kuuluvat paikalle"
    Etäpaikalla on omat tunnuksensa. Paikka, joka on määritetty jaetuilla pilvitunnuksilla, käyttää niitä, kunnes sen pääsyasetuksia muutetaan paikan tiedoissa.

### SMB ja WebDAV ilman liitosta isäntään {#smb-webdav}

**Lisää paikka** -ikkunan rclone-vaihtoehdossa on lomake Windows- tai Samba-jaolle ja WebDAV-palvelimelle (Nextcloud, ownCloud, SharePoint tai mikä tahansa muu). Täytä lyhyt nimi, isäntä ja jako (SMB) tai URL ja palvelintyyppi (WebDAV), käyttäjä ja salasana, niin BombVault kirjoittaa rclone-osion puolestasi. rclone hämärtää salasanan itse ennen sen tallentamista; jos lisäät kohteen nimellä, joka on jo olemassa, se korvaa kyseisen osion eikä lisää toista.

Uusi etäyhteys näkyy sen jälkeen lomakkeen etäyhteyksien luettelossa, josta valitset sen paikalle. Jako on polun ensimmäinen osa, ei osa nimeä.

Tämä on parempi tie kuin jaon liittäminen Unraidiin: restic neuvoo olemaan pitämättä arkistoa liitetyllä CIFS-jaolla, eikä tässä liitetä mitään. NFS puuttuu lomakkeesta, koska resticillä ja rclonella ei kummallakaan ole NFS-taustajärjestelmää; NFS:ää varten liitä vienti isäntään ja lisää se paikaksi valinnalla **Muu jako**.

## Muuttumaton (append-only) etäsijainti

Merkitse etärepo append-only-tilaan, jotta kiristysohjelma, tai vaarantunut isäntä, ei voi poistaa tai uudelleenkirjoittaa varmuuskopioitasi. Vastapuoli (`restic/rest-server`, joka pyörii `--append-only`-tilassa) **valvoo** sitä. BombVault vain aina **todentaa** sen eikä koskaan näytä vihreää pelkän kokoonpanoväitteen perusteella.

**Lisää paikka** -ikkunassa on valmis liitettävä ohje append-only-tilassa toimivalle rest-serverille, jossa on yksi käyttäjä tälle BombVaultille. Kun rest-server-paikassa on **Append-only** päällä, paikan tietojen **Testaa append-only** ajaa peukalointitestin jokaista paikan toimialuepolkua, päälle kytkettyä kopiota ja arkistoa vastaan ja antaa paikalle yhden vastauksen, joten append-only-etäsijainti on tavoitettavissa ilman määritysten käsin muokkaamista.

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

Paikassa **Testaa append-only** koettelee jokaista siellä olevaa toimialuepolkua, päälle kytkettyä kopiota ja arkistoa niiden omilla tunnuksilla ja yhdistää tulokset yhdeksi vastaukseksi: yksikin arkisto, joka hyväksyy poiston, tekee koko paikasta *poistot sallitaan*.

## DR-harjoitukset

BombVault tarjoaa kaksi tasoa todisteita siitä, että varmuuskopiosi ovat tosiasiassa palautuskelpoisia, eivät vain olemassa.

- **Palautuksen tarkistusharjoitukset (paikallinen).** BombVault ajaa ajoittain `restic check --read-data-subset` (rajattu, ei koskaan levyn täyttävää täyspalautusta) ja näyttää *viimeksi todennettu palautuskelpoiseksi* -merkin per toimialue. Tahti asuu kohdassa Asetukset, Aikataulut; merkki kohdassa Asetukset, Eheys.
- **DR-harjoitukset (etä).** BombVault palauttaa oikean kohteen etärepositoriosta kertakäyttöiseen hiekkalaatikkoon, tarkistaa sen tiedosto tiedostolta ja tavu tavulta, ja siivoaa sitten. Tämä todistaa, että voit toipua etäsijainnista, ei vain että repo vastaa. Vain toisessa rakennuksessa olevia paikkoja harjoitellaan, koska kopio samassa rakennuksessa ei todista mitään rakennuksen menettämisestä. Toimialue, joka kopioidaan useaan niistä, harjoitellaan jokaisella ajastetulla ajolla yhtä niistä vastaan vuorotellen, ja Kojelauta näyttää viimeisimmän harjoituksen paikan.

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

**1. Pystytä append-only-palvelin VAULTiin.** Avaa TOWERin BombVaultissa *Asetukset → Tallennustila*, napsauta **Lisää paikka**, valitse **rest-server** ja napsauta **Näytä ohje**. Kopioi **Unraid-malli**-lohko, tallenna se VAULTiin nimellä `/boot/config/plugins/dockerMan/templates-user/my-rest-server.xml`, valitse sitten *Docker → Add Container* ja mallilistasta **rest-server**. Kirjoita näytetty `htpasswd`-rivi VAULTissa tiedostoon `/mnt/user/appdata/rest-server/.htpasswd` ennen käynnistystä. Salasana näytetään kerran eikä sitä tallenneta; ohje on jo kirjoittanut sen ja käyttäjän TOWERin lomakkeeseen, joten jätä se ikkuna auki. `htpasswd`-rivi sisältää saman salasanan jo bcrypt-tiivistettynä, joten sinun ei tarvitse tiivistää mitään itse.

    Jätä `--append-only` OPTIONS-kenttään. Ilman sitä VAULT on taas pelkkä tavallinen jako.

**2. Lisää paikka TOWERissa.** Syötä VAULTin osoite `http://VAULT:8000` ohjeen täyttämän käyttäjän ja salasanan viereen ja napsauta sitten **Testaa yhteys**. BombVault rakentaa osoitteen niistä:

    rest:http://VAULT:8000/tower

Polun ensimmäinen osa on htpasswd-käyttäjä, tässä `tower`, ja jokainen toimialue saa kansionsa sen alle, esimerkiksi `rest:http://VAULT:8000/tower/container`. Vastaa kysymykseen **Missä laite on?** valitsemalla **Toisessa rakennuksessa**, napsauta **Lisää** ja rastita paikka kohdassa **Kopiointikohteet** niille toimialueille, joiden kopiot kuuluvat sinne.

**3. Kytke TOWERissa Append-only päälle** paikan tietojen kohdassa **Suojaus** ja napsauta sitten **Testaa append-only**. Testi koettelee jokaista paikan toimialuepolkua, kopiota ja arkistoa ja antaa paikalle yhden vastauksen, jonka on oltava *poistot torjutaan*. Mitä vastaukset tarkoittavat:

| Tulos | Mitä tapahtui |
| --- | --- |
| **poistot torjutaan** | VAULT kieltäytyi poistosta. Tämä on ainoa hyväksytty tila. |
| **poistot sallitaan** | VAULT hyväksyi poiston. `--append-only` puuttuu tai se on poistettu. |
| viesti tuloksen sijaan | Testiä ei voitu ajaa. Yleensä osoite ei ole se, jota restic itse käyttää, tai tunnukset ovat muuttuneet. Mitään ei kirjata eikä hälytystä laukaista. |

**4. Katso VAULTissa, mitä saapuu.** Kytke päälle *Asetukset → Vastaanotin*, avaa **Vastaanotin**-välilehti ja rekisteröi varasto vain luku -tilassa.

!!! warning "Sijainti on polku kontin **sisällä**, kirjoitettuna suhteessa isäntäliitokseen"
    Syötä `user/appdata/rest-server/tower/container`, **ei** `/mnt/user/appdata/…`. BombVault ajetaan kontissa, jossa isännän `/mnt` on liitetty muualle; isännän absoluuttista polkua ei siellä ole. Jos liität sellaisen, BombVault kertoo, mitä suhteellista polkua käyttää sen sijaan.

    **Lähettävä APP_KEY** on TOWERin avain, ei VAULTin. Löydät sen TOWERista kohdasta *Asetukset → Järjestelmä*.

**5. Tee siitä molemminpuolinen, jos haluat.** Toista samat viisi vaihetta toiseen suuntaan: rest-server TOWERissa vastaanottamassa VAULTin kopiota. Silloin kumpikin kone pakottaa muuttumattomuuden toiselle, eikä kumpikaan voi poistaa toisen varmuuskopioita.

## Ohjattu palautus

Erillinen **Palautus**-välilehti opastaa tuoreen tai uudelleenrakennetun asennuksen läpi katastrofitilanteen, yhdessä paikassa:

1. **Tarkistaa, että BombVault voi lukea varmuuskopiosi** (salausavaimen kompastuskivi heti alkuun).
2. **Palauttaa BombVaultin omat asetukset**, jotta varmuuskopiopolut, etäkohteet ja tunnukset, joita muu kulku tarvitsee, tulevat esitäytettyinä. Se lukee asetusten varmuuskopion paikasta, jonka Itsevarmuuskopio-rivi nimeää kohdassa **Tallennuspaikka**, tai Itsevarmuuskopion kopiosta kohdassa **Kopiointikohteet**, ja näyttää paikan osoitteineen; jos haluat lukea toisesta paikasta, muuta ensin Itsevarmuuskopio-riviä vaiheessa 3. Palautus sovelletaan itsensä uudelleenkäynnistyksellä Docker-soketin yli, joten käynnissä olevaa asetustietokantaa ei koskaan ylikirjoiteta avoimen kahvan alla.
3. **Liittää olemassa olevat varmuuskopiosi** Toimialueet-kortin rivien kautta: valitse kunkin toimialueen rivillä kohdassa **Tallennuspaikka** paikka, jossa sen varmuuskopiot ovat, ja kohdassa **Kopiointikohteet** paikat, joissa sen kopiot ovat. Paikka, jota mikään rivi ei vielä tarjoa, kuten jako, palvelin tai pilvisäilö, yhdistetään **Lisää paikka** -ikkunalla, samalla kuin kohdassa Asetukset, Tallennustila. **Yhdistä ja esikatsele** tarkistaa sitten, että varmuuskopiot voidaan lukea.
4. **Tunnistaa** siihen tallennetut kontit, virtuaalikoneet, tiedostojoukot ja ZFS-tietojoukot.
5. **Palauttaa kontit ja virtuaalikoneet kerralla** (jätettynä pysäytetyiksi, jotta käynnistät ne harkiten) ja luettelee tiedostojoukot ja ZFS-kohteet, jotka palautat yksi kerrallaan; ZFS-kohteet palaavat pois päältä. Palautuspakettisi on yhden napsautuksen päässä.

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

!!! warning "Uusin tilannevedos ei aina ole se, joka kannattaa palauttaa"
    Restic 0.17:stä lähtien `restic snapshots` näyttää jokaisen tilannevedoksen koon. Tietojen menetyksen jälkeen uusin tilannevedos voi olla tyhjennetty, joten älä palauta tilannevedosta, joka on paljon edellisiä pienempi. Kiristyshaittaohjelman jälkeen se voi olla salattu, tavallisen kokoinen. Jos BombVault on yhä käynnissä, katso ensin sen sivu **Poikkeamat**: se nimeää viimeisen hyvän varmuuskopion. Palautus ei tarvitse BombVaultin poikkeamatietoja, ja säilytyksen tauko vain säilyttää enemmän tilannevedoksia.

### Paketin sinetöinti

Jos olet kytkenyt age-salauksen päälle selkokielisille vienneille (Asetukset), myös palautuspaketti sinetöidään sillä ja latautuu nimellä `bombvault-recovery-kit.md.age`. Se on ASCII-panssaroitu eikä binäärinen, joten se on yhä pelkkää tekstiä: sen voi liittää salasanojen hallintaan tai tulostaa aivan kuten ennenkin, sisältöä ei vain voi lukea ilman avaintasi.

!!! warning "Älä säilytä age-avainta paketin sisällä"
    Sinetöidyn paketin avaamiseen tarvitset **yksityisen age-avaimesi**. Säilytä se paikassa, joka ei riipu paketista itsestään, muuten sinulla on palautettavana kaksi asiaa yhden sijaan. Sinetöinti kannattaa, kun paketti on paikassa, jota et täysin hallitse (jaettu salasanojen hallinta, pilvimuistiinpanot, tuloste toimistossa); omassa kassakaapissasi olevaa pakettia suojaa jo kassakaappi.

    Kun salaus on päällä eikä käyttökelpoista vastaanottajaa ole määritetty, lataus estetään suoraan. BombVault ei koskaan turvaudu antamaan pääavainta selkotekstinä.

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

## Tietokantavedoksen hakeminen takaisin {#database-dumps}

Tietokantavedos on oma palautuspisteensä konttivarastossa, tunnisteella `dbdump:<container>` ja yhdellä tiedostolla, `/dbdump/<container>.sql`. BombVault luetteloi, lataa ja tuo ne kohdassa **Varmuuskopiot**; alla samat vaiheet pelkällä resticillä, sitä päivää varten kun BombVaultia ei ole.

```sh
restic -r <repo> snapshots --tag dbdump:<container>
restic -r <repo> dump --tag dbdump:<container> latest /dbdump/<container>.sql > <container>.sql
```

Kunkin vedoksen tunnisteet `dbversion:` ja `dbname:` kertovat, mistä palvelinversiosta se on ja mitä tietokantoja se sisältää. Täydellinen tiedosto päättyy riviin `-- PostgreSQL database cluster dump complete` tai `-- Dump completed`.

Tuo se konttiin, jossa on sama tai uudempi versio (PostgreSQL) tai sama pääversio (MySQL ja MariaDB) ja joka on käynnistetty kerran tyhjällä datakansiolla, jotta se alustaa itsensä. Isäntä ei tarvitse tietokanta-asiakasta, kontissa on sellainen:

```sh
docker exec -i <container> sh -c 'exec psql -X -U "${POSTGRES_USER:-postgres}" -d postgres' < <container>.sql
docker exec -i <container> sh -c 'exec mariadb -uroot -p"$MARIADB_ROOT_PASSWORD"' < <container>.sql
docker exec -i <container> sh -c 'exec mysql -uroot -p"$MYSQL_ROOT_PASSWORD"' < <container>.sql
```

Yhden tietokannan poimimiseen täydestä vedoksesta MySQL ja MariaDB ottavat asiakasohjelman komennolle `--one-database <name>`. PostgreSQL-vedoksessa on jokaiselle tietokannalle oma osansa, joka alkaa rivillä `\connect <name>`: kopioi se osa omaan tiedostoonsa ja tuo se `-d <name>`-valitsimella sen jälkeen kun olet luonut tietokannan.

!!! warning "Rootina otettu vedos tuo mukanaan palvelimen käyttäjät"
    Rootina otettu täysi MySQL- tai MariaDB-vedos sisältää järjestelmätietokannan `mysql`, joten sen tuonti korvaa uuden palvelimen tilit, root-salasana mukaan lukien, vedoksen tileillä. PostgreSQL:ssä ilmoitus `role ... already exists` kontin itsensä luomasta käyttäjästä on odotettu eikä haittaa.
