# Začínáme

Tato stránka vás provede od čerstvého stroje s Unraidem až k vaší první záloze.

## Požadavky

| Požadavek | Poznámky |
|---|---|
| **Unraid 6.12+** | Starší verze nejsou testovány. Unraid je hlavní platforma, ale BombVault běží i na obyčejném hostiteli Dockeru a na TrueNAS Scale (viz [Obecný hostitel Dockeru](#generic-docker-host)). |
| **Umístění restic repozitáře** | Místní cesta (doporučeno: vaše pole nebo cache), SMB, NFS nebo libovolný rclone backend. |
| **Docker socket** | Připojen šablonou automaticky (`/var/run/docker.sock`). |
| **Unraid flash** (`/boot`) | Připojen celý šablonou automaticky (`/boot` na `/host/boot`). Pohání zálohu flashe a umožňuje obnovenému kontejneru znovu se objevit jako běžná, editovatelná aplikace Unraidu. |
| **KVM VM** (volitelné) | Záloha VM komunikuje s libvirt přes SSH, bez připojení libvirt. Nastavte ji v Nastavení (viz [Konfigurace](configuration.md)). |
| **Datové sady ZFS** (volitelné) | Stejné spojení SSH jako u záloh VM, `zfs` na hostiteli a Host Data připojené jako `/mnt` s režimem přístupu Read/Write - Slave, což je výchozí nastavení šablony. Viz [Datové sady ZFS](zfs-datasets.md). |
| **Aplikace pro Android** (nepovinné) | Android 10 nebo novější, spárovaná se servery ve verzi 9.7.0 nebo novější. Viz [Aplikace pro Android](android.md). |

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

## Obecný hostitel Dockeru {#generic-docker-host}

Nemáte Unraid? BombVault běží i jako prostý kontejner na libovolném hostiteli Dockeru (na tom stojí také podpora kontejnerů na TrueNAS Scale, ještě před vlastní položkou v tamním katalogu aplikací).

1. Stáhněte si z repozitáře soubor [`deploy/docker-compose.generic.yml`](https://github.com/junkerderprovinz/bombvault/blob/main/deploy/docker-compose.generic.yml), připravený k úpravám.
2. Nastavte `APP_KEY` (viz níže) a nasměrujte svazek Host Data na svůj skutečný kořen dat: komentáře v souboru provedou obojím.
3. `docker compose up -d`, poté otevřete `https://<ip-hostitele>:3443/`.

V čem se to liší od Unraidu:

- **Žádná doména flash/USB.** Není tu žádný zaváděcí USB disk, který by se dal zachytit nebo obnovit, doména Flash v nastavení tedy nemá co dělat. Místo toho doména Složky nabízí návrh na jedno kliknutí **Přidat přednastavení: konfigurace hostitelského systému** (výchozí sada souborů `/etc`, kterou si před uložením projdete a upravíte), jako praktický obecný protějšek.
- **Žádná nativní oznámení Unraidu.** Vlastní oznamovací kanály BombVaultu (webhook, upozornění na selhání mimo lokalitu a podobně) fungují normálně; vynechává se jen odeslání do oznamovacího systému Unraidu, protože takový systém tu není.
- **Záloha virtuálních strojů je volitelná a potřebuje samostatného hostitele libvirtd dostupného přes SSH.** Viz zakomentovaný blok v souboru compose. Obecný hostitel Dockeru sám o sobě žádného správce virtuálních strojů nemá.
- **Žádný widget na dashboardu.** BombVault Widget je plugin pro Unraid, takže i tento krok odpadá.
- **Jak se najdou data kontejneru.** Bez konvence `appdata` z Unraidu se datová složka kontejneru najde podle segmentů v `DATA_ROOT_SEGMENTS`, pojmenovaných svazků Dockeru, pracovního adresáře projektu Compose a štítku `bombvault.data` (viz [Rozpoznávání zdrojů zálohy](configuration.md#backup-source-detection)). Pojmenované svazky i přednastavení `/etc` dosáhnou jen na cesty uvnitř připojení Host Data, takže nasměrujte Host Data na společného předka, který pokrývá i kořen dat Dockeru.
- **`PLATFORM`.** Nastavte ji na `generic` nebo `truenas`. Když ji nenastavíte, BombVault rozpozná Unraid podle jeho vlastní značky na připojení flash a cokoli jiného bere jako obecného hostitele, a kroky jen pro Unraid se přeskočí, místo aby se zkoušely a selhávaly.

**TrueNAS Scale** jde stejnou cestou přes compose; položka katalogu je v repozitáři připravená, ale zatím nebyla odeslána. Záloha VM tam potřebuje `LIBVIRT_URI`, protože libvirtd na TrueNAS naslouchá na vlastním socketu (`/run/truenas_libvirt/libvirt-sock`), který tři proměnné `LIBVIRT_*` nedokážou vyjádřit (viz [Konfigurace](configuration.md)). Nakolik je to ověřené: záloha zvolu proběhla na skutečném stroji s TrueNAS Scale, na zvolu připojeném k běžící VM, a `zfs snapshot`, `zfs send`, restic a `zfs receive` ho přenesly tam a zpět bajt po bajtu beze změny. Úplná obnova řízená samotným BombVaultem na hardwaru TrueNAS zatím neproběhla a ten zvol byl řídký (sparse), takže propustnost při mnoha gigabajtech není otestovaná. Než se na to spolehnete, vyzkoušejte tam obnovu.

## Jediné povinné nastavení

Jedinou proměnnou, kterou musíte nastavit, je `APP_KEY`, 32bajtové hex tajemství (64 hex znaků) použité k odvození hesla k restic repozitáři.

Vygenerujte si jej na libovolném stroji:

```bash
openssl rand -hex 32
```

Výsledek vložte do pole `APP_KEY` v šabloně (Unraid), nebo do proměnné prostředí `APP_KEY` v `docker-compose.yml` (běžný Docker host).

!!! danger "Neztraťte svůj APP_KEY"
    Ztráta `APP_KEY` učiní vaše šifrované zálohy neobnovitelnými. Uložte jej na bezpečné místo oddělené od serveru. Jakmile BombVault běží, použijte jeho **sadu pro obnovu šifrovacího klíče** na jedno kliknutí (viz [Mimo lokalitu a obnova](offsite-recovery.md)) k uložení kompletního balíčku pro obnovu.

Šablona za vás také připojí Docker socket, flash (`/boot`) a kořen **Host Data** (`/mnt`). *Zdroje* i *cíle* záloh žijí pod Host Data. Kompletní referenci proměnných a nastavení mimo lokalitu najdete v [Konfiguraci](configuration.md).

## První spuštění

![Přehled po první záloze: co je chráněno, co poběží dál a živý protokol.](assets/screenshots/dashboard.png)

*Přehled po první záloze: co je chráněno, co poběží dál a živý protokol.*

1. Otevřete webové rozhraní na `https://<your-unraid-ip>:3443` (samopodepsaný certifikát rovnou z krabice).
2. V **Nastavení** povolte zálohovací domény, které chcete (Kontejnery, VMs, Flash, Autozáloha, Složky, Datové sady ZFS), a vyberte barvu zvýraznění.
3. V záložce **Kontejnery** vyberte kontejner a klikněte na **Zálohovat nyní** pro vytvoření svého prvního bodu obnovení. Cesty repozitářů mají výchozí hodnotu `/mnt/user/bombvault/{container,vms,flash,config,files,zfs}` a vytvoří se při první záloze.
4. Nastavte plánování v **Nastavení, Plány**. Pro kontejnery a VM je k dispozici *Zahrnout vše do plánu* na jedno kliknutí.

!!! tip "Volitelné: zvolte pořadí zálohování"
    Pokud by se některé kontejnery měly vždy zálohovat před ostatními (například databáze před aplikací, která ji používá), otevřete panel **Pořadí záloh** na stránce Kontejnery a přetáhněte je do požadované sekvence. Naplánované běhy a běhy s vícenásobným výběrem se jím pak řídí; cokoli neuspořádaného se zálohuje od nejvíce po termínu, jako dříve.

!!! note "Kontrola integrace hostitele"
    Po spuštění kontejneru otevřete `/spike` ve webovém rozhraní. Prozkoumá každé připojení a CLI (Docker socket, libvirt, restic, qemu-img, rclone) a nahlásí případné chybějící části, takže si můžete potvrdit, že je kontejner správně zapojen, dříve než se na něj budete spoléhat.

## Jednoduché vs. pokročilé

![Nastavení nemá tlačítko Uložit: každá změna se zapíše hned.](assets/screenshots/settings.png)

*Nastavení nemá tlačítko Uložit: každá změna se zapíše hned.*

Ve výchozím nastavení rozhraní zobrazuje jen to nejnutnější (zálohovat, obnovit, plánovat). Použijte přepínač **Jednoduché zobrazení / Pokročilé zobrazení** v postranním panelu k odhalení expertních ovládacích prvků: uchovávání, kopie mimo lokalitu, pre/post hooky, obnova na úrovni souborů, oznámení, metriky Prometheus a nástroje integrity/údržby. Jde o předvolbu na úrovni prohlížeče, ve výchozím stavu vypnutou, takže nováčci dostanou čisté UI a pokročilí uživatelé dostanou vše.

## Sestavení ze zdrojového kódu {#build-from-source}

BombVault je jediná statická binárka v Go, která obsluhuje JSON API a vestavěné rozhraní v Reactu. Nejprve sestavte rozhraní, pak spusťte binárku:

```bash
npm --prefix web ci
npm --prefix web run build     # writes web/dist, which the binary embeds
export APP_KEY=$(openssl rand -hex 32)
go test ./...                  # unit and integration tests, with a real restic round trip
golangci-lint run ./...
go run ./cmd/bombvault         # serves https://localhost:3443 with a self-signed certificate
```

Sestavení rozhraní je potřeba i pro `go run`. Repozitář obsahuje pod `web/dist` jen prázdnou značku, takže bez `npm --prefix web run build` binárka nic nevloží a odpoví `500 SPA index not found`, což je očekávané. Docker, libvirt a Unraid nejde v CI otestovat, takže připojení, restic a SSH spojení k VM ověřte na skutečném hostiteli pomocí Kontroly integrace hostitele (`/spike`), než otevřete pull request.

## Další kroky

- Projděte si kompletní **[Funkce](features.md)**.
- Dejte si všechny servery své skupiny do telefonu s **[aplikací pro Android](android.md)**.
- Přidejte jednu nebo více replik **[Mimo lokalitu a obnova](offsite-recovery.md)** (každá doména může odesílat na několik cílů najednou) a uložte si svou sadu pro obnovu.
- Klonujete sestavu nebo přecházíte na nový stroj? Přeneste celou svou konfiguraci pomocí karty **Export / import nastavení**. Viz [Konfigurace](configuration.md#portable-settings-export-and-import).
- Narazili jste na problém? Viz **[Řešení problémů](troubleshooting.md)**.
