# VM-sikkerhetskopier

BombVault tar sikkerhetskopi av KVM/libvirt-VM-er uten å montere noen libvirt-sti. Den kjører `virsh` på verten over SSH (`qemu+ssh://`), leser og skriver VM-diskene gjennom Host Data-monteringen (`/mnt` på verten er `/host/user` i containeren) og kopierer UEFI-NVRAM over SSH. Fordi ingenting som hører til libvirt blir bind-montert, kan BombVault aldri komme i veien for Unraid VM Manager.

Denne veiledningen dekker oppsettet, sikkerhetskopimetodene, VM-er med Unraid-øyeblikksbilder og TrueNAS SCALE. Containersikkerhetskopier trenger ingenting av dette.

Sikkerhetskopier av ZFS-datasett bruker den samme SSH-forbindelsen, med samme nøkkel, vert og bruker. Så snart VM-sikkerhetskopiene kobler til, kobler ZFS-datasettelementene til også. Se [ZFS-datasett](zfs-datasets.md).

## Hva du setter opp {#what-you-configure}

| Innstilling | Hvor | Standard | Betydning |
|---|---|---|---|
| **Host SSH: Address** (`LIBVIRT_HOST`) | malvariabel | `host.docker.internal` | Unraid-verten, nådd over SSH |
| **Host SSH: Port** (`LIBVIRT_SSH_PORT`) | malvariabel | `22` | Unraids SSH-port |
| **Host SSH: User** (`LIBVIRT_SSH_USER`) | malvariabel | `root` | SSH-brukeren på verten |
| Offentlig nøkkel | **Innstillinger, Integrasjoner, Verts-SSH** | lages ved første oppstart | Autoriser den på verten |

BombVault lager SSH-nøkkelparet sitt ved første oppstart i `/config/ssh/id_ed25519`, lagret i appdata. Vertens nøkkel festes i `/config/ssh/known_hosts` ved den første tilkoblingen.

Malen fyller **Host SSH: Address** med plassholderen `192.168.x.x`. BombVault regner det som ikke satt og prøver `host.docker.internal`, som virker på standardnettverket `bridge`. På et egendefinert nettverk bytter du den ut med vertens LAN-IP (trinn 3 B).

## Trinn 1: Slå på SSH på Unraid-verten {#step-1}

1. Åpne **Settings, Management Access** i Unraid og sett **Use SSH** til **Yes**.
2. Hvis du endret SSH-porten, noter den. **Host SSH: Port** må stemme.

## Trinn 2: Autoriser BombVaults offentlige nøkkel {#step-2}

Unraids `/root` ligger i minnet og tømmes ved hver oppstart, så nøkkelen må ligge på flash-disken også. Kjør dette i Unraid-terminalen:

```sh
K=$(docker exec BombVault cat /config/ssh/id_ed25519.pub)
mkdir -p /root/.ssh /boot/config/ssh && chmod 700 /root/.ssh
grep -qxF "$K" /root/.ssh/authorized_keys 2>/dev/null  || echo "$K" >> /root/.ssh/authorized_keys
grep -qxF "$K" /boot/config/ssh/root.pubkeys 2>/dev/null || echo "$K" >> /boot/config/ssh/root.pubkeys
chmod 600 /root/.ssh/authorized_keys
```

Unraid 6.9 og nyere kopierer `/boot/config/ssh/root.pubkeys` inn i roots `authorized_keys` ved hver oppstart, så dette overlever omstarter. Det gjelder for hver `root`-innlogging, uansett hvilket vertsnavn eller hvilken port BombVault bruker.

## Trinn 3: Nettverk {#step-3}

BombVault må åpne en TCP-forbindelse fra containeren til vertens SSH-port. Velg tilfellet som passer til oppsettet ditt.

### A. BombVault på standard bridge-nettverk {#bridge}

- La **Host SSH: Address** stå på `host.docker.internal` eller malens plassholder. Malen legger til `--add-host=host.docker.internal:host-gateway`, som peker på docker0-gatewayen, og den er verten.
- Sett **Host SSH: Port** til SSH-porten din.

### B. BombVault på et egendefinert nettverk {#custom-network}

En container på `br0.x`, macvlan eller ipvlan kan ikke nå verten via `host.docker.internal`, fordi 172.17.0.1 er docker0 og ikke kan nås fra `br0.x`. Gjør i stedet dette:

1. Sett **Settings, Docker, Host access to custom networks** til **Enabled** i Unraid.
2. Sett **Host SSH: Address** til Unraid-vertens LAN-IP, adressen du åpner webgrensesnittet på.
3. Hvis containerens nettverk og verten ligger på ulike VLAN, tillat ruten på ruteren eller brannmuren: fra container-VLAN-et til vertens IP på SSH-porten, over TCP.
4. Hvis vertens LAN-IP fortsatt ikke kan nås, bruk vertens shim-IP på containerens subnett. Finn den på verten med `ip -4 addr show | grep -B2 <container-subnet>`.

### Sjekk at verten svarer {#verify-reachability}

Kjør dette i Unraid-terminalen, med din IP og port:

```sh
docker exec BombVault timeout 6 bash -c 'echo > /dev/tcp/192.168.1.10/22' && echo OPEN || echo UNREACHABLE
```

`OPEN` betyr at veien virker. `UNREACHABLE` betyr at nettverket i trinn 3 må rettes før noe annet.

## Trinn 4: Sett variablene og test {#step-4}

1. Åpne **Docker** i Unraid, klikk på BombVault, velg **Edit**, sett **Host SSH: Address** (og **Host SSH: Port** hvis den ikke er 22) og trykk på **Apply**. Hvis variablene mangler, importer malen på nytt, fordi Unraid beholder den lagrede konfigurasjonen til en eksisterende container.
2. Åpne **Innstillinger, Integrasjoner, Verts-SSH** i BombVault og trykk på **Test tilkobling**. Den skal bli grønn.

Det samme kallet fra terminalen lister VM-ene dine:

```sh
docker exec BombVault virsh -c "qemu+ssh://root@192.168.1.10:22/system?keyfile=/config/ssh/id_ed25519&known_hosts=/config/ssh/known_hosts&known_hosts_verify=auto" list --all
```

## Trinn 5: Metoden per VM {#per-vm-method}

Hver VM har en **Metode** i kortet **VM** på fanen Innstillinger (åpne VM-en under **Sikkerhetskopier**):

- **Pent (nedstenging)** er standarden. Den slår VM-en av, sikkerhetskopierer diskene og starter den igjen, så sikkerhetskopien alltid er konsistent. Den venter opptil ti minutter på at gjesten slår seg av, og tvinger den aldri av. En gjest som fortsatt kjører etter det, får være i fred, og den sikkerhetskopien feiler med årsaken.
- **Live-snapshot** tar sikkerhetskopi av en kjørende VM gjennom et eksternt øyeblikksbilde og `blockcommit`. Den trenger qemu guest agent i VM-en, for et stilnet øyeblikksbilde, og disken på `/mnt/cache` eller `/mnt/diskX`, ikke `/mnt/user`. På en VM som er avslått, faller Live tilbake til Pent av seg selv, og det samme skjer når øyeblikksbildet ikke kan lages.

## Bare endrede blokker {#changed-blocks}

En VM med qcow2-disker kan sikkerhetskopieres ved å lese bare det som er endret. Slå på **Bare endrede blokker** i samme kort, så bruker BombVault libvirts backup-API i stedet for metoden over:

- Den starter en backup-jobb i pull-modus (`virsh backup-begin`) som lager et sjekkpunkt med navnet `bombvault-<time>` i samme øyeblikk, og leser diskene over SSH-forbindelsen gjennom en videresendt Unix-socket. Når gjesteagenten svarer, fryses filsystemene til gjesten bare mens jobben starter. Et opptining som feiler, prøves på nytt i omtrent femten sekunder, og hvis gjesten fortsatt ikke tør opp, sender BombVault et varsel.
- Den første kjøringen leser hver allokerte blokk. Senere kjøringer leser bare blokkene sjekkpunktets bitmap markerer som skrevet siden forrige sikkerhetskopi.
- Hver disk lagres som en mappe med segmenter av fast størrelse som har det gjesten ser. Uendrede segmenter overtas fra forrige øyeblikksbilde uten å bli lest, så hvert øyeblikksbilde er komplett og gjenopprettes for seg selv.
- En gjenoppretting bygger disken opp igjen fra segmentene og konverterer den tilbake til qcow2 med `qemu-img`. Disken har det samme innholdet som da sikkerhetskopien ble tatt, men filen er en ny qcow2-fil, og en kjede av underliggende filer kommer tilbake flatet ut.
- Sjekken før en slik gjenoppretting lister hver disk den bygger opp igjen og regner den med gjestens fulle størrelse, to ganger for qcow2, siden det rå bildet og den konverterte kopien ligger side om side til den gamle filen er erstattet. Det er det meste en disk kan ta, så et underskudd etter denne tellingen gjør plasslinjen grå i stedet for å blokkere. Gjenopprettingskontrollen etter den første sikkerhetskopien, og **Sjekk gjenoppretting**, passer også på at øyeblikksbildet har hvert segment av hver disk i størrelsen det skal ha.
- Mellom kjøringene har VM-en ikke noe sjekkpunkt registrert i libvirt. BombVault beholder definisjonen av det nyeste `bombvault-`-sjekkpunktet under `/config/vm-checkpoints` og fjerner bare libvirts registrering av det (`virsh checkpoint-delete --metadata`). Bitmapet blir i qcow2-filen og fortsetter å telle skrivinger, og neste kjøring setter sjekkpunktet tilbake (`virsh checkpoint-create --redefine`) før den starter. libvirt nekter å fjerne definisjonen av en VM som har sjekkpunkter, så dette er det som lar Unraids **Remove VM** og en gjenoppretting virke som vanlig.
- En backup-jobb som BombVault lot stå igjen fordi den stoppet midt i en sikkerhetskopi, avsluttes når BombVault starter igjen, sammen med sjekkpunktene den kjøringen lot stå registrert.
- Å slå bryteren av, ta VM-en ut av tidsplanen eller slette sikkerhetskopiene dens avslutter også en slik jobb, og sletter bitmapet også mens VM-en kjører. På en VM som er av, kan libvirt ikke slette det, og det blir værende ubrukt i bildet.
- Sjekkpunkter fra andre verktøy lar BombVault være i fred. En VM som har noen, kan ikke gjenopprettes før de er slettet, og gjenopprettingen stopper før den rører VM-en.

Bryteren faller tilbake av seg selv. Et sjekkpunkt som mangler eller er ødelagt, en disk som endret størrelse eller et nyeste øyeblikksbilde som ikke er et med endrede blokker, betyr at hele disken leses én gang til. En VM som er av, en rå disk eller zvol, ingen SSH-forbindelse til verten, en libvirt uten backup-API-et eller for lite ledig plass under `/config` betyr den vanlige sikkerhetskopien med metoden over. Kortet sier hvilken av disse den siste kjøringen brukte og hvorfor.

Krav: libvirt med backup-API-et (testet med libvirt 12.2 og QEMU 10.2 på Unraid 7.3), sshd med `AllowStreamLocalForwarding yes` (standard, også på Unraid, som slår av TCP-videresending) og qcow2-disker. libvirt nekter sjekkpunkter på rå disker, og Unraid lager rå vdisker som standard. VM-en fortsetter å kjøre under sikkerhetskopien, som med Live, og segmentene som venter på restic tar opptil 4 GiB under `/config`.

## VM-er med Unraid-øyeblikksbilder {#unraid-snapshots}

Et øyeblikksbilde som Unraid VM Manager tar, gjør en qcow2-disk til en kjede: det opprinnelige bildet nederst, og oppå det én fil per øyeblikksbilde, hver med bare det som er endret siden det under. VM-en skriver til den nyeste filen. En disk med to øyeblikksbilder er tre filer.

BombVault tar sikkerhetskopi av en slik VM i sin helhet:

- **Hver fil i kjeden.** Den følger hver disks kjede av underliggende filer til slutten og tar sikkerhetskopi av hver fil i den, sammen med Unraids liste over øyeblikksbilder (`/etc/libvirt/qemu/snapshotdb/<vm>/snapshots.db`, lest over SSH). En kjede som ikke kan følges til slutten, for eksempel fordi en fil i midten er borte, får sikkerhetskopien til å feile og nevner filen.
- **En live-sikkerhetskopi beholder øyeblikksbildene.** **Live-snapshot** legger et midlertidig overlegg øverst mens den leser disken, og slår det sammen igjen etterpå, bare inn i bildet rett under. Kjeden er den samme etter sikkerhetskopien som før den. En VM uten øyeblikksbilder oppfører seg som den alltid har gjort.
- **En gjenoppretting henter kjeden tilbake.** En gjenoppretting på stedet skriver hver fil tilbake dit den lå og setter Unraids liste over øyeblikksbilder tilbake, så VM Manager lister øyeblikksbildene igjen. En gjenoppretting ved siden av originalen skriver filene til mappen til kopien og peker de gjenopprettede diskene på de gjenopprettede filene, så kopien aldri leser fra bildene til originalen. Unraids liste over øyeblikksbilder kommer bare tilbake ved en gjenoppretting på stedet, fordi den nevner filene der de lå da sikkerhetskopien ble tatt. Uten den starter kopien likevel, og VM Manager lister bare ingen øyeblikksbilder for den.
- **Bare endrede blokker** leser det gjesten ser, så gjenopprettingen kommer tilbake som én flatet ut qcow2-fil uten øyeblikksbildene.

### Eksempel: en VM med to øyeblikksbilder {#example-unraid-snapshots}

VM-en `homeassistant` har én disk, `vdisk1.qcow2`, og to øyeblikksbilder du tok i VM Manager før to oppdateringer. Disken er nå en kjede på tre filer. Den kjører hele tiden, så den bruker **Live-snapshot**.

1. Åpne **Sikkerhetskopier**, deretter `homeassistant`, så fanen **Innstillinger**. Sjekk i kortet **VM** at **Metode** er **Live-snapshot** og at **Bare endrede blokker** er av.
2. Trykk på **Sikkerhetskopier nå** øverst på siden.
3. Når kjøringen er ferdig, er linjen under **Siste kjøringer** på fanen **Verifisering** grønn. VM-en fortsatte å kjøre hele tiden, og sikkerhetskopien har alle tre filene til disken og Unraids liste over øyeblikksbilder.
4. I VM Manager er de to øyeblikksbildene fortsatt der, og disken leser fortsatt gjennom de samme tre filene.

En måned senere går en oppdatering galt, og du vil ha gårsdagens tilstand tilbake ved siden av den ødelagte, for å sammenligne:

1. Åpne **Gjenoppretting**, velg **VM-er** og klikk på `homeassistant`.
2. Behold den anbefalte sikkerhetskopien fra i natt og velg **Ved siden av originalen** under **Hvor til**.
3. Trykk på **Gjenopprett**. De tre filene havner i en mappe oppkalt etter VM-en med `-restored`, og den gjenopprettede disken peker på de gjenopprettede filene. Ingenting endres på den kjørende VM-en.

For å rulle hele VM-en tilbake i stedet velger du **Erstatt originalen** med **Ta sikkerhetskopi av dagens tilstand først** på. BombVault tar sikkerhetskopi av den ødelagte tilstanden, slår VM-en av, skriver de tre filene og Unraids liste over øyeblikksbilder tilbake, og starter den igjen.

## TrueNAS SCALE {#truenas-scale}

Trinnene over er skrevet for Unraid, men den samme `qemu+ssh://`-forbindelsen virker mot hvilken som helst libvirtd containeren kan nå, TrueNAS SCALE inkludert, som har VM-er på libvirt igjen siden 25.04.2. På TrueNAS trenger tilkoblingsstrengen en overstyring, og nøkkelen legges til root i TrueNAS sine egne brukerinnstillinger. Tre ting til endrer hva en sikkerhetskopi inneholder: disker på zvols, vTPM-tilstand og navnene TrueNAS gir VM-ene sine.

### 1. libvirt-socketen {#truenas-socket}

TrueNAS sin libvirtd lytter ikke der tilkoblingsstrengen som bygges fra feltene **Host SSH** forventer. Sett `LIBVIRT_URI` i stedet, med den ekstra parameteren `?socket=`:

```sh
LIBVIRT_URI=qemu+ssh://<user>@<truenas-host>/system?socket=/run/truenas_libvirt/libvirt-sock
```

BombVault bruker `LIBVIRT_URI` akkurat slik den er skrevet, og `LIBVIRT_HOST`, `LIBVIRT_SSH_USER` og `LIBVIRT_SSH_PORT` trengs ikke. For hver av dem som ikke er satt, tar BombVault vert, bruker og port fra URI-en også for sine egne SSH-kommandoer, så NVRAM-kopieringen og sikkerhetskopier av ZFS-datasett når den samme maskinen som `virsh`. En variabel du setter selv, vinner alltid, og oppstartsloggen nevner målet den tok fra URI-en. På TrueNAS trenger en annen bruker enn root også `zfs allow <user> snapshot,destroy,mount <dataset>` for sikkerhetskopier av ZFS-datasett, se [ZFS-datasett](zfs-datasets.md#truenas).

Legg TrueNAS LAN-adressen inn i URI-en. Med verken `LIBVIRT_URI` eller `LIBVIRT_HOST` satt, tar BombVault appens standard-gateway som vert, fordi `host.docker.internal` ikke kan slås opp i en TrueNAS-app, og oppstartsloggen nevner adressen den valgte. Det er nok for sikkerhetskopier av ZFS-datasett. VM-sikkerhetskopier trenger fortsatt URI-en for socketen.

Appen trenger capabilityen `SETPCAP` for VM-sikkerhetskopier, ved siden av `CHOWN`, `DAC_OVERRIDE`, `FOWNER`, `SETGID` og `SETUID`. Uten den stopper libvirt med «cannot apply process capabilities». YAML-en under [TrueNAS SCALE](getting-started.md#truenas) setter alle sammen, sammen med `SYS_CHROOT`, som øyeblikksbilde i stedet for stopp trenger.

På TrueNAS SCALE 25.10.0 ble socket-stien sjekket på ekte maskinvare: standardverdien `qemu:///system` feiler med `Failed to connect socket to '/var/run/libvirt/libvirt-sock': No such file or directory`, og det samme kallet med `?socket=/run/truenas_libvirt/libvirt-sock` lister domenene.

### 2. BombVaults nøkkel på TrueNAS {#truenas-key}

Trinn 2 over skriver til Unraids flash-disk og gjelder ikke her. På TrueNAS:

1. Slå på SSH-tjenesten under **System, Services**.
2. Under **Credentials, Users** redigerer du **root** og limer inn BombVaults offentlige nøkkel fra **Innstillinger, Integrasjoner, Verts-SSH** som SSH-nøkkelen dens. TrueNAS beholder den gjennom omstarter og oppdateringer.

Ferske installasjoner av TrueNAS 24.10 og nyere gir root ikke noe passord, og nøkkelinnloggingen trenger ikke noe. Hvis du heller ikke vil bruke root, autoriserer du nøkkelen for en administratorkonto med sudo og setter navnet dens i stedet for `root` i `LIBVIRT_URI`. Sikkerhetskopier av ZFS-datasett trenger da `zfs allow` som beskrevet over.

En ZFS-replika til en andre server logger inn med en egen nøkkel, som replikadialogen viser. Verts-SSH-nøkkelen autoriserer ikke en replika, og replikanøkkelen autoriserer ikke VM-sikkerhetskopier.

!!! warning "TrueNAS ser ikke endringer gjort med virsh"
    TrueNAS sin middleware, laget bak VM-skjermene, er ment å være den eneste veien VM-ene endres på. En `virsh`-kommando over denne SSH-forbindelsen, som er det BombVaults VM-sikkerhetskopi og -gjenoppretting kjører, går utenom den, og TrueNAS kan senere avstemme eller overskrive tilstand det ikke selv har laget. Dette bygger på TrueNAS sin dokumenterte oppførsel og er ikke sett på en ekte boks. Det er samme slags risiko som at Unraids VM Manager avstemmer endringer gjort utenfor den, bare skarpere på TrueNAS.

### 3. Disker på zvols {#truenas-zvols}

TrueNAS-VM-disker er ofte zvols, ZFS-blokkenheter på `/dev/zvol/<pool>/<dataset>`, ikke qcow2-filer. restic kan ikke ta sikkerhetskopi av en blokkenhet via stien slik den gjør med en fil, så BombVault gjenkjenner en zvol-disk i domene-XML-en (`<source dev="...">` i stedet for `<source file="...">`) og håndterer den over samme SSH-forbindelse. BombVault-containeren trenger ingen ZFS-verktøy.

En sikkerhetskopi av en zvol:

1. `zfs snapshot <dataset>@bombvault-vm-<time>` fester et tidspunkt, også mens VM-en kjører.
2. `zfs clone -o volmode=dev -o readonly=on` gjør det øyeblikksbildet lesbart som en enhet. `volmode=dev` hindrer verten i å skanne partisjonene til gjesten.
3. `dd` leser enheten, og det rå bildet går inn i restic som én fil, `/vm-disks/<dataset>.img`, merket `vm:<name>:zvol:<dev>`. restic deler det i biter etter innhold, så blokker som ikke er endret siden forrige sikkerhetskopi, lagres ikke på nytt.
4. Klonen og øyeblikksbildet ødelegges etterpå, enten sikkerhetskopien virket eller ikke. Hvis BombVault stoppet midt i en sikkerhetskopi, fjerner neste sikkerhetskopi av den zvolen først øyeblikksbildet `bombvault-vm-<time>` og klonen `<dataset>-bombvault-vm-<time>`. Den lar alle andre øyeblikksbilder være, og beholder ett som noe annet ble klonet fra. Loggen nevner hva den fjernet eller beholdt.

BombVault bruker ikke `zfs send` til dette. Hver post i en send-strøm bærer GUID-en til øyeblikksbildet og en løpende sjekksum, så restic ville ikke funnet noe av forrige sikkerhetskopi å gjenbruke og lagret hele disken på nytt hver gang. Målt på TrueNAS SCALE 25.10 med en zvol på 512 MiB som holdt 200 MiB data: den første sikkerhetskopien la til 211 MB, en andre uten endringer 52 KB, og en etter at 1 MiB var endret 3,6 MB. Den samme zvolen gjennom `zfs send` la til 214 MB ved hver kjøring.

Hver kjøring skriver også et lite øyeblikksbilde merket `vm:<name>` med domene-XML-en, når alle diskene er inne. En VM der alle diskene er zvols, har ikke noe filøyeblikksbilde, og dette er det som lister kjøringen under VM-en og gjør den mulig å gjenopprette.

En gjenoppretting skriver hver zvol tilbake på plass, slik en fildisk skrives tilbake til filen sin, så den gjenopprettede VM-en starter med de gjenopprettede dataene:

1. Før noe røres, sjekker BombVault at kjøringen har et bilde av hver zvol-disk. En kjøring som mangler ett, avvises, og VM-en blir som den er.
2. En eksisterende zvol får først et øyeblikksbilde, `<dataset>@bombvault-before-restore-<time>`, med tiden i UTC som hvert øyeblikksbildenavn BombVault lager, så den kan ligge timer unna den lokale klokken. Loggen nevner det, og `zfs rollback` til det angrer gjenopprettingen så lenge det er det nyeste øyeblikksbildet av zvolen. Når et nyere finnes, for eksempel fra neste gjenoppretting, gjør `zfs rollback -r` det og ødelegger de nyere øyeblikksbildene. Det blir liggende til du fjerner det med `zfs destroy` når den gjenopprettede VM-en virker.
3. En zvol som ikke finnes lenger, opprettes i størrelsen den hadde da den ble sikkerhetskopiert, og en eksisterende av en annen størrelse settes tilbake til den størrelsen.
4. `dd` skriver bildet til `/dev/zvol/<dataset>`. Med komprimering på, som er TrueNAS-standarden, kommer de tomme delene av bildet tilbake som hull og tar ingen plass.

**En gjenoppretting til en annen instans trenger en målpool.** Den flytter fildisker til en målmappe, men en mappe nevner ingen ZFS-pool, så uten en avviser BombVault gjenopprettingen før den starter. Send poolen som parameteren `zvolPool` til `POST /api/foreign/restore`: hver zvol beholder stien sin under poolen (`tank/vms/win10/disk0` til `flashpool` blir `flashpool/vms/win10/disk0`), og domene-XML-en peker på den. Vinduet **Fra en gammel sikkerhetskopimappe** på siden Gjenoppretting har ikke noe felt for dette, så det kan bare nås med et direkte API-kall. En gjenoppretting på samme instans skriver hver zvol tilbake til seg selv.

**Eksport som filer utelater zvols.** En VM der alle diskene er zvols, avvises med den begrunnelsen, så gjenopprett den fra en sikkerhetskopi i stedet. For en VM med fildisker også har eksporten fildiskene og en merknad som nevner zvolene den utelot.

Dette ble prøvd på TrueNAS SCALE 25.10 med ZFS 2.3.4: tre sikkerhetskopier av en prøvezvol, en gjenoppretting på stedet etter at zvolen var overskrevet og vokst, og en gjenoppretting til en zvol som var slettet. Hver gjenoppretting leste tilbake med samme SHA-256 som kilden, zvolen hadde sin gamle størrelse igjen, og ingen øyeblikksbilder eller kloner ble liggende. En hel VM der den eneste disken er en zvol, ble sikkerhetskopiert mens den kjørte, med Pent og med Live, og avslått, og en gjenoppretting av hele VM-en (fjern definisjonen, skriv zvolene, definer, start) kjørte der også: zvolen leste tilbake med kildens sjekksum, og VM-en startet.

Fildisker, slik Unraid bruker dem, påvirkes ikke av noe av dette.

### 4. vTPM-tilstand {#truenas-vtpm}

En VM kan ha en virtuell TPM, som Secure Boot og Windows 11-gjester trenger. Tilstanden ligger utenfor disken og NVRAM, så en sikkerhetskopi av bare de to ville ført en gjest tilbake med en ny, tom TPM.

- **En passthrough-TPM**, en ekte brikke som sendes videre til gjesten, nevner stien sin i domene-XML-en. BombVault lagrer tilstanden dens ved siden av NVRAM over SSH og skriver den tilbake før `virsh define`, etter beste evne som NVRAM: en lesing eller skriving som feiler, logges og får ikke sikkerhetskopien eller gjenopprettingen til å feile.
- **En emulert vTPM**, den slags TrueNAS setter opp for Windows 11- og Secure Boot-gjester, publiserer ingen tilstandssti i domene-XML-en. BombVault melder da «no TPM path found» i stedet for å gjette en sti. Ha gjenopprettingsnøkkelen til en slik gjest, for eksempel BitLocker-nøkkelen, for hånden før du gjenoppretter den.
- **Du kan ikke selv legge en TPM til en TrueNAS-VM.** VM-enhets-API-et i TrueNAS SCALE 25.10 godtar bare `CDROM`, `DISPLAY`, `NIC`, `PCI`, `RAW`, `DISK` og `USB`, og avviser `TPM` som en ukjent type.

TPM-håndteringen er bare enhetstestet og har ikke kjørt mot en ekte TrueNAS-gjest med vTPM. En VM uten TPM, som på Unraid, sikkerhetskopieres og gjenopprettes akkurat som før.

NVRAM hentes som vanlig. TrueNAS holder den per VM på `/var/db/system/vm/nvram/<id>_<name>_VARS.fd`, og domene-XML-en nevner den stien.

### 5. VM-navn på TrueNAS {#truenas-names}

På Unraid er en VMs libvirt-navn navnet du valgte. TrueNAS finner selv på libvirt-navnet, og har endret hvordan to ganger:

| TrueNAS-versjon | libvirt-navn | Hvor navnet ditt ligger |
|---|---|---|
| 25.10 «Goldeye» | `{id}_{name}`, for eksempel `1_debian` | i libvirt-navnet, bak `{id}_` |
| 26 | VM-ens UUID, for eksempel `550e8400-e29b-41d4-a716-446655440000` | bare i `<title>` i domene-XML-en |

På en vert BombVault gjenkjenner som TrueNAS, eller som `PLATFORM=truenas` nevner, viser listen Sikkerhetskopier navnet ditt: `debian` i stedet for `1_debian` på 25.10, og `<title>` i stedet for UUID-en på 26, eller UUID-en når det ikke finnes noen tittel. Dette er bare det listen viser. Hvert `virsh`-kall og innstillingene per VM bruker fortsatt libvirt-navnet, så sikkerhetskopi og gjenoppretting alltid virker på riktig VM. På TrueNAS 26 betyr det at en VM som TrueNAS oppretter på nytt med en ny UUID, dukker opp som et nytt element uten innstillingene sine, selv om navnet ser likt ut. På Unraid endres ingenting, heller ikke for en VM du kalte `10_Windows`.

Ordningen i 25.10 ble sett på en ekte TrueNAS SCALE 25.10-boks. Ordningen i TrueNAS 26 bygger på dokumentasjonen til TrueNAS og er bare enhetstestet.

## Feilsøking {#troubleshooting}

| Symptom | Årsak og løsning |
|---|---|
| **Test tilkobling** henger, og feiler så | Verten kan ikke nås. Sjekk trinn 3 (nettverk, VLAN, brannmur) og **Host SSH: Address** og **Port**. |
| `Permission denied (publickey)` selv om nøkkelen står i `authorized_keys` | sshds `StrictModes` avviser nøkkelfilen fordi en overordnet mappe har feil eier eller modus. Det er vanlig når `/root/.ssh` er en lenke til den FAT-formaterte flash-disken (`/boot/config/ssh/...`), som er skrivbar for alle. Vertsloggen viser `Authentication refused: bad ownership or modes for directory ...`. Legg `StrictModes no` til i vertens sshd-konfigurasjon i den globale delen, før en eventuell `Match`-blokk, og kjør så `/etc/rc.d/rc.sshd restart`. Unraid skriver `/etc/ssh/sshd_config` på nytt ved hver oppstart, så gjør det varig slik det står nedenfor. |
| `Permission denied (publickey)` og nøkkelen mangler virkelig | Gjenta trinn 2, og sjekk at SSH er på. |
| `Host key verification failed` | Kjør `docker exec BombVault rm -f /config/ssh/known_hosts` og test på nytt. |
| Sjekken i trinn 3 sier `UNREACHABLE` | Containeren kan ikke nå vertens SSH-port. Se trinn 3: vertstilgang til egendefinerte nettverk, VLAN-ruting, eller shim- eller LAN-IP-en. |
| Variablene mangler under **Edit** | Importer malen på nytt. En eksisterende container beholder sin gamle lagrede konfigurasjon. |
| En VM med Unraid-øyeblikksbilder feiler og nevner en fil | Kjeden av underliggende filer til disken kan ikke følges til slutten. Sjekk i VM Manager at hvert øyeblikksbilde av den disken fortsatt er der, og at filen som er nevnt, finnes under `/mnt`. |

Flere VM-symptomer står i [Feilsøking](troubleshooting.md#vm-backup-will-not-connect).

## Å beholde det gjennom omstarter {#persistence}

Unraid skriver `/etc/ssh/sshd_config` på nytt ved hver oppstart, så endringer på vertssiden må gjøres igjen ved oppstart:

- **SSH-porten** beholdes i `/boot/config/ident.cfg`, satt gjennom Unraids webgrensesnitt.
- **`StrictModes no`**, hvis du trengte det over, beholdes ikke. Bruk det ved oppstart fra `/boot/config/go`:

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

- BombVaults egen SSH-nøkkel og `~/.ssh/config` ligger i `/config` (appdata) og lages på nytt ved hver oppstart, så de overlever omstarter av containeren og verten.

## Sikkerhet {#security}

Den autoriserte nøkkelen gir root på verten, samme tillitsnivå som Docker-socketen BombVault allerede bruker. Hold BombVault på et betrodd nettverk. Nøkkelen er ed25519, lagret med modus `0600` i appdata, og vertens nøkkel er festet. Hver vertskommando sendes som separate argumenter, aldri gjennom et fjernskall, og SSH-forbindelsen bruker `BatchMode` og en tidsgrense for tilkobling.
