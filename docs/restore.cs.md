# Obnova

Každá obnova začíná na stránce **Obnova**: vyber sekci, vyhledej, co chceš zpátky, a zvol zálohu. Jedno okno obnovuje každý druh položky a ptá se po krocích. Ve výchozím stavu se na originálu nic nezmění, protože se záloha vrací vedle něj.

![Stránka Obnova se sekcemi jako dlaždicemi, vyhledáváním a kartou Stacky.](assets/screenshots/restore/page-dark.png#only-dark)
![Stránka Obnova se sekcemi jako dlaždicemi, vyhledáváním a kartou Stacky.](assets/screenshots/restore/page-light.png#only-light)

## Stránka Obnova {#page}

- **Sekce** vypisuje sekce jako dlaždice, u každé s počtem položek, které mají snapshoty.
- **Co chceš vrátit?** prohledává položky vybrané sekce. Každý řádek ukazuje svůj nejnovější snapshot a **Otevřít položku** vede na stránku položky.
- **Najít soubor** hledá soubor nebo složku podle názvu ve všech zálohách, viz [Hledání souboru](#find-a-file).
- **Stacky** vypisují projekty Compose s nejméně dvěma zálohovanými kontejnery, například `immich` s databází a Redisem. Vyhledávání najde stack podle jeho jména nebo podle kteréhokoli z jeho kontejnerů.
- **Právě běží** ukazuje obnovu, dokud běží, i poté, co jsi zavřel její okno.
- **Nouze** spustí Nouzového průvodce pro server, který je pryč nebo byl nastaven znovu, viz [Nouzový průvodce](offsite-recovery.md#emergency).
- **Další zdroje** drží **Ze staré složky se zálohami**, pro zálohy jiného BombVaultu, **Import z pluginu Appdata.Backup** a **Úlohy z Duplicati nebo Duplicacy**.

![Stránka Obnova v telefonu.](assets/screenshots/restore/page-phone-dark.png#only-dark)
![Stránka Obnova v telefonu.](assets/screenshots/restore/page-phone-light.png#only-light)

## Okno obnovy {#window}

Klik na položku otevře **Obnova** s jejím jménem. Ptá se po krocích a každý krok se dá před spuštěním změnit:

1. **Záloha**: doporučený snapshot je předvybraný. Je to ten nejnovější, nebo po otevřené anomálii poslední před ní. Záloha pořízená před dřívější obnovou obsahuje stav, který ta obnova nahradila, takže se přeskočí, pokud není žádná jiná. Starší jsou v seznamu a **Porovnat** ukáže, čím se dva z nich liší.
2. **Zdroj**: odkud soubory pocházejí, když snapshot leží na více místech. **Místní** je nejrychlejší.
3. **Co**: **Vše**, nebo **Jednotlivé soubory**, kde zaškrtneš soubory a složky, které chceš.
4. **Kam**: cíl. Možnosti závisejí na druhu položky:

    | Cíl | Co dělá |
    |---|---|
    | **Vedle originálu** | Výchozí. Kontejner se vrátí jako druhý kontejner jménem `<jméno>-restored` s vlastními porty, disky a NVRAM VM skončí ve složce `<složka>-restored` a sada složek se vrátí jako `<cesta>-restored`. Na originálu se nic nemění. |
    | **Nahradit originál** | Přepíše to, co tam je teď. S **Nejdřív zálohovat současný stav**, které je ve výchozím stavu zapnuté, BombVault nejdřív zazálohuje dnešní stav, abys mohl zpět. Pokud ta záloha selže, obnova se nespustí. Funguje to i pro kontejner nebo VM, které ze serveru zmizely: složky, které obnova přepíše, se zazálohují samostatně. |
    | **Do složky** | Jen soubory, k prohlížení. |
    | **Stáhnout jako ZIP** | Flashka, pro novou flashku. BombVault nikdy nezapisuje do běžící flashky. |
    | **Do nové datové sady** a **Obnovit do datové sady** | Datové sady ZFS, viz [Datové sady ZFS](zfs-datasets.md#restore). |

5. **Co se stane** říká slovy, co obnova udělá, a scéna ukazuje cíl, který jsi vybral.

Než se **Obnova** odemkne, okno zkontroluje, že repozitář odpovídá, že ho klíč otevře, že snapshot existuje a že na cíli je místo. Dokud některá kontrola selhává, tlačítko zůstává zamčené a (i) v něm říká, která.

Okno také ukazuje, co obnova mění oproti tomu, co tam je teď: nové, nahrazené a nezměněné soubory, se seznamem na požádání, a soubory na cíli, které v záloze nejsou a zůstanou tam, kde jsou. Restic to spočítá jako zkušební běh podle velikosti a času změny, aniž by soubory četl, a velmi velký strom se po 30 sekundách zastaví a řekne to. U kontejnerů a VM porovnává nastavení, která obnova znovu vytváří, s běžícími: image a tag, porty, jména proměnných a svazky, nebo paměť, vCPU, disky a síť.

Některé věci mohou obnovu zastavit dřív, než cokoli změní. Statická IP kontejneru a jeho publikované porty musí být volné a jeho síť musí existovat. Na Unraidu každá obnova na místě odmítne složku, která není na připojeném poolu nebo sdílené složce, protože soubory by skončily v paměti a po restartu by byly pryč. Obnova na místě jmenuje každý další kontejner, jehož bind mount sahá do složky, do které se zapisuje, například „tato cesta se používá i v nextcloud-db“, ale to jen varuje.

Několik voleb se objeví tam, kde dávají smysl: **Poté nechat zastavené**, abys kontejner nebo VM mohl prohlédnout, než poběží znovu, a **Obnovit bez GPU a běhového prostředí** na hostiteli, kterému chybí GPU nebo běhové prostředí, s nímž byla záloha vytvořena.

Obnova běží na pozadí. Okno můžeš zavřít a průběh se ukazuje i pod **Právě běží**. Dlouhá obnova ukazuje procenta a dá se zrušit a zrušená obnova se zapíše jako zrušená, ne jako neúspěšná. Obnova zrušená, zatímco se kontejner zastavuje, nebo obnova, která nemůže kontejner odstranit, ještě nic neobnovila, takže se kontejner znovu spustí. Kontejner, který bere svazky z jiného přes `--volumes-from`, si je ponechá a obnova odmítne začít, dokud ten druhý kontejner neexistuje.

![Okno obnovy v kroku Co, se zaškrtnutými jednotlivými soubory.](assets/screenshots/restore/window-what-dark.png#only-dark)
![Okno obnovy v kroku Co, se zaškrtnutými jednotlivými soubory.](assets/screenshots/restore/window-what-light.png#only-light)

### Kam míří obnova do složky {#folders}

**Složky pro obnovu** v **Nastavení, Obnova** nastavují **Výchozí složka pro obnovu**, s **Podsložka pro každou sekci**, pokud chceš, a **Výjimka** pro jednu sekci. Vybrané soubory skončí přímo v cílové složce: jeden soubor se vrátí jako jen ten soubor, složka jako složka s obsahem a výběry z různých složek si ponechají jen svou cestu pod složkou, kterou sdílejí. BombVault odmítne cíl, který není na připojeném úložišti, protože na Unraidu taková složka žije v paměti a po restartu je pryč, a nikdy neobnovuje do vlastní datové složky.

## Příklad: dostat zpět jeden soubor {#example-single-file}

Někdo včera v noci upravil `config.php` Nextcloudu a Nextcloud už nenastartuje. Noční záloha ve 03:00 má dobrý soubor ještě v sobě. Chceš ten jediný soubor, ne celý kontejner.

1. Otevři **Obnova**, vyber dlaždici **Kontejnery** a napiš `nextcloud` do **Co chceš vrátit?**.
2. Klikni na `nextcloud`. Okno se otevře s předvybranou zálohou ze 03:00 jako doporučenou.
3. Nech **Zdroj** na **Místní**.
4. V kroku **Co** zvol **Jednotlivé soubory**. Otevři složky až k `config` a zaškrtni `config.php`.
5. V kroku **Kam** zvol **Do složky** a vyber složku, například `user/restore`, což je na hostiteli `/mnt/user/restore`.
6. Přečti si **Co se stane**: soubor skončí v cílové složce a na kontejneru se nic nemění. Stiskni **Obnova**.

Okno skončí hláškou **Hotovo: soubory jsou v /mnt/user/restore**. Soubor najdeš jako `/mnt/user/restore/config.php`, bez složek nad ním. Porovnej ho s rozbitým, zkopíruj ho zpět do složky appdata a restartuj Nextcloud.

Chceš-li soubor vrátit rovnou na místo, zvol v kroku 5 **Nahradit originál**. Nech zapnuté **Nejdřív zálohovat současný stav**, aby BombVault zazálohoval rozbitý stav dřív, než zapíše, napiš `nextcloud` pro potvrzení a stiskni **Obnova**. Nextcloud se zastaví, zatímco se vybraný soubor zapisuje zpět na své místo, a potom se znovu spustí, a zapíšou se jen soubory, které jsi zaškrtl. Obnova, která selže v půlce, nechá Nextcloud zastavený, protože soubor, který čte, může být napůl zapsaný, a běh to řekne.

![Okno obnovy v kroku Co se stane, s vybraným Nahradit originál.](assets/screenshots/restore/window-replace-dark.png#only-dark)
![Okno obnovy v kroku Co se stane, s vybraným Nahradit originál.](assets/screenshots/restore/window-replace-light.png#only-light)

## Hledání souboru {#find-a-file}

Když znáš název souboru, ale nevíš, která položka nebo záloha ho ještě má, napiš název do **Najít soubor** a stiskni **Hledat**. Obyčejný text se najde kdekoli v názvu a na velikosti písmen nezáleží, takže `invoice` najde `Invoice-2024.pdf`. S `*` nebo `?` musí vzor odpovídat celému názvu, takže `*.pdf` najde každé PDF. Napiš jen název, bez složky.

Hledání čte každou zálohu kontejnerů, sad složek, datových sad ZFS a flashky. Každý nález ukazuje položku, cestu a všechny zálohy, které soubor obsahují, od nejnovější, s datem, velikostí a místem, odkud se četla. Záloha, ve které má soubor stejnou velikost a čas změny jako v té předchozí, říká **beze změny**, takže poznáš, kde se soubor naposledy změnil. Klik na zálohu otevře na ní okno obnovy s vybranými **Jednotlivé soubory** a zaškrtnutým souborem. Soubor z flashky se vrátí jako součást ZIPu flashky, protože flashka se obnovuje jen celá.

Hledání chvíli trvá, protože restic prochází každou zálohu. **Zastavit** ho ukončí, stejně jako odchod ze stránky. Pod polem je vidět, která místa přečetlo:

- Místa na tomto serveru a ve tvé síti se prohledávají vždy.
- Záloha, která leží na více místech, se čte jen jednou, na prvním z nich. Místo, jehož zálohy se všechny přečetly jinde, říká **už prohledáno na jiném místě**.
- Místa mimo lokalitu jako S3, B2 nebo SFTP se vynechají, dokud nezapneš **Hledat i na místech mimo lokalitu**. Pak se tam čtou jen zálohy, které nemá žádné místo tady, obvykle ty starší, které delší uchovávání drží mimo lokalitu. První hledání na takovém místě může trvat dlouho a někteří poskytovatelé si stažená data účtují.
- Místo, které nešlo otevřít, říká **nešlo přečíst** a (i) vedle něj řekne proč. Ostatní místa se prohledají i tak.

Virtuální stroje se ukládají jako celé disky, proto se soubory v nich neprohledávají. Neprohledává se ani autozáloha a zálohy jiného BombVaultu pod **Další zdroje**. Hledání vypíše prvních 200 souborů, takže krátký název jako `log` může potřebovat víc písmen.

## Co se vrací, podle druhu {#kinds}

**Kontejnery** se znovu vytvářejí z uložené definice přes API Dockeru, s entrypointem, hostname, nastavením zastavení, svazky a mounty, limity prostředků, ovladačem logů, nastavením DNS a staršími linky. Kontejner se vrátí na image, na kterém běžel v době zálohy, ne na novějším, stáhnutém později, a chybějící image se nejdřív stáhne, s přihlášením z **Registry kontejnerů** v **Nastavení, Obnova**. Vrací se i jeho GPU nebo runtime (`--gpus`, `--runtime=nvidia`). Na hostiteli bez tohoto ovladače nebo runtime to obnova řekne a nabídne **Obnovit bez GPU a běhového prostředí**. Link na kontejner, který chybí nebo je zastavený, když startuje obnovený, se vynechá a historie běhu to řekne. Obnova, která selže poté, co byl kontejner odstraněn, ho vrátí zpět, zastavený.

Záloha může obsahovat složky, které kontejner už nemá vybrané, například starší záloha pořízená dřív, než nějaká složka zmizela. Složka, která zmizela nebo je prázdná, se vrátí. Ta, ve které teď jsou soubory, zůstane, jak je, protože jsi ji možná odškrtl záměrně, a kontrola před obnovou i historie běhů ji jmenují. Vrátíš ji pomocí **Jednotlivé soubory** nebo **Do složky**.

**VM** dostanou své XML znovu definované přes SSH a své disky a UEFI NVRAM znovu připojené, i když byla VM smazána. Pro VM se snapshoty Unraidu a disky na zvolech viz [Zálohy VM](vm-backup-ssh-setup.md).

**Sady složek** se vrací s vlastnictvím, režimy a časovými značkami, jak je restic ukládá. Kde cílová složka odmítne jen vlastnictví, jako to dělá `/mnt/user`, běh skončí poznámkou, která to říká. BombVault pak porovná soubory se zálohou a soubor, který nedorazil, způsobí selhání obnovy.

**Flashka** se nikdy nedotkne živého `/boot`. Obnova je stažení s názvem `flash-<id>.zip`, připravené k vložení do Unraid USB Creator. ZIP obsahuje `APP_KEY`, takže jeho stažení vyžaduje přihlašovací heslo a přihlášení přímo na instanci. Okno také vypisuje pluginy v každé záloze flashky s jejich verzí a velikostí a umí vrátit jediný z nich do běžící flashky: jeho soubor `.plg`, jeho složku pod `config/plugins` a soubory balíčků, které záloha drží. Unraid ho nainstaluje při dalším startu, nebo hned pod **Plugins, Install Plugin**.

**Datové sady ZFS** mají vlastní cíle a zachovávají své vlastnosti ZFS, viz [Obnova](zfs-datasets.md#restore).

**Autozáloha** se vrací jen přes Nouzového průvodce, který BombVault restartuje, takže živá databáze se nikdy nepřepíše pod otevřeným handlem.

**Výpisy databáze** se nikdy nenahrávají zpět samy. Řádek pod **Výpisy databáze** lze stáhnout, uložit do složky nebo naimportovat do čerstvě spuštěné databáze. Viz [Výpisy databází](backups.md#database-dumps).

## Obnova více položek najednou {#several}

Na stránce Obnova vybírej položky tak, jak vybíráš řádky v seznamu Záloh: Ctrl+klik, Shift+klik, Ctrl+A, dlouhý stisk, nebo **Vybrat**. **Obnova** pak otevře jedno okno pro všechny.

- **Která záloha**: **Doporučená** vezme u každé položky zálohu, kterou by doporučilo její vlastní okno. **Jeden okamžik pro všechny** vezme u každé položky její nejnovější zálohu z **Okamžik**, který vybereš, nebo dřívější, například večer předtím, než se něco pokazilo.
- **Kam**: **Vedle originálu** nebo **Nahradit originál**, s **Nejdřív zálohovat současný stav**.

Každá položka se zkontroluje tak jako jednotlivá obnova a pak běží jedna po druhé. Každý řádek ukazuje, jak dopadla ta jeho: **Čeká**, **Probíhá**, **Obnoveno**, **Selhalo** nebo **Vynecháno**. **Zastavit po této** nechá běžící obnovu doběhnout a další už nespustí. Flashka přijde jako ZIP ke stažení a datové sady ZFS a autozáloha se obnovují po jedné.

![Obnova více položek: jeden řádek na položku s jejím průběhem.](assets/screenshots/restore/several-dark.png#only-dark)
![Obnova více položek: jeden řádek na položku s jejím průběhem.](assets/screenshots/restore/several-light.png#only-light)

## Obnova stacku {#stacks}

Klik na stack v kartě **Stacky** otevře **Obnovit stack**. Obnoví každého člena z jeho nejnovější zálohy současně a s **Po obnovení spustit v pořadí závislostí** je spustí v pořadí podle `depends_on`: nejdřív databáze a cache, pak aplikace. Když složka projektu některého člena na vybraném místě chybí, okno nabídne místo toho stav z Místní.

BombVault nejdřív zkontroluje členy a složku projektu a obnova, která by nemohla fungovat, nic nezmění. Jinak zastaví běžící členy, aplikace před jejich databázemi, potom zapíše složku projektu a potom obnoví členy. Pokud se složka projektu nevrátí, členové se přesto obnoví, ale zůstanou zastavení, protože složka může být napůl zapsaná a obvykle v ní leží bind mounty členů. Běh každého člena to řekne.

Databáze, která má jen výpisy databáze a žádnou zálohu souborů a jejíž kontejner i pojmenovaný svazek zmizely, se vrátí bez svých dat. Kontrola to řekne před obnovou. Zůstane zastavená a stejně tak kontejnery, které na ní závisí, aby žádná aplikace nespustila úvodní nastavení nad prázdnou databází. Spusť ji, naimportuj výpis a spusť ostatní. Viz [Výpisy databází](backups.md#database-dumps).

## Další zdroje {#other-sources}

- **Ze staré složky se zálohami** otevře repozitář jiného BombVaultu, který už neběží, na disku nebo v cíli mimo lokalitu, s jeho sadou pro obnovu nebo napsaným `APP_KEY`. Relace je jen pro čtení a žije jen v paměti. Viz [Ze staré složky se zálohami](offsite-recovery.md#restore-from-another-bombvault-repo).
- **Import z pluginu Appdata.Backup** čte složku záloh pluginu. Každý archiv kontejneru se stane bodem obnovy svého kontejneru, datovaným podle toho, kdy ho plugin vytvořil. Dříve importované archivy se přeskočí a samotné archivy se jen čtou. Kontejner potřebuje v BombVaultu nejdřív jednu zálohu, aby měla obnova jeho definici. Uchovávání importované body obnovy nechává na pokoji, takže jeden smaž sám, až ho nebudeš potřebovat.
- Sekce **Úlohy z Duplicati nebo Duplicacy** převede zálohovací úlohy těchto nástrojů na sady složek. Čte přitom jen jejich nastavení. Staré zálohy zůstávají u starého nástroje a BombVault začne vlastní první zálohou. Ani jeden z nástrojů tu nemusí být nainstalovaný.
    - **Z Duplicati**: v Duplicati exportuj každou úlohu do souboru bez hesla a vyber soubory `.json`. Export, který obsahuje hesla, přenese i přihlašovací údaje k cíli.
    - **Z Duplicacy**: vyber složku Duplicacy Web, ve které je `duplicacy.json`, nebo složku, kterou zálohuješ z příkazové řádky Duplicacy a ve které je `.duplicacy`. Klíče, které příkazová řádka uložila do `.duplicacy/preferences`, se přenesou. Duplicacy Web ukládá své přihlašovací údaje šifrovaně, takže si o ně okno řekne.

    Nic se nevytvoří, dokud neuvidíš náhled. Z každé zdrojové složky vznikne sada složek se složkou na tomto serveru, kde byla nalezena, s vyloučeními, která šlo převést, a s plánem. Když starý nástroj běžel ve vlastním kontejneru a nějaká složka se nenašla, vybereš ji sám. Zálohy půjdou do nového cíle ve složce `bombvault` uvnitř starého cíle, a to pro místní složku, S3, SFTP, remote rclone nebo Backblaze B2 přes jeho adresu S3. Přihlašovací údaje ze souboru se uloží k tvým ostatním přihlašovacím údajům a o ty, které v souboru chybí, si okno řekne. Poznámky pod každou úlohou říkají, co se nepřeneslo, například pravidla zahrnutí, regulární výrazy, jak dlouho se zálohy uchovávaly, a staré šifrovací heslo, které BombVault nepotřebuje.

## Po obnově {#after}

Kopie obnovená vedle originálu je vypsaná pod **Zálohy** jako vyloučená, takže se nepočítá jako otevřený úkol. Přidej ji do plánu, pokud ji chceš udržet. Kontejner nebo VM, které v době zálohy běžely, se vrátí běžící, zastavené zůstane zastavené. Na Unraidu se obnovený kontejner znovu objeví v záložce Docker jako normální, upravitelná aplikace a VM ve VM Manageru s připojenými disky a NVRAM.
