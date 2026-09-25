# ZFS-adatkészletek

A **ZFS** oldal ZFS-adatkészletekről készít mentést. Egy elem egy adatkészlet az alatta lévő összes adatkészlettel együtt. Minden mentéshez a BombVault egyetlen ZFS-pillanatképet készít a teljes fáról, így a benne lévő összes adatkészlet ugyanabban a pillanatban kerül rögzítésre. Ezután ebből a pillanatképből beolvassa az egyes adatkészletek fájljait, a resticcel ugyanúgy tárolja őket, mint egy mappát, majd rögtön eltávolítja a pillanatképet. A mentések deduplikáltak, mindegyik böngészhető, és egyes fájlok is visszaállíthatók.

A BombVault adatkészletekhez soha nem használ `zfs send`-et, soha nem görget vissza adatkészletet, és soha nem töröl egyet sem.

## Követelmények {#requirements}

- **Az SSH-kapcsolat ehhez a kiszolgálóhoz.** A ZFS-adatkészletek ugyanazt a kulcsot, gazdát és felhasználót használják, mint a VM-mentések. Ha a VM-mentések már működnek, ez is működik. Egyébként kövesd a GitHubon található [VM-mentés SSH-n keresztül útmutatót](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md). A sablon mezői: **Host SSH: Address**, **Host SSH: Port** és **Host SSH: User**.
- **A `zfs` parancs azon a gazdán.** Az Unraid 6.12 és újabb, valamint a TrueNAS SCALE tartalmazza.
- **A Host Data `/mnt`-ként leképezve, Read/Write - Slave Access Mode-dal.** Ez a sablon alapértéke. Egy adatkészlet pillanatképe csak a BombVault indulása után jelenik meg az adatkészlet `.zfs/snapshot` mappájában, ezért a konténernek meg kell kapnia a gazda által később létrehozott csatolásokat.
- **Az adatkészletek a `/mnt` alá csatolva.** Unraidon a poolok a `/mnt/<pool>` alatt vannak, így ez már teljesül.

Kapcsold be a tartományt a **Beállítások, Általános** alatt (ZFS-adatkészletek). A ZFS oldal ekkor megjeleníti a **Kapcsolat ezzel a kiszolgálóval** kártyát. Ez teszteli az SSH-kapcsolatot, megnevezi a felhasználót és a gazdát, amelyhez kapcsolódik, és megmondja, mi hiányzik, ha valami hiányzik. A gazdaintegrációs ellenőrzés (`/spike`) ugyanazt az eredményt mutatja.

## Elemek és gyermek-adatkészletek {#items-and-children}

Nyisd meg a ZFS oldalon az **Adatkészletek hozzáadása** lehetőséget. A lista a kiszolgálóról jön. Válaszd ki azt az adatkészletet, amely a menteni kívánt rész legtetején van, például `cache/appdata`, és az elem lefedi azt és az alatta lévő összes adatkészletet.

- **Az új gyermek-adatkészletek maguktól csatlakoznak.** Az elem alatt később létrehozott adatkészletet a következő futás menti, és azt újként jelzi. Az első mentés egyszer teljes egészében beolvassa; utána csak a változások kerülnek beolvasásra.
- **Egyes gyermekeket kihagyhatsz.** Kapcsolj ki egyet az elem beállításaiban, és az alatta lévő mindennel együtt kimarad. Egy kihagyott gyermek, amely már nem létezik a kiszolgálón, ilyenként jelölődik, és eltávolítható a listából.
- **Az olvashatatlan gyermekek kimaradnak, de soha nem csendben.** A futás felsorolja őket, az elem mutatja, hány maradt ki, és az irányítópult lefedettségi kártyája mindegyiket védtelennek számolja. A futás ettől még minden mást ment, és egy kihagyott gyermek miatt nem hiúsul meg. Az okok az [okkódok táblázatában](#reason-codes) vannak: nem csatolt adatkészlet, `canmount=off`, `legacy` vagy hiányzó csatolási pont, be nem töltött titkosítási kulcs, kikapcsolt pillanatkép-hozzáférés vagy olyan csatolási pont, amelyet a BombVault nem lát.
- **Egy kihagyott adatkészlet nem viszi magával a gyermekeit.** Egy `canmount=off` adatkészlet, amely csak más adatkészleteket tartalmaz, kimarad ("csak szerkezet"-ként jelenik meg), a csatolt gyermekei pedig mentésre kerülnek. Egy titkosított adatkészlet, amelynek kulcsa nincs betöltve, a kulcsát megosztó gyermekekkel együtt marad ki.
- **A VM-lemezként vagy rendszeradatként szolgáló gyermekek kikapcsolva indulnak** a hozzáadási párbeszédablakban, az okkal a kapcsoló mellett. Egy teljes pool hozzáadásához meg kell erősíteni egy párbeszédablakot, amely felsorolja, mit tartalmaz.

### Kötetek {#volumes}

Egy kötet (zvol) fájlok helyett virtuális lemezt tartalmaz, és a ZFS oldal soha nem ment ilyet.

- Egy VM által használt kötetet a **VM-ek** oldalon mentünk az adott VM-mel együtt.
- Egy kötetet, amelyet egyetlen VM sem használ (iSCSI extent, leválasztott lemez), **a BombVault nem ment**. A hozzáadási párbeszédablak és a ZFS oldal megszámolja ezeket a köteteket, és ezt jelzi is. Egy későbbi verzió menteni fogja őket.

Egy elem fájában lévő kötetek minden futáskor kimaradnak, és meg vannak nevezve.

### A Docker tárolója {#docker-storage}

A Docker ZFS-tárolóillesztőjével minden képréteg egy `legacy` csatolási pontú adatkészlet. A hozzáadási párbeszédablak ezeket szülőnként egy sorba vonja össze. Egy olyan fa, amelyben 20-nál több ilyen adatkészlet van, nem lehet elem: amíg létezik róla pillanatkép, a Docker nem tud képrétegeket eltávolítani. Helyette az alatta lévő adatkészleteket add hozzá, például `appdata`.

### Az elemek soha nem fedik egymást {#overlap}

Egy adatkészlet csak egy elemhez tartozhat. A BombVault elutasít egy új elemet, amely egy meglévőn belül van, vagy amely tartalmazna egyet. Ha több gyermekelemet egy szülőelembe szeretnél összevonni, előbb töröld a gyermekelemeket úgy, hogy megtartod a mentéseiket, majd add hozzá a szülőt. Minden adatkészlet a saját neve alatt őrzi az előzményeit, így a következő mentés ott folytatja, ahol a régi elemek abbahagyták, és nem olvas be mindent újra.

## Konténerek leállítása és parancsok a pillanatkép körül {#consistency}

Egy futó adatbázisról készült pillanatkép olyan, mint egy hirtelen áramszünet: az adatbázis általában helyreáll, de ezt meg kell tennie. Minden elem két dolgot tehet ez ellen, és mindkettő csak a pillanatkép pillanatára vonatkozik, nem az egész mentésre.

- **Állítsd le ezeket a konténereket a pillanatképhez.** A BombVault leállítja a felsorolt konténereket, elkészíti a pillanatképet, és azonnal újraindítja őket. Az azonos függőségi szintű konténerek párhuzamosan állnak le, előbb a függők, így az egész ablak általában néhány másodperc; a futás megmutatja, mennyi volt. A mentés ezután a befagyasztott pillanatképet olvassa, miközben az alkalmazások már újra futnak. Csak a futó konténerek állnak le.
- **Egy parancs a pillanatkép előtt és után.** Egy általad választott konténerben fut, például hogy közvetlenül a pillanatkép előtt kiírjon egy adatbázist az adatkészletbe, anélkül hogy bármit leállítana. Ha a pillanatkép előtti parancs sikertelen, a mentés meghiúsul, és nem készül pillanatkép. A pillanatkép utáni sikertelen parancs megjelenik a futásnál, de nem hiúsítja meg a mentést.

Mi történik, ha valami rosszul sül el:

- Ha egy konténert nem lehet leállítani, a BombVault elindítja azokat, amelyeket már leállított, és a mentés a konténer nevével meghiúsul. Soha nem tér vissza futó alkalmazások pillanatképére.
- A leállítás megvárja, amíg egy folyamatban lévő konténermentés befejeződik (kézi futásnál legfeljebb 30 percig, ütemezettnél a mentés időkorlátjáig), így a kettő soha nem állítja le és indítja el ugyanazt a konténert egyszerre.
- Mielőtt az első konténer leállna, a BombVault feljegyzi, melyeket állítja le. Ha a BombVaultot az ablakon belül leállítják, a következő indításakor újraindítja ezeket a konténereket, értesítést küld, és az elem piros megjegyzést mutat minden olyan konténernél, amelyet nem tudott elindítani.

Az automatikus adatbázis-kiírások (lásd [Funkciók](features.md)) egy konténer saját mentésével futnak a **Konténerek** oldalon, nem egy ZFS-elemmel. Az az adatbázis, amelynek konténerét csak az adatkészletén keresztül mentik, nem kap kiírást, ezért adj neki itt egy parancsot.

Egy konténer egyszerre lehet ezen a listán és a **Konténerek** oldalon. Az adatai ilyenkor kétszer tárolódnak, két tárolóban, és a **Teljes mentés** kétszer állítja le. Az elem jelzi ezt.

## Visszaállítás {#restore}

Nyisd meg az elemen a **Biztonsági mentések** részt, válaszd ki a mentést, majd az adatkészletet. Alapértelmezés szerint ez az elem legfelső adatkészlete.

- **Visszaállítás az adatkészletbe.** A mentés fájljai az adatkészlet csatolási pontjába íródnak. Az azonos nevű fájlok felülíródnak, a többiek maradnak. Az adatkészletet soha nem görgetjük vissza és nem cseréljük le. A BombVault ellenőrzi, hogy az adatkészlet csatolva, látható és írható, egyszer a kezdés előtt, és újra közvetlenül az írás előtt. Ahol egy gyermek-adatkészlet van benne csatolva, oda semmi nem íródik: a gyermek megtartja fájljait, tulajdonosát és jogosultságait, és a saját mentéséből áll vissza.
- **Visszaállítás mappába.** Válassz egy mappát a `/mnt` alatt. A BombVault ellenőrzi, hogy a mappa csatolt poolon vagy megosztáson van-e, és van-e elég szabad hely. Ez SSH-kapcsolat nélkül és már nem létező adatkészleteknél is működik.
- **Fájlok kiválasztása** (speciális): csak a kiválasztott fájlokat és mappákat írja vissza az adatkészletbe.
- **Ennek a mentésnek az összes adatkészlete** (speciális): a fa minden adatkészletét a kiválasztott mappa saját almappájába. A mentésben kihagyott adatkészletek meg vannak nevezve.
- **Másik kiszolgálóról:** a **Helyreállítás** oldal egy másik BombVault tárolójából állít vissza, mindig mappába: egy mentés összes adatkészletét, mindegyiket saját almappába, vagy a fa egy adatkészletét, egészben vagy kiválasztott fájlokat.

Az elem leállítandó konténereinek listáját az adatkészletbe történő visszaállításnál is felajánlja. Ezek a konténerek az egész visszaállítás alatt leállítva maradnak, a konténermentések addig várnak.

### A biztonsági pillanatkép {#safety-snapshot}

Mielőtt írna egy adatkészletbe, a BombVault ZFS-pillanatképet készít csak arról az adatkészletről `bombvault-prerestore-<idő>` néven. Alapértelmezés szerint be van kapcsolva; kikapcsolásához második megerősítés kell. Ha a pillanatkép nem készíthető el, semmi nem áll vissza.

A BombVault soha nem töröl magától biztonsági pillanatképet. Az elem felsorolja őket korukkal és méretükkel, mindegyiket **Törlés** művelettel, és figyelmeztet, ha a legrégebbi 30 napnál idősebb, mert törölt és módosított adatokat tart a poolban.

Ha egy visszaállítás után vissza szeretnél lépni, másolj egyes fájlokat az adatkészletben lévő `.zfs/snapshot/bombvault-prerestore-<idő>` mappából. A `zfs rollback <dataset>@bombvault-prerestore-<idő>` csak addig működik, amíg ez az adatkészlet legújabb pillanatképe. A `zfs rollback -r` minden újabb pillanatképet töröl, az automatikusakat is.

### Visszaállítás új adatkészletként {#new-dataset}

A BombVault nem hoz létre adatkészleteket. Hozd létre a kiszolgálón a kívánt tulajdonságokkal, majd állítsd vissza egy olyan mappába, amely a csatolási pontja:

```
zfs create -o compression=lz4 cache/appdata-restored
```

majd a BombVaultban állítsd vissza a `/mnt` alatti `cache/appdata-restored` mappába.

## Mi van a mentésben {#contents}

A mentésben: minden mentett adatkészlet fájljai és mappái, tulajdonossal, jogosultságokkal, időbélyegekkel és kiterjesztett attribútumokkal, ahogy a restic tárolja őket.

Nincs a mentésben:

- az adatkészletek ZFS-tulajdonságai (compression, recordsize, quota, mountpoint és a többi);
- az egyes adatkészletek legfelső mappájának saját tulajdonosa és jogosultságai (minden alatta lévő benne van). Az adatkészletbe történő visszaállítás a meglévő legfelső mappát változatlanul hagyja, a mappába történő visszaállítás `0755` jogosultsággal hozza létre;
- a meglévő ZFS-pillanatképek;
- a kihagyott vagy kizárt gyermekek;
- a kötetek.

Új poolra való visszaállításhoz előbb hozd létre az adatkészleteket a kívánt tulajdonságokkal. Azt még nem ellenőriztük, hogy az NFSv4 ACL-ek, ahogy a TrueNAS használja őket SMB-adatkészleteken, úgy jönnek-e vissza, ahogy várod, ezért próbálj ki egy visszaállítást a saját adataidon, mielőtt rájuk hagyatkozol.

## Titkosított adatkészletek {#encryption}

Egy titkosított adatkészletet csak akkor ment, amíg a kulcsa be van töltve. Egyébként figyelmeztetéssel kimarad; töltsd be a kulcsot a `zfs load-key` paranccsal, és csatold az adatkészletet. A BombVault visszafejtve olvassa az adatokat, és a restic tárolójában tárolja őket, amely titkosított. Ha kikapcsoltad a titkosítást a BombVaultban, az a tároló nem titkosított.

## Megmaradt pillanatképek {#leftover-snapshots}

Egy mentés pillanatképének neve `<dataset>@bombvault-<14 számjegy>`, például `cache/appdata@bombvault-20260924021500` (UTC). A BombVault közvetlenül a mentés után eltávolítja. Ha ez nem sikerül, például mert az adatkészlet foglalt, vagy a BombVaultot leállították, a BombVault eltávolítja:

- az adott elem következő mentése előtt,
- a BombVault indulásakor, minden elemnél, kikapcsolt tartománnyal is,
- amikor törlöd az elemet,
- amikor megnyomod az elemen az **Eltávolítás most** gombot, amely azt is mutatja, hány maradt.

Csak azok a nevek törlődnek, amelyek pontosan `bombvault-` és 14 számjegy. A biztonsági pillanatképekhez, a saját pillanatképeidhez és az automatikus pillanatképekhez soha nem nyúl. Ha kézzel szeretnél eltávolítani egyet:

```
zfs destroy -r cache/appdata@bombvault-20260924021500
```

## Anomáliák {#anomalies}

Egy kiürített gyermek alig változtat egy nagy fa összegén, ezért az anomáliaészlelés egy elem minden adatkészletét külön figyeli: a méretnek, a fájlszámnak, az új adatoknak és a restic-időnek mind saját előzménye van. Ez az előzmény az adatkészlet nevéhez tartozik, így megmarad, ha a fát később egy másik elem menti.

Egy adatkészlet, amelyet az előző futás mentett, de ez a futás nem tudott beolvasni, kiürítettnek számít, feltéve, hogy az elem kiválasztása nem változott. Ez lefedi a be nem töltött kulcsot, a nem csatolt adatkészletet és azt, amelyik eltűnt a fából. Egy gyermek, amelyet te magad zársz ki, megváltoztatja a kiválasztást, ezért az előzménye ilyenkor elölről kezdődik. Amíg egy elveszett adatokról szóló megállapítás nyitva van, a megőrzés csak annak az adatkészletnek a régi mentéseit tartja meg, a fa többi részét a szokásos módon ritkítja.

Az **Anomáliák** oldal **Elemek** lapján minden adatkészletnek saját sora van az eleme alatt, és az elem fája ezen az oldalon minden adatkészlet mellett mutatja a nyitott megállapításokat. Egy megállapítás hivatkozása az elem visszaállítási paneljét az adatkészlet utolsó jó mentésénél nyitja meg. Azt, hogy egy futás befejeződik-e, az egész elemre nézve ítéljük meg, mert egy futás egészként sikerül vagy hiúsul meg.

Magukat az ellenőrzéseket a [Funkciók](features.md) írja le. Egy, az [MCP-kiszolgálón](mcp.md) keresztül kapcsolódó asszisztens listázhatja egy ZFS-elem visszaállítási pontjait, elindíthatja a mentését és olvashatja a megállapításokat, de egy megállapítás nyugtázása az **Anomáliák** oldalon történik.

## Okkódok {#reason-codes}

Az oldal, a futási előzmények és az értesítések ezek egyikével nevezik meg a problémát. A legtöbbnél a megoldás is ott van mellette az oldalon.

| Kód | Jelentés | Mi a teendő |
|---|---|---|
| `ssh-missing` | Az SSH-kapcsolat nincs beállítva ebben a konténerben. | Állítsd be az SSH-kapcsolatot, mint a VM-mentésekhez. |
| `host-placeholder` | A Host SSH: Address még mindig a mintaérték, és a `host.docker.internal` sem válaszolt. | Állítsd a Host SSH: Address értékét ennek a kiszolgálónak a LAN IP-jére. |
| `host-fallback` | A Host SSH: Address még mindig a mintaérték, és a `host.docker.internal` működik. | Semmi, vagy állítsd be a LAN IP-t. |
| `ssh-unreachable` | A kiszolgáló SSH-n nem érhető el. | Ellenőrizd a címet és a portot, és hogy az SSH be van-e kapcsolva. |
| `ssh-auth` | A kiszolgáló elutasította a BombVault kulcsát. | Futtasd egyszer a kiszolgálón a kapcsolati kártyán látható parancsot. |
| `zfs-not-found` | Az SSH-gazdán nincs `zfs` parancs. | Irányítsd a Host SSH: Address értéket arra a gépre, amelyé a poolok. |
| `zfs-permission` | Az SSH-felhasználó nem futtathatja ezt a zfs-parancsot. | Használd a rootot, vagy lásd [TrueNAS SCALE](#truenas). |
| `uri-mismatch` | A `LIBVIRT_URI` más gazdát vagy felhasználót nevez meg, mint az SSH-mezők. | Hangold össze őket, vagy ürítsd ki az SSH-mezőket, hogy mindkettő az URI-ból jöjjön. |
| `zfs-error` | A zfs más hibát jelzett. | A részletek mutatják az üzenetét. |
| `propagation-missing` | A gazdán létrejövő új csatolások nem érik el a konténert. | Állítsd a Host Data Access Mode értékét Read/Write - Slave-re, és indítsd újra a BombVaultot. |
| `invalid-name` | Olyan adatkészletnév, amelyet a BombVault nem fogad el. | Nevezd át az adatkészletet. |
| `name-too-long` | A fa egyik adatkészlete túl hosszú egy pillanatkép nevéhez. | Nevezd át, vagy add hozzá elemként az alatta lévő adatkészletet. |
| `invalid-exclude` | Egy kizárási minta vagy egy kihagyott gyermek nem illik az elemhez. | Javítsd az üzenetben megnevezett bejegyzést. Egy teljes gyermek-adatkészlet kihagyásához kapcsold ki, ahelyett hogy mintát írnál. |
| `not-found` | Az adatkészlet nem létezik a kiszolgálón. | Távolítsd el az elemet, vagy hozd létre újra az adatkészletet. A mentései visszaállíthatók maradnak. |
| `not-filesystem` | Ez egy kötet, nem fájlrendszer. | Lásd [Kötetek](#volumes). |
| `overlaps-item` | Az adatkészlet átfedésben van egy meglévő elemmel. | Lásd [Az elemek soha nem fedik egymást](#overlap). |
| `docker-storage` | A fa a Docker képtárolóját tartalmazza. | Lásd [A Docker tárolója](#docker-storage). |
| `nothing-readable` | Az elem egyetlen adatkészlete sem olvasható most. | Nézd meg a kihagyott adatkészletek kódjait. |
| `snapshot-failed` | A pillanatképet nem sikerült létrehozni. | A részletek mutatják a zfs üzenetét. |
| `containers-busy` | Egy konténermentés még futott, amikor a konténereknek le kellett állniuk. | Indítsd újra később. Az ütemezett futások maguktól várnak. |
| `consistency-stop-failed` | Egy konténert nem sikerült leállítani, ezért nem készült pillanatkép. | Ellenőrizd a konténert, vagy vedd le a listáról. |
| `pre-snapshot-failed` | A pillanatkép előtti parancs sikertelen volt. | A futás részletei mutatják a kimenetét. |
| `container-unknown` | Egy felsorolt konténer nem létezik. | Vedd le a listáról. |
| `container-is-self` | A BombVault nem tudja leállítani a saját konténerét. | Vedd le a listáról. |
| `leftover-snapshots` | Olyan pillanatképek vannak még a kiszolgálón, amelyeket a BombVault nem tudott eltávolítani. | Nyomd meg az **Eltávolítás most** gombot, lásd [Megmaradt pillanatképek](#leftover-snapshots). |
| `zvol` | Egy kötet a fában, kihagyva. | Lásd [Kötetek](#volumes). |
| `canmount-off` | Soha nincs csatolva (`canmount=off`), kihagyva. | Ha adatot tartalmaz, csatold, vagy helyezd át az adatokat egy gyermek-adatkészletbe. |
| `legacy-mount` | Legacy csatolási pont, kihagyva. | Adj neki csatolási pontot a `/mnt` alatt. |
| `no-mountpoint` | Nincs csatolási pont, kihagyva. | Adj neki csatolási pontot a `/mnt` alatt. |
| `not-mounted` | Nincs csatolva a kiszolgálón, kihagyva. | Csatold a `zfs mount` paranccsal, vagy állítsd be: `canmount=on`. |
| `key-not-loaded` | Titkosított, és a kulcs nincs betöltve, kihagyva. | `zfs load-key`, majd csatold. |
| `snapdir-disabled` | A pillanatkép-hozzáférés ki van kapcsolva, kihagyva. | `zfs set snapdir=hidden <dataset>`. A `.zfs` mappa rejtve marad. |
| `not-visible` | A BombVault nem látja az adatkészlet csatolási pontját. | Helyezd a csatolási pontot a Host Data útvonal alá, vagy képezd le a konténerbe ugyanazon az útvonalon Read/Write - Slave-vel. |
| `shfs-only` | Az adatkészlet csak a `/mnt/user` útvonalon látszik, amely elrejti a pillanatképeket. | A `/mnt`-t képezd le Host Datának, ne a `/mnt/user`-t. |
| `snapshot-not-visible` | A pillanatkép elkészült, de nem jelent meg a BombVaultban. | Futtasd a **Próbáld ki a pillanatképek elérését** műveletet; lásd lent. |
| `snapshot-loop` | A pillanatkép nem jutott el a BombVaultig, mert a Host Data nem engedi át az új csatolásokat. | Állítsd a Host Data Access Mode értékét Read/Write - Slave-re, és indítsd újra a BombVaultot. |
| `backup-failed` | A restic ennél az adatkészletnél sikertelen volt. | A futás részletei mutatják az okát. |
| `not-reached` | A futás ezen adatkészlet előtt véget ért. | Futtasd újra a mentést. |
| `gone` | Az adatkészlet már nincs a kiszolgálón. | Semmi. A mentései visszaállíthatók maradnak. |
| `read-only-mount` | A BombVault csak olvasni tudja az adatkészletet, ezért nem tud bele visszaállítani. | Állítsd a leképezést Read/Write - Slave-re, vagy állítsd vissza mappába. |
| `destination-not-mounted` | A mappa nem csatolt poolon vagy megosztáson van. | Válassz egy mappát poolon vagy megosztáson. |
| `not-enough-space` | Nincs elég szabad hely a célhelyen. | Szabadíts fel helyet, vagy válassz másik mappát. |
| `safety-snapshot-failed` | A biztonsági pillanatképet nem sikerült elkészíteni, ezért semmi nem állt vissza. | A részletek mutatják a zfs üzenetét. |
| `safety-name-too-long` | Az adatkészlet neve túl hosszú egy biztonsági pillanatképhez. | Kapcsold ki a biztonsági pillanatképet, vagy állítsd vissza mappába. |

### Mit lát a konténer {#mountinfo}

A **Próbáld ki a pillanatképek elérését** egy elemen valódi pillanatképet készít a fájáról, minden adatkészlethez megkeresi a BombVaulton belül, majd újra eltávolítja. Ez a leggyorsabb módja annak, hogy az első ütemezett futás előtt igazold a teljes utat.

Ha magad szeretnéd megnézni, futtasd ezt a kiszolgálón:

```
docker exec BombVault grep zfs /proc/self/mountinfo
```

Minden sor egy csatolás a konténerben. Egy adatkészlet sora mutatja az útvonalát a konténeren belül (a `/host/user` alatt) és az adatkészlet nevét. Ha ebben a sorban van egy `master:N` mező, az azt jelenti, hogy a csatolás megkapja a gazda által később létrehozott csatolásokat, és erre van szüksége a pillanatkép-hozzáférésnek. Ha hiányzik, állítsd a Host Data Access Mode értékét Read/Write - Slave-re, és indítsd újra a BombVaultot.

## TrueNAS SCALE {#truenas}

- Ha a `LIBVIRT_URI` be van állítva (mint a VM-mentésekhez TrueNAS-on), a BombVault a zfs-parancsaihoz az SSH-gazdát, a felhasználót és a portot az URI-ból veszi, mindegyiket, amely nincs külön beállítva. VM-mentések nélkül helyette állítsd be a `LIBVIRT_HOST`, `LIBVIRT_SSH_USER` és `LIBVIRT_SSH_PORT` változókat. A változókat az **Additional Environment Variables** alatt add hozzá.
- A roottól eltérő felhasználónak jogosultság kell az elem legfelső adatkészletére, amely aztán minden alatta lévő adatkészletet lefed:

  ```
  zfs allow <user> snapshot,destroy,mount <dataset>
  ```

  TrueNAS-on egy nem root SSH-munkamenet útvonalában nincs benne a `/usr/sbin`; a BombVault ilyenkor közvetlenül a `/usr/sbin/zfs` parancsot hívja.
- Az alkalmazás **Host Data** értékének az adatkészletek feletti gazdaútvonalnak kell lennie, például `/mnt/tank`, nem ixVolume-nak. Gazdaútvonallal az alkalmazás továbbadja a gazda új csatolásait a BombVaultnak (`rslave`), és erre van szüksége a pillanatkép-hozzáférésnek.
