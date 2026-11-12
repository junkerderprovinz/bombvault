# Mimo lokalitu a obnova

!!! note "Kopie mimo lokalitu čekají po opětovném sestavení"
    Když Nouzový průvodce nebo **Objevit zálohy** znovu sestaví položky bez starého nastavení, kopie mimo lokalitu těchto sekcí čekají, dokud jejich zálohy nepřevezmeš jako vlastní zálohy tohoto serveru. Viz [Po opětovném sestavení](#after-a-rebuild).

Místní zálohy tě chrání před ztraceným kontejnerem nebo špatnou aktualizací. Kopie mimo lokalitu a vyzkoušená sada pro obnovu tě chrání před ztrátou celého stroje, ransomwarem nebo požárem. Tato stránka se věnuje cílům a kopiím mimo lokalitu, tomu, jak kopii ochránit před manipulací, jak prokázat, že obnova funguje, párování více serverů a tomu, jak dostat všechno zpátky, když BombVault sám zmizí.

## Kopie mimo lokalitu {#off-site-copies}

Nech si rychlou místní zálohu a přidej jeden či více cílů mimo lokalitu. BombVault tam nové snapshoty kopíruje pomocí `restic copy` na základě nejlepší snahy, takže zádrhel mimo lokalitu nikdy nezhatí místní zálohu. V tomto tvaru každá sekce zapisuje nejdřív do své vlastní složky a repozitář mimo lokalitu je kopie. Vlastní složka nemusí být na tomto serveru: jak ji držet v cíli, jako je S3 nebo rest-server, viz [Vlastní složka sekce](#own-folder).

- **Několik cílů na sekci.** Každá sekce (kontejnery, VM, flash, autozáloha, složky a datové sady ZFS) může kopírovat do několika cílů mimo lokalitu najednou, takže můžeš mít například rest-server na krabici kamaráda a bucket S3 souběžně. Karta **Cíle podle sekcí** na stránce Cíle zapíná nebo vypíná každou linku ze sekce do cíle.
- **Kdy se kopíruje.** Karta **Kopírovat** cíle určuje **Kdy kopírovat**: **Po každé záloze**, nebo podle vlastního plánu, který snapshoty sbírá a kopíruje v pevných časech. V **Nastavení, Plán, Kopie mimo lokalitu podle sekcí** dává **Výjimka** jedné sekci vlastní plán kopírování.
- **Karta Kopie mimo lokalitu.** V **Nastavení, Cíle** má každá sekce s kopií mimo lokalitu řádek v **Kopie mimo lokalitu**, s **Otestovat připojení**, **Replikovat nyní** pro spuštění kopie na požádání a **Nastavit** nebo **Upravit**. Zbytek nastavení kopírování sekce se odtud otevře v jejím okně.
- **Uchovávání** se nastavuje pro každý cíl pod **Jak dlouho uchovávat**, takže kopie mimo lokalitu můžeš držet déle jako archiv. Kopie v poli sekce mimo lokalitu se jím řídí jako každá jiná kopie a sekce si může ponechat vlastní hodnoty pod **Pravidla podle sekcí**. Samé nuly snapshoty mimo lokalitu nikdy neořezávají. Viz [Jak dlouho uchovávat](#keep).
- **Limity šířky pásma** (**Šířka pásma mimo lokalitu** v **Nastavení, Cíle**, nebo karta **Kopírovat** cíle) omezují rychlost nahrávání a stahování resticu, aby kopie nezahltily tvou WAN. Zadávají se v KiB/s, což je jednotka, kterou restic bere, a každé pole při psaní ukazuje stejnou rychlost i v MB/s a Mbit/s: 2048 se čte jako asi 2,0 MB/s (16,8 Mbit/s).
- **Komprese** je **Vypnuto**, **Automaticky** (výchozí v resticu) nebo **Maximum**, nastavená u každého cíle a pro jednu sekci v jejích pravidlech tam. Zálohy, kopie i čištění zapisují s ní a sada pro obnovu ji jmenuje, takže prostý restic může zapisovat dál stejně.
- **Nejdřív streamování.** Zatímco streamuje mediální server jako Plex, Jellyfin nebo Emby, kopie nahrávají s nižším limitem a několik minut po skončení streamu se vrátí k běžnému. Karta **Streamování** na stránce Cíle má **Zpomalit kopie mimo lokalitu při streamování**, **Mediální servery** (kontejnery s Plex, Jellyfin nebo Emby v názvu image, dokud nevybereš), **Stream od (Mbit/s)**, **Limit nahrávání při streamování (KiB/s)** a **Zpět k normálu po (min)**. BombVault čte provoz z Dockeru, takže mediální server v síti hostitele změřit nejde. REST, S3, B2, Azure, Google Cloud, Swift a rclone přes HTTP zpomalí uprostřed kopírování. SFTP a místní či připojené složky berou nižší limit při dalším kroku kopírování.
- **Studené úložiště (S3).** Cíl S3 může použít levnější **Třídu úložiště**, kterou restic přesto přečte hned, například Standard-IA, One Zone-IA, Intelligent-Tiering nebo Glacier Instant Retrieval. Glacier Flexible Retrieval a Deep Archive jsou vynechány, protože z nich restic obnovit neumí. Remote rclone nastavuje třídu v konfiguraci rclone.
- **Souběžná připojení.** U cíle S3, B2, Azure, Google Cloud, Swift, rest-server, SFTP nebo rclone určuje **Souběžná připojení** na jeho kartě **Připojení**, kolik připojení restic otevře najednou, od 1 do 32. Když pole necháš prázdné, zůstane restic u své výchozí hodnoty 5. Víc jich může zrychlit vzdálenou nebo pomalou linku, méně jich ušetří poskytovatele, který omezuje rychlost. Používají je zálohy, kopie, kontroly, čištění i obnovy na tomto cíli a export nastavení je přenáší. Místní ani připojená složka takové nastavení nemá.
- **Rozpočet růstu.** **Rozpočet růstu (GB, 0 = vypnuto)** v okně kopírování sekce spustí poplach, když repozitář přeroste tuto velikost. Nikdy nic nemaže, takže se hodí pro kopii append-only, kterou odtud nic nečistí.
- Zatímco kopie běží, hlavička stavu v Přehledu říká **Probíhá kopie mimo lokalitu**. Je to ukazatel činnosti, ne procentní pruh, protože `restic copy` nezveřejňuje strojově čitelný průběh.

!!! note "Obnova z libovolného místa"
    Každá položka vypisuje své zálohy jako jednu časovou osu přes všechna místa, kde záloha leží, na záložce **Zálohy**. Záloha zkopírovaná na B2 se ukáže jednou, označená každým místem, které ji drží. Obnova bere první místo, kam se dostane, počínaje repozitářem, do kterého se položka zapisuje, a u každého snapshotu můžeš vybrat jiné. Cíle mimo lokalitu se čtou, až když klikneš na **Kontrola**, protože výpis cloudového úložiště chvíli trvá. Smazání na jednom místě nejdřív zkontroluje ostatní a řekne, zda šlo o poslední kopii. Viz [Obnova](restore.md).

## Cíle {#destinations}

Cíl je místo, kam zálohy jdou: místní složka, sdílená složka, tvůj vlastní server nebo cloudová služba. Nastaví se jednou a pak slouží každé sekci. **Cíle** otevřeš z jejich dlaždice v nastavení, nebo tam zapni **Zobrazit také v postranním panelu**, aby se připnuly.

![Stránka Cíle: seznam cílů a tok, která sekce kam míří.](assets/screenshots/storage/locations-dark.png#only-dark)
![Stránka Cíle: seznam cílů a tok, která sekce kam míří.](assets/screenshots/storage/locations-light.png#only-light)

**Přidat cíl** otevře okno o čtyřech krocích:

1. **Poskytovatel.** Každá služba je vypsána se svým logem, s vyhledáváním vpravo nahoře v okně, ve skupinách: **Místní** pro disk nebo sdílenou složku připojenou k tomuto serveru, **NAS nebo server** (rest-server, Hetzner Storage Box, SFTP, SMB, WebDAV a další), **Služba S3** (Backblaze B2, Wasabi, Cloudflare R2, Hetzner Object Storage, Amazon S3 a další), tvůj vlastní server S3 (Garage, SeaweedFS, RustFS, Silo, Ceph, JuiceFS, Versity S3 Gateway), **Cloudové úložiště** (OneDrive, Google Drive, Dropbox, pCloud, Nextcloud a zbytek, co rclone podporuje) a **Z tvé skupiny**, přijímací servery tvé skupiny. Každá říká, jak se hodí pro zálohy: cloudové disky při mnoha požadavcích zpomalují, takže první záloha a čištění tam trvají déle. Pro službu, kterou žádná dlaždice nepokrývá, bere dlaždice **Rclone** pod **Vlastní konfigurace rclone** remoty vložené z rclone.conf.
2. **Přihlášení.** Pole závisejí na službě: přístupový klíč pro S3, uživatel a heslo pro WebDAV a SMB, heslo aplikace tam, kde dvoufázové přihlášení blokuje běžné, veřejný klíč SSH BombVaultu pro SFTP a Storage Box, nebo token pro služby, které se přihlašují přes prohlížeč. Pro ně okno ukáže příkaz `rclone authorize`, který spustíš na počítači s prohlížečem, a token, který vypíše, jde do pole. Pokud už pro službu máš [uložené přihlašovací údaje](#sign-ins), vybere je **Uložené přihlašovací údaje** místo **Nové klíče**. **Otestovat připojení** ověří přihlášení dřív, než se cokoli uloží. Okno pak vypíše složky v cíli, s **Nová složka** pro vytvoření jedné a s volným místem tam, kde ho služba hlásí. Prázdná složka je nejbezpečnější. BombVault do ní dá jednu složku na sekci.
3. **Ochrana.** Okno říká jasně, co služba umí. Rest-server v režimu append-only mazání odmítá a tamper test to ověřuje. Bucket S3 může držet staré verze pomocí verzování a zámku objektů, což BombVault zatím ověřit neumí. Cloudový disk mazání odmítnout vůbec neumí: kdo se dostane na server, dostane se i k té kopii. **Ochranu proti smazání (append-only)** zapni jen tam, kde druhá strana mazání opravdu odmítá. BombVault tam pak nikdy nečistí. **Pro případ nouze** říká, co sada pro obnovu pro tento cíl drží.
4. **Pro které sekce.** Vyber sekce, které se tam zálohují. První kopie přenese celou historii. Položky, které si zvolily vlastní cíle, nový nedostanou samy a okno je jmenuje.

![Přidat cíl: první krok, s každým poskytovatelem jako dlaždicí.](assets/screenshots/storage/add-providers-dark.png#only-dark)
![Přidat cíl: první krok, s každým poskytovatelem jako dlaždicí.](assets/screenshots/storage/add-providers-light.png#only-light)

![Přidat cíl: krok přihlášení po úspěšném testu.](assets/screenshots/storage/add-signin-dark.png#only-dark)
![Přidat cíl: krok přihlášení po úspěšném testu.](assets/screenshots/storage/add-signin-light.png#only-light)

Některé služby žádají při přihlášení o trochu víc:

- **SFTP a Storage Box.** **Přidat klíč** vloží veřejný klíč BombVaultu na server pomocí **Heslo účtu**, které se použije pro to jedno přihlášení a nikde se neuchovává. Příkaz pro ruční postup je zobrazen pod tím. Test ukáže **Klíč serveru** s jeho otiskem. BombVault si ho zapamatuje a server odmítne, pokud později ukáže jiný.
- **Klíče S3 pro jediný bucket**, například objektový token Cloudflare R2 nebo omezený klíč AWS, nemusí vypsat ostatní buckety. Napiš bucket do **Bucket** a test i seznam složek otevřou ten bucket.
- **Regiony a adresy S3.** Region se čte z endpointu, který ho jmenuje (AWS, Backblaze B2, Hetzner, Scaleway, OVH, Wasabi), Garage začíná vlastním výchozím regionem a IDrive e2 chce endpoint, který ukazuje jeho dashboard. Adresa vložená i s bucketem, tak jak ji ukazuje dashboard R2, vyplní server i bucket. Bucket vytvořený z prohlížeče složek jde do regionu, který jmenuje formulář.
- **Třída úložiště.** Výběr nabízí jen třídy, které služba bere, a **Další nastavení** ukazuje jen volby, které tvarují nový bucket.
- **Google Drive a OneDrive.** Vyčištěná data se mažou natrvalo místo toho, aby šla do koše, kde by se pořád počítala do kvóty. Sdílený disk Google patří pod **Další nastavení**.
- **Servery SMB, které nevypisují své sdílené složky**, berou jméno jedné pod **Sdílená složka** a test otevře přímo tu složku.
- **FTP** se ptá na **Šifrování**: **Žádné**, **Explicitní TLS (FTPES)** nebo **Implicitní TLS (FTPS, port 990)**.
- **OneDrive a Zoho WorkDrive.** Když přihlášení otevře více disků nebo pracovních prostorů, okno se zeptá, na který zálohy půjdou.
- **Rest-server se samopodepsaným certifikátem.** Ulož certifikát jako soubor PEM do složky `certs` v datové složce BombVaultu (`/config/certs` v kontejneru, `appdata/bombvault/certs` na Unraidu). Zálohy, kopie i test ochrany proti smazání mu důvěřují bez restartu.

Služby S3 běží přes vlastní backend S3 resticu, díky čemuž se může uplatnit třída úložiště a zámek objektů. K rest-serveru se restic připojuje přímo. Každá další služba běží přes rclone, který BombVault dodává, a její sekce konfigurace rclone se ukazuje v kartě **rclone.conf** na stránce cíle, s skrytými tajemstvími. Export nastavení nese cíle a s přihlašovacími údaji nese i jejich přihlášení.

Uložení cíle, který už drží zálohy kontejnerů, VM nebo složek, například jednoho, který jsi odebral a přidáváš znovu, ho přidá k těmto sekcím jako nezaškrtnutou kopii. Nic se tam nekopíruje, dokud ho nezaškrtneš, ale **Obnova** nabídne staré zálohy hned.

Složka na tomto serveru se jmenuje všude **Místní**: na stránkách úložiště, v toku, u položky i v Obnově. Při několika složkách na tomto serveru jde každá pod svým jménem. Cíl, který založí pole sekce mimo lokalitu, se jmenuje podle svého místa, poslední složky své adresy nebo, bez ní, svého serveru, a dostane číslo, když už pod tímto jménem jiný cíl je. Každý cíl lze přejmenovat v poli **Název** své karty **Připojení**, také složku v bucketu, kterou sdílí několik sekcí, a kopii, kterou popisuje pole sekce mimo lokalitu, a jméno, které mu dáš, zůstane. Kde stojí vedle sebe dva cíle se stejným jménem, nese každý svůj server nebo složku.

### Stránka cíle {#location-page}

Každý cíl má vlastní stránku. Ukazuje jen karty, které se na daný cíl vztahují, v tomto pořadí:

![Stránka cíle: Vlastní složky, Připojení a Přihlašovací údaje.](assets/screenshots/storage/location-page-dark.png#only-dark)
![Stránka cíle: Vlastní složky, Připojení a Přihlašovací údaje.](assets/screenshots/storage/location-page-light.png#only-light)

- **Vlastní složky**: sekce, které sem zapisují své zálohy jako první. Přihlášení, ochrana a limity tohoto cíle na ně platí. **Použít jako vlastní složku** sem přesune vlastní složku dalších sekcí a **Změnit** jednu zase odsune. Viz [Vlastní složka sekce](#own-folder).
- **Zálohuje se rovnou sem**: každá sekce, jejíž položky zálohují rovnou na toto místo místo do vlastní složky, a kolik položek to je. Takový přímý repozitář bere všechna svá nastavení od tohoto cíle. Ten, na který už nic nezálohuje, to říká a **Odebrat** ho vyřadí ze seznamu. Zálohy na jeho místě zůstávají. Viz [Přímé repozitáře](#direct-repositories).
- **Připojení**: **Název**, přepínač **Dostupné** (vypnuto znamená, že BombVault sem už nezapisuje a položky, které sem zálohují, selžou s jasnou zprávou, místo aby skončily jinde), složka, **Komprese**, **Třída úložiště**, **Souběžná připojení** a **Mimo areál**. U cíle **Místní** vypisuje pod **Vlastní složky zde** složku každé sekce. **Změnit připojení** otestuje a uloží novou adresu nebo přihlášení. Když se stejnými uloženými přihlašovacími údaji přihlašují i jiná místa, okno to řekne a změna zde dá tomuto cíli vlastní přihlášení místo přepsání sdíleného.
- **Přihlašovací údaje**: které uložené přihlašovací údaje používá (**Přihlašuje se pomocí**), které další cíle je používají také (**Používá také**) a **Upravit**, které otevře okno přihlášení. Viz [Přihlašovací údaje](#sign-ins).
- **rclone.conf**: sekce konfigurace rclone tohoto cíle, pro cíl, který běží přes rclone.
- **Ochrana proti smazání** s **Ochrana proti smazání (append-only)**, **Přijmout bez ochrany proti smazání**, **Otestovat ochranu proti smazání hned** a **Kdo čistí**, což říká, zda BombVault kopii a vlastní složky tady čistí, podle karty **Jak dlouho uchovávat** tohoto cíle nebo vlastních hodnot sekce pod **Pravidla podle sekcí**, nebo to nechává na druhé straně. Místní složka tuto kartu ukazuje, jen když má ochranu, kterou jde vypnout. Viz [Ochrana proti smazání](#delete-protection-append-only).
- **Kopírovat**: **Kdy kopírovat** a **Limit nahrávání (KiB/s)** a **Limit stahování (KiB/s)**. Cíl, který drží jen vlastní složky a žádné kopie, ukazuje limity jako kartu **Šířka pásma**.
- **Jak dlouho uchovávat**: **Krátké**, **Vyvážené**, **Dlouhé** nebo **Vlastní hodnoty**, s časovou osou, které zálohy pravidlo drží. Viz [Jak dlouho uchovávat](#keep).
- **Už pod tímto cílem**: sekce, jejichž ručně napsaná adresa leží ve složce tohoto cíle. Převzetí jedné nechá repozitář, kde je. Přihlášení, třída úložiště a ochrana proti smazání pak pocházejí z cíle. BombVault napřed zkontroluje, že přihlášení cíle repozitář otevře, a odmítne dát repozitář append-only pod cíl, který append-only není.
- **Pravidla podle sekcí**: vlastní repozitář, uchovávání nebo komprese jedné sekce v tomto cíli. **Změnit pravidla** je otevře a **Vrátit na hodnoty cíle** vrátí sekci zpět. Přihlášení, šířka pásma a kdy se kopíruje vždy pocházejí z cíle.
- Nádoba, která ukazuje, jak je cíl plný, se značkou, kde je plný, a kolik týdnů zbývá při růstu posledních týdnů. Cíl, který nehlásí velikost, říká **Velikost neznámá**.

**Přesunout** a **Odebrat cíl** stojí mezi akcemi stránky, popsané v [Přesun a odebrání](#move-remove).

Vzdálený cíl, který držel zálohy a najednou už žádný repozitář nedrží, například rest-server, jehož disk se nepřipojil, nedostane sám nový, prázdný. Jeho stránka to řekne, jakmile záloha nebo kopie zjistí, že repozitář zmizel. Připoj disk znovu. Pokud jsi starý repozitář vyprázdnil schválně, **Založit tu nový repozitář** začne od začátku, pro všechny sekce cíle a jeho repozitáře najednou. Tato pojistka je pro cíl, který zůstal na místě. Cíl, který přidáš, nebo sekce, kterou u něj zaškrtneš, začíná načisto: když na té adrese žádný repozitář není, první kopie ho vytvoří, i tam, kde ho měl dříve odebraný cíl.

Dvě odmítnutí jmenují, co napravit, v běhu kopie i na stránce cíle. **Tento bucket na serveru neexistuje** přijde, jakmile to server S3 řekne, a BombVault tam nic nevytvoří, takže špatně napsaný název bucketu zůstane vidět. Server, který vůbec neodpovídá, je jiný případ: restic to zkouší znovu asi 15 minut na každé volání, pro případ, že se vrátí. **Tady už je repozitář restic a klíč této instance ho neotevře** znamená, že zálohy tam patří k jinému `APP_KEY`.

Přijímací server, který provozuje jiná instance tvé skupiny, se mezi poskytovateli ukáže jako **Z tvé skupiny**. Viz [Přijímací server](#receiving-server).

BombVault své cíle kontroluje při každém startu. Napraví, co nepotřebuje tvé rozhodnutí, například odkaz na kopii nebo repozitář, který zmizel, kopii na vlastní složku sekce nebo přímý repozitář, jehož kopie zmizela, a zapíše do logu řádek o každé opravě. Co nemá bezpečnou opravu, se jen zaloguje.

![Stejná stránka v telefonu.](assets/screenshots/storage/location-page-phone-dark.png#only-dark)
![Stejná stránka v telefonu.](assets/screenshots/storage/location-page-phone-light.png#only-light)

### Jak dlouho uchovávat {#keep}

Jak dlouho se zálohy uchovávají, se nastavuje pro každý cíl v jeho kartě **Jak dlouho uchovávat**. Pravidlo se vztahuje na kopie v tom cíli a na vlastní složky, které tam leží, včetně cíle **Místní**. Sekce si může ponechat vlastní hodnoty v jednom cíli pod **Pravidla podle sekcí** a ty mají pro tu sekci přednost před kartou cíle.

Pravidlo drží jednu zálohu na den, týden, měsíc a rok, tak daleko do minulosti, jak nastavíš, a zbytek po každé záloze nebo kopii vyčistí:

| Předvolba | Poslední zálohy | Dny | Týdny | Měsíce | Roky |
|---|---|---|---|---|---|
| **Krátké** | 0 | 7 | 4 | 3 | 0 |
| **Vyvážené** | 0 | 7 | 4 | 6 | 2 |
| **Dlouhé** | 0 | 14 | 8 | 12 | 3 |

**Vlastní hodnoty** nastavíš těch pět počtů sám. 0 daný počet vypne a pravidlo se všemi počty na 0 drží všechno. Tečky v kartě ukazují, které ze 40 ukázkových záloh pravidlo drží, nejnovější úplně vpravo. Ukázka je pro každý cíl stejná, protože obrázek se kreslí dřív, než BombVault přečte tamní zálohy. Počítá zpět od dneška, takže počet teček, které zůstanou, se mění s datem.

Složka na tomto hostiteli a ručně napsaná vzdálená složka mají také vlastní kartu, která pokrývá vlastní složky v té složce a žádné jiné. Dokud ji neuložíš, taková složka stárne podle pravidla, které pro ni BombVault už měl.

**Zeptat se před uchováním méně**, ve výchozím stavu zapnuté, se zeptá před uložením změny, která drží méně záloh, a řekne, kolik ze 40 ukázkových záloh nové pravidlo odstraní. Platí pro každý cíl. **Vrátit na hodnoty cíle** pod **Pravidla podle sekcí** klade stejnou otázku.

Cíl s ochranou proti smazání se z tohoto stroje nikdy nečistí. Jeho druhá strana drží nebo zahazuje staré zálohy podle svých vlastních pravidel. Dokud má položka otevřenou kritickou anomálii o ztracených datech, uchovávání drží staré zálohy té položky, dokud ji nepotvrdíš. Viz [Anomálie](anomalies.md).

Záloha, která obsahuje méně, než by měla, nikdy nevytlačí ty před sebou. Je to záloha, která nepřečte žádné soubory tam, kde dřívější nějaké měly, které chybí vybraná složka, nebo která některé soubory nemohla přečíst. Její běh skončí varováním, které jmenuje, co chybí, a říká, že starší zálohy zůstaly zachované, stejnou zprávu dostaneš na svých kanálech oznámení a žádné čištění, ve vlastní složce položky ani u jejích kopií, se starých záloh té položky nedotkne, dokud neprojde úplná záloha. První záloha a vybraná složka, která byla vždy prázdná, se počítají jako úplné.

Chceš-li vidět, co příští běh opravdu smaže, otevři **Náhled** v **Nastavení, Cíle**. Vypíše **Co by odstranilo příští spuštění** pro vlastní složku sekce a pro každou z jejích kopií, podle repozitáře a podle položky, se jmenovanými body obnovy. Nezamyká nic a nic nemění.

![Jak dlouho uchovávat s vybraným Vyvážené: tečky ukazují, které ze 40 ukázkových záloh zůstanou.](assets/screenshots/storage/keep-dark.png#only-dark)
![Jak dlouho uchovávat s vybraným Vyvážené: tečky ukazují, které ze 40 ukázkových záloh zůstanou.](assets/screenshots/storage/keep-light.png#only-light)

#### Příklad: co drží Vyvážené {#example-retention}

Kontejner se zálohuje každou noc ve 03:00 od 1. října 2024, takže ve středu 4. listopadu 2026 má 765 záloh. Jeho cíl je nastavený na **Vyvážené**: 7 dní, 4 týdny, 6 měsíců a 2 roky.

Každý počet drží nejnovější zálohu svého dne, týdne, měsíce nebo roku a záloha, kterou vybere více počtů, se drží jednou. Týdny jdou od pondělí do neděle:

| Pravidlo | Zálohy, které vybere | Nové, které přidá |
|---|---|---|
| 7 dní | 4. 11., 3. 11., 2. 11., 1. 11., 31. 10., 30. 10., 29. 10. | všech 7 |
| 4 týdny | 4. 11., 1. 11., 25. 10., 18. 10. | 25. 10., 18. 10. |
| 6 měsíců | 4. 11., 31. 10., 30. 9., 31. 8., 31. 7., 30. 6. | 30. 9., 31. 8., 31. 7., 30. 6. |
| 2 roky | 4. 11., 31. 12. 2025 | 31. 12. 2025 |

Zbývá tak 14 záloh ze 765. Nejstarší je z 31. prosince 2025, takže se dnes můžeš vrátit o něco víc než deset měsíců. Díky deduplikaci zabírají jen o málo víc místa než jedna úplná záloha plus to, co se mezitím změnilo.

**Roky** stojí za druhý pohled. Počítají kalendářní roky, ve kterých je záloha, počínaje tímto, a nejnovější záloha tohoto roku je dnešní. Takže první rok k ostatním počtům nic nepřidá a druhý přidá poslední zálohu předchozího roku. Proto se dosah pravidla během roku mění. Stejné zálohy pod každou předvolbou:

| Předvolba | Držených záloh dnes | Nejstarší záloha dnes | Dosah během roku |
|---|---|---|---|
| **Krátké** | 10 | 30. září 2026 | 1 až 2 měsíce |
| **Vyvážené** | 14 | 31. prosince 2025 | zhruba 4 měsíce brzy na jaře, celý rok 31. prosince |
| **Dlouhé** | 30 | 31. prosince 2024 | 1 rok 1. ledna, 2 roky 31. prosince |

Přechod z **Vyvážené** na **Krátké** se nejdřív zeptá, kvůli **Zeptat se před uchováním méně**. Otázka říká, kolik ze 40 záloh na obrázku nové pravidlo odstraní. To číslo pochází z ukázky, takže se mění s datem a nepočítá tvoje vlastní zálohy. Ze záloh v tomto příkladu by odešly 31. srpen, 31. červenec, 30. červen a 31. prosinec 2025 by odešly. **Přesto uložit** pravidlo použije a příští čištění je z toho cíle smaže natrvalo.

Chceš-li kontejnery držet v tomto cíli déle než zbytek, stiskni **Změnit pravidla** pod **Pravidla podle sekcí**, vyber **Kontejnery** a nastav **Vlastní hodnoty** v **Pravidla pro Kontejnery**, například 14 dní, 8 týdnů, 12 měsíců a 2 roky. Ostatní sekce se dál řídí kartou cíle a **Vrátit na hodnoty cíle** vrátí Kontejnery zpět.

### Přihlašovací údaje {#sign-ins}

Přihlašovací údaje jsou klíč nebo heslo, které BombVault používá pro službu S3 nebo rest-server. Jsou to jeden seznam, karta **Přihlašovací údaje** v **Nastavení, Cíle**, a každý řádek říká, které cíle je používají (**Používá**).

- **Uložené při přidání cíle.** Přidat cíl uloží klíče, které zadáš, jako přihlašovací údaje. Další cíl u stejné služby je může vybrat pod **Uložené přihlašovací údaje** místo **Nové klíče** a pozdější změna se uplatní u každého cíle, který je používá.
- **Výchozí.** Hvězdička označuje výchozí přihlašovací údaje. Používají se všude, kde nejsou vybrány žádné, například u ručně napsané adresy nebo obnovy na čerstvé instalaci. Zdroj stahování si je nepůjčuje. **Nastavit jako výchozí** dá hvězdičku jiným.
- **Upravit a Otestovat.** **Upravit**, tady nebo v kartě **Přihlašovací údaje** cíle, otevře jedno okno pro přihlašovací údaje. **Otestovat** se s tím, co jsi zadal, přihlásí ke každému cíli, který je používá, dřív než se cokoli uloží. **Uložit** je uloží.
- **Odebrání.** Přihlašovací údaje lze odebrat, jen když je nepoužívá žádný cíl, zdroj stahování ani přijatý repozitář a nepotřebují je žádné staré zálohy, a výchozí odejdou až jako poslední. Staré zálohy je potřebují, když se vlastní složka odstěhovala z místa, které se jimi přihlašuje, a nechala je tam. BombVault pak řekne, o které místo jde.
- **Změna připojení cíle** nikdy nepřepíše přihlašovací údaje, které sdílí jiná místa. Cíl místo toho dostane vlastní a okno předem řekne, že jeho přihlašovací údaje jsou sdílené. Chceš-li změnit sdílené všude, uprav je v kartě **Přihlašovací údaje**.

![Karta Přihlašovací údaje: každé uložené přihlášení s tím, co ho používá, a hvězdičkou u výchozího.](assets/screenshots/storage/signins-dark.png#only-dark)
![Karta Přihlašovací údaje: každé uložené přihlášení s tím, co ho používá, a hvězdičkou u výchozího.](assets/screenshots/storage/signins-light.png#only-light)

Složka na tomto serveru žádné přihlášení nepotřebuje. SMB, WebDAV, SFTP, Storage Box a cloudové disky drží své v konfiguraci rclone, kterou ukazuje karta **rclone.conf** cíle, takže se v tomto seznamu neobjevují.

### SMB, WebDAV a další remoty rclone {#smb-webdav}

Sdílená složka Windows nebo Samba a server WebDAV (Nextcloud, ownCloud, SharePoint nebo jiný) jsou dlaždice poskytovatelů v **Přidat cíl**, pod **NAS nebo server**. Vyplň hostitele a sdílenou složku (SMB) nebo adresu a typ serveru (WebDAV), uživatele a heslo a BombVault napíše sekci rclone za tebe. rclone heslo před uložením zakryje.

Pro službu, kterou žádná dlaždice nepokrývá, bere dlaždice **Rclone** pod **Vlastní konfigurace rclone** remoty vložené z rclone.conf. Přidají se k remotům, které BombVault už má, a nikdy je nenahradí, a ukládají se zašifrované. Chceš-li na jeden z nich zálohovat sekci, zadej `rclone:<remote>:<složka>` jako repozitář sekce mimo lokalitu, například `rclone:nas:backups/bombvault`. Sdílená složka nebo bucket je první segment cesty, ne součást jména.

Tohle je lepší cesta než připojení sdílené složky na Unraidu: restic nedoporučuje držet repozitář na připojené sdílené složce CIFS a tady se nic nepřipojuje. NFS dlaždici nemá, protože restic ani rclone nemá backend NFS. Pro NFS připoj export na hostiteli a použij ho jako místní složku.

### Přesun a odebrání {#move-remove}

- **Přesunout** vezme kopii jedné sekce do jiného cíle, s celou její historií. Všechno zůstává na starém místě. BombVault tam nic nemaže. Přesun běží na pozadí a může trvat dlouho: asi 9 minut na 100 GB mezi místními disky, hodiny po pomalé lince. Přehled ho za běhu ukazuje.
- **Odebrat cíl** nabízí dvě cesty. **Odebrat cíl, zálohy ponechat** přestane tam zapisovat a nechá zálohy na místě. Později je můžeš přečíst pod **Obnova, Ze staré složky se zálohami**. **Odebrat cíl a smazat tamní zálohy** nejdřív přečte, co je tam uloženo, a vypíše to: které zálohy odejdou, které z nich nejsou nikde jinde a které repozitáře zůstanou, protože ochrana proti smazání odmítá, položky na ně stále zálohují rovnou, jiný cíl nebo sekce tam drží své kopie, nebo se nedaly přečíst. Položky, které by přišly o jedinou zálohu, jsou vypsány zvlášť a zůstanou, pokud nezapneš **Smazat i tyto**. Potvrdíš zadáním jména cíle a mazání běží na pozadí.
- Funguje to pro každý druh cíle: ten, který používá více sekcí, cíl nastavený pro jednu sekci, repozitář, který jsi založil, i složku, do které sekce zapisuje své vlastní zálohy. Položky, které zálohují do odebraného repozitáře, se znovu řídí svou sekcí, stejně tak výchozí nastavení sekce, které tam ukazovalo. Složka sekce nemůže prostě zmizet, protože každá sekce nějakou potřebuje. Okno se zeptá na složku, kam sekce přejdou, a každá si pod ní ponechá své jméno složky.
- Mazání záloh v cíli funguje i bez přihlašovacího hesla. Když je heslo nastavené, vyžaduje přihlášení přímo na instanci.

### Příklad: kopie mimo lokalitu na Backblaze B2 {#example-s3}

Kontejnery a autozáloha mají žít i na Backblaze B2, aby je mrtvý server nevzal s sebou. Stejné kroky platí pro Wasabi, Hetzner Object Storage a většinu dalších služeb S3.

1. **V B2** vytvoř soukromý bucket, například `tower-backups`, v regionu poblíž tebe. Pod **Application Keys** vytvoř klíč s právem čtení a zápisu omezený na ten bucket a poznamenej si jeho ID klíče a aplikační klíč. B2 ukáže aplikační klíč jednou. Nastav životní cyklus bucketu tak, aby držel jen poslední verzi, jinak B2 drží vyčištěná data jako skryté verze a účtuje je.
2. **Poskytovatel.** V BombVaultu otevři **Cíle**, stiskni **Přidat cíl** a pod **Služba S3** vyber **Backblaze B2**.
3. **Přihlášení.** Zadej ID klíče jako **ID přístupového klíče** a aplikační klíč jako **Tajný přístupový klíč**. Zadej endpoint bucketu pod **Endpoint**, například `s3.eu-central-003.backblazeb2.com`, a BombVault z něj přečte region. Napiš `tower-backups` do **Bucket**, protože tento klíč otevře jen jeden bucket.
4. **Otestovat připojení.** Mělo by říct **Spojení navázáno** a vypsat složky bucketu. Stiskni **Nová složka** a pojmenuj ji `bombvault`. BombVault do ní dá jednu složku na sekci, například `bombvault/containers`.
5. **Ochrana.** Okno vysvětluje, že B2 umí držet staré verze se zámkem objektů a že BombVault to neumí ověřit. Nech **Ochranu proti smazání (append-only)** vypnutou, pokud bucket opravdu nemazání neodmítá, aby BombVault mohl čistit podle pravidel uchovávání cíle.
6. **Pro které sekce.** Zaškrtni **Kontejnery** a **Autozáloha**. Autozáloha nese tvá nastavení, aby je nová instalace mohla vrátit. Okno říká, kolik položek a, kde je to známo, kolik historie první kopie přenese.
7. Stiskni **Uložit cíl**.

První kopie proběhne po příští záloze. Chceš-li ji spustit hned, otevři **Nastavení, Cíle** a stiskni **Replikovat nyní** v řádku sekce v **Kopie mimo lokalitu**. Hlavička stavu říká **Probíhá kopie mimo lokalitu**. První kopie přenese celou historii, takže 200 GB po lince 100 Mbit/s trvá asi čtyři a půl hodiny. Další kopie posílají jen to, co je nové.

Aby ti zůstala linka volná, otevři stránku cíle a nastav **Limit nahrávání (KiB/s)** v kartě **Kopírovat**, například na `5120`, což se čte jako asi 5,0 MB/s (41,9 Mbit/s). Když tam budeš, zkontroluj na stejné stránce i **Jak dlouho uchovávat**.

Že to fungovalo, uvidíš ve **Stav ochrany** v Přehledu, kde **Mimo lokalitu** přejde na **kopie je aktuální**, a na záložce **Zálohy** kontejneru, kde jeho snapshoty nyní jmenují `tower-backups` jako druhé místo. Potom znovu stiskni **Stáhnout sadu pro obnovu** v **Nastavení, Obnova**, protože sada obsahuje nový cíl a jeho přihlášení.

**Cloudflare R2 funguje stejně,** s objektovým tokenem místo klíče. Vlož adresu S3, kterou ukazuje dashboard R2, i s bucketem, a BombVault vyplní server i bucket.

### Příklad: rest-server, který si provozuješ sám {#example-rest-server}

Rest-server v režimu append-only mazání odmítá, takže kopie na něm přežije, i kdyby někdo převzal tvůj server. Stačí druhá krabice, u kamaráda nebo v garáži. Spárovaný BombVault ti to může nastavit, viz [Přijímací server](#receiving-server). Chceš-li ho spustit ručně na libovolném hostiteli Dockeru:

```sh
docker run -d --name rest-server --restart unless-stopped \
  -p 8000:8000 \
  -v /mnt/user/backups/rest-server:/data \
  -e OPTIONS="--append-only --private-repos" \
  restic/rest-server
docker exec -it rest-server create_user bombvault
```

Poslední řádek se zeptá na heslo a zapíše uživatele do souboru `.htpasswd` v datové složce.

1. V BombVaultu stiskni **Přidat cíl** a pod **NAS nebo server** vyber **rest-server**.
2. Zadej adresu s uživatelem jako složkou, například `http://192.168.2.20:8000/bombvault`, a **Uživatelské jméno** a **Heslo**. Stiskni **Otestovat připojení**.
3. V **Ochrana** zapni **Ochranu proti smazání (append-only)**. BombVault tam pak nikdy nečistí a nechá to na druhé straně.
4. Zaškrtni sekce a stiskni **Uložit cíl**.
5. Jakmile je první kopie tam, otevři stránku cíle a stiskni **Otestovat ochranu proti smazání hned**. BombVault se pokusí smazat zkušební objekt. Karta by měla říct **Naposledy: smazání odmítnuto** a **Ochrana proti smazání** v Přehledu přejde na **ověřeno**. Pokud říká **Naposledy: smazání neodmítnuto**, chybí `--append-only`.

Protože tuto kopii odsud nic nečistí, nastav **Rozpočet růstu (GB, 0 = vypnuto)** v okně sekce, otevřeném přes **Upravit** v jejím řádku v **Kopie mimo lokalitu**, abys to věděl dřív, než se disk zaplní. U samopodepsaného certifikátu viz poznámka o `certs` výše.

## Vlastní složka sekce {#own-folder}

Každá sekce má vlastní složku: repozitář, do kterého se její zálohy zapisují jako první. Kopie do jiných cílů odcházejí odtud. Nová instalace drží vlastní složky na tomto serveru, pod `/mnt/user/bombvault`, a dohromady tvoří cíl **Místní**.

Vlastní složka může ležet i v cíli, který už máš, třeba na NAS, rest-serveru nebo v bucketu S3. Sekce pak zálohuje přímo tam, bez místní kopie mezi tím, a vlastní složka se řídí přihlášením, ochranou proti smazání a limity šířky pásma toho cíle. BombVault ji po každé záloze čistí podle karty **Jak dlouho uchovávat** toho cíle a vlastní hodnoty sekce pod **Pravidla podle sekcí** mají před kartou přednost. Cíl sekci vypisuje ve své kartě **Vlastní složky** a **Cíle podle sekcí** ukazuje cíl jako domov sekce.

### Přesun vlastní složky {#move-own-folder}

1. Otevři cíl, který má vlastní složku držet, a zvol **Použít jako vlastní složku**, nebo **Přidat sekce** v jeho kartě **Vlastní složky**. Chceš-li přesunout více sekcí najednou, zvol **Přesunout vlastní složky** vedle **Vlastní složky zde** v kartě **Připojení** cíle Místní.
2. Vyber sekce pod **Sekce** a místo pod **Přesunout do**. **Jiné místo** bere složku na tomto hostiteli nebo ručně napsanou adresu vzdáleného repozitáře, s přihlášením z tvých uložených přihlašovacích údajů.
3. Přečti si **Co se stane**. U každé sekce říká, kam půjdou nové zálohy, zda se kopie, která už na novém místě je, stane vlastní složkou, a které zálohy se napřed zkopírují.
4. **Zkopírovat už vytvořené zálohy** je ve výchozím stavu zapnuté. Vypnuté, staré zálohy zůstanou, kde jsou, ale položka je už nevypisuje. **Ze staré složky se zálohami** na stránce Obnova je stále přečte s adresou složky a u vzdálené složky s jejím přihlášením.
5. Zvol **Přepnout**. Když karta **Jak dlouho uchovávat** nového místa drží méně než dnešní pravidlo sekce, BombVault se nejdřív zeptá **Uchovat méně?** Tam **Ponechat současné hodnoty pro** tuto sekci **v části Pravidla podle sekcí** přenese dnešní hodnoty s sebou, **Přesto uložit** pokračuje a **Zpět** nechá všechno, jak bylo. Když je potřeba zálohy napřed zkopírovat, BombVault je zkopíruje a pak vlastní složku přepne. Když tam už jsou, před přepnutím je zkontroluje.

Po přesunu jdou nové zálohy do přesunuté vlastní složky, i u sekce, která měla **Místní** vypnuté a zálohovala rovnou na kopii. Kopie, která už na novém místě byla, se stane vlastní složkou a kopírování tam skončí. Kde ta kopie držela některé zálohy déle, než určují pravidla uchovávání sekce, drží sekce to, co drží kterékoli z pravidel, takže žádná nezmizí.

Když přesun nelze provést, okno řekne proč v rámečku pod **Co se stane**. Když překáží jen kopírování starých záloh, vypnutí **Zkopírovat už vytvořené zálohy** nechá vlastní složku přesunout bez nich.

Sekce vždy potřebuje vlastní složku. Odebrání cíle, který ji drží, se zeptá na složku, kam sekce přejde, viz [Přesun a odebrání](#move-remove). Vlastní složka, kolem které nové zálohy jdou, protože sekce zálohuje rovnou na kopii, je označena **(vypnuto)** tam, kde cíl vypisuje, kdo ho používá.

![Okno, které přesouvá vlastní složku, s vyplněným Co se stane pro jednu sekci.](assets/screenshots/storage/move-own-folder-dark.png#only-dark)
![Okno, které přesouvá vlastní složku, s vyplněným Co se stane pro jednu sekci.](assets/screenshots/storage/move-own-folder-light.png#only-light)

#### Příklad: Kontejnery se stěhují na NAS {#example-own-folder}

Pool cache se plní a ty máš cíl jménem `nas`, sdílenou složku SMB na druhé krabici. Sekce Kontejnery má 61 záloh ve své vlastní složce na tomto serveru a kopíruje na Backblaze B2.

1. Otevři **Cíle**, pak `nas` a stiskni **Použít jako vlastní složku**.
2. Zaškrtni **Kontejnery** pod **Sekce**.
3. Přečti si **Co se stane**. Říká, že nové zálohy Kontejnerů jdou do složky na `nas`, že 61 záloh v Místní se tam napřed zkopíruje, že kopie dál jdou na Backblaze B2 a že ochrana, limity šířky pásma a poplach růstu pocházejí z `nas`.
4. Nech **Zkopírovat už vytvořené zálohy** zapnuté a stiskni **Přepnout**.

BombVault zkopíruje zálohy na pozadí a pak vlastní složku přepne. Stránka `nas` vypíše Kontejnery pod **Vlastní složky**, položka, která se řídí svou sekcí, jmenuje `nas` jako svůj domov a příští záloha tam přistane, s kopií na B2 odcházející z `nas`. Od této chvíle složku čistí karta **Jak dlouho uchovávat** cíle `nas`. Stará složka na tomto serveru zůstává, kde je, a BombVault tam nic nemaže.

### Ruční zadání vzdálené adresy {#remote-primary-repositories}

Karta **Připojení** cíle Místní bere pro vlastní složku sekce pořád i vzdálenou adresu: **Změnit** vedle sekce otevře okno s přepínačem **Místní / Vzdálené**. **Vzdálené** bere adresu resticu jako `s3:...`, `rest:http://host:8000/repo`, `sftp:user@host:/repo` nebo `rclone:remote:bucket/path` a **Nastavení zabezpečení vzdáleného primárního repozitáře** přidává test připojení, limity nahrávání a stahování, ochranu append-only se stejným tamper testem, jaký dostane cíl mimo lokalitu, a poplach rozpočtu růstu.

Taková adresa se přihlašuje výchozími přihlašovacími údaji. Když je to přesně ta složka, kterou cíl, který máš, pro sekci drží, řídí se tím cílem sama, včetně karty cíle a jeho přepínače **Dostupné**, pokud se nepřihlašuje jinak nebo není append-only tam, kde cíl append-only není. Pravidlo, které složku do té doby čistilo, zůstane jako vlastní pravidlo sekce pod **Pravidla podle sekcí**. Když leží v jiné složce toho cíle, stránka toho cíle vypíše sekci pod **Už pod tímto cílem** a převzetí jí dá přihlášení, třídu úložiště a ochranu proti smazání cíle. Přesun vlastní složky do cíle pomocí **Použít jako vlastní složku** je jednodušší cesta, protože okno zkontroluje už vytvořené zálohy dřív, než se cokoli změní.

## Umístění podle položky {#placement}

Karta **Plán a cíl** na záložce Nastavení položky má řadu tlačítek: **Místní** a jedno tlačítko za každý cíl sekce mimo lokalitu. Rozsvícená tlačítka dostávají zálohy položky.

- S rozsvíceným **Místní** se položka zapisuje do repozitáře uvedeného pod **Uloženo na** a kopíruje do každého dalšího rozsvíceného cíle. Zhasni cíl a nedostane z této položky nic nového. Samotné Místní nekopíruje nikam, což se hodí pro data, která už mají druhou kopii, například sdílenou složku, která žije na NAS.
- Se zhasnutým **Místní** se položka zapisuje rovnou do přímého repozitáře prvního rozsvíceného cíle a odtud se kopíruje do ostatních rozsvícených. Poprvé ten přímý repozitář vytvoří dialog.
- Jedno tlačítko zůstává rozsvícené, protože záloha potřebuje kam jít. Chceš-li položku ze záloh vynechat, použij **Nezálohovat**.

BombVault nikdy nepřesouvá zálohy mezi repozitáři. Položka, která se řídí svou sekcí, jde s sekcí, když se změní její výchozí nastavení: její příští záloha skončí na novém místě a zálohy, které má, zůstanou, kde jsou. Cíl vybraný na samotné položce zůstává pevný od její první zálohy. Karta **Repozitář** na záložce Nastavení ho jmenuje. Kopie se mohou měnit kdykoli. Cíl, který položka už nedostává, si ponechá kopie, které má, a při příštím běhu sekce mimo lokalitu je ořeže na své uchovávání. Z cílů append-only nelze mazat.

Místo, které položka opustila, si ponechá její zálohy. Na záložce **Zálohy** položky a v **Obnova** stojí jako **Jen starší zálohy**, obnova z něj stále může číst a čištění tam tyto zálohy nechá na pokoji.

**Převzít výchozí** vrátí položku s vlastní volbou ke své sekci. Napřed se zeptá, řekne, kam pak půjdou nové zálohy, a jmenuje, co každý cíl přestane dostávat a co by příští běh nahrál.

Pod řadou karta říká, kam položka jde a co tam opravdu je: kolik míst ji drží, kdy byl každý cíl naposledy viděn a zda je splněno 3-2-1. Místo je server s původními daty, každý cíl mimo lokalitu a každý repozitář označený **Mimo areál**. BombVault kontroluje kopie a místa. Část „dvě média“ z 3-2-1 nekontroluje.

### Výchozí nastavení podle sekcí {#placement-defaults}

Karta **Cíle podle sekcí** na stránce Cíle kreslí výchozí nastavení každé sekce jako tok 3-2-1. Každá linka ze sekce do cíle je přepínač, včetně **Místní**, pro každou sekci, i flashku, datové sady ZFS a autozálohu. Klik na linku nebo tečku ji přepne hned a čárkované linky jsou vypnuté. Jen vypnutí **Místní** sekce se nejdřív zeptá, ve stejném dialogu jako karta výchozího nastavení, protože sekce pak zálohuje rovnou na kopii. **Zrušit** nechá všechno, jak bylo. **Graf / Přepínače** ukazuje totéž jako řady tlačítek. Přepínač rozhoduje, kam jdou nové zálohy. Položky bez vlastní volby a složky projektů zásobníků Compose se řídí hned, takže jejich příští záloha jde na nová místa. Už uložené zálohy zůstávají, kde jsou, a lze z nich dál obnovovat. Flash, datové sady ZFS a autozáloha zálohují do své místní složky a kopírují mimo lokalitu z té zálohy. Jejich linku na **Místní** lze také vypnout. Jejich zálohy pak jdou rovnou do jejich cílů mimo lokalitu a položka ZFS s replikou a bez jakéhokoli cíle posílá jen svou repliku.

S vypnutým **Místní** a zapnutými jen cíli mimo lokalitu sekce zálohuje rovnou na ně a BombVault si repozitář tam vybere sám. Kontejnery, VM a sady složek jdou do přímého repozitáře prvního zapnutého cíle, popsaného níže, a odtud se kopírují do ostatních. Flash, datové sady ZFS a autozáloha zapisují každou zálohu do každého zapnutého cíle a každý cíl je drží podle svého uchovávání.

Sekce se všemi linkami vypnutými říká **Nezálohováno**. Její položky nedostávají žádné zálohy a počítají se jako nechráněné.

Položky, které si na své stránce zvolily vlastní cíle, si je ponechají. Tok říká, kolik jich v sekci je, například **2 položky se liší**, a vede k nim: na záložku Nastavení položky, když je jedna, na seznam s tím, kam která zálohuje, když jich je několik. **Převzít výchozí** tam položku vrátí.

Nový cíl mimo lokalitu dostane každou položku, která není nastavená na Místní. Okno, které ho přidává, říká, kolik položek a, kde je to známo, kolik historie to je.

### Přímé repozitáře {#direct-repositories}

Přímý repozitář je místo, kam položky zálohují rovnou na cíl mimo lokalitu. BombVault ho umístí vedle složky cíle s přidaným `-direct` ke jménu, nebo, když cíl sedí v kořeni svého bucketu, remotu nebo uživatele rest-serveru, do složky uvnitř pojmenované podle sekce, například `containers-direct`. V toku na stránce Cíle se vytvoří bez ptaní. Vypnutí Místní u jedné položky tak, že domovem se stane cíl bez přímého repozitáře, otevře dialog s navrženým místem, například `s3:https://s3.eu-central-003.backblazeb2.com/bucket/containers-direct`, a testem připojení, který nic nevytváří. **Vytvořit a použít** repozitář vytvoří a nasměruje na něj položku. Přímý repozitář nemá žádná vlastní nastavení. Řídí se přepínačem, přihlášením, třídou úložiště, limity, nastavením append-only a uchováváním svého cíle a mění se s nimi. Stránka cíle ho vypisuje pod **Zálohuje se rovnou sem**, s každou sekcí, která na něj zálohuje rovnou, a kolika položkami. Ten, na který už nic nezálohuje, to tam říká a lze ho odebrat, zálohy na jeho místě zůstávají. Když ho nový klíč cíle neotevře, přímý repozitář si ponechá klíč, který má, a uložení to řekne. Položka na přímém repozitáři se z něj kopíruje do ostatních rozsvícených cílů, nikdy ne do cíle, kterému repozitář patří. Jeho snapshoty nesou štítek `bv:direct` a každé další čištění uchovávání je drží, takže přímý repozitář, který ztratil vazbu na svůj cíl, nikdy nestárne podle místních pravidel. K B2 se dostaneš přes jeho endpoint S3, s ID klíče a aplikačním klíčem zadanými jako klíče S3. Klíč omezený na vlastní složku cíle nedosáhne na složku vedle, proto klíč omez raději na složku nad ní.

### Mimo areál {#off-the-premises}

Cíl nebo repozitář lze označit **Mimo areál**. Vzdálené začínají označené. Vypni to u rest-serveru ve stejné budově. Značka jen počítá místa a 3-2-1 na kartách. Žádnou kopii nemění.

Replika ZFS je kopie mimo areál, ale ne záloha s historií. Drží jen snapshoty, které nastavíš, a staré verze a jednotlivé soubory se pořád berou ze záloh. Jde z jednoho serveru ZFS přímo na druhý, přes SSH na server ZFS nebo na přijímací koncový bod spárované instance. Skupina párování nese jen žádost. Viz [Replika](zfs-datasets.md#replica).

### Po opětovném sestavení {#after-a-rebuild}

Volby kopií žijí ve vlastním nastavení BombVaultu. Po opětovném sestavení přes **Objevit zálohy** nebo Nouzového průvodce bez obnovených nastavení jsou pryč a kopírování všeho by poslalo položky, které jsi vynechal, znovu na B2. Totéž platí, když sekce, která z této databáze nikdy nekopírovala mimo lokalitu, najde vlastní zálohy v cíli nebo starší zálohy ve své vlastní složce, například poté, co byl cíl přidán přes zálohy dřívější instalace se stejným `APP_KEY`. Kopie mimo lokalitu té sekce pak čekají na tvé rozhodnutí.

**Vyžaduje pozornost** v přehledu a stránka každého cíle, do kterého sekce kopíruje, se ptají, zda jsou zálohy vlastní zálohy tohoto serveru. **Převzít pro tento server** potvrdí výchozí nastavení sekce a její kopie poběží znovu od příštího průchodu. **Nejdřív zkontrolovat pravidla** vede na řádek sekce v **Cíle podle sekcí**, kde vidíš, kam která položka jde, a ztracené pravidlo můžeš znovu nastavit na stránce položky. Kopírování tam dál čeká, dokud nestiskneš **Převzít pro tento server**, které nejdřív ukáže, co příští běh zkopíruje, a jména v zálohách, která nemají položku a můžeš je vynechat. Nejnovější záloha každé čekající položky to řekne ve svém běhu, s odkazem na rozhodnutí, a když jsou oznámení zapnutá, odejde oznámení. Import souboru s nastavením vrátí pravidla a výchozí hodnoty, ale za tebe nerozhodne.

![Vyžaduje pozornost po opětovném sestavení: sady složek našly zálohy z doby před touto databází a jejich kopie mimo lokalitu čekají na rozhodnutí.](assets/screenshots/offsite/copy-decision-dark.png#only-dark)
![Vyžaduje pozornost po opětovném sestavení: sady složek našly zálohy z doby před touto databází a jejich kopie mimo lokalitu čekají na rozhodnutí.](assets/screenshots/offsite/copy-decision-light.png#only-light)

## Ochrana proti smazání (append-only) {#delete-protection-append-only}

Zapni ochranu proti smazání pro cíl mimo lokalitu, aby ransomware nebo napadený hostitel nemohl smazat ani přepsat tvé zálohy. Druhá strana (`restic/rest-server` běžící v režimu `--append-only`) ji **vynucuje**. BombVault ji vždy jen **ověřuje** a nikdy neukáže zeleně jen na základě tvrzení v konfiguraci. Složka na tomto serveru mazání odmítnout nemůže, protože kdo ovládá BombVault, ovládá i tu složku, takže místní cíle přepínač nenabízejí.

Přepínač patří cíli: sekce, která tam kopíruje, ho přebírá od cíle, takže ho zapni na stránce cíle pod **Cíle**. Plánovaná **Kontrola ochrany proti smazání** běží jen tehdy, když sekce kopíruje do cíle se zapnutým přepínačem. Druhá strana musí být spuštěna s `--append-only`. U image `restic/rest-server` se to na TrueNAS jako na kterémkoli jiném hostiteli Dockeru zadává do proměnné `OPTIONS`.

Některá místa mazání odmítnout neumějí vůbec, třeba cloudový disk jako OneDrive. Pokud tam kopie držíš schválně, zapni na stránce cíle **Přijmout bez ochrany proti smazání**. Přehled pak ukáže chybějící ochranu proti smazání u sekce, která kopíruje jen do přijatých cílů, jako přijaté riziko, šedý kruh místo oranžové tečky, a barvu sekce určují její ostatní kontroly. Nic jiného se nemění: zálohy tam lze pořád smazat, BombVault je pořád čistí podle pravidel uchovávání cíle a přepínač zmizí, jakmile je ochrana proti smazání zapnutá. Kopie, která se přesune jinam, přijetí ztratí. Zapnutí ochrany proti smazání ho také zruší.

Nejrychlejší cesta ke kopii append-only je spárovaný BombVault s [přijímacím serverem](#receiving-server): rest-server za tebe spustí v režimu append-only.

Ve [sdíleném repozitáři](#shared-repository) to platí pro členy, ne pro jeho vlastníka. Vlastník dosáhne na složku přímo a zapomíná a čistí tam.

!!! note "Úspěšné smazání pod `/locks/` se očekává"
    Append-only neznamená, že se nikdy nic nesmí odstranit. restic musí své zámky brát a uvolňovat, takže `/locks/` zůstává z principu zapisovatelné a mazatelné. Snapshoty a data za nimi, což je to, po čem by šel ransomware, odstranit nejde. Pokud druhou stranu zkoušíš sám, smazání, které uspěje pod `/locks/`, je správné chování a ne díra v ochraně.

!!! warning "Repozitáře append-only se z tohoto stroje nikdy nečistí"
    Cíl append-only odsud nikdy nečistí staré snapshoty. Čištění je věcí druhé strany. Nastav pro něj **poplach rozpočtu růstu**, abys byl upozorněn dřív, než velikost repozitáře uteče.

## Test odolnosti proti manipulaci {#tamper-test}

BombVault pravidelně dokazuje záruku append-only tím, že se proti repozitáři mimo lokalitu opravdu pokusí o smazání, namířené na neexistující objekt. **Otestovat ochranu proti smazání hned** na stránce cíle ho spustí na požádání a **Kontrola ochrany proti smazání** v **Nastavení, Ověření, Automatické kontroly** ho spouští podle plánu.

Vypnutí **Kontrola ochrany proti smazání** mezeru neskryje. Cíl s ochranou proti smazání, který nikdy nebyl testován, se pak čte „neprokázáno, jeho kontrola je vypnutá“ a cíl bez ochrany proti smazání se čte „žádná“ tak či tak.

Kopie, která je vypnutá, se netestuje a cíl to říká. Test připojení na ní přesto běží, protože jen čte, takže kopii lze zkontrolovat dřív, než se zapne.

- **Odmítnuto** znamená chráněno.
- **Přijato** znamená nechráněno.
- **Neprůkazný** výsledek (server nedostupný, chyba ověření) uložený verdikt nikdy nepřepne.

Skutečné přepnutí z chráněného na nechráněné spustí jediné upozornění.

## Cvičné obnovy po havárii {#dr-drills}

BombVault nabízí dvě úrovně důkazu, že tvé zálohy jsou opravdu obnovitelné, ne jen přítomné.

- **Cvičení ověření obnovy (místní).** BombVault pravidelně spouští `restic check --read-data-subset` (omezeně, nikdy ne celou obnovu zaplňující disk). Frekvence je v **Nastavení, Ověření, Automatické kontroly** a karta **Zkontrolovat zálohy** na stejné dlaždici ukazuje výsledek.
- **Zkušební obnova (mimo lokalitu).** BombVault obnoví skutečný cíl z repozitáře mimo lokalitu do uzavřeného pískoviště, ověří ho soubor po souboru a bajt po bajtu a pak uklidí. To dokazuje, že se umíš zotavit z mimo lokalitu, ne jen že repozitář odpovídá.

Karta **Stav ochrany** v Přehledu to shrnuje podle sekcí do čtyř kontrol: aktuální záloha na tomto serveru, kopie mimo lokalitu, ochrana proti smazání tam a zkušební obnova odtud. Klik na sekci ukáže podrobnosti a karta zezelená jen na ověřených faktech. Cíl, který jsi přijal bez ochrany proti smazání, se tam počítá jako přijaté riziko, ne jako varování.

## Párování instancí {#pairing}

Několik BombVaultů pracuje společně jako jedna skupina: ukazují si navzájem stav ochrany, přebírají jeden pro druhého role a mohou si navzájem držet klíč. Instance se do skupiny připojí dvanácti slovy.

![Stránka párování se svými třemi kartami kroků.](assets/screenshots/offsite/pairing-dark.png#only-dark)
![Stránka párování se svými třemi kartami kroků.](assets/screenshots/offsite/pairing-light.png#only-light)

Na první instanci otevři **Nastavení, Párování** a stiskni **Vygenerovat frázi**. Tři karty kroků ukazují, jak to jde. Dvanáct slov se objeví v okně s tlačítkem **Kopírovat**. Na každé další instanci otevři totéž místo, stiskni **Zadat frázi** a vlož nebo napiš je, nebo stiskni **Vložit** v tom okně. Slovo, které není na seznamu, se při psaní jmenuje s jeho pozicí a poslední slovo nese kontrolní součet, takže překlep nebo prohozené slovo se zachytí dřív, než se cokoli spáruje. Frázi vygeneruj jen na jedné instanci: dvě instance, které ji obě vygenerují, tvoří dvě oddělené skupiny. Pokud se minutu nikdo neobjeví, karta ti řekne, ať slova zadáš tam, a **Zobrazit frázi** je znovu ukáže. Karta také nabídne, že místo toho zadáš slova druhé instance. Párování funguje i bez přihlašovacího hesla, ale nastav si ho: bez něj může slova přečíst každý, kdo otevře webové rozhraní, a přes skupinu žádat o role, které vydávají heslo resticu instancí v ní. Karta párování to říká, dokud heslo není nastaveno. S heslem se zobrazení fráze znovu zeptá na něj. **Opustit** vyjme instanci ze skupiny i z pevnosti zároveň. Pod kartou fráze stojí členové ve stejných kartách jako na stránce Instance, a dokud ještě nikdo nepřišel, řádek říká, že instance čeká na jinou s touto frází. Obrázek, kdo co pro koho dělá, je na stránce Instance.

Samotná slova nikoho dovnitř nepustí. Nová instance nebo telefon je v kartě **Čeká na schválení**, dokud na instanci, která už ve skupině je, nestiskneš **Schválit**, a tvé rozhodnutí dojde ke každé dosažitelné instanci. Porovnej klíč zobrazený vedle s tím, který nová instance ukazuje na své vlastní stránce párování. **Odmítnout** ji natrvalo zamítne. Když se párují dvě nové instance, každá vede tu druhou jako čekající, takže ji schval na obou. Členové, kteří byli spárovaní už dřív, zůstávají schválení. Se slovy přesto zacházej jako s heslem: kdo je má, může požádat o připojení.

**Jak se členové dosahují.** Každá instance zjistí svou vlastní adresu v síti z tvého prohlížeče ve chvíli, kdy se přihlásíš. V téže síti členové tuto adresu oznamují multicastem a mluví spolu přímo, a kde multicast nemůže přejít sítí kontejnerů, například výchozí bridge Dockeru, instance místo toho prohledá svou podsíť podepsaným voláním, na které odpoví jen člen skupiny, takže párování dokončí za pár vteřin i bez relay. Párování jde vždy přes dvanáct slov a pole pro adresu tu není. Instance v různých sítích jdou přes relay, vybraný v jedné ze čtyř možností v kartě **Přeposílač** na téže stránce. Ke každé podobě patří obrázek:

- **Přeposílač projektu** (výchozí): `parleyport.halleluja.design`, relay, který používá i KnightLoader. Není co nastavovat.
- **Vlastní přeposílač**: kontejner [**ParleyPort**](https://github.com/junkerderprovinz/parleyport) z Unraid Community Apps (také pod **Nastavení, Aplikace**). Zadej jeho adresu na každé instanci, která ho má používat.
- **Instance BombVaultu**: jedna z tvých instancí, která je už dosažitelná zvenčí, se zapnutým **Sloužit jako přeposílač**. Ta instance pak odpovídá na `/relay/connect` na vlastní adrese, za reverzní proxy a certifikátem, které už má, a pouští dovnitř jen tvou skupinu. Zadej její adresu na každé instanci, která ji má používat.
- **Bez přeposílače**: členové se najdou automaticky ve stejné síti a nikde jinde.

**Co přeposílač vidí.** Tlačítko stejného jména vedle čtyř podob to ukáže pro vybranou podobu. Každé volání mezi členy je zapečetěno AES-256-GCM pod klíčem odvozeným z dvanácti slov a ten klíč tvé instance nikdy neopustí. Přeposílač se dozví hash, který seskupuje spojení, pro kterou instanci zpráva je, jak je velká a kdy projde. Přímé volání v místní síti je zapečetěno stejně a navíc podepsáno, takže nic nezávisí na samopodepsaném certifikátu, který instance poskytuje. Volání, která něco vydávají (odpovědi na role, heslo resticu, přihlášení přijímače, úschovy klíčů), jsou navíc podepsána klíčem člena, který je posílá, připnutým při prvním setkání. Viz [Bezpečnostní model](configuration.md#security-model).

**Co putuje přes skupinu.** Stav ochrany na stránce Instance, žádost o okamžitou kontrolu jedné sekce, žádosti o role a jejich odpovědi, nabídky úložiště a to, co přijímač nebo stahovač potřebuje, jakmile je jeho role povolena: umístění repozitářů druhé instance a její heslo resticu. Data záloh nikdy. Ta jdou pořád rovnou k backendům resticu. Ani APP_KEY ne: heslo resticu otevírá repozitáře té instance a nic jiného, ne její uložená tajemství, relace ani kódy úschovy.

**Položky z doby před párováním.** Instance přidané tokenem flotily a přijaté repozitáře a zdroje stahování nastavené pomocí APP_KEY druhé instance po aktualizaci zůstávají a jsou označené **Spárovat znovu**. Fungují dál: při prvním startu BombVault nahradí každý uložený APP_KEY heslem resticu, které z něj odvodí. Spáruj obě instance, pak položku uprav a vyber její instanci. Do té doby takové položky stojí v kartě **Bez instance** na stránce Instance.

Jediné místo, které ještě bere APP_KEY ručně, je [Ze staré složky se zálohami](#restore-from-another-bombvault-repo), pro případ, že druhá instance zmizela a ve skupině odpovědět nemůže.

## Pevnost {#fortress}

Pevnost spojuje BombVault s ostatními aplikacemi z rodiny, třeba ShipLog, TowerTorch, FireSquire a TollRoll, pod jednou frází z dvanácti slov. Je to fráze ze stránky párování: slova, která párují tvoje instance a telefony, přivedou i ostatní aplikace. Zadej slova z BombVault v jiné aplikaci, nebo slova, která ukazuje jiná aplikace, zadej přes **Zadat frázi** na instanci BombVault, která ještě není v žádné skupině. Když se fráze změní, instance a telefony, se kterými je BombVault spárovaný, přejdou s ním bez nového párování, a ta, která zrovna nebyla dostupná, se přidá, jakmile se vrátí. Stará fráze zůstane platná, dokud nepřejde každé zařízení, nejdéle 30 dní. Zařízení, které potom pořád chybí, se ukáže na stránce párování a v aplikaci, takže na něm můžeš zadat frázi, nebo ho odebrat.

- **Název instance**: jediné jméno, pod kterým tuto instanci znají její ostatní instance, její telefony i ostatní aplikace. Když zůstane prázdné, použije se jméno serveru Unraid. ShipLog a TowerTorch podle něj najdou BombVault na vlastním serveru.
- **Ostatní aplikace**: když je zapnuté **Zobrazit ostatní aplikace pevnosti**, stojí pod členy ve stejných kartách, každá se svým logem, pokud ho aplikace zveřejňuje.
- **Oprávnění**: všechno, co smí jedna aplikace dělat u jiné, má vlastní přepínač, například zjistit, kdy naposledy vyšla záloha, vyžádat zálohu kontejneru nebo procházet zálohy. Na začátku jsou všechny zapnuté.
- **Zobrazit frázi** ukáže slova vedle QR kódu, jakmile zadáš heslo. **Změnit** dá pevnosti novou frázi, která dojde ke každé dosažitelné aplikaci a instanci. Stará fráze funguje ještě 30 dní a stránka párování vyjmenuje každé zařízení, ke kterému změna nedošla. **Opustit** vyvede BombVault z pevnosti i z jeho skupiny.

**Co dělají ostatní aplikace.** ShipLog si před aktualizací kontejneru vyžádá jeho zálohu a BombVault mu aktualizaci kontejnerů, které ShipLog spravuje, přenechá, takže jejich přepínač **Aktualizovat po úspěšné záloze** zůstává vypnutý. TowerTorch si vyžádá zálohu před údržbou a FireSquire kontroluje, kdy se naposledy zálohovala flashka. Požadavek počká, dokud běží jiná záloha. TollRoll ukazuje BombVault ve své dlaždici na dashboardu Unraidu. Ostatní aplikace čtou stav BombVaultu na `GET /api/fortress/v1/status` a `GET /api/fortress/v1/log`, a to umí přečíst i API token nebo přihlášený prohlížeč.

**Vlastní záloha aplikace.** Než BombVault zazálohuje kontejner aplikace, která se umí zálohovat sama, stáhne si její vlastní zálohu a uloží ji do repozitáře kontejneru, ale jen když je ten repozitář šifrovaný. Objeví se v seznamu záloh kontejneru se jménem aplikace a odkazem ke stažení, stárne se zálohami kontejneru, putuje s nimi mimo lokalitu a smaže se spolu s nimi. Když je zapnuté **Vracet zálohy aplikacím**, **Vrátit do** ji předá aplikaci zpátky, a tím nahradí to, co aplikace má teď. Ten přepínač je vypnutý, dokud ho nezapneš.

## Instance a role {#roles}

Stránka **Instance** (dlaždice nastavení, kterou lze připnout do postranního panelu) ukazuje skupinu jako síť, kdo co pro koho dělá, kartu této instance a kartu každého dalšího člena, telefony s Androidem a servery ZFS. **Párování** vpravo dole vede na stránku párování. Které části jsou zapnuté, se nastavuje v **Nastavení, Instance, Více serverů**: **Instance**, **Přijímač** a **Stažení**.

![Stránka Instance: skupina jako síť a karta pro každou instanci s dlaždicemi rolí, jedna z nich Neaktivní.](assets/screenshots/offsite/instances-dark.png#only-dark)
![Stránka Instance: skupina jako síť a karta pro každou instanci s dlaždicemi rolí, jedna z nich Neaktivní.](assets/screenshots/offsite/instances-light.png#only-light)

Každá karta ukazuje stav ochrany člena a jeho role jako dlaždice. Role je něco, co jedna instance dělá pro jinou:

- **Přijímač** drží kopie tohoto serveru mimo lokalitu v rest-serveru v režimu append-only. Tento server posílá po každé záloze a nemůže tam nic smazat, ani kdyby ho někdo převzal.
- **Stahovač** stahuje zálohy tohoto serveru podle vlastního plánu. Tento server k tomu nemusí být vzhůru. Instance, která stahuje, potřebuje zapnuté **Stažení**, než může žádost povolit.
- **Server ZFS** přijímá repliky ZFS tohoto serveru. Každá položka ZFS ho vybírá pod **Replika** ve svém nastavení.
- **Strážce klíče** drží zapečetěnou kopii sady pro obnovu tohoto serveru. Viz [Strážce klíče](#key-keeper).

Zapnutí role pošle žádost a nic nezačne, dokud někdo na druhé instanci nestiskne **Povolit**. Žádost se na kartě té instance ukáže jako odznak a její **Podrobnosti** ji vypisují s **Povolit** a **Odmítnout**. Povolenou roli lze tam kdykoli odvolat. Čárkovaná linka v síti je role, která ještě čeká. Instance na BombVaultu 9.x na žádost odpovědět nemůže, takže role je tam aktivní, jakmile ji zapneš, a žádost od instance 9.x čeká na **Povolit** tady.

**Podrobnosti** na kartě otevřou zbytek: přijaté repozitáře a zdroje stahování toho člena, nabídky úložiště, sledování, **Odebrat** a **Zapomenout klíč**.

Pod kartami patří ke [sdílenému repozitáři](#shared-repository) karta **Sdílený repozitář** na krabici s přijímacím serverem a karta **Sdílené repozitáře** na členovi.

Člen může druhému členovi přes skupinu nabídnout vlastní úložiště mimo lokalitu. Druhý správce vidí nabídku v **Podrobnostech** člena a přijme ji nebo odmítne. Přijetí vytvoří uložené přihlašovací údaje a obyčejný cíl, po stejných kontrolách jako u cíle, který zadáš sám. Cíl přijde se zapnutou ochranou proti smazání, protože recept, který nabídka ukazuje, je rest-server append-only, takže **Jak dlouho uchovávat** tam neplatí. Každá nabídka má vlastní přihlášení a vlastní složku, takže když sekci nabídneš znovu, stejnému členovi nebo jinému, kopie prvního partnera zůstanou nedotčené: přidej nový řádek do `.htpasswd` a nech server běžet. Takto putují jen údaje o připojení, nikdy data záloh. **Zkontrolovat nyní** požádá člena, aby ověřil repozitář jedné sekce. Nic na stránce nemůže spustit zálohu, obnovu ani smazání na jiné krabici.

Instance, které jsou vypnuté, a telefony se zavřenou aplikací si ponechají kartu jako **Neaktivní**, šedě, s tím, kdy byly naposledy vidět, a stojí v síti ztlumeně, dokud je neodebereš pomocí **Odebrat**. Hned po startu tohoto serveru taková karta říká **Ověřuje se**, dokud se skupina nevrátí. **Odebrat** na takové kartě funguje, ať jsou zapnuté jakékoli části, a ukončí také otevřené žádosti člena, jeho nečinné sloty příjmu ZFS a čekající žádosti o klíč. Karta **Instance** na stránce párování tyto členy vypisuje také jako **Neaktivní**.

## Přijímač a stahovač {#receiver}

Na krabici, která **přijímá** kopie append-only od jiného BombVaultu, ti přijaté repozitáře dávají nezávislé sledování jen pro čtení na přijímajícím hardwaru, takže tiché selhání na druhém konci nezůstane bez povšimnutí. Zapni **Přijímač** v **Nastavení, Instance**, pak otevři **Podrobnosti** odesílající instance na stránce Instance a přidej přijatý repozitář (jen pro čtení, otevřený heslem resticu odesílající instance, které dostane přes [skupinu párování](#pairing)). Dostaneš:

- **Soupis podle zdroje**, takže přesně vidíš, které kontejnery, VM a sady složek dorazily. Zálohy, které odesílající instance vytvořila dřív, než měla [vlastní host](#backup-host), se počítají ke stejnému zdroji.
- **Naposledy přijato** u každého zdroje, takže víš, jak je každý čerstvý.
- **Nezávislý `restic check`** spuštěný na přijímajícím hardwaru, takže se integrita ověřuje tam, kde data skutečně leží, ne jen na odesílateli.
- **Spínač mrtvého muže:** upozornění, když zdroj přestane posílat v okně, které nastavíš. Kontroluje jednou denně, v 09:15. Jedno upozornění dostaneš také, když repozitář nejde přečíst vůbec, ať plán kontrol říká cokoli, a když celé okno po jeho přidání nic nedorazilo. Repozitář, který přidáš před první kopií odesílatele, je do té doby v pořádku.
- **Upozornění na integritu:** upozornění, když kontrola na přijímací straně selže. Kontrola, která repozitář nemohla otevřít, o integritě nic neříká, takže nezanechá žádný verdikt a zkusí to znovu další den ve stejnou dobu.

Repozitář na jiném serveru potřebuje přihlášení toho serveru: ulož je jako [přihlašovací údaje](#sign-ins) a vyber je v dialogu pod **Přihlašovací údaje**.

Přijímač je přísně jen pro čtení. Do přijatého repozitáře nikdy nezapisuje, takže nikdy nemůže porušit záruku append-only, na kterou se odesílatel spoléhá.

Stahovač funguje opačně: se zapnutým **Stažení** instance čte zálohy jiného člena podle vlastního plánu do vlastního repozitáře pro každý zdroj, takže se nikdy nepočítají jako vlastní zálohy této krabice. Jeho zdroje stojí také v podrobnostech toho člena: umístění repozitáře, jaký druh zálohy drží a jak často stahovat. Heslo resticu přichází přes skupinu, nikdy APP_KEY, a druhá strana nemá co dalšího nastavovat a nemusí být spuštěná. Zdroj se vždy jen čte a každá strana si drží vlastní přihlášení. Stažení běží v den a čas, které říká jeho plán, a nový zdroj na ten čas čeká také. **Stáhnout nyní** nečeká. Stažení zmeškané proto, že byla tato krabice vypnutá, proběhne, jakmile je zase zapnutá. Neúspěšné stažení to zkusí znovu další den ve stejnou dobu, i při týdenním plánu, a neúspěšné stažení se ukáže v protokolu aktivit a pošle jedno upozornění, dokud stažení zase nezačne fungovat. Zdroj `rclone:` se odmítá, protože rclone by na něj sáhl remoty této instance.

Stažená kopie jen roste. BombVault do ní přidává a nečistí ji ani nekontroluje, takže to, co zdroj dávno zapomněl, tu zůstane, dokud složku nesmažeš, a stránka Cíle ji nepočítá. Leží v `pulled/<source id>` uvnitř vlastní složky sekce, a když se ta složka přesune, kopie zůstane, kde je, a další stažení pokračují tam, místo aby kopírovala všechno znovu. Pokud ji smažeš, další stažení začne znovu ve vlastní složce, kterou sekce v tu chvíli má.

### Přijímací server {#receiving-server}

Přijímací krabice může také provozovat rest-server, na který ostatní kopírují. Nastaví se poprvé, když povolíš žádost **Přijímač**, která si ho vyžádá: okno říká, že server potřebuje přijímací server, a ptá se na složku na sdílené složce, s **Nová složka** pro její vytvoření, a na port (8000, pokud ho nepoužívá jiný kontejner). BombVault pak:

1. odmítne, pokud už existuje kontejner jménem `rest-server` nebo port drží jiný kontejner,
2. stáhne `restic/rest-server` a spustí ho přes Docker socket v režimu append-only s privátními repozitáři a souborem s přihlášeními ve složce,
3. zapíše jeho šablonu Unraidu na flashku, aby kontejner zůstal upravitelný v záložce Docker, nebo šablonu nabídne ke stažení, když flashka není dosažitelná,
4. spustí proti němu tamper test a ukáže, zda mazání odmítá.

Instance ve tvé skupině pak server najdou mezi poskytovateli pod **Z tvé skupiny**, pojmenovaný podle přijímací krabice, a role **Přijímač** ho nabídne jako bezpečnou volbu. Každá instance dostane při prvním použití serveru vlastní přihlášení a zapisuje jen do své vlastní složky tam. Se zapnutým **Přijímač** v **Nastavení, Instance** otevře **Přijímací server** na kartě této instance server znovu. Jeho okno vypisuje přihlášení pod **Partneři s přihlášením** a **Odvolat přihlášení** jedno odebere. Co ta instance už zkopírovala, ve složce zůstává. Nastavení také vytvoří jedno **Přihlášení mimo skupinu**, jehož heslo okno ukáže jednou.

Instance, která dosáhne na přijímací krabici jen přes relay, server použít nemůže, protože relay nenese zálohy. Přidej nejdřív na stránce párování adresu přijímací krabice. Když BombVault běží na vlastní IP adrese (třeba na br0), vyplň **Adresa pro partnery**, protože server naslouchá na adrese hostitele.

Přijímací krabice může pro skupinu držet i [sdílený repozitář](#shared-repository).

## Sdílený repozitář {#shared-repository}

Několik serverů jedné skupiny může kopírovat společně do jednoho repozitáře resticu. Data, která mají společná, třeba stejnou sadu složek nebo stejná appdata, se tam uloží jen jednou, takže dohromady potřebují méně místa, než kdyby měl každý svůj repozitář.

Jeden server skupiny je vlastníkem repozitáře. Jen on tam zapomíná staré zálohy a čistí. Ostatní jsou členové: k repozitáři se dostanou přes druhý rest-server v režimu append-only a mohou do něj jen přidávat. Každý server zapisuje pod [vlastním hostem](#backup-host), takže zálohy zůstanou oddělené, i když mají dva servery položku stejného jména.

Co je potřeba:

- [skupina párování](#pairing),
- zapnuté **Šifrovat zálohy**, na vlastníkovi i na každém členovi, protože každý server dostane vlastní klíč resticu,
- [přijímací server](#receiving-server) na vlastníkovi, na který členové dosáhnou. Když BombVault běží na vlastní IP adrese, vyplň nejdřív **Adresa pro partnery** v okně přijímacího serveru.

Server může být vlastníkem nanejvýš jednoho sdíleného repozitáře.

### Nastavení {#shared-set-up}

Jakmile přijímací server běží, má stránka **Instance** jeho krabice pod instancemi kartu **Sdílený repozitář**. Stiskni **Nastavit sdílený repozitář**, vyber **Složka pro sdílený repozitář** na sdílené složce s dostatkem místa, nech **Port** na 8001, pokud ho nepoužívá jiný kontejner, a stiskni **Nastavit a spustit**. BombVault pak:

1. odmítne, pokud už existuje kontejner jménem `rest-server-shared` nebo port drží jiný kontejner,
2. vytvoří repozitář ve složce `shared` uvnitř ní, s heslem tohoto serveru,
3. spustí `restic/rest-server` jako `rest-server-shared`, v režimu append-only se souborem s přihlášeními ve složce,
4. na Unraidu zapíše jeho šablonu na flashku, aby kontejner zůstal upravitelný v záložce Docker,
5. spustí proti němu tamper test. **Zkontrolovat znovu** na kartě ho spustí ještě jednou.

Karta vypisuje **Členové**, každého s jeho klíči a poslední zálohou, žádosti o připojení a u každého člena **Odebrat**.

### Připojení {#shared-join}

Na serveru, který se chce připojit, otevři **Přidat cíl**. Pod **Z tvé skupiny** se vlastník ukáže podruhé, jako **Sdílený repozitář na** a jeho jméno. Když ho vybereš, pošle se vlastníkovi žádost, která se u něj objeví na kartě **Sdílený repozitář** s **Povolit** a **Odmítnout**.

Po **Povolit** vydá vlastník přes skupinu jednorázový klíč a přihlášení ke svému druhému rest-serveru. Člen jím přidá vlastní klíč resticu a vlastník pak jednorázový klíč odstraní. Člen, který to do jednoho dne nedokončí, jednorázový klíč i přihlášení zase ztratí a může požádat znovu. Když se vlastník se svou odpovědí ke členovi nedostal, nabídne karta **Oznámit znovu**.

Vlastník server odmítne, dokud ještě nezapisuje pod vlastním hostem, což znamená, že běží na starším vydání, a také když už pod stejným hostem zapisuje jiný člen. Dva servery mají stejný host, když mají stejný `APP_KEY`.

Jakmile je člen uvnitř, objeví se pod **Cíle** cíl s názvem **Shared with** a jménem vlastníka, se všemi sekcemi vypnutými. Sekce, které tam mají kopírovat, zapni na stránce cíle. Cíl má ochranu proti smazání a žádná vlastní pravidla uchovávání, protože zapomíná tam jen vlastník. Jdou tam jen zálohy pod vlastním hostem člena, takže starší zálohy pod hostem `bombvault` zůstanou, kde jsou.

Vlastník může do repozitáře kopírovat i své vlastní zálohy: **Kopírovat sem i zálohy tohoto serveru** na jeho kartě přidá ten cíl i jemu. Pro vlastníka ta kopie leží na jeho vlastních discích a nic mu nebrání tam mazat, takže to není kopie mimo lokalitu.

### Kdo uklízí a kdy {#shared-upkeep}

Zapomíná a čistí jen vlastník. Jeho nastavení je v části **Sdílený repozitář** okna **Přijímací server**, které se otevře z vlastní karty vlastníka na stránce Instance:

- **Okno pro úklid** určuje, kdy čištění běží, denně nebo týdně. Dokud ho nezměníš, začíná v neděli v 03:00 a **Délka okna v hodinách** je 6.
- **Nevyužité místo, které smí čištění nechat, v %** je 5, dokud to nezměníš. Vyšší hodnota znamená méně přebalování, takže čištění skončí dřív.
- **Data přečtená zpět po čištění, v %** je 5, dokud to nezměníš. Po každém čištění vlastník repozitář zkontroluje a tento podíl dat přečte zpět.
- **Jak dlouho uchovávat** se u každé sekce řídí pravidly vlastníka pro kopie mimo lokalitu a **Nastavit vlastní pravidlo** dá sdílenému repozitáři vlastní. Zálohy každého serveru stárnou samostatně.
- **Uklidit teď** hned zapomene, vyčistí a zkontroluje, jakmile dá vědět členům.

Mimo okno vlastník zapomíná jednou denně a místo se vrátí s dalším čištěním. Před čištěním dá vědět členům a ti do repozitáře nic nekopírují, dokud se okno nezavře. Člen, který ještě kopíruje, dostane až půl hodiny na dokončení, jinak se čištění přesune do dalšího okna. Kopie, která padne do okna, počká, v protokolu aktivit a na kartě člena se ukáže jako odložená a další průchod ji dožene.

Kde si vlastník není jistý, nechá víc. Položka, kterou člen drží kvůli [anomálii](anomalies.md), si nechá všechny své zálohy, člen, který neumí říct, co drží, si tentokrát nechá všechny své zálohy, a zálohy pod hostem, pod kterým nezapisuje žádný server skupiny, se nikdy nezapomenou. Okno ukazuje i místo: kolik repozitář zabírá, kolik by servery potřebovaly každý zvlášť a kolik se tím ušetří. Čísla přicházejí s každým úklidem.

### Co vidí členové {#shared-members}

Na členovi má stránka Instance kartu **Sdílené repozitáře** s každým sdíleným repozitářem, do kterého server kopíruje nebo o připojení k němuž požádal, a se stavem jeho žádosti: **Čeká na souhlas**, **Připojuje se**, **Připojeno**, **Odmítnuto**, **Vypršelo** nebo **Není členem**. U repozitáře, ve kterém je, ukazuje karta **Kdo čistí**, **Příští úklid**, **Společně s** pro ostatní členy, **Poslední kopie sem** a kolik místa repozitář zabírá a šetří.

### Odchod {#shared-leave}

**Odejít** na kartě člena server vyřadí: vlastník odstraní jeho klíč a přihlášení a jeho cíl tam se vypne. **Odebrat** na kartě vlastníka vyřadí člena z druhé strany. Tak či tak zálohy serveru zůstanou v repozitáři a stárnou podle pravidel vlastníka. U vlastníka stojí pod **Servery, které odešly**, kde je **Zapomenout zálohy** všechny zapomene, jakmile napíšeš jméno serveru. Místo vrátí další okno pro úklid.

### Zálohy jiných serverů {#shared-restore}

Každý server najde své vlastní zálohy ve sdíleném repozitáři na záložce **Zálohy** svých položek, jako kteroukoli jinou kopii. Zálohy ostatních členů jsou na stránce Obnova pod [Další servery](restore.md#other-servers).

### Po havárii {#shared-emergency}

[Sada pro obnovu](#recovery-kit) má kapitolu **Shared repositories**. Uvádí každý sdílený repozitář s jeho vlastníkem, sekce, které tam kopírují, ostatní členy s jejich hosty a příkaz `restic snapshots` s hostem tohoto serveru. Sada vlastníka uvádí navíc port, složku a přihlášení jeho rest-serveru.

V Nouzovém průvodci se repozitář ukáže mezi místy jako **Sdílený repozitář na serveru** a jméno vlastníka. Nová instalace, která převezme klíč člena, zapisuje pod stejným hostem, protože host pochází z klíče. Dostane zpět svůj cíl ve sdíleném repozitáři a vypisuje a obnovuje tam jen své vlastní zálohy.

Nová instalace, která převezme klíč vlastníka, se stane vlastníkem repozitáře místo něj. Nechává zálohy v něm stárnout a uklízí ho jako dřív a znovu spustí rest-server na stejné složce. Spáruj ji znovu se skupinou, nastav její přijímací server pod **Instance** a znovu nastav pravidla v okně přijímacího serveru, protože v sadě nejsou. Jakmile skupina dosáhne na členy, nový vlastník jim to oznámí sám. Každý člen to nejdřív ověří: přidá do repozitáře zkušební klíč, který může odstranit jen server u složky repozitáře, a nového vlastníka následuje, jakmile ten klíč zmizí. Pokud nový vlastník odpovídá na jiné adrese, každý člen tam tuto kontrolu provede se svým vlastním klíčem a přihlášením, ověří si, že najde stejný repozitář, a teprve pak přepne své cíle na novou adresu. Do té doby dál kopíruje na starou adresu a jeho karta ukazuje důvod. Klíče, které po takových kontrolách zůstanou, vlastník odstraní při dalším úklidu.

Pokud starý vlastník ještě běží, jeho karta říká, že repozitář teď drží druhý server. **Vzít zpět** z něj udělá znovu vlastníka, pro případ, že byla sada omylem převzata na druhém serveru.

### Omezení {#shared-limits}

- **Každý klíč čte všechno.** Všechny klíče repozitáře resticu otevírají stejný hlavní klíč, takže každý člen může číst zálohy ostatních. Definice kontejnerů a VM zůstávají zapečetěné vlastním klíčem každého serveru, takže kontejnery a VM jiného serveru se vrátí jen jako soubory. Server, který kdysi měl přístup, mohl hlavní klíč přečíst, takže ho jeho odebrání natrvalo nevyřadí. To dokáže jen nový repozitář.
- **Vlastník může mazat.** Žádný člen tam nemůže nic smazat, ale vlastník dosáhne na složku přímo a zapomíná a čistí zálohy všech členů. Chyba nebo převzetí na straně vlastníka zasáhne všechny.
- **Poškození zasáhne každý server.** Poškozený pack chybí každému serveru, jehož zálohy ho používají, takže sdílený repozitář nenahrazuje druhou kopii.
- **Jen na přijímacím serveru.** S3, SFTP a jiná místa nedokážou člena omezit jen na přidávání, takže BombVault nabízí sdílený repozitář jen na přijímacím serveru.
- **Jen jako cíl kopie.** Vlastní složka sekce nemůže ležet ve sdíleném repozitáři, protože každé čištění by zdrželo zálohy všech serverů.
- **Starší historie zůstává na místě.** Jdou tam jen zálohy pod vlastním hostem každého serveru a z jiných repozitářů se nic nepřesouvá.

## Příklad: spárování dvou krabic s Unraidem {#worked-example-two-unraid-boxes-end-to-end}

Tohle je jedno kompletní nastavení výše popsaných částí, protože se skládají snáz, když je jednou uvidíš složené.

Dvě krabice: **TOWER** provozuje kontejnery a posílá zálohy. **VAULT** je přijímá a vynucuje append-only. Dosaď si vlastní jména, adresy a cesty ke sdíleným složkám.

**1. Spáruj obě krabice.** Na TOWER otevři **Nastavení, Párování**, stiskni **Vygenerovat frázi** a zadej dvanáct slov na VAULT pod **Zadat frázi**. Nastav nejdřív na obou přihlašovací heslo.

**2. Na TOWER požádej VAULT, aby byl přijímačem.** Na stránce Instance zapni **Přijímač** na kartě VAULT, vyber sekce a jako místo **Přijímací server VAULT** a pošli žádost.

**3. Na VAULT ji povol.** Stránka Instance na VAULT ukáže žádost na kartě TOWER. Stiskni **Povolit**. Poprvé se VAULT zeptá na složku pro svůj přijímací server, například `user/appdata/rest-server`, spustí rest-server v režimu append-only a spustí proti němu tamper test.

**4. Zpět na TOWER.** Od té chvíle TOWER kopíruje na VAULT po každé záloze a nový cíl se objeví na stránce Cíle na TOWER s ochranou proti smazání.

Co tamper test odpovídá:

| Výsledek | Co se stalo |
| --- | --- |
| **chráněno** | VAULT smazání odmítl. To je jediný stav, který projde. |
| **nechráněno** | VAULT smazání přijal. `--append-only` chybí nebo byl odstraněn. |
| **neprůkazné** | Ani jedno. Obvykle URL není ta, kterou používá sám restic, nebo se změnily přihlašovací údaje. Nic se nezapíše a nespustí se žádné upozornění. |

**5. Sleduj, co dorazí, pokud chceš.** Zapni **Přijímač** v **Nastavení, Instance** na VAULT, otevři **Podrobnosti** TOWER a přidej přijatý repozitář: klikni na jeden z cílů, které dialog vypíše. VAULT dostane heslo resticu TOWER přes skupinu, a protože cíl leží na vlastním přijímacím serveru VAULT, čte složku přímo. Nikdo nepíše klíč ani přihlášení.

**6. Udělej to oboustranné.** Zopakuj kroky opačným směrem: VAULT požádá TOWER, aby byl jeho přijímačem. Každá krabice pak vynucuje append-only pro tu druhou a ani jedna nemůže mazat zálohy té druhé.

Funguje i rest-server, který provozuješ ručně: přidej ho přes **Přidat cíl** jako rest-server, s uživatelem a heslem z jeho `.htpasswd`, a zapni ochranu proti smazání. Nech v jeho volbách `--append-only`. Bez něj je druhá strana zase obyčejná sdílená složka.

!!! warning "Místní cesta je cesta uvnitř kontejneru"
    Kde BombVault žádá o složku na tomto serveru, zadej ji relativně k připojení hostitele, například `user/appdata/rest-server/bombvault-containers`, ne `/mnt/user/appdata/...`. BombVault běží v kontejneru, kde je `/mnt` hostitele připojeno jinde. Absolutní cesta hostitele tam neexistuje. Pokud ji vložíš, BombVault ti řekne, jakou relativní cestu použít místo ní.

## Nouzový průvodce {#emergency}

**Nouzový průvodce** vrátí server, který je pryč nebo byl nastaven znovu. Začíná na stránce **Obnova** pod **Nouze** a z **Obnovit ze zálohy** na uvítací stránce nové instalace. Běží ve čtyřech krocích: **Přístup**, **Nastavení**, **Výběr** a **Obnova**.

1. **Přístup.** Ukaž BombVaultu klíč starých záloh, jedním ze tří způsobů:
    - **Nahrát sadu pro obnovu**: soubor, který jsi stáhl při nastavení. Drží klíč i každý cíl a BombVault z něj doplní zbytek. Sada zapečetěná pomocí age se zeptá na identitu, pro kterou byla zapečetěna. Použije se jednou a neukládá se.
    - **Přes skupinu**: instance tvé skupiny drží klíč jako úschovu. Potřebuješ dvanáct slov skupiny, svůj kód úschovy a někoho u té instance. Viz [Strážce klíče](#key-keeper).
    - **Zadat klíč**: `APP_KEY` starého serveru a kde jsou jeho zálohy.

    BombVault pak vyzkouší klíč na každém cíli a u každého řekne, zda je čitelný, nenalezený, zamčený jiným klíčem, nedosažitelný nebo mu stále chybí přihlášení. Zatím se nic neuložilo a klíč zůstává v paměti omezenou dobu. **Převzít klíč** způsobí, že tento server odteď zapisuje zálohy tímto klíčem. **Vrátit převzetí** se vrátí k vlastnímu klíči, dokud neproběhne první záloha. Server, který už má vlastní zálohy, převzetí odmítne a odkáže na [Ze staré složky se zálohami](#restore-from-another-bombvault-repo). **Uložit přihlašovací údaje** uloží přihlášení, která sada nese pro vzdálené cíle, a zeptá se, než nahradí přihlášení, která tento server už má.
2. **Nastavení.** **Vrátit nastavení** obnoví vlastní nastavení BombVaultu z nejnovější autozálohy: cíle, přihlašovací údaje, plány a pravidla. BombVault se pro jejich použití restartuje, přes Docker socket, takže se živá databáze nastavení nikdy nepřepíše pod otevřeným handlem, a průvodce poté pokračuje sám. Přihlášení tohoto serveru zůstává: dál se přihlašuješ heslem, které používáš teď. Tajemství, která starý server uložený měl a nelze je převzít, jsou vypsána, abys je mohl zadat znovu. Obnova ukončí všechny relace a odstraní passkeys, takže je potom nastav znovu.
3. **Výběr.** BombVault projde tvé zálohy a vypíše kontejnery, VM, sady složek a datové sady ZFS, které najde. Nejnovější snapshot je všude předvybraný. Co tento server ještě má, začíná vypnuté, protože by to obnova nahradila. Sady složek po opětovném sestavení už nevědí, odkud pocházely, takže jdou do složky, kterou vybereš.
4. **Obnova.** Všechno vybrané se vrátí najednou, nechané zastavené, pokud chceš každý kontejner a VM spustit sám, což je po úplné ztrátě nejbezpečnější. Potom stáhni novou sadu pro obnovu: stará patří předchozí instalaci.

Sdílený repozitář ze sady se mezi místy ukáže jako **Sdílený repozitář na serveru** a jméno serveru, který je jeho vlastníkem, a průvodce tam vypíše jen vlastní zálohy tohoto serveru. Viz [Po havárii](#shared-emergency).

![Nouzový průvodce v prvním kroku, se třemi cestami dovnitř.](assets/screenshots/offsite/emergency-dark.png#only-dark)
![Nouzový průvodce v prvním kroku, se třemi cestami dovnitř.](assets/screenshots/offsite/emergency-light.png#only-light)

### Příklad: nový server ze sady pro obnovu {#example-emergency}

Server TOWER zemřel i s disky. Držel 23 kontejnerů a 2 VM a kopíroval je i s autozálohou na Backblaze B2. Máš `bombvault-recovery-kit.md` ze správce hesel a novou krabici s Unraidem. Jeho starý `APP_KEY` je pryč a nepotřebuješ ho.

1. **Nainstaluj BombVault** na novou krabici a dej mu nový `APP_KEY`, jako při jakékoli nové instalaci. Nastav spojení pro VM v **Nastavení, Integrace, SSH k hostiteli**, pokud chceš VM zpět, protože obnova VM ho potřebuje.
2. **Spusť průvodce.** Na **Vítej v BombVaultu** zvol **Obnovit ze zálohy**. Na stránce Obnova zvol pod **Nouze** **Spustit nouzového průvodce**.
3. **Přístup.** Zvol **Nahrát sadu pro obnovu** a pusť soubor. BombVault vyzkouší její klíč na každém cíli a řekne například **Sada pro obnovu přečtena. Její klíč otevírá 0 z 2 cílů.** Místní složka říká **nenalezeno**, protože disk zemřel, a Backblaze B2 říká **chybí přihlašovací údaje**.
4. **Uložit přihlašovací údaje.** Sada nese přihlášení každého cíle. Stiskni **Uložit přihlašovací údaje** a cíle se vyzkouší znovu, takže Backblaze B2 teď říká **čitelný**. Sada zapsaná před verzí 10.5 obsahuje jen výchozí přihlašovací údaje. Kde cíl stále říká **chybí přihlašovací údaje**, zadej jeho přihlašovací údaje pod **Zadat chybějící přihlašovací údaje** a stiskni **Zkusit cíle znovu**.
5. **Převzít klíč.** Tento server teď zapisuje zálohy starým klíčem, takže nové zálohy zapadnou k starým. Svůj vlastní `APP_KEY` si v šabloně ponechává. **Vrátit převzetí** funguje, dokud neproběhne první záloha.
6. **Nastavení.** Krok se otevře na místě, kde klíč autozálohu našel, tady na B2, s vyplněnou adresou. Zvol **Vrátit nastavení**, které odtud přečte nejnovější autozálohu. BombVault se pro její použití restartuje a průvodce pokračuje sám. Tvé cíle, plány a pravidla jsou zpět a dál se přihlašuješ heslem tohoto serveru. Zadej znovu tajemství, která průvodce vypíše, například nastavení oznámení.
7. **Výběr.** **Kde jsou zálohy?** se otevře na kopiích mimo lokalitu, když nic místního nezbylo. BombVault přečte každý cíl, takže mrtvý cíl neskryje ostatní, vypíše 23 kontejnerů a 2 VM a každý obnoví z cíle, který drží jeho nejnovější snapshot.
8. **Obnova.** Zapni **Poté nechat zastavené**, což je po úplné ztrátě nejbezpečnější, a spusť. **Co se stane** říká, co se vrátí. Řádek s výsledkem říká například **25 obnoveno, 0 selhalo. Spusť je pod Zálohy, až bude třeba.**
9. **Dokončení.** Spusť nejdřív databáze, pak aplikace, které je používají. Otevři **Nastavení, Obnova** a stáhni novou sadu pro obnovu, protože stará patří předchozí instalaci. Zkontroluj **Vyžaduje pozornost** v Přehledu a spusť **Zálohovat vše nyní**, jakmile všechno funguje.

Pokud chybí i autozáloha, krok se otevře na **Přeskočit: nemám autozálohu** a řekne to. Cíle pak pocházejí jen ze sady, plány a pravidla nastavíš znovu a kopie mimo lokalitu čekají, dokud jejich zálohy nepřevezmeš jako vlastní zálohy tohoto serveru, viz [Po opětovném sestavení](#after-a-rebuild). **Výběr** pořád vypíše, co drží kopie mimo lokalitu. Sady složek jdou do složky, kterou vybereš. Kontejnery a VM se vrátí jako obyčejné soubory ve složce pod **Vrátit lze jen soubory**, protože jejich definice žijí vedle místního repozitáře a v autozáloze a kopie mimo lokalitu nedrží ani jedno.

!!! tip "Plánovaná migrace versus katastrofa"
    Nouzový průvodce obnovuje vlastní nastavení BombVaultu ze zálohy. Pro *plánované* stěhování na novou krabici můžeš místo toho přenést konfiguraci přímo kartou **Export / import nastavení** (přenosný soubor JSON). Viz [Konfigurace](configuration.md#portable-settings-export-and-import).

### Ze staré složky se zálohami {#restore-from-another-bombvault-repo}

**Ze staré složky se zálohami** pod **Další zdroje** na stránce Obnova otevře repozitář *jiné* instance BombVaultu (sdílenou složku připojenou pod `/mnt` nebo vzdálenou adresu) pomocí sady pro obnovu té instance nebo jejího napsaného `APP_KEY`, v jednorázové relaci jen pro čtení. Prohlédni si kontejnery, VM, sady složek a datové sady ZFS, které tam jsou uložené, vyber snapshot a obnov ho a obnovený objekt se stane normálním místním kontejnerem, VM nebo sadou složek. Do druhého repozitáře se nikdy nic nezapisuje a tvá vlastní nastavení záloh zůstanou nedotčená (relace žije v paměti a sama vyprší). Přesun kontejneru ze serveru A na server B neznamená přenastavovat své nastavení a pak je vracet zpět. Toto okno je jednorázové: otevře relaci, obnoví, co vybereš, a druhou instanci zapomene. Pokud místo toho chceš trvalé uspořádání, kde tato krabice stahuje snapshoty jiné instance do vlastního repozitáře podle plánu, to je role **Stahovač** na stránce Instance.

### Co tento server stáhl {#restore-what-was-fetched}

Každý zdroj, ze kterého tento server stahoval, má vlastní řádek **Staženo z** pod **Další zdroje** na stránce Obnova. Stažená kopie je zamčená vlastním klíčem tohoto serveru, ne klíčem zdroje, takže se na nic neptá: klíč zdroje ji neotevře. Sady složek a datové sady ZFS jdou do složky, kterou vybereš. Kontejnery a VM se vrátí jako obyčejné soubory ve složce, protože stažení nese snapshoty, a ne definice. Chceš-li je znovu vytvořit, použij v Nouzovém průvodci sadu pro obnovu a autozálohu zdroje.

Kontejner, jehož síť na tomto serveru neexistuje, například síť `br0` z Unraidu na obyčejném hostiteli Dockeru, ukáže pod svým řádkem výběr sítě. BombVault ho vytvoří na síti, kterou vybereš, spolu s jeho ostatními sítěmi. Pevná IP a adresa MAC patřily staré síti a zahodí se, takže je nová síť přidělí sama.

## Strážce klíče {#key-keeper}

Strážce klíče je instance tvé skupiny, která drží zapečetěnou kopii sady pro obnovu tohoto serveru. Kopii sama otevřít nemůže. Pokud se tento server někdy ztratí, nová instalace, která se připojí k tvé skupině, požádá strážce o kopii a otevře ji tvým kódem úschovy.

![Okno kódu úschovy: dvacet znaků ve čtyřech skupinách, zobrazených jednou.](assets/screenshots/offsite/recovery-code-dark.png#only-dark)
![Okno kódu úschovy: dvacet znaků ve čtyřech skupinách, zobrazených jednou.](assets/screenshots/offsite/recovery-code-light.png#only-light)

1. Na stránce Instance zapni **Strážce klíče** na kartě instance, která ho má držet, a zvol **Uložit do úschovy**. Uložení do úschovy vyžaduje přihlašovací heslo.
2. Tento server jednou ukáže **Kód úschovy**: dvacet písmen a číslic ve čtyřech skupinách po pěti. Zapiš si ho a potvrď **Kód je zapsaný**. Tento server kód neukládá, jen to, co potřebuje k zapečetění úschovy pro něj.
3. Strážce nyní drží úschovu a tento server ji obnovuje, když se sada změní. **Úschova klíče** na kartě této instance vypisuje **Úschova tohoto serveru** s každým strážcem a jeho stavem a **Úschovy, které tento server uchovává** pro ostatní. **Vytvořit nový kód** kód nahradí. Každý strážce přejde na nový, jakmile dostane novou úschovu.

V ten den potřebuješ tři věci: dvanáct slov své skupiny, kód úschovy a přístup k rozhraní strážce, kde někdo musí stisknout **Povolit**. V Nouzovém průvodci zvol **Přes skupinu**, vyber úschovu, požádej o ni a zadej kód úschovy. Kód se použije na novém serveru a strážci se neposílá. U strážce se žádost ukáže pod **Úschova klíče** s tím, kdo žádá, zda tam tuto instanci někdy předtím viděli a jak často žádala. **Povolit** vydá zapečetěnou kopii a povolení vyžaduje přihlašovací heslo. Nová instalace má na vyzvednutí po povolení 30 minut, žádost, na kterou nikdo neodpoví, vyprší po dni a instance, která žádá příliš často, musí počkat hodinu.

Bez kódu kopii nikdo otevřít nemůže, ani strážce. Pokud se kód ztratí, vytvoř nový, dokud tento server ještě běží. Pokud je server už pryč, sada pro obnovu a napsaný APP_KEY stále fungují. Strážce může úschovu kdykoli smazat a vlastník pak musí požádat znovu.

## Sada pro obnovu {#recovery-kit}

Tohle je kus, který umožňuje obnovu po havárii i tehdy, když žádný BombVault neběží.

**Stáhnout sadu pro obnovu** v kartě **Pro nouzové případy** v **Nastavení, Obnova** ti dá **klíč**, **odvozené heslo resticu**, každý **cíl** s repozitářem každé sekce, přihlášení, která nová instalace potřebuje, aby se k nim dostala, a **přesné příkazy resticu**, takže můžeš obnovovat rovnou pomocí CLI resticu na libovolném stroji. Vyžaduje přihlašovací heslo. Sada vypíše klíč i s vypnutým šifrováním, protože definice kontejnerů a VM jsou jím vždy zapečetěny. Po převzetí vypíše převzatý klíč záloh a APP_KEY tohoto serveru, který nová instalace potřebuje jen k vrácení nastavení. Karta **Ulož si sadu pro obnovu** v Přehledu ti to připomíná, dokud ji neuložíš.

!!! danger "Ulož sadu pro obnovu mimo server"
    Sada obsahuje tajemství, které dešifruje tvé zálohy. Drž ji na bezpečném místě odděleném od serveru (správce hesel, vytištěná kopie v trezoru). Pokud ztratíš BombVault i `APP_KEY` bez sady pro obnovu a bez úschovy klíče, tvé šifrované zálohy nelze obnovit.

!!! warning "Nejnovější snapshot není vždy ten, který chceš obnovit"
    Od resticu 0.17 ukazuje `restic snapshots` velikost každého snapshotu. Po ztrátě dat může být nejnovější snapshot ten vyprázdněný, takže neobnovuj snapshot, který je výrazně menší než ty před ním. Po ransomwaru to může být ten zašifrovaný v obvyklé velikosti. Pokud BombVault ještě běží, podívej se nejdřív na jeho stránku **Anomálie**: jmenuje poslední dobrou zálohu a okno obnovy ji předvybere. Obnova nepotřebuje žádná data anomálií BombVaultu a pozastavení uchovávání vždy jen drží víc snapshotů.

### Host záloh serveru {#backup-host}

Každý server zapisuje své zálohy pod vlastním hostem resticu: `bv-` a dvanáct znaků vypočtených z klíče záloh, například `bv-3f9a0c41d2e7`. Zálohy z doby, kdy server ještě vlastní host neměl, nesou host `bombvault`. Sada uvádí host tohoto serveru a její příkazy předávají oba přes `--host`, takže `latest` nikdy nevybere zálohu, kterou do stejného repozitáře zapsal jiný server. Server, který převezme zálohy jiného, pokračuje pod hostem toho serveru, protože host pochází z klíče. Pro každý [sdílený repozitář](#shared-repository), do kterého server kopíruje nebo jehož je vlastníkem, uvádí kapitola sady **Shared repositories** ostatní servery v něm s jejich hosty.

### Zapečetění sady {#sealing-the-kit}

Pokud jsi zapnul **Šifrovat exporty pomocí age** v **Nastavení, Systém**, sada je jím zapečetěna také a stáhne se jako `bombvault-recovery-kit.md.age`. Je v ASCII obalu, ne binární, takže je to pořád prostý text: vložení do správce hesel nebo vytištění funguje přesně jako dřív, obsah je bez tvého klíče nečitelný.

!!! warning "Neukládej klíč age do sady"
    K otevření zapečetěné sady potřebuješ svůj **soukromý** klíč age. Drž ho někde, co nezávisí na samotné sadě, jinak budeš mít místo jedné věci k obnově dvě. Zapečetění se vyplatí, když je sada uložena někde, co plně neovládáš (sdílený správce hesel, cloudové poznámky, výtisk v kanceláři). Sada ve tvém vlastním trezoru je už chráněna trezorem.

    Se zapnutým šifrováním a bez použitelného příjemce se stažení rovnou odmítne. BombVault nikdy nespadne na to, že by hlavní klíč vydal otevřeně.

### Když nemáš sadu po ruce {#if-you-do-not-have-the-kit-to-hand}

Heslo se nikde neukládá, **počítá se** z klíče záloh, takže si ho můžeš zopakovat sám, jen s klíčem a shellem:

```sh
printf 'bombvault:restic-repo'   | openssl dgst -sha256 -mac HMAC -macopt hexkey:$APP_KEY -r   | cut -d' ' -f1
```

To je HMAC-SHA256 nad pevným řetězcem `bombvault:restic-repo`, s klíčem ze surových bajtů hex `APP_KEY`, vypsané jako 64 malých hex znaků. Stejná hodnota je v sadě, vypsaná jako odvozené heslo resticu. Je to pro den, kdy je sada někde, kde nejsi. Na serveru, který převzal zálohy jiného, použij převzatý klíč záloh, což je starý APP_KEY toho serveru.

[Host](#backup-host) pochází ze stejného klíče a řetězce `bombvault:host`: `bv-` a prvních dvanáct z těchto znaků.

```sh
echo "bv-$(printf 'bombvault:host' | openssl dgst -sha256 -mac HMAC -macopt hexkey:$APP_KEY -r | cut -c1-12)"
```

!!! warning "U přijatého repozitáře použij klíč ODESÍLAJÍCÍ instance"
    Repozitář, který sem dorazil přes kopii mimo lokalitu, vytvořil stroj, který ho poslal, s **jeho** klíčem. Odvození z klíče přijímající krabice dá heslo, které restic odmítne, což vypadá přesně jako poškozený repozitář, a není. To je obvyklý důvod, proč `restic check` na přijatém repozitáři žádá o heslo pořád dokola.

Protože definice pro obnovu žijí **uvnitř** každého repozitáře (`<repo>/def`, `<repo>/vm-def`), zkopírovaná složka repozitáře je zcela samostatná, takže sada plus repozitář je všechno, co obnova na holém železe potřebuje. Kopie mimo lokalitu a stažená kopie drží snapshoty bez definic: vrací soubory a kontejner nebo VM se znovu vytvoří z autozálohy.

## Jak dostat zpět výpis databáze {#database-dumps}

Výpis databáze je vlastní bod obnovy v repozitáři kontejnerů, označený `dbdump:<container>`, s jediným souborem `/dbdump/<container>.sql`. BombVault je vypisuje, stahuje a importuje na záložce **Zálohy** kontejneru. Níže jsou stejné kroky jen s resticem, pro den, kdy BombVault není k dispozici.

```sh
restic -r <repo> snapshots --host <host> --host bombvault --tag dbdump:<container>
restic -r <repo> dump --host <host> --host bombvault --tag dbdump:<container> latest /dbdump/<container>.sql > <container>.sql
```

`<host>` je [host](#backup-host) serveru ze sady pro obnovu.

Štítky `dbversion:` a `dbname:` u každého výpisu říkají, z jaké verze serveru pochází a které databáze drží. Úplný soubor končí `-- PostgreSQL database cluster dump complete` nebo `-- Dump completed`.

Naimportuj ho do kontejneru stejné nebo novější verze (PostgreSQL), nebo stejné hlavní verze (MySQL a MariaDB), jednou spuštěného s prázdnou datovou složkou, aby se inicializoval. Hostitel nepotřebuje žádného databázového klienta, kontejner ho má:

```sh
docker exec -i <container> sh -c 'exec psql -X -U "${POSTGRES_USER:-postgres}" -d postgres' < <container>.sql
docker exec -i <container> sh -c 'exec mariadb -uroot -p"$MARIADB_ROOT_PASSWORD"' < <container>.sql
docker exec -i <container> sh -c 'exec mysql -uroot -p"$MYSQL_ROOT_PASSWORD"' < <container>.sql
```

Pro jedinou databázi z úplného výpisu berou MySQL a MariaDB `--one-database <name>` v příkazu klienta. Výpis PostgreSQL má jednu sekci na databázi, každá začíná řádkem `\connect <name>`: zkopíruj tu sekci do vlastního souboru a naimportuj ji s `-d <name>` po vytvoření databáze.

!!! warning "Výpis roota nese uživatele serveru"
    Úplný výpis MySQL nebo MariaDB pořízený jako root obsahuje systémovou databázi `mysql`, takže import nahradí účty nového serveru, včetně hesla roota, těmi z výpisu. U PostgreSQL je `role ... already exists` pro uživatele, kterého kontejner vytvořil, očekávané a neškodné.
