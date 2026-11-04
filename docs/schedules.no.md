# Tidsplaner

**Innstillinger, Tidsplan** samler alle planer: den fulle sikkerhetskopien som dekker alt, en egen plan for et område, de eksterne kopiene, hvilke elementer som er med og i hvilken rekkefølge. Et enkelt element kan få en egen plan på sin egen side. Denne siden forklarer hvordan det henger sammen, og går gjennom et typisk oppsett.

## Planer som leses som setninger {#plans}

Hver plan er en knapp som leses som en setning, for eksempel *daglig kl. 03:00* eller *hver 2. dag kl. 01:30*. Et klikk åpner den med fem moduser:

| Modus | Hva du setter |
|---|---|
| **Av** | Ingenting kjører på denne planen. |
| **Daglig** | **Tid** |
| **Ukentlig** | **Dager** og **Tid** |
| **Hver N. dag** | **Hver** så mange **dager**, og **Tid**. Den teller fra siste planlagte kjøring, så en sikkerhetskopi av ett element for hånd skyver ikke planen tilbake. |
| **Cron** | Et **Uttrykk** med fem felt, for eksempel `0 */6 * * *` for hver 6. time. **Eksempler** tilbyr noen få, og vinduet viser de neste kjøringene. |

Alle klokkeslett følger serverens tidssone. På Unraid er det systemets egen. Andre steder setter du `TZ`, ellers kjører alle planer i UTC, se [Miljøvariabler](configuration.md#environment-variables).

## Uke {#week}

Kortet **Uke** tegner hver planlagte kjøring fra mandag til søndag, én linje per dag: sikkerhetskopier, eksterne kopier og kontroller. Pek på en strek for å lese hva som starter der. Der tre planer starter innen to timer, er stedet merket, så du kan spre dem utover.

![Innstillinger, Tidsplan: Uke-båndet øverst, deretter den fulle sikkerhetskopien og planene per område.](assets/screenshots/schedule/week-dark.png#only-dark)
![Innstillinger, Tidsplan: Uke-båndet øverst, deretter den fulle sikkerhetskopien og planene per område.](assets/screenshots/schedule/week-light.png#only-light)

## Full sikkerhetskopi {#full-backup}

**Full sikkerhetskopi** er standardplanen for alt. **Når sikkerhetskopiene kjører** setter den: én runde som tar sikkerhetskopi av hvert område etter tur, containere, VM-er, flash, mapper, ZFS-datasett og til slutt auto-sikkerhetskopien. Linjen under sier hvilke områder den omfatter. **Av** slår den av, og da kjører bare det som har en egen plan.

- **Ta igjen tapte sikkerhetskopier etter oppstart**: når serveren var av til et planlagt tidspunkt, kjører BombVault den sikkerhetskopien omtrent to minutter etter oppstart.
- **Kommandoer før og etter**: **Kommando før den fulle sikkerhetskopien** kjører på serveren med `sh -c` før runden starter, for eksempel et skript som stopper en tjeneste. Hvis den feiler, kjører den fulle sikkerhetskopien likevel. **Kommando etter den fulle sikkerhetskopien** kjører én gang etter at alle områder er forsøkt, selv om ett av dem feilet, noe som passer for en ping som bekrefter at hele serveren er beskyttet. Kommandoer for én enkelt container finner du på siden til containeren.

**Sikkerhetskopier alt nå** på siden Sikkerhetskopier starter den samme runden for hånd, over alle områder som er slått på.

## Planer per område {#plans-per-section}

Et område kan gå ut av den fulle sikkerhetskopien og kjøre til sin egen tid. I **Tidsplaner per område** legger **Unntak** til et område med egen plan, og **Fjern unntaket** sender det tilbake. Linjen i kortet sier hva de andre gjør, for eksempel **Alle andre områder kjører i den fulle sikkerhetskopien.**

Et område med egen plan sikkerhetskopieres ikke på nytt i den fulle sikkerhetskopien, så ingenting tas to ganger. Det samme gjelder et enkelt element med egen plan.

**Off-site-kopier per område** fungerer på samme måte for kopier. Som standard kopierer hvert område til sine eksterne mål rett etter hver sikkerhetskopi. **Unntak** gir ett område en egen kopiplan, for eksempel slik at VM-ene bare kopierer til Backblaze B2 søndag kveld. Et måls eget kort **Kopier** kan også sette **Når det kopieres**, se [Eksterne kopier](offsite-recovery.md#off-site-copies).

## Planlagte elementer {#scheduled-entries}

**Planlagte elementer** sier per område hvor mange elementer som er med, for eksempel **38 av 41 planlagt**. **Elementer** åpner listen med en bryter **I tidsplanen** per element. Et element som er slått av, sikkerhetskopieres bare når du starter det for hånd. Det regnes som ubeskyttet med mindre du merker det **Ikke sikkerhetskopier**.

## Rekkefølge {#order}

**Rekkefølge** setter i hvilken rekkefølge planlagte kjøringer og den fulle sikkerhetskopien tar sikkerhetskopi av containere og VM-er. Dra et element på plass eller bruk pilene, og hvert flytt lagres med en gang. Elementer du lar stå uten rekkefølge, sikkerhetskopieres mest forsinket først. En enkelt sikkerhetskopi for hånd påvirkes ikke.

**Omstart etter sikkerhetskopiering** i samme kort gjelder containerne som **Stopp andre containere** tok ned under en sikkerhetskopi. De starter igjen i avhengighetsrekkefølge, og med **Vent til avhengigheter er friske før den neste startes** må hver av dem melde seg frisk eller kjørende før den neste starter. **Helsetidsavbrudd per container (sekunder)** setter et tak for ventetiden, 120 som standard, i et område fra 5 til 3600.

## Øyeblikksbilde i stedet for stopp {#snapshot-instead-of-stop}

**Stopp containere på ZFS bare for et øyeblikksbilde** er på som standard. En container med data på ZFS stoppes bare mens BombVault tar et øyeblikksbilde av dataene, vanligvis noen sekunder, og sikkerhetskopien leses deretter fra øyeblikksbildet. Det krever SSH-tilkoblingen til verten som ZFS-datasett bruker. En container med en del av dataene et annet sted stoppes under hele sikkerhetskopien som før, og kjøringen sier hvorfor, mens en container uten data på ZFS stoppes uten noen merknad. Det fungerer på en Unraid-brukerdeling som ligger på én enkelt ZFS-pool. På TrueNAS trenger appen også kapabiliteten `SYS_CHROOT`, se [TrueNAS SCALE](getting-started.md#truenas). En enkelt container kan settes annerledes på sin egen side.

Sikkerhetskopien havner under de samme stiene og taggene som en tatt med stoppet container, så gjenopprettinger og kontroller behandler begge likt. Kommandoen før sikkerhetskopien og databasedumpen kjører fortsatt først, de andre containerne som skal stoppes går ned og kommer opp igjen sammen med den, og kommandoen etter sikkerhetskopien kjører så snart containeren er oppe igjen. En oppdatering etter sikkerhetskopien venter fortsatt til sikkerhetskopien er ferdig. Kjøringene viser **Snapshot** og hvor mange sekunder containeren var nede. Skulle BombVault stoppe midt i en containersikkerhetskopi, starter den containerne den holdt nede igjen neste gang den starter.

## Oppdateringer etter sikkerhetskopien {#updates}

- **Oppdater containere etter en vellykket sikkerhetskopi** henter et nyere image først etter at sikkerhetskopien før det er ferdig, så det alltid finnes et ferskt gjenopprettingspunkt først. Det er av som standard, og en enkelt container kan settes annerledes på sin side under **Oppdater etter en vellykket sikkerhetskopi**.
- **Fjern det gamle imaget etter en oppdatering** sletter imaget containeren forlot. Et basisimage som andre containere deler, slettes aldri.
- **Oppdater Unraids oppdateringsstatus etter oppdatering av en container** fjerner Unraids oppdateringsbanner for den containeren.

En melding per oppdatert container settes under **Innstillinger, Varsler** med **Varsle ved containeroppdatering**.

## Ro før sikkerhetskopi {#idle}

Kortet **Ro før sikkerhetskopi** setter når en app regnes som i ro for en container hvis planlagte sikkerhetskopi venter på det: **CPU under (% av en kjerne)**, **Trafikk under (Mbit/s)** og **I minst (min)**. En medieserver regnes som i ro når den ikke strømmer, og for en container på vertsnettverket teller bare CPU. Selve ventingen settes på hver container, se [Når appen er opptatt](backups.md#app-busy).

## En egen plan for ett element {#own-plan}

Hvert element har **Egen tidsplan** i kortet **Tidsplan og mål** på fanen Innstillinger. Med en plan der kjører elementet til sine egne tider og forlater den fulle sikkerhetskopien og planen til området sitt. Uten en følger det dem.

## Eksempel: tre planer som passer sammen {#example}

En typisk Unraid-boks: de fleste containerne kan sikkerhetskopieres om natten, VM-ene er store og endrer seg sakte, og Immich får nye bilder hele dagen.

1. **Full sikkerhetskopi kl. 03:00.** Åpne **Innstillinger, Tidsplan**. I **Full sikkerhetskopi** setter du **Når sikkerhetskopiene kjører** til **Daglig** kl. `03:00`. Slå på **Ta igjen tapte sikkerhetskopier etter oppstart**, så en natt med serveren av tas igjen.
2. **VM-er én gang i uken.** I **Tidsplaner per område** trykker du på **Unntak**, velger **VM-er** og setter planen til **Ukentlig**, **Dager** søndag, **Tid** `04:00`. Kortet sier nå at alle andre områder kjører i den fulle sikkerhetskopien.
3. **VM-ene kopierer til off-site også søndag.** I **Off-site-kopier per område** trykker du på **Unntak**, velger **VM-er** og setter **Ukentlig**, søndag kl. `06:00`, etter at sikkerhetskopien deres har hatt tid til å bli ferdig.
4. **Immich hver 6. time.** Åpne **Sikkerhetskopier**, deretter `immich`, så fanen **Innstillinger**. Sett **Egen tidsplan** til **Cron** med `0 */6 * * *`.

Det som kjører nå:

| Når | Hva |
|---|---|
| Hver dag kl. 03:00 | Containere unntatt `immich`, flash, mapper, ZFS-datasett og auto-sikkerhetskopien, ett område etter det andre |
| Søndag kl. 04:00 | VM-er |
| Søndag kl. 06:00 | Den eksterne kopien av VM-ene |
| 00:00, 06:00, 12:00 og 18:00 | `immich` for seg |

Kortet **Uke** viser 03:00-linjen hver dag, strekene kl. 04:00 og 06:00 søndag og fire streker om dagen for `immich`. `immich` sikkerhetskopieres ikke en gang til kl. 03:00, fordi et element med egen plan forlater den fulle sikkerhetskopien. Kopier av de andre områdene kjører fortsatt rett etter hver sikkerhetskopi.

Når du senere fjerner `immich`s egen plan, er den med i kjøringen kl. 03:00 igjen sammen med området sitt.
