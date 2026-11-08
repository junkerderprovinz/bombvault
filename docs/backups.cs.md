# Zálohy a položky

Všechno, co BombVault zálohuje, stojí v jednom seznamu pod **Zálohy**: kontejnery, VM, sady složek, flashka, datové sady ZFS a autozáloha. Každá z nich je **položka** s vlastní stránkou. Tahle stránka projde seznam, stránku položky a její tři záložky a nastavení, která položka může mít.

![Seznam Záloh s vybranou dlaždicí Kontejnery: každý řádek ukazuje své poslední běhy jako pruh a jeden stav.](assets/screenshots/backups/list-dark.png#only-dark)
![Seznam Záloh s vybranou dlaždicí Kontejnery: každý řádek ukazuje své poslední běhy jako pruh a jeden stav.](assets/screenshots/backups/list-light.png#only-light)

## Co každá sekce ukládá {#what-is-saved}

| Sekce | Co se ukládá |
|---|---|
| **Kontejnery** | Složka appdata plus definice kontejneru: image, proměnné, porty, štítky a svazky. Karta **Zálohované složky** na záložce **Nastavení** kontejneru zaškrtává přesně ty složky, které záloha pokrývá, s živým počtem cest a seznamem toho, co jsi vynechal. Data, která kontejner drží mimo Host Data, například v ixVolume na TrueNAS nebo v projektu Compose pod `/opt`, se nezálohují a běh jmenuje složky, které vynechal. Běh jmenuje i složku uvnitř Host Data, která tam právě teď není. Systémové cesty jako `/etc/localtime` nebo socket Dockeru se za data nepočítají. |
| **VM** | Obrazy disků, definice XML a UEFI NVRAM, s řádným vypnutím nebo živým snímkem, přes SSH. Viz [Zálohy VM](vm-backup-ssh-setup.md). |
| **Flash** | Jen Unraid. Celá flashka USB (`/boot`): OS, licence, konfigurace pole, sdílené složky, síť a konfigurace pluginů. |
| **Autozáloha** | Vlastní `/config` BombVaultu: jeho databáze nastavení, přihlašovací údaje cílů, pár klíčů SSH pro libvirt, klíč a připnuté klíče hostitelů serverů ZFS a certifikáty, zachycené pomocí SQLite `VACUUM INTO`, aby se databáze nikdy nezachytila uprostřed zápisu. |
| **Složky** | Libovolná složka na serveru jako sada složek, třeba sdílená složka, tvoje dokumenty nebo knihovna fotek, s volitelnými vzory vyloučení. Sady složek mají plány, uchovávání, kopie mimo lokalitu, kontroly a zkoušky obnovy jako všechno ostatní. |
| **Datové sady ZFS** | Datová sada a každá pod ní, čtené z jednoho snapshotu ZFS, takže všechny pocházejí ze stejného okamžiku, a uložené resticem jako složka. Položka ZFS může navíc držet blokově přesnou repliku na druhém serveru ZFS. Viz [Datové sady ZFS](zfs-datasets.md). |

Každá záloha jde přes restic, takže je inkrementální a deduplikovaná a ani velké disky VM repozitář nenafouknou. BombVault nikdy nezálohuje, a tedy ani nezastavuje, svůj vlastní kontejner. Běh lze bezpečně zastavit, protože restic zapisuje snapshot jako poslední, takže přerušený běh nechá po sobě nereferencovaná data a žádný snapshot. Zámek, který po sobě nechal restart uprostřed běhu, se uvolní a běh se zkusí ještě jednou, až když zámek deset minut nebyl obnoven.

## Seznam {#list}

Dlaždice nad seznamem vybírají druh, například **Kontejnery** nebo **ZFS**, a každá dlaždice počítá své položky. Dlaždice vybírá, zbytek stránky neskrývá.

Každý řádek ukazuje posledních čtrnáct běhů položky jako malé čtverečky a jeden stav, například **Chráněno**, **Není chráněno**, **Odloženo** nebo **Replikuje se**. Vše ostatní o ní je v informační bublině vedle jejího jména a když ukazatel spočine na řádku, ten text stojí přímo v něm. Značka vedle jména kontejneru říká, že byl od poslední zálohy znovu vytvořen s jiným image, jinými porty, proměnnými nebo svazky. S příští zálohou zmizí.

Tři tlačítka nad seznamem ho zužují:

- **Filtry**: podle **Plán** (**Vše**, **Naplánováno**, **Pozastaveno**), podle **Záloha** (**Zálohováno**, **Nikdy nezálohováno**) a u **Kontejnery a VM** podle **Nainstalované** nebo **Není nainstalováno**.
- **Řazení**: podle jména, stavu, IP, poslední zálohy, velikosti nebo plánu. Každé pole kromě IP řadí oběma směry, například **Velikost, největší první** a **Velikost, nejmenší první**.
- **Hledat**: podle jména, image nebo IP.

Položky, které už nejsou nainstalované, ale mají ještě zálohy, stojí stranou pod **Není nainstalováno (pouze zálohy)**. Obnov je, nebo smaž jejich zálohy, aby se uvolnilo místo.

Hlavní akce stránky plave vpravo dole: **Zálohovat vše nyní** spustí běh přes všechny zapnuté sekce. **Více** drží zbytek:

- **Vybrat** zahájí výběr, viz níže.
- **Objevit zálohy** přečte, co cíle drží, a z toho znovu postaví položky, například po čerstvé instalaci. Kontejner bez vlastních dat nemá žádný snapshot a vrátí se z definice zrcadlené vedle repozitáře. Kopie mimo lokalitu znovu postavených sekcí pak čekají, dokud jejich zálohy nepřevezmeš jako vlastní zálohy tohoto serveru, viz [Po opětovném sestavení](offsite-recovery.md#after-a-rebuild).
- **Připojení ZFS k tomuto serveru** otevře test SSH sekce ZFS, viz [Datové sady ZFS](zfs-datasets.md#requirements).
- **Replikovat vše teď** pošle s vybranou dlaždicí ZFS repliku každé položky ZFS.

![Stejný seznam v telefonu.](assets/screenshots/backups/list-phone-dark.png#only-dark)
![Stejný seznam v telefonu.](assets/screenshots/backups/list-phone-light.png#only-light)

## Výběr více položek {#select}

Řádky vybíráš tak, jak vybíráš soubory ve správci souborů: Ctrl+klik, Shift+klik, Ctrl+A, nebo dlouhý stisk v telefonu. Vybraný řádek ukazuje fajfku místo ikony a lišta nad seznamem je počítá a nabízí **Zálohovat vybrané**, **Pozastavit plán** a **Obnovit plán**, **Vybrat vše** a **Zrušit výběr**. Každý řádek má také nabídku na pravé tlačítko s vlastními akcemi.

Výběr se zálohuje na serveru, položku po položce, takže pokračuje, i když zavřeš kartu. Chceš-li obnovit více položek společně, použij [stránku Obnova](restore.md#several).

![Dva vybrané kontejnery s lištou nad seznamem.](assets/screenshots/backups/bulk-dark.png#only-dark)
![Dva vybrané kontejnery s lištou nad seznamem.](assets/screenshots/backups/bulk-light.png#only-light)

## Přidání složky nebo datové sady ZFS {#add}

Kontejnery, VM a flashka se objeví samy, jakmile je jejich sekce zapnutá v **Nastavení, Obecné**. Složky a datové sady ZFS vybíráš sám, protože BombVault nemůže hádat, které myslíš. **Přidat** otevře **Co má BombVault zálohovat navíc?**:

- **Jedna složka** vypíše sdílené složky, které BombVault našel, a vezme libovolnou jinou složku pod Host Data. Dej sadě složek jméno, například `Documents`. Po první záloze je jméno pevné, protože pod ním jsou uložené snapshoty.
- **Datová sada ZFS** vypíše datové sady tohoto serveru. Vyber tu na vrcholu toho, co chceš zálohovat, viz [Položky a podřízené datové sady](zfs-datasets.md#items-and-children).
- **Konfigurace systému** se objeví na hostiteli bez Unraidu, jehož Host Data sahají na `/etc`. Je to výchozí sada složky `/etc`, kterou před uložením zkontroluješ.

## Vynechání položky {#dont-back-up}

Položku, kterou vynecháš schválně, například testovací kontejner, lze označit **Nezálohovat**. Pak se přestane počítat jako nechráněná a **Přestat vylučovat** ji vrátí. **Pozastavit plán** je jemnější cesta: položka opustí plán i úplnou zálohu, nese šedý štítek **Plán pozastaven** a **Zálohovat nyní** pořád funguje. Nastavuje stejný přepínač jako **V plánu** na záložce Nastavení položky.

## Stránka položky {#entry-page}

Klik na řádek otevře stránku položky. Hlavička jmenuje položku a její stav v jednom řádku, například **Naposledy zálohováno před 3 hodinami** nebo **Odloženo, další pokus v 03:15**, a drží její akce: **Zálohovat nyní**, **Obnova** a pod **Více** ty vzácné, například **Exportovat nejnovější zálohu**, **Exportovat kontejner** nebo **Exportovat VM**, **Propojit s dřívější položkou**, **Smazat všechny zálohy**, **Odebrat položku** a **Smazat zálohy všude**. Tlačítko zpět vlevo od ikony vede do seznamu. Zatímco záloha běží, vedle jejího průběhu stojí **Zrušit zálohu** a zrušený běh se zapíše jako zrušený, ne jako neúspěšný.

**Smazat zálohy všude** odstraní zálohy položky ve všech cílech najednou, včetně kopií mimo lokalitu. Okno vypíše každé místo s počtem záloh, které na něm jsou, a tlačítko se odemkne, až napíšeš název položky. Místo nastavené jako append-only si své zálohy ponechá a řekne to. Položka a její nastavení zůstanou, takže položka, kterou už nezálohuješ, tak zůstane a zmizí jen její zálohy a jejich historie.

Stránka má tři záložky: **Zálohy**, **Nastavení** a **Ověření**.

### Záložka Zálohy {#backups-tab}

Tato záložka vypisuje snapshoty položky, od nejnovějšího, s pruhem za každý den. Záloha, která leží na více místech, se objeví jednou, označená každým cílem, který ji drží. Klik na snapshot ho obnoví a klik na místo uvnitř snapshotu ho obnoví přesně odtamtud.

- **Cíle** jmenují každé místo se zálohami této položky a kolik jich je. Cíle mimo lokalitu se čtou, až když stiskneš **Kontrola**, protože výpis cloudového úložiště chvíli trvá. Místo, kam položka už nezálohuje, stojí jako **Jen starší zálohy** a lze z něj dál obnovovat.
- Každý snapshot má nabídku: **Přidat štítek** nebo **Změnit štítek**, **Porovnat s jinou zálohou**, která vypíše soubory, jež přišly, zmizely nebo se změnily, **Export** jako tar, zip nebo vlastní repozitář resticu a smazání jen na jednom místě.
- **Výpisy databáze** kontejneru stojí ve vlastním seznamu, se stažením, uložením do složky a importem.

Kolik snapshotů zůstane, nastavuje cíl, viz [Jak dlouho uchovávat](offsite-recovery.md#keep).

![Záložka Zálohy kontejneru: pruh za každý den a cíle, které drží jednotlivé snapshoty.](assets/screenshots/backups/entry-backups-dark.png#only-dark)
![Záložka Zálohy kontejneru: pruh za každý den a cíle, které drží jednotlivé snapshoty.](assets/screenshots/backups/entry-backups-light.png#only-light)

### Záložka Nastavení {#settings-tab}

Každá položka začíná kartou **Plán a cíl**:

- **V plánu** bere položku do plánovaných běhů nebo z nich.
- **Vlastní plán** dává této jedné položce plán jen pro ni, viz [Vlastní plán pro jednu položku](schedules.md#own-plan). Bez něj běží na plánu své sekce nebo v úplné záloze.
- Řada tlačítek, **Místní** a jedno za každý cíl mimo lokalitu, říká, kam zálohy jdou. **Řídí se výchozím nastavením** znamená, že položka jde tam, kam její sekce, a klik na to otevře **Cíle podle sekcí**. Viz [Umístění podle položky](offsite-recovery.md#placement).
- **Repozitář** jmenuje, kam se zálohy zapisují (**Uloženo na**). Toto místo je od první zálohy pevné, kopie se mohou měnit kdykoli.

Karty níže závisejí na druhu položky:

| Druh | Karty |
|---|---|
| Kontejner | **Zálohované složky** (které namapované složky se zálohují, s **Přeskočit složky mezipaměti (CACHEDIR.TAG)**), **Vzory vyloučení**, **Zastavit ostatní kontejnery**, **Příkazy před a po**, **Databáze**, **Když je aplikace zaneprázdněná**, **Aktualizovat po úspěšné záloze**, **Snímek místo zastavení**, **Dřívější názvy** |
| VM | **VM** s **Metoda** (**Graceful (vypnutí)** nebo **Živý snímek**) a **Jen změněné bloky**, viz [průvodce zálohou VM](vm-backup-ssh-setup.md#per-vm-method) |
| Sada složek | **Složka** a **Vzory vyloučení** |
| Flashka | **Export Flash do zip**, který zapíše každou zálohu flashky jako obyčejné `.zip` do složky, kterou vybereš, v jednom kroku, takže synchronizační nástroj nikdy neuvidí půlku souboru. Záloha flashky, která obsahuje méně, než by měla, ponechá poslední ZIP na místě a běh exportu to řekne. |
| Datová sada ZFS | podřízené datové sady, kontejnery k zastavení a příkazy kolem snapshotu, **Replika** a **Vyzkoušet přístup ke snímkům**, viz [Datové sady ZFS](zfs-datasets.md) |

Nastavení se ukládají samy, jak je měníš.

![Záložka Nastavení kontejneru s Vlastním plánem v první kartě.](assets/screenshots/backups/entry-settings-dark.png#only-dark)
![Záložka Nastavení kontejneru s Vlastním plánem v první kartě.](assets/screenshots/backups/entry-settings-light.png#only-light)

### Záložka Ověření {#verify-tab}

- **Kontroly** ukazují kontrolu obnovy, která následuje po první záloze. **Ověřit obnovu** ji spustí znovu na nejnovější záloze: BombVault obnoví vzorek až 100 souborů a 256 MiB do dočasné složky a nechá restic přečíst každý soubor zpět proti jeho hashům.
- **Test spuštění** u kontejneru obnoví nejnovější zálohu do izolované kopie a spustí ji: jméno začínající `bombvault-test-`, vlastní interní síť Dockeru bez publikovaných portů a bez cesty do LAN, 1 CPU a 2 GiB paměti. Projde, když kopie hlásí, že je zdravá, odpovídá na svém prvním vystaveném portu, nebo když při absenci obojího prostě běží. Originál se nikdy nedotkne a kopie, její síť i data se potom odstraní, i po restartu uprostřed testu. Kontejnery v síti hostitele, privilegované, se zařízeními a ty, které potřebují jiný kontejner, se ukazují jako netestovatelné. **Test spuštění** pod **Nastavení, Ověření, Automatické kontroly** testuje jeden kontejner na běh, a to ten, který byl testován nejdéle před tím.
- **Anomálie** nastavují citlivost a úroveň oznámení této položky a ukazují její otevřené nálezy.
- **Velikost podle složek** ukazuje, co zabírá místo v nejnovější záloze a kolik z toho přinesla poslední záloha, vždy o jednu úroveň.
- **Nedávné běhy** vypisují poslední běhy s jejich začátkem, koncem a délkou. Klik ukáže podrobnosti.

![Záložka Ověření kontejneru s otevřenou Velikostí podle složek.](assets/screenshots/backups/entry-verify-dark.png#only-dark)
![Záložka Ověření kontejneru s otevřenou Velikostí podle složek.](assets/screenshots/backups/entry-verify-light.png#only-light)

## Vzory vyloučení {#excludes}

Karta **Vzory vyloučení** bere jeden vzor na řádek. Piš cesty tak, jak je vidíš uvnitř kontejneru, například `/config/Library/Application Support/Plex Media Server/Cache`, nebo holé jméno jako `.git`, které odpovídá v jakékoli hloubce. U sady složek piš cestu tak, jak ji zná hostitel, například `/mnt/user/data/cache`. Náhled pod polem ukazuje, na co se každý řádek rozloží, a upozorní, když řádek neodpovídá ničemu nebo by vyloučil všechno. Řádek `/` nebo `*` či ten, který jmenuje celou zálohovanou složku, se ignoruje, takže vyloučení nikdy nemůže zálohu vyprázdnit. Chceš-li vynechat celou složku, odškrtni ji raději pod **Zálohované složky**.

U známých image, jako jsou Plex, Jellyfin, Sonarr, Radarr, Immich a Nextcloud, karta nabízí složky, které aplikace sama znovu naplní, například mezipaměti, logy a náhledy. Nic se nevyloučí, dokud nestiskneš **Vyloučit vybrané**.

Stejná karta nese upozornění, která prohlídka složek nevidí. Immich například drží alba, obličeje a data v databázi PostgreSQL v samostatném kontejneru, takže souborová záloha samotného kontejneru Immich obnoví obrázky bez toho všeho a obnova vypadá, že se povedla. Zálohuj i kontejner s databází.

## Výpisy databází {#database-dumps}

Kontejnery PostgreSQL, MySQL a MariaDB, které BombVault pozná, se před každou zálohou vypíšou z běžícího serveru do vlastního bodu obnovy vedle souborů. To pokrývá oficiální image, PostGIS, TimescaleDB, pgvector, pgautoupgrade, databázové image Immich, image MariaDB od linuxserver, yobasystems a jc21 a mysql-server od Oraclu. Kontejner, který jen vypadá jako databáze, dostane stejnou volbu, vypnutou, dokud ji nevybereš.

Výpis teče přímo do repozitáře a nikdy se nezapíše na disk. Přihlašovací údaje pocházejí z vlastních proměnných kontejneru, včetně tajemství `*_FILE`, a nikdy ho neopustí. Karta **Databáze** na záložce Nastavení kontejneru říká, zda se datová složka ukládá při zastaveném kontejneru, kopíruje za běhu, nebo neukládá, a zapíná či vypíná výpis pro tento kontejner. Štítek `bombvault.dbdump=false` dělá totéž a má přednost před kartou. **Automatické výpisy databází** v **Nastavení, Obecné** to přepíná pro všechny.

Neúspěšný výpis nezpůsobí selhání zálohy. Objeví se jako vlastní neúspěšný běh s důvodem a pošle oznámení. Výpisy se nikdy nenahrávají zpět samy: stáhni jeden, obyčejný nebo komprimovaný, ulož ho do složky, nebo ho jedním klikem na záložce **Zálohy** naimportuj do čerstvě spuštěné databáze. Jak výpis dostat zpět bez BombVaultu, najdeš v [Jak dostat zpět výpis databáze](offsite-recovery.md#database-dumps). Když jeden selže, viz [Výpis databáze selhal](troubleshooting.md#a-database-dump-failed).

## Příkazy a jiné kontejnery {#commands}

- **Příkazy před a po** běží uvnitř kontejneru přes `sh -c`, například aby vyprázdnily mezipaměť na disk. Příkaz před zálohou, který selže, zálohu přeruší. Příkaz před zálohou, který skončí s kódem 75, plánovanou zálohu místo toho odloží, viz níže.
- **Zastavit ostatní kontejnery** jmenuje kontejnery, které se mají zastavit, zatímco se zálohuje tento, třeba jeho databáze. Potom se spustí znovu v pořadí podle `depends_on` v Compose a ve výchozím stavu musí každý hlásit, že je zdravý, než nastartuje další. Čekání a jeho časový limit jsou pod **Restart po záloze** na kartě **Pořadí** v **Nastavení, Plán**.
- Kontejner, který sdílí síťový, PID nebo IPC namespace s kontejnerem, který se zastavuje, přes `--net=container:` nebo `network_mode: service:` v Compose, se zastaví a spustí s ním, i když není na seznamu, a spustí se až po kontejneru, ke kterému se připojuje. Restartovaný kontejner dostane novou síť a ten, který by zůstal běžet ve staré síti, by měl jen svůj loopback. Samotný seznam zůstane, jak jsi ho nastavil. Pokud takový kontejner nešlo zastavit, běh řekne, že nemá síť, dokud se nerestartuje. Totéž platí pro vlastní kontejner BombVaultu, který se nikdy sám nezastaví a nelze ho dát na seznam.
- Kontejner, který potom znovu neběží, jmenuje běh i oznámení, i když samotná záloha selhala.

## Když je aplikace zaneprázdněná {#app-busy}

Záloha zastavuje svůj kontejner, takže plánovaná může počkat, zatímco někdo aplikaci používá, a sama to zkusit znovu. Nastavuje se to na kartě **Když je aplikace zaneprázdněná** na záložce Nastavení kontejneru:

- **Kontrola aktivity** se před plánovanou zálohou zeptá Plexu, Jellyfinu nebo Emby, zda se někdo dívá. Plex se ptá s tokenem z vlastního `Preferences.xml`, takže není co zadávat. Jellyfin a Emby berou **Klíč API**, vytvořený na dashboardu serveru pod API keys.
- **Ignorovat pozastavené relace po (min)** nechá film pozastavený na noc přestat se počítat.
- **Zálohovat, když server neodpovídá** rozhoduje, co se stane, když se server nedá zeptat: zapnuto, záloha proběhne a log to řekne. Vypnuto, čeká, jako by se někdo díval.
- **Čekat, dokud kontejner odesílá data** je pro aplikace bez takové kontroly, například herní server nebo stahovač. Záloha čeká, dokud kontejner při jakémkoli odečtu za posledních **V klidu po dobu (min)** minut odesílal víc než **Zaneprázdněný od (KiB/s)**.
- **Zkusit znovu každých (min)**, **Zkoušet jen v časovém okně** s **Zkoušet mezi** a **Čekat nejvýš (hodin)** nastavují, jak pokusy probíhají.
- **Na konci čekání** buď to vzdá, takže se záloha ukáže pod **Vyžaduje pozornost** s **Zkusit znovu**, nebo **Přesto zálohovat**.

Čekající záloha se zapíše jako **Odloženo**, jantarově, a zbytek běhu pokračuje. Přehled, seznam i položka ji ukazují s dalším pokusem a upozornění na zpoždění i stav ochrany na ni čekají do její lhůty. Dostaneš jedno oznámení, když se odloží, a jedno, když skončí, ne jedno za pokus. Pokus počká, zatímco běží jiná záloha, a čekání po restartu pokračuje. Záloha, kterou spustíš sám, vždy běží hned, ať už přes **Zálohovat nyní**, **Zkusit znovu**, výběr, API nebo MCP, a ta, která projde, ukončí čekání, které bylo v běhu.

Záložka Nastavení kontejneru má i starší, jednodušší cestu: **Počkat, až bude aplikace v klidu** s **Čekat nejvýš (hodin)**. Záloha začne, jakmile je aplikace v klidu, což definuje karta **Klid před zálohou** v **Nastavení, Plán**. Členové zásobníku Compose, kteří přijdou na řadu ve stejném běhu, čekají společně a čekající záloha nedrží žádný zámek, takže ostatní kontejnery pokračují.

![Karta Když je aplikace zaneprázdněná u kontejneru Plex s nastavenou kontrolou aktivity.](assets/screenshots/backups/app-busy-dark.png#only-dark)
![Karta Když je aplikace zaneprázdněná u kontejneru Plex s nastavenou kontrolou aktivity.](assets/screenshots/backups/app-busy-light.png#only-light)

### Příklad: Plex počká, dokud se někdo dívá {#example-plex}

Úplná záloha běží každou noc ve 03:00. Nikdo by neměl vidět, že se film zastavil, protože Plex spadl kvůli záloze, ale záloha se přesto má stát před ránem.

1. Otevři **Zálohy**, pak položku `plex` a její záložku **Nastavení**.
2. V **Když je aplikace zaneprázdněná** nastav **Kontrola aktivity** na Plex. Není co psát.
3. Nastav **Ignorovat pozastavené relace po (min)** na `30`, aby se film pozastavený o půlnoci přestal počítat v půl jedné.
4. Nech **Zálohovat, když server neodpovídá** zapnuté.
5. Nastav **Zkusit znovu každých (min)** na `15`.
6. Zapni **Zkoušet jen v časovém okně** a nastav **Zkoušet mezi** na `22:00` a `06:00`.
7. Nastav **Čekat nejvýš (hodin)** na `3` a **Na konci čekání** na **Přesto zálohovat**.

Co se stane v noci, kdy se ve 03:00 ještě někdo dívá: BombVault se zeptá Plexu, najde relaci, která hraje, a zapíše zálohu jako **Odloženo** s dalším pokusem ve 03:15. Ostatní kontejnery běhu se zálohují jako obvykle. Když film v 04:10 skončí, pokus ve 04:15 najde Plex v klidu a zazálohuje ho a čekání skončí jedním oznámením. Kdyby film ještě v 06:00 běžel, čekání by dosáhlo konce a Plex by se zazálohoval stejně, takže jeho zálohy nikdy nezaostanou o víc než tři hodiny.

Pro vlastní kontrolu nech **Příkaz před zálohou** skončit s kódem 75, dokud je aplikace zaneprázdněná, například `[ "$(cat /tmp/players)" = 0 ] || exit 75`. Další příznaky a opravy jsou v [Řešení problémů](troubleshooting.md#deferred-backups).

## Prosté exporty {#exports}

Restic zůstává motorem, ale export ti dá kopii, která nepotřebuje ani BombVault, ani restic.

- **Exportovat kontejner** a **Exportovat VM** pod **Více** na stránce položky zapíšou kopii tak, jak je právě teď, vedle repozitáře: zálohované složky kontejneru plus jeho šablonu Unraidu, nebo obrazy disků VM plus její definici, obnovitelnou přes `virsh define`.
- **Export** u snapshotu zapíše uloženou zálohu do exportní složky na serveru, jako tar, zip nebo vlastní repozitář resticu, a může vzít jeden snapshot nebo všechny. Záloha flashky a autozáloha obsahují klíče, takže jejich export vyžaduje přihlašovací heslo a přihlášení přímo na instanci.

Exporty jsou ve výchozím stavu obyčejné soubory. **Šifrovat exporty pomocí age** v **Nastavení, Systém** je zapečetí, včetně ZIPu flashky, pro příjemce, které vypíšeš, každý je veřejný klíč age nebo veřejný klíč SSH. Později je mimo server dešifruješ odpovídajícím soukromým klíčem. Se zapnutým šifrováním a bez platného příjemce export selže s jasnou chybou, místo aby kdy zapsal obyčejné soubory.

## Co ti běh řekne {#run-notes}

- **Časování.** Každý běh uvádí začátek, konec a délku a každá položka má své **Nedávné běhy**.
- **Proč byla záloha pomalá.** Zatímco záloha běží, BombVault sleduje, jak je vytížený CPU, disky a síť. Když záloha trvá mnohem déle než obvykle a jedna věc byla zjevně na hranici, běh to řekne, například „Cílový disk disk1 byl z 98 % vytížený“. Jinak neřekne nic.
- **Průběh kontrol a prune.** Protokol aktivity a karta **Zkontrolovat zálohy** ukazují, kam až restic napočítal, například *12 z 47 packů*, s časem zbývajícím v tomto kroku, jakmile je dost dat na odhad.
- **Soubory, které nešlo přečíst.** Záloha, která prošla, ale něco vynechala, zoranžoví svůj řádek a jmenuje, co vynechala.
