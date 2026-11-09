# Varmuuskopiot ja merkinnät

Kaikki, minkä BombVault varmuuskopioi, on yhdessä listassa kohdassa **Varmuuskopiot**: kontit, virtuaalikoneet, kansiojoukot, flash-asema, ZFS-tietojoukot ja itsevarmuuskopio. Jokainen niistä on **merkintä**, jolla on oma sivunsa. Tällä sivulla käydään läpi lista, merkinnän sivu ja sen kolme välilehteä sekä asetukset, joita merkinnällä voi olla.

![Varmuuskopiot-lista, Kontit-ruutu valittuna: jokaisella rivillä on viimeiset ajot nauhana ja yksi tila.](assets/screenshots/backups/list-dark.png#only-dark)
![Varmuuskopiot-lista, Kontit-ruutu valittuna: jokaisella rivillä on viimeiset ajot nauhana ja yksi tila.](assets/screenshots/backups/list-light.png#only-light)

## Mitä kukin osio tallentaa {#what-is-saved}

| Osio | Mitä tallennetaan |
|---|---|
| **Kontit** | Appdata-kansio sekä kontin määritys: image, muuttujat, portit, tunnisteet ja taltiot. Kontin **Asetukset**-välilehden **Varmuuskopioitavat kansiot** -kortti valitsee tarkasti, mitkä kansiot varmuuskopio kattaa, ja näyttää polkujen määrän reaaliajassa sekä listan siitä, minkä jätit pois. Dataa, jonka kontti pitää Host Datan ulkopuolella, esimerkiksi TrueNASin ixVolumessa tai Compose-projektissa `/opt`-hakemiston alla, ei varmuuskopioida, ja ajo nimeää kansiot, jotka se jätti pois. Ajo nimeää myös Host Datan sisällä olevan kansion, jota ei juuri nyt ole. Järjestelmäpolkuja, kuten `/etc/localtime` tai Dockerin socketia, ei lasketa dataksi. |
| **VMs** | Levykuvat, XML-määritys ja UEFI NVRAM, hallitulla sammutuksella tai live-tilannekuvalla, SSH:n yli. Katso [VM-varmuuskopiot](vm-backup-ssh-setup.md). |
| **Flash** | Vain Unraid. Koko USB-flash (`/boot`): käyttöjärjestelmä, lisenssi, arrayn asetukset, jaot, verkko ja liitännäisten asetukset. |
| **Itsevarmuuskopio** | BombVaultin oma `/config`: asetustietokanta, kohteiden tunnukset, libvirt-SSH-avainpari, ZFS-palvelinten avain ja kiinnitetyt isäntäavaimet sekä varmenteet, otettu SQLiten `VACUUM INTO` -komennolla, jotta tietokantaa ei koskaan kopioida kesken kirjoituksen. |
| **Kansiot** | Mikä tahansa palvelimen kansio kansiojoukkona, esimerkiksi jako, asiakirjasi tai kuvakirjasto, valinnaisin poissulkumallein. Kansiojoukoilla on aikataulut, säilytys, etäkopiot, tarkistukset ja palautusharjoitukset kuten kaikella muullakin. |
| **ZFS-tietojoukot** | Tietojoukko ja jokainen sen alla oleva tietojoukko luettuna yhdestä ZFS-tilannevedoksesta, jotta kaikki ovat samalta hetkeltä, ja tallennettuna resticillä kuten kansio. ZFS-merkinnällä voi myös olla lohkotarkka replika toisella ZFS-palvelimella. Katso [ZFS-tietojoukot](zfs-datasets.md). |

Jokainen varmuuskopio kulkee resticin kautta, joten se on inkrementaalinen ja deduplikoitu, eivätkä edes suuret virtuaalikoneiden levyt paisuta repoa. BombVault ei koskaan varmuuskopioi omaa konttiaan, joten se ei myöskään pysäytä sitä. Ajon voi pysäyttää turvallisesti, koska restic kirjoittaa tilannevedoksensa viimeisenä, joten keskeytetty ajo jättää jälkeensä viittaamatonta dataa mutta ei tilannevedosta. Lukko, joka jäi uudelleenkäynnistyksestä kesken ajon, puretaan ja ajoa yritetään vielä kerran, mutta vasta kun lukkoa ei ole päivitetty kymmeneen minuuttiin.

## Lista {#list}

Listan yläpuolella olevat ruudut valitsevat lajin, kuten **Kontit** tai **ZFS**, ja jokainen ruutu laskee merkintänsä. Ruutu valitsee, se ei piilota muuta sivua.

Jokainen rivi näyttää merkinnän neljätoista viimeistä ajoa pieninä neliöinä ja yhden tilan, kuten **Suojattu**, **Ei suojattu**, **Lykätty** tai **Replikoidaan**. Kaikki muu siitä on nimen vieressä olevassa tietokuplassa, ja kun osoitin on rivillä, teksti näkyy itse rivillä. Kontin nimen vieressä oleva merkki kertoo, että kontti luotiin uudelleen toisella imagella, toisilla porteilla, muuttujilla tai taltioilla sen viimeisen varmuuskopion jälkeen. Se katoaa seuraavalla varmuuskopiolla.

Kolme listan yläpuolella olevaa painiketta rajaavat sitä:

- **Suodattimet**: kohdilla **Aikataulu** (**Kaikki**, **Aikataulutettu**, **Tauolla**), **Varmuuskopioitu** (**Varmuuskopioitu**, **Ei koskaan varmuuskopioitu**) ja kohdassa **Kontit ja VM:t** **Asennetut** tai **Ei asennettu**.
- **Lajittelu**: nimen, tilan, IP:n, viimeisen varmuuskopion, koon tai aikataulun mukaan. Jokainen kenttä paitsi IP lajittelee molempiin suuntiin, esimerkiksi **Koko, suurin ensin** ja **Koko, pienin ensin**.
- **Hae**: nimen, imagen tai IP:n mukaan.

Merkinnät, joita ei enää ole asennettuna mutta joilla on vielä varmuuskopioita, ovat erillään kohdassa **Ei asennettu (vain varmuuskopiot)**. Palauta ne tai poista niiden varmuuskopiot vapauttaaksesi tilan.

Sivun pääasiallinen toiminto leijuu oikeassa alakulmassa: **Varmuuskopioi kaikki nyt** käynnistää ajon kaikille päällä oleville osioille. **Lisää** sisältää loput:

- **Valitse** aloittaa valinnan, katso alla.
- **Tunnista varmuuskopiot** lukee, mitä kohteet sisältävät, ja rakentaa merkinnät siitä uudelleen, esimerkiksi uuden asennuksen jälkeen. Kontilla, jolla ei ole omaa dataa, ei ole tilannevedosta, ja se palautuu repon viereen peilatusta määrityksestä. Uudelleenrakennettujen osioiden etäkopiot odottavat sitten, kunnes otat niiden varmuuskopiot tämän palvelimen omiksi, katso [Uudelleenrakennuksen jälkeen](offsite-recovery.md#after-a-rebuild).
- **ZFS-yhteys tähän palvelimeen** avaa ZFS-osion SSH-testin, katso [ZFS-tietojoukot](zfs-datasets.md#requirements).
- **Replikoi kaikki nyt**, ZFS-ruudun ollessa valittuna, lähettää jokaisen ZFS-merkinnän replikan.

![Sama lista puhelimella.](assets/screenshots/backups/list-phone-dark.png#only-dark)
![Sama lista puhelimella.](assets/screenshots/backups/list-phone-light.png#only-light)

## Useiden merkintöjen valinta {#select}

Valitse rivejä samalla tavalla kuin tiedostoja tiedostonhallinnassa: Ctrl+napsautus, Shift+napsautus, Ctrl+A tai pitkä painallus puhelimella. Valitulla rivillä on valintamerkki kuvakkeen paikalla, ja listan yläpuolella oleva palkki laskee ne ja tarjoaa vaihtoehdot **Varmuuskopioi valitut**, **Keskeytä aikataulu** ja **Jatka aikataulua**, **Valitse kaikki** ja **Tyhjennä valinta**. Jokaisella rivillä on myös oikean napsautuksen valikko omine toimintoineen.

Valinta varmuuskopioidaan palvelimella, yksi merkintä kerrallaan, joten se jatkuu, vaikka suljet välilehden. Useiden merkintöjen palauttamiseen yhdessä käytä [Palautus-sivua](restore.md#several).

![Kaksi konttia valittuna, palkki listan yläpuolella.](assets/screenshots/backups/bulk-dark.png#only-dark)
![Kaksi konttia valittuna, palkki listan yläpuolella.](assets/screenshots/backups/bulk-light.png#only-light)

## Kansion tai ZFS-tietojoukon lisääminen {#add}

Kontit, virtuaalikoneet ja flash-asema ilmestyvät itsestään, kun niiden osio on kytketty päälle kohdassa **Asetukset, Yleiset**. Kansiot ja ZFS-tietojoukot valitset itse, koska BombVault ei voi arvata, mitä tarkoitat. **Lisää** avaa **Mitä muuta BombVaultin pitäisi varmuuskopioida?**:

- **Yksi kansio** listaa jaot, jotka BombVault löysi, ja ottaa minkä tahansa muun kansion Host Datan alta. Anna kansiojoukolle nimi, esimerkiksi `Documents`. Nimi lukittuu ensimmäisen varmuuskopion jälkeen, koska tilannevedokset tallennetaan sen alle.
- **ZFS-tietojoukko** listaa tämän palvelimen tietojoukot. Valitse se, joka on varmuuskopioitavan yläpäässä, katso [Merkinnät ja alatietojoukot](zfs-datasets.md#items-and-children).
- **Järjestelmän asetukset** ilmestyy isännälle ilman Unraidia, jonka Host Data yltää kansioon `/etc`. Se on `/etc`-kansiojoukon aloituspohja, jonka tarkistat ennen tallennusta.

## Merkinnän jättäminen pois {#dont-back-up}

Merkinnän, jonka jätät tarkoituksella pois, esimerkiksi testikontin, voi merkitä kohtaan **Älä varmuuskopioi**. Se lakkaa silloin olemasta suojaamaton, ja **Älä enää sulje pois** tuo sen takaisin. **Keskeytä aikataulu** on pehmeämpi tapa: merkintä poistuu aikataulusta ja täysvarmuuskopiosta, saa harmaan **Aikataulu keskeytetty** -merkin, ja **Varmuuskopioi nyt** toimii edelleen. Se asettaa saman kytkimen kuin **Aikataulussa** merkinnän Asetukset-välilehdellä.

## Merkinnän sivu {#entry-page}

Rivin napsautus avaa merkinnän sivun. Pää nimeää merkinnän ja sen tilan yhdellä rivillä, esimerkiksi **Viimeksi varmuuskopioitu 3 tuntia sitten** tai **Lykätty, seuraava yritys kello 03.15**, ja sisältää sen toiminnot: **Varmuuskopioi nyt**, **Palauta** ja kohdassa **Lisää** harvinaisemmat, kuten **Vie uusin varmuuskopio**, **Vie kontti** tai **Vie VM**, **Yhdistä aiempaan merkintään**, **Poista kaikki varmuuskopiot**, **Poista merkintä** ja **Poista varmuuskopiot kaikkialta**. Takaisin-painike kuvakkeen vasemmalla puolella johtaa listaan. Kun varmuuskopiointi on käynnissä, **Peruuta varmuuskopiointi** on edistymisen vieressä, ja peruutettu ajo kirjataan peruutetuksi, ei epäonnistuneeksi.

**Poista varmuuskopiot kaikkialta** poistaa merkinnän varmuuskopiot kerralla jokaisesta kohteesta, myös etäkopiot. Ikkuna näyttää jokaisen paikan ja sen, montako varmuuskopiota siellä on, ja painike avautuu, kun kirjoitat merkinnän nimen. Append-only-tilaan asetettu paikka säilyttää varmuuskopionsa ja kertoo sen. Merkintä ja sen asetukset säilyvät, joten merkintä, jota et enää varmuuskopioi, pysyy sellaisena, ja vain sen varmuuskopiot ja niiden historia katoavat.

Sivulla on kolme välilehteä: **Varmuuskopiot**, **Asetukset** ja **Tarkista**.

### Varmuuskopiot-välilehti {#backups-tab}

Tämä välilehti listaa merkinnän tilannevedokset, uusin ensin, päivää kohti yksi palkki. Varmuuskopio, joka on useassa paikassa, näkyy kerran, ja siinä on merkintä jokaisesta kohteesta, joka sen sisältää. Tilannevedoksen napsautus palauttaa sen, ja tilannevedoksen sisällä olevan paikan napsautus palauttaa sen täsmälleen sieltä.

- **Kohteet** nimeää jokaisen paikan, jossa on tämän merkinnän varmuuskopioita, ja kuinka monta. Etäpaikat luetaan vasta, kun painat **Tarkistus**, koska pilvitallennustilan listaaminen vie hetken. Paikka, johon merkintä ei enää varmuuskopioi, näkyy nimellä **Vain aiemmat varmuuskopiot**, ja siitä voi edelleen palauttaa.
- Jokaisella tilannevedoksella on valikko: **Lisää tunniste** tai **Vaihda tunniste**, **Vertaa toiseen varmuuskopioon**, joka listaa tulleet, menneet tai muuttuneet tiedostot, **Vie** tar-, zip- tai oman restic-repon muotoon, sekä poisto vain yhdestä paikasta.
- Kontin **Tietokantavedokset** ovat omassa listassaan, ja niissä on lataus, tallennus kansioon ja tuonti.

Kuinka monta tilannevedosta säilyy, asetetaan kohteessa, katso [Kuinka kauan säilytetään](offsite-recovery.md#keep).

![Kontin Varmuuskopiot-välilehti: palkki päivää kohti ja kohteet, joissa kukin tilannevedos on.](assets/screenshots/backups/entry-backups-dark.png#only-dark)
![Kontin Varmuuskopiot-välilehti: palkki päivää kohti ja kohteet, joissa kukin tilannevedos on.](assets/screenshots/backups/entry-backups-light.png#only-light)

### Asetukset-välilehti {#settings-tab}

Jokainen merkintä alkaa kortilla **Aikataulu ja kohde**:

- **Aikataulussa** ottaa merkinnän mukaan ajastettuihin ajoihin tai pois niistä.
- **Oma aikataulu** antaa tälle yhdelle merkinnälle oman aikataulun, katso [Oma aikataulu yhdelle merkinnälle](schedules.md#own-plan). Ilman sitä se ajetaan osionsa aikataululla tai täysvarmuuskopiossa.
- Painikerivi, **Paikallinen** ja yksi jokaista etäkohdetta kohti, kertoo, minne varmuuskopiot menevät. **Noudattaa oletusta** tarkoittaa, että merkintä menee sinne, minne sen osiokin, ja sen napsautus avaa **Kohteet osioittain**. Katso [Sijoittelu merkinnän mukaan](offsite-recovery.md#placement).
- **Arkisto** nimeää paikan, johon varmuuskopiot kirjoitetaan (**Tallennuspaikka**). Tämä paikka on lukittu ensimmäisestä varmuuskopiosta alkaen, kopiot voivat muuttua milloin tahansa.

Alla olevat kortit riippuvat merkinnän lajista:

| Laji | Kortit |
|---|---|
| Kontti | **Varmuuskopioitavat kansiot** (mitkä liitetyt kansiot varmuuskopioidaan, kohdalla **Ohita välimuistikansiot (CACHEDIR.TAG)**), **Poissulkumallit**, **Pysäytä muut säiliöt**, **Komennot ennen ja jälkeen**, **Tietokanta**, **Kun sovellus on kiireinen**, **Päivitä onnistuneen varmuuskopioinnin jälkeen**, **Tilannevedos pysäytyksen sijaan**, **Aiemmat nimet** |
| VM | **VM** kohdilla **Menetelmä** (**Hallittu (sammutus)** tai **Live-tilannekuva**) ja **Vain muuttuneet lohkot**, katso [VM-varmuuskopiointiopas](vm-backup-ssh-setup.md#per-vm-method) |
| Kansiojoukko | **Kansio** ja **Poissulkumallit** |
| Flash-asema | **Flashin zip-vienti**, joka kirjoittaa jokaisen flash-varmuuskopion tavallisena `.zip`-tiedostona valitsemaasi kansioon yhdellä kertaa, jotta synkronointityökalu ei koskaan näe puolikasta tiedostoa. Jos flash-varmuuskopiossa on vähemmän kuin pitäisi, viimeisin ZIP jää paikalleen, ja viennin ajo kertoo sen. |
| ZFS-tietojoukko | alatietojoukot, pysäytettävät kontit ja tilannevedoksen ympärillä olevat komennot, **Replika** ja **Kokeile pääsyä tilannevedoksiin**, katso [ZFS-tietojoukot](zfs-datasets.md) |

Asetukset tallentuvat itsestään, kun muutat niitä.

![Kontin Asetukset-välilehti, Oma aikataulu ensimmäisessä kortissa.](assets/screenshots/backups/entry-settings-dark.png#only-dark)
![Kontin Asetukset-välilehti, Oma aikataulu ensimmäisessä kortissa.](assets/screenshots/backups/entry-settings-light.png#only-light)

### Tarkista-välilehti {#verify-tab}

- **Tarkistukset** näyttää palautustarkistuksen, joka seuraa ensimmäistä varmuuskopiota. **Tarkista palautus** ajaa sen uudelleen uusimmalle varmuuskopiolle: BombVault palauttaa enintään 100 tiedoston ja 256 MiB:n otoksen väliaikaiseen kansioon ja antaa resticin lukea jokaisen tiedoston takaisin tiivisteitään vasten.
- **Käynnistystesti** kontilla palauttaa uusimman varmuuskopion eristettyyn kopioon ja käynnistää sen: nimi alkaa `bombvault-test-`, sillä on oma sisäinen Docker-verkkonsa ilman julkaistuja portteja ja ilman tietä lähiverkkoon, 1 suoritin ja 2 GiB muistia. Testi läpäistään, kun kopio raportoi olevansa kunnossa, vastaa ensimmäisellä avatulla portillaan tai, jos kumpaakaan ei ole, pysyy käynnissä. Alkuperäiseen ei koskaan kosketa, ja kopio, sen verkko ja sen data poistetaan jälkeenpäin, myös silloin kun uudelleenkäynnistys tulee kesken testin. Isäntäverkossa olevat kontit, etuoikeutetut, laitteita käyttävät ja toista konttia tarvitsevat näytetään testaamattomina. **Käynnistystesti** kohdassa **Asetukset, Tarkista, Automaattiset tarkistukset** testaa yhden kontin ajoa kohti, sen, joka on testattu kauimmin sitten, ensin.
- **Poikkeamat** asettaa tämän merkinnän oman herkkyyden ja ilmoitustason ja näyttää sen avoimet havainnot.
- **Koko kansioittain** näyttää, mikä vie tilaa uusimmassa varmuuskopiossa ja kuinka paljon siitä viimeisin varmuuskopio toi mukanaan, yksi taso kerrallaan.
- **Viimeisimmät ajot** listaa viimeiset ajot alkuineen, loppuineen ja kestoineen. Napsautus näyttää tiedot.

![Kontin Tarkista-välilehti, Koko kansioittain auki.](assets/screenshots/backups/entry-verify-dark.png#only-dark)
![Kontin Tarkista-välilehti, Koko kansioittain auki.](assets/screenshots/backups/entry-verify-light.png#only-light)

## Poissulkumallit {#excludes}

**Poissulkumallit**-kortti ottaa yhden mallin riviä kohti. Kirjoita polut sellaisina kuin ne näet kontin sisällä, esimerkiksi `/config/Library/Application Support/Plex Media Server/Cache`, tai pelkkä nimi, kuten `.git`, joka täsmää millä tahansa syvyydellä. Kansiojoukolle kirjoitetaan polku sellaisena kuin isäntä sen tuntee, esimerkiksi `/mnt/user/data/cache`. Kentän alla oleva esikatselu näyttää, mihin kukin rivi johtaa, ja varoittaa, kun rivi ei täsmää mihinkään tai sulkisi pois kaiken. Rivi `/` tai `*` tai rivi, joka nimeää kokonaisen varmuuskopioitavan kansion, ohitetaan, joten poissulku ei voi koskaan tyhjentää varmuuskopiota. Jättääksesi kokonaisen kansion pois poista sen valinta kohdassa **Varmuuskopioitavat kansiot**.

Tunnetuille imageille, kuten Plex, Jellyfin, Sonarr, Radarr, Immich ja Nextcloud, kortti tarjoaa kansiot, jotka sovellus täyttää itse uudelleen, kuten välimuistit, lokit ja pikkukuvat. Mitään ei suljeta pois, ennen kuin painat **Sulje valitut pois**.

Uusi kansiojoukko alkaa **Järjestelmäroska**-ryhmä päällä: `.DS_Store`, `Thumbs.db`, `desktop.ini`, `@eaDir` (Synology), `.Recycle.Bin` (Unraidin Recycle Bin -lisäosa), `.Trash-*` ja `*.tmp`. Jokainen niistä on pelkkä nimi, joten se täsmää millä tahansa syvyydellä kansion alla, ja voit kytkeä minkä tahansa niistä pois ennen kuin luot joukon. Olemassa oleva kansiojoukko säilyttää poissulkunsa. Sen muokkausikkunassa **Sulje järjestelmäroska pois** tarjoaa samat rivit, ja ne päätyvät joukkoon vasta, kun painat **Sulje valitut pois** ja tallennat.

Samalla kortilla on varoituksia, joita kansioiden skannaus ei näe. Immich esimerkiksi pitää albuminsa, kasvonsa ja päivämääränsä PostgreSQL-tietokannassa erillisessä kontissa, joten pelkän Immich-kontin tiedostovarmuuskopio palauttaa kuvat ilman mitään tästä, ja palautus näyttää onnistuneen. Varmuuskopioi myös tietokantakontti.

## Tietokantavedokset {#database-dumps}

PostgreSQL-, MySQL- ja MariaDB-kontit, jotka BombVault tunnistaa, vedostetaan ennen jokaista varmuuskopiota käynnissä olevalta palvelimelta omaksi palautuspisteekseen tiedostojen viereen. Se kattaa viralliset imaget, PostGISin, TimescaleDB:n, pgvectorin, pgautoupgraden, Immichin tietokantaimaget, linuxserverin, yobasystemsin ja jc21:n MariaDB-imaget sekä Oraclen mysql-serverin. Kontti, joka vain näyttää tietokannalta, saa saman valinnan, kytkettynä pois, kunnes valitset sen.

Vedos virtaa suoraan repoon eikä sitä koskaan kirjoiteta levylle. Tunnukset tulevat kontin omista muuttujista, `*_FILE`-salaisuudet mukaan lukien, eivätkä koskaan jätä sitä. Kontin Asetukset-välilehden **Tietokanta**-kortti kertoo, tallennetaanko datakansio pysäytettynä, kopioidaanko se käynnissä vai jääkö se tallentamatta, ja kytkee vedoksen päälle tai pois tälle kontille. Tunniste `bombvault.dbdump=false` tekee saman ja voittaa kortin. **Automaattiset tietokantavedokset** kohdassa **Asetukset, Yleiset** kytkee sen kaikille.

Epäonnistunut vedos ei kaada varmuuskopiota. Se näkyy omana epäonnistuneena ajonaan syineen ja lähettää ilmoituksen. Vedoksia ei koskaan ladata takaisin itsestään: lataa yksi, selkeänä tai pakattuna, tallenna se kansioon tai tuo se juuri käynnistettyyn tietokantaan yhdellä napsautuksella **Varmuuskopiot**-välilehdellä. Vedoksen saaminen takaisin ilman BombVaultia: katso [Tietokantavedoksen palauttaminen](offsite-recovery.md#database-dumps). Kun yksi epäonnistuu, katso [Tietokantavedos epäonnistui](troubleshooting.md#a-database-dump-failed).

## Komennot ja muut kontit {#commands}

- **Komennot ennen ja jälkeen** ajetaan kontin sisällä komennolla `sh -c`, esimerkiksi välimuistin tyhjentämiseksi levylle. Epäonnistuva komento ennen varmuuskopiota keskeyttää sen. Komento ennen varmuuskopiota, joka päättyy koodiin 75, lykkää ajastetun varmuuskopion, katso alla.
- **Pysäytä muut säiliöt** nimeää kontit, jotka pysäytetään tämän varmuuskopioinnin ajaksi, kuten sen tietokannan. Jälkeenpäin ne käynnistyvät uudelleen Composen `depends_on`-järjestyksessä, ja oletuksena jokaisen on raportoitava olevansa kunnossa, ennen kuin seuraava käynnistyy. Odotus ja sen aikakatkaisu ovat kortissa **Järjestys** kohdassa **Uudelleenkäynnistys varmuuskopioinnin jälkeen**, joka on kohdassa **Asetukset, Aikataulut**.
- Kontti, joka jakaa pysäytettävän kontin verkko-, PID- tai IPC-nimiavaruuden `--net=container:`- tai Composen `network_mode: service:` -asetuksella, pysähtyy ja käynnistyy sen mukana, vaikka se ei olisi listalla, ja se käynnistyy sen kontin jälkeen, johon se liittyy. Uudelleen käynnistetty kontti saa uuden verkon, ja vanhaan verkkoon käyntiin jätetyllä kontilla olisi vain loopback. Itse lista pysyy sellaisena kuin sen asetit. Jos tällaista konttia ei voitu pysäyttää, ajo kertoo, ettei sillä ole verkkoa ennen kuin se käynnistetään uudelleen. Sama koskee BombVaultin omaa konttia, joka ei koskaan pysäytä itseään eikä sitä voi lisätä listalle.
- Kontti, joka ei käynnisty uudelleen jälkeenpäin, nimetään ajossa ja ilmoituksessa, myös silloin, kun itse varmuuskopiointi epäonnistui.

## Kun sovellus on kiireinen {#app-busy}

Varmuuskopio pysäyttää konttinsa, joten ajastettu varmuuskopio voi odottaa, kun joku käyttää sovellusta, ja yrittää itsestään uudelleen. Kontin Asetukset-välilehden **Kun sovellus on kiireinen** -kortti asettaa sen:

- **Aktiivisuustarkistus** kysyy Plexiltä, Jellyfiniltä tai Embyltä ennen ajastettua varmuuskopiota, katsooko joku. Plexiltä kysytään sen oman `Preferences.xml`-tiedoston tunnisteella, joten mitään ei tarvitse syöttää. Jellyfin ja Emby tarvitsevat **API-avaimen**, joka tehdään palvelimen kojelaudan kohdassa API keys.
- **Ohita tauotetut istunnot (min) jälkeen** antaa yöksi tauolle jätetyn elokuvan lakata vaikuttamasta.
- **Varmuuskopioi, kun palvelin ei vastaa** päättää, mitä tapahtuu, kun palvelimelta ei voi kysyä: päällä varmuuskopio ajetaan ja loki kertoo sen. Pois päältä se odottaa ikään kuin joku katsoisi.
- **Odota, kun kontti lähettää dataa** on sovelluksille, joilla ei ole tällaista tarkistusta, kuten pelipalvelin tai latausohjelma. Varmuuskopio odottaa, kun kontti lähetti enemmän kuin **Kiireinen alkaen (KiB/s)** missä tahansa lukemassa viimeisten **Hiljaisena (min)** minuutin aikana.
- **Yritä uudelleen (min) välein**, **Yritä vain aikaikkunan sisällä** kohdalla **Yritä välillä** ja **Odota enintään (tuntia)** asettavat yritysten kulun.
- **Odotuksen päättyessä** joko luovuttaa, jolloin varmuuskopio näkyy kohdassa **Vaatii huomiota** painikkeella **Yritä uudelleen**, tai **Varmuuskopioi silti**.

Odottava varmuuskopio kirjataan tilaan **Lykätty**, meripihkanvärisenä, ja ajon loppuosa jatkuu. Yleiskatsaus, lista ja merkintä näyttävät sen seuraavan yrityksen kanssa, ja myöhässä-hälytys ja suojauksen tila odottavat sitä määräaikaan asti. Saat yhden ilmoituksen, kun se lykätään, ja yhden, kun se päättyy, et yhtä yritystä kohti. Yritys odottaa, kun toinen varmuuskopio on käynnissä, ja odotus jatkuu uudelleenkäynnistyksen jälkeen. Itse käynnistämäsi varmuuskopio ajetaan aina heti, olipa se **Varmuuskopioi nyt**, **Yritä uudelleen**, valinta, API tai MCP, ja onnistunut varmuuskopio päättää odotuksen, joka oli kesken.

Kontin Asetukset-välilehdellä on myös vanhempi, yksinkertaisempi tapa: **Odota, kunnes sovellus on joutilas** kohdalla **Odota enintään (tuntia)**. Varmuuskopio alkaa heti, kun sovellus on joutilas, minkä määrittelee **Joutilaisuus ennen varmuuskopiota** -kortti kohdassa **Asetukset, Aikataulut**. Compose-stackin jäsenet, jotka ovat samassa ajossa vuorossa, odottavat yhdessä, ja odottava varmuuskopio ei pidä lukkoa, joten muut kontit etenevät.

![Plex-kontin Kun sovellus on kiireinen -kortti aktiivisuustarkistus asetettuna.](assets/screenshots/backups/app-busy-dark.png#only-dark)
![Plex-kontin Kun sovellus on kiireinen -kortti aktiivisuustarkistus asetettuna.](assets/screenshots/backups/app-busy-light.png#only-light)

### Esimerkki: Plex odottaa, kun joku katsoo {#example-plex}

Täysvarmuuskopio ajetaan joka yö kello 03.00. Kenenkään ei pitäisi nähdä elokuvan pysähtyvän, koska Plex meni alas varmuuskopioinnin vuoksi, mutta varmuuskopion pitäisi silti tapahtua ennen aamua.

1. Avaa **Varmuuskopiot**, sitten `plex`-merkintä ja sen **Asetukset**-välilehti.
2. Aseta kohdassa **Kun sovellus on kiireinen** **Aktiivisuustarkistus** Plexiksi. Mitään ei tarvitse kirjoittaa.
3. Aseta **Ohita tauotetut istunnot (min) jälkeen** arvoon `30`, jotta keskiyöllä tauolle jätetty elokuva lakkaa vaikuttamasta puoli yhdeltä.
4. Jätä **Varmuuskopioi, kun palvelin ei vastaa** päälle.
5. Aseta **Yritä uudelleen (min) välein** arvoon `15`.
6. Kytke päälle **Yritä vain aikaikkunan sisällä** ja aseta **Yritä välillä** arvoon `22:00` ja `06:00`.
7. Aseta **Odota enintään (tuntia)** arvoon `3` ja **Odotuksen päättyessä** arvoon **Varmuuskopioi silti**.

Mitä tapahtuu yönä, jona joku katsoo vielä kello 03.00: BombVault kysyy Plexiltä, löytää istunnon, joka toistaa, ja kirjaa varmuuskopion tilaan **Lykätty** seuraavalla yrityksellä kello 03.15. Ajon muut kontit varmuuskopioidaan tavalliseen tapaan. Kun elokuva päättyy kello 04.10, yritys kello 04.15 löytää Plexin joutilaana ja varmuuskopioi sen, ja odotus päättyy yhteen ilmoitukseen. Jos elokuva olisi vielä käynnissä kello 06.00, odotus saavuttaisi päänsä ja Plex varmuuskopioitaisiin silti, joten sen varmuuskopiot eivät koskaan jää yli kolmea tuntia jälkeen.

Omaa tarkistustasi varten tee **Varmuuskopiota edeltävästä komennosta** sellainen, että se päättyy koodiin 75, kun sovellus on kiireinen, esimerkiksi `[ "$(cat /tmp/players)" = 0 ] || exit 75`. Lisää oireita ja korjauksia on kohdassa [Vianmääritys](troubleshooting.md#deferred-backups).

## Selkeät viennit {#exports}

restic pysyy moottorina, mutta vienti antaa kopion, joka ei tarvitse BombVaultia eikä resticiä.

- **Vie kontti** ja **Vie VM** kohdassa **Lisää** merkinnän sivulla kirjoittavat kopion sellaisena kuin se on juuri nyt, repon viereen: kontin varmuuskopioitavat kansiot ja sen Unraid-malli tai virtuaalikoneen levykuvat ja sen määritys, palautettavissa komennolla `virsh define`.
- **Vie** tilannevedoksella kirjoittaa tallennetun varmuuskopion palvelimen vientikansioon tar-, zip- tai oman restic-repon muodossa, ja voi ottaa yhden tilannevedoksen tai kaikki. Flash-varmuuskopio ja itsevarmuuskopio sisältävät avaimia, joten niiden vienti vaatii kirjautumissalasanan ja kirjautumisen instanssissa.

Viennit ovat oletuksena selkeitä tiedostoja. **Salaa viennit age:lla** kohdassa **Asetukset, Järjestelmä** sinetöi ne, myös flash-ZIPin, listaamillesi vastaanottajille, joista jokainen on age-julkinen avain tai SSH-julkinen avain. Puret ne myöhemmin laatikon ulkopuolella vastaavalla yksityisellä avaimella. Salauksen ollessa päällä ja ilman kelvollista vastaanottajaa vienti epäonnistuu selvään virheeseen sen sijaan, että kirjoittaisi koskaan selkeitä tiedostoja.

## Mitä ajo kertoo sinulle {#run-notes}

- **Ajoitus.** Jokainen ajo kertoo alun, lopun ja keston, ja jokaisella merkinnällä on omat **Viimeisimmät ajot**.
- **Miksi varmuuskopio oli hidas.** Varmuuskopioinnin aikana BombVault seuraa, kuinka kiireinen suoritin, levyt ja verkko ovat. Kun varmuuskopio kestää paljon tavallista kauemmin ja yksi asia oli selvästi rajallaan, ajo kertoo sen, esimerkiksi "Kohdelevy disk1 oli 98 % kiireinen". Muuten se ei sano mitään.
- **Tarkistusten ja karsintojen eteneminen.** Toimintaloki ja **Tarkista varmuuskopiot** -kortti näyttävät, kuinka pitkälle restic on laskenut, esimerkiksi *12 / 47 pakettia*, ja siihen vaiheeseen jäljellä olevan ajan, kun arvioon on riittävästi tietoa.
- **Tiedostot, joita ei voitu lukea.** Varmuuskopio, joka onnistui mutta jätti jotakin pois, muuttaa rivinsä oranssiksi ja nimeää sen, mitä jäi pois.
