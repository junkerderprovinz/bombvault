# Meldingen

BombVault kan je laten weten wanneer een back-up mislukt, wanneer er een slaagt, wanneer er een achterstallig is, en één keer per week hoe het ging. Alles stel je in onder **Instellingen, Meldingen**: wat een bericht veroorzaakt in de kaart **Meldingen**, waarheen berichten gaan in de kaart **Kanalen** en de check-in-pings in de kaart **Healthchecks**.

![Instellingen, Meldingen met het kanaal Gotify ingevuld.](assets/screenshots/notifications/gotify-dark.png#only-dark)
![Instellingen, Meldingen met het kanaal Gotify ingevuld.](assets/screenshots/notifications/gotify-light.png#only-light)

## Wat een bericht veroorzaakt {#what}

De kaart **Meldingen** bepaalt de regels voor alle kanalen tegelijk:

- **Melden**: **Nooit**, **Alleen bij mislukking** of **Bij succes en mislukking**, voor elk kanaal hieronder.
- **Geplande runs samenvatten**: één bericht per geplande run, zoals *42 van de 45 geslaagd*, in plaats van één per container of VM. Een back-up die je met de hand start, stuurt nog steeds een eigen bericht. Een bevroren gast, een mislukte bewaarronde of een overschreden groeibudget gaat meteen de deur uit, ook met samenvattingen aan.
- **Melden bij containerupdate**: één bericht voor elke container die **Containers bijwerken na een geslaagde back-up** naar een nieuwere image heeft verplaatst, zodat je kunt nagaan dat hij nog werkt.
- **Achterstallige back-ups melden**: eenmaal per dag om 09:00 controleert BombVault of een ingeschakelde back-up achterstallig is, dus ouder dan twee keer zijn planning, en stuurt het per geval één bericht. De volgende geslaagde back-up ruimt het op.
- **Unraid-meldingen**: stuurt naar het eigen meldingssysteem van Unraid, dat kan doorsturen naar Pushover, e-mail of Discord. Het loopt over de SSH-verbinding uit **Instellingen, Integraties, Host-SSH**, dus die sleutel moet zijn geautoriseerd. Op TrueNAS en andere hosts blijft het uit.
- **Wekelijkse samenvatting**: één samenvatting per week met het aantal runs, nieuwe back-updata, of de off-site-kopieën actueel zijn en de belangrijkste mislukkingen. De planning ervan is een knop zoals elke andere planning, en **Uit** schakelt haar uit. Zie [Planningen](schedules.md#plans).

Welke anomalieën een bericht sturen, stel je in onder **Instellingen, Verificatie**, zie [Anomalieën](anomalies.md). Een uitgestelde back-up stuurt één bericht als hij begint te wachten en één als het wachten eindigt, niet één per poging.

**Test versturen** onderaan de kaart stuurt een testbericht via elk kanaal dat aanstaat, met de waarden zoals ze in het formulier staan.

## Kanalen {#channels}

Elk kanaal heeft een eigen schakelaar in de kaart **Kanalen**, en zijn velden verschijnen zodra hij aanstaat. Elk kanaal verstuurt zelfstandig met een eigen tijdslimiet, dus één host die nooit antwoordt, houdt de andere niet op, en een bericht dat geen kanaal aannam, wordt opnieuw geprobeerd.

| Kanaal | Velden | Opmerkingen |
|---|---|---|
| **Webhook** | **Webhook-URL**, **Webhook-formaat** | Formaten: **Generic JSON**, **Discord**, **Slack**, **Gotify** en **ntfy**. Generic JSON post `title`, `message` en `ok`. |
| **Apprise** | **Apprise API-URL**, **Tags (optioneel)** | Wijs het naar het eindpunt `/notify/<key>` van je Apprise-API-container om zijn 100+ diensten te bereiken. Tags kiezen een deel van de doelen van die sleutel. |
| **Matrix** | **Homeserver-URL**, **Toegangstoken**, **Room-ID** | Het token blijft op de server en wordt nooit meer getoond. |
| **Gotify** | **Server-URL**, **App-token**, **Prioriteit bij mislukking**, **Prioriteit bij succes** | Prioriteiten lopen van 0 tot 10 en beginnen op 8 voor mislukkingen en 5 voor successen. |
| **E-mail (SMTP)** | **SMTP-host**, **Poort**, **Versleuteling**, **Gebruikersnaam**, **Wachtwoord**, **Afzenderadres**, **Ontvangeradres** | **Versleuteling** is **STARTTLS**, **TLS (implicit)** of **None**. |

Geheimen zoals het Matrix-token, het Gotify-app-token en het SMTP-wachtwoord worden versleuteld opgeslagen. Onder een opgeslagen geheim staat in het veld **Opgeslagen. Laat het veld leeg om het te behouden.** Wijzig je het adres van de server, dan stuurt BombVault het oude geheim niet naar de nieuwe plek: het veld vraagt er opnieuw om, en het nieuwe adres wordt opgeslagen zodra je het intypt.

## Healthchecks {#healthchecks}

Een dienst zoals [Healthchecks.io](https://healthchecks.io) slaat alarm als BombVault niet meer incheckt. Plak de **Ping-URL** van de check, en BombVault pingt die tijdens de hele levenscyclus van elke back-up: bij de start, daarna bij succes of met `/fail`. Dat gebeurt zodra er een URL is ingesteld, wat **Melden** ook zegt, dus de check blijft bij succes groen, ook met berichten alleen bij mislukking.

Een onderdeel kan een eigen adres pingen: **Uitzondering** in de kaart voegt er een toe, en de regel zegt **Alle andere onderdelen pingen deze URL.**

## Voorbeeld: Gotify op je telefoon {#example-gotify}

Je draait Gotify op `https://gotify.example.com` en wilt dat mislukkingen op je telefoon overgaan terwijl successen stil binnenkomen.

1. Open in Gotify **Apps**, maak een applicatie met de naam `BombVault` en kopieer haar token.
2. Open in BombVault **Instellingen, Meldingen**. Zet in **Meldingen** **Melden** op **Bij succes en mislukking** en schakel **Geplande runs samenvatten** in, zodat een nacht met 45 back-ups één bericht stuurt in plaats van 45.
3. Schakel in **Kanalen** **Gotify** in.
4. Vul `https://gotify.example.com` in als **Server-URL**. BombVault voegt zelf `/message` toe.
5. Plak het token in **App-token**.
6. Laat **Prioriteit bij mislukking** op 8 en **Prioriteit bij succes** op 5. De Android-app van Gotify behandelt 8 en hoger als hoog, dus alleen mislukkingen maken geluid.
7. Druk op **Test versturen**. De regel onder de kaart zegt of Gotify het aannam, en een geweigerd token wordt als zodanig genoemd.

De volgende geplande run stuurt één samenvatting met prioriteit 5, of met 8 als er iets in mislukte.

## Voorbeeld: ntfy als webhook {#example-ntfy}

ntfy heeft voor een openbaar topic geen account nodig. Kies een topicnaam die moeilijk te raden is, want wie hem kent, kan hem lezen.

1. Schakel in **Kanalen** **Webhook** in.
2. Vul het adres van het topic in als **Webhook-URL**, zoals `https://ntfy.sh/bombvault-tower-7f3k`, of het adres van je eigen ntfy-server.
3. Zet **Webhook-formaat** op **ntfy**.
4. Abonneer je in de ntfy-app op hetzelfde topic en druk op **Test versturen**.

BombVault post het bericht als body met de titel in `X-Title`. Een mislukking gaat uit met prioriteit hoog en de tag `warning`, zodat ze in de app opvalt.

## Als berichten niet aankomen {#troubleshooting}

Zie [Meldingen komen niet aan](troubleshooting.md#notifications-do-not-arrive) bij Problemen oplossen.
