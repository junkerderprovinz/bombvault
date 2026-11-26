# Android-sovellus

Android-sovellus tuo ryhmäsi kaikki BombVault-palvelimet puhelimeesi. Se avautuu palvelinluetteloon, jonka yläpuolella on kaikkien palvelinten toimintaloki, samat rivit kuin yleiskatsauksessa, ja avaa napauttamasi palvelimen puhelinnäkymän. Sovellus itse ei varmuuskopioi mitään.

![Sovelluksen aloitusnäyttö: kaikkien palvelinten toimintaloki ylhäällä, sen alla palvelinluettelo.](assets/screenshots/android/start-dark.png#only-dark)
![Sovelluksen aloitusnäyttö: kaikkien palvelinten toimintaloki ylhäällä, sen alla palvelinluettelo.](assets/screenshots/android/start-light.png#only-light)

## Sovelluksen hankkiminen {#install}

- **Asetukset, Sovellukset:** Android-sovelluksen kortti tarjoaa APK:n palvelimesi ajamalle julkaisulle, ja mukana on QR-koodi, jonka voit skannata puhelimelta. Kortti linkittää myös F-Droid-listauksen.
- **APK:** jokaisen julkaisun sivulla on `bombvault-android.apk`, ja [tämä linkki](https://github.com/junkerderprovinz/bombvault/releases/latest/download/bombvault-android.apk) lataa aina uusimman version. Sovellus vaatii Android 10:n tai uudemman, ja Android kysyy kerran, saako sovellus, jolla avaat tiedoston, asentaa sovelluksia.
- **Google Play:** [BombVault Google Playssa](https://play.google.com/store/apps/details?id=bombvault.halleluja.design). Se on allekirjoitettu samalla avaimella kuin APK, joten kumpi tahansa asentuu toisen päälle.
- **F-Droid:** [BombVault F-Droidissa](https://f-droid.org/packages/bombvault.halleluja.design/). F-Droid kääntää sovelluksen itse lähdekoodista ja allekirjoittaa sen omalla avaimellaan, joten uusi julkaisu tulee sinne muutaman päivän myöhemmin. Jos haluat vaihtaa F-Droidin sovelluksen ja APK:n välillä, poista toinen ensin.

Pariliitä sovellus palvelimiin, joiden versio on 9.7.0 tai uudempi. Vanhemman version palvelin voi olla samassa ryhmässä, mutta se näyttää puhelimen tavallisena instanssina, ja sovellus lukee kyseisen palvelimen toimintaa vasta kirjautumisen jälkeen.

## Pariliitos QR-koodilla {#pairing}

1. Avaa millä tahansa ryhmäsi palvelimella **Asetukset, Pariliitos** ja valitse **Näytä lause**. Kaksitoista sanaa tulevat näkyviin, ja niiden vieressä on QR-koodi.
2. Napauta sovelluksessa **Skannaa QR-koodi** ja osoita puhelimella koodia. Voit myös liittää tai kirjoittaa sanat.
3. Sovellus näyttää ryhmän palvelimet ennen kuin tallentaa mitään. **Lisää kaikki** lisää ne kaikki.
4. Puhelin odottaa sitten ryhmän palvelimilla kortissa **Odottaa hyväksyntää** kohdassa **Asetukset, Pariliitos**. Paina jollakin niistä **Hyväksy**, niin sovellus näyttää palvelimet tilassa **Yhdistetty**. Pelkät sanat eivät päästä sisään uutta laitetta, katso [Instanssien pariliitos](offsite-recovery.md#pairing).

Puhelin liittyy sitten ryhmään kuin mikä tahansa muu instanssi. Se lukee ryhmän kautta ilman kirjautumista, mitä kullakin palvelimella on käynnissä: kotona suoraan ja kodin ulkopuolella releen kautta. Itse ryhmän toiminta kuvataan kohdassa [Instanssien pariliitos](offsite-recovery.md#pairing).

Sovellus liittyy ryhmään releen kautta, myös kotona. Jos instanssisi asetuksena on **Ei relettä**, valitse rele kohdassa **Asetukset, Pariliitos** ennen kuin skannaat koodin.

## Poissa kotoa {#away}

Sovellus kokeilee palvelinta ensin sen omassa osoitteessa. Jos osoite ei vastaa, esimerkiksi mobiiliverkossa, se avaa palvelimen käyttöliittymän ryhmän releen kautta. Ryhmän palvelin, jonka osoitetta sovellus ei vielä tunne, avautuu samalla tavalla. Jos ei osoite eikä rele vastaa, aloitusnäyttö kertoo sen palvelimen kortissa.

Releen kautta sivu yltää siihen, mihin pariliitetty puhelin yltää kotona, kuten alla kuvataan. Salasanalla kirjautuminen ja tiedostojen tallennus toimivat vain palvelimen omassa osoitteessa. Edistymispalkit päivittyvät kahden sekunnin välein eivätkä jatkuvasti. Käyttöliittymän tiedostot jäävät puhelimeen ensimmäisen käynnin jälkeen, joten myöhemmät käynnit latautuvat nopeammin. Ennen versiota 10.0.0 oleva palvelin avautuu vain omassa osoitteessaan.

Sovellus hyväksyy vastauksen vain, jos palvelin on allekirjoittanut sen avaimella, jonka sovellus kiinnitti, kun se tapasi palvelimen ensimmäisen kerran. Jos palvelin vastaa myöhemmin toisella avaimella, aloitusnäyttö näyttää uuden avaimen sormenjäljen, ja sivu avautuu vasta, kun luotat siihen.

## Palvelimen vaihtaminen {#switch}

Kun palvelin on auki, sen yleiskatsauksen yläreunassa näkyy palvelimen nimi ja sen edessä paluunuoli. Nuoli vie sovelluksen yleiskatsaukseen eli aloitusnäyttöön, joka luettelee kaikki sovelluksen tuntemat palvelimet tiloineen: **Yhdistetty**, **Ei yhteyttä**, **Kirjautuminen tarvitaan** tai **Tarkista varmenne**. Ryhmän palvelin, joka ei vastaa, on **Ei aktiivinen**, harmaana, ja sen kohdalla lukee, milloin ryhmä näki sen viimeksi, ja palvelin, josta sovellus ei ole vielä kuullut, näyttää **Tarkistetaan**. Joka kerta yhdistäessään sovellus lukee ryhmän luettelon yhdeltä sen ilmentymistä, joten luettelossa on myös ilmentymä, jota sovellus ei ole koskaan tavannut releen kautta, esimerkiksi sellainen, joka oli sammutettuna, kun puhelin liittyi ryhmään. Napauta siellä toista avataksesi sen, kotona suoraan ja muulloin releen kautta.

## Kirjautuneena pariliitetyllä puhelimella {#sign-in}

Ryhmääsi pariliitetty puhelin avaa jokaisen ryhmän palvelimen valmiiksi kirjautuneena. Ennen sivun lataamista se pyytää ryhmän kautta palvelimelta istunnon, ja palvelin antaa sellaisen vain jäsenelle, joka on puhelin.

Pariliitetty puhelin ei saa kaikkea, minkä salasana antaa. Sen istunto yltää kiinteään luetteloon siitä, mitä sovellus näyttää: yleiskatsaus, ajot ja toimintaloki, katseltavat varmuuskopiot, käynnistys, tarkistus ja palautus, poikkeamat, päivitysten tarkistus ja katseltava Ilmentymät-sivu, ilman jäsenten vastaanotettuja repositoryja ja noutolähteitä. Varmuuskopio- ja merkintäsivuilla se voi myös ottaa merkinnän ajastukseen tai poistaa sen siitä, hylätä sen, lisätä varmuuskopiolle tunnisteen, poistaa ZFS-puun jääneet tilannevedokset ja etsiä merkintöjä varmuuskopioiden tallennustilasta. Merkinnän lisääminen jää verkkokäyttöliittymälle, koska uusi merkintä valitaan palvelimen kansioista. Kohteet, ZFS-palvelimet ja komennot tulevat sille ilman osoitteitaan, kirjautumistietojaan ja salaisuuksiaan.

Palautus, joka korvaa alkuperäisen, varmuuskopioi ensin nykytilan: kontin, virtuaalikoneen, kansiojoukon, koko Compose-stackin projektikansioineen, flash-liitännäisen (flash varmuuskopioidaan) ja käynnissä olevaan tietokantaan tuotavan tietokantavedoksen (kontti varmuuskopioidaan). Jos varmuuskopiointi epäonnistuu, mitään ei korvata. Palautus kansioon, samoin kuin kansioon tallennettava tietokantavedos, menee uuteen kansioon kohdassa **Asetukset, Palautus** asetetun palautuskansion sisälle, eikä ikkuna tarjoa muuta kansiota.

Kohdassa **Muut lähteet** valinta **Vanhasta varmuuskopiokansiosta** avaa toisen ilmentymän repositoryn sen avaimella tai palautuspaketilla. Verkossa oleva repository avautuu kirjoittamillasi kirjautumistiedoilla, joita käytetään vain siinä istunnossa eikä niitä koskaan lähetetä takaisin. Palvelimella oleva kansio avautuu vain palautuskansion sisältä, ja jokainen merkintä palaa tiedostoina uuteen kansioon sinne: konttia tai virtuaalikonetta ei luoda uudelleen puhelimesta. **Tuo Appdata.Backup-liitännäisestä** ja **Tehtävät Duplicatista tai Duplicacysta** vaativat kirjautumisen itse palvelimella, koska ne lukevat kansioita mistä tahansa palvelimelta tai luovat kohteita ja kirjautumistietoja.

Puhelimessa kuitattu tai odotetuksi merkitty poikkeama pitää vanhat varmuuskopiot tallessa, kunnes joku kirjautuu salasanalla ja valitsee suljettujen löydösten joukosta **Lopeta vanhojen varmuuskopioiden säilytys**. Avaimet, palautuspaketti, avaintalletukset, asetukset ja varmuuskopioiden poistaminen vaativat kirjautumisen itse palvelimella, ja kaikki muu vastaa tarvitsevansa sen kirjautumisen. Releen kautta pätee sama luettelo, myös palvelimella, jolla ei ole salasanaa.

Jokainen puhelin, jonka ryhmä on päästänyt sisään, kirjautuu itse, eikä sille ole kytkintä. Salasanan nollaus kirjaa jokaisen puhelimen ulos, ja puhelimet kirjautuvat sen jälkeen itse uudelleen. Jos haluat pitää yhden puhelimen ulkona, poista se Ilmentymät-sivulla.

## Varmenteet {#certificates}

Itse allekirjoitettuun varmenteeseen luotetaan kerran sen SHA-256-sormenjäljen perusteella. Jos palvelin myöhemmin näyttää eri varmenteen, sovellus varoittaa sinua ja avaa sen vasta, kun luotat uuteen varmenteeseen.

## Puhelin Ilmentymät-sivulla {#instances}

Puhelin saa oman korttinsa ryhmän jokaisen palvelimen Ilmentymät-sivulle, merkittynä Android-sovellukseksi ja nimettynä puhelimelle antamasi nimen mukaan. Sillä ei ole tuloskorttia, koska se ei varmuuskopioi mitään. **Poista** poistaa sen sivulta ja päättää sen istunnon heti, samoin kuin **Kysele tätä instanssia** -kohdan kytkeminen pois sen tietojen kohdassa **Valvonta**. Puhelin tuntee yhä sanat, joten se ilmestyy takaisin, kun sen sovellus avataan, mutta pois kytkettynä, eikä se saa istuntoa ennen kuin joku kytkee sen päälle. Kadonneen puhelimen kohdalla poista sen kortti jokaiselta palvelimelta tai anna jokaisen ilmentymän valita **Poistu ryhmästä** ja pariuta ne uudelleen uusilla sanoilla.

Ennen versiota 10.0.0 pariutettu puhelin alkaa allekirjoittaa kutsunsa ensimmäisellä käynnillä päivityksen jälkeen, ja jokainen palvelin pitää ensimmäisen puhelimelle saamansa avaimen. Jos joku muu, joka tuntee ryhmän sanat, ehtii ensin puhelimen ollessa offline-tilassa, palvelin hylkää puhelimen siitä lähtien, eikä sovellus enää voi avata sitä releen kautta. Korjaa tämä kirjautumalla kyseiselle palvelimelle selaimella, valitsemalla puhelimen kortista Ilmentymät-sivulla **Unohda avain** ja sitten **Vahvista unohtaminen**, ja avaamalla sovellus uudelleen. Palvelin ottaa puhelimen oman avaimen seuraavassa kutsussa. Jos puhelin hylätään uudelleen, joku muu tuntee sanat: anna jokaisen ilmentymän valita **Poistu ryhmästä** ja pariuta ne uudelleen uusilla sanoilla, kuten kadonneen puhelimen kohdalla.

## Solmut {#nodes}

[Solmuja](nodes.md) hoidetaan verkkokäyttöliittymässä. Pariutetun puhelimen istunto ei ulotu niihin, joten sovellus ei näytä Ilmentymät-sivulla solmujen kortteja, yleiskatsauksessa **Solmut**-korttia eikä kohdassa **Varmuuskopiot** solmujen osioita, ja **Lisää solmu** vastaa, että se vaatii kirjautumisen itse palvelimella. Avaa palvelin selaimessa ja kirjaudu salasanalla, kun haluat lisätä solmun, muuttaa sen varmuuskopioiden määränpäätä tai palauttaa jonkin sen merkinnöistä.

## Asetukset {#settings}

Plussan vieressä oleva ratas avaa sovelluksen asetukset:

- kieli sekä nimi, jolla puhelin näkyy Ilmentymät-sivulla (tyhjä tarkoittaa puhelimen mallia),
- ulkoasu, joka seuraa luettelon ensimmäistä palvelinta, kunnes asetat oman, sekä animaatiot, joilla on oma asetuksensa,
- Versio-kortti, joka kertoo, mistä kaupasta päivitykset tulevat (Google Play, F-Droid tai GitHub Releases), ja avaa tämän version tiedot kohdasta **Uutta**, sekä Komponentit-kortti,
- raportti kopioitavaksi, kun ilmoitat ongelmasta. Se ei sisällä osoitetta, nimeä eikä lausetta,
- Tietoja-kortti tietosuojaselosteineen,
- **Poista kaikki palvelimet**, joka poistaa sovelluksesta kaikki palvelimet ja eroaa ryhmästä. Jokainen palvelin, jonka puhelin vielä tavoittaa, poistaa sen Ilmentymät-sivultaan.

## Lataukset ja lähetykset {#files}

Pariliitetyn puhelimen istunto tallentaa yhdenlaisia tiedostoja: tietokantavedos latautuu puhelimen Lataukset-kansioon, kun sovellus avaa palvelimen sen omassa osoitteessa. Kaikkea muuta varten kirjaudu ensin salasanalla palvelimen omassa osoitteessa. Näin kirjautuneena viennit, palautuspaketit ja flashin ZIP-tiedostot tallentuvat puhelimen Lataukset-kansioon samaan tapaan kuin selaimesta, ja asetusten tuonti avaa puhelimen tiedostovalitsimen. Releen kautta ei tallenneta tiedostoja.

## Kosketusnäytöllä {#touch}

Sormen alla mikään ei saa hover-tilaa, joten säädin himmenee, kun sitä pidetään painettuna, ja brändilogollinen painike hehkuu brändin värissä, kunnes nostat sormen. Pitkä painallus painikkeeseen lasketaan hitaaksi napautukseksi eikä avaa linkkivalikkoa. Harmaana olevan painikkeen napautus näyttää, miksi se on harmaana, kunnes napautat jotain muuta kohtaa.

## Selaimessa sovelluksen sijaan {#browser}

Chrome ja Edge voivat asentaa BombVaultin verkkokäyttöliittymän sovellukseksi omaan ikkunaansa, puhelimessa samoin kuin tietokoneessa. Mitään ei tallenneta välimuistiin, joten päivitys näkyy heti.

## Tietosuoja {#privacy}

Sovelluksessa ei ole tilejä, mainoksia eikä analytiikkaa, eikä se aja mitään taustalla. Sen [tietosuojaseloste](https://github.com/junkerderprovinz/bombvault/blob/main/android/PRIVACY.md) kertoo, mitä se tallentaa ja mitä se lähettää minne.
