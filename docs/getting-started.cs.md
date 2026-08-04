# Začínáme

Tato stránka vás provede od čerstvého stroje s Unraidem až k vaší první záloze.

## Požadavky

| Požadavek | Poznámky |
|---|---|
| **Unraid 6.12+** | Starší verze nejsou testovány. |
| **Umístění restic repozitáře** | Místní cesta (doporučeno: vaše pole nebo cache), SMB, NFS nebo libovolný rclone backend. |
| **Docker socket** | Připojen šablonou automaticky (`/var/run/docker.sock`). |
| **Unraid flash** (`/boot`) | Připojen celý šablonou automaticky (`/boot` na `/host/boot`). Pohání zálohu flashe a umožňuje obnovenému kontejneru znovu se objevit jako běžná, editovatelná aplikace Unraidu. |
| **KVM VM** (volitelné) | Záloha VM komunikuje s libvirt přes SSH, bez připojení libvirt. Nastavte ji v Nastavení (viz [Konfigurace](configuration.md)). |

## Instalace na Unraid

Nejsnazší cestou je **Community Applications**.

1. Otevřete záložku **Apps** v Unraidu.
2. Vyhledejte **BombVault**.
3. Klikněte na **Install**, nastavte požadované proměnné (níže) a aplikujte.

!!! tip "Ruční instalace šablony"
    Pokud dáváte přednost ručnímu přidání šablony:

    1. Přejděte na **Docker, Add Container, Template repositories** a přidejte:
       ```
       https://github.com/junkerderprovinz/unraid-apps
       ```
    2. Vyhledejte **BombVault** v Templates.
    3. Nastavte požadované proměnné a klikněte na **Apply**.

## Jediné povinné nastavení

Jedinou proměnnou, kterou musíte nastavit, je `APP_KEY`, 32bajtové hex tajemství (64 hex znaků) použité k odvození hesla k restic repozitáři.

Vygenerujte si jej na libovolném stroji:

```bash
openssl rand -hex 32
```

Výsledek vložte do pole `APP_KEY` v šabloně.

!!! danger "Neztraťte svůj APP_KEY"
    Ztráta `APP_KEY` učiní vaše šifrované zálohy neobnovitelnými. Uložte jej na bezpečné místo oddělené od serveru. Jakmile BombVault běží, použijte jeho **sadu pro obnovu šifrovacího klíče** na jedno kliknutí (viz [Mimo lokalitu a obnova](offsite-recovery.md)) k uložení kompletního balíčku pro obnovu.

Šablona za vás také připojí Docker socket, flash (`/boot`) a kořen **Host Data** (`/mnt`). *Zdroje* i *cíle* záloh žijí pod Host Data. Kompletní referenci proměnných a nastavení mimo lokalitu najdete v [Konfiguraci](configuration.md).

## První spuštění

1. Otevřete webové rozhraní na `https://<your-unraid-ip>:3443` (samopodepsaný certifikát rovnou z krabice).
2. V **Nastavení** povolte zálohovací domény, které chcete (Kontejnery, VM, Flash, Config, Soubory), a vyberte barvu zvýraznění.
3. V záložce **Kontejnery** vyberte kontejner a klikněte na **Zálohovat** pro vytvoření svého prvního bodu obnovení. Cesty repozitářů mají výchozí hodnotu `/mnt/user/bombvault/{container,vms,flash,config,files}` a vytvoří se při první záloze.
4. Nastavte plánování v **Nastavení, Plány**. Pro kontejnery a VM je k dispozici *zahrnout vše do plánu* na jedno kliknutí.

!!! tip "Volitelné: zvolte pořadí zálohování"
    Pokud by se některé kontejnery měly vždy zálohovat před ostatními (například databáze před aplikací, která ji používá), otevřete panel **pořadí zálohování** na stránce Kontejnery a přetáhněte je do požadované sekvence. Naplánované běhy a běhy s vícenásobným výběrem se jím pak řídí; cokoli neuspořádaného se zálohuje od nejvíce po termínu, jako dříve.

!!! note "Kontrola integrace hostitele"
    Po spuštění kontejneru otevřete `/spike` ve webovém rozhraní. Prozkoumá každé připojení a CLI (Docker socket, libvirt, restic, qemu-img, rclone) a nahlásí případné chybějící části, takže si můžete potvrdit, že je kontejner správně zapojen, dříve než se na něj budete spoléhat.

## Jednoduché vs. pokročilé

Ve výchozím nastavení rozhraní zobrazuje jen to nejnutnější (zálohovat, obnovit, plánovat). Použijte přepínač **Jednoduché / Pokročilé** v postranním panelu k odhalení expertních ovládacích prvků: uchovávání, kopie mimo lokalitu, pre/post hooky, obnova na úrovni souborů, oznámení, metriky Prometheus a nástroje integrity/údržby. Jde o předvolbu na úrovni prohlížeče, ve výchozím stavu vypnutou, takže nováčci dostanou čisté UI a pokročilí uživatelé dostanou vše.

## Další kroky

- Projděte si kompletní **[Funkce](features.md)**.
- Přidejte jednu nebo více replik **[Mimo lokalitu a obnova](offsite-recovery.md)** (každá doména může odesílat na několik cílů najednou) a uložte si svou sadu pro obnovu.
- Klonujete sestavu nebo přecházíte na nový stroj? Přeneste celou svou konfiguraci pomocí karty **Export a import nastavení**. Viz [Konfigurace](configuration.md#portable-settings-export-and-import).
- Narazili jste na problém? Viz **[Řešení problémů](troubleshooting.md)**.
