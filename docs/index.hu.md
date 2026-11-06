# BombVault

**Az Unraid-adataid széfbe zárva. Dobj be egy mentést. Robbantsd be a visszaállítást.**

![Az áttekintés: az állapotsor, ami rád vár, az egyes szakaszok védettsége, a mentési célok és az előzmények.](assets/screenshots/overview/home-dark.png#only-dark)
![Az áttekintés: az állapotsor, ami rád vár, az egyes szakaszok védettsége, a mentési célok és az előzmények.](assets/screenshots/overview/home-light.png#only-light)

A BombVault egy saját üzemeltetésű webalkalmazás **biztonsági mentéshez és teljes vészhelyreállításhoz** Unraidon és más Docker-hosztokon, például TrueNAS SCALE-en, ZimaOS-en vagy egy sima Linux gépen. Menti a Docker-konténereket, a KVM/libvirt VM-eket, a mappákat, a ZFS-adatkészleteket és az Unraid flash meghajtóját. Egyetlen többarchitektúrás Docker-konténerként fut, webes felületet ad sötét, világos vagy a rendszerhez igazodó témával, és kezeli a teljes életciklust: mentés, ütemezés, ellenőrzés és visszaállítás.

A visszaállítás automatikus. Unraidon a konténerek pontosan úgy jelennek meg újra a Docker fülön, mint korábban, a VM-ek pedig újradefiniálódnak a VM Managerben, a lemezeikkel és az UEFI NVRAM-jukkal újracsatolva. Nincs kézi újratelepítés, nincs újrakonfigurálás, nincs dráma. TrueNAS-on az eredeti mellé visszaállított konténer sima Docker-konténerként fut, amelyet az Apps képernyő nem listáz.

A [restic](https://restic.net) hajtja, így minden mentés deduplikált, inkrementális és alapból titkosított.

!!! note "Óvd meg az APP_KEY-t"
    A BombVault a restic tároló jelszavát egy `APP_KEY` nevű, 32 bájtos titokból származtatja. Ha elveszíted a helyreállítási csomaggal együtt, a titkosított mentések visszaállíthatatlanná válnak. Generálj egyet az `openssl rand -hex 32` paranccsal, és tárold biztonságos helyen. Lásd: [Konfiguráció](configuration.md).

## Mit véd a BombVault {#what-bombvault-protects}

Minden konténer, VM, mappakészlet, a flash, minden ZFS-adatkészlet és a BombVault saját beállításai egy-egy **elem**, saját oldallal. Az azonos fajtájú elemek egy **szakaszt** alkotnak, amelyet a **Beállítások, Általános** alatt kapcsolsz be.

| Szakasz | Mi kerül mentésre |
|---|---|
| **Konténerek** | Az appdata könyvtár, valamint a konténer definíciója (image, környezeti változók, portok, címkék, kötetek). |
| **VM-ek** | A VM lemezképe(i), az XML-definíció és az UEFI NVRAM, SSH-n keresztül mentve (nincs libvirt-csatolás). |
| **Flash** | A teljes USB flash (`/boot`): operációs rendszer, licenc, tömbkonfiguráció, megosztások, hálózati és bővítmény-konfiguráció. |
| **Önmentés** | A BombVault saját `/config` mappája: a beállítás-adatbázisa, a mentési célok bejelentkezései, a libvirt SSH-kulcspár, a ZFS-szerverek kulcsa és rögzített hosztkulcsai, valamint a tanúsítványok. |
| **Mappák** | A szerver bármely mappája mappakészletként, mindegyik opcionális kizárási mintákkal. |
| **ZFS-adatkészletek** | Egy adatkészlet az összes alatta lévővel, egy ZFS pillanatképből olvasva és mappaként tárolva. Lásd: [ZFS-adatkészletek](zfs-datasets.md). |

## A visszaállítás a főszereplő {#restore-is-the-star}

Miután visszamásolta az adatokat a restic mentésből, a BombVault visszajátssza a mentett konténerdefiníciót a Docker API felé, így a konténer pontosan úgy jelenik meg újra az Unraid Docker fülén, mintha mindig is ott lett volna (ugyanaz az image, ugyanazok a beállítások, ugyanazok a port-hozzárendelések). A VM-ek XML-je SSH-n keresztül újradefiniálódik, a lemezeik és az UEFI NVRAM-juk pedig újracsatolódik, még akkor is, ha a VM-et törölték.

Egyetlen ablak állít vissza mindenfajta elemet. Alapból az **Az eredeti mellé** van kiválasztva, amely a mentést másolatként állítja vissza, az eredetit pedig érintetlenül hagyja. Az **Eredeti cseréje** előbb lementi a jelenlegi állapotot, így a visszaállítás visszavonható. A további célok egy mappa vagy egy új ZFS-adatkészlet.

Amikor egy mentés leállítja a függő konténereket, azok a megfelelő sorrendben térnek vissza: a BombVault a Compose `depends_on` sorrendjükben indítja újra őket, és megvárja, amíg mindegyik egészségesnek jelenti magát, mielőtt elindítaná a rá épülőket, így semmi sem előzi meg egy még nem elérhető adatbázist vagy átjárót. Lásd: [Visszaállítás](restore.md) és [Parancsok és más konténerek](backups.md#commands).

## Hogyan működik {#how-it-works}

```
Browser --HTTPS--> BombVault container
                   |- Go binary: JSON API + embedded React UI
                   |- Background worker (per-section scheduler + job executor)
                   |
                   |- /var/run/docker.sock  -> Docker API (container stop/inspect/recreate)
                   |- qemu+ssh://host       -> libvirt / KVM on the HOST over SSH (no mount)
                   |- /mnt/ -> /host/user   -> appdata, VM disks + restic repos (read/write)
                   |- /boot/ -> /host/boot  -> Unraid flash backup (whole USB)
                   |- /config               -> BombVault's own settings + sign-ins (self-backup)
                   '- <repo path>           -> restic repository (local or remote: rclone/s3/rest/sftp)
```

A BombVault a Docker socketen keresztül állítja le a konténereket mentés előtt, és hozza létre őket újra visszaállítás után. A VM-ekhez SSH-n keresztül (`qemu+ssh://`) futtatja a `virsh` parancsot a hoszton, hogy szabályosan leállítsa a VM-et, vagy élő pillanatképet készítsen. Soha nem csatol be (bind mount) libvirt-útvonalat, így nem zavarhatja a hoszt VM Managerét.

A BombVault az orkesztrációs és felhasználói felületi réteg, nem a tárolómotor. Minden tényleges adatmozgatás a resticen keresztül történik.

## Gyorsindítás {#quick-start}

Új vagy itt? Ugorj a **[Kezdő lépések](getting-started.md)** oldalra, hogy telepítsd a BombVaultot, és lefuttasd az első mentésedet, majd állítsd be a **[Telephelyen kívüli mentést és helyreállítást](offsite-recovery.md)**. A **[részletes példák](guides.md)** a gyakori feladatokat járják végig az elejétől a végéig, a **[Beállítások](settings.md)** oldal minden csempét felsorol, a **[Funkciók](features.md)** pedig a rövid katalógus arról, mit tud a BombVault.

Minden szakasz egyszerre több mentési célra is másolhat, egy párosított BombVault pedig egy olyan szerveren tarthatja a telephelyen kívüli másolataidat, amely megtagadja a törlést, a **Vészvarázsló** pedig a helyreállítási csomagjából hozza vissza az elveszett szervert, a csoportodon keresztül vagy a begépelt kulcsával. A konfigurációdat egy új gépre a **Beállítások exportálása / importálása** kártyával viheted át. Lásd: [Telephelyen kívüli mentés és helyreállítás](offsite-recovery.md) és [Konfiguráció](configuration.md#portable-settings-export-and-import).

Az **[Android-alkalmazás](android.md)** a csoportod összes szerverét a telefonodra hozza, mindegyikük tevékenységnaplójával egyetlen képernyőn.

A dokumentáció 26 nyelven érhető el, a felület 42 nyelven. Azok az oldalak, amelyeknek még nincs fordítása, angolul jelennek meg.

## Az itt használt szavak {#glossary}

Néhány szónak rögzített jelentése van a BombVaultban és ezeken az oldalakon.

| Szó | Jelentés |
|---|---|
| **Elem** | Egy dolog, amelyet a BombVault ment: egy konténer, egy VM, egy mappakészlet, a flash meghajtó, egy ZFS-adatkészlet az alatta lévőkkel, vagy a saját beállításai. Mindegyiknek saját oldala van. |
| **Szakasz** | Az azonos fajtájú elemek összessége, például a Konténerek. A szakaszokat a **Beállítások, Általános** alatt kapcsolod be. |
| **Mentési cél** | Egy hely, ahová a mentések kerülnek: egy mappa ezen a szerveren, egy megosztás, egy saját szerver vagy egy felhős szolgáltatás. Lásd: [Mentési célok](offsite-recovery.md#destinations). |
| **Helyi** | A mentési cél ezen a szerveren, amely a szakaszok mappáit tartja. |
| **Saját mappa** | A tároló, amelybe egy szakasz először írja a mentéseit. Alapból ezen a szerveren van, és lehet egy mentési célon is. Lásd: [Egy szakasz saját mappája](offsite-recovery.md#own-folder). |
| **Másolat** | Egy mentés, amelyet a saját mappából egy másik mentési célra másoltak a `restic copy` paranccsal. |
| **Közvetlen tároló** | Egy telephelyen kívüli tároló, amelybe az elemek közvetlenül mentenek, helyi másolat közbeiktatása nélkül. Lásd: [Közvetlen tárolók](offsite-recovery.md#direct-repositories). |
| **Bejelentkezés** | Az S3-szolgáltatáshoz vagy rest-serverhez mentett kulcs vagy jelszó. Az egyik az alapértelmezett. Lásd: [Bejelentkezések](offsite-recovery.md#sign-ins). |
| **Elhelyezés** | Hogy mely mentési célok kapják egy elem mentéseit, az elemen vagy a szakasz alapértékeként beállítva. Lásd: [Elhelyezés elemenként](offsite-recovery.md#placement). |
| **Replika** | Egy ZFS-elem blokkpontos másolata egy második ZFS-szerveren, `zfs send` paranccsal készítve. A mentés mellett áll, és nem helyettesíti azt. Lásd: [Replika](zfs-datasets.md#replica). |
| **Szerep** | Valami, amit a csoportod egyik példánya tesz egy másikért: Fogadó, Lehívó, ZFS-kiszolgáló vagy Kulcsőrző. Lásd: [Példányok és szerepek](offsite-recovery.md#roles). |

## Köszönetnyilvánítás {#credits}

- A **[VolumeVault](https://github.com/Darkdragon14/VolumeVault)** ([@Darkdragon14](https://github.com/Darkdragon14), Apache-2.0) adta a BombVault kiinduló ötletét: a Docker-konténerek egykattintásos mentését és automatikus újratelepítését. A BombVault egy különálló, Go-ra és resticre épülő megvalósítás, amely ezt az ötletet a VM-ekre, a flashre és egyebekre is kiterjeszti.
- A **[restic](https://restic.net/)** az a gyors, biztonságos, deduplikáló mentőmotor, amelyet a BombVault vezérel.
- Az **[rclone](https://rclone.org/)** biztosítja a felhős backendeket.
- A gombokon lévő jelek többsége a **[Streamline](https://streamlinehq.com)** ingyenes Core Solid készletéből származik ([CC BY 4.0](https://creativecommons.org/licenses/by/4.0/), [forrás](https://github.com/webalys-hq/streamline-vectors)). A többi a Font Awesome Free, a Material Design Icons, a Simple Icons és a Tabler Icons készletből való, vagy a projekthez rajzolták.

## Licenc {#license}

Copyright (C) 2026 Junker der Provinz. A BombVault szabad szoftver, a **GNU Affero General Public License v3.0** alatt ([LICENSE](https://github.com/junkerderprovinz/bombvault/blob/main/LICENSE)). Futtathatod, tanulmányozhatod, megoszthatod és módosíthatod. Ha terjeszted, vagy egy módosított változatát hálózati szolgáltatásként futtatod, a forráskódodat ugyanazon licenc alatt közzé kell tenned, és meg kell őrizned a meglévő szerzői jogi és szerzőségi megjegyzéseket.

A név és az arculat nem tartozik a licenc alá. Az AGPL csak a forráskódra vonatkozik: a „BombVault” név, a logója és az arculata fenntartott marad, így egy forknak saját nevet és arculatot kell használnia, és nem adhatja ki magát BombVaultnak.

## Hivatkozások {#links}

- **Forráskód:** [github.com/junkerderprovinz/bombvault](https://github.com/junkerderprovinz/bombvault)
- **Unraid támogatói téma:** [forums.unraid.net](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/)
- **Hibajegyek:** [github.com/junkerderprovinz/bombvault/issues](https://github.com/junkerderprovinz/bombvault/issues)

!!! warning "Root-szintű vezérlés a hoszt felett"
    A Docker socketen keresztül a BombVault le tud állítani, el tud távolítani és újra létre tud hozni konténereket, valamint olvasni és írni tudja az appdatát, a VM-mentéshez pedig SSH-n keresztül bejelentkezik a hosztra, hogy futtassa a `virsh` parancsot. Aki eléri a webes felületét, annak gyakorlatilag root-jogosultsága van a hoszton. A BombVaultot csak megbízható, nem kitett hálózaton futtasd, és állíts be bejelentkezési jelszót a **Beállítások, Biztonság** alatt. A helyreállítási csomaghoz, a kulcsletétekhez, a flash ZIP-hez és egy flash-mentés vagy önmentés exportálásához kell egy. A teljes biztonsági modellhez lásd: [Konfiguráció](configuration.md#security-model).
