# Funkce

BombVault je ve výchozím nastavení jednoduchý a hluboký, když to potřebujete. Rozhraní zobrazuje jen to nejnutnější, dokud nepřepnete přepínač **Jednoduché / Pokročilé**. Tato stránka seskupuje kompletní sadu funkcí.

## Rozsah zálohování

| Co | Co se ukládá |
|---|---|
| **Docker kontejnery** | Adresář appdata plus definice kontejneru (image, proměnné prostředí, porty, štítky, svazky). |
| **KVM / libvirt VM** | Diskové image VM, definice XML a UEFI NVRAM (šetrné vypnutí nebo živý snímek, přes SSH). Živé snímky se automaticky vrátí k šetrné záloze, pokud snímek nelze vytvořit, takže záloha VM nikdy jen tak neskončí chybou. |
| **Unraid flash** | Celý USB flash (`/boot`): OS, licence, konfigurace pole, sdílené složky, síť a konfigurace pluginů. Obnova je stažení `.zip` na jedno kliknutí a nikdy nepřepíše živý flash. |
| **Konfigurace aplikace** | Vlastní `/config` BombVaultu (databáze nastavení, přihlašovací údaje mimo lokalitu, pár klíčů SSH pro libvirt), zachyceno pomocí SQLite `VACUUM INTO`, takže databáze v režimu WAL není nikdy zachycena uprostřed zápisu. Obnovováno pomocí sebe-restartu, takže se živá databáze nikdy nepřepisuje pod otevřeným handlem. |
| **Soubory a složky** | Pojmenované **sady souborů**: libovolná složka na serveru (sdílená složka, vaše dokumenty, knihovna fotek), každá s volitelnými vylučovacími vzory pro danou sadu. Plná rovnocennost s ostatními doménami (plány, uchovávání, kopie mimo lokalitu, kontroly integrity a cvičné obnovy). |

## Obnova

- **Plná obnova na jedno kliknutí.** Vyberte snímek, klikněte na Obnovit. Hotovo.
- **Obnova z místní nebo mimo lokalitu.** Každý prohlížeč záloh má přepínač **Místní / Mimo lokalitu**, takže pokud se místní repozitář ztratí nebo poškodí, můžete vypsat a obnovit přímo z repliky mimo lokalitu. Mazání je pro každý zdroj zvlášť: odstranění zálohy ovlivní jen kopii, kterou si prohlížíte.
- **Kontejnery se automaticky přeinstalují.** Definice kontejneru je přehrána proti Docker API, takže se kontejner znovu objeví v záložce Docker v Unraidu přesně tak, jak byl.
- **VM se automaticky znovu vytvoří.** XML je znovu naimportováno přes SSH, takže se VM znovu objeví ve VM Manageru se svým diskem a UEFI NVRAM opět připojenými, i po smazání VM. **Objevit zálohy** znovu sestaví položku, která zcela zmizela (například po čisté instalaci).
- **Individuální obnova.** Obnovte jeden kontejner, jednu VM nebo jednu sadu souborů, aniž byste se dotkli ostatních.
- **Obnova flashe je stažení `.zip`.** Streamuje se do vašeho prohlížeče jako `flash-<id>.zip`, připravená k vložení do Unraid USB creatoru. Živého `/boot` se to nikdy nedotkne.
- **Naplánovaný export flash zipu.** Po každé záloze flashe volitelně zapiš snímek jako prostý `.zip` do složky, kterou zvolíte (jediný přepisovaný `flash-latest.zip` nebo klouzavá historie). Nasměrujte jej na složku Syncthing nebo rclone, aby vaše záloha bootovacího USB opouštěla server automaticky.
- **Předletová kontrola konfliktů.** Než se cokoli zastaví nebo odebere, obnova ověří, že statická IP kontejneru a publikované hostitelské porty jsou volné, a přeruší se s jasnou zprávou místo toho, aby zanechala napůl dokončenou obnovu.
- **Obnova na úrovni souborů.** Rozbalte **Soubory** snímku kontejneru, filtrujte, zaškrtněte libovolný počet souborů a složek a poté obnovte výběr na původní místo nebo do složky, kterou zvolíte.
- **Obnova sady souborů.** Obnovte snímek sady souborů na původní místo (po explicitním potvrzení) nebo do složky, kterou zvolíte, nikdy tiše. Selektivní obnova funguje i zde.
- **Obnova zachovává stav běhu.** Kontejner nebo VM, které běžely při zálohování, se vrátí spuštěné; ty, které byly zastavené, zůstanou zastavené. Zaškrtněte **Ponechat zastavené po obnově** pro znovuvytvoření bez spuštění.
- **Obnova celého stacku.** Kontejnery ze stejného projektu Docker Compose jsou seskupeny do panelu **Stacky**. **Obnovit stack** znovu sestaví každého člena z jeho nejnovější zálohy ponechaného zastaveného a poté je volitelně spustí v pořadí `depends_on`.
- **Živý průběh, zrušení a zpětná vazba o zaneprázdnění.** Dlouhá obnova zobrazuje živý procentuální panel a lze ji zrušit potvrzením zohledňujícím typ. Zrušená obnova se zaznamená jako *zrušená*, nikoli jako neúspěšná.
- **Řízená obnova.** Vyhrazená záložka **Obnova** provede čistou instalaci havarijním případem. Viz [Mimo lokalitu a obnova](offsite-recovery.md).
- **Obnova z jiného BombVault repozitáře.** Jednorázová relace jen pro čtení otevře repozitář jiné instance BombVaultu s `APP_KEY` dané instance, takže můžete přenést kontejner ze serveru A na server B, aniž byste se dotkli vlastního nastavení. Viz [Mimo lokalitu a obnova](offsite-recovery.md).

## Úložiště a plánování

- Inkrementální, deduplikované zálohy přes restic, takže ani velké disky VM repozitář nenafouknou.
- **Cíle:** místní cesta nebo mimo lokalitu. SMB/CIFS a NFS (připojte sdílenou složku v Unraidu a nasměrujte na ni Zálohovací cestu), nativní restic backendy bez rclone (`s3:...`, `rest:http://host:8000/repo`, `b2:...`, `sftp:user@host:/repo`) nebo libovolný rclone remote přes `rclone:<remote>:<bucket>/path`. Všechny přihlašovací údaje se ukládají šifrovaně.
- **SSH cíle nevyžadují nic nainstalovaného na druhé straně.** `sftp:` vyžaduje pouze SSH server, takže i holé Raspberry Pi (bez Dockeru, bez restic) funguje jako cíl mimo lokalitu. Hostitelské klíče se automaticky připnou při prvním kontaktu.
- **Kopie mimo lokalitu (místní + vzdálená).** Ponechte rychlou místní zálohu a přidejte jednu nebo více replik mimo lokalitu, replikovaných pomocí `restic copy` na základě nejlepší snahy (zádrhel mimo lokalitu nikdy nezhatí místní zálohu). Každá doména má vlastní plán mimo lokalitu, plus tlačítko **Replikovat nyní**.
- **Více cílů mimo lokalitu na doménu.** Každá doména (kontejnery, VM, flash, config a sady souborů) může replikovat na několik cílů mimo lokalitu najednou, ne jen na jeden. Přidejte další cíle v záložce Mimo lokalitu, každý s vlastním repozitářem, třídou úložiště S3, příznakem append-only, uchováváním a rozpočtem růstu. Vaše stávající kopie mimo lokalitu se přenese jako první cíl, takže se nic nezmění, dokud nepřidáte druhý, a každý cíl domény replikuje podle plánu mimo lokalitu dané domény.
- **Ruční pořadí zálohování.** Nastavte přesné pořadí, ve kterém se vaše kontejnery zálohují, z panelu pořadí zálohování na stránce Kontejnery. Naplánované běhy a běhy s vícenásobným výběrem se jím řídí; každý neuspořádaný kontejner zachovává předchozí chování od nejvíce po termínu a záloha jediného kontejneru je nezměněná.
- **Konfigurovatelné uchovávání:** keep-last / denní / týdenní / měsíční, prořezáváno automaticky po každé záloze, nastavené **pro každý zdroj** (místní vedle zálohovacích cest, mimo lokalitu v záložce Mimo lokalitu, takže můžete kopie mimo lokalitu držet déle jako archiv).
- Plánování na doménu (denní / týdenní včetně vícedenních sad / každých N dní / raw cron), vše upravováno na jednom místě v **Nastavení, Plány**.
- **Limity šířky pásma mimo lokalitu.** Omezte rychlost nahrávání/stahování restic, aby replikace nezasytila vaše WAN.
- **Studená a archivní třída úložiště (S3).** Pro nativní S3 repozitář mimo lokalitu můžete zvolit třídu úložiště, omezenou na úrovně čitelné pro obnovu (Standard, Standard-IA, One Zone-IA, Intelligent-Tiering, Glacier Instant Retrieval), takže archivní cena nikdy tiše nerozbije obnovu. Úrovně hlubokého archivu, které nejprve potřebují asynchronní rozmražení (Glacier Flexible, Deep Archive), jsou záměrně vynechány. Pouze nativní S3 backendy; rclone remotes nastavují svou třídu v konfiguraci rclone.
- **Zálohovací složky zůstávají kopírovatelné mimo stroj.** Po každé záloze BombVault uvolní strom místního repozitáře na složky `0755` / soubory `0644` (repozitáře jsou šifrované, takže nic není odhaleno), aby synchronizační uživatel bez root přes SMB nezůstal zamčen venku. Definice pro obnovu žijí uvnitř každého repozitáře, takže zkopírovaná složka repozitáře je plně soběstačná.

## Přehled, ověření a monitorování

- **Stav ochrany (RPO).** Přehled zobrazuje zelený / oranžový / červený indikátor na doménu, porovnávající poslední úspěšnou zálohu s jejím plánem, takže záloha po termínu zčervená místo toho, aby se skryla v logu.
- **Heatmapa stavu záloh.** Kalendář ve stylu příspěvků na GitHubu s denními výsledky záloh na doménu, s přepínačem Kontejnery / VM / Flash / Config / Soubory.
- **Časování běhů všude.** Každá položka historie běhů uvádí `start, end (duration)` a každý kontejner a VM nese vlastní seznam **Nedávné běhy** na své stránce.
- **Přehled, který si můžete přeuspořádat.** Přepněte režim přizpůsobení a přetáhněte karty do svého pořadí a skryjte ty, které nepotřebujete. Rozvržení se ukládá pro každý prohlížeč.
- **Trend velikosti repozitáře a deduplikace.** Aktuální velikost repozitáře, poměr deduplikace a počet snímků na doménu, se sparkline růstu úložiště.
- **Cvičné obnovy s ověřením.** BombVault pravidelně dokazuje, že vaše zálohy jsou obnovitelné (`restic check --read-data-subset`, omezené) a zobrazuje odznak *naposledy ověřeno jako obnovitelné* na doménu.
- **Samohojící operace.** Prokazatelně osiřelý restic zámek (zanechaný restartem uprostřed operace) je automaticky násilně vyčištěn a jednou zopakován. Uchovávání je stabilní vůči identitě (prořezáváno na položku, imunní vůči změnám cesty nebo hostitele) a selhání uchovávání odešle oznámení.
- **Sada pro obnovu šifrovacího klíče.** Stažení hlavního klíče na jedno kliknutí, odvozené heslo restic a přesná umístění a příkazy repozitáře, takže můžete obnovit bez běžícího BombVaultu. Viz [Mimo lokalitu a obnova](offsite-recovery.md).
- **Export a import vašich nastavení.** Karta *Export a import nastavení* na stránce Nastavení zapíše celou vaši konfiguraci (nastavení domén, cíle mimo lokalitu, plány, uchovávání, oznámení) do přenosného souboru JSON, takže přechod na nový stroj nebo klonování sestavy neznamená znovu vše zadávat ručně. Vy zvolíte, zda zahrnout přihlašovací údaje mimo lokalitu a oznámení; s nimi je soubor stejně citlivý jako vaše sada pro obnovu. Import zobrazí náhled a požádá o potvrzení a nikdy se nedotkne vašich zálohovaných dat ani historie.
- **Oznámení.** Webhook (Discord / Slack / Gotify / ntfy), Matrix, Healthchecks.io, e-mail (SMTP), self-hostovaný server [Apprise API](https://github.com/caronc/apprise-api) a nativní systém oznámení Unraidu. Zásada na zálohu: nikdy / při selhání / vždy. Naplánovaný běh mnoha položek může odeslat jeden souhrn *N z M uspělo*. Healthchecks dostane celý životní cyklus (`/start`, poté úspěch nebo `/fail`), kdykoli je nastavena URL.
- **Prometheus `/metrics`.** Volitelné (výchozí vypnuto, volitelný bearer token) pro Grafana nebo Uptime Kuma. Zpřístupňuje stav záloh, velikosti a časové značky, bez tajemství nebo cest ve štítcích.

## Ochrana proti ransomwaru

- **Neměnné (append-only) mimo lokalitu.** Označte repozitář mimo lokalitu jako append-only, aby ransomware nebo kompromitovaný hostitel nemohl smazat nebo přepsat vaše zálohy. Druhá strana (`restic/rest-server` v režimu `--append-only`) to vynucuje; BombVault to pouze ověřuje a nikdy nezobrazí zelenou jen na základě konfiguračního tvrzení.
- **Test odolnosti proti manipulaci.** BombVault pravidelně dokazuje záruku append-only tím, že skutečně zkusí mazání proti repozitáři mimo lokalitu (zaměřené na neexistující objekt): odmítnuto znamená chráněno, přijato znamená nechráněno. Neprůkazný výsledek nikdy nezmění uložený verdikt.
- **Řízené nastavení mimo lokalitu.** Průvodce vás provede od volby backendu přes připravený úryvek pro nasazení rest-serveru, test připojení, přepínač neměnnosti a strategii uchovávání.
- **Cvičné obnovy po havárii (mimo lokalitu).** Obnovte skutečný cíl z repozitáře mimo lokalitu do jednorázového sandboxu, ověřte jej soubor po souboru a bajt po bajtu, poté ukliďte. Viz [Mimo lokalitu a obnova](offsite-recovery.md).
- **Vysvědčení ochrany proti ransomwaru.** Karta na Přehledu se zeleným / oranžovým / červeným postojem na doménu a kontrolním seznamem s věkovou značkou; každý červený řádek odkazuje přímo na opravu. Zelená se rozsvítí jen na ověřených faktech.
- **Alarm rozpočtu růstu.** Pro neměnné mimo lokalitu (kde se staré snímky záměrně nikdy neprořezávají) nastavte rozpočet velikosti a nechte se upozornit dříve, než se to vymkne kontrole.
- **Řídicí panel příjemce (přijímací strana).** Na stroji, který přijímá neměnné kopie mimo lokalitu z jiného BombVaultu, zapněte přepínač **Příjemce** (Nastavení) k odhalení záložky **Příjemce**. Zaregistrujte přijatý repozitář jen pro čtení (otevřený klíčem odesílající instance) pro zobrazení jeho inventáře snímků seskupeného podle zdroje, kdy každý zdroj naposledy dorazil, a spusťte nezávislý `restic check` na přijímacím hardwaru. Upozorní vás, když zdroj přestane odesílat v okně, které nastavíte (pojistka mrtvého muže), nebo když kontrola integrity selže. Striktně jen pro čtení, takže nikdy nezapisuje do přijatého repozitáře, a ve výchozím stavu vypnuto. Viz [Mimo lokalitu a obnova](offsite-recovery.md).

## Prosté exporty

- **Prostý export kontejneru.** Tlačítko **Export** na kontejner zapíše procházitelnou kopii bez nástrojů vedle repozitáře: `<name>.tar.gz` zálohovacích složek plus Unraid šablonu `<name>.xml`. Restic zůstává enginem; toto je kopie navíc pro pohodlí.
- **Prostý export VM.** VM mají stejný **Export (prostý tar)**: `<name>.tar.gz` diskových image plus `<name>.xml`, obnovitelný pomocí `virsh define` plus disk, bez BombVaultu nebo restic.
- **Šifrujte prosté exporty (age).** Exporty leží mimo restic, takže jsou ve výchozím stavu prostým textem. Zapněte šifrování age v Nastavení a přidejte jednoho nebo více příjemců (veřejný klíč age nebo veřejný klíč SSH). Každý export (kontejner a VM `.tar.gz`, jejich `.xml` sidecary a flash ZIP) je pak zapečetěn pro tyto příjemce a vy jej později dešifrujete mimo stroj odpovídajícím soukromým klíčem. Jako bezpečnostní pravidlo, se zapnutým šifrováním a bez nastaveného platného příjemce export selže s jasnou chybou místo toho, aby kdy zapsal prostý text.

## Ostatní

- **Zálohujte mnoho najednou.** Vyberte více kontejnerů a klikněte na **Zálohovat vybrané**. Dávka běží na straně serveru, takže pokračuje, i když zavřete kartu nebo ztratíte připojení. BombVault nikdy nezálohuje (a tedy nikdy nezastavuje) vlastní kontejner.
- **Prohlížeč snímků** se seznamem bodů obnovení, mazáním jednotlivých snímků a sbalitelným stromem složek pro obnovu na úrovni souborů.
- **Údržba repozitáře na doménu:** **Ověřit** (`restic check`), **Odemknout** (vyčistit zaseklý zámek) a **Vyčistit** (aplikuje zásadu uchovávání na vyžádání, když je nějaká nastavena, jinak prosté uvolnění místa).
- **Pre/post-zálohovací hooky na kontejner.** Shellové příkazy běží uvnitř kontejneru (například `mysqldump` do appdata před zálohou); selhání pre-hooku zruší zálohu.
- **Zastavte ostatní kontejnery během zálohy, s restartem řízeným podle stavu health.** Pojmenujte závislé kontejnery (například databázi), které se mají zastavit, dokud se tento zálohuje. Poté je BombVault vrátí zpět v pořadí `depends_on` z Compose a ve výchozím nastavení čeká, až každý ohlásí stav healthy (nebo running, pokud nemá healthcheck), než spustí kontejnery, které na něm závisí, takže závislost jako Pi-hole, databáze nebo VPN brána je skutečně v provozu dříve než služby, které ji potřebují, místo toho, aby ty vracely *connection refused*. Čekání je omezeno timeoutem na kontejner (výchozí 120 sekund), takže pomalý nebo nikdy zdravý kontejner nemůže běh zaseknout; jak čekání, tak timeout žijí v Nastavení, Plány (vypněte čekání pro předchozí restart všech najednou). Stejný uspořádaný restart řízený podle stavu health také obaluje aktualizaci image po záloze, takže v den, kdy dorazí aktualizace, jsou závislé kontejnery drženy dole po celé znovuvytvoření a vráceny zpět, řízené podle stavu health, teprve až je hotovo.
- **Vylučovací vzory na kontejner.** Vypište podadresáře k přeskočení uvnitř zálohovaného svazku, jeden na řádek. Napište cesty tak, jak je vidíte uvnitř kontejneru; živý náhled ukazuje, na co se každý řádek vyhodnotí, a varuje, když by řádek nic nevyloučil.
- **Aktualizovat po úspěšné záloze (pokročilé, ve výchozím stavu vypnuto).** Zapněte to na kontejneru a BombVault stáhne nejnovější image a znovu jej vytvoří, ale jen když skutečně existuje novější image, takže nejprve vždy existuje čerstvý bod obnovení. Volitelné extra: oznámení na aktualizovaný kontejner a úklid image (základní image sdílený jinými kontejnery se nikdy nesmaže). Po aktualizaci BombVault také požádá Unraid, aby znovu zkontroloval stav aktualizace daného kontejneru, takže se zastaralý banner *update available* v záložce Docker sám vyčistí místo toho, aby přetrvával (aktualizace Unraidu jdou přímo přes Docker API, takže jeho cachovaný stav a na některých verzích cachovaný digest by jinak banner nadále zobrazovaly). Je to na základě nejlepší snahy, nikdy neovlivní zálohu, ve výchozím stavu zapnuto a má přepínač v Nastavení.
- **Obnova do alternativní složky** pro klonování nebo inspekci.
- **Rozdíl snímků a štítky.** Porovnejte dva snímky a zjistěte, co se změnilo, a označte snímky štítky pro jejich filtrování.
- **Co je nového po aktualizaci.** Poznámky k vydání vyskočí jednou na novou verzi, poskytované z poznámek zabudovaných v binárce, takže dialog funguje offline.
- **HTTPS rovnou z krabice** (samopodepsaný, nebo přineste si vlastní certifikát za reverzní proxy).
- **Docker healthcheck.** Kontejner hlásí healthy/unhealthy ze svého vlastního `/api/health`, takže ho nástroj pro automatické hojení může restartovat, pokud se engine kdy zasekne.
- **Tmavé/světlé UI v 42 jazycích** s výběrem vlajky.
