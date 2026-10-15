# Začínáme

Tato stránka tě provede od čerstvého stroje s Unraidem až k tvé první záloze.

## Požadavky

| Požadavek | Poznámky |
|---|---|
| **Unraid 6.12+** | Starší verze nejsou testovány. Unraid je hlavní platforma, ale BombVault běží i na obyčejném hostiteli Dockeru a na TrueNAS Scale (viz [Obecný hostitel Dockeru](#generic-docker-host) a [TrueNAS SCALE](#truenas)). |
| **Umístění restic repozitáře** | Místní cesta (doporučeno: tvoje pole nebo cache), SMB, NFS nebo libovolný rclone backend. |
| **Docker socket** | Připojen šablonou automaticky (`/var/run/docker.sock`). |
| **Unraid flash** (`/boot`) | Připojen celý šablonou automaticky (`/boot` na `/host/boot`). Pohání zálohu flashe a umožňuje obnovenému kontejneru znovu se objevit jako běžná, editovatelná aplikace Unraidu. |
| **KVM VM** (volitelné) | Záloha VM komunikuje s libvirt přes SSH, bez připojení libvirt. Nastav ji v **Nastavení, Integrace, SSH k hostiteli** (viz [Konfigurace](configuration.md)). |
| **Datové sady ZFS** (volitelné) | Stejné spojení SSH jako u záloh VM, `zfs` na hostiteli a Host Data připojené jako `/mnt` s režimem přístupu Read/Write - Slave, což je výchozí nastavení šablony. Viz [Datové sady ZFS](zfs-datasets.md). |
| **Aplikace pro Android** (nepovinné) | Android 10 nebo novější, spárovaná se servery ve verzi 9.7.0 nebo novější. Viz [Aplikace pro Android](android.md). |

## Instalace na Unraid

Nejsnazší cestou je **Community Applications**.

1. Otevři záložku **Apps** v Unraidu.
2. Vyhledej **BombVault**.
3. Klikni na **Install**, nastav požadované proměnné (níže) a aplikuj.

!!! tip "Ruční instalace šablony"
    Pokud dáváš přednost ručnímu přidání šablony:

    1. Přejdi na **Docker, Add Container, Template repositories** a přidej:
       ```
       https://github.com/junkerderprovinz/unraid-apps
       ```
    2. Vyhledej **BombVault** v Templates.
    3. Nastav požadované proměnné a klikni na **Apply**.

## Obecný hostitel Dockeru {#generic-docker-host}

Nemáš Unraid? BombVault běží i jako prostý kontejner na libovolném hostiteli Dockeru. TrueNAS SCALE má vlastní kroky v [TrueNAS SCALE](#truenas).

1. Stáhni si z repozitáře soubor [`deploy/docker-compose.generic.yml`](https://github.com/junkerderprovinz/bombvault/blob/main/deploy/docker-compose.generic.yml), připravený k úpravám.
2. Nastav `APP_KEY` (viz níže) a nasměruj svazek Host Data na svůj skutečný kořen dat. Komentáře v souboru provedou obojím.
3. `docker compose up -d`, poté otevři `https://<ip-hostitele>:3443/`.

V čem se to liší od Unraidu:

- **Žádná sekce Flash.** Není tu žádný zaváděcí USB disk, který by se dal zachytit nebo obnovit, sekce Flash tedy nemá co dělat. Místo toho, když Host Data sahá do `/etc` hostitele, nabízí **Přidat** na stránce Zálohy jako návrh složky **Konfigurace systému**: výchozí sadu složek `/etc`, kterou si před uložením projdeš a upravíš.
- **Žádná nativní oznámení Unraidu.** Vlastní oznamovací kanály BombVaultu (webhook, upozornění na selhání mimo lokalitu a podobně) fungují normálně; vynechává se jen odeslání do oznamovacího systému Unraidu, protože takový systém tu není.
- **Záloha virtuálních strojů je volitelná a potřebuje samostatného hostitele libvirtd dostupného přes SSH.** Viz zakomentovaný blok v souboru compose. Obecný hostitel Dockeru sám o sobě žádného správce virtuálních strojů nemá.
- **Žádný widget na dashboardu.** BombVault Widget je plugin pro Unraid, takže i tento krok odpadá.
- **Jak se najdou data kontejneru.** Bez konvence `appdata` z Unraidu se datová složka kontejneru najde podle segmentů v `DATA_ROOT_SEGMENTS`, pojmenovaných svazků Dockeru, pracovního adresáře projektu Compose a štítku `bombvault.data` (viz [Rozpoznávání zdrojů zálohy](configuration.md#backup-source-detection)). Pojmenované svazky i přednastavení `/etc` dosáhnou jen na cesty uvnitř připojení Host Data, takže nasměruj Host Data na společného předka, který pokrývá i kořen dat Dockeru.
- **`PLATFORM`.** Nastav ji na `generic` nebo `truenas`. Když ji nenastavíš, BombVault rozpozná Unraid podle jeho vlastní značky na připojení flash a cokoli jiného bere jako obecného hostitele, a kroky jen pro Unraid se přeskočí, místo aby se zkoušely a selhávaly.

### TrueNAS SCALE {#truenas}

BombVault zatím není v katalogu aplikací TrueNAS, takže ho nainstaluj jako vlastní aplikaci: **Apps, Discover Apps** a pak **Install via YAML** v nabídce vedle **Custom App**. YAML níže nastavuje totéž co položka katalogu ve složce `truenas-apps` v repozitáři. Nahraď `tank` svým poolem a vyplň `APP_KEY`, jak popisuje další oddíl.

```yaml
services:
  bombvault:
    image: junkerderprovinz/bombvault:latest
    hostname: bombvault
    restart: unless-stopped
    cap_drop: [ALL]
    cap_add: [CHOWN, DAC_OVERRIDE, FOWNER, SETGID, SETPCAP, SETUID]
    environment:
      APP_KEY: ""
      PLATFORM: truenas
      HOST_SOURCE_ROOT: /mnt/tank
      HOST_MOUNT_ROOT: /mnt/tank
      # Pro zálohy VM:
      # LIBVIRT_URI: qemu+ssh://root@<truenas-ip>/system?socket=/run/truenas_libvirt/libvirt-sock
    volumes:
      - /mnt/tank/apps/bombvault:/config
      - /var/run/docker.sock:/var/run/docker.sock
      - type: bind
        source: /mnt/tank
        target: /mnt/tank
        bind:
          propagation: rslave
    ports:
      - "30528:3443"
```

Pak otevři `https://<truenas-ip>:30528/`. Co je uvedeno v [V čem se to liší od Unraidu](#generic-docker-host), platí i tady, a na TrueNAS navíc:

- **Capabilities.** BombVault potřebuje šest výše uvedených. Bez `SETPCAP` zálohy VM selžou.
- **Host Data** je cesta hostitele na datové sadě, zde `/mnt/tank`, připojená pod stejnou cestou uvnitř aplikace. Proto ji uvádějí `HOST_SOURCE_ROOT` i `HOST_MOUNT_ROOT`. `rslave` předává připojení, která hostitel vytvoří později, a ta potřebují zálohy datových sad ZFS.
- **Data v ixVolume se nezálohují.** Aplikace, jejímž úložištěm je ixVolume, drží data pod `/mnt/.ix-apps`, mimo Host Data. BombVault uloží jen definici aplikace a běh uvede složky, které vynechal. Pro každou aplikaci, jejíž data chceš zálohovat, zvol úložiště **Host Path** na datové sadě pod Host Data.
- **Zálohy VM a datových sad ZFS** se k TrueNAS připojují přes SSH jako root. Zapni službu SSH v **System, Services** a přidej veřejný klíč z **Nastavení, Integrace, SSH k hostiteli** uživateli root v **Credentials, Users**. Zálohy VM navíc potřebují `LIBVIRT_URI` jako výše, protože libvirtd na TrueNAS naslouchá na vlastním socketu. Když není nastavena `LIBVIRT_URI` ani `LIBVIRT_HOST`, BombVault použije jako hostitele výchozí bránu aplikace, protože `host.docker.internal` se v aplikaci TrueNAS nepřeloží. Podrobnosti, včetně VM na zvolech, popisuje [průvodce zálohou VM](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md#truenas-scale).
- **Replika ZFS používá jiný klíč.** Replika na druhém serveru ZFS se přihlašuje klíčem, který ukazuje dialog repliky, ne klíčem SSH k hostiteli. Ten přidej na přijímajícím serveru.
- **Smazaná aplikace se vrací ve dvou krocích.** Nejdřív ji v TrueNAS znovu nainstaluj pod stejným názvem, čímž se znovu vytvoří její síť, a pak obnov její data v BombVaultu. Do té doby kontrola před obnovou hlásí chybějící síť a obnova odmítne dřív, než cokoli změní.
- **Vedle originálu** vytvoří prostý kontejner Dockeru. Běží, ale obrazovka Apps v TrueNAS ho neukazuje.
- **Dumpy databází** fungují na připojeních datových sad. Import do datové složky, která je samostatnou datovou sadou, přesune starý obsah do složky vedle ní, takže nadřazená datová sada pro něj potřebuje místo.

## Jediné povinné nastavení

Jedinou proměnnou, kterou musíš nastavit, je `APP_KEY`, 32bajtové hex tajemství (64 hex znaků) použité k odvození hesla k restic repozitáři.

Vygeneruj si jej na libovolném stroji:

```bash
openssl rand -hex 32
```

Výsledek vlož do pole `APP_KEY` v šabloně (Unraid), nebo do proměnné prostředí `APP_KEY` v `docker-compose.yml` (běžný Docker host).

!!! danger "Neztrať svůj APP_KEY"
    Ztráta `APP_KEY` učiní tvé šifrované zálohy neobnovitelnými. Ulož jej na bezpečné místo oddělené od serveru. Jakmile BombVault běží, nastav přihlašovací heslo a stáhni **sadu pro obnovu** v **Nastavení, Obnova** (viz [Mimo lokalitu a obnova](offsite-recovery.md#recovery-kit)). Obsahuje klíč, každý cíl a příkazy resticu.

Šablona za tebe také připojí Docker socket, flash (`/boot`) a kořen **Host Data** (`/mnt`). *Zdroje* i *cíle* záloh žijí pod Host Data. Kompletní referenci proměnných a nastavení mimo lokalitu najdeš v [Konfiguraci](configuration.md).

## První spuštění

![Přehled po první záloze: co vyžaduje pozornost, ochrana každé sekce, cíle a historie.](assets/screenshots/dashboard.png)

*Přehled po první záloze: co vyžaduje pozornost, ochrana každé sekce, cíle a historie.*

1. Otevři webové rozhraní na `https://<your-unraid-ip>:3443` (ze startu samopodepsaný certifikát). Nová instalace se otevře na **Vítej v BombVaultu**, kde je seznam toho, co BombVault na serveru našel.
2. Zvol **Jdeme na to**. Přehled pak ukáže kartu **První kroky** se třemi kroky:
    1. **Vyber sekce**: zapni, co BombVault zálohuje (Kontejnery, VMs, Flash, Složky, Datové sady ZFS, Autozáloha). **Vybrat** otevře seznam položek sekce, kde **Pozastavit plán** jednu vynechá. Sekce zapnuté tady se spouštějí každou noc, což změníš v **Nastavení, Plán**.
    2. **Přidat cíl**: kam se zálohy ukládají. Na začátek stačí místní složka. Nejbezpečnější je druhý cíl mimo lokalitu, například Storage Box nebo Backblaze B2. Místní repozitáře mají ve výchozím stavu `/mnt/user/bombvault/{container,vms,flash,config,files,zfs}` a vytvoří se při první záloze.
    3. **První záloha**: potom už běží každou noc sama.
3. Otevři **Zálohy** a uvidíš všechny položky v jednom seznamu. **Zálohovat vše nyní** spustí běh přes všechny zapnuté sekce a **Zálohovat nyní** na stránce položky zazálohuje jen tuto jednu.

Už jsi BombVault používal? **Obnovit ze zálohy** na uvítací stránce otevře stránku Obnova, kde **Nouzový průvodce** vrátí nastavení, kontejnery, VM a složky ztraceného serveru. Viz [Mimo lokalitu a obnova](offsite-recovery.md#emergency).

!!! tip "Volitelné: zvol pořadí zálohování"
    Pokud by se některé kontejnery měly vždy zálohovat před ostatními (například databáze před aplikací, která ji používá), otevři kartu **Pořadí** v **Nastavení, Plán** a přetáhni je do požadované sekvence. Naplánované běhy a úplná záloha se jím pak řídí; cokoli neuspořádaného se zálohuje od nejvíce po termínu.

!!! note "Kontrola integrace hostitele"
    Karta **Integrace hostitele** v Přehledu prozkoumá každé připojení a CLI (Docker socket, libvirt, restic, qemu-img, rclone) a nahlásí případné chybějící části, takže si můžeš potvrdit, že je kontejner správně zapojen, dříve než se na něj budeš spoléhat. Čeká pod **Přizpůsobit**, dokud ji nepřidáš.

## Orientace v rozhraní {#navigation}

![Nastavení jako dlaždice, vedle nich karty zvolené dlaždice. Každá změna se zapíše hned, jak ji uděláš.](assets/screenshots/settings.png)

*Nastavení jako dlaždice, vedle nich karty zvolené dlaždice. Každá změna se zapíše hned, jak ji uděláš.*

Postranní panel má pět položek: **Přehled**, **Anomálie** (dokud je zapnuté zjišťování anomálií), **Zálohy**, **Obnova** a pod **Odhlásit se** **Nastavení**. Hlavní akce stránky plave vpravo dole a **Více** skrývá ty vzácné. Na telefonu lišta obsahuje Přehled, Zálohy, Obnovu a Nastavení a **Více** má zbytek.

Stránky a nastavení využívají celou šířku okna. Okno drží svá tlačítka v patě a to, které vede dál, je vpravo. Na telefonu pata s více než třemi tlačítky ponechá cestu ven a hlavní akci a ostatní dá pod **Více**.

- **Zálohy** vypisují každý kontejner, VM, sadu složek, flash disk, každou datovou sadu ZFS a autozálohu jako položky. Dlaždice nad seznamem vybírají druh a **Filtry**, **Řazení** a **Hledat** výběr zužují. Každá položka má vlastní stránku se třemi záložkami: **Zálohy**, **Nastavení** a **Ověření**. Tlačítko zpět vlevo od ikony položky vede na seznam.
- **Obnova** je jedna stránka pro všechno: vyber sekci, vyhledej položku a zvol zálohu. Odtud začíná i Nouzový průvodce a další zdroje.
- **Nastavení** je čtrnáct dlaždic: Obecné, Vzhled, Plán, Cíle, Obnova, Oznámení, Ověření, Zabezpečení, Instance, Párování, Integrace, Aplikace, Systém a Info. Klávesa `/` nebo Ctrl+K prohledá všechny, i podle názvů, které stránky měly v dřívějších verzích. Dlaždice si přetáhni do vlastního pořadí pod **Vzhled**. Kde by sloupec dlaždic vedle stránky stránku zúžil, například v okně širokém asi 1200 pixelů, stanou se dlaždice pruhem nad stránkou, který se posouvá do stran. Telefon ukazuje stejný pruh.
- **Cíle** a **Instance** jsou dlaždice nastavení s vlastními stránkami. **Zobrazit také v postranním panelu** na jejich dlaždici je připne do postranního panelu.

Nic nemá tlačítko Uložit a nic se nerozbaluje: vzácné věci mají vlastní kartu nebo okno a vysvětlení jsou v bublinách (i). Po aktualizaci se samy otevřou poznámky k vydání a zelené tečky označují, co se změnilo. Klepnutím tečku skryješ. **Novinky** na kartě Verze v **Nastavení, Info** počítají, co jsi ještě neviděl, a otevřou poznámky k vydání a seznam každé změny s **Přejít tam** jako dvě záložky jednoho okna.

## Sestavení ze zdrojového kódu {#build-from-source}

BombVault je jediná statická binárka v Go, která obsluhuje JSON API a vestavěné rozhraní v Reactu. Nejprve sestav rozhraní, pak spusť binárku:

```bash
npm --prefix web ci
npm --prefix web run build     # writes web/dist, which the binary embeds
export APP_KEY=$(openssl rand -hex 32)
go test ./...                  # unit and integration tests, with a real restic round trip
golangci-lint run ./...
go run ./cmd/bombvault         # serves https://localhost:3443 with a self-signed certificate
```

Sestavení rozhraní je potřeba i pro `go run`. Repozitář obsahuje pod `web/dist` jen prázdnou značku, takže bez `npm --prefix web run build` binárka nic nevloží a odpoví `500 SPA index not found`, což je očekávané. Docker, libvirt a Unraid nejde v CI otestovat, takže připojení, restic a SSH spojení k VM ověř na skutečném hostiteli pomocí karty **Integrace hostitele** v Přehledu (nebo `GET /api/spike`), než otevřeš pull request.

## Další kroky

- Projdi si kompletní **[Funkce](features.md)**.
- Dej si všechny servery své skupiny do telefonu s **[aplikací pro Android](android.md)**.
- Přidej jeden nebo více cílů mimo lokalitu (každá sekce může kopírovat do několika najednou) a ulož si sadu pro obnovu, jak je popsáno v **[Mimo lokalitu a obnova](offsite-recovery.md)**.
- Klonuješ sestavu nebo přecházíš na nový stroj? Přenes celou svou konfiguraci pomocí karty **Export / import nastavení**. Viz [Konfigurace](configuration.md#portable-settings-export-and-import).
- Narazil jsi na problém? Viz **[Řešení problémů](troubleshooting.md)**.
