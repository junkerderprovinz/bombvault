# VM-mentések

A BombVault a KVM/libvirt VM-eket úgy menti, hogy semmilyen libvirt-útvonalat nem csatol. A `virsh` parancsot a hoszton futtatja SSH-n át (`qemu+ssh://`), a VM-lemezeket a Host Data csatoláson keresztül olvassa és írja (a hoszton a `/mnt` a konténerben `/host/user`), az UEFI NVRAM-ot pedig SSH-n át másolja. Mivel semmi, ami a libvirthez tartozik, nincs bind-mountolva, a BombVault soha nem kerülhet az Unraid VM Manager útjába.

Ez az útmutató a beállítást, a mentési módszereket, az Unraid-pillanatképes VM-eket és a TrueNAS SCALE-t tárgyalja. A konténermentésekhez ebből semmi nem kell.

A ZFS-adatkészlet-mentések ugyanazt az SSH-kapcsolatot használják, ugyanazzal a kulccsal, hoszttal és felhasználóval. Amint a VM-mentések csatlakoznak, a ZFS-adatkészlet-elemek is csatlakoznak. Lásd: [ZFS-adatkészletek](zfs-datasets.md).

## Mit kell beállítanod {#what-you-configure}

| Beállítás | Hol | Alapérték | Jelentés |
|---|---|---|---|
| **Host SSH: Address** (`LIBVIRT_HOST`) | sablonváltozó | `host.docker.internal` | Az SSH-n át elért Unraid hoszt |
| **Host SSH: Port** (`LIBVIRT_SSH_PORT`) | sablonváltozó | `22` | Az Unraid SSH-portja |
| **Host SSH: User** (`LIBVIRT_SSH_USER`) | sablonváltozó | `root` | Az SSH-felhasználó a hoszton |
| Nyilvános kulcs | **Beállítások, Integrációk, Gazdagép SSH** | az első indításkor jön létre | Engedélyezd a hoszton |

A BombVault az első indításkor létrehozza az SSH-kulcspárját a `/config/ssh/id_ed25519` helyen, az appdatában tartva. A hoszt kulcsát az első kapcsolódáskor rögzíti a `/config/ssh/known_hosts` fájlban.

A sablon a `192.168.x.x` helykitöltővel tölti ki a **Host SSH: Address** mezőt. A BombVault ezt nem beállítottnak veszi, és a `host.docker.internal` címet próbálja, amely az alapértelmezett `bridge` hálózaton működik. Egyéni hálózaton cseréld a hoszt LAN IP-jére (3. lépés, B).

## 1. lépés: Az SSH engedélyezése az Unraid hoszton {#step-1}

1. Az Unraidben nyisd meg a **Settings, Management Access** oldalt, és állítsd a **Use SSH** értékét **Yes**-re.
2. Ha megváltoztattad az SSH-portot, jegyezd fel. A **Host SSH: Port** értékének egyeznie kell vele.

## 2. lépés: A BombVault nyilvános kulcsának engedélyezése {#step-2}

Az Unraid `/root` mappája a memóriában él, és minden indításkor törlődik, ezért a kulcsnak a flash-en is rajta kell lennie. Futtasd ezt az Unraid terminálján:

```sh
K=$(docker exec BombVault cat /config/ssh/id_ed25519.pub)
mkdir -p /root/.ssh /boot/config/ssh && chmod 700 /root/.ssh
grep -qxF "$K" /root/.ssh/authorized_keys 2>/dev/null  || echo "$K" >> /root/.ssh/authorized_keys
grep -qxF "$K" /boot/config/ssh/root.pubkeys 2>/dev/null || echo "$K" >> /boot/config/ssh/root.pubkeys
chmod 600 /root/.ssh/authorized_keys
```

Az Unraid 6.9 és újabb minden indításkor átmásolja a `/boot/config/ssh/root.pubkeys` fájlt a root `authorized_keys` fájljába, így ez túléli az újraindításokat. Bármely `root` bejelentkezésre érvényes, akármilyen hosztnevet vagy portot használ a BombVault.

## 3. lépés: Hálózat {#step-3}

A BombVaultnak TCP-kapcsolatot kell nyitnia a konténerből a hoszt SSH-portjára. Válaszd azt az esetet, amelyik a te felállásodhoz illik.

### A. A BombVault az alapértelmezett bridge hálózaton {#bridge}

- Hagyd a **Host SSH: Address** értékét `host.docker.internal`-on vagy a sablon helykitöltőjén. A sablon hozzáadja a `--add-host=host.docker.internal:host-gateway` opciót, amely a docker0 átjáróra mutat, ami a hoszt.
- Állítsd a **Host SSH: Port** értékét az SSH-portodra.

### B. A BombVault egyéni hálózaton {#custom-network}

A `br0.x`, macvlan vagy ipvlan hálózaton lévő konténer nem éri el a hosztot a `host.docker.internal` néven, mert a 172.17.0.1 a docker0, és a `br0.x`-ről nem érhető el. Ehelyett:

1. Az Unraidben állítsd a **Settings, Docker, Host access to custom networks** értékét **Enabled**-re.
2. Állítsd a **Host SSH: Address** értékét az Unraid hoszt LAN IP-jére, arra a címre, amelyen a webes felületet megnyitod.
3. Ha a konténer hálózata és a hoszt különböző VLAN-on van, engedélyezd az útvonalat a routeren vagy a tűzfalon: a konténer VLAN-jától a hoszt IP-jére az SSH-porton, TCP-n.
4. Ha a hoszt LAN IP-je még mindig nem érhető el, használd a hoszt shim IP-jét a konténer alhálózatán. A hoszton ezt az `ip -4 addr show | grep -B2 <container-subnet>` paranccsal találod meg.

### Ellenőrizd, hogy a hoszt válaszol {#verify-reachability}

Futtasd ezt az Unraid terminálján, a saját IP-ddel és portoddal:

```sh
docker exec BombVault timeout 6 bash -c 'echo > /dev/tcp/192.168.1.10/22' && echo OPEN || echo UNREACHABLE
```

Az `OPEN` azt jelenti, hogy az út működik. Az `UNREACHABLE` azt jelenti, hogy a 3. lépés hálózatát ki kell javítani, mielőtt bármi mást csinálsz.

## 4. lépés: A változók beállítása és tesztelés {#step-4}

1. Az Unraidben nyisd meg a **Docker** oldalt, kattints a BombVaultra, válaszd az **Edit** lehetőséget, állítsd be a **Host SSH: Address** értékét (és a **Host SSH: Port** értékét, ha nem 22), majd nyomd meg az **Apply** gombot. Ha a változók hiányoznak, importáld újra a sablont, mert az Unraid megtartja egy meglévő konténer elmentett konfigurációját.
2. A BombVaultban nyisd meg a **Beállítások, Integrációk, Gazdagép SSH** oldalt, és nyomd meg a **Kapcsolat tesztelése** gombot. Zöldre kell váltania.

Ugyanez a hívás a terminálról kilistázza a VM-eidet:

```sh
docker exec BombVault virsh -c "qemu+ssh://root@192.168.1.10:22/system?keyfile=/config/ssh/id_ed25519&known_hosts=/config/ssh/known_hosts&known_hosts_verify=auto" list --all
```

## 5. lépés: A módszer VM-enként {#per-vm-method}

Minden VM-nek van egy **Módszer** beállítása a **VM** kártyán a Beállítások lapján (nyisd meg a VM-et a **Biztonsági mentések** alatt):

- A **Szabályos (leállítás)** az alapértelmezett. Leállítja a VM-et, lementi a lemezeket, és újra elindítja, így a mentés mindig konzisztens. Legfeljebb tíz percet vár a vendég leállására, és soha nem kényszeríti ki. Az a vendég, amely ezután is fut, békén marad, és az a mentés az okával meghiúsul. Ha a vendég a következő órán belül leáll, a BombVault újra elindítja. Ha ez az indítás meghiúsul, a sikertelen futás ezt jelzi.
- Az **Élő pillanatkép** egy futó VM-et ment le egy külső pillanatképen és `blockcommit` használatával. Szüksége van arra, hogy a lemez a `/mnt/cache` vagy `/mnt/diskX` alatt legyen, ne a `/mnt/user` alatt. A VM-ben lévő qemu guest agenttel a pillanatkép nyugalmi, nélküle összeomlás-konzisztens. Egy leállított VM-nél az Élő magától Szabályosra vált. Ha a pillanatképet nem lehet elkészíteni, az a mentés meghiúsul, és a VM tovább fut, mert egy élő mentésre választott VM-et emiatt soha nem állít le.

## Amit a mentés kihagy {#left-out}

Egy mentés úgy is sikerülhet, hogy nem tartalmazza a teljes VM-et. A futás ekkor figyelmeztető hangvételű megjegyzést kap, amely megnevezi, mi hiányzik:

- **Lemezek, amelyeket a BombVault nem olvas.** A csak olvasható lemez, a `lun`, valamint a `network` vagy `volume` típusú lemez kimarad, és a megjegyzés mindegyiket megnevezi a céleszközével és a fajtájával. A mentésben lévő definíció továbbra is felsorolja őket. A cdromot és a floppyt nem nevezi meg. Az a sima blokkeszköz, amely nem ZFS zvol, ehelyett elbuktatja a mentést.
- **Az emulált TPM állapota.** Nincs útvonala a domain XML-ben. Az Unraid VM Manager az **OVMF TPM** BIOS-szal rendelkező VM-eknek ilyen TPM-et állít be, így a megjegyzés Unraidon is megjelenik. Az a vendég, amely titkot tart a TPM-jében, ahogy a BitLocker is teheti, egy ezt az állapotot elvesztő visszaállítás után kérheti a helyreállítási kulcsát, ezért tartsd kéznél azt a kulcsot.

## Vékony lemezek {#thin-disks}

A visszaállítás a vékony lemezt vékonyan tartja, akár lecseréli a lemezt, akár újat ír: a restic a nullás szakaszokat lyukakként írja egy új fájlba, a BombVault pedig egy lecserélt fájl nullás blokkjait a megírása után a `fallocate --dig-holes` segítségével adja vissza. Amíg a restic lecserél egy fájlt, az a fájl a teljes méretét foglalja, ha a poolon van rá hely. A lyukasztást (hole punching) nem ismerő fájlrendszer a teljes méretén hagyja a lemezt, és a napló ezt jelzi. A helyellenőrzés egy új lemezt továbbra is a teljes méretével számol.

## Sikertelen visszaállítás {#restore-fails}

A helyben történő visszaállítás kikapcsolja a VM-et, megírja a lemezeket, és a definíciót meg az NVRAM-ot csak akkor cseréli le, amikor a lemezek már megvannak. A sikertelen visszaállítás a VM-et definiálva hagyja az NVRAM-jával együtt. Ha a lemezei nem változtak, és futott, újra fut. Ha változhattak, kikapcsolva marad, mert egy félig írt lemez indítása ráírna, és a futás ezt jelzi. Ugyanannak a mentésnek az újbóli visszaállítása befejezi a munkát.

A libvirtben nyilvántartott pillanatképek (az Unraid VM Manager nem így készíti a sajátjait) még a VM érintése előtt leállítják a helyben történő visszaállítást, mert a libvirt nem tud eltávolítani olyan VM-et, amelynek vannak ilyenek. Előbb töröld őket.

## Csak a változott blokkok {#changed-blocks}

A qcow2 lemezű VM úgy is menthető, hogy csak azt olvassa, ami megváltozott. Kapcsold be a **Csak a változott blokkok** beállítást ugyanazon a kártyán, és a BombVault a libvirt backup API-ját használja a fenti módszer helyett:

- Elindít egy pull módú mentési feladatot (`virsh backup-begin`), amely ugyanabban a pillanatban létrehoz egy `bombvault-<time>` nevű ellenőrzőpontot, és a lemezeket az SSH-kapcsolaton át, egy továbbított Unix-socketen olvassa. Ha a vendég agentje válaszol, a vendég fájlrendszerei csak a feladat indulása alatt vannak befagyasztva. A sikertelen felolvasztást körülbelül tizenöt másodpercig újra próbálja, és ha a vendég még mindig nem olvad fel, a BombVault értesítést küld.
- Az első futás minden lefoglalt blokkot beolvas. A későbbi futások csak azokat a blokkokat olvassák, amelyeket az ellenőrzőpont bitmapje az előző mentés óta írtnak jelöl.
- Minden lemez fix méretű szegmensek mappájaként tárolódik, azt tartalmazva, amit a vendég lát. A változatlan szegmenseket az előző pillanatképből olvasás nélkül veszi át, így minden pillanatkép teljes, és önmagában visszaállítható.
- A visszaállítás a szegmensekből újraépíti a lemezt, és a `qemu-img` segítségével visszaalakítja qcow2-vé. A lemez tartalma ugyanaz, mint a mentés idején, de a fájl új qcow2 fájl, és egy háttérlánc laposítva tér vissza.
- Az ilyen visszaállítás előtti ellenőrzés felsorol minden lemezt, amelyet újraépít, és a vendég teljes méretével számolja, qcow2-nél kétszer, mert a nyers kép és az átalakított másolata egymás mellett áll, amíg a régi fájl le nem cserélődik. Ez a legtöbb, amit egy lemez elvihet, így az ezzel a számmal mért hiány a szabad hely sorát szürkén hagyja, nem blokkol. Az első mentés utáni visszaállítás-ellenőrzés és a **Visszaállítás ellenőrzése** arról is gondoskodik, hogy a pillanatkép minden lemez minden szegmensét a megfelelő méretben tartalmazza.
- A futások között a VM-nek nincs ellenőrzőpontja a libvirtben nyilvántartva. A BombVault a legújabb `bombvault-` ellenőrzőpont definícióját a `/config/vm-checkpoints` alatt tartja, és csak a libvirt róla szóló nyilvántartását távolítja el (`virsh checkpoint-delete --metadata`). A bitmapje a qcow2 fájlban marad, és tovább számolja az írásokat, a következő futás pedig az indulása előtt visszateszi az ellenőrzőpontot (`virsh checkpoint-create --redefine`). A libvirt megtagadja egy olyan VM definíciójának törlését, amelynek vannak ellenőrzőpontjai, így ettől működik a szokott módon az Unraid **Remove VM** és egy visszaállítás.
- Egy mentési feladatot, amelyet a BombVault futva hagyott, mert egy mentés közepén leállt, a BombVault újraindulásakor lezárja, azokkal az ellenőrzőpontokkal együtt, amelyeket az a futás nyilvántartva hagyott.
- A kapcsoló kikapcsolása, a VM kivétele az ütemezésből vagy a mentéseinek törlése szintén lezár egy ilyen feladatot, és törli a bitmapet is, amíg a VM fut. Egy leállított VM-nél a libvirt nem tudja törölni, és az használatlanul a képben marad.
- Más eszközök ellenőrzőpontjait békén hagyja. Az a VM, amelynek vannak ilyenjei, nem állítható vissza, amíg nem törlöd őket, és a visszaállítás megáll, mielőtt a VM-hez nyúlna.

A kapcsoló magától visszavált. Egy hiányzó vagy hibás ellenőrzőpont, egy lemez, amely méretet váltott, vagy egy legújabb pillanatkép, amely nem változott-blokkos, azt jelenti, hogy a teljes lemezt még egyszer beolvassa. Egy leállított VM, egy nyers vagy zvol lemez, hiányzó hoszt SSH-kapcsolat, egy backup API nélküli libvirt vagy a `/config` alatti túl kevés szabad hely azt jelenti, hogy a szokásos mentés fut a fenti módszerrel. A kártya megmondja, ezek közül az utolsó futás melyiket használta és miért.

Követelmények: libvirt a backup API-val (tesztelve libvirt 12.2 és QEMU 10.2 mellett Unraid 7.3-on), `AllowStreamLocalForwarding yes` beállítású sshd (az alapérték, az Unraidon is, amely a TCP-továbbítást kikapcsolja), és qcow2 lemezek. A libvirt visszautasítja az ellenőrzőpontokat nyers lemezeken, az Unraid pedig alapból nyers vdiskeket hoz létre. A VM a mentés alatt fut tovább, mint az Élőnél, és a restic-re váró szegmensek legfeljebb 4 GiB-ot foglalnak a `/config` alatt.

## Unraid-pillanatképes VM-ek {#unraid-snapshots}

Az a pillanatkép, amelyet az Unraid VM Manager készít, egy qcow2 lemezt lánccá alakít: alul az eredeti kép, rajta pillanatképenként egy fájl, mindegyik csak azt tartalmazza, ami az alatta lévő óta változott. A VM a legújabb fájlba ír. Egy két pillanatképes lemez három fájl.

A BombVault az ilyen VM-et teljes egészében menti:

- **A lánc minden fájlja.** Követi minden lemez háttérláncát a végéig, és lementi a benne lévő összes fájlt, az Unraid pillanatkép-listájával együtt (`/etc/libvirt/qemu/snapshotdb/<vm>/snapshots.db`, SSH-n át olvasva). Az a lánc, amelyet nem lehet a végéig követni, például mert egy fájl hiányzik a közepén, elbuktatja a mentést, és megnevezi a fájlt.
- **Az élő mentés megtartja a pillanatképeket.** Az **Élő pillanatkép** egy ideiglenes overlayt tesz a tetejére, amíg a lemezt olvassa, és utána visszaolvasztja, csak a közvetlenül alatta lévő képbe. A lánc a mentés után ugyanaz, mint előtte. A pillanatkép nélküli VM úgy viselkedik, mint mindig.
- **A visszaállítás visszahozza a láncot.** A helyben történő visszaállítás minden fájlt oda ír vissza, ahol volt, és visszateszi az Unraid pillanatkép-listáját, így a VM Manager újra listázza a pillanatképeket. Az eredeti mellé történő visszaállítás a fájlokat a másolat mappájába írja, és a visszaállított lemezeket a visszaállított fájlokra irányítja, így a másolat soha nem olvas az eredeti képeiből. Az Unraid pillanatkép-listája csak a helyben történő visszaállítással kerül vissza, mert azokat a fájlokat nevezi meg, ahol a mentéskor voltak. Nélküle a másolat ugyanúgy elindul, és a VM Manager egyszerűen nem listáz hozzá pillanatképeket.
- A **Csak a változott blokkok** azt olvassa, amit a vendég lát, ezért a visszaállítása egyetlen laposított qcow2 fájlként jön vissza a pillanatképek nélkül.
- **A visszaállítás a kiválasztott mentés lemezét hozza vissza.** Az azóta készült vagy összeolvasztott pillanatképek ezen nem változtatnak. Egy pillanatkép előtti mentés a benne lévő fájlra definiálja a VM-et, egy összeolvasztás előtti mentés pedig visszahozza az overlayét, és azon fut. Az újabb fájlok ott maradnak, ahol vannak, használatlanul és változatlanul. Az a mentés, amely tartalmaz egy overlayt, de az alatta lévő fájlt nem, a VM érintése előtt elutasításra kerül. A VM definíciója a visszaállítás után nem sorol fel láncot, így a libvirt a visszaállított fájlokból olvassa ki a láncot.

### Példa: egy VM két pillanatképpel {#example-unraid-snapshots}

A `homeassistant` VM-nek egy lemeze van, a `vdisk1.qcow2`, és két pillanatképet készítettél a VM Managerben két frissítés előtt. A lemeze most három fájlból álló lánc. Egész nap fut, ezért **Élő pillanatképet** használ.

1. Nyisd meg a **Biztonsági mentések** oldalt, majd a `homeassistant` elemet, aztán a **Beállítások** lapját. A **VM** kártyán ellenőrizd, hogy a **Módszer** **Élő pillanatkép**, és a **Csak a változott blokkok** ki van kapcsolva.
2. Nyomd meg a **Biztonsági mentés most** gombot az oldal fejében.
3. Amikor a futás kész, a sora a **Legutóbbi futások** között az **Ellenőrzés** lapon zöld. A VM végig futott, és a mentés a lemez mindhárom fájlját és az Unraid pillanatkép-listáját tartalmazza.
4. A VM Managerben a két pillanatkép még ott van, és a lemez még ugyanazon a három fájlon át olvas.

Egy hónappal később egy frissítés rosszul sül el, és a tegnapi állapotot szeretnéd visszakapni a tönkrement mellé, összehasonlításhoz:

1. Nyisd meg a **Visszaállítás** oldalt, válaszd a **VM-ek** szakaszt, és kattints a `homeassistant` elemre.
2. Tartsd meg a tegnap éjszakai ajánlott mentést, és a **Hová** alatt válaszd az **Az eredeti mellé** lehetőséget.
3. Nyomd meg a **Visszaállítás** gombot. A három fájl egy, a VM nevéről `-restored` utótaggal elnevezett mappába kerül, és a visszaállított lemez a visszaállított fájlokra mutat. A futó VM nem változik.

Ha inkább az egész VM-et visszagörgetnéd, válaszd az **Eredeti cseréje** lehetőséget bekapcsolt **Előbb a jelenlegi állapot mentése** mellett. A BombVault lementi a tönkrement állapotot, leállítja a VM-et, visszaírja a három fájlt és az Unraid pillanatkép-listáját, és újra elindítja.

## TrueNAS SCALE {#truenas-scale}

A fenti lépések Unraidre vannak írva, de ugyanez a `qemu+ssh://` kapcsolat működik bármely libvirtd ellen, amelyet a konténer elér, a TrueNAS SCALE-t is beleértve, amelynek a 25.04.2 óta újra libvirten vannak a VM-ei. TrueNAS-on a kapcsolati karakterláncot felül kell bírálni, és a kulcs a TrueNAS saját felhasználói beállításaiban kerül a roothoz. Három további dolog változtatja meg, mit tartalmaz egy mentés: a zvolokon lévő lemezek, a vTPM-állapot, és a nevek, amelyeket a TrueNAS a VM-eknek ad.

### 1. A libvirt socket {#truenas-socket}

A TrueNAS libvirtd-je nem ott figyel, ahol a **Host SSH** mezőkből összeállított kapcsolati karakterlánc várja. Állítsd be helyette a `LIBVIRT_URI`-t, a plusz `?socket=` paraméterrel:

```sh
LIBVIRT_URI=qemu+ssh://<user>@<truenas-host>/system?socket=/run/truenas_libvirt/libvirt-sock
```

A BombVault a `LIBVIRT_URI`-t pontosan úgy használja, ahogy írva van, és a `LIBVIRT_HOST`, `LIBVIRT_SSH_USER` és `LIBVIRT_SSH_PORT` nem kell. Mindegyikükhöz, amelyik nincs beállítva, a BombVault az URI-ból veszi a hosztot, felhasználót és portot a saját SSH-parancsaihoz is, így az NVRAM-másolat és a ZFS-adatkészlet-mentések ugyanazt a gépet érik el, mint a `virsh`. Az általad beállított változó mindig erősebb, és az indulási napló megnevezi a célt, amelyet az URI-ból vett. TrueNAS-on a rootól eltérő felhasználónak a ZFS-adatkészlet-mentésekhez a `zfs allow <user> snapshot,destroy,mount <dataset>` is kell, lásd: [ZFS-adatkészletek](zfs-datasets.md#truenas).

Írd az URI-ba a TrueNAS LAN-címét. Ha sem a `LIBVIRT_URI`, sem a `LIBVIRT_HOST` nincs beállítva, a BombVault az alkalmazás alapértelmezett átjáróját veszi hosztnak, mert a `host.docker.internal` nem oldható fel egy TrueNAS-alkalmazásban, és az indulási napló megnevezi a kiválasztott címet. Ez elég a ZFS-adatkészlet-mentésekhez. A VM-mentésekhez még kell az URI a sockethez.

Az alkalmazásnak a VM-mentésekhez kell a `SETPCAP` képesség, a `CHOWN`, `DAC_OVERRIDE`, `FOWNER`, `SETGID` és `SETUID` mellett. Nélküle a libvirt „cannot apply process capabilities” hibával megáll. A [TrueNAS SCALE](getting-started.md#truenas) alatti YAML mindet beállítja, a `SYS_CHROOT`-tal együtt, amelyre a pillanatkép leállítás helyett szükség van.

TrueNAS SCALE 25.10.0-n a socket útvonalát valódi hardveren ellenőrizték: az alapértelmezett `qemu:///system` a `Failed to connect socket to '/var/run/libvirt/libvirt-sock': No such file or directory` hibával elbukik, ugyanez a hívás a `?socket=/run/truenas_libvirt/libvirt-sock` résszel pedig kilistázza a domaineket.

### 2. A BombVault kulcsa a TrueNAS-on {#truenas-key}

A fenti 2. lépés az Unraid flash-ére ír, és itt nem alkalmazható. TrueNAS-on:

1. Kapcsold be az SSH szolgáltatást a **System, Services** alatt.
2. A **Credentials, Users** alatt szerkeszd a **root** felhasználót, és illeszd be a BombVault nyilvános kulcsát a **Beállítások, Integrációk, Gazdagép SSH** helyről az SSH nyilvános kulcsaként. A TrueNAS megtartja újraindításokon és frissítéseken át.

A TrueNAS 24.10 és újabb friss telepítései nem adnak jelszót a rootnak, és a kulcsos bejelentkezéshez nem is kell. Ha inkább nem használnál rootot, engedélyezd a kulcsot egy sudóval rendelkező admin fiókhoz, és írd a nevét a `root` helyére a `LIBVIRT_URI`-ban. A ZFS-adatkészlet-mentésekhez ilyenkor a fent leírt `zfs allow` is kell.

A második szerverre történő ZFS-replika saját kulccsal jelentkezik be, amelyet a replika párbeszédablaka mutat. A Gazdagép SSH kulcsa nem engedélyezi a replikát, és a replika kulcsa nem engedélyezi a VM-mentéseket.

!!! warning "A TrueNAS nem látja a virsh-sel végzett változtatásokat"
    A TrueNAS middleware-ének, a VM-képernyői mögötti rétegnek kellene lennie az egyetlen módnak, ahogy a VM-jei változnak. Egy `virsh` parancs ezen az SSH-kapcsolaton át, amit a BombVault VM-mentése és -visszaállítása futtat, megkerüli, és a TrueNAS később összeegyeztethet vagy felülírhat olyan állapotot, amelyet nem ő hozott létre. Ez a TrueNAS dokumentált működéséből következik, és valódi gépen még nem láttuk. Ugyanolyan kockázat, mint amikor az Unraid VM Managere egyezteti a rajta kívül végzett változtatásokat, csak TrueNAS-on élesebb.

### 3. Lemezek zvolokon {#truenas-zvols}

A TrueNAS VM-lemezei gyakran zvolok, ZFS blokkeszközök a `/dev/zvol/<pool>/<dataset>` helyen, nem qcow2 fájlok. A restic nem tud blokkeszközt az útvonala alapján menteni úgy, ahogy egy fájlt, ezért a BombVault felismeri a zvol lemezt a domain XML-ben (`<source dev="...">` a `<source file="...">` helyett), és ugyanazon az SSH-kapcsolaton kezeli. A BombVault konténernek nem kellenek ZFS-eszközök.

Egy zvol mentése:

1. A `zfs snapshot <dataset>@bombvault-vm-<time>` rögzít egy időpontot, a VM futása közben is.
2. A `zfs clone -o volmode=dev -o readonly=on` olvashatóvá teszi azt a pillanatképet eszközként. A `volmode=dev` megakadályozza, hogy a hoszt átvizsgálja a vendég partícióit.
3. A `dd` beolvassa az eszközt, és a nyers kép egyetlen fájlként kerül a resticbe, `/vm-disks/<dataset>.img` néven, `vm:<name>:zvol:<dev>` címkével. A restic tartalom szerint darabokra vágja, így az utolsó mentés óta nem változott blokkokat nem tárolja újra.
4. A klón és a pillanatkép utána megsemmisül, akár sikerült a mentés, akár nem. Ha a BombVault egy mentés közepén leállt, az adott zvol következő mentése előbb eltávolítja a `bombvault-vm-<time>` pillanatképet és a `<dataset>-bombvault-vm-<time>` klónt. Minden más pillanatképet békén hagy, és megtart egyet, amelyből valami mást klónoztak. A napló megnevezi, mit távolított el vagy tartott meg.

A BombVault ehhez nem használ `zfs send` parancsot. Egy send-folyam minden rekordja hordozza a pillanatkép GUID-ját és egy futó ellenőrzőösszeget, így a restic semmit nem találna az utolsó mentésből, amit újra felhasználhatna, és minden alkalommal újra eltárolná a teljes lemezt. TrueNAS SCALE 25.10-en mérve egy 512 MiB-os zvolon, amely 200 MiB adatot tartalmazott: az első mentés 211 MB-ot adott hozzá, a második változatlan adattal 52 KB-ot, az 1 MiB megváltozása utáni pedig 3,6 MB-ot. Ugyanez a zvol a `zfs send`-en át minden futáskor 214 MB-ot adott hozzá.

Minden futás ezen felül ír egy kis, `vm:<name>` címkéjű pillanatképet a domain XML-lel, miután az összes lemez bent van. Az a VM, amelynek minden lemeze zvol, nem rendelkezik fájl-pillanatképpel, és ez az, amely a futást a VM alatt listázza, és visszaállíthatóvá teszi.

A visszaállítás minden zvolt a helyére ír vissza, ahogy egy fájllemezt a fájljába, így a visszaállított VM a visszaállított adatokból indul:

1. Mielőtt bármihez hozzányúlna, a BombVault ellenőrzi, hogy a futás minden zvol lemezről tartalmaz-e képet. Az a futás, amelyből ilyen hiányzik, elutasításra kerül, és a VM úgy marad, ahogy van.
2. Egy meglévő zvol előbb kap egy pillanatképet, `<dataset>@bombvault-before-restore-<time>`, az idővel UTC-ben, mint minden BombVault által készített pillanatképnév, ezért órákkal eltérhet a helyi órától. A napló megnevezi, és a `zfs rollback` rá visszavonja a visszaállítást, amíg az a zvol legújabb pillanatképe. Amint van újabb, például a következő visszaállításé, a `zfs rollback -r` megteszi, és megsemmisíti az újabb pillanatképeket. Addig marad, amíg el nem távolítod a `zfs destroy` paranccsal, miután a visszaállított VM működik.
3. Az a zvol, amely már nem létezik, abban a méretben jön létre, amelyben a mentéskor volt, a meglévő, más méretű pedig visszaáll erre a méretre.
4. A `dd` a képet a `/dev/zvol/<dataset>` helyre írja. Bekapcsolt tömörítésnél, ami a TrueNAS alapértelmezése, a kép üres részei lyukakként térnek vissza, és nem foglalnak helyet.

**Egy másik példányra történő visszaállításhoz célpool kell.** A fájllemezeket egy célmappába helyezi, de egy mappa nem nevez meg ZFS-poolt, ezért nélküle a BombVault az indulás előtt elutasítja a visszaállítást. Add meg a poolt a `POST /api/foreign/restore` `zvolPool` paramétereként: minden zvol megtartja a pool alatti útvonalát (`tank/vms/win10/disk0` a `flashpool`-ra `flashpool/vms/win10/disk0` lesz), és a domain XML erre mutat. A Visszaállítás oldalon a **Régi mentési mappából** ablaknak nincs erre mezője, ezért csak közvetlen API-hívással érhető el. Ugyanazon a példányon a visszaállítás minden zvolt önmagára ír vissza.

**A fájlként exportálás kihagyja a zvolokat.** Az a VM, amelynek minden lemeze zvol, ezzel az indokkal elutasításra kerül, ezért állítsd vissza egy mentésből. Egy fájllemezekkel is rendelkező VM-nél az export a fájllemezeket tartalmazza és egy megjegyzést, amely megnevezi a kihagyott zvolokat.

Ezt kipróbálták TrueNAS SCALE 25.10-en ZFS 2.3.4-gyel: egy tesztzvol három mentése, egy helyben történő visszaállítás azután, hogy a zvolt felülírták és megnövelték, és egy visszaállítás egy törölt zvolra. Mindegyik visszaállítás a forrással azonos SHA-256 értékkel olvasódott vissza, a zvol újra a régi méretű volt, és nem maradt pillanatkép vagy klón. Egy egész VM-et, amelynek egyetlen lemeze zvol, lementették futva, Szabályos és Élő móddal, valamint leállítva, és egy teljes VM-visszaállítás (definíció törlése, a zvolok írása, definiálás, indítás) is lefutott ott: a zvol a forrás ellenőrzőösszegével olvasódott vissza, és a VM elindult.

A fájllemezeket, ahogy az Unraid használja őket, ebből semmi nem érinti.

### 4. A vTPM-állapot {#truenas-vtpm}

Egy VM-nek lehet virtuális TPM-je, amelyre a Secure Boot és a Windows 11 vendégek igényt tartanak. Az állapota a lemezen és az NVRAM-on kívül él, így csak e kettő mentése új, üres TPM-mel hozná vissza a vendéget.

- Egy **átengedett TPM**, egy valódi chip, amelyet a vendégnek adtak át, megnevezi az útvonalát a domain XML-ben. A BombVault az állapotát az NVRAM mellett menti SSH-n át, és a `virsh define` előtt visszaírja, legjobb szándékkal, mint az NVRAM-ot: a sikertelen olvasás vagy írás naplózódik, és nem buktatja el a mentést vagy a visszaállítást.
- Egy **emulált vTPM**, amilyet a TrueNAS a Windows 11 és a Secure Boot vendégeknek állít be, nem tesz közzé állapotútvonalat a domain XML-ben, ezért az állapota nem kerül mentésre. A BombVault nem találgat útvonalat, és a mentés egy megjegyzéssel zárul, hogy a VM TPM-jének állapota nincs benne. Tartsd kéznél egy ilyen vendég helyreállítási kulcsát, például a BitLocker-kulcsát, mielőtt visszaállítod.
- **Nem tudsz TPM-et csatolni egy TrueNAS VM-hez magad.** A TrueNAS SCALE 25.10 VM-eszköz API-ja csak a `CDROM`, `DISPLAY`, `NIC`, `PCI`, `RAW`, `DISK` és `USB` típust fogadja el, a `TPM`-et ismeretlen típusként utasítja vissza.

A TPM-kezelést csak egységtesztelték, valódi, vTPM-es TrueNAS-vendéggel még nem futott. A TPM nélküli VM pontosan a korábbiak szerint menthető és állítható vissza.

Az NVRAM-ot a szokott módon menti. A TrueNAS VM-enként a `/var/db/system/vm/nvram/<id>_<name>_VARS.fd` helyen tartja, és a domain XML megnevezi ezt az útvonalat.

### 5. VM-nevek a TrueNAS-on {#truenas-names}

Unraidon a VM libvirt-neve az a név, amelyet választottál. A TrueNAS kitalálja a libvirt-nevet, és kétszer változtatott rajta:

| TrueNAS-verzió | libvirt-név | Hol van a te neved |
|---|---|---|
| 25.10 „Goldeye” | `{id}_{name}`, például `1_debian` | a libvirt-névben, az `{id}_` mögött |
| 26 | a VM UUID-ja, például `550e8400-e29b-41d4-a716-446655440000` | csak a domain XML `<title>` elemében |

Egy olyan hoszton, amelyet a BombVault TrueNAS-nak ismer fel, vagy amelyet a `PLATFORM=truenas` megnevez, a Biztonsági mentések lista a te nevedet mutatja: `debian` az `1_debian` helyett a 25.10-en, és a `<title>` az UUID helyett a 26-on, vagy az UUID, ha nincs cím. Ez csak az, amit a lista mutat. Minden `virsh` hívás és a VM-enkénti beállítások továbbra is a libvirt-nevet használják, így a mentés és a visszaállítás mindig a megfelelő VM-en dolgozik. A TrueNAS 26-on ez azt jelenti, hogy egy VM, amelyet a TrueNAS új UUID-val újra létrehoz, új elemként jelenik meg a beállításai nélkül, még ha a neve ugyanúgy néz is ki. Unraidon semmi nem változik, a `10_Windows` nevű VM-nél sem.

A 25.10-es sémát egy valódi TrueNAS SCALE 25.10 gépen láttuk. A TrueNAS 26 sémája a TrueNAS dokumentációjából származik, és csak egységtesztelt.

## Hibaelhárítás {#troubleshooting}

| Tünet | Ok és javítás |
|---|---|
| A **Kapcsolat tesztelése** lefagy, majd meghiúsul | A hoszt nem érhető el. Ellenőrizd a 3. lépést (hálózat, VLAN, tűzfal), valamint a **Host SSH: Address** és **Port** értékét. |
| `Permission denied (publickey)`, bár a kulcs az `authorized_keys` fájlban van | Az sshd `StrictModes` beállítása elutasítja a kulcsfájlt, mert egy szülőmappa tulajdonosa vagy módja rossz. Ez gyakori, ha a `/root/.ssh` a FAT flash-re mutató hivatkozás (`/boot/config/ssh/...`), amely mindenki számára írható. A hoszt naplója ezt mutatja: `Authentication refused: bad ownership or modes for directory ...`. Add hozzá a `StrictModes no` sort a hoszt sshd-konfigurációjához a globális részben, bármely `Match` blokk előtt, majd futtasd a `/etc/rc.d/rc.sshd restart` parancsot. Az Unraid minden indításkor újraírja az `/etc/ssh/sshd_config` fájlt, ezért tartósítsd az alább leírtak szerint. |
| `Permission denied (publickey)`, és a kulcs tényleg hiányzik | Ismételd meg a 2. lépést, és ellenőrizd, hogy az SSH be van kapcsolva. |
| `Host key verification failed` | Futtasd a `docker exec BombVault rm -f /config/ssh/known_hosts` parancsot, és teszteld újra. |
| A 3. lépés ellenőrzése `UNREACHABLE`-t ír | A konténer nem éri el a hoszt SSH-portját. Lásd a 3. lépést: hoszthozzáférés egyéni hálózatokhoz, VLAN-útválasztás, vagy a shim vagy LAN IP. |
| A változók hiányoznak az **Edit** alatt | Importáld újra a sablont. Egy meglévő konténer megtartja a régi elmentett konfigurációját. |
| Egy Unraid-pillanatképes VM meghiúsul, és megnevez egy fájlt | A lemez háttérláncát nem lehet a végéig követni. Ellenőrizd a VM Managerben, hogy az adott lemez minden pillanatképe megvan, és hogy a megnevezett fájl létezik a `/mnt` alatt. |

További VM-tünetek a [Hibaelhárításban](troubleshooting.md#vm-backup-will-not-connect) vannak.

## Újraindítások átvészelése {#persistence}

Az Unraid minden indításkor újraírja az `/etc/ssh/sshd_config` fájlt, ezért a hoszt oldali változtatásokat indításkor újra el kell végezni:

- Az **SSH-port** a `/boot/config/ident.cfg` fájlban van, az Unraid webes felületén át beállítva.
- A **`StrictModes no`**, ha szükséged volt rá fent, nem marad meg. Alkalmazd indításkor a `/boot/config/go` fájlból:

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

- A BombVault saját SSH-kulcsa és `~/.ssh/config` fájlja a `/config` alatt (appdata) él, és minden indításkor újra létrejön, így túlélik a konténer és a hoszt újraindításait.

## Biztonság {#security}

Az engedélyezett kulcs root-jogot ad a hoszton, ugyanakkora bizalmat, mint a Docker socket, amelyet a BombVault már most is használ. Tartsd a BombVaultot megbízható hálózaton. A kulcs ed25519, `0600` móddal tárolva az appdatában, és a hoszt kulcsa rögzítve van. Minden hosztparancs külön argumentumként kerül átadásra, soha nem távoli shellen át, és az SSH-kapcsolat `BatchMode`-ot és csatlakozási időkorlátot használ.
