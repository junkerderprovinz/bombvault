# Gendannelse

Hver gendannelse starter på siden **Gendannelse**: Vælg et område, søg efter det, du vil have tilbage, og vælg en sikkerhedskopi. Ét vindue gendanner alle slags poster og spørger trin for trin. Som standard ændres intet på originalen, fordi sikkerhedskopien kommer tilbage ved siden af den.

![Siden Gendannelse med områderne som fliser, søgningen og kortet Stakke.](assets/screenshots/restore/page-dark.png#only-dark)
![Siden Gendannelse med områderne som fliser, søgningen og kortet Stakke.](assets/screenshots/restore/page-light.png#only-light)

## Siden Gendannelse {#page}

- **Områder** viser områderne som fliser, hver med hvor mange poster der har gendannelsespunkter.
- **Hvad vil du hente tilbage?** søger i posterne i det valgte område. Hver række viser sit nyeste gendannelsespunkt, og **Åbn element** fører til postens side.
- **Find en fil** leder efter en fil eller mappe ud fra navnet i hver sikkerhedskopi, se [Søg efter en fil](#find-a-file).
- **Stakke** viser Compose-projekterne med mindst to sikkerhedskopierede containere, for eksempel `immich` med sin database og Redis. Søgningen finder en stak på dens navn eller på en af dens containere.
- **Kører nu** viser en gendannelse, mens den kører, også efter at du har lukket dens vindue.
- **Nødsituation** starter nødguiden for en server, der er væk eller blev sat op igen, se [Nødguide](offsite-recovery.md#emergency).
- **Andre kilder** rummer **Fra en gammel sikkerhedskopimappe**, til en anden BombVaults sikkerhedskopier, og **Importér fra Appdata.Backup-pluginnet**.

![Siden Gendannelse på en telefon.](assets/screenshots/restore/page-phone-dark.png#only-dark)
![Siden Gendannelse på en telefon.](assets/screenshots/restore/page-phone-light.png#only-light)

## Gendannelsesvinduet {#window}

Et klik på en post åbner **Gendan** med dens navn. Vinduet spørger trin for trin, og hvert trin kan ændres, før du starter:

1. **Sikkerhedskopi**: Det anbefalede gendannelsespunkt er valgt på forhånd. Det er det nyeste, eller efter en åben afvigelse det sidste før den. Den sikkerhedskopi, der blev taget før en tidligere gendannelse, rummer den tilstand, den gendannelse erstattede, så den springes over, medmindre der ikke er nogen anden. Ældre ligger på listen, og **Sammenlign** viser, hvad der adskiller to af dem.
2. **Kilde**: Hvor filerne kommer fra, når et gendannelsespunkt ligger flere steder. **Lokal** er hurtigst.
3. **Hvad**: **Alle** eller **Enkelte filer**, hvor du sætter flueben ved de filer og mapper, du vil have.
4. **Hvorhen**: Målet. Valgene afhænger af postens slags:

    | Mål | Hvad det gør |
    |---|---|
    | **Ved siden af originalen** | Standarden. En container kommer tilbage som en anden container ved navn `<name>-restored` med egne porte, en VMs diske og NVRAM havner i en mappe ved navn `<folder>-restored`, og et mappesæt kommer tilbage som `<path>-restored`. Intet ændres på originalen. |
    | **Erstat originalen** | Overskriver det, der er der nu. Med **Tag først en sikkerhedskopi af den nuværende tilstand**, som er slået til som standard, tager BombVault først en sikkerhedskopi af dagens tilstand, så du kan gå tilbage. Mislykkes den sikkerhedskopi, starter gendannelsen ikke. Det virker også for en container eller VM, der er væk fra serveren: de mapper, gendannelsen skriver over, sikkerhedskopieres for sig. |
    | **Til en mappe** | Kun filerne, til at kigge i. |
    | **Download som ZIP** | Flashdrevet, til en ny pind. BombVault skriver aldrig ind i det kørende flashdrev. |
    | **Til et nyt datasæt** og **Gendan ind i datasættet** | ZFS-datasæt, se [ZFS-datasæt](zfs-datasets.md#restore). |

5. **Hvad der sker** siger med ord, hvad gendannelsen vil gøre, og en scene viser det mål, du har valgt.

Før **Gendan** låses op, tjekker vinduet, at repositoryet svarer, at nøglen åbner det, at gendannelsespunktet findes, og at målet har plads. Så længe et tjek fejler, forbliver knappen låst, og (i) i den siger hvilket.

Vinduet viser også, hvad gendannelsen ændrer i forhold til det, der er der nu: nye, erstattede og uændrede filer, med listen på forlangende, og de filer på målet, som ikke er i sikkerhedskopien og bliver liggende. restic regner det ud som en tørkørsel ud fra størrelse og ændringstidspunkt uden at læse filerne, og et meget stort træ stopper efter 30 sekunder og siger det. For containere og VM'er sammenligner det de indstillinger, gendannelsen genskaber, med de kørende: image og tag, porte, variabelnavne og volumener, eller hukommelse, vCPU'er, diske og netværk.

Nogle ting kan stoppe en gendannelse, før den ændrer noget. En containers statiske IP og udgivne porte skal være frie, og dens netværk skal findes. På Unraid afviser enhver gendannelse på stedet en mappe, der ikke ligger på en monteret pool eller share, fordi filerne ville havne i hukommelsen og være væk efter en genstart. En gendannelse på stedet nævner hver anden container, hvis bind mount rækker ind i en mappe, den skriver til, for eksempel "denne sti bruges også af nextcloud-db", men det er kun en advarsel.

Nogle få valg dukker op, hvor de passer: **Lad den stå stoppet bagefter**, for at se på en container eller VM, før den kører igen, og **Gendan uden GPU og runtime** på en vært, der mangler den GPU eller runtime, sikkerhedskopien blev taget med.

Gendannelsen kører i baggrunden. Du kan lukke vinduet, og fremdriften vises også under **Kører nu**. En lang gendannelse viser en procent og kan annulleres, og en annulleret gendannelse registreres som annulleret, ikke som mislykket. En gendannelse, der annulleres, mens containeren er ved at stoppe, eller en, der ikke kan fjerne containeren, har ikke gendannet noget endnu, så containeren starter igen. En container, der tager sine volumener fra en anden med `--volumes-from`, beholder dem, og gendannelsen begynder ikke, så længe den anden container ikke findes.

![Gendannelsesvinduet ved trinnet Hvad med enkelte filer afkrydset.](assets/screenshots/restore/window-what-dark.png#only-dark)
![Gendannelsesvinduet ved trinnet Hvad med enkelte filer afkrydset.](assets/screenshots/restore/window-what-light.png#only-light)

### Hvor en gendannelse til en mappe havner {#folders}

**Mapper til gendannelser** under **Indstillinger, Gendannelse** sætter **Standardmappe til gendannelse**, om du vil med en **Undermappe pr. område**, og en **Undtagelse** for ét område. Valgte filer havner direkte i målmappen: Én fil kommer tilbage som netop den fil, en mappe som mappen med sit indhold, og valg fra forskellige mapper beholder kun deres sti under den mappe, de deler. BombVault afviser et mål, der ikke ligger på monteret lager, fordi sådan en mappe på Unraid ligger i hukommelsen og er væk efter en genstart, og den gendanner aldrig ind i sin egen datamappe.

## Eksempel: hent én fil tilbage {#example-single-file}

Nogen har redigeret Nextclouds `config.php` i nat, og Nextcloud starter ikke længere. Natsikkerhedskopien fra kl. 03:00 har stadig den gode fil. Du vil have den ene fil, ikke hele containeren.

1. Åbn **Gendannelse**, vælg fligen **Containers**, og skriv `nextcloud` i **Hvad vil du hente tilbage?**.
2. Klik på `nextcloud`. Vinduet åbner med sikkerhedskopien fra kl. 03:00 valgt på forhånd som den anbefalede.
3. Lad **Kilde** stå på **Lokal**.
4. Vælg **Enkelte filer** under **Hvad**. Åbn mapperne ned til `config`, og sæt flueben ved `config.php`.
5. Vælg **Til en mappe** under **Hvorhen**, og vælg en mappe, for eksempel `user/restore`, som er `/mnt/user/restore` på værten.
6. Læs **Hvad der sker**: Filen havner i målmappen, og intet ændres på containeren. Tryk på **Gendan**.

Vinduet slutter med **Færdig: filerne ligger i /mnt/user/restore**. Du finder filen som `/mnt/user/restore/config.php`, uden mapperne over den. Sammenlign den med den ødelagte, kopiér den tilbage i appdata-mappen, og genstart Nextcloud.

Vil du i stedet lægge filen direkte tilbage, så vælg **Erstat originalen** i trin 5. Lad **Tag først en sikkerhedskopi af den nuværende tilstand** stå slået til, så BombVault sikkerhedskopierer den ødelagte tilstand, før den skriver, skriv `nextcloud` for at bekræfte, og tryk på **Gendan**. Nextcloud stoppes, mens den valgte fil skrives tilbage på sin plads, og startes igen bagefter, og kun de filer, du satte flueben ved, skrives. En gendannelse, der mislykkes undervejs, lader Nextcloud stå stoppet, da den fil, den læser, kan være halvt skrevet, og kørslen siger det.

![Gendannelsesvinduet ved Hvad der sker med Erstat originalen valgt.](assets/screenshots/restore/window-replace-dark.png#only-dark)
![Gendannelsesvinduet ved Hvad der sker med Erstat originalen valgt.](assets/screenshots/restore/window-replace-light.png#only-light)

## Søg efter en fil {#find-a-file}

Når du kender navnet på en fil, men ikke ved, hvilken post eller sikkerhedskopi der stadig har den, så skriv navnet i **Find en fil**, og tryk på **Søg**. Almindelig tekst matcher hvor som helst i navnet og ser bort fra store og små bogstaver, så `invoice` finder `Invoice-2024.pdf`. Med `*` eller `?` skal mønstret matche hele navnet, så `*.pdf` finder hver PDF. Skriv kun navnet, uden mappe.

Søgningen læser hver sikkerhedskopi af containerne, mappesættene, ZFS-datasættene og flashdrevet. Hvert fund viser posten, stien og hver sikkerhedskopi, der indeholder filen, nyeste først, med dato, størrelse og det sted, den blev læst fra. En sikkerhedskopi, hvor filen har samme størrelse og ændringstidspunkt som i den forrige, siger **uændret**, så du kan se, hvor filen sidst ændrede sig. Et klik på en sikkerhedskopi åbner gendannelsesvinduet på den, med **Enkelte filer** valgt og flueben ved filen. En fil fra flashdrevet kommer tilbage som en del af flashdrevets ZIP, fordi flashdrevet kun gendannes som helhed.

Søgningen tager et stykke tid, fordi restic går gennem hver sikkerhedskopi. **Stop** afslutter den, og det gør det også at forlade siden. Under feltet står, hvilke steder den har læst:

- Steder på denne server og på dit netværk gennemsøges altid.
- En sikkerhedskopi, der ligger flere steder, læses kun én gang, på det første. Et sted, hvis sikkerhedskopier alle blev læst andetsteds, siger **allerede gennemsøgt et andet sted**.
- Eksterne steder som S3, B2 eller SFTP springes over, medmindre du slår **Søg også på eksterne steder** til. Så læses der kun de sikkerhedskopier, som intet sted her har, typisk de ældre, som en længere opbevaring beholder eksternt. Den første søgning på sådan et sted kan tage lang tid, og nogle udbydere tager betaling for de data, den henter.
- Et sted, der ikke kunne åbnes, siger **kunne ikke læses**, og (i) ved siden af fortæller hvorfor. De andre steder gennemsøges alligevel.

Virtuelle maskiner gemmes som hele diske, så filerne i dem gennemsøges ikke. Auto-sikkerhedskopien og sikkerhedskopierne fra en anden BombVault under **Andre kilder** gennemsøges heller ikke. En søgning viser de første 200 filer, så et kort navn som `log` kan have brug for flere bogstaver.

## Hvad der kommer tilbage, efter slags {#kinds}

**Containere** genskabes ud fra deres gemte definition gennem Docker-API'et med deres entrypoint, værtsnavn, stopindstillinger, volumener og mounts, ressourcegrænser, log-driver, DNS-indstillinger og ældre links. En container kommer tilbage på det image, den kørte på, da sikkerhedskopien blev taget, ikke på et nyere, der blev hentet senere, og et image, der mangler, hentes først, med adgangsoplysningerne fra **Container-registre** under **Indstillinger, Gendannelse**. Dens GPU eller runtime (`--gpus`, `--runtime=nvidia`) kommer også tilbage. På en vært uden den driver eller runtime siger gendannelsen det og tilbyder **Gendan uden GPU og runtime**. Et link til en container, der mangler, eller som er stoppet, mens den gendannede starter, udelades, og kørselshistorikken siger det. En gendannelse, der mislykkes, efter at containeren blev fjernet, sætter den tilbage, stoppet.

En sikkerhedskopi kan rumme mapper, som containeren ikke længere har valgt, for eksempel en ældre, taget før en mappe forsvandt. En mappe, der er væk eller tom, kommer tilbage. En, der rummer filer nu, bliver, som den er, fordi du måske har fjernet fluebenet med vilje, og tjekket før gendannelsen og kørselshistorikken nævner den. Hent den tilbage med **Enkelte filer** eller **Til en mappe**.

**VM'er** får deres XML defineret igen over SSH og deres diske og UEFI-NVRAM sat til igen, selv efter at VM'en blev slettet. Se [VM-sikkerhedskopier](vm-backup-ssh-setup.md) for VM'er med Unraid-snapshots og diske på zvols.

**Mappesæt** kommer tilbage med deres ejerskab, tilladelser og tidsstempler, som restic gemmer dem. Hvor målet kun afviser ejerskabet, som `/mnt/user` gør, slutter kørslen med en note, der siger det. BombVault sammenligner så filerne med sikkerhedskopien, og en fil, der ikke kom frem, får gendannelsen til at mislykkes.

**Flashdrevet** rører aldrig den kørende `/boot`. Gendannelsen er en download ved navn `flash-<id>.zip`, klar til at lægge i Unraid USB Creator. ZIP'en rummer `APP_KEY`, så download af den kræver en login-adgangskode og en login på instansen. Vinduet viser også pluginnene i hver flash-sikkerhedskopi med version og størrelse og kan lægge et enkelt tilbage i det kørende flashdrev: dets `.plg`-fil, dets mappe under `config/plugins` og de pakkefiler, sikkerhedskopien rummer. Unraid installerer det ved næste opstart, eller med det samme under **Plugins, Install Plugin**.

**ZFS-datasæt** har egne mål og beholder deres ZFS-egenskaber, se [Gendannelse](zfs-datasets.md#restore).

**Auto-sikkerhedskopien** kommer kun tilbage gennem nødguiden, som genstarter BombVault, så den kørende database aldrig overskrives under et åbent håndtag.

**Databasedumps** indlæses aldrig tilbage af sig selv. En række under **Databasedumps** kan downloades, gemmes i en mappe eller importeres i en frisk startet database. Se [Databasedumps](backups.md#database-dumps).

## Gendan flere poster på én gang {#several}

På siden Gendannelse vælger du poster på samme måde som rækker på listen Sikkerhedskopier: Ctrl+klik, Shift+klik, Ctrl+A, et langt tryk eller **Vælg**. **Gendan** åbner så ét vindue til dem alle.

- **Hvilken sikkerhedskopi**: **Anbefalet** tager for hver post den sikkerhedskopi, dens eget vindue ville anbefale. **Ét tidspunkt til alle** tager for hver post dens nyeste sikkerhedskopi fra et **Tidspunkt**, du vælger, eller tidligere, for eksempel aftenen før noget gik galt.
- **Hvorhen**: **Ved siden af originalen** eller **Erstat originalen**, med **Tag først en sikkerhedskopi af den nuværende tilstand**.

Hver post tjekkes, som en enkelt gendannelse tjekkes, og så kører de efter hinanden. Hver række viser, hvordan dens egen gik: **Venter**, **Kører**, **Gendannet**, **Mislykkedes** eller **Holdt udenfor**. **Stop efter dette** lader den kørende gendannelse blive færdig og starter ikke flere. Flashdrevet kommer som en ZIP til download, og ZFS-datasæt og auto-sikkerhedskopien gendannes én ad gangen.

![Gendan flere poster: én række pr. post med dens fremdrift.](assets/screenshots/restore/several-dark.png#only-dark)
![Gendan flere poster: én række pr. post med dens fremdrift.](assets/screenshots/restore/several-light.png#only-light)

## Gendan en stak {#stacks}

Et klik på en stak på kortet **Stakke** åbner **Gendan stack**. Den gendanner hvert medlem fra dets seneste sikkerhedskopi på samme tid, og med **Start i afhængighedsrækkefølge efter gendannelse** starter den dem i deres `depends_on`-rækkefølge: databaser og caches først, så appen. Mangler et medlems projektmappe på det sted, du valgte, tilbyder vinduet tilstanden fra Lokal i stedet.

BombVault tjekker først medlemmerne og projektmappen, og en gendannelse, der ikke kunne lykkes, ændrer intet. Ellers stopper den de kørende medlemmer, apps før deres databaser, skriver så projektmappen og gendanner derefter medlemmerne. Kommer projektmappen ikke tilbage, gendannes medlemmerne alligevel, men bliver stående stoppet, fordi mappen kan være halvt skrevet, og det er dér, et medlems bind mounts som regel ligger. Hvert medlems kørsel siger det.

En database, der kun har databasedumps og ingen sikkerhedskopi af filer, og hvis container og navngivne volumen er væk, kommer tilbage uden sine data. Tjekket siger det før gendannelsen. Den bliver stående stoppet, og det samme gør de containere, der afhænger af den, så ingen app kører sin opsætning på en tom database. Start den, importér et dump, og start de andre. Se [Databasedumps](backups.md#database-dumps).

## Andre kilder {#other-sources}

- **Fra en gammel sikkerhedskopimappe** åbner repositoryet fra en anden BombVault, som ikke kører mere, på en disk eller på en off-site-destination, med dens gendannelseskit eller dens indtastede `APP_KEY`. Sessionen er skrivebeskyttet og lever kun i hukommelsen. Se [Fra en gammel sikkerhedskopimappe](offsite-recovery.md#restore-from-another-bombvault-repo).
- **Importér fra Appdata.Backup-pluginnet** læser pluginnets sikkerhedskopimappe. Hvert containerarkiv bliver et gendannelsespunkt for sin container, dateret til det tidspunkt, pluginnet lavede det. Arkiver, der er importeret før, springes over, og selve arkiverne bliver kun læst. Containeren skal først have én sikkerhedskopi i BombVault, så gendannelsen har dens definition. Opbevaringen lader importerede gendannelsespunkter være, så slet dem selv, når du ikke har brug for dem mere.

## Efter en gendannelse {#after}

En kopi, der er gendannet ved siden af originalen, står under **Sikkerhedskopier** som udeladt, så den ikke tæller som en åben opgave. Føj den til tidsplanen, hvis du vil beholde den. En container eller VM, der kørte, da den blev sikkerhedskopieret, kommer tilbage kørende, en der var stoppet, forbliver stoppet. På Unraid dukker en gendannet container op igen på fanen Docker som en almindelig, redigerbar app, og en VM i VM Manager med sine diske og NVRAM sat til.
