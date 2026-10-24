# Hibaelhárítás

Egy rövid GYIK. A teljes VM-SSH hoszt-oldali hibaelhárítási táblázatért (permission-denied, hoszt-kulcs ellenőrzés, hiányzó sablonváltozók és több) lásd a [VM-mentés SSH-n keresztül útmutatót](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) a GitHubon.

## Valami nincs helyesen bekötve

Add hozzá az áttekintéshez a **Hosztintegráció** kártyát a **Testreszabás** alatt. A hosztintegráció-ellenőrzés minden csatolást és CLI-t megvizsgál (Docker socket, libvirt, restic, qemu-img, rclone), és jelenti a hiányzó darabokat, az **Ellenőrzés most** pedig újra lefuttatja. Itt kezdd, mielőtt hibát feltételeznél: egy hiányzó csatolás vagy egy elérhetetlen hoszt azonnal megjelenik.

## Nem érem el a webes felületet

A BombVault alapból HTTPS-t szolgál ki a `3443` porton (önaláírt tanúsítvánnyal), így nyisd meg a `https://<your-unraid-ip>:3443` címet. Fogadd el az önaláírt tanúsítvány figyelmeztetését, vagy tedd a BombVaultot egy reverse proxy mögé a saját tanúsítványoddal. Ha `HTTP_ONLY=true` mellett futtatod, akkor egyszerű HTTP-t szolgál ki a `3000` porton (egy TLS-lezáró proxy mögötti használatra szánva).

## Elveszítettem az APP_KEY-t

Az `APP_KEY` származtatja a restic tároló jelszavát. Nélküle, a helyreállítási csomag nélkül és a csoportodban lévő kulcsletét nélkül a titkosított mentések nem állíthatók helyre. Ezért mutatja az áttekintés a **Mentsd el a helyreállítási csomagot** feliratot, amíg le nem töltötted. Lásd: [Telephelyen kívüli mentés és helyreállítás](offsite-recovery.md#recovery-kit). Generálj egy kulcsot az `openssl rand -hex 32` paranccsal, és tárold a szerveren kívül, mielőtt bármely mentésre hagyatkoznál.

## Elfelejtettem a bejelentkezési jelszót

Válaszd az **Elfelejtetted a jelszót?** gombot a bejelentkezési oldalon. A BombVault egy 12 karakteres kódot ír a konténere naplójába (Unraidben: Docker fül, BombVault ikon, **Logs**). Add meg 10 percen belül egy új jelszóval. A visszaállítás a kétlépcsős azonosítást is kikapcsolja, eltávolít minden hozzáférési kulcsot, visszavon minden API-tokent, MCP-kulcsot és OAuth-bejelentkezést, lezár minden más munkamenetet, és kikapcsolja a telefonok párosítását. Add hozzá újra a hozzáférési kulcsaidat a **Beállítások, Biztonság** alatt, kapcsold be ott újra a telefonok párosítását, és hozz létre új tokeneket és kulcsokat ott, ahol még szükséged van rájuk. Lásd: [Biztonsági modell](configuration.md#login).

## A mentések leálltak, és azt írják, hogy a kulcs már nem nyitható ki

Ez a szerver a Vészvarázslóban átvette egy másik szerver mentési kulcsát, és utána az `APP_KEY` megváltozott. Az átvett kulcs a régi APP_KEY-jel van lezárva, így a BombVault nem tudja kinyitni, és inkább semmit sem ment, mintsem rossz kulccsal írjon. Az áttekintésen a **Figyelmet igényel** megnevezi. Állítsd vissza az APP_KEY-t arra, ami volt, vagy töltsd fel újra a helyreállítási csomagot a Visszaállítás oldalon a **Vészhelyzet** alatt.

## Egy telefon a bejelentkezési oldalra érkezik

Egy párosított telefon csak addig kapja meg a munkamenetét a csoporton át, amíg a **Beállítások, Biztonság** alatt be van kapcsolva ez: **A telefonok párosíthatók a csoport tizenkét szavával**. A jelszó visszaállítása kikapcsolja. A szervernél régebbi alkalmazás sem kap munkamenetet, amíg nem frissül. A cím szerint hozzáadott szerver mindig jelszót kér. A Példányok oldalról eltávolított telefon kikapcsolva tér vissza, amikor újra megnyílik az alkalmazása; hogy beengedd, kapcsold be a részleteiben a **Felügyelet** alatt az **Ennek a példánynak a lekérdezése** kapcsolót.

## A VM-mentés nem csatlakozik

A VM-mentés SSH-n keresztül kommunikál a libvirttel, soha nem egy csatoláson.

- Ellenőrizd, hogy az SSH engedélyezve van-e a hoszton, és a BombVault nyilvános kulcsa engedélyezve van-e a `/root/.ssh/authorized_keys` fájlban (a Beállítások, Integrációk, Gazdagép SSH mutatja a kulcsot és egy **Kapcsolat tesztelése** gombot).
- Egy egyéni `br0.x` hálózaton állítsd a `LIBVIRT_HOST`-ot az Unraid LAN IP-jére (a konténer ott nem éri el a hosztot a `host.docker.internal`-on keresztül). Engedélyezd a **Settings, Docker, Host access to custom networks** opciót.
- Ha megváltoztattad az Unraid SSH-portját, állítsd be a `LIBVIRT_SSH_PORT`-ot, hogy egyezzen.
- TrueNAS-on állítsd be a `LIBVIRT_URI`-t a TrueNAS saját libvirt-socketjével, add meg az alkalmazásnak a `SETPCAP` capabilityt, és add hozzá a kulcsot a roothoz a **Credentials, Users** alatt (lásd: [TrueNAS SCALE](getting-started.md#truenas)).
- A teljes, lépésről lépésre diagnózis (elérhetőségi teszt, VLAN-útválasztás, `Permission denied (publickey)`, `Host key verification failed`) a [VM-mentés SSH-n keresztül útmutatóban](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) található.

## Egy élő VM-pillanatkép nem futott le

Az élő pillanatképekhez szükség van a qemu guest agent telepítésére a VM-ben, és arra, hogy a lemez a `/mnt/cache`-en (vagy a `/mnt/diskX`-en) legyen, ne a `/mnt/user`-en. Egy kikapcsolt VM-en az élő automatikusan visszaesik szabályosra. Egy szabályos mentés leállítja a VM-et, menti a lemezeket, majd újraindítja, így mindig konzisztens. Legfeljebb tíz percig vár, hogy a vendég leálljon, és soha nem kapcsolja ki erőszakkal, így egy frissítéseket telepítő Windows-vendég nem szakad meg félúton. Az a vendég, amely ezután is fut, érintetlen marad, és az a mentés az okkal együtt hiúsul meg.

## Egy mentés a "repository is already locked" hibával hiúsult meg

Ez általában egy árva restic zárolás, amely akkor maradt hátra, amikor a konténert egy művelet közben frissítették vagy újraindították. A BombVault észlel egy bizonyíthatóan árva zárolást, kényszerítve törli és egyszer újrapróbálja, automatikusan. Ha továbbra is fennáll, nyisd meg a **Mentések ellenőrzése** kártyát a **Beállítások, Ellenőrzés** alatt, és az érintett szakasz sorában válaszd a **Feloldás** lehetőséget a **Több** alatt, hogy kézzel törölj egy elavult zárolást. Egy valódi probléma továbbra is felszínre kerül, ahelyett hogy elrejtenék. Újraindítás után a BombVault megvárja, hogy egy ilyen zárolás tíz percig ne frissüljön. A még futó restic, például egy második BombVaultban ugyanazon a tárolón, ötpercenként frissíti a zárolását.

## A telephelyen kívüli másolatom nem történt meg egy mentés után

A telephelyen kívüli replikáció szándékosan legjobb szándék szerinti, így egy telephelyen kívüli zökkenő soha nem hibáztatja el a helyi mentést. Ellenőrizd, mikor másol a cél: a **Másolás** kártyáján az áll, hogy **Minden biztonsági mentés után**, vagy saját tervet nevez meg, és a **Beállítások, Ütemezés, Telephelyen kívüli másolatok szakaszonként** saját tervet adhat egy szakasznak. Használd a **Replikálás most** gombot a szakasz **Külső másolat** kártyáján a **Beállítások, Célok** alatt egy igény szerinti futáshoz, és figyeld az áttekintés állapotfejlécét, amely futás közben azt írja: **Telephelyen kívüli másolás folyamatban**. A sikertelen másolat a **Figyelmet igényel** alatt jelenik meg.

## Egy rest-server tanúsítványa nem megbízható

A restic elutasítja az általa nem ismert tanúsítványt, és a hibaüzenet megmondja, hová kell tenni. Másold a rest-server önaláírt tanúsítványát, vagy az őt aláíró hitelesítésszolgáltatóét, PEM-fájlként a BombVault adatmappájának `certs` mappájába (`/config/certs` a konténerben, `appdata/bombvault/certs` Unraidben), és teszteld újra. Újraindításra nincs szükség.

## Egy mentés jelzi, hogy a Host Data területén kívüli adatokról nem készült mentés {#data-outside-host-data}

A konténer olyan mappában tart adatokat, amelyet a BombVault a Host Data felől nem tud olvasni. A mentés elmenti a definíciót és minden mappát, amelyet elér, a sorát narancssárgára színezi a tevékenységnaplóban, és megnevezi a kihagyott mappákat. Két eset gyakori: egy ixVolume TrueNAS-on a `/mnt/.ix-apps` alatt, és egy elnevezett Docker-kötet a `/var/lib/docker/volumes` alatt, például egy compose stack adatbázis-kötete. A konténer **Beállítások** lapján lévő **Mentett mappák** kártya minden ilyen mappát megmutat a konténeren belüli és a gazdagépes útvonalával, pirossal jelölve.

A névtelen Docker-kötetről nincs jelzés. A Docker magától hozza létre egy olyan mappához, amelyet az image megad, de senki nem csatolt, 64 hexadecimális számjegyből álló nevet ad neki, és a `/var/lib/docker/volumes` alatt tárolja. Többnyire gyorsítótár van benne, ezért a mentés zöld marad, a kártya pedig csak felsorolja egy megjegyzéssel, hogy névtelen.

- **Szeretnéd menteni ezeket az adatokat.** Csatold a mappát egy Host Data alatti gazdagépes útvonalhoz, például egy mappához az appdata megosztásodban, és a következő mentés már tartalmazza.
- **Nincs rájuk szükséged.** Add hozzá a két útvonala közül az egyiket sorként ugyanezen a lapon a **Kizárási minták** kártyához: a konténeren belüli útvonalat, például `/cache`, vagy a gazdagépes útvonalat, amelyet a figyelmeztetés megnevez, például `/var/lib/docker/volumes/jellyfin_cache/_data`. A következő mentéstől a figyelmeztetés kihagyja ezt a mappát, a mező alatti előnézet pedig megmondja, miért. Egy fölötte lévő mappa sora is működik, egy önálló név vagy a `/` nem.

## Egy mentés Elhalasztva állapotban áll {#deferred-backups}

A konténer alkalmazása foglalt volt, ezért a mentése vár, és magától újra próbálkozik. A tevékenységnapló sora megmondja, miért, és mikor lesz a következő próbálkozás; a konténer **Beállítások** fülén lévő **Ha az alkalmazás foglalt** kártya ugyanezt mutatja, és azt is, mikor ér véget a várakozás. Semmit sem kell tenned. Ha azonnal menteni szeretnél, használd a **Biztonsági mentés most** gombot: az általad indított mentés soha nem vár, 75-ös kilépési kódnál sem.

- **Saját ellenőrzés.** Állítsd be, hogy a **Biztonsági mentés előtti parancs** 75-ös kóddal lépjen ki, amíg az alkalmazás foglalt, például `[ "$(cat /tmp/players)" = 0 ] || exit 75`. Minden más, 0-tól eltérő kód ugyanúgy sikertelenné teszi a mentést, így egy hibás szkript továbbra is megjelenik a **Figyelmet igényel** alatt.
- **A Plexet, a Jellyfint vagy az Embyt sosem kérdezi meg.** A BombVault előbb a konténer saját címén kérdezi a szervert, és ha ott nincs válasz, a konténeren belül curl vagy wget segítségével próbálja. Egy image, amelyben egyik sincs, nem kérdezhető meg önmagán keresztül; ilyenkor a napló ezt megnevezi. Ha a **Mentés, ha a szerver nem válaszol** be van kapcsolva, egy ilyen mentés mindenképp lefut, kikapcsolva vár.
- **A Jellyfin vagy az Emby elutasítja.** Ellenőrizd az **API-kulcs** értékét. A szerver irányítópultján, az API-kulcsok alatt hozható létre.
- **A Plex elutasítja.** A BombVault a tokent a `/config` alá csatolt mappában lévő `Preferences.xml` fájlból olvassa. Egy szervernek, amelybe sosem jelentkeztek be, nincs tokenje, és csak ott válaszol, ahol token nélkül is enged hozzáférést.
- **Egy szüneteltetett film egész éjjel várakoztatja a mentést.** Állítsd be a **Szüneteltetett munkamenetek kihagyása ennyi után (perc)** értékét, például 30-ra.
- **A forgalomellenőrzés sosem engedi át a mentést.** Emeld a **Foglalt ettől (KiB/s)** értékét afölé, amennyit a konténer akkor küld, amikor senki sem használja. Közvetlenül a BombVault indulása után az ellenőrzés addig vár, amíg **Csendes ennyi ideig (perc)** percig figyelte a konténert.
- **A mentés éjjel sosem fut.** Beállított ablaknál a próbálkozások csak azon belül történnek. Egy várakozás, amelynek határideje az ablakon kívülre esik, akkor is a határidőnél ér véget.

## Egy nagy mentést memóriahiány miatt leállítottak

Állítsd a **Memóriakorlát biztonsági mentésenként (MiB, 0 = nincs)** értékét a **Teljesítmény** kártyán a **Beállítások, Rendszer** alatt jóval az alá, amennyit a konténer használhat. A korlát közelében a restic gyakrabban szabadít fel memóriát, ami némi sebességbe kerül, így a mentés lelassul ahelyett, hogy leállítanák.

## Egy visszaállítás megszakadt, mielőtt elindult volna

Mielőtt bármit is leállítanának vagy eltávolítanának, a visszaállítás lefuttat egy előzetes ütközésellenőrzést: ellenőrzi, hogy a konténer statikus IP-je és a közzétett hoszt-portjai szabadok-e. Ha egy másik konténer már foglal egyet, világos, cselekvésre késztető üzenettel megszakad, ahelyett hogy félig kész visszaállítást hagyna hátra. Szabadítsd fel az ütköző portot vagy IP-t, majd próbáld újra.

Ha a konténer hálózata nem létezik, például mert a TrueNAS-alkalmazását törölték, a visszaállítás előtti ellenőrzés megnevezi a hálózatot, és a visszaállítás elutasítja a futást, mielőtt bármit megváltoztatna. Telepítsd újra az alkalmazást a TrueNAS-ban ugyanazzal a névvel, ami újra létrehozza a hálózatot, majd állítsd vissza az adatait.

## Egy egyszerű export meghiúsult ahelyett, hogy fájlt írt volna

Ha az **Exportok titkosítása age-dzsel** be van kapcsolva a **Beállítások, Rendszer** alatt, de nincs beállítva érvényes címzett, egy export világos hibával leáll, ahelyett hogy nyílt szöveget írna. Adj hozzá egy érvényes címzettet (egy age nyilvános kulcs vagy egy SSH nyilvános kulcs), vagy kapcsold ki a titkosítást, ha az exportot nyílt szövegnek szánod. Lásd: [Funkciók](features.md).

## Egy adatbázis-dump sikertelen volt

A sikertelen dump sosem buktatja el a körülötte futó mentést; saját sikertelen futásként kerül be, és az ok megmondja, mit kell javítani.

- **Elutasított bejelentkezés.** A dump a konténer saját jelszóváltozóival lép be (`POSTGRES_PASSWORD`, `MARIADB_ROOT_PASSWORD`, `MYSQL_ROOT_PASSWORD` vagy ezek `_FILE` változatai). Ellenőrizd őket az adatbázis-konténeren. Ugyanígy bukik el az a `_FILE` változó, amely olyan titokra mutat, amelyet a konténer saját felhasználója nem olvashat.
- **Hiányzó jogosultságok.** Véletlenszerű root-jelszó mellett a dump csak az alkalmazás felhasználójaként tud belépni, így csak azt az egy adatbázist tartalmazza, a MySQL 8.4 és újabb pedig teljesen elutasíthatja. Adj a konténernek valódi root-jelszót, vagy kapcsold ki nála a dumpot.
- **A rendszertáblák frissítést kérnek.** A MariaDB megtagadja a dumpot, ha a rendszertáblái régebbi verzióból valók (1558-as hiba). Vedd fel a `MARIADB_AUTO_UPGRADE=1` változót és indítsd újra a konténert, vagy futtasd benne egyszer a `mariadb-upgrade` parancsot.
- **Nincs dump eszköz.** Egy lecsupaszított vagy saját kezűleg épített képfájlból `pg_dump`, `mysqldump` vagy `mariadb-dump` nélkül nem lehet dumpot készíteni. Válts a hivatalos képfájlra, vagy kapcsold ki a dumpot.
- **Időkorlát.** Egy dump a `DB_DUMP_MAX_HOURS` keretet kapja (alapból 6), a körülötte futó mentés a `BACKUP_MAX_HOURS` keretet, és az a dump, amelyik nem halad tovább, `BACKUP_STALL_HOURS` után elvágódik. Az utóbbi mögött rendszerint az alkalmazás által tartott zár áll. Emeld meg azt a korlátot, amelyik életbe lépett, vagy akkor dumpolj, amikor az alkalmazás nyugton van.
- **A konténer szünetel vagy újraindul.** A dump a futó kiszolgálóval beszél. Ha a konténer folyton újraindul, a saját naplója megmondja, miért.
- **Egy sérült dumpot nem lehetett eltávolítani.** Azt a dumpot, amelyet a BombVault nem tudott befejezni, törli. Ha ez a törlés nem sikerül, a dump sérültként megjelölve a listában marad, és onnan törölheted.

## Egy import sikertelen volt

Az import leállítja a konténert, félreteszi az adatmappáját, és hagyja, hogy a képfájl a helyére egy üreset hozzon létre. Ha az import előtti lépések egyike hiúsul meg, a régi mappa magától visszakerül. Ha maga az import bukik el, a konténernél marad a friss mappa, a régi pedig mellette marad `<adatmappa>.bombvault-before-import-<időbélyeg>` néven; a futás hibaüzenete megnevezi a pontos elérési utat.

Egy önálló adatkészletet alkotó adatmappa, mint TrueNAS-on, nem nevezhető át. Az import ilyenkor a helyén hagyja a mappát, és a tartalmát a mellette lévő megőrzött mappába helyezi át, ezért a fölötte lévő adatkészletnek helyre van szüksége hozzá. Az import ezt ellenőrzi, mielőtt leállítja a konténert.

Kézi visszaállítás: állítsd le a konténert, nevezd át az aktuális adatmappát az útból, nevezd vissza a megőrzött mappát az eredeti nevére, majd indítsd el a konténert. Adatkészletnél a mappák átnevezése helyett a tartalmat helyezd át. Unraidon ezt a Shares fül fájlkezelője elvégzi.

## Egy ZFS-adatkészlet mentése hibára futott vagy kihagyott egy adatkészletet {#zfs-datasets}

Minden problémához szögletes zárójelben ok-kód tartozik, és a [ZFS-adatkészletek](zfs-datasets.md#reason-codes) oldal mindet felsorolja a javítással. A három leggyakoribb:

- **`snapshot-loop`**: a pillanatkép nem jutott el a BombVaulthoz, mert a Host Data nem adja tovább az új csatolásokat. Szerkeszd a konténert, állítsd a Host Data Access Mode értékét Read/Write - Slave-re, és indítsd újra a BombVaultot.
- **`key-not-loaded`**: a titkosított adatkészletet, amelynek kulcsa nincs betöltve, kihagyja. Töltsd be a kulcsot a `zfs load-key` paranccsal, és csatold az adatkészletet; a következő mentés már tartalmazza.
- **`ssh-auth`**: a kiszolgáló elutasította a BombVault kulcsát. A **ZFS-kapcsolat ehhez a kiszolgálóhoz** ablak, amelyet a Biztonsági mentések oldalon a **Kapcsolat megtekintése** nyit meg, mutatja a parancsot, amely engedélyezi; futtasd egyszer a kiszolgálón.

## Egy elem "Tanul N/10" állapotban marad

A legtöbb anomália-ellenőrzés egy elem 10 sikeres mentése után indul, és a számlálás újraindul a **Jelölés várhatóként** után és az elem kijelölésének módosulása után. Az ütemezés nélküli elem nem tanul, egy appdata nélküli konténernek pedig nincs miből tanulnia, amit a jelvénye is mutat. A **Tanulási folyamat** az Anomáliák oldalon elemenként tíz részből álló gyűrűt mutat.

## A megőrzés nem törli többé egy elem régi mentéseit

Egy nyitott kritikus anomália tartja vissza őket: az elem forrása majdnem üres, erősen összezsugorodott, vagy egy mentés az adatok nagy részét újra eltárolta. Nyisd meg az anomáliát az elem jelvényéről. Ha adat hiányzik vagy titkosították, előbb állítsd vissza a hivatkozott utolsó jó mentésből. Utána nyugtázd az anomáliát, vagy jelöld várhatónak, ha a változás tőled jött, és a következő futás a szokásos módon takarít. A megőrzési előnézet az ilyen elemet megtartottként jelöli. Egy ZFS-elemnél csak az anomáliában megnevezett adatkészlet tartja meg a régi mentéseit; a fa többi adatkészlete a szokásos módon takarítódik.

## A kézi tisztítás szerint néhány elemet megtartott

Ugyanaz az ok: a tisztítás békén hagyja az ilyen anomáliájú elem régi mentéseit, és az üzenetében megnevezi az elemet. Minden mást a szokásos módon tisztít.

## Az előzmények importja szerint egy tárolót nem sikerült beolvasni

Frissítés után a BombVault egyszer beolvassa a korábbi mentések méretét minden tárolóból. Az a tároló, amely akkor nem volt elérhető, például egy leállt külső cél vagy egy fel nem csatolt megosztás, megjelenik a **Beállítások, Ellenőrzés** alatti **Anomáliák** kártyán, és naponta egyszer újrapróbálja. Addig az elemei az új mentésekből tanulnak.

## A lemezterület-figyelmeztetés nem egyezik az Unraid irányítópulttal

Az Unraid felhasználói megosztásán (`/mnt/user`) a szabad hely az egész tömbé, nem egyetlen lemezé. A távoli tárolókat csak olyan rclone távoli tárolókon keresztül méri, amelyek jelentik a szabad helyüket; az S3, B2, REST és SFTP tárolóknak nincs adatuk, és az **Anomáliák** kártyán nem mértként szerepelnek.

## Egy MI-asszisztens nem tud csatlakozni

Az [MCP-kiszolgáló](mcp.md#troubleshooting) oldal leírja, mit jelentenek az MCP-végpont egyes állapotkódjai és elutasításai, és mit tehetsz ellenük.

## A konténer folyamatosan újraindul vagy egészségtelennek tűnik

A BombVault a saját `/api/health`-jéből jelent egészségeset/egészségtelent. Egy automatikus gyógyító eszköz (mint az Autoheal) automatikusan újraindíthatja, ha a motor valaha beragadna. Ellenőrizd a konténer naplóját és a **Hosztintegráció** kártyát a mögöttes okért.

## Még mindig elakadtál?

- Olvasd el a teljes [Konfiguráció](configuration.md) és [Telephelyen kívüli mentés és helyreállítás](offsite-recovery.md) oldalakat.
- Kérdezz az [Unraid támogatói témában](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/).
- Nyiss egy [GitHub-hibajegyet](https://github.com/junkerderprovinz/bombvault/issues).
