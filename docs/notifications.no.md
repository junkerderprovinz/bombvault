# Varsler

BombVault kan si fra når en sikkerhetskopi feiler, når den lykkes, når en er forsinket, og en gang i uken om hvordan det har gått. Alt settes under **Innstillinger, Varsler**: hva som utløser en melding i kortet **Varsler**, hvor meldingene går i kortet **Kanaler**, og innsjekkingspingene i kortet **Healthchecks**.

![Innstillinger, Varsler med Gotify-kanalen utfylt.](assets/screenshots/notifications/gotify-dark.png#only-dark)
![Innstillinger, Varsler med Gotify-kanalen utfylt.](assets/screenshots/notifications/gotify-light.png#only-light)

## Hva som sender en melding {#what}

Kortet **Varsler** setter reglene for alle kanaler samtidig:

- **Varsle**: **Aldri**, **Kun ved feil** eller **Ved suksess og feil**, for alle kanalene nedenfor. En ny installasjon starter på **Kun ved feil**, så en kanal du fyller ut, sier fra når en sikkerhetskopi går i stykker.
- **Oppsummer planlagte kjøringer**: én melding per planlagte kjøring, for eksempel *42 av 45 lyktes*, i stedet for én per container eller VM. En sikkerhetskopi du starter for hånd, sender fortsatt sin egen melding. En frosset gjest, en mislykket oppbevaring eller et overskredet vekstbudsjett går ut med en gang, også med oppsummeringer på.
- **Varsle ved containeroppdatering**: én melding for hver container som **Oppdater containere etter en vellykket sikkerhetskopi** flyttet til et nyere image, så du kan sjekke at den fortsatt fungerer.
- **Varsle om forsinkede sikkerhetskopier**: én gang om dagen klokken 09:00 sjekker BombVault om en sikkerhetskopi som er slått på er forsinket, altså eldre enn dobbelt så lenge som tidsplanen tilsier, og sender én melding per tilfelle, med navnet på elementet som ble liggende etter. Den neste vellykkede sikkerhetskopien fjerner varselet.
- **Unraid-varsler**: sender til Unraids eget varslingssystem, som kan videresende til Pushover, e-post eller Discord. Det går over SSH-tilkoblingen fra **Innstillinger, Integrasjoner, Verts-SSH**, så nøkkelen må være autorisert der. På TrueNAS og andre verter er det av.
- **Ukentlig oppsummering**: én oppsummering i uken med antall kjøringer, nye sikkerhetskopidata, hvilke områder som er forsinket, om de eksterne kopiene er à jour og de vanligste feilene. Planen er en knapp som alle andre planer, og **Av** slår den av. Se [Tidsplaner](schedules.md#plans).

Hvilke avvik som sender en melding, settes under **Innstillinger, Verifisering**, se [Avvik](anomalies.md). En utsatt sikkerhetskopi sender én melding når den begynner å vente og én når ventetiden er over, ikke én per forsøk.

**Send test** nederst i kortet sender en testmelding gjennom hver kanal som er slått på, med verdiene slik de står i skjemaet, og nevner hver kanal som feilet. En Healthchecks-sjekk får en oppføring i loggen sin, som lar tilstanden dens være som den er.

## Kanaler {#channels}

Hver kanal har sin egen bryter i kortet **Kanaler**, og feltene vises når den er på. Hver kanal sender for seg med sin egen tidsgrense, så én vert som aldri svarer, holder ikke opp de andre. Et varsel om forsinkelse, budsjett eller avvik som ingen kanal tok imot, prøves på nytt, og det samme gjelder en ukentlig oppsummering. Meldingen for en enkelt sikkerhetskopi eller en oppsummering sendes én gang.

| Kanal | Felt | Merknader |
|---|---|---|
| **Webhook** | **Webhook-URL**, **Webhook-format** | Formater: **Generic JSON**, **Discord**, **Slack**, **Gotify** og **ntfy**. Generic JSON sender `title`, `message` og `ok`. |
| **Apprise** | **Apprise API-URL**, **Tagger (valgfritt)** | Pek den mot endepunktet `/notify/<key>` i Apprise API-containeren din for å nå de over 100 tjenestene. Tagger velger en delmengde av målene til den nøkkelen. |
| **Matrix** | **Homeserver-URL**, **Tilgangstoken**, **Rom-ID** | Tokenet blir på serveren og vises aldri igjen. |
| **Gotify** | **Server-URL**, **App-token**, **Prioritet ved feil**, **Prioritet ved suksess** | Prioritetene går fra 0 til 10 og starter på 8 for feil og 5 for suksess. |
| **E-post (SMTP)** | **SMTP-host**, **Port**, **Kryptering**, **Brukernavn**, **Passord**, **Fra-adresse**, **Til-adresse** | **Kryptering** er **STARTTLS**, **TLS (implicit)** eller **None**. |

Hemmeligheter som Matrix-tokenet, Gotify-app-tokenet og SMTP-passordet lagres kryptert. Under en lagret hemmelighet står det **Lagret. La feltet stå tomt for å beholde det.** Når du endrer serverens adresse, sender ikke BombVault den gamle hemmeligheten til det nye stedet: feltet ber om den på nytt, og den nye adressen lagres når du har skrevet den inn.

## Healthchecks {#healthchecks}

En tjeneste som [Healthchecks.io](https://healthchecks.io) slår alarm når BombVault slutter å sjekke inn. Lim inn sjekkens **Ping-URL**, så pinger BombVault den gjennom hele forløpet til hver sikkerhetskopi: ved start, deretter ved suksess eller med `/fail`. Det skjer så lenge en URL er satt, uansett hva **Varsle** sier, så sjekken holder seg grønn ved suksess selv om du bare får melding ved feil. Et område som ikke har noe sted å ta sikkerhetskopi til, regnes som feilet, så sjekken blir rød i stedet for å få høre om en suksess.

Områder med samme URL deler én sjekk. Når en sikkerhetskopi av ett av dem har feilet, pinger også de vellykkede sikkerhetskopiene av de andre `/fail` og nevner det, så sjekken forblir rød til det elementet har en vellykket sikkerhetskopi igjen eller du kvitterer for feilen under **Krever oppmerksomhet**.

Et område kan pinge en egen adresse: **Unntak** i kortet legger til en, og linjen sier **Alle andre områder pinger denne URL-en.**

## Eksempel: Gotify på telefonen {#example-gotify}

Du kjører Gotify på `https://gotify.example.com` og vil at feil skal ringe på telefonen mens suksesser kommer stille.

1. Åpne **Apps** i Gotify, lag et program som heter `BombVault` og kopier tokenet.
2. Åpne **Innstillinger, Varsler** i BombVault. I **Varsler** setter du **Varsle** til **Ved suksess og feil** og slår på **Oppsummer planlagte kjøringer**, så en natt med 45 sikkerhetskopier sender én melding i stedet for 45.
3. Slå på **Gotify** i **Kanaler**.
4. Skriv inn `https://gotify.example.com` som **Server-URL**. BombVault legger til `/message` selv.
5. Lim tokenet inn i **App-token**.
6. La **Prioritet ved feil** stå på 8 og **Prioritet ved suksess** på 5. Gotifys Android-app regner 8 og høyere som høy, så bare feil lager lyd.
7. Trykk på **Send test**. Linjen under kortet sier om Gotify tok imot den, og et avvist token oppgis som det.

Den neste planlagte kjøringen sender én oppsummering med prioritet 5, eller 8 hvis noe i den feilet.

## Eksempel: ntfy som webhook {#example-ntfy}

ntfy trenger ingen konto for et offentlig emne. Velg et emnenavn som er vanskelig å gjette, for alle som kjenner det, kan lese det.

1. Slå på **Webhook** i **Kanaler**.
2. Skriv inn emnets adresse som **Webhook-URL**, for eksempel `https://ntfy.sh/bombvault-tower-7f3k`, eller adressen til din egen ntfy-server.
3. Sett **Webhook-format** til **ntfy**.
4. Abonner på det samme emnet i ntfy-appen og trykk på **Send test**.

BombVault sender meldingen som innhold med tittelen i `X-Title`. En feil går ut med høy prioritet og taggen `warning`, så den skiller seg ut i appen.

## Når meldinger ikke kommer fram {#troubleshooting}

Se [Varsler kommer ikke fram](troubleshooting.md#notifications-do-not-arrive) i Feilsøking.
