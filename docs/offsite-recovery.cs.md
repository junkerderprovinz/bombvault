# Mimo lokalitu a obnova

!!! note "Kopie mimo lokalitu čekají po opětovném sestavení"
    Když krok 4 znovu sestaví položky bez starého nastavení, replikace mimo lokalitu těchto domén se pozastaví, dokud se nepotvrdí výchozí umístění. Viz [Umístění pro jednotlivé položky](#placement).

Místní zálohy vás chrání před ztraceným kontejnerem nebo špatnou aktualizací. Replikace mimo lokalitu a otestovaná sada pro obnovu vás chrání před celým strojem, ransomwarem nebo požárem. Tato stránka pokrývá replikaci mimo lokalitu, zajištění odolnosti té kopie proti manipulaci, prokázání, že umíte obnovit, a obnovu, když je samotný BombVault pryč.

## Replikace mimo lokalitu

Ponechte rychlou místní zálohu a přidejte jednu nebo více replik mimo lokalitu. Nastavte repozitář na doménu na stránce **Nastavení, Mimo lokalitu**. BombVault tam replikuje nové snímky pomocí `restic copy` na základě nejlepší snahy, takže zádrhel mimo lokalitu nikdy nezhatí místní zálohu. V tomto uspořádání zůstává místní repozitář primární a repozitář mimo lokalitu je replika, primární repozitář domény ale vůbec nemusí být místní; viz [Vzdálené primární repozitáře](#remote-primary-repositories) níže, jak zálohovat přímo do S3, rest-serveru atd. místo replikace do nich.

- **Více cílů mimo lokalitu na doménu.** Každá doména (kontejnery, VM, flash, config, sady souborů a datové sady ZFS) může replikovat na několik cílů mimo lokalitu najednou, ne jen na jeden, takže můžete držet například rest-server na stroji kamaráda a S3 bucket paralelně. Přidejte další cíle v Nastavení, Mimo lokalitu, každý s vlastním repozitářem, třídou úložiště S3, příznakem append-only, uchováváním a rozpočtem růstu. Stávající jednotlivé nastavení mimo lokalitu se nedotčeno přenese jako první cíl a každý cíl domény replikuje podle plánu mimo lokalitu dané domény.
- **Plán mimo lokalitu na doménu** (upravovaný spolu s každým dalším plánem v Nastavení, Plány): ponechte prázdný pro replikaci po každé místní záloze, nebo nastavte kadenci (například `weekly Sun 03:00`) pro odesílání mimo lokalitu méně často, než zálohujete místně. Tlačítko **Replikovat nyní** pokrývá běhy na vyžádání.
- **Uchovávání mimo lokalitu** žije v Nastavení, Uchovávání, takže můžete kopie mimo lokalitu držet déle jako archiv. Ponechte zásadu celou na nule, aby se snímky mimo lokalitu nikdy automaticky neprořezávaly.
- **Limity šířky pásma** (Nastavení, Mimo lokalitu) omezují rychlost nahrávání/stahování restic, aby replikace nezasytila vaše WAN.
- **Indikátor replikace** zobrazuje, která doména právě replikuje, zatímco běží (na její stránce a na Přehledu). Je to aktivní indikátor, nikoli procentuální panel, protože `restic copy` nezpřístupňuje žádný strojově čitelný průběh.

!!! note "Obnova z libovolného místa"
    Každý kontejner, VM, sada složek, flash i konfigurace aplikace vypisují své zálohy jako jednu časovou osu napříč všemi místy, kde záloha leží. Záloha zkopírovaná do B2 se objeví jednou, označená každým místem, které ji drží. Obnova vezme první dosažitelné místo, počínaje repozitářem, do kterého se položka zapisuje, a u každého řádku můžete zvolit jiné místo. Místa mimo lokalitu se čtou, jen když je otevřete. Mazání na jednom místě nejdřív zkontroluje ostatní a řekne, jestli to byla poslední kopie.

## Cíle {#destinations}

Nastavení, Mimo lokalitu začíná kartou **Cíle**: místa, kam míří kopie mimo lokalitu, nastavená jednou pro všechny domény. Cíl se pak objeví jako tlačítko v řádku **Umístění** každé domény a položky. Při prvním zaškrtnutí pro doménu BombVault vytvoří repozitář té domény ve složce pod cílem, například `rclone:onedrive:BombVault/containers`.

**Přidat cíl** otevře průvodce o pěti krocích:

1. **Kam se mají zálohy ukládat?** Každá služba je vypsaná se svým logem ve čtyřech skupinách: úložné služby s buckety S3 (Backblaze B2, Wasabi, Cloudflare R2, Hetzner Object Storage, Amazon S3 a další), váš vlastní server S3 (Garage, SeaweedFS, RustFS, Silo, Ceph, JuiceFS, Versity S3 Gateway), váš vlastní server a sdílené složky (rest-server, Hetzner Storage Box, SFTP, SMB, WebDAV, připojená cesta) a cloudová úložiště (OneDrive, Google Drive, Dropbox, pCloud, Nextcloud a vše ostatní, co rclone podporuje). U každé je uvedeno, jak se pro zálohy hodí: cloudové disky zpomalují při velkém počtu požadavků, takže tam první záloha a prořezávání trvají déle.
2. **Přihlášení.** Pole závisí na službě: přístupový klíč pro S3, uživatel a heslo pro WebDAV a SMB, heslo aplikace tam, kde dvoufázové přihlášení blokuje běžné heslo, veřejný klíč SSH z BombVaultu pro SFTP a Storage Box, nebo token u služeb, které se přihlašují přes prohlížeč. Pro ně průvodce ukáže příkaz `rclone authorize`, který spustíte na počítači s prohlížečem; token, který vypíše, vložíte do pole. **Otestovat připojení** ověří přihlášení dřív, než se cokoli uloží.
3. **Vyberte složku.** Průvodce vypíše složky na cíli, s tlačítkem **Nová složka** pro založení nové a s volným místem, pokud ho služba hlásí. Nejbezpečnější je prázdná složka.
4. **Ochrana proti smazání.** Průvodce otevřeně řekne, co služba umí. Rest-server v režimu append-only smazání odmítne a test odolnosti proti manipulaci to ověří. Bucket S3 může uchovávat staré verze pomocí verzování a zámku objektů, což BombVault zatím ověřit neumí. Cloudový disk smazání odmítnout neumí vůbec: kdo se dostane na server, dostane se i k té kopii. Zapněte **Neměnné (append-only)** jen tam, kde druhá strana smazání opravdu odmítá; BombVault tam pak nikdy neprořezává.
5. **Pro případ nouze.** Sada pro obnovu vypisuje každý cíl s repozitářem každé domény pod ním. Přihlášení se vrátí se zálohou nastavení BombVaultu; na čisté instalaci bez ní nastavte cíl znovu na stejném místě.

Služby S3 běží přes vlastní backend S3 resticu, díky čemuž se může uplatnit třída úložiště a zámek objektů. Všechny ostatní služby běží přes rclone, který BombVault dodává, a jejich remote se pak objeví v konfiguraci rclone v Nastavení, Cloudový přístup. Export nastavení zahrnuje karty Cíle; s přihlašovacími údaji zahrnuje i jejich přihlášení.

Přijímací server, který provozuje jiná instance vaší skupiny, se v průvodci objeví pod **Z vaší skupiny**; viz [Přijímací server](#receiving-server).

Cíl domény vytvořený z karty Cíle převezme její název, umístění, přihlašovací údaje, třídu úložiště a přepínač neměnnosti. Uchovávání, komprese a rozpočet růstu zůstávají na doménu a jeho umístění se přesunout nedá, protože tam leží repozitář domény. **Přidat cíl jen pro tuto doménu** pod každou doménou dál přijímá ručně zadanou URL repozitáře.

Ručně zadaný cíl domény, jehož repozitář leží ve složce některého cíle, se k tomuto cíli může připojit. Cíl takové cíle domén vypisuje pod **Už pod tímto cílem** a **Převzít** jeden z nich pod něj zařadí. Cíl domény si ponechá svůj repozitář, snapshoty, uchovávání a umístění a převezme název, přihlašovací údaje, třídu úložiště a přepínač neměnnosti cíle. BombVault nejprve ověří, že přihlášení cíle repozitář otevře, a odmítne zařadit append-only cíl domény pod cíl, který append-only není. Převzetí primárního cíle domény vyprázdní pole Off-site této domény.

## Umístění pro jednotlivé položky {#placement}

Každá karta kontejneru, VM a sady složek má řádek **Umístění** z tlačítek: **Místní** a jedno tlačítko na každý cíl domény mimo lokalitu, za nimi cíle, pod kterými doména zatím žádný cíl nemá. Rozsvícená tlačítka dostávají zálohy položky.

- S rozsvíceným **Místní** se položka zapíše do repozitáře zobrazeného pod **Uloženo na** a zkopíruje se do každého dalšího rozsvíceného cíle. Zhasněte cíl a ten od této položky nedostane nic nového. Samotné Místní nekopíruje nikam, což se hodí pro data, která už mají druhou kopii, například sdílenou složku, jež žije na NAS.
- Se zhasnutým **Místní** se položka zapíše přímo do přímého repozitáře prvního rozsvíceného cíle a odtud se zkopíruje do ostatních rozsvícených cílů. Poprvé ten přímý repozitář vytvoří dialog.
- Tlačítko cíle vytvoří cíl domény pod tímto cílem a rozsvítí ho jen pro tuto položku. Každá další položka tam začíná bez kopie.
- Jedno tlačítko zůstane rozsvícené, protože záloha potřebuje místo, kam půjde. Chcete-li něco ze záloh vynechat, vyloučte to.

Umístění je pevné od první zálohy položky, protože BombVault nikdy nepřesouvá zálohy mezi repozitáři. Kopie se mohou kdykoli změnit. Cíl, který už položku nedostává, si ponechá kopie, které má, a při dalším běhu mimo lokalitu dané domény je zkrátí podle vlastního uchovávání; **Smazat v B2** na kartě je odstraní okamžitě. Pokud některé z těchto kopií neexistují nikde jinde, potvrzení je vypíše podle data a požádá o název položky. Z cílů append-only mazat nelze.

Pod řádkem karta ukazuje, kam položka směřuje a co tam skutečně je: na kolika lokalitách leží, kdy byl každý cíl naposledy viděn a jestli je splněno 3-2-1. Lokalita je server s původními daty, každý cíl mimo lokalitu a každý repozitář označený **Mimo objekt**. BombVault kontroluje kopie a lokality; část 3-2-1 o "dvou médiích" nekontroluje.

### Výchozí umístění

Nastavení, Úložiště, **Výchozí umístění** má jeden řádek na doménu se stejnými tlačítky. Kopie platí okamžitě pro každou položku bez vlastní volby a pro projektové složky Compose stacků. Umístění platí pro novou položku při její první záloze; jeho změna žádné zálohy nepřesune. Před uložením řádek jmenuje každý cíl, který získává nebo ztrácí položky, a kolik snímků to znamená. **Použít na položky bez záloh** vrátí každou položku bez dosavadní zálohy na výchozí nastavení.

Nový cíl mimo lokalitu dostane každou položku, která není nastavena na Místní. Dialog, který jej přidává, uvádí počet položek a, je-li známa, kolik historie to představuje, a nabízí vynechat položky, které jsou už vyloučené u jiných cílů.

### Přímé repozitáře

Vypnutí Místní u položky, takže jejím domovem se stane cíl bez přímého repozitáře, otevře dialog s navrhovaným umístěním vedle cíle, například `s3:https://s3.eu-central-003.backblazeb2.com/bucket/containers-direct`, a testem připojení, který nic nezaloží. **Vytvořit a použít** vytvoří repozitář a nasměruje na něj položku. Přímý repozitář přebírá klíč cíle, třídu úložiště, limity, nastavení append-only a uchovávání a mění se s nimi; karta Repozitáře jej zobrazuje jako jen pro čtení. Když nový klíč cíle repozitář neotevře, přímý repozitář si ponechá klíč, který má, a uložení to oznámí. Položka v přímém repozitáři se z něj kopíruje do ostatních rozsvícených cílů, nikdy do cíle, kterému repozitář patří. Jeho snímky nesou značku `bv:direct` a každý další běh uchovávání je ponechá, takže přímý repozitář, který ztratil spojení se svým cílem, nikdy nestárne podle místních pravidel. K B2 se přistupuje přes její S3 endpoint, kde jako přihlašovací údaje S3 zadáte ID klíče a aplikační klíč; klíč omezený na vlastní složku cíle se nedostane do složky vedle ní, proto klíč omezte místo toho na složku nad cílem.

### Mimo objekt

Pojmenovaný repozitář lze na kartě Repozitáře označit jako **Mimo objekt**. Vzdálené repozitáře začínají označené; vypněte to pro rest-server ve stejné budově. Označení se na kartách počítá jen do lokalit a 3-2-1. Žádnou kopii nemění.

### Po opětovném sestavení

Volby kopírování žijí ve vlastním nastavení BombVaultu. Po opětovném sestavení přes Objevit zálohy bez obnoveného `/config` jsou pryč a kopírování všeho by znovu poslalo do B2 položky, které jste dřív vynechali. Replikace mimo lokalitu každé znovu sestavené domény se proto pozastaví. Přehled to zobrazí oranžově a Výchozí umístění nabídne **Potvrdit výchozí nastavení** s náhledem toho, co příští běh zkopíruje, a jmény v zálohách, které nemají žádnou položku, jež tam můžete vynechat. Pozastavení ukončí jen potvrzení; import souboru nastavení vrátí pravidla a výchozí hodnoty, ale pozastavení neukončí.

## Vzdálené primární repozitáře {#remote-primary-repositories}

Cesta zálohy domény (Nastavení, Úložiště) se neomezuje na místní složku: nasměrujte ji rovnou na vzdálený repozitář resticu (`s3:...`, `rest:http://host:8000/repo`, `sftp:uživatel@host:/repo`, `rclone:remote:bucket/cesta`) a BombVault zálohuje přímo tam, bez samostatné místní kopie a bez kroku replikace. Je to opravdu jiný tvar než replikace mimo lokalitu výše: tam je primární místní repozitář a ten mimo lokalitu je jeho archivem podle možností; zde **je** primární ten vzdálený a je jedinou kopií, dokud pro tuto doménu nenastavíte i replikaci mimo lokalitu (nebo druhý vzdálený repozitář).

Každé ze šesti polí cesty (Kontejnery, VMs, Flash, Autozáloha, Složky, Datové sady ZFS) má hned vedle přepínač **Místní / Vzdálené**:

- **Místní** zobrazí známý prohlížeč složek.
- **Vzdálené** jej vymění za prosté pole URL a tlačítko, které otevře stejné okno testu připojení a přihlašovacích údajů, jaké používají cíle mimo lokalitu, jen nastavené pro tento primární repozitář. Odtud získáte:
    - **Test připojení** proti skutečné cestě, dřív než se na ni spolehnete.
    - **Omezení šířky pásma** (odesílání a stahování), aby plánovaná záloha do vzdáleného primárního repozitáře nezahltila vaši linku WAN: tytéž přepínače resticu `--limit-upload` a `--limit-download`, které používá replikace mimo lokalitu, uplatněné na zálohu samotnou.
    - **Ochranu append-only (neměnnost)**, ověřenou stejným aktivním testem manipulace (skutečná sonda DELETE proti druhé straně), jaký dostávají cíle mimo lokalitu. Když je zapnutá, BombVault odmítne repozitář prořezávat: protože za ním není samostatná místní kopie, přihlašovací údaje na tomto stroji nesmějí být schopné smazat jedinou kopii zálohy.
    - **Výstrahu rozpočtu růstu**, odvozenou ze stejného trendu velikosti repozitáře, který karta Úložiště už sleduje.

Nic z toho není povinné: ručně zadaná vzdálená cesta bez uložených bezpečnostních nastavení zálohuje přesně jako dosud (neomezená šířka pásma, lze prořezávat, žádná výstraha rozpočtu). Bezpečnostní okno je tu pro chvíli, kdy chcete stejnou ochranu, jakou dostává kopie mimo lokalitu, aniž byste kvůli tomu museli zakládat samostatný cíl mimo lokalitu.

!!! note "Přihlašovací údaje ke cloudu a REST jsou sdílené"
    Vzdálený primární repozitář se ověřuje stejnými údaji S3/REST, které jsou nastavené v Nastavení, Cloudový přístup, Sdílené cloudové přihlašovací údaje. Samostatné úložiště údajů pro primární repozitáře neexistuje.

### SMB a WebDAV bez připojení na hostiteli {#smb-webdav}

V Nastavení, Cloudový přístup, rclone je formulář pro sdílenou složku Windows nebo Samba a pro server WebDAV (Nextcloud, ownCloud, SharePoint nebo jakýkoli jiný). Vyplňte krátký název, hostitele a sdílenou složku (SMB) nebo URL a typ serveru (WebDAV), uživatele a heslo a BombVault za vás zapíše sekci rclone. rclone heslo před uložením sám zamaskuje; přidání cíle s názvem, který už existuje, tuto sekci nahradí, místo aby přidalo druhou.

Formulář odpoví hotovým umístěním, například `rclone:nas:backups`. Vložte ho do Zálohovací cesty nebo do cíle mimo lokalitu a pokud chcete, přidejte podsložku (`rclone:nas:backups/bombvault`). Sdílená složka je první segment cesty, ne součást názvu.

To je lepší cesta než připojit sdílenou složku v Unraidu: restic nedoporučuje držet repozitář na připojené sdílené složce CIFS a tady se nic nepřipojuje. NFS ve formuláři není, protože restic ani rclone nemají backend NFS; pro NFS připojte export na hostiteli a nasměrujte na něj Zálohovací cestu.

## Neměnné (append-only) mimo lokalitu

Označte repozitář mimo lokalitu jako append-only, aby ransomware nebo kompromitovaný hostitel nemohl smazat nebo přepsat vaše zálohy. Druhá strana (`restic/rest-server` běžící v režimu `--append-only`) to **vynucuje**. BombVault to pouze **ověřuje** a nikdy nezobrazí zelenou jen na základě konfiguračního tvrzení.

Průvodce **řízeného nastavení mimo lokalitu** vás provede od volby backendu (rest-server / rclone / S3) přes připravený úryvek pro nasazení rest-serveru, test připojení, přepínač neměnnosti (který spustí test odolnosti okamžitě) a strategii uchovávání, takže append-only mimo lokalitu je dosažitelné bez ručního editování konfigurací.

!!! note "Úspěšné smazání v `/locks/` je očekávané"
    Append-only neznamená, že už nelze nic smazat. restic musí zakládat a uvolňovat vlastní zámky, proto `/locks/` záměrně zůstává zapisovatelný a smazatelný. Snapshoty a data za nimi, tedy přesně to, na co by mířil ransomware, odstranit nelze. Pokud si vzdálenou stranu ověříš sám, úspěšné smazání v `/locks/` je správné chování, ne díra v ochraně.

!!! warning "Neměnné repozitáře se z tohoto stroje nikdy neprořezávají"
    Neměnné mimo lokalitu záměrně nikdy neprořezává staré snímky. Nastavte pro něj **alarm rozpočtu růstu**, abyste byli upozorněni dříve, než se velikost repozitáře vymkne kontrole.

## Test odolnosti proti manipulaci

BombVault pravidelně dokazuje záruku append-only tím, že skutečně zkusí mazání proti repozitáři mimo lokalitu, zaměřené na neexistující objekt:

- **Odmítnuto** znamená chráněno.
- **Přijato** znamená nechráněno.
- **Neprůkazný** výsledek (server nedosažitelný, chyba autentizace) nikdy nezmění uložený verdikt.

Skutečný přechod z chráněno na nechráněno spustí jediné upozornění.

## Cvičné obnovy po havárii

BombVault nabízí dvě úrovně důkazu, že vaše zálohy jsou skutečně obnovitelné, nejen přítomné.

- **Cvičné obnovy s ověřením (místní).** BombVault pravidelně spouští `restic check --read-data-subset` (omezené, nikdy plná obnova zaplňující disk) a zobrazuje odznak *Ověřeno jako obnovitelné* na doménu. Kadence žije v Nastavení, Plány; odznak v Nastavení, Integrita.
- **Cvičné obnovy po havárii (mimo lokalitu).** BombVault obnoví skutečný cíl z repozitáře mimo lokalitu do jednorázového sandboxu, ověří jej soubor po souboru a bajt po bajtu, poté ukliďte. To dokazuje, že umíte obnovit z mimo lokalitu, nejen že repozitář odpovídá.

**Vysvědčení ochrany proti ransomwaru** na Přehledu to shrne do zeleného / oranžového / červeného postoje na doménu, s kontrolním seznamem s věkovou značkou (mimo lokalitu nakonfigurováno, append-only ověřeno, replikace aktuální, cvičná obnova prošla, šifrování zapnuto, strategie prořezávání nastavena). Každý červený řádek odkazuje přímo na opravu a karta se rozsvítí zeleně jen na ověřených faktech.

## Párování instancí {#pairing}

Přijímače, zdroje stahování, stránka Instance i Mesh mimo lokalitu, to všechno mluví s jiným BombVaultem. Dělají to jako členové jedné párovací skupiny a instance do skupiny vstupuje dvanácti slovy.

Na první instanci otevřete **Nastavení → Párování** a klikněte na **Vygenerovat frázi** v kartách párování. Objeví se dvanáct slov v okně s tlačítkem **Kopírovat**. Na každé další instanci otevřete stejné místo, klikněte na **Zadat frázi** a vložte je nebo je napište, nebo klikněte v tomto okně na **Vložit**. Slovo, které není na seznamu, stránka pojmenuje i s jeho pozicí hned při psaní, a poslední slovo nese kontrolní součet, takže se překlep nebo prohozené slovo odhalí dřív, než se cokoli spáruje. Vygenerujte frázi jen na jedné instanci: dvě instance, které obě vytvoří frázi, vytvoří dvě oddělené skupiny. Pokud se minutu nikdo neohlásí, záložka nabídne dvě cesty ven: znovu zobrazit slova, abyste je zadali tam, nebo zadat slova druhé instance a připojit se k její skupině najednou. Párování funguje i bez přihlašovacího hesla, ale nastavte si ho: bez něj si může slova přečíst kdokoli, kdo dokáže otevřít toto webové rozhraní, a přes skupinu získat heslo restic každé instance v ní. Karta párování na to upozorňuje, dokud heslo nenastavíte. S heslem si o něj opětovné zobrazení fráze řekne. **Opustit skupinu** instanci ze skupiny zase vyřadí.

Kdokoli zná ta slova, může se do skupiny přidat, takže s nimi zacházejte jako s heslem.

**Jak se členové navzájem najdou.** Každá instance se dozví svou vlastní adresu v síti z vašeho prohlížeče hned, jak se přihlásíte; v kartě přeposílače se zobrazí jako **Tato instance ve vaší síti**, a pokud před ní stojí reverzní proxy nebo neobvyklý port, opravte ji tam. Ve stejné síti si členové tuto adresu oznamují multicastem a mluví spolu přímo, a tam, kde multicast nedokáže projít sítí kontejnerů, jako je výchozí bridge síť Dockeru, instance místo toho prohledá svou vlastní podsíť a najde ostatní podepsaným voláním, na které umí odpovědět jen člen skupiny, takže se páruje v řádu sekund i bez přeposílače. Pokud se nic nenajde, **Nemůžete ji najít?** pod kartou párování přijme jednu adresu ručně, pro jinou podsíť nebo neobvyklý port. Instance v různých sítích jdou přes přeposílač, který se vybírá na stejné záložce:

- **Přeposílač projektu** (výchozí): `parleyport.halleluja.design`, stejný přeposílač, jaký používá i KnightLoader. Není co nastavovat.
- **Vlastní přeposílač**: kontejner [**ParleyPort**](https://github.com/junkerderprovinz/parleyport) z Unraid Community Apps, nebo jedna z vašich instancí, která je už zvenčí dostupná se zapnutým přepínačem **Sloužit jako přeposílač**. Taková instance pak odpovídá na `/relay/connect` na své vlastní adrese, za reverzní proxy a certifikátem, které už má, a pustí dovnitř jen vaši skupinu. Adresu přeposílače zadejte na každé instanci, která ho má používat.
- **Bez přeposílače**: členové se najdou automaticky jen ve stejné síti, a nikde jinde.

**Co vidí přeposílač.** Každé volání mezi členy je zapečetěné pomocí AES-256-GCM klíčem odvozeným z dvanácti slov, a ten klíč nikdy neopustí vaše instance. Přeposílač se dozví hash, který sdružuje spojení, dále pro kterou instanci je zpráva určená, jak je velká a kdy prochází. Přímé volání v lokální síti je zapečetěné stejným způsobem a navíc podepsané, takže nic nezávisí na self-signed certifikátu, který instance nabízí.

**Co prochází skupinou.** Vysvědčení na stránce Instance, žádost o okamžitou kontrolu jedné domény, nabídky Mesh mimo lokalitu a to, co potřebuje přijímač nebo zdroj stahování: umístění repozitářů druhé instance a její heslo restic. Data záloh po ní nikdy neprochází, ta jdou pořád přímo do backendů restic. Ani APP_KEY ne: heslo restic otevírá repozitáře té instance a nic jiného, ne její uložená tajemství, relace ani obnovovací kódy.

**Záznamy z doby před párováním.** Instance přidané fleet tokenem a přijímače i zdroje stahování nastavené pomocí APP_KEY druhé instance zůstávají po aktualizaci zachované a jsou označené jako **Spárovat znovu**. Přijímače a zdroje stahování dál fungují: při prvním spuštění BombVault nahradí každý uložený APP_KEY heslem restic z něj odvozeným. Spárujte obě instance, pak upravte záznam a vyberte jeho instanci. Taková instance převezme svou starou kartu, jakmile se ve skupině objeví instance se stejným jménem.

Jediné místo, které pořád bere APP_KEY ručně, je [Obnova z jiného BombVault repozitáře](#restore-from-another-bombvault-repo), pro případ, že druhá instance zmizela a nemůže odpovídat ve skupině.

## Řídicí panel přijímače (přijímací strana)

![Přijímající strana, sledovaná jen pro čtení, s kontrolou integrity na tomto stroji.](assets/screenshots/receiver.png)

*Přijímající strana, sledovaná jen pro čtení, s kontrolou integrity na tomto stroji.*

Vše výše je *odesílající* strana. Na stroji, který **přijímá** neměnné kopie mimo lokalitu z jiného BombVaultu, vám řídicí panel přijímače dává nezávislé monitorování těchto repozitářů jen pro čtení na přijímacím hardwaru, takže tiché selhání na druhém konci nezůstane bez povšimnutí.

Zapněte přepínač **Přijímač** v Nastavení k odhalení záložky **Přijímač**. Ve výchozím stavu je vypnuto; zapněte jej jen na stroji, který skutečně přijímá neměnné zálohy mimo lokalitu. Poté zaregistrujte přijatý repozitář (jen pro čtení, otevřený heslem restic odesílající instance, které přichází přes [párovací skupinu](#pairing)) pro získání:

- **Inventáře snímků seskupeného podle zdroje**, takže vidíte přesně, které kontejnery, VM a sady souborů dorazily.
- **Naposledy přijato** na zdroj, takže víte, jak čerstvý každý je.
- **Nezávislého `restic check`** spuštěného na přijímacím hardwaru, takže integrita je ověřena tam, kde data skutečně leží, nejen na odesílateli.
- **Pojistky mrtvého muže:** upozornění, když zdroj přestane odesílat v okně, které nastavíte.
- **Upozornění na integritu:** upozornění, když kontrola na přijímací straně selže.

Přijímač je striktně jen pro čtení. Nikdy nezapisuje do přijatého repozitáře, takže nikdy nemůže porušit záruku append-only, na kterou se odesílatel spoléhá.

### Přijímací server {#receiving-server}

Přijímací stroj může také provozovat rest-server, na který ostatní kopírují. **Nastavit přijímací server** nahoře na záložce **Přijímač** se zeptá na složku na sdílené složce, s tlačítkem **Nová složka** pro její vytvoření, a na port (8000, pokud ho nepoužívá jiný kontejner). BombVault pak:

1. odmítne pokračovat, pokud už existuje kontejner s názvem `rest-server` nebo port drží jiný kontejner;
2. stáhne `restic/rest-server` a spustí ho přes Docker socket v režimu append-only se soukromými repozitáři a souborem s přihlášením ve složce;
3. zapíše jeho šablonu pro Unraid na flash disk, takže kontejner zůstane upravitelný na záložce Docker, nebo šablonu nabídne ke stažení, když je flash disk mimo dosah;
4. spustí na něm test odolnosti proti manipulaci a ukáže, zda odmítá mazání.

Instance vaší skupiny pak server najdou v průvodci cílem pod **Z vaší skupiny**, pojmenovaný podle přijímacího stroje. Každá instance dostane vlastní přihlášení při prvním výběru serveru a zapisuje tam jen do své složky. Karta tato přihlášení vypisuje a **Odvolat přihlášení** jedno z nich odebere; co ta instance už zkopírovala, ve složce zůstane. Nastavení vytvoří také jedno přihlášení pro někoho mimo skupinu, jehož heslo karta ukáže jen jednou.

Instance, která se k přijímacímu stroji dostane jen přes relay, server použít nemůže, protože relay žádné zálohy nepřenáší. Nejprve přidejte adresu přijímacího stroje v **Nastavení → Párování**. Když BombVault běží na vlastní IP adrese (například na br0), vyplňte **Adresa pro partnery**, protože server naslouchá na adrese hostitele.

## Kompletní příklad: dva stroje Unraid, od začátku do konce

Výše jsou popsány jednotlivé díly. Tohle je jedno úplné nastavení se skutečnými hodnotami, protože díly se skládají snáz, když je člověk jednou viděl složené.

Dva stroje: **TOWER** provozuje kontejnery a posílá zálohy, **VAULT** je přijímá a vynucuje neměnnost. Dosaďte vlastní názvy, adresy a cesty ke sdílení.

**1. Na VAULT postavte server v režimu append-only.** V BombVaultu na TOWER jděte do *Nastavení → Mimo lokalitu → Nastavit*, zvolte **rest-server** a vygenerujte recept. Zkopírujte kartu **Šablona Unraid (XML)**, uložte ji na VAULT jako `/boot/config/plugins/dockerMan/templates-user/my-rest-server.xml`, pak *Docker → Add Container* a vyberte **rest-server** ze seznamu šablon. Před spuštěním zapište zobrazený řádek `htpasswd` na VAULT do `/mnt/user/appdata/rest-server/.htpasswd`. Jednorázové heslo se zobrazí jen jednou a nikdy se neukládá: zkopírujte si ho teď. Ten řádek nese stejné heslo, už zahašované bcryptem: otevřený text patří do REST přihlašovacích údajů na TOWER, zahašovaný řádek do `.htpasswd` na VAULT. Sami nic hašovat nemusíte.

    Nechte `--append-only` v poli OPTIONS. O to tu celou dobu jde: bez toho je VAULT zase obyčejné sdílení.

**2. Na TOWER na něj nasměrujte vzdálený repozitář.** URL repozitáře má tvar, který recept vypíše:

    rest:http://VAULT:8000/bombvault-containers/containers

První část cesty je uživatel htpasswd, druhá je repozitář. Zadejte vygenerovaného uživatele a heslo jako přihlašovací údaje REST pro cíl a spusťte **test připojení**.

**3. Na TOWER zapněte «Neměnné».** Test manipulace proběhne hned a musí hlásit *chráněno*. Co odpovědi znamenají:

| Výsledek | Co se stalo |
| --- | --- |
| **chráněno** | VAULT smazání odmítl. To je jediný vyhovující stav. |
| **NENÍ chráněno** | VAULT smazání přijal. Chybí `--append-only`, nebo byl odebrán. |
| **neprůkazné** | Ani jedno. Obvykle URL není ta, kterou používá sám restic, nebo se změnily přihlašovací údaje. Nic se nezaznamená a nespustí se žádné upozornění. |

**4. Na VAULT sledujte, co přichází.** Spárujte obě krabice ([Párování instancí](#pairing)), zapněte *Nastavení → Obecné → Přijímač*, otevřete kartu **Přijímač** a zaregistrujte repozitář jen pro čtení s TOWER jako odesílající instancí.

!!! warning "Umístění je cesta **uvnitř** kontejneru, zapsaná relativně k připojení hostitele"
    Zadejte `user/appdata/rest-server/bombvault-containers/containers`, **ne** `/mnt/user/appdata/…`. BombVault běží v kontejneru, kde je `/mnt` hostitele připojeno jinde; absolutní cesta hostitele tam neexistuje. Když ji vložíte, BombVault vám nyní sdělí relativní cestu, kterou máte použít.

    VAULT dostane heslo restic od TOWER přes skupinu, když uložíte; klíč nikdo neopisuje.

**5. Pokud chcete, udělejte to oboustranně.** Zopakujte stejných pět kroků opačným směrem: rest-server na TOWER přijímající kopii z VAULT. Každý stroj pak vynucuje neměnnost pro ten druhý a ani jeden nemůže smazat zálohy toho druhého.

## Řízená obnova

Vyhrazená záložka **Obnova** provede čistou nebo znovu sestavenou instalaci havarijním případem, na jednom místě:

1. **Nejprve obnoví vlastní nastavení BombVaultu**, takže zálohovací cesty, cíle mimo lokalitu a přihlašovací údaje, které zbytek postupu potřebuje, přijdou předvyplněné (aplikováno přes sebe-restart přes Docker socket, takže se živá databáze nastavení nikdy nepřepisuje pod otevřeným handlem).
2. **Zkontroluje, že BombVault umí číst vaše zálohy** (zádrhel se šifrovacím klíčem hned zkraje).
3. Nechá vás **nasměrovat na váš existující repozitář** (místní nebo mimo lokalitu).
4. **Objeví** kontejnery, VM, sady souborů a datové sady ZFS v něm uložené.
5. **Obnoví najednou kontejnery a VM** (ponechané zastavené, takže je spustíte záměrně) a vypíše sady souborů a položky ZFS k obnovení jednu po druhé; položky ZFS se vrátí vypnuté. Sada pro obnovu je na jedno kliknutí.

!!! tip "Plánovaná migrace versus havárie"
    Řízená obnova obnovuje vlastní nastavení BombVaultu ze zálohy. Pro *plánovaný* přesun na nový stroj můžete místo toho přenést konfiguraci přímo pomocí karty **Export / import nastavení** (přenosný soubor JSON). Viz [Konfigurace](configuration.md#portable-settings-export-and-import).

### Obnova z jiného BombVault repozitáře {#restore-from-another-bombvault-repo}

Samostatná karta v záložce **Obnova** otevře repozitář *jiné* instance BombVaultu (sdílená složka připojená pod `/mnt`, nebo vzdálená URL) s **`APP_KEY` dané instance**, v jednorázové relaci jen pro čtení. Procházejte kontejnery, VM a sady souborů tam uložené, vyberte snímek a obnovte jej, a obnovený objekt se stane běžným místním kontejnerem, VM nebo sadou souborů. Do druhého repozitáře se nikdy nic nezapíše a vaše vlastní nastavení záloh zůstane nedotčeno (relace žije v paměti a sama vyprší). Přesun kontejneru ze serveru A na server B neznamená přesměrovávat nastavení repozitáře a poté je vracet zpět. Tato karta je jednorázová: otevře relaci, obnoví, co vyberete, a na druhou instanci zapomene. Pokud místo toho chcete trvalé uspořádání, kde tento stroj podle plánu stahuje snímky jiné instance do vlastního repozitáře, slouží k tomu záložka **Stažení** na stránce **Instance**.

Kontejner, jehož síť na tomto serveru neexistuje, třeba síť `br0` z Unraidu na běžném Docker hostiteli, ukáže pod svým řádkem výběr sítě. BombVault ho vytvoří ve zvolené síti, spolu s jeho ostatními sítěmi. Pevná IP a MAC adresa patřily ke staré síti a odpadají, obě přidělí nová síť.

## Sada pro obnovu šifrovacího klíče

Toto je díl, který umožňuje zotavení po havárii, i když neběží žádný BombVault.

Jedno kliknutí stáhne **hlavní klíč**, **odvozené heslo restic** a **přesná umístění a příkazy repozitáře**, takže můžete obnovit přímo pomocí restic CLI na libovolném stroji. Připomínka na Přehledu vás popohání, dokud si ji neuložíte.

!!! danger "Uložte sadu pro obnovu mimo server"
    Sada obsahuje tajemství, které dešifruje vaše zálohy. Uchovejte ji na bezpečném místě odděleně od serveru (správce hesel, tištěná kopie v trezoru). Pokud ztratíte jak BombVault, tak `APP_KEY` bez sady pro obnovu, vaše šifrované zálohy nelze obnovit.

!!! warning "Nejnovější snímek není vždy ten, který obnovit"
    Od restic 0.17 ukazuje `restic snapshots` velikost každého snímku. Po ztrátě dat může být nejnovější snímek ten vyprázdněný, proto neobnovujte snímek, který je mnohem menší než ty před ním. Po ransomwaru to může být ten zašifrovaný v obvyklé velikosti. Pokud BombVault ještě běží, podívejte se nejdřív na jeho stránku **Anomálie**: uvádí poslední dobrou zálohu. Obnova nepotřebuje žádná data o anomáliích z BombVault a pozastavení uchovávání vždy jen ponechá více snímků.

### Zapečetění sady

Pokud jste pro prosté exporty zapnuli šifrování age (Nastavení), zapečetí se jím i sada a stáhne se jako `bombvault-recovery-kit.md.age`. Je v ASCII-armored podobě, ne binární, takže je to pořád prostý text: vložení do správce hesel nebo tisk funguje přesně jako dřív, jen je obsah bez vašeho klíče nečitelný.

!!! warning "Klíč age neukládejte do sady"
    K otevření zapečetěné sady potřebujete svůj **soukromý** klíč age. Uložte ho někde, kde nezávisí na sadě samotné, jinak budete obnovovat dvě věci místo jedné. Zapečetění se vyplatí, když sada leží na místě, které plně nemáte pod kontrolou (sdílený správce hesel, poznámky v cloudu, výtisk v kanceláři); sada ve vlastním trezoru je už chráněná trezorem.

    Se zapnutým šifrováním a bez nastaveného použitelného příjemce se stažení rovnou odmítne. BombVault se nikdy neuchýlí k vydání hlavního klíče v otevřené podobě.

### Když sada není po ruce

Heslo není nikde uloženo, **počítá se** z `APP_KEY`. S klíčem a shellem si je tedy dokážete odvodit sami:

```sh
printf 'bombvault:restic-repo' \
  | openssl dgst -sha256 -mac HMAC -macopt hexkey:$APP_KEY -r \
  | cut -d' ' -f1
```

Je to HMAC-SHA256 nad pevným řetězcem `bombvault:restic-repo`, klíčem jsou syrové bajty šestnáctkového `APP_KEY`, výstup je 64 malých šestnáctkových znaků. Stejná hodnota je v sadě jako odvozené heslo restic; tohle je pro den, kdy sada leží jinde než vy.

!!! warning "U přijatého úložiště použijte klíč ODESÍLAJÍCÍ instance"
    Úložiště, které sem přišlo replikací mimo lokalitu, vytvořil stroj, který je odeslal, svým **vlastním** `APP_KEY`. Odvození z klíče přijímajícího stroje dá heslo, které restic odmítne, což vypadá přesně jako poškozené úložiště, aniž by jím bylo. To je obvyklý důvod, proč se `restic check` na přijatém úložišti stále dokola ptá na heslo.

Protože definice pro obnovu žijí **uvnitř** každého repozitáře (`<repo>/def`, `<repo>/vm-def`), je zkopírovaná složka repozitáře plně soběstačná, takže sada plus repozitář jsou vším, co obnova na holém železe potřebuje.

## Získání dumpu databáze zpět {#database-dumps}

Dump databáze je vlastní bod obnovy v repozitáři kontejnerů, se štítkem `dbdump:<container>` a jediným souborem `/dbdump/<container>.sql`. BombVault je vypisuje, stahuje a importuje v sekci **Zálohy**; níže jsou tytéž kroky se samotným resticem, pro den, kdy BombVault k ruce není.

```sh
restic -r <repo> snapshots --tag dbdump:<container>
restic -r <repo> dump --tag dbdump:<container> latest /dbdump/<container>.sql > <container>.sql
```

Štítky `dbversion:` a `dbname:` u každého dumpu říkají, z jaké verze serveru pochází a jaké databáze obsahuje. Úplný soubor končí řádkem `-- PostgreSQL database cluster dump complete` nebo `-- Dump completed`.

Naimportujte ho do kontejneru ve stejné nebo novější verzi (PostgreSQL), případně ve stejné hlavní verzi (MySQL a MariaDB), jednou spuštěného s prázdnou datovou složkou, aby se inicializoval. Hostitel žádného databázového klienta nepotřebuje, kontejner ho má:

```sh
docker exec -i <container> sh -c 'exec psql -X -U "${POSTGRES_USER:-postgres}" -d postgres' < <container>.sql
docker exec -i <container> sh -c 'exec mariadb -uroot -p"$MARIADB_ROOT_PASSWORD"' < <container>.sql
docker exec -i <container> sh -c 'exec mysql -uroot -p"$MYSQL_ROOT_PASSWORD"' < <container>.sql
```

Pro jednu databázi z úplného dumpu berou MySQL a MariaDB `--one-database <name>` v příkazu klienta. Dump PostgreSQL má na každou databázi jednu sekci, každá začíná řádkem `\connect <name>`: zkopírujte tu svou do vlastního souboru a po vytvoření databáze ho naimportujte s `-d <name>`.

!!! warning "Dump pořízený jako root nese uživatele serveru"
    Úplný dump MySQL nebo MariaDB pořízený jako root obsahuje systémovou databázi `mysql`, takže jeho import nahradí účty nového serveru, včetně hesla roota, účty z dumpu. U PostgreSQL je `role ... already exists` pro uživatele, kterého vytvořil kontejner, očekávané a neškodné.
