# ZFS-datasets

ZFS-datasets zijn items in de lijst **Back-ups**, net als containers en VM's. Een item is één dataset samen met elke dataset eronder. Voor elke back-up maakt BombVault één ZFS-snapshot van de hele boom, zodat elke dataset erin op hetzelfde moment wordt vastgelegd. Daarna leest het de bestanden van elke dataset uit die snapshot en slaat ze met restic op, op dezelfde manier als een map, en verwijdert de snapshot meteen daarna. De back-ups zijn gededupliceerd, je kunt ze allemaal doorbladeren, en losse bestanden zijn te herstellen.

De back-up stuurt nooit een stream naar restic en draait nooit een dataset terug. BombVault vernietigt alleen snapshots die het zelf maakte. De optionele [replica](#replica) is de enige plek die `zfs send` gebruikt: ze kopieert de datasets naar een tweede ZFS-server en raakt de back-up niet aan.

## Vereisten {#requirements}

- **De SSH-verbinding met deze server.** ZFS-datasets gebruiken dezelfde sleutel, host en gebruiker als VM-back-ups. Werken VM-back-ups al, dan werkt dit ook. Volg anders de [gids voor VM-back-up via SSH](vm-backup-ssh-setup.md). De templatevelden heten **Host SSH: Address**, **Host SSH: Port** en **Host SSH: User**.
- **De opdracht `zfs` op die host.** Unraid 6.12 en nieuwer en TrueNAS SCALE hebben hem.
- **Host Data gekoppeld als `/mnt` met Access Mode Read/Write - Slave.** Dat is de standaard van de template. De snapshot van een dataset verschijnt pas in de map `.zfs/snapshot` van de dataset nadat BombVault is gestart, dus de container moet mounts ontvangen die de host later maakt.
- **De datasets gemount onder `/mnt`.** Op Unraid staan pools op `/mnt/<pool>`, dus dat is al zo.

Schakel het onderdeel in onder **Instellingen, Algemeen** (ZFS-datasets). **ZFS-verbinding met deze server** onder **Meer** op de pagina Back-ups opent dan **Verbinding met deze server**. Het test de SSH-verbinding, noemt de gebruiker en host waarmee het verbindt, en zegt wat er ontbreekt als er iets mist. Is de verbinding weg, dan zegt de lijst Back-ups dat met **Verbinding bekijken**, en de kaart **Host-integratie** op het overzicht toont hetzelfde resultaat.

## Items en onderliggende datasets {#items-and-children}

Kies **Toevoegen** op de pagina Back-ups en dan **Een ZFS-dataset**. De lijst komt van de server. Kies de dataset bovenaan wat je wilt back-uppen, bijvoorbeeld `cache/appdata`, en het item omvat haar en elke dataset eronder.

- **Nieuwe onderliggende datasets sluiten zich vanzelf aan.** Een dataset die later onder het item wordt aangemaakt, wordt bij de volgende run geback-upt, en die run noemt haar als nieuw. Haar eerste back-up leest haar één keer helemaal. Daarna worden alleen wijzigingen gelezen.
- **Je kunt losse onderliggende datasets weglaten.** Schakel een onderliggende uit op het tabblad Instellingen van het item en ze wordt samen met alles eronder weggelaten. Een weggelaten onderliggende die niet meer op de server bestaat, wordt als zodanig gemarkeerd en kan van de lijst worden verwijderd.
- **Onderliggende datasets die niet gelezen kunnen worden, worden overgeslagen, nooit stilletjes.** De run toont ze, het item toont hoeveel er zijn overgeslagen, en de kaart **Zonder back-up** op het overzicht telt elke als onbeschermd. De run maakt nog steeds een back-up van al het andere en mislukt niet door een overgeslagen onderliggende. De redenen staan in de [tabel met redencodes](#reason-codes): een dataset die niet is gemount, `canmount=off` heeft, een `legacy` of geen mountpoint, een encryptiesleutel die niet is geladen, snapshottoegang die uitstaat, of een mountpoint dat BombVault niet kan zien.
- **Een overgeslagen dataset neemt zijn onderliggende niet mee.** Een `canmount=off`-dataset die alleen andere datasets bevat, wordt overgeslagen (getoond als "alleen structuur") en haar gemounte onderliggende worden wel geback-upt. Een versleutelde dataset waarvan de sleutel niet is geladen, wordt samen met de onderliggende datasets overgeslagen die zijn sleutel delen.
- **Onderliggende datasets die VM-schijven of systeemdata zijn, beginnen uitgeschakeld** in het toevoegvenster, met de reden naast de schakelaar. Een hele pool toevoegen vraagt om een bevestiging die toont wat erin zit.

### Volumes {#volumes}

Een volume (zvol) bevat een virtuele schijf in plaats van bestanden, en een ZFS-item maakt er nooit een back-up van.

- Een volume dat een VM gebruikt, wordt met die VM geback-upt.
- Een volume dat geen VM gebruikt (een iSCSI-extent, een schijf die je loskoppelde) wordt **niet door BombVault geback-upt**. Het toevoegvenster en de pagina Back-ups tellen deze volumes en zeggen dat. Een latere versie gaat ze back-uppen.

Volumes binnen de boom van een item worden overgeslagen en bij elke run genoemd.

### De opslag van Docker {#docker-storage}

Met de ZFS-opslagdriver van Docker is elke imagelaag een dataset met een `legacy` mountpoint. Het toevoegvenster vouwt deze samen tot één regel per bovenliggende. Een boom met meer dan 20 van zulke datasets kan geen item worden: zolang er een snapshot van bestaat, kan Docker geen imagelagen verwijderen. Voeg in plaats daarvan de datasets eronder toe, bijvoorbeeld `appdata`.

### Items overlappen nooit {#overlap}

Een dataset kan maar bij één item horen. BombVault weigert een nieuw item dat binnen een bestaand item ligt, of dat er een zou bevatten. Om meerdere onderliggende items samen te voegen tot één bovenliggend item, verwijder je eerst de onderliggende items en kies je ervoor hun back-ups te bewaren, en voeg je dan het bovenliggende toe. Elke dataset houdt zijn geschiedenis onder zijn eigen naam, dus de volgende back-up gaat verder waar de oude items ophielden en leest niet alles opnieuw.

## Containers stoppen en opdrachten rond de snapshot {#consistency}

Een snapshot van een draaiende database is als een plotselinge stroomuitval: de database herstelt meestal, maar hij moet het wel doen. Elk item kan daar twee dingen aan doen, en beide gelden alleen voor het moment van de snapshot, niet voor de hele back-up.

- **Deze containers stoppen voor de snapshot.** BombVault stopt de genoemde containers, maakt de snapshot en start ze meteen weer. Containers van één niveau van afhankelijkheden stoppen parallel, afhankelijken eerst, dus het hele venster is meestal een paar seconden. De run toont hoe lang het was. De back-up leest dan de bevroren snapshot terwijl de apps al weer draaien. Alleen containers die draaiden, worden gestopt.
- **Een opdracht vóór en na de snapshot.** Ze draait in een container die je kiest, bijvoorbeeld om vlak vóór de snapshot een database naar de dataset te dumpen, zonder iets te stoppen. Mislukt de opdracht vóór de snapshot, dan mislukt de back-up en wordt er geen snapshot gemaakt. Een opdracht na de snapshot die mislukt, wordt op de run getoond maar laat de back-up niet mislukken.

Wat er gebeurt als er iets misgaat:

- Kan een container niet worden gestopt, dan start BombVault de containers die het al stopte en mislukt de back-up, met de naam van de container. Het valt nooit terug op een snapshot van draaiende apps.
- Het stoppen wacht tot een lopende containerback-up klaar is (tot 30 minuten voor een handmatige run, tot de tijdslimiet van de back-up voor een geplande), zodat de twee nooit dezelfde container tegelijk stoppen en starten.
- Voordat de eerste container stopt, schrijft BombVault op welke het stopt. Wordt BombVault binnen het venster gedood, dan start het die containers de volgende keer dat het start weer, stuurt het een melding, en toont het item een rode opmerking voor elke container die het niet kon starten.

De automatische [databasedumps](backups.md#database-dumps) draaien met de eigen back-up van een container, niet met een ZFS-item. Een database waarvan de container alleen via zijn dataset wordt geback-upt, krijgt geen dump, dus geef hem hier een opdracht.

Een container kan op deze lijst staan en tegelijk als container worden geback-upt. Zijn data wordt dan twee keer opgeslagen, in twee repository's, en de volledige back-up stopt hem twee keer. Het item zegt dat.

## Herstellen {#restore}

Open het item en kies **Herstellen**, of kies het op de pagina **Herstel**. Het herstelvenster vraagt om de back-up en dan om de **Dataset**. Standaard is dat de bovenste dataset van het item. Onder **Wat** herstel je alles of alleen de bestanden en mappen die je kiest. **Waarheen** biedt:

- **Terugzetten in de dataset.** Bestanden uit de back-up worden in het mountpoint van de dataset geschreven. Bestanden met dezelfde naam worden overschreven, andere bestanden blijven. De dataset wordt nooit teruggedraaid of vervangen. BombVault controleert één keer voordat het begint, en nog eens vlak voordat het schrijft, dat de dataset is gemount, zichtbaar en beschrijfbaar. Waar een onderliggende dataset erin is gemount, wordt niets geschreven: de onderliggende houdt zijn bestanden, eigenaar en rechten, en wordt vanuit zijn eigen back-up hersteld.
- **Naar een map.** Kies een map onder `/mnt`. BombVault controleert dat de map op een gemounte pool of share staat en dat er genoeg vrije ruimte is. Dit werkt zonder de SSH-verbinding en voor datasets die niet meer bestaan.
- **In een nieuwe dataset.** Noem een dataset die nog niet bestaat. Het venster stelt `<dataset>-restored` voor. BombVault maakt hem aan met de ZFS-eigenschappen die in de back-up zijn opgeslagen en herstelt erin, zie [Terugzetten als nieuwe dataset](#new-dataset).

Kies je **Alle datasets van deze back-up** als dataset, dan wordt elke dataset van de boom in een eigen submap van de map die je kiest geschreven. Datasets die in die back-up waren overgeslagen, worden genoemd. Om uit het repository van een andere BombVault te herstellen, gebruik je **Uit een oude back-upmap** op de pagina Herstel. Dat herstelt altijd naar een map: alle datasets van één back-up, elk in een eigen submap, of één dataset van de boom, geheel of met gekozen bestanden.

De lijst met te stoppen containers van het item wordt ook aangeboden voor een herstel in de dataset. Die containers blijven gestopt voor het hele herstel, en containerback-ups wachten intussen.

### De veiligheidssnapshot {#safety-snapshot}

Voordat het in een dataset schrijft, maakt BombVault een ZFS-snapshot van alleen die dataset, genaamd `bombvault-prerestore-<time>`. **Eerst een ZFS-snapshot van de huidige toestand bewaren** staat standaard aan. Uitschakelen vraagt om een tweede bevestiging. Kan de snapshot niet worden gemaakt, dan wordt er niets hersteld.

BombVault verwijdert een veiligheidssnapshot nooit zelf. Het item toont ze met hun leeftijd en grootte, elk met een actie **Verwijderen**, en waarschuwt als de oudste meer dan 30 dagen oud is, omdat ze verwijderde en gewijzigde data op de pool houdt.

Om na een herstel terug te gaan, kopieer je losse bestanden uit `.zfs/snapshot/bombvault-prerestore-<time>` binnen de dataset. `zfs rollback <dataset>@bombvault-prerestore-<time>` werkt alleen zolang het de nieuwste snapshot van die dataset is. `zfs rollback -r` verwijdert elke nieuwere snapshot, automatische inbegrepen.

### Terugzetten als nieuwe dataset {#new-dataset}

BombVault slaat bij elke back-up de lokaal ingestelde ZFS-eigenschappen van elke dataset op: compressie, recordgrootte, quota, reservering, atime, xattr, acltype, hoofdlettergevoeligheid en je eigen gebruikerseigenschappen. Overgeërfde en alleen-lezen waarden blijven weg, omdat ze vanzelf terugkomen. Back-ups van vóór BombVault ze opsloeg, hebben ze niet.

- **In een nieuwe dataset** voert `zfs create` uit met elke opgeslagen eigenschap. Hoofdlettergevoeligheid, normalisatie en utf8only kunnen alleen zo worden ingesteld. Quota en reserveringen worden na de bestanden ingesteld, zodat ze ze niet kunnen weigeren. Het mountpoint blijft weg, zodat de kopie niet botst met het origineel, en `canmount`, `readonly` en de versleutelingsinstellingen ook, zodat het herstel kan schrijven. Een nieuwe dataset onder een versleutelde neemt haar versleuteling over. De dataset boven de nieuwe moet bestaan. Gaat er iets mis nadat de nieuwe dataset is gemaakt, dan blijft ze op de server staan, omdat BombVault nooit een dataset vernietigt.
- **Terugzetten in de dataset** toont de opgeslagen eigenschappen naast het herstel. **Deze eigenschappen ook instellen** stelt de eigenschappen in die een bestaande dataset nog kan nemen, voordat er een bestand wordt geschreven. Quota en reserveringen worden na de bestanden ingesteld, zodat ze ze niet kunnen weigeren. Zonder die schakelaar houdt de dataset zijn instellingen.

## Wat er in de back-up zit {#contents}

In de back-up: de bestanden en mappen van elke geback-upte dataset, met hun eigenaar, modi, tijdstempels en uitgebreide attributen zoals restic ze opslaat, en de lokaal ingestelde ZFS-eigenschappen van elke dataset.

Niet in de back-up:

- de eigenaar en modus van de bovenste map van elke dataset zelf (alles eronder is wel inbegrepen). Een herstel in de dataset laat de bestaande bovenste map zoals hij is, een herstel naar een map maakt hem aan met modus `0755`,
- bestaande ZFS-snapshots,
- onderliggende datasets die zijn overgeslagen of weggelaten,
- volumes.

Om op een nieuwe pool te herstellen, maak je de pool aan en herstel je elke dataset in een nieuwe dataset. Of NFSv4-ACL's, zoals TrueNAS ze op SMB-datasets gebruikt, terugkomen zoals je verwacht, is nog niet geverifieerd, dus controleer een herstel op je eigen data voordat je erop vertrouwt.

## Versleutelde datasets {#encryption}

Een versleutelde dataset wordt alleen geback-upt zolang haar sleutel is geladen. Anders wordt ze met een waarschuwing overgeslagen. Laad de sleutel met `zfs load-key` en mount de dataset. BombVault leest de data ontsleuteld en slaat ze op in het repository van restic, dat versleuteld is. Heb je versleuteling in BombVault uitgeschakeld, dan is dat repository het niet.

## Replica {#replica}

Een replica is een kopie van de datasets van een item op een tweede ZFS-server. BombVault houdt haar actueel met `zfs send` en `zfs receive`. De eerste run stuurt alles, daarna reizen alleen de gewijzigde blokken. Op de andere server kun je de kopie meteen mounten.

Een replica vervangt de back-up nooit. Oudere versies, losse bestanden en de controle komen nog uit de back-ups, en de replica houdt alleen zoveel snapshots als je instelt. Een actuele replica telt als kopie buiten de locatie, maar niet als back-up.

De replica wacht niet op de back-up. **Na elke back-up** betekent na de back-uprun van het item, of die run nu slaagde, mislukte of niets te doen vond. Heeft het ZFS-onderdeel geen bestemming ingeschakeld en het item geen eigen repository, dan stuurt een back-uprun van het item alleen de replica, en de kaart Replica zegt dat in de bubbel naast **Wanneer**. De lijst Back-ups, de kop van het item en de runhistorie tonen de replica: **Bezig met repliceren** terwijl ze loopt, en **Gerepliceerd** met de tijd zodra ze klaar is.

**Alleen replica, geen back-up** op de kaart Replica laat het hele ZFS-onderdeel replica's sturen en geen back-ups maken. De eigen map en de off-site-bestemmingen houden hun schakelaars, dus uitschakelen maakt weer back-ups naar hen, en de kaart van elk ZFS-item toont de schakelaar van het onderdeel zoals hij staat. Zo'n item wacht op zijn eerste replica in plaats van op een back-up, **Nu back-up maken** wordt aangeboden als **Nu repliceren**, en de containers van het item stoppen en zijn opdrachten draaien rond de snapshot zoals bij een back-up. Zijn replica's tellen voor een planning van elke paar dagen, de melding van achterstallige back-ups en de dekking.

Schakel haar in in de kaart **Replica** op het tabblad Instellingen van het item. De kaart toont elke dataset en elk volume met zijn status, en **Nu repliceren** start een run. Een replicarun heeft een eigen lock, zodat een lange eerste overdracht de back-ups nooit ophoudt.

| Veld | Wat het instelt |
|---|---|
| **Repliceert naar** | Het doel: een ZFS-server (gemarkeerd **via SSH**) of een **gekoppelde instantie**. **ZFS-server toevoegen** stelt een nieuwe in. |
| **Wanneer** | **Na elke back-up**, wat betekent na de back-uprun van het item, of **Eigen plan**. |
| **Bewaren op het doel** | Hoeveel snapshots op de ZFS-server blijven: **Kort**, **Gebalanceerd**, **Lang** of **Eigen waarden**. Een gekoppelde instantie beslist dit zelf. |
| **Bewaren op deze server** | Hoeveel van de nieuwste replicasnapshots ook op deze server blijven. 0 of 1 bewaart alleen de laatste. |
| **Alleen replica, geen back-up** | Het hele ZFS-onderdeel stuurt replica's en maakt geen back-ups. |

![De kaart Replica van een ZFS-item.](assets/screenshots/zfs/replica-dark.png#only-dark)
![De kaart Replica van een ZFS-item.](assets/screenshots/zfs/replica-light.png#only-light)

**Nu alles repliceren**, naast de back-upacties wanneer de tegel ZFS op de pagina Back-ups is gekozen, stuurt de replica van elk ZFS-item, het ene na het andere. De server voert de batch uit, dus ze gaat door als je de pagina verlaat, en een item dat niet kan repliceren wordt genoemd en overgeslagen.

### Naar een ZFS-server pushen {#replica-push}

Elke machine met ZFS en SSH kan ontvangen, bijvoorbeeld een tweede Unraid of een TrueNAS. BombVault hoeft daar niet te draaien.

1. Kies onder **Instellingen, Koppeling** **Verbinden via SSH** in **ZFS-server zonder BombVault**, of **ZFS-server toevoegen** in de kaart **Replica** van het item.
2. Vul adres, gebruiker en poort in. Het venster toont de publieke sleutel van BombVault. Voeg hem toe aan `~/.ssh/authorized_keys` van de gebruiker op de server. Op Unraid is dat onder **Users, root, SSH keys**. Afhankelijk van **Display Settings** staat Users in het kopmenu of onder Settings. Bewerk op TrueNAS de gebruiker onder **Credentials, Users**. Dit is de replicasleutel. Hij verschilt van de sleutel van de VM- en host-SSH-verbinding, dus voeg hem toe, ook als die al is geautoriseerd.
3. Test de verbinding. Het venster toont dan de pools van de server. Kies er een en stel de root in, die standaard `<pool>/bombvault-replica` is.
4. Kies de nieuwe server in de kaart **Replica** van het item. De server heeft dan een eigen kaart op de pagina Instanties, en **Openen** leidt naar zijn pagina.

Met root is niets anders nodig. Een eigen gebruiker heeft deze rechten nodig op de pool van het doel, die het venster ook toont:

```
zfs allow <user> receive,create,mount,rollback,destroy,userprop <pool>
```

Op de bron heeft dezelfde soort gebruiker deze nodig op de bovenste dataset van het item:

```
zfs allow <user> send,snapshot,hold,release,bookmark,destroy <dataset>
```

In deze richting houdt de BombVault die het item heeft ook een sleutel die op de server kan schrijven.

BombVault zet de hostsleutel die de server bij de eerste verbinding toont vast en weigert later een andere. Is de server opnieuw geïnstalleerd, druk dan op de pagina van de server op **Hostsleutel vergeten**, en de volgende verbinding zet de nieuwe sleutel vast.

Kan de host van deze server de ZFS-server zelf bereiken, dan pipet hij `zfs send` rechtstreeks in zijn eigen ssh naar de server, met de replicasleutel en de vastgezette hostsleutel, zodat de stream niet door BombVault loopt. Anders loopt de stream door BombVault.

### Naar een gekoppelde instantie sturen {#replica-receive}

Een gekoppelde BombVault kan de replica zelf ontvangen. Niemand krijgt SSH-toegang tot de andere host, en er komt geen sleutel in een `authorized_keys`-bestand.

1. Kies in de kaart **Replica** van het item de gekoppelde instantie als doel, of schakel de rol **ZFS-server** in op de kaart van die instantie op de pagina Instanties en kies daar de ZFS-items. De kaart toont **Wacht op goedkeuring** tot ze antwoordt.
2. Open op de ontvangende instantie **Instanties**. Het verzoek staat op de kaart van de verzendende instantie als badge, en zijn **Details** tonen het. Kies de pool en root op die server en hoeveel snapshots blijven, en druk dan op **Toestaan** of **Afwijzen**.
3. Na **Toestaan** verstuurt de bron volgens zijn eigen planning, zoals voor elk ander doel.

De ontvangende instantie accepteert alleen wat de goedkeuring dekt: de datasets van het item, in haar eigen root. Ze voert `zfs receive` zelf uit, en de bron kan daar niets verwijderen of terugdraaien. Zo overleeft de kopie een bron die iemand heeft overgenomen. De ontvangende instantie houdt haar eigen bewaarbeleid. De bron stelt alleen een regel voor als hij erom vraagt.

**Toegang intrekken** op de ontvangende instantie beëindigt de goedkeuring op elk moment en meldt het aan de bron, die dan toont dat de goedkeuring is ingetrokken en stopt. Wat de ontvangende instantie al heeft, blijft daar. Een afgewezen verzoek blijft afgewezen. Meer datasets, of een verzoek na een intrekking, vragen opnieuw.

De ontvangende instantie moet via HTTPS antwoorden. De bron stuurt niets naar een gewoon http-adres, omdat het token en de data dan onversleuteld over het netwerk gaan. Elke overdracht laat ook een tiende van de ontvangende pool vrij en stopt voordat ze daarin zou snijden, zodat één bron de pool van de ontvangende server niet kan vullen.

### Waar de data terechtkomt {#replica-target}

Elke dataset komt terecht op `<root>/<server>/<pool>/<path>`. De servermap is de naam van de broninstantie, vastgelegd bij de eerste overdracht, zodat twee servers met dezelfde poolnaam elkaar nooit in de weg zitten. Bijvoorbeeld: `cache/appdata` van een server genaamd `tower` belandt op `backup/bombvault-replica/tower/cache/appdata`.

Nadat BombVault opnieuw is ingesteld, weigert het doel de map die deze BombVault eerder vulde. De kaart Replica biedt dan **Map overnemen** aan, en de volgende replica gaat verder vanaf de kopieën erin. Neem hem alleen over als geen andere BombVault nog in die map repliceert.

De kopie op het doel is alleen-lezen. Standaard is ze niet gemount, zodat ze op die server nooit iets bedekt. Een ZFS-server die via SSH is verbonden, heeft op zijn pagina een schakelaar **Alleen-lezen aankoppelen op de server**: staat die aan, dan mount BombVault de kopieën daar na elke run alleen-lezen, zodat je hun bestanden op het doel kunt lezen en kopiëren. Versleutelde kopieën blijven vergrendeld tot hun sleutel is geladen, en uitschakelen unmount niets. Een gekoppelde instantie die de replica ontvangt, houdt haar niet gemount. ZFS-eigenschappen reizen mee, behalve het mountpoint en `sharenfs` en `sharesmb`. De reserveringen `reservation` en `refreservation` blijven ook achter, zodat de kopie alleen de ruimte inneemt die haar data nodig heeft.

### Wat erin gaat {#replica-contents}

Alles wat het item back-upt gaat erin, en de volumes eronder ook, die de back-up overslaat. Een onderliggende dataset die je in het item hebt uitgeschakeld, blijft erbuiten. Alle datasets van een run komen uit één snapshot, zoals in de back-up.

### Hoe lang snapshots blijven {#replica-retention}

Op het doel bewaart een nieuwe replica 7 dagelijkse en 3 wekelijkse snapshots. Kies in plaats daarvan **Kort**, **Gebalanceerd** of **Lang**, of stel **Eigen waarden** in. Alleen snapshots met de naam `bombvault-replica-<14 digits>` worden daar ooit verwijderd, en nooit de nieuwste die beide kanten delen.

Op de bron bewaart BombVault de laatste replicasnapshot, plus een bladwijzer voor elke toestand die het heeft verstuurd. Bladwijzers kosten geen ruimte. De volgende overdracht begint ervan.

Om ook op de bron meer te bewaren, stel je **Bewaren op deze server** in op de kaart Replica in, bijvoorbeeld op 7. Zoveel van de nieuwste replicasnapshots blijven dan na elke replica ook op deze server staan, naast wat het doel bewaart. 0 of 1 bewaart alleen de laatste. De laatste draagt een hold, zodat de basis van de volgende overdracht niet per ongeluk wordt vernietigd. De oudere hebben er geen. Elke snapshot houdt de blokken vast die erna veranderden, dus elke die je bewaart, neemt ruimte op de pool in. De replica uitschakelen of naar een ander doel zetten, of het item verwijderen, verwijdert ze ook van deze server.

De bestanden van een bewaarde snapshot kunnen meteen worden gelezen of gekopieerd, uit de map `.zfs/snapshot` in de dataset of uit een clone, zonder de dataset of de replica aan te raken:

```
ls /mnt/cache/appdata/.zfs/snapshot/bombvault-replica-20261006014100
zfs clone cache/appdata@bombvault-replica-20261006014100 cache/appdata-20261006
```

Een clone neemt geen ruimte in tot er iets in verandert. Vernietig haar met `zfs destroy cache/appdata-20261006` als je klaar bent, want een snapshot met een clone kan niet worden verwijderd.

Teruggaan naar de laatste replicasnapshot heeft hier alleen `zfs rollback cache/appdata@<latest>` nodig, en niets op het doel. De hele dataset terugdraaien naar een oudere vernietigt elke snapshot daarna, op deze server en op het doel. De laatste is vastgehouden, dus hef eerst zijn hold op en draai de dataset dan hier terug:

```
zfs release -r bombvault-replica cache/appdata@bombvault-replica-20261008014200
zfs rollback -r cache/appdata@bombvault-replica-20261006014100
```

De kopie op het doel moet ook terug, maar niet altijd naar dezelfde snapshot, omdat haar eigen bewaarregels die misschien hebben laten vallen. Voer de replica één keer uit. Ze stopt voor die dataset omdat het doel nieuwere snapshots heeft, en de kaart Replica en de runhistorie noemen de snapshot waarnaar het doel moet worden teruggedraaid: de nieuwste die beide kanten nog delen. Draai de kopie ernaar terug op de ZFS-server, of op de host van de gekoppelde instantie die de replica ontvangt, bijvoorbeeld:

```
zfs rollback -r backup/bombvault-replica/tower/cache/appdata@bombvault-replica-20261005014000
```

`zfs rollback` werkt op één dataset, dus herhaal de rollbacks voor elke onderliggende dataset die je ook terug wilt. Daarna gaat **Nu repliceren** verder vanaf de gedeelde snapshot.

### Versleutelde datasets in een replica {#replica-encryption}

Een versleutelde dataset wordt ruw verstuurd. Ze blijft op het doel versleuteld, en het doel ziet de sleutel nooit. Bewaar de sleutel veilig: je hebt hem nodig om de kopie na een herstel te openen, en een replica zonder hem is onleesbaar.

### De replica gebruiken {#replica-use}

Open het tabblad **Back-ups** van het item en klik op de rij **ZFS-replica** tussen de plekken die zijn snapshots bevatten. Het blad toont de snapshots op het doel en laat de opdrachten met je echte namen zien.

Om een oude toestand te bekijken, kloon je een snapshot op het doel. Een clone neemt geen ruimte in tot er iets verandert, en de replica blijft onaangeroerd:

```
zfs clone backup/bombvault-replica/tower/cache/appdata@bombvault-replica-20261006014100 backup/bombvault-replica/clone-appdata
```

Valt de bron uit, maak de kopie dan op het doel een gewone, beschrijfbare dataset:

```
zfs inherit -r readonly backup/bombvault-replica/tower/cache/appdata && zfs inherit -r canmount backup/bombvault-replica/tower/cache/appdata && zfs mount -a
```

BombVault stopt daarna met repliceren naar die dataset, tot je een nieuwe eerste run start.

Om een toestand terug te krijgen op de bron, druk je in het blad op **Terugzetten als nieuwe dataset**. BombVault stuurt de snapshot naar een nieuwe dataset naast het origineel, genaamd `<dataset>-bombvault-restore-` plus een tijdstempel. Het schrijft nooit over het origineel.

### Voorbeeld: een replica naar een tweede Unraid-machine {#example-replica}

De server `tower` maakt een back-up van de dataset `cache/appdata` als ZFS-item. Een tweede Unraid-machine op `192.168.1.20`, zonder BombVault, heeft een pool genaamd `backup` en moet een kopie bewaren die je op elk moment kunt lezen.

1. Open **Back-ups**, dan `cache/appdata`, dan zijn tabblad **Instellingen**. Kies in de kaart **Replica** onder **Repliceert naar** **ZFS-server toevoegen**.
2. Vul `192.168.1.20` in, de gebruiker `root` en poort `22`. Het venster toont de replicasleutel. Plak hem op de tweede machine onder **Users, root, SSH keys** en druk op **Apply**. Deze sleutel verschilt van de Host-SSH-sleutel, dus voeg hem toe, ook als die al is toegevoegd.
3. Test de verbinding. Kies de pool `backup` en houd de root aan die voorgesteld wordt, `backup/bombvault-replica`.
4. Terug op de kaart is de nieuwe server gekozen onder **Repliceert naar**. Laat **Wanneer** op **Na elke back-up**, laat **Bewaren op het doel** op zijn 7 dagelijkse en 3 wekelijkse snapshots en zet **Bewaren op deze server** op `7`.
5. Druk op **Nu repliceren**. De eerste run stuurt alles, dus 300 GB over een lijn van 1 Gbit/s duurt minstens 40 minuten. De kaart en de lijst Back-ups zeggen **Bezig met repliceren**, dan **Gerepliceerd** met de tijd. Latere runs sturen alleen de blokken die veranderden.

De datasets staan nu op `backup/bombvault-replica/tower/cache/appdata` op de tweede machine, alleen-lezen. Op het overzicht telt **Off-site** van het ZFS-onderdeel de replica als kopie buiten de locatie.

Om één bestand van afgelopen dinsdag te krijgen, heb je drie wegen:

- **Op deze server**, omdat **Bewaren op deze server** zeven snapshots bewaart:

    ```
    cp /mnt/cache/appdata/.zfs/snapshot/bombvault-replica-20261103014100/plex/Preferences.xml /mnt/user/restore/
    ```

- **Op de tweede machine**, met een clone van een snapshot op het doel. Het tabblad **Back-ups** van het item toont de opdrachten met je echte namen als je op de rij **ZFS-replica** klikt. Verwijder de clone met `zfs destroy` als je klaar bent.
- **Met een mount**, door **Alleen-lezen aankoppelen op de server** in te schakelen op de pagina van de ZFS-server. Na de volgende run staan de kopieën alleen-lezen gemount op de tweede machine.

Versies die ouder zijn dan de replica bewaart, komen nog uit de back-ups, zoals het volgende voorbeeld laat zien.

### Voorbeeld: één bestand uit een ZFS-back-up {#example-zfs-file}

1. Open **Herstel**, kies de tegel **ZFS** en klik op `cache/appdata`.
2. Kies de back-up, en onder **Dataset** de dataset die het bestand bevat.
3. Kies onder **Wat** **Losse bestanden** en vink het bestand aan.
4. Kies onder **Waarheen** **Naar een map** en kies een map onder `/mnt`, zoals `user/restore`. Dit heeft geen SSH-verbinding nodig en werkt ook voor een dataset die niet meer bestaat.
5. Druk op **Herstellen**. Het bestand komt rechtstreeks in die map, zonder het pad erboven.

## Achtergebleven snapshots {#leftover-snapshots}

De snapshot van een back-up heet `<dataset>@bombvault-<14 digits>`, bijvoorbeeld `cache/appdata@bombvault-20260924021500` (UTC). BombVault verwijdert hem meteen na de back-up. Mislukt dat, bijvoorbeeld omdat de dataset bezig is of BombVault werd gestopt, dan verwijdert BombVault hem:

- vóór de volgende back-up van dat item,
- wanneer BombVault start, voor elk item, ook met het onderdeel uitgeschakeld,
- wanneer je het item verwijdert,
- wanneer je op het item op **Nu verwijderen** drukt, wat toont hoeveel er over zijn.

Alleen namen die exact `bombvault-` plus 14 cijfers zijn, worden verwijderd. Veiligheidssnapshots, je eigen snapshots en automatische snapshots worden nooit aangeraakt. Om er een met de hand te verwijderen:

```
zfs destroy -r cache/appdata@bombvault-20260924021500
```

De snapshot van een replica heet `<dataset>@bombvault-replica-<14 digits>` en is geen overblijfsel. Hij blijft op de bron staan tot een nieuwere replicarun hem vervangt, of zolang **Bewaren op deze server** hem bewaart, en op het doel zolang het bewaarbeleid hem bewaart. De opruimer raakt hem nooit aan, omdat hij alleen `bombvault-` gevolgd door precies 14 cijfers herkent.

## Anomalieën {#anomalies}

Een onderliggende dataset die is leeggemaakt, verandert het totaal van een grote boom nauwelijks, dus anomaliedetectie bewaakt elke dataset van een item afzonderlijk: zijn grootte, aantal bestanden, nieuwe data en restic-tijd hebben elk een eigen geschiedenis. Die geschiedenis hoort bij de naam van de dataset, dus ze blijft bestaan als de boom later door een ander item wordt geback-upt.

Een dataset die de vorige run back-upte en deze run niet kon lezen, telt als leeggemaakt, zolang de selectie van het item niet veranderde. Dat dekt een sleutel die niet is geladen, een dataset die niet is gemount en een die uit de boom is verdwenen. Een onderliggende die je zelf uitsluit, verandert de selectie, dus zijn geschiedenis begint dan opnieuw. Zolang een bevinding over verloren data openstaat, bewaart het bewaarbeleid de oude back-ups van die ene dataset en schoont de rest van de boom zoals gewoonlijk op.

Elke dataset heeft een eigen regel in de bewaking van het item, die opent met **Bewaking** op de pagina **Anomalieën** en op het tabblad **Verificatie** van het item staat. De openstaande bevindingen staan naast elke dataset. De link in een bevinding opent het herstelvenster op de laatste goede back-up van de dataset. Of een run slaagt, wordt voor het hele item beoordeeld, want een run slaagt of mislukt als geheel.

De controles zelf staan beschreven onder [Anomalieën](anomalies.md). Een assistent die via de [MCP-server](mcp.md) is verbonden, kan de herstelpunten van een ZFS-item tonen, zijn back-up starten en de bevindingen lezen, maar het bevestigen van een bevinding gebeurt op de pagina **Anomalieën**.

## Redencodes {#reason-codes}

De pagina van het item, de runhistorie en de meldingen noemen een probleem met een van deze codes. De meeste hebben de oplossing ook naast zich op de pagina.

| Code | Betekenis | Wat te doen |
|---|---|---|
| `ssh-missing` | De SSH-verbinding is niet ingesteld in deze container. | Stel de SSH-verbinding in zoals voor VM-back-ups. |
| `host-placeholder` | Host SSH: Address is nog de plaatshouder, en `host.docker.internal` antwoordde ook niet. | Zet Host SSH: Address op het LAN-IP van deze server. |
| `host-fallback` | Host SSH: Address is nog de plaatshouder, en `host.docker.internal` werkt. | Niets, of stel het LAN-IP in. |
| `ssh-unreachable` | De server is niet bereikbaar via SSH. | Controleer adres en poort, en of SSH aanstaat. |
| `ssh-auth` | De server weigerde de sleutel van BombVault. | Voer de opdracht uit **Verbinding met deze server** één keer uit op de server. |
| `zfs-not-found` | De SSH-host heeft geen opdracht `zfs`. | Wijs Host SSH: Address naar de machine die de pools beheert. |
| `zfs-permission` | De SSH-gebruiker mag deze zfs-opdracht niet uitvoeren. | Gebruik root, of zie [TrueNAS SCALE](#truenas). |
| `uri-mismatch` | `LIBVIRT_URI` noemt een andere host of gebruiker dan de SSH-velden. | Laat ze overeenkomen, of wis de SSH-velden zodat beide uit de URI komen. |
| `zfs-error` | zfs meldde een andere fout. | De details tonen zijn bericht. |
| `propagation-missing` | Nieuwe mounts op de host bereiken de container niet. | Zet de Access Mode van Host Data op Read/Write - Slave en herstart BombVault. |
| `invalid-name` | Een datasetnaam die BombVault niet accepteert. | Hernoem de dataset. |
| `name-too-long` | Een dataset in de boom is te lang voor een snapshotnaam. | Hernoem haar, of voeg een dataset eronder toe als het item. |
| `invalid-exclude` | Een uitsluitpatroon of een weggelaten onderliggende past niet bij het item. | Herstel het item dat de melding noemt. Om een hele onderliggende dataset weg te laten, schakel je haar uit in plaats van een patroon te schrijven. |
| `not-found` | De dataset bestaat niet op de server. | Verwijder het item, of maak de dataset opnieuw aan. Zijn back-ups blijven herstelbaar. |
| `not-filesystem` | Dit is een volume, geen bestandssysteem. | Zie [Volumes](#volumes). |
| `overlaps-item` | De dataset overlapt een bestaand item. | Zie [Items overlappen nooit](#overlap). |
| `docker-storage` | De boom bevat de imageopslag van Docker. | Zie [De opslag van Docker](#docker-storage). |
| `nothing-readable` | Geen enkele dataset in het item kan nu worden gelezen. | Kijk naar de codes van de overgeslagen datasets. |
| `snapshot-failed` | De snapshot kon niet worden gemaakt. | De details tonen het bericht van zfs. |
| `containers-busy` | Een containerback-up liep nog toen de containers moesten stoppen. | Begin later opnieuw. Geplande runs wachten uit zichzelf. |
| `consistency-stop-failed` | Een container kon niet worden gestopt, dus er is geen snapshot gemaakt. | Controleer de container, of haal hem van de lijst. |
| `pre-snapshot-failed` | De opdracht vóór de snapshot mislukte. | De rundetails tonen de uitvoer. |
| `container-unknown` | Een genoemde container bestaat niet. | Verwijder hem van de lijst. |
| `container-is-self` | BombVault kan zijn eigen container niet stoppen. | Verwijder hem van de lijst. |
| `leftover-snapshots` | Snapshots die BombVault niet kon verwijderen, staan nog op de server. | Druk op **Nu verwijderen**, zie [Achtergebleven snapshots](#leftover-snapshots). |
| `zvol` | Een volume in de boom, overgeslagen. | Zie [Volumes](#volumes). |
| `canmount-off` | Nooit gemount (`canmount=off`), overgeslagen. | Bevat hij data, mount hem dan of verplaats de data naar een onderliggende dataset. |
| `legacy-mount` | Legacy mountpoint, overgeslagen. | Geef hem een mountpoint onder `/mnt`. |
| `no-mountpoint` | Geen mountpoint, overgeslagen. | Geef hem een mountpoint onder `/mnt`. |
| `not-mounted` | Niet gemount op de server, overgeslagen. | Voer `zfs mount` uit, of zet `canmount=on`. |
| `key-not-loaded` | Versleuteld en de sleutel is niet geladen, overgeslagen. | `zfs load-key`, en mount hem dan. |
| `snapdir-disabled` | Snapshottoegang staat uit, overgeslagen. | `zfs set snapdir=hidden <dataset>`. De map `.zfs` blijft verborgen. |
| `not-visible` | BombVault kan het mountpoint van de dataset niet zien. | Verplaats het mountpoint onder het pad van Host Data, of koppel het op hetzelfde pad in de container met Read/Write - Slave. |
| `shfs-only` | De dataset is alleen zichtbaar via `/mnt/user`, dat snapshots verbergt. | Koppel `/mnt`, niet `/mnt/user`, als Host Data. |
| `snapshot-not-visible` | De snapshot is gemaakt maar verscheen niet in BombVault. | Voer **Toegang tot snapshots testen** uit. Zie hieronder. |
| `snapshot-loop` | De snapshot bereikte BombVault niet omdat Host Data nieuwe mounts niet doorgeeft. | Zet de Access Mode van Host Data op Read/Write - Slave en herstart BombVault. |
| `backup-failed` | restic mislukte voor deze dataset. | De rundetails tonen waarom. |
| `not-reached` | De run eindigde vóór deze dataset. | Voer de back-up opnieuw uit. |
| `gone` | De dataset staat niet meer op de server. | Niets. Zijn back-ups blijven herstelbaar. |
| `read-only-mount` | BombVault kan de dataset alleen lezen, dus kan er niet in herstellen. | Zet de koppeling op Read/Write - Slave, of herstel naar een map. |
| `destination-not-mounted` | De map staat niet op een gemounte pool of share. | Kies een map op een pool of share. |
| `not-enough-space` | Niet genoeg vrije ruimte op de bestemming. | Maak ruimte vrij of kies een andere map. |
| `safety-snapshot-failed` | De veiligheidssnapshot kon niet worden gemaakt, dus er is niets hersteld. | De details tonen het bericht van zfs. |
| `safety-name-too-long` | De datasetnaam is te lang voor een veiligheidssnapshot. | Schakel de veiligheidssnapshot uit, of herstel naar een map. |
| `dataset-exists` | Er bestaat al een dataset met deze naam. | Kies een nieuwe naam, of herstel in de dataset zelf. |
| `create-failed` | De nieuwe dataset kon niet worden aangemaakt. | De details tonen het bericht van zfs. Controleer of de dataset erboven bestaat. |
| `new-dataset-not-visible` | De nieuwe dataset is aangemaakt, maar BombVault kan haar niet zien, dus er is niets hersteld. | De dataset blijft op de server. Mount haar onder het pad van Host Data en herstel erin. |
| `set-properties-failed` | De opgeslagen eigenschappen konden niet worden ingesteld, dus er is niets hersteld. | De details tonen het bericht van zfs. |
| `set-limits-failed` | De bestanden zijn hersteld, maar het opgeslagen quotum of de reservering kon niet worden ingesteld. | De details tonen het bericht van zfs. Stel het quotum of de reservering zelf in met `zfs set`. |

### Controleren wat de container ziet {#mountinfo}

**Toegang tot snapshots testen** op het tabblad Instellingen van het item maakt een echte snapshot van zijn boom, zoekt ernaar in BombVault voor elke dataset en verwijdert hem weer. Het is de snelste manier om het hele pad te bewijzen vóór de eerste geplande run.

Om zelf te kijken, voer je dit uit op de server:

```
docker exec BombVault grep zfs /proc/self/mountinfo
```

Elke regel is één mount in de container. De regel van een dataset toont zijn pad in de container (onder `/host/user`) en de datasetnaam. Een veld `master:N` op die regel betekent dat de mount de mounts ontvangt die de host later maakt, wat snapshottoegang nodig heeft. Ontbreekt het, zet dan de Access Mode van Host Data op Read/Write - Slave en herstart BombVault.

## TrueNAS SCALE {#truenas}

- Is `LIBVIRT_URI` ingesteld (zoals voor VM-back-ups op TrueNAS), dan neemt BombVault de SSH-host, gebruiker en poort voor zijn zfs-opdrachten uit de URI, elk die niet zelf is ingesteld. Zonder VM-back-ups hoeft niets te worden ingesteld: `host.docker.internal` lost in een TrueNAS-app niet op, dus BombVault bereikt de host via de standaardgateway van de app, als root op poort 22. Stel `LIBVIRT_HOST`, `LIBVIRT_SSH_USER` en `LIBVIRT_SSH_PORT` in als je een ander adres of een andere gebruiker wilt, in de `environment` van de app.
- Voeg de Host-SSH-sleutel uit **Instellingen, Integraties, Host-SSH** toe aan root onder **Credentials, Users**, en zet de SSH-service aan onder **System, Services**. Een [replica](#replica-push) naar deze of een andere TrueNAS heeft ook de replicasleutel nodig, wat een andere is.
- Een andere gebruiker dan root heeft rechten nodig op de bovenste dataset van het item, die dan elke dataset eronder dekken:

  ```
  zfs allow <user> snapshot,destroy,mount <dataset>
  ```

  Herstellen in een nieuwe dataset heeft ook `create` nodig op de dataset erboven, en het instellen van opgeslagen eigenschappen heeft rechten voor die eigenschappen nodig.

  Een niet-root-SSH-sessie op TrueNAS heeft `/usr/sbin` niet in zijn pad. BombVault roept dan `/usr/sbin/zfs` rechtstreeks aan.
- De **Host Data** van de app moet een hostpad boven de datasets zijn, bijvoorbeeld `/mnt/tank`, geen ixVolume. Met een hostpad geeft de app de nieuwe mounts van de host door aan BombVault (`rslave`), wat snapshottoegang nodig heeft. Zie [TrueNAS SCALE](getting-started.md#truenas) voor de hele appinstallatie.
