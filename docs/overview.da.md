# Oversigt

Oversigten er den første side i sidemenuen og den, du kan kigge på hver morgen. Den svarer på to spørgsmål med et enkelt blik: Er alt sikkerhedskopieret, og er der noget, du skal tage dig af? Den starter med en statusdel og fire kort, og flere kort venter under **Tilpas**.

![Oversigten efter nogle dage: statusdelen, Kræver opmærksomhed med én advarsel, beskyttelsen af hvert område, destinationerne og historikken.](assets/screenshots/overview/home-dark.png#only-dark)
![Oversigten efter nogle dage: statusdelen, Kræver opmærksomhed med én advarsel, beskyttelsen af hvert område, destinationerne og historikken.](assets/screenshots/overview/home-light.png#only-light)

## Statusdelen {#status-head}

Statusdelen sidder øverst på siden og bliver altid der. Ringen viser, hvor meget der er beskyttet, for eksempel **41 af 43 beskyttet**, og linjen ved siden af siger, hvordan det står til:

| Statusdelen siger | Hvad det betyder |
|---|---|
| **Alt er sikkerhedskopieret** | Hver post i hvert aktiveret område har en aktuel sikkerhedskopi. |
| **Sikkerhedskopieret, med bemærkninger** | Alt er sikkerhedskopieret, og nogle kørsler har efterladt en bemærkning, for eksempel filer, de ikke kunne læse. |
| **Ikke alt er beskyttet endnu** | Nogle poster har ingen aktuel sikkerhedskopi, for eksempel fordi de venter på den første. |
| **3 ting kræver opmærksomhed** | Noget er fejlet eller venter på en beslutning. Kortet **Kræver opmærksomhed** viser det. |
| **Tager backup nu** | En sikkerhedskopiering kører. Ringen bliver til en fremdriftsring med stationerne **Stop**, **Læs** og **Start**, og en fuld sikkerhedskopiering tæller sine poster undervejs, for eksempel **Tager backup nu: 12 af 43**. **Vis kørsel** åbner kørslen. |
| **Off-site-kopi kører** | En kopiering til en destination kører. Der vises ingen procent, fordi `restic copy` ikke melder nogen. |

Under linjen står det, hvornår den sidste kørsel var, og hvornår den næste planlagte starter.

## Kræver opmærksomhed {#needs-you}

Dette kort viser, hvad du bør se på, nyeste først, og skriver **Intet at gøre.**, når listen er tom. Hver række har sin egen vej ud:

- **En mislykket sikkerhedskopiering** nævner posten og årsagen. **Vis fejl** viser restics eget output, og **Prøv igen** starter sikkerhedskopieringen en gang til.
- **Off-site-kopi mislykkedes** nævner området og siden hvornår. **Replikér nu** prøver igen.
- **Afvigelse ved plex** fører til fundet med **Tjek**. Så længe et kritisk fund er åbent, beholder opbevaringen den posts gamle sikkerhedskopier.
- **Nye** poster, som BombVault har fundet, og som endnu ikke bliver sikkerhedskopieret, for eksempel en container, du lige har installeret, tilbyder **Begynd at sikkerhedskopiere** eller **Sikkerhedskopiér ikke**.
- **En sikkerhedskopiering, der gav op**, fordi dens app var optaget i hele ventetiden, se [Når appen er optaget](backups.md#app-busy).
- **En mislykket ZFS-replika** med en forklaring i almindeligt sprog, for eksempel at serveren ikke har nogen zfs-kommando, og med serverens output under **Vis**.
- **Sikkerhedskopieringen står stille: Nøglen kan ikke længere åbnes** vises, når denne server har overtaget en anden servers nøgle, og `APP_KEY` er blevet ændret bagefter. Se [Fejlfinding](troubleshooting.md#backups-stopped-and-say-the-key-can-no-longer-be-opened).
- **Gem dit gendannelseskit** bliver stående, indtil du har hentet kittet og trykket på **Jeg har gemt det sikkert**.

## Beskyttelsesstatus {#protection-status}

Hvert aktiveret område har en række med fire kontroller:

| Kontrol | Den bliver grøn, når | Andre ord, du kan se |
|---|---|---|
| **Lokal** | den sidste sikkerhedskopi er aktuel | ingen sikkerhedskopi endnu, ingen tidsplan, sidste kørsel mislykkedes, sikkerhedskopi overskredet, ikke alle poster er beskyttet, sikkerhedskopi snart forfalden |
| **Off-site** | den sidste kopi uden for huset er aktuel | ingen off-site-kopi, kopi kører, kopi overskredet, kopi sat på pause, ingen kopi endnu, replika mislykkedes |
| **Beskyttelse mod sletning** | en manipulationstest har vist, at den fjerne side afviser sletninger | ingen, ingen og risiko accepteret, ikke bevist endnu, test mislykkedes, ikke bevist, fordi dens kontrol er slået fra |
| **Testgendannelse** | en gendannelse fra off-site-kopien er bestået | aldrig kørt, kører, mislykkedes, overskredet, kun manuelt, ikke mulig for dette område |

Et område er kun grønt på bekræftede kendsgerninger. En kopi på denne server tæller ikke som off-site, og en destination, du har markeret **Accepter uden beskyttelse mod sletning**, viser en grå ring i stedet for en gul prik, så områdets farve følger dets øvrige kontroller.

Et planlagt område viser **Endnu ingen sikkerhedskopi** og forbliver neutralt, indtil tidspunktet for dets første kørsel er nået. Derefter står der **Forfalden**, og det advarer én gang. Et område uden noget at sikkerhedskopiere, fordi ingen tidsplan når nogen af dets poster, viser **Ikke planlagt** og er aldrig forfaldent.

Et klik på et område åbner dets vindue med hver kontrol udskrevet, de poster, den tæller, og **Start testgendannelse** eller **Kør testgendannelse igen**. Det viser også **Beskæringsregel angivet** eller **Ingen beskæringsregel**, og behold-reglen for et områdes egen mappe tæller lige så vel som dets off-site-regler.

![Beskyttelsesvinduet for området Containers med dets fire kontroller udskrevet.](assets/screenshots/overview/protection-window-dark.png#only-dark)
![Beskyttelsesvinduet for området Containers med dets fire kontroller udskrevet.](assets/screenshots/overview/protection-window-light.png#only-light)

## Destinationer {#storage-locations}

Hver destination står her som et kar med sit fyldniveau, hvor hurtigt det vokser, for eksempel **Vokser 4.2 GB/uge**, og hvornår det ville være fuldt i det tempo, for eksempel **fuld om omkring 31 uger**. S3, B2 og rest-servere melder ikke, hvor meget plads der er tilbage, så de viser **Ledig plads ukendt** og kun deres vækst. Et klik fører til destinationens side. Se [Destinationer](offsite-recovery.md#destinations).

## Historik {#history}

**Historik** har to faner. **Kørsler** viser hver kørsel med resultat, varighed og årsag, og et klik åbner den. **Log** viser hver linje med sit klokkeslæt, som BombVault skriver den, og kan begrænses til én dag. En linje bliver orange, når en sikkerhedskopiering gik igennem, men udeladte noget, og linjen nævner, hvad den udeladte.

## Flere kort under Tilpas {#customize}

**Tilpas** øverst på siden lader dig arrangere kortene. Træk et kort i dets bjælke for at flytte det, tilpas størrelsen ved prikken i dets nederste hjørne, eller skjul det. Skjulte kort venter under **Skjulte kort**, og **Vis** henter et tilbage. Mens du arrangerer, vugger kortene lidt, og **Færdig** afslutter det. Opstillingen gemmes i denne browser.

Disse kort starter skjult:

- **Afvigelser**: de åbne fund, se [Afvigelser](anomalies.md).
- **Ikke sikkerhedskopieret**: alt på denne server, som ingen planlagt sikkerhedskopiering dækker, hver med sin årsag, for eksempel **aldrig tilføjet**, **ikke inkluderet i tidsplanen** eller **alle destinationer i dets område er slået fra**. Kortet læser containere direkte fra Docker, så det finder også en, som BombVault aldrig har set.
- **Kalender**: de seneste 90 dage, ét felt pr. dag. Et klik på en dag viser dens linjer i loggen under **Historik**.
- **Lagring**: hvor meget hvert område fylder, og hvor godt dets data dedupliceres.
- **Oversigt**: det ældste gendannelsespunkt på tværs af alle områder, hvornår den næste sikkerhedskopiering kører, og hvordan den sidste gik.
- **Statistik**: hvor mange containere og VM'er der er, hvor mange fejl, og hvor mange planer der er aktive eller sat på pause.
- **Vært-integration**: hvad BombVault når og har brug for på værten, for eksempel Docker-socket, libvirt, restic og rclone. Start her, når noget ser ud til ikke at være koblet til.

![Tilpas-tilstand: kortene kan trækkes, ændres i størrelse og skjules, og de skjulte venter nedenunder.](assets/screenshots/overview/customize-dark.png#only-dark)
![Tilpas-tilstand: kortene kan trækkes, ændres i størrelse og skjules, og de skjulte venter nedenunder.](assets/screenshots/overview/customize-light.png#only-light)

## Sådan tæller BombVault {#counting}

Hvert kort og statusdelen tæller på samme måde: posterne i de aktiverede områder, som stadig er installeret, og som du ikke har markeret **Sikkerhedskopiér ikke**. En post tæller kun som beskyttet, så længe en tidsplan når den, og den har en sikkerhedskopi. En gammel sikkerhedskopi alene tæller ikke, så en container, du har taget ud af tidsplanen, vises som ubeskyttet, indtil du sætter den ind igen eller markerer den **Sikkerhedskopiér ikke**.

## Første trin {#first-steps}

Ved en ny installation starter oversigten med kortet **Første trin**: **Vælg områder**, **Tilføj destination** og **Første sikkerhedskopi**. Det bliver stående, indtil den første sikkerhedskopiering er færdig. [Kom godt i gang](getting-started.md#first-run) gennemgår det.
