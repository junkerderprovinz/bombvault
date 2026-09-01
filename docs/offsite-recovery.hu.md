# Telephelyen kívüli mentés és helyreállítás

A helyi mentések megvédenek egy elveszett konténertől vagy egy rossz frissítéstől. A telephelyen kívüli replikáció és egy tesztelt helyreállítási csomag megvéd a teljes gép elvesztésétől, a zsarolóvírustól vagy egy tűztől. Ez az oldal a telephelyen kívüli replikálást, a másolat manipulációbiztossá tételét, a visszaállíthatóság bizonyítását és a helyreállítást ismerteti arra az esetre, amikor maga a BombVault eltűnt.

## Telephelyen kívüli replikáció

Tartsd meg a gyors helyi mentést, és adj hozzá egy vagy több telephelyen kívüli replikát. Állíts be egy tárolót tartományonként a **Beállítások, Telephelyen kívüli** fülön. A BombVault az új pillanatképeket `restic copy` segítségével, legjobb szándék szerint replikálja oda, így egy telephelyen kívüli zökkenő soha nem hibáztatja el a helyi mentést. A helyi tároló marad az elsődleges.

- **Több telephelyen kívüli cél tartományonként.** Minden tartomány (konténerek, VM-ek, flash, config és fájlkészletek) egyszerre több telephelyen kívüli célra is replikálhat, nem csak egyre, így párhuzamosan tarthatsz például egy rest-servert egy barátod gépén és egy S3-bucketet is. Adj hozzá további célokat a Beállítások, Telephelyen kívüli alatt, mindegyiket saját tárolóval, S3-tárolási osztállyal, append-only jelzővel, megőrzéssel és növekedési kerettel. Egy meglévő egyetlen telephelyen kívüli beállítás érintetlenül, az első célként öröklődik át, és egy tartomány minden célja az adott tartomány telephelyen kívüli ütemezése szerint replikál.
- **Tartományonkénti telephelyen kívüli ütemezés** (minden más ütemezés mellett a Beállítások, Ütemezések alatt szerkesztve): hagyd üresen, hogy minden helyi mentés után replikáljon, vagy állíts be egy ütemet (például `weekly Sun 03:00`), hogy ritkábban szállítson telephelyen kívülre, mint amilyen gyakran helyben mentesz. Egy **Replikálás most** gomb fedi le az igény szerinti futásokat.
- **A telephelyen kívüli megőrzés** a Beállítások, Telephelyen kívüli alatt él, így a telephelyen kívüli másolatokat archívumként tovább megtarthatod. Hagyd a szabályt mind nullán, hogy soha ne nyesse automatikusan a telephelyen kívüli pillanatképeket.
- **A sávszélesség-korlátok** (Beállítások, Telephelyen kívüli) korlátozzák a restic fel- és letöltési sebességét, hogy a replikáció ne telítse a WAN-odat.
- Egy **replikációs jelző** mutatja, melyik tartomány replikál éppen, amíg fut (a saját oldalán és az irányítópulton). Ez egy aktív jelző, nem egy százalékos sáv, mert a `restic copy` nem tesz közzé géppel olvasható folyamatjelzést.

!!! note "Visszaállítás közvetlenül telephelyen kívülről"
    Minden mentésböngészőben van egy **Helyi / Telephelyen kívüli** kapcsoló, így ha egy helyi tároló elveszik vagy megsérül, közvetlenül a telephelyen kívüli replikából listázhatsz és állíthatsz vissza. A törlés forrásonkénti: egy mentés eltávolítása csak azt a másolatot érinti, amelyet éppen nézel.

## Távoli elsődleges tárolók {#remote-primary-repositories}

Egy tartomány mentési útvonala (Beállítások, Útvonalak és tárolás) nem korlátozódik helyi mappára: irányítsd egyenesen egy restic távoli tárolóra (`s3:...`, `rest:http://host:8000/repo`, `b2:...`, `sftp:felhasznalo@host:/repo`, `rclone:remote:bucket/utvonal`), és a BombVault közvetlenül oda ment, külön helyi másolat és replikációs lépés nélkül. Ez valóban más alak, mint a fenti külső telephelyi replikáció: ott a helyi tároló az elsődleges, a külső pedig annak legjobb tudás szerinti archívuma; itt a távoli tároló **maga** az elsődleges, és ez az egyetlen példány, amíg az adott tartományhoz nem állítasz be külső telephelyi replikációt is (vagy egy második távoli tárolót).

Az öt útvonalmező (Konténerek, Virtuális gépek, Flash, Konfiguráció, Fájlok) mindegyike mellett közvetlenül ott áll egy **Helyi / Távoli** kapcsoló:

- **Helyi** a megszokott mappaböngészőt mutatja.
- **Távoli** ezt egy egyszerű URL-mezőre cseréli, plusz egy gombra, amely ugyanazt a kapcsolatteszt és hitelesítőadat párbeszédet nyitja meg, amit a külső telephelyi célok használnak, csak épp ehhez az elsődleges tárolóhoz beállítva. Onnan a következőket kapod:
    - **Kapcsolattesztet** a valódi útvonal ellen, mielőtt rábíznád magad.
    - **Sávszélesség-korlátokat** (fel- és letöltés), hogy egy ütemezett mentés a távoli elsődleges tárolóba ne telítse a WAN-vonaladat: ugyanazok a restic kapcsolók, `--limit-upload` és `--limit-download`, amiket a külső telephelyi replikáció használ, most magára a mentésre alkalmazva.
    - **Append-only védelmet (változtathatatlanság)**, ugyanazzal az aktív manipulációs teszttel ellenőrizve (valódi DELETE próba a túloldal ellen), amit a külső telephelyi célok kapnak. Bekapcsolva a BombVault megtagadja a tároló saját nyesését: mivel mögötte nincs külön helyi másolat, az ezen a gépen lévő hitelesítő adatok nem lehetnek képesek törölni a mentés egyetlen példányát.
    - **Növekedési keret riasztást**, ugyanabból a tárolóméret-trendből számolva, amit a Tárolás kártya amúgy is követ.

Ezek közül semmi sem kötelező: egy kézzel beírt távoli útvonal mentett biztonsági beállítások nélkül pontosan úgy ment, ahogy eddig (korlátlan sávszélesség, nyesehető, nincs keretriasztás). A biztonsági párbeszéd arra az esetre van, amikor ugyanazt a védelmet szeretnéd, amit egy külső telephelyi másolat kap, anélkül hogy pusztán ezért külön külső célt kellene létrehoznod.

!!! note "A felhő- és REST-hitelesítő adatok közösek"
    Egy távoli elsődleges tároló ugyanazokkal az S3/REST hitelesítő adatokkal azonosít, amelyek a Beállítások, Külső telephely, Felhő hitelesítő adatok alatt vannak beállítva. Az elsődleges tárolóknak nincs külön hitelesítőadat-tárolójuk.

## Módosíthatatlan (append-only) telephelyen kívüli

Jelölj egy telephelyen kívüli tárolót append-only-ként, hogy a zsarolóvírus vagy egy feltört hoszt ne tudja törölni vagy átírni a mentéseidet. A túloldal (egy `restic/rest-server` `--append-only` módban futva) **érvényesíti**. A BombVault csak **ellenőrzi**, és soha nem mutat zöldet pusztán egy konfigurációs állítás alapján.

A **vezetett telephelyen kívüli beállítás** varázsló végigvezet a backend választásától (rest-server / rclone / S3) egészen egy beilleszthető rest-server telepítési kódrészletig, egy kapcsolattesztig, a módosíthatatlan kapcsolóig (amely azonnal lefuttatja a manipulációs tesztet) és egy megőrzési stratégiáig, így az append-only telephelyen kívüli mentés elérhető a konfigok kézi szerkesztése nélkül.

!!! warning "A módosíthatatlan tárolók soha nem nyesődnek erről a gépről"
    Egy módosíthatatlan telephelyen kívüli tároló szándékosan soha nem nyesi a régi pillanatképeket. Állíts be egy **növekedési keret riasztást** hozzá, hogy értesülj, mielőtt a tárolóméret elszabadulna.

## Manipulációs teszt

A BombVault időnként bizonyítja az append-only garanciát azzal, hogy ténylegesen megkísérel egy törlést a telephelyen kívüli tároló ellen, egy nem létező objektumra célozva:

- Az **elutasítás** azt jelenti, hogy védett.
- Az **elfogadás** azt jelenti, hogy nem védett.
- Egy **nem meggyőző** eredmény (a szerver elérhetetlen, hitelesítési hiba) soha nem billenti át a tárolt ítéletet.

Egy valódi védett-védtelen átbillenés egyetlen riasztást indít.

## DR-próbák

A BombVault kétféle szintű bizonyítékot kínál arra, hogy a mentéseid ténylegesen visszaállíthatók, nem csak jelen vannak.

- **Visszaállítás-ellenőrző próbák (helyi).** A BombVault időnként lefuttatja a `restic check --read-data-subset` parancsot (korlátozva, soha nem egy lemezt megtöltő teljes visszaállítás), és tartományonként *utoljára visszaállíthatónak igazolva* jelvényt mutat. Az ütem a Beállítások, Ütemezések alatt él; a jelvény a Beállítások, Integritás alatt.
- **DR-próbák (telephelyen kívüli).** A BombVault visszaállít egy valódi célt a telephelyen kívüli tárolóból egy eldobható homokozóba, ellenőrzi fájlról fájlra és bájtról bájtra, majd feltakarít. Ez bizonyítja, hogy telephelyen kívülről helyre tudsz állni, nem csak azt, hogy a tároló válaszol.

A **zsarolóvírus-védelmi eredménytábla** az irányítópulton mindezt tartományonkénti zöld / sárga / piros helyzetté gyűjti össze, egy korral bélyegzett ellenőrzőlistával (telephelyen kívüli beállítva, append-only igazolva, replikáció naprakész, visszaállítási próba sikeres, titkosítás be, nyesési stratégia beállítva). Minden piros sor mélyhivatkozással a javításra mutat, és a kártya csak igazolt tényeken vált valaha is zöldre.

## Fogadó irányítópult (a fogadó oldal)

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

**1. A VAULT gépen állítsd fel az append-only kiszolgálót.** A TOWER BombVaultjában menj a *Beállítások → Külső telephely → vezetett beállítás* pontra, válaszd a **rest-server** lehetőséget, és készítsd el a receptet. Másold ki az **Unraid sablon (XML)** fület, mentsd a VAULT gépen `/boot/config/plugins/dockerMan/templates-user/my-rest-server.xml` néven, majd *Docker → Add Container*, és válaszd a **rest-server** elemet a sablonlistából. Indítás előtt írd be a megjelenített `htpasswd` sort a VAULT gépen a `/mnt/user/appdata/rest-server/.htpasswd` fájlba. Az egyszer használatos jelszó egyszer jelenik meg és sosem kerül tárolásra, másold ki most.

    Hagyd bent a `--append-only` kapcsolót az OPTIONS mezőben. Ez az egésznek a lényege: nélküle a VAULT megint csak egy hétköznapi megosztás.

**2. A TOWER gépen irányítsd oda a külső tárolót.** A tároló címe azt a mintát követi, amit a recept kiír:

    rest:http://VAULT:8000/bombvault-containers/containers

Az útvonal első szakasza a htpasswd felhasználó, a második a tároló. Add meg a generált felhasználót és jelszót a cél REST hitelesítő adataiként, majd futtasd a **kapcsolatellenőrzést**.

**3. A TOWER gépen kapcsold be a „Változtathatatlan” beállítást.** A manipulációs teszt azonnal lefut, és *védett* eredményt kell adnia. Mit jelentenek a válaszok:

| Eredmény | Mi történt |
| --- | --- |
| **védett** | A VAULT elutasította a törlést. Ez az egyetlen megfelelő állapot. |
| **NEM védett** | A VAULT elfogadott egy törlést. Hiányzik a `--append-only`, vagy eltávolították. |
| **nem egyértelmű** | Egyik sem. Általában a cím nem az, amit maga a restic használ, vagy megváltoztak a hitelesítő adatok. Semmi nem kerül rögzítésre, és nem indul riasztás. |

**4. A VAULT gépen nézd meg, mi érkezik.** Kapcsold be a *Beállítások → Fogadó* pontot, nyisd meg a **Fogadó** fület, és regisztráld a tárolót csak olvasható módon.

!!! warning "A hely a konténeren **belüli** útvonal, a gazdagép csatolási pontjához képest megadva"
    Ezt add meg: `user/appdata/rest-server/bombvault-containers/containers`, és **ne** ezt: `/mnt/user/appdata/…`. A BombVault konténerben fut, ahol a gazdagép `/mnt` könyvtára máshová van csatolva; abszolút gazdagép-útvonal ott nem létezik. Ha mégis beilleszted, a BombVault mostantól megmondja a helyette használandó relatív útvonalat.

    A **küldő APP_KEY** a TOWER kulcsa, nem a VAULT-é. A TOWER gépen a *Beállítások → Rendszer* alatt találod.

**5. Ha akarod, tedd kölcsönössé.** Ismételd meg ugyanazt az öt lépést a másik irányban: egy rest-server a TOWER gépen, amely a VAULT másolatát fogadja. Ekkor mindkét gép kikényszeríti a másik változtathatatlanságát, és egyik sem tudja törölni a másik mentéseit.

## Vezetett helyreállítás

Egy dedikált **Helyreállítás** fül egy helyen végigvezet egy friss vagy újraépített telepítést a katasztrófaeseten:

1. **Először visszaállítja a BombVault saját beállításait**, így a mentési útvonalak, telephelyen kívüli célok és hitelesítő adatok, amelyekre a folyamat többi része szüksége van, előre kitöltve jelennek meg (a Docker socketen keresztüli önújraindítással alkalmazva, így az élő beállítás-adatbázis soha nem íródik felül nyitott handle alatt).
2. **Ellenőrzi, hogy a BombVault olvasni tudja-e a mentéseidet** (a titkosításikulcs-buktató előre).
3. Lehetővé teszi, hogy **rámutass a meglévő tárolódra** (helyi vagy telephelyen kívüli).
4. **Felfedezi** a benne tárolt konténereket, VM-eket és fájlkészleteket.
5. **Mindet visszaállítja** (leállítva hagyva, így te indítod el őket szándékosan), a helyreállítási csomagoddal egy kattintásnyira.

!!! tip "Tervezett migráció versus katasztrófa"
    A vezetett helyreállítás egy mentésből állítja vissza a BombVault saját beállításait. Egy *tervezett* átköltözéshez egy új gépre ehelyett közvetlenül átviheted a konfigurációdat az **Exportálás és importálás beállítások** kártyával (egy hordozható JSON-fájl). Lásd: [Konfiguráció](configuration.md#portable-settings-export-and-import).

### Visszaállítás egy másik BombVault tárolóból

Egy külön kártya a **Helyreállítás** fülön megnyit egy *másik* BombVault-példány tárolóját (egy a `/mnt` alá csatolt megosztás vagy egy távoli URL) **annak a példánynak az `APP_KEY`-ével**, egy egyszeri, csak olvasható munkamenetben. Böngészd az ott tárolt konténereket, VM-eket és fájlkészleteket, válassz egy pillanatképet és állítsd vissza, és a visszaállított objektum normál helyi konténerré, VM-mé vagy fájlkészletté válik. Semmi sem íródik soha a másik tárolóba, és a saját mentési beállításaid érintetlenek maradnak (a munkamenet a memóriában él és magától lejár). Egy konténer áthelyezése az A szerverről a B szerverre többé nem jelenti a tárolóbeállításaid átirányítását és utólagos visszaállítását. Az élő szerver-szerver federáció kifejezetten hatókörön kívüli; ez egy szándékos, egyszeri áthúzás.

## Titkosításikulcs-helyreállító csomag

Ez az a darab, amely a vészhelyreállítást akkor is lehetővé teszi, amikor nincs futó BombVault.

Egy kattintás letölti a **mesterkulcsot**, a **származtatott restic jelszót**, valamint a **pontos tárolóhelyeket és parancsokat**, így közvetlenül a restic CLI-vel állíthatsz vissza bármely gépen. Egy irányítópult-emlékeztető nyaggat, amíg el nem tárolod.

!!! danger "Tárold a helyreállítási csomagot a szerveren kívül"
    A csomag azt a titkot tartalmazza, amely visszafejti a mentéseidet. Tartsd biztonságos, a szervertől elkülönített helyen (egy jelszókezelő, egy nyomtatott példány egy széfben). Ha elveszíted a BombVaultot és az `APP_KEY`-t is, helyreállítási csomag nélkül, a titkosított mentéseid nem állíthatók helyre.

Mivel a helyreállítási definíciók minden tárolón **belül** élnek (`<repo>/def`, `<repo>/vm-def`), egy másolt tárolómappa teljesen önálló, így a csomag plusz a tároló minden, amire egy bare-metal visszaállításnak szüksége van.
