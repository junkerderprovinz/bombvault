# Server MCP

BombVault má vestavěný server pro Model Context Protocol (MCP), protokol, přes který asistenti s umělou inteligencí jako Claude Code a Claude Desktop sahají po vnějších nástrojích. Asistent si přes něj může přečíst, jak jsou na tom vaše zálohy, a pokud to dovolíte, spustit zálohu nebo zrušit zálohu, kterou sám spustil. Server je vypnutý, dokud nevytvoříte klíč: bez aktivního klíče odpovídá koncový bod `/mcp` na všechno `404`.

## Co asistent smí a co ne {#tools}

| Nástroj | Co dělá | Druh |
|---|---|---|
| `get_health` | Verze, název instance, zda běží záloha a co tento klíč smí | čtení |
| `get_status` | Stav ochrany podle domén: poslední úspěšná záloha, očekávaný interval, ověření a kontroly off-site, další naplánované běhy | čtení |
| `get_coverage` | Co BombVault chrání a co ne, u každé položky s důvodem | čtení |
| `list_items` | Každý chráněný kontejner, VM a sada složek, flash disk a konfigurace aplikace, s plánem, tím, co záloha zastaví, poslední zálohou a její délkou; databázové kontejnery uvádějí i poslední dump; uvádí i datasety ZFS s výsledkem jejich poslední kontroly | čtení |
| `list_runs` | Historie běhů od nejnovějších, filtrovatelná podle domény, položky, stavu, druhu a času | čtení |
| `list_restore_points` | Body obnovy jedné položky z jejího primárního repozitáře a u kontejneru i jeho databázové dumpy; dataset ZFS má jeden bod obnovy na zálohu se snapshotem každého datasetu pod ním | čtení |
| `get_activity` | Co právě běží, s fází a procenty | čtení |
| `get_storage_stats` | Historie velikosti primárního repozitáře domény a její týdenní růst | čtení |
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
- **Ochrana uchovávání.** Když doména uchovává pevný počet bodů obnovy (jen "ponechat posledních N", bez denního, týdenního či měsíčního pravidla, lokálně nebo v cíli off-site), každá nová záloha vytlačí tu nejstarší. BombVault pak odmítne spuštění položky přes MCP, pokud jejích nejnovějších N-1 úspěšných záloh spustilo MCP. V uchovávané sadě tak vždy zůstane alespoň jeden bod obnovy, který vytvořil plán nebo vy. Při nastavení "ponechat poslední 1" nemůže asistent tuto položku zálohovat vůbec. Další naplánovaná záloha zase udělá místo.

Spuštění domény nebo Backup Everything vynechá položky, které nějaký limit zadrží, a vyjmenuje je v odpovědi. Webové rozhraní ani plán se žádného z těchto limitů netýkají. Hodinový rozpočet je jen v paměti, takže restart BombVaultu ho vynuluje.

## Zapnutí {#switch-on}

1. Otevřete **Nastavení, Systém, Server MCP** a klikněte na **Nový klíč**.
2. Dejte klíči název, který říká, kde se používá, například "Claude Code na notebooku". S jedním klíčem na klienta můžete jeden odvolat, aniž byste sahali na ostatní.
3. Nechte zapnuté **Povolit spouštění záloh**, nebo ho vypněte u klíče, který má jen číst. Později to můžete změnit na dlaždici klíče a změna platí od příštího požadavku asistenta, bez nového připojení.
4. Klikněte na **Vytvořit klíč**. Klíč se zobrazí jednou. BombVault si z něj uchová jen otisk a znovu ho ukázat nemůže, proto ho hned zkopírujte, nebo si vezměte některý z úryvků pod ním, které pak obsahují skutečný klíč.

Bez přihlašovacího hesla je samotné webové rozhraní otevřené všem ve vaší síti a kdo ho otevře, může také vytvořit klíč. Karta na to upozorňuje. Pokud BombVault otevřete pod názvem, který vypadá veřejně (například `bombvault.example.com` za reverzní proxy), a přihlašovací heslo nastavené není, nelze z této adresy klíče vytvářet ani nahrazovat, aby žádná webová stránka na internetu nemohla přimět váš prohlížeč, aby nějaký vytvořil. Nastavte přihlašovací heslo, nebo otevřete BombVault přes jeho IP adresu či místní název jako `tower` nebo `tower.local`.

## Vaše klíče a jejich protokol {#keys}

Každý klíč má na kartě vlastní dlaždici. Ukazuje název klíče, zda smí spouštět zálohy, nebo jen čte, poslední čtyři znaky klíče, kdy byl vytvořen nebo naposledy nahrazen, kdy ho klient naposledy použil a kolik volání dnes udělal. Na dlaždici klíč přejmenujete, změníte jeho oprávnění, nahradíte ho nebo zneplatníte. Zneplatněný klíč se přesune do seznamu zneplatněných klíčů, kde ho můžete natrvalo smazat, jakmile ho už žádný běh v historii neuvádí.

**Protokol** na dlaždici otevře, co tento klíč dělal. Nahoře jsou zálohy, které spustil, každá se svým stavem a odkazem na daný běh v protokolu aktivit na přehledu. Pod nimi jsou jeho volání, od nejnovějšího, s nástrojem a výsledkem. Odmítnutí uvádí důvod: klíč smí jen číst, ochrana uchovávání zálohu zadržela, už běžela jiná záloha, položka byla přes MCP zálohována před několika minutami, nebo klíč poslal příliš mnoho požadavků. Zrušení odkazuje na běh, o který šlo.

BombVault uchovává záznamy každého klíče nejvýše 30 dní: nejnovějších 500 spuštění, zrušení, odmítnutí a chyb a vedle nich nejnovějších 200 úspěšných čtení, takže asistent, který se opakovaně ptá na běžící zálohu, nemůže z protokolu vytlačit její spuštění. U každého volání ukládá nástroj, výsledek a u zrušení daný běh. Nikdy neukládá, co asistent poslal, ani klíč či jeho otisk. Diagnostický balíček záznamy jen počítá a export nastavení je vynechává.

## Připojení klienta {#clients}

Karta zobrazuje hotové úryvky pro adresu, na které jste ji otevřeli: vyberte klienta a zkopírujte úryvek. Zbytek oddílu vysvětluje, co úryvky dělají, a uvádí podoby, které karta nezobrazuje.

### Claude Code {#claude-code}

Příkaz z karty spusťte jednou v terminálu. S certifikátem, kterému váš počítač důvěřuje, vypadá takto:

```bash
claude mcp add --transport http bombvault --scope user https://bombvault.example.com/mcp --header "Authorization: Bearer <your key>"
```

Připojení ověříte příkazem `/mcp` v Claude Code. `--scope user` uloží klíč do vaší uživatelské konfigurace, ne do souboru projektu.

Příkaz obsahuje klíč a váš shell si ho může uložit do historie. Tomu se vyhnete souborem `.mcp.json` ve složce projektu a klíčem v proměnné prostředí. Claude Code při čtení souboru dosadí `${BOMBVAULT_MCP_KEY}`:

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

Proměnnou `BOMBVAULT_MCP_KEY` nastavte tam, kde se Claude Code spouští, například v profilu shellu, a to v textovém editoru, ne napsáním do příkazového řádku. Nikdy necommitujte `.mcp.json`, ve kterém je klíč vypsaný.

S vlastním certifikátem BombVaultu (viz [TLS a certifikáty](#tls)) spustí příkaz z karty místo toho `mcp-remote` a nasměruje Node.js na stažený certifikát:

```bash
claude mcp add bombvault --scope user -e "NODE_EXTRA_CA_CERTS=<path of the downloaded bombvault-cert.pem>" -e "BOMBVAULT_MCP_KEY=<your key>" -- npx -y mcp-remote https://192.168.1.10:3443/mcp --header 'X-API-Key:${BOMBVAULT_MCP_KEY}'
```

Jednoduché uvozovky brání shellu v rozvinutí proměnné; to udělá `mcp-remote` sám. Stejná podoba funguje v `.mcp.json`: použijte položku pro Claude Desktop níže a vynechte `BOMBVAULT_MCP_KEY` z jejího `env`, klíč se pak vezme z vašeho prostředí.

### Claude Desktop {#claude-desktop}

Claude Desktop se k BombVaultu dostane přes `mcp-remote`, který na daném počítači potřebuje Node.js. Konfigurační soubor otevřete v Claude Desktop přes **Settings, Developer, Edit Config**. Ve Windows je v `%APPDATA%\Claude\claude_desktop_config.json`, v macOS v `~/Library/Application Support/Claude/claude_desktop_config.json`. Položku z karty přidejte do `"mcpServers"` vedle serverů, které tam už jsou, a Claude Desktop restartujte:

```json
{
  "mcpServers": {
    "bombvault": {
      "command": "npx",
      "args": ["-y", "mcp-remote", "https://192.168.1.10:3443/mcp", "--header", "X-API-Key:${BOMBVAULT_MCP_KEY}"],
      "env": {
        "BOMBVAULT_MCP_KEY": "<your key>",
        "NODE_EXTRA_CA_CERTS": "<path of the downloaded bombvault-cert.pem>"
      }
    }
  }
}
```

- `NODE_EXTRA_CA_CERTS` je tam jen kvůli vlastnímu certifikátu BombVaultu. Za certifikátem, kterému váš počítač už důvěřuje, ho vynechte.
- `--allow-http` se přidává jen u prosté adresy `http://`.
- Hlavička se píše `X-API-Key:${BOMBVAULT_MCP_KEY}`, bez mezery za dvojtečkou a s klíčem v `env`. Na některých systémech `mcp-remote` rozdělí hodnotu `--header` na první mezeře a klíč napsaný za mezerou by se ztratil.

### Vlastní konektory v nastavení Claude {#custom-connectors}

Konektory přidané v nastavení samotného Claude (na claude.ai a v seznamu konektorů Claude Desktop) zatím podporované nejsou. Tyto konektory se volají z cloudu společnosti Anthropic, takže potřebují veřejnou adresu HTTPS, a přihlašují se přes OAuth. Pevný klíč poslat neumějí a BombVault nabízí jen pevné klíče, žádné přihlášení OAuth. Vystavit kvůli nim BombVault do internetu by nepomohlo. Použijte Claude Code, nebo Claude Desktop přes `mcp-remote` jako výše.

### Ostatní klienti {#other-clients}

Funguje každý klient, který umí Streamable HTTP:

- URL: adresa webového rozhraní plus `/mcp`, například `https://192.168.1.10:3443/mcp`.
- Klíč v `Authorization: Bearer <key>` nebo v `X-API-Key: <key>`. Pokud přijdou obě hlavičky, musí nést stejný klíč.
- `POST` s `Content-Type: application/json` a `Accept: application/json, text/event-stream`.
- Jedna zpráva JSON-RPC na požadavek; dávky (batch) se odmítají.
- Verze protokolu 2026-07-28, 2025-11-25, 2025-06-18 a 2025-03-26.

## TLS a certifikáty {#tls}

BombVault poskytuje HTTPS s certifikátem, který si vydal sám, a ten zpočátku uvádí jen `localhost`, `127.0.0.1` a `::1`. Claude Code a `mcp-remote` ho na adrese v místní síti odmítnou. Cesty kolem toho, v pořadí, které vyhovuje většině instalací Unraid:

1. **Přidat adresu v kartě MCP.** Když kartu otevřete přes HTTPS na adrese, kterou certifikát neuvádí, karta to řekne a nabídne **Přidat tuto adresu do certifikátu**. BombVault pak certifikát vydá znovu i s touto adresou (prohlížeč jednou znovu varuje, stejně jako poprvé). Potom klikněte na **Stáhnout certifikát**; úryvky nastaví `NODE_EXTRA_CA_CERTS` na stažený soubor, takže klient důvěřuje právě tomuto certifikátu.
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

- Bez aktivního klíče odpovídá `/mcp` kódem `404`.
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
- Zacházejte s klíčem jako s heslem. Claude Code a Claude Desktop ho ukládají v otevřeném textu ve své konfiguraci. Na počítači, kterému důvěřujete méně, dejte přednost klíči, který smí jen číst.

## Co opouští stroj {#privacy}

Vše, co asistent přečte, odchází k poskytovateli umělé inteligence za ním: názvy položek, plány, historie běhů s chybovými zprávami, id a časy bodů obnovy, názvy databázových strojů a velikosti dumpů, probíhající činnost, údaje o úložišti, pokrytí a stav. BombVault odstraní cesty hostitele, umístění repozitářů, názvy hostitelů, přihlašovací údaje, příkazy hooků a klíče dřív, než cokoli odejde.

## Řešení potíží {#troubleshooting}

| Co vidíte | Co to znamená |
|---|---|
| `404` | Žádný aktivní klíč, nebo špatná cesta jako `/api/mcp`. Koncový bod je `/mcp`. |
| `401` | Klíč chybí, je překlepnutý, odvolaný nebo nahrazený. Možná proxy zahazuje hlavičku `Authorization` (zkuste `X-API-Key`). Pokud karta označí klíč jako už neplatný, změnil se `APP_KEY`: klíč nahraďte. |
| `403` | Požadavek přišel ze stránky prohlížeče s jiným originem. Použijte desktopového klienta nebo klienta pro příkazový řádek. |
| `405` u GET | Normální. Koncový bod přijímá jen `POST`. |
| `400` "Accept must contain both 'application/json' and 'text/event-stream'" | Klient je na Streamable HTTP příliš starý. Aktualizujte ho. |
| `400` "batch requests are not accepted" | Klient posílá dávky JSON-RPC. Posílejte jednu zprávu na požadavek. |
| `429` | Příliš mnoho špatných klíčů z této adresy, nebo víc než 120 požadavků za minutu s jedním klíčem. Minutu počkejte a zkontrolujte, zda asistent neuvízl ve smyčce. |
| Chyby s "certificate", "self-signed" nebo "unable to verify" | Klient nedůvěřuje certifikátu BombVaultu. Viz [TLS a certifikáty](#tls). |
| `busy` | Doménu zabírá jiná záloha nebo úloha údržby. Zkuste to znovu, až skončí. |
| `cooldown` | Tato položka, tato doména nebo Backup Everything byla přes MCP spuštěna před méně než 15 minutami. |
| `retention_guard` | Další záloha přes MCP by v okně "ponechat posledních N" nechala jen body obnovy z MCP. Místo udělá další naplánovaná záloha, nebo ji spusťte ve webovém rozhraní. |
| `rate_limited` | Klíč vyčerpal svých 12 spuštění pro tuto hodinu. |
| `not_permitted` při spuštění | Klíč smí jen číst. Zapněte v kartě **Povolit spouštění záloh**; nové připojení není potřeba. U zrušení to znamená, že běh nespustil tento klíč. |
| `domain_off` | Tento druh zálohy je v nastavení vypnutý. |
| `not_found` | BombVault tuto položku nechrání. Nejdřív ji přidejte ve webovém rozhraní; MCP nikdy nevytváří konfiguraci. |

Nenastavujte na kontejneru proměnnou prostředí `MCPGODEBUG`. Mění chování knihovny MCP a chybná hodnota zastaví BombVault při startu dřív, než zapíše jediný řádek logu.
