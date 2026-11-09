# Hibaelhárítás

Egy rövid GYIK. A teljes, VM-ek SSH-n át történő mentésének hoszt-oldali hibaelhárítási táblázatához (permission-denied, hoszt-kulcs ellenőrzés, hiányzó sablonváltozók és más) lásd a [VM-mentés SSH-n át útmutatót](vm-backup-ssh-setup.md).

## Valami nincs helyesen bekötve {#something-is-not-wired-up-correctly}

Add hozzá a **Hosztintegráció** kártyát az áttekintéshez a **Testreszabás** alatt. A hosztintegráció-ellenőrzés minden csatolást és CLI-t megvizsgál (Docker socket, libvirt, restic, qemu-img, rclone), és jelenti a hiányzó darabokat, az **Ellenőrzés most** pedig újra lefuttatja. Itt kezdd, mielőtt hibát feltételeznél: egy hiányzó csatolás vagy egy elérhetetlen hoszt azonnal látszik.

## Nem érem el a webes felületet {#i-cannot-reach-the-web-ui}

A BombVault alapból HTTPS-t szolgál ki a `3443` porton (önaláírt tanúsítvánnyal), ezért nyisd meg a `https://<your-unraid-ip>:3443` címet. Fogadd el az önaláírt tanúsítvány figyelmeztetését, vagy tedd a BombVaultot egy reverse proxy mögé a saját tanúsítványoddal. Ha `HTTP_ONLY=true` mellett futtatod, egyszerű HTTP-t szolgál ki a `3000` porton (TLS-lezáró proxy mögötti használatra szánva).

## Elveszítettem az APP_KEY-t {#i-lost-my-app_key}

Az `APP_KEY`-ből származik a restic tároló jelszava. Nélküle, a helyreállítási csomag nélkül és egy kulcsletét nélkül a csoportodban a titkosított mentések nem állíthatók helyre. Ezért mutatja az áttekintés a **Mentsd el a helyreállítási csomagot** üzenetet, amíg le nem töltötted. Lásd: [Telephelyen kívüli mentés és helyreállítás](offsite-recovery.md#recovery-kit). Generálj kulcsot az `openssl rand -hex 32` paranccsal, és tárold a szerveren kívül, mielőtt bármelyik mentésre hagyatkoznál.

## Elfelejtettem a bejelentkezési jelszót {#i-forgot-the-login-password}

Válaszd az **Elfelejtetted a jelszót?** lehetőséget a bejelentkezési oldalon. A BombVault egy 12 karakteres kódot ír a konténere naplójába (Unraidon: Docker fül, BombVault ikon, **Logs**). Írd be 10 percen belül egy új jelszóval. A visszaállítás a kétlépcsős azonosítást is kikapcsolja, eltávolít minden hozzáférési kulcsot, visszavon minden API-tokent, MCP-kulcsot és OAuth-bejelentkezést, lezár minden más munkamenetet, és kikapcsolja a telefonok párosítását. Add hozzá újra a hozzáférési kulcsaidat a **Beállítások, Biztonság** alatt, kapcsold vissza ott a telefonok párosítását, és hozz létre új tokeneket és kulcsokat ott, ahol még szükség van rájuk. Lásd: [Biztonsági modell](configuration.md#login).

## A mentések leálltak, és azt írják, hogy a kulcs már nem nyitható meg {#backups-stopped-and-say-the-key-can-no-longer-be-opened}

Ez a szerver átvette egy másik szerver mentési kulcsát a Vészvarázslóban, és utána megváltozott az `APP_KEY`. Az átvett kulcs a régi APP_KEY-jel van lezárva, ezért a BombVault nem tudja megnyitni, és inkább nem ment semmit, mintsem rossz kulccsal írjon. Az áttekintésen a **Figyelmet igényel** megnevezi. Állítsd vissza az APP_KEY-t arra, ami volt, vagy töltsd fel újra a helyreállítási csomagot a Visszaállítás oldalon a **Vészhelyzet** alatt.

## Egy telefon a bejelentkezési oldalra érkezik {#a-phone-lands-on-the-login-page}

Egy párosított telefon csak akkor kap munkamenetet a csoporton át, ha a **Beállítások, Biztonság** alatt be van kapcsolva az **A telefonok párosíthatók a csoport tizenkét szavával**. A jelszó visszaállítása kikapcsolja. A szervernél régebbi alkalmazás sem kap munkamenetet, amíg frissítve nincs. A címével hozzáadott szerver mindig a jelszót kéri. A Példányok oldalról eltávolított telefon kikapcsolva tér vissza, amikor az alkalmazása újra megnyílik. Kapcsold be az **Ennek a példánynak a lekérdezése** beállítást a részleteiben a **Felügyelet** alatt, hogy beengedd.

## A VM-mentés nem csatlakozik {#vm-backup-will-not-connect}

A VM-mentés SSH-n át beszél a libvirttel, soha nem csatoláson keresztül.

- Ellenőrizd, hogy az SSH engedélyezve van a hoszton, és a BombVault nyilvános kulcsa engedélyezett a `/root/.ssh/authorized_keys` fájlban (a Beállítások, Integrációk, Gazdagép SSH megmutatja a kulcsot és egy **Kapcsolat tesztelése** gombot).
- Egyéni `br0.x` hálózaton állítsd a `LIBVIRT_HOST`-ot az Unraid LAN IP-jére (a konténer ott nem éri el a hosztot a `host.docker.internal` néven). Kapcsold be a **Settings, Docker, Host access to custom networks** beállítást.
- Ha megváltoztattad az Unraid SSH-portját, állítsd be a `LIBVIRT_SSH_PORT`-ot, hogy egyezzen.
- TrueNAS-on állítsd be a `LIBVIRT_URI`-t a TrueNAS saját libvirt-socketjével, add meg az alkalmazásnak a `SETPCAP` képességet, és add hozzá a kulcsot a roothoz a **Credentials, Users** alatt (lásd: [TrueNAS SCALE](getting-started.md#truenas)).
- A teljes, lépésről lépésre diagnózis (elérhetőségi teszt, VLAN-útválasztás, `Permission denied (publickey)`, `Host key verification failed`) a [VM-mentés SSH-n át útmutatóban](vm-backup-ssh-setup.md) van.

## Egy élő VM-pillanatkép nem futott le {#a-live-vm-snapshot-did-not-run}

Az élő pillanatképekhez a lemeznek a `/mnt/cache` (vagy `/mnt/diskX`) alatt kell lennie, nem a `/mnt/user` alatt. A VM-ben lévő qemu guest agenttel a pillanatkép nyugalmi, nélküle összeomlás-konzisztens. Egy leállított VM-nél az élő automatikusan szabályosra vált. A szabályos mentés leállítja a VM-et, lementi a lemezeket, majd újraindítja, ezért mindig konzisztens. Legfeljebb tíz percet vár a vendég leállására, és soha nem kényszeríti ki, így egy frissítéseket telepítő Windows-vendéget nem szakít félbe. Az a vendég, amely ezután is fut, békén marad, és az a mentés az okával meghiúsul.

## Egy mentés a „repository is already locked” hibával hiúsult meg {#a-backup-failed-with-repository-is-already-locked}

Ez általában egy elárvult restic-zár, amely akkor maradt vissza, amikor a konténert egy művelet közben frissítették vagy újraindították. A BombVault felismeri a bizonyíthatóan elárvult zárat, feloldja, és egyszer automatikusan újrapróbál. Újraindítás után megvárja, amíg egy ilyen zár tíz percig nem frissült. Egy még futó restic öt percenként frissíti a zárját, és a BombVault soha nem távolít el olyan zárat, amelyet még frissítenek, mert az egy második, ugyanabba a tárolóba író BombVaulté is lehet. A 30 perce nem frissült zárat eltávolítja, akárki hozta is létre. Ha a hiba megmarad, nyisd meg a **Mentések ellenőrzése** kártyát a **Beállítások, Ellenőrzés** alatt, és válaszd a **Feloldás** műveletet a **Több** alatt az érintett szakasz sorában, hogy kézzel oldd fel a megmaradt zárat. A **Feloldás** is meghagyja a még frissülő zárat, és megmondja, mikor frissült utoljára. Egy valódi probléma így is felszínre kerül, nem rejtőzik el.

## A telephelyen kívüli másolatom nem történt meg egy mentés után {#my-off-site-copy-did-not-happen-after-a-backup}

A telephelyen kívüli replikáció szándékosan legjobb szándékú, így egy ottani zökkenő soha nem buktatja el a helyi mentést. Nézd meg, mikor másol a cél: a **Másolás** kártyáján az áll, hogy **Minden mentés után**, vagy megnevez egy saját tervet, és a **Beállítások, Ütemezések, Telephelyen kívüli másolatok szakaszonként** egy szakasznak saját tervet adhat. Használd a **Replikálás most** gombot a szakasz sorában a **Telephelyen kívüli másolatok** kártyán a **Beállítások, Célok** alatt egy igény szerinti futtatáshoz, és figyeld az áttekintés állapotfejét, amely a futás alatt a **Telephelyen kívüli másolás folyamatban** szöveget mutatja. Egy sikertelen másolat a **Figyelmet igényel** alatt jelenik meg.

## Egy rest-server tanúsítványa nem megbízható {#a-rest-servers-certificate-is-not-trusted}

A restic visszautasítja a számára ismeretlen tanúsítványt, és a hiba megmondja, hová kell tenni. Másold a rest-server önaláírt tanúsítványát, vagy az őt aláíró hitelesítésszolgáltatóét, PEM-fájlként a BombVault adatmappájának `certs` mappájába (`/config/certs` a konténerben, `appdata/bombvault/certs` Unraidon), és teszteld újra. Újraindításra nincs szükség.

## Egy mentés jelzi, hogy a Host Data területén kívüli adatokról nem készült mentés {#data-outside-host-data}

A konténer olyan mappában tart adatot, amelyet a BombVault a Host Data-n keresztül nem tud elolvasni. A mentés elmenti a definíciót és minden mappát, amelyet elér, a sorát narancssárgára színezi a tevékenységnaplóban, és megnevezi a kihagyott mappákat. Két eset gyakori: egy ixVolume TrueNAS-on a `/mnt/.ix-apps` alatt, és egy nevesített Docker-kötet a `/var/lib/docker/volumes` alatt, például egy compose-stack adatbázis-kötete. A konténer **Beállítások** lapján lévő **Mentett mappák** kártya minden ilyen mappát felsorol a konténeren belüli útvonalával és a hoszt-útvonalával, pirossal jelölve.

Egy névtelen Docker-kötetet nem jelez. A Docker magától hoz létre ilyet egy olyan mappához, amelyet az image deklarál, de senki nem képezett le, 64 hexadecimális számjeggyel nevezi el, és a `/var/lib/docker/volumes` alatt tartja. Általában gyorsítótárat tartalmaz, ezért a mentés zöld marad, és a kártya csak felsorolja, megjegyzéssel, hogy névtelen.

- **Szeretnéd menteni azt az adatot.** Képezd le a mappát egy Host Data alatti hoszt-útvonalra, például egy mappára az appdata megosztásodban, és a következő mentés tartalmazni fogja.
- **Nincs rá szükséged.** Add hozzá a két útvonalának egyikét sorként a **Kizárási minták** kártyán ugyanazon a lapon: a konténeren belüli útvonalat, például `/cache`, vagy a hoszt-útvonalat, amelyet a figyelmeztetés megnevez, például `/var/lib/docker/volumes/jellyfin_cache/_data`. A következő mentéstől a figyelmeztetés kihagyja azt a mappát, és a mező alatti előnézet megmondja, miért. Egy fölötte lévő mappa sora is működik, egy puszta név vagy a `/` nem.

## Egy mentés Elhalasztva állapotban áll {#deferred-backups}

A konténer alkalmazása foglalt volt, ezért a mentése vár, és magától újra próbálkozik. A tevékenységnapló sora megmondja, miért, és mikor lesz a következő próba. A konténer **Beállítások** lapján lévő **Ha az alkalmazás foglalt** kártya ugyanezt mondja, és hogy mikor ér véget a várakozás. Nem kell tenned semmit. Az azonnali mentéshez használd a **Biztonsági mentés most** gombot: a magad indította mentés soha nem vár, a 75-ös kilépési kód miatt sem.

- **A saját ellenőrzésed.** Add meg, hogy a **Biztonsági mentés előtti parancs** 75-ös kóddal lépjen ki, amíg az alkalmazás foglalt, például `[ "$(cat /tmp/players)" = 0 ] || exit 75`. Minden más, 0-tól eltérő kód a korábbiak szerint elbuktatja a mentést, így egy elromlott szkript továbbra is megjelenik a **Figyelmet igényel** alatt.
- **A Plexet, Jellyfint vagy Emby-t soha nem kérdezi meg.** A BombVault előbb a konténer saját címén kérdezi meg a szervert, és ha az nem válaszol, curlt vagy wgetet futtat a konténeren belül. Az az image, amelyben egyik sincs, nem kérdezhető meg önmagán át. A napló ekkor ezt megnevezi. Bekapcsolt **Mentés, ha a szerver nem válaszol** mellett az ilyen mentés mégis lefut, kikapcsolva vár.
- **A Jellyfin vagy az Emby visszautasítja.** Ellenőrizd az **API-kulcsot**. A szerver vezérlőpultján az API-kulcsok alatt hozod létre.
- **A Plex visszautasítja.** A BombVault a tokent a `/config` helyre csatolt mappa `Preferences.xml` fájljából olvassa. Egy soha be nem jelentkezett szervernek nincs ilyen, és csak ott válaszol, ahol token nélkül is engedi a hozzáférést.
- **Egy megállított film egész éjjel várakoztatja a mentést.** Állítsd be a **Szüneteltetett munkamenetek kihagyása ennyi után (perc)** értékét, például 30-ra.
- **A forgalomellenőrzés sosem engedi át a mentést.** Emeld a **Foglalt ettől (KiB/s)** értékét afölé, amit a konténer küld, amíg senki nem használja. Közvetlenül a BombVault indulása után az ellenőrzés megvárja, amíg a **Csendes ennyi ideig (perc)** percig figyelte a konténert.
- **A mentés sosem fut éjjel.** Beállított időablak mellett a próbák csak azon belül történnek. Egy olyan várakozás, amely az ablakon kívül éri el a határidejét, a határidőnél mégis véget ér.

## Egy nagy mentést memóriahiány miatt leállítottak {#a-large-backup-was-killed-for-running-out-of-memory}

Állítsd be a **Memóriakorlát biztonsági mentésenként (MiB, 0 = nincs)** értéket a **Teljesítmény** kártyán a **Beállítások, Rendszer** alatt, jóval azalatt, amit a konténer használhat. A korlát közelében a restic gyakrabban szabadít fel memóriát, ami valamennyi sebességbe kerül, így a mentés lelassul ahelyett, hogy leállítanák.

## Egy visszaállítás megszakadt, mielőtt elindult volna {#a-restore-aborted-before-it-started}

Mielőtt bármi leállna vagy törlődne, a visszaállítás egy előzetes ütközésellenőrzést futtat: megnézi, hogy a konténer statikus IP-címe és közzétett hoszt-portjai szabadok-e. Ha egy másik konténer már foglalja valamelyiket, érthető, teendőt adó üzenettel megszakad, ahelyett hogy félkész visszaállítást hagyna. Szabadítsd fel az ütköző portot vagy IP-t, majd próbáld újra.

Ha a konténer hálózata nem létezik, például mert a TrueNAS-alkalmazását törölték, a visszaállítás-ellenőrzés megnevezi a hálózatot, és a visszaállítás elutasít, mielőtt bármit megváltoztatna. Telepítsd újra az alkalmazást a TrueNAS-ban ugyanazzal a névvel, ami újra létrehozza a hálózatot, majd állítsd vissza az adatait.

## Egy egyszerű export meghiúsult ahelyett, hogy fájlt írt volna {#a-plain-export-failed-instead-of-writing-a-file}

Ha az **Exportok titkosítása age-dzsel** be van kapcsolva a **Beállítások, Rendszer** alatt, de nincs érvényes címzett megadva, az export egyértelmű hibával meghiúsul ahelyett, hogy sima szöveget írna. Adj hozzá érvényes címzettet (egy age nyilvános kulcsot vagy egy SSH nyilvános kulcsot), vagy kapcsold ki a titkosítást, ha az exportnak szándékosan sima szövegnek kell lennie. Lásd: [Sima exportok](backups.md#exports).

## Egy adatbázis-dump sikertelen volt {#a-database-dump-failed}

Egy sikertelen dump soha nem buktatja el a körülötte lévő mentést. Saját sikertelen futásként van rögzítve, és az ok megnevezi, mit kell javítani.

- **A bejelentkezést elutasították.** A dump a konténer saját jelszóváltozóival jelentkezik be (`POSTGRES_PASSWORD`, `MARIADB_ROOT_PASSWORD`, `MYSQL_ROOT_PASSWORD` vagy a `_FILE` változataik). Ellenőrizd őket az adatbázis-konténeren. Egy `_FILE` változó, amely olyan titokra mutat, amelyet a konténer saját felhasználója nem tud elolvasni, ugyanígy hibázik.
- **Hiányzó jogosultságok.** Véletlenszerű root jelszó mellett a dump csak az alkalmazás felhasználójaként tud bejelentkezni, így csak azt az egy adatbázist éri el, és a MySQL 8.4 és újabb akár teljesen el is utasíthatja. Adj a konténernek valódi root jelszót, vagy kapcsold ki rá a dumpot.
- **A rendszertáblákat frissíteni kell.** A MariaDB megtagadja a dumpot, ha a rendszertáblái egy régebbi verzióból származnak (1558-as hiba). Add hozzá a `MARIADB_AUTO_UPGRADE=1` változót, és indítsd újra a konténert, vagy futtasd egyszer a `mariadb-upgrade` parancsot benne.
- **Nincs dump-eszköz.** Egy karcsú vagy saját építésű image, amelyben nincs `pg_dump`, `mysqldump` vagy `mariadb-dump`, nem dumpolható. Használd a hivatalos image-et, vagy kapcsold ki a dumpot.
- **Időkorlát.** A dump a `DB_DUMP_MAX_HOURS` korlátot kapja (alapból 6), a körülötte lévő mentés a `BACKUP_MAX_HOURS` korlátot, és az a dump, amely nem halad tovább, a `BACKUP_STALL_HOURS` után le van vágva. Az utóbbi szokásos oka egy zár, amelyet az alkalmazás tart. Emeld azt a korlátot, amely működésbe lépett, vagy dumpolj, amikor az alkalmazás nyugalomban van.
- **A konténer szünetel vagy újraindul.** A dump a futó szerverrel beszél. Ha a konténer folyton újraindul, a saját naplója megmondja, miért.
- **Egy sérült dumpot nem sikerült eltávolítani.** Azt a dumpot, amelyet a BombVault nem tudott befejezni, ismét törli. Ha ez a törlés meghiúsul, a dump sérültként megjelölve a listában marad, és ott törölheted.

## Egy import sikertelen volt {#an-import-failed}

Az import leállítja a konténert, félretolja az adatmappáját, és hagyja, hogy az image újat hozzon létre a helyén. Ha egy lépés az import előtt hibázik, a régi mappa automatikusan visszakerül. Ha maga az import hibázik, a konténer megtartja a friss mappát, a régi pedig mellette marad `<data folder>.bombvault-before-import-<timestamp>` néven. A futás hibaüzenete megnevezi a pontos útvonalat.

Egy önálló adatkészlet adatmappáját, mint TrueNAS-on, nem lehet átnevezni. Az import ilyenkor a mappát a helyén hagyja, és a tartalmát a mellette lévő megtartott mappába helyezi át, ezért a fölötte lévő adatkészletnek helyre van szüksége hozzá. Az import ezt ellenőrzi, mielőtt leállítaná a konténert.

Kézi visszatételhez: állítsd le a konténert, nevezd át az aktuális adatmappát, hogy ne legyen útban, nevezd vissza a megtartott mappát az eredeti nevére, és indítsd el a konténert. Egy adatkészletnél a tartalmat mozgasd, ne a mappákat nevezd át. Unraidon a fájlkezelő ezt a Shares fülről elvégzi.

## Egy ZFS-adatkészlet mentése hibára futott vagy kihagyott egy adatkészletet {#zfs-datasets}

Minden problémához tartozik egy zárójeles okkód, és a [ZFS-adatkészletek](zfs-datasets.md#reason-codes) oldal mindet felsorolja a javítással. A három leggyakoribb:

- **`snapshot-loop`**: a pillanatkép nem jutott el a BombVaulthoz, mert a Host Data nem engedi át az új csatolásokat. Szerkeszd a konténert, állítsd a Host Data Access Mode értékét Read/Write - Slave módra, és indítsd újra a BombVaultot.
- **`key-not-loaded`**: egy titkosított adatkészletet, amelynek a kulcsa nincs betöltve, kihagy. Töltsd be a kulcsot a `zfs load-key` paranccsal, és csatold az adatkészletet. A következő mentés tartalmazni fogja.
- **`ssh-auth`**: a szerver visszautasította a BombVault kulcsát. A **ZFS-kapcsolat ehhez a kiszolgálóhoz**, amelyet a **Kapcsolat megtekintése** nyit meg a Biztonsági mentések oldalon, megmutatja a parancsot, amely engedélyezi. Futtasd egyszer a szerveren.

## Egy elem „Tanul N/10” állapotban marad {#an-entry-stays-at-learning-n10}

A legtöbb anomália-ellenőrzés egy elem 10 sikeres mentése után indul, és a számlálás újraindul a **Jelölés várhatóként** után és az elem kijelölésének módosulása után. Az ütemezés nélküli elem nem tanul, egy appdata nélküli konténernek pedig nincs miből tanulnia, amit a jelvénye is mutat. A **Tanulási folyamat** az Anomáliák oldalon elemenként tíz részből álló gyűrűt mutat.

## A megőrzés nem törli többé egy elem régi mentéseit {#retention-stopped-deleting-old-backups-of-one-entry}

Egy nyitott kritikus anomália tartja vissza őket: az elem forrása szinte üres, élesen összezsugorodott, vagy egy mentés az adatainak nagy részét újra eltárolta. Nyisd meg az anomáliát az elem jelvényéről. Ha adat hiányzik vagy titkosítva lett, előbb állítsd vissza a hivatkozott utolsó jó mentésből. Aztán nyugtázd az anomáliát, vagy jelöld várhatónak, ha a változás a te műved volt, és a következő futás a szokásos módon nyes. A megőrzési előnézet az ilyen elemet megtartottként jelöli. Egy ZFS-elemnél csak az anomáliában megnevezett adatkészlet tartja meg a régi mentéseit, a fa többi adatkészletét a szokásos módon nyesi.

## A kézi nyesés szerint néhány elemet megtartott {#manual-prune-says-some-entries-were-kept}

Ugyanaz az ok: a nyesés békén hagyja az ilyen anomáliával rendelkező elem régi mentéseit, és az üzenetében megnevezi az elemet. Minden mást a szokásos módon nyes.

## Egy mentés az „on no mounted pool or share” hibával hiúsul meg {#no-mounted-pool}

Unraidon a `/mnt` a memóriában él, így egy ottani mappa, amelyre nincs pool csatolva, a következő újraindításkor eltűnik, a visszatérő pool pedig eltakarja. A BombVault nem ír oda mentést vagy flash ZIP-et, akkor sem, ha egy korábbi írásból már ott van egy tároló abban a mappában. Csatold a poolt, és futtasd újra a mentést.

## Az előzmények importja szerint egy tárolót nem sikerült beolvasni {#history-import-says-a-repository-could-not-be-read}

A frissítés után a BombVault egyszer beolvassa a korábbi mentések méretét minden tárolóból. Azt a tárolót, amelyet ekkor nem ért el, például egy leállt telephelyen kívüli célt vagy egy nem csatolt megosztást, a **Beállítások, Ellenőrzés** alatti **Anomáliák** kártya megszámolja, és naponta egyszer újra megpróbálja. Az elemei addig az új mentésekből tanulnak.

## A lemezterület-figyelmeztetés nem egyezik az Unraid irányítópulttal {#the-disk-space-warning-does-not-match-the-unraid-dashboard}

Az Unraid felhasználói megosztásán (`/mnt/user`) a szabad hely az egész tömbé, nem egyetlen lemezé. Azokat a távoli célokat, amelyeket a BombVault az rclone-on át ér el, például SFTP, a Hetzner Storage Box, SMB, WebDAV vagy egy felhős meghajtó, akkor méri, ha a túloldal jelenti a szabad helyét, és ugyanez igaz a kézzel beírt `sftp:` címre. Az S3-, B2- és REST-tárolóknak nincs adata: a céljuk azt írja, hogy **A méret ismeretlen**, és az **Anomáliák** kártya mérés nélkülinek sorolja fel őket.

## Egy MI-asszisztens nem tud csatlakozni {#an-ai-assistant-cannot-connect}

Az [MCP-kiszolgáló oldal](mcp.md#troubleshooting) felsorolja, mit jelent az MCP-végpont minden állapotkódja és minden elutasítása, és mit tegyél velük.

## A konténer folyamatosan újraindul vagy egészségtelennek tűnik {#the-container-keeps-restarting-or-looks-unhealthy}

A BombVault a saját `/api/health` végpontjából jelent egészséges vagy egészségtelen állapotot. Egy automatikus javító eszköz (például az Autoheal) automatikusan újraindíthatja, ha a motor valaha beragad. Nézd meg a konténer naplóját és a **Hosztintegráció** kártyát a mögöttes okért.

## Egy cél nem hajlandó elmenteni magát {#location-refused}

A BombVault ellenőriz egy célt, mielőtt elmenti, és az elutasítás egy mondatban megmondja az okot. A szokásosak:

- **„Ez a hely egy másik tárolón vagy célon belül van, vagy tartalmaz egyet.”** Egy tárolón belüli tároló mindkettőt tönkretenné. Az üzenet megnevezi a másik helyet. Válassz másik mappát, vagy előbb távolítsd el azt.
- **Ez egy tároló, amelyet te állítottál be.** A hely a **Tárolók** alatti sorok egyike, vagy egy ilyen belsejében vagy körül van. Válassz másik helyet, vagy előbb távolítsd el azt a tárolót.
- **Ugyanaz a hely, mint egy szakasz saját mappája.** Egy szakasz saját mappájára történő másolás minden futáskor önmagára másolná a mentéseit, ezért elutasítja. Válassz másik helyet, vagy lásd: [Egy szakasz saját mappája](offsite-recovery.md#own-folder).
- **„Ez nem ennek a szakasznak a telephelyen kívüli célja”** egy eltávolított célra mutató hivatkozásból jött. Egy aktuális verzió ezeket az induláskor és egy importnál törli. Ha még mindig látod, frissítsd a BombVaultot.
- **Egy teljes hoszt-útvonal**, például `/mnt/user/backups`. A BombVault konténerben fut, ezért a Host Data alatti útvonalat kéri, `user/backups`, és az üzenet megnevezi azt, amelyet be kell írni.

## Egy távoli cél azt mondja, hogy a tárolója eltűnt {#repository-gone}

Az a cél, amely mentéseket tartott, és hirtelen nem tart tárolót, nem kap magától újat és üreset, mert az elrejtené a valódit, amint egy lemez visszatér. Általában egy rest-server nem csatolódott lemeze az ok, ezért csatold újra a lemezt, és futtasd újra a mentést vagy a másolást. Csak ha a régi tárolót szándékosan ürítetted ki vagy törölted, nyomd meg az **Új tároló létrehozása itt** gombot a cél oldalán. Ez a cél minden szakaszánál egyszerre újrakezdi.

## Egy felhős bejelentkezés lejárt {#cloud-sign-in-expired}

A teszt vagy a következő másolat ekkor azt mondja, hogy a bejelentkezés lejárt. A **Kapcsolat módosítása** a cél oldalán újra bejelentkeztet.

- **Box, Jottacloud és mások egyszeri frissítő tokenekkel:** a BombVault megtartja azt a tokent, amelyet az rclone frissített, újraindításokon át és a helyreállítási csomagban is. Egy régebbi beállításmentésből történő visszaállítás után a benne lévő token lejárhatott, ezért jelentkezz be még egyszer. A tároló úgy marad, ahogy van.
- **iCloud Drive:** az Apple 30 nap után megszünteti a bejelentkezést. Új megbízhatósági tokenre és sütikre van szüksége az `rclone config` parancsból.
- **OneDrive:** a több meghajtós bejelentkezés megkérdezi, melyiket használja, ezért a választás nélkül hagyott cél addig használhatatlan, amíg nem választasz. A OneDrive Business és a Google Drive esetén lásd a nyesett adatokról szóló megjegyzést a [Célok](offsite-recovery.md#destinations) alatt.

## „Az utolsó mentés telephelyen kívüli másolata még fut” {#copy-still-running}

Az előtte lévő mentés befejeződött, és a másolata még úton van, ami blokkolja a szakasz új mentését. Várd meg, amíg az állapotfej már nem azt írja, hogy **Telephelyen kívüli másolás folyamatban**. A mentést indító gombok ugyanezt mondják, amíg várnak. Egy új helyre az első másolás órákig tarthat, lásd az [S3-példát](offsite-recovery.md#example-s3).

## Egy ZFS-replika megáll, mert a cél újabb pillanatképeket tart {#replica-target-newer}

A cél olyan pillanatképeket tart, amelyek újabbak, mint a legújabb, amelyet ezzel a szerverrel közösen birtokol, például mert az adatkészletet itt visszagörgették, vagy a célon lévő másolatot megváltoztatták. A Replika kártya és a futási előzmények megnevezik a pillanatképet a **Roll the target back to** alatt. Futtasd a `zfs rollback -r` parancsot arra a pillanatképre a célon, minden adatkészletre, amelyet a futás megnevez, és nyomd meg a **Replikálás most** gombot. Lásd: [Mennyi ideig maradnak meg a pillanatképek](zfs-datasets.md#replica-retention).

## Egy Unraid-pillanatképes VM meghiúsul, és megnevez egy fájlt {#vm-chain}

A BombVault követi minden lemez háttérláncát, és a benne lévő összes fájlt lementi. Az a lánc, amelyet nem lehet a végéig követni, elbuktatja a mentést ahelyett, hogy a lemez egy részét kihagyná, és az üzenet megnevezi a fájlt. Ellenőrizd, hogy a fájl létezik a `/mnt` alatt, és a pillanatkép még szerepel a VM-kezelőben. A Host Data-n kívüli fájl egyáltalán nem olvasható. Lásd: [Unraid-pillanatképes VM-ek](vm-backup-ssh-setup.md#unraid-snapshots).

## A telephelyen kívüli másolatok egy újraépítés után a döntésedre várnak {#copies-paused}

A **Biztonsági mentések felfedezése** vagy a visszaállított beállítások nélküli Vészvarázsló után, vagy ha egy szakasz ennél az adatbázisnál régebbi saját mentéseket talál, annak a szakasznak a telephelyen kívüli másolatai várnak, mert a BombVault már nem tudja, mely elemeket hagytad ki. A **Figyelmet igényel** és a cél oldala rákérdez. Nyomd meg az **Átvétel a szerver saját mentéseiként** gombot a folytatáshoz, vagy az **Előbb a szabályok ellenőrzése** gombot, hogy előtte megnézd a szakasz szabályait. Egy beállításfájl importálása nem hozza meg helyetted a döntést. Lásd: [Újraépítés után](offsite-recovery.md#after-a-rebuild).

## Az értesítések nem érkeznek meg {#notifications-do-not-arrive}

Kezdd a **Teszt küldése** gombbal a **Beállítások, Értesítések** alatt. A kártya alatti sor megmondja, mi történt.

- **Egyáltalán nem megy ki semmi.** Ellenőrizd az **Értesítés** beállítást. A **Soha** mellett nem megy ki üzenet, csak egy Healthchecks URL pingjei, a **Csak hiba esetén** mellett pedig egy sikeres futás csendben marad. Bekapcsolt **Ütemezett futások összegzése** mellett az ütemezett futás a végén egy összegzést küld.
- **Új cím.** Miután megváltoztatod a Matrix, a Gotify vagy az e-mail szerverét, a mező újra kéri a tokent vagy a jelszót, és az új cím csak akkor mentődik, ha beírod. A BombVault nem küldi el a régi titkot egy új helyre.
- **A Gotify** azt mondja, hogy a szerver visszautasította az alkalmazás-tokent: másold ki újra a Gotify alkalmazásából. A **Szerver URL** az alapcím, a `/message` nélkül.
- **Webhook:** ellenőrizd, hogy a **Webhook formátum** illik-e a szolgáltatáshoz. Egy Discord-cím a **Discord** formátumot kéri, az ntfy pedig az **ntfy**-t.
- Az **Unraid-értesítések** a Gazdagép SSH-kapcsolaton futnak, ezért azt teszteld előbb a **Beállítások, Integrációk** alatt. Más platformon kikapcsolva maradnak, és a kártya ezt jelzi.
- **Egy értesítő e-mail sosem érkezik meg.** Ellenőrizd a **Titkosítás** értékét a porthoz képest, általában **STARTTLS** a 587-en vagy **TLS (implicit)** a 465-ön.

A Healthchecks pingek nem függnek ezektől, lásd: [Healthchecks](notifications.md#healthchecks).

## Az Android-alkalmazás nem tud megnyitni egy szervert {#android-cannot-open}

A kezdőképernyő a szerver kártyáján megmondja, miért:

- **Nincs kapcsolat:** sem a szerver címe, sem a relay nem válaszol. A **Nincs relay** esetén az alkalmazás egy szervert csak a saját címén ér el.
- **Ellenőrizd a tanúsítványt:** a szerver más tanúsítványt mutat, mint amelyiket elfogadtál. Az alkalmazás csak az új elfogadása után nyitja meg, ezért győződj meg róla, hogy a változás a te műved.
- **Bejelentkezés szükséges:** a címével hozzáadott szerver a jelszót kéri. Lásd még: [Egy telefon a bejelentkezési oldalra érkezik](#a-phone-lands-on-the-login-page).
- **A kulcs megváltozott:** a kezdőképernyő az új kulcs ujjlenyomatát mutatja, és az oldal csak az elfogadása után nyílik meg. Lásd: [Otthonról távol](android.md#away).

## Egy másik felhasználó nem tudja olvasni a helyi tárolót {#repository-permissions}

A helyi tároló olvasható marad más felhasználók számára, például egy SMB-n át szinkronizáló felhasználónak. A 10.4.1 előtt a nyesés, címkézés, lehívás, szakaszáthelyezés, törlés vagy stackmentés által írt fájlok a következő éjszakáig olvashatatlanok maradhattak. Frissítsd a BombVaultot, és azonnal olvashatók. Az önmentés tárolója szándékosan csak a root számára olvasható marad, mert a BombVault beállításait tartalmazza.

## Még mindig elakadtál? {#still-stuck}

- Olvasd el a teljes [Konfiguráció](configuration.md) és [Telephelyen kívüli mentés és helyreállítás](offsite-recovery.md) oldalt.
- A **Hibajelentés** a **Súgó** kártyán a **Beállítások, Információ** alatt letölt egy ZIP-et egy GitHub-hibajegyhez: a hosztellenőrzést, a titkoktól megtisztított konfigurációdat, a legutóbbi futásokat, azt, mi van következőnek ütemezve, és a legutóbbi naplót a konténer utolsó indulása óta. Tartalmazza az egyes adatbázisok utolsó dumpját, a ZFS-elemeket a csatolásaikkal, a nyitott anomáliákat és azt, hány MCP-kulcs létezik. A jelszavakat, tokeneket és az rclone konfigurációt eltávolítja, és a csomag a saját jegyzékében ezt jelzi. Belépési jelszó kell hozzá. Egy olyan összeomláshoz, amely újraindította a konténert, nézd meg a `docker logs` kimenetét.
- Kérdezz az [Unraid támogatási szálon](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/).
- Nyiss egy [GitHub-hibajegyet](https://github.com/junkerderprovinz/bombvault/issues).
