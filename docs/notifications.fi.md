# Ilmoitukset

BombVault voi kertoa sinulle, kun varmuuskopio epäonnistuu, onnistuu tai on myöhässä, ja kerran viikossa, miten asiat sujuivat. Kaikki asetetaan kohdassa **Asetukset, Ilmoitukset**: **Ilmoitukset**-kortissa se, mikä laukaisee viestin, **Kanavat**-kortissa se, minne viestit menevät, ja **Healthchecks**-kortissa kirjautumispingit.

![Asetukset, Ilmoitukset, Gotify-kanava täytettynä.](assets/screenshots/notifications/gotify-dark.png#only-dark)
![Asetukset, Ilmoitukset, Gotify-kanava täytettynä.](assets/screenshots/notifications/gotify-light.png#only-light)

## Mikä lähettää viestin {#what}

**Ilmoitukset**-kortti asettaa säännöt kaikille kanaville kerralla:

- **Ilmoita**: **Ei koskaan**, **Vain epäonnistuessa** tai **Onnistuessa ja epäonnistuessa**, kaikille alla oleville kanaville.
- **Tiivistä ajastetut ajot**: yksi viesti ajastettua ajoa kohti, esimerkiksi *42 / 45 onnistui*, yhden viestin sijaan jokaista konttia tai virtuaalikonetta kohti. Itse käynnistämäsi varmuuskopio lähettää silti oman viestinsä. Jäätynyt vieras, epäonnistunut säilytys tai ylitetty kasvubudjetti lähtee heti, vaikka tiivistelmät olisivat päällä.
- **Ilmoita containerin päivityksestä**: yksi viesti jokaisesta kontista, jonka **Päivitä kontit onnistuneen varmuuskopioinnin jälkeen** siirsi uudempaan imageen, jotta voit tarkistaa, että se toimii edelleen.
- **Ilmoita myöhässä olevista varmuuskopioista**: kerran päivässä kello 09.00 BombVault tarkistaa, onko päällä oleva varmuuskopio myöhässä eli vanhempi kuin kaksi kertaa sen aikataulun väli, ja lähettää yhden viestin tapausta kohti. Seuraava onnistunut varmuuskopio poistaa ilmoituksen.
- **Unraid-ilmoitukset**: lähettää Unraidin omaan ilmoitusjärjestelmään, joka voi välittää viestit Pushoveriin, sähköpostiin tai Discordiin. Se toimii kohdan **Asetukset, Integraatiot, Palvelimen SSH** SSH-yhteyden kautta, joten avain on valtuutettava. TrueNASissa ja muilla isännillä se pysyy pois päältä.
- **Viikkokooste**: yksi yhteenveto viikossa ajojen määristä, uudesta varmuuskopiodatasta, ovatko etäkopiot ajan tasalla ja yleisimmistä epäonnistumisista. Sen aikataulu on painike kuten jokainen muukin aikataulu, ja **Pois** kytkee sen pois. Katso [Aikataulut](schedules.md#plans).

Mitkä poikkeamat lähettävät viestin, asetetaan kohdassa **Asetukset, Tarkista**, katso [Poikkeamat](anomalies.md). Lykätty varmuuskopio lähettää yhden viestin, kun se alkaa odottaa, ja yhden, kun odotus päättyy, ei yhtä jokaisesta yrityksestä.

**Lähetä testi** kortin alareunassa lähettää testiviestin jokaisen päällä olevan kanavan kautta lomakkeessa olevilla arvoilla.

## Kanavat {#channels}

Jokaisella kanavalla on oma kytkimensä **Kanavat**-kortissa, ja sen kentät näkyvät, kun se on päällä. Jokainen kanava lähettää itsenäisesti omalla aikarajallaan, joten yksi isäntä, joka ei koskaan vastaa, ei hidasta muita, ja viestiä, jonka mikään kanava ei ottanut vastaan, yritetään uudelleen.

| Kanava | Kentät | Huomiot |
|---|---|---|
| **Webhook** | **Webhook-URL**, **Webhook-muoto** | Muodot: **Yleinen JSON**, **Discord**, **Slack**, **Gotify** ja **ntfy**. Yleinen JSON lähettää kentät `title`, `message` ja `ok`. |
| **Apprise** | **Apprise API:n URL**, **Tunnisteet (valinnainen)** | Osoita se Apprise API -kontin `/notify/<key>`-päätepisteeseen, niin tavoitat sen yli 100 palvelua. Tunnisteilla valitaan osa kyseisen avaimen kohteista. |
| **Matrix** | **Homeserver-URL**, **Käyttötunniste**, **Huoneen tunnus** | Tunniste pysyy palvelimella, eikä sitä näytetä enää koskaan. |
| **Gotify** | **Palvelimen URL**, **Sovelluksen tunnus**, **Prioriteetti epäonnistuessa**, **Prioriteetti onnistuessa** | Prioriteetit ovat välillä 0-10, ja oletus on 8 epäonnistumisille ja 5 onnistumisille. |
| **Sähköposti (SMTP)** | **SMTP-isäntä**, **Portti**, **Salaus**, **Käyttäjätunnus**, **Salasana**, **Lähettäjän osoite**, **Vastaanottajan osoite** | **Salaus** on **STARTTLS**, **TLS (implicit)** tai **Ei mitään**. |

Salaisuudet, kuten Matrix-tunniste, Gotifyn sovellustunnus ja SMTP-salasana, tallennetaan salattuina. Tallennetun salaisuuden alla kenttä kertoo: **Tallennettu. Jätä kenttä tyhjäksi säilyttääksesi sen.** Kun muutat palvelimen osoitetta, BombVault ei lähetä vanhaa salaisuutta uuteen paikkaan. Kenttä pyytää sitä uudelleen, ja uusi osoite tallentuu, kun olet kirjoittanut sen.

## Healthchecks {#healthchecks}

Palvelu, kuten [Healthchecks.io](https://healthchecks.io), hälyttää, kun BombVault lakkaa kirjautumasta. Liitä tarkistuksen **Ping-URL**, ja BombVault pingaa sitä jokaisen varmuuskopion koko elinkaaren ajan: alussa, sitten onnistuessa tai `/fail`-polulla. Se tekee niin aina, kun URL on asetettu, riippumatta siitä, mitä **Ilmoita** sanoo, joten tarkistus pysyy vihreänä onnistumisessa myös silloin, kun viestit tulevat vain epäonnistumisista.

Osio voi pingata omaa osoitettaan: kortin **Poikkeus** lisää sellaisen, ja rivillä lukee **Kaikki muut osiot pingaavat tätä URL-osoitetta.**

## Esimerkki: Gotify puhelimessasi {#example-gotify}

Käytät Gotifyta osoitteessa `https://gotify.example.com` ja haluat epäonnistumisten soivan puhelimessa, kun taas onnistumiset tulevat hiljaa.

1. Avaa Gotifyssa **Apps**, luo sovellus nimeltä `BombVault` ja kopioi sen tunnus.
2. Avaa BombVaultissa **Asetukset, Ilmoitukset**. Aseta **Ilmoitukset**-kortissa **Ilmoita** arvoon **Onnistuessa ja epäonnistuessa** ja kytke päälle **Tiivistä ajastetut ajot**, jotta 45 varmuuskopion yö lähettää yhden viestin 45:n sijaan.
3. Kytke **Kanavat**-kortissa päälle **Gotify**.
4. Kirjoita `https://gotify.example.com` kenttään **Palvelimen URL**. BombVault lisää `/message`-osan itse.
5. Liitä tunnus kenttään **Sovelluksen tunnus**.
6. Jätä **Prioriteetti epäonnistuessa** arvoon 8 ja **Prioriteetti onnistuessa** arvoon 5. Gotifyn Android-sovellus pitää arvoa 8 ja sitä suurempia korkeina, joten vain epäonnistumiset päästävät äänen.
7. Paina **Lähetä testi**. Kortin alla oleva rivi kertoo, otiko Gotify viestin vastaan, ja hylätty tunnus mainitaan sellaisenaan.

Seuraava ajastettu ajo lähettää yhden yhteenvedon prioriteetilla 5 tai prioriteetilla 8, jos jokin siinä epäonnistui.

## Esimerkki: ntfy webhookina {#example-ntfy}

ntfy ei tarvitse tiliä julkiselle aiheelle. Valitse aiheen nimi, jota on vaikea arvata, sillä jokainen, joka tietää sen, voi lukea sen.

1. Kytke **Kanavat**-kortissa päälle **Webhook**.
2. Kirjoita aiheen osoite kenttään **Webhook-URL**, esimerkiksi `https://ntfy.sh/bombvault-tower-7f3k`, tai oman ntfy-palvelimesi osoite.
3. Aseta **Webhook-muoto** arvoon **ntfy**.
4. Tilaa sama aihe ntfy-sovelluksessa ja paina **Lähetä testi**.

BombVault lähettää viestin sisällön leipätekstinä ja otsikon `X-Title`-kentässä. Epäonnistuminen lähtee korkealla prioriteetilla ja tunnisteella `warning`, joten se erottuu sovelluksessa.

## Kun viestit eivät tule perille {#troubleshooting}

Katso Vianmäärityksestä [Ilmoitukset eivät saavu perille](troubleshooting.md#notifications-do-not-arrive).
