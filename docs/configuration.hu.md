# Konfiguráció

Ez az oldal a konténer környezeti változóit, a sablon által biztosított csatolásokat, a VM-mentést SSH-n keresztül, valamint a telephelyen kívüli beállítást ismerteti. A mentési **tároló-útvonalak** az alkalmazáson belül konfigurálhatók (Beállítások, Mentési útvonalak), nem környezeti változókon keresztül.

## Környezeti változók

| Változó | Kötelező | Leírás |
|---|---|---|
| `APP_KEY` | **Igen** | 32 bájtos hexadecimális titok (64 hexadecimális karakter), amely a restic tároló jelszavának származtatására szolgál. Generáld az `openssl rand -hex 32` paranccsal. Óvd ezt: az elvesztése visszaállíthatatlanná teszi a titkosított mentéseket. |
| `LIBVIRT_HOST` | VM-ekhez | Az SSH-n keresztül elért Unraid hoszt a VM-mentéshez (alapból `host.docker.internal`; a sablon egy LAN-IP helyőrzővel tölti ki előre). Használd az Unraid LAN IP-jét, egyéni `br0.x` hálózaton kötelező. |
| `LIBVIRT_SSH_PORT` | Nem | A hoszt SSH-portja a VM-mentéshez (alapból `22`). |
| `LIBVIRT_SSH_USER` | Nem | SSH-felhasználó a hoszton a VM-mentéshez (alapból `root`). |
| `LIBVIRT_URI` | Nem | Teljes libvirt kapcsolati URI, amelyet a rendszer **szó szerint** használ a fenti három `LIBVIRT_*` változóból történő összeállítás helyett (ezeket a kapcsolati karakterlánc előállításakor ekkor figyelmen kívül hagyja). Alapból nincs beállítva. TrueNAS Scale-en szükséges, ahol a libvirtd egy nem szabványos socketen figyel, amit az összeállított forma nem tud kifejezni: `qemu+ssh://<user>@<truenas-host>/system?socket=/run/truenas_libvirt/libvirt-sock`. Lásd a [docs/vm-backup-ssh-setup.md](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) TrueNAS Scale szakaszát. |
| `PORT` | Nem | HTTP-port (alapból `3000`; csak `HTTP_ONLY=true` mellett használatos). |
| `HTTPS_PORT` | Nem | HTTPS-port (alapból `3443`; a sablon 1:1 arányban teszi közzé, így a WebUI a `https://<ip>:3443` címen válaszol). |
| `HTTP_ONLY` | Nem | Állítsd `true`-ra az önaláírt HTTPS-figyelő letiltásához, és csak egyszerű HTTP kiszolgálásához (egy TLS-lezáró reverse proxy mögötti használatra). |
| `HOST_SOURCE_ROOT` | Nem | A **Host Data**-ként csatolt hoszt-útvonal (alapból `/mnt`). A BombVault a Docker által jelentett bind-mount forrásokat lefordítja az ez alatt a csatolás alatti útvonalakra. Csak akkor változtasd meg, ha eltérő hoszt-gyökeret csatoltál. |
| `DATA_ROOT_SEGMENTS` | Nem | Vesszővel elválasztott útvonal-szegmens nevek, amelyek egy bind-mount forrást mentési adatként jelölnek meg (alapból `appdata`, az Unraid `/mnt/user/appdata/<container>` konvenciójának megfelelően). Egy konténer bind-mountja automatikusan kiválasztódik mentésre, ha a felsorolt szegmensek közül BÁRMELYIK teljes útvonal-összetevőként megjelenik a hoszt-forrásában, például a `DATA_ROOT_SEGMENTS=appdata,config` a `.../config` csatolást is felveszi. A konténer adatmappájának megtalálására szolgáló további, mindig aktív módszerekért lásd: [Mentési forrás felismerése](#backup-source-detection). |
| `PLATFORM` | Nem | Kikényszeríti, hogy a BombVault milyen platformon fut szerinte, ahelyett hogy automatikusan felismerné: `unraid`, `generic` vagy `truenas` (alapból nincs beállítva: a flash csatolás alatti `dockerMan` jelző keresésével automatikusan felismeri az Unraidet, egyébként `generic`; egy nem felismert érték szintén `generic`-re esik vissza, naplózva). Állítsd be kifejezetten egy generikus Docker-hoszton vagy TrueNAS Scale-en, ahelyett hogy a csak Unraidre működő automatikus felismerésre hagyatkoznál; a generikus compose-fájl ezt teszi. Megváltoztatja az appdata-tartalék konvenciót, a példányok közötti visszaállítási cél alapértelmezéseit, valamint azt, hogy a csak Unraidre vonatkozó értesítési és kísérő bővítmény lépéseket egyáltalán megkísérli-e (lásd: `internal/platform`). |
| `BOMBVAULT_SELF_CONTAINER` | Nem | Magának a BombVault konténernek a neve, hogy soha ne mentse (és így ne állítsa le) önmagát (alapból `BombVault`; bridge hálózaton a hostname alapján automatikusan felismerve). |
| `BACKUP_MAX_HOURS` | Nem | A maximális valós idejű órák száma, ameddig egyetlen mentési futás a tartományzárolását tarthatja, mielőtt kényszerítve megszakadna (egy védelem, hogy egy beragadt futás ne blokkolhassa örökre a tartományt). Üresen (az alapértelmezett) `48`-at használ. Emeld nagyon nagy vagy lassú felhőmentésekhez (egy a korlátnál megszakított futás `context deadline exceeded` hibával hiúsul meg). Állítsd `0`-ra a korlát teljes letiltásához. |
| `TZ` | Nem | Időzóna az ütemezőhöz (például `Europe/Berlin`). **Ha nincs beállítva, minden ütemezés UTC szerint fut**: a 02:30-ra állított ütemezés ekkor 02:30 UTC-kor indul, nem a helyi idő szerint. Unraiden ezt soha nem kell beállítania: a rendszer a saját időzónáját adja át minden konténernek. |

## Csatolások

Csatold a Docker socketet, a flasht (`/boot`) és a **Host Data** gyökeret (`/mnt`), ahogy a CA-sablonban látható. A mentési *források* és *célok* egyaránt a Host Data alatt találhatók, és az **rslave** módban van csatolva, így egy távoli megosztás, amely a konténer indulása után csatolódik (például a `/mnt/remotes` alatt), újraindítás nélkül válik láthatóvá.

A mentési tároló-útvonalak alapértelmezetten a `/mnt/user/bombvault/{container,vms,flash,config,files}` útvonalra mutatnak, és az első mentéskor jönnek létre. A helyet bármikor megváltoztathatod a **Beállítások, Mentési útvonalak** alatt.

!!! note "Hosztintegráció-ellenőrzés"
    A konténer elindulása után nyisd meg a `/spike` oldalt a webes felületen. Ez minden csatolást és CLI-t megvizsgál (Docker socket, libvirt, restic, qemu-img, rclone), és jelenti a hiányzó darabokat.

## Biztonsági modell

!!! warning "Root-szintű vezérlés a hoszt felett"
    A Docker socketen keresztül a BombVault le tud állítani, el tud távolítani és újra létre tud hozni konténereket, valamint olvasni és írni tudja az appdatát, a VM-mentéshez pedig SSH-n keresztül bejelentkezik a hosztra (`qemu+ssh://`, alapból root), hogy futtassa a `virsh` parancsot. Aki eléri a webes felületét, annak gyakorlatilag root-jogosultsága van a hoszton.

- **Opcionális jelszavas védelem** (Beállítások, Biztonság): állíts be egy jelszót a bejelentkezés megköveteléséhez, töröld a letiltáshoz. Alapból ki van kapcsolva a megbízható LAN-os használathoz. A munkamenetek aláírtak (HMAC az `APP_KEY`-ből származtatva), és a jelszó megváltoztatása érvényteleníti őket; a bejelentkezések sebességkorlátozottak.
- Mivel a védelem opcionális, ha nincs beállítva, a teljes felület és API (beleértve a telephelyen kívüli beállítást, a manipulációs teszt útvonalait és a helyreállítási csomagot) elérhető bárki számára, aki eléri a portot. Kapcsold be a védelmet, amint telephelyen kívüli, módosíthatatlan mentések vagy titkosítás van használatban.
- A BombVaultot csak megbízható, nem kitett hálózaton futtasd. A távoli hozzáféréshez tedd egy reverse proxy mögé, amely hitelesítést és TLS-t ad hozzá. A válaszok alapszintű biztonsági fejléceket hordoznak (CSP, `nosniff`, `X-Frame-Options`, `Referrer-Policy`).
- A `HTTP_ONLY=true` mellett a munkamenet-süti elveszíti a `Secure` jelzőjét (muszáj, hogy egyszerű HTTP-n működjön), így csak egy TLS-lezáró proxy mögött kapcsold be a jelszót, ha a bizalmasság számít.
- A VM-mentés SSH-kapcsolata az első kapcsolatfelvételkor megbízik a hoszt-kulcsban (TOFU), és utána rögzíti. Ellenőrizd a hoszt kulcsát sávon kívül, ha a konténer-hoszt útvonalad nem megbízható.
- A mentések a restic által titkosítottak, ha a titkosítás engedélyezve van (Beállítások; alapból be), a kulcs az `APP_KEY`-ből származtatva.

## VM-mentés SSH-n keresztül

A BombVault a KVM/libvirt VM-eket **bármely libvirt-útvonal csatolása nélkül** menti. A `virsh` parancsot a hoszton, SSH-n keresztül futtatja (`qemu+ssh://`), így soha nem tudja befolyásolni a hoszt VM Managerét.

Gyors beállítás:

1. **Beállítások, Rendszer, VM-mentés SSH-n keresztül:** másold ki a megjelenített nyilvános kulcsot.
2. Fűzd hozzá az Unraid `/root/.ssh/authorized_keys` fájljához (a flashre is mentve, így túléli az újraindításokat).
3. Kattints a **Kapcsolat tesztelése** gombra.

A sablon hozzáadja a `--add-host=host.docker.internal:host-gateway` opciót, hogy a konténer elérhesse a hosztot. Állítsd a `LIBVIRT_HOST`-ot az Unraid LAN IP-jére, ha ez a név nem oldódik fel (például amikor a konténer egyéni `br0.x` hálózaton fut). Ha megváltoztattad az Unraid SSH-portját, állítsd be a `LIBVIRT_SSH_PORT`-ot, hogy egyezzen. Az **élő pillanatképekhez** ezen felül szükség van a qemu guest agentre a VM-ben, és arra, hogy a lemez a `/mnt/cache`-en (ne a `/mnt/user`-en) legyen.

!!! important "Teljes VM-beállítási és hálózati útmutató"
    A teljes, lépésről lépésre útmutató (SSH engedélyezése, tartós kulcs-engedélyezés, egyéni hálózati és VLAN-útválasztás, VM-enkénti módszer és hoszt-oldali hibaelhárítás) a [docs/vm-backup-ssh-setup.md](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) oldalon található a GitHubon.

## Telephelyen kívüli beállítás

Állíts be egy telephelyen kívüli replikát a **Beállítások, Telephelyen kívüli** fülön. A teljes munkafolyamathoz (módosíthatatlan/append-only, manipulációs tesztelés és DR-próbák) lásd: [Telephelyen kívüli mentés és helyreállítás](offsite-recovery.md). Röviden:

- **Backendek:** SMB/CIFS és NFS (csatold a megosztást, és irányíts rá egy Mentési útvonalat), natív restic backendek rclone nélkül (`s3:...`, `rest:http://host:8000/repo`, `b2:...`, `sftp:user@host:/repo`), vagy bármely rclone remote (`rclone:<remote>:<bucket>/path`).
- **A felhő hitelesítő adatai** titkosítva tárolódnak a Beállítások, Telephelyen kívüli, Felhő hitelesítő adatok alatt.
- **Az SSH-célokhoz semmit sem kell telepíteni a túloldalon.** Az `sftp:` csak egy SSH-szervert igényel. Add hozzá a nyilvános kulcsot a **Beállítások, Rendszer, VM-mentés SSH-n keresztül** alól (a `/config/ssh/id_ed25519.pub` alatt is) a célfelhasználó `~/.ssh/authorized_keys` fájljához.
- **Telephelyen kívüli másolat:** A BombVault az új pillanatképeket `restic copy` segítségével, legjobb szándék szerint replikálja. A helyi tároló marad az elsődleges. Minden tartománynak saját telephelyen kívüli ütemezése van, plusz egy **Replikálás most** gomb.
- **Több telephelyen kívüli cél tartományonként:** minden tartomány egyszerre több telephelyen kívüli célra is replikálhat. Adj hozzá további célokat a Beállítások, Telephelyen kívüli alatt, mindegyiket saját tárolóval, S3-tárolási osztállyal, append-only jelzővel, megőrzéssel és növekedési kerettel; mindegyik az adott tartomány telephelyen kívüli ütemezése szerint replikál. Egy meglévő egyetlen telephelyen kívüli beállítás az első célként öröklődik át.
- **Megőrzés forrásonként:** a helyi szabály a Beállítások, Útvonalak és tárolás alatt él; a telephelyen kívüli szabály a Beállítások, Telephelyen kívüli alatt (hagyd mind nullán, hogy soha ne nyesse automatikusan a telephelyen kívüli pillanatképeket).
- **Sávszélesség-korlátok:** korlátozd a restic fel- és letöltési sebességét a Beállítások, Telephelyen kívüli alatt.
- **Hideg és archív tárolási osztály (S3):** egy natív S3 telephelyen kívüli tárolóhoz válassz egy visszaállításra olvasható szintet (Standard, Standard-IA, One Zone-IA, Intelligent-Tiering, Glacier Instant Retrieval). Az rclone remote-ok a saját osztályukat az rclone konfigban állítják be.

## Hordozható beállítások (exportálás és importálás) {#portable-settings-export-and-import}

Az **Exportálás és importálás beállítások** kártya a Beállítások oldalon a teljes BombVault-konfigurációdat (tartománybeállítások, telephelyen kívüli célok, ütemezések, megőrzés, értesítések) egy hordozható JSON-fájlba írja, amelyet egy másik példányon importálhatsz, így egy új gépre költözés vagy egy beállítás klónozása nem jelenti azt, hogy mindent kézzel kell újra beírni. Az importálás előnézetet mutat és megerősítést kér, és soha nem érinti a mentési adataidat vagy előzményeidet.

!!! warning "Az export hitelesítő adatokat tartalmazhat"
    Te választod meg, hogy belefoglalod-e a telephelyen kívüli és értesítési hitelesítő adatokat a fájlba. A hitelesítő adatokkal együtt az export olyan érzékeny, mint a helyreállítási csomagod, ezért tárold biztonságos helyen. Nélkülük a fájl csak nem-titkos beállításokat tartalmaz.
