# Řešení problémů

Krátké FAQ. Kompletní tabulku řešení problémů na straně hostitele pro VM přes SSH (permission-denied, ověření hostitelského klíče, chybějící proměnné šablony a další) najdeš v [průvodci Záloha VM přes SSH](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) na GitHubu.

## Něco není správně zapojeno

Přidej kartu **Integrace hostitele** do Přehledu pod **Přizpůsobit**. Kontrola integrace hostitele prozkoumá každé připojení a CLI (Docker socket, libvirt, restic, qemu-img, rclone) a nahlásí případné chybějící části a **Zkontrolovat nyní** ji spustí znovu. Začni zde, než budeš předpokládat chybu: chybějící připojení nebo nedosažitelný hostitel se objeví okamžitě.

## Nemohu se dostat k webovému rozhraní

BombVault obsluhuje HTTPS rovnou z krabice na portu `3443` (samopodepsaný certifikát), takže otevři `https://<your-unraid-ip>:3443`. Přijmi varování o samopodepsaném certifikátu, nebo umísti BombVault za reverzní proxy s vlastním certifikátem. Pokud běžíš s `HTTP_ONLY=true`, obsluhuje místo toho prosté HTTP na portu `3000` (určeno pro použití za proxy terminující TLS).

## Ztratil jsem svůj APP_KEY

`APP_KEY` odvozuje heslo k restic repozitáři. Bez něj, bez sady pro obnovu a bez úschovy klíče ve tvé skupině nelze šifrované zálohy obnovit. Proto Přehled ukazuje **Ulož si sadu pro obnovu**, dokud ji nestáhneš. Viz [Mimo lokalitu a obnova](offsite-recovery.md#recovery-kit). Vygeneruj klíč pomocí `openssl rand -hex 32` a ulož jej mimo server, dříve než se budeš spoléhat na jakoukoli zálohu.

## Zapomněl jsem přihlašovací heslo

Zvol **Zapomenuté heslo?** na přihlašovací stránce. BombVault zapíše kód o 12 znacích do logu svého kontejneru (na Unraidu: záložka Docker, ikona BombVault, **Logs**). Zadej ho s novým heslem do 10 minut. Reset také vypne dvoufázové ověření, odstraní všechny passkeys, zruší všechny API tokeny, klíče MCP a přihlášení OAuth, ukončí všechny ostatní relace a vypne párování telefonů. Passkeys přidej znovu v **Nastavení, Zabezpečení**, párování telefonů tam zase zapni a nové tokeny a klíče vytvoř tam, kde je ještě potřebuješ. Viz [Bezpečnostní model](configuration.md#login).

## Zálohy se zastavily a hlásí, že klíč už nejde otevřít

Tento server převzal v Nouzovém průvodci zálohovací klíč jiného serveru a `APP_KEY` se potom změnil. Převzatý klíč je zapečetěný starým APP_KEY, takže ho BombVault neotevře a raději nezálohuje nic, než aby zapisoval špatným klíčem. Pojmenuje ho **Vyžaduje pozornost** v Přehledu. Nastav APP_KEY zpět na původní hodnotu, nebo znovu nahraj sadu pro obnovu na stránce Obnova pod **Nouze**.

## Telefon skončí na přihlašovací stránce

Spárovaný telefon dostane relaci přes skupinu jen tehdy, když je v **Nastavení, Zabezpečení** zapnuto **Telefony se mohou spárovat s dvanácti slovy skupiny**. Reset hesla ho vypne. Aplikace starší než server také nedostane relaci, dokud se neaktualizuje. Server přidaný podle adresy se na heslo ptá vždy. Telefon odebraný ze stránky Instance se po dalším otevření aplikace vrátí vypnutý; abys ho pustil dovnitř, zapni v jeho podrobnostech v části **Sledování** volbu **Dotázat se na tuto instanci**.

## Záloha VM se nepřipojí

Záloha VM komunikuje s libvirt přes SSH, nikdy přes připojení.

- Potvrď, že SSH je povoleno na hostiteli a veřejný klíč BombVaultu je autorizovaný v `/root/.ssh/authorized_keys` (Nastavení, Integrace, SSH k hostiteli zobrazuje klíč a tlačítko **Otestovat připojení**).
- Na vlastní síti `br0.x` nastav `LIBVIRT_HOST` na svou LAN IP Unraidu (kontejner tam nemůže dosáhnout na hostitele přes `host.docker.internal`). Povol **Settings, Docker, Host access to custom networks**.
- Pokud jsi změnil SSH port Unraidu, nastav `LIBVIRT_SSH_PORT`, aby odpovídal.
- Na TrueNAS nastav `LIBVIRT_URI` s vlastním socketem libvirt z TrueNAS, dej aplikaci capability `SETPCAP` a přidej klíč uživateli root v **Credentials, Users** (viz [TrueNAS SCALE](getting-started.md#truenas)).
- Kompletní krok za krokem diagnóza (test dosažitelnosti, směrování VLAN, `Permission denied (publickey)`, `Host key verification failed`) je v [průvodci Záloha VM přes SSH](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md).

## Živý snímek VM neproběhl

Živé snímky potřebují qemu guest agent nainstalovaný ve VM a disk na `/mnt/cache` (nebo `/mnt/diskX`), nikoli `/mnt/user`. Na vypnuté VM se živý automaticky vrátí k šetrnému. Šetrná záloha VM vypne, zálohuje disky, poté ji znovu spustí, takže je vždy konzistentní. Čeká až deset minut, než se host vypne, a nikdy ho nevypíná násilím, takže host s Windows, který zrovna instaluje aktualizace, se nepřeruší v půlce. Host, který potom stále běží, zůstane nedotčen a tato záloha selže s uvedením důvodu.

## Záloha selhala s "repository is already locked"

Toto je obvykle osiřelý restic zámek zanechaný, když byl kontejner aktualizován nebo restartován uprostřed operace. BombVault detekuje prokazatelně osiřelý zámek, násilně jej vyčistí a jednou zopakuje, automaticky. Pokud přetrvává, otevři kartu **Zkontrolovat zálohy** v **Nastavení, Ověření** a zvol **Odemknout** pod **Více** na řádku postižené sekce k ručnímu vyčištění zaseklého zámku. Skutečný problém se stále objeví, místo aby byl skryt. Po restartu BombVault čeká, dokud takový zámek nebude deset minut bez obnovení. Restic, který ještě běží, třeba v druhém BombVaultu nad stejným repozitářem, svůj zámek obnovuje každých pět minut.

## Moje kopie mimo lokalitu neproběhla po záloze

Replikace mimo lokalitu je na základě nejlepší snahy záměrně, takže zádrhel mimo lokalitu nikdy nezhatí místní zálohu. Zkontroluj, kdy cíl kopíruje: jeho karta **Kopírovat** říká **Po každé záloze** nebo uvádí vlastní plán a **Nastavení, Plán, Kopie mimo lokalitu podle sekcí** může dát sekci vlastní plán. Použij **Replikovat nyní** na kartě sekce **Kopie mimo lokalitu** v **Nastavení, Cíle** pro běh na vyžádání a sleduj hlavičku stavu v Přehledu, která za běhu říká **Probíhá kopie mimo lokalitu**. Neúspěšná kopie se ukáže pod **Vyžaduje pozornost**.

## Certifikátu rest-serveru se nedůvěřuje

restic odmítne certifikát, který nezná, a chybová zpráva řekne, kam ho dát. Zkopíruj samopodepsaný certifikát rest-serveru, nebo certifikát autority, která ho podepsala, jako soubor PEM do složky `certs` v datové složce BombVaultu (`/config/certs` v kontejneru, `appdata/bombvault/certs` na Unraidu) a otestuj znovu. Restart není potřeba.

## Záloha hlásí, že data mimo Host Data nebyla zálohována {#data-outside-host-data}

Kontejner má data ve složce, kterou BombVault přes Host Data nepřečte. Záloha uloží definici a každou složku, ke které se dostane, obarví svůj řádek v protokolu aktivit oranžově a vyjmenuje složky, které vynechala. Dva případy jsou běžné: ixVolume na TrueNAS pod `/mnt/.ix-apps` a pojmenovaný svazek Dockeru pod `/var/lib/docker/volumes`, třeba svazek s databází ze stacku compose. Karta **Zálohované složky** na kartě **Nastavení** kontejneru ukazuje každou takovou složku s její cestou uvnitř kontejneru a cestou na hostiteli, označenou červeně.

Anonymní svazek Dockeru se nehlásí. Docker ho sám vytvoří pro složku, kterou image deklaruje a nikdo ji nenamapoval, pojmenuje ho 64 šestnáctkovými číslicemi a uloží pod `/var/lib/docker/volumes`. Většinou v něm je mezipaměť, proto záloha zůstane zelená a karta ho jen uvede s poznámkou, že je anonymní.

- **Chceš ta data zálohovat.** Namapuj složku na cestu na hostiteli pod Host Data, třeba na složku ve sdílení appdata, a další záloha ji zahrne.
- **Nepotřebuješ je.** Přidej jednu z jejích dvou cest jako řádek do karty **Vzory vyloučení** na stejné kartě: cestu uvnitř kontejneru, třeba `/cache`, nebo cestu na hostiteli, kterou varování uvádí, třeba `/var/lib/docker/volumes/jellyfin_cache/_data`. Od další zálohy varování tuto složku vynechá a náhled pod polem řekne proč. Řádek pro nadřazenou složku funguje také, samotný název nebo `/` ne.

## Záloha hlásí Odloženo {#deferred-backups}

Aplikace kontejneru byla zaneprázdněná, takže jeho záloha čeká a sama se zkusí znovu. Řádek v protokolu aktivit říká proč a kdy přijde další pokus; karta **Když je aplikace zaneprázdněná** na kartě **Nastavení** kontejneru říká totéž a navíc, kdy čekání skončí. Nemusíš nic dělat. Chceš-li zálohovat hned, použij **Zálohovat nyní**: záloha, kterou spustíš sám, nikdy nečeká, ani při návratovém kódu 75.

- **Vlastní kontrola.** Nech **Příkaz před zálohou** skončit s kódem 75, dokud je aplikace zaneprázdněná, například `[ "$(cat /tmp/players)" = 0 ] || exit 75`. Jakýkoli jiný kód než 0 nechá zálohu selhat jako dosud, takže rozbitý skript se dál ukáže v **Vyžaduje pozornost**.
- **Na Plex, Jellyfin ani Emby nikdy nepřijde dotaz.** BombVault se nejdřív zeptá serveru na vlastní adrese kontejneru, a když tam nic neodpoví, spustí curl nebo wget uvnitř kontejneru. Image bez obou se přes sebe samu zeptat nedá; protokol to pak uvede. Se zapnutým **Zálohovat, když server neodpovídá** taková záloha stejně proběhne, s vypnutým čeká.
- **Jellyfin nebo Emby odmítá.** Zkontroluj **Klíč API**. Vytvoříš ho v dashboardu serveru v části klíčů API.
- **Plex odmítá.** BombVault čte token z `Preferences.xml` ve složce připojené na `/config`. Server, ke kterému se nikdo nikdy nepřihlásil, žádný token nemá a odpovídá jen tam, kde povoluje přístup bez tokenu.
- **Pozastavený film nechá zálohu čekat celou noc.** Nastav **Ignorovat pozastavené relace po (min)**, třeba na 30.
- **Kontrola provozu zálohu nikdy nepustí.** Zvyš **Zaneprázdněný od (KiB/s)** nad to, co kontejner odesílá, když ho nikdo nepoužívá. Hned po startu BombVaultu kontrola čeká, dokud kontejner nesleduje **V klidu po dobu (min)** minut.
- **Záloha v noci nikdy neproběhne.** S nastaveným oknem přicházejí pokusy jen uvnitř něj. Čekání, které dosáhne svého termínu mimo okno, skončí v termínu i tak.

## Velká záloha byla ukončena kvůli nedostatku paměti

Nastav **Limit paměti na zálohu (MiB, 0 = žádný)** v kartě **Výkon** v **Nastavení, Systém** výrazně pod to, co smí kontejner použít. Blízko limitu restic častěji uvolňuje paměť, což stojí trochu rychlosti, takže záloha zpomalí, místo aby byla ukončena.

## Obnova se přerušila dříve, než začala

Než se cokoli zastaví nebo odebere, obnova spustí předletovou kontrolu konfliktů: ověří, že statická IP kontejneru a publikované hostitelské porty jsou volné. Pokud je jeden z nich již držen jiným kontejnerem, přeruší se s jasnou, akceschopnou zprávou místo toho, aby zanechala napůl dokončenou obnovu. Uvolni konfliktní port nebo IP, poté opakuj.

Pokud síť kontejneru neexistuje, například protože byla smazána jeho aplikace TrueNAS, kontrola před obnovou tu síť uvede a obnova odmítne dřív, než cokoli změní. Nainstaluj aplikaci v TrueNAS znovu pod stejným názvem, čímž se síť znovu vytvoří, a pak obnov její data.

## Prostý export selhal místo zapsání souboru

Pokud je **Šifrovat exporty pomocí age** zapnuto v **Nastavení, Systém**, ale není nastaven platný příjemce, export selže s jasnou chybou místo zapsání prostého textu. Přidej platného příjemce (veřejný klíč age nebo veřejný klíč SSH), nebo vypni šifrování, pokud zamýšlíš, aby export byl prostý text. Viz [Funkce](features.md).

## Dump databáze selhal

Neúspěšný dump nikdy nepoloží zálohu kolem sebe; zapíše se jako vlastní neúspěšný běh a důvod říká, co spravit.

- **Odmítnuté přihlášení.** Dump se přihlašuje proměnnými s heslem samotného kontejneru (`POSTGRES_PASSWORD`, `MARIADB_ROOT_PASSWORD`, `MYSQL_ROOT_PASSWORD` nebo jejich verzemi `_FILE`). Zkontroluj je na kontejneru s databází. Proměnná `_FILE` ukazující na tajemství, které uživatel kontejneru nesmí číst, dopadne stejně.
- **Chybějící oprávnění.** S náhodným heslem roota se dump přihlásí jen jako uživatel aplikace, obsáhne tedy jen tu jednu databázi, a MySQL 8.4 a novější ho může odmítnout úplně. Dej kontejneru skutečné heslo roota, nebo mu dump vypni.
- **Systémové tabulky potřebují povýšit.** MariaDB odmítá dump, když její systémové tabulky pocházejí ze starší verze (chyba 1558). Přidej proměnnou `MARIADB_AUTO_UPGRADE=1` a restartuj kontejner, nebo v něm jednou spusť `mariadb-upgrade`.
- **Žádný nástroj pro dump.** Odlehčený nebo vlastnoručně sestavený image bez `pg_dump`, `mysqldump` či `mariadb-dump` dumpovat nejde. Vezmi oficiální image, nebo dump vypni.
- **Časový limit.** Dump dostane `DB_DUMP_MAX_HOURS` (výchozí 6), záloha kolem něj `BACKUP_MAX_HOURS`, a dump, který přestane postupovat, se utne po `BACKUP_STALL_HOURS`. Za posledním případem bývá zámek držený aplikací. Zvedni limit, který zabral, nebo dumpuj, když je aplikace v klidu.
- **Kontejner je pozastavený nebo se restartuje.** Dump mluví s běžícím serverem. Pokud se kontejner restartuje pořád dokola, jeho vlastní log řekne proč.
- **Poškozený dump se nepodařilo odstranit.** Dump, který BombVault nedokázal dokončit, se zase smaže. Když se to smazání nepovede, dump zůstane v seznamu označený jako poškozený a můžeš ho tam smazat.

## Import selhal

Import zastaví kontejner, odsune jeho datovou složku stranou a nechá image vytvořit na jejím místě prázdnou. Selže-li krok před samotným importem, stará složka se vrátí sama. Selže-li import, kontejner si nechá čerstvou složku a stará zůstane vedle jako `<datová složka>.bombvault-before-import-<časová značka>`; chybová hláška běhu uvádí přesnou cestu.

Datovou složku, která je samostatnou datovou sadou, jako na TrueNAS, nejde přejmenovat. Import pak složku nechá na místě a její obsah přesune do zachované složky vedle ní, takže nadřazená datová sada pro něj potřebuje místo. Import to ověří dřív, než kontejner zastaví.

Ruční návrat: zastav kontejner, přejmenuj současnou datovou složku stranou, přejmenuj zachovanou složku zpět na původní název a kontejner spusť. U datové sady místo přejmenování složek přesuň obsah. Na Unraidu to zvládne správce souborů na kartě Shares.

## Záloha datové sady ZFS selhala nebo nějakou sadu přeskočila {#zfs-datasets}

Každý problém nese kód důvodu v hranatých závorkách a stránka [Datové sady ZFS](zfs-datasets.md#reason-codes) je uvádí všechny i s nápravou. Tři nejčastější:

- **`snapshot-loop`**: snímek se k BombVaultu nedostal, protože Host Data nepředává nová připojení. Uprav kontejner, nastav Access Mode u Host Data na Read/Write - Slave a restartuj BombVault.
- **`key-not-loaded`**: šifrovaná datová sada bez načteného klíče se přeskočí. Načti klíč pomocí `zfs load-key` a připoj sadu; příští záloha ji zahrne.
- **`ssh-auth`**: server odmítl klíč BombVaultu. **Připojení ZFS k tomuto serveru**, které otevře **Zobrazit připojení** na stránce Zálohy, ukazuje příkaz, který ho povolí; spusť ho jednou na serveru.

## Položka zůstává na "Učí se N/10"

Většina kontrol anomálií začíná po 10 úspěšných zálohách položky a počítání začne znovu po **Označit jako očekávané** a po změně výběru položky. Položka bez plánu se neučí a kontejner bez appdata se nemá z čeho učit, což říká i jeho odznak. **Průběh učení** na stránce Anomálie ukazuje u každé položky prstenec o deseti dílech.

## Uchovávání přestalo mazat staré zálohy jedné položky

Drží je otevřená kritická anomálie: zdroj položky je téměř prázdný, výrazně se zmenšil, nebo záloha znovu uložila většinu dat. Otevři anomálii z odznaku u položky. Pokud data chybí nebo byla zašifrována, obnov nejprve z odkazované poslední dobré zálohy. Pak anomálii potvrď, nebo ji označ jako očekávanou, pokud změna pochází od tebe, a další běh uklidí jako obvykle. Náhled uchovávání takovou položku označí jako ponechanou. U položky ZFS si staré zálohy ponechá jen datová sada, kterou anomálie jmenuje; ostatní datové sady stromu se pročistí jako obvykle.

## Ruční čištění hlásí, že některé položky byly ponechány

Stejná příčina: čištění nechá staré zálohy položky s takovou anomálií na pokoji a uvede ji ve své zprávě. Všechno ostatní se vyčistí jako obvykle.

## Import historie hlásí, že repozitář nešlo přečíst

Po aktualizaci BombVault jednou načte velikosti dřívějších záloh z každého repozitáře. Repozitář, který v tu chvíli nebyl dostupný, například nefunkční vzdálený cíl nebo nepřipojená sdílená složka, je uveden na kartě **Anomálie** v **Nastavení, Ověření** a zkouší se znovu jednou denně. Jeho položky se mezitím učí z nových záloh.

## Varování o místě na disku nesouhlasí s přehledem Unraidu

Na uživatelské sdílené složce Unraidu (`/mnt/user`) je volné místo celého pole, ne jednoho disku. Vzdálené repozitáře se měří jen přes rclone remoty, které hlásí své volné místo; repozitáře S3, B2, REST a SFTP žádný údaj nemají a na kartě **Anomálie** jsou uvedeny jako neměřené.

## Asistent s umělou inteligencí se nepřipojí

Stránka [Server MCP](mcp.md#troubleshooting) uvádí, co znamená každý stavový kód a každé odmítnutí koncového bodu MCP a co s tím dělat.

## Kontejner se stále restartuje nebo vypadá unhealthy

BombVault hlásí healthy/unhealthy ze svého vlastního `/api/health`. Nástroj pro automatické hojení (například Autoheal) jej může restartovat automaticky, pokud se engine kdy zasekne. Zkontroluj log kontejneru a kartu **Integrace hostitele** pro základní příčinu.

## Stále zaseknutí?

- Přečti si kompletní stránky [Konfigurace](configuration.md) a [Mimo lokalitu a obnova](offsite-recovery.md).
- Zeptej se ve [vlákně podpory Unraid](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/).
- Otevři [GitHub issue](https://github.com/junkerderprovinz/bombvault/issues).
