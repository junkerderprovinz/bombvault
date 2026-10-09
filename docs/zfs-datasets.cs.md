# Datové sady ZFS

Stránka **ZFS** zálohuje datové sady ZFS. Položka je jedna datová sada spolu se všemi datovými sadami pod ní. Pro každou zálohu pořídí BombVault jediný snímek ZFS celého stromu, takže každá datová sada v něm je zachycena ve stejném okamžiku. Poté z tohoto snímku přečte soubory každé datové sady, uloží je pomocí resticu stejně jako složku a snímek hned potom odstraní. Zálohy jsou deduplikované, každou z nich můžete procházet a lze obnovit jednotlivé soubory.

Záloha nikdy neposílá proud do restic a nikdy nevrací datovou sadu do dřívějšího stavu. BombVault ničí jen snímky, které sám vytvořil. Volitelná [replika](#replica) je jediné místo, které používá `zfs send`: kopíruje datové sady na druhý server ZFS a zálohy se nedotýká.

## Požadavky {#requirements}

- **Spojení SSH s tímto serverem.** Datové sady ZFS používají stejný klíč, hostitele a uživatele jako zálohy VM. Pokud zálohy VM už fungují, funguje i tohle. Jinak postupujte podle [průvodce Záloha VM přes SSH](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) na GitHubu. Pole šablony se jmenují **Host SSH: Address**, **Host SSH: Port** a **Host SSH: User**.
- **Příkaz `zfs` na tomto hostiteli.** Unraid 6.12 a novější a TrueNAS SCALE ho mají.
- **Host Data namapované jako `/mnt` s Access Mode Read/Write - Slave.** To je výchozí hodnota šablony. Snímek datové sady se ve složce `.zfs/snapshot` datové sady objeví až po spuštění BombVaultu, takže kontejner musí dostávat připojení, která hostitel vytvoří později.
- **Datové sady připojené pod `/mnt`.** Na Unraidu jsou pooly pod `/mnt/<pool>`, takže to už platí.

Zapněte doménu v **Nastavení, Obecné** (Datové sady ZFS). Stránka ZFS pak zobrazí kartu **Spojení s tímto serverem**. Otestuje spojení SSH, uvede uživatele a hostitele, ke kterým se připojuje, a řekne, co chybí, když něco chybí. Kontrola integrace s hostitelem (`/spike`) ukazuje stejný výsledek.

## Položky a podřízené datové sady {#items-and-children}

Na stránce ZFS otevřete **Přidat datové sady**. Seznam pochází ze serveru. Vyberte datovou sadu, která je nejvýše z toho, co chcete zálohovat, například `cache/appdata`, a položka pokryje ji i každou datovou sadu pod ní.

- **Nové podřízené datové sady se přidávají samy.** Datová sada vytvořená později pod položkou se zálohuje při dalším běhu a ten ji uvede jako novou. Její první záloha ji jednou přečte celou; potom se čtou jen změny.
- **Jednotlivé podřízené sady můžete vynechat.** Vypněte jednu v nastavení položky a bude vynechána i se vším pod ní. Vynechaná podřízená sada, která už na serveru neexistuje, je tak označena a lze ji ze seznamu odebrat.
- **Podřízené sady, které nelze přečíst, se přeskakují, nikdy potichu.** Běh je vypíše, položka ukáže, kolik jich bylo přeskočeno, a karta pokrytí na přehledu počítá každou z nich jako nechráněnou. Běh přesto zálohuje vše ostatní a kvůli přeskočené sadě neselže. Důvody jsou v [tabulce kódů důvodů](#reason-codes): datová sada, která není připojená, má `canmount=off`, přípojný bod `legacy` nebo žádný, nenačtený šifrovací klíč, vypnutý přístup ke snímkům nebo přípojný bod, který BombVault nevidí.
- **Přeskočená datová sada nestrhne své podřízené sady s sebou.** Datová sada s `canmount=off`, která obsahuje jen jiné datové sady, se přeskočí (zobrazí se jako "jen struktura") a její připojené podřízené sady se zálohují. Šifrovaná datová sada s nenačteným klíčem se přeskočí spolu s podřízenými sadami, které sdílejí její klíč.
- **Podřízené sady, které jsou disky VM nebo systémová data, začínají vypnuté** v dialogu přidání, s důvodem vedle přepínače. Přidání celého poolu vyžaduje potvrzení, které vypíše, co obsahuje.

### Svazky {#volumes}

Svazek (zvol) obsahuje virtuální disk místo souborů a stránka ZFS žádný nezálohuje.

- Svazek, který používá VM, se zálohuje s touto VM na stránce **VMs**.
- Svazek, který nepoužívá žádná VM (iSCSI extent, odpojený disk), **BombVault nezálohuje**. Dialog přidání a stránka ZFS takové svazky spočítají a uvedou to. Pozdější verze je bude zálohovat.

Svazky ve stromu položky se při každém běhu přeskočí a uvedou.

### Úložiště Dockeru {#docker-storage}

S úložným ovladačem ZFS v Dockeru je každá vrstva obrazu datovou sadou s přípojným bodem `legacy`. Dialog přidání je sloučí do jednoho řádku na rodiče. Strom, který obsahuje víc než 20 takových datových sad, se nemůže stát položkou: dokud existuje jeho snímek, Docker nemůže odstraňovat vrstvy obrazů. Přidejte místo toho datové sady pod ním, například `appdata`.

### Položky se nikdy nepřekrývají {#overlap}

Datová sada může patřit jen k jedné položce. BombVault odmítne novou položku, která leží uvnitř existující nebo by nějakou obsahovala. Chcete-li sloučit několik podřízených položek do jedné nadřazené, nejprve podřízené položky smažte se zachováním jejich záloh a pak přidejte nadřazenou. Každá datová sada si ponechává historii pod svým jménem, takže další záloha pokračuje tam, kde staré položky skončily, a nečte všechno znovu.

## Zastavení kontejnerů a příkazy kolem snímku {#consistency}

Snímek běžící databáze je jako náhlý výpadek proudu: databáze se obvykle zotaví, ale musí to udělat. Každá položka s tím může udělat dvě věci a obě se týkají jen okamžiku snímku, ne celé zálohy.

- **Zastavit tyto kontejnery kvůli snímku.** BombVault zastaví uvedené kontejnery, pořídí snímek a hned je znovu spustí. Kontejnery na stejné úrovni závislostí se zastavují souběžně, závislé nejdřív, takže celé okno obvykle trvá pár sekund; běh ukáže, jak dlouho. Záloha pak čte zmrazený snímek, zatímco aplikace už zase běží. Zastavují se jen kontejnery, které běžely.
- **Příkaz před snímkem a po něm.** Běží uvnitř kontejneru podle vašeho výběru, například aby těsně před snímkem vypsal databázi do datové sady, aniž by cokoli zastavil. Selže-li příkaz před snímkem, záloha selže a žádný snímek se nepořídí. Selhání příkazu po snímku se ukáže u běhu, ale zálohu neshodí.

Co se stane, když se něco pokazí:

- Nelze-li kontejner zastavit, BombVault spustí ty, které už zastavil, a záloha selže s názvem kontejneru. Nikdy se nespokojí se snímkem běžících aplikací.
- Zastavení počká, až doběhne probíhající záloha kontejneru (až 30 minut u ručního běhu, až do časového limitu zálohy u plánovaného), aby oba nikdy současně nezastavovaly a nespouštěly stejný kontejner.
- Než se zastaví první kontejner, BombVault si zapíše, které zastavuje. Pokud je BombVault během okna ukončen, při příštím spuštění tyto kontejnery znovu spustí, pošle oznámení a položka ukáže červenou poznámku u každého kontejneru, který se nepodařilo spustit.

Automatické výpisy databází (viz [Funkce](features.md)) běží s vlastní zálohou kontejneru na stránce **Kontejnery**, ne s položkou ZFS. Databáze, jejíž kontejner se zálohuje jen přes jeho datovou sadu, výpis nedostane, takže jí zde zadejte příkaz.

Kontejner může být současně na tomto seznamu i na stránce **Kontejnery**. Jeho data se pak ukládají dvakrát, do dvou repozitářů, a **Záloha všeho** ho zastaví dvakrát. Položka na to upozorní.

## Obnova {#restore}

Otevřete **Zálohy** u položky, vyberte zálohu a pak datovou sadu. Výchozí je nejvyšší datová sada položky.

- **Obnovit do datové sady.** Soubory ze zálohy se zapíší do přípojného bodu datové sady. Soubory se stejným názvem se přepíšou, ostatní zůstanou. Datová sada se nikdy nevrací do dřívějšího stavu ani nenahrazuje. BombVault zkontroluje, že je datová sada připojená, viditelná a zapisovatelná, jednou před začátkem a znovu těsně před zápisem. Tam, kde je uvnitř připojená podřízená sada, se nic nezapisuje: ta si ponechá své soubory, vlastníka a oprávnění a obnoví se ze své vlastní zálohy.
- **Obnovit do složky.** Vyberte složku pod `/mnt`. BombVault zkontroluje, že složka leží na připojeném poolu nebo sdílené složce a že je dost volného místa. Funguje to bez spojení SSH i pro datové sady, které už neexistují.
- **Do nové datové sady.** Zadej datovou sadu, která ještě neexistuje. BombVault ji vytvoří s vlastnostmi ZFS uloženými v záloze a obnoví do ní, viz [Obnova jako nová datová sada](#new-dataset).
- **Vybrat soubory** (pokročilé): zapsat zpět do datové sady jen soubory a složky, které vyberete.
- **Všechny datové sady této zálohy** (pokročilé): každou datovou sadu stromu do vlastní podsložky vybrané složky. Datové sady, které byly v této záloze přeskočeny, se uvedou.
- **Z jiného serveru:** stránka **Obnova** obnovuje z repozitáře jiného BombVaultu, vždy do složky: všechny datové sady jedné zálohy, každou do vlastní podsložky, nebo jednu datovou sadu stromu, celou nebo vybrané soubory.

Seznam kontejnerů k zastavení z položky se nabízí i pro obnovu do datové sady. Tyto kontejnery zůstanou zastavené po celou dobu obnovy a zálohy kontejnerů mezitím čekají.

### Bezpečnostní snímek {#safety-snapshot}

Než BombVault zapíše do datové sady, pořídí snímek ZFS jen této datové sady s názvem `bombvault-prerestore-<čas>`. Ve výchozím stavu je zapnutý; jeho vypnutí vyžaduje druhé potvrzení. Nelze-li snímek pořídit, nic se neobnoví.

BombVault bezpečnostní snímek nikdy sám nesmaže. Položka je vypíše s jejich stářím a velikostí, každý s akcí **Smazat**, a varuje, když je nejstarší starší než 30 dní, protože drží v poolu smazaná a změněná data.

Chcete-li se po obnově vrátit, zkopírujte jednotlivé soubory z `.zfs/snapshot/bombvault-prerestore-<čas>` uvnitř datové sady. `zfs rollback <dataset>@bombvault-prerestore-<čas>` funguje jen, dokud je to nejnovější snímek této datové sady. `zfs rollback -r` smaže každý novější snímek včetně automatických.

### Obnova jako nová datová sada {#new-dataset}

BombVault s každou zálohou uloží lokálně nastavené vlastnosti ZFS každé datové sady: compression, recordsize, quota, reservation, atime, xattr, acltype, casesensitivity a tvoje vlastní uživatelské vlastnosti. Zděděné hodnoty a hodnoty jen pro čtení se vynechají, protože se vrátí samy. Zálohy z doby, kdy je BombVault ještě neukládal, žádné nemají.

- **Do nové datové sady** spustí `zfs create` se všemi uloženými vlastnostmi. casesensitivity, normalization a utf8only jde nastavit jen takto. Kvóty a rezervace se nastaví až po souborech, aby je nemohly odmítnout. Přípojný bod se vynechá, aby kopie nekolidovala s originálem, stejně jako `canmount`, `readonly` a šifrování, aby obnova mohla zapisovat. Nová datová sada pod šifrovanou převezme její šifrování. Nadřazená datová sada musí existovat. Když po vytvoření něco selže, nová datová sada zůstane na serveru, protože BombVault nikdy datovou sadu neničí.
- **Obnovit do datové sady** ukáže uložené vlastnosti vedle obnovy. **Nastavit i tyto vlastnosti** nastaví ty, které existující datová sada ještě přijme, dřív než se zapíše jakýkoli soubor. Kvóty a rezervace se nastaví až po souborech, aby je nemohly odmítnout. Bez tohoto přepínače si datová sada ponechá své nastavení.

## Co záloha obsahuje {#contents}

V záloze: soubory a složky každé zálohované datové sady, s vlastníkem, oprávněními, časovými razítky a rozšířenými atributy tak, jak je restic ukládá a lokálně nastavené vlastnosti ZFS každé datové sady.

Není v záloze:

- vlastník a oprávnění samotné nejvyšší složky každé datové sady (vše pod ní je zahrnuto). Obnova do datové sady nechá existující nejvyšší složku, jak je, obnova do složky ji vytvoří s oprávněními `0755`;
- existující snímky ZFS;
- podřízené sady, které byly přeskočeny nebo vynechány;
- svazky.

Pro obnovu na nový pool vytvoř pool a obnov každou datovou sadu do nové datové sady. Zda se ACL NFSv4, jak je TrueNAS používá na datových sadách SMB, vrátí tak, jak čekáte, zatím nebylo ověřeno, proto si obnovu vyzkoušejte na vlastních datech, než se na to spolehnete.

## Šifrované datové sady {#encryption}

Šifrovaná datová sada se zálohuje, jen dokud je její klíč načtený. Jinak se přeskočí s varováním; načtěte klíč pomocí `zfs load-key` a datovou sadu připojte. BombVault čte data dešifrovaná a ukládá je do repozitáře resticu, který je šifrovaný. Pokud jste šifrování v BombVaultu vypnuli, tento repozitář šifrovaný není.

## Replika {#replica}

Replika je kopie datových sad položky na druhém serveru ZFS. BombVault ji udržuje aktuální pomocí `zfs send` a `zfs receive`. První běh pošle všechno, potom putují jen změněné bloky. Na druhém serveru můžete kopii hned připojit.

Replika nikdy nenahrazuje zálohu. Starší verze, jednotlivé soubory i kontrola se dál berou ze záloh a replika drží jen tolik snímků, kolik nastavíte. Aktuální replika se počítá jako kopie mimo lokalitu, ale položka s replikou a bez zálohy zůstává oranžová.

Zapnete ji na kartě **Replica** v nastavení položky. Tam vyberete, kam replika půjde, kdy se spouští (**After every backup** nebo **Own plan**) a kolik snímků zůstane na cíli. Karta vypisuje každou datovou sadu a každý svazek s jeho stavem a **Replikovat nyní** spustí běh. Běh repliky má vlastní zámek, takže dlouhý první přenos nikdy nezdrží zálohy.

### Odeslání na server ZFS {#replica-push}

Přijímat může každý stroj se ZFS a SSH, například druhý Unraid nebo TrueNAS. BombVault tam běžet nemusí.

1. V **Nastavení, Storage locations** otevřete **Add storage location** a vyberte **ZFS server**.
2. Zadejte adresu, uživatele a port. Dialog ukáže veřejný klíč BombVaultu. Přidejte ho do `~/.ssh/authorized_keys` uživatele na serveru. V Unraidu je to v **Settings, Users, root, SSH keys**.
3. Otestujte spojení. Dialog pak vypíše pooly serveru. Vyberte jeden a nastavte kořen, který je ve výchozím stavu `<pool>/bombvault-replica`.
4. Vyberte nový server na kartě **Replica** položky.

S rootem není potřeba nic dalšího. Vlastní uživatel potřebuje tato oprávnění na poolu cíle, která dialog také ukazuje:

```
zfs allow <user> receive,create,mount,rollback,destroy,userprop <pool>
```

Na zdroji potřebuje stejný druh uživatele tato oprávnění na nejvyšší datové sadě položky:

```
zfs allow <user> send,snapshot,hold,release,bookmark,destroy <dataset>
```

V tomto směru drží BombVault, který položku vlastní, také klíč, který může na server zapisovat.

### Odeslání spárované instanci {#replica-receive}

Spárovaný BombVault umí repliku přijmout sám. Nikdo nedostane přístup SSH k druhému hostiteli a do souboru `authorized_keys` se nepřidává žádný klíč.

1. Na kartě **Replica** položky vyberte jako cíl spárovanou instanci. Karta ukazuje **Čeká na schválení**, dokud neodpoví.
2. Na přijímající instanci otevřete **Instance**, pak **Příjem**. Karta **ZFS** vypíše žádost. Vyberte pool a kořen na tom serveru a kolik snímků zůstane, potom stiskněte **Allow** nebo **Decline**.
3. Po **Allow** zdroj odesílá podle vlastního plánu, jako u každého jiného cíle.

Přijímající instance přijme jen to, co schválení pokrývá: datové sady položky, do vlastního kořene. Příkaz `zfs receive` spouští sama a zdroj tam nemůže nic smazat ani vrátit zpět. Kopie tedy přežije i zdroj, který někdo převzal. Přijímající instance si drží vlastní uchovávání. Zdroj při žádosti jen navrhne pravidlo.

**Revoke access** na přijímající instanci kdykoli ukončí schválení a oznámí to zdroji, který pak zobrazí, že schválení bylo odvoláno, a přestane odesílat. Co přijímající instance už má, tam zůstane. Zamítnutá žádost zůstává zamítnutá. Další datové sady nebo žádost po odvolání se ptají znovu.

### Kam data přijdou {#replica-target}

Každá datová sada skončí v `<root>/<server>/<pool>/<path>`. Složka serveru je název zdrojové instance, ustálený při prvním přenosu, takže si dva servery se stejným názvem poolu nikdy nepřekážejí. Například `cache/appdata` serveru s názvem `tower` skončí v `backup/bombvault-replica/tower/cache/appdata`.

Kopie na cíli je jen pro čtení a není připojená, takže nikdy nic na tom serveru nepřekryje. Vlastnosti ZFS putují s ní, kromě přípojného bodu, `sharenfs` a `sharesmb`.

### Co se zahrnuje {#replica-contents}

Zahrnuje se všechno, co položka zálohuje, a také svazky pod ní, které záloha přeskakuje. Podřízená datová sada, kterou jste v položce vypnuli, zůstane venku. Všechny datové sady jednoho běhu pocházejí z jednoho snímku, stejně jako u zálohy.

### Jak dlouho snímky zůstávají {#replica-retention}

Na cíli drží nová replika 7 denních a 3 týdenní snímky. Místo toho vyberte **Short**, **Balanced** nebo **Long**, nebo nastavte **Custom values**. Odstraňují se tam jen snímky pojmenované `bombvault-replica-<14 digits>`, a nikdy ne nejnovější, který mají obě strany společný.

Na zdroji drží BombVault jen poslední snímek repliky a k tomu záložku pro každý odeslaný stav. Záložky nezabírají místo. Další přenos od nich vychází.

### Šifrované datové sady v replice {#replica-encryption}

Šifrovaná datová sada se posílá surově. Na cíli zůstává šifrovaná a cíl klíč nikdy nevidí. Klíč dobře uschovejte: potřebujete ho k otevření kopie po obnově a replika bez něj je nečitelná.

### Použití repliky {#replica-use}

Otevřete záložku **Zálohy** položky a klikněte na řádek repliky na kartě **Storage locations**. List vypíše snímky na cíli a ukáže příkazy s vašimi skutečnými názvy.

Chcete-li se podívat na starý stav, naklonujte snímek na cíli. Klon nezabírá místo, dokud se něco nezmění, a replika zůstane nedotčená:

```
zfs clone backup/bombvault-replica/tower/cache/appdata@bombvault-replica-20261006014100 backup/bombvault-replica/clone-appdata
```

Pokud zdroj selže, změňte kopii na cíli na normální zapisovatelnou datovou sadu:

```
zfs inherit -r readonly backup/bombvault-replica/tower/cache/appdata && zfs inherit -r canmount backup/bombvault-replica/tower/cache/appdata && zfs mount -a
```

BombVault pak do této datové sady přestane replikovat, dokud nespustíte nový první běh.

Chcete-li stav vrátit na zdroj, stiskněte v listu **Bring back as a new dataset**. BombVault pošle snímek do nové datové sady vedle originálu, pojmenované `<dataset>-bombvault-restore-` plus časové razítko. Originál nikdy nepřepisuje.

## Zbylé snímky {#leftover-snapshots}

Snímek zálohy se jmenuje `<dataset>@bombvault-<14 číslic>`, například `cache/appdata@bombvault-20260924021500` (UTC). BombVault ho odstraní hned po záloze. Pokud se to nepovede, například protože je datová sada zaneprázdněná nebo byl BombVault zastaven, odstraní ho BombVault:

- před další zálohou této položky,
- při spuštění BombVaultu, pro každou položku, i s vypnutou doménou,
- když položku smažete,
- když u položky stisknete **Odstranit teď**, kde je také vidět, kolik jich zbývá.

Odstraňují se jen názvy, které přesně odpovídají `bombvault-` plus 14 číslic. Bezpečnostních snímků, vašich vlastních snímků a automatických snímků se to nikdy nedotkne. Chcete-li jeden odstranit ručně:

```
zfs destroy -r cache/appdata@bombvault-20260924021500
```

Snímek repliky se jmenuje `<dataset>@bombvault-replica-<14 digits>` a není zbytkem. Zůstává na zdroji, dokud ho další běh repliky nenahradí, a na cíli, dokud ho uchovávání drží. Úklid se ho nikdy nedotkne, protože odpovídá jen `bombvault-` následovanému přesně 14 číslicemi.

## Anomálie {#anomalies}

Podřízená sada, která byla vyprázdněna, sotva změní součet velkého stromu, a proto detekce anomálií sleduje každou datovou sadu položky zvlášť: velikost, počet souborů, nová data a doba resticu mají každá vlastní historii. Tato historie patří ke jménu datové sady, takže zůstane, i když strom později zálohuje jiná položka.

Datová sada, kterou předchozí běh zálohoval a kterou tento běh nemohl přečíst, se počítá jako vyprázdněná, pokud se výběr položky nezměnil. To pokrývá nenačtený klíč, nepřipojenou datovou sadu i sadu, která ze stromu zmizela. Podřízená sada, kterou sami vyloučíte, mění výběr, takže její historie začne znovu. Dokud je otevřené zjištění o ztracených datech, uchovávání ponechá staré zálohy právě této datové sady a zbytek stromu pročistí jako obvykle.

Na stránce **Anomálie** má každá datová sada vlastní řádek v panelu položky, který se otevře přes **Sledování** na kartě položky, nebo z jejího řádku na kartě **Nic otevřeného**, když u položky nic otevřeného není. Strom položky na této stránce ukazuje otevřená zjištění u každé datové sady. Odkaz ve zjištění otevře panel obnovy položky u poslední dobré zálohy datové sady. Zda běh doběhne, se posuzuje pro celou položku, protože běh uspěje nebo selže jako celek.

Samotné kontroly popisuje [Funkce](features.md). Asistent připojený přes [server MCP](mcp.md) může vypsat body obnovy položky ZFS, spustit její zálohu a číst zjištění, ale potvrzení zjištění probíhá na stránce **Anomálie**.

## Kódy důvodů {#reason-codes}

Stránka, historie běhů a oznámení pojmenují problém jedním z těchto kódů. U většiny je řešení uvedeno i vedle na stránce.

| Kód | Význam | Co dělat |
|---|---|---|
| `ssh-missing` | Spojení SSH není v tomto kontejneru nastavené. | Nastavte spojení SSH jako pro zálohy VM. |
| `host-placeholder` | Host SSH: Address je stále vzorová hodnota a `host.docker.internal` také neodpověděl. | Nastavte Host SSH: Address na IP adresu LAN tohoto serveru. |
| `host-fallback` | Host SSH: Address je stále vzorová hodnota a `host.docker.internal` funguje. | Nic, nebo nastavte IP adresu LAN. |
| `ssh-unreachable` | Server není přes SSH dosažitelný. | Zkontrolujte adresu a port a zda je SSH zapnuté. |
| `ssh-auth` | Server odmítl klíč BombVaultu. | Spusťte jednou na serveru příkaz zobrazený na kartě spojení. |
| `zfs-not-found` | Hostitel SSH nemá příkaz `zfs`. | Nasměrujte Host SSH: Address na stroj, kterému pooly patří. |
| `zfs-permission` | Uživatel SSH nesmí spustit tento příkaz zfs. | Použijte root, nebo viz [TrueNAS SCALE](#truenas). |
| `uri-mismatch` | `LIBVIRT_URI` uvádí jiného hostitele nebo uživatele než pole SSH. | Sjednoťte je, nebo pole SSH vyprázdněte, aby obojí pocházelo z URI. |
| `zfs-error` | zfs nahlásil jinou chybu. | Podrobnosti ukazují jeho zprávu. |
| `propagation-missing` | Nová připojení na hostiteli nedorazí do kontejneru. | Nastavte Access Mode u Host Data na Read/Write - Slave a restartujte BombVault. |
| `invalid-name` | Název datové sady, který BombVault nepřijímá. | Přejmenujte datovou sadu. |
| `name-too-long` | Datová sada ve stromu je na název snímku příliš dlouhá. | Přejmenujte ji, nebo přidejte jako položku datovou sadu pod ní. |
| `invalid-exclude` | Vzor vyloučení nebo vynechaná podřízená sada k položce nepasuje. | Opravte záznam, který zpráva uvádí. Chcete-li vynechat celou podřízenou sadu, vypněte ji místo psaní vzoru. |
| `not-found` | Datová sada na serveru neexistuje. | Odeberte položku, nebo datovou sadu vytvořte znovu. Její zálohy zůstávají obnovitelné. |
| `not-filesystem` | Toto je svazek, ne souborový systém. | Viz [Svazky](#volumes). |
| `overlaps-item` | Datová sada se překrývá s existující položkou. | Viz [Položky se nikdy nepřekrývají](#overlap). |
| `docker-storage` | Strom obsahuje úložiště obrazů Dockeru. | Viz [Úložiště Dockeru](#docker-storage). |
| `nothing-readable` | Žádnou datovou sadu položky teď nelze přečíst. | Podívejte se na kódy přeskočených datových sad. |
| `snapshot-failed` | Snímek nešlo vytvořit. | Podrobnosti ukazují zprávu zfs. |
| `containers-busy` | Když se měly kontejnery zastavit, ještě běžela záloha kontejneru. | Spusťte znovu později. Plánované běhy počkají samy. |
| `consistency-stop-failed` | Kontejner nešlo zastavit, takže se snímek nepořídil. | Zkontrolujte kontejner, nebo ho odeberte ze seznamu. |
| `pre-snapshot-failed` | Příkaz před snímkem selhal. | Podrobnosti běhu ukazují jeho výstup. |
| `container-unknown` | Uvedený kontejner neexistuje. | Odeberte ho ze seznamu. |
| `container-is-self` | BombVault nemůže zastavit svůj vlastní kontejner. | Odeberte ho ze seznamu. |
| `leftover-snapshots` | Na serveru jsou stále snímky, které BombVault nemohl odstranit. | Stiskněte **Odstranit teď**, viz [Zbylé snímky](#leftover-snapshots). |
| `zvol` | Svazek ve stromu, přeskočen. | Viz [Svazky](#volumes). |
| `canmount-off` | Nikdy nepřipojeno (`canmount=off`), přeskočeno. | Pokud obsahuje data, připojte ho nebo data přesuňte do podřízené sady. |
| `legacy-mount` | Přípojný bod legacy, přeskočeno. | Dejte mu přípojný bod pod `/mnt`. |
| `no-mountpoint` | Bez přípojného bodu, přeskočeno. | Dejte mu přípojný bod pod `/mnt`. |
| `not-mounted` | Na serveru nepřipojeno, přeskočeno. | Připojte ho pomocí `zfs mount`, nebo nastavte `canmount=on`. |
| `key-not-loaded` | Šifrováno a klíč není načtený, přeskočeno. | `zfs load-key`, pak ho připojte. |
| `snapdir-disabled` | Přístup ke snímkům je vypnutý, přeskočeno. | `zfs set snapdir=hidden <dataset>`. Složka `.zfs` zůstane skrytá. |
| `not-visible` | BombVault nevidí přípojný bod datové sady. | Přesuňte přípojný bod pod cestu Host Data, nebo ho namapujte do kontejneru na stejnou cestu s Read/Write - Slave. |
| `shfs-only` | Datová sada je vidět jen přes `/mnt/user`, které skrývá snímky. | Jako Host Data namapujte `/mnt`, ne `/mnt/user`. |
| `snapshot-not-visible` | Snímek byl vytvořen, ale v BombVaultu se neobjevil. | Spusťte **Vyzkoušet přístup ke snímkům**; viz níže. |
| `snapshot-loop` | Snímek nedorazil do BombVaultu, protože Host Data nepropouští nová připojení. | Nastavte Access Mode u Host Data na Read/Write - Slave a restartujte BombVault. |
| `backup-failed` | restic pro tuto datovou sadu selhal. | Podrobnosti běhu ukazují proč. |
| `not-reached` | Běh skončil před touto datovou sadou. | Spusťte zálohu znovu. |
| `gone` | Datová sada už na serveru není. | Nic. Její zálohy zůstávají obnovitelné. |
| `read-only-mount` | BombVault může datovou sadu jen číst, takže do ní nemůže obnovovat. | Nastavte mapování na Read/Write - Slave, nebo obnovte do složky. |
| `destination-not-mounted` | Složka neleží na připojeném poolu ani sdílené složce. | Vyberte složku na poolu nebo sdílené složce. |
| `not-enough-space` | V cíli není dost volného místa. | Uvolněte místo, nebo vyberte jinou složku. |
| `safety-snapshot-failed` | Bezpečnostní snímek nešel pořídit, takže se nic neobnovilo. | Podrobnosti ukazují zprávu zfs. |
| `safety-name-too-long` | Název datové sady je na bezpečnostní snímek příliš dlouhý. | Vypněte bezpečnostní snímek, nebo obnovte do složky. |
| `dataset-exists` | Datová sada s tímto názvem už existuje. | Zvol nový název, nebo obnov přímo do datové sady. |
| `create-failed` | Novou datovou sadu se nepodařilo vytvořit. | Podrobnosti ukazují zprávu zfs. Zkontroluj, že nadřazená datová sada existuje. |
| `new-dataset-not-visible` | Nová datová sada byla vytvořena, ale BombVault ji nevidí, takže se nic neobnovilo. | Datová sada zůstává na serveru. Připoj ji pod cestu Host Data a obnov do ní. |
| `set-properties-failed` | Uložené vlastnosti se nepodařilo nastavit, takže se nic neobnovilo. | Podrobnosti ukazují zprávu zfs. |
| `set-limits-failed` | Soubory se obnovily, ale uloženou kvótu nebo rezervaci se nepodařilo nastavit. | Podrobnosti ukazují zprávu zfs. Nastav kvótu nebo rezervaci sám pomocí `zfs set`. |

### Kontrola toho, co kontejner vidí {#mountinfo}

**Vyzkoušet přístup ke snímkům** u položky pořídí skutečný snímek jejího stromu, hledá ho v BombVaultu pro každou datovou sadu a zase ho odstraní. Je to nejrychlejší způsob, jak ověřit celou cestu před prvním plánovaným během.

Chcete-li se podívat sami, spusťte na serveru:

```
docker exec BombVault grep zfs /proc/self/mountinfo
```

Každý řádek je jedno připojení uvnitř kontejneru. Řádek datové sady ukazuje její cestu v kontejneru (pod `/host/user`) a název datové sady. Pole `master:N` na tomto řádku znamená, že připojení dostává připojení, která hostitel vytvoří později, a to přístup ke snímkům potřebuje. Pokud chybí, nastavte Access Mode u Host Data na Read/Write - Slave a restartujte BombVault.

## TrueNAS SCALE {#truenas}

- Když je nastavena `LIBVIRT_URI` (jako pro zálohy VM na TrueNAS), BombVault vezme hostitele, uživatele a port SSH pro své příkazy zfs z URI, každý, který není nastaven samostatně. Bez záloh VM nastavte místo toho `LIBVIRT_HOST`, `LIBVIRT_SSH_USER` a `LIBVIRT_SSH_PORT`. Proměnné přidejte v **Additional Environment Variables**.
- Uživatel jiný než root potřebuje oprávnění k nejvyšší datové sadě položky, která pak pokrývají každou datovou sadu pod ní:

  ```
  zfs allow <user> snapshot,destroy,mount <dataset>
  ```

  Obnova do nové datové sady navíc potřebuje `create` na nadřazené datové sadě a nastavení uložených vlastností potřebuje oprávnění k těmto vlastnostem.

  Relace SSH bez roota na TrueNAS nemá v cestě `/usr/sbin`; BombVault pak volá přímo `/usr/sbin/zfs`.
- **Host Data** aplikace musí být cesta hostitele nad datovými sadami, například `/mnt/tank`, ne ixVolume. S cestou hostitele předává aplikace nová připojení hostitele do BombVaultu (`rslave`), a to přístup ke snímkům potřebuje.
