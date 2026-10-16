# Ekstern lagring og gjenoppretting

!!! note "Eksterne kopier venter etter en ombygging"
    Når Nødveiviser eller **Oppdag sikkerhetskopier** bygger opp elementer uten de gamle innstillingene, settes eksterne kopier av de områdene på pause til standarden deres er bekreftet. Se [Etter en ombygging](#after-a-rebuild).

Lokale sikkerhetskopier beskytter deg mot en tapt container eller en dårlig oppdatering. Eksterne kopier og et testet gjenopprettingssett beskytter deg mot hele boksen, løsepengevirus eller en brann. Denne siden dekker mål og eksterne kopier, å gjøre en kopi manipuleringssikker, å bevise at du kan gjenopprette, å koble sammen flere servere og å få alt tilbake når selve BombVault er borte.

## Eksterne kopier {#off-site-copies}

Behold den raske lokale sikkerhetskopien og legg til ett eller flere mål utenfor huset. BombVault kopierer nye sikkerhetskopier dit med `restic copy` på best-effort-basis, så et eksternt hikk feiler aldri den lokale sikkerhetskopien. I denne formen er det lokale repoet primært og det eksterne repoet en kopi, men primærrepoet til et område trenger ikke å være lokalt i det hele tatt; se [Eksterne primære arkiver](#remote-primary-repositories) nedenfor for å sikkerhetskopiere rett til S3, en rest-server og lignende i stedet for å kopiere dit.

- **Flere mål per område.** Hvert område (containere, VM-er, flash, auto-sikkerhetskopi, mapper og ZFS-datasett) kan kopiere til flere eksterne mål samtidig, så du for eksempel kan ha en rest-server hos en venn og en S3-bøtte side om side. Kortet **Mål per område** på siden Mål slår hver linje fra et område til et mål av eller på.
- **Når kopier kjører.** Kortet **Kopier** til et mål setter **Når det kopieres**: **Etter hver sikkerhetskopi**, eller etter en egen plan som samler sikkerhetskopiene og kopierer til faste tider. Under **Innstillinger, Tidsplan, Off-site-kopier per område** gir **Unntak** ett område en egen kopieringsplan. **Replikér nå** i kortet **Off-site-kopi** til et område under **Innstillinger, Mål** starter en kopi på forespørsel.
- **Oppbevaring** settes per mål under **Hvor lenge det beholdes**, så du kan beholde eksterne kopier lenger som arkiv. Kopien i det eksterne feltet til et område følger kortet **Off-site-oppbevaring** under **Innstillinger, Mål**; la det stå på null for aldri å trimme eksterne sikkerhetskopier automatisk.
- **Båndbreddegrenser** (**Ekstern båndbredde** under **Innstillinger, Mål**, eller kortet **Kopier** til et mål) begrenser resticts opplastings-/nedlastingshastighet slik at kopier ikke metter WAN-et ditt.
- Mens en kopi kjører, sier statushodet på oversikten **Off-site-kopi pågår**. Det er en aktiv indikator, ikke en prosentlinje, fordi `restic copy` ikke gir maskinlesbar fremdrift.

!!! note "Gjenopprett fra hvilket som helst sted"
    Hvert element viser sikkerhetskopiene sine som én tidslinje på tvers av alle steder en sikkerhetskopi ligger, på fanen **Sikkerhetskopier**. En sikkerhetskopi som er kopiert til B2, vises én gang, merket med hvert sted som har den. En gjenoppretting tar det første stedet den når, med start i repoet elementet skrives til, og du kan velge et annet sted per sikkerhetskopi. Eksterne mål leses først når du klikker **Sjekk**, fordi det tar en stund å liste skylagring. Å slette på ett sted sjekker først de andre og sier om det var den siste kopien.

## Mål {#destinations}

Et mål er et sted sikkerhetskopier havner: en lokal mappe, en deling, en egen server eller en skytjeneste. Det settes opp én gang og betjener deretter hvert område. Åpne **Mål** fra innstillingsflisen, eller slå på **Vis også i sidefeltet** der for å feste det.

**Legg til mål** åpner et vindu i fire steg:

1. **Leverandør.** Hver tjeneste står med logoen sin, med et søkefelt øverst til høyre i vinduet, i grupper: lagringstjenester med S3-bøtter (Backblaze B2, Wasabi, Cloudflare R2, Hetzner Object Storage, Amazon S3 og andre), din egen S3-server (Garage, SeaweedFS, RustFS, Silo, Ceph, JuiceFS, Versity S3 Gateway), din egen server og delinger (rest-server, Hetzner Storage Box, SFTP, SMB, WebDAV, en montert sti), skylagring (OneDrive, Google Drive, Dropbox, pCloud, Nextcloud og resten rclone støtter) og mottaksserverne i gruppen din. Hver sier hvor godt den passer for sikkerhetskopier: skystasjoner går tregere under mange forespørsler, så den første sikkerhetskopien og beskjæringen tar lengre tid der.
2. **Logg inn.** Feltene avhenger av tjenesten: en tilgangsnøkkel for S3, bruker og passord for WebDAV og SMB, et app-passord der tofaktorinnlogging blokkerer det vanlige, BombVaults offentlige SSH-nøkkel for SFTP og Storage Box, eller et token for tjenester som logger inn via en nettleser. For dem viser vinduet en `rclone authorize`-kommando du kjører på en datamaskin med nettleser; tokenet den skriver ut, føres inn i feltet. **Test** sjekker innloggingen før noe lagres. Vinduet lister deretter mappene på målet, med **Ny mappe** for å lage en og ledig plass der tjenesten melder det. En tom mappe er tryggest; BombVault legger én mappe per område i den.
3. **Beskyttelse.** Vinduet sier rett ut hva tjenesten kan. En rest-server i append-only-modus avslår sletting, og tamper-testen sjekker det. En S3-bøtte kan beholde gamle versjoner med versjonering og objektlås, noe BombVault ikke kan sjekke ennå. En skystasjon kan ikke avslå sletting i det hele tatt: den som kommer inn på serveren, kommer også inn på den kopien. Slå **Beskyttelse mot sletting (append-only)** på bare der den andre siden virkelig avslår sletting; BombVault beskjærer da aldri der. **For en nødsituasjon** sier hva gjenopprettingssettet inneholder for dette målet.
4. **For hvilke områder.** Velg områdene som sikkerhetskopieres hit. Den første kopien tar med hele historikken. Elementer som har valgt egne mål, får ikke det nye av seg selv, og vinduet nevner dem.

S3-tjenester går gjennom resticts egen S3-backend, og det er det som lar en lagringsklasse og objektlås gjelde. Alle andre tjenester går gjennom rclone som følger med BombVault, og remoten vises da i kortet **Off-site (rclone)** under **Innstillinger, Mål**. En eksport av innstillinger tar med målene; med legitimasjon inkludert tar den med påloggingen deres også.

Mappen på denne serveren heter **Lokal** overalt: på målsidene, i flyten, på et element og under Gjenopprett. Finnes det flere mapper på denne serveren, har hver sitt eget navn. Et mål som det eksterne feltet til et område oppretter, får navn etter stedet sitt, den siste mappen i adressen eller, uten mappe, serveren, og får et nummer når et annet mål allerede heter det. Et navn du gir det på siden sin, blir stående. Står to mål med samme navn side om side, har hvert med serveren eller mappen sin.

Hvert mål har en egen side:

- **Tilkobling** med navnet, bryteren **Tilgjengelig** (av betyr at BombVault ikke lenger skriver hit, og elementer som sikkerhetskopieres hit, feiler med en tydelig melding i stedet for å havne et annet sted), mappen, komprimering, lagringsklasse og **Utenfor lokalet**. **Endre tilkobling** tester og lagrer en ny adresse eller pålogging.
- **Legitimasjon**: det delte settet eller et eget sett.
- **Beskyttelse mot sletting**, med **Test beskyttelsen mot sletting nå** og hvem som beskjærer denne kopien.
- **Kopier**: når det kopieres, og opplastings- og nedlastingsgrensene.
- **Hvor lenge det beholdes**: **Kort**, **Balansert**, **Lang** eller **Egne verdier**, med en tidslinje over hvilke sikkerhetskopier regelen beholder. **Spør før det beholdes mindre** spør før en kortere regel sletter sikkerhetskopier hvis eneste kopi ligger her.
- **Regler per område**: ett områdes eget arkiv, egen oppbevaring eller komprimering på dette målet. **Endre regler** åpner dem, og **Tilbake til målet** setter et område tilbake.
- **Ligger allerede under dette målet**: områder hvis håndskrevne adresse ligger i en mappe på dette målet. Å ta ett over lar arkivet ligge der det er; pålogging, lagringsklasse og beskyttelse mot sletting kommer da fra målet. BombVault sjekker først at påloggingen til målet åpner arkivet, og nekter å legge et append-only-arkiv under et mål som ikke er append-only.
- Et kar som viser hvor fullt målet er, med merket der det er fullt og hvor mange uker som er igjen ved veksten de siste ukene. Et mål som ikke melder størrelsen sin, sier **Størrelse ukjent**.

En mottaksserver som en annen instans i gruppen din kjører, vises blant leverandørene som **Fra gruppen din**; se [Mottaksserver](#receiving-server).

### Flytte og fjerne {#move-remove}

- **Flytt** tar kopien av ett område til et annet mål, med hele historikken. Alt blir stående på det gamle stedet; BombVault sletter ingenting der. Flyttingen kjører i bakgrunnen og kan ta lang tid: omtrent 9 minutter per 100 GB mellom lokale disker, timer over en treg forbindelse. Oversikten viser den mens den kjører.
- **Fjern mål** tilbyr to veier. **Fjern mål, behold sikkerhetskopiene** slutter å skrive der og lar sikkerhetskopiene ligge; du kan lese dem senere under **Gjenoppretting, Fra en gammel sikkerhetskopimappe**. **Fjern mål og slett sikkerhetskopiene der** leser først hva som er lagret der og lister det: hvilke sikkerhetskopier som forsvinner, hvilke av dem som ikke finnes andre steder, og hvilke arkiver som blir fordi beskyttelse mot sletting nekter det, elementer fortsatt sikkerhetskopierer rett til dem, et annet mål eller område har kopiene sine der, eller de ikke kunne leses. Elementer som ville miste den eneste sikkerhetskopien sin, står for seg og beholdes med mindre du slår på **Slett disse også**. Du bekrefter ved å skrive navnet på målet, og slettingen kjører i bakgrunnen.
- Å slette sikkerhetskopiene på et mål krever et påloggingspassord og en pålogging på selve instansen. Uten passord kan målet fortsatt fjernes, bare sikkerhetskopiene kan ikke slettes herfra.

## Plassering per element {#placement}

Kortet **Tidsplan og mål** på fanen Innstillinger til et element har en rad med knapper: **Lokal** og én knapp per eksternt mål i området. Tente knapper får sikkerhetskopiene til elementet.

- Med **Lokal** tent skrives elementet til repoet som vises under **Lagret på** og kopieres til hvert annet tent mål. Gjør et mål mørkt, så får det ikke noe nytt fra dette elementet. Bare Lokal kopierer ingen steder, noe som passer data som allerede har en andre kopi, for eksempel en deling som ligger på en NAS.
- Med **Lokal** mørk skrives elementet rett til det direkte repoet til det første tente målet og kopieres derfra til de andre tente. Første gang oppretter en dialog det direkte repoet.
- Én knapp forblir tent, fordi en sikkerhetskopi trenger et sted å havne. For å utelate et element fra sikkerhetskopier bruker du **Ikke sikkerhetskopier**.

BombVault flytter aldri sikkerhetskopier mellom arkiver. Et element som følger området sitt, flytter med når standarden til området endres: den neste sikkerhetskopien havner på det nye stedet, og sikkerhetskopiene det allerede har, blir der de er. Et mål som er valgt på selve elementet, er fast fra første sikkerhetskopi; kortet **Repository** på fanen Innstillinger navngir det. Kopiene kan endres når som helst. Et mål som ikke lenger får et element, beholder kopiene det har og trimmer dem til sin egen oppbevaring ved neste eksterne kjøring av området. Fra append-only-mål kan man ikke slette.

Stedet et element har forlatt, beholder sikkerhetskopiene sine. På fanen **Sikkerhetskopier** til elementet og under **Gjenopprett** står det som **Bare eldre sikkerhetskopier**, en gjenoppretting kan fortsatt lese derfra, og en prune der lar disse sikkerhetskopiene være i fred.

**Følg standarden** henter et element med eget valg tilbake til området sitt. Det spør først, sier hvor nye sikkerhetskopier da går, og navngir hva hvert mål slutter å få og hva neste kjøring ville laste opp.

Under raden sier kortet hvor elementet går og hva som faktisk er der: hvor mange steder som har det, når hvert mål sist ble sett og om 3-2-1 er oppfylt. Et sted er serveren med de opprinnelige dataene, hvert eksternt mål og hvert arkiv merket **Utenfor bygningen**. BombVault sjekker kopier og steder; det sjekker ikke «to medier»-delen av 3-2-1.

### Standarder per område {#placement-defaults}

Kortet **Mål per område** på siden Mål tegner standarden for hvert område som en 3-2-1-flyt. Hver linje fra et område til et mål er en bryter, også **Lokal**, og det for hvert område, altså også for flash-stikken, ZFS-datasettene og auto-sikkerhetskopien. Et klikk på en linje eller en prikk slår den om med en gang, uten spørsmål; stiplede linjer er av. **Graf / Brytere** viser det samme som rader med knapper. En bryter bestemmer hvor nye sikkerhetskopier går. Elementer uten eget valg og prosjektmappene til Compose-stakker følger med en gang, så den neste sikkerhetskopien deres går til de nye stedene. Det som allerede er lagret, blir der det er, og kan fortsatt gjenopprettes derfra. Flash, ZFS-datasett og auto-sikkerhetskopien sikkerhetskopieres til den lokale mappen sin og kopieres off-site fra den sikkerhetskopien. Linjen deres til **Lokal** kan også slås av; sikkerhetskopiene går da rett til off-site-målene deres, og et ZFS-element med en replika og uten noe mål i det hele tatt sender bare replikaen sin.

Er **Lokal** av og bare eksterne mål på, sikkerhetskopierer et område rett dit, og BombVault velger repoet der selv. Kontainere, VM-er og mappesett går til det direkte repoet til det første målet som er på, beskrevet nedenfor, og kopieres derfra til de andre. Flash-stikken, ZFS-datasettene og auto-sikkerhetskopien skriver hver sikkerhetskopi til hvert mål som er på, og hvert mål beholder dem etter sin egen oppbevaring.

Et område der alle linjer er av, viser **Sikkerhetskopieres ikke**. Elementene får ingen sikkerhetskopier og regnes som ubeskyttet.

Elementer som har valgt egne mål på siden sin, beholder dem. Flyten sier hvor mange det er i et område, for eksempel **2 elementer avviker**, og fører til dem: til fanen Innstillinger for elementet når det er ett, til en liste som viser hvor hvert sikkerhetskopierer når det er flere. Der henter **Følg standarden** et element tilbake.

Et nytt eksternt mål får hvert element som ikke er satt til Lokal. Vinduet som legger det til, sier hvor mange elementer og, der det er kjent, hvor mye historikk det er.

### Direkte repoer

Et direkte repo er stedet der elementer sikkerhetskopierer rett til et eksternt mål. BombVault legger det ved siden av mappen til målet med `-direct` lagt til navnet, eller, når målet ligger i roten av bucketen, remoten eller en rest-server-bruker, i en mappe inni som er oppkalt etter området, for eksempel `containers-direct`. I flyten på siden Mål opprettes det uten spørsmål. Å slå Lokal av for ett enkelt element, slik at et mål uten direkte repo blir hjemmet, åpner en dialog med den foreslåtte plasseringen, for eksempel `s3:https://s3.eu-central-003.backblazeb2.com/bucket/containers-direct`, og en tilkoblingstest som ikke oppretter noe. **Opprett og bruk** oppretter repoet og peker elementet dit. Et direkte repo overtar nøkkelen, lagringsklassen, grensene, append-only-innstillingen og oppbevaringen til målet og endres med dem; kortet Repoer viser det skrivebeskyttet. Når en ny nøkkel for målet ikke kan åpne det, beholder det direkte repoet nøkkelen det har, og lagringen sier det. Et element på et direkte repo kopieres derfra til de andre tente målene, aldri til målet repoet hører til. Øyeblikksbildene bærer taggen `bv:direct`, og hver annen oppbevaringsrunde beholder dem, så et direkte repo som mistet koblingen til målet sitt, aldri eldes etter de lokale reglene. B2 nås gjennom S3-endepunktet, med nøkkel-ID-en og applikasjonsnøkkelen som S3-legitimasjon; en nøkkel som er begrenset til målets egen mappe, når ikke mappen ved siden av, så begrens nøkkelen til mappen over i stedet.

### Utenfor lokalet

Et mål eller et navngitt arkiv kan merkes **Utenfor lokalet**. Eksterne starter merket; slå det av for en rest-server i samme bygning. Merket teller bare steder og 3-2-1 på kortene. Det endrer ingen kopi.

En ZFS-replika er en kopi utenfor lokalet, men ikke en sikkerhetskopi med historikk. Den beholder bare øyeblikksbildene du setter, og gamle versjoner og enkeltfiler kommer fortsatt fra sikkerhetskopiene. Den går fra én ZFS-server rett til den andre, over SSH til en ZFS-server eller til mottaksendepunktet til en paret instans. Paringsgruppen bærer bare forespørselen. Se [Replika](zfs-datasets.md#replica).

### Etter en ombygging {#after-a-rebuild}

Kopivalg bor i BombVaults egne innstillinger. Etter en ombygging via **Oppdag sikkerhetskopier** eller Nødveiviser uten gjenopprettede innstillinger er de borte, og å kopiere alt ville sendt elementene du hadde utelatt, til B2 igjen. Eksterne kopier av hvert ombygd område settes derfor på pause. Oversikten sier det, og kortet **Mål per område** tilbyr **Bekreft standard** med en forhåndsvisning av hva neste kjøring kopierer og navnene i sikkerhetskopiene som ikke har noe element, som du kan utelate der. Bare bekreftelsen avslutter pausen; å importere en innstillingsfil gir tilbake regler og standarder, men avslutter den ikke.

## Eksterne primære arkiver {#remote-primary-repositories}

Stien til et område er ikke begrenset til en lokal mappe: pek den rett på en restic-remote (`s3:...`, `rest:http://host:8000/repo`, `sftp:user@host:/repo`, `rclone:remote:bucket/path`), så sikkerhetskopierer BombVault rett dit, uten egen lokal kopi og uten kopieringstrinn. Dette er en annen form enn de eksterne kopiene ovenfor: der er det lokale repoet primært og det eksterne repoet et best-effort-arkiv av det; her **er** det eksterne repoet primæren, og det er den eneste kopien med mindre du også kopierer området til et annet mål.

Stien til hvert område (Kontainere, VM-er, Flash, Auto-sikkerhetskopi, Mapper, ZFS-datasett) settes på siden til målet **Lokal**: **Endre** ved siden av området i kortet **Tilkobling** åpner et vindu med en **Lokal / Ekstern**-bryter:

- **Lokal** viser den kjente mappevelgeren.
- **Ekstern** bytter den ut med et vanlig URL-felt og tilkoblingsinnstillingene et eksternt mål bruker, satt opp for denne primæren i stedet. Derfra får du:
    - **En tilkoblingstest** mot den levende stien, før du stoler på den.
    - **Båndbreddegrenser** (opplasting/nedlasting) slik at en planlagt sikkerhetskopi til en ekstern primær ikke metter WAN-et ditt: de samme restic-flaggene `--limit-upload`/`--limit-download` som eksterne kopier bruker, brukt på selve sikkerhetskopien.
    - **Append-only-beskyttelse**, verifisert med den samme aktive tamper-testen (en ekte DELETE-sonde mot den andre siden) som eksterne mål får. Når den er på, nekter BombVault å beskjære selve repoet, siden det ikke ligger en egen lokal kopi bak og påloggingen på denne boksen ikke må kunne slette den eneste kopien av sikkerhetskopien.
    - **Et vekstbudsjett-varsel**, samplet fra den samme trenden for repostørrelse som kortet Lagring allerede følger.

Ingenting av dette er påkrevd: en håndskrevet ekstern sti uten lagrede sikkerhetsinnstillinger sikkerhetskopierer ubegrenset, kan beskjæres og har ikke noe budsjettvarsel. Sikkerhetsinnstillingene er der for når du vil ha beskyttelsen en ekstern kopi får, uten et eget mål bare for å få den.

!!! note "Sky-/REST-legitimasjon deles"
    En ekstern primær logger inn med den delte skylegitimasjonen under **Innstillinger, Mål**; det finnes ikke noe eget legitimasjonslager for primærarkiver.

### SMB og WebDAV uten vertsmontering {#smb-webdav}

Kortet **Off-site (rclone)** under **Innstillinger, Mål** har et skjema for en Windows- eller Samba-deling og for en WebDAV-server (Nextcloud, ownCloud, SharePoint eller en annen). Fyll inn et kort navn, verten og delingen (SMB) eller URL-en og servertypen (WebDAV), brukeren og passordet, så skriver BombVault rclone-seksjonen for deg. rclone tilslører passordet selv før det lagres; å legge til ett med et navn som allerede finnes, erstatter den seksjonen i stedet for å legge til en andre. **Legg til mål** tilbyr SMB og WebDAV som leverandører med de samme feltene.

Skjemaet svarer med den ferdige plasseringen, for eksempel `rclone:nas:backups`. Bruk den som sti til et område eller som mål og legg til en undermappe hvis du vil ha en (`rclone:nas:backups/bombvault`). Delingen er det første stisegmentet, ikke en del av navnet.

Dette er den bedre veien enn å montere delingen på Unraid: restic fraråder å ha et repository på en montert CIFS-deling, og her monteres ingenting. NFS er ikke med i skjemaet fordi verken restic eller rclone har en NFS-backend; for NFS monterer du eksporten på hosten og bruker den som en lokal mappe.

## Beskyttelse mot sletting (append-only)

Slå på beskyttelse mot sletting for et eksternt mål slik at løsepengevirus, eller en kompromittert host, ikke kan slette eller skrive om sikkerhetskopiene dine. Den andre siden (en `restic/rest-server` som kjører i `--append-only`-modus) **håndhever** det. BombVault bare **verifiserer** det og viser aldri grønt på en konfigurasjonspåstand alene. En mappe på denne serveren kan ikke avslå sletting, fordi den som kontrollerer BombVault også kontrollerer mappen, så lokale mål tilbyr ikke bryteren.

Bryteren hører til målet: en seksjon som kopierer dit, tar den fra målet, så slå den på på målets side under **Mål**. Den planlagte **Kontroller slettebeskyttelse** kjører bare så lenge en seksjon kopierer til et mål med bryteren slått på. Den andre siden må startes med `--append-only`; med imaget `restic/rest-server`, på TrueNAS som på enhver annen Docker-vert, legges det i variabelen `OPTIONS`.

Den raskeste veien til en append-only-kopi er en paret BombVault med en [mottaksserver](#receiving-server): den kjører rest-serveren for deg, i append-only-modus.

!!! note "En vellykket sletting under `/locks/` er forventet"
    Append-only betyr ikke at ingenting noensinne kan fjernes. restic må ta og slippe sine egne låser, så `/locks/` forblir skrivbar og slettbar etter design. Sikkerhetskopier og dataene bak dem, som er det løsepengevirus går etter, kan ikke fjernes. Hvis du selv tester den andre siden, er en sletting som lykkes under `/locks/` riktig oppførsel og ikke et hull i beskyttelsen.

!!! warning "Append-only-repoer beskjæres aldri fra denne boksen"
    Et append-only-mål beskjærer aldri gamle sikkerhetskopier herfra; beskjæring er den andre sidens jobb. Sett et **vekstbudsjett-varsel** for det, så du får beskjed før repostørrelsen løper løpsk.

## Tamper-test

BombVault beviser append-only-garantien jevnlig ved faktisk å forsøke en sletting mot det eksterne repoet, rettet mot et objekt som ikke finnes. **Test beskyttelsen mot sletting nå** på siden til et mål kjører den på forespørsel, og **Kontroller slettebeskyttelse** under **Innstillinger, Verifisering, Automatiske kontroller** kjører den etter en tidsplan.

En kopi som er slått av, testes ikke, og målet sier det. Tilkoblingstesten virker fortsatt, fordi den bare leser, så en kopi kan kontrolleres før den slås på.

- **Avvist** betyr beskyttet.
- **Godtatt** betyr ikke beskyttet.
- Et **uavklart** resultat (server utilgjengelig, autentiseringsfeil) snur aldri den lagrede dommen.

En ekte omslag fra beskyttet til ubeskyttet utløser ett enkelt varsel.

## DR-øvelser

BombVault tilbyr to nivåer av bevis for at sikkerhetskopiene dine faktisk er gjenopprettbare, ikke bare til stede.

- **Gjenopprettingsverifiserings-øvelser (lokalt).** BombVault kjører jevnlig `restic check --read-data-subset` (begrenset, aldri en diskfyllende full gjenoppretting). Rytmen ligger under **Innstillinger, Verifisering, Automatiske kontroller**, og kortet **Kontroller sikkerhetskopier** på samme flis viser resultatet.
- **Testgjenoppretting (eksternt).** BombVault gjenoppretter et ekte mål fra det eksterne repoet til en avskjermet sandkasse, verifiserer det fil for fil og byte for byte, og rydder opp. Dette beviser at du kan gjenopprette fra eksternt, ikke bare at repoet svarer.

Kortet **Beskyttelsesstatus** på oversikten sammenfatter dette per område i fire kontroller: en fersk sikkerhetskopi på denne serveren, en kopi eksternt, beskyttelse mot sletting der og en testgjenoppretting derfra. Et klikk på et område viser detaljene, og kortet blir bare grønt på verifiserte fakta.

## Koble instanser sammen {#pairing}

Flere BombVault-instanser jobber sammen som én gruppe: de viser hverandres beskyttelsesstatus, tar roller for hverandre og kan oppbevare hverandres nøkkel. En instans blir med i gruppen med tolv ord.

Åpne **Innstillinger, Paring** på den første instansen og trykk **Generer frase**. Tre stegkort viser hvordan det går. Tolv ord vises i et vindu med en **Kopier**-knapp. Åpne det samme stedet på hver annen instans, trykk **Angi frase** og lim inn eller skriv dem, eller trykk **Lim inn** i det vinduet. Et ord som ikke står på listen, navngis med posisjonen mens du skriver, og det siste ordet har en kontrollsum, så et feilskrevet eller byttet ord fanges før noe pares. Generer frasen på bare én instans: to instanser som begge genererer en, danner to separate grupper. Hvis ingen dukker opp på et minutt, tilbyr siden to utveier: vis ordene igjen for å skrive dem inn der, eller skriv inn den andre instansens ord og bli med i gruppen hennes i ett steg. Paring fungerer uten påloggingspassord, men sett ett: uten kan alle som kan åpne nettgrensesnittet, lese ordene og, gjennom gruppen, be om roller som deler ut restic-passordet til instansene i den. Paringskortet sier det til et passord er satt. Med passord ber det om det å vise frasen igjen. **Forlat gruppen** tar en instans ut igjen. Kortet **Instanser** på paringssiden lister medlemmene i gruppen; bildet av hvem som gjør hva for hvem står på siden Instanser.

Alle som kjenner ordene, kan bli med i gruppen, så behandle dem som et passord.

**Hvordan medlemmer når hverandre.** Hver instans lærer sin egen adresse på nettverket fra nettleseren din i det du logger inn; den står under **Kjente adresser** i kortet **Denne instansen** på paringssiden sammen med dem instansen fant selv; rett den der hvis en omvendt proxy eller en uvanlig port står foran. På samme nettverk kunngjør medlemmene den adressen med multicast og snakker direkte, og der multicast ikke kan krysse et containernettverk, som Dockers standard bridge, søker en instans i stedet gjennom sitt eget subnett etter de andre med et signert kall bare et gruppemedlem kan svare på, så paringen blir ferdig på sekunder uten relay. Hvis ingenting dukker opp, tar **Finner du den ikke?** én adresse for hånd, for et annet subnett eller en ikke-standard port. Instanser på ulike nettverk går gjennom et relay, valgt i én rad med fire i kortet **Relay** på samme side; hver form kommer med et bilde:

- **Prosjektrelay** (standard): `parleyport.halleluja.design`, relayet KnightLoader også bruker. Ingenting å sette opp.
- **Eget relay**: containeren [**ParleyPort**](https://github.com/junkerderprovinz/parleyport) fra Unraid Community Apps (også under **Innstillinger, Apper**). Skriv inn adressen på hver instans som skal bruke det.
- **BombVault-instans**: en av instansene dine som allerede er nåbar utenfra, med **Fungere som relay** slått på. Den instansen svarer da på `/relay/connect` på sin egen adresse, bak den omvendte proxyen og det sertifikatet den allerede har, og slipper bare gruppen din inn. Skriv inn adressen på hver instans som skal bruke den.
- **Ingen relay**: medlemmer finner hverandre automatisk på samme nettverk, og ingen andre steder.

**Hva relayet ser.** En knapp med samme navn ved siden av de fire formene viser dette for formen du valgte. Hvert kall mellom medlemmer er forseglet med AES-256-GCM under en nøkkel utledet fra de tolv ordene, og den nøkkelen forlater aldri instansene dine. Relayet lærer en hash som grupperer forbindelsene, hvilken instans en melding er til, hvor stor den er og når den passerer. Et direkte kall på det lokale nettverket er forseglet på samme måte og i tillegg signert, så ingenting avhenger av det selvsignerte sertifikatet en instans tilbyr. Kall som deler ut noe (rollesvar, et restic-passord, mottakerpålogginger, nøkkeldeponeringer) er i tillegg signert med en nøkkel fra medlemmet som sender dem, festet ved første møte; se [Sikkerhetsmodell](configuration.md#security-model).

**Hva som går over gruppen.** Beskyttelsesstatusen på siden Instanser, en forespørsel om å sjekke ett område nå, rolleforespørsler og svarene på dem, tilbud om mål, og det en mottaker eller henter trenger når rollen er tillatt: den andre instansens arkivplasseringer og restic-passordet hennes. Sikkerhetskopidata gjør det aldri; de går fortsatt rett til restic-backendene. APP_KEY gjør det heller ikke: restic-passordet åpner arkivene til den instansen og ingenting annet, ikke de lagrede hemmelighetene, øktene eller gjenopprettingskodene hennes.

**Oppføringer fra før paringen.** Instanser lagt til med et fleet-token, og mottatte arkiver og pull-kilder satt opp med den andre instansens APP_KEY, blir stående etter oppdateringen og er merket **Par på nytt**. De fortsetter å virke: ved første oppstart erstatter BombVault hver lagrede APP_KEY med restic-passordet utledet fra den. Par begge instansene, rediger deretter oppføringen og velg instansen. Til da står slike oppføringer i kortet **Uten instans** på siden Instanser.

Det eneste stedet som fortsatt tar en APP_KEY for hånd, er [Fra en gammel sikkerhetskopimappe](#restore-from-another-bombvault-repo), for tilfellet der den andre instansen er borte og ikke kan svare i en gruppe.

## Instanser og roller {#roles}

Siden **Instanser** (en innstillingsflis som kan festes til sidefeltet) viser gruppen som et nett av hvem som gjør hva for hvem, et kort for denne instansen og et kort for hvert annet medlem, Android-telefonene og ZFS-serverne. **Paring** nederst til høyre fører til paringssiden. Hvilke deler som er på, settes under **Innstillinger, Instanser, Flere servere**: **Instanser**, **Mottaker** og **Henting**.

Hvert kort viser medlemmets beskyttelsesstatus og rollene som fliser. En rolle er noe én instans gjør for en annen:

- **Mottaker** oppbevarer denne serverens eksterne kopier i en rest-server i append-only-modus. Denne serveren sender etter hver sikkerhetskopi og kan ikke slette noe der, selv om noen tar den over.
- **Henter** henter denne serverens sikkerhetskopier etter sin egen tidsplan. Denne serveren trenger ikke være våken for det.
- **ZFS-server** tar imot denne serverens ZFS-replikaer. Hvert ZFS-element velger den under **Replika** i innstillingene sine.
- **Nøkkelvokter** oppbevarer en forseglet kopi av gjenopprettingssettet til denne serveren. Se [Nøkkelvokter](#key-keeper).

Å slå på en rolle sender en forespørsel, og ingenting starter før noen på den andre instansen trykker **Tillat**. Forespørselen vises på kortet til den instansen som et merke, og **Detaljer** lister den med **Tillat** og **Avslå**; en tillatt rolle kan trekkes tilbake der når som helst. En stiplet linje i nettet er en rolle som fortsatt venter. En instans på BombVault 9.x kan ikke svare på en forespørsel, så en rolle er aktiv der så snart du slår den på, og en forespørsel fra en 9.x-instans venter på **Tillat** her.

**Detaljer** på et kort åpner resten: de mottatte arkivene og pull-kildene til det medlemmet, tilbud om mål, overvåking, **Fjern** og **Glem nøkkelen**.

## Mottaker og henter {#receiver}

![En mottatt ekstern kopi i detaljene til en instans, overvåket skrivebeskyttet, med en integritetssjekk kjørt på denne maskinvaren.](assets/screenshots/receiver.png)

*En mottatt ekstern kopi i detaljene til en instans, overvåket skrivebeskyttet, med en integritetssjekk kjørt på denne maskinvaren.*

På boksen som **mottar** append-only-kopier fra en annen BombVault gir de mottatte arkivene deg uavhengig, skrivebeskyttet overvåking på mottaksmaskinvaren, så en stille feil i den andre enden ikke går ubemerket hen. Slå på **Mottaker** under **Innstillinger, Instanser**, åpne deretter **Detaljer** for den sendende instansen på siden Instanser og legg til det mottatte arkivet (skrivebeskyttet, åpnet med den sendende instansens restic-passord, som den får over [paringsgruppen](#pairing)). Du får:

- **Et inventar per kilde**, så du kan se nøyaktig hvilke containere, VM-er og mappesett som har kommet.
- **Sist mottatt** per kilde, så du vet hvor fersk hver enkelt er.
- **En uavhengig `restic check`** kjørt på mottaksmaskinvaren, så integriteten verifiseres der dataene faktisk ligger, ikke bare hos senderen.
- **En dødmannsbryter:** et varsel når en kilde slutter å sende innenfor et vindu du setter.
- **Integritetsvarsler:** et varsel når en sjekk på mottakssiden feiler.

Mottakeren er strengt skrivebeskyttet. Den skriver aldri til det mottatte arkivet, så den kan aldri bryte append-only-garantien senderen stoler på.

En henter virker motsatt vei: med **Henting** slått på leser en instans sikkerhetskopiene til et annet medlem inn i sitt eget arkiv etter sin egen tidsplan. Kildene hennes står også i detaljene til det medlemmet.

### Mottaksserver {#receiving-server}

Mottaksboksen kan også kjøre rest-serveren de andre kopierer til. Den settes opp første gang du tillater en **Mottaker**-forespørsel som ber om det: vinduet sier at serveren trenger en mottaksserver og ber om en mappe på en deling, med **Ny mappe** for å lage en, og en port (8000 med mindre en annen container bruker den). BombVault:

1. avslår hvis en container som heter `rest-server` allerede finnes, eller en annen container holder porten;
2. henter `restic/rest-server` og starter den gjennom Docker-socketen i append-only-modus med private arkiver og en påloggingsfil i mappen;
3. skriver Unraid-malen til flash-stasjonen, slik at containeren kan redigeres i Docker-fanen, eller tilbyr malen som nedlasting når flash-stasjonen er utenfor rekkevidde;
4. kjører tamper-testen mot den og viser om den avslår sletting.

Instansene i gruppen din finner serveren blant leverandørene under **Fra gruppen din**, oppkalt etter mottaksboksen, og rollen **Mottaker** tilbyr den som det trygge valget. Hver instans får en egen pålogging første gang den bruker serveren, og skriver bare i sin egen mappe der. Med **Mottaker** slått på under **Innstillinger, Instanser** åpner **Mottaksserver** på kortet til denne instansen serveren igjen. Vinduet lister påloggingene under **Partnere med pålogging**, og **Trekk tilbake pålogging** tar en bort; det instansen allerede har kopiert, blir i mappen. Oppsettet lager også én **Pålogging utenfor gruppen**, hvis passord vinduet viser én gang.

En instans som bare når mottaksboksen gjennom relayet, kan ikke bruke serveren, fordi relayet ikke bærer sikkerhetskopier. Legg først til adressen til mottaksboksen på paringssiden. Når BombVault kjører på en egen IP-adresse (for eksempel på br0), fyll inn **Adresse for partnere**, fordi serveren lytter på hostens adresse.

## Gjennomgått eksempel: to Unraid-maskiner, hele veien

Alt ovenfor beskriver delene. Dette er ett komplett oppsett, fordi delene er lettere å sette sammen når du har sett dem satt sammen én gang.

To maskiner: **TOWER** kjører containerne og pusher sikkerhetskopier; **VAULT** mottar dem og håndhever append-only. Bytt ut med dine egne navn, adresser og delingsstier.

**1. Par de to maskinene.** Åpne **Innstillinger, Paring** på TOWER, trykk **Generer frase** og skriv inn de tolv ordene på VAULT under **Angi frase**. Sett først et påloggingspassord på begge.

**2. Be VAULT om å være mottaker, fra TOWER.** Slå på **Mottaker** på kortet til VAULT på siden Instanser, velg områdene og **mottaksserveren til VAULT** som sted, og send forespørselen.

**3. Tillat den på VAULT.** Siden Instanser på VAULT viser forespørselen på kortet til TOWER. Trykk **Tillat**. Første gang ber VAULT om en mappe til mottaksserveren sin, for eksempel `user/appdata/rest-server`, starter rest-serveren i append-only-modus og kjører tamper-testen mot den.

**4. Tilbake på TOWER.** Fra da av kopierer TOWER til VAULT etter hver sikkerhetskopi, og det nye målet vises på siden Mål på TOWER med beskyttelsen mot sletting.

Hva tamper-testen svarer:

| Resultat | Hva som skjedde |
| --- | --- |
| **beskyttet** | VAULT avslo slettingen. Dette er den eneste tilstanden som består. |
| **ikke beskyttet** | VAULT godtok en sletting. `--append-only` mangler eller ble fjernet. |
| **uavklart** | Ingen av delene. Vanligvis er URL-en ikke den restic selv bruker, eller påloggingen er endret. Ingenting registreres og ingen varsler utløses. |

**5. Følg med på det som kommer, hvis du vil.** Slå på **Mottaker** under **Innstillinger, Instanser** på VAULT, åpne **Detaljer** for TOWER og legg til det mottatte arkivet. VAULT får restic-passordet til TOWER over gruppen; ingen skriver en nøkkel.

**6. Gjør det gjensidig.** Gjenta stegene i motsatt retning: VAULT ber TOWER om å være mottakeren sin. Hver boks håndhever da append-only for den andre, og ingen av dem kan slette den andres sikkerhetskopier.

En rest-server du kjører for hånd, fungerer også: legg den til med **Legg til mål** som rest-server, med brukeren og passordet fra `.htpasswd`, og slå på beskyttelse mot sletting. La `--append-only` stå i alternativene; uten det er den andre siden en vanlig deling igjen.

!!! warning "En lokal sti er en sti inne i containeren"
    Der BombVault ber om en mappe på denne serveren, skriv den relativt til vertsmonteringen, for eksempel `user/appdata/rest-server/bombvault-containers`, ikke `/mnt/user/appdata/...`. BombVault kjører i en container, der hostens `/mnt` er montert et annet sted; en absolutt vertssti finnes ikke der inne. Hvis du limer inn en, forteller BombVault deg hvilken relativ sti du skal bruke i stedet.

## Nødveiviser {#emergency}

**Nødveiviser** henter tilbake en server som er borte eller satt opp på nytt. Den starter på siden **Gjenoppretting** under **Nød**, og fra **Gjenopprett fra en sikkerhetskopi** på velkomstsiden til en ny installasjon. Den går i fire steg: **Tilgang**, **Innstillinger**, **Utvalg** og **Gjenoppretting**.

1. **Tilgang.** Vis BombVault nøkkelen til de gamle sikkerhetskopiene, på én av tre måter:
    - **Last opp gjenopprettingssett**: filen du lastet ned under oppsettet. Den inneholder nøkkelen og hvert mål, og BombVault fyller inn resten fra den. Et sett forseglet med age ber om identiteten det ble forseglet for; den brukes én gang og lagres ikke.
    - **Via gruppen**: en instans i gruppen din oppbevarer nøkkelen som en deponering. Du trenger gruppens tolv ord, frigivelseskoden din og noen på den instansen. Se [Nøkkelvokter](#key-keeper).
    - **Skriv inn nøkkelen**: den gamle serverens `APP_KEY`, og hvor sikkerhetskopiene står.

    BombVault prøver deretter nøkkelen på hvert mål og sier for hvert om det er lesbart, ikke funnet, låst med en annen nøkkel, ikke nåbart eller fortsatt mangler pålogging. Ingenting er lagret ennå, og nøkkelen blir i minnet en begrenset tid. **Overta nøkkelen** gjør at denne serveren heretter skriver sikkerhetskopiene sine med den nøkkelen; **Angre overtakelsen** går tilbake til serverens egen nøkkel til den første sikkerhetskopien har kjørt. En server som allerede har egne sikkerhetskopier, nekter overtakelsen og peker i stedet til [Fra en gammel sikkerhetskopimappe](#restore-from-another-bombvault-repo). **Lagre påloggingsopplysninger** lagrer påloggingene settet har for eksterne mål, og spør før det erstatter pålogginger denne serveren allerede har.
2. **Innstillinger.** **Hent tilbake innstillinger** gjenoppretter BombVaults egne innstillinger fra den nyeste auto-sikkerhetskopien: mål, legitimasjon, tidsplaner og regler. BombVault starter på nytt for å ta dem i bruk, gjennom Docker-socketen, slik at den levende innstillingsdatabasen aldri overskrives under et åpent håndtak, og veiviseren fortsetter av seg selv etterpå. Påloggingen til denne serveren blir: du logger fortsatt inn med passordet du bruker nå. Hemmeligheter den gamle serveren hadde lagret og som ikke kan overtas, listes, så du kan skrive dem inn igjen.
3. **Utvalg.** BombVault går gjennom sikkerhetskopiene dine og lister containerne, VM-ene, mappesettene og ZFS-datasettene det finner. Den nyeste sikkerhetskopien er forhåndsvalgt overalt. Det denne serveren fortsatt har, starter avslått, fordi en gjenoppretting ville erstatte det. Mappesett vet ikke lenger hvor de kom fra etter en ombygging, så de havner i en mappe du velger.
4. **Gjenoppretting.** Alt valgt kommer tilbake i én omgang, stoppet hvis du vil starte hver container og VM selv, noe som er tryggest etter et totaltap. Last etterpå ned et nytt gjenopprettingssett: det gamle hører til den forrige installasjonen.

!!! tip "Planlagt migrering versus katastrofe"
    Nødveiviser gjenoppretter BombVaults egne innstillinger fra en sikkerhetskopi. For en *planlagt* flytting til en ny boks kan du i stedet ta med konfigurasjonen direkte med kortet **Eksporter / importer innstillinger** (en portabel JSON-fil). Se [Konfigurasjon](configuration.md#portable-settings-export-and-import).

### Fra en gammel sikkerhetskopimappe {#restore-from-another-bombvault-repo}

**Fra en gammel sikkerhetskopimappe** under **Andre kilder** på siden Gjenoppretting åpner arkivet til en *annen* BombVault-instans (en deling montert under `/mnt`, eller en ekstern adresse) med gjenopprettingssettet til den instansen eller dens inntastede `APP_KEY`, i en engangs, skrivebeskyttet økt. Bla gjennom containerne, VM-ene, mappesettene og ZFS-datasettene som ligger der, velg en sikkerhetskopi og gjenopprett den, og det gjenopprettede objektet blir en vanlig lokal container, VM eller et mappesett. Det skrives aldri noe til det andre arkivet, og dine egne sikkerhetskopiinnstillinger blir urørt (økten lever i minnet og utløper av seg selv). Å flytte en container fra server A til server B betyr ikke at du må peke innstillingene dine om og tilbake etterpå. Dette vinduet er en engangsting: det åpner en økt, gjenoppretter det du velger og glemmer den andre instansen. Vil du i stedet ha en fast ordning, der denne boksen henter en annen instans' sikkerhetskopier inn i sitt eget arkiv etter en tidsplan, er det rollen **Henter** på siden Instanser.

En container hvis nettverk ikke finnes på denne serveren, for eksempel et Unraid-nettverk `br0` på en vanlig Docker-vert, viser en nettverksvelger under raden sin. BombVault oppretter det på nettverket du velger, sammen med de andre nettverkene sine. Den faste IP-en og MAC-adressen hørte til det gamle nettverket og fjernes, så det nye nettverket tildeler dem.

## Nøkkelvokter {#key-keeper}

En nøkkelvokter er en instans i gruppen din som oppbevarer en forseglet kopi av gjenopprettingssettet til denne serveren. Den kan ikke åpne kopien selv. Hvis denne serveren noen gang går tapt, ber en ny installasjon som blir med i gruppen din, vokteren om kopien og åpner den med frigivelseskoden din.

1. Slå på **Nøkkelvokter** på kortet til instansen som skal oppbevare den på siden Instanser, og velg **Deponer**. Å deponere krever et påloggingspassord.
2. Denne serveren viser **Frigivelseskode** én gang: tjue bokstaver og sifre i fire grupper på fem. Skriv den ned og bekreft med **Jeg har skrevet ned koden**. Denne serveren lagrer ikke koden, bare det den trenger for å forsegle en deponering for den.
3. Vokteren oppbevarer nå deponeringen, og denne serveren fornyer den når settet endres. **Nøkkeldeponering** på kortet til denne instansen lister **Deponeringen til denne serveren** med hver vokter og tilstanden deres, og **Deponeringer denne serveren oppbevarer** for de andre. **Lag en ny kode** erstatter koden; hver vokter bytter til den nye når den får den nye deponeringen.

Den dagen trenger du tre ting: de tolv ordene i gruppen din, frigivelseskoden og tilgang til vokterens grensesnitt, der noen må trykke **Tillat**. Velg **Via gruppen** i Nødveiviser, velg deponeringen, be om den og skriv frigivelseskoden. Koden brukes på den nye serveren og sendes ikke til vokteren. Hos vokteren vises forespørselen under **Nøkkeldeponering** med hvem som spør, om den instansen noen gang er sett der før og hvor ofte den spurte; **Tillat** leverer ut den forseglede kopien, og å tillate krever et påloggingspassord. Den nye installasjonen har 30 minutter på å hente den etter tillatelsen, en forespørsel ingen svarer på, utløper etter et døgn, og en instans som spør for ofte, må vente en time.

Uten koden kan ingen åpne kopien, vokteren heller ikke. Er koden tapt, lag en ny mens denne serveren fortsatt kjører; er serveren allerede borte, fungerer gjenopprettingssettet og den inntastede APP_KEY fortsatt. Vokteren kan slette en deponering når som helst, og eieren må da spørre på nytt.

## Gjenopprettingssett {#recovery-kit}

Dette er delen som gjør katastrofegjenoppretting mulig selv når det ikke finnes noen kjørende BombVault.

**Last ned gjenopprettingssett** i kortet **For nødstilfeller** under **Innstillinger, Gjenoppretting** gir deg **nøkkelen**, det **utledede restic-passordet**, hvert **mål** med arkivet til hvert område, påloggingene en ny installasjon trenger for å nå dem, og de **nøyaktige restic-kommandoene**, så du kan gjenopprette rett med restic-CLI på hvilken som helst maskin. Det krever et påloggingspassord. Settet skriver ut nøkkelen også med kryptering slått av, fordi container- og VM-definisjonene alltid er forseglet med den. Etter en overtakelse skriver det ut den overtatte sikkerhetskopinøkkelen og APP_KEY til denne serveren, som en ny installasjon bare trenger for å hente tilbake innstillingene. Et kort **Lagre gjenopprettingssettet ditt** på oversikten minner deg på det til du har lagret det.

!!! danger "Oppbevar gjenopprettingssettet bort fra serveren"
    Settet inneholder hemmeligheten som dekrypterer sikkerhetskopiene dine. Oppbevar det et trygt sted og adskilt fra serveren (en passordbehandler, en utskrift i et pengeskap). Mister du både BombVault og `APP_KEY` uten gjenopprettingssett og uten nøkkeldeponering, kan de krypterte sikkerhetskopiene dine ikke gjenopprettes.

!!! warning "Den nyeste sikkerhetskopien er ikke alltid den som skal gjenopprettes"
    Siden restic 0.17 viser `restic snapshots` størrelsen på hvert øyeblikksbilde. Etter datatap kan den nyeste sikkerhetskopien være den tømte, så ikke gjenopprett en sikkerhetskopi som er langt mindre enn de foran. Etter løsepengevirus kan den være den krypterte, i vanlig størrelse. Kjører BombVault fortsatt, sjekk først siden **Avvik**: den navngir den siste gode sikkerhetskopien, og gjenopprettingsvinduet forhåndsvelger den. En gjenoppretting trenger ingen av BombVaults avviksdata, og oppbevaringspausen beholder alltid bare flere sikkerhetskopier.

### Forsegle settet

Har du slått på **Krypter eksporter med age** under **Innstillinger, System**, forsegles settet også med den og lastes ned som `bombvault-recovery-kit.md.age`. Det er ASCII-armert i stedet for binært, så det er fortsatt ren tekst: å lime det inn i en passordbehandler eller skrive det ut fungerer akkurat som før, innholdet er bare uleselig uten nøkkelen din.

!!! warning "Ikke oppbevar age-nøkkelen i settet"
    Du trenger den **private** age-nøkkelen din for å åpne et forseglet sett. Oppbevar den et sted som ikke er avhengig av selve settet, ellers har du to ting å gjenopprette i stedet for én. Forsegling lønner seg når settet ligger et sted du ikke har full kontroll over (en delt passordbehandler, notater i skyen, en utskrift på et kontor); et sett i ditt eget safe er allerede beskyttet av safet.

    Med kryptering på og ingen brukbar mottaker satt opp nektes nedlastingen helt. BombVault faller aldri tilbake på å gi ut hovednøkkelen i klartekst.

### Når settet ikke er for hånden

Passordet lagres ingen steder, det **beregnes** ut fra nøkkelen til sikkerhetskopiene. Med nøkkelen og et skall kan du altså gjenskape det selv:

```sh
printf 'bombvault:restic-repo' \
  | openssl dgst -sha256 -mac HMAC -macopt hexkey:$APP_KEY -r \
  | cut -d' ' -f1
```

Det er HMAC-SHA256 over den faste strengen `bombvault:restic-repo`, med de rå bytene i den heksadesimale `APP_KEY` som nøkkel, skrevet ut som 64 små heksadesimale tegn. Den samme verdien står i settet, oppført som det utledede restic-passordet; dette er for dagen settet ligger et sted du ikke er. På en server som har overtatt en annens sikkerhetskopier, bruker du den overtatte sikkerhetskopinøkkelen, som er den serverens gamle APP_KEY.

!!! warning "For et mottatt arkiv, bruk den SENDENDE instansens nøkkel"
    Et arkiv som havnet her via en ekstern kopi, ble opprettet av maskinen som sendte det, med **dens** nøkkel. Å utlede fra den mottakende maskinens nøkkel gir et passord restic avviser, noe som ser nøyaktig ut som et ødelagt arkiv uten å være det. Det er den vanlige grunnen til at `restic check` på et mottatt arkiv spør om passordet igjen og igjen.

Fordi gjenopprettingsdefinisjoner ligger **inne i** hvert repo (`<repo>/def`, `<repo>/vm-def`), er en kopiert repo-mappe fullstendig selvstendig, så settet pluss repoet er alt en bare-metal-gjenoppretting trenger.

## Hente en databasedump tilbake {#database-dumps}

En databasedump er sitt eget gjenopprettingspunkt i container-repositoriet, med etiketten `dbdump:<container>` og den ene filen `/dbdump/<container>.sql`. BombVault lister, laster ned og importerer dem på fanen **Sikkerhetskopier** til containeren; under står de samme stegene med restic alene, til dagen BombVault ikke er der.

```sh
restic -r <repo> snapshots --tag dbdump:<container>
restic -r <repo> dump --tag dbdump:<container> latest /dbdump/<container>.sql > <container>.sql
```

Etikettene `dbversion:` og `dbname:` på hver dump sier hvilken serverversjon den kommer fra, og hvilke databaser den inneholder. En komplett fil slutter med `-- PostgreSQL database cluster dump complete` eller `-- Dump completed`.

Importer den i en container med samme eller nyere versjon (PostgreSQL), eller samme hovedversjon (MySQL og MariaDB), startet én gang med tom datamappe slik at den initialiserer seg. Verten trenger ingen databaseklient, containeren har en:

```sh
docker exec -i <container> sh -c 'exec psql -X -U "${POSTGRES_USER:-postgres}" -d postgres' < <container>.sql
docker exec -i <container> sh -c 'exec mariadb -uroot -p"$MARIADB_ROOT_PASSWORD"' < <container>.sql
docker exec -i <container> sh -c 'exec mysql -uroot -p"$MYSQL_ROOT_PASSWORD"' < <container>.sql
```

For én enkelt database ut av en full dump tar MySQL og MariaDB `--one-database <name>` på klientkommandoen. En PostgreSQL-dump har én seksjon per database, hver innledet med linjen `\connect <name>`: kopier den seksjonen til sin egen fil og importer den med `-d <name>` etter at databasen er opprettet.

!!! warning "En dump tatt som root bringer med seg serverens brukere"
    En full MySQL- eller MariaDB-dump tatt som root inneholder systemdatabasen `mysql`, så import erstatter den nye serverens kontoer, root-passordet inkludert, med dem fra dumpen. På PostgreSQL er `role ... already exists` for brukeren containeren selv opprettet, forventet og ufarlig.
