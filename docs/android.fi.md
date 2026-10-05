# Android-sovellus

Android-sovellus tuo ryhmäsi kaikki BombVault-palvelimet puhelimeesi. Se avautuu palvelinluetteloon, jonka yläpuolella on kaikkien palvelinten toimintaloki, samat rivit kuin Kojelaudassa, ja avaa napauttamasi palvelimen puhelinnäkymän. Sovellus itse ei varmuuskopioi mitään.

## Sovelluksen hankkiminen {#install}

- **Asetukset, Sovellukset:** Android-sovelluksen kortti tarjoaa APK:n palvelimesi ajamalle julkaisulle, ja mukana on QR-koodi, jonka voit skannata puhelimelta.
- **APK:** jokaisen julkaisun sivulla on `bombvault-android.apk`, ja [tämä linkki](https://github.com/junkerderprovinz/bombvault/releases/latest/download/bombvault-android.apk) lataa aina uusimman version. Sovellus vaatii Android 10:n tai uudemman, ja Android kysyy kerran, saako sovellus, jolla avaat tiedoston, asentaa sovelluksia.
- **Google Play:** sovellus on suljetussa testauksessa, kunnes se voidaan julkaista. Google Play julkaisee uuden kehittäjätilin sovelluksen vasta, kun vähintään 12 testaajaa on pitänyt sen asennettuna 14 päivän ajan. Jos haluat auttaa, liity [testaajaryhmään](https://groups.google.com/g/arrowloop-testers), avaa [testisivu](https://play.google.com/apps/testing/bombvault.halleluja.design), napauta **Ryhdy testaajaksi** ja asenna BombVault Google Playsta.
- **F-Droid:** listaus tulee myöhemmin.

Pariliitä sovellus palvelimiin, joiden versio on 9.7.0 tai uudempi. Vanhemman version palvelin voi olla samassa ryhmässä, mutta se näyttää puhelimen tavallisena instanssina, ja sovellus lukee kyseisen palvelimen toimintaa vasta kirjautumisen jälkeen.

## Pariliitos QR-koodilla {#pairing}

1. Avaa millä tahansa ryhmäsi palvelimella **Asetukset, Pariliitos** ja valitse **Näytä lause**. Kaksitoista sanaa tulevat näkyviin, ja niiden vieressä on QR-koodi.
2. Napauta sovelluksessa **Skannaa QR-koodi** ja osoita puhelimella koodia. Voit myös liittää tai kirjoittaa sanat.
3. Sovellus näyttää ryhmän palvelimet ennen kuin tallentaa mitään. **Lisää kaikki** lisää ne kaikki.

Puhelin liittyy sitten ryhmään kuin mikä tahansa muu instanssi. Se lukee ryhmän kautta ilman kirjautumista, mitä kullakin palvelimella on käynnissä: kotona suoraan ja kodin ulkopuolella releen kautta. Itse ryhmän toiminta kuvataan kohdassa [Instanssien pariliitos](offsite-recovery.md#pairing).

!!! note "Käyttöliittymä tarvitsee yhä reitin palvelimelle"
    Palvelinluettelo ja toimintaloki tulevat ryhmän kautta. Palvelimen käyttöliittymä avautuu kuitenkin suoraan, joten puhelimen on tavoitettava palvelimen osoite, kotona tai VPN:n kautta.

## Kirjautuneena pariliitetyllä puhelimella {#sign-in}

Ryhmääsi pariliitetty puhelin avaa jokaisen ryhmän palvelimen valmiiksi kirjautuneena. Ennen sivun lataamista se pyytää ryhmän kautta palvelimelta istunnon, ja palvelin antaa sellaisen vain jäsenelle, joka on puhelin. Osoitteella lisätty palvelin kysyy salasanaa kuten selaimessa. Kuka tahansa, joka tietää kaksitoista sanaa, voi jo nyt avata jokaisen ryhmän varmuuskopion, joten pariliitos ei anna mitään uutta.

## Ryhmän ulkopuoliset palvelimet {#other-servers}

- **Lisää palvelin** ottaa osoitteen, jolla avaat BombVaultin selaimessa, kuten `192.168.1.10:3443`. Jos alussa ei ole `http://` tai `https://`, sovellus käyttää https:ää.
- Lähiverkossa itsestään ilmoittavat palvelimet näkyvät kohdassa **Tässä verkossa** ja avautuvat yhdellä napautuksella. Ne ilmoittavat itsestään niin kauan kuin **Löydä verkosta** on päällä kohdassa Asetukset, Integraatiot. Toisessa verkossa tai VPN:n takana oleva palvelin ei näy siellä.
- Itse allekirjoitettuun varmenteeseen luotetaan kerran sen SHA-256-sormenjäljen perusteella. Jos palvelin myöhemmin näyttää eri varmenteen, sovellus varoittaa sinua ja avaa sen vasta, kun luotat uuteen varmenteeseen.

## Puhelin Ilmentymät-sivulla {#instances}

Puhelin saa oman korttinsa ryhmän jokaisen palvelimen Ilmentymät-sivulle, merkittynä Android-sovellukseksi ja nimettynä puhelimelle antamasi nimen mukaan. Sillä ei ole tuloskorttia, koska se ei varmuuskopioi mitään, ja **Poista** poistaa sen sivulta.

## Asetukset {#settings}

Plussan vieressä oleva ratas avaa sovelluksen asetukset:

- kieli sekä nimi, jolla puhelin näkyy Ilmentymät-sivulla (tyhjä tarkoittaa puhelimen mallia),
- ulkoasu, joka seuraa luettelon ensimmäistä palvelinta, kunnes asetat oman, sekä animaatiot, joilla on oma asetuksensa,
- raportti kopioitavaksi, kun ilmoitat ongelmasta; se ei sisällä osoitetta, nimeä eikä lausetta,
- Tietoja-kortti tietosuojaselosteineen,
- **Poista kaikki palvelimet**, joka poistaa sovelluksesta kaikki palvelimet ja eroaa ryhmästä. Itse palvelimilla mikään ei muutu.

## Lataukset ja lähetykset {#files}

Viennit, palautuspaketit, flashin ZIP-tiedostot ja tietokantavedokset tallentuvat puhelimen Lataukset-kansioon samaan tapaan kuin selaimesta. Asetusten tuonti avaa puhelimen tiedostovalitsimen.

## Kosketusnäytöllä {#touch}

Sormen alla mikään ei saa hover-tilaa, joten säädin himmenee, kun sitä pidetään painettuna, ja brändilogollinen painike hehkuu brändin värissä, kunnes nostat sormen. Pitkä painallus painikkeeseen lasketaan hitaaksi napautukseksi eikä avaa linkkivalikkoa.

## Selaimessa sovelluksen sijaan {#browser}

Chrome ja Edge voivat asentaa BombVaultin verkkokäyttöliittymän sovellukseksi omaan ikkunaansa, puhelimessa samoin kuin tietokoneessa. Mitään ei tallenneta välimuistiin, joten päivitys näkyy heti.

## Tietosuoja {#privacy}

Sovelluksessa ei ole tilejä, mainoksia eikä analytiikkaa, eikä se aja mitään taustalla. Sen [tietosuojaseloste](https://github.com/junkerderprovinz/bombvault/blob/main/android/PRIVACY.md) kertoo, mitä se tallentaa ja mitä se lähettää minne.
