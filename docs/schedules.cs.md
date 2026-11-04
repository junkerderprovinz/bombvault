# Plány

**Nastavení, Plán** drží všechny plány: úplnou zálohu, která pokrývá všechno, vlastní plán sekce, kopie mimo lokalitu, které položky se účastní a v jakém pořadí. Jedna položka může dostat vlastní plán na své stránce. Tahle stránka vysvětluje, jak do sebe zapadají, a projde typické nastavení.

## Plány čtené jako věty {#plans}

Každý plán je tlačítko, které se čte jako věta, například *denně ve 03:00* nebo *každé 2 dny v 01:30*. Klik ho otevře s pěti režimy:

| Režim | Co nastavuješ |
|---|---|
| **Vypnuto** | Na tomto plánu nic neběží. |
| **Denně** | **Čas** |
| **Týdně** | **Dny** a **Čas** |
| **Každých N dní** | **Každých** tolik **dní** a **Čas**. Počítá se od posledního plánovaného běhu, takže ruční záloha jedné položky plán neposune. |
| **Cron** | **Výraz** o pěti polích, například `0 */6 * * *` pro každých 6 hodin. **Příklady** jich pár nabízí a okno ukazuje další běhy. |

Všechny časy jsou v časovém pásmu serveru. Na Unraidu je to pásmo systému. Jinde nastav `TZ`, jinak každý plán běží v UTC, viz [Proměnné prostředí](configuration.md#environment-variables).

## Týden {#week}

Karta **Týden** kreslí každý plánovaný běh od pondělí do neděle, jeden řádek na den: zálohy, kopie mimo lokalitu a kontroly. Ukaž na čárku a přečteš si, co tam začíná. Kde se tři plány spustí v rozmezí dvou hodin, je úsek označen, takže je můžeš rozložit.

![Nastavení, Plán: pás Týden nahoře, pak úplná záloha a plány podle sekcí.](assets/screenshots/schedule/week-dark.png#only-dark)
![Nastavení, Plán: pás Týden nahoře, pak úplná záloha a plány podle sekcí.](assets/screenshots/schedule/week-light.png#only-light)

## Úplná záloha {#full-backup}

**Úplná záloha** je výchozí plán pro všechno. Nastavuje ho **Kdy zálohy běží**: jeden průchod, který zálohuje každou sekci postupně, kontejnery, VM, flashku, složky, datové sady ZFS a nakonec autozálohu. Řádek pod ním říká, které sekce zahrnuje. **Vypnuto** ho vypne a pak běží jen to, co má vlastní plán.

- **Dohnat zmeškané zálohy po startu**: když byl server v plánovaný čas vypnutý, BombVault tu zálohu spustí asi dvě minuty po svém startu.
- **Příkazy před a po**: **Příkaz před úplnou zálohou** běží na serveru přes `sh -c` před začátkem průchodu, například skript, který zastaví službu. Když selže, úplná záloha přesto proběhne. **Příkaz po úplné záloze** běží jednou po tom, co se zkusila každá sekce, i když některá selhala, což se hodí pro ping potvrzující, že celý server je chráněn. Příkazy pro jeden kontejner jsou na jeho stránce.

**Zálohovat vše nyní** na stránce Zálohy spustí stejný průchod ručně, přes všechny zapnuté sekce.

## Plány podle sekcí {#plans-per-section}

Sekce může opustit úplnou zálohu a běžet ve svém vlastním čase. V **Plány podle sekcí** přidá **Výjimka** sekci s vlastním plánem a **Odebrat výjimku** ji pošle zpět. Řádek v kartě říká, co dělají ostatní, například **Všechny ostatní sekce běží v úplné záloze.**

Sekce s vlastním plánem se v úplné záloze nezálohuje podruhé, takže se nic nezálohuje dvakrát. Totéž platí pro jednu položku s vlastním plánem.

**Kopie mimo lokalitu podle sekcí** funguje stejně pro kopie. Ve výchozím stavu každá sekce kopíruje do svých cílů mimo lokalitu hned po každé záloze. **Výjimka** dá jedné sekci vlastní plán kopírování, například aby se VM kopírovaly na Backblaze B2 jen v neděli v noci. Vlastní karta **Kopírovat** cíle může také nastavit **Kdy kopírovat**, viz [Kopie mimo lokalitu](offsite-recovery.md#off-site-copies).

## Naplánované položky {#scheduled-entries}

**Naplánované položky** říkají podle sekcí, kolik položek se účastní, například **38 z 41 naplánováno**. **Položky** otevřou seznam s přepínačem **V plánu** u každé položky. Položka, která je vypnutá, se zálohuje, jen když ji spustíš ručně. Počítá se jako nechráněná, pokud ji neoznačíš **Nezálohovat**.

## Pořadí {#order}

**Pořadí** určuje, v jakém pořadí plánované běhy a úplná záloha zálohují kontejnery a VM. Přetáhni položku na místo nebo použij šipky a každý posun se hned uloží. Položky, které necháš bez pořadí, se zálohují od nejvíc zpožděné. Jednotlivá ruční záloha tím není dotčena.

**Restart po záloze** ve stejné kartě se týká kontejnerů, které **Zastavit ostatní kontejnery** během zálohy shodilo. Spustí se znovu v pořadí podle závislostí a s **Před spuštěním dalšího počkat, až budou závislosti v pořádku** musí každý hlásit, že je zdravý nebo běží, než nastartuje další. **Časový limit stavu na kontejner (sekundy)** čekání omezuje, ve výchozím stavu 120, v rozsahu 5 až 3600.

## Snímek místo zastavení {#snapshot-instead-of-stop}

**Kontejnery na ZFS zastavit jen kvůli snímku** je ve výchozím stavu zapnuté. Kontejner, jehož data leží na ZFS, se zastaví jen na dobu, kdy BombVault pořizuje snapshot těch dat, obvykle pár vteřin, a záloha se pak čte ze snapshotu. Potřebuje spojení SSH k hostiteli, které používají datové sady ZFS. Kontejner s částí dat jinde se zastaví na celou zálohu jako dřív a jeho běh řekne proč, zatímco kontejner bez dat na ZFS se zastaví bez poznámky. Funguje na uživatelské sdílené složce Unraidu, která leží na jediném poolu ZFS. Na TrueNAS potřebuje aplikace také oprávnění `SYS_CHROOT`, viz [TrueNAS SCALE](getting-started.md#truenas). Jeden kontejner lze na své stránce nastavit jinak.

Záloha skončí pod stejnými cestami a štítky jako ta pořízená se zastaveným kontejnerem, takže obnovy a kontroly zacházejí s oběma stejně. Příkaz před zálohou a výpis databáze běží pořád jako první, ostatní kontejnery k zastavení se shodí a vrátí s ním a příkaz po záloze běží, jakmile kontejner zase stojí. Aktualizace po záloze pořád čeká, až záloha skončí. Jeho běhy ukazují **Snímek** a kolik vteřin kontejner stál. Kdyby se BombVault zastavil uprostřed zálohy kteréhokoli kontejneru, při dalším startu znovu spustí kontejnery, které držel dole.

## Aktualizace po záloze {#updates}

- **Aktualizovat kontejnery po úspěšné záloze** stáhne novější image až poté, co skončila záloha před ním, takže vždy nejdřív existuje čerstvý bod obnovy. Je ve výchozím stavu vypnuté a jeden kontejner lze na jeho stránce nastavit jinak pod **Aktualizovat po úspěšné záloze**.
- **Odstranit starý image po aktualizaci** smaže image, který kontejner opustil. Základní image sdílený s jinými kontejnery se nikdy nesmaže.
- **Po aktualizaci kontejneru obnovit stav aktualizací Unraid** smaže u tohoto kontejneru banner aktualizace Unraidu.

Zpráva za každý aktualizovaný kontejner se nastavuje v **Nastavení, Oznámení** pomocí **Upozornit na aktualizaci kontejneru**.

## Klid před zálohou {#idle}

Karta **Klid před zálohou** určuje, kdy se aplikace počítá jako v klidu, pro kontejner, jehož plánovaná záloha na to čeká: **CPU pod (% jádra)**, **Provoz pod (Mbit/s)** a **Aspoň (min)**. Mediální server se počítá jako v klidu, když nestreamuje, a u kontejneru v síti hostitele se počítá jen CPU. Samotné čekání se nastavuje u každého kontejneru, viz [Když je aplikace zaneprázdněná](backups.md#app-busy).

## Vlastní plán pro jednu položku {#own-plan}

Každá položka má **Vlastní plán** v kartě **Plán a cíl** na záložce Nastavení. S plánem tam položka běží ve svých vlastních časech a opouští úplnou zálohu i plán své sekce. Bez něj se jimi řídí.

## Příklad: tři plány, které do sebe zapadají {#example}

Typická krabice s Unraidem: většinu kontejnerů lze zálohovat v noci, VM jsou velké a mění se pomalu a do Immichu přibývají fotky celý den.

1. **Úplná záloha ve 03:00.** Otevři **Nastavení, Plán**. V **Úplná záloha** nastav **Kdy zálohy běží** na **Denně** ve `03:00`. Zapni **Dohnat zmeškané zálohy po startu**, aby se noc s vypnutým serverem dohnala.
2. **VM jednou týdně.** V **Plány podle sekcí** stiskni **Výjimka**, vyber **VM** a nastav jejich plán na **Týdně**, **Dny** neděle, **Čas** `04:00`. Karta teď říká, že všechny ostatní sekce běží v úplné záloze.
3. **VM se v neděli kopírují i mimo lokalitu.** V **Kopie mimo lokalitu podle sekcí** stiskni **Výjimka**, vyber **VM** a nastav **Týdně**, neděle v `06:00`, až bude záloha mít čas doběhnout.
4. **Immich každých 6 hodin.** Otevři **Zálohy**, pak `immich` a jeho záložku **Nastavení**. Nastav **Vlastní plán** na **Cron** s `0 */6 * * *`.

Co teď běží:

| Kdy | Co |
|---|---|
| Každý den ve 03:00 | Kontejnery kromě `immich`, flashka, složky, datové sady ZFS a autozáloha, sekce po sekci |
| Neděle ve 04:00 | VM |
| Neděle v 06:00 | Kopie VM mimo lokalitu |
| 00:00, 06:00, 12:00 a 18:00 | `immich` samostatně |

Karta **Týden** ukazuje čáru ve 03:00 každý den, čárky ve 04:00 a 06:00 v neděli a čtyři čárky denně pro `immich`. `immich` se ve 03:00 nezálohuje podruhé, protože položka s vlastním plánem opouští úplnou zálohu. Kopie ostatních sekcí pořád běží hned po každé záloze.

Až později vlastní plán `immich` odstraníš, připojí se zase se svou sekcí k běhu ve 03:00.
