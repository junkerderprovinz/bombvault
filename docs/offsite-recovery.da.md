# Off-site og gendannelse

Lokale sikkerhedskopier beskytter dig mod en tabt container eller en dårlig opdatering. Off-site-replikering og et testet gendannelseskit beskytter dig mod hele boksen, ransomware eller en brand. Denne side dækker replikering off-site, at gøre den kopi manipulationssikker, at bevise at du kan gendanne, og at gendanne, når BombVault selv er væk.

## Off-site-replikering

Behold den hurtige lokale sikkerhedskopi, og kopiér den til et eller flere andre steder. Hvilke steder et domæne kopieres til, vælger du på kortet **Domæner** under **Indstillinger, Lagring**, én chip pr. sted (se [Lagringssteder](storage-places.md#domains)). BombVault kopierer nye øjebliksbilleder dertil med `restic copy` på et best-effort-grundlag, så en mislykket kopi aldrig får den lokale sikkerhedskopi til at fejle. Det sted, et domæne gemmes på, behøver ikke at være lokalt; se [Et domæne gemt på et fjernt sted](#remote-primary-repositories).

- **Flere kopisteder pr. domæne.** Et domæne kan kopieres til flere steder på én gang, for eksempel en rest-server hos en ven og en B2-bucket. Opbevaring, lagringsklasse, append-only, grænser og vækstbudget hører til stedet, så hver kopi følger reglerne for det sted, den lander på.
- **Kopitidsplan pr. domæne** (redigeret sammen med alle andre tidsplaner på Indstillinger, Tidsplaner): lad den stå tom for at kopiere efter hver lokal sikkerhedskopi, eller sæt en kadence (for eksempel `weekly Sun 03:00`) for at kopiere sjældnere, end du sikkerhedskopierer. **Kopiér nu** i domænets række kører den efter behov.
- **Opbevaring pr. sted.** Hvert sted har sine egne regler, så et off-site-sted kan beholde kopier længere som et arkiv. Et sted, hvor alle regler står på nul, beskærer aldrig noget.
- **Båndbreddegrænser** pr. sted begrænser restics upload- og downloadhastighed, så kopiering ikke mætter dit WAN.
- En **replikeringsindikator** viser, hvilket domæne der kopierer, mens det kører (på dets side og på Oversigten). Det er en aktiv indikator, ikke en procentbjælke, fordi `restic copy` ikke eksponerer nogen maskinlæsbar fremdrift.

!!! note "Gendan fra ethvert sted"
    Hver container, VM, mappesæt, flashen og appkonfigurationen lister deres sikkerhedskopier som én tidslinje på tværs af alle steder, en sikkerhedskopi ligger. En sikkerhedskopi kopieret til B2 vises én gang, markeret med hvert sted, der holder den. En gendannelse tager det første sted, den kan nå, startende med det arkiv, elementet skrives til, og du kan vælge et andet sted pr. række. Eksterne steder læses kun, når du åbner dem. At slette ét sted tjekker først de andre og siger, om det var den sidste kopi.

## Placering pr. element {#placement}

Hvert container-, VM- og mappesæt-kort har en række **Placering** med tre segmenter:

- **Lokal** skriver elementet til det arkiv, der vises under **Gemt på**, og kopierer det ingen steder. Brug det til data, der allerede har en anden kopi, for eksempel en deling, der ligger på et NAS.
- **Lokal + ekstern** skriver det også dertil og kopierer det til de destinationer, der er markeret under **Kopiér til**, én chip pr. off-site-destination i domænet. Fjern et flueben, og den destination får ikke længere noget nyt fra dette element.
- **Kun ekstern** skriver elementet direkte til stedet under **Send til**, et hvilket som helst andet sted end domænets hjemsted. Kopieres domænet allerede til det sted, får elementet et direkte arkiv ved siden af kopierne; ellers opretter BombVault et arkiv til domænet der.

Placeringen er fast fra elementets første sikkerhedskopi, fordi BombVault aldrig flytter sikkerhedskopier mellem arkiver. Kopierne kan ændres når som helst. En destination, der ikke længere får et element, beholder de kopier, den har, og beskærer dem til sin egen opbevaring ved domænets næste off-site-kørsel; **Slet i B2** på kortet fjerner dem med det samme. Findes nogle af de kopier ingen andre steder, lister bekræftelsen dem efter dato og spørger om elementets navn. Der kan ikke slettes fra append-only-destinationer.

Under rækken viser kortet, hvor elementet går hen, og hvad der faktisk er der: hvor mange lokationer det holdes på, hvornår hver destination sidst blev set, og om 3-2-1 er opfyldt. En lokation er serveren med de originale data og hvert sted på en anden lokation (se [Uden for bygningen](#off-the-premises-mark)). BombVault tjekker kopier og lokationer; det tjekker ikke "to medier"-delen af 3-2-1.

### Standarder pr. domæne

Kortet **Domæner** under Indstillinger, Lagring har én række pr. domæne. **Kopieret til** gælder med det samme for hvert element uden eget valg og for projektmapperne i Compose-stacks. Når et domæne har sikkerhedskopier, gælder **Gemt på** for et nyt element ved dets første sikkerhedskopi, og at ændre det flytter ingen sikkerhedskopier. Før ændringen gemmes, nævner rækken hvert sted, der vinder eller mister elementer, og hvor mange øjebliksbilleder det betyder, og spørgsmålet har kontakten **Anvend på elementer uden sikkerhedskopier**, som også sætter hvert element uden sikkerhedskopi endnu på den nye standard. **Undtagelser** lister elementerne med deres eget valg.

Sætter du flueben ved et nyt sted under **Kopieret til**, modtager det hvert element, der ikke er sat til Lokal. Bekræftelsen angiver, hvor mange elementer det er, og hvor det er kendt, hvor meget historik det udgør.

### Direkte arkiver

Vælger du under Kun ekstern et sted, som domænet allerede kopieres til, spørger BombVault én gang og opretter derefter et direkte arkiv ved siden af kopierne, for eksempel `s3:https://s3.eu-central-003.backblazeb2.com/bucket/container-direct`, og peger elementet på det. For et kopimål uden sted åbner valget en dialog med en foreslået adresse og en forbindelsestest, der ikke opretter noget, og **Opret og brug** opretter arkivet. Et direkte arkiv overtager stedets nøgle, lagringsklasse, grænser, append-only-indstilling og opbevaring og ændrer sig med dem. Kan en ny nøgle til stedet ikke åbne det, beholder det direkte arkiv den nøgle, det har, og det noteres ved gemning. Dets øjebliksbilleder bærer mærket `bv:direct`, og alle andre opbevaringskørsler lader dem stå, så et direkte arkiv, der har mistet forbindelsen til sit sted, aldrig ældes efter de lokale regler. En B2-nøgle, der er begrænset til én mappe, skal dække stedets adresse og ikke kun domænets mappe, ellers kan den ikke nå mappen ved siden af.

### Uden for bygningen {#off-the-premises-mark}

En kopi tæller kun som en lokation for sig, når dens sted er på en anden lokation. Et cloudsted tæller altid, og en mappe på denne Unraid gør det aldrig; for et NAS, en rest-server eller en SFTP-server besvarer du **Hvor står enheden?** i stedets detaljer med **Her i huset** eller **På en anden lokation**. Svaret tæller kun med i lokationer og 3-2-1 på kortene og på Oversigten. Det ændrer ingen kopi.

### Efter en genopbygning

Kopivalg lever i BombVaults egne indstillinger. Efter en genopbygning via Opdag sikkerhedskopier uden et gendannet `/config` er de væk, og at kopiere alt ville sende de elementer, du havde udeladt, til B2 igen. Off-site-replikeringen af hvert genopbygget domæne sættes derfor på pause. Oversigten viser det i gult, og domænets række på kortet Domæner tilbyder **Bekræft standard** med en forhåndsvisning af, hvad næste kørsel kopierer, og navnene i de sikkerhedskopier, der ikke har noget element, som du kan udelade der. Kun bekræftelsen afslutter pausen; at importere en indstillingsfil bringer regler og standarder tilbage, men afslutter ikke pausen.

## Et domæne gemt på et fjernt sted {#remote-primary-repositories}

Et domæne behøver ikke at blive gemt lokalt. Så længe dets sikkerhedskopisti ikke rummer nogen sikkerhedskopier, kan du vælge et fjernt sted under **Gemt på** på kortet Domæner, og domænet sikkerhedskopierer så direkte dertil, uden lokal kopi og uden kopitrin. Fjernarkivet er da den eneste kopi, medmindre domænet også kopieres til et andet sted. Hvert fjernt sted har de samme sikkerhedsforanstaltninger:

- **En forbindelsestest**, før der skrives noget.
- **Båndbreddegrænser** for selve sikkerhedskopien, de samme flag `--limit-upload` og `--limit-download`, som en kopi bruger.
- **Append-only-beskyttelse**, efterprøvet med den samme aktive manipulationstest. Er den slået til, beskærer BombVault aldrig arkivet, fordi legitimationsoplysningerne på denne maskine ikke må kunne slette sikkerhedskopiens eneste kopi.
- **Et vækstbudget**, taget fra den samme udvikling i størrelse, som Lagerkortet følger.

Et domæne gemt på et fjernt sted er kilden til sine kopier ligesom et lokalt; se [Kopier mellem steder med forskellige legitimationsoplysninger](storage-places.md#different-credentials).

!!! note "Legitimationsoplysninger hører til stedet"
    Et fjernt sted har sine egne legitimationsoplysninger. Et sted, der er sat op med de fælles cloud-legitimationsoplysninger, bliver ved med at bruge dem, indtil dets adgang ændres i dets detaljer.

## Uforanderlig (append-only) off-site

Flag et off-site-repo append-only, så ransomware eller en kompromitteret vært ikke kan slette eller omskrive dine sikkerhedskopier. Den anden side (en `restic/rest-server`, der kører i `--append-only`-tilstand) **håndhæver** det. BombVault **verificerer** det kun altid og viser aldrig grønt alene på en konfigurationspåstand.

Vinduet **Tilføj sted** har en opskrift, klar til at indsætte, på en rest-server i append-only-tilstand med én bruger til denne BombVault. På et rest-server-sted med **Append-only** slået til kører **Test append-only-beskyttelse** i stedets detaljer manipulationstesten mod hver domænesti, hver slået-til kopi og hvert arkiv på stedet og giver ét svar for stedet, så append-only off-site er tilgængelig uden manuel redigering af configs.

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

På et sted afprøver **Test append-only-beskyttelse** hver domænesti, hver slået-til kopi og hvert arkiv der med deres egne legitimationsoplysninger og samler resultaterne i ét svar: et enkelt arkiv, der accepterer en sletning, gør hele stedet til *sletninger accepteret*.

## DR-øvelser

BombVault tilbyder to niveauer af bevis for, at dine sikkerhedskopier faktisk kan gendannes, ikke bare er til stede.

- **Gendannelses-verifikationsøvelser (lokal).** BombVault kører periodisk `restic check --read-data-subset` (afgrænset, aldrig en disk-fyldende fuld gendannelse) og viser et *sidst verificeret gendannelig*-badge pr. domæne. Kadencen lever på Indstillinger, Tidsplaner; badge't på Indstillinger, Integritet.
- **DR-øvelser (off-site).** BombVault gendanner et rigtigt mål fra off-site-repoet ind i en engangs-sandkasse, verificerer det fil-for-fil og byte-for-byte, og rydder så op. Dette beviser, at du kan gendanne fra off-site, ikke bare at repoet svarer. Kun steder på en anden lokation øves, fordi en kopi i samme hus intet beviser om at miste huset. Et domæne, der kopieres til flere af dem, øves mod ét pr. planlagt kørsel, på skift, og Oversigten nævner stedet for den seneste øvelse.

**Ransomware-beskyttelses-scorekortet** på Oversigten samler dette til en grøn / gul / rød position pr. domæne, med en aldersstemplet tjekliste (off-site konfigureret, append-only verificeret, replikering aktuel, gendannelsesøvelse bestået, kryptering til, beskæringsstrategi sat). Hver rød række dyb-linker til rettelsen, og kortet bliver kun nogensinde grønt på verificerede fakta.

## Modtager-dashboard (den modtagende side)

![Den modtagende side, overvåget skrivebeskyttet, med en integritetskontrol kørt på denne maskine.](assets/screenshots/receiver.png)

*Den modtagende side, overvåget skrivebeskyttet, med en integritetskontrol kørt på denne maskine.*

Alt ovenstående er den *afsendende* side. På den boks, der **modtager** uforanderlige off-site-kopier fra en anden BombVault, giver modtager-dashboardet dig uafhængig, skrivebeskyttet overvågning af disse repositorier på den modtagende hardware, så en tavs fejl i den anden ende ikke går ubemærket hen.

Slå **Receiver**-omskifteren til i Indstillinger for at afsløre en **Receiver**-fane. Den er som standard fra; aktivér den kun på en boks, der faktisk modtager uforanderlige off-site-sikkerhedskopier. Registrer så et modtaget repository (skrivebeskyttet, åbnet med den afsendende instans' nøgle) for at få:

- **Et øjebliksbillede-inventar grupperet efter kilde**, så du kan se præcis, hvilke containere, VM'er og filsæt der er landet.
- **Sidst-modtaget** pr. kilde, så du ved, hvor frisk hver enkelt er.
- **Et uafhængigt `restic check`** kørt på den modtagende hardware, så integritet verificeres, hvor data faktisk sidder, ikke kun på afsenderen.
- **En dødmandsknap:** en advarsel, når en kilde holder op med at sende inden for et vindue, du sætter.
- **Integritetsadvarsler:** en advarsel, når et tjek på den modtagende side fejler.

Modtageren er strengt skrivebeskyttet. Den skriver aldrig til det modtagne repository, så den kan aldrig bryde append-only-garantien, afsenderen forlader sig på.

## Gennemgået eksempel: to Unraid-maskiner, hele vejen

Ovenfor beskrives delene. Her er én komplet opsætning med rigtige værdier, for dele er nemmere at samle, når man har set dem samlet én gang.

To maskiner: **TOWER** kører containerne og sender sikkerhedskopierne, **VAULT** modtager dem og håndhæver uforanderligheden. Udskift med dine egne navne, adresser og delingsstier.

**1. Rejs append-only-serveren på VAULT.** I BombVault på TOWER: åbn *Indstillinger → Lagring*, klik på **Tilføj sted**, vælg **rest-server**, og klik på **Vis opskrift**. Kopiér blokken **Unraid-skabelon**, gem den på VAULT som `/boot/config/plugins/dockerMan/templates-user/my-rest-server.xml`, og vælg derefter *Docker → Add Container* og **rest-server** fra skabelonlisten. Skriv den viste `htpasswd`-linje ind i `/mnt/user/appdata/rest-server/.htpasswd` på VAULT, før du starter den. Adgangskoden vises én gang og gemmes aldrig; opskriften har allerede sat den og brugeren ind i formularen på TOWER, så lad det vindue stå åbent. `htpasswd`-linjen bærer den samme adgangskode, allerede bcrypt-hashet for dig, så du skal ikke hashe noget selv.

    Lad `--append-only` blive stående i OPTIONS-feltet. Uden det er VAULT bare en almindelig deling igen.

**2. Tilføj stedet på TOWER.** Indtast VAULTs adresse, `http://VAULT:8000`, ved siden af den bruger og adgangskode, opskriften har udfyldt, og klik derefter på **Test forbindelse**. BombVault bygger adressen ud fra dem:

    rest:http://VAULT:8000/tower

Første led i stien er htpasswd-brugeren, her `tower`, og hvert domæne får sin mappe under den, for eksempel `rest:http://VAULT:8000/tower/container`. Besvar **Hvor står enheden?** med **På en anden lokation**, klik på **Tilføj**, og sæt flueben ved stedet under **Kopieret til** for de domæner, der skal derhen.

**3. Slå Append-only til på TOWER** under **Beskyttelse** i stedets detaljer, og klik derefter på **Test append-only-beskyttelse**. Testen afprøver hver domænesti, hver kopi og hvert arkiv på stedet og giver ét svar for stedet, som skal være *sletninger afvist*. Hvad svarene betyder:

| Resultat | Hvad der skete |
| --- | --- |
| **sletninger afvist** | VAULT afviste sletningen. Det er den eneste beståede tilstand. |
| **sletninger accepteret** | VAULT accepterede en sletning. `--append-only` mangler eller er fjernet. |
| en besked i stedet for et resultat | Testen kunne ikke køre. Som regel er adressen ikke den, restic selv bruger, eller legitimationen er ændret. Intet registreres, og ingen advarsel udløses. |

**4. Se på VAULT, hvad der kommer ind.** Slå *Indstillinger → Modtager* til, åbn fanen **Modtager**, og registrér arkivet skrivebeskyttet.

!!! warning "Placeringen er en sti **inde i** containeren, skrevet relativt til værtsmonteringen"
    Indtast `user/appdata/rest-server/tower/container`, **ikke** `/mnt/user/appdata/…`. BombVault kører i en container, hvor værtens `/mnt` er monteret et andet sted; en absolut værtssti findes ikke derinde. Indsætter du en, fortæller BombVault dig den relative sti, du skal bruge i stedet.

    **Afsendende APP_KEY** er TOWERs nøgle, ikke VAULTs. Du finder den på TOWER under *Indstillinger → System*.

**5. Gør det gensidigt, hvis du vil.** Gentag de samme fem trin den anden vej: en rest-server på TOWER, der modtager VAULTs kopi. Så håndhæver hver maskine uforanderligheden for den anden, og ingen kan slette den andens sikkerhedskopier.

## Guidet gendannelse

En dedikeret **Recovery**-fane fører en frisk eller genopbygget installation gennem katastrofetilfældet, ét sted:

1. **Tjekker, at BombVault kan læse dine sikkerhedskopier** (krypteringsnøgle-faldgruben på forkant).
2. **Gendanner BombVaults egne indstillinger**, så de sikkerhedskopi-stier, off-site-destinationer og legitimationsoplysninger, resten af forløbet har brug for, er forudfyldte. Indstillingssikkerhedskopien læses fra det sted, som rækken Auto-sikkerhedskopi angiver under **Gemt på**, eller fra Auto-sikkerhedskopiens kopi under **Kopieret til**, og stedet vises med sin adresse; vil du læse fra et andet sted, så ændr først rækken Auto-sikkerhedskopi i trin 3. Gendannelsen anvendes via en selv-genstart over Docker-socket'en, så den kørende indstillingsdatabase aldrig overskrives under et åbent handle.
3. **Tilknytter dine eksisterende sikkerhedskopier** via rækkerne på kortet Domæner: i hvert domænes række vælger du stedet, hvor dets sikkerhedskopier ligger, under **Gemt på** og stederne med dets kopier under **Kopieret til**. Et sted, som ingen række tilbyder endnu, for eksempel en deling, en server eller en cloud-bucket, forbindes med **Tilføj sted**, det samme vindue som på Indstillinger, Lagring. **Opret forbindelse & forhåndsvis** tjekker derefter, at sikkerhedskopierne kan læses.
4. **Opdager** de containere, VM'er og filsæt, der er gemt i det.
5. **Gendanner dem alle** (efterladt stoppet, så du starter dem bevidst), med dit gendannelseskit et klik væk.

!!! note "Eksterne kopier venter efter en genopbygning"
    Når trin 4 genopbygger elementer uden de gamle indstillinger, sættes off-site-replikeringen af de domæner på pause, indtil standardplaceringen er bekræftet. Se [Placering pr. element](#placement).

!!! tip "Planlagt migrering versus katastrofe"
    Guidet gendannelse gendanner BombVaults egne indstillinger fra en sikkerhedskopi. For et *planlagt* flyt til en ny boks kan du i stedet bære din konfiguration over direkte med kortet **Eksportér og importér indstillinger** (en bærbar JSON-fil). Se [Konfiguration](configuration.md#portable-settings-export-and-import).

### Gendan fra et andet BombVault-repo

Et separat kort på **Recovery**-fanen åbner et *andet* BombVault-instans' repo (en share monteret under `/mnt`, eller en remote-URL) med **den instans' `APP_KEY`**, i en engangs, skrivebeskyttet session. Gennemse de containere, VM'er og filsæt, der er gemt der, vælg et øjebliksbillede og gendan det, og det gendannede objekt bliver en normal lokal container, VM eller filsæt. Intet skrives nogensinde til det andet repo, og dine egne sikkerhedskopiindstillinger forbliver urørte (sessionen lever i hukommelsen og udløber af sig selv). At flytte en container fra server A til server B betyder ikke længere at ompege dine repo-indstillinger og tilbageføre dem bagefter. Live server-til-server-federation er eksplicit uden for scope; dette er et bevidst engangstræk.

## Gendannelseskit til krypteringsnøglen

Dette er den brik, der gør katastrofegendannelse mulig, selv når der ikke er nogen kørende BombVault.

Ét klik downloader **hovednøglen**, den **afledte restic-adgangskode** og de **præcise repo-placeringer og kommandoer**, så du kan gendanne direkte med restic-CLI'en på en hvilken som helst maskine. En Oversigts-påmindelse nager, indtil du har gemt det.

!!! danger "Opbevar gendannelseskittet uden for serveren"
    Kittet indeholder hemmeligheden, der dekrypterer dine sikkerhedskopier. Hold det et sikkert sted adskilt fra serveren (en adgangskodemanager, en printet kopi i en boks). Hvis du mister både BombVault og `APP_KEY` uden noget gendannelseskit, kan dine krypterede sikkerhedskopier ikke gendannes.

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
