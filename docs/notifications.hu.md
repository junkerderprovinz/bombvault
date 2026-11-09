# Értesítések

A BombVault szólhat, ha egy mentés sikertelen, ha sikerül, ha késik, és hetente egyszer arról is, hogyan alakultak a dolgok. Minden a **Beállítások, Értesítések** alatt állítható: az **Értesítések** kártyán az, hogy a BombVault az egyes mentéseken túl mit küld, a **Csatornák** kártyán pedig az, hogy hová mennek az üzenetek. Minden csatorna maga dönti el, mikor szól, és egy Healthchecks-ellenőrzés is csatorna.

![Beállítások, Értesítések a kitöltött Gotify csatornával.](assets/screenshots/notifications/gotify-dark.png#only-dark)
![Beállítások, Értesítések a kitöltött Gotify csatornával.](assets/screenshots/notifications/gotify-light.png#only-light)

## Mi küld üzenetet {#what}

Az **Értesítések** kártya kapcsolói minden csatornára érvényesek:

- **Ütemezett futások összegzése**: ütemezett futásonként egy üzenet, például *45-ből 42 sikerült*, konténerenként vagy VM-enként egy helyett. A kézzel indított mentés továbbra is saját üzenetet küld. Egy befagyasztott vendég, egy sikertelen megőrzés vagy egy átlépett növekedési keret az összegzések mellett is azonnal kimegy.
- **Értesítés container frissítésekor**: egy üzenet minden olyan konténerről, amelyet a **Konténerek frissítése egy sikeres biztonsági mentés után** újabb image-re léptetett, hogy ellenőrizhesd, még működik-e.
- **Késő mentések jelentése**: naponta egyszer 09:00-kor a BombVault megnézi, késik-e egy bekapcsolt mentés, vagyis régebbi-e az ütemezése kétszeresénél, és esetenként egy üzenetet küld, amely megnevezi a lemaradt elemet. A következő sikeres mentés törli.
- **Heti összefoglaló**: hetente egy összegzés a futások számával, az új mentési adattal, a késésben lévő szakaszokkal, azzal, hogy a telephelyen kívüli másolatok frissek-e, és a leggyakoribb hibákkal. Az ütemezése ugyanolyan gomb, mint bármelyik másik terv, és a **Ki** kikapcsolja. Lásd: [Ütemezések](schedules.md#plans).

Hogy egy csatorna a sikerekről is hall-e, azt magán a csatornán állítod be, az **Értesítés** alatt. Hogy mely anomáliák küldenek üzenetet, a **Beállítások, Ellenőrzés** alatt állítható, lásd: [Anomáliák](anomalies.md). Egy elhalasztott mentés egy üzenetet küld, amikor várakozni kezd, és egyet, amikor a várakozás véget ér, nem próbálkozásonként egyet.

## Csatornák {#channels}

A **Csatornák** kártya alján a **Csatorna hozzáadása** ezeket kínálja: **Unraid-értesítések**, **ntfy**, **Gotify**, **Discord**, **Slack**, **Matrix**, **E-mail (SMTP)**, **Apprise**, **Webhook** és **Healthchecks**. Annyit adhatsz hozzá, amennyit szeretnél, ugyanabból a fajtából kettőt is, például egy ntfy-témát a hibáknak és egy másikat mindennek. Egy új csatorna kikapcsolva indul, így már el van mentve, mielőtt kitöltenéd a mezőit.

Minden csatornának van:

- saját kapcsolója, amely kikapcsolja anélkül, hogy a mezői elvesznének.
- **Értesítés** beállítása: **Soha**, **Csak hiba esetén** vagy **Sikernél és hibánál**. Egy új csatorna a **Csak hiba esetén** beállítással indul. Az a mentés, amely sikerült, de kihagyott valamit, a csak hibát jelző csatornához is eljut.
- **Név** mezője, amely nem kötelező, és a fajta helyén jelenik meg, ez jól jön, ha egy fajtából kettő van.
- **Teszt küldése** gombja, amely próbaüzenetet küld ezen a csatornán az űrlapon éppen látható értékekkel, akár mentetted őket, akár nem. A csatorna alatti sor megmondja, átvette-e a szolgáltatás, és ha nem, miért.
- **Csatorna eltávolítása** gombja, a mellette lévő kuka.

Minden csatorna magától és saját időkorláttal küld, így egy hoszt, amely sosem válaszol, nem tartja fel a többit. Ha egy szolgáltatás úgy hibázik, hogy egy újabb próbálkozás segíthet, a csatorna legfeljebb háromszor próbálkozik. A késésről, a növekedési keretről vagy egy anomáliáról szóló riasztást, amelyet egyik csatorna sem vett át, később újra megpróbálja, és ugyanígy a heti összefoglalót is.

| Csatorna | Mezők | Megjegyzések |
|---|---|---|
| **Unraid-értesítések** | nincs | Az Unraid saját értesítési rendszerébe küld, amely továbbíthat Pushoverre, e-mailre vagy Discordra. A **Beállítások, Integrációk, Gazdagép SSH** alatti SSH-kapcsolaton fut, ezért annak a kulcsnak engedélyezettnek kell lennie. TrueNAS-on és más hosztokon csendben marad, és a csatorna megmondja, miért. |
| **ntfy** | **Téma URL**, **Hozzáférési token (nem kötelező)** | A cím a témád nevével végződik. Token csak védett témához kell. |
| **Gotify** | **Szerver URL**, **Alkalmazás-token**, **Prioritás hibánál**, **Prioritás sikernél** | A prioritások 0-tól 10-ig terjednek, hibánál 8-cal, sikernél 5-tel indulnak. |
| **Discord** | **Webhook URL** | Egy webhook címe a Discord-csatorna beállításaiból. |
| **Slack** | **Webhook URL** | A Slack-alkalmazásod egy bejövő webhookjának címe. |
| **Matrix** | **Homeserver URL**, **Hozzáférési token**, **Szoba azonosító** | A token mögötti fióknak benne kell lennie a szobában. |
| **E-mail (SMTP)** | **SMTP hoszt**, **Port**, **Titkosítás**, **Felhasználónév**, **Jelszó**, **Feladó címe**, **Címzett címe** | A **Titkosítás** értéke **STARTTLS**, **TLS (implicit)** vagy **None**. |
| **Apprise** | **Apprise API URL**, **Címkék (opcionális)**, **Apprise-címek (nem kötelező)** | Az Apprise API konténered `/notify/<key>` végpontjára irányítsd, hogy elérd a 100+ szolgáltatását, a címkék pedig a kulcs céljainak egy részhalmazát választják ki. Vagy sorold fel az Apprise-címeket, soronként egyet, például `tgram://` a Telegramhoz vagy `pover://` a Pushoverhez, és ekkor az API URL kulcs nélkül, `/notify` végződéssel szerepeljen. |
| **Webhook** | **Webhook URL**, **Metódus**, **Fejlécek**, **Törzs** | Egy kérés, amelyet magad alakítasz, lásd: [Webhook](#webhook). |
| **Healthchecks** | **Ping URL** | Lásd: [Healthchecks](#healthchecks). |

A titkos adatok, például a tokenek, az SMTP-jelszó és a webhook-fejlécek értékei titkosítva tárolódnak, és a szerver soha nem adja vissza őket az oldalnak. Egy elmentett titok alatt a mező ezt írja: **Elmentve. Hagyd üresen a mezőt, hogy megmaradjon.** Ha megváltoztatod, hová küld egy csatorna, a BombVault nem küldi el a régi titkot az új helyre: a mezőben ez áll: **Az új cím akkor mentődik, amikor ezt újra megadod.**, és az új cím akkor mentődik, ha beírod.

A csatornalista előtti verziók beállításait a BombVault az első indításkor átveszi, és semmi sem vész el. Minden kitöltött csatornád külön bejegyzés lesz, a korábbi **Értesítés** beállításoddal. A Discord, Slack, Gotify vagy ntfy formátumú webhook ilyen fajtájú csatorna lesz, a Generic JSON webhook pedig a **Törzs** révén továbbra is a `title`, `message` és `ok` mezőket küldi. Az Unraid kapcsolóból Unraid-csatorna lesz, a Healthchecks URL-ből a kivételeivel együtt Healthchecks-csatorna.

### Webhook {#webhook}

A **Webhook** csatorna egyetlen HTTP-kérést küld bármilyen címre. A **Metódus** lehet GET, POST vagy PUT. A **Fejléc hozzáadása** új sort ad a **Fejlécek** alá, és a fejlécek értékei titokként tárolódnak, így egy `Authorization` fejléc ott biztonságban van. A **Törzs** pontosan úgy megy ki, ahogy megírod, ezekkel a helyőrzőkkel kitöltve:

| Helyőrző | Ez lesz belőle |
|---|---|
| `%%title%%` | az üzenet címe |
| `%%body%%` | maga az üzenet |
| `%%ok%%` | `true`, hibánál `false` |
| `%%event%%` | ami történt, például `container.failed` |
| `%%severity%%` | `info`, `success`, `warning`, `failure` vagy `critical` |
| `%%importance%%` | az Unraid szintje hozzá: `normal`, `warning` vagy `alert` |
| `%%link%%` | az üzenethez tartozó link, üres, ha nincs ilyen |
| `%%key%%` | a szakasz, például `container` vagy `VM` |
| `%%time%%` | az idő UTC-ben, például `2026-11-09T03:00:00Z` |
| `%%source%%` | `BombVault` |

A helyőrzők a címben és a fejlécek értékeiben is működnek, és a BombVault minden értéket a helyéhez illően kódol: JSON-ként, ha a `Content-Type` fejléc JSON-t ad meg, és lekérdezési paraméterként a címben. Egy új webhook ezzel a fejléccel indul, és ezt küldi: `{"title":"%%title%%","message":"%%body%%","ok":%%ok%%}`. Az a név, amelyet a BombVault nem ismer, úgy marad, ahogy írtad, így egy elírás látszik az üzenetben. Ha üresen hagyod a **Törzs** mezőt, nem küld törzset.

## Healthchecks {#healthchecks}

Egy olyan szolgáltatás, mint a [Healthchecks.io](https://healthchecks.io), riaszt, ha a BombVault abbahagyja a bejelentkezést. Adj hozzá egy **Healthchecks** csatornát, és illeszd be az ellenőrzés **Ping URL**-jét, a BombVault pedig minden mentés teljes életciklusa alatt pingeli: az elején, majd sikernél vagy `/fail` címmel. Ennek a csatornának nincs **Értesítés** beállítása, mert egy ellenőrzésnek szüksége van a sikeres pingekre, hogy zöld maradjon. Az a szakasz, amelynek nincs hová mentenie, sikertelennek számít, így az ellenőrzése piros lesz, ahelyett hogy sikert kapna.

Az ugyanazon az URL-en lévő szakaszok egy ellenőrzésen osztoznak. Ha az egyik mentése meghiúsult, a többiek sikeres mentései is a `/fail` címet pingelik, és megnevezik, így az ellenőrzés piros marad, amíg annak az elemnek újra sikeres mentése nem lesz, vagy amíg nem nyugtázod a hibát a **Figyelmet igényel** alatt.

Egy szakasz saját címet is pingelhet: a Healthchecks-csatornában a **Kivétel** hoz létre egyet, és a sor ekkor ezt írja: **Az összes többi szakasz erre az URL-re pingel.** A **Teszt küldése** bejegyzést ír minden ellenőrzés naplójába, és az állapotukat változatlanul hagyja.

## Példa: Gotify a telefonodon {#example-gotify}

A Gotify a `https://gotify.example.com` címen fut nálad, és azt szeretnéd, hogy a hibák csengjenek a telefonodon, a sikerek pedig csendben érkezzenek.

1. A Gotifyban nyisd meg az **Apps** részt, hozz létre egy `BombVault` nevű alkalmazást, és másold ki a tokenjét.
2. A BombVaultban nyisd meg a **Beállítások, Értesítések** oldalt, és kapcsold be az **Ütemezett futások összegzése** beállítást, hogy egy 45 mentéses éjszaka egy üzenetet küldjön 45 helyett.
3. A **Csatornák** kártyán nyomd meg a **Csatorna hozzáadása** gombot, és válaszd a **Gotify**-t.
4. Add meg a `https://gotify.example.com` címet **Szerver URL**-ként. A `/message` részt a BombVault maga teszi hozzá.
5. Illeszd be a tokent az **Alkalmazás-token** mezőbe.
6. Állítsd az **Értesítés** értékét **Sikernél és hibánál**-ra. Hagyd a **Prioritás hibánál** értékét 8-on, a **Prioritás sikernél** értékét 5-ön. A Gotify Android-alkalmazása a 8-at és afölöttit magasnak veszi, így csak a hibák szólalnak meg.
7. Kapcsold be a csatornát, és nyomd meg a **Teszt küldése** gombot. A csatorna alatti sor megmondja, átvette-e a Gotify, és az elutasított tokent is megnevezi.

A következő ütemezett futás egy összegzést küld 5-ös prioritással, vagy 8-assal, ha bármi sikertelen volt benne.

## Példa: ntfy fiók nélkül {#example-ntfy}

Az ntfy-hoz nem kell fiók egy nyilvános témához. Válassz nehezen kitalálható témanevet, mert bárki elolvashatja, aki ismeri.

1. A **Csatornák** kártyán nyomd meg a **Csatorna hozzáadása** gombot, és válaszd az **ntfy**-t.
2. Add meg a téma címét **Téma URL**-ként, például `https://ntfy.sh/bombvault-tower-7f3k`, vagy egy témát a saját ntfy-szervereden. Nyilvános témánál hagyd üresen a **Hozzáférési token (nem kötelező)** mezőt.
3. Iratkozz fel ugyanarra a témára az ntfy alkalmazásban, kapcsold be a csatornát, és nyomd meg a **Teszt küldése** gombot.

A BombVault az üzenetet törzsként küldi el, a címével a `Title` fejlécben. A hiba magas prioritással és `warning` címkével megy ki, így kitűnik az alkalmazásban, a befagyva maradt VM pedig sürgősként.

## Ha az üzenetek nem érkeznek meg {#troubleshooting}

Lásd: [Az értesítések nem érkeznek meg](troubleshooting.md#notifications-do-not-arrive) a Hibaelhárításban.
