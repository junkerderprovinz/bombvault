# Asetukset

Tämä sivu käsittelee kontin ympäristömuuttujat, mallin tarjoamat liitokset, VM-varmuuskopioinnin SSH:n yli ja etäsijainnin määrityksen. Varmuuskopioinnin **repopolut** määritetään sovelluksen sisällä (Asetukset, Varmuuskopiopolut), ei ympäristömuuttujilla.

## Ympäristömuuttujat

| Muuttuja | Vaadittu | Kuvaus |
|---|---|---|
| `APP_KEY` | **Kyllä** | 32-tavuinen heksadesimaalisalaisuus (64 heksamerkkiä), jota käytetään restic-repon salasanan johtamiseen. Luo komennolla `openssl rand -hex 32`. Pidä tämä turvassa: sen menettäminen tekee salatuista varmuuskopioista palautuskelvottomia. |
| `LIBVIRT_HOST` | Virtuaalikoneille | Unraid-isäntä, johon otetaan yhteys SSH:n yli VM-varmuuskopiointia varten (oletus `host.docker.internal`; malli esitäyttää LAN-IP-paikanvaraajan). Käytä Unraidin LAN-IP-osoitetta, vaadittu mukautetussa `br0.x`-verkossa. |
| `LIBVIRT_SSH_PORT` | Ei | Isännän SSH-portti VM-varmuuskopiointiin (oletus `22`). |
| `LIBVIRT_SSH_USER` | Ei | SSH-käyttäjä isännällä VM-varmuuskopiointiin (oletus `root`). |
| `LIBVIRT_URI` | Ei | Täydellinen libvirt-yhteys-URI, jota käytetään **sellaisenaan** sen sijaan, että se rakennettaisiin yllä olevista kolmesta `LIBVIRT_*`-muuttujasta (jotka jätetään silloin huomiotta yhteysmerkkijonon osalta). Oletuksena asettamaton. Tarvitaan TrueNAS Scalessa, jonka libvirtd kuuntelee epästandardissa soketissa, jota rakennettu merkkijonomuoto ei pysty ilmaisemaan: `qemu+ssh://<user>@<truenas-host>/system?socket=/run/truenas_libvirt/libvirt-sock`. Katso TrueNAS Scale -osio tiedostosta [docs/vm-backup-ssh-setup.md](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md). |
| `PORT` | Ei | HTTP-portti (oletus `3000`; käytetään vain asetuksella `HTTP_ONLY=true`). |
| `HTTPS_PORT` | Ei | HTTPS-portti (oletus `3443`; malli julkaisee sen 1:1, joten WebUI vastaa osoitteessa `https://<ip>:3443`). |
| `HTTP_ONLY` | Ei | Aseta `true` poistaaksesi itse allekirjoitetun HTTPS-kuuntelijan käytöstä ja tarjotaksesi vain selkeää HTTP:tä (käytettäväksi TLS:n päättävän käänteisen välityspalvelimen takana). |
| `HOST_SOURCE_ROOT` | Ei | Isäntäpolku, joka liitetään **Host Datana** (oletus `/mnt`). BombVault kääntää Dockerin raportoimat bind-liitosten lähteet poluiksi tämän liitoksen alle. Muuta vain, jos liitit eri isäntäjuuren. |
| `DATA_ROOT_SEGMENTS` | Ei | Pilkuin eroteltu lista polkusegmenttien nimistä, jotka merkitsevät bind-liitoksen lähteen varmuuskopiodataksi (oletus `appdata`, Unraidin `/mnt/user/appdata/<container>`-käytännön mukaisesti). Kontin bind-liitos valitaan automaattisesti varmuuskopioitavaksi, kun MIKÄ TAHANSA listatuista segmenteistä esiintyy täytenä polkusegmenttinä sen isäntälähteessä, esimerkiksi `DATA_ROOT_SEGMENTS=appdata,config` poimii myös `.../config`-liitoksen. Katso [Varmuuskopiolähteen tunnistus](#backup-source-detection) muista, aina käytössä olevista tavoista, joilla kontin datakansio löydetään. |
| `PLATFORM` | Ei | Pakottaa alustan, jolla BombVault katsoo itsensä toimivan, sen sijaan että se tunnistaisi sen automaattisesti: `unraid`, `generic` tai `truenas` (oletuksena asettamaton: tunnistaa Unraidin automaattisesti koettelemalla sen `dockerMan`-merkkiä flash-liitoksen alta, muuten `generic`; tunnistamaton arvo palautuu myös arvoon `generic`, mistä kirjataan loki). Aseta se eksplisiittisesti yleisellä Docker-isännällä tai TrueNAS Scalessa sen sijaan, että luotat pelkkään Unraid-tunnistukseen. Yleinen compose-tiedosto tekee näin. Muuttaa appdata-varakäytäntöä, instanssien välisen palautuskohteen oletuksia sekä sitä, yritetäänkö Unraid-kohtaisia ilmoitus-/kumppanilisäosavaiheita ylipäätään (katso `internal/platform`). |
| `BOMBVAULT_SELF_CONTAINER` | Ei | Itse BombVault-kontin nimi, jotta se ei koskaan varmuuskopioi (ja siten pysäytä) itseään (oletus `BombVault`; tunnistetaan automaattisesti isäntänimen kautta siltaverkossa). |
| `BACKUP_MAX_HOURS` | Ei | Maksimimäärä kellonaikatunteja, jonka yksittäinen varmuuskopiointiajo saa pitää toimialuelukkoaan ennen kuin se pakotetaan peruutettavaksi (suoja, jotta jumittunut ajo ei voi tukkia toimialuetta ikuisesti). Tyhjä (oletus) käyttää arvoa `48`. Nosta sitä hyvin suuria tai hitaita pilvivarmuuskopioita varten (ajo, joka peruutetaan katossa, epäonnistuu virheellä `context deadline exceeded`). Aseta `0` poistaaksesi katon kokonaan käytöstä. |
| `TZ` | Ei | Ajastimen aikavyöhyke (esimerkiksi `Europe/Berlin`). |

## Liitokset

Liitä Docker-soketti, flash (`/boot`) ja **Host Data** -juuri (`/mnt`) kuten CA-mallissa on näytetty. Varmuuskopioinnin *lähteet* ja *kohteet* asuvat molemmat Host Datan alla, ja se liitetään **rslave**-tilassa, joten etäjako, joka liittyy kontin käynnistymisen jälkeen (esimerkiksi kohtaan `/mnt/remotes`), tulee näkyviin ilman uudelleenkäynnistystä.

Varmuuskopioinnin repopolut ovat oletuksena `/mnt/user/bombvault/{container,vms,flash,config,files}`, luotuina ensimmäisen varmuuskopion yhteydessä. Vaihda sijaintia milloin tahansa kohdassa **Asetukset, Varmuuskopiopolut**.

!!! note "Isäntäintegraation tarkistus"
    Avaa `/spike` verkkokäyttöliittymässä kontin käynnistyttyä. Se koettaa jokaista liitosta ja komentorivityökalua (Docker-soketti, libvirt, restic, qemu-img, rclone) ja raportoi puuttuvat palaset.

## Turvallisuusmalli

!!! warning "Root-tasoinen isännän hallinta"
    Docker-soketin kautta BombVault voi pysäyttää, poistaa ja luoda uudelleen kontteja sekä lukea ja kirjoittaa appdataa, ja VM-varmuuskopiointia varten se kirjautuu isäntään SSH:n yli (`qemu+ssh://`, oletuksena root) ajaakseen `virsh`-komennon. Kuka tahansa, joka pääsee sen verkkokäyttöliittymään, hallitsee käytännössä isäntää root-oikeuksin.

- **Valinnainen salasanasuoja** (Asetukset, Turvallisuus): aseta salasana vaatiaksesi kirjautumista, tyhjennä se poistaaksesi käytöstä. Oletuksena pois päältä luotetun LAN:n käyttöä varten. Istunnot ovat allekirjoitettuja (`APP_KEY`:stä johdettu HMAC) ja salasanan vaihtaminen mitätöi ne; kirjautumisia rajoitetaan nopeudeltaan.
- Koska portti on käyttöön otettava, kun se on asettamatta, koko käyttöliittymä ja rajapinta (mukaan lukien etäsijainnin määritys, peukalointitestireitit ja palautuspaketti) ovat kenen tahansa tavoitettavissa, joka pääsee porttiin. Ota portti käyttöön heti kun käytät etäsijaintia, muuttumattomia varmuuskopioita tai salausta.
- Aja BombVaultia vain luotetussa, altistamattomassa verkossa. Etäkäyttöä varten sijoita se käänteisen välityspalvelimen taakse, joka lisää todennuksen ja TLS:n. Vastaukset kantavat perustason turvaotsikot (CSP, `nosniff`, `X-Frame-Options`, `Referrer-Policy`).
- Asetuksella `HTTP_ONLY=true` istuntoeväste menettää `Secure`-lippunsa (sen on pakko, jotta se toimisi selkeän HTTP:n yli), joten ota salasana käyttöön TLS:n päättävän välityspalvelimen takana vain jos luottamuksellisuudella on väliä.
- VM-varmuuskopioinnin SSH-yhteys luottaa isäntäavaimeen ensimmäisellä yhteydellä (TOFU) ja kiinnittää sen sen jälkeen. Vahvista isännän avain erillistä kanavaa pitkin, jos kontti-isäntä-reittisi ei ole luotettu.
- Varmuuskopiot ovat resticin salaamia, kun salaus on käytössä (Asetukset; oletuksena päällä), avaimen ollessa johdettuna `APP_KEY`:stä.

## VM-varmuuskopiointi SSH:n yli

BombVault varmuuskopioi KVM/libvirt-virtuaalikoneet **liittämättä yhtäkään libvirt-polkua**. Se ajaa `virsh`-komennon isännällä SSH:n yli (`qemu+ssh://`), joten se ei voi koskaan vaikuttaa isäntäsi VM Manageriin.

Pikamääritys:

1. **Asetukset, Järjestelmä, VM Backup over SSH:** kopioi näytetty julkinen avain.
2. Lisää se Unraidin tiedostoon `/root/.ssh/authorized_keys` (myös flashiin tallennettuna, jotta se säilyy uudelleenkäynnistysten yli).
3. Napsauta **Test connection**.

Malli lisää `--add-host=host.docker.internal:host-gateway`, jotta kontti tavoittaa isännän. Aseta `LIBVIRT_HOST` Unraidin LAN-IP-osoitteeseesi, jos tuo nimi ei ratkea (esimerkiksi kun kontti pyörii mukautetussa `br0.x`-verkossa). Jos vaihdoit Unraidin SSH-porttia, aseta `LIBVIRT_SSH_PORT` vastaamaan. **Live-tilannevedokset** tarvitsevat lisäksi qemu guest agentin VM:ssä ja levyn sijainniksi `/mnt/cache` (ei `/mnt/user`).

!!! important "Täydellinen VM-määritys- ja verkko-opas"
    Täydellinen vaihe vaiheelta -opas (SSH:n käyttöönotto, pysyvä avaimen valtuutus, mukautetun verkon ja VLAN:n reititys, VM-kohtainen menetelmä ja isäntäpuolen vianmääritys) sijaitsee osoitteessa [docs/vm-backup-ssh-setup.md](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) GitHubissa.

## Etäsijainnin määritys

Määritä etäreplika **Asetukset, Etä** -välilehdellä. Katso [Etäsijainti ja palautus](offsite-recovery.md) koko työnkulkua varten (muuttumaton/append-only, peukalointitestaus ja DR-harjoitukset). Lyhyesti:

- **Taustajärjestelmät:** SMB/CIFS ja NFS (liitä jako ja osoita varmuuskopiopolku siihen), natiivit restic-taustajärjestelmät ilman rclonea (`s3:...`, `rest:http://host:8000/repo`, `b2:...`, `sftp:user@host:/repo`) tai mikä tahansa rclone-etäsijainti (`rclone:<remote>:<bucket>/path`).
- **Pilvitunnukset** tallennetaan salattuina kohdassa Asetukset, Etä, Pilvitunnukset.
- **SSH-kohteet eivät vaadi mitään asennettavaksi vastapuolelle.** `sftp:` tarvitsee vain SSH-palvelimen. Lisää julkinen avain kohdasta **Asetukset, Järjestelmä, VM Backup over SSH** (myös tiedostossa `/config/ssh/id_ed25519.pub`) kohdekäyttäjän tiedostoon `~/.ssh/authorized_keys`.
- **Etäkopio:** BombVault replikoi uudet tilannevedokset `restic copy` -komennolla parhaan yrityksen periaatteella. Paikallinen repo pysyy ensisijaisena. Jokaisella toimialueella on oma etäaikataulunsa sekä **Replikoi nyt** -painike.
- **Useita etäkohteita per toimialue:** jokainen toimialue voi replikoitua useaan etäkohteeseen kerralla. Lisää lisäkohteita kohtaan Asetukset, Etä, kukin omalla repositoriollaan, S3-tallennusluokallaan, append-only-lipullaan, säilytyksellään ja kasvubudjetillaan; ne kaikki replikoituvat kyseisen toimialueen etäaikataulun mukaan. Olemassa oleva yksittäinen etämääritys siirretään ensimmäiseksi kohteeksi.
- **Säilytys lähdekohtaisesti:** paikallinen käytäntö asuu kohdassa Asetukset, Polut ja tallennus; etäkäytäntö kohdassa Asetukset, Etä (jätä se pelkiksi nolliksi, jotta etätilannevedoksia ei koskaan karsita automaattisesti).
- **Kaistanleveyden rajat:** rajoita resticin lähetys-/latausnopeutta kohdassa Asetukset, Etä.
- **Kylmä- ja arkistotallennusluokka (S3):** natiiville S3-etärepolle valitse palautuksesta luettava taso (Standard, Standard-IA, One Zone-IA, Intelligent-Tiering, Glacier Instant Retrieval). rclone-etäsijainnit asettavat luokkansa rclone-määrityksessä.

## Siirrettävät asetukset (vienti ja tuonti) {#portable-settings-export-and-import}

**Vie ja tuo asetukset** -kortti Asetukset-sivulla kirjoittaa koko BombVault-kokoonpanosi (toimialueasetukset, etäkohteet, aikataulut, säilytys, ilmoitukset) siirrettävään JSON-tiedostoon, jonka voit tuoda toiseen instanssiin, joten uuteen laatikkoon siirtyminen tai kokoonpanon kloonaus ei tarkoita kaiken syöttämistä uudelleen käsin. Tuonti näyttää esikatselun ja pyytää vahvistusta, eikä se koskaan kosketa varmuuskopiodataasi tai historiaasi.

!!! warning "Vienti voi sisältää tunnuksia"
    Valitset itse, sisällytetäänkö etä- ja ilmoitustunnukset tiedostoon. Tunnusten kanssa vienti on yhtä arkaluontoinen kuin palautuspakettisi, joten säilytä se turvallisessa paikassa. Ilman niitä tiedosto sisältää vain salaamattomat asetukset.
