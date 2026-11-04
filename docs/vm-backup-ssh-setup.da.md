# VM-sikkerhedskopier

BombVault sikkerhedskopierer KVM/libvirt-VM'er uden at montere nogen libvirt-sti. Den kører `virsh` på værten over SSH (`qemu+ssh://`), læser og skriver VM-diskene gennem Host Data-monteringen (`/mnt` på værten er `/host/user` i containeren) og kopierer UEFI-NVRAM over SSH. Fordi intet, der hører til libvirt, er bind-monteret, kan BombVault aldrig komme i vejen for Unraids VM Manager.

Denne guide dækker opsætningen, sikkerhedskopimetoderne, VM'er med Unraid-snapshots og TrueNAS SCALE. Containersikkerhedskopier har ikke brug for noget af det.

ZFS-datasætsikkerhedskopier bruger det samme SSH-link med den samme nøgle, vært og bruger. Når VM-sikkerhedskopier forbinder, forbinder ZFS-datasætposter også. Se [ZFS-datasæt](zfs-datasets.md).

## Hvad du indstiller {#what-you-configure}

| Indstilling | Hvor | Standard | Betydning |
|---|---|---|---|
| **Host SSH: Address** (`LIBVIRT_HOST`) | skabelonvariabel | `host.docker.internal` | Unraid-værten, nået over SSH |
| **Host SSH: Port** (`LIBVIRT_SSH_PORT`) | skabelonvariabel | `22` | Unraids SSH-port |
| **Host SSH: User** (`LIBVIRT_SSH_USER`) | skabelonvariabel | `root` | SSH-brugeren på værten |
| Offentlig nøgle | **Indstillinger, Integrationer, Værts-SSH** | laves ved første start | Godkend den på værten |

BombVault laver sit SSH-nøglepar ved første start i `/config/ssh/id_ed25519`, gemt i appdata. Værtens nøgle fastgøres i `/config/ssh/known_hosts` ved den første forbindelse.

Skabelonen udfylder **Host SSH: Address** med pladsholderen `192.168.x.x`. BombVault behandler den som ikke sat og prøver `host.docker.internal`, som virker på standardnetværket `bridge`. På et brugerdefineret netværk erstatter du den med værtens LAN-IP (trin 3 B).

## Trin 1: Slå SSH til på Unraid-værten {#step-1}

1. Åbn **Settings, Management Access** i Unraid, og sæt **Use SSH** til **Yes**.
2. Hvis du har ændret SSH-porten, så notér den. **Host SSH: Port** skal passe.

## Trin 2: Godkend BombVaults offentlige nøgle {#step-2}

Unraids `/root` ligger i hukommelsen og tømmes ved hver opstart, så nøglen skal også ligge på flashdrevet. Kør dette i Unraid-terminalen:

```sh
K=$(docker exec BombVault cat /config/ssh/id_ed25519.pub)
mkdir -p /root/.ssh /boot/config/ssh && chmod 700 /root/.ssh
grep -qxF "$K" /root/.ssh/authorized_keys 2>/dev/null  || echo "$K" >> /root/.ssh/authorized_keys
grep -qxF "$K" /boot/config/ssh/root.pubkeys 2>/dev/null || echo "$K" >> /boot/config/ssh/root.pubkeys
chmod 600 /root/.ssh/authorized_keys
```

Unraid 6.9 og nyere kopierer `/boot/config/ssh/root.pubkeys` ind i roots `authorized_keys` ved hver opstart, så dette overlever genstarter. Det gælder for ethvert `root`-login, uanset hvilket værtsnavn eller hvilken port BombVault bruger.

## Trin 3: Netværk {#step-3}

BombVault skal kunne åbne en TCP-forbindelse fra containeren til værtens SSH-port. Vælg det tilfælde, der passer til din opsætning.

### A. BombVault på standardnetværket bridge {#bridge}

- Lad **Host SSH: Address** stå på `host.docker.internal` eller skabelonens pladsholder. Skabelonen tilføjer `--add-host=host.docker.internal:host-gateway`, som peger på docker0-gatewayen, og det er værten.
- Sæt **Host SSH: Port** til din SSH-port.

### B. BombVault på et brugerdefineret netværk {#custom-network}

En container på `br0.x`, macvlan eller ipvlan kan ikke nå værten gennem `host.docker.internal`, fordi 172.17.0.1 er docker0 og ikke kan nås fra `br0.x`. Gør i stedet dette:

1. Sæt **Settings, Docker, Host access to custom networks** til **Enabled** i Unraid.
2. Sæt **Host SSH: Address** til Unraid-værtens LAN-IP, den adresse du åbner webgrænsefladen på.
3. Hvis containerens netværk og værten ligger på forskellige VLAN, så tillad ruten på din router eller firewall: fra container-VLAN'et til værtens IP på SSH-porten, over TCP.
4. Hvis værtens LAN-IP stadig ikke kan nås, så brug værtens shim-IP på containerens subnet. Find den på værten med `ip -4 addr show | grep -B2 <container-subnet>`.

### Tjek, at værten svarer {#verify-reachability}

Kør dette i Unraid-terminalen med din IP og port:

```sh
docker exec BombVault timeout 6 bash -c 'echo > /dev/tcp/192.168.1.10/22' && echo OPEN || echo UNREACHABLE
```

`OPEN` betyder, at vejen virker. `UNREACHABLE` betyder, at netværket i trin 3 skal rettes, før noget andet.

## Trin 4: Sæt variablerne og test {#step-4}

1. Åbn **Docker** i Unraid, klik på BombVault, vælg **Edit**, sæt **Host SSH: Address** (og **Host SSH: Port**, hvis den ikke er 22), og tryk på **Apply**. Mangler variablerne, så importér skabelonen igen, fordi Unraid beholder den gemte konfiguration for en eksisterende container.
2. Åbn **Indstillinger, Integrationer, Værts-SSH** i BombVault, og tryk på **Test forbindelse**. Den skal blive grøn.

Det samme kald fra terminalen viser dine VM'er:

```sh
docker exec BombVault virsh -c "qemu+ssh://root@192.168.1.10:22/system?keyfile=/config/ssh/id_ed25519&known_hosts=/config/ssh/known_hosts&known_hosts_verify=auto" list --all
```

## Trin 5: Metoden pr. VM {#per-vm-method}

Hver VM har en **Metode** på kortet **VM** på sin fane Indstillinger (åbn VM'en under **Sikkerhedskopier**):

- **Pænt (nedlukning)** er standarden. Den lukker VM'en ned, sikkerhedskopierer diskene og starter den igen, så sikkerhedskopien altid er konsistent. Den venter op til ti minutter på, at gæsten lukker ned, og tvinger den aldrig fra. En gæst, der stadig kører efter det, lades i fred, og sikkerhedskopieringen mislykkes med årsagen.
- **Live-snapshot** sikkerhedskopierer en kørende VM gennem et eksternt snapshot og `blockcommit`. Den kræver qemu-gæsteagenten i VM'en, for et stilnet snapshot, og disken på `/mnt/cache` eller `/mnt/diskX`, ikke `/mnt/user`. På en VM, der er slukket, falder Live af sig selv tilbage til Pænt, og det gør den også, når snapshottet ikke kan laves.

## Kun ændrede blokke {#changed-blocks}

En VM med qcow2-diske kan sikkerhedskopieres ved kun at læse det, der er ændret. Slå **Kun ændrede blokke** til på det samme kort, så bruger BombVault libvirts backup-API i stedet for metoden ovenfor:

- Den starter et pull-mode-backupjob (`virsh backup-begin`), der samtidig opretter et checkpoint ved navn `bombvault-<time>`, og læser diskene over SSH-linket gennem en videresendt Unix-socket. Når gæsteagenten svarer, fryses gæstens filsystemer kun, mens jobbet starter. En optøning, der mislykkes, prøves igen i omkring femten sekunder, og hvis gæsten stadig ikke tør op, sender BombVault en notifikation.
- Den første kørsel læser hver allokeret blok. Senere kørsler læser kun de blokke, som checkpointets bitmap markerer som skrevet siden den forrige sikkerhedskopi.
- Hver disk gemmes som en mappe med segmenter af fast størrelse, der rummer det, gæsten ser. Uændrede segmenter overtages fra det forrige gendannelsespunkt uden at blive læst, så hvert gendannelsespunkt er komplet og kan gendannes for sig selv.
- En gendannelse genopbygger disken ud fra dens segmenter og konverterer den tilbage til qcow2 med `qemu-img`. Disken har det samme indhold som på sikkerhedskopitidspunktet, men filen er en ny qcow2-fil, og en backing-kæde kommer tilbage fladtrykt.
- Tjekket før sådan en gendannelse viser hver disk, den genopbygger, og tæller den i gæstens fulde størrelse, to gange for qcow2, da det rå image og dets konverterede kopi ligger side om side, til den gamle fil er erstattet. Det er det mest, en disk kan fylde, så et underskud efter den optælling gør pladslinjen grå i stedet for at blokere. Gendannelsestjekket efter den første sikkerhedskopi og **Tjek gendannelse** sikrer også, at gendannelsespunktet rummer hvert segment af hver disk i den størrelse, det skal have.
- Mellem kørslerne har VM'en intet checkpoint registreret i libvirt. BombVault beholder definitionen af det nyeste `bombvault-`-checkpoint under `/config/vm-checkpoints` og fjerner kun libvirts registrering af det (`virsh checkpoint-delete --metadata`). Dets bitmap bliver i qcow2-filen og bliver ved med at tælle skrivninger, og den næste kørsel lægger checkpointet tilbage (`virsh checkpoint-create --redefine`), før den starter. libvirt nægter at fjerne definitionen af en VM, der har checkpoints, så det er det, der lader Unraids **Remove VM** og en gendannelse virke som sædvanligt.
- Et backupjob, som BombVault lod køre, fordi den stoppede midt i en sikkerhedskopiering, afsluttes, når BombVault starter igen, sammen med de checkpoints, den kørsel efterlod registreret.
- At slå kontakten fra, tage VM'en ud af tidsplanen eller slette dens sikkerhedskopier afslutter også sådan et job og sletter også bitmappet, mens VM'en kører. På en VM, der er slukket, kan libvirt ikke slette det, og det bliver ubrugt i imaget.
- Andre værktøjers checkpoints lades i fred. En VM, der har nogen, kan ikke gendannes, før de er slettet, og gendannelsen stopper, før den rører VM'en.

Kontakten falder tilbage af sig selv. Et manglende eller ødelagt checkpoint, en disk der har ændret størrelse, eller et nyeste gendannelsespunkt, der ikke er et med ændrede blokke, betyder, at hele disken læses en gang til. En VM, der er slukket, en rå disk eller zvol-disk, intet SSH-link til værten, en libvirt uden backup-API'et eller for lidt ledig plads under `/config` betyder den sædvanlige sikkerhedskopiering med metoden ovenfor. Kortet siger, hvilken af dem den sidste kørsel brugte, og hvorfor.

Krav: libvirt med backup-API'et (testet med libvirt 12.2 og QEMU 10.2 på Unraid 7.3), sshd med `AllowStreamLocalForwarding yes` (standarden, også på Unraid, som slår TCP-videresendelse fra) og qcow2-diske. libvirt afviser checkpoints på rå diske, og Unraid laver rå vdisks som standard. VM'en bliver ved med at køre under sikkerhedskopieringen som med Live, og segmenterne, der venter på restic, fylder op til 4 GiB under `/config`.

## VM'er med Unraid-snapshots {#unraid-snapshots}

Et snapshot, som Unraids VM Manager tager, gør en qcow2-disk til en kæde: det oprindelige image nederst og oven på det én fil pr. snapshot, hver med kun det, der er ændret siden den under. VM'en skriver i den nyeste fil. En disk med to snapshots er tre filer.

BombVault sikkerhedskopierer sådan en VM fuldt ud:

- **Hver fil i kæden.** Den følger hver disks backing-kæde til dens ende og sikkerhedskopierer hver fil i den sammen med Unraids snapshotliste (`/etc/libvirt/qemu/snapshotdb/<vm>/snapshots.db`, læst over SSH). En kæde, der ikke kan følges til sin ende, for eksempel fordi en fil midt i er væk, får sikkerhedskopieringen til at mislykkes og nævner filen.
- **En live-sikkerhedskopi beholder snapshottene.** **Live-snapshot** lægger et midlertidigt overlay øverst, mens den læser disken, og fletter det tilbage bagefter, kun ind i det image, der ligger lige under. Kæden er den samme efter sikkerhedskopieringen som før. En VM uden snapshots opfører sig, som den altid har gjort.
- **En gendannelse bringer kæden tilbage.** En gendannelse på stedet skriver hver fil tilbage, hvor den var, og lægger Unraids snapshotliste tilbage, så VM Manager igen viser snapshottene. En gendannelse ved siden af originalen skriver filerne i kopiens mappe og peger de gendannede diske på de gendannede filer, så kopien aldrig læser fra originalens images. Unraids snapshotliste kommer kun tilbage med en gendannelse på stedet, fordi den nævner filerne, hvor de blev sikkerhedskopieret. Uden den starter kopien på samme måde, og VM Manager viser blot ingen snapshots for den.
- **Kun ændrede blokke** læser det, gæsten ser, så dens gendannelse kommer tilbage som én fladtrykt qcow2-fil uden snapshottene.

### Eksempel: en VM med to snapshots {#example-unraid-snapshots}

VM'en `homeassistant` har én disk, `vdisk1.qcow2`, og to snapshots, du tog i VM Manager før to opdateringer. Dens disk er nu en kæde af tre filer. Den kører hele tiden, så den bruger **Live-snapshot**.

1. Åbn **Sikkerhedskopier**, så `homeassistant`, så dens fane **Indstillinger**. Tjek på kortet **VM**, at **Metode** er **Live-snapshot**, og at **Kun ændrede blokke** er slået fra.
2. Tryk på **Sikkerhedskopier nu** i sidens hoved.
3. Når kørslen er færdig, er dens linje under **Seneste kørsler** på fanen **Verificering** grøn. VM'en blev ved med at køre hele tiden, og sikkerhedskopien rummer alle tre filer på disken og Unraids snapshotliste.
4. I VM Manager er de to snapshots der stadig, og disken læser stadig gennem de samme tre filer.

En måned senere går en opdatering galt, og du vil have gårsdagens tilstand tilbage ved siden af den ødelagte for at sammenligne:

1. Åbn **Gendannelse**, vælg **VMs**, og klik på `homeassistant`.
2. Behold den anbefalede sikkerhedskopi fra i nat, og vælg **Ved siden af originalen** under **Hvorhen**.
3. Tryk på **Gendan**. De tre filer havner i en mappe opkaldt efter VM'en med `-restored`, og den gendannede disk peger på de gendannede filer. Intet ændres på den kørende VM.

Vil du i stedet rulle hele VM'en tilbage, så vælg **Erstat originalen** med **Tag først en sikkerhedskopi af den nuværende tilstand** slået til. BombVault sikkerhedskopierer den ødelagte tilstand, lukker VM'en ned, skriver de tre filer og Unraids snapshotliste tilbage og starter den igen.

## TrueNAS SCALE {#truenas-scale}

Trinene ovenfor er skrevet til Unraid, men det samme `qemu+ssh://`-link virker mod enhver libvirtd, containeren kan nå, TrueNAS SCALE inklusive, som har VM'er på libvirt igen siden 25.04.2. På TrueNAS skal forbindelsesstrengen overstyres, og nøglen lægges på root i TrueNAS' egne brugerindstillinger. Tre ting mere ændrer, hvad en sikkerhedskopi rummer: diske på zvols, vTPM-tilstand og de navne, TrueNAS giver sine VM'er.

### 1. libvirt-socketten {#truenas-socket}

TrueNAS' libvirtd lytter ikke dér, hvor forbindelsesstrengen bygget af **Host SSH**-felterne forventer det. Sæt i stedet `LIBVIRT_URI` med den ekstra parameter `?socket=`:

```sh
LIBVIRT_URI=qemu+ssh://<user>@<truenas-host>/system?socket=/run/truenas_libvirt/libvirt-sock
```

BombVault bruger `LIBVIRT_URI` præcis som skrevet, og `LIBVIRT_HOST`, `LIBVIRT_SSH_USER` og `LIBVIRT_SSH_PORT` er ikke nødvendige. For hver af dem, der ikke er sat, tager BombVault vært, bruger og port fra URI'en også til sine egne SSH-kommandoer, så NVRAM-kopien og ZFS-datasætsikkerhedskopier når samme maskine som `virsh`. En variabel, du selv sætter, vinder altid, og opstartsloggen nævner det mål, den tog fra URI'en. På TrueNAS kræver en anden bruger end root også `zfs allow <user> snapshot,destroy,mount <dataset>` til ZFS-datasætsikkerhedskopier, se [ZFS-datasæt](zfs-datasets.md#truenas).

Skriv TrueNAS' LAN-adresse i URI'en. Med hverken `LIBVIRT_URI` eller `LIBVIRT_HOST` sat tager BombVault appens standardgateway som vært, fordi `host.docker.internal` ikke kan slås op i en TrueNAS-app, og opstartsloggen nævner den adresse, den valgte. Det er nok til ZFS-datasætsikkerhedskopier. VM-sikkerhedskopier har stadig brug for URI'en til socketten.

Appen har brug for kapabiliteten `SETPCAP` til VM-sikkerhedskopier ved siden af `CHOWN`, `DAC_OVERRIDE`, `FOWNER`, `SETGID` og `SETUID`. Uden den stopper libvirt med "cannot apply process capabilities". YAML'en under [TrueNAS SCALE](getting-started.md#truenas) sætter dem alle sammen med den `SYS_CHROOT`, som snapshot i stedet for stop kræver.

På TrueNAS SCALE 25.10.0 blev socketstien tjekket på rigtig hardware: Standarden `qemu:///system` fejler med `Failed to connect socket to '/var/run/libvirt/libvirt-sock': No such file or directory`, og det samme kald med `?socket=/run/truenas_libvirt/libvirt-sock` viser domænerne.

### 2. BombVaults nøgle på TrueNAS {#truenas-key}

Trin 2 ovenfor skriver til Unraids flashdrev og gælder ikke her. På TrueNAS:

1. Slå SSH-tjenesten til under **System, Services**.
2. Redigér **root** under **Credentials, Users**, og indsæt BombVaults offentlige nøgle fra **Indstillinger, Integrationer, Værts-SSH** som dens offentlige SSH-nøgle. TrueNAS beholder den på tværs af genstarter og opdateringer.

Friske installationer af TrueNAS 24.10 og nyere giver root ingen adgangskode, og nøglelogin kræver ingen. Vil du helst ikke bruge root, så godkend nøglen for en admin-konto med sudo, og skriv dens navn i stedet for `root` i `LIBVIRT_URI`. ZFS-datasætsikkerhedskopier har så brug for `zfs allow` som beskrevet ovenfor.

En ZFS-replika til en anden server logger ind med en nøgle for sig, som replikadialogen viser. Værts-SSH-nøglen godkender ikke en replika, og replikanøglen godkender ikke VM-sikkerhedskopier.

!!! warning "TrueNAS ser ikke ændringer lavet med virsh"
    TrueNAS' middleware, laget bag dets VM-skærme, er ment som den eneste vej, hvorved dets VM'er ændres. En `virsh`-kommando over dette SSH-link, som er det, BombVaults VM-sikkerhedskopiering og -gendannelse kører, går uden om den, og TrueNAS kan senere afstemme eller overskrive en tilstand, den ikke selv har skabt. Det kommer af TrueNAS' dokumenterede opførsel og er ikke set på en rigtig boks. Det er den samme slags risiko som Unraids VM Manager, der afstemmer ændringer lavet uden for den, bare skarpere på TrueNAS.

### 3. Diske på zvols {#truenas-zvols}

TrueNAS' VM-diske er ofte zvols, ZFS-blokenheder på `/dev/zvol/<pool>/<dataset>`, ikke qcow2-filer. restic kan ikke sikkerhedskopiere en blokenhed via dens sti, som den gør med en fil, så BombVault genkender en zvol-disk i domæne-XML'en (`<source dev="...">` i stedet for `<source file="...">`) og håndterer den over det samme SSH-link. BombVault-containeren har ikke brug for nogen ZFS-værktøjer.

En sikkerhedskopi af en zvol:

1. `zfs snapshot <dataset>@bombvault-vm-<time>` fastlåser et tidspunkt, også mens VM'en kører.
2. `zfs clone -o volmode=dev -o readonly=on` gør det snapshot læsbart som en enhed. `volmode=dev` holder værten fra at scanne gæstens partitioner.
3. `dd` læser enheden, og det rå image går ind i restic som én fil, `/vm-disks/<dataset>.img`, mærket `vm:<name>:zvol:<dev>`. restic skærer den i bidder efter indhold, så blokke, der ikke er ændret siden den sidste sikkerhedskopi, ikke gemmes igen.
4. Klonen og snapshottet destrueres bagefter, uanset om sikkerhedskopieringen virkede. Hvis BombVault stoppede midt i en sikkerhedskopiering, fjerner den næste sikkerhedskopiering af den zvol først snapshottet `bombvault-vm-<time>` og dets klon `<dataset>-bombvault-vm-<time>`. Den lader alle andre snapshots være og beholder et, som noget andet er klonet fra. Loggen nævner, hvad den fjernede eller beholdt.

BombVault bruger ikke `zfs send` til dette. Hver post i en send-strøm bærer snapshottets GUID og en løbende kontrolsum, så restic ville ikke finde noget fra den sidste sikkerhedskopi at genbruge og ville gemme hele disken igen hver gang. Målt på TrueNAS SCALE 25.10 med en zvol på 512 MiB med 200 MiB data: Den første sikkerhedskopi tilføjede 211 MB, en anden uden ændringer 52 KB, og en efter at 1 MiB var ændret 3,6 MB. Den samme zvol gennem `zfs send` tilføjede 214 MB ved hver kørsel.

Hver kørsel skriver desuden et lille gendannelsespunkt mærket `vm:<name>` med domæne-XML'en, når alle diske er inde. En VM, hvis diske alle er zvols, har intet filgendannelsespunkt, og det er dette, der viser kørslen under VM'en og gør den gendannelig.

En gendannelse skriver hver zvol tilbage på stedet, som en fildisk skrives tilbage til sin fil, så den gendannede VM starter med de gendannede data:

1. Før noget røres, tjekker BombVault, at kørslen rummer et image af hver zvol-disk. En kørsel, der mangler et, afvises, og VM'en lades, som den er.
2. En eksisterende zvol får først et snapshot, `<dataset>@bombvault-before-restore-<time>`, med tidspunktet i UTC som hvert snapshotnavn, BombVault laver, så det kan afvige med timer fra det lokale ur. Loggen nævner det, og `zfs rollback` til det fortryder gendannelsen, så længe det er zvolens nyeste snapshot. Når der findes et nyere, for eksempel næste gendannelses, gør `zfs rollback -r` det og destruerer de nyere snapshots. Det bliver liggende, til du fjerner det med `zfs destroy`, når den gendannede VM virker.
3. En zvol, der ikke længere findes, oprettes i den størrelse, den havde, da den blev sikkerhedskopieret, og en eksisterende af en anden størrelse sættes tilbage til den størrelse.
4. `dd` skriver imaget på `/dev/zvol/<dataset>`. Med komprimering slået til, TrueNAS' standard, kommer de tomme dele af imaget tilbage som huller og fylder intet.

**En gendannelse til en anden instans har brug for en destinationspulje.** Den flytter fildiske til en destinationsmappe, men en mappe nævner ingen ZFS-pulje, så uden en afviser BombVault gendannelsen, før den starter. Giv puljen som parameteren `zvolPool` til `POST /api/foreign/restore`: Hver zvol beholder sin sti under puljen (`tank/vms/win10/disk0` til `flashpool` bliver til `flashpool/vms/win10/disk0`), og domæne-XML'en peger på den. Vinduet **Fra en gammel sikkerhedskopimappe** på siden Gendannelse har intet felt til dette, så det kan kun nås gennem et direkte API-kald. En gendannelse på samme instans skriver hver zvol tilbage på sig selv.

**Eksport som filer udelader zvols.** En VM, hvis diske alle er zvols, afvises med den begrundelse, så gendan den fra en sikkerhedskopi i stedet. For en VM med også fildiske rummer eksporten fildiskene og en note, der nævner de zvols, den udelod.

Dette blev prøvet på TrueNAS SCALE 25.10 med ZFS 2.3.4: tre sikkerhedskopier af en zvol til prøve, en gendannelse på stedet, efter at zvollen var overskrevet og udvidet, og en gendannelse til en zvol, der var slettet. Hver gendannelse blev læst tilbage med samme SHA-256 som kilden, zvollen havde sin gamle størrelse igen, og intet snapshot eller klon blev efterladt. En hel VM, hvis eneste disk er en zvol, blev sikkerhedskopieret kørende, med Pænt og med Live, og slukket, og en gendannelse af en hel VM (fjern definitionen, skriv zvols, definér, start) kørte dér også: Zvollen blev læst tilbage med kildens kontrolsum, og VM'en startede.

Fildiske, som Unraid bruger, berøres ikke af noget af dette.

### 4. vTPM-tilstand {#truenas-vtpm}

En VM kan have en virtuel TPM, som Secure Boot og Windows 11-gæster har brug for. Dens tilstand ligger uden for disken og NVRAM, så en sikkerhedskopi af kun de to ville bringe en gæst tilbage med en ny, tom TPM.

- **En gennemstillet TPM**, en rigtig chip sendt videre til gæsten, nævner sin sti i domæne-XML'en. BombVault gemmer dens tilstand ved siden af NVRAM over SSH og skriver den tilbage før `virsh define`, efter bedste evne som NVRAM: En læsning eller skrivning, der mislykkes, logges og får ikke sikkerhedskopieringen eller gendannelsen til at mislykkes.
- **En emuleret vTPM**, den slags TrueNAS sætter op til Windows 11- og Secure Boot-gæster, udgiver ingen tilstandssti i domæne-XML'en. BombVault melder så "no TPM path found" i stedet for at gætte en sti. Hav sådan en gæsts gendannelsesnøgle, for eksempel dens BitLocker-nøgle, ved hånden, før du gendanner den.
- **Du kan ikke selv sætte en TPM på en TrueNAS-VM.** TrueNAS SCALE 25.10's VM-enheds-API accepterer kun `CDROM`, `DISPLAY`, `NIC`, `PCI`, `RAW`, `DISK` og `USB` og afviser `TPM` som en ukendt type.

TPM-håndteringen er kun enhedstestet og har ikke kørt mod en rigtig TrueNAS-gæst med en vTPM. En VM uden TPM, som på Unraid, sikkerhedskopieres og gendannes præcis som før.

NVRAM fanges som sædvanligt. TrueNAS gemmer den pr. VM i `/var/db/system/vm/nvram/<id>_<name>_VARS.fd`, og domæne-XML'en nævner den sti.

### 5. VM-navne på TrueNAS {#truenas-names}

På Unraid er en VMs libvirt-navn det navn, du valgte. TrueNAS finder selv på libvirt-navnet og har ændret, hvordan, to gange:

| TrueNAS-version | libvirt-navn | Hvor dit navn står |
|---|---|---|
| 25.10 "Goldeye" | `{id}_{name}`, for eksempel `1_debian` | i libvirt-navnet, bag `{id}_` |
| 26 | VM'ens UUID, for eksempel `550e8400-e29b-41d4-a716-446655440000` | kun i domæne-XML'ens `<title>` |

På en vært, som BombVault registrerer som TrueNAS, eller som `PLATFORM=truenas` nævner, viser listen Sikkerhedskopier dit navn: `debian` i stedet for `1_debian` på 25.10 og `<title>` i stedet for UUID'et på 26, eller UUID'et, når der ingen titel er. Det er kun det, listen viser. Hvert `virsh`-kald og indstillingerne pr. VM bruger stadig libvirt-navnet, så sikkerhedskopiering og gendannelse altid virker på den rigtige VM. På TrueNAS 26 betyder det, at en VM, som TrueNAS opretter igen med et nyt UUID, dukker op som en ny post uden sine indstillinger, selv om dens navn ser ens ud. På Unraid ændres intet, også for en VM, du har kaldt `10_Windows`.

25.10-skemaet blev set på en rigtig TrueNAS SCALE 25.10-boks. TrueNAS 26-skemaet kommer fra TrueNAS' dokumentation og er kun enhedstestet.

## Fejlfinding {#troubleshooting}

| Symptom | Årsag og løsning |
|---|---|
| **Test forbindelse** hænger og fejler så | Værten kan ikke nås. Tjek trin 3 (netværk, VLAN, firewall) og **Host SSH: Address** og **Port**. |
| `Permission denied (publickey)`, selv om nøglen står i `authorized_keys` | sshds `StrictModes` afviser nøglefilen, fordi en overordnet mappe har den forkerte ejer eller tilstand. Det er almindeligt, når `/root/.ssh` er et link til det FAT-formaterede flashdrev (`/boot/config/ssh/...`), som kan skrives af alle. Værtens log viser `Authentication refused: bad ownership or modes for directory ...`. Tilføj `StrictModes no` til værtens sshd-konfiguration i det globale afsnit, før en `Match`-blok, og kør så `/etc/rc.d/rc.sshd restart`. Unraid skriver `/etc/ssh/sshd_config` på ny ved hver opstart, så gør det varigt som beskrevet nedenfor. |
| `Permission denied (publickey)`, og nøglen mangler virkelig | Gentag trin 2, og tjek, at SSH er slået til. |
| `Host key verification failed` | Kør `docker exec BombVault rm -f /config/ssh/known_hosts`, og test igen. |
| Tjekket i trin 3 siger `UNREACHABLE` | Containeren kan ikke nå værtens SSH-port. Se trin 3: værtsadgang til brugerdefinerede netværk, VLAN-routing eller shim- eller LAN-IP'en. |
| Variablerne mangler under **Edit** | Importér skabelonen igen. En eksisterende container beholder sin gamle gemte konfiguration. |
| En VM med Unraid-snapshots mislykkes og nævner en fil | Diskens backing-kæde kan ikke følges til sin ende. Tjek i VM Manager, at hvert snapshot af den disk stadig er der, og at den nævnte fil findes under `/mnt`. |

Flere VM-symptomer står under [Fejlfinding](troubleshooting.md#vm-backup-will-not-connect).

## Hold det oppe på tværs af genstarter {#persistence}

Unraid skriver `/etc/ssh/sshd_config` på ny ved hver opstart, så ændringer på værtssiden skal laves igen ved opstart:

- **SSH-porten** gemmes i `/boot/config/ident.cfg`, sat gennem Unraids webgrænseflade.
- **`StrictModes no`**, hvis du havde brug for det ovenfor, bevares ikke. Anvend det ved opstart fra `/boot/config/go`:

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

- BombVaults egen SSH-nøgle og `~/.ssh/config` ligger i `/config` (appdata) og laves på ny ved hver start, så de overlever genstarter af containeren og værten.

## Sikkerhed {#security}

Den godkendte nøgle giver root på værten, det samme tillidsniveau som Docker-socketten, BombVault allerede bruger. Hold BombVault på et pålideligt netværk. Nøglen er ed25519, gemt med tilstanden `0600` i appdata, og værtens nøgle er fastgjort. Hver værtskommando sendes som separate argumenter, aldrig gennem et fjernskal, og SSH-forbindelsen bruger `BatchMode` og en forbindelsestimeout.
