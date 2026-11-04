# Ütemezések

A **Beállítások, Ütemezések** alatt van minden terv: a mindent lefedő teljes mentés, egy szakasz saját terve, a telephelyen kívüli másolatok, hogy mely elemek vesznek részt, és milyen sorrendben. Egy-egy elem a saját oldalán külön tervet is kaphat. Ez az oldal azt magyarázza el, hogyan illeszkednek ezek egymáshoz, és végigvesz egy tipikus beállítást.

## A tervek mondatként olvashatók {#plans}

Minden terv egy gomb, amely mondatként olvasható, például *naponta 03:00-kor* vagy *2 naponta 01:30-kor*. Egy kattintás megnyitja, öt móddal:

| Mód | Mit állítasz be |
|---|---|
| **Ki** | Ezen a terven semmi nem fut. |
| **Naponta** | **Idő** |
| **Hetente** | **Napok** és **Idő** |
| **N naponta** | **Minden** ennyi **nap**, és az **Idő**. Az utolsó ütemezett futástól számol, így egy elem kézi mentése nem tolja hátrébb a tervet. |
| **Cron** | Öt mezős **Kifejezés**, például `0 */6 * * *` 6 óránként. A **Példák** felkínál párat, az ablak pedig a következő futásokat mutatja. |

Minden idő a szerver időzónájában értendő. Unraidon ez a rendszeré. Máshol állítsd be a `TZ` értékét, különben minden terv UTC szerint fut, lásd: [Környezeti változók](configuration.md#environment-variables).

## Hét {#week}

A **Hét** kártya hétfőtől vasárnapig megrajzol minden tervezett futást, naponta egy sorban: mentéseket, telephelyen kívüli másolatokat és ellenőrzéseket. Mutass egy vonásra, hogy lásd, mi indul ott. Ahol három terv két órán belül indul, a szakaszt megjelöli, így széthúzhatod őket.

![Beállítások, Ütemezések: fent a Hét sáv, alatta a teljes mentés és a szakaszonkénti tervek.](assets/screenshots/schedule/week-dark.png#only-dark)
![Beállítások, Ütemezések: fent a Hét sáv, alatta a teljes mentés és a szakaszonkénti tervek.](assets/screenshots/schedule/week-light.png#only-light)

## Teljes biztonsági mentés {#full-backup}

A **Teljes biztonsági mentés** az alapértelmezett terv mindenre. A **Mikor futnak a mentések** állítja be: egy menet, amely sorban minden szakaszt lementi, a konténereket, VM-eket, a flasht, a mappákat, a ZFS-adatkészleteket és legvégül az önmentést. Az alatta lévő sor megmondja, mely szakaszokat foglalja magában. A **Ki** kikapcsolja, és akkor csak az fut, aminek saját terve van.

- **Elmaradt biztonsági mentések pótlása indulás után**: ha a szerver ki volt kapcsolva egy tervezett időpontban, a BombVault az indulása után körülbelül két perccel lefuttatja azt a mentést.
- **Parancsok előtte és utána**: a **Parancs a teljes biztonsági mentés előtt** a szerveren fut `sh -c` segítségével, mielőtt a menet elindul, például egy szkript, amely leállít egy szolgáltatást. Ha ez hibázik, a teljes mentés mégis lefut. A **Parancs a teljes biztonsági mentés után** egyszer fut, miután minden szakaszt megpróbált, akkor is, ha az egyik hibázott, ami egy olyan pinghez illik, amely megerősíti, hogy az egész szerver védett. Az egyes konténerek parancsai a saját oldalukon vannak.

Az **Összes mentése most** a Biztonsági mentések oldalon ugyanazt a menetet indítja el kézzel, minden bekapcsolt szakaszon.

## Szakaszonkénti tervek {#plans-per-section}

Egy szakasz kiléphet a teljes mentésből, és a saját idejében futhat. Az **Ütemezések szakaszonként** alatt a **Kivétel** hozzáad egy saját tervvel rendelkező szakaszt, a **Kivétel eltávolítása** pedig visszaküldi. A kártyán lévő sor megmondja, mit csinál a többi, például: **Az összes többi szakasz benne van a teljes biztonsági mentésben.**

Egy saját tervvel rendelkező szakaszt a teljes mentés nem ment le újra, így semmi nem készül kétszer. Ugyanez igaz egy saját tervvel rendelkező egyedi elemre.

A **Telephelyen kívüli másolatok szakaszonként** ugyanígy működik a másolatoknál. Alapértelmezés szerint minden szakasz közvetlenül minden mentés után másol a telephelyen kívüli céljaira. A **Kivétel** egy szakasznak saját másolási tervet ad, például hogy a VM-ek csak vasárnap éjjel másoljanak a Backblaze B2-re. Egy cél saját **Másolás** kártyája is beállíthatja a **Mikor másoljon** értéket, lásd: [Telephelyen kívüli másolatok](offsite-recovery.md#off-site-copies).

## Ütemezett elemek {#scheduled-entries}

Az **Ütemezett elemek** szakaszonként megmondja, hány elem vesz részt, például **38 / 41 ütemezve**. Az **Elemek** megnyitja a listát elemenként egy **Ütemezésben** kapcsolóval. A kikapcsolt elemet csak akkor menti a rendszer, ha kézzel indítod. Védetlennek számít, hacsak nem jelölöd **Ne legyen mentés** állapotúra.

## Sorrend {#order}

A **Sorrend** azt állítja be, milyen sorrendben menti az ütemezett futás és a teljes mentés a konténereket és a VM-eket. Húzz egy elemet a helyére, vagy használd a nyilakat, és minden mozdulat azonnal mentődik. A rendezetlenül hagyott elemeket a legrégebben mentettel kezdve menti. Egy kézi egyedi mentést ez nem érint.

Az **Újraindítás biztonsági mentés után** ugyanazon a kártyán azokat a konténereket érinti, amelyeket a **Más konténerek leállítása** leállított egy mentés alatt. Ezek a függőségi sorrendjükben indulnak újra, és a **Várakozás, amíg a függőségek egészségesek lesznek, mielőtt a következő elindul** esetén mindegyiknek egészségesnek vagy futónak kell jelentkeznie, mielőtt a következő indul. Az **Egészség-időkorlát konténerenként (másodperc)** korlátozza a várakozást, alapértelmezés szerint 120, 5 és 3600 közötti tartományban.

## Pillanatkép leállítás helyett {#snapshot-instead-of-stop}

A **ZFS-en a konténerek leállítása csak a pillanatkép idejére** alapértelmezés szerint be van kapcsolva. Az a konténer, amelynek adatai ZFS-en vannak, csak addig áll le, amíg a BombVault pillanatképet készít azokról az adatokról, ez általában néhány másodperc, és a mentést utána a pillanatképből olvassa. Szüksége van a ZFS-adatkészletek által használt SSH-kapcsolatra a hoszthoz. Az a konténer, amelynek az adatai részben máshol vannak, a teljes mentés idejére leáll, mint korábban, és a futása megmondja, miért, míg az a konténer, amelynek semmi adata nincs ZFS-en, megjegyzés nélkül áll le. Egyetlen ZFS-készleten lévő Unraid felhasználói megosztáson működik. TrueNAS-on az alkalmazásnak a `SYS_CHROOT` képesség is kell, lásd: [TrueNAS SCALE](getting-started.md#truenas). Egy konténer a saját oldalán másképp is beállítható.

A mentés ugyanazokra az útvonalakra és címkékre kerül, mint a leállított konténerrel készített, így a visszaállítások és az ellenőrzések mindkettőt egyformán kezelik. A mentés előtti parancs és az adatbázis-mentés továbbra is előbb fut, a leállítandó többi konténer vele együtt megy le és jön vissza, a mentés utáni parancs pedig akkor fut, amint a konténer újra felállt. Egy mentés utáni frissítés továbbra is megvárja, amíg a mentés kész. A futásai a **Pillanatkép** szót és azt mutatják, hány másodpercig állt a konténer. Ha a BombVault bármelyik konténermentés közepén leáll, a következő indulásakor újraindítja azokat a konténereket, amelyeket lent tartott.

## Frissítések a mentés után {#updates}

- **Konténerek frissítése egy sikeres biztonsági mentés után**: csak akkor húz újabb image-et, ha az előtte lévő mentés befejeződött, így mindig van friss visszaállítási pont. Alapértelmezés szerint ki van kapcsolva, és egy konténer a saját oldalán másképp is beállítható a **Frissítés egy sikeres biztonsági mentés után** alatt.
- **A régi image eltávolítása frissítés után**: törli azt az image-et, amelyet a konténer elhagyott. Egy alapimage-et, amelyen más konténerek osztoznak, soha nem töröl.
- **Az Unraid frissítési állapotának frissítése egy konténer frissítése után**: törli az Unraid frissítési szalagját arra a konténerre.

Frissített konténerenként egy üzenet a **Beállítások, Értesítések** alatt, az **Értesítés container frissítésekor** beállítással kérhető.

## Nyugalom a mentés előtt {#idle}

A **Nyugalom biztonsági mentés előtt** kártya azt állítja be, mikor számít egy alkalmazás tétlennek egy olyan konténernél, amelynek ütemezett mentése rá vár: **CPU ez alatt (egy mag %-a)**, **Forgalom ez alatt (Mbit/s)** és **Legalább (perc)**. Egy médiaszerver tétlennek számít, ha nem streamel, a hosthálózaton lévő konténernél pedig csak a CPU számít. Magát a várakozást minden konténernél külön állítod, lásd: [Ha az alkalmazás foglalt](backups.md#app-busy).

## Saját terv egy elemhez {#own-plan}

Minden elemnek van **Saját terv** beállítása a Beállítások lapja **Ütemezés és cél** kártyáján. Ha itt van terv, az elem a saját időpontjaiban fut, és kilép a teljes mentésből és a szakasza tervéből. Ha nincs, követi azokat.

## Példa: három terv, amely illeszkedik {#example}

Egy tipikus Unraid-gép: a legtöbb konténer éjjel menthető, a VM-ek nagyok és lassan változnak, az Immich pedig egész nap új fotókat kap.

1. **Teljes mentés 03:00-kor.** Nyisd meg a **Beállítások, Ütemezések** oldalt. A **Teljes biztonsági mentés** alatt állítsd a **Mikor futnak a mentések** értékét **Naponta**, `03:00`-ra. Kapcsold be az **Elmaradt biztonsági mentések pótlása indulás után** beállítást, hogy egy kikapcsolt szerverrel töltött éjszakát pótoljon.
2. **A VM-ek hetente egyszer.** Az **Ütemezések szakaszonként** alatt nyomd meg a **Kivétel** gombot, válaszd a **VM-ek** szakaszt, és állítsd a tervét **Hetente**, **Napok** vasárnap, **Idő** `04:00`-ra. A kártya most azt mondja, hogy az összes többi szakasz benne van a teljes mentésben.
3. **A VM-ek vasárnap telephelyen kívülre is másolnak.** A **Telephelyen kívüli másolatok szakaszonként** alatt nyomd meg a **Kivétel** gombot, válaszd a **VM-ek** szakaszt, és állítsd **Hetente**, vasárnap `06:00`-ra, miután a mentésüknek volt ideje befejeződni.
4. **Immich 6 óránként.** Nyisd meg a **Biztonsági mentések** oldalt, majd az `immich` elemet, aztán a **Beállítások** lapját. Állítsd a **Saját terv** értékét **Cron**-ra `0 */6 * * *` kifejezéssel.

Most ez fut:

| Mikor | Mi |
|---|---|
| Minden nap 03:00-kor | Az `immich` kivételével a konténerek, a flash, a mappák, a ZFS-adatkészletek és az önmentés, szakaszonként egymás után |
| Vasárnap 04:00-kor | VM-ek |
| Vasárnap 06:00-kor | A VM-ek telephelyen kívüli másolata |
| 00:00, 06:00, 12:00 és 18:00 | Az `immich` külön |

A **Hét** kártya a 03:00-s sort minden napon, a 04:00-s és 06:00-s vonásokat vasárnap, az `immich`-hez pedig naponta négy vonást mutat. Az `immich` nem kerül le újra 03:00-kor, mert a saját tervvel rendelkező elem kilép a teljes mentésből. A többi szakasz másolatai továbbra is közvetlenül minden mentés után futnak.

Ha később eltávolítod az `immich` saját tervét, újra csatlakozik a 03:00-s futáshoz a szakaszával együtt.
