# Off-site og gjenoppretting

!!! note "Off-site-kopier venter etter en ombygging"
    Når Nødveiviser eller **Oppdag sikkerhetskopier** bygger opp elementer uten de gamle innstillingene, settes off-site-kopier av de områdene på pause til standarden deres er bekreftet. Se [Etter en ombygging](#after-a-rebuild).

Lokale sikkerhetskopier beskytter deg mot en tapt container eller en dårlig oppdatering. Off-site-kopier og et testet gjenopprettingssett beskytter deg mot hele boksen, løsepengevirus eller en brann. Denne siden tar for seg mål og off-site-kopier, hvordan du gjør en kopi sikker mot manipulering, hvordan du beviser at du kan gjenopprette, hvordan du parer flere servere, og hvordan du får alt tilbake når selve BombVault er borte.

## Off-site-kopier {#off-site-copies}

Behold den raske lokale sikkerhetskopien og legg til ett eller flere mål utenfor huset. BombVault kopierer nye øyeblikksbilder dit med `restic copy` på best-effort-basis, så et hikk utenfor huset aldri får den lokale sikkerhetskopien til å feile. I denne formen skriver hvert område først til sin egen mappe, og off-site-repoet er en kopi. Den egne mappen trenger ikke ligge på denne serveren: se [Et områdes egen mappe](#own-folder) for å ha den på et mål som S3 eller en rest-server.

- **Flere mål per område.** Hvert område (containere, VM-er, flash, auto-sikkerhetskopi, mapper og ZFS-datasett) kan kopiere til flere off-site-mål samtidig, så du for eksempel kan ha en rest-server hos en venn og en S3-bøtte parallelt. Kortet **Mål per område** på siden Mål slår hver linje fra et område til et mål av eller på.
- **Når kopier kjører.** Kortet **Kopier** til et mål setter **Når det kopieres**: **Etter hver sikkerhetskopi**, eller etter en egen plan som samler øyeblikksbildene og kopierer til faste tider. Under **Innstillinger, Tidsplan, Off-site-kopier per område** gir **Unntak** ett område en egen kopieringsplan.
- **Kortet Eksterne kopier.** Under **Innstillinger, Mål** har hvert område med en off-site-kopi en rad i **Eksterne kopier**, med **Test tilkobling**, **Replikér nå** for å starte en kopi på forespørsel, og **Sett opp** eller **Rediger**. Resten av områdets kopiinnstillinger åpnes i vinduet fra den raden.
- **Oppbevaring** settes per mål under **Hvor lenge det beholdes**, så du kan beholde off-site-kopier lenger som arkiv. Kopien i et områdes off-site-felt følger den som alle andre kopier, og et område kan ha egne verdier under **Regler per område**. Med alle verdier på null beskjæres aldri off-site-øyeblikksbilder. Se [Hvor lenge det beholdes](#keep).
- **Båndbreddegrenser** (**Ekstern båndbredde** under **Innstillinger, Mål**, eller kortet **Kopier** til et mål) begrenser resticts opplastings- og nedlastingshastighet slik at kopier ikke metter WAN-et ditt. De skrives i KiB/s, enheten restic bruker, og hvert felt viser samme hastighet i MB/s og Mbit/s mens du skriver: 2048 leses som omtrent 2,0 MB/s (16,8 Mbit/s).
- **Komprimering** er **Av**, **Automatisk** (resticts standard) eller **Maksimum**, satt på hvert mål og, for ett område, under reglene der. Sikkerhetskopier, kopier og prune skriver med den, og gjenopprettingssettet nevner den, så vanlig restic kan fortsette å skrive på samme måte.
- **Strømming først.** Mens en medieserver som Plex, Jellyfin eller Emby strømmer, lastes kopier opp med en lavere grense og går tilbake til den vanlige noen minutter etter at strømmen er slutt. Kortet **Strømming** på siden Mål har **Senk eksterne kopier under strømming**, **Medieservere** (containere med Plex, Jellyfin eller Emby i image-navnet til du velger selv), **Strømming over (Mbit/s)**, **Opplastingsgrense under strømming (KiB/s)** og **Tilbake til normalt etter (min)**. BombVault leser trafikken fra Docker, så en medieserver på vertsnettverket kan ikke måles. REST, S3, B2, Azure, Google Cloud, Swift og rclone over HTTP bremser midt i en kopi. SFTP og lokale eller monterte mapper tar den lavere grensen ved neste kopitrinn.
- **Kald lagring (S3).** Et S3-mål kan bruke en billigere **Lagringsklasse** som restic fortsatt kan lese med en gang, for eksempel Standard-IA, One Zone-IA, Intelligent-Tiering eller Glacier Instant Retrieval. Glacier Flexible Retrieval og Deep Archive er utelatt, fordi restic ikke kan gjenopprette fra dem. En rclone-remote setter klassen sin i rclone-konfigurasjonen.
- **Vekstbudsjett.** **Vekstbudsjett (GB, 0 = av)** i kopivinduet til et område slår alarm når repositoryet vokser forbi den størrelsen. Det sletter aldri noe, så det passer for en append-only-kopi som ingenting beskjærer herfra.
- Mens en kopi kjører, står det **Off-site-kopi pågår** i statushodet på oversikten. Det er en aktiv indikator, ikke en prosentlinje, fordi `restic copy` ikke gir noen maskinlesbar fremdrift.

!!! note "Gjenopprett fra hvilket som helst sted"
    Hvert element lister sikkerhetskopiene sine som én tidslinje på tvers av alle steder en sikkerhetskopi ligger, på fanen **Sikkerhetskopier**. En sikkerhetskopi som er kopiert til B2, vises én gang, merket med hvert sted som har den. En gjenoppretting tar det første stedet den kommer til, og begynner med repositoryet elementet skrives til, og du kan velge et annet sted per øyeblikksbilde. Off-site-mål leses bare når du klikker **Sjekk**, fordi det tar tid å liste skylagring. Å slette på ett sted sjekker først de andre og sier om det var den siste kopien. Se [Gjenoppretting](restore.md).

## Mål {#destinations}

Et mål er et sted sikkerhetskopier går: en lokal mappe, en deling, en server du eier selv eller en skytjeneste. Det settes opp én gang og betjener så hvert område. Åpne **Mål** fra innstillingsflisen, eller slå på **Vis også i sidefeltet** der for å feste det.

![Siden Mål: listen over mål og flyten som viser hvilket område som går hvor.](assets/screenshots/storage/locations-dark.png#only-dark)
![Siden Mål: listen over mål og flyten som viser hvilket område som går hvor.](assets/screenshots/storage/locations-light.png#only-light)

**Legg til mål** åpner et vindu i fire trinn:

1. **Leverandør.** Hver tjeneste er listet med logoen sin, med et søkefelt øverst til høyre i vinduet, i grupper: **Lokal** for en disk eller deling som er koblet til denne serveren, **NAS eller server** (rest-server, Hetzner Storage Box, SFTP, SMB, WebDAV og andre), **S3-tjeneste** (Backblaze B2, Wasabi, Cloudflare R2, Hetzner Object Storage, Amazon S3 og andre), din egen S3-server (Garage, SeaweedFS, RustFS, Silo, Ceph, JuiceFS, Versity S3 Gateway), **Skylagring** (OneDrive, Google Drive, Dropbox, pCloud, Nextcloud og resten rclone støtter) og **Fra gruppen din**, mottaksserverne i gruppen din. Hver sier hvor godt den passer for sikkerhetskopier: skystasjoner går tregere under mange forespørsler, så den første sikkerhetskopien og prune tar lengre tid der. For en tjeneste ingen flis dekker, tar flisen **Rclone** under **Egen rclone-konfigurasjon** imot remotes limt inn fra en rclone.conf.
2. **Logg inn.** Feltene avhenger av tjenesten: en tilgangsnøkkel for S3, brukernavn og passord for WebDAV og SMB, et appassord der tofaktorpålogging blokkerer det vanlige, BombVaults offentlige SSH-nøkkel for SFTP og Storage Box, eller et token for tjenester som logger inn via en nettleser. For dem viser vinduet en `rclone authorize`-kommando du kjører på en datamaskin med nettleser, og tokenet den skriver ut, går i feltet. Har du allerede en [lagret pålogging](#sign-ins) for tjenesten, velger **Lagret pålogging** den i stedet for **Nye nøkler**. **Test tilkobling** sjekker påloggingen før noe lagres. Vinduet lister så mappene på målet, med **Ny mappe** for å lage en og ledig plass der tjenesten melder den. En tom mappe er tryggest. BombVault legger én mappe per område i den.
3. **Beskyttelse.** Vinduet sier rett ut hva tjenesten kan gjøre. En rest-server i append-only-modus nekter sletting, og tamper-testen sjekker det. En S3-bøtte kan beholde gamle versjoner med versjonering og objektlås, som BombVault ikke kan sjekke ennå. En skystasjon kan ikke nekte sletting i det hele tatt: den som kommer inn på serveren, kommer også inn på den kopien. Slå **Beskyttelse mot sletting (append-only)** på bare der den andre siden virkelig nekter sletting. BombVault beskjærer da aldri der. **For en nødsituasjon** sier hva gjenopprettingssettet inneholder for dette målet.
4. **For hvilke områder.** Velg områdene som sikkerhetskopierer hit. Den første kopien tar med hele historikken. Elementer som har valgt egne mål, får ikke det nye av seg selv, og vinduet nevner dem.

![Legg til mål: det første trinnet, med hver leverandør som en flis.](assets/screenshots/storage/add-providers-dark.png#only-dark)
![Legg til mål: det første trinnet, med hver leverandør som en flis.](assets/screenshots/storage/add-providers-light.png#only-light)

![Legg til mål: påloggingstrinnet etter en vellykket test.](assets/screenshots/storage/add-signin-dark.png#only-dark)
![Legg til mål: påloggingstrinnet etter en vellykket test.](assets/screenshots/storage/add-signin-light.png#only-light)

Noen tjenester ber om litt mer ved påloggingen:

- **SFTP og Storage Box.** **Legg til nøkkel** legger BombVaults offentlige nøkkel på serveren med **Passordet til kontoen**, som brukes til den ene påloggingen og ikke lagres noe sted. Kommandoen for å gjøre det for hånd står under. Testen viser **Servernøkkel** med fingeravtrykket. BombVault husker den og avviser serveren hvis den senere viser en annen.
- **S3-nøkler for én enkelt bucket**, som et Cloudflare R2-objekttoken eller en avgrenset AWS-nøkkel, kan mangle lov til å liste de andre bucketene. Skriv bucketen i **Bucket**, så åpner testen og mappelisten den bucketen.
- **S3-regioner og adresser.** Regionen leses fra et endepunkt som nevner den (AWS, Backblaze B2, Hetzner, Scaleway, OVH, Wasabi), Garage starter med sin egen standardregion, og IDrive e2 ber om endepunktet dashbordet viser. En adresse limt inn med bucketen på, slik R2-dashbordet viser den, fyller ut serveren og bucketen. En bucket som lages fra mappevelgeren, havner i regionen skjemaet nevner.
- **Lagringsklasse.** Velgeren tilbyr bare klassene tjenesten tar, og **Flere innstillinger** viser bare valgene som former en ny bucket.
- **Google Drive og OneDrive.** Beskårne data slettes for godt i stedet for å havne i papirkurven, der de fortsatt ville telle mot kvoten. En delt Google-stasjon står under **Flere innstillinger**.
- **SMB-servere som ikke lister delingene sine**, tar navnet på én under **Delt ressurs**, og testen åpner den delingen direkte.
- **FTP** ber om **Kryptering**: **Ingen**, **Eksplisitt TLS (FTPES)** eller **Implisitt TLS (FTPS, port 990)**.
- **OneDrive og Zoho WorkDrive.** Når en pålogging åpner flere stasjoner eller arbeidsområder, spør vinduet hvilken sikkerhetskopiene skal til.
- **En rest-server med selvsignert sertifikat.** Legg sertifikatet som en PEM-fil i mappen `certs` i BombVaults datamappe (`/config/certs` i containeren, `appdata/bombvault/certs` på Unraid). Sikkerhetskopier, kopier og testen av slettebeskyttelsen stoler på det uten omstart.

S3-tjenester går gjennom resticts egen S3-backend, og det er det som lar en lagringsklasse og objektlås gjelde. En rest-server nås av restic direkte. Hver annen tjeneste går gjennom rclone-versjonen BombVault leverer, og seksjonen deres i rclone-konfigurasjonen vises i kortet **rclone.conf** på siden til målet, med hemmelighetene skjult. En eksport av innstillinger tar med målene, og med påloggingsopplysninger inkludert tar den med påloggingene deres også.

Å lagre et mål som allerede har sikkerhetskopier av containere, VM-er eller mapper, for eksempel et du fjernet og legger til igjen, legger det til de områdene som en kopi uten hake. Ingenting kopieres dit før du huker av, men **Gjenoppretting** tilbyr de gamle sikkerhetskopiene med en gang.

Mappen på denne serveren heter **Lokal** overalt: på lagringssidene, i flyten, på et element og på Gjenoppretting. Med flere mapper på denne serveren går hver under sitt eget navn. Et mål som settes opp fra et områdes off-site-felt, får navn etter stedet sitt, den siste mappen i adressen eller, uten en, serveren, og får et nummer når et annet mål allerede heter det. Hvert mål kan gis nytt navn i feltet **Navn** i kortet **Tilkobling**, også en bøttemappe som flere områder deler og kopien et områdes off-site-felt beskriver, og et navn du gir det, blir stående. Der to mål med samme navn står side ved side, bærer hvert sin server eller mappe.

### Siden til et mål {#location-page}

Hvert mål har en egen side. Den viser bare kortene som gjelder for det målet, i denne rekkefølgen:

![Siden til et mål: Egne mapper, Tilkobling og Legitimasjon.](assets/screenshots/storage/location-page-dark.png#only-dark)
![Siden til et mål: Egne mapper, Tilkobling og Legitimasjon.](assets/screenshots/storage/location-page-light.png#only-light)

- **Egne mapper**: områdene som skriver sikkerhetskopiene sine hit først. Påloggingen, beskyttelsen og grensene til dette målet gjelder for dem. **Bruk som egen mappe** flytter den egne mappen til flere områder hit, og **Endre** flytter en bort igjen. Se [Et områdes egen mappe](#own-folder).
- **Sikkerhetskopieres rett hit**: hvert område der elementer sikkerhetskopierer rett til dette stedet i stedet for til den egne mappen sin, og hvor mange elementer som gjør det. Et slikt direkte repository henter alle innstillingene sine fra dette målet. Et som ingenting sikkerhetskopierer til lenger, sier det, og **Fjern** tar det av listen. Sikkerhetskopiene på stedet blir. Se [Direkte repoer](#direct-repositories).
- **Tilkobling**: **Navn**, bryteren **Tilgjengelig** (av betyr at BombVault ikke skriver hit lenger, og elementer som sikkerhetskopierer hit, feiler med en tydelig melding i stedet for å havne et annet sted), mappen, **Komprimering**, **Lagringsklasse** og **Utenfor lokalet**. På målet **Lokal** lister den mappen til hvert område under **Egne mapper her**. **Endre tilkobling** tester og lagrer en ny adresse eller pålogging. Når andre steder logger inn med samme lagrede pålogging, sier vinduet det, og en endring her gir dette målet en egen pålogging i stedet for å skrive om den delte.
- **Legitimasjon**: hvilken lagret pålogging den bruker (**Logger på med**), hvilke andre mål som bruker den også (**Brukes også av**), og **Rediger**, som åpner påloggingsvinduet. Se [Pålogginger](#sign-ins).
- **rclone.conf**: seksjonen til dette målet i rclone-konfigurasjonen, for et mål som går gjennom rclone.
- **Beskyttelse mot sletting**, med **Beskyttelse mot sletting (append-only)**, **Godta uten beskyttelse mot sletting**, **Test beskyttelsen mot sletting nå** og **Hvem beskjærer**, som sier om BombVault beskjærer kopien og de egne mappene her, etter kortet **Hvor lenge det beholdes** for dette målet eller et områdes egne verdier under **Regler per område**, eller overlater det til den andre siden. En lokal mappe viser dette kortet bare når den har en beskyttelse du kan slå av. Se [Beskyttelse mot sletting](#delete-protection-append-only).
- **Kopier**: **Når det kopieres**, og **Opplastingsgrense (KiB/s)** og **Nedlastingsgrense (KiB/s)**. Et mål som bare har egne mapper og ingen kopier, viser grensene som et kort som heter **Båndbredde**.
- **Hvor lenge det beholdes**: **Kort**, **Balansert**, **Lang** eller **Egne verdier**, med en tidslinje over hvilke sikkerhetskopier regelen beholder. Se [Hvor lenge det beholdes](#keep).
- **Ligger allerede under dette målet**: områder der en adresse skrevet for hånd ligger i en mappe på dette målet. Å ta et over lar repositoryet bli der det er. Pålogging, lagringsklasse og beskyttelse mot sletting kommer da fra målet. BombVault sjekker først at påloggingen til målet åpner repositoryet, og nekter å legge et append-only-repository under et mål som ikke er append-only.
- **Regler per område**: ett områdes eget repository, oppbevaring eller komprimering på dette målet. **Endre regler** åpner dem, og **Tilbake til målet** henter et område tilbake. Pålogging, båndbredde og når kopier kjører, kommer alltid fra målet.
- Et kar som viser hvor fullt målet er, med merket der det er fullt og hvor mange uker som er igjen ved veksten de siste ukene. Et mål som ikke melder størrelsen sin, sier **Størrelse ukjent**.

**Flytt** og **Fjern mål** står blant handlingene på siden, beskrevet under [Flytte og fjerne](#move-remove).

Et eksternt mål som hadde sikkerhetskopier og plutselig ikke har noe repository, for eksempel en rest-server der disken ikke ble montert, får ikke et nytt, tomt av seg selv. Siden dens sier det så snart en sikkerhetskopi eller kopi oppdager at repositoryet er borte. Monter disken igjen. Hvis du tømte det gamle repositoryet med vilje, starter **Opprett et nytt depot her** på nytt, for hvert område på målet og repositoryene deres på én gang.

En mottaksserver som en annen instans i gruppen din kjører, vises blant leverandørene som **Fra gruppen din**. Se [Mottaksserver](#receiving-server).

BombVault sjekker målene sine ved hver oppstart. Den ordner det som ikke trenger noen avgjørelse fra deg, som en lenke til en kopi eller et repository som er borte, en kopi på den egne mappen til et område, eller et direkte repository der kopien er borte, og skriver en linje i loggen for hver rettelse. Det som ikke har noen trygg rettelse, blir bare logget.

![Den samme siden på en telefon.](assets/screenshots/storage/location-page-phone-dark.png#only-dark)
![Den samme siden på en telefon.](assets/screenshots/storage/location-page-phone-light.png#only-light)

### Hvor lenge det beholdes {#keep}

Hvor lenge sikkerhetskopier beholdes, settes per mål, i kortet **Hvor lenge det beholdes**. Regelen gjelder kopiene på det målet og de egne mappene som ligger der, **Lokal** inkludert. Et område kan ha egne verdier på ett mål under **Regler per område**, og de gjelder foran kortet til målet for det området.

En regel beholder én sikkerhetskopi per dag, uke, måned og år, så langt tilbake som du setter, og rydder bort resten etter hver sikkerhetskopi eller kopi:

| Forhåndsvalg | Siste sikkerhetskopier | Dager | Uker | Måneder | År |
|---|---|---|---|---|---|
| **Kort** | 0 | 7 | 4 | 3 | 0 |
| **Balansert** | 0 | 7 | 4 | 6 | 2 |
| **Lang** | 0 | 14 | 8 | 12 | 3 |

**Egne verdier** setter de fem tallene selv. En 0 slår det tallet av, og en regel med alle tall på 0 beholder alt. Prikkene i kortet viser hvilke av 40 eksempelkopier en regel beholder, den nyeste lengst til høyre. Eksempelet er det samme for hvert mål, siden bildet tegnes før BombVault har lest sikkerhetskopiene der. Det teller bakover fra i dag, så hvor mange prikker som blir igjen, endrer seg med datoen.

**Spør før det beholdes mindre**, på som standard, spør før en endring som beholder færre sikkerhetskopier lagres, og sier hvor mange av de 40 eksempelkopiene den nye regelen fjerner. Det gjelder hvert mål. **Tilbake til målet** under **Regler per område** stiller det samme spørsmålet.

Et mål med beskyttelse mot sletting beskjæres aldri fra denne boksen. Den andre siden beholder eller dropper gamle sikkerhetskopier etter sine egne regler. Mens et element har et åpent kritisk avvik om tapte data, beholder oppbevaringen de gamle sikkerhetskopiene til elementet til du kvitterer for det. Se [Avvik](anomalies.md).

For å se hva neste kjøring faktisk ville slette, åpner du **Forhåndsvisning** under **Innstillinger, Mål**. Den lister **Hva neste kjøring ville fjerne**, per repository og per element med gjenopprettingspunktene nevnt. Den tar ingen lås og endrer ingenting.

![Hvor lenge det beholdes med Balansert valgt: prikkene viser hvilke av de 40 eksempelkopiene som blir.](assets/screenshots/storage/keep-dark.png#only-dark)
![Hvor lenge det beholdes med Balansert valgt: prikkene viser hvilke av de 40 eksempelkopiene som blir.](assets/screenshots/storage/keep-light.png#only-light)

#### Eksempel: hva Balansert beholder {#example-retention}

En container har blitt sikkerhetskopiert hver natt kl. 03:00 siden 1. oktober 2024, så onsdag 4. november 2026 har den 765 sikkerhetskopier. Målet er satt til **Balansert**: 7 dager, 4 uker, 6 måneder og 2 år.

Hvert tall beholder den nyeste sikkerhetskopien fra sin dag, uke, måned eller år, og en sikkerhetskopi som flere tall velger, beholdes én gang. Uker går fra mandag til søndag:

| Regel | Sikkerhetskopier den velger | Nye den legger til |
|---|---|---|
| 7 dager | 4. nov, 3. nov, 2. nov, 1. nov, 31. okt, 30. okt, 29. okt | alle 7 |
| 4 uker | 4. nov, 1. nov, 25. okt, 18. okt | 25. okt, 18. okt |
| 6 måneder | 4. nov, 31. okt, 30. sep, 31. aug, 31. jul, 30. jun | 30. sep, 31. aug, 31. jul, 30. jun |
| 2 år | 4. nov, 31. des 2025 | 31. des 2025 |

Det gir 14 sikkerhetskopier av 765. Den eldste er fra 31. desember 2025, så i dag kan du gå litt over ti måneder tilbake. Takket være deduplisering tar de bare litt mer plass enn én full sikkerhetskopi pluss det som endret seg imellom.

**År** fortjener et ekstra blikk. Det teller kalenderår som har en sikkerhetskopi, med det inneværende først, og den nyeste sikkerhetskopien i år er dagens. Så det første året legger ikke til noe utover de andre tallene, og det andre legger til den siste sikkerhetskopien fra året før. Derfor varierer rekkevidden til en regel gjennom året. De samme sikkerhetskopiene under hvert forhåndsvalg:

| Forhåndsvalg | Sikkerhetskopier beholdt i dag | Eldste sikkerhetskopi i dag | Rekkevidde gjennom året |
|---|---|---|---|
| **Kort** | 10 | 30. september 2026 | 1 til 2 måneder |
| **Balansert** | 14 | 31. desember 2025 | omtrent 4 måneder tidlig på våren, et helt år 31. desember |
| **Lang** | 30 | 31. desember 2024 | 1 år 1. januar, 2 år 31. desember |

Å gå fra **Balansert** til **Kort** spør først, på grunn av **Spør før det beholdes mindre**. Spørsmålet sier hvor mange av de 40 sikkerhetskopiene i bildet den nye regelen fjerner. Tallet kommer fra eksempelet, så det endrer seg med datoen og teller ikke dine egne sikkerhetskopier. Av sikkerhetskopiene i dette eksempelet ville 31. august, 31. juli, 30. juni og 31. desember 2025 gå. **Lagre likevel** bruker regelen, og neste prune sletter dem for godt fra det målet.

For å beholde kontainerne lenger enn resten på dette målet trykker du **Endre regler** under **Regler per område**, velger **Kontainere** og setter **Egne verdier** i **Regler for Kontainere**, for eksempel 14 dager, 8 uker, 12 måneder og 2 år. De andre områdene følger fortsatt kortet til målet, og **Tilbake til målet** henter Kontainere tilbake.

### Pålogginger {#sign-ins}

En pålogging er nøkkelen eller passordet BombVault bruker for en S3-tjeneste eller en rest-server. Påloggingene er én liste, kortet **Pålogginger** under **Innstillinger, Mål**, og hver rad sier hvilke mål som bruker den (**Brukes av**).

- **Lagret når du legger til et mål.** Legg til mål lagrer nøklene du skriver, som en pålogging. Det neste målet hos samme tjeneste kan velge den under **Lagret pålogging** i stedet for **Nye nøkler**, og en senere endring av den gjelder hvert mål som bruker den.
- **Standarden.** En stjerne markerer standardpåloggingen. Den brukes overalt der ingen pålogging er valgt, som en adresse skrevet for hånd, en henting-kilde eller en gjenoppretting på en fersk installasjon. **Gjør til standard** gir stjernen til en annen.
- **Rediger og Test.** **Rediger**, her eller i kortet **Legitimasjon** på et mål, åpner ett vindu for påloggingen. **Test** logger inn på hvert mål som bruker den med det du skrev, før noe lagres. **Lagre** lagrer den.
- **Fjerne.** En pålogging kan fjernes bare når ingen mål eller henting-kilde bruker den, og standarden går bare som den siste. Raden sier da hva som fortsatt bruker den.
- **Å endre tilkoblingen til et mål** skriver aldri om en pålogging som andre steder deler. Målet får i stedet en egen pålogging, og vinduet sier på forhånd at påloggingen er delt. For å endre den delte overalt redigerer du den i kortet **Pålogginger**.

![Kortet Pålogginger: hver lagrede pålogging med det som bruker den, og stjernen på standarden.](assets/screenshots/storage/signins-dark.png#only-dark)
![Kortet Pålogginger: hver lagrede pålogging med det som bruker den, og stjernen på standarden.](assets/screenshots/storage/signins-light.png#only-light)

En mappe på denne serveren trenger ingen pålogging. SMB, WebDAV, SFTP, Storage Box og skystasjonene har sine i rclone-konfigurasjonen, som kortet **rclone.conf** på målet viser, så de dukker ikke opp i denne listen.

### SMB, WebDAV og andre rclone-remotes {#smb-webdav}

En Windows- eller Samba-deling og en WebDAV-server (Nextcloud, ownCloud, SharePoint eller en annen) er leverandørfliser i **Legg til mål**, under **NAS eller server**. Fyll ut verten og delingen (SMB) eller adressen og servertypen (WebDAV), brukeren og passordet, så skriver BombVault rclone-seksjonen for deg. rclone tilslører passordet før det lagres.

For en tjeneste ingen flis dekker, tar flisen **Rclone** under **Egen rclone-konfigurasjon** imot remotes limt inn fra en rclone.conf. De legges til remotene BombVault allerede har og erstatter dem aldri, og de lagres kryptert. For å sikkerhetskopiere et område til en av dem skriver du `rclone:<remote>:<folder>` som off-site-repository for det området, for eksempel `rclone:nas:backups/bombvault`. Delingen eller bucketen er det første stisegmentet, ikke en del av navnet.

Dette er den bedre veien enn å montere delingen på Unraid: restic advarer mot å ha et repository på en montert CIFS-deling, og her monteres ingenting. NFS har ingen flis fordi verken restic eller rclone har en NFS-backend. For NFS monterer du eksporten på verten og bruker den som en lokal mappe.

### Flytte og fjerne {#move-remove}

- **Flytt** tar kopien av ett område til et annet mål, med hele historikken. Alt blir i det gamle stedet. BombVault sletter ingenting der. Flyttingen kjører i bakgrunnen og kan ta lang tid: omtrent 9 minutter per 100 GB mellom lokale disker, timer over en treg linje. Oversikten viser den mens den pågår.
- **Fjern mål** tilbyr to måter. **Fjern mål, behold sikkerhetskopiene** slutter å skrive der og lar sikkerhetskopiene ligge. Du kan lese dem senere under **Gjenoppretting, Fra en gammel sikkerhetskopimappe**. **Fjern mål og slett sikkerhetskopiene der** leser først hva som er lagret der og lister det: hvilke sikkerhetskopier som går, hvilke av dem som ikke finnes noe annet sted, og hvilke repositoryer som blir fordi beskyttelse mot sletting nekter, elementer fortsatt sikkerhetskopierer rett til dem, et annet mål eller område har kopiene sine der, eller de ikke kunne leses. Elementer som ville miste sin eneste sikkerhetskopi, listes for seg og beholdes med mindre du slår på **Slett disse også**. Du bekrefter ved å skrive navnet på målet, og slettingen kjører i bakgrunnen.
- Dette virker for alle slags mål: ett som flere områder bruker, et mål satt opp for ett område, et repository du har satt opp, og mappen et område skriver sine egne sikkerhetskopier til. Elementer som sikkerhetskopierer til et fjernet repository, følger området sitt igjen, og det gjør også en områdestandard som pekte dit. Mappen til et område kan ikke bare forsvinne, for hvert område trenger en. Vinduet spør etter mappen områdene flytter til, og hvert område beholder mappenavnet sitt under den.
- Å slette sikkerhetskopiene på et mål krever et påloggingspassord og en pålogging på instansen. Uten passord kan målet fortsatt fjernes, bare sikkerhetskopiene kan ikke slettes herfra.

### Eksempel: en off-site-kopi på Backblaze B2 {#example-s3}

Containerne og auto-sikkerhetskopien skal også ligge hos Backblaze B2, så en død server ikke tar dem med seg. De samme trinnene passer for Wasabi, Hetzner Object Storage og de fleste andre S3-tjenester.

1. **I B2** oppretter du en privat bucket, for eksempel `tower-backups`, i en region nær deg. Under **Application Keys** oppretter du en nøkkel med lese- og skrivetilgang, begrenset til den bucketen, og noterer nøkkel-ID-en og applikasjonsnøkkelen. B2 viser applikasjonsnøkkelen én gang. Sett livssyklusen til bucketen til å beholde bare siste versjon, ellers beholder B2 beskårne data som skjulte versjoner og fakturerer deg for dem.
2. **Leverandør.** Åpne **Mål** i BombVault, trykk **Legg til mål** og velg **Backblaze B2** under **S3-tjeneste**.
3. **Logg inn.** Skriv nøkkel-ID-en som **Tilgangsnøkkel-ID** og applikasjonsnøkkelen som **Hemmelig tilgangsnøkkel**. Skriv endepunktet til bucketen under **Endepunkt**, for eksempel `s3.eu-central-003.backblazeb2.com`, så leser BombVault regionen fra det. Skriv `tower-backups` i **Bucket**, fordi denne nøkkelen bare åpner én bucket.
4. **Test tilkobling.** Den skal si **Forbindelsen er opprettet** og liste mappene i bucketen. Trykk **Ny mappe** og kall den `bombvault`. BombVault legger én mappe per område i den, for eksempel `bombvault/containers`.
5. **Beskyttelse.** Vinduet forklarer at B2 kan beholde gamle versjoner med objektlås og at BombVault ikke kan sjekke det. La **Beskyttelse mot sletting (append-only)** stå av med mindre bucketen virkelig nekter sletting, så BombVault kan beskjære etter oppbevaringsreglene til målet.
6. **For hvilke områder.** Huk av **Kontainere** og **Auto-sikkerhetskopi**. Auto-sikkerhetskopien bærer innstillingene dine, så en ny installasjon kan hente dem tilbake. Vinduet sier hvor mange elementer og, der det er kjent, hvor mye historikk den første kopien tar med.
7. Trykk **Lagre mål**.

Den første kopien kjører etter neste sikkerhetskopi. For å starte den nå åpner du **Innstillinger, Mål** og trykker **Replikér nå** i raden til området under **Eksterne kopier**. Statushodet sier **Off-site-kopi pågår**. Den første kopien tar med hele historikken, så 200 GB over en opplinje på 100 Mbit/s tar omtrent fire og en halv time. Senere kopier sender bare det som er nytt.

For å holde linjen din fri åpner du siden til målet og setter **Opplastingsgrense (KiB/s)** i kortet **Kopier**, for eksempel til `5120`, som leses som omtrent 5,0 MB/s (41,9 Mbit/s). Sjekk **Hvor lenge det beholdes** på samme side mens du er der.

For å se at det virket ser du på **Beskyttelsesstatus** på oversikten, der **Off-site** går over til **kopi oppdatert**, og på fanen **Sikkerhetskopier** til en container, der øyeblikksbildene nå nevner `tower-backups` som et andre sted. Trykk så **Last ned gjenopprettingssett** igjen under **Innstillinger, Gjenoppretting**, fordi settet lister det nye målet og påloggingen til det.

**Cloudflare R2 fungerer på samme måte,** med et objekttoken i stedet for en nøkkel. Lim inn S3-adressen R2-dashbordet viser, med bucketen på, så fyller BombVault ut serveren og bucketen.

### Eksempel: en rest-server du kjører selv {#example-rest-server}

En rest-server i append-only-modus nekter sletting, så en kopi på den overlever selv om noen tar over serveren din. En andre boks, hos en venn eller i garasjen, er nok. En paret BombVault kan sette dette opp for deg, se [Mottaksserver](#receiving-server). For å kjøre den for hånd på en hvilken som helst Docker-vert:

```sh
docker run -d --name rest-server --restart unless-stopped \
  -p 8000:8000 \
  -v /mnt/user/backups/rest-server:/data \
  -e OPTIONS="--append-only --private-repos" \
  restic/rest-server
docker exec -it rest-server create_user bombvault
```

Den siste linjen ber om et passord og skriver brukeren til filen `.htpasswd` i datamappen.

1. Trykk **Legg til mål** i BombVault og velg **rest-server** under **NAS eller server**.
2. Skriv adressen med brukeren som mappe, for eksempel `http://192.168.2.20:8000/bombvault`, og **Brukernavn** og **Passord**. Trykk **Test tilkobling**.
3. Slå på **Beskyttelse mot sletting (append-only)** i **Beskyttelse**. BombVault beskjærer da aldri der og overlater det til den andre siden.
4. Huk av områdene og trykk **Lagre mål**.
5. Når den første kopien er der, åpner du siden til målet og trykker **Test beskyttelsen mot sletting nå**. BombVault prøver å slette et testobjekt. Kortet skal si **Sist: sletting avvist**, og **Beskyttelse mot sletting** på oversikten går over til **kontrollert**. Hvis det står **Sist: sletting ikke avvist**, mangler `--append-only`.

Siden ingenting beskjærer denne kopien herfra, setter du **Vekstbudsjett (GB, 0 = av)** i vinduet til området, åpnet med **Rediger** i raden under **Eksterne kopier**, så du får høre om det før disken blir full. Med et selvsignert sertifikat, se merknaden om `certs` ovenfor.

## Et områdes egen mappe {#own-folder}

Hvert område har en egen mappe: repositoryet sikkerhetskopiene skrives til først. Kopier til andre mål går ut derfra. En ny installasjon har de egne mappene på denne serveren, under `/mnt/user/bombvault`, og sammen danner de målet **Lokal**.

En egen mappe kan også ligge på et mål du allerede har, som en NAS, en rest-server eller en S3-bucket. Området sikkerhetskopierer da rett dit, uten en lokal kopi imellom, og den egne mappen følger påloggingen, beskyttelsen mot sletting og båndbreddegrensene til det målet. BombVault beskjærer den etter hver sikkerhetskopi ut fra kortet **Hvor lenge det beholdes** for målet, og et områdes egne verdier under **Regler per område** går foran kortet. Målet lister området i kortet **Egne mapper**, og **Mål per område** viser målet som hjemmet til området.

### Flytte en egen mappe {#move-own-folder}

1. Åpne målet som skal ha den egne mappen og velg **Bruk som egen mappe**, eller **Legg til seksjoner** i kortet **Egne mapper**. For å flytte flere områder på én gang velger du **Flytt egne mapper** ved siden av **Egne mapper her** i kortet **Tilkobling** på målet Lokal.
2. Velg områdene under **Områder** og stedet under **Flytt til**. **Et annet sted** tar en mappe på denne verten eller adressen til et eksternt repository skrevet for hånd, med en pålogging fra de lagrede påloggingene dine.
3. Les **Hva som skjer**. For hvert område sier den hvor nye sikkerhetskopier går, om en kopi som allerede ligger på det nye stedet, blir den egne mappen, og hvilke sikkerhetskopier som kopieres over først.
4. **Kopier sikkerhetskopiene som allerede finnes** er på som standard. Er den slått av, blir de gamle sikkerhetskopiene der de er og kan fortsatt gjenopprettes derfra.
5. Velg **Bytt**. Når kortet **Hvor lenge det beholdes** på det nye stedet beholder mindre enn regelen området har i dag, spør BombVault først **Beholde mindre?** Der tar **Behold gjeldende verdier for** det området **under Regler per område** med dagens verdier, **Lagre likevel** går videre, og **Tilbake** lar alt være som det var. Når sikkerhetskopier må kopieres først, kopierer BombVault dem og bytter så den egne mappen. Når de allerede er der, sjekker den dem før byttet.

Etter flyttingen går nye sikkerhetskopier til den flyttede egne mappen, også for et område som hadde **Lokal** slått av og sikkerhetskopierte rett til en kopi. En kopi som allerede lå på det nye stedet, blir den egne mappen, og kopieringen dit tar slutt. Der kopien beholdt noen sikkerhetskopier lenger enn oppbevaringsreglene til området, beholder området det begge reglene beholder, så ingen av dem går tapt.

Når flyttingen ikke kan gå videre, sier vinduet hvorfor i en boks under **Hva som skjer**. Når bare kopieringen av de gamle sikkerhetskopiene står i veien, lar det den egne mappen flytte uten dem hvis du slår av **Kopier sikkerhetskopiene som allerede finnes**.

Et område trenger alltid en egen mappe. Å fjerne målet som har den, spør etter mappen området flytter til, se [Flytte og fjerne](#move-remove). En egen mappe som nye sikkerhetskopier går forbi, fordi området sikkerhetskopierer rett til en kopi, er merket **(av)** der et mål lister hvem som bruker den.

![Vinduet som flytter en egen mappe, med Hva som skjer utfylt for ett område.](assets/screenshots/storage/move-own-folder-dark.png#only-dark)
![Vinduet som flytter en egen mappe, med Hva som skjer utfylt for ett område.](assets/screenshots/storage/move-own-folder-light.png#only-light)

#### Eksempel: Kontainere flytter til en NAS {#example-own-folder}

Cache-poolen fylles opp, og du har et mål som heter `nas`, en SMB-deling på en andre boks. Området Kontainere har 61 sikkerhetskopier i den egne mappen på denne serveren og kopierer til Backblaze B2.

1. Åpne **Mål**, så `nas`, og trykk **Bruk som egen mappe**.
2. Huk av **Kontainere** under **Områder**.
3. Les **Hva som skjer**. Den sier at nye sikkerhetskopier av Kontainere går til mappen på `nas`, at de 61 sikkerhetskopiene i Lokal kopieres dit først, at kopier fortsetter til Backblaze B2, og at beskyttelse, båndbreddegrenser og vekstalarmen kommer fra `nas`.
4. La **Kopier sikkerhetskopiene som allerede finnes** stå på og trykk **Bytt**.

BombVault kopierer sikkerhetskopiene i bakgrunnen og bytter så den egne mappen. Siden til `nas` lister Kontainere under **Egne mapper**, et element som følger området sitt, nevner `nas` som hjemmet sitt, og neste sikkerhetskopi havner der, med kopien til B2 som går ut fra `nas`. Fra nå av er det kortet **Hvor lenge det beholdes** på `nas` som beskjærer mappen. Den gamle mappen på denne serveren blir der den er, og BombVault sletter ingenting der.

### Skrive en ekstern adresse for hånd {#remote-primary-repositories}

Kortet **Tilkobling** på målet Lokal tar fortsatt imot en ekstern adresse til den egne mappen til et område: **Endre** ved siden av området åpner et vindu med en bryter **Lokal / Ekstern**. **Ekstern** tar en restic-adresse som `s3:...`, `rest:http://host:8000/repo`, `sftp:user@host:/repo` eller `rclone:remote:bucket/path`, og **Sikkerhetsinnstillinger for ekstern primærrepo** legger til en tilkoblingstest, opplastings- og nedlastingsgrenser, append-only-beskyttelse med den samme tamper-testen som et off-site-mål får, og en vekstbudsjett-alarm.

En slik adresse logger inn med standardpåloggingen. Når den ligger i en mappe på et mål du har, lister siden til det målet området under **Ligger allerede under dette målet**, og å ta det over gir det pålogging, lagringsklasse og beskyttelse mot sletting fra målet. Å flytte den egne mappen til et mål med **Bruk som egen mappe** er den enklere veien, fordi vinduet sjekker sikkerhetskopiene som allerede finnes før noe endres.

## Plassering per element {#placement}

Kortet **Tidsplan og mål** på fanen Innstillinger til et element har en rad med knapper: **Lokal** og én knapp per off-site-mål for området. Tente knapper får sikkerhetskopiene til elementet.

- Med **Lokal** tent skrives elementet til repositoryet som vises under **Lagret på** og kopieres til hvert annet tent mål. Mørklegg et mål, så får det ikke noe nytt fra dette elementet. Bare Lokal kopierer ingen steder, og det passer for data som allerede har en andre kopi, for eksempel en deling som ligger på en NAS.
- Med **Lokal** mørk skrives elementet rett til det direkte repositoryet til det første tente målet og kopieres derfra til de andre tente. Første gang oppretter en dialog det direkte repositoryet.
- Én knapp blir tent, fordi en sikkerhetskopi trenger et sted å gå. For å utelate et element fra sikkerhetskopier bruker du **Ikke sikkerhetskopier**.

BombVault flytter aldri sikkerhetskopier mellom repositoryer. Et element som følger området sitt, går med når standarden til området endres: neste sikkerhetskopi havner på det nye stedet, og sikkerhetskopiene det har, blir der de er. Et mål valgt på selve elementet står fast fra den første sikkerhetskopien. Kortet **Repository** på fanen Innstillinger nevner det. Kopiene kan endres når som helst. Et mål som ikke får et element lenger, beholder kopiene det har og beskjærer dem til sin egen oppbevaring ved neste off-site-kjøring for området. Det kan ikke slettes fra append-only-mål.

Stedet et element forlot, beholder sikkerhetskopiene sine. På fanen **Sikkerhetskopier** til elementet og på **Gjenoppretting** står det som **Bare eldre sikkerhetskopier**, en gjenoppretting kan fortsatt lese fra det, og en prune der lar de sikkerhetskopiene være.

**Følg standarden** henter et element med et eget valg tilbake til området sitt. Den spør først, sier hvor nye sikkerhetskopier går da, og nevner hva hvert mål slutter å få og hva neste kjøring ville laste opp.

Under raden sier kortet hvor elementet går og hva som faktisk er der: hvor mange steder som har det, når hvert mål sist ble sett, og om 3-2-1 er oppfylt. Et sted er serveren med de opprinnelige dataene, hvert off-site-mål og hvert repository merket **Utenfor lokalet**. BombVault sjekker kopier og steder. Den sjekker ikke «to medier»-delen av 3-2-1.

### Standarder per område {#placement-defaults}

Kortet **Mål per område** på siden Mål tegner standarden til hvert område som en 3-2-1-flyt. Hver linje fra et område til et mål er en bryter, **Lokal** inkludert, for hvert område, også flashen, ZFS-datasett og auto-sikkerhetskopien. Et klikk på en linje eller en prikk slår den av eller på med en gang, og stiplede linjer er av. Bare å slå **Lokal** av for et område spør først, i den samme dialogen som standardkortet, fordi området da sikkerhetskopierer rett til en kopi. **Avbryt** lar alt være som det var. **Graf / Brytere** viser det samme som rader med knapper. En bryter avgjør hvor nye sikkerhetskopier går. Elementer uten eget valg, og prosjektmappene til Compose-stakker, følger med en gang, så neste sikkerhetskopi deres går til de nye stedene. Sikkerhetskopier som allerede er lagret, blir der de er og kan fortsatt gjenopprettes derfra. Flashen, ZFS-datasett og auto-sikkerhetskopien sikkerhetskopierer til den lokale mappen sin og kopierer off-site fra den sikkerhetskopien. Linjen deres til **Lokal** kan også slås av. Sikkerhetskopiene deres går da rett til off-site-målene, og et ZFS-element med en replika og ikke noe mål i det hele tatt sender bare replikaen sin.

Med **Lokal** av og bare off-site-mål på sikkerhetskopierer et område rett til dem, og BombVault velger repositoryet der selv. Containere, VM-er og mappesett går til det direkte repositoryet til det første målet som er på, beskrevet under, og kopieres derfra til de andre. Flashen, ZFS-datasett og auto-sikkerhetskopien skriver hver sikkerhetskopi til hvert mål som er på, og hvert mål beholder dem etter sin egen oppbevaring.

Et område med alle linjer av sier **Ikke sikkerhetskopiert**. Elementene får ingen sikkerhetskopier og teller som ubeskyttet.

Elementer som har valgt egne mål på siden sin, beholder dem. Flyten sier hvor mange det er i et område, for eksempel **2 elementer avviker**, og leder til dem: til fanen Innstillinger til elementet når det er ett, til en liste over hvor hvert sikkerhetskopierer når det er flere. **Følg standarden** der henter et element tilbake.

Et nytt off-site-mål får hvert element som ikke er satt til Lokal. Vinduet som legger det til, sier hvor mange elementer og, der det er kjent, hvor mye historikk det er.

### Direkte repoer {#direct-repositories}

Et direkte repository er der elementer sikkerhetskopierer rett til et off-site-mål. BombVault legger det ved siden av mappen til målet med `-direct` lagt til navnet, eller, når målet ligger i roten av bucketen sin, remoten sin eller en rest-server-bruker, i en mappe inni den oppkalt etter området, for eksempel `containers-direct`. I flyten på siden Mål opprettes det uten spørsmål. Å slå Lokal av for ett enkelt element, slik at et mål uten direkte repository blir hjemmet, åpner en dialog med det foreslåtte stedet, for eksempel `s3:https://s3.eu-central-003.backblazeb2.com/bucket/containers-direct`, og en tilkoblingstest som ikke oppretter noe. **Opprett og bruk** oppretter repositoryet og peker elementet på det. Et direkte repository har ingen egne innstillinger. Det følger bryteren, påloggingen, lagringsklassen, grensene, append-only-innstillingen og oppbevaringen til målet sitt og endres med dem. Siden til målet lister det under **Sikkerhetskopieres rett hit**, med hvert område som sikkerhetskopierer rett til det og hvor mange elementer som gjør det. Et som ingenting sikkerhetskopierer til lenger, sier det der og kan fjernes, og sikkerhetskopiene på stedet blir. Når en ny nøkkel til målet ikke kan åpne det, beholder det direkte repositoryet nøkkelen det har, og lagringen sier det. Et element på et direkte repository kopieres derfra til de andre tente målene, aldri til målet repositoryet hører til. Øyeblikksbildene har taggen `bv:direct`, og hver annen oppbevaringsrunde beholder dem, så et direkte repository som mistet koblingen til målet sitt, aldri eldes etter de lokale reglene. B2 nås gjennom S3-endepunktet sitt, med nøkkel-ID-en og applikasjonsnøkkelen skrevet som S3-nøklene. En nøkkel begrenset til målets egen mappe kan ikke nå mappen ved siden av, så begrens nøkkelen til mappen over i stedet.

### Utenfor lokalet {#off-the-premises}

Et mål eller et repository kan merkes **Utenfor lokalet**. Eksterne starter merket. Slå det av for en rest-server i samme bygning. Merket teller bare steder og 3-2-1 på kortene. Det endrer ingen kopi.

En ZFS-replika er en kopi utenfor lokalet, men ikke en sikkerhetskopi med historikk. Den beholder bare øyeblikksbildene du setter, og gamle versjoner og enkeltfiler kommer fortsatt fra sikkerhetskopiene. Den går fra én ZFS-server rett til den andre, over SSH til en ZFS-server eller til mottaksendepunktet til en paret instans. Paringsgruppen bærer bare forespørselen. Se [Replika](zfs-datasets.md#replica).

### Etter en ombygging {#after-a-rebuild}

Kopivalg ligger i BombVaults egne innstillinger. Etter en ombygging via **Oppdag sikkerhetskopier** eller Nødveiviser uten gjenopprettede innstillinger er de borte, og å kopiere alt ville sendt elementene du hadde utelatt, til B2 igjen. Off-site-kopier av hvert ombygd område settes derfor på pause. Oversikten sier det, og kortet **Mål per område** tilbyr **Bekreft standard** med en forhåndsvisning av hva neste kjøring kopierer og navnene i sikkerhetskopiene som ikke har noe element, som du kan utelate der. Bare bekreftelsen avslutter pausen. Å importere en innstillingsfil henter tilbake regler og standarder, men avslutter den ikke.

## Beskyttelse mot sletting (append-only) {#delete-protection-append-only}

Slå på beskyttelse mot sletting for et off-site-mål slik at løsepengevirus, eller en kompromittert vert, ikke kan slette eller skrive om sikkerhetskopiene dine. Den andre siden (en `restic/rest-server` som kjører i `--append-only`-modus) **håndhever** den. BombVault bare **verifiserer** den og viser aldri grønt på en konfigurasjonspåstand alene. En mappe på denne serveren kan ikke nekte sletting, fordi den som styrer BombVault også styrer mappen, så lokale mål tilbyr ikke bryteren.

Bryteren hører til målet: et område som kopierer dit, tar den fra målet, så slå den på fra siden til målet under **Mål**. Den planlagte **Kontroller slettebeskyttelse** kjører bare mens et område kopierer til et mål med bryteren på. Den andre siden må startes med `--append-only`. Med `restic/rest-server`-imaget, på TrueNAS som på enhver annen Docker-vert, går det i variabelen `OPTIONS`.

Noen steder kan ikke nekte en sletting i det hele tatt, en skystasjon som OneDrive for eksempel. Beholder du kopier der med vilje, slår du på **Godta uten beskyttelse mot sletting** på siden til målet. Oversikten viser da den manglende beskyttelsen mot sletting for et område som bare kopierer til godtatte mål som en akseptert risiko, en grå ring i stedet for en gul prikk, og fargen til området følger de andre kontrollene. Ellers endres ingenting: sikkerhetskopiene der kan fortsatt slettes, BombVault beskjærer dem fortsatt etter oppbevaringsreglene til målet, og bryteren forsvinner når beskyttelse mot sletting er på. En kopi som flyttes til et annet sted, mister godtakelsen. Å slå på beskyttelse mot sletting fjerner den også.

Den raskeste veien til en append-only-kopi er en paret BombVault med en [mottaksserver](#receiving-server): den kjører rest-serveren for deg, i append-only-modus.

!!! note "En vellykket sletting under `/locks/` er forventet"
    Append-only betyr ikke at ingenting noensinne kan fjernes. restic må ta og slippe sine egne låser, så `/locks/` kan med vilje skrives til og slettes. Øyeblikksbilder og dataene bak dem, som er det løsepengevirus ville gå etter, kan ikke fjernes. Hvis du selv tester den andre siden, er en sletting som lykkes under `/locks/` riktig oppførsel og ikke et hull i beskyttelsen.

!!! warning "Append-only-repoer beskjæres aldri fra denne boksen"
    Et append-only-mål beskjærer aldri gamle øyeblikksbilder herfra. Beskjæring er jobben til den andre siden. Sett en **vekstbudsjett-alarm** for det, så du får beskjed før repostørrelsen løper løpsk.

## Tamper-test {#tamper-test}

BombVault beviser jevnlig append-only-garantien ved faktisk å forsøke en sletting mot off-site-repoet, rettet mot et objekt som ikke finnes. **Test beskyttelsen mot sletting nå** på siden til et mål kjører den på forespørsel, og **Kontroller slettebeskyttelse** under **Innstillinger, Verifiser, Automatiske kontroller** kjører den etter en tidsplan.

Å slå **Kontroller slettebeskyttelse** av skjuler ikke et hull. Et mål med beskyttelse mot sletting som aldri er testet, leses da som «ikke bevist, kontrollen er slått av», og et mål uten beskyttelse mot sletting leses som «ingen» uansett.

En kopi som er slått av, testes ikke, og målet sier det. Tilkoblingstesten kjører likevel på den, fordi den bare leser, så en kopi kan sjekkes før den slås på.

- **Avvist** betyr beskyttet.
- **Godtatt** betyr ikke beskyttet.
- Et **uavklart** resultat (serveren ikke tilgjengelig, autentiseringsfeil) snur aldri den lagrede dommen.

Et ekte skifte fra beskyttet til ubeskyttet utløser én enkelt alarm.

## DR-øvelser {#dr-drills}

BombVault tilbyr to nivåer av bevis for at sikkerhetskopiene dine faktisk kan gjenopprettes, ikke bare finnes.

- **Øvelser som verifiserer gjenoppretting (lokalt).** BombVault kjører jevnlig `restic check --read-data-subset` (avgrenset, aldri en full gjenoppretting som fyller disken). Hyppigheten ligger under **Innstillinger, Verifiser, Automatiske kontroller**, og kortet **Kontroller sikkerhetskopier** på samme flis viser resultatet.
- **Prøvegjenoppretting (off-site).** BombVault gjenoppretter et ekte mål fra off-site-repoet til en avstengt sandkasse, verifiserer det fil for fil og byte for byte, og rydder så opp. Det beviser at du kan komme deg tilbake fra off-site, ikke bare at repoet svarer.

Kortet **Beskyttelsesstatus** på oversikten samler dette per område i fire kontroller: en gjeldende sikkerhetskopi på denne serveren, en kopi off-site, beskyttelse mot sletting der og en prøvegjenoppretting derfra. Et klikk på et område viser detaljene, og kortet blir bare grønt på verifiserte fakta. Et mål du har godtatt uten beskyttelse mot sletting, teller der som en akseptert risiko, ikke som en advarsel.

## Pare instanser {#pairing}

Flere BombVault-er jobber sammen som én gruppe: de viser hverandres beskyttelsesstatus, tar på seg roller for hverandre og kan oppbevare hverandres nøkkel. En instans blir med i gruppen med tolv ord.

![Parings-siden med sine tre trinnkort.](assets/screenshots/offsite/pairing-dark.png#only-dark)
![Parings-siden med sine tre trinnkort.](assets/screenshots/offsite/pairing-light.png#only-light)

På den første instansen åpner du **Innstillinger, Paring** og trykker **Generer frase**. Tre trinnkort viser hvordan det går. Tolv ord vises i et vindu med en knapp **Kopier**. På hver annen instans åpner du samme sted, trykker **Angi frase** og limer inn eller skriver dem, eller trykker **Lim inn** i det vinduet. Et ord som ikke står på listen, nevnes med plasseringen sin mens du skriver, og det siste ordet bærer en sjekksum, så et feilskrevet eller byttet ord oppdages før noe pares. Generer frasen på bare én instans: to instanser som begge genererer en, danner to separate grupper. Hvis ingen dukker opp på et minutt, tilbyr siden to utveier: vis ordene igjen for å skrive dem inn der borte, eller skriv inn ordene til den andre instansen og bli med i gruppen dens i ett steg. Paring virker uten påloggingspassord, men sett et: uten det kan alle som kan åpne webgrensesnittet, lese ordene og, via gruppen, be om roller som deler ut restic-passordet til instansene i den. Paringskortet sier det til et passord er satt. Med et passord må du oppgi det for å vise frasen igjen. **Forlat gruppen** tar en instans ut igjen. Kortet **Instanser** på parings-siden lister medlemmene i gruppen. Bildet av hvem som gjør hva for hvem, står på siden Instanser.

Alle som kan ordene, kan bli med i gruppen, så behandle dem som et passord.

**Hvordan medlemmene når hverandre.** Hver instans lærer sin egen adresse på nettverket fra nettleseren din i det du logger inn, listet under **Kjente adresser** i kortet **Denne instansen** på parings-siden sammen med de instansen fant selv. Rett den der hvis en omvendt proxy eller en uvanlig port står foran. På samme nettverk kunngjør medlemmene den adressen med multicast og snakker direkte, og der multicast ikke kan krysse et containernettverk, som Dockers standard bridge, søker en instans i stedet gjennom sitt eget subnett etter de andre med et signert kall bare et gruppemedlem kan svare på, så paringen blir likevel ferdig på sekunder uten relay. Hvis ingenting dukker opp, tar **Finner du den ikke?** én adresse for hånd, for et annet subnett eller en uvanlig port. Instanser på ulike nettverk går gjennom et relay, valgt i én rad med fire i kortet **Relay** på samme side. Hver form kommer med et bilde:

- **Prosjektrelay** (standard): `parleyport.halleluja.design`, relayet KnightLoader også bruker. Ingenting å sette opp.
- **Eget relay**: containeren [**ParleyPort**](https://github.com/junkerderprovinz/parleyport) fra Unraid Community Apps (også under **Innstillinger, Apper**). Skriv adressen på hver instans som skal bruke det.
- **BombVault-instans**: en av instansene dine som allerede kan nås utenfra, med **Fungere som relay** slått på. Den instansen svarer da på `/relay/connect` på sin egen adresse, bak den omvendte proxyen og det sertifikatet den allerede har, og slipper bare inn gruppen din. Skriv adressen på hver instans som skal bruke det.
- **Ingen relay**: medlemmene finner hverandre automatisk på samme nettverk, og ingen andre steder.

**Hva relayet ser.** En knapp med samme navn ved siden av de fire formene viser dette for formen du valgte. Hvert kall mellom medlemmer er forseglet med AES-256-GCM under en nøkkel utledet fra de tolv ordene, og den nøkkelen forlater aldri instansene dine. Relayet lærer en hash som grupperer forbindelsene, hvilken instans en melding er til, hvor stor den er og når den passerer. Et direkte kall på det lokale nettverket forsegles på samme måte og signeres i tillegg, så ingenting avhenger av det selvsignerte sertifikatet en instans leverer. Kall som deler ut noe (rollesvar, et restic-passord, mottakerpåloggingene, nøkkeldeponeringer) signeres også med en nøkkel fra medlemmet som sender dem, festet ved første øyekast. Se [Sikkerhetsmodell](configuration.md#security-model).

**Hva som går over gruppen.** Beskyttelsesstatusen på siden Instanser, en forespørsel om å sjekke ett område nå, rolleforespørsler og svarene på dem, lagringstilbud, og det en mottaker eller henter trenger når rollen er tillatt: repositoryplasseringene til den andre instansen og restic-passordet. Sikkerhetskopidata går aldri over den. De går fortsatt rett til restic-backendene. APP_KEY gjør det heller ikke: restic-passordet åpner repositoryene til den instansen og ingenting annet, ikke de lagrede hemmelighetene, øktene eller gjenopprettingskodene.

**Oppføringer fra før paringen.** Instanser lagt til med et flåte-token, og mottatte repositoryer og henting-kilder satt opp med APP_KEY til den andre instansen, blir etter oppdateringen og merkes **Par på nytt**. De fortsetter å virke: ved første oppstart erstatter BombVault hver lagrede APP_KEY med restic-passordet utledet fra den. Par begge instansene, rediger så oppføringen og velg instansen. Til da står slike oppføringer i kortet **Uten instans** på siden Instanser.

Det eneste stedet som fortsatt tar en APP_KEY for hånd, er [Fra en gammel sikkerhetskopimappe](#restore-from-another-bombvault-repo), for tilfellet der den andre instansen er borte og ikke kan svare i en gruppe.

## Instanser og roller {#roles}

Siden **Instanser** (en innstillingsflis som kan festes i sidefeltet) viser gruppen som et nett over hvem som gjør hva for hvem, et kort for denne instansen og et kort for hvert annet medlem, Android-telefonene og ZFS-serverne. **Paring** nede til høyre leder til parings-siden. Hvilke deler som er på, settes under **Innstillinger, Instanser, Flere servere**: **Instanser**, **Mottaker** og **Henting**.

![Siden Instanser: gruppen som et nett, og et kort per instans med rollefliser, en av dem Inaktiv.](assets/screenshots/offsite/instances-dark.png#only-dark)
![Siden Instanser: gruppen som et nett, og et kort per instans med rollefliser, en av dem Inaktiv.](assets/screenshots/offsite/instances-light.png#only-light)

Hvert kort viser beskyttelsesstatusen til medlemmet og rollene som fliser. En rolle er noe én instans gjør for en annen:

- **Mottaker** oppbevarer off-site-kopiene til denne serveren i en rest-server i append-only-modus. Denne serveren sender etter hver sikkerhetskopi og kan ikke slette noe der, selv om noen tar den over.
- **Henter** henter sikkerhetskopiene til denne serveren etter sin egen tidsplan. Denne serveren trenger ikke være våken for det.
- **ZFS-server** tar imot ZFS-replikaene til denne serveren. Hvert ZFS-element velger den under **Replica** i innstillingene sine.
- **Nøkkelvokter** oppbevarer en forseglet kopi av gjenopprettingssettet til denne serveren. Se [Nøkkelvokter](#key-keeper).

Å slå en rolle på sender en forespørsel, og ingenting starter før noen på den andre instansen trykker **Tillat**. Forespørselen vises på kortet til den instansen som et merke, og **Detaljer** lister den med **Tillat** og **Avslå**. En tillatt rolle kan trekkes tilbake der når som helst. En stiplet linje i nettet er en rolle som fortsatt venter. En instans på BombVault 9.x kan ikke svare på en forespørsel, så en rolle er aktiv der så snart du slår den på, og en forespørsel fra en 9.x-instans venter på **Tillat** her.

**Detaljer** på et kort åpner resten: de mottatte repositoryene og henting-kildene til det medlemmet, lagringstilbud, overvåking, **Fjern** og **Glem nøkkelen**.

Et medlem kan tilby sin egen off-site-lagring til et annet medlem over gruppen. Den andre administratoren ser tilbudet i **Detaljer** til medlemmet og godtar eller avslår det. Å godta oppretter en lagret pålogging og et vanlig mål, etter de samme kontrollene som for et mål du skriver inn. Bare tilkoblingsdetaljer går denne veien, aldri sikkerhetskopidata. **Sjekk nå** ber et medlem verifisere repositoryet til ett område. Ingenting på siden kan starte en sikkerhetskopi, gjenoppretting eller sletting på en annen boks.

Instanser som er slått av og telefoner med appen lukket, beholder kortet sitt som **Inaktiv**, i grått, med når de sist ble sett, og står dempet i nettet til du **Fjern**er dem. Rett etter at denne serveren starter, sier et slikt kort **Sjekker** til gruppen er tilbake. **Fjern** virker på et slikt kort uansett hvilke deler som er slått på, og den avslutter også de åpne forespørslene til medlemmet, de ledige ZFS-mottaksplassene og de ventende nøkkelforespørslene. Kortet **Instanser** på parings-siden lister disse medlemmene som **Inaktiv** også.

## Mottaker og henter {#receiver}

På boksen som **mottar** append-only-kopier fra en annen BombVault gir de mottatte repositoryene deg uavhengig, skrivebeskyttet overvåking på mottaksmaskinvaren, så en stille feil i den andre enden ikke går ubemerket. Slå på **Mottaker** under **Innstillinger, Instanser**, åpne så **Detaljer** til den sendende instansen på siden Instanser og legg til det mottatte repositoryet (skrivebeskyttet, åpnet med restic-passordet til den sendende instansen, som det får over [paringsgruppen](#pairing)). Du får:

- **En oversikt per kilde**, så du ser nøyaktig hvilke containere, VM-er og mappesett som har kommet.
- **Sist mottatt** per kilde, så du vet hvor fersk hver er.
- **En uavhengig `restic check`** kjørt på mottaksmaskinvaren, så integriteten verifiseres der dataene faktisk ligger, ikke bare hos avsenderen.
- **En dødmannsbryter:** en alarm når en kilde slutter å sende innenfor et vindu du setter.
- **Integritetsalarmer:** en alarm når en kontroll på mottakssiden feiler.

Mottakeren er strengt skrivebeskyttet. Den skriver aldri til det mottatte repositoryet, så den kan aldri bryte append-only-garantien avsenderen stoler på.

En henter virker den andre veien: med **Henting** slått på leser en instans sikkerhetskopiene til et annet medlem etter sin egen tidsplan, inn i et eget repository for hver kilde, så de aldri teller som sikkerhetskopiene til denne boksen. Kildene står også i detaljene til det medlemmet: repositoryplasseringen, hvilket slag sikkerhetskopi det har og hvor ofte det skal hentes. Restic-passordet kommer over gruppen, aldri APP_KEY, og den andre siden har ikke noe mer å sette opp og trenger ikke kjøre. Kilden leses bare, og hver side beholder sin egen pålogging. En `rclone:`-kilde avvises, fordi rclone ville nå den med remotene til denne instansen.

### Mottaksserver {#receiving-server}

Mottaksboksen kan også kjøre rest-serveren de andre kopierer til. Den settes opp første gang du tillater en **Mottaker**-forespørsel som ber om det: vinduet sier at serveren trenger en mottaksserver og ber om en mappe på en deling, med **Ny mappe** for å lage en, og en port (8000 med mindre en annen container bruker den). BombVault da:

1. nekter hvis en container som heter `rest-server` allerede finnes eller en annen container har porten,
2. henter `restic/rest-server` og starter den gjennom Docker-socketen i append-only-modus med private repositoryer og en påloggingsfil i mappen,
3. skriver Unraid-malen sin til flash-stasjonen, så containeren kan redigeres i Docker-fanen, eller tilbyr malen som nedlasting når flash-stasjonen er utenfor rekkevidde,
4. kjører tamper-testen mot den og viser om den nekter sletting.

Instansene i gruppen din finner så serveren blant leverandørene under **Fra gruppen din**, oppkalt etter mottaksboksen, og rollen **Mottaker** tilbyr den som det trygge valget. Hver instans får en egen pålogging første gang den bruker serveren, og skriver bare inn i sin egen mappe der. Med **Mottaker** slått på under **Innstillinger, Instanser** åpner **Mottaksserver** på kortet til denne instansen serveren igjen. Vinduet lister påloggingene under **Partnere med pålogging**, og **Trekk tilbake pålogging** tar en bort. Det den instansen allerede har kopiert, blir i mappen. Oppsettet oppretter også én **Pålogging utenfor gruppen**, hvis passord vinduet viser én gang.

En instans som bare når mottaksboksen gjennom relayet, kan ikke bruke serveren, fordi relayet ikke bærer sikkerhetskopier. Legg først til adressen til mottaksboksen på parings-siden. Når BombVault kjører på en egen IP-adresse (på br0, for eksempel), fyll ut **Adresse for partnere**, fordi serveren lytter på adressen til verten.

## Eksempel: pare to Unraid-bokser {#worked-example-two-unraid-boxes-end-to-end}

Dette er ett komplett oppsett av delene ovenfor, fordi de er lettere å sette sammen når du har sett dem satt sammen.

To bokser: **TOWER** kjører containerne og sender sikkerhetskopier. **VAULT** mottar dem og håndhever append-only. Bytt inn dine egne navn, adresser og delingsstier.

**1. Par de to boksene.** På TOWER åpner du **Innstillinger, Paring**, trykker **Generer frase** og skriver de tolv ordene på VAULT under **Angi frase**. Sett et påloggingspassord på begge først.

**2. På TOWER ber du VAULT være mottaker.** På siden Instanser slår du på **Mottaker** på kortet til VAULT, velger områdene og **mottaksserveren til VAULT** som sted, og sender forespørselen.

**3. På VAULT tillater du den.** Siden Instanser på VAULT viser forespørselen på kortet til TOWER. Trykk **Tillat**. Første gang spør VAULT etter en mappe til mottaksserveren, for eksempel `user/appdata/rest-server`, starter rest-serveren i append-only-modus og kjører tamper-testen mot den.

**4. Tilbake på TOWER.** Fra da av kopierer TOWER til VAULT etter hver sikkerhetskopi, og det nye målet vises på siden Mål på TOWER med beskyttelsen mot sletting.

Hva tamper-testen svarer:

| Resultat | Hva som skjedde |
| --- | --- |
| **beskyttet** | VAULT avviste slettingen. Dette er den eneste bestått-tilstanden. |
| **ikke beskyttet** | VAULT godtok en sletting. `--append-only` mangler eller ble fjernet. |
| **uavklart** | Ingen av delene. Som regel er URL-en ikke den restic selv bruker, eller påloggingsopplysningene er endret. Ingenting registreres og ingen alarm utløses. |

**5. Følg med på det som kommer, hvis du vil.** Slå på **Mottaker** under **Innstillinger, Instanser** på VAULT, åpne **Detaljer** til TOWER og legg til det mottatte repositoryet. VAULT får restic-passordet til TOWER over gruppen. Ingen skriver en nøkkel.

**6. Gjør det gjensidig.** Gjenta trinnene i den andre retningen: VAULT ber TOWER være mottakeren sin. Hver boks håndhever da append-only for den andre, og ingen av dem kan slette den andres sikkerhetskopier.

En rest-server du kjører for hånd, virker også: legg den til med **Legg til mål** som en rest-server, med brukeren og passordet fra `.htpasswd`, og slå på beskyttelse mot sletting. Behold `--append-only` i valgene. Uten det er den andre siden en vanlig deling igjen.

!!! warning "En lokal sti er en sti inni containeren"
    Der BombVault ber om en mappe på denne serveren, skriver du den relativt til vertsmonteringen, for eksempel `user/appdata/rest-server/bombvault-containers`, ikke `/mnt/user/appdata/...`. BombVault kjører i en container, der `/mnt` på verten er montert et annet sted. En absolutt vertssti finnes ikke der inne. Hvis du limer inn en, sier BombVault hvilken relativ sti du skal bruke i stedet.

## Nødveiviser {#emergency}

**Nødveiviser** henter tilbake en server som er borte eller ble satt opp på nytt. Den starter på siden **Gjenoppretting** under **Nød**, og fra **Gjenopprett fra en sikkerhetskopi** på velkomstsiden til en ny installasjon. Den går i fire trinn: **Tilgang**, **Innstillinger**, **Utvalg** og **Gjenoppretting**.

1. **Tilgang.** Vis BombVault nøkkelen til de gamle sikkerhetskopiene, på en av tre måter:
    - **Last opp gjenopprettingssett**: filen du lastet ned under oppsettet. Den inneholder nøkkelen og hvert mål, og BombVault fyller ut resten fra den. Et sett som er forseglet med age, ber om identiteten det ble forseglet for. Den brukes én gang og lagres ikke.
    - **Via gruppen**: en instans i gruppen din oppbevarer nøkkelen som en deponering. Du trenger de tolv ordene til gruppen, gjenopprettingskoden din og noen på den instansen. Se [Nøkkelvokter](#key-keeper).
    - **Skriv inn nøkkelen**: `APP_KEY` til den gamle serveren, og hvor sikkerhetskopiene ligger.

    BombVault prøver så nøkkelen på hvert mål og sier for hvert om det er lesbart, ikke funnet, låst med en annen nøkkel, ikke tilgjengelig eller fortsatt mangler pålogging. Ingenting er lagret ennå, og nøkkelen blir i minnet en begrenset tid. **Overta nøkkelen** får denne serveren til å skrive sikkerhetskopiene sine med den nøkkelen fra nå av. **Angre overtakelsen** går tilbake til serverens egen nøkkel til den første sikkerhetskopien har kjørt. En server som allerede har egne sikkerhetskopier, avviser overtakelsen og peker til [Fra en gammel sikkerhetskopimappe](#restore-from-another-bombvault-repo) i stedet. **Lagre påloggingsopplysninger** lagrer påloggingene settet har med til eksterne mål, og spør før det erstatter påloggingene denne serveren allerede har.
2. **Innstillinger.** **Hent tilbake innstillinger** gjenoppretter BombVaults egne innstillinger fra den nyeste auto-sikkerhetskopien: mål, pålogginger, tidsplaner og regler. BombVault starter på nytt for å bruke dem, gjennom Docker-socketen, så den levende innstillingsdatabasen aldri skrives over under et åpent håndtak, og veiviseren fortsetter av seg selv etterpå. Påloggingen til denne serveren blir: du logger fortsatt inn med passordet du bruker nå. Hemmeligheter den gamle serveren hadde lagret og som ikke kan overtas, listes, så du kan skrive dem inn igjen. Gjenopprettingen avslutter alle økter og fjerner passnøklene, så sett dem opp igjen etterpå.
3. **Utvalg.** BombVault går gjennom sikkerhetskopiene dine og lister containerne, VM-ene, mappesettene og ZFS-datasettene den finner. Det nyeste øyeblikksbildet er forhåndsvalgt overalt. Det denne serveren fortsatt har, starter avslått, fordi en gjenoppretting ville erstattet det. Mappesett vet ikke lenger hvor de kom fra etter en ombygging, så de går til en mappe du velger.
4. **Gjenoppretting.** Alt som er valgt, kommer tilbake på én gang, og blir stående stoppet hvis du vil starte hver container og VM selv, noe som er tryggest etter et totalt tap. Last etterpå ned et nytt gjenopprettingssett: det gamle hører til den forrige installasjonen.

![Nødveiviser på det første trinnet, med de tre veiene inn.](assets/screenshots/offsite/emergency-dark.png#only-dark)
![Nødveiviser på det første trinnet, med de tre veiene inn.](assets/screenshots/offsite/emergency-light.png#only-light)

### Eksempel: en ny server fra gjenopprettingssettet {#example-emergency}

Serveren TOWER døde sammen med diskene sine. Den hadde 23 containere og 2 VM-er, og kopierte dem og auto-sikkerhetskopien til Backblaze B2. Du har `bombvault-recovery-kit.md` fra passordbehandleren og en ny boks med Unraid på. Den gamle `APP_KEY`-en er borte, og du trenger den ikke.

1. **Installer BombVault** på den nye boksen og gi den en ny `APP_KEY`, som på enhver ny installasjon. Sett opp VM-koblingen under **Innstillinger, Integrasjoner, Verts-SSH** hvis du vil ha VM-ene tilbake, fordi en VM-gjenoppretting trenger den.
2. **Start veiviseren.** På **Velkommen til BombVault** velger du **Gjenopprett fra en sikkerhetskopi**. På siden Gjenoppretting velger du **Start nødveiviseren** under **Nød**.
3. **Tilgang.** Velg **Last opp gjenopprettingssett** og slipp filen. BombVault prøver nøkkelen på hvert mål og sier for eksempel **Gjenopprettingssettet er lest. Nøkkelen i det åpner 0 av 2 mål.** Den lokale mappen sier **ikke funnet**, siden disken døde, og Backblaze B2 sier **pålogging mangler**.
4. **Lagre påloggingsopplysninger.** Settet har med påloggingen til de eksterne målene. Trykk **Lagre påloggingsopplysninger**, så prøves målene igjen, og Backblaze B2 sier nå **lesbar**.
5. **Overta nøkkelen.** Denne serveren skriver nå sikkerhetskopiene sine med den gamle nøkkelen, så de nye sikkerhetskopiene passer til de gamle. Den beholder sin egen `APP_KEY` i malen. **Angre overtakelsen** virker til den første sikkerhetskopien har kjørt.
6. **Innstillinger.** Velg **Hent tilbake innstillinger**, som leser den nyeste auto-sikkerhetskopien fra B2. BombVault starter på nytt for å bruke den, og veiviseren fortsetter av seg selv. Målene, tidsplanene og reglene dine er tilbake, og du logger fortsatt inn med passordet til denne serveren. Skriv inn hemmelighetene veiviseren lister igjen, for eksempel varslingsinnstillinger.
7. **Utvalg.** Sett **Hvor ligger sikkerhetskopiene?** til off-site-kopiene, fordi de lokale gikk ned med den gamle serveren. BombVault lister de 23 containerne og de 2 VM-ene med det nyeste øyeblikksbildet forhåndsvalgt, og du kan velge et annet for hver av dem.
8. **Gjenoppretting.** Slå på **La stå stoppet etterpå**, som er tryggest etter et totalt tap, og start. **Hva som skjer** sier hva som kommer tilbake. Resultatlinjen lyder for eksempel **25 gjenopprettet, 0 feilet. Start dem under Sikkerhetskopier ved behov.**
9. **Avslutt.** Start databasene dine først, så appene som bruker dem. Åpne **Innstillinger, Gjenoppretting** og last ned et nytt gjenopprettingssett, fordi det gamle hører til den forrige installasjonen. Sjekk **Krever oppmerksomhet** på oversikten, og kjør **Sikkerhetskopier alt nå** én gang når alt virker.

Mangler auto-sikkerhetskopien også, velger du **Hopp over, jeg har ingen auto-sikkerhetskopi**. Målene kommer da fra settet alene, du setter opp tidsplaner og regler på nytt, og off-site-kopier står på pause til du bekrefter områdestandardene, se [Etter en ombygging](#after-a-rebuild).

!!! tip "Planlagt flytting kontra katastrofe"
    Nødveiviser gjenoppretter BombVaults egne innstillinger fra en sikkerhetskopi. For en *planlagt* flytting til en ny boks kan du i stedet ta med konfigurasjonen direkte med kortet **Eksporter / importer innstillinger** (en portabel JSON-fil). Se [Konfigurasjon](configuration.md#portable-settings-export-and-import).

### Fra en gammel sikkerhetskopimappe {#restore-from-another-bombvault-repo}

**Fra en gammel sikkerhetskopimappe** under **Andre kilder** på siden Gjenoppretting åpner repositoryet til en *annen* BombVault-instans (en deling montert under `/mnt`, eller en ekstern adresse) med gjenopprettingssettet til den instansen eller den innskrevne `APP_KEY`-en, i en engangs, skrivebeskyttet økt. Bla gjennom containerne, VM-ene, mappesettene og ZFS-datasettene som er lagret der, velg et øyeblikksbilde og gjenopprett det, så blir det gjenopprettede objektet en vanlig lokal container, VM eller et mappesett. Ingenting skrives noensinne til det andre repositoryet, og dine egne sikkerhetskopiinnstillinger forblir urørt (økten lever i minnet og utløper av seg selv). Å flytte en container fra server A til server B betyr ikke at du peker innstillingene dine om og tilbakestiller dem etterpå. Dette vinduet er et engangsvindu: det åpner en økt, gjenoppretter det du velger og glemmer den andre instansen. Vil du heller ha en varig ordning, der denne boksen henter øyeblikksbildene til en annen instans inn i sitt eget repository etter en tidsplan, er det rollen **Henter** på siden Instanser.

En container hvis nettverk ikke finnes på denne serveren, som et Unraid-`br0`-nettverk på en vanlig Docker-vert, viser en nettverksvelger under raden sin. BombVault oppretter den på nettverket du velger, sammen med de andre nettverkene sine. Den faste IP-en og MAC-adressen hørte til det gamle nettverket og fjernes, så det nye nettverket tildeler dem.

## Nøkkelvokter {#key-keeper}

En nøkkelvokter er en instans i gruppen din som oppbevarer en forseglet kopi av gjenopprettingssettet til denne serveren. Den kan ikke åpne kopien selv. Hvis denne serveren noen gang går tapt, ber en ny installasjon som blir med i gruppen din, vokteren om kopien og åpner den med gjenopprettingskoden din.

![Vinduet med gjenopprettingskoden: tjue tegn i fire grupper, vist én gang.](assets/screenshots/offsite/recovery-code-dark.png#only-dark)
![Vinduet med gjenopprettingskoden: tjue tegn i fire grupper, vist én gang.](assets/screenshots/offsite/recovery-code-light.png#only-light)

1. På siden Instanser slår du på **Nøkkelvokter** på kortet til instansen som skal oppbevare den, og velger **Deponer**. Å deponere krever et påloggingspassord.
2. Denne serveren viser **Frigivelseskode** én gang: tjue bokstaver og sifre i fire grupper på fem. Skriv den ned og bekreft med **Jeg har skrevet ned koden**. Denne serveren lagrer ikke koden, bare det den trenger for å forsegle en deponering for den.
3. Vokteren har nå deponeringen, og denne serveren fornyer den når settet endres. **Nøkkeldeponering** på kortet til denne instansen lister **Deponeringen til denne serveren** med hver vokter og tilstanden deres, og **Deponeringer denne serveren oppbevarer** for de andre. **Lag en ny kode** erstatter koden. Hver vokter bytter til den nye når den får den nye deponeringen.

Den dagen trenger du tre ting: de tolv ordene til gruppen din, gjenopprettingskoden og tilgang til grensesnittet til vokteren, der noen må trykke **Tillat**. I Nødveiviser velger du **Via gruppen**, velger deponeringen, ber om den og skriver gjenopprettingskoden. Koden brukes på den nye serveren og sendes ikke til vokteren. På vokteren vises forespørselen under **Nøkkeldeponering** med hvem som spør, om den instansen noen gang er sett der før og hvor ofte den har spurt. **Tillat** deler ut den forseglede kopien, og å tillate krever et påloggingspassord. Den nye installasjonen har 30 minutter på å hente den etter Tillat, en forespørsel ingen svarer på, utløper etter et døgn, og en instans som spør for ofte, må vente en time.

Uten koden kan ingen åpne kopien, vokteren inkludert. Er koden tapt, lag en ny mens denne serveren fortsatt kjører. Er serveren allerede borte, virker gjenopprettingssettet og den innskrevne APP_KEY-en fortsatt. Vokteren kan slette en deponering når som helst, og eieren må da spørre på nytt.

## Gjenopprettingssett {#recovery-kit}

Dette er delen som gjør katastrofegjenoppretting mulig selv når ingen BombVault kjører.

**Last ned gjenopprettingssett** i kortet **For nødstilfeller** under **Innstillinger, Gjenoppretting** gir deg **nøkkelen**, det **utledede restic-passordet**, hvert **mål** med repositoryet til hvert område, påloggingene en ny installasjon trenger for å nå dem, og de **nøyaktige restic-kommandoene**, så du kan gjenopprette rett med restic-CLI-en på en hvilken som helst maskin. Det krever et påloggingspassord. Settet skriver ut nøkkelen også med kryptering slått av, fordi container- og VM-definisjonene alltid er forseglet med den. Etter en overtakelse skriver det ut sikkerhetskopinøkkelen som er overtatt og APP_KEY til denne serveren, som en ny installasjon bare trenger for å hente tilbake innstillingene. Et kort **Lagre gjenopprettingssettet ditt** på oversikten minner deg på det til du har lagret det.

!!! danger "Lagre gjenopprettingssettet utenfor serveren"
    Settet inneholder hemmeligheten som dekrypterer sikkerhetskopiene dine. Oppbevar det et trygt sted, adskilt fra serveren (en passordbehandler, en utskrift i en safe). Mister du både BombVault og `APP_KEY` uten gjenopprettingssett og uten nøkkeldeponering, kan de krypterte sikkerhetskopiene dine ikke gjenopprettes.

!!! warning "Det nyeste øyeblikksbildet er ikke alltid det du skal gjenopprette"
    Siden restic 0.17 viser `restic snapshots` størrelsen på hvert øyeblikksbilde. Etter datatap kan det nyeste øyeblikksbildet være det tømte, så ikke gjenopprett et øyeblikksbilde som er mye mindre enn de før. Etter løsepengevirus kan det være det krypterte med vanlig størrelse. Kjører BombVault fortsatt, sjekk siden **Avvik** først: den nevner den siste gode sikkerhetskopien, og gjenopprettingsvinduet forhåndsvelger den. En gjenoppretting trenger ingen av avviksdataene til BombVault, og oppbevaringspausen beholder bare flere øyeblikksbilder.

### Forsegle settet {#sealing-the-kit}

Har du slått på **Krypter eksporter med age** under **Innstillinger, System**, forsegles settet også med den og lastes ned som `bombvault-recovery-kit.md.age`. Det er ASCII-armert i stedet for binært, så det er fortsatt ren tekst: å lime det inn i en passordbehandler eller skrive det ut virker akkurat som før, innholdet er uleselig uten nøkkelen din.

!!! warning "Ikke lagre age-nøkkelen inni settet"
    Du trenger den **private** age-nøkkelen din for å åpne et forseglet sett. Oppbevar den et sted som ikke avhenger av selve settet, ellers har du to ting å gjenopprette i stedet for én. Å forsegle er verdt det når settet lagres et sted du ikke kontrollerer helt (en delt passordbehandler, skynotater, en utskrift på et kontor). Et sett i din egen safe er allerede beskyttet av safen.

    Med kryptering på og ingen brukbar mottaker satt opp nektes nedlastingen rett ut. BombVault faller aldri tilbake til å dele ut hovednøkkelen i klartekst.

### Hvis du ikke har settet for hånden {#if-you-do-not-have-the-kit-to-hand}

Passordet lagres ikke noe sted, det **beregnes** fra nøkkelen til sikkerhetskopiene, så du kan gjenskape det selv med ingenting annet enn nøkkelen og et skall:

```sh
printf 'bombvault:restic-repo'   | openssl dgst -sha256 -mac HMAC -macopt hexkey:$APP_KEY -r   | cut -d' ' -f1
```

Det er HMAC-SHA256 over den faste strengen `bombvault:restic-repo`, med de rå bytene til den heksadesimale `APP_KEY` som nøkkel, skrevet ut som 64 små heksadesimale tegn. Den samme verdien står i settet, oppført som det utledede restic-passordet. Dette er til den dagen settet er et sted du ikke er. På en server som har overtatt sikkerhetskopiene til en annen, bruker du sikkerhetskopinøkkelen som er overtatt, som er den gamle APP_KEY-en til den serveren.

!!! warning "For et mottatt repository bruker du nøkkelen til den SENDENDE instansen"
    Et repository som kom hit gjennom en off-site-kopi, ble opprettet av maskinen som sendte det, med **dens** nøkkel. Å utlede fra nøkkelen til mottaksboksen gir et passord restic avviser, noe som ser nøyaktig ut som et ødelagt repository og ikke er det. Dette er den vanlige grunnen til at `restic check` på et mottatt repo ber om passord igjen og igjen.

Fordi gjenopprettingsdefinisjonene ligger **inni** hvert repo (`<repo>/def`, `<repo>/vm-def`), er en kopiert repo-mappe helt selvstendig, så settet pluss repoet er alt en gjenoppretting på bar maskinvare trenger.

## Hente en databasedump tilbake {#database-dumps}

En databasedump er et eget gjenopprettingspunkt i repositoryet for containere, merket `dbdump:<container>`, med den ene filen `/dbdump/<container>.sql`. BombVault lister, laster ned og importerer dem på fanen **Sikkerhetskopier** til containeren. Under står de samme trinnene med bare restic, til den dagen BombVault ikke er der.

```sh
restic -r <repo> snapshots --tag dbdump:<container>
restic -r <repo> dump --tag dbdump:<container> latest /dbdump/<container>.sql > <container>.sql
```

Taggene `dbversion:` og `dbname:` på hver dump sier hvilken serverversjon den kom fra og hvilke databaser den har. En komplett fil ender med `-- PostgreSQL database cluster dump complete` eller `-- Dump completed`.

Importer den i en container med samme eller en nyere versjon (PostgreSQL), eller samme hovedversjon (MySQL og MariaDB), startet én gang med en tom datamappe så den initialiserer seg. Verten trenger ingen databaseklient, containeren har en:

```sh
docker exec -i <container> sh -c 'exec psql -X -U "${POSTGRES_USER:-postgres}" -d postgres' < <container>.sql
docker exec -i <container> sh -c 'exec mariadb -uroot -p"$MARIADB_ROOT_PASSWORD"' < <container>.sql
docker exec -i <container> sh -c 'exec mysql -uroot -p"$MYSQL_ROOT_PASSWORD"' < <container>.sql
```

For én enkelt database fra en full dump tar MySQL og MariaDB `--one-database <name>` på klientkommandoen. En PostgreSQL-dump har én seksjon per database, hver begynner med en linje `\connect <name>`: kopier den seksjonen til en egen fil og importer den med `-d <name>` etter at du har opprettet databasen.

!!! warning "En root-dump har med brukerne til serveren"
    En full MySQL- eller MariaDB-dump tatt som root inneholder systemdatabasen `mysql`, så å importere den erstatter kontoene til den nye serveren, root-passordet inkludert, med de fra dumpen. På PostgreSQL er `role ... already exists` for brukeren containeren opprettet, forventet og harmløst.
