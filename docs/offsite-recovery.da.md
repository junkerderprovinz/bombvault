# Off-site og gendannelse

Lokale sikkerhedskopier beskytter dig mod en tabt container eller en dårlig opdatering. Off-site-replikering og et testet gendannelseskit beskytter dig mod hele boksen, ransomware eller en brand. Denne side dækker replikering off-site, at gøre den kopi manipulationssikker, at bevise at du kan gendanne, og at gendanne, når BombVault selv er væk.

## Off-site-replikering

Behold den hurtige lokale sikkerhedskopi, og tilføj en eller flere off-site-replikaer. Sæt et repo pr. domæne på fanen **Indstillinger, Off-site**. BombVault replikerer nye øjebliksbilleder dertil med `restic copy` på et best-effort-grundlag, så et off-site-hikke aldrig får den lokale sikkerhedskopi til at fejle. Det lokale repo forbliver primært.

- **Flere off-site-destinationer pr. domæne.** Hvert domæne (containere, VM'er, flash, config og filsæt) kan replikere til flere off-site-destinationer på én gang, ikke kun én, så du kan beholde for eksempel en rest-server på en vens boks og en S3-bucket parallelt. Tilføj ekstra destinationer på Indstillinger, Off-site, hver med sit eget repository, sin S3-lagringsklasse, sit append-only-flag, sin opbevaring og sit vækstbudget. En eksisterende enkelt off-site-opsætning overføres urørt som den første destination, og hver destination i et domæne replikerer på det domænes off-site-tidsplan.
- **Off-site-tidsplan pr. domæne** (redigeret sammen med alle andre tidsplaner på Indstillinger, Tidsplaner): lad den stå tom for at replikere efter hver lokal sikkerhedskopi, eller sæt en kadence (for eksempel `weekly Sun 03:00`) for at sende off-site sjældnere, end du sikkerhedskopierer lokalt. En **Replikér nu**-knap dækker on-demand-kørsler.
- **Off-site-opbevaring** lever på Indstillinger, Off-site, så du kan beholde off-site-kopier længere som et arkiv. Lad politikken stå helt-nul for aldrig at auto-trimme off-site-øjebliksbilleder.
- **Båndbreddegrænser** (Indstillinger, Off-site) begrænser restic-upload/download-hastigheden, så replikering ikke mætter dit WAN.
- En **replikeringsindikator** viser, hvilket domæne der replikerer, mens det kører (på dets side og på Oversigten). Det er en aktiv indikator, ikke en procentbjælke, fordi `restic copy` ikke eksponerer nogen maskinlæsbar fremdrift.

!!! note "Gendan fra ethvert sted"
    Hver container, VM, mappesæt, flashen og appkonfigurationen lister deres sikkerhedskopier som én tidslinje på tværs af alle steder, en sikkerhedskopi ligger. En sikkerhedskopi kopieret til B2 vises én gang, markeret med hvert sted, der holder den. En gendannelse tager det første sted, den kan nå, startende med det arkiv, elementet skrives til, og du kan vælge et andet sted pr. række. Eksterne steder læses kun, når du åbner dem. At slette ét sted tjekker først de andre og siger, om det var den sidste kopi.

## Placering pr. element {#placement}

Hvert container-, VM- og mappesæt-kort har en række **Placering** med tre segmenter:

- **Lokal** skriver elementet til det arkiv, der vises under **Gemt på**, og kopierer det ingen steder. Brug det til data, der allerede har en anden kopi, for eksempel en deling, der ligger på et NAS.
- **Lokal + ekstern** skriver det også dertil og kopierer det til de destinationer, der er markeret under **Kopiér til**, én chip pr. off-site-destination i domænet. Fjern et flueben, og den destination får ikke længere noget nyt fra dette element.
- **Kun ekstern** skriver elementet direkte til stedet under **Send til**: et direkte arkiv ved siden af en off-site-destination, eller et fjernarkiv, du sætter op under Indstillinger, Stier og lagring, Depoter.

Placeringen er fast fra elementets første sikkerhedskopi, fordi BombVault aldrig flytter sikkerhedskopier mellem arkiver. Kopierne kan ændres når som helst. En destination, der ikke længere får et element, beholder de kopier, den har, og beskærer dem til sin egen opbevaring ved domænets næste off-site-kørsel; **Slet i B2** på kortet fjerner dem med det samme. Findes nogle af de kopier ingen andre steder, lister bekræftelsen dem efter dato og spørger om elementets navn. Der kan ikke slettes fra append-only-destinationer.

Under rækken viser kortet, hvor elementet går hen, og hvad der faktisk er der: hvor mange lokationer det holdes på, hvornår hver destination sidst blev set, og om 3-2-1 er opfyldt. En lokation er serveren med de originale data, hver off-site-destination og hvert arkiv markeret **Uden for bygningen**. BombVault tjekker kopier og lokationer; det tjekker ikke "to medier"-delen af 3-2-1.

### Standardplaceringer

Indstillinger, Stier og lagring, **Standardplaceringer** har en række pr. domæne med de samme tre segmenter. Kopierne gælder med det samme for hvert element uden eget valg, og for projektmapperne i Compose-stacks. Placeringen gælder for et nyt element ved dets første sikkerhedskopi; at ændre den flytter ingen sikkerhedskopier. Før den gemmes, navngiver rækken hver destination, der vinder eller mister elementer, og hvor mange øjebliksbilleder det betyder. **Anvend på elementer uden sikkerhedskopier** sætter hvert element uden nogen sikkerhedskopi endnu tilbage på standarden.

En ny off-site-destination modtager hvert element, der ikke er sat til Lokal. Dialogen, der tilføjer den, angiver, hvor mange elementer det er, og hvor det er kendt, hvor meget historik det udgør, og tilbyder at udelade de elementer, der allerede er udelukket fra andre destinationer.

### Direkte arkiver

At vælge en destinations direkte arkiv under Kun ekstern åbner en dialog med en foreslået placering ved siden af destinationen, for eksempel `b2:bucket:containers-direct`, og en forbindelsestest, der ikke opretter noget. **Opret og brug** opretter arkivet og peger elementet på det. Et direkte arkiv overtager destinationens nøgle, lagringsklasse, grænser, append-only-indstilling og opbevaring og ændrer sig med dem; Depoter-kortet viser det skrivebeskyttet. Kan en ny nøgle til destinationen ikke åbne det, beholder det direkte arkiv den nøgle, det har, og det noteres ved gemning. Dets øjebliksbilleder bærer mærket `bv:direct`, og alle andre opbevaringskørsler lader dem stå, så et direkte arkiv, der har mistet forbindelsen til sin destination, aldrig ældes efter de lokale regler. En B2-nøgle, der er begrænset til destinationens egen mappe, kan ikke nå mappen ved siden af den; begræns i stedet nøglen til mappen over destinationen.

### Uden for bygningen

Et navngivet arkiv kan markeres **Uden for bygningen** på Depoter-kortet. Fjernarkiver starter markeret; slå det fra for en rest-server i samme bygning. Markeringen tæller kun med i lokationer og 3-2-1 på kortene. Den ændrer ingen kopi.

### Efter en genopbygning

Kopivalg lever i BombVaults egne indstillinger. Efter en genopbygning via Opdag sikkerhedskopier uden et gendannet `/config` er de væk, og at kopiere alt ville sende de elementer, du havde udeladt, til B2 igen. Off-site-replikeringen af hvert genopbygget domæne sættes derfor på pause. Oversigten viser det i gult, og Standardplaceringer tilbyder **Bekræft standard** med en forhåndsvisning af, hvad næste kørsel kopierer, og navnene i de sikkerhedskopier, der ikke har noget element, som du kan udelade der. Kun bekræftelsen afslutter pausen; at importere en indstillingsfil bringer regler og standarder tilbage, men afslutter ikke pausen.

## Fjernbetjente primære arkiver {#remote-primary-repositories}

Et domænes sti til sikkerhedskopi (Indstillinger, Stier og lager) er ikke begrænset til en lokal mappe: peg den direkte på et restic-fjernarkiv (`s3:...`, `rest:http://vært:8000/arkiv`, `b2:...`, `sftp:bruger@vært:/arkiv`, `rclone:fjern:bucket/sti`), så sikkerhedskopierer BombVault direkte dertil, uden separat lokal kopi og uden replikeringstrin. Det er en virkelig anden form end off-site-replikeringen ovenfor: dér er det lokale arkiv det primære, og off-site-arkivet er et arkiv af det efter bedste evne; her **er** fjernarkivet det primære, og det er den eneste kopi, så længe du ikke også opsætter off-site-replikering (eller et andet fjernarkiv) for det domæne.

Hvert af de fem stifelter (Containere, Virtuelle maskiner, Flash, Konfiguration, Filer) har en kontakt **Lokal / Fjern** lige ved siden af:

- **Lokal** viser den velkendte mappebrowser.
- **Fjern** bytter den ud med et almindeligt URL-felt plus en knap, der åbner den samme dialog til forbindelsestest og adgangsoplysninger, som off-site-destinationer bruger, blot indstillet til dette primære arkiv. Derfra får du:
    - **En forbindelsestest** mod den rigtige sti, før du forlader dig på den.
    - **Båndbreddegrænser** (upload og download), så en planlagt sikkerhedskopi til et fjernprimært arkiv ikke mætter din WAN-forbindelse: de samme restic-flag `--limit-upload` og `--limit-download`, som off-site-replikeringen bruger, nu anvendt på selve sikkerhedskopien.
    - **Append-only-beskyttelse (uforanderlighed)**, efterprøvet med den samme aktive manipulationstest (en rigtig DELETE-sonde mod den anden ende), som off-site-destinationer får. Er den slået til, nægter BombVault selv at beskære arkivet: da der ikke står en separat lokal kopi bag, må adgangsoplysningerne på denne maskine ikke kunne slette sikkerhedskopiens eneste kopi.
    - **En alarm for vækstbudgettet**, taget fra den samme udvikling i arkivets størrelse, som Lagerkortet allerede følger.

Intet af dette er påkrævet: en håndskrevet fjernsti uden gemte sikkerhedsindstillinger sikkerhedskopierer nøjagtig som før (ubegrænset båndbredde, kan beskæres, ingen budgetalarm). Sikkerhedsdialogen er der til, når du vil have den samme beskyttelse, som en off-site-kopi får, uden at skulle oprette en off-site-destination alene af den grund.

!!! note "Sky- og REST-adgangsoplysninger deles"
    Et fjernprimært arkiv godkendes med de samme S3-/REST-adgangsoplysninger, der er sat op under Indstillinger, Off-site, Skyadgangsoplysninger. Der findes ikke et separat sted til adgangsoplysninger for primære arkiver.

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

- **Gendannelses-verifikationsøvelser (lokal).** BombVault kører periodisk `restic check --read-data-subset` (afgrænset, aldrig en disk-fyldende fuld gendannelse) og viser et *sidst verificeret gendannelig*-badge pr. domæne. Kadencen lever på Indstillinger, Tidsplaner; badge't på Indstillinger, Integritet.
- **DR-øvelser (off-site).** BombVault gendanner et rigtigt mål fra off-site-repoet ind i en engangs-sandkasse, verificerer det fil-for-fil og byte-for-byte, og rydder så op. Dette beviser, at du kan gendanne fra off-site, ikke bare at repoet svarer.

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

**1. Rejs append-only-serveren på VAULT.** I BombVault på TOWER: gå til *Indstillinger → Eksternt → guidet opsætning*, vælg **rest-server** og generér opskriften. Kopiér fanen **Unraid-skabelon (XML)**, gem den på VAULT som `/boot/config/plugins/dockerMan/templates-user/my-rest-server.xml`, og vælg derefter *Docker → Add Container* og **rest-server** fra skabelonlisten. Skriv den viste `htpasswd`-linje ind i `/mnt/user/appdata/rest-server/.htpasswd` på VAULT, før du starter den. Engangsadgangskoden vises én gang og gemmes aldrig, så kopiér den nu. Den linje bærer den samme adgangskode, allerede bcrypt-hashet for dig: klarteksten hører til i REST-legitimationsoplysningerne på TOWER, den hashede linje i `.htpasswd` på VAULT. Du skal ikke hashe noget selv.

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

**4. Se på VAULT, hvad der kommer ind.** Slå *Indstillinger → Modtager* til, åbn fanen **Modtager**, og registrér arkivet skrivebeskyttet.

!!! warning "Placeringen er en sti **inde i** containeren, skrevet relativt til værtsmonteringen"
    Indtast `user/appdata/rest-server/bombvault-containers/containers`, **ikke** `/mnt/user/appdata/…`. BombVault kører i en container, hvor værtens `/mnt` er monteret et andet sted; en absolut værtssti findes ikke derinde. Indsætter du en, fortæller BombVault dig nu den relative sti, du skal bruge i stedet.

    **Afsendende APP_KEY** er TOWERs nøgle, ikke VAULTs. Du finder den på TOWER under *Indstillinger → System*.

**5. Gør det gensidigt, hvis du vil.** Gentag de samme fem trin den anden vej: en rest-server på TOWER, der modtager VAULTs kopi. Så håndhæver hver maskine uforanderligheden for den anden, og ingen kan slette den andens sikkerhedskopier.

## Guidet gendannelse

En dedikeret **Recovery**-fane fører en frisk eller genopbygget installation gennem katastrofetilfældet, ét sted:

1. **Gendanner BombVaults egne indstillinger først**, så de sikkerhedskopi-stier, off-site-destinationer og legitimationsoplysninger, resten af forløbet har brug for, er forudfyldte (anvendt via en selv-genstart over Docker-socket'en, så den kørende indstillingsdatabase aldrig overskrives under et åbent handle).
2. **Tjekker, at BombVault kan læse dine sikkerhedskopier** (krypteringsnøgle-faldgruben på forkant).
3. Lader dig **pege mod dit eksisterende repo** (lokalt eller off-site).
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
