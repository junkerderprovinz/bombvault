# Datové sady ZFS

Stránka **ZFS** zálohuje datové sady ZFS. Položka je jedna datová sada spolu se všemi datovými sadami pod ní. Pro každou zálohu pořídí BombVault jediný snímek ZFS celého stromu, takže každá datová sada v něm je zachycena ve stejném okamžiku. Poté z tohoto snímku přečte soubory každé datové sady, uloží je pomocí resticu stejně jako složku a snímek hned potom odstraní. Zálohy jsou deduplikované, každou z nich můžete procházet a lze obnovit jednotlivé soubory.

BombVault pro datové sady nikdy nepoužívá `zfs send`, nikdy datovou sadu nevrací do dřívějšího stavu a nikdy žádnou nezničí.

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
- **Vybrat soubory** (pokročilé): zapsat zpět do datové sady jen soubory a složky, které vyberete.
- **Všechny datové sady této zálohy** (pokročilé): každou datovou sadu stromu do vlastní podsložky vybrané složky. Datové sady, které byly v této záloze přeskočeny, se uvedou.
- **Z jiného serveru:** stránka **Obnova** obnovuje z repozitáře jiného BombVaultu, vždy do složky: všechny datové sady jedné zálohy, každou do vlastní podsložky, nebo jednu datovou sadu stromu, celou nebo vybrané soubory.

Seznam kontejnerů k zastavení z položky se nabízí i pro obnovu do datové sady. Tyto kontejnery zůstanou zastavené po celou dobu obnovy a zálohy kontejnerů mezitím čekají.

### Bezpečnostní snímek {#safety-snapshot}

Než BombVault zapíše do datové sady, pořídí snímek ZFS jen této datové sady s názvem `bombvault-prerestore-<čas>`. Ve výchozím stavu je zapnutý; jeho vypnutí vyžaduje druhé potvrzení. Nelze-li snímek pořídit, nic se neobnoví.

BombVault bezpečnostní snímek nikdy sám nesmaže. Položka je vypíše s jejich stářím a velikostí, každý s akcí **Smazat**, a varuje, když je nejstarší starší než 30 dní, protože drží v poolu smazaná a změněná data.

Chcete-li se po obnově vrátit, zkopírujte jednotlivé soubory z `.zfs/snapshot/bombvault-prerestore-<čas>` uvnitř datové sady. `zfs rollback <dataset>@bombvault-prerestore-<čas>` funguje jen, dokud je to nejnovější snímek této datové sady. `zfs rollback -r` smaže každý novější snímek včetně automatických.

### Obnova jako nová datová sada {#new-dataset}

BombVault datové sady nevytváří. Vytvořte ji na serveru s požadovanými vlastnostmi a pak obnovte do složky, která je jejím přípojným bodem:

```
zfs create -o compression=lz4 cache/appdata-restored
```

a v BombVaultu obnovte do složky `cache/appdata-restored` pod `/mnt`.

## Co záloha obsahuje {#contents}

V záloze: soubory a složky každé zálohované datové sady, s vlastníkem, oprávněními, časovými razítky a rozšířenými atributy tak, jak je restic ukládá.

Není v záloze:

- vlastnosti ZFS datových sad (compression, recordsize, quota, mountpoint a ostatní);
- vlastník a oprávnění samotné nejvyšší složky každé datové sady (vše pod ní je zahrnuto). Obnova do datové sady nechá existující nejvyšší složku, jak je, obnova do složky ji vytvoří s oprávněními `0755`;
- existující snímky ZFS;
- podřízené sady, které byly přeskočeny nebo vynechány;
- svazky.

Pro obnovu na nový pool nejprve vytvořte datové sady s požadovanými vlastnostmi. Zda se ACL NFSv4, jak je TrueNAS používá na datových sadách SMB, vrátí tak, jak čekáte, zatím nebylo ověřeno, proto si obnovu vyzkoušejte na vlastních datech, než se na to spolehnete.

## Šifrované datové sady {#encryption}

Šifrovaná datová sada se zálohuje, jen dokud je její klíč načtený. Jinak se přeskočí s varováním; načtěte klíč pomocí `zfs load-key` a datovou sadu připojte. BombVault čte data dešifrovaná a ukládá je do repozitáře resticu, který je šifrovaný. Pokud jste šifrování v BombVaultu vypnuli, tento repozitář šifrovaný není.

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

## Anomálie {#anomalies}

Podřízená sada, která byla vyprázdněna, sotva změní součet velkého stromu, a proto detekce anomálií sleduje každou datovou sadu položky zvlášť: velikost, počet souborů, nová data a doba resticu mají každá vlastní historii. Tato historie patří ke jménu datové sady, takže zůstane, i když strom později zálohuje jiná položka.

Datová sada, kterou předchozí běh zálohoval a kterou tento běh nemohl přečíst, se počítá jako vyprázdněná, pokud se výběr položky nezměnil. To pokrývá nenačtený klíč, nepřipojenou datovou sadu i sadu, která ze stromu zmizela. Podřízená sada, kterou sami vyloučíte, mění výběr, takže její historie začne znovu. Dokud je otevřené zjištění o ztracených datech, uchovávání ponechá staré zálohy právě této datové sady a zbytek stromu pročistí jako obvykle.

Na kartě **Položky** stránky **Anomálie** má každá datová sada vlastní řádek pod svou položkou a strom položky na této stránce ukazuje otevřená zjištění u každé datové sady. Odkaz ve zjištění otevře panel obnovy položky u poslední dobré zálohy datové sady. Zda běh doběhne, se posuzuje pro celou položku, protože běh uspěje nebo selže jako celek.

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

  Relace SSH bez roota na TrueNAS nemá v cestě `/usr/sbin`; BombVault pak volá přímo `/usr/sbin/zfs`.
- **Host Data** aplikace musí být cesta hostitele nad datovými sadami, například `/mnt/tank`, ne ixVolume. S cestou hostitele předává aplikace nová připojení hostitele do BombVaultu (`rslave`), a to přístup ke snímkům potřebuje.
