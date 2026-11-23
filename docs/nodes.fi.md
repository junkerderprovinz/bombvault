# Solmut

Solmu on BombVault ilman verkkokäyttöliittymää, esimerkiksi VPS:llä tai pienellä palvelimella ystävän luona. Se varmuuskopioi oman palvelimensa kontit ja kansiot, ja BombVault, johon lisäät sen, tällä sivulla **kotiinstanssi**, kertoo sille, milloin varmuuskopioidaan ja minne varmuuskopiot menevät. Hoidat solmua kotiinstanssin verkkokäyttöliittymästä omien merkintöjesi rinnalla.

![Solmun vps-falkenstein kortti Ilmentymät-sivulla: Yhdistetty, sen versio, sen varmuuskopiot vastaanottopalvelimella, milloin kukin osio varmuuskopioidaan sekä Vaihda määränpää, Korvaa ja Vapauta.](assets/screenshots/nodes/card-dark.png#only-dark)
![Solmun vps-falkenstein kortti Ilmentymät-sivulla: Yhdistetty, sen versio, sen varmuuskopiot vastaanottopalvelimella, milloin kukin osio varmuuskopioidaan sekä Vaihda määränpää, Korvaa ja Vapauta.](assets/screenshots/nodes/card-light.png#only-light)

## Näin se toimii {#how-it-works}

- **Kotiinstanssi pitää solmun avainta.** Kun lisäät solmun, kotiinstanssi luo sen `APP_KEY`-avaimen, pitää siitä sinetöidyn kopion ja antaa sen sinulle kerran, solmun compose-tiedostossa. Tämän kopion avulla kotiinstanssi tavoittaa solmun ja avaa sen varmuuskopiot.
- **Solmu ei tarvitse avointa porttia.** Kotiinstanssi ja solmu keskustelevat releen kautta omaa yksityistä yhteyttään pitkin, erillään pariliitosryhmästäsi. Kotiinstanssi aloittaa jokaisen kutsun ja allekirjoittaa sen, eikä solmu pyydä kotiinstanssilta mitään muuta kuin tervehdyksensä. Ainoa määränpää, joka tarvitsee avoimen portin, on [varmuuskopioiden säilyttäminen solmulla](#keep-on-node).
- **Varmuuskopiot eivät koskaan kulje releen kautta.** Solmu lähettää ne suoraan valitsemaasi paikkaan: kotiinstanssin vastaanottopalvelimelle, johonkin kotiinstanssin kohteista tai solmun omaan rest-serveriin.
- **Solmu ei suunnittele itse mitään.** Se ei aja aikatauluja, ei lähetä ilmoituksia eikä liity ryhmään tai linnoitukseen. Kotiinstanssi käynnistää sen varmuuskopiot, seuraa niiden kulkua ja lähettää hälytykset.
- **Solmu varmuuskopioi kontteja ja kansiojoukkoja.** Virtuaalikoneet, flash-asema, ZFS-tietojoukot ja itsevarmuuskopio jäävät täysille asennuksille.

Yhteys käyttää kotiinstanssin omaa relettä, kun sen ryhmällä on sellainen, ja muuten projektin relettä, myös silloin kun kotiinstanssi pariutuu ilman relettä.

## Ennen kuin aloitat {#before-you-start}

- **Kirjautumissalasana kotiinstanssissa.** Solmun lisääminen, sen compose-tiedoston näyttäminen uudelleen ja sen merkintöjen avaaminen vaativat kirjautumisen salasanalla, katso [Kirjautuminen](configuration.md#login).
- **Docker ja Compose solmun palvelimella.**
- **Paikka varmuuskopioille**, yksi kolmesta kohdassa [Minne varmuuskopiot menevät](#placement). Vastaanottopalvelin on turvallisin valinta, koska solmu ei voi poistaa sieltä mitään.

## Solmun lisääminen {#add}

1. Avaa kotiinstanssissa **Ilmentymät** ja valitse oikeasta alakulmasta **Lisää solmu**.
2. **Nimi.** Kirjoita, millä nimellä solmu näkyy täällä, esimerkiksi palvelimen nimi, ja valitse **Seuraava**.

    ![Lisää solmu -ikkuna ensimmäisessä vaiheessa, nimeksi kirjoitettuna vps-helsinki.](assets/screenshots/nodes/add-name-dark.png#only-dark)
    ![Lisää solmu -ikkuna ensimmäisessä vaiheessa, nimeksi kirjoitettuna vps-helsinki.](assets/screenshots/nodes/add-name-light.png#only-light)

3. **Varmuuskopiot.** Valitse, minne varmuuskopiot menevät, ja täytä, mitä kyseinen paikka kysyy. **Varmuuskopioi päivittäin klo** on aika, jolloin molemmat osiot alkavat. Kotiinstanssi ehdottaa puolta tuntia omien konttivarmuuskopioidensa jälkeen, jotta ne eivät käytä samoja levyjä yhtä aikaa. Voit myöhemmin antaa kortilla kullekin osiolle oman aikataulun. Valitse **Seuraava**.

    ![Toinen vaihe: vastaanottopalvelin valittuna, Osoite solmuille täytettynä https-osoitteella ja Varmuuskopioi päivittäin klo 03:00.](assets/screenshots/nodes/add-placement-dark.png#only-dark)
    ![Toinen vaihe: vastaanottopalvelin valittuna, Osoite solmuille täytettynä https-osoitteella ja Varmuuskopioi päivittäin klo 03:00.](assets/screenshots/nodes/add-placement-light.png#only-light)

4. **Käynnistys.** Ikkuna näyttää solmun compose-tiedoston, jossa `MODE`, `APP_KEY`, `NODE_HOME` ja `NODE_NAME` on valmiiksi täytetty. Kopioi se ja pane `APP_KEY` talteen turvalliseen paikkaan. Solmun palvelimella:

    ```sh
    mkdir -p /opt/bombvault-node && cd /opt/bombvault-node
    nano docker-compose.yml   # liitä tiedosto ikkunasta
    docker compose up -d
    ```

    Ennen kuin käynnistät sen, osoita Host Data -taltio dataan, jonka haluat varmuuskopioida, ja aseta `HOST_SOURCE_ROOT` samaan polkuun, kuten alla oleva esimerkki näyttää.

5. Ikkuna odottaa solmua. Kun solmu vastaa, kotiinstanssi kertoo sille, minne sen varmuuskopiot menevät, ja tarkistaa, että se tavoittaa ne. Ikkunassa lukee silloin **vps-helsinki on verkossa ja ottaa ohjeensa tältä instanssilta.**

    ![Viimeinen vaihe: compose-tiedosto ja sen jälkeen solmu verkossa ja sen varmuuskopiot sijoitettuina vastaanottopalvelimelle.](assets/screenshots/nodes/add-start-dark.png#only-dark)
    ![Viimeinen vaihe: compose-tiedosto ja sen jälkeen solmu verkossa ja sen varmuuskopiot sijoitettuina vastaanottopalvelimelle.](assets/screenshots/nodes/add-start-light.png#only-light)

Voit sulkea ikkunan ennen kuin solmu vastaa. Sen kortissa lukee silloin **Odotetaan solmua**, ja kotiinstanssi kertoo sille varmuuskopioiden määränpään, kun se ilmestyy.

### Esimerkki: VPS, joka varmuuskopioi vastaanottopalvelimelle {#example-vps}

Kotiinstanssi on Unraid-kone nimeltä `tower`, joka ajaa [vastaanottopalvelinta](offsite-recovery.md#receiving-server). Käänteinen välityspalvelin julkaisee sen osoitteessa `https://backup.tower.example.org`. VPS `vps-helsinki` ajaa Nextcloudia ja Caddya, joiden data on hakemistossa `/srv/appdata`.

1. Koneella `tower`: **Ilmentymät**, **Lisää solmu**, nimeksi `vps-helsinki`, **Seuraava**.
2. Valitse **Tämän instanssin vastaanottopalvelin** ja kirjoita `https://backup.tower.example.org` kohtaan **Osoite solmuille**. Kotiinstanssin kontit varmuuskopioidaan kello 02.30, joten ikkuna tarjoaa aikaa 03:00. Pidä se ja valitse **Seuraava**.
3. Tallenna VPS:llä ikkunan tiedosto niin, että taltiot ja `HOST_SOURCE_ROOT` osoittavat hakemistoon `/srv/appdata`:

    ```yaml
    services:
      bombvault-node:
        image: junkerderprovinz/bombvault:latest
        restart: unless-stopped
        hostname: bombvault-node
        container_name: bombvault-node
        volumes:
          - ./config:/config
          - /var/run/docker.sock:/var/run/docker.sock
          - /srv/appdata:/host/user
        environment:
          MODE: node
          APP_KEY: "<filled in by home>"
          NODE_HOME: "<filled in by home>"
          NODE_NAME: "vps-helsinki"
          PLATFORM: generic
          HOST_SOURCE_ROOT: /srv/appdata
    ```

4. `docker compose up -d`. Muutamaa sekuntia myöhemmin koneen `tower` ikkuna kertoo, että solmu on verkossa, ja **Varmuuskopioiden määränpää on https://backup.tower.example.org.**
5. Kello 03.00 kotiinstanssi käynnistää solmun varmuuskopiot. Seuraavana aamuna yleiskatsauksen **Solmut**-kortti näyttää solmun `vps-helsinki` viimeisen varmuuskopion, ja **Varmuuskopiot** listaa sen kontit omassa osiossaan.

Miten kontin data löytyy tavalliselta Docker-isännältä, kerrotaan kohdassa [Varmuuskopiolähteen tunnistus](configuration.md#backup-source-detection).

## Minne varmuuskopiot menevät {#placement}

Jokaisella solmulla on yksi määränpää molemmille osioilleen. Kortin **Vaihda määränpää** muuttaa sen, kun solmu on verkossa. Uusi paikka saa uudet repositoryt, ja vanhan paikan varmuuskopiot jäävät sinne, missä ne ovat.

### Tämän instanssin vastaanottopalvelin {#to-receiver}

Solmu kirjoittaa kotiinstanssin vastaanottopalvelimelle append-only-tilassa: se voi lisätä varmuuskopioita mutta ei poistaa niitä. Se saa sinne oman tunnuksen solmun nimellä ja yhden repositoryn osiota kohti omassa kansiossaan. Kotiinstanssi lisää oman avaimensa jokaiseen repositoryyn ja siivoaa ne itse, koska solmu ei voi.

Jos kotiinstanssilla ei vielä ole vastaanottopalvelinta, vaihe kertoo sen ja tarjoaa **Ota vastaanottopalvelin käyttöön**. Kotiinstanssi ei tarvitse siihen ryhmää. Valitse kansio jaolta ja portti, valitse **Ota käyttöön ja jatka** ja sitten **Valmis**, kun ikkuna näyttää palvelimen, niin vaihe jatkaa sen kanssa. Yksityiskohdat ovat kohdassa [Vastaanottopalvelin](offsite-recovery.md#receiving-server).

Solmu tavoittaa palvelimen osoitteessa **Osoite solmuille**, jonka asetat toisessa vaiheessa tai **Vastaanottopalvelin**-ikkunassa. Solmu lähettää yleensä internetistä, joten osoitteen on oltava jokin näistä:

- `https://`, esimerkiksi käänteisen välityspalvelimen, Pangolinin tai tunnelin takana. Kun palvelin näyttää oman varmenteensa julkisen varmentajan varmenteen sijaan, liitä se PEM-muodossa kohtaan **Varmenne**.
- `http://` yksityisellä IP-osoitteella, kuten `10.0.0.5` WireGuard-yhteydellä tai `100.101.102.103` Tailscalessa.

Pelkkä `http://` julkiseen osoitteeseen hylätään, koska solmu lähettäisi varmuuskopionsa internetin yli salaamattomina. Kun muutat osoitetta, kotiinstanssi kertoo sen jokaiselle vastaanottimen solmulle, ja poissa oleva solmu kuulee sen palattuaan.

### Kohde {#to-location}

Solmu kirjoittaa johonkin kotiinstanssin kohteista, jonka se itse tavoittaa: S3-palveluun tai REST-palvelimeen, `https://`-osoitteella tai yksityisessä osoitteessa. Sen repositoryt ovat siellä polulla `bombvault-node-<node id>-containers` ja `bombvault-node-<node id>-files`.

Solmu kirjautuu sitä varten tehdyillä [tallennetuilla tunnuksilla](offsite-recovery.md#sign-ins), joiden on oltava eri kuin ne, joilla kotiinstanssi kirjoittaa kohteeseen. Lisää ensin tunnukset, jos ikkuna kertoo, ettei niitä ole. Näillä tunnuksilla solmu voi yleensä poistaa kohteessa olevaa, toisin kuin vastaanottopalvelimella, ja ikkuna kertoo sen.

### Säilytä ne solmulla {#keep-on-node}

Solmu säilyttää varmuuskopionsa itse oman rest-serverinsä takana append-only-tilassa, ja kotiinstanssi hakee kustakin osiosta kopion omaan tallennustilaansa. Tämä sopii solmulle, jolle kotiinstanssi ei voi tarjota paikkaa. Se on ainoa määränpää, joka tarvitsee avoimen portin.

![Toinen vaihe, jossa Säilytä ne solmulla on valittuna, osoite https://203.0.113.7:8443 ja huomautus avoimesta portista.](assets/screenshots/nodes/add-keep-dark.png#only-dark)
![Toinen vaihe, jossa Säilytä ne solmulla on valittuna, osoite https://203.0.113.7:8443 ja huomautus avoimesta portista.](assets/screenshots/nodes/add-keep-light.png#only-light)

1. Kirjoita kohtaan **Solmun osoite**, mistä kotiinstanssi tavoittaa solmun: sen julkinen osoite ja vapaa portti, kuten `https://203.0.113.7:8443`, ilman kansiota perässä. Käytä `http://`-osoitetta vain yksityisellä osoitteella, kuten yllä.
2. Solmu käynnistää kontin `bombvault-node-rest-server` tähän porttiin, ja sen repositoryt ovat Host Datan kansiossa `bombvault-node`. `https://`-osoitteella se tekee varmenteen tälle osoitteelle, ja kotiinstanssi kiinnittää sen.
3. Päästä portti solmun palomuurin läpi, jotta kotiinstanssi tavoittaa sen. Jos portti on jo jonkin toisen solmun kontin käytössä, se hylätään ja kontin nimi kerrotaan.

Kotiinstanssi hakee osion heti, kun solmun sen osion varmuuskopiot päättyvät, ja varalta joka tunti vartin yli, myös kun **Nouto** on pois päältä. Mitä kotiinstanssi on hakenut, jää kotiinstanssiin kuten mikä tahansa [noudettu kopio](offsite-recovery.md#receiver), myös solmun poistuttua. Solmu siivoaa omat repositorynsa, kun kotiinstanssi pyytää, koska sen rest-server ei ota vastaan poistoja, ja sen kansiojoukot jättävät kansion `bombvault-node` pois, joten se ei koskaan varmuuskopioi omia varmuuskopioitaan.

### Etäkopiot kotiinstanssin kopioiden mukana {#copy-onward}

Kytke **Kopioi muualle tämän instanssin varmuuskopioiden mukana** päälle vaiheessa **Varmuuskopiot** tai kohdassa **Vaihda määränpää**, niin kotiinstanssi kopioi solmun kontit ja kansiojoukot niihin kohteisiin, joihin sen omat Kontit ja Kansiot kopioivat, samalla aikataululla. Solmu ei tarvitse tähän omaa kansiota eikä omaa kopioaikataulua. Kohdassa **Vaihda määränpää** kytkin tulee voimaan heti.

- Kopiot ovat samoissa repositoryissa kuin kotiinstanssin kopiot, solmun isännän alla. Kotiinstanssin säännöt kohteessa jättävät ne rauhaan, ja kotiinstanssi säilyttää ne solmun säännön mukaan, saman, jonka mukaan se [siivoaa solmun varmuuskopiot](#schedule).
- Kotiinstanssi lukee solmun repositoryt omalla avaimellaan ja kirjoittaa kohteeseen omalla tunnuksellaan. Solmu ei saa uusia tunnuksia, ja vastaanottopalvelin pysyy append-only-tilassa. Solmulle, joka säilyttää varmuuskopionsa itse, kotiinstanssi kopioi sen, mitä se on hakenut.
- Solmun repositoryilla on omat chunker-asetuksensa, joten niiden kopiot jakavat kohteessa vain vähän dataa kotiinstanssin kopioiden kanssa.
- Append-only-kohteessa mitään ei siivota, kuten kotiinstanssin omissa kopioissakaan. [Jaettu arkisto](offsite-recovery.md#shared-repository) ei saa solmun kopioita, koska sen omistaja siivoaa sen ryhmän palvelinten sääntöjen mukaan.
- Kortilla näkyy **Kopioiden kohde** ja kohteet. Epäonnistunut kopio näkyy kortilla virheenä ja lähettää hälytyksen, katso [Solmujen seuraaminen](#watch).
- Pois päältä kotiinstanssi ei kopioi enää mitään. Jo olemassa olevat kopiot jäävät, ja kotiinstanssin säännöt jättävät ne edelleen rauhaan.

#### Esimerkki: VPS kopioituna B2:een kotiinstanssin konttien kanssa {#example-copy-onward}

`tower` säilyttää kohdassa **Kuinka kauan säilytetään** 7 varmuuskopiota kustakin kontista ja kopioi Konttinsa B2-bucketiin joka yö kello 04.00. Bucket säilyttää viimeiset 14. `vps-helsinki` varmuuskopioi `tower`:n vastaanottopalvelimelle, jossa sen varmuuskopiot siivotaan samojen 7:n mukaan. Kun kytkin on päällä, `tower` kopioi kello 04.00 myös VPS:n kontit bucketiin. Bucketissa on silloin 14 `tower`:n varmuuskopiota kustakin kontista ja 7 kustakin VPS:n kontista.

## Milloin solmut varmuuskopioivat {#schedule}

Kotiinstanssi omistaa jokaisen solmun aikataulun. Kortti näyttää aikataulun osiota kohti, esimerkiksi **Kontit** *päivittäin kello 2.00* ja **Kansiot** *viikoittain (su) kello 4.00*. Napsautus avaa saman muokkaimen kuin kohdassa [Aikataulut](schedules.md#plans), ja muutos tallentuu heti.

![Solmun konttien aikataulu avattuna sen kortilta: Pois, Päivittäin, Viikoittain, Joka N. päivä ja Cron.](assets/screenshots/nodes/schedule-dark.png#only-dark)
![Solmun konttien aikataulu avattuna sen kortilta: Pois, Päivittäin, Viikoittain, Joka N. päivä ja Cron.](assets/screenshots/nodes/schedule-light.png#only-light)

- **Poissa ollut solmu** käynnistää väliin jääneet varmuuskopiot heti, kun se palaa. Kotiinstanssi katsoo tilanteen viiden minuutin välein.
- **Kiireiseltä solmulta** kysytään uudelleen viiden minuutin kuluttua.
- **Kun kotiinstanssi siivoaa** solmun vanhoja varmuuskopioita, solmu ei käynnistä mitään ennen kuin kotiinstanssi on valmis. Vastaanottopalvelimella ja itse solmulla tämä ikkuna aukeaa joka sunnuntai kello 03.00 ja kestää enintään kuusi tuntia. Kohteessa se seuraa kotiinstanssin etäkopioaikataulua kyseiselle kohteelle. Kotiinstanssi säilyttää varmuuskopiot osion oman sääntönsä mukaan kohdassa **Kuinka kauan säilytetään** tai kohteen säännön mukaan, jos kohteella on sellainen.
- **Solmun ajolistassa** kotiinstanssin käynnistämät varmuuskopiot ja palautukset näkyvät kotiinstanssin käynnistäminä.

## Solmujen seuraaminen {#watch}

Yleiskatsauksen **Solmut**-kortti listaa jokaisen solmun tilan, viimeisen varmuuskopion, virheet ja avoimet poikkeamat. Kortti ilmestyy, kun sinulla on solmu.

![Yleiskatsauksen Solmut-kortti, jossa on kolme solmua: yksi yhdistetty, yksi ei aktiivinen kahdella virheellä ja yksi, joka odottaa yhä solmua.](assets/screenshots/nodes/overview-card-dark.png#only-dark)
![Yleiskatsauksen Solmut-kortti, jossa on kolme solmua: yksi yhdistetty, yksi ei aktiivinen kahdella virheellä ja yksi, joka odottaa yhä solmua.](assets/screenshots/nodes/overview-card-light.png#only-light)

Kotiinstanssi lähettää yhden hälytyksen jaksoa kohti [ilmoituskanaviensa](notifications.md) kautta ja seuraavan vasta, kun jakso on ohi:

- **Varmuuskopio epäonnistui**, esimerkiksi *BombVault: vps-helsinki: backup failed*, merkinnän ja syyn kanssa. Saman merkinnän myöhempi onnistunut varmuuskopio päättää jakson.
- **Solmu ei ole tavoitettavissa**, esimerkiksi *BombVault: vps-helsinki is out of reach*. Kotiinstanssi odottaa solmun repositoryjen lyhimmän kuolleen miehen kytkimen ajan, 26 tuntia, ellet muuta sitä, tai 6 tuntia, kun solmulla ei vielä ole määränpäätä.
- **Etäkopio epäonnistui**, esimerkiksi *BombVault: vps-helsinki: off-site copy failed*, kun kotiinstanssi ei voinut kopioida solmun varmuuskopioita johonkin kohteistaan. Seuraava onnistunut kopio päättää jakson.

Kotiinstanssin vastaanotinvalvonta seuraa myös jokaista solmun repositorya kuolleen miehen kytkimellä ja päivittäisellä `restic check` -ajolla kello 04.00, katso [Vastaanotin ja noutaja](offsite-recovery.md#receiver).

## Solmun merkinnät {#entries}

Kohdassa **Varmuuskopiot** Kontit- ja Kansiot-listoissa on osio jokaiselle solmulle omien merkintöjesi alapuolella. Solmun omaa konttia ei listata. Merkinnän valikon **Varmuuskopioi nyt** käynnistää sen varmuuskopion solmulla, ja **Avaa** avaa sen sivun.

Solmun osiossa Kansiot-listassa **Lisää kansiojoukko** lisää solmulle kansiojoukon, ja joukon valikon **Muokkaa** muuttaa sitä. Molemmat avaavat saman ikkunan kuin omille joukoillesi. Ikkunan kansionvalitsin selaa solmun kansioita, mutta ei luo sinne uusia.

![Solmun vps-falkenstein osio Kontit-listassa: nextcloud, nextcloud-db, caddy ja uptime-kuma.](assets/screenshots/nodes/backups-section-dark.png#only-dark)
![Solmun vps-falkenstein osio Kontit-listassa: nextcloud, nextcloud-db, caddy ja uptime-kuma.](assets/screenshots/nodes/backups-section-light.png#only-light)

Solmun merkinnän sivulla lukee **Solmulla** ja solmun nimi, ja sivu listaa merkinnän varmuuskopiot. **Palauta** jonkin niistä kohdalla palauttaa merkinnän solmulle ja korvaa sen, mitä siellä nyt on. Kontti pysäytetään sitä varten, aivan kuten kotiinstanssissakin.

![Merkinnän nextcloud sivu solmulla vps-falkenstein, varmuuskopiot ja jokaisen vieressä Palauta-painike.](assets/screenshots/nodes/entry-dark.png#only-dark)
![Merkinnän nextcloud sivu solmulla vps-falkenstein, varmuuskopiot ja jokaisen vieressä Palauta-painike.](assets/screenshots/nodes/entry-light.png#only-light)

Kotiinstanssin kautta tavoitat kiinteän listan solmun omia reittejä: sen kontit, kansiojoukot, ajot ja poikkeamat sekä niiden varmuuskopioinnin ja palauttamisen. Solmun asetukset, avaimet ja tallennustila jäävät ulottumattomiin, eikä mikään listan reitti poista mitään. Katso [Solmut API:ssa](api.md#nodes).

## Korvaaminen ja vapauttaminen {#replace-release}

**Korvaa** näyttää solmun compose-tiedoston uudelleen, kun olet kirjautunut salasanalla. Käytä sitä, kun siirrät solmun toiselle palvelimelle tai rakennat sen uudelleen: käynnistä tiedosto siellä ja sammuta vanha palvelin. Sama `APP_KEY` ja `NODE_HOME` tekevät siitä taas saman solmun varmuuskopioineen ja historioineen. Kotiinstanssi säilyttää jokaisen solmun kansiojoukot, kontit ja kaiken, mitä niiden uudelleen luomiseen tarvitaan, sekä ajot. Kun solmu käynnistyy tyhjällä `config`-kansiolla, kotiinstanssi ottaa sen taas hoitoonsa, kertoo sille, minne sen varmuuskopiot menevät, ja antaa kaiken tämän takaisin ennen seuraavaa varmuuskopiota, joten solmu voi palauttaa kontin, jota ei enää ole. Uudella palvelimella solmu, joka säilyttää varmuuskopionsa itse, aloittaa tyhjillä repositoryillä. Sen aiemmat varmuuskopiot jäävät kopioihin, jotka kotiinstanssi on hakenut. Kaksi palvelinta, joilla on sama `APP_KEY`, lasketaan yhdeksi solmuksi, joten älä koskaan aja molempia.

**Vapauta** päästää solmun menemään. Se lakkaa ottamasta ohjeita kotiinstanssilta eikä varmuuskopioi enää mitään. Sen varmuuskopiot jäävät paikoilleen, samoin jokainen kopio, jonka kotiinstanssi on hakenut. Sen tunnus vastaanottopalvelimella poistuu, ja kotiinstanssi lopettaa sen repositoryjen valvonnan. Jos solmu on poissa, kun vapautat sen, sen kortti näyttää tilaa **Vapautetaan**, kunnes solmu kuulee asiasta. Vapautettu solmu ei ota uutta kotiinstanssia samalla `APP_KEY`-avaimella. Jos haluat käyttää palvelinta taas, lisää se uutena solmuna ja käynnistä se uudella compose-tiedostolla ja tyhjällä `config`-kansiolla.

## Turvallisuus {#security}

Jos kotiinstanssi kaapataan, hyökkääjä hallitsee myös sen solmuja, ja solmulla on palvelimensa Docker käytössään, mikä vastaa siellä root-oikeuksia. Suojaa kotiinstanssi salasanalla ja pidä se poissa internetistä. Jos solmu kaapataan, hyökkääjä ei saa pariliitosryhmää, linnoitusta, muita solmuja eikä muita repositoryja, eikä voi poistaa historiaa vastaanottopalvelimelta. Yksityiskohdat ovat kohdassa [Solmut tietoturvamallissa](configuration.md#nodes).

[Palautuspaketissa](offsite-recovery.md#recovery-kit) on luku **Solmut**, jossa on jokaisen solmun `APP_KEY`, sen restic-salasana ja sen repositoryt, joten voit palauttaa solmun tai sen varmuuskopiot pelkällä resticillä.
