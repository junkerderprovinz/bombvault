# Kezdő lépések

Ez az oldal végigvezet egy friss Unraid-géptől az első mentésedig.

## Követelmények {#requirements}

| Követelmény | Megjegyzések |
|---|---|
| **Unraid 6.12+** | A korábbi verziók nincsenek tesztelve. Az Unraid a fő platform, de a BombVault sima Docker-gazdagépen és TrueNAS Scale-en is fut (lásd: [Általános Docker-gazdagép](#generic-docker-host) és [TrueNAS SCALE](#truenas)). |
| **Restic tároló helye** | Helyi útvonal (ajánlott: a tömböd vagy a gyorsítótárad), SMB, NFS vagy bármely rclone backend. |
| **Docker socket** | A sablon automatikusan csatolja (`/var/run/docker.sock`). |
| **Unraid flash** (`/boot`) | A sablon automatikusan, teljes egészében csatolja (`/boot` a `/host/boot` alá). Ez teszi lehetővé a flash-mentést, és azt, hogy egy visszaállított konténer normál, szerkeszthető Unraid-alkalmazásként jelenjen meg újra. |
| **KVM VM-ek** (opcionális) | A VM-mentés SSH-n keresztül kommunikál a libvirttel, nincs libvirt-csatolás. Állítsd be a **Beállítások, Integrációk, Gazdagép SSH** alatt (lásd: [Konfiguráció](configuration.md)). |
| **ZFS-adatkészletek** (opcionális) | Ugyanaz az SSH-kapcsolat, mint a VM-mentéseknél, `zfs` a hoszton, és a Host Data `/mnt`-ként leképezve, Read/Write - Slave hozzáférési móddal, ami a sablon alapértelmezése. Lásd: [ZFS-adatkészletek](zfs-datasets.md). |
| **Android-alkalmazás** (nem kötelező) | Android 10 vagy újabb, 9.7.0-s vagy újabb verziójú szerverekkel párosítva. Lásd: [Android-alkalmazás](android.md). |

## Telepítés Unraidre {#install-on-unraid}

A legegyszerűbb út a **Community Applications**.

1. Nyisd meg az **Apps** fület az Unraidben.
2. Keress rá a **BombVault** kifejezésre.
3. Kattints az **Install** gombra, állítsd be a szükséges változókat (lásd lent), majd alkalmazd.

!!! tip "Kézi sablontelepítés"
    Ha inkább kézzel adnád hozzá a sablont:

    1. Menj a **Docker, Add Container, Template repositories** menübe, és add hozzá:
       ```
       https://github.com/junkerderprovinz/unraid-apps
       ```
    2. Keress rá a **BombVault** kifejezésre a Templates alatt.
    3. Állítsd be a szükséges változókat, és kattints az **Apply** gombra.

## Általános Docker-gazdagép {#generic-docker-host}

Nem Unraid? A BombVault sima konténerként is fut bármelyik Docker-gazdagépen. A TrueNAS SCALE saját lépései a [TrueNAS SCALE](#truenas) alatt találhatók.

1. Töltsd le a tárolóból a szerkesztésre kész [`deploy/docker-compose.generic.yml`](https://github.com/junkerderprovinz/bombvault/blob/main/deploy/docker-compose.generic.yml) fájlt.
2. Állítsd be az `APP_KEY` értékét (lásd lent), és irányítsd a Host Data kötetet a valódi adatgyökeredre. A fájl megjegyzései mindkettőt végigvezetik.
3. `docker compose up -d`, majd nyisd meg a `https://<gazdagép-ip>:3443/` címet.

Miben más ez, mint az Unraid:

- **Nincs Flash szakasz.** Nincs indító pendrive, amit el kellene menteni vagy vissza kellene állítani, így a Flash szakasznak itt nincs dolga. Helyette, ha a Host Data eléri a gazdagép `/etc` mappáját, a **Biztonsági mentések** oldalon a **Hozzáadás** felkínálja a **Rendszerkonfiguráció** mappajavaslatot: egy induló `/etc` mappakészletet, amit mentés előtt átnézel és szerkesztesz.
- **Nincsenek Unraid-natív értesítések.** A BombVault saját értesítési csatornái (webhook, külső telephelyi hiba riasztásai és így tovább) a szokott módon működnek. Csak az Unraid saját értesítési rendszerébe küldés marad el, mert ilyen rendszer itt nincs.
- **A virtuális gépek mentése választható, és külön, SSH-n elérhető libvirtd gazdagépet igényel.** Lásd a compose fájl kikommentezett blokkját. Egy általános Docker-gazdagépben magában nincs virtuálisgép-kezelő.
- **Nincs irányítópult-widget.** A BombVault Widget egy Unraid-bővítmény, így ez a lépés is kimarad.
- **Egy konténer adatainak megtalálása.** Az Unraid `appdata` konvenciója nélkül a konténer adatmappáját a `DATA_ROOT_SEGMENTS` szegmensei, a Docker elnevezett kötetei, egy Compose-projekt munkakönyvtára és a `bombvault.data` címke alapján találja meg (lásd: [A mentési források felismerése](configuration.md#backup-source-detection)). Az elnevezett kötetek és az `/etc` előbeállítás csak a Host Data csatoláson belüli útvonalakat érik el, ezért a Host Data egy olyan közös szülőkönyvtárra mutasson, amely a Docker adatgyökerét is lefedi.
- **`PLATFORM`.** Állítsd `generic` vagy `truenas` értékre. Ha nincs beállítva, a BombVault az Unraidet a flash-csatoláson lévő saját jelölője alapján ismeri fel, minden mást általánosként kezel, és a csak Unraidre vonatkozó lépéseket kihagyja, ahelyett hogy megpróbálná és elbukna rajtuk.

### TrueNAS SCALE {#truenas}

A BombVault még nincs benne a TrueNAS alkalmazáskatalógusában, ezért egyéni alkalmazásként telepítsd: **Apps, Discover Apps**, majd **Install via YAML** a **Custom App** melletti menüben. Az alábbi YAML ugyanazt állítja be, mint a repó `truenas-apps` mappájában lévő katalógusbejegyzés. Cseréld a `tank` nevet a saját poolodra, és töltsd ki az `APP_KEY` értékét a következő szakasz szerint.

```yaml
services:
  bombvault:
    image: junkerderprovinz/bombvault:latest
    hostname: bombvault
    restart: unless-stopped
    cap_drop: [ALL]
    cap_add: [CHOWN, DAC_OVERRIDE, FOWNER, SETGID, SETPCAP, SETUID, SYS_CHROOT]
    environment:
      APP_KEY: ""
      PLATFORM: truenas
      HOST_SOURCE_ROOT: /mnt/tank
      HOST_MOUNT_ROOT: /mnt/tank
      # For VM backups:
      # LIBVIRT_URI: qemu+ssh://root@<truenas-ip>/system?socket=/run/truenas_libvirt/libvirt-sock
    volumes:
      - /mnt/tank/apps/bombvault:/config
      - /var/run/docker.sock:/var/run/docker.sock
      - type: bind
        source: /mnt/tank
        target: /mnt/tank
        bind:
          propagation: rslave
    ports:
      - "30528:3443"
```

Ezután nyisd meg a `https://<truenas-ip>:30528/` címet. Ami a [Miben más ez, mint az Unraid](#generic-docker-host) alatt szerepel, itt is érvényes, TrueNAS-on pedig még:

- **Capabilityk.** A BombVaultnak a fenti hétre van szüksége. `SETPCAP` nélkül a VM-mentések meghiúsulnak, `SYS_CHROOT` nélkül pedig minden konténer a teljes mentése idejére leáll, ahelyett, hogy a BombVault pillanatképből olvasná.
- **Host Data** egy adatkészleten lévő hoszt-útvonal, itt `/mnt/tank`, az alkalmazáson belül ugyanazon az útvonalon csatolva. Ezért adja meg a `HOST_SOURCE_ROOT` és a `HOST_MOUNT_ROOT` is. Az `rslave` továbbadja a hoszt később létrehozott csatolásait, amelyekre a ZFS-adatkészletek mentésének szüksége van.
- **Az ixVolume-ban lévő adatok nem kerülnek mentésre.** Az az alkalmazás, amelynek tárolója ixVolume, a `/mnt/.ix-apps` alatt tartja az adatait, a Host Data-n kívül. A BombVault csak az alkalmazás definícióját menti, és a futás megnevezi a kihagyott mappákat. Minden alkalmazáshoz, amelynek adatait menteni szeretnéd, válassz **Host Path** tárolót egy Host Data alatti adatkészleten.
- **A VM- és ZFS-adatkészlet-mentések** SSH-n keresztül, rootként érik el a TrueNAS-t. Kapcsold be az SSH szolgáltatást a **System, Services** alatt, és add hozzá a nyilvános kulcsot a **Beállítások, Integrációk, Gazdagép SSH** helyről a root felhasználóhoz a **Credentials, Users** alatt. A VM-mentésekhez a fenti `LIBVIRT_URI` is kell, mert a TrueNAS libvirtd-je saját socketen figyel. Ha sem a `LIBVIRT_URI`, sem a `LIBVIRT_HOST` nincs beállítva, a BombVault az alkalmazás alapértelmezett átjáróját használja hosztként, mert a `host.docker.internal` egy TrueNAS-alkalmazásban nem oldható fel. A részleteket, a zvolokon lévő VM-eket is beleértve, a [VM-mentési útmutató](vm-backup-ssh-setup.md#truenas-scale) írja le.
- **A ZFS-replika másik kulcsot használ.** A második ZFS-kiszolgálón lévő replika azzal a kulccsal jelentkezik be, amelyet a replika párbeszédablaka mutat, nem a Gazdagép SSH kulcsával. Azt a fogadó kiszolgálón add hozzá.
- **Egy törölt alkalmazás két lépésben tér vissza.** Előbb telepítsd újra a TrueNAS-ban ugyanazzal a névvel, ami újra létrehozza a hálózatát, majd állítsd vissza az adatait a BombVaultban. Addig a visszaállítás előtti ellenőrzés jelzi a hiányzó hálózatot, és a visszaállítás elutasítja a futást, mielőtt bármit megváltoztatna.
- Ha az **Az eredeti mellé** módot választod, sima Docker-konténer jön létre. Fut, de a TrueNAS Apps képernyője nem listázza.
- **Az adatbázis-dumpok** adatkészlet-csatolásokon működnek. Egy importálás olyan adatmappába, amely önálló adatkészlet, a régi tartalmat egy mellette lévő mappába helyezi át, ezért a fölötte lévő adatkészletnek helyre van szüksége hozzá.

### ZimaOS, CasaOS és más hosztok {#other-hosts}

- **ZimaOS és CasaOS:** add hozzá a [közösségi áruházat](https://github.com/junkerderprovinz/zimaos-apps#install), és telepítsd belőle a BombVaultot. Itt `PLATFORM=generic` beállítással fut, a Host Data a `/DATA` mappán. A ZimaOS az alkalmazásadatokat a `/DATA/AppData/<app>` alatt tartja, amit a BombVault magától megtalál, mert a `DATA_ROOT_SEGMENTS` az `appdata` szót bármilyen kis- és nagybetűvel illeszti.
- **MOS:** telepítsd a BombVaultot a [MOS Hubról](https://mos-official.net/apps.html?search=BombVault). Itt `PLATFORM=generic` beállítással fut, a Host Data a `/mnt/cache` mappán.
- **Synology:** a Container Managerben hozz létre egy projektet a compose fájlból (DSM 7.2 vagy újabb).
- **Proxmox:** futtasd egy olyan VM-ben vagy LXC-konténerben, amelyben van Docker.

Megjegyzések nélkül az általános compose fájl rövid. Cseréld a `./data` mappát arra a hoszt-mappára, amelyben a konténereid adatai vannak, állítsd a `HOST_SOURCE_ROOT` értékét ugyanerre az útvonalra, és töltsd ki az `APP_KEY` értékét:

```yaml
services:
  bombvault:
    image: junkerderprovinz/bombvault:latest
    restart: unless-stopped
    hostname: bombvault
    container_name: bombvault
    volumes:
      - ./config:/config
      - /var/run/docker.sock:/var/run/docker.sock
      - ./data:/host/user
    environment:
      APP_KEY: ""
      PLATFORM: generic
      HOST_SOURCE_ROOT: /host/user
    ports:
      - "3443:3443"
```

A Docker elnevezett köteteinek mentéséhez is csatolj egy olyan mappát, amely az adataidat és a Docker adatgyökerét is tartalmazza (`docker info -f '{{.DockerRootDir}}'`, általában `/var/lib/docker`), ahogy az Unraid-sablon a teljes `/mnt` mappát csatolja.

## Az egyetlen kötelező beállítás {#the-one-required-setting}

Az egyetlen változó, amelyet be kell állítanod, az `APP_KEY`, egy 32 bájtos hexadecimális titok (64 hexadecimális karakter), amely a restic tároló jelszavának származtatására szolgál.

Generálj egyet bármely gépen:

```bash
openssl rand -hex 32
```

Illeszd be az eredményt a sablon `APP_KEY` mezőjébe (Unraid), vagy a `docker-compose.yml` `APP_KEY` környezeti változójába (általános Docker-gép).

!!! danger "Ne veszítsd el az APP_KEY-t"
    Az `APP_KEY` elvesztése visszaállíthatatlanná teszi a titkosított mentéseidet. Tárold biztonságos helyen, a szervertől elkülönítve. Amint a BombVault fut, állíts be bejelentkezési jelszót, és töltsd le a **Helyreállítási csomagot** a **Beállítások, Visszaállítás** alatt (lásd: [Telephelyen kívüli mentés és helyreállítás](offsite-recovery.md#recovery-kit)). Ebben ott van a kulcs, minden cél és a restic parancsai.

A sablon ezen felül csatolja a Docker socketet, a flasht (`/boot`) és a **Host Data** gyökeret (`/mnt`) is helyetted. A mentési *források* és *célok* egyaránt a Host Data alatt találhatók. A teljes változó-referenciáért és a telephelyen kívüli beállításért lásd: [Konfiguráció](configuration.md).

## Első futtatás {#first-run}

![Egy új telepítés üdvözlő oldala azzal, amit a BombVault a szerveren talált.](assets/screenshots/getting-started/welcome-dark.png#only-dark)
![Egy új telepítés üdvözlő oldala azzal, amit a BombVault a szerveren talált.](assets/screenshots/getting-started/welcome-light.png#only-light)

1. Nyisd meg a webes felületet a `https://<your-unraid-ip>:3443` címen (alapból önaláírt tanúsítvánnyal). Egy új telepítés az **Üdv a BombVaultban** oldalon nyílik meg, amely felsorolja, mit talált a BombVault a szerveren.
2. Válaszd a **Kezdjük** gombot. Az áttekintésen ekkor megjelenik az **Első lépések** kártya három lépéssel:
    1. **Válaszd ki a szakaszokat**: kapcsold be, amit a BombVault ment (Konténerek, VM-ek, Flash, Mappák, ZFS-adatkészletek, Önmentés). A **Kiválasztás** megnyitja a szakasz elemeinek listáját, ahol az **Ütemezés szüneteltetése** kihagy egyet. Az itt bekapcsolt szakaszok minden éjjel futnak, ezt a **Beállítások, Ütemezések** alatt változtathatod.
    2. **Cél hozzáadása**: a mentések második helye. Minden szakasznak már van egy mappája ezen a szerveren, `/mnt/user/bombvault/{container,vms,flash,config,files,zfs}`, amely az első mentéskor jön létre. A legbiztonságosabb egy második cél telephelyen kívül, például egy Storage Box vagy a Backblaze B2.
    3. **Első mentés**: ezután magától fut minden éjjel. A kártya addig marad, amíg az első mentés be nem fejeződött.
3. Nyisd meg a **Biztonsági mentések** oldalt, ott minden elem egy listában van. Az **Összes mentése most** a bekapcsolt szakaszok mindegyikén elindít egy futást, egy elem oldalán a **Biztonsági mentés most** csak azt az egyet menti.

![Az Első lépések kártya három lépéssel, a második megnyitva.](assets/screenshots/getting-started/first-steps-dark.png#only-dark)
![Az Első lépések kártya három lépéssel, a második megnyitva.](assets/screenshots/getting-started/first-steps-light.png#only-light)

Használtad már a BombVaultot? A **Visszaállítás mentésből** az üdvözlő oldalon megnyitja a Visszaállítás oldalt, ahol a **Vészvarázsló** visszahozza egy elveszett szerver beállításait, konténereit, VM-jeit és mappáit. Lásd: [Telephelyen kívüli mentés és helyreállítás](offsite-recovery.md#emergency).

!!! tip "Opcionális: válassz mentési sorrendet"
    Ha egyes konténereket mindig más konténerek előtt kell menteni (például egy adatbázist az azt használó alkalmazás előtt), nyisd meg a **Sorrend** kártyát a **Beállítások, Ütemezések** alatt, és húzd őket a kívánt sorrendbe. Az ütemezett futások és a teljes mentés ezt követik. Amit rendezetlenül hagysz, azt a legrégebben mentettel kezdve menti.

## Példa: az első konténermentésed, elejétől végéig {#example-first-backup}

Ez egy friss Unraid-gépen vezet végig, néhány konténerrel, köztük a `plex`-szel, az első mentés elkészültéig, és addig, hogy tudd, hol találod meg. Körülbelül tíz percet vesz igénybe, plusz az első mentés idejét.

1. **Telepítés.** Az Unraid **Apps** fülén keress rá a **BombVault** kifejezésre, és nyomd meg az **Install** gombot.
2. **A kulcs beállítása.** Egy számítógépen futtasd az `openssl rand -hex 32` parancsot, és illeszd be a 64 karaktert a sablon `APP_KEY` mezőjébe. Mentsd el egy másolatát a jelszókezelődbe még most, mindenek előtt. Nyomd meg az **Apply** gombot.
3. **Megnyitás.** Menj a `https://<your-unraid-ip>:3443` címre, és fogadd el az önaláírt tanúsítványt. Az **Üdv a BombVaultban** felsorolja, mit talált, például 12 konténert, 2 VM-et és a flasht.
4. **Kezdjük.** Az áttekintés az **Első lépések** kártyával nyílik meg.
5. **Válaszd ki a szakaszokat.** A **Konténerek** eleve be van kapcsolva. Kapcsold be, amit még szeretnél, például a **Flash**-t. A **Kiválasztás** megnyitja a talált konténerek listáját, ahol az **Ütemezés szüneteltetése** kihagy egyet, például egy teszt konténert, amely nem érdekel. A bekapcsolt szakaszok minden éjjel futnak a teljes mentésben.
6. **Cél hozzáadása.** Minden szakasznak már van saját mappája ezen a szerveren, a `/mnt/user/bombvault` alatt, így az első mentésnek van hová mennie. Ez a lépés egy második helyet ad hozzá, és a legbiztonságosabb a telephelyen kívüli. Megteheted most az [S3-példával](offsite-recovery.md#example-s3), vagy visszatérhetsz rá az első mentés után. A kártya addig marad.
7. **Első mentés.** Nyisd meg a **Biztonsági mentések** oldalt, és nyomd meg az **Összes mentése most** gombot, vagy várd meg az éjszakát. Az állapotfejben a gyűrű folyamatjelző gyűrűvé változik, és számolja az elemeket, például **Mentés folyamatban: 3 / 12**. Minden konténer leáll, amíg az adatait olvassa, és közvetlenül utána újra elindul. Az a konténer, amelynek adatai ZFS-en vannak, csak a pillanatkép néhány másodpercére áll le. Az első mentés mindent beolvas, ezért tart a legtovább. A későbbiek csak azt tárolják, ami változott.
8. **Ellenőrzés.** Amikor a gyűrű azt írja, hogy **Minden mentve van**, nyisd meg az áttekintésen az **Előzményeket**: minden konténernek zöld sora van a **Futások** között. Nyisd meg a **Biztonsági mentések** oldalt, kattints a `plex`-re, és nézd meg a **Biztonsági mentések** lapját: ott az első pillanatkép, a **Helyi** mint a tartalmazó hely. Néhány perc múlva az **Ellenőrzés** lapja megmutatja a visszaállítás-ellenőrzést, amely visszaállított egy mintát a mentésből, és visszaolvasta.
9. **A kulcs védelme.** Nyisd meg a **Beállítások, Biztonság** oldalt, és állíts be bejelentkezési jelszót. Aztán nyisd meg a **Beállítások, Visszaállítás** oldalt, és a **Vészhelyzetre** részben nyomd meg a **Helyreállítási csomag letöltése** gombot. Tárold a fájlt a szerveren kívül, a 2. lépésből származó `APP_KEY` másolat mellett. Az áttekintésen a **Mentsd el a helyreállítási csomagot** emlékeztető eltűnik, amint megnyomod a **Biztonságosan eltároltam** gombot.

Mostantól a konténereid minden éjjel mentésre kerülnek. Jó következő lépések, ebben a sorrendben: egy [telephelyen kívüli cél](offsite-recovery.md#example-s3), hogy egy elpusztult szerver ne vigye magával a mentéseket, aztán az [értesítések](notifications.md), hogy egy hiba eljusson hozzád, majd egy pillantás az [Ütemezésekre](schedules.md), ha a 03:00 nem felel meg.

![Az állapotfej az első mentés futása közben, a folyamatjelző gyűrűvel és az állomásaival.](assets/screenshots/getting-started/status-ring-dark.png#only-dark)
![Az állapotfej az első mentés futása közben, a folyamatjelző gyűrűvel és az állomásaival.](assets/screenshots/getting-started/status-ring-light.png#only-light)

!!! note "Hosztintegráció-ellenőrzés"
    Az áttekintés **Hosztintegráció** kártyája minden csatolást és CLI-t megvizsgál (Docker socket, libvirt, restic, qemu-img, rclone), és jelenti a hiányzó darabokat, így megbizonyosodhatsz róla, hogy a konténer helyesen van bekötve, mielőtt rá hagyatkoznál. A **Testreszabás** alatt vár, amíg hozzáadod.

## Eligazodás {#navigation}

![A beállítások csempékként, mellettük a kiválasztott csempe kártyái. Minden változás azonnal kiíródik.](assets/screenshots/getting-started/settings-rail-dark.png#only-dark)
![A beállítások csempékként, mellettük a kiválasztott csempe kártyái. Minden változás azonnal kiíródik.](assets/screenshots/getting-started/settings-rail-light.png#only-light)

Az oldalsávnak öt eleme van: **Áttekintés**, **Anomáliák** (amíg az anomáliafelismerés be van kapcsolva), **Biztonsági mentések**, **Visszaállítás**, és a **Kijelentkezés** alatt a **Beállítások**. Egy oldal fő művelete lebeg a jobb alsó sarokban, a ritka műveleteket a **Több** gyűjti. Telefonon a sáv az Áttekintést, a Biztonsági mentéseket, a Visszaállítást és a Beállításokat tartalmazza, a többit a **Több** adja.

Az oldalak és a beállítások az ablak teljes szélességét használják. Egy ablak a gombjait a láblécében tartja, az előre vivő gombot jobb oldalon. Telefonon a háromnál több gombos lábléc a kiutat és a fő műveletet megtartja, a többit a **Több** alá teszi.

![Telefonon a sáv alul, a Több megnyitva.](assets/screenshots/getting-started/phone-more-dark.png#only-dark)
![Telefonon a sáv alul, a Több megnyitva.](assets/screenshots/getting-started/phone-more-light.png#only-light)

- Az **Áttekintés** megmutatja, hogy minden le van-e mentve, és mi igényel figyelmet. Lásd: [Áttekintés](overview.md).
- A **Biztonsági mentések** elemekként listázza az összes konténert, VM-et, mappakészletet, a flasht, minden ZFS-adatkészletet és az Önmentést. A lista feletti csempék fajtát választanak, a **Szűrők**, a **Rendezés** és a **Keresés** szűkít. Minden elemnek saját oldala van három füllel: **Biztonsági mentések**, **Beállítások** és **Ellenőrzés**. Az elem ikonjától balra lévő visszagomb a listához vezet. Lásd: [Biztonsági mentések és elemek](backups.md).
- A **Visszaállítás** egyetlen oldal mindenhez: válassz szakaszt, keress egy elemet, és válassz egy mentést. A Vészvarázsló és a többi forrás is innen indul. Lásd: [Visszaállítás](restore.md).
- A **Beállítások** tizennégy csempe: Általános, Megjelenés, Ütemezések, Célok, Visszaállítás, Értesítések, Ellenőrzés, Biztonság, Példányok, Párosítás, Integrációk, Alkalmazások, Rendszer és Információ. Lásd: [Beállítások](settings.md). A `/` vagy a Ctrl+K lenyomásával mindben kereshetsz, a korábbi verziókban használt oldalnevekre is. A csempéket a **Megjelenés** alatt húzhatod saját sorrendbe. Ahol az oldal melletti csempeoszlop összeszorítaná az oldalt, például egy kb. 1200 pixel széles ablakban, a csempék az oldal fölötti sávvá válnak, amely oldalra görgethető. A telefon ugyanezt a sávot mutatja.
- A **Célok** és a **Példányok** beállítási csempék, saját oldalakkal. A csempéjükön a **Mutatás az oldalsávban is** kitűzi őket az oldalsávra.

Semminek nincs Mentés gombja, és semmi sem hajlik szét: a ritka dolgoknak saját kártyájuk vagy ablakuk van, a magyarázatok pedig az (i) buborékokban vannak. Frissítés után magától megnyílik egy ablak minden kiadáshoz egy kártyával az utolsó verziód óta, és zöld pontok jelzik, mi változott. Egy kattintás elrejti a pontot. Az **Újdonságok** a **Beállítások, Információ** alatti Verzió kártyán megszámolja, mit nem láttál még, és újra megnyitja ezt az ablakot, minden kiadás megjegyzéseivel és változásaival az **Ugrás oda** gombbal.

## Fordítás forráskódból {#build-from-source}

A BombVault egyetlen statikus Go bináris, amely egy JSON API-t és egy beágyazott React-felületet szolgál ki. Először fordítsd le a felületet, aztán futtasd a binárist:

```bash
npm --prefix web ci
npm --prefix web run build     # writes web/dist, which the binary embeds
export APP_KEY=$(openssl rand -hex 32)
go test ./...                  # unit and integration tests, with a real restic round trip
golangci-lint run ./...
go run ./cmd/bombvault         # serves https://localhost:3443 with a self-signed certificate
```

A felület fordítása a `go run`-hoz is kell. A repó a `web/dist` alatt csak egy üres jelölőfájlt követ, így `npm --prefix web run build` nélkül a bináris semmit sem ágyaz be, és `500 SPA index not found` választ ad, ami várható. A Docker, a libvirt és az Unraid nem tesztelhető CI-ben, ezért mielőtt pull requestet nyitsz, ellenőrizd a csatolásokat, a resticet és a VM SSH-kapcsolatát egy valódi hoszton az áttekintés **Hosztintegráció** kártyájával (vagy a `GET /api/spike` hívással).

## Következő lépések {#next-steps}

- Adj hozzá egy vagy több telephelyen kívüli célt (minden szakasz egyszerre több célra is másolhat), és mentsd el a helyreállítási csomagodat, ahogy a **[Telephelyen kívüli mentés és helyreállítás](offsite-recovery.md)** leírja. Az [S3-példa](offsite-recovery.md#example-s3) végigvezet egyen.
- Gondoskodj róla, hogy egy hiba eljusson hozzád az **[Értesítésekkel](notifications.md)**.
- Nézd meg, mikor mi fut, az **[Ütemezéseknél](schedules.md)**.
- Vidd a csoportod összes szerverét a telefonodra az **[Android-alkalmazással](android.md)**.
- A **[részletes példák](guides.md)** végigvezetnek a gyakori feladatokon, a **[Funkciók](features.md)** pedig felsorol mindent, amit a BombVault tud.
- Egy beállítást klónozol, vagy új gépre költözöl? Vidd át a teljes konfigurációdat a **Beállítások exportálása / importálása** kártyával. Lásd: [Konfiguráció](configuration.md#portable-settings-export-and-import).
- Elakadtál? Lásd: **[Hibaelhárítás](troubleshooting.md)**.
