# Kezdő lépések

Ez az oldal végigvezet egy friss Unraid-géptől az első mentésedig.

## Követelmények

| Követelmény | Megjegyzések |
|---|---|
| **Unraid 6.12+** | A korábbi verziók nincsenek tesztelve. Az Unraid a fő platform, de a BombVault sima Docker-gazdagépen és TrueNAS Scale-en is fut (lásd: [Általános Docker-gazdagép](#generic-docker-host)). |
| **Restic tároló helye** | Helyi útvonal (ajánlott: a tömböd vagy a gyorsítótárad), SMB, NFS vagy bármely rclone backend. |
| **Docker socket** | A sablon automatikusan csatolja (`/var/run/docker.sock`). |
| **Unraid flash** (`/boot`) | A sablon automatikusan, teljes egészében csatolja (`/boot` a `/host/boot` alá). Ez teszi lehetővé a flash-mentést, és azt, hogy egy visszaállított konténer normál, szerkeszthető Unraid-alkalmazásként jelenjen meg újra. |
| **KVM VM-ek** (opcionális) | A VM-mentés SSH-n keresztül kommunikál a libvirttel, nincs libvirt-csatolás. Állítsd be a Beállításokban (lásd: [Konfiguráció](configuration.md)). |
| **ZFS-adatkészletek** (opcionális) | Ugyanaz az SSH-kapcsolat, mint a VM-mentéseknél, `zfs` a hoszton, és a Host Data `/mnt`-ként leképezve, Read/Write - Slave hozzáférési móddal, ami a sablon alapértelmezése. Lásd: [ZFS-adatkészletek](zfs-datasets.md). |
| **Android-alkalmazás** (nem kötelező) | Android 10 vagy újabb, 9.7.0-s vagy újabb verziójú szerverekkel párosítva. Lásd: [Android-alkalmazás](android.md). |

## Telepítés Unraidre

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

Nem Unraid? A BombVault sima konténerként is fut bármelyik Docker-gazdagépen (ez hordozza a TrueNAS Scale konténertámogatását is, még az ottani alkalmazáskatalógus saját bejegyzése előtt).

1. Töltsd le a tárolóból a szerkesztésre kész [`deploy/docker-compose.generic.yml`](https://github.com/junkerderprovinz/bombvault/blob/main/deploy/docker-compose.generic.yml) fájlt.
2. Állítsd be az `APP_KEY` értékét (lásd lent), és irányítsd a Host Data kötetet a valódi adatgyökeredre: a fájl megjegyzései mindkettőt végigvezetik.
3. `docker compose up -d`, majd nyisd meg a `https://<gazdagép-ip>:3443/` címet.

Miben más ez, mint az Unraid:

- **Nincs flash/USB tartomány.** Nincs indító pendrive, amit el kellene menteni vagy vissza kellene állítani, így a beállítások Flash tartományának itt nincs dolga. Helyette a Mappák tartomány egy kattintással felkínálja az **Előbeállítás hozzáadása: gazdagép rendszerkonfigurációja** javaslatot (egy induló `/etc` fájlkészlet, amit mentés előtt átnézel és szerkesztesz), gyakorlatias általános megfelelőként.
- **Nincsenek Unraid-natív értesítések.** A BombVault saját értesítési csatornái (webhook, külső telephelyi hiba riasztásai és így tovább) a szokott módon működnek; csak az Unraid saját értesítési rendszerébe küldés marad el, mert ilyen rendszer itt nincs.
- **A virtuális gépek mentése választható, és külön, SSH-n elérhető libvirtd gazdagépet igényel.** Lásd a compose fájl kikommentezett blokkját. Egy általános Docker-gazdagépben magában nincs virtuálisgép-kezelő.
- **Nincs irányítópult-widget.** A BombVault Widget egy Unraid-bővítmény, így ez a lépés is kimarad.
- **Egy konténer adatainak megtalálása.** Az Unraid `appdata` konvenciója nélkül a konténer adatmappáját a `DATA_ROOT_SEGMENTS` szegmensei, a Docker elnevezett kötetei, egy Compose-projekt munkakönyvtára és a `bombvault.data` címke alapján találja meg (lásd: [A mentési források felismerése](configuration.md#backup-source-detection)). Az elnevezett kötetek és az `/etc` előbeállítás csak a Host Data csatoláson belüli útvonalakat érik el, ezért a Host Data egy olyan közös szülőkönyvtárra mutasson, amely a Docker adatgyökerét is lefedi.
- **`PLATFORM`.** Állítsd `generic` vagy `truenas` értékre. Ha nincs beállítva, a BombVault az Unraidet a flash-csatoláson lévő saját jelölője alapján ismeri fel, minden mást általánosként kezel, és a csak Unraidre vonatkozó lépéseket kihagyja, ahelyett hogy megpróbálná és elbukna rajtuk.

A **TrueNAS Scale** ugyanezt a compose-utat járja; egy katalógusbejegyzés elő van készítve a repóban, de még nincs beküldve. A VM-mentéshez ott `LIBVIRT_URI` kell, mert a TrueNAS libvirtd-je saját socketen figyel (`/run/truenas_libvirt/libvirt-sock`), amelyet a három `LIBVIRT_*` változó nem tud kifejezni (lásd: [Konfiguráció](configuration.md)). Mennyire bizonyított ez: a zvol-mentést egy valódi TrueNAS Scale gépen futtattuk, egy futó VM-hez csatolt zvolon, és a `zfs snapshot`, a `zfs send`, a restic és a `zfs receive` bájtra pontosan oda-vissza vitte. A BombVault által vezérelt teljes visszaállítás TrueNAS hardveren még nem futott, és az a zvol ritka (sparse) volt, így a sok gigabájtos áteresztőképesség nincs tesztelve. Mielőtt ráhagyatkoznál, próbálj ki ott egy visszaállítást.

## Az egyetlen kötelező beállítás

Az egyetlen változó, amelyet be kell állítanod, az `APP_KEY`, egy 32 bájtos hexadecimális titok (64 hexadecimális karakter), amely a restic tároló jelszavának származtatására szolgál.

Generálj egyet bármely gépen:

```bash
openssl rand -hex 32
```

Illeszd be az eredményt a sablon `APP_KEY` mezőjébe (Unraid), vagy a `docker-compose.yml` `APP_KEY` környezeti változójába (általános Docker-gép).

!!! danger "Ne veszítsd el az APP_KEY-t"
    Az `APP_KEY` elvesztése visszaállíthatatlanná teszi a titkosított mentéseidet. Tárold biztonságos helyen, a szervertől elkülönítve. Amint a BombVault fut, használd az egykattintásos **titkosításikulcs-helyreállító csomagját** (lásd: [Telephelyen kívüli mentés és helyreállítás](offsite-recovery.md)) a teljes helyreállítási csomag elmentéséhez.

A sablon ezen felül csatolja a Docker socketet, a flasht (`/boot`) és a **Host Data** gyökeret (`/mnt`) is helyetted. A mentési *források* és *célok* egyaránt a Host Data alatt találhatók. A teljes változó-referenciáért és a telephelyen kívüli beállításért lásd: [Konfiguráció](configuration.md).

## Első futtatás

![A műszerfal az első mentés után: mi védett, mi fut legközelebb, és egy élő napló.](assets/screenshots/dashboard.png)

*A műszerfal az első mentés után: mi védett, mi fut legközelebb, és egy élő napló.*

1. Nyisd meg a webes felületet a `https://<your-unraid-ip>:3443` címen (alapból önaláírt tanúsítvánnyal).
2. A **Beállításokban** engedélyezd a kívánt mentési tartományokat (Konténerek, VM-ek, Flash, Önmentés, Mappák, ZFS-adatkészletek), és válassz egy kiemelőszínt.
3. A **Konténerek** fülön válassz egy konténert, és kattints a **Mentés most** gombra az első visszaállítási pont létrehozásához. A tároló útvonalai alapértelmezetten a `/mnt/user/bombvault/{container,vms,flash,config,files,zfs}` útvonalra mutatnak, és az első mentéskor jönnek létre.
4. Állítsd be az ütemezést a **Beállítások, Ütemezések** alatt. A konténerekhez és VM-ekhez van egykattintásos *Mind az ütemezésbe* lehetőség.

!!! tip "Opcionális: válassz mentési sorrendet"
    Ha egyes konténereket mindig más konténerek előtt kell menteni (például egy adatbázist az azt használó alkalmazás előtt), nyisd meg a **Mentések sorrendje** panelt a Konténerek oldalon, és húzd őket a kívánt sorrendbe. Az ütemezett és a többszörös kijelöléses futások ezt követik; amit rendezetlenül hagysz, azt a korábbi módon a legrégebben esedékes elve szerint menti.

!!! note "Hosztellenőrzés"
    A konténer elindulása után nyisd meg a `/spike` oldalt a webes felületen. Ez minden csatolást és CLI-t megvizsgál (Docker socket, libvirt, restic, qemu-img, rclone), és jelenti a hiányzó darabokat, így megbizonyosodhatsz róla, hogy a konténer helyesen van bekötve, mielőtt rá hagyatkoznál.

## Egyszerű vs Speciális

![A beállításoknak nincs Mentés gombja: minden változás azonnal kiíródik.](assets/screenshots/settings.png)

*A beállításoknak nincs Mentés gombja: minden változás azonnal kiíródik.*

Alapértelmezetten a felület csak a lényeget mutatja (mentés, visszaállítás, ütemezés). Használd az **Egyszerű nézet / Speciális nézet** kapcsolót az oldalsávban a szakértői vezérlők felfedéséhez: megőrzés, telephelyen kívüli másolat, mentés előtti/utáni horgok, fájlszintű visszaállítás, értesítések, Prometheus-metrikák és az integritási/karbantartási eszközök. Ez böngészőnkénti beállítás, és alapból ki van kapcsolva, így az újoncok tiszta felületet, a haladók pedig mindent megkapnak.

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

A felület fordítása a `go run`-hoz is kell. A repó a `web/dist` alatt csak egy üres jelölőfájlt követ, így `npm --prefix web run build` nélkül a bináris semmit sem ágyaz be, és `500 SPA index not found` választ ad, ami várható. A Docker, a libvirt és az Unraid nem tesztelhető CI-ben, ezért mielőtt pull requestet nyitsz, ellenőrizd a csatolásokat, a resticet és a VM SSH-kapcsolatát egy valódi hoszton a Hosztellenőrzéssel (`/spike`).

## Következő lépések

- Böngészd a teljes **[Funkciók](features.md)** oldalt.
- Vidd a csoportod összes szerverét a telefonodra az **[Android-alkalmazással](android.md)**.
- Adj hozzá egy vagy több **[Telephelyen kívüli mentés és helyreállítás](offsite-recovery.md)** replikát (minden tartomány egyszerre több célra is szállíthat), és mentsd el a helyreállítási csomagodat.
- Egy beállítást klónozol, vagy új gépre költözöl? Vidd át a teljes konfigurációdat az **Beállítások exportálása / importálása** kártyával. Lásd: [Konfiguráció](configuration.md#portable-settings-export-and-import).
- Elakadtál? Lásd: **[Hibaelhárítás](troubleshooting.md)**.
