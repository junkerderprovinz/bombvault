# Telephelyen kívüli mentés és helyreállítás

!!! note "A telephelyen kívüli másolatok várnak egy újraépítés után"
    Amikor a Vészvarázsló vagy a **Biztonsági mentések felfedezése** a régi beállítások nélkül építi újra az elemeket, ezeknek a szakaszoknak a telephelyen kívüli másolatai várnak, amíg át nem veszed a mentéseiket a szerver saját mentéseiként. Lásd: [Újraépítés után](#after-a-rebuild).

A helyi mentések megvédenek egy elveszett konténertől vagy egy rossz frissítéstől. A telephelyen kívüli másolatok és egy tesztelt helyreállítási csomag megvéd a teljes gép elvesztésétől, a zsarolóvírustól vagy egy tűztől. Ez az oldal a mentési célokat és a telephelyen kívüli másolatokat, a másolat manipulációbiztossá tételét, a visszaállíthatóság bizonyítását, több szerver párosítását, valamint azt ismerteti, hogyan kapsz vissza mindent, ha maga a BombVault eltűnt.

## Telephelyen kívüli másolatok {#off-site-copies}

Tartsd meg a gyors helyi mentést, és adj hozzá egy vagy több mentési célt a telephelyen kívül. A BombVault az új mentéseket `restic copy` segítségével, legjobb szándék szerint másolja oda, így egy telephelyen kívüli zökkenő soha nem hibáztatja el a helyi mentést. Ebben a felállásban minden szakasz először a saját mappájába ír, a telephelyen kívüli tároló pedig egy másolat. A saját mappának nem kell ezen a szerveren lennie: lásd [Egy szakasz saját mappája](#own-folder), hogyan tarthatod egy mentési célon, például S3-on vagy egy rest-serveren.

- **Több mentési cél szakaszonként.** Minden szakasz (konténerek, VM-ek, flash, Önmentés, mappák és ZFS-adatkészletek) egyszerre több telephelyen kívüli mentési célra is másolhat, így párhuzamosan tarthatsz például egy rest-servert egy barátod gépén és egy S3-bucketet is. A Célok oldal **Célok szakaszonként** kártyája kapcsolja be vagy ki a szakaszból a célba vezető vonalakat.
- **Mikor futnak a másolatok.** Egy cél **Másolás** kártyáján a **Mikor másoljon** adja meg az időpontot: **Minden mentés után**, vagy egy saját terv, amely összegyűjti a mentéseket, és rögzített időpontokban másol. A **Beállítások, Ütemezés, Telephelyen kívüli másolatok szakaszonként** alatt a **Kivétel** egy szakasznak saját másolási tervet ad.
- **A Telephelyen kívüli másolatok kártya.** A **Beállítások, Célok** alatt minden szakasznak, amelynek van telephelyen kívüli másolata, saját sora van a **Telephelyen kívüli másolatok** között, a **Kapcsolat tesztelése**, a **Replikálás most** (igény szerint elindít egy másolást) és a **Beállítás** vagy **Szerkesztés** gombokkal. A szakasz többi másolási beállítása innen nyílik meg a saját ablakában.
- **A megőrzést** célonként a **Meddig őrizze meg** alatt állítod be, így a telephelyen kívüli másolatokat archívumként hosszabban is megtarthatod. A szakasz telephelyen kívüli mezőjében lévő másolat ugyanúgy követi, mint minden más másolat, és egy szakasz a **Szabályok szakaszonként** alatt megtarthatja a saját értékeit. A csupa nulla soha nem vágja vissza a telephelyen kívüli mentéseket. Lásd: [Meddig őrizze meg](#keep).
- **Sávszélesség-korlátok** (**Telephelyen kívüli sávszélesség** a **Beállítások, Célok** alatt, vagy egy cél **Másolás** kártyája) korlátozzák a restic feltöltési/letöltési sebességét, hogy a másolás ne telítse a WAN-odat. KiB/s-ben adod meg őket, abban az egységben, amelyet a restic vár, és minden mező gépelés közben megmutatja ugyanazt a sebességet MB/s-ben és Mbit/s-ben is: a 2048 nagyjából 2,0 MB/s (16,8 Mbit/s).
- **A tömörítés** lehet **Ki**, **Automatikus** (a restic alapértelmezése) vagy **Maximális**, minden mentési célon beállítva, egy szakasznál pedig a szabályai alatt. A mentések, a másolatok és a metszés ezzel írnak, a helyreállítási csomag pedig megnevezi, így a sima restic is ugyanúgy írhat tovább.
- **Elsőbbség a streamelésnek.** Amíg egy médiaszerver, például a Plex, a Jellyfin vagy az Emby streamel, a másolatok alacsonyabb korláttal töltenek fel, és a stream vége után néhány perccel visszatérnek a megszokotthoz. A Célok oldal **Streamelés** kártyáján ott a **Telephelyen kívüli másolatok lassítása streameléskor**, a **Médiaszerverek** (amíg nem választasz, azok a konténerek, amelyek image-nevében Plex, Jellyfin vagy Emby szerepel), a **Stream e fölött (Mbit/s)**, a **Feltöltési korlát streameléskor (KiB/s)** és a **Vissza a normálba ennyi után (perc)**. A BombVault a forgalmat a Dockerből olvassa, ezért a gazdagép hálózatán futó médiaszervert nem lehet megmérni. A REST, az S3, a B2, az Azure, a Google Cloud, a Swift és a HTTP-n át futó rclone a másolás közben lassul le. Az SFTP, valamint a helyi vagy csatolt mappák a következő másolási lépésüknél veszik át az alacsonyabb korlátot.
- **Hideg tárolás (S3).** Egy S3-cél olcsóbb **Tárolási osztályt** is használhat, amelyet a restic még azonnal el tud olvasni, például Standard-IA, One Zone-IA, Intelligent-Tiering vagy Glacier Instant Retrieval. A Glacier Flexible Retrieval és a Deep Archive kimarad, mert a restic nem tud belőlük visszaállítani. Egy rclone-remote az osztályát az rclone-konfigurációban állítja be.
- **Növekedési keret.** A **Növekedési keret (GB, 0 = ki)** egy szakasz másolási ablakában riasztást indít, ha a tároló ennél a méretnél nagyobbra nő. Soha nem töröl semmit, ezért egy olyan append-only másolathoz illik, amelyet innen semmi nem metsz.
- Amíg egy másolás fut, az Áttekintés állapotfejléce ezt írja: **Telephelyen kívüli másolás folyamatban**. Ez aktív jelzés, nem százalékos folyamatjelző, mert a `restic copy` nem ad gépileg olvasható előrehaladást.

!!! note "Visszaállítás bármelyik helyről"
    Minden elem a **Biztonsági mentések** lapján egyetlen idővonalként sorolja fel a mentéseit mindazokon a helyeken át, ahol egy mentés megtalálható. A B2-re másolt mentés egyszer jelenik meg, megjelölve minden hellyel, amely tartalmazza. A visszaállítás az első elérhető helyet használja, azzal a tárolóval kezdve, amelyre az elem íródik, és mentésenként választhatsz másikat. A telephelyen kívüli mentési célokat csak akkor olvassa a BombVault, ha az **Ellenőrzés** gombra kattintasz, mert a felhőtár listázása időbe telik. Ha egy helyen törölsz, a BombVault előbb megnézi a többit, és megmondja, hogy ez volt-e az utolsó másolat. Lásd: [Visszaállítás](restore.md).

## Mentési célok {#destinations}

A mentési cél egy hely, ahová a mentések kerülnek: egy helyi mappa, egy megosztás, egy saját szerver vagy egy felhőszolgáltatás. Egyszer kell beállítani, utána minden szakaszt kiszolgál. A **Célok** oldalt a beállítások csempéjéről nyithatod meg, vagy kapcsold be ott a **Mutatás az oldalsávban is** opciót, hogy kitűzd.

![A Célok oldal: a célok listája és az ábra arról, melyik szakasz hová megy.](assets/screenshots/storage/locations-dark.png#only-dark)
![A Célok oldal: a célok listája és az ábra arról, melyik szakasz hová megy.](assets/screenshots/storage/locations-light.png#only-light)

A **Cél hozzáadása** négylépéses ablakot nyit:

1. **Szolgáltató.** Minden szolgáltatás a logójával szerepel, az ablak jobb felső sarkában kereséssel, csoportokban: a **Helyi** egy ehhez a szerverhez csatolt lemezhez vagy megosztáshoz, a **NAS vagy szerver** (rest-server, Hetzner Storage Box, SFTP, SMB, WebDAV és mások), az **S3-szolgáltatás** (Backblaze B2, Wasabi, Cloudflare R2, Hetzner Object Storage, Amazon S3 és mások), a saját S3-kiszolgálód (Garage, SeaweedFS, RustFS, Silo, Ceph, JuiceFS, Versity S3 Gateway), a **Felhőtárhely** (OneDrive, Google Drive, Dropbox, pCloud, Nextcloud és minden más, amit az rclone támogat), valamint **A csoportodból**, vagyis a csoportod fogadószerverei. Mindegyik megmondja, mennyire illik mentéshez: a felhőmeghajtók sok kérés alatt lelassulnak, ezért az első mentés és a metszés ott tovább tart. Egy olyan szolgáltatáshoz, amelyet egyetlen csempe sem fed le, a **Saját rclone-konfiguráció** alatti **Rclone** csempe egy rclone.conf-ból bemásolt remote-okat fogad.
2. **Bejelentkezés.** A mezők a szolgáltatástól függnek: hozzáférési kulcs S3-hoz, felhasználó és jelszó WebDAV-hoz és SMB-hez, alkalmazásjelszó ott, ahol a kétlépcsős bejelentkezés blokkolja a megszokottat, a BombVault nyilvános SSH-kulcsa SFTP-hez és a Storage Boxhoz, vagy token a böngészőn át bejelentkező szolgáltatásokhoz. Ezekhez az ablak egy `rclone authorize` parancsot mutat, amelyet egy böngészős számítógépen kell lefuttatni, és a kiírt token a mezőbe kerül. Ahol a szolgáltatáshoz már van [mentett bejelentkezésed](#sign-ins), az **Új kulcsok** helyett a **Mentett bejelentkezés** azt választja ki. A **Kapcsolat tesztelése** a mentés előtt ellenőrzi a bejelentkezést. Az ablak ezután listázza a mentési cél mappáit, az **Új mappa** gombbal újat hozhatsz létre, és kiírja a szabad helyet, ahol a szolgáltatás jelenti. Egy üres mappa a legbiztonságosabb. A BombVault szakaszonként egy mappát hoz létre benne.
3. **Védelem.** Az ablak nyíltan megmondja, mit tud a szolgáltatás. Az append-only módban futó rest-server megtagadja a törlést, és a manipulációs teszt ezt ellenőrzi. Egy S3-bucket meg tudja őrizni a régi verziókat verziókezeléssel és object lockkal, ezt a BombVault még nem tudja ellenőrizni. A felhőmeghajtó egyáltalán nem tudja megtagadni a törlést: aki bejut a szerverre, bejut abba a másolatba is. A **Törlés elleni védelem (append-only)** kapcsolót csak ott kapcsold be, ahol a túloldal valóban megtagadja a törlést. A BombVault ott soha nem metsz. A **Vészhelyzetre** megmondja, mit tartalmaz a helyreállítási csomag ehhez a mentési célhoz.
4. **Mely szakaszokhoz.** Válaszd ki a szakaszokat, amelyek ide mentenek. Az első másolat az egész előzményt magával viszi. Azok az elemek, amelyek saját mentési célokat választottak, nem kapják meg az újat maguktól, és az ablak név szerint felsorolja őket.

![Cél hozzáadása: az első lépés, minden szolgáltató egy-egy csempeként.](assets/screenshots/storage/add-providers-dark.png#only-dark)
![Cél hozzáadása: az első lépés, minden szolgáltató egy-egy csempeként.](assets/screenshots/storage/add-providers-light.png#only-light)

![Cél hozzáadása: a bejelentkezési lépés egy sikeres teszt után.](assets/screenshots/storage/add-signin-dark.png#only-dark)
![Cél hozzáadása: a bejelentkezési lépés egy sikeres teszt után.](assets/screenshots/storage/add-signin-light.png#only-light)

Egyes szolgáltatások a bejelentkezésnél kicsit többet kérnek:

- **SFTP és a Storage Box.** A **Kulcs hozzáadása** felteszi a BombVault nyilvános kulcsát a szerverre az **A fiók jelszava** mezőben megadott jelszóval, amelyet csak ehhez az egy bejelentkezéshez használ, és sehol nem tárol. Alatta látható a parancs, amellyel kézzel is elvégezheted. A teszt megmutatja a **Szerverkulcs** értékét az ujjlenyomatával. A BombVault megjegyzi, és elutasítja a szervert, ha később másikat mutat.
- **Egyetlen bucketre szóló S3-kulcsok**, például egy Cloudflare R2 objektumtoken vagy egy szűkített AWS-kulcs, nem feltétlenül listázhatják a többi bucketet. Írd be a bucketet a **Bucket** mezőbe, és a teszt meg a mappalista ezt a bucketet nyitja meg.
- **S3-régiók és címek.** A régiót a BombVault abból a végpontból olvassa ki, amely megnevezi (AWS, Backblaze B2, Hetzner, Scaleway, OVH, Wasabi), a Garage a saját alapértelmezett régiójával indul, az IDrive e2 pedig azt a végpontot kéri, amelyet az irányítópultja mutat. A bucketet is tartalmazó beillesztett cím, ahogy az R2 irányítópultja mutatja, kitölti a szervert és a bucketet. A mappaböngészőből létrehozott bucket abba a régióba kerül, amelyet az űrlap megnevez.
- **Tárolási osztály.** A választó csak azokat az osztályokat kínálja, amelyeket a szolgáltatás elfogad, a **További beállítások** pedig csak azokat az opciókat mutatja, amelyek egy új bucketet alakítanak.
- **Google Drive és OneDrive.** A metszett adatok véglegesen törlődnek ahelyett, hogy a kukába vagy a lomtárba kerülnének, ahol még beszámítanának a kvótába. A Google megosztott meghajtója a **További beállítások** alá kerül.
- **Azok az SMB-szerverek, amelyek nem listázzák a megosztásaikat,** a **Megosztás** mezőben kapják meg az egyik nevét, és a teszt közvetlenül azt a megosztást nyitja meg.
- **Az FTP** a **Titkosítás** módját kérdezi: **Nincs**, **Explicit TLS (FTPES)** vagy **Implicit TLS (FTPS, 990-es port)**.
- **OneDrive és Zoho WorkDrive.** Ha egy bejelentkezés több meghajtót vagy munkaterületet nyit meg, az ablak megkérdezi, melyikre menjenek a mentések.
- **Önaláírt tanúsítványú rest-server.** Tedd a tanúsítványt PEM-fájlként a BombVault adatmappájának `certs` mappájába (`/config/certs` a konténerben, `appdata/bombvault/certs` Unraidben). A mentések, a másolatok és a törlés elleni védelem tesztje újraindítás nélkül megbíznak benne.

Az S3-szolgáltatások a restic saját S3-háttérrendszerén futnak, ez teszi lehetővé a tárolási osztály és az object lock alkalmazását. A rest-servert a restic közvetlenül éri el. Minden más szolgáltatás a BombVault mellé csomagolt rclone-on fut, és az rclone-konfigurációjának a szakasza a cél oldalán az **rclone.conf** kártyán látszik, a titkok elrejtve. A beállítások exportja tartalmazza a mentési célokat, a hitelesítő adatokkal együtt pedig a bejelentkezésüket is.

Ha elmentesz egy olyan mentési célt, amelyen már vannak konténerek, VM-ek vagy mappák mentései, például egy eltávolított és újra felvett célt, azokhoz a szakaszokhoz kipipálatlan másolatként adja hozzá. Semmi sem másolódik oda, amíg ki nem pipálod, de a **Visszaállítás** a régi mentéseket azonnal felkínálja.

A mappát ezen a szerveren mindenhol **Helyi** néven hívják: a tárolási oldalakon, az ábrán, egy elemen és a Visszaállításnál. Ha ezen a szerveren több mappa van, mindegyik a saját nevén szerepel. Az a mentési cél, amelyet egy szakasz telephelyen kívüli mezője hoz létre, a helyéről kapja a nevét, vagyis a címe utolsó mappájáról, ennek híján a szerveréről, és számot kap, ha már van ilyen nevű cél. Minden cél átnevezhető a **Kapcsolat** kártyájának **Név** mezőjében, ez igaz egy bucketmappára is, amelyet több szakasz oszt meg, és arra a másolatra is, amelyet egy szakasz telephelyen kívüli mezője ír le, és a megadott név megmarad. Ha két azonos nevű cél áll egymás mellett, mindegyik mellé kerül a szervere vagy a mappája.

### A mentési cél oldala {#location-page}

Minden mentési célnak saját oldala van. Csak azokat a kártyákat mutatja, amelyek arra a célra érvényesek, ebben a sorrendben:

![Egy mentési cél oldala: Saját mappák, Kapcsolat és Hitelesítő adatok.](assets/screenshots/storage/location-page-dark.png#only-dark)
![Egy mentési cél oldala: Saját mappák, Kapcsolat és Hitelesítő adatok.](assets/screenshots/storage/location-page-light.png#only-light)

- **Saját mappák**: azok a szakaszok, amelyek a mentéseiket először ide írják. Ennek a célnak a bejelentkezése, védelme és korlátai rájuk érvényesek. A **Használat saját mappaként** több szakasz saját mappáját viszi ide, a **Módosítás** pedig egyet újra elvisz innen. Lásd: [Egy szakasz saját mappája](#own-folder).
- **Ide ment közvetlenül**: minden szakasz, amelynek elemei a saját mappájuk helyett közvetlenül erre a helyre mentenek, és hogy hány elem teszi ezt. Egy ilyen közvetlen tároló minden beállítását ettől a céltól veszi át. Amelyikbe már semmi nem ment, az ezt jelzi, és az **Eltávolítás** leveszi a listáról. A helyén lévő mentések megmaradnak. Lásd: [Közvetlen tárolók](#direct-repositories).
- **Kapcsolat**: a **Név**, az **Elérhető** kapcsoló (kikapcsolva a BombVault már nem ír ide, és az ide mentő elemek érthető üzenettel hibáznak ahelyett, hogy máshová kerülnének), a mappa, a **Tömörítés**, a **Tárolási osztály** és a **Telephelyen kívül**. A **Helyi** célnál a **Saját mappák itt** alatt felsorolja minden szakasz mappáját. A **Kapcsolat módosítása** teszteli és menti az új címet vagy bejelentkezést. Ha más helyek ugyanazzal a mentett bejelentkezéssel jelentkeznek be, az ablak ezt jelzi, és az itteni módosítás ennek a célnak saját bejelentkezést ad ahelyett, hogy átírná a megosztottat.
- **Hitelesítő adatok**: melyik mentett bejelentkezést használja (**Bejelentkezik ezzel**), mely más célok használják még (**Ezek is használják**), és a **Szerkesztés**, amely megnyitja a bejelentkezési ablakot. Lásd: [Bejelentkezések](#sign-ins).
- **rclone.conf**: ennek a célnak az rclone-konfigurációs szakasza, az rclone-on át futó célnál.
- **Törlés elleni védelem**, a **Törlés elleni védelem (append-only)**, az **Elfogadás törlés elleni védelem nélkül**, a **Törlés elleni védelem tesztelése most** és a **Ki nyes** elemekkel, amely megmondja, hogy a BombVault metszi-e itt a másolatot és a saját mappákat, e cél **Meddig őrizze meg** kártyája vagy egy szakasz saját értékei szerint a **Szabályok szakaszonként** alatt, vagy ezt a túloldalra hagyja. Egy helyi mappa csak akkor mutatja ezt a kártyát, ha van kikapcsolható védelme. Lásd: [Törlés elleni védelem](#delete-protection-append-only).
- **Másolás**: a **Mikor másoljon**, valamint a **Feltöltési korlát (KiB/s)** és a **Letöltési korlát (KiB/s)**. Az a cél, amely csak saját mappákat tart, másolatokat nem, a korlátokat **Sávszélesség** nevű kártyán mutatja.
- **Meddig őrizze meg**: **Rövid**, **Kiegyensúlyozott**, **Hosszú** vagy **Saját értékek**, idővonallal arról, hogy a szabály mely mentéseket tartja meg. Lásd: [Meddig őrizze meg](#keep).
- **Már ennek a célnak az alatt van**: szakaszok, amelyek kézzel beírt címe ennek a célnak egy mappájában van. Az átvétel a tárolót ott hagyja, ahol van. A bejelentkezés, a tárolási osztály és a törlés elleni védelem ezután a céltól jön. A BombVault előbb ellenőrzi, hogy a cél bejelentkezése megnyitja-e a tárolót, és megtagadja, hogy egy append-only tárolót olyan cél alá tegyen, amely nem append-only.
- **Szabályok szakaszonként**: egy szakasz saját tárolója, megőrzése vagy tömörítése ennél a célnál. A **Szabályok módosítása** megnyitja őket, a **Vissza a cél értékeihez** pedig visszaviszi a szakaszt. A bejelentkezés, a sávszélesség és a másolatok időzítése mindig a céltól jön.
- Egy edény, amely mutatja, mennyire tele van a cél, a jelöléssel, hogy hol telik meg, és hogy az utolsó hetek növekedése mellett hány hét van hátra. Az a cél, amely nem jelenti a méretét, ezt írja: **A méret ismeretlen**.

A **Költöztetés** és a **Cél eltávolítása** az oldal műveletei között áll, leírásukat lásd a [Költöztetés és eltávolítás](#move-remove) alatt.

Az a távoli mentési cél, amelyen mentések voltak, és hirtelen nincs rajta tároló, például egy rest-server, amelynek a lemeze nem csatolódott fel, nem kap magától új, üres tárolót. Az oldala ezt jelzi, amint egy mentés vagy másolás észreveszi, hogy a tároló eltűnt. Csatold fel újra a lemezt. Ha szándékosan ürítetted ki a régi tárolót, az **Új tároló létrehozása itt** elölről kezdi, a cél összes szakaszára és tárolóira egyszerre. Ez a védelem olyan célnak szól, amely a helyén maradt. Egy újonnan hozzáadott cél, vagy egy szakasz, amelyet bejelölsz rajta, tisztán indul: ha azon a címen nincs tároló, az első másolás létrehozza, akkor is, ha egy korábban eltávolított célnak ott volt egy.

Két elutasítás megnevezi, mit kell rendbe tenni, a másolás futásában és a cél oldalán. Az **Ez a bucket nem létezik a szerveren** üzenet azonnal megjelenik, amint egy S3-szerver ezt mondja, és a BombVault ott semmit sem hoz létre, így egy elgépelt bucketnév látható marad. Az a szerver, amely egyáltalán nem válaszol, más eset: a restic hívásonként nagyjából 15 percig újrapróbálkozik, hátha visszatér. Az **Itt már van egy restic-tároló, és ennek a példánynak a kulcsa nem nyitja meg** azt jelenti, hogy az ottani mentések egy másik `APP_KEY`-hez tartoznak.

A csoportod egy másik példánya által üzemeltetett fogadószerver a szolgáltatók között **A csoportodból** néven jelenik meg. Lásd: [Fogadószerver](#receiving-server).

A BombVault minden indításkor ellenőrzi a mentési céljait. Helyretesz mindent, ami nem kíván tőled döntést, például egy már nem létező másolatra vagy tárolóra mutató hivatkozást, egy szakasz saját mappáján lévő másolatot, vagy egy közvetlen tárolót, amelynek a másolata eltűnt, és mindegyik javításról ír egy sort a naplóba. Amire nincs biztonságos javítás, azt csak naplózza.

![Ugyanez az oldal telefonon.](assets/screenshots/storage/location-page-phone-dark.png#only-dark)
![Ugyanez az oldal telefonon.](assets/screenshots/storage/location-page-phone-light.png#only-light)

### Meddig őrizze meg {#keep}

Hogy a mentések meddig maradnak meg, mentési célonként állítod be, a **Meddig őrizze meg** kártyán. A szabály az ottani másolatokra és az ott lévő saját mappákra érvényes, a **Helyi** célt is beleértve. Egy szakasz a **Szabályok szakaszonként** alatt megtarthatja a saját értékeit egy célnál, és ezek felülírják a cél kártyáját az adott szakaszra.

A szabály naponta, hetente, havonta és évente egy mentést tart meg, addig visszamenőleg, ameddig beállítod, a többit pedig minden mentés vagy másolás után kitakarítja:

| Előbeállítás | Legutóbbi mentések | Napok | Hetek | Hónapok | Évek |
|---|---|---|---|---|---|
| **Rövid** | 0 | 7 | 4 | 3 | 0 |
| **Kiegyensúlyozott** | 0 | 7 | 4 | 6 | 2 |
| **Hosszú** | 0 | 14 | 8 | 12 | 3 |

A **Saját értékek** az öt számot te adod meg. A 0 kikapcsolja az adott számot, és az a szabály, amelyben minden szám 0, mindent megtart. A kártyán lévő pontok megmutatják, hogy egy szabály a 40 mintamentés közül melyiket tartja meg, a legújabb ott van a jobb szélen. A minta minden célnál ugyanaz, mert a kép azelőtt készül, hogy a BombVault beolvasná az ottani mentéseket. A minta a mai naptól számol visszafelé, így az, hogy hány pont marad, a dátummal változik.

Egy ezen a hoszton lévő mappának és egy kézzel beírt távoli mappának is saját kártyája van, amely az abban a mappában lévő saját mappákra vonatkozik, másra nem. Amíg nem mented, egy ilyen mappa azon szabály szerint öregszik, amely a BombVaultnál már megvolt hozzá.

A **Rákérdezés a kevesebb megőrzés előtt**, amely alapból be van kapcsolva, rákérdez, mielőtt egy kevesebb mentést megtartó módosítás elmentődik, és megmondja, a 40 mintamentés közül hányat távolít el az új szabály. Minden mentési célra érvényes. A **Vissza a cél értékeihez** a **Szabályok szakaszonként** alatt ugyanezt kérdezi.

A törlés elleni védelemmel rendelkező célt ez a gép soha nem metszi. A túloldala a saját szabályai szerint tartja meg vagy dobja el a régi mentéseket. Amíg egy elemnek van nyitott kritikus anomáliája elveszett adatokról, a megőrzés megtartja az elem régi mentéseit, amíg nyugtázod. Lásd: [Anomáliák](anomalies.md).

Az a mentés, amely kevesebbet tartalmaz a kelleténél, soha nem öregíti ki az előtte lévőket. Ilyen az a mentés, amely egy fájlt sem olvas be ott, ahol a korábbiakban voltak fájlok, amelyből hiányzik egy kiválasztott mappa, vagy amely néhány fájlt nem tudott beolvasni. A futása figyelmeztetéssel zárul, amely megnevezi, mi hiányzik, és közli, hogy a régebbi mentések megmaradtak, ugyanezt az üzenetet megkapod az értesítési csatornáidon is, és egyetlen nyesés sem nyúl az elem régi mentéseihez, sem az elem saját mappájában, sem a másolatainál, amíg egy teljes mentés át nem jut. Az első mentés és az a kiválasztott mappa, amely mindig üres volt, teljesnek számít.

Hogy a következő futás valójában mit törölne, azt az **Előnézet** mutatja a **Beállítások, Célok** alatt. Felsorolja, **Mit törölne a következő futás** a szakasz saját mappájára és minden másolatára, tárolónként és elemenként, a visszaállítási pontok megnevezésével. Nem vesz zárat, és semmit nem változtat.

![A Meddig őrizze meg a Kiegyensúlyozott kiválasztásával: a pontok megmutatják, a 40 mintamentés közül melyik marad.](assets/screenshots/storage/keep-dark.png#only-dark)
![A Meddig őrizze meg a Kiegyensúlyozott kiválasztásával: a pontok megmutatják, a 40 mintamentés közül melyik marad.](assets/screenshots/storage/keep-light.png#only-light)

#### Példa: mit tart meg a Kiegyensúlyozott {#example-retention}

Egy konténert 2024. október 1. óta minden éjjel 03:00-kor mentenek, így 2026. november 4-én, szerdán 765 mentése van. A mentési célja **Kiegyensúlyozott** értéken áll: 7 nap, 4 hét, 6 hónap és 2 év.

Minden szám a saját napjának, hetének, hónapjának vagy évének legújabb mentését tartja meg, és az a mentés, amelyet több szám is kiválaszt, egyszer marad meg. A hetek hétfőtől vasárnapig tartanak:

| Szabály | Mentések, amelyeket kiválaszt | Új mentések, amelyeket hozzáad |
|---|---|---|
| 7 nap | nov. 4., nov. 3., nov. 2., nov. 1., okt. 31., okt. 30., okt. 29. | mind a 7 |
| 4 hét | nov. 4., nov. 1., okt. 25., okt. 18. | okt. 25., okt. 18. |
| 6 hónap | nov. 4., okt. 31., szept. 30., aug. 31., júl. 31., jún. 30. | szept. 30., aug. 31., júl. 31., jún. 30. |
| 2 év | nov. 4., 2025. dec. 31. | 2025. dec. 31. |

Ez 765 mentésből 14-et hagy meg. A legrégebbi 2025. december 31-i, így ma valamivel több mint tíz hónapot mehetsz vissza. A deduplikációnak köszönhetően alig foglalnak több helyet, mint egy teljes mentés plusz az, ami közben változott.

Az **Évek** érdemes még egy pillantásra. A naptári éveket számolja, amelyekben van mentés, az aktuálissal kezdve, és ennek az évnek a legújabb mentése a mai. Ezért az első év nem ad hozzá semmit a többi számhoz, a második pedig az előző év utolsó mentését adja hozzá. Emiatt változik egy szabály visszanyúlása az év során. Ugyanezek a mentések az egyes előbeállításokkal:

| Előbeállítás | Megtartott mentések ma | Legrégebbi mentés ma | Visszanyúlás az év során |
|---|---|---|---|
| **Rövid** | 10 | 2026. szeptember 30. | 1-2 hónap |
| **Kiegyensúlyozott** | 14 | 2025. december 31. | kora tavasszal körülbelül 4 hónap, december 31-én egy teljes év |
| **Hosszú** | 30 | 2024. december 31. | január 1-jén 1 év, december 31-én 2 év |

A **Kiegyensúlyozott** értékről a **Rövid** értékre váltás előbb rákérdez, a **Rákérdezés a kevesebb megőrzés előtt** miatt. A kérdés megmondja, hogy a képen látható 40 mentésből hányat töröl az új szabály. Ez a szám a mintából jön, ezért a dátummal változik, és nem a saját mentéseidet számolja. Ennek a példának a mentései közül augusztus 31., július 31., június 30. és 2025. december 31. menne el. A **Mentés mégis** alkalmazza a szabályt, és a következő metszés véglegesen törli őket abból a célból.

Hogy a konténereket tovább tartsd, mint a többit ennél a célnál, nyomd meg a **Szabályok módosítása** gombot a **Szabályok szakaszonként** alatt, válaszd a **Konténerek** szakaszt, és állítsd be a **Saját értékek** értéket a **Konténerek szabályai** között, például 14 nap, 8 hét, 12 hónap és 2 év. A többi szakasz továbbra is a cél kártyáját követi, a **Vissza a cél értékeihez** pedig visszahozza a Konténereket.

### Bejelentkezések {#sign-ins}

A bejelentkezés az a kulcs vagy jelszó, amelyet a BombVault egy S3-szolgáltatáshoz vagy rest-serverhez használ. A bejelentkezések egyetlen lista, a **Beállítások, Célok** alatti **Bejelentkezések** kártya, és minden sor megmondja, mely mentési célok használják (**Használják**).

- **Elmentve, amikor felveszel egy célt.** A Cél hozzáadása a beírt kulcsokat bejelentkezésként menti. A következő, ugyanazon szolgáltatásnál lévő cél az **Új kulcsok** helyett a **Mentett bejelentkezés** alatt kiválaszthatja, és egy későbbi módosítás minden célra érvényes, amely használja.
- **Az alapértelmezett.** Egy csillag jelöli az alapértelmezett bejelentkezést. Mindenhol ezt használja, ahol nincs bejelentkezés kiválasztva, például egy kézzel beírt címnél vagy egy friss telepítésen végzett helyreállításnál. Egy lehívási forrás nem kölcsönzi. A **Legyen alapértelmezett** a csillagot egy másiknak adja.
- **Szerkesztés és teszt.** A **Szerkesztés**, itt vagy egy cél **Hitelesítő adatok** kártyáján, egyetlen ablakot nyit a bejelentkezésnek. A **Teszt** a beírt adatokkal bejelentkezik minden célba, amely használja, még mielőtt bármi elmentődne. A **Mentés** eltárolja.
- **Eltávolítás.** Egy bejelentkezést csak akkor lehet eltávolítani, ha egyetlen mentési cél, lehívási forrás vagy fogadott tároló sem használja, és régi mentéseknek sincs rá szükségük, és az alapértelmezett csak utolsóként távozhat. Régi mentéseknek akkor van rá szükségük, ha egy saját mappa elköltözött egy olyan helyről, amely ezzel jelentkezik be, és ott hagyta őket. A BombVault ilyenkor megmondja, melyik helyről van szó.
- **Egy cél kapcsolatának módosítása** soha nem írja át a más helyek által megosztott bejelentkezést. A cél ehelyett saját bejelentkezést kap, és az ablak előre megmondja, hogy a bejelentkezése megosztott. Hogy a megosztottat mindenhol megváltoztasd, szerkeszd a **Bejelentkezések** kártyán.

![A Bejelentkezések kártya: minden mentett bejelentkezés azzal, hogy mi használja, és a csillag az alapértelmezetten.](assets/screenshots/storage/signins-dark.png#only-dark)
![A Bejelentkezések kártya: minden mentett bejelentkezés azzal, hogy mi használja, és a csillag az alapértelmezetten.](assets/screenshots/storage/signins-light.png#only-light)

Egy ezen a szerveren lévő mappának nincs szüksége bejelentkezésre. Az SMB, a WebDAV, az SFTP, a Storage Box és a felhőmeghajtók a sajátjukat az rclone-konfigurációban tartják, amelyet a cél **rclone.conf** kártyája mutat, ezért nem szerepelnek ebben a listában.

### SMB, WebDAV és más rclone-remote-ok {#smb-webdav}

Egy Windows- vagy Samba-megosztás és egy WebDAV-kiszolgáló (Nextcloud, ownCloud, SharePoint vagy bármely más) szolgáltatói csempe a **Cél hozzáadása** alatt, a **NAS vagy szerver** csoportban. Add meg a gazdagépet és a megosztást (SMB) vagy a címet és a kiszolgáló típusát (WebDAV), a felhasználót és a jelszót, és a BombVault megírja helyetted az rclone-szakaszt. Az rclone tárolás előtt elhomályosítja a jelszót.

Egy olyan szolgáltatáshoz, amelyet egyetlen csempe sem fed le, a **Saját rclone-konfiguráció** alatti **Rclone** csempe egy rclone.conf-ból bemásolt remote-okat fogad. Ezek hozzáadódnak a BombVault meglévő remote-jaihoz, soha nem váltják le őket, és titkosítva tárolódnak. Hogy egy szakaszt egyikükre ments, add meg az `rclone:<remote>:<mappa>` címet a szakasz telephelyen kívüli tárolójaként, például `rclone:nas:backups/bombvault`. A megosztás vagy a bucket az útvonal első szegmense, nem a név része.

Ez jobb út, mint a megosztás csatolása Unraidon: a restic nem javasolja, hogy a tároló csatolt CIFS-megosztáson legyen, itt pedig semmi sincs csatolva. Az NFS azért nincs csempén, mert sem a restic, sem az rclone nem rendelkezik NFS-háttérrendszerrel. NFS-hez csatold az exportot a gazdagépen, és használd helyi mappaként.

### Költöztetés és eltávolítás {#move-remove}

- A **Költöztetés** egy szakasz másolatát az egész előzményével egy másik mentési célra viszi. Minden a régi helyen marad. A BombVault ott nem töröl semmit. A költöztetés a háttérben fut, és sokáig tarthat: kb. 9 perc 100 GB-onként helyi lemezek között, órákig is tarthat lassú kapcsolaton. Az Áttekintés mutatja, amíg fut.
- A **Cél eltávolítása** két utat kínál. A **Cél eltávolítása, a mentések megtartása** nem ír tovább oda, a mentéseket a helyükön hagyja. Később elolvashatod őket a **Visszaállítás, Régi mentési mappából** alatt. A **Cél eltávolítása és az ottani mentések törlése** előbb beolvassa, mi van ott tárolva, és felsorolja: mely mentések tűnnek el, ezek közül melyek nincsenek máshol, és mely tárolók maradnak, mert a törlés elleni védelem megtagadja, elemek még közvetlenül oda mentenek, egy másik cél vagy szakasz tartja ott a másolatait, vagy nem lehetett őket olvasni. Azok az elemek, amelyek elveszítenék az egyetlen mentésüket, külön szerepelnek, és megmaradnak, kivéve ha bekapcsolod az **Ezeket is töröld** kapcsolót. A cél nevének beírásával erősíted meg, és a törlés a háttérben fut.
- Ez minden fajta mentési célra működik: olyanra, amelyet több szakasz használ, egyetlen szakaszhoz beállított célra, magad létrehozott tárolóra és arra a mappára, amelybe egy szakasz a saját mentéseit írja. Az eltávolított tárolóba mentő elemek ezután újra a szakaszukat követik, és ugyanígy a szakasz oda mutató alapértelmezése is. Egy szakasz mappáját nem lehet csak úgy törölni, mert minden szakasznak kell egy. Az ablak ezért megkérdezi, melyik mappába költöznek a szakaszok, és mindegyik szakasz megtartja alatta a saját mappanevét.
- Egy mentési cél mentéseinek törlése bejelentkezési jelszó nélkül is működik. Ha van jelszó, bejelentkezés kell hozzá magán a példányon.

### Példa: telephelyen kívüli másolat a Backblaze B2-n {#example-s3}

A konténereknek és az Önmentésnek a Backblaze B2-n is ott kell lennie, hogy egy elpusztult szerver ne vigye magával őket. Ugyanezek a lépések illenek a Wasabihoz, a Hetzner Object Storage-hoz és a legtöbb más S3-szolgáltatáshoz.

1. **A B2-ben** hozz létre egy privát bucketet, például `tower-backups` néven, egy hozzád közeli régióban. Az **Application Keys** alatt hozz létre egy olvasási és írási hozzáférésű kulcsot, erre a bucketre korlátozva, és jegyezd fel a kulcsazonosítóját és az alkalmazáskulcsot. A B2 az alkalmazáskulcsot egyszer mutatja. Állítsd a bucket életciklusát úgy, hogy csak az utolsó verziót tartsa meg, különben a B2 a metszett adatokat rejtett verzióként megőrzi, és ki is számlázza őket.
2. **Szolgáltató.** A BombVaultban nyisd meg a **Célok** oldalt, nyomd meg a **Cél hozzáadása** gombot, és válaszd a **Backblaze B2** csempét az **S3-szolgáltatás** alatt.
3. **Bejelentkezés.** Add meg a kulcsazonosítót **Hozzáférési kulcs azonosítója** néven és az alkalmazáskulcsot **Titkos hozzáférési kulcs** néven. A **Végpont** alá írd be a bucket végpontját, például `s3.eu-central-003.backblazeb2.com`, és a BombVault ebből olvassa ki a régiót. Írd be a `tower-backups` nevet a **Bucket** mezőbe, mert ez a kulcs csak egy bucketet nyit meg.
4. **Kapcsolat tesztelése.** Ezt kell mondania: **A kapcsolat létrejött**, és listáznia kell a bucket mappáit. Nyomd meg az **Új mappa** gombot, és nevezd el `bombvault`-nak. A BombVault szakaszonként egy mappát hoz létre benne, például `bombvault/containers`.
5. **Védelem.** Az ablak elmagyarázza, hogy a B2 object lockkal meg tudja őrizni a régi verziókat, és hogy a BombVault ezt nem tudja ellenőrizni. Hagyd kikapcsolva a **Törlés elleni védelem (append-only)** kapcsolót, hacsak a bucket tényleg meg nem tagadja a törlést, hogy a BombVault a cél megőrzési szabályai szerint metszhessen.
6. **Mely szakaszokhoz.** Pipáld ki a **Konténerek** és az **Önmentés** szakaszt. Az Önmentés a beállításaidat viszi, így egy új telepítés vissza tudja hozni őket. Az ablak megmondja, hány elemről van szó, és ahol ismert, mennyi előzményt visz magával az első másolat.
7. Nyomd meg a **Cél mentése** gombot.

Az első másolat a következő mentés után fut. Hogy most indítsd el, nyisd meg a **Beállítások, Célok** oldalt, és nyomd meg a **Replikálás most** gombot a szakasz sorában a **Telephelyen kívüli másolatok** között. Az állapotfejléc ezt írja: **Telephelyen kívüli másolás folyamatban**. Az első másolat az egész előzményt viszi, így 200 GB egy 100 Mbit/s-os feltöltésen nagyjából négy és fél óra. A későbbi másolatok csak azt küldik, ami új.

Hogy a vonalad szabad maradjon, nyisd meg a cél oldalát, és állítsd be a **Feltöltési korlát (KiB/s)** értékét a **Másolás** kártyán, például `5120`-ra, ami nagyjából 5,0 MB/s (41,9 Mbit/s). Ha már ott vagy, nézd meg a **Meddig őrizze meg** kártyát is ugyanazon az oldalon.

Hogy lásd, működött, nézd meg az Áttekintés **Védelmi állapot** kártyáját, ahol a **Telephelyen kívüli** állapot átvált a **másolat naprakész** állapotra, és egy konténer **Biztonsági mentések** lapját, ahol a mentései már a `tower-backups` bucketet nevezik meg második helyként. Ezután nyomd meg újra a **Helyreállítási csomag letöltése** gombot a **Beállítások, Visszaállítás** alatt, mert a csomag az új célt és a bejelentkezését is felsorolja.

**A Cloudflare R2 ugyanígy működik,** kulcs helyett objektumtokennel. Illeszd be az S3-címet, amelyet az R2 irányítópultja mutat, a bucketet is tartalmazva, és a BombVault kitölti a szervert és a bucketet.

### Példa: egy rest-server, amelyet magad üzemeltetsz {#example-rest-server}

Az append-only módban futó rest-server megtagadja a törlést, így egy rajta lévő másolat akkor is túléli, ha valaki átveszi az irányítást a szervered felett. Egy második gép elég hozzá, egy barátodnál vagy a garázsban. Egy párosított BombVault be tudja állítani helyetted, lásd: [Fogadószerver](#receiving-server). Hogy kézzel futtasd bármely Docker-gazdán:

```sh
docker run -d --name rest-server --restart unless-stopped \
  -p 8000:8000 \
  -v /mnt/user/backups/rest-server:/data \
  -e OPTIONS="--append-only --private-repos" \
  restic/rest-server
docker exec -it rest-server create_user bombvault
```

Az utolsó sor jelszót kér, és kiírja a felhasználót az adatmappa `.htpasswd` fájljába.

1. A BombVaultban nyomd meg a **Cél hozzáadása** gombot, és válaszd a **rest-server** csempét a **NAS vagy szerver** alatt.
2. Add meg a címet a felhasználóval mint mappával, például `http://192.168.2.20:8000/bombvault`, valamint a **Felhasználónév** és a **Jelszó** mezőt. Nyomd meg a **Kapcsolat tesztelése** gombot.
3. A **Védelem** lépésben kapcsold be a **Törlés elleni védelem (append-only)** kapcsolót. A BombVault ezután ott soha nem metsz, és ezt a túloldalra hagyja.
4. Pipáld ki a szakaszokat, és nyomd meg a **Cél mentése** gombot.
5. Amint az első másolat ott van, nyisd meg a cél oldalát, és nyomd meg a **Törlés elleni védelem tesztelése most** gombot. A BombVault megpróbál törölni egy tesztobjektumot. A kártyának ezt kell írnia: **Utoljára: törlés elutasítva**, az Áttekintés **Törlés elleni védelem** értéke pedig **ellenőrizve** lesz. Ha azt írja, hogy **Utoljára: a törlés nem lett elutasítva**, hiányzik az `--append-only`.

Mivel innen semmi nem metszi ezt a másolatot, állítsd be a **Növekedési keret (GB, 0 = ki)** értékét a szakasz ablakában, amelyet a **Telephelyen kívüli másolatok** között a sorában lévő **Szerkesztés** gombbal nyitsz meg, hogy szólni tudjon, mielőtt a lemez megtelik. Önaláírt tanúsítványnál lásd a `certs` mappáról szóló fenti megjegyzést.

## Egy szakasz saját mappája {#own-folder}

Minden szakasznak van saját mappája: az a tároló, amelybe a mentései először íródnak. A más mentési célokra menő másolatok onnan indulnak. Egy új telepítés a saját mappákat ezen a szerveren tartja, a `/mnt/user/bombvault` alatt, és együtt alkotják a **Helyi** mentési célt.

A saját mappa egy már meglévő mentési célon is lehet, például egy NAS-on, egy rest-serveren vagy egy S3-bucketen. A szakasz ilyenkor közvetlenül oda ment, helyi másolat közbeiktatása nélkül, és a saját mappa a cél bejelentkezését, törlés elleni védelmét és sávszélesség-korlátait követi. A BombVault minden mentés után a cél **Meddig őrizze meg** kártyája szerint metszi, és egy szakasz saját értékei a **Szabályok szakaszonként** alatt felülírják a kártyát. A cél a **Saját mappák** kártyáján felsorolja a szakaszt, a **Célok szakaszonként** pedig a célt a szakasz otthonaként mutatja.

### Saját mappa áthelyezése {#move-own-folder}

1. Nyisd meg azt a mentési célt, amelynek a saját mappát kell tartania, és válaszd a **Használat saját mappaként** gombot, vagy a **Szakaszok hozzáadása** gombot a **Saját mappák** kártyáján. Hogy egyszerre több szakaszt költöztess, válaszd a **Saját mappák áthelyezése** gombot a **Helyi** cél **Kapcsolat** kártyáján a **Saját mappák itt** mellett.
2. Válaszd ki a szakaszokat a **Szakaszok** alatt, a helyet pedig az **Áthelyezés ide** alatt. A **Másik hely** egy ezen a gazdán lévő mappát vagy egy kézzel beírt távoli tároló címét fogadja, a mentett bejelentkezéseid közül választott bejelentkezéssel.
3. Olvasd el a **Mi történik** részt. Szakaszonként megmondja, hová mennek az új mentések, a már az új helyen lévő másolatból saját mappa lesz-e, és mely mentések másolódnak át előbb.
4. A **A már elkészült mentések másolása** alapból be van kapcsolva. Kikapcsolva a régi mentések ott maradnak, ahol vannak, de az elem már nem sorolja fel őket. A Visszaállítás oldalon a **Régi mentési mappából** továbbra is beolvassa őket a mappa címével, távoli mappánál pedig a bejelentkezésével.
5. Válaszd az **Átváltás** gombot. Ha az új hely **Meddig őrizze meg** kártyája kevesebbet tart meg, mint a szakasz mai szabálya, a BombVault előbb megkérdezi: **Kevesebbet őrizzen meg?** Itt a szakasz **jelenlegi értékeinek megtartása a Szabályok szakaszonként részben** magával viszi a mai értékeket, a **Mentés mégis** továbbmegy, a **Vissza** pedig mindent úgy hagy, ahogy volt. Ha előbb mentéseket kell átmásolni, a BombVault átmásolja őket, majd átváltja a saját mappát. Ha már ott vannak, az átváltás előtt ellenőrzi őket.

Az áthelyezés után az új mentések az áthelyezett saját mappába mennek, olyan szakasznál is, amelynél a **Helyi** ki volt kapcsolva, és közvetlenül egy másolatra mentett. Az a másolat, amely már az új helyen volt, saját mappa lesz, és az oda másolás megszűnik. Ahol az a másolat bizonyos mentéseket tovább őrzött, mint a szakasz megőrzési szabályai, a szakasz megtart mindent, amit akár az egyik, akár a másik szabály megtart, így egyik sem vész el.

Ha az áthelyezés nem mehet végbe, az ablak a **Mi történik** alatti dobozban megmondja, miért. Ha csak a régi mentések másolása az akadály, a **A már elkészült mentések másolása** kikapcsolásával a saját mappa azok nélkül is áthelyezhető.

Egy szakasznak mindig kell saját mappa. Az azt tartó mentési cél eltávolítása megkérdezi, melyik mappába költözik a szakasz, lásd: [Költöztetés és eltávolítás](#move-remove). Az a saját mappa, amely mellett az új mentések elmennek, mert a szakasz közvetlenül egy másolatra ment, **(ki)** jelölést kap ott, ahol egy cél felsorolja, ki használja.

![Az ablak, amely egy saját mappát helyez át, a Mi történik kitöltve egy szakaszra.](assets/screenshots/storage/move-own-folder-dark.png#only-dark)
![Az ablak, amely egy saját mappát helyez át, a Mi történik kitöltve egy szakaszra.](assets/screenshots/storage/move-own-folder-light.png#only-light)

#### Példa: a Konténerek egy NAS-ra költöznek {#example-own-folder}

A cache-pool telik, és van egy `nas` nevű mentési célod, egy SMB-megosztás egy második gépen. A Konténerek szakasznak 61 mentése van a saját mappájában ezen a szerveren, és a Backblaze B2-re másol.

1. Nyisd meg a **Célok** oldalt, majd a `nas` célt, és nyomd meg a **Használat saját mappaként** gombot.
2. Pipáld ki a **Konténerek** szakaszt a **Szakaszok** alatt.
3. Olvasd el a **Mi történik** részt. Azt mondja, hogy a Konténerek új mentései a `nas` mappájába mennek, a Helyiben lévő 61 mentés előbb odamásolódik, a másolatok továbbra is a Backblaze B2-re mennek, a védelem, a sávszélesség-korlátok és a növekedési riasztás pedig a `nas`-tól jön.
4. Hagyd bekapcsolva a **A már elkészült mentések másolása** kapcsolót, és nyomd meg az **Átváltás** gombot.

A BombVault a háttérben átmásolja a mentéseket, majd átváltja a saját mappát. A `nas` oldala a Konténereket a **Saját mappák** alatt sorolja fel, a szakaszát követő elem a `nas`-t nevezi meg otthonaként, és a következő mentés oda kerül, a B2-re menő másolattal, amely a `nas`-ról indul. Mostantól a `nas` **Meddig őrizze meg** kártyája metszi a mappát. A régi mappa ezen a szerveren ott marad, ahol van, és a BombVault ott nem töröl semmit.

### Távoli cím kézzel beírva {#remote-primary-repositories}

A **Helyi** cél **Kapcsolat** kártyája továbbra is fogad távoli címet egy szakasz saját mappájához: a szakasz melletti **Módosítás** egy **Helyi / Távoli** kapcsolós ablakot nyit. A **Távoli** egy restic-címet fogad, például `s3:...`, `rest:http://host:8000/repo`, `sftp:user@host:/repo` vagy `rclone:remote:bucket/path`, a **Távoli elsődleges tároló biztonsági beállításai** pedig kapcsolatesztet, feltöltési és letöltési korlátokat, append-only védelmet ugyanazzal a manipulációs teszttel, amelyet egy telephelyen kívüli cél kap, valamint növekedésikeret-riasztást adnak hozzá.

Egy ilyen cím az alapértelmezett bejelentkezéssel jelentkezik be. Ha pontosan az a mappa, amelyet egy meglévő mentési cél a szakasznak tart fenn, magától követi azt a célt, a cél kártyáját és az **Elérhető** kapcsolóját is beleértve, kivéve, ha másképp jelentkezik be, vagy append-only, miközben a cél nem az. Az a szabály, amely addig a mappát nyeste, a szakasz saját szabályaként megmarad a **Szabályok szakaszonként** alatt. Ha a cél egy másik mappájában van, a cél oldala a szakaszt a **Már ennek a célnak az alatt van** alatt sorolja fel, és az átvétel megadja neki a cél bejelentkezését, tárolási osztályát és törlés elleni védelmét. A saját mappa mentési célba költöztetése a **Használat saját mappaként** gombbal az egyszerűbb út, mert az ablak minden változtatás előtt ellenőrzi a már elkészült mentéseket.

## Elhelyezés elemenként {#placement}

Egy elem Beállítások lapján az **Ütemezés és cél** kártya gombsort tartalmaz: **Helyi**, valamint egy gomb a szakasz minden telephelyen kívüli mentési céljához. A világító gombok kapják az elem mentéseit.

- Ha a **Helyi** világít, az elem a **Tárolva itt** alatt látható tárolóba íródik, és átmásolódik minden más világító célra. Sötétíts el egy célt, és az nem kap többé újat ettől az elemtől. A csak Helyi sehová sem másol, ami olyan adatnak jó, amelynek már van második másolata, például egy NAS-on élő megosztásnak.
- Ha a **Helyi** sötét, az elem közvetlenül az első világító cél közvetlen tárolójába íródik, onnan pedig átmásolódik a többi világító célra. Először egy párbeszédablak létrehozza ezt a közvetlen tárolót.
- Egy gomb mindig világít, mert a mentésnek kell egy hely, ahová menjen. Ha egy elemet ki akarsz hagyni a mentésekből, használd a **Ne legyen mentés** opciót.

A BombVault soha nem mozgat mentéseket tárolók között. Az az elem, amely követi a szakaszát, vele tart, ha a szakasz alapértelmezése változik: a következő mentése az új helyre kerül, a meglévő mentései pedig ott maradnak, ahol vannak. Az elemen választott hely az első mentésétől fogva rögzített. A Beállítások lap **Tároló** kártyája megnevezi. A másolatok bármikor változhatnak. Az a cél, amely már nem kap egy elemet, megtartja a meglévő másolatait, és a szakasz következő telephelyen kívüli futásakor a saját megőrzésére vágja őket vissza. A csak hozzáfűzésre szolgáló célokból nem lehet törölni.

A hely, amelyet egy elem elhagyott, megtartja a mentéseit. Az elem **Mentések** fülén és a **Visszaállításnál** **Csak korábbi mentések** néven áll, a visszaállítás továbbra is olvashat belőle, a metszés pedig békén hagyja ezeket a mentéseket.

Az **Alapértelmezés követése** visszahozza a saját választású elemet a szakaszához. Előbb rákérdez, megmondja, hová mennek ezután az új mentések, és megnevezi, mit nem kap többé az egyes cél, és mit töltene fel a következő futás.

A sor alatt a kártya megmondja, hová megy az elem, és mi van valójában ott: hány telephely tartja, mikor látták utoljára az egyes célokat, és teljesül-e a 3-2-1. Telephely az eredeti adatokat tartalmazó szerver, minden telephelyen kívüli cél és minden **Telephelyen kívül** jelölésű tároló. A BombVault a másolatokat és a telephelyeket ellenőrzi. A 3-2-1 „két adathordozó” részét nem.

### Alapértelmezések szakaszonként {#placement-defaults}

A Célok oldal **Célok szakaszonként** kártyája minden szakasz alapértelmezését 3-2-1 ábraként rajzolja fel. A szakasztól egy mentési célig vezető minden vonal egy kapcsoló, a **Helyi** is, minden szakaszra, a flash-meghajtóra, a ZFS-adatkészletekre és az Önmentésre is. Egy vonalra vagy pontra kattintva azonnal átkapcsolod, a szaggatott vonalak ki vannak kapcsolva. Csak egy szakasz **Helyi** vonalának kikapcsolása kérdez rá előbb, ugyanabban a párbeszédablakban, mint az alapértelmezés kártya, mert a szakasz ilyenkor közvetlenül egy másolatra ment. A **Mégse** mindent úgy hagy, ahogy volt. Az **Ábra / Kapcsolók** ugyanezt gombsorokként mutatja. A kapcsoló dönti el, hová mennek az új mentések. A saját választás nélküli elemek és a Compose-stackek projektmappái azonnal követik, így a következő mentésük az új helyekre megy. A már eltárolt mentések ott maradnak, ahol vannak, és onnan továbbra is vissza lehet állítani őket. A flash, a ZFS-adatkészletek és az Önmentés a helyi mappájukba mentenek, és onnan másolnak a telephelyen kívülre. A **Helyi** felé vezető vonaluk is kikapcsolható. A mentéseik ilyenkor közvetlenül a telephelyen kívüli céljaikra mennek, és az a ZFS-elem, amelynek van replikája, de egyáltalán nincs célja, csak a replikáját küldi.

Ha a **Helyi** ki van kapcsolva, és csak telephelyen kívüli célok vannak bekapcsolva, a szakasz közvetlenül oda ment, és a tárolót ott a BombVault maga választja ki. A konténerek, VM-ek és mappakészletek az első bekapcsolt cél közvetlen tárolójába kerülnek, az alábbiak szerint, és onnan másolódnak a többire. A flash-meghajtó, a ZFS-adatkészletek és az Önmentés minden mentést minden bekapcsolt célra írnak, és minden cél a saját megőrzése szerint tartja meg őket.

Az a szakasz, amelynek minden vonala ki van kapcsolva, azt mondja: **Nincs mentve**. Az elemei nem kapnak mentést, és nem védettnek számítanak.

Az elemek, amelyek az oldalukon saját mentési célokat választottak, megtartják azokat. Az ábra megmondja, hányan vannak ilyenek egy szakaszban, például **2 elem eltér**, és hozzájuk vezet: egynél az elem Beállítások fülére, többnél egy listához, amely megmutatja, ki hová ment. Ott az **Alapértelmezés követése** visszahoz egy elemet.

Az új telephelyen kívüli cél minden olyan elemet megkap, amely nincs Helyire állítva. Az ablak, amely hozzáadja, megmondja, hány elemről van szó, és ahol ismert, mennyi előzményről.

### Közvetlen tárolók {#direct-repositories}

A közvetlen tároló az a hely, ahová az elemek közvetlenül egy telephelyen kívüli célra mentenek. A BombVault a cél mappája mellé teszi, a névhez `-direct` toldalékot fűzve, vagy ha a cél a bucketje, a remote-ja vagy egy rest-server felhasználó gyökerében áll, egy benne lévő, a szakaszról elnevezett mappába, például `containers-direct`. A Célok oldal ábráján rákérdezés nélkül jön létre. Ha egyetlen elemnél kapcsolod ki a Helyit, hogy egy közvetlen tároló nélküli cél legyen az otthona, megnyílik egy párbeszédablak a javasolt hellyel, például `s3:https://s3.eu-central-003.backblazeb2.com/bucket/containers-direct`, és egy kapcsolatteszttel, amely nem hoz létre semmit. A **Létrehozás és használat** létrehozza a tárolót, és az elemet odairányítja. A közvetlen tárolónak nincsenek saját beállításai. A cél kapcsolóját, bejelentkezését, tárolási osztályát, korlátait, append-only beállítását és megőrzését követi, és ezekkel együtt változik. A cél oldala az **Ide ment közvetlenül** alatt sorolja fel, minden szakasszal, amely közvetlenül oda ment, és hogy hány elem teszi ezt. Amelyikbe már semmi nem ment, az ott ezt jelzi, és eltávolítható, a helyén lévő mentések pedig megmaradnak. Ha a cél új kulcsa nem tudja megnyitni, a közvetlen tároló megtartja a kulcsát, és a mentés ezt jelzi. A közvetlen tárolón lévő elem onnan másolódik a többi világító célra, soha nem arra a célra, amelyhez a tároló tartozik. A pillanatképeik `bv:direct` címkét viselnek, és minden más megőrzési menet megtartja őket, így az a közvetlen tároló, amely elvesztette a kapcsolatát a céljával, soha nem öregszik a helyi szabályok szerint. A B2-t az S3-végpontján éred el, a kulcsazonosítót és az alkalmazáskulcsot S3-kulcsként megadva. A cél saját mappájára korlátozott kulcs nem éri el a mellette lévő mappát, ezért a kulcsot inkább az fölötte lévő mappára korlátozd.

### Telephelyen kívül {#off-the-premises}

Egy mentési cél vagy egy tároló megjelölhető **Telephelyen kívül**. A távoliak jelöltként indulnak. Kapcsold ki egy ugyanabban az épületben lévő rest-servernél. A jelölés csak a telephelyeket és a 3-2-1-et számolja a kártyákon. Egyetlen másolatot sem változtat.

A ZFS-replika telephelyen kívüli másolat, de nem előzményes mentés. Csak az általad beállított pillanatképeket őrzi meg, a régi verziók és az egyes fájlok továbbra is a mentésekből jönnek. Az egyik ZFS-kiszolgálóról közvetlenül a másikra megy, SSH-n át egy ZFS-kiszolgálóra vagy egy párosított példány fogadóvégpontjára. A párosítási csoport csak a kérést szállítja. Lásd: [Replika](zfs-datasets.md#replica).

### Újraépítés után {#after-a-rebuild}

A másolási döntések a BombVault saját beállításaiban élnek. A **Biztonsági mentések felfedezése** vagy a Vészvarázsló utáni újraépítés után, visszaállított beállítások nélkül, ezek elvesznek, és mindent másolni azokat az elemeket is újra B2-re küldené, amelyeket kihagytál. Ugyanez érvényes, ha egy szakasz, amely ebből az adatbázisból még soha nem másolt telephelyen kívülre, saját mentéseket talál egy mentési célon, vagy régebbieket a saját mappájában, például miután egy célt egy korábbi, ugyanazzal az `APP_KEY`-jel futó telepítés mentései fölé adtak hozzá. Ennek a szakasznak a telephelyen kívüli másolatai ekkor a döntésedre várnak.

Az áttekintésen a **Figyelmet igényel** és minden olyan cél oldala, amelyre a szakasz másol, megkérdezi, hogy a mentések a szerver saját mentései-e. Az **Átvétel a szerver saját mentéseiként** megerősíti a szakasz alapértelmezését, és a másolatai a következő körtől újra futnak. Az **Előbb a szabályok ellenőrzése** a szakasz sorához visz a **Célok szakaszonként** kártyán, ahol látod, hová kerül az egyes elem, és egy elveszett szabályt újra beállíthatsz az elem oldalán. A másolás ott tovább vár, amíg meg nem nyomod az **Átvétel a szerver saját mentéseiként** gombot, amely előbb megmutatja, mit másol a következő futás, és azokat a neveket a mentésekben, amelyekhez nincs elem, és amelyeket kihagyhatsz. Minden váró elem legújabb mentése ezt a futásában jelzi, a döntésre mutató hivatkozással, és ha az értesítések be vannak kapcsolva, értesítés is megy. Egy beállításfájl importálása visszahozza a szabályokat és az alapértelmezéseket, de nem hozza meg helyetted a döntést.

![A Figyelmet igényel újraépítés után: a mappakészletek ennél az adatbázisnál régebbi mentéseket találtak, és a telephelyen kívüli másolataik a döntésre várnak.](assets/screenshots/offsite/copy-decision-dark.png#only-dark)
![A Figyelmet igényel újraépítés után: a mappakészletek ennél az adatbázisnál régebbi mentéseket találtak, és a telephelyen kívüli másolataik a döntésre várnak.](assets/screenshots/offsite/copy-decision-light.png#only-light)

## Törlés elleni védelem (append-only) {#delete-protection-append-only}

Kapcsold be a törlés elleni védelmet egy telephelyen kívüli mentési célnál, hogy a zsarolóvírus vagy egy feltört gazdagép ne törölhesse vagy írhassa felül a mentéseidet. A túloldal (egy `--append-only` módban futó `restic/rest-server`) **érvényesíti**. A BombVault csak **ellenőrzi**, és soha nem mutat zöldet pusztán egy beállítási állítás alapján. Az ezen a szerveren lévő mappa nem tagadhatja meg a törlést, mert aki a BombVaultot irányítja, a mappát is irányítja, ezért a helyi célok nem kínálják a kapcsolót.

A kapcsoló a mentési célhoz tartozik: egy szakasz, amely oda másol, a céltól veszi át, ezért a cél oldalán, a **Célok** alatt kapcsold be. Az ütemezett **Törlésvédelem ellenőrzése** csak addig fut, amíg egy szakasz olyan célra másol, amelyen a kapcsoló be van kapcsolva. A túloldalt `--append-only` kapcsolóval kell indítani. A `restic/rest-server` image esetén ez TrueNAS-on, mint bármely más Docker-gazdán, az `OPTIONS` változóba kerül.

Egyes helyek egyáltalán nem tudják megtagadni a törlést, például egy felhőmeghajtó, mint a OneDrive. Ha tudatosan tartasz ott másolatokat, kapcsold be a hely oldalán az **Elfogadás törlés elleni védelem nélkül** kapcsolót. Az Áttekintés ekkor annak a szakasznak a hiányzó törlés elleni védelmét, amely csak elfogadott helyekre másol, elfogadott kockázatként mutatja, szürke gyűrűként borostyánsárga pont helyett, és a szakasz színe a többi ellenőrzését követi. Más nem változik: az ottani mentések továbbra is törölhetők, a BombVault továbbra is a hely megőrzési szabályai szerint metszi őket, és a kapcsoló eltűnik, amint a törlés elleni védelem bekapcsol. Egy másik helyre költöző másolat elveszti az elfogadást. A törlés elleni védelem bekapcsolása is visszavonja.

A leggyorsabb út egy append-only másolathoz egy párosított BombVault [fogadószerverrel](#receiving-server): ez helyetted futtatja a rest-servert, append-only módban.

!!! note "A `/locks/` alatti sikeres törlés várható"
    Az append-only nem azt jelenti, hogy soha semmit nem lehet eltávolítani. A restic-nek fel kell vennie és el kell engednie a saját zárait, ezért a `/locks/` szándékosan írható és törölhető marad. A pillanatképeket és a mögöttük lévő adatokat, amelyekre a zsarolóvírus menne, nem lehet eltávolítani. Ha magad próbálgatod a túloldalt, a `/locks/` alatti sikeres törlés helyes viselkedés, és nem lyuk a védelemben.

!!! warning "Az append-only tárolókat ez a gép soha nem metszi"
    Egy append-only mentési cél innen soha nem metszi a régi pillanatképeket. A metszés a túloldal dolga. Állíts be hozzá **növekedésikeret-riasztást**, hogy időben értesülj, mielőtt a tároló mérete elszabadul.

## Manipulációs teszt {#tamper-test}

A BombVault rendszeresen bizonyítja az append-only garanciát azzal, hogy ténylegesen törlést kísérel meg a telephelyen kívüli tárolón, egy nem létező objektumra célozva. A **Törlés elleni védelem tesztelése most** egy cél oldalán igény szerint lefuttatja, a **Törlésvédelem ellenőrzése** pedig a **Beállítások, Ellenőrzés, Automatikus ellenőrzések** alatt ütemezve futtatja.

A **Törlésvédelem ellenőrzése** kikapcsolása nem rejt el hiányt. Egy törlés elleni védelemmel rendelkező, de soha nem tesztelt cél ekkor azt írja: „nem bizonyított, az ellenőrzése ki van kapcsolva”, a törlés elleni védelem nélküli cél pedig mindkét esetben „nincs” értéket mutat.

A kikapcsolt másolatot nem teszteli, és a cél ezt jelzi. A kapcsolatteszt továbbra is működik rajta, mert csak olvas, így egy másolat bekapcsolás előtt is ellenőrizhető.

- Az **Elutasítva** azt jelenti, hogy védett.
- Az **Elfogadva** azt jelenti, hogy nem védett.
- Egy **nem meggyőző** eredmény (a szerver elérhetetlen, hitelesítési hiba) soha nem billenti át a tárolt ítéletet.

Egy valódi védett-védtelen átbillenés egyetlen riasztást indít.

## DR-próbák {#dr-drills}

A BombVault kétféle szintű bizonyítékot kínál arra, hogy a mentéseid ténylegesen visszaállíthatók, nem csak jelen vannak.

- **Visszaállítás-ellenőrző próbák (helyi).** A BombVault rendszeresen lefuttatja a `restic check --read-data-subset` parancsot (korlátozott, soha nem lemezt megtöltő teljes visszaállítás). Az ütemezés a **Beállítások, Ellenőrzés, Automatikus ellenőrzések** alatt él, az eredményt ugyanezen a csempén a **Mentések ellenőrzése** kártya mutatja.
- **Próbavisszaállítás (telephelyen kívüli).** A BombVault egy valódi célt visszaállít a telephelyen kívüli tárolóból egy elzárt homokozóba, fájlonként és bájtonként ellenőrzi, majd takarít. Ez bizonyítja, hogy a telephelyen kívülről helyre tudsz állni, nem csak azt, hogy a tároló válaszol.

Az Áttekintés **Védelmi állapot** kártyája ezt szakaszonként négy ellenőrzésbe sűríti: friss mentés ezen a szerveren, másolat telephelyen kívül, törlés elleni védelem ott, és próbavisszaállítás onnan. Egy szakaszra kattintva megjelennek a részletek, és a kártya csak ellenőrzött tények alapján lesz zöld. Egy hely, amelyet törlés elleni védelem nélkül fogadtál el, ott elfogadott kockázatnak számít, nem figyelmeztetésnek.

## Példányok párosítása {#pairing}

Több BombVault dolgozik együtt egy csoportként: megmutatják egymásnak a védelmi állapotukat, szerepeket vállalnak egymásért, és őrizhetik egymás kulcsát. Egy példány tizenkét szóval csatlakozik a csoporthoz.

![A párosítási oldal a három lépéskártyájával.](assets/screenshots/offsite/pairing-dark.png#only-dark)
![A párosítási oldal a három lépéskártyájával.](assets/screenshots/offsite/pairing-light.png#only-light)

Az első példányon nyisd meg a **Beállítások, Párosítás** oldalt, és nyomd meg a **Jelmondat létrehozása** gombot. Három lépéskártya mutatja a menetet. Tizenkét szó jelenik meg egy ablakban a **Másolás** gombbal. Minden további példányon nyisd meg ugyanazt a helyet, nyomd meg a **Jelmondat megadása** gombot, és illeszd be vagy írd be őket, vagy nyomd meg a **Beillesztés** gombot abban az ablakban. A listán nem szereplő szót gépelés közben a helyével együtt megnevezi, az utolsó szó pedig ellenőrzőösszeget hordoz, így egy elgépelt vagy felcserélt szó még a párosítás előtt kiderül. A jelmondatot csak egy példányon hozd létre: két példány, amely mindkettő létrehoz egyet, két külön csoportot alkot. Ha egy percig senki sem jelentkezik, az oldal két kiutat kínál: mutasd meg újra a szavakat, hogy ott add meg őket, vagy add meg a másik példány szavait, és egy lépésben csatlakozz az ő csoportjához. A párosítás bejelentkezési jelszó nélkül is működik, de állíts be egyet: nélküle bárki, aki megnyithatja a webes felületet, elolvashatja a szavakat, és a csoporton át olyan szerepeket kérhet, amelyek kiadják a benne lévő példányok restic-jelszavát. A párosítási kártya ezt jelzi, amíg nincs jelszó beállítva. Jelszóval a jelmondat ismételt megmutatásához is kell a jelszó. A **Csoport elhagyása** kivesz egy példányt. A párosítási oldal **Példányok** kártyája a csoport tagjait listázza. Hogy ki mit csinál kinek, azt a Példányok oldal mutatja képen.

Bárki, aki ismeri a szavakat, csatlakozhat a csoporthoz, ezért kezeld őket jelszóként.

**Hogyan érik el egymást a tagok.** Minden példány a bejelentkezés pillanatában a böngésződből tudja meg a saját hálózati címét, amely a párosítási oldal **Ez a példány** kártyáján az **Ismert címek** alatt szerepel, azokkal együtt, amelyeket a példány maga talált. Javítsd ott, ha reverse proxy vagy szokatlan port áll előtte. Ugyanazon a hálózaton a tagok multicasttal hirdetik ezt a címet, és közvetlenül beszélnek, ahol pedig a multicast nem tud átmenni egy konténerhálózaton, például a Docker alapértelmezett bridge-én, a példány ehelyett a saját alhálózatán keresi a többit egy aláírt hívással, amelyre csak egy csoporttag tud válaszolni, így a párosítás relay nélkül is másodpercek alatt végbemegy. Ha semmi sem akad fenn, a **Nem találod?** egy kézzel megadott címet fogad, másik alhálózathoz vagy nem szokásos porthoz. A különböző hálózaton lévő példányok relayn át mennek, amelyet ugyanazon az oldalon, a **Relay** kártyán négy közül egy sorban választasz ki. Minden formájához egy-egy kép tartozik:

- **Projekt-relay** (az alapértelmezés): `parleyport.halleluja.design`, ugyanaz a relay, amelyet a KnightLoader is használ. Nincs mit beállítani.
- **Saját relay**: az Unraid Community Appsből a [**ParleyPort**](https://github.com/junkerderprovinz/parleyport) konténer (a **Beállítások, Alkalmazások** alatt is). Add meg a címét minden példányon, amelynek használnia kell.
- **BombVault-példány**: egy olyan példányod, amely kívülről már elérhető, és be van kapcsolva a **Szolgáljon relayként**. Ez a példány ezután a `/relay/connect` címen válaszol a saját címén, a már meglévő reverse proxy és tanúsítvány mögött, és csak a csoportodat engedi be. Add meg a címét minden példányon, amelynek használnia kell.
- **Nincs relay**: a tagok automatikusan megtalálják egymást ugyanazon a hálózaton, máshol nem.

**Mit lát a relay.** A négy forma mellett egy azonos nevű gomb ezt mutatja a kiválasztott formára. A tagok közötti minden hívást AES-256-GCM-mel zárnak le a tizenkét szóból származtatott kulccsal, és ez a kulcs soha nem hagyja el a példányaidat. A relay megtudja a kapcsolatokat csoportosító hash-t, hogy egy üzenet melyik példánynak szól, mekkora, és mikor halad át. A helyi hálózaton a közvetlen hívást ugyanígy lezárják és alá is írják, így semmi sem függ az önaláírt tanúsítványtól, amelyet egy példány kiszolgál. Azokat a hívásokat, amelyek valamit kiadnak (szerepválaszok, restic-jelszó, fogadói bejelentkezések, kulcsletétek), ráadásul az őket küldő tag egy kulcsával is aláírják, amelyet az első találkozáskor rögzítenek. Lásd: [Biztonsági modell](configuration.md#security-model).

**Mi halad át a csoporton.** A Példányok oldal védelmi állapota, egy szakasz azonnali ellenőrzésének kérése, a szerepkérések és válaszaik, célajánlatok, valamint az, amire egy fogadónak vagy lehívónak szüksége van, miután a szerepét engedélyezték: a másik példány tárolóhelyei és restic-jelszava. A mentési adatok soha. Azok továbbra is közvetlenül a restic-háttérrendszerekhez mennek. Az APP_KEY sem: a restic-jelszó az adott példány tárolóit nyitja meg, és semmi mást, sem a tárolt titkait, a munkameneteit, sem a helyreállítási kódjait.

**Párosítás előtti bejegyzések.** A flottatokennel hozzáadott példányok, valamint a másik példány APP_KEY-ével beállított fogadott tárolók és lehívási források a frissítés után is megmaradnak, és **Újra párosítás** jelölést kapnak. Tovább működnek: az első indításkor a BombVault minden tárolt APP_KEY-t lecserél a belőle származtatott restic-jelszóra. Párosítsd mindkét példányt, majd szerkeszd a bejegyzést, és válaszd ki a példányát. Addig az ilyen bejegyzések a Példányok oldal **Példány nélkül** kártyáján állnak.

Az egyetlen hely, amely még kézzel kér APP_KEY-t, a [Régi mentési mappából](#restore-from-another-bombvault-repo), arra az esetre, ha a másik példány eltűnt, és nem tud válaszolni egy csoportban.

## Példányok és szerepek {#roles}

A **Példányok** oldal (egy beállítási csempe, amelyet kitűzhetsz az oldalsávra) a csoportot hálóként mutatja, hogy ki mit csinál kinek, egy kártyával ehhez a példányhoz, és egyel minden más taghoz, az Android-telefonokhoz és a ZFS-kiszolgálókhoz. A jobb alul lévő **Párosítás** a párosítási oldalra vezet. Hogy mely részek vannak bekapcsolva, azt a **Beállítások, Példányok, Több szerver** alatt állítod: **Példányok**, **Fogadó** és **Lehívás**.

![A Példányok oldal: a csoport hálóként, és példányonként egy kártya a szerepcsempéivel, egyikük Inaktív.](assets/screenshots/offsite/instances-dark.png#only-dark)
![A Példányok oldal: a csoport hálóként, és példányonként egy kártya a szerepcsempéivel, egyikük Inaktív.](assets/screenshots/offsite/instances-light.png#only-light)

Minden kártya mutatja a tag védelmi állapotát és a szerepeit csempékként. A szerep olyasmi, amit egy példány egy másikért tesz:

- A **Fogadó** ennek a szervernek a telephelyen kívüli másolatait egy append-only módú rest-serverben tartja. Ez a szerver minden mentés után küld, és semmit nem tud ott törölni, akkor sem, ha valaki átveszi az irányítást fölötte.
- A **Lehívó** ennek a szervernek a mentéseit saját ütemezés szerint lehívja. Ehhez ennek a szervernek nem kell ébren lennie. A lehívó példánynak bekapcsolt **Lehívás** kell, mielőtt engedélyezheti a kérést.
- A **ZFS-kiszolgáló** fogadja ennek a szervernek a ZFS-replikáit. Minden ZFS-elem a beállításaiban a **Replika** alatt választja ki.
- A **Kulcsőrző** ennek a szervernek a helyreállítási csomagjáról őriz egy lezárt másolatot. Lásd: [Kulcsőrző](#key-keeper).

Egy szerep bekapcsolása kérést küld, és semmi sem indul el, amíg valaki a másik példányon meg nem nyomja az **Engedélyezés** gombot. A kérés a példány kártyáján jelvényként jelenik meg, a **Részletek** pedig felsorolja az **Engedélyezés** és **Elutasítás** gombbal. Egy engedélyezett szerep ott bármikor visszavonható. A hálóban szaggatott vonal a még várakozó szerep. A BombVault 9.x-es példány nem tud válaszolni egy kérésre, ezért ott a szerep abban a pillanatban aktív, ahogy bekapcsolod, a 9.x-es példánytól jövő kérés pedig itt vár az **Engedélyezés**re.

Egy kártya **Részletek** gombja megnyitja a többit: a tag fogadott tárolóit és lehívási forrásait, a célajánlatokat, a felügyeletet, az **Eltávolítás** és a **Kulcs elfelejtése** gombot.

Egy tag felajánlhatja a saját telephelyen kívüli tárolóját egy másik tagnak a csoporton át. A másik adminisztrátor az ajánlatot a tag **Részletek** ablakában látja, és elfogadja vagy elutasítja. Az elfogadás mentett bejelentkezést és egy átlagos mentési célt hoz létre, ugyanazokkal az ellenőrzésekkel, mint egy általad beírt célnál. A cél bekapcsolt törlés elleni védelemmel érkezik, mert az ajánlatban látható recept egy append-only rest-server, így a **Meddig őrizze meg** ott nem érvényes. Minden ajánlatnak saját bejelentkezése és saját mappája van, így ha egy szakaszt újra felajánlasz, ugyanannak a tagnak vagy egy másiknak, az első partner másolatai érintetlenek maradnak: add hozzá az új sort a `.htpasswd` fájlhoz, és hagyd futni a szervert. Csak kapcsolati adatok utaznak így, mentési adatok soha. Az **Ellenőrzés most** megkér egy tagot, hogy ellenőrizze egy szakasz tárolóját. Az oldalon semmi nem tud mentést, visszaállítást vagy törlést indítani egy másik gépen.

A leállított példányok és a bezárt alkalmazású telefonok **Inaktív** állapotban, szürkén megtartják a kártyájukat, azzal együtt, hogy mikor látták őket utoljára, és a hálóban halványítva állnak, amíg az **Eltávolítás** gombbal el nem távolítod őket. Közvetlenül ennek a szervernek az indulása után egy ilyen kártya **Ellenőrzés** állapotot mutat, amíg a csoport újra elérhető nem lesz. Az **Eltávolítás** egy ilyen kártyán attól függetlenül működik, hogy mely részek vannak bekapcsolva, és a tag nyitott kéréseit, kihasználatlan ZFS-fogadóhelyeit és várakozó kulcskéréseit is lezárja. A párosítási oldal **Példányok** kártyája ezeket a tagokat szintén **Inaktív** állapotban sorolja fel.

## Fogadó és lehívó {#receiver}

Azon a gépen, amely egy másik BombVaulttól append-only másolatokat **fogad**, a fogadott tárolók független, csak olvasható felügyeletet adnak a fogadó hardveren, így a túlsó végen történő csendes hiba nem marad észrevétlen. Kapcsold be a **Fogadó** opciót a **Beállítások, Példányok** alatt, majd nyisd meg a küldő példány **Részletek** gombját a Példányok oldalon, és add hozzá a fogadott tárolót (csak olvasható, a küldő példány restic-jelszavával nyitva, amelyet a [párosítási csoporton](#pairing) át kap meg). Ezt kapod:

- **Forrásonkénti leltár**, hogy pontosan lásd, mely konténerek, VM-ek és mappakészletek érkeztek meg. Azok a mentések, amelyeket a küldő azelőtt készített, hogy saját [hosztja](#backup-host) lett volna, ugyanahhoz a forráshoz számítanak.
- **Utoljára fogadva** forrásonként, hogy tudd, mindegyik mennyire friss.
- **Független `restic check`** a fogadó hardveren, hogy az integritást ott ellenőrizzék, ahol az adatok ténylegesen vannak, ne csak a küldőn.
- **Holtember-kapcsoló:** riasztás, ha egy forrás az általad megadott időablakon belül nem küld. Naponta egyszer néz rá, 09:15-kor. Egy riasztást akkor is kapsz, ha a tároló egyáltalán nem olvasható, függetlenül attól, mit mond az ellenőrzési ütemezés, és akkor is, ha a hozzáadása után egy teljes időablakon át semmi sem érkezett. Az a tároló, amelyet a küldő első másolata előtt adsz hozzá, addig rendben van.
- **Integritásriasztások:** riasztás, ha a fogadó oldali ellenőrzés meghiúsul. Az az ellenőrzés, amely nem tudta megnyitni a tárolót, semmit sem mond az integritásról, ezért nem hagy ítéletet, és másnap ugyanabban az időben újra próbálkozik.

Egy másik szerveren lévő tárolóhoz annak a szervernek a bejelentkezése kell: mentsd el [bejelentkezésként](#sign-ins), és válaszd ki a párbeszédablakban a **Hitelesítő adatok** alatt.

A fogadó szigorúan csak olvas. Soha nem ír a fogadott tárolóba, így soha nem törheti meg az append-only garanciát, amelyre a küldő támaszkodik.

A lehívó fordítva dolgozik: bekapcsolt **Lehívással** egy példány saját ütemezés szerint beolvassa egy másik tag mentéseit, forrásonként egy saját tárolóba, így ezek soha nem számítanak ennek a gépnek a saját mentéseinek. A forrásai szintén annak a tagnak a részleteiben állnak: a tároló helye, hogy milyen fajta mentést tart, és milyen gyakran kell lehívni. A restic-jelszó a csoporton át jön, az APP_KEY soha, a túloldalon nincs több teendő, és nem is kell futnia. A forrást csak olvassa a rendszer, és mindkét oldal megtartja a saját bejelentkezését. Egy lehívás azon a napon és időpontban fut, amelyet az ütemezése megad, és egy új forrás is erre az időpontra vár. A **Lehívás most** nem vár. Az a lehívás, amely azért maradt el, mert ez a gép ki volt kapcsolva, lefut, amint újra működik. A sikertelen lehívás másnap ugyanabban az időben újra próbálkozik, heti ütemezésnél is, a sikertelen lehívás pedig megjelenik a tevékenységnaplóban, és egy riasztást küld, amíg egy lehívás újra nem sikerül. Egy `rclone:` forrást elutasít, mert az rclone ennek a példánynak a remote-jaival érné el.

A lehívott másolat csak nő. A BombVault hozzáad, de nem nyesi és nem is ellenőrzi, így amit a forrás már rég elfelejtett, az itt megmarad, amíg nem törlöd a mappát, és a Célok oldal nem számolja bele. A szakasz saját mappáján belül a `pulled/<source id>` alatt van, és ha az a mappa költözik, a másolat ott marad, ahol van, és a későbbi lehívások ott folytatódnak, ahelyett hogy mindent újra másolnának. Ha törlöd, a következő lehívás elölről kezdi abban a saját mappában, amely a szakasznak addigra van.

### Fogadószerver {#receiving-server}

A fogadó gép azt a rest-servert is futtathatja, amelyre a többiek másolnak. Az első olyan **Fogadó** kérés engedélyezésekor állítódik be, amely ezt kéri: az ablak megmondja, hogy a szerverhez fogadószerver kell, és kér egy mappát egy megosztáson (az **Új mappa** gombbal újat hozhatsz létre), valamint egy portot (8000, hacsak egy másik konténer nem használja). A BombVault ezután:

1. visszautasítja, ha már létezik `rest-server` nevű konténer, vagy egy másik konténer foglalja a portot,
2. letölti a `restic/rest-server` képet, és a Docker-socketen át append-only módban, privát tárolókkal és a mappában lévő bejelentkezési fájllal elindítja,
3. kiírja az Unraid-sablonját a flash-meghajtóra, hogy a konténer szerkeszthető maradjon a Docker lapon, vagy letöltésként ajánlja fel a sablont, ha a flash-meghajtó elérhetetlen,
4. lefuttatja ellene a manipulációs tesztet, és megmutatja, megtagadja-e a törléseket.

A csoportod példányai ezután a szolgáltatók között **A csoportodból** alatt találják meg a szervert, a fogadó gép nevén, és a **Fogadó** szerep ezt kínálja biztonságos választásként. Minden példány saját bejelentkezést kap, amikor először használja a szervert, és ott csak a saját mappájába ír. Ha a **Fogadó** be van kapcsolva a **Beállítások, Példányok** alatt, a **Fogadószerver** ennek a példánynak a kártyáján újra megnyitja a szervert. Az ablaka a **Bejelentkezéssel rendelkező partnerek** alatt felsorolja a bejelentkezéseket, a **Bejelentkezés visszavonása** pedig elvesz egyet. Amit az a példány már átmásolt, a mappában marad. A beállítás létrehoz egy **Bejelentkezés a csoporton kívülre** is, amelynek jelszavát az ablak egyszer mutatja meg.

Az a példány, amely a fogadó gépet csak a relayn át éri el, nem használhatja a szervert, mert a relay nem szállít mentéseket. Előbb add hozzá a fogadó gép címét a párosítási oldalon. Ha a BombVault saját IP-címen fut (például br0-n), töltsd ki a **Cím a partnereknek** mezőt, mert a szerver a gazdagép címén hallgat.

## Példa: két Unraid-gép párosítása {#worked-example-two-unraid-boxes-end-to-end}

Ez a fenti részek egy teljes beállítása, mert a részeket könnyebb összerakni, ha egyszer láttad őket összerakva.

Két gép: a **TOWER** a konténereket futtatja és a mentéseket küldi. A **VAULT** fogadja őket, és érvényesíti az append-only módot. Helyettesítsd be a saját neveidet, címeidet és megosztási útvonalaidat.

**1. Párosítsd a két gépet.** A TOWER-en nyisd meg a **Beállítások, Párosítás** oldalt, nyomd meg a **Jelmondat létrehozása** gombot, és add meg a tizenkét szót a VAULT-on a **Jelmondat megadása** alatt. Előbb állíts be bejelentkezési jelszót mindkettőn.

**2. A TOWER-en kérd meg a VAULT-ot, hogy legyen a fogadó.** A Példányok oldalon kapcsold be a **Fogadó** szerepet a VAULT kártyáján, válaszd ki a szakaszokat, és helyként a **VAULT fogadószerverét**, majd küldd el a kérést.

**3. A VAULT-on engedélyezd.** A VAULT Példányok oldala mutatja a kérést a TOWER kártyáján. Nyomd meg az **Engedélyezés** gombot. Először a VAULT egy mappát kér a fogadószerverének, például `user/appdata/rest-server`, elindítja a rest-servert append-only módban, és lefuttatja ellene a manipulációs tesztet.

**4. Vissza a TOWER-en.** Ettől fogva a TOWER minden mentés után másol a VAULT-ra, és az új mentési cél megjelenik a TOWER Célok oldalán a törlés elleni védelmével.

Mit válaszol a manipulációs teszt:

| Eredmény | Mi történt |
| --- | --- |
| **védett** | A VAULT megtagadta a törlést. Ez az egyetlen megfelelt állapot. |
| **nem védett** | A VAULT elfogadott egy törlést. Hiányzik a `--append-only`, vagy eltávolították. |
| **nem egyértelmű** | Egyik sem. Általában az URL nem az, amelyet maga a restic használ, vagy megváltoztak a hitelesítő adatok. Semmi sem kerül rögzítésre, és nem indul riasztás. |

**5. Figyeld, mi érkezik, ha akarod.** A VAULT-on kapcsold be a **Fogadó** opciót a **Beállítások, Példányok** alatt, nyisd meg a TOWER **Részletek** gombját, és add hozzá a fogadott tárolót: kattints a párbeszédablak által felsorolt célok egyikére. A VAULT a csoporton át kapja meg a TOWER restic-jelszavát, és mivel a cél a VAULT saját fogadó szerverén van, közvetlenül olvassa a mappát. Senki sem gépel be kulcsot vagy bejelentkezést.

**6. Tedd kölcsönössé.** Ismételd a lépéseket az ellenkező irányban: a VAULT megkéri a TOWER-t, hogy legyen a fogadója. Ekkor mindkét gép érvényesíti az append-only módot a másiknak, és egyik sem törölheti a másik mentéseit.

Egy kézzel futtatott rest-server is működik: add hozzá a **Cél hozzáadása** gombbal rest-serverként, a `.htpasswd`-jében lévő felhasználóval és jelszóval, és kapcsold be a törlés elleni védelmet. Tartsd meg az `--append-only` kapcsolót a beállításaiban. Nélküle a túloldal megint egy átlagos megosztás.

!!! warning "A helyi útvonal a konténeren belüli útvonal"
    Ahol a BombVault egy mappát kér ezen a szerveren, a gazdagép csatolásához viszonyítva add meg, például `user/appdata/rest-server/bombvault-containers`, ne `/mnt/user/appdata/...` alakban. A BombVault konténerben fut, ahol a gazdagép `/mnt` könyvtára máshol van csatolva. Egy abszolút gazdagép-útvonal ott nem létezik. Ha beillesztesz egyet, a BombVault megmondja, melyik relatív útvonalat használd helyette.

## Vészvarázsló {#emergency}

A **Vészvarázsló** visszahoz egy elveszett vagy újra felállított szervert. A **Visszaállítás** oldalon indul a **Vészhelyzet** alatt, és egy új telepítés üdvözlőoldalán a **Visszaállítás mentésből** gombról. Négy lépésben fut: **Hozzáférés**, **Beállítások**, **Kijelölés** és **Visszaállítás**.

1. **Hozzáférés.** Mutasd meg a BombVaultnak a régi mentések kulcsát, háromféleképpen:
    - **Helyreállítási kit feltöltése**: a fájl, amelyet a beállításkor letöltöttél. Tartalmazza a kulcsot és az összes mentési célt, és a BombVault ebből tölti ki a többit. Az age-gel lezárt csomag elkéri azt az identitást, amelyre lezárták. Egyszer használják, és nem tárolják.
    - **A csoporton át**: a csoportod egy példánya letétként őrzi a kulcsot. Kell hozzá a csoport tizenkét szava, a feloldókódod és valaki annál a példánynál. Lásd: [Kulcsőrző](#key-keeper).
    - **Kulcs beírása**: a régi szerver `APP_KEY` értéke, és hogy hol vannak a mentései.

    A BombVault ezután kipróbálja a kulcsot minden mentési célon, és mindegyikről megmondja, hogy olvasható, nem található, másik kulccsal zárolt, nem elérhető, vagy még hiányzik hozzá a bejelentkezés. Még semmi sincs elmentve, és a kulcs korlátozott ideig a memóriában marad. A **Kulcs átvétele** azt éri el, hogy ez a szerver ezután ezzel a kulccsal írja a mentéseit. Az **Átvétel visszavonása** visszatér a saját kulcshoz, amíg az első mentés le nem futott. Az a szerver, amelynek már vannak saját mentései, megtagadja az átvételt, és a [Régi mentési mappából](#restore-from-another-bombvault-repo) opcióhoz irányít. A **Hozzáférési adatok mentése** eltárolja azokat a bejelentkezéseket, amelyeket a csomag a távoli mentési célokhoz hoz, és rákérdez, mielőtt felülírná azokat, amelyek már megvannak ezen a szerveren.
2. **Beállítások.** A **Beállítások visszahozása** a BombVault saját beállításait állítja vissza a legfrissebb Önmentésből: mentési célokat, bejelentkezéseket, ütemezéseket és szabályokat. A BombVault újraindul, hogy alkalmazza őket, a Docker-socketen át, így az éles beállítási adatbázist soha nem írják felül nyitott kezelő alatt, és a varázsló ezután magától folytatja. Ennek a szervernek a bejelentkezése megmarad: a mostani jelszavaddal jelentkezel be tovább. A régi szerveren tárolt, át nem vehető titkok fel vannak sorolva, hogy újra megadhasd őket. A visszaállítás lezár minden munkamenetet, és eltávolítja a hozzáférési kulcsokat, ezért utána állítsd be őket újra.
3. **Kijelölés.** A BombVault végignézi a mentéseidet, és felsorolja a talált konténereket, VM-eket, mappakészleteket és ZFS-adatkészleteket. A legfrissebb mentés mindenhol előre ki van jelölve. Ami ezen a szerveren még megvan, kikapcsolva indul, mert a visszaállítás lecserélné. A mappakészletek az újraépítés után már nem tudják, honnan jöttek, ezért egy általad választott mappába kerülnek.
4. **Visszaállítás.** Minden kijelölt egyszerre visszajön, megállítva hagyva, ha minden konténert és VM-et magad akarsz elindítani, ami teljes elvesztés után a legbiztonságosabb. Utána tölts le egy új helyreállítási csomagot: a régi az előző telepítéshez tartozik.

![A Vészvarázsló az első lépésénél, a három bejárattal.](assets/screenshots/offsite/emergency-dark.png#only-dark)
![A Vészvarázsló az első lépésénél, a három bejárattal.](assets/screenshots/offsite/emergency-light.png#only-light)

### Példa: új szerver a helyreállítási csomagból {#example-emergency}

A TOWER szerver a lemezeivel együtt elpusztult. 23 konténert és 2 VM-et tartott, és ezeket meg az Önmentését a Backblaze B2-re másolta. Megvan a `bombvault-recovery-kit.md` a jelszókezelődből, és van egy új gép Unraiddal. A régi `APP_KEY` elveszett, és nincs is rá szükséged.

1. **Telepítsd a BombVaultot** az új gépre, és adj neki új `APP_KEY` értéket, mint bármely új telepítésnél. Ha a VM-eket is vissza akarod kapni, állítsd be a VM-kapcsolatot a **Beállítások, Integrációk, Host SSH** alatt, mert a VM-visszaállításhoz kell.
2. **Indítsd el a varázslót.** Az **Üdv a BombVaultban** oldalon válaszd a **Visszaállítás mentésből** gombot. A Visszaállítás oldalon válaszd a **Vészvarázsló indítása** gombot a **Vészhelyzet** alatt.
3. **Hozzáférés.** Válaszd a **Helyreállítási kit feltöltése** lehetőséget, és dobd rá a fájlt. A BombVault kipróbálja a kulcsát minden mentési célon, és például ezt mondja: **A helyreállítási csomag beolvasva. A kulcsa 2 mentési célból 0-t nyit meg.** A helyi mappa azt mondja: **nem található**, mert a lemez elpusztult, a Backblaze B2 pedig azt: **hozzáférés hiányzik**.
4. **Hozzáférési adatok mentése.** A csomag minden cél bejelentkezését hordozza. Nyomd meg a **Hozzáférési adatok mentése** gombot, és a célokat újra kipróbálja, így a Backblaze B2 most azt mondja: **olvasható**. Egy 10.5 előtt írt csomag csak az alapértelmezett bejelentkezést tartalmazza. Ahol egy cél még mindig azt mondja: **hozzáférés hiányzik**, add meg a bejelentkezését a **Hiányzó hozzáférési adatok megadása** alatt, és nyomd meg a **Célok újrapróbálása** gombot.
5. **A kulcs átvétele.** Ez a szerver mostantól a régi kulccsal írja a mentéseit, így az új mentések illeszkednek a régiekhez. A sablonban megtartja a saját `APP_KEY` értékét. Az **Átvétel visszavonása** az első mentés lefutásáig működik.
6. **Beállítások.** A lépés azon a helyen nyílik meg, ahol a kulcs megtalálta az Önmentést, itt a B2-n, kitöltött címmel. Válaszd a **Beállítások visszahozása** gombot, amely onnan olvassa be a legfrissebb Önmentést. A BombVault újraindul, hogy alkalmazza, és a varázsló magától folytatja. A mentési céljaid, ütemezéseid és szabályaid visszajöttek, és tovább ennek a szervernek a jelszavával jelentkezel be. Add meg újra a titkokat, amelyeket a varázsló felsorol, például az értesítési beállításokat.
7. **Kijelölés.** A **Hol vannak a mentések?** a telephelyen kívüli másolatokon nyílik meg, ha helyben semmi sem maradt. A BombVault minden célt beolvas, így egy halott cél nem takarja el a többit, felsorolja a 23 konténert és a 2 VM-et, és mindegyiket abból a célból állítja vissza, amely a legfrissebb mentését tartja.
8. **Visszaállítás.** Kapcsold be az **Utána hagyd leállítva** kapcsolót, ami teljes elvesztés után a legbiztonságosabb, és indítsd el. A **Mi történik** megmondja, mi jön vissza. Az eredménysor például ezt írja: **25 visszaállítva, 0 sikertelen. Indítsd el őket a Biztonsági mentések alatt, ha szükséges.**
9. **Befejezés.** Előbb indítsd el az adatbázisaidat, utána az őket használó alkalmazásokat. Nyisd meg a **Beállítások, Visszaállítás** oldalt, és tölts le egy új helyreállítási csomagot, mert a régi az előző telepítéshez tartozik. Nézd meg a **Figyelmet igényel** kártyát az Áttekintésen, és ha minden működik, futtasd le egyszer az **Összes mentése most** műveletet.

Ha az Önmentés is hiányzik, a lépés a **Kihagyás: nincs Önmentésem** lehetőségen nyílik meg, és ezt jelzi. A mentési célok ilyenkor csak a csomagból jönnek, az ütemezéseket és szabályokat újra beállítod, a telephelyen kívüli másolatok pedig várnak, amíg át nem veszed a mentéseiket a szerver saját mentéseiként, lásd: [Újraépítés után](#after-a-rebuild). A **Kijelölés** továbbra is felsorolja, mit tartanak a telephelyen kívüli másolatok. A mappakészletek egy általad választott mappába kerülnek. A konténerek és a VM-ek sima fájlokként térnek vissza egy mappába a **Csak a fájlok állíthatók vissza** alatt, mert a definícióik a helyi tároló mellett és az Önmentésben vannak, egy telephelyen kívüli másolat pedig egyiket sem tartalmazza.

!!! tip "Tervezett migráció és katasztrófa"
    A Vészvarázsló a BombVault saját beállításait állítja vissza egy mentésből. Egy *tervezett* költözéshez egy új gépre ehelyett közvetlenül is átviheted a konfigurációdat a **Beállítások exportálása / importálása** kártyával (egy hordozható JSON-fájl). Lásd: [Konfiguráció](configuration.md#portable-settings-export-and-import).

### Régi mentési mappából {#restore-from-another-bombvault-repo}

A Visszaállítás oldalon a **Más források** alatti **Régi mentési mappából** egy *másik* BombVault-példány tárolóját (egy `/mnt` alatt csatolt megosztást vagy távoli címet) nyitja meg annak a példánynak a helyreállítási csomagjával vagy a beírt `APP_KEY` értékkel, egyszeri, csak olvasható munkamenetben. Böngészd az ott tárolt konténereket, VM-eket, mappakészleteket és ZFS-adatkészleteket, válassz egy mentést, és állítsd vissza, a visszaállított objektum pedig normál helyi konténerré, VM-mé vagy mappakészlette válik. A másik tárolóba soha nem írnak, és a saját mentési beállításaid érintetlenek maradnak (a munkamenet a memóriában él, és magától lejár). Egy konténer áthelyezése az A szerverről a B szerverre nem jelenti, hogy át kell irányítanod a beállításaidat, majd később vissza kell állítanod őket. Ez az ablak egyszeri: megnyit egy munkamenetet, visszaállítja, amit kiválasztasz, és elfelejti a másik példányt. Ha ehelyett tartós megoldást akarsz, ahol ez a gép ütemezve a saját tárolójába tölti le egy másik példány mentéseit, az a Példányok oldal **Lehívó** szerepe.

### Amit ez a szerver lehívott {#restore-what-was-fetched}

Minden forrásnak, amelyből ez a szerver lehívott, saját **Lekérve innen** sora van a Visszaállítás oldalon a **Más források** alatt. A lehívott másolat ennek a szervernek a saját kulcsával van lezárva, nem a forráséval, ezért semmit sem kér: a forrás kulcsa nem nyitja meg. A mappakészletek és a ZFS-adatkészletek egy általad választott mappába kerülnek. A konténerek és a VM-ek sima fájlokként térnek vissza egy mappába, mert a lehívás a pillanatképeket hozza, a definíciókat nem. Az újbóli létrehozásukhoz használd a Vészvarázslóban a forrás helyreállítási csomagját és Önmentését.

Az a konténer, amelynek a hálózata nem létezik ezen a szerveren, például egy Unraid `br0` hálózat egy sima Docker-gazdán, hálózatválasztót mutat a sora alatt. A BombVault a választott hálózaton hozza létre, a többi hálózatával együtt. A rögzített IP és MAC-cím a régi hálózathoz tartozott, ezért elhagyják, és az új hálózat osztja ki őket.

## Kulcsőrző {#key-keeper}

A kulcsőrző a csoportod egy példánya, amely ennek a szervernek a helyreállítási csomagjáról egy lezárt másolatot tart. A másolatot maga nem tudja megnyitni. Ha ez a szerver valaha elvész, egy új telepítés, amely csatlakozik a csoportodhoz, elkéri a másolatot az őrzőtől, és a feloldókóddal nyitja meg.

![A feloldókód ablaka: húsz karakter négy csoportban, egyszer mutatva.](assets/screenshots/offsite/recovery-code-dark.png#only-dark)
![A feloldókód ablaka: húsz karakter négy csoportban, egyszer mutatva.](assets/screenshots/offsite/recovery-code-light.png#only-light)

1. A Példányok oldalon kapcsold be a **Kulcsőrző** szerepet annak a példánynak a kártyáján, amelyik őrizze, és válaszd a **Letét létrehozása** gombot. Letét létrehozásához bejelentkezési jelszó kell.
2. Ez a szerver egyszer mutatja a **Feloldókódot**: húsz betű és számjegy négy ötös csoportban. Írd le, és erősítsd meg a **Felírtam a kódot** gombbal. Ez a szerver nem tárolja a kódot, csak azt, ami egy hozzá tartozó letét lezárásához kell.
3. Az őrző mostantól tartja a letétet, és ez a szerver megújítja, amikor a csomag változik. Ennek a példánynak a kártyáján a **Kulcsletét** felsorolja **Ennek a szervernek a letétje** alatt az egyes őrzőket és az állapotukat, a **Letétek, amelyeket ez a szerver őriz** alatt pedig a többiekét. Az **Új kód készítése** kicseréli a kódot. Minden őrző átáll az újra, amikor megkapja az új letétet.

Azon a napon három dolog kell: a csoportod tizenkét szava, a feloldókód, és hozzáférés az őrző felületéhez, ahol valakinek meg kell nyomnia az **Engedélyezés** gombot. A Vészvarázslóban válaszd **A csoporton át** opciót, válaszd ki a letétet, kérd el, és add meg a feloldókódot. A kódot az új szerveren használják, az őrzőnek nem küldik el. Az őrzőn a kérés a **Kulcsletét** alatt jelenik meg: ki kéri, látták-e valaha ott azt a példányt, és hányszor kért. Az **Engedélyezés** kiadja a lezárt másolatot, az engedélyezéshez pedig bejelentkezési jelszó kell. Az új telepítésnek az Engedélyezés után 30 perce van átvenni, a kérés, amelyre senki sem válaszol, egy nap után lejár, és az a példány, amely túl gyakran kér, egy órát vár.

Kód nélkül senki nem tudja megnyitni a másolatot, az őrző sem. Ha a kód elveszett, készíts újat, amíg ez a szerver még fut. Ha a szerver már eltűnt, a helyreállítási csomag és a beírt APP_KEY továbbra is működik. Az őrző bármikor törölhet egy letétet, és a tulajdonosnak ezután újra kérnie kell.

## Helyreállítási csomag {#recovery-kit}

Ez az a darab, amely akkor is lehetővé teszi a katasztrófa utáni helyreállítást, ha nincs futó BombVault.

A **Beállítások, Visszaállítás** alatti **Vészhelyzetre** kártyán a **Helyreállítási csomag letöltése** megadja a **kulcsot**, a **származtatott restic-jelszót**, minden **mentési célt** az egyes szakaszok tárolójával, azokat a bejelentkezéseket, amelyek egy új telepítésnek kellenek az eléréshez, és a **pontos restic-parancsokat**, így közvetlenül a restic CLI-vel visszaállíthatsz bármely gépen. Bejelentkezési jelszó kell hozzá. A csomag a kulcsot kikapcsolt titkosításnál is kiírja, mert a konténer- és VM-definíciókat mindig ezzel zárják le. Átvétel után kiírja az átvett mentési kulcsot és ennek a szervernek az APP_KEY-ét, amelyre egy új telepítésnek csak a beállítások visszahozásához van szüksége. Az Áttekintés **Mentsd el a helyreállítási csomagot** kártyája emlékeztet, amíg el nem tároltad.

!!! danger "A helyreállítási csomagot a szerveren kívül tárold"
    A csomag tartalmazza azt a titkot, amely visszafejti a mentéseidet. Tartsd biztos helyen, a szervertől elkülönítve (jelszókezelő, kinyomtatott példány egy széfben). Ha a BombVaultot és az `APP_KEY` értékét is elveszíted helyreállítási csomag és kulcsletét nélkül, a titkosított mentéseidet nem lehet visszaszerezni.

!!! warning "A legújabb pillanatkép nem mindig az, amit vissza kell állítani"
    A restic 0.17 óta a `restic snapshots` mutatja minden pillanatkép méretét. Adatvesztés után a legújabb pillanatkép lehet a kiürített, ezért ne állíts vissza olyan pillanatképet, amely jóval kisebb a korábbiaknál. Zsarolóvírus után lehet a titkosított, a szokásos méretben. Ha a BombVault még fut, előbb nézd meg az **Anomáliák** oldalát: megnevezi az utolsó jó mentést, és a visszaállítási ablak ezt jelöli ki előre. A visszaállításhoz nincs szükség a BombVault anomáliaadataira, és a megőrzési szünet mindig csak több pillanatképet tart meg.

### Egy szerver mentéseinek hosztja {#backup-host}

Minden szerver saját restic-hoszt alatt írja a mentéseit: `bv-` és tizenkét karakter, amelyet a mentések kulcsából számítanak ki, például `bv-3f9a0c41d2e7`. Azoknak a mentéseknek, amelyek még azelőtt készültek, hogy a szervernek saját hosztja lett volna, `bombvault` a hosztjuk. A csomag megnevezi ennek a szervernek a hosztját, a parancsai pedig mindkettőt átadják a `--host` kapcsolóval, így a `latest` soha nem választ olyan mentést, amelyet egy másik szerver írt ugyanabba a tárolóba. Az a szerver, amely átveszi egy másik mentéseit, annak a szervernek a hosztja alatt folytatja, mert a hoszt a kulcsból származik.

### A csomag lezárása {#sealing-the-kit}

Ha bekapcsoltad az **Exportok titkosítása age-dzsel** opciót a **Beállítások, Rendszer** alatt, a csomagot is ezzel zárják le, és `bombvault-recovery-kit.md.age` néven töltődik le. ASCII-páncélozott, nem bináris, tehát továbbra is egyszerű szöveg: jelszókezelőbe illesztése vagy kinyomtatása pontosan úgy működik, mint korábban, csak a tartalom olvashatatlan a kulcsod nélkül.

!!! warning "Ne tárold az age-kulcsot a csomagban"
    Egy lezárt csomag megnyitásához szükséged van az age **privát** kulcsodra. Olyan helyen tartsd, amely nem magától a csomagtól függ, különben egy helyett két dolgot kell majd helyreállítanod. A lezárás akkor éri meg, ha a csomag olyan helyen van, amely felett nincs teljes ellenőrzésed (megosztott jelszókezelő, felhős jegyzetek, egy kinyomtatott példány egy irodában). A saját széfedben lévő csomagot már a széf védi.

    Bekapcsolt titkosítás és beállított, használható címzett nélkül a letöltést a BombVault eleve elutasítja. Soha nem tér át arra, hogy a mesterkulcsot nyílt szövegként adja ki.

### Ha a csomag épp nincs kéznél {#if-you-do-not-have-the-kit-to-hand}

A jelszót semmi sem tárolja, a mentések kulcsából **számítják ki**, így csak a kulccsal és egy shell-lel te magad is előállíthatod:

```sh
printf 'bombvault:restic-repo'   | openssl dgst -sha256 -mac HMAC -macopt hexkey:$APP_KEY -r   | cut -d' ' -f1
```

Ez egy HMAC-SHA256 a rögzített `bombvault:restic-repo` karakterláncon, kulcsként a hexadecimális `APP_KEY` nyers bájtjaival, 64 kisbetűs hexadecimális karakterként kiírva. Ugyanez az érték szerepel a csomagban származtatott restic-jelszóként. Ez arra a napra való, amikor a csomag ott van, ahol te nem. Azon a szerveren, amely átvette egy másik mentéseit, használd az átvett mentési kulcsot, vagyis annak a szervernek a régi APP_KEY-ét.

A [hoszt](#backup-host) ugyanebből a kulcsból és a `bombvault:host` karakterláncból származik: `bv-` és ezeknek a karaktereknek az első tizenkettője.

```sh
echo "bv-$(printf 'bombvault:host' | openssl dgst -sha256 -mac HMAC -macopt hexkey:$APP_KEY -r | cut -c1-12)"
```

!!! warning "Fogadott tárolónál a KÜLDŐ példány kulcsát használd"
    Az a tároló, amely telephelyen kívüli másolattal érkezett ide, a küldő gépen jött létre, annak **saját** kulcsával. A fogadó gép kulcsából származtatva olyan jelszó adódik, amelyet a restic elutasít, és ez pontosan úgy néz ki, mint egy sérült tároló, de nem az. Ez a szokásos oka annak, hogy a `restic check` egy fogadott tárolón újra meg újra jelszót kér.

Mivel a helyreállítási definíciók minden tárolón **belül** élnek (`<repo>/def`, `<repo>/vm-def`), egy másolt tárolómappa teljesen önálló, így a csomag plusz a tároló minden, amire egy bare-metal visszaállításnak szüksége van. A telephelyen kívüli másolat és a lehívott másolat a pillanatképeket a definíciók nélkül tartalmazza: fájlokat adnak vissza, a konténert vagy VM-et pedig az Önmentésből lehet újra létrehozni.

## Adatbázis-mentés visszaszerzése {#database-dumps}

Az adatbázis-dump a konténerek tárolójában külön visszaállítási pont `dbdump:<container>` címkével, amely az egyetlen `/dbdump/<container>.sql` fájlt tartalmazza. A BombVault a konténer **Biztonsági mentések** lapján listázza, tölti le és importálja őket. Lent ugyanezek a lépések csak a resticcel, arra a napra, amikor a BombVault nincs ott.

```sh
restic -r <repo> snapshots --host <host> --host bombvault --tag dbdump:<container>
restic -r <repo> dump --host <host> --host bombvault --tag dbdump:<container> latest /dbdump/<container>.sql > <container>.sql
```

A `<host>` a szerver [hosztja](#backup-host) a helyreállítási csomagból.

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
