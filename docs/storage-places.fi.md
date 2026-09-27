# Tallennuspaikat

Tallennuspaikka on paikka, jossa BombVault säilyttää varmuuskopioita: kansio tässä Unraidissa, jako NAS-laitteella, bucket pilvipalvelussa, rest-server, SFTP-tili tai Nextcloud. Yhdistät jokaisen paikan kerran kohdassa **Asetukset, Tallennustila**, ja sen tunnukset, säilytys, suojaus ja sijainti kuuluvat sille. Viisi toimialuetta (kontit, virtuaalikoneet, flash, BombVaultin omat asetukset ja tiedostojoukot) valitsevat sitten näistä paikoista, mihin kukin toimialue tallennetaan ja minne se kopioidaan.

## Paikan lisääminen {#add-a-place}

**Lisää paikka** avaa ikkunan, jossa on yksi ruutu kutakin palveluntarjoajaa kohden kolmessa ryhmässä: pilvitallennus, itse ylläpidetyt palvelut sekä NAS-laitteet ja tämä palvelin.

1. Valitse ruutu ja täytä sen lomake. Silmäpainike näyttää kirjoittamasi salaisuuden.
2. **Testaa yhteys** tarkistaa paikan eikä luo mitään. Jokaisen toimialueen kansiosta se kertoo, mitä löytyi: kansio on tyhjä tai sitä ei vielä ole, siinä on jo restic-arkisto, tai virhe, johon testi pysähtyi.
3. Anna paikalle nimi; palveluntarjoajan nimi on valmiiksi täytetty. Jos laite on itse ylläpitämäsi, vastaa kysymykseen **Missä laite on?**. Pilvipalvelut ovat aina toisessa rakennuksessa, ja kansio tässä Unraidissa on aina tässä rakennuksessa.
4. **Lisää** tallentaa paikan.

Uutta paikkaa ei vielä käytä mikään toimialue. Valitse se [Toimialueet-kortilla](#domains) kohdassa **Tallennuspaikka** tai **Kopiointikohteet**, tai kohteen kortilla vain sille kohteelle.

## Kansiot {#folders}

Paikassa on yksi kansio kutakin toimialuetta kohden: `container`, `vms`, `flash`, `config` ja `files`, samat nimet kuin oletusarvoisissa varmuuskopiosijainneissa. Kansiot näkyvät paikan tiedoissa, ja niitä voi nimetä siellä uudelleen (katso [Osoitteen muuttaminen](#addresses)). Toimialue, jolla ei ole kansiota paikassa, ei voi valita sitä paikkaa.

Kun toimialue käyttää samaa paikkaa kummassakin roolissa, toinen rooli saa päätteen ja ensimmäinen säilyttää kansionsa. Paikka, joka jo vastaanottaa toimialueen kopiot, tallentaa suoraan sinne lähetetyt kohteet kansioon `<folder>-direct`; paikka, johon toimialue jo tallennetaan, vastaanottaa sen kopiot kansioon `<folder>-copies`.

Jotkin paikat ovat itse restic-arkistoja: osoite, jossa oli jo arkisto paikkaa lisättäessä, olemassa olevan kokoonpanon nimetty arkisto tai bucketin juureen osoittava kopiointikohde. Tällaisella paikalla ei ole kansioita, kaikki toimialueet jakavat sen ainoan arkiston, eikä sille voi antaa toista roolia. Jos haluat tallentaa enemmän saman palveluntarjoajan luo, yhdistä toinen bucket tai kansio omaksi paikakseen.

## Paikan tiedot {#details}

Jokainen paikka on rivi, jossa näkyvät sen palveluntarjoaja, mihin sitä käytetään ja sen viimeisin testi tai kopio. **Testaa** tarkistaa paikan jokaisen osoitteen, ja **Tiedot** avaa sen asetukset. Jokainen muutos tiedoissa tallentuu heti, kun teet sen.

- **Yleiset**: nimi, kytkin, joka kytkee paikan päälle ja pois, osoite sekä itse ylläpitämälle laitteelle **Missä laite on?** (katso [Rakennuksen ulkopuolella](#off-the-premises)).
- **Säilytys**: säilytä viimeiset, päivittäin, viikoittain ja kuukausittain, jokaiselle paikan arkistolle. Uusi paikka alkaa oletussäännöillä; paikka, jonka jokainen sääntö on nolla, ei koskaan karsi mitään.
- **Suojaus**: **Append-only**-kytkin. Vastapuolen on itse valvottava append-only-tilaa; kun kytkin on päällä, BombVault ei koskaan karsi eikä poista siellä mitään. Kun rest-serverissä on append-only päällä, **Testaa append-only** ajaa peukalointitestin jokaista paikan toimialuepolkua, päälle kytkettyä kopiota ja arkistoa vastaan ja näyttää koko paikalle yhden vastauksen, *poistot torjutaan* tai *poistot sallitaan* (katso [Etäsijainti ja palautus](offsite-recovery.md)). Tämä osio on vain etäpaikoilla, koska mikään tällä koneella ei voi estää paikallisen arkiston poistamista.
- **Pääsy**: tunnukset ja S3:lle tallennusluokka. Paikka, joka käyttää jaettuja tunnuksia, saa ensimmäisellä muutoksella omat tunnuksensa. Paikan suora arkisto, jota uudet tunnukset eivät avaa, säilyttää vanhat, ja vastaus kertoo sen. Kansio-, SFTP- ja rclone-paikoilla tätä osiota ei ole.
- **Rajoitukset**: lähetys- ja latausnopeus sekä kasvubudjetti.
- **Kansiot**: yksi kytkin kutakin toimialuetta kohden, sen kansion nimen kera. Toimialue, joka on kytketty tässä pois, ei voi valita paikkaa.

Säilytyksen pienentäminen kysyy ensin ja kertoo, kuinka moneen kohteeseen se vaikuttaa; append-only-tilan kytkeminen pois kysyy ensin ja kertoo, kuinka moni paikan arkisto menettää sen. Paikan kytkeminen pois kytkee pois jokaisen sen arkiston; paikkaa, johon jokin toimialue tallennetaan, ei voi kytkeä pois.

## Toimialueet-kortti {#domains}

Kortilla on yksi rivi kutakin toimialuetta kohden: sen aikataulu, mihin se tallennetaan, minne se kopioidaan ja sen poikkeukset.

- **Tallennuspaikka**: niin kauan kuin toimialueen varmuuskopiosijainnissa ei ole varmuuskopioita, valitusta paikasta tulee toimialueen kotipaikka ja sijainti siirtyy sinne. Kun siellä on varmuuskopioita, konttien, virtuaalikoneiden ja tiedostojoukkojen valinnasta tulee uusien kohteiden oletus, jonka ne ottavat ensimmäisessä varmuuskopiossaan; kohteet, joilla on jo varmuuskopioita, pysyvät paikallaan, koska BombVault ei koskaan siirrä varmuuskopiota. Flashin ja BombVaultin omien asetusten kotipaikka siirtyy, ja jo kirjoitetut varmuuskopiot jäävät vanhaan paikkaan.
- **Kopiointikohteet**: yksi chip jokaista paikkaa kohden, joka voi ottaa vastaan toimialueen kopioita. Chipin rastittaminen tekee paikasta toimialueen kopiointikohteen; ensimmäisellä kerralla BombVault kertoo etukäteen, kuinka monta kohdetta ja tilannevedosta ja kuinka paljon dataa ensimmäinen ajo lähettää. Rastin poistaminen lopettaa uudet kopiot: siellä jo olevat kopiot jäävät ja vanhenevat paikan säilytyksen mukaan, ja kohteet, joilla on oma valinta, kopioivat sinne edelleen. Viimeisen rastin poistaminen lopettaa kaikki kopiot, myös myöhemmin lisättyihin paikkoihin, kunnes jokin rastitetaan taas. Pois päältä oleva paikka näkyy himmennettynä chipinä, eikä sitä voi valita.
- **Poikkeukset**: kohteet, joilla on oma valinta, luettelona linkkeineen niiden kortteihin.
- **Kopioi nyt** ajaa toimialueen kopioinnin heti.

Toimialue, joka on keskeytetty Tunnista-toiminnolla tehdyn uudelleenrakennuksen jälkeen, näyttää keskeytyksen rivillään yhdessä **Vahvista oletus** -painikkeen kanssa (katso [Sijoittelu per kohde](offsite-recovery.md#placement)).

## Osoitteen muuttaminen {#addresses}

Toimialueen kansiota voi muuttaa paikan tiedoissa, samoin paikallisen paikan osoitetta, esimerkiksi kun arkisto on siirretty käsin toiselle levylle. BombVault testaa jokaisen osoitteen, johon muutos vaikuttaa, ja hyväksyy muutoksen, kun jokainen uusi osoite on tyhjä eikä vanhaan ole tallennettu mitään, tai kun jokaisessa uudessa osoitteessa on sama restic-arkisto kuin vanhassa. Kaikki muu torjutaan, ja vastaus kertoo, montako varmuuskopiota vanhassa osoitteessa yhä on. Etäpaikka säilyttää osoitteensa; jos haluat varmuuskopioida muualle, yhdistä se omaksi paikakseen.

BombVault kokoaa paikkojen luettelon omasta tietokannastaan eikä koskaan listaa etäarkistoa sitä varten; testi ajetaan vain, kun muutat jotain.

## Paikan poistaminen {#remove}

Paikan voi poistaa vain, kun mikään ei käytä sitä: mitään toimialuetta ei tallenneta sinne, mikään oletus ei osoita siihen, mitään kohdetta ei tallenneta sinne, eikä yhdessäkään sen suorassa arkistossa ole kohteita. Muuten kieltäytyminen luettelee, mikä pitää paikan käytössä. Poisto vie mukanaan paikan kopiointikohteet sekä sen omat tunnukset, ellei jokin noutolähde tai toinen paikka käytä niitä. Itse tallennustilasta ei poisteta mitään, ja vahvistus kertoo, montako kopiota sinne jää.

## Ilman paikkaa {#without-a-place}

Osoite, joka ei sovi muotoon paikka plus kansio, toimii edelleen ja näkyy osoitteineen kohdassa **Ilman paikkaa**. Natiivit `b2:`-, `gs:`- ja `swift:`-osoitteet kuuluvat näihin. **Liitä paikkaan** liittää tällaisen rivin paikkaan saman testin jälkeen kuin [osoitteen muuttamisessa](#addresses). Kopiointikohde ilman paikkaa mainitaan myös toimialueensa rivillä chipien vieressä, ja se jatkaa kopiointia. Etärivillä on siellä oma **Append-only**-kytkimensä, ja sen kytkeminen pois kysyy ensin ja kertoo, kuinka moni kohde säilyttää varmuuskopioita siinä osoitteessa. Suora arkisto seuraa kohteensa kytkintä.

## Rakennuksen ulkopuolella {#off-the-premises}

Kysymykseen **Missä laite on?** on kaksi vastausta: **Tässä rakennuksessa** ja **Toisessa rakennuksessa**. Kopio lasketaan rakennuksen ulkopuoliseksi korttien 3-2-1-rivillä ja Kojelaudan etätarkistuksissa vain, kun sen paikka on toisessa rakennuksessa. Toinen levy tai NAS samassa rakennuksessa on toinen kopio, mutta ei kopio rakennuksen ulkopuolella. Vastaus ei muuta yhtään kopiota. Pilvipalvelut ovat aina toisessa rakennuksessa ja kansio tässä Unraidissa aina tässä rakennuksessa, joten lomake ei kysy niistä; kaikille muille paikoille vastauksen voi muuttaa paikan tiedoissa. Toisessa rakennuksessa olevan paikan rivillä on merkintä **Rakennuksen ulkopuolella**.

## Yhteystyypit

### Kansio tässä Unraidissa tai NAS-laitteella {#kind-local}

Osoite on polku hakemiston `/mnt` alla ilman alkuosaa `/mnt`, esimerkiksi `user/bombvault`, ja kunkin toimialueen kansio on sen alla: `user/bombvault/container`.

- **Kansio tässä Unraidissa** valitsee jaoista, levyistä ja pooleista.
- **Synology**, **QNAP**, **TrueNAS**, **Toinen Unraid** ja **Muu jako** valitsevat hakemistosta `/mnt/remotes`. Liitä jako ensin Unraidiin, esimerkiksi Unassigned Devices -laajennuksella. Host Data on liitettävä tilassa Read/Write - Slave, muuten jako, joka liitetään BombVaultin käynnistymisen jälkeen, pysyy näkymättömänä uudelleenkäynnistykseen asti (katso [Asetukset](configuration.md)).

Kansiovalitsin luo kansion nykyiseen kohtaansa painikkeella **Uusi kansio**. Testi tarkistaa, että kansio on tyhjä tai puuttuu ja että BombVault voi kirjoittaa sinne.

### S3 {#kind-s3}

Osoite on `s3:https://<endpoint>/<bucket>/<path>`, esimerkiksi `s3:https://s3.eu-central-003.backblazeb2.com/tower-backups/bombvault`.

- **Backblaze B2** tarvitsee vain avaintunnuksen ja sovellusavaimen. BombVault kysyy B2:lta, mihin bucketiin, S3-päätepisteeseen ja kansioon avain on rajattu, ja rakentaa osoitteen niistä. Jos avain pääsee kaikkiin bucketeihin, sen bucketit tarjotaan valittaviksi.
- **Amazon S3**, **Cloudflare R2**, **Wasabi**, **Hetzner Object Storage**, **Storj**, **IDrive e2**, **Scaleway**, **OVHcloud**, **DigitalOcean Spaces**, **IONOS**, **Contabo**, **Exoscale** ja **Vultr** kysyvät avainta ja, jos palveluntarjoaja sitä tarvitsee, aluetta, tilin tunnusta tai päätepistettä. BombVault täyttää päätepisteen ja listaa bucketit, jos avain saa listata ne; muuten kirjoita bucketin nimi.
- **Google Cloud Storage** toimii S3-rajapintansa kautta HMAC-avaimella, joka luodaan Cloud Storagen asetuksissa kohdassa Interoperability. Palvelutilin tiedosto ei toimi tässä.
- **MinIO**, **SeaweedFS**, **Garage**, **Ceph**, **JuiceFS**, **RustFS**, **Versity S3 Gateway** ja **Muu S3-palvelu** ottavat palvelun osoitteen ja avaimen.

Tallennusluokka asetetaan paikan tiedoissa, ja se on rajattu tasoihin, joilta palautus voi lukea ilman sulatusta.

### rest-server {#kind-rest}

Osoite on `rest:<url>/<user>`, esimerkiksi `rest:https://nas.lan:8000/tower`. Lomake kysyy palvelimen osoitetta, käyttäjää ja salasanaa. Valitsimella `--private-repos` käyttäjä pääsee vain polkuihin, jotka alkavat sen omalla nimellä, joten BombVault laittaa käyttäjän polun alkuun, ellet kirjoita muuta polkua. Kun palvelin torjuu polun käyttäjän oman polun ulkopuolella, virheilmoitus kertoo sen.

rest-server-lomakkeessa on valmis liitettävä ohje append-only-tilassa toimivalle rest-serverille, jossa on yksi käyttäjä tälle BombVaultille. **Näytä ohje** luo salasanan, joka näytetään kerran, ja antaa `docker run` -rivin, compose-tiedoston ja Unraid-mallin, kukin palvelimelle vietävän `htpasswd`-rivin kera; käyttäjä ja salasana menevät suoraan lomakkeeseen.

**Toinen BombVault** listaa omien kenttiensä yläpuolella avoimet tarjoukset, jotka muut instanssit ovat lähettäneet Laivueen kautta. Tarjouksen hyväksyminen lisää paikan, joka säilyttää vain tarjotun toimialueen kopiot, koska tarjous sisältää käyttäjän vain sille yhdelle toimialueelle. Hyväksyminen Laivue-sivulla lisää saman paikan.

### SFTP {#kind-sftp}

Osoite on `sftp://<user>@<host>:<port>/<path>`, esimerkiksi `sftp://bv@backup.lan:22/bombvault`. Lomake kysyy isäntää, porttia ja käyttäjää ja näyttää BombVaultin julkisen avaimen. Lisää tämä avain palvelimella käyttäjän tiedostoon `~/.ssh/authorized_keys`; muuta ei tarvitse asentaa. BombVault hyväksyy palvelimen isäntäavaimen ensimmäisellä yhteydellä ja tarkistaa sen siitä lähtien.

**Hetzner Storage Box** täyttää osoitteen `<user>.your-storagebox.de` ja portin 23. Asenna avain boxiin Hetznerin omalla komennolla, joka kysyy boxin salasanaa kerran:

```sh
echo '<public key>' | ssh -p 23 <user>@<user>.your-storagebox.de install-ssh-key
```

### WebDAV: Nextcloud, ownCloud, OpenCloud {#kind-webdav}

Lomake kysyy palvelimen osoitetta, käyttäjää ja sovellussalasanaa. Luo sovellussalasana tilin tietoturva-asetuksissa ja syötä käyttäjätunnus eikä sähköpostiosoitetta. BombVault rakentaa tuotteen käyttämän WebDAV-polun ja välittää yhteyden resticille rclonen ympäristömuuttujien kautta, salasana rclonen hämärretyssä muodossa. Osoite on muotoa `rclone:bvp<id>:<path>`, jossa `bvp<id>` on etäsijainti, joka on olemassa vain siinä ympäristössä; rclone-määritykseen ei kirjoiteta mitään.

### Azure Blob {#kind-azure}

Osoite on `azure:<container>:/<path>`. Lomake kysyy tallennustiliä ja sen käyttöavainta; **Testaa yhteys** -toiminnon jälkeen se listaa tilin säilöt valittaviksi, tai voit kirjoittaa säilön nimen. BombVault välittää tilin ja avaimen resticille muuttujina `AZURE_ACCOUNT_NAME` ja `AZURE_ACCOUNT_KEY`.

### rclone {#kind-rclone}

Osoite on `rclone:<remote>:<path>`. Lomake listaa BombVaultin rclone-määrityksen etäsijainnit valittaviksi. Jos haluat korvata määrityksen, liitä koko `rclone.conf` kohtaan **rclone-määritys** ja napsauta **Tallenna määritys**. Määritys tallennetaan heti ja palvelee jokaista rclone-paikkaa, lisäsipä ikkuna sen jälkeen paikan tai ei.

## Kopiot paikkojen välillä eri tunnuksilla {#different-credentials}

Etäpaikkaan tallennettu toimialue on kopioidensa lähde. `restic copy` ajetaan yhdellä ympäristöllä, ja BombVault lisää lähteen tunnukset kohteen tunnuksiin, kun ne eivät aseta samaa muuttujaa eri arvoihin. Nextcloud-paikka ja B2-paikka käyttävät eri muuttujia, joten Nextcloudiin tallennetun toimialueen voi kopioida B2:een. Kaksi S3-tiliä tai kaksi rest-server-käyttäjää tarvitsisivat samat muuttujat eri arvoilla; restic ei voi ottaa molempia, ja Toimialueet-kortin chip kertoo, etteivät tunnukset sovi yhteen.
