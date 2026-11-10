# Etäsijainti ja palautus

!!! note "Etäkopiot odottavat uudelleenrakennuksen jälkeen"
    Kun Hätäohjattu toiminto tai **Tunnista varmuuskopiot** rakentaa merkintöjä uudelleen ilman vanhoja asetuksia, näiden osioiden etäkopiot odottavat, kunnes otat niiden varmuuskopiot tämän palvelimen omiksi. Katso [Uudelleenrakennuksen jälkeen](#after-a-rebuild).

Paikalliset varmuuskopiot suojaavat kadonneelta kontilta tai huonolta päivitykseltä. Etäkopiot ja testattu palautuspaketti suojaavat koko laatikon menetykseltä, kiristysohjelmalta tai tulipalolta. Tämä sivu käsittelee kohteita ja etäkopioita, kopion tekemistä peukalointia kestäväksi, palautuskyvyn todistamista, usean palvelimen pariliitosta ja kaiken palauttamista, kun itse BombVaultia ei enää ole.

## Etäkopiot {#off-site-copies}

Pidä nopea paikallinen varmuuskopio ja lisää yksi tai useampi kohde tilojen ulkopuolelle. BombVault kopioi uudet varmuuskopiot sinne `restic copy` -komennolla parhaan yrityksen periaatteella, joten etäkohteen nikottelu ei koskaan kaada paikallista varmuuskopiota. Tässä muodossa kukin osio kirjoittaa ensin omaan kansioonsa ja etärepo on kopio. Oman kansion ei tarvitse olla tällä palvelimella: katso [Osion oma kansio](#own-folder), jos haluat pitää sitä kohteessa, kuten S3:ssa tai rest-serverissä.

- **Useita kohteita osiota kohti.** Jokainen osio (kontit, VM:t, flash, itsevarmuuskopio, kansiot ja ZFS-tietojoukot) voi kopioida useaan etäkohteeseen kerralla, joten voit pitää rinnakkain esimerkiksi rest-serverin ystävän laatikossa ja S3-bucketin. Kohteet-sivun kortti **Kohteet osioittain** kytkee jokaisen osiosta kohteeseen vievän rivin päälle tai pois.
- **Milloin kopiot ajetaan.** Kohteen **Kopioi**-kortti asettaa kohdan **Milloin kopioidaan**: **Jokaisen varmuuskopion jälkeen** tai oma aikataulu, joka kerää varmuuskopiot ja kopioi kiinteinä aikoina. Kohdassa **Asetukset, Aikataulu, Etäkopiot osioittain** **Poikkeus** antaa yhdelle osiolle oman kopiointiaikataulun.
- **Etäkopiot-kortti.** Kohdassa **Asetukset, Kohteet** jokaisella osiolla, jolla on etäkopio, on rivi kortissa **Etäkopiot**. Siinä ovat **Testaa yhteys**, **Replikoi nyt** kopioinnin käynnistämiseen pyydettäessä sekä **Määritä** tai **Muokkaa**. Osion muut kopioasetukset avautuvat sieltä sen omaan ikkunaan.
- **Säilytys** asetetaan kohdekohtaisesti kohdassa **Kuinka kauan säilytetään**, joten etäkopioita voi pitää pidempään arkistona. Osion etäkentän kopio noudattaa sitä kuten kaikki muutkin kopiot, ja osio voi pitää omat arvonsa kohdassa **Säännöt osioittain**. Kaikki nollilla ei koskaan karsi etävarmuuskopioita. Katso [Kuinka kauan säilytetään](#keep).
- **Kaistanleveyden rajat** (**Etäkaistanleveys** kohdassa **Asetukset, Kohteet** tai kohteen **Kopioi**-kortti) rajoittavat resticin lähetys- ja latausnopeutta, jotta kopiot eivät tuki WAN-yhteyttäsi. Ne annetaan yksikössä KiB/s, jota restic käyttää, ja jokainen kenttä näyttää saman nopeuden myös yksiköissä MB/s ja Mbit/s kirjoittaessasi: 2048 näkyy noin 2,0 MB/s (16,8 Mbit/s).
- **Pakkaus** on **Pois**, **Automaattinen** (resticin oletus) tai **Maksimi**. Se asetetaan jokaiselle kohteelle ja yhdelle osiolle sen säännöissä siellä. Varmuuskopiot, kopiot ja karsinta kirjoittavat sillä, ja palautuspaketti nimeää sen, joten tavallinen restic voi jatkaa kirjoittamista samalla tavalla.
- **Suoratoisto ensin.** Kun mediapalvelin, kuten Plex, Jellyfin tai Emby, suoratoistaa, kopiot lähetetään alemmalla rajalla ja palaavat normaaliin muutaman minuutin kuluttua suoratoiston päättymisestä. Kohteet-sivun kortissa **Suoratoisto** ovat **Hidasta etäkopioita suoratoiston aikana**, **Mediapalvelimet** (kontit, joiden image-nimessä on Plex, Jellyfin tai Emby, kunnes valitset itse), **Suoratoisto yli (Mbit/s)**, **Lähetysraja suoratoiston aikana (KiB/s)** ja **Takaisin normaaliin (min) jälkeen**. BombVault lukee liikenteen Dockerista, joten isäntäverkossa olevaa mediapalvelinta ei voi mitata. REST, S3, B2, Azure, Google Cloud, Swift ja rclone HTTP:n yli hidastuvat kesken kopion. SFTP sekä paikalliset tai liitetyt kansiot ottavat alemman rajan käyttöön seuraavassa kopiointivaiheessa.
- **Kylmätallennus (S3).** S3-kohde voi käyttää edullisempaa **Tallennusluokkaa**, jonka restic pystyy yhä lukemaan heti, kuten Standard-IA, One Zone-IA, Intelligent-Tiering tai Glacier Instant Retrieval. Glacier Flexible Retrieval ja Deep Archive on jätetty pois, koska restic ei voi palauttaa niistä. Rclone-etäsijainti asettaa luokkansa rclone-määrityksessä.
- **Samanaikaiset yhteydet.** S3-, B2-, Azure-, Google Cloud -, Swift-, rest-server-, SFTP- tai rclone-kohteessa sen **Yhteys**-kortin **Samanaikaiset yhteydet** määrää, kuinka monta yhteyttä restic avaa kerralla, 1:stä 32:een. Tyhjänä restic pitää oletuksensa 5. Useampi yhteys voi nopeuttaa kaukaista tai hidasta linjaa, harvempi säästää palveluntarjoajaa, joka rajoittaa nopeutta. Kaikki kohteen varmuuskopiot, kopiot, tarkistukset, karsinnat ja palautukset käyttävät sitä, ja asetusten vienti kuljettaa sen mukana. Paikallisella tai liitetyllä kansiolla ei ole tällaista asetusta.
- **Kasvubudjetti.** **Kasvubudjetti (GB, 0 = pois)** osion kopioikkunassa antaa hälytyksen, kun arkisto kasvaa yli sen koon. Se ei koskaan poista mitään, joten se sopii append-only-kopiolle, jota täältä ei karsita.
- Kun kopio on käynnissä, yleiskatsauksen tilarivi kertoo **Etäkopiointi käynnissä**. Se on aktiivisuusilmaisin, ei prosenttipalkki, koska `restic copy` ei tarjoa koneluettavaa etenemistietoa.

!!! note "Palautus mistä tahansa paikasta"
    Jokainen merkintä listaa varmuuskopionsa yhtenä aikajanana kaikkien paikkojen yli, joissa varmuuskopio on, **Varmuuskopiot**-välilehdellään. B2:een kopioitu varmuuskopio näkyy kerran, merkittynä jokaisella paikalla, joka sen pitää. Palautus ottaa ensimmäisen paikan, johon se pääsee, aloittaen arkistosta, johon merkintä kirjoitetaan, ja voit valita toisen paikan varmuuskopiokohtaisesti. Etäkohteet luetaan vasta, kun napsautat **Tarkistus**, koska pilvitallennuksen listaaminen kestää. Poistaminen yhdessä paikassa tarkistaa ensin muut ja kertoo, oliko se viimeinen kopio. Katso [Palautus](restore.md).

## Kohteet {#destinations}

Kohde on paikka, johon varmuuskopiot menevät: paikallinen kansio, jako, oma palvelin tai pilvipalvelu. Se määritetään kerran ja palvelee jokaista osiota. Avaa **Kohteet** sen asetusruudusta tai kytke siellä päälle **Näytä myös sivupalkissa** kiinnittääksesi sen.

![Kohteet-sivu: kohteiden luettelo ja virta, joka näyttää, mikä osio menee minne.](assets/screenshots/storage/locations-dark.png#only-dark)
![Kohteet-sivu: kohteiden luettelo ja virta, joka näyttää, mikä osio menee minne.](assets/screenshots/storage/locations-light.png#only-light)

**Lisää kohde** avaa ikkunan neljässä vaiheessa:

1. **Palveluntarjoaja.** Jokainen palvelu on listattu logoineen, hakukenttä ikkunan oikeassa yläkulmassa, ryhmissä: **Paikallinen** tähän palvelimeen liitetylle levylle tai jaolle, **NAS tai palvelin** (rest-server, Hetzner Storage Box, SFTP, SMB, WebDAV ja muut), **S3-palvelu** (Backblaze B2, Wasabi, Cloudflare R2, Hetzner Object Storage, Amazon S3 ja muut), oma S3-palvelimesi (Garage, SeaweedFS, RustFS, Silo, Ceph, JuiceFS, Versity S3 Gateway), **Pilvitallennus** (OneDrive, Google Drive, Dropbox, pCloud, Nextcloud ja muut, joita rclone tukee) sekä **Ryhmästäsi**, eli ryhmäsi vastaanottopalvelimet. Jokainen kertoo, kuinka hyvin se sopii varmuuskopioille: pilvilevyt hidastuvat monien pyyntöjen alla, joten ensimmäinen varmuuskopio ja karsinta kestävät siellä kauemmin. Palvelulle, jota mikään ruutu ei kata, kohdan **Oma rclone-määritys** ruutu **Rclone** ottaa vastaan rclone.conf-tiedostosta liitetyt etäsijainnit.
2. **Kirjaudu sisään.** Kentät riippuvat palvelusta: S3:lle käyttöavain, WebDAV:lle ja SMB:lle käyttäjä ja salasana, sovellussalasana, kun kaksivaiheinen kirjautuminen estää tavallisen, BombVaultin julkinen SSH-avain SFTP:lle ja Storage Boxille tai token palveluille, jotka kirjautuvat selaimen kautta. Niille ikkuna näyttää `rclone authorize` -komennon, joka ajetaan tietokoneella, jossa on selain, ja sen tulostama token liitetään kenttään. Jos palvelulle on jo [tallennetut tunnukset](#sign-ins), **Tallennetut tunnukset** valitsee ne kohdan **Uudet avaimet** sijaan. **Testaa yhteys** tarkistaa kirjautumisen ennen kuin mitään tallennetaan. Ikkuna listaa sitten kohteen kansiot, **Uusi kansio** -toiminnolla uuden luomiseen ja vapaalla tilalla, kun palvelu sen raportoi. Tyhjä kansio on turvallisin. BombVault luo siihen yhden kansion osiota kohti.
3. **Suojaus.** Ikkuna kertoo suoraan, mihin palvelu pystyy. Rest-server append-only-tilassa kieltäytyy poistosta, ja peukalointitesti tarkistaa sen. S3-bucket voi säilyttää vanhat versiot versioinnilla ja objektilukolla, mitä BombVault ei vielä voi tarkistaa. Pilvilevy ei voi kieltäytyä poistosta lainkaan: kuka tahansa, joka pääsee palvelimelle, pääsee myös tuohon kopioon. Kytke **Poistosuoja (append-only)** päälle vain siellä, missä vastapuoli todella kieltäytyy poistosta. BombVault ei sen jälkeen koskaan karsi siellä. **Hätätilanteen varalle** kertoo, mitä palautuspaketti sisältää tästä kohteesta.
4. **Mille osioille.** Valitse osiot, jotka varmuuskopioivat tänne. Ensimmäinen kopio tuo koko historian mukanaan. Merkinnät, jotka valitsivat omat kohteensa, eivät saa uutta itsestään, ja ikkuna nimeää ne.

![Lisää kohde: ensimmäinen vaihe, jossa jokainen palveluntarjoaja on ruutuna.](assets/screenshots/storage/add-providers-dark.png#only-dark)
![Lisää kohde: ensimmäinen vaihe, jossa jokainen palveluntarjoaja on ruutuna.](assets/screenshots/storage/add-providers-light.png#only-light)

![Lisää kohde: kirjautumisvaihe onnistuneen testin jälkeen.](assets/screenshots/storage/add-signin-dark.png#only-dark)
![Lisää kohde: kirjautumisvaihe onnistuneen testin jälkeen.](assets/screenshots/storage/add-signin-light.png#only-light)

Jotkin palvelut kysyvät kirjautuessa hieman enemmän:

- **SFTP ja Storage Box.** **Lisää avain** vie BombVaultin julkisen avaimen palvelimelle kentän **Tilin salasana** avulla. Salasanaa käytetään vain tuohon yhteen kirjautumiseen eikä sitä tallenneta mihinkään. Sen alla näkyy komento, jolla saman voi tehdä käsin. Testi näyttää kohdan **Palvelimen avain** sormenjälkineen. BombVault muistaa sen ja hylkää palvelimen, jos se myöhemmin näyttää eri avaimen.
- **Yhden bucketin S3-avaimet**, kuten Cloudflare R2:n objektitoken tai rajattu AWS-avain, eivät välttämättä saa listata muita bucketteja. Kirjoita bucket kenttään **Bucket**, niin testi ja kansioluettelo avaavat juuri sen bucketin.
- **S3:n alueet ja osoitteet.** Alue luetaan päätepisteestä, joka nimeää sen (AWS, Backblaze B2, Hetzner, Scaleway, OVH, Wasabi). Garage aloittaa omalla oletusalueellaan, ja IDrive e2 pyytää päätepistettä, jonka sen hallintapaneeli näyttää. Osoite, joka liitetään bucket-nimen kanssa niin kuin R2:n hallintapaneeli sen näyttää, täyttää palvelimen ja bucketin. Kansioselaimesta luotu bucket menee lomakkeessa nimettyyn alueeseen.
- **Tallennusluokka.** Valitsin tarjoaa vain luokat, jotka palvelu ottaa vastaan, ja **Lisäasetukset** näyttää vain uuteen bucketiin vaikuttavat asetukset.
- **Google Drive ja OneDrive.** Karsittu data poistetaan lopullisesti sen sijaan, että se menisi roskakoriin, jossa se kuluttaisi yhä kiintiötä. Googlen jaettu asema asetetaan kohdassa **Lisäasetukset**.
- **SMB-palvelimet, jotka eivät listaa jakojaan,** saavat yhden jaon nimen kenttään **Jako**, ja testi avaa sen jaon suoraan.
- **FTP** kysyy kohdassa **Salaus**: **Ei mitään**, **Eksplisiittinen TLS (FTPES)** tai **Implisiittinen TLS (FTPS, portti 990)**.
- **OneDrive ja Zoho WorkDrive.** Kun kirjautuminen avaa useita asemia tai työtiloja, ikkuna kysyy, mihin niistä varmuuskopiot menevät.
- **Rest-server, jolla on itse allekirjoitettu varmenne.** Laita varmenne PEM-tiedostona BombVaultin datakansion kansioon `certs` (`/config/certs` kontissa, `appdata/bombvault/certs` Unraidissa). Varmuuskopiot, kopiot ja poistosuojan testi luottavat siihen ilman uudelleenkäynnistystä.

S3-palvelut kulkevat resticin oman S3-taustajärjestelmän kautta, mikä mahdollistaa tallennusluokan ja objektilukon. Restic tavoittaa rest-serverin suoraan. Kaikki muut palvelut kulkevat BombVaultin mukana tulevan rclonen kautta, ja niiden osa rclone-määrityksestä näkyy kohteen sivun kortissa **rclone.conf**, salaisuudet piilotettuina. Asetusten vienti sisältää kohteet, ja tunnistetiedot mukaan otettuina se sisältää myös niiden tunnukset.

Kun tallennat kohteen, jossa on jo konttien, VM:ien tai kansioiden varmuuskopioita, esimerkiksi poistamasi ja uudelleen lisäämäsi kohteen, se lisätään näihin osioihin ruksaamattomana kopiona. Mitään ei kopioida sinne, ennen kuin ruksaat sen, mutta **Palautus** tarjoaa vanhat varmuuskopiot heti.

Tämän palvelimen kansio on kaikkialla nimeltään **Paikallinen**: kohteiden sivuilla, virrassa, merkinnässä ja Palautus-sivulla. Jos tällä palvelimella on useita kansioita, kullakin on oma nimensä. Kohde, jonka osion etäkenttä luo, nimetään paikkansa mukaan eli osoitteensa viimeisen kansion mukaan tai, jos kansiota ei ole, palvelimensa mukaan, ja se saa numeron, jos jokin toinen kohde on jo samanniminen. Jokaisen kohteen voi nimetä uudelleen kentässä **Nimi** sen **Yhteys**-kortissa, myös bucket-kansion, jota useat osiot jakavat, ja kopion, jonka osion etäkenttä kuvaa, ja antamasi nimi säilyy. Kun kaksi samannimistä kohdetta on rinnakkain, kummallakin on mukana palvelimensa tai kansionsa.

### Kohteen sivu {#location-page}

Jokaisella kohteella on oma sivunsa. Se näyttää vain kortit, jotka koskevat kyseistä kohdetta, tässä järjestyksessä:

![Kohteen sivu: Omat kansiot, Yhteys ja Tunnistetiedot.](assets/screenshots/storage/location-page-dark.png#only-dark)
![Kohteen sivu: Omat kansiot, Yhteys ja Tunnistetiedot.](assets/screenshots/storage/location-page-light.png#only-light)

- **Omat kansiot**: osiot, jotka kirjoittavat varmuuskopionsa ensin tänne. Tämän kohteen tunnukset, suojaus ja rajat koskevat niitä. **Käytä omana kansiona** siirtää useamman osion oman kansion tänne, ja **Vaihda** siirtää yhden pois. Katso [Osion oma kansio](#own-folder).
- **Varmuuskopioidaan suoraan tänne**: jokainen osio, jonka merkinnät varmuuskopioivat suoraan tähän paikkaan oman kansionsa sijaan, ja kuinka moni merkintä niin tekee. Tällainen suora arkisto ottaa kaikki asetuksensa tältä kohteelta. Sellainen, jolle mikään ei enää varmuuskopioi, kertoo sen, ja **Poista** ottaa sen pois luettelosta. Varmuuskopiot sen paikalla säilyvät. Katso [Suorat arkistot](#direct-repositories).
- **Yhteys**: **Nimi**, kytkin **Käytettävissä** (pois tarkoittaa, ettei BombVault enää kirjoita tänne, ja merkinnät, jotka varmuuskopioivat tänne, epäonnistuvat selkeällä viestillä sen sijaan, että päätyisivät muualle), kansio, **Pakkaus**, **Tallennusluokka**, **Samanaikaiset yhteydet** ja **Toimipaikan ulkopuolella**. **Paikallinen**-kohteessa se listaa kunkin osion kansion kohdassa **Omat kansiot täällä**. **Muuta yhteyttä** testaa ja tallentaa uuden osoitteen tai kirjautumisen. Kun muut paikat kirjautuvat samoilla tallennetuilla tunnuksilla, ikkuna kertoo sen, ja muutos tässä antaa tälle kohteelle omat tunnukset jaettujen uudelleenkirjoittamisen sijaan.
- **Tunnistetiedot**: mitkä tallennetut tunnukset se käyttää (**Kirjautuu tunnuksilla**), mitkä muut kohteet käyttävät niitä myös (**Käyttää myös**) ja **Muokkaa**, joka avaa tunnusten ikkunan. Katso [Tunnukset](#sign-ins).
- **rclone.conf**: tämän kohteen osa rclone-määrityksestä, kohteelle, joka kulkee rclonen kautta.
- **Poistosuoja**, jossa **Poistosuoja (append-only)**, **Hyväksy ilman poistosuojaa**, **Testaa poistosuoja nyt** ja **Kuka karsii**, joka kertoo, karsiiko BombVault kopion ja omat kansiot täällä, joko tämän kohteen kortin **Kuinka kauan säilytetään** tai osion omien arvojen mukaan kohdassa **Säännöt osioittain**, vai jättääkö se sen vastapuolelle. Paikallinen kansio näyttää tämän kortin vain, kun sillä on suoja, jonka voi kytkeä pois. Katso [Poistosuoja](#delete-protection-append-only).
- **Kopioi**: **Milloin kopioidaan** sekä **Lähetysraja (KiB/s)** ja **Latausraja (KiB/s)**. Kohde, jossa on vain omia kansioita eikä kopioita, näyttää rajat kortissa nimeltä **Kaistanleveys**.
- **Kuinka kauan säilytetään**: **Lyhyt**, **Tasapainoinen**, **Pitkä** tai **Omat arvot**, aikajanalla siitä, mitkä varmuuskopiot sääntö säilyttää. Katso [Kuinka kauan säilytetään](#keep).
- **Jo tämän kohteen alla**: osiot, joiden käsin kirjoitettu osoite on tämän kohteen kansiossa. Haltuunotto jättää arkiston paikalleen. Tunnukset, tallennusluokka ja poistosuoja tulevat sen jälkeen kohteelta. BombVault tarkistaa ensin, että kohteen tunnukset avaavat arkiston, ja kieltäytyy asettamasta append-only-arkistoa kohteen alle, joka ei ole append-only.
- **Säännöt osioittain**: yhden osion oma arkisto, säilytys tai pakkaus tässä kohteessa. **Muuta sääntöjä** avaa ne, ja **Palauta kohteen asetukset** tuo osion takaisin. Tunnukset, kaistanleveys ja kopiointiajat tulevat aina kohteelta.
- Astia, joka näyttää, kuinka täynnä kohde on, merkillä täyttymiskohdasta ja viikkojen määrällä, jotka jäljellä ovat viime viikkojen kasvuvauhdilla. Kohde, joka ei raportoi kokoaan, sanoo **Koko tuntematon**.

**Siirrä** ja **Poista kohde** ovat sivun toimintojen joukossa, ja ne on kuvattu kohdassa [Siirtäminen ja poistaminen](#move-remove).

Etäkohde, jossa oli varmuuskopioita ja jossa yhtäkkiä ei ole arkistoa, esimerkiksi rest-server, jonka levy ei liittynyt, ei saa itsestään uutta, tyhjää arkistoa. Sen sivu kertoo siitä heti, kun varmuuskopio tai kopio huomaa arkiston kadonneen. Liitä levy uudelleen. Jos tyhjensit vanhan arkiston tarkoituksella, **Luo tähän uusi arkisto** aloittaa alusta, kaikille kohteen osioille ja sen arkistoille kerralla. Suoja koskee kohdetta, joka on pysynyt paikallaan. Lisäämäsi kohde tai osio, jonka valitset kohteessa, aloittaa puhtaalta pöydältä: jos osoitteessa ei ole arkistoa, ensimmäinen kopio luo sen, vaikka aiemmin poistetulla kohteella olisi ollut siellä arkisto.

Kaksi hylkäystä kertoo, mitä pitää korjata, kopion ajossa ja kohteen sivulla. **Tätä bucketia ei ole palvelimella** tulee heti, kun S3-palvelin ilmoittaa niin, eikä BombVault luo sinne mitään, joten väärin kirjoitettu bucketin nimi pysyy näkyvissä. Palvelin, joka ei vastaa lainkaan, on eri tapaus: restic yrittää uudelleen noin 15 minuuttia kutsua kohti siltä varalta, että palvelin palaa. **Täällä on jo restic-arkisto, eikä tämän instanssin avain avaa sitä** tarkoittaa, että siellä olevat varmuuskopiot kuuluvat toiselle `APP_KEY`:lle.

Ryhmäsi toisen instanssin ajama vastaanottopalvelin näkyy palveluntarjoajien joukossa kohdassa **Ryhmästäsi**. Katso [Vastaanottopalvelin](#receiving-server).

BombVault tarkistaa kohteensa jokaisessa käynnistyksessä. Se korjaa sen, mikä ei vaadi päätöstäsi, kuten linkin kadonneeseen kopioon tai arkistoon, kopion osion omassa kansiossa tai suoran arkiston, jonka kopio on kadonnut, ja kirjoittaa lokiin rivin jokaisesta korjauksesta. Se, minkä turvallista korjausta ei ole, vain kirjataan lokiin.

![Sama sivu puhelimella.](assets/screenshots/storage/location-page-phone-dark.png#only-dark)
![Sama sivu puhelimella.](assets/screenshots/storage/location-page-phone-light.png#only-light)

### Kuinka kauan säilytetään {#keep}

Se, kuinka kauan varmuuskopioita säilytetään, asetetaan kohdekohtaisesti sen kortissa **Kuinka kauan säilytetään**. Sääntö koskee kyseisen kohteen kopioita ja siellä olevia omia kansioita, myös **Paikallinen**-kohdetta. Osio voi pitää omat arvonsa yhdessä kohteessa kohdassa **Säännöt osioittain**, ja ne voittavat kohteen kortin kyseisen osion osalta.

Sääntö säilyttää yhden varmuuskopion päivää, viikkoa, kuukautta ja vuotta kohti niin kauas taaksepäin kuin asetat ja siivoaa loput jokaisen varmuuskopion tai kopion jälkeen:

| Esiasetus | Viimeisimmät | Päivät | Viikot | Kuukaudet | Vuodet |
|---|---|---|---|---|---|
| **Lyhyt** | 0 | 7 | 4 | 3 | 0 |
| **Tasapainoinen** | 0 | 7 | 4 | 6 | 2 |
| **Pitkä** | 0 | 14 | 8 | 12 | 3 |

**Omat arvot** antaa asettaa viisi lukua itse. 0 kytkee kyseisen luvun pois, ja sääntö, jossa kaikki luvut ovat 0, säilyttää kaiken. Kortin pisteet näyttävät, mitkä 40 esimerkkivarmuuskopiosta sääntö säilyttää, uusin aivan oikealla. Esimerkki on sama jokaiselle kohteelle, koska kuva piirretään ennen kuin BombVault on lukenut siellä olevat varmuuskopiot. Se laskee taaksepäin tästä päivästä, joten jäljelle jäävien pisteiden määrä vaihtelee päivämäärän mukaan.

Myös tämän isännän kansiolla ja käsin kirjoitetulla etäkansiolla on oma korttinsa, joka koskee juuri sen kansion omia kansioita eikä muita. Kunnes tallennat sen, tällainen kansio noudattaa sääntöä, joka BombVaultilla jo oli sille.

**Kysy ennen vähemmän säilyttämistä** on oletuksena päällä. Se kysyy ennen kuin muutos, joka säilyttää vähemmän varmuuskopioita, tallennetaan, ja kertoo, kuinka monta 40 esimerkkivarmuuskopiosta uusi sääntö poistaa. Se koskee jokaista kohdetta. **Palauta kohteen asetukset** kohdassa **Säännöt osioittain** kysyy saman kysymyksen.

Kohdetta, jolla on poistosuoja, ei koskaan karsita tältä laatikolta. Sen vastapuoli säilyttää tai pudottaa vanhat varmuuskopiot omien sääntöjensä mukaan. Kun merkinnällä on avoin kriittinen poikkeama kadonneesta datasta, säilytys pitää kyseisen merkinnän vanhat varmuuskopiot, kunnes kuittaat sen. Katso [Poikkeamat](anomalies.md).

Varmuuskopio, jossa on vähemmän kuin pitäisi, ei koskaan työnnä aiempia pois. Sellainen on varmuuskopio, joka ei lue yhtään tiedostoa, vaikka aiemmissa oli tiedostoja, josta puuttuu valittu kansio tai joka ei pystynyt lukemaan joitakin tiedostoja. Sen ajo päättyy varoitukseen, joka nimeää puuttuvan ja kertoo, että vanhemmat varmuuskopiot säilytettiin. Saat saman viestin ilmoituskanavillesi, eikä mikään karsinta merkinnän omassa kansiossa tai sen kopioissa koske merkinnän vanhoihin varmuuskopioihin, ennen kuin täydellinen varmuuskopio on mennyt läpi. Ensimmäinen varmuuskopio ja valittu kansio, joka on aina ollut tyhjä, lasketaan täydellisiksi.

Nähdäksesi, mitä seuraava ajo todella poistaisi, avaa **Esikatselu** kohdassa **Asetukset, Kohteet**. Se listaa kohdan **Mitä seuraava ajo poistaisi** osion omalle kansiolle ja jokaiselle sen kopiolle arkistoittain ja merkinnöittäin palautuspisteet nimettyinä. Se ei ota lukkoa eikä muuta mitään.

![Kuinka kauan säilytetään, Tasapainoinen valittuna: pisteet näyttävät, mitkä 40 esimerkkivarmuuskopiosta jäävät.](assets/screenshots/storage/keep-dark.png#only-dark)
![Kuinka kauan säilytetään, Tasapainoinen valittuna: pisteet näyttävät, mitkä 40 esimerkkivarmuuskopiosta jäävät.](assets/screenshots/storage/keep-light.png#only-light)

#### Esimerkki: mitä Tasapainoinen säilyttää {#example-retention}

Kontista on otettu varmuuskopio joka yö kello 03.00 1. lokakuuta 2024 lähtien, joten keskiviikkona 4. marraskuuta 2026 sillä on 765 varmuuskopiota. Sen kohteeksi on asetettu **Tasapainoinen**: 7 päivää, 4 viikkoa, 6 kuukautta ja 2 vuotta.

Jokainen luku säilyttää päivänsä, viikkonsa, kuukautensa tai vuotensa uusimman varmuuskopion, ja varmuuskopio, jonka useampi luku valitsee, säilytetään kerran. Viikot kulkevat maanantaista sunnuntaihin:

| Sääntö | Varmuuskopiot, jotka se valitsee | Uudet, jotka se lisää |
|---|---|---|
| 7 päivää | 4.11., 3.11., 2.11., 1.11., 31.10., 30.10., 29.10. | kaikki 7 |
| 4 viikkoa | 4.11., 1.11., 25.10., 18.10. | 25.10., 18.10. |
| 6 kuukautta | 4.11., 31.10., 30.9., 31.8., 31.7., 30.6. | 30.9., 31.8., 31.7., 30.6. |
| 2 vuotta | 4.11., 31.12.2025 | 31.12.2025 |

Jäljelle jää 14 varmuuskopiota 765:stä. Vanhin on päivältä 31. joulukuuta 2025, joten tänään pääset taaksepäin hieman yli kymmenen kuukautta. Deduplikoinnin ansiosta ne vievät vain vähän enemmän tilaa kuin yksi täysi varmuuskopio ja siinä välillä muuttunut data.

**Vuodet** ansaitsee toisen katseen. Se laskee kalenterivuodet, joissa on varmuuskopio, nykyisestä alkaen, ja tämän vuoden uusin varmuuskopio on tämän päivän. Ensimmäinen vuosi ei siis lisää mitään muihin lukuihin, ja toinen lisää edellisen vuoden viimeisen varmuuskopion. Siksi säännön ulottuvuus vaihtelee vuoden mittaan. Samat varmuuskopiot kullakin esiasetuksella:

| Esiasetus | Tänään säilytetyt varmuuskopiot | Vanhin varmuuskopio tänään | Ulottuvuus vuoden mittaan |
|---|---|---|---|
| **Lyhyt** | 10 | 30. syyskuuta 2026 | 1 tai 2 kuukautta |
| **Tasapainoinen** | 14 | 31. joulukuuta 2025 | noin 4 kuukautta alkukeväällä, koko vuosi 31. joulukuuta |
| **Pitkä** | 30 | 31. joulukuuta 2024 | 1 vuosi 1. tammikuuta, 2 vuotta 31. joulukuuta |

Siirtyminen arvosta **Tasapainoinen** arvoon **Lyhyt** kysyy ensin, koska **Kysy ennen vähemmän säilyttämistä** on päällä. Kysymys kertoo, kuinka monta kuvan 40 varmuuskopiosta uusi sääntö poistaa. Luku tulee esimerkistä, joten se vaihtelee päivämäärän mukaan eikä laske omia varmuuskopioitasi. Tämän esimerkin varmuuskopioista 31. elokuuta, 31. heinäkuuta, 30. kesäkuuta ja 31. joulukuuta 2025 poistuisivat. **Tallenna silti** ottaa säännön käyttöön, ja seuraava karsinta poistaa ne kyseisestä kohteesta lopullisesti.

Jos haluat säilyttää konttien varmuuskopioita pidempään kuin muiden tässä kohteessa, paina **Muuta sääntöjä** kohdassa **Säännöt osioittain**, valitse **Kontit** ja aseta **Omat arvot** kohdassa **Säännöt osiolle Kontit**, esimerkiksi 14 päivää, 8 viikkoa, 12 kuukautta ja 2 vuotta. Muut osiot seuraavat yhä kohteen korttia, ja **Palauta kohteen asetukset** tuo Kontit takaisin.

### Tunnukset {#sign-ins}

Tunnukset ovat avain tai salasana, jota BombVault käyttää S3-palvelussa tai rest-serverissä. Tunnukset ovat yksi luettelo, kortti **Tunnukset** kohdassa **Asetukset, Kohteet**, ja jokainen rivi kertoo, mitkä kohteet sitä käyttävät (**Käyttää**).

- **Tallennetaan kohdetta lisättäessä.** Kohteen lisääminen tallentaa kirjoittamasi avaimet tunnuksiksi. Seuraava kohde samassa palvelussa voi valita ne kohdassa **Tallennetut tunnukset** kohdan **Uudet avaimet** sijaan, ja myöhempi muutos niihin koskee jokaista kohdetta, joka niitä käyttää.
- **Oletus.** Tähti merkitsee oletustunnukset. Niitä käytetään kaikkialla, missä tunnuksia ei ole valittu, esimerkiksi käsin kirjoitetussa osoitteessa tai palautuksessa uudella asennuksella. Noutolähde ei lainaa niitä. **Aseta oletukseksi** antaa tähden toisille.
- **Muokkaa ja Testaa.** **Muokkaa**, täällä tai kohteen kortissa **Tunnistetiedot**, avaa tunnuksille yhden ikkunan. **Testaa** kirjautuu jokaiseen niitä käyttävään kohteeseen kirjoittamillasi tiedoilla ennen kuin mitään tallennetaan. **Tallenna** tallentaa ne.
- **Poistaminen.** Tunnukset voi poistaa vain, kun mikään kohde, noutolähde tai vastaanotettu repository ei käytä niitä eikä mikään vanha varmuuskopio tarvitse niitä, ja oletus poistuu vasta viimeisenä. Vanhat varmuuskopiot tarvitsevat niitä, kun oma kansio siirtyi pois paikasta, joka kirjautuu niillä, ja jätti varmuuskopiot sinne. BombVault kertoo silloin, mikä paikka on kyseessä.
- **Kohteen yhteyden muuttaminen** ei koskaan kirjoita uudelleen tunnuksia, joita muut paikat jakavat. Kohde saa sen sijaan omat tunnukset, ja ikkuna kertoo etukäteen, että sen tunnukset ovat jaetut. Muuttaaksesi jaettuja kaikkialla muokkaa niitä kortissa **Tunnukset**.

![Tunnukset-kortti: jokainen tallennettu tunnus ja se, mikä niitä käyttää, sekä tähti oletuksella.](assets/screenshots/storage/signins-dark.png#only-dark)
![Tunnukset-kortti: jokainen tallennettu tunnus ja se, mikä niitä käyttää, sekä tähti oletuksella.](assets/screenshots/storage/signins-light.png#only-light)

Tämän palvelimen kansio ei tarvitse tunnuksia. SMB, WebDAV, SFTP, Storage Box ja pilvilevyt pitävät omansa rclone-määrityksessä, jonka kohteen kortti **rclone.conf** näyttää, joten ne eivät näy tässä luettelossa.

### SMB, WebDAV ja muut rclone-etäsijainnit {#smb-webdav}

Windows- tai Samba-jako ja WebDAV-palvelin (Nextcloud, ownCloud, SharePoint tai mikä tahansa muu) ovat palveluntarjoajien ruutuja kohdassa **Lisää kohde**, ryhmässä **NAS tai palvelin**. Täytä isäntä ja jako (SMB) tai osoite ja palvelintyyppi (WebDAV), käyttäjä ja salasana, niin BombVault kirjoittaa rclone-osion puolestasi. rclone hämärtää salasanan ennen tallennusta.

Palvelulle, jota mikään ruutu ei kata, kohdan **Oma rclone-määritys** ruutu **Rclone** ottaa vastaan rclone.conf-tiedostosta liitetyt etäsijainnit. Ne lisätään niihin, jotka BombVaultilla jo on, eivätkä koskaan korvaa niitä, ja ne tallennetaan salattuina. Varmuuskopioidaksesi osion johonkin niistä syötä `rclone:<etäsijainti>:<kansio>` kyseisen osion etäarkistoksi, esimerkiksi `rclone:nas:backups/bombvault`. Jako tai bucket on polun ensimmäinen segmentti, ei osa nimeä.

Tämä on parempi tie kuin jaon liittäminen Unraidiin: restic neuvoo olemaan pitämättä arkistoa liitetyllä CIFS-jaolla, ja täällä mitään ei liitetä. NFS:lle ei ole ruutua, koska resticillä eikä rclonella ole NFS-taustajärjestelmää. NFS:lle liitä jako isännälle ja käytä sitä paikallisena kansiona.

### Siirtäminen ja poistaminen {#move-remove}

- **Siirrä** vie yhden osion kopion toiseen kohteeseen koko historioineen. Kaikki jää vanhalle paikalle. BombVault ei poista siellä mitään. Siirto ajetaan taustalla ja voi kestää: noin 9 minuuttia 100 Gt:tä kohti paikallisten levyjen välillä, tunteja hitaan yhteyden yli. Yleiskatsaus näyttää sen sen ajan.
- **Poista kohde** tarjoaa kaksi tietä. **Poista kohde, säilytä varmuuskopiot** lopettaa kirjoittamisen sinne ja jättää varmuuskopiot paikalleen. Voit lukea ne myöhemmin kohdassa **Palautus, Vanhasta varmuuskopiokansiosta**. **Poista kohde ja poista siellä olevat varmuuskopiot** lukee ensin, mitä siellä on, ja listaa sen: mitkä varmuuskopiot poistuvat, mitkä niistä eivät ole missään muualla ja mitkä arkistot jäävät, koska poistosuoja kieltäytyy, merkinnät varmuuskopioivat yhä suoraan niihin, toinen kohde tai osio säilyttää kopionsa siellä tai niitä ei voitu lukea. Merkinnät, jotka menettäisivät ainoan varmuuskopionsa, listataan erikseen ja säilytetään, ellet kytke päälle **Poista nämäkin**. Vahvistat kirjoittamalla kohteen nimen, ja poisto ajetaan taustalla.
- Tämä toimii kaikenlaisille kohteille: kohteelle, jota useat osiot käyttävät, yhtä osiota varten luodulle kohteelle, itse luomallesi arkistolle ja kansiolle, johon osio kirjoittaa omat varmuuskopionsa. Merkinnät, jotka varmuuskopioivat poistettuun arkistoon, seuraavat sen jälkeen taas osiotaan, samoin osion oletus, joka osoitti sinne. Osion kansiota ei voi vain poistaa, koska jokainen osio tarvitsee sellaisen. Ikkuna kysyy siksi kansiota, johon osiot siirtyvät, ja jokainen osio säilyttää sen alla oman kansionsa nimen.
- Kohteen varmuuskopioiden poistaminen toimii myös ilman kirjautumissalasanaa. Kun salasana on asetettu, se vaatii kirjautumisen itse instanssissa.

### Esimerkki: etäkopio Backblaze B2:een {#example-s3}

Konttien ja itsevarmuuskopion pitäisi olla myös Backblaze B2:ssa, jotta kuollut palvelin ei vie niitä mukanaan. Samat vaiheet sopivat Wasabille, Hetzner Object Storagelle ja useimmille muille S3-palveluille.

1. **B2:ssa** luo yksityinen bucket, esimerkiksi `tower-backups`, lähellä sinua olevalle alueelle. Luo kohdassa **Application Keys** avain, jolla on luku- ja kirjoitusoikeus ja joka on rajattu tähän bucketiin, ja merkitse muistiin sen avaintunnus ja sovellusavain. B2 näyttää sovellusavaimen vain kerran. Aseta bucketin elinkaareksi säilyttää vain viimeinen versio, tai B2 pitää karsitun datan piilotettuina versioina ja laskuttaa niistä.
2. **Palveluntarjoaja.** Avaa BombVaultissa **Kohteet**, paina **Lisää kohde** ja valitse **Backblaze B2** ryhmästä **S3-palvelu**.
3. **Kirjaudu sisään.** Anna avaintunnus kohtaan **Käyttöavaimen tunnus** ja sovellusavain kohtaan **Salainen käyttöavain**. Anna bucketin päätepiste kohtaan **Päätepiste**, esimerkiksi `s3.eu-central-003.backblazeb2.com`, ja BombVault lukee alueen siitä. Kirjoita `tower-backups` kenttään **Bucket**, koska tämä avain avaa vain yhden bucketin.
4. **Testaa yhteys.** Sen pitäisi sanoa **Yhteys on muodostettu** ja listata bucketin kansiot. Paina **Uusi kansio** ja nimeä se `bombvault`. BombVault luo siihen yhden kansion osiota kohti, esimerkiksi `bombvault/containers`.
5. **Suojaus.** Ikkuna selittää, että B2 voi säilyttää vanhoja versioita objektilukolla ja ettei BombVault voi tarkistaa sitä. Jätä **Poistosuoja (append-only)** pois päältä, ellei bucket todella kieltäydy poistoista, jotta BombVault voi karsia kohteen säilytyssääntöjen mukaan.
6. **Mille osioille.** Ruksaa **Kontit** ja **Itsevarmuuskopio**. Itsevarmuuskopio sisältää asetuksesi, jotta uusi asennus voi palauttaa ne. Ikkuna kertoo, kuinka monta merkintää ja, kun tiedetään, kuinka paljon historiaa ensimmäinen kopio tuo mukanaan.
7. Paina **Tallenna kohde**.

Ensimmäinen kopio ajetaan seuraavan varmuuskopion jälkeen. Käynnistääksesi sen heti avaa **Asetukset, Kohteet** ja paina **Replikoi nyt** osion rivillä kortissa **Etäkopiot**. Tilarivi sanoo **Etäkopiointi käynnissä**. Ensimmäinen kopio tuo koko historian, joten 200 Gt 100 Mbit/s:n lähetysyhteydellä kestää noin neljä ja puoli tuntia. Myöhemmät kopiot lähettävät vain sen, mikä on uutta.

Pitääksesi yhteytesi vapaana avaa kohteen sivu ja aseta **Lähetysraja (KiB/s)** kortissa **Kopioi**, esimerkiksi arvoon `5120`, joka vastaa noin 5,0 MB/s (41,9 Mbit/s). Tarkista samalla sivulla **Kuinka kauan säilytetään**.

Nähdäksesi, että se toimi, katso yleiskatsauksen **Suojaustila**-korttia, jossa **Etä** muuttuu tilaan **kopio ajan tasalla**, sekä kontin **Varmuuskopiot**-välilehteä, jossa sen varmuuskopiot nimeävät nyt `tower-backups`-kohteen toisena paikkana. Lataa sen jälkeen **Lataa palautuspaketti** uudelleen kohdassa **Asetukset, Palautus**, koska paketti listaa uuden kohteen ja sen tunnukset.

**Cloudflare R2 toimii samalla tavalla,** avaimen sijaan objektitokenilla. Liitä S3-osoite, jonka R2:n hallintapaneeli näyttää, bucket mukana, ja BombVault täyttää palvelimen ja bucketin.

### Esimerkki: itse ajamasi rest-server {#example-rest-server}

Rest-server append-only-tilassa kieltäytyy poistoista, joten sillä oleva kopio säilyy, vaikka joku kaappaisi palvelimesi. Toinen laatikko ystävän luona tai autotallissa riittää. Pariliitetty BombVault voi pystyttää tämän puolestasi, katso [Vastaanottopalvelin](#receiving-server). Ajaaksesi sen käsin missä tahansa Docker-isännässä:

```sh
docker run -d --name rest-server --restart unless-stopped \
  -p 8000:8000 \
  -v /mnt/user/backups/rest-server:/data \
  -e OPTIONS="--append-only --private-repos" \
  restic/rest-server
docker exec -it rest-server create_user bombvault
```

Viimeinen rivi kysyy salasanaa ja kirjoittaa käyttäjän datakansion `.htpasswd`-tiedostoon.

1. Paina BombVaultissa **Lisää kohde** ja valitse **rest-server** ryhmästä **NAS tai palvelin**.
2. Anna osoite niin, että käyttäjä on sen kansio, esimerkiksi `http://192.168.2.20:8000/bombvault`, sekä **Käyttäjätunnus** ja **Salasana**. Paina **Testaa yhteys**.
3. Kytke kohdassa **Suojaus** päälle **Poistosuoja (append-only)**. BombVault ei sen jälkeen koskaan karsi siellä vaan jättää sen vastapuolelle.
4. Ruksaa osiot ja paina **Tallenna kohde**.
5. Kun ensimmäinen kopio on perillä, avaa kohteen sivu ja paina **Testaa poistosuoja nyt**. BombVault yrittää poistaa testiobjektin. Kortin pitäisi sanoa **Viimeksi: poisto estettiin**, ja yleiskatsauksen **Poistosuoja** muuttuu tilaan **tarkistettu**. Jos se sanoo **Viimeksi: poistoa ei estetty**, `--append-only` puuttuu.

Koska mikään ei karsi tätä kopiota täältä, aseta **Kasvubudjetti (GB, 0 = pois)** osion ikkunassa, joka avataan kohdalla **Muokkaa** sen rivillä kortissa **Etäkopiot**, jotta kuulet siitä ennen kuin levy täyttyy. Itse allekirjoitetun varmenteen kanssa katso yllä oleva huomautus kansiosta `certs`.

## Osion oma kansio {#own-folder}

Jokaisella osiolla on oma kansio: arkisto, johon sen varmuuskopiot kirjoitetaan ensin. Kopiot muihin kohteisiin lähtevät sieltä. Uusi asennus pitää omat kansiot tällä palvelimella hakemiston `/mnt/user/bombvault` alla, ja yhdessä ne muodostavat **Paikallinen**-kohteen.

Oma kansio voi sijaita myös kohteessa, joka sinulla jo on, kuten NAS:ssa, rest-serverissä tai S3-bucketissa. Osio varmuuskopioi silloin suoraan sinne ilman paikallista kopiota välissä, ja oma kansio noudattaa sen kohteen tunnuksia, poistosuojaa ja kaistanleveyden rajoja. BombVault karsii sen jokaisen varmuuskopion jälkeen kohteen kortin **Kuinka kauan säilytetään** mukaan, ja osion omat arvot kohdassa **Säännöt osioittain** menevät kortin edelle. Kohde listaa osion kortissaan **Omat kansiot**, ja **Kohteet osioittain** näyttää kohteen osion kotina.

### Oman kansion siirtäminen {#move-own-folder}

1. Avaa kohde, jonka pitäisi pitää oma kansio, ja valitse **Käytä omana kansiona** tai **Lisää osioita** sen kortissa **Omat kansiot**. Siirtääksesi useita osioita kerralla valitse **Siirrä omat kansiot** kohdan **Omat kansiot täällä** vieressä Paikallinen-kohteen kortissa **Yhteys**.
2. Valitse osiot kohdassa **Osiot** ja paikka kohdassa **Siirrä kohteeseen**. **Muu paikka** ottaa kansion tällä isännällä tai käsin kirjoitetun etäarkiston osoitteen, tallennetuista tunnuksistasi valittavilla tunnuksilla.
3. Lue **Mitä tapahtuu**. Se kertoo jokaisesta osiosta, minne uudet varmuuskopiot menevät, muuttuuko uudessa paikassa jo oleva kopio omaksi kansioksi ja mitkä varmuuskopiot kopioidaan ensin.
4. **Kopioi jo tehdyt varmuuskopiot** on oletuksena päällä. Pois kytkettynä vanhat varmuuskopiot jäävät paikalleen, mutta merkintä ei enää listaa niitä. **Vanhasta varmuuskopiokansiosta** Palautus-sivulla lukee ne yhä kansion osoitteella ja etäkansion kohdalla sen kirjautumistiedoilla.
5. Valitse **Vaihda**. Kun uuden paikan kortti **Kuinka kauan säilytetään** säilyttää vähemmän kuin osion tämänhetkinen sääntö, BombVault kysyy ensin **Säilytetäänkö vähemmän?** Siinä **Säilytä** tuon osion **nykyiset arvot kohdassa Säännöt osioittain** ottaa tämänhetkiset arvot mukaan, **Tallenna silti** jatkaa, ja **Takaisin** jättää kaiken ennalleen. Kun varmuuskopiot pitää kopioida ensin, BombVault kopioi ne ja vaihtaa sitten oman kansion. Kun ne ovat jo perillä, se tarkistaa ne ennen vaihtoa.

Siirron jälkeen uudet varmuuskopiot menevät siirrettyyn omaan kansioon, myös osiolle, jolla **Paikallinen** oli kytketty pois ja joka varmuuskopioi suoraan kopioon. Kopio, joka oli jo uudessa paikassa, muuttuu omaksi kansioksi, ja sinne kopiointi päättyy. Jos kyseinen kopio piti joitakin varmuuskopioita pidempään kuin osion säilytyssäännöt, osio säilyttää kaiken, minkä jompikumpi sääntö säilyttää, joten mitään ei menetetä.

Kun siirto ei voi edetä, ikkuna kertoo syyn ruudussa kohdan **Mitä tapahtuu** alla. Kun vain vanhojen varmuuskopioiden kopiointi on esteenä, kohdan **Kopioi jo tehdyt varmuuskopiot** kytkeminen pois antaa oman kansion siirtyä ilman niitä.

Osio tarvitsee aina oman kansion. Sen sisältävän kohteen poistaminen kysyy kansiota, johon osio siirtyy, katso [Siirtäminen ja poistaminen](#move-remove). Oma kansio, jonka ohi uudet varmuuskopiot menevät, koska osio varmuuskopioi suoraan kopioon, on merkitty **(pois päältä)** siellä, missä kohde listaa sen käyttäjät.

![Ikkuna, joka siirtää oman kansion, kohta Mitä tapahtuu täytettynä yhdelle osiolle.](assets/screenshots/storage/move-own-folder-dark.png#only-dark)
![Ikkuna, joka siirtää oman kansion, kohta Mitä tapahtuu täytettynä yhdelle osiolle.](assets/screenshots/storage/move-own-folder-light.png#only-light)

#### Esimerkki: Kontit siirtyvät NAS:lle {#example-own-folder}

Cache-pooli täyttyy, ja sinulla on kohde nimeltä `nas`, SMB-jako toisessa laatikossa. Kontit-osiolla on 61 varmuuskopiota omassa kansiossaan tällä palvelimella, ja se kopioi Backblaze B2:een.

1. Avaa **Kohteet**, sitten `nas`, ja paina **Käytä omana kansiona**.
2. Ruksaa **Kontit** kohdassa **Osiot**.
3. Lue **Mitä tapahtuu**. Se kertoo, että Konttien uudet varmuuskopiot menevät `nas`-kohteen kansioon, että Paikallinen-kohteen 61 varmuuskopiota kopioidaan sinne ensin, että kopiot jatkavat Backblaze B2:een ja että suojaus, kaistanleveyden rajat ja kasvuhälytys tulevat `nas`-kohteelta.
4. Jätä **Kopioi jo tehdyt varmuuskopiot** päälle ja paina **Vaihda**.

BombVault kopioi varmuuskopiot taustalla ja vaihtaa sitten oman kansion. `nas`-sivu listaa Kontit kohdassa **Omat kansiot**, osiotaan seuraava merkintä nimeää `nas`-kohteen kodikseen, ja seuraava varmuuskopio päätyy sinne, ja B2-kopio lähtee `nas`-kohteesta. Tästä lähtien `nas`-kohteen kortti **Kuinka kauan säilytetään** karsii kansion. Vanha kansio tällä palvelimella pysyy paikallaan, eikä BombVault poista siellä mitään.

### Etäosoitteen kirjoittaminen käsin {#remote-primary-repositories}

Paikallinen-kohteen kortti **Yhteys** ottaa yhä vastaan etäosoitteen osion omalle kansiolle: osion vieressä oleva **Vaihda** avaa ikkunan, jossa on kytkin **Paikallinen / Etä**. **Etä** ottaa restic-osoitteen, kuten `s3:...`, `rest:http://host:8000/repo`, `sftp:user@host:/repo` tai `rclone:remote:bucket/path`, ja **Etäisen ensisijaisen tietovaraston turva-asetukset** lisäävät yhteystestin, lähetys- ja latausrajat, append-only-suojan samalla peukalointitestillä, jonka etäkohde saa, sekä kasvubudjettihälytyksen.

Tällainen osoite kirjautuu oletustunnuksilla. Kun se on täsmälleen se kansio, jota jokin sinulla oleva kohde pitää osiolle, se seuraa kyseistä kohdetta itsestään, kohteen kortti ja sen **Käytettävissä**-kytkin mukaan lukien, ellei se kirjaudu eri tavalla tai ole append-only-tilassa silloin, kun kohde ei ole. Sääntö, joka siihen asti karsi kansiota, jää osion omaksi kohtaan **Säännöt osioittain**. Kun se on kyseisen kohteen toisessa kansiossa, kohteen sivu listaa osion kohdassa **Jo tämän kohteen alla**, ja haltuunotto antaa sille kohteen tunnukset, tallennusluokan ja poistosuojan. Oman kansion siirtäminen kohteeseen toiminnolla **Käytä omana kansiona** on yksinkertaisempi tie, koska ikkuna tarkistaa jo tehdyt varmuuskopiot ennen kuin mitään muuttuu.

## Sijoittelu merkinnöittäin {#placement}

Merkinnän Asetukset-välilehden kortissa **Aikataulu ja kohde** on painikerivi: **Paikallinen** ja yksi painike osion jokaista etäkohdetta kohti. Päällä olevat painikkeet saavat merkinnän varmuuskopiot.

- Kun **Paikallinen** on päällä, merkintä kirjoitetaan kohdassa **Tallennuspaikka** näkyvään arkistoon ja kopioidaan jokaiseen muuhun päällä olevaan kohteeseen. Sammuta kohde, niin se ei saa tältä merkinnältä mitään uutta. Pelkkä Paikallinen ei kopioi minnekään, mikä sopii datalle, jolla on jo toinen kopio, esimerkiksi NAS:lla olevalle jaolle.
- Kun **Paikallinen** on pois, merkintä kirjoitetaan suoraan ensimmäisen päällä olevan kohteen suoraan arkistoon ja kopioidaan sieltä muihin päällä oleviin. Ensimmäisellä kerralla ikkuna luo tuon suoran arkiston.
- Yksi painike pysyy päällä, koska varmuuskopio tarvitsee paikan, minne mennä. Jättääksesi merkinnän varmuuskopioiden ulkopuolelle käytä **Älä varmuuskopioi**.

BombVault ei koskaan siirrä varmuuskopioita arkistojen välillä. Merkintä, joka seuraa osiotaan, siirtyy mukana, kun osion oletus muuttuu: sen seuraava varmuuskopio päätyy uuteen paikkaan, ja jo olemassa olevat varmuuskopiot jäävät paikalleen. Merkinnälle itselleen valittu sijainti lukitaan sen ensimmäisestä varmuuskopiosta alkaen. Asetukset-välilehden kortti **Repository** nimeää sen. Kopiot voivat muuttua milloin tahansa. Kohde, joka ei enää saa merkintää, säilyttää kopiot, jotka sillä on, ja karsii ne omaan säilytykseensä osion seuraavassa etäajossa. Append-only-kohteista ei voi poistaa.

Paikka, jonka merkintä jätti, säilyttää varmuuskopionsa. Merkinnän **Varmuuskopiot**-välilehdellä ja Palautus-sivulla se näkyy nimellä **Vain aiemmat varmuuskopiot**, palautus voi yhä lukea sieltä, ja karsinta siellä jättää nuo varmuuskopiot rauhaan.

**Noudata oletusta** tuo merkinnän, jolla on oma valinta, takaisin osioonsa. Se kysyy ensin, kertoo, minne uudet varmuuskopiot silloin menevät, ja nimeää, mitä kukin kohde lakkaa saamasta ja mitä seuraava ajo lähettäisi.

Rivin alla kortti kertoo, minne merkintä menee ja mitä siellä todella on: kuinka moni paikka pitää sitä, milloin kukin kohde on viimeksi nähty ja täyttyykö 3-2-1. Paikka on palvelin alkuperäisen datan kanssa, jokainen etäkohde ja jokainen arkisto, joka on merkitty **Toimipaikan ulkopuolella**. BombVault tarkistaa kopiot ja paikat. Se ei tarkista 3-2-1:n "kaksi tallennusvälinettä" -osaa.

### Oletukset osioittain {#placement-defaults}

Kohteet-sivun kortti **Kohteet osioittain** piirtää jokaisen osion oletuksen 3-2-1-virtana. Jokainen viiva osiosta kohteeseen on kytkin, myös **Paikallinen**, ja tämä koskee jokaista osiota, myös flash-asemaa, ZFS-tietojoukkoja ja itsevarmuuskopiota. Viivan tai pisteen napsautus kytkee sen heti, ja katkoviivat ovat pois päältä. Vain osion **Paikallinen**-kytkimen sammuttaminen kysyy ensin, samassa ikkunassa kuin oletuskortti, koska osio varmuuskopioi silloin suoraan kopioon. **Peruuta** jättää kaiken ennalleen. **Kaavio / Kytkimet** näyttää saman painikeriveinä. Kytkin ratkaisee, minne uudet varmuuskopiot menevät. Merkinnät, joilla ei ole omaa valintaa, ja Compose-stackien projektikansiot seuraavat heti, joten niiden seuraava varmuuskopio menee uusiin paikkoihin. Jo tallennetut varmuuskopiot jäävät paikalleen, ja ne voi yhä palauttaa sieltä. Flash, ZFS-tietojoukot ja itsevarmuuskopio varmuuskopioivat paikalliseen kansioonsa ja kopioivat etäkohteisiin siitä varmuuskopiosta. Niiden viivan kohteeseen **Paikallinen** voi myös kytkeä pois. Niiden varmuuskopiot menevät silloin suoraan niiden etäkohteisiin, ja ZFS-merkintä, jolla on replika mutta ei yhtään kohdetta, lähettää vain replikansa.

Kun **Paikallinen** on pois ja vain etäkohteet ovat päällä, osio varmuuskopioi suoraan niihin, ja BombVault valitsee arkiston siellä itse. Kontit, virtuaalikoneet ja kansiojoukot menevät ensimmäisen päällä olevan kohteen suoraan arkistoon, jota kuvataan alempana, ja kopioidaan sieltä muihin. Flash-asema, ZFS-tietojoukot ja itsevarmuuskopio kirjoittavat jokaisen varmuuskopion jokaiseen päällä olevaan kohteeseen, ja kukin kohde säilyttää ne oman säilytyksensä mukaan.

Osio, jonka kaikki viivat ovat pois, näyttää **Ei varmuuskopioitu**. Sen merkinnät eivät saa varmuuskopioita ja lasketaan suojaamattomiksi.

Merkinnät, jotka ovat valinneet omat kohteensa sivullaan, pitävät ne. Virta kertoo, kuinka monta niitä osiossa on, esimerkiksi **2 merkintää poikkeaa**, ja vie niiden luo: yhden kohdalla merkinnän Asetukset-välilehdelle, useamman kohdalla luetteloon, joka näyttää, minne kukin varmuuskopioi. **Noudata oletusta** tuo merkinnän siellä takaisin.

Uusi etäkohde saa jokaisen merkinnän, jota ei ole asetettu Paikalliseksi. Sen lisäävä ikkuna kertoo, kuinka monta merkintää ja, kun tiedetään, kuinka paljon historiaa se on.

### Suorat arkistot {#direct-repositories}

Suora arkisto on paikka, jossa merkinnät varmuuskopioidaan suoraan etäkohteeseen. BombVault sijoittaa sen kohteen kansion viereen lisäämällä nimeen `-direct` tai, kun kohde on bucketin, etäsijainnin tai rest-server-käyttäjän juuressa, sen sisällä olevaan kansioon, joka on nimetty osion mukaan, esimerkiksi `containers-direct`. Kohteet-sivun virrassa se luodaan kysymättä. Kun Paikallinen sammutetaan yksittäiseltä merkinnältä niin, että kohteesta ilman suoraa arkistoa tulee sen koti, avautuu ikkuna, jossa on ehdotettu sijainti, esimerkiksi `s3:https://s3.eu-central-003.backblazeb2.com/bucket/containers-direct`, ja yhteystesti, joka ei luo mitään. **Luo ja käytä** luo arkiston ja osoittaa merkinnän siihen. Suoralla arkistolla ei ole omia asetuksia. Se seuraa kohteensa kytkintä, tunnuksia, tallennusluokkaa, rajoja, append-only-asetusta ja säilytystä ja muuttuu niiden mukana. Kohteen sivu listaa sen kohdassa **Varmuuskopioidaan suoraan tänne**, jokaisen osion kanssa, joka varmuuskopioi suoraan siihen, ja sen, kuinka moni merkintä niin tekee. Sellainen, jolle mikään ei enää varmuuskopioi, kertoo sen siellä ja sen voi poistaa, ja varmuuskopiot sen paikalla säilyvät. Kun kohteen uusi avain ei voi avata sitä, suora arkisto pitää nykyisen avaimensa ja tallennus kertoo sen. Suoralla arkistolla oleva merkintä kopioidaan sieltä muihin päällä oleviin kohteisiin, ei koskaan kohteeseen, jolle arkisto kuuluu. Sen varmuuskopioilla on tunniste `bv:direct`, ja jokainen muu säilytyskierros säilyttää ne, joten suora arkisto, joka menetti linkkinsä kohteeseensa, ei koskaan vanhene paikallisten sääntöjen mukaan. B2 tavoitetaan sen S3-päätepisteen kautta, avaintunnus ja sovellusavain S3-avaimina. Kohteen omaan kansioon rajattu avain ei yllä viereiseen kansioon, joten rajaa avain sen yläpuoliseen kansioon.

### Toimipaikan ulkopuolella {#off-the-premises}

Kohde tai arkisto voidaan merkitä **Toimipaikan ulkopuolella** -merkillä. Etäkohteet alkavat merkittyinä. Kytke se pois rest-serveriltä samassa rakennuksessa. Merkki vain laskee paikat ja 3-2-1:n korteilla. Se ei muuta mitään kopiota.

ZFS-replika on kopio tilojen ulkopuolella, mutta ei varmuuskopio historialla. Se säilyttää vain asettamasi tilannevedokset, ja vanhat versiot ja yksittäiset tiedostot tulevat yhä varmuuskopioista. Se kulkee yhdeltä ZFS-palvelimelta suoraan toiselle, SSH:n yli ZFS-palvelimelle tai pariliitetyn instanssin vastaanottopäätepisteeseen. Pariliitosryhmä kuljettaa vain pyynnön. Katso [Replika](zfs-datasets.md#replica).

### Uudelleenrakennuksen jälkeen {#after-a-rebuild}

Kopiointivalinnat sijaitsevat BombVaultin omissa asetuksissa. Uudelleenrakennuksen jälkeen kohdalla **Tunnista varmuuskopiot** tai Hätäohjatulla toiminnolla ilman palautettuja asetuksia ne ovat poissa, ja kaiken kopioiminen lähettäisi pois jättämäsi merkinnät taas B2:een. Sama pätee, kun osio, joka ei ole koskaan kopioinut etäkopioita tästä tietokannasta, löytää omia varmuuskopioitaan kohteesta tai vanhempia omasta kansiostaan, esimerkiksi kun kohde lisättiin samalla `APP_KEY`:llä tehdyn aiemman asennuksen varmuuskopioiden päälle. Silloin osion etäkopiot odottavat päätöstäsi.

**Vaatii huomiota** yleiskatsauksessa ja jokaisen kohteen sivu, johon osio kopioi, kysyvät, ovatko varmuuskopiot tämän palvelimen omia. **Ota ne tämän palvelimen omiksi** vahvistaa osion oletuksen, ja sen kopiot ajetaan taas seuraavasta kierroksesta alkaen. **Tarkista ensin säännöt** vie osion riville kortissa **Kohteet osioittain**, jossa näet, minne kukin merkintä menee, ja voit asettaa kadonneen säännön uudelleen merkinnän sivulla. Kopiointi odottaa siellä, kunnes painat **Ota ne tämän palvelimen omiksi**, joka näyttää ensin, mitä seuraava ajo kopioi, ja nimet varmuuskopioissa, joilla ei ole merkintää ja jotka voit jättää pois. Jokaisen odottavan merkinnän uusin varmuuskopio kertoo sen ajossaan ja linkittää päätökseen, ja ilmoitus lähtee, kun ilmoitukset ovat päällä. Asetustiedoston tuonti tuo takaisin säännöt ja oletukset mutta ei tee päätöstä puolestasi.

![Vaatii huomiota uudelleenrakennuksen jälkeen: kansiojoukot löysivät varmuuskopioita ajalta ennen tätä tietokantaa, ja niiden etäkopiot odottavat päätöstä.](assets/screenshots/offsite/copy-decision-dark.png#only-dark)
![Vaatii huomiota uudelleenrakennuksen jälkeen: kansiojoukot löysivät varmuuskopioita ajalta ennen tätä tietokantaa, ja niiden etäkopiot odottavat päätöstä.](assets/screenshots/offsite/copy-decision-light.png#only-light)

## Poistosuoja (append-only) {#delete-protection-append-only}

Kytke poistosuoja päälle etäkohteelle, jotta kiristysohjelma tai kaapattu isäntä ei voi poistaa tai kirjoittaa uudelleen varmuuskopioitasi. Vastapuoli (`restic/rest-server` tilassa `--append-only`) **panee sen täytäntöön**. BombVault vain **varmentaa** sen eikä koskaan näytä vihreää pelkän kokoonpanoväitteen perusteella. Tämän palvelimen kansio ei voi kieltäytyä poistosta, koska se, joka hallitsee BombVaultia, hallitsee myös kansiota, joten paikalliset kohteet eivät tarjoa kytkintä.

Kytkin kuuluu kohteelle: osio, joka kopioi sinne, saa sen kohteelta, joten kytke se päälle kohteen sivulla kohdassa **Kohteet**. Ajastettu **Tarkista poistosuoja** ajetaan vain, kun jokin osio kopioi kohteeseen, jossa kytkin on päällä. Toisen pään on käynnistyttävä valinnalla `--append-only`. `restic/rest-server`-imagessa se laitetaan TrueNASissa kuten missä tahansa muussa Docker-isännässä muuttujaan `OPTIONS`.

Jotkin paikat eivät voi lainkaan kieltäytyä poistosta, esimerkiksi pilviasema kuten OneDrive. Jos säilytät kopioita siellä tietoisesti, kytke paikan sivulla päälle **Hyväksy ilman poistosuojaa**. Yleiskatsaus näyttää silloin puuttuvan poistosuojan hyväksyttynä riskinä osiolle, joka kopioi vain hyväksyttyihin paikkoihin, harmaana renkaana oranssin pisteen sijaan, ja osion väri seuraa sen muita tarkistuksia. Muuta ei muutu: siellä olevat varmuuskopiot voi yhä poistaa, BombVault karsii niitä edelleen paikan sääntöjen mukaan, ja kytkin katoaa heti, kun poistosuoja on päällä. Toiseen paikkaan siirtyvä kopio menettää hyväksynnän. Myös poistosuojan kytkeminen päälle poistaa sen.

Nopein tie append-only-kopioon on pariliitetty BombVault, jolla on [vastaanottopalvelin](#receiving-server): se ajaa rest-serverin puolestasi append-only-tilassa.

[Jaetussa arkistossa](#shared-repository) tämä koskee jäseniä, ei arkiston omistajaa. Omistaja pääsee kansioon suoraan ja unohtaa ja karsii siellä.

!!! note "Onnistunut poisto polussa `/locks/` on odotettua"
    Append-only ei tarkoita, ettei mitään voisi enää poistaa. resticin on otettava ja vapautettava omat lukkonsa, joten `/locks/` pysyy tarkoituksella kirjoitettavana ja poistettavana. Varmuuskopioita ja niiden takana olevaa dataa, eli juuri sitä mihin kiristysohjelma tähtäisi, ei voi poistaa. Jos koettelet vastapuolta itse, onnistunut poisto polussa `/locks/` on oikea toiminta eikä aukko suojauksessa.

!!! warning "Append-only-repoja ei koskaan karsita tältä laatikolta"
    Append-only-kohde ei koskaan karsi vanhoja varmuuskopioita täältä. Karsinta on vastapuolen työtä. Aseta sille **kasvubudjettihälytys**, jotta saat varoituksen, ennen kuin repon koko karkaa käsistä.

## Peukalointitesti {#tamper-test}

BombVault todistaa ajoittain append-only-takeen yrittämällä oikeasti poistoa etärepoa vastaan, kohdistettuna olemattomaan objektiin. **Testaa poistosuoja nyt** kohteen sivulla ajaa sen pyynnöstä, ja **Tarkista poistosuoja** kohdassa **Asetukset, Tarkistus, Automaattiset tarkistukset** ajaa sen aikataulun mukaan.

Kun **Tarkista poistosuoja** kytketään pois, aukko ei piiloudu: koskaan testaamaton poistosuoja näkyy silloin todistamattomana, koska tarkistus on pois päältä, ja paikka ilman poistosuojaa näkyy joka tapauksessa suojattomana.

Pois päältä olevaa kopiota ei testata, ja kohde kertoo sen. Yhteystesti toimii silti, koska se vain lukee, joten kopion voi tarkistaa ennen kuin sen ottaa käyttöön.

- **Torjuttu** tarkoittaa suojattua.
- **Hyväksytty** tarkoittaa ei-suojattua.
- **Epäselvä** tulos (palvelin tavoittamattomissa, todennusvirhe) ei koskaan käännä tallennettua tuomiota.

Todellinen suojatusta suojaamattomaksi -kääntyminen laukaisee yhden hälytyksen.

## DR-harjoitukset {#dr-drills}

BombVault tarjoaa kaksi tasoa todisteita siitä, että varmuuskopiosi ovat tosiasiassa palautuskelpoisia, eivät vain olemassa.

- **Palautuksen todentamisharjoitukset (paikalliset).** BombVault ajaa ajoittain `restic check --read-data-subset` (rajattu, ei koskaan levyn täyttävä täyspalautus). Tahti on kohdassa **Asetukset, Tarkistus, Automaattiset tarkistukset**, ja saman ruudun kortti **Tarkista varmuuskopiot** näyttää tuloksen.
- **Koepalautus (etä).** BombVault palauttaa oikean kohteen etärepositorysta eristettyyn hiekkalaatikkoon, varmentaa sen tiedosto tiedostolta ja tavu tavulta ja siivoaa sitten. Tämä todistaa, että voit toipua etäkopiosta, ei vain että repo vastaa.

Yleiskatsauksen kortti **Suojaustila** kokoaa tämän osioittain neljäksi tarkistukseksi: ajantasainen varmuuskopio tällä palvelimella, kopio etänä, poistosuoja siellä ja koepalautus sieltä. Osion napsautus näyttää tiedot, ja kortti muuttuu vihreäksi vain varmennetuilla tosiasioilla. Paikka, jonka hyväksyit ilman poistosuojaa, lasketaan siellä hyväksytyksi riskiksi eikä varoitukseksi.

## Instanssien pariliitos {#pairing}

Useat BombVaultit toimivat yhdessä ryhmänä: ne näyttävät toistensa suojaustilan, ottavat toisilleen rooleja ja voivat säilyttää toistensa avainta. Instanssi liittyy ryhmään kahdellatoista sanalla.

![Pariliitossivu ja sen kolme vaihekorttia.](assets/screenshots/offsite/pairing-dark.png#only-dark)
![Pariliitossivu ja sen kolme vaihekorttia.](assets/screenshots/offsite/pairing-light.png#only-light)

Avaa ensimmäisellä instanssilla **Asetukset, Pariliitos** ja paina **Luo lause**. Kolme vaihekorttia näyttää, miten se menee. Kaksitoista sanaa ilmestyy ikkunaan, jossa on **Kopioi**-painike. Avaa jokaisella muulla instanssilla sama paikka, paina **Syötä lause** ja liitä tai kirjoita ne, tai paina **Liitä** siinä ikkunassa. Sana, jota ei ole luettelossa, nimetään sijaintinsa kanssa kirjoittaessasi, ja viimeisessä sanassa on tarkistussumma, joten väärin kirjoitettu tai vaihtunut sana huomataan, ennen kuin mitään pariutuu. Luo lause vain yhdellä instanssilla: kaksi instanssia, jotka kumpikin luovat sellaisen, muodostavat kaksi erillistä ryhmää. Jos kukaan ei ilmesty minuutin kuluessa, sivu tarjoaa kaksi ulospääsyä: näytä sanat uudelleen syötettäviksi toisaalla tai syötä toisen instanssin sanat ja liity sen ryhmään yhdellä kertaa. Pariliitos toimii ilman kirjautumissalasanaa, mutta aseta sellainen: ilman sitä kuka tahansa, joka voi avata verkkokäyttöliittymän, voi lukea sanat ja ryhmän kautta pyytää rooleja, jotka luovuttavat siinä olevien instanssien restic-salasanan. Pariliitoskortti kertoo sen, kunnes salasana on asetettu. Salasanan kanssa lauseen näyttäminen uudelleen kysyy sitä. **Poistu ryhmästä** ottaa instanssin pois. Pariliitossivun **Ilmentymät**-kortti listaa ryhmän jäsenet. Kuva siitä, kuka tekee mitä kenelle, on Ilmentymät-sivulla.

Kuka tahansa sanat tietävä voi liittyä ryhmään, joten kohtele niitä kuin salasanaa.

**Miten jäsenet tavoittavat toisensa.** Jokainen instanssi oppii oman osoitteensa verkossa selaimestasi heti kirjautuessasi, listattuna kohdassa **Tunnetut osoitteet** pariliitossivun **Tämä instanssi** -kortissa yhdessä niiden kanssa, jotka instanssi löysi itse. Korjaa se siellä, jos käänteinen välityspalvelin tai epätavallinen portti on edessä. Samassa verkossa jäsenet ilmoittavat tuon osoitteen multicastilla ja puhuvat suoraan, ja siellä missä multicast ei pääse konttiverkon yli, kuten Dockerin oletussillassa, instanssi hakee sen sijaan omasta aliverkostaan muita allekirjoitetulla kutsulla, johon vain ryhmän jäsen voi vastata, joten pariliitos valmistuu sekunneissa ilman relettä. Jos mitään ei löydy, **Etkö löydä sitä?** ottaa yhden osoitteen käsin, toista aliverkkoa tai epätavallista porttia varten. Eri verkoissa olevat instanssit kulkevat releen kautta, joka valitaan neljän rivissä samalla sivulla olevassa **Rele**-kortissa. Jokaisella muodolla on kuva:

- **Projektin rele** (oletus): `parleyport.halleluja.design`, sama rele jota myös KnightLoader käyttää. Ei mitään asennettavaa.
- **Oma rele**: Unraidin Community Appsin [**ParleyPort**](https://github.com/junkerderprovinz/parleyport)-kontti (myös kohdassa **Asetukset, Sovellukset**). Syötä sen osoite jokaisella instanssilla, jonka pitää käyttää sitä.
- **BombVault-instanssi**: yksi instansseistasi, joka on jo tavoitettavissa ulkoa ja jolla **Toimi releenä** on päällä. Tuo instanssi vastaa silloin polussa `/relay/connect` omassa osoitteessaan, jo olemassa olevan käänteisen välityspalvelimen ja varmenteen takana, ja päästää sisään vain ryhmäsi. Syötä sen osoite jokaisella instanssilla, jonka pitää käyttää sitä.
- **Ei relettä**: jäsenet löytävät toisensa automaattisesti vain samassa verkossa, eivät missään muualla.

**Mitä rele näkee.** Samanniminen painike neljän muodon vieressä näyttää tämän valitsemallesi muodolle. Jokainen jäsenten välinen kutsu sinetöidään AES-256-GCM:llä kahdestatoista sanasta johdetulla avaimella, eikä tuo avain koskaan poistu instansseistasi. Rele oppii tiivisteen, joka ryhmittelee yhteydet, mille instanssille viesti on tarkoitettu, kuinka suuri se on ja milloin se kulkee. Suora kutsu paikallisessa verkossa sinetöidään samoin ja allekirjoitetaan lisäksi, joten mikään ei riipu instanssin tarjoamasta itse allekirjoitetusta varmenteesta. Kutsut, jotka luovuttavat jotakin (roolivastaukset, restic-salasana, vastaanottajan tunnukset, avaintalletukset), allekirjoitetaan myös ne lähettävän jäsenen avaimella, kiinnitettynä ensimmäisellä näkemällä. Katso [Turvallisuusmalli](configuration.md#security-model).

**Mitä ryhmässä kulkee.** Ilmentymät-sivun suojaustila, pyyntö tarkistaa yksi osio nyt, roolipyynnöt ja niiden vastaukset, kohdetarjoukset sekä se, mitä vastaanotin tai noutaja tarvitsee, kun sen rooli on sallittu: toisen instanssin repositoryjen sijainnit ja sen restic-salasana. Varmuuskopiodata ei koskaan. Se kulkee yhä suoraan restic-taustajärjestelmiin. Eikä APP_KEY: restic-salasana avaa kyseisen instanssin repositoryt eikä mitään muuta, ei sen tallennettuja salaisuuksia, istuntoja tai noutokoodeja.

**Merkinnät ennen pariliitosta.** Laiteryhmätunnuksella lisätyt instanssit sekä vastaanotetut repositoryt ja noutolähteet, jotka on asetettu toisen instanssin APP_KEY:llä, säilyvät päivityksen jälkeen ja merkitään **Pariuta uudelleen**. Ne toimivat edelleen: ensimmäisellä käynnistyksellä BombVault korvaa jokaisen tallennetun APP_KEY:n siitä johdetulla restic-salasanalla. Pariuta molemmat instanssit, muokkaa sitten merkintää ja valitse sen instanssi. Siihen asti tällaiset merkinnät ovat Ilmentymät-sivun kortissa **Ilman instanssia**.

Ainoa paikka, joka yhä ottaa APP_KEY:n käsin, on [Vanhasta varmuuskopiokansiosta](#restore-from-another-bombvault-repo), tilanteeseen, jossa toinen instanssi on poissa eikä voi vastata ryhmässä.

## Ilmentymät ja roolit {#roles}

**Ilmentymät**-sivu (asetusruutu, jonka voi kiinnittää sivupalkkiin) näyttää ryhmän verkkona siitä, kuka tekee mitä kenelle, kortti tälle instanssille ja kortti jokaiselle muulle jäsenelle, Android-puhelimille ja ZFS-palvelimille. **Pariliitos** oikeassa alakulmassa vie pariliitossivulle. Mitkä osat ovat päällä, asetetaan kohdassa **Asetukset, Ilmentymät, Useita palvelimia**: **Ilmentymät**, **Vastaanotin** ja **Nouto**.

![Ilmentymät-sivu: ryhmä verkkona ja kortti kutakin instanssia kohti rooliruutuineen, yksi niistä Ei aktiivinen.](assets/screenshots/offsite/instances-dark.png#only-dark)
![Ilmentymät-sivu: ryhmä verkkona ja kortti kutakin instanssia kohti rooliruutuineen, yksi niistä Ei aktiivinen.](assets/screenshots/offsite/instances-light.png#only-light)

Jokainen kortti näyttää jäsenen suojaustilan ja sen roolit ruutuina. Rooli on jotakin, mitä yksi instanssi tekee toiselle:

- **Vastaanotin** säilyttää tämän palvelimen etäkopiot rest-serverissä append-only-tilassa. Tämä palvelin lähettää jokaisen varmuuskopion jälkeen eikä voi poistaa siellä mitään, vaikka joku kaappaisi sen.
- **Noutaja** hakee tämän palvelimen varmuuskopiot omalla aikataulullaan. Tämän palvelimen ei tarvitse olla hereillä sitä varten. Noutavalla instanssilla on oltava **Nouto** päällä, ennen kuin se voi sallia pyynnön.
- **ZFS-palvelin** ottaa vastaan tämän palvelimen ZFS-replikat. Jokainen ZFS-merkintä valitsee sen asetuksissaan kohdassa **Replika**.
- **Säilyttäjä** säilyttää tämän palvelimen palautuspaketin sinetöityä kopiota. Katso [Avaimen säilyttäjä](#key-keeper).

Roolin kytkeminen päälle lähettää pyynnön, eikä mikään käynnisty, ennen kuin joku toisessa instanssissa painaa **Salli**. Pyyntö näkyy sen instanssin kortilla merkkinä, ja sen **Tiedot** listaavat sen toiminnoilla **Salli** ja **Hylkää**. Sallitun roolin voi peruuttaa siellä milloin tahansa. Katkoviiva verkossa on rooli, joka yhä odottaa. BombVault 9.x -versiolla oleva instanssi ei voi vastata pyyntöön, joten rooli on siellä aktiivinen heti kun kytket sen päälle, ja 9.x-instanssin pyyntö odottaa täällä **Salli**-toimintoa.

Kortin **Tiedot** avaa loput: jäsenen vastaanotetut repositoryt ja noutolähteet, kohdetarjoukset, valvonta, **Poista** ja **Unohda avain**.

Korttien alla oleva **Jaettu arkisto** -kortti laatikossa, jolla on vastaanottopalvelin, ja **Jaetut arkistot** -kortti jäsenellä kuuluvat [jaettuun arkistoon](#shared-repository).

Jäsen voi tarjota omaa etätallennustilaansa toiselle jäsenelle ryhmän kautta. Toinen ylläpitäjä näkee tarjouksen jäsenen kohdassa **Tiedot** ja hyväksyy tai hylkää sen. Hyväksyminen luo tallennetut tunnukset ja tavallisen kohteen samojen tarkistusten jälkeen kuin itse kirjoittamasi kohde. Kohteen poistosuoja on valmiiksi päällä, koska tarjouksen näyttämä ohje on append-only-tilainen rest-server, joten **Kuinka kauan säilytetään** ei koske sitä. Jokaisella tarjouksella on omat tunnuksensa ja oma kansionsa, joten osion tarjoaminen uudelleen samalle tai toiselle jäsenelle jättää ensimmäisen kumppanin kopiot rauhaan: lisää uusi rivi tiedostoon `.htpasswd` ja pidä palvelin käynnissä. Tätä kautta kulkevat vain yhteystiedot, ei koskaan varmuuskopiodataa. **Tarkista nyt** pyytää jäsentä tarkistamaan yhden osion repositoryn. Mikään sivulla ei voi käynnistää varmuuskopiota tai palautusta eikä poistaa mitään toisella laatikolla.

Sammutetut instanssit ja puhelimet, joiden sovellus on suljettu, säilyttävät korttinsa tilassa **Ei aktiivinen**, harmaana ja tiedolla siitä, milloin ne nähtiin viimeksi, ja ne näkyvät verkossa himmennettyinä, kunnes poistat ne toiminnolla **Poista**. Heti tämän palvelimen käynnistyttyä tällainen kortti näyttää **Tarkistetaan**, kunnes ryhmä on taas tavoitettavissa. **Poista** toimii tällaisella kortilla riippumatta siitä, mitkä osat on kytketty päälle, ja se päättää myös jäsenen avoimet pyynnöt, sen käyttämättömät ZFS-vastaanottopaikat ja sen odottavat avainpyynnöt. Pariliitossivun **Ilmentymät**-kortti listaa nämä jäsenet myös tilassa **Ei aktiivinen**.

## Vastaanotin ja noutaja {#receiver}

Laatikossa, joka **vastaanottaa** append-only-kopioita toiselta BombVaultilta, vastaanotetut repositoryt antavat riippumattoman, vain luku -tilaisen valvonnan vastaanottavalla laitteistolla, jotta äänetön vika vastapäässä ei jää huomaamatta. Kytke **Vastaanotin** päälle kohdassa **Asetukset, Ilmentymät**, avaa sitten lähettävän instanssin **Tiedot** Ilmentymät-sivulla ja lisää vastaanotettu repository (vain luku, avattuna lähettäjäinstanssin restic-salasanalla, joka tulee [pariliitosryhmän](#pairing) kautta). Saat:

- **Tilannevedosluettelon lähteittäin ryhmiteltynä**, jotta näet tarkalleen, mitkä kontit, VMs ja kansiojoukot ovat saapuneet. Varmuuskopiot, jotka lähettäjä teki ennen kuin sillä oli oma [isäntä](#backup-host), lasketaan samaan lähteeseen.
- **Viimeksi vastaanotettu** lähteittäin, jotta tiedät, kuinka tuore kukin on.
- **Riippumattoman `restic check`** -ajon vastaanottavalla laitteistolla, jotta eheys todennetaan siellä missä data tosiasiassa sijaitsee, ei vain lähettäjällä.
- **Kuolleen miehen kytkimen:** hälytys, kun lähde lakkaa lähettämästä asettamasi ikkunan sisällä. Se tarkistaa kerran päivässä, kello 09.15. Saat yhden hälytyksen myös silloin, kun repositorya ei voi lukea lainkaan, sanoi tarkistusaikataulu mitä tahansa, ja kun mitään ei ole saapunut kokonaisen ikkunan aikana sen lisäämisen jälkeen. Repository, jonka lisäät ennen lähettäjän ensimmäistä kopiota, on siihen asti kunnossa.
- **Eheyshälytykset:** hälytys, kun tarkistus vastaanottavalla puolella epäonnistuu. Tarkistus, joka ei pystynyt avaamaan repositorya, ei kerro eheydestä mitään, joten se ei anna tulosta ja yrittää uudelleen seuraavana päivänä samaan aikaan.

Toisella palvelimella oleva repository tarvitsee sen palvelimen kirjautumistiedot: tallenna ne [tunnuksiksi](#sign-ins) ja valitse ne valintaikkunan kohdassa **Kirjautumistiedot**.

Vastaanotin on tiukasti vain luku. Se ei koskaan kirjoita vastaanotettuun repositoryyn, joten se ei voi koskaan rikkoa append-only-takuuta, johon lähettäjä luottaa.

Noutaja toimii toisin päin: kun **Nouto** on päällä, instanssi lukee toisen jäsenen varmuuskopiot omalla aikataulullaan, jokaiselle lähteelle omaan arkistoonsa, joten niitä ei koskaan lasketa tämän laatikon omiksi varmuuskopioiksi. Sen lähteet ovat myös kyseisen jäsenen tiedoissa: arkiston sijainti, minkä lajin varmuuskopioita se sisältää ja kuinka usein noudetaan. Restic-salasana tulee ryhmän kautta, ei koskaan APP_KEY, eikä vastapuolella ole mitään muuta määritettävää, eikä sen tarvitse olla käynnissä. Lähdettä vain luetaan, ja kumpikin puoli pitää omat tunnuksensa. Nouto ajetaan sinä päivänä ja siihen aikaan, jonka sen aikataulu kertoo, ja uusikin lähde odottaa sitä aikaa. **Nouda nyt** ei odota. Nouto, joka jäi väliin, koska tämä laatikko oli pois päältä, ajetaan heti, kun laatikko on taas päällä. Epäonnistunut nouto yrittää uudelleen seuraavana päivänä samaan aikaan, myös viikoittaisella aikataululla, ja se näkyy toimintalokissa ja lähettää yhden hälytyksen, kunnes nouto taas onnistuu. `rclone:`-lähde torjutaan, koska rclone tavoittaisi sen tämän instanssin etäsijainneilla.

Noudettu kopio vain kasvaa. BombVault lisää siihen eikä karsi tai tarkista sitä, joten se, minkä lähde on jo kauan sitten unohtanut, pysyy täällä, kunnes poistat kansion, eikä Kohteet-sivu laske sitä mukaan. Se on polussa `pulled/<source id>` osion oman kansion sisällä, ja kun kansio siirtyy, kopio jää paikalleen ja myöhemmät noudot jatkavat siellä sen sijaan, että kaikki kopioitaisiin uudelleen. Jos poistat sen, seuraava nouto aloittaa alusta siinä omassa kansiossa, joka osiolla silloin on.

### Vastaanottopalvelin {#receiving-server}

Vastaanottava laatikko voi myös ajaa rest-serverin, johon muut kopioivat. Se määritetään ensimmäisellä kerralla, kun sallit **Vastaanotin**-pyynnön, joka sitä pyytää: ikkuna kertoo, että palvelin tarvitsee vastaanottopalvelimen, ja kysyy kansiota jaolta, **Uusi kansio** -toiminnolla uuden luomiseen, sekä porttia (8000, ellei toinen kontti käytä sitä). BombVault sitten:

1. kieltäytyy jatkamasta, jos kontti nimeltä `rest-server` on jo olemassa tai toinen kontti pitää porttia,
2. hakee `restic/rest-server`in ja käynnistää sen Dockerin socketin kautta append-only-tilassa yksityisillä repositorioilla ja kansiossa olevalla kirjautumistiedostolla,
3. kirjoittaa sen Unraid-mallin flash-asemalle, jolloin kontti pysyy muokattavana Docker-välilehdellä, tai tarjoaa mallin ladattavaksi, kun flash-asema ei ole saatavilla,
4. ajaa sille peukalointitestin ja näyttää, kieltäytyykö se poistoista.

Ryhmäsi instanssit löytävät palvelimen sitten palveluntarjoajien joukosta kohdassa **Ryhmästäsi**, vastaanottavan laatikon nimellä, ja **Vastaanotin**-rooli tarjoaa sen turvallisena valintana. Jokainen instanssi saa oman tunnuksen, kun se ensimmäisen kerran käyttää palvelinta, ja kirjoittaa siellä vain omaan kansioonsa. Kun **Vastaanotin** on päällä kohdassa **Asetukset, Ilmentymät**, tämän instanssin kortti **Vastaanottopalvelin** avaa palvelimen uudelleen. Sen ikkuna listaa tunnukset kohdassa **Kumppanit, joilla on tunnus**, ja **Peruuta tunnus** poistaa yhden. Mitä kyseinen instanssi on jo kopioinut, jää kansioon. Käyttöönotto luo myös yhden **Tunnuksen ryhmän ulkopuolelle**, jonka salasanan ikkuna näyttää kerran.

Instanssi, joka tavoittaa vastaanottavan laatikon vain releen kautta, ei voi käyttää palvelinta, koska rele ei kuljeta varmuuskopioita. Lisää vastaanottavan laatikon osoite pariliitossivulla ensin. Kun BombVault toimii omalla IP-osoitteellaan (esimerkiksi br0:ssa), täytä **Osoite kumppaneille**, koska palvelin kuuntelee isännän osoitteessa.

Vastaanottava laatikko voi myös pitää ryhmälle [jaettua arkistoa](#shared-repository).

## Jaettu arkisto {#shared-repository}

Useampi ryhmän palvelin voi kopioida yhdessä yhteen restic-arkistoon. Niiden yhteinen data, kuten sama kansiojoukko tai sama appdata, tallennetaan sinne vain kerran, joten yhdessä ne tarvitsevat vähemmän tilaa kuin jos jokaisella olisi oma arkisto.

Yksi ryhmän palvelimista on arkiston omistaja. Vain se unohtaa siellä vanhat varmuuskopiot ja karsii. Muut ovat jäseniä: ne pääsevät arkistoon toisen, append-only-tilassa toimivan rest-serverin kautta ja voivat vain lisätä. Jokainen palvelin kirjoittaa [omalla isännällään](#backup-host), joten varmuuskopiot pysyvät erillään silloinkin, kun kahdella palvelimella on samanniminen merkintä.

Mitä tarvitaan:

- [pariliitosryhmä](#pairing),
- **Salaa varmuuskopiot** päällä omistajalla ja jokaisella jäsenellä, koska jokainen palvelin saa oman restic-avaimen,
- omistajalla [vastaanottopalvelin](#receiving-server), jonka jäsenet tavoittavat. Kun BombVault toimii omalla IP-osoitteellaan, täytä ensin **Osoite kumppaneille** vastaanottopalvelimen ikkunassa.

Palvelin voi omistaa enintään yhden jaetun arkiston.

### Käyttöönotto {#shared-set-up}

Kun vastaanottopalvelin on käynnissä, sen laatikon **Ilmentymät**-sivulla on instanssien alla **Jaettu arkisto** -kortti. Paina **Ota jaettu arkisto käyttöön**, valitse **Jaetun arkiston kansio** jaolta, jolla on runsaasti tilaa, pidä **Portti** arvossa 8001, ellei toinen kontti käytä sitä, ja paina **Ota käyttöön ja käynnistä**. BombVault sitten:

1. kieltäytyy jatkamasta, jos kontti nimeltä `rest-server-shared` on jo olemassa tai toinen kontti pitää porttia,
2. luo arkiston sen sisälle kansioon `shared` tämän palvelimen salasanalla,
3. käynnistää `restic/rest-server`in nimellä `rest-server-shared` append-only-tilassa ja kansiossa olevalla kirjautumistiedostolla,
4. kirjoittaa Unraidissa sen mallin flash-asemalle, jolloin kontti pysyy muokattavana Docker-välilehdellä,
5. ajaa sille peukalointitestin. Kortin **Tarkista uudelleen** ajaa sen vielä kerran.

Kortti listaa **Jäsenet**, kunkin avaimineen ja viimeisimpine varmuuskopioineen, liittymispyynnöt sekä jokaisen jäsenen kohdalla **Poista**.

### Liittyminen {#shared-join}

Avaa liittyä haluavalla palvelimella **Lisää kohde**. Kohdassa **Ryhmästäsi** omistaja näkyy toisen kerran, muodossa **Jaettu arkisto instanssissa** ja sen nimi. Sen valitseminen lähettää omistajalle pyynnön, joka näkyy siellä **Jaettu arkisto** -kortilla painikkeiden **Salli** ja **Hylkää** kanssa.

Kun pyyntö on sallittu, omistaja antaa ryhmän kautta kertakäyttöisen avaimen ja tunnuksen toiseen rest-serveriinsä. Jäsen lisää sillä oman restic-avaimen, ja omistaja poistaa sen jälkeen kertakäyttöisen avaimen. Jäsen, joka ei ole valmis vuorokauden kuluessa, menettää kertakäyttöisen avaimen ja tunnuksen ja voi pyytää uudelleen. Kun omistaja ei tavoittanut jäsentä vastauksellaan, kortti tarjoaa **Kerro uudelleen**.

Omistaja hylkää palvelimen, niin kauan kuin se ei vielä kirjoita omalla isännällään, mikä tarkoittaa, että se käyttää vanhempaa julkaisua, ja silloin, kun toinen jäsen kirjoittaa jo samalla isännällä. Kahdella palvelimella on sama isäntä, kun niillä on sama `APP_KEY`.

Kun jäsen on mukana, kohtaan **Kohteet** ilmestyy kohde nimeltä **Shared with** ja omistajan nimi, kaikki osiot pois päältä. Kytke kohteen sivulla päälle osiot, joiden pitää kopioida sinne. Kohteella on poistosuoja eikä omia säilytyssääntöjä, koska siellä unohtaa vain omistaja. Sinne menevät vain jäsenen omalla isännällä tehdyt varmuuskopiot, joten vanhemmat varmuuskopiot isännällä `bombvault` jäävät paikoilleen.

Omistaja voi kopioida myös omat varmuuskopionsa arkistoon: sen kortin **Kopioi myös tämän palvelimen varmuuskopiot** lisää kohteen myös sille. Omistajalle tämä kopio on sen omilla levyillä, eikä mikään estä sitä poistamasta siellä, joten se ei ole etäkopio.

### Kuka siivoaa ja milloin {#shared-upkeep}

Vain omistaja unohtaa ja karsii. Sen asetukset ovat **Vastaanottopalvelin**-ikkunan **Jaettu arkisto** -osassa, joka avautuu omistajan omasta kortista Ilmentymät-sivulla:

- **Siivousikkuna** määrää, milloin karsinta ajetaan, päivittäin tai viikoittain. Kunnes muutat sitä, se alkaa sunnuntaisin klo 03:00, ja **Ikkunan pituus tunteina** on 6.
- **Käyttämätön tila, jonka karsinta saa jättää, %** on 5, kunnes muutat sitä. Suurempi arvo pakkaa uudelleen vähemmän, joten karsinta valmistuu nopeammin.
- **Karsinnan jälkeen takaisin luettu data, %** on 5, kunnes muutat sitä. Jokaisen karsinnan jälkeen omistaja tarkistaa arkiston ja lukee tämän osuuden datasta takaisin.
- **Kuinka kauan säilytetään** seuraa kunkin osion kohdalla omistajan etäkopiosääntöjä, ja **Aseta oma sääntö** antaa jaetulle arkistolle oman. Kunkin palvelimen varmuuskopiot vanhenevat erikseen.
- **Siivoa nyt** unohtaa, karsii ja tarkistaa heti, kun se on ilmoittanut jäsenille.

Ikkunan ulkopuolella omistaja unohtaa kerran päivässä, ja tila vapautuu seuraavassa karsinnassa. Ennen karsintaa se ilmoittaa jäsenille, eivätkä ne kopioi arkistoon mitään ennen kuin ikkuna sulkeutuu. Jäsen, joka vielä kopioi, saa enintään puoli tuntia aikaa valmistua, muuten karsinta siirtyy seuraavaan ikkunaan. Ikkunaan osuva kopio odottaa, näkyy lykättynä toimintalokissa ja jäsenen kortilla, ja seuraava kierros ottaa sen kiinni.

Kun omistaja on epävarma, se säilyttää enemmän. Merkintä, jota jäsen pitää [poikkeaman](anomalies.md) vuoksi, säilyttää kaikki varmuuskopionsa, jäsen, joka ei pysty kertomaan, mitä se pitää, säilyttää sillä kertaa kaikki varmuuskopionsa, ja varmuuskopioita isännällä, jolla mikään ryhmän palvelin ei kirjoita, ei unohdeta koskaan. Ikkuna näyttää myös tilan: paljonko arkisto vie, paljonko palvelimet tarvitsisivat erikseen ja paljonko sillä säästyy. Luvut päivittyvät jokaisen siivouksen yhteydessä.

### Mitä jäsenet näkevät {#shared-members}

Jäsenen Ilmentymät-sivulla on **Jaetut arkistot** -kortti, jossa on jokainen jaettu arkisto, johon palvelin kopioi tai johon se on pyytänyt liittyä, ja missä sen pyyntö on: **Odottaa hyväksyntää**, **Liittymässä**, **Mukana**, **Hylätty**, **Vanhentunut** tai **Ei jäsen**. Arkistosta, jossa se on mukana, kortti näyttää **Kuka karsii**, **Seuraava siivous**, muut jäsenet kohdassa **Mukana myös**, **Viimeisin kopio tänne** sekä paljonko tilaa arkisto vie ja säästää.

### Eroaminen {#shared-leave}

Jäsenen kortin **Eroa** vie palvelimen pois: omistaja poistaa sen avaimen ja tunnuksen, ja sen kohde siellä kytketään pois. Omistajan kortin **Poista** vie jäsenen pois toiselta puolelta. Kummassakin tapauksessa palvelimen varmuuskopiot jäävät arkistoon ja vanhenevat omistajan sääntöjen mukaan. Omistajalla ne näkyvät kohdassa **Lähteneet palvelimet**, jossa **Unohda varmuuskopiot** unohtaa ne kaikki, kun kirjoitat palvelimen nimen. Seuraava siivousikkuna vapauttaa tilan.

### Muiden palvelimien varmuuskopiot {#shared-restore}

Jokainen palvelin löytää omat varmuuskopionsa jaetusta arkistosta merkintöjensä **Varmuuskopiot**-välilehdeltä, kuten minkä tahansa muun kopion. Muiden jäsenten varmuuskopiot ovat Palautus-sivulla kohdassa [Muut palvelimet](restore.md#other-servers).

### Hätätilanteen jälkeen {#shared-emergency}

[Palautuspaketissa](#recovery-kit) on luku **Shared repositories**. Se nimeää jokaisen jaetun arkiston omistajineen, sinne kopioivat osiot, muut jäsenet isäntineen ja `restic snapshots` -komennon tämän palvelimen isännällä. Omistajan paketti nimeää lisäksi sen rest-serverin portin, kansion ja tunnuksen.

Hätäohjatussa toiminnossa arkisto näkyy paikkojen joukossa muodossa **Jaettu arkisto palvelimella** ja omistajan nimi. Uusi asennus, joka ottaa jäsenen avaimen haltuunsa, kirjoittaa samalla isännällä, koska isäntä tulee avaimesta. Se saa kohteensa jaetussa arkistossa takaisin ja listaa ja palauttaa siellä vain omat varmuuskopionsa.

Uusi asennus, joka ottaa omistajan avaimen haltuunsa, ryhtyy arkiston omistajaksi sen tilalle. Se vanhentaa ja siivoaa arkistoa kuten ennenkin ja käynnistää rest-serverin uudelleen samaan kansioon. Liitä se uudelleen ryhmään, ota sen vastaanottopalvelin käyttöön kohdassa **Ilmentymät** ja aseta säännöt uudelleen vastaanottopalvelimen ikkunassa, koska ne eivät ole paketissa. Kun ryhmä tavoittaa jäsenet, uusi omistaja kertoo niille itse. Jokainen jäsen tarkistaa ensin: se lisää arkistoon kokeiluavaimen, jonka vain arkiston kansion äärellä oleva palvelin voi poistaa, ja seuraa uutta omistajaa, kun avain on poistunut. Jos uusi omistaja vastaa toisessa osoitteessa, jokainen jäsen tekee tämän tarkistuksen siellä omalla avaimellaan ja tunnuksellaan, varmistaa löytävänsä saman arkiston ja vasta sitten siirtää kohteensa uuteen osoitteeseen. Siihen asti se kopioi edelleen vanhaan osoitteeseen, ja sen kortti kertoo syyn. Tällaisista tarkistuksista jääneet avaimet omistaja poistaa seuraavassa siivouksessa.

Jos vanha omistaja on yhä käynnissä, sen kortti kertoo, että arkisto on nyt toisen palvelimen hallussa. **Ota takaisin** tekee siitä taas omistajan siltä varalta, että paketti otettiin vahingossa käyttöön toisella palvelimella.

### Rajoitukset {#shared-limits}

- **Jokainen avain lukee kaiken.** Restic-arkiston kaikki avaimet avaavat saman pääavaimen, joten jokainen jäsen voi lukea muiden varmuuskopiot. Konttien ja VM:ien määritykset pysyvät sinetöityinä kunkin palvelimen omalla avaimella, joten toisen palvelimen kontit ja VM:t palaavat vain tiedostoina. Palvelin, jolla on joskus ollut pääsy, on voinut lukea pääavaimen, joten sen poistaminen ei sulje sitä ulos lopullisesti. Sen tekee vain uusi arkisto.
- **Omistaja voi poistaa.** Yksikään jäsen ei voi poistaa sieltä mitään, mutta omistaja pääsee kansioon suoraan ja unohtaa ja karsii jokaisen jäsenen varmuuskopioita. Virhe tai kaappaus omistajan päässä ulottuu kaikkiin.
- **Vaurio ulottuu jokaiseen palvelimeen.** Rikkinäinen pack puuttuu jokaiselta palvelimelta, jonka varmuuskopiot käyttävät sitä, joten jaettu arkisto ei korvaa toista kopiota.
- **Vain vastaanottopalvelimella.** S3, SFTP ja muut paikat eivät pysty rajaamaan jäsentä pelkkään lisäämiseen, joten BombVault tarjoaa jaettua arkistoa vain vastaanottopalvelimella.
- **Vain kopioinnin kohteena.** Osion oma kansio ei voi olla jaetussa arkistossa, koska jokainen karsinta pysäyttäisi kaikkien palvelimien varmuuskopiot.
- **Vanhempi historia jää paikoilleen.** Sinne menevät vain kunkin palvelimen omalla isännällä tehdyt varmuuskopiot, eikä mitään siirretä muista arkistoista.

## Esimerkki: kahden Unraid-koneen pariliitos {#worked-example-two-unraid-boxes-end-to-end}

Tämä on yksi täydellinen käyttöönotto yllä kuvatuista osista, koska ne on helpompi koota, kun on nähnyt ne koottuina kerran.

Kaksi laatikkoa: **TOWER** ajaa kontteja ja työntää varmuuskopioita. **VAULT** vastaanottaa ne ja panee append-only-tilan täytäntöön. Korvaa omilla nimilläsi, osoitteillasi ja jakopoluillasi.

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

**5. Seuraa saapuvaa, jos haluat.** Kytke **Vastaanotin** päälle VAULTilla kohdassa **Asetukset, Ilmentymät**, avaa TOWERin **Tiedot** ja lisää vastaanotettu repository: napsauta yhtä valintaikkunan listaamista sijainneista. VAULT saa TOWERin restic-salasanan ryhmän kautta, ja koska sijainti on VAULTin omalla vastaanottopalvelimella, se lukee kansion suoraan. Kukaan ei kirjoita avainta eikä kirjautumistietoja.

**6. Tee siitä molemminpuolinen.** Toista vaiheet toiseen suuntaan: VAULT pyytää TOWERia vastaanottajakseen. Kumpikin laatikko panee sitten append-only-tilan täytäntöön toiselle, eikä kumpikaan voi poistaa toisen varmuuskopioita.

Käsin ajettu rest-server toimii myös: lisää se kohdalla **Lisää kohde** rest-serverinä, käyttäjällä ja salasanalla sen `.htpasswd`-tiedostosta, ja kytke poistosuoja päälle. Pidä `--append-only` sen asetuksissa. Ilman sitä vastapuoli on taas tavallinen jako.

!!! warning "Paikallinen polku on polku kontin sisällä"
    Kun BombVault pyytää kansiota tältä palvelimelta, syötä se suhteessa isännän liitokseen, kuten `user/appdata/rest-server/bombvault-containers`, ei `/mnt/user/appdata/...`. BombVault toimii kontissa, jossa isännän `/mnt` on liitetty muualle. Isännän absoluuttista polkua ei ole sen sisällä. Jos liität sellaisen, BombVault kertoo suhteellisen polun, jota käyttää sen sijaan.

## Hätäohjattu toiminto {#emergency}

**Hätäohjattu toiminto** tuo takaisin palvelimen, joka on poissa tai pystytetty uudelleen. Se alkaa **Palautus**-sivulta kohdasta **Hätätilanne** sekä uuden asennuksen tervetulosivun kohdasta **Palauta varmuuskopiosta**. Se etenee neljässä vaiheessa: **Pääsy**, **Asetukset**, **Valinta** ja **Palautus**.

1. **Pääsy.** Näytä BombVaultille vanhojen varmuuskopioiden avain yhdellä kolmesta tavasta:
    - **Lataa palautuspaketti**: käyttöönoton aikana lataamasi tiedosto. Se sisältää avaimen ja jokaisen kohteen, ja BombVault täyttää loput sen perusteella. Age-salattu paketti kysyy identiteettiä, jolle se sinetöitiin. Sitä käytetään kerran eikä tallenneta.
    - **Ryhmän kautta**: ryhmäsi instanssi säilyttää avainta talletuksena. Tarvitset ryhmän kaksitoista sanaa, noutokoodisi ja jonkun kyseisessä instanssissa. Katso [Avaimen säilyttäjä](#key-keeper).
    - **Kirjoita avain**: vanhan palvelimen `APP_KEY` ja se, missä sen varmuuskopiot ovat.

    BombVault kokeilee sitten avainta jokaisessa kohteessa ja kertoo kustakin, onko se luettavissa, ei löydy, on lukittu toisella avaimella, ei ole tavoitettavissa vai puuttuuko siltä vielä kirjautuminen. Mitään ei ole vielä tallennettu, ja avain pysyy muistissa rajoitetun ajan. **Ota avain haltuun** saa tämän palvelimen kirjoittamaan varmuuskopionsa tuolla avaimella tästä lähtien. **Kumoa haltuunotto** palaa sen omaan avaimeen, kunnes ensimmäinen varmuuskopio on ajettu. Palvelin, jolla on jo omia varmuuskopioita, kieltäytyy haltuunotosta ja ohjaa kohtaan [Vanhasta varmuuskopiokansiosta](#restore-from-another-bombvault-repo). **Tallenna tunnukset** tallentaa tunnukset, jotka paketti sisältää etäkohteille, ja kysyy ennen kuin korvaa tunnukset, jotka tällä palvelimella jo on.
2. **Asetukset.** **Palauta asetukset** palauttaa BombVaultin omat asetukset uusimmasta Itsevarmuuskopiosta: kohteet, tunnistetiedot, aikataulut ja säännöt. BombVault käynnistyy uudelleen ottaakseen ne käyttöön Docker-soketin kautta, jotta käytössä olevaa asetustietokantaa ei koskaan ylikirjoiteta avoimen kahvan alla, ja ohjattu toiminto jatkuu itsestään sen jälkeen. Tämän palvelimen kirjautuminen säilyy: kirjaudut yhä nykyisellä salasanallasi. Salaisuudet, jotka vanha palvelin oli tallentanut ja joita ei voi ottaa haltuun, listataan, jotta voit syöttää ne uudelleen. Palautus päättää kaikki istunnot ja poistaa pääsyavaimet, joten ota ne sen jälkeen uudelleen käyttöön.
3. **Valinta.** BombVault käy läpi varmuuskopiosi ja listaa löytämänsä kontit, VMs, kansiojoukot ja ZFS-tietojoukot. Uusin varmuuskopio on esivalittu kaikkialla. Se, mikä tällä palvelimella yhä on, alkaa pois päältä, koska palautus korvaisi sen. Kansiojoukot eivät enää tiedä, mistä ne tulivat uudelleenrakennuksen jälkeen, joten ne menevät valitsemaasi kansioon.
4. **Palautus.** Kaikki valittu palaa kerralla, jätettynä pysäytetyksi, jos haluat käynnistää jokaisen kontin ja VM:n itse, mikä on turvallisinta täydellisen menetyksen jälkeen. Lataa sen jälkeen uusi palautuspaketti: vanha kuuluu edelliselle asennukselle.

Paketin jaettu arkisto näkyy paikkojen joukossa muodossa **Jaettu arkisto palvelimella** ja sen omistavan palvelimen nimi, ja ohjattu toiminto listaa siellä vain tämän palvelimen omat varmuuskopiot. Katso [Hätätilanteen jälkeen](#shared-emergency).

![Hätäohjattu toiminto ensimmäisessä vaiheessaan, kolme tapaa sisään.](assets/screenshots/offsite/emergency-dark.png#only-dark)
![Hätäohjattu toiminto ensimmäisessä vaiheessaan, kolme tapaa sisään.](assets/screenshots/offsite/emergency-light.png#only-light)

### Esimerkki: uusi palvelin palautuspaketista {#example-emergency}

Palvelin TOWER kuoli levyineen. Siinä oli 23 konttia ja 2 VM:ää, ja se kopioi ne ja itsevarmuuskopionsa Backblaze B2:een. Sinulla on `bombvault-recovery-kit.md` salasanahallinnasta ja uusi laatikko, jossa on Unraid. Sen vanha `APP_KEY` on kadonnut, etkä tarvitse sitä.

1. **Asenna BombVault** uuteen laatikkoon ja anna sille uusi `APP_KEY`, kuten missä tahansa uudessa asennuksessa. Määritä VM-yhteys kohdassa **Asetukset, Integraatiot, Palvelimen SSH**, jos haluat VM:t takaisin, koska VM:n palautus tarvitsee sitä.
2. **Käynnistä ohjattu toiminto.** Valitse sivulla **Tervetuloa BombVaultiin** vaihtoehto **Palauta varmuuskopiosta**. Valitse Palautus-sivulla kohdassa **Hätätilanne** vaihtoehto **Käynnistä hätäohjattu toiminto**.
3. **Pääsy.** Valitse **Lataa palautuspaketti** ja pudota tiedosto ikkunaan. BombVault kokeilee sen avainta jokaisessa kohteessa ja sanoo esimerkiksi **Palautuspaketti luettu. Sen avain avaa 0 kohdetta 2:sta.** Paikallinen kansio sanoo **ei löytynyt**, koska levy kuoli, ja Backblaze B2 sanoo **tunnukset puuttuvat**.
4. **Tallenna tunnukset.** Paketti sisältää jokaisen kohteen tunnukset. Paina **Tallenna tunnukset**, niin kohteita kokeillaan uudelleen, ja Backblaze B2 sanoo nyt **luettavissa**. Ennen versiota 10.5 kirjoitettu paketti sisältää vain oletustunnukset. Jos kohde sanoo yhä **tunnukset puuttuvat**, syötä sen tunnukset kohdassa **Syötä puuttuvat tunnukset** ja paina **Yritä kohteita uudelleen**.
5. **Ota avain haltuun.** Tämä palvelin kirjoittaa nyt varmuuskopionsa vanhalla avaimella, joten uudet varmuuskopiot sopivat vanhojen kanssa. Se pitää oman `APP_KEY`:nsä mallissa. **Kumoa haltuunotto** toimii, kunnes ensimmäinen varmuuskopio on ajettu.
6. **Asetukset.** Vaihe avautuu paikkaan, josta avain löysi itsevarmuuskopion, tässä B2, ja sen osoite on valmiiksi täytetty. Valitse **Palauta asetukset**, joka lukee uusimman itsevarmuuskopion sieltä. BombVault käynnistyy uudelleen ottaakseen sen käyttöön, ja ohjattu toiminto jatkuu itsestään. Kohteesi, aikataulusi ja sääntösi ovat takaisin, ja kirjaudut yhä tämän palvelimen salasanalla. Syötä uudelleen salaisuudet, jotka ohjattu toiminto listaa, kuten ilmoitusasetukset.
7. **Valinta.** **Missä varmuuskopiot ovat?** avautuu etäkopioihin, kun mitään paikallista ei ole jäljellä. BombVault lukee jokaisen kohteen, joten kuollut kohde ei piilota muita, listaa 23 konttia ja 2 VM:ää ja palauttaa jokaisen kohteesta, jossa on sen uusin varmuuskopio.
8. **Palautus.** Kytke päälle **Jätä pysäytetyksi jälkeenpäin**, mikä on turvallisinta täydellisen menetyksen jälkeen, ja aloita. **Mitä tapahtuu** kertoo, mitä palaa. Tulosrivi kuuluu esimerkiksi **25 palautettu, 0 epäonnistui. Käynnistä ne tarvittaessa kohdassa Varmuuskopiot.**
9. **Viimeistely.** Käynnistä tietokantasi ensin ja sitten niitä käyttävät sovellukset. Avaa **Asetukset, Palautus** ja lataa uusi palautuspaketti, koska vanha kuuluu edelliselle asennukselle. Tarkista **Vaatii huomiota** yleiskatsauksessa ja aja **Varmuuskopioi kaikki nyt**, kun kaikki toimii.

Jos itsevarmuuskopiokin puuttuu, vaihe avautuu kohtaan **Ohita: minulla ei ole Itsevarmuuskopiota** ja kertoo sen. Kohteet tulevat silloin pelkästään paketista, aikataulut ja säännöt asetetaan uudelleen, ja etäkopiot odottavat, kunnes otat niiden varmuuskopiot tämän palvelimen omiksi, katso [Uudelleenrakennuksen jälkeen](#after-a-rebuild). **Valinta** listaa yhä, mitä etäkopiot sisältävät. Kansiojoukot menevät valitsemaasi kansioon. Kontit ja VM:t palaavat tavallisina tiedostoina kansioon kohdassa **Vain tiedostot voidaan palauttaa**, koska niiden määritykset ovat paikallisen arkiston vieressä ja itsevarmuuskopiossa, eikä etäkopio sisällä kumpaakaan.

!!! tip "Suunniteltu siirto vastaan katastrofi"
    Hätäohjattu toiminto palauttaa BombVaultin omat asetukset varmuuskopiosta. *Suunniteltua* siirtoa uuteen laatikkoon varten voit sen sijaan viedä kokoonpanosi suoraan **Vie / tuo asetukset** -kortilla (siirrettävä JSON-tiedosto). Katso [Asetukset](configuration.md#portable-settings-export-and-import).

### Vanhasta varmuuskopiokansiosta {#restore-from-another-bombvault-repo}

**Vanhasta varmuuskopiokansiosta** kohdassa **Muut lähteet** Palautus-sivulla avaa *toisen* BombVault-instanssin repositoryn (`/mnt`-alle liitetty jako tai etäosoite) kyseisen instanssin palautuspaketilla tai sen kirjoitetulla `APP_KEY`:llä kertaluonteisessa, vain luku -istunnossa. Selaa siellä säilytettyjä kontteja, VMs, kansiojoukkoja ja ZFS-tietojoukkoja, valitse varmuuskopio ja palauta se, ja palautetusta kohteesta tulee tavallinen paikallinen kontti, VM tai kansiojoukko. Toiseen repositoryyn ei koskaan kirjoiteta, ja omat varmuuskopioasetuksesi pysyvät koskemattomina (istunto elää muistissa ja vanhenee itsestään). Kontin siirtäminen palvelimelta A palvelimelle B ei tarkoita oman repo-asetuksesi osoittamista toisaalle ja palauttamista jälkeenpäin. Tämä ikkuna on kertakäyttöinen: se avaa istunnon, palauttaa valitsemasi ja unohtaa toisen instanssin. Jos haluat sen sijaan pysyvän järjestelyn, jossa tämä laatikko hakee toisen instanssin varmuuskopiot omaan repositoryynsä aikataulun mukaan, se on **Noutaja**-rooli Ilmentymät-sivulla.

### Mitä tämä palvelin on noutanut {#restore-what-was-fetched}

Jokaisella lähteellä, josta tämä palvelin on noutanut, on oma **Haettu**-rivinsä kohdassa **Muut lähteet** Palautus-sivulla. Noudettu kopio on lukittu tämän palvelimen omalla avaimella eikä lähteen avaimella, joten mitään ei kysytä: lähteen avain ei avaa sitä. Kansiojoukot ja ZFS-tietojoukot menevät valitsemaasi kansioon. Kontit ja VM:t palaavat tavallisina tiedostoina kansioon, koska nouto tuo tilannevedokset mutta ei määrityksiä. Luodaksesi ne uudelleen käytä lähteen palautuspakettia ja itsevarmuuskopiota Hätäohjatussa toiminnossa.

Kontti, jonka verkkoa tällä palvelimella ei ole, esimerkiksi Unraidin `br0`-verkko tavallisella Docker-isännällä, näyttää rivinsä alla verkon valinnan. BombVault luo sen valitsemaasi verkkoon muine verkkoineen. Kiinteä IP-osoite ja MAC-osoite kuuluivat vanhaan verkkoon ja jäävät pois, joten uusi verkko antaa ne.

## Avaimen säilyttäjä {#key-keeper}

Avaimen säilyttäjä on ryhmäsi instanssi, joka pitää tämän palvelimen palautuspaketin sinetöityä kopiota. Se ei voi itse avata kopiota. Jos tämä palvelin joskus katoaa, uusi asennus, joka liittyy ryhmääsi, pyytää kopion säilyttäjältä ja avaa sen noutokoodillasi.

![Noutokoodin ikkuna: kaksikymmentä merkkiä neljässä ryhmässä, näytetään kerran.](assets/screenshots/offsite/recovery-code-dark.png#only-dark)
![Noutokoodin ikkuna: kaksikymmentä merkkiä neljässä ryhmässä, näytetään kerran.](assets/screenshots/offsite/recovery-code-light.png#only-light)

1. Kytke Ilmentymät-sivulla **Säilyttäjä** päälle sen instanssin kortilla, jonka pitää säilyttää, ja valitse **Tee talletus**. Talletuksen tekeminen vaatii kirjautumissalasanan.
2. Tämä palvelin näyttää **Noutokoodin** kerran: kaksikymmentä kirjainta ja numeroa neljässä viiden ryhmässä. Kirjoita se muistiin ja vahvista **Kirjoitin koodin muistiin**. Tämä palvelin ei tallenna koodia, vain sen, mitä se tarvitsee talletuksen sinetöintiin sille.
3. Säilyttäjä pitää nyt talletusta, ja tämä palvelin uusii sen, kun paketti muuttuu. **Avaimen talletus** tämän instanssin kortilla listaa **Tämän palvelimen talletuksen** kunkin säilyttäjän ja sen tilan kanssa sekä **Talletukset, joita tämä palvelin säilyttää** muille. **Tee uusi koodi** korvaa koodin. Jokainen säilyttäjä siirtyy uuteen, kun se saa uuden talletuksen.

Sinä päivänä tarvitset kolme asiaa: ryhmäsi kaksitoista sanaa, noutokoodin ja pääsyn säilyttäjän käyttöliittymään, jossa jonkun täytyy painaa **Salli**. Valitse Hätäohjatussa toiminnossa **Ryhmän kautta**, valitse talletus, pyydä sitä ja kirjoita noutokoodi. Koodia käytetään uudella palvelimella, eikä sitä lähetetä säilyttäjälle. Säilyttäjällä pyyntö näkyy kohdassa **Avaimen talletus** pyytäjän, sen, onko kyseinen instanssi koskaan nähty siellä aiemmin, ja pyyntöjen määrän kanssa. **Salli** luovuttaa sinetöidyn kopion, ja sallimiseen tarvitaan kirjautumissalasana. Uudella asennuksella on 30 minuuttia aikaa noutaa se Sallimisen jälkeen, pyyntö, johon kukaan ei vastaa, vanhenee päivän kuluttua, ja instanssin, joka pyytää liian usein, täytyy odottaa tunti.

Ilman koodia kukaan ei voi avata kopiota, säilyttäjäkään. Jos koodi katoaa, tee uusi, kun tämä palvelin yhä toimii. Jos palvelin on jo poissa, palautuspaketti ja kirjoitettu APP_KEY toimivat yhä. Säilyttäjä voi poistaa talletuksen milloin tahansa, ja omistajan täytyy silloin pyytää uudelleen.

## Palautuspaketti {#recovery-kit}

Tämä on se pala, joka tekee katastrofista toipumisen mahdolliseksi silloinkin kun käynnissä olevaa BombVaultia ei ole.

**Lataa palautuspaketti** kortissa **Hätätilanteiden varalle** kohdassa **Asetukset, Palautus** antaa sinulle **avaimen**, **johdetun restic-salasanan**, jokaisen **kohteen** kunkin osion repositoryineen, tunnukset, joita uusi asennus tarvitsee niihin pääsyyn, sekä **tarkat restic-komennot**, jotta voit palauttaa suoraan restic-komentorivillä millä tahansa koneella. Se vaatii kirjautumissalasanan. Paketti tulostaa avaimen myös salaus pois päältä, koska kontti- ja VM-määritykset sinetöidään aina sillä. Haltuunoton jälkeen se tulostaa haltuun otetun varmuuskopioavaimen ja tämän palvelimen APP_KEY:n, jota uusi asennus tarvitsee vain asetusten palauttamiseen. Yleiskatsauksen kortti **Tallenna palautuspakettisi** muistuttaa, kunnes olet tallentanut sen.

!!! danger "Säilytä palautuspaketti palvelimen ulkopuolella"
    Paketti sisältää salaisuuden, joka purkaa varmuuskopiosi. Säilytä se turvallisessa paikassa erillään palvelimesta (salasanahallinta, tulostettu kopio kassakaapissa). Jos menetät sekä BombVaultin että `APP_KEY`:n ilman palautuspakettia ja ilman avaintalletusta, salattuja varmuuskopioitasi ei voi palauttaa.

!!! warning "Uusin varmuuskopio ei aina ole se, joka kannattaa palauttaa"
    Resticistä 0.17 lähtien `restic snapshots` näyttää kunkin tilannevedoksen koon. Tietojen menetyksen jälkeen uusin varmuuskopio voi olla tyhjennetty, joten älä palauta varmuuskopiota, joka on paljon pienempi kuin sitä edeltävät. Kiristysohjelman jälkeen se voi olla salattu tavallisen kokoinen. Jos BombVault yhä toimii, katso ensin sen **Poikkeamat**-sivu: se nimeää viimeisen hyvän varmuuskopion, ja palautusikkuna esivalitsee sen. Palautus ei tarvitse mitään BombVaultin poikkeamadataa, ja säilytyksen tauko vain säilyttää enemmän varmuuskopioita.

### Palvelimen varmuuskopioiden isäntä {#backup-host}

Jokainen palvelin kirjoittaa varmuuskopionsa omalla restic-isännällään: `bv-` ja kaksitoista merkkiä, jotka lasketaan varmuuskopioiden avaimesta, esimerkiksi `bv-3f9a0c41d2e7`. Varmuuskopioiden isäntä on `bombvault`, jos ne on tehty ennen kuin palvelimella oli oma isäntä. Paketti nimeää tämän palvelimen isännän, ja sen komennot antavat molemmat `--host`-valitsimella, joten `latest` ei koskaan valitse varmuuskopiota, jonka jokin toinen palvelin on kirjoittanut samaan repositoryyn. Palvelin, joka ottaa haltuunsa toisen varmuuskopiot, jatkaa sen palvelimen isännällä, koska isäntä tulee avaimesta. Jokaisesta [jaetusta arkistosta](#shared-repository), johon palvelin kopioi tai jonka se omistaa, paketin luku **Shared repositories** nimeää siellä olevat muut palvelimet isäntineen.

### Paketin sinetöinti {#sealing-the-kit}

Jos olet kytkenyt päälle **Salaa viennit age:lla** kohdassa **Asetukset, Järjestelmä**, paketti sinetöidään myös sillä ja latautuu nimellä `bombvault-recovery-kit.md.age`. Se on ASCII-panssaroitu eikä binäärinen, joten se on yhä tekstiä: sen liittäminen salasanahallintaan tai tulostaminen toimii täsmälleen kuten ennen, sisältö vain on lukukelvoton ilman avaintasi.

!!! warning "Älä säilytä age-avainta paketin sisällä"
    Sinetöidyn paketin avaamiseen tarvitset age-**yksityisavaimesi**. Säilytä se paikassa, joka ei riipu itse paketista, tai sinulla on palautettavana kaksi asiaa yhden sijaan. Sinetöinti kannattaa, kun paketti on tallessa paikassa, jota et täysin hallitse (jaettu salasanojen hallinta, pilvimuistiinpanot, tuloste toimistolla). Omassa kassakaapissasi oleva paketti on jo kassakaapin suojaama.

    Kun salaus on päällä eikä käyttökelpoista vastaanottajaa ole määritetty, lataus torjutaan suoraan. BombVault ei koskaan turvaudu antamaan pääavainta selkotekstinä.

### Kun paketti ei ole käsillä {#if-you-do-not-have-the-kit-to-hand}

Salasanaa ei ole tallennettu minnekään, se **lasketaan** varmuuskopioiden avaimesta, joten voit toistaa sen itse pelkällä avaimella ja komentotulkilla:

```sh
printf 'bombvault:restic-repo'   | openssl dgst -sha256 -mac HMAC -macopt hexkey:$APP_KEY -r   | cut -d' ' -f1
```

Se on HMAC-SHA256 kiinteälle merkkijonolle `bombvault:restic-repo`, avaimena heksa-`APP_KEY`:n raakatavut, tulostettuna 64 pienenä heksamerkkinä. Sama arvo on paketissa, listattuna johdettuna restic-salasanana. Tämä on sitä päivää varten, kun paketti on jossain, missä et ole. Palvelimella, joka otti haltuunsa toisen varmuuskopiot, käytä haltuun otettua varmuuskopioavainta, joka on kyseisen palvelimen vanha APP_KEY.

[Isäntä](#backup-host) lasketaan samasta avaimesta ja merkkijonosta `bombvault:host`: `bv-` ja näistä merkeistä kaksitoista ensimmäistä.

```sh
echo "bv-$(printf 'bombvault:host' | openssl dgst -sha256 -mac HMAC -macopt hexkey:$APP_KEY -r | cut -c1-12)"
```

!!! warning "Vastaanotetussa arkistossa käytä LÄHETTÄVÄN instanssin avainta"
    Repository, joka saapui tänne etäkopion kautta, on sen lähettäneen koneen luoma **sen** avaimella. Johtaminen vastaanottavan laatikon avaimesta tuottaa salasanan, jonka restic hylkää, mikä näyttää täsmälleen vioittuneelta repositorylta eikä ole sitä. Tämä on tavallinen syy siihen, että `restic check` vastaanotetulla repolla kysyy salasanaa yhä uudelleen.

Koska palautusmääritykset asuvat kunkin repon **sisällä** (`<repo>/def`, `<repo>/vm-def`), kopioitu repokansio on täysin itsenäinen, joten paketti plus repo on kaikki mitä paljasrautainen palautus tarvitsee. Etäkopio ja noudettu kopio sisältävät tilannevedokset ilman määrityksiä: ne palauttavat tiedostoja, ja kontti tai VM luodaan uudelleen itsevarmuuskopiosta.

## Tietokantavedoksen hakeminen takaisin {#database-dumps}

Tietokantavedos on oma palautuspisteensä konttien repositoryssä, tunnisteella `dbdump:<kontti>`, ja sisältää yhden tiedoston `/dbdump/<kontti>.sql`. BombVault listaa, lataa ja tuo ne kontin **Varmuuskopiot**-välilehdellä. Alla ovat samat vaiheet pelkällä resticillä sitä päivää varten, kun BombVaultia ei ole.

```sh
restic -r <repo> snapshots --host <host> --host bombvault --tag dbdump:<container>
restic -r <repo> dump --host <host> --host bombvault --tag dbdump:<container> latest /dbdump/<container>.sql > <container>.sql
```

`<host>` on palautuspaketissa mainittu palvelimen [isäntä](#backup-host).

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
