# Přehled

Přehled je první stránka v postranním panelu a ta, na kterou se hodí mrknout každé ráno. Odpovídá na dvě otázky najednou: je všechno zálohované a potřebuje něco tebe? Začíná hlavičkou stavu a čtyřmi kartami, další karty čekají pod **Přizpůsobit**.

![Přehled po několika dnech: hlavička stavu, karta Vyžaduje pozornost s jedním varováním, ochrana každé sekce, cíle a historie.](assets/screenshots/overview/home-dark.png#only-dark)
![Přehled po několika dnech: hlavička stavu, karta Vyžaduje pozornost s jedním varováním, ochrana každé sekce, cíle a historie.](assets/screenshots/overview/home-light.png#only-light)

## Hlavička stavu {#status-head}

Hlavička sedí nahoře na stránce a zůstává tam pořád. Její kruh ukazuje, kolik je chráněno, třeba **41 z 43 chráněno**, a řádek vedle něj říká, jak na tom věci jsou:

| Hlavička říká | Co to znamená |
|---|---|
| **Vše zálohováno** | Každá položka každé zapnuté sekce má aktuální zálohu. |
| **Zálohováno, s poznámkami** | Všechno je zálohované a některé běhy po sobě nechaly poznámku, například o souborech, které nemohly přečíst. |
| **Ještě není chráněno všechno** | Některé položky nemají aktuální zálohu, například protože čekají na svou první. |
| **3 věci vyžadují pozornost** | Něco selhalo nebo čeká na rozhodnutí. Karta **Vyžaduje pozornost** to vypisuje. |
| **Právě se zálohuje** | Běží záloha. Kruh se změní na ukazatel průběhu se stanicemi **Zastavit**, **Číst** a **Spustit** a úplná záloha počítá své položky za pochodu, třeba **Právě se zálohuje: 12 z 43**. **Zobrazit běh** otevře daný běh. |
| **Probíhá kopie mimo lokalitu** | Běží kopírování do cíle. Neukazuje procenta, protože `restic copy` žádná nehlásí. |

Pod řádkem stojí, kdy proběhl poslední běh a kdy začíná další plánovaný.

## Vyžaduje pozornost {#needs-you}

Tahle karta vypisuje, na co by ses měl podívat, od nejnovějšího, a když je seznam prázdný, říká **Není co dělat.** Každý řádek má vlastní cestu ven:

- **Neúspěšná záloha** jmenuje položku a důvod. **Zobrazit chyby** ukáže výstup samotného resticu a **Zkusit znovu** spustí zálohu ještě jednou.
- **Kopie mimo lokalitu selhala** jmenuje sekci a od kdy. **Replikovat nyní** to zkusí znovu.
- **Anomálie u plex** vede přes **Kontrola** k nálezu. Dokud je kritický nález otevřený, uchovávání nechává staré zálohy té položky na pokoji.
- **Nové** položky, které BombVault našel a nic je nezálohuje, například právě nainstalovaný kontejner, nabízejí **Začít zálohovat** nebo **Nezálohovat**.
- **Záloha, která to vzdala**, protože její aplikace byla po celou dobu čekání vytížená, viz [Když je aplikace vytížená](backups.md#app-busy).
- **Neúspěšná replika ZFS**, popsaná prostými slovy, třeba že server nemá příkaz zfs, s výstupem serveru pod **Zobrazit**.
- **Zálohy stojí: klíč už nejde otevřít** se objeví, když tento server převzal klíč jiného serveru a `APP_KEY` se potom změnil. Viz [Řešení problémů](troubleshooting.md#backups-stopped-and-say-the-key-can-no-longer-be-opened).
- **Ulož si sadu pro obnovu** zůstává, dokud sadu nestáhneš a nestiskneš **Uložil jsem ji bezpečně**.

## Stav ochrany {#protection-status}

Každá zapnutá sekce má řádek se čtyřmi kontrolami:

| Kontrola | Zezelená, když | Jiná slova, která můžeš vidět |
|---|---|---|
| **Místní** | poslední záloha je aktuální | zatím žádná záloha, žádný plán, poslední běh selhal, záloha je po termínu, není chráněna každá položka, záloha brzy přijde na řadu |
| **Mimo lokalitu** | poslední kopie mimo lokalitu je aktuální | žádná kopie mimo lokalitu, kopie běží, kopie je po termínu, kopie je pozastavená, zatím žádná kopie, replika selhala |
| **Ochrana proti smazání** | zkouška odolnosti prokázala, že druhá strana mazání odmítá | žádná, žádná a riziko přijato, zatím neprokázáno, zkouška selhala, neprokázáno, protože její kontrola je vypnutá |
| **Zkušební obnova** | obnova z kopie mimo lokalitu prošla | nikdy nespuštěna, běží, selhala, po termínu, jen ručně, pro tuto sekci není možná |

Sekce je zelená jen na základě ověřených faktů. Kopie na tomto serveru se za kopii mimo lokalitu nepočítá a cíl, který jsi označil **Přijmout bez ochrany proti smazání**, ukazuje šedý kruh místo oranžové tečky, takže barvu sekce určují její ostatní kontroly.

Naplánovaná sekce ukazuje **Zatím žádná záloha** a zůstává neutrální, dokud nepřijde termín jejího prvního běhu. Potom ukazuje **Po termínu** a jednou upozorní. Sekce, ve které není co zálohovat, protože žádný plán nezasahuje žádnou z jejích položek, ukazuje **Neplánováno** a nikdy není po termínu.

Klik na sekci otevře její okno se všemi kontrolami rozepsanými, s položkami, které počítá, a s tlačítkem **Spustit zkušební obnovu** nebo **Spustit zkušební obnovu znovu**. Uvádí také **Pravidlo pro vyčištění je nastaveno** nebo **Žádné pravidlo pro vyčištění** a pravidlo uchovávání vlastní složky sekce se počítá stejně jako její pravidla mimo lokalitu.

![Okno ochrany sekce Kontejnery se čtyřmi rozepsanými kontrolami.](assets/screenshots/overview/protection-window-dark.png#only-dark)
![Okno ochrany sekce Kontejnery se čtyřmi rozepsanými kontrolami.](assets/screenshots/overview/protection-window-light.png#only-light)

## Cíle {#storage-locations}

Každý cíl tu stojí jako nádoba se svou mírou naplnění, rychlostí růstu, třeba **Roste 4,2 GB/týden**, a údajem, kdy by se při tomto tempu zaplnil, třeba **plný asi za 31 týdnů**. S3, B2 a rest-servery nehlásí, kolik místa zbývá, takže ukazují **Volné místo neznámé** a jen svůj růst. Klik vede na stránku cíle. Viz [Cíle](offsite-recovery.md#destinations).

## Historie {#history}

**Historie** má dvě záložky. **Běhy** vypisují každý běh s výsledkem, délkou a důvodem a klik ho otevře. **Protokol** ukazuje každý řádek s časem, tak jak ho BombVault píše, a dá se zúžit na jeden den. Řádek zoranžoví, když záloha prošla, ale něco vynechala, a řádek říká, co to bylo.

## Další karty pod Přizpůsobit {#customize}

**Přizpůsobit** nahoře na stránce ti dovolí karty uspořádat. Kartu přesuneš tažením za její lištu, velikost změníš tečkou v jejím dolním rohu, nebo ji skryješ. Skryté karty čekají pod **Skryté karty** a **Zobrazit** ji vrátí. Při úpravě se karty trochu kolébají a **Hotovo** to ukončí. Rozvržení se ukládá v tomto prohlížeči.

Tyto karty začínají skryté:

- **Anomálie**: otevřené nálezy, viz [Anomálie](anomalies.md).
- **Nezálohováno**: všechno na tomto serveru, co žádná plánovaná záloha nepokrývá, vždy s důvodem, například **nikdy nepřidáno**, **není zahrnuto v plánu** nebo **všechny cíle jeho sekce jsou vypnuté**. Kontejnery čte přímo z Dockeru, takže najde i ten, který BombVault nikdy neviděl.
- **Kalendář**: posledních 90 dní, jeden čtvereček na den. Klik na den ukáže jeho řádky v protokolu pod **Historie**.
- **Úložiště**: kolik místa zabírá která sekce a jak dobře se její data deduplikují.
- **Přehled**: nejstarší snapshot napříč všemi sekcemi, kdy poběží další záloha a jak dopadla ta poslední.
- **Statistiky**: kolik je kontejnerů a VM, kolik chyb a kolik plánů je aktivních nebo pozastavených.
- **Integrace hostitele**: čeho se BombVault na hostiteli dotýká a co potřebuje, například Docker socket, libvirt, restic a rclone. Začni tady, když se zdá, že něco není zapojené.

![Režim Přizpůsobit: karty se dají tahat, měnit jim velikost a skrývat, skryté čekají dole.](assets/screenshots/overview/customize-dark.png#only-dark)
![Režim Přizpůsobit: karty se dají tahat, měnit jim velikost a skrývat, skryté čekají dole.](assets/screenshots/overview/customize-light.png#only-light)

## Jak BombVault počítá {#counting}

Každá karta i hlavička počítají stejně: položky zapnutých sekcí, které jsou stále nainstalované a které jsi neoznačil **Nezálohovat**. Položka se počítá jako chráněná, jen dokud ji plán zasahuje a má zálohu. Stará záloha sama o sobě nestačí, takže kontejner, který jsi vyřadil z plánu, se ukáže jako nechráněný, dokud ho nevrátíš zpět nebo neoznačíš **Nezálohovat**.

## První kroky {#first-steps}

Na čerstvé instalaci začíná přehled kartou **První kroky**: **Vyber sekce**, **Přidat cíl** a **První záloha**. Zůstává, dokud první záloha neskončí. [Začínáme](getting-started.md#first-run) tě jí provede.
