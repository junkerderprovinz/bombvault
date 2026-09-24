# Vianmääritys

Lyhyt UKK. Täydellistä VM-over-SSH-isäntäpuolen vianmääritystaulukkoa varten (permission-denied, isäntäavaimen todennus, puuttuvat mallimuuttujat ja muuta) katso [VM-varmuuskopiointi SSH:n yli -opas](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) GitHubissa.

## Jokin ei ole kytketty oikein

Avaa `/spike` verkkokäyttöliittymässä. Isäntäintegraation tarkistus koettaa jokaista liitosta ja komentorivityökalua (Docker-soketti, libvirt, restic, qemu-img, rclone) ja raportoi puuttuvat palaset. Aloita täältä ennen kuin oletat bugia: puuttuva liitos tai tavoittamaton isäntä näkyy heti.

## En saa yhteyttä verkkokäyttöliittymään

BombVault tarjoaa HTTPS:ää valmiiksi portissa `3443` (itse allekirjoitettu varmenne), joten avaa `https://<your-unraid-ip>:3443`. Hyväksy itse allekirjoitetun varmenteen varoitus tai sijoita BombVault käänteisen välityspalvelimen taakse omalla varmenteellasi. Jos ajat asetuksella `HTTP_ONLY=true`, se tarjoaa sen sijaan selkeää HTTP:tä portissa `3000` (tarkoitettu käytettäväksi TLS:n päättävän välityspalvelimen takana).

## Menetin APP_KEY:ni

`APP_KEY` johtaa restic-arkiston salasanan. Ilman sitä (ja ilman salausavaimen palautuspakettia) salattuja varmuuskopioita ei voi palauttaa. Tämän vuoksi Kojelauta nalkuttaa sinua lataamaan palautuspaketin. Katso [Etäsijainti ja palautus](offsite-recovery.md). Luo avain komennolla `openssl rand -hex 32` ja säilytä se palvelimen ulkopuolella ennen kuin luotat mihinkään varmuuskopioon.

## VM-varmuuskopiointi ei muodosta yhteyttä

VM-varmuuskopiointi keskustelee libvirtin kanssa SSH:n yli, ei koskaan liitoksen kautta.

- Vahvista, että SSH on käytössä isännällä ja BombVaultin julkinen avain on valtuutettu tiedostossa `/root/.ssh/authorized_keys` (Asetukset, Järjestelmä, VM Backup over SSH näyttää avaimen ja **Test connection** -painikkeen).
- Mukautetussa `br0.x`-verkossa aseta `LIBVIRT_HOST` Unraidin LAN-IP-osoitteeseesi (kontti ei voi tavoittaa isäntää `host.docker.internal`-nimellä siellä). Ota käyttöön **Settings, Docker, Host access to custom networks**.
- Jos vaihdoit Unraidin SSH-porttia, aseta `LIBVIRT_SSH_PORT` vastaamaan.
- Täydellinen vaihe vaiheelta -diagnoosi (tavoitettavuustesti, VLAN-reititys, `Permission denied (publickey)`, `Host key verification failed`) on [VM-varmuuskopiointi SSH:n yli -oppaassa](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md).

## Live-VM-tilannevedos ei ajautunut

Live-tilannevedokset tarvitsevat qemu guest agentin asennettuna VM:ään ja levyn sijainniksi `/mnt/cache` (tai `/mnt/diskX`), ei `/mnt/user`. Sammutetussa VM:ssä live palautuu automaattisesti hallittuun. Hallittu varmuuskopio sammuttaa VM:n, varmuuskopioi levyt ja käynnistää sen sitten uudelleen, joten se on aina eheä.

## Varmuuskopio epäonnistui virheeseen "repository is already locked"

Tämä on yleensä orpo restic-lukko, joka jäi jälkeen kun kontti päivitettiin tai käynnistettiin uudelleen kesken toiminnan. BombVault havaitsee todistettavasti orvon lukon, pakottaa sen auki ja yrittää uudelleen kerran, automaattisesti. Jos se jää pysyväksi, käytä **Asetukset, Eheys ja ylläpito, Avaa lukitus** kyseiselle toimialueelle poistaaksesi jumittuneen lukon käsin. Todellinen ongelma nousee silti pintaan sen sijaan että piiloutuisi.

## Etäkopiotani ei tapahtunut varmuuskopion jälkeen

Etäreplikointi on suunnitellusti parhaan yrityksen mukaista, joten etäsijainnin nikottelu ei koskaan kaada paikallista varmuuskopiota. Tarkista kyseisen toimialueen etäaikataulu (Asetukset, Aikataulut): tyhjä aikataulu replikoi jokaisen paikallisen varmuuskopion jälkeen, kun taas tahti lähettää harvemmin. Käytä **Replikoi nyt** Etä-välilehdellä pyydettäessä tehtävään ajoon, ja tarkkaile replikointiosoitinta Kojelaudalla.

## Palautus keskeytyi ennen kuin se alkoi

Ennen kuin mitään pysäytetään tai poistetaan, palautus ajaa esitarkastuksen ristiriidoista: se varmistaa, että kontin kiinteä IP ja julkaistut isäntäportit ovat vapaana. Jos toinen kontti jo pitää yhtä hallussaan, se keskeyttää selkeällä, toimintakelpoisella viestillä sen sijaan että jättäisi puolivalmiin palautuksen. Vapauta ristiriitainen portti tai IP ja yritä sitten uudelleen.

## Selkokielinen vienti epäonnistui tiedoston kirjoittamisen sijaan

Jos age-salaus on päällä (Asetukset) mutta kelvollista vastaanottajaa ei ole asetettu, vienti epäonnistuu selkeällä virheellä selkotekstin kirjoittamisen sijaan. Lisää kelvollinen vastaanottaja (age-julkinen avain tai SSH-julkinen avain), tai kytke salaus pois päältä, jos tarkoitat viennin olevan selkotekstiä. Katso [Ominaisuudet](features.md).

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

Käsin palautus: pysäytä kontti, nimeä nykyinen datakansio pois tieltä, nimeä säilytetty kansio takaisin alkuperäiselle nimelleen ja käynnistä kontti. Unraidissa tämän hoitaa Shares-välilehden tiedostonhallinta.

## ZFS-tietojoukon varmuuskopio epäonnistui tai ohitti tietojoukon {#zfs-datasets}

Jokaisella ongelmalla on syykoodi hakasulkeissa, ja sivu [ZFS-tietojoukot](zfs-datasets.md#reason-codes) luettelee ne kaikki korjauksineen. Kolme yleisintä:

- **`snapshot-loop`**: tilannevedos ei päässyt BombVaultiin, koska Host Data ei välitä uusia liitoksia. Muokkaa konttia, aseta Host Datan Access Mode arvoon Read/Write - Slave ja käynnistä BombVault uudelleen.
- **`key-not-loaded`**: salattu tietojoukko, jonka avainta ei ole ladattu, ohitetaan. Lataa avain komennolla `zfs load-key` ja liitä tietojoukko; seuraava varmuuskopio ottaa sen mukaan.
- **`ssh-auth`**: palvelin hylkäsi BombVaultin avaimen. ZFS-sivun yhteyskortti näyttää komennon, joka valtuuttaa sen; aja se kerran palvelimella.

## Kohde jää tilaan "Oppii N/10"

Useimmat poikkeamatarkistukset alkavat kohteen 10 onnistuneen varmuuskopion jälkeen, ja laskenta alkaa alusta toiminnon **Merkitse odotetuksi** jälkeen ja kun kohteen valinta on muuttunut. Kohde, jolla ei ole ajastusta, ei opi, eikä kontilla ilman appdataa ole mistä oppia, minkä sen merkki myös kertoo.

## Säilytys ei enää poista yhden kohteen vanhoja varmuuskopioita

Avoin kriittinen poikkeama pitää ne: kohteen lähde on lähes tyhjä, on kutistunut voimakkaasti tai varmuuskopio on tallentanut suurimman osan datasta uudelleen. Avaa poikkeama kohteen merkistä. Jos dataa puuttuu tai se on salattu, palauta ensin linkitetystä viimeisestä hyvästä varmuuskopiosta. Kuittaa sitten poikkeama tai merkitse se odotetuksi, jos muutos oli sinun, niin seuraava ajo siivoaa tavalliseen tapaan. Säilytyksen esikatselu merkitsee tällaisen kohteen säilytetyksi. ZFS-kohteessa vain poikkeaman nimeämä tietojoukko säilyttää vanhat varmuuskopionsa; puun muut tietojoukot siivotaan tavalliseen tapaan.

## Manuaalinen siivous kertoo, että joitakin kohteita säilytettiin

Sama syy: siivous jättää tällaisen poikkeaman kohteen vanhat varmuuskopiot rauhaan ja nimeää kohteen viestissään. Kaikki muu siivotaan tavalliseen tapaan.

## Historian tuonti kertoo, ettei tietovarastoa voitu lukea

Päivityksen jälkeen BombVault lukee kerran aiempien varmuuskopioiden koot jokaisesta tietovarastosta. Tietovarasto, jota ei silloin tavoitettu, esimerkiksi alhaalla ollut ulkoinen kohde tai liittämätön jako, näkyy kortissa **Poikkeamat** kohdassa **Asetukset, Eheys**, ja sitä yritetään uudelleen kerran päivässä. Sillä välin sen kohteet oppivat uusista varmuuskopioista.

## Levytilavaroitus ei vastaa Unraidin kojelautaa

Unraidin käyttäjäjaolla (`/mnt/user`) vapaa tila on koko arrayn, ei yksittäisen levyn. Etätietovarastot mitataan vain rclone-etäkohteiden kautta, jotka ilmoittavat vapaan tilansa; S3-, B2-, REST- ja SFTP-tietovarastoilla ei ole lukua, ja ne näkyvät kortissa **Poikkeamat** mittaamattomina.

## Tekoälyavustaja ei saa yhteyttä

Sivu [MCP-palvelin](mcp.md#troubleshooting) kertoo, mitä kukin tilakoodi ja kukin MCP-päätepisteen hylkäys tarkoittaa ja mitä asialle voi tehdä.

## Kontti käynnistyy jatkuvasti uudelleen tai näyttää epäterveeltä

BombVault raportoi terve/epäterve omasta `/api/health`-päätepisteestään. Automaattinen korjaustyökalu (kuten Autoheal) voi käynnistää sen uudelleen automaattisesti, jos moottori koskaan jumiutuu. Tarkista kontin loki ja `/spike`-raportti taustalla olevan syyn selvittämiseksi.

## Yhä jumissa?

- Lue täydelliset [Asetukset](configuration.md)- ja [Etäsijainti ja palautus](offsite-recovery.md) -sivut.
- Kysy [Unraid-tukiketjussa](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/).
- Avaa [GitHub-ongelma](https://github.com/junkerderprovinz/bombvault/issues).
