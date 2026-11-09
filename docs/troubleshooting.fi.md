# Vianmääritys

Lyhyt UKK. Täydellisen VM-over-SSH-isäntäpuolen vianmääritystaulukon (permission-denied, isäntäavaimen todennus, puuttuvat mallimuuttujat ja muuta) löydät [VM-varmuuskopiointi SSH:n yli -oppaasta](vm-backup-ssh-setup.md).

## Jokin ei ole kytketty oikein {#something-is-not-wired-up-correctly}

Lisää **Isäntäintegraatio**-kortti yleiskatsaukseen kohdassa **Mukauta**. Isäntäintegraation tarkistus koettelee jokaisen liitoksen ja komentorivityökalun (Docker-soketti, libvirt, restic, qemu-img, rclone) ja raportoi puuttuvat osat, ja **Tarkista nyt** ajaa sen uudelleen. Aloita tästä, ennen kuin oletat vian: puuttuva liitos tai tavoittamaton isäntä näkyy heti.

## En pääse verkkokäyttöliittymään {#i-cannot-reach-the-web-ui}

BombVault tarjoaa HTTPS:n valmiiksi portissa `3443` (itse allekirjoitettu varmenne), joten avaa `https://<your-unraid-ip>:3443`. Hyväksy itse allekirjoitetun varmenteen varoitus tai laita BombVault käänteisen välityspalvelimen taakse omalla varmenteellasi. Jos ajat asetuksella `HTTP_ONLY=true`, se tarjoaa tavallisen HTTP:n portissa `3000` (tarkoitettu TLS:n päättävän välityspalvelimen taakse).

## Hukkasin APP_KEYn {#i-lost-my-app_key}

`APP_KEY` johtaa resticin repon salasanan. Ilman sitä, ilman palautuspakettia ja ilman ryhmäsi avaintallennetta salattuja varmuuskopioita ei voi palauttaa. Siksi yleiskatsaus näyttää **Tallenna palautuspakettisi**, kunnes olet ladannut sen. Katso [Etäkopiot ja palautus](offsite-recovery.md#recovery-kit). Luo avain komennolla `openssl rand -hex 32` ja säilytä se palvelimen ulkopuolella, ennen kuin luotat mihinkään varmuuskopioon.

## Unohdin kirjautumissalasanan {#i-forgot-the-login-password}

Valitse kirjautumissivulla **Unohtuiko salasana?**. BombVault kirjoittaa 12 merkin koodin konttinsa lokiin (Unraidissa: Docker-välilehti, BombVaultin kuvake, **Lokit**). Kirjoita se uuden salasanan kanssa 10 minuutin kuluessa. Nollaus kytkee myös kaksivaiheisen tunnistautumisen pois, poistaa jokaisen pääsyavaimen, kumoaa jokaisen API-tokenin, MCP-avaimen ja OAuth-kirjautumisen, päättää jokaisen muun istunnon ja kytkee puhelinten pariliitoksen pois. Lisää pääsyavaimesi uudelleen kohdassa **Asetukset, Turvallisuus**, kytke puhelinten pariliitos takaisin päälle siellä ja luo uudet tokenit ja avaimet siellä, missä niitä vielä tarvitset. Katso [Tietoturvamalli](configuration.md#login).

## Varmuuskopiointi on pysähtynyt ja sanoo, ettei avainta voi enää avata {#backups-stopped-and-say-the-key-can-no-longer-be-opened}

Tämä palvelin otti toisen palvelimen varmuuskopioavaimen käyttöön Hätäohjatussa toiminnossa, ja `APP_KEY` muutettiin sen jälkeen. Käyttöön otettu avain on sinetöity vanhalla APP_KEYllä, joten BombVault ei voi avata sitä ja jättää varmuuskopioimatta sen sijaan, että kirjoittaisi väärällä avaimella. **Vaatii huomiota** yleiskatsauksessa nimeää sen. Aseta APP_KEY takaisin siihen, mikä se oli, tai lähetä palautuspaketti uudelleen Palautus-sivulla kohdassa **Hätätilanne**.

## Puhelin päätyy kirjautumissivulle {#a-phone-lands-on-the-login-page}

Pariliitetty puhelin saa istuntonsa ryhmän kautta vain niin kauan kuin **Puhelimet voivat pariutua ryhmän kahdentoista sanan avulla** on päällä kohdassa **Asetukset, Turvallisuus**. Salasanan nollaus kytkee sen pois. Palvelinta vanhempi sovellus ei myöskään saa istuntoa, ennen kuin se päivitetään. Osoitteellaan lisätty palvelin kysyy aina salasanaa. Puhelin, joka poistettiin Ilmentymät-sivulta, palaa kytkettynä pois, kun sen sovellus avataan uudelleen. Kytke päälle **Kysele tätä instanssia** kohdassa **Valvonta** sen tiedoissa päästääksesi sen sisään.

## VM-varmuuskopio ei yhdisty {#vm-backup-will-not-connect}

VM-varmuuskopiointi puhuu libvirtille SSH:n yli, ei koskaan liitoksen kautta.

- Varmista, että SSH on käytössä isännällä ja BombVaultin julkinen avain on valtuutettu tiedostossa `/root/.ssh/authorized_keys` (Asetukset, Integraatiot, Palvelimen SSH näyttää avaimen ja **Testaa yhteys** -painikkeen).
- Mukautetulla `br0.x`-verkolla aseta `LIBVIRT_HOST` Unraidisi lähiverkon IP-osoitteeksi (kontti ei tavoita isäntää siellä osoitteella `host.docker.internal`). Ota käyttöön **Asetukset, Docker, Host access to custom networks**.
- Jos vaihdoit Unraidin SSH-portin, aseta `LIBVIRT_SSH_PORT` vastaamaan sitä.
- TrueNASissa aseta `LIBVIRT_URI` TrueNASin oman libvirt-soketin kanssa, anna sovellukselle `SETPCAP`-oikeus ja lisää avain rootille kohdassa **Credentials, Users** (katso [TrueNAS SCALE](getting-started.md#truenas)).
- Täydellinen vaiheittainen diagnoosi (tavoitettavuustesti, VLAN-reititys, `Permission denied (publickey)`, `Host key verification failed`) on [VM-varmuuskopiointi SSH:n yli -oppaassa](vm-backup-ssh-setup.md).

## Live-tilannekuva virtuaalikoneesta ei ajettu {#a-live-vm-snapshot-did-not-run}

Live-tilannekuvat tarvitsevat levyn polussa `/mnt/cache` (tai `/mnt/diskX`), ei `/mnt/user`. Kun virtuaalikoneessa on qemu guest agent, tilannevedos on hiljennetty, ja ilman sitä tilannevedos on kaatumisyhdenmukainen (crash-consistent). Sammutetulla virtuaalikoneella live palaa automaattisesti hallittuun. Hallittu varmuuskopio sammuttaa virtuaalikoneen, varmuuskopioi levyt ja käynnistää sen uudelleen, joten se on aina yhdenmukainen. Se odottaa jopa kymmenen minuuttia vieraan sammumista eikä koskaan pakota sitä pois, joten päivityksiä asentavaa Windows-vierasta ei katkaista kesken kaiken. Vieras, joka silti pyörii sen jälkeen, jätetään rauhaan, ja kyseinen varmuuskopio epäonnistuu syyn kanssa.

## Varmuuskopio epäonnistui viestillä "repository is already locked" {#a-backup-failed-with-repository-is-already-locked}

Kyseessä on yleensä orpo restic-lukko, joka jäi, kun kontti päivitettiin tai käynnistettiin uudelleen kesken toiminnon. BombVault tunnistaa todistettavasti orvon lukon, purkaa sen ja yrittää kerran uudelleen, automaattisesti. Uudelleenkäynnistyksen jälkeen se odottaa, kunnes tällaista lukkoa ei ole päivitetty kymmeneen minuuttiin. Restic, joka yhä ajaa, päivittää lukkonsa viiden minuutin välein, eikä BombVault koskaan poista lukkoa, jota yhä päivitetään, koska se voi kuulua toiselle BombVaultille, joka kirjoittaa samaan repoon. Lukko, jota ei ole päivitetty 30 minuuttiin, poistetaan riippumatta siitä, kuka sen otti. Jos virhe jatkuu, avaa **Tarkista varmuuskopiot** -kortti kohdassa **Asetukset, Tarkista** ja valitse **Avaa lukitus** kohdasta **Lisää** kyseisen osion rivillä purkaaksesi vanhentuneen lukon käsin. Myös **Avaa lukitus** jättää yhä päivitettävän lukon paikalleen ja kertoo, milloin sitä viimeksi päivitettiin. Aito ongelma tulee silti esiin eikä piiloudu.

## Etäkopiointi ei tapahtunut varmuuskopion jälkeen {#my-off-site-copy-did-not-happen-after-a-backup}

Etäreplikointi on tarkoituksella paras mahdollinen yritys, joten etäpään hikka ei koskaan kaada paikallista varmuuskopiota. Tarkista, milloin kohde kopioi: sen **Kopioi**-kortti sanoo **Jokaisen varmuuskopion jälkeen** tai nimeää oman aikataulun, ja **Asetukset, Aikataulut, Etäkopiot osioittain** voi antaa osiolle oman aikataulun. Käytä **Replikoi nyt** osion rivillä **Etäkopiot**-kortissa kohdassa **Asetukset, Kohteet** ajaaksesi kopioinnin pyydettäessä, ja seuraa yleiskatsauksen tilapäätä, joka kertoo **Etäkopiointi käynnissä**, kun se on käynnissä. Epäonnistunut kopio näkyy kohdassa **Vaatii huomiota**.

## Rest-palvelimen varmenteeseen ei luoteta {#a-rest-servers-certificate-is-not-trusted}

restic hylkää varmenteen, jota se ei tunne, ja virhe kertoo, minne se pitää laittaa. Kopioi rest-palvelimen itse allekirjoitettu varmenne tai sen allekirjoittaneen tahon varmenne PEM-tiedostona BombVaultin datakansion `certs`-kansioon (`/config/certs` kontissa, `appdata/bombvault/certs` Unraidissa) ja testaa uudelleen. Uudelleenkäynnistystä ei tarvita.

## Varmuuskopio kertoo, ettei Host Datan ulkopuolista dataa varmuuskopioitu {#data-outside-host-data}

Kontti pitää dataa kansiossa, jota BombVault ei voi lukea Host Datan kautta. Varmuuskopio tallentaa määrityksen ja jokaisen kansion, johon se yltää, muuttaa rivinsä toimintolokissa oranssiksi ja nimeää kansiot, jotka se jätti pois. Kaksi tapausta on yleistä: ixVolume TrueNASissa polussa `/mnt/.ix-apps` ja nimetty Docker-taltio polussa `/var/lib/docker/volumes`, kuten compose-stackin tietokantataltio. Kontin **Asetukset**-välilehden **Varmuuskopioitavat kansiot** -kortti listaa jokaisen tällaisen kansion polkuineen kontin sisällä ja isännällä, punaisella merkittynä.

Nimetöntä Docker-taltiota ei raportoida. Docker luo sellaisen itse kansiolle, jonka image ilmoittaa ja jota kukaan ei liittänyt, nimeää sen 64 heksadesimaalimerkillä ja pitää sen polussa `/var/lib/docker/volumes`. Se sisältää yleensä välimuistin, joten varmuuskopio pysyy vihreänä ja kortti vain listaa sen huomautuksella, että se on nimetön.

- **Haluat tämän datan varmuuskopioitavan.** Liitä kansio isännän polkuun Host Datan alla, esimerkiksi kansioon appdata-jaossasi, ja seuraava varmuuskopio sisältää sen.
- **Et tarvitse sitä.** Lisää jompikumpi sen kahdesta polusta riviksi **Poissulkumallit**-korttiin samalla välilehdellä: polku kontin sisällä, kuten `/cache`, tai isännän polku, jonka varoitus nimeää, kuten `/var/lib/docker/volumes/jellyfin_cache/_data`. Seuraavasta varmuuskopiosta alkaen varoitus jättää kyseisen kansion pois, ja kentän alla oleva esikatselu kertoo syyn. Rivi sen yläpuolella olevalle kansiolle toimii myös, pelkkä nimi tai `/` ei.

## Varmuuskopio sanoo Lykätty {#deferred-backups}

Kontin sovellus oli kiireinen, joten sen varmuuskopio odottaa ja sitä yritetään itsestään uudelleen. Toimintolokin rivi kertoo syyn ja milloin seuraava yritys on. Kontin **Asetukset**-välilehden **Kun sovellus on kiireinen** -kortti kertoo saman ja sen, milloin odotus päättyy. Mitään ei tarvitse tehdä. Varmuuskopioidaksesi heti käytä **Varmuuskopioi nyt**: itse käynnistämäsi varmuuskopio ei koskaan odota, ei myöskään poistumiskoodin 75 vuoksi.

- **Oma tarkistuksesi.** Tee **Varmuuskopiota edeltävästä komennosta** sellainen, että se päättyy koodiin 75, kun sovellus on kiireinen, esimerkiksi `[ "$(cat /tmp/players)" = 0 ] || exit 75`. Mikä tahansa muu koodi, joka ei ole 0, kaataa varmuuskopion kuten ennenkin, joten rikkinäinen skripti näkyy edelleen kohdassa **Vaatii huomiota**.
- **Plexiltä, Jellyfiniltä tai Embyltä ei koskaan kysytä.** BombVault kysyy ensin palvelimelta kontin omassa osoitteessa ja ajaa, kun se ei vastaa, curlin tai wgetin kontin sisällä. Image, jossa ei ole kumpaakaan, ei voi kysyä itsensä kautta. Loki nimeää silloin tämän. Kun **Varmuuskopioi, kun palvelin ei vastaa** on päällä, tällainen varmuuskopio ajetaan silti, pois päältä se odottaa.
- **Jellyfin tai Emby hylkää.** Tarkista **API-avain**. Se tehdään palvelimen kojelaudan kohdassa API keys.
- **Plex hylkää.** BombVault lukee tunnisteen tiedostosta `Preferences.xml` kansiosta, joka on liitetty polkuun `/config`. Palvelimella, johon ei koskaan kirjauduttu, sellaista ei ole, ja se vastaa vain siellä, missä se sallii pääsyn ilman.
- **Tauolle jätetty elokuva pitää varmuuskopion odottamassa koko yön.** Aseta **Ohita tauotetut istunnot (min) jälkeen**, esimerkiksi arvoon 30.
- **Liikennetarkistus ei koskaan päästä varmuuskopiota läpi.** Nosta **Kiireinen alkaen (KiB/s)** korkeammaksi kuin mitä kontti lähettää, kun kukaan ei käytä sitä. Heti BombVaultin käynnistyksen jälkeen tarkistus odottaa, kunnes se on seurannut konttia **Hiljaisena (min)** minuuttia.
- **Varmuuskopio ei koskaan ajaudu yöllä.** Kun aikaikkuna on asetettu, yritykset tulevat vain sen sisällä. Odotus, joka saavuttaa määräaikansa ikkunan ulkopuolella, päättyy silti määräaikaan.

## Suuri varmuuskopio tapettiin muistin loppumisen vuoksi {#a-large-backup-was-killed-for-running-out-of-memory}

Aseta **Muistiraja varmuuskopiota kohden (MiB, 0 = ei rajaa)** **Suorituskyky**-kortissa kohdassa **Asetukset, Järjestelmä**, reilusti sen alapuolelle, mitä kontti saa käyttää. Rajan lähellä restic vapauttaa muistia useammin, mikä maksaa hieman nopeutta, joten varmuuskopio hidastuu sen sijaan, että se tapettaisiin.

## Palautus keskeytyi ennen alkua {#a-restore-aborted-before-it-started}

Ennen kuin mitään pysäytetään tai poistetaan, palautus ajaa ennakkotarkistuksen ristiriidoille: se varmistaa, että kontin staattinen IP ja julkaistut isäntäportit ovat vapaina. Jos toinen kontti jo pitää jompaakumpaa, palautus keskeytyy selkeään, toimintaan johtavaan viestiin sen sijaan, että jättäisi puolivalmiin palautuksen. Vapauta ristiriitainen portti tai IP ja yritä uudelleen.

Jos kontin verkkoa ei ole olemassa, esimerkiksi koska sen TrueNAS-sovellus poistettiin, palautustarkistus nimeää verkon ja palautus kieltäytyy ennen kuin se muuttaa mitään. Asenna sovellus TrueNASissa uudelleen samalla nimellä, mikä luo verkon uudelleen, ja palauta sitten sen data.

## Selkeä vienti epäonnistui tiedoston kirjoittamisen sijaan {#a-plain-export-failed-instead-of-writing-a-file}

Jos **Salaa viennit age:lla** on päällä kohdassa **Asetukset, Järjestelmä** mutta kelvollista vastaanottajaa ei ole asetettu, vienti epäonnistuu selkeään virheeseen sen sijaan, että kirjoittaisi selkotekstiä. Lisää kelvollinen vastaanottaja (age-julkinen avain tai SSH-julkinen avain) tai kytke salaus pois, jos tarkoitat viennin olevan selkotekstiä. Katso [Selkeät viennit](backups.md#exports).

## Tietokantavedos epäonnistui {#a-database-dump-failed}

Epäonnistunut vedos ei koskaan kaada sen ympärillä olevaa varmuuskopiota. Se kirjataan omaksi epäonnistuneeksi ajokseen, ja syy nimeää korjattavan asian.

- **Kirjautuminen hylättiin.** Vedos kirjautuu kontin omilla salasanamuuttujilla (`POSTGRES_PASSWORD`, `MARIADB_ROOT_PASSWORD`, `MYSQL_ROOT_PASSWORD` tai niiden `_FILE`-versiot). Tarkista ne tietokantakontilta. `_FILE`-muuttuja, joka osoittaa salaisuuteen, jota kontin oma käyttäjä ei voi lukea, epäonnistuu samoin.
- **Puuttuvat oikeudet.** Satunnaisella root-salasanalla vedos voi kirjautua vain sovelluskäyttäjänä, joten se kattaa vain sen yhden tietokannan, ja MySQL 8.4 ja uudemmat voivat kieltäytyä siitä kokonaan. Anna kontille oikea root-salasana tai kytke vedos pois sen osalta.
- **Järjestelmätaulut tarvitsevat päivityksen.** MariaDB kieltäytyy vedostamasta, kun sen järjestelmätaulut ovat vanhemmasta versiosta (virhe 1558). Lisää muuttuja `MARIADB_AUTO_UPGRADE=1` ja käynnistä kontti uudelleen tai aja `mariadb-upgrade` sen sisällä kerran.
- **Ei vedostyökalua.** Kevyttä tai itse rakennettua imagea, jossa ei ole `pg_dump`, `mysqldump` tai `mariadb-dump`, ei voi vedostaa. Käytä virallista imagea tai kytke vedos pois.
- **Aikaraja.** Vedos saa `DB_DUMP_MAX_HOURS` (oletuksena 6), sen ympärillä oleva varmuuskopio saa `BACKUP_MAX_HOURS`, ja vedos, joka lakkaa edistymästä, katkaistaan `BACKUP_STALL_HOURS` jälkeen. Viimeisen tavallinen syy on lukko, jota sovellus pitää. Nosta rajaa, joka lauennut, tai vedosta, kun sovellus on hiljaa.
- **Kontti on tauolla tai käynnistyy uudelleen.** Vedos puhuu käynnissä olevalle palvelimelle. Jos kontti käynnistyy jatkuvasti uudelleen, sen oma loki kertoo syyn.
- **Vahingoittunutta vedosta ei voitu poistaa.** Vedos, jota BombVault ei saanut valmiiksi, poistetaan uudelleen. Kun poisto epäonnistuu, vedos jää listaan vahingoittuneeksi merkittynä, ja voit poistaa sen siellä.

## Tuonti epäonnistui {#an-import-failed}

Tuonti pysäyttää kontin, siirtää sen datakansion sivuun ja antaa imagen luoda tyhjän tilalle. Jos tuontia edeltävä vaihe epäonnistuu, vanha kansio laitetaan automaattisesti takaisin. Jos tuonti epäonnistuu, kontti pitää uuden kansion ja vanha jää sen viereen nimellä `<data folder>.bombvault-before-import-<timestamp>`. Ajon virheviesti nimeää tarkan polun.

Datakansiota, joka on oma tietojoukkonsa, kuten TrueNASissa, ei voi nimetä uudelleen. Tuonti jättää silloin kansion paikalleen ja siirtää sen sisällön viereen säilytettyyn kansioon, joten yläpuolella olevan tietojoukon täytyy mahtua ne. Tuonti tarkistaa sen, ennen kuin se pysäyttää kontin.

Laita se takaisin käsin: pysäytä kontti, nimeä nykyinen datakansio sivuun, nimeä säilytetty kansio takaisin alkuperäiseen nimeen ja käynnistä kontti. Tietojoukolle siirrä sisältö kansioiden uudelleennimeämisen sijaan. Unraidissa tiedostonhallinta tekee tämän Shares-välilehdeltä.

## ZFS-tietojoukon varmuuskopio epäonnistui tai ohitti tietojoukon {#zfs-datasets}

Jokaisella ongelmalla on syykoodi hakasulkeissa, ja [ZFS-tietojoukot](zfs-datasets.md#reason-codes)-sivu listaa ne kaikki korjausohjeineen. Kolme yleisintä:

- **`snapshot-loop`**: tilannevedos ei päässyt BombVaultiin, koska Host Data ei välitä uusia liitoksia. Muokkaa konttia, aseta Host Datan Access Mode arvoon Read/Write - Slave ja käynnistä BombVault uudelleen.
- **`key-not-loaded`**: salattu tietojoukko, jonka avainta ei ole ladattu, ohitetaan. Lataa avain komennolla `zfs load-key` ja liitä tietojoukko. Seuraava varmuuskopio sisältää sen.
- **`ssh-auth`**: palvelin hylkäsi BombVaultin avaimen. **ZFS-yhteys tähän palvelimeen**, joka avataan painikkeella **Näytä yhteys** Varmuuskopiot-sivulla, näyttää komennon, joka valtuuttaa sen. Aja se kerran palvelimella.

## Merkintä jää tilaan "Oppii N/10" {#an-entry-stays-at-learning-n10}

Useimmat poikkeamatarkistukset alkavat merkinnän 10 onnistuneen varmuuskopion jälkeen, ja laskenta alkaa alusta kohdan **Merkitse odotetuksi** jälkeen ja kun merkinnän valinta muuttui. Merkintä, jota ei ole ajastettu, ei opi, eikä kontilla, jolla ei ole appdataa, ole mistä oppia, minkä sen merkki kertoo. Poikkeamat-sivun **Oppimisen edistyminen** näyttää kymmenen osan renkaan merkintää kohti.

## Säilytys lakkasi poistamasta yhden merkinnän vanhoja varmuuskopioita {#retention-stopped-deleting-old-backups-of-one-entry}

Avoin kriittinen poikkeama pitää niitä: merkinnän lähde on lähes tyhjä, kutistui jyrkästi, tai yksi varmuuskopio tallensi suurimman osan datastaan uudelleen. Avaa poikkeama merkinnän merkistä. Jos dataa puuttuu tai se salattiin, palauta ensin linkitetystä viimeisestä hyvästä varmuuskopiosta. Kuittaa sitten poikkeama tai merkitse se odotetuksi, jos muutos oli sinun, ja seuraava ajo karsii tavalliseen tapaan. Säilytyksen esikatselu merkitsee tällaisen merkinnän säilytettäväksi. ZFS-merkinnällä vain poikkeamassa nimetty tietojoukko pitää vanhat varmuuskopionsa, ja puun muut tietojoukot karsitaan tavalliseen tapaan.

## Käsin tehty karsinta sanoo, että joitakin merkintöjä säilytettiin {#manual-prune-says-some-entries-were-kept}

Sama syy: karsinta jättää rauhaan merkinnän vanhat varmuuskopiot, kun sillä on tällainen poikkeama, ja nimeää merkinnän viestissään. Kaikki muu karsitaan tavalliseen tapaan.

## Varmuuskopio epäonnistuu viestillä "on no mounted pool or share" {#no-mounted-pool}

Unraidissa `/mnt` on muistissa, joten siellä oleva kansio, johon ei ole liitetty poolia, katoaa seuraavassa uudelleenkäynnistyksessä ja jää piiloon, kun pooli on taas paikallaan. BombVault kieltäytyy kirjoittamasta sinne varmuuskopiota tai flashin ZIP-tiedostoa, myös silloin, kun kansiossa on jo aiemman kirjoituksen repo. Liitä pooli ja aja varmuuskopio uudelleen.

## Historian tuonti sanoo, ettei reposta voitu lukea {#history-import-says-a-repository-could-not-be-read}

Päivityksen jälkeen BombVault lukee aiempien varmuuskopioiden koot jokaisesta reposta kerran. Repo, jota se ei tavoittanut tuolloin, esimerkiksi etäkohde, joka oli alhaalla, tai jako, jota ei ollut liitetty, lasketaan **Poikkeamat**-kortissa kohdassa **Asetukset, Tarkista** ja yritetään uudelleen kerran päivässä. Sen merkinnät oppivat sillä välin uusista varmuuskopioista.

## Levytilavaroitus ei vastaa Unraidin kojelautaa {#the-disk-space-warning-does-not-match-the-unraid-dashboard}

Unraidin käyttäjäjaolla (`/mnt/user`) vapaa tila on koko arrayn, ei yhden levyn. Etäkohteet, joihin BombVault yltää rclonen kautta, kuten SFTP, Hetzner Storage Box, SMB, WebDAV tai pilviasema, mitataan, kun toinen pää raportoi vapaan tilansa, samoin käsin kirjoitettu `sftp:`-osoite. S3-, B2- ja REST-repoilla ei ole lukua: niiden kohde sanoo **Koko tuntematon**, ja **Poikkeamat**-kortti listaa ne mittaamattomina.

## Tekoälyavustaja ei saa yhteyttä {#an-ai-assistant-cannot-connect}

[MCP-palvelimen sivu](mcp.md#troubleshooting) listaa, mitä kukin tilakoodi ja jokainen MCP-päätepisteen kieltäytyminen tarkoittaa ja mitä sille voi tehdä.

## Kontti käynnistyy jatkuvasti uudelleen tai näyttää huonokuntoiselta {#the-container-keeps-restarting-or-looks-unhealthy}

BombVault raportoi kunnossa/huonossa kunnossa omasta `/api/health`-päätepisteestään. Automaattinen parantaja (kuten Autoheal) voi käynnistää sen uudelleen itsestään, jos moottori joskus jumittuu. Tarkista kontin loki ja **Isäntäintegraatio**-kortti taustalla olevan syyn selvittämiseksi.

## Kohde kieltäytyy tallentumasta {#location-refused}

BombVault tarkistaa kohteen ennen kuin tallentaa sen, ja kieltäytyminen kertoo syyn yhdellä lauseella. Tavallisimmat:

- **"This location lies inside another repository or target, or contains one."** Repo repon sisällä rikkoisi molemmat. Viesti nimeää toisen paikan. Valitse toinen kansio tai poista se ensin.
- **Se on repo, jonka olet määrittänyt.** Kohde on yksi **Arkistot**-kohdan riveistä tai on sellaisen sisällä tai ympärillä. Valitse toinen paikka tai poista ensin kyseinen repo.
- **Sama paikka kuin osion oma kansio.** Kopio osion omaan kansioon kopioisi sen varmuuskopiot itselleen jokaisella ajolla, joten se kielletään. Valitse toinen paikka tai katso [Osion oma kansio](offsite-recovery.md#own-folder).
- **"That is not an off-site target of this domain"** tuli linkistä kohteeseen, joka oli poistettu. Nykyinen versio tyhjentää nämä alussa ja tuonnissa. Jos näet sen yhä, päivitä BombVault.
- **Koko isännän polku**, kuten `/mnt/user/backups`. BombVault ajaa kontissa, joten se haluaa polun Host Datan alta, `user/backups`, ja viesti nimeää sen, joka kirjoitetaan.

## Etäkohde sanoo, että sen repo on poissa {#repository-gone}

Kohde, jossa oli varmuuskopioita ja jossa yhtäkkiä ei ole reposta, ei saa uutta tyhjää itsestään, koska se piilottaisi oikean, kun levy palaa. Tavallinen syy on rest-palvelin, jonka levyä ei liitetty, joten liitä levy uudelleen ja aja varmuuskopiointi tai kopiointi vielä kerran. Vain jos tyhjensit tai poistit vanhan repon tarkoituksella, paina kohteen sivulla **Luo tähän uusi arkisto**. Se aloittaa alusta kaikille kyseisen kohteen osioille kerralla.

## Pilvikirjautuminen vanheni {#cloud-sign-in-expired}

Testi tai seuraava kopio kertoo silloin, että kirjautuminen vanheni. **Muuta yhteyttä** kohteen sivulla kirjaa sisään uudelleen.

- **Box, Jottacloud ja muut kertakäyttöisillä virkistystunnisteilla:** BombVault pitää tunnisteen, jonka rclone virkisti, myös uudelleenkäynnistysten yli ja palautuspaketissa. Vanhemmasta asetusvarmuuskopiosta tehdyn palautuksen jälkeen siinä oleva tunniste on voinut vanhentua, joten kirjaudu sisään vielä kerran. Repo pysyy ennallaan.
- **iCloud Drive:** Apple päättää kirjautumisen 30 päivän jälkeen. Se tarvitsee uuden luottamustunnisteen ja evästeet komennolla `rclone config`.
- **OneDrive:** useamman aseman kirjautuminen kysyy, mitä asemaa käytetään, joten kohde, jolle ei tehty valintaa, pysyy käyttökelvottomana, kunnes valitset. OneDrive Businessin ja Google Driven osalta katso karsitun datan huomautus kohdasta [Kohteet](offsite-recovery.md#destinations).

## "Viimeisen varmuuskopion etäkopio on yhä käynnissä" {#copy-still-running}

Sitä edeltävä varmuuskopio valmistui, ja sen kopio on yhä matkalla, mikä estää kyseisen osion uuden varmuuskopion. Odota, kunnes tilapää ei enää sano **Etäkopiointi käynnissä**. Varmuuskopioinnin käynnistävät painikkeet kertovat saman odottaessaan. Ensimmäinen kopio uuteen kohteeseen voi kestää tunteja, katso [S3-esimerkki](offsite-recovery.md#example-s3).

## ZFS-replika pysähtyy, koska kohteessa on uudempia tilannevedoksia {#replica-target-newer}

Kohteessa on tilannevedoksia, jotka ovat uudempia kuin uusin, jonka se jakaa tämän palvelimen kanssa, esimerkiksi koska tietojoukko palautettiin täällä aiempaan tilaan tai kohteen kopiota muutettiin. Replika-kortti ja ajohistoria nimeävät tilannevedoksen kohdassa **Palauta kohde tilaan**. Aja `zfs rollback -r` kohteessa kyseiseen tilannevedokseen jokaiselle tietojoukolle, jonka ajo nimeää, ja paina **Replikoi nyt**. Katso [Kuinka kauan tilannevedokset säilyvät](zfs-datasets.md#replica-retention).

## Unraid-tilannevedoksilla varustettu virtuaalikone epäonnistuu ja nimeää tiedoston {#vm-chain}

BombVault seuraa jokaisen levyn taustaketjua ja varmuuskopioi jokaisen sen tiedoston. Ketju, jota ei voi seurata loppuun, kaataa varmuuskopion sen sijaan, että jättäisi osan levystä pois, ja viesti nimeää tiedoston. Tarkista, että tiedosto on olemassa polun `/mnt` alla ja että tilannevedos on yhä listattuna VM Managerissa. Tiedostoa Host Datan ulkopuolella ei voi lukea lainkaan. Katso [Unraid-tilannevedoksilla varustetut VM:t](vm-backup-ssh-setup.md#unraid-snapshots).

## Etäkopiot odottavat päätöstäsi uudelleenrakennuksen jälkeen {#copies-paused}

Toiminnon **Tunnista varmuuskopiot** tai Hätäohjatun toiminnon jälkeen, kun asetuksia ei palautettu, tai kun osio löytää omia varmuuskopioitaan ajalta ennen tätä tietokantaa, osion etäkopiot odottavat, koska BombVault ei enää tiedä, mitkä merkinnät jätit pois. **Vaatii huomiota** ja kohteen sivu kysyvät asiasta. Paina **Ota ne tämän palvelimen omiksi** jatkaaksesi tai **Tarkista ensin säännöt** katsoaksesi osion sääntöjä ennen sitä. Asetustiedoston tuonti ei tee päätöstä puolestasi. Katso [Uudelleenrakennuksen jälkeen](offsite-recovery.md#after-a-rebuild).

## Ilmoitukset eivät saavu perille {#notifications-do-not-arrive}

Aloita kanavan painikkeesta **Lähetä testi** kohdassa **Asetukset, Ilmoitukset**. Kanavan alla oleva rivi kertoo, mitä tapahtui.

- **Mitään ei lähde lainkaan.** Tarkista, että kanava on päällä, ja katso sen **Ilmoita**-valinta. Arvolla **Ei koskaan** se ei lähetä mitään, ja arvolla **Vain epäonnistuessa** onnistunut ajo pysyy hiljaa. Kun **Tiivistä ajastetut ajot** on päällä, ajastettu ajo lähettää yhden yhteenvedon loppuunsa.
- **Uusi osoite.** Kun muutat, minne kanava lähettää, esimerkiksi Matrixin, Gotifyn tai ntfy:n palvelimen tai sähköpostikanavan isännän, kenttä kysyy tunnistetta tai salasanaa uudelleen, ja uusi osoite tallentuu vasta, kun kirjoitat sen. BombVault ei lähetä vanhaa salaisuutta uuteen paikkaan.
- **Gotify** sanoo, että palvelin hylkäsi sovellustunnuksen: kopioi se uudelleen Gotifyn sovelluksesta. **Palvelimen URL** on perusosoite ilman `/message`.
- **Discord, Slack ja ntfy** ovat omia kanaviaan. **Webhook**-kanavan Discord-osoite saa JSONia, jota Discord ei ymmärrä, joten lisää sitä varten **Discord**-kanava.
- **Webhook:** paikkamerkki, jota BombVault ei tunne, jää sellaisekseen, joten tarkista kirjoitusasu, esimerkiksi `%%title%%`. Pidä JSON-rungossa `Content-Type`-otsakkeen arvona `application/json`, jotta arvot koodataan JSONina.
- **Unraid-ilmoitukset** kulkevat isännän SSH-yhteyden kautta, joten testaa se ensin kohdassa **Asetukset, Integraatiot**. Muulla alustalla ne pysyvät pois päältä, ja kanava kertoo sen.
- **Ilmoitussähköposti ei koskaan tule.** Tarkista **Salaus** suhteessa porttiin, yleensä **STARTTLS** portissa 587 tai **TLS (implicit)** portissa 465.

Healthchecks-pingit eivät riipu **Ilmoita**-valinnasta, katso [Healthchecks](notifications.md#healthchecks).

## Android-sovellus ei avaa palvelinta {#android-cannot-open}

Aloitusnäyttö kertoo syyn palvelimen kortilla:

- **Ei yhteyttä:** palvelimen osoite eikä rele vastaa. Arvolla **Ei relettä** sovellus tavoittaa palvelimen vain sen omassa osoitteessa.
- **Tarkista varmenne:** palvelin näyttää toisen varmenteen kuin sen, johon luotit. Sovellus avaa sen vasta, kun luotat uuteen, joten varmista, että muutos on sinun.
- **Kirjautuminen tarvitaan:** osoitteella lisätty palvelin kysyy salasanaa. Katso myös [Puhelin päätyy kirjautumissivulle](#a-phone-lands-on-the-login-page).
- **Avain muuttui:** aloitusnäyttö näyttää uuden avaimen sormenjäljen, ja sivu aukeaa vasta, kun luotat siihen. Katso [Kodin ulkopuolella](android.md#away).

## Toinen käyttäjä ei voi lukea paikallista repoa {#repository-permissions}

Paikallinen repo pysyy luettavana muille käyttäjille, kuten SMB:n yli synkronoivalle käyttäjälle. Ennen versiota 10.4.1 tiedostot, jotka karsinta, tunniste, nouto, osion siirto, poisto tai stackin varmuuskopio kirjoitti, saattoivat jäädä lukukelvottomiksi seuraavaan yöhön. Päivitä BombVault, niin ne ovat luettavissa heti. Itsevarmuuskopion repo pysyy tarkoituksella vain rootin luettavana, koska se sisältää BombVaultin asetukset.

## Yhä jumissa? {#still-stuck}

- Lue koko [Kokoonpano](configuration.md)- ja [Etäsijainti ja palautus](offsite-recovery.md) -sivut.
- **Virheraportti** **Ohje**-kortissa kohdassa **Asetukset, Tietoja** lataa ZIP-tiedoston GitHub-issuea varten: isäntätarkistus, kokoonpanosi jokainen salaisuus poistettuna, viimeisimmät ajot, mikä on seuraavaksi ajastettu ja viimeaikainen loki siitä lähtien, kun kontti viimeksi käynnistyi. Siinä on myös jokaisen tietokannan viimeisin vedos, ZFS-merkinnät liitoksineen, avoimet poikkeamat ja se, kuinka monta MCP-avainta on olemassa. Salasanat, tunnisteet ja rclone-kokoonpano poistetaan, ja paketti kertoo sen omassa manifestissaan. Se vaatii kirjautumissalasanan. Kontin uudelleenkäynnistäneestä kaatumisesta katso `docker logs`.
- Kysy [Unraidin tukisäikeessä](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/).
- Avaa [GitHub-issue](https://github.com/junkerderprovinz/bombvault/issues).
