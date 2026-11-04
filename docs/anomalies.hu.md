# Anomáliák

Egy mentésben a zsarolóvírus és egy elromlott alkalmazás hasonlóan néz ki: hirtelen az adatok nagy része új, vagy nagy része eltűnt. A BombVault minden mentést összevet az adott elem saját előzményeivel, és leletet jelez, ha valami nem stimmel. Amíg egy adatvesztésre utaló lelet nyitva van, az elem régi mentéseit is megtartja, így az utolsó jó mentés nem évülhet el. Az **Anomáliák** menüpont az oldalsávban akkor látszik, ha a felismerés be van kapcsolva.

![Az Anomáliák oldal egy nyitott kritikus lelettel.](assets/screenshots/anomalies/page-dark.png#only-dark)
![Az Anomáliák oldal egy nyitott kritikus lelettel.](assets/screenshots/anomalies/page-light.png#only-light)

## Az oldal {#page}

A **Kritikus**, **Figyelmeztetések** és **Megjegyzések** csempék súlyosságot választanak, a lista fölötti **Nyitott** és **Lezárt** pedig átkapcsol a nyitott, illetve az elmúlt 30 napban lezárt leletek között. A kritikus lelet ugyanazon a felületen áll, mint a többi, a súlyosságát a jelvénye és a színes széle mutatja. Egy lelet először a megoldását kínálja fel, a részleteiben pedig az érték görbeként látszik a szokásos tartományon.

Ha semmi sincs nyitva, a **Tanulási folyamat** elemenként egy tíz részből álló gyűrűt mutat, minden rész egy mentést jelent, amelyből a BombVault tanult. A **Felügyelet** megnyitja azon elemek listáját, amelyeknek saját beállításaik vannak.

## Mit ellenőriz {#checks}

Egy konténer, VM, mappakészlet, adatbázis-mentés, a flash és az önmentés minden mentését összeveti az elem saját előzményeivel. Az ellenőrzések ezeket nézik:

- az egy futás során eltárolt új adatot, a friss mentések legnagyobb szokásos mennyiségeihez és az óránkénti szokásos sebességhez képest,
- azt a mentést, amely az adatok nagy részét újra eltárolta, ide értve az átnevezett és újraírt fájlokat is,
- a forrás méretét és a fájlok számát, amelyet a restic minden elemhez és minden adatbázis-mentéshez jelent,
- a restic saját mentési idejét,
- a hibasorozatokat és az időnként ismétlődő hibákat,
- a már nem sikeres visszaállítás-ellenőrzéseket,
- a helyi, SFTP- és rclone-tárolók szabad helyét, a tároló növekedéséből előre vetítve.

Egy elem az első 10 mentéséből tanul. A szinte üres forrást, az adatok nagy részének újraírását és a hibákat az első naptól fogva ellenőrzi.

A forrás méretére vagy fájlszámára vonatkozó leletet arra az első mentésre datálja a rendszer, amelyben a változás megjelent. Az **Összevetés az előző biztonsági mentéssel** felsorolja azokat a mappákat, ahol fájlok tűntek el, jöttek létre vagy változtak, és megjegyzi, ha ennek szinte az egésze egy keresőindexben, egy gyorsítótárban vagy bélyegképek között van, amelyeket az alkalmazás magától újraépít.

A ZFS-elemeket adatkészletenként figyeli, lásd: [ZFS-adatkészletek](zfs-datasets.md#anomalies). Egy asszisztens az [MCP-szerveren](mcp.md#tools) keresztül elolvashatja a nyitott leleteket.

Az ellenőrzéseknek vannak korlátai. Az S3-, B2- és REST-tárolóknak nincs szabadhely-adata, az Unraid felhasználói megosztásán a szabad hely az egész tömbé, és a restic 0.17 előtt készült mentésekhez nem tartozik méretelőzmény.

## Egy lelet lezárása {#closing}

- A **Figyelmeztetések** maguktól lezárulnak, amint az ok megszűnt.
- Az adatvesztésről és a megtelő lemezről szóló **kritikus** leletek addig maradnak, amíg nem teszel valamit.
- A **Nyugtázás** látottnak jelöl egy leletet, és lezárja. Addig nem jelzi újra, amíg az oka legalább egyszer meg nem szűnt.
- A **Jelölés várhatóként** azt mondja, hogy a változás a te műved volt. Az új szint 10 mentés után lesz a normális, de az „szinte üres” ellenőrzést a rendszer soha nem kapcsolja ki.
- A megváltozott kijelölés, például egy bepipált vagy kipipált mappa, magától újraindítja az elem előzményeit.

Értesítés epizódonként egyszer megy ki, és a már értesítést küldő hibákat és visszaállítás-ellenőrzéseket nem jelzi kétszer.

## A régi mentések addig megmaradnak {#retention-hold}

Amíg egy forrás szinte üres, erősen összezsugorodott, vagy az adatainak nagy része újra el lett tárolva, a megőrzés és a prune békén hagyja az elem régi mentéseit, a lelet pedig az utolsó jó mentésre mutat, amelyet a visszaállítási ablak előre ki is választ. Mielőtt egy ilyen leletet lezársz, a BombVault közli, hogy a következő takarítás a megőrzési szabályok szerint újra törli a régi mentéseket. Ha tehát adat hiányzik vagy titkosítva lett, előbb állítsd vissza, és csak utána zárd le a leletet.

## Beállítások {#settings}

Az **Anomáliák** kártya a **Beállítások, Ellenőrzés** alatt tartalmazza a globális beállításokat. Minden vezérlő azonnal ment, amint megváltoztatod, és a kapcsoló alatti három rejtve van, amíg a felismerés ki van kapcsolva.

| Beállítás | Alapérték | Mit csinál |
|---|---|---|
| **Anomáliák felismerése** | Be | Minden mentést összevet az elem saját előzményeivel. Kikapcsolva semmi újat nem ellenőriz, és az **Anomáliák** menüpont eltűnik az oldalsávból. A kártya továbbra is a korábbi leletekre mutat. |
| **Érzékenység** | Kiegyensúlyozott | A Szigorú kisebb változásokat is jelez, a Megengedő csak nagyokat. |
| **Értesítés küldése ekkor** | Csak kritikus leletek | A legalacsonyabb súlyosság, amelynél üzenet megy az [Értesítések](notifications.md) alatt beállított csatornákon. Az ismétlődő mentési és adatbázis-mentési hibák, valamint a sikertelen ütemezett visszaállítás-ellenőrzések már küldenek saját üzenetet, ezért nem mennek ki kétszer. |
| **Régi biztonsági mentések megtartása, ha egy forrás erősen zsugorodik vagy újraíródik** | Be | Amíg egy elemnél nyitott lelet van egy szinte üres forrásról, éles zsugorodásról vagy arról, hogy az adatainak nagy része újra el lett tárolva, a megőrzés és a prune békén hagyja az elem régi mentéseit. Nyugtázd a leletet, vagy jelöld várhatóként, hogy elengedje őket. |

Minden elem használhat saját érzékenységet és saját értesítési minimumot. Állítsd be őket az elem **Ellenőrzés** lapján, vagy az **Anomáliák** oldalon a **Felügyelet** alatt. Egy ZFS-elemnél a fa minden adatkészletére érvényesek.
