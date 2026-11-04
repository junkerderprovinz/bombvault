# Off-site og gendannelse

!!! note "Off-site-kopier venter efter en genopbygning"
    Når nødguiden eller **Opdag sikkerhedskopier** genopbygger poster uden de gamle indstillinger, holder off-site-kopier af de områder pause, til deres standard er bekræftet. Se [Efter en genopbygning](#after-a-rebuild).

Lokale sikkerhedskopier beskytter dig mod en tabt container eller en dårlig opdatering. Off-site-kopier og et afprøvet gendannelseskit beskytter dig mod hele boksen, ransomware eller en brand. Denne side dækker destinationer og off-site-kopier, hvordan du gør en kopi manipulationssikker, hvordan du beviser, at du kan gendanne, parring af flere servere og hvordan du får alt tilbage, når BombVault selv er væk.

## Off-site-kopier {#off-site-copies}

Behold den hurtige lokale sikkerhedskopi, og tilføj en eller flere destinationer uden for huset. BombVault kopierer nye gendannelsespunkter dertil med `restic copy` efter bedste evne, så et hik off-site aldrig får den lokale sikkerhedskopi til at fejle. I denne form skriver hvert område først til sin egen mappe, og off-site-repositoryet er en kopi. Den egne mappe behøver ikke ligge på denne server: Se [Et områdes egen mappe](#own-folder) for at holde den på en destination som S3 eller en rest-server.

- **Flere destinationer pr. område.** Hvert område (containere, VM'er, flash, auto-sikkerhedskopi, mapper og ZFS-datasæt) kan kopiere til flere off-site-destinationer på én gang, så du for eksempel kan have en rest-server hos en ven og en S3-bucket ved siden af hinanden. Kortet **Destinationer pr. område** på siden Destinationer slår hver linje fra et område til en destination til eller fra.
- **Hvornår kopier kører.** En destinations kort **Kopier** sætter **Hvornår der kopieres**: **Efter hver sikkerhedskopi** eller efter en plan for sig selv, der samler gendannelsespunkterne og kopierer på faste tidspunkter. Under **Indstillinger, Tidsplan, Off-site-kopier pr. område** giver **Undtagelse** ét område sin egen kopiplan.
- **Kortet Eksterne kopier.** Under **Indstillinger, Destinationer** har hvert område med en off-site-kopi en række i **Eksterne kopier** med **Test forbindelse**, **Replikér nu** til at starte en kopi på forlangende og **Opsæt** eller **Rediger**. Resten af områdets kopiindstillinger åbner i dets vindue derfra.
- **Opbevaring** indstilles pr. destination under **Hvor længe der gemmes**, så du kan beholde off-site-kopier længere som et arkiv. Kopien i et områdes off-site-felt følger den som alle andre kopier, og et område kan beholde egne værdier under **Regler pr. område**. Alle nuller beskærer aldrig off-site-gendannelsespunkter. Se [Hvor længe der gemmes](#keep).
- **Båndbreddegrænser** (**Off-site-båndbredde** under **Indstillinger, Destinationer** eller en destinations kort **Kopier**) begrænser restics upload- og downloadhastighed, så kopier ikke mætter din WAN-forbindelse. De skrives i KiB/s, den enhed restic tager, og hvert felt viser den samme hastighed i MB/s og Mbit/s, mens du skriver: 2048 læses som omkring 2,0 MB/s (16,8 Mbit/s).
- **Komprimering** er **Fra**, **Automatisk** (restics standard) eller **Maksimum**, sat på hver destination og for ét område under dets regler dér. Sikkerhedskopier, kopier og beskæring skriver med den, og gendannelseskittet nævner den, så almindelig restic kan blive ved med at skrive på samme måde.
- **Streaming først.** Mens en medieserver som Plex, Jellyfin eller Emby streamer, uploader kopier med en lavere grænse og vender tilbage til den normale nogle få minutter efter, at streamen er slut. Kortet **Streaming** på siden Destinationer har **Sænk off-site-kopier under streaming**, **Medieservere** (containere med Plex, Jellyfin eller Emby i image-navnet, indtil du vælger), **Stream over (Mbit/s)**, **Upload-grænse under streaming (KiB/s)** og **Tilbage til normal efter (min)**. BombVault læser trafikken fra Docker, så en medieserver på værtsnetværket ikke kan måles. REST, S3, B2, Azure, Google Cloud, Swift og rclone over HTTP sænker farten midt i en kopi. SFTP og lokale eller monterede mapper tager den lavere grænse ved deres næste kopitrin.
- **Kold lagring (S3).** En S3-destination kan bruge en billigere **Lagerklasse**, som restic stadig kan læse med det samme, for eksempel Standard-IA, One Zone-IA, Intelligent-Tiering eller Glacier Instant Retrieval. Glacier Flexible Retrieval og Deep Archive er udeladt, fordi restic ikke kan gendanne fra dem. En rclone-remote sætter sin klasse i rclone-konfigurationen.
- **Vækstbudget.** **Vækstbudget (GB, 0 = fra)** i et områdes kopivindue slår alarm, når repositoryet vokser ud over den størrelse. Det sletter aldrig noget, så det passer til en append-only-kopi, som intet beskærer herfra.
- Mens en kopi kører, siger oversigtens statusdel **Off-site-kopi kører**. Det er en aktiv indikator, ikke en procentbjælke, fordi `restic copy` ikke afslører nogen maskinlæsbar fremdrift.

!!! note "Gendan fra ethvert sted"
    Hver post viser sine sikkerhedskopier som én tidslinje på tværs af alle steder, en sikkerhedskopi ligger, på sin fane **Sikkerhedskopier**. En sikkerhedskopi, der er kopieret til B2, vises én gang, mærket med hvert sted, der rummer den. En gendannelse tager det første sted, den kan nå, begyndende med det repository, posten skrives til, og du kan vælge et andet sted pr. gendannelsespunkt. Off-site-destinationer læses kun, når du klikker på **Tjek**, fordi det tager et stykke tid at liste cloud-lager. Sletning på ét sted tjekker først de andre og siger, om det var den sidste kopi. Se [Gendannelse](restore.md).

## Destinationer {#destinations}

En destination er et sted, sikkerhedskopier havner: en lokal mappe, en share, en server, du selv ejer, eller en cloudtjeneste. Den sættes op én gang og tjener så hvert område. Åbn **Destinationer** fra dens indstillingsflise, eller slå **Vis også i sidepanelet** til dér for at fæstne den.

![Siden Destinationer: listen over destinationer og flowet over, hvilket område der går hvorhen.](assets/screenshots/storage/locations-dark.png#only-dark)
![Siden Destinationer: listen over destinationer og flowet over, hvilket område der går hvorhen.](assets/screenshots/storage/locations-light.png#only-light)

**Tilføj destination** åbner et vindue i fire trin:

1. **Udbyder.** Hver tjeneste står med sit logo, med en søgning øverst til højre i vinduet, i grupper: **Lokal** til en disk eller share, der er sluttet til denne server, **NAS eller server** (rest-server, Hetzner Storage Box, SFTP, SMB, WebDAV og andre), **S3-tjeneste** (Backblaze B2, Wasabi, Cloudflare R2, Hetzner Object Storage, Amazon S3 og andre), din egen S3-server (Garage, SeaweedFS, RustFS, Silo, Ceph, JuiceFS, Versity S3 Gateway), **Cloud-lagring** (OneDrive, Google Drive, Dropbox, pCloud, Nextcloud og resten, rclone understøtter) og **Fra din gruppe**, de modtagende servere i din gruppe. Hver siger, hvor godt den passer til sikkerhedskopier: Clouddrev går i stå under mange forespørgsler, så den første sikkerhedskopi og beskæring tager længere tid dér. Til en tjeneste, ingen flise dækker, tager fliserne **Rclone** under **Egen rclone-konfiguration** remotes, der er indsat fra en rclone.conf.
2. **Log ind.** Felterne afhænger af tjenesten: en adgangsnøgle til S3, et brugernavn og en adgangskode til WebDAV og SMB, en app-adgangskode, hvor tofaktorlogin blokerer den almindelige, BombVaults offentlige SSH-nøgle til SFTP og Storage Box eller et token til tjenester, der logger ind gennem en browser. Til dem viser vinduet en `rclone authorize`-kommando, der skal køres på en computer med en browser, og det token, den skriver, indsættes i feltet. Hvor du allerede har [gemte adgangsoplysninger](#sign-ins) til tjenesten, vælger **Gemte adgangsoplysninger** dem i stedet for **Nye nøgler**. **Test forbindelse** tjekker loginet, før noget gemmes. Vinduet viser så mapperne på destinationen, med **Ny mappe** til at oprette en og den ledige plads, hvor tjenesten melder den. En tom mappe er sikrest. BombVault lægger én mappe pr. område i den.
3. **Beskyttelse.** Vinduet siger ligeud, hvad tjenesten kan. En rest-server i append-only-tilstand afviser sletning, og manipulationstesten tjekker det. En S3-bucket kan beholde gamle versioner med versionering og objektlås, hvilket BombVault endnu ikke kan tjekke. Et clouddrev kan slet ikke afvise sletning: Den, der kommer ind på serveren, kommer også ind på den kopi. Slå **Beskyttelse mod sletning (append-only)** til, kun hvor den fjerne side virkelig afviser sletning. BombVault beskærer så aldrig dér. **I en nødsituation** siger, hvad gendannelseskittet rummer for denne destination.
4. **Til hvilke områder.** Vælg de områder, der sikkerhedskopierer hertil. Den første kopi tager hele historikken med. Poster, der har valgt deres egne destinationer, får ikke den nye af sig selv, og vinduet nævner dem.

![Tilføj destination: det første trin med hver udbyder som en flise.](assets/screenshots/storage/add-providers-dark.png#only-dark)
![Tilføj destination: det første trin med hver udbyder som en flise.](assets/screenshots/storage/add-providers-light.png#only-light)

![Tilføj destination: loginstrinnet efter en vellykket test.](assets/screenshots/storage/add-signin-dark.png#only-dark)
![Tilføj destination: loginstrinnet efter en vellykket test.](assets/screenshots/storage/add-signin-light.png#only-light)

Nogle tjenester beder om lidt mere ved login:

- **SFTP og Storage Box.** **Tilføj nøgle** lægger BombVaults offentlige nøgle på serveren med **Kontoens adgangskode**, som bruges til det ene login og ikke gemmes nogen steder. Kommandoen til at gøre det i hånden står under den. Testen viser **Servernøgle** med dens fingeraftryk. BombVault husker den og afviser serveren, hvis den senere viser en anden.
- **S3-nøgler til en enkelt bucket**, for eksempel et Cloudflare R2-objekttoken eller en afgrænset AWS-nøgle, kan måske ikke liste de andre buckets. Skriv bucketten i **Bucket**, så åbner testen og mappelisten den bucket.
- **S3-regioner og adresser.** Regionen læses fra et endpoint, der nævner den (AWS, Backblaze B2, Hetzner, Scaleway, OVH, Wasabi), Garage starter med sin egen standardregion, og IDrive e2 beder om det endpoint, dets dashboard viser. En adresse indsat med bucketten på, som R2-dashboardet viser den, udfylder serveren og bucketten. En bucket, der er lavet fra mappebrowseren, havner i den region, formularen nævner.
- **Lagerklasse.** Vælgeren tilbyder kun de klasser, tjenesten tager, og **Flere indstillinger** viser kun de muligheder, der former en ny bucket.
- **Google Drive og OneDrive.** Beskårne data slettes for altid i stedet for at gå i papirkurven, hvor de stadig ville tælle med i kvoten. Et delt Google-drev står under **Flere indstillinger**.
- **SMB-servere, der ikke lister deres shares,** tager navnet på en under **Deling**, og testen åbner den share direkte.
- **FTP** beder om sin **Kryptering**: **Ingen**, **Eksplicit TLS (FTPES)** eller **Implicit TLS (FTPS, port 990)**.
- **OneDrive og Zoho WorkDrive.** Når et login åbner flere drev eller arbejdsområder, spørger vinduet, hvilket sikkerhedskopierne skal til.
- **En rest-server med et selvsigneret certifikat.** Læg certifikatet som en PEM-fil i mappen `certs` i BombVaults datamappe (`/config/certs` i containeren, `appdata/bombvault/certs` på Unraid). Sikkerhedskopier, kopier og testen af beskyttelsen mod sletning stoler på det uden en genstart.

S3-tjenester kører gennem restics egen S3-backend, og det er det, der lader en lagerklasse og objektlås gælde. En rest-server nås direkte af restic. Alle andre tjenester kører gennem den rclone, BombVault leverer, og deres afsnit af rclone-konfigurationen vises i kortet **rclone.conf** på destinationens side med hemmelighederne skjult. En eksport af indstillinger bærer destinationerne, og med adgangsoplysninger medtaget bærer den også deres login.

Når du gemmer en destination, der allerede rummer sikkerhedskopier af containere, VM'er eller mapper, for eksempel en du fjernede og tilføjer igen, føjes den til de områder som en kopi uden flueben. Intet kopieres dertil, før du sætter flueben, men **Gendan** tilbyder de gamle sikkerhedskopier med det samme.

Mappen på denne server hedder **Lokal** overalt: på lagersiderne, i flowet, på en post og på Gendannelse. Med flere mapper på denne server går hver under sit eget navn. En destination, som et områdes off-site-felt sætter op, opkaldes efter sit sted, den sidste mappe i dens adresse eller, uden en, dens server, og får et nummer, når en anden destination allerede går under det navn. Hver destination kan omdøbes i feltet **Navn** i dens kort **Forbindelse**, også en bucketmappe, som flere områder deler, og den kopi, et områdes off-site-felt beskriver, og et navn, du giver den, bliver stående. Hvor to destinationer med samme navn står side om side, bærer hver sin server eller mappe.

### En destinations side {#location-page}

Hver destination har en side for sig. Den viser kun de kort, der gælder for den destination, i denne rækkefølge:

![En destinations side: Egne mapper, Forbindelse og Legitimationsoplysninger.](assets/screenshots/storage/location-page-dark.png#only-dark)
![En destinations side: Egne mapper, Forbindelse og Legitimationsoplysninger.](assets/screenshots/storage/location-page-light.png#only-light)

- **Egne mapper**: de områder, der skriver deres sikkerhedskopier hertil først. Denne destinations login, beskyttelse og grænser gælder for dem. **Brug som egen mappe** flytter flere områders egne mapper hertil, og **Skift** flytter en væk igen. Se [Et områdes egen mappe](#own-folder).
- **Sikkerhedskopierer direkte hertil**: hvert område, hvis poster sikkerhedskopierer direkte til dette sted i stedet for til deres egen mappe, og hvor mange poster der gør det. Sådan et direkte repository henter alle sine indstillinger fra denne destination. Et, som intet sikkerhedskopierer til mere, siger det, og **Fjern** tager det af listen. Sikkerhedskopierne på dets sted bliver. Se [Direkte repositories](#direct-repositories).
- **Forbindelse**: **Navn**, kontakten **Tilgængeligt** (slået fra betyder, at BombVault ikke længere skriver hertil, og at poster, der sikkerhedskopierer hertil, fejler med en tydelig besked i stedet for at havne et andet sted), mappen, **Komprimering**, **Lagerklasse** og **Uden for lokationen**. På destinationen **Lokal** lister den mappen for hvert område under **Egne mapper her**. **Skift forbindelse** tester og gemmer en ny adresse eller et nyt login. Når andre steder logger ind med de samme gemte adgangsoplysninger, siger vinduet det, og en ændring her giver denne destination egne adgangsoplysninger i stedet for at omskrive de delte.
- **Legitimationsoplysninger**: hvilke gemte adgangsoplysninger den bruger (**Logger ind med**), hvilke andre destinationer der også bruger dem (**Bruges også af**) og **Rediger**, som åbner vinduet til adgangsoplysninger. Se [Adgangsoplysninger](#sign-ins).
- **rclone.conf**: denne destinations afsnit af rclone-konfigurationen, for en destination der kører gennem rclone.
- **Beskyttelse mod sletning**, med **Beskyttelse mod sletning (append-only)**, **Accepter uden beskyttelse mod sletning**, **Test beskyttelsen mod sletning nu** og **Hvem pruner**, som siger, om BombVault beskærer kopien og de egne mapper her eller overlader det til den fjerne side. En lokal mappe viser kun dette kort, når den har en beskyttelse, du kan slå fra. Se [Beskyttelse mod sletning](#delete-protection-append-only).
- **Kopier**: **Hvornår der kopieres** og **Upload-grænse (KiB/s)** og **Download-grænse (KiB/s)**. En destination, der kun rummer egne mapper og ingen kopier, viser grænserne som et kort kaldet **Båndbredde**.
- **Hvor længe der gemmes**: **Kort**, **Afbalanceret**, **Lang** eller **Egne værdier**, med en tidslinje over, hvilke sikkerhedskopier reglen beholder. Se [Hvor længe der gemmes](#keep).
- **Ligger allerede under denne destination**: områder, hvis håndskrevne adresse ligger i en mappe på denne destination. At overtage et lader repositoryet blive, hvor det er. Login, lagerklasse og beskyttelse mod sletning kommer så fra destinationen. BombVault tjekker først, at destinationens login åbner repositoryet, og afviser at lægge et append-only-repository under en destination, der ikke er append-only.
- **Regler pr. område**: ét områdes eget repository, opbevaring eller komprimering på denne destination. **Ændr regler** åbner dem, og **Tilbage til destinationen** henter et område tilbage. Login, båndbredde og hvornår kopier kører kommer altid fra destinationen.
- Et kar, der viser, hvor fuld destinationen er, med mærket for, hvor den er fuld, og hvor mange uger der er tilbage ved de seneste ugers vækst. En destination, der ikke melder sin størrelse, siger **Størrelse ukendt**.

**Flyt** og **Fjern destination** står blandt sidens handlinger, beskrevet under [Flyt og fjern](#move-remove).

En fjern destination, som rummede sikkerhedskopier og pludselig ikke rummer noget repository, for eksempel en rest-server, hvis disk ikke blev monteret, får ikke et nyt, tomt af sig selv. Dens side siger det, så snart en sikkerhedskopiering eller kopi finder repositoryet væk. Montér disken igen. Hvis du med vilje har tømt det gamle repository, starter **Opret et nyt depot her** forfra, for hvert område på destinationen og dens repositories på én gang.

En modtagende server, som en anden instans i din gruppe kører, vises blandt udbyderne som **Fra din gruppe**. Se [Modtageserver](#receiving-server).

BombVault tjekker sine destinationer ved hver start. Den retter det, der ikke kræver en beslutning fra dig, for eksempel et link til en kopi eller et repository, der er væk, en kopi på et områdes egen mappe eller et direkte repository, hvis kopi er væk, og skriver en linje i loggen for hver rettelse. Det, der ikke har nogen sikker rettelse, logges kun.

![Den samme side på en telefon.](assets/screenshots/storage/location-page-phone-dark.png#only-dark)
![Den samme side på en telefon.](assets/screenshots/storage/location-page-phone-light.png#only-light)

### Hvor længe der gemmes {#keep}

Hvor længe sikkerhedskopier bliver, indstilles pr. destination i dens kort **Hvor længe der gemmes**. Reglen dækker kopierne på den destination og de egne mapper, der ligger dér, **Lokal** inklusive. Et område kan beholde egne værdier på én destination under **Regler pr. område**, og de vinder over destinationens kort for det område.

En regel beholder én sikkerhedskopi pr. dag, uge, måned og år, så langt tilbage som du angiver, og rydder resten ud efter hver sikkerhedskopiering eller kopi:

| Forudindstilling | Seneste sikkerhedskopier | Dage | Uger | Måneder | År |
|---|---|---|---|---|---|
| **Kort** | 0 | 7 | 4 | 3 | 0 |
| **Afbalanceret** | 0 | 7 | 4 | 6 | 1 |
| **Lang** | 0 | 14 | 8 | 12 | 3 |

**Egne værdier** sætter de fem tal selv. Et 0 slår det tal fra, og en regel med alle tal på 0 beholder alt. Prikkerne i kortet viser, hvilke af 40 eksempelsikkerhedskopier en regel beholder, med den nyeste yderst til højre. Eksemplet er det samme for hver destination, da billedet tegnes, før BombVault har læst sikkerhedskopierne dér.

**Spørg før der gemmes mindre**, slået til som standard, spørger, før en ændring, der beholder færre sikkerhedskopier, gemmes, og siger, hvor mange af de 40 eksempelsikkerhedskopier den nye regel fjerner. Den gælder for hver destination. **Tilbage til destinationen** under **Regler pr. område** stiller det samme spørgsmål.

En destination med beskyttelse mod sletning beskæres aldrig fra denne boks. Dens fjerne side beholder eller dropper gamle sikkerhedskopier efter sine egne regler. Mens en post har en åben kritisk afvigelse om tabte data, beholder opbevaringen postens gamle sikkerhedskopier, til du kvitterer for den. Se [Afvigelser](anomalies.md).

For at se, hvad den næste kørsel virkelig ville slette, åbn **Forhåndsvisning** under **Indstillinger, Destinationer**. Den viser **Hvad den næste kørsel ville fjerne**, pr. repository og pr. post med gendannelsespunkterne nævnt. Den tager ingen lås og ændrer intet.

![Hvor længe der gemmes med Afbalanceret valgt: Prikkerne viser, hvilke af de 40 eksempelsikkerhedskopier der bliver.](assets/screenshots/storage/keep-dark.png#only-dark)
![Hvor længe der gemmes med Afbalanceret valgt: Prikkerne viser, hvilke af de 40 eksempelsikkerhedskopier der bliver.](assets/screenshots/storage/keep-light.png#only-light)

#### Eksempel: hvad Afbalanceret beholder {#example-retention}

En container er blevet sikkerhedskopieret hver nat kl. 03:00 siden 1. oktober 2025, så onsdag den 4. november 2026 har den 400 sikkerhedskopier. Dens destination er sat til **Afbalanceret**: 7 dage, 4 uger, 6 måneder og 1 år.

Hvert tal beholder den nyeste sikkerhedskopi fra sin dag, uge, måned eller år, og en sikkerhedskopi, som flere tal vælger, beholdes én gang. Uger går fra mandag til søndag:

| Regel | Sikkerhedskopier den vælger | Nye, den tilføjer |
|---|---|---|
| 7 dage | 4. nov, 3. nov, 2. nov, 1. nov, 31. okt, 30. okt, 29. okt | alle 7 |
| 4 uger | 4. nov, 1. nov, 25. okt, 18. okt | 25. okt, 18. okt |
| 6 måneder | 4. nov, 31. okt, 30. sep, 31. aug, 31. jul, 30. jun | 30. sep, 31. aug, 31. jul, 30. jun |
| 1 år | 4. nov | ingen |

Det efterlader 13 sikkerhedskopier ud af 400. Den ældste er fra 30. juni, så du kan gå lidt over fire måneder tilbage. Takket være deduplikering fylder de kun lidt mere end én fuld sikkerhedskopi plus det, der er ændret imellem.

**År** er værd at se en ekstra gang på. Det tæller kalenderår med en sikkerhedskopi i, begyndende med det nuværende, og den nyeste sikkerhedskopi i år er dagens. Så et 1 i **År** tilføjer intet til de andre tal, og du skal bruge et 2 for at nå ind i sidste år. De samme 400 sikkerhedskopier under de andre forudindstillinger:

| Forudindstilling | Sikkerhedskopier beholdt | Ældste sikkerhedskopi |
|---|---|---|
| **Kort** | 10 | 30. september 2026 |
| **Afbalanceret** | 13 | 30. juni 2026 |
| **Lang** | 29 | 31. december 2025 |

At gå fra **Afbalanceret** til **Kort** spørger først på grund af **Spørg før der gemmes mindre**: "Af de 40 sikkerhedskopier på billedet fjerner den nye regel 3, som den nuværende beholder." Tallet kommer fra eksemplet på billedet, ikke fra dine egne sikkerhedskopier. Her passer det tilfældigvis: 31. august, 31. juli og 30. juni ville forsvinde. **Gem alligevel** anvender reglen, og den næste beskæring sletter dem for altid fra den destination.

For at beholde containerne længere end resten på denne destination skal du trykke på **Ændr regler** under **Regler pr. område**, vælge **Containers** og sætte **Egne værdier** i **Regler for Containers**, for eksempel 14 dage, 8 uger, 12 måneder og 2 år. De andre områder følger stadig destinationens kort, og **Tilbage til destinationen** henter Containers tilbage.

### Adgangsoplysninger {#sign-ins}

Adgangsoplysninger er den nøgle eller adgangskode, BombVault bruger til en S3-tjeneste eller en rest-server. Adgangsoplysninger er én liste, kortet **Adgangsoplysninger** under **Indstillinger, Destinationer**, og hver række siger, hvilke destinationer der bruger dem (**Bruges af**).

- **Gemt, når du tilføjer en destination.** Tilføj destination gemmer de nøgler, du skriver, som adgangsoplysninger. Den næste destination hos samme tjeneste kan vælge dem under **Gemte adgangsoplysninger** i stedet for **Nye nøgler**, og en senere ændring af dem gælder for hver destination, der bruger dem.
- **Standarden.** En stjerne markerer standardadgangsoplysningerne. De bruges, hvor ingen er valgt, for eksempel en adresse skrevet i hånden, en hentningskilde eller en genopretning på en frisk installation. **Gør til standard** giver stjernen til andre.
- **Rediger og Test.** **Rediger**, her eller i kortet **Legitimationsoplysninger** på en destination, åbner ét vindue til adgangsoplysningerne. **Test** logger ind på hver destination, der bruger dem, med det, du har skrevet, før noget gemmes. **Gem** gemmer dem.
- **Fjernes.** Adgangsoplysninger kan kun fjernes, når ingen destination eller hentningskilde bruger dem, og standarden kan kun gå som den sidste. Rækken siger så, hvad der stadig bruger dem.
- **At ændre en destinations forbindelse** omskriver aldrig adgangsoplysninger, som andre steder deler. Destinationen får i stedet egne adgangsoplysninger, og vinduet siger på forhånd, at dens adgangsoplysninger er delt. For at ændre de delte overalt skal du redigere dem i kortet **Adgangsoplysninger**.

![Kortet Adgangsoplysninger: hver gemt adgangsoplysning med det, der bruger den, og stjernen ved standarden.](assets/screenshots/storage/signins-dark.png#only-dark)
![Kortet Adgangsoplysninger: hver gemt adgangsoplysning med det, der bruger den, og stjernen ved standarden.](assets/screenshots/storage/signins-light.png#only-light)

En mappe på denne server behøver ingen adgangsoplysninger. SMB, WebDAV, SFTP, Storage Box og clouddrevene har deres i rclone-konfigurationen, som destinationens kort **rclone.conf** viser, så de vises ikke på denne liste.

### SMB, WebDAV og andre rclone-remotes {#smb-webdav}

En Windows- eller Samba-share og en WebDAV-server (Nextcloud, ownCloud, SharePoint eller en anden) er udbyderfliser i **Tilføj destination** under **NAS eller server**. Udfyld vært og share (SMB) eller adresse og servertype (WebDAV), brugeren og adgangskoden, så skriver BombVault rclone-afsnittet for dig. rclone slører adgangskoden, før den gemmes.

Til en tjeneste, ingen flise dækker, tager fliserne **Rclone** under **Egen rclone-konfiguration** remotes, der er indsat fra en rclone.conf. De føjes til de remotes, BombVault allerede har, og erstatter dem aldrig, og de gemmes krypteret. For at sikkerhedskopiere et område til en af dem skal du skrive `rclone:<remote>:<folder>` som det områdes off-site-repository, for eksempel `rclone:nas:backups/bombvault`. Sharen eller bucketten er det første stisegment, ikke en del af navnet.

Det er den bedre vej end at montere sharen på Unraid: restic fraråder at holde et repository på en monteret CIFS-share, og her monteres intet. NFS har ingen flise, fordi hverken restic eller rclone har en NFS-backend. Til NFS skal du montere eksporten på værten og bruge den som en lokal mappe.

### Flyt og fjern {#move-remove}

- **Flyt** tager ét områdes kopi til en anden destination, med hele dens historik. Alt bliver i det gamle sted. BombVault sletter intet dér. Flytningen kører i baggrunden og kan tage lang tid: omkring 9 minutter pr. 100 GB mellem lokale diske, timer over en langsom forbindelse. Oversigten viser den, mens den kører.
- **Fjern destination** tilbyder to veje. **Fjern destination, behold sikkerhedskopierne** holder op med at skrive dér og lader sikkerhedskopierne blive. Du kan læse dem senere under **Gendannelse, Fra en gammel sikkerhedskopimappe**. **Fjern destination og slet sikkerhedskopierne dér** læser først, hvad der er gemt dér, og lister det: hvilke sikkerhedskopier der går, hvilke af dem der ikke findes andre steder, og hvilke repositories der bliver, fordi beskyttelse mod sletning afviser, poster stadig sikkerhedskopierer direkte til dem, en anden destination eller et andet område har sine kopier dér, eller de ikke kunne læses. Poster, der ville miste deres eneste sikkerhedskopi, står for sig og beholdes, medmindre du slår **Slet også disse** til. Du bekræfter ved at skrive destinationens navn, og sletningen kører i baggrunden.
- Det virker for alle slags destinationer: en som flere områder bruger, et mål sat op til ét område, et repository, du har sat op, og den mappe, et område skriver sine egne sikkerhedskopier til. Poster, der sikkerhedskopierer til et fjernet repository, følger deres område igen, og det gør en områdestandard, der pegede dertil, også. Et områdes mappe kan ikke bare forsvinde, for hvert område skal have en. Vinduet spørger efter den mappe, områderne flytter til, og hvert område beholder sit mappenavn under den.
- At slette sikkerhedskopierne på en destination kræver en loginadgangskode og et login på instansen. Uden en adgangskode kan destinationen stadig fjernes, bare ikke dens sikkerhedskopier slettes herfra.

### Eksempel: en off-site-kopi på Backblaze B2 {#example-s3}

Containerne og auto-sikkerhedskopien skal også ligge hos Backblaze B2, så en død server ikke tager dem med. De samme trin passer til Wasabi, Hetzner Object Storage og de fleste andre S3-tjenester.

1. **I B2** opretter du en privat bucket, for eksempel `tower-backups`, i en region nær dig. Under **Application Keys** opretter du en nøgle med læse- og skriveadgang, begrænset til den bucket, og noterer dens nøgle-ID og applikationsnøgle. B2 viser applikationsnøglen én gang. Sæt bucketens livscyklus til kun at beholde den sidste version, ellers beholder B2 beskårne data som skjulte versioner og sender dig en regning for dem.
2. **Udbyder.** Åbn **Destinationer** i BombVault, tryk på **Tilføj destination**, og vælg **Backblaze B2** under **S3-tjeneste**.
3. **Log ind.** Skriv nøgle-ID'et som **Adgangsnøgle-ID** og applikationsnøglen som **Hemmelig adgangsnøgle**. Skriv bucketens endpoint under **Endpoint**, for eksempel `s3.eu-central-003.backblazeb2.com`, så læser BombVault regionen fra det. Skriv `tower-backups` i **Bucket**, fordi denne nøgle kun åbner én bucket.
4. **Test forbindelse.** Den skal sige **Forbindelsen er oprettet** og vise bucketens mapper. Tryk på **Ny mappe**, og kald den `bombvault`. BombVault lægger én mappe pr. område i den, for eksempel `bombvault/containers`.
5. **Beskyttelse.** Vinduet forklarer, at B2 kan beholde gamle versioner med objektlås, og at BombVault ikke kan tjekke det. Lad **Beskyttelse mod sletning (append-only)** stå slået fra, medmindre bucketen virkelig afviser sletninger, så BombVault kan beskære efter destinationens behold-regler.
6. **Til hvilke områder.** Sæt flueben ved **Containers** og **Auto-sikkerhedskopi**. Auto-sikkerhedskopien bærer dine indstillinger, så en ny installation kan hente dem tilbage. Vinduet siger, hvor mange poster, og hvor det er kendt, hvor meget historik den første kopi tager med.
7. Tryk på **Gem destination**.

Den første kopi kører efter den næste sikkerhedskopiering. For at starte den nu skal du åbne **Indstillinger, Destinationer** og trykke på **Replikér nu** i områdets række under **Eksterne kopier**. Statusdelen siger **Off-site-kopi kører**. Den første kopi tager hele historikken med, så 200 GB over en 100 Mbit/s-uplink tager omkring fire en halv time. Senere kopier sender kun det nye.

For at holde din linje fri åbner du destinationens side og sætter **Upload-grænse (KiB/s)** i kortet **Kopier**, for eksempel til `5120`, som læses som omkring 5,0 MB/s (41,9 Mbit/s). Tjek også **Hvor længe der gemmes** på samme side, mens du er der.

For at se, at det virkede, så se på **Beskyttelsesstatus** på oversigten, hvor **Off-site** skifter til **kopi opdateret**, og på en containers fane **Sikkerhedskopier**, hvor dens gendannelsespunkter nu nævner `tower-backups` som et andet sted. Tryk så på **Download gendannelseskit** igen under **Indstillinger, Gendannelse**, fordi kittet lister den nye destination og dens login.

**Cloudflare R2 virker på samme måde,** med et objekttoken i stedet for en nøgle. Indsæt den S3-adresse, R2-dashboardet viser, med bucketten på, så udfylder BombVault serveren og bucketten.

### Eksempel: en rest-server, du kører selv {#example-rest-server}

En rest-server i append-only-tilstand afviser sletninger, så en kopi på den overlever, selv om nogen overtager din server. En anden boks, hos en ven eller i garagen, er nok. En parret BombVault kan sætte den op for dig, se [Modtageserver](#receiving-server). For at køre den i hånden på enhver Docker-vært:

```sh
docker run -d --name rest-server --restart unless-stopped \
  -p 8000:8000 \
  -v /mnt/user/backups/rest-server:/data \
  -e OPTIONS="--append-only --private-repos" \
  restic/rest-server
docker exec -it rest-server create_user bombvault
```

Den sidste linje beder om en adgangskode og skriver brugeren i filen `.htpasswd` i datamappen.

1. Tryk på **Tilføj destination** i BombVault, og vælg **rest-server** under **NAS eller server**.
2. Skriv adressen med brugeren som mappe, for eksempel `http://192.168.2.20:8000/bombvault`, og **Brugernavn** og **Adgangskode**. Tryk på **Test forbindelse**.
3. Slå **Beskyttelse mod sletning (append-only)** til under **Beskyttelse**. BombVault beskærer så aldrig dér og overlader det til den fjerne side.
4. Sæt flueben ved områderne, og tryk på **Gem destination**.
5. Når den første kopi er der, åbn destinationens side og tryk på **Test beskyttelsen mod sletning nu**. BombVault prøver at slette et testobjekt. Kortet skal sige **Sidst: sletning afvist**, og **Beskyttelse mod sletning** på oversigten skifter til **kontrolleret**. Hvis der står **Sidst: sletning ikke afvist**, mangler `--append-only`.

Da intet beskærer denne kopi herfra, skal du sætte **Vækstbudget (GB, 0 = fra)** i områdets vindue, åbnet med **Rediger** i dets række under **Eksterne kopier**, så du hører om det, før disken løber fuld. Med et selvsigneret certifikat, se noten om `certs` ovenfor.

## Et områdes egen mappe {#own-folder}

Hvert område har en egen mappe: det repository, dets sikkerhedskopier skrives til først. Kopier til andre destinationer går ud derfra. En ny installation holder de egne mapper på denne server under `/mnt/user/bombvault`, og tilsammen udgør de destinationen **Lokal**.

En egen mappe kan også ligge på en destination, du allerede har, for eksempel en NAS, en rest-server eller en S3-bucket. Området sikkerhedskopierer så direkte dertil uden en lokal kopi imellem, og den egne mappe følger den destinations login, beskyttelse mod sletning, båndbreddegrænser og behold-regler. Destinationen lister området i sit kort **Egne mapper**, og **Destinationer pr. område** viser destinationen som områdets hjem.

### Flyt en egen mappe {#move-own-folder}

1. Åbn den destination, der skal rumme den egne mappe, og vælg **Brug som egen mappe** eller **Tilføj sektioner** i dens kort **Egne mapper**. For at flytte flere områder på én gang skal du vælge **Flyt egne mapper** ved siden af **Egne mapper her** i kortet **Forbindelse** på destinationen Lokal.
2. Vælg områderne under **Områder** og stedet under **Flyt til**. **Et andet sted** tager en mappe på denne vært eller adressen på et fjernt repository skrevet i hånden, med et login fra dine gemte adgangsoplysninger.
3. Læs **Hvad der sker**. For hvert område siger det, hvor nye sikkerhedskopier havner, om en kopi, der allerede ligger på det nye sted, bliver den egne mappe, og hvilke sikkerhedskopier der kopieres over først.
4. **Kopiér de allerede lavede sikkerhedskopier** er slået til som standard. Slået fra bliver de gamle sikkerhedskopier, hvor de er, og kan stadig gendannes derfra.
5. Vælg **Skift**. Når sikkerhedskopier først skal kopieres, kopierer BombVault dem og skifter så den egne mappe. Når de allerede er der, tjekker den dem før skiftet.

Efter flytningen havner nye sikkerhedskopier i den flyttede egne mappe, også for et område, der havde **Lokal** slået fra og sikkerhedskopierede direkte til en kopi. En kopi, der allerede lå på det nye sted, bliver den egne mappe, og kopieringen dertil ophører. Hvor den kopi beholdt nogle sikkerhedskopier længere end områdets behold-regler, beholder området alt det, begge regler beholder, så ingen af dem går tabt.

Når flytningen ikke kan gå videre, siger vinduet hvorfor i en boks under **Hvad der sker**. Når kun kopieringen af de gamle sikkerhedskopier er i vejen, lader det at slå **Kopiér de allerede lavede sikkerhedskopier** fra den egne mappe flytte uden dem.

Et område har altid brug for en egen mappe. At fjerne den destination, der rummer den, spørger efter den mappe, området flytter til, se [Flyt og fjern](#move-remove). En egen mappe, som nye sikkerhedskopier går uden om, fordi området sikkerhedskopierer direkte til en kopi, er markeret **(fra)**, hvor en destination lister, hvem der bruger den.

![Vinduet, der flytter en egen mappe, med Hvad der sker udfyldt for ét område.](assets/screenshots/storage/move-own-folder-dark.png#only-dark)
![Vinduet, der flytter en egen mappe, med Hvad der sker udfyldt for ét område.](assets/screenshots/storage/move-own-folder-light.png#only-light)

#### Eksempel: Containers flytter til en NAS {#example-own-folder}

Cache-puljen er ved at blive fuld, og du har en destination kaldet `nas`, en SMB-share på en anden boks. Området Containers har 61 sikkerhedskopier i sin egen mappe på denne server og kopierer til Backblaze B2.

1. Åbn **Destinationer**, så `nas`, og tryk på **Brug som egen mappe**.
2. Sæt flueben ved **Containers** under **Områder**.
3. Læs **Hvad der sker**. Det siger, at nye sikkerhedskopier af Containers havner i mappen på `nas`, at de 61 sikkerhedskopier i Lokal kopieres derhen først, at kopier fortsætter til Backblaze B2, og at beskyttelse, båndbreddegrænser og vækstalarmen kommer fra `nas`.
4. Lad **Kopiér de allerede lavede sikkerhedskopier** stå slået til, og tryk på **Skift**.

BombVault kopierer sikkerhedskopierne i baggrunden og skifter så den egne mappe. Siden `nas` lister Containers under **Egne mapper**, en post, der følger sit område, nævner `nas` som sit hjem, og den næste sikkerhedskopiering havner dér, med kopien til B2 ud fra `nas`. Den gamle mappe på denne server bliver, hvor den er, og BombVault sletter intet dér.

### Skriv en fjern adresse i hånden {#remote-primary-repositories}

Kortet **Forbindelse** på destinationen Lokal tager stadig en fjern adresse til et områdes egen mappe: **Skift** ved siden af området åbner et vindue med en kontakt **Lokal / Fjern**. **Fjern** tager en restic-adresse som `s3:...`, `rest:http://host:8000/repo`, `sftp:user@host:/repo` eller `rclone:remote:bucket/path`, og **Sikkerhedsindstillinger for fjern primær-repo** tilføjer en forbindelsestest, upload- og downloadgrænser, append-only-beskyttelse med den samme manipulationstest, en off-site-destination får, og en vækstbudgetalarm.

En sådan adresse logger ind med standardadgangsoplysningerne. Når den ligger i en mappe på en destination, du har, lister den destinations side området under **Ligger allerede under denne destination**, og at overtage det giver det destinationens login, lagerklasse og beskyttelse mod sletning. At flytte den egne mappe ind på en destination med **Brug som egen mappe** er den enklere vej, fordi vinduet tjekker de allerede lavede sikkerhedskopier, før noget ændres.

## Placering pr. element {#placement}

Kortet **Tidsplan og destination** på et elements fane Indstillinger har en række knapper: **Lokal** og én knap for hver off-site-destination i området. Oplyste knapper får elementets sikkerhedskopier.

- Med **Lokal** oplyst skrives elementet til det repository, der vises under **Gemt på**, og kopieres til hver anden oplyst destination. Gør du en destination mørk, får den intet nyt fra dette element. Kun Lokal kopierer ingen steder hen, hvilket passer til data, der allerede har en anden kopi, for eksempel en share, der ligger på en NAS.
- Med **Lokal** mørk skrives elementet direkte til det direkte repository på den første oplyste destination og kopieres derfra til de andre oplyste. Første gang opretter en dialog det direkte repository.
- Én knap forbliver oplyst, fordi en sikkerhedskopiering har brug for et sted at havne. For at udelade et element fra sikkerhedskopier skal du bruge **Sikkerhedskopiér ikke**.

BombVault flytter aldrig sikkerhedskopier mellem repositories. Et element, der følger sit område, følger med, når områdets standard ændres: Dets næste sikkerhedskopi havner på det nye sted, og de sikkerhedskopier, det har, bliver, hvor de er. En destination valgt på selve elementet ligger fast fra dets første sikkerhedskopi. Kortet **Repository** på fanen Indstillinger nævner den. Kopierne kan ændres når som helst. En destination, der ikke længere får et element, beholder de kopier, den har, og trimmer dem til sin egen opbevaring ved områdets næste off-site-kørsel. Append-only-destinationer kan der ikke slettes fra.

Det sted, et element forlod, beholder sine sikkerhedskopier. På elementets fane **Sikkerhedskopier** og på **Gendannelse** står det som **Kun ældre sikkerhedskopier**, en gendannelse kan stadig læse derfra, og en beskæring dér lader de sikkerhedskopier være.

**Følg standarden** henter et element med et eget valg tilbage til sit område. Den spørger først, siger, hvor nye sikkerhedskopier så havner, og nævner, hvad hver destination holder op med at få, og hvad den næste kørsel ville uploade.

Under rækken siger kortet, hvor elementet havner, og hvad der faktisk er der: hvor mange steder der rummer det, hvornår hver destination sidst blev set, og om 3-2-1 er opfyldt. Et sted er serveren med de oprindelige data, hver off-site-destination og hvert repository markeret **Uden for lokationen**. BombVault tjekker kopier og steder. Den tjekker ikke "to medier"-delen af 3-2-1.

### Standarder pr. område {#placement-defaults}

Kortet **Destinationer pr. område** på siden Destinationer tegner hvert områdes standard som et 3-2-1-flow. Hver linje fra et område til en destination er en kontakt, **Lokal** inklusive, for hvert område, også flashdrevet, ZFS-datasæt og auto-sikkerhedskopien. Et klik på en linje eller en prik slår den til eller fra med det samme, og stiplede linjer er slået fra. Kun at slå et områdes **Lokal** fra spørger først, i den samme dialog som standardkortet, fordi området så sikkerhedskopierer direkte til en kopi. **Annuller** lader alt være, som det var. **Graf / Til/fra** viser det samme som rækker af knapper. En kontakt afgør, hvor nye sikkerhedskopier havner. Poster uden et eget valg og Compose-stakkenes projektmapper følger med det samme, så deres næste sikkerhedskopi havner på de nye steder. Sikkerhedskopier, der allerede er gemt, bliver, hvor de er, og kan stadig gendannes derfra. Flash, ZFS-datasæt og auto-sikkerhedskopien sikkerhedskopierer til deres lokale mappe og kopierer off-site fra den sikkerhedskopi. Deres linje til **Lokal** kan også slås fra. Deres sikkerhedskopier går så direkte til deres off-site-destinationer, og et ZFS-element med en replika og ingen destination overhovedet sender kun sin replika.

Med **Lokal** slået fra og kun off-site-destinationer slået til sikkerhedskopierer et område direkte til dem, og BombVault vælger selv repositoryet dér. Containere, VM'er og mappesæt går til det direkte repository på den første destination, der er slået til, beskrevet nedenfor, og kopieres derfra til de andre. Flashdrevet, ZFS-datasæt og auto-sikkerhedskopien skriver hver sikkerhedskopi til hver destination, der er slået til, og hver destination beholder dem efter sin egen opbevaring.

Et område med alle linjer slået fra siger **Ikke sikkerhedskopieret**. Dets poster får ingen sikkerhedskopier og tæller som ubeskyttede.

Poster, der har valgt egne destinationer på deres side, beholder dem. Flowet siger, hvor mange der er i et område, for eksempel **2 poster afviger**, og fører til dem: til postens fane Indstillinger, når der er én, til en liste med, hvor hver sikkerhedskopierer, når der er flere. **Følg standarden** dér henter en post tilbage.

En ny off-site-destination modtager hver post, der ikke er sat til Lokal. Vinduet, der tilføjer den, siger, hvor mange poster, og hvor det er kendt, hvor meget historik det er.

### Direkte repositories {#direct-repositories}

Et direkte repository er det sted, hvor poster sikkerhedskopierer direkte til en off-site-destination. BombVault lægger det ved siden af destinationens mappe med `-direct` føjet til navnet, eller, når destinationen ligger i roden af sin bucket, sin remote eller en rest-server-bruger, i en mappe inde i den opkaldt efter området, for eksempel `containers-direct`. I flowet på siden Destinationer oprettes det uden et spørgsmål. At slå Lokal fra for en enkelt post, så en destination uden et direkte repository bliver dens hjem, åbner en dialog med den foreslåede placering, for eksempel `s3:https://s3.eu-central-003.backblazeb2.com/bucket/containers-direct`, og en forbindelsestest, der ikke opretter noget. **Opret og brug** opretter repositoryet og peger posten på det. Et direkte repository har ingen egne indstillinger. Det følger sin destinations kontakt, login, lagerklasse, grænser, append-only-indstilling og opbevaring og ændrer sig med dem. Destinationens side lister det under **Sikkerhedskopierer direkte hertil** med hvert område, der sikkerhedskopierer direkte til det, og hvor mange poster der gør det. Et, som intet sikkerhedskopierer til mere, siger det dér og kan fjernes, og sikkerhedskopierne på dets sted bliver. Når en ny nøgle til destinationen ikke kan åbne det, beholder det direkte repository den nøgle, det har, og gemningen siger det. En post på et direkte repository kopieres derfra til de andre oplyste destinationer, aldrig til den destination, repositoryet hører til. Dets gendannelsespunkter bærer tagget `bv:direct`, og hvert andet opbevaringsgennemløb beholder dem, så et direkte repository, der har mistet sit link til sin destination, aldrig ældes efter de lokale regler. B2 nås gennem sit S3-endpoint, med nøgle-ID'et og applikationsnøglen skrevet som S3-nøglerne. En nøgle begrænset til destinationens egen mappe kan ikke nå mappen ved siden af, så begræns i stedet nøglen til mappen over.

### Uden for lokationen {#off-the-premises}

En destination eller et repository kan markeres **Uden for lokationen**. Fjerne starter markeret. Slå det fra for en rest-server i samme bygning. Mærket tæller kun steder og 3-2-1 på kortene. Det ændrer ingen kopi.

En ZFS-replika er en kopi uden for huset, men ikke en sikkerhedskopi med historik. Den beholder kun de snapshots, du angiver, og gamle versioner og enkelte filer kommer stadig fra sikkerhedskopierne. Den går fra én ZFS-server direkte til den anden, over SSH til en ZFS-server eller til modtagerendpointet på en parret instans. Parringsgruppen bærer kun anmodningen. Se [Replika](zfs-datasets.md#replica).

### Efter en genopbygning {#after-a-rebuild}

Kopivalg ligger i BombVaults egne indstillinger. Efter en genopbygning gennem **Opdag sikkerhedskopier** eller nødguiden uden gendannede indstillinger er de væk, og at kopiere alt ville sende de poster, du havde udeladt, til B2 igen. Off-site-kopier af hvert genopbygget område holder derfor pause. Oversigten siger det, og kortet **Destinationer pr. område** tilbyder **Bekræft standard** med en forhåndsvisning af det, den næste kørsel kopierer, og navnene i de sikkerhedskopier, der ikke har nogen post, som du kan udelade dér. Kun bekræftelsen afslutter pausen. At importere en indstillingsfil bringer regler og standarder tilbage, men afslutter den ikke.

## Beskyttelse mod sletning (append-only) {#delete-protection-append-only}

Slå beskyttelse mod sletning til for en off-site-destination, så ransomware eller en kompromitteret vært ikke kan slette eller omskrive dine sikkerhedskopier. Den anden side (en `restic/rest-server`, der kører i tilstanden `--append-only`) **håndhæver** den. BombVault **verificerer** den kun og viser aldrig grønt på en konfigurationspåstand alene. En mappe på denne server kan ikke afvise sletning, fordi den, der styrer BombVault, også styrer mappen, så lokale destinationer tilbyder ikke kontakten.

Kontakten hører til destinationen: en sektion, der kopierer dertil, tager den fra destinationen, så slå den til på destinationens side under **Destinationer**. Det planlagte **Tjek sletningsbeskyttelse** kører kun, så længe en sektion kopierer til en destination med kontakten slået til. Den anden side skal startes med `--append-only`. Med imaget `restic/rest-server`, på TrueNAS som på enhver anden Docker-vært, skal det i variablen `OPTIONS`.

Nogle steder kan slet ikke afvise en sletning, for eksempel et cloud-drev som OneDrive. Hvis du bevidst lægger kopier der, så slå **Accepter uden beskyttelse mod sletning** til på lagerplaceringens side. Oversigten viser så den manglende beskyttelse for et afsnit, der kun kopierer til accepterede placeringer, som en accepteret risiko, en grå ring i stedet for en gul prik, og afsnittets farve følger dets øvrige kontroller. Ellers ændres intet: sikkerhedskopierne der kan stadig slettes, BombVault rydder stadig op i dem efter placeringens regler, og kontakten forsvinder, så snart beskyttelse mod sletning er slået til. En kopi, der flytter et andet sted hen, mister accepten. Det samme gør det at slå beskyttelse mod sletning til.

Den hurtigste vej til en append-only-kopi er en parret BombVault med en [modtageserver](#receiving-server): den kører rest-serveren for dig, i append-only-tilstand.

!!! note "En vellykket sletning under `/locks/` er forventet"
    Append-only betyder ikke, at intet længere kan slettes. restic skal tage og frigive sine egne låse, så `/locks/` forbliver bevidst skrivbar og sletbar. Sikkerhedskopier og dataene bag dem, altså præcis det ransomware ville gå efter, kan ikke fjernes. Tester du selv modparten, er en sletning der lykkes under `/locks/` korrekt adfærd og ikke et hul i beskyttelsen.

!!! warning "Append-only-repositories trimmes aldrig fra denne boks"
    En append-only-destination trimmer aldrig gamle sikkerhedskopier herfra. Trimning er den anden sides opgave. Sæt en **vækstbudgetalarm** for den, så du advares, før repositoriets størrelse løber løbsk.

## Manipulationstest {#tamper-test}

BombVault beviser jævnligt append-only-garantien ved faktisk at forsøge en sletning i off-site-repositoriet, rettet mod et ikke-eksisterende objekt. **Test beskyttelsen mod sletning nu** på en destinations side kører den på forlangende, og **Tjek sletningsbeskyttelse** under **Indstillinger, Verificering, Automatiske tjek** kører den efter en tidsplan.

At slå **Tjek sletningsbeskyttelse** fra skjuler ikke et hul: en beskyttelse, der aldrig er testet, vises så som ikke bevist, fordi kontrollen er slået fra, og en placering uden beskyttelse mod sletning vises som uden beskyttelse under alle omstændigheder.

En kopi, der er slået fra, testes ikke, og destinationen siger det. Forbindelsestesten virker stadig, fordi den kun læser, så en kopi kan tjekkes, før den slås til.

- **Afvist** betyder beskyttet.
- **Accepteret** betyder ikke beskyttet.
- Et **inkonklusivt** resultat (server uopnåelig, autentificeringsfejl) vender aldrig den gemte dom.

En reel beskyttet-til-ubeskyttet-vending udløser én enkelt advarsel.

## DR-øvelser {#dr-drills}

BombVault tilbyder to niveauer af bevis for, at dine sikkerhedskopier faktisk kan gendannes, ikke bare er til stede.

- **Gendannelsesverificeringsøvelser (lokale).** BombVault kører jævnligt `restic check --read-data-subset` (afgrænset, aldrig en diskfyldende fuld gendannelse). Kadencen ligger under **Indstillinger, Verificering, Automatiske tjek**, og kortet **Tjek sikkerhedskopier** på samme felt viser resultatet.
- **Prøvegendannelse (off-site).** BombVault gendanner et rigtigt mål fra off-site-repositoriet til en aflukket sandkasse, verificerer det fil for fil og byte for byte og rydder så op. Det beviser, at du kan komme tilbage fra off-site, ikke bare at repositoriet svarer.

Kortet **Beskyttelsesstatus** på oversigten sammenfatter det pr. område i fire tjek: en aktuel sikkerhedskopi på denne server, en kopi off-site, sletningsbeskyttelse dér og en prøvegendannelse derfra. Et klik på et område viser detaljerne, og kortet bliver kun grønt på verificerede fakta. En placering, du har accepteret uden beskyttelse mod sletning, tæller der som en accepteret risiko, ikke som en advarsel.

## Parring af instanser {#pairing}

Flere BombVault-instanser arbejder sammen som én gruppe: de viser hinanden beskyttelsesstatus, overtager roller for hinanden og kan opbevare hinandens nøgle. En instans slutter sig til gruppen med tolv ord.

![Parringssiden med sine tre trinkort.](assets/screenshots/offsite/pairing-dark.png#only-dark)
![Parringssiden med sine tre trinkort.](assets/screenshots/offsite/pairing-light.png#only-light)

På den første instans åbner du **Indstillinger, Parring** og trykker på **Generer sætning**. Tre trinkort viser, hvordan det går. Tolv ord vises i et vindue med en knap **Kopier**. På hver anden instans åbner du det samme sted, trykker på **Indtast sætning** og indsætter eller skriver dem, eller trykker på **Indsæt** i det vindue. Et ord, der ikke står på listen, navngives med sin position, mens du skriver, og det sidste ord bærer en kontrolsum, så et forkert tastet eller ombyttet ord opdages, før noget parres. Generer sætningen på kun én instans: to instanser, der begge genererer én, danner to adskilte grupper. Hvis ingen dukker op i et minut, tilbyder siden to veje ud: vis ordene igen for at indtaste dem dér, eller indtast den anden instans' ord og slut dig til dens gruppe i ét trin. Parring virker uden login-adgangskode, men sæt en: uden den kan enhver, der kan åbne web-UI'en, læse ordene og gennem gruppen bede om roller, der udleverer restic-adgangskoden til instanserne i den. Parringskortet siger det, indtil der er angivet en adgangskode. Med en adgangskode spørger den efter den, når sætningen vises igen. **Forlad gruppen** tager en instans ud igen. Kortet **Instanser** på parringssiden oplister gruppens medlemmer. Billedet af, hvem der gør hvad for hvem, findes på siden Instanser.

Enhver, der kender ordene, kan komme med i gruppen, så behandl dem som en adgangskode.

**Hvordan medlemmerne når hinanden.** Hver instans lærer sin egen adresse på netværket fra din browser i det øjeblik, du logger ind, oplistet under **Kendte adresser** i kortet **Denne instans** på parringssiden sammen med dem, instansen selv fandt. Ret den dér, hvis en reverse proxy eller en usædvanlig port står foran. På samme netværk annoncerer medlemmerne den adresse med multicast og taler direkte, og hvor multicast ikke kan krydse et containernetværk, som Dockers standardbro, søger en instans i stedet sit eget undernet efter de andre med et signeret kald, kun et gruppemedlem kan besvare, så parringen stadig bliver færdig på sekunder uden relay. Hvis intet dukker op, tager **Kan du ikke finde den?** én adresse i hånden, til et andet undernet eller en ikke-standardport. Instanser på forskellige netværk går gennem et relay, valgt i en række af fire i kortet **Videresender** på samme side. Hver form har et billede:

- **Projektets videresender** (standard): `parleyport.halleluja.design`, den samme videresender som KnightLoader også bruger. Intet at sætte op.
- **Egen videresender**: containeren [**ParleyPort**](https://github.com/junkerderprovinz/parleyport) fra Unraid Community Apps (også under **Indstillinger, Apps**). Indtast dens adresse på hver instans, der skal bruge den.
- **BombVault-instans**: en af dine instanser, der allerede kan nås udefra, med **Fungere som videresender** slået til. Den instans svarer så på `/relay/connect` på sin egen adresse, bag den reverse proxy og det certifikat, den allerede har, og lukker kun din gruppe ind. Indtast dens adresse på hver instans, der skal bruge den.
- **Ingen videresender**: medlemmerne finder automatisk hinanden på samme netværk, og ingen andre steder.

**Hvad videresenderen ser.** En knap med samme navn ved siden af de fire former viser det for den form, du har valgt. Hvert kald mellem medlemmer er forseglet med AES-256-GCM under en nøgle udledt af de tolv ord, og den nøgle forlader aldrig dine instanser. Relayet lærer en hash, der grupperer forbindelserne, hvilken instans en besked er til, hvor stor den er, og hvornår den passerer. Et direkte kald på det lokale netværk er forseglet på samme måde og desuden signeret, så intet afhænger af det selvsignerede certifikat, en instans serverer. Kald, der udleverer noget (rollesvar, en restic-adgangskode, modtagerlogins, nøgledeponeringer), signeres også med en nøgle fra det medlem, der sender dem, fastlåst ved første syn. Se [Sikkerhedsmodel](configuration.md#security-model).

**Hvad der rejser over gruppen.** Beskyttelsesstatus på siden Instanser, en anmodning om at tjekke ét område nu, rolleanmodninger og svarene på dem, destinationstilbud og det, en modtager eller henter behøver, når dens rolle er tilladt: den anden instans' repositoriesteder og dens restic-adgangskode. Sikkerhedskopidata gør det aldrig. De går stadig direkte til restic-backends. Det gør APP_KEY heller ikke: restic-adgangskoden åbner den instans' repositories og intet andet, ikke dens gemte hemmeligheder, sessioner eller gendannelseskoder.

**Poster fra før parringen.** Instanser tilføjet med et flåde-token og modtagne repositories og hentekilder sat op med den anden instans' APP_KEY bliver efter opdateringen og er markeret **Par igen**. De bliver ved med at virke: ved første start erstatter BombVault hver gemt APP_KEY med den restic-adgangskode, der er udledt af den. Par begge instanser, redigér så posten og vælg dens instans. Indtil da står sådanne poster i kortet **Uden instans** på siden Instanser.

Det eneste sted, der stadig tager en APP_KEY i hånden, er [Fra en gammel sikkerhedskopimappe](#restore-from-another-bombvault-repo), til det tilfælde, at den anden instans er væk og ikke kan svare i en gruppe.

## Instanser og roller {#roles}

Siden **Instanser** (et indstillingsfelt, der kan fæstnes til sidepanelet) viser gruppen som et net af, hvem der gør hvad for hvem, et kort for denne instans og et kort for hvert andet medlem, Android-telefonerne og ZFS-serverne. **Parring** nederst til højre fører til parringssiden. Hvilke dele der er slået til, sættes under **Indstillinger, Instanser, Flere servere**: **Instanser**, **Modtager** og **Hentning**.

![Siden Instanser: gruppen som et net og et kort pr. instans med dets rolleflader, hvoraf én er Inaktiv.](assets/screenshots/offsite/instances-dark.png#only-dark)
![Siden Instanser: gruppen som et net og et kort pr. instans med dets rolleflader, hvoraf én er Inaktiv.](assets/screenshots/offsite/instances-light.png#only-light)

Hvert kort viser medlemmets beskyttelsesstatus og dets roller som felter. En rolle er noget, en instans gør for en anden:

- **Modtager** opbevarer denne servers off-site-kopier i en rest-server i append-only-tilstand. Denne server sender efter hver sikkerhedskopi og kan ikke slette noget dér, selv om nogen overtager den.
- **Henter** henter denne servers sikkerhedskopier efter sin egen tidsplan. Denne server behøver ikke være vågen til det.
- **ZFS-server** tager imod denne servers ZFS-replikaer. Hvert ZFS-element vælger den under **Replika** i sine indstillinger.
- **Nøgleholder** opbevarer en forseglet kopi af denne servers gendannelseskit. Se [Nøgleholder](#key-keeper).

At slå en rolle til sender en anmodning, og intet starter, før nogen på den anden instans trykker på **Tillad**. Anmodningen vises på den instans' kort som et mærke, og dens **Detaljer** oplister den med **Tillad** og **Afvis**. En tilladt rolle kan tilbagekaldes dér når som helst. En stiplet linje i nettet er en rolle, der stadig venter. En instans på BombVault 9.x kan ikke svare på en anmodning, så en rolle er aktiv dér, så snart du slår den til, og en anmodning fra en 9.x-instans venter på **Tillad** her.

**Detaljer** på et kort åbner resten: det medlems modtagne repositories og hentekilder, destinationstilbud, overvågning, **Fjern** og **Glem nøglen**.

Et medlem kan tilbyde sin egen off-site-lagring til et andet medlem over gruppen. Den anden administrator ser tilbuddet i medlemmets **Detaljer** og accepterer eller afviser det. Hvis tilbuddet accepteres, oprettes gemte adgangsoplysninger og en almindelig destination efter de samme tjek som en destination, du selv skriver ind. Kun forbindelsesoplysninger rejser på den måde, aldrig sikkerhedskopidata. **Tjek nu** beder et medlem om at verificere ét områdes repository. Intet på siden kan starte en sikkerhedskopiering, en gendannelse eller en sletning på en anden boks.

Instanser, der er lukket ned, og telefoner med appen lukket beholder deres kort som **Inaktiv**, i gråt, med hvornår de sidst blev set, og står dæmpet i nettet, indtil du fjerner dem med **Fjern**. Lige efter at denne server er startet, viser sådan et kort **Kontrollerer**, indtil gruppen er tilbage. **Fjern** virker på sådan et kort, uanset hvilke dele der er slået til, og afslutter også medlemmets åbne anmodninger, dets ubrugte ZFS-modtagepladser og dets ventende nøgleanmodninger. Kortet **Instanser** på parringssiden viser også disse medlemmer som **Inaktiv**.

## Modtager og henter {#receiver}

På den boks, der **modtager** append-only-kopier fra en anden BombVault, giver de modtagne repositories dig uafhængig, skrivebeskyttet overvågning på modtagerhardwaren, så en stille fejl i den anden ende ikke går ubemærket hen. Slå **Modtager** til under **Indstillinger, Instanser**, åbn så den sendende instans' **Detaljer** på siden Instanser og tilføj det modtagne repository (skrivebeskyttet, åbnet med den sendende instans' restic-adgangskode, som den får over [parringsgruppen](#pairing)). Du får:

- **En beholdning efter kilde**, så du kan se præcis, hvilke containere, VM'er og mappesæt der er landet.
- **Sidst modtaget** pr. kilde, så du ved, hvor frisk hver er.
- **Et uafhængigt `restic check`** kørt på den modtagende hardware, så integritet verificeres, hvor data faktisk sidder, ikke kun på afsenderen.
- **En dødmandsknap:** en advarsel, når en kilde holder op med at sende inden for et vindue, du sætter.
- **Integritetsadvarsler:** en advarsel, når et tjek på den modtagende side fejler.

Modtageren er strengt skrivebeskyttet. Den skriver aldrig til det modtagne repository, så den kan aldrig bryde den append-only-garanti, afsenderen støtter sig til.

En henter virker den anden vej: med **Hentning** slået til læser en instans et andet medlems sikkerhedskopier efter sin egen tidsplan, ind i et eget repository for hver kilde, så de aldrig tæller som denne boks' egne sikkerhedskopier. Dens kilder står også i det medlems detaljer: repositoryets sted, hvilken slags sikkerhedskopi det rummer, og hvor ofte der hentes. Restic-adgangskoden kommer over gruppen, aldrig APP_KEY, og den anden side har intet mere at sætte op og behøver ikke køre. Kilden bliver kun læst, og hver side beholder sit eget login. En `rclone:`-kilde afvises, fordi rclone ville nå den med denne instans' remotes.

### Modtageserver {#receiving-server}

Modtagerboksen kan også køre den rest-server, de andre kopierer til. Den sættes op første gang, du tillader en **Modtager**-anmodning, der beder om det: vinduet siger, at serveren har brug for en modtageserver, og beder om en mappe på en share, med **Ny mappe** til at oprette én, og en port (8000, medmindre en anden container bruger den). BombVault så:

1. afviser, hvis der allerede findes en container med navnet `rest-server`, eller en anden container holder porten;
2. henter `restic/rest-server` og starter den gennem Docker-socketten i append-only-tilstand med private repositories og en loginfil i mappen;
3. skriver dens Unraid-skabelon til flashdrevet, så containeren kan redigeres i fanen Docker, eller tilbyder skabelonen som download, når flashdrevet ikke kan nås;
4. kører manipulationstesten mod den og viser, om den afviser sletning.

Instanserne i din gruppe finder så serveren blandt udbyderne under **Fra din gruppe**, opkaldt efter modtagerboksen, og rollen **Modtager** tilbyder den som det sikre valg. Hver instans får et eget login, første gang den bruger serveren, og skriver kun i sin egen mappe dér. Med **Modtager** slået til under **Indstillinger, Instanser** åbner **Modtageserver** på denne instans' kort serveren igen. Dens vindue oplister loginsene under **Partnere med login**, og **Træk login tilbage** fjerner ét. Det, den instans allerede har kopieret, bliver i mappen. Opsætningen opretter også ét **Login uden for gruppen**, hvis adgangskode vinduet viser én gang.

En instans, der kun når modtagerboksen gennem relayet, kan ikke bruge serveren, fordi relayet ikke bærer sikkerhedskopier. Tilføj først modtagerboksens adresse på parringssiden. Når BombVault kører på en egen IP-adresse (for eksempel på br0), udfyld **Adresse for partnere**, fordi serveren lytter på værtens adresse.

## Eksempel: parring af to Unraid-bokse {#worked-example-two-unraid-boxes-end-to-end}

Dette er én komplet opsætning af delene ovenfor, fordi de er lettere at samle, når man har set dem samlet.

To bokse: **TOWER** kører containerne og sender sikkerhedskopier. **VAULT** modtager dem og håndhæver append-only. Indsæt dine egne navne, adresser og share-stier.

**1. Par de to bokse.** På TOWER åbner du **Indstillinger, Parring**, trykker på **Generer sætning** og indtaster de tolv ord på VAULT under **Indtast sætning**. Angiv først en login-adgangskode på begge.

**2. På TOWER, bed VAULT om at være modtager.** På siden Instanser slår du **Modtager** til på VAULTs kort, vælger områderne og **VAULTs modtageserver** som sted og sender anmodningen.

**3. På VAULT, tillad det.** VAULTs side Instanser viser anmodningen på TOWERs kort. Tryk på **Tillad**. Første gang spørger VAULT efter en mappe til sin modtageserver, for eksempel `user/appdata/rest-server`, starter rest-serveren i append-only-tilstand og kører tamper-testen mod den.

**4. Tilbage på TOWER.** Herfra kopierer TOWER til VAULT efter hver sikkerhedskopi, og den nye destination vises på TOWERs side Destinationer med sin beskyttelse mod sletning.

Hvad tamper-testen svarer:

| Resultat | Hvad der skete |
| --- | --- |
| **beskyttet** | VAULT afviste sletningen. Det er den eneste beståede tilstand. |
| **ikke beskyttet** | VAULT accepterede en sletning. `--append-only` mangler eller blev fjernet. |
| **ikke entydigt** | Hverken eller. Som regel er adressen ikke den, restic selv bruger, eller legitimationen er ændret. Intet registreres, og ingen advarsel udløses. |

**5. Følg med i, hvad der kommer, hvis du vil.** Slå **Modtager** til under **Indstillinger, Instanser** på VAULT, åbn TOWERs **Detaljer** og tilføj det modtagne repository. VAULT får TOWERs restic-adgangskode over gruppen. Ingen taster en nøgle.

**6. Gør det gensidigt.** Gentag trinnene i den anden retning: VAULT beder TOWER om at være dens modtager. Hver boks håndhæver så append-only for den anden, og ingen af dem kan slette den andens sikkerhedskopier.

En rest-server, du kører i hånden, virker også: tilføj den med **Tilføj destination** som en rest-server, med brugeren og adgangskoden fra dens `.htpasswd`, og slå sletningsbeskyttelse til. Behold `--append-only` i dens indstillinger. Uden det er den anden side en almindelig share igen.

!!! warning "En lokal sti er en sti inde i containeren"
    Hvor BombVault beder om en mappe på denne server, indtast den relativt til værtsmonteringen, for eksempel `user/appdata/rest-server/bombvault-containers`, ikke `/mnt/user/appdata/...`. BombVault kører i en container, hvor værtens `/mnt` er monteret et andet sted. En absolut værtssti findes ikke inde i den. Hvis du indsætter en, fortæller BombVault dig den relative sti, du skal bruge i stedet.

## Nødguide {#emergency}

**Nødguiden** henter en server tilbage, der er væk eller blev sat op igen. Den starter på siden **Gendannelse** under **Nødsituation** og fra **Gendan fra en sikkerhedskopi** på velkomstsiden for en ny installation. Den kører i fire trin: **Adgang**, **Indstillinger**, **Valg** og **Gendannelse**.

1. **Adgang.** Vis BombVault nøglen til de gamle sikkerhedskopier, på en af tre måder:
    - **Upload gendannelseskit**: filen, du downloadede under opsætningen. Den rummer nøglen og hver destination, og BombVault udfylder resten ud fra den. Et kit forseglet med age beder om den identitet, det blev forseglet til. Den bruges én gang og gemmes ikke.
    - **Via gruppen**: en instans i din gruppe opbevarer nøglen som deponering. Du skal bruge gruppens tolv ord, din frigivelseskode og en person ved den instans. Se [Nøgleholder](#key-keeper).
    - **Indtast nøglen**: den gamle servers `APP_KEY` og hvor dens sikkerhedskopier er.

    BombVault prøver så nøglen på hver destination og siger for hver, om den er læsbar, ikke fundet, låst med en anden nøgle, ikke til at nå eller stadig mangler sit login. Intet er gemt endnu, og nøglen bliver i hukommelsen i en begrænset tid. **Overtag nøglen** får denne server til at skrive sine sikkerhedskopier med den nøgle fra nu af. **Fortryd overtagelsen** går tilbage til dens egen nøgle, indtil den første sikkerhedskopi har kørt. En server, der allerede har sikkerhedskopier af sig selv, afviser overtagelsen og henviser til [Fra en gammel sikkerhedskopimappe](#restore-from-another-bombvault-repo) i stedet. **Gem adgangsoplysninger** gemmer de logins, kittet bærer til fjerne destinationer, og spørger, før det erstatter logins, denne server allerede har.
2. **Indstillinger.** **Hent indstillinger tilbage** gendanner BombVaults egne indstillinger fra den nyeste auto-sikkerhedskopi: destinationer, adgangsoplysninger, tidsplaner og regler. BombVault genstarter for at anvende dem, gennem Docker-socket'en, så den levende indstillingsdatabase aldrig overskrives under et åbent håndtag, og guiden fortsætter af sig selv bagefter. Denne servers login bliver: du bliver ved med at logge ind med den adgangskode, du bruger nu. Hemmeligheder, den gamle server havde gemt, som ikke kan overtages, oplistes, så du kan indtaste dem igen. Gendannelsen afslutter alle sessioner og fjerner adgangsnøglerne, så sæt dem op igen bagefter.
3. **Valg.** BombVault gennemgår dine sikkerhedskopier og oplister de containere, VM'er, mappesæt og ZFS-datasæt, den finder. Den nyeste sikkerhedskopi er forvalgt overalt. Det, denne server stadig har, starter slået fra, fordi en gendannelse ville erstatte det. Mappesæt ved ikke længere, hvor de kom fra efter en genopbygning, så de går i en mappe, du vælger.
4. **Gendannelse.** Alt valgt kommer tilbage på én gang, efterladt stoppet, hvis du selv vil starte hver container og VM, hvilket er sikrest efter et totalt tab. Download bagefter et nyt gendannelseskit: det gamle hører til den forrige installation.

![Nødguiden ved sit første trin med de tre veje ind.](assets/screenshots/offsite/emergency-dark.png#only-dark)
![Nødguiden ved sit første trin med de tre veje ind.](assets/screenshots/offsite/emergency-light.png#only-light)

### Eksempel: en ny server fra gendannelseskittet {#example-emergency}

Serveren TOWER døde sammen med sine diske. Den havde 23 containere og 2 VM'er og kopierede dem og sin auto-sikkerhedskopi til Backblaze B2. Du har `bombvault-recovery-kit.md` fra din adgangskodeadministrator og en ny boks med Unraid på. Dens gamle `APP_KEY` er væk, og du behøver den ikke.

1. **Installér BombVault** på den nye boks, og giv den en ny `APP_KEY`, som ved enhver ny installation. Sæt VM-linket op under **Indstillinger, Integrationer, Værts-SSH**, hvis du vil have VM'erne tilbage, fordi en VM-gendannelse har brug for det.
2. **Start guiden.** Vælg **Gendan fra en sikkerhedskopi** på **Velkommen til BombVault**. På siden Gendannelse vælger du **Start nødguiden** under **Nødsituation**.
3. **Adgang.** Vælg **Upload gendannelseskit**, og slip filen. BombVault prøver sin nøgle på hver destination og siger for eksempel **Gendannelseskit læst. Dets nøgle åbner 0 af 2 destinationer.** Den lokale mappe siger **ikke fundet**, da disken døde, og Backblaze B2 siger **adgangsoplysninger mangler**.
4. **Gem adgangsoplysninger.** Kittet bærer loginet til sine fjerne destinationer. Tryk på **Gem adgangsoplysninger**, så prøves destinationerne igen, og Backblaze B2 siger nu **læsbar**.
5. **Overtag nøglen.** Denne server skriver nu sine sikkerhedskopier med den gamle nøgle, så de nye sikkerhedskopier passer til de gamle. Den beholder sin egen `APP_KEY` i skabelonen. **Fortryd overtagelsen** virker, til den første sikkerhedskopiering har kørt.
6. **Indstillinger.** Vælg **Hent indstillinger tilbage**, som læser den nyeste auto-sikkerhedskopi fra B2. BombVault genstarter for at anvende den, og guiden fortsætter af sig selv. Dine destinationer, tidsplaner og regler er tilbage, og du bliver ved med at logge ind med denne servers adgangskode. Skriv de hemmeligheder, guiden oplister, ind igen, for eksempel notifikationsindstillinger.
7. **Valg.** Sæt **Hvor er sikkerhedskopierne?** til off-site-kopierne, fordi de lokale gik ned sammen med den gamle server. BombVault oplister de 23 containere og 2 VM'er med den nyeste sikkerhedskopi valgt på forhånd, og du kan vælge en anden til hver af dem.
8. **Gendannelse.** Slå **Lad den stå stoppet bagefter** til, hvilket er sikrest efter et totalt tab, og start. **Hvad der sker** siger, hvad der kommer tilbage. Resultatlinjen lyder for eksempel **25 gendannet, 0 mislykkedes. Start dem under Sikkerhedskopier, når det er nødvendigt.**
9. **Afslut.** Start dine databaser først, så de apps der bruger dem. Åbn **Indstillinger, Gendannelse**, og download et nyt gendannelseskit, fordi det gamle hører til den forrige installation. Se efter under **Kræver opmærksomhed** på oversigten, og kør **Sikkerhedskopiér alt nu**, når alt virker.

Mangler auto-sikkerhedskopien også, så vælg **Spring over: jeg har ingen auto-sikkerhedskopi**. Destinationerne kommer så kun fra kittet, du sætter tidsplaner og regler op igen, og off-site-kopier holder pause, til du bekræfter områdernes standarder, se [Efter en genopbygning](#after-a-rebuild).


!!! tip "Planlagt migrering versus katastrofe"
    Nødguiden gendanner BombVaults egne indstillinger fra en sikkerhedskopi. Til en *planlagt* flytning til en ny boks kan du i stedet føre din konfiguration direkte over med kortet **Eksportér / importér indstillinger** (en bærbar JSON-fil). Se [Konfiguration](configuration.md#portable-settings-export-and-import).

### Fra en gammel sikkerhedskopimappe {#restore-from-another-bombvault-repo}

**Fra en gammel sikkerhedskopimappe** under **Andre kilder** på siden Gendannelse åbner en *anden* BombVault-instans' repository (en share monteret under `/mnt` eller en fjernadresse) med den instans' gendannelseskit eller dens indtastede `APP_KEY`, i en engangssession, skrivebeskyttet. Gennemse de containere, VM'er, mappesæt og ZFS-datasæt, der er gemt dér, vælg en sikkerhedskopi og gendan den, og det gendannede objekt bliver en almindelig lokal container, VM eller et almindeligt mappesæt. Der skrives aldrig noget til det andet repository, og dine egne backupindstillinger forbliver urørte (sessionen lever i hukommelsen og udløber af sig selv). At flytte en container fra server A til server B betyder ikke at omdirigere dine indstillinger og vende dem tilbage bagefter. Dette vindue er et engangsvindue: det åbner en session, gendanner det, du vælger, og glemmer den anden instans. Hvis du i stedet vil have en stående ordning, hvor denne boks efter en tidsplan henter en anden instans' sikkerhedskopier ind i sit eget repository, er det rollen **Henter** på siden Instanser.

En container, hvis netværk ikke findes på denne server, for eksempel et Unraid-`br0`-netværk på en almindelig Docker-vært, viser en netværksvælger under sin række. BombVault opretter den på det netværk, du vælger, sammen med dens andre netværk. Den faste IP- og MAC-adresse hørte til det gamle netværk og falder væk, så det nye netværk tildeler dem.

## Nøgleholder {#key-keeper}

En nøgleholder er en instans i din gruppe, der opbevarer en forseglet kopi af denne servers gendannelseskit. Den kan ikke selv åbne kopien. Hvis denne server nogensinde går tabt, beder en ny installation, der slutter sig til din gruppe, holderen om kopien og åbner den med din frigivelseskode.

![Vinduet med frigivelseskoden: tyve tegn i fire grupper, vist én gang.](assets/screenshots/offsite/recovery-code-dark.png#only-dark)
![Vinduet med frigivelseskoden: tyve tegn i fire grupper, vist én gang.](assets/screenshots/offsite/recovery-code-light.png#only-light)

1. På siden Instanser slår du **Nøgleholder** til på kortet for den instans, der skal opbevare den, og vælger **Deponer**. At deponere kræver en login-adgangskode.
2. Denne server viser **Frigivelseskode** én gang: tyve bogstaver og cifre i fire grupper af fem. Skriv den ned og bekræft med **Jeg har skrevet koden ned**. Denne server gemmer ikke koden, kun det, den skal bruge til at forsegle en deponering til den.
3. Holderen har nu deponeringen, og denne server fornyer den, når kittet ændres. **Nøgledeponering** på denne instans' kort oplister **Denne servers deponering** hos hver holder og dens tilstand, og **Deponeringer, som denne server opbevarer** for de andre. **Lav en ny kode** erstatter koden. Hver holder skifter til den nye kode, når den får den nye deponering.

Den dag skal du bruge tre ting: din gruppes tolv ord, frigivelseskoden og adgang til holderens grænseflade, hvor nogen skal trykke på **Tillad**. I Nødguiden vælger du **Via gruppen**, vælger deponeringen, beder om den og taster frigivelseskoden. Koden bruges på den nye server og sendes ikke til holderen. Hos holderen vises anmodningen under **Nøgledeponering** med, hvem der spørger, om den instans nogensinde er set dér før, og hvor ofte den har spurgt. **Tillad** udleverer den forseglede kopi, og tilladelsen kræver en login-adgangskode. Den nye installation har 30 minutter til at hente den efter Tillad, en anmodning, ingen besvarer, udløber efter et døgn, og en instans, der spørger for ofte, må vente en time.

Uden koden kan ingen åbne kopien, heller ikke holderen. Hvis koden går tabt, lav en ny, mens denne server stadig kører. Hvis serveren allerede er væk, virker gendannelseskittet og den indtastede APP_KEY stadig. Holderen kan til enhver tid slette en deponering, og ejeren må så spørge igen.

## Gendannelseskit {#recovery-kit}

Dette er den brik, der gør katastrofegendannelse mulig, selv når der ikke er nogen kørende BombVault.

**Download gendannelseskit** i kortet **Til nødsituationer** under **Indstillinger, Gendannelse** giver dig **nøglen**, det **afledte restic-kodeord**, hver **destination** med repositoriet for hvert område, de logins, en ny installation har brug for for at nå dem, og de **præcise restic-kommandoer**, så du kan gendanne direkte med restic CLI på en hvilken som helst maskine. Det kræver en login-adgangskode. Kittet udskriver også nøglen med kryptering slået fra, fordi container- og VM-definitionerne altid er forseglet med den. Efter en overtagelse udskriver det den overtagne sikkerhedskopinøgle og denne servers APP_KEY, som en ny installation kun skal bruge for at hente indstillingerne tilbage. Et kort **Gem dit gendannelseskit** på oversigten minder dig om det, indtil du har gemt det.

!!! danger "Opbevar gendannelseskittet uden for serveren"
    Kittet indeholder den hemmelighed, der dekrypterer dine sikkerhedskopier. Opbevar det et sikkert sted adskilt fra serveren (en adgangskodemanager, en udskrift i et pengeskab). Mister du både BombVault og `APP_KEY` uden gendannelseskit og uden nøgledeponering, kan dine krypterede sikkerhedskopier ikke gendannes.

!!! warning "Den nyeste sikkerhedskopi er ikke altid den, der skal gendannes"
    Siden restic 0.17 viser `restic snapshots` hvert snapshots størrelse. Efter datatab kan den nyeste sikkerhedskopi være den tømte, så gendan ikke en sikkerhedskopi, der er langt mindre end dem før den. Efter ransomware kan den være den krypterede i den sædvanlige størrelse. Hvis BombVault stadig kører, så tjek først siden **Afvigelser**: den navngiver den sidste gode sikkerhedskopi, og gendannelsesvinduet forvælger den. En gendannelse behøver ingen af BombVaults afvigelsesdata, og opbevaringspausen beholder altid kun flere sikkerhedskopier.

### Forsegling af kittet {#sealing-the-kit}

Hvis du har slået **Krypter eksporter med age** til under **Indstillinger, System**, er kittet forseglet med det også og downloades som `bombvault-recovery-kit.md.age`. Det er ASCII-armeret frem for binært, så det er stadig klartekst: at indsætte det i en adgangskodemanager eller udskrive det virker præcis som før, indholdet er bare ulæseligt uden din nøgle.

!!! warning "Opbevar ikke age-nøglen i kittet"
    Du skal bruge din **private** age-nøgle for at åbne et forseglet kit. Opbevar den et sted, der ikke afhænger af selve kittet, ellers har du to ting at gendanne i stedet for én. Forsegling er umagen værd, når kittet ligger et sted, du ikke selv har fuld kontrol over (en delt adgangskodemanager, noter i skyen, en udskrift på et kontor). Et kit i dit eget pengeskab er allerede beskyttet af pengeskabet.

    Med kryptering slået til og ingen brugbar modtager sat op nægtes downloadet helt. BombVault falder aldrig tilbage til at udlevere hovednøglen i klartekst.

### Når sættet ikke er ved hånden {#if-you-do-not-have-the-kit-to-hand}

Adgangskoden er ikke gemt nogen steder, den **beregnes** ud fra sikkerhedskopiernes nøgle, så du selv kan gengive den med intet andet end nøglen og en shell:

```sh
printf 'bombvault:restic-repo'   | openssl dgst -sha256 -mac HMAC -macopt hexkey:$APP_KEY -r   | cut -d' ' -f1
```

Det er HMAC-SHA256 over den faste streng `bombvault:restic-repo`, med de rå bytes af den hexadecimale `APP_KEY` som nøgle, udskrevet som 64 små hexadecimale tegn. Den samme værdi står i kittet, oplistet som det afledte restic-kodeord. Dette er til den dag, kittet er et sted, hvor du ikke er. På en server, der overtog en andens sikkerhedskopier, bruges den overtagne sikkerhedskopinøgle, som er den servers gamle APP_KEY.

!!! warning "Brug den AFSENDENDE instans' nøgle ved et modtaget arkiv"
    Et repository, der kom hertil gennem en off-site-kopi, blev oprettet af maskinen, der sendte det, med **dens** nøgle. At udlede fra modtagerboksens nøgle giver en adgangskode, restic afviser, hvilket ser nøjagtig ud som et beskadiget repository og ikke er det. Det er den sædvanlige grund til, at `restic check` på et modtaget repo bliver ved med at bede om en adgangskode.

Fordi gendannelsesdefinitioner lever **inde** i hvert repo (`<repo>/def`, `<repo>/vm-def`), er en kopieret repo-mappe fuldt selvstændig, så kittet plus repoet er alt, hvad en bare-metal-gendannelse har brug for.

## Hent et databasedump tilbage {#database-dumps}

Et databasedump er et gendannelsespunkt for sig i containerrepositoriet, mærket `dbdump:<container>`, med den ene fil `/dbdump/<container>.sql`. BombVault oplister, downloader og importerer dem under containerens fane **Sikkerhedskopier**. Nedenfor er de samme trin med restic alene, til den dag BombVault ikke er der.

```sh
restic -r <repo> snapshots --tag dbdump:<container>
restic -r <repo> dump --tag dbdump:<container> latest /dbdump/<container>.sql > <container>.sql
```

Etiketterne `dbversion:` og `dbname:` på hvert dump fortæller, hvilken serverversion det kommer fra, og hvilke databaser det rummer. En komplet fil slutter med `-- PostgreSQL database cluster dump complete` eller `-- Dump completed`.

Importér det i en container med samme eller en nyere version (PostgreSQL) eller samme hovedversion (MySQL og MariaDB), startet én gang med en tom datamappe, så den initialiserer sig. Værten har ikke brug for en databaseklient, containeren har en:

```sh
docker exec -i <container> sh -c 'exec psql -X -U "${POSTGRES_USER:-postgres}" -d postgres' < <container>.sql
docker exec -i <container> sh -c 'exec mariadb -uroot -p"$MARIADB_ROOT_PASSWORD"' < <container>.sql
docker exec -i <container> sh -c 'exec mysql -uroot -p"$MYSQL_ROOT_PASSWORD"' < <container>.sql
```

For en enkelt database ud af et fuldt dump tager MySQL og MariaDB `--one-database <name>` på klientkommandoen. Et PostgreSQL-dump har én sektion per database, hver indledt med en linje `\connect <name>`: kopiér den sektion over i sin egen fil, og importér den med `-d <name>`, efter at databasen er oprettet.

!!! warning "Et dump taget som root bringer serverens brugere med"
    Et fuldt MySQL- eller MariaDB-dump taget som root indeholder systemdatabasen `mysql`, så en import erstatter den nye servers konti, root-adgangskoden inklusive, med dem fra dumpet. På PostgreSQL er `role ... already exists` for den bruger, containeren selv oprettede, forventet og harmløst.
