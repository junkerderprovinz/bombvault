# BombVault

**Az Unraid-adataid széfbe zárva. Dobj be egy mentést. Robbantsd be a visszaállítást.**

A BombVault egy saját üzemeltetésű, Unraid-natív webalkalmazás a Docker-konténereid és KVM/libvirt VM-jeid **biztonsági mentéséhez és teljes vészhelyreállításához**. Egyetlen többarchitektúrás Docker-konténerként fut, modern webes felületet ad, amely követi a rendszered világos/sötét témabeállítását, és kezeli a teljes életciklust: mentés, ütemezés, ellenőrzés és visszaállítás.

A visszaállítás automatikus. A konténerek pontosan úgy jelennek meg újra az Unraid Docker fülén, mint korábban, a VM-ek pedig újradefiniálódnak a VM Managerben, a lemezeikkel és az UEFI NVRAM-jukkal újracsatolva. Nincs kézi újratelepítés, nincs újrakonfigurálás, nincs dráma.

A [restic](https://restic.net) hajtja, így minden mentés deduplikált, inkrementális és mindig titkosított.

!!! note "Óvd meg az APP_KEY-t"
    A BombVault a restic tároló jelszavát egy `APP_KEY` nevű, 32 bájtos titokból származtatja. Ha elveszíted, a titkosított mentések visszaállíthatatlanná válnak. Generálj egyet az `openssl rand -hex 32` paranccsal, és tárold biztonságos helyen. Lásd: [Konfiguráció](configuration.md).

## Mit véd a BombVault

| Tartomány | Mi kerül mentésre |
|---|---|
| **Docker-konténerek** | Az appdata könyvtár, valamint a konténer definíciója (image, környezeti változók, portok, címkék, kötetek). |
| **KVM / libvirt VM-ek** | A VM lemezképe(i), az XML-definíció és az UEFI NVRAM, SSH-n keresztül mentve (nincs libvirt-csatolás). |
| **Unraid flash** | A teljes USB flash (`/boot`): operációs rendszer, licenc, tömbkonfiguráció, megosztások, hálózati és bővítmény-konfiguráció. |
| **Alkalmazás-konfiguráció** | A BombVault saját `/config` mappája: a beállítás-adatbázisa, a telephelyen kívüli hitelesítő adatok és a libvirt SSH-kulcspár. |
| **Fájlok és mappák** | Elnevezett **fájlkészletek**, a szerver bármely mappája, mindegyik opcionális, készletenkénti kizárási mintákkal. |

## A visszaállítás a főszereplő

Miután visszamásolta az adatokat a restic pillanatképből, a BombVault visszajátssza a mentett konténerdefiníciót a Docker API felé, így a konténer pontosan úgy jelenik meg újra az Unraid Docker fülén, mintha mindig is ott lett volna (ugyanaz az image, ugyanazok a beállítások, ugyanazok a port-hozzárendelések). A VM-ek XML-je SSH-n keresztül újradefiniálódik, a lemezeik és az UEFI NVRAM-juk pedig újracsatolódik, még akkor is, ha a VM-et törölték.

Amikor egy mentés leállítja a függő konténereket, azok a megfelelő sorrendben térnek vissza: a BombVault a Compose `depends_on` sorrendjükben indítja újra őket, és megvárja, amíg mindegyik egészségesnek jelenti magát, mielőtt elindítaná a rá épülőket, így semmi sem előzi meg egy még nem elérhető adatbázist vagy átjárót. Lásd: [Funkciók](features.md).

## Hogyan működik

```
Browser --HTTPS--> BombVault container
                   |- Go binary: JSON API + embedded React UI
                   |- Background worker (per-domain scheduler + job executor)
                   |
                   |- /var/run/docker.sock  -> Docker API (container stop/inspect/recreate)
                   |- qemu+ssh://host       -> libvirt / KVM on the HOST over SSH (no mount)
                   |- /mnt/ -> /host/user   -> appdata, VM disks + restic repos (read/write)
                   |- /boot/ -> /host/boot  -> Unraid flash backup (whole USB)
                   |- /config               -> BombVault's own settings + credentials (self-backup)
                   '- <repo path>           -> restic repository (local or remote: rclone/s3/rest/sftp)
```

A BombVault az orkesztrációs és felhasználói felületi réteg, nem a tárolómotor. Minden tényleges adatmozgatás a resticen keresztül történik.

## Gyorsindítás

Új vagy itt? Ugorj a **[Kezdő lépések](getting-started.md)** oldalra, hogy a Community Applications segítségével telepítsd a BombVaultot Unraidre, és lefuttasd az első mentésedet. Ezután fedezd fel a teljes **[Funkciók](features.md)** listát, hangold a **[Konfigurációt](configuration.md)**, és állítsd be a **[Telephelyen kívüli mentést és helyreállítást](offsite-recovery.md)**.

A telephelyen kívüli mentés tartományonként egyszerre több célra is szétoszthat, egy csak olvasható **fogadó irányítópult** figyeli ezeket a másolatokat azon a gépen, amely fogadja őket, a teljes konfigurációdat pedig átviheted egy új gépre az **Exportálás és importálás beállítások** kártyával. Lásd: [Telephelyen kívüli mentés és helyreállítás](offsite-recovery.md) és [Konfiguráció](configuration.md#portable-settings-export-and-import).

## Hivatkozások

- **Forráskód:** [github.com/junkerderprovinz/bombvault](https://github.com/junkerderprovinz/bombvault)
- **Unraid támogatói téma:** [forums.unraid.net](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/)
- **Hibajegyek:** [github.com/junkerderprovinz/bombvault/issues](https://github.com/junkerderprovinz/bombvault/issues)

!!! warning "Root-szintű vezérlés a hoszt felett"
    A Docker socketen keresztül a BombVault le tud állítani, el tud távolítani és újra létre tud hozni konténereket, valamint olvasni és írni tudja az appdatát, a VM-mentéshez pedig SSH-n keresztül bejelentkezik a hosztra, hogy futtassa a `virsh` parancsot. Aki eléri a webes felületét, annak gyakorlatilag root-jogosultsága van a hoszton. A BombVaultot csak megbízható, nem kitett hálózaton futtasd, és kapcsold be az opcionális jelszavas védelmet (Beállítások, Biztonság), amint telephelyen kívüli vagy módosíthatatlan mentéseket használsz. A teljes biztonsági modellhez lásd: [Konfiguráció](configuration.md).
