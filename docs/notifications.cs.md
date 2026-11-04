# Oznámení

BombVault ti dokáže říct, když záloha selže, když uspěje, když je nějaká po termínu a jednou týdně, jak to šlo. Všechno se nastavuje v **Nastavení, Oznámení**: co zprávu spouští v kartě **Oznámení**, kam zprávy jdou v kartě **Kanály** a pingy pro hlášení, že žiješ, v kartě **Healthchecks**.

![Nastavení, Oznámení s vyplněným kanálem Gotify.](assets/screenshots/notifications/gotify-dark.png#only-dark)
![Nastavení, Oznámení s vyplněným kanálem Gotify.](assets/screenshots/notifications/gotify-light.png#only-light)

## Co posílá zprávu {#what}

Karta **Oznámení** nastavuje pravidla pro všechny kanály najednou:

- **Oznámit**: **Nikdy**, **Pouze při selhání** nebo **Při úspěchu i selhání**, pro každý kanál níže.
- **Shrnout naplánované běhy**: jedna zpráva za naplánovaný běh, třeba *42 z 45 uspělo*, místo jedné za kontejner nebo VM. Záloha, kterou spustíš ručně, pořád pošle vlastní zprávu. Zamrzlý host, neúspěšné uchovávání nebo překročený rozpočet růstu odejdou hned, i když jsou souhrny zapnuté.
- **Upozornit na aktualizaci kontejneru**: jedna zpráva za každý kontejner, který **Aktualizovat kontejnery po úspěšné záloze** posunulo na novější image, abys mohl ověřit, že pořád funguje.
- **Hlásit zálohy po termínu**: jednou denně v 09:00 BombVault zkontroluje, zda některá zapnutá záloha není po termínu, tedy starší než dvojnásobek jejího plánu, a pošle jednu zprávu za každý případ. Příští úspěšná záloha ji zruší.
- **Oznámení Unraid**: posílá do vlastního systému oznámení Unraidu, který umí přeposílat na Pushover, e-mail nebo Discord. Běží přes spojení SSH z **Nastavení, Integrace, SSH k hostiteli**, takže ten klíč musí být autorizovaný. Na TrueNAS a jiných hostitelích zůstává vypnuté.
- **Týdenní souhrn**: jeden souhrn týdně s počty běhů, novými daty záloh, zda jsou kopie mimo lokalitu aktuální, a nejčastějšími selháními. Jeho plán je tlačítko jako každý jiný plán a **Vypnuto** ho vypne. Viz [Plány](schedules.md#plans).

Které anomálie pošlou zprávu, se nastavuje v **Nastavení, Ověření**, viz [Anomálie](anomalies.md). Odložená záloha pošle jednu zprávu, když začne čekat, a jednu, když čekání skončí, ne jednu za pokus.

**Odeslat test** dole v kartě pošle zkušební zprávu každým zapnutým kanálem, s hodnotami tak, jak právě stojí ve formuláři.

## Kanály {#channels}

Každý kanál má v kartě **Kanály** vlastní přepínač a jeho pole se objeví, jakmile je zapnutý. Každý kanál posílá sám, s vlastním časovým limitem, takže jeden hostitel, který nikdy neodpoví, ostatní nezdrží, a zpráva, kterou žádný kanál nepřijal, se zkusí znovu.

| Kanál | Pole | Poznámky |
|---|---|---|
| **Webhook** | **URL Webhook**, **Formát Webhook** | Formáty: **Generic JSON**, **Discord**, **Slack**, **Gotify** a **ntfy**. Generic JSON posílá `title`, `message` a `ok`. |
| **Apprise** | **URL Apprise API**, **Tagy (volitelné)** | Nasměruj ho na koncový bod `/notify/<key>` svého kontejneru Apprise API a dosáhneš na jeho více než 100 služeb. Tagy vyberou podmnožinu cílů toho klíče. |
| **Matrix** | **URL Homeserver**, **Přístupový token**, **ID místnosti** | Token zůstává na serveru a už se nikdy nezobrazí. |
| **Gotify** | **URL serveru**, **Token aplikace**, **Priorita při selhání**, **Priorita při úspěchu** | Priority jdou od 0 do 10 a začínají na 8 pro selhání a 5 pro úspěchy. |
| **E-mail (SMTP)** | **Host SMTP**, **Port**, **Šifrování**, **Uživatelské jméno**, **Heslo**, **Adresa odesílatele**, **Adresa příjemce** | **Šifrování** je **STARTTLS**, **TLS (implicit)** nebo **Žádné**. |

Tajné údaje jako token Matrixu, token aplikace Gotify a heslo SMTP se ukládají zašifrované. Pod uloženým tajemstvím pole říká **Uloženo. Nech pole prázdné, aby zůstalo zachováno.** Když změníš adresu serveru, BombVault staré tajemství na nové místo neposílá: pole se zeptá znovu a nová adresa se uloží, až ho napíšeš.

## Healthchecks {#healthchecks}

Služba jako [Healthchecks.io](https://healthchecks.io) spustí poplach, když BombVault přestane hlásit, že žije. Vlož **URL pingu** té kontroly a BombVault ji pinguje po celý životní cyklus každé zálohy: na začátku, pak při úspěchu nebo s `/fail`. Dělá to vždy, když je URL nastavená, ať říká **Oznámit** cokoli, takže kontrola zůstane zelená při úspěchu i se zprávami jen o selhání.

Sekce může pingovat vlastní adresu: **Výjimka** v kartě ji přidá a řádek říká **Všechny ostatní sekce pingují tuto URL.**

## Příklad: Gotify v telefonu {#example-gotify}

Provozuješ Gotify na `https://gotify.example.com` a chceš, aby selhání zvonila v telefonu, zatímco úspěchy dorazí potichu.

1. V Gotify otevři **Apps**, vytvoř aplikaci jménem `BombVault` a zkopíruj její token.
2. V BombVaultu otevři **Nastavení, Oznámení**. V **Oznámení** nastav **Oznámit** na **Při úspěchu i selhání** a zapni **Shrnout naplánované běhy**, aby noc o 45 zálohách poslala jednu zprávu místo 45.
3. V **Kanály** zapni **Gotify**.
4. Zadej `https://gotify.example.com` jako **URL serveru**. BombVault přidá `/message` sám.
5. Vlož token do **Token aplikace**.
6. Nech **Priorita při selhání** na 8 a **Priorita při úspěchu** na 5. Aplikace Gotify pro Android bere 8 a víc jako vysokou, takže zvuk vydají jen selhání.
7. Stiskni **Odeslat test**. Řádek pod kartou říká, zda to Gotify přijalo, a odmítnutý token je jako takový pojmenován.

Příští plánovaný běh pošle jeden souhrn s prioritou 5, nebo 8, když v něm cokoli selhalo.

## Příklad: ntfy jako webhook {#example-ntfy}

ntfy nepotřebuje u veřejného tématu žádný účet. Zvol název tématu, který se těžko hádá, protože ho může číst každý, kdo ho zná.

1. V **Kanály** zapni **Webhook**.
2. Zadej adresu tématu jako **URL Webhook**, například `https://ntfy.sh/bombvault-tower-7f3k`, nebo adresu svého vlastního serveru ntfy.
3. Nastav **Formát Webhook** na **ntfy**.
4. Přihlas se k odběru stejného tématu v aplikaci ntfy a stiskni **Odeslat test**.

BombVault pošle zprávu jako tělo s titulkem v `X-Title`. Selhání odejde s vysokou prioritou a štítkem `warning`, takže v aplikaci vynikne.

## Když zprávy nedorazí {#troubleshooting}

Viz [Oznámení nedorazí](troubleshooting.md#notifications-do-not-arrive) v Řešení problémů.
