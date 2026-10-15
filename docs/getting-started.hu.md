# Kezdő lépések

Ez az oldal végigvezet egy friss Unraid-géptől az első mentésedig.

## Követelmények

| Követelmény | Megjegyzések |
|---|---|
| **Unraid 6.12+** | A korábbi verziók nincsenek tesztelve. Az Unraid a fő platform, de a BombVault sima Docker-gazdagépen és TrueNAS Scale-en is fut (lásd: [Általános Docker-gazdagép](#generic-docker-host)). |
| **Restic tároló helye** | Helyi útvonal (ajánlott: a tömböd vagy a gyorsítótárad), SMB, NFS vagy bármely rclone backend. |
| **Docker socket** | A sablon automatikusan csatolja (`/var/run/docker.sock`). |
| **Unraid flash** (`/boot`) | A sablon automatikusan, teljes egészében csatolja (`/boot` a `/host/boot` alá). Ez teszi lehetővé a flash-mentést, és azt, hogy egy visszaállított konténer normál, szerkeszthető Unraid-alkalmazásként jelenjen meg újra. |
| **KVM VM-ek** (opcionális) | A VM-mentés SSH-n keresztül kommunikál a libvirttel, nincs libvirt-csatolás. Állítsd be a **Beállítások, Integrációk, Gazdagép SSH** alatt (lásd: [Konfiguráció](configuration.md)). |
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
2. Állítsd be az `APP_KEY` értékét (lásd lent), és irányítsd a Host Data kötetet a valódi adatgyökeredre. A fájl megjegyzései mindkettőt végigvezetik.
3. `docker compose up -d`, majd nyisd meg a `https://<gazdagép-ip>:3443/` címet.

Miben más ez, mint az Unraid:

- **Nincs Flash szakasz.** Nincs indító pendrive, amit el kellene menteni vagy vissza kellene állítani, így a Flash szakasznak itt nincs dolga. Helyette a **Biztonsági mentések** oldalon a **Hozzáadás** felkínálja a **Rendszerkonfiguráció** mappajavaslatot: egy induló `/etc` mappakészletet, amit mentés előtt átnézel és szerkesztesz.
- **Nincsenek Unraid-natív értesítések.** A BombVault saját értesítési csatornái (webhook, külső telephelyi hiba riasztásai és így tovább) a szokott módon működnek; csak az Unraid saját értesítési rendszerébe küldés marad el, mert ilyen rendszer itt nincs.
- **A virtuális gépek mentése választható, és külön, SSH-n elérhető libvirtd gazdagépet igényel.** Lásd a compose fájl kikommentezett blokkját. Egy általános Docker-gazdagépben magában nincs virtuálisgép-kezelő.
- **Nincs irányítópult-widget.** A BombVault Widget egy Unraid-bővítmény, így ez a lépés is kimarad.
- **Egy konténer adatainak megtalálása.** Az Unraid `appdata` konvenciója nélkül a konténer adatmappáját a `DATA_ROOT_SEGMENTS` szegmensei, a Docker elnevezett kötetei, egy Compose-projekt munkakönyvtára és a `bombvault.data` címke alapján találja meg (lásd: [A mentési források felismerése](configuration.md#backup-source-detection)). Az elnevezett kötetek és az `/etc` előbeállítás csak a Host Data csatoláson belüli útvonalakat érik el, ezért a Host Data egy olyan közös szülőkönyvtárra mutasson, amely a Docker adatgyökerét is lefedi.
- **`PLATFORM`.** Állítsd `generic` vagy `truenas` értékre. Ha nincs beállítva, a BombVault az Unraidet a flash-csatoláson lévő saját jelölője alapján ismeri fel, minden mást általánosként kezel, és a csak Unraidre vonatkozó lépéseket kihagyja, ahelyett hogy megpróbálná és elbukna rajtuk.

A **TrueNAS Scale** ugyanezt a compose-utat járja; egy katalógusbejegyzés elő van készítve a repóban, de még nincs beküldve. A VM-mentéshez ott `LIBVIRT_URI` kell, mert a TrueNAS libvirtd-je saját socketen figyel (`/run/truenas_libvirt/libvirt-sock`), amelyet a három `LIBVIRT_*` változó nem tud kifejezni (lásd: [Konfiguráció](configuration.md)). Mennyire bizonyított ez: a BombVault saját zvol-mentése és -visszaállítása SSH-n keresztül futott egy valódi TrueNAS Scale gépen, és minden visszaállítás bájtra pontosan visszaolvasható volt, a helyén és egy korábban törölt zvolon is. Egy teljes VM visszaállítása ott (a domain újradefiniálása és indítása) és a sok gigabájtos áteresztőképesség nincs tesztelve. Mielőtt ráhagyatkoznál, próbálj ki ott egy visszaállítást.

## Az egyetlen kötelező beállítás

Az egyetlen változó, amelyet be kell állítanod, az `APP_KEY`, egy 32 bájtos hexadecimális titok (64 hexadecimális karakter), amely a restic tároló jelszavának származtatására szolgál.

Generálj egyet bármely gépen:

```bash
openssl rand -hex 32
```

Illeszd be az eredményt a sablon `APP_KEY` mezőjébe (Unraid), vagy a `docker-compose.yml` `APP_KEY` környezeti változójába (általános Docker-gép).

!!! danger "Ne veszítsd el az APP_KEY-t"
    Az `APP_KEY` elvesztése visszaállíthatatlanná teszi a titkosított mentéseidet. Tárold biztonságos helyen, a szervertől elkülönítve. Amint a BombVault fut, állíts be bejelentkezési jelszót, és töltsd le a **Helyreállítási csomagot** a **Beállítások, Visszaállítás** alatt (lásd: [Telephelyen kívüli mentés és helyreállítás](offsite-recovery.md#recovery-kit)). Ebben ott van a kulcs, minden cél és a restic parancsai.

A sablon ezen felül csatolja a Docker socketet, a flasht (`/boot`) és a **Host Data** gyökeret (`/mnt`) is helyetted. A mentési *források* és *célok* egyaránt a Host Data alatt találhatók. A teljes változó-referenciáért és a telephelyen kívüli beállításért lásd: [Konfiguráció](configuration.md).

## Első futtatás

![Az áttekintés az első mentés után: mi igényel figyelmet, az egyes szakaszok védelme, a célok és az előzmények.](assets/screenshots/dashboard.png)

*Az áttekintés az első mentés után: mi igényel figyelmet, az egyes szakaszok védelme, a célok és az előzmények.*

1. Nyisd meg a webes felületet a `https://<your-unraid-ip>:3443` címen (alapból önaláírt tanúsítvánnyal). Egy új telepítés az **Üdv a BombVaultban** oldalon nyílik meg, amely felsorolja, mit talált a BombVault a szerveren.
2. Válaszd a **Kezdjük** gombot. Az áttekintésen ekkor megjelenik az **Első lépések** kártya három lépéssel:
    1. **Válaszd ki a szakaszokat**: kapcsold be, amit a BombVault ment (Konténerek, VM-ek, Flash, Mappák, ZFS-adatkészletek, Önmentés). A **Kiválasztás** megnyitja a szakasz elemeinek listáját, ahol az **Ütemezés szüneteltetése** kihagy egyet. Az itt bekapcsolt szakaszok minden éjjel futnak, ezt a **Beállítások, Ütemezés** alatt változtathatod.
    2. **Cél hozzáadása**: ide kerülnek a mentések. Egy helyi mappa elég az induláshoz. A legbiztonságosabb egy második cél telephelyen kívül, például egy Storage Box vagy a Backblaze B2. A helyi tárolók alapértelmezetten a `/mnt/user/bombvault/{container,vms,flash,config,files,zfs}` útvonalra mutatnak, és az első mentéskor jönnek létre.
    3. **Első mentés**: ezután magától fut minden éjjel.
3. Nyisd meg a **Biztonsági mentések** oldalt, ott minden elem egy listában van. Az **Összes mentése most** a bekapcsolt szakaszok mindegyikén elindít egy futást, egy elem oldalán a **Biztonsági mentés most** csak azt az egyet menti.

Használtad már a BombVaultot? A **Visszaállítás mentésből** az üdvözlő oldalon megnyitja a Visszaállítás oldalt, ahol a **Vészvarázsló** visszahozza egy elveszett szerver beállításait, konténereit, VM-jeit és mappáit. Lásd: [Telephelyen kívüli mentés és helyreállítás](offsite-recovery.md#emergency).

!!! tip "Opcionális: válassz mentési sorrendet"
    Ha egyes konténereket mindig más konténerek előtt kell menteni (például egy adatbázist az azt használó alkalmazás előtt), nyisd meg a **Sorrend** kártyát a **Beállítások, Ütemezés** alatt, és húzd őket a kívánt sorrendbe. Az ütemezett futások és a teljes mentés ezt követik; amit rendezetlenül hagysz, azt a legrégebben esedékes elve szerint menti.

!!! note "Hosztintegráció-ellenőrzés"
    Az áttekintés **Hosztintegráció** kártyája minden csatolást és CLI-t megvizsgál (Docker socket, libvirt, restic, qemu-img, rclone), és jelenti a hiányzó darabokat, így megbizonyosodhatsz róla, hogy a konténer helyesen van bekötve, mielőtt rá hagyatkoznál. A **Testreszabás** alatt vár, amíg hozzáadod.

## Eligazodás {#navigation}

![A beállítások csempékként, mellettük a kiválasztott csempe kártyái. Minden változás azonnal kiíródik.](assets/screenshots/settings.png)

*A beállítások csempékként, mellettük a kiválasztott csempe kártyái. Minden változás azonnal kiíródik.*

Az oldalsávnak öt eleme van: **Áttekintés**, **Anomáliák** (amíg az anomáliafelismerés be van kapcsolva), **Biztonsági mentések**, **Visszaállítás**, és a **Kijelentkezés** alatt a **Beállítások**. Egy oldal fő művelete lebeg a jobb alsó sarokban, a ritka műveleteket a **Több** gyűjti. Telefonon a sáv az Áttekintést, a Biztonsági mentéseket, a Visszaállítást és a Beállításokat tartalmazza, a többit a **Több** adja.

Az oldalak és a beállítások az ablak teljes szélességét használják. Egy ablak a gombjait a láblécében tartja, az előre vivő gombot jobb oldalon. Telefonon a háromnál több gombos lábléc a kiutat és a fő műveletet megtartja, a többit a **Több** alá teszi.

- A **Biztonsági mentések** elemekként listázza az összes konténert, VM-et, mappakészletet, a flasht, minden ZFS-adatkészletet és az Önmentést. A lista feletti csempék fajtát választanak, a **Szűrők**, a **Rendezés** és a **Keresés** szűkít. Minden elemnek saját oldala van három füllel: **Biztonsági mentések**, **Beállítások** és **Ellenőrzés**. Az elem ikonjától balra lévő visszagomb a listához vezet.
- A **Visszaállítás** egyetlen oldal mindenhez: válassz szakaszt, keress egy elemet, és válassz egy mentést. A Vészvarázsló és a többi forrás is innen indul.
- A **Beállítások** tizennégy csempe: Általános, Megjelenés, Ütemezés, Célok, Visszaállítás, Értesítések, Ellenőrzés, Biztonság, Példányok, Párosítás, Integrációk, Alkalmazások, Rendszer és Információ. A `/` vagy a Ctrl+K lenyomásával mindben kereshetsz, a korábbi verziókban használt oldalnevekre is. A csempéket a **Megjelenés** alatt húzhatod saját sorrendbe. Ahol az oldal melletti csempeoszlop összeszorítaná az oldalt, például egy kb. 1200 pixel széles ablakban, a csempék az oldal fölötti sávvá válnak, amely oldalra görgethető. A telefon ugyanezt a sávot mutatja.
- A **Célok** és a **Példányok** beállítási csempék, saját oldalakkal. A csempéjükön a **Mutatás az oldalsávban is** kitűzi őket az oldalsávra.

Semminek nincs Mentés gombja, és semmi sem hajlik szét: a ritka dolgoknak saját kártyájuk vagy ablakuk van, a magyarázatok pedig az (i) buborékokban vannak. Frissítés után a kiadási megjegyzések maguktól megnyílnak, és zöld pontok jelzik, mi változott. Egy kattintás elrejti a pontot. Az **Újdonságok** a **Beállítások, Információ** alatti Verzió kártyán megszámolja, mit nem láttál még, és megnyitja a kiadási megjegyzéseket és az összes változás listáját az **Ugrás oda** gombbal, egy ablak két füleként.

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

## Következő lépések

- Böngészd a teljes **[Funkciók](features.md)** oldalt.
- Vidd a csoportod összes szerverét a telefonodra az **[Android-alkalmazással](android.md)**.
- Adj hozzá egy vagy több telephelyen kívüli célt (minden szakasz egyszerre több célra is másolhat), és mentsd el a helyreállítási csomagodat, ahogy a **[Telephelyen kívüli mentés és helyreállítás](offsite-recovery.md)** leírja.
- Egy beállítást klónozol, vagy új gépre költözöl? Vidd át a teljes konfigurációdat a **Beállítások exportálása / importálása** kártyával. Lásd: [Konfiguráció](configuration.md#portable-settings-export-and-import).
- Elakadtál? Lásd: **[Hibaelhárítás](troubleshooting.md)**.
