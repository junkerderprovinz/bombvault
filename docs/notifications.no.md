# Varsler

BombVault kan si fra når en sikkerhetskopi feiler, når den lykkes, når en er forsinket, og en gang i uken om hvordan det har gått. Alt settes under **Innstillinger, Varsler**: hva BombVault sender utover enkelte sikkerhetskopier i kortet **Varsler**, og hvor meldingene går i kortet **Kanaler**. Hver kanal bestemmer selv når den sier fra, og en Healthchecks-sjekk er også en kanal.

![Innstillinger, Varsler med Gotify-kanalen utfylt.](assets/screenshots/notifications/gotify-dark.png#only-dark)
![Innstillinger, Varsler med Gotify-kanalen utfylt.](assets/screenshots/notifications/gotify-light.png#only-light)

## Hva som sender en melding {#what}

Bryterne i kortet **Varsler** gjelder for alle kanaler:

- **Oppsummer planlagte kjøringer**: én melding per planlagte kjøring, for eksempel *42 av 45 lyktes*, i stedet for én per container eller VM. En sikkerhetskopi du starter for hånd, sender fortsatt sin egen melding. En frosset gjest, en mislykket oppbevaring eller et overskredet vekstbudsjett går ut med en gang, også med oppsummeringer på.
- **Varsle ved containeroppdatering**: én melding for hver container som **Oppdater containere etter en vellykket sikkerhetskopi** flyttet til et nyere image, så du kan sjekke at den fortsatt fungerer.
- **Varsle om forsinkede sikkerhetskopier**: én gang om dagen klokken 09:00 sjekker BombVault om en sikkerhetskopi som er slått på er forsinket, altså eldre enn dobbelt så lenge som tidsplanen tilsier, og sender én melding per tilfelle, med navnet på elementet som ble liggende etter. Den neste vellykkede sikkerhetskopien fjerner varselet.
- **Ukentlig oppsummering**: én oppsummering i uken med antall kjøringer, nye sikkerhetskopidata, hvilke områder som er forsinket, om de eksterne kopiene er à jour og de vanligste feilene. Planen er en knapp som alle andre planer, og **Av** slår den av. Se [Tidsplaner](schedules.md#plans).

Om en kanal også får høre om suksesser, settes på kanalen under **Varsle**. Hvilke avvik som sender en melding, settes under **Innstillinger, Verifisering**, se [Avvik](anomalies.md). En utsatt sikkerhetskopi sender én melding når den begynner å vente og én når ventetiden er over, ikke én per forsøk.

## Kanaler {#channels}

**Legg til kanal** nederst i kortet **Kanaler** tilbyr **Unraid-varsler**, **ntfy**, **Gotify**, **Discord**, **Slack**, **Matrix**, **E-post (SMTP)**, **Apprise**, **Webhook** og **Healthchecks**. Legg til så mange du vil, også to av samme slag, for eksempel ett ntfy-emne for feil og et annet for alt. En ny kanal starter slått av, så den er lagret før feltene er fylt ut.

Hver kanal har:

- sin egen bryter, som slår den av uten at feltene går tapt.
- **Varsle**: **Aldri**, **Kun ved feil** eller **Ved suksess og feil**. En ny kanal starter på **Kun ved feil**. En sikkerhetskopi som lyktes, men utelot noe, når også fram til en kanal som bare melder feil.
- **Navn**, som er valgfritt og står i stedet for typen, nyttig når du har to av samme slag.
- **Send test**, som sender en testmelding gjennom akkurat denne kanalen med verdiene slik de står i skjemaet, lagret eller ikke. Linjen under kanalen sier om tjenesten tok imot den, og hvis ikke, hvorfor.
- **Fjern kanal**, søppelbøtta ved siden av.

Hver kanal sender for seg med sin egen tidsgrense, så én vert som aldri svarer, holder ikke opp de andre. Når en tjeneste feiler på en måte som et nytt forsøk kan rette opp, prøver kanalen opptil tre ganger. Et varsel om forsinkelse, budsjett eller avvik som ingen kanal tok imot, prøves på nytt senere, og det samme gjelder en ukentlig oppsummering.

| Kanal | Felt | Merknader |
|---|---|---|
| **Unraid-varsler** | ingen | Sender til Unraids eget varslingssystem, som kan videresende til Pushover, e-post eller Discord. Det går over SSH-tilkoblingen fra **Innstillinger, Integrasjoner, Verts-SSH**, så nøkkelen må være autorisert der. På TrueNAS og andre verter forblir den stille, og kanalen sier hvorfor. |
| **ntfy** | **Emne-URL**, **Tilgangstoken (valgfritt)** | Adressen slutter med emnet ditt. Et token trengs bare for et beskyttet emne. |
| **Gotify** | **Server-URL**, **App-token**, **Prioritet ved feil**, **Prioritet ved suksess** | Prioritetene går fra 0 til 10 og starter på 8 for feil og 5 for suksess. |
| **Discord** | **Webhook-URL** | Adressen til en webhook fra innstillingene til Discord-kanalen. |
| **Slack** | **Webhook-URL** | Adressen til en innkommende webhook i Slack-appen din. |
| **Matrix** | **Homeserver-URL**, **Tilgangstoken**, **Rom-ID** | Kontoen bak tokenet må være med i rommet. |
| **E-post (SMTP)** | **SMTP-host**, **Port**, **Kryptering**, **Brukernavn**, **Passord**, **Fra-adresse**, **Til-adresse** | **Kryptering** er **STARTTLS**, **TLS (implicit)** eller **None**. |
| **Apprise** | **Apprise API-URL**, **Tagger (valgfritt)**, **Apprise-adresser (valgfritt)** | Pek den mot endepunktet `/notify/<key>` i Apprise API-containeren din for å nå de over 100 tjenestene, og tagger velger en delmengde av målene til den nøkkelen. Eller før opp Apprise-adresser, én per linje, for eksempel `tgram://` for Telegram eller `pover://` for Pushover, og la API-URL-en slutte på `/notify` uten nøkkel. |
| **Webhook** | **Webhook-URL**, **Metode**, **HTTP-headere**, **Innhold** | En forespørsel du former selv, se [Webhook](#webhook). |
| **Healthchecks** | **Ping-URL** | Se [Healthchecks](#healthchecks). |

Hemmeligheter som tokenene, SMTP-passordet og verdiene i webhookens HTTP-headere lagres kryptert, og serveren gir dem aldri tilbake til siden. Under en lagret hemmelighet står det **Lagret. La feltet stå tomt for å beholde det.** Når du endrer hvor en kanal sender til, sender ikke BombVault den gamle hemmeligheten til det nye stedet: feltet sier **Den nye adressen lagres når du skriver inn dette igjen.**, og den nye adressen lagres så snart du skriver den inn.

Innstillinger fra en versjon før kanallisten blir overført ved første oppstart, og ingenting går tapt. Hver kanal du hadde fylt ut, blir et eget element med valget du hadde under **Varsle**. En webhook satt til formatet Discord, Slack, Gotify eller ntfy blir en kanal av den typen, og en Generic JSON-webhook fortsetter å sende `title`, `message` og `ok` gjennom **Innhold**. Unraid-bryteren blir en Unraid-kanal, og Healthchecks-URL-en med unntakene sine blir en Healthchecks-kanal.

### Webhook {#webhook}

En **Webhook**-kanal sender én HTTP-forespørsel til en hvilken som helst adresse. **Metode** er GET, POST eller PUT. **Legg til header** legger til en rad under **HTTP-headere**, og verdiene der lagres som hemmeligheter, så en `Authorization`-header er trygg der. **Innhold** sendes akkurat slik du skriver det, med disse plassholderne fylt inn:

| Plassholder | Blir til |
|---|---|
| `%%title%%` | tittelen på meldingen |
| `%%body%%` | selve meldingen |
| `%%ok%%` | `true`, eller `false` ved feil |
| `%%event%%` | hva som skjedde, for eksempel `container.failed` |
| `%%severity%%` | `info`, `success`, `warning`, `failure` eller `critical` |
| `%%importance%%` | Unraids nivå for den: `normal`, `warning` eller `alert` |
| `%%link%%` | en lenke som hører til meldingen, tom når det ikke finnes noen |
| `%%key%%` | området, for eksempel `container` eller `VM` |
| `%%time%%` | tiden i UTC, for eksempel `2026-11-09T03:00:00Z` |
| `%%source%%` | `BombVault` |

Plassholdere virker også i adressen og i headerverdier, og hver verdi escapes etter hvor den står: som JSON når headeren `Content-Type` sier JSON, og som spørringsverdi i adressen. En ny webhook starter med den headeren og sender `{"title":"%%title%%","message":"%%body%%","ok":%%ok%%}`. Et navn BombVault ikke kjenner, blir stående som det er skrevet, så en skrivefeil synes i meldingen. La **Innhold** stå tomt for å ikke sende noe innhold.

## Healthchecks {#healthchecks}

En tjeneste som [Healthchecks.io](https://healthchecks.io) slår alarm når BombVault slutter å sjekke inn. Legg til en **Healthchecks**-kanal og lim inn sjekkens **Ping-URL**, så pinger BombVault den gjennom hele forløpet til hver sikkerhetskopi: ved start, deretter ved suksess eller med `/fail`. Denne kanalen har ikke noe **Varsle**-valg, fordi en sjekk trenger pingene for suksess for å holde seg grønn. Et område som ikke har noe sted å ta sikkerhetskopi til, regnes som feilet, så sjekken blir rød i stedet for å få høre om en suksess.

Områder med samme URL deler én sjekk. Når en sikkerhetskopi av ett av dem har feilet, pinger også de vellykkede sikkerhetskopiene av de andre `/fail` og nevner det, så sjekken forblir rød til det elementet har en vellykket sikkerhetskopi igjen eller du kvitterer for feilen under **Krever oppmerksomhet**.

Et område kan pinge en egen adresse: **Unntak** i Healthchecks-kanalen legger til en, og linjen sier da **Alle andre områder pinger denne URL-en.** **Send test** skriver en oppføring i loggen til hver sjekk, som lar tilstanden være som den er.

## Eksempel: Gotify på telefonen {#example-gotify}

Du kjører Gotify på `https://gotify.example.com` og vil at feil skal ringe på telefonen mens suksesser kommer stille.

1. Åpne **Apps** i Gotify, lag et program som heter `BombVault` og kopier tokenet.
2. Åpne **Innstillinger, Varsler** i BombVault og slå på **Oppsummer planlagte kjøringer**, så en natt med 45 sikkerhetskopier sender én melding i stedet for 45.
3. Trykk på **Legg til kanal** i **Kanaler** og velg **Gotify**.
4. Skriv inn `https://gotify.example.com` som **Server-URL**. BombVault legger til `/message` selv.
5. Lim tokenet inn i **App-token**.
6. Sett **Varsle** til **Ved suksess og feil**. La **Prioritet ved feil** stå på 8 og **Prioritet ved suksess** på 5. Gotifys Android-app regner 8 og høyere som høy, så bare feil lager lyd.
7. Slå på kanalen og trykk på **Send test**. Linjen under kanalen sier om Gotify tok imot den, og et avvist token oppgis som det.

Den neste planlagte kjøringen sender én oppsummering med prioritet 5, eller 8 hvis noe i den feilet.

## Eksempel: ntfy uten konto {#example-ntfy}

ntfy trenger ingen konto for et offentlig emne. Velg et emnenavn som er vanskelig å gjette, for alle som kjenner det, kan lese det.

1. Trykk på **Legg til kanal** i **Kanaler** og velg **ntfy**.
2. Skriv inn emnets adresse som **Emne-URL**, for eksempel `https://ntfy.sh/bombvault-tower-7f3k`, eller et emne på din egen ntfy-server. La **Tilgangstoken (valgfritt)** stå tomt for et offentlig emne.
3. Abonner på det samme emnet i ntfy-appen, slå på kanalen og trykk på **Send test**.

BombVault sender meldingen som innhold med tittelen i headeren `Title`. En feil går ut med høy prioritet og taggen `warning`, så den skiller seg ut i appen, og en VM som blir stående frosset, går ut som hastemelding.

## Når meldinger ikke kommer fram {#troubleshooting}

Se [Varsler kommer ikke fram](troubleshooting.md#notifications-do-not-arrive) i Feilsøking.
