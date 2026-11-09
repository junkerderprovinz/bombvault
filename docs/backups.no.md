# Sikkerhetskopier og elementer

Alt BombVault tar sikkerhetskopi av står i én liste under **Sikkerhetskopier**: containere, VM-er, mappesamlinger, flash-disken, ZFS-datasett og auto-sikkerhetskopien. Hver av dem er et **element** med en egen side. Denne siden går gjennom listen, elementsiden og dens tre faner, og innstillingene et element kan ha.

![Listen Sikkerhetskopier med flisen Kontainere valgt: hver rad viser de siste kjøringene som en stripe og én status.](assets/screenshots/backups/list-dark.png#only-dark)
![Listen Sikkerhetskopier med flisen Kontainere valgt: hver rad viser de siste kjøringene som en stripe og én status.](assets/screenshots/backups/list-light.png#only-light)

## Hva hvert område lagrer {#what-is-saved}

| Område | Hva som lagres |
|---|---|
| **Kontainere** | Appdata-mappen pluss containerdefinisjonen: image, variabler, porter, etiketter og volumer. Kortet **Sikkerhetskopierte mapper** på containerens fane **Innstillinger** krysser av nøyaktig hvilke mapper sikkerhetskopien dekker, med en levende telling av stiene og en liste over det du utelot. Data en container har utenfor Host Data, for eksempel i et TrueNAS-ixVolume eller et Compose-prosjekt under `/opt`, sikkerhetskopieres ikke, og kjøringen nevner mappene den utelot. Kjøringen nevner også en mappe i Host Data som ikke finnes akkurat nå. Systemstier som `/etc/localtime` eller Docker-socketen regnes ikke som data. |
| **VM-er** | Diskbildene, XML-definisjonen og UEFI-NVRAM, med en pen nedstenging eller et live-øyeblikksbilde, over SSH. Se [VM-sikkerhetskopier](vm-backup-ssh-setup.md). |
| **Flash** | Bare Unraid. Hele USB-flashen (`/boot`): operativsystem, lisens, array-konfigurasjon, delinger, nettverk og programtilleggskonfigurasjon. |
| **Auto-sikkerhetskopi** | BombVaults egen `/config`: innstillingsdatabasen, påloggingene til målene, libvirt-SSH-nøkkelparet, nøkkelen og de festede vertsnøklene til ZFS-serverne og sertifikatene, tatt som øyeblikksbilde med SQLite `VACUUM INTO`, så databasen aldri fanges midt i en skriving. |
| **Mapper** | Hvilken som helst mappe på serveren som en mappesamling, for eksempel en deling, dokumentene dine eller et bildebibliotek, med valgfrie ekskluderingsmønstre. Mappesamlinger har tidsplaner, oppbevaring, eksterne kopier, kontroller og gjenopprettingsøvelser som alt annet. |
| **ZFS-datasett** | Et datasett og hvert datasett under det, lest fra ett ZFS-øyeblikksbilde så alle kommer fra samme øyeblikk, og lagret med restic som en mappe. Et ZFS-element kan også ha en blokkeksakt replika på en andre ZFS-server. Se [ZFS-datasett](zfs-datasets.md). |

Hver sikkerhetskopi går gjennom restic, så den er inkrementell og deduplisert, og selv store VM-disker blåser ikke opp depotet. BombVault tar aldri sikkerhetskopi av sin egen container, og stopper den dermed heller aldri. En kjøring kan stoppes trygt, fordi restic skriver øyeblikksbildet sitt sist, så en avbrutt kjøring etterlater ubrukte data og ingen snapshot. En lås som er igjen etter en omstart midt i en kjøring, fjernes og kjøringen prøves én gang til, og først etter at låsen har gått ti minutter uten å bli fornyet.

## Listen {#list}

Fliser over listen velger en type, for eksempel **Kontainere** eller **ZFS**, og hver flis teller elementene sine. En flis velger, den skjuler ikke resten av siden.

Hver rad viser elementets siste fjorten kjøringer som små ruter og én status, for eksempel **Beskyttet**, **Ikke beskyttet**, **Utsatt** eller **Replikerer**. Alt annet om det står i infoboblen ved siden av navnet, og mens pekeren hviler på raden, står den teksten i selve raden. Et merke ved navnet til en container sier at den er opprettet på nytt med et annet image, andre porter, variabler eller volumer siden siste sikkerhetskopi. Det forsvinner med neste sikkerhetskopi.

Tre knapper over listen avgrenser den:

- **Filtre**: etter **Tidsplan** (**Alle**, **Planlagt**, **På pause**), etter **Sikkerhetskopiering** (**Sikkerhetskopiert**, **Aldri sikkerhetskopiert**) og, for **Kontainere og VM-er**, **Installert** eller **Ikke installert**.
- **Sorter**: etter navn, status, IP, siste sikkerhetskopi, størrelse eller tidsplan. Alle felt unntatt IP sorterer begge veier, for eksempel **Størrelse, største først** og **Størrelse, minste først**.
- **Søk**: etter navn, image eller IP.

Elementer som ikke lenger er installert men fortsatt har sikkerhetskopier, står for seg under **Ikke installert (kun sikkerhetskopier)**. Gjenopprett dem, eller slett sikkerhetskopiene deres for å frigjøre plassen.

Sidens hovedhandling flyter nederst til høyre: **Sikkerhetskopier alt nå** starter en kjøring over alle områder som er slått på. **Mer** har resten:

- **Velg** starter et utvalg, se nedenfor.
- **Oppdag sikkerhetskopier** leser hva målene inneholder og bygger opp elementer fra det, for eksempel etter en fersk installasjon. En container uten egne data har ikke noe øyeblikksbilde, og den kommer tilbake fra definisjonen som speiles ved siden av depotet. Eksterne kopier av gjenoppbygde områder venter da til du overtar sikkerhetskopiene deres som denne serverens egne, se [Etter en ombygging](offsite-recovery.md#after-a-rebuild).
- **ZFS-tilkobling til denne serveren** åpner SSH-testen for ZFS-området, se [ZFS-datasett](zfs-datasets.md#requirements).
- **Replikér alt nå**, med ZFS-flisen valgt, sender replikaen av hvert ZFS-element.

![Den samme listen på en telefon.](assets/screenshots/backups/list-phone-dark.png#only-dark)
![Den samme listen på en telefon.](assets/screenshots/backups/list-phone-light.png#only-light)

## Velge flere elementer {#select}

Velg rader slik du velger filer i en filbehandler: Ctrl+klikk, Shift+klikk, Ctrl+A eller et langt trykk på en telefon. En valgt rad viser et hakemerke i stedet for ikonet, og en linje over listen teller dem og tilbyr **Sikkerhetskopier valgte**, **Sett tidsplanen på pause** og **Gjenoppta tidsplanen**, **Velg alle** og **Fjern valg**. Hver rad har også en høyreklikkmeny med sine egne handlinger.

Et utvalg sikkerhetskopieres på serveren, ett element etter det andre, så det fortsetter når du lukker fanen. For å gjenopprette flere elementer sammen bruker du [siden Gjenoppretting](restore.md#several).

![To containere valgt, med linjen over listen.](assets/screenshots/backups/bulk-dark.png#only-dark)
![To containere valgt, med linjen over listen.](assets/screenshots/backups/bulk-light.png#only-light)

## Legge til en mappe eller et ZFS-datasett {#add}

Containere, VM-er og flash-disken dukker opp av seg selv når området er slått på under **Innstillinger, Generelt**. Mapper og ZFS-datasett velger du selv, fordi BombVault ikke kan gjette hvilke du mener. **Legg til** åpner **Hva mer skal BombVault sikkerhetskopiere?**:

- **Én mappe** lister delingene BombVault fant og tar hvilken som helst annen mappe under Host Data. Gi mappesamlingen et navn, for eksempel `Documents`. Navnet ligger fast etter den første sikkerhetskopien, fordi øyeblikksbildene lagres under det.
- **Et ZFS-datasett** lister datasettene på denne serveren. Velg det øverst i det du vil ha sikkerhetskopiert, se [Elementer og underdatasett](zfs-datasets.md#items-and-children).
- **Systemkonfigurasjon** vises på en vert uten Unraid der Host Data når `/etc`. Det er en `/etc`-mappesamling å starte med, som du ser over før du lagrer.

## La et element stå utenfor {#dont-back-up}

Et element du bevisst lar stå utenfor, for eksempel en testcontainer, kan merkes **Ikke sikkerhetskopier**. Det slutter da å telle som ubeskyttet, og **Ikke unnta lenger** henter det tilbake. **Sett tidsplanen på pause** er den mykere veien: elementet forlater tidsplanen og den fulle sikkerhetskopien, får et grått merke **Tidsplan på pause**, og **Sikkerhetskopier nå** virker fortsatt. Det setter den samme bryteren som **I tidsplanen** på elementets fane Innstillinger.

## Siden til et element {#entry-page}

Et klikk på en rad åpner elementets side. Hodet nevner elementet og tilstanden i én linje, for eksempel **Sist sikkerhetskopiert for 3 timer siden** eller **Utsatt, neste forsøk kl. 03:15**, og har handlingene: **Sikkerhetskopier nå**, **Gjenopprett**, og under **Mer** de sjeldne, som **Eksporter nyeste sikkerhetskopi**, **Eksporter container** eller **Eksporter VM**, **Koble til et tidligere element**, **Slett alle sikkerhetskopier**, **Fjern element** og **Slett sikkerhetskopier overalt**. En tilbakeknapp til venstre for ikonet fører til listen. Mens en sikkerhetskopi kjører, står **Avbryt sikkerhetskopi** ved siden av framdriften, og en avbrutt kjøring registreres som avbrutt, ikke som mislykket.

**Slett sikkerhetskopier overalt** fjerner elementets sikkerhetskopier på alle mål på én gang, eksterne kopier medregnet. Vinduet viser hvert sted med hvor mange sikkerhetskopier det har, og knappen låses opp når du skriver inn navnet på elementet. Et sted som er satt til append-only, beholder sikkerhetskopiene sine og sier fra om det. Elementet og innstillingene blir værende, så et element du ikke lenger sikkerhetskopierer, forblir slik, og bare sikkerhetskopiene og historikken deres forsvinner.

Siden har tre faner: **Sikkerhetskopier**, **Innstillinger** og **Verifisering**.

### Fanen Sikkerhetskopier {#backups-tab}

Fanen lister elementets øyeblikksbilder, nyeste først, med en søyle per dag. En sikkerhetskopi som ligger flere steder, vises én gang, merket med hvert mål som har den. Et klikk på et øyeblikksbilde gjenoppretter det, og et klikk på et sted inne i et øyeblikksbilde gjenoppretter det akkurat derfra.

- **Mål** nevner hvert sted med sikkerhetskopier av dette elementet og hvor mange. Eksterne steder leses bare når du trykker på **Sjekk**, fordi det tar litt tid å liste skylagring. Et sted elementet ikke lenger sikkerhetskopierer til, står som **Bare eldre sikkerhetskopier** og kan fortsatt gjenopprettes fra.
- Hvert øyeblikksbilde har en meny: **Legg til tagg** eller **Endre tag**, **Sammenlign med en annen sikkerhetskopi**, som lister filene som kom, forsvant eller endret seg, **Eksporter** som tar-arkiv, zip eller et eget restic-depot, og sletting på bare ett sted.
- **Databasedumper** for en container står i en egen liste, med nedlasting, lagring i en mappe og import.

Hvor mange øyeblikksbilder som blir værende, settes av målet, se [Hvor lenge det beholdes](offsite-recovery.md#keep).

![Fanen Sikkerhetskopier for en container: en søyle per dag og målene som har hvert øyeblikksbilde.](assets/screenshots/backups/entry-backups-dark.png#only-dark)
![Fanen Sikkerhetskopier for en container: en søyle per dag og målene som har hvert øyeblikksbilde.](assets/screenshots/backups/entry-backups-light.png#only-light)

### Fanen Innstillinger {#settings-tab}

Hvert element starter med **Tidsplan og mål**:

- **I tidsplanen** tar elementet med i eller ut av de planlagte kjøringene.
- **Egen tidsplan** gir akkurat dette elementet en egen plan, se [En egen plan for ett element](schedules.md#own-plan). Uten en kjører det på planen til området sitt eller i den fulle sikkerhetskopien.
- En rad med knapper, **Lokal** og én per eksternt mål, sier hvor sikkerhetskopiene går. **Følger standarden** betyr at elementet går dit området går, og et klikk på den åpner **Mål per område**. Se [Plassering per element](offsite-recovery.md#placement).
- **Repository** nevner hvor sikkerhetskopiene skrives (**Lagret på**). Det stedet ligger fast fra den første sikkerhetskopien, kopiene kan endres når som helst.

Kortene under avhenger av typen element:

| Type | Kort |
|---|---|
| Container | **Sikkerhetskopierte mapper** (hvilke tilordnede mapper som sikkerhetskopieres, med **Hopp over cachemapper (CACHEDIR.TAG)**), **Ekskluderingsmønstre**, **Stopp andre containere**, **Kommandoer før og etter**, **Database**, **Når appen er opptatt**, **Oppdater etter en vellykket sikkerhetskopi**, **Øyeblikksbilde i stedet for stopp**, **Tidligere navn** |
| VM | **VM** med **Metode** (**Pent (nedstenging)** eller **Live-snapshot**) og **Bare endrede blokker**, se [VM-veiledningen](vm-backup-ssh-setup.md#per-vm-method) |
| Mappesamling | **Mappe** og **Ekskluderingsmønstre** |
| Flash-disk | **Flash-zip-eksport**, som skriver hver flash-sikkerhetskopi ut som en vanlig `.zip` til en mappe du velger, i ett steg, så et synkroniseringsverktøy aldri ser en halv fil. En flash-sikkerhetskopi som inneholder mindre enn den burde, lar den siste ZIP-filen ligge, og kjøringen til eksporten sier det. |
| ZFS-datasett | underdatasettene, containerne som skal stoppes og kommandoene rundt øyeblikksbildet, **Replika** og **Test tilgangen til øyeblikksbilder**, se [ZFS-datasett](zfs-datasets.md) |

Innstillinger lagres av seg selv mens du endrer dem.

![Fanen Innstillinger for en container, med Egen tidsplan i det første kortet.](assets/screenshots/backups/entry-settings-dark.png#only-dark)
![Fanen Innstillinger for en container, med Egen tidsplan i det første kortet.](assets/screenshots/backups/entry-settings-light.png#only-light)

### Fanen Verifisering {#verify-tab}

- **Kontroller** viser gjenopprettingskontrollen som følger etter den første sikkerhetskopien. **Sjekk gjenoppretting** kjører den på nytt på den nyeste sikkerhetskopien: BombVault gjenoppretter et utvalg på opptil 100 filer og 256 MiB til en midlertidig mappe og lar restic lese hver fil tilbake mot hashene sine.
- **Starttest**, på en container, gjenoppretter den nyeste sikkerhetskopien til en isolert kopi og starter den: et navn som begynner med `bombvault-test-`, et eget internt Docker-nettverk uten publiserte porter og uten vei til LAN, 1 CPU og 2 GiB minne. Den består når kopien melder seg frisk, svarer på sin første eksponerte port, eller, med ingen av delene, fortsetter å kjøre. Originalen røres aldri, og kopien, nettverket og dataene fjernes etterpå, også etter en omstart midt i en test. Containere på vertsnettverket, privilegerte, de med enheter og de som trenger en annen container, vises som ikke testbare. **Starttest** under **Innstillinger, Verifisering, Automatiske kontroller** tester én container per kjøring, den som ble testet for lengst siden først.
- **Avvik** setter dette elementets egen følsomhet og eget varselnivå og viser de åpne funnene.
- **Størrelse per mappe** viser hva som tar plassen i den nyeste sikkerhetskopien og hvor mye av det den siste sikkerhetskopien tok med, ett nivå om gangen.
- **Siste kjøringer** lister de siste kjøringene med start, slutt og varighet. Et klikk viser detaljene.

![Fanen Verifisering for en container med Størrelse per mappe åpen.](assets/screenshots/backups/entry-verify-dark.png#only-dark)
![Fanen Verifisering for en container med Størrelse per mappe åpen.](assets/screenshots/backups/entry-verify-light.png#only-light)

## Ekskluderingsmønstre {#excludes}

Kortet **Ekskluderingsmønstre** tar ett mønster per linje. Skriv stiene slik du ser dem inne i containeren, for eksempel `/config/Library/Application Support/Plex Media Server/Cache`, eller et bart navn som `.git`, som treffer på hvilken som helst dybde. For et mappesett skriver du stien slik verten kjenner den, for eksempel `/mnt/user/data/cache`. En forhåndsvisning under feltet viser hva hver linje blir til, og advarer når en linje ikke treffer noe eller ville utelatt alt. En linje med `/` eller `*`, eller en som nevner en hel sikkerhetskopiert mappe, ignoreres, så et unntak aldri kan tømme en sikkerhetskopi. For å utelate en hel mappe fjerner du heller avkrysningen under **Sikkerhetskopierte mapper**.

For kjente images som Plex, Jellyfin, Sonarr, Radarr, Immich og Nextcloud tilbyr kortet mappene appen fyller på nytt av seg selv, for eksempel cacher, logger og miniatyrbilder. Ingenting utelates før du trykker på **Unnta valgte**.

Et nytt mappesett starter med gruppen **Systemrask** slått på: `.DS_Store`, `Thumbs.db`, `desktop.ini`, `@eaDir` (Synology), `.Recycle.Bin` (Unraid Recycle Bin plugin), `.Trash-*` og `*.tmp`. Hver linje er et bart navn, så den treffer på hvilken som helst dybde under mappen, og du kan slå av hvilken som helst av dem før du oppretter settet. Et eksisterende mappesett beholder unntakene det har. **Unnta systemrask** i redigeringsvinduet tilbyr de samme linjene, og de kommer først inn i settet når du trykker på **Unnta valgte** og lagrer.

Det samme kortet har advarsler en mappeskanning ikke kan se. Immich holder for eksempel albumene, ansiktene og datoene sine i en PostgreSQL-database i en egen container, så en filsikkerhetskopi av Immich-containeren alene gjenoppretter bildene uten noe av det, og gjenopprettingen ser ut til å ha virket. Ta sikkerhetskopi av databasecontaineren også.

## Databasedumper {#database-dumps}

PostgreSQL-, MySQL- og MariaDB-containere som BombVault kjenner igjen, dumpes før hver sikkerhetskopi, fra den kjørende serveren, til et eget gjenopprettingspunkt ved siden av filene. Det dekker de offisielle imagene, PostGIS, TimescaleDB, pgvector, pgautoupgrade, Immichs databaseimages, MariaDB-imagene fra linuxserver, yobasystems og jc21 og Oracles mysql-server. En container som bare ligner på en database, får det samme valget, slått av til du velger det.

Dumpen strømmer rett inn i depotet og skrives aldri til en disk. Påloggingsdata hentes fra containerens egne variabler, `*_FILE`-hemmeligheter inkludert, og forlater den aldri. Kortet **Database** på containerens fane Innstillinger sier om datamappen lagres mens den er stoppet, kopieres mens den kjører, eller ikke lagres, og slår dumpen av eller på for denne containeren. Etiketten `bombvault.dbdump=false` gjør det samme og går foran kortet. **Automatiske databasedumper** under **Innstillinger, Generelt** slår det av eller på for alle.

En mislykket dump får ikke sikkerhetskopien til å feile. Den vises som en egen mislykket kjøring med årsaken og sender et varsel. Dumper lastes aldri tilbake av seg selv: last ned én, vanlig eller komprimert, lagre den i en mappe, eller importer den i en nystartet database med ett klikk på fanen **Sikkerhetskopier**. For å få en dump tilbake uten BombVault, se [Hente en databasedump tilbake](offsite-recovery.md#database-dumps). Når en feiler, se [En databasedump feilet](troubleshooting.md#a-database-dump-failed).

## Kommandoer og andre containere {#commands}

- **Kommandoer før og etter** kjører inne i containeren med `sh -c`, for eksempel for å skrive en cache til disk. En kommando før sikkerhetskopien som feiler, avbryter den. En kommando før sikkerhetskopien som avslutter med kode 75, utsetter i stedet en planlagt sikkerhetskopi, se nedenfor.
- **Stopp andre containere** nevner containere som skal stoppes mens denne sikkerhetskopieres, for eksempel databasen. Etterpå starter de igjen i Compose-rekkefølgen `depends_on`, og som standard må hver av dem melde seg frisk før den neste starter. Ventingen og tidsavbruddet står under **Omstart etter sikkerhetskopiering** i kortet **Rekkefølge** under **Innstillinger, Tidsplan**.
- En container som deler nettverks-, PID- eller IPC-navnerommet med en som stoppes, gjennom `--net=container:` eller Composes `network_mode: service:`, stopper og starter sammen med den selv når den ikke står på listen, og den starter etter containeren den kobler seg til. En container som startes på nytt, får et nytt nettverk, og en som ble stående og kjøre på det gamle nettverket, ville bare hatt loopback. Selve listen blir slik du satte den. Hvis en slik container ikke kunne stoppes, sier kjøringen at den er uten nettverk til den startes på nytt. Det samme gjelder BombVaults egen container, som aldri stopper seg selv og ikke kan settes på listen.
- En container som ikke kjører igjen etterpå, nevnes i kjøringen og i et varsel, også når selve sikkerhetskopien feilet.

## Når appen er opptatt {#app-busy}

En sikkerhetskopi stopper containeren sin, så en planlagt kan vente mens noen bruker appen og prøve igjen av seg selv. Kortet **Når appen er opptatt** på en containers fane Innstillinger setter det opp:

- **Aktivitetssjekk** spør Plex, Jellyfin eller Emby før en planlagt sikkerhetskopi om noen ser på. Plex spørres med tokenet fra sin egen `Preferences.xml`, så det er ingenting å skrive inn. Jellyfin og Emby tar en **API-nøkkel**, laget i serverens dashbord under API-nøkler.
- **Ignorer pausede økter etter (min)** lar en film som er satt på pause for natten, slutte å telle.
- **Ta sikkerhetskopi når serveren ikke svarer** bestemmer hva som skjer når serveren ikke kan spørres: på, så kjører sikkerhetskopien og loggen sier det. Av, så venter den som om noen så på.
- **Vent mens containeren sender data** er for apper uten en slik sjekk, for eksempel en spillserver eller en nedlaster. Sikkerhetskopien venter mens containeren sendte mer enn **Opptatt fra (KiB/s)** ved en hvilken som helst måling de siste **Rolig i (min)** minuttene.
- **Nytt forsøk hvert (min)**, **Prøv bare innenfor et tidsvindu** med **Prøv mellom**, og **Vent maks (timer)** setter hvordan forsøkene går.
- **Når ventetiden er over** gir enten opp, som viser sikkerhetskopien under **Krever oppmerksomhet** med **Prøv igjen**, eller **Ta sikkerhetskopi likevel**.

En ventende sikkerhetskopi registreres som **Utsatt**, i gult, og resten av kjøringen fortsetter. Oversikten, listen og elementet viser den med neste forsøk, og forsinkelsesvarselet og beskyttelsesstatusen venter på den til fristen. Du får ett varsel når den utsettes og ett når den er over, ikke ett per forsøk. Et forsøk venter mens en annen sikkerhetskopi kjører, og en ventetid fortsetter etter en omstart. En sikkerhetskopi du starter selv, kjører alltid med en gang, enten med **Sikkerhetskopier nå**, **Prøv igjen**, et utvalg, API-et eller MCP, og en som går gjennom, avslutter en ventetid som sto og ventet.

Containerens fane Innstillinger har også en eldre, enklere måte: **Vent til appen er i ro** med **Vent maks (timer)**. Sikkerhetskopien starter så snart appen er i ro, noe kortet **Ro før sikkerhetskopi** under **Innstillinger, Tidsplan** definerer. Medlemmene av en Compose-stack som skal kjøre i samme runde, venter sammen, og den ventende sikkerhetskopien holder ingen lås, så de andre containerne går videre.

![Kortet Når appen er opptatt for en Plex-container med aktivitetssjekken satt opp.](assets/screenshots/backups/app-busy-dark.png#only-dark)
![Kortet Når appen er opptatt for en Plex-container med aktivitetssjekken satt opp.](assets/screenshots/backups/app-busy-light.png#only-light)

### Eksempel: Plex venter mens noen ser på {#example-plex}

Den fulle sikkerhetskopien kjører hver natt kl. 03:00. Ingen skal oppleve at en film stopper fordi Plex gikk ned for sikkerhetskopien, men sikkerhetskopien skal likevel bli tatt før morgenen.

1. Åpne **Sikkerhetskopier**, deretter elementet `plex`, så fanen **Innstillinger**.
2. I **Når appen er opptatt** setter du **Aktivitetssjekk** til Plex. Det er ingenting å skrive inn.
3. Sett **Ignorer pausede økter etter (min)** til `30`, så en film som ble satt på pause ved midnatt, slutter å telle halv ett.
4. La **Ta sikkerhetskopi når serveren ikke svarer** stå på.
5. Sett **Nytt forsøk hvert (min)** til `15`.
6. Slå på **Prøv bare innenfor et tidsvindu** og sett **Prøv mellom** til `22:00` og `06:00`.
7. Sett **Vent maks (timer)** til `3` og **Når ventetiden er over** til **Ta sikkerhetskopi likevel**.

Dette skjer en natt da noen fortsatt ser på kl. 03:00: BombVault spør Plex, finner en økt som spilles, og registrerer sikkerhetskopien som **Utsatt** med neste forsøk kl. 03:15. De andre containerne i kjøringen sikkerhetskopieres som vanlig. Når filmen slutter kl. 04:10, finner forsøket kl. 04:15 Plex i ro og tar sikkerhetskopi, og ventetiden slutter med ett varsel. Hvis filmen fortsatt gikk kl. 06:00, ville ventetiden nådd slutten og Plex blitt sikkerhetskopiert likevel, så sikkerhetskopiene aldri havner mer enn tre timer på etterskudd.

For din egen sjekk lar du **Pre-sikkerhetskopikommando** avslutte med kode 75 mens appen er opptatt, for eksempel `[ "$(cat /tmp/players)" = 0 ] || exit 75`. Flere symptomer og løsninger står i [Feilsøking](troubleshooting.md#deferred-backups).

## Rene eksporter {#exports}

restic er fortsatt motoren, men en eksport gir deg en kopi som verken trenger BombVault eller restic.

- **Eksporter container** og **Eksporter VM** under **Mer** på et elements side skriver en kopi slik den er akkurat nå, ved siden av depotet: containerens sikkerhetskopierte mapper pluss Unraid-malen, eller VM-ens diskbilder pluss definisjonen, som kan gjenopprettes med `virsh define`.
- **Eksporter** på et øyeblikksbilde skriver en lagret sikkerhetskopi til eksportmappen på serveren, som tar-arkiv, zip eller et eget restic-depot, og kan ta ett øyeblikksbilde eller alle. En flash-sikkerhetskopi og en auto-sikkerhetskopi inneholder nøkler, så eksport av en slik krever et påloggingspassord og en pålogging på instansen.

Eksporter er vanlige filer som standard. **Krypter eksporter med age** under **Innstillinger, System** forsegler dem, flash-ZIP-en inkludert, for mottakerne du lister opp, hver en offentlig age-nøkkel eller en offentlig SSH-nøkkel. Du dekrypterer dem senere utenfor boksen med den tilhørende private nøkkelen. Med kryptering på og ingen gyldig mottaker feiler en eksport med en tydelig feil i stedet for å skrive vanlige filer.

## Hva en kjøring forteller deg {#run-notes}

- **Tidsbruk.** Hver kjøring viser start, slutt og varighet, og hvert element har sine egne **Siste kjøringer**.
- **Hvorfor en sikkerhetskopi var treg.** Mens en sikkerhetskopi kjører, følger BombVault med på hvor opptatt CPU-en, diskene og nettverket er. Når en sikkerhetskopi tar mye lenger tid enn vanlig og én ting tydelig var på grensen, sier kjøringen det, for eksempel «Måldisken disk1 var 98 % opptatt». Ellers sier den ingenting.
- **Framdrift for kontroller og beskjæringer.** Aktivitetsloggen og kortet **Kontroller sikkerhetskopier** viser hvor langt restic har telt, for eksempel *12 av 47 pakker*, med gjenstående tid i det trinnet når det er nok å anslå ut fra.
- **Filer den ikke kunne lese.** En sikkerhetskopi som gikk gjennom men utelot noe, gjør linjen sin oransje og nevner hva som ble utelatt.
