# BombVault

**Tvoje data z Unraidu, zapečetěná v trezoru. Vhoď zálohu. Odpal obnovu.**

![Přehled: hlavička stavu, co vyžaduje pozornost, ochrana každé sekce, cíle a historie.](assets/screenshots/overview/home-dark.png#only-dark)
![Přehled: hlavička stavu, co vyžaduje pozornost, ochrana každé sekce, cíle a historie.](assets/screenshots/overview/home-light.png#only-light)

BombVault je self-hostovaná webová aplikace pro **zálohování a plné zotavení po havárii** na Unraidu a dalších hostitelích Dockeru, jako je TrueNAS SCALE, ZimaOS nebo obyčejný Linux. Zálohuje kontejnery Dockeru, VM v KVM/libvirt, složky, datové sady ZFS a flashku Unraidu. Běží jako jediný multi-arch Docker kontejner, nabízí webové rozhraní v tmavém, světlém nebo systémovém motivu, a zvládá celý životní cyklus: zálohovat, plánovat, ověřovat a obnovovat.

Obnovy jsou automatické. Na Unraidu se kontejnery znovu objeví v záložce Docker přesně jako předtím a VM jsou znovu definovány ve VM Manageru se svými disky a UEFI NVRAM opět připojenými. Žádná ruční reinstalace, žádná rekonfigurace, žádné drama. Na TrueNAS běží kontejner obnovený vedle originálu jako obyčejný kontejner Dockeru, který obrazovka Apps nevypisuje.

Postaveno na [restic](https://restic.net), takže každá záloha je deduplikovaná, inkrementální a ve výchozím stavu šifrovaná.

!!! note "Uchovej svůj APP_KEY v bezpečí"
    BombVault odvozuje heslo k restic repozitáři z 32bajtového tajemství s názvem `APP_KEY`. Jeho ztráta spolu se sadou pro obnovu učiní šifrované zálohy neobnovitelnými. Vygeneruj si jej pomocí `openssl rand -hex 32` a ulož na bezpečné místo. Viz [Konfigurace](configuration.md).

## Co BombVault chrání {#what-bombvault-protects}

Každý kontejner, VM, sada složek, flash disk, každá datová sada ZFS a vlastní nastavení BombVaultu je **položka** s vlastní stránkou. Položky jednoho druhu tvoří **sekci**, kterou zapneš v **Nastavení, Obecné**.

| Sekce | Co se ukládá |
|---|---|
| **Kontejnery** | Adresář appdata plus definice kontejneru (image, proměnné prostředí, porty, štítky, svazky). |
| **VMs** | Diskové image VM, definice XML a UEFI NVRAM, zálohováno přes SSH (bez připojení libvirt). |
| **Flash** | Celý USB flash (`/boot`): OS, licence, konfigurace pole, sdílené složky, síť a konfigurace pluginů. |
| **Autozáloha** | Vlastní `/config` BombVaultu: jeho databáze nastavení, přihlašovací údaje jeho cílů, pár klíčů SSH pro libvirt, klíč a připnuté klíče hostitelů serverů ZFS a certifikáty. |
| **Složky** | Libovolná složka na serveru jako sada složek, každá s volitelnými vylučovacími vzory. |
| **Datové sady ZFS** | Datová sada se všemi sadami pod ní, čtená z jednoho snímku ZFS a uložená jako složka. Viz [Datové sady ZFS](zfs-datasets.md). |

## Obnova je hvězdou {#restore-is-the-star}

Po zkopírování dat zpět ze snímku restic BombVault přehraje uloženou definici kontejneru proti Docker API, takže se kontejner znovu objeví v záložce Docker v Unraidu, jako by tam byl vždy (stejný image, stejné nastavení, stejné mapování portů). VM dostanou XML znovu definované přes SSH a jejich disky a UEFI NVRAM opět připojené, i po smazání VM.

Jedno okno obnovuje každý druh. Ve výchozím stavu zvolí **Vedle originálu**, které vrátí zálohu jako kopii a originál nechá být. **Nahradit originál** nejdřív zazálohuje aktuální stav, takže obnovu lze vrátit. Dalšími cíli jsou složka nebo nová datová sada ZFS.

Když záloha zastaví závislé kontejnery, vrátí se ve správném pořadí: BombVault je znovu spustí v pořadí `depends_on` z Compose a čeká, až každý ohlásí stav healthy, než spustí ty, které na něm závisí, takže nic nepředběhne databázi nebo bránu, která ještě není v provozu. Viz [Obnova](restore.md) a [Příkazy a jiné kontejnery](backups.md#commands).

## Jak to funguje {#how-it-works}

```
Browser --HTTPS--> BombVault container
                   |- Go binary: JSON API + embedded React UI
                   |- Background worker (per-section scheduler + job executor)
                   |
                   |- /var/run/docker.sock  -> Docker API (container stop/inspect/recreate)
                   |- qemu+ssh://host       -> libvirt / KVM on the HOST over SSH (no mount)
                   |- /mnt/ -> /host/user   -> appdata, VM disks + restic repos (read/write)
                   |- /boot/ -> /host/boot  -> Unraid flash backup (whole USB)
                   |- /config               -> BombVault's own settings + sign-ins (self-backup)
                   '- <repo path>           -> restic repository (local or remote: rclone/s3/rest/sftp)
```

BombVault používá Docker socket k zastavení kontejnerů před zálohou a k jejich znovuvytvoření po obnově. U VM spouští `virsh` na hostiteli přes SSH (`qemu+ssh://`), aby VM šetrně vypnul nebo pořídil živý snímek. Nikdy nepřipojuje (bind-mount) žádnou cestu libvirt, takže nemůže překážet VM Manageru na hostiteli.

BombVault je vrstva orchestrace a UI, nikoli úložný engine. Veškerý skutečný přenos dat prochází přes restic.

## Rychlý start {#quick-start}

Jsi tu nový? Přejdi na **[Začínáme](getting-started.md)**, nainstaluj BombVault a spusť svou první zálohu, a pak si nastav **[Mimo lokalitu a obnova](offsite-recovery.md)**. **[Praktické příklady](guides.md)** provedou běžnými úkoly od začátku do konce, stránka **[Nastavení](settings.md)** vypisuje každou dlaždici a **[Funkce](features.md)** jsou stručný katalog všeho, co BombVault umí.

Každá sekce může kopírovat do několika cílů najednou, spárovaný BombVault může držet tvé kopie mimo lokalitu na serveru, který odmítá mazání, a **Nouzový průvodce** vrátí ztracený server z jeho sady pro obnovu, přes tvou skupinu nebo napsaným klíčem. Konfiguraci na nový stroj přenese karta **Export / import nastavení**. Viz [Mimo lokalitu a obnova](offsite-recovery.md) a [Konfigurace](configuration.md#portable-settings-export-and-import).

**[Aplikace pro Android](android.md)** ti dá do telefonu všechny servery tvé skupiny, s protokolem aktivit všech z nich na jedné obrazovce.

Dokumentace je ve 26 jazycích a rozhraní ve 42. Stránky, které zatím nemají překlad, se ukazují anglicky.

## Použité pojmy {#glossary}

Několik slov má v BombVaultu a na těchto stránkách pevný význam.

| Slovo | Význam |
|---|---|
| **Položka** | Jedna věc, kterou BombVault zálohuje: kontejner, VM, sada složek, flashka, datová sada ZFS s těmi pod ní, nebo jeho vlastní nastavení. Každá má vlastní stránku. |
| **Sekce** | Všechny položky jednoho druhu, například Kontejnery. Sekce zapneš v **Nastavení, Obecné**. |
| **Cíl** | Místo, kam zálohy jdou: složka na tomto serveru, sdílená složka, tvůj vlastní server nebo cloudová služba. Viz [Cíle](offsite-recovery.md#destinations). |
| **Místní** | Cíl na tomto serveru, který drží složky sekcí. |
| **Vlastní složka** | Repozitář, do kterého sekce zapisuje zálohy jako první. Ve výchozím stavu je na tomto serveru a může ležet v některém cíli. Viz [Vlastní složka sekce](offsite-recovery.md#own-folder). |
| **Kopie** | Záloha zkopírovaná z vlastní složky do jiného cíle pomocí `restic copy`. |
| **Přímý repozitář** | Repozitář v cíli mimo lokalitu, do kterého položky zálohují rovnou, bez místní kopie mezi tím. Viz [Přímé repozitáře](offsite-recovery.md#direct-repositories). |
| **Přihlašovací údaje** | Uložený klíč nebo heslo ke službě S3 nebo rest-serveru. Jedny z nich jsou výchozí. Viz [Přihlašovací údaje](offsite-recovery.md#sign-ins). |
| **Umístění** | Které cíle dostávají zálohy položky, nastavené na položce nebo jako výchozí nastavení její sekce. Viz [Umístění podle položky](offsite-recovery.md#placement). |
| **Replika** | Blokově přesná kopie položky ZFS na druhém serveru ZFS, pořízená pomocí `zfs send`. Stojí vedle zálohy a nenahrazuje ji. Viz [Replika](zfs-datasets.md#replica). |
| **Role** | Něco, co jedna instance tvé skupiny dělá pro jinou: Přijímač, Stahovač, Server ZFS nebo Strážce klíče. Viz [Instance a role](offsite-recovery.md#roles). |

## Poděkování {#credits}

- **[VolumeVault](https://github.com/Darkdragon14/VolumeVault)** od [@Darkdragon14](https://github.com/Darkdragon14) (Apache-2.0) dal BombVaultu výchozí myšlenku: zálohu na jedno kliknutí a automatickou reinstalaci kontejnerů Dockeru. BombVault je samostatná implementace v Go nad restic, která tuto myšlenku přenáší na VM, flash a další.
- **[restic](https://restic.net/)** je rychlý, bezpečný a deduplikující zálohovací engine, který BombVault řídí.
- **[rclone](https://rclone.org/)** poskytuje cloudové backendy.
- Většina symbolů na tlačítkách pochází z bezplatné sady Core Solid od **[Streamline](https://streamlinehq.com)** ([CC BY 4.0](https://creativecommons.org/licenses/by/4.0/), [zdroj](https://github.com/webalys-hq/streamline-vectors)). Ostatní pocházejí z Font Awesome Free, Material Design Icons, Simple Icons a Tabler Icons, nebo byly nakresleny pro tento projekt.

## Licence {#license}

Copyright (C) 2026 Junker der Provinz. BombVault je svobodný software pod licencí **GNU Affero General Public License v3.0** ([LICENSE](https://github.com/junkerderprovinz/bombvault/blob/main/LICENSE)). Smíš ho spouštět, studovat, sdílet a měnit. Pokud ho šíříš nebo provozuješ upravenou verzi jako síťovou službu, musíš zveřejnit svůj zdrojový kód pod stejnou licencí a zachovat stávající upozornění na autorská práva a uvedení autorství.

Název a vizuální identita nejsou licencovány. AGPL pokrývá pouze zdrojový kód: "BombVault", jeho logo a jeho vizuální identita zůstávají vyhrazeny, takže fork musí používat vlastní název a vizuální identitu a nesmí se vydávat za BombVault.

## Odkazy {#links}

- **Zdrojový kód:** [github.com/junkerderprovinz/bombvault](https://github.com/junkerderprovinz/bombvault)
- **Vlákno podpory Unraid:** [forums.unraid.net](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/)
- **Problémy:** [github.com/junkerderprovinz/bombvault/issues](https://github.com/junkerderprovinz/bombvault/issues)

!!! warning "Kontrola nad hostitelem na úrovni root"
    Skrze Docker socket může BombVault zastavovat, odebírat a znovu vytvářet kontejnery a číst/zapisovat appdata, a pro zálohu VM se přihlašuje k hostiteli přes SSH, aby spustil `virsh`. Kdokoli, kdo se dostane k jeho webovému rozhraní, má fakticky root na hostiteli. Provozuj BombVault pouze v důvěryhodné, nevystavené síti a nastav přihlašovací heslo v **Nastavení, Zabezpečení**. Sada pro obnovu, úschova klíčů, ZIP flashky, export zálohy flashky nebo autozálohy a mazání záloh v cíli ho vyžadují. Kompletní bezpečnostní model najdeš v [Konfiguraci](configuration.md#security-model).
