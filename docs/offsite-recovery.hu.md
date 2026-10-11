# Telephelyen kívüli mentés és helyreállítás

!!! note "A telephelyen kívüli másolatok várnak egy újraépítés után"
    Amikor a Vészvarázsló vagy a **Biztonsági mentések felfedezése** a régi beállítások nélkül építi újra az elemeket, ezeknek a szakaszoknak a telephelyen kívüli másolatai szünetelnek, amíg az alapértelmezésüket meg nem erősíted. Lásd: [Újraépítés után](#after-a-rebuild).

A helyi mentések megvédenek egy elveszett konténertől vagy egy rossz frissítéstől. A telephelyen kívüli másolatok és egy tesztelt helyreállítási csomag megvéd a teljes gép elvesztésétől, a zsarolóvírustól vagy egy tűztől. Ez az oldal a célokat és a telephelyen kívüli másolatokat, a másolat manipulációbiztossá tételét, a visszaállíthatóság bizonyítását, több szerver párosítását, valamint azt ismerteti, hogyan kapsz vissza mindent, ha maga a BombVault eltűnt.

## Telephelyen kívüli másolatok {#off-site-copies}

Tartsd meg a gyors helyi mentést, és adj hozzá egy vagy több telephelyen kívüli célt. A BombVault az új mentéseket `restic copy` segítségével, legjobb szándék szerint másolja oda, így egy telephelyen kívüli zökkenő soha nem hibáztatja el a helyi mentést. Ebben a felállásban a helyi tároló marad az elsődleges, a telephelyen kívüli pedig egy másolat, de egy szakasz elsődleges tárolójának egyáltalán nem kell helyinek lennie; lásd lejjebb a [Távoli elsődleges tárolók](#remote-primary-repositories) szakaszt arról, hogyan menthetsz közvetlenül S3-ra, rest-serverre és hasonlókra ahelyett, hogy oda másolnál.

- **Több cél szakaszonként.** Minden szakasz (konténerek, VM-ek, flash, Önmentés, mappák és ZFS-adatkészletek) egyszerre több telephelyen kívüli célra is másolhat, így párhuzamosan tarthatsz például egy rest-servert egy barátod gépén és egy S3-bucketet is. A Célok oldal **Célok szakaszonként** kártyája kapcsolja be vagy ki a szakaszból a célba vezető vonalakat.
- **Mikor futnak a másolatok.** Egy cél **Másolás** kártyáján a **Mikor másoljon** adja meg az időpontot: **Minden biztonsági mentés után**, vagy egy saját terv, amely összegyűjti a mentéseket, és rögzített időpontokban másol. A **Beállítások, Ütemezés, Telephelyen kívüli másolatok szakaszonként** alatt a **Kivétel** egy szakasznak saját másolási tervet ad. A **Replikálás most** egy szakasz **Külső másolat** kártyáján a **Beállítások, Célok** alatt igény szerint elindít egy másolást.
- **A megőrzést** célonként a **Meddig őrizze meg** alatt állítod be, így a telephelyen kívüli másolatokat archívumként hosszabban is megtarthatod. A szakasz telephelyen kívüli mezőjében lévő másolat a **Beállítások, Célok** alatti **Telephelyen kívüli megőrzés** kártyát követi; hagyd csupa nullán, hogy a telephelyen kívüli mentéseket soha ne vágja vissza automatikusan.
- **Sávszélesség-korlátok** (**Telephelyen kívüli sávszélesség** a **Beállítások, Célok** alatt, vagy egy cél **Másolás** kártyája) korlátozzák a restic feltöltési/letöltési sebességét, hogy a másolás ne telítse a WAN-odat.
- Amíg egy másolás fut, az Áttekintés állapotfejléce ezt írja: **Telephelyen kívüli másolás folyamatban**. Ez aktív jelzés, nem százalékos folyamatjelző, mert a `restic copy` nem ad gépileg olvasható előrehaladást.

!!! note "Visszaállítás bármelyik helyről"
    Minden elem a **Biztonsági mentések** lapján egyetlen idővonalként sorolja fel a mentéseit mindazokon a helyeken át, ahol egy mentés megtalálható. A B2-re másolt mentés egyszer jelenik meg, megjelölve minden hellyel, amely tartalmazza. A visszaállítás az első elérhető helyet használja, azzal a tárolóval kezdve, amelyre az elem íródik, és mentésenként választhatsz másikat. A telephelyen kívüli célokat csak akkor olvassa a BombVault, ha az **Ellenőrzés** gombra kattintasz, mert a felhőtár listázása időbe telik. Ha egy helyen törölsz, a BombVault előbb megnézi a többit, és megmondja, hogy ez volt-e az utolsó másolat.

## Célok {#destinations}

A cél egy hely, ahová a mentések kerülnek: egy helyi mappa, egy megosztás, egy saját szerver vagy egy felhőszolgáltatás. Egyszer kell beállítani, utána minden szakaszt kiszolgál. A **Célok** oldalt a beállítások csempéjéről nyithatod meg, vagy kapcsold be ott a **Mutatás az oldalsávban is** opciót, hogy kitűzd.

A **Cél hozzáadása** négylépéses ablakot nyit:

1. **Szolgáltató.** Minden szolgáltatás a logójával szerepel, fölötte kereséssel, csoportokban: S3-bucketes tárolószolgáltatások (Backblaze B2, Wasabi, Cloudflare R2, Hetzner Object Storage, Amazon S3 és mások), a saját S3-kiszolgálód (Garage, SeaweedFS, RustFS, Silo, Ceph, JuiceFS, Versity S3 Gateway), a saját kiszolgálód és megosztásaid (rest-server, Hetzner Storage Box, SFTP, SMB, WebDAV, egy csatolt útvonal), felhőtárolók (OneDrive, Google Drive, Dropbox, pCloud, Nextcloud és minden más, amit az rclone támogat), valamint a csoportod fogadószerverei. Mindegyik megmondja, mennyire illik mentéshez: a felhőmeghajtók sok kérés alatt lelassulnak, ezért az első mentés és a metszés ott tovább tart.
2. **Bejelentkezés.** A mezők a szolgáltatástól függnek: hozzáférési kulcs S3-hoz, felhasználó és jelszó WebDAV-hoz és SMB-hez, alkalmazásjelszó ott, ahol a kétlépcsős bejelentkezés blokkolja a megszokottat, a BombVault nyilvános SSH-kulcsa SFTP-hez és a Storage Boxhoz, vagy token a böngészőn át bejelentkező szolgáltatásokhoz. Ezekhez az ablak egy `rclone authorize` parancsot mutat, amelyet egy böngészős számítógépen kell lefuttatni; a kiírt token a mezőbe kerül. A **Teszt** a mentés előtt ellenőrzi a bejelentkezést. Az ablak ezután listázza a cél mappáit, az **Új mappa** gombbal újat hozhatsz létre, és kiírja a szabad helyet, ahol a szolgáltatás jelenti. Egy üres mappa a legbiztonságosabb; a BombVault szakaszonként egy mappát hoz létre benne.
3. **Védelem.** Az ablak nyíltan megmondja, mit tud a szolgáltatás. Az append-only módban futó rest-server megtagadja a törlést, és a manipulációs teszt ezt ellenőrzi. Egy S3-bucket meg tudja őrizni a régi verziókat verziókezeléssel és object lockkal, ezt a BombVault még nem tudja ellenőrizni. A felhőmeghajtó egyáltalán nem tudja megtagadni a törlést: aki bejut a szerverre, bejut abba a másolatba is. A **Törlés elleni védelem (append-only)** kapcsolót csak ott kapcsold be, ahol a túloldal valóban megtagadja a törlést; a BombVault ott soha nem metsz. A **Vészhelyzetre** megmondja, mit tartalmaz a helyreállítási csomag ehhez a célhoz.
4. **Mely szakaszokhoz.** Válaszd ki a szakaszokat, amelyek ide mentenek. Az első másolat az egész előzményt magával viszi. Azok az elemek, amelyek saját célokat választottak, nem kapják meg az újat maguktól, és az ablak név szerint felsorolja őket.

Az S3-szolgáltatások a restic saját S3-háttérrendszerén futnak, ez teszi lehetővé a tárolási osztály és az object lock alkalmazását. Minden más szolgáltatás a BombVault mellé csomagolt rclone-on fut, és a távoli eleme ezután a **Külső helyszín (rclone)** kártyán jelenik meg a **Beállítások, Célok** alatt. A beállítások exportja tartalmazza a célokat; a hitelesítő adatokkal együtt a bejelentkezésüket is.

Minden célnak saját oldala van:

- **Kapcsolat** a névvel, az **Elérhető** kapcsolóval (kikapcsolva a BombVault már nem ír ide, és az ide mentő elemek érthető üzenettel hibáznak ahelyett, hogy máshová kerülnének), a mappával, a tömörítéssel, a tárolási osztállyal és a **Telephelyen kívül** jelöléssel. A **Kapcsolat módosítása** teszteli és menti az új címet vagy bejelentkezést.
- **Hitelesítő adatok**: a megosztott készlet vagy egy saját.
- **Törlés elleni védelem**, a **Törlés elleni védelem tesztelése most** gombbal, és azzal, hogy ki metszi ezt a másolatot.
- **Másolás**: mikor másoljon, valamint a feltöltési és letöltési korlátok.
- **Meddig őrizze meg**: **Rövid**, **Kiegyensúlyozott**, **Hosszú** vagy **Saját értékek**, idővonallal arról, hogy a szabály mely mentéseket tartja meg. A **Rákérdezés a kevesebb megőrzés előtt** rákérdez, mielőtt egy rövidebb szabály olyan mentéseket törölne, amelyeknek az egyetlen másolata itt van.
- **Szabályok szakaszonként**: egy szakasz saját tárolója, megőrzése vagy tömörítése ennél a célnál. A **Szabályok módosítása** megnyitja őket, a **Vissza a cél értékeihez** pedig visszaviszi a szakaszt a cél értékeihez.
- **Már ennek a célnak az alatt van**: szakaszok, amelyek kézzel beírt címe ennek a célnak egy mappájában van. Az átvétel a tárolót ott hagyja, ahol van; a bejelentkezés, a tárolási osztály és a törlés elleni védelem ezután a céltól jön. A BombVault előbb ellenőrzi, hogy a cél bejelentkezése megnyitja-e a tárolót, és megtagadja, hogy egy append-only tárolót olyan cél alá tegyen, amely nem append-only.
- Egy edény, amely mutatja, mennyire tele van a cél, a jelöléssel, hogy hol telik meg, és hogy az utolsó hetek növekedése mellett hány hét van hátra. Az a cél, amely nem jelenti a méretét, ezt írja: **A méret ismeretlen**.

A csoportod egy másik példánya által üzemeltetett fogadószerver a szolgáltatók között **A csoportodból** néven jelenik meg; lásd: [Fogadószerver](#receiving-server).

### Költöztetés és eltávolítás {#move-remove}

- A **Költöztetés** egy szakasz másolatát az egész előzményével egy másik célra viszi. Minden a régi helyen marad; a BombVault ott nem töröl semmit. A költöztetés a háttérben fut, és sokáig tarthat: kb. 9 perc 100 GB-onként helyi lemezek között, óráig is tarthat lassú kapcsolaton. Az Áttekintés mutatja, amíg fut.
- A **Cél eltávolítása** két utat kínál. A **Cél eltávolítása, a mentések megtartása** nem ír tovább oda, a mentéseket a helyükön hagyja; később elolvashatod őket a **Visszaállítás, Régi mentési mappából** alatt. A **Cél eltávolítása és az ottani mentések törlése** előbb beolvassa, mi van ott tárolva, és felsorolja: mely mentések tűnnek el, ezek közül melyek nincsenek máshol, és mely tárolók maradnak, mert a törlés elleni védelem megtagadja, elemek még közvetlenül oda mentenek, egy másik cél vagy szakasz tartja ott a másolatait, vagy nem lehetett őket olvasni. Azok az elemek, amelyek elveszítenék az egyetlen mentésüket, külön szerepelnek, és megmaradnak, kivéve ha bejelölöd az **Ezeket is töröld** opciót. A cél nevének beírásával erősíted meg, és a törlés a háttérben fut.
- Egy cél mentéseinek törléséhez bejelentkezési jelszó és bejelentkezés kell a példányon. Jelszó nélkül a cél eltávolítható, csak a mentései nem törölhetők innen.

## Elhelyezés elemenként {#placement}

Egy elem Beállítások lapján az **Ütemezés és cél** kártya gombsort tartalmaz: **Helyi**, valamint egy gomb a szakasz minden telephelyen kívüli céljához. A világító gombok kapják az elem mentéseit.

- Ha a **Helyi** világít, az elem a **Tárolva itt** alatt látható tárolóba íródik, és átmásolódik minden más világító célra. Sötétíts el egy célt, és az nem kap többé újat ettől az elemtől. A csak Helyi sehová sem másol, ami olyan adatnak jó, amelynek már van második másolata, például egy NAS-on élő megosztásnak.
- Ha a **Helyi** sötét, az elem közvetlenül az első világító cél közvetlen tárolójába íródik, onnan pedig átmásolódik a többi világító célra. Először egy párbeszédablak létrehozza ezt a közvetlen tárolót.
- Egy gomb mindig világít, mert a mentésnek kell egy hely, ahová menjen. Ha egy elemet ki akarsz hagyni a mentésekből, használd a **Ne legyen mentés** opciót.

A hely az elem első mentésétől fogva rögzített, mert a BombVault soha nem mozgat mentéseket tárolók között; a Beállítások lap **Tároló** kártyája megnevezi. A másolatok bármikor változhatnak. Az a cél, amely már nem kap egy elemet, megtartja a meglévő másolatait, és a szakasz következő telephelyen kívüli futásakor a saját megőrzésére vágja őket vissza. A csak hozzáfűzésre szolgáló célokból nem lehet törölni.

A sor alatt a kártya megmondja, hová megy az elem, és mi van valójában ott: hány telephely tartja, mikor látták utoljára az egyes célokat, és teljesül-e a 3-2-1. Telephely az eredeti adatokat tartalmazó szerver, minden telephelyen kívüli cél és minden **Telephelyen kívül** jelölésű tároló. A BombVault a másolatokat és a telephelyeket ellenőrzi; a 3-2-1 „két adathordozó” részét nem.

### Alapértelmezések szakaszonként {#placement-defaults}

A Célok oldal **Célok szakaszonként** kártyája minden szakasz alapértelmezése. Az ábrán minden vonal egy másolat, egy vonalra vagy pontra kattintva be- vagy kikapcsolod; a szaggatott vonalak ki vannak kapcsolva. Az **Ábra / Kapcsolók** ugyanezt gombsorokként mutatja. A másolatok azonnal érvényesek minden olyan elemre, amelynek nincs saját választása, és a Compose-stackek projektmappáira is. A hely egy új elemre az első mentésénél érvényes; módosítása nem mozgat mentéseket. Mentés előtt a BombVault megnevez minden célt, amely elemeket nyer vagy veszít, és hogy ez hány mentést jelent.

Azok az elemek, amelyek máshová mentenek, mint a szakaszuk, a **Saját választás** kártyán állnak, mindegyik **Alapértelmezés követése** gombbal. Az új mentések ezután a szakasz céljaira mennek, a korábbiak ott maradnak, ahol vannak.

Az új telephelyen kívüli cél minden olyan elemet megkap, amely nincs Helyire állítva. Az ablak, amely hozzáadja, megmondja, hány elemről van szó, és ahol ismert, mennyi előzményről.

### Közvetlen tárolók

Ha egy elemnél kikapcsolod a Helyit, hogy egy közvetlen tároló nélküli cél legyen az otthona, megnyílik egy párbeszédablak a cél mellé javasolt hellyel, például `s3:https://s3.eu-central-003.backblazeb2.com/bucket/containers-direct`, és egy kapcsolatteszttel, amely nem hoz létre semmit. A **Létrehozás és használat** létrehozza a tárolót, és az elemet odairányítja. A közvetlen tároló átveszi a cél kulcsát, tárolási osztályát, korlátait, append-only beállítását és megőrzését, és ezekkel együtt változik; a Tároló kártya csak olvashatóan mutatja. Ha a cél új kulcsa nem tudja megnyitni, a közvetlen tároló megtartja a kulcsát, és a mentés ezt jelzi. A közvetlen tárolón lévő elem onnan másolódik a többi világító célra, soha nem arra a célra, amelyhez a tároló tartozik. A mentéseik `bv:direct` címkét viselnek, és minden más megőrzési menet megtartja őket, így az a közvetlen tároló, amely elvesztette a kapcsolatát a céljával, soha nem öregszik a helyi szabályok szerint. A B2-t az S3-végpontján éred el, a kulcsazonosítót és az alkalmazáskulcsot S3-hitelesítő adatként megadva; a cél saját mappájára korlátozott kulcs nem éri el a mellette lévő mappát, ezért a kulcsot inkább az fölötte lévő mappára korlátozd.

### Telephelyen kívül

Egy cél vagy egy elnevezett tároló megjelölhető **Telephelyen kívül**. A távoliak jelöltként indulnak; kapcsold ki egy ugyanabban az épületben lévő rest-servernél. A jelölés csak a telephelyeket és a 3-2-1-et számolja a kártyákon. Egyetlen másolatot sem változtat.

A ZFS-replika telephelyen kívüli másolat, de nem előzményes mentés. Csak az általad beállított pillanatképeket őrzi meg, a régi verziók és az egyes fájlok továbbra is a mentésekből jönnek. Az egyik ZFS-kiszolgálóról közvetlenül a másikra megy, SSH-n át egy ZFS-kiszolgálóra vagy egy párosított példány fogadóvégpontjára. A párosítási csoport csak a kérést szállítja. Lásd: [Replika](zfs-datasets.md#replica).

### Újraépítés után {#after-a-rebuild}

A másolási döntések a BombVault saját beállításaiban élnek. A **Biztonsági mentések felfedezése** vagy a Vészvarázsló utáni újraépítés után, visszaállított beállítások nélkül, ezek elvesznek, és mindent másolni azokat az elemeket is újra B2-re küldené, amelyeket kihagytál. Ezért minden újraépített szakasz telephelyen kívüli másolatai szünetelnek. Az Áttekintés ezt jelzi, a **Célok szakaszonként** kártya pedig felkínálja az **Alapértelmezés megerősítése** gombot egy előnézettel arról, mit másol a következő futás, és azokkal a nevekkel a mentésekben, amelyekhez nincs elem, és amelyeket ott kihagyhatsz. Csak a megerősítés szünteti meg a szünetet; egy beállításfájl importálása visszahozza a szabályokat és az alapértelmezéseket, de nem szünteti meg.

## Távoli elsődleges tárolók {#remote-primary-repositories}

Egy szakasz saját útvonala nem korlátozódik helyi mappára: irányítsd közvetlenül egy restic távoli címre (`s3:...`, `rest:http://host:8000/repo`, `sftp:user@host:/repo`, `rclone:remote:bucket/path`), és a BombVault közvetlenül oda ment, külön helyi másolat és másolási lépés nélkül. Ez más felállás, mint a fenti telephelyen kívüli másolatok: ott a helyi tároló az elsődleges, a telephelyen kívüli pedig legjobb szándékú archívuma; itt a távoli tároló **maga az** elsődleges, és az egyetlen másolat, hacsak a szakaszt nem másolod egy másik célra is.

Az egyes szakaszok útvonalát (Konténerek, VM-ek, Flash, Önmentés, Mentett mappák, ZFS-adatkészletek) a **Helyi mappa** cél oldalán állítod be: a **Kapcsolat** kártyán a szakasz melletti **Módosítás** egy **Helyi / Távoli** kapcsolós ablakot nyit:

- A **Helyi** a megszokott mappaböngészőt mutatja.
- A **Távoli** ezt egy sima URL-mezőre cseréli, és azokra a kapcsolatbeállításokra, amelyeket egy telephelyen kívüli cél használ, itt ehhez az elsődlegeshez beállítva. Innen kapod:
    - **Kapcsolatteszt** az éles útvonalon, mielőtt rá hagyatkoznál.
    - **Sávszélesség-korlátok** (feltöltés/letöltés), hogy egy távoli elsődlegesre ütemezett mentés ne telítse a WAN-odat: ugyanazok a `--limit-upload`/`--limit-download` restic kapcsolók, amelyeket a telephelyen kívüli másolatok használnak, magára a mentésre alkalmazva.
    - **Append-only védelem**, ugyanazzal az aktív manipulációs teszttel ellenőrizve (valódi DELETE-próba a túloldalon), amelyet a telephelyen kívüli célok kapnak. Bekapcsolva a BombVault megtagadja a tároló metszését, mert nincs mögötte külön helyi másolat, és a gépen lévő hitelesítő adatok nem törölhetik a mentés egyetlen másolatát.
    - **Növekedésikeret-riasztás**, ugyanabból a tárolóméret-trendből mintavételezve, amelyet a Tárhely kártya már követ.

Mindez nem kötelező: egy kézzel beírt távoli útvonal mentett biztonsági beállítások nélkül korlátlanul, metszhetően és keretriasztás nélkül ment. A biztonsági beállítások arra valók, ha azokat a védelmeket akarod, amelyeket egy telephelyen kívüli másolat kap, anélkül hogy emiatt külön célt kellene felvenned.

!!! note "A felhős és REST hitelesítő adatok közösek"
    A távoli elsődleges a **Beállítások, Célok** alatti megosztott felhős hitelesítő adatokkal hitelesít; az elsődleges tárolóknak nincs külön hitelesítőadat-tára.

### SMB és WebDAV gazdagép-csatolás nélkül {#smb-webdav}

A **Beállítások, Célok** alatti **Külső helyszín (rclone)** kártyán van egy űrlap Windows- vagy Samba-megosztáshoz és WebDAV-kiszolgálóhoz (Nextcloud, ownCloud, SharePoint vagy bármely más). Add meg a rövid nevet, a gazdagépet és a megosztást (SMB), vagy az URL-t és a kiszolgáló típusát (WebDAV), a felhasználót és a jelszót, és a BombVault megírja helyetted az rclone-szakaszt. Az rclone tárolás előtt maga elhomályosítja a jelszót; egy már létező nevű hozzáadása felülírja azt a szakaszt, nem vesz fel egy másodikat. A **Cél hozzáadása** SMB-t és WebDAV-ot is kínál szolgáltatóként, ugyanezekkel a mezőkkel.

Az űrlap a kész helyet adja vissza, például `rclone:nas:backups`. Használd egy szakasz útvonalaként vagy célként, és adj hozzá almappát, ha akarsz (`rclone:nas:backups/bombvault`). A megosztás az útvonal első szegmense, nem a név része.

Ez jobb út, mint a megosztás csatolása Unraidon: a restic nem javasolja, hogy a tároló csatolt CIFS-megosztáson legyen, itt pedig semmi sincs csatolva. Az NFS azért nincs az űrlapon, mert sem a restic, sem az rclone nem rendelkezik NFS-háttérrendszerrel; NFS-hez csatold az exportot a gazdagépen, és használd helyi mappaként.

## Törlés elleni védelem (append-only)

Kapcsold be a törlés elleni védelmet egy telephelyen kívüli célnál, hogy a zsarolóvírus vagy egy feltört gazdagép ne törölhesse vagy írhassa felül a mentéseidet. A túloldal (egy `--append-only` módban futó `restic/rest-server`) **érvényesíti**. A BombVault csak **ellenőrzi**, és soha nem mutat zöldet pusztán egy beállítási állítás alapján. Az ezen a szerveren lévő mappa nem tagadhatja meg a törlést, mert aki a BombVaultot irányítja, a mappát is irányítja, ezért a helyi célok nem kínálják a kapcsolót.

A leggyorsabb út egy append-only másolathoz egy párosított BombVault [fogadószerverrel](#receiving-server): ez helyetted futtatja a rest-servert, append-only módban.

!!! note "A `/locks/` alatti sikeres törlés várható"
    Az append-only nem azt jelenti, hogy soha semmit nem lehet eltávolítani. A restic-nek fel kell vennie és el kell engednie a saját zárait, ezért a `/locks/` szándékosan írható és törölhető marad. A mentéseket és a mögöttük lévő adatokat, amelyekre a zsarolóvírus menne, nem lehet eltávolítani. Ha magad próbálgatod a túloldalt, a `/locks/` alatti sikeres törlés helyes viselkedés, és nem lyuk a védelemben.

!!! warning "Az append-only tárolókat ez a gép soha nem metszi"
    Egy append-only cél innen soha nem metszi a régi mentéseket; a metszés a túloldal dolga. Állíts be hozzá **növekedésikeret-riasztást**, hogy időben értesülj, mielőtt a tároló mérete elszabadul.

## Manipulációs teszt

A BombVault rendszeresen bizonyítja az append-only garanciát azzal, hogy ténylegesen törlést kísérel meg a telephelyen kívüli tárolón, egy nem létező objektumra célozva. A **Törlés elleni védelem tesztelése most** egy cél oldalán igény szerint lefuttatja, a **Törlésvédelem ellenőrzése** pedig a **Beállítások, Ellenőrzés, Automatikus ellenőrzések** alatt ütemezve futtatja.

- Az **elutasítás** azt jelenti, hogy védett.
- Az **elfogadás** azt jelenti, hogy nem védett.
- Egy **nem meggyőző** eredmény (a szerver elérhetetlen, hitelesítési hiba) soha nem billenti át a tárolt ítéletet.

Egy valódi védett-védtelen átbillenés egyetlen riasztást indít.

## DR-próbák

A BombVault kétféle szintű bizonyítékot kínál arra, hogy a mentéseid ténylegesen visszaállíthatók, nem csak jelen vannak.

- **Visszaállítás-ellenőrző próbák (helyi).** A BombVault rendszeresen lefuttatja a `restic check --read-data-subset` parancsot (korlátozott, soha nem lemezt megtöltő teljes visszaállítás). Az ütemezés a **Beállítások, Ellenőrzés, Automatikus ellenőrzések** alatt él, az eredményt ugyanezen a csempén a **Mentések ellenőrzése** kártya mutatja.
- **Próbavisszaállítás (telephelyen kívüli).** A BombVault egy valódi célt visszaállít a telephelyen kívüli tárolóból egy elzárt homokozóba, fájlonként és bájtonként ellenőrzi, majd takarít. Ez bizonyítja, hogy a telephelyen kívülről helyre tudsz állni, nem csak azt, hogy a tároló válaszol.

Az Áttekintés **Védelmi állapot** kártyája ezt szakaszonként négy ellenőrzésbe sűríti: friss mentés ezen a szerveren, másolat telephelyen kívül, törlés elleni védelem ott, és próbavisszaállítás onnan. Egy szakaszra kattintva megjelennek a részletek, és a kártya csak ellenőrzött tények alapján lesz zöld.

## Példányok párosítása {#pairing}

Több BombVault dolgozik együtt egy csoportként: megmutatják egymásnak a védelmi állapotukat, szerepeket vállalnak egymásért, és őrizhetik egymás kulcsát. Egy példány tizenkét szóval csatlakozik a csoporthoz.

Az első példányon nyisd meg a **Beállítások, Párosítás** oldalt, és nyomd meg a **Jelmondat létrehozása** gombot. Három lépéskártya mutatja a menetet. Tizenkét szó jelenik meg egy ablakban a **Másolás** gombbal. Minden további példányon nyisd meg ugyanazt a helyet, nyomd meg a **Jelmondat megadása** gombot, és illeszd be vagy írd be őket, vagy nyomd meg a **Beillesztés** gombot abban az ablakban. A listán nem szereplő szót gépelés közben a helyével együtt megnevezi, az utolsó szó pedig ellenőrzőösszeget hordoz, így egy elgépelt vagy felcserélt szó még a párosítás előtt kiderül. A jelmondatot csak egy példányon hozd létre: két példány, amely mindkettő létrehoz egyet, két külön csoportot alkot. Ha egy percig senki sem jelentkezik, az oldal két kiutat kínál: mutasd meg újra a szavakat, hogy ott add meg őket, vagy add meg a másik példány szavait, és egy lépésben csatlakozz az ő csoportjához. A párosítás bejelentkezési jelszó nélkül is működik, de állíts be egyet: nélküle bárki, aki megnyithatja a webes felületet, elolvashatja a szavakat, és a csoporton át olyan szerepeket kérhet, amelyek kiadják a benne lévő példányok restic-jelszavát. A párosítási kártya ezt jelzi, amíg nincs jelszó beállítva. Jelszóval a jelmondat ismételt megmutatásához is kell a jelszó. A **Csoport elhagyása** kivesz egy példányt.

Bárki, aki ismeri a szavakat, csatlakozhat a csoporthoz, ezért kezeld őket jelszóként.

**Hogyan érik el egymást a tagok.** Minden példány a bejelentkezés pillanatában a böngésződből tudja meg a saját hálózati címét, amely a párosítási oldalon az **Ismert címek** alatt szerepel, azokkal együtt, amelyeket a példány maga talált; javítsd ott, ha reverse proxy vagy szokatlan port áll előtte. Ugyanazon a hálózaton a tagok multicasttal hirdetik ezt a címet, és közvetlenül beszélnek, ahol pedig a multicast nem tud átmenni egy konténerhálózaton, például a Docker alapértelmezett bridge-én, a példány ehelyett a saját alhálózatán keresi a többit egy aláírt hívással, amelyre csak egy csoporttag tud válaszolni, így a párosítás relay nélkül is másodpercek alatt végbemegy. Ha semmi sem akad fenn, a **Nem találod?** egy kézzel megadott címet fogad, másik alhálózathoz vagy nem szokásos porthoz. A különböző hálózaton lévő példányok relayn át mennek, amelyet ugyanazon az oldalon választasz ki; négy formájához egy-egy kép tartozik:

- **Projekt-relay** (az alapértelmezés): `parleyport.halleluja.design`, ugyanaz a relay, amelyet a KnightLoader is használ. Nincs mit beállítani.
- **Saját relay**: az Unraid Community Appsből a [**ParleyPort**](https://github.com/junkerderprovinz/parleyport) konténer (a **Beállítások, Alkalmazások** alatt is), vagy egy olyan példányod, amely kívülről már elérhető, és be van kapcsolva a **Szolgáljon relayként**. Ez a példány ezután a `/relay/connect` címen válaszol a saját címén, a már meglévő reverse proxy és tanúsítvány mögött, és csak a csoportodat engedi be. Add meg a relay címét minden példányon, amelynek használnia kell.
- **Nincs relay**: a tagok automatikusan megtalálják egymást ugyanazon a hálózaton, máshol nem.

**Mit lát a relay.** A tagok közötti minden hívást AES-256-GCM-mel zárnak le a tizenkét szóból származtatott kulccsal, és ez a kulcs soha nem hagyja el a példányaidat. A relay megtudja a kapcsolatokat csoportosító hash-t, hogy egy üzenet melyik példánynak szól, mekkora, és mikor halad át. A helyi hálózaton a közvetlen hívást ugyanígy lezárják és alá is írják, így semmi sem függ az önaláírt tanúsítványtól, amelyet egy példány kiszolgál. Azokat a hívásokat, amelyek valamit kiadnak (szerepválaszok, restic-jelszó, fogadói bejelentkezések, kulcsletétek), ráadásul az őket küldő tag egy kulcsával is aláírják, amelyet az első találkozáskor rögzítenek; lásd: [Biztonsági modell](configuration.md#security-model).

**Mi halad át a csoporton.** A Példányok oldal védelmi állapota, egy szakasz azonnali ellenőrzésének kérése, a szerepkérések és válaszaik, célajánlatok, valamint az, amire egy fogadónak vagy lehívónak szüksége van, miután a szerepét engedélyezték: a másik példány tárolóhelyei és restic-jelszava. A mentési adatok soha; azok továbbra is közvetlenül a restic-háttérrendszerekhez mennek. Az APP_KEY sem: a restic-jelszó az adott példány tárolóit nyitja meg, és semmi mást, sem a tárolt titkait, a munkameneteit, sem a helyreállítási kódjait.

**Párosítás előtti bejegyzések.** A flottatokennel hozzáadott példányok, valamint a másik példány APP_KEY-ével beállított fogadott tárolók és lehívási források a frissítés után is megmaradnak, és **Újra párosítás** jelölést kapnak. Tovább működnek: az első indításkor a BombVault minden tárolt APP_KEY-t lecserél a belőle származtatott restic-jelszóra. Párosítsd mindkét példányt, majd szerkeszd a bejegyzést, és válaszd ki a példányát. Addig az ilyen bejegyzések a Példányok oldal **Példány nélkül** kártyáján állnak.

Az egyetlen hely, amely még kézzel kér APP_KEY-t, a [Régi mentési mappából](#restore-from-another-bombvault-repo), arra az esetre, ha a másik példány eltűnt, és nem tud válaszolni egy csoportban.

## Példányok és szerepek {#roles}

A **Példányok** oldal (egy beállítási csempe, amelyet kitűzhetsz az oldalsávra) a csoportot hálóként mutatja, hogy ki mit csinál kinek, egy kártyával ehhez a példányhoz, és egyel minden más taghoz, az Android-telefonokhoz és a ZFS-kiszolgálókhoz. A **Példány párosítása** lebeg jobb alul. Hogy mely részek vannak bekapcsolva, azt a **Beállítások, Példányok, Több szerver** alatt állítod: **Példányok**, **Fogadó** és **Lehívás**.

Minden kártya mutatja a tag védelmi állapotát és a szerepeit csempékként. A szerep olyasmi, amit egy példány egy másikért tesz:

- A **Fogadó** ennek a szervernek a telephelyen kívüli másolatait egy append-only módú rest-serverben tartja. Ez a szerver minden mentés után küld, és semmit nem tud ott törölni, akkor sem, ha valaki átveszi az irányítást fölötte.
- A **Lehívó** ennek a szervernek a mentéseit saját ütemezés szerint lehívja. Ehhez ennek a szervernek nem kell ébren lennie.
- A **ZFS-kiszolgáló** fogadja ennek a szervernek a ZFS-replikáit. Minden ZFS-elem a beállításaiban a **Replika** alatt választja ki.
- A **Kulcsőrző** ennek a szervernek a helyreállítási csomagjáról őriz egy lezárt másolatot. Lásd: [Kulcsőrző](#key-keeper).

Egy szerep bekapcsolása kérést küld, és semmi sem indul el, amíg valaki a másik példányon meg nem nyomja az **Engedélyezés** gombot. A kérés a példány kártyáján jelvényként jelenik meg, a **Részletek** pedig felsorolja az **Engedélyezés** és **Elutasítás** gombbal; egy engedélyezett szerep ott bármikor visszavonható. A hálóban szaggatott vonal a még várakozó szerep. A BombVault 9.x-es példány nem tud válaszolni egy kérésre, ezért ott a szerep abban a pillanatban aktív, ahogy bekapcsolod, a 9.x-es példánytól jövő kérés pedig itt vár az **Engedélyezés**re.

Egy kártya **Részletek** gombja megnyitja a többit: a tag fogadott tárolóit és lehívási forrásait, a célajánlatokat, a felügyeletet, az **Eltávolítás** és a **Kulcs elfelejtése** gombot.

## Fogadó és lehívó {#receiver}

![Egy fogadott telephelyen kívüli másolat egy példány részleteiben, csak olvashatóan figyelve, a fogadó hardveren futtatott integritásellenőrzéssel.](assets/screenshots/receiver.png)

*Egy fogadott telephelyen kívüli másolat egy példány részleteiben, csak olvashatóan figyelve, a fogadó hardveren futtatott integritásellenőrzéssel.*

Azon a gépen, amely egy másik BombVaulttól append-only másolatokat **fogad**, a fogadott tárolók független, csak olvasható felügyeletet adnak a fogadó hardveren, így a túlsó végen történő csendes hiba nem marad észrevétlen. Kapcsold be a **Fogadó** opciót a **Beállítások, Példányok** alatt, majd nyisd meg a küldő példány **Részletek** gombját a Példányok oldalon, és add hozzá a fogadott tárolót (csak olvasható, a küldő példány restic-jelszavával nyitva, amelyet a [párosítási csoporton](#pairing) át kap meg). Ezt kapod:

- **Forrásonkénti leltár**, hogy pontosan lásd, mely konténerek, VM-ek és mappakészletek érkeztek meg.
- **Utoljára fogadott** forrásonként, hogy tudd, mindegyik mennyire friss.
- **Független `restic check`** a fogadó hardveren, hogy az integritást ott ellenőrizzék, ahol az adatok ténylegesen vannak, ne csak a küldőn.
- **Holtember-kapcsoló:** riasztás, ha egy forrás az általad megadott időablakon belül nem küld.
- **Integritásriasztások:** riasztás, ha a fogadó oldali ellenőrzés meghiúsul.

A fogadó szigorúan csak olvas. Soha nem ír a fogadott tárolóba, így soha nem törheti meg az append-only garanciát, amelyre a küldő támaszkodik.

A lehívó fordítva dolgozik: bekapcsolt **Lehívással** egy példány saját ütemezés szerint beolvassa egy másik tag mentéseit a saját tárolójába. A forrásai szintén annak a tagnak a részleteiben állnak.

### Fogadószerver {#receiving-server}

A fogadó gép azt a rest-servert is futtathatja, amelyre a többiek másolnak. Az első olyan **Fogadó** kérés engedélyezésekor állítódik be, amely ezt kéri: az ablak megmondja, hogy a szerverhez fogadószerver kell, és kér egy mappát egy megosztáson (az **Új mappa** gombbal újat hozhatsz létre), valamint egy portot (8000, hacsak egy másik konténer nem használja). A BombVault ezután:

1. visszautasítja, ha már létezik `rest-server` nevű konténer, vagy egy másik konténer foglalja a portot;
2. letölti a `restic/rest-server` képet, és a Docker-socketen át append-only módban, privát tárolókkal és a mappában lévő bejelentkezési fájllal elindítja;
3. kiírja az Unraid-sablonját a flash-meghajtóra, hogy a konténer szerkeszthető maradjon a Docker lapon, vagy letöltésként ajánlja fel a sablont, ha a flash-meghajtó elérhetetlen;
4. lefuttatja ellene a manipulációs tesztet, és megmutatja, megtagadja-e a törléseket.

A csoportod példányai ezután a szolgáltatók között **A csoportodból** alatt találják meg a szervert, a fogadó gép nevén, és a **Fogadó** szerep ezt kínálja biztonságos választásként. Minden példány saját bejelentkezést kap, amikor először használja a szervert, és ott csak a saját mappájába ír. Ha a **Fogadó** be van kapcsolva a **Beállítások, Példányok** alatt, a **Fogadószerver** ennek a példánynak a kártyáján újra megnyitja a szervert. Az ablaka a **Bejelentkezéssel rendelkező partnerek** alatt felsorolja a bejelentkezéseket, a **Bejelentkezés visszavonása** pedig elvesz egyet; amit az a példány már átmásolt, a mappában marad. A beállítás létrehoz egy **Bejelentkezés a csoporton kívülre** is, amelynek jelszavát az ablak egyszer mutatja meg.

Az a példány, amely a fogadó gépet csak a relayn át éri el, nem használhatja a szervert, mert a relay nem szállít mentéseket. Előbb add hozzá a fogadó gép címét a párosítási oldalon. Ha a BombVault saját IP-címen fut (például br0-n), töltsd ki a **Cím a partnereknek** mezőt, mert a szerver a gazdagép címén hallgat.

## Végigvitt példa: két Unraid-gép, elejétől a végéig

A fentiek a részeket írják le. Ez egy teljes beállítás, mert a részeket könnyebb összerakni, ha egyszer láttad őket összerakva.

Két gép: a **TOWER** a konténereket futtatja és a mentéseket küldi; a **VAULT** fogadja őket, és érvényesíti az append-only módot. Helyettesítsd be a saját neveidet, címeidet és megosztási útvonalaidat.

**1. Párosítsd a két gépet.** A TOWER-en nyisd meg a **Beállítások, Párosítás** oldalt, nyomd meg a **Jelmondat létrehozása** gombot, és add meg a tizenkét szót a VAULT-on a **Jelmondat megadása** alatt. Előbb állíts be bejelentkezési jelszót mindkettőn.

**2. A TOWER-en kérd meg a VAULT-ot, hogy legyen a fogadó.** A Példányok oldalon kapcsold be a **Fogadó** szerepet a VAULT kártyáján, válaszd ki a szakaszokat, és helyként a **VAULT fogadószerverét**, majd küldd el a kérést.

**3. A VAULT-on engedélyezd.** A VAULT Példányok oldala mutatja a kérést a TOWER kártyáján. Nyomd meg az **Engedélyezés** gombot. Először a VAULT egy mappát kér a fogadószerverének, például `user/appdata/rest-server`, elindítja a rest-servert append-only módban, és lefuttatja ellene a manipulációs tesztet.

**4. Vissza a TOWER-en.** Ettől fogva a TOWER minden mentés után másol a VAULT-ra, és az új cél megjelenik a TOWER Célok oldalán a törlés elleni védelmével.

Mit válaszol a manipulációs teszt:

| Eredmény | Mi történt |
| --- | --- |
| **védett** | A VAULT megtagadta a törlést. Ez az egyetlen megfelelt állapot. |
| **nem védett** | A VAULT elfogadott egy törlést. Hiányzik a `--append-only`, vagy eltávolították. |
| **nem egyértelmű** | Egyik sem. Általában az URL nem az, amelyet maga a restic használ, vagy megváltoztak a hitelesítő adatok. Semmi sem kerül rögzítésre, és nem indul riasztás. |

**5. Figyeld, mi érkezik, ha akarod.** A VAULT-on kapcsold be a **Fogadó** opciót a **Beállítások, Példányok** alatt, nyisd meg a TOWER **Részletek** gombját, és add hozzá a fogadott tárolót. A VAULT a csoporton át kapja meg a TOWER restic-jelszavát; senki sem gépel be kulcsot.

**6. Tedd kölcsönössé.** Ismételd a lépéseket az ellenkező irányban: a VAULT megkéri a TOWER-t, hogy legyen a fogadója. Ekkor mindkét gép érvényesíti az append-only módot a másiknak, és egyik sem törölheti a másik mentéseit.

Egy kézzel futtatott rest-server is működik: add hozzá a **Cél hozzáadása** gombbal rest-serverként, a `.htpasswd`-jében lévő felhasználóval és jelszóval, és kapcsold be a törlés elleni védelmet. Tartsd meg az `--append-only` kapcsolót a beállításaiban; nélküle a túloldal megint egy átlagos megosztás.

!!! warning "A helyi útvonal a konténeren belüli útvonal"
    Ahol a BombVault egy mappát kér ezen a szerveren, a gazdagép csatolásához viszonyítva add meg, például `user/appdata/rest-server/bombvault-containers`, ne `/mnt/user/appdata/...` alakban. A BombVault konténerben fut, ahol a gazdagép `/mnt` könyvtára máshol van csatolva; egy abszolút gazdagép-útvonal ott nem létezik. Ha beillesztesz egyet, a BombVault megmondja, melyik relatív útvonalat használd helyette.

## Vészvarázsló {#emergency}

A **Vészvarázsló** visszahoz egy elveszett vagy újra felállított szervert. A **Visszaállítás** oldalon indul a **Vészhelyzet** alatt, és egy új telepítés üdvözlőoldalán a **Visszaállítás mentésből** gombról. Négy lépésben fut: **Hozzáférés**, **Beállítások**, **Kijelölés** és **Visszaállítás**.

1. **Hozzáférés.** Mutasd meg a BombVaultnak a régi mentések kulcsát, háromféleképpen:
    - **Helyreállítási kit feltöltése**: a fájl, amelyet a beállításkor letöltöttél. Tartalmazza a kulcsot és az összes célt, és a BombVault ebből tölti ki a többit. Az age-gel lezárt csomag elkéri azt az identitást, amelyre lezárták; egyszer használják, és nem tárolják.
    - **A csoporton át**: a csoportod egy példánya letétként őrzi a kulcsot. Kell hozzá a csoport tizenkét szava, a feloldókódod és valaki annál a példánynál. Lásd: [Kulcsőrző](#key-keeper).
    - **Kulcs beírása**: a régi szerver `APP_KEY` értéke, és hogy hol vannak a mentései.

    A BombVault ezután kipróbálja a kulcsot minden célon, és mindegyikről megmondja, hogy olvasható, nem található, másik kulccsal zárolt, nem elérhető, vagy még hiányzik hozzá a bejelentkezés. Még semmi sincs elmentve, és a kulcs korlátozott ideig a memóriában marad. A **Kulcs átvétele** azt éri el, hogy ez a szerver ezután ezzel a kulccsal írja a mentéseit; az **Átvétel visszavonása** visszatér a saját kulcshoz, amíg az első mentés le nem futott. Az a szerver, amelynek már vannak saját mentései, megtagadja az átvételt, és a [Régi mentési mappából](#restore-from-another-bombvault-repo) opcióhoz irányít. A **Hozzáférési adatok mentése** eltárolja azokat a bejelentkezéseket, amelyeket a csomag a távoli célokhoz hoz, és rákérdez, mielőtt felülírná azokat, amelyek már megvannak ezen a szerveren.
2. **Beállítások.** A **Beállítások visszahozása** a BombVault saját beállításait állítja vissza a legfrissebb Önmentésből: célokat, hitelesítő adatokat, ütemezéseket és szabályokat. A BombVault újraindul, hogy alkalmazza őket, a Docker-socketen át, így az éles beállítási adatbázist soha nem írják felül nyitott kezelő alatt, és a varázsló ezután magától folytatja. Ennek a szervernek a bejelentkezése megmarad: a mostani jelszavaddal jelentkezel be tovább. A régi szerveren tárolt, át nem vehető titkok fel vannak sorolva, hogy újra megadhasd őket.
3. **Kijelölés.** A BombVault végignézi a mentéseidet, és felsorolja a talált konténereket, VM-eket, mappakészleteket és ZFS-adatkészleteket. A legfrissebb mentés mindenhol előre ki van jelölve. Ami ezen a szerveren még megvan, kikapcsolva indul, mert a visszaállítás lecserélné. A mappakészletek az újraépítés után már nem tudják, honnan jöttek, ezért egy általad választott mappába kerülnek.
4. **Visszaállítás.** Minden kijelölt egyszerre visszajön, megállítva hagyva, ha minden konténert és VM-et magad akarsz elindítani, ami teljes elvesztés után a legbiztonságosabb. Utána tölts le egy új helyreállítási csomagot: a régi az előző telepítéshez tartozik.

!!! tip "Tervezett migráció és katasztrófa"
    A Vészvarázsló a BombVault saját beállításait állítja vissza egy mentésből. Egy *tervezett* költözéshez egy új gépre ehelyett közvetlenül is átviheted a konfigurációdat az **Beállítások exportja / importja** kártyával (egy hordozható JSON-fájl). Lásd: [Konfiguráció](configuration.md#portable-settings-export-and-import).

### Régi mentési mappából {#restore-from-another-bombvault-repo}

A Visszaállítás oldalon a **Más források** alatti **Régi mentési mappából** egy *másik* BombVault-példány tárolóját (egy `/mnt` alatt csatolt megosztást vagy távoli címet) nyitja meg annak a példánynak a helyreállítási csomagjával vagy a beírt `APP_KEY` értékkel, egyszeri, csak olvasható munkamenetben. Böngészd az ott tárolt konténereket, VM-eket, mappakészleteket és ZFS-adatkészleteket, válassz egy mentést, és állítsd vissza, a visszaállított objektum pedig normál helyi konténerré, VM-mé vagy mappakészlette válik. A másik tárolóba soha nem írnak, és a saját mentési beállításaid érintetlenek maradnak (a munkamenet a memóriában él, és magától lejár). Egy konténer áthelyezése az A szerverről a B szerverre nem jelenti, hogy át kell irányítanod a beállításaidat, majd később vissza kell állítanod őket. Ez az ablak egyszeri: megnyit egy munkamenetet, visszaállítja, amit kiválasztasz, és elfelejti a másik példányt. Ha ehelyett tartós megoldást akarsz, ahol ez a gép ütemezve a saját tárolójába tölti le egy másik példány mentéseit, az a Példányok oldal **Lehívó** szerepe.

Az a konténer, amelynek a hálózata nem létezik ezen a szerveren, például egy Unraid `br0` hálózat egy sima Docker-gazdán, hálózatválasztót mutat a sora alatt. A BombVault a választott hálózaton hozza létre, a többi hálózatával együtt. A rögzített IP és MAC-cím a régi hálózathoz tartozott, ezért elhagyják, és az új hálózat osztja ki őket.

## Kulcsőrző {#key-keeper}

A kulcsőrző a csoportod egy példánya, amely ennek a szervernek a helyreállítási csomagjáról egy lezárt másolatot tart. A másolatot maga nem tudja megnyitni. Ha ez a szerver valaha elvész, egy új telepítés, amely csatlakozik a csoportodhoz, elkéri a másolatot az őrzőtől, és a feloldókóddal nyitja meg.

1. A Példányok oldalon kapcsold be a **Kulcsőrző** szerepet annak a példánynak a kártyáján, amelyik őrizze, és válaszd a **Letét létrehozása** gombot. Letét létrehozásához bejelentkezési jelszó kell.
2. Ez a szerver egyszer mutatja a **Feloldókódot**: húsz betű és számjegy négy ötös csoportban. Írd le, és erősítsd meg a **Felírtam a kódot** gombbal. Ez a szerver nem tárolja a kódot, csak azt, ami egy hozzá tartozó letét lezárásához kell.
3. Az őrző mostantól tartja a letétet, és ez a szerver megújítja, amikor a csomag változik. Ennek a példánynak a kártyáján a **Kulcsletét** felsorolja **Ennek a szervernek a letétje** alatt az egyes őrzőket és az állapotukat, a **Letétek, amelyeket ez a szerver őriz** alatt pedig a többiekét. Az **Új kód készítése** kicseréli a kódot; minden őrző átáll az újra, amikor megkapja az új letétet.

Azon a napon három dolog kell: a csoportod tizenkét szava, a feloldókód, és hozzáférés az őrző felületéhez, ahol valakinek meg kell nyomnia az **Engedélyezés** gombot. A Vészvarázslóban válaszd **A csoporton át** opciót, válaszd ki a letétet, kérd el, és add meg a feloldókódot. A kódot az új szerveren használják, az őrzőnek nem küldik el. Az őrzőn a kérés a **Kulcsletét** alatt jelenik meg: ki kéri, látták-e valaha ott azt a példányt, és hányszor kért; az **Engedélyezés** kiadja a lezárt másolatot, az engedélyezéshez pedig bejelentkezési jelszó kell. Az új telepítésnek az Engedélyezés után 30 perce van átvenni, a kérés, amelyre senki sem válaszol, egy nap után lejár, és az a példány, amely túl gyakran kér, egy órát vár.

Kód nélkül senki nem tudja megnyitni a másolatot, az őrző sem. Ha a kód elveszett, készíts újat, amíg ez a szerver még fut; ha a szerver már eltűnt, a helyreállítási csomag és a beírt APP_KEY továbbra is működik. Az őrző bármikor törölhet egy letétet, és a tulajdonosnak ezután újra kérnie kell.

## Helyreállítási csomag {#recovery-kit}

Ez az a darab, amely akkor is lehetővé teszi a katasztrófa utáni helyreállítást, ha nincs futó BombVault.

A **Beállítások, Visszaállítás** alatti **Vészhelyzetre** kártyán a **Helyreállítási csomag letöltése** megadja a **kulcsot**, a **származtatott restic-jelszót**, minden **célt** az egyes szakaszok tárolójával, azokat a bejelentkezéseket, amelyek egy új telepítésnek kellenek az eléréshez, és a **pontos restic-parancsokat**, így közvetlenül a restic CLI-vel visszaállíthatsz bármely gépen. Bejelentkezési jelszó kell hozzá. A csomag a kulcsot kikapcsolt titkosításnál is kiírja, mert a konténer- és VM-definíciókat mindig ezzel zárják le. Átvétel után kiírja az átvett mentési kulcsot és ennek a szervernek az APP_KEY-ét, amelyre egy új telepítésnek csak a beállítások visszahozásához van szüksége. Az Áttekintés **Mentsd el a helyreállítási csomagot** kártyája emlékeztet, amíg el nem tároltad.

!!! danger "A helyreállítási csomagot a szerveren kívül tárold"
    A csomag tartalmazza azt a titkot, amely visszafejti a mentéseidet. Tartsd biztos helyen, a szervertől elkülönítve (jelszókezelő, kinyomtatott példány egy széfben). Ha a BombVaultot és az `APP_KEY` értékét is elveszíted helyreállítási csomag és kulcsletét nélkül, a titkosított mentéseidet nem lehet visszaszerezni.

!!! warning "A legújabb mentés nem mindig az, amit vissza kell állítani"
    A restic 0.17 óta a `restic snapshots` mutatja minden pillanatkép méretét. Adatvesztés után a legújabb mentés lehet a kiürített, ezért ne állíts vissza olyan mentést, amely jóval kisebb a korábbiaknál. Zsarolóvírus után lehet a titkosított, a szokásos méretben. Ha a BombVault még fut, előbb nézd meg az **Anomáliák** oldalát: megnevezi az utolsó jó mentést, és a visszaállítási ablak ezt jelöli ki előre. A visszaállításhoz nincs szükség a BombVault anomáliaadataira, és a megőrzési szünet mindig csak több mentést tart meg.

### A csomag lezárása

Ha bekapcsoltad az **Exportok titkosítása age-dzsel** opciót a **Beállítások, Rendszer** alatt, a csomagot is ezzel zárják le, és `bombvault-recovery-kit.md.age` néven töltődik le. ASCII-páncélozott, nem bináris, tehát továbbra is egyszerű szöveg: jelszókezelőbe illesztése vagy kinyomtatása pontosan úgy működik, mint korábban, csak a tartalom olvashatatlan a kulcsod nélkül.

!!! warning "Ne tárold az age-kulcsot a csomagban"
    Egy lezárt csomag megnyitásához szükséged van az age **privát** kulcsodra. Olyan helyen tartsd, amely nem magától a csomagtól függ, különben egy helyett két dolgot kell majd helyreállítanod. A lezárás akkor éri meg, ha a csomag olyan helyen van, amely felett nincs teljes ellenőrzésed (megosztott jelszókezelő, felhős jegyzetek, egy kinyomtatott példány egy irodában); a saját széfedben lévő csomagot már a széf védi.

    Bekapcsolt titkosítás és beállított, használható címzett nélkül a letöltést a BombVault eleve elutasítja. Soha nem tér át arra, hogy a mesterkulcsot nyílt szövegként adja ki.

### Ha a csomag épp nincs kéznél

A jelszót semmi sem tárolja, a mentések kulcsából **számítják ki**, így csak a kulccsal és egy shell-lel te magad is előállíthatod:

```sh
printf 'bombvault:restic-repo' \
  | openssl dgst -sha256 -mac HMAC -macopt hexkey:$APP_KEY -r \
  | cut -d' ' -f1
```

Ez egy HMAC-SHA256 a rögzített `bombvault:restic-repo` karakterláncon, kulcsként a hexadecimális `APP_KEY` nyers bájtjaival, 64 kisbetűs hexadecimális karakterként kiírva. Ugyanez az érték szerepel a csomagban származtatott restic-jelszóként; ez arra a napra való, amikor a csomag ott van, ahol te nem. Azon a szerveren, amely átvette egy másik mentéseit, használd az átvett mentési kulcsot, vagyis annak a szervernek a régi APP_KEY-ét.

!!! warning "Fogadott tárolónál a KÜLDŐ példány kulcsát használd"
    Az a tároló, amely telephelyen kívüli másolattal érkezett ide, a küldő gépen jött létre, annak **saját** kulcsával. A fogadó gép kulcsából származtatva olyan jelszó adódik, amelyet a restic elutasít, és ez pontosan úgy néz ki, mint egy sérült tároló, de nem az. Ez a szokásos oka annak, hogy a `restic check` egy fogadott tárolón újra meg újra jelszót kér.

Mivel a helyreállítási definíciók minden tárolón **belül** élnek (`<repo>/def`, `<repo>/vm-def`), egy másolt tárolómappa teljesen önálló, így a csomag plusz a tároló minden, amire egy bare-metal visszaállításnak szüksége van.

## Adatbázis-mentés visszaszerzése {#database-dumps}

Az adatbázis-dump a konténerek tárolójában külön visszaállítási pont `dbdump:<container>` címkével, amely az egyetlen `/dbdump/<container>.sql` fájlt tartalmazza. A BombVault a konténer **Biztonsági mentések** lapján listázza, tölti le és importálja őket; lent ugyanezek a lépések csak a resticcel, arra a napra, amikor a BombVault nincs ott.

```sh
restic -r <repo> snapshots --tag dbdump:<container>
restic -r <repo> dump --tag dbdump:<container> latest /dbdump/<container>.sql > <container>.sql
```

Az egyes dumpokon a `dbversion:` és `dbname:` címke megmondja, melyik kiszolgálóverzióból való és mely adatbázisokat tartalmazza. A teljes fájl vége `-- PostgreSQL database cluster dump complete` vagy `-- Dump completed`.

Importáld egy azonos vagy újabb verziójú (PostgreSQL), illetve azonos főverziójú (MySQL és MariaDB) konténerbe, amelyet egyszer üres adatmappával indítottál, hogy feltöltse magát. A gazdagépnek nem kell adatbázis-kliens, a konténerben van:

```sh
docker exec -i <container> sh -c 'exec psql -X -U "${POSTGRES_USER:-postgres}" -d postgres' < <container>.sql
docker exec -i <container> sh -c 'exec mariadb -uroot -p"$MARIADB_ROOT_PASSWORD"' < <container>.sql
docker exec -i <container> sh -c 'exec mysql -uroot -p"$MYSQL_ROOT_PASSWORD"' < <container>.sql
```

Ha egy teljes dumpból csak egy adatbázis kell, a MySQL és a MariaDB elfogadja a `--one-database <name>` kapcsolót a kliens parancsán. A PostgreSQL-dumpban adatbázisonként egy szakasz van, mindegyik egy `\connect <name>` sorral kezdődik: másold a szakaszt külön fájlba, és az adatbázis létrehozása után `-d <name>` kapcsolóval importáld.

!!! warning "A rootként készült dump magával viszi a kiszolgáló felhasználóit"
    A rootként készült teljes MySQL- vagy MariaDB-dump tartalmazza a `mysql` rendszeradatbázist, így az importálás az új kiszolgáló fiókjait, a root jelszavát is beleértve, a dumpban lévőkre cseréli. PostgreSQL-en a konténer által létrehozott felhasználóra kapott `role ... already exists` üzenet várható és ártalmatlan.
