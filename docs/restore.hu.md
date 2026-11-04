# Visszaállítás

Minden visszaállítás a **Visszaállítás** oldalon indul: válassz egy szakaszt, keresd meg, amit vissza szeretnél kapni, és válassz egy mentést. Egyetlen ablak állít vissza minden fajta elemet, és lépésenként kérdez. Alapértelmezés szerint az eredeti nem változik meg, mert a mentés mellé kerül vissza.

![A Visszaállítás oldal a szakaszokkal csempékként, a kereséssel és a Stackek kártyával.](assets/screenshots/restore/page-dark.png#only-dark)
![A Visszaállítás oldal a szakaszokkal csempékként, a kereséssel és a Stackek kártyával.](assets/screenshots/restore/page-light.png#only-light)

## A Visszaállítás oldal {#page}

- A **Szakaszok** csempékként sorolja fel a szakaszokat, mindegyiknél azzal, hány elemnek vannak pillanatképei.
- A **Mit szeretnél visszahozni?** a kiválasztott szakasz elemei között keres. Minden sor a legújabb pillanatképét mutatja, és az **Elem megnyitása** az elem oldalára vezet.
- A **Stackek** azokat a Compose-projekteket sorolja fel, amelyekben legalább két lementett konténer van, például az `immich` az adatbázisával és a Redisszel. A keresés a stacket a nevéről vagy bármelyik konténeréről megtalálja.
- A **Most fut** egy visszaállítást mutat, amíg fut, akkor is, ha bezártad az ablakát.
- A **Vészhelyzet** elindítja a Vészhelyzet varázslót egy olyan szerverhez, amely elveszett vagy újra lett felállítva, lásd: [Vészhelyzet varázsló](offsite-recovery.md#emergency).
- A **Más források** tartalmazza a **Régi mentési mappából** és az **Importálás az Appdata.Backup bővítményből** lehetőséget, egy másik BombVault mentéseihez, illetve a bővítményéhez.

![A Visszaállítás oldal telefonon.](assets/screenshots/restore/page-phone-dark.png#only-dark)
![A Visszaállítás oldal telefonon.](assets/screenshots/restore/page-phone-light.png#only-light)

## A visszaállítási ablak {#window}

Egy elemre kattintva megnyílik a **Visszaállítás** a nevével. Lépésenként kérdez, és minden lépés megváltoztatható az indítás előtt:

1. **Biztonsági mentés**: az ajánlott pillanatkép előre ki van választva. Ez a legújabb, vagy egy nyitott anomália után az azt megelőző utolsó. A régebbiek a listában vannak, és az **Összehasonlítás** megmutatja, miben különbözik kettő.
2. **Forrás**: honnan jönnek a fájlok, ha egy pillanatkép több helyen is megvan. A **Helyi** a leggyorsabb.
3. **Mit**: **Összes**, vagy **Egyes fájlok**, ahol bepipálod a kívánt fájlokat és mappákat.
4. **Hová**: a cél. A lehetőségek az elem fajtájától függnek:

    | Cél | Mit csinál |
    |---|---|
    | **Az eredeti mellé** | Az alapértelmezés. Egy konténer második konténerként tér vissza `<name>-restored` néven, saját portokkal, egy VM lemezei és NVRAM-ja egy `<folder>-restored` nevű mappába kerülnek, egy mappakészlet pedig `<path>-restored` néven jön vissza. Az eredeti nem változik meg. |
    | **Eredeti cseréje** | Felülírja, ami most ott van. Az **Előbb a jelenlegi állapot mentése** esetén, amely alapértelmezés szerint be van kapcsolva, a BombVault először lementi a mai állapotot, hogy vissza lehessen menni. Ha ez a mentés sikertelen, a visszaállítás nem indul el. |
    | **Egy mappába** | Csak a fájlok, böngészéshez. |
    | **Letöltés ZIP-ként** | A flash, egy új pendrive-hoz. A BombVault soha nem ír a futó flash-re. |
    | **Új adatkészletbe** és **Visszaállítás az adatkészletbe** | ZFS-adatkészletek, lásd: [ZFS-adatkészletek](zfs-datasets.md#restore). |

5. **Mi történik** szavakkal elmondja, mit fog tenni a visszaállítás, és egy jelenet mutatja a választott célt.

Mielőtt a **Visszaállítás** gomb elérhetővé válik, az ablak ellenőrzi, hogy a tároló válaszol, a kulcs megnyitja, a pillanatkép megvan, és a célon van elég hely. Amíg egy ellenőrzés nem sikerül, a gomb zárva marad, és az (i) benne megmondja, melyik.

Az ablak azt is megmutatja, mit változtat a visszaállítás ahhoz képest, ami most ott van: új, lecserélt és változatlan fájlokat, kérésre a listával, valamint azokat a fájlokat a célon, amelyek nincsenek a mentésben, és a helyükön maradnak. A restic ezt szárazfutásként számolja ki a méretből és a módosítási időből, a fájlok olvasása nélkül, és egy nagyon nagy fa 30 másodperc után megáll, és ezt kiírja. Konténereknél és VM-eknél összeveti azokat a beállításokat, amelyeket a visszaállítás újra létrehoz, a futókkal: image és címke, portok, változónevek és kötetek, illetve memória, vCPU-k, lemezek és hálózat.

Néhány dolog megállíthat egy visszaállítást, mielőtt bármit megváltoztatna. Egy konténer statikus IP-címének és közzétett portjainak szabadnak kell lenniük, a hálózatának pedig léteznie kell. A helyben történő visszaállítás megnevezi minden más konténert, amelynek bind mountja olyan mappába nyúlik, amelybe ír, például „ezt az útvonalat a nextcloud-db is használja”, de ez csak figyelmeztet.

Néhány lehetőség ott jelenik meg, ahol értelme van: **Utána hagyd leállítva**, hogy megnézhess egy konténert vagy VM-et, mielőtt újra fut, és **Visszaállítás GPU és futtatókörnyezet nélkül** egy olyan hoszton, amelyen nincs meg a GPU vagy a futtatókörnyezet, amellyel a mentés készült.

A visszaállítás a háttérben fut. Bezárhatod az ablakot, és a haladás a **Most fut** alatt is látszik. A hosszú visszaállítás százalékot mutat, és megszakítható, a megszakított visszaállítás pedig megszakítottként van rögzítve, nem sikertelenként.

![A visszaállítási ablak a Mit lépésnél, egyes fájlok bepipálva.](assets/screenshots/restore/window-what-dark.png#only-dark)
![A visszaállítási ablak a Mit lépésnél, egyes fájlok bepipálva.](assets/screenshots/restore/window-what-light.png#only-light)

### Hová kerül egy mappába történő visszaállítás {#folders}

A **Beállítások, Visszaállítás** alatti **Visszaállítási mappák** állítja be az **Alapértelmezett visszaállítási mappát**, kívánság szerint **Almappa szakaszonként** beállítással, és egy szakaszhoz **Kivételt**. A kiválasztott fájlok közvetlenül a célmappába kerülnek: egy fájl csak maga a fájl, egy mappa a mappa a tartalmával, a különböző mappákból kiválasztottak pedig csak az útvonalukat tartják meg a közös mappájuk alatt. A BombVault visszautasít egy olyan célt, amely nem csatolt tárhelyen van, mert Unraidon egy ilyen mappa a memóriában él, és újraindítás után eltűnik, és soha nem állít vissza a saját adatmappájába.

## Példa: egy fájl visszahozása {#example-single-file}

Valaki tegnap éjjel átszerkesztette a Nextcloud `config.php` fájlját, és a Nextcloud többé nem indul. A 03:00-s éjszakai mentésben még megvan a jó fájl. Azt az egy fájlt szeretnéd, nem az egész konténert.

1. Nyisd meg a **Visszaállítás** oldalt, válaszd a **Konténerek** csempét, és írd be a `nextcloud` szót a **Mit szeretnél visszahozni?** mezőbe.
2. Kattints a `nextcloud` elemre. Az ablak megnyílik, a 03:00-s mentés ajánlottként előre kiválasztva.
3. Hagyd a **Forrás** értékét **Helyi**-n.
4. A **Mit** alatt válaszd az **Egyes fájlok** lehetőséget. Nyisd meg a mappákat a `config`-ig, és pipáld be a `config.php` fájlt.
5. A **Hová** alatt válaszd az **Egy mappába** lehetőséget, és adj meg egy mappát, például `user/restore`, ami a hoszton `/mnt/user/restore`.
6. Olvasd el a **Mi történik** részt: a fájl a célmappába kerül, és a konténeren semmi nem változik. Nyomd meg a **Visszaállítás** gombot.

Az ablak ezzel zárul: **Kész: a fájlok itt vannak: /mnt/user/restore**. A fájlt `/mnt/user/restore/config.php` néven találod, a fölötte lévő mappák nélkül. Hasonlítsd össze a tönkrement fájllal, másold vissza az appdata mappába, és indítsd újra a Nextcloudot.

Ha inkább rögtön a helyére tennéd vissza, az 5. lépésben válaszd az **Eredeti cseréje** lehetőséget. Hagyd bekapcsolva az **Előbb a jelenlegi állapot mentése** beállítást, hogy a BombVault írás előtt lementse a tönkrement állapotot, a megerősítéshez írd be a `nextcloud` szót, és nyomd meg a **Visszaállítás** gombot. A Nextcloud addig áll, amíg a kiválasztott fájl a helyére kerül, és csak azok a fájlok íródnak, amelyeket bepipáltál.

![A visszaállítási ablak a Mi történik lépésnél, az Eredeti cseréje kiválasztva.](assets/screenshots/restore/window-replace-dark.png#only-dark)
![A visszaállítási ablak a Mi történik lépésnél, az Eredeti cseréje kiválasztva.](assets/screenshots/restore/window-replace-light.png#only-light)

## Mi tér vissza, fajtánként {#kinds}

A **konténerek** a mentett definíciójukból jönnek létre újra a Docker API-n keresztül, az entrypointjukkal, hostnevükkel, leállítási beállításaikkal, köteteikkel és csatolásaikkal, erőforrás-korlátaikkal, naplómeghajtójukkal, DNS-beállításaikkal és régi linkjeikkel. A konténer arra az image-re tér vissza, amelyen a mentés idején futott, nem egy később letöltött újabbra, a hiányzó image-et pedig előbb letölti, a **Beállítások, Visszaállítás** alatti **Container registry-k** bejelentkezésével. A GPU-ja vagy futtatókörnyezete (`--gpus`, `--runtime=nvidia`) szintén visszatér. Olyan hoszton, amelyen nincs meg az az illesztő vagy futtatókörnyezet, a visszaállítás ezt közli, és felkínálja a **Visszaállítás GPU és futtatókörnyezet nélkül** lehetőséget. Egy hiányzó, vagy a visszaállított indulásakor leállított konténerre mutató link kimarad, és ezt a futási előzmények közlik. Az a visszaállítás, amely a konténer eltávolítása után hibázik, visszateszi azt, leállított állapotban.

A **VM-ek** XML-jét újra definiálja SSH-n keresztül, lemezeiket és UEFI NVRAM-jukat pedig újra csatolja, még a VM törlése után is. A VM-ekről, amelyeknek Unraid-pillanatképeik vannak, és a zvolokon lévő lemezekről lásd: [VM-mentések](vm-backup-ssh-setup.md).

A **mappakészletek** a tulajdonosukkal, módjaikkal és időbélyegeikkel térnek vissza, ahogy a restic tárolja őket.

A **flash** soha nem érinti az éles `/boot`-ot. A visszaállítás egy `flash-<id>.zip` nevű letöltés, amely az Unraid USB Creatorba dobható. Az ablak felsorolja az egyes flash-mentésekben lévő bővítményeket a verziójukkal és méretükkel, és egyetlen bővítményt vissza is tud tenni a futó flash-re: a `.plg` fájlját, a `config/plugins` alatti mappáját és azokat a csomagfájlokat, amelyeket a mentés tartalmaz. Az Unraid a következő indításkor telepíti, vagy azonnal a **Plugins, Install Plugin** alatt.

A **ZFS-adatkészletek** saját célokkal rendelkeznek, és megtartják a ZFS-tulajdonságaikat, lásd: [Visszaállítás](zfs-datasets.md#restore).

Az **önmentés** csak a Vészhelyzet varázslón keresztül tér vissza, amely újraindítja a BombVaultot, így az éles adatbázist soha nem írja felül nyitott kezelő alatt.

Az **adatbázis-mentések** soha nem töltődnek vissza maguktól. Az **Adatbázis-mentések** alatti sor letölthető, mappába menthető, vagy egy frissen indított adatbázisba importálható. Lásd: [Adatbázis-mentések](backups.md#database-dumps).

## Több elem egyszerre {#several}

A Visszaállítás oldalon az elemeket úgy jelölheted ki, mint a sorokat a Biztonsági mentések listájában: Ctrl+kattintás, Shift+kattintás, Ctrl+A, hosszú nyomás vagy **Kijelölés**. A **Visszaállítás** ekkor egy ablakot nyit mindegyikhez.

- **Melyik biztonsági mentés**: az **Ajánlott** minden elemnél azt a mentést veszi, amelyet a saját ablaka ajánlana. Az **Egy időpont mindenkinek** minden elemnél a legújabb mentését veszi attól az **Időponttól** kezdve, amelyet megadsz, vagy korábbit, például a hiba előtti estéét.
- **Hová**: **Az eredeti mellé** vagy **Eredeti cseréje**, az **Előbb a jelenlegi állapot mentése** beállítással.

Minden elemet ugyanúgy ellenőriz, mint egy egyedi visszaállítást, majd egymás után futnak. Minden sor megmutatja, hogyan alakult a sajátja: **Várakozik**, **Fut**, **Visszaállítva**, **Sikertelen** vagy **Kihagyva**. A **Megállítás ezután** hagyja befejezni a futó visszaállítást, és nem indít többet. A flash ZIP-ként érkezik letöltésre, a ZFS-adatkészleteket és az önmentést pedig egyenként állítja vissza.

![Több elem visszaállítása: elemenként egy sor a haladásával.](assets/screenshots/restore/several-dark.png#only-dark)
![Több elem visszaállítása: elemenként egy sor a haladásával.](assets/screenshots/restore/several-light.png#only-light)

## Egy stack visszaállítása {#stacks}

A **Stackek** kártyán egy stackre kattintva megnyílik a **Stack visszaállítása**. Minden tagot a legutóbbi mentéséből állít vissza egyszerre, és az **Indítás függőségi sorrendben a visszaállítás után** esetén a `depends_on` sorrendjükben indítja: előbb az adatbázisokat és gyorsítótárakat, aztán az alkalmazást. Ha egy tag projektmappája hiányzik a kiválasztott helyen, az ablak a Helyi állapotát ajánlja fel helyette.

## Más források {#other-sources}

- A **Régi mentési mappából** megnyitja egy másik, már nem futó BombVault tárolóját egy lemezen vagy egy telephelyen kívüli célon, a helyreállítási csomagjával vagy a beírt `APP_KEY` értékével. A munkamenet csak olvasható, és csak a memóriában él. Lásd: [Régi mentési mappából](offsite-recovery.md#restore-from-another-bombvault-repo).
- Az **Importálás az Appdata.Backup bővítményből** a bővítmény mentési mappáját olvassa. Minden konténerarchívum a konténerének visszaállítási pontja lesz, azzal a dátummal, amikor a bővítmény készítette. A korábban importált archívumokat kihagyja, és magukat az archívumokat csak olvassa. A konténernek előbb kell egy mentés a BombVaultban, hogy a visszaállításnak meglegyen a definíciója. A megőrzés békén hagyja az importált visszaállítási pontokat, ezért töröld magad, ha már nincs rájuk szükséged.

## Egy visszaállítás után {#after}

Az eredeti mellé visszaállított másolat a **Biztonsági mentések** alatt kizártként szerepel, így nem számít nyitott feladatnak. Add hozzá az ütemezéshez, ha meg szeretnéd tartani. Az a konténer vagy VM, amely a mentésekor futott, futva tér vissza, az, amelyik le volt állítva, leállítva marad. Unraidon a visszaállított konténer újra megjelenik a Docker lapon normál, szerkeszthető alkalmazásként, a VM pedig a VM-kezelőben a lemezeivel és az NVRAM-mal csatolva.
