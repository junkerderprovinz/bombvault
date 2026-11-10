# Gjenoppretting

Hver gjenoppretting starter på siden **Gjenoppretting**: velg et område, søk etter det du vil ha tilbake og velg en sikkerhetskopi. Ett vindu gjenoppretter alle typer elementer og spør i trinn. Som standard endres ingenting på originalen, fordi sikkerhetskopien kommer tilbake ved siden av den.

![Siden Gjenoppretting med områdene som fliser, søket og kortet Stacker.](assets/screenshots/restore/page-dark.png#only-dark)
![Siden Gjenoppretting med områdene som fliser, søket og kortet Stacker.](assets/screenshots/restore/page-light.png#only-light)

## Siden Gjenoppretting {#page}

- **Områder** lister områdene som fliser, hver med hvor mange elementer som har øyeblikksbilder.
- **Hva vil du hente tilbake?** søker i elementene i det valgte området. Hver rad viser det nyeste øyeblikksbildet, og **Åpne element** fører til elementets side.
- **Finn en fil** leter etter en fil eller mappe etter navnet i hver sikkerhetskopi, se [Finne en fil](#find-a-file).
- **Stacker** lister Compose-prosjektene med minst to sikkerhetskopierte containere, for eksempel `immich` med databasen og Redis. Søket finner en stack på navnet eller på en av containerne.
- **Pågår nå** viser en gjenoppretting mens den kjører, også etter at du har lukket vinduet.
- **Nød** starter nødveiviseren for en server som er borte eller satt opp på nytt, se [Nødveiviser](offsite-recovery.md#emergency).
- **Andre kilder** har **Fra en gammel sikkerhetskopimappe**, for sikkerhetskopiene til en annen BombVault, **Importer fra Appdata.Backup-programtillegget** og **Jobber fra Duplicati eller Duplicacy**.
- **Andre servere** lister de andre serverne i et felles repositorium som denne serveren er med i, se [Andre servere](#other-servers).

![Siden Gjenoppretting på en telefon.](assets/screenshots/restore/page-phone-dark.png#only-dark)
![Siden Gjenoppretting på en telefon.](assets/screenshots/restore/page-phone-light.png#only-light)

## Gjenopprettingsvinduet {#window}

Et klikk på et element åpner **Gjenopprett** med navnet. Vinduet spør i trinn, og hvert trinn kan endres før du starter:

1. **Sikkerhetskopi**: det anbefalte øyeblikksbildet er forhåndsvalgt. Det er det nyeste, eller etter et åpent avvik det siste før det. Sikkerhetskopien som ble tatt før en tidligere gjenoppretting, inneholder tilstanden den gjenopprettingen erstattet, så den hoppes over med mindre det ikke finnes noen annen. Eldre ligger i listen, og **Sammenlign** viser hva som skiller to av dem.
2. **Kilde**: hvor filene kommer fra, når et øyeblikksbilde ligger flere steder. **Lokal** er raskest.
3. **Hva**: **Alt**, eller **Enkeltfiler**, der du krysser av filene og mappene du vil ha.
4. **Hvor til**: målet. Valgene avhenger av typen element:

    | Mål | Hva det gjør |
    |---|---|
    | **Ved siden av originalen** | Standarden. En container kommer tilbake som en ekstra container med navnet `<name>-restored` og egne porter, en VMs disker og NVRAM havner i en mappe som heter `<folder>-restored`, og en mappesamling kommer tilbake som `<path>-restored`. Ingenting endres på originalen. |
    | **Erstatt originalen** | Overskriver det som ligger der nå. Med **Ta sikkerhetskopi av dagens tilstand først**, som er på som standard, tar BombVault først sikkerhetskopi av dagens tilstand, så du kan gå tilbake. Hvis den sikkerhetskopien feiler, starter ikke gjenopprettingen. Det virker også for en container eller VM som ikke lenger finnes på serveren: mappene gjenopprettingen skriver over, sikkerhetskopieres hver for seg. |
    | **Til en mappe** | Bare filene, til gjennomsyn. |
    | **Last ned som ZIP** | Flash-disken, til en ny minnepinne. BombVault skriver aldri inn i den kjørende flash-disken. |
    | **Til et nytt dataset** og **Gjenopprett inn i datasettet** | ZFS-datasett, se [ZFS-datasett](zfs-datasets.md#restore). |

5. **Hva som skjer** sier med ord hva gjenopprettingen kommer til å gjøre, og en scene viser målet du valgte.

Før **Gjenopprett** låses opp, sjekker vinduet at depotet svarer, at nøkkelen åpner det, at øyeblikksbildet finnes og at målet har plass. Mens en sjekk feiler, forblir knappen låst, og (i)-en i den sier hvilken.

Vinduet viser også hva gjenopprettingen endrer sammenlignet med det som ligger der nå: nye, erstattede og uendrede filer, med listen på forespørsel, og filene på målet som ikke er i sikkerhetskopien og blir liggende. restic regner dette ut som en tørrkjøring fra størrelse og endringstid, uten å lese filene, og et veldig stort tre stopper etter 30 sekunder og sier fra om det. For containere og VM-er sammenligner det innstillingene gjenopprettingen oppretter på nytt med de som kjører: image og tag, porter, variabelnavn og volumer, eller minne, vCPU-er, disker og nettverk.

Noen ting kan stoppe en gjenoppretting før den endrer noe. En containers statiske IP og publiserte porter må være ledige, og nettverket må finnes. På Unraid avviser hver gjenoppretting på stedet en mappe som ikke ligger på en montert pool eller share, fordi filene ville havnet i minnet og vært borte etter en omstart. En gjenoppretting på stedet nevner hver annen container hvis bind-montering når inn i en mappe den skriver til, for eksempel «denne stien brukes også av nextcloud-db», men det er bare en advarsel.

Noen valg dukker opp der de passer: **La stå stoppet etterpå**, for å se over en container eller VM før den kjører igjen, og **Gjenopprett uten GPU og runtime** på en vert som mangler GPU-en eller runtime-en sikkerhetskopien ble laget med.

Gjenopprettingen kjører i bakgrunnen. Du kan lukke vinduet, og framdriften vises også under **Pågår nå**. En lang gjenoppretting viser en prosent og kan avbrytes, og en avbrutt gjenoppretting registreres som avbrutt, ikke som mislykket. En gjenoppretting som avbrytes mens containeren stopper, eller en som ikke kan fjerne containeren, har ikke gjenopprettet noe ennå, så containeren starter igjen. En container som tar volumene sine fra en annen med `--volumes-from`, beholder dem, og gjenopprettingen nekter å begynne så lenge den andre containeren ikke finnes.

![Gjenopprettingsvinduet ved trinnet Hva, med enkeltfiler avkrysset.](assets/screenshots/restore/window-what-dark.png#only-dark)
![Gjenopprettingsvinduet ved trinnet Hva, med enkeltfiler avkrysset.](assets/screenshots/restore/window-what-light.png#only-light)

### Hvor en gjenoppretting til en mappe havner {#folders}

**Mapper for gjenoppretting** under **Innstillinger, Gjenoppretting** setter **Standard gjenopprettingsmappe**, med en **Undermappe per område** hvis du vil, og et **Unntak** for ett område. Valgte filer havner rett i målmappen: én fil kommer tilbake som bare den filen, en mappe som mappen med innholdet, og valg fra ulike mapper beholder bare stien under mappen de deler. BombVault avviser et mål som ikke ligger på montert lagring, fordi en slik mappe på Unraid ligger i minnet og er borte etter en omstart, og den gjenoppretter aldri inn i sin egen datamappe.

## Eksempel: hent tilbake én fil {#example-single-file}

Noen redigerte Nextclouds `config.php` i går kveld, og Nextcloud starter ikke lenger. Nattens sikkerhetskopi fra kl. 03:00 har fortsatt den gode filen. Du vil ha akkurat den ene filen, ikke hele containeren.

1. Åpne **Gjenoppretting**, velg flisen **Kontainere** og skriv `nextcloud` i **Hva vil du hente tilbake?**.
2. Klikk på `nextcloud`. Vinduet åpnes med sikkerhetskopien fra kl. 03:00 forhåndsvalgt som den anbefalte.
3. La **Kilde** stå på **Lokal**.
4. Under **Hva** velger du **Enkeltfiler**. Åpne mappene ned til `config` og kryss av `config.php`.
5. Under **Hvor til** velger du **Til en mappe** og velger en mappe, for eksempel `user/restore`, som er `/mnt/user/restore` på verten.
6. Les **Hva som skjer**: filen havner i målmappen, og ingenting endres på containeren. Trykk på **Gjenopprett**.

Vinduet ender med **Ferdig: filene ligger i /mnt/user/restore**. Du finner filen som `/mnt/user/restore/config.php`, uten mappene over. Sammenlign den med den ødelagte, kopier den tilbake til appdata-mappen og start Nextcloud på nytt.

For å legge filen rett tilbake i stedet velger du **Erstatt originalen** i trinn 5. La **Ta sikkerhetskopi av dagens tilstand først** stå på, så BombVault tar sikkerhetskopi av den ødelagte tilstanden før den skriver, skriv `nextcloud` for å bekrefte og trykk på **Gjenopprett**. Nextcloud stoppes mens den valgte filen skrives tilbake på plassen sin, og startes igjen etterpå, og bare filene du krysset av, skrives. En gjenoppretting som feiler underveis, lar Nextcloud stå stoppet, fordi filen den leser, kan være halvskrevet, og kjøringen sier det.

![Gjenopprettingsvinduet ved Hva som skjer, med Erstatt originalen valgt.](assets/screenshots/restore/window-replace-dark.png#only-dark)
![Gjenopprettingsvinduet ved Hva som skjer, med Erstatt originalen valgt.](assets/screenshots/restore/window-replace-light.png#only-light)

## Finne en fil {#find-a-file}

Når du vet navnet på en fil, men ikke hvilket element eller hvilken sikkerhetskopi som fortsatt har den, skriver du navnet i **Finn en fil** og trykker på **Søk**. Vanlig tekst treffer hvor som helst i navnet og skiller ikke mellom store og små bokstaver, så `invoice` finner `Invoice-2024.pdf`. Med `*` eller `?` må mønsteret passe til hele navnet, så `*.pdf` finner alle PDF-er. Skriv bare navnet, uten mappe.

Søket leser hver sikkerhetskopi av containerne, mappesamlingene, ZFS-datasettene og flash-disken. Hvert treff viser elementet, stien og hver sikkerhetskopi som har filen, nyeste først, med dato, størrelse og målet den ble lest fra. En sikkerhetskopi der filen har samme størrelse og endringstid som i den forrige, viser **uendret**, så du ser hvor filen sist ble endret. Et klikk på en sikkerhetskopi åpner gjenopprettingsvinduet på den, med **Enkeltfiler** valgt og filen krysset av. En fil fra flash-disken kommer tilbake som en del av ZIP-filen til flash-disken, fordi flash-disken bare gjenopprettes som en helhet.

Søket tar litt tid, fordi restic går gjennom hver sikkerhetskopi. **Stopp** avslutter det, og det gjør det også å forlate siden. Under feltet står det hvilke mål det leste:

- Mål på denne serveren og i nettverket ditt blir alltid gjennomsøkt.
- En sikkerhetskopi som ligger på flere mål, leses bare én gang, på det første. Et mål der alle sikkerhetskopiene ble lest et annet sted, viser **allerede gjennomsøkt på et annet mål**.
- Off-site-mål som S3, B2 eller SFTP holdes utenfor, med mindre du slår på **Søk også i off-site-mål**. Da leses bare sikkerhetskopiene som ingen mål her har, som regel de eldre som en lengre oppbevaring beholder off-site. Det første søket på et slikt mål kan ta lang tid, og noen leverandører tar betalt for dataene som lastes ned.
- Et mål som ikke kunne åpnes, viser **kunne ikke leses**, og (i)-en ved siden av forteller hvorfor. De andre målene blir fortsatt gjennomsøkt.

Virtuelle maskiner lagres som hele disker, så filene inni dem blir ikke gjennomsøkt. Auto-sikkerhetskopien, sikkerhetskopiene til en annen BombVault under **Andre kilder** og de til serverne under **Andre servere** blir heller ikke gjennomsøkt. Et søk lister de første 200 filene, så et kort navn som `log` kan trenge noen flere bokstaver.

## Hva som kommer tilbake, etter type {#kinds}

**Containere** opprettes på nytt fra den lagrede definisjonen gjennom Docker-APIet, med entrypoint, vertsnavn, stoppinnstillinger, volumer og monteringer, ressursgrenser, loggdriver, DNS-innstillinger og gamle lenker. En container kommer tilbake på imaget den kjørte da sikkerhetskopien ble tatt, ikke på et nyere som ble hentet senere, og et image som mangler, hentes først, med påloggingen fra **Container-registre** under **Innstillinger, Gjenoppretting**. GPU-en eller runtime-en (`--gpus`, `--runtime=nvidia`) kommer også tilbake. På en vert uten den driveren eller runtime-en sier gjenopprettingen fra om det og tilbyr **Gjenopprett uten GPU og runtime**. En lenke til en container som mangler, eller som er stoppet mens den gjenopprettede starter, utelates, og kjørehistorikken sier fra om det. En gjenoppretting som feiler etter at containeren ble fjernet, setter den tilbake, stoppet.

En sikkerhetskopi kan inneholde mapper containeren ikke lenger har valgt, for eksempel en eldre som ble tatt før en mappe forsvant. En mappe som er borte eller tom, kommer tilbake. En som inneholder filer nå, blir som den er, fordi du kan ha fjernet avkrysningen med vilje, og kontrollen før gjenopprettingen og kjørehistorikken nevner den. Få den tilbake med **Enkeltfiler** eller **Til en mappe**.

**VM-er** får XML-en sin definert på nytt over SSH og diskene og UEFI-NVRAM tilkoblet igjen, selv etter at VM-en ble slettet. Se [VM-sikkerhetskopier](vm-backup-ssh-setup.md) for VM-er med Unraid-øyeblikksbilder og disker på zvols.

**Mappesamlinger** kommer tilbake med eierskap, tilgangsmoduser og tidsstempler slik restic lagrer dem. Der målet bare avviser eierskapet, slik `/mnt/user` gjør, ender kjøringen med en merknad som sier det. BombVault sammenligner så filene med sikkerhetskopien, og en fil som ikke kom fram, får gjenopprettingen til å feile.

**Flash-disken** rører aldri den levende `/boot`. Gjenopprettingen er en nedlasting som heter `flash-<id>.zip`, klar til å slippes inn i Unraid USB Creator. ZIP-filen inneholder `APP_KEY`, så nedlastingen krever et påloggingspassord og en pålogging på instansen. Vinduet lister også programtilleggene i hver flash-sikkerhetskopi med versjon og størrelse, og kan legge ett enkelt tilbake på den kjørende flash-disken: `.plg`-filen, mappen under `config/plugins` og pakkefilene sikkerhetskopien har. Unraid installerer det ved neste oppstart, eller med en gang under **Plugins, Install Plugin**.

**ZFS-datasett** har egne mål og beholder ZFS-egenskapene sine, se [Gjenoppretting](zfs-datasets.md#restore).

**Auto-sikkerhetskopien** kommer bare tilbake gjennom nødveiviseren, som starter BombVault på nytt, så den levende databasen aldri overskrives under et åpent håndtak.

**Databasedumper** lastes aldri tilbake av seg selv. En rad under **Databasedumper** kan lastes ned, lagres i en mappe eller importeres i en nystartet database. Se [Databasedumper](backups.md#database-dumps).

## Gjenopprette flere elementer samtidig {#several}

På siden Gjenoppretting velger du elementer på samme måte som du velger rader i listen Sikkerhetskopier: Ctrl+klikk, Shift+klikk, Ctrl+A, et langt trykk eller **Velg**. **Gjenopprett** åpner da ett vindu for alle.

- **Hvilken sikkerhetskopi**: **Anbefalt** tar for hvert element sikkerhetskopien dets eget vindu ville anbefalt. **Ett tidspunkt for alle** tar for hvert element den nyeste sikkerhetskopien fra et **Tidspunkt** du velger eller eldre, for eksempel kvelden før noe gikk galt.
- **Hvor til**: **Ved siden av originalen** eller **Erstatt originalen**, med **Ta sikkerhetskopi av dagens tilstand først**.

Hvert element sjekkes på samme måte som en enkeltgjenoppretting, og deretter kjører de ett etter ett. Hver rad viser hvordan det gikk med det: **Venter**, **Kjører**, **Gjenopprettet**, **Mislyktes** eller **Holdt utenfor**. **Stopp etter dette** lar den kjørende gjenopprettingen bli ferdig og starter ingen flere. Flash-disken kommer som en ZIP å laste ned, og ZFS-datasett og auto-sikkerhetskopien gjenopprettes én om gangen.

![Gjenoppretting av flere elementer: én rad per element med framdriften.](assets/screenshots/restore/several-dark.png#only-dark)
![Gjenoppretting av flere elementer: én rad per element med framdriften.](assets/screenshots/restore/several-light.png#only-light)

## Gjenopprette en stack {#stacks}

Et klikk på en stack i kortet **Stacker** åpner **Gjenopprett stack**. Den gjenoppretter hvert medlem fra siste sikkerhetskopi samtidig, og med **Start i avhengighetsrekkefølge etter gjenoppretting** starter den dem i `depends_on`-rekkefølge: databaser og cacher først, så appen. Når et medlems prosjektmappe mangler på stedet du valgte, tilbyr vinduet tilstanden fra Lokal i stedet.

BombVault sjekker medlemmene og prosjektmappen først, og en gjenoppretting som ikke kunne fungere, endrer ingenting. Ellers stopper den medlemmene som kjører, appene før databasene deres, skriver så prosjektmappen og gjenoppretter så medlemmene. Kommer ikke prosjektmappen tilbake, blir medlemmene likevel gjenopprettet, men de forblir stoppet, fordi mappen kan være halvskrevet og det er der bind-monteringene til et medlem vanligvis ligger. Kjøringen til hvert medlem sier det.

En database som bare har databasedumper og ingen filsikkerhetskopi, og der containeren og det navngitte volumet er borte, kommer tilbake uten dataene sine. Kontrollen sier det før gjenopprettingen. Den forblir stoppet, og det samme gjør containerne som er avhengige av den, så ingen app kjører oppsettet sitt mot en tom database. Start den, importer en dump og start de andre. Se [Databasedumper](backups.md#database-dumps).

## Andre kilder {#other-sources}

- **Fra en gammel sikkerhetskopimappe** åpner depotet til en annen BombVault som ikke lenger kjører, på en disk eller på et eksternt mål, med gjenopprettingssettet eller en innskrevet `APP_KEY`. Økten er skrivebeskyttet og lever bare i minnet. Se [Fra en gammel sikkerhetskopimappe](offsite-recovery.md#restore-from-another-bombvault-repo).
- **Importer fra Appdata.Backup-programtillegget** leser programtilleggets sikkerhetskopimappe. Hvert containerarkiv blir et gjenopprettingspunkt for containeren sin, datert til når programtillegget laget det. Arkiver som er importert før, hoppes over, og selve arkivene bare leses. Containeren trenger først én sikkerhetskopi i BombVault, så gjenopprettingen har definisjonen. Oppbevaring lar importerte gjenopprettingspunkter være i fred, så slett ett selv når du ikke trenger det mer.
- **Jobber fra Duplicati eller Duplicacy** gjør sikkerhetskopijobbene i disse verktøyene om til mappesett. Importen leser bare innstillingene deres. De gamle sikkerhetskopiene blir igjen hos det gamle verktøyet, og BombVault begynner med en egen første sikkerhetskopi. Ingen av verktøyene trenger å være installert her.
    - **Fra Duplicati**: eksporter hver jobb til en fil uten passord i Duplicati, og velg `.json`-filene. En eksport som tar med passordene, tar også med påloggingen til målet.
    - **Fra Duplicacy**: velg Duplicacy Web-mappen som inneholder `duplicacy.json`, eller en mappe du sikkerhetskopierer med Duplicacys kommandolinje, som inneholder `.duplicacy`. Nøkler som kommandolinjen har lagret i `.duplicacy/preferences`, blir med. Duplicacy Web lagrer påloggingene sine kryptert, så vinduet ber deg om dem.

    Ingenting opprettes før du har sett forhåndsvisningen. Hver kildemappe blir et mappesett med mappen på denne serveren der den ble funnet, ekskluderingene som kunne oversettes, og tidsplanen. Hvis det gamle verktøyet kjørte i en egen container og en mappe ikke ble funnet, velger du den selv. Sikkerhetskopiene går til et nytt mål i en mappe `bombvault` inne i det gamle målet, når det er en lokal mappe, S3, SFTP, en rclone-remote eller Backblaze B2 via S3-adressen. En pålogging fra filen lagres sammen med de andre påloggingene dine, og en pålogging som mangler i filen, blir du spurt om. Merknader under hver jobb forteller hva som ikke ble med, for eksempel inkluderingsregler, regulære uttrykk, hvor lenge sikkerhetskopiene ble beholdt, og det gamle krypteringspassordet, som BombVault ikke trenger.

## Andre servere {#other-servers}

I et [felles repositorium](offsite-recovery.md#shared-repository) åpner hver nøkkel sikkerhetskopiene til alle serverne. Derfor lister siden Gjenoppretting de andre medlemmene under **Andre servere**, hver med navn, vert, hvor mange elementer den har og når den nyeste sikkerhetskopien kjørte. En vert som ikke hører til noen server denne kjenner, vises som **En server denne ikke kjenner**. Et klikk åpner **Sikkerhetskopier fra** og navnet på serveren, skrivebeskyttet og uten noe å skrive inn. Mappesett og ZFS-datasett havner i en mappe du velger. Containere og VM-er kommer tilbake som vanlige filer i en mappe, fordi den andre serveren forsegler definisjonene deres med sin egen nøkkel. Denne serverens egne sikkerhetskopier i det felles repositoriet ligger på fanen **Sikkerhetskopier** for hvert element.

## Etter en gjenoppretting {#after}

En kopi som er gjenopprettet ved siden av originalen, står under **Sikkerhetskopier** som utelatt, så den ikke teller som en åpen oppgave. Ta den med i tidsplanen hvis du vil beholde den. En container eller VM som kjørte da den ble sikkerhetskopiert, kommer tilbake kjørende, og en som var stoppet, forblir stoppet. På Unraid dukker en gjenopprettet container opp igjen i Docker-fanen som en vanlig, redigerbar app, og en VM i VM Manager med diskene og NVRAM tilkoblet.
