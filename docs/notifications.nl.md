# Meldingen

BombVault kan je laten weten wanneer een back-up mislukt, wanneer er een slaagt, wanneer er een achterstallig is, en één keer per week hoe het ging. Alles stel je in onder **Instellingen, Meldingen**: wat BombVault naast losse back-ups verstuurt in de kaart **Meldingen**, en waarheen berichten gaan in de kaart **Kanalen**. Elk kanaal bepaalt zelf wanneer het iets laat horen, en ook een Healthchecks-check is een kanaal.

![Instellingen, Meldingen met het kanaal Gotify ingevuld.](assets/screenshots/notifications/gotify-dark.png#only-dark)
![Instellingen, Meldingen met het kanaal Gotify ingevuld.](assets/screenshots/notifications/gotify-light.png#only-light)

## Wat een bericht veroorzaakt {#what}

De schakelaars in de kaart **Meldingen** gelden voor elk kanaal:

- **Geplande runs samenvatten**: één bericht per geplande run, zoals *42 van de 45 geslaagd*, in plaats van één per container of VM. Een back-up die je met de hand start, stuurt nog steeds een eigen bericht. Een bevroren gast, een mislukte bewaarronde of een overschreden groeibudget gaat meteen de deur uit, ook met samenvattingen aan.
- **Melden bij containerupdate**: één bericht voor elke container die **Containers bijwerken na een geslaagde back-up** naar een nieuwere image heeft verplaatst, zodat je kunt nagaan dat hij nog werkt.
- **Achterstallige back-ups melden**: eenmaal per dag om 09:00 controleert BombVault of een ingeschakelde back-up achterstallig is, dus ouder dan twee keer zijn planning, en stuurt het per geval één bericht, met de naam van het item dat achterloopt. De volgende geslaagde back-up ruimt het op.
- **Wekelijkse samenvatting**: één samenvatting per week met het aantal runs, nieuwe back-updata, welke onderdelen achterstallig zijn, of de off-site-kopieën actueel zijn en de belangrijkste mislukkingen. De planning ervan is een knop zoals elke andere planning, en **Uit** schakelt haar uit. Zie [Planningen](schedules.md#plans).

Of een kanaal ook over successen hoort, stel je in bij het kanaal zelf, onder **Melden**. Welke anomalieën een bericht sturen, stel je in onder **Instellingen, Verificatie**, zie [Anomalieën](anomalies.md). Een uitgestelde back-up stuurt één bericht als hij begint te wachten en één als het wachten eindigt, niet één per poging.

## Kanalen {#channels}

**Kanaal toevoegen** onderaan de kaart **Kanalen** biedt **Unraid-meldingen**, **ntfy**, **Gotify**, **Discord**, **Slack**, **Matrix**, **E-mail (SMTP)**, **Apprise**, **Webhook** en **Healthchecks**. Voeg er zoveel toe als je wilt, ook twee van dezelfde soort, zoals een ntfy-topic voor mislukkingen en een ander voor alles. Een nieuw kanaal begint uitgeschakeld, zodat het al is opgeslagen voordat je de velden invult.

Elk kanaal heeft:

- een eigen schakelaar, die het uitzet zonder dat de velden verloren gaan.
- **Melden**: **Nooit**, **Alleen bij mislukking** of **Bij succes en mislukking**. Een nieuw kanaal begint op **Alleen bij mislukking**. Een back-up die slaagde maar iets oversloeg, bereikt ook een kanaal dat alleen mislukkingen meldt.
- **Naam**, die je mag leeg laten en die in plaats van de soort verschijnt, handig als je twee kanalen van dezelfde soort hebt.
- **Test versturen**, dat een testbericht via dit kanaal stuurt met de waarden zoals ze in het formulier staan, opgeslagen of niet. De regel onder het kanaal zegt of de dienst het aannam, en zo niet, waarom.
- **Kanaal verwijderen**, de prullenbak ernaast.

Elk kanaal verstuurt zelfstandig met een eigen tijdslimiet, dus één host die nooit antwoordt, houdt de andere niet op. Mislukt een dienst op een manier die een nieuwe poging kan verhelpen, dan probeert het kanaal het tot drie keer. Een melding over een achterstand, het budget of een anomalie die geen kanaal aannam, wordt later opnieuw geprobeerd, en een wekelijkse samenvatting ook.

| Kanaal | Velden | Opmerkingen |
|---|---|---|
| **Unraid-meldingen** | geen | Stuurt naar het eigen meldingssysteem van Unraid, dat kan doorsturen naar Pushover, e-mail of Discord. Het loopt over de SSH-verbinding uit **Instellingen, Integraties, Host-SSH**, dus die sleutel moet zijn geautoriseerd. Op TrueNAS en andere hosts blijft het stil, en het kanaal zegt waarom. |
| **ntfy** | **Topic-URL**, **Toegangstoken (optioneel)** | Het adres eindigt op je topic. Een token is alleen nodig voor een beveiligd topic. |
| **Gotify** | **Server-URL**, **App-token**, **Prioriteit bij mislukking**, **Prioriteit bij succes** | Prioriteiten lopen van 0 tot 10 en beginnen op 8 voor mislukkingen en 5 voor successen. |
| **Discord** | **Webhook-URL** | Het adres van een webhook uit de instellingen van het Discord-kanaal. |
| **Slack** | **Webhook-URL** | Het adres van een inkomende webhook van je Slack-app. |
| **Matrix** | **Homeserver-URL**, **Toegangstoken**, **Room-ID** | Het account achter het token moet in de room zitten. |
| **E-mail (SMTP)** | **SMTP-host**, **Poort**, **Versleuteling**, **Gebruikersnaam**, **Wachtwoord**, **Afzenderadres**, **Ontvangeradres** | **Versleuteling** is **STARTTLS**, **TLS (implicit)** of **None**. |
| **Apprise** | **Apprise API-URL**, **Tags (optioneel)**, **Apprise-adressen (optioneel)** | Wijs het naar het eindpunt `/notify/<key>` van je Apprise-API-container om zijn 100+ diensten te bereiken, en tags kiezen een deel van de doelen van die sleutel. Of zet er Apprise-adressen in, één per regel, zoals `tgram://` voor Telegram of `pover://` voor Pushover, en laat de API-URL zonder sleutel op `/notify` eindigen. |
| **Webhook** | **Webhook-URL**, **Methode**, **Headers**, **Inhoud** | Een verzoek dat je zelf vormgeeft, zie [Webhook](#webhook). |
| **Healthchecks** | **Ping-URL** | Zie [Healthchecks](#healthchecks). |

Geheimen zoals de tokens, het SMTP-wachtwoord en de waarden van webhook-headers worden versleuteld opgeslagen, en de server geeft ze nooit terug aan de pagina. Onder een opgeslagen geheim staat in het veld **Opgeslagen. Laat het veld leeg om het te behouden.** Wijzig je waarheen een kanaal verstuurt, dan stuurt BombVault het oude geheim niet naar de nieuwe plek: het veld zegt **Het nieuwe adres wordt opgeslagen zodra je dit opnieuw invoert.**, en het nieuwe adres wordt opgeslagen zodra je het intypt.

Instellingen uit een versie van vóór de kanalenlijst worden bij de eerste start overgenomen, en er gaat niets verloren. Elk kanaal dat je had ingevuld, wordt een eigen item met de keuze bij **Melden** die je had. Een webhook in het formaat Discord, Slack, Gotify of ntfy wordt een kanaal van die soort, en een webhook in Generic JSON blijft `title`, `message` en `ok` posten via zijn **Inhoud**. De Unraid-schakelaar wordt een Unraid-kanaal, en de Healthchecks-URL met zijn uitzonderingen een Healthchecks-kanaal.

### Webhook {#webhook}

Een **Webhook**-kanaal stuurt één HTTP-verzoek naar elk adres dat je wilt. **Methode** is GET, POST of PUT. **Header toevoegen** voegt een regel toe onder **Headers**, en de waarden van headers worden als geheimen bewaard, dus een `Authorization`-header is daar veilig. **Inhoud** wordt precies zo verstuurd als je hem schrijft, met deze placeholders ingevuld:

| Placeholder | Wordt |
|---|---|
| `%%title%%` | de titel van het bericht |
| `%%body%%` | het bericht zelf |
| `%%ok%%` | `true`, of `false` bij een mislukking |
| `%%event%%` | wat er gebeurde, zoals `container.failed` |
| `%%severity%%` | `info`, `success`, `warning`, `failure` of `critical` |
| `%%importance%%` | het niveau dat Unraid ervoor gebruikt: `normal`, `warning` of `alert` |
| `%%link%%` | een link die bij het bericht hoort, leeg als er geen is |
| `%%key%%` | het onderdeel, zoals `container` of `VM` |
| `%%time%%` | de tijd in UTC, zoals `2026-11-09T03:00:00Z` |
| `%%source%%` | `BombVault` |

Placeholders werken ook in het adres en in de waarden van headers, en elke waarde wordt geëscaped voor de plek waar hij staat: als JSON wanneer de header `Content-Type` JSON noemt, en als queryparameter in het adres. Een nieuwe webhook begint met die header en post `{"title":"%%title%%","message":"%%body%%","ok":%%ok%%}`. Een naam die BombVault niet kent, blijft staan zoals je hem schreef, zodat een typfout in het bericht opvalt. Laat **Inhoud** leeg om niets mee te sturen.

## Healthchecks {#healthchecks}

Een dienst zoals [Healthchecks.io](https://healthchecks.io) slaat alarm als BombVault niet meer incheckt. Voeg een kanaal **Healthchecks** toe en plak de **Ping-URL** van de check, en BombVault pingt die tijdens de hele levenscyclus van elke back-up: bij de start, daarna bij succes of met `/fail`. Dit kanaal heeft geen keuze bij **Melden**, want een check heeft zijn succespings nodig om groen te blijven. Een onderdeel dat nergens heen een back-up kan maken, telt als mislukt, dus zijn check wordt rood in plaats van een succes te horen.

Onderdelen met dezelfde URL delen één check. Is een back-up van het ene mislukt, dan pingen de geslaagde back-ups van de andere ook `/fail` en noemen ze het, zodat de check rood blijft tot dat item weer een geslaagde back-up heeft of je de mislukking bevestigt onder **Aandacht nodig**.

Een onderdeel kan een eigen adres pingen: **Uitzondering** in het Healthchecks-kanaal voegt er een toe, en de regel zegt dan **Alle andere onderdelen pingen deze URL.** **Test versturen** schrijft een regel in het log van elke check, en de status blijft zoals hij is.

## Voorbeeld: Gotify op je telefoon {#example-gotify}

Je draait Gotify op `https://gotify.example.com` en wilt dat mislukkingen op je telefoon overgaan terwijl successen stil binnenkomen.

1. Open in Gotify **Apps**, maak een applicatie met de naam `BombVault` en kopieer haar token.
2. Open in BombVault **Instellingen, Meldingen** en schakel **Geplande runs samenvatten** in, zodat een nacht met 45 back-ups één bericht stuurt in plaats van 45.
3. Druk in **Kanalen** op **Kanaal toevoegen** en kies **Gotify**.
4. Vul `https://gotify.example.com` in als **Server-URL**. BombVault voegt zelf `/message` toe.
5. Plak het token in **App-token**.
6. Zet **Melden** op **Bij succes en mislukking**. Laat **Prioriteit bij mislukking** op 8 en **Prioriteit bij succes** op 5. De Android-app van Gotify behandelt 8 en hoger als hoog, dus alleen mislukkingen maken geluid.
7. Schakel het kanaal in en druk op **Test versturen**. De regel onder het kanaal zegt of Gotify het aannam, en een geweigerd token wordt als zodanig genoemd.

De volgende geplande run stuurt één samenvatting met prioriteit 5, of met 8 als er iets in mislukte.

## Voorbeeld: ntfy zonder account {#example-ntfy}

ntfy heeft voor een openbaar topic geen account nodig. Kies een topicnaam die moeilijk te raden is, want wie hem kent, kan hem lezen.

1. Druk in **Kanalen** op **Kanaal toevoegen** en kies **ntfy**.
2. Vul het adres van het topic in als **Topic-URL**, zoals `https://ntfy.sh/bombvault-tower-7f3k`, of een topic op je eigen ntfy-server. Laat **Toegangstoken (optioneel)** leeg voor een openbaar topic.
3. Abonneer je in de ntfy-app op hetzelfde topic, schakel het kanaal in en druk op **Test versturen**.

BombVault post het bericht als body met de titel in de header `Title`. Een mislukking gaat uit met prioriteit hoog en de tag `warning`, zodat ze in de app opvalt, en een VM die bevroren bleef, gaat uit als urgent.

## Als berichten niet aankomen {#troubleshooting}

Zie [Meldingen komen niet aan](troubleshooting.md#notifications-do-not-arrive) bij Problemen oplossen.
