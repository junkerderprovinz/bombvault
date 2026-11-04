# Backupuri VM

BombVault face backup VM-urilor KVM/libvirt fără să monteze vreo cale libvirt. Rulează `virsh` pe gazdă prin SSH (`qemu+ssh://`), citește și scrie discurile VM prin montarea Host Data (`/mnt` de pe gazdă este `/host/user` în container) și copiază NVRAM-ul UEFI prin SSH. Pentru că nimic din ce ține de libvirt nu este montat prin bind, BombVault nu poate încurca niciodată VM Manager-ul Unraid.

Acest ghid acoperă configurarea, metodele de backup, VM-urile cu instantanee Unraid și TrueNAS SCALE. Backupurile de containere nu au nevoie de nimic din toate acestea.

Backupurile seturilor de date ZFS folosesc aceeași legătură SSH, cu aceeași cheie, aceeași gazdă și același utilizator. Odată ce backupurile VM se conectează, se conectează și elementele de seturi de date ZFS. Vezi [Seturi de date ZFS](zfs-datasets.md).

## Ce configurezi {#what-you-configure}

| Setare | Unde | Implicit | Înțeles |
|---|---|---|---|
| **Host SSH: Address** (`LIBVIRT_HOST`) | variabilă de șablon | `host.docker.internal` | Gazda Unraid, accesată prin SSH |
| **Host SSH: Port** (`LIBVIRT_SSH_PORT`) | variabilă de șablon | `22` | Portul SSH al Unraid |
| **Host SSH: User** (`LIBVIRT_SSH_USER`) | variabilă de șablon | `root` | Utilizatorul SSH de pe gazdă |
| Cheia publică | **Setări, Integrări, SSH al gazdei** | creată la prima pornire | Autorizeaz-o pe gazdă |

BombVault își creează perechea de chei SSH la prima pornire, în `/config/ssh/id_ed25519`, păstrată în appdata. Cheia gazdei este fixată în `/config/ssh/known_hosts` la prima conexiune.

Șablonul completează **Host SSH: Address** cu substituentul `192.168.x.x`. BombVault îl tratează ca nesetat și încearcă `host.docker.internal`, care funcționează pe rețeaua `bridge` implicită. Pe o rețea personalizată, înlocuiește-l cu IP-ul LAN al gazdei (pasul 3 B).

## Pasul 1: Activează SSH pe gazda Unraid {#step-1}

1. În Unraid, deschide **Settings, Management Access** și setează **Use SSH** pe **Yes**.
2. Dacă ai schimbat portul SSH, notează-l. **Host SSH: Port** trebuie să corespundă.

## Pasul 2: Autorizează cheia publică a BombVault {#step-2}

`/root` din Unraid trăiește în memorie și este șters la fiecare pornire, deci cheia trebuie să fie și pe unitatea flash. Rulează asta în terminalul Unraid:

```sh
K=$(docker exec BombVault cat /config/ssh/id_ed25519.pub)
mkdir -p /root/.ssh /boot/config/ssh && chmod 700 /root/.ssh
grep -qxF "$K" /root/.ssh/authorized_keys 2>/dev/null  || echo "$K" >> /root/.ssh/authorized_keys
grep -qxF "$K" /boot/config/ssh/root.pubkeys 2>/dev/null || echo "$K" >> /boot/config/ssh/root.pubkeys
chmod 600 /root/.ssh/authorized_keys
```

Unraid 6.9 și mai nou copiază `/boot/config/ssh/root.pubkeys` în `authorized_keys` al lui root la fiecare pornire, deci asta rezistă la reporniri. Este valabil pentru orice autentificare `root`, indiferent de numele gazdei sau portul pe care îl folosește BombVault.

## Pasul 3: Rețeaua {#step-3}

BombVault trebuie să deschidă o conexiune TCP de la container la portul SSH al gazdei. Alege cazul care se potrivește configurației tale.

### A. BombVault pe rețeaua bridge implicită {#bridge}

- Lasă **Host SSH: Address** pe `host.docker.internal` sau pe substituentul șablonului. Șablonul adaugă `--add-host=host.docker.internal:host-gateway`, care arată spre gateway-ul docker0, adică gazda.
- Setează **Host SSH: Port** la portul tău SSH.

### B. BombVault pe o rețea personalizată {#custom-network}

Un container pe `br0.x`, macvlan sau ipvlan nu poate ajunge la gazdă prin `host.docker.internal`, pentru că 172.17.0.1 este docker0 și nu poate fi atins din `br0.x`. În schimb:

1. În Unraid, setează **Settings, Docker, Host access to custom networks** pe **Enabled**.
2. Setează **Host SSH: Address** la IP-ul LAN al gazdei Unraid, adresa la care deschizi interfața web.
3. Dacă rețeaua containerului și gazda sunt pe VLAN-uri diferite, permite ruta pe routerul sau firewall-ul tău: de la VLAN-ul containerului la IP-ul gazdei, pe portul SSH, prin TCP.
4. Dacă IP-ul LAN al gazdei tot nu poate fi atins, folosește IP-ul shim al gazdei din subrețeaua containerului. Îl găsești pe gazdă cu `ip -4 addr show | grep -B2 <container-subnet>`.

### Verifică dacă gazda răspunde {#verify-reachability}

Rulează asta în terminalul Unraid, cu IP-ul și portul tău:

```sh
docker exec BombVault timeout 6 bash -c 'echo > /dev/tcp/192.168.1.10/22' && echo OPEN || echo UNREACHABLE
```

`OPEN` înseamnă că drumul funcționează. `UNREACHABLE` înseamnă că rețeaua de la pasul 3 trebuie reparată înainte de orice altceva.

## Pasul 4: Setează variabilele și testează {#step-4}

1. În Unraid, deschide **Docker**, fă clic pe BombVault, alege **Edit**, setează **Host SSH: Address** (și **Host SSH: Port** dacă nu este 22) și apasă **Apply**. Dacă variabilele lipsesc, importă din nou șablonul, pentru că Unraid păstrează configurația salvată a unui container existent.
2. În BombVault, deschide **Setări, Integrări, SSH al gazdei** și apasă **Testează conexiunea**. Ar trebui să devină verde.

Același apel din terminal îți listează VM-urile:

```sh
docker exec BombVault virsh -c "qemu+ssh://root@192.168.1.10:22/system?keyfile=/config/ssh/id_ed25519&known_hosts=/config/ssh/known_hosts&known_hosts_verify=auto" list --all
```

## Pasul 5: Metoda pe VM {#per-vm-method}

Fiecare VM are o **Metodă** în cardul **VM** din fila lui Setări (deschide VM-ul la **Copii de rezervă**):

- **Controlat (oprire)** este implicitul. Oprește VM-ul, face backup discurilor și îl pornește din nou, deci backupul este mereu consistent. Așteaptă până la zece minute ca oaspetele să se oprească și nu îl oprește niciodată forțat. Un oaspete care încă rulează după aceea este lăsat în pace, iar acel backup eșuează cu motivul.
- **Instantaneu live** face backup unui VM care rulează printr-un instantaneu extern și `blockcommit`. Are nevoie de qemu guest agent în VM, pentru un instantaneu liniștit, și de discul pe `/mnt/cache` sau `/mnt/diskX`, nu pe `/mnt/user`. Pe un VM oprit, Live revine singur la Controlat, și tot așa când instantaneul nu poate fi făcut.

## Doar blocurile modificate {#changed-blocks}

Un VM cu discuri qcow2 poate fi salvat citind doar ce s-a schimbat. Pornește **Doar blocurile modificate** în același card, iar BombVault folosește API-ul de backup al libvirt în loc de metoda de mai sus:

- Pornește o sarcină de backup în modul pull (`virsh backup-begin`) care creează în același moment un punct de control numit `bombvault-<time>` și citește discurile prin legătura SSH, printr-un socket Unix redirecționat. Când agentul oaspetelui răspunde, sistemele de fișiere ale oaspetelui sunt înghețate doar cât pornește sarcina. O dezghețare care eșuează este reîncercată circa cincisprezece secunde, iar dacă oaspetele tot nu se dezgheață, BombVault trimite o notificare.
- Prima rulare citește fiecare bloc alocat. Rulările următoare citesc doar blocurile pe care bitmap-ul punctului de control le marchează ca scrise de la backupul precedent.
- Fiecare disc este stocat ca un folder de segmente de dimensiune fixă, care conține ce vede oaspetele. Segmentele nemodificate sunt preluate din instantaneul precedent fără să fie citite, deci fiecare instantaneu este complet și se restaurează singur.
- O restaurare reconstruiește discul din segmentele lui și îl convertește înapoi în qcow2 cu `qemu-img`. Discul are același conținut ca la momentul backupului, dar fișierul este un fișier qcow2 nou, iar un lanț de bază revine aplatizat.
- Verificarea dinaintea unei asemenea restaurări listează fiecare disc pe care îl reconstruiește și îl numără la dimensiunea completă a oaspetelui, de două ori pentru qcow2, pentru că imaginea brută și copia ei convertită stau una lângă alta până când fișierul vechi este înlocuit. Este maximul pe care îl poate ocupa un disc, deci o lipsă după această numărătoare lasă rândul de spațiu gri, în loc să blocheze. Verificarea de restaurare de după primul backup și **Verifică restaurarea** se asigură și că instantaneul conține fiecare segment al fiecărui disc la dimensiunea pe care ar trebui s-o aibă.
- Între rulări, VM-ul nu are niciun punct de control înregistrat în libvirt. BombVault păstrează definiția celui mai nou punct de control `bombvault-` în `/config/vm-checkpoints` și elimină doar înregistrarea lui din libvirt (`virsh checkpoint-delete --metadata`). Bitmap-ul rămâne în fișierul qcow2 și continuă să numere scrierile, iar următoarea rulare pune punctul de control la loc (`virsh checkpoint-create --redefine`) înainte să înceapă. libvirt refuză să șteargă definiția unui VM care are puncte de control, așa că asta face ca **Remove VM** din Unraid și o restaurare să funcționeze ca de obicei.
- O sarcină de backup pe care BombVault a lăsat-o în execuție, pentru că s-a oprit în mijlocul unui backup, este încheiată când BombVault pornește din nou, împreună cu punctele de control pe care acea rulare le-a lăsat înregistrate.
- Oprirea comutatorului, scoaterea VM-ului din programare sau ștergerea backupurilor lui încheie de asemenea o astfel de sarcină și șterge și bitmap-ul cât timp VM-ul rulează. Pe un VM oprit, libvirt nu îl poate șterge, iar el rămâne nefolosit în imagine.
- Punctele de control ale altor instrumente sunt lăsate în pace. Un VM care are astfel de puncte nu poate fi restaurat până nu sunt șterse, iar restaurarea se oprește înainte să atingă VM-ul.

Comutatorul revine singur la metoda obișnuită. Un punct de control lipsă sau stricat, un disc care și-a schimbat dimensiunea sau un instantaneu cel mai nou care nu este de tip blocuri modificate înseamnă că întregul disc este citit încă o dată. Un VM oprit, un disc brut sau zvol, lipsa legăturii SSH la gazdă, un libvirt fără API-ul de backup sau prea puțin spațiu liber sub `/config` înseamnă backupul obișnuit cu metoda de mai sus. Cardul spune pe care dintre acestea a folosit-o ultima rulare și de ce.

Cerințe: libvirt cu API-ul de backup (testat cu libvirt 12.2 și QEMU 10.2 pe Unraid 7.3), sshd cu `AllowStreamLocalForwarding yes` (implicit, și pe Unraid, care oprește redirecționarea TCP) și discuri qcow2. libvirt refuză punctele de control pe discuri brute, iar Unraid creează implicit vdisk-uri brute. VM-ul continuă să ruleze în timpul backupului, ca la Live, iar segmentele care așteaptă restic ocupă până la 4 GiB sub `/config`.

## VM-uri cu instantanee Unraid {#unraid-snapshots}

Un instantaneu pe care îl face VM Manager-ul Unraid transformă un disc qcow2 într-un lanț: imaginea originală jos și deasupra ei un fișier pe instantaneu, fiecare conținând doar ce s-a schimbat față de cel de sub el. VM-ul scrie în cel mai nou fișier. Un disc cu două instantanee înseamnă trei fișiere.

BombVault face backup unui astfel de VM în întregime:

- **Fiecare fișier al lanțului.** Urmărește lanțul de bază al fiecărui disc până la capăt și face backup fiecărui fișier din el, împreună cu lista de instantanee a Unraid (`/etc/libvirt/qemu/snapshotdb/<vm>/snapshots.db`, citită prin SSH). Un lanț care nu poate fi urmărit până la capăt, de exemplu pentru că lipsește un fișier din mijloc, face backupul să eșueze și numește fișierul.
- **Un backup live păstrează instantaneele.** **Instantaneu live** pune deasupra un overlay temporar cât citește discul și îl îmbină la loc după aceea, doar în imaginea de imediat dedesubt. Lanțul este același după backup ca înainte. Un VM fără instantanee se comportă ca întotdeauna.
- **O restaurare readuce lanțul.** O restaurare pe loc scrie fiecare fișier înapoi unde era și pune la loc lista de instantanee a Unraid, deci VM Manager-ul listează din nou instantaneele. O restaurare lângă original scrie fișierele în folderul copiei și îndreaptă discurile restaurate spre fișierele restaurate, așa că copia nu citește niciodată din imaginile originalului. Lista de instantanee a Unraid revine doar la o restaurare pe loc, pentru că numește fișierele acolo unde au fost salvate. Fără ea copia pornește la fel, iar VM Manager-ul doar nu listează niciun instantaneu pentru ea.
- **Doar blocurile modificate** citește ce vede oaspetele, deci restaurarea lui revine ca un singur fișier qcow2 aplatizat, fără instantanee.

### Exemplu: un VM cu două instantanee {#example-unraid-snapshots}

VM-ul `homeassistant` are un disc, `vdisk1.qcow2`, și două instantanee pe care le-ai făcut în VM Manager înainte de două actualizări. Discul lui este acum un lanț de trei fișiere. Rulează tot timpul, deci folosește **Instantaneu live**.

1. Deschide **Copii de rezervă**, apoi `homeassistant`, apoi fila lui **Setări**. În cardul **VM**, verifică dacă **Metodă** este **Instantaneu live** și **Doar blocurile modificate** este oprit.
2. Apasă **Copiază acum** în capul paginii.
3. Când rularea s-a terminat, rândul ei din **Rulări recente**, în fila **Verificare**, este verde. VM-ul a rulat tot timpul, iar backupul conține toate cele trei fișiere ale discului și lista de instantanee a Unraid.
4. În VM Manager, cele două instantanee sunt încă acolo, iar discul se citește în continuare prin aceleași trei fișiere.

O lună mai târziu, o actualizare merge prost și vrei starea de ieri lângă cea stricată, ca să le compari:

1. Deschide **Restaurare**, alege **VM-uri** și fă clic pe `homeassistant`.
2. Păstrează backupul recomandat de azi-noapte și alege **Lângă original** la **Unde**.
3. Apasă **Restaurare**. Cele trei fișiere ajung într-un folder numit ca VM-ul, cu `-restored`, iar discul restaurat arată spre fișierele restaurate. Nimic nu se schimbă la VM-ul care rulează.

Ca să dai în schimb întregul VM înapoi, alege **Înlocuiește originalul** cu **Fă mai întâi o copie a stării actuale** pornit. BombVault salvează starea stricată, oprește VM-ul, scrie înapoi cele trei fișiere și lista de instantanee a Unraid și îl pornește din nou.

## TrueNAS SCALE {#truenas-scale}

Pașii de mai sus sunt scriși pentru Unraid, dar aceeași legătură `qemu+ssh://` funcționează cu orice libvirtd pe care îl poate atinge containerul, inclusiv TrueNAS SCALE, care are din nou VM-uri pe libvirt din 25.04.2. Pe TrueNAS șirul de conexiune are nevoie de o suprascriere, iar cheia ajunge la root în setările de utilizator proprii ale TrueNAS. Alte trei lucruri schimbă ce conține un backup: discurile pe zvol-uri, starea vTPM și numele pe care TrueNAS le dă VM-urilor sale.

### 1. Socketul libvirt {#truenas-socket}

libvirtd-ul TrueNAS nu ascultă unde se așteaptă șirul de conexiune construit din câmpurile **Host SSH**. Setează în schimb `LIBVIRT_URI`, cu parametrul suplimentar `?socket=`:

```sh
LIBVIRT_URI=qemu+ssh://<user>@<truenas-host>/system?socket=/run/truenas_libvirt/libvirt-sock
```

BombVault folosește `LIBVIRT_URI` exact așa cum este scris, iar `LIBVIRT_HOST`, `LIBVIRT_SSH_USER` și `LIBVIRT_SSH_PORT` nu sunt necesare. Pentru fiecare dintre ele care nu este setată, BombVault ia gazda, utilizatorul și portul din URI și pentru propriile comenzi SSH, deci copierea NVRAM și backupurile seturilor de date ZFS ajung pe aceeași mașină ca `virsh`. O variabilă setată de tine câștigă mereu, iar jurnalul de pornire numește ținta luată din URI. Pe TrueNAS, un utilizator în afară de root are nevoie și de `zfs allow <user> snapshot,destroy,mount <dataset>` pentru backupurile seturilor de date ZFS, vezi [Seturi de date ZFS](zfs-datasets.md#truenas).

Pune adresa LAN a TrueNAS în URI. Fără `LIBVIRT_URI` și fără `LIBVIRT_HOST`, BombVault ia ca gazdă gateway-ul implicit al aplicației, pentru că `host.docker.internal` nu se rezolvă într-o aplicație TrueNAS, iar jurnalul de pornire numește adresa aleasă. Asta ajunge pentru backupurile seturilor de date ZFS. Backupurile VM au tot nevoie de URI pentru socket.

Aplicația are nevoie de capabilitatea `SETPCAP` pentru backupurile VM, lângă `CHOWN`, `DAC_OVERRIDE`, `FOWNER`, `SETGID` și `SETUID`. Fără ea libvirt se oprește cu „cannot apply process capabilities”. YAML-ul de la [TrueNAS SCALE](getting-started.md#truenas) le setează pe toate, împreună cu `SYS_CHROOT`, de care are nevoie instantaneul în loc de oprire.

Pe TrueNAS SCALE 25.10.0, calea socketului a fost verificată pe hardware real: `qemu:///system` standard eșuează cu `Failed to connect socket to '/var/run/libvirt/libvirt-sock': No such file or directory`, iar același apel cu `?socket=/run/truenas_libvirt/libvirt-sock` listează domeniile.

### 2. Cheia BombVault pe TrueNAS {#truenas-key}

Pasul 2 de mai sus scrie pe unitatea flash a Unraid și nu se aplică aici. Pe TrueNAS:

1. Pornește serviciul SSH la **System, Services**.
2. La **Credentials, Users**, editează **root** și lipește cheia publică a BombVault din **Setări, Integrări, SSH al gazdei** ca cheie publică SSH a lui. TrueNAS o păstrează peste reporniri și actualizări.

Instalările noi de TrueNAS 24.10 și mai nou nu dau lui root nicio parolă, iar autentificarea cu cheie nu are nevoie de niciuna. Dacă preferi să nu folosești root, autorizează cheia pentru un cont de administrator cu sudo și pune numele lui în locul lui `root` în `LIBVIRT_URI`. Backupurile seturilor de date ZFS au atunci nevoie de `zfs allow`, așa cum s-a descris mai sus.

O replică ZFS către un al doilea server se autentifică cu o cheie proprie, pe care o arată dialogul replicii. Cheia SSH a gazdei nu autorizează o replică, iar cheia replicii nu autorizează backupurile VM.

!!! warning "TrueNAS nu vede schimbările făcute cu virsh"
    Middleware-ul TrueNAS, stratul din spatele ecranelor sale de VM-uri, este gândit să fie singura cale prin care se schimbă VM-urile lui. O comandă `virsh` prin această legătură SSH, adică ce rulează backupul și restaurarea VM ale BombVault, o ocolește, iar TrueNAS poate mai târziu să reconcilieze sau să suprascrie o stare pe care nu a creat-o. Asta vine din comportamentul documentat al TrueNAS și nu a fost văzut pe un sistem real. Este același fel de risc ca VM Manager-ul Unraid care reconciliază schimbările făcute în afara lui, doar mai ascuțit pe TrueNAS.

### 3. Discuri pe zvol-uri {#truenas-zvols}

Discurile VM de pe TrueNAS sunt adesea zvol-uri, dispozitive bloc ZFS la `/dev/zvol/<pool>/<dataset>`, nu fișiere qcow2. restic nu poate face backup unui dispozitiv bloc după calea lui așa cum face backup unui fișier, deci BombVault recunoaște un disc zvol în XML-ul domeniului (`<source dev="...">` în loc de `<source file="...">`) și îl tratează prin aceeași legătură SSH. Containerul BombVault nu are nevoie de unelte ZFS.

Un backup al unui zvol:

1. `zfs snapshot <dataset>@bombvault-vm-<time>` fixează un moment în timp, și cât rulează VM-ul.
2. `zfs clone -o volmode=dev -o readonly=on` face acel instantaneu citibil ca dispozitiv. `volmode=dev` împiedică gazda să scaneze partițiile oaspetelui.
3. `dd` citește dispozitivul, iar imaginea brută intră în restic ca un singur fișier, `/vm-disks/<dataset>.img`, etichetat `vm:<name>:zvol:<dev>`. restic o taie în bucăți după conținut, deci blocurile care nu s-au schimbat de la ultimul backup nu sunt stocate din nou.
4. Clona și instantaneul sunt distruse după aceea, indiferent dacă backupul a mers sau nu. Dacă BombVault s-a oprit în mijlocul unui backup, următorul backup al acelui zvol elimină mai întâi instantaneul `bombvault-vm-<time>` și clona lui `<dataset>-bombvault-vm-<time>`. Lasă în pace orice alt instantaneu și îl păstrează pe cel din care s-a clonat altceva. Jurnalul numește ce a eliminat sau a păstrat.

BombVault nu folosește `zfs send` pentru asta. Fiecare înregistrare a unui flux de send poartă GUID-ul instantaneului și o sumă de control curentă, deci restic nu ar găsi nimic din ultimul backup de refolosit și ar stoca de fiecare dată tot discul din nou. Măsurat pe TrueNAS SCALE 25.10 cu un zvol de 512 MiB care conține 200 MiB de date: primul backup a adăugat 211 MB, al doilea, fără nicio schimbare, 52 KB, iar unul după ce se schimbase 1 MiB, 3,6 MB. Același zvol prin `zfs send` a adăugat 214 MB la fiecare rulare.

Fiecare rulare scrie și un mic instantaneu etichetat `vm:<name>` cu XML-ul domeniului, după ce toate discurile sunt înăuntru. Un VM ale cărui discuri sunt toate zvol-uri nu are instantaneu de fișiere, iar acesta este cel care listează rularea sub VM și o face restaurabilă.

O restaurare scrie fiecare zvol înapoi pe loc, cum un disc fișier este scris înapoi în fișierul lui, deci VM-ul restaurat pornește cu datele restaurate:

1. Înainte să se atingă ceva, BombVault verifică dacă rularea conține o imagine pentru fiecare disc zvol. O rulare căreia îi lipsește una este refuzată, iar VM-ul rămâne cum este.
2. Un zvol existent primește mai întâi un instantaneu, `<dataset>@bombvault-before-restore-<time>`, cu ora în UTC ca fiecare nume de instantaneu făcut de BombVault, deci poate diferi cu ore față de ceasul local. Jurnalul îl numește, iar `zfs rollback` la el anulează restaurarea cât timp este cel mai nou instantaneu al zvol-ului. Odată ce există unul mai nou, de exemplu cel al următoarei restaurări, `zfs rollback -r` o face și distruge instantaneele mai noi. Rămâne până îl elimini cu `zfs destroy`, după ce VM-ul restaurat funcționează.
3. Un zvol care nu mai există este creat la dimensiunea pe care o avea la backup, iar unul existent de altă dimensiune este adus înapoi la acea dimensiune.
4. `dd` scrie imaginea pe `/dev/zvol/<dataset>`. Cu compresia pornită, implicit pe TrueNAS, părțile goale ale imaginii revin ca goluri și nu ocupă spațiu.

**O restaurare pe altă instanță are nevoie de un pool de destinație.** Mută discurile fișier într-un folder de destinație, dar un folder nu numește niciun pool ZFS, deci fără unul BombVault refuză restaurarea înainte să înceapă. Dă poolul ca parametru `zvolPool` al `POST /api/foreign/restore`: fiecare zvol își păstrează calea de sub pool (`tank/vms/win10/disk0` pe `flashpool` devine `flashpool/vms/win10/disk0`), iar XML-ul domeniului arată spre ea. Fereastra **Dintr-un folder vechi de copii** de pe pagina Restaurare nu are un câmp pentru asta, deci se poate ajunge acolo doar printr-un apel API direct. O restaurare pe aceeași instanță scrie fiecare zvol înapoi peste el însuși.

**Exportul ca fișiere lasă zvol-urile deoparte.** Un VM ale cărui discuri sunt toate zvol-uri este refuzat cu acest motiv, deci restaurează-l dintr-un backup. Pentru un VM cu și discuri fișier, exportul conține discurile fișier și o notă care numește zvol-urile lăsate deoparte.

Asta a fost încercat pe TrueNAS SCALE 25.10 cu ZFS 2.3.4: trei backupuri ale unui zvol de probă, o restaurare pe loc după ce zvol-ul fusese suprascris și mărit și o restaurare pe un zvol care fusese șters. Fiecare restaurare a fost citită înapoi cu același SHA-256 ca sursa, zvol-ul a avut din nou dimensiunea veche și nu a rămas niciun instantaneu sau nicio clonă. Un VM întreg al cărui unic disc este un zvol a fost salvat în execuție, cu Controlat și cu Live, și oprit, iar acolo a rulat și o restaurare a unui VM întreg (undefine, scrierea zvol-urilor, define, pornire): zvol-ul a fost citit înapoi cu suma de control a sursei, iar VM-ul a pornit.

Discurile fișier, așa cum le folosește Unraid, nu sunt afectate de nimic din toate acestea.

### 4. Starea vTPM {#truenas-vtpm}

Un VM poate avea un TPM virtual, de care au nevoie Secure Boot și oaspeții Windows 11. Starea lui trăiește în afara discului și a NVRAM-ului, deci un backup doar al acestora două ar readuce un oaspete cu un TPM nou, gol.

- **Un TPM passthrough**, un cip real dat oaspetelui, își numește calea în XML-ul domeniului. BombVault îi salvează starea lângă NVRAM prin SSH și o scrie înapoi înainte de `virsh define`, în regim best-effort ca NVRAM: o citire sau o scriere eșuată este scrisă în jurnal și nu face backupul sau restaurarea să eșueze.
- **Un vTPM emulat**, genul pe care îl configurează TrueNAS pentru oaspeții Windows 11 și Secure Boot, nu publică nicio cale de stare în XML-ul domeniului. BombVault raportează atunci „no TPM path found” în loc să ghicească o cale. Ține la îndemână cheia de recuperare a unui astfel de oaspete, de exemplu cheia lui BitLocker, înainte să-l restaurezi.
- **Nu poți atașa singur un TPM unui VM TrueNAS.** API-ul de dispozitive VM din TrueNAS SCALE 25.10 acceptă doar `CDROM`, `DISPLAY`, `NIC`, `PCI`, `RAW`, `DISK` și `USB` și refuză `TPM` ca tip necunoscut.

Tratarea TPM este testată doar prin teste unitare și nu a rulat pe un oaspete TrueNAS real cu vTPM. Un VM fără TPM, ca pe Unraid, se salvează și se restaurează exact ca înainte.

NVRAM-ul este captat ca de obicei. TrueNAS îl ține pe VM la `/var/db/system/vm/nvram/<id>_<name>_VARS.fd`, iar XML-ul domeniului numește acea cale.

### 5. Numele VM-urilor pe TrueNAS {#truenas-names}

Pe Unraid numele libvirt al unui VM este numele pe care l-ai ales. TrueNAS inventează numele libvirt și l-a schimbat de două ori:

| Versiune TrueNAS | Nume libvirt | Unde trăiește numele tău |
|---|---|---|
| 25.10 „Goldeye” | `{id}_{name}`, de exemplu `1_debian` | în numele libvirt, după `{id}_` |
| 26 | UUID-ul VM-ului, de exemplu `550e8400-e29b-41d4-a716-446655440000` | doar în `<title>` al XML-ului domeniului |

Pe o gazdă pe care BombVault o detectează ca TrueNAS sau pe care `PLATFORM=truenas` o numește, lista Copii de rezervă arată numele tău: `debian` în loc de `1_debian` pe 25.10 și `<title>` în loc de UUID pe 26, sau UUID-ul când nu există titlu. Este doar ce arată lista. Fiecare apel `virsh` și setările pe VM folosesc în continuare numele libvirt, deci backupul și restaurarea acționează mereu pe VM-ul corect. Pe TrueNAS 26 asta înseamnă că un VM pe care TrueNAS îl creează din nou cu un UUID nou apare ca un element nou, fără setările lui, chiar dacă numele arată la fel. Pe Unraid nu se schimbă nimic, nici pentru un VM pe care l-ai numit `10_Windows`.

Schema 25.10 a fost văzută pe un sistem TrueNAS SCALE 25.10 real. Schema TrueNAS 26 vine din documentația TrueNAS și este testată doar prin teste unitare.

## Depanare {#troubleshooting}

| Simptom | Cauză și remediere |
|---|---|
| **Testează conexiunea** se blochează, apoi eșuează | Gazda nu poate fi atinsă. Verifică pasul 3 (rețea, VLAN, firewall) și **Host SSH: Address** și **Port**. |
| `Permission denied (publickey)` deși cheia este în `authorized_keys` | `StrictModes` din sshd respinge fișierul cu chei pentru că un folder părinte are proprietarul sau modul greșit. Este frecvent când `/root/.ssh` este un link către unitatea flash FAT (`/boot/config/ssh/...`), care poate fi scrisă de oricine. Jurnalul gazdei arată `Authentication refused: bad ownership or modes for directory ...`. Adaugă `StrictModes no` în configurația sshd a gazdei, în secțiunea globală, înainte de orice bloc `Match`, apoi rulează `/etc/rc.d/rc.sshd restart`. Unraid scrie `/etc/ssh/sshd_config` din nou la fiecare pornire, deci fă-l persistent așa cum e descris mai jos. |
| `Permission denied (publickey)` și cheia chiar lipsește | Repetă pasul 2 și verifică dacă SSH este pornit. |
| `Host key verification failed` | Rulează `docker exec BombVault rm -f /config/ssh/known_hosts` și testează din nou. |
| Verificarea de la pasul 3 spune `UNREACHABLE` | Containerul nu poate ajunge la portul SSH al gazdei. Vezi pasul 3: accesul gazdei la rețele personalizate, rutarea VLAN sau IP-ul shim ori LAN. |
| Variabilele lipsesc la **Edit** | Importă din nou șablonul. Un container existent păstrează vechea configurație salvată. |
| Un VM cu instantanee Unraid eșuează și numește un fișier | Lanțul de bază al discului nu poate fi urmărit până la capăt. Verifică în VM Manager dacă fiecare instantaneu al acelui disc este încă acolo și dacă fișierul numit există sub `/mnt`. |

Mai multe simptome VM se găsesc la [Depanare](troubleshooting.md#vm-backup-will-not-connect).

## Păstrarea după reporniri {#persistence}

Unraid scrie `/etc/ssh/sshd_config` din nou la fiecare pornire, deci schimbările de pe partea gazdei trebuie refăcute la pornire:

- **Portul SSH** este păstrat în `/boot/config/ident.cfg`, setat prin interfața web a Unraid.
- **`StrictModes no`**, dacă ai avut nevoie de el mai sus, nu este păstrat. Aplică-l la pornire din `/boot/config/go`:

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

- Cheia SSH proprie a BombVault și `~/.ssh/config` trăiesc în `/config` (appdata) și sunt create din nou la fiecare pornire, deci supraviețuiesc repornirilor containerului și ale gazdei.

## Securitate {#security}

Cheia autorizată dă acces root la gazdă, același nivel de încredere ca socketul Docker pe care BombVault îl folosește deja. Ține BombVault într-o rețea de încredere. Cheia este ed25519, stocată cu modul `0600` în appdata, iar cheia gazdei este fixată. Fiecare comandă către gazdă este transmisă ca argumente separate, niciodată printr-un shell la distanță, iar conexiunea SSH folosește `BatchMode` și un timp limită de conectare.
