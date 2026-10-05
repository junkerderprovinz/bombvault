# Etäsijainti ja palautus

!!! note "Etäkopiot odottavat uudelleenrakennuksen jälkeen"
    Kun vaihe 4 rakentaa merkinnät uudelleen ilman vanhoja asetuksia, näiden toimialueiden etäreplikointi keskeytyy, kunnes sijoittelun oletus vahvistetaan. Katso [Sijoittelu per kohde](#placement).

Paikalliset varmuuskopiot suojaavat sinua kadonneelta kontilta tai huonolta päivitykseltä. Etäreplikointi ja testattu palautuspaketti suojaavat sinua koko laatikon menetykseltä, kiristysohjelmalta tai tulipalolta. Tämä sivu käsittelee etäsijaintiin replikoinnin, kyseisen kopion peukalointisuojauksen, palautuskyvyn todistamisen ja toipumisen silloin kun BombVault itse on kadonnut.

## Etäreplikointi

Säilytä nopea paikallinen varmuuskopio ja lisää yksi tai useampi etäreplika. Aseta repo per toimialue **Asetukset, Etä** -sivulla. BombVault replikoi uudet tilannevedokset sinne `restic copy` -komennolla parhaan yrityksen periaatteella, joten etäsijainnin nikottelu ei koskaan kaada paikallista varmuuskopiota. Tässä muodossa paikallinen repo pysyy ensisijaisena ja etärepo on replika, mutta toimialueen ensisijaisen repon ei tarvitse olla lainkaan paikallinen; katso alta [Etäsijaintiset ensisijaiset arkistot](#remote-primary-repositories), jos haluat varmuuskopioida suoraan S3:een, rest-serveriin tms. sen sijaan että replikoisit sinne.

- **Useita etäkohteita per toimialue.** Jokainen toimialue (kontit, virtuaalikoneet, flash, config, tiedostojoukot ja ZFS-tietojoukot) voi replikoitua useaan etäkohteeseen kerralla, ei vain yhteen, joten voit pitää esimerkiksi rest-serverin ystävän laatikossa ja S3-ämpärin rinnakkain. Lisää lisäkohteita kohtaan Asetukset, Etä, kukin omalla repositoriollaan, S3-tallennusluokallaan, append-only-lipullaan, säilytyksellään ja kasvubudjetillaan. Olemassa oleva yksittäinen etämääritys siirretään koskemattomana ensimmäiseksi kohteeksi, ja toimialueen jokainen kohde replikoituu kyseisen toimialueen etäaikataulun mukaan.
- **Toimialuekohtainen etäaikataulu** (muokattuna jokaisen muun aikataulun rinnalla kohdassa Asetukset, Aikataulut): jätä se tyhjäksi replikoidaksesi jokaisen paikallisen varmuuskopion jälkeen, tai aseta tahti (esimerkiksi `weekly Sun 03:00`) lähettääksesi etäsijaintiin harvemmin kuin varmuuskopioit paikallisesti. **Replikoi nyt** -painike kattaa pyydettäessä tehtävät ajot.
- **Etäsäilytys** asuu kohdassa Asetukset, Säilytys, jotta voit säilyttää etäkopioita pidempään arkistona. Jätä käytäntö pelkiksi nolliksi, jotta etätilannevedoksia ei koskaan karsita automaattisesti.
- **Kaistanleveyden rajat** (Asetukset, Etä) rajoittavat resticin lähetys-/latausnopeutta, jotta replikointi ei tuki WAN-yhteyttäsi.
- **Replikointiosoitin** näyttää, mikä toimialue replikoituu sen ollessa käynnissä (sen sivulla ja Kojelaudalla). Se on aktiivisuusosoitin, ei prosenttipalkki, koska `restic copy` ei paljasta koneluettavaa edistymistä.

!!! note "Palautus mistä tahansa paikasta"
    Jokainen kontti, VM, tiedostojoukko, flash ja sovelluksen asetukset listaavat varmuuskopionsa yhtenä aikajanana kaikkien paikkojen yli, joissa varmuuskopio sijaitsee. B2:een kopioitu varmuuskopio näkyy vain kerran, merkittynä jokaisella paikalla joka sen sisältää. Palautus ottaa ensimmäisen paikan, johon se pääsee, aloittaen arkistosta johon kohde on kirjoitettu, ja voit valita toisen paikan riviä kohden. Etäpaikat luetaan vasta, kun avaat ne. Poistaminen yhdessä paikassa tarkistaa ensin muut ja kertoo, oliko se viimeinen kopio.

## Sijoittelu per kohde {#placement}

Jokaisella kontti-, VM- ja tiedostojoukkokortilla on **Sijoittelu**-rivi kolmella segmentillä:

- **Paikallinen** kirjoittaa kohteen arkistoon, joka näkyy kohdassa **Tallennuspaikka**, eikä kopioi sitä minnekään. Käytä tätä datalle, jolla on jo toinen kopio, esimerkiksi jaolle joka asuu NAS-laitteella.
- **Paikallinen + etä** kirjoittaa sen myös sinne ja kopioi sen kohdassa **Kopiointikohteet** rastitettuihin kohteisiin, yksi chip per toimialueen etäkohde. Poista chipin rasti, niin kyseinen kohde ei saa mitään uutta tältä kohteelta.
- **Vain etä** kirjoittaa kohteen suoraan kohdassa **Lähetyskohde** näkyvään paikkaan: suoraan arkistoon etäkohteen vieressä, tai etäarkistoon jonka olet perustanut kohdassa Asetukset, Tallennus, Arkistot.

Sijainti on kiinteä kohteen ensimmäisestä varmuuskopiosta lähtien, koska BombVault ei koskaan siirrä varmuuskopioita arkistojen välillä. Kopiot voivat muuttua milloin tahansa. Kohde, joka ei enää saa uutta kohdetta, säilyttää olemassa olevat kopionsa ja typistää ne omaan säilytykseensä toimialueen seuraavalla etäajolla; **Poista kohteessa B2** kortilla poistaa ne heti. Kun osa noista kopioista ei ole olemassa missään muualla, vahvistus listaa ne päivämäärän mukaan ja pyytää kohteen nimeä. Vain lisäystä sallivista kohteista ei voi poistaa.

Rivin alla kortti kertoo, minne kohde menee ja mitä siellä oikeasti on: kuinka monessa paikassa se on, milloin kukin kohde nähtiin viimeksi, ja täyttyykö 3-2-1. Paikka on alkuperäisen datan sisältävä palvelin, jokainen etäkohde ja jokainen **Rakennuksen ulkopuolella** merkitty arkisto. BombVault tarkistaa kopiot ja paikat; se ei tarkista 3-2-1:n "kaksi mediaa" -osaa.

### Sijoittelun oletukset

Asetukset, Tallennus, **Sijoittelun oletukset** sisältää yhden rivin per toimialue, samoilla kolmella segmentillä. Kopiot pätevät heti jokaiseen kohteeseen, jolla ei ole omaa valintaa, sekä Compose-pinojen projektikansioihin. Sijainti pätee uuteen kohteeseen sen ensimmäisessä varmuuskopiossa; sen muuttaminen ei siirrä yhtään varmuuskopiota. Ennen tallennusta rivi nimeää jokaisen kohteen, joka saa tai menettää kohteita, ja kuinka monta tilannevedosta se tarkoittaa. **Käytä kohteisiin ilman varmuuskopioita** palauttaa oletukseen jokaisen kohteen, jolla ei vielä ole varmuuskopiota.

Uusi etäkohde saa jokaisen kohteen, jota ei ole asetettu Paikalliseksi. Sen lisäävä valintaikkuna kertoo kuinka monta kohdetta on kyseessä ja, jos tiedossa, kuinka paljon historiaa se on, ja tarjoutuu jättämään pois kohteet jotka on jo jätetty pois muista kohteista.

### Suorat arkistot

Kohteen suoran arkiston valitseminen kohdassa Vain etä avaa valintaikkunan, jossa on ehdotettu sijainti kohteen vieressä, esimerkiksi `s3:https://s3.eu-central-003.backblazeb2.com/bucket/containers-direct`, ja yhteystesti joka ei luo mitään. **Luo ja käytä** luo arkiston ja osoittaa kohteen siihen. Suora arkisto ottaa kohteen avaimen, tallennusluokan, rajat, append-only-asetuksen ja säilytyksen, ja muuttuu niiden mukana; Arkistot-kortti näyttää sen vain luku -tilassa. Kun kohteen uusi avain ei avaa sitä, suora arkisto säilyttää avaimen joka sillä on, ja tallennus kertoo sen. Sen tilannevedokset kantavat tunnistetta `bv:direct`, ja jokainen muu säilytysajo säästää ne, joten suora arkisto joka on menettänyt yhteytensä kohteeseensa ei koskaan vanhene paikallisten sääntöjen mukaan. B2:een päästään sen S3-päätepisteen kautta syöttämällä avaintunnus ja sovellusavain S3-tunnuksina; kohteen omaan kansioon rajattu avain ei pääse sen vieressä olevaan kansioon, joten rajaa avain sen sijaan kohteen yläpuolella olevaan kansioon.

### Rakennuksen ulkopuolella

Nimetty arkisto voidaan merkitä **Rakennuksen ulkopuolella** Arkistot-kortilla. Etäarkistot alkavat merkittyinä; kytke se pois rest-serveriltä samassa rakennuksessa. Merkintä laskee vain paikat ja 3-2-1:n korteilla. Se ei muuta yhtään kopiota.

### Uudelleenrakennuksen jälkeen

Kopiointivalinnat asuvat BombVaultin omissa asetuksissa. Uudelleenrakennuksen jälkeen Tunnista varmuuskopiot -toiminnolla ilman palautettua `/config`-kansiota ne ovat poissa, ja kaiken kopiointi lähettäisi B2:een uudelleen kohteet jotka olit jättänyt pois. Siksi jokaisen uudelleenrakennetun toimialueen etäreplikointi keskeytyy. Kojelauta näyttää sen keltaisena, ja Sijoittelun oletukset tarjoaa **Vahvista oletus** -toiminnon, jossa on esikatselu siitä mitä seuraava ajo kopioi ja nimet varmuuskopioissa joilla ei ole merkintää, jotka voit jättää pois siellä. Vain vahvistus lopettaa keskeytyksen; asetustiedoston tuonti tuo takaisin säännöt ja oletukset muttei lopeta sitä.

## Etäsijaintiset ensisijaiset arkistot {#remote-primary-repositories}

Alueen varmuuskopiopolku (Asetukset, Tallennus) ei rajoitu paikalliseen kansioon: osoita se suoraan restic-etäarkistoon (`s3:...`, `rest:http://isanta:8000/arkisto`, `sftp:kayttaja@isanta:/arkisto`, `rclone:etä:bucket/polku`), niin BombVault varmuuskopioi suoraan sinne, ilman erillistä paikallista kopiota ja ilman replikointivaihetta. Tämä on aidosti eri muoto kuin yllä kuvattu off-site-replikointi: siellä paikallinen arkisto on ensisijainen ja off-site-arkisto sen paras mahdollinen arkistokopio; täällä etäarkisto **on** ensisijainen ja ainoa kopio, ellet määritä kyseiselle alueelle lisäksi off-site-replikointia (tai toista etäarkistoa).

Kussakin kuudesta polkukentästä (Kontit, VMs, Flash, Itsevarmuuskopio, Kansiot, ZFS-tietojoukot) on aivan vieressä kytkin **Paikallinen / Etä**:

- **Paikallinen** näyttää tutun kansioselaimen.
- **Etä** vaihtaa sen tavalliseen URL-kenttään ja lisää painikkeen, joka avaa saman yhteystestin ja tunnusten valintaikkunan kuin off-site-kohteet käyttävät, mutta tälle ensisijaiselle arkistolle säädettynä. Sieltä saat:
    - **Yhteystestin** todellista polkua vasten, ennen kuin luotat siihen.
    - **Kaistarajat** (lähetys ja lataus), jottei ajastettu varmuuskopiointi etäensisijaiseen arkistoon täytä WAN-yhteyttäsi: samat restic-valitsimet `--limit-upload` ja `--limit-download`, joita off-site-replikointi käyttää, nyt itse varmuuskopiointiin sovellettuina.
    - **Append-only-suojan (muuttumattomuus)**, varmennettuna samalla aktiivisella peukalointitestillä (aito DELETE-koetus vastapuolta vastaan), jonka off-site-kohteet saavat. Päällä ollessaan BombVault kieltäytyy karsimasta arkistoa itse: koska takana ei ole erillistä paikallista kopiota, tämän koneen tunnukset eivät saa kyetä poistamaan varmuuskopion ainoaa kappaletta.
    - **Kasvubudjetin hälytyksen**, joka johdetaan samasta arkiston koon kehityksestä, jota Tallennus-kortti jo seuraa.

Mikään tästä ei ole pakollista: käsin kirjoitettu etäpolku ilman tallennettuja turva-asetuksia varmuuskopioi täsmälleen kuten ennenkin (rajaton kaista, karsittavissa, ei budjettihälytystä). Turvavalintaikkuna on siltä varalta, että haluat samat suojaukset kuin off-site-kopio saa, ilman että sinun tarvitsee luoda erillistä off-site-kohdetta vain sitä varten.

!!! note "Pilvi- ja REST-tunnukset ovat yhteiset"
    Etäensisijainen arkisto tunnistautuu samoilla S3-/REST-tunnuksilla, jotka on määritetty kohdassa Asetukset, Pilvipääsy, Jaetut pilvitunnistetiedot. Ensisijaisille arkistoille ei ole erillistä tunnusvarastoa.

### SMB ja WebDAV ilman liitosta isäntään {#smb-webdav}

Kohdassa Asetukset, Pilvipääsy, rclone on lomake Windows- tai Samba-jaolle ja WebDAV-palvelimelle (Nextcloud, ownCloud, SharePoint tai mikä tahansa muu). Täytä lyhyt nimi, isäntä ja jako (SMB) tai URL ja palvelintyyppi (WebDAV), käyttäjä ja salasana, niin BombVault kirjoittaa rclone-osion puolestasi. rclone hämärtää salasanan itse ennen tallennusta; kohteen lisääminen nimellä, joka on jo olemassa, korvaa kyseisen osion sen sijaan että lisäisi toisen.

Lomake vastaa valmiilla sijainnilla, esimerkiksi `rclone:nas:backups`. Laita se varmuuskopiopolkuun tai etäkohteeseen ja lisää halutessasi alikansio (`rclone:nas:backups/bombvault`). Jako on polun ensimmäinen osa, ei osa nimeä.

Tämä on parempi tapa kuin jaon liittäminen Unraidiin: restic neuvoo olemaan pitämättä repositoriota liitetyllä CIFS-jaolla, eikä tässä liitetä mitään. NFS ei ole lomakkeessa, koska resticillä ja rclonella ei ole NFS-taustajärjestelmää; NFS:ää varten liitä export isäntään ja osoita varmuuskopiopolku siihen.

## Muuttumaton (append-only) etäsijainti

Merkitse etärepo append-only-tilaan, jotta kiristysohjelma, tai vaarantunut isäntä, ei voi poistaa tai uudelleenkirjoittaa varmuuskopioitasi. Vastapuoli (`restic/rest-server`, joka pyörii `--append-only`-tilassa) **valvoo** sitä. BombVault vain aina **todentaa** sen eikä koskaan näytä vihreää pelkän kokoonpanoväitteen perusteella.

**Ohjattu etäsijainnin määritys** -toiminto vie sinut taustajärjestelmän valinnasta (rest-server / rclone / S3) valmiin liitettävän rest-server-käyttöönottokatkelman, yhteystestin, muuttumattomuuskytkimen (joka ajaa peukalointitestin heti) ja säilytysstrategian läpi, joten append-only-etäsijainti on tavoitettavissa ilman määritysten käsin muokkaamista.

!!! note "Onnistunut poisto polussa `/locks/` on odotettua"
    Append-only ei tarkoita, ettei mitään voisi enää poistaa. resticin on otettava ja vapautettava omat lukkonsa, joten `/locks/` pysyy tarkoituksella kirjoitettavana ja poistettavana. Tilannevedoksia ja niiden takana olevaa dataa, eli juuri sitä mihin kiristysohjelma tähtäisi, ei voi poistaa. Jos koettelet vastapuolta itse, onnistunut poisto polussa `/locks/` on oikea toiminta eikä aukko suojauksessa.

!!! warning "Muuttumattomia repoja ei koskaan karsita tästä laatikosta"
    Muuttumaton etäsijainti ei tarkoituksella koskaan karsi vanhoja tilannevedoksia. Aseta sille **kasvubudjettihälytys**, jotta saat hälytyksen ennen kuin repon koko karkaa käsistä.

## Peukalointitesti

BombVault todistaa ajoittain append-only-takuun tosiasiallisesti yrittämällä poistoa etärepoa vasten, kohdistettuna olemattomaan objektiin:

- **Torjuttu** tarkoittaa suojattua.
- **Hyväksytty** tarkoittaa ei-suojattua.
- **Epäselvä** tulos (palvelin tavoittamattomissa, todennusvirhe) ei koskaan käännä tallennettua tuomiota.

Todellinen suojatusta suojaamattomaksi -kääntyminen laukaisee yhden hälytyksen.

## DR-harjoitukset

BombVault tarjoaa kaksi tasoa todisteita siitä, että varmuuskopiosi ovat tosiasiassa palautuskelpoisia, eivät vain olemassa.

- **Palautuksen tarkistusharjoitukset (paikallinen).** BombVault ajaa ajoittain `restic check --read-data-subset` (rajattu, ei koskaan levyn täyttävää täyspalautusta) ja näyttää *Palautettavuus vahvistettu* -merkin per toimialue. Tahti asuu kohdassa Asetukset, Aikataulut; merkki kohdassa Asetukset, Eheys.
- **DR-harjoitukset (etä).** BombVault palauttaa oikean kohteen etärepositoriosta kertakäyttöiseen hiekkalaatikkoon, tarkistaa sen tiedosto tiedostolta ja tavu tavulta, ja siivoaa sitten. Tämä todistaa, että voit toipua etäsijainnista, ei vain että repo vastaa.

**Kiristysohjelmasuojan tuloskortti** Kojelaudalla kokoaa tämän vihreä / keltainen / punainen -asennoksi per toimialue, iällä leimatun tarkistuslistan kera (etäsijainti määritetty, append-only todennettu, replikointi ajan tasalla, palautusharjoitus läpäisty, salaus päällä, karsintastrategia asetettu). Jokainen punainen rivi linkittää syvälle korjaukseen, ja kortti muuttuu vihreäksi vain todennettujen tosiasioiden perusteella.

## Instanssien pariliitos {#pairing}

Vastaanottimet, noutolähteet, Ilmentymät-sivu ja Mesh-etäsijainti kaikki puhuvat toiselle BombVaultille. Ne tekevät sen yhden pariliitosryhmän jäseninä, ja instanssi liittyy ryhmään kahdellatoista sanalla.

Avaa ensimmäisessä instanssissa **Asetukset → Pariliitos** ja valitse pariliitoskorteista **Luo lause**. Näkyviin tulee kaksitoista sanaa ikkunassa, jossa on **Kopioi**-painike. Avaa jokaisessa muussa instanssissa sama paikka, valitse **Syötä lause** ja liitä tai kirjoita ne, tai valitse kyseisessä ikkunassa **Liitä**. Sanan, joka ei ole listalla, sivu nimeää sijainteineen heti kirjoitettaessa, ja viimeinen sana sisältää tarkistussumman, joten väärin kirjoitettu tai vaihtunut sana huomataan ennen kuin mikään pariutuu. Luo lause vain yhdessä instanssissa: kaksi instanssia, jotka molemmat luovat lauseen, muodostavat kaksi erillistä ryhmää. Jos kukaan ei ilmesty minuutin kuluessa, välilehti tarjoaa kaksi keinoa: näytä sanat uudelleen syöttääksesi ne toisella instanssilla, tai syötä toisen instanssin sanat ja liity sen ryhmään yhdellä askeleella. Pariliitos toimii ilman kirjautumissalasanaakin, mutta aseta sellainen: ilman sitä kuka tahansa, joka pystyy avaamaan tämän käyttöliittymän, voi lukea sanat ja saada ryhmän kautta jokaisen siihen kuuluvan instanssin restic-salasanan. Pariliitoskortti kertoo tämän, kunnes salasana on asetettu. Salasanan kanssa lauseen näyttäminen uudelleen kysyy sitä. **Poistu ryhmästä** ottaa instanssin taas pois.

Kuka tahansa sanat tietävä voi liittyä ryhmään, joten kohtele niitä kuin salasanaa.

**Miten jäsenet tavoittavat toisensa.** Jokainen instanssi oppii oman osoitteensa verkossa selaimestasi heti, kun kirjaudut sisään, ja se näkyy relekortissa nimellä **Tämä instanssi verkossasi**; korjaa se siellä, jos edessä on käänteinen proxy tai epätavallinen portti. Samassa verkossa jäsenet ilmoittavat tuon osoitteen multicastilla ja puhuvat suoraan, ja siellä missä multicast ei pääse konttiverkon läpi, kuten Dockerin oletusarvoisen bridge-verkon, instanssi sen sijaan etsii omasta aliverkostaan muita allekirjoitetulla kutsulla, johon vain ryhmän jäsen osaa vastata, joten pariliitos valmistuu silti sekunneissa ilman relettä. Jos mitään ei löydy, **Etkö löydä sitä?** pariliitoskortin alla ottaa yhden osoitteen käsin syötettynä, toista aliverkkoa tai muuta kuin vakioporttia varten. Eri verkoissa olevat instanssit kulkevat releen kautta, joka valitaan samalla välilehdellä:

- **Projektin rele** (oletus): `parleyport.halleluja.design`, sama rele jota myös KnightLoader käyttää. Ei mitään asennettavaa.
- **Oma rele**: [**ParleyPort**](https://github.com/junkerderprovinz/parleyport) -kontti Unraidin Community Appsista, tai joku instansseistasi, joka on jo tavoitettavissa ulkopuolelta ja jossa **Toimi releenä** on päällä. Kyseinen instanssi vastaa silloin osoitteessaan `/relay/connect`, sen jo olemassa olevan käänteisen proxyn ja sertifikaatin takana, ja päästää sisään vain oman ryhmäsi. Syötä releen osoite jokaiseen instanssiin, jonka pitäisi käyttää sitä.
- **Ei relettä**: jäsenet löytävät toisensa automaattisesti vain samassa verkossa, eivät missään muualla.

**Mitä rele näkee.** Jokainen jäsenten välinen kutsu on sinetöity AES-256-GCM:llä avaimella, joka on johdettu kahdestatoista sanasta, eikä se avain koskaan poistu instansseistasi. Rele saa tietää hajautusarvon, joka ryhmittelee yhteydet, sekä sen, kenelle instanssille viesti on tarkoitettu, kuinka suuri se on ja milloin se kulkee läpi. Suora kutsu paikallisverkossa on sinetöity samalla tavalla ja lisäksi allekirjoitettu, joten mikään ei riipu instanssin tarjoamasta itse allekirjoitetusta varmenteesta.

**Mitä ryhmän kautta kulkee.** Ilmentymät-sivun tuloskortit, pyyntö tarkistaa yksi toimialue heti, Mesh-etäsijainnin tarjoukset, ja se mitä vastaanotin tai noutolähde tarvitsee: toisen instanssin repositorion sijainnit ja sen restic-salasana. Varmuuskopiodata ei koskaan kulje sitä kautta, vaan menee edelleen suoraan restic-taustajärjestelmiin. Ei myöskään APP_KEY: restic-salasana avaa sen instanssin repositoriot eikä mitään muuta, ei sen tallennettuja salaisuuksia, istuntoja eikä palautuskoodeja.

**Ennen pariliitosta tehdyt merkinnät.** Fleet-token-tunnuksella lisätyt instanssit sekä toisen instanssin APP_KEY:llä asetetut vastaanottimet ja noutolähteet säilyvät päivityksen jälkeen ja on merkitty **Pariuta uudelleen**. Vastaanottimet ja noutolähteet jatkavat toimintaansa: ensimmäisellä käynnistyksellään BombVault korvaa jokaisen tallennetun APP_KEY:n siitä johdetulla restic-salasanalla. Pariuta molemmat instanssit, muokkaa sitten merkintää ja valitse sen instanssi. Tällainen instanssi ottaa vanhan korttinsa takaisin heti kun samanniminen instanssi ilmestyy ryhmään.

Ainoa paikka, joka yhä ottaa APP_KEY:n käsin, on [Palautus toisesta BombVault-repositoriosta](#restore-from-another-bombvault-repo), sitä tapausta varten, jossa toinen instanssi on poissa eikä voi vastata ryhmässä.

## Vastaanottimen kojelauta (vastaanottava puoli)

![Vastaanottava puoli, vain luku -tilassa valvottuna, ja eheystarkistus ajetaan tällä koneella.](assets/screenshots/receiver.png)

*Vastaanottava puoli, vain luku -tilassa valvottuna, ja eheystarkistus ajetaan tällä koneella.*

Kaikki yllä oleva on *lähettävä* puoli. Laatikossa, joka **vastaanottaa** muuttumattomia etäkopioita toisesta BombVaultista, Vastaanottimen kojelauta antaa sinulle riippumattoman, vain luku -tilaisen valvonnan noista repositorioista vastaanottavalla laitteistolla, jotta hiljainen epäonnistuminen vastapäässä ei jää huomaamatta.

Kytke **Vastaanotin**-kytkin päälle Asetuksissa paljastaaksesi **Vastaanotin**-välilehden. Se on oletuksena pois päältä; ota se käyttöön vain laatikossa, joka tosiasiassa vastaanottaa muuttumattomia etävarmuuskopioita. Rekisteröi sitten vastaanotettu repositorio (vain luku, avattuna lähettävän instanssin restic-salasanalla, joka saapuu [pariliitosryhmän](#pairing) kautta) saadaksesi:

- **Lähteittäin ryhmitellyn tilannevedosinventaarion**, jotta näet tarkalleen mitkä kontit, virtuaalikoneet ja tiedostojoukot ovat saapuneet.
- **Viimeksi vastaanotettu** per lähde, jotta tiedät kuinka tuore kukin on.
- **Riippumattoman `restic check`** -ajon vastaanottavalla laitteistolla, jotta eheys todennetaan siellä missä data tosiasiassa sijaitsee, ei vain lähettäjällä.
- **Kuolleen miehen kytkimen:** hälytys, kun lähde lakkaa lähettämästä asettamasi ikkunan sisällä.
- **Eheyshälytykset:** hälytys, kun tarkistus vastaanottavalla puolella epäonnistuu.

Vastaanotin on ehdottoman vain luku -tilainen. Se ei koskaan kirjoita vastaanotettuun repositorioon, joten se ei voi koskaan rikkoa append-only-takuuta, johon lähettäjä nojaa.

## Läpikäyty esimerkki: kaksi Unraid-konetta, päästä päähän

Yllä kuvataan osat. Tässä on yksi kokonainen kokoonpano oikeilla arvoilla, sillä osat on helpompi koota, kun ne on kerran nähnyt koottuina.

Kaksi konetta: **TOWER** ajaa kontit ja lähettää varmuuskopiot, **VAULT** ottaa ne vastaan ja pakottaa muuttumattomuuden. Korvaa omilla nimillä, osoitteilla ja jakopoluilla.

**1. Pystytä append-only-palvelin VAULTiin.** Mene TOWERin BombVaultissa kohtaan *Asetukset → Etä → Määritä*, valitse **rest-server** ja luo resepti. Kopioi välilehti **Unraid-malli (XML)**, tallenna se VAULTiin nimellä `/boot/config/plugins/dockerMan/templates-user/my-rest-server.xml`, valitse sitten *Docker → Add Container* ja mallilistasta **rest-server**. Kirjoita näytetty `htpasswd`-rivi VAULTissa tiedostoon `/mnt/user/appdata/rest-server/.htpasswd` ennen käynnistystä. Kertakäyttösalasana näytetään kerran eikä sitä tallenneta, joten kopioi se nyt. Se rivi sisältää saman salasanan, jo bcrypt-tiivistettynä: selkoteksti menee TOWERin REST-tunnuksiin, tiivistetty rivi VAULTin `.htpasswd`-tiedostoon. Sinun ei tarvitse tiivistää mitään itse.

    Jätä `--append-only` OPTIONS-kenttään. Se on koko juju: ilman sitä VAULT on taas tavallinen jako.

**2. Osoita etävarasto sinne TOWERissa.** Varaston osoite noudattaa reseptin tulostamaa muotoa:

    rest:http://VAULT:8000/bombvault-containers/containers

Polun ensimmäinen osa on htpasswd-käyttäjä, toinen on varasto. Syötä luotu käyttäjä ja salasana kohteen REST-tunnuksiksi ja aja **yhteystesti**.

**3. Kytke TOWERissa ”Muuttumaton” päälle.** Peukalointitesti ajetaan heti ja sen on sanottava *suojattu*. Mitä vastaukset tarkoittavat:

| Tulos | Mitä tapahtui |
| --- | --- |
| **suojattu** | VAULT kieltäytyi poistosta. Tämä on ainoa hyväksytty tila. |
| **EI suojattu** | VAULT hyväksyi poiston. `--append-only` puuttuu tai se on poistettu. |
| **ei ratkaiseva** | Ei kumpikaan. Yleensä osoite ei ole se, jota restic itse käyttää, tai tunnukset ovat muuttuneet. Mitään ei kirjata eikä hälytystä laukaista. |

**4. Katso VAULTissa, mitä saapuu.** Pariuta molemmat laatikot ([Instanssien pariliitos](#pairing)), kytke päälle *Asetukset → Yleiset → Vastaanotin*, avaa **Vastaanotin**-välilehti ja rekisteröi varasto vain luku -tilassa TOWER lähettävänä instanssina.

!!! warning "Sijainti on polku kontin **sisällä**, kirjoitettuna suhteessa isäntäliitokseen"
    Syötä `user/appdata/rest-server/bombvault-containers/containers`, **ei** `/mnt/user/appdata/…`. BombVault ajetaan kontissa, jossa isännän `/mnt` on liitetty muualle; isännän absoluuttista polkua ei siellä ole. Jos liität sellaisen, BombVault kertoo nyt käytettävän suhteellisen polun.

    VAULT saa TOWERin restic-salasanan ryhmän kautta tallennettaessa; kukaan ei kirjoita avainta.

**5. Tee siitä molemminpuolinen, jos haluat.** Toista samat viisi vaihetta toiseen suuntaan: rest-server TOWERissa vastaanottamassa VAULTin kopiota. Silloin kumpikin kone pakottaa muuttumattomuuden toiselle, eikä kumpikaan voi poistaa toisen varmuuskopioita.

## Ohjattu palautus

Erillinen **Palautus**-välilehti opastaa tuoreen tai uudelleenrakennetun asennuksen läpi katastrofitilanteen, yhdessä paikassa:

1. **Palauttaa BombVaultin omat asetukset ensin**, jotta varmuuskopiopolut, etäkohteet ja tunnukset, joita muu kulku tarvitsee, tulevat esitäytettyinä (sovellettuna itsensä uudelleenkäynnistyksellä Docker-soketin yli, joten käynnissä olevaa asetustietokantaa ei koskaan ylikirjoiteta avoimen kahvan alla).
2. **Tarkistaa, että BombVault voi lukea varmuuskopiosi** (salausavaimen kompastuskivi heti alkuun).
3. Antaa sinun **osoittaa olemassa olevaan repoosi** (paikallinen tai etä).
4. **Tunnistaa** siihen tallennetut kontit, virtuaalikoneet, tiedostojoukot ja ZFS-tietojoukot.
5. **Palauttaa kontit ja virtuaalikoneet kerralla** (jätettynä pysäytetyiksi, jotta käynnistät ne harkiten) ja luettelee tiedostojoukot ja ZFS-kohteet, jotka palautat yksi kerrallaan; ZFS-kohteet palaavat pois päältä. Palautuspakettisi on yhden napsautuksen päässä.

!!! tip "Suunniteltu siirto vastaan katastrofi"
    Ohjattu palautus palauttaa BombVaultin omat asetukset varmuuskopiosta. *Suunniteltua* siirtoa uuteen laatikkoon varten voit sen sijaan kantaa kokoonpanosi mukanasi suoraan **Vie / tuo asetukset** -kortilla (siirrettävä JSON-tiedosto). Katso [Asetukset](configuration.md#portable-settings-export-and-import).

### Palautus toisesta BombVault-repositoriosta {#restore-from-another-bombvault-repo}

Erillinen kortti **Palautus**-välilehdellä avaa *toisen* BombVault-instanssin repon (kohtaan `/mnt` liitetty jako tai etä-URL) **kyseisen instanssin `APP_KEY`:llä**, kertaluonteisessa, vain luku -tilaisessa istunnossa. Selaa siihen tallennettuja kontteja, virtuaalikoneita ja tiedostojoukkoja, valitse tilannevedos ja palauta se, ja palautetusta objektista tulee normaali paikallinen kontti, VM tai tiedostojoukko. Toiseen repoon ei koskaan kirjoiteta mitään, ja omat varmuuskopioasetuksesi pysyvät koskemattomina (istunto asuu muistissa ja vanhenee itsestään). Kontin siirtäminen palvelimelta A palvelimelle B ei tarkoita repoasetustesi uudelleensuuntaamista ja niiden palauttamista jälkeenpäin. Tämä kortti on kertaluonteinen: se avaa istunnon, palauttaa valitsemasi ja unohtaa toisen instanssin. Jos haluat sen sijaan pysyvän järjestelyn, jossa tämä laatikko noutaa toisen instanssin tilannevedokset omaan repoonsa aikataulun mukaan, siihen on **Ilmentymät**-sivun **Nouto**-välilehti.

Kontti, jonka verkkoa tällä palvelimella ei ole, esimerkiksi Unraidin `br0`-verkko tavallisella Docker-isännällä, näyttää rivinsä alla verkon valinnan. BombVault luo sen valitsemaasi verkkoon MAC-osoitteineen ja muine verkkoineen. Kiinteä IP-osoite kuului vanhaan verkkoon ja jää pois, joten uusi verkko antaa osoitteen.

## Salausavaimen palautuspaketti

Tämä on se pala, joka tekee katastrofista toipumisen mahdolliseksi silloinkin kun käynnissä olevaa BombVaultia ei ole.

Yksi napsautus lataa **pääavaimen**, **johdetun restic-salasanan** ja **tarkat repon sijainnit ja komennot**, joten voit palauttaa suoraan restic-komentorivillä millä tahansa koneella. Kojelaudan muistutus nalkuttaa, kunnes olet tallentanut sen.

!!! danger "Säilytä palautuspaketti palvelimen ulkopuolella"
    Paketti sisältää salaisuuden, joka purkaa varmuuskopiosi salauksen. Pidä se turvallisessa paikassa erillään palvelimesta (salasananhallinta, tulostettu kopio kassakaapissa). Jos menetät sekä BombVaultin että `APP_KEY`:n ilman palautuspakettia, salattuja varmuuskopioitasi ei voi palauttaa.

!!! warning "Uusin tilannevedos ei aina ole se, joka kannattaa palauttaa"
    Restic 0.17:stä lähtien `restic snapshots` näyttää jokaisen tilannevedoksen koon. Tietojen menetyksen jälkeen uusin tilannevedos voi olla tyhjennetty, joten älä palauta tilannevedosta, joka on paljon edellisiä pienempi. Kiristyshaittaohjelman jälkeen se voi olla salattu, tavallisen kokoinen. Jos BombVault on yhä käynnissä, katso ensin sen sivu **Poikkeamat**: se nimeää viimeisen hyvän varmuuskopion. Palautus ei tarvitse BombVaultin poikkeamatietoja, ja säilytyksen tauko vain säilyttää enemmän tilannevedoksia.

### Paketin sinetöinti

Jos olet ottanut age-salauksen käyttöön selkokielisille vienneille (Asetukset), paketti sinetöidään myös sillä ja se latautuu nimellä `bombvault-recovery-kit.md.age`. Se on ASCII-armored-muodossa eikä binäärinen, joten se on yhä pelkkää tekstiä: sen liittäminen salasanojen hallintaan tai tulostaminen toimii täsmälleen kuten ennenkin, sisältö on vain lukukelvoton ilman avaintasi.

!!! warning "Älä säilytä age-avainta paketin sisällä"
    Sinetöidyn paketin avaamiseen tarvitset age-**yksityisavaimesi**. Säilytä se paikassa, joka ei riipu itse paketista, tai sinulla on palautettavana kaksi asiaa yhden sijaan. Sinetöinti kannattaa, kun paketti on tallessa paikassa, jota et täysin hallitse (jaettu salasanojen hallinta, pilvimuistiinpanot, tuloste toimistolla); omassa kassakaapissasi oleva paketti on jo kassakaapin suojaama.

    Kun salaus on päällä eikä käyttökelpoista vastaanottajaa ole määritetty, lataus torjutaan suoraan. BombVault ei koskaan turvaudu antamaan pääavainta selkotekstinä.

### Kun paketti ei ole käsillä

Salasanaa ei ole tallennettu mihinkään, se **lasketaan** `APP_KEY`-avaimesta. Avaimen ja komentotulkin avulla voit siis muodostaa sen itse:

```sh
printf 'bombvault:restic-repo' \
  | openssl dgst -sha256 -mac HMAC -macopt hexkey:$APP_KEY -r \
  | cut -d' ' -f1
```

Kyseessä on HMAC-SHA256 kiinteästä merkkijonosta `bombvault:restic-repo`, avaimena heksadesimaalisen `APP_KEY`-arvon raakatavut, tulostettuna 64 pienenä heksamerkkinä. Sama arvo on paketissa johdettuna restic-salasanana; tämä on sitä päivää varten, jona paketti on jossain muualla kuin sinä.

!!! warning "Vastaanotetussa arkistossa käytä LÄHETTÄVÄN instanssin avainta"
    Arkisto, joka päätyi tänne off-site-replikoinnilla, luotiin sen lähettäneellä koneella **sen omalla** `APP_KEY`-avaimella. Vastaanottavan koneen avaimesta johtaminen tuottaa salasanan, jonka restic hylkää, ja se näyttää täsmälleen rikkinäiseltä arkistolta olematta sitä. Tämä on tavallisin syy siihen, että `restic check` kyselee vastaanotetulla arkistolla salasanaa yhä uudelleen.

Koska palautusmääritykset asuvat kunkin repon **sisällä** (`<repo>/def`, `<repo>/vm-def`), kopioitu repokansio on täysin itsenäinen, joten paketti plus repo on kaikki mitä paljasrautainen palautus tarvitsee.

## Tietokantavedoksen hakeminen takaisin {#database-dumps}

Tietokantavedos on oma palautuspisteensä konttivarastossa, tunnisteella `dbdump:<container>` ja yhdellä tiedostolla, `/dbdump/<container>.sql`. BombVault luetteloi, lataa ja tuo ne kohdassa **Varmuuskopiot**; alla samat vaiheet pelkällä resticillä, sitä päivää varten kun BombVaultia ei ole.

```sh
restic -r <repo> snapshots --tag dbdump:<container>
restic -r <repo> dump --tag dbdump:<container> latest /dbdump/<container>.sql > <container>.sql
```

Kunkin vedoksen tunnisteet `dbversion:` ja `dbname:` kertovat, mistä palvelinversiosta se on ja mitä tietokantoja se sisältää. Täydellinen tiedosto päättyy riviin `-- PostgreSQL database cluster dump complete` tai `-- Dump completed`.

Tuo se konttiin, jossa on sama tai uudempi versio (PostgreSQL) tai sama pääversio (MySQL ja MariaDB) ja joka on käynnistetty kerran tyhjällä datakansiolla, jotta se alustaa itsensä. Isäntä ei tarvitse tietokanta-asiakasta, kontissa on sellainen:

```sh
docker exec -i <container> sh -c 'exec psql -X -U "${POSTGRES_USER:-postgres}" -d postgres' < <container>.sql
docker exec -i <container> sh -c 'exec mariadb -uroot -p"$MARIADB_ROOT_PASSWORD"' < <container>.sql
docker exec -i <container> sh -c 'exec mysql -uroot -p"$MYSQL_ROOT_PASSWORD"' < <container>.sql
```

Yhden tietokannan poimimiseen täydestä vedoksesta MySQL ja MariaDB ottavat asiakasohjelman komennolle `--one-database <name>`. PostgreSQL-vedoksessa on jokaiselle tietokannalle oma osansa, joka alkaa rivillä `\connect <name>`: kopioi se osa omaan tiedostoonsa ja tuo se `-d <name>`-valitsimella sen jälkeen kun olet luonut tietokannan.

!!! warning "Rootina otettu vedos tuo mukanaan palvelimen käyttäjät"
    Rootina otettu täysi MySQL- tai MariaDB-vedos sisältää järjestelmätietokannan `mysql`, joten sen tuonti korvaa uuden palvelimen tilit, root-salasana mukaan lukien, vedoksen tileillä. PostgreSQL:ssä ilmoitus `role ... already exists` kontin itsensä luomasta käyttäjästä on odotettu eikä haittaa.
