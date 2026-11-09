# Notifikationer

BombVault kan give dig besked, når en sikkerhedskopiering mislykkes, når den lykkes, når en er forsinket, og én gang om ugen, hvordan det er gået. Alt indstilles under **Indstillinger, Notifikationer**: hvad BombVault sender ud over de enkelte sikkerhedskopieringer, på kortet **Notifikationer**, og hvor beskederne sendes hen, på kortet **Kanaler**. Hver kanal bestemmer selv, hvornår den siger til, og et Healthchecks-tjek er også en kanal.

![Indstillinger, Notifikationer med Gotify-kanalen udfyldt.](assets/screenshots/notifications/gotify-dark.png#only-dark)
![Indstillinger, Notifikationer med Gotify-kanalen udfyldt.](assets/screenshots/notifications/gotify-light.png#only-light)

## Hvad der sender en besked {#what}

Kontakterne på kortet **Notifikationer** gælder for alle kanaler:

- **Opsummer planlagte kørsler**: én besked pr. planlagt kørsel, for eksempel *42 af 45 lykkedes*, i stedet for én pr. container eller VM. En sikkerhedskopiering, du starter i hånden, sender stadig sin egen besked. En frosset gæst, en mislykket opbevaring eller et overskredet vækstbudget går ud med det samme, selv når opsummeringer er slået til.
- **Giv besked ved containeropdatering**: én besked for hver container, som **Opdater containere efter en vellykket sikkerhedskopiering** har flyttet til et nyere image, så du kan se efter, at den stadig virker.
- **Meld forsinkede sikkerhedskopier**: én gang om dagen kl. 09:00 tjekker BombVault, om en aktiveret sikkerhedskopiering er forsinket, altså ældre end dobbelt så lang tid som sin tidsplan, og sender én besked pr. tilfælde, der nævner den post, der er kommet bagud. Den næste vellykkede sikkerhedskopiering rydder den.
- **Ugentlig oversigt**: én opsummering om ugen med antal kørsler, nye sikkerhedskopidata, hvilke områder der er forsinkede, om off-site-kopierne er aktuelle, og de hyppigste fejl. Dens plan er en knap som alle andre planer, og **Fra** slår den fra. Se [Tidsplaner](schedules.md#plans).

Om en kanal også hører om succeser, indstiller du på kanalen under **Underret**. Hvilke afvigelser der sender en besked, indstiller du under **Indstillinger, Verificering**, se [Afvigelser](anomalies.md). En udskudt sikkerhedskopiering sender én besked, når den begynder at vente, og én, når ventetiden slutter, ikke én pr. forsøg.

## Kanaler {#channels}

**Tilføj kanal** nederst på kortet **Kanaler** tilbyder **Unraid-notifikationer**, **ntfy**, **Gotify**, **Discord**, **Slack**, **Matrix**, **E-mail (SMTP)**, **Apprise**, **Webhook** og **Healthchecks**. Tilføj så mange, du vil, også to af samme slags, for eksempel ét ntfy-emne til fejl og et andet til alt. En ny kanal starter slået fra, så den er gemt, før dens felter er udfyldt.

Hver kanal har:

- sin egen kontakt, der slår den fra uden at miste dens felter.
- **Underret**: **Aldrig**, **Kun ved fejl** eller **Ved succes og fejl**. En ny kanal starter på **Kun ved fejl**. En sikkerhedskopiering, der lykkedes, men udelod noget, når også frem til en kanal, der kun melder fejl.
- **Navn**, som er valgfrit og står i stedet for typen, praktisk når du har to af samme slags.
- **Send test**, der sender en testbesked gennem netop denne kanal med værdierne, som de står i formularen, gemt eller ej. Linjen under kanalen siger, om tjenesten tog imod den, og hvis ikke, hvorfor.
- **Fjern kanal**, skraldespanden ved siden af.

Hver kanal sender for sig selv med sin egen tidsgrænse, så en vært, der aldrig svarer, ikke holder de andre op. Når en tjeneste fejler på en måde, som et nyt forsøg kan klare, prøver kanalen op til tre gange. En advarsel om forsinkelse, budget eller en anomali, som ingen kanal tog imod, prøves igen senere, og det samme gør en ugentlig oversigt.

| Kanal | Felter | Bemærkninger |
|---|---|---|
| **Unraid-notifikationer** | ingen | Sender til Unraids eget notifikationssystem, som kan videresende til Pushover, e-mail eller Discord. Det kører over SSH-forbindelsen fra **Indstillinger, Integrationer, Værts-SSH**, så den nøgle skal være godkendt. På TrueNAS og andre værter forbliver den tavs, og kanalen siger hvorfor. |
| **ntfy** | **Emne-URL**, **Adgangstoken (valgfrit)** | Adressen ender på dit emne. Et token skal kun bruges til et beskyttet emne. |
| **Gotify** | **Server-URL**, **App-token**, **Prioritet ved fejl**, **Prioritet ved succes** | Prioriteterne går fra 0 til 10 og starter på 8 for fejl og 5 for succes. |
| **Discord** | **Webhook-URL** | Adressen på en webhook fra Discord-kanalens indstillinger. |
| **Slack** | **Webhook-URL** | Adressen på en indgående webhook i din Slack-app. |
| **Matrix** | **Homeserver-URL**, **Adgangstoken**, **Rum-ID** | Kontoen bag tokenet skal være med i rummet. |
| **E-mail (SMTP)** | **SMTP-vært**, **Port**, **Kryptering**, **Brugernavn**, **Adgangskode**, **Fra-adresse**, **Til-adresse** | **Kryptering** er **STARTTLS**, **TLS (implicit)** eller **None**. |
| **Apprise** | **Apprise API-URL**, **Tags (valgfrit)**, **Apprise-adresser (valgfrit)** | Peg på `/notify/<key>`-endpointet i din Apprise API-container for at nå dens 100+ tjenester, og tags vælger en delmængde af den nøgles mål. Eller skriv Apprise-adresser, én pr. linje, for eksempel `tgram://` til Telegram eller `pover://` til Pushover, og lad API-URL'en ende på `/notify` uden nøgle. |
| **Webhook** | **Webhook-URL**, **Metode**, **Headere**, **Indhold** | En forespørgsel, du selv former, se [Webhook](#webhook). |
| **Healthchecks** | **Ping-URL** | Se [Healthchecks](#healthchecks). |

Hemmeligheder som tokenerne, SMTP-adgangskoden og værdierne i webhook-headere gemmes krypteret, og serveren sender dem aldrig tilbage til siden. Under en gemt hemmelighed står der i feltet **Gemt. Lad feltet stå tomt for at beholde det.** Når du ændrer, hvor en kanal sender hen, sender BombVault ikke den gamle hemmelighed til det nye sted: Feltet siger **Den nye adresse gemmes, når du indtaster dette igen.**, og den nye adresse gemmes, så snart du skriver den.

Indstillinger fra en version før kanallisten overføres ved første start, og intet går tabt. Hver kanal, du havde udfyldt, bliver sin egen post med det valg under **Underret**, du havde. En webhook sat til formatet Discord, Slack, Gotify eller ntfy bliver en kanal af den slags, og en Generic JSON-webhook bliver ved med at sende `title`, `message` og `ok` gennem sit **Indhold**. Unraid-kontakten bliver en Unraid-kanal, og Healthchecks-URL'en med sine undtagelser bliver en Healthchecks-kanal.

### Webhook {#webhook}

En **Webhook**-kanal sender én HTTP-forespørgsel til en hvilken som helst adresse. **Metode** er GET, POST eller PUT. **Tilføj header** tilføjer en række under **Headere**, og værdierne i headerne gemmes som hemmeligheder, så en `Authorization`-header er sikker der. **Indhold** sendes præcis, som du skriver det, med disse pladsholdere udfyldt:

| Pladsholder | Bliver til |
|---|---|
| `%%title%%` | beskedens titel |
| `%%body%%` | selve beskeden |
| `%%ok%%` | `true`, eller `false` ved en fejl |
| `%%event%%` | hvad der skete, for eksempel `container.failed` |
| `%%severity%%` | `info`, `success`, `warning`, `failure` eller `critical` |
| `%%importance%%` | Unraids niveau for den: `normal`, `warning` eller `alert` |
| `%%link%%` | et link, der hører til beskeden, tomt, når der ikke er noget |
| `%%key%%` | området, for eksempel `container` eller `VM` |
| `%%time%%` | tidspunktet i UTC, for eksempel `2026-11-09T03:00:00Z` |
| `%%source%%` | `BombVault` |

Pladsholdere virker også i adressen og i headerværdier, og hver værdi escapes efter, hvor den står: som JSON, når headeren `Content-Type` siger JSON, og som forespørgselsværdi i adressen. En ny webhook starter med den header og sender `{"title":"%%title%%","message":"%%body%%","ok":%%ok%%}`. Et navn, BombVault ikke kender, bliver stående, som det er skrevet, så en tastefejl kan ses i beskeden. Lad **Indhold** stå tomt for ikke at sende noget indhold.

## Healthchecks {#healthchecks}

En tjeneste som [Healthchecks.io](https://healthchecks.io) slår alarm, når BombVault holder op med at tjekke ind. Tilføj en **Healthchecks**-kanal, og indsæt tjekkets **Ping-URL**, så pinger BombVault den under hele forløbet af hver sikkerhedskopiering: ved start, derefter ved succes eller med `/fail`. Denne kanal har ikke noget valg under **Underret**, fordi et tjek har brug for sine succespings for at forblive grønt. Et område uden et sted at sikkerhedskopiere til tæller som mislykket, så dets tjek bliver rødt i stedet for at høre om en succes.

Områder på samme URL deler ét tjek. Er en sikkerhedskopiering af ét af dem mislykkedes, pinger de gode sikkerhedskopieringer af de andre også `/fail` og nævner det, så tjekket forbliver rødt, indtil den post har en god sikkerhedskopi igen, eller du kvitterer for fejlen under **Kræver opmærksomhed**.

Et område kan pinge en adresse for sig selv: **Undtagelse** i Healthchecks-kanalen tilføjer en, og linjen siger så **Alle andre områder pinger denne URL.** **Send test** skriver en linje i loggen for hvert tjek, og tjekkets tilstand bliver, som den er.

## Eksempel: Gotify på din telefon {#example-gotify}

Du kører Gotify på `https://gotify.example.com` og vil have, at fejl ringer på din telefon, mens succeser kommer stille.

1. Åbn **Apps** i Gotify, opret et program kaldet `BombVault`, og kopiér dets token.
2. Åbn **Indstillinger, Notifikationer** i BombVault, og slå **Opsummer planlagte kørsler** til, så en nat med 45 sikkerhedskopieringer sender én besked i stedet for 45.
3. Tryk på **Tilføj kanal** under **Kanaler**, og vælg **Gotify**.
4. Skriv `https://gotify.example.com` som **Server-URL**. BombVault tilføjer selv `/message`.
5. Indsæt tokenet i **App-token**.
6. Sæt **Underret** til **Ved succes og fejl**. Lad **Prioritet ved fejl** stå på 8 og **Prioritet ved succes** på 5. Gotifys Android-app behandler 8 og opefter som høj, så kun fejl giver lyd.
7. Slå kanalen til, og tryk på **Send test**. Linjen under kanalen siger, om Gotify tog imod den, og et afvist token nævnes som sådan.

Den næste planlagte kørsel sender én opsummering med prioritet 5, eller 8, hvis noget i den mislykkedes.

## Eksempel: ntfy uden konto {#example-ntfy}

ntfy kræver ingen konto til et offentligt emne. Vælg et emnenavn, der er svært at gætte, for alle, der kender det, kan læse med.

1. Tryk på **Tilføj kanal** under **Kanaler**, og vælg **ntfy**.
2. Skriv emnets adresse som **Emne-URL**, for eksempel `https://ntfy.sh/bombvault-tower-7f3k`, eller et emne på din egen ntfy-server. Lad **Adgangstoken (valgfrit)** stå tomt til et offentligt emne.
3. Abonnér på det samme emne i ntfy-appen, slå kanalen til, og tryk på **Send test**.

BombVault sender beskeden som brødtekst med titlen i headeren `Title`. En fejl går ud med høj prioritet og tagget `warning`, så den skiller sig ud i appen, og en VM, der forbliver frosset, går ud som akut.

## Når beskeder ikke kommer frem {#troubleshooting}

Se [Notifikationer kommer ikke frem](troubleshooting.md#notifications-do-not-arrive) under Fejlfinding.
