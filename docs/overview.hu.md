# Áttekintés

Az áttekintés az oldalsáv első oldala, és ezt érdemes minden reggel megnézni. Két kérdésre ad választ egy pillantásra: minden le van-e mentve, és kell-e valamit tenned. Egy állapotfejjel és négy kártyával indul, a további kártyák a **Testreszabás** alatt várnak.

![Az áttekintés néhány nap után: az állapotfej, a Figyelmet igényel egy figyelmeztetéssel, az egyes szakaszok védelme, a célok és az előzmények.](assets/screenshots/overview/home-dark.png#only-dark)
![Az áttekintés néhány nap után: az állapotfej, a Figyelmet igényel egy figyelmeztetéssel, az egyes szakaszok védelme, a célok és az előzmények.](assets/screenshots/overview/home-light.png#only-light)

## Az állapotfej {#status-head}

A fej az oldal tetején áll, és mindig ott marad. A gyűrűje azt mutatja, mennyi van védve, például **41 / 43 védve**, a mellette lévő sor pedig elmondja, hogyan állnak a dolgok:

| A fej ezt mondja | Mit jelent |
|---|---|
| **Minden mentve van** | Minden bekapcsolt szakasz minden elemének van friss mentése. |
| **Mentve, megjegyzésekkel** | Minden le van mentve, és néhány futás megjegyzést hagyott maga után, például fájlokról, amelyeket nem tudott elolvasni. |
| **Még nincs minden védve** | Néhány elemnek nincs friss mentése, például mert még az elsőre vár. |
| **3 dolog figyelmet igényel** | Valami nem sikerült, vagy döntésre vár. A **Figyelmet igényel** kártya felsorolja. |
| **Mentés folyamatban** | Egy mentés fut. A gyűrű folyamatjelző gyűrűvé változik a **Leállítás**, **Olvasás** és **Indítás** állomásokkal, a teljes mentés pedig menet közben számolja az elemeit, például **Mentés folyamatban: 12 / 43**. A **Futás megtekintése** megnyitja a futást. |
| **Telephelyen kívüli másolás folyamatban** | Egy célra másolás fut. Százalékot nem mutat, mert a `restic copy` nem jelent ilyet. |

A sor alatt az áll, mikor volt az utolsó futás, és mikor indul a következő tervezett.

## Figyelmet igényel {#needs-you}

Ez a kártya azt sorolja fel, amire rá kell nézned, a legújabbal kezdve, és **Nincs teendő.** áll rajta, ha a lista üres. Minden sornak megvan a saját kiútja:

- Egy **sikertelen mentés** megnevezi az elemet és az okát. A **Hibák megtekintése** a restic saját kimenetét mutatja, az **Újrapróbálás** pedig újra elindítja a mentést.
- A **telephelyen kívüli másolás sikertelen** megnevezi a szakaszt és azt, mióta tart. A **Replikálás most** újra próbálkozik.
- Az **Anomália itt: plex** a leletre vezet az **Ellenőrzés** gombbal. Amíg egy kritikus lelet nyitva van, a megőrzés megtartja az elem régi mentéseit.
- Az **Új** elemeknél, amelyeket a BombVault megtalált, de még semmi nem menti őket, például egy frissen telepített konténernél, a **Mentés megkezdése** vagy a **Ne legyen mentés** választható.
- Egy **mentés, amely feladta**, mert az alkalmazása a teljes várakozás alatt foglalt maradt, lásd: [Ha az alkalmazás foglalt](backups.md#app-busy).
- Egy **sikertelen ZFS-replika**, egyszerű szavakkal, például hogy a szerveren nincs zfs parancs, a szerver kimenetével a **Megtekintés** alatt.
- Ez jelenik meg: **A mentések állnak: a kulcs már nem nyitható meg**, ha ez a szerver átvette egy másik szerver kulcsát, és az `APP_KEY` utána megváltozott. Lásd: [Hibaelhárítás](troubleshooting.md#backups-stopped-and-say-the-key-can-no-longer-be-opened).
- A **Mentsd el a helyreállítási csomagot** addig marad, amíg le nem töltötted a csomagot, és meg nem nyomtad a **Biztonságosan eltároltam** gombot.

## Védelmi állapot {#protection-status}

Minden bekapcsolt szakaszhoz tartozik egy sor négy ellenőrzéssel:

| Ellenőrzés | Mikor lesz zöld | Más szavak, amelyeket láthatsz |
|---|---|---|
| **Helyi** | az utolsó mentés friss | még nincs mentés, nincs ütemezés, az utolsó futás sikertelen, a mentés késik, nem minden elem védett, a mentés hamarosan esedékes |
| **Telephelyen kívüli** | a telephelyen kívüli utolsó másolat friss | nincs telephelyen kívüli másolat, másolás fut, a másolat késik, a döntésedre vár, még nincs másolat, a replika sikertelen |
| **Törlés elleni védelem** | egy manipulációs teszt igazolta, hogy a túloldal visszautasítja a törlést | nincs, nincs és a kockázat elfogadva, még nem bizonyított, a teszt sikertelen, nem bizonyított, mert az ellenőrzése ki van kapcsolva |
| **Próba-visszaállítás** | a telephelyen kívüli másolatból a visszaállítás sikerült | soha nem futott, fut, sikertelen, késik, csak kézzel, ennél a szakasznál nem lehetséges |

Egy szakasz csak ellenőrzött tények alapján zöld. Az ezen a szerveren lévő másolat nem számít telephelyen kívülinek, és az a cél, amelyet **Elfogadás törlés elleni védelem nélkül** jelöléssel láttál el, borostyánsárga pont helyett szürke gyűrűt mutat, így a szakasz színe a többi ellenőrzését követi.

Egy ütemezett szakasz a **Még nincs biztonsági mentés** feliratot mutatja, és semleges marad, amíg az első futása esedékessé nem válik, attól számítva, hogy a tervét utoljára beállították. Ezután a **Lejárt** feliratot mutatja, és egyszer riaszt. A **Lejárt** ahhoz az elemhez igazodik, amelyik a legjobban lemaradt a saját tervéhez képest, így egy futó szomszéd nem takarja el azt, amelyik leállt. Az az elem, amelynek a konténere eltűnt, vagy amelyet egyetlen ütemezés sem ér el, nem számít. A saját heti tervvel rendelkező elemet nem mérik a szakasz napi tervéhez. Az a szakasz, amelyben nincs mit menteni, mert egyetlen ütemezés sem éri el egyik elemét sem, a **Nincs ütemezve** feliratot mutatja, és sosem jár le.

A szakaszra kattintva megnyílik az ablaka az összes ellenőrzéssel szavakba öntve, a beleszámított elemekkel és a **Próba-visszaállítás indítása** vagy **Próba-visszaállítás újraindítása** gombbal. Azt is kiírja, hogy **Nyesési szabály beállítva** vagy **Nincs nyesési szabály**, és a szakasz saját mappájának megőrzési szabálya ugyanúgy számít, mint a telephelyen kívüli szabályai.

![A Konténerek szakasz védelmi ablaka a négy kiírt ellenőrzéssel.](assets/screenshots/overview/protection-window-dark.png#only-dark)
![A Konténerek szakasz védelmi ablaka a négy kiírt ellenőrzéssel.](assets/screenshots/overview/protection-window-light.png#only-light)

## Célok {#storage-locations}

Minden cél itt egy edényként áll a töltöttségi szintjével, azzal, milyen gyorsan nő, például **Heti 4,2 GB növekedés**, és hogy ezen az ütemen mikor telne meg, például **tele kb. 31 hét múlva**. Az S3, a B2 és a rest-szerverek nem jelentik, mennyi hely maradt, ezért **Szabad hely ismeretlen** látszik náluk, és csak a növekedésük. Egy kattintás a cél oldalára visz. Lásd: [Célok](offsite-recovery.md#destinations).

## Előzmények {#history}

Az **Előzmények** két lapból áll. A **Futások** minden futást felsorol az eredményével, időtartamával és okával, és egy kattintás megnyitja. A **Napló** minden sort az idejével mutat, úgy, ahogy a BombVault írja, és egyetlen napra szűkíthető. Egy sor narancssárga lesz, ha egy mentés lefutott, de kihagyott valamit, és a sor megnevezi, mit hagyott ki.

## További kártyák a Testreszabás alatt {#customize}

Az oldal tetején lévő **Testreszabás** lehetővé teszi a kártyák elrendezését. Húzd a kártyát a sávjánál fogva az áthelyezéshez, méretezd át a jobb alsó sarkában lévő pontnál, vagy rejtsd el. Az elrejtett kártyák a **Rejtett kártyák** alatt várnak, és a **Megjelenítés** visszahoz egyet. Amíg rendezed őket, a kártyák kicsit ringatóznak, és a **Kész** befejezi ezt. Az elrendezés ebben a böngészőben van elmentve.

Ezek a kártyák rejtve indulnak:

- **Anomáliák**: a nyitott leletek, lásd: [Anomáliák](anomalies.md).
- **Nincs mentve**: minden, ami ezen a szerveren van, és amit egyetlen ütemezett mentés sem fed le, mindegyik az okával, például **soha nem lett hozzáadva**, **nincs benne az ütemezésben** vagy **a szakaszának minden célja ki van kapcsolva**. A konténereket magából a Dockerből olvassa, így azt is megtalálja, amelyet a BombVault még soha nem látott.
- **Naptár**: az utolsó 90 nap, naponta egy négyzet. Egy napra kattintva a naplósorai láthatók az **Előzmények** alatt.
- **Tárhely**: mennyit foglal el az egyes szakaszok adata, és mennyire jól deduplikálódik.
- **Áttekintés**: a legrégebbi pillanatkép az összes szakaszból, mikor fut a következő mentés, és hogyan sikerült az utolsó.
- **Statisztika**: hány konténer és VM van, hány hiba, és hány terv aktív vagy szünetel.
- **Hosztintegráció**: mit ér el és mire van szüksége a BombVaultnak a hoszton, például a Docker-socketet, a libvirtet, a restic és az rclone. Itt kezdd, ha valami nincs bekötve.

![Testreszabás mód: a kártyák húzhatók, átméretezhetők és elrejthetők, az elrejtettek lent várnak.](assets/screenshots/overview/customize-dark.png#only-dark)
![Testreszabás mód: a kártyák húzhatók, átméretezhetők és elrejthetők, az elrejtettek lent várnak.](assets/screenshots/overview/customize-light.png#only-light)

## Hogyan számol a BombVault {#counting}

Minden kártya és a fej ugyanúgy számol: a bekapcsolt szakaszok azon elemeit, amelyek még telepítve vannak, és amelyeket nem jelöltél **Ne legyen mentés** állapotúra. Egy elem csak addig számít védettnek, amíg egy ütemezés eléri, és van mentése. Egy régi mentés önmagában nem számít, így az ütemezésből kivett konténer nem védettként jelenik meg, amíg vissza nem teszed, vagy **Ne legyen mentés** jelölést nem kap.

## Első lépések {#first-steps}

Új telepítésnél az áttekintés az **Első lépések** kártyával indul: **Válaszd ki a szakaszokat**, **Cél hozzáadása** és **Első mentés**. Addig marad, amíg az első mentés be nem fejeződött. A [Kezdő lépések](getting-started.md#first-run) végigvezet rajta.
