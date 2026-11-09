# Ilmoitukset

BombVault voi kertoa sinulle, kun varmuuskopio epäonnistuu, onnistuu tai on myöhässä, ja kerran viikossa, miten asiat sujuivat. Kaikki asetetaan kohdassa **Asetukset, Ilmoitukset**: **Ilmoitukset**-kortissa se, mitä BombVault lähettää yksittäisten varmuuskopioiden lisäksi, ja **Kanavat**-kortissa se, minne viestit menevät. Jokainen kanava päättää itse, milloin se ilmoittaa, ja myös Healthchecks-tarkistus on kanava.

![Asetukset, Ilmoitukset, Gotify-kanava täytettynä.](assets/screenshots/notifications/gotify-dark.png#only-dark)
![Asetukset, Ilmoitukset, Gotify-kanava täytettynä.](assets/screenshots/notifications/gotify-light.png#only-light)

## Mikä lähettää viestin {#what}

**Ilmoitukset**-kortin kytkimet koskevat kaikkia kanavia:

- **Tiivistä ajastetut ajot**: yksi viesti ajastettua ajoa kohti, esimerkiksi *42 / 45 onnistui*, yhden viestin sijaan jokaista konttia tai virtuaalikonetta kohti. Itse käynnistämäsi varmuuskopio lähettää silti oman viestinsä. Jäätynyt vieras, epäonnistunut säilytys tai ylitetty kasvubudjetti lähtee heti, vaikka tiivistelmät olisivat päällä.
- **Ilmoita containerin päivityksestä**: yksi viesti jokaisesta kontista, jonka **Päivitä kontit onnistuneen varmuuskopioinnin jälkeen** siirsi uudempaan imageen, jotta voit tarkistaa, että se toimii edelleen.
- **Ilmoita myöhässä olevista varmuuskopioista**: kerran päivässä kello 09.00 BombVault tarkistaa, onko päällä oleva varmuuskopio myöhässä eli vanhempi kuin kaksi kertaa sen aikataulun väli, ja lähettää tapausta kohti yhden viestin, joka nimeää jälkeen jääneen merkinnän. Seuraava onnistunut varmuuskopio poistaa ilmoituksen.
- **Viikkokooste**: yksi yhteenveto viikossa ajojen määristä, uudesta varmuuskopiodatasta, mitkä osiot ovat myöhässä, ovatko etäkopiot ajan tasalla ja yleisimmistä epäonnistumisista. Sen aikataulu on painike kuten jokainen muukin aikataulu, ja **Pois** kytkee sen pois. Katso [Aikataulut](schedules.md#plans).

Se, kuuleeko kanava myös onnistumisista, asetetaan kanavassa kohdassa **Ilmoita**. Mitkä poikkeamat lähettävät viestin, asetetaan kohdassa **Asetukset, Tarkista**, katso [Poikkeamat](anomalies.md). Lykätty varmuuskopio lähettää yhden viestin, kun se alkaa odottaa, ja yhden, kun odotus päättyy, ei yhtä jokaisesta yrityksestä.

## Kanavat {#channels}

**Kanavat**-kortin alareunan **Lisää kanava** tarjoaa vaihtoehdot **Unraid-ilmoitukset**, **ntfy**, **Gotify**, **Discord**, **Slack**, **Matrix**, **Sähköposti (SMTP)**, **Apprise**, **Webhook** ja **Healthchecks**. Lisää niitä niin monta kuin haluat, myös kaksi samanlaista, esimerkiksi yksi ntfy-aihe epäonnistumisille ja toinen kaikelle. Uusi kanava alkaa pois päältä, joten se on tallennettu jo ennen kuin sen kentät on täytetty.

Jokaisella kanavalla on:

- oma kytkin, joka kytkee sen pois päältä kenttiä menettämättä.
- **Ilmoita**: **Ei koskaan**, **Vain epäonnistuessa** tai **Onnistuessa ja epäonnistuessa**. Uusi kanava alkaa asetuksella **Vain epäonnistuessa**. Varmuuskopio, joka onnistui mutta jätti jotain pois, menee myös kanavalle, joka ilmoittaa vain epäonnistumisista.
- **Nimi**, joka on valinnainen ja näkyy tyypin paikalla, kätevä, kun sinulla on kaksi samanlaista.
- **Lähetä testi**, joka lähettää testiviestin juuri tämän kanavan kautta lomakkeessa olevilla arvoilla, tallennettuina tai ei. Kanavan alla oleva rivi kertoo, ottiko palvelu viestin vastaan, ja jos ei, miksi.
- **Poista kanava**, roskakori sen vieressä.

Jokainen kanava lähettää itsenäisesti omalla aikarajallaan, joten yksi isäntä, joka ei koskaan vastaa, ei hidasta muita. Kun palvelu epäonnistuu tavalla, jonka uusi yritys voi korjata, kanava yrittää enintään kolme kertaa. Myöhästymis-, budjetti- tai poikkeamahälytystä, jota mikään kanava ei ottanut vastaan, yritetään myöhemmin uudelleen, ja samoin viikkokoostetta.

| Kanava | Kentät | Huomiot |
|---|---|---|
| **Unraid-ilmoitukset** | ei kenttiä | Lähettää Unraidin omaan ilmoitusjärjestelmään, joka voi välittää viestit Pushoveriin, sähköpostiin tai Discordiin. Se toimii kohdan **Asetukset, Integraatiot, Palvelimen SSH** SSH-yhteyden kautta, joten avain on valtuutettava. TrueNASissa ja muilla isännillä se pysyy hiljaa, ja kanava kertoo miksi. |
| **ntfy** | **Aiheen URL**, **Käyttötunnus (valinnainen)** | Osoite päättyy aiheeseesi. Tunnusta tarvitaan vain suojattuun aiheeseen. |
| **Gotify** | **Palvelimen URL**, **Sovelluksen tunnus**, **Prioriteetti epäonnistuessa**, **Prioriteetti onnistuessa** | Prioriteetit ovat välillä 0-10, ja oletus on 8 epäonnistumisille ja 5 onnistumisille. |
| **Discord** | **Webhook-URL** | Discord-kanavan asetuksista saatavan webhookin osoite. |
| **Slack** | **Webhook-URL** | Slack-sovelluksesi saapuvan webhookin osoite. |
| **Matrix** | **Homeserver-URL**, **Käyttötunniste**, **Huoneen tunnus** | Tunnisteen takana olevan tilin on oltava huoneessa. |
| **Sähköposti (SMTP)** | **SMTP-isäntä**, **Portti**, **Salaus**, **Käyttäjätunnus**, **Salasana**, **Lähettäjän osoite**, **Vastaanottajan osoite** | **Salaus** on **STARTTLS**, **TLS (implicit)** tai **None**. |
| **Apprise** | **Apprise API:n URL**, **Tunnisteet (valinnainen)**, **Apprise-osoitteet (valinnainen)** | Osoita se Apprise API -kontin `/notify/<key>`-päätepisteeseen, niin tavoitat sen yli 100 palvelua, ja tunnisteilla valitaan osa kyseisen avaimen kohteista. Tai luettele Apprise-osoitteet, yksi kullekin riville, esimerkiksi `tgram://` Telegramille tai `pover://` Pushoverille, ja päätä API:n URL osaan `/notify` ilman avainta. |
| **Webhook** | **Webhook-URL**, **Menetelmä**, **Otsakkeet**, **Runko** | Itse muotoilemasi pyyntö, katso [Webhook](#webhook). |
| **Healthchecks** | **Ping-URL** | Katso [Healthchecks](#healthchecks). |

Salaisuudet, kuten tunnukset, SMTP-salasana ja webhook-otsakkeiden arvot, tallennetaan salattuina, eikä palvelin koskaan palauta niitä sivulle. Tallennetun salaisuuden alla kenttä kertoo: **Tallennettu. Jätä kenttä tyhjäksi säilyttääksesi sen.** Kun muutat, minne kanava lähettää, BombVault ei lähetä vanhaa salaisuutta uuteen paikkaan. Kenttä kertoo **Uusi osoite tallentuu, kun syötät tämän uudelleen.**, ja uusi osoite tallentuu heti, kun kirjoitat salaisuuden.

Kanavaluetteloa edeltävän version asetukset siirtyvät ensimmäisellä käynnistyskerralla, eikä mitään katoa. Jokaisesta täyttämästäsi kanavasta tulee oma merkintänsä, jossa on sama **Ilmoita**-valinta kuin ennen. Discord-, Slack-, Gotify- tai ntfy-muotoon asetetusta webhookista tulee sen tyyppinen kanava, ja Yleinen JSON -muotoinen webhook lähettää edelleen kentät `title`, `message` ja `ok` **Runko**-kentän kautta. Unraid-kytkimestä tulee Unraid-kanava ja Healthchecks-URL:sta poikkeuksineen Healthchecks-kanava.

### Webhook {#webhook}

**Webhook**-kanava lähettää yhden HTTP-pyynnön mihin tahansa osoitteeseen. **Menetelmä** on GET, POST tai PUT. **Lisää otsake** lisää rivin kohtaan **Otsakkeet**, ja otsakkeiden arvot säilytetään salaisuuksina, joten `Authorization`-otsake on siellä turvassa. **Runko** lähetetään juuri sellaisena kuin sen kirjoitat, ja nämä paikkamerkit täytetään:

| Paikkamerkki | Tilalle tulee |
|---|---|
| `%%title%%` | viestin otsikko |
| `%%body%%` | itse viesti |
| `%%ok%%` | `true`, tai epäonnistumisessa `false` |
| `%%event%%` | mitä tapahtui, esimerkiksi `container.failed` |
| `%%severity%%` | `info`, `success`, `warning`, `failure` tai `critical` |
| `%%importance%%` | Unraidin taso sille: `normal`, `warning` tai `alert` |
| `%%link%%` | viestiin kuuluva linkki, tyhjä, jos linkkiä ei ole |
| `%%key%%` | osio, esimerkiksi `container` tai `VM` |
| `%%time%%` | aika UTC-ajassa, esimerkiksi `2026-11-09T03:00:00Z` |
| `%%source%%` | `BombVault` |

Paikkamerkit toimivat myös osoitteessa ja otsakkeiden arvoissa, ja jokainen arvo koodataan paikkansa mukaan: JSONina, kun `Content-Type`-otsake kertoo JSONista, ja kyselyarvona osoitteessa. Uusi webhook alkaa tällä otsakkeella ja lähettää `{"title":"%%title%%","message":"%%body%%","ok":%%ok%%}`. Nimi, jota BombVault ei tunne, jää sellaisekseen, joten kirjoitusvirhe näkyy viestissä. Jätä **Runko** tyhjäksi, jos et halua lähettää runkoa.

## Healthchecks {#healthchecks}

Palvelu, kuten [Healthchecks.io](https://healthchecks.io), hälyttää, kun BombVault lakkaa kirjautumasta. Lisää **Healthchecks**-kanava ja liitä tarkistuksen **Ping-URL**, niin BombVault pingaa sitä jokaisen varmuuskopion koko elinkaaren ajan: alussa, sitten onnistuessa tai `/fail`-polulla. Tällä kanavalla ei ole **Ilmoita**-valintaa, koska tarkistus tarvitsee onnistumispingit pysyäkseen vihreänä. Osio, jolla ei ole minne varmuuskopioida, lasketaan epäonnistuneeksi, joten sen tarkistus muuttuu punaiseksi sen sijaan, että se saisi onnistumisviestin.

Saman URL:n osiot jakavat yhden tarkistuksen. Kun yhden niistä varmuuskopio on epäonnistunut, myös muiden onnistuneet varmuuskopiot pingaavat `/fail` ja nimeävät sen, joten tarkistus pysyy punaisena, kunnes kyseisellä merkinnällä on taas onnistunut varmuuskopio tai kuittaat epäonnistumisen kohdassa **Vaatii huomiota**.

Osio voi pingata omaa osoitettaan: Healthchecks-kanavan **Poikkeus** lisää sellaisen, ja rivillä lukee silloin **Kaikki muut osiot pingaavat tätä URL-osoitetta.** **Lähetä testi** kirjoittaa merkinnän jokaisen tarkistuksen lokiin, eikä tarkistuksen tila muutu.

## Esimerkki: Gotify puhelimessasi {#example-gotify}

Käytät Gotifyta osoitteessa `https://gotify.example.com` ja haluat epäonnistumisten soivan puhelimessa, kun taas onnistumiset tulevat hiljaa.

1. Avaa Gotifyssa **Apps**, luo sovellus nimeltä `BombVault` ja kopioi sen tunnus.
2. Avaa BombVaultissa **Asetukset, Ilmoitukset** ja kytke päälle **Tiivistä ajastetut ajot**, jotta 45 varmuuskopion yö lähettää yhden viestin 45:n sijaan.
3. Paina **Kanavat**-kortissa **Lisää kanava** ja valitse **Gotify**.
4. Kirjoita `https://gotify.example.com` kenttään **Palvelimen URL**. BombVault lisää `/message`-osan itse.
5. Liitä tunnus kenttään **Sovelluksen tunnus**.
6. Aseta **Ilmoita** arvoon **Onnistuessa ja epäonnistuessa**. Jätä **Prioriteetti epäonnistuessa** arvoon 8 ja **Prioriteetti onnistuessa** arvoon 5. Gotifyn Android-sovellus pitää arvoa 8 ja sitä suurempia korkeina, joten vain epäonnistumiset päästävät äänen.
7. Kytke kanava päälle ja paina **Lähetä testi**. Kanavan alla oleva rivi kertoo, otiko Gotify viestin vastaan, ja hylätty tunnus mainitaan sellaisenaan.

Seuraava ajastettu ajo lähettää yhden yhteenvedon prioriteetilla 5 tai prioriteetilla 8, jos jokin siinä epäonnistui.

## Esimerkki: ntfy ilman tiliä {#example-ntfy}

ntfy ei tarvitse tiliä julkiselle aiheelle. Valitse aiheen nimi, jota on vaikea arvata, sillä jokainen, joka tietää sen, voi lukea sen.

1. Paina **Kanavat**-kortissa **Lisää kanava** ja valitse **ntfy**.
2. Kirjoita aiheen osoite kenttään **Aiheen URL**, esimerkiksi `https://ntfy.sh/bombvault-tower-7f3k`, tai oman ntfy-palvelimesi aihe. Jätä **Käyttötunnus (valinnainen)** tyhjäksi, kun aihe on julkinen.
3. Tilaa sama aihe ntfy-sovelluksessa, kytke kanava päälle ja paina **Lähetä testi**.

BombVault lähettää viestin leipätekstinä ja otsikon `Title`-otsakkeessa. Epäonnistuminen lähtee korkealla prioriteetilla ja tunnisteella `warning`, joten se erottuu sovelluksessa, ja jäätyneeksi jäänyt virtuaalikone lähtee kiireellisenä.

## Kun viestit eivät tule perille {#troubleshooting}

Katso Vianmäärityksestä [Ilmoitukset eivät saavu perille](troubleshooting.md#notifications-do-not-arrive).
