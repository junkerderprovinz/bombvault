# Anomálie

Ransomware a rozbitá aplikace vypadají v záloze podobně: najednou je většina dat nová, nebo je většina pryč. BombVault porovnává každou zálohu s vlastní historií dané položky a když něco vypadá špatně, vyvolá nález. Dokud je otevřený nález o ztracených datech, drží také staré zálohy té položky, aby poslední dobrá nemohla zestárnout. Položka **Anomálie** v postranním panelu se ukazuje, dokud je rozpoznávání zapnuté.

![Stránka Anomálie s jedním otevřeným kritickým nálezem.](assets/screenshots/anomalies/page-dark.png#only-dark)
![Stránka Anomálie s jedním otevřeným kritickým nálezem.](assets/screenshots/anomalies/page-light.png#only-light)

## Stránka {#page}

Dlaždice **Kritická**, **Varování** a **Poznámky** vybírají závažnost a **Otevřená** a **Uzavřená** nad seznamem přepínají mezi tím, co je otevřené a co se uzavřelo za posledních 30 dní. Kritický nález sedí na stejné ploše jako ostatní, jeho závažnost se pozná podle štítku a barevného okraje. Nález nejdřív nabídne svou opravu a jeho podrobnosti ukazují hodnotu jako křivku přes obvyklé rozmezí.

Když není nic otevřené, **Průběh učení** ukazuje u každé položky kruh o deseti dílech, jeden díl za každou zálohu, ze které se BombVault učil. **Sledování** otevře seznam položek s vlastním nastavením.

## Co kontroluje {#checks}

Každá záloha kontejneru, VM, sady složek, výpisu databáze, flashky a autozálohy se porovnává s historií dané položky. Kontroly sledují:

- nová data, která běh uložil, proti největším obvyklým množstvím z nedávných záloh a proti obvyklému tempu za hodinu,
- zálohu, která uložila znovu většinu dat, včetně souborů, které byly přejmenovány a přepsány,
- velikost zdroje a počet souborů, které restic hlásí pro každou položku a každý výpis databáze,
- vlastní čas zálohy podle resticu,
- série selhání a občasná selhání,
- kontroly obnovy, které přestaly procházet,
- volné místo místních repozitářů a repozitářů SFTP a rclone, odhadnuté z růstu repozitáře.

Položka se učí z prvních 10 záloh. Téměř prázdný zdroj, přepsání většiny dat a selhání se kontrolují od začátku.

Nález o velikosti zdroje nebo počtu souborů nese datum první zálohy, ve které se ukázal. **Porovnat s předchozí zálohou** vypíše složky, kde soubory zmizely, přibyly nebo se změnily, s poznámkou, když téměř všechno leží ve vyhledávacím indexu, cache nebo náhledech, které si aplikace sama znovu postaví.

Položky ZFS se hlídají datovou sadu po datové sadě, viz [Datové sady ZFS](zfs-datasets.md#anomalies). Asistent může otevřené nálezy číst přes [server MCP](mcp.md#tools).

Kontroly mají své meze. Repozitáře S3, B2 a REST nemají údaj o volném místě, na uživatelské sdílené složce Unraidu je volné místo celého pole a zálohy vzniklé před resticem 0.17 nenesou historii velikosti.

## Uzavření nálezu {#closing}

- **Varování** se uzavírají sama, jakmile příčina zmizí.
- **Kritické** nálezy o ztracených datech a plnícím se disku zůstávají, dokud nezasáhneš.
- **Potvrdit** označí nález jako viděný a uzavře ho. Znovu se nevyvolá, dokud příčina aspoň jednou nezmizí.
- **Označit jako očekávané** říká, že ta změna byla tvoje. Nová úroveň se po 10 zálohách stane normální, ale kontrola téměř prázdného zdroje se nikdy nevypne.
- Změněný výběr, například složka, kterou jsi zaškrtl nebo odškrtl, začne historii položky sám od začátku.

Oznámení odejde jednou za epizodu a selhání a kontroly obnovy, které už oznamují, se nehlásí dvakrát.

## Staré zálohy se mezitím drží {#retention-hold}

Dokud je zdroj téměř prázdný, prudce se zmenšil nebo se většina jeho dat uložila znovu, uchovávání a čištění nechávají staré zálohy té položky na pokoji a nález odkazuje na poslední dobrou zálohu, kterou okno obnovy předvybere. Než takový nález uzavřeš, BombVault ti řekne, že další úklid smaže staré zálohy podle pravidel uchovávání. Takže pokud data chybí nebo byla zašifrována, nejdřív obnov a nález uzavři až potom.

## Nastavení {#settings}

Karta **Anomálie** v **Nastavení, Ověření** drží globální nastavení. Každý ovladač se uloží hned, jak ho změníš, a tři pod přepínačem jsou skryté, dokud je rozpoznávání vypnuté.

| Nastavení | Výchozí | Co dělá |
|---|---|---|
| **Rozpoznávat anomálie** | Zapnuto | Porovnává každou zálohu s historií dané položky. Po vypnutí se nic nového nekontroluje a položka **Anomálie** zmizí z postranního panelu. Karta pořád odkazuje na dřívější nálezy. |
| **Citlivost** | Vyvážená | Přísná hlásí menší změny, Shovívavá jen velké. |
| **Posílat oznámení pro** | Jen kritické nálezy | Nejnižší závažnost, která pošle zprávu kanály nastavenými v [Oznámeních](notifications.md). Opakovaná selhání záloh a výpisů a neúspěšné plánované kontroly obnovy už posílají vlastní zprávu a neposílají se dvakrát. |
| **Ponechat staré zálohy, když se zdroj prudce zmenší nebo je přepsán** | Zapnuto | Dokud má položka otevřený nález o téměř prázdném zdroji, prudkém zmenšení nebo znovu uložené většině dat, uchovávání a čištění nechávají staré zálohy té položky na pokoji. Nález potvrď nebo ho označ jako očekávaný, aby je pustily. |

Každá položka může mít vlastní citlivost a minimum pro oznámení. Nastavíš je na záložce položky **Ověření**, nebo na stránce **Anomálie** pod **Sledování**. U položky ZFS platí pro každou datovou sadu jejího stromu.
