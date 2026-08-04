# Off-site og gendannelse

Lokale sikkerhedskopier beskytter dig mod en tabt container eller en dårlig opdatering. Off-site-replikering og et testet gendannelseskit beskytter dig mod hele boksen, ransomware eller en brand. Denne side dækker replikering off-site, at gøre den kopi manipulationssikker, at bevise at du kan gendanne, og at gendanne, når BombVault selv er væk.

## Off-site-replikering

Behold den hurtige lokale sikkerhedskopi, og tilføj en eller flere off-site-replikaer. Sæt et repo pr. domæne på fanen **Indstillinger, Off-site**. BombVault replikerer nye øjebliksbilleder dertil med `restic copy` på et best-effort-grundlag, så et off-site-hikke aldrig får den lokale sikkerhedskopi til at fejle. Det lokale repo forbliver primært.

- **Flere off-site-destinationer pr. domæne.** Hvert domæne (containere, VM'er, flash, config og filsæt) kan replikere til flere off-site-destinationer på én gang, ikke kun én, så du kan beholde for eksempel en rest-server på en vens boks og en S3-bucket parallelt. Tilføj ekstra destinationer på Indstillinger, Off-site, hver med sit eget repository, sin S3-lagringsklasse, sit append-only-flag, sin opbevaring og sit vækstbudget. En eksisterende enkelt off-site-opsætning overføres urørt som den første destination, og hver destination i et domæne replikerer på det domænes off-site-tidsplan.
- **Off-site-tidsplan pr. domæne** (redigeret sammen med alle andre tidsplaner på Indstillinger, Tidsplaner): lad den stå tom for at replikere efter hver lokal sikkerhedskopi, eller sæt en kadence (for eksempel `weekly Sun 03:00`) for at sende off-site sjældnere, end du sikkerhedskopierer lokalt. En **Replikér nu**-knap dækker on-demand-kørsler.
- **Off-site-opbevaring** lever på Indstillinger, Off-site, så du kan beholde off-site-kopier længere som et arkiv. Lad politikken stå helt-nul for aldrig at auto-trimme off-site-øjebliksbilleder.
- **Båndbreddegrænser** (Indstillinger, Off-site) begrænser restic-upload/download-hastigheden, så replikering ikke mætter dit WAN.
- En **replikeringsindikator** viser, hvilket domæne der replikerer, mens det kører (på dets side og på Oversigten). Det er en aktiv indikator, ikke en procentbjælke, fordi `restic copy` ikke eksponerer nogen maskinlæsbar fremdrift.

!!! note "Gendan direkte fra off-site"
    Hver sikkerhedskopi-browser har en **Lokal / Off-site**-kontakt, så hvis et lokalt repo går tabt eller bliver beskadiget, kan du liste og gendanne direkte fra off-site-replikaen. Sletning er pr. kilde: at fjerne en sikkerhedskopi påvirker kun den kopi, du ser på.

## Uforanderlig (append-only) off-site

Flag et off-site-repo append-only, så ransomware eller en kompromitteret vært ikke kan slette eller omskrive dine sikkerhedskopier. Den anden side (en `restic/rest-server`, der kører i `--append-only`-tilstand) **håndhæver** det. BombVault **verificerer** det kun altid og viser aldrig grønt alene på en konfigurationspåstand.

Guiden til **guidet off-site-opsætning** fører dig fra backend-valg (rest-server / rclone / S3) gennem et klar-til-indsæt rest-server-deploy-snippet, en forbindelsestest, uforanderligheds-omskifteren (som kører manipulationstesten med det samme) og en opbevaringsstrategi, så append-only off-site er tilgængelig uden manuel redigering af configs.

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

- **Gendannelses-verifikationsøvelser (lokal).** BombVault kører periodisk `restic check --read-data-subset` (afgrænset, aldrig en disk-fyldende fuld gendannelse) og viser et *sidst verificeret gendannelig*-badge pr. domæne. Kadencen lever på Indstillinger, Tidsplaner; badge't på Indstillinger, Integritet.
- **DR-øvelser (off-site).** BombVault gendanner et rigtigt mål fra off-site-repoet ind i en engangs-sandkasse, verificerer det fil-for-fil og byte-for-byte, og rydder så op. Dette beviser, at du kan gendanne fra off-site, ikke bare at repoet svarer.

**Ransomware-beskyttelses-scorekortet** på Oversigten samler dette til en grøn / gul / rød position pr. domæne, med en aldersstemplet tjekliste (off-site konfigureret, append-only verificeret, replikering aktuel, gendannelsesøvelse bestået, kryptering til, beskæringsstrategi sat). Hver rød række dyb-linker til rettelsen, og kortet bliver kun nogensinde grønt på verificerede fakta.

## Modtager-dashboard (den modtagende side)

Alt ovenstående er den *afsendende* side. På den boks, der **modtager** uforanderlige off-site-kopier fra en anden BombVault, giver modtager-dashboardet dig uafhængig, skrivebeskyttet overvågning af disse repositorier på den modtagende hardware, så en tavs fejl i den anden ende ikke går ubemærket hen.

Slå **Receiver**-omskifteren til i Indstillinger for at afsløre en **Receiver**-fane. Den er som standard fra; aktivér den kun på en boks, der faktisk modtager uforanderlige off-site-sikkerhedskopier. Registrer så et modtaget repository (skrivebeskyttet, åbnet med den afsendende instans' nøgle) for at få:

- **Et øjebliksbillede-inventar grupperet efter kilde**, så du kan se præcis, hvilke containere, VM'er og filsæt der er landet.
- **Sidst-modtaget** pr. kilde, så du ved, hvor frisk hver enkelt er.
- **Et uafhængigt `restic check`** kørt på den modtagende hardware, så integritet verificeres, hvor data faktisk sidder, ikke kun på afsenderen.
- **En dødmandsknap:** en advarsel, når en kilde holder op med at sende inden for et vindue, du sætter.
- **Integritetsadvarsler:** en advarsel, når et tjek på den modtagende side fejler.

Modtageren er strengt skrivebeskyttet. Den skriver aldrig til det modtagne repository, så den kan aldrig bryde append-only-garantien, afsenderen forlader sig på.

## Guidet gendannelse

En dedikeret **Recovery**-fane fører en frisk eller genopbygget installation gennem katastrofetilfældet, ét sted:

1. **Gendanner BombVaults egne indstillinger først**, så de sikkerhedskopi-stier, off-site-destinationer og legitimationsoplysninger, resten af forløbet har brug for, er forudfyldte (anvendt via en selv-genstart over Docker-socket'en, så den kørende indstillingsdatabase aldrig overskrives under et åbent handle).
2. **Tjekker, at BombVault kan læse dine sikkerhedskopier** (krypteringsnøgle-faldgruben på forkant).
3. Lader dig **pege mod dit eksisterende repo** (lokalt eller off-site).
4. **Opdager** de containere, VM'er og filsæt, der er gemt i det.
5. **Gendanner dem alle** (efterladt stoppet, så du starter dem bevidst), med dit gendannelseskit et klik væk.

!!! tip "Planlagt migrering versus katastrofe"
    Guidet gendannelse gendanner BombVaults egne indstillinger fra en sikkerhedskopi. For et *planlagt* flyt til en ny boks kan du i stedet bære din konfiguration over direkte med kortet **Eksportér og importér indstillinger** (en bærbar JSON-fil). Se [Konfiguration](configuration.md#portable-settings-export-and-import).

### Gendan fra et andet BombVault-repo

Et separat kort på **Recovery**-fanen åbner et *andet* BombVault-instans' repo (en share monteret under `/mnt`, eller en remote-URL) med **den instans' `APP_KEY`**, i en engangs, skrivebeskyttet session. Gennemse de containere, VM'er og filsæt, der er gemt der, vælg et øjebliksbillede og gendan det, og det gendannede objekt bliver en normal lokal container, VM eller filsæt. Intet skrives nogensinde til det andet repo, og dine egne sikkerhedskopiindstillinger forbliver urørte (sessionen lever i hukommelsen og udløber af sig selv). At flytte en container fra server A til server B betyder ikke længere at ompege dine repo-indstillinger og tilbageføre dem bagefter. Live server-til-server-federation er eksplicit uden for scope; dette er et bevidst engangstræk.

## Gendannelseskit til krypteringsnøglen

Dette er den brik, der gør katastrofegendannelse mulig, selv når der ikke er nogen kørende BombVault.

Ét klik downloader **hovednøglen**, den **afledte restic-adgangskode** og de **præcise repo-placeringer og kommandoer**, så du kan gendanne direkte med restic-CLI'en på en hvilken som helst maskine. En Oversigts-påmindelse nager, indtil du har gemt det.

!!! danger "Opbevar gendannelseskittet uden for serveren"
    Kittet indeholder hemmeligheden, der dekrypterer dine sikkerhedskopier. Hold det et sikkert sted adskilt fra serveren (en adgangskodemanager, en printet kopi i en boks). Hvis du mister både BombVault og `APP_KEY` uden noget gendannelseskit, kan dine krypterede sikkerhedskopier ikke gendannes.

Fordi gendannelsesdefinitioner lever **inde** i hvert repo (`<repo>/def`, `<repo>/vm-def`), er en kopieret repo-mappe fuldt selvstændig, så kittet plus repoet er alt, hvad en bare-metal-gendannelse har brug for.
