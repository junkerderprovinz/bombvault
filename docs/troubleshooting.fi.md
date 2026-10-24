# Vianmääritys

Lyhyt UKK. Täydellistä VM-over-SSH-isäntäpuolen vianmääritystaulukkoa varten (permission-denied, isäntäavaimen todennus, puuttuvat mallimuuttujat ja muuta) katso [VM-varmuuskopiointi SSH:n yli -opas](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) GitHubissa.

## Jokin ei ole kytketty oikein

Lisää **Isäntäintegraatio**-kortti yleiskatsaukseen kohdassa **Mukauta**. Isäntäintegraation tarkistus koettaa jokaista liitosta ja komentorivityökalua (Docker-soketti, libvirt, restic, qemu-img, rclone) ja raportoi puuttuvat palaset, ja **Tarkista nyt** ajaa sen uudelleen. Aloita täältä ennen kuin oletat bugia: puuttuva liitos tai tavoittamaton isäntä näkyy heti.

## En saa yhteyttä verkkokäyttöliittymään

BombVault tarjoaa HTTPS:ää valmiiksi portissa `3443` (itse allekirjoitettu varmenne), joten avaa `https://<your-unraid-ip>:3443`. Hyväksy itse allekirjoitetun varmenteen varoitus tai sijoita BombVault käänteisen välityspalvelimen taakse omalla varmenteellasi. Jos ajat asetuksella `HTTP_ONLY=true`, se tarjoaa sen sijaan selkeää HTTP:tä portissa `3000` (tarkoitettu käytettäväksi TLS:n päättävän välityspalvelimen takana).

## Menetin APP_KEY:ni

`APP_KEY` johtaa restic-arkiston salasanan. Ilman sitä, ilman palautuspakettia ja ilman avaintalletusta ryhmässäsi salattuja varmuuskopioita ei voi palauttaa. Tämän vuoksi yleiskatsaus näyttää **Tallenna palautuspakettisi**, kunnes olet ladannut sen. Katso [Etäsijainti ja palautus](offsite-recovery.md#recovery-kit). Luo avain komennolla `openssl rand -hex 32` ja säilytä se palvelimen ulkopuolella ennen kuin luotat mihinkään varmuuskopioon.

## Unohdin kirjautumissalasanan

Valitse kirjautumissivulla **Unohtuiko salasana?** BombVault kirjoittaa 12 merkin koodin konttinsa lokiin (Unraidissa: Docker-välilehti, BombVault-kuvake, **Logs**). Kirjoita se uuden salasanan kanssa 10 minuutin kuluessa. Nollaus myös kytkee kaksivaiheisen tunnistautumisen pois, poistaa kaikki pääsyavaimet, mitätöi kaikki API-tokenit, MCP-avaimet ja OAuth-kirjautumiset, päättää kaikki muut istunnot ja kytkee puhelinten pariliitoksen pois. Lisää pääsyavaimesi uudelleen kohdassa **Asetukset, Turvallisuus**, kytke puhelinten pariliitos takaisin päälle siellä ja luo uudet tokenit ja avaimet sinne, missä tarvitset niitä yhä. Katso [Turvallisuusmalli](configuration.md#login).

## Varmuuskopiointi pysähtyi, ja avainta ei voi enää avata

Tämä palvelin otti Hätäohjatussa toiminnossa haltuun toisen palvelimen varmuuskopioavaimen, ja `APP_KEY`:tä muutettiin sen jälkeen. Haltuun otettu avain on sinetöity vanhalla APP_KEY:llä, joten BombVault ei voi avata sitä eikä varmuuskopioi mitään sen sijaan, että kirjoittaisi väärällä avaimella. Yleiskatsauksen **Vaatii huomiota** nimeää sen. Aseta APP_KEY takaisin entiselleen tai lataa palautuspaketti uudelleen Palautus-sivulla kohdassa **Hätätilanne**.

## Puhelin päätyy kirjautumissivulle

Pariliitetty puhelin saa istuntonsa ryhmän kautta vain niin kauan kuin **Puhelimet voivat pariutua ryhmän kahdentoista sanan avulla** on päällä kohdassa **Asetukset, Turvallisuus**. Salasanan nollaus kytkee sen pois. Palvelinta vanhempi sovellus ei myöskään saa istuntoa, ennen kuin se päivitetään. Osoitteella lisätty palvelin kysyy aina salasanaa. Ilmentymät-sivulta poistettu puhelin palaa pois kytkettynä, kun sen sovellus avataan uudelleen; päästä se sisään kytkemällä sen tiedoissa kohdassa **Valvonta** päälle **Kysele tätä instanssia**.

## VM-varmuuskopiointi ei muodosta yhteyttä

VM-varmuuskopiointi keskustelee libvirtin kanssa SSH:n yli, ei koskaan liitoksen kautta.

- Vahvista, että SSH on käytössä isännällä ja BombVaultin julkinen avain on valtuutettu tiedostossa `/root/.ssh/authorized_keys` (Asetukset, Integraatiot, Palvelimen SSH näyttää avaimen ja **Testaa yhteys** -painikkeen).
- Mukautetussa `br0.x`-verkossa aseta `LIBVIRT_HOST` Unraidin LAN-IP-osoitteeseesi (kontti ei voi tavoittaa isäntää `host.docker.internal`-nimellä siellä). Ota käyttöön **Settings, Docker, Host access to custom networks**.
- Jos vaihdoit Unraidin SSH-porttia, aseta `LIBVIRT_SSH_PORT` vastaamaan.
- TrueNASissa aseta `LIBVIRT_URI` TrueNASin omalla libvirt-soketilla, anna sovellukselle `SETPCAP`-capability ja lisää avain root-käyttäjälle kohdassa **Credentials, Users** (katso [TrueNAS SCALE](getting-started.md#truenas)).
- Täydellinen vaihe vaiheelta -diagnoosi (tavoitettavuustesti, VLAN-reititys, `Permission denied (publickey)`, `Host key verification failed`) on [VM-varmuuskopiointi SSH:n yli -oppaassa](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md).

## Live-VM-tilannevedos ei ajautunut

Live-tilannevedokset tarvitsevat qemu guest agentin asennettuna VM:ään ja levyn sijainniksi `/mnt/cache` (tai `/mnt/diskX`), ei `/mnt/user`. Sammutetussa VM:ssä live palautuu automaattisesti hallittuun. Hallittu varmuuskopio sammuttaa VM:n, varmuuskopioi levyt ja käynnistää sen sitten uudelleen, joten se on aina eheä. Se odottaa enintään kymmenen minuuttia, että vieras sammuu, eikä koskaan pakota sitä pois päältä, joten päivityksiä asentava Windows-vieras ei katkea kesken. Vieras, joka on sen jälkeen yhä käynnissä, jätetään rauhaan, ja varmuuskopio epäonnistuu syyn kertoen.

## Varmuuskopio epäonnistui virheeseen "repository is already locked"

Tämä on yleensä orpo restic-lukko, joka jäi jälkeen kun kontti päivitettiin tai käynnistettiin uudelleen kesken toiminnan. BombVault havaitsee todistettavasti orvon lukon, pakottaa sen auki ja yrittää uudelleen kerran, automaattisesti. Jos se jää pysyväksi, avaa kortti **Tarkista varmuuskopiot** kohdassa **Asetukset, Tarkistus** ja valitse **Avaa lukitus** kohdasta **Lisää** kyseisen osion rivillä poistaaksesi jumittuneen lukon käsin. Todellinen ongelma nousee silti pintaan sen sijaan että piiloutuisi. Uudelleenkäynnistyksen jälkeen BombVault odottaa, kunnes tällaista lukkoa ei ole uusittu kymmeneen minuuttiin. Yhä käynnissä oleva restic, esimerkiksi toisessa samaa tietovarastoa käyttävässä BombVaultissa, uusii lukkonsa viiden minuutin välein.

## Etäkopiotani ei tapahtunut varmuuskopion jälkeen

Etäreplikointi on suunnitellusti parhaan yrityksen mukaista, joten etäsijainnin nikottelu ei koskaan kaada paikallista varmuuskopiota. Tarkista, milloin kohde kopioi: sen **Kopioi**-kortti sanoo **Jokaisen varmuuskopion jälkeen** tai nimeää oman aikataulun, ja **Asetukset, Aikataulu, Etäkopiot osioittain** voi antaa osiolle oman aikataulun. Käytä **Replikoi nyt** osion **Etäkopio**-kortissa kohdassa **Asetukset, Kohteet** pyydettäessä tehtävään ajoon, ja tarkkaile yleiskatsauksen tilariviä, joka kertoo **Etäkopiointi käynnissä**, kun kopiointi on käynnissä. Epäonnistunut kopio näkyy kohdassa **Vaatii huomiota**.

## rest-serverin varmenteeseen ei luoteta

restic hylkää varmenteen, jota se ei tunne, ja virheilmoitus kertoo, minne varmenne kuuluu. Kopioi rest-serverin itse allekirjoitettu varmenne, tai sen allekirjoittaneen varmentajan varmenne, PEM-tiedostona BombVaultin datakansion kansioon `certs` (`/config/certs` kontissa, `appdata/bombvault/certs` Unraidissa) ja testaa uudelleen. Uudelleenkäynnistystä ei tarvita.

## Varmuuskopio kertoo, ettei Host Datan ulkopuolisia tietoja varmuuskopioitu {#data-outside-host-data}

Kontti pitää tietoja kansiossa, jota BombVault ei voi lukea Host Datan kautta. Varmuuskopio tallentaa määrittelyn ja jokaisen kansion, johon se ylettyy, värittää rivinsä tapahtumalokissa oranssiksi ja nimeää kansiot, jotka se jätti pois. Kaksi tapausta on yleisiä: ixVolume TrueNASissa polussa `/mnt/.ix-apps` ja nimetty Docker-taltio polussa `/var/lib/docker/volumes`, kuten compose-pinon tietokantataltio. Kontin **Asetukset**-välilehden kortti **Varmuuskopioitavat kansiot** näyttää jokaisen tällaisen kansion polun kontin sisällä ja isännällä punaisella merkittynä.

Nimettömästä Docker-taltiosta ei varoiteta. Docker luo sen itse kansiolle, jonka image määrittelee eikä kukaan ole liittänyt, nimeää sen 64 heksadesimaalinumerolla ja säilyttää sitä polussa `/var/lib/docker/volumes`. Siinä on yleensä välimuisti, joten varmuuskopio pysyy vihreänä ja kortti vain näyttää sen ja mainitsee, että se on nimetön.

- **Haluat varmuuskopioida nuo tiedot.** Liitä kansio isännän polkuun Host Datan alla, esimerkiksi appdata-jaon kansioon, niin seuraava varmuuskopio ottaa sen mukaan.
- **Et tarvitse niitä.** Lisää jompikumpi sen kahdesta polusta riviksi kortille **Poissulkumallit** samalla välilehdellä: polku kontin sisällä, kuten `/cache`, tai isännän polku, jonka varoitus mainitsee, kuten `/var/lib/docker/volumes/jellyfin_cache/_data`. Seuraavasta varmuuskopiosta alkaen varoitus jättää kansion pois, ja kentän alla oleva esikatselu kertoo miksi. Myös rivi yläpuoliselle kansiolle toimii, pelkkä nimi tai `/` ei.

## Suuri varmuuskopio lopetettiin muistin loppumisen vuoksi

Aseta **Muistiraja varmuuskopiota kohden (MiB, 0 = ei rajaa)** kortissa **Suorituskyky** kohdassa **Asetukset, Järjestelmä** selvästi pienemmäksi kuin muisti, jota kontti saa käyttää. Rajan lähellä restic vapauttaa muistia useammin, mikä maksaa hieman nopeutta, joten varmuuskopio hidastuu sen sijaan, että se lopetettaisiin.

## Palautus keskeytyi ennen kuin se alkoi

Ennen kuin mitään pysäytetään tai poistetaan, palautus ajaa esitarkastuksen ristiriidoista: se varmistaa, että kontin kiinteä IP ja julkaistut isäntäportit ovat vapaana. Jos toinen kontti jo pitää yhtä hallussaan, se keskeyttää selkeällä, toimintakelpoisella viestillä sen sijaan että jättäisi puolivalmiin palautuksen. Vapauta ristiriitainen portti tai IP ja yritä sitten uudelleen.

Jos kontin verkkoa ei ole olemassa, esimerkiksi koska sen TrueNAS-sovellus poistettiin, tarkistus ennen palautusta nimeää verkon, ja palautus kieltäytyy ennen kuin muuttaa mitään. Asenna sovellus TrueNASiin uudelleen samalla nimellä, jolloin verkko luodaan uudelleen, ja palauta sitten sen data.

## Selkokielinen vienti epäonnistui tiedoston kirjoittamisen sijaan

Jos **Salaa viennit age:lla** on päällä kohdassa **Asetukset, Järjestelmä** mutta kelvollista vastaanottajaa ei ole asetettu, vienti epäonnistuu selkeällä virheellä selkotekstin kirjoittamisen sijaan. Lisää kelvollinen vastaanottaja (age-julkinen avain tai SSH-julkinen avain), tai kytke salaus pois päältä, jos tarkoitat viennin olevan selkotekstiä. Katso [Ominaisuudet](features.md).

## Tietokantavedos epäonnistui

Epäonnistunut vedos ei koskaan kaada ympärillään olevaa varmuuskopiota; se kirjataan omaksi epäonnistuneeksi ajokseen, ja syy kertoo, mitä korjata.

- **Kirjautuminen hylättiin.** Vedos kirjautuu kontin omilla salasanamuuttujilla (`POSTGRES_PASSWORD`, `MARIADB_ROOT_PASSWORD`, `MYSQL_ROOT_PASSWORD` tai niiden `_FILE`-versioilla). Tarkista ne tietokantakontista. `_FILE`-muuttuja, joka osoittaa salaisuuteen, jota kontin oma käyttäjä ei saa lukea, epäonnistuu samalla tavalla.
- **Puuttuvat oikeudet.** Satunnaisella root-salasanalla vedos pääsee kirjautumaan vain sovelluskäyttäjänä, jolloin se sisältää vain sen yhden tietokannan, ja MySQL 8.4 ja uudemmat voivat kieltäytyä kokonaan. Anna kontille oikea root-salasana tai kytke sen vedos pois.
- **Järjestelmätaulut vaativat päivityksen.** MariaDB kieltäytyy vedostamasta, kun sen järjestelmätaulut ovat vanhemmasta versiosta (virhe 1558). Lisää muuttuja `MARIADB_AUTO_UPGRADE=1` ja käynnistä kontti uudelleen, tai aja `mariadb-upgrade` sen sisällä kerran.
- **Ei vedostustyökalua.** Riisuttua tai itse rakennettua levykuvaa ilman `pg_dump`-, `mysqldump`- tai `mariadb-dump`-työkalua ei voi vedostaa. Käytä virallista levykuvaa tai kytke vedos pois.
- **Aikaraja.** Vedos saa `DB_DUMP_MAX_HOURS` (oletuksena 6), sen ympärillä oleva varmuuskopio saa `BACKUP_MAX_HOURS`, ja vedos, joka lakkaa edistymästä, katkaistaan `BACKUP_STALL_HOURS`-ajan jälkeen. Viimeisen takana on yleensä sovelluksen pitämä lukko. Nosta sitä rajaa, joka laukesi, tai vedosta silloin kun sovellus on rauhallinen.
- **Kontti on pysäytetty tauolle tai käynnistyy uudelleen.** Vedos puhuu käynnissä olevan palvelimen kanssa. Jos kontti käynnistyy yhä uudelleen, sen oma loki kertoo miksi.
- **Vaurioitunutta vedosta ei saatu poistettua.** Vedos, jota BombVault ei saanut valmiiksi, poistetaan. Kun poisto epäonnistuu, vedos jää listalle merkittynä vaurioituneeksi ja voit poistaa sen sieltä.

## Tuonti epäonnistui

Tuonti pysäyttää kontin, siirtää sen datakansion sivuun ja antaa levykuvan luoda tilalle tyhjän. Jos jokin vaihe ennen varsinaista tuontia epäonnistuu, vanha kansio palautetaan itsestään. Jos tuonti epäonnistuu, kontille jää tuore kansio ja vanha jää sen viereen nimellä `<datakansio>.bombvault-before-import-<aikaleima>`; ajon virheilmoitus kertoo tarkan polun.

Datakansiota, joka on oma tietojoukkonsa, kuten TrueNASissa, ei voi nimetä uudelleen. Tuonti jättää silloin kansion paikalleen ja siirtää sen sisällön viereiseen säilytettyyn kansioon, joten yläpuolisessa tietojoukossa on oltava sille tilaa. Tuonti tarkistaa tämän ennen kuin pysäyttää kontin.

Käsin palautus: pysäytä kontti, nimeä nykyinen datakansio pois tieltä, nimeä säilytetty kansio takaisin alkuperäiselle nimelleen ja käynnistä kontti. Tietojoukon kohdalla siirrä sisältö kansioiden uudelleennimeämisen sijaan. Unraidissa tämän hoitaa Shares-välilehden tiedostonhallinta.

## ZFS-tietojoukon varmuuskopio epäonnistui tai ohitti tietojoukon {#zfs-datasets}

Jokaisella ongelmalla on syykoodi hakasulkeissa, ja sivu [ZFS-tietojoukot](zfs-datasets.md#reason-codes) luettelee ne kaikki korjauksineen. Kolme yleisintä:

- **`snapshot-loop`**: tilannevedos ei päässyt BombVaultiin, koska Host Data ei välitä uusia liitoksia. Muokkaa konttia, aseta Host Datan Access Mode arvoon Read/Write - Slave ja käynnistä BombVault uudelleen.
- **`key-not-loaded`**: salattu tietojoukko, jonka avainta ei ole ladattu, ohitetaan. Lataa avain komennolla `zfs load-key` ja liitä tietojoukko; seuraava varmuuskopio ottaa sen mukaan.
- **`ssh-auth`**: palvelin hylkäsi BombVaultin avaimen. **ZFS-yhteys tähän palvelimeen**, joka avautuu Varmuuskopiot-sivun kohdasta **Näytä yhteys**, näyttää komennon, joka valtuuttaa sen; aja se kerran palvelimella.

## Merkintä jää tilaan "Oppii N/10"

Useimmat poikkeamatarkistukset alkavat merkinnän 10 onnistuneen varmuuskopion jälkeen, ja laskenta alkaa alusta toiminnon **Merkitse odotetuksi** jälkeen ja kun merkinnän valinta on muuttunut. Merkintä, jolla ei ole ajastusta, ei opi, eikä kontilla ilman appdataa ole mistä oppia, minkä sen merkki myös kertoo. **Oppimisen edistyminen** Poikkeamat-sivulla näyttää kymmenosaisen renkaan merkintää kohti.

## Säilytys ei enää poista yhden merkinnän vanhoja varmuuskopioita

Avoin kriittinen poikkeama pitää ne: merkinnän lähde on lähes tyhjä, on kutistunut voimakkaasti tai varmuuskopio on tallentanut suurimman osan datasta uudelleen. Avaa poikkeama merkinnän merkistä. Jos dataa puuttuu tai se on salattu, palauta ensin linkitetystä viimeisestä hyvästä varmuuskopiosta. Kuittaa sitten poikkeama tai merkitse se odotetuksi, jos muutos oli sinun, niin seuraava ajo siivoaa tavalliseen tapaan. Säilytyksen esikatselu merkitsee tällaisen merkinnän säilytetyksi. ZFS-merkinnässä vain poikkeaman nimeämä tietojoukko säilyttää vanhat varmuuskopionsa; puun muut tietojoukot siivotaan tavalliseen tapaan.

## Manuaalinen siivous kertoo, että joitakin merkintöjä säilytettiin

Sama syy: siivous jättää tällaisen poikkeaman merkinnän vanhat varmuuskopiot rauhaan ja nimeää merkinnän viestissään. Kaikki muu siivotaan tavalliseen tapaan.

## Historian tuonti kertoo, ettei tietovarastoa voitu lukea

Päivityksen jälkeen BombVault lukee kerran aiempien varmuuskopioiden koot jokaisesta tietovarastosta. Tietovarasto, jota ei silloin tavoitettu, esimerkiksi alhaalla ollut ulkoinen kohde tai liittämätön jako, näkyy kortissa **Poikkeamat** kohdassa **Asetukset, Tarkistus**, ja sitä yritetään uudelleen kerran päivässä. Sillä välin sen merkinnät oppivat uusista varmuuskopioista.

## Levytilavaroitus ei vastaa Unraidin kojelautaa

Unraidin käyttäjäjaolla (`/mnt/user`) vapaa tila on koko arrayn, ei yksittäisen levyn. Etätietovarastot mitataan vain rclone-etäkohteiden kautta, jotka ilmoittavat vapaan tilansa; S3-, B2-, REST- ja SFTP-tietovarastoilla ei ole lukua, ja ne näkyvät kortissa **Poikkeamat** mittaamattomina.

## Tekoälyavustaja ei saa yhteyttä

Sivu [MCP-palvelin](mcp.md#troubleshooting) kertoo, mitä kukin tilakoodi ja kukin MCP-päätepisteen hylkäys tarkoittaa ja mitä asialle voi tehdä.

## Kontti käynnistyy jatkuvasti uudelleen tai näyttää epäterveeltä

BombVault raportoi terve/epäterve omasta `/api/health`-päätepisteestään. Automaattinen korjaustyökalu (kuten Autoheal) voi käynnistää sen uudelleen automaattisesti, jos moottori koskaan jumiutuu. Tarkista kontin loki ja **Isäntäintegraatio**-kortti taustalla olevan syyn selvittämiseksi.

## Yhä jumissa?

- Lue täydelliset [Asetukset](configuration.md)- ja [Etäsijainti ja palautus](offsite-recovery.md) -sivut.
- Kysy [Unraid-tukiketjussa](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/).
- Avaa [GitHub-ongelma](https://github.com/junkerderprovinz/bombvault/issues).
