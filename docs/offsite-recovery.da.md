# Off-site og gendannelse

!!! note "Eksterne kopier venter efter en genopbygning"
    Når Nødguiden eller **Opdag sikkerhedskopier** genopbygger elementer uden de gamle indstillinger, holder off-site-kopier af de områder pause, til deres standard er bekræftet. Se [Efter en genopbygning](#after-a-rebuild).

Lokale sikkerhedskopier beskytter dig mod en tabt container eller en dårlig opdatering. Off-site-kopier og et afprøvet gendannelseskit beskytter dig mod hele boksen, ransomware eller en brand. Denne side dækker destinationer og off-site-kopier, at gøre en kopi manipulationssikker, at bevise, at du kan gendanne, parring af flere servere og at få alt tilbage, når BombVault selv er væk.

## Off-site-kopier {#off-site-copies}

Behold den hurtige lokale sikkerhedskopi og tilføj en eller flere destinationer uden for huset. BombVault kopierer nye sikkerhedskopier dertil med `restic copy` efter bedste evne, så et off-site-hikke aldrig får den lokale sikkerhedskopi til at fejle. I denne form forbliver det lokale repository det primære og off-site-repositoriet en kopi, men et områdes primære repository behøver slet ikke være lokalt; se [Fjerne primære repositories](#remote-primary-repositories) nedenfor for at sikkerhedskopiere direkte til S3, en rest-server og lignende i stedet for at kopiere dertil.

- **Flere destinationer pr. område.** Hvert område (containere, VM'er, flash, auto-sikkerhedskopi, mapper og ZFS-datasæt) kan kopiere til flere off-site-destinationer på én gang, så du for eksempel kan have en rest-server på en vens boks og en S3-bucket side om side. Kortet **Destinationer pr. område** på siden Destinationer slår hver linje fra et område til en destination til eller fra.
- **Hvornår kopierne kører.** En destinations kort **Kopier** sætter **Hvornår der kopieres**: **Efter hver sikkerhedskopi**, eller efter en egen plan, der samler sikkerhedskopier og kopierer på faste tidspunkter. Under **Indstillinger, Tidsplan, Off-site-kopier pr. område** giver **Undtagelse** ét område sin egen kopiplan. **Replikér nu** i et områdes kort **Off-site-kopi** under **Indstillinger, Destinationer** starter en kopiering på forlangende.
- **Opbevaring** sættes pr. destination under **Hvor længe der gemmes**, så du kan opbevare off-site-kopier længere som et arkiv. Kopien i et områdes off-site-felt følger kortet **Off-site-opbevaring** under **Indstillinger, Destinationer**; lad det stå helt på nul for aldrig at trimme off-site-sikkerhedskopier automatisk.
- **Båndbreddegrænser** (**Off-site-båndbredde** under **Indstillinger, Destinationer**, eller en destinations kort **Kopier**) begrænser restics upload/download-hastighed, så kopier ikke mætter din WAN.
- Mens en kopiering kører, siger statushovedet på oversigten **Off-site-kopi kører**. Det er en aktivitetsindikator, ikke en procentbjælke, fordi `restic copy` ikke afslører et maskinlæsbart forløb.

!!! note "Gendan fra ethvert sted"
    Hvert element oplister sine sikkerhedskopier som én tidslinje på tværs af alle steder, hvor en sikkerhedskopi ligger, under fanen **Sikkerhedskopier**. En sikkerhedskopi kopieret til B2 vises én gang, markeret med hvert sted, der rummer den. En gendannelse tager det første sted, den kan nå, begyndende med det repository, elementet skrives til, og du kan vælge et andet sted pr. sikkerhedskopi. Off-site-destinationer læses først, når du klikker på **Tjek**, fordi det tager lidt tid at oplistes cloudlager. Sletning ét sted tjekker først de andre og siger, om det var den sidste kopi.

## Destinationer {#destinations}

En destination er et sted, sikkerhedskopier havner: en lokal mappe, en share, en server du selv har eller en cloudtjeneste. Den sættes op én gang og tjener så hvert område. Åbn **Destinationer** fra dens indstillingsfelt, eller slå **Vis også i sidepanelet** til dér for at fæste den.

**Tilføj destination** åbner et vindue i fire trin:

1. **Udbyder.** Hver tjeneste er oplistet med sit logo, med en søgning over dem, i grupper: lagertjenester med S3-buckets (Backblaze B2, Wasabi, Cloudflare R2, Hetzner Object Storage, Amazon S3 og andre), din egen S3-server (Garage, SeaweedFS, RustFS, Silo, Ceph, JuiceFS, Versity S3 Gateway), din egen server og shares (rest-server, Hetzner Storage Box, SFTP, SMB, WebDAV, en monteret sti), cloudlager (OneDrive, Google Drive, Dropbox, pCloud, Nextcloud og resten, rclone understøtter) og din gruppes modtagerservere. Hver siger, hvor godt den egner sig til sikkerhedskopier: cloud-drev bliver langsomme under mange forespørgsler, så den første sikkerhedskopi og prune tager længere dér.
2. **Log ind.** Felterne afhænger af tjenesten: en adgangsnøgle til S3, en bruger og adgangskode til WebDAV og SMB, en app-adgangskode, hvor tofaktorlogin blokerer det normale, BombVaults offentlige SSH-nøgle til SFTP og Storage Box, eller et token til tjenester, der logger ind gennem en browser. Til dem viser vinduet en `rclone authorize`-kommando, der kører på en computer med en browser; det token, den udskriver, kommer i feltet. **Test** tjekker loginnet, før noget gemmes. Vinduet oplister så mapperne på destinationen, med **Ny mappe** til at oprette én og den ledige plads, hvor tjenesten rapporterer den. En tom mappe er sikrest; BombVault lægger én mappe pr. område i den.
3. **Beskyttelse.** Vinduet siger ligeud, hvad tjenesten kan. En rest-server i append-only-tilstand afviser sletning, og tamper-testen tjekker det. En S3-bucket kan beholde gamle versioner med versionering og objektlås, hvilket BombVault endnu ikke kan tjekke. Et cloud-drev kan slet ikke afvise sletning: den, der kommer ind på serveren, kommer også ind på den kopi. Slå **Beskyttelse mod sletning (append-only)** til kun, hvor den anden side virkelig afviser sletning; BombVault trimmer så aldrig dér. **Til en nødsituation** siger, hvad gendannelseskittet rummer for denne destination.
4. **Til hvilke områder.** Vælg de områder, der sikkerhedskopierer her. Den første kopi tager hele historikken med. Elementer, der har valgt egne destinationer, får ikke den nye af sig selv, og vinduet navngiver dem.

S3-tjenester kører gennem restics eget S3-backend, hvilket er det, der lader en lagerklasse og objektlås gælde. Enhver anden tjeneste kører gennem den rclone, BombVault leverer, og dens remote vises så i kortet **Off-site (rclone)** under **Indstillinger, Destinationer**. En indstillingseksport bærer destinationerne; med legitimationsoplysninger inkluderet bærer den også deres login.

Hver destination har en side for sig:

- **Forbindelse** med dens navn, kontakten **Tilgængeligt** (fra betyder, at BombVault ikke længere skriver her, og at elementer, der sikkerhedskopierer her, fejler med en tydelig besked i stedet for at havne et andet sted), mappen, komprimering, lagerklasse og **Uden for bygningen**. **Skift forbindelse** tester og gemmer en ny adresse eller et nyt login.
- **Legitimationsoplysninger**: det delte sæt eller et sæt for sig.
- **Beskyttelse mod sletning**, med **Test beskyttelsen mod sletning nu** og hvem der trimmer denne kopi.
- **Kopier**: hvornår der kopieres, og upload- og downloadgrænserne.
- **Hvor længe der gemmes**: **Kort**, **Afbalanceret**, **Lang** eller **Egne værdier**, med en tidslinje over, hvilke sikkerhedskopier reglen beholder. **Spørg før der gemmes mindre** spørger, før en kortere regel sletter sikkerhedskopier, hvis eneste kopi ligger her.
- **Regler pr. område**: ét områdes eget repository, egen opbevaring eller komprimering på denne destination. **Ændr regler** åbner dem, og **Tilbage til destinationen** henter et område tilbage.
- **Ligger allerede under denne destination**: områder, hvis håndskrevne adresse ligger i en mappe på denne destination. At overtage ét lader repositoriet blive, hvor det er; login, lagerklasse og beskyttelse mod sletning kommer så fra destinationen. BombVault tjekker først, at destinationens login åbner repositoriet, og nægter at lægge et append-only-repository under en destination, der ikke er append-only.
- Et kar, der viser, hvor fuld destinationen er, med mærket for, hvor den er fuld, og hvor mange uger der er tilbage ved de seneste ugers vækst. En destination, der ikke rapporterer sin størrelse, siger **Størrelse ukendt**.

En modtagerserver, som en anden instans i din gruppe kører, vises blandt udbyderne som **Fra din gruppe**; se [Modtageserver](#receiving-server).

### Flyt og fjern {#move-remove}

- **Flyt** tager kopien af ét område til en anden destination, med hele dens historik. Alt bliver på det gamle sted; BombVault sletter intet dér. Flytningen kører i baggrunden og kan tage lang tid: omkring 9 minutter pr. 100 GB mellem lokale diske, timer over et langsomt link. Oversigten viser den, mens den kører.
- **Fjern destination** tilbyder to veje. **Fjern destination, behold sikkerhedskopierne** holder op med at skrive dér og lader sikkerhedskopierne ligge; du kan læse dem senere under **Gendannelse, Fra en gammel sikkerhedskopimappe**. **Fjern destination og slet sikkerhedskopierne dér** læser først, hvad der er gemt dér, og oplister det: hvilke sikkerhedskopier der forsvinder, hvilke af dem der ikke findes andre steder, og hvilke repositories der bliver, fordi sletningsbeskyttelse afviser, elementer stadig sikkerhedskopierer direkte til dem, en anden destination eller et andet område har sine kopier dér, eller de ikke kunne læses. Elementer, der ville miste deres eneste sikkerhedskopi, oplistes for sig og beholdes, medmindre du sætter hak i **Slet også disse**. Du bekræfter ved at skrive destinationens navn, og sletningen kører i baggrunden.
- At slette sikkerhedskopierne i en destination kræver en login-adgangskode og en login på instansen. Uden en adgangskode kan destinationen stadig fjernes, kun dens sikkerhedskopier kan ikke slettes herfra.

## Placering pr. element {#placement}

Kortet **Tidsplan og destination** under et elements fane Indstillinger har en række knapper: **Lokal** og én knap for hver off-site-destination i området. De oplyste knapper får elementets sikkerhedskopier.

- Med **Lokal** oplyst skrives elementet til det repository, der vises under **Gemt på**, og kopieres til hver anden oplyst destination. Gør en destination mørk, og den får intet nyt fra dette element. Lokal alene kopierer ingen steder hen, hvilket passer til data, der allerede har en anden kopi, for eksempel en share, der ligger på en NAS.
- Med **Lokal** mørk skrives elementet direkte til det direkte repository på den første oplyste destination og kopieres derfra til de andre oplyste. Første gang opretter en dialog det direkte repository.
- Én knap forbliver oplyst, fordi en sikkerhedskopi skal have et sted at havne. For at udelade et element fra sikkerhedskopier bruges **Sikkerhedskopiér ikke**.

Stedet er fast fra elementets første sikkerhedskopi, fordi BombVault aldrig flytter sikkerhedskopier mellem repositories; kortet **Repository** under fanen Indstillinger navngiver det. Kopierne kan ændres når som helst. En destination, der ikke længere får et element, beholder de kopier, den har, og trimmer dem efter sin egen opbevaring ved områdets næste off-site-kørsel. Der kan ikke slettes fra append-only-destinationer.

Under rækken siger kortet, hvor elementet havner, og hvad der faktisk er der: hvor mange steder der rummer det, hvornår hver destination sidst blev set, og om 3-2-1 er opfyldt. Et sted er serveren med de oprindelige data, hver off-site-destination og hvert repository markeret **Uden for bygningen**. BombVault tjekker kopier og steder; det tjekker ikke »to medier«-delen af 3-2-1.

### Standarder pr. område {#placement-defaults}

Kortet **Destinationer pr. område** på siden Destinationer er standarden for hvert område. I grafen er hver linje en kopi, og et klik på en linje eller en prik slår den til eller fra; stiplede linjer er slået fra. **Graf / Til/fra** viser det samme som rækker af knapper. Kopierne gælder med det samme for hvert element uden eget valg og for projektmapperne i Compose-stacks. Destinationen gælder for et nyt element ved dets første sikkerhedskopi; at ændre den flytter ingen sikkerhedskopier. Før gemning navngiver BombVault hver destination, der får eller mister elementer, og hvor mange sikkerhedskopier det betyder.

Elementer, der sikkerhedskopierer et andet sted end deres område, står i kortet **Eget valg**, hvert med **Følg standarden**. Nye sikkerhedskopier går så til områdets destinationer, og de tidligere bliver, hvor de er.

En ny off-site-destination får hvert element, der ikke er sat til Lokal. Vinduet, der tilføjer den, siger, hvor mange elementer det er, og, hvor det er kendt, hvor meget historik.

### Direkte arkiver

At slå Lokal fra for et element, så en destination uden et direkte repository bliver dets hjem, åbner en dialog med en foreslået placering ved siden af, for eksempel `s3:https://s3.eu-central-003.backblazeb2.com/bucket/containers-direct`, og en forbindelsestest, der ikke opretter noget. **Opret og brug** opretter repositoriet og peger elementet på det. Et direkte repository tager destinationens nøgle, lagerklasse, grænser, append-only-indstilling og opbevaring og ændrer sig med dem; kortet Depoter viser det skrivebeskyttet. Når en ny nøgle til destinationen ikke kan åbne det, beholder det direkte repository den nøgle, det har, og gemningen siger det. Et element på et direkte repository kopieres derfra til de andre oplyste destinationer, aldrig til den destination, repositoriet hører til. Dets sikkerhedskopier bærer mærket `bv:direct`, og hvert andet opbevaringsgennemløb beholder dem, så et direkte repository, der mistede forbindelsen til sin destination, aldrig ældes efter de lokale regler. B2 nås gennem dets S3-endpoint, med nøgle-ID og applikationsnøgle indtastet som S3-legitimationsoplysninger; en nøgle begrænset til destinationens egen mappe kan ikke nå mappen ved siden af, så begræns nøglen til mappen over i stedet.

### Uden for bygningen

En destination eller et navngivet repository kan markeres **Uden for bygningen**. Fjerne starter markeret; slå det fra for en rest-server i samme bygning. Mærket tæller kun steder og 3-2-1 på kortene. Det ændrer ingen kopi.

En ZFS-replika er en kopi uden for bygningen, men ikke en sikkerhedskopi med historik. Den beholder kun de snapshots, du sætter, og gamle versioner og enkelte filer kommer stadig fra sikkerhedskopierne. Den går fra en ZFS-server direkte til den anden, over SSH til en ZFS-server eller til modtagepunktet i en parret instans. Parringsgruppen bærer kun anmodningen. Se [Replika](zfs-datasets.md#replica).

### Efter en genopbygning {#after-a-rebuild}

Kopivalg lever i BombVaults egne indstillinger. Efter en genopbygning gennem **Opdag sikkerhedskopier** eller Nødguiden uden gendannede indstillinger er de væk, og at kopiere alt ville sende de elementer, du havde udeladt, til B2 igen. Off-site-kopier af hvert genopbygget område holder derfor pause. Oversigten siger det, og kortet **Destinationer pr. område** tilbyder **Bekræft standard** med en forhåndsvisning af, hvad næste kørsel kopierer, og navnene i sikkerhedskopierne, der ikke har noget element, som du kan udelade dér. Kun bekræftelsen afslutter pausen; import af en indstillingsfil bringer regler og standarder tilbage, men afslutter den ikke.

## Fjernbetjente primære arkiver {#remote-primary-repositories}

Et områdes egen sti er ikke begrænset til en lokal mappe: peg den direkte på en restic-remote (`s3:...`, `rest:http://host:8000/repo`, `sftp:user@host:/repo`, `rclone:remote:bucket/path`), og BombVault sikkerhedskopierer direkte dertil, uden en separat lokal kopi og uden kopitrin. Det er en anden form end off-site-kopierne ovenfor: dér er det lokale repository det primære og off-site-repositoriet et arkiv af det efter bedste evne; her **er** fjern-repositoriet det primære, og det er den eneste kopi, medmindre du også kopierer det område til en anden destination.

Stien for hvert område (Containers, VMs, Flash, Auto-sikkerhedskopi, Mapper, ZFS-datasæt) sættes på siden for destinationen **Lokal mappe**: **Skift** ved siden af området i dens kort **Forbindelse** åbner et vindue med en kontakt **Lokal / Fjern**:

- **Lokal** viser den velkendte mappebrowser.
- **Fjern** bytter det ud med et almindeligt URL-felt og de forbindelsesindstillinger, en off-site-destination bruger, sat op til dette primære repository i stedet. Derfra får du:
    - **En forbindelsestest** mod den rigtige sti, før du forlader dig på den.
    - **Båndbreddegrænser** (upload/download), så en planlagt sikkerhedskopi til et fjern-primært repository ikke mætter din WAN: de samme restic-flag `--limit-upload`/`--limit-download`, som off-site-kopier bruger, anvendt på selve sikkerhedskopien.
    - **Append-only-beskyttelse**, verificeret med den samme aktive tamper-test (en rigtig DELETE-probe mod den anden side), som off-site-destinationer får. Når den er slået til, nægter BombVault selv at trimme repositoriet, da der ikke er en separat lokal kopi bag det, og legitimationsoplysningerne på denne boks ikke må kunne slette den eneste kopi af sikkerhedskopien.
    - **En alarm for vækstbudgettet**, taget fra den samme udvikling i arkivets størrelse, som Lagerkortet allerede følger.

Intet af dette er påkrævet: en håndskrevet fjernsti uden gemte sikkerhedsindstillinger sikkerhedskopierer ubegrænset, kan trimmes og uden budgetalarm. Sikkerhedsindstillingerne er der til, når du vil have de beskyttelser, en off-site-kopi får, uden en separat destination bare for at få dem.

!!! note "Sky- og REST-adgangsoplysninger deles"
    Et fjern-primært repository autentificerer med de delte cloud-legitimationsoplysninger under **Indstillinger, Destinationer**; der er intet separat legitimationslager til primære repositories.

### SMB og WebDAV uden værtsmontering {#smb-webdav}

Kortet **Off-site (rclone)** under **Indstillinger, Destinationer** har en formular til en Windows- eller Samba-share og til en WebDAV-server (Nextcloud, ownCloud, SharePoint eller en hvilken som helst anden). Udfyld et kort navn, værten og sharen (SMB) eller URL'en og servertypen (WebDAV), brugeren og adgangskoden, og BombVault skriver rclone-sektionen for dig. rclone slører selv adgangskoden, før den gemmes; at tilføje en med et navn, der allerede findes, erstatter den sektion i stedet for at tilføje en anden. **Tilføj destination** tilbyder SMB og WebDAV som udbydere med de samme felter.

Formularen svarer med den færdige placering, for eksempel `rclone:nas:backups`. Brug den som et områdes sti eller en destination, og tilføj en undermappe, hvis du vil have en (`rclone:nas:backups/bombvault`). Sharen er det første stisegment, ikke en del af navnet.

Det er en bedre vej end at montere sharen på Unraid: restic fraråder at have et repository på en monteret CIFS-share, og her monteres intet. NFS er ikke i formularen, fordi hverken restic eller rclone har et NFS-backend; til NFS monterer du eksporten på værten og bruger den som en lokal mappe.

## Beskyttelse mod sletning (append-only)

Slå beskyttelse mod sletning til for en off-site-destination, så ransomware eller en kompromitteret vært ikke kan slette eller omskrive dine sikkerhedskopier. Den anden side (en `restic/rest-server`, der kører i tilstanden `--append-only`) **håndhæver** den. BombVault **verificerer** den kun og viser aldrig grønt på en konfigurationspåstand alene. En mappe på denne server kan ikke afvise sletning, fordi den, der styrer BombVault, også styrer mappen, så lokale destinationer tilbyder ikke kontakten.

Den hurtigste vej til en append-only-kopi er en parret BombVault med en [modtageserver](#receiving-server): den kører rest-serveren for dig, i append-only-tilstand.

!!! note "En vellykket sletning under `/locks/` er forventet"
    Append-only betyder ikke, at intet længere kan slettes. restic skal tage og frigive sine egne låse, så `/locks/` forbliver bevidst skrivbar og sletbar. Sikkerhedskopier og dataene bag dem, altså præcis det ransomware ville gå efter, kan ikke fjernes. Tester du selv modparten, er en sletning der lykkes under `/locks/` korrekt adfærd og ikke et hul i beskyttelsen.

!!! warning "Append-only-repositories trimmes aldrig fra denne boks"
    En append-only-destination trimmer aldrig gamle sikkerhedskopier herfra; trimning er den anden sides opgave. Sæt en **vækstbudgetalarm** for den, så du advares, før repositoriets størrelse løber løbsk.

## Manipulationstest

BombVault beviser jævnligt append-only-garantien ved faktisk at forsøge en sletning i off-site-repositoriet, rettet mod et ikke-eksisterende objekt. **Test beskyttelsen mod sletning nu** på en destinations side kører den på forlangende, og **Tjek sletningsbeskyttelse** under **Indstillinger, Verificering, Automatiske tjek** kører den efter en tidsplan.

- **Afvist** betyder beskyttet.
- **Accepteret** betyder ikke beskyttet.
- Et **inkonklusivt** resultat (server uopnåelig, autentificeringsfejl) vender aldrig den gemte dom.

En reel beskyttet-til-ubeskyttet-vending udløser én enkelt advarsel.

## DR-øvelser

BombVault tilbyder to niveauer af bevis for, at dine sikkerhedskopier faktisk kan gendannes, ikke bare er til stede.

- **Gendannelsesverificeringsøvelser (lokale).** BombVault kører jævnligt `restic check --read-data-subset` (afgrænset, aldrig en diskfyldende fuld gendannelse). Kadencen ligger under **Indstillinger, Verificering, Automatiske tjek**, og kortet **Tjek sikkerhedskopier** på samme felt viser resultatet.
- **Prøvegendannelse (off-site).** BombVault gendanner et rigtigt mål fra off-site-repositoriet til en aflukket sandkasse, verificerer det fil for fil og byte for byte og rydder så op. Det beviser, at du kan komme tilbage fra off-site, ikke bare at repositoriet svarer.

Kortet **Beskyttelsesstatus** på oversigten sammenfatter det pr. område i fire tjek: en aktuel sikkerhedskopi på denne server, en kopi off-site, sletningsbeskyttelse dér og en prøvegendannelse derfra. Et klik på et område viser detaljerne, og kortet bliver kun grønt på verificerede fakta.

## Parring af instanser {#pairing}

Flere BombVault-instanser arbejder sammen som én gruppe: de viser hinanden beskyttelsesstatus, overtager roller for hinanden og kan opbevare hinandens nøgle. En instans slutter sig til gruppen med tolv ord.

På den første instans åbner du **Indstillinger, Parring** og trykker på **Generer sætning**. Tre trinkort viser, hvordan det går. Tolv ord vises i et vindue med en knap **Kopier**. På hver anden instans åbner du det samme sted, trykker på **Indtast sætning** og indsætter eller skriver dem, eller trykker på **Indsæt** i det vindue. Et ord, der ikke står på listen, navngives med sin position, mens du skriver, og det sidste ord bærer en kontrolsum, så et forkert tastet eller ombyttet ord opdages, før noget parres. Generer sætningen på kun én instans: to instanser, der begge genererer én, danner to adskilte grupper. Hvis ingen dukker op i et minut, tilbyder siden to veje ud: vis ordene igen for at indtaste dem dér, eller indtast den anden instans' ord og slut dig til dens gruppe i ét trin. Parring virker uden login-adgangskode, men sæt en: uden den kan enhver, der kan åbne web-UI'en, læse ordene og gennem gruppen bede om roller, der udleverer restic-adgangskoden til instanserne i den. Parringskortet siger det, indtil der er angivet en adgangskode. Med en adgangskode spørger den efter den, når sætningen vises igen. **Forlad gruppen** tager en instans ud igen.

Enhver, der kender ordene, kan komme med i gruppen, så behandl dem som en adgangskode.

**Hvordan medlemmerne når hinanden.** Hver instans lærer sin egen adresse på netværket fra din browser i det øjeblik, du logger ind, oplistet under **Kendte adresser** på parringssiden sammen med dem, instansen selv fandt; ret den dér, hvis en reverse proxy eller en usædvanlig port står foran. På samme netværk annoncerer medlemmerne den adresse med multicast og taler direkte, og hvor multicast ikke kan krydse et containernetværk, som Dockers standardbro, søger en instans i stedet sit eget undernet efter de andre med et signeret kald, kun et gruppemedlem kan besvare, så parringen stadig bliver færdig på sekunder uden relay. Hvis intet dukker op, tager **Kan du ikke finde den?** én adresse i hånden, til et andet undernet eller en ikke-standardport. Instanser på forskellige netværk går gennem et relay, valgt på samme side; dets fire former har hver et billede:

- **Projektets videresender** (standard): `parleyport.halleluja.design`, den samme videresender som KnightLoader også bruger. Intet at sætte op.
- **Eget relay**: containeren [**ParleyPort**](https://github.com/junkerderprovinz/parleyport) fra Unraid Community Apps (også under **Indstillinger, Apps**), eller en af dine instanser, der allerede kan nås udefra, med **Fungere som videresender** slået til. Den instans svarer så på `/relay/connect` på sin egen adresse, bag den reverse proxy og det certifikat, den allerede har, og lukker kun din gruppe ind. Indtast relayets adresse på hver instans, der skal bruge det.
- **Ingen videresender**: medlemmerne finder automatisk hinanden på samme netværk, og ingen andre steder.

**Hvad relayet ser.** Hvert kald mellem medlemmer er forseglet med AES-256-GCM under en nøgle udledt af de tolv ord, og den nøgle forlader aldrig dine instanser. Relayet lærer en hash, der grupperer forbindelserne, hvilken instans en besked er til, hvor stor den er, og hvornår den passerer. Et direkte kald på det lokale netværk er forseglet på samme måde og desuden signeret, så intet afhænger af det selvsignerede certifikat, en instans serverer. Kald, der udleverer noget (rollesvar, en restic-adgangskode, modtagerlogins, nøgledeponeringer), signeres også med en nøgle fra det medlem, der sender dem, fastlåst ved første syn; se [Sikkerhedsmodel](configuration.md#security-model).

**Hvad der rejser over gruppen.** Beskyttelsesstatus på siden Instanser, en anmodning om at tjekke ét område nu, rolleanmodninger og svarene på dem, destinationstilbud og det, en modtager eller henter behøver, når dens rolle er tilladt: den anden instans' repositoriesteder og dens restic-adgangskode. Sikkerhedskopidata gør det aldrig; de går stadig direkte til restic-backends. Det gør APP_KEY heller ikke: restic-adgangskoden åbner den instans' repositories og intet andet, ikke dens gemte hemmeligheder, sessioner eller gendannelseskoder.

**Poster fra før parringen.** Instanser tilføjet med et flåde-token og modtagne repositories og hentekilder sat op med den anden instans' APP_KEY bliver efter opdateringen og er markeret **Par igen**. De bliver ved med at virke: ved første start erstatter BombVault hver gemt APP_KEY med den restic-adgangskode, der er udledt af den. Par begge instanser, redigér så posten og vælg dens instans. Indtil da står sådanne poster i kortet **Uden instans** på siden Instanser.

Det eneste sted, der stadig tager en APP_KEY i hånden, er [Fra en gammel sikkerhedskopimappe](#restore-from-another-bombvault-repo), til det tilfælde, at den anden instans er væk og ikke kan svare i en gruppe.

## Instanser og roller {#roles}

Siden **Instanser** (et indstillingsfelt, der kan fæstnes til sidepanelet) viser gruppen som et net af, hvem der gør hvad for hvem, et kort for denne instans og et kort for hvert andet medlem, Android-telefonerne og ZFS-serverne. **Par en instans** flyder nederst til højre. Hvilke dele der er slået til, sættes under **Indstillinger, Instanser, Flere servere**: **Instanser**, **Modtager** og **Hentning**.

Hvert kort viser medlemmets beskyttelsesstatus og dets roller som felter. En rolle er noget, en instans gør for en anden:

- **Modtager** opbevarer denne servers off-site-kopier i en rest-server i append-only-tilstand. Denne server sender efter hver sikkerhedskopi og kan ikke slette noget dér, selv om nogen overtager den.
- **Henter** henter denne servers sikkerhedskopier efter sin egen tidsplan. Denne server behøver ikke være vågen til det.
- **ZFS-server** tager imod denne servers ZFS-replikaer. Hvert ZFS-element vælger den under **Replika** i sine indstillinger.
- **Nøgleholder** opbevarer en forseglet kopi af denne servers gendannelseskit. Se [Nøgleholder](#key-keeper).

At slå en rolle til sender en anmodning, og intet starter, før nogen på den anden instans trykker på **Tillad**. Anmodningen vises på den instans' kort som et mærke, og dens **Detaljer** oplister den med **Tillad** og **Afvis**; en tilladt rolle kan tilbagekaldes dér når som helst. En stiplet linje i nettet er en rolle, der stadig venter. En instans på BombVault 9.x kan ikke svare på en anmodning, så en rolle er aktiv dér, så snart du slår den til, og en anmodning fra en 9.x-instans venter på **Tillad** her.

**Detaljer** på et kort åbner resten: det medlems modtagne repositories og hentekilder, destinationstilbud, overvågning, **Fjern** og **Glem nøglen**.

## Modtager og henter {#receiver}

![En modtaget off-site-kopi i en instans' detaljer, overvåget skrivebeskyttet, med et integritetstjek kørt på denne hardware.](assets/screenshots/receiver.png)

*En modtaget off-site-kopi i en instans' detaljer, overvåget skrivebeskyttet, med et integritetstjek kørt på denne hardware.*

På den boks, der **modtager** append-only-kopier fra en anden BombVault, giver de modtagne repositories dig uafhængig, skrivebeskyttet overvågning på modtagerhardwaren, så en stille fejl i den anden ende ikke går ubemærket hen. Slå **Modtager** til under **Indstillinger, Instanser**, åbn så den sendende instans' **Detaljer** på siden Instanser og tilføj det modtagne repository (skrivebeskyttet, åbnet med den sendende instans' restic-adgangskode, som den får over [parringsgruppen](#pairing)). Du får:

- **En beholdning efter kilde**, så du kan se præcis, hvilke containere, VM'er og mappesæt der er landet.
- **Sidst modtaget** pr. kilde, så du ved, hvor frisk hver er.
- **Et uafhængigt `restic check`** kørt på den modtagende hardware, så integritet verificeres, hvor data faktisk sidder, ikke kun på afsenderen.
- **En dødmandsknap:** en advarsel, når en kilde holder op med at sende inden for et vindue, du sætter.
- **Integritetsadvarsler:** en advarsel, når et tjek på den modtagende side fejler.

Modtageren er strengt skrivebeskyttet. Den skriver aldrig til det modtagne repository, så den kan aldrig bryde den append-only-garanti, afsenderen støtter sig til.

En henter virker den anden vej: med **Hentning** slået til læser en instans et andet medlems sikkerhedskopier ind i sit eget repository efter sin egen tidsplan. Dens kilder står også i det medlems detaljer.

### Modtageserver {#receiving-server}

Modtagerboksen kan også køre den rest-server, de andre kopierer til. Den sættes op første gang, du tillader en **Modtager**-anmodning, der beder om det: vinduet siger, at serveren har brug for en modtageserver, og beder om en mappe på en share, med **Ny mappe** til at oprette én, og en port (8000, medmindre en anden container bruger den). BombVault så:

1. afviser, hvis der allerede findes en container med navnet `rest-server`, eller en anden container holder porten;
2. henter `restic/rest-server` og starter den gennem Docker-socketten i append-only-tilstand med private repositories og en loginfil i mappen;
3. skriver dens Unraid-skabelon til flashdrevet, så containeren kan redigeres i fanen Docker, eller tilbyder skabelonen som download, når flashdrevet ikke kan nås;
4. kører manipulationstesten mod den og viser, om den afviser sletning.

Instanserne i din gruppe finder så serveren blandt udbyderne under **Fra din gruppe**, opkaldt efter modtagerboksen, og rollen **Modtager** tilbyder den som det sikre valg. Hver instans får et eget login, første gang den bruger serveren, og skriver kun i sin egen mappe dér. Med **Modtager** slået til under **Indstillinger, Instanser** åbner **Modtageserver** på denne instans' kort serveren igen. Dens vindue oplister loginsene under **Partnere med login**, og **Træk login tilbage** fjerner ét; det, den instans allerede har kopieret, bliver i mappen. Opsætningen opretter også ét **Login uden for gruppen**, hvis adgangskode vinduet viser én gang.

En instans, der kun når modtagerboksen gennem relayet, kan ikke bruge serveren, fordi relayet ikke bærer sikkerhedskopier. Tilføj først modtagerboksens adresse på parringssiden. Når BombVault kører på en egen IP-adresse (for eksempel på br0), udfyld **Adresse for partnere**, fordi serveren lytter på værtens adresse.

## Gennemgået eksempel: to Unraid-maskiner, hele vejen

Alt ovenfor beskriver delene. Dette er én komplet opsætning, fordi delene er lettere at samle, når man har set dem samlet én gang.

To bokse: **TOWER** kører containerne og sender sikkerhedskopier; **VAULT** modtager dem og håndhæver append-only. Indsæt dine egne navne, adresser og share-stier.

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

**5. Følg med i, hvad der kommer, hvis du vil.** Slå **Modtager** til under **Indstillinger, Instanser** på VAULT, åbn TOWERs **Detaljer** og tilføj det modtagne repository. VAULT får TOWERs restic-adgangskode over gruppen; ingen taster en nøgle.

**6. Gør det gensidigt.** Gentag trinnene i den anden retning: VAULT beder TOWER om at være dens modtager. Hver boks håndhæver så append-only for den anden, og ingen af dem kan slette den andens sikkerhedskopier.

En rest-server, du kører i hånden, virker også: tilføj den med **Tilføj destination** som en rest-server, med brugeren og adgangskoden fra dens `.htpasswd`, og slå sletningsbeskyttelse til. Behold `--append-only` i dens indstillinger; uden det er den anden side en almindelig share igen.

!!! warning "En lokal sti er en sti inde i containeren"
    Hvor BombVault beder om en mappe på denne server, indtast den relativt til værtsmonteringen, for eksempel `user/appdata/rest-server/bombvault-containers`, ikke `/mnt/user/appdata/...`. BombVault kører i en container, hvor værtens `/mnt` er monteret et andet sted; en absolut værtssti findes ikke inde i den. Hvis du indsætter en, fortæller BombVault dig den relative sti, du skal bruge i stedet.

## Nødguide {#emergency}

**Nødguiden** henter en server tilbage, der er væk eller blev sat op igen. Den starter på siden **Gendannelse** under **Nødsituation** og fra **Gendan fra en sikkerhedskopi** på velkomstsiden for en ny installation. Den kører i fire trin: **Adgang**, **Indstillinger**, **Valg** og **Gendannelse**.

1. **Adgang.** Vis BombVault nøglen til de gamle sikkerhedskopier, på en af tre måder:
    - **Upload gendannelseskit**: filen, du downloadede under opsætningen. Den rummer nøglen og hver destination, og BombVault udfylder resten ud fra den. Et kit forseglet med age beder om den identitet, det blev forseglet til; den bruges én gang og gemmes ikke.
    - **Via gruppen**: en instans i din gruppe opbevarer nøglen som deponering. Du skal bruge gruppens tolv ord, din frigivelseskode og en person ved den instans. Se [Nøgleholder](#key-keeper).
    - **Indtast nøglen**: den gamle servers `APP_KEY` og hvor dens sikkerhedskopier er.

    BombVault prøver så nøglen på hver destination og siger for hver, om den er læsbar, ikke fundet, låst med en anden nøgle, ikke til at nå eller stadig mangler sit login. Intet er gemt endnu, og nøglen bliver i hukommelsen i en begrænset tid. **Overtag nøglen** får denne server til at skrive sine sikkerhedskopier med den nøgle fra nu af; **Fortryd overtagelsen** går tilbage til dens egen nøgle, indtil den første sikkerhedskopi har kørt. En server, der allerede har sikkerhedskopier af sig selv, afviser overtagelsen og henviser til [Fra en gammel sikkerhedskopimappe](#restore-from-another-bombvault-repo) i stedet. **Gem adgangsoplysninger** gemmer de logins, kittet bærer til fjerne destinationer, og spørger, før det erstatter logins, denne server allerede har.
2. **Indstillinger.** **Hent indstillinger tilbage** gendanner BombVaults egne indstillinger fra den nyeste auto-sikkerhedskopi: destinationer, legitimationsoplysninger, tidsplaner og regler. BombVault genstarter for at anvende dem, gennem Docker-socket'en, så den levende indstillingsdatabase aldrig overskrives under et åbent håndtag, og guiden fortsætter af sig selv bagefter. Denne servers login bliver: du bliver ved med at logge ind med den adgangskode, du bruger nu. Hemmeligheder, den gamle server havde gemt, som ikke kan overtages, oplistes, så du kan indtaste dem igen.
3. **Valg.** BombVault gennemgår dine sikkerhedskopier og oplister de containere, VM'er, mappesæt og ZFS-datasæt, den finder. Den nyeste sikkerhedskopi er forvalgt overalt. Det, denne server stadig har, starter slået fra, fordi en gendannelse ville erstatte det. Mappesæt ved ikke længere, hvor de kom fra efter en genopbygning, så de går i en mappe, du vælger.
4. **Gendannelse.** Alt valgt kommer tilbage på én gang, efterladt stoppet, hvis du selv vil starte hver container og VM, hvilket er sikrest efter et totalt tab. Download bagefter et nyt gendannelseskit: det gamle hører til den forrige installation.

!!! tip "Planlagt migrering versus katastrofe"
    Nødguiden gendanner BombVaults egne indstillinger fra en sikkerhedskopi. Til en *planlagt* flytning til en ny boks kan du i stedet føre din konfiguration direkte over med kortet **Eksportér / importér indstillinger** (en bærbar JSON-fil). Se [Konfiguration](configuration.md#portable-settings-export-and-import).

### Fra en gammel sikkerhedskopimappe {#restore-from-another-bombvault-repo}

**Fra en gammel sikkerhedskopimappe** under **Andre kilder** på siden Gendannelse åbner en *anden* BombVault-instans' repository (en share monteret under `/mnt` eller en fjernadresse) med den instans' gendannelseskit eller dens indtastede `APP_KEY`, i en engangssession, skrivebeskyttet. Gennemse de containere, VM'er, mappesæt og ZFS-datasæt, der er gemt dér, vælg en sikkerhedskopi og gendan den, og det gendannede objekt bliver en almindelig lokal container, VM eller et almindeligt mappesæt. Der skrives aldrig noget til det andet repository, og dine egne backupindstillinger forbliver urørte (sessionen lever i hukommelsen og udløber af sig selv). At flytte en container fra server A til server B betyder ikke at omdirigere dine indstillinger og vende dem tilbage bagefter. Dette vindue er et engangsvindue: det åbner en session, gendanner det, du vælger, og glemmer den anden instans. Hvis du i stedet vil have en stående ordning, hvor denne boks efter en tidsplan henter en anden instans' sikkerhedskopier ind i sit eget repository, er det rollen **Henter** på siden Instanser.

En container, hvis netværk ikke findes på denne server, for eksempel et Unraid-`br0`-netværk på en almindelig Docker-vært, viser en netværksvælger under sin række. BombVault opretter den på det netværk, du vælger, sammen med dens andre netværk. Den faste IP- og MAC-adresse hørte til det gamle netværk og falder væk, så det nye netværk tildeler dem.

## Nøgleholder {#key-keeper}

En nøgleholder er en instans i din gruppe, der opbevarer en forseglet kopi af denne servers gendannelseskit. Den kan ikke selv åbne kopien. Hvis denne server nogensinde går tabt, beder en ny installation, der slutter sig til din gruppe, holderen om kopien og åbner den med din frigivelseskode.

1. På siden Instanser slår du **Nøgleholder** til på kortet for den instans, der skal opbevare den, og vælger **Deponer**. At deponere kræver en login-adgangskode.
2. Denne server viser **Frigivelseskode** én gang: tyve bogstaver og cifre i fire grupper af fem. Skriv den ned og bekræft med **Jeg har skrevet koden ned**. Denne server gemmer ikke koden, kun det, den skal bruge til at forsegle en deponering til den.
3. Holderen har nu deponeringen, og denne server fornyer den, når kittet ændres. **Nøgledeponering** på denne instans' kort oplister **Denne servers deponering** hos hver holder og dens tilstand, og **Deponeringer, som denne server opbevarer** for de andre. **Lav en ny kode** erstatter koden; hver holder skifter til den nye kode, når den får den nye deponering.

Den dag skal du bruge tre ting: din gruppes tolv ord, frigivelseskoden og adgang til holderens grænseflade, hvor nogen skal trykke på **Tillad**. I Nødguiden vælger du **Via gruppen**, vælger deponeringen, beder om den og taster frigivelseskoden. Koden bruges på den nye server og sendes ikke til holderen. Hos holderen vises anmodningen under **Nøgledeponering** med, hvem der spørger, om den instans nogensinde er set dér før, og hvor ofte den har spurgt; **Tillad** udleverer den forseglede kopi, og tilladelsen kræver en login-adgangskode. Den nye installation har 30 minutter til at hente den efter Tillad, en anmodning, ingen besvarer, udløber efter et døgn, og en instans, der spørger for ofte, må vente en time.

Uden koden kan ingen åbne kopien, heller ikke holderen. Hvis koden går tabt, lav en ny, mens denne server stadig kører; hvis serveren allerede er væk, virker gendannelseskittet og den indtastede APP_KEY stadig. Holderen kan til enhver tid slette en deponering, og ejeren må så spørge igen.

## Gendannelseskit {#recovery-kit}

Dette er den brik, der gør katastrofegendannelse mulig, selv når der ikke er nogen kørende BombVault.

**Download gendannelseskit** i kortet **Til nødsituationer** under **Indstillinger, Gendannelse** giver dig **nøglen**, det **afledte restic-kodeord**, hver **destination** med repositoriet for hvert område, de logins, en ny installation har brug for for at nå dem, og de **præcise restic-kommandoer**, så du kan gendanne direkte med restic CLI på en hvilken som helst maskine. Det kræver en login-adgangskode. Kittet udskriver også nøglen med kryptering slået fra, fordi container- og VM-definitionerne altid er forseglet med den. Efter en overtagelse udskriver det den overtagne sikkerhedskopinøgle og denne servers APP_KEY, som en ny installation kun skal bruge for at hente indstillingerne tilbage. Et kort **Gem dit gendannelseskit** på oversigten minder dig om det, indtil du har gemt det.

!!! danger "Opbevar gendannelseskittet uden for serveren"
    Kittet indeholder den hemmelighed, der dekrypterer dine sikkerhedskopier. Opbevar det et sikkert sted adskilt fra serveren (en adgangskodemanager, en udskrift i et pengeskab). Mister du både BombVault og `APP_KEY` uden gendannelseskit og uden nøgledeponering, kan dine krypterede sikkerhedskopier ikke gendannes.

!!! warning "Den nyeste sikkerhedskopi er ikke altid den, der skal gendannes"
    Siden restic 0.17 viser `restic snapshots` hvert snapshots størrelse. Efter datatab kan den nyeste sikkerhedskopi være den tømte, så gendan ikke en sikkerhedskopi, der er langt mindre end dem før den. Efter ransomware kan den være den krypterede i den sædvanlige størrelse. Hvis BombVault stadig kører, så tjek først siden **Afvigelser**: den navngiver den sidste gode sikkerhedskopi, og gendannelsesvinduet forvælger den. En gendannelse behøver ingen af BombVaults afvigelsesdata, og opbevaringspausen beholder altid kun flere sikkerhedskopier.

### Forsegling af kittet

Hvis du har slået **Krypter eksporter med age** til under **Indstillinger, System**, er kittet forseglet med det også og downloades som `bombvault-recovery-kit.md.age`. Det er ASCII-armeret frem for binært, så det er stadig klartekst: at indsætte det i en adgangskodemanager eller udskrive det virker præcis som før, indholdet er bare ulæseligt uden din nøgle.

!!! warning "Opbevar ikke age-nøglen i kittet"
    Du skal bruge din **private** age-nøgle for at åbne et forseglet kit. Opbevar den et sted, der ikke afhænger af selve kittet, ellers har du to ting at gendanne i stedet for én. Forsegling er umagen værd, når kittet ligger et sted, du ikke selv har fuld kontrol over (en delt adgangskodemanager, noter i skyen, en udskrift på et kontor); et kit i dit eget pengeskab er allerede beskyttet af pengeskabet.

    Med kryptering slået til og ingen brugbar modtager sat op nægtes downloadet helt. BombVault falder aldrig tilbage til at udlevere hovednøglen i klartekst.

### Når sættet ikke er ved hånden

Adgangskoden er ikke gemt nogen steder, den **beregnes** ud fra sikkerhedskopiernes nøgle, så du selv kan gengive den med intet andet end nøglen og en shell:

```sh
printf 'bombvault:restic-repo'   | openssl dgst -sha256 -mac HMAC -macopt hexkey:$APP_KEY -r   | cut -d' ' -f1
```

Det er HMAC-SHA256 over den faste streng `bombvault:restic-repo`, med de rå bytes af den hexadecimale `APP_KEY` som nøgle, udskrevet som 64 små hexadecimale tegn. Den samme værdi står i kittet, oplistet som det afledte restic-kodeord; dette er til den dag, kittet er et sted, hvor du ikke er. På en server, der overtog en andens sikkerhedskopier, bruges den overtagne sikkerhedskopinøgle, som er den servers gamle APP_KEY.

!!! warning "Brug den AFSENDENDE instans' nøgle ved et modtaget arkiv"
    Et repository, der kom hertil gennem en off-site-kopi, blev oprettet af maskinen, der sendte det, med **dens** nøgle. At udlede fra modtagerboksens nøgle giver en adgangskode, restic afviser, hvilket ser nøjagtig ud som et beskadiget repository og ikke er det. Det er den sædvanlige grund til, at `restic check` på et modtaget repo bliver ved med at bede om en adgangskode.

Fordi gendannelsesdefinitioner lever **inde** i hvert repo (`<repo>/def`, `<repo>/vm-def`), er en kopieret repo-mappe fuldt selvstændig, så kittet plus repoet er alt, hvad en bare-metal-gendannelse har brug for.

## Hent et databasedump tilbage {#database-dumps}

Et databasedump er et gendannelsespunkt for sig i containerrepositoriet, mærket `dbdump:<container>`, med den ene fil `/dbdump/<container>.sql`. BombVault oplister, downloader og importerer dem under containerens fane **Sikkerhedskopier**; nedenfor er de samme trin med restic alene, til den dag BombVault ikke er der.

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
