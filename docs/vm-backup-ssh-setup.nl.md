# VM-back-ups

BombVault maakt back-ups van KVM/libvirt-VM's zonder een libvirt-pad te mounten. Het draait `virsh` op de host via SSH (`qemu+ssh://`), leest en schrijft de VM-schijven via de Host Data-mount (`/mnt` op de host is `/host/user` in de container) en kopieert de UEFI NVRAM via SSH. Omdat niets wat bij libvirt hoort wordt gebind-mount, kan BombVault de VM Manager van Unraid nooit in de weg zitten.

Deze gids behandelt de installatie, de back-upmethoden, VM's met Unraid-snapshots en TrueNAS SCALE. Containerback-ups hebben hier niets van nodig.

Back-ups van ZFS-datasets gebruiken dezelfde SSH-verbinding, met dezelfde sleutel, host en gebruiker. Zodra VM-back-ups verbinden, verbinden ook ZFS-datasetitems. Zie [ZFS-datasets](zfs-datasets.md).

## Wat je instelt {#what-you-configure}

| Instelling | Waar | Standaard | Betekenis |
|---|---|---|---|
| **Host SSH: Address** (`LIBVIRT_HOST`) | templatevariabele | `host.docker.internal` | De Unraid-host, bereikt via SSH |
| **Host SSH: Port** (`LIBVIRT_SSH_PORT`) | templatevariabele | `22` | De SSH-poort van Unraid |
| **Host SSH: User** (`LIBVIRT_SSH_USER`) | templatevariabele | `root` | De SSH-gebruiker op de host |
| Publieke sleutel | **Instellingen, Integraties, Host-SSH** | gemaakt bij de eerste start | Autoriseer hem op de host |

BombVault maakt zijn SSH-sleutelpaar bij de eerste start op `/config/ssh/id_ed25519`, bewaard in appdata. De sleutel van de host wordt bij de eerste verbinding vastgezet in `/config/ssh/known_hosts`.

De template vult **Host SSH: Address** met de plaatshouder `192.168.x.x`. BombVault behandelt dat als niet ingesteld en probeert `host.docker.internal`, wat werkt op het standaardnetwerk `bridge`. Vervang het op een eigen netwerk door het LAN-IP van de host (stap 3 B).

## Stap 1: SSH inschakelen op de Unraid-host {#step-1}

1. Open in Unraid **Settings, Management Access** en zet **Use SSH** op **Yes**.
2. Heb je de SSH-poort gewijzigd, noteer hem dan. **Host SSH: Port** moet overeenkomen.

## Stap 2: De publieke sleutel van BombVault autoriseren {#step-2}

De `/root` van Unraid leeft in het geheugen en wordt bij elke start gewist, dus de sleutel moet ook op de flash staan. Voer dit uit in de Unraid-terminal:

```sh
K=$(docker exec BombVault cat /config/ssh/id_ed25519.pub)
mkdir -p /root/.ssh /boot/config/ssh && chmod 700 /root/.ssh
grep -qxF "$K" /root/.ssh/authorized_keys 2>/dev/null  || echo "$K" >> /root/.ssh/authorized_keys
grep -qxF "$K" /boot/config/ssh/root.pubkeys 2>/dev/null || echo "$K" >> /boot/config/ssh/root.pubkeys
chmod 600 /root/.ssh/authorized_keys
```

Unraid 6.9 en nieuwer kopiëren `/boot/config/ssh/root.pubkeys` bij elke start naar de `authorized_keys` van root, dus dit overleeft herstarts. Het geldt voor elke login als `root`, welke hostnaam of poort BombVault ook gebruikt.

## Stap 3: Netwerk {#step-3}

BombVault moet vanuit de container een TCP-verbinding openen naar de SSH-poort van de host. Kies het geval dat bij je opstelling past.

### A. BombVault op het standaard bridge-netwerk {#bridge}

- Laat **Host SSH: Address** op `host.docker.internal` of de plaatshouder van de template staan. De template voegt `--add-host=host.docker.internal:host-gateway` toe, wat naar de gateway van docker0 wijst, en dat is de host.
- Zet **Host SSH: Port** op je SSH-poort.

### B. BombVault op een eigen netwerk {#custom-network}

Een container op `br0.x`, macvlan of ipvlan kan de host niet via `host.docker.internal` bereiken, omdat 172.17.0.1 docker0 is en niet bereikbaar vanaf `br0.x`. Doe in plaats daarvan het volgende:

1. Zet in Unraid **Settings, Docker, Host access to custom networks** op **Enabled**.
2. Zet **Host SSH: Address** op het LAN-IP van de Unraid-host, het adres waarop je de webinterface opent.
3. Staan het netwerk van de container en de host op verschillende VLAN's, sta dan de route toe op je router of firewall: van het VLAN van de container naar het IP van de host op de SSH-poort, via TCP.
4. Is het LAN-IP van de host nog steeds niet bereikbaar, gebruik dan het shim-IP van de host op het subnet van de container. Vind het op de host met `ip -4 addr show | grep -B2 <container-subnet>`.

### Controleer of de host antwoordt {#verify-reachability}

Voer dit uit in de Unraid-terminal, met je IP en poort:

```sh
docker exec BombVault timeout 6 bash -c 'echo > /dev/tcp/192.168.1.10/22' && echo OPEN || echo UNREACHABLE
```

`OPEN` betekent dat het pad werkt. `UNREACHABLE` betekent dat het netwerk in stap 3 eerst gerepareerd moet worden.

## Stap 4: De variabelen instellen en testen {#step-4}

1. Open in Unraid **Docker**, klik op BombVault, kies **Edit**, stel **Host SSH: Address** in (en **Host SSH: Port** als die niet 22 is) en druk op **Apply**. Ontbreken de variabelen, importeer de template dan opnieuw, want Unraid bewaart de opgeslagen configuratie van een bestaande container.
2. Open in BombVault **Instellingen, Integraties, Host-SSH** en druk op **Verbinding testen**. Het hoort groen te worden.

Dezelfde aanroep vanuit de terminal toont je VM's:

```sh
docker exec BombVault virsh -c "qemu+ssh://root@192.168.1.10:22/system?keyfile=/config/ssh/id_ed25519&known_hosts=/config/ssh/known_hosts&known_hosts_verify=auto" list --all
```

## Stap 5: De methode per VM {#per-vm-method}

Elke VM heeft een **Methode** in de kaart **VM** op zijn tabblad Instellingen (open de VM onder **Back-ups**):

- **Net (afsluiten)** is de standaard. Het sluit de VM af, maakt een back-up van de schijven en start hem weer, dus de back-up is altijd consistent. Het wacht tot tien minuten tot de gast is afgesloten en forceert hem nooit uit. Een gast die daarna nog draait, wordt met rust gelaten, en die back-up mislukt met de reden.
- **Live-snapshot** maakt een back-up van een draaiende VM via een externe snapshot en `blockcommit`. Het heeft de qemu guest agent in de VM nodig, voor een gequiesceerde snapshot, en de schijf op `/mnt/cache` of `/mnt/diskX`, niet `/mnt/user`. Bij een VM die uitstaat valt Live vanzelf terug op Net (afsluiten), en als de snapshot niet kan worden gemaakt, valt het ook terug.

## Alleen gewijzigde blokken {#changed-blocks}

Een VM met qcow2-schijven kan worden geback-upt door alleen te lezen wat veranderde. Schakel **Alleen gewijzigde blokken** in dezelfde kaart in, en BombVault gebruikt de back-up-API van libvirt in plaats van de methode hierboven:

- Het start een backupjob in pull-modus (`virsh backup-begin`) dat op hetzelfde moment een checkpoint met de naam `bombvault-<time>` aanmaakt, en leest de schijven over de SSH-verbinding via een doorgestuurde Unix-socket. Antwoordt de guest agent, dan worden de bestandssystemen van de gast alleen bevroren terwijl het job start. Een ontdooiing die mislukt, wordt ongeveer vijftien seconden opnieuw geprobeerd, en ontdooit de gast dan nog niet, dan stuurt BombVault een melding.
- De eerste run leest elk toegewezen blok. Latere runs lezen alleen de blokken die de bitmap van het checkpoint markeert als geschreven sinds de vorige back-up.
- Elke schijf wordt opgeslagen als een map met segmenten van vaste grootte die bevatten wat de gast ziet. Ongewijzigde segmenten worden uit de vorige snapshot overgenomen zonder te worden gelezen, dus elke snapshot is compleet en herstelt op zichzelf.
- Een herstel bouwt de schijf uit zijn segmenten op en zet hem met `qemu-img` terug om naar qcow2. De schijf heeft dezelfde inhoud als op het moment van de back-up, maar het bestand is een nieuw qcow2-bestand, en een backingketen komt afgevlakt terug.
- De controle vóór zo'n herstel toont elke schijf die het opbouwt en telt hem op de volledige grootte van de gast, twee keer voor qcow2, omdat het ruwe image en zijn omgezette kopie naast elkaar staan tot het oude bestand is vervangen. Dat is het meeste wat een schijf kan innemen, dus een tekort volgens deze telling laat de ruimteregel grijs in plaats van te blokkeren. De herstelcontrole na de eerste back-up, en **Herstel controleren**, zorgen er ook voor dat de snapshot elk segment van elke schijf heeft op de grootte die het hoort te hebben.
- Tussen runs heeft de VM geen checkpoint in libvirt geregistreerd. BombVault bewaart de definitie van het nieuwste checkpoint `bombvault-` onder `/config/vm-checkpoints` en verwijdert alleen de registratie ervan door libvirt (`virsh checkpoint-delete --metadata`). Zijn bitmap blijft in het qcow2-bestand en blijft schrijfacties tellen, en de volgende run zet het checkpoint terug (`virsh checkpoint-create --redefine`) voordat hij begint. libvirt weigert een VM te undefinen die checkpoints heeft, dus dit is wat **Remove VM** van Unraid en een herstel zoals gewoonlijk laat werken.
- Een backupjob dat BombVault liet doorlopen, omdat het midden in een back-up stopte, wordt beëindigd als BombVault weer start, samen met de checkpoints die die run registreerde.
- De schakelaar uitzetten, de VM uit de planning halen of zijn back-ups verwijderen beëindigt zo'n job ook, en verwijdert ook de bitmap zolang de VM draait. Bij een VM die uitstaat, kan libvirt hem niet verwijderen, en hij blijft ongebruikt in het image.
- Checkpoints van andere tools blijven met rust. Een VM die er heeft, kan niet worden hersteld tot ze zijn verwijderd, en het herstel stopt voordat het de VM aanraakt.

De schakelaar valt vanzelf terug. Een ontbrekend of kapot checkpoint, een schijf die van grootte veranderde, of een nieuwste snapshot die geen snapshot met gewijzigde blokken is, betekent dat de hele schijf nog eens wordt gelezen. Een VM die uitstaat, een schijf van het type raw of zvol, geen host-SSH-verbinding, een libvirt zonder de back-up-API, of te weinig vrije ruimte onder `/config` betekent de gebruikelijke back-up met de methode hierboven. De kaart zegt welke daarvan de laatste run gebruikte en waarom.

Vereisten: libvirt met de back-up-API (getest met libvirt 12.2 en QEMU 10.2 op Unraid 7.3), sshd met `AllowStreamLocalForwarding yes` (de standaard, ook op Unraid, dat TCP-forwarding uitzet), en qcow2-schijven. libvirt weigert checkpoints op raw-schijven, en Unraid maakt standaard raw vdisks aan. De VM blijft tijdens de back-up draaien, zoals bij Live, en de segmenten die op restic wachten nemen tot 4 GiB onder `/config` in beslag.

## VM's met Unraid-snapshots {#unraid-snapshots}

Een snapshot die de VM Manager van Unraid maakt, verandert een qcow2-schijf in een keten: het originele image onderaan, en daarop één bestand per snapshot, elk met alleen wat veranderde sinds het bestand eronder. De VM schrijft in het nieuwste bestand. Een schijf met twee snapshots is drie bestanden.

BombVault maakt van zo'n VM een volledige back-up:

- **Elk bestand van de keten.** Het volgt de backingketen van elke schijf tot het einde en maakt een back-up van elk bestand erin, samen met de snapshotlijst van Unraid (`/etc/libvirt/qemu/snapshotdb/<vm>/snapshots.db`, gelezen via SSH). Een keten die niet tot het einde gevolgd kan worden, bijvoorbeeld omdat een bestand in het midden weg is, laat de back-up mislukken en noemt het bestand.
- **Een live back-up bewaart de snapshots.** **Live-snapshot** zet een tijdelijke overlay erbovenop terwijl het de schijf leest en voegt hem daarna weer samen, alleen in het image er direct onder. De keten is na de back-up dezelfde als ervoor. Een VM zonder snapshots gedraagt zich zoals altijd.
- **Een herstel brengt de keten terug.** Een herstel op zijn plaats schrijft elk bestand terug waar het stond en zet de snapshotlijst van Unraid terug, zodat de VM Manager de snapshots weer toont. Een herstel naast het origineel schrijft de bestanden in de map van de kopie en wijst de herstelde schijven naar de herstelde bestanden, zodat de kopie nooit uit de images van het origineel leest. De snapshotlijst van Unraid komt alleen terug bij een herstel op zijn plaats, omdat ze de bestanden noemt waar ze zijn geback-upt. Zonder haar start de kopie hetzelfde op, en de VM Manager toont er alleen geen snapshots voor.
- **Alleen gewijzigde blokken** leest wat de gast ziet, dus het herstel komt terug als één afgevlakt qcow2-bestand zonder de snapshots.

### Voorbeeld: een VM met twee snapshots {#example-unraid-snapshots}

De VM `homeassistant` heeft één schijf, `vdisk1.qcow2`, en twee snapshots die je in de VM Manager maakte vóór twee updates. Zijn schijf is nu een keten van drie bestanden. Hij draait de hele tijd, dus hij gebruikt **Live-snapshot**.

1. Open **Back-ups**, dan `homeassistant`, dan zijn tabblad **Instellingen**. Controleer in de kaart **VM** dat **Methode** op **Live-snapshot** staat en **Alleen gewijzigde blokken** uit.
2. Druk in de kop van de pagina op **Nu back-up maken**.
3. Als de run klaar is, is zijn regel in **Recente uitvoeringen** op het tabblad **Verificatie** groen. De VM bleef de hele tijd draaien, en de back-up bevat alle drie de bestanden van de schijf en de snapshotlijst van Unraid.
4. In de VM Manager staan de twee snapshots er nog, en de schijf leest nog steeds via dezelfde drie bestanden.

Een maand later gaat een update mis en je wilt de toestand van gisteren terug naast de kapotte, om te vergelijken:

1. Open **Herstel**, kies **VM's** en klik op `homeassistant`.
2. Houd de aanbevolen back-up van afgelopen nacht en kies **Naast het origineel** onder **Waarheen**.
3. Druk op **Herstellen**. De drie bestanden komen in een map die naar de VM is genoemd met `-restored`, en de herstelde schijf wijst naar de herstelde bestanden. Er verandert niets aan de draaiende VM.

Om de hele VM in plaats daarvan terug te draaien, kies je **Origineel vervangen** met **Eerst de huidige staat back-uppen** aan. BombVault maakt een back-up van de kapotte toestand, sluit de VM af, schrijft de drie bestanden en de snapshotlijst van Unraid terug en start hem weer.

## TrueNAS SCALE {#truenas-scale}

De stappen hierboven zijn voor Unraid geschreven, maar dezelfde `qemu+ssh://`-verbinding werkt tegen elke libvirtd die de container kan bereiken, TrueNAS SCALE inbegrepen, dat sinds 25.04.2 weer VM's op libvirt heeft. Op TrueNAS heeft de verbindingsstring een override nodig en gaat de sleutel naar root in de eigen gebruikersinstellingen van TrueNAS. Drie dingen meer veranderen wat een back-up bevat: schijven op zvols, vTPM-status en de namen die TrueNAS zijn VM's geeft.

### 1. De libvirt-socket {#truenas-socket}

De libvirtd van TrueNAS luistert niet waar de verbindingsstring die uit de velden van **Host-SSH** wordt opgebouwd het verwacht. Stel in plaats daarvan `LIBVIRT_URI` in, met de extra parameter `?socket=`:

```sh
LIBVIRT_URI=qemu+ssh://<user>@<truenas-host>/system?socket=/run/truenas_libvirt/libvirt-sock
```

BombVault gebruikt `LIBVIRT_URI` precies zoals geschreven, en `LIBVIRT_HOST`, `LIBVIRT_SSH_USER` en `LIBVIRT_SSH_PORT` zijn niet nodig. Voor elk daarvan dat niet is ingesteld, neemt BombVault de host, gebruiker en poort uit de URI, ook voor zijn eigen SSH-opdrachten, zodat de NVRAM-kopie en back-ups van ZFS-datasets dezelfde machine bereiken als `virsh`. Een variabele die je zelf instelt, wint altijd, en het opstartlogboek noemt het doel dat uit de URI is genomen. Op TrueNAS heeft een andere gebruiker dan root voor back-ups van ZFS-datasets ook `zfs allow <user> snapshot,destroy,mount <dataset>` nodig, zie [ZFS-datasets](zfs-datasets.md#truenas).

Zet het LAN-adres van TrueNAS in de URI. Zijn noch `LIBVIRT_URI` noch `LIBVIRT_HOST` ingesteld, dan neemt BombVault de standaardgateway van de app als host, omdat `host.docker.internal` in een TrueNAS-app niet oplost, en het opstartlogboek noemt het adres dat het koos. Dat is genoeg voor back-ups van ZFS-datasets. VM-back-ups hebben de URI nog steeds nodig voor de socket.

De app heeft de capability `SETPCAP` nodig voor VM-back-ups, naast `CHOWN`, `DAC_OVERRIDE`, `FOWNER`, `SETGID` en `SETUID`. Zonder stopt libvirt met "cannot apply process capabilities". De YAML onder [TrueNAS SCALE](getting-started.md#truenas) stelt ze allemaal in, samen met de `SYS_CHROOT` die snapshot in plaats van stoppen nodig heeft.

Op TrueNAS SCALE 25.10.0 is het socketpad op echte hardware gecontroleerd: de standaard `qemu:///system` mislukt met `Failed to connect socket to '/var/run/libvirt/libvirt-sock': No such file or directory`, en dezelfde aanroep met `?socket=/run/truenas_libvirt/libvirt-sock` toont de domeinen.

### 2. De sleutel van BombVault op TrueNAS {#truenas-key}

Stap 2 hierboven schrijft naar de flash van Unraid en geldt hier niet. Op TrueNAS:

1. Zet de SSH-service aan onder **System, Services**.
2. Bewerk onder **Credentials, Users** **root** en plak de publieke sleutel van BombVault uit **Instellingen, Integraties, Host-SSH** als zijn publieke SSH-sleutel. TrueNAS bewaart hem over herstarts en updates heen.

Verse installaties van TrueNAS 24.10 en nieuwer geven root geen wachtwoord, en het inloggen met de sleutel heeft er geen nodig. Wil je root liever niet gebruiken, autoriseer de sleutel dan voor een beheerdersaccount met sudo en zet zijn naam in plaats van `root` in `LIBVIRT_URI`. Back-ups van ZFS-datasets hebben dan `zfs allow` nodig zoals hierboven beschreven.

Een ZFS-replica naar een tweede server logt in met een eigen sleutel, die het replicavenster toont. De Host-SSH-sleutel autoriseert geen replica, en de replicasleutel autoriseert geen VM-back-ups.

!!! warning "TrueNAS ziet wijzigingen met virsh niet"
    De middleware van TrueNAS, de laag achter zijn VM-schermen, is bedoeld als de enige weg waarlangs zijn VM's worden gewijzigd. Een `virsh`-opdracht over deze SSH-verbinding, wat de VM-back-up en het herstel van BombVault uitvoeren, gaat eromheen, en TrueNAS kan later een toestand die het niet maakte, corrigeren of overschrijven. Dit komt uit het gedocumenteerde gedrag van TrueNAS en is niet op een echte machine gezien. Het is hetzelfde soort risico als dat de VM Manager van Unraid wijzigingen corrigeert die buiten hem om zijn gemaakt, alleen scherper op TrueNAS.

### 3. Schijven op zvols {#truenas-zvols}

VM-schijven van TrueNAS zijn vaak zvols, ZFS-blokapparaten op `/dev/zvol/<pool>/<dataset>`, geen qcow2-bestanden. restic kan geen blokapparaat via zijn pad back-uppen zoals een bestand, dus BombVault herkent een zvol-schijf in de domein-XML (`<source dev="...">` in plaats van `<source file="...">`) en behandelt hem over dezelfde SSH-verbinding. De BombVault-container heeft geen ZFS-tools nodig.

Een back-up van een zvol:

1. `zfs snapshot <dataset>@bombvault-vm-<time>` legt een moment vast, ook terwijl de VM draait.
2. `zfs clone -o volmode=dev -o readonly=on` maakt die snapshot leesbaar als apparaat. `volmode=dev` houdt de host ervan af de partities van de gast te scannen.
3. `dd` leest het apparaat, en het ruwe image gaat als één bestand in restic, `/vm-disks/<dataset>.img`, getagd `vm:<name>:zvol:<dev>`. restic knipt het op inhoud in stukken, zodat blokken die sinds de vorige back-up niet veranderden niet opnieuw worden opgeslagen.
4. De clone en de snapshot worden daarna vernietigd, of de back-up nu werkte of niet. Stopte BombVault midden in een back-up, dan verwijdert de volgende back-up van die zvol eerst de snapshot `bombvault-vm-<time>` en zijn clone `<dataset>-bombvault-vm-<time>`. Het laat elke andere snapshot met rust en houdt er een waarvan iets anders is gekloond. Het logboek noemt wat het verwijderde of hield.

BombVault gebruikt hiervoor geen `zfs send`. Elk record van een sendstream draagt de GUID van de snapshot en een lopende checksum, dus restic zou niets van de laatste back-up vinden om te hergebruiken en zou de hele schijf elke keer opnieuw opslaan. Gemeten op TrueNAS SCALE 25.10 met een zvol van 512 MiB met 200 MiB data: de eerste back-up voegde 211 MB toe, een tweede zonder wijzigingen 52 KB, en een na 1 MiB wijziging 3,6 MB. Hetzelfde zvol via `zfs send` voegde bij elke run 214 MB toe.

Elke run schrijft ook een kleine snapshot getagd `vm:<name>` met de domein-XML, zodra alle schijven erin zijn. Een VM waarvan alle schijven zvols zijn, heeft geen bestandssnapshot, en dit is degene die de run onder de VM toont en hem herstelbaar maakt.

Een herstel schrijft elke zvol terug op zijn plaats, zoals een bestandsschijf naar zijn bestand wordt teruggeschreven, zodat de herstelde VM de herstelde data opstart:

1. Voordat iets wordt aangeraakt, controleert BombVault dat de run een image van elke zvol-schijf bevat. Een run die er een mist, wordt geweigerd en de VM blijft zoals hij is.
2. Een bestaande zvol krijgt eerst een snapshot, `<dataset>@bombvault-before-restore-<time>`, met de tijd in UTC zoals elke snapshotnaam die BombVault maakt, dus hij kan uren afwijken van de lokale klok. Het logboek noemt hem, en `zfs rollback` ernaartoe maakt het herstel ongedaan zolang het de nieuwste snapshot van de zvol is. Zodra er een nieuwere bestaat, zoals die van het volgende herstel, doet `zfs rollback -r` het en vernietigt de nieuwere snapshots. Hij blijft staan tot je hem met `zfs destroy` verwijdert nadat de herstelde VM werkt.
3. Een zvol die niet meer bestaat, wordt aangemaakt op de grootte die hij had toen hij werd geback-upt, en een bestaande van een andere grootte wordt op die grootte teruggezet.
4. `dd` schrijft het image op `/dev/zvol/<dataset>`. Met compressie aan, de standaard van TrueNAS, komen de lege delen van het image terug als gaten en nemen geen ruimte in.

**Een herstel op een andere instantie heeft een doelpool nodig.** Het verplaatst bestandsschijven naar een doelmap, maar een map noemt geen ZFS-pool, dus zonder pool weigert BombVault het herstel voordat het begint. Geef de pool mee als de parameter `zvolPool` van `POST /api/foreign/restore`: elke zvol behoudt zijn pad onder de pool (`tank/vms/win10/disk0` naar `flashpool` wordt `flashpool/vms/win10/disk0`), en de domein-XML wijst ernaar. Het venster **Uit een oude back-upmap** op de pagina Herstel heeft hier geen veld voor, dus het is alleen bereikbaar met een directe API-aanroep. Een herstel op dezelfde instantie schrijft elke zvol terug op zichzelf.

**Exporteren als bestanden laat zvols weg.** Een VM waarvan alle schijven zvols zijn, wordt met die reden geweigerd, dus herstel hem vanuit een back-up. Voor een VM met ook bestandsschijven bevat de export de bestandsschijven en een opmerking die de zvols noemt die zijn weggelaten.

Dit is geprobeerd op TrueNAS SCALE 25.10 met ZFS 2.3.4: drie back-ups van een proefzvol, een herstel op zijn plaats nadat de zvol was overschreven en vergroot, en een herstel naar een zvol die was verwijderd. Elk herstel las terug met dezelfde SHA-256 als de bron, de zvol had weer zijn oude grootte, en er bleef geen snapshot of clone achter. Een hele VM waarvan de enige schijf een zvol is, werd draaiend geback-upt, met Net (afsluiten) en met Live, en uitgeschakeld, en een herstel van een hele VM (undefine, de zvols schrijven, define, opstarten) liep daar ook: de zvol las terug met de checksum van de bron en de VM startte.

Bestandsschijven, zoals Unraid ze gebruikt, worden hierdoor niet geraakt.

### 4. vTPM-status {#truenas-vtpm}

Een VM kan een virtuele TPM hebben, die Secure Boot en Windows 11-gasten nodig hebben. De status ervan leeft buiten de schijf en de NVRAM, dus een back-up van alleen die twee zou een gast terugbrengen met een nieuwe, lege TPM.

- **Een passthrough-TPM**, een echte chip die aan de gast wordt doorgegeven, noemt zijn pad in de domein-XML. BombVault bewaart zijn status naast de NVRAM via SSH en schrijft hem terug vóór `virsh define`, op best-effort-basis zoals de NVRAM: een mislukte lees- of schrijfactie wordt gelogd en laat de back-up of het herstel niet mislukken.
- **Een geëmuleerde vTPM**, het soort dat TrueNAS instelt voor Windows 11- en Secure Boot-gasten, publiceert geen statuspad in de domein-XML. BombVault meldt dan "no TPM path found" in plaats van een pad te raden. Houd de herstelsleutel van zo'n gast, zoals zijn BitLocker-sleutel, bij de hand voordat je hem herstelt.
- **Je kunt zelf geen TPM aan een TrueNAS-VM koppelen.** De VM-device-API van TrueNAS SCALE 25.10 accepteert alleen `CDROM`, `DISPLAY`, `NIC`, `PCI`, `RAW`, `DISK` en `USB`, en weigert `TPM` als onbekend type.

De TPM-afhandeling is alleen met unit-tests getest en is niet gedraaid tegen een echte TrueNAS-gast met een vTPM. Een VM zonder TPM, zoals op Unraid, wordt precies als voorheen geback-upt en hersteld.

NVRAM wordt zoals gewoonlijk bewaard. TrueNAS bewaart hem per VM op `/var/db/system/vm/nvram/<id>_<name>_VARS.fd`, en de domein-XML noemt dat pad.

### 5. VM-namen op TrueNAS {#truenas-names}

Op Unraid is de libvirt-naam van een VM de naam die je koos. TrueNAS verzint de libvirt-naam, en heeft de manier al twee keer veranderd:

| TrueNAS-versie | libvirt-naam | Waar jouw naam leeft |
|---|---|---|
| 25.10 "Goldeye" | `{id}_{name}`, zoals `1_debian` | in de libvirt-naam, achter de `{id}_` |
| 26 | de UUID van de VM, zoals `550e8400-e29b-41d4-a716-446655440000` | alleen in de `<title>` van de domein-XML |

Op een host die BombVault als TrueNAS detecteert, of die `PLATFORM=truenas` noemt, toont de lijst Back-ups jouw naam: `debian` in plaats van `1_debian` op 25.10, en de `<title>` in plaats van de UUID op 26, of de UUID als er geen titel is. Dit is alleen wat de lijst toont. Elke `virsh`-aanroep en de instellingen per VM gebruiken nog de libvirt-naam, dus back-up en herstel werken altijd op de juiste VM. Op TrueNAS 26 betekent dat dat een VM die TrueNAS opnieuw aanmaakt met een nieuwe UUID verschijnt als een nieuw item zonder zijn instellingen, ook al lijkt zijn naam hetzelfde. Op Unraid verandert er niets, ook niet voor een VM die je `10_Windows` noemde.

Het schema van 25.10 is gezien op een echte TrueNAS SCALE 25.10-machine. Het schema van TrueNAS 26 komt uit de documentatie van TrueNAS en is alleen met unit-tests getest.

## Problemen oplossen {#troubleshooting}

| Symptoom | Oorzaak en oplossing |
|---|---|
| **Verbinding testen** blijft hangen en mislukt dan | De host is niet bereikbaar. Controleer stap 3 (netwerk, VLAN, firewall) en **Host SSH: Address** en **Port**. |
| `Permission denied (publickey)` hoewel de sleutel in `authorized_keys` staat | `StrictModes` van sshd weigert het sleutelbestand omdat een bovenliggende map de verkeerde eigenaar of modus heeft. Dat komt vaak voor als `/root/.ssh` een link is naar de FAT-flash (`/boot/config/ssh/...`), die voor iedereen schrijfbaar is. Het hostlogboek toont `Authentication refused: bad ownership or modes for directory ...`. Voeg `StrictModes no` toe aan de sshd-configuratie van de host in de globale sectie, vóór elk `Match`-blok, en voer dan `/etc/rc.d/rc.sshd restart` uit. Unraid schrijft `/etc/ssh/sshd_config` bij elke start opnieuw, dus maak het blijvend zoals hieronder beschreven. |
| `Permission denied (publickey)` en de sleutel ontbreekt echt | Herhaal stap 2 en controleer of SSH aanstaat. |
| `Host key verification failed` | Voer `docker exec BombVault rm -f /config/ssh/known_hosts` uit en test opnieuw. |
| De controle in stap 3 zegt `UNREACHABLE` | De container kan de SSH-poort van de host niet bereiken. Zie stap 3: hosttoegang tot eigen netwerken, VLAN-routering, of het shim- of LAN-IP. |
| De variabelen ontbreken onder **Edit** | Importeer de template opnieuw. Een bestaande container houdt zijn oude opgeslagen configuratie. |
| Een VM met Unraid-snapshots mislukt en noemt een bestand | De backingketen van de schijf kan niet tot het einde worden gevolgd. Controleer in de VM Manager dat elke snapshot van die schijf er nog is en dat het genoemde bestand onder `/mnt` bestaat. |

Meer VM-symptomen staan bij [Problemen oplossen](troubleshooting.md#vm-backup-will-not-connect).

## Blijvend maken over herstarts heen {#persistence}

Unraid schrijft `/etc/ssh/sshd_config` bij elke start opnieuw, dus wijzigingen aan de hostkant moeten bij het opstarten opnieuw worden gemaakt:

- De **SSH-poort** wordt bewaard in `/boot/config/ident.cfg`, ingesteld via de Unraid-webinterface.
- **`StrictModes no`**, als je het hierboven nodig had, wordt niet bewaard. Pas het bij het opstarten toe vanuit `/boot/config/go`:

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

- De eigen SSH-sleutel en `~/.ssh/config` van BombVault staan in `/config` (appdata) en worden bij elke start opnieuw gemaakt, dus ze overleven herstarts van de container en de host.

## Beveiliging {#security}

De geautoriseerde sleutel geeft root op de host, hetzelfde vertrouwensniveau als de Docker-socket die BombVault al gebruikt. Houd BombVault op een vertrouwd netwerk. De sleutel is ed25519, opgeslagen met modus `0600` in appdata, en de sleutel van de host is vastgezet. Elke hostopdracht wordt als aparte argumenten doorgegeven, nooit via een remote shell, en de SSH-verbinding gebruikt `BatchMode` en een verbindingstime-out.
