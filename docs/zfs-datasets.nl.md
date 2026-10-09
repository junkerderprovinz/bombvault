# ZFS-datasets

De pagina **ZFS** maakt back-ups van ZFS-datasets. Een item is één dataset samen met elke dataset eronder. Voor elke back-up maakt BombVault één ZFS-snapshot van de hele boom, zodat elke dataset erin op hetzelfde moment wordt vastgelegd. Daarna leest het de bestanden van elke dataset uit die snapshot, slaat ze met restic op zoals het een map opslaat en verwijdert de snapshot meteen daarna. De back-ups zijn gededupliceerd, je kunt elke back-up doorbladeren en losse bestanden kunnen worden teruggezet.

De back-up stuurt nooit een stream naar restic en zet nooit een dataset terug naar een eerdere stand. BombVault vernietigt alleen snapshots die het zelf heeft gemaakt. De optionele [replica](#replica) is de enige plek die `zfs send` gebruikt: ze kopieert de datasets naar een tweede ZFS-server en raakt de back-up niet aan.

## Vereisten {#requirements}

- **De SSH-verbinding met deze server.** ZFS-datasets gebruiken dezelfde sleutel, host en gebruiker als VM-back-ups. Werken VM-back-ups al, dan werkt dit ook. Volg anders de [handleiding VM-back-up via SSH](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) op GitHub. De templatevelden heten **Host SSH: Address**, **Host SSH: Port** en **Host SSH: User**.
- **Het commando `zfs` op die host.** Unraid 6.12 en nieuwer en TrueNAS SCALE hebben het.
- **Host Data gemapt als `/mnt` met Access Mode Read/Write - Slave.** Dat is de standaard van de template. De snapshot van een dataset verschijnt pas in de map `.zfs/snapshot` van de dataset nadat BombVault is gestart, dus de container moet de mounts ontvangen die de host later maakt.
- **De datasets gemount onder `/mnt`.** Op Unraid staan pools onder `/mnt/<pool>`, dus dat is al zo.

Schakel het domein in onder **Instellingen, Algemeen** (ZFS-datasets). De ZFS-pagina toont dan een kaart **Verbinding met deze server**. Die test de SSH-verbinding, noemt de gebruiker en host waarmee hij verbindt en zegt wat er ontbreekt als er iets ontbreekt. De controle van de hostintegratie (`/spike`) toont hetzelfde resultaat.

## Items en onderliggende datasets {#items-and-children}

Open **Datasets toevoegen** op de ZFS-pagina. De lijst komt van de server. Kies de dataset die het hoogst staat van wat je wilt back-uppen, bijvoorbeeld `cache/appdata`, en het item omvat die dataset en elke dataset eronder.

- **Nieuwe onderliggende datasets komen er vanzelf bij.** Een dataset die later onder het item wordt aangemaakt, wordt bij de volgende run geback-upt, en die run meldt hem als nieuw. Zijn eerste back-up leest hem één keer volledig; daarna worden alleen wijzigingen gelezen.
- **Je kunt losse onderliggende datasets weglaten.** Schakel er een uit in de instellingen van het item en hij blijft buiten, samen met alles eronder. Een weggelaten onderliggende dataset die niet meer op de server bestaat, wordt zo gemarkeerd en kan uit de lijst worden verwijderd.
- **Onderliggende datasets die niet leesbaar zijn, worden overgeslagen, nooit stilletjes.** De run somt ze op, het item toont hoeveel er zijn overgeslagen en de dekkingskaart op het dashboard telt elk ervan als niet beschermd. De run back-upt toch al het andere en mislukt niet door een overgeslagen dataset. De redenen staan in de [tabel met redencodes](#reason-codes): een dataset die niet gemount is, `canmount=off` heeft, een `legacy`- of geen mountpoint, een versleutelingssleutel die niet geladen is, uitgeschakelde snapshottoegang of een mountpoint dat BombVault niet ziet.
- **Een overgeslagen dataset neemt zijn onderliggende datasets niet mee.** Een dataset met `canmount=off` die alleen andere datasets bevat, wordt overgeslagen (getoond als "alleen structuur") en zijn gemounte onderliggende datasets worden geback-upt. Een versleutelde dataset waarvan de sleutel niet geladen is, wordt overgeslagen samen met de onderliggende datasets die zijn sleutel delen.
- **Onderliggende datasets die VM-schijven of systeemdata zijn, beginnen uitgeschakeld** in het toevoegvenster, met de reden naast de schakelaar. Een hele pool toevoegen vraagt om een bevestiging die opsomt wat erin zit.

### Volumes {#volumes}

Een volume (zvol) bevat een virtuele schijf in plaats van bestanden, en de ZFS-pagina back-upt er nooit een.

- Een volume dat een VM gebruikt, wordt met die VM geback-upt op de pagina **VM's**.
- Een volume dat geen enkele VM gebruikt (een iSCSI-extent, een losgekoppelde schijf) **wordt niet door BombVault geback-upt**. Het toevoegvenster en de ZFS-pagina tellen deze volumes en zeggen dat. Een latere versie zal ze back-uppen.

Volumes in de boom van een item worden bij elke run overgeslagen en genoemd.

### De opslag van Docker {#docker-storage}

Met de ZFS-opslagdriver van Docker is elke imagelaag een dataset met een `legacy`-mountpoint. Het toevoegvenster voegt die samen tot één regel per bovenliggende dataset. Een boom met meer dan 20 van zulke datasets kan geen item worden: zolang er een snapshot van bestaat, kan Docker geen imagelagen verwijderen. Voeg in plaats daarvan de datasets eronder toe, bijvoorbeeld `appdata`.

### Items overlappen nooit {#overlap}

Een dataset kan maar bij één item horen. BombVault weigert een nieuw item dat in een bestaand item ligt of er een zou bevatten. Om meerdere onderliggende items samen te voegen tot één bovenliggend item, verwijder je eerst de onderliggende items en kies je ervoor hun back-ups te bewaren, en voeg je daarna het bovenliggende item toe. Elke dataset houdt zijn geschiedenis onder zijn eigen naam, dus de volgende back-up gaat verder waar de oude items waren gebleven en leest niet alles opnieuw.

## Containers stoppen en commando's rond de snapshot {#consistency}

Een snapshot van een draaiende database is als een plotselinge stroomstoring: de database herstelt zich meestal, maar moet dat wel doen. Elk item kan daar twee dingen aan doen, en beide gelden alleen voor het moment van de snapshot, niet voor de hele back-up.

- **Deze containers stoppen voor het snapshot.** BombVault stopt de genoemde containers, maakt de snapshot en start ze meteen weer. Containers op hetzelfde afhankelijkheidsniveau stoppen parallel, afhankelijke eerst, dus het hele venster duurt meestal een paar seconden; de run toont hoe lang. De back-up leest daarna de bevroren snapshot terwijl de apps alweer draaien. Alleen containers die draaiden, worden gestopt.
- **Een commando voor en na de snapshot.** Het draait in een container naar keuze, bijvoorbeeld om vlak voor de snapshot een database naar de dataset te dumpen, zonder iets te stoppen. Mislukt het commando voor de snapshot, dan mislukt de back-up en wordt er geen snapshot gemaakt. Een mislukt commando na de snapshot wordt bij de run getoond, maar laat de back-up niet mislukken.

Wat er gebeurt als er iets misgaat:

- Kan een container niet worden gestopt, dan start BombVault de containers die het al had gestopt en mislukt de back-up met de naam van de container. Het valt nooit terug op een snapshot van draaiende apps.
- Het stoppen wacht tot een lopende containerback-up klaar is (tot 30 minuten bij een handmatige run, tot de tijdslimiet van de back-up bij een geplande), zodat die twee nooit tegelijk dezelfde container stoppen en starten.
- Voordat de eerste container stopt, noteert BombVault welke het stopt. Wordt BombVault binnen het venster beëindigd, dan start het die containers bij de volgende start opnieuw, stuurt een melding, en het item toont een rode notitie voor elke container die het niet kon starten.

De automatische databasedumps (zie [Functies](features.md)) draaien met de eigen back-up van een container op de pagina **Containers**, niet met een ZFS-item. Een database waarvan de container alleen via zijn dataset wordt geback-upt, krijgt geen dump, dus geef hem hier een commando.

Een container kan tegelijk op deze lijst en op de pagina **Containers** staan. Zijn data wordt dan twee keer opgeslagen, in twee repository's, en de **Volledige back-up** stopt hem twee keer. Het item wijst daarop.

## Terugzetten {#restore}

Open **Back-ups** bij het item, kies de back-up en daarna de dataset. Standaard is dat de bovenste dataset van het item.

- **Terugzetten in de dataset.** Bestanden uit de back-up worden in het mountpoint van de dataset geschreven. Bestanden met dezelfde naam worden overschreven, andere blijven staan. De dataset wordt nooit teruggezet naar een eerdere stand of vervangen. BombVault controleert of de dataset gemount, zichtbaar en beschrijfbaar is, één keer voor het begint en opnieuw vlak voor het schrijft. Waar een onderliggende dataset erin is gemount, wordt niets geschreven: die behoudt zijn bestanden, eigenaar en rechten en wordt teruggezet uit zijn eigen back-up.
- **Terugzetten naar een map.** Kies een map onder `/mnt`. BombVault controleert of de map op een gemounte pool of share staat en of er genoeg vrije ruimte is. Dit werkt zonder de SSH-verbinding en voor datasets die niet meer bestaan.
- **In een nieuwe dataset.** Geef een dataset op die nog niet bestaat. BombVault maakt hem aan met de ZFS-eigenschappen uit de back-up en zet de bestanden erin terug, zie [Terugzetten als nieuwe dataset](#new-dataset).
- **Bestanden kiezen** (geavanceerd): alleen de bestanden en mappen die je kiest terugschrijven in de dataset.
- **Alle datasets van deze back-up** (geavanceerd): elke dataset van de boom in een eigen submap van de map die je kiest. Datasets die in die back-up werden overgeslagen, worden genoemd.
- **Van een andere server:** de pagina **Herstel** zet terug uit de repository van een andere BombVault, altijd naar een map: alle datasets van één back-up, elk in een eigen submap, of één dataset van de boom, geheel of gekozen bestanden.

De lijst met te stoppen containers van het item wordt ook aangeboden bij terugzetten in de dataset. Die containers blijven de hele tijd gestopt, en containerback-ups wachten zolang.

### De veiligheidssnapshot {#safety-snapshot}

Voordat het in een dataset schrijft, maakt BombVault een ZFS-snapshot van alleen die dataset, met de naam `bombvault-prerestore-<tijd>`. Die staat standaard aan; uitschakelen vraagt een tweede bevestiging. Kan de snapshot niet worden gemaakt, dan wordt er niets teruggezet.

BombVault verwijdert een veiligheidssnapshot nooit uit zichzelf. Het item somt ze op met leeftijd en grootte, elk met de actie **Verwijderen**, en waarschuwt als de oudste meer dan 30 dagen oud is, omdat die verwijderde en gewijzigde data op de pool vasthoudt.

Om na het terugzetten terug te gaan, kopieer je losse bestanden uit `.zfs/snapshot/bombvault-prerestore-<tijd>` in de dataset. `zfs rollback <dataset>@bombvault-prerestore-<tijd>` werkt alleen zolang het de nieuwste snapshot van die dataset is. `zfs rollback -r` verwijdert elke nieuwere snapshot, automatische inbegrepen.

### Terugzetten als nieuwe dataset {#new-dataset}

BombVault bewaart bij elke back-up de lokaal ingestelde ZFS-eigenschappen van elke dataset: compression, recordsize, quota, reservation, atime, xattr, acltype, casesensitivity en je eigen gebruikerseigenschappen. Geërfde en alleen-lezen waarden blijven weg, omdat ze vanzelf terugkomen. Back-ups van voordat BombVault ze bewaarde, hebben er geen.

- **In een nieuwe dataset** voert `zfs create` uit met elke bewaarde eigenschap. casesensitivity, normalization en utf8only kunnen alleen zo worden ingesteld. Quota en reserveringen volgen pas na de bestanden, zodat ze die niet kunnen weigeren. Het koppelpunt blijft weg zodat de kopie niet met het origineel botst, net als `canmount`, `readonly` en de versleuteling, zodat het herstel kan schrijven. Een nieuwe dataset onder een versleutelde neemt diens versleuteling over. De dataset erboven moet bestaan. Mislukt er iets nadat hij is aangemaakt, dan blijft de nieuwe dataset op de server, want BombVault vernietigt nooit een dataset.
- **Terugzetten in de dataset** toont de bewaarde eigenschappen naast het herstel. **Deze eigenschappen ook instellen** stelt de eigenschappen in die een bestaande dataset nog aanneemt, voordat er een bestand wordt geschreven. Quota en reserveringen volgen pas na de bestanden, zodat ze die niet kunnen weigeren. Zonder die schakelaar houdt de dataset zijn instellingen.

## Wat er in de back-up zit {#contents}

In de back-up: de bestanden en mappen van elke geback-upte dataset, met eigenaar, rechten, tijdstempels en uitgebreide attributen zoals restic ze opslaat, en de lokaal ingestelde ZFS-eigenschappen van elke dataset.

Niet in de back-up:

- de eigenaar en rechten van de bovenste map van elke dataset zelf (alles daaronder zit erin). Terugzetten in de dataset laat de bestaande bovenste map zoals hij is, terugzetten naar een map maakt hem aan met rechten `0755`;
- bestaande ZFS-snapshots;
- onderliggende datasets die werden overgeslagen of weggelaten;
- volumes.

Om op een nieuwe pool terug te zetten, maak je de pool aan en zet je elke dataset terug in een nieuwe dataset. Of NFSv4-ACL's, zoals TrueNAS die op SMB-datasets gebruikt, terugkomen zoals je verwacht, is nog niet gecontroleerd; test dus een terugzetting op je eigen data voordat je erop vertrouwt.

## Versleutelde datasets {#encryption}

Een versleutelde dataset wordt alleen geback-upt zolang zijn sleutel geladen is. Anders wordt hij overgeslagen met een waarschuwing; laad de sleutel met `zfs load-key` en mount de dataset. BombVault leest de data ontsleuteld en slaat ze op in de repository van restic, die versleuteld is. Heb je versleuteling in BombVault uitgeschakeld, dan is die repository dat niet.

## Replica {#replica}

Een replica is een kopie van de datasets van een item op een tweede ZFS-server. BombVault houdt ze actueel met `zfs send` en `zfs receive`. De eerste keer wordt alles verstuurd, daarna reizen alleen de gewijzigde blokken mee. Op de andere server kun je de kopie meteen mounten.

Een replica vervangt de back-up nooit. Oudere versies, losse bestanden en de controle komen nog steeds uit de back-ups, en de replica bewaart maar zoveel snapshots als je instelt. Een actuele replica telt als kopie buiten de locatie, maar een item met een replica en zonder back-up blijft oranje.

Schakel haar in op de kaart **Replica** in de instellingen van het item. Daar kies je waarheen de replica gaat, wanneer ze draait (**After every backup** of **Own plan**) en hoeveel snapshots er op het doel blijven. De kaart toont elke dataset en elk volume met zijn status, en **Nu repliceren** start een run. Een replica-run heeft een eigen vergrendeling, zodat een lange eerste overdracht de back-ups nooit ophoudt.

### Naar een ZFS-server sturen {#replica-push}

Elke machine met ZFS en SSH kan ontvangen, bijvoorbeeld een tweede Unraid of een TrueNAS. BombVault hoeft daar niet te draaien.

1. Open **Instanties, ZFS-servers** en klik op **ZFS-server toevoegen**.
2. Voer adres, gebruiker en poort in. Het venster toont de publieke sleutel van BombVault. Voeg die toe aan de `~/.ssh/authorized_keys` van de gebruiker op de server. Op Unraid staat dat onder **Settings, Users, root, SSH keys**.
3. Test de verbinding. Het venster toont dan de pools van de server. Kies er een en stel de root in, die standaard `<pool>/bombvault-replica` is.
4. Kies de nieuwe server op de kaart **Replica** van het item.

Met root is verder niets nodig. Een eigen gebruiker heeft deze rechten nodig op de pool van het doel, die het venster ook toont:

```
zfs allow <user> receive,create,mount,rollback,destroy,userprop <pool>
```

Aan de bronkant heeft dezelfde soort gebruiker deze rechten nodig op de bovenste dataset van het item:

```
zfs allow <user> send,snapshot,hold,release,bookmark,destroy <dataset>
```

In deze richting heeft de BombVault die het item beheert ook een sleutel waarmee op de server kan worden geschreven.

BombVault legt bij de eerste verbinding de hostsleutel vast die de server laat zien en weigert later elke andere. Is de server opnieuw geïnstalleerd, druk dan op de pagina van de server op **Hostsleutel vergeten**; de volgende verbinding legt de nieuwe sleutel vast.

### Versturen naar een gekoppelde instantie {#replica-receive}

Een gekoppelde BombVault kan de replica zelf ontvangen. Niemand krijgt SSH-toegang tot de andere host en er komt geen sleutel in een `authorized_keys`-bestand.

1. Kies op de kaart **Replica** van het item de gekoppelde instantie als doel. De kaart toont **Wacht op goedkeuring** totdat die antwoordt.
2. Open op de ontvangende instantie **Instanties**, dan **Ontvangen**. De kaart **ZFS** toont het verzoek. Kies de pool en root op die server en hoeveel snapshots blijven, en druk dan op **Toestaan** of **Afwijzen**.
3. Na **Toestaan** verstuurt de bron volgens haar eigen schema, net als bij elk ander doel.

De ontvangende instantie accepteert alleen wat de goedkeuring dekt: de datasets van het item, in haar eigen root. Ze voert zelf `zfs receive` uit en de bron kan daar niets verwijderen of terugdraaien. Zo overleeft de kopie een bron die iemand heeft overgenomen. De ontvangende instantie houdt haar eigen retentie aan. De bron stelt alleen een regel voor wanneer ze het verzoek doet.

**Revoke access** op de ontvangende instantie beëindigt de goedkeuring op elk moment en meldt dat aan de bron, die dan toont dat de goedkeuring is ingetrokken en stopt. Wat de ontvangende instantie al heeft, blijft daar staan. Een afgewezen verzoek blijft afgewezen. Meer datasets, of een verzoek na een intrekking, vragen opnieuw om goedkeuring.

De ontvangende instantie moet via HTTPS antwoorden. Naar een gewoon http-adres stuurt de bron niets, omdat het token en de data dan onversleuteld over het netwerk zouden gaan. Elke overdracht laat bovendien een tiende van de ontvangende pool vrij en stopt voordat ze daaraan zou komen, zodat één bron de pool van de ontvangende server niet kan vullen.

### Waar de data terechtkomt {#replica-target}

Elke dataset komt terecht op `<root>/<server>/<pool>/<path>`. De servermap is de naam van de broninstantie, vastgelegd bij de eerste overdracht, zodat twee servers met dezelfde poolnaam elkaar nooit in de weg zitten. `cache/appdata` van een server met de naam `tower` komt bijvoorbeeld terecht op `backup/bombvault-replica/tower/cache/appdata`.

De kopie op het doel is alleen-lezen en niet gemount, dus ze bedekt nooit iets op die server. ZFS-eigenschappen reizen mee, behalve het mountpoint en `sharenfs` en `sharesmb`. Ook de reserveringen `reservation` en `refreservation` blijven achter, zodat de kopie alleen de ruimte inneemt die haar data nodig heeft.

### Wat erin zit {#replica-contents}

Alles wat het item back-upt, gaat erin, en ook de volumes eronder, die de back-up overslaat. Een onderliggende dataset die je in het item hebt uitgeschakeld, blijft erbuiten. Alle datasets van een run komen uit één snapshot, net als bij de back-up.

### Hoe lang snapshots blijven {#replica-retention}

Op het doel bewaart een nieuwe replica 7 dagelijkse en 3 wekelijkse snapshots. Kies in plaats daarvan **Short**, **Balanced** of **Long**, of stel **Custom values** in. Alleen snapshots met de naam `bombvault-replica-<14 digits>` worden daar ooit verwijderd, en nooit de nieuwste die beide kanten delen.

Aan de bronkant bewaart BombVault alleen de laatste replica-snapshot, plus een bookmark voor elke verstuurde stand. Bookmarks kosten geen ruimte. De volgende overdracht begint daarvandaan.

### Versleutelde datasets in een replica {#replica-encryption}

Een versleutelde dataset wordt raw verstuurd. Hij blijft op het doel versleuteld en het doel ziet de sleutel nooit. Bewaar de sleutel goed: je hebt hem nodig om de kopie na een herstel te openen, en een replica zonder sleutel is onleesbaar.

### De replica gebruiken {#replica-use}

Open het tabblad **Back-ups** van het item en klik op de replicarij op de kaart **Storage locations**. Het blad toont de snapshots op het doel en de commando's met je echte namen.

Om een oude stand te bekijken, kloon je een snapshot op het doel. Een kloon neemt geen ruimte in totdat er iets verandert, en de replica blijft onaangeraakt:

```
zfs clone backup/bombvault-replica/tower/cache/appdata@bombvault-replica-20261006014100 backup/bombvault-replica/clone-appdata
```

Als de bron uitvalt, maak je van de kopie op het doel een gewone, beschrijfbare dataset:

```
zfs inherit -r readonly backup/bombvault-replica/tower/cache/appdata && zfs inherit -r canmount backup/bombvault-replica/tower/cache/appdata && zfs mount -a
```

BombVault stopt daarna met repliceren naar die dataset, totdat je een nieuwe eerste run start.

Om een stand terug te brengen naar de bron, druk je in het blad op **Bring back as a new dataset**. BombVault stuurt de snapshot naar een nieuwe dataset naast het origineel, met de naam `<dataset>-bombvault-restore-` plus een tijdstempel. Het origineel wordt nooit overschreven.

## Achtergebleven snapshots {#leftover-snapshots}

De snapshot van een back-up heet `<dataset>@bombvault-<14 cijfers>`, bijvoorbeeld `cache/appdata@bombvault-20260924021500` (UTC). BombVault verwijdert hem direct na de back-up. Lukt dat niet, bijvoorbeeld omdat de dataset bezet is of BombVault werd gestopt, dan verwijdert BombVault hem:

- voor de volgende back-up van dat item,
- wanneer BombVault start, voor elk item, ook met het domein uitgeschakeld,
- wanneer je het item verwijdert,
- wanneer je bij het item op **Nu verwijderen** drukt, dat ook toont hoeveel er over zijn.

Alleen namen die precies `bombvault-` plus 14 cijfers zijn, worden verwijderd. Veiligheidssnapshots, je eigen snapshots en automatische snapshots worden nooit aangeraakt. Om er een met de hand te verwijderen:

```
zfs destroy -r cache/appdata@bombvault-20260924021500
```

De snapshot van een replica heet `<dataset>@bombvault-replica-<14 digits>` en is geen restant. Hij blijft op de bron staan totdat de volgende replica-run hem vervangt, en op het doel zolang de retentie hem bewaart. De opruimer raakt hem nooit aan, omdat die alleen `bombvault-` gevolgd door precies 14 cijfers herkent.

## Anomalieën {#anomalies}

Een onderliggende dataset die werd leeggemaakt, verandert het totaal van een grote boom nauwelijks, dus anomaliedetectie bekijkt elke dataset van een item afzonderlijk: grootte, aantal bestanden, nieuwe data en restic-tijd hebben elk hun eigen geschiedenis. Die geschiedenis hoort bij de naam van de dataset, dus ze blijft als de boom later door een ander item wordt geback-upt.

Een dataset die de vorige run heeft geback-upt en die deze run niet kon lezen, telt als leeggemaakt, zolang de selectie van het item niet veranderde. Dat dekt een sleutel die niet geladen is, een dataset die niet gemount is en een die uit de boom is verdwenen. Een onderliggende dataset die je zelf uitsluit, verandert de selectie, dus zijn geschiedenis begint dan opnieuw. Zolang een bevinding over verloren data open staat, bewaart de retentie de oude back-ups van alleen die dataset en snoeit de rest van de boom zoals gewoonlijk.

Op de pagina **Anomalieën** heeft elke dataset een eigen regel in het paneel van het item, dat opengaat met **Bewaking** op de kaart van het item of, als er voor het item niets open is, vanuit zijn regel in de kaart **Niets open**. De boom van het item op deze pagina toont de open bevindingen naast elke dataset. De link in een bevinding opent het terugzetpaneel van het item bij de laatste goede back-up van de dataset. Of een run afloopt, wordt beoordeeld voor het hele item, omdat een run als geheel slaagt of mislukt.

De controles zelf staan beschreven onder [Functies](features.md). Een assistent die via de [MCP-server](mcp.md) is verbonden, kan de herstelpunten van een ZFS-item opsommen, zijn back-up starten en de bevindingen lezen, maar een bevinding bevestigen gebeurt op de pagina **Anomalieën**.

## Redencodes {#reason-codes}

De pagina, de rungeschiedenis en de meldingen noemen een probleem met een van deze codes. Bij de meeste staat de oplossing ook ernaast op de pagina.

| Code | Betekenis | Wat te doen |
|---|---|---|
| `ssh-missing` | De SSH-verbinding is in deze container niet ingesteld. | Stel de SSH-verbinding in zoals voor VM-back-ups. |
| `host-placeholder` | Host SSH: Address is nog de voorbeeldwaarde, en `host.docker.internal` antwoordde ook niet. | Zet Host SSH: Address op het LAN-IP van deze server. |
| `host-fallback` | Host SSH: Address is nog de voorbeeldwaarde, en `host.docker.internal` werkt. | Niets, of zet het LAN-IP. |
| `ssh-unreachable` | De server is niet bereikbaar via SSH. | Controleer adres en poort, en of SSH aan staat. |
| `ssh-auth` | De server weigerde de sleutel van BombVault. | Voer het commando van de verbindingskaart één keer uit op de server. |
| `zfs-not-found` | De SSH-host heeft geen commando `zfs`. | Laat Host SSH: Address wijzen naar de machine die de pools bezit. |
| `zfs-permission` | De SSH-gebruiker mag dit zfs-commando niet uitvoeren. | Gebruik root, of zie [TrueNAS SCALE](#truenas). |
| `uri-mismatch` | `LIBVIRT_URI` noemt een andere host of gebruiker dan de SSH-velden. | Laat ze overeenkomen, of maak de SSH-velden leeg zodat beide uit de URI komen. |
| `zfs-error` | zfs meldde een andere fout. | De details tonen de melding. |
| `propagation-missing` | Nieuwe mounts op de host bereiken de container niet. | Zet de Access Mode van Host Data op Read/Write - Slave en herstart BombVault. |
| `invalid-name` | Een datasetnaam die BombVault niet accepteert. | Hernoem de dataset. |
| `name-too-long` | Een dataset in de boom is te lang voor een snapshotnaam. | Hernoem hem, of voeg een dataset eronder toe als item. |
| `invalid-exclude` | Een uitsluitpatroon of een weggelaten onderliggende dataset past niet bij het item. | Corrigeer de invoer die de melding noemt. Om een hele onderliggende dataset weg te laten, schakel je hem uit in plaats van een patroon te schrijven. |
| `not-found` | De dataset bestaat niet op de server. | Verwijder het item of maak de dataset opnieuw aan. Zijn back-ups blijven terug te zetten. |
| `not-filesystem` | Dit is een volume, geen bestandssysteem. | Zie [Volumes](#volumes). |
| `overlaps-item` | De dataset overlapt met een bestaand item. | Zie [Items overlappen nooit](#overlap). |
| `docker-storage` | De boom bevat de imageopslag van Docker. | Zie [De opslag van Docker](#docker-storage). |
| `nothing-readable` | Geen enkele dataset in het item is op dit moment leesbaar. | Bekijk de codes van de overgeslagen datasets. |
| `snapshot-failed` | De snapshot kon niet worden gemaakt. | De details tonen de melding van zfs. |
| `containers-busy` | Er liep nog een containerback-up toen de containers moesten stoppen. | Start later opnieuw. Geplande runs wachten vanzelf. |
| `consistency-stop-failed` | Een container kon niet worden gestopt, dus er is geen snapshot gemaakt. | Controleer de container, of haal hem van de lijst. |
| `pre-snapshot-failed` | Het commando voor de snapshot is mislukt. | De rundetails tonen de uitvoer. |
| `container-unknown` | Een genoemde container bestaat niet. | Haal hem van de lijst. |
| `container-is-self` | BombVault kan zijn eigen container niet stoppen. | Haal hem van de lijst. |
| `leftover-snapshots` | Snapshots die BombVault niet kon verwijderen, staan nog op de server. | Druk op **Nu verwijderen**, zie [Achtergebleven snapshots](#leftover-snapshots). |
| `zvol` | Een volume in de boom, overgeslagen. | Zie [Volumes](#volumes). |
| `canmount-off` | Nooit gemount (`canmount=off`), overgeslagen. | Staat er data in, mount hem dan of verplaats de data naar een onderliggende dataset. |
| `legacy-mount` | Legacy-mountpoint, overgeslagen. | Geef hem een mountpoint onder `/mnt`. |
| `no-mountpoint` | Geen mountpoint, overgeslagen. | Geef hem een mountpoint onder `/mnt`. |
| `not-mounted` | Niet gemount op de server, overgeslagen. | Mount hem met `zfs mount`, of zet `canmount=on`. |
| `key-not-loaded` | Versleuteld en de sleutel is niet geladen, overgeslagen. | `zfs load-key`, daarna mounten. |
| `snapdir-disabled` | Snapshottoegang staat uit, overgeslagen. | `zfs set snapdir=hidden <dataset>`. De map `.zfs` blijft verborgen. |
| `not-visible` | BombVault ziet het mountpoint van de dataset niet. | Verplaats het mountpoint onder het Host Data-pad, of map het in de container op hetzelfde pad met Read/Write - Slave. |
| `shfs-only` | De dataset is alleen zichtbaar via `/mnt/user`, dat snapshots verbergt. | Map `/mnt`, niet `/mnt/user`, als Host Data. |
| `snapshot-not-visible` | De snapshot is gemaakt maar verscheen niet in BombVault. | Voer **Toegang tot snapshots testen** uit; zie hieronder. |
| `snapshot-loop` | De snapshot bereikte BombVault niet omdat Host Data nieuwe mounts niet doorgeeft. | Zet de Access Mode van Host Data op Read/Write - Slave en herstart BombVault. |
| `backup-failed` | restic is mislukt voor deze dataset. | De rundetails tonen waarom. |
| `not-reached` | De run eindigde voor deze dataset. | Voer de back-up opnieuw uit. |
| `gone` | De dataset staat niet meer op de server. | Niets. Zijn back-ups blijven terug te zetten. |
| `read-only-mount` | BombVault kan de dataset alleen lezen en er dus niet in terugzetten. | Zet de mapping op Read/Write - Slave, of zet terug naar een map. |
| `destination-not-mounted` | De map staat niet op een gemounte pool of share. | Kies een map op een pool of share. |
| `not-enough-space` | Niet genoeg vrije ruimte op de bestemming. | Maak ruimte vrij of kies een andere map. |
| `safety-snapshot-failed` | De veiligheidssnapshot kon niet worden gemaakt, dus er is niets teruggezet. | De details tonen de melding van zfs. |
| `safety-name-too-long` | De datasetnaam is te lang voor een veiligheidssnapshot. | Schakel de veiligheidssnapshot uit, of zet terug naar een map. |
| `dataset-exists` | Er bestaat al een dataset met deze naam. | Kies een nieuwe naam, of herstel in de dataset zelf. |
| `create-failed` | De nieuwe dataset kon niet worden aangemaakt. | De details tonen de melding van zfs. Controleer of de dataset erboven bestaat. |
| `new-dataset-not-visible` | De nieuwe dataset is aangemaakt, maar BombVault ziet hem niet, dus er is niets hersteld. | De dataset blijft op de server. Koppel hem onder het Host Data-pad en herstel erin. |
| `set-properties-failed` | De opgeslagen eigenschappen konden niet worden ingesteld, dus er is niets hersteld. | De details tonen de melding van zfs. |
| `set-limits-failed` | De bestanden zijn hersteld, maar het opgeslagen quotum of de reservering kon niet worden ingesteld. | De details tonen de melding van zfs. Stel het quotum of de reservering zelf in met `zfs set`. |

### Controleren wat de container ziet {#mountinfo}

**Toegang tot snapshots testen** bij een item maakt een echte snapshot van zijn boom, zoekt hem in BombVault voor elke dataset en verwijdert hem weer. Het is de snelste manier om het hele pad te bewijzen voor de eerste geplande run.

Om zelf te kijken, voer je dit uit op de server:

```
docker exec BombVault grep zfs /proc/self/mountinfo
```

Elke regel is een mount in de container. De regel van een dataset toont zijn pad in de container (onder `/host/user`) en de datasetnaam. Een veld `master:N` op die regel betekent dat de mount de mounts ontvangt die de host later maakt, en dat is wat snapshottoegang nodig heeft. Ontbreekt het, zet dan de Access Mode van Host Data op Read/Write - Slave en herstart BombVault.

## TrueNAS SCALE {#truenas}

- Als `LIBVIRT_URI` is ingesteld (zoals voor VM-back-ups op TrueNAS), haalt BombVault de SSH-host, gebruiker en poort voor zijn zfs-commando's uit de URI, elk die niet apart is ingesteld. Zonder VM-back-ups stel je in plaats daarvan `LIBVIRT_HOST`, `LIBVIRT_SSH_USER` en `LIBVIRT_SSH_PORT` in. Voeg de variabelen toe onder **Additional Environment Variables**.
- Een andere gebruiker dan root heeft rechten nodig op de bovenste dataset van het item, die dan elke dataset eronder dekken:

  ```
  zfs allow <user> snapshot,destroy,mount <dataset>
  ```

  Terugzetten in een nieuwe dataset heeft daarnaast `create` nodig op de dataset erboven, en bewaarde eigenschappen instellen heeft rechten op die eigenschappen nodig.

  Een SSH-sessie zonder root heeft op TrueNAS `/usr/sbin` niet in het pad; BombVault roept dan direct `/usr/sbin/zfs` aan.
- **Host Data** van de app moet een hostpad boven de datasets zijn, bijvoorbeeld `/mnt/tank`, geen ixVolume. Met een hostpad geeft de app nieuwe mounts van de host door aan BombVault (`rslave`), en dat heeft snapshottoegang nodig.
