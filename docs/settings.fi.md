# Asetukset

**Asetukset** koostuvat neljästätoista ruudusta. Jokainen ruutu on sivu täynnä kortteja, ja jokainen muutos kirjoitetaan heti, kun teet sen, ja säädin välähtää lyhyesti tai tärähtää, kun palvelin hylkää muutoksen. Muutamassa ikkunassa on **Tallenna**-painike, koska niiden tallentaminen vain puoliksi valmiina olisi vaarallista, kuten kirjautuminen tai kirjautumissalasana. Selitykset ovat (i)-kuplissa jokaisen säätimen vieressä.

Paina `/` tai Ctrl+K missä tahansa asetuksissa hakeaksesi kaikista, myös niillä nimillä, jotka sivuilla oli aiemmissa versioissa. Ruutujen järjestystä voi muuttaa kohdassa **Ulkoasu**. Kun sivun vieressä oleva ruutujen sarake puristaisi sivua, kuten noin 1200 pikselin levyisessä ikkunassa tai puhelimella, ruuduista tulee sivun yläpuolella oleva kaista, jota voi vierittää sivuttain.

![Asetukset, Yleiset-ruutu auki.](assets/screenshots/getting-started/settings-rail-dark.png#only-dark)
![Asetukset, Yleiset-ruutu auki.](assets/screenshots/getting-started/settings-rail-light.png#only-light)

Tällä sivulla listataan jokainen ruutu ja sen kortit sekä linkit sivulle, joka selittää kunkin tarkemmin.

## Yleiset {#general}

- **Osiot** kytkee jokaisen osion päälle tai pois: **Kontit**, **VMs**, **Flash**, **Kansiot**, **ZFS-tietojoukot** ja **Itsevarmuuskopio**. Pois päältä olevaa osiota ei varmuuskopioida millään aikataululla, ei myöskään merkinnän omalla aikataululla, eivätkä sen merkinnät lasketa mukaan. **Varmuuskopioi nyt** toimii silti. **Flash** näkyy vain Unraidissa.
- **Kontit** sisältää sen, mikä koskee jokaista konttia, kuten **Automaattiset tietokantavedokset**. Kaikki, mikä vaikuttaa yhteen konttiin, on sen omalla sivulla. Katso [Tietokantavedokset](backups.md#database-dumps).

## Ulkoasu {#look}

- **Ulkoasu**: **Kieli** (42 kieltä), **Teema** (**Tumma**, **Vaalea** tai **Järjestelmä**) ja sen alla **OLED**, joka antaa OLED- ja AMOLED-näytöille aidon mustan, kun tumma teema on näkyvissä, **Kulmat**, **Animaatio** (**Pois**, **Hillitty**, **Villi** tai **Myrskyisä**), **Hiljaiset ilmoitukset**, joka piilottaa onnistumisilmoitukset, kuten tallennusvahvistukset, ja **Asetussivujen järjestys**, jossa pidät ruutua pohjassa ja vedät sen paikalleen tai käytät Alt-näppäintä ja nuolinäppäimiä.
- **Nimikkeet**: kuinka paljon säätimestä näytetään, erikseen kohdille **Painikkeet**, **Sivupalkki**, **Välilehdet** ja **Alapalkki**: **Teksti**, **Teksti ja symboli**, **Symboli** tai **Reagoiva**, joka näyttää pelkän symbolin ja liu'uttaa sanat esiin, kun osoitat sitä. **Sivupalkki** vaikuttaa vain työpöytäasetteluun ja **Alapalkki** vain puhelinasetteluun.
- **Värit**: yksi **Korostusväri** tai **Sateenkaaritila**, jossa jokainen listan rivi saa oman värinsä kahdeksan värin paletista. Sateenkaaritila lisää **Disko-tilan**, jossa värit liukuvat hitaasti seuraavaan, **Reaktiivisen tilan**, jossa rivin väri näkyy vain, kun osoitat sitä, se on käynnissä tai valittuna, **Värikierron** ja itse **Väripaletin**. Korostusvärien alussa oleva taikasauva, **Järjestelmän väri**, ottaa käyttöön laitteesi oman värin: Material You Android-sovelluksessa Android 12:sta alkaen sekä järjestelmän värin Firefoxissa ja Safarissa. Se näkyy vain, jos laitteella on tällainen väri.

BombVault pitää ulkoasun palvelimella. Selain, joka tulee ensimmäistä kertaa, ottaa sen käyttöön. **OLED** ja **Järjestelmän väri** ovat poikkeus: jokainen laite pitää ne itsellään, koska ne riippuvat sen näytöstä. Kun järjestelmäsi pyytää vähemmän liikettä, se voittaa aina **Animaation**.

## Aikataulut {#schedule}

**Viikko**, **Täysvarmuuskopio** kohtineen **Milloin varmuuskopiot ajetaan**, **Aja väliin jääneet varmuuskopiot käynnistyksen jälkeen** ja **Komennot ennen ja jälkeen**, **Aikataulut osioittain** kohtineen **Etäkopiot osioittain**, **Aikataulutetut merkinnät**, **Järjestys** kohtineen **Uudelleenkäynnistys varmuuskopioinnin jälkeen**, **Tilannevedos pysäytyksen sijaan**, **Päivitykset varmuuskopioinnin jälkeen** ja **Joutilaisuus ennen varmuuskopiota**. Katso [Aikataulut](schedules.md).

## Kohteet {#storage-locations}

- **Näytä myös sivupalkissa** kiinnittää Kohteet-sivun sivupalkkiin.
- Lista kohteistasi kohdan **Lisää kohde** kanssa. Napsautus avaa kohteen sivun, katso [Kohteen sivu](offsite-recovery.md#location-page).
- **Tunnukset**: tallennetut tunnukset oletustähtineen, katso [Tunnukset](offsite-recovery.md#sign-ins).
- **Kohteet osioittain**: 3-2-1-kulku, jossa jokainen rivi on kytkin, katso [Osioiden oletukset](offsite-recovery.md#placement-defaults).
- **Arkistot**: yksittäisille merkinnöille tehdyt repot, katso [Sijoittelu merkinnän mukaan](offsite-recovery.md#placement).
- **Kuinka kauan säilytetään** osoittaa kohteisiin, joissa säilytys asetetaan, ja **Esikatselu** näyttää **Mitä seuraava ajo poistaisi**, repoa ja merkintää kohti, ennen kuin se tapahtuu. Se ei muuta eikä lukitse mitään, joten se vastaa myös varmuuskopion ollessa käynnissä.
- **Etäkopiot**: yksi rivi osiota kohti, kohdilla **Testaa yhteys**, **Replikoi nyt** ja **Määritä** tai **Muokkaa**, katso [Etäkopiot](offsite-recovery.md#off-site-copies).
- **Etäkaistanleveys**: **Lähetysraja (KiB/s)** ja **Latausraja (KiB/s)** kaikille kopioille. 0 tarkoittaa rajatonta.

**Suoratoisto**-kortti, joka laskee lähetysrajaa, kun Plex, Jellyfin tai Emby striimaa, on Kohteet-sivulla itsellään.

## Palautus {#restore}

- Linkki **Palautus**-sivulle.
- **Palautuskansiot**: **Palautuksen oletuskansio** kansioon palautusta varten, kohdilla **Alikansio osiota kohti** ja **Poikkeus** osiota kohti. Katso [Minne kansioon palautus menee](restore.md#folders).
- **Container-rekisterit**: tunnukset yksityisille rekistereille, kuten sponsorimuurin takana olevalle `ghcr.io`-imagelle. Palautus käyttää niitä puuttuvan imagen hakemiseen ja varmuuskopion jälkeinen päivitys uudemman tarkistamiseen. Julkiset imaget haetaan ilman. **Lisää rekisteri** tarjoaa julkisia rekistereitä, itse ylläpidettyjä, pilvipalveluntarjoajan rekistereitä ja **Muu rekisteri** mille tahansa osoitteelle.
- **Hätätilanteiden varalle**: **Lataa palautuspaketti**, joka vaatii kirjautumissalasanan. Katso [Palautuspaketti](offsite-recovery.md#recovery-kit).

## Ilmoitukset {#notifications}

**Ilmoitukset** kohtineen **Ilmoita**, yhteenvedot, **Ilmoita myöhässä olevista varmuuskopioista**, **Unraid-ilmoitukset** ja **Viikkokooste**, **Kanavat** kanavineen Webhook, Apprise, Matrix, Gotify ja sähköposti (SMTP), sekä **Healthchecks**. Katso [Ilmoitukset](notifications.md).

## Tarkista {#verify}

- **Tarkista varmuuskopiot**: viimeisen `restic check` -ajon tulos osiota kohti, ja jokaisella rivillä **Avaa lukitus** ja **Karsi** kohdassa **Lisää**. **Avaa lukitus** poistaa vanhentuneen lukon, ja **Karsi** soveltaa säilytyssääntöjä pyydettäessä.
- **Automaattiset tarkistukset**: ajastettu palautustarkistus kohtineen **Datanäyte (%)** ja testikohteet, **Käynnistystesti**, joka palauttaa myös yhden kontin eristettyyn kopioon ja käynnistää sen, **Ajastettu etä-DR-harjoitus** ja **Tarkista poistosuoja**, ajastettu peukalointitesti. Katso [DR-harjoitukset](offsite-recovery.md#dr-drills) ja [Peukalointitesti](offsite-recovery.md#tamper-test).
- **Poikkeamat**: tunnistus, herkkyys ja se, mitkä havainnot lähettävät viestin. Katso [Poikkeamat](anomalies.md).

## Turvallisuus {#security}

- **Salasana**: kirjautumissalasana, vähintään 12 merkkiä. Sen vaihtaminen tai tyhjentäminen kysyy nykyistä ja kaksivaiheisen tunnistautumisen ollessa päällä koodin.
- **Kaksivaiheinen tunnistautuminen**: tunnistussovelluksen koodi salasanan lisäksi, kahdeksan palautuskoodia näytetään kerran.
- **Pääsyavaimet**: kirjaudu avaimella puhelimessa, kannettavassa tai turvatikulla. Ne tarvitsevat oikean verkkotunnuksen ja varmenteen, johon selain luottaa.
- **Android-sovellus**: **Puhelimet voivat pariutua ryhmän kahdentoista sanan avulla**, jolloin pariliitetty puhelin voi kirjautua ilman salasanaa.
- **Salaus**: **Salaa varmuuskopiot**, oletuksena päällä.

Useat asiat vaativat kirjautumissalasanan, kuten palautuspaketti ja avainten tallennus. Katso [Tietoturvamalli](configuration.md#security-model).

## Ilmentymät {#instances}

- **Näytä myös sivupalkissa** kiinnittää Ilmentymät-sivun sivupalkkiin.
- **Useita palvelimia** kytkee päälle ryhmän osat: **Ilmentymät**, sivu, jolla on kortti jokaiselle jäsenelle, **Vastaanotin** ja **Nouto**. Katso [Ilmentymät ja roolit](offsite-recovery.md#roles).
- Linkki Ilmentymät-sivulle, mukana päällä olevat osat.

## Pariliitos {#pairing}

**Pariliitos** kohdilla **Luo lause** tai **Syötä lause**, vaihekortit, tämän ilmentymän nimi ja **Tunnetut osoitteet**, **Rele**-kortti neljine lomakkeineen ja **Mitä rele näkee**, ryhmän jäsenet sekä **ZFS-palvelin ilman BombVaultia** replikakohteelle SSH:n yli. Katso [Ilmentymien pariliitos](offsite-recovery.md#pairing) ja [Lähetys ZFS-palvelimelle](zfs-datasets.md#replica-push).

## Integraatiot {#integrations}

- **MCP-palvelin**: avaimet tekoälyavustajille, **Valitse asiakasohjelma** ja jokaisen avaimen loki. Katso [MCP-palvelin](mcp.md).
- **API-tokenit**: nimetyt tokenit skripteille ja kojelaudoille. Katso [Tokenit](api.md#tokens).
- **Palvelut**: **Home Assistant** MQTT:n yli, **Löydä verkosta** (mDNS nimellä `bombvault.local`) ja **Tarjoa /metrics** Prometheukselle, kohdalla **Scrape-tunniste (valinnainen)**. Katso [Home Assistant](api.md#home-assistant) ja [BombVaultin löytäminen verkosta](api.md#mdns).
- **Dashboard-widget**: vain luku -tunniste ja osoite mille tahansa kojelaudalle, joka näyttää iframen, kuten Homepage tai Heimdall, pelkällä live-toimintalokilla. Katso [Dashboard-widget](api.md#dashboard-widget).
- **Palvelimen SSH**: BombVaultin julkinen avain ja **Testaa yhteys** virtuaalikoneiden ja ZFS-tietojoukkojen varmuuskopioille sekä Unraid-ilmoituksille. Katso [VM-varmuuskopiointiopas](vm-backup-ssh-setup.md).

## Sovellukset {#apps}

- **Android-sovellus**: palvelimesi ajaman version APK QR-koodilla ja F-Droid-listaus. Katso [Android-sovellus](android.md#install).
- **ParleyPort**: relekontti Unraid-malleineen ja Docker-komento, joka sen käynnistää.
- **BombVault Widget**: ruutu Unraidin kojelaudalla BombVaultin toimintalokilla ja seuraavalla ajolla. Isännän SSH-yhteydellä kortti asentaa ja poistaa liitännäisen itse. Ilman sitä se antaa `.plg`-osoitteen asennettavaksi kohdassa **Plugins, Install Plugin**.

## Järjestelmä {#system}

- **Vie / tuo asetukset**: koko kokoonpanosi siirrettävänä JSON-tiedostona, tunnuksineen tai ilman. Tuonti näyttää ensin **Tämä tiedosto sisältää** ja jättää pois linkit kohteisiin, joita ei enää ole. **Salaa viennit age:lla** sinetöi selkeät viennit, flash-ZIPin ja palautuspaketin listaamillesi **age-vastaanottajille**. Katso [Siirrettävät asetukset](configuration.md#portable-settings-export-and-import).
- **Suorituskyky**: **CPU-säikeet (0 = kaikki)**, **Välimuistin kokoraja (Mt, 0 = rajaton)** resticin välimuistille polussa `/config` ja **Muistiraja varmuuskopiota kohden (MiB, 0 = ei rajaa)**, pehmeä raja, joka saa ison varmuuskopion hidastumaan sen sijaan, että se tapettaisiin muistin loppumisen vuoksi.

## Tietoja {#info}

- **Versio**: BombVault, GlimStone, restic ja rclone, kohdilla **Hae päivityksiä** ja **Uutta**, joka laskee muutokset, joita et ole vielä nähnyt, ja avaa ikkunan, joka listaa ne.
- **Ohje**: **Käyttöopas** avaa tämän dokumentaation, ja **Virheraportti** lataa sensuroidun ZIP-tiedoston GitHub-issuea varten. Se vaatii kirjautumissalasanan.
- **Tietoja BombVaultista**: projekti, sen lisenssi ja miten tavoitat tekijän.
