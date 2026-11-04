# Értesítések

A BombVault szólhat, ha egy mentés sikertelen, ha sikerül, ha késik, és hetente egyszer arról is, hogyan alakultak a dolgok. Minden a **Beállítások, Értesítések** alatt állítható: mi vált ki üzenetet az **Értesítések** kártyán, hová mennek az üzenetek a **Csatornák** kártyán, és a bejelentkező pingek a **Healthchecks** kártyán.

![Beállítások, Értesítések a kitöltött Gotify csatornával.](assets/screenshots/notifications/gotify-dark.png#only-dark)
![Beállítások, Értesítések a kitöltött Gotify csatornával.](assets/screenshots/notifications/gotify-light.png#only-light)

## Mi küld üzenetet {#what}

Az **Értesítések** kártya egyszerre állítja be a szabályokat az összes csatornára:

- **Értesítés**: **Soha**, **Csak hiba esetén** vagy **Sikernél és hibánál**, az alábbi összes csatornára.
- **Ütemezett futások összegzése**: ütemezett futásonként egy üzenet, például *45-ből 42 sikerült*, konténerenként vagy VM-enként egy helyett. A kézzel indított mentés továbbra is saját üzenetet küld. Egy befagyasztott vendég, egy sikertelen megőrzés vagy egy átlépett növekedési keret az összegzések mellett is azonnal kimegy.
- **Értesítés container frissítésekor**: egy üzenet minden olyan konténerről, amelyet a **Konténerek frissítése egy sikeres biztonsági mentés után** újabb image-re léptetett, hogy ellenőrizhesd, még működik-e.
- **Késő mentések jelentése**: naponta egyszer 09:00-kor a BombVault megnézi, késik-e egy bekapcsolt mentés, vagyis régebbi-e az ütemezése kétszeresénél, és esetenként egy üzenetet küld. A következő sikeres mentés törli.
- **Unraid-értesítések**: az Unraid saját értesítési rendszerébe küld, amely továbbíthat Pushoverre, e-mailre vagy Discordra. A **Beállítások, Integrációk, Gazdagép SSH** alatti SSH-kapcsolaton fut, ezért annak a kulcsnak engedélyezettnek kell lennie. TrueNAS-on és más hosztokon kikapcsolva marad.
- **Heti összefoglaló**: hetente egy összegzés a futások számával, az új mentési adattal, azzal, hogy a telephelyen kívüli másolatok frissek-e, és a leggyakoribb hibákkal. Az ütemezése ugyanolyan gomb, mint bármelyik másik terv, és a **Ki** kikapcsolja. Lásd: [Ütemezések](schedules.md#plans).

Hogy mely anomáliák küldenek üzenetet, a **Beállítások, Ellenőrzés** alatt állítható, lásd: [Anomáliák](anomalies.md). Egy elhalasztott mentés egy üzenetet küld, amikor várakozni kezd, és egyet, amikor a várakozás véget ér, nem próbálkozásonként egyet.

A kártya alján a **Teszt küldése** próbaüzenetet küld minden bekapcsolt csatornán, az űrlapon éppen látható értékekkel.

## Csatornák {#channels}

Minden csatornának saját kapcsolója van a **Csatornák** kártyán, és a mezői akkor jelennek meg, ha be van kapcsolva. Minden csatorna magától és saját időkorláttal küld, így egy hoszt, amely sosem válaszol, nem tartja fel a többit, és az üzenetet, amelyet egyik csatorna sem vett át, újra megpróbálja.

| Csatorna | Mezők | Megjegyzések |
|---|---|---|
| **Webhook** | **Webhook URL**, **Webhook formátum** | Formátumok: **Generic JSON**, **Discord**, **Slack**, **Gotify** és **ntfy**. A Generic JSON a `title`, `message` és `ok` mezőket küldi. |
| **Apprise** | **Apprise API URL**, **Címkék (opcionális)** | Az Apprise API konténered `/notify/<key>` végpontjára irányítsd, hogy elérd a 100+ szolgáltatását. A címkék a kulcs céljainak egy részhalmazát választják ki. |
| **Matrix** | **Homeserver URL**, **Hozzáférési token**, **Szoba azonosító** | A token a szerveren marad, és soha többé nem jelenik meg. |
| **Gotify** | **Szerver URL**, **Alkalmazás-token**, **Prioritás hibánál**, **Prioritás sikernél** | A prioritások 0-tól 10-ig terjednek, hibánál 8-cal, sikernél 5-tel indulnak. |
| **E-mail (SMTP)** | **SMTP hoszt**, **Port**, **Titkosítás**, **Felhasználónév**, **Jelszó**, **Feladó címe**, **Címzett címe** | A **Titkosítás** értéke **STARTTLS**, **TLS (implicit)** vagy **Nincs**. |

A titkos adatok, például a Matrix-token, a Gotify alkalmazás-token és az SMTP-jelszó titkosítva tárolódnak. Egy elmentett titok alatt a mező ezt írja: **Elmentve. Hagyd üresen a mezőt, hogy megmaradjon.** Ha megváltoztatod a szerver címét, a BombVault nem küldi el a régi titkot az új helyre: a mező újra kéri, és az új cím akkor mentődik, ha beírod.

## Healthchecks {#healthchecks}

Egy olyan szolgáltatás, mint a [Healthchecks.io](https://healthchecks.io), riaszt, ha a BombVault abbahagyja a bejelentkezést. Illeszd be az ellenőrzés **Ping URL**-jét, és a BombVault minden mentés teljes életciklusa alatt pingeli: az elején, majd sikernél vagy `/fail` címmel. Ezt minden alkalommal megteszi, ha van beállított URL, függetlenül attól, mit mond az **Értesítés**, így az ellenőrzés sikernél akkor is zöld marad, ha csak hibáról jönnek üzenetek.

Egy szakasz saját címet is pingelhet: a kártyán a **Kivétel** hoz létre egyet, és a sor ezt írja: **Az összes többi szakasz erre az URL-re pingel.**

## Példa: Gotify a telefonodon {#example-gotify}

A Gotify a `https://gotify.example.com` címen fut nálad, és azt szeretnéd, hogy a hibák csengjenek a telefonodon, a sikerek pedig csendben érkezzenek.

1. A Gotifyban nyisd meg az **Apps** részt, hozz létre egy `BombVault` nevű alkalmazást, és másold ki a tokenjét.
2. A BombVaultban nyisd meg a **Beállítások, Értesítések** oldalt. Az **Értesítések** kártyán állítsd az **Értesítés** értékét **Sikernél és hibánál**-ra, és kapcsold be az **Ütemezett futások összegzése** beállítást, hogy egy 45 mentéses éjszaka egy üzenetet küldjön 45 helyett.
3. A **Csatornák** között kapcsold be a **Gotify**-t.
4. Add meg a `https://gotify.example.com` címet **Szerver URL**-ként. A `/message` részt a BombVault maga teszi hozzá.
5. Illeszd be a tokent az **Alkalmazás-token** mezőbe.
6. Hagyd a **Prioritás hibánál** értékét 8-on, a **Prioritás sikernél** értékét 5-ön. A Gotify Android-alkalmazása a 8-at és afölöttit magasnak veszi, így csak a hibák szólalnak meg.
7. Nyomd meg a **Teszt küldése** gombot. A kártya alatti sor megmondja, átvette-e a Gotify, és az elutasított tokent is megnevezi.

A következő ütemezett futás egy összegzést küld 5-ös prioritással, vagy 8-assal, ha bármi sikertelen volt benne.

## Példa: ntfy webhookként {#example-ntfy}

Az ntfy-hoz nem kell fiók egy nyilvános témához. Válassz nehezen kitalálható témanevet, mert bárki elolvashatja, aki ismeri.

1. A **Csatornák** között kapcsold be a **Webhook**-ot.
2. Add meg a téma címét **Webhook URL**-ként, például `https://ntfy.sh/bombvault-tower-7f3k`, vagy a saját ntfy-szervered címét.
3. Állítsd a **Webhook formátum** értékét **ntfy**-ra.
4. Iratkozz fel ugyanarra a témára az ntfy alkalmazásban, és nyomd meg a **Teszt küldése** gombot.

A BombVault az üzenetet törzsként küldi el, a címével az `X-Title` fejlécben. A hiba magas prioritással és `warning` címkével megy ki, így kitűnik az alkalmazásban.

## Ha az üzenetek nem érkeznek meg {#troubleshooting}

Lásd: [Az értesítések nem érkeznek meg](troubleshooting.md#notifications-do-not-arrive) a Hibaelhárításban.
