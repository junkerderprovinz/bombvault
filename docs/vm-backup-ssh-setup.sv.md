# VM-säkerhetskopior

BombVault säkerhetskopierar KVM/libvirt-VM:ar utan att montera någon libvirt-sökväg. Den kör `virsh` på värden över SSH (`qemu+ssh://`), läser och skriver VM-diskarna via Host Data-monteringen (`/mnt` på värden är `/host/user` i containern) och kopierar UEFI NVRAM över SSH. Eftersom inget som hör till libvirt bind-monteras kan BombVault aldrig komma i vägen för Unraids VM Manager.

Den här guiden täcker uppsättningen, säkerhetskopieringsmetoderna, VM:ar med Unraid-ögonblicksbilder och TrueNAS SCALE. Containersäkerhetskopior behöver inget av detta.

Säkerhetskopior av ZFS-datauppsättningar använder samma SSH-anslutning, med samma nyckel, värd och användare. När VM-säkerhetskopior ansluter gör ZFS-poster det också. Se [ZFS-datauppsättningar](zfs-datasets.md).

## Vad du ställer in {#what-you-configure}

| Inställning | Var | Standard | Betydelse |
|---|---|---|---|
| **Host SSH: Address** (`LIBVIRT_HOST`) | mallvariabel | `host.docker.internal` | Unraid-värden, nådd över SSH |
| **Host SSH: Port** (`LIBVIRT_SSH_PORT`) | mallvariabel | `22` | Unraids SSH-port |
| **Host SSH: User** (`LIBVIRT_SSH_USER`) | mallvariabel | `root` | SSH-användaren på värden |
| Publik nyckel | **Inställningar, Integrationer, Värd-SSH** | skapas vid första starten | Auktorisera den på värden |

BombVault skapar sitt SSH-nyckelpar vid första starten på `/config/ssh/id_ed25519`, som ligger kvar i appdata. Värdens nyckel fästs i `/config/ssh/known_hosts` vid den första anslutningen.

Mallen fyller **Host SSH: Address** med platshållaren `192.168.x.x`. BombVault behandlar den som osatt och provar `host.docker.internal`, som fungerar på standardnätverket `bridge`. På ett anpassat nätverk byter du ut den mot värdens LAN-IP (steg 3 B).

## Steg 1: Slå på SSH på Unraid-värden {#step-1}

1. Öppna **Settings, Management Access** i Unraid och sätt **Use SSH** till **Yes**.
2. Om du ändrat SSH-porten, notera den. **Host SSH: Port** måste stämma.

## Steg 2: Auktorisera BombVaults publika nyckel {#step-2}

Unraids `/root` ligger i minnet och töms vid varje uppstart, så nyckeln måste finnas på flashenheten också. Kör det här i Unraid-terminalen:

```sh
K=$(docker exec BombVault cat /config/ssh/id_ed25519.pub)
mkdir -p /root/.ssh /boot/config/ssh && chmod 700 /root/.ssh
grep -qxF "$K" /root/.ssh/authorized_keys 2>/dev/null  || echo "$K" >> /root/.ssh/authorized_keys
grep -qxF "$K" /boot/config/ssh/root.pubkeys 2>/dev/null || echo "$K" >> /boot/config/ssh/root.pubkeys
chmod 600 /root/.ssh/authorized_keys
```

Unraid 6.9 och nyare kopierar `/boot/config/ssh/root.pubkeys` till roots `authorized_keys` vid varje uppstart, så detta överlever omstarter. Det gäller för vilken `root`-inloggning som helst, oavsett vilket värdnamn eller vilken port BombVault använder.

## Steg 3: Nätverk {#step-3}

BombVault måste öppna en TCP-anslutning från containern till värdens SSH-port. Välj det fall som passar din uppsättning.

### A. BombVault på standardnätverket bridge {#bridge}

- Låt **Host SSH: Address** stå på `host.docker.internal` eller mallens platshållare. Mallen lägger till `--add-host=host.docker.internal:host-gateway`, som pekar på docker0-gatewayen, alltså värden.
- Sätt **Host SSH: Port** till din SSH-port.

### B. BombVault på ett anpassat nätverk {#custom-network}

En container på `br0.x`, macvlan eller ipvlan når inte värden via `host.docker.internal`, eftersom 172.17.0.1 är docker0 och inte nåbar från `br0.x`. Gör i stället så här:

1. Sätt **Settings, Docker, Host access to custom networks** till **Enabled** i Unraid.
2. Sätt **Host SSH: Address** till Unraid-värdens LAN-IP, adressen du öppnar webbgränssnittet på.
3. Om containerns nätverk och värden ligger i olika VLAN tillåter du rutten i din router eller brandvägg: från container-VLAN:et till värdens IP på SSH-porten, över TCP.
4. Om värdens LAN-IP fortfarande inte går att nå använder du värdens shim-IP på containerns subnät. Hitta den på värden med `ip -4 addr show | grep -B2 <container-subnet>`.

### Kontrollera att värden svarar {#verify-reachability}

Kör det här i Unraid-terminalen, med din IP och port:

```sh
docker exec BombVault timeout 6 bash -c 'echo > /dev/tcp/192.168.1.10/22' && echo OPEN || echo UNREACHABLE
```

`OPEN` betyder att vägen fungerar. `UNREACHABLE` betyder att nätverket i steg 3 måste fixas före allt annat.

## Steg 4: Sätt variablerna och testa {#step-4}

1. Öppna **Docker** i Unraid, klicka på BombVault, välj **Edit**, sätt **Host SSH: Address** (och **Host SSH: Port** om den inte är 22) och tryck på **Apply**. Om variablerna saknas, importera mallen igen, eftersom Unraid behåller den sparade konfigurationen för en befintlig container.
2. Öppna **Inställningar, Integrationer, Värd-SSH** i BombVault och tryck på **Testa anslutning**. Den ska bli grön.

Samma anrop från terminalen listar dina VM:ar:

```sh
docker exec BombVault virsh -c "qemu+ssh://root@192.168.1.10:22/system?keyfile=/config/ssh/id_ed25519&known_hosts=/config/ssh/known_hosts&known_hosts_verify=auto" list --all
```

## Steg 5: Metoden per VM {#per-vm-method}

Varje VM har en **Metod** i kortet **VM** på sin flik Inställningar (öppna VM:en under **Säkerhetskopior**):

- **Mjuk (avstängning)** är standard. Den stänger av VM:en, säkerhetskopierar diskarna och startar den igen, så säkerhetskopian är alltid konsekvent. Den väntar upp till tio minuter på att gästen ska stängas av och tvingar den aldrig. En gäst som fortfarande kör efter det lämnas ifred, och den säkerhetskopian misslyckas med orsaken.
- **Live-ögonblicksbild** säkerhetskopierar en körande VM via en extern ögonblicksbild och `blockcommit`. Den behöver qemu guest agent i VM:en, för en vilande ögonblicksbild, och disken på `/mnt/cache` eller `/mnt/diskX`, inte `/mnt/user`. På en VM som är avstängd faller Live tillbaka på Mjuk av sig själv, och när ögonblicksbilden inte kan göras faller den också tillbaka.

## Endast ändrade block {#changed-blocks}

En VM med qcow2-diskar kan säkerhetskopieras genom att bara läsa det som ändrats. Slå på **Endast ändrade block** i samma kort, så använder BombVault libvirts säkerhetskopierings-API i stället för metoden ovan:

- Den startar ett säkerhetskopieringsjobb i pull-läge (`virsh backup-begin`) som skapar en kontrollpunkt med namnet `bombvault-<time>` i samma ögonblick, och läser diskarna över SSH-anslutningen via en vidarebefordrad Unix-socket. När gästagenten svarar fryses gästens filsystem bara medan jobbet startar. En upptining som misslyckas provas igen i ungefär femton sekunder, och om gästen fortfarande inte tinar skickar BombVault en avisering.
- Den första körningen läser varje allokerat block. Senare körningar läser bara de block som kontrollpunktens bitmap markerar som skrivna sedan den föregående säkerhetskopian.
- Varje disk lagras som en mapp med segment av fast storlek som innehåller det gästen ser. Oförändrade segment tas över från den föregående ögonblicksbilden utan att läsas, så varje ögonblicksbild är fullständig och återställs för sig själv.
- En återställning bygger upp disken från dess segment och konverterar tillbaka den till qcow2 med `qemu-img`. Disken har samma innehåll som vid säkerhetskopieringen, men filen är en ny qcow2-fil, och en stödkedja kommer tillbaka tillplattad.
- Kontrollen före en sådan återställning listar varje disk den bygger upp och räknar den med gästens fulla storlek, två gånger för qcow2, eftersom råavbilden och dess konverterade kopia ligger bredvid varandra tills den gamla filen ersätts. Det är det mesta en disk kan ta, så ett underskott enligt den räkningen lämnar utrymmesraden grå i stället för att blockera. Återställningskontrollen efter den första säkerhetskopian, och **Kontrollera återställning**, ser också till att ögonblicksbilden innehåller varje segment av varje disk i den storlek det ska ha.
- Mellan körningarna har VM:en ingen kontrollpunkt registrerad i libvirt. BombVault behåller definitionen av den nyaste `bombvault-`-kontrollpunkten under `/config/vm-checkpoints` och tar bara bort libvirts uppgift om den (`virsh checkpoint-delete --metadata`). Dess bitmap stannar kvar i qcow2-filen och fortsätter räkna skrivningar, och nästa körning lägger tillbaka kontrollpunkten (`virsh checkpoint-create --redefine`) innan den startar. libvirt vägrar avdefiniera en VM som har kontrollpunkter, så det här är vad som låter Unraids **Remove VM** och en återställning fungera som vanligt.
- Ett säkerhetskopieringsjobb som BombVault lämnade körande, eftersom den stannade mitt i en säkerhetskopiering, avslutas när BombVault startar igen, tillsammans med de kontrollpunkter den körningen lämnade registrerade.
- Att stänga av omkopplaren, ta VM:en ur schemat eller radera dess säkerhetskopior avslutar också ett sådant jobb och raderar bitmappen också medan VM:en körs. På en VM som är av kan libvirt inte radera den, och den stannar oanvänd i avbilden.
- Andra verktygs kontrollpunkter lämnas ifred. En VM som har några kan inte återställas förrän de raderas, och återställningen stannar innan den rör VM:en.

Omkopplaren faller tillbaka av sig själv. En saknad eller trasig kontrollpunkt, en disk som ändrat storlek eller en nyaste ögonblicksbild som inte är en med ändrade block betyder att hela disken läses en gång till. En VM som är av, en rå disk eller zvol, ingen SSH-anslutning till värden, en libvirt utan säkerhetskopierings-API:et eller för lite ledigt utrymme under `/config` betyder den vanliga säkerhetskopian med metoden ovan. Kortet säger vilket av dessa den senaste körningen använde och varför.

Krav: libvirt med säkerhetskopierings-API:et (testat med libvirt 12.2 och QEMU 10.2 på Unraid 7.3), sshd med `AllowStreamLocalForwarding yes` (standard, även på Unraid, som stänger av TCP-vidarebefordran) och qcow2-diskar. libvirt vägrar kontrollpunkter på råa diskar, och Unraid skapar råa vdiskar som standard. VM:en fortsätter köra under säkerhetskopieringen, som med Live, och segmenten som väntar på restic tar upp till 4 GiB under `/config`.

## VM:ar med Unraid-ögonblicksbilder {#unraid-snapshots}

En ögonblicksbild som Unraids VM Manager tar gör en qcow2-disk till en kedja: originalavbilden längst ner, och ovanpå den en fil per ögonblicksbild, var och en med bara det som ändrats sedan den under. VM:en skriver i den nyaste filen. En disk med två ögonblicksbilder är tre filer.

BombVault säkerhetskopierar en sådan VM fullständigt:

- **Varje fil i kedjan.** Den följer varje disks stödkedja till dess slut och säkerhetskopierar varje fil i den, tillsammans med Unraids ögonblicksbildslista (`/etc/libvirt/qemu/snapshotdb/<vm>/snapshots.db`, läst över SSH). En kedja som inte går att följa till slutet, till exempel för att en fil i mitten är borta, får säkerhetskopian att misslyckas och nämner filen.
- **En live-säkerhetskopia behåller ögonblicksbilderna.** **Live-ögonblicksbild** lägger ett tillfälligt överlägg överst medan den läser disken och slår ihop det igen efteråt, bara in i avbilden precis under. Kedjan är densamma efter säkerhetskopian som före den. En VM utan ögonblicksbilder beter sig som den alltid gjort.
- **En återställning för tillbaka kedjan.** En återställning på plats skriver tillbaka varje fil där den låg och lägger tillbaka Unraids ögonblicksbildslista, så VM Manager listar ögonblicksbilderna igen. En återställning bredvid originalet skriver filerna i kopians mapp och pekar de återställda diskarna på de återställda filerna, så kopian aldrig läser från originalets avbilder. Unraids ögonblicksbildslista kommer tillbaka bara med en återställning på plats, eftersom den nämner filerna där de säkerhetskopierades. Utan den startar kopian likadant, och VM Manager listar bara inga ögonblicksbilder för den.
- **Endast ändrade block** läser det gästen ser, så dess återställning kommer tillbaka som en tillplattad qcow2-fil utan ögonblicksbilderna.

### Exempel: en VM med två ögonblicksbilder {#example-unraid-snapshots}

VM:en `homeassistant` har en disk, `vdisk1.qcow2`, och två ögonblicksbilder som du tog i VM Manager före två uppdateringar. Dess disk är nu en kedja av tre filer. Den körs hela tiden, så den använder **Live-ögonblicksbild**.

1. Öppna **Säkerhetskopior**, sedan `homeassistant` och dess flik **Inställningar**. Kontrollera i kortet **VM** att **Metod** är **Live-ögonblicksbild** och att **Endast ändrade block** är av.
2. Tryck på **Säkerhetskopiera nu** i sidans huvud.
3. När körningen är klar är dess rad i **Senaste körningar** på fliken **Verifiering** grön. VM:en fortsatte köra hela tiden, och säkerhetskopian innehåller alla tre filerna på disken och Unraids ögonblicksbildslista.
4. I VM Manager finns de två ögonblicksbilderna kvar, och disken läser fortfarande genom samma tre filer.

En månad senare går en uppdatering fel och du vill ha gårdagens läge tillbaka bredvid det trasiga, för att jämföra:

1. Öppna **Återställning**, välj **VMs** och klicka på `homeassistant`.
2. Behåll den rekommenderade säkerhetskopian från i natt och välj **Bredvid originalet** under **Vart**.
3. Tryck på **Återställ**. De tre filerna hamnar i en mapp som heter som VM:en med `-restored`, och den återställda disken pekar på de återställda filerna. Ingenting ändras på den körande VM:en.

För att i stället rulla tillbaka hela VM:en väljer du **Ersätt originalet** med **Säkerhetskopiera nuvarande läge först** på. BombVault säkerhetskopierar det trasiga läget, stänger av VM:en, skriver tillbaka de tre filerna och Unraids ögonblicksbildslista och startar den igen.

## TrueNAS SCALE {#truenas-scale}

Stegen ovan är skrivna för Unraid, men samma `qemu+ssh://`-länk fungerar mot vilken libvirtd som helst som containern når, TrueNAS SCALE inkluderat, som har VM:ar på libvirt igen sedan 25.04.2. På TrueNAS behöver anslutningssträngen en åsidosättning och nyckeln går till root i TrueNAS egna användarinställningar. Tre saker till ändrar vad en säkerhetskopia innehåller: diskar på zvols, vTPM-tillstånd och namnen TrueNAS ger sina VM:ar.

### 1. Libvirt-socketen {#truenas-socket}

TrueNAS libvirtd lyssnar inte där anslutningssträngen som byggs av fälten **Host SSH** förväntar sig. Sätt `LIBVIRT_URI` i stället, med den extra parametern `?socket=`:

```sh
LIBVIRT_URI=qemu+ssh://<user>@<truenas-host>/system?socket=/run/truenas_libvirt/libvirt-sock
```

BombVault använder `LIBVIRT_URI` exakt som den är skriven, och `LIBVIRT_HOST`, `LIBVIRT_SSH_USER` och `LIBVIRT_SSH_PORT` behövs inte. För var och en av dem som är osatt tar BombVault värd, användare och port från URI:n även för sina egna SSH-kommandon, så NVRAM-kopian och säkerhetskopior av ZFS-datauppsättningar når samma maskin som `virsh`. En variabel du satt själv gäller alltid före, och startloggen nämner målet den tog från URI:n. På TrueNAS behöver en annan användare än root också `zfs allow <user> snapshot,destroy,mount <dataset>` för säkerhetskopior av ZFS-datauppsättningar, se [ZFS-datauppsättningar](zfs-datasets.md#truenas).

Lägg TrueNAS LAN-adress i URI:n. Med varken `LIBVIRT_URI` eller `LIBVIRT_HOST` satt tar BombVault appens standardgateway som värd, eftersom `host.docker.internal` inte går att slå upp i en TrueNAS-app, och startloggen nämner adressen den valde. Det räcker för säkerhetskopior av ZFS-datauppsättningar. VM-säkerhetskopior behöver fortfarande URI:n för socketen.

Appen behöver förmågan `SETPCAP` för VM-säkerhetskopior, vid sidan av `CHOWN`, `DAC_OVERRIDE`, `FOWNER`, `SETGID` och `SETUID`. Utan den stannar libvirt med "cannot apply process capabilities". YAML-koden under [TrueNAS SCALE](getting-started.md#truenas) sätter alla, tillsammans med `SYS_CHROOT` som ögonblicksbild i stället för stopp behöver.

På TrueNAS SCALE 25.10.0 kontrollerades socketsökvägen på riktig hårdvara: standardvärdet `qemu:///system` misslyckas med `Failed to connect socket to '/var/run/libvirt/libvirt-sock': No such file or directory`, och samma anrop med `?socket=/run/truenas_libvirt/libvirt-sock` listar domänerna.

### 2. BombVaults nyckel på TrueNAS {#truenas-key}

Steg 2 ovan skriver till Unraids flashenhet och gäller inte här. På TrueNAS:

1. Slå på SSH-tjänsten under **System, Services**.
2. Redigera **root** under **Credentials, Users** och klistra in BombVaults publika nyckel från **Inställningar, Integrationer, Värd-SSH** som dess publika SSH-nyckel. TrueNAS behåller den över omstarter och uppdateringar.

Nyinstallationer av TrueNAS 24.10 och nyare ger root inget lösenord, och nyckelinloggningen behöver inget. Om du hellre inte vill använda root auktoriserar du nyckeln för ett administratörskonto med sudo och sätter dess namn i stället för `root` i `LIBVIRT_URI`. Säkerhetskopior av ZFS-datauppsättningar behöver då `zfs allow` som beskrivs ovan.

En ZFS-replik till en andra server loggar in med en egen nyckel, som replikdialogen visar. Värd-SSH-nyckeln auktoriserar inte en replik, och replikanyckeln auktoriserar inte VM-säkerhetskopior.

!!! warning "TrueNAS ser inte ändringar som görs med virsh"
    TrueNAS middleware, lagret bakom dess VM-skärmar, är tänkt att vara det enda sättet att ändra dess VM:ar. Ett `virsh`-kommando över den här SSH-länken, vilket är vad BombVaults VM-säkerhetskopiering och återställning kör, går förbi det, och TrueNAS kan senare avstämma eller skriva över tillstånd det inte skapade. Det här kommer från TrueNAS dokumenterade beteende och har inte setts på en riktig låda. Det är samma sorts risk som när Unraids VM Manager avstämmer ändringar gjorda utanför den, bara skarpare på TrueNAS.

### 3. Diskar på zvols {#truenas-zvols}

TrueNAS VM-diskar är ofta zvols, ZFS-blockenheter på `/dev/zvol/<pool>/<dataset>`, inte qcow2-filer. restic kan inte säkerhetskopiera en blockenhet via dess sökväg som det säkerhetskopierar en fil, så BombVault känner igen en zvol-disk i domänens XML (`<source dev="...">` i stället för `<source file="...">`) och hanterar den över samma SSH-länk. BombVault-containern behöver inga ZFS-verktyg.

En säkerhetskopiering av en zvol:

1. `zfs snapshot <dataset>@bombvault-vm-<time>` fixerar en tidpunkt, även medan VM:en körs.
2. `zfs clone -o volmode=dev -o readonly=on` gör den ögonblicksbilden läsbar som en enhet. `volmode=dev` hindrar värden från att skanna gästens partitioner.
3. `dd` läser enheten, och råavbilden går in i restic som en fil, `/vm-disks/<dataset>.img`, taggad `vm:<name>:zvol:<dev>`. restic styckar den efter innehåll, så block som inte ändrats sedan den förra säkerhetskopian lagras inte igen.
4. Klonen och ögonblicksbilden förstörs efteråt, vare sig säkerhetskopian fungerade eller inte. Om BombVault stannade mitt i en säkerhetskopiering tar nästa säkerhetskopiering av den zvolen först bort ögonblicksbilden `bombvault-vm-<time>` och dess klon `<dataset>-bombvault-vm-<time>`. Den lämnar varje annan ögonblicksbild ifred och behåller en som något annat klonats från. Loggen nämner vad den tog bort eller behöll.

BombVault använder inte `zfs send` för detta. Varje post i en send-ström bär ögonblicksbildens GUID och en löpande kontrollsumma, så restic skulle inte hitta något av den förra säkerhetskopian att återanvända och lagra hela disken på nytt varje gång. Uppmätt på TrueNAS SCALE 25.10 med en zvol på 512 MiB som innehåller 200 MiB data: den första säkerhetskopian lade till 211 MB, en andra utan ändringar 52 KB, och en efter att 1 MiB ändrats 3,6 MB. Samma zvol genom `zfs send` lade till 214 MB vid varje körning.

Varje körning skriver också en liten ögonblicksbild taggad `vm:<name>` med domänens XML, när alla diskar är inne. En VM vars diskar alla är zvols har ingen filögonblicksbild, och det är den som listar körningen under VM:en och gör den återställningsbar.

En återställning skriver tillbaka varje zvol på plats, som en fildisk skrivs tillbaka till sin fil, så den återställda VM:en startar den återställda datan:

1. Innan något rörs kontrollerar BombVault att körningen innehåller en avbild av varje zvol-disk. En körning som saknar en nekas och VM:en lämnas som den är.
2. En befintlig zvol får först en ögonblicksbild, `<dataset>@bombvault-before-restore-<time>`, med tiden i UTC som varje ögonblicksbildsnamn BombVault gör, så den kan avvika timmar från den lokala klockan. Loggen nämner den, och `zfs rollback` till den ångrar återställningen så länge den är zvolens nyaste ögonblicksbild. När en nyare finns, till exempel nästa återställnings, gör `zfs rollback -r` det och förstör de nyare ögonblicksbilderna. Den blir kvar tills du tar bort den med `zfs destroy` när den återställda VM:en fungerar.
3. En zvol som inte längre finns skapas i den storlek den hade när den säkerhetskopierades, och en befintlig av en annan storlek sätts tillbaka till den storleken.
4. `dd` skriver avbilden till `/dev/zvol/<dataset>`. Med komprimering på, TrueNAS standard, kommer de tomma delarna av avbilden tillbaka som hål och tar ingen plats.

**En återställning till en annan instans behöver en målpool.** Den flyttar fildiskar till en målmapp, men en mapp nämner ingen ZFS-pool, så utan en nekar BombVault återställningen innan den startar. Ange poolen som parametern `zvolPool` i `POST /api/foreign/restore`: varje zvol behåller sin sökväg under poolen (`tank/vms/win10/disk0` till `flashpool` blir `flashpool/vms/win10/disk0`), och domänens XML pekar på den. Fönstret **Från en gammal säkerhetskopiemapp** på sidan Återställning har inget fält för detta, så det går att nå bara med ett direkt API-anrop. En återställning på samma instans skriver tillbaka varje zvol på sig själv.

**Export som filer utelämnar zvols.** En VM vars diskar alla är zvols nekas med den orsaken, så återställ den från en säkerhetskopia i stället. För en VM med fildiskar också innehåller exporten fildiskarna och en anmärkning som nämner de zvols den lämnade utanför.

Detta prövades på TrueNAS SCALE 25.10 med ZFS 2.3.4: tre säkerhetskopior av en testzvol, en återställning på plats efter att zvolen skrivits över och vuxit, och en återställning till en zvol som raderats. Varje återställning lästes tillbaka med samma SHA-256 som källan, zvolen hade sin gamla storlek igen och ingen ögonblicksbild eller klon blev kvar. En hel VM vars enda disk är en zvol säkerhetskopierades körande, med Mjuk och med Live, och avstängd, och en återställning av hela VM:en (avdefiniera, skriv zvolarna, definiera, starta) kördes där också: zvolen lästes tillbaka med källans kontrollsumma och VM:en startade.

Fildiskar, som Unraid använder, påverkas inte alls av detta.

### 4. vTPM-tillstånd {#truenas-vtpm}

En VM kan ha en virtuell TPM, som Secure Boot och Windows 11-gäster behöver. Dess tillstånd ligger utanför disken och NVRAM, så en säkerhetskopia av bara de två skulle ge tillbaka en gäst med en ny, tom TPM.

- **En genomkopplad TPM**, ett riktigt chip som släpps igenom till gästen, nämner sin sökväg i domänens XML. BombVault sparar dess tillstånd bredvid NVRAM över SSH och skriver tillbaka det före `virsh define`, efter bästa förmåga som NVRAM: en misslyckad läsning eller skrivning loggas och får inte säkerhetskopian eller återställningen att misslyckas.
- **En emulerad vTPM**, den sort TrueNAS sätter upp för Windows 11- och Secure Boot-gäster, publicerar ingen tillståndssökväg i domänens XML. BombVault rapporterar då "no TPM path found" i stället för att gissa en sökväg. Ha en sådan gästs återställningsnyckel, till exempel dess BitLocker-nyckel, till hands innan du återställer den.
- **Du kan inte själv koppla en TPM till en TrueNAS-VM.** TrueNAS SCALE 25.10:s API för VM-enheter accepterar bara `CDROM`, `DISPLAY`, `NIC`, `PCI`, `RAW`, `DISK` och `USB`, och nekar `TPM` som en okänd typ.

TPM-hanteringen är bara enhetstestad och har inte körts mot en riktig TrueNAS-gäst med en vTPM. En VM utan TPM, som på Unraid, säkerhetskopieras och återställs precis som förut.

NVRAM fångas som vanligt. TrueNAS håller det per VM på `/var/db/system/vm/nvram/<id>_<name>_VARS.fd`, och domänens XML nämner den sökvägen.

### 5. VM-namn på TrueNAS {#truenas-names}

På Unraid är en VM:s libvirt-namn det namn du valde. TrueNAS hittar på libvirt-namnet, och har ändrat hur två gånger:

| TrueNAS-version | libvirt-namn | Var ditt namn finns |
|---|---|---|
| 25.10 "Goldeye" | `{id}_{name}`, till exempel `1_debian` | i libvirt-namnet, bakom `{id}_` |
| 26 | VM:ens UUID, till exempel `550e8400-e29b-41d4-a716-446655440000` | bara i domänens XML-`<title>` |

På en värd som BombVault känner igen som TrueNAS, eller som `PLATFORM=truenas` nämner, visar listan Säkerhetskopior ditt namn: `debian` i stället för `1_debian` på 25.10, och `<title>` i stället för UUID:t på 26, eller UUID:t när det inte finns någon titel. Det är bara vad listan visar. Varje `virsh`-anrop och inställningarna per VM använder fortfarande libvirt-namnet, så säkerhetskopiering och återställning verkar alltid på rätt VM. På TrueNAS 26 betyder det att en VM som TrueNAS skapar på nytt med ett nytt UUID dyker upp som en ny post utan sina inställningar, även om dess namn ser likadant ut. På Unraid ändras ingenting, inte heller för en VM du döpt till `10_Windows`.

Upplägget i 25.10 sågs på en riktig TrueNAS SCALE 25.10-låda. Upplägget i TrueNAS 26 kommer från TrueNAS dokumentation och är bara enhetstestat.

## Felsökning {#troubleshooting}

| Symptom | Orsak och åtgärd |
|---|---|
| **Testa anslutning** hänger sig och misslyckas sedan | Värden går inte att nå. Kontrollera steg 3 (nätverk, VLAN, brandvägg) och **Host SSH: Address** och **Port**. |
| `Permission denied (publickey)` fast nyckeln finns i `authorized_keys` | sshd:s `StrictModes` avvisar nyckelfilen eftersom en överordnad mapp har fel ägare eller läge. Det är vanligt när `/root/.ssh` är en länk till den FAT-formaterade flashenheten (`/boot/config/ssh/...`), som är skrivbar för alla. Värdens logg visar `Authentication refused: bad ownership or modes for directory ...`. Lägg till `StrictModes no` i värdens sshd-konfiguration i den globala delen, före varje `Match`-block, och kör sedan `/etc/rc.d/rc.sshd restart`. Unraid skriver `/etc/ssh/sshd_config` på nytt vid varje uppstart, så gör det bestående som beskrivs nedan. |
| `Permission denied (publickey)` och nyckeln verkligen saknas | Upprepa steg 2 och kontrollera att SSH är på. |
| `Host key verification failed` | Kör `docker exec BombVault rm -f /config/ssh/known_hosts` och testa igen. |
| Kontrollen i steg 3 säger `UNREACHABLE` | Containern når inte värdens SSH-port. Se steg 3: värdåtkomst till anpassade nätverk, VLAN-routing eller shim- eller LAN-IP. |
| Variablerna saknas under **Edit** | Importera mallen igen. En befintlig container behåller sin gamla sparade konfiguration. |
| En VM med Unraid-ögonblicksbilder misslyckas och nämner en fil | Diskens stödkedja går inte att följa till slutet. Kontrollera i VM Manager att varje ögonblicksbild av den disken finns kvar, och att den nämnda filen finns under `/mnt`. |

Fler VM-symptom finns i [Felsökning](troubleshooting.md#vm-backup-will-not-connect).

## Så att det består över omstarter {#persistence}

Unraid skriver `/etc/ssh/sshd_config` på nytt vid varje uppstart, så ändringar på värdsidan måste göras om vid uppstart:

- **SSH-porten** sparas i `/boot/config/ident.cfg`, inställd via Unraids webbgränssnitt.
- **`StrictModes no`**, om du behövde det ovan, sparas inte. Tillämpa det vid uppstart från `/boot/config/go`:

  ```sh
  # /boot/config/go: keep SSH key auth working with flash-based authorized_keys
  ( for i in $(seq 1 30); do [ -f /etc/ssh/sshd_config ] && break; sleep 2; done
    grep -q '^StrictModes no' /etc/ssh/sshd_config || \
      sed -i '0,/^Match /s/^Match /StrictModes no\n&/' /etc/ssh/sshd_config
    # SIGHUP reloads sshd's config in place: it keeps the listener + open sessions
    # and avoids the "killing listener process / Restarting SSH server daemon"
    # warning that `rc.sshd restart` prints at boot. Only reload if sshd is already
    # up; if it starts after this runs, it reads the edited config on its own.
    pidof sshd >/dev/null && killall -HUP sshd ) &
  ```

- BombVaults egen SSH-nyckel och `~/.ssh/config` ligger i `/config` (appdata) och skapas på nytt vid varje start, så de överlever omstarter av containern och värden.

## Säkerhet {#security}

Den auktoriserade nyckeln ger root på värden, samma förtroendenivå som Docker-socketen som BombVault redan använder. Håll BombVault på ett betrott nätverk. Nyckeln är ed25519, lagras med läge `0600` i appdata, och värdens nyckel är fäst. Varje värdkommando skickas som separata argument, aldrig genom ett fjärrskal, och SSH-anslutningen använder `BatchMode` och en anslutningstidsgräns.
