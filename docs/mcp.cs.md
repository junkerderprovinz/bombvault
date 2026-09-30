# Server MCP

BombVault má vestavěný server pro Model Context Protocol (MCP), protokol, kterým AI asistenti jako Claude Code a Claude Desktop sahají po vnějších nástrojích. Přes něj může asistent číst, jak jsou na tom tvoje zálohy, a pokud to povolíš, spustit zálohu nebo zrušit tu, kterou sám spustil. Je vypnutý, dokud nevytvoříš klíč nebo nezapneš [přihlášení přes OAuth](#oauth): do té doby koncový bod `/mcp` odpovídá na všechno `404`.

## Co asistent smí a co ne {#tools}

| Nástroj | Co dělá | Druh |
|---|---|---|
| `get_health` | Verze, název instance, zda běží záloha a co tento klíč smí | čtení |
| `get_status` | Stav ochrany podle domén: poslední úspěšná záloha, očekávaný interval, ověření a kontroly off-site, další naplánované běhy, u kontejnerů nejnovější test spuštění | čtení |
| `get_coverage` | Co BombVault chrání a co ne, u každé položky s důvodem | čtení |
| `list_items` | Každý chráněný kontejner, VM a sada složek, flash disk a konfigurace aplikace, s plánem, tím, co záloha zastaví, poslední zálohou a její délkou; databázové kontejnery uvádějí i poslední dump; uvádí i datasety ZFS s výsledkem jejich poslední kontroly; každá položka nese svou poslední kontrolu obnovy, kontejner navíc svůj poslední test spuštění nebo důvod, proč ho nelze otestovat; kontejner znovu vytvořený s jiným nastavením od poslední zálohy uvádí, co se změnilo | čtení |
| `list_runs` | Historie běhů od nejnovějších, filtrovatelná podle domény, položky, stavu, druhu a času; pomalá záloha, kterou brzdila jedna věc, ji jmenuje | čtení |
| `list_restore_points` | Body obnovy jedné položky z jejího primárního repozitáře a u kontejneru i jeho databázové dumpy; dataset ZFS má jeden bod obnovy na zálohu se snapshotem každého datasetu pod ním | čtení |
| `get_activity` | Co právě běží, s fází a procenty | čtení |
| `get_storage_stats` | Historie velikosti primárního repozitáře domény a její týdenní růst, k tomu obsazené, volné a celkové místo na disku nebo vzdáleném úložišti každého jejího repozitáře | čtení |
| `get_size_breakdown` | Které složky a soubory zabírají místo v nejnovější záloze kontejneru, VM nebo sady složek a kolik z nich přidala poslední záloha | čtení |
| `list_anomalies` | Anomálie, kterých si BombVault všiml v zálohách, lze filtrovat podle stavu, závažnosti a domény, se souhrnem toho, co je otevřené | čtení |
| `get_anomaly` | Jedno z těchto zjištění s poznámkou, která zůstala při jeho potvrzení | čtení |
| `start_backup` | Hned zazálohuje jednu položku | spuštění |
| `start_domain_backup` | Zazálohuje každou chráněnou položku jedné domény | spuštění |
| `start_backup_everything` | Spustí průchod Backup Everything | spuštění |
| `cancel_backup` | Zruší běžící zálohu, kterou spustil tento klíč | zrušení |

Ve webovém rozhraní zůstává: obnova jakéhokoli druhu (včetně stažení, uložení nebo importu databázového dumpu), mazání záloh, prune, unlock, kontroly a cvičení, replikace off-site, nastavení, přihlašovací údaje a klíče MCP a také zrušení zálohy, kterou spustil plán, webové rozhraní nebo jiný klíč. Totéž platí pro potvrzení anomálie nebo její označení jako očekávané, což se dělá na stránce **Anomálie**. Důvod: odpovědi nástrojů obsahují názvy a chybové zprávy z vašeho serveru a kterákoli z nich může nést text napsaný tak, aby asistenta ovládl. Asistent, který na takový text naletí, může nanejvýš spustit zálohu v mezích uvedených níže nebo zrušit zálohu, kterou sám spustil.

Pokud je primární repozitář položky vzdálený (S3, REST, SFTP, rclone), `list_restore_points` se k němu připojí a volání může chvíli trvat. Kopie off-site přes MCP vypsat nelze. Na co se kontroly anomálií dívají, popisuje stránka [Funkce](features.md), a jak položka ZFS drží jeden snímek pro každou datovou sadu, stránka [Datové sady ZFS](zfs-datasets.md#contents).

## Co spuštěná záloha dělá {#starting-backups}

Záloha spuštěná asistentem je stejná záloha, jakou spouští webové rozhraní. Běžící kontejner se zastaví, dokud jeho záloha neskončí, spolu s kontejnery nastavenými k zastavení s ním. VM s metodou "graceful" se vypne a znovu spustí. Dataset ZFS zastaví kontejnery, které jsou pro něj nastavené, po dobu pořizování svého snapshotu. Sady složek, flash disk a konfigurace běží dál. Poté BombVault použije zásady uchovávání a případně zkopíruje data do repozitáře off-site. `list_items` asistentovi řekne, co položka zastaví a jak dlouho trvala její poslední záloha, a popisy nástrojů ho žádají, aby vám to řekl dřív, než cokoli spustí.

Protože záloha zastavuje služby a vytlačuje staré body obnovy, jsou spuštění přes MCP omezená:

- 12 spuštěných záloh za hodinu na klíč.
- 15 minut mezi dvěma spuštěními přes MCP u téže položky, téže domény nebo Backup Everything.
- Nejvýše 4 spuštění téže položky přes MCP za 24 hodin.
- **Ochrana uchovávání.** Když doména uchovává pevný počet bodů obnovy (jen "ponechat posledních N", bez denního, týdenního či měsíčního pravidla, lokálně nebo v cíli off-site), každá nová záloha vytlačí tu nejstarší. BombVault pak odmítne spuštění položky přes MCP, pokud jejích nejnovějších N-1 úspěšných záloh spustilo MCP. V uchovávané sadě tak vždy zůstane alespoň jeden bod obnovy, který vytvořil plán nebo vy. Při nastavení "ponechat poslední 1" nemůže asistent tuto položku zálohovat vůbec. Další naplánovaná záloha zase udělá místo. Samotné roční pravidlo se počítá jako "ponechat poslední 1", protože pro aktuální rok ponechá jen jeden bod obnovení.

Spuštění domény nebo Backup Everything vynechá položky, které nějaký limit zadrží, a vyjmenuje je v odpovědi. Webové rozhraní ani plán se žádného z těchto limitů netýkají. Hodinový rozpočet je jen v paměti, takže restart BombVaultu ho vynuluje.

## Zapnutí {#switch-on}

1. Otevři **Nastavení, Integrace, Server MCP** a klikni na tlačítko svého klienta. Klient, který v seznamu není, se připojí přes **Jiný klient**.
2. V části **Klíč** nech **Nový klíč** a navržený název, tedy název klienta, nebo napiš takový, který říká, kde se klíč používá, například „Claude Code na notebooku“. Jeden klíč na klienta ti dovolí jeden odvolat, aniž bys sahal na ostatní. **Existující klíč** dá klientovi klíč, který jsi vytvořil dřív.
3. Zapni **Povolit spouštění záloh** u klíče, který má moci spouštět zálohy; bez toho může jen číst. Později to můžeš změnit na dlaždici klíče a změna platí od dalšího požadavku asistenta, bez nového připojení.
4. Klikni na **Vytvořit klíč**. Klíč se zobrazí jen jednou. BombVault si nechává jen jeho otisk a znovu ho ukázat nedokáže, takže si ho hned zkopíruj. Když zavřeš okno dřív, než klient klíč použije, karta ho dál ukazuje, dokud nepotvrdíš, že sis ho zkopíroval.

Bez přihlašovacího hesla je samotné webové rozhraní otevřené všem ve vaší síti a kdo ho otevře, může také vytvořit klíč. Karta na to upozorňuje. Pokud BombVault otevřete pod názvem, který vypadá veřejně (například `bombvault.example.com` za reverzní proxy), a přihlašovací heslo nastavené není, nelze z této adresy klíče vytvářet ani nahrazovat, aby žádná webová stránka na internetu nemohla přimět váš prohlížeč, aby nějaký vytvořil. Nastavte přihlašovací heslo, nebo otevřete BombVault přes jeho IP adresu či místní název jako `tower` nebo `tower.local`.

## Vaše klíče a jejich protokol {#keys}

Každý klíč má na kartě vlastní dlaždici. Ukazuje název klíče, zda smí spouštět zálohy, nebo jen čte, poslední čtyři znaky klíče, kdy byl vytvořen nebo naposledy nahrazen, kdy ho klient naposledy použil a kolik volání dnes udělal. Na dlaždici klíč přejmenujete, změníte jeho oprávnění, nahradíte ho nebo zneplatníte. Zneplatněný klíč se přesune do seznamu zneplatněných klíčů, kde ho můžete natrvalo smazat, jakmile ho už žádný běh v historii neuvádí.

Vedle názvu ukazuje dlaždice logo klienta, pro kterého byl klíč vytvořen. Klíč vytvořený přes **Jiný klient**, nebo dřív, než karta klienty vypisovala, ukazuje místo toho klíč.

**Protokol** na dlaždici otevře, co tento klíč dělal. Nahoře jsou zálohy, které spustil, každá se svým stavem a odkazem na daný běh v protokolu aktivit na přehledu. Pod nimi jsou jeho volání, od nejnovějšího, s nástrojem a výsledkem. Odmítnutí uvádí důvod: klíč smí jen číst, ochrana uchovávání zálohu zadržela, už běžela jiná záloha, položka byla přes MCP zálohována před několika minutami, nebo klíč poslal příliš mnoho požadavků. Zrušení odkazuje na běh, o který šlo.

BombVault uchovává záznamy každého klíče nejvýše 30 dní: nejnovějších 500 úspěšných spuštění a zrušení a vedle nich nejnovějších 200 ostatních volání (čtení, odmítnutí a chyby), takže asistent, který se opakovaně ptá na běžící zálohu nebo znovu zkouší odmítnuté volání, nemůže z protokolu vytlačit její spuštění. U každého volání ukládá nástroj, výsledek a u zrušení daný běh. Nikdy neukládá, co asistent poslal, ani klíč či jeho otisk. Diagnostický balíček záznamy jen počítá a export nastavení je vynechává.

## Připojení klienta {#clients}

Každý klient má na kartě tlačítko, ve skupině **Na tomto počítači** nebo **V cloudu**. Tlačítko otevře okno ve třech krocích: klíč; konfigurace pro daného klienta s adresou, na které jsi kartu otevřel, tlačítkem pro kopírování, místem, kde konfigurace leží, a u vlastního certifikátu BombVaultu i tím, co klient potřebuje, aby mu důvěřoval; a čekání na první volání klienta. Okno sleduje poslední použití klíče a zezelená, jakmile to volání přijde.

Okno drží klíč mimo každý příkazový řádek. Kde klient umí klíč číst z proměnné prostředí (`BOMBVAULT_MCP_KEY`), ze skrytého dotazu nebo z vlastního souboru, konfigurace ho jen jmenuje. Kde klient takovou možnost nemá, leží klíč v jeho konfiguračním souboru nebo nastavení a okno to řekne. Kde dokumentace klienta neříká, jak zachází s neznámým certifikátem, popíše okno tento krok jako to, co udělat, když klient certifikát BombVaultu odmítne.

| Klient | Nastavení | Odkud klíč pochází |
|---|---|---|
| AnythingLLM | konfigurační soubor | konfigurační soubor |
| Antigravity | konfigurační soubor | proměnná prostředí |
| Claude Code | příkaz | soubor s klíčem |
| Claude Desktop | konfigurační soubor | soubor s klíčem |
| Cline | konfigurační soubor | konfigurační soubor |
| Codex CLI | konfigurační soubor | proměnná prostředí |
| Continue | konfigurační soubor | `~/.continue/.env` |
| Copilot CLI | konfigurační soubor | konfigurační soubor |
| Cursor | konfigurační soubor | proměnná prostředí |
| Gemini CLI | konfigurační soubor | proměnná prostředí |
| GitHub Copilot (VS Code) | konfigurační soubor | skrytý dotaz |
| Goose | konfigurační soubor | proměnná prostředí |
| Jan | formulář v aplikaci | nastavení aplikace |
| JetBrains (AI Assistant, Junie) | konfigurační soubor | konfigurační soubor |
| Kimi Code | konfigurační soubor | konfigurační soubor |
| LM Studio | konfigurační soubor | konfigurační soubor |
| Mistral Vibe | konfigurační soubor | proměnná prostředí |
| Msty | formulář v aplikaci | nastavení aplikace |
| n8n | formulář v aplikaci | přihlašovací údaje v n8n |
| Open WebUI | formulář v aplikaci | nastavení aplikace |
| opencode | konfigurační soubor | proměnná prostředí |
| Perplexity (Mac) | formulář v aplikaci | soubor s klíčem |
| Qwen Code | konfigurační soubor | proměnná prostředí |
| Roo Code | konfigurační soubor | proměnná prostředí |
| Visual Studio | konfigurační soubor | konfigurační soubor |
| Warp | konfigurační soubor | konfigurační soubor |
| Windsurf | konfigurační soubor | proměnná prostředí |
| Zed | konfigurační soubor | konfigurační soubor |
| Grok | formulář, v cloudu | servery poskytovatele |
| Le Chat | formulář, v cloudu | servery poskytovatele |
| ChatGPT | přihlášení přes OAuth, v cloudu | přístupový token, viz [níže](#oauth) |
| Claude (claude.ai) | přihlášení přes OAuth, v cloudu | přístupový token, viz [níže](#oauth) |

Oddíly níže podrobněji vysvětlují nastavení Claude Code a Claude Desktop a uvádějí, co potřebuje každý jiný klient.

### Claude Code {#claude-code}

Claude Code se k BombVaultu dostane přes `mcp-remote`, který na daném počítači potřebuje Node.js. Nejdřív uložte klíč do samostatného textového souboru, na jeden řádek:

```text
X-API-Key: <your key>
```

Pak příkaz z karty jednou spusťte v terminálu, s doplněnou cestou k tomuto souboru. Za certifikátem, kterému váš počítač důvěřuje, vypadá takto:

```bash
claude mcp add bombvault --scope user -- npx -y mcp-remote@latest https://bombvault.example.com/mcp --header-file "<path of the file with your key>"
```

S vlastním certifikátem BombVaultu (viz [TLS a certifikáty](#tls)) příkaz navíc nasměruje Node.js na stažený certifikát:

```bash
claude mcp add bombvault --scope user -e "NODE_EXTRA_CA_CERTS=<path of the downloaded bombvault-cert.pem>" -- npx -y mcp-remote@latest https://192.168.1.10:3443/mcp --header-file "<path of the file with your key>"
```

Připojení ověříte příkazem `/mcp` v Claude Code. `--scope user` zpřístupní BombVault ve všech vašich projektech. Claude Code si pamatuje jen cestu k souboru s klíčem, takže se klíč neobjeví v příkazu ani v historii shellu, ani v seznamu procesů. Soubor mějte tam, kde ho můžete číst jen vy, a mimo každou složku, kterou commitujete. Díky `@latest` si `npx` stáhne aktuální `mcp-remote`; jinak by se použil starší, globálně nainstalovaný, který `--header-file` nezná.

Nepište `${BOMBVAULT_MCP_KEY}` do argumentů `mcp-remote` pro Claude Code. Claude Code takový odkaz dosadí ze svého vlastního prostředí ještě před spuštěním `mcp-remote`, takže klíč skončí v příkazovém řádku tohoto procesu, kde ho mohou číst jiné programy a uživatelé počítače.

Bez Node.js se Claude Code umí připojit sám, ale jen za certifikátem, kterému váš počítač důvěřuje. Do složky projektu dejte soubor `.mcp.json`:

```json
{
  "mcpServers": {
    "bombvault": {
      "type": "http",
      "url": "https://bombvault.example.com/mcp",
      "headers": {
        "Authorization": "Bearer ${BOMBVAULT_MCP_KEY}"
      }
    }
  }
}
```

Proměnnou `BOMBVAULT_MCP_KEY` nastavte tam, kde se Claude Code spouští, například pod `"env"` v `~/.claude/settings.json` nebo v profilu shellu, a to v textovém editoru, ne napsáním do příkazového řádku. Tady je odkaz bezpečný, protože Claude Code nespouští žádný druhý proces, který by ho nesl. S vlastním certifikátem BombVaultu to takto nefunguje: připojení, které Claude Code navazuje sám, ho odmítne i s nastavenou `NODE_EXTRA_CA_CERTS`. Nikdy necommitujte `.mcp.json`, ve kterém je klíč vypsaný.

### Claude Desktop {#claude-desktop}

Claude Desktop se k BombVaultu dostane přes `mcp-remote`, který na daném počítači potřebuje Node.js. Nejdřív uložte klíč do samostatného textového souboru jako jediný řádek, jak je popsáno u [Claude Code](#claude-code). Konfigurační soubor otevřete v Claude Desktop přes **Settings, Developer, Edit Config**. Ve Windows je v `%APPDATA%\Claude\claude_desktop_config.json`, v macOS v `~/Library/Application Support/Claude/claude_desktop_config.json`. Položku z karty přidejte do `"mcpServers"` vedle serverů, které tam už jsou, a Claude Desktop restartujte:

```json
{
  "mcpServers": {
    "bombvault": {
      "command": "npx",
      "args": ["-y", "mcp-remote@latest", "https://192.168.1.10:3443/mcp", "--header-file", "<path of the file with your key>"],
      "env": {
        "NODE_EXTRA_CA_CERTS": "<path of the downloaded bombvault-cert.pem>"
      }
    }
  }
}
```

- `NODE_EXTRA_CA_CERTS` je tam jen kvůli vlastnímu certifikátu BombVaultu. Za certifikátem, kterému váš počítač už důvěřuje, ho vynechte.
- `--allow-http` se přidává jen u prosté adresy `http://`.
- Ve Windows pište cesty s obyčejnými lomítky, například `C:/Users/sam/bombvault-key.txt`, protože samotné zpětné lomítko není platný JSON. Cesta k souboru s klíčem nesmí obsahovat mezery: Claude Desktop ve Windows předá cestu s mezerou programu `npx` ve dvou kusech.
- Konfigurace uvádí jen soubor s klíčem, takže se klíč neobjeví ani v ní, ani v seznamu procesů. Soubor mějte tam, kde ho můžete číst jen vy.

### Klienti v cloudu {#cloud-clients}

ChatGPT, Claude na claude.ai, Grok a Le Chat volají BombVault ze serverů svého poskytovatele, takže BombVault musí být dostupný z internetu s veřejně důvěryhodným certifikátem, například za reverzní proxy; Le Chat odmítá certifikáty podepsané sebou samým. Přihlášení na proxy smí chránit webové rozhraní, ale `/mcp` musí k BombVault projít bez něj: tyto služby se k proxy přihlásit neumějí a BombVault kontroluje jejich klíč nebo token sám. Grok a Le Chat posílají pevný klíč a jejich tlačítka je nastaví jako ostatní. ChatGPT a ve většině organizací i Claude na claude.ai se připojují jen přihlášením přes OAuth, popsaným dál.

### Přihlášení přes OAuth {#oauth}

Pro klienta, který neumí převzít klíč, je BombVault jeho vlastním autorizačním serverem OAuth. Klient se sám zaregistruje, pošle tě na stránku BombVault a tam se přihlásíš svým přihlašovacím heslem (a druhým faktorem, pokud ho máš nastavený) a povolíš ho. Klient pak dostane token, který platí jen pro koncový bod MCP tohoto BombVault, a sám si ho obnovuje.

1. Nastav přihlašovací heslo v **Nastavení, Zabezpečení**. Bez něj BombVault žádné přihlášení nenabízí, protože by nebylo koho požádat o souhlas.
2. Zpřístupni BombVault z internetu přes https s certifikátem, kterému prohlížeče věří, obvykle přes reverzní proxy. Klient volá `/mcp`, `/oauth/` a `/.well-known/` ze svých vlastních serverů, takže proxy s vlastním přihlášením musí tyto tři cesty pustit až k BombVault. Stránka se souhlasem na `/oauth/authorize` se otevírá ve tvém vlastním prohlížeči a smí zůstat za přihlášením proxy. Uveď proxy také v `TRUSTED_PROXY` (viz [Konfigurace](configuration.md)). BombVault omezuje registrace klientů podle adresy a bez toho se zdá, že každý klient přichází z proxy.
3. Na kartě MCP zapni **Přihlášení přes OAuth** a zadej **Veřejná adresa**: adresu https bez cesty, například `https://backup.example.com`. Každý token je na tuto adresu vázaný, takže po změně se musí každý klient přihlásit znovu.
4. Klikni na tlačítko ChatGPT nebo Claude. Dialog ukáže **URL konektoru**, tedy veřejnou adresu s `/mcp` na konci, a kam v daném klientovi patří. V ChatGPT zapneš vývojářský režim v **Nastavení, Aplikace a konektory, Pokročilé nastavení**, zvolíš **Vytvořit**, vložíš URL konektoru jako URL MCP serveru a jako ověřování zvolíš OAuth. Na claude.ai otevřeš **Nastavení, Konektory, Přidat vlastní konektor**, vložíš URL konektoru, ID klienta a tajný klíč OAuth necháš prázdné a zvolíš **Připojit**.
5. Klient otevře stránku se souhlasem. Ukazuje, kdo žádá, kam tě tvoje odpověď vrátí, a přepínač **Povolit spouštění záloh**, který je na začátku vypnutý. Zvol **Povolit** nebo **Odmítnout**.

Každý přihlášený klient dostane dlaždici vedle klíčů, se svou značkou, svým protokolem, **Zneplatnit** a **Povolit spouštění záloh**, a stejné limity jako klíč. Zneplatnění platí okamžitě. Když se stejný klient přihlásí znovu, jeho nové povolení nahradí to staré, a povolení, které 30 dní nikdo nepoužil, vyprší. Najednou může být přihlášeno až 10 klientů, navíc k 10 klíčům.

Stránka se souhlasem přijme požadavek jen od registrovaného klienta, který uvede přesně jednu ze svých registrovaných návratových adres: https, nebo adresu loopback s libovolným portem pro klienta na tvém vlastním počítači. Přijímá se jen tok autorizačního kódu s PKCE (S256) a tvoje odpověď je vázaná na tvou relaci, takže žádný jiný web ji za tebe odeslat nemůže. Přístupové tokeny platí hodinu. Obnovovací token se při každém použití nahradí, a pokud se některý potom objeví znovu, BombVault povolení zruší, protože kopii má někdo jiný. Klient, který do 30 sekund zopakuje svou poslední obnovu, protože k němu odpověď nedorazila, dostane místo toho nové tokeny. BombVault nestahuje metadata klientů z internetu, takže se klienti registrují přes dynamickou registraci klientů.

### Ostatní klienti {#other-clients}

Funguje každý klient, který umí Streamable HTTP:

- URL: adresa webového rozhraní plus `/mcp`, například `https://192.168.1.10:3443/mcp`.
- Klíč v `Authorization: Bearer <key>` nebo v `X-API-Key: <key>`. Pokud přijdou obě hlavičky, musí nést stejný klíč.
- `POST` s `Content-Type: application/json` a `Accept: application/json, text/event-stream`.
- Jedna zpráva JSON-RPC na požadavek; dávky (batch) se odmítají.
- Verze protokolu 2026-07-28, 2025-11-25, 2025-06-18 a 2025-03-26.

## TLS a certifikáty {#tls}

BombVault poskytuje HTTPS s certifikátem, který si vydal sám, a ten zpočátku uvádí jen `localhost`, `127.0.0.1` a `::1`. Claude Code a `mcp-remote` ho na adrese v místní síti odmítnou. Cesty kolem toho, v pořadí, které vyhovuje většině instalací Unraid:

1. **Přidat adresu v kartě MCP.** Když kartu otevřete přes HTTPS na adrese, kterou certifikát neuvádí, karta to řekne a nabídne **Přidat tuto adresu do certifikátu**. BombVault pak certifikát vydá znovu i s touto adresou (prohlížeč jednou znovu varuje, stejně jako poprvé). Potom klikněte na **Stáhnout certifikát**; úryvky nastaví `NODE_EXTRA_CA_CERTS` na stažený soubor, takže klient důvěřuje právě tomuto certifikátu. Znamená to také, že každý klient nastavený s dříve staženým souborem se přestane připojovat, jakmile je certifikát vydán znovu, na tomto počítači i na všech ostatních, dokud nedostane nový soubor.
2. **Reverzní proxy s důvěryhodným certifikátem** (Nginx Proxy Manager, SWAG, Caddy, Traefik). Klient pak vidí certifikát proxy a nic dalšího nepotřebuje a karta na certifikát BombVaultu neupozorňuje.
3. **Tailscale.** `tailscale serve` před kontejnerem nebo integrace Tailscale v Unraidu vám dá název `ts.net` s důvěryhodným certifikátem.
4. **`HTTP_ONLY=true`**, jen za proxy, která ukončuje TLS, nebo v síti, které plně důvěřujete. Přepne celé webové rozhraní na prosté HTTP, vyžaduje změnu nastavení kontejneru a posílá klíč nešifrovaně.

Nikdy nenastavujte `NODE_TLS_REJECT_UNAUTHORIZED=0`. Vypne to kontrolu certifikátů pro všechno, s čím daný proces Node.js komunikuje.

Reverzní proxy musí hlavičku `Authorization` (nebo `X-API-Key`) předat dál, což proxy dělají, pokud jim neřeknete jinak, a nesmí `/mcp` bufferovat ani přepisovat. Blok location pro Nginx nebo Nginx Proxy Manager, který zároveň ověřuje certifikát BombVaultu:

```nginx
location /mcp {
    proxy_pass https://192.168.1.10:3443;
    proxy_ssl_verify on;
    proxy_ssl_trusted_certificate /data/bombvault-cert.pem;
    proxy_ssl_name localhost;
    proxy_http_version 1.1;
    proxy_buffering off;
    proxy_set_header Host $host;
}
```

Za proxy nese každý požadavek adresu proxy. Pět špatných klíčů od jediného špatně nastaveného klienta pak na minutu zablokuje všechny klienty MCP za touto proxy. Uveďte proxy v `TRUSTED_PROXY` (viz [Konfigurace](configuration.md)), aby se počítalo po klientech.

## Bezpečnostní model {#security}

- Bez aktivního klíče a s vypnutým přihlášením přes OAuth odpovídá `/mcp` `404`.
- Přihlášení přes OAuth se nabízí jen, dokud je nastavené přihlašovací heslo. Tokeny, kódy a tajné klíče klientů se ukládají jen jako otisk a token platí jen pro adresu, pro kterou byl vydán.
- Klient se z jedné adresy může zaregistrovat nejvýš 10krát za hodinu a BombVault drží nejvýš 100 registrovaných klientů, se kterými se nikdo nepřihlásil, každého jeden den. Špatné kódy a obnovovací tokeny se počítají do stejného blokování jako špatné klíče.
- Povolení se při obnovení zálohy konfigurace nebo změně `APP_KEY` chovají jako klíče: po obnovení se musí každý klient přihlásit znovu.
- Žádná adresa nemá výjimku. Požadavky z `localhost`, z hostitele Unraid, z reverzní proxy nebo z `tailscale serve` potřebují klíč jako všechny ostatní, i když webové rozhraní nemá přihlašovací heslo.
- Klíče se ukládají jen jako otisky, zobrazí se jednou a lze je přejmenovat, nahradit a odvolat. Až 10 aktivních klíčů, každý s vlastním přepínačem **Povolit spouštění záloh**.
- Každé vytvoření, nahrazení, změna oprávnění a odvolání odešle oznámení vašimi kanály oznámení i s adresou, odkud přišlo, pokud oznámení nemáte vypnutá.
- 5 špatných klíčů za minutu na adresu, pak `429`. 120 požadavků za minutu a 12 spuštěných záloh za hodinu na klíč, k tomu výše popsaná čekací doba a ochrana uchovávání.
- Požadavky ze stránky prohlížeče s jiným originem se odmítají.
- Dokud není nastavené přihlašovací heslo, nelze vytvářet klíče z názvu hostitele, který vypadá veřejně.
- Každá záloha, kterou spustí asistent, a z ní plynoucí běhy prune a off-site jsou označené "přes MCP" s názvem klíče v protokolu aktivit, v panelu chyb a v oznámení o záloze.
- Každé volání nástroje se zapíše do logu kontejneru s id klíče a jeho posledními čtyřmi znaky (nikdy s názvem) a započítá se v `/metrics` (`bombvault_mcp_requests_total`, `bombvault_mcp_tool_calls_total`, `bombvault_mcp_active_keys`).
- Obnova zálohy konfigurace odvolá všechny klíče, protože obnovená databáze může obsahovat klíče, které jste odvolali až po jejím uložení. Potom vytvořte nové.
- Klíč přestane fungovat, když se změní `APP_KEY` (reinstalace nebo obnova do jiného kontejneru). Karta to pozná a klíč označí a **Nahradit klíč** mu dá znovu platné tajemství.
- Zacházej s klíčem jako s heslem. Klient, který neumí číst klíč z proměnné prostředí, z dotazu ani ze souboru s klíčem, ho má jako prostý text v konfiguraci nebo nastavení a jeho okno to řekne. Na počítači, kterému věříš méně, použij raději klíč jen pro čtení.

## Co opouští stroj {#privacy}

Vše, co asistent přečte, odchází k poskytovateli umělé inteligence za ním: názvy položek, plány, historie běhů s chybovými zprávami, id a časy bodů obnovy, názvy databázových strojů a velikosti dumpů, probíhající činnost, údaje o úložišti, pokrytí a stav. BombVault odstraní cesty hostitele, umístění repozitářů, názvy hostitelů, přihlašovací údaje, příkazy hooků a klíče dřív, než cokoli odejde.

## Řešení potíží {#troubleshooting}

| Co vidíte | Co to znamená |
|---|---|
| `404` | Žádný aktivní klíč a přihlášení přes OAuth je vypnuté, nebo špatná cesta jako `/api/mcp`. Koncový bod je `/mcp`. |
| `401` | Klíč chybí, je překlepnutý, odvolaný nebo nahrazený. Možná proxy zahazuje hlavičku `Authorization` (zkuste `X-API-Key`). Pokud karta označí klíč jako už neplatný, změnil se `APP_KEY`: klíč nahraďte. |
| `403` | Požadavek přišel ze stránky prohlížeče s jiným originem. Použijte desktopového klienta nebo klienta pro příkazový řádek. |
| `405` u GET | Normální. Koncový bod přijímá jen `POST`. |
| `400` "Accept must contain both 'application/json' and 'text/event-stream'" | Klient je na Streamable HTTP příliš starý. Aktualizujte ho. |
| `400` "batch requests are not accepted" | Klient posílá dávky JSON-RPC. Posílejte jednu zprávu na požadavek. |
| `429` | Příliš mnoho špatných klíčů z této adresy, nebo víc než 120 požadavků za minutu s jedním klíčem. Minutu počkejte a zkontrolujte, zda asistent neuvízl ve smyčce. |
| Chyby s "certificate", "self-signed" nebo "unable to verify" | Klient nedůvěřuje certifikátu BombVaultu. Viz [TLS a certifikáty](#tls). |
| `busy` | Doménu zabírá jiná záloha nebo úloha údržby. Zkuste to znovu, až skončí. |
| `cooldown` | Tato položka, tato doména nebo Backup Everything byla přes MCP spuštěna před méně než 15 minutami. |
| `retention_guard` | Další záloha přes MCP by v okně "ponechat posledních N" nechala jen body obnovy z MCP, nebo položka už za posledních 24 hodin dostala 4 zálohy přes MCP, včetně neúspěšných a zrušených. V prvním případě udělá místo další naplánovaná záloha, ve druhém je položka znovu volná 24 hodin po nejstarší z těchto záloh. Ve webovém rozhraní ji můžete spustit kdykoli. |
| `rate_limited` | Klíč vyčerpal svých 12 spuštění pro tuto hodinu. |
| `not_permitted` při spuštění | Klíč smí jen číst. Zapněte v kartě **Povolit spouštění záloh**; nové připojení není potřeba. U zrušení to znamená, že běh nespustil tento klíč. |
| `domain_off` | Tento druh zálohy je v nastavení vypnutý. |
| `not_found` | BombVault tuto položku nechrání. Nejdřív ji přidejte ve webovém rozhraní; MCP nikdy nevytváří konfiguraci. |
| Klient nenajde autorizační server | Přihlášení přes OAuth je vypnuté, není nastavené přihlašovací heslo, nebo proxy nepouští `/.well-known/` k BombVault. |
| Stránka se souhlasem hlásí neregistrovanou návratovou adresu | Klient poslal návratovou adresu, kterou nezaregistroval. Odeber konektor v klientovi a přidej ho znovu. |
| Přihlášený klient dostává `401` | Jeho povolení bylo zneplatněno, vypršelo po 30 dnech bez použití, nebo se změnila veřejná adresa. Klient se přihlásí znovu. |

Nenastavujte na kontejneru proměnnou prostředí `MCPGODEBUG`. Mění chování knihovny MCP a chybná hodnota zastaví BombVault při startu dřív, než zapíše jediný řádek logu.
