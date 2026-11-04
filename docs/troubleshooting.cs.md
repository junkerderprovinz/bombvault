# Řešení problémů

Krátké FAQ. Kompletní tabulku řešení problémů na straně hostitele pro VM přes SSH (permission-denied, ověření hostitelského klíče, chybějící proměnné šablony a další) najdeš v [průvodci Záloha VM přes SSH](vm-backup-ssh-setup.md).

## Něco není správně zapojeno {#something-is-not-wired-up-correctly}

Přidej kartu **Integrace hostitele** do Přehledu pod **Přizpůsobit**. Kontrola integrace hostitele prozkoumá každé připojení a CLI (Docker socket, libvirt, restic, qemu-img, rclone) a nahlásí případné chybějící části a **Zkontrolovat nyní** ji spustí znovu. Začni zde, než budeš předpokládat chybu: chybějící připojení nebo nedosažitelný hostitel se objeví okamžitě.

## Nemohu se dostat k webovému rozhraní {#i-cannot-reach-the-web-ui}

BombVault obsluhuje HTTPS rovnou z krabice na portu `3443` (samopodepsaný certifikát), takže otevři `https://<your-unraid-ip>:3443`. Přijmi varování o samopodepsaném certifikátu, nebo umísti BombVault za reverzní proxy s vlastním certifikátem. Pokud běžíš s `HTTP_ONLY=true`, obsluhuje místo toho prosté HTTP na portu `3000` (určeno pro použití za proxy terminující TLS).

## Ztratil jsem svůj APP_KEY {#i-lost-my-app_key}

`APP_KEY` odvozuje heslo k restic repozitáři. Bez něj, bez sady pro obnovu a bez úschovy klíče ve tvé skupině nelze šifrované zálohy obnovit. Proto Přehled ukazuje **Ulož si sadu pro obnovu**, dokud ji nestáhneš. Viz [Mimo lokalitu a obnova](offsite-recovery.md#recovery-kit). Vygeneruj klíč pomocí `openssl rand -hex 32` a ulož jej mimo server, dříve než se budeš spoléhat na jakoukoli zálohu.

## Zapomněl jsem přihlašovací heslo {#i-forgot-the-login-password}

Zvol **Zapomenuté heslo?** na přihlašovací stránce. BombVault zapíše kód o 12 znacích do logu svého kontejneru (na Unraidu: záložka Docker, ikona BombVault, **Logy**). Zadej ho s novým heslem do 10 minut. Reset také vypne dvoufázové ověření, odstraní všechny passkeys, zruší všechny API tokeny, klíče MCP a přihlášení OAuth, ukončí všechny ostatní relace a vypne párování telefonů. Passkeys přidej znovu v **Nastavení, Zabezpečení**, párování telefonů tam zase zapni a nové tokeny a klíče vytvoř tam, kde je ještě potřebuješ. Viz [Bezpečnostní model](configuration.md#login).

## Zálohy se zastavily a hlásí, že klíč už nejde otevřít {#backups-stopped-and-say-the-key-can-no-longer-be-opened}

Tento server převzal v Nouzovém průvodci zálohovací klíč jiného serveru a `APP_KEY` se potom změnil. Převzatý klíč je zapečetěný starým APP_KEY, takže ho BombVault neotevře a raději nezálohuje nic, než aby zapisoval špatným klíčem. Pojmenuje ho **Vyžaduje pozornost** v Přehledu. Nastav APP_KEY zpět na původní hodnotu, nebo znovu nahraj sadu pro obnovu na stránce Obnova pod **Nouze**.

## Telefon skončí na přihlašovací stránce {#a-phone-lands-on-the-login-page}

Spárovaný telefon dostane relaci přes skupinu jen tehdy, když je v **Nastavení, Zabezpečení** zapnuto **Telefony se mohou spárovat s dvanácti slovy skupiny**. Reset hesla ho vypne. Aplikace starší než server také nedostane relaci, dokud se neaktualizuje. Server přidaný podle adresy se na heslo ptá vždy. Telefon odebraný ze stránky Instance se po dalším otevření aplikace vrátí vypnutý. Abys ho pustil dovnitř, zapni v jeho podrobnostech v části **Sledování** volbu **Dotázat se na tuto instanci**.

## Záloha VM se nepřipojí {#vm-backup-will-not-connect}

Záloha VM komunikuje s libvirt přes SSH, nikdy přes připojení.

- Potvrď, že SSH je povoleno na hostiteli a veřejný klíč BombVaultu je autorizovaný v `/root/.ssh/authorized_keys` (Nastavení, Integrace, SSH k hostiteli zobrazuje klíč a tlačítko **Otestovat připojení**).
- Na vlastní síti `br0.x` nastav `LIBVIRT_HOST` na svou LAN IP Unraidu (kontejner tam nemůže dosáhnout na hostitele přes `host.docker.internal`). Povol **Settings, Docker, Host access to custom networks**.
- Pokud jsi změnil SSH port Unraidu, nastav `LIBVIRT_SSH_PORT`, aby odpovídal.
- Na TrueNAS nastav `LIBVIRT_URI` s vlastním socketem libvirt z TrueNAS, dej aplikaci capability `SETPCAP` a přidej klíč uživateli root v **Credentials, Users** (viz [TrueNAS SCALE](getting-started.md#truenas)).
- Kompletní krok za krokem diagnóza (test dosažitelnosti, směrování VLAN, `Permission denied (publickey)`, `Host key verification failed`) je v [průvodci Záloha VM přes SSH](vm-backup-ssh-setup.md).

## Živý snímek VM neproběhl {#a-live-vm-snapshot-did-not-run}

Živé snímky potřebují qemu guest agent nainstalovaný ve VM a disk na `/mnt/cache` (nebo `/mnt/diskX`), nikoli `/mnt/user`. Na vypnuté VM se živý automaticky vrátí k šetrnému. Šetrná záloha VM vypne, zálohuje disky, poté ji znovu spustí, takže je vždy konzistentní. Čeká až deset minut, než se host vypne, a nikdy ho nevypíná násilím, takže host s Windows, který zrovna instaluje aktualizace, se nepřeruší v půlce. Host, který potom stále běží, zůstane nedotčen a tato záloha selže s uvedením důvodu.

## Záloha selhala s "repository is already locked" {#a-backup-failed-with-repository-is-already-locked}

Toto je obvykle osiřelý restic zámek zanechaný, když byl kontejner aktualizován nebo restartován uprostřed operace. BombVault detekuje prokazatelně osiřelý zámek, násilně jej vyčistí a jednou zopakuje, automaticky. Pokud přetrvává, otevři kartu **Zkontrolovat zálohy** v **Nastavení, Ověření** a zvol **Odemknout** pod **Více** na řádku postižené sekce k ručnímu vyčištění zaseklého zámku. Skutečný problém se stále objeví, místo aby byl skryt. Po restartu BombVault čeká, dokud takový zámek nebude deset minut bez obnovení. Restic, který ještě běží, třeba v druhém BombVaultu nad stejným repozitářem, svůj zámek obnovuje každých pět minut.

## Moje kopie mimo lokalitu neproběhla po záloze {#my-off-site-copy-did-not-happen-after-a-backup}

Replikace mimo lokalitu je na základě nejlepší snahy záměrně, takže zádrhel mimo lokalitu nikdy nezhatí místní zálohu. Zkontroluj, kdy cíl kopíruje: jeho karta **Kopírovat** říká **Po každé záloze** nebo uvádí vlastní plán a **Nastavení, Plán, Kopie mimo lokalitu podle sekcí** může dát sekci vlastní plán. Použij **Replikovat nyní** v řádku sekce na kartě **Kopie mimo lokalitu** v **Nastavení, Cíle** pro běh na vyžádání a sleduj hlavičku stavu v Přehledu, která za běhu říká **Probíhá kopie mimo lokalitu**. Neúspěšná kopie se ukáže pod **Vyžaduje pozornost**.

## Certifikátu rest-serveru se nedůvěřuje {#a-rest-servers-certificate-is-not-trusted}

restic odmítne certifikát, který nezná, a chybová zpráva řekne, kam ho dát. Zkopíruj samopodepsaný certifikát rest-serveru, nebo certifikát autority, která ho podepsala, jako soubor PEM do složky `certs` v datové složce BombVaultu (`/config/certs` v kontejneru, `appdata/bombvault/certs` na Unraidu) a otestuj znovu. Restart není potřeba.

## Záloha hlásí, že data mimo Host Data nebyla zálohována {#data-outside-host-data}

Kontejner má data ve složce, kterou BombVault přes Host Data nepřečte. Záloha uloží definici a každou složku, ke které se dostane, obarví svůj řádek v protokolu aktivit oranžově a vyjmenuje složky, které vynechala. Dva případy jsou běžné: ixVolume na TrueNAS pod `/mnt/.ix-apps` a pojmenovaný svazek Dockeru pod `/var/lib/docker/volumes`, třeba svazek s databází ze stacku compose. Karta **Zálohované složky** na záložce **Nastavení** kontejneru ukazuje každou takovou složku s její cestou uvnitř kontejneru a cestou na hostiteli, označenou červeně.

Anonymní svazek Dockeru se nehlásí. Docker ho sám vytvoří pro složku, kterou image deklaruje a nikdo ji nenamapoval, pojmenuje ho 64 šestnáctkovými číslicemi a uloží pod `/var/lib/docker/volumes`. Většinou v něm je mezipaměť, proto záloha zůstane zelená a karta ho jen uvede s poznámkou, že je anonymní.

- **Chceš ta data zálohovat.** Namapuj složku na cestu na hostiteli pod Host Data, třeba na složku ve sdílení appdata, a další záloha ji zahrne.
- **Nepotřebuješ je.** Přidej jednu z jejích dvou cest jako řádek do karty **Vzory vyloučení** na stejné záložce: cestu uvnitř kontejneru, třeba `/cache`, nebo cestu na hostiteli, kterou varování uvádí, třeba `/var/lib/docker/volumes/jellyfin_cache/_data`. Od další zálohy varování tuto složku vynechá a náhled pod polem řekne proč. Řádek pro nadřazenou složku funguje také, samotný název nebo `/` ne.

## Záloha hlásí Odloženo {#deferred-backups}

Aplikace kontejneru byla zaneprázdněná, takže jeho záloha čeká a sama se zkusí znovu. Řádek v protokolu aktivit říká proč a kdy přijde další pokus. Karta **Když je aplikace zaneprázdněná** na záložce **Nastavení** kontejneru říká totéž a navíc, kdy čekání skončí. Nemusíš nic dělat. Chceš-li zálohovat hned, použij **Zálohovat nyní**: záloha, kterou spustíš sám, nikdy nečeká, ani při návratovém kódu 75.

- **Vlastní kontrola.** Nech **Příkaz před zálohou** skončit s kódem 75, dokud je aplikace zaneprázdněná, například `[ "$(cat /tmp/players)" = 0 ] || exit 75`. Jakýkoli jiný kód než 0 nechá zálohu selhat jako dosud, takže rozbitý skript se dál ukáže v **Vyžaduje pozornost**.
- **Na Plex, Jellyfin ani Emby nikdy nepřijde dotaz.** BombVault se nejdřív zeptá serveru na vlastní adrese kontejneru, a když tam nic neodpoví, spustí curl nebo wget uvnitř kontejneru. Image bez obou se přes sebe samu zeptat nedá. Protokol to pak uvede. Se zapnutým **Zálohovat, když server neodpovídá** taková záloha stejně proběhne, s vypnutým čeká.
- **Jellyfin nebo Emby odmítá.** Zkontroluj **Klíč API**. Vytvoříš ho v dashboardu serveru v části API keys.
- **Plex odmítá.** BombVault čte token z `Preferences.xml` ve složce připojené na `/config`. Server, ke kterému se nikdo nikdy nepřihlásil, žádný token nemá a odpovídá jen tam, kde povoluje přístup bez tokenu.
- **Pozastavený film nechá zálohu čekat celou noc.** Nastav **Ignorovat pozastavené relace po (min)**, třeba na 30.
- **Kontrola provozu zálohu nikdy nepustí.** Zvyš **Zaneprázdněný od (KiB/s)** nad to, co kontejner odesílá, když ho nikdo nepoužívá. Hned po startu BombVaultu kontrola čeká, dokud kontejner nesleduje **V klidu po dobu (min)** minut.
- **Záloha v noci nikdy neproběhne.** S nastaveným oknem přicházejí pokusy jen uvnitř něj. Čekání, které dosáhne svého termínu mimo okno, skončí v termínu i tak.

## Velká záloha byla ukončena kvůli nedostatku paměti {#a-large-backup-was-killed-for-running-out-of-memory}

Nastav **Limit paměti na zálohu (MiB, 0 = žádný)** v kartě **Výkon** v **Nastavení, Systém** výrazně pod to, co smí kontejner použít. Blízko limitu restic častěji uvolňuje paměť, což stojí trochu rychlosti, takže záloha zpomalí, místo aby byla ukončena.

## Obnova se přerušila dříve, než začala {#a-restore-aborted-before-it-started}

Než se cokoli zastaví nebo odebere, obnova spustí předletovou kontrolu konfliktů: ověří, že statická IP kontejneru a publikované hostitelské porty jsou volné. Pokud je jeden z nich již držen jiným kontejnerem, přeruší se s jasnou, akceschopnou zprávou místo toho, aby zanechala napůl dokončenou obnovu. Uvolni konfliktní port nebo IP, poté opakuj.

Pokud síť kontejneru neexistuje, například protože byla smazána jeho aplikace TrueNAS, kontrola před obnovou tu síť uvede a obnova odmítne dřív, než cokoli změní. Nainstaluj aplikaci v TrueNAS znovu pod stejným názvem, čímž se síť znovu vytvoří, a pak obnov její data.

## Prostý export selhal místo zapsání souboru {#a-plain-export-failed-instead-of-writing-a-file}

Pokud je **Šifrovat exporty pomocí age** zapnuto v **Nastavení, Systém**, ale není nastaven platný příjemce, export selže s jasnou chybou místo zapsání prostého textu. Přidej platného příjemce (veřejný klíč age nebo veřejný klíč SSH), nebo vypni šifrování, pokud zamýšlíš, aby export byl prostý text. Viz [Prosté exporty](backups.md#exports).

## Výpis databáze selhal {#a-database-dump-failed}

Neúspěšný výpis nikdy nepoloží zálohu kolem sebe. Zapíše se jako vlastní neúspěšný běh a důvod říká, co spravit.

- **Odmítnuté přihlášení.** Výpis se přihlašuje proměnnými s heslem samotného kontejneru (`POSTGRES_PASSWORD`, `MARIADB_ROOT_PASSWORD`, `MYSQL_ROOT_PASSWORD` nebo jejich verzemi `_FILE`). Zkontroluj je na kontejneru s databází. Proměnná `_FILE` ukazující na tajemství, které uživatel kontejneru nesmí číst, dopadne stejně.
- **Chybějící oprávnění.** S náhodným heslem roota se výpis přihlásí jen jako uživatel aplikace, obsáhne tedy jen tu jednu databázi, a MySQL 8.4 a novější ho může odmítnout úplně. Dej kontejneru skutečné heslo roota, nebo mu výpis vypni.
- **Systémové tabulky potřebují povýšit.** MariaDB odmítá výpis, když její systémové tabulky pocházejí ze starší verze (chyba 1558). Přidej proměnnou `MARIADB_AUTO_UPGRADE=1` a restartuj kontejner, nebo v něm jednou spusť `mariadb-upgrade`.
- **Žádný nástroj pro výpis.** Odlehčený nebo vlastnoručně sestavený image bez `pg_dump`, `mysqldump` či `mariadb-dump` vypsat nejde. Vezmi oficiální image, nebo výpis vypni.
- **Časový limit.** Výpis dostane `DB_DUMP_MAX_HOURS` (výchozí 6), záloha kolem něj `BACKUP_MAX_HOURS`, a výpis, který přestane postupovat, se utne po `BACKUP_STALL_HOURS`. Za posledním případem bývá zámek držený aplikací. Zvedni limit, který zabral, nebo vypisuj, když je aplikace v klidu.
- **Kontejner je pozastavený nebo se restartuje.** Výpis mluví s běžícím serverem. Pokud se kontejner restartuje pořád dokola, jeho vlastní log řekne proč.
- **Poškozený výpis se nepodařilo odstranit.** Výpis, který BombVault nedokázal dokončit, se zase smaže. Když se to smazání nepovede, výpis zůstane v seznamu označený jako poškozený a můžeš ho tam smazat.

## Import selhal {#an-import-failed}

Import zastaví kontejner, odsune jeho datovou složku stranou a nechá image vytvořit na jejím místě prázdnou. Selže-li krok před samotným importem, stará složka se vrátí sama. Selže-li import, kontejner si nechá čerstvou složku a stará zůstane vedle jako `<datová složka>.bombvault-before-import-<časová značka>`. Chybová hláška běhu uvádí přesnou cestu.

Datovou složku, která je samostatnou datovou sadou, jako na TrueNAS, nejde přejmenovat. Import pak složku nechá na místě a její obsah přesune do zachované složky vedle ní, takže nadřazená datová sada pro něj potřebuje místo. Import to ověří dřív, než kontejner zastaví.

Ruční návrat: zastav kontejner, přejmenuj současnou datovou složku stranou, přejmenuj zachovanou složku zpět na původní název a kontejner spusť. U datové sady místo přejmenování složek přesuň obsah. Na Unraidu to zvládne správce souborů na záložce Shares.

## Záloha datové sady ZFS selhala nebo nějakou sadu přeskočila {#zfs-datasets}

Každý problém nese kód důvodu v hranatých závorkách a stránka [Datové sady ZFS](zfs-datasets.md#reason-codes) je uvádí všechny i s nápravou. Tři nejčastější:

- **`snapshot-loop`**: snímek se k BombVaultu nedostal, protože Host Data nepředává nová připojení. Uprav kontejner, nastav Access Mode u Host Data na Read/Write - Slave a restartuj BombVault.
- **`key-not-loaded`**: šifrovaná datová sada bez načteného klíče se přeskočí. Načti klíč pomocí `zfs load-key` a připoj sadu. Příští záloha ji zahrne.
- **`ssh-auth`**: server odmítl klíč BombVaultu. **Připojení ZFS k tomuto serveru**, které otevře **Zobrazit připojení** na stránce Zálohy, ukazuje příkaz, který ho povolí. Spusť ho jednou na serveru.

## Položka zůstává na "Učí se N/10" {#an-entry-stays-at-learning-n10}

Většina kontrol anomálií začíná po 10 úspěšných zálohách položky a počítání začne znovu po **Označit jako očekávané** a po změně výběru položky. Položka bez plánu se neučí a kontejner bez appdata se nemá z čeho učit, což říká i jeho odznak. **Průběh učení** na stránce Anomálie ukazuje u každé položky kruh o deseti dílech.

## Uchovávání přestalo mazat staré zálohy jedné položky {#retention-stopped-deleting-old-backups-of-one-entry}

Drží je otevřená kritická anomálie: zdroj položky je téměř prázdný, výrazně se zmenšil, nebo záloha znovu uložila většinu dat. Otevři anomálii z odznaku u položky. Pokud data chybí nebo byla zašifrována, obnov nejprve z odkazované poslední dobré zálohy. Pak anomálii potvrď, nebo ji označ jako očekávanou, pokud změna pochází od tebe, a další běh uklidí jako obvykle. Náhled uchovávání takovou položku označí jako ponechanou. U položky ZFS si staré zálohy ponechá jen datová sada, kterou anomálie jmenuje, a ostatní datové sady stromu se pročistí jako obvykle.

## Ruční čištění hlásí, že některé položky byly ponechány {#manual-prune-says-some-entries-were-kept}

Stejná příčina: čištění nechá staré zálohy položky s takovou anomálií na pokoji a uvede ji ve své zprávě. Všechno ostatní se vyčistí jako obvykle.

## Import historie hlásí, že repozitář nešlo přečíst {#history-import-says-a-repository-could-not-be-read}

Po aktualizaci BombVault jednou načte velikosti dřívějších záloh z každého repozitáře. Repozitář, který v tu chvíli nebyl dostupný, například nefunkční cíl mimo lokalitu nebo nepřipojená sdílená složka, je započítán na kartě **Anomálie** v **Nastavení, Ověření** a zkouší se znovu jednou denně. Jeho položky se mezitím učí z nových záloh.

## Varování o místě na disku nesouhlasí s přehledem Unraidu {#the-disk-space-warning-does-not-match-the-unraid-dashboard}

Na uživatelské sdílené složce Unraidu (`/mnt/user`) je volné místo celého pole, ne jednoho disku. Vzdálené cíle, ke kterým se BombVault dostává přes rclone, například SFTP, Hetzner Storage Box, SMB, WebDAV nebo cloudový disk, se měří, když druhá strana hlásí své volné místo, a stejně tak adresa `sftp:` zadaná ručně. Repozitáře S3, B2 a REST žádný údaj nemají: jejich cíl říká **Velikost neznámá** a karta **Anomálie** je uvádí jako neměřené.

## Asistent s umělou inteligencí se nepřipojí {#an-ai-assistant-cannot-connect}

Stránka [Server MCP](mcp.md#troubleshooting) uvádí, co znamená každý stavový kód a každé odmítnutí koncového bodu MCP a co s tím dělat.

## Kontejner se stále restartuje nebo vypadá unhealthy {#the-container-keeps-restarting-or-looks-unhealthy}

BombVault hlásí healthy/unhealthy ze svého vlastního `/api/health`. Nástroj pro automatické hojení (například Autoheal) jej může restartovat automaticky, pokud se engine kdy zasekne. Zkontroluj log kontejneru a kartu **Integrace hostitele** pro základní příčinu.

## Cíl se odmítá uložit {#location-refused}

BombVault cíl před uložením zkontroluje a odmítnutí řekne důvod jednou větou. Obvyklé případy:

- **„Toto místo leží uvnitř jiného repozitáře nebo cíle, nebo ho obsahuje.“** Repozitář uvnitř repozitáře by pokazil oba. Zpráva jmenuje to druhé místo. Vyber jinou složku, nebo to druhé místo nejdřív odeber.
- **Je to repozitář, který jsi založil.** Cíl je jeden z řádků pod **Repozitáře**, nebo leží uvnitř jednoho z nich či kolem něj. Zvol jiné místo, nebo ten repozitář nejdřív odeber.
- **Stejné místo jako vlastní složka sekce.** Kopie na vlastní složku sekce by při každém běhu kopírovala její zálohy samy na sebe, proto se odmítá. Vyber jiné místo, nebo viz [Vlastní složka sekce](offsite-recovery.md#own-folder).
- **„To není cíl mimo lokalitu této sekce“** pocházelo z odkazu na cíl, který byl odebrán. Aktuální verze takové odkazy při startu a při importu vyčistí. Pokud to přesto vidíš, aktualizuj BombVault.
- **Úplná cesta na hostiteli**, například `/mnt/user/backups`. BombVault běží v kontejneru, takže chce cestu pod Host Data, `user/backups`, a zpráva jmenuje tu, kterou zadat.

## Vzdálený cíl hlásí, že jeho repozitář je pryč {#repository-gone}

Cíl, který držel zálohy a najednou už žádný repozitář nedrží, nedostane sám nový, prázdný, protože ten by zakryl skutečný, jakmile se disk vrátí. Obvyklou příčinou je rest-server, jehož disk se nepřipojil, takže disk připoj znovu a spusť zálohu nebo kopii ještě jednou. Jen když jsi starý repozitář vyprázdnil nebo smazal schválně, stiskni na stránce cíle **Založit tu nový repozitář**. Začne od začátku pro všechny sekce toho cíle najednou.

## Přihlášení do cloudu vypršelo {#cloud-sign-in-expired}

Test nebo příští kopie pak řekne, že přihlášení vypršelo. **Změnit připojení** na stránce cíle se přihlásí znovu.

- **Box, Jottacloud a další s jednorázovými obnovovacími tokeny:** BombVault drží token, který rclone obnovil, i přes restarty a v sadě pro obnovu. Po obnově ze starší zálohy nastavení může být token v ní už prošlý, takže se přihlas ještě jednou. Repozitář zůstává, jak je.
- **iCloud Drive:** Apple ukončí přihlášení po 30 dnech. Potřebuje nový token důvěry a cookies z `rclone config`.
- **OneDrive:** přihlášení s více disky se zeptá, který použít, takže cíl bez výběru zůstane nepoužitelný, dokud nevybereš. K OneDrive Business a Google Drive viz poznámku o odstraněných datech v [Cílech](offsite-recovery.md#destinations).

## „Kopie poslední zálohy mimo lokalitu stále běží“ {#copy-still-running}

Záloha před ní skončila a její kopie je stále na cestě, což blokuje novou zálohu té sekce. Počkej, až hlavička stavu přestane říkat **Probíhá kopie mimo lokalitu**. Tlačítka, která spouštějí zálohu, říkají totéž, dokud čekají. První kopie do nového cíle může trvat hodiny, viz [příklad se S3](offsite-recovery.md#example-s3).

## Replika ZFS se zastaví, protože cíl drží novější snímky {#replica-target-newer}

Cíl drží snímky novější, než je ten nejnovější, který sdílí s tímto serverem, například protože datová sada tady byla vrácena zpět nebo se kopie na cíli změnila. Karta Replika a historie běhů jmenují snímek pod **Vrátit cíl zpět na**. Spusť na cíli `zfs rollback -r` na ten snímek, pro každou datovou sadu, kterou běh jmenuje, a stiskni **Replikovat nyní**. Viz [Jak dlouho snímky zůstávají](zfs-datasets.md#replica-retention).

## VM se snapshoty Unraidu selže a jmenuje soubor {#vm-chain}

BombVault sleduje řetěz podkladových souborů každého disku a zálohuje každý soubor v něm. Řetěz, který nejde sledovat až do konce, zálohu raději shodí, než aby vynechal část disku, a zpráva jmenuje soubor. Zkontroluj, že soubor existuje pod `/mnt` a že snapshot je pořád vypsán ve VM Manageru. Soubor mimo Host Data nejde přečíst vůbec. Viz [VM se snapshoty Unraidu](vm-backup-ssh-setup.md#unraid-snapshots).

## Kopie mimo lokalitu jsou po opětovném sestavení pozastavené {#copies-paused}

Po **Objevit zálohy** nebo po Nouzovém průvodci bez obnovených nastavení kopie mimo lokalitu každé znovu sestavené sekce čekají, protože BombVault už neví, které položky jsi vynechal. Přehled to říká. Stiskni **Potvrdit výchozí nastavení** v **Cíle podle sekcí**, zkontroluj náhled a vynech, co má zůstat venku. Import souboru s nastavením pozastavení neukončí. Viz [Po opětovném sestavení](offsite-recovery.md#after-a-rebuild).

## Oznámení nedorazí {#notifications-do-not-arrive}

Začni s **Odeslat test** v **Nastavení, Oznámení**. Řádek pod kartou říká, co se stalo.

- **Neodchází vůbec nic.** Zkontroluj **Oznámit**. S **Nikdy** nic neodchází a s **Pouze při selhání** zůstává dobrý běh potichu. Se zapnutým **Shrnout naplánované běhy** pošle naplánovaný běh na svém konci jeden souhrn.
- **Nová adresa.** Po změně serveru u Matrixu, Gotify nebo e-mailu se pole znovu zeptá na token nebo heslo a nová adresa se uloží, až ho napíšeš. BombVault staré tajemství na nové místo neposílá.
- **Gotify** říká, že server odmítl token aplikace: zkopíruj ho znovu z aplikace v Gotify. **URL serveru** je základní adresa, bez `/message`.
- **Webhook:** zkontroluj, že **Formát Webhook** odpovídá službě. Adresa Discordu potřebuje **Discord** a ntfy potřebuje **ntfy**.
- **Oznámení Unraid** běží přes spojení SSH k hostiteli, proto ho nejdřív otestuj v **Nastavení, Integrace**. Na jiné platformě zůstávají vypnutá a karta to říká.
- **E-mail s oznámením nikdy nepřijde.** Zkontroluj **Šifrování** proti portu, obvykle **STARTTLS** na 587 nebo **TLS (implicit)** na 465.

Pingy Healthchecks na ničem z toho nezávisejí, viz [Healthchecks](notifications.md#healthchecks).

## Aplikace pro Android neotevře server {#android-cannot-open}

Úvodní obrazovka řekne důvod na kartě serveru:

- **Nepřipojeno:** neodpovídá ani adresa serveru, ani relay. U **Bez přeposílače** aplikace dosáhne na server jen na jeho vlastní adrese.
- **Zkontroluj certifikát:** server ukazuje jiný certifikát než ten, kterému jsi důvěřoval. Aplikace ho otevře, až když novému důvěřuješ, takže se ujisti, že ta změna je tvoje.
- **Nutné přihlášení:** server přidaný podle adresy se ptá na heslo. Viz také [Telefon skončí na přihlašovací stránce](#a-phone-lands-on-the-login-page).
- **Klíč se změnil:** úvodní obrazovka ukáže otisk nového klíče a stránka se otevře, až mu důvěřuješ. Viz [Mimo domov](android.md#away).

## Jiný uživatel nemůže číst místní repozitář {#repository-permissions}

Místní repozitář zůstává čitelný pro ostatní uživatele, například pro uživatele synchronizace přes SMB. Před 10.4.1 mohly soubory, které zapsalo čištění, štítkování, stahování, přesun sekce, smazání nebo záloha stacku, zůstat do příští noci nečitelné. Aktualizuj BombVault a budou čitelné hned. Repozitář autozálohy zůstává čitelný jen pro roota, schválně, protože drží nastavení BombVaultu.

## Pořád to nejde? {#still-stuck}

- Přečti si kompletní stránky [Konfigurace](configuration.md) a [Mimo lokalitu a obnova](offsite-recovery.md).
- **Hlášení chyby** v kartě **Nápověda** v **Nastavení, Info** stáhne ZIP pro issue na GitHubu: kontrolu hostitele, tvou konfiguraci s odstraněnými tajemstvími, nedávné běhy, co je naplánováno dál a nedávný log od posledního startu kontejneru. Obsahuje také poslední výpis každé databáze, položky ZFS s jejich připojeními, otevřené anomálie a kolik existuje klíčů MCP. Hesla, tokeny a konfigurace rclone jsou odstraněny a balíček to říká ve svém vlastním manifestu. Vyžaduje přihlašovací heslo. U pádu, který kontejner restartoval, se podívej do `docker logs`.
- Zeptej se ve [vlákně podpory Unraid](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/).
- Otevři [GitHub issue](https://github.com/junkerderprovinz/bombvault/issues).
