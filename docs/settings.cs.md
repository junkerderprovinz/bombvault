# Nastavení

**Nastavení** je čtrnáct dlaždic. Každá dlaždice je stránka karet a každá změna se zapíše hned, jak ji uděláš, s krátkým probliknutím ovládacího prvku, nebo zachvěním, když ji server odmítne. Několik oken si nechává tlačítko **Uložit**, protože uložení napůl hotového by nebylo bezpečné, například přihlášení nebo přihlašovací heslo. Vysvětlení najdeš v bublinách (i) vedle každého ovládacího prvku.

Stiskni `/` nebo Ctrl+K kdekoli v nastavení a prohledáš všechna, i podle názvů, které stránky měly v dřívějších verzích. Pořadí dlaždic se dá změnit pod **Vzhled**. Tam, kde by sloupec dlaždic vedle stránky stránku stlačil, třeba v okně široké asi 1200 pixelů nebo v telefonu, se dlaždice změní v pás nad stránkou, který se posouvá do stran.

![Nastavení s otevřenou dlaždicí Obecné.](assets/screenshots/getting-started/settings-rail-dark.png#only-dark)
![Nastavení s otevřenou dlaždicí Obecné.](assets/screenshots/getting-started/settings-rail-light.png#only-light)

Tato stránka vypisuje každou dlaždici a její karty a odkazuje na stránku, která každou z nich vysvětluje do hloubky.

## Obecné {#general}

- **Sekce** zapíná a vypíná každou sekci: **Kontejnery**, **VM**, **Flash**, **Složky**, **Datové sady ZFS** a **Autozáloha**. Sekce, která je vypnutá, se nezálohuje podle žádného plánu, ani podle vlastního plánu položky, a její položky se nepočítají. **Zálohovat nyní** pořád funguje. **Flash** se ukazuje jen na Unraidu.
- **Kontejnery** drží to, co platí pro každý kontejner, například **Automatické výpisy databází**. Cokoli, co se týká jednoho kontejneru, je na jeho stránce. Viz [Výpisy databází](backups.md#database-dumps).

## Vzhled {#look}

- **Vzhled**: **Jazyk** (42 jazyků), **Motiv** (**Tmavý**, **Světlý** nebo **Systémový**), **Rohy**, **Animace** (**Vypnuto**, **Jemné**, **Divoké** nebo **Bouřlivé**), **Tichá oznámení**, která skrývají oznámení o úspěchu, například potvrzení uložení, a **Pořadí stránek nastavení**, kde dlaždici podržíš a přetáhneš na místo, nebo použiješ Alt a šipky.
- **Popisky**: kolik z ovládacího prvku se ukáže, zvlášť pro **Tlačítka**, **Postranní panel**, **Karty** a **Spodní panel**: **Text**, **Text a symbol**, **Symbol**, nebo **Reaktivní**, které ukáže jen symbol a slova vysune, když na něj ukážeš. **Postranní panel** ovlivňuje jen rozvržení na počítači a **Spodní panel** jen rozvržení v telefonu.
- **Barvy**: jedna **Barva zvýraznění**, nebo **Duhový režim**, kde každý řádek seznamu dostane vlastní barvu z palety osmi. Duhový režim přidává **Režim disko**, kde barvy pomalu přeplouvají na další, **Reaktivní režim**, kde se barva řádku ukazuje, jen když na něj ukážeš, běží nebo je vybraný, **Rotace barev** a samotnou **Barevnou paletu**.

BombVault drží vzhled na serveru. Prohlížeč, který přijde poprvé, ho převezme. Když tvůj systém žádá méně pohybu, vždy to vyhraje nad **Animace**.

## Plán {#schedule}

**Týden**, **Úplná záloha** s **Kdy zálohy běží**, **Dohnat zmeškané zálohy po startu** a **Příkazy před a po**, **Plány podle sekcí** s **Kopie mimo lokalitu podle sekcí**, **Naplánované položky**, **Pořadí** s **Restart po záloze**, **Snímek místo zastavení**, **Aktualizace po záloze** a **Klid před zálohou**. Viz [Plány](schedules.md).

## Cíle {#storage-locations}

- **Zobrazit také v postranním panelu** připne stránku Cíle do postranního panelu.
- Seznam tvých cílů s **Přidat cíl**. Klik otevře stránku cíle, viz [Stránka cíle](offsite-recovery.md#location-page).
- **Přihlašovací údaje**: uložená přihlášení s výchozí hvězdičkou, viz [Přihlašovací údaje](offsite-recovery.md#sign-ins).
- **Cíle podle sekcí**: tok 3-2-1, kde je každý řádek přepínač, viz [Výchozí nastavení podle sekcí](offsite-recovery.md#placement-defaults).
- **Repozitáře**: repozitáře vytvořené pro jednotlivé položky, viz [Umístění podle položky](offsite-recovery.md#placement).
- **Jak dlouho uchovávat** ukazuje na cíle, kde se uchovávání nastavuje, a **Náhled** ukáže **Co by odstranilo příští spuštění**, podle repozitáře a podle položky, než se to stane. Nic nemění a nic nezamyká, takže odpoví i za běhu zálohy.
- **Kopie mimo lokalitu**: jeden řádek na sekci s **Otestovat připojení**, **Replikovat nyní** a **Nastavit** nebo **Upravit**, viz [Kopie mimo lokalitu](offsite-recovery.md#off-site-copies).
- **Šířka pásma mimo lokalitu**: **Limit nahrávání (KiB/s)** a **Limit stahování (KiB/s)** pro každou kopii. 0 znamená bez omezení.

Karta **Streamování**, která snižuje limit nahrávání, zatímco streamuje Plex, Jellyfin nebo Emby, sedí přímo na stránce Cíle.

## Obnova {#restore}

- Odkaz na stránku **Obnova**.
- **Složky pro obnovu**: **Výchozí složka pro obnovu** pro obnovu do složky, s **Podsložka pro každou sekci** a **Výjimka** pro každou sekci. Viz [Kam míří obnova do složky](restore.md#folders).
- **Registry kontejnerů**: přihlášení k soukromým registrům, například k image `ghcr.io` za sponzorskou závorou. Obnova je používá ke stažení chybějícího image a aktualizace po záloze ke zjištění novějšího. Veřejné image se stahují bez něj. **Přidat registr** nabízí veřejné registry, vlastní hostované, ty u cloudového poskytovatele a **Jiný registr** pro jakoukoli adresu.
- **Pro nouzové případy**: **Stáhnout sadu pro obnovu**, která vyžaduje přihlašovací heslo. Viz [Sada pro obnovu](offsite-recovery.md#recovery-kit).

## Oznámení {#notifications}

**Oznámení** s **Oznámit**, souhrny, **Hlásit zálohy po termínu**, **Oznámení Unraid** a **Týdenní souhrn**, **Kanály** s Webhookem, Apprise, Matrixem, Gotify a e-mailem (SMTP) a **Healthchecks**. Viz [Oznámení](notifications.md).

## Ověření {#verify}

- **Zkontrolovat zálohy**: výsledek posledního `restic check` pro každou sekci, s **Odemknout** a **Vyčistit** pod **Více** v každém řádku. **Odemknout** uvolní zastaralý zámek a **Vyčistit** použije pravidla uchovávání na požádání.
- **Automatické kontroly**: plánovaná kontrola obnovy s **Vzorek dat (%)** a testovanými cíli, **Test spuštění**, který také obnoví jeden kontejner do izolované kopie a spustí ho, **Naplánované cvičení DR mimo lokalitu** a **Kontrola ochrany proti smazání**, plánovaný tamper test. Viz [Cvičení DR](offsite-recovery.md#dr-drills) a [Tamper test](offsite-recovery.md#tamper-test).
- **Anomálie**: rozpoznávání, citlivost a které nálezy pošlou zprávu. Viz [Anomálie](anomalies.md).

## Zabezpečení {#security}

- **Heslo**: přihlašovací heslo, nejméně 12 znaků. Změna nebo smazání žádá o současné a při zapnutém dvoufaktorovém ověření i o kód.
- **Dvoufaktorové ověření**: kód z aplikace autentizátoru navíc k heslu, s osmi záložními kódy zobrazenými jednou.
- **Přístupové klíče**: přihlášení klíčem v telefonu, notebooku nebo na bezpečnostní tyčince. Potřebují skutečné doménové jméno a certifikát, kterému prohlížeč důvěřuje.
- **Aplikace pro Android**: **Telefony se mohou spárovat s dvanácti slovy skupiny**, což spárovanému telefonu dovolí přihlásit se bez hesla.
- **Šifrování**: **Šifrovat zálohy**, ve výchozím stavu zapnuto.

Několik věcí vyžaduje přihlašovací heslo, například sada pro obnovu, uložení klíčů u strážce a trvalé mazání záloh. Viz [Bezpečnostní model](configuration.md#security-model).

## Instance {#instances}

- **Zobrazit také v postranním panelu** připne stránku Instance do postranního panelu.
- **Více serverů** zapíná části skupiny: **Instance**, stránku s kartou pro každého člena, **Přijímač** a **Stažení**. Viz [Instance a role](offsite-recovery.md#roles).
- Odkaz na stránku Instance, s částmi, které jsou zapnuté.

## Párování {#pairing}

**Párování** s **Vygenerovat frázi** nebo **Zadat frázi**, karty kroků, jméno této instance a **Známé adresy**, karta **Přeposílač** se svými čtyřmi podobami a **Co přeposílač vidí**, členové skupiny a **Server ZFS bez BombVault** pro cíl repliky přes SSH. Viz [Párování instancí](offsite-recovery.md#pairing) a [Push na server ZFS](zfs-datasets.md#replica-push).

## Integrace {#integrations}

- **Server MCP**: klíče pro asistenty AI, **Vybrat klienta** a log každého klíče. Viz [Server MCP](mcp.md).
- **Tokeny API**: pojmenované tokeny pro skripty a dashboardy. Viz [Tokeny](api.md#tokens).
- **Služby**: **Home Assistant** přes MQTT, **Najít v síti** (mDNS jako `bombvault.local`) a **Zpřístupnit /metrics** pro Prometheus, s **Scrape token (volitelné)**. Viz [Home Assistant](api.md#home-assistant) a [Najít BombVault v síti](api.md#mdns).
- **Widget pro dashboard**: token jen pro čtení a adresa pro libovolný dashboard, který umí zobrazit iframe, například Homepage nebo Heimdall, jen s živým protokolem aktivity. Viz [Widget pro dashboard](api.md#dashboard-widget).
- **SSH k hostiteli**: veřejný klíč BombVaultu a **Otestovat připojení**, pro zálohy VM a datových sad ZFS a oznámení Unraidu. Viz [průvodce zálohou VM](vm-backup-ssh-setup.md).

## Aplikace {#apps}

- **Aplikace pro Android**: APK pro vydání, se kterým tvůj server běží, s QR kódem, a záznam v F-Droidu. Viz [Aplikace pro Android](android.md#install).
- **ParleyPort**: kontejner přeposílače s jeho šablonou pro Unraid a příkazem Dockeru, který ho spustí.
- **BombVault Widget**: dlaždice na dashboardu Unraidu s protokolem aktivity BombVaultu a dalším během. S připojením SSH k hostiteli karta plugin sama nainstaluje a odebere. Bez něj ti podá adresu `.plg` k instalaci pod **Plugins, Install Plugin**.

## Systém {#system}

- **Export / import nastavení**: celá tvoje konfigurace jako přenosný soubor JSON, s přihlašovacími údaji nebo bez nich. Import nejdřív ukáže **Tento soubor obsahuje** a vynechá odkazy na cíle, které už neexistují. **Šifrovat exporty pomocí age** zapečetí obyčejné exporty, ZIP flashky a sadu pro obnovu pro **Příjemci age**, které vypíšeš. Viz [Přenosné nastavení](configuration.md#portable-settings-export-and-import).
- **Výkon**: **Vlákna CPU (0 = vše)**, **Limit velikosti mezipaměti (MB, 0 = bez limitu)** pro cache resticu pod `/config` a **Limit paměti na zálohu (MiB, 0 = žádný)**, měkký limit, který velkou zálohu zpomalí, místo aby ji nechal zabít pro nedostatek paměti.

## Info {#info}

- **Verze**: BombVault, GlimStone, restic a rclone, s **Hledat aktualizace** a **Nové**, které počítá změny, jež jsi ještě neviděl, a otevře okno, které je vypíše.
- **Nápověda**: **Příručka** otevře tuto dokumentaci a **Hlášení chyby** stáhne vyčištěné ZIP pro issue na GitHubu. Vyžaduje přihlašovací heslo.
- **O aplikaci BombVault**: projekt, jeho licence a jak autora zastihnout.
