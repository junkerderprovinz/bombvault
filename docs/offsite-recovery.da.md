# Off-site og gendannelse

!!! note "Eksterne kopier venter efter en genopbygning"
    Når trin 4 genopbygger elementer uden de gamle indstillinger, sættes off-site-replikeringen af de domæner på pause, indtil standardplaceringen er bekræftet. Se [Placering pr. element](#placement).

Lokale sikkerhedskopier beskytter dig mod en tabt container eller en dårlig opdatering. Off-site-replikering og et testet gendannelseskit beskytter dig mod hele boksen, ransomware eller en brand. Denne side dækker replikering off-site, at gøre den kopi manipulationssikker, at bevise at du kan gendanne, og at gendanne, når BombVault selv er væk.

## Off-site-replikering

Behold den hurtige lokale sikkerhedskopi, og tilføj en eller flere off-site-replikaer. Sæt et repo pr. domæne på siden **Indstillinger, Off-site**. BombVault replikerer nye øjebliksbilleder dertil med `restic copy` på et best-effort-grundlag, så et off-site-hikke aldrig får den lokale sikkerhedskopi til at fejle. I denne form forbliver det lokale repo primært, og off-site-repoet er en replika, men et domænes primære repo behøver slet ikke at være lokalt; se [Fjernbetjente primære arkiver](#remote-primary-repositories) nedenfor for at sikkerhedskopiere direkte til S3, en rest-server osv. i stedet for at replikere dertil.

- **Flere off-site-destinationer pr. domæne.** Hvert domæne (containere, VM'er, flash, config, filsæt og ZFS-datasæt) kan replikere til flere off-site-destinationer på én gang, ikke kun én, så du kan beholde for eksempel en rest-server på en vens boks og en S3-bucket parallelt. Tilføj ekstra destinationer på Indstillinger, Off-site, hver med sit eget repository, sin S3-lagringsklasse, sit append-only-flag, sin opbevaring og sit vækstbudget. En eksisterende enkelt off-site-opsætning overføres urørt som den første destination, og hver destination i et domæne replikerer på det domænes off-site-tidsplan.
- **Off-site-tidsplan pr. domæne** (redigeret sammen med alle andre tidsplaner på Indstillinger, Tidsplaner): lad den stå tom for at replikere efter hver lokal sikkerhedskopi, eller sæt en kadence (for eksempel `weekly Sun 03:00`) for at sende off-site sjældnere, end du sikkerhedskopierer lokalt. En **Replikér nu**-knap dækker on-demand-kørsler.
- **Off-site-opbevaring** lever på Indstillinger, Opbevaring, så du kan beholde off-site-kopier længere som et arkiv. Lad politikken stå helt-nul for aldrig at auto-trimme off-site-øjebliksbilleder.
- **Båndbreddegrænser** (Indstillinger, Off-site) begrænser restic-upload/download-hastigheden, så replikering ikke mætter dit WAN.
- En **replikeringsindikator** viser, hvilket domæne der replikerer, mens det kører (på dets side og på Oversigten). Det er en aktiv indikator, ikke en procentbjælke, fordi `restic copy` ikke eksponerer nogen maskinlæsbar fremdrift.

!!! note "Gendan fra ethvert sted"
    Hver container, VM, mappesæt, flashen og appkonfigurationen lister deres sikkerhedskopier som én tidslinje på tværs af alle steder, en sikkerhedskopi ligger. En sikkerhedskopi kopieret til B2 vises én gang, markeret med hvert sted, der holder den. En gendannelse tager det første sted, den kan nå, startende med det arkiv, elementet skrives til, og du kan vælge et andet sted pr. række. Eksterne steder læses kun, når du åbner dem. At slette ét sted tjekker først de andre og siger, om det var den sidste kopi.

## Destinationer {#destinations}

Indstillinger, Off-site starter med **Destinationer**: de steder, off-site-kopierne havner, oprettet én gang for alle domæner. En destination vises derefter som en knap i rækken **Placering** for hvert domæne og hvert element. Første gang den afkrydses for et domæne, opretter BombVault domænets repository i en mappe under den, for eksempel `rclone:onedrive:BombVault/containers`.

**Tilføj destination** åbner en guide i fem trin:

1. **Hvor skal sikkerhedskopierne hen?** Hver tjeneste vises med sit logo i fire grupper: lagringstjenester med S3-buckets (Backblaze B2, Wasabi, Cloudflare R2, Hetzner Object Storage, Amazon S3 og flere), din egen S3-server (Garage, SeaweedFS, RustFS, Silo, Ceph, JuiceFS, Versity S3 Gateway), din egen server og delinger (rest-server, Hetzner Storage Box, SFTP, SMB, WebDAV, en monteret sti) og cloud-lagring (OneDrive, Google Drive, Dropbox, pCloud, Nextcloud og resten, som rclone understøtter). Hver af dem siger, hvor godt den egner sig til sikkerhedskopier: cloud-drev bremser ved mange forespørgsler, så den første sikkerhedskopi og prune tager længere tid der.
2. **Log ind på** tjenesten. Felterne afhænger af tjenesten: en adgangsnøgle til S3, et brugernavn og en adgangskode til WebDAV og SMB, en app-adgangskode, hvor tofaktorlogin blokerer den normale, BombVaults offentlige SSH-nøgle til SFTP og Storage Box, eller et token til tjenester, der logger ind via en browser. Til dem viser guiden en `rclone authorize`-kommando, som køres på en computer med en browser; det token, den udskriver, indsættes i feltet. **Test forbindelse** kontrollerer loginet, før noget gemmes.
3. **Vælg en mappe.** Guiden viser mapperne på destinationen, med **Ny mappe** til at oprette en og den ledige plads, hvor tjenesten oplyser den. En tom mappe er sikrest.
4. **Beskyttelse mod sletning.** Guiden siger ligeud, hvad tjenesten kan. En rest-server i append-only-tilstand afviser sletning, og manipulationstesten kontrollerer det. En S3-bucket kan beholde gamle versioner med versionering og object lock, hvilket BombVault endnu ikke kan kontrollere. Et cloud-drev kan slet ikke afvise sletning: den, der kommer ind på serveren, kommer også ind i den kopi. Slå **Uforanderlig (append-only)** til kun, hvor fjernsiden reelt afviser sletning; BombVault pruner så aldrig dér.
5. **I en nødsituation.** Gendannelsespakken viser hver destination med repositoryet for hvert domæne under den. Loginet kommer tilbage med BombVaults indstillingssikkerhedskopi; ved en ny installation uden den opsættes destinationen igen på samme sted.

S3-tjenester kører gennem restics eget S3-backend, og det er det, der gør, at en lagringsklasse og object lock kan bruges. Alle andre tjenester kører gennem den rclone, BombVault leveres med, og deres remote vises derefter i rclone-konfigurationen under Indstillinger, Cloud-adgang. En eksport af indstillingerne indeholder destinationerne; med legitimationsoplysninger inkluderet indeholder den også deres login.

En modtageserver, som en anden instans i din gruppe kører, vises i guiden under **Fra din gruppe**; se [Modtageserver](#receiving-server).

Et domænes mål, der er oprettet ud fra en destination, overtager destinationens navn, placering, legitimationsoplysninger, lagringsklasse og kontakten Uforanderlig. Opbevaring, komprimering og vækstbudget forbliver pr. domæne, og placeringen kan ikke flyttes, fordi domænets repository ligger dér. **Tilføj et mål kun til dette domæne** under hvert domæne tager stadig en håndskrevet repository-URL.

Et håndskrevet mål, hvis repository ligger i en mappe under en destination, kan blive en del af den. Destinationen viser sådanne mål under **Ligger allerede under denne destination**, og **Overtag** hænger et på den. Målet beholder sit repository, sine snapshots, sin opbevaring og sin placering og overtager destinationens navn, legitimationsoplysninger, lagringsklasse og kontakten Uforanderlig. BombVault kontrollerer først, at destinationens login åbner repositoryet, og nægter at lægge et append-only-mål under en destination, der ikke er append-only. Overtager du et domænes primære mål, bliver domænets off-site-felt tømt.

## Placering pr. element {#placement}

Hvert container-, VM- og mappesæt-kort har en række knapper under **Placering**: **Lokal** og én knap pr. off-site-mål i domænet, efterfulgt af de destinationer, domænet endnu ikke har noget mål under. Tændte knapper får elementets sikkerhedskopier.

- Med **Lokal** tændt skrives elementet til det arkiv, der vises under **Gemt på**, og kopieres til hvert andet tændt mål. Sluk et mål, og det får ikke længere noget nyt fra dette element. Lokal alene kopierer ingen steder, hvilket passer til data, der allerede har en anden kopi, for eksempel en deling, der ligger på et NAS.
- Med **Lokal** slukket skrives elementet direkte til det første tændte måls direkte arkiv og kopieres derfra til de andre tændte mål. Første gang opretter en dialog det direkte arkiv.
- En destinationsknap opretter domænets mål under destinationen og tænder det kun for dette element. Alle andre elementer starter uden kopi dér.
- Én knap forbliver tændt, fordi en sikkerhedskopi skal have et sted at gå hen. Vil du holde noget uden for sikkerhedskopierne, så udeluk det.

Placeringen er fast fra elementets første sikkerhedskopi, fordi BombVault aldrig flytter sikkerhedskopier mellem arkiver. Kopierne kan ændres når som helst. En destination, der ikke længere får et element, beholder de kopier, den har, og beskærer dem til sin egen opbevaring ved domænets næste off-site-kørsel; **Slet i B2** på kortet fjerner dem med det samme. Findes nogle af de kopier ingen andre steder, lister bekræftelsen dem efter dato og spørger om elementets navn. Der kan ikke slettes fra append-only-destinationer.

Under rækken viser kortet, hvor elementet går hen, og hvad der faktisk er der: hvor mange lokationer det holdes på, hvornår hver destination sidst blev set, og om 3-2-1 er opfyldt. En lokation er serveren med de originale data, hver off-site-destination og hvert arkiv markeret **Uden for bygningen**. BombVault tjekker kopier og lokationer; det tjekker ikke "to medier"-delen af 3-2-1.

### Standardplaceringer

Indstillinger, Lagring, **Standardplaceringer** har en række pr. domæne med de samme knapper. Kopierne gælder med det samme for hvert element uden eget valg, og for projektmapperne i Compose-stacks. Placeringen gælder for et nyt element ved dets første sikkerhedskopi; at ændre den flytter ingen sikkerhedskopier. Før den gemmes, navngiver rækken hver destination, der vinder eller mister elementer, og hvor mange øjebliksbilleder det betyder. **Anvend på elementer uden sikkerhedskopier** sætter hvert element uden nogen sikkerhedskopi endnu tilbage på standarden.

En ny off-site-destination modtager hvert element, der ikke er sat til Lokal. Dialogen, der tilføjer den, angiver, hvor mange elementer det er, og hvor det er kendt, hvor meget historik det udgør, og tilbyder at udelade de elementer, der allerede er udelukket fra andre destinationer.

### Direkte arkiver

At slå Lokal fra for et element, så et mål uden direkte arkiv bliver dets hjem, åbner en dialog med en foreslået placering ved siden af målet, for eksempel `s3:https://s3.eu-central-003.backblazeb2.com/bucket/containers-direct`, og en forbindelsestest, der ikke opretter noget. **Opret og brug** opretter arkivet og peger elementet på det. Et direkte arkiv overtager destinationens nøgle, lagringsklasse, grænser, append-only-indstilling og opbevaring og ændrer sig med dem; Depoter-kortet viser det skrivebeskyttet. Kan en ny nøgle til destinationen ikke åbne det, beholder det direkte arkiv den nøgle, det har, og det noteres ved gemning. Et element på et direkte arkiv kopieres derfra til de andre tændte mål, aldrig til det mål, arkivet hører til. Dets øjebliksbilleder bærer mærket `bv:direct`, og alle andre opbevaringskørsler lader dem stå, så et direkte arkiv, der har mistet forbindelsen til sin destination, aldrig ældes efter de lokale regler. B2 nås via dets S3-endpoint, hvor nøgle-id'et og applikationsnøglen angives som S3-legitimationsoplysningerne; en nøgle, der er begrænset til destinationens egen mappe, kan ikke nå mappen ved siden af den, så begræns i stedet nøglen til mappen over destinationen.

### Uden for bygningen

Et navngivet arkiv kan markeres **Uden for bygningen** på Depoter-kortet. Fjernarkiver starter markeret; slå det fra for en rest-server i samme bygning. Markeringen tæller kun med i lokationer og 3-2-1 på kortene. Den ændrer ingen kopi.

### Efter en genopbygning

Kopivalg lever i BombVaults egne indstillinger. Efter en genopbygning via Opdag sikkerhedskopier uden et gendannet `/config` er de væk, og at kopiere alt ville sende de elementer, du havde udeladt, til B2 igen. Off-site-replikeringen af hvert genopbygget domæne sættes derfor på pause. Oversigten viser det i gult, og Standardplaceringer tilbyder **Bekræft standard** med en forhåndsvisning af, hvad næste kørsel kopierer, og navnene i de sikkerhedskopier, der ikke har noget element, som du kan udelade der. Kun bekræftelsen afslutter pausen; at importere en indstillingsfil bringer regler og standarder tilbage, men afslutter ikke pausen.

## Fjernbetjente primære arkiver {#remote-primary-repositories}

Et domænes sti til sikkerhedskopi (Indstillinger, Lagring) er ikke begrænset til en lokal mappe: peg den direkte på et restic-fjernarkiv (`s3:...`, `rest:http://vært:8000/arkiv`, `sftp:bruger@vært:/arkiv`, `rclone:fjern:bucket/sti`), så sikkerhedskopierer BombVault direkte dertil, uden separat lokal kopi og uden replikeringstrin. Det er en virkelig anden form end off-site-replikeringen ovenfor: dér er det lokale arkiv det primære, og off-site-arkivet er et arkiv af det efter bedste evne; her **er** fjernarkivet det primære, og det er den eneste kopi, så længe du ikke også opsætter off-site-replikering (eller et andet fjernarkiv) for det domæne.

Hvert af de seks stifelter (Containers, VMs, Flash, Auto-sikkerhedskopi, Mapper, ZFS-datasæt) har en kontakt **Lokal / Fjern** lige ved siden af:

- **Lokal** viser den velkendte mappebrowser.
- **Fjern** bytter den ud med et almindeligt URL-felt plus en knap, der åbner den samme dialog til forbindelsestest og adgangsoplysninger, som off-site-destinationer bruger, blot indstillet til dette primære arkiv. Derfra får du:
    - **En forbindelsestest** mod den rigtige sti, før du forlader dig på den.
    - **Båndbreddegrænser** (upload og download), så en planlagt sikkerhedskopi til et fjernprimært arkiv ikke mætter din WAN-forbindelse: de samme restic-flag `--limit-upload` og `--limit-download`, som off-site-replikeringen bruger, nu anvendt på selve sikkerhedskopien.
    - **Append-only-beskyttelse (uforanderlighed)**, efterprøvet med den samme aktive manipulationstest (en rigtig DELETE-sonde mod den anden ende), som off-site-destinationer får. Er den slået til, nægter BombVault selv at beskære arkivet: da der ikke står en separat lokal kopi bag, må adgangsoplysningerne på denne maskine ikke kunne slette sikkerhedskopiens eneste kopi.
    - **En alarm for vækstbudgettet**, taget fra den samme udvikling i arkivets størrelse, som Lagerkortet allerede følger.

Intet af dette er påkrævet: en håndskrevet fjernsti uden gemte sikkerhedsindstillinger sikkerhedskopierer nøjagtig som før (ubegrænset båndbredde, kan beskæres, ingen budgetalarm). Sikkerhedsdialogen er der til, når du vil have den samme beskyttelse, som en off-site-kopi får, uden at skulle oprette en off-site-destination alene af den grund.

!!! note "Sky- og REST-adgangsoplysninger deles"
    Et fjernprimært arkiv godkendes med de samme S3-/REST-adgangsoplysninger, der er sat op under Indstillinger, Cloud-adgang, Delte cloud-legitimationsoplysninger. Der findes ikke et separat sted til adgangsoplysninger for primære arkiver.

### SMB og WebDAV uden værtsmontering {#smb-webdav}

Indstillinger, Cloud-adgang, rclone har en formular til en Windows- eller Samba-share og til en WebDAV-server (Nextcloud, ownCloud, SharePoint eller en anden). Udfyld et kort navn, værten og sharen (SMB) eller URL'en og servertypen (WebDAV), brugeren og adgangskoden, så skriver BombVault rclone-sektionen for dig. rclone slører selv adgangskoden, før den gemmes; tilføjer du en destination med et navn, der allerede findes, erstatter den den sektion i stedet for at tilføje en ekstra.

Formularen svarer med den færdige placering, for eksempel `rclone:nas:backups`. Sæt den ind i en sikkerhedskopisti eller en off-site-destination, og tilføj en undermappe, hvis du vil have en (`rclone:nas:backups/bombvault`). Sharen er det første led i stien, ikke en del af navnet.

Det er en bedre vej end at montere sharen på Unraid: restic fraråder at have et repository på en monteret CIFS-share, og her monteres intet. NFS er ikke med i formularen, fordi hverken restic eller rclone har en NFS-backend; til NFS monterer du eksporten på værten og peger en sikkerhedskopisti mod den.

## Uforanderlig (append-only) off-site

Flag et off-site-repo append-only, så ransomware eller en kompromitteret vært ikke kan slette eller omskrive dine sikkerhedskopier. Den anden side (en `restic/rest-server`, der kører i `--append-only`-tilstand) **håndhæver** det. BombVault **verificerer** det kun altid og viser aldrig grønt alene på en konfigurationspåstand.

Guiden til **guidet off-site-opsætning** fører dig fra backend-valg (rest-server / rclone / S3) gennem et klar-til-indsæt rest-server-deploy-snippet, en forbindelsestest, uforanderligheds-omskifteren (som kører manipulationstesten med det samme) og en opbevaringsstrategi, så append-only off-site er tilgængelig uden manuel redigering af configs.

!!! note "En vellykket sletning under `/locks/` er forventet"
    Append-only betyder ikke, at intet længere kan slettes. restic skal tage og frigive sine egne låse, så `/locks/` forbliver bevidst skrivbar og sletbar. Snapshots og dataene bag dem, altså præcis det ransomware ville gå efter, kan ikke fjernes. Tester du selv modparten, er en sletning der lykkes under `/locks/` korrekt adfærd og ikke et hul i beskyttelsen.

!!! warning "Uforanderlige repos beskæres aldrig fra denne boks"
    En uforanderlig off-site beskærer bevidst aldrig gamle øjebliksbilleder. Sæt en **vækstbudget-alarm** for den, så du bliver adviseret, før repo-størrelsen løber løbsk.

## Manipulationstest

BombVault beviser periodisk append-only-garantien ved faktisk at forsøge en sletning mod off-site-repoet, rettet mod et ikke-eksisterende objekt:

- **Afvist** betyder beskyttet.
- **Accepteret** betyder ikke beskyttet.
- Et **inkonklusivt** resultat (server uopnåelig, autentificeringsfejl) vender aldrig den gemte dom.

En reel beskyttet-til-ubeskyttet-vending udløser én enkelt advarsel.

## DR-øvelser

BombVault tilbyder to niveauer af bevis for, at dine sikkerhedskopier faktisk kan gendannes, ikke bare er til stede.

- **Gendannelses-verifikationsøvelser (lokal).** BombVault kører periodisk `restic check --read-data-subset` (afgrænset, aldrig en disk-fyldende fuld gendannelse) og viser et *Verificeret gendannelig*-badge pr. domæne. Kadencen lever på Indstillinger, Tidsplaner; badge't på Indstillinger, Integritet.
- **DR-øvelser (off-site).** BombVault gendanner et rigtigt mål fra off-site-repoet ind i en engangs-sandkasse, verificerer det fil-for-fil og byte-for-byte, og rydder så op. Dette beviser, at du kan gendanne fra off-site, ikke bare at repoet svarer.

**Ransomware-beskyttelses-scorekortet** på Oversigten samler dette til en grøn / gul / rød position pr. domæne, med en aldersstemplet tjekliste (off-site konfigureret, append-only verificeret, replikering aktuel, gendannelsesøvelse bestået, kryptering til, beskæringsstrategi sat). Hver rød række dyb-linker til rettelsen, og kortet bliver kun nogensinde grønt på verificerede fakta.

## Parring af instanser {#pairing}

Modtagere, hentekilder, Instanser-siden og Mesh-off-site taler alle med en anden BombVault. De gør det som medlemmer af én parringsgruppe, og en instans kommer med i gruppen med tolv ord.

Åbn **Indstillinger → Parring** på den første instans, og klik på **Generer sætning** i parringskortene. Der dukker tolv ord op i et vindue med en **Kopier**-knap. Åbn det samme sted på hver af de andre instanser, klik på **Indtast sætning**, og indsæt eller skriv dem, eller klik på **Indsæt** i det vindue. Et ord, der ikke findes på listen, bliver nævnt med sin placering, mens du skriver, og det sidste ord bærer et tjeksum, så et forkert tastet eller byttet om ord bliver opdaget, før noget parres. Generer sætningen på kun én instans: to instanser, der begge opretter en sætning, danner to adskilte grupper. Melder ingen sig i løbet af et minut, tilbyder fanen to veje ud: vis ordene igen for at indtaste dem derovre, eller indtast den anden instans' ord og bliv medlem af dens gruppe i ét trin. Parring virker uden en loginadgangskode, men sæt en: uden den kan alle, der kan åbne denne brugerflade, læse ordene og via gruppen få fat i restic-adgangskoden til hver instans i den. Parringskortet siger det, indtil der er sat en adgangskode. Med en adgangskode beder en fornyet visning af sætningen om den. **Forlad gruppen** tager en instans ud igen.

Enhver, der kender ordene, kan komme med i gruppen, så behandl dem som en adgangskode.

**Sådan finder medlemmerne hinanden.** Hver instans lærer sin egen adresse på netværket fra din browser, i det øjeblik du logger ind, vist i videresenderkortet som **Denne instans på dit netværk**; ret den der, hvis en reverse proxy eller en usædvanlig port sidder foran. På samme netværk annoncerer medlemmerne den adresse via multicast og taler direkte sammen, og hvor multicast ikke kan krydse et containernetværk, som Dockers standard-bridge-netværk, søger en instans i stedet sit eget subnet igennem efter de andre med et signeret kald, som kun et gruppemedlem kan besvare, så parringen stadig er færdig på sekunder uden en videresender. Dukker der ikke noget op, tager **Kan du ikke finde den?** under parringskortet imod én adresse i hånden, til et andet subnet eller en ikke-standard port. Instanser på forskellige netværk går gennem en videresender, som vælges på den samme fane:

- **Projektets videresender** (standard): `parleyport.halleluja.design`, den samme videresender som KnightLoader også bruger. Intet at sætte op.
- **Egen videresender**: containeren [**ParleyPort**](https://github.com/junkerderprovinz/parleyport) fra Unraid Community Apps, eller en af dine instanser, der allerede er tilgængelig udefra, med **Fungere som videresender** slået til. Den instans svarer så på `/relay/connect` på sin egen adresse, bag den reverse proxy og det certifikat, den allerede har, og lukker kun din gruppe ind. Indtast videresenderens adresse på hver instans, der skal bruge den.
- **Ingen videresender**: medlemmerne finder automatisk hinanden på samme netværk, og ingen andre steder.

**Hvad videresenderen ser.** Hvert opkald mellem medlemmer er forseglet med AES-256-GCM under en nøgle udledt af de tolv ord, og den nøgle forlader aldrig dine instanser. Videresenderen lærer et hash, der grupperer forbindelserne, hvilken instans en besked er til, hvor stor den er, og hvornår den passerer. Et direkte opkald på det lokale netværk er forseglet på samme måde og signeret oveni, så intet afhænger af det selvsignerede certifikat, en instans udsteder.

**Hvad der rejser over gruppen.** Scorekortene på Instanser-siden, en anmodning om at tjekke en domæne nu, Mesh-off-site-tilbud, og det, en modtager eller hentekilde har brug for: den anden instans' repository-placeringer og dens restic-adgangskode. Sikkerhedskopidata gør det aldrig; de går stadig direkte til restic-backendene. Det gør APP_KEY heller ikke: restic-adgangskoden åbner den instans' repositorier og intet andet, hverken dens gemte hemmeligheder, sessioner eller gendannelseskoder.

**Poster fra før parring.** Instanser tilføjet med et fleet-token, samt modtagere og hentekilder sat op med den anden instans' APP_KEY, forbliver efter opdateringen og er markeret **Par igen**. Modtagere og hentekilder bliver ved med at virke: ved sin første start erstatter BombVault hver gemt APP_KEY med den restic-adgangskode, der udledes af den. Par begge instanser, og redigér så posten og vælg dens instans. En sådan instans overtager sit gamle kort, så snart en instans med samme navn dukker op i gruppen.

Det eneste sted, der stadig tager imod en APP_KEY manuelt, er [Gendan fra et andet BombVault-repo](#restore-from-another-bombvault-repo), til det tilfælde, hvor den anden instans er væk og ikke kan svare i en gruppe.

## Modtager-dashboard (den modtagende side)

![Den modtagende side, overvåget skrivebeskyttet, med en integritetskontrol kørt på denne maskine.](assets/screenshots/receiver.png)

*Den modtagende side, overvåget skrivebeskyttet, med en integritetskontrol kørt på denne maskine.*

Alt ovenstående er den *afsendende* side. På den boks, der **modtager** uforanderlige off-site-kopier fra en anden BombVault, giver modtager-dashboardet dig uafhængig, skrivebeskyttet overvågning af disse repositorier på den modtagende hardware, så en tavs fejl i den anden ende ikke går ubemærket hen.

Slå **Modtager**-omskifteren til i Indstillinger for at afsløre en **Modtager**-fane. Den er som standard fra; aktivér den kun på en boks, der faktisk modtager uforanderlige off-site-sikkerhedskopier. Registrer så et modtaget repository (skrivebeskyttet, åbnet med den afsendende instans' restic-adgangskode, som den får over [parringsgruppen](#pairing)) for at få:

- **Et øjebliksbillede-inventar grupperet efter kilde**, så du kan se præcis, hvilke containere, VM'er og filsæt der er landet.
- **Sidst-modtaget** pr. kilde, så du ved, hvor frisk hver enkelt er.
- **Et uafhængigt `restic check`** kørt på den modtagende hardware, så integritet verificeres, hvor data faktisk sidder, ikke kun på afsenderen.
- **En dødmandsknap:** en advarsel, når en kilde holder op med at sende inden for et vindue, du sætter.
- **Integritetsadvarsler:** en advarsel, når et tjek på den modtagende side fejler.

Modtageren er strengt skrivebeskyttet. Den skriver aldrig til det modtagne repository, så den kan aldrig bryde append-only-garantien, afsenderen forlader sig på.

### Modtageserver {#receiving-server}

Den modtagende boks kan også køre den rest-server, de andre kopierer til. **Opsæt modtageserver** øverst i fanen **Modtager** spørger efter en mappe på et share, med **Ny mappe** til at oprette en, og en port (8000, medmindre en anden container bruger den). BombVault gør så følgende:

1. afviser, hvis der allerede findes en container med navnet `rest-server`, eller en anden container holder porten;
2. henter `restic/rest-server` og starter den gennem Docker-socketten i append-only-tilstand med private repositories og en loginfil i mappen;
3. skriver dens Unraid-skabelon til flashdrevet, så containeren kan redigeres i fanen Docker, eller tilbyder skabelonen som download, når flashdrevet ikke kan nås;
4. kører manipulationstesten mod den og viser, om den afviser sletning.

Instanserne i din gruppe finder så serveren i destinationsguiden under **Fra din gruppe**, opkaldt efter den modtagende boks. Hver instans får sit eget login, første gang den vælger serveren, og skriver kun i sin egen mappe dér. Kortet viser disse logins, og **Tilbagekald login** fjerner ét; det, den instans allerede har kopieret, bliver liggende i mappen. Opsætningen opretter også ét login til en person uden for gruppen, hvis adgangskode kortet viser én gang.

En instans, der kun når den modtagende boks gennem relayet, kan ikke bruge serveren, fordi relayet ikke bærer sikkerhedskopier. Tilføj først den modtagende bokss adresse under **Indstillinger → Parring**. Når BombVault kører på sin egen IP-adresse (for eksempel på br0), skal du udfylde **Adresse til partnere**, fordi serveren lytter på værtens adresse.

## Gennemgået eksempel: to Unraid-maskiner, hele vejen

Ovenfor beskrives delene. Her er én komplet opsætning med rigtige værdier, for dele er nemmere at samle, når man har set dem samlet én gang.

To maskiner: **TOWER** kører containerne og sender sikkerhedskopierne, **VAULT** modtager dem og håndhæver uforanderligheden. Udskift med dine egne navne, adresser og delingsstier.

**1. Rejs append-only-serveren på VAULT.** I BombVault på TOWER: gå til *Indstillinger → Off-site → Opsæt*, vælg **rest-server** og generér opskriften. Kopiér fanen **Unraid-skabelon (XML)**, gem den på VAULT som `/boot/config/plugins/dockerMan/templates-user/my-rest-server.xml`, og vælg derefter *Docker → Add Container* og **rest-server** fra skabelonlisten. Skriv den viste `htpasswd`-linje ind i `/mnt/user/appdata/rest-server/.htpasswd` på VAULT, før du starter den. Engangsadgangskoden vises én gang og gemmes aldrig, så kopiér den nu. Den linje bærer den samme adgangskode, allerede bcrypt-hashet for dig: klarteksten hører til i REST-legitimationsoplysningerne på TOWER, den hashede linje i `.htpasswd` på VAULT. Du skal ikke hashe noget selv.

    Lad `--append-only` blive stående i OPTIONS-feltet. Det er hele pointen: uden det er VAULT en almindelig deling igen.

**2. Peg det eksterne arkiv derhen på TOWER.** Arkivets adresse følger mønstret, som opskriften skriver ud:

    rest:http://VAULT:8000/bombvault-containers/containers

Første led i stien er htpasswd-brugeren, det andet er arkivet. Indtast den genererede bruger og adgangskode som destinationens REST-legitimation, og kør **forbindelsestesten**.

**3. Slå ”Uforanderlig” til på TOWER.** Manipulationstesten kører med det samme og skal sige *beskyttet*. Hvad svarene betyder:

| Resultat | Hvad der skete |
| --- | --- |
| **beskyttet** | VAULT afviste sletningen. Det er den eneste beståede tilstand. |
| **IKKE beskyttet** | VAULT accepterede en sletning. `--append-only` mangler eller er fjernet. |
| **ikke entydigt** | Hverken eller. Som regel er adressen ikke den, restic selv bruger, eller legitimationen er ændret. Intet registreres, og ingen advarsel udløses. |

**4. Se på VAULT, hvad der kommer ind.** Par de to bokse ([Parring af instanser](#pairing)), slå *Indstillinger → Generelt → Modtager* til, åbn fanen **Modtager**, og registrér arkivet skrivebeskyttet med TOWER som den afsendende instans.

!!! warning "Placeringen er en sti **inde i** containeren, skrevet relativt til værtsmonteringen"
    Indtast `user/appdata/rest-server/bombvault-containers/containers`, **ikke** `/mnt/user/appdata/…`. BombVault kører i en container, hvor værtens `/mnt` er monteret et andet sted; en absolut værtssti findes ikke derinde. Indsætter du en, fortæller BombVault dig nu den relative sti, du skal bruge i stedet.

    VAULT får TOWERs restic-adgangskode over gruppen, når du gemmer; ingen skal skrive en nøgle af.

**5. Gør det gensidigt, hvis du vil.** Gentag de samme fem trin den anden vej: en rest-server på TOWER, der modtager VAULTs kopi. Så håndhæver hver maskine uforanderligheden for den anden, og ingen kan slette den andens sikkerhedskopier.

## Guidet gendannelse

En dedikeret **Gendannelse**-fane fører en frisk eller genopbygget installation gennem katastrofetilfældet, ét sted:

1. **Gendanner BombVaults egne indstillinger først**, så de sikkerhedskopi-stier, off-site-destinationer og legitimationsoplysninger, resten af forløbet har brug for, er forudfyldte (anvendt via en selv-genstart over Docker-socket'en, så den kørende indstillingsdatabase aldrig overskrives under et åbent handle).
2. **Tjekker, at BombVault kan læse dine sikkerhedskopier** (krypteringsnøgle-faldgruben på forkant).
3. Lader dig **pege mod dit eksisterende repo** (lokalt eller off-site).
4. **Opdager** de containere, VM'er, filsæt og ZFS-datasæt, der er gemt i det.
5. **Gendanner containere og VM'er i ét hug** (efterladt stoppet, så du starter dem bevidst) og viser filsæt og ZFS-elementer, som du gendanner ét ad gangen; ZFS-elementer kommer tilbage slået fra. Dit gendannelseskit er et klik væk.

!!! tip "Planlagt migrering versus katastrofe"
    Guidet gendannelse gendanner BombVaults egne indstillinger fra en sikkerhedskopi. For et *planlagt* flyt til en ny boks kan du i stedet bære din konfiguration over direkte med kortet **Eksportér / importér indstillinger** (en bærbar JSON-fil). Se [Konfiguration](configuration.md#portable-settings-export-and-import).

### Gendan fra et andet BombVault-repo {#restore-from-another-bombvault-repo}

Et separat kort på **Gendannelse**-fanen åbner et *andet* BombVault-instans' repo (en share monteret under `/mnt`, eller en remote-URL) med **den instans' `APP_KEY`**, i en engangs, skrivebeskyttet session. Gennemse de containere, VM'er og filsæt, der er gemt der, vælg et øjebliksbillede og gendan det, og det gendannede objekt bliver en normal lokal container, VM eller filsæt. Intet skrives nogensinde til det andet repo, og dine egne sikkerhedskopiindstillinger forbliver urørte (sessionen lever i hukommelsen og udløber af sig selv). At flytte en container fra server A til server B betyder ikke længere at ompege dine repo-indstillinger og tilbageføre dem bagefter. Dette kort er til én gang: det åbner en session, gendanner det, du vælger, og glemmer den anden instans. Vil du i stedet have en fast ordning, hvor denne boks efter en tidsplan henter en anden instans' øjebliksbilleder ind i sit eget repository, er det fanen **Hentning** på siden **Instanser**.

En container, hvis netværk ikke findes på denne server, for eksempel et Unraid-`br0`-netværk på en almindelig Docker-vært, viser en netværksvælger under sin række. BombVault opretter den på det netværk, du vælger, sammen med dens andre netværk. Den faste IP- og MAC-adresse hørte til det gamle netværk og falder væk, så det nye netværk tildeler dem.

## Gendannelseskit til krypteringsnøglen

Dette er den brik, der gør katastrofegendannelse mulig, selv når der ikke er nogen kørende BombVault.

Ét klik downloader **hovednøglen**, den **afledte restic-adgangskode** og de **præcise repo-placeringer og kommandoer**, så du kan gendanne direkte med restic-CLI'en på en hvilken som helst maskine. En Oversigts-påmindelse nager, indtil du har gemt det.

!!! danger "Opbevar gendannelseskittet uden for serveren"
    Kittet indeholder hemmeligheden, der dekrypterer dine sikkerhedskopier. Hold det et sikkert sted adskilt fra serveren (en adgangskodemanager, en printet kopi i en boks). Hvis du mister både BombVault og `APP_KEY` uden noget gendannelseskit, kan dine krypterede sikkerhedskopier ikke gendannes.

!!! warning "Det nyeste snapshot er ikke altid det, der skal gendannes"
    Siden restic 0.17 viser `restic snapshots` størrelsen på hvert snapshot. Efter datatab kan det nyeste snapshot være det tømte, så gendan ikke et snapshot, der er langt mindre end dem før det. Efter ransomware kan det være det krypterede i den sædvanlige størrelse. Hvis BombVault stadig kører, så se først på siden **Afvigelser**: den nævner den seneste gode sikkerhedskopi. En gendannelse kræver ingen anomalidata fra BombVault, og opbevaringspausen beholder kun flere snapshots.

### Forsegling af kittet

Hvis du har slået age-kryptering til for de almindelige eksporter (Indstillinger), forsegles kittet også med den og downloades som `bombvault-recovery-kit.md.age`. Det er ASCII-armored frem for binært, så det er stadig almindelig tekst: at indsætte det i en adgangskodemanager eller printe det virker præcis som før, indholdet kan bare ikke læses uden din nøgle.

!!! warning "Opbevar ikke age-nøglen i kittet"
    Du skal bruge din **private** age-nøgle for at åbne et forseglet kit. Opbevar den et sted, der ikke afhænger af selve kittet, ellers har du to ting at gendanne i stedet for én. Forsegling er umagen værd, når kittet ligger et sted, du ikke selv har fuld kontrol over (en delt adgangskodemanager, noter i skyen, en udskrift på et kontor); et kit i dit eget pengeskab er allerede beskyttet af pengeskabet.

    Med kryptering slået til og ingen brugbar modtager sat op nægtes downloadet helt. BombVault falder aldrig tilbage til at udlevere hovednøglen i klartekst.

### Når sættet ikke er ved hånden

Adgangskoden gemmes ingen steder, den **beregnes** ud fra `APP_KEY`. Med nøglen og en shell kan du altså genskabe den selv:

```sh
printf 'bombvault:restic-repo' \
  | openssl dgst -sha256 -mac HMAC -macopt hexkey:$APP_KEY -r \
  | cut -d' ' -f1
```

Det er HMAC-SHA256 over den faste streng `bombvault:restic-repo`, med de rå bytes i den hexadecimale `APP_KEY` som nøgle, skrevet ud som 64 små hexadecimale tegn. Samme værdi står i sættet som den udledte restic-adgangskode; dette er til den dag, hvor sættet ligger et andet sted end dig.

!!! warning "Brug den AFSENDENDE instans' nøgle ved et modtaget arkiv"
    Et arkiv, der er landet her via off-site-replikering, blev oprettet af maskinen, der sendte det, med **dens** `APP_KEY`. Udleder du fra den modtagende maskines nøgle, får du en adgangskode, restic afviser, hvilket ligner et ødelagt arkiv til forveksling uden at være det. Det er den sædvanlige grund til, at `restic check` på et modtaget arkiv bliver ved med at spørge om adgangskoden.

Fordi gendannelsesdefinitioner lever **inde** i hvert repo (`<repo>/def`, `<repo>/vm-def`), er en kopieret repo-mappe fuldt selvstændig, så kittet plus repoet er alt, hvad en bare-metal-gendannelse har brug for.

## Hent et databasedump tilbage {#database-dumps}

Et databasedump er sit eget gendannelsespunkt i container-repositoriet, med etiketten `dbdump:<container>` og den ene fil `/dbdump/<container>.sql`. BombVault viser, henter og importerer dem under **Sikkerhedskopier**; nedenfor er de samme trin med restic alene, til den dag BombVault ikke er der.

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
