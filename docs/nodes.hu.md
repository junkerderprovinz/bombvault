# Csomópontok

A csomópont egy webes felület nélküli BombVault, például egy VPS-en vagy egy barátodnál álló kis szerveren. A saját szerverén lévő konténereket és mappákat menti, azt pedig, hogy mikor mentsen és hová kerüljenek a mentések, az a BombVault mondja meg neki, amelyiken hozzáadod. Ezt ezen az oldalon **anyapéldánynak** hívjuk. A csomópontot az anyapéldány webes felületéről kezeled, a saját elemeid mellett.

![A vps-falkenstein csomópont kártyája a Példányok oldalon: Csatlakozva, a verziója, a mentései a fogadószerveren, hogy melyik szakasz mikor ment, valamint a Cél módosítása, a Csere és az Elengedés.](assets/screenshots/nodes/card-dark.png#only-dark)
![A vps-falkenstein csomópont kártyája a Példányok oldalon: Csatlakozva, a verziója, a mentései a fogadószerveren, hogy melyik szakasz mikor ment, valamint a Cél módosítása, a Csere és az Elengedés.](assets/screenshots/nodes/card-light.png#only-light)

## Hogyan működik {#how-it-works}

- **Az anyapéldány tartja a csomópont kulcsát.** Amikor hozzáadsz egy csomópontot, az anyapéldány elkészíti az `APP_KEY` kulcsát, megtart belőle egy lezárt másolatot, és egyszer odaadja neked, a csomópont compose fájljában. Ezzel a másolattal éri el az anyapéldány a csomópontot, és ezzel nyitja meg a mentéseit.
- **A csomópontnak nem kell nyitott port.** Az anyapéldány és a csomópont a relayen át, egy saját, privát kapcsolaton beszél, a párosítási csoportodtól külön. Minden hívást az anyapéldány kezd és ír alá, a csomópont pedig a köszönésén kívül semmit sem kér tőle. Az egyetlen cél, amelyhez nyitott port kell, [a mentések megtartása a csomóponton](#keep-on-node).
- **A mentések soha nem mennek át a relayen.** A csomópont egyenesen oda küldi őket, ahová választod: az anyapéldány fogadószerverére, az anyapéldány egyik mentési céljába vagy egy rest-serverre magán a csomóponton.
- **A csomópont maga semmit sem ütemez.** Nem futtat ütemezést, nem küld értesítést, és nem lép be semmilyen csoportba vagy erődbe. Az anyapéldány indítja a mentéseit, figyeli, hogyan mennek, és küldi a riasztásokat.
- **A csomópont konténereket és mappakészleteket ment.** A VM-ek, a flash, a ZFS-adatkészletek és az önmentés a teljes telepítéseknél maradnak.

A kapcsolat az anyapéldány saját relayét használja, ha a csoportjának van ilyen, egyébként a projekt relayét, akkor is, ha az anyapéldány relay nélkül párosít.

## Mielőtt elkezded {#before-you-start}

- **Bejelentkezési jelszó az anyapéldányon.** A csomópont hozzáadásához, a compose fájljának újbóli megjelenítéséhez és az elemei megnyitásához jelszavas bejelentkezés kell, lásd: [Bejelentkezés](configuration.md#login).
- **Docker és Compose a csomópont szerverén.**
- **Hely a mentéseknek**, a [Hová kerülnek a mentések](#placement) alatti három közül az egyik. A fogadószerver a legbiztonságosabb választás, mert ott a csomópont semmit sem tud törölni.

## Csomópont hozzáadása {#add}

1. Az anyapéldányon nyisd meg a **Példányok** oldalt, és válaszd jobb alul a **Csomópont hozzáadása** gombot.
2. **Név.** Írd be, milyen néven jelenjen meg itt a csomópont, például a szerver nevén, és válaszd a **Tovább** gombot.

    ![A Csomópont hozzáadása ablak első lépése, beírt vps-helsinki névvel.](assets/screenshots/nodes/add-name-dark.png#only-dark)
    ![A Csomópont hozzáadása ablak első lépése, beírt vps-helsinki névvel.](assets/screenshots/nodes/add-name-light.png#only-light)

3. **Mentések.** Válaszd ki, hová kerüljenek a mentések, és töltsd ki, amit az a hely kér. A **Naponta ment ekkor:** az az időpont, amikor mindkét szakasz elindul. Az anyapéldány fél órával a saját konténermentései utánra javasolja, hogy a kettő ne egyszerre nyúljon ugyanazokhoz a lemezekhez. Később a kártyán minden szakasznak saját tervet adhatsz. Válaszd a **Tovább** gombot.

    ![A második lépés: kiválasztott fogadószerver, https-címmel kitöltött Cím a csomópontoknak, és Naponta ment ekkor: 03:00.](assets/screenshots/nodes/add-placement-dark.png#only-dark)
    ![A második lépés: kiválasztott fogadószerver, https-címmel kitöltött Cím a csomópontoknak, és Naponta ment ekkor: 03:00.](assets/screenshots/nodes/add-placement-light.png#only-light)

4. **Indítás.** Az ablak megmutatja a csomópont compose fájlját, kitöltött `MODE`, `APP_KEY`, `NODE_HOME` és `NODE_NAME` értékkel. Másold ki, és az `APP_KEY` kulcsot tedd biztos helyre. A csomópont szerverén:

    ```sh
    mkdir -p /opt/bombvault-node && cd /opt/bombvault-node
    nano docker-compose.yml   # paste the file from the window
    docker compose up -d
    ```

    Indítás előtt irányítsd a Host Data kötetet a menteni kívánt adatokra, és állítsd a `HOST_SOURCE_ROOT` értékét ugyanarra az útvonalra, ahogy a lenti példa mutatja.

5. Az ablak vár a csomópontra. Amint a csomópont válaszol, az anyapéldány megmondja neki, hová kerülnek a mentései, és ellenőrzi, hogy eléri-e őket. Ekkor az ablakban ez áll: **vps-helsinki online, és ettől a példánytól kapja az utasításait.**

    ![Az utolsó lépés: a compose fájl, majd az online csomópont és a fogadószerverre irányított mentései.](assets/screenshots/nodes/add-start-dark.png#only-dark)
    ![Az utolsó lépés: a compose fájl, majd az online csomópont és a fogadószerverre irányított mentései.](assets/screenshots/nodes/add-start-light.png#only-light)

Az ablakot bezárhatod, mielőtt a csomópont válaszol. A kártyáján ilyenkor a **Várakozás a csomópontra** áll, és az anyapéldány közli vele, hová kerülnek a mentései, amint megjelenik.

### Példa: egy VPS, amely a fogadószerverre ment {#example-vps}

Az anyapéldány egy `tower` nevű Unraid-gép, amely a [fogadószervert](offsite-recovery.md#receiving-server) futtatja. Egy fordított proxy `https://backup.tower.example.org` címen teszi elérhetővé. A `vps-helsinki` VPS Nextcloudot és Caddyt futtat, az adataik a `/srv/appdata` alatt vannak.

1. A `tower` gépen: **Példányok**, **Csomópont hozzáadása**, a név `vps-helsinki`, **Tovább**.
2. Válaszd az **Ennek a példánynak a fogadószervere** lehetőséget, és a **Cím a csomópontoknak** mezőbe írd be: `https://backup.tower.example.org`. Az anyapéldány konténerei 02:30-kor mentenek, ezért az ablak 03:00-t kínál. Hagyd így, és válaszd a **Tovább** gombot.
3. A VPS-en mentsd el az ablakban látható fájlt úgy, hogy a kötetek és a `HOST_SOURCE_ROOT` a `/srv/appdata` útvonalra mutassanak:

    ```yaml
    services:
      bombvault-node:
        image: junkerderprovinz/bombvault:latest
        restart: unless-stopped
        hostname: bombvault-node
        container_name: bombvault-node
        volumes:
          - ./config:/config
          - /var/run/docker.sock:/var/run/docker.sock
          - /srv/appdata:/host/user
        environment:
          MODE: node
          APP_KEY: "<filled in by home>"
          NODE_HOME: "<filled in by home>"
          NODE_NAME: "vps-helsinki"
          PLATFORM: generic
          HOST_SOURCE_ROOT: /srv/appdata
    ```

4. `docker compose up -d`. Néhány másodperc múlva a `tower` ablaka jelzi, hogy a csomópont online, és **A biztonsági mentések ide kerülnek: https://backup.tower.example.org.**
5. 03:00-kor az anyapéldány elindítja a csomópont mentéseit. Másnap reggel az áttekintés **Csomópontok** kártyája mutatja a `vps-helsinki` utolsó mentését, a **Biztonsági mentések** pedig külön szakaszban sorolja fel a konténereit.

Hogy egy sima Docker-hoszton hogyan találja meg a BombVault egy konténer adatait, azt [A mentési források felismerése](configuration.md#backup-source-detection) írja le.

## Hová kerülnek a mentések {#placement}

Minden csomópontnak egyetlen célja van mindkét szakaszához. A kártyáján a **Cél módosítása** megváltoztatja, ha a csomópont online. Az új hely új tárolókat kap, a régi helyen lévő mentések pedig ott maradnak, ahol vannak.

### Ennek a példánynak a fogadószervere {#to-receiver}

A csomópont append-only módban ír az anyapéldány fogadószerverére: mentéseket hozzáadhat, de egyet sem törölhet. Saját, a csomópontról elnevezett bejelentkezést kap ott, és szakaszonként egy tárolót a saját mappájában. Az anyapéldány minden tárolóhoz hozzáadja a saját kulcsát, és maga tartja rendben őket, mert a csomópont erre nem képes.

A csomópont a **Cím a csomópontoknak** címen éri el a szervert, amelyet a második lépésben vagy a **Fogadószerver** ablakban állítasz be. Egy csomópont általában az internetről küld, ezért a cím ezek egyike legyen:

- `https://`, például fordított proxy, Pangolin vagy alagút mögött. Ha a szerver nyilvános hitelesítésszolgáltatótól kapott tanúsítvány helyett sajátot mutat, illeszd be PEM formában a **Tanúsítvány** mezőbe.
- `http://` privát IP-címmel, például `10.0.0.5` egy WireGuard-kapcsolaton vagy `100.101.102.103` Tailscale-en.

A sima `http://` nyilvános címre el van utasítva, mert a csomópont titkosítatlanul küldené a mentéseit az interneten át. Ha megváltoztatod a címet, az anyapéldány szól minden csomópontnak a fogadón, az éppen távol lévő csomópont pedig akkor értesül, amikor visszatér.

### Egy cél {#to-location}

A csomópont az anyapéldány egyik olyan mentési céljába ír, amelyet maga is elér: egy S3-szolgáltatásba vagy egy REST-szerverre, `https://` címen vagy privát címen. A tárolói ott a `bombvault-node-<node id>-containers` és a `bombvault-node-<node id>-files` alatt vannak.

A csomópont egy neki készült [mentett bejelentkezéssel](offsite-recovery.md#sign-ins) jelentkezik be, amelynek különböznie kell attól, amellyel az anyapéldány a célba ír. Ha az ablak szerint nincs ilyen, előbb vegyél fel egyet. Ezzel a bejelentkezéssel a csomópont a célban általában törölni is tudja, ami ott van, a fogadószerverrel ellentétben, és ezt az ablak jelzi is.

### Maradjanak a csomóponton {#keep-on-node}

A csomópont maga őrzi a mentéseit, egy saját, append-only módú rest-server mögött, az anyapéldány pedig minden szakaszról másolatot hív le a saját tárolójába. Ez annak a csomópontnak való, amelynek az anyapéldány nem tud helyet kínálni. Ez az egyetlen cél, amelyhez nyitott port kell.

![A második lépés a kiválasztott Maradjanak a csomóponton lehetőséggel, a https://203.0.113.7:8443 címmel és a nyitott portról szóló megjegyzéssel.](assets/screenshots/nodes/add-keep-dark.png#only-dark)
![A második lépés a kiválasztott Maradjanak a csomóponton lehetőséggel, a https://203.0.113.7:8443 címmel és a nyitott portról szóló megjegyzéssel.](assets/screenshots/nodes/add-keep-light.png#only-light)

1. **A csomópont címe** mezőbe írd be, hol éri el az anyapéldány a csomópontot: a nyilvános címét és egy szabad portot, például `https://203.0.113.7:8443`, mappa nélkül a végén. A `http://` címet csak privát címmel használd, ahogy fent.
2. A csomópont ezen a porton elindítja a `bombvault-node-rest-server` konténert, a tárolóit a Host Data alatti `bombvault-node` mappában tartva. `https://` esetén tanúsítványt készít erre a címre, az anyapéldány pedig rögzíti.
3. Engedd át a portot a csomópont tűzfalán, hogy az anyapéldány elérje. Ha a portot egy másik konténer foglalja a csomóponton, a BombVault elutasítja, és megnevezi a konténert.

Az anyapéldány egy szakaszt rögtön azután hív le, hogy a csomóponton véget értek a mentései, tartalékként pedig óránként negyed után, akkor is, ha a **Lehívás** ki van kapcsolva. Amit az anyapéldány lehívott, az nála marad, mint bármely [lehívott másolat](offsite-recovery.md#receiver), akkor is, ha a csomópont már nincs meg. A csomópont kérésre maga tartja rendben a tárolóit, mert a rest-servere nem fogad törlést, a mappakészletei pedig kihagyják a `bombvault-node` mappát, így soha nem menti a saját mentéseit.

## Mikor mentenek a csomópontok {#schedule}

Minden csomópont ütemezése az anyapéldányé. A kártya szakaszonként mutat egy tervet, például **Konténerek** *naponta 2:00-kor* és **Mappák** *hetente (V) 4:00-kor*. Egy kattintás ugyanazt a szerkesztőt nyitja meg, mint az [Ütemezések](schedules.md#plans) alatt, és a módosítás azonnal mentődik.

![Egy csomópont konténereinek terve a kártyájáról megnyitva, Ki, Naponta, Hetente, N naponta és Cron lehetőséggel.](assets/screenshots/nodes/schedule-dark.png#only-dark)
![Egy csomópont konténereinek terve a kártyájáról megnyitva, Ki, Naponta, Hetente, N naponta és Cron lehetőséggel.](assets/screenshots/nodes/schedule-light.png#only-light)

- **A távol lévő csomópont**, amint visszatér, elindítja a kimaradt mentéseit. Az anyapéldány ötpercenként néz rá.
- **A foglalt csomópontot** öt perc múlva újra megkérdezi.
- **Amíg az anyapéldány rendet rak** egy csomópont régi mentései között, a csomópont semmit sem indít, amíg az anyapéldány el nem készül. A fogadószerveren és magán a csomóponton ez az időablak minden vasárnap 03:00-kor nyílik, és legfeljebb hat óráig tart. Egy célnál az anyapéldány ahhoz a célhoz tartozó telephelyen kívüli másolási tervét követi. Az anyapéldány a mentéseket a szakasz saját szabálya szerint őrzi meg a **Meddig őrizze meg** alatt, vagy a cél szabálya szerint, ha annak van ilyen.
- **A csomópont futáslistájában** az anyapéldány által indított mentések és visszaállítások az anyapéldány indításaként jelennek meg.

## A csomópontok figyelése {#watch}

Az áttekintés **Csomópontok** kártyája minden csomópontot felsorol az állapotával, az utolsó mentésével, a hibáival és a nyitott anomáliáival. Akkor jelenik meg, amikor már van csomópontod.

![Az áttekintés Csomópontok kártyája három csomóponttal: egy csatlakozva, egy inaktív két hibával, egy pedig még várakozik a csomópontra.](assets/screenshots/nodes/overview-card-dark.png#only-dark)
![Az áttekintés Csomópontok kártyája három csomóponttal: egy csatlakozva, egy inaktív két hibával, egy pedig még várakozik a csomópontra.](assets/screenshots/nodes/overview-card-light.png#only-light)

Az anyapéldány epizódonként egy riasztást küld az [értesítési csatornáin](notifications.md), a következőt pedig csak akkor, ha az az epizód véget ért:

- **Egy mentés sikertelen**, például *BombVault: vps-helsinki: backup failed*, az elemmel és az okkal. Az epizódot az elem egy későbbi, sikeres mentése zárja le.
- **A csomópont nem érhető el**, például *BombVault: vps-helsinki is out of reach*. Az anyapéldány a csomópont tárolóin beállított legrövidebb holtember-kapcsoló idejéig vár, ami 26 óra, ha nem módosítod, vagy 6 óra, amíg a csomópontnak még nincs célja.

Az anyapéldány fogadófigyelése a csomópont minden tárolójára is ügyel, holtember-kapcsolóval és napi `restic check` futtatással 04:00-kor, lásd: [Fogadó és lehívó](offsite-recovery.md#receiver).

## Egy csomópont elemei {#entries}

A **Biztonsági mentések** alatt a Konténerek és a Mappák lista a saját elemeid alatt minden csomópontnak külön szakaszt ad. A csomópont saját konténere kimarad. Egy elem menüjében a **Biztonsági mentés most** elindítja a mentését a csomóponton, a **Megnyitás** pedig megnyitja az oldalát.

Egy csomópont szakaszában a Mappák listában a **Mappakészlet hozzáadása** mappakészletet ad a csomóponthoz, egy készlet menüjében a **Szerkesztés** pedig módosítja azt. Mindkettő ugyanazt az ablakot nyitja meg, mint a saját készleteidnél. Az ablak mappaválasztója a csomópont mappáit böngészi, de új mappát nem hoz létre ott.

![A vps-falkenstein csomópont szakasza a Konténerek listában: nextcloud, nextcloud-db, caddy és uptime-kuma.](assets/screenshots/nodes/backups-section-dark.png#only-dark)
![A vps-falkenstein csomópont szakasza a Konténerek listában: nextcloud, nextcloud-db, caddy és uptime-kuma.](assets/screenshots/nodes/backups-section-light.png#only-light)

Egy csomóponti elem oldalán az **Itt:** felirat és a csomópont neve áll, alatta a mentései. Az egyik melletti **Visszaállítás** a csomóponton állítja vissza az elemet, és lecseréli azt, ami most ott van. A konténer ehhez leáll, ugyanúgy, mint az anyapéldányon.

![A nextcloud oldala a vps-falkenstein csomóponton, a mentéseivel és mindegyik mellett egy Visszaállítás gombbal.](assets/screenshots/nodes/entry-dark.png#only-dark)
![A nextcloud oldala a vps-falkenstein csomóponton, a mentéseivel és mindegyik mellett egy Visszaállítás gombbal.](assets/screenshots/nodes/entry-light.png#only-light)

Az anyapéldányon át a csomópont saját útvonalainak egy rögzített listáját éred el: a konténereit, mappakészleteit, futásait és anomáliáit, valamint ezek mentését és visszaállítását. A csomópont beállításai, kulcsai és tárolója elérhetetlenek maradnak, és a listán semmi sem töröl. Lásd: [Csomópontok az API-ban](api.md#nodes).

## Csere és elengedés {#replace-release}

A **Csere** jelszavas bejelentkezés után újra megmutatja a csomópont compose fájlját. Akkor használd, ha másik szerverre viszed vagy újraépíted a csomópontot: indítsd el ott a fájlt, és állítsd le a régi szervert. Ugyanaz az `APP_KEY` és `NODE_HOME` ugyanazzá a csomóponttá teszi újra, a mentéseivel és az előzményeivel együtt. Az anyapéldány minden csomóponttól megőrzi a mappakészleteit, a konténereit mindazzal, ami az újbóli létrehozásukhoz kell, és a futásait. Ha a csomópont üres `config` mappával indul, az anyapéldány újra magához veszi, megmondja neki, hová mennek a mentései, és a következő mentés előtt mindezt visszaadja neki, így a csomópont egy már nem létező konténert is vissza tud állítani. Új szerveren az a csomópont, amely maga őrzi a mentéseit, üres tárolókkal kezd. A korábbi mentései az anyapéldány által lehívott másolatokban maradnak. Két szerver ugyanazzal az `APP_KEY` kulccsal egy csomópontnak számít, ezért soha ne futtasd mindkettőt.

Az **Elengedés** elengedi a csomópontot. Többé nem fogad utasítást az anyapéldánytól, és semmit sem ment. A mentései ott maradnak, ahol vannak, ahogy minden másolat is, amelyet az anyapéldány lehívott. A bejelentkezése a fogadószerveren megszűnik, és az anyapéldány már nem figyeli a tárolóit. Ha a csomópont távol van, amikor elengeded, a kártyája **Elengedés folyamatban** állapotban marad, amíg értesül róla. Az elengedett csomópont ugyanazzal az `APP_KEY` kulccsal nem fogad el újra anyapéldányt. Ha újra használnád a szervert, add hozzá új csomópontként, és indítsd az új compose fájllal és egy üres `config` mappával.

## Biztonság {#security}

Ha valaki átveszi az anyapéldányt, a csomópontjait is irányítja, a csomópont pedig kezében tartja a szervere Dockerét, ami ott root jogot jelent. Védd az anyapéldányt jelszóval, és tartsd távol az internettől. Ha valaki egy csomópontot vesz át, nem kapja meg a párosítási csoportot, az erődöt, más csomópontot vagy más tárolót, és a fogadószerveren nem tud előzményeket törölni. A részletek a [Csomópontok a biztonsági modellben](configuration.md#nodes) alatt vannak.

A [helyreállítási csomag](offsite-recovery.md#recovery-kit) **Csomópontok** fejezete tartalmazza minden csomópont `APP_KEY` kulcsát, restic-jelszavát és tárolóit, így a csomópontot vagy a mentéseit sima restickel is visszahozhatod.
