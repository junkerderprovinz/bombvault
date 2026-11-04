# Ominaisuudet

Lyhyt luettelo siitä, mitä BombVault tekee. Jokainen rivi linkittää sivulle, joka selittää sen.

## Varmuuskopioinnin laajuus {#backup-scope}

- **Kontit**: appdata-kansio ja kontin määritys, kansiot valittavissa konttikohtaisesti. [Varmuuskopiot ja merkinnät](backups.md#what-is-saved)
- **VMs**: levykuvat, XML ja UEFI NVRAM SSH:n yli, hallitulla sammutuksella, live-tilannekuvalla tai vain muuttuneilla lohkoilla. Unraid-tilannevedoksilla ja zvol-levyillä varustetut virtuaalikoneet katetaan kokonaan. [VM-varmuuskopiot](vm-backup-ssh-setup.md)
- **Flash**: koko Unraidin USB-flash, ajastetulla zip-viennillä. [Varmuuskopiot ja merkinnät](backups.md#settings-tab)
- **Itsevarmuuskopio**: BombVaultin omat asetukset ja tunnukset. [Hätäohjattu toiminto](offsite-recovery.md#emergency)
- **Kansiot**: mikä tahansa palvelimen kansio kansiojoukkona. [Kansion lisääminen](backups.md#add)
- **ZFS-tietojoukot**: tietojoukkopuu luettuna yhdestä tilannevedoksesta, yksittäiset tiedostot palautettavissa ja valinnainen replika toisella ZFS-palvelimella. [ZFS-tietojoukot](zfs-datasets.md)
- **Tietokantavedokset**: PostgreSQL, MySQL ja MariaDB, vedostettuna käynnissä olevalta palvelimelta omaksi palautuspisteekseen. [Tietokantavedokset](backups.md#database-dumps)

## Varmuuskopiot ja merkintäsivut {#backups}

- **Yksi lista** ruutuineen, kohdilla **Suodattimet**, **Lajittelu** ja **Hae**, ja jokaisen rivin neljätoista viimeistä ajoa nauhana. [Lista](backups.md#list)
- **Valinta kuten tiedostonhallinnassa**, ja valinnan varmuuskopiointi tai keskeytys palvelimella. [Useiden merkintöjen valinta](backups.md#select)
- **Sivu jokaiselle merkinnälle** kohdilla **Varmuuskopiot**, **Asetukset** ja **Tarkista**. [Merkinnän sivu](backups.md#entry-page)
- **Tunnisteet ja vertailu** tilannevedoksille sekä **Vienti** tar-, zip- tai restic-repona. [Varmuuskopiot-välilehti](backups.md#backups-tab)
- **Poissulkumallit** reaaliaikaisella esikatselulla ja suositellut poissulut tunnetuille sovelluksille. [Poissulkumallit](backups.md#excludes)
- **Komennot ennen ja jälkeen** sekä **Pysäytä muut säiliöt** kunnon mukaan etenevällä uudelleenkäynnistyksellä riippuvuusjärjestyksessä. [Komennot ja muut kontit](backups.md#commands)
- **Kun sovellus on kiireinen**: ajastettu varmuuskopio odottaa Plexiä, Jellyfiniä, Embyä, omaa tarkistustasi tai hiljaista liikennettä ja yrittää itsestään uudelleen. [Kun sovellus on kiireinen](backups.md#app-busy)
- **Tilannevedos pysäytyksen sijaan**: ZFS:llä oleva kontti on alhaalla vain muutaman sekunnin. [Aikataulut](schedules.md#snapshot-instead-of-stop)
- **Selkeät viennit**, halutessasi age:lla sinetöityinä. [Selkeät viennit](backups.md#exports)

## Palautus {#restore}

- **Yksi sivu ja yksi ikkuna** kaikille lajeille, suositeltu varmuuskopio valmiiksi valittuna. [Palautus](restore.md)
- **Alkuperäisen viereen** oletuksena, ja **Korvaa alkuperäinen** varmuuskopioi nykytilan ensin. [Palautusikkuna](restore.md#window)
- **Yksittäiset tiedostot** kansioon tai takaisin paikalleen. [Yksi tiedosto takaisin](restore.md#example-single-file)
- **Useita merkintöjä kerralla**, kukin suositellusta varmuuskopiostaan tai yhdestä hetkestä. [Useiden merkintöjen palautus](restore.md#several)
- **Kokonaiset Compose-stackit**, käynnistettyinä riippuvuusjärjestyksessä. [Stackin palautus](restore.md#stacks)
- **Kontit ja virtuaalikoneet luodaan uudelleen** imagelle, jolla ne ajoivat, GPU:n, rajojen ja linkkien kanssa. [Mitä palautuu](restore.md#kinds)
- **Tarkistukset ennen palautusta** ja lista siitä, mitä se muuttaa. [Palautusikkuna](restore.md#window)
- **Flash ZIP-tiedostona**, ja yksi liitännäinen kerrallaan takaisin käynnissä olevalle flashille. [Mitä palautuu](restore.md#kinds)
- **ZFS** tietojoukkoon turvatilannevedoksella, kansioon tai uuteen tietojoukkoon ominaisuuksineen. [ZFS-tietojoukot](zfs-datasets.md#restore)
- **Hätäohjattu toiminto** kadonneelle palvelimelle, palautuspaketilla, ryhmän kautta tai kirjoitetulla avaimella. [Hätäohjattu toiminto](offsite-recovery.md#emergency)
- **Vanhasta varmuuskopiokansiosta** ja **Tuo Appdata.Backup-liitännäisestä**. [Muut lähteet](restore.md#other-sources)

## Tallennus ja ajastus {#storage-scheduling}

- **Kohteet** kaikenlaisina: paikalliset kansiot, SMB, WebDAV, SFTP, rest-palvelin, S3, pilvilevyt ja mikä tahansa rclone-remote. [Kohteet](offsite-recovery.md#destinations)
- **Tunnukset**: yksi lista oletuksella. [Tunnukset](offsite-recovery.md#sign-ins)
- **Omat kansiot** tällä palvelimella tai kohteessa. [Osion oma kansio](offsite-recovery.md#own-folder)
- **3-2-1 kulkuna**, jossa jokainen rivi on kytkin, ja **sijoittelu merkinnän mukaan**. [Sijoittelu merkinnän mukaan](offsite-recovery.md#placement)
- **Etäkopiot** useaan kohteeseen osiota kohti, kaistanleveysrajoin, pakkauksella, kylmillä tallennusluokilla ja hidastuksella Plexin striimatessa. [Etäkopiot](offsite-recovery.md#off-site-copies)
- **Suorat arkistot**, listattuina kohdassa **Varmuuskopioidaan suoraan tänne**. [Suorat arkistot](offsite-recovery.md#direct-repositories)
- **Säilytys aikajanana**: **Lyhyt**, **Tasapainoinen**, **Pitkä** tai **Omat arvot** kohdetta kohti, säännöt osioittain. [Kuinka kauan säilytetään](offsite-recovery.md#keep)
- **Osion historian siirto** ja **Poista kohde** varmuuskopioineen tai ilman. [Siirto ja poisto](offsite-recovery.md#move-remove)
- **Lauseina luettavat aikataulut**, **Viikko**-palkki, täysvarmuuskopio, osioiden aikataulut ja oma aikataulu merkinnälle. [Aikataulut](schedules.md)
- **Väliin jääneiden varmuuskopioiden ajaminen**, käsin asetettu järjestys ja päivitykset onnistuneen varmuuskopion jälkeen. [Aikataulut](schedules.md)

## Näkyvyys, tarkistus ja valvonta {#insight-verification-monitoring}

- **Yleiskatsaus**: tilapää, **Vaatii huomiota**, **Suojaustila**, tallennustilan ennuste, historia ja valinnaiset kortit. [Yleiskatsaus](overview.md)
- **Poikkeamien tunnistus** merkintää kohti, joka säilyttää myös vanhat varmuuskopiot, kun lähde romahtaa. [Poikkeamat](anomalies.md)
- **Palautustarkistus** ensimmäisen varmuuskopion jälkeen ja **Käynnistystesti** eristetyssä kopiossa. [Tarkista-välilehti](backups.md#verify-tab)
- **Ajastetut tarkistukset ja koepalautukset** etäkopiosta. [DR-harjoitukset](offsite-recovery.md#dr-drills)
- **Koko kansioittain** ja se, miksi varmuuskopio oli hidas. [Mitä ajo kertoo sinulle](backups.md#run-notes)
- **Ilmoitukset**: webhook, Gotify, Matrix, sähköposti, Apprise, Unraid ja Healthchecks, viikkokoosteella ja myöhässä-raportilla. [Ilmoitukset](notifications.md)
- **Prometheus**, Home Assistant -laite, HTTP-API ja mDNS. [API ja integraatiot](api.md)
- **Virheraportti**: sensuroitu ZIP GitHub-issuea varten. [Vianmääritys](troubleshooting.md#still-stuck)

## Kiristysohjelmasuoja {#ransomware-protection}

- **Poistosuoja (append-only)**, jonka toinen pää panee täytäntöön ja jonka peukalointitesti todistaa. [Poistosuoja](offsite-recovery.md#delete-protection-append-only)
- **Hyväksy ilman poistosuojaa** paikalle, joka ei voi kieltäytyä poistosta. [Poistosuoja](offsite-recovery.md#delete-protection-append-only)
- **Varmuuskopioiden poistaminen lopullisesti vaatii kirjautumissalasanan.** [Tietoturvamalli](configuration.md#security-model)
- **Palautuspaketti**, halutessasi age:lla sinetöitynä, ja **säilyttäjä** ryhmässäsi. [Palautuspaketti](offsite-recovery.md#recovery-kit), [Säilyttäjä](offsite-recovery.md#key-keeper)

## Useita palvelimia {#several-servers}

- **Pariliitos kahdellatoista sanalla**, päästä päähän salattu, suoraan tai releen kautta. [Ilmentymien pariliitos](offsite-recovery.md#pairing)
- **Ilmentymät-sivu**, jolla ryhmä näkyy verkkona, ja roolit **Vastaanotin**, **Noutaja**, **ZFS-palvelin** ja **Säilyttäjä**. [Ilmentymät ja roolit](offsite-recovery.md#roles)
- **Vastaanottava palvelin** valmiiksi asetettuna, append-only-tilassa. [Vastaanottava palvelin](offsite-recovery.md#receiving-server)

## Tekoälyavustajat (MCP) {#mcp}

Sisäänrakennettu MCP-palvelin antaa avustajan lukea varmuuskopioiden tilan, kattavuuden, ajohistorian ja palautuspisteet, ja avaimella, joka sen sallii, käynnistää varmuuskopion. Palautukset, poistot, karsinta ja asetukset pysyvät verkkokäyttöliittymässä. [MCP-palvelin](mcp.md)

## Sovellukset ja apuohjelmat {#apps}

- **Android-sovellus**: kaikki ryhmäsi palvelimet puhelimessasi. [Android-sovellus](android.md)
- **BombVault Widget**: ruutu Unraidin kojelaudalla. [Sovellukset](settings.md#apps)
- **Upotettava toimintoloki** kojelaudoille, kuten Homepage tai Heimdall. [Dashboard-widget](api.md#dashboard-widget)

## Muuta {#other}

- **42 kieltä**, kolme teemaa sekä nimikkeet, värit, kulmat ja animaatio valintasi mukaan. [Ulkoasu](settings.md#look)
- **Itsestään tallentuvat asetukset** ja haku kaikista neljästätoista ruudusta. [Asetukset](settings.md)
- **Uutta** päivityksen jälkeen, vihreillä pisteillä sen kohdalla, mikä muuttui. [Tietoja](settings.md#info)
- **HTTPS valmiina** ja terveystarkistus automaattisen parannuksen työkaluille. [Kokoonpano](configuration.md#network)
