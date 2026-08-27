# Konfigurace

Tato stránka pokrývá proměnné prostředí kontejneru, připojení, která šablona poskytuje, zálohu VM přes SSH a nastavení mimo lokalitu. **Cesty repozitářů** záloh se konfigurují uvnitř aplikace (Nastavení, Zálohovací cesty), nikoli přes proměnné prostředí.

## Proměnné prostředí

| Proměnná | Povinná | Popis |
|---|---|---|
| `APP_KEY` | **Ano** | 32bajtové hex tajemství (64 hex znaků) použité k odvození hesla k restic repozitáři. Vygenerujte pomocí `openssl rand -hex 32`. Uchovejte v bezpečí: jeho ztráta učiní šifrované zálohy neobnovitelnými. |
| `LIBVIRT_HOST` | Pro VM | Hostitel Unraidu dosažený přes SSH pro zálohu VM (výchozí `host.docker.internal`; šablona předvyplní zástupný symbol LAN IP). Použijte svou LAN IP Unraidu, povinné na vlastní síti `br0.x`. |
| `LIBVIRT_SSH_PORT` | Ne | SSH port hostitele pro zálohu VM (výchozí `22`). |
| `LIBVIRT_SSH_USER` | Ne | SSH uživatel na hostiteli pro zálohu VM (výchozí `root`). |
| `LIBVIRT_URI` | Ne | Úplné URI připojení k libvirt, použité **doslovně** místo sestavení ze tří výše uvedených proměnných `LIBVIRT_*` (ty se pak pro sestavení URI ignorují). Ve výchozím stavu nenastaveno. Potřebné na TrueNAS Scale, jehož libvirtd naslouchá na nestandardním socketu, který sestavená podoba nedokáže vyjádřit: `qemu+ssh://<user>@<truenas-host>/system?socket=/run/truenas_libvirt/libvirt-sock`. Viz sekce TrueNAS Scale v [docs/vm-backup-ssh-setup.md](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md). |
| `PORT` | Ne | HTTP port (výchozí `3000`; použit jen s `HTTP_ONLY=true`). |
| `HTTPS_PORT` | Ne | HTTPS port (výchozí `3443`; šablona jej publikuje 1:1, takže WebUI odpovídá na `https://<ip>:3443`). |
| `HTTP_ONLY` | Ne | Nastavte `true` pro zakázání samopodepsaného HTTPS listeneru a obsluhu pouze prostého HTTP (pro použití za reverzní proxy terminující TLS). |
| `HOST_SOURCE_ROOT` | Ne | Hostitelská cesta připojená jako **Host Data** (výchozí `/mnt`). BombVault překládá zdroje bind-mountů, které Docker hlásí, na cesty pod tímto připojením. Změňte jen pokud jste připojili jiný kořen hostitele. |
| `DATA_ROOT_SEGMENTS` | Ne | Čárkou oddělené názvy segmentů cesty, které označují zdroj bind-mountu jako zálohovaná data (výchozí `appdata`, podle konvence Unraidu `/mnt/user/appdata/<container>`). Bind-mount kontejneru je pro zálohu automaticky vybrán, když se KTERÝKOLI uvedený segment objeví jako celý segment cesty jeho zdroje na hostiteli; například `DATA_ROOT_SEGMENTS=appdata,config` zachytí i připojení `.../config`. Další, vždy aktivní způsoby, jak se najde datová složka kontejneru, viz [Detekce zdroje zálohy](#backup-source-detection). |
| `PLATFORM` | Ne | Vynutí platformu, na které BombVault předpokládá, že běží, místo automatické detekce: `unraid`, `generic` nebo `truenas` (výchozí nenastaveno; automaticky detekuje Unraid hledáním jeho značky `dockerMan` pod připojením flash, jinak `generic`; nerozpoznaná hodnota se rovněž vrátí na `generic`, což se zaznamená do logu). Nastavte ji explicitně na obecném Docker hostiteli nebo na TrueNAS Scale, místo spoléhání na automatickou detekci dostupnou jen pro Unraid; obecný compose soubor to tak dělá. Mění konvenci náhradního umístění appdata, výchozí cíle obnovy mezi instancemi a to, zda se vůbec zkouší kroky oznámení/doprovodného pluginu dostupné jen pro Unraid (viz `internal/platform`). |
| `BOMBVAULT_SELF_CONTAINER` | Ne | Název samotného kontejneru BombVault, takže nikdy nezálohuje (a tedy nezastaví) sám sebe (výchozí `BombVault`; automaticky detekován přes hostname na bridge síti). |
| `BACKUP_MAX_HOURS` | Ne | Maximální počet hodin reálného času, po které jeden zálohovací běh smí držet zámek své domény, než je násilně zrušen (pojistka, aby zaseknutý běh nemohl navždy blokovat doménu). Prázdné (výchozí) použije `48`. Zvyšte pro velmi velké nebo pomalé cloudové zálohy (běh zrušený na stropu selže s `context deadline exceeded`). Nastavte `0` pro úplné vypnutí stropu. |
| `TZ` | Ne | Časové pásmo pro plánovač (například `Europe/Berlin`). **Pokud ji nenastavíte, běží všechny plány v UTC**: plán nastavený na 02:30 se pak spustí ve 02:30 UTC, nikoli podle místního času. |

## Připojení

Připojte Docker socket, flash (`/boot`) a kořen **Host Data** (`/mnt`), jak je zobrazeno v CA šabloně. *Zdroje* i *cíle* záloh žijí pod Host Data, a to je připojeno jako **rslave**, takže vzdálená sdílená složka, která se připojí až po spuštění kontejneru (například pod `/mnt/remotes`), se stane viditelnou bez restartu.

Cesty repozitářů záloh mají výchozí hodnotu `/mnt/user/bombvault/{container,vms,flash,config,files}`, vytvořené při první záloze. Umístění změňte kdykoli v **Nastavení, Zálohovací cesty**.

!!! note "Kontrola integrace hostitele"
    Po spuštění kontejneru otevřete `/spike` ve webovém rozhraní. Prozkoumá každé připojení a CLI (Docker socket, libvirt, restic, qemu-img, rclone) a nahlásí případné chybějící části.

## Bezpečnostní model

!!! warning "Kontrola nad hostitelem na úrovni root"
    Skrze Docker socket může BombVault zastavovat, odebírat a znovu vytvářet kontejnery a číst/zapisovat appdata, a pro zálohu VM se přihlašuje k hostiteli přes SSH (`qemu+ssh://`, ve výchozím stavu root), aby spustil `virsh`. Kdokoli, kdo se dostane k jeho webovému rozhraní, má fakticky root na hostiteli.

- **Volitelná ochrana heslem** (Nastavení, Zabezpečení): nastavte heslo pro vyžadování přihlášení, vymažte jej pro zakázání. Ve výchozím stavu vypnuto pro použití v důvěryhodné LAN. Relace jsou podepsané (HMAC odvozený z `APP_KEY`) a změna hesla je zneplatní; přihlášení jsou rate-limitovaná.
- Protože je ochrana volitelná, když není nastavena, jsou celé UI a API (včetně nastavení mimo lokalitu, tras testu odolnosti a sady pro obnovu) dosažitelné každým, kdo se dostane k portu. Zapněte ochranu, jakmile používáte mimo lokalitu, neměnné zálohy nebo šifrování.
- Provozujte BombVault pouze v důvěryhodné, nevystavené síti. Pro vzdálený přístup jej umístěte za reverzní proxy, která přidává autentizaci a TLS. Odpovědi nesou základní bezpečnostní hlavičky (CSP, `nosniff`, `X-Frame-Options`, `Referrer-Policy`).
- S `HTTP_ONLY=true` ztrácí session cookie svůj příznak `Secure` (musí, aby fungovala přes prosté HTTP), takže zapněte heslo za proxy terminující TLS jen pokud je důvěrnost důležitá.
- SSH připojení pro zálohu VM důvěřuje hostitelskému klíči při prvním připojení (TOFU) a poté jej připne. Ověřte hostitelský klíč mimo pásmo, pokud vaše cesta z kontejneru k hostiteli není důvěryhodná.
- Zálohy jsou šifrovány pomocí restic, když je šifrování povoleno (Nastavení; ve výchozím stavu zapnuto), s klíčem odvozeným z `APP_KEY`.

## Záloha VM přes SSH

BombVault zálohuje KVM/libvirt VM **bez připojení jakékoli libvirt cesty**. Spouští `virsh` na hostiteli přes SSH (`qemu+ssh://`), takže nikdy nemůže ovlivnit váš hostitelský VM Manager.

Rychlé nastavení:

1. **Nastavení, Systém, Záloha VM přes SSH:** zkopírujte zobrazený veřejný klíč.
2. Připojte jej do `/root/.ssh/authorized_keys` Unraidu (také persistováno na flash, aby přežilo restarty).
3. Klikněte na **Otestovat připojení**.

Šablona přidává `--add-host=host.docker.internal:host-gateway`, aby kontejner dosáhl na hostitele. Nastavte `LIBVIRT_HOST` na svou LAN IP Unraidu, pokud se ten název neresolvuje (například když kontejner běží na vlastní síti `br0.x`). Pokud jste změnili SSH port Unraidu, nastavte `LIBVIRT_SSH_PORT`, aby odpovídal. **Živé snímky** navíc potřebují qemu guest agent ve VM a disk na `/mnt/cache` (nikoli `/mnt/user`).

!!! important "Kompletní průvodce nastavením VM a sítí"
    Kompletní krok za krokem průvodce (povolení SSH, persistentní autorizace klíče, směrování ve vlastní síti a VLAN, metoda na VM a řešení problémů na straně hostitele) žije na [docs/vm-backup-ssh-setup.md](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) na GitHubu.

## Nastavení mimo lokalitu

Nastavte repliku mimo lokalitu v záložce **Nastavení, Mimo lokalitu**. Kompletní postup (neměnné/append-only, testování odolnosti a cvičné obnovy po havárii) najdete v [Mimo lokalitu a obnova](offsite-recovery.md). Ve zkratce:

- **Backendy:** SMB/CIFS a NFS (připojte sdílenou složku a nasměrujte na ni Zálohovací cestu), nativní restic backendy bez rclone (`s3:...`, `rest:http://host:8000/repo`, `b2:...`, `sftp:user@host:/repo`) nebo libovolný rclone remote (`rclone:<remote>:<bucket>/path`).
- **Přihlašovací údaje cloudu** se ukládají šifrovaně pod Nastavení, Mimo lokalitu, Přihlašovací údaje cloudu.
- **SSH cíle nevyžadují nic nainstalovaného na druhé straně.** `sftp:` potřebuje jen SSH server. Přidejte veřejný klíč z **Nastavení, Systém, Záloha VM přes SSH** (také na `/config/ssh/id_ed25519.pub`) do `~/.ssh/authorized_keys` cílového uživatele.
- **Kopie mimo lokalitu:** BombVault replikuje nové snímky pomocí `restic copy` na základě nejlepší snahy. Místní repozitář zůstává primární. Každá doména má vlastní plán mimo lokalitu, plus tlačítko **Replikovat nyní**.
- **Více cílů mimo lokalitu na doménu:** každá doména může replikovat na několik cílů mimo lokalitu najednou. Přidejte další cíle v Nastavení, Mimo lokalitu, každý s vlastním repozitářem, třídou úložiště S3, příznakem append-only, uchováváním a rozpočtem růstu; všechny replikují podle plánu mimo lokalitu dané domény. Stávající jednotlivé nastavení mimo lokalitu se přenese jako první cíl.
- **Uchovávání na zdroj:** místní zásada žije v Nastavení, Cesty a úložiště; zásada mimo lokalitu v Nastavení, Mimo lokalitu (ponechte vše na nule, aby se snímky mimo lokalitu nikdy automaticky neprořezávaly).
- **Limity šířky pásma:** omezte rychlost nahrávání/stahování restic pod Nastavení, Mimo lokalitu.
- **Studená a archivní třída úložiště (S3):** pro nativní S3 repozitář mimo lokalitu vyberte úroveň čitelnou pro obnovu (Standard, Standard-IA, One Zone-IA, Intelligent-Tiering, Glacier Instant Retrieval). rclone remotes nastavují svou třídu v konfiguraci rclone.

## Přenositelná nastavení (export a import) {#portable-settings-export-and-import}

Karta **Export a import nastavení** na stránce Nastavení zapíše celou vaši konfiguraci BombVaultu (nastavení domén, cíle mimo lokalitu, plány, uchovávání, oznámení) do přenosného souboru JSON, který můžete importovat na jiné instanci, takže přechod na nový stroj nebo klonování sestavy neznamená znovu vše zadávat ručně. Import zobrazí náhled a požádá o potvrzení a nikdy se nedotkne vašich zálohovaných dat ani historie.

!!! warning "Export může obsahovat přihlašovací údaje"
    Vy zvolíte, zda do souboru zahrnout přihlašovací údaje mimo lokalitu a oznámení. Se zahrnutými přihlašovacími údaji je export stejně citlivý jako vaše sada pro obnovu, takže jej uložte na bezpečné místo. Bez nich soubor obsahuje jen netajná nastavení.
