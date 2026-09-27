# Tárhelyek

A tárhely egy hely, ahol a BombVault mentéseket tart: egy mappa ezen az Unraidon, egy megosztás egy NAS-on, egy bucket egy felhőszolgáltatónál, egy rest-server, egy SFTP-fiók vagy egy Nextcloud. Minden tárhelyet egyszer csatlakoztatsz, a **Beállítások, Tárolás** alatt, és a hitelesítő adatai, a megőrzése, a védelme és a helyszíne is hozzá tartozik. Az öt tartomány (a konténerek, a VM-ek, a flash, a BombVault saját konfigurációja és a fájlkészletek) ezután a tárhelyek közül választ: hol tárolódjon, és hová másolódjon.

## Tárhely hozzáadása {#add-a-place}

A **Tárhely hozzáadása** egy ablakot nyit meg, szolgáltatónként egy csempével, három csoportban: felhőalapú tárolás, saját üzemeltetésű szolgáltatások, valamint NAS-eszközök és ez a szerver.

1. Válassz egy csempét, és töltsd ki az űrlapját. A szem gomb megmutatja a beírt titkot.
2. A **Kapcsolat tesztelése** ellenőrzi a tárhelyet, és semmit nem hoz létre. Minden tartomány mappájáról jelenti, mit talált: üres vagy még nem létezik, már restic tárolót tartalmaz, vagy melyik hiba állította meg.
3. Adj nevet a tárhelynek; a szolgáltató neve előre ki van töltve. Saját üzemeltetésű eszköznél válaszolj a **Hol van az eszköz?** kérdésre. A felhőszolgáltatók mindig másik helyszínen vannak, egy mappa ezen az Unraidon pedig mindig itt.
4. A **Hozzáadás** elmenti a tárhelyet.

Egy új tárhelyet még egyetlen tartomány sem használ. Válaszd ki a **Tárolva itt** vagy a **Másolva ide** alatt a [Tartományok kártyán](#domains), vagy egy elem kártyáján csak arra az elemre.

## Mappák {#folders}

Egy tárhely tartományonként egy mappát tart: `container`, `vms`, `flash`, `config` és `files`, ugyanazokkal a nevekkel, mint az alapértelmezett mentési helyek. A mappák a tárhely részleteiben szerepelnek, és ott át is nevezhetők (lásd: [Cím módosítása](#addresses)). Az a tartomány, amelynek nincs mappája egy tárhelyen, nem választhatja azt a tárhelyet.

Ha egy tartomány mindkét szerepben használ egy tárhelyet, a második szerep utótagot kap, az első pedig megtartja a mappáját. Az a tárhely, amely már fogadja egy tartomány másolatait, a közvetlenül oda küldött elemeket a `<folder>-direct` mappában tárolja; az a tárhely, amely már tárol egy tartományt, a másolatait a `<folder>-copies` mappában fogadja.

Néhány tárhely maga egy restic tároló: egy cím, amelyen már volt tároló, amikor a tárhelyet hozzáadtad, egy elnevezett tároló egy meglévő beállításból, vagy egy másolási cél egy bucket gyökerében. Egy ilyen tárhelynek nincsenek mappái, minden tartomány az egyetlen tárolóján osztozik, és második szerepet nem vállal. Ha ugyanannál a szolgáltatónál többet szeretnél tárolni, csatlakoztass egy másik bucketet vagy mappát külön tárhelyként.

## A tárhely részletei {#details}

Minden tárhely egy sor a szolgáltatójával, azzal, hogy mire szolgál, és az utolsó tesztjével vagy másolásával. A **Tesztelés** a tárhely minden címét ellenőrzi, a **Részletek** pedig megnyitja a beállításait. A részletekben minden módosítás azonnal mentődik.

- **Általános**: a név, a tárhelyet be- és kikapcsoló kapcsoló, a cím, és saját üzemeltetésű eszköznél a **Hol van az eszköz?** kérdés (lásd: [Az épületen kívül](#off-the-premises)).
- **Megőrzés**: utolsók megtartása, napi, heti és havi, a tárhely minden tárolójára. Egy új tárhely az alapértelmezett szabályokkal indul; az a tárhely, amelynek minden szabálya nulla, soha nem vág vissza semmit.
- **Védelem**: az **Append-only** kapcsoló. Az append-only módot a túloldalnak kell kikényszerítenie; bekapcsolt kapcsolóval a BombVault ott soha nem nyes és nem töröl. Ha egy rest-serveren be van kapcsolva az append-only, az **Append-only tesztelése** lefuttatja a manipulációs tesztet a tárhely minden tartományútvonalán, bekapcsolt másolatán és tárolóján, és egyetlen választ mutat az egész tárhelyre, *törlések elutasítva* vagy *törlések elfogadva* (lásd: [Telephelyen kívüli mentés és helyreállítás](offsite-recovery.md)). Ez a szakasz csak a távoli tárhelyeknél van meg, mert ezen a gépen semmi sem akadályozhatja meg egy helyi tároló törlését.
- **Hozzáférés**: a hitelesítő adatok, S3-nál pedig a tárolási osztály is. A közös hitelesítő adatokat használó tárhely az első módosításkor saját készletet kap. Ha az új hitelesítő adatok nem tudnak megnyitni egy közvetlen tárolót a tárhelyen, az megtartja a régieket, és a válasz ezt jelzi. A mappa-, SFTP- és rclone-tárhelyeknek nincs ilyen szakaszuk.
- **Korlátok**: a fel- és letöltési sebesség, valamint a növekedési keret.
- **Mappák**: tartományonként egy kapcsoló, a mappája nevével. Az itt kikapcsolt tartomány nem választhatja a tárhelyet.

A megőrzés csökkentése előbb rákérdez, és megmondja, hány elemet érint; az append-only kikapcsolása előbb rákérdez, és megmondja, a tárhely hány tárolója veszíti el. Egy tárhely kikapcsolása a rajta lévő összes tárolót kikapcsolja; az a tárhely, amelyen egy tartomány tárolódik, nem kapcsolható ki.

## A Tartományok kártya {#domains}

A kártyán tartományonként egy sor van: az ütemezése, hogy hol tárolódik, hová másolódik, és a kivételei.

- **Tárolva itt**: amíg a tartomány mentési helyén nincs mentés, a választott tárhely lesz a tartomány otthona, és a mentési hely oda költözik. Ha már vannak mentések, a konténereknél, a VM-eknél és a fájlkészleteknél a választás az új elemek alapértelmezése lesz, amelyet az első mentésükkor vesznek át; a már mentéssel rendelkező elemek ott maradnak, ahol vannak, mert a BombVault soha nem mozgat mentést. A flash és a BombVault saját konfigurációja esetén az otthon költözik, a már megírt mentések pedig a régi tárhelyen maradnak.
- **Másolva ide**: egy chip minden tárhelyhez, amely fogadhatja a tartomány másolatait. Egy chip bepipálásával a tárhely a tartomány másolási célja lesz; első alkalommal a BombVault előre megmondja, hány elemet és pillanatképet, valamint mennyi adatot küld az első futás. A pipa kivétele leállítja az új másolatokat: a már ott lévő másolatok megmaradnak, és a tárhely megőrzése szerint öregszenek, a saját választással rendelkező elemek pedig továbbra is oda másolnak. Az utolsó pipa kivétele minden másolást leállít, a később hozzáadott tárhelyekre is, amíg újra be nem pipálsz egyet. A kikapcsolt tárhely halványított chipként jelenik meg, és nem választható.
- **Kivételek**: a saját választással rendelkező elemek listája, a kártyáikra mutató hivatkozásokkal.
- A **Másolás most** azonnal lefuttatja a tartomány másolásait.

Egy Felfedezésen keresztüli újraépítés után szüneteltetett tartomány a sorában mutatja a szünetet, az **Alapértelmezés megerősítése** gombbal (lásd: [Elhelyezés elemenként](offsite-recovery.md#placement)).

## Cím módosítása {#addresses}

Egy tartomány mappája a tárhely részleteiben módosítható, és ugyanígy egy helyi tárhely címe is, például ha egy tárolót kézzel áthelyeztél egy másik lemezre. A BombVault minden címet tesztel, amelyet a módosítás érint, és akkor fogadja el, ha minden új cím üres és a régin nem volt semmi tárolva, vagy ha minden új cím ugyanazt a restic tárolót tartalmazza, mint a régi. Minden mást elutasít, és megmondja, hány mentés van még a régi címen. Egy távoli tárhely megtartja a címét; ha máshová szeretnél menteni, azt csatlakoztasd külön tárhelyként.

A BombVault a tárhelyek listáját a saját adatbázisából állítja össze, és ehhez soha nem listáz ki távoli tárolót; a teszt csak akkor fut, amikor módosítasz valamit.

## Tárhely eltávolítása {#remove}

Egy tárhely csak akkor távolítható el, ha semmi sem használja: egyetlen tartomány sem tárolódik ott, egyetlen alapértelmezés sem mutat rá, egyetlen elem sincs ott tárolva, és egyetlen ott lévő közvetlen tárolóban sincsenek elemek. Ellenkező esetben az elutasítás felsorolja, mi tartja. Az eltávolítás a másolási céljait is viszi, és a saját hitelesítő adatait is, hacsak egy lehívási forrás vagy egy másik tárhely nem használja őket. Magán a tárhelyen semmi sem törlődik, és a megerősítés megmondja, hány másolat marad ott.

## Tárhely nélkül {#without-a-place}

Az a cím, amely nem illik a tárhely plusz mappa formába, továbbra is működik, és a **Tárhely nélkül** alatt szerepel, a címével együtt. Ide tartoznak a natív `b2:`, `gs:` és `swift:` címek is. A **Hozzárendelés tárhelyhez** egy ilyen sort egy tárhelyhez kapcsol, ugyanazzal a teszttel, mint a [cím módosítása](#addresses). A tárhely nélküli másolási célt a tartománya sora is megnevezi, a chipek mellett, és a cél továbbra is másol. Az ottani távoli sornak saját **Append-only** kapcsolója van, és a kikapcsolása előbb rákérdez, megadva, hány elem tart mentéseket ezen a címen. A közvetlen tároló a célja kapcsolóját követi.

## Az épületen kívül {#off-the-premises}

A **Hol van az eszköz?** kérdésre két válasz van: **Itt, az épületben** és **Másik helyszínen**. Egy másolat a kártyák 3-2-1 sorában és az irányítópult telephelyen kívüli ellenőrzéseiben csak akkor számít külön helyszínnek, ha a tárhelye másik helyszínen van. Egy második lemez vagy egy NAS ugyanabban az épületben második másolat, nem második helyszín. A válasz egyetlen másolatot sem változtat meg. A felhőszolgáltatók mindig másik helyszínen vannak, egy mappa ezen az Unraidon pedig mindig itt, ezért az űrlap ezekre nem kérdez rá; minden más tárhelynél a részletekben módosíthatod a választ. A másik helyszínen lévő tárhely sorában a **Másik helyszín** jelölés áll.

## Kapcsolattípusok

### Mappa ezen az Unraidon vagy egy NAS-on {#kind-local}

A cím egy `/mnt` alatti útvonal, `/mnt` nélkül írva, például `user/bombvault`, és minden tartomány mappája alatta van: `user/bombvault/container`.

- A **Mappa ezen az Unraidon** a megosztások, a lemezek és a poolok közül választ.
- A **Synology**, a **QNAP**, a **TrueNAS**, a **Másik Unraid** és a **Más megosztás** a `/mnt/remotes` alól választ. Előbb csatold a megosztást az Unraidben, például az Unassigned Devices bővítménnyel. A Host Data csatolásnak Read/Write - Slave módúnak kell lennie, különben az a megosztás, amely a BombVault indulása után csatolódik, egy újraindításig láthatatlan marad (lásd: [Konfiguráció](configuration.md)).

A mappaválasztó az **Új mappa** gombbal ott hoz létre mappát, ahol éppen áll. A teszt ellenőrzi, hogy a mappa üres vagy nem létezik, és hogy a BombVault tud-e oda írni.

### S3 {#kind-s3}

A cím `s3:https://<endpoint>/<bucket>/<path>`, például `s3:https://s3.eu-central-003.backblazeb2.com/tower-backups/bombvault`.

- A **Backblaze B2**-höz csak a kulcsazonosító és az alkalmazáskulcs kell. A BombVault megkérdezi a B2-t, melyik bucketre, S3-végpontra és mappára van korlátozva a kulcs, és ezekből állítja össze a címet. Az a kulcs, amely minden buckethez hozzáfér, felkínálja a bucketjeit választásra.
- **Amazon S3**, **Cloudflare R2**, **Wasabi**, **Hetzner Object Storage**, **Storj**, **IDrive e2**, **Scaleway**, **OVHcloud**, **DigitalOcean Spaces**, **IONOS**, **Contabo**, **Exoscale** és **Vultr** esetén az űrlap a kulcsot kéri, és ahol a szolgáltatónak kell, a régiót, a fiókazonosítót vagy a végpontot is. A BombVault kitölti a végpontot, és listázza a bucketeket, ha a kulcs listázhatja őket; különben írd be a bucket nevét.
- A **Google Cloud Storage** az S3-felületén keresztül, egy HMAC-kulccsal csatlakozik, amelyet a Cloud Storage beállításaiban, az Interoperability alatt hozhatsz létre. Szolgáltatásfiók-fájl itt nem működik.
- **MinIO**, **SeaweedFS**, **Garage**, **Ceph**, **JuiceFS**, **RustFS**, **Versity S3 Gateway** és **Más S3-szolgáltatás** esetén a szolgáltatás címét és egy kulcsot kell megadni.

A tárolási osztályt a tárhely részleteiben állíthatod be, azokra a szintekre korlátozva, amelyeket egy visszaállítás felolvasztás nélkül olvasni tud.

### rest-server {#kind-rest}

A cím `rest:<url>/<user>`, például `rest:https://nas.lan:8000/tower`. Az űrlap a szerver címét, egy felhasználót és egy jelszót kér. A `--private-repos` kapcsolóval egy felhasználó csak a saját nevével kezdődő útvonalakat érheti el, ezért a BombVault a felhasználót teszi az elejére, hacsak nem írsz be másik útvonalat. Ha a szerver elutasít egy útvonalat, amely kívül esik a felhasználó sajátján, a hibaüzenet ezt megmondja.

A rest-server űrlap egy beilleszthető receptet tartalmaz egy append-only módú rest-serverhez, egy felhasználóval ennek a BombVaultnak. A **Recept megjelenítése** létrehoz egy jelszót, amely csak egyszer látható, és ad egy `docker run` sort, egy compose-fájlt és egy Unraid-sablont, mindegyiket a szerverre kerülő `htpasswd` sorral; a felhasználó és a jelszó egyenesen az űrlapba kerül.

A **Másik BombVault** a saját mezői fölött felsorolja a nyitott ajánlatokat, amelyeket más példányok a Flottán keresztül küldtek. Egy ajánlat elfogadása olyan tárhelyet ad hozzá, amely csak a felajánlott tartomány másolatait őrzi, mert egy ajánlat csak ahhoz az egy tartományhoz hordoz felhasználót. A Flotta oldalon történő elfogadás ugyanezt a tárhelyet adja hozzá.

### SFTP {#kind-sftp}

A cím `sftp://<user>@<host>:<port>/<path>`, például `sftp://bv@backup.lan:22/bombvault`. Az űrlap a hosztot, a portot és a felhasználót kéri, és megmutatja a BombVault nyilvános kulcsát. Add hozzá ezt a kulcsot a felhasználó `~/.ssh/authorized_keys` fájljához a szerveren; ott semmi mást nem kell telepíteni. A BombVault az első kapcsolatfelvételkor elfogadja a szerver hosztkulcsát, és onnantól ellenőrzi.

A **Hetzner Storage Box** kitölti a `<user>.your-storagebox.de` címet és a 23-as portot. A kulcsot a Hetzner saját parancsával telepítsd a boxra, amely egyszer kéri a box jelszavát:

```sh
echo '<public key>' | ssh -p 23 <user>@<user>.your-storagebox.de install-ssh-key
```

### WebDAV: Nextcloud, ownCloud, OpenCloud {#kind-webdav}

Az űrlap a szerver címét, a felhasználót és egy alkalmazásjelszót kér. Az alkalmazásjelszót a fiók biztonsági beállításaiban hozd létre, és e-mail-cím helyett a felhasználói azonosítót add meg. A BombVault összeállítja a termék által használt WebDAV-útvonalat, és a kapcsolatot az rclone környezeti változóin keresztül adja át a resticnek, a jelszót az rclone obscure formájában. A cím `rclone:bvp<id>:<path>` alakú, ahol a `bvp<id>` egy olyan remote, amely csak abban a környezetben létezik; az rclone-konfigurációba semmi sem íródik.

### Azure Blob {#kind-azure}

A cím `azure:<container>:/<path>`. Az űrlap a tárfiókot és annak hozzáférési kulcsát kéri; a **Kapcsolat tesztelése** után felsorolja a fiók konténereit választásra, vagy beírod egy konténer nevét. A BombVault a fiókot és a kulcsot `AZURE_ACCOUNT_NAME` és `AZURE_ACCOUNT_KEY` néven adja át a resticnek.

### rclone {#kind-rclone}

A cím `rclone:<remote>:<path>`. Az űrlap felsorolja a BombVault rclone-konfigurációjának remote-jait választásra. A konfiguráció cseréjéhez illessz be egy teljes `rclone.conf` fájlt az **rclone-konfiguráció** alá, és kattints a **Konfiguráció mentése** gombra. Azonnal elmentődik, és minden rclone-tárhelyet kiszolgál, akár hozzáad utána tárhelyet az ablak, akár nem.

## Másolatok eltérő hitelesítő adatú tárhelyek között {#different-credentials}

Egy távoli tárhelyen tárolt tartomány a saját másolatainak forrása. A `restic copy` egyetlen környezettel fut, és a BombVault a forrás hitelesítő adatait hozzáadja a céléhoz, ha a kettő nem állítja ugyanazt a változót különböző értékre. Egy Nextcloud-tárhely és egy B2-tárhely különböző változókat használ, így egy Nextcloudban tárolt tartomány másolható a B2-be. Két S3-fióknak vagy két rest-server felhasználónak ugyanazokra a változókra lenne szüksége különböző értékekkel; a restic nem tudja mindkettőt fogadni, és a Tartományok kártyán lévő chip jelzi, hogy a hitelesítő adatok nem illenek össze.
