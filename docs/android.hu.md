# Android-alkalmazás

Az Android-alkalmazás a csoportod összes BombVault-szerverét a telefonodra hozza. A szervereid listájával indul, felette mindegyikük tevékenységnaplójával, ugyanazokkal a sorokkal, amelyeket az Irányítópult mutat, és annak a szervernek a telefonos nézetét nyitja meg, amelyikre koppintasz. Maga az alkalmazás semmit sem ment.

## Az alkalmazás beszerzése {#install}

- **Beállítások, Alkalmazások:** az Android-alkalmazás kártyája felkínálja a szerver által futtatott kiadás APK-ját, QR-kóddal, amelyet a telefonról lehet beolvasni.
- **APK:** minden kiadás oldalán ott van a `bombvault-android.apk`, és [ez a hivatkozás](https://github.com/junkerderprovinz/bombvault/releases/latest/download/bombvault-android.apk) mindig a legújabb buildet tölti le. Android 10 vagy újabb kell hozzá, és az Android egyszer megkérdezi, hogy az az alkalmazás, amellyel a fájlt megnyitod, telepíthet-e alkalmazásokat.
- **Google Play:** az alkalmazás zárt tesztben van, amíg nyilvános nem lehet. A Google Play egy új fejlesztői fiók alkalmazását csak akkor teszi közzé, ha legalább 12 tesztelő 14 napig telepítve tartotta. Ha segítenél, lépj be a [tesztelői csoportba](https://groups.google.com/g/arrowloop-testers), nyisd meg a [tesztoldalt](https://play.google.com/apps/testing/bombvault.halleluja.design), koppints a **Legyen tesztelő** gombra, és telepítsd a BombVaultot a Google Playről.
- **F-Droid:** a bejegyzés később érkezik.

Az alkalmazást 9.7.0-s vagy újabb verziójú szerverekkel párosítsd. Egy régebbi verziójú szerver lehet ugyanabban a csoportban, de a telefont sima példányként mutatja, és az alkalmazás csak bejelentkezés után olvassa annak a szervernek a tevékenységét.

## Párosítás QR-kóddal {#pairing}

1. A csoportod bármelyik szerverén nyisd meg a **Beállítások, Párosítás** oldalt, és válaszd a **Jelmondat megjelenítése** lehetőséget. Megjelenik a tizenkét szó, mellettük egy QR-kóddal.
2. Az alkalmazásban koppints a **QR-kód beolvasása** gombra, és irányítsd a telefont a kódra. A szavakat be is illesztheted vagy begépelheted.
3. Mielőtt bármit mentene, az alkalmazás felsorolja a csoport szervereit. A **Mind a(z) N hozzáadása** gomb mindegyiket felveszi.

Ezután a telefon egy újabb példányként csatlakozik a csoporthoz. Azt, hogy mi fut az egyes szervereken, bejelentkezés nélkül, a csoporton keresztül olvassa: otthon közvetlenül, otthonon kívül a relén át. Magának a csoportnak a működését a [Példányok párosítása](offsite-recovery.md#pairing) írja le.

Az alkalmazás a relén keresztül csatlakozik a csoporthoz, otthon is. Ha a példányod **Nincs relay** beállításon van, a kód beolvasása előtt válassz egyet a **Beállítások, Párosítás** oldalon.

## Otthonról távol {#away}

Az alkalmazás először a szerver saját címén próbálkozik. Ha az nem válaszol, például mobilhálózaton, a szerver felületét a csoport relayén keresztül nyitja meg. A csoport olyan szervere, amelynek címét az alkalmazás még nem ismeri, ugyanígy nyílik meg. Ha sem a cím, sem a relay nem válaszol, a kezdőképernyő ezt a szerver kártyáján jelzi.

A relayen át az oldal azt éri el, amit egy párosított telefon otthon, ahogy lent le van írva. A jelszavas bejelentkezés és a fájlmentés csak a szerver saját címén működik. A folyamatjelzők kétmásodpercenként frissülnek folyamatos helyett. A felület fájljai az első látogatás után a telefonon maradnak, így a későbbi látogatások gyorsabban töltenek be. A 10.0.0-nál régebbi szerver csak a saját címén nyílik meg.

Az alkalmazás csak akkor fogad el választ, ha a szerver azt azzal a kulccsal írta alá, amelyet az alkalmazás a szerverrel való első találkozáskor rögzített. Ha a szerver később más kulccsal válaszol, a kezdőképernyő az új kulcs ujjlenyomatát mutatja, és az oldal csak azután nyílik meg, hogy megbízhatónak jelölted.

## Szerverváltás {#switch}

Ha egy szerver meg van nyitva, az áttekintése tetején a szerver neve áll, előtte egy vissza nyíllal. A nyíl az alkalmazás áttekintéséhez, a kezdőképernyőre visz, amely felsorolja az összes szervert, amelyet az alkalmazás ismer, az állapotukkal: **Csatlakozva**, **Nincs csatlakozva**, **Bejelentkezés szükséges** vagy **Tanúsítvány ellenőrzése**. A csoport olyan szervere, amely nem válaszol, **Inaktív**, szürkén, azzal együtt, hogy a csoport mikor látta utoljára, az pedig, amelyről az alkalmazás még nem hallott, **Ellenőrzés** állapotot mutat. Az alkalmazás minden csatlakozáskor beolvassa a csoport listáját az egyik példányától, így olyan példány is szerepel a listán, amellyel a relayen át soha nem találkozott, például egy olyan, amely ki volt kapcsolva, amikor a telefon csatlakozott a csoporthoz. Ott koppints egy másikra a megnyitásához, otthon közvetlenül, egyébként a relayen át.

## Bejelentkezve a párosított telefonon {#sign-in}

A csoportoddal párosított telefon a csoport minden szerverét eleve bejelentkezve nyitja meg. Mielőtt betöltene egy oldalt, a csoporton át munkamenetet kér az adott szervertől, és a szerver csak olyan tagnak ad ilyet, amely telefon. A cím alapján hozzáadott szerver jelszót kér, ahogy egy böngészőben is. Aki ismeri a tizenkét szót, az amúgy is megnyithatja a csoport minden mentését, így a párosítás nem ad semmi újat.

## Csoporton kívüli szerverek {#other-servers}

- A **Szerver hozzáadása** azt a címet várja, amellyel a BombVaultot böngészőben megnyitod, például `192.168.1.10:3443`. Ha nincs előtte `http://` vagy `https://`, az alkalmazás https-t használ.
- A helyi hálózaton magukat bejelentő szerverek az **Ezen a hálózaton** alatt jelennek meg, és egy koppintással megnyílnak. Ezt addig teszik, amíg a Beállítások, Integrációk alatt be van kapcsolva a **Megtalálás a hálózaton**. Egy másik hálózatban vagy VPN mögött lévő szerver ott nem jelenik meg.
- Egy önaláírt tanúsítványt egyszer, az SHA-256 ujjlenyomata alapján fogadsz el megbízhatóként. Ha a szerver később másikat mutat, az alkalmazás figyelmeztet, és csak azután nyitja meg, hogy az új tanúsítványt is megbízhatónak jelölted.

## A telefon a Példányok oldalon {#instances}

A telefon saját kártyát kap a csoport minden szerverének Példányok oldalán, Android-alkalmazásként megjelölve, és azon a néven, amelyet a telefonnak adtál. Nincs eredménytáblája, mert semmit sem ment. Az **Eltávolítás** leveszi az oldalról, és azonnal lezárja a munkamenetét, ahogy az is, ha a részleteiben a **Felügyelet** alatt kikapcsolod az **Ennek a példánynak a lekérdezése** kapcsolót. A telefon továbbra is ismeri a szavakat, így újra megjelenik, amikor megnyílik az alkalmazása, de kikapcsolva, és addig nem kap munkamenetet, amíg valaki be nem kapcsolja. Elveszett telefonnál távolítsd el a kártyáját minden szerveren, kapcsold ki a **A telefonok párosíthatók a csoport tizenkét szavával** kapcsolót a **Beállítások, Biztonság** alatt, vagy válaszd minden példánynál a **Csoport elhagyása** lehetőséget, és párosítsd őket újra új szavakkal.

A 10.0.0 előtt párosított telefon a frissítés utáni első látogatásakor kezdi aláírni a hívásait, és minden szerver megtartja az első kulcsot, amelyet a telefonhoz kap. Ha a telefon offline állapota alatt valaki más, aki ismeri a csoport szavait, előbb ér oda, az a szerver attól kezdve elutasítja a telefont, és az alkalmazás már nem tudja megnyitni a relayen keresztül. A javításhoz jelentkezz be arra a szerverre böngészőből, válaszd a **Kulcs elfelejtése** gombot a telefon kártyáján a Példányok oldalon, majd az **Elfelejtés megerősítése** gombot, és nyisd meg újra az alkalmazást. A szerver a következő híváskor a telefon saját kulcsát veszi át. Ha a telefont újra elutasítják, valaki másnak megvannak a szavak: válaszd minden példánynál a **Csoport elhagyása** lehetőséget, és párosítsd őket újra új szavakkal, mint elveszett telefon esetén.

## Beállítások {#settings}

A plusz melletti fogaskerék nyitja meg az alkalmazás beállításait:

- a nyelv, és az a név, amellyel a telefon a Példányok oldalon megjelenik (üresen hagyva a telefon típusa),
- a megjelenés, amely a lista első szerverét követi, amíg sajátot nem állítasz be, valamint az animációk, amelyeknek külön beállításuk van,
- egy másolható jelentés, ha hibát jelentesz; nincs benne cím, név vagy jelmondat,
- a Névjegy kártya az adatvédelmi irányelvekkel,
- a **Minden szerver eltávolítása**, amely minden szervert eltávolít az alkalmazásból, és kilép a csoportból. Minden szerver, amelyet a telefon még elér, leveszi a telefont a Példányok oldaláról.

## Letöltések és feltöltések {#files}

Az exportok, a helyreállítási csomagok, a flash ZIP-ek és az adatbázis-mentések a telefon Letöltések mappájába kerülnek, ahogy egy böngészőből is. A beállítások importálása a telefon fájlválasztóját nyitja meg. A relayen át nem mentődik fájl; ehhez nyisd meg a szervert a saját címén.

## Érintőképernyőn {#touch}

Az ujj alatt nincs rámutatás (hover), ezért egy vezérlő elhalványul, amíg nyomva tartod, egy márkalogós gomb pedig a márka színében világít, amíg fel nem emeled az ujjad. Egy gomb hosszú megnyomása lassú koppintásnak számít, és nem nyit hivatkozásmenüt. Ha egy kiszürkített gombra koppintasz, megmutatja, miért szürke, amíg máshová nem koppintasz.

## Inkább böngészőben {#browser}

A Chrome és az Edge a BombVault webes felületét saját ablakban futó alkalmazásként tudja telepíteni, telefonon ugyanúgy, mint számítógépen. Semmi sincs gyorsítótárazva, így egy frissítés azonnal látszik.

## Adatvédelem {#privacy}

Az alkalmazásban nincsenek fiókok, hirdetések és analitika, és semmit sem futtat a háttérben. Az [adatvédelmi irányelvei](https://github.com/junkerderprovinz/bombvault/blob/main/android/PRIVACY.md) felsorolják, mit tárol, és mit hova küld.
