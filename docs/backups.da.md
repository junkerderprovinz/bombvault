# Sikkerhedskopier og poster

Alt, hvad BombVault sikkerhedskopierer, står i én liste under **Sikkerhedskopier**: containere, VM'er, mappesæt, flashdrevet, ZFS-datasæt og auto-sikkerhedskopien. Hver af dem er en **post** med en side for sig selv. Denne side gennemgår listen, postens side og dens tre faner og de indstillinger, en post kan have.

![Listen Sikkerhedskopier med fligen Containers valgt: Hver række viser sine seneste kørsler som en stribe og én status.](assets/screenshots/backups/list-dark.png#only-dark)
![Listen Sikkerhedskopier med fligen Containers valgt: Hver række viser sine seneste kørsler som en stribe og én status.](assets/screenshots/backups/list-light.png#only-light)

## Hvad hvert område gemmer {#what-is-saved}

| Område | Hvad der gemmes |
|---|---|
| **Containers** | Appdata-mappen plus containerens definition: image, variabler, porte, labels og volumener. Kortet **Sikkerhedskopierede mapper** på containerens fane **Indstillinger** sætter flueben ved præcis de mapper, sikkerhedskopien dækker, med en live optælling af stierne og en liste over det, du har udeladt. Data, en container gemmer uden for Host Data, for eksempel i et TrueNAS ixVolume eller et Compose-projekt under `/opt`, sikkerhedskopieres ikke, og kørslen nævner de mapper, den udelod. Kørslen nævner også en mappe inde i Host Data, der ikke findes lige nu. Systemstier som `/etc/localtime` eller Docker-socketten tæller ikke som data. |
| **VMs** | Diskimages, XML-definitionen og UEFI-NVRAM, med en pæn nedlukning eller et live-snapshot, over SSH. Se [VM-sikkerhedskopier](vm-backup-ssh-setup.md). |
| **Flash** | Kun Unraid. Hele USB-flashdrevet (`/boot`): OS, licens, array-konfiguration, shares, netværk og plugin-konfiguration. |
| **Auto-sikkerhedskopi** | BombVaults egen `/config`: dens indstillingsdatabase, destinationernes adgangsoplysninger, libvirt-SSH-nøglenparret, nøglen og de gemte værtsnøgler til ZFS-serverne og certifikaterne, taget som snapshot med SQLite `VACUUM INTO`, så databasen aldrig fanges midt i en skrivning. |
| **Mapper** | Enhver mappe på serveren som et mappesæt, for eksempel en share, dine dokumenter eller et fotobibliotek, med valgfrie udelukkelsesmønstre. Mappesæt har tidsplaner, opbevaring, off-site-kopier, tjek og gendannelsestest som alt andet. |
| **ZFS-datasæt** | Et datasæt og alle datasæt under det, læst fra ét ZFS-snapshot, så alle kommer fra samme øjeblik, og gemt med restic som en mappe. En ZFS-post kan også holde en blokpræcis replika på en anden ZFS-server. Se [ZFS-datasæt](zfs-datasets.md). |

Hver sikkerhedskopi går gennem restic, så den er inkrementel og dedupliceret, og selv store VM-diske blæser ikke repositoryet op. BombVault sikkerhedskopierer aldrig, og stopper derfor aldrig, sin egen container. En kørsel kan stoppes sikkert, fordi restic skriver sit gendannelsespunkt til sidst, så en afbrudt kørsel efterlader ureferenceret data og intet gendannelsespunkt. En lås, der er efterladt af en genstart midt i en kørsel, ryddes, og kørslen prøves én gang til, og først når låsen har ligget ti minutter uden at blive fornyet.

## Listen {#list}

Fliser over listen vælger en slags, for eksempel **Containers** eller **ZFS**, og hver flise tæller sine poster. En flise vælger, den skjuler ikke resten af siden.

Hver række viser postens seneste fjorten kørsler som små firkanter og én status, for eksempel **Beskyttet**, **Ikke beskyttet**, **Udsat** eller **Replikerer**. Alt andet om den står i infoboblen ved siden af dens navn, og mens markøren hviler på rækken, står den tekst i selve rækken. Et mærke ved en containers navn siger, at den er genskabt med et andet image, andre porte, variabler eller volumener siden sin sidste sikkerhedskopi. Det forsvinder ved næste sikkerhedskopi.

Tre knapper over listen indsnævrer den:

- **Filtre**: efter **Tidsplan** (**Alle**, **Planlagt**, **Pauset**), efter **Sikkerhedskopi** (**Sikkerhedskopieret**, **Aldrig sikkerhedskopieret**) og for **Containers og VMs** **Installerede** eller **Ikke installeret**.
- **Sortér**: efter navn, status, IP, seneste sikkerhedskopi, størrelse eller tidsplan. Alle felter undtagen IP sorterer begge veje, for eksempel **Størrelse, største først** og **Størrelse, mindste først**.
- **Søg**: efter navn, image eller IP.

Poster, der ikke længere er installeret, men stadig har sikkerhedskopier, står for sig under **Ikke installeret (kun sikkerhedskopier)**. Gendan dem, eller slet deres sikkerhedskopier for at frigøre pladsen.

Sidens hovedhandling svæver nederst til højre: **Sikkerhedskopiér alt nu** starter en kørsel over hvert aktiveret område. **Mere** rummer resten:

- **Vælg** starter et valg, se nedenfor.
- **Opdag sikkerhedskopier** læser, hvad destinationerne rummer, og genopbygger poster ud fra det, for eksempel efter en frisk installation. En container uden egne data har intet snapshot og kommer tilbage fra den definition, der er spejlet ved siden af repositoryet. Off-site-kopier af genopbyggede områder venter så, til du overtager deres sikkerhedskopier som denne servers egne, se [Efter en genopbygning](offsite-recovery.md#after-a-rebuild).
- **ZFS-forbindelse til denne server** åbner SSH-testen for ZFS-området, se [ZFS-datasæt](zfs-datasets.md#requirements).
- **Replikér alt nu**, med fligen ZFS valgt, sender replikaen af hver ZFS-post.

![Den samme liste på en telefon.](assets/screenshots/backups/list-phone-dark.png#only-dark)
![Den samme liste på en telefon.](assets/screenshots/backups/list-phone-light.png#only-light)

## Vælg flere poster {#select}

Vælg rækker, som du vælger filer i en filhåndtering: Ctrl+klik, Shift+klik, Ctrl+A eller et langt tryk på en telefon. En valgt række viser et flueben i stedet for sit ikon, og en bjælke over listen tæller dem og tilbyder **Sikkerhedskopier valgte**, **Sæt tidsplan på pause** og **Genoptag tidsplan**, **Vælg alle** og **Ryd valg**. Hver række har også en højreklik-menu med sine egne handlinger.

Et valg sikkerhedskopieres på serveren, én post efter den anden, så det fortsætter, når du lukker fanen. For at gendanne flere poster sammen bruger du [siden Gendannelse](restore.md#several).

![To containere valgt, med bjælken over listen.](assets/screenshots/backups/bulk-dark.png#only-dark)
![To containere valgt, med bjælken over listen.](assets/screenshots/backups/bulk-light.png#only-light)

## Tilføj en mappe eller et ZFS-datasæt {#add}

Containere, VM'er og flashdrevet dukker op af sig selv, når deres område er slået til under **Indstillinger, Generelt**. Mapper og ZFS-datasæt vælger du selv, fordi BombVault ikke kan gætte, hvilke du mener. **Tilføj** åbner **Hvad skal BombVault ellers sikkerhedskopiere?**:

- **Én mappe** viser de shares, BombVault fandt, og tager enhver anden mappe under Host Data. Giv mappesættet et navn, for eksempel `Documents`. Navnet er fast efter den første sikkerhedskopi, fordi gendannelsespunkterne gemmes under det.
- **Et ZFS-datasæt** viser serverens datasæt. Vælg det øverste i det, du vil have sikkerhedskopieret, se [Poster og underdatasæt](zfs-datasets.md#items-and-children).
- **Systemkonfiguration** vises på en vært uden Unraid, hvis Host Data når `/etc`. Det er et mappesæt til `/etc` som udgangspunkt, du gennemgår, før du gemmer.

## Udelad en post {#dont-back-up}

En post, du med vilje udelader, for eksempel en testcontainer, kan markeres **Sikkerhedskopiér ikke**. Den holder så op med at tælle som ubeskyttet, og **Udeluk ikke længere** henter den tilbage. **Sæt tidsplan på pause** er den blødere vej: Posten forlader tidsplanen og den fulde sikkerhedskopi, får et gråt mærke **Tidsplan på pause**, og **Sikkerhedskopier nu** virker stadig. Det sætter den samme kontakt som **På tidsplanen** på postens fane Indstillinger.

## En posts side {#entry-page}

Et klik på en række åbner postens side. Hovedet nævner posten og dens tilstand på én linje, for eksempel **Senest sikkerhedskopieret for 3 timer siden** eller **Udsat, næste forsøg kl. 03:15**, og rummer dens handlinger: **Sikkerhedskopier nu**, **Gendan** og under **Mere** de sjældne, for eksempel **Eksporter nyeste sikkerhedskopi**, **Eksporter container** eller **Eksporter VM**, **Knyt til et tidligere element**, **Slet alle sikkerhedskopier**, **Fjern element** og **Slet sikkerhedskopier overalt**. En tilbageknap til venstre for ikonet fører til listen. Mens en sikkerhedskopiering kører, står **Annullér sikkerhedskopi** ved siden af dens fremdrift, og en annulleret kørsel registreres som annulleret, ikke som mislykket.

**Slet sikkerhedskopier overalt** fjerner postens sikkerhedskopier på alle destinationer på én gang, off-site-kopier medregnet. Vinduet viser hvert sted med antallet af sikkerhedskopier der, og knappen låses op, når du skriver postens navn. Et sted, der er sat til append-only, beholder sine sikkerhedskopier og siger det. Posten og dens indstillinger bliver, så en post, du ikke længere sikkerhedskopierer, forbliver sådan, og kun dens sikkerhedskopier og deres historik forsvinder.

Siden har tre faner: **Sikkerhedskopier**, **Indstillinger** og **Verificering**.

### Fanen Sikkerhedskopier {#backups-tab}

Denne fane viser postens gendannelsespunkter, nyeste først, med en bjælke pr. dag. En sikkerhedskopi, der ligger flere steder, vises én gang, mærket med hver destination, der rummer den. Et klik på et gendannelsespunkt gendanner det, og et klik på et sted inde i et gendannelsespunkt gendanner det netop derfra.

- **Destinationer** nævner hvert sted med sikkerhedskopier af denne post og hvor mange. Off-site-steder læses kun, når du trykker på **Tjek**, fordi det tager et stykke tid at liste cloud-lager. Et sted, posten ikke længere sikkerhedskopierer til, står som **Kun ældre sikkerhedskopier** og kan stadig gendannes fra.
- Hvert gendannelsespunkt har en menu: **Tilføj tag** eller **Skift tag**, **Sammenlign med en anden sikkerhedskopi**, som viser de filer, der kom, forsvandt eller blev ændret, **Eksporter** som tar, zip eller et restic-repository for sig selv, og slet kun ét sted.
- **Databasedumps** fra en container står i en liste for sig med download, gem i en mappe og import.

Hvor mange gendannelsespunkter der bliver, indstiller destinationen, se [Hvor længe der gemmes](offsite-recovery.md#keep).

![Fanen Sikkerhedskopier for en container: en bjælke pr. dag og de destinationer, der rummer hvert gendannelsespunkt.](assets/screenshots/backups/entry-backups-dark.png#only-dark)
![Fanen Sikkerhedskopier for en container: en bjælke pr. dag og de destinationer, der rummer hvert gendannelsespunkt.](assets/screenshots/backups/entry-backups-light.png#only-light)

### Fanen Indstillinger {#settings-tab}

Hver post starter med **Tidsplan og destination**:

- **På tidsplanen** tager posten ind i eller ud af de planlagte kørsler.
- **Egen plan** giver netop denne post en plan for sig selv, se [En egen plan til én post](schedules.md#own-plan). Uden en kører den efter sit områdes plan eller i den fulde sikkerhedskopi.
- En række knapper, **Lokal** og én pr. off-site-destination, siger, hvor sikkerhedskopierne havner. **Følger standarden** betyder, at posten går derhen, hvor dens område går, og et klik på den åbner **Destinationer pr. område**. Se [Placering pr. element](offsite-recovery.md#placement).
- **Repository** nævner, hvor sikkerhedskopierne skrives (**Gemt på**). Det sted er fast fra den første sikkerhedskopi, kopierne kan ændres når som helst.

Kortene nedenfor afhænger af postens slags:

| Slags | Kort |
|---|---|
| Container | **Sikkerhedskopierede mapper** (hvilke tilknyttede mapper der sikkerhedskopieres, med **Spring cachemapper over (CACHEDIR.TAG)**), **Udelukkelsesmønstre**, **Stop andre containere**, **Kommandoer før og efter**, **Database**, **Når appen er optaget**, **Opdater efter en vellykket sikkerhedskopiering**, **Snapshot i stedet for stop**, **Tidligere navne** |
| VM | **VM** med **Metode** (**Pænt (nedlukning)** eller **Live-snapshot**) og **Kun ændrede blokke**, se [VM-guiden](vm-backup-ssh-setup.md#per-vm-method) |
| Mappesæt | **Mappe** og **Udelukkelsesmønstre** |
| Flashdrev | **Flash-zip-eksport**, som skriver hver flash-sikkerhedskopi ud som en almindelig `.zip` til en mappe, du vælger, i ét trin, så et synkroniseringsværktøj aldrig ser en halv fil. En flash-sikkerhedskopi, der rummer mindre, end den burde, lader den sidste ZIP blive liggende, og eksportens kørsel siger det. |
| ZFS-datasæt | underdatasæt, containere der skal stoppes, og kommandoerne omkring snapshottet, **Replika** og **Test adgangen til snapshots**, se [ZFS-datasæt](zfs-datasets.md) |

Indstillinger gemmer sig selv, mens du ændrer dem.

![Fanen Indstillinger for en container med Egen plan i det første kort.](assets/screenshots/backups/entry-settings-dark.png#only-dark)
![Fanen Indstillinger for en container med Egen plan i det første kort.](assets/screenshots/backups/entry-settings-light.png#only-light)

### Fanen Verificering {#verify-tab}

- **Kontroller** viser gendannelsestjekket, der følger efter den første sikkerhedskopi. **Tjek gendannelse** kører det igen på den nyeste sikkerhedskopi: BombVault gendanner en stikprøve på op til 100 filer og 256 MiB til en midlertidig mappe og lader restic læse hver fil tilbage mod dens hashes.
- **Starttest**, på en container, gendanner den nyeste sikkerhedskopi til en isoleret kopi og starter den: et navn, der begynder med `bombvault-test-`, et eget internt Docker-netværk uden udgivne porte og uden vej til LAN, 1 CPU og 2 GiB hukommelse. Den består, når kopien melder sund, svarer på sin første eksponerede port, eller, med ingen af delene, bliver ved med at køre. Originalen røres aldrig, og kopien, dens netværk og dens data fjernes bagefter, også efter en genstart midt i en test. Containere på værtsnetværket, privilegerede, dem med enheder og dem, der har brug for en anden container, vises som ikke testbare. **Starttest** under **Indstillinger, Verificering, Automatiske tjek** tester én container pr. kørsel, den der blev testet for længst tid siden først.
- **Afvigelser** sætter denne posts egen følsomhed og notifikationsniveau og viser dens åbne fund.
- **Størrelse pr. mappe** viser, hvad der fylder i den nyeste sikkerhedskopi, og hvor meget af det den seneste sikkerhedskopi bragte ind, ét niveau ad gangen.
- **Seneste kørsler** viser de sidste kørsler med start, slut og varighed. Et klik viser detaljerne.

![Fanen Verificering for en container med Størrelse pr. mappe åben.](assets/screenshots/backups/entry-verify-dark.png#only-dark)
![Fanen Verificering for en container med Størrelse pr. mappe åben.](assets/screenshots/backups/entry-verify-light.png#only-light)

## Udelukkelsesmønstre {#excludes}

Kortet **Udelukkelsesmønstre** tager ét mønster pr. linje. Skriv stier, som du ser dem inde i containeren, for eksempel `/config/Library/Application Support/Plex Media Server/Cache`, eller et blotte navn som `.git`, der passer på enhver dybde. På et mappesæt skriver du en sti, som værten kender den, for eksempel `/mnt/user/data/cache`. En forhåndsvisning under feltet viser, hvad hver linje løses til, og advarer, når en linje ikke passer på noget eller ville udelukke alt. En linje med `/` eller `*`, eller en, der nævner en hel sikkerhedskopieret mappe, ignoreres, så en udelukkelse aldrig kan tømme en sikkerhedskopi. For at udelade en hel mappe fjerner du i stedet fluebenet ved den under **Sikkerhedskopierede mapper**.

For velkendte images som Plex, Jellyfin, Sonarr, Radarr, Immich og Nextcloud tilbyder kortet de mapper, appen selv fylder op igen, for eksempel caches, logs og miniaturebilleder. Intet udelukkes, før du trykker på **Udeluk valgte**.

Det samme kort bærer advarsler, som en mappescanning ikke kan se. Immich gemmer for eksempel sine albummer, ansigter og datoer i en PostgreSQL-database i en separat container, så en filsikkerhedskopi af Immich-containeren alene gendanner billederne uden noget af det, og gendannelsen ser ud til at være lykkedes. Sikkerhedskopiér også databasecontaineren.

## Databasedumps {#database-dumps}

PostgreSQL-, MySQL- og MariaDB-containere, som BombVault genkender, dumpes før hver sikkerhedskopiering, fra den kørende server, til et gendannelsespunkt for sig ved siden af filerne. Det dækker de officielle images, PostGIS, TimescaleDB, pgvector, pgautoupgrade, Immichs databaseimages, MariaDB-imagene fra linuxserver, yobasystems og jc21 og Oracles mysql-server. En container, der blot ligner en database, får den samme mulighed, slået fra, til du vælger den.

Dumpet streames direkte ind i repositoryet og skrives aldrig til en disk. Adgangsoplysningerne kommer fra containerens egne variabler, `*_FILE`-hemmeligheder inklusive, og forlader den aldrig. Kortet **Database** på containerens fane Indstillinger siger, om datamappen gemmes, mens den er stoppet, kopieres, mens den kører, eller ikke gemmes, og slår dumpet til eller fra for denne container. Labelen `bombvault.dbdump=false` gør det samme og vinder over kortet. **Automatiske databasedumps** under **Indstillinger, Generelt** slår det til eller fra for dem alle.

Et mislykket dump får ikke sikkerhedskopieringen til at mislykkes. Det vises som en mislykket kørsel for sig med årsagen og sender en notifikation. Dumps indlæses aldrig tilbage af sig selv: Download et, almindeligt eller komprimeret, gem det i en mappe, eller importér det i en frisk startet database med ét klik på fanen **Sikkerhedskopier**. For at få et dump tilbage uden BombVault, se [Få et databasedump tilbage](offsite-recovery.md#database-dumps). Når et mislykkes, se [Et databasedump mislykkedes](troubleshooting.md#a-database-dump-failed).

## Kommandoer og andre containere {#commands}

- **Kommandoer før og efter** kører inde i containeren med `sh -c`, for eksempel for at skrive en cache ud til disk. En kommando før sikkerhedskopieringen, der mislykkes, afbryder den. En kommando før sikkerhedskopieringen, der slutter med kode 75, udskyder i stedet en planlagt sikkerhedskopiering, se nedenfor.
- **Stop andre containere** nævner containere, der skal stoppes, mens denne sikkerhedskopieres, for eksempel dens database. Bagefter starter de igen i deres Compose-`depends_on`-rækkefølge, og som standard skal hver melde sund, før den næste starter. Ventetiden og dens timeout står under **Genstart efter sikkerhedskopiering** på kortet **Rækkefølge** under **Indstillinger, Tidsplan**.
- En container, der deler netværks-, PID- eller IPC-namespace med en container, der stoppes, via `--net=container:` eller `network_mode: service:` i Compose, stopper og starter med den, også når den ikke står på listen, og den starter efter den container, den slutter sig til. En genstartet container får et nyt netværk, og en, der blev ved med at køre på det gamle netværk, ville kun have sin loopback. Selve listen bliver, som du har sat den. Kunne sådan en container ikke stoppes, siger kørslen, at den ikke har netværk, før den bliver genstartet. Det samme gælder BombVaults egen container, som aldrig stopper sig selv og ikke kan sættes på listen.
- En container, der ikke kører igen bagefter, nævnes i kørslen og i en notifikation, også når selve sikkerhedskopieringen mislykkedes.

## Når appen er optaget {#app-busy}

En sikkerhedskopiering stopper sin container, så en planlagt kan vente, mens nogen bruger appen, og prøve igen af sig selv. Kortet **Når appen er optaget** på en containers fane Indstillinger sætter det op:

- **Aktivitetstjek** spørger Plex, Jellyfin eller Emby før en planlagt sikkerhedskopiering, om nogen ser noget. Plex spørges med tokenet fra dens egen `Preferences.xml`, så der er intet at skrive. Jellyfin og Emby tager en **API-nøgle**, lavet i serverens dashboard under API-nøgler.
- **Ignorer sessioner på pause efter (min)** lader en film, der er sat på pause for natten, holde op med at tælle.
- **Sikkerhedskopier, når serveren ikke svarer** afgør, hvad der sker, når serveren ikke kan spørges: Slået til kører sikkerhedskopieringen, og loggen siger det. Slået fra venter den, som om nogen så noget.
- **Vent, mens containeren sender data** er til apps uden sådan et tjek, for eksempel en spilserver eller en downloader. Sikkerhedskopieringen venter, mens containeren sendte mere end **Optaget fra (KiB/s)** ved en eller anden måling inden for de seneste **Rolig i (min)** minutter.
- **Prøv igen hvert (min)**, **Prøv kun inden for et tidsvindue** med **Prøv mellem** og **Vent højst (timer)** sætter, hvordan forsøgene går.
- **Når ventetiden er slut** giver enten op, hvilket viser sikkerhedskopieringen under **Kræver opmærksomhed** med **Prøv igen**, eller **Sikkerhedskopier alligevel**.

En ventende sikkerhedskopiering registreres som **Udsat**, i rav, og resten af kørslen fortsætter. Oversigten, listen og posten viser den med det næste forsøg, og alarmen for forsinkelse og beskyttelsesstatus venter på den til dens frist. Du får én notifikation, når den udsættes, og én, når den slutter, ikke én pr. forsøg. Et forsøg venter, mens en anden sikkerhedskopiering kører, og en ventetid fortsætter efter en genstart. En sikkerhedskopiering, du selv starter, kører altid med det samme, hvad enten det er med **Sikkerhedskopier nu**, **Prøv igen**, et valg, API'et eller MCP, og en, der går igennem, afslutter en ventetid, der var i gang.

Containerens fane Indstillinger har også en ældre, enklere måde: **Vent, til appen er i ro** med **Vent højst (timer)**. Sikkerhedskopieringen starter, så snart appen er i ro, hvilket kortet **Ro før sikkerhedskopiering** under **Indstillinger, Tidsplan** definerer. Medlemmerne af en Compose-stak, der skal med i den samme kørsel, venter sammen, og den ventende sikkerhedskopiering holder ingen lås, så de andre containere går videre.

![Kortet Når appen er optaget for en Plex-container med aktivitetstjekket sat op.](assets/screenshots/backups/app-busy-dark.png#only-dark)
![Kortet Når appen er optaget for en Plex-container med aktivitetstjekket sat op.](assets/screenshots/backups/app-busy-light.png#only-light)

### Eksempel: Plex venter, mens nogen ser noget {#example-plex}

Den fulde sikkerhedskopi kører hver nat kl. 03:00. Ingen skal opleve, at en film stopper, fordi Plex gik ned til sin sikkerhedskopiering, men sikkerhedskopieringen skal alligevel ske før morgenen.

1. Åbn **Sikkerhedskopier**, så posten `plex`, så dens fane **Indstillinger**.
2. Sæt **Aktivitetstjek** til Plex under **Når appen er optaget**. Der er intet at skrive.
3. Sæt **Ignorer sessioner på pause efter (min)** til `30`, så en film, der er sat på pause ved midnat, holder op med at tælle klokken halv et.
4. Lad **Sikkerhedskopier, når serveren ikke svarer** stå slået til.
5. Sæt **Prøv igen hvert (min)** til `15`.
6. Slå **Prøv kun inden for et tidsvindue** til, og sæt **Prøv mellem** til `22:00` og `06:00`.
7. Sæt **Vent højst (timer)** til `3` og **Når ventetiden er slut** til **Sikkerhedskopier alligevel**.

Hvad der sker en nat, hvor nogen stadig ser noget kl. 03:00: BombVault spørger Plex, finder en session, der afspiller, og registrerer sikkerhedskopieringen som **Udsat** med det næste forsøg kl. 03:15. Kørslens andre containere sikkerhedskopieres som sædvanligt. Når filmen slutter kl. 04:10, finder forsøget kl. 04:15 Plex i ro og sikkerhedskopierer den, og ventetiden slutter med én notifikation. Kørte filmen stadig kl. 06:00, ville ventetiden nå sin slutning, og Plex ville blive sikkerhedskopieret alligevel, så dens sikkerhedskopier aldrig ligger mere end tre timer bagud.

Til dit eget tjek kan du få **Pre-sikkerhedskopikommando** til at slutte med kode 75, mens appen er optaget, for eksempel `[ "$(cat /tmp/players)" = 0 ] || exit 75`. Flere symptomer og løsninger står under [Fejlfinding](troubleshooting.md#deferred-backups).

## Almindelige eksporter {#exports}

restic forbliver motoren, men en eksport giver dig en kopi, der hverken kræver BombVault eller restic.

- **Eksporter container** og **Eksporter VM** under **Mere** på en posts side skriver en kopi, som den er lige nu, ved siden af repositoryet: containerens sikkerhedskopierede mapper plus dens Unraid-skabelon, eller VM'ens diskimages plus dens definition, som kan gendannes med `virsh define`.
- **Eksporter** på et gendannelsespunkt skriver en gemt sikkerhedskopi til eksportmappen på serveren, som tar, zip eller et restic-repository for sig, og kan tage ét gendannelsespunkt eller alle. En flash-sikkerhedskopi og en auto-sikkerhedskopi indeholder nøgler, så eksport af dem kræver en login-adgangskode og en login på instansen.

Eksporter er almindelige filer som standard. **Krypter eksporter med age** under **Indstillinger, System** forsegler dem, flash-ZIP'en inklusive, til de modtagere, du angiver, hver en offentlig age-nøgle eller en offentlig SSH-nøgle. Du dekrypterer dem senere uden for boksen med den tilhørende private nøgle. Med kryptering slået til og uden en gyldig modtager mislykkes en eksport med en tydelig fejl i stedet for nogensinde at skrive almindelige filer.

## Hvad en kørsel fortæller dig {#run-notes}

- **Tid.** Hver kørsel viser start, slut og varighed, og hver post har sine egne **Seneste kørsler**.
- **Hvorfor en sikkerhedskopiering var langsom.** Mens en sikkerhedskopiering kører, holder BombVault øje med, hvor travlt CPU'en, diskene og netværket har. Når en sikkerhedskopiering tager meget længere end sædvanligt, og én ting tydeligt var på sin grænse, siger kørslen det, for eksempel "Måldisken disk1 var 98 % optaget". Ellers siger den intet.
- **Fremdrift for tjek og beskæringer.** Aktivitetsloggen og kortet **Tjek sikkerhedskopier** viser, hvor langt restic har talt, for eksempel *12 af 47 pakker*, med den resterende tid i det trin, når der er nok til at anslå den.
- **Filer, den ikke kunne læse.** En sikkerhedskopiering, der gik igennem, men udeladte noget, gør sin linje orange og nævner, hvad der blev udeladt.
