# ZFS-datasæt

Siden **ZFS** tager sikkerhedskopier af ZFS-datasæt. Et element er ét datasæt sammen med alle datasæt under det. Til hver sikkerhedskopi tager BombVault ét ZFS-snapshot af hele træet, så hvert datasæt i det fanges i samme øjeblik. Derefter læser det filerne i hvert datasæt fra det snapshot, gemmer dem med restic på samme måde som en mappe og fjerner snapshottet lige bagefter. Sikkerhedskopierne er deduplikerede, du kan gennemse hver enkelt, og enkelte filer kan gendannes.

Sikkerhedskopien sender aldrig en strøm ind i restic og ruller aldrig et datasæt tilbage. BombVault sletter kun snapshots, som det selv har oprettet. Den valgfri [replika](#replica) er det eneste sted, der bruger `zfs send`: den kopierer datasættene til en anden ZFS-server og rører ikke sikkerhedskopien.

## Krav {#requirements}

- **SSH-forbindelsen til denne server.** ZFS-datasæt bruger samme nøgle, vært og bruger som VM-sikkerhedskopier. Virker VM-sikkerhedskopier allerede, virker dette også. Ellers følg [guiden til VM-sikkerhedskopi over SSH](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) på GitHub. Skabelonfelterne hedder **Host SSH: Address**, **Host SSH: Port** og **Host SSH: User**.
- **Kommandoen `zfs` på den vært.** Unraid 6.12 og nyere og TrueNAS SCALE har den.
- **Host Data mappet som `/mnt` med Access Mode Read/Write - Slave.** Det er skabelonens standard. Snapshottet af et datasæt dukker først op i datasættets mappe `.zfs/snapshot`, efter at BombVault er startet, så containeren skal modtage de monteringer, værten laver senere.
- **Datasættene monteret under `/mnt`.** På Unraid ligger pools under `/mnt/<pool>`, så det er allerede tilfældet.

Slå domænet til under **Indstillinger, Generelt** (ZFS-datasæt). ZFS-siden viser så kortet **Forbindelse til denne server**. Det tester SSH-forbindelsen, nævner den bruger og vært, det forbinder til, og siger, hvad der mangler, når noget mangler. Værtsintegrationstjekket (`/spike`) viser samme resultat.

## Elementer og underdatasæt {#items-and-children}

Åbn **Tilføj datasæt** på ZFS-siden. Listen kommer fra serveren. Vælg det datasæt, der ligger øverst i det, du vil sikkerhedskopiere, for eksempel `cache/appdata`, og elementet dækker det og alle datasæt under det.

- **Nye underdatasæt kommer selv med.** Et datasæt, der senere oprettes under elementet, sikkerhedskopieres ved næste kørsel, og den kørsel nævner det som nyt. Dets første sikkerhedskopi læser det helt én gang; derefter læses kun ændringer.
- **Du kan udelade enkelte underdatasæt.** Slå et fra i elementets indstillinger, så udelades det sammen med alt under det. Et udeladt underdatasæt, der ikke længere findes på serveren, markeres som sådan og kan fjernes fra listen.
- **Underdatasæt, der ikke kan læses, springes over, aldrig i stilhed.** Kørslen oplister dem, elementet viser, hvor mange der blev sprunget over, og dækningskortet på oversigten tæller hvert af dem som ubeskyttet. Kørslen sikkerhedskopierer alligevel alt andet og fejler ikke på grund af et oversprunget underdatasæt. Årsagerne står i [tabellen over årsagskoder](#reason-codes): et datasæt, der ikke er monteret, har `canmount=off`, et `legacy`- eller intet monteringspunkt, en krypteringsnøgle, der ikke er indlæst, slået snapshotadgang fra eller et monteringspunkt, BombVault ikke kan se.
- **Et oversprunget datasæt tager ikke sine underdatasæt med.** Et datasæt med `canmount=off`, der kun rummer andre datasæt, springes over (vist som "kun struktur"), og dets monterede underdatasæt sikkerhedskopieres. Et krypteret datasæt, hvis nøgle ikke er indlæst, springes over sammen med de underdatasæt, der deler dets nøgle.
- **Underdatasæt, der er VM-diske eller systemdata, starter slået fra** i tilføjelsesdialogen, med årsagen ved siden af kontakten. Tilføjer du en hel pool, skal du bekræfte det i en dialog, der oplister, hvad den indeholder.

### Volumener {#volumes}

Et volumen (zvol) indeholder en virtuel disk i stedet for filer, og ZFS-siden sikkerhedskopierer aldrig et.

- Et volumen, som en VM bruger, sikkerhedskopieres med den VM på siden **VMs**.
- Et volumen, som ingen VM bruger (et iSCSI-extent, en disk du har koblet fra), **sikkerhedskopieres ikke af BombVault**. Tilføjelsesdialogen og ZFS-siden tæller disse volumener og siger det. En senere version vil sikkerhedskopiere dem.

Volumener i et elements træ springes over og nævnes ved hver kørsel.

### Dockers lager {#docker-storage}

Med Dockers ZFS-lagerdriver er hvert billedlag et datasæt med et `legacy`-monteringspunkt. Tilføjelsesdialogen samler dem i én linje pr. overordnet datasæt. Et træ med mere end 20 sådanne datasæt kan ikke blive et element: Så længe der findes et snapshot af det, kan Docker ikke fjerne billedlag. Tilføj i stedet datasættene under det, for eksempel `appdata`.

### Elementer overlapper aldrig {#overlap}

Et datasæt kan kun høre til ét element. BombVault afviser et nyt element, der ligger inde i et eksisterende eller ville indeholde et. For at slå flere underelementer sammen til ét overordnet element skal du først slette underelementerne og vælge at beholde deres sikkerhedskopier og derefter tilføje det overordnede. Hvert datasæt beholder sin historik under sit eget navn, så næste sikkerhedskopi fortsætter, hvor de gamle elementer slap, og læser ikke alt igen.

## Stop containere og kør kommandoer omkring snapshottet {#consistency}

Et snapshot af en kørende database er som et pludseligt strømsvigt: Databasen kommer sig som regel, men den skal gøre det. Hvert element kan gøre to ting ved det, og begge dækker kun øjeblikket for snapshottet, ikke hele sikkerhedskopien.

- **Stop disse containere for snapshottet.** BombVault stopper de angivne containere, tager snapshottet og starter dem igen med det samme. Containere på samme afhængighedsniveau stopper parallelt, de afhængige først, så hele vinduet varer som regel nogle få sekunder; kørslen viser, hvor længe. Sikkerhedskopien læser derefter det frosne snapshot, mens apps allerede kører igen. Kun containere, der kørte, stoppes.
- **En kommando før og efter snapshottet.** Den kører inde i en container efter eget valg, for eksempel for at dumpe en database ind i datasættet lige før snapshottet uden at stoppe noget. Fejler kommandoen før snapshottet, fejler sikkerhedskopien, og der tages intet snapshot. En kommando efter snapshottet, der fejler, vises på kørslen, men får ikke sikkerhedskopien til at fejle.

Hvad der sker, når noget går galt:

- Kan en container ikke stoppes, starter BombVault dem, det allerede har stoppet, og sikkerhedskopien fejler med containerens navn. Det falder aldrig tilbage på et snapshot af kørende apps.
- Stoppet venter, til en igangværende containersikkerhedskopi er færdig (op til 30 minutter ved en manuel kørsel, op til sikkerhedskopiens tidsgrænse ved en planlagt), så de to aldrig stopper og starter samme container på én gang.
- Før den første container stopper, skriver BombVault ned, hvilke det stopper. Bliver BombVault dræbt inden for vinduet, starter det de containere igen næste gang det starter, sender en notifikation, og elementet viser en rød note for hver container, det ikke kunne starte.

De automatiske databasedumps (se [Funktioner](features.md)) kører med en containers egen sikkerhedskopi på siden **Containers**, ikke med et ZFS-element. En database, hvis container kun sikkerhedskopieres gennem sit datasæt, får intet dump, så giv den en kommando her.

En container kan stå på denne liste og på siden **Containers** på samme tid. Dens data gemmes så to gange, i to repositories, og **Fuld sikkerhedskopi** stopper den to gange. Elementet gør opmærksom på det.

## Gendannelse {#restore}

Åbn **Sikkerhedskopier** på elementet, vælg sikkerhedskopien og derefter datasættet. Som standard er det elementets øverste datasæt.

- **Gendan ind i datasættet.** Filer fra sikkerhedskopien skrives ind i datasættets monteringspunkt. Filer med samme navn overskrives, andre bliver liggende. Datasættet rulles aldrig tilbage eller erstattes. BombVault tjekker, at datasættet er monteret, synligt og skrivbart, én gang før det starter og igen lige før det skriver. Hvor et underdatasæt er monteret inde i det, skrives intet: Underdatasættet beholder sine filer, sin ejer og sine rettigheder og gendannes fra sin egen sikkerhedskopi.
- **Gendan til en mappe.** Vælg en mappe under `/mnt`. BombVault tjekker, at mappen ligger på en monteret pool eller share, og at der er plads nok. Det virker uden SSH-forbindelsen og for datasæt, der ikke længere findes.
- **Til et nyt datasæt.** Angiv et datasæt, der ikke findes endnu. BombVault opretter det med ZFS-egenskaberne fra sikkerhedskopien og gendanner ind i det, se [Gendan som nyt datasæt](#new-dataset).
- **Vælg filer** (avanceret): skriv kun de filer og mapper, du vælger, tilbage i datasættet.
- **Alle datasæt i denne sikkerhedskopi** (avanceret): hvert datasæt i træet i sin egen undermappe af den mappe, du vælger. Datasæt, der blev sprunget over i den sikkerhedskopi, nævnes.
- **Fra en anden server:** Siden **Gendannelse** gendanner fra en anden BombVaults repository, altid til en mappe: alle datasæt i én sikkerhedskopi, hvert i sin egen undermappe, eller ét datasæt i træet, helt eller valgte filer.

Elementets liste over containere, der skal stoppes, tilbydes også ved gendannelse ind i datasættet. De containere forbliver stoppet under hele gendannelsen, og containersikkerhedskopier venter imens.

### Sikkerhedssnapshottet {#safety-snapshot}

Før det skriver ind i et datasæt, tager BombVault et ZFS-snapshot af netop det datasæt med navnet `bombvault-prerestore-<tid>`. Det er slået til som standard; at slå det fra kræver en ekstra bekræftelse. Kan snapshottet ikke tages, gendannes intet.

BombVault sletter aldrig selv et sikkerhedssnapshot. Elementet oplister dem med alder og størrelse, hver med handlingen **Slet**, og advarer, når det ældste er mere end 30 dage gammelt, fordi det holder slettede og ændrede data fast i poolen.

For at gå tilbage efter en gendannelse kopierer du enkelte filer fra `.zfs/snapshot/bombvault-prerestore-<tid>` inde i datasættet. `zfs rollback <dataset>@bombvault-prerestore-<tid>` virker kun, så længe det er datasættets nyeste snapshot. `zfs rollback -r` sletter alle nyere snapshots, også de automatiske.

### Gendan som nyt datasæt {#new-dataset}

BombVault gemmer med hver sikkerhedskopi de lokalt satte ZFS-egenskaber for hvert datasæt: compression, recordsize, quota, reservation, atime, xattr, acltype, casesensitivity og dine egne brugeregenskaber. Nedarvede og skrivebeskyttede værdier udelades, fordi de kommer tilbage af sig selv. Sikkerhedskopier fra før BombVault gemte dem, har ingen.

- **Til et nyt datasæt** kører `zfs create` med hver gemt egenskab. casesensitivity, normalization og utf8only kan kun sættes på den måde. Kvoter og reservationer sættes først efter filerne, så de ikke kan afvise dem. Monteringspunktet udelades, så kopien ikke støder sammen med originalen, og det samme gælder `canmount`, `readonly` og krypteringen, så gendannelsen kan skrive. Et nyt datasæt under et krypteret overtager dets kryptering. Datasættet ovenover skal findes. Hvis noget fejler, efter det er oprettet, bliver det nye datasæt på serveren, fordi BombVault aldrig ødelægger et datasæt.
- **Gendan ind i datasættet** viser de gemte egenskaber ved siden af gendannelsen. **Sæt også disse egenskaber** sætter dem, et eksisterende datasæt stadig tager imod, før en eneste fil skrives. Kvoter og reservationer sættes først efter filerne, så de ikke kan afvise dem. Uden den kontakt beholder datasættet sine indstillinger.

## Hvad sikkerhedskopien indeholder {#contents}

I sikkerhedskopien: filerne og mapperne i hvert sikkerhedskopieret datasæt, med ejer, rettigheder, tidsstempler og udvidede attributter, som restic gemmer dem og de lokalt satte ZFS-egenskaber for hvert datasæt.

Ikke i sikkerhedskopien:

- ejer og rettigheder for selve den øverste mappe i hvert datasæt (alt under den er med). En gendannelse ind i datasættet lader den eksisterende øverste mappe være, som den er, en gendannelse til en mappe opretter den med rettighederne `0755`;
- eksisterende ZFS-snapshots;
- underdatasæt, der blev sprunget over eller udeladt;
- volumener.

For at gendanne til en ny pool opretter du poolen og gendanner hvert datasæt til et nyt datasæt. Om NFSv4-ACL'er, som TrueNAS bruger dem på SMB-datasæt, kommer tilbage som forventet, er endnu ikke afprøvet, så test en gendannelse med dine egne data, før du stoler på dem.

## Krypterede datasæt {#encryption}

Et krypteret datasæt sikkerhedskopieres kun, mens dets nøgle er indlæst. Ellers springes det over med en advarsel; indlæs nøglen med `zfs load-key` og montér datasættet. BombVault læser dataene dekrypteret og gemmer dem i restics repository, som er krypteret. Har du slået kryptering fra i BombVault, er det repository det ikke.

## Replika {#replica}

En replika er en kopi af et elements datasæt på en anden ZFS-server. BombVault holder den ajour med `zfs send` og `zfs receive`. Den første kørsel sender alt, bagefter rejser kun de ændrede blokke. På den anden server kan du montere kopien med det samme.

En replika erstatter aldrig sikkerhedskopien. Ældre versioner, enkelte filer og kontrollen kommer stadig fra sikkerhedskopierne, og replikaen beholder kun så mange snapshots, som du angiver. En aktuel replika tæller som en kopi uden for stedet, men et element med replika og uden sikkerhedskopi forbliver orange.

Slå den til på kortet **Replica** i elementets indstillinger. Der vælger du, hvor replikaen skal hen, hvornår den kører (**After every backup** eller **Own plan**), og hvor mange snapshots der bliver på målet. Kortet oplister hvert datasæt og hvert volumen med dets tilstand, og **Replikér nu** starter en kørsel. En replikakørsel har sin egen lås, så en lang første overførsel aldrig holder sikkerhedskopierne tilbage.

### Push til en ZFS-server {#replica-push}

Enhver maskine med ZFS og SSH kan modtage, for eksempel en anden Unraid eller en TrueNAS. BombVault behøver ikke køre dér.

1. Under **Indstillinger, Storage locations** åbner du **Add storage location** og vælger **ZFS server**.
2. Angiv adresse, bruger og port. Dialogen viser BombVaults offentlige nøgle. Føj den til brugerens `~/.ssh/authorized_keys` på serveren. På Unraid findes det under **Settings, Users, root, SSH keys**.
3. Test forbindelsen. Dialogen oplister derefter serverens pools. Vælg én og angiv roden, som som standard er `<pool>/bombvault-replica`.
4. Vælg den nye server på elementets kort **Replica**.

Med root skal der ikke mere til. En bruger af sit eget skal bruge disse rettigheder på målets pool, som dialogen også viser:

```
zfs allow <user> receive,create,mount,rollback,destroy,userprop <pool>
```

På kilden skal den samme slags bruger have disse på elementets øverste datasæt:

```
zfs allow <user> send,snapshot,hold,release,bookmark,destroy <dataset>
```

I denne retning har den BombVault, der ejer elementet, også en nøgle, der kan skrive på serveren.

### Pull fra en anden BombVault {#replica-pull}

Den anden vej henter en parret BombVault selv replikaen. Kilden har så ingen nøgle, der kan skrive eller slette på kopien, så kopien overlever en kilde, som nogen har overtaget.

1. På den instans, der skal holde kopien, åbner du **Instanser**, derefter **Hentning**, trykker på **Add source** og vælger **ZFS-datasæt**.
2. Vælg den parrede instans og dens ZFS-elementer. Vælg derefter pool og rod på denne server, tidsplanen og hvor mange snapshots der bliver.
3. Når du gemmer, spørges kilden. Elementet dér viser anmodningen, og der sker intet, før nogen trykker på **Allow**.

Derefter tilføjer kilden en nøgle til den anden instans. Nøglen kan kun sende dette elements snapshots og oprette sine egne replika-snapshots. Den kan ikke slette noget og ikke se andet. **Revoke access** på elementets kort **Replica** fjerner nøglen når som helst. Det, den anden instans allerede har, bliver liggende.

Den hentende instans kører efter sin egen tidsplan og rydder selv op. Tidsplan og opbevaring angives på dens side.

### Hvor dataene havner {#replica-target}

Hvert datasæt havner i `<root>/<server>/<pool>/<path>`. Servermappen er kildeinstansens navn, fastlagt ved den første overførsel, så to servere med samme poolnavn aldrig kommer i vejen for hinanden. For eksempel havner `cache/appdata` fra en server ved navn `tower` i `backup/bombvault-replica/tower/cache/appdata`.

Kopien på målet er skrivebeskyttet og ikke monteret, så den aldrig dækker noget på den server. ZFS-egenskaber følger med, bortset fra monteringspunktet, `sharenfs` og `sharesmb`. Reservationerne `reservation` og `refreservation` bliver også tilbage, så kopien kun optager den plads, dens data kræver.

### Hvad der kommer med {#replica-contents}

Alt, hvad elementet sikkerhedskopierer, kommer med, og volumenerne under det også, som sikkerhedskopien springer over. Et underdatasæt, du har slået fra i elementet, bliver udenfor. Alle datasæt i en kørsel stammer fra ét snapshot, som i sikkerhedskopien.

### Hvor længe snapshots bliver {#replica-retention}

På målet beholder en ny replika 7 daglige og 3 ugentlige snapshots. Vælg i stedet **Short**, **Balanced** eller **Long**, eller angiv **Custom values**. Kun snapshots med navnet `bombvault-replica-<14 digits>` fjernes dér, og aldrig det nyeste, som begge sider deler.

På kilden beholder BombVault kun det seneste replika-snapshot plus et bogmærke for hver tilstand, det har sendt. Bogmærker koster ingen plads. Næste overførsel starter fra dem.

### Krypterede datasæt i en replika {#replica-encryption}

Et krypteret datasæt sendes råt. Det forbliver krypteret på målet, og målet ser aldrig nøglen. Opbevar nøglen sikkert: du skal bruge den for at åbne kopien efter en gendannelse, og en replika uden den kan ikke læses.

### Brug af replikaen {#replica-use}

Åbn elementets fane **Sikkerhedskopier** og klik på replikarækken på kortet **Storage locations**. Arket oplister snapshotsene på målet og viser kommandoerne med dine rigtige navne.

For at se en gammel tilstand kloner du et snapshot på målet. En klon fylder intet, før noget ændres, og replikaen forbliver urørt:

```
zfs clone backup/bombvault-replica/tower/cache/appdata@bombvault-replica-20261006014100 backup/bombvault-replica/clone-appdata
```

Hvis kilden svigter, gør du kopien på målet til et almindeligt datasæt, der kan skrives til:

```
zfs inherit -r readonly backup/bombvault-replica/tower/cache/appdata && zfs inherit -r canmount backup/bombvault-replica/tower/cache/appdata && zfs mount -a
```

BombVault holder derefter op med at replikere til det datasæt, indtil du starter en ny første kørsel.

For at få en tilstand tilbage til kilden trykker du på **Bring back as a new dataset** i arket. BombVault sender snapshottet ind i et nyt datasæt ved siden af originalen, med navnet `<dataset>-bombvault-restore-` plus et tidsstempel. Det skriver aldrig oven på originalen.

## Efterladte snapshots {#leftover-snapshots}

Snapshottet af en sikkerhedskopi hedder `<dataset>@bombvault-<14 cifre>`, for eksempel `cache/appdata@bombvault-20260924021500` (UTC). BombVault fjerner det lige efter sikkerhedskopien. Lykkes det ikke, for eksempel fordi datasættet er optaget eller BombVault blev stoppet, fjerner BombVault det:

- før elementets næste sikkerhedskopi,
- når BombVault starter, for hvert element, også med domænet slået fra,
- når du sletter elementet,
- når du trykker **Fjern nu** på elementet, som også viser, hvor mange der er tilbage.

Kun navne, der præcis er `bombvault-` plus 14 cifre, fjernes. Sikkerhedssnapshots, dine egne snapshots og automatiske snapshots røres aldrig. For at fjerne et i hånden:

```
zfs destroy -r cache/appdata@bombvault-20260924021500
```

En replikas snapshot hedder `<dataset>@bombvault-replica-<14 digits>` og er ikke en rest. Det bliver på kilden, indtil næste replikakørsel erstatter det, og på målet, så længe opbevaringen beholder det. Oprydningen rører det aldrig, fordi den kun matcher `bombvault-` efterfulgt af præcis 14 cifre.

## Afvigelser {#anomalies}

Et underdatasæt, der er blevet tømt, ændrer næsten ikke totalen for et stort træ, så afvigelsesregistreringen holder øje med hvert datasæt i et element for sig: størrelse, antal filer, nye data og restic-tid har hver deres egen historik. Den historik hører til datasættets navn, så den bliver, når træet senere sikkerhedskopieres af et andet element.

Et datasæt, som den forrige kørsel sikkerhedskopierede, og som denne kørsel ikke kunne læse, tæller som tømt, så længe elementets udvalg ikke er ændret. Det dækker en nøgle, der ikke er indlæst, et datasæt, der ikke er monteret, og et, der er forsvundet fra træet. Et underdatasæt, du selv udelukker, ændrer udvalget, så dets historik begynder forfra i stedet. Så længe et fund om mistede data er åbent, beholder opbevaringen de gamle sikkerhedskopier af netop det datasæt og beskærer resten af træet som normalt.

På siden **Afvigelser** har hvert datasæt sin egen linje i elementets panel, som åbnes med **Overvågning** på elementets kort eller, når intet er åbent for elementet, fra dets linje i kortet **Intet åbent**. Elementets træ på denne side viser de åbne fund ved hvert datasæt. Linket i et fund åbner elementets gendannelsespanel ved datasættets sidste gode sikkerhedskopi. Om en kørsel bliver færdig, vurderes for hele elementet, fordi en kørsel lykkes eller fejler som helhed.

Selve tjekkene er beskrevet under [Funktioner](features.md). En assistent, der er forbundet via [MCP-serveren](mcp.md), kan liste et ZFS-elements gendannelsespunkter, starte dets sikkerhedskopi og læse fundene, men et fund kvitteres på siden **Afvigelser**.

## Årsagskoder {#reason-codes}

Siden, kørselshistorikken og notifikationerne nævner et problem med en af disse koder. De fleste har også løsningen ved siden af på siden.

| Kode | Betydning | Hvad du gør |
|---|---|---|
| `ssh-missing` | SSH-forbindelsen er ikke sat op i denne container. | Sæt SSH-forbindelsen op som til VM-sikkerhedskopier. |
| `host-placeholder` | Host SSH: Address er stadig eksempelværdien, og `host.docker.internal` svarede heller ikke. | Sæt Host SSH: Address til denne servers LAN-IP. |
| `host-fallback` | Host SSH: Address er stadig eksempelværdien, og `host.docker.internal` virker. | Intet, eller sæt LAN-IP'en. |
| `ssh-unreachable` | Serveren kan ikke nås over SSH. | Tjek adresse og port, og at SSH er slået til. |
| `ssh-auth` | Serveren afviste BombVaults nøgle. | Kør kommandoen fra forbindelseskortet én gang på serveren. |
| `zfs-not-found` | SSH-værten har ingen `zfs`-kommando. | Peg Host SSH: Address på den maskine, der ejer poolerne. |
| `zfs-permission` | SSH-brugeren må ikke køre denne zfs-kommando. | Brug root, eller se [TrueNAS SCALE](#truenas). |
| `uri-mismatch` | `LIBVIRT_URI` nævner en anden vært eller bruger end SSH-felterne. | Få dem til at stemme overens, eller tøm SSH-felterne, så begge kommer fra URI'en. |
| `zfs-error` | zfs rapporterede en anden fejl. | Detaljerne viser dens besked. |
| `propagation-missing` | Nye monteringer på værten når ikke containeren. | Sæt Access Mode for Host Data til Read/Write - Slave og genstart BombVault. |
| `invalid-name` | Et datasætnavn, BombVault ikke accepterer. | Omdøb datasættet. |
| `name-too-long` | Et datasæt i træet er for langt til et snapshotnavn. | Omdøb det, eller tilføj et datasæt under det som element. |
| `invalid-exclude` | Et udelukkelsesmønster eller et udeladt underdatasæt passer ikke til elementet. | Ret den post, beskeden nævner. For at udelade et helt underdatasæt skal du slå det fra i stedet for at skrive et mønster. |
| `not-found` | Datasættet findes ikke på serveren. | Fjern elementet, eller genopret datasættet. Dets sikkerhedskopier kan stadig gendannes. |
| `not-filesystem` | Dette er et volumen, ikke et filsystem. | Se [Volumener](#volumes). |
| `overlaps-item` | Datasættet overlapper et eksisterende element. | Se [Elementer overlapper aldrig](#overlap). |
| `docker-storage` | Træet indeholder Dockers billedlager. | Se [Dockers lager](#docker-storage). |
| `nothing-readable` | Intet datasæt i elementet kan læses lige nu. | Se på koderne for de oversprungne datasæt. |
| `snapshot-failed` | Snapshottet kunne ikke oprettes. | Detaljerne viser zfs' besked. |
| `containers-busy` | En containersikkerhedskopi kørte stadig, da containerne skulle stoppe. | Start igen senere. Planlagte kørsler venter selv. |
| `consistency-stop-failed` | En container kunne ikke stoppes, så der blev ikke taget et snapshot. | Tjek containeren, eller fjern den fra listen. |
| `pre-snapshot-failed` | Kommandoen før snapshottet fejlede. | Kørselsdetaljerne viser dens output. |
| `container-unknown` | En angivet container findes ikke. | Fjern den fra listen. |
| `container-is-self` | BombVault kan ikke stoppe sin egen container. | Fjern den fra listen. |
| `leftover-snapshots` | Snapshots, som BombVault ikke kunne fjerne, ligger stadig på serveren. | Tryk **Fjern nu**, se [Efterladte snapshots](#leftover-snapshots). |
| `zvol` | Et volumen i træet, sprunget over. | Se [Volumener](#volumes). |
| `canmount-off` | Aldrig monteret (`canmount=off`), sprunget over. | Rummer det data, så montér det eller flyt dataene til et underdatasæt. |
| `legacy-mount` | Legacy-monteringspunkt, sprunget over. | Giv det et monteringspunkt under `/mnt`. |
| `no-mountpoint` | Intet monteringspunkt, sprunget over. | Giv det et monteringspunkt under `/mnt`. |
| `not-mounted` | Ikke monteret på serveren, sprunget over. | Montér det med `zfs mount`, eller sæt `canmount=on`. |
| `key-not-loaded` | Krypteret, og nøglen er ikke indlæst, sprunget over. | `zfs load-key`, montér det derefter. |
| `snapdir-disabled` | Snapshotadgang er slået fra, sprunget over. | `zfs set snapdir=hidden <dataset>`. Mappen `.zfs` forbliver skjult. |
| `not-visible` | BombVault kan ikke se datasættets monteringspunkt. | Flyt monteringspunktet under Host Data-stien, eller map det ind i containeren på samme sti med Read/Write - Slave. |
| `shfs-only` | Datasættet er kun synligt gennem `/mnt/user`, som skjuler snapshots. | Map `/mnt`, ikke `/mnt/user`, som Host Data. |
| `snapshot-not-visible` | Snapshottet blev oprettet, men dukkede ikke op inde i BombVault. | Kør **Test adgangen til snapshots**; se nedenfor. |
| `snapshot-loop` | Snapshottet nåede ikke BombVault, fordi Host Data ikke sender nye monteringer videre. | Sæt Access Mode for Host Data til Read/Write - Slave og genstart BombVault. |
| `backup-failed` | restic fejlede for dette datasæt. | Kørselsdetaljerne viser hvorfor. |
| `not-reached` | Kørslen sluttede før dette datasæt. | Kør sikkerhedskopien igen. |
| `gone` | Datasættet er ikke længere på serveren. | Intet. Dets sikkerhedskopier kan stadig gendannes. |
| `read-only-mount` | BombVault kan kun læse datasættet og kan derfor ikke gendanne ind i det. | Sæt mappingen til Read/Write - Slave, eller gendan til en mappe. |
| `destination-not-mounted` | Mappen ligger ikke på en monteret pool eller share. | Vælg en mappe på en pool eller share. |
| `not-enough-space` | Ikke nok ledig plads på destinationen. | Frigør plads, eller vælg en anden mappe. |
| `safety-snapshot-failed` | Sikkerhedssnapshottet kunne ikke tages, så intet blev gendannet. | Detaljerne viser zfs' besked. |
| `safety-name-too-long` | Datasætnavnet er for langt til et sikkerhedssnapshot. | Slå sikkerhedssnapshottet fra, eller gendan til en mappe. |
| `dataset-exists` | Der findes allerede et datasæt med dette navn. | Vælg et nyt navn, eller gendan ind i selve datasættet. |
| `create-failed` | Det nye datasæt kunne ikke oprettes. | Detaljerne viser beskeden fra zfs. Tjek, at datasættet ovenover findes. |
| `new-dataset-not-visible` | Det nye datasæt blev oprettet, men BombVault kan ikke se det, så intet blev gendannet. | Datasættet bliver på serveren. Montér det under Host Data-stien, og gendan ind i det. |
| `set-properties-failed` | De gemte egenskaber kunne ikke sættes, så intet blev gendannet. | Detaljerne viser zfs' besked. |
| `set-limits-failed` | Filerne blev gendannet, men den gemte kvote eller reservation kunne ikke sættes. | Detaljerne viser beskeden fra zfs. Sæt selv kvoten eller reservationen med `zfs set`. |

### Tjek, hvad containeren ser {#mountinfo}

**Test adgangen til snapshots** på et element tager et rigtigt snapshot af dets træ, leder efter det inde i BombVault for hvert datasæt og fjerner det igen. Det er den hurtigste måde at bevise hele vejen på før den første planlagte kørsel.

For at se selv skal du køre dette på serveren:

```
docker exec BombVault grep zfs /proc/self/mountinfo
```

Hver linje er en montering inde i containeren. Et datasæts linje viser dets sti inde i containeren (under `/host/user`) og datasætnavnet. Et felt `master:N` på den linje betyder, at monteringen modtager de monteringer, værten laver senere, og det er det, snapshotadgang kræver. Mangler det, så sæt Access Mode for Host Data til Read/Write - Slave og genstart BombVault.

## TrueNAS SCALE {#truenas}

- Når `LIBVIRT_URI` er sat (som til VM-sikkerhedskopier på TrueNAS), tager BombVault SSH-vært, bruger og port til sine zfs-kommandoer fra URI'en, hver af dem der ikke er sat for sig. Uden VM-sikkerhedskopier sætter du i stedet `LIBVIRT_HOST`, `LIBVIRT_SSH_USER` og `LIBVIRT_SSH_PORT`. Tilføj variablerne under **Additional Environment Variables**.
- En anden bruger end root skal have rettigheder på elementets øverste datasæt, som så dækker alle datasæt under det:

  ```
  zfs allow <user> snapshot,destroy,mount <dataset>
  ```

  Gendannelse til et nyt datasæt kræver desuden `create` på datasættet ovenover, og at sætte gemte egenskaber kræver rettigheder til de egenskaber.

  En SSH-session uden root på TrueNAS har ikke `/usr/sbin` i sin sti; BombVault kalder så `/usr/sbin/zfs` direkte.
- Appens **Host Data** skal være en værtssti over datasættene, for eksempel `/mnt/tank`, ikke et ixVolume. Med en værtssti sender appen værtens nye monteringer videre til BombVault (`rslave`), og det kræver snapshotadgang.
