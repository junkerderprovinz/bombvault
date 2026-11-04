# VM-varmuuskopiot

BombVault varmuuskopioi KVM/libvirt-virtuaalikoneet liittämättä yhtään libvirt-polkua. Se ajaa `virsh`-komentoa isännällä SSH:n yli (`qemu+ssh://`), lukee ja kirjoittaa virtuaalikoneiden levyjä Host Data -liitoksen kautta (isännän `/mnt` on kontissa `/host/user`) ja kopioi UEFI NVRAMin SSH:n yli. Koska mitään libvirtiin kuuluvaa ei liitetä bind mountilla, BombVault ei voi koskaan olla Unraidin VM Managerin tiellä.

Tämä opas kattaa asennuksen, varmuuskopiointimenetelmät, Unraid-tilannevedoksilla varustetut virtuaalikoneet ja TrueNAS SCALEn. Konttivarmuuskopiot eivät tarvitse mitään tästä.

ZFS-tietojoukkojen varmuuskopiot käyttävät samaa SSH-yhteyttä, samaa avainta, isäntää ja käyttäjää. Kun VM-varmuuskopiot yhdistävät, myös ZFS-tietojoukkomerkinnät yhdistävät. Katso [ZFS-tietojoukot](zfs-datasets.md).

## Mitä määrität {#what-you-configure}

| Asetus | Missä | Oletus | Merkitys |
|---|---|---|---|
| **Host SSH: Address** (`LIBVIRT_HOST`) | mallin muuttuja | `host.docker.internal` | Unraid-isäntä, johon otetaan yhteys SSH:n yli |
| **Host SSH: Port** (`LIBVIRT_SSH_PORT`) | mallin muuttuja | `22` | Unraidin SSH-portti |
| **Host SSH: User** (`LIBVIRT_SSH_USER`) | mallin muuttuja | `root` | SSH-käyttäjä isännällä |
| Julkinen avain | **Asetukset, Integraatiot, Palvelimen SSH** | luodaan ensimmäisessä käynnistyksessä | Valtuuta se isännällä |

BombVault luo SSH-avainparinsa ensimmäisessä käynnistyksessä polkuun `/config/ssh/id_ed25519`, appdataan. Isännän avain kiinnitetään tiedostoon `/config/ssh/known_hosts` ensimmäisellä yhteydellä.

Malli täyttää **Host SSH: Address** -kenttään paikanvaraajan `192.168.x.x`. BombVault käsittelee sen asettamattomana ja kokeilee `host.docker.internal`-osoitetta, joka toimii oletusverkossa `bridge`. Mukautetussa verkossa korvaa se isännän lähiverkon IP-osoitteella (vaihe 3 B).

## Vaihe 1: Ota SSH käyttöön Unraid-isännällä {#step-1}

1. Avaa Unraidissa **Settings, Management Access** ja aseta **Use SSH** arvoon **Yes**.
2. Jos vaihdoit SSH-portin, merkitse se muistiin. **Host SSH: Port** pitää vastata sitä.

## Vaihe 2: Valtuuta BombVaultin julkinen avain {#step-2}

Unraidin `/root` sijaitsee muistissa ja tyhjenee jokaisessa käynnistyksessä, joten avaimen täytyy olla myös flash-asemalla. Aja tämä Unraidin päätteessä:

```sh
K=$(docker exec BombVault cat /config/ssh/id_ed25519.pub)
mkdir -p /root/.ssh /boot/config/ssh && chmod 700 /root/.ssh
grep -qxF "$K" /root/.ssh/authorized_keys 2>/dev/null  || echo "$K" >> /root/.ssh/authorized_keys
grep -qxF "$K" /boot/config/ssh/root.pubkeys 2>/dev/null || echo "$K" >> /boot/config/ssh/root.pubkeys
chmod 600 /root/.ssh/authorized_keys
```

Unraid 6.9 ja uudemmat kopioivat tiedoston `/boot/config/ssh/root.pubkeys` rootin `authorized_keys`-tiedostoon jokaisessa käynnistyksessä, joten tämä säilyy uudelleenkäynnistyksissä. Se pätee mihin tahansa `root`-kirjautumiseen, riippumatta isäntänimestä tai portista, jota BombVault käyttää.

## Vaihe 3: Verkko {#step-3}

BombVaultin täytyy avata TCP-yhteys kontista isännän SSH-porttiin. Valitse tapaus, joka vastaa asetustasi.

### A. BombVault oletussillatussa verkossa {#bridge}

- Jätä **Host SSH: Address** arvoon `host.docker.internal` tai mallin paikanvaraajaan. Malli lisää `--add-host=host.docker.internal:host-gateway`, joka osoittaa docker0-yhdyskäytävään eli isäntään.
- Aseta **Host SSH: Port** SSH-portiksesi.

### B. BombVault mukautetussa verkossa {#custom-network}

Kontti verkossa `br0.x`, macvlan tai ipvlan ei tavoita isäntää osoitteella `host.docker.internal`, koska 172.17.0.1 on docker0 eikä sitä tavoiteta verkosta `br0.x`. Tee sen sijaan näin:

1. Aseta Unraidissa **Settings, Docker, Host access to custom networks** arvoon **Enabled**.
2. Aseta **Host SSH: Address** Unraid-isännän lähiverkon IP-osoitteeksi, osoitteeksi, jolla avaat verkkokäyttöliittymän.
3. Jos kontin verkko ja isäntä ovat eri VLANeissa, salli reitti reitittimessäsi tai palomuurissasi: kontin VLANista isännän IP-osoitteeseen SSH-porttiin, TCP:n yli.
4. Jos isännän lähiverkon IP-osoitetta ei silti tavoiteta, käytä isännän shim-IP:tä kontin aliverkossa. Löydät sen isännältä komennolla `ip -4 addr show | grep -B2 <container-subnet>`.

### Tarkista, että isäntä vastaa {#verify-reachability}

Aja tämä Unraidin päätteessä omalla IP-osoitteellasi ja portillasi:

```sh
docker exec BombVault timeout 6 bash -c 'echo > /dev/tcp/192.168.1.10/22' && echo OPEN || echo UNREACHABLE
```

`OPEN` tarkoittaa, että reitti toimii. `UNREACHABLE` tarkoittaa, että vaiheen 3 verkko pitää korjata ennen mitään muuta.

## Vaihe 4: Aseta muuttujat ja testaa {#step-4}

1. Avaa Unraidissa **Docker**, napsauta BombVaultia, valitse **Edit**, aseta **Host SSH: Address** (ja **Host SSH: Port**, jos se ei ole 22) ja paina **Apply**. Jos muuttujat puuttuvat, tuo malli uudelleen, koska Unraid säilyttää olemassa olevan kontin tallennetun kokoonpanon.
2. Avaa BombVaultissa **Asetukset, Integraatiot, Palvelimen SSH** ja paina **Testaa yhteys**. Sen pitäisi muuttua vihreäksi.

Sama kutsu päätteestä listaa virtuaalikoneesi:

```sh
docker exec BombVault virsh -c "qemu+ssh://root@192.168.1.10:22/system?keyfile=/config/ssh/id_ed25519&known_hosts=/config/ssh/known_hosts&known_hosts_verify=auto" list --all
```

## Vaihe 5: Menetelmä virtuaalikonetta kohti {#per-vm-method}

Jokaisella virtuaalikoneella on **Menetelmä** sen Asetukset-välilehden **VM**-kortissa (avaa virtuaalikone kohdasta **Varmuuskopiot**):

- **Hallittu (sammutus)** on oletus. Se sammuttaa virtuaalikoneen, varmuuskopioi levyt ja käynnistää sen uudelleen, joten varmuuskopio on aina yhdenmukainen. Se odottaa jopa kymmenen minuuttia vieraan sammumista eikä koskaan pakota sitä pois. Vieras, joka silti pyörii sen jälkeen, jätetään rauhaan, ja kyseinen varmuuskopio epäonnistuu syyn kanssa.
- **Live-tilannekuva** varmuuskopioi käynnissä olevan virtuaalikoneen ulkoisen tilannevedoksen ja `blockcommit`-komennon kautta. Se tarvitsee qemu guest agentin virtuaalikoneessa hiljennettyä tilannevedosta varten ja levyn polussa `/mnt/cache` tai `/mnt/diskX`, ei `/mnt/user`. Sammutetulla virtuaalikoneella Live palaa itsestään Hallittuun, ja samoin tapahtuu, kun tilannevedosta ei voida tehdä.

## Vain muuttuneet lohkot {#changed-blocks}

qcow2-levyillä varustetun virtuaalikoneen voi varmuuskopioida lukemalla vain sen, mikä muuttui. Kytke päälle **Vain muuttuneet lohkot** samalla kortilla, ja BombVault käyttää libvirtin varmuuskopio-API:a yllä olevan menetelmän sijaan:

- Se käynnistää pull-tilan varmuuskopiotyön (`virsh backup-begin`), joka luo samalla hetkellä tarkistuspisteen nimeltä `bombvault-<time>`, ja lukee levyt SSH-yhteyden yli välitetyn Unix-soketin kautta. Kun vierasagentti vastaa, vieraan tiedostojärjestelmät jäädytetään vain työn käynnistymisen ajaksi. Epäonnistunutta sulatusta yritetään uudelleen noin viisitoista sekuntia, ja jos vieras ei silti sula, BombVault lähettää ilmoituksen.
- Ensimmäinen ajo lukee jokaisen varatun lohkon. Myöhemmät ajot lukevat vain lohkot, jotka tarkistuspisteen bittikartta merkitsee kirjoitetuiksi edellisen varmuuskopion jälkeen.
- Jokainen levy tallennetaan kansiona kiinteänkokoisia segmenttejä, jotka sisältävät sen, minkä vieras näkee. Muuttumattomat segmentit otetaan edellisestä tilannevedoksesta lukematta niitä, joten jokainen tilannevedos on täydellinen ja palautuu yksinään.
- Palautus rakentaa levyn segmenteistään ja muuntaa sen takaisin qcow2-muotoon komennolla `qemu-img`. Levyllä on sama sisältö kuin varmuuskopiointihetkellä, mutta tiedosto on uusi qcow2-tiedosto, ja taustaketju palautuu litistettynä.
- Tällaista palautusta edeltävä tarkistus listaa jokaisen levyn, jonka se rakentaa uudelleen, ja laskee sen vieraan täydessä koossa, qcow2:lle kahdesti, koska raakakuva ja sen muunnettu kopio ovat vierekkäin, kunnes vanha tiedosto korvataan. Se on enimmäismäärä, jonka levy voi viedä, joten vajaus tällä laskennalla jättää tilarivin harmaaksi sen sijaan, että estäisi. Ensimmäisen varmuuskopion jälkeinen palautustarkistus ja **Tarkista palautus** varmistavat myös, että tilannevedos sisältää jokaisen levyn jokaisen segmentin oikeassa koossa.
- Ajojen välillä virtuaalikoneella ei ole tarkistuspistettä libvirtin kirjanpidossa. BombVault säilyttää uusimman `bombvault-`-tarkistuspisteen määrityksen polussa `/config/vm-checkpoints` ja poistaa vain libvirtin merkinnän siitä (`virsh checkpoint-delete --metadata`). Sen bittikartta pysyy qcow2-tiedostossa ja jatkaa kirjoitusten laskemista, ja seuraava ajo laittaa tarkistuspisteen takaisin (`virsh checkpoint-create --redefine`) ennen alkua. libvirt kieltäytyy poistamasta määritystä virtuaalikoneelta, jolla on tarkistuspisteitä, joten tämä mahdollistaa sen, että Unraidin **Remove VM** ja palautus toimivat tavalliseen tapaan.
- Varmuuskopiotyö, jonka BombVault jätti käyntiin, koska se pysähtyi kesken varmuuskopioinnin, päätetään, kun BombVault käynnistyy uudelleen, yhdessä niiden tarkistuspisteiden kanssa, jotka kyseinen ajo jätti kirjanpitoon.
- Kytkimen kytkeminen pois, virtuaalikoneen poistaminen aikataulusta tai sen varmuuskopioiden poistaminen päättää myös tällaisen työn ja poistaa bittikartankin virtuaalikoneen ollessa käynnissä. Sammutetulla virtuaalikoneella libvirt ei voi poistaa sitä, ja se jää käyttämättömänä kuvaan.
- Muiden työkalujen tarkistuspisteet jätetään rauhaan. Virtuaalikonetta, jolla sellaisia on, ei voi palauttaa, ennen kuin ne on poistettu, ja palautus pysähtyy ennen kuin se koskee virtuaalikoneeseen.

Kytkin palautuu itsestään. Puuttuva tai rikkinäinen tarkistuspiste, levy, jonka koko muuttui, tai uusin tilannevedos, joka ei ole muuttuneiden lohkojen tilannevedos, tarkoittaa, että koko levy luetaan vielä kerran. Sammutettu virtuaalikone, raaka- tai zvol-levy, puuttuva isännän SSH-yhteys, libvirt ilman varmuuskopio-API:a tai liian vähän vapaata tilaa polun `/config` alla tarkoittaa tavallista varmuuskopiota yllä olevalla menetelmällä. Kortti kertoo, mitä näistä viimeinen ajo käytti ja miksi.

Vaatimukset: libvirt varmuuskopio-API:n kanssa (testattu libvirt 12.2:lla ja QEMU 10.2:lla Unraid 7.3:ssa), sshd asetuksella `AllowStreamLocalForwarding yes` (oletus, myös Unraidissa, joka kytkee TCP-välityksen pois) ja qcow2-levyt. libvirt kieltäytyy tarkistuspisteistä raakalevyillä, ja Unraid luo raaka-vdiskit oletuksena. Virtuaalikone pysyy käynnissä varmuuskopioinnin ajan kuten Livessä, ja resticiä odottavat segmentit vievät jopa 4 GiB polun `/config` alla.

## Unraid-tilannevedoksilla varustetut virtuaalikoneet {#unraid-snapshots}

Tilannevedos, jonka Unraidin VM Manager ottaa, muuttaa qcow2-levyn ketjuksi: alkuperäinen kuva pohjalla ja sen päällä yksi tiedosto tilannevedosta kohti, kukin sisältäen vain sen, mikä muuttui sen alla olevan jälkeen. Virtuaalikone kirjoittaa uusimpaan tiedostoon. Levy, jolla on kaksi tilannevedosta, on kolme tiedostoa.

BombVault varmuuskopioi tällaisen virtuaalikoneen kokonaan:

- **Ketjun jokainen tiedosto.** Se seuraa jokaisen levyn taustaketjua sen loppuun ja varmuuskopioi jokaisen sen tiedoston, yhdessä Unraidin tilannevedoslistan kanssa (`/etc/libvirt/qemu/snapshotdb/<vm>/snapshots.db`, luettuna SSH:n yli). Ketju, jota ei voi seurata loppuun, esimerkiksi koska yksi tiedosto keskeltä puuttuu, kaataa varmuuskopion ja nimeää tiedoston.
- **Live-varmuuskopio säilyttää tilannevedokset.** **Live-tilannekuva** laittaa väliaikaisen peittokuvan päälle levyä lukiessaan ja yhdistää sen takaisin jälkeenpäin, vain heti sen alla olevaan kuvaan. Ketju on varmuuskopioinnin jälkeen sama kuin ennen sitä. Virtuaalikone ilman tilannevedoksia käyttäytyy kuten aina.
- **Palautus tuo ketjun takaisin.** Paikalleen tehtävä palautus kirjoittaa jokaisen tiedoston takaisin sinne, missä se oli, ja laittaa Unraidin tilannevedoslistan takaisin, joten VM Manager listaa tilannevedokset taas. Alkuperäisen viereen tehtävä palautus kirjoittaa tiedostot kopion kansioon ja osoittaa palautetut levyt palautettuihin tiedostoihin, joten kopio ei koskaan lue alkuperäisen kuvista. Unraidin tilannevedoslista palaa vain paikalleen tehtävässä palautuksessa, koska se nimeää tiedostot sellaisina kuin ne olivat varmuuskopioitaessa. Ilman sitä kopio käynnistyy samoin, ja VM Manager vain ei listaa sille tilannevedoksia.
- **Vain muuttuneet lohkot** lukee sen, minkä vieras näkee, joten sen palautus palaa yhtenä litistettynä qcow2-tiedostona ilman tilannevedoksia.

### Esimerkki: virtuaalikone kahdella tilannevedoksella {#example-unraid-snapshots}

Virtuaalikoneella `homeassistant` on yksi levy, `vdisk1.qcow2`, ja kaksi tilannevedosta, jotka otit VM Managerissa ennen kahta päivitystä. Sen levy on nyt kolmen tiedoston ketju. Se on koko ajan käynnissä, joten se käyttää menetelmää **Live-tilannekuva**.

1. Avaa **Varmuuskopiot**, sitten `homeassistant` ja sen **Asetukset**-välilehti. Tarkista **VM**-kortissa, että **Menetelmä** on **Live-tilannekuva** ja **Vain muuttuneet lohkot** on pois päältä.
2. Paina **Varmuuskopioi nyt** sivun päässä.
3. Kun ajo on valmis, sen rivi kohdassa **Viimeisimmät ajot** **Tarkista**-välilehdellä on vihreä. Virtuaalikone pysyi käynnissä koko ajan, ja varmuuskopio sisältää levyn kaikki kolme tiedostoa ja Unraidin tilannevedoslistan.
4. VM Managerissa kaksi tilannevedosta ovat yhä siellä, ja levy lukee edelleen saman kolmen tiedoston läpi.

Kuukautta myöhemmin päivitys menee pieleen ja haluat eilisen tilan takaisin rikkinäisen viereen vertailua varten:

1. Avaa **Palautus**, valitse **VMs** ja napsauta `homeassistant`.
2. Pidä viime yön suositeltu varmuuskopio ja valitse kohdassa **Minne** **Alkuperäisen viereen**.
3. Paina **Palauta**. Kolme tiedostoa päätyvät kansioon, jonka nimi on virtuaalikoneen nimi lisäyksellä `-restored`, ja palautettu levy osoittaa palautettuihin tiedostoihin. Käynnissä olevaan virtuaalikoneeseen ei muutu mitään.

Palauttaaksesi koko virtuaalikoneen aiempaan tilaan valitse sen sijaan **Korvaa alkuperäinen** niin, että **Varmuuskopioi nykytila ensin** on päällä. BombVault varmuuskopioi rikkinäisen tilan, sammuttaa virtuaalikoneen, kirjoittaa kolme tiedostoa ja Unraidin tilannevedoslistan takaisin ja käynnistää sen uudelleen.

## TrueNAS SCALE {#truenas-scale}

Yllä olevat vaiheet on kirjoitettu Unraidille, mutta sama `qemu+ssh://`-yhteys toimii mitä tahansa libvirtd:tä vastaan, jonka kontti tavoittaa, TrueNAS SCALE mukaan lukien, jossa virtuaalikoneet ovat taas libvirtissä versiosta 25.04.2 alkaen. TrueNASissa yhteysmerkkijono tarvitsee ohituksen, ja avain menee rootille TrueNASin omissa käyttäjäasetuksissa. Kolme muuta asiaa muuttaa sitä, mitä varmuuskopio sisältää: zvol-levyt, vTPM-tila ja nimet, jotka TrueNAS antaa virtuaalikoneilleen.

### 1. Libvirt-soketti {#truenas-socket}

TrueNASin libvirtd ei kuuntele siellä, missä **Host SSH** -kentistä rakennettu yhteysmerkkijono sitä odottaa. Aseta sen sijaan `LIBVIRT_URI` lisäparametrilla `?socket=`:

```sh
LIBVIRT_URI=qemu+ssh://<user>@<truenas-host>/system?socket=/run/truenas_libvirt/libvirt-sock
```

BombVault käyttää `LIBVIRT_URI`-arvoa täsmälleen sellaisenaan, eikä `LIBVIRT_HOST`, `LIBVIRT_SSH_USER` ja `LIBVIRT_SSH_PORT` ole tarpeen. Jokaiselle niistä, joka on asettamatta, BombVault ottaa isännän, käyttäjän ja portin URI:sta myös omille SSH-komennoilleen, joten NVRAM-kopio ja ZFS-tietojoukkojen varmuuskopiot tavoittavat saman koneen kuin `virsh`. Itse asettamasi muuttuja voittaa aina, ja käynnistysloki nimeää kohteen, jonka se otti URI:sta. TrueNASissa muu käyttäjä kuin root tarvitsee ZFS-tietojoukkojen varmuuskopioita varten myös `zfs allow <user> snapshot,destroy,mount <dataset>`, katso [ZFS-tietojoukot](zfs-datasets.md#truenas).

Laita TrueNASin lähiverkon osoite URI:in. Ilman `LIBVIRT_URI`- ja `LIBVIRT_HOST`-arvoja BombVault ottaa sovelluksen oletusyhdyskäytävän isännäksi, koska `host.docker.internal` ei ratkea TrueNAS-sovelluksessa, ja käynnistysloki nimeää valitsemansa osoitteen. Se riittää ZFS-tietojoukkojen varmuuskopioille. VM-varmuuskopiot tarvitsevat silti URI:n soketin vuoksi.

Sovellus tarvitsee `SETPCAP`-oikeuden VM-varmuuskopioita varten, oikeuksien `CHOWN`, `DAC_OVERRIDE`, `FOWNER`, `SETGID` ja `SETUID` lisäksi. Ilman sitä libvirt pysähtyy viestiin "cannot apply process capabilities". Kohdan [TrueNAS SCALE](getting-started.md#truenas) YAML asettaa ne kaikki, yhdessä `SYS_CHROOT`-oikeuden kanssa, jota tilannevedos pysäytyksen sijaan tarvitsee.

TrueNAS SCALE 25.10.0:ssa soketin polku tarkistettiin oikealla laitteistolla: oletuksena oleva `qemu:///system` epäonnistuu viestiin `Failed to connect socket to '/var/run/libvirt/libvirt-sock': No such file or directory`, ja sama kutsu parametrilla `?socket=/run/truenas_libvirt/libvirt-sock` listaa domainit.

### 2. BombVaultin avain TrueNASissa {#truenas-key}

Yllä oleva vaihe 2 kirjoittaa Unraidin flash-asemalle eikä päde tässä. TrueNASissa:

1. Kytke SSH-palvelu päälle kohdassa **System, Services**.
2. Muokkaa kohdassa **Credentials, Users** käyttäjää **root** ja liitä BombVaultin julkinen avain kohdasta **Asetukset, Integraatiot, Palvelimen SSH** sen SSH-julkiseksi avaimeksi. TrueNAS säilyttää sen uudelleenkäynnistysten ja päivitysten yli.

TrueNAS 24.10:n ja uudempien tuoreet asennukset eivät anna rootille salasanaa, eikä avainkirjautuminen tarvitse sellaista. Jos et halua käyttää rootia, valtuuta avain admin-tilille sudolla ja laita sen nimi `root`-kohdan tilalle `LIBVIRT_URI`-arvossa. ZFS-tietojoukkojen varmuuskopiot tarvitsevat silloin `zfs allow` -komennon kuten yllä on kuvattu.

ZFS-replika toiselle palvelimelle kirjautuu omalla avaimellaan, jonka replikaikkuna näyttää. Host SSH -avain ei valtuuta replikaa, eikä replika-avain valtuuta VM-varmuuskopioita.

!!! warning "TrueNAS ei näe virsh-komennolla tehtyjä muutoksia"
    TrueNASin middleware, sen VM-näkymien takana oleva kerros, on tarkoitettu ainoaksi tavaksi muuttaa sen virtuaalikoneita. `virsh`-komento tämän SSH-yhteyden yli, mitä BombVaultin VM-varmuuskopiointi ja -palautus ajavat, kiertää sen, ja TrueNAS voi myöhemmin sovittaa tai ylikirjoittaa tilan, jota se ei luonut. Tämä perustuu TrueNASin dokumentoituun käyttäytymiseen, eikä sitä ole nähty oikealla laatikolla. Se on samanlainen riski kuin se, että Unraidin VM Manager sovittaa sen ulkopuolella tehtyjä muutoksia, vain terävämpi TrueNASissa.

### 3. Levyt zvoleilla {#truenas-zvols}

TrueNAS-virtuaalikoneiden levyt ovat usein zvoleja, ZFS-lohkolaitteita polussa `/dev/zvol/<pool>/<dataset>`, eivät qcow2-tiedostoja. restic ei voi varmuuskopioida lohkolaitetta sen polun kautta samaan tapaan kuin tiedostoa, joten BombVault tunnistaa zvol-levyn domain-XML:stä (`<source dev="...">` kohdan `<source file="...">` sijaan) ja käsittelee sen saman SSH-yhteyden yli. BombVault-kontti ei tarvitse ZFS-työkaluja.

Zvolin varmuuskopio:

1. `zfs snapshot <dataset>@bombvault-vm-<time>` kiinnittää ajanhetken, myös virtuaalikoneen ollessa käynnissä.
2. `zfs clone -o volmode=dev -o readonly=on` tekee kyseisestä tilannevedoksesta luettavan laitteena. `volmode=dev` estää isäntää skannaamasta vieraan osioita.
3. `dd` lukee laitteen, ja raakakuva menee resticiin yhtenä tiedostona, `/vm-disks/<dataset>.img`, tunnisteella `vm:<name>:zvol:<dev>`. restic pilkkoo sen paloihin sisällön mukaan, joten lohkoja, jotka eivät muuttuneet viime varmuuskopion jälkeen, ei tallenneta uudelleen.
4. Klooni ja tilannevedos tuhotaan jälkeenpäin, onnistuiko varmuuskopio tai ei. Jos BombVault pysähtyi kesken varmuuskopioinnin, kyseisen zvolin seuraava varmuuskopio poistaa ensin `bombvault-vm-<time>`-tilannevedoksen ja sen `<dataset>-bombvault-vm-<time>`-kloonin. Se jättää kaikki muut tilannevedokset rauhaan ja säilyttää sellaisen, josta jotain muuta kloonattiin. Loki nimeää, mitä se poisti tai säilytti.

BombVault ei käytä tähän `zfs send` -komentoa. Jokainen send-virran tietue kantaa tilannevedoksen GUID:ta ja juoksevaa tarkistussummaa, joten restic ei löytäisi mitään viime varmuuskopiosta uudelleenkäytettäväksi ja tallentaisi koko levyn uudelleen joka kerta. Mitattuna TrueNAS SCALE 25.10:ssä 512 MiB:n zvolilla, jossa oli 200 MiB dataa: ensimmäinen varmuuskopio lisäsi 211 MB, toinen ilman muutoksia 52 KB ja yksi sen jälkeen, kun 1 MiB oli muuttunut, 3,6 MB. Sama zvol `zfs send` -komennon kautta lisäsi 214 MB joka ajolla.

Jokainen ajo kirjoittaa myös pienen tilannevedoksen, jolla on tunniste `vm:<name>` ja domain-XML, kun kaikki levyt ovat mukana. Virtuaalikoneella, jonka kaikki levyt ovat zvoleja, ei ole tiedostotilannevedosta, ja tämä on se, joka listaa ajon virtuaalikoneen alle ja tekee siitä palautettavan.

Palautus kirjoittaa jokaisen zvolin takaisin paikalleen, kuten tiedostolevy kirjoitetaan takaisin tiedostoonsa, joten palautettu virtuaalikone käynnistyy palautetulla datalla:

1. Ennen kuin mitään kosketaan, BombVault tarkistaa, että ajo sisältää kuvan jokaisesta zvol-levystä. Ajo, josta sellainen puuttuu, hylätään ja virtuaalikone jätetään ennalleen.
2. Olemassa olevasta zvolista otetaan ensin tilannevedos, `<dataset>@bombvault-before-restore-<time>`, aika UTC:nä kuten jokaisessa BombVaultin tekemässä tilannevedosnimessä, joten se voi olla tunteja paikallisesta kellosta poikkeava. Loki nimeää sen, ja `zfs rollback` siihen kumoaa palautuksen, kun se on zvolin uusin tilannevedos. Kun uudempi on olemassa, kuten seuraavan palautuksen, `zfs rollback -r` tekee sen ja tuhoaa uudemmat tilannevedokset. Se pysyy, kunnes poistat sen komennolla `zfs destroy`, kun palautettu virtuaalikone toimii.
3. Zvol, jota ei enää ole, luodaan sillä koolla, joka sillä oli varmuuskopioitaessa, ja olemassa oleva eri kokoinen asetetaan takaisin siihen kokoon.
4. `dd` kirjoittaa kuvan polkuun `/dev/zvol/<dataset>`. Pakkauksen ollessa päällä, mikä on TrueNASin oletus, kuvan tyhjät osat palaavat aukkoina eivätkä vie tilaa.

**Palautus toiseen ilmentymään tarvitsee kohdepoolin.** Se siirtää tiedostolevyt kohdekansioon, mutta kansio ei nimeä ZFS-poolia, joten ilman sitä BombVault hylkää palautuksen ennen alkua. Anna pooli parametrina `zvolPool` kutsussa `POST /api/foreign/restore`: jokainen zvol säilyttää polkunsa poolin alla (`tank/vms/win10/disk0` poolille `flashpool` tulee `flashpool/vms/win10/disk0`), ja domain-XML osoittaa siihen. Palautus-sivun **Vanhasta varmuuskopiokansiosta** -ikkunassa ei ole tälle kenttää, joten se on tavoitettavissa vain suoralla API-kutsulla. Palautus samalle ilmentymälle kirjoittaa jokaisen zvolin takaisin itseensä.

**Vienti tiedostoina jättää zvolit pois.** Virtuaalikone, jonka kaikki levyt ovat zvoleja, hylätään tällä syyllä, joten palauta se varmuuskopiosta. Virtuaalikoneelle, jolla on myös tiedostolevyjä, vienti sisältää tiedostolevyt ja huomautuksen, joka nimeää pois jätetyt zvolit.

Tätä kokeiltiin TrueNAS SCALE 25.10:ssä ZFS 2.3.4:llä: kolme varmuuskopiota testizvolista, paikalleen tehtävä palautus sen jälkeen, kun zvol oli ylikirjoitettu ja kasvatettu, ja palautus zvolille, joka oli poistettu. Jokainen palautus luki takaisin samalla SHA-256:lla kuin lähde, zvolilla oli vanha kokonsa taas, eikä tilannevedosta tai kloonia jäänyt jäljelle. Kokonainen virtuaalikone, jonka ainoa levy on zvol, varmuuskopioitiin käynnissä, Hallitulla ja Livellä, ja sammutettuna, ja koko virtuaalikoneen palautus (määrityksen poisto, zvolien kirjoitus, määrittely, käynnistys) ajettiin siellä myös: zvol luettiin takaisin lähteen tarkistussummalla ja virtuaalikone käynnistyi.

Tiedostolevyt, kuten Unraid niitä käyttää, eivät kärsi tästä mitenkään.

### 4. vTPM-tila {#truenas-vtpm}

Virtuaalikoneella voi olla virtuaalinen TPM, jota Secure Boot ja Windows 11 -vieraat tarvitsevat. Sen tila on levyn ja NVRAMin ulkopuolella, joten pelkästään niiden varmuuskopio toisi vieraan takaisin uudella, tyhjällä TPM:llä.

- **Läpivientinä oleva TPM**, oikea siru annettuna vieraalle, nimeää polkunsa domain-XML:ssä. BombVault tallentaa sen tilan NVRAMin viereen SSH:n yli ja kirjoittaa sen takaisin ennen `virsh define` -komentoa, parhaan yrityksen periaatteella kuten NVRAM: epäonnistunut luku tai kirjoitus kirjataan lokiin eikä kaada varmuuskopiota tai palautusta.
- **Emuloitu vTPM**, sellainen jonka TrueNAS asettaa Windows 11- ja Secure Boot -vieraille, ei julkaise tilapolkua domain-XML:ssä. BombVault raportoi silloin "no TPM path found" sen sijaan, että arvaisi polun. Pidä tällaisen vieraan palautusavain, kuten sen BitLocker-avain, saatavilla ennen kuin palautat sen.
- **Et voi liittää TPM:ää TrueNAS-virtuaalikoneeseen itse.** TrueNAS SCALE 25.10:n VM-laite-API hyväksyy vain `CDROM`, `DISPLAY`, `NIC`, `PCI`, `RAW`, `DISK` ja `USB` ja hylkää `TPM`:n tuntemattomana tyyppinä.

TPM-käsittely on vain yksikkötestattu, eikä sitä ole ajettu oikeaa TrueNAS-vierasta vastaan vTPM:llä. Virtuaalikone ilman TPM:ää, kuten Unraidissa, varmuuskopioituu ja palautuu täsmälleen kuten ennenkin.

NVRAM tallennetaan tavalliseen tapaan. TrueNAS pitää sen virtuaalikonetta kohti polussa `/var/db/system/vm/nvram/<id>_<name>_VARS.fd`, ja domain-XML nimeää kyseisen polun.

### 5. Virtuaalikoneiden nimet TrueNASissa {#truenas-names}

Unraidissa virtuaalikoneen libvirt-nimi on nimi, jonka valitsit. TrueNAS keksii libvirt-nimen, ja on muuttanut tapaa kahdesti:

| TrueNAS-versio | libvirt-nimi | Missä nimesi on |
|---|---|---|
| 25.10 "Goldeye" | `{id}_{name}`, kuten `1_debian` | libvirt-nimessä, `{id}_`-osan perässä |
| 26 | virtuaalikoneen UUID, kuten `550e8400-e29b-41d4-a716-446655440000` | vain domain-XML:n `<title>`-kentässä |

Isännällä, jonka BombVault tunnistaa TrueNASiksi tai jonka `PLATFORM=truenas` nimeää, Varmuuskopiot-lista näyttää nimesi: `debian` merkinnän `1_debian` sijaan versiossa 25.10 ja `<title>` UUID:n sijaan versiossa 26, tai UUID, kun otsikkoa ei ole. Tämä on vain se, mitä lista näyttää. Jokainen `virsh`-kutsu ja virtuaalikonekohtaiset asetukset käyttävät yhä libvirt-nimeä, joten varmuuskopiointi ja palautus toimivat aina oikealle virtuaalikoneelle. TrueNAS 26:ssa se tarkoittaa, että virtuaalikone, jonka TrueNAS luo uudelleen uudella UUID:lla, ilmestyy uutena merkintänä ilman asetuksiaan, vaikka sen nimi näyttää samalta. Unraidissa mikään ei muutu, ei myöskään virtuaalikoneelle, jonka nimesit `10_Windows`.

Version 25.10 kaava nähtiin oikealla TrueNAS SCALE 25.10 -laatikolla. TrueNAS 26:n kaava tulee TrueNASin dokumentaatiosta ja on vain yksikkötestattu.

## Vianmääritys {#troubleshooting}

| Oire | Syy ja korjaus |
|---|---|
| **Testaa yhteys** jumittuu ja epäonnistuu sitten | Isäntää ei tavoiteta. Tarkista vaihe 3 (verkko, VLAN, palomuuri) sekä **Host SSH: Address** ja **Port**. |
| `Permission denied (publickey)` vaikka avain on tiedostossa `authorized_keys` | sshd:n `StrictModes` hylkää avaintiedoston, koska ylähakemistolla on väärä omistaja tai tila. Se on yleistä, kun `/root/.ssh` on linkki FAT-flash-asemalle (`/boot/config/ssh/...`), joka on kaikkien kirjoitettavissa. Isännän loki näyttää `Authentication refused: bad ownership or modes for directory ...`. Lisää `StrictModes no` isännän sshd-asetuksiin globaaliin osaan, ennen mitään `Match`-lohkoa, ja aja sitten `/etc/rc.d/rc.sshd restart`. Unraid kirjoittaa `/etc/ssh/sshd_config`-tiedoston uudelleen jokaisessa käynnistyksessä, joten tee se pysyväksi alla kuvatulla tavalla. |
| `Permission denied (publickey)` ja avain todella puuttuu | Toista vaihe 2 ja tarkista, että SSH on päällä. |
| `Host key verification failed` | Aja `docker exec BombVault rm -f /config/ssh/known_hosts` ja testaa uudelleen. |
| Vaiheen 3 tarkistus sanoo `UNREACHABLE` | Kontti ei tavoita isännän SSH-porttia. Katso vaihe 3: isäntäpääsy mukautetuille verkoille, VLAN-reititys tai shim- tai lähiverkon IP. |
| Muuttujat puuttuvat kohdasta **Edit** | Tuo malli uudelleen. Olemassa oleva kontti säilyttää vanhan tallennetun kokoonpanonsa. |
| Unraid-tilannevedoksilla varustettu virtuaalikone epäonnistuu ja nimeää tiedoston | Levyn taustaketjua ei voi seurata loppuun. Tarkista VM Managerissa, että jokainen kyseisen levyn tilannevedos on yhä olemassa ja että nimetty tiedosto on olemassa polun `/mnt` alla. |

Lisää VM-oireita on kohdassa [Vianmääritys](troubleshooting.md#vm-backup-will-not-connect).

## Säilyminen uudelleenkäynnistysten yli {#persistence}

Unraid kirjoittaa `/etc/ssh/sshd_config`-tiedoston uudelleen jokaisessa käynnistyksessä, joten isäntäpuolen muutokset on tehtävä uudelleen käynnistyksessä:

- **SSH-portti** säilyy tiedostossa `/boot/config/ident.cfg`, asetettuna Unraidin verkkokäyttöliittymän kautta.
- **`StrictModes no`**, jos tarvitsit sitä yllä, ei säily. Ota se käyttöön käynnistyksessä tiedostosta `/boot/config/go`:

  ```sh
  # /boot/config/go: keep SSH key auth working with flash-based authorized_keys
  ( for i in $(seq 1 30); do [ -f /etc/ssh/sshd_config ] && break; sleep 2; done
    grep -q '^StrictModes no' /etc/ssh/sshd_config || \
      sed -i '0,/^Match /s/^Match /StrictModes no\n&/' /etc/ssh/sshd_config
    # SIGHUP reloads sshd's config in place: it keeps the listener + open sessions
    # and avoids the "killing listener process / Restarting SSH server daemon"
    # warning that `rc.sshd restart` prints at boot. Only reload if sshd is already
    # up; if it starts after this runs, it reads the edited config on its own.
    pidof sshd >/dev/null && killall -HUP sshd ) &
  ```

- BombVaultin oma SSH-avain ja `~/.ssh/config` sijaitsevat polussa `/config` (appdata) ja tehdään uudelleen jokaisessa käynnistyksessä, joten ne säilyvät kontin ja isännän uudelleenkäynnistyksissä.

## Tietoturva {#security}

Valtuutettu avain antaa rootin isännällä, saman luottamustason kuin Docker-soketti, jota BombVault jo käyttää. Pidä BombVault luotetussa verkossa. Avain on ed25519, tallennettu tilalla `0600` appdataan, ja isännän avain on kiinnitetty. Jokainen isäntäkomento välitetään erillisinä argumentteina, ei koskaan etäkuoren kautta, ja SSH-yhteys käyttää `BatchMode`-tilaa ja yhteyden aikakatkaisua.
