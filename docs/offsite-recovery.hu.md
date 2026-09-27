# Telephelyen kívüli mentés és helyreállítás

A helyi mentések megvédenek egy elveszett konténertől vagy egy rossz frissítéstől. A telephelyen kívüli replikáció és egy tesztelt helyreállítási csomag megvéd a teljes gép elvesztésétől, a zsarolóvírustól vagy egy tűztől. Ez az oldal a telephelyen kívüli replikálást, a másolat manipulációbiztossá tételét, a visszaállíthatóság bizonyítását és a helyreállítást ismerteti arra az esetre, amikor maga a BombVault eltűnt.

## Telephelyen kívüli replikáció

Tartsd meg a gyors helyi mentést, és másold egy vagy több másik tárhelyre. Azt, hogy egy tartomány mely tárhelyekre másolódik, a **Beállítások, Tárolás** alatti **Tartományok** kártyán választod ki, tárhelyenként egy chippel (lásd: [Tárhelyek](storage-places.md#domains)). A BombVault az új pillanatképeket `restic copy` segítségével, legjobb szándék szerint másolja oda, így egy sikertelen másolás soha nem hiúsítja meg a helyi mentést. Annak a tárhelynek, amelyen egy tartomány tárolódik, nem kell helyinek lennie; lásd: [Távoli tárhelyen tárolt tartomány](#remote-primary-repositories).

- **Több másolási tárhely tartományonként.** Egy tartomány egyszerre több tárhelyre is másolható, például egy rest-serverre egy barátod házában és egy B2-bucketbe. A megőrzés, a tárolási osztály, az append-only, a korlátok és a növekedési keret a tárhelyhez tartozik, így minden másolat annak a tárhelynek a szabályait követi, ahová kerül.
- **Tartományonkénti másolási ütemezés** (minden más ütemezés mellett a Beállítások, Ütemezések alatt szerkesztve): hagyd üresen, hogy minden helyi mentés után másoljon, vagy állíts be egy ütemet (például `weekly Sun 03:00`), hogy ritkábban másoljon, mint amilyen gyakran mentesz. A tartomány sorában lévő **Másolás most** igény szerint lefuttatja.
- **Megőrzés tárhelyenként.** Minden tárhelynek saját szabályai vannak, így egy telephelyen kívüli tárhely archívumként tovább megtarthatja a másolatokat. Az a tárhely, amelynek minden szabálya nulla, soha nem vág vissza semmit.
- **A sávszélesség-korlátok** tárhelyenként korlátozzák a restic fel- és letöltési sebességét, hogy a másolás ne telítse a WAN-odat.
- Egy **replikációs jelző** mutatja, melyik tartomány másol éppen, amíg fut (a saját oldalán és az irányítópulton). Ez egy aktív jelző, nem egy százalékos sáv, mert a `restic copy` nem tesz közzé géppel olvasható folyamatjelzést.

!!! note "Visszaállítás bármely helyről"
    Minden konténer, VM, fájlkészlet, a flash és az alkalmazás-konfiguráció egyetlen idővonalként sorolja fel a mentéseit minden hely között, ahol egy mentés fekszik. Egy B2-be másolt mentés egyszer jelenik meg, minden azt tartalmazó hellyel megjelölve. A visszaállítás az első elérhető helyet veszi, a tárolóval kezdve, ahova az elem íródik, és soronként másik helyet is választhatsz. A telephelyen kívüli helyek csak akkor olvasódnak, amikor megnyitod őket. Az egy helyen történő törlés előbb a többit ellenőrzi, és megmondja, hogy az volt-e az utolsó másolat.

## Elhelyezés elemenként {#placement}

Minden konténer-, VM- és fájlkészlet-kártyának van egy **Elhelyezés** sora három szegmenssel:

- **Helyi** a **Tárolva itt** alatt látható tárolóba írja az elemet, és sehova nem másolja. Olyan adathoz használd, amelynek már van egy második másolata, például egy NAS-on élő megosztáshoz.
- **Helyi + telephelyen kívüli** ott is megírja, és a **Másolás ide** alatt kipipált célokra másolja, egy chip a tartomány minden telephelyen kívüli céljára. Vedd ki egy chip pipáját, és az a cél semmi újat nem kap ettől az elemtől.
- **Csak telephelyen kívüli** egyenesen a **Küldés ide** alatti tárhelyre írja az elemet, amely a tartomány otthonán kívül bármely tárhely lehet. Ahol a tartomány már másolódik arra a tárhelyre, az elem egy közvetlen tárolót kap a másolatok mellett; különben a BombVault létrehoz ott egy tárolót a tartomány számára.

A hely az elem első biztonsági mentésétől fogva rögzített, mert a BombVault soha nem mozgat mentéseket tárolók között. A másolatok bármikor változhatnak. Az a cél, amely már nem kap egy elemet, megtartja a meglévő másolatait, és a tartomány következő telephelyen kívüli futásakor a saját megőrzésére vágja őket vissza; a kártyán a **Törlés itt: B2** azonnal eltávolítja őket. Ha ezek közül a másolatok közül néhány sehol máshol nem létezik, a megerősítés dátum szerint felsorolja őket, és kéri az elem nevét. A csak hozzáfűzésre szolgáló célokból nem lehet törölni.

A sor alatt a kártya megmondja, hova kerül az elem, és mi van ott ténylegesen: hány helyszín tartja, mikor látták utoljára az egyes célokat, és teljesül-e a 3-2-1. Egy helyszín az eredeti adatokat tartalmazó szerver és minden másik helyszínen lévő tárhely (lásd: [Az épületen kívül](#off-the-premises-mark)). A BombVault a másolatokat és a helyszíneket ellenőrzi; a 3-2-1 "két adathordozó" részét nem ellenőrzi.

### Alapértelmezések tartományonként

A Beállítások, Tárolás alatti **Tartományok** kártyán tartományonként egy sor van. A **Másolva ide** azonnal érvényes minden saját választás nélküli elemre, és a Compose-stackek projektmappáira is. Ha egy tartománynak már vannak mentései, a **Tárolva itt** egy új elemre az első mentésekor válik érvényessé, és a megváltoztatása egyetlen mentést sem mozgat. Mentés előtt a sor megnevez minden tárhelyet, amely elemeket nyer vagy veszít, és hogy ez hány pillanatképet jelent, a kérdésben pedig ott van az **Alkalmazás a biztonsági mentés nélküli elemekre** kapcsoló, amely minden még mentés nélküli elemet is az új alapértelmezésre állít. A **Kivételek** a saját választással rendelkező elemeket sorolja fel.

Ha egy új tárhelyet bepipálsz a **Másolva ide** alatt, az megkap minden elemet, amely nincs Helyire állítva. A megerősítés megmondja, hány elemről van szó, és ahol ismert, mennyi előzményt jelent ez.

### Közvetlen tárolók

Ha a Csak telephelyen kívüli alatt olyan tárhelyet választasz, ahová a tartomány már másolódik, a BombVault egyszer rákérdez, majd létrehoz egy közvetlen tárolót a másolatok mellett, például `s3:https://s3.eu-central-003.backblazeb2.com/bucket/container-direct`, és odairányítja az elemet. Tárhely nélküli másolási célnál a választás egy párbeszédablakot nyit meg egy javasolt címmel és egy kapcsolatteszttel, amely semmit nem hoz létre, és a **Létrehozás és használat** létrehozza a tárolót. A közvetlen tároló átveszi a tárhely kulcsát, tárolási osztályát, korlátait, append-only beállítását és megőrzését, és velük együtt változik. Ha a tárhely új kulcsa nem tudja megnyitni, a közvetlen tároló megtartja azt a kulcsot, amije van, és a mentés ezt jelzi. A pillanatképei a `bv:direct` címkét viselik, és minden más megőrzési kör megtartja őket, így egy közvetlen tároló, amely elvesztette a kapcsolatát a tárhelyével, soha nem öregszik a helyi szabályok szerint. Egy egyetlen mappára korlátozott B2-kulcsnak a tárhely címét kell lefednie, nem csak a tartomány mappáját, különben a mellette lévő mappa elérhetetlen.

### Az épületen kívül {#off-the-premises-mark}

Egy másolat csak akkor számít külön helyszínnek, ha a tárhelye másik helyszínen van. Egy felhőtárhely mindig számít, egy mappa ezen az Unraidon soha; NAS-nál, rest-servernél vagy SFTP-szervernél a tárhely részleteiben válaszolj a **Hol van az eszköz?** kérdésre az **Itt, az épületben** vagy a **Másik helyszínen** lehetőséggel. A válasz csak a helyszíneket és a 3-2-1-et számolja a kártyákon és az irányítópulton. Egyetlen másolatot sem változtat meg.

### Újraépítés után

A másolási választások a BombVault saját beállításaiban élnek. Egy Felfedezésen keresztüli újraépítés után, visszaállított `/config` nélkül, ezek eltűnnek, és minden másolása újra elküldené a B2-be azokat az elemeket, amelyeket kihagytál. Ezért minden újraépített tartomány telephelyen kívüli replikációja szünetel. Az irányítópult sárgán mutatja, és a tartomány sora a Tartományok kártyán felajánlja az **Alapértelmezés megerősítése** lehetőséget, egy előnézettel arról, mit másol a következő futás, és azokkal a nevekkel a mentésekben, amelyeknek nincs bejegyzésük, amelyeket ott kihagyhatsz. Csak a megerősítés zárja le a szünetet; egy beállításfájl importálása visszahozza a szabályokat és az alapértelmezéseket, de nem zárja le.

## Távoli tárhelyen tárolt tartomány {#remote-primary-repositories}

Egy tartományt nem kell helyben tárolni. Amíg a mentési helyén nincs mentés, válassz egy távoli tárhelyet a **Tárolva itt** alatt a Tartományok kártyán, és a tartomány egyenesen oda ment, helyi másolat és másolási lépés nélkül. A távoli tároló ekkor az egyetlen példány, hacsak a tartomány nem másolódik egy másik tárhelyre is. Minden távoli tárhely ugyanazokkal a biztosítékokkal jár:

- **Kapcsolatteszt**, mielőtt bármi íródna.
- **Sávszélesség-korlátok** magára a mentésre, ugyanazokkal a `--limit-upload` és `--limit-download` kapcsolókkal, amelyeket egy másolás használ.
- **Append-only védelem**, ugyanazzal az aktív manipulációs teszttel ellenőrizve. Bekapcsolva a BombVault soha nem nyesi a tárolót, mert az ezen a gépen lévő hitelesítő adatok nem lehetnek képesek törölni a mentés egyetlen példányát.
- **Növekedési keret**, ugyanabból a méret-trendből mintavételezve, amelyet a Tárolás kártya követ.

Egy távoli tárhelyen tárolt tartomány ugyanúgy a másolatainak forrása, mint egy helyi; lásd: [Másolatok eltérő hitelesítő adatú tárhelyek között](storage-places.md#different-credentials).

!!! note "A hitelesítő adatok a tárhelyhez tartoznak"
    Egy távoli tárhely saját hitelesítő adatokat tart. Az a tárhely, amelyet a közös felhő-hitelesítő adatokkal állítottak be, addig használja azokat, amíg a hozzáférését meg nem változtatod a részleteiben.

## Módosíthatatlan (append-only) telephelyen kívüli

Jelölj egy telephelyen kívüli tárolót append-only-ként, hogy a zsarolóvírus vagy egy feltört hoszt ne tudja törölni vagy átírni a mentéseidet. A túloldal (egy `restic/rest-server` `--append-only` módban futva) **érvényesíti**. A BombVault csak **ellenőrzi**, és soha nem mutat zöldet pusztán egy konfigurációs állítás alapján.

A **Tárhely hozzáadása** ablak egy beilleszthető receptet tartalmaz egy append-only módú rest-serverhez, egy felhasználóval ennek a BombVaultnak. Egy rest-server tárhelyen, amelyen az **Append-only** be van kapcsolva, a tárhely részleteiben az **Append-only tesztelése** lefuttatja a manipulációs tesztet a tárhely minden tartományútvonalán, bekapcsolt másolatán és tárolóján, és egyetlen választ ad a tárhelyre, így az append-only telephelyen kívüli mentés elérhető a konfigok kézi szerkesztése nélkül.

!!! note "A `/locks/` alatti sikeres törlés várt viselkedés"
    Az append-only nem azt jelenti, hogy semmit sem lehet többé törölni. A resticnek fel kell vennie és el kell engednie a saját zárait, ezért a `/locks/` szándékosan írható és törölhető marad. A pillanatfelvételek és a mögöttük lévő adatok, vagyis pontosan az, amit egy zsarolóvírus célba venne, nem távolíthatók el. Ha magad próbálod ki a túloldalt, a `/locks/` alatt sikeres törlés helyes viselkedés, nem rés a védelemben.

!!! warning "A módosíthatatlan tárolók soha nem nyesődnek erről a gépről"
    Egy módosíthatatlan telephelyen kívüli tároló szándékosan soha nem nyesi a régi pillanatképeket. Állíts be egy **növekedési keret riasztást** hozzá, hogy értesülj, mielőtt a tárolóméret elszabadulna.

## Manipulációs teszt

A BombVault időnként bizonyítja az append-only garanciát azzal, hogy ténylegesen megkísérel egy törlést a telephelyen kívüli tároló ellen, egy nem létező objektumra célozva:

- Az **elutasítás** azt jelenti, hogy védett.
- Az **elfogadás** azt jelenti, hogy nem védett.
- Egy **nem meggyőző** eredmény (a szerver elérhetetlen, hitelesítési hiba) soha nem billenti át a tárolt ítéletet.

Egy valódi védett-védtelen átbillenés egyetlen riasztást indít.

Egy tárhelyen az **Append-only tesztelése** minden ott lévő tartományútvonalat, bekapcsolt másolatot és tárolót a saját hitelesítő adataival próbál meg, és az eredményeket egyetlen válaszba vonja össze: egyetlen tároló, amely elfogad egy törlést, az egész tárhelyet *törlések elfogadva* állapotba teszi.

## DR-próbák

A BombVault kétféle szintű bizonyítékot kínál arra, hogy a mentéseid ténylegesen visszaállíthatók, nem csak jelen vannak.

- **Visszaállítás-ellenőrző próbák (helyi).** A BombVault időnként lefuttatja a `restic check --read-data-subset` parancsot (korlátozva, soha nem egy lemezt megtöltő teljes visszaállítás), és tartományonként *utoljára visszaállíthatónak igazolva* jelvényt mutat. Az ütem a Beállítások, Ütemezések alatt él; a jelvény a Beállítások, Integritás alatt.
- **DR-próbák (telephelyen kívüli).** A BombVault visszaállít egy valódi célt a telephelyen kívüli tárolóból egy eldobható homokozóba, ellenőrzi fájlról fájlra és bájtról bájtra, majd feltakarít. Ez bizonyítja, hogy telephelyen kívülről helyre tudsz állni, nem csak azt, hogy a tároló válaszol. Csak a másik helyszínen lévő tárhelyeken fut próba, mert egy ugyanabban az épületben lévő másolat semmit sem bizonyít az épület elvesztéséről. Egy több ilyen tárhelyre másolt tartomány ütemezett futásonként sorra egyiken próbálódik, és az irányítópult megnevezi az utolsó próba tárhelyét.

A **zsarolóvírus-védelmi eredménytábla** az irányítópulton mindezt tartományonkénti zöld / sárga / piros helyzetté gyűjti össze, egy korral bélyegzett ellenőrzőlistával (telephelyen kívüli beállítva, append-only igazolva, replikáció naprakész, visszaállítási próba sikeres, titkosítás be, nyesési stratégia beállítva). Minden piros sor mélyhivatkozással a javításra mutat, és a kártya csak igazolt tényeken vált valaha is zöldre.

## Fogadó irányítópult (a fogadó oldal)

![A fogadó oldal, csak olvasható módon figyelve, integritás-ellenőrzéssel ezen a gépen.](assets/screenshots/receiver.png)

*A fogadó oldal, csak olvasható módon figyelve, integritás-ellenőrzéssel ezen a gépen.*

Minden fenti a *küldő* oldal. Azon a gépen, amely módosíthatatlan telephelyen kívüli másolatokat **fogad** egy másik BombVaulttól, a Fogadó irányítópult független, csak olvasható monitorozást ad azokról a tárolókról a fogadó hardveren, így egy csendes hiba a túlsó végen nem marad észrevétlen.

Kapcsold be a **Fogadó** kapcsolót a Beállításokban egy **Fogadó** fül felfedéséhez. Alapból ki van kapcsolva; csak olyan gépen engedélyezd, amely ténylegesen fogad módosíthatatlan telephelyen kívüli mentéseket. Ezután regisztrálj egy fogadott tárolót (csak olvasható, a küldő példány kulcsával megnyitva), hogy megkapd:

- **Egy forrás szerint csoportosított pillanatkép-leltárt**, így pontosan látod, mely konténerek, VM-ek és fájlkészletek landoltak.
- **Utoljára fogadva** forrásonként, így tudod, mindegyik mennyire friss.
- **Egy független `restic check`-et**, amely a fogadó hardveren fut, így az integritás ott ellenőrződik, ahol az adat ténylegesen ül, nem csak a küldőn.
- **Egy holtemberkapcsolót:** riasztás, amikor egy forrás abbahagyja a küldést egy általad beállított időablakon belül.
- **Integritási riasztásokat:** riasztás, amikor egy ellenőrzés a fogadó oldalon meghiúsul.

A Fogadó szigorúan csak olvasható. Soha nem ír a fogadott tárolóba, így soha nem tudja megtörni az append-only garanciát, amelyre a küldő támaszkodik.

## Végigvezetett példa: két Unraid gép, elejétől a végéig

Fent az alkatrészek szerepelnek. Itt egy teljes összeállítás valódi értékekkel, mert az alkatrészeket könnyebb összerakni, ha az ember egyszer már látta őket összerakva.

Két gép: a **TOWER** futtatja a konténereket és küldi a mentéseket, a **VAULT** fogadja őket és kikényszeríti a változtathatatlanságot. Cseréld a saját neveidre, címeidre és megosztási útvonalaidra.

**1. A VAULT gépen állítsd fel az append-only kiszolgálót.** A TOWER BombVaultjában nyisd meg a *Beállítások → Tárolás* oldalt, kattints a **Tárhely hozzáadása** gombra, válaszd a **rest-server** lehetőséget, és kattints a **Recept megjelenítése** gombra. Másold ki az **Unraid-sablon** blokkot, mentsd a VAULT gépen `/boot/config/plugins/dockerMan/templates-user/my-rest-server.xml` néven, majd *Docker → Add Container*, és válaszd a **rest-server** elemet a sablonlistából. Indítás előtt írd be a megjelenített `htpasswd` sort a VAULT gépen a `/mnt/user/appdata/rest-server/.htpasswd` fájlba. A jelszó csak egyszer jelenik meg, és sosem kerül tárolásra; a recept már beírta a jelszót és a felhasználót az űrlapba a TOWER gépen, ezért hagyd nyitva azt az ablakot. A `htpasswd` sor ugyanazt a jelszót hordozza, már bcrypttel kivonatolva, így neked semmit sem kell kivonatolnod.

    Hagyd bent a `--append-only` kapcsolót az OPTIONS mezőben. Nélküle a VAULT megint csak egy hétköznapi megosztás.

**2. A TOWER gépen add hozzá a tárhelyet.** Írd be a VAULT címét, `http://VAULT:8000`, a recept által kitöltött felhasználó és jelszó mellé, majd kattints a **Kapcsolat tesztelése** gombra. A BombVault ezekből állítja össze a címet:

    rest:http://VAULT:8000/tower

Az útvonal első szakasza a htpasswd felhasználó, itt `tower`, és minden tartomány alatta kapja a saját mappáját, például `rest:http://VAULT:8000/tower/container`. A **Hol van az eszköz?** kérdésre válaszolj a **Másik helyszínen** lehetőséggel, kattints a **Hozzáadás** gombra, és pipáld be a tárhelyet a **Másolva ide** alatt azoknál a tartományoknál, amelyeknek oda kell kerülniük.

**3. A TOWER gépen kapcsold be az Append-only módot** a tárhely részleteiben a **Védelem** alatt, majd kattints az **Append-only tesztelése** gombra. A teszt a tárhely minden tartományútvonalát, másolatát és tárolóját megpróbálja, és egyetlen választ ad a tárhelyre, amelynek *törlések elutasítva* eredménynek kell lennie. Mit jelentenek a válaszok:

| Eredmény | Mi történt |
| --- | --- |
| **törlések elutasítva** | A VAULT elutasította a törlést. Ez az egyetlen megfelelő állapot. |
| **törlések elfogadva** | A VAULT elfogadott egy törlést. Hiányzik a `--append-only`, vagy eltávolították. |
| eredmény helyett egy üzenet | A teszt nem tudott lefutni. Általában a cím nem az, amit maga a restic használ, vagy megváltoztak a hitelesítő adatok. Semmi nem kerül rögzítésre, és nem indul riasztás. |

**4. A VAULT gépen nézd meg, mi érkezik.** Kapcsold be a *Beállítások → Fogadó* pontot, nyisd meg a **Fogadó** fület, és regisztráld a tárolót csak olvasható módon.

!!! warning "A hely a konténeren **belüli** útvonal, a gazdagép csatolási pontjához képest megadva"
    Ezt add meg: `user/appdata/rest-server/tower/container`, és **ne** ezt: `/mnt/user/appdata/…`. A BombVault konténerben fut, ahol a gazdagép `/mnt` könyvtára máshová van csatolva; abszolút gazdagép-útvonal ott nem létezik. Ha mégis beilleszted, a BombVault megmondja a helyette használandó relatív útvonalat.

    A **küldő APP_KEY** a TOWER kulcsa, nem a VAULT-é. A TOWER gépen a *Beállítások → Rendszer* alatt találod.

**5. Ha akarod, tedd kölcsönössé.** Ismételd meg ugyanazt az öt lépést a másik irányban: egy rest-server a TOWER gépen, amely a VAULT másolatát fogadja. Ekkor mindkét gép kikényszeríti a másik változtathatatlanságát, és egyik sem tudja törölni a másik mentéseit.

## Vezetett helyreállítás

Egy dedikált **Helyreállítás** fül egy helyen végigvezet egy friss vagy újraépített telepítést a katasztrófaeseten:

1. **Ellenőrzi, hogy a BombVault olvasni tudja-e a mentéseidet** (a titkosításikulcs-buktató előre).
2. **Visszaállítja a BombVault saját beállításait**, így a mentési útvonalak, telephelyen kívüli célok és hitelesítő adatok, amelyekre a folyamat többi része szüksége van, előre kitöltve jelennek meg. A beállítás-mentést abból a tárhelyből olvassa, amelyet az Önmentés sora a **Tárolva itt** alatt megnevez, vagy az Önmentés **Másolva ide** alatti másolatából, és a tárhelyet a címével együtt mutatja; ha másik tárhelyről olvasnál, előbb módosítsd az Önmentés sorát a 3. lépésben. A visszaállítás a Docker socketen keresztüli önújraindítással érvényesül, így az élő beállítás-adatbázis soha nem íródik felül nyitott handle alatt.
3. **Csatolja a meglévő mentéseidet** a Tartományok kártya sorain keresztül: minden tartomány sorában válaszd ki a **Tárolva itt** alatt azt a tárhelyet, ahol a mentései vannak, a **Másolva ide** alatt pedig azokat a tárhelyeket, amelyek a másolatait őrzik. Egy tárhelyet, amelyet még egyik sor sem kínál, például egy megosztást, szervert vagy felhős bucketet, a **Tárhely hozzáadása** ablakkal csatlakoztatsz, ugyanazzal, mint a Beállítások, Tárolás alatt. A **Csatlakozás és előnézet** ezután ellenőrzi, hogy a mentések olvashatók-e.
4. **Felfedezi** a benne tárolt konténereket, VM-eket, fájlkészleteket és ZFS-adatkészleteket.
5. **A konténereket és a VM-eket egyszerre visszaállítja** (leállítva hagyva, így te indítod el őket szándékosan), a fájlkészleteket és a ZFS-elemeket pedig felsorolja, hogy egyenként állítsd vissza őket; a ZFS-elemek kikapcsolva térnek vissza. A helyreállítási csomagod egy kattintásnyira van.

!!! note "A telephelyen kívüli másolatok várnak egy újraépítés után"
    Amikor a 4. lépés a régi beállítások nélkül épít újra bejegyzéseket, azoknak a tartományoknak a telephelyen kívüli replikációja szünetel, amíg az elhelyezési alapértelmezést meg nem erősítik. Lásd: [Elhelyezés elemenként](#placement).

!!! tip "Tervezett migráció versus katasztrófa"
    A vezetett helyreállítás egy mentésből állítja vissza a BombVault saját beállításait. Egy *tervezett* átköltözéshez egy új gépre ehelyett közvetlenül átviheted a konfigurációdat az **Exportálás és importálás beállítások** kártyával (egy hordozható JSON-fájl). Lásd: [Konfiguráció](configuration.md#portable-settings-export-and-import).

### Visszaállítás egy másik BombVault tárolóból

Egy külön kártya a **Helyreállítás** fülön megnyit egy *másik* BombVault-példány tárolóját (egy a `/mnt` alá csatolt megosztás vagy egy távoli URL) **annak a példánynak az `APP_KEY`-ével**, egy egyszeri, csak olvasható munkamenetben. Böngészd az ott tárolt konténereket, VM-eket és fájlkészleteket, válassz egy pillanatképet és állítsd vissza, és a visszaállított objektum normál helyi konténerré, VM-mé vagy fájlkészletté válik. Semmi sem íródik soha a másik tárolóba, és a saját mentési beállításaid érintetlenek maradnak (a munkamenet a memóriában él és magától lejár). Egy konténer áthelyezése az A szerverről a B szerverre többé nem jelenti a tárolóbeállításaid átirányítását és utólagos visszaállítását. Az élő szerver-szerver federáció kifejezetten hatókörön kívüli; ez egy szándékos, egyszeri áthúzás.

## Titkosításikulcs-helyreállító csomag

Ez az a darab, amely a vészhelyreállítást akkor is lehetővé teszi, amikor nincs futó BombVault.

Egy kattintás letölti a **mesterkulcsot**, a **származtatott restic jelszót**, valamint a **pontos tárolóhelyeket és parancsokat**, így közvetlenül a restic CLI-vel állíthatsz vissza bármely gépen. Egy irányítópult-emlékeztető nyaggat, amíg el nem tárolod.

!!! danger "Tárold a helyreállítási csomagot a szerveren kívül"
    A csomag azt a titkot tartalmazza, amely visszafejti a mentéseidet. Tartsd biztonságos, a szervertől elkülönített helyen (egy jelszókezelő, egy nyomtatott példány egy széfben). Ha elveszíted a BombVaultot és az `APP_KEY`-t is, helyreállítási csomag nélkül, a titkosított mentéseid nem állíthatók helyre.

!!! warning "Nem mindig a legújabb pillanatképet kell visszaállítani"
    A restic 0.17 óta a `restic snapshots` minden pillanatkép méretét mutatja. Adatvesztés után a legújabb pillanatkép lehet a kiürített, ezért ne állíts vissza olyan pillanatképet, amely sokkal kisebb az előzőeknél. Zsarolóvírus után lehet a titkosított, szokásos méretben. Ha a BombVault még fut, előbb nézd meg az **Anomáliák** oldalát: megnevezi az utolsó jó mentést. A visszaállításhoz nincs szükség a BombVault anomáliaadataira, és a megőrzés szüneteltetése mindig csak több pillanatképet tart meg.

### Ha a csomag épp nincs kéznél

A jelszó sehol nincs tárolva, az `APP_KEY` értékéből **számolódik**. A kulccsal és egy shellel tehát magad is előállíthatod:

```sh
printf 'bombvault:restic-repo' \
  | openssl dgst -sha256 -mac HMAC -macopt hexkey:$APP_KEY -r \
  | cut -d' ' -f1
```

Ez HMAC-SHA256 a rögzített `bombvault:restic-repo` karakterlánc fölött, kulcsként a hexadecimális `APP_KEY` nyers bájtjaival, kimenetként 64 kisbetűs hexadecimális karakter. Ugyanez az érték szerepel a csomagban származtatott restic jelszóként; ez a szakasz arra a napra való, amikor a csomag máshol van, mint te.

!!! warning "Fogadott tárolónál a KÜLDŐ példány kulcsát használd"
    Az a tároló, amely külső telephelyi replikációval érkezett ide, a küldő gépen jött létre, annak **saját** `APP_KEY` kulcsával. A fogadó gép kulcsából származtatva olyan jelszót kapsz, amelyet a restic elutasít, és ez pontosan úgy néz ki, mint egy sérült tároló, holott nem az. Ez a szokásos oka annak, hogy a `restic check` egy fogadott tárolón újra és újra jelszót kér.

Mivel a helyreállítási definíciók minden tárolón **belül** élnek (`<repo>/def`, `<repo>/vm-def`), egy másolt tárolómappa teljesen önálló, így a csomag plusz a tároló minden, amire egy bare-metal visszaállításnak szüksége van.

## Adatbázis-dump visszaszerzése {#database-dumps}

Egy adatbázis-dump önálló visszaállítási pont a konténerek tárolójában, `dbdump:<container>` címkével és egyetlen fájllal, `/dbdump/<container>.sql`. A BombVault a **Mentések** alatt listázza, letölti és importálja őket; alább ugyanezek a lépések pusztán a restickel, arra a napra, amikor a BombVault nincs kéznél.

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
