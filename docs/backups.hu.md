# Biztonsági mentések és elemek

Minden, amit a BombVault ment, egyetlen listában áll a **Biztonsági mentések** alatt: konténerek, VM-ek, mappakészletek, a flash, ZFS-adatkészletek és az önmentés. Mindegyik egy **elem**, saját oldallal. Ez az oldal végigvezet a listán, az elem oldalán és három lapján, valamint az elemek beállításain.

![A Biztonsági mentések lista a Konténerek csempével kiválasztva: minden sor az utolsó futásait sávként mutatja, és egy állapotot.](assets/screenshots/backups/list-dark.png#only-dark)
![A Biztonsági mentések lista a Konténerek csempével kiválasztva: minden sor az utolsó futásait sávként mutatja, és egy állapotot.](assets/screenshots/backups/list-light.png#only-light)

## Mit ment az egyes szakasz {#what-is-saved}

| Szakasz | Mi mentődik |
|---|---|
| **Konténerek** | Az appdata mappa és a konténer definíciója: image, változók, portok, címkék és kötetek. A konténer **Beállítások** lapján lévő **Mentett mappák** kártya pontosan kijelöli, mely mappákat fedi le a mentés, élő számlálóval az útvonalakról és a kihagyottak listájával. Az az adat, amelyet egy konténer a Host Data-n kívül tart, például egy TrueNAS ixVolume-ban vagy egy `/opt` alatti Compose-projektben, nem kerül mentésre, és a futás megnevezi a kihagyott mappákat. A futás azt a Host Data-n belüli mappát is megnevezi, amely éppen nincs meg. Az olyan rendszerútvonalak, mint az `/etc/localtime` vagy a Docker-socket, nem számítanak adatnak. |
| **VM-ek** | A lemezképek, az XML-definíció és az UEFI NVRAM, szabályos leállítással vagy élő pillanatképpel, SSH-n keresztül. Lásd: [VM-mentések](vm-backup-ssh-setup.md). |
| **Flash** | Csak Unraidon. A teljes USB flash (`/boot`): operációs rendszer, licenc, tömbkonfiguráció, megosztások, hálózati és bővítménykonfiguráció. |
| **Önmentés** | A BombVault saját `/config` mappája: a beállítási adatbázisa, a célok bejelentkezései, a libvirt SSH-kulcspárja, a ZFS-szerverek kulcsa és rögzített hosztkulcsai, valamint a tanúsítványok, SQLite `VACUUM INTO` használatával pillanatképezve, hogy az adatbázis soha ne írás közben kerüljön mentésre. |
| **Mappák** | A szerver bármelyik mappája mappakészletként, például egy megosztás, a dokumentumaid vagy egy fotótár, opcionális kizárási mintákkal. A mappakészleteknek is van ütemezésük, megőrzésük, telephelyen kívüli másolatuk, ellenőrzésük és visszaállítási gyakorlatuk, mint mindennek. |
| **ZFS-adatkészletek** | Egy adatkészlet és minden alatta lévő, egyetlen ZFS-pillanatképből olvasva, hogy mind ugyanabból a pillanatból származzon, és a resticcel tárolva, mint egy mappa. Egy ZFS-elem blokkpontos replikát is tarthat egy második ZFS-szerveren. Lásd: [ZFS-adatkészletek](zfs-datasets.md). |

Minden mentés a restic-en megy át, ezért inkrementális és deduplikált, és még a nagy VM-lemezek sem fújják fel a tárolót. A BombVault soha nem menti, és így soha nem állítja le a saját konténerét. Egy futás biztonságosan leállítható, mert a restic a pillanatképét utoljára írja, így egy megszakított futás hivatkozás nélküli adatot hagy maga után, pillanatképet nem. A futás közbeni újraindítás után maradt zárat feloldja, és a futást még egyszer megpróbálja, de csak akkor, ha a zár tíz percig nem frissült.

## A lista {#list}

A lista fölötti csempék fajtát választanak, például **Konténerek** vagy **ZFS**, és mindegyik megszámolja az elemeit. Egy csempe kijelöl, nem rejti el az oldal többi részét.

Minden sor az elem utolsó tizennégy futását kis négyzetekként mutatja, és egy állapotot, például **Védett**, **Nincs védve**, **Elhalasztva** vagy **Replikálás folyamatban**. Minden más a neve melletti infóbuborékban van, és amíg a mutató a soron pihen, ez a szöveg magában a sorban áll. A konténer neve melletti jel azt jelzi, hogy az utolsó mentése óta másik image-gel, más portokkal, változókkal vagy kötetekkel jött létre újra. A következő mentéssel eltűnik.

Három gomb a lista fölött szűkíti:

- **Szűrők**: **Ütemezés** szerint (**Összes**, **Ütemezett**, **Szüneteltetve**), **Biztonsági mentés** szerint (**Mentve**, **Soha nem mentve**), a **Konténerek és VM-ek** esetén pedig **Telepített** vagy **Nincs telepítve**.
- **Rendezés**: név, állapot, IP, utolsó mentés, méret vagy ütemezés szerint. Az IP kivételével minden mező mindkét irányba rendez, például **Méret, a legnagyobb elöl** és **Méret, a legkisebb elöl**.
- **Keresés**: név, image vagy IP szerint.

Azok az elemek, amelyek már nincsenek telepítve, de még vannak mentéseik, külön állnak a **Nincs telepítve (csak biztonsági mentések)** alatt. Állítsd vissza őket, vagy töröld a mentéseiket a hely felszabadításához.

Az oldal fő művelete lebeg a jobb alsó sarokban: az **Összes mentése most** futást indít minden bekapcsolt szakaszon. A **Több** tartalmazza a többit:

- A **Kijelölés** kijelölést indít, lásd lent.
- A **Biztonsági mentések felfedezése** beolvassa, mit tartanak a célok, és elemeket épít újra belőle, például egy friss telepítés után. Egy saját adat nélküli konténernek nincs pillanatképe, és a tároló mellé tükrözött definícióból tér vissza. Az újraépített szakaszok telephelyen kívüli másolatai ekkor várnak, amíg át nem veszed a mentéseiket a szerver saját mentéseiként, lásd: [Újraépítés után](offsite-recovery.md#after-a-rebuild).
- A **ZFS-kapcsolat ehhez a kiszolgálóhoz** megnyitja a ZFS-szakasz SSH-tesztjét, lásd: [ZFS-adatkészletek](zfs-datasets.md#requirements).
- Az **Összes replikálása most**, a ZFS csempe kiválasztva, minden ZFS-elem replikáját elküldi.

![Ugyanez a lista telefonon.](assets/screenshots/backups/list-phone-dark.png#only-dark)
![Ugyanez a lista telefonon.](assets/screenshots/backups/list-phone-light.png#only-light)

## Több elem kijelölése {#select}

A sorokat úgy jelölheted ki, ahogy a fájlokat egy fájlkezelőben: Ctrl+kattintás, Shift+kattintás, Ctrl+A, vagy hosszú nyomás telefonon. A kijelölt sor az ikonja helyett pipát mutat, a lista fölötti sáv megszámolja őket, és felkínálja a **Kijelöltek biztonsági mentése**, az **Ütemezés szüneteltetése** és az **Ütemezés folytatása**, az **Összes kijelölése** és a **Kijelölés törlése** műveletet. Minden sornak jobb gombos menüje is van a saját műveleteivel.

A kijelölést a szerver menti le, elemenként egymás után, így akkor is folytatódik, ha bezárod a lapot. Több elem együttes visszaállításához használd a [Visszaállítás oldalt](restore.md#several).

![Két kijelölt konténer a lista fölötti sávval.](assets/screenshots/backups/bulk-dark.png#only-dark)
![Két kijelölt konténer a lista fölötti sávval.](assets/screenshots/backups/bulk-light.png#only-light)

## Mappa vagy ZFS-adatkészlet hozzáadása {#add}

A konténerek, VM-ek és a flash maguktól megjelennek, amint a szakaszuk be van kapcsolva a **Beállítások, Általános** alatt. A mappákat és a ZFS-adatkészleteket te választod ki, mert a BombVault nem tudhatja, melyekre gondolsz. A **Hozzáadás** megnyitja a **Mit mentsen még a BombVault?** ablakot:

- Az **Egy mappa** felsorolja a BombVault által talált megosztásokat, és elfogad bármely más mappát a Host Data alatt. Adj nevet a mappakészletnek, például `Documents`. A név az első mentés után rögzített, mert a pillanatképek ez alatt tárolódnak.
- Az **Egy ZFS adatkészlet** felsorolja a szerver adatkészleteit. Azt válaszd, amelyik a menteni kívánt rész tetején áll, lásd: [Elemek és gyermek-adatkészletek](zfs-datasets.md#items-and-children).
- A **Rendszerkonfiguráció** olyan Unraid nélküli hoszton jelenik meg, amelynek Host Data-ja eléri az `/etc` mappát. Ez egy kiinduló `/etc` mappakészlet, amelyet mentés előtt átnézhetsz.

## Egy elem kihagyása {#dont-back-up}

Az az elem, amelyet szándékosan kihagysz, például egy teszt konténer, megjelölhető **Ne legyen mentés** állapotúra. Ekkor már nem számít védetlennek, és a **Kizárás visszavonása** visszahozza. Az **Ütemezés szüneteltetése** a szelídebb út: az elem kilép az ütemezésből és a teljes mentésből, szürke **Ütemezés szünetel** jelvényt kap, és a **Biztonsági mentés most** továbbra is működik. Ugyanazt a kapcsolót állítja, mint az **Ütemezésben** az elem Beállítások lapján.

## Egy elem oldala {#entry-page}

Egy sorra kattintva megnyílik az elem oldala. A fej egy sorban megnevezi az elemet és az állapotát, például **Utoljára mentve 3 órája** vagy **Elhalasztva, következő próba 03:15-kor**, és tartalmazza a műveleteit: **Biztonsági mentés most**, **Visszaállítás**, valamint a **Több** alatt a ritkábbakat, például **Legújabb mentés exportálása**, **Konténer exportálása** vagy **VM exportálása**, **Összekötés korábbi elemmel**, **Összes biztonsági mentés törlése**, **Elem eltávolítása** és **Biztonsági mentések törlése mindenhol**. Az ikontól balra lévő vissza gomb a listához vezet. Amíg egy mentés fut, a haladása mellett ott áll a **Biztonsági mentés megszakítása**, a megszakított futás pedig megszakítottként van rögzítve, nem sikertelenként.

A **Biztonsági mentések törlése mindenhol** egyszerre törli az elem biztonsági mentéseit minden célon, a telephelyen kívüli másolatokkal együtt. Az ablak minden helyet felsorol azzal együtt, hány mentés van rajta, és a gomb akkor válik elérhetővé, amikor begépeled az elem nevét. Az append-only módra állított hely megtartja a mentéseit, és ezt jelzi is. Az elem és a beállításai megmaradnak, így egy elem, amelyről már nem készítesz mentést, az is marad, csak a mentései és azok előzményei tűnnek el.

Az oldalnak három lapja van: **Biztonsági mentések**, **Beállítások** és **Ellenőrzés**.

### Biztonsági mentések lap {#backups-tab}

Ez a lap az elem pillanatképeit sorolja fel, a legújabbal kezdve, naponta egy sávval. Egy több helyen is meglévő mentés egyszer jelenik meg, minden azt tartalmazó céllal megjelölve. Egy pillanatképre kattintva visszaállítod, egy pillanatképen belüli helyre kattintva pedig pontosan onnan.

- A **Célok** megnevezi minden helyet, ahol van mentés ebből az elemből, és hányat. A telephelyen kívüli helyeket csak az **Ellenőrzés** megnyomásakor olvassa be, mert a felhőtár listázása időbe telik. Az a hely, amelyre az elem már nem ment, **Csak korábbi mentések** felirattal áll, és még visszaállítható belőle.
- Minden pillanatképnek van menüje: **Címke hozzáadása** vagy **Címke módosítása**, **Összehasonlítás másik mentéssel**, amely felsorolja az érkezett, elmúlt vagy megváltozott fájlokat, **Exportálás** tarként, zipként vagy saját restic-tárolóként, és törlés csak egy helyen.
- Egy konténer **Adatbázis-mentései** külön listában állnak, letöltéssel, mappába mentéssel és importtal.

Hány pillanatkép marad meg, azt a cél állítja be, lásd: [Meddig őrizze meg](offsite-recovery.md#keep).

![Egy konténer Biztonsági mentések lapja: naponta egy sáv, és a célok, amelyek az egyes pillanatképeket tartalmazzák.](assets/screenshots/backups/entry-backups-dark.png#only-dark)
![Egy konténer Biztonsági mentések lapja: naponta egy sáv, és a célok, amelyek az egyes pillanatképeket tartalmazzák.](assets/screenshots/backups/entry-backups-light.png#only-light)

### Beállítások lap {#settings-tab}

Minden elem az **Ütemezés és cél** kártyával kezd:

- Az **Ütemezésben** be- vagy kivesz az elemet az ütemezett futásokból.
- A **Saját terv** ennek az egy elemnek külön tervet ad, lásd: [Saját terv egy elemhez](schedules.md#own-plan). Anélkül a szakasza tervén vagy a teljes mentésben fut.
- A gombok sora, a **Helyi** és egy-egy minden telephelyen kívüli célhoz, megmondja, hová mennek a mentések. Az **Az alapértelmezést követi** azt jelenti, hogy az elem oda megy, ahová a szakasza, és egy kattintás rajta megnyitja a **Célok szakaszonként** ablakot. Lásd: [Elhelyezés elemenként](offsite-recovery.md#placement).
- A **Tároló** megnevezi, hová íródnak a mentések (**Tárolva itt**). Ez a hely az első mentéstől rögzített, a másolatok bármikor változhatnak.

Az alatta lévő kártyák az elem fajtájától függnek:

| Fajta | Kártyák |
|---|---|
| Konténer | **Mentett mappák** (mely leképezett mappák kerülnek mentésre, a **Gyorsítótár-mappák kihagyása (CACHEDIR.TAG)** beállítással), **Kizárási minták**, **Más konténerek leállítása**, **Parancsok előtte és utána**, **Adatbázis**, **Ha az alkalmazás foglalt**, **Frissítés egy sikeres biztonsági mentés után**, **Pillanatkép leállítás helyett**, **Korábbi nevek** |
| VM | **VM** a **Módszer** beállítással (**Szabályos (leállítás)** vagy **Élő pillanatkép**) és a **Csak a változott blokkok** beállítással, lásd a [VM-mentési útmutatót](vm-backup-ssh-setup.md#per-vm-method) |
| Mappakészlet | **Mappa** és **Kizárási minták** |
| Flash | **Flash ZIP-exportálás**, amely minden flash-mentést sima `.zip` fájlként kiír egy általad választott mappába, egy lépésben, így egy szinkronizáló eszköz soha nem lát félkész fájlt. Ha egy flash-mentés kevesebbet tartalmaz a kelleténél, az utolsó ZIP a helyén marad, és az export futása ezt jelzi. |
| ZFS-adatkészlet | a gyermek-adatkészletek, a leállítandó konténerek és a pillanatkép körüli parancsok, a **Replika** és a **Próbáld ki a pillanatképek elérését**, lásd: [ZFS-adatkészletek](zfs-datasets.md) |

A beállítások maguktól mentődnek, ahogy megváltoztatod őket.

![Egy konténer Beállítások lapja, az első kártyán a Saját terv.](assets/screenshots/backups/entry-settings-dark.png#only-dark)
![Egy konténer Beállítások lapja, az első kártyán a Saját terv.](assets/screenshots/backups/entry-settings-light.png#only-light)

### Ellenőrzés lap {#verify-tab}

- Az **Ellenőrzések** az első mentést követő visszaállítás-ellenőrzést mutatja. A **Visszaállítás ellenőrzése** újra lefuttatja a legújabb mentésen: a BombVault legfeljebb 100 fájlból és 256 MiB-nyi mintát visszaállít egy ideiglenes mappába, és a resticcel minden fájlt visszaolvastat a hash-eivel szemben.
- Az **Indítási teszt** egy konténernél a legújabb mentést egy elszigetelt másolatba állítja vissza, és elindítja: `bombvault-test-` kezdetű névvel, saját belső Docker-hálózattal közzétett portok és a LAN-hoz vezető út nélkül, 1 CPU-val és 2 GiB memóriával. Akkor sikeres, ha a másolat egészségesnek jelentkezik, válaszol az első kitett portján, vagy, ha egyik sincs, tovább fut. Az eredetihez soha nem nyúl, a másolatot, a hálózatát és az adatait utána eltávolítja, egy teszt közbeni újraindítás után is. A hosthálózaton lévő, a privilegizált, az eszközöket használó és a másik konténert igénylő konténerek nem tesztelhetőként jelennek meg. Az **Indítási teszt** a **Beállítások, Ellenőrzés, Automatikus ellenőrzések** alatt futásonként egy konténert tesztel, azt, amelyiket a legrégebben.
- Az **Anomáliák** beállítja az elem saját érzékenységét és értesítési szintjét, és megmutatja a nyitott leleteit.
- A **Méret mappánként** megmutatja, mi viszi el a helyet a legújabb mentésben, és abból mennyit hozott a legutóbbi mentés, egyszerre egy szintet.
- A **Legutóbbi futások** felsorolja az utolsó futásokat az indításukkal, befejezésükkel és időtartamukkal. Egy kattintás megmutatja a részleteket.

![Egy konténer Ellenőrzés lapja a Méret mappánként megnyitva.](assets/screenshots/backups/entry-verify-dark.png#only-dark)
![Egy konténer Ellenőrzés lapja a Méret mappánként megnyitva.](assets/screenshots/backups/entry-verify-light.png#only-light)

## Kizárási minták {#excludes}

A **Kizárási minták** kártya soronként egy mintát vár. Az útvonalakat úgy írd, ahogy a konténeren belül látod őket, például `/config/Library/Application Support/Plex Media Server/Cache`, vagy egy puszta nevet, például `.git`, amely tetszőleges mélységben illeszkedik. Mappakészletnél az útvonalat úgy írd, ahogy a hoszt ismeri, például `/mnt/user/data/cache`. A mező alatti előnézet megmutatja, mivé oldódik fel az egyes sor, és figyelmeztet, ha egy sor semmire nem illik, vagy mindent kizárna. A `/` vagy `*` sort, illetve azt, amely egy egész mentett mappát nevez meg, figyelmen kívül hagyja, így egy kizárás soha nem ürítheti ki a mentést. Egy egész mappa kihagyásához inkább vedd ki a pipát a **Mentett mappák** alatt.

Jól ismert image-eknél, például Plex, Jellyfin, Sonarr, Radarr, Immich és Nextcloud esetén a kártya felkínálja azokat a mappákat, amelyeket az alkalmazás magától újra feltölt, például gyorsítótárakat, naplókat és bélyegképeket. Semmi nem kerül kizárásra, amíg meg nem nyomod a **Kijelöltek kizárása** gombot.

Ugyanez a kártya olyan figyelmeztetéseket is hordoz, amelyeket egy mappaellenőrzés nem láthat. Az Immich például az albumait, arcait és dátumait egy külön konténerben lévő PostgreSQL-adatbázisban tartja, így az Immich konténer önmagában vett fájlmentése a képeket úgy állítja vissza, hogy ezek egyike sincs meg, és a visszaállítás úgy néz ki, mintha sikerült volna. Mentsd le az adatbázis-konténert is.

## Adatbázis-mentések {#database-dumps}

Az általa felismert PostgreSQL-, MySQL- és MariaDB-konténereket a BombVault minden mentés előtt lementi (dump), a futó szerverről, a fájlok mellett egy saját visszaállítási pontba. Ez lefedi a hivatalos image-eket, a PostGIS-t, a TimescaleDB-t, a pgvectort, a pgautoupgrade-et, az Immich adatbázis-image-eit, a linuxserver, yobasystems és jc21 MariaDB-image-eket és az Oracle mysql-server-ét. Az a konténer, amely csak adatbázisnak látszik, ugyanezt a lehetőséget kapja, kikapcsolva, amíg nem választod be.

A dump közvetlenül a tárolóba folyik, és soha nem íródik lemezre. A hitelesítő adatok a konténer saját változóiból jönnek, a `*_FILE` titkokat is beleértve, és soha nem hagyják el. A konténer Beállítások lapján lévő **Adatbázis** kártya megmondja, hogy az adatmappa leállított állapotban mentődik, futás közben másolódik, vagy nem mentődik, és ennél a konténernél be- vagy kikapcsolja a dumpot. A `bombvault.dbdump=false` címke ugyanezt teszi, és erősebb a kártyánál. Az **Automatikus adatbázis-mentések** a **Beállítások, Általános** alatt mindegyikre egyszerre kapcsolja.

Egy sikertelen dump nem buktatja el a mentést. Saját sikertelen futásként jelenik meg az okkal, és értesítést küld. A dumpokat soha nem tölti vissza magától: tölts le egyet, sima vagy tömörített formában, mentsd mappába, vagy importáld egy frissen indított adatbázisba egy kattintással a **Biztonsági mentések** lapon. A dump visszaszerzéséhez a BombVault nélkül lásd: [Egy adatbázis-dump visszaszerzése](offsite-recovery.md#database-dumps). Ha egy meghiúsul, lásd: [Egy adatbázis-dump sikertelen volt](troubleshooting.md#a-database-dump-failed).

## Parancsok és más konténerek {#commands}

- A **Parancsok előtte és utána** a konténeren belül futnak `sh -c` segítségével, például hogy egy gyorsítótárat lemezre írjanak. A mentés előtti sikertelen parancs megszakítja azt. Az a mentés előtti parancs, amely 75-ös kóddal lép ki, ehelyett elhalaszt egy ütemezett mentést, lásd lent.
- A **Más konténerek leállítása** megnevez konténereket, amelyek leállnak, amíg ez menti magát, például az adatbázisát. Utána a Compose `depends_on` sorrendjükben újra elindulnak, és alapértelmezés szerint mindegyiknek egészségesnek kell jelentkeznie, mielőtt a következő indul. A várakozás és az időkorlátja a **Beállítások, Ütemezések** alatti **Sorrend** kártya **Újraindítás biztonsági mentés után** részénél van.
- Az a konténer, amely egy leálló konténer hálózati, PID- vagy IPC-névterén osztozik a `--net=container:` vagy a Compose `network_mode: service:` beállítása révén, vele együtt áll le és indul, akkor is, ha nincs a listán, és annak a konténernek az indulása után indul, amelyhez csatlakozik. Egy újraindított konténer új hálózatot kap, és a régi hálózaton futva hagyott konténernek csak a loopbackje maradna. Maga a lista úgy marad, ahogy beállítottad. Ha egy ilyen konténert nem sikerült leállítani, a futás jelzi, hogy nincs hálózata, amíg újra nem indítják. Ugyanez érvényes a BombVault saját konténerére, amely soha nem állítja le önmagát, és nem tehető fel a listára.
- Az a konténer, amely utána nem fut újra, a futásban és egy értesítésben is név szerint szerepel, akkor is, ha maga a mentés meghiúsult.

## Ha az alkalmazás foglalt {#app-busy}

Egy mentés leállítja a konténerét, ezért az ütemezett mentés várhat, amíg valaki használja az alkalmazást, és magától újra próbálkozhat. A konténer Beállítások lapján lévő **Ha az alkalmazás foglalt** kártya állítja be:

- Az **Aktivitásellenőrzés** az ütemezett mentés előtt megkérdezi a Plexet, Jellyfint vagy Emby-t, nézi-e valaki. A Plexet a saját `Preferences.xml` fájljából vett tokennel kérdezi, így nem kell semmit beírni. A Jellyfin és az Emby **API-kulcsot** kér, amelyet a szerver vezérlőpultján az API-kulcsok alatt hozol létre.
- A **Szüneteltetett munkamenetek kihagyása ennyi után (perc)** hagyja, hogy egy éjszakára megállított film ne számítson.
- A **Mentés, ha a szerver nem válaszol** azt dönti el, mi történik, ha a szervert nem lehet megkérdezni: bekapcsolva a mentés lefut, és a napló ezt rögzíti. Kikapcsolva úgy vár, mintha valaki nézné.
- A **Várjon, amíg a konténer adatot küld** olyan alkalmazásokhoz való, amelyeknek nincs ilyen ellenőrzése, például egy játékszerverhez vagy letöltőhöz. A mentés addig vár, amíg a konténer többet küldött, mint a **Foglalt ettől (KiB/s)**, az elmúlt **Csendes ennyi ideig (perc)** percben bármelyik leolvasásnál.
- Az **Újrapróbálás ennyi időnként (perc)**, a **Csak egy időablakon belül próbálkozzon** a **Próbálkozás ideje** értékkel, és a **Legfeljebb ennyit várjon (óra)** állítja be, hogyan folynak a próbálkozások.
- **A várakozás végén**: vagy feladja, ami a mentést a **Figyelmet igényel** alatt mutatja az **Újrapróbálás** gombbal, vagy **Mentés mégis**.

A várakozó mentés **Elhalasztva** állapotként, borostyánsárgán van rögzítve, és a futás többi része folytatódik. Az áttekintés, a lista és az elem a következő próbával együtt mutatja, a késés-riasztás és a védelmi állapot pedig a határidejéig vár rá. Egy értesítést kapsz, amikor elhalasztódik, és egyet, amikor véget ér, nem próbálkozásonként egyet. Egy próba vár, amíg egy másik mentés fut, és a várakozás újraindítás után is folytatódik. A magad indította mentés mindig azonnal lefut, akár a **Biztonsági mentés most**, az **Újrapróbálás**, egy kijelölés, az API vagy az MCP indítja, és az, amelyik sikeresen lefut, véget vet egy függőben lévő várakozásnak.

A konténer Beállítások lapján van egy régebbi, egyszerűbb mód is: a **Várjon, amíg az alkalmazás nyugalomban van** a **Legfeljebb ennyit várjon (óra)** értékkel. A mentés akkor indul, amint az alkalmazás nyugalomban van, amit a **Beállítások, Ütemezések** alatti **Nyugalom biztonsági mentés előtt** kártya határoz meg. Egy Compose-stack ugyanabban a futásban esedékes tagjai együtt várnak, és a várakozó mentés nem tart zárat, így a többi konténer előremegy.

![A Ha az alkalmazás foglalt kártya egy Plex-konténernél, beállított aktivitásellenőrzéssel.](assets/screenshots/backups/app-busy-dark.png#only-dark)
![A Ha az alkalmazás foglalt kártya egy Plex-konténernél, beállított aktivitásellenőrzéssel.](assets/screenshots/backups/app-busy-light.png#only-light)

### Példa: a Plex vár, amíg valaki néz {#example-plex}

A teljes mentés minden éjjel 03:00-kor fut. Senkinek nem kellene látnia, hogy egy film megáll, mert a Plex leállt a mentése miatt, de a mentésnek reggelig mégis meg kell történnie.

1. Nyisd meg a **Biztonsági mentések** oldalt, majd a `plex` elemet, aztán a **Beállítások** lapját.
2. A **Ha az alkalmazás foglalt** alatt állítsd az **Aktivitásellenőrzés** értékét Plexre. Nem kell semmit beírni.
3. Állítsd a **Szüneteltetett munkamenetek kihagyása ennyi után (perc)** értékét `30`-ra, hogy egy éjfélkor megállított film fél egykor már ne számítson.
4. Hagyd bekapcsolva a **Mentés, ha a szerver nem válaszol** beállítást.
5. Állítsd az **Újrapróbálás ennyi időnként (perc)** értékét `15`-re.
6. Kapcsold be a **Csak egy időablakon belül próbálkozzon** beállítást, és a **Próbálkozás ideje** legyen `22:00` és `06:00`.
7. Állítsd a **Legfeljebb ennyit várjon (óra)** értékét `3`-ra, és az **A várakozás végén** értékét **Mentés mégis**-re.

Mi történik egy olyan éjszakán, amikor valaki 03:00-kor még néz valamit: a BombVault megkérdezi a Plexet, talál egy lejátszó munkamenetet, és a mentést **Elhalasztva** állapotban rögzíti, a következő próbával 03:15-kor. A futás többi konténerét a szokásos módon menti. Amikor a film 04:10-kor véget ér, a 04:15-ös próba nyugalomban találja a Plexet, lementi, és a várakozás egy értesítéssel véget ér. Ha a film 06:00-kor még futna, a várakozás a végére érne, és a Plexet mégis lementené, így a mentései soha nem maradnak három óránál többet le.

A saját ellenőrzésedhez add meg, hogy a **Biztonsági mentés előtti parancs** 75-ös kóddal lépjen ki, amíg az alkalmazás foglalt, például `[ "$(cat /tmp/players)" = 0 ] || exit 75`. További tünetek és megoldások a [Hibaelhárításban](troubleshooting.md#deferred-backups) vannak.

## Sima exportok {#exports}

A restic marad a motor, de az export olyan másolatot ad, amelyhez sem a BombVault, sem a restic nem kell.

- A **Konténer exportálása** és a **VM exportálása** az elem oldalán a **Több** alatt a jelenlegi állapotról ír másolatot a tároló mellé: a konténer mentett mappáit az Unraid-sablonjával, vagy a VM lemezképeit a definíciójával, amely `virsh define` paranccsal visszaállítható.
- Egy pillanatkép **Exportálás** művelete egy tárolt mentést ír a szerveren az exportmappába tarként, zipként vagy saját restic-tárolóként, és egy pillanatképet vagy az összeset is elvihet. A flash-mentés és az önmentés kulcsokat tartalmaz, ezért az exportálásukhoz bejelentkezési jelszó és magán a példányon történő bejelentkezés kell.

Az exportok alapértelmezés szerint sima fájlok. Az **Exportok titkosítása age-dzsel** a **Beállítások, Rendszer** alatt lezárja őket, a flash ZIP-et is, az általad felsorolt címzettek számára, amelyek mindegyike egy age nyilvános kulcs vagy egy SSH nyilvános kulcs. Később a megfelelő privát kulccsal fejted vissza őket a gépen kívül. Bekapcsolt titkosítás és érvényes címzett nélkül az export egyértelmű hibával meghiúsul ahelyett, hogy valaha sima fájlokat írna.

## Amit egy futás elmond {#run-notes}

- **Időzítés.** Minden futás kiírja az indulást, a befejezést és az időtartamot, és minden elemnek megvannak a saját **Legutóbbi futásai**.
- **Miért volt lassú egy mentés.** Amíg egy mentés fut, a BombVault figyeli, mennyire foglalt a CPU, a lemezek és a hálózat. Ha egy mentés sokkal tovább tart a szokásosnál, és valami egyértelműen a határán volt, a futás ezt kiírja, például „A cél lemez, a disk1, 98%-ban foglalt volt”. Egyébként nem mond semmit.
- **Az ellenőrzések és prune-ok haladása.** A tevékenységnapló és a **Mentések ellenőrzése** kártya megmutatja, meddig számolt a restic, például *47-ből 12 csomag*, az adott lépés hátralévő idejével, ha már van elég adat a becsléshez.
- **Fájlok, amelyeket nem tudott elolvasni.** Egy mentés, amely lefutott, de kihagyott valamit, narancssárgára színezi a sorát, és megnevezi, mit hagyott ki.
