# BombVault

**Vaše data z Unraidu, zapečetěná v trezoru. Vhoďte zálohu. Odpalte obnovu.**

BombVault je self-hostovaná webová aplikace nativní pro Unraid pro **zálohování a plné zotavení po havárii** vašich Docker kontejnerů a KVM/libvirt VM. Běží jako jediný multi-arch Docker kontejner, nabízí moderní webové rozhraní, které se řídí preferencí světlého/tmavého režimu vašeho systému, a zvládá celý životní cyklus: zálohovat, plánovat, ověřovat a obnovovat.

Obnovy jsou automatické. Kontejnery se znovu objeví v záložce Docker v Unraidu přesně jako předtím a VM jsou znovu definovány ve VM Manageru se svými disky a UEFI NVRAM opět připojenými. Žádná ruční reinstalace, žádná rekonfigurace, žádné drama.

Postaveno na [restic](https://restic.net), takže každá záloha je deduplikovaná, inkrementální a vždy šifrovaná.

!!! note "Uchovejte svůj APP_KEY v bezpečí"
    BombVault odvozuje heslo k restic repozitáři z 32bajtového tajemství s názvem `APP_KEY`. Jeho ztráta učiní šifrované zálohy neobnovitelnými. Vygenerujte si jej pomocí `openssl rand -hex 32` a uložte na bezpečné místo. Viz [Konfigurace](configuration.md).

## Co BombVault chrání

| Doména | Co se ukládá |
|---|---|
| **Docker kontejnery** | Adresář appdata plus definice kontejneru (image, proměnné prostředí, porty, štítky, svazky). |
| **KVM / libvirt VM** | Diskové image VM, definice XML a UEFI NVRAM, zálohováno přes SSH (bez připojení libvirt). |
| **Unraid flash** | Celý USB flash (`/boot`): OS, licence, konfigurace pole, sdílené složky, síť a konfigurace pluginů. |
| **Konfigurace aplikace** | Vlastní `/config` BombVaultu: jeho databáze nastavení, přihlašovací údaje mimo lokalitu a pár klíčů SSH pro libvirt. |
| **Soubory a složky** | Pojmenované **sady souborů**, libovolná složka na serveru, každá s volitelnými vylučovacími vzory pro danou sadu. |

## Obnova je hvězdou

Po zkopírování dat zpět ze snímku restic BombVault přehraje uloženou definici kontejneru proti Docker API, takže se kontejner znovu objeví v záložce Docker v Unraidu, jako by tam byl vždy (stejný image, stejné nastavení, stejné mapování portů). VM dostanou XML znovu definované přes SSH a jejich disky a UEFI NVRAM opět připojené, i po smazání VM.

Když záloha zastaví závislé kontejnery, vrátí se ve správném pořadí: BombVault je znovu spustí v pořadí `depends_on` z Compose a čeká, až každý ohlásí stav healthy, než spustí ty, které na něm závisí, takže nic nepředběhne databázi nebo bránu, která ještě není v provozu. Viz [Funkce](features.md).

## Jak to funguje

```
Browser --HTTPS--> BombVault container
                   |- Go binary: JSON API + embedded React UI
                   |- Background worker (per-domain scheduler + job executor)
                   |
                   |- /var/run/docker.sock  -> Docker API (container stop/inspect/recreate)
                   |- qemu+ssh://host       -> libvirt / KVM on the HOST over SSH (no mount)
                   |- /mnt/ -> /host/user   -> appdata, VM disks + restic repos (read/write)
                   |- /boot/ -> /host/boot  -> Unraid flash backup (whole USB)
                   |- /config               -> BombVault's own settings + credentials (self-backup)
                   '- <repo path>           -> restic repository (local or remote: rclone/s3/rest/sftp)
```

BombVault je vrstva orchestrace a UI, nikoli úložný engine. Veškerý skutečný přenos dat prochází přes restic.

## Rychlý start

Jste tu noví? Přejděte na **[Začínáme](getting-started.md)** a nainstalujte BombVault na Unraid přes Community Applications a spusťte svou první zálohu. Poté prozkoumejte kompletní **[Funkce](features.md)**, vylaďte si **[Konfiguraci](configuration.md)** a nastavte si **[Mimo lokalitu a obnova](offsite-recovery.md)**.

Mimo lokalitu se může rozvětvit na několik cílů na doménu najednou, **řídicí panel příjemce** určený jen pro čtení monitoruje tyto kopie na stroji, který je přijímá, a celou svou konfiguraci můžete přenést na nový stroj pomocí karty **Export a import nastavení**. Viz [Mimo lokalitu a obnova](offsite-recovery.md) a [Konfigurace](configuration.md#portable-settings-export-and-import).

## Odkazy

- **Zdrojový kód:** [github.com/junkerderprovinz/bombvault](https://github.com/junkerderprovinz/bombvault)
- **Vlákno podpory Unraid:** [forums.unraid.net](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/)
- **Problémy:** [github.com/junkerderprovinz/bombvault/issues](https://github.com/junkerderprovinz/bombvault/issues)

!!! warning "Kontrola nad hostitelem na úrovni root"
    Skrze Docker socket může BombVault zastavovat, odebírat a znovu vytvářet kontejnery a číst/zapisovat appdata, a pro zálohu VM se přihlašuje k hostiteli přes SSH, aby spustil `virsh`. Kdokoli, kdo se dostane k jeho webovému rozhraní, má fakticky root na hostiteli. Provozujte BombVault pouze v důvěryhodné, nevystavené síti a zapněte volitelnou ochranu heslem (Nastavení, Zabezpečení), jakmile používáte zálohy mimo lokalitu nebo neměnné zálohy. Kompletní bezpečnostní model najdete v [Konfiguraci](configuration.md).
