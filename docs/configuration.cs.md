# Konfigurace

Tato stránka popisuje proměnné prostředí kontejneru, připojení, která poskytuje šablona, bezpečnostní model, zálohování VM přes SSH a nastavení mimo lokalitu. Kam se zálohy ukládají, se nastavuje přímo v aplikaci jako **cíle**, ne proměnnými prostředí.

## Proměnné prostředí

| Proměnná | Povinná | Popis |
|---|---|---|
| `APP_KEY` | **Ano** | 32bajtové hex tajemství (64 hex znaků) použité k odvození hesla k restic repozitáři. Vygeneruj pomocí `openssl rand -hex 32`. Uchovej v bezpečí: jeho ztráta učiní šifrované zálohy neobnovitelnými. |
| `LIBVIRT_HOST` | Pro VM a datové sady ZFS | Hostitel Unraidu dosažený přes SSH pro zálohu VM (výchozí `host.docker.internal`; šablona předvyplní zástupný symbol LAN IP). Použij svou LAN IP Unraidu, povinné na vlastní síti `br0.x`. Používá se i pro zálohy datových sad ZFS (pole šablony **Host SSH: Address**); zástupná hodnota `192.168.x.x` se bere jako nenastavená. |
| `LIBVIRT_SSH_PORT` | Ne | SSH port hostitele pro zálohu VM (výchozí `22`). Pole šablony **Host SSH: Port**, platí i pro datové sady ZFS. |
| `LIBVIRT_SSH_USER` | Ne | SSH uživatel na hostiteli pro zálohu VM (výchozí `root`). Pole šablony **Host SSH: User**, platí i pro datové sady ZFS. |
| `LIBVIRT_URI` | Ne | Úplné URI připojení k libvirt, použité **doslovně** místo sestavení ze tří výše uvedených proměnných `LIBVIRT_*` (ty se pak pro sestavení URI ignorují). Ve výchozím stavu nenastaveno. Potřebné na TrueNAS Scale, jehož libvirtd naslouchá na nestandardním socketu, který sestavená podoba nedokáže vyjádřit: `qemu+ssh://<user>@<truenas-host>/system?socket=/run/truenas_libvirt/libvirt-sock`. Viz sekce TrueNAS Scale v [docs/vm-backup-ssh-setup.md](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md). Je-li to URI `qemu+ssh://`, převezme se z něj každá z proměnných `LIBVIRT_HOST`, `LIBVIRT_SSH_USER` a `LIBVIRT_SSH_PORT`, která není nastavená, i pro vlastní SSH příkazy BombVaultu (přenos NVRAM, datové sady ZFS). |
| `PORT` | Ne | HTTP port (výchozí `3000`; použit jen s `HTTP_ONLY=true`). |
| `HTTPS_PORT` | Ne | HTTPS port (výchozí `3443`; šablona jej publikuje 1:1, takže WebUI odpovídá na `https://<ip>:3443`). |
| `HTTP_ONLY` | Ne | Nastav `true` pro zakázání samopodepsaného HTTPS listeneru a obsluhu pouze prostého HTTP (pro použití za reverzní proxy terminující TLS). |
| `BIND_HOST` | Ne | Adresa, na které WebUI naslouchá (výchozí `0.0.0.0`, všechna rozhraní). V kontejneru ji nenastavuj, jeho publikované porty potřebují všechna rozhraní; `127.0.0.1` se hodí pro běh mimo Docker. Healthcheck se ptá na stejnou adresu. |
| `TRUSTED_PROXY` | Ne | Čárkami oddělené adresy nebo rozsahy CIDR reverzní proxy před BombVaultem (například `192.168.20.11` nebo `10.0.0.0/8`). Jen z těchto uzlů se věří hlavičce `X-Forwarded-For` a omezení přihlašování pak počítá neúspěchy na skutečného klienta místo toho, aby všechny za proxy házelo do jednoho pytle. Nenastaveno (výchozí) znamená nevěřit nikomu: bezpodmínečně věřená hlavička by každému volajícímu dovolila zvolit si vlastní počítadlo. |
| `HOST_SOURCE_ROOT` | Ne | Hostitelská cesta připojená jako **Host Data** (výchozí `/mnt`). BombVault překládá zdroje bind-mountů, které Docker hlásí, na cesty pod tímto připojením. Změň jen pokud jsi připojil jiný kořen hostitele. |
| `DATA_ROOT_SEGMENTS` | Ne | Čárkou oddělené názvy segmentů cesty, které označují zdroj bind-mountu jako zálohovaná data (výchozí `appdata`, podle konvence Unraidu `/mnt/user/appdata/<container>`). Bind-mount kontejneru je pro zálohu automaticky vybrán, když se KTERÝKOLI uvedený segment objeví jako celý segment cesty jeho zdroje na hostiteli; například `DATA_ROOT_SEGMENTS=appdata,config` zachytí i připojení `.../config`. Další, vždy aktivní způsoby, jak se najde datová složka kontejneru, viz [Detekce zdroje zálohy](#backup-source-detection). |
| `PLATFORM` | Ne | Vynutí platformu, na které BombVault předpokládá, že běží, místo automatické detekce: `unraid`, `generic` nebo `truenas` (výchozí nenastaveno; automaticky detekuje Unraid hledáním jeho značky `dockerMan` pod připojením flash, jinak `generic`; nerozpoznaná hodnota se rovněž vrátí na `generic`, což se zaznamená do logu). Nastav ji explicitně na obecném Docker hostiteli nebo na TrueNAS Scale, místo spoléhání na automatickou detekci dostupnou jen pro Unraid; obecný compose soubor to tak dělá. Mění konvenci náhradního umístění appdata, výchozí cíle obnovy mezi instancemi a to, zda se vůbec zkouší kroky oznámení/doprovodného pluginu dostupné jen pro Unraid (viz `internal/platform`). |
| `BOMBVAULT_SELF_CONTAINER` | Ne | Název samotného kontejneru BombVault, takže nikdy nezálohuje (a tedy nezastaví) sám sebe. |
| `BACKUP_MAX_HOURS` | Ne | Maximální počet hodin reálného času, po které jeden zálohovací běh smí držet zámek své sekce, než je násilně zrušen (pojistka, aby zaseknutý běh nemohl navždy blokovat sekci). Prázdné (výchozí) použije `48`. Zvyš pro velmi velké nebo pomalé cloudové zálohy (běh zrušený na stropu selže s `context deadline exceeded`). Nastav `0` pro úplné vypnutí stropu. |
| `BACKUP_STALL_HOURS` | Ne | Počet hodin, po které záloha smí **vůbec nepostupovat**, než je zrušena. Prázdné (výchozí) použije `2`; nastav `0`, aby se kvůli zastavení nikdy nerušila. Je to jemnější z obou pojistek a obvykle ta, která zasáhne: sleduje, zda se ještě něco děje, a ne jak dlouho běh trvá, takže pomalá, ale zdravá záloha mnoha terabajtů zůstane nedotčena, zatímco záloha zaseknutá na nereagující sdílené složce se zastaví za hodiny místo za dny. Po 30 minutách ticha, ještě než se cokoli zruší, se do logu zapíše varování. Procházení se počítá jako postup: restic při procházení velkého stromu nezapisuje žádné bajty a tato fáze se sleduje přes jeho součty souborů a bajtů, ne přes zapsané bajty. Obě proměnné jsou nezávislé a `BACKUP_MAX_HOURS` dál ohraničuje fáze po samotné záloze (uchovávání, statistiky, kopie mimo lokalitu), kde nejsou žádné čítače ke sledování. |
| `DB_DUMP_MAX_HOURS` | Ne | Hodiny, po které smí jeden automatický dump databáze běžet, než je zastaven. Prázdné (výchozí) použije `6`; povolené jsou hodnoty `1` až `48` a limit zůstává hodinu pod `BACKUP_MAX_HOURS` (je-li kratší než dvě hodiny, na jeho polovině), aby dlouhý dump utnul jeho vlastní limit a byl tak i nahlášen, místo aby strhl zálohu s sebou. Dump, který přestane postupovat, se zastaví dřív, po `BACKUP_STALL_HOURS`. Zastavený dump selže sám za sebe a záloha kontejneru pokračuje. Na Unraidu přidej proměnnou ke kontejneru BombVault přes **Add another Path, Port, Variable**. |
| `TZ` | Ne | Časové pásmo pro plánovač (například `Europe/Berlin`). **Pokud ji nenastavíš, běží všechny plány v UTC**: plán nastavený na 02:30 se pak spustí ve 02:30 UTC, nikoli podle místního času. Na Unraidu to nikdy nenastavuješ sám: systém předává vlastní časové pásmo do každého kontejneru. Úvodní log při spuštění uvádí, na které pásmo se nastavení přeložilo. Pásmo s letním časem na jaře jeden běh vynechá a na podzim jeden spustí dvakrát, kdežto UTC nedělá ani jedno, zato se dvakrát do roka posune o hodinu oproti tvým hodinám. |

## Připojení

Připoj Docker socket, flash (`/boot`) a kořen **Host Data** (`/mnt`), jak je zobrazeno v CA šabloně. *Zdroje* i *cíle* záloh žijí pod Host Data, a to je připojeno jako **slave**, takže vzdálená sdílená složka, která se připojí až po spuštění kontejneru (například pod `/mnt/remotes`), se stane viditelnou bez restartu.

Zálohy datových sad ZFS tento režim potřebují také: snímek datové sady hostitel připojí až poté, co kontejner nastartoval. Viz [Datové sady ZFS](zfs-datasets.md).

Místní repozitáře mají ve výchozím stavu `/mnt/user/bombvault/{container,vms,flash,config,files,zfs}` a vytvoří se při první záloze. Tvoří cíl **Místní složka**. Na jeho stránce v **Cíle** karta **Připojení** mění složku každé sekce a okno, které otevře, má přepínač **Místní / Vzdálené**: cesta sekce může být vzdálený restic (`s3:...`, `rest:...`, `sftp:...`, `rclone:...`) místo místní složky a zálohuje se přímo do něj bez samostatné místní kopie; viz [Vzdálené primární repozitáře](offsite-recovery.md#remote-primary-repositories).

!!! note "Kontrola integrace hostitele"
    Karta **Integrace hostitele** v Přehledu (přidáš ji pod **Přizpůsobit**) prozkoumá každé připojení a CLI (Docker socket, libvirt, restic, qemu-img, rclone) a nahlásí případné chybějící části.

## Rozpoznávání zdrojů zálohy {#backup-source-detection}

Pro každý kontejner BombVault sám vybírá, které bind mounty a pojmenované svazky zálohovat. Cesta se vybere, když platí cokoli z následujícího (výsledek můžeš vždy upravit v kartě **Zálohované složky** na záložce Nastavení kontejneru):

- **Shoda se segmentem kořene dat:** hostitelský zdroj bindu obsahuje jeden ze segmentů `DATA_ROOT_SEGMENTS` jako celou složku cesty (ve výchozím stavu pouze `appdata`).
- **Pojmenované svazky Dockeru** se zahrnují vždy, protože k nim neexistuje jednorázový protějšek, a není tedy co filtrovat, **ale jen tehdy, když je skutečná úložná cesta svazku na hostiteli dosažitelná přes připojení Host Data**, přesně jako každá jiná hostitelská cesta, kterou BombVault zálohuje. Výchozí ovladač místních svazků ukládá svazek pod kořen dat samotného démona, tedy `/var/lib/docker/volumes/<název>/_data`, pokud to nebylo upraveno (ověříš příkazem `docker info -f '{{.DockerRootDir}}'`). Toto místo NENÍ pokryto úzkým, jednoadresářovým připojením Host Data, které obecný `docker-compose.yml` ve výchozím stavu používá. Nedosažitelný svazek se tiše přeskočí, není to chyba. Aby se pojmenované svazky na obecném hostiteli opravdu zálohovaly, nasměruj Host Data (a `HOST_SOURCE_ROOT`) na společného předka, který pokrývá i kořen dat Dockeru: kompromis popisuje komentář Host Data v souboru compose (Unraid to obchází tím, že ze stejného důvodu připojí celé `/mnt`, svou vlastní univerzální konvenci nejvyšší úrovně).
- **Adresář projektu Docker Compose:** nese-li kontejner obvyklý štítek `com.docker.compose.project.working_dir` (nastavuje jej automaticky `docker compose up`), přidá se i tento adresář, bez ohledu na to, zda některý bind odpovídal segmentu kořene dat.
- **Přepis štítkem `bombvault.data`:** nastav kontejneru štítek `bombvault.data=true`, aby se zahrnula VŠECHNA jeho připojení bind, pro uspořádání, které nezachytí ani jedna z výše uvedených konvencí (například jediný bind `/srv/plex/config` bez projektu Compose). Jakákoli neprázdná hodnota jiná než `false` platí jako pravda; chybějící štítek nebo `bombvault.data=false` nemění nic.
- **Štítek `bombvault.dbdump`:** nastav `bombvault.dbdump=false` na kontejneru, aby se vypnul jeho automatický výpis databáze (totéž dělají `0`, `no` a `off`), nebo uveď engine (`postgres`, `mysql`, `mariadb`), aby se vypsal kontejner, který BombVault sám nerozpozná. Štítek má přednost před přepínačem v kartě **Databáze** na záložce Nastavení kontejneru, což je způsob, jak to udělat na Unraidu.

## Bezpečnostní model {#security-model}

!!! warning "Kontrola nad hostitelem na úrovni root"
    Skrze Docker socket může BombVault zastavovat, odebírat a znovu vytvářet kontejnery a číst/zapisovat appdata, a pro zálohu VM se přihlašuje k hostiteli přes SSH (`qemu+ssh://`, ve výchozím stavu root), aby spustil `virsh`. Kdokoli, kdo se dostane k jeho webovému rozhraní, má fakticky root na hostiteli.

### Přihlášení {#login}

- **Volitelná ochrana heslem** (karta **Heslo** v **Nastavení, Zabezpečení**): nastavením hesla se vyžaduje přihlášení, jeho smazáním se vypne. Ve výchozím stavu vypnuto pro důvěryhodnou LAN. Heslo se ukládá pomocí Argon2id nad pepřem navázaným na `APP_KEY`, takže zkopírovaný `/config` je bez klíče bezcenný a s ním pomalý k napadení. Nové heslo potřebuje alespoň 12 znaků; stávající kratší funguje, dokud se nezmění. Relace jsou podepsané (HMAC odvozený z `APP_KEY`) a změna hesla je zneplatní; přihlášení je omezeno na pět neúspěchů za minutu na klienta.
- **Některé věci vyžadují heslo.** Stažení sady pro obnovu a hlášení o chybě, uložení klíče u strážce klíče, povolení vydání úschovy, vytvoření nového kódu úschovy, smazání záloh v cíli, export nastavení s přihlašovacími údaji, dvoufázové ověření a passkeys jsou odmítnuty, dokud není nastaveno přihlašovací heslo, a vyžadují přihlášení přímo na instanci.
- **Dvoufaktorové ověření** (**Nastavení, Zabezpečení**): časový kód z aplikace pro ověřování navíc k heslu, plus osm jednorázových záložních kódů vydaných jednou při zapnutí. Sdílené tajemství je uloženo zašifrované pomocí `APP_KEY` a vypnutí tohoto faktoru vyžaduje aktuální kód.
- **Přístupové klíče** (WebAuthn) mají vlastní kartu, jakmile je nastavené heslo, a fungují vedle hesla, nikdy místo něj, takže odebráním všech přístupových klíčů se nikdo nezamkne. Potřebují skutečný doménový název a certifikát, kterému prohlížeč důvěřuje. Výchozí `https://<ip>:3443` je přesně to, co WebAuthn odmítá, a karta to řekne, místo aby nabídla tlačítko, které selže.
- **Zapomenuté heslo** se resetuje na přihlašovací stránce. **Zapomenuté heslo?** způsobí, že BombVault zapíše kód o 12 znacích ve třech skupinách po čtyřech do logu svého kontejneru (na Unraidu: záložka Docker, ikona BombVault, **Logs**). Tento kód a nové heslo tě přihlásí. Reset také vypne dvoufázové ověření, odstraní všechny passkeys, zruší všechny API tokeny, klíče MCP a přihlášení OAuth, ukončí všechny ostatní relace, odhlásí všechny spárované telefony a vypne párování telefonů; stránka potom vypíše, co zmizelo. Kód platí 10 minut a špatné pokusy ho nespotřebují. Tokeny widgetu a metrik zůstávají, protože jen čtou.
- **Kdo může číst log kontejneru, může resetovat heslo.** To je root na hostiteli, ale také prohlížeč logů jako Dozzle, každý kontejner s přidělenou Docker socket (stačí jen pro čtení) a nástroj, který přeposílá výstup kontejnerů. Dávej je jen lidem, kterým bys svěřil heslo. Kdo se dostane jen k webovému rozhraní, kód nikdy neuvidí.

### Telefony a členové skupiny {#phones-and-members}

- **Spárované telefony dosáhnou na povolený seznam.** Telefon, který zná dvanáct slov skupiny, se přihlásí bez hesla, dokud je zapnuto **Telefony se mohou spárovat s dvanácti slovy skupiny** v kartě **Aplikace pro Android** v **Nastavení, Zabezpečení**. Jeho relace sahá jen k tomu, co aplikace ukazuje: přehled, běhy, zálohy k prohlížení, spuštění a obnově, anomálie ke čtení a stránku Instance k prohlížení. Cíle a příkazy k němu dorazí bez adres a tajemství a obnova, která nahrazuje originál, nejdřív zazálohuje aktuální stav. Všechno ostatní odpoví, že potřebuje přihlášení přímo na instanci: klíče, sada pro obnovu, úschovy klíčů, nastavení, mazání záloh. Reset hesla vypne párování telefonů, dokud ho znovu nezapneš.
- **Mimo domov otevírá telefon stránky přes relay.** Každý požadavek stránky je zapečetěný klíčem skupiny, který relay nemůže přečíst ani změnit, a podepsaný vlastním klíčem telefonu, který si server připnul, když se telefonu poprvé zeptal. Server požadavek provede jen jednou a jen dokud je čerstvý. Odpovídá jen telefonu, který je na stránce Instance, dokud je zapnuté párování telefonů, a jen v rámci relace, kterou tomuto telefonu dal, takže odhlášení všude i reset hesla tyto stránky také ukončí. Server podepisuje každou odpověď a aplikace ověřuje podpis proti klíči, který si pro tento server připnula, a při změně klíče upozorní. Taková stránka dosáhne na stejný povolený seznam jako spárovaný telefon, s přihlašovacím heslem i bez něj, a navíc na stav přihlášení, který stránka čte jako první. Přihlášení a odhlášení, reset hesla a stahování zůstávají u vlastní adresy serveru a jedna odpověď je omezena na 16 MB.
- **Členové podepisují, o co žádají.** Žádosti o role a odpovědi na ně, vydání hesla resticu příjemci nebo stahovači, přihlášení příjemce a úschovy klíčů se podepisují klíčem každého člena, který si instance připne, když člena poprvé uvidí. Volání podepsané jiným klíčem se odmítne. **Zapomenout klíč** v podrobnostech instance připnutý klíč zahodí, pro případ, že ten známý tady není klíčem samotného člena; člen je při dalším podepsaném volání požádán o klíč znovu. Členové na BombVaultu 9.x nepodepisují a kdokoli se slovy je může napodobit, jako v 9.9.
- **Dvanáct slov je heslo.** Kdo je má, vstoupí do skupiny, vidí stav ochrany členů a může žádat o role, které musí osoba na druhé instanci ještě povolit. Bez přihlašovacího hesla je může přečíst každý, kdo otevře webové rozhraní, a karta párování to říká, dokud se heslo nenastaví.

### Klíče {#keys}

- **APP_KEY dělá dvě věci.** Zapečeťuje, co tato instalace ukládá (hesla, přihlášení, tokeny), a je klíčem záloh: odvozuje se z něj heslo resticu a zapečeťují se jím definice kontejnerů a VM.
- **Převzatý klíč se drží zvlášť.** Když Nouzový průvodce převezme zálohy jiného serveru, tato instalace dál pečeťuje vlastním APP_KEY a klíč druhého serveru uloží jako svůj **Klíč záloh**, zapečetěný jako každé jiné tajemství. Sada pro obnovu pak vytiskne oba. Pokud se APP_KEY později změní, klíč záloh už nejde otevřít: BombVault přestane zálohovat a řekne to pod **Vyžaduje pozornost**, místo aby zapisoval špatným klíčem. Nahraj sadu pro obnovu znovu v **Obnova, Nouze**, nebo vrať APP_KEY zpět. Viz [Nouzový průvodce](offsite-recovery.md#emergency).
- **Šifrování** je ve výchozím stavu zapnuté (**Šifrovat zálohy** v kartě **Šifrování** v **Nastavení, Zabezpečení**). Zálohy šifruje restic heslem odvozeným z klíče záloh.
- **Úschova klíčů.** Strážce klíče ve tvé skupině drží zapečetěnou kopii sady pro obnovu tohoto serveru, kterou sám otevřít nemůže. Otevře ji jen kód úschovy o 20 znacích a strážce ji vydá, až když tam někdo stiskne **Povolit**. Viz [Strážce klíče](offsite-recovery.md#key-keeper).

### Síť {#network}

- **Změny vyžadují JSON.** Požadavek, který něco mění, musí posílat `Content-Type: application/json` a prohlížeč ho nesmí označit jako cross-site, takže stránka na jiném webu nemůže přimět tvůj prohlížeč, aby změnil nastavení na adrese v LAN. Skript, který ovládá API, tuto hlavičku posílá; cokoli jiného se odmítne s `415`.
- Protože je brána volitelná, je při nenastavení celé UI a API (včetně cílů, tamper-test cest a cest obnovy) dostupné každému, kdo se dostane na port. Nastav přihlašovací heslo, jakmile používáš zálohy mimo lokalitu, append-only zálohy nebo šifrování.
- Provozuj BombVault pouze v důvěryhodné, nevystavené síti. Pro vzdálený přístup jej umísti za reverzní proxy, která přidává autentizaci a TLS. Odpovědi nesou základní bezpečnostní hlavičky (CSP, `nosniff`, `X-Frame-Options`, `Referrer-Policy`).
- Za reverzní proxy nese každý požadavek adresu proxy, takže bez `TRUSTED_PROXY` počítá omezení všechny klienty dohromady a neúspěchy útočníka zamknou i tebe. Uveď proxy v `TRUSTED_PROXY` a počítání se vrátí na jednotlivé klienty.
- Reverzní proxy před BombVaultem musí předávat hlavičku `Authorization` nebo `X-API-Key` na `/mcp` a nesmí bufferovat odpovědi, jinak se asistenti nepřipojí. Viz [Server MCP](mcp.md#tls).
- Endpoint MCP `/mcp` odpovídá `404`, dokud neexistuje klíč nebo není zapnuté přihlašování přes OAuth, a od každého klienta vyžaduje jeho klíč nebo token i při vypnutém přihlašovacím heslu; žádná adresa není výjimkou, ani `localhost`. Nemá nástroje pro obnovu ani mazání a obnova autozálohy odvolá všechny klíče. Viz [Server MCP](mcp.md#security).
- S `HTTP_ONLY=true` ztrácí session cookie svůj příznak `Secure` (musí, aby fungovala přes prosté HTTP), takže zapni heslo za proxy terminující TLS jen pokud je důvěrnost důležitá.
- SSH připojení pro zálohu VM důvěřuje hostitelskému klíči při prvním připojení (TOFU) a poté jej připne. Ověř hostitelský klíč mimo pásmo, pokud tvoje cesta z kontejneru k hostiteli není důvěryhodná.

## Server MCP {#mcp-server}

Server MCP nepotřebuje žádnou proměnnou prostředí. Zapneš ho vytvořením klíče v **Nastavení, Integrace, Server MCP** a odpovídá na `/mcp` na stejném portu jako webové rozhraní (například `https://192.168.1.10:3443/mcp`). Bez aktivního klíče tato cesta odpovídá `404`. Klienty, certifikáty a limity popisuje stránka [Server MCP](mcp.md).

## Záloha VM přes SSH

BombVault zálohuje KVM/libvirt VM **bez připojení jakékoli libvirt cesty**. Spouští `virsh` na hostiteli přes SSH (`qemu+ssh://`), takže nikdy nemůže ovlivnit tvůj hostitelský VM Manager.

Připojit socket libvirt hostitele do kontejneru je na Unraidu křehké: tyto cesty spravuje VM Manager a přepnutí "Enable VMs" může libvirt zanechat ve stavu, kdy se nedokáže spustit. SSH klíč dává na hostiteli práva roota, tedy stejnou úroveň důvěry jako Docker socket, který už BombVault používá.

Rychlé nastavení:

1. **Nastavení, Integrace, SSH k hostiteli:** zkopíruj zobrazený veřejný klíč.
2. Připoj jej do `/root/.ssh/authorized_keys` Unraidu (také persistováno na flash, aby přežilo restarty).
3. Klikni na **Otestovat připojení**.

Šablona přidává `--add-host=host.docker.internal:host-gateway`, aby kontejner dosáhl na hostitele. Nastav `LIBVIRT_HOST` na svou LAN IP Unraidu, pokud se ten název neresolvuje (například když kontejner běží na vlastní síti `br0.x`). Pokud jsi změnil SSH port Unraidu, nastav `LIBVIRT_SSH_PORT`, aby odpovídal. **Živé snímky** navíc potřebují qemu guest agent ve VM a disk na `/mnt/cache` (nikoli `/mnt/user`).

!!! important "Kompletní průvodce nastavením VM a sítí"
    Kompletní krok za krokem průvodce (povolení SSH, persistentní autorizace klíče, směrování ve vlastní síti a VLAN, metoda na VM a řešení problémů na straně hostitele) žije na [docs/vm-backup-ssh-setup.md](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) na GitHubu.

## Nastavení mimo lokalitu

Kopie mimo lokalitu jdou do **cílů**. Otevři **Cíle** z jejich dlaždice v nastavení (nebo je připni do postranního panelu) a zvol **Přidat cíl**. Úplný postup (ochrana proti mazání, tamper testy a DR cvičení) viz [Mimo lokalitu a obnova](offsite-recovery.md). Ve zkratce:

- **Backendy:** SMB/CIFS a WebDAV přes rclone bez připojení na hostiteli, NFS (připoj export na hostiteli a použij ho jako místní složku), nativní backendy resticu bez rclone (`s3:...`, `rest:http://host:8000/repo`, `sftp:user@host:/repo`) nebo libovolný rclone remote (`rclone:<remote>:<bucket>/path`). Backblaze B2 tu nativní backend nemá: dosáhni na něj přes jeho S3 endpoint (`s3:https://s3.<region>.backblazeb2.com/<bucket>/<path>`) s ID klíče a aplikačním klíčem jako S3 přihlašovacími údaji.
- **Přihlašovací údaje** se ukládají zašifrované, buď jako sdílené cloudové přihlašovací údaje, nebo jako pojmenovaná sada. Obojí jsou karty v **Nastavení, Cíle** a stránka samotného cíle vybírá sadu, kterou používá, pod **Přihlašovací údaje**.
- **SSH cíle nevyžadují nic nainstalovaného na druhé straně.** `sftp:` potřebuje jen SSH server. Přidej veřejný klíč z **Nastavení, Integrace, SSH k hostiteli** (také na `/config/ssh/id_ed25519.pub`) do `~/.ssh/authorized_keys` cílového uživatele.
- **Kopie mimo lokalitu:** BombVault kopíruje nové zálohy pomocí `restic copy` na základě nejlepší snahy, nad (obvykle místním) primárním repozitářem. Karta **Kopírovat** cíle určuje, zda kopíruje **Po každé záloze**, nebo podle vlastního plánu, a **Nastavení, Plán, Kopie mimo lokalitu podle sekcí** dá jedné sekci vlastní plán kopírování. **Replikovat nyní** je v kartě **Kopie mimo lokalitu** každé sekce v **Nastavení, Cíle**.
- **Několik cílů na sekci:** karta **Cíle podle sekcí** na stránce Cíle zapíná nebo vypíná každou linku ze sekce do cíle. **Pravidla podle sekcí** cíle dají jedné sekci tam vlastní repozitář, uchovávání nebo kompresi.
- **Umístění podle položky:** každý kontejner, VM, sada složek a datová sada ZFS rozsvítí **Místní** a cíle, které dostávají její zálohy, v kartě **Plán a cíl** na záložce Nastavení. Položky bez vlastní volby se řídí svou sekcí. Viz [Umístění podle položky](offsite-recovery.md#placement).
- **Uchovávání:** každý cíl drží zálohy podle své karty **Jak dlouho uchovávat** a sekce tam může mít vlastní hodnoty pod **Pravidla podle sekcí**. Kopie v poli sekce mimo lokalitu drží pravidla karty **Uchovávání mimo lokalitu** v **Nastavení, Cíle**, která má **Pravidla uchovávání podle zdroje** pro jednotlivé sekce (nech vše na nule, aby se zálohy mimo lokalitu nikdy automaticky neořezávaly).
- **Limity šířky pásma:** omez rychlost nahrávání/stahování resticu v kartě **Šířka pásma mimo lokalitu** v **Nastavení, Cíle**, nebo pro každý cíl v jeho kartě **Kopírovat**.
- **Nejdřív streamování:** v kartě **Streamování** na stránce Cíle vyber mediální servery (Plex, Jellyfin a Emby jsou předvybrané podle názvu image), rychlost odesílání, od které se počítá streamování, limit nahrávání při streamování a jak dlouho po streamu se vrátí běžný limit.
- **Studená a archivní třída úložiště (S3):** pro cíl S3 vyber na jeho stránce úroveň čitelnou při obnově (Standard, Standard-IA, One Zone-IA, Intelligent-Tiering, Glacier Instant Retrieval). Rclone remoty nastavují třídu v konfiguraci rclone.
- **Vzdálený primární místo místního:** vlastní cesta sekce může být sama jedním z backendů výše, bez místní kopie a bez kroku replikace. Viz [Vzdálené primární repozitáře](offsite-recovery.md#remote-primary-repositories) pro přepínač **Místní / Vzdálené** a jeho nastavení šířky pásma, append-only a rozpočtu růstu.

## Anomálie {#anomalies}

Zjišťování anomálií se nastavuje v kartě **Anomálie** v **Nastavení, Ověření**. Každý ovladač se uloží, jakmile ho změníš, a tři pod přepínačem jsou skryté, dokud je zjišťování vypnuté.

| Nastavení | Výchozí | Co dělá |
|---|---|---|
| **Rozpoznávat anomálie** | Zapnuto | Porovnává každou zálohu s vlastní historií položky. Po vypnutí se nic nového nekontroluje a položka **Anomálie** zmizí z postranního panelu; karta stále odkazuje na dřívější zjištění. |
| **Citlivost** | Vyvážená | Přísná hlásí i menší změny, Shovívavá jen velké. |
| **Posílat oznámení pro** | Jen kritické nálezy | Nejnižší závažnost, která pošle zprávu kanály nastavenými v Oznámení. Opakovaně selhané zálohy a výpisy a selhané plánované kontroly obnovy už posílají vlastní zprávu a neposílají se dvakrát. |
| **Ponechat staré zálohy, když se zdroj prudce zmenší nebo je přepsán** | Zapnuto | Dokud má položka otevřené zjištění pro téměř prázdný zdroj, prudké zmenšení nebo znovu uložená většina dat, uchovávání a čištění nechají staré zálohy této položky být. Potvrď zjištění nebo ho označ jako očekávané, aby šly pryč. |

Každá položka může mít vlastní citlivost a vlastní minimum pro oznámení. Nastav je na záložce **Ověření** položky, nebo na stránce **Anomálie** pod **Sledování**. U položky ZFS platí pro každou datovou sadu jejího stromu.

## Přenositelná nastavení (export a import) {#portable-settings-export-and-import}

Karta **Export / import nastavení** v **Nastavení, Systém** zapíše celou konfiguraci BombVaultu (nastavení sekcí, cíle, plány, uchovávání, oznámení) do přenosného souboru JSON, který můžeš importovat na jiné instanci, takže přesun na nový stroj nebo klonování nastavení neznamená zadávat všechno ručně. Import ukáže náhled a požádá o potvrzení a nikdy se nedotkne dat záloh ani historie.

!!! warning "Export může obsahovat přihlašovací údaje"
    Ty zvolíš, zda do souboru zahrnout přihlašovací údaje mimo lokalitu, oznámení a brokeru MQTT. Se zahrnutými přihlašovacími údaji je export stejně citlivý jako tvoje sada pro obnovu, takže jej ulož na bezpečné místo. Bez nich soubor obsahuje jen netajná nastavení.
