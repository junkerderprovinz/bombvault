# Tidsplaner

**Indstillinger, Tidsplan** rummer alle planer: den fulde sikkerhedskopi, der dækker alt, en plan for sig selv til et område, off-site-kopierne, hvilke poster der er med, og i hvilken rækkefølge. En enkelt post kan få en plan for sig selv på sin side. Denne side forklarer, hvordan de passer sammen, og gennemgår en typisk opsætning.

## Planer, der læses som sætninger {#plans}

Hver plan er en knap, der læses som en sætning, for eksempel *dagligt kl. 03:00* eller *hver 2. dag kl. 01:30*. Et klik åbner den med fem tilstande:

| Tilstand | Hvad du indstiller |
|---|---|
| **Fra** | Intet kører efter denne plan. |
| **Dagligt** | **Tid** |
| **Ugentligt** | **Dage** og **Tid** |
| **Hver N. dag** | **Hver** så mange **dage**, og **Tid**. Den tæller fra den sidste planlagte kørsel, så en sikkerhedskopiering af én post i hånden skubber ikke planen tilbage. |
| **Cron** | Et **Udtryk** med fem felter, for eksempel `0 */6 * * *` for hver 6. time. **Eksempler** tilbyder nogle stykker, og vinduet viser de næste kørsler. |

Alle tider er i serverens tidszone. På Unraid er det systemets egen. Andre steder skal du sætte `TZ`, ellers kører hver plan i UTC, se [Miljøvariabler](configuration.md#environment-variables).

## Uge {#week}

Kortet **Uge** tegner hver planlagt kørsel fra mandag til søndag, én linje pr. dag: sikkerhedskopier, off-site-kopier og tjek. Peg på en streg for at læse, hvad der starter dér. Hvor tre planer starter inden for to timer, er strækket markeret, så du kan sprede dem.

![Indstillinger, Tidsplan: båndet Uge øverst, så den fulde sikkerhedskopi og planerne pr. område.](assets/screenshots/schedule/week-dark.png#only-dark)
![Indstillinger, Tidsplan: båndet Uge øverst, så den fulde sikkerhedskopi og planerne pr. område.](assets/screenshots/schedule/week-light.png#only-light)

## Fuld sikkerhedskopi {#full-backup}

Den **Fulde sikkerhedskopi** er standardplanen for alt. **Hvornår sikkerhedskopierne kører** sætter den: ét gennemløb, der sikkerhedskopierer hvert område efter tur, containere, VM'er, flash, mapper, ZFS-datasæt og til sidst auto-sikkerhedskopien. Linjen under siger, hvilke områder den omfatter. **Fra** slår den fra, og så kører kun det, der har en plan for sig selv.

- **Indhent forpassede sikkerhedskopieringer efter start**: Når serveren var slukket på et planlagt tidspunkt, kører BombVault den sikkerhedskopiering omkring to minutter efter start.
- **Kommandoer før og efter**: **Kommando før den fulde sikkerhedskopi** kører på serveren med `sh -c`, før gennemløbet starter, for eksempel et script, der stopper en tjeneste. Mislykkes den, kører den fulde sikkerhedskopi alligevel. **Kommando efter den fulde sikkerhedskopi** kører én gang, efter at hvert område er prøvet, selv om et af dem mislykkedes, hvilket passer til en ping, der bekræfter, at hele serveren er beskyttet. Kommandoer til en enkelt container står på dens side.

**Sikkerhedskopiér alt nu** på siden Sikkerhedskopier starter det samme gennemløb i hånden, over hvert aktiveret område.

## Planer pr. område {#plans-per-section}

Et område kan forlade den fulde sikkerhedskopi og køre på sit eget tidspunkt. Under **Tidsplaner pr. område** tilføjer **Undtagelse** et område med en plan for sig selv, og **Fjern undtagelsen** sender det tilbage. Linjen på kortet siger, hvad de andre gør, for eksempel **Alle andre områder kører i den fulde sikkerhedskopi.**

Et område med sin egen plan sikkerhedskopieres ikke igen i den fulde sikkerhedskopi, så intet sikkerhedskopieres to gange. Det samme gælder en enkelt post med sin egen plan.

**Off-site-kopier pr. område** virker på samme måde for kopier. Som standard kopierer hvert område til sine off-site-destinationer lige efter hver sikkerhedskopiering. **Undtagelse** giver ét område en kopiplan for sig selv, for eksempel så VM'erne kun kopierer til Backblaze B2 søndag nat. En destinations eget kort **Kopier** kan også sætte **Hvornår der kopieres**, se [Off-site-kopier](offsite-recovery.md#off-site-copies).

## Planlagte elementer {#scheduled-entries}

**Planlagte elementer** siger pr. område, hvor mange poster der er med, for eksempel **38 af 41 planlagt**. **Elementer** åbner listen med en kontakt **På tidsplanen** pr. post. En post, der er slået fra, sikkerhedskopieres kun, når du starter den i hånden. Den tæller som ubeskyttet, medmindre du markerer den **Sikkerhedskopiér ikke**.

## Rækkefølge {#order}

**Rækkefølge** sætter den rækkefølge, som planlagte kørsler og den fulde sikkerhedskopi sikkerhedskopierer containere og VM'er i. Træk en post på plads eller brug pilene, og hver flytning gemmes med det samme. Poster, du lader stå uden rækkefølge, sikkerhedskopieres med den mest forsinkede først. En enkelt sikkerhedskopiering i hånden påvirkes ikke.

**Genstart efter sikkerhedskopiering** på samme kort dækker de containere, som **Stop andre containere** tog ned under en sikkerhedskopiering. De starter igen i deres afhængighedsrækkefølge, og med **Vent, til afhængigheder er sunde, før den næste startes** skal hver melde sund eller kørende, før den næste starter. **Sundhedstimeout pr. container (sekunder)** sætter loft over ventetiden, 120 som standard, i et område fra 5 til 3600.

## Snapshot i stedet for stop {#snapshot-instead-of-stop}

**Stop kun containere på ZFS for at tage et snapshot** er slået til som standard. En container, hvis data ligger på ZFS, stoppes kun, mens BombVault tager et snapshot af de data, som regel nogle få sekunder, og sikkerhedskopien læses så fra snapshottet. Det kræver SSH-forbindelsen til værten, som ZFS-datasæt bruger. En container med en del af sine data andre steder stoppes under hele sikkerhedskopieringen som før, og dens kørsel siger hvorfor, mens en container uden data på ZFS stoppes uden en note. Det virker på en Unraid-brugershare, der ligger på én ZFS-pulje. På TrueNAS skal appen også have kapabiliteten `SYS_CHROOT`, se [TrueNAS SCALE](getting-started.md#truenas). En enkelt container kan indstilles anderledes på sin egen side.

Sikkerhedskopien havner under de samme stier og tags som en, der er taget med containeren stoppet, så gendannelser og tjek behandler begge ens. Kommandoen før sikkerhedskopien og databasedumpet kører stadig først, de andre containere, der skal stoppes, går ned og kommer op sammen med den, og kommandoen efter sikkerhedskopien kører, så snart containeren er oppe igen. En opdatering efter sikkerhedskopien venter stadig, til sikkerhedskopien er færdig. Dens kørsler viser **Snapshot** og hvor mange sekunder containeren var nede. Hvis BombVault stopper midt i en containers sikkerhedskopiering, starter den de containere, den holdt nede, igen næste gang, den starter.

## Opdateringer efter sikkerhedskopien {#updates}

- **Opdater containere efter en vellykket sikkerhedskopiering** henter et nyere image først, når sikkerhedskopieringen før er færdig, så der altid findes et frisk gendannelsespunkt først. Den er slået fra som standard, og en enkelt container kan indstilles anderledes på sin side under **Opdater efter en vellykket sikkerhedskopiering**.
- **Fjern det gamle image efter en opdatering** sletter det image, containeren forlod. Et basisimage, som andre containere deler, slettes aldrig.
- **Opdater Unraids opdateringsstatus efter opdatering af en container** rydder Unraids opdateringsbanner for den container.

En besked pr. opdateret container indstilles under **Indstillinger, Notifikationer** med **Giv besked ved containeropdatering**.

## Ro før sikkerhedskopiering {#idle}

Kortet **Ro før sikkerhedskopiering** sætter, hvornår en app regnes for rolig for en container, hvis planlagte sikkerhedskopiering venter på den: **CPU under (% af en kerne)**, **Trafik under (Mbit/s)** og **I mindst (min)**. En medieserver regnes for rolig, når den ikke streamer, og for en container på værtsnetværket tæller kun CPU. Selve ventetiden indstilles på hver container, se [Når appen er optaget](backups.md#app-busy).

## En egen plan til én post {#own-plan}

Hver post har **Egen plan** på kortet **Tidsplan og destination** under fanen Indstillinger. Med en plan dér kører posten på sine egne tidspunkter og forlader den fulde sikkerhedskopi og sit områdes plan. Uden en følger den dem.

## Eksempel: tre planer, der passer sammen {#example}

En typisk Unraid-boks: De fleste containere kan sikkerhedskopieres om natten, VM'erne er store og ændrer sig langsomt, og Immich får nye fotos hele dagen.

1. **Fuld sikkerhedskopi kl. 03:00.** Åbn **Indstillinger, Tidsplan**. Sæt **Hvornår sikkerhedskopierne kører** til **Dagligt** kl. `03:00` under **Fuld sikkerhedskopi**. Slå **Indhent forpassede sikkerhedskopieringer efter start** til, så en nat med slukket server bliver indhentet.
2. **VM'er én gang om ugen.** Tryk på **Undtagelse** under **Tidsplaner pr. område**, vælg **VMs**, og sæt dens plan til **Ugentligt**, **Dage** søndag, **Tid** `04:00`. Kortet siger nu, at alle andre områder kører i den fulde sikkerhedskopi.
3. **VM'erne kopierer også off-site om søndagen.** Tryk på **Undtagelse** under **Off-site-kopier pr. område**, vælg **VMs**, og sæt **Ugentligt**, søndag kl. `06:00`, når deres sikkerhedskopiering har haft tid til at blive færdig.
4. **Immich hver 6. time.** Åbn **Sikkerhedskopier**, så `immich`, så dens fane **Indstillinger**. Sæt **Egen plan** til **Cron** med `0 */6 * * *`.

Hvad der kører nu:

| Hvornår | Hvad |
|---|---|
| Hver dag kl. 03:00 | Containere undtagen `immich`, flash, mapper, ZFS-datasæt og auto-sikkerhedskopien, ét område efter det andet |
| Søndag kl. 04:00 | VM'er |
| Søndag kl. 06:00 | Off-site-kopien af VM'erne |
| 00:00, 06:00, 12:00 og 18:00 | `immich` for sig selv |

Kortet **Uge** viser 03:00-linjen på hver dag, stregerne kl. 04:00 og 06:00 om søndagen og fire streger om dagen for `immich`. `immich` sikkerhedskopieres ikke en gang til kl. 03:00, fordi en post med sin egen plan forlader den fulde sikkerhedskopi. Kopier af de andre områder kører stadig lige efter hver sikkerhedskopiering.

Når du senere fjerner `immich`s egen plan, er den med i kørslen kl. 03:00 igen sammen med sit område.
