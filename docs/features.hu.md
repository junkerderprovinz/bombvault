# Funkciók

Rövid katalógus arról, mit tud a BombVault. Minden sor arra az oldalra mutat, amely elmagyarázza.

## Mentési hatókör {#backup-scope}

- **Konténerek**: az appdata mappa és a konténer definíciója, konténerenként választható mappákkal. [Biztonsági mentések és elemek](backups.md#what-is-saved)
- **VM-ek**: lemezképek, XML és UEFI NVRAM SSH-n át, szabályos leállítással, élő pillanatképpel vagy csak a változott blokkokkal. Az Unraid-pillanatképes VM-ek és a zvolokon lévő lemezek teljes körűen le vannak fedve. [VM-mentések](vm-backup-ssh-setup.md)
- **Flash**: a teljes Unraid USB flash, ütemezett zip-exporttal. [Biztonsági mentések és elemek](backups.md#settings-tab)
- **Önmentés**: a BombVault saját beállításai és bejelentkezései. [Vészvarázsló](offsite-recovery.md#emergency)
- **Mappák**: a szerver bármelyik mappája mappakészletként. [Mappa hozzáadása](backups.md#add)
- **ZFS-adatkészletek**: egyetlen pillanatképből olvasott adatkészlet-fa, egyes fájlok visszaállíthatók, és opcionális replika egy második ZFS-szerveren. [ZFS-adatkészletek](zfs-datasets.md)
- **Adatbázis-mentések**: PostgreSQL, MySQL és MariaDB, a futó szerverről lementve egy saját visszaállítási pontba. [Adatbázis-mentések](backups.md#database-dumps)

## Mentések és elemoldalak {#backups}

- **Egy lista** csempékkel, **Szűrőkkel**, **Rendezéssel** és **Kereséssel**, soronként az utolsó tizennégy futással sávként. [A lista](backups.md#list)
- **Kijelölés, mint egy fájlkezelőben**, és egy kijelölés mentése vagy szüneteltetése a szerveren. [Több elem kijelölése](backups.md#select)
- **Elemenként egy oldal** a **Biztonsági mentések**, **Beállítások** és **Ellenőrzés** lappal. [Egy elem oldala](backups.md#entry-page)
- **Címkék és összehasonlítás** a pillanatképekhez, és **Exportálás** tarként, zipként vagy restic-tárolóként. [Biztonsági mentések lap](backups.md#backups-tab)
- **Kizárási minták** élő előnézettel, és ajánlott kizárások jól ismert alkalmazásokhoz. [Kizárási minták](backups.md#excludes)
- **Parancsok előtte és utána**, és **Más konténerek leállítása** egészségfigyeléses újraindítással, függőségi sorrendben. [Parancsok és más konténerek](backups.md#commands)
- **Ha az alkalmazás foglalt**: az ütemezett mentés vár a Plexre, a Jellyfinre, az Emby-re, a saját ellenőrzésedre vagy a csendes forgalomra, és magától újra próbálkozik. [Ha az alkalmazás foglalt](backups.md#app-busy)
- **Pillanatkép leállítás helyett**: a ZFS-en lévő konténer csak néhány másodpercre áll le. [Ütemezések](schedules.md#snapshot-instead-of-stop)
- **Sima exportok**, igény szerint age-dzsel lezárva. [Sima exportok](backups.md#exports)

## Visszaállítás {#restore}

- **Egy oldal és egy ablak** minden fajtához, az ajánlott mentés előre kiválasztva. [Visszaállítás](restore.md)
- **Az eredeti mellé** alapértelmezés szerint, az **Eredeti cseréje** pedig előbb lementi a jelenlegi állapotot. [A visszaállítási ablak](restore.md#window)
- **Egyes fájlok** egy mappába vagy vissza a helyükre. [Egy fájl visszahozása](restore.md#example-single-file)
- **Több elem egyszerre**, mindegyik az ajánlott mentéséből vagy egyetlen időpontból. [Több elem egyszerre](restore.md#several)
- **Teljes Compose-stackek**, függőségi sorrendben indítva. [Egy stack visszaállítása](restore.md#stacks)
- **A konténerek és VM-ek újra létrejönnek** azon az image-en, amelyen futottak, GPU-val, korlátokkal és linkekkel. [Mi tér vissza](restore.md#kinds)
- **Ellenőrzések a visszaállítás előtt**, és a változtatásainak listája. [A visszaállítási ablak](restore.md#window)
- **A flash ZIP-ként**, és egyszerre egy bővítmény vissza a futó flash-re. [Mi tér vissza](restore.md#kinds)
- **ZFS** az adatkészletbe biztonsági pillanatképpel, egy mappába vagy egy új adatkészletbe a tulajdonságaival. [ZFS-adatkészletek](zfs-datasets.md#restore)
- **Vészvarázsló** egy elveszett szerverhez, a helyreállítási csomaggal, a csoporton át vagy a beírt kulccsal. [Vészvarázsló](offsite-recovery.md#emergency)
- **Régi mentési mappából**, **Importálás az Appdata.Backup bővítményből**, és **Feladatok a Duplicatiból vagy a Duplicacyből**, amely a mentési feladataikból mappakészleteket készít. [Más források](restore.md#other-sources)

## Tárolás és ütemezés {#storage-scheduling}

- **Célok** minden fajtából: helyi mappák, SMB, WebDAV, SFTP, rest-server, S3, felhős meghajtók és bármely rclone remote. [Célok](offsite-recovery.md#destinations)
- **Bejelentkezések**: egy lista egy alapértelmezettel. [Bejelentkezések](offsite-recovery.md#sign-ins)
- **Saját mappák** ezen a szerveren vagy egy célon. [Egy szakasz saját mappája](offsite-recovery.md#own-folder)
- **3-2-1 folyamatként**, ahol minden sor egy kapcsoló, és **elhelyezés elemenként**. [Elhelyezés elemenként](offsite-recovery.md#placement)
- **Telephelyen kívüli másolatok** szakaszonként több célra, sávszélesség-korlátokkal, tömörítéssel, hideg tárolási osztályokkal és lassítással, amíg a Plex streamel. [Telephelyen kívüli másolatok](offsite-recovery.md#off-site-copies)
- **Közvetlen tárolók**, az **Ide ment közvetlenül** alatt felsorolva. [Közvetlen tárolók](offsite-recovery.md#direct-repositories)
- **Megőrzés idővonalként**: **Rövid**, **Kiegyensúlyozott**, **Hosszú** vagy **Saját értékek** célonként, szakaszonkénti szabályokkal. [Meddig őrizze meg](offsite-recovery.md#keep)
- **Egy szakasz előzményeinek áthelyezése**, és a **Cél eltávolítása** a mentéseivel vagy anélkül. [Költöztetés és eltávolítás](offsite-recovery.md#move-remove)
- **Mondatként olvasható ütemezések**, a **Hét** sáv, egy teljes mentés, szakaszonkénti tervek és saját terv elemenként. [Ütemezések](schedules.md)
- **Elmaradt mentések pótlása**, kézi sorrend és frissítések egy sikeres mentés után. [Ütemezések](schedules.md)

## Betekintés, ellenőrzés és figyelés {#insight-verification-monitoring}

- **Az áttekintés**: állapotfej, **Figyelmet igényel**, **Védelmi állapot**, tárhely-előrejelzés, előzmények és opcionális kártyák. [Áttekintés](overview.md)
- **Anomáliafelismerés** elemenként, amely a régi mentéseket is megtartja, ha egy forrás összeomlik. [Anomáliák](anomalies.md)
- **Visszaállítás-ellenőrzés** az első mentés után, és **Indítási teszt** egy elszigetelt másolatban. [Ellenőrzés lap](backups.md#verify-tab)
- **Ütemezett ellenőrzések és próba-visszaállítások** a telephelyen kívüli másolatból. [DR-próbák](offsite-recovery.md#dr-drills)
- **Méret mappánként**, és hogy miért volt lassú egy mentés. [Amit egy futás elmond](backups.md#run-notes)
- **Értesítések**: csatornalista az Unraid, ntfy, Gotify, Discord, Slack, Matrix, e-mail, Apprise, bármilyen webhook és Healthchecks számára, mindegyik saját szabállyal, heti összefoglalóval és késés-jelentéssel. [Értesítések](notifications.md)
- **Prometheus**, egy Home Assistant eszköz, egy HTTP API és mDNS. [API és integrációk](api.md)
- **Hibajelentés**: kitakart ZIP egy GitHub-hibajegyhez. [Hibaelhárítás](troubleshooting.md#still-stuck)

## Zsarolóvírus elleni védelem {#ransomware-protection}

- **Törlés elleni védelem (append-only)**, a túloldal kényszeríti ki, és manipulációs teszt bizonyítja. [Törlés elleni védelem](offsite-recovery.md#delete-protection-append-only)
- **Elfogadás törlés elleni védelem nélkül** olyan helyhez, amely nem tud visszautasítani egy törlést. [Törlés elleni védelem](offsite-recovery.md#delete-protection-append-only)
- **A mentések végleges törléséhez saját bejelentkezés kell**, ha van belépési jelszó, és párosított telefon nem teheti meg. [Biztonsági modell](configuration.md#security-model)
- **Helyreállítási csomag**, igény szerint age-dzsel lezárva, és egy **kulcsőrző** a csoportodban. [Helyreállítási csomag](offsite-recovery.md#recovery-kit), [Kulcsőrző](offsite-recovery.md#key-keeper)

## Több szerver {#several-servers}

- **Párosítás tizenkét szóval**, végponttól végpontig titkosítva, közvetlenül vagy relayen át. [Példányok párosítása](offsite-recovery.md#pairing)
- **A Példányok oldal**, a csoporttal hálóként, és a **Fogadó**, **Lehívó**, **ZFS-kiszolgáló** és **Kulcsőrző** szerepekkel. [Példányok és szerepek](offsite-recovery.md#roles)
- **Egy fogadó szerver** beállítva helyetted, append-only módban. [Fogadó szerver](offsite-recovery.md#receiving-server)
- **Egy közös tároló**, amelybe több szerver másol, így ami közös bennük, az csak egyszer tárolódik. Csak a tulajdonos szerver nyes. [Közös tároló](offsite-recovery.md#shared-repository)

## MI-asszisztensek (MCP) {#mcp}

Egy beépített MCP-szerver lehetővé teszi, hogy egy asszisztens elolvassa a mentések állapotát, lefedettségét, a futási előzményeket és a visszaállítási pontokat, és egy erre jogosító kulccsal mentést indítson. A visszaállítások, törlések, a prune és a beállítások a webes felületen maradnak. [MCP-kiszolgáló](mcp.md)

## Alkalmazások és társak {#apps}

- **Android-alkalmazás**: a csoportod összes szervere a telefonodon. [Android-alkalmazás](android.md)
- **BombVault Widget**: egy csempe az Unraid vezérlőpultján. [Alkalmazások](settings.md#apps)
- **Beágyazható tevékenységnapló** irányítópultokhoz, például a Homepage-hez vagy a Heimdallhoz. [Irányítópult-widget](api.md#dashboard-widget)

## Egyéb {#other}

- **42 nyelv**, három téma, valamint tetszés szerinti feliratok, színek, sarkok és animáció. [Megjelenés](settings.md#look)
- **Magától mentő beállítások**, az összes tizennégy csempén átívelő kereséssel. [Beállítások](settings.md)
- **Újdonságok** egy frissítés után, zöld pontokkal azon, ami változott. [Információ](settings.md#info)
- **HTTPS készen**, és egészségellenőrzés az automatikus javító eszközöknek. [Konfiguráció](configuration.md#network)
