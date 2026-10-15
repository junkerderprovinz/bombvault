# Etäsijainti ja palautus

!!! note "Etäkopiot odottavat uudelleenrakennuksen jälkeen"
    Kun Hätäohjattu toiminto tai **Tunnista varmuuskopiot** rakentaa merkintöjä uudelleen ilman vanhoja asetuksia, näiden osioiden etäkopiot pysähtyvät, kunnes niiden oletus on vahvistettu. Katso [Uudelleenrakennuksen jälkeen](#after-a-rebuild).

Paikalliset varmuuskopiot suojaavat kadonneelta kontilta tai huonolta päivitykseltä. Etäkopiot ja testattu palautuspaketti suojaavat koko laatikon menetykseltä, kiristysohjelmalta tai tulipalolta. Tämä sivu käsittelee kohteita ja etäkopioita, kopion tekemistä peukalointia kestäväksi, palautuskyvyn todistamista, usean palvelimen pariliitosta ja kaiken palauttamista, kun itse BombVaultia ei enää ole.

## Etäkopiot {#off-site-copies}

Pidä nopea paikallinen varmuuskopio ja lisää yksi tai useampi kohde tilojen ulkopuolelle. BombVault kopioi uudet varmuuskopiot sinne `restic copy` -komennolla parhaan yrityksen periaatteella, joten etäkohteen nikottelu ei koskaan kaada paikallista varmuuskopiota. Tässä muodossa paikallinen repo pysyy ensisijaisena ja etärepo on kopio, mutta osion ensisijaisen repon ei tarvitse olla paikallinen lainkaan; katso alla [Etäsijaintiset ensisijaiset arkistot](#remote-primary-repositories), jos haluat varmuuskopioida suoraan S3:een, rest-serveriin tms. kopioimisen sijaan.

  - **Useita kohteita per osio.** Jokainen osio (kontit, VMs, flash, Itsevarmuuskopio, kansiot ja ZFS-tietojoukot) voi kopioida useaan etäkohteeseen kerralla, joten voit pitää rinnakkain esimerkiksi rest-serverin ystävän laatikossa ja S3-bucketin. **Kohteet osioittain** -kortti Kohteet-sivulla kytkee kunkin osiosta kohteeseen vievän rivin päälle tai pois.
  - **Milloin kopioidaan.** Kohteen **Kopioi**-kortti asettaa kohdan **Milloin kopioidaan**: **Jokaisen varmuuskopion jälkeen** tai oma aikataulu, joka kerää varmuuskopiot ja kopioi kiinteinä aikoina. Kohdassa **Asetukset, Aikataulu, Etäkopiot osioittain** **Poikkeus** antaa osiolle oman kopiointiaikataulun. **Replikoi nyt** osion **Etäkopio**-kortissa kohdassa **Asetukset, Kohteet** käynnistää kopioinnin pyynnöstä.
  - **Säilytys** asetetaan kohdekohtaisesti kohdassa **Kuinka kauan säilytetään**, joten etäkopioita voi pitää pidempään arkistona. Osion etäkentän kopio noudattaa korttia **Etäsäilytys** kohdassa **Asetukset, Kohteet**; jätä se kaikki nollille, jotta etävarmuuskopioita ei koskaan karsita automaattisesti.
  - **Kaistanleveyden rajat** (**Etäkaistanleveys** kohdassa **Asetukset, Kohteet** tai kohteen **Kopioi**-kortti) rajoittavat resticin lähetys-/latausnopeutta, jotta kopiot eivät tuki WAN-yhteyttäsi.
  - Kun kopio on käynnissä, yleiskatsauksen tilarivi kertoo **Etäkopiointi käynnissä**. Se on aktiivisuusilmaisin, ei prosenttipalkki, koska `restic copy` ei tarjoa koneluettavaa etenemistietoa.

!!! note "Palautus mistä tahansa paikasta"
    Jokainen merkintä listaa varmuuskopionsa yhtenä aikajanana kaikkien paikkojen yli, joissa varmuuskopio on, **Varmuuskopiot**-välilehdellään. B2:een kopioitu varmuuskopio näkyy kerran, merkittynä jokaisella paikalla, joka sen pitää. Palautus ottaa ensimmäisen paikan, johon se pääsee, aloittaen repositorysta, johon merkintä kirjoitetaan, ja voit valita toisen paikan varmuuskopiokohtaisesti. Etäkohteet luetaan vasta, kun napsautat **Tarkista**, koska pilvitallennuksen listaaminen kestää. Poistaminen yhdessä paikassa tarkistaa ensin muut ja kertoo, oliko se viimeinen kopio.

## Kohteet {#destinations}

Kohde on paikka, johon varmuuskopiot menevät: paikallinen kansio, jako, oma palvelin tai pilvipalvelu. Se määritetään kerran ja palvelee jokaista osiota. Avaa **Kohteet** sen asetusruudusta tai kytke siellä päälle **Näytä myös sivupalkissa** kiinnittääksesi sen.

**Lisää kohde** avaa ikkunan neljässä vaiheessa:

1. **Palveluntarjoaja.** Jokainen palvelu on listattu logoineen, hakukenttä ikkunan oikeassa yläkulmassa, ryhmissä: S3-bucketteja tarjoavat tallennuspalvelut (Backblaze B2, Wasabi, Cloudflare R2, Hetzner Object Storage, Amazon S3 ja muut), oma S3-palvelimesi (Garage, SeaweedFS, RustFS, Silo, Ceph, JuiceFS, Versity S3 Gateway), oma palvelimesi ja jakosi (rest-server, Hetzner Storage Box, SFTP, SMB, WebDAV, liitetty polku), pilvitallennus (OneDrive, Google Drive, Dropbox, pCloud, Nextcloud ja muut, joita rclone tukee) sekä ryhmäsi vastaanottopalvelimet. Jokainen kertoo, kuinka hyvin se sopii varmuuskopioille: pilvilevyt hidastuvat monien pyyntöjen alla, joten ensimmäinen varmuuskopio ja karsinta kestävät siellä kauemmin.
2. **Kirjautuminen.** Kentät riippuvat palvelusta: S3:lle käyttöavain, WebDAV:lle ja SMB:lle käyttäjä ja salasana, sovellussalasana, kun kaksivaiheinen kirjautuminen estää tavallisen, BombVaultin julkinen SSH-avain SFTP:lle ja Storage Boxille tai token palveluille, jotka kirjautuvat selaimen kautta. Niille ikkuna näyttää `rclone authorize` -komennon, joka ajetaan tietokoneella, jossa on selain; sen tulostama token liitetään kenttään. **Testaa** tarkistaa kirjautumisen ennen kuin mitään tallennetaan. Ikkuna listaa sitten kohteen kansiot, **Uusi kansio** -toiminnolla uuden luomiseen ja vapaalla tilalla, kun palvelu sen raportoi. Tyhjä kansio on turvallisin; BombVault luo siihen yhden kansion osiota kohti.
3. **Suojaus.** Ikkuna kertoo suoraan, mihin palvelu pystyy. Rest-server append-only-tilassa kieltäytyy poistosta, ja peukalointitesti tarkistaa sen. S3-bucket voi säilyttää vanhat versiot versioinnilla ja objektilukolla, mitä BombVault ei vielä voi tarkistaa. Pilvilevy ei voi kieltäytyä poistosta lainkaan: kuka tahansa, joka pääsee palvelimelle, pääsee myös tuohon kopioon. Kytke **Poistosuoja (append-only)** päälle vain siellä, missä vastapuoli todella kieltäytyy poistosta; BombVault ei sen jälkeen koskaan karsi siellä. **Hätätilanteen varalle** kertoo, mitä palautuspaketti sisältää tästä kohteesta.
4. **Mille osioille.** Valitse osiot, jotka varmuuskopioivat tänne. Ensimmäinen kopio tuo koko historian mukanaan. Merkinnät, jotka valitsivat omat kohteensa, eivät saa uutta itsestään, ja ikkuna nimeää ne.

S3-palvelut kulkevat resticin oman S3-taustajärjestelmän kautta, mikä mahdollistaa tallennusluokan ja objektilukon. Kaikki muut palvelut kulkevat BombVaultin mukana tulevan rclonen kautta, ja niiden etäsijainti ilmestyy **Etäsijainti (rclone)** -korttiin kohdassa **Asetukset, Kohteet**. Asetusten vienti sisältää kohteet; tunnistetiedot mukaan otettuina se sisältää myös niiden kirjautumisen.

Tämän palvelimen kansio on kaikkialla nimeltään **Paikallinen**: kohteiden sivuilla, virrassa, merkinnässä ja Palautus-sivulla. Jos tällä palvelimella on useita kansioita, kullakin on oma nimensä. Kohde, jonka osion etäkenttä luo, nimetään paikkansa mukaan eli osoitteensa viimeisen kansion mukaan tai, jos kansiota ei ole, palvelimensa mukaan, ja se saa numeron, jos jokin toinen kohde on jo samanniminen. Nimi, jonka annat sille sen sivulla, säilyy. Kun kaksi samannimistä kohdetta on rinnakkain, kummallakin on mukana palvelimensa tai kansionsa.

Jokaisella kohteella on oma sivunsa:

- **Yhteys**: nimi, kytkin **Käytettävissä** (pois tarkoittaa, ettei BombVault enää kirjoita tänne, ja merkinnät, jotka varmuuskopioivat tänne, epäonnistuvat selkeällä viestillä sen sijaan, että päätyisivät muualle), kansio, pakkaus, tallennusluokka ja **Rakennuksen ulkopuolella**. **Muuta yhteyttä** testaa ja tallentaa uuden osoitteen tai kirjautumisen.
- **Tunnistetiedot**: jaettu joukko tai oma joukko.
- **Poistosuoja**, jossa **Testaa poistosuoja nyt** ja se, kuka karsii tämän kopion.
- **Kopioi**: milloin kopioidaan, sekä lähetys- ja latausrajat.
- **Kuinka kauan säilytetään**: **Lyhyt**, **Tasapainoinen**, **Pitkä** tai **Omat arvot**, aikajanalla siitä, mitkä varmuuskopiot sääntö säilyttää. **Kysy ennen vähemmän säilyttämistä** kysyy, ennen kuin lyhyempi sääntö poistaa varmuuskopioita, joiden ainoa kopio on täällä.
- **Säännöt osioittain**: yhden osion oma repository, säilytys tai pakkaus tässä kohteessa. **Muuta sääntöjä** avaa ne, ja **Palauta kohteen asetukset** tuo osion takaisin.
- **Jo tämän kohteen alla**: osiot, joiden käsin kirjoitettu osoite on tämän kohteen kansiossa. Haltuunotto jättää repositoryn paikalleen; kirjautuminen, tallennusluokka ja poistosuoja tulevat sen jälkeen kohteelta. BombVault tarkistaa ensin, että kohteen kirjautuminen avaa repositoryn, ja kieltäytyy asettamasta append-only-repositorya kohteen alle, joka ei ole append-only.
- Astia, joka näyttää, kuinka täynnä kohde on, merkillä täyttymiskohdasta ja viikkojen määrällä, jotka jäljellä ovat viime viikkojen kasvuvauhdilla. Kohde, joka ei raportoi kokoaan, sanoo **Koko tuntematon**.

Ryhmäsi toisen instanssin ajama vastaanottopalvelin näkyy palveluntarjoajien joukossa kohdassa **Ryhmästäsi**; katso [Vastaanottopalvelin](#receiving-server).

### Siirtäminen ja poistaminen {#move-remove}

- **Siirrä** vie yhden osion kopion toiseen kohteeseen koko historioineen. Kaikki jää vanhalle paikalle; BombVault ei poista siellä mitään. Siirto ajetaan taustalla ja voi kestää: noin 9 minuuttia 100 Gt:tä kohti paikallisten levyjen välillä, tunteja hitaan yhteyden yli. Yleiskatsaus näyttää sen sen ajan.
- **Poista kohde** tarjoaa kaksi tietä. **Poista kohde, säilytä varmuuskopiot** lopettaa kirjoittamisen sinne ja jättää varmuuskopiot paikalleen; voit lukea ne myöhemmin kohdassa **Palautus, Vanhasta varmuuskopiokansiosta**. **Poista kohde ja poista siellä olevat varmuuskopiot** lukee ensin, mitä siellä on, ja listaa sen: mitkä varmuuskopiot poistuvat, mitkä niistä eivät ole missään muualla ja mitkä repositoryt jäävät, koska poistosuoja kieltäytyy, merkinnät varmuuskopioivat yhä suoraan niihin, toinen kohde tai osio säilyttää kopionsa siellä tai niitä ei voitu lukea. Merkinnät, jotka menettäisivät ainoan varmuuskopionsa, listataan erikseen ja säilytetään, ellet kytke päälle **Poista nämäkin**. Vahvistat kirjoittamalla kohteen nimen, ja poisto ajetaan taustalla.
- Kohteen varmuuskopioiden poistaminen vaatii kirjautumissalasanan ja kirjautumisen instanssissa. Ilman salasanaa kohteen voi yhä poistaa, vain sen varmuuskopioita ei voi poistaa täältä.

## Sijoittelu per merkintä {#placement}

Merkinnän Asetukset-välilehden kortissa **Aikataulu ja kohde** on painikerivi: **Paikallinen** ja yksi painike osion jokaista etäkohdetta kohti. Päällä olevat painikkeet saavat merkinnän varmuuskopiot.

- Kun **Paikallinen** on päällä, merkintä kirjoitetaan kohdassa **Tallennuspaikka** näkyvään repositoryyn ja kopioidaan jokaiseen muuhun päällä olevaan kohteeseen. Sammuta kohde, niin se ei saa tältä merkinnältä mitään uutta. Pelkkä Paikallinen ei kopioi minnekään, mikä sopii datalle, jolla on jo toinen kopio, esimerkiksi NAS:lla olevalle jaolle.
- Kun **Paikallinen** on pois, merkintä kirjoitetaan suoraan ensimmäisen päällä olevan kohteen suoraan repositoryyn ja kopioidaan sieltä muihin päällä oleviin. Ensimmäisellä kerralla ikkuna luo tuon suoran repositoryn.
- Yksi painike pysyy päällä, koska varmuuskopio tarvitsee paikan, minne mennä. Jättääksesi merkinnän varmuuskopioiden ulkopuolelle käytä **Älä varmuuskopioi**.

BombVault ei koskaan siirrä varmuuskopioita repositoryjen välillä. Merkintä, joka seuraa osiotaan, siirtyy mukana, kun osion oletus muuttuu: sen seuraava varmuuskopio päätyy uuteen paikkaan, ja jo olemassa olevat varmuuskopiot jäävät paikalleen. Merkinnälle itselleen valittu sijainti lukitaan sen ensimmäisestä varmuuskopiosta alkaen; Asetukset-välilehden kortti **Repository** nimeää sen. Kopiot voivat muuttua milloin tahansa. Kohde, joka ei enää saa merkintää, säilyttää kopiot, jotka sillä on, ja karsii ne omaan säilytykseensä osion seuraavassa etäajossa. Append-only-kohteista ei voi poistaa.

Paikka, jonka merkintä jätti, säilyttää varmuuskopionsa. Merkinnän **Varmuuskopiot**-välilehdellä ja Palautus-sivulla se näkyy nimellä **Vain aiemmat varmuuskopiot**, palautus voi yhä lukea sieltä, ja karsinta siellä jättää nuo varmuuskopiot rauhaan.

**Noudata oletusta** tuo merkinnän, jolla on oma valinta, takaisin osioonsa. Se kysyy ensin, kertoo, minne uudet varmuuskopiot silloin menevät, ja nimeää, mitä kukin kohde lakkaa saamasta ja mitä seuraava ajo lähettäisi.

Rivin alla kortti kertoo, minne merkintä menee ja mitä siellä todella on: kuinka moni paikka pitää sitä, milloin kukin kohde on viimeksi nähty ja täyttyykö 3-2-1. Paikka on palvelin alkuperäisen datan kanssa, jokainen etäkohde ja jokainen repository, joka on merkitty **Rakennuksen ulkopuolella**. BombVault tarkistaa kopiot ja paikat; se ei tarkista 3-2-1:n "kaksi tallennusvälinettä" -osaa.

### Oletukset osioittain {#placement-defaults}

Kohteet-sivun kortti **Kohteet osioittain** piirtää jokaisen osion oletuksen 3-2-1-virtana. Jokainen viiva osiosta kohteeseen on kytkin, myös **Paikallinen**, ja tämä koskee jokaista osiota, myös flash-asemaa, ZFS-tietojoukkoja ja itsevarmuuskopiota. Viivan tai pisteen napsautus kytkee sen heti, kysymättä; katkoviivat ovat pois päältä. **Kaavio / Kytkimet** näyttää saman painikeriveinä. Kytkin ratkaisee, minne uudet varmuuskopiot menevät. Merkinnät, joilla ei ole omaa valintaa, ja Compose-stackien projektikansiot seuraavat heti, joten niiden seuraava varmuuskopio menee uusiin paikkoihin. Jo tallennetut varmuuskopiot jäävät paikalleen, ja ne voi yhä palauttaa sieltä. Flash, ZFS-tietojoukot ja Itsevarmuuskopio varmuuskopioivat paikalliseen kansioonsa ja kopioivat etäkohteisiin siitä varmuuskopiosta. Niiden viivan kohteeseen **Paikallinen** voi myös kytkeä pois; niiden varmuuskopiot menevät silloin suoraan niiden etäkohteisiin, ja ZFS-merkintä, jolla on replika mutta ei yhtään kohdetta, lähettää vain replikansa.

Kun **Paikallinen** on pois ja vain etäkohteet ovat päällä, osio varmuuskopioi suoraan niihin, ja BombVault valitsee repositoryn siellä itse. Kontit, virtuaalikoneet ja kansiojoukot menevät ensimmäisen päällä olevan kohteen suoraan repositoryyn, jota kuvataan alempana, ja kopioidaan sieltä muihin. Flash-asema, ZFS-tietojoukot ja itsevarmuuskopio kirjoittavat jokaisen varmuuskopion jokaiseen päällä olevaan kohteeseen, ja kukin kohde säilyttää ne oman säilytyksensä mukaan.

Osio, jonka kaikki viivat ovat pois, näyttää **Ei varmuuskopioida**. Sen merkinnät eivät saa varmuuskopioita ja lasketaan suojaamattomiksi.

Merkinnät, jotka ovat valinneet omat kohteensa sivullaan, pitävät ne. Virta kertoo, kuinka monta niitä osiossa on, esimerkiksi **2 merkintää poikkeaa**, ja vie niiden luo: yhden kohdalla merkinnän Asetukset-välilehdelle, useamman kohdalla luetteloon, joka näyttää, minne kukin varmuuskopioi. **Noudata oletusta** tuo merkinnän siellä takaisin.

Uusi etäkohde saa jokaisen merkinnän, jota ei ole asetettu Paikalliseksi. Sen lisäävä ikkuna kertoo, kuinka monta merkintää ja, kun tiedetään, kuinka paljon historiaa se on.

### Suorat arkistot

Suora repository on paikka, jossa merkinnät varmuuskopioidaan suoraan etäkohteeseen. BombVault sijoittaa sen kohteen kansion viereen lisäämällä nimeen `-direct` tai, kun kohde on bucketin, etäsijainnin tai rest-server-käyttäjän juuressa, sen sisällä olevaan kansioon, joka on nimetty osion mukaan, esimerkiksi `containers-direct`. Kohteet-sivun virrassa se luodaan kysymättä. Kun Paikallinen sammutetaan yksittäiseltä merkinnältä niin, että kohteesta ilman suoraa repositorya tulee sen koti, avautuu ikkuna, jossa on ehdotettu sijainti, esimerkiksi `s3:https://s3.eu-central-003.backblazeb2.com/bucket/containers-direct`, ja yhteystesti, joka ei luo mitään. **Luo ja käytä** luo repositoryn ja osoittaa merkinnän siihen. Suora repository ottaa kohteen avaimen, tallennusluokan, rajat, append-only-asetuksen ja säilytyksen ja muuttuu niiden mukana; Repositoryt-kortti näyttää sen vain luettavana. Kun kohteen uusi avain ei voi avata sitä, suora repository pitää nykyisen avaimensa ja tallennus kertoo sen. Suoralla repositoryllä oleva merkintä kopioidaan sieltä muihin päällä oleviin kohteisiin, ei koskaan kohteeseen, jolle repository kuuluu. Sen varmuuskopioilla on tunniste `bv:direct`, ja jokainen muu säilytyskierros säilyttää ne, joten suora repository, joka menetti linkkinsä kohteeseensa, ei koskaan vanhene paikallisten sääntöjen mukaan. B2 tavoitetaan sen S3-päätepisteen kautta, avaintunnus ja sovellusavain S3-tunnuksina; kohteen omaan kansioon rajattu avain ei yllä viereiseen kansioon, joten rajaa avain sen yläpuoliseen kansioon.

### Rakennuksen ulkopuolella

Kohde tai nimetty repository voidaan merkitä **Rakennuksen ulkopuolella** -merkillä. Etäsijainnit alkavat merkittyinä; kytke se pois rest-serveriltä samassa rakennuksessa. Merkki vain laskee paikat ja 3-2-1:n korteilla. Se ei muuta mitään kopiota.

ZFS-replika on kopio tilojen ulkopuolella, mutta ei varmuuskopio historialla. Se säilyttää vain asettamasi tilannevedokset, ja vanhat versiot ja yksittäiset tiedostot tulevat yhä varmuuskopioista. Se kulkee yhdeltä ZFS-palvelimelta suoraan toiselle, SSH:n yli ZFS-palvelimelle tai pariliitetyn instanssin vastaanottopäätepisteeseen. Pariliitosryhmä kuljettaa vain pyynnön. Katso [Replika](zfs-datasets.md#replica).

### Uudelleenrakennuksen jälkeen {#after-a-rebuild}

Kopiointivalinnat sijaitsevat BombVaultin omissa asetuksissa. Uudelleenrakennuksen jälkeen kohdalla **Tunnista varmuuskopiot** tai Hätäohjatulla toiminnolla ilman palautettuja asetuksia ne ovat poissa, ja kaiken kopioiminen lähettäisi pois jättämäsi merkinnät taas B2:een. Siksi jokaisen uudelleenrakennetun osion etäkopiot pysähtyvät. Yleiskatsaus kertoo sen, ja kortti **Kohteet osioittain** tarjoaa **Vahvista oletus** -toiminnon, jossa on esikatselu siitä, mitä seuraava ajo kopioi, ja nimet varmuuskopioissa, joilla ei ole merkintää, jotka voit jättää siellä pois. Vain vahvistus päättää tauon; asetustiedoston tuonti tuo takaisin säännöt ja oletukset mutta ei päätä sitä.

## Etäsijaintiset ensisijaiset arkistot {#remote-primary-repositories}

Osion oma polku ei rajoitu paikalliseen kansioon: osoita se suoraan restic-etäsijaintiin (`s3:...`, `rest:http://host:8000/repo`, `sftp:user@host:/repo`, `rclone:remote:bucket/path`), niin BombVault varmuuskopioi suoraan sinne, ilman erillistä paikallista kopiota ja ilman kopiointivaihetta. Tämä on aidosti eri muoto kuin yllä olevat etäkopiot: siellä paikallinen repo on ensisijainen ja etärepo parhaan yrityksen arkisto siitä; täällä etärepo **on** ensisijainen, ja se on ainoa kopio, ellet kopioi osiota lisäksi toiseen kohteeseen.

Kunkin osion (Kontit, VMs, Flash, Itsevarmuuskopio, Kansiot, ZFS-tietojoukot) polku asetetaan **Paikallinen**-kohteen sivulla: **Muuta** osion vieressä sen **Yhteys**-kortissa avaa ikkunan, jossa on **Paikallinen / Etä** -kytkin:

- **Paikallinen** näyttää tutun kansioselaimen.
- **Etä** korvaa sen pelkällä URL-kentällä ja niillä yhteysasetuksilla, joita etäkohde käyttää, määritettynä tälle ensisijaiselle. Sieltä saat:
    - **Yhteystestin** todellista polkua vasten, ennen kuin luotat siihen.
    - **Kaistanleveyden rajat** (lähetys/lataus), jotta ajastettu varmuuskopio etäiseen ensisijaiseen ei tuki WAN-yhteyttäsi: samat `--limit-upload`/`--limit-download`-liput, joita etäkopiot käyttävät, sovellettuna itse varmuuskopioon.
    - **Append-only-suoja**, varmennettuna samalla aktiivisella peukalointitestillä (oikea DELETE-koetin vastapuolta vasten), jonka etäkohteet saavat. Kun se on päällä, BombVault kieltäytyy karsimasta repoa itse, koska sen takana ei ole erillistä paikallista kopiota eikä tämän laatikon tunnusten saa voida poistaa varmuuskopion ainoaa kopiota.
    - **Kasvubudjetin hälytyksen**, joka johdetaan samasta arkiston koon kehityksestä, jota Tallennus-kortti jo seuraa.

Mikään tästä ei ole pakollista: käsin kirjoitettu etäpolku ilman tallennettuja turva-asetuksia varmuuskopioi rajoittamatta, sen voi karsia eikä sillä ole budjettihälytystä. Turva-asetukset ovat sitä varten, että haluat samat suojaukset kuin etäkopiolla, tarvitsematta erillistä kohdetta pelkästään niiden saamiseksi.

!!! note "Pilvi- ja REST-tunnukset ovat yhteiset"
    Etäinen ensisijainen todentaa jaetuilla pilvitunnistetiedoilla, jotka on määritetty kohdassa **Asetukset, Kohteet**; ensisijaisille repoille ei ole erillistä tunnistetietovarastoa.

### SMB ja WebDAV ilman liitosta isäntään {#smb-webdav}

Kortissa **Etäsijainti (rclone)** kohdassa **Asetukset, Kohteet** on lomake Windows- tai Samba-jaolle ja WebDAV-palvelimelle (Nextcloud, ownCloud, SharePoint tai mikä tahansa muu). Täytä lyhyt nimi, isäntä ja jako (SMB) tai URL ja palvelintyyppi (WebDAV), käyttäjä ja salasana, niin BombVault kirjoittaa rclone-osion puolestasi. rclone hämärtää salasanan itse ennen tallennusta; sellaisen lisääminen nimellä, joka on jo olemassa, korvaa kyseisen osion toisen lisäämisen sijaan. **Lisää kohde** tarjoaa SMB:n ja WebDAV:n palveluntarjoajina samoilla kentillä.

Lomake vastaa valmiilla sijainnilla, esimerkiksi `rclone:nas:backups`. Käytä sitä osion polkuna tai kohteena ja lisää alikansio, jos haluat (`rclone:nas:backups/bombvault`). Jako on polun ensimmäinen segmentti, ei osa nimeä.

Tämä on parempi tie kuin jaon liittäminen Unraidiin: restic neuvoo olemaan pitämättä repositorya liitetyllä CIFS-jaolla, ja täällä mitään ei liitetä. NFS ei ole lomakkeessa, koska resticillä eikä rclonella ole NFS-taustajärjestelmää; NFS:lle liitä jako isännälle ja käytä sitä paikallisena kansiona.

## Poistosuoja (append-only)

Kytke poistosuoja päälle etäkohteelle, jotta kiristysohjelma tai kaapattu isäntä ei voi poistaa tai kirjoittaa uudelleen varmuuskopioitasi. Vastapuoli (`restic/rest-server` tilassa `--append-only`) **panee sen täytäntöön**. BombVault vain **varmentaa** sen eikä koskaan näytä vihreää pelkän kokoonpanoväitteen perusteella. Tämän palvelimen kansio ei voi kieltäytyä poistosta, koska se, joka hallitsee BombVaultia, hallitsee myös kansiota, joten paikalliset kohteet eivät tarjoa kytkintä.

Nopein tie append-only-kopioon on pariliitetty BombVault, jolla on [vastaanottopalvelin](#receiving-server): se ajaa rest-serverin puolestasi append-only-tilassa.

!!! note "Onnistunut poisto polussa `/locks/` on odotettua"
    Append-only ei tarkoita, ettei mitään voisi enää poistaa. resticin on otettava ja vapautettava omat lukkonsa, joten `/locks/` pysyy tarkoituksella kirjoitettavana ja poistettavana. Varmuuskopioita ja niiden takana olevaa dataa, eli juuri sitä mihin kiristysohjelma tähtäisi, ei voi poistaa. Jos koettelet vastapuolta itse, onnistunut poisto polussa `/locks/` on oikea toiminta eikä aukko suojauksessa.

!!! warning "Append-only-repoja ei koskaan karsita tältä laatikolta"
    Append-only-kohde ei koskaan karsi vanhoja varmuuskopioita täältä; karsinta on vastapuolen työtä. Aseta sille **kasvubudjettihälytys**, jotta saat varoituksen, ennen kuin repon koko karkaa käsistä.

## Peukalointitesti

BombVault todistaa ajoittain append-only-takeen yrittämällä oikeasti poistoa etärepoa vastaan, kohdistettuna olemattomaan objektiin. **Testaa poistosuoja nyt** kohteen sivulla ajaa sen pyynnöstä, ja **Tarkista poistosuoja** kohdassa **Asetukset, Tarkistus, Automaattiset tarkistukset** ajaa sen aikataulun mukaan.

- **Torjuttu** tarkoittaa suojattua.
- **Hyväksytty** tarkoittaa ei-suojattua.
- **Epäselvä** tulos (palvelin tavoittamattomissa, todennusvirhe) ei koskaan käännä tallennettua tuomiota.

Todellinen suojatusta suojaamattomaksi -kääntyminen laukaisee yhden hälytyksen.

## DR-harjoitukset

BombVault tarjoaa kaksi tasoa todisteita siitä, että varmuuskopiosi ovat tosiasiassa palautuskelpoisia, eivät vain olemassa.

- **Palautuksen todentamisharjoitukset (paikalliset).** BombVault ajaa ajoittain `restic check --read-data-subset` (rajattu, ei koskaan levyn täyttävä täyspalautus). Tahti on kohdassa **Asetukset, Tarkistus, Automaattiset tarkistukset**, ja saman ruudun kortti **Tarkista varmuuskopiot** näyttää tuloksen.
- **Koepalautus (etä).** BombVault palauttaa oikean kohteen etärepositorysta eristettyyn hiekkalaatikkoon, varmentaa sen tiedosto tiedostolta ja tavu tavulta ja siivoaa sitten. Tämä todistaa, että voit toipua etäkopiosta, ei vain että repo vastaa.

Yleiskatsauksen kortti **Suojaustila** kokoaa tämän osioittain neljäksi tarkistukseksi: ajantasainen varmuuskopio tällä palvelimella, kopio etänä, poistosuoja siellä ja koepalautus sieltä. Osion napsautus näyttää tiedot, ja kortti muuttuu vihreäksi vain varmennetuilla tosiasioilla.

## Instanssien pariliitos {#pairing}

Useat BombVaultit toimivat yhdessä ryhmänä: ne näyttävät toistensa suojaustilan, ottavat toisilleen rooleja ja voivat säilyttää toistensa avainta. Instanssi liittyy ryhmään kahdellatoista sanalla.

Avaa ensimmäisellä instanssilla **Asetukset, Pariliitos** ja paina **Luo lause**. Kolme vaihekorttia näyttää, miten se menee. Kaksitoista sanaa ilmestyy ikkunaan, jossa on **Kopioi**-painike. Avaa jokaisella muulla instanssilla sama paikka, paina **Syötä lause** ja liitä tai kirjoita ne, tai paina **Liitä** siinä ikkunassa. Sana, jota ei ole luettelossa, nimetään sijaintinsa kanssa kirjoittaessasi, ja viimeisessä sanassa on tarkistussumma, joten väärin kirjoitettu tai vaihtunut sana huomataan, ennen kuin mitään pariutuu. Luo lause vain yhdellä instanssilla: kaksi instanssia, jotka kumpikin luovat sellaisen, muodostavat kaksi erillistä ryhmää. Jos kukaan ei ilmesty minuutin kuluessa, sivu tarjoaa kaksi ulospääsyä: näytä sanat uudelleen syötettäviksi toisaalla tai syötä toisen instanssin sanat ja liity sen ryhmään yhdellä kertaa. Pariliitos toimii ilman kirjautumissalasanaa, mutta aseta sellainen: ilman sitä kuka tahansa, joka voi avata verkkokäyttöliittymän, voi lukea sanat ja ryhmän kautta pyytää rooleja, jotka luovuttavat siinä olevien instanssien restic-salasanan. Pariliitoskortti kertoo sen, kunnes salasana on asetettu. Salasanan kanssa lauseen näyttäminen uudelleen kysyy sitä. **Poistu ryhmästä** ottaa instanssin pois. Pariliitossivun **Ilmentymät**-kortti listaa ryhmän jäsenet; kuva siitä, kuka tekee mitä kenelle, on Ilmentymät-sivulla.

Kuka tahansa sanat tietävä voi liittyä ryhmään, joten kohtele niitä kuin salasanaa.

**Miten jäsenet tavoittavat toisensa.** Jokainen instanssi oppii oman osoitteensa verkossa selaimestasi heti kirjautuessasi, listattuna kohdassa **Tunnetut osoitteet** pariliitossivun **Tämä instanssi** -kortissa yhdessä niiden kanssa, jotka instanssi löysi itse; korjaa se siellä, jos käänteinen välityspalvelin tai epätavallinen portti on edessä. Samassa verkossa jäsenet ilmoittavat tuon osoitteen multicastilla ja puhuvat suoraan, ja siellä missä multicast ei pääse konttiverkon yli, kuten Dockerin oletussillassa, instanssi hakee sen sijaan omasta aliverkostaan muita allekirjoitetulla kutsulla, johon vain ryhmän jäsen voi vastata, joten pariliitos valmistuu sekunneissa ilman relettä. Jos mitään ei löydy, **Etkö löydä sitä?** ottaa yhden osoitteen käsin, toista aliverkkoa tai epätavallista porttia varten. Eri verkoissa olevat instanssit kulkevat releen kautta, joka valitaan neljän rivissä samalla sivulla olevassa **Rele**-kortissa; jokaisella muodolla on kuva:

- **Projektin rele** (oletus): `parleyport.halleluja.design`, sama rele jota myös KnightLoader käyttää. Ei mitään asennettavaa.
- **Oma rele**: Unraidin Community Appsin [**ParleyPort**](https://github.com/junkerderprovinz/parleyport)-kontti (myös kohdassa **Asetukset, Sovellukset**). Syötä sen osoite jokaisella instanssilla, jonka pitää käyttää sitä.
- **BombVault-instanssi**: yksi instansseistasi, joka on jo tavoitettavissa ulkoa ja jolla **Toimi releenä** on päällä. Tuo instanssi vastaa silloin polussa `/relay/connect` omassa osoitteessaan, jo olemassa olevan käänteisen välityspalvelimen ja varmenteen takana, ja päästää sisään vain ryhmäsi. Syötä sen osoite jokaisella instanssilla, jonka pitää käyttää sitä.
- **Ei relettä**: jäsenet löytävät toisensa automaattisesti vain samassa verkossa, eivät missään muualla.

**Mitä rele näkee.** Samanniminen painike neljän muodon vieressä näyttää tämän valitsemallesi muodolle. Jokainen jäsenten välinen kutsu sinetöidään AES-256-GCM:llä kahdestatoista sanasta johdetulla avaimella, eikä tuo avain koskaan poistu instansseistasi. Rele oppii tiivisteen, joka ryhmittelee yhteydet, mille instanssille viesti on tarkoitettu, kuinka suuri se on ja milloin se kulkee. Suora kutsu paikallisessa verkossa sinetöidään samoin ja allekirjoitetaan lisäksi, joten mikään ei riipu instanssin tarjoamasta itse allekirjoitetusta varmenteesta. Kutsut, jotka luovuttavat jotakin (roolivastaukset, restic-salasana, vastaanottajan tunnukset, avaintalletukset), allekirjoitetaan myös ne lähettävän jäsenen avaimella, kiinnitettynä ensimmäisellä näkemällä; katso [Turvallisuusmalli](configuration.md#security-model).

**Mitä ryhmässä kulkee.** Ilmentymät-sivun suojaustila, pyyntö tarkistaa yksi osio nyt, roolipyynnöt ja niiden vastaukset, kohdetarjoukset sekä se, mitä vastaanotin tai noutaja tarvitsee, kun sen rooli on sallittu: toisen instanssin repositoryjen sijainnit ja sen restic-salasana. Varmuuskopiodata ei koskaan; se kulkee yhä suoraan restic-taustajärjestelmiin. Eikä APP_KEY: restic-salasana avaa kyseisen instanssin repositoryt eikä mitään muuta, ei sen tallennettuja salaisuuksia, istuntoja tai noutokoodeja.

**Merkinnät ennen pariliitosta.** Laiteryhmätunnuksella lisätyt instanssit sekä vastaanotetut repositoryt ja noutolähteet, jotka on asetettu toisen instanssin APP_KEY:llä, säilyvät päivityksen jälkeen ja merkitään **Pariuta uudelleen**. Ne toimivat edelleen: ensimmäisellä käynnistyksellä BombVault korvaa jokaisen tallennetun APP_KEY:n siitä johdetulla restic-salasanalla. Pariuta molemmat instanssit, muokkaa sitten merkintää ja valitse sen instanssi. Siihen asti tällaiset merkinnät ovat Ilmentymät-sivun kortissa **Ilman instanssia**.

Ainoa paikka, joka yhä ottaa APP_KEY:n käsin, on [Vanhasta varmuuskopiokansiosta](#restore-from-another-bombvault-repo), tilanteeseen, jossa toinen instanssi on poissa eikä voi vastata ryhmässä.

## Ilmentymät ja roolit {#roles}

**Ilmentymät**-sivu (asetusruutu, jonka voi kiinnittää sivupalkkiin) näyttää ryhmän verkkona siitä, kuka tekee mitä kenelle, kortti tälle instanssille ja kortti jokaiselle muulle jäsenelle, Android-puhelimille ja ZFS-palvelimille. **Pariliitos** oikeassa alakulmassa vie pariliitossivulle. Mitkä osat ovat päällä, asetetaan kohdassa **Asetukset, Ilmentymät, Useita palvelimia**: **Ilmentymät**, **Vastaanotin** ja **Nouto**.

Jokainen kortti näyttää jäsenen suojaustilan ja sen roolit ruutuina. Rooli on jotakin, mitä yksi instanssi tekee toiselle:

- **Vastaanotin** säilyttää tämän palvelimen etäkopiot rest-serverissä append-only-tilassa. Tämä palvelin lähettää jokaisen varmuuskopion jälkeen eikä voi poistaa siellä mitään, vaikka joku kaappaisi sen.
- **Noutaja** hakee tämän palvelimen varmuuskopiot omalla aikataulullaan. Tämän palvelimen ei tarvitse olla hereillä sitä varten.
- **ZFS-palvelin** ottaa vastaan tämän palvelimen ZFS-replikat. Jokainen ZFS-merkintä valitsee sen asetuksissaan kohdassa **Replika**.
- **Säilyttäjä** säilyttää tämän palvelimen palautuspaketin sinetöityä kopiota. Katso [Avaimen säilyttäjä](#key-keeper).

Roolin kytkeminen päälle lähettää pyynnön, eikä mikään käynnisty, ennen kuin joku toisessa instanssissa painaa **Salli**. Pyyntö näkyy sen instanssin kortilla merkkinä, ja sen **Tiedot** listaavat sen toiminnoilla **Salli** ja **Hylkää**; sallitun roolin voi peruuttaa siellä milloin tahansa. Katkoviiva verkossa on rooli, joka yhä odottaa. BombVault 9.x -versiolla oleva instanssi ei voi vastata pyyntöön, joten rooli on siellä aktiivinen heti kun kytket sen päälle, ja 9.x-instanssin pyyntö odottaa täällä **Salli**-toimintoa.

Kortin **Tiedot** avaa loput: jäsenen vastaanotetut repositoryt ja noutolähteet, kohdetarjoukset, valvonta, **Poista** ja **Unohda avain**.

## Vastaanotin ja noutaja {#receiver}

![Vastaanotettu etäkopio instanssin tiedoissa, vain luku -valvonnassa, eheystarkistuksella ajettuna tällä laitteistolla.](assets/screenshots/receiver.png)

*Vastaanotettu etäkopio instanssin tiedoissa, vain luku -valvonnassa, eheystarkistuksella ajettuna tällä laitteistolla.*

Laatikossa, joka **vastaanottaa** append-only-kopioita toiselta BombVaultilta, vastaanotetut repositoryt antavat riippumattoman, vain luku -tilaisen valvonnan vastaanottavalla laitteistolla, jotta äänetön vika vastapäässä ei jää huomaamatta. Kytke **Vastaanotin** päälle kohdassa **Asetukset, Ilmentymät**, avaa sitten lähettävän instanssin **Tiedot** Ilmentymät-sivulla ja lisää vastaanotettu repository (vain luku, avattuna lähettäjäinstanssin restic-salasanalla, joka tulee [pariliitosryhmän](#pairing) kautta). Saat:

- **Tilannevedosluettelon lähteittäin ryhmiteltynä**, jotta näet tarkalleen, mitkä kontit, VMs ja kansiojoukot ovat saapuneet.
- **Viimeksi vastaanotettu** lähteittäin, jotta tiedät, kuinka tuore kukin on.
- **Riippumattoman `restic check`** -ajon vastaanottavalla laitteistolla, jotta eheys todennetaan siellä missä data tosiasiassa sijaitsee, ei vain lähettäjällä.
- **Kuolleen miehen kytkimen:** hälytys, kun lähde lakkaa lähettämästä asettamasi ikkunan sisällä.
- **Eheyshälytykset:** hälytys, kun tarkistus vastaanottavalla puolella epäonnistuu.

Vastaanotin on tiukasti vain luku. Se ei koskaan kirjoita vastaanotettuun repositoryyn, joten se ei voi koskaan rikkoa append-only-takuuta, johon lähettäjä luottaa.

Noutaja toimii toisin päin: kun **Nouto** on päällä, instanssi lukee toisen jäsenen varmuuskopiot omaan repositoryynsä omalla aikataulullaan. Sen lähteet ovat myös kyseisen jäsenen tiedoissa.

### Vastaanottopalvelin {#receiving-server}

Vastaanottava laatikko voi myös ajaa rest-serverin, johon muut kopioivat. Se määritetään ensimmäisellä kerralla, kun sallit **Vastaanotin**-pyynnön, joka sitä pyytää: ikkuna kertoo, että palvelin tarvitsee vastaanottopalvelimen, ja kysyy kansiota jaolta, **Uusi kansio** -toiminnolla uuden luomiseen, sekä porttia (8000, ellei toinen kontti käytä sitä). BombVault sitten:

1. kieltäytyy jatkamasta, jos kontti nimeltä `rest-server` on jo olemassa tai toinen kontti pitää porttia;
2. hakee `restic/rest-server`in ja käynnistää sen Dockerin socketin kautta append-only-tilassa yksityisillä repositorioilla ja kansiossa olevalla kirjautumistiedostolla;
3. kirjoittaa sen Unraid-mallin flash-asemalle, jolloin kontti pysyy muokattavana Docker-välilehdellä, tai tarjoaa mallin ladattavaksi, kun flash-asema ei ole saatavilla;
4. ajaa sille peukalointitestin ja näyttää, kieltäytyykö se poistoista.

Ryhmäsi instanssit löytävät palvelimen sitten palveluntarjoajien joukosta kohdassa **Ryhmästäsi**, vastaanottavan laatikon nimellä, ja **Vastaanotin**-rooli tarjoaa sen turvallisena valintana. Jokainen instanssi saa oman tunnuksen, kun se ensimmäisen kerran käyttää palvelinta, ja kirjoittaa siellä vain omaan kansioonsa. Kun **Vastaanotin** on päällä kohdassa **Asetukset, Ilmentymät**, tämän instanssin kortti **Vastaanottopalvelin** avaa palvelimen uudelleen. Sen ikkuna listaa tunnukset kohdassa **Kumppanit, joilla on tunnus**, ja **Peruuta tunnus** poistaa yhden; mitä kyseinen instanssi on jo kopioinut, jää kansioon. Käyttöönotto luo myös yhden **Tunnuksen ryhmän ulkopuolelle**, jonka salasanan ikkuna näyttää kerran.

Instanssi, joka tavoittaa vastaanottavan laatikon vain releen kautta, ei voi käyttää palvelinta, koska rele ei kuljeta varmuuskopioita. Lisää vastaanottavan laatikon osoite pariliitossivulla ensin. Kun BombVault toimii omalla IP-osoitteellaan (esimerkiksi br0:ssa), täytä **Osoite kumppaneille**, koska palvelin kuuntelee isännän osoitteessa.

## Läpikäyty esimerkki: kaksi Unraid-konetta, päästä päähän

Kaikki yllä kuvaa osat. Tämä on yksi täydellinen käyttöönotto, koska osat on helpompi koota, kun on nähnyt ne koottuina kerran.

Kaksi laatikkoa: **TOWER** ajaa kontteja ja työntää varmuuskopioita; **VAULT** vastaanottaa ne ja panee append-only-tilan täytäntöön. Korvaa omilla nimilläsi, osoitteillasi ja jakopoluillasi.

**1. Pariuta kaksi laatikkoa.** Avaa TOWERilla **Asetukset, Pariliitos**, paina **Luo lause** ja syötä kaksitoista sanaa VAULTilla kohdassa **Syötä lause**. Aseta ensin kirjautumissalasana molemmille.

**2. Pyydä TOWERilta VAULTia vastaanottajaksi.** Kytke Ilmentymät-sivulla **Vastaanotin** päälle VAULTin kortilla, valitse osiot ja paikaksi **VAULTin vastaanottopalvelin**, ja lähetä pyyntö.

**3. Salli se VAULTilla.** VAULTin Ilmentymät-sivu näyttää pyynnön TOWERin kortilla. Paina **Salli**. Ensimmäisellä kerralla VAULT kysyy kansiota vastaanottopalvelimelleen, kuten `user/appdata/rest-server`, käynnistää rest-serverin append-only-tilassa ja ajaa sitä vastaan peukalointitestin.

**4. Takaisin TOWERilla.** Siitä lähtien TOWER kopioi VAULTille jokaisen varmuuskopion jälkeen, ja uusi kohde ilmestyy TOWERin Kohteet-sivulle poistosuojineen.

Mitä peukalointitesti vastaa:

| Tulos | Mitä tapahtui |
| --- | --- |
| **suojattu** | VAULT kieltäytyi poistosta. Tämä on ainoa hyväksytty tila. |
| **ei suojattu** | VAULT hyväksyi poiston. `--append-only` puuttuu tai se poistettiin. |
| **ei ratkaiseva** | Ei kumpikaan. Yleensä osoite ei ole se, jota restic itse käyttää, tai tunnukset ovat muuttuneet. Mitään ei kirjata eikä hälytystä laukaista. |

**5. Seuraa saapuvaa, jos haluat.** Kytke **Vastaanotin** päälle VAULTilla kohdassa **Asetukset, Ilmentymät**, avaa TOWERin **Tiedot** ja lisää vastaanotettu repository. VAULT saa TOWERin restic-salasanan ryhmän kautta; kukaan ei kirjoita avainta.

**6. Tee siitä molemminpuolinen.** Toista vaiheet toiseen suuntaan: VAULT pyytää TOWERia vastaanottajakseen. Kumpikin laatikko panee sitten append-only-tilan täytäntöön toiselle, eikä kumpikaan voi poistaa toisen varmuuskopioita.

Käsin ajettu rest-server toimii myös: lisää se kohdalla **Lisää kohde** rest-serverinä, käyttäjällä ja salasanalla sen `.htpasswd`-tiedostosta, ja kytke poistosuoja päälle. Pidä `--append-only` sen asetuksissa; ilman sitä vastapuoli on taas tavallinen jako.

!!! warning "Paikallinen polku on polku kontin sisällä"
    Kun BombVault pyytää kansiota tältä palvelimelta, syötä se suhteessa isännän liitokseen, kuten `user/appdata/rest-server/bombvault-containers`, ei `/mnt/user/appdata/...`. BombVault toimii kontissa, jossa isännän `/mnt` on liitetty muualle; isännän absoluuttista polkua ei ole sen sisällä. Jos liität sellaisen, BombVault kertoo suhteellisen polun, jota käyttää sen sijaan.

## Hätäohjattu toiminto {#emergency}

**Hätäohjattu toiminto** tuo takaisin palvelimen, joka on poissa tai pystytetty uudelleen. Se alkaa **Palautus**-sivulta kohdasta **Hätätilanne** sekä uuden asennuksen tervetulosivun kohdasta **Palauta varmuuskopiosta**. Se etenee neljässä vaiheessa: **Pääsy**, **Asetukset**, **Valinta** ja **Palautus**.

1. **Pääsy.** Näytä BombVaultille vanhojen varmuuskopioiden avain yhdellä kolmesta tavasta:
    - **Lataa palautuspaketti**: käyttöönoton aikana lataamasi tiedosto. Se sisältää avaimen ja jokaisen kohteen, ja BombVault täyttää loput sen perusteella. Age-salattu paketti kysyy identiteettiä, jolle se sinetöitiin; sitä käytetään kerran eikä tallenneta.
    - **Ryhmän kautta**: ryhmäsi instanssi säilyttää avainta talletuksena. Tarvitset ryhmän kaksitoista sanaa, noutokoodisi ja jonkun kyseisessä instanssissa. Katso [Avaimen säilyttäjä](#key-keeper).
    - **Kirjoita avain**: vanhan palvelimen `APP_KEY` ja se, missä sen varmuuskopiot ovat.

    BombVault kokeilee sitten avainta jokaisessa kohteessa ja kertoo kustakin, onko se luettavissa, ei löydy, on lukittu toisella avaimella, ei ole tavoitettavissa vai puuttuuko siltä vielä kirjautuminen. Mitään ei ole vielä tallennettu, ja avain pysyy muistissa rajoitetun ajan. **Ota avain haltuun** saa tämän palvelimen kirjoittamaan varmuuskopionsa tuolla avaimella tästä lähtien; **Kumoa haltuunotto** palaa sen omaan avaimeen, kunnes ensimmäinen varmuuskopio on ajettu. Palvelin, jolla on jo omia varmuuskopioita, kieltäytyy haltuunotosta ja ohjaa kohtaan [Vanhasta varmuuskopiokansiosta](#restore-from-another-bombvault-repo). **Tallenna tunnukset** tallentaa tunnukset, jotka paketti sisältää etäkohteille, ja kysyy ennen kuin korvaa tunnukset, jotka tällä palvelimella jo on.
2. **Asetukset.** **Palauta asetukset** palauttaa BombVaultin omat asetukset uusimmasta Itsevarmuuskopiosta: kohteet, tunnistetiedot, aikataulut ja säännöt. BombVault käynnistyy uudelleen ottaakseen ne käyttöön Docker-soketin kautta, jotta käytössä olevaa asetustietokantaa ei koskaan ylikirjoiteta avoimen kahvan alla, ja ohjattu toiminto jatkuu itsestään sen jälkeen. Tämän palvelimen kirjautuminen säilyy: kirjaudut yhä nykyisellä salasanallasi. Salaisuudet, jotka vanha palvelin oli tallentanut ja joita ei voi ottaa haltuun, listataan, jotta voit syöttää ne uudelleen.
3. **Valinta.** BombVault käy läpi varmuuskopiosi ja listaa löytämänsä kontit, VMs, kansiojoukot ja ZFS-tietojoukot. Uusin varmuuskopio on esivalittu kaikkialla. Se, mikä tällä palvelimella yhä on, alkaa pois päältä, koska palautus korvaisi sen. Kansiojoukot eivät enää tiedä, mistä ne tulivat uudelleenrakennuksen jälkeen, joten ne menevät valitsemaasi kansioon.
4. **Palautus.** Kaikki valittu palaa kerralla, jätettynä pysäytetyksi, jos haluat käynnistää jokaisen kontin ja VM:n itse, mikä on turvallisinta täydellisen menetyksen jälkeen. Lataa sen jälkeen uusi palautuspaketti: vanha kuuluu edelliselle asennukselle.

!!! tip "Suunniteltu siirto vastaan katastrofi"
    Hätäohjattu toiminto palauttaa BombVaultin omat asetukset varmuuskopiosta. *Suunniteltua* siirtoa uuteen laatikkoon varten voit sen sijaan viedä kokoonpanosi suoraan **Vie / tuo asetukset** -kortilla (siirrettävä JSON-tiedosto). Katso [Asetukset](configuration.md#portable-settings-export-and-import).

### Vanhasta varmuuskopiokansiosta {#restore-from-another-bombvault-repo}

**Vanhasta varmuuskopiokansiosta** kohdassa **Muut lähteet** Palautus-sivulla avaa *toisen* BombVault-instanssin repositoryn (`/mnt`-alle liitetty jako tai etäosoite) kyseisen instanssin palautuspaketilla tai sen kirjoitetulla `APP_KEY`:llä kertaluonteisessa, vain luku -istunnossa. Selaa siellä säilytettyjä kontteja, VMs, kansiojoukkoja ja ZFS-tietojoukkoja, valitse varmuuskopio ja palauta se, ja palautetusta kohteesta tulee tavallinen paikallinen kontti, VM tai kansiojoukko. Toiseen repositoryyn ei koskaan kirjoiteta, ja omat varmuuskopioasetuksesi pysyvät koskemattomina (istunto elää muistissa ja vanhenee itsestään). Kontin siirtäminen palvelimelta A palvelimelle B ei tarkoita oman repo-asetuksesi osoittamista toisaalle ja palauttamista jälkeenpäin. Tämä ikkuna on kertakäyttöinen: se avaa istunnon, palauttaa valitsemasi ja unohtaa toisen instanssin. Jos haluat sen sijaan pysyvän järjestelyn, jossa tämä laatikko hakee toisen instanssin varmuuskopiot omaan repositoryynsä aikataulun mukaan, se on **Noutaja**-rooli Ilmentymät-sivulla.

Kontti, jonka verkkoa tällä palvelimella ei ole, esimerkiksi Unraidin `br0`-verkko tavallisella Docker-isännällä, näyttää rivinsä alla verkon valinnan. BombVault luo sen valitsemaasi verkkoon muine verkkoineen. Kiinteä IP-osoite ja MAC-osoite kuuluivat vanhaan verkkoon ja jäävät pois, joten uusi verkko antaa ne.

## Avaimen säilyttäjä {#key-keeper}

Avaimen säilyttäjä on ryhmäsi instanssi, joka pitää tämän palvelimen palautuspaketin sinetöityä kopiota. Se ei voi itse avata kopiota. Jos tämä palvelin joskus katoaa, uusi asennus, joka liittyy ryhmääsi, pyytää kopion säilyttäjältä ja avaa sen noutokoodillasi.

1. Kytke Ilmentymät-sivulla **Säilyttäjä** päälle sen instanssin kortilla, jonka pitää säilyttää, ja valitse **Tee talletus**. Talletuksen tekeminen vaatii kirjautumissalasanan.
2. Tämä palvelin näyttää **Noutokoodin** kerran: kaksikymmentä kirjainta ja numeroa neljässä viiden ryhmässä. Kirjoita se muistiin ja vahvista **Kirjoitin koodin muistiin**. Tämä palvelin ei tallenna koodia, vain sen, mitä se tarvitsee talletuksen sinetöintiin sille.
3. Säilyttäjä pitää nyt talletusta, ja tämä palvelin uusii sen, kun paketti muuttuu. **Avaimen talletus** tämän instanssin kortilla listaa **Tämän palvelimen talletuksen** kunkin säilyttäjän ja sen tilan kanssa sekä **Talletukset, joita tämä palvelin säilyttää** muille. **Tee uusi koodi** korvaa koodin; jokainen säilyttäjä siirtyy uuteen, kun se saa uuden talletuksen.

Sinä päivänä tarvitset kolme asiaa: ryhmäsi kaksitoista sanaa, noutokoodin ja pääsyn säilyttäjän käyttöliittymään, jossa jonkun täytyy painaa **Salli**. Valitse Hätäohjatussa toiminnossa **Ryhmän kautta**, valitse talletus, pyydä sitä ja kirjoita noutokoodi. Koodia käytetään uudella palvelimella, eikä sitä lähetetä säilyttäjälle. Säilyttäjällä pyyntö näkyy kohdassa **Avaimen talletus** pyytäjän, sen, onko kyseinen instanssi koskaan nähty siellä aiemmin, ja pyyntöjen määrän kanssa; **Salli** luovuttaa sinetöidyn kopion, ja sallimiseen tarvitaan kirjautumissalasana. Uudella asennuksella on 30 minuuttia aikaa noutaa se Sallimisen jälkeen, pyyntö, johon kukaan ei vastaa, vanhenee päivän kuluttua, ja instanssin, joka pyytää liian usein, täytyy odottaa tunti.

Ilman koodia kukaan ei voi avata kopiota, säilyttäjäkään. Jos koodi katoaa, tee uusi, kun tämä palvelin yhä toimii; jos palvelin on jo poissa, palautuspaketti ja kirjoitettu APP_KEY toimivat yhä. Säilyttäjä voi poistaa talletuksen milloin tahansa, ja omistajan täytyy silloin pyytää uudelleen.

## Palautuspaketti {#recovery-kit}

Tämä on se pala, joka tekee katastrofista toipumisen mahdolliseksi silloinkin kun käynnissä olevaa BombVaultia ei ole.

**Lataa palautuspaketti** kortissa **Hätätilanteiden varalle** kohdassa **Asetukset, Palautus** antaa sinulle **avaimen**, **johdetun restic-salasanan**, jokaisen **kohteen** kunkin osion repositoryineen, tunnukset, joita uusi asennus tarvitsee niihin pääsyyn, sekä **tarkat restic-komennot**, jotta voit palauttaa suoraan restic-komentorivillä millä tahansa koneella. Se vaatii kirjautumissalasanan. Paketti tulostaa avaimen myös salaus pois päältä, koska kontti- ja VM-määritykset sinetöidään aina sillä. Haltuunoton jälkeen se tulostaa haltuun otetun varmuuskopioavaimen ja tämän palvelimen APP_KEY:n, jota uusi asennus tarvitsee vain asetusten palauttamiseen. Yleiskatsauksen kortti **Tallenna palautuspakettisi** muistuttaa, kunnes olet tallentanut sen.

!!! danger "Säilytä palautuspaketti palvelimen ulkopuolella"
    Paketti sisältää salaisuuden, joka purkaa varmuuskopiosi. Säilytä se turvallisessa paikassa erillään palvelimesta (salasanahallinta, tulostettu kopio kassakaapissa). Jos menetät sekä BombVaultin että `APP_KEY`:n ilman palautuspakettia ja ilman avaintalletusta, salattuja varmuuskopioitasi ei voi palauttaa.

!!! warning "Uusin varmuuskopio ei aina ole se, joka kannattaa palauttaa"
    Resticistä 0.17 lähtien `restic snapshots` näyttää kunkin tilannevedoksen koon. Tietojen menetyksen jälkeen uusin varmuuskopio voi olla tyhjennetty, joten älä palauta varmuuskopiota, joka on paljon pienempi kuin sitä edeltävät. Kiristysohjelman jälkeen se voi olla salattu tavallisen kokoinen. Jos BombVault yhä toimii, katso ensin sen **Poikkeamat**-sivu: se nimeää viimeisen hyvän varmuuskopion, ja palautusikkuna esivalitsee sen. Palautus ei tarvitse mitään BombVaultin poikkeamadataa, ja säilytyksen tauko vain säilyttää enemmän varmuuskopioita.

### Paketin sinetöinti

Jos olet kytkenyt päälle **Salaa viennit age:lla** kohdassa **Asetukset, Järjestelmä**, paketti sinetöidään myös sillä ja latautuu nimellä `bombvault-recovery-kit.md.age`. Se on ASCII-panssaroitu eikä binäärinen, joten se on yhä tekstiä: sen liittäminen salasanahallintaan tai tulostaminen toimii täsmälleen kuten ennen, sisältö vain on lukukelvoton ilman avaintasi.

!!! warning "Älä säilytä age-avainta paketin sisällä"
    Sinetöidyn paketin avaamiseen tarvitset age-**yksityisavaimesi**. Säilytä se paikassa, joka ei riipu itse paketista, tai sinulla on palautettavana kaksi asiaa yhden sijaan. Sinetöinti kannattaa, kun paketti on tallessa paikassa, jota et täysin hallitse (jaettu salasanojen hallinta, pilvimuistiinpanot, tuloste toimistolla); omassa kassakaapissasi oleva paketti on jo kassakaapin suojaama.

    Kun salaus on päällä eikä käyttökelpoista vastaanottajaa ole määritetty, lataus torjutaan suoraan. BombVault ei koskaan turvaudu antamaan pääavainta selkotekstinä.

### Kun paketti ei ole käsillä

Salasanaa ei ole tallennettu minnekään, se **lasketaan** varmuuskopioiden avaimesta, joten voit toistaa sen itse pelkällä avaimella ja komentotulkilla:

```sh
printf 'bombvault:restic-repo' \
  | openssl dgst -sha256 -mac HMAC -macopt hexkey:$APP_KEY -r \
  | cut -d' ' -f1
```

Se on HMAC-SHA256 kiinteälle merkkijonolle `bombvault:restic-repo`, avaimena heksa-`APP_KEY`:n raakatavut, tulostettuna 64 pienenä heksamerkkinä. Sama arvo on paketissa, listattuna johdettuna restic-salasanana; tämä on sitä päivää varten, kun paketti on jossain, missä et ole. Palvelimella, joka otti haltuunsa toisen varmuuskopiot, käytä haltuun otettua varmuuskopioavainta, joka on kyseisen palvelimen vanha APP_KEY.

!!! warning "Vastaanotetussa arkistossa käytä LÄHETTÄVÄN instanssin avainta"
    Repository, joka saapui tänne etäkopion kautta, on sen lähettäneen koneen luoma **sen** avaimella. Johtaminen vastaanottavan laatikon avaimesta tuottaa salasanan, jonka restic hylkää, mikä näyttää täsmälleen vioittuneelta repositorylta eikä ole sitä. Tämä on tavallinen syy siihen, että `restic check` vastaanotetulla repolla kysyy salasanaa yhä uudelleen.

Koska palautusmääritykset asuvat kunkin repon **sisällä** (`<repo>/def`, `<repo>/vm-def`), kopioitu repokansio on täysin itsenäinen, joten paketti plus repo on kaikki mitä paljasrautainen palautus tarvitsee.

## Tietokantavedoksen hakeminen takaisin {#database-dumps}

Tietokantavedos on oma palautuspisteensä konttien repositoryssä, tunnisteella `dbdump:<kontti>`, ja sisältää yhden tiedoston `/dbdump/<kontti>.sql`. BombVault listaa, lataa ja tuo ne kontin **Varmuuskopiot**-välilehdellä; alla ovat samat vaiheet pelkällä resticillä sitä päivää varten, kun BombVaultia ei ole.

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
