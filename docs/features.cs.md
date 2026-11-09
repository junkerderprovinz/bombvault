# Funkce

Stručný katalog toho, co BombVault umí. Každý řádek odkazuje na stránku, která to vysvětluje.

## Rozsah zálohování {#backup-scope}

- **Kontejnery**: složka appdata a definice kontejneru, s výběrem složek pro každý kontejner. [Zálohy a položky](backups.md#what-is-saved)
- **VM**: obrazy disků, XML a UEFI NVRAM přes SSH, s řádným vypnutím, živým snímkem nebo jen změněnými bloky. VM se snapshoty Unraidu a disky na zvolech jsou pokryté celé. [Zálohy VM](vm-backup-ssh-setup.md)
- **Flash**: celá flashka USB s Unraidem, s plánovaným exportem do zip. [Zálohy a položky](backups.md#settings-tab)
- **Autozáloha**: vlastní nastavení BombVaultu a přihlašovací údaje. [Nouzový průvodce](offsite-recovery.md#emergency)
- **Složky**: libovolná složka na serveru jako sada složek. [Přidání složky](backups.md#add)
- **Datové sady ZFS**: strom datových sad čtený z jednoho snapshotu, s obnovou jednotlivých souborů a volitelnou replikou na druhém serveru ZFS. [Datové sady ZFS](zfs-datasets.md)
- **Výpisy databází**: PostgreSQL, MySQL a MariaDB, vypsané z běžícího serveru do vlastního bodu obnovy. [Výpisy databází](backups.md#database-dumps)

## Zálohy a stránky položek {#backups}

- **Jeden seznam** s dlaždicemi, **Filtry**, **Řazení** a **Hledat** a s posledními čtrnácti běhy každého řádku jako pruhem. [Seznam](backups.md#list)
- **Výběr jako ve správci souborů** a záloha nebo pozastavení výběru na serveru. [Výběr více položek](backups.md#select)
- **Stránka pro každou položku** se záložkami **Zálohy**, **Nastavení** a **Ověření**. [Stránka položky](backups.md#entry-page)
- **Štítky a porovnání** snapshotů a **Export** jako tar, zip nebo repozitář resticu. [Záložka Zálohy](backups.md#backups-tab)
- **Vzory vyloučení** s živým náhledem a doporučená vyloučení pro známé aplikace. [Vzory vyloučení](backups.md#excludes)
- **Příkazy před a po** a **Zastavit ostatní kontejnery** s restartem podle zdraví v pořadí závislostí. [Příkazy a jiné kontejnery](backups.md#commands)
- **Když je aplikace zaneprázdněná**: plánovaná záloha počká na Plex, Jellyfin, Emby, tvou vlastní kontrolu nebo klidný provoz a sama to zkusí znovu. [Když je aplikace zaneprázdněná](backups.md#app-busy)
- **Snímek místo zastavení**: kontejner na ZFS stojí jen pár vteřin. [Plány](schedules.md#snapshot-instead-of-stop)
- **Prosté exporty**, volitelně zapečetěné pomocí age. [Prosté exporty](backups.md#exports)

## Obnova {#restore}

- **Jedna stránka a jedno okno** pro každý druh, s předvybranou doporučenou zálohou. [Obnova](restore.md)
- **Vedle originálu** ve výchozím stavu a **Nahradit originál** nejdřív zazálohuje současný stav. [Okno obnovy](restore.md#window)
- **Jednotlivé soubory** do složky nebo zpět na jejich místo. [Dostat zpět jeden soubor](restore.md#example-single-file)
- **Více položek najednou**, každá ze své doporučené zálohy nebo z jednoho okamžiku. [Obnova více položek](restore.md#several)
- **Celé stacky Compose**, spuštěné v pořadí závislostí. [Obnova stacku](restore.md#stacks)
- **Kontejnery a VM se znovu vytvoří** na image, na kterém běžely, s GPU, limity a linky. [Co se vrací](restore.md#kinds)
- **Kontroly před obnovou** a seznam toho, co změní. [Okno obnovy](restore.md#window)
- **Flash jako ZIP** a po jednom pluginu zpět do běžící flashky. [Co se vrací](restore.md#kinds)
- **ZFS** do datové sady s bezpečnostním snapshotem, do složky nebo do nové datové sady s jejími vlastnostmi. [Datové sady ZFS](zfs-datasets.md#restore)
- **Nouzový průvodce** pro ztracený server, se sadou pro obnovu, přes skupinu nebo napsaným klíčem. [Nouzový průvodce](offsite-recovery.md#emergency)
- **Ze staré složky se zálohami**, **Import z pluginu Appdata.Backup** a **Úlohy z Duplicati nebo Duplicacy**, kde se jejich zálohovací úlohy převedou na sady složek. [Další zdroje](restore.md#other-sources)

## Úložiště a plánování {#storage-scheduling}

- **Cíle** všech druhů: místní složky, SMB, WebDAV, SFTP, rest-server, S3, cloudové disky a libovolný remote rclone. [Cíle](offsite-recovery.md#destinations)
- **Přihlašovací údaje**: jeden seznam s výchozím. [Přihlašovací údaje](offsite-recovery.md#sign-ins)
- **Vlastní složky** na tomto serveru nebo v cíli. [Vlastní složka sekce](offsite-recovery.md#own-folder)
- **3-2-1 jako tok**, kde je každý řádek přepínač, a **umístění podle položky**. [Umístění podle položky](offsite-recovery.md#placement)
- **Kopie mimo lokalitu** do několika cílů na sekci, s limity šířky pásma, kompresí, studenými třídami úložiště a zpomalením, zatímco streamuje Plex. [Kopie mimo lokalitu](offsite-recovery.md#off-site-copies)
- **Přímé repozitáře**, vypsané pod **Zálohuje se rovnou sem**. [Přímé repozitáře](offsite-recovery.md#direct-repositories)
- **Uchovávání jako časová osa**: **Krátké**, **Vyvážené**, **Dlouhé** nebo **Vlastní hodnoty** pro každý cíl, s pravidly podle sekcí. [Jak dlouho uchovávat](offsite-recovery.md#keep)
- **Přesun historie sekce** a **Odebrat cíl** se zálohami nebo bez nich. [Přesun a odebrání](offsite-recovery.md#move-remove)
- **Plány, které se čtou jako věty**, pás **Týden**, úplná záloha, plány podle sekcí a vlastní plán pro každou položku. [Plány](schedules.md)
- **Dohnání zmeškaných záloh**, ruční pořadí a aktualizace po úspěšné záloze. [Plány](schedules.md)

## Přehled, ověřování a sledování {#insight-verification-monitoring}

- **Přehled**: hlavička stavu, **Vyžaduje pozornost**, **Stav ochrany**, předpověď úložiště, historie a volitelné karty. [Přehled](overview.md)
- **Rozpoznávání anomálií** u každé položky, které také drží staré zálohy, když se zdroj zhroutí. [Anomálie](anomalies.md)
- **Kontrola obnovy** po první záloze a **Test spuštění** v izolované kopii. [Záložka Ověření](backups.md#verify-tab)
- **Plánované kontroly a zkušební obnovy** z kopie mimo lokalitu. [Cvičení DR](offsite-recovery.md#dr-drills)
- **Velikost podle složek** a proč byla záloha pomalá. [Co ti běh řekne](backups.md#run-notes)
- **Oznámení**: webhook, Gotify, Matrix, e-mail, Apprise, Unraid a Healthchecks, s týdenním souhrnem a hlášením záloh po termínu. [Oznámení](notifications.md)
- **Prometheus**, zařízení Home Assistantu, API přes HTTP a mDNS. [API a integrace](api.md)
- **Hlášení chyby**: vyčištěné ZIP pro issue na GitHubu. [Řešení problémů](troubleshooting.md#still-stuck)

## Ochrana proti ransomwaru {#ransomware-protection}

- **Ochrana proti smazání (append-only)**, vynucená druhou stranou a prokázaná tamper testem. [Ochrana proti smazání](offsite-recovery.md#delete-protection-append-only)
- **Přijmout bez ochrany proti smazání** pro místo, které smazání odmítnout neumí. [Ochrana proti smazání](offsite-recovery.md#delete-protection-append-only)
- **Trvalé mazání záloh vyžaduje tvoje vlastní přihlášení**, jakmile je nastavené přihlašovací heslo, a spárovaný telefon to nedokáže. [Bezpečnostní model](configuration.md#security-model)
- **Sada pro obnovu**, volitelně zapečetěná pomocí age, a **strážce klíče** ve tvé skupině. [Sada pro obnovu](offsite-recovery.md#recovery-kit), [Strážce klíče](offsite-recovery.md#key-keeper)

## Více serverů {#several-servers}

- **Párování dvanácti slovy**, šifrované od konce ke konci, přímo nebo přes relay. [Párování instancí](offsite-recovery.md#pairing)
- **Stránka Instance** se skupinou jako sítí a rolemi **Přijímač**, **Stahovač**, **Server ZFS** a **Strážce klíče**. [Instance a role](offsite-recovery.md#roles)
- **Přijímací server** nastavený za tebe, v režimu append-only. [Přijímací server](offsite-recovery.md#receiving-server)

## Asistenti AI (MCP) {#mcp}

Vestavěný server MCP dovolí asistentovi číst stav záloh, pokrytí, historii běhů a body obnovy a s klíčem, který to povolí, spustit zálohu. Obnovy, mazání, čištění a nastavení zůstávají ve webovém rozhraní. [Server MCP](mcp.md)

## Aplikace a doplňky {#apps}

- **Aplikace pro Android**: všechny servery tvé skupiny v telefonu. [Aplikace pro Android](android.md)
- **BombVault Widget**: dlaždice na dashboardu Unraidu. [Aplikace](settings.md#apps)
- **Vložitelný protokol aktivity** pro dashboardy jako Homepage nebo Heimdall. [Widget pro dashboard](api.md#dashboard-widget)

## Další {#other}

- **42 jazyků**, tři motivy a popisky, barvy, rohy a animace podle tvého výběru. [Vzhled](settings.md#look)
- **Nastavení, která se ukládají sama**, s vyhledáváním přes všech čtrnáct dlaždic. [Nastavení](settings.md)
- **Novinky** po aktualizaci, se zelenými tečkami u toho, co se změnilo. [Info](settings.md#info)
- **HTTPS hned po instalaci** a healthcheck pro nástroje s automatickou opravou. [Konfigurace](configuration.md#network)
