# Notifikationer

BombVault kan give dig besked, når en sikkerhedskopiering mislykkes, når den lykkes, når en er forsinket, og én gang om ugen, hvordan det er gået. Alt indstilles under **Indstillinger, Notifikationer**: hvad der udløser en besked, på kortet **Notifikationer**, hvor beskederne sendes hen, på kortet **Kanaler**, og indtjekningspingene på kortet **Healthchecks**.

![Indstillinger, Notifikationer med Gotify-kanalen udfyldt.](assets/screenshots/notifications/gotify-dark.png#only-dark)
![Indstillinger, Notifikationer med Gotify-kanalen udfyldt.](assets/screenshots/notifications/gotify-light.png#only-light)

## Hvad der sender en besked {#what}

Kortet **Notifikationer** sætter reglerne for alle kanaler på én gang:

- **Underret**: **Aldrig**, **Kun ved fejl** eller **Ved succes og fejl**, for alle kanalerne nedenfor.
- **Opsummer planlagte kørsler**: én besked pr. planlagt kørsel, for eksempel *42 af 45 lykkedes*, i stedet for én pr. container eller VM. En sikkerhedskopiering, du starter i hånden, sender stadig sin egen besked. En frosset gæst, en mislykket opbevaring eller et overskredet vækstbudget går ud med det samme, selv når opsummeringer er slået til.
- **Giv besked ved containeropdatering**: én besked for hver container, som **Opdater containere efter en vellykket sikkerhedskopiering** har flyttet til et nyere image, så du kan se efter, at den stadig virker.
- **Meld forsinkede sikkerhedskopier**: én gang om dagen kl. 09:00 tjekker BombVault, om en aktiveret sikkerhedskopiering er forsinket, altså ældre end dobbelt så lang tid som sin tidsplan, og sender én besked pr. tilfælde. Den næste vellykkede sikkerhedskopiering rydder den.
- **Unraid-notifikationer**: sender til Unraids eget notifikationssystem, som kan videresende til Pushover, e-mail eller Discord. Det kører over SSH-forbindelsen fra **Indstillinger, Integrationer, Værts-SSH**, så den nøgle skal være godkendt. På TrueNAS og andre værter forbliver det slået fra.
- **Ugentlig oversigt**: én opsummering om ugen med antal kørsler, nye sikkerhedskopidata, om off-site-kopierne er aktuelle, og de hyppigste fejl. Dens plan er en knap som alle andre planer, og **Fra** slår den fra. Se [Tidsplaner](schedules.md#plans).

Hvilke afvigelser der sender en besked, indstiller du under **Indstillinger, Verificering**, se [Afvigelser](anomalies.md). En udskudt sikkerhedskopiering sender én besked, når den begynder at vente, og én, når ventetiden slutter, ikke én pr. forsøg.

**Send test** nederst på kortet sender en testbesked gennem hver kanal, der er slået til, med værdierne, som de står i formularen.

## Kanaler {#channels}

Hver kanal har sin egen kontakt på kortet **Kanaler**, og dens felter dukker op, når den er slået til. Hver kanal sender for sig selv med sin egen tidsgrænse, så en vært, der aldrig svarer, ikke holder de andre op, og en besked, ingen kanal tog imod, prøves igen.

| Kanal | Felter | Bemærkninger |
|---|---|---|
| **Webhook** | **Webhook-URL**, **Webhook-format** | Formater: **Generic JSON**, **Discord**, **Slack**, **Gotify** og **ntfy**. Generic JSON sender `title`, `message` og `ok`. |
| **Apprise** | **Apprise API-URL**, **Tags (valgfrit)** | Peg på `/notify/<key>`-endpointet i din Apprise API-container for at nå dens 100+ tjenester. Tags vælger en delmængde af den nøgles mål. |
| **Matrix** | **Homeserver-URL**, **Adgangstoken**, **Rum-ID** | Tokenet bliver på serveren og vises aldrig igen. |
| **Gotify** | **Server-URL**, **App-token**, **Prioritet ved fejl**, **Prioritet ved succes** | Prioriteterne går fra 0 til 10 og starter på 8 for fejl og 5 for succes. |
| **E-mail (SMTP)** | **SMTP-vært**, **Port**, **Kryptering**, **Brugernavn**, **Adgangskode**, **Fra-adresse**, **Til-adresse** | **Kryptering** er **STARTTLS**, **TLS (implicit)** eller **Ingen**. |

Hemmeligheder som Matrix-tokenet, Gotifys app-token og SMTP-adgangskoden gemmes krypteret. Under en gemt hemmelighed står der i feltet **Gemt. Lad feltet stå tomt for at beholde det.** Når du ændrer serverens adresse, sender BombVault ikke den gamle hemmelighed til det nye sted: Feltet beder om den igen, og den nye adresse gemmes, når du har skrevet den.

## Healthchecks {#healthchecks}

En tjeneste som [Healthchecks.io](https://healthchecks.io) slår alarm, når BombVault holder op med at tjekke ind. Indsæt tjekkets **Ping-URL**, og BombVault pinger den under hele forløbet af hver sikkerhedskopiering: ved start, derefter ved succes eller med `/fail`. Det sker, så snart en URL er sat, uanset hvad **Underret** siger, så tjekket forbliver grønt ved succes, selv når kun fejl giver besked.

Et område kan pinge en adresse for sig selv: **Undtagelse** på kortet tilføjer en, og linjen siger **Alle andre områder pinger denne URL.**

## Eksempel: Gotify på din telefon {#example-gotify}

Du kører Gotify på `https://gotify.example.com` og vil have, at fejl ringer på din telefon, mens succeser kommer stille.

1. Åbn **Apps** i Gotify, opret et program kaldet `BombVault`, og kopiér dets token.
2. Åbn **Indstillinger, Notifikationer** i BombVault. Sæt **Underret** til **Ved succes og fejl** på kortet **Notifikationer**, og slå **Opsummer planlagte kørsler** til, så en nat med 45 sikkerhedskopieringer sender én besked i stedet for 45.
3. Slå **Gotify** til under **Kanaler**.
4. Skriv `https://gotify.example.com` som **Server-URL**. BombVault tilføjer selv `/message`.
5. Indsæt tokenet i **App-token**.
6. Lad **Prioritet ved fejl** stå på 8 og **Prioritet ved succes** på 5. Gotifys Android-app behandler 8 og opefter som høj, så kun fejl giver lyd.
7. Tryk på **Send test**. Linjen under kortet siger, om Gotify tog imod den, og et afvist token nævnes som sådan.

Den næste planlagte kørsel sender én opsummering med prioritet 5, eller 8, hvis noget i den mislykkedes.

## Eksempel: ntfy som webhook {#example-ntfy}

ntfy kræver ingen konto til et offentligt emne. Vælg et emnenavn, der er svært at gætte, for alle, der kender det, kan læse med.

1. Slå **Webhook** til under **Kanaler**.
2. Skriv emnets adresse som **Webhook-URL**, for eksempel `https://ntfy.sh/bombvault-tower-7f3k`, eller adressen på din egen ntfy-server.
3. Sæt **Webhook-format** til **ntfy**.
4. Abonnér på det samme emne i ntfy-appen, og tryk på **Send test**.

BombVault sender beskeden som brødtekst med titlen i `X-Title`. En fejl går ud med høj prioritet og tagget `warning`, så den skiller sig ud i appen.

## Når beskeder ikke kommer frem {#troubleshooting}

Se [Notifikationer kommer ikke frem](troubleshooting.md#notifications-do-not-arrive) under Fejlfinding.
