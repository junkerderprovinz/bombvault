# Řešení problémů

Krátké FAQ. Kompletní tabulku řešení problémů na straně hostitele pro VM přes SSH (permission-denied, ověření hostitelského klíče, chybějící proměnné šablony a další) najdete v [průvodci Záloha VM přes SSH](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) na GitHubu.

## Něco není správně zapojeno

Otevřete `/spike` ve webovém rozhraní. Kontrola integrace hostitele prozkoumá každé připojení a CLI (Docker socket, libvirt, restic, qemu-img, rclone) a nahlásí případné chybějící části. Začněte zde, než budete předpokládat chybu: chybějící připojení nebo nedosažitelný hostitel se objeví okamžitě.

## Nemohu se dostat k webovému rozhraní

BombVault obsluhuje HTTPS rovnou z krabice na portu `3443` (samopodepsaný certifikát), takže otevřete `https://<your-unraid-ip>:3443`. Přijměte varování o samopodepsaném certifikátu, nebo umístěte BombVault za reverzní proxy s vlastním certifikátem. Pokud běžíte s `HTTP_ONLY=true`, obsluhuje místo toho prosté HTTP na portu `3000` (určeno pro použití za proxy terminující TLS).

## Ztratil jsem svůj APP_KEY

`APP_KEY` odvozuje heslo k restic repozitáři. Bez něj (a bez sady pro obnovu šifrovacího klíče) nelze šifrované zálohy obnovit. Proto vás Přehled popohání ke stažení sady pro obnovu. Viz [Mimo lokalitu a obnova](offsite-recovery.md). Vygenerujte klíč pomocí `openssl rand -hex 32` a uložte jej mimo server, dříve než se budete spoléhat na jakoukoli zálohu.

## Záloha VM se nepřipojí

Záloha VM komunikuje s libvirt přes SSH, nikdy přes připojení.

- Potvrďte, že SSH je povoleno na hostiteli a veřejný klíč BombVaultu je autorizovaný v `/root/.ssh/authorized_keys` (Nastavení, Systém, Záloha VM přes SSH zobrazuje klíč a tlačítko **Otestovat připojení**).
- Na vlastní síti `br0.x` nastavte `LIBVIRT_HOST` na svou LAN IP Unraidu (kontejner tam nemůže dosáhnout na hostitele přes `host.docker.internal`). Povolte **Nastavení, Docker, Host access to custom networks**.
- Pokud jste změnili SSH port Unraidu, nastavte `LIBVIRT_SSH_PORT`, aby odpovídal.
- Kompletní krok za krokem diagnóza (test dosažitelnosti, směrování VLAN, `Permission denied (publickey)`, `Host key verification failed`) je v [průvodci Záloha VM přes SSH](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md).

## Živý snímek VM neproběhl

Živé snímky potřebují qemu guest agent nainstalovaný ve VM a disk na `/mnt/cache` (nebo `/mnt/diskX`), nikoli `/mnt/user`. Na vypnuté VM se živý automaticky vrátí k šetrnému. Šetrná záloha VM vypne, zálohuje disky, poté ji znovu spustí, takže je vždy konzistentní.

## Záloha selhala s "repository is already locked"

Toto je obvykle osiřelý restic zámek zanechaný, když byl kontejner aktualizován nebo restartován uprostřed operace. BombVault detekuje prokazatelně osiřelý zámek, násilně jej vyčistí a jednou zopakuje, automaticky. Pokud přetrvává, použijte **Nastavení, Integrita a údržba, Odemknout** pro postiženou doménu k ručnímu vyčištění zaseklého zámku. Skutečný problém se stále objeví, místo aby byl skryt.

## Moje kopie mimo lokalitu neproběhla po záloze

Replikace mimo lokalitu je na základě nejlepší snahy záměrně, takže zádrhel mimo lokalitu nikdy nezhatí místní zálohu. Zkontrolujte plán mimo lokalitu pro danou doménu (Nastavení, Plány): prázdný plán replikuje po každé místní záloze, zatímco kadence odesílá méně často. Použijte **Replikovat nyní** v záložce Mimo lokalitu pro běh na vyžádání a sledujte indikátor replikace na Přehledu.

## Obnova se přerušila dříve, než začala

Než se cokoli zastaví nebo odebere, obnova spustí předletovou kontrolu konfliktů: ověří, že statická IP kontejneru a publikované hostitelské porty jsou volné. Pokud je jeden z nich již držen jiným kontejnerem, přeruší se s jasnou, akceschopnou zprávou místo toho, aby zanechala napůl dokončenou obnovu. Uvolněte konfliktní port nebo IP, poté opakujte.

## Prostý export selhal místo zapsání souboru

Pokud je šifrování age zapnuto (Nastavení), ale není nastaven platný příjemce, export selže s jasnou chybou místo zapsání prostého textu. Přidejte platného příjemce (veřejný klíč age nebo veřejný klíč SSH), nebo vypněte šifrování, pokud zamýšlíte, aby export byl prostý text. Viz [Funkce](features.md).

## Dump databáze selhal

Neúspěšný dump nikdy nepoloží zálohu kolem sebe; zapíše se jako vlastní neúspěšný běh a důvod říká, co spravit.

- **Odmítnuté přihlášení.** Dump se přihlašuje proměnnými s heslem samotného kontejneru (`POSTGRES_PASSWORD`, `MARIADB_ROOT_PASSWORD`, `MYSQL_ROOT_PASSWORD` nebo jejich verzemi `_FILE`). Zkontrolujte je na kontejneru s databází. Proměnná `_FILE` ukazující na tajemství, které uživatel kontejneru nesmí číst, dopadne stejně.
- **Chybějící oprávnění.** S náhodným heslem roota se dump přihlásí jen jako uživatel aplikace, obsáhne tedy jen tu jednu databázi, a MySQL 8.4 a novější ho může odmítnout úplně. Dejte kontejneru skutečné heslo roota, nebo mu dump vypněte.
- **Systémové tabulky potřebují povýšit.** MariaDB odmítá dump, když její systémové tabulky pocházejí ze starší verze (chyba 1558). Přidejte proměnnou `MARIADB_AUTO_UPGRADE=1` a restartujte kontejner, nebo v něm jednou spusťte `mariadb-upgrade`.
- **Žádný nástroj pro dump.** Odlehčený nebo vlastnoručně sestavený image bez `pg_dump`, `mysqldump` či `mariadb-dump` dumpovat nejde. Vezměte oficiální image, nebo dump vypněte.
- **Časový limit.** Dump dostane `DB_DUMP_MAX_HOURS` (výchozí 6), záloha kolem něj `BACKUP_MAX_HOURS`, a dump, který přestane postupovat, se utne po `BACKUP_STALL_HOURS`. Za posledním případem bývá zámek držený aplikací. Zvedněte limit, který zabral, nebo dumpujte, když je aplikace v klidu.
- **Kontejner je pozastavený nebo se restartuje.** Dump mluví s běžícím serverem. Pokud se kontejner restartuje pořád dokola, jeho vlastní log řekne proč.
- **Poškozený dump se nepodařilo odstranit.** Dump, který BombVault nedokázal dokončit, se zase smaže. Když se to smazání nepovede, dump zůstane v seznamu označený jako poškozený a můžete ho tam smazat.

## Import selhal

Import zastaví kontejner, odsune jeho datovou složku stranou a nechá image vytvořit na jejím místě prázdnou. Selže-li krok před samotným importem, stará složka se vrátí sama. Selže-li import, kontejner si nechá čerstvou složku a stará zůstane vedle jako `<datová složka>.bombvault-before-import-<časová značka>`; chybová hláška běhu uvádí přesnou cestu.

Ruční návrat: zastavte kontejner, přejmenujte současnou datovou složku stranou, přejmenujte zachovanou složku zpět na původní název a kontejner spusťte. Na Unraidu to zvládne správce souborů na kartě Shares.

## Asistent s umělou inteligencí se nepřipojí

Stránka [Server MCP](mcp.md#troubleshooting) uvádí, co znamená každý stavový kód a každé odmítnutí koncového bodu MCP a co s tím dělat.

## Kontejner se stále restartuje nebo vypadá unhealthy

BombVault hlásí healthy/unhealthy ze svého vlastního `/api/health`. Nástroj pro automatické hojení (například Autoheal) jej může restartovat automaticky, pokud se engine kdy zasekne. Zkontrolujte log kontejneru a report `/spike` pro základní příčinu.

## Stále zaseknutí?

- Přečtěte si kompletní stránky [Konfigurace](configuration.md) a [Mimo lokalitu a obnova](offsite-recovery.md).
- Zeptejte se ve [vlákně podpory Unraid](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/).
- Otevřete [GitHub issue](https://github.com/junkerderprovinz/bombvault/issues).
