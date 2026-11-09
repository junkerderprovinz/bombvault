# Oznámení

BombVault ti dokáže říct, když záloha selže, když uspěje, když je nějaká po termínu a jednou týdně, jak to šlo. Všechno se nastavuje v **Nastavení, Oznámení**: co BombVault posílá nad rámec jednotlivých záloh v kartě **Oznámení** a kam zprávy jdou v kartě **Kanály**. Každý kanál sám rozhoduje, kdy se ozve, a kanálem je i kontrola Healthchecks.

![Nastavení, Oznámení s vyplněným kanálem Gotify.](assets/screenshots/notifications/gotify-dark.png#only-dark)
![Nastavení, Oznámení s vyplněným kanálem Gotify.](assets/screenshots/notifications/gotify-light.png#only-light)

## Co posílá zprávu {#what}

Přepínače v kartě **Oznámení** platí pro všechny kanály:

- **Shrnout naplánované běhy**: jedna zpráva za naplánovaný běh, třeba *42 z 45 uspělo*, místo jedné za kontejner nebo VM. Záloha, kterou spustíš ručně, pořád pošle vlastní zprávu. Zamrzlý host, neúspěšné uchovávání nebo překročený rozpočet růstu odejdou hned, i když jsou souhrny zapnuté.
- **Upozornit na aktualizaci kontejneru**: jedna zpráva za každý kontejner, který **Aktualizovat kontejnery po úspěšné záloze** posunulo na novější image, abys mohl ověřit, že pořád funguje.
- **Hlásit zálohy po termínu**: jednou denně v 09:00 BombVault zkontroluje, zda některá zapnutá záloha není po termínu, tedy starší než dvojnásobek jejího plánu, a pošle jednu zprávu za každý případ, se jménem položky, která se opozdila. Příští úspěšná záloha ji zruší.
- **Týdenní souhrn**: jeden souhrn týdně s počty běhů, novými daty záloh, tím, které sekce jsou po termínu, zda jsou kopie mimo lokalitu aktuální, a nejčastějšími selháními. Jeho plán je tlačítko jako každý jiný plán a **Vypnuto** ho vypne. Viz [Plány](schedules.md#plans).

Zda kanál slyší i o úspěších, se nastavuje přímo u kanálu v **Oznámit**. Které anomálie pošlou zprávu, se nastavuje v **Nastavení, Ověření**, viz [Anomálie](anomalies.md). Odložená záloha pošle jednu zprávu, když začne čekat, a jednu, když čekání skončí, ne jednu za pokus.

## Kanály {#channels}

**Přidat kanál** dole v kartě **Kanály** nabízí **Oznámení Unraid**, **ntfy**, **Gotify**, **Discord**, **Slack**, **Matrix**, **E-mail (SMTP)**, **Apprise**, **Webhook** a **Healthchecks**. Přidej jich, kolik chceš, i dva stejného druhu, třeba jedno téma ntfy pro selhání a druhé pro všechno. Nový kanál začíná vypnutý, takže se uloží dřív, než vyplníš jeho pole.

Každý kanál má:

- vlastní přepínač, který ho vypne, aniž by přišel o svá pole.
- **Oznámit**: **Nikdy**, **Pouze při selhání** nebo **Při úspěchu i selhání**. Nový kanál začíná na **Pouze při selhání**. Záloha, která uspěla, ale něco vynechala, dorazí i do kanálu jen pro selhání.
- **Název**, který je volitelný a zobrazí se místo druhu. Hodí se, když máš dva kanály stejného druhu.
- **Odeslat test**, který pošle zkušební zprávu tímto kanálem s hodnotami tak, jak stojí ve formuláři, uložené nebo ne. Řádek pod kanálem říká, zda ji služba přijala, a pokud ne, proč.
- **Odebrat kanál**, koš vedle něj.

Každý kanál posílá sám, s vlastním časovým limitem, takže jeden hostitel, který nikdy neodpoví, ostatní nezdrží. Když služba selže tak, že by další pokus mohl pomoct, kanál to zkusí až třikrát. Upozornění na zpoždění, rozpočet nebo anomálii, které žádný kanál nepřijal, se později zkusí znovu, stejně jako týdenní souhrn.

| Kanál | Pole | Poznámky |
|---|---|---|
| **Oznámení Unraid** | žádná | Posílá do vlastního systému oznámení Unraidu, který umí přeposílat na Pushover, e-mail nebo Discord. Běží přes spojení SSH z **Nastavení, Integrace, SSH k hostiteli**, takže ten klíč musí být autorizovaný. Na TrueNAS a jiných hostitelích mlčí a kanál řekne proč. |
| **ntfy** | **URL tématu**, **Přístupový token (volitelné)** | Adresa končí tvým tématem. Token potřebuješ jen pro chráněné téma. |
| **Gotify** | **URL serveru**, **Token aplikace**, **Priorita při selhání**, **Priorita při úspěchu** | Priority jdou od 0 do 10 a začínají na 8 pro selhání a 5 pro úspěchy. |
| **Discord** | **URL Webhook** | Adresa webhooku z nastavení kanálu na Discordu. |
| **Slack** | **URL Webhook** | Adresa příchozího webhooku tvé aplikace Slack. |
| **Matrix** | **URL Homeserver**, **Přístupový token**, **ID místnosti** | Účet, kterému token patří, musí být v místnosti. |
| **E-mail (SMTP)** | **Host SMTP**, **Port**, **Šifrování**, **Uživatelské jméno**, **Heslo**, **Adresa odesílatele**, **Adresa příjemce** | **Šifrování** je **STARTTLS**, **TLS (implicit)** nebo **None**. |
| **Apprise** | **URL Apprise API**, **Tagy (volitelné)**, **Adresy Apprise (volitelné)** | Nasměruj ho na koncový bod `/notify/<key>` svého kontejneru Apprise API a dosáhneš na jeho více než 100 služeb, tagy pak vyberou podmnožinu cílů toho klíče. Nebo vypiš adresy Apprise, jednu na řádek, třeba `tgram://` pro Telegram nebo `pover://` pro Pushover, a URL API zakonči `/notify` bez klíče. |
| **Webhook** | **URL Webhook**, **Metoda**, **Hlavičky**, **Tělo** | Požadavek, který si poskládáš sám, viz [Webhook](#webhook). |
| **Healthchecks** | **URL pingu** | Viz [Healthchecks](#healthchecks). |

Tajné údaje jako tokeny, heslo SMTP a hodnoty hlaviček webhooku se ukládají zašifrované a server je stránce nikdy nevrátí. Pod uloženým tajemstvím pole říká **Uloženo. Nech pole prázdné, aby zůstalo zachováno.** Když změníš, kam kanál posílá, BombVault staré tajemství na nové místo neposílá: pole říká **Nová adresa se uloží, jakmile toto zadáš znovu.** a nová adresa se uloží, až ho napíšeš.

Nastavení z verze před seznamem kanálů se při prvním startu převezmou a nic se neztratí. Každý kanál, který jsi měl vyplněný, se stane samostatnou položkou s volbou **Oznámit**, kterou jsi měl. Webhook nastavený na formát Discord, Slack, Gotify nebo ntfy se stane kanálem toho druhu a webhook Generic JSON dál posílá `title`, `message` a `ok` přes své **Tělo**. Z přepínače Unraid se stane kanál Unraid a z URL Healthchecks s jejími výjimkami kanál Healthchecks.

### Webhook {#webhook}

Kanál **Webhook** pošle jeden požadavek HTTP na libovolnou adresu. **Metoda** je GET, POST nebo PUT. **Přidat hlavičku** přidá řádek pod **Hlavičky** a hodnoty hlaviček se uchovávají jako tajné údaje, takže hlavička `Authorization` je tam v bezpečí. **Tělo** se pošle přesně tak, jak ho napíšeš, a tyto zástupné symboly se v něm doplní:

| Zástupný symbol | Změní se na |
|---|---|
| `%%title%%` | titulek zprávy |
| `%%body%%` | samotnou zprávu |
| `%%ok%%` | `true`, nebo `false` při selhání |
| `%%event%%` | to, co se stalo, třeba `container.failed` |
| `%%severity%%` | `info`, `success`, `warning`, `failure` nebo `critical` |
| `%%importance%%` | odpovídající úroveň Unraidu: `normal`, `warning` nebo `alert` |
| `%%link%%` | odkaz, který ke zprávě patří, nebo prázdný text, když žádný není |
| `%%key%%` | sekci, třeba `container` nebo `VM` |
| `%%time%%` | čas v UTC, třeba `2026-11-09T03:00:00Z` |
| `%%source%%` | `BombVault` |

Zástupné symboly fungují i v adrese a v hodnotách hlaviček a každá hodnota se escapuje podle toho, kde stojí: jako JSON, když hlavička `Content-Type` říká JSON, a jako hodnota dotazu v adrese. Nový webhook začíná s touto hlavičkou a posílá `{"title":"%%title%%","message":"%%body%%","ok":%%ok%%}`. Název, který BombVault nezná, zůstane tak, jak je napsaný, takže překlep uvidíš ve zprávě. Když necháš **Tělo** prázdné, žádné se nepošle.

## Healthchecks {#healthchecks}

Služba jako [Healthchecks.io](https://healthchecks.io) spustí poplach, když BombVault přestane hlásit, že žije. Přidej kanál **Healthchecks**, vlož do něj **URL pingu** té kontroly a BombVault ji bude pingovat po celý životní cyklus každé zálohy: na začátku, pak při úspěchu nebo s `/fail`. Tento kanál nemá volbu **Oznámit**, protože kontrola potřebuje pingy o úspěchu, aby zůstala zelená. Sekce, která nemá kam zálohovat, se počítá jako selhaná, takže její kontrola zčervená, místo aby slyšela úspěch.

Sekce na stejné URL sdílejí jednu kontrolu. Jakmile záloha jedné z nich selže, dobré zálohy ostatních pingnou také `/fail` a jmenují ji, takže kontrola zůstane červená, dokud ta položka zase nemá dobrou zálohu nebo dokud selhání nepotvrdíš pod **Vyžaduje pozornost**.

Sekce může pingovat vlastní adresu: **Výjimka** v kanálu Healthchecks ji přidá a řádek pak říká **Všechny ostatní sekce pingují tuto URL.** **Odeslat test** zapíše do logu každé kontroly záznam, který její stav nezmění.

## Příklad: Gotify v telefonu {#example-gotify}

Provozuješ Gotify na `https://gotify.example.com` a chceš, aby selhání zvonila v telefonu, zatímco úspěchy dorazí potichu.

1. V Gotify otevři **Apps**, vytvoř aplikaci jménem `BombVault` a zkopíruj její token.
2. V BombVaultu otevři **Nastavení, Oznámení** a zapni **Shrnout naplánované běhy**, aby noc o 45 zálohách poslala jednu zprávu místo 45.
3. V **Kanály** stiskni **Přidat kanál** a vyber **Gotify**.
4. Zadej `https://gotify.example.com` jako **URL serveru**. BombVault přidá `/message` sám.
5. Vlož token do **Token aplikace**.
6. Nastav **Oznámit** na **Při úspěchu i selhání**. Nech **Priorita při selhání** na 8 a **Priorita při úspěchu** na 5. Aplikace Gotify pro Android bere 8 a víc jako vysokou, takže zvuk vydají jen selhání.
7. Zapni kanál a stiskni **Odeslat test**. Řádek pod kanálem říká, zda to Gotify přijalo, a odmítnutý token je jako takový pojmenován.

Příští plánovaný běh pošle jeden souhrn s prioritou 5, nebo 8, když v něm cokoli selhalo.

## Příklad: ntfy bez účtu {#example-ntfy}

ntfy nepotřebuje u veřejného tématu žádný účet. Zvol název tématu, který se těžko hádá, protože ho může číst každý, kdo ho zná.

1. V **Kanály** stiskni **Přidat kanál** a vyber **ntfy**.
2. Zadej adresu tématu jako **URL tématu**, například `https://ntfy.sh/bombvault-tower-7f3k`, nebo téma na svém vlastním serveru ntfy. U veřejného tématu nech **Přístupový token (volitelné)** prázdné.
3. Přihlas se k odběru stejného tématu v aplikaci ntfy, zapni kanál a stiskni **Odeslat test**.

BombVault pošle zprávu jako tělo s titulkem v hlavičce `Title`. Selhání odejde s vysokou prioritou a štítkem `warning`, takže v aplikaci vynikne, a zpráva o VM, která zůstala zamrzlá, odejde jako naléhavá.

## Když zprávy nedorazí {#troubleshooting}

Viz [Oznámení nedorazí](troubleshooting.md#notifications-do-not-arrive) v Řešení problémů.
