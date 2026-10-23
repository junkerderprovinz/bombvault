# Mimo lokalitu a obnova

!!! note "Kopie mimo lokalitu čekají po opětovném sestavení"
    Když Nouzový průvodce nebo **Objevit zálohy** znovu sestaví položky bez starého nastavení, kopie mimo lokalitu těchto sekcí se pozastaví, dokud není potvrzeno jejich výchozí nastavení. Viz [Po opětovném sestavení](#after-a-rebuild).

Místní zálohy tě chrání před ztraceným kontejnerem nebo špatnou aktualizací. Kopie mimo lokalitu a vyzkoušená sada pro obnovu tě chrání před ztrátou celého stroje, ransomwarem nebo požárem. Tato stránka se věnuje cílům a kopiím mimo lokalitu, tomu, jak kopii ochránit před manipulací, jak dokázat, že umíš obnovit, párování několika serverů a návratu všeho, když zmizí sám BombVault.

## Kopie mimo lokalitu {#off-site-copies}

Nech si rychlou místní zálohu a přidej jeden či více cílů mimo lokalitu. BombVault tam nové zálohy kopíruje pomocí `restic copy` na základě nejlepší snahy, takže zádrhel mimo lokalitu nikdy nezhatí místní zálohu. V tomto tvaru zůstává místní repozitář primární a repozitář mimo lokalitu je kopie, ale primární repozitář sekce vůbec nemusí být místní; zálohování rovnou na S3, rest-server a podobné místo kopírování na ně viz [Vzdálené primární repozitáře](#remote-primary-repositories) níže.

- **Několik cílů na sekci.** Každá sekce (kontejnery, VM, flash, autozáloha, složky a datové sady ZFS) může kopírovat do několika cílů mimo lokalitu najednou, takže můžeš mít například rest-server na krabici kamaráda a bucket S3 souběžně. Karta **Cíle podle sekcí** na stránce Cíle zapíná nebo vypíná každou linku ze sekce do cíle.
- **Kdy se kopíruje.** Karta **Kopírovat** cíle určuje **Kdy kopírovat**: **Po každé záloze**, nebo podle vlastního plánu, který zálohy sbírá a kopíruje v pevných časech. V **Nastavení, Plán, Kopie mimo lokalitu podle sekcí** dává **Výjimka** jedné sekci vlastní plán kopírování. **Replikovat nyní** v kartě **Kopie mimo lokalitu** sekce v **Nastavení, Cíle** spustí kopírování na požádání.
- **Uchovávání** se nastavuje pro každý cíl pod **Jak dlouho uchovávat**, takže kopie mimo lokalitu můžeš držet déle jako archiv. Kopie v poli sekce mimo lokalitu se řídí kartou **Uchovávání mimo lokalitu** v **Nastavení, Cíle**; nech ji celou na nule, aby se zálohy mimo lokalitu nikdy automaticky neořezávaly.
- **Limity šířky pásma** (**Šířka pásma mimo lokalitu** v **Nastavení, Cíle**, nebo karta **Kopírovat** cíle) omezují rychlost nahrávání/stahování resticu, aby kopie nezahltily tvou WAN.
- Zatímco kopírování běží, hlavička stavu v Přehledu říká **Probíhá kopie mimo lokalitu**. Je to ukazatel činnosti, ne procentní pruh, protože `restic copy` nezveřejňuje strojově čitelný průběh.

!!! note "Obnova z libovolného místa"
    Každá položka vypisuje své zálohy jako jednu časovou osu přes všechna místa, kde záloha leží, na záložce **Zálohy**. Záloha zkopírovaná na B2 se ukáže jednou, označená každým místem, které ji drží. Obnova bere první místo, kam se dostane, počínaje repozitářem, do kterého se položka zapisuje, a u každé zálohy můžeš zvolit jiné místo. Cíle mimo lokalitu se čtou až po kliknutí na **Zkontrolovat**, protože výpis cloudového úložiště chvíli trvá. Smazání na jednom místě nejdřív zkontroluje ostatní a řekne, zda šlo o poslední kopii.

## Cíle {#destinations}

Cíl je místo, kam zálohy jdou: místní složka, sdílená složka, tvůj vlastní server nebo cloudová služba. Nastaví se jednou a pak slouží každé sekci. **Cíle** otevřeš z jejich dlaždice v nastavení, nebo tam zapni **Zobrazit také v postranním panelu**, aby se připnuly.

**Přidat cíl** otevře okno o čtyřech krocích:

1. **Poskytovatel.** Každá služba je vypsána se svým logem, s vyhledáváním vpravo nahoře v okně, ve skupinách: úložné služby s buckety S3 (Backblaze B2, Wasabi, Cloudflare R2, Hetzner Object Storage, Amazon S3 a další), tvůj vlastní server S3 (Garage, SeaweedFS, RustFS, Silo, Ceph, JuiceFS, Versity S3 Gateway), tvůj vlastní server a sdílené složky (rest-server, Hetzner Storage Box, SFTP, SMB, WebDAV, připojená cesta), cloudové úložiště (OneDrive, Google Drive, Dropbox, pCloud, Nextcloud a zbytek, co rclone podporuje) a přijímací servery tvé skupiny. Každá říká, jak se hodí pro zálohy: cloudové disky zpomalují při mnoha požadavcích, takže první záloha a prune tam trvají déle.
2. **Přihlášení.** Pole závisejí na službě: přístupový klíč pro S3, uživatel a heslo pro WebDAV a SMB, heslo aplikace tam, kde dvoufázové přihlášení blokuje běžné, veřejný klíč SSH BombVaultu pro SFTP a Storage Box, nebo token pro služby, které se přihlašují přes prohlížeč. Pro ně okno ukáže příkaz `rclone authorize`, který spustíš na počítači s prohlížečem; token, který vypíše, patří do pole. **Otestovat** zkontroluje přihlášení dřív, než se cokoli uloží. Okno pak vypíše složky na cíli, s **Nová složka** pro vytvoření jedné a volným místem tam, kde ho služba hlásí. Prázdná složka je nejbezpečnější; BombVault do ní dá jednu složku na sekci.
3. **Ochrana.** Okno říká jasně, co služba umí. Rest-server v režimu append-only mazání odmítá a tamper test to ověřuje. Bucket S3 může držet staré verze pomocí verzování a zámku objektů, což BombVault zatím ověřit neumí. Cloudový disk mazání odmítnout vůbec neumí: kdo se dostane na server, dostane se i na tu kopii. **Ochrana proti smazání (append-only)** zapni jen tam, kde druhá strana mazání opravdu odmítá; BombVault tam pak nikdy neořezává. **Pro případ nouze** říká, co sada pro obnovu pro tento cíl obsahuje.
4. **Pro které sekce.** Vyber sekce, které se tam zálohují. První kopie přenese celou historii. Položky, které si zvolily vlastní cíle, nový nedostanou samy a okno je jmenuje.

Některé služby chtějí při přihlášení trochu víc:

- **SFTP a Storage Box.** **Přidat klíč** uloží veřejný klíč BombVaultu na server s heslem z pole **Heslo účtu**, které poslouží jen pro to jedno přihlášení a nikde se neuloží. Pod ním je příkaz, jak to udělat ručně. Test ukáže **Klíč serveru** s jeho otiskem; BombVault si ho zapamatuje a server odmítne, pokud později ukáže jiný.
- **Klíče S3 pro jediný bucket**, třeba objektový token Cloudflare R2 nebo omezený klíč AWS, nemusí smět vypsat ostatní buckety. Zadej bucket do pole **Bucket** a test i seznam složek otevřou právě tento bucket.
- **Servery SMB, které své sdílené složky nevypisují,** dostanou do pole **Sdílená složka** název jedné z nich a test ji otevře přímo.
- **FTP** se ptá na **Šifrování**: **Žádné**, **Explicitní TLS (FTPES)** nebo **Implicitní TLS (FTPS, port 990)**.
- **OneDrive a Zoho WorkDrive.** Když přihlášení otevře více disků nebo pracovních prostorů, okno se zeptá, na který mají jít zálohy.
- **rest-server se samopodepsaným certifikátem.** Ulož certifikát jako soubor PEM do složky `certs` v datové složce BombVaultu (`/config/certs` v kontejneru, `appdata/bombvault/certs` na Unraidu). Zálohy, kopie i test ochrany proti smazání mu důvěřují bez restartu.

Služby S3 běží přes vlastní backend S3 resticu, díky čemuž lze použít třídu úložiště a zámek objektů. Každá další služba běží přes rclone, který BombVault dodává, a její remote se pak objeví v kartě **Off-site (rclone)** v **Nastavení, Cíle**. Export nastavení nese cíle; s přihlašovacími údaji nese i jejich přihlášení.

Složka na tomto serveru se všude jmenuje **Místní**: na stránkách cílů, v toku, u položky i v Obnově. Je-li na tomto serveru složek víc, nese každá svůj vlastní název. Cíl, který založí pole mimo lokalitu u sekce, se jmenuje podle svého místa, tedy podle poslední složky své adresy, nebo když žádnou nemá, podle svého serveru, a dostane číslo, pokud už takto jmenovaný cíl existuje. Název, který mu dáš na jeho stránce, zůstane. Stojí-li vedle sebe dva cíle se stejným názvem, nese každý navíc svůj server nebo složku.

Každý cíl má vlastní stránku:

- **Připojení** s jeho názvem, přepínačem **Dostupné** (vypnuto znamená, že BombVault sem už nezapisuje a položky, které se sem zálohují, selžou s jasnou zprávou, místo aby skončily jinde), složkou, kompresí, třídou úložiště a **Mimo objekt**. **Změnit připojení** otestuje a uloží novou adresu nebo přihlášení.
- **Přihlašovací údaje**: sdílená sada nebo vlastní sada.
- **Ochrana proti smazání**, s **Otestovat ochranu proti smazání hned** a údajem, kdo tuto kopii ořezává.
- **Kopírovat**: kdy kopírovat a limity nahrávání a stahování.
- **Jak dlouho uchovávat**: **Krátké**, **Vyvážené**, **Dlouhé** nebo **Vlastní hodnoty**, s časovou osou toho, které zálohy pravidlo drží. **Zeptat se před uchováním méně** se zeptá, než kratší pravidlo smaže zálohy, jejichž jediná kopie je tady.
- **Pravidla podle sekcí**: vlastní repozitář, uchovávání nebo komprese jedné sekce na tomto cíli. **Změnit pravidla** je otevře a **Vrátit na hodnoty cíle** sekci vrátí.
- **Už pod tímto cílem**: sekce, jejichž ručně zadaná adresa leží ve složce tohoto cíle. Převzetí jedné nechá repozitář, kde je; přihlášení, třída úložiště a ochrana proti smazání pak pocházejí z cíle. BombVault nejdřív zkontroluje, že přihlášení cíle repozitář otevře, a odmítne dát repozitář append-only pod cíl, který append-only není.
- Nádoba, která ukazuje, jak je cíl plný, se značkou, kde je plný, a kolik týdnů zbývá při růstu posledních týdnů. Cíl, který svou velikost nehlásí, říká **Velikost neznámá**.

Vzdálený cíl, který měl zálohy a najednou nemá žádný repozitář, třeba rest-server, jehož disk se nepřipojil, nedostane sám od sebe nový, prázdný. Jeho stránka to řekne, jakmile záloha nebo kopie zjistí, že repozitář chybí. Připoj disk znovu. Pokud jsi starý repozitář vyprázdnil záměrně, **Založit tu nový repozitář** začne znovu, pro všechny sekce cíle a jeho pojmenované repozitáře najednou.

Přijímací server, který provozuje jiná instance tvé skupiny, se objeví mezi poskytovateli jako **Z tvé skupiny**; viz [Přijímací server](#receiving-server).

### Přesun a odebrání {#move-remove}

- **Přesunout** vezme kopii jedné sekce do jiného cíle, s celou její historií. Všechno zůstane na starém místě; BombVault tam nic nemaže. Přesun běží na pozadí a může trvat dlouho: asi 9 minut na 100 GB mezi místními disky, hodiny přes pomalé spojení. Přehled ho za běhu ukazuje.
- **Odebrat cíl** nabízí dva způsoby. **Odebrat cíl, zálohy ponechat** přestane tam zapisovat a nechá zálohy na místě; později je přečteš v **Obnova, Ze staré složky se zálohami**. **Odebrat cíl a smazat tamní zálohy** nejdřív přečte, co je tam uloženo, a vypíše to: které zálohy zmizí, které z nich nejsou nikde jinde a které repozitáře zůstanou, protože ochrana proti smazání odmítá, položky se do nich stále zálohují přímo, jiný cíl nebo sekce tam drží své kopie, nebo se nedaly přečíst. Položky, které by přišly o jedinou zálohu, jsou vypsané zvlášť a ponechají se, pokud nezapneš **Smazat i tyto**. Potvrdíš napsáním názvu cíle a smazání běží na pozadí.
- Funguje to u každého druhu cíle: u cíle, který používá více sekcí, u cíle zřízeného jen pro jednu sekci, u repozitáře, který sis založil, i u složky, do které sekce zapisuje své vlastní zálohy. Položky, které se zálohují do odebraného repozitáře, pak zase následují svou sekci, stejně tak výchozí umístění sekce, které tam ukazovalo. Složku sekce nelze jen tak zrušit, protože ji potřebuje každá sekce. Okno se proto zeptá na složku, do které sekce přejdou, a každá sekce si pod ní ponechá název své složky.
- Smazání záloh v cíli vyžaduje přihlašovací heslo a přihlášení na instanci. Bez hesla lze cíl pořád odebrat, jen jeho zálohy odsud smazat nejdou.

## Umístění podle položky {#placement}

Karta **Plán a cíl** na záložce Nastavení položky má řadu tlačítek: **Místní** a jedno tlačítko pro každý cíl sekce mimo lokalitu. Rozsvícená tlačítka dostanou zálohy položky.

- S rozsvíceným **Místní** se položka zapisuje do repozitáře uvedeného pod **Uloženo na** a kopíruje se do každého dalšího rozsvíceného cíle. Zhasni cíl a nedostane od této položky nic nového. Samotné Místní nekopíruje nikam, což se hodí pro data, která už druhou kopii mají, například sdílenou složku, která žije na NAS.
- Se zhasnutým **Místní** se položka zapisuje přímo do přímého repozitáře prvního rozsvíceného cíle a odtud se kopíruje do ostatních rozsvícených. Poprvé ten přímý repozitář vytvoří dialog.
- Jedno tlačítko zůstává rozsvícené, protože záloha potřebuje místo, kam jít. Chceš-li položku ze záloh vynechat, použij **Nezálohovat**.

BombVault nikdy nepřesouvá zálohy mezi repozitáři. Položka, která se řídí svou sekcí, se přesune s ní, když se změní výchozí nastavení sekce: její další záloha skončí na novém místě a zálohy, které už má, zůstanou, kde jsou. Místo zvolené přímo u položky je pevné od její první zálohy; názvem ho pojmenovává karta **Repozitář** na záložce Nastavení. Kopie se mohou měnit kdykoli. Cíl, který položku už nedostává, si ponechá kopie, které má, a při dalším běhu mimo lokalitu té sekce je ořeže podle vlastního uchovávání. Z cílů append-only nelze mazat.

Místo, které položka opustila, si své zálohy ponechá. Na záložce **Zálohy** položky a v **Obnově** stojí jako **Jen starší zálohy**, obnova z něj může dál číst a prune tam tyto zálohy nechá být.

**Převzít výchozí** vrátí položku s vlastní volbou k její sekci. Nejdřív se zeptá, řekne, kam pak půjdou nové zálohy, a pojmenuje, co každý cíl přestane dostávat a co by příští běh nahrál.

Pod řádkem karta říká, kam položka jde a co tam skutečně je: kolik míst ji drží, kdy byl každý cíl naposledy vidět a zda je splněno 3-2-1. Místo je server s původními daty, každý cíl mimo lokalitu a každý repozitář označený **Mimo objekt**. BombVault kontroluje kopie a místa; část „dvě média“ z 3-2-1 nekontroluje.

### Výchozí umístění podle sekce {#placement-defaults}

Karta **Cíle podle sekcí** na stránce Cíle kreslí výchozí nastavení každé sekce jako tok 3-2-1. Každá čára ze sekce do cíle je přepínač, včetně **Místní**, a to u každé sekce, tedy i u flash disku, datových sad ZFS a autozálohy. Klepnutí na čáru nebo tečku ji přepne hned, bez dotazu; čárkované čáry jsou vypnuté. **Graf / Přepínače** ukazuje totéž jako řady tlačítek. Přepínač určuje, kam jdou nové zálohy. Položky bez vlastní volby a projektové složky stacků Compose se řídí hned, takže jejich další záloha jde na nová místa. Co už je zálohované, zůstane, kde je, a lze odtud dál obnovovat. Flash, datové sady ZFS a autozáloha se zálohují do své místní složky a z této zálohy se kopírují mimo lokalitu. Také jejich čáru k **Místní** lze vypnout; jejich zálohy pak jdou přímo na jejich cíle mimo lokalitu a položka ZFS s replikou a bez jakéhokoli cíle posílá jen svou repliku.

Když je **Místní** vypnuté a zapnuté jsou jen cíle mimo lokalitu, sekce zálohuje přímo na ně a BombVault tam repozitář vybere sám. Kontejnery, VM a sady složek jdou do přímého repozitáře prvního zapnutého cíle, jak je popsáno níže, a odtud se kopírují do ostatních. Flash disk, datové sady ZFS a autozáloha zapisují každou zálohu do každého zapnutého cíle a každý cíl je drží podle vlastního uchovávání.

Sekce, jejíž čáry jsou všechny vypnuté, ukazuje **Nezálohuje se**. Její položky nedostanou žádné zálohy a počítají se jako nechráněné.

Položky, které si na své stránce zvolily vlastní cíle, si je ponechají. Tok říká, kolik jich v sekci je, například **2 položky se liší**, a vede k nim: u jedné na záložku Nastavení položky, u více na seznam, který ukazuje, kam každá zálohuje. **Převzít výchozí** tam položku vrátí.

Nový cíl mimo lokalitu dostane každou položku, která není nastavená na Místní. Okno, které ho přidává, říká, kolik je to položek a, kde je to známo, kolik historie.

### Přímé repozitáře

Přímý repozitář je místo, kam položky zálohují přímo na cíl mimo lokalitu. BombVault ho umístí vedle složky cíle a k jejímu názvu přidá `-direct`, nebo, když cíl leží v kořeni svého bucketu, svého remote nebo uživatele rest-serveru, do složky v něm pojmenované podle sekce, například `containers-direct`. V toku na stránce Cíle vznikne bez dotazu. Vypnutí Místní u jedné položky, takže jejím domovem se stane cíl bez přímého repozitáře, otevře dialog s navrženým umístěním, například `s3:https://s3.eu-central-003.backblazeb2.com/bucket/containers-direct`, a test připojení, který nic nevytvoří. **Vytvořit a použít** repozitář vytvoří a nasměruje na něj položku. Přímý repozitář přebírá klíč cíle, třídu úložiště, limity, nastavení append-only a uchovávání a mění se s nimi; karta Repozitáře ho ukazuje jen pro čtení. Když nový klíč cíle nedokáže repozitář otevřít, ponechá si přímý repozitář klíč, který má, a uložení to říká. Položka na přímém repozitáři se kopíruje odtud do ostatních rozsvícených cílů, nikdy do cíle, ke kterému repozitář patří. Její snímky nesou značku `bv:direct` a každý jiný průchod uchovávání je ponechá, takže přímý repozitář, který ztratil vazbu na svůj cíl, nikdy nestárne podle místních pravidel. B2 se dosahuje přes jeho endpoint S3, s ID klíče a aplikačním klíčem zadanými jako přihlašovací údaje S3; klíč omezený na vlastní složku cíle se nedostane do složky vedle ní, takže klíč omez raději na složku nad ní.

### Mimo objekt

Cíl nebo pojmenovaný repozitář lze označit **Mimo objekt**. Vzdálené začínají označené; u rest-serveru ve stejné budově to vypni. Značka jen počítá místa a 3-2-1 na kartách. Žádnou kopii nemění.

Replika ZFS je kopie mimo lokalitu, ale není to záloha s historií. Drží jen snímky, které nastavíš, a staré verze a jednotlivé soubory se pořád berou ze záloh. Jde z jednoho serveru ZFS přímo na druhý, přes SSH na server ZFS nebo na přijímací koncový bod spárované instance. Skupina párování nese jen žádost. Viz [Replika](zfs-datasets.md#replica).

### Po opětovném sestavení {#after-a-rebuild}

Volby kopírování žijí ve vlastním nastavení BombVaultu. Po opětovném sestavení přes **Objevit zálohy** nebo Nouzového průvodce bez obnoveného nastavení jsou pryč a kopírování všeho by položky, které jsi vynechal, poslalo na B2 znovu. Kopie mimo lokalitu každé znovu sestavené sekce se proto pozastaví. Přehled to říká a karta **Cíle podle sekcí** nabízí **Potvrdit výchozí nastavení** s náhledem toho, co příští běh zkopíruje, a názvy v zálohách, které nemají položku a které tam můžeš vynechat. Pozastavení ukončí jen potvrzení; import souboru nastavení vrátí pravidla a výchozí hodnoty, ale pozastavení neukončí.

## Vzdálené primární repozitáře {#remote-primary-repositories}

Vlastní cesta sekce není omezená na místní složku: nasměruj ji rovnou na restic remote (`s3:...`, `rest:http://host:8000/repo`, `sftp:user@host:/repo`, `rclone:remote:bucket/path`) a BombVault zálohuje přímo tam, bez samostatné místní kopie a bez kroku kopírování. To je jiný tvar než kopie mimo lokalitu výše: tam je místní repozitář primární a repozitář mimo lokalitu je jeho archiv na základě nejlepší snahy; tady vzdálený repozitář **je** primární a je jedinou kopií, pokud tu sekci nekopíruješ ještě do jiného cíle.

Cesta každé sekce (Kontejnery, VMs, Flash, Autozáloha, Složky, Datové sady ZFS) se nastavuje na stránce cíle **Místní**: **Změnit** vedle sekce v jeho kartě **Připojení** otevře okno s přepínačem **Místní / Vzdálené**:

- **Místní** zobrazí známý prohlížeč složek.
- **Vzdálené** jej vymění za prosté pole URL a nastavení připojení, jaké používá cíl mimo lokalitu, nastavené místo toho pro tento primární repozitář. Odtud dostaneš:
    - **Test připojení** proti skutečné cestě, dřív než se na ni spolehneš.
    - **Limity šířky pásma** (nahrávání/stahování), aby plánovaná záloha na vzdálený primární repozitář nezahltila tvou WAN: stejné příznaky resticu `--limit-upload`/`--limit-download`, které používají kopie mimo lokalitu, použité na samotnou zálohu.
    - **Ochranu append-only**, ověřenou stejným aktivním tamper testem (skutečná sonda DELETE proti druhé straně), jaký dostávají cíle mimo lokalitu. Když je zapnutá, BombVault odmítá repozitář sám ořezávat, protože za ním není samostatná místní kopie a přihlašovací údaje na tomto stroji nesmějí umět smazat jedinou kopii zálohy.
    - **Výstrahu rozpočtu růstu**, odvozenou ze stejného trendu velikosti repozitáře, který karta Úložiště už sleduje.

Nic z toho není povinné: ručně zadaná vzdálená cesta bez uložených bezpečnostních nastavení zálohuje bez limitů, lze ořezávat a bez alarmu rozpočtu. Bezpečnostní nastavení tu jsou pro případ, že chceš ochrany, které kopie mimo lokalitu dostává, bez samostatného cíle jen kvůli nim.

!!! note "Přihlašovací údaje ke cloudu a REST jsou sdílené"
    Vzdálený primární repozitář se ověřuje sdílenými cloudovými přihlašovacími údaji v **Nastavení, Cíle**; pro primární repozitáře není samostatné úložiště přihlašovacích údajů.

### SMB a WebDAV bez připojení na hostiteli {#smb-webdav}

Karta **Off-site (rclone)** v **Nastavení, Cíle** má formulář pro sdílenou složku Windows nebo Samba a pro server WebDAV (Nextcloud, ownCloud, SharePoint nebo jakýkoli jiný). Vyplň krátký název, hostitele a sdílenou složku (SMB) nebo URL a typ serveru (WebDAV), uživatele a heslo a BombVault za tebe zapíše sekci rclone. rclone heslo před uložením sám zamaskuje; přidání se jménem, které už existuje, tu sekci nahradí místo přidání druhé. **Přidat cíl** nabízí SMB a WebDAV jako poskytovatele se stejnými poli.

Formulář odpoví hotovým umístěním, například `rclone:nas:backups`. Použij ho jako cestu sekce nebo cíl a případně přidej podsložku (`rclone:nas:backups/bombvault`). Sdílená složka je první segment cesty, ne součást názvu.

To je lepší cesta než připojit sdílenou složku na Unraidu: restic nedoporučuje držet repozitář na připojené sdílené složce CIFS a tady se nic nepřipojuje. NFS ve formuláři není, protože restic ani rclone nemají backend NFS; pro NFS připoj export na hostiteli a použij ho jako místní složku.

## Ochrana proti smazání (append-only)

Zapni ochranu proti smazání pro cíl mimo lokalitu, aby ransomware nebo napadený hostitel nemohl smazat ani přepsat tvé zálohy. Druhá strana (`restic/rest-server` běžící v režimu `--append-only`) ji **vynucuje**. BombVault ji jen **ověřuje** a nikdy neukáže zelenou jen na tvrzení v konfiguraci. Složka na tomto serveru mazání odmítnout nemůže, protože kdo ovládá BombVault, ovládá i tu složku, takže místní cíle přepínač nenabízejí.

Přepínač patří cíli: sekce, která do něj kopíruje, ho přebírá od cíle, takže ho zapni na stránce cíle v části **Cíle**. Plánovaná **Kontrola ochrany proti smazání** běží jen tehdy, když některá sekce kopíruje do cíle se zapnutým přepínačem. Druhá strana musí běžet s `--append-only`; u image `restic/rest-server`, na TrueNAS stejně jako na jakémkoli jiném hostiteli Dockeru, to patří do proměnné `OPTIONS`.

Nejrychlejší cesta ke kopii append-only je spárovaný BombVault s [přijímacím serverem](#receiving-server): rest-server za tebe provozuje v režimu append-only.

!!! note "Úspěšné smazání v `/locks/` je očekávané"
    Append-only neznamená, že už nelze nic smazat. restic musí zakládat a uvolňovat vlastní zámky, proto `/locks/` záměrně zůstává zapisovatelný a smazatelný. Snapshoty a data za nimi, tedy přesně to, na co by mířil ransomware, odstranit nelze. Pokud si vzdálenou stranu ověříš sám, úspěšné smazání v `/locks/` je správné chování, ne díra v ochraně.

!!! warning "Repozitáře append-only se z tohoto stroje nikdy neořezávají"
    Cíl append-only odsud staré zálohy nikdy neořezává; ořezávání je práce druhé strany. Nastav pro něj **alarm rozpočtu růstu**, abys byl upozorněn, než velikost repozitáře uteče.

## Test odolnosti proti manipulaci

BombVault pravidelně dokazuje záruku append-only tím, že se skutečně pokusí o smazání v repozitáři mimo lokalitu, zaměřené na neexistující objekt. **Otestovat ochranu proti smazání hned** na stránce cíle ho spustí na požádání a **Kontrola ochrany proti smazání** v **Nastavení, Ověření, Automatické kontroly** ho spouští podle plánu.

Vypnutá kopie se netestuje a cíl to uvádí. Test připojení u ní funguje dál, protože jen čte, takže kopii lze ověřit dřív, než ji zapneš.

- **Odmítnuto** znamená chráněno.
- **Přijato** znamená nechráněno.
- **Neprůkazný** výsledek (server nedosažitelný, chyba autentizace) nikdy nezmění uložený verdikt.

Skutečný přechod z chráněno na nechráněno spustí jediné upozornění.

## Cvičné obnovy po havárii

BombVault nabízí dvě úrovně důkazu, že tvoje zálohy jsou skutečně obnovitelné, nejen přítomné.

- **Cvičení ověření obnovy (místní).** BombVault pravidelně spouští `restic check --read-data-subset` (omezeně, nikdy ne plnou obnovu zaplňující disk). Kadence je v **Nastavení, Ověření, Automatické kontroly** a karta **Zkontrolovat zálohy** na stejné dlaždici ukazuje výsledek.
- **Zkušební obnova (mimo lokalitu).** BombVault obnoví skutečný cíl z repozitáře mimo lokalitu do uzavřeného pískoviště, ověří ho soubor po souboru a bajt po bajtu a pak uklidí. To dokazuje, že umíš obnovit z off-site, nejen že repozitář odpovídá.

Karta **Stav ochrany** v Přehledu to shrnuje podle sekcí do čtyř kontrol: aktuální záloha na tomto serveru, kopie mimo lokalitu, ochrana proti smazání tam a zkušební obnova odtamtud. Klepnutí na sekci ukáže podrobnosti a karta zezelená jen na ověřených faktech.

## Párování instancí {#pairing}

Několik BombVaultů pracuje společně jako jedna skupina: ukazují si navzájem stav ochrany, přebírají pro sebe role a mohou si navzájem držet klíč. Instance se do skupiny připojí dvanácti slovy.

Na první instanci otevři **Nastavení, Párování** a stiskni **Vygenerovat frázi**. Tři karty kroků ukazují, jak to jde. V okně se objeví dvanáct slov s tlačítkem **Kopírovat**. Na každé další instanci otevři totéž místo, stiskni **Zadat frázi** a slova vlož nebo napiš, nebo stiskni **Vložit** v tom okně. Slovo, které není v seznamu, se během psaní pojmenuje s jeho pozicí a poslední slovo nese kontrolní součet, takže se překlep nebo prohozené slovo zachytí dřív, než se cokoli spáruje. Frázi vygeneruj jen na jedné instanci: dvě instance, které ji obě vygenerují, tvoří dvě oddělené skupiny. Pokud se nikdo minutu neobjeví, stránka nabídne dvě cesty ven: ukázat slova znovu a zadat je tam, nebo zadat slova druhé instance a připojit se do její skupiny jedním krokem. Párování funguje bez přihlašovacího hesla, ale nastav si ho: bez něj může kdokoli, kdo otevře webové rozhraní, slova přečíst a přes skupinu žádat o role, které vydávají heslo resticu instancí v ní. Karta párování to říká, dokud se heslo nenastaví. S heslem se při opětovném zobrazení fráze na heslo zeptá. **Opustit skupinu** instanci zase vyjme. Karta **Instance** na stránce párování vypisuje členy skupiny; obrázek toho, kdo co pro koho dělá, je na stránce Instance.

Kdokoli zná ta slova, může se do skupiny přidat, takže s nimi zacházej jako s heslem.

**Jak se členové navzájem najdou.** Každá instance zjistí svou vlastní adresu v síti z tvého prohlížeče ve chvíli, kdy se přihlásíš, a je uvedena pod **Známé adresy** v kartě **Tato instance** na stránce párování spolu s těmi, které instance našla sama; oprav ji tam, pokud před ní stojí reverzní proxy nebo neobvyklý port. Ve stejné síti členové tuto adresu ohlašují multicastem a mluví přímo, a kde multicast nemůže projít sítí kontejneru, například výchozím mostem Dockeru, hledá instance ostatní ve své podsíti podepsaným voláním, na které může odpovědět jen člen skupiny, takže párování skončí během sekund i bez přeposílače. Když se nic neobjeví, **Nemůžeš ji najít?** přijme jednu adresu ručně, pro jinou podsíť nebo nestandardní port. Instance v různých sítích jdou přes přeposílač, zvolený v jedné řadě čtyř v kartě **Přeposílač** na téže stránce; každá podoba má svůj obrázek:

- **Přeposílač projektu** (výchozí): `parleyport.halleluja.design`, stejný přeposílač, jaký používá i KnightLoader. Není co nastavovat.
- **Vlastní přeposílač**: kontejner [**ParleyPort**](https://github.com/junkerderprovinz/parleyport) z Unraid Community Apps (také v **Nastavení, Aplikace**). Jeho adresu zadej na každé instanci, která ho má používat.
- **Instance BombVaultu**: jedna z tvých instancí, která je zvenčí už dosažitelná, se zapnutým **Sloužit jako přeposílač**. Ta instance pak odpovídá na `/relay/connect` na vlastní adrese, za reverzní proxy a certifikátem, které už má, a pouští dovnitř jen tvou skupinu. Její adresu zadej na každé instanci, která ji má používat.
- **Bez přeposílače**: členové se najdou automaticky jen ve stejné síti, a nikde jinde.

**Co přeposílač vidí.** Tlačítko stejného názvu vedle čtyř podob to ukáže pro tu, kterou sis zvolil. Každé volání mezi členy je zapečetěno AES-256-GCM klíčem odvozeným z dvanácti slov a ten klíč nikdy neopustí tvé instance. Přeposílač se dozví hash, který seskupuje spojení, pro kterou instanci zpráva je, jak je velká a kdy projde. Přímé volání v místní síti je zapečetěno stejně a navíc podepsáno, takže nic nezávisí na samopodepsaném certifikátu, který instance nabízí. Volání, která něco vydávají (odpovědi na role, heslo resticu, přihlášení příjemce, úschovy klíčů), jsou navíc podepsána klíčem člena, který je posílá, připnutým při prvním spatření; viz [Bezpečnostní model](configuration.md#security-model).

**Co putuje přes skupinu.** Stav ochrany na stránce Instance, žádost o kontrolu jedné sekce hned, žádosti o role a odpovědi na ně, nabídky cílů a to, co příjemce či stahovač potřebuje, jakmile je jeho role povolena: umístění repozitářů druhé instance a její heslo resticu. Data záloh nikdy; ta jdou dál přímo na backendy resticu. Ani APP_KEY: heslo resticu otevře repozitáře té instance a nic jiného, ne její uložená tajemství, relace ani kódy pro obnovu.

**Záznamy z doby před párováním.** Instance přidané tokenem flotily a přijaté repozitáře a zdroje stahování nastavené pomocí APP_KEY druhé instance po aktualizaci zůstanou a jsou označené **Spárovat znovu**. Dál fungují: při prvním startu BombVault nahradí každý uložený APP_KEY heslem resticu z něj odvozeným. Spáruj obě instance, pak záznam uprav a zvol jeho instanci. Do té doby takové záznamy stojí v kartě **Bez instance** na stránce Instance.

Jediné místo, které pořád bere APP_KEY ručně, je [Ze staré složky se zálohami](#restore-from-another-bombvault-repo), pro případ, že druhá instance zmizela a ve skupině odpovědět nemůže.

## Instance a role {#roles}

Stránka **Instance** (dlaždice nastavení, kterou lze připnout do postranního panelu) ukazuje skupinu jako síť toho, kdo co pro koho dělá, kartu pro tuto instanci a kartu pro každého dalšího člena, telefony s Androidem a servery ZFS. **Párování** vpravo dole vede na stránku párování. Které části jsou zapnuté, se nastavuje v **Nastavení, Instance, Více serverů**: **Instance**, **Přijímač** a **Stažení**.

Každá karta ukazuje stav ochrany člena a jeho role jako dlaždice. Role je něco, co jedna instance dělá pro druhou:

- **Přijímač** drží kopie tohoto serveru mimo lokalitu v rest-serveru v režimu append-only. Tento server posílá po každé záloze a nemůže tam nic smazat, ani kdyby ho někdo převzal.
- **Stahovač** stahuje zálohy tohoto serveru podle vlastního plánu. Tento server k tomu nemusí být vzhůru.
- **Server ZFS** přebírá repliky ZFS tohoto serveru. Každá položka ZFS si ho vybere pod **Replika** ve svém nastavení.
- **Strážce klíče** drží zapečetěnou kopii sady pro obnovu tohoto serveru. Viz [Strážce klíče](#key-keeper).

Zapnutí role pošle žádost a nic nezačne, dokud někdo na druhé instanci nestiskne **Povolit**. Žádost se ukáže na kartě té instance jako odznak a její **Podrobnosti** ji vypisují s **Povolit** a **Odmítnout**; povolenou roli lze tam kdykoli odvolat. Čárkovaná čára v síti je role, která ještě čeká. Instance na BombVaultu 9.x nemůže na žádost odpovědět, takže tam role platí hned po zapnutí, a žádost od instance 9.x tu čeká na **Povolit**.

**Podrobnosti** na kartě otevřou zbytek: přijaté repozitáře a zdroje stahování toho člena, nabídky cílů, sledování, **Odebrat** a **Zapomenout klíč**.

Vypnuté instance a telefony se zavřenou aplikací si nechají svou kartu jako **Neaktivní**, šedě, s údajem, kdy byly naposledy vidět, a v síti stojí ztlumené, dokud je přes **Odebrat** neodebereš. Hned po startu tohoto serveru ukazuje taková karta **Ověřuje se**, dokud skupina není zase dostupná. **Odebrat** na takové kartě funguje bez ohledu na to, které části jsou zapnuté, a ukončí také otevřené žádosti člena, jeho nevyužité přijímací sloty ZFS a jeho čekající žádosti o klíč. Karta **Instance** na stránce párování vede tyto členy také jako **Neaktivní**.

## Přijímač a stahovač {#receiver}

![Přijatá kopie mimo lokalitu v podrobnostech instance, sledovaná jen pro čtení, s kontrolou integrity spuštěnou na tomto hardwaru.](assets/screenshots/receiver.png)

*Přijatá kopie mimo lokalitu v podrobnostech instance, sledovaná jen pro čtení, s kontrolou integrity spuštěnou na tomto hardwaru.*

Na stroji, který **přijímá** kopie append-only od jiného BombVaultu, ti přijaté repozitáře dávají nezávislé sledování jen pro čtení na přijímajícím hardwaru, takže tiché selhání na druhém konci nezůstane nepovšimnuto. Zapni **Přijímač** v **Nastavení, Instance**, pak otevři **Podrobnosti** odesílající instance na stránce Instance a přidej přijatý repozitář (jen pro čtení, otevřený heslem resticu odesílající instance, které dostává přes [skupinu párování](#pairing)). Dostaneš:

- **Inventář podle zdroje**, takže vidíš přesně, které kontejnery, VM a sady složek dorazily.
- **Naposledy přijato** podle zdroje, takže víš, jak je každý čerstvý.
- **Nezávislého `restic check`** spuštěného na přijímacím hardwaru, takže integrita je ověřena tam, kde data skutečně leží, nejen na odesílateli.
- **Pojistky mrtvého muže:** upozornění, když zdroj přestane odesílat v okně, které nastavíš.
- **Upozornění na integritu:** upozornění, když kontrola na přijímací straně selže.

Přijímač je striktně jen pro čtení. Nikdy nezapisuje do přijatého repozitáře, takže nikdy nemůže porušit záruku append-only, na které odesílatel stojí.

Stahovač funguje opačně: se zapnutým **Stažení** instance čte zálohy jiného člena podle vlastního plánu, do vlastního repozitáře pro každý zdroj, takže se nikdy nepočítají jako vlastní zálohy tohoto stroje. Jeho zdroje stojí také v podrobnostech toho člena.

### Přijímací server {#receiving-server}

Přijímací stroj může také provozovat rest-server, na který ostatní kopírují. Nastaví se poprvé, když povolíš žádost **Přijímač**, která ho vyžaduje: okno říká, že server potřebuje přijímací server, a žádá o složku na sdílené složce, s **Nová složka** pro vytvoření jedné, a port (8000, pokud ho nepoužívá jiný kontejner). BombVault pak:

1. odmítne pokračovat, pokud už existuje kontejner s názvem `rest-server` nebo port drží jiný kontejner;
2. stáhne `restic/rest-server` a spustí ho přes Docker socket v režimu append-only se soukromými repozitáři a souborem s přihlášením ve složce;
3. zapíše jeho šablonu pro Unraid na flash disk, takže kontejner zůstane upravitelný na záložce Docker, nebo šablonu nabídne ke stažení, když je flash disk mimo dosah;
4. spustí na něm test odolnosti proti manipulaci a ukáže, zda odmítá mazání.

Instance tvé skupiny pak server najdou mezi poskytovateli pod **Z tvé skupiny**, pojmenovaný podle přijímacího stroje, a role **Přijímač** ho nabízí jako bezpečnou volbu. Každá instance dostane při prvním použití serveru vlastní přihlášení a zapisuje jen do své složky tam. Se zapnutým **Přijímač** v **Nastavení, Instance** otevře **Přijímací server** na kartě této instance server znovu. Jeho okno vypisuje přihlášení pod **Partneři s přihlášením** a **Odvolat přihlášení** jedno odebere; co ta instance už zkopírovala, zůstane ve složce. Nastavení také vytvoří jedno **Přihlášení mimo skupinu**, jehož heslo okno ukáže jednou.

Instance, která se k přijímacímu stroji dostane jen přes přeposílač, server použít nemůže, protože přeposílač nenese zálohy. Přidej nejdřív adresu přijímacího stroje na stránce párování. Když BombVault běží na vlastní IP adrese (například na br0), vyplň **Adresa pro partnery**, protože server naslouchá na adrese hostitele.

## Kompletní příklad: dva stroje Unraid, od začátku do konce

Vše výše popisuje jednotlivé díly. Tohle je jedno kompletní nastavení, protože díly se skládají snáz, když jsi je jednou viděl složené.

Dva stroje: **TOWER** provozuje kontejnery a odesílá zálohy; **VAULT** je přijímá a vynucuje append-only. Dosaď své vlastní názvy, adresy a cesty ke sdíleným složkám.

**1. Spáruj oba stroje.** Na TOWER otevři **Nastavení, Párování**, stiskni **Vygenerovat frázi** a dvanáct slov zadej na VAULT pod **Zadat frázi**. Na obou si předtím nastav přihlašovací heslo.

**2. Na TOWER požádej VAULT, aby byl přijímač.** Na stránce Instance zapni **Přijímač** na kartě VAULT, vyber sekce a **přijímací server VAULT** jako místo a odešli žádost.

**3. Na VAULT to povol.** Stránka Instance na VAULT ukáže žádost na kartě TOWER. Stiskni **Povolit**. Poprvé se VAULT zeptá na složku pro svůj přijímací server, například `user/appdata/rest-server`, spustí rest-server v režimu append-only a spustí proti němu tamper test.

**4. Zpět na TOWER.** Od té chvíle TOWER kopíruje na VAULT po každé záloze a nový cíl se objeví na stránce Cíle na TOWER s jeho ochranou proti smazání.

Co tamper test odpovídá:

| Výsledek | Co se stalo |
| --- | --- |
| **chráněno** | VAULT smazání odmítl. To je jediný vyhovující stav. |
| **nechráněno** | VAULT přijal smazání. `--append-only` chybí nebo byl odstraněn. |
| **neprůkazné** | Ani jedno. Obvykle URL není ta, kterou používá sám restic, nebo se změnily přihlašovací údaje. Nic se nezaznamená a nespustí se žádné upozornění. |

**5. Sleduj, co přichází, pokud chceš.** Zapni **Přijímač** v **Nastavení, Instance** na VAULT, otevři **Podrobnosti** TOWER a přidej přijatý repozitář. VAULT dostane heslo resticu TOWER přes skupinu; nikdo nepíše klíč.

**6. Udělej to oboustranné.** Zopakuj kroky opačným směrem: VAULT požádá TOWER, aby byl jeho přijímač. Každý stroj pak vynucuje append-only pro druhý a ani jeden nemůže smazat zálohy druhého.

Funguje i rest-server, který provozuješ ručně: přidej ho přes **Přidat cíl** jako rest-server, s uživatelem a heslem z jeho `.htpasswd`, a zapni ochranu proti smazání. Nech `--append-only` v jeho volbách; bez něj je druhá strana zase obyčejná sdílená složka.

!!! warning "Místní cesta je cesta uvnitř kontejneru"
    Kde BombVault žádá o složku na tomto serveru, zadej ji relativně k připojení hostitele, například `user/appdata/rest-server/bombvault-containers`, ne `/mnt/user/appdata/...`. BombVault běží v kontejneru, kde je `/mnt` hostitele připojeno jinde; absolutní cesta hostitele uvnitř neexistuje. Pokud ji vložíš, BombVault ti řekne relativní cestu, kterou použít místo ní.

## Nouzový průvodce {#emergency}

**Nouzový průvodce** vrátí server, který zmizel nebo byl nastaven znovu. Začíná na stránce **Obnova** pod **Nouze** a z **Obnovit ze zálohy** na uvítací stránce nové instalace. Běží ve čtyřech krocích: **Přístup**, **Nastavení**, **Výběr** a **Obnova**.

1. **Přístup.** Ukaž BombVaultu klíč starých záloh jedním ze tří způsobů:
    - **Nahrát sadu pro obnovu**: soubor, který jsi stáhl při nastavení. Obsahuje klíč a každý cíl a BombVault z něj doplní zbytek. Sada zapečetěná pomocí age žádá o identitu, pro kterou byla zapečetěna; použije se jednou a neukládá se.
    - **Přes skupinu**: instance tvé skupiny drží klíč jako úschovu. Potřebuješ dvanáct slov skupiny, svůj kód úschovy a někoho u té instance. Viz [Strážce klíče](#key-keeper).
    - **Zadat klíč**: `APP_KEY` starého serveru a kde jsou jeho zálohy.

    BombVault pak klíč vyzkouší na každém cíli a pro každý řekne, zda je čitelný, nenalezen, zamčený jiným klíčem, nedosažitelný, nebo mu ještě chybí přihlášení. Nic se zatím neuložilo a klíč zůstává v paměti omezenou dobu. **Převzít klíč** způsobí, že tento server odteď píše zálohy tímto klíčem; **Vrátit převzetí** se vrátí k jeho vlastnímu klíči, dokud neproběhne první záloha. Server, který už má vlastní zálohy, převzetí odmítne a odkáže na [Ze staré složky se zálohami](#restore-from-another-bombvault-repo). **Uložit přihlašovací údaje** uloží přihlášení, která sada nese pro vzdálené cíle, a zeptá se před nahrazením přihlášení, která tento server už má.
2. **Nastavení.** **Vrátit nastavení** obnoví vlastní nastavení BombVaultu z nejnovější autozálohy: cíle, přihlašovací údaje, plány a pravidla. BombVault se restartuje, aby je použil, přes Docker socket, takže živá databáze nastavení se nikdy nepřepíše pod otevřeným handlem, a průvodce poté pokračuje sám. Přihlášení tohoto serveru zůstává: dál se přihlašuješ heslem, které používáš teď. Tajemství, která starý server uložil a nelze je převzít, jsou vypsána, takže je můžeš zadat znovu. Obnova ukončí všechny relace a odstraní přístupové klíče, takže si je potom znovu nastav.
3. **Výběr.** BombVault projde tvé zálohy a vypíše kontejnery, VM, sady složek a datové sady ZFS, které najde. Všude je předvybrána nejnovější záloha. Co tento server ještě má, začíná vypnuté, protože by to obnova nahradila. Sady složek po opětovném sestavení nevědí, odkud pocházely, takže jdou do složky, kterou zvolíš.
4. **Obnova.** Vše vybrané se vrátí najednou, ponechané zastavené, pokud chceš každý kontejner a VM spustit sám, což je po úplné ztrátě nejbezpečnější. Poté stáhni novou sadu pro obnovu: stará patří předchozí instalaci.

!!! tip "Plánovaná migrace versus havárie"
    Nouzový průvodce obnovuje vlastní nastavení BombVaultu ze zálohy. Pro *plánovaný* přesun na nový stroj můžeš místo toho konfiguraci přenést přímo kartou **Export / import nastavení** (přenosný soubor JSON). Viz [Konfigurace](configuration.md#portable-settings-export-and-import).

### Ze staré složky se zálohami {#restore-from-another-bombvault-repo}

**Ze staré složky se zálohami** pod **Další zdroje** na stránce Obnova otevře repozitář *jiné* instance BombVaultu (sdílenou složku připojenou pod `/mnt` nebo vzdálenou adresu) se sadou pro obnovu té instance nebo jejím napsaným `APP_KEY`, v jednorázové relaci jen pro čtení. Prohlížej kontejnery, VM, sady složek a datové sady ZFS, které tam jsou uloženy, vyber snímek a obnov ho, a obnovený objekt se stane běžným místním kontejnerem, VM nebo sadou složek. Do druhého repozitáře se nikdy nic nezapíše a tvá vlastní nastavení záloh zůstanou nedotčená (relace žije v paměti a vyprší sama). Přesun kontejneru ze serveru A na server B tak neznamená přesměrovat nastavení a pak je vracet. Toto okno je jednorázové: otevře relaci, obnoví, co vybereš, a druhou instanci zapomene. Chceš-li místo toho trvalé uspořádání, kde tento stroj podle plánu stahuje snímky jiné instance do svého repozitáře, je to role **Stahovač** na stránce Instance.

Kontejner, jehož síť na tomto serveru neexistuje, třeba síť `br0` z Unraidu na běžném Docker hostiteli, ukáže pod svým řádkem výběr sítě. BombVault ho vytvoří ve zvolené síti, spolu s jeho ostatními sítěmi. Pevná IP a MAC adresa patřily ke staré síti a odpadají, obě přidělí nová síť.

## Strážce klíče {#key-keeper}

Strážce klíče je instance tvé skupiny, která drží zapečetěnou kopii sady pro obnovu tohoto serveru. Kopii sama otevřít nemůže. Když se tento server někdy ztratí, nová instalace, která se připojí do tvé skupiny, o kopii strážce požádá a otevře ji kódem úschovy.

1. Na stránce Instance zapni **Strážce klíče** na kartě instance, která ji má držet, a zvol **Uložit do úschovy**. Uložení do úschovy vyžaduje přihlašovací heslo.
2. Tento server jednou ukáže **Kód úschovy**: dvacet písmen a číslic ve čtyřech skupinách po pěti. Zapiš si ho a potvrď **Kód je zapsaný**. Tento server kód neukládá, jen to, co potřebuje k zapečetění úschovy pro něj.
3. Strážce teď drží úschovu a tento server ji obnovuje, když se sada změní. **Úschova klíče** na kartě této instance vypisuje **Úschova tohoto serveru** u každého strážce a jeho stav a **Úschovy, které tento server uchovává** pro ostatní. **Vytvořit nový kód** kód nahradí; každý strážce přejde na nový kód, jakmile dostane novou úschovu.

V ten den potřebuješ tři věci: dvanáct slov své skupiny, kód úschovy a přístup k rozhraní strážce, kde někdo musí stisknout **Povolit**. V Nouzovém průvodci zvol **Přes skupinu**, vyber úschovu, vyžádej ji a napiš kód úschovy. Kód se použije na novém serveru a strážci se neposílá. Na strážci se žádost ukáže pod **Úschova klíče** s tím, kdo žádá, zda byla ta instance tam někdy předtím vidět a kolikrát žádala; **Povolit** vydá zapečetěnou kopii a povolení vyžaduje přihlašovací heslo. Nová instalace má na vyzvednutí po povolení 30 minut, žádost, na kterou nikdo neodpoví, vyprší za den a instance, která žádá příliš často, musí počkat hodinu.

Bez kódu kopii nikdo otevřít nemůže, ani strážce. Pokud se kód ztratí, vytvoř nový, dokud tento server ještě běží; je-li server už pryč, sada pro obnovu a napsaný APP_KEY pořád fungují. Strážce může úschovu kdykoli smazat a vlastník si pak musí požádat znovu.

## Sada pro obnovu {#recovery-kit}

Toto je díl, který umožňuje zotavení po havárii, i když neběží žádný BombVault.

**Stáhnout sadu pro obnovu** v kartě **Pro nouzové případy** v **Nastavení, Obnova** ti dá **klíč**, **odvozené heslo resticu**, každý **cíl** s repozitářem každé sekce, přihlášení, která nová instalace potřebuje k jejich dosažení, a **přesné příkazy resticu**, takže můžeš obnovovat rovnou pomocí restic CLI na jakémkoli stroji. Vyžaduje přihlašovací heslo. Sada klíč vytiskne i při vypnutém šifrování, protože definice kontejnerů a VM jsou jím vždy zapečetěny. Po převzetí vytiskne převzatý klíč záloh a APP_KEY tohoto serveru, který nová instalace potřebuje jen k vrácení nastavení. Karta **Ulož si sadu pro obnovu** v Přehledu ti to připomíná, dokud sadu neuložíš.

!!! danger "Ulož sadu pro obnovu mimo server"
    Sada obsahuje tajemství, které dešifruje tvé zálohy. Ulož ji na bezpečné místo oddělené od serveru (správce hesel, vytištěná kopie v trezoru). Pokud ztratíš BombVault i `APP_KEY` bez sady pro obnovu a bez úschovy klíče, šifrované zálohy nelze obnovit.

!!! warning "Nejnovější snímek není vždy ten, který obnovit"
    Od restic 0.17 ukazuje `restic snapshots` velikost každého snímku. Po ztrátě dat může být nejnovějším snímkem ten vyprázdněný, takže neobnovuj snímek, který je mnohem menší než ty před ním. Po ransomwaru to může být ten zašifrovaný v obvyklé velikosti. Pokud BombVault ještě běží, podívej se nejdřív na stránku **Anomálie**: pojmenuje poslední dobrou zálohu a okno obnovy ji předvybírá. Obnova nepotřebuje žádná data o anomáliích z BombVaultu a pozastavení uchovávání vždy jen ponechá víc snímků.

### Zapečetění sady

Pokud jsi v **Nastavení, Systém** zapnul **Šifrovat exporty pomocí age**, sada je jím zapečetěna také a stahuje se jako `bombvault-recovery-kit.md.age`. Je ASCII-armored, ne binární, takže je to pořád prostý text: vložení do správce hesel nebo vytištění funguje přesně jako dřív, jen obsah je bez tvého klíče nečitelný.

!!! warning "Klíč age neukládej do sady"
    K otevření zapečetěné sady potřebuješ svůj **soukromý** klíč age. Ulož ho někde, kde nezávisí na sadě samotné, jinak budeš obnovovat dvě věci místo jedné. Zapečetění se vyplatí, když sada leží na místě, které plně nemáš pod kontrolou (sdílený správce hesel, poznámky v cloudu, výtisk v kanceláři); sada ve vlastním trezoru je už chráněná trezorem.

    Se zapnutým šifrováním a bez nastaveného použitelného příjemce se stažení rovnou odmítne. BombVault se nikdy neuchýlí k vydání hlavního klíče v otevřené podobě.

### Když sada není po ruce

Heslo není nikde uloženo, **počítá se** z klíče záloh, takže ho umíš sám zopakovat jen s klíčem a shellem:

```sh
printf 'bombvault:restic-repo'   | openssl dgst -sha256 -mac HMAC -macopt hexkey:$APP_KEY -r   | cut -d' ' -f1
```

To je HMAC-SHA256 nad pevným řetězcem `bombvault:restic-repo`, s klíčem tvořeným surovými bajty hex `APP_KEY`, vytištěný jako 64 malých hexadecimálních znaků. Stejná hodnota je v sadě, uvedená jako odvozené heslo resticu; tohle je pro den, kdy je sada někde, kde nejsi. Na serveru, který převzal zálohy jiného, použij převzatý klíč záloh, což je starý APP_KEY toho serveru.

!!! warning "U přijatého úložiště použij klíč ODESÍLAJÍCÍ instance"
    Repozitář, který sem přišel kopií mimo lokalitu, vytvořil stroj, který ho poslal, **jeho** klíčem. Odvození z klíče přijímajícího stroje vyrobí heslo, které restic odmítne, a to vypadá přesně jako poškozený repozitář, ale není. To je obvyklý důvod, proč `restic check` na přijatém repozitáři dokola žádá o heslo.

Protože definice pro obnovu žijí **uvnitř** každého repozitáře (`<repo>/def`, `<repo>/vm-def`), je zkopírovaná složka repozitáře plně soběstačná, takže sada plus repozitář jsou vším, co obnova na holém železe potřebuje.

## Získání dumpu databáze zpět {#database-dumps}

Dump databáze je vlastní bod obnovy v repozitáři kontejnerů, označený `dbdump:<container>`, obsahující jediný soubor `/dbdump/<container>.sql`. BombVault je vypisuje, stahuje a importuje na záložce **Zálohy** kontejneru; níže jsou stejné kroky jen s resticem, pro den, kdy BombVault není k dispozici.

```sh
restic -r <repo> snapshots --tag dbdump:<container>
restic -r <repo> dump --tag dbdump:<container> latest /dbdump/<container>.sql > <container>.sql
```

Štítky `dbversion:` a `dbname:` u každého dumpu říkají, z jaké verze serveru pochází a jaké databáze obsahuje. Úplný soubor končí řádkem `-- PostgreSQL database cluster dump complete` nebo `-- Dump completed`.

Naimportuj ho do kontejneru ve stejné nebo novější verzi (PostgreSQL), případně ve stejné hlavní verzi (MySQL a MariaDB), jednou spuštěného s prázdnou datovou složkou, aby se inicializoval. Hostitel žádného databázového klienta nepotřebuje, kontejner ho má:

```sh
docker exec -i <container> sh -c 'exec psql -X -U "${POSTGRES_USER:-postgres}" -d postgres' < <container>.sql
docker exec -i <container> sh -c 'exec mariadb -uroot -p"$MARIADB_ROOT_PASSWORD"' < <container>.sql
docker exec -i <container> sh -c 'exec mysql -uroot -p"$MYSQL_ROOT_PASSWORD"' < <container>.sql
```

Pro jednu databázi z úplného dumpu berou MySQL a MariaDB `--one-database <name>` v příkazu klienta. Dump PostgreSQL má na každou databázi jednu sekci, každá začíná řádkem `\connect <name>`: zkopíruj tu svou do vlastního souboru a po vytvoření databáze ho naimportuj s `-d <name>`.

!!! warning "Dump pořízený jako root nese uživatele serveru"
    Úplný dump MySQL nebo MariaDB pořízený jako root obsahuje systémovou databázi `mysql`, takže jeho import nahradí účty nového serveru, včetně hesla roota, účty z dumpu. U PostgreSQL je `role ... already exists` pro uživatele, kterého vytvořil kontejner, očekávané a neškodné.
